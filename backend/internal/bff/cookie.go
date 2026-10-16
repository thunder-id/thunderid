// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package bff

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// maxCookieChunk keeps each cookie, with its name and attributes, under the 4096 byte limit
// browsers enforce per cookie.
const maxCookieChunk = 3800

// sealer encrypts cookie payloads with the server's encryption key.
type sealer struct {
	crypto providers.RuntimeCryptoProvider
}

func (s sealer) seal(ctx context.Context, payload any) (string, error) {
	plaintext, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	ciphertext, _, err := s.crypto.Encrypt(ctx, nil, string(cryptolib.AlgorithmAESGCM), nil, plaintext)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt cookie: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(ciphertext), nil
}

func (s sealer) open(ctx context.Context, value string, payload any) error {
	ciphertext, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return fmt.Errorf("failed to decode cookie: %w", err)
	}
	plaintext, err := s.crypto.Decrypt(ctx, nil, string(cryptolib.AlgorithmAESGCM), nil, ciphertext)
	if err != nil {
		return fmt.Errorf("failed to decrypt cookie: %w", err)
	}
	return json.Unmarshal(plaintext, payload)
}

// cookieSpec names one BFF cookie and fixes its scope.
type cookieSpec struct {
	name   string
	path   string
	secure bool
}

// newCookieSpec builds a cookie spec. A host-only cookie at path "/" gets the __Host- prefix and any
// other secure cookie the __Secure- prefix, so the browser enforces Secure and the scope.
func newCookieSpec(stem, path string, secure bool) cookieSpec {
	name := stem
	if secure {
		if path == "/" {
			name = "__Host-" + stem
		} else {
			name = "__Secure-" + stem
		}
	}
	return cookieSpec{name: name, path: path, secure: secure}
}

func (c cookieSpec) cookie(name, value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     c.path,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   c.secure,
		SameSite: http.SameSiteLaxMode,
	}
}

// read returns the cookie's value, joining its chunks when it was split.
func (c cookieSpec) read(r *http.Request) (string, bool) {
	if cookie, err := r.Cookie(c.name); err == nil && cookie.Value != "" {
		return cookie.Value, true
	}
	var value strings.Builder
	for i := 0; ; i++ {
		cookie, err := r.Cookie(c.chunkName(i))
		if err != nil {
			break
		}
		value.WriteString(cookie.Value)
	}
	if value.Len() == 0 {
		return "", false
	}
	return value.String(), true
}

// write sets the cookie, splitting a long value into chunks, and expires whatever the request
// carried under this name that the new value no longer uses.
func (c cookieSpec) write(w http.ResponseWriter, r *http.Request, value string, maxAge int) {
	written := map[string]bool{}
	if len(value) <= maxCookieChunk {
		http.SetCookie(w, c.cookie(c.name, value, maxAge))
		written[c.name] = true
	} else {
		for i := 0; len(value) > 0; i++ {
			n := min(maxCookieChunk, len(value))
			name := c.chunkName(i)
			http.SetCookie(w, c.cookie(name, value[:n], maxAge))
			written[name] = true
			value = value[n:]
		}
	}
	c.expireExcept(w, r, written)
}

// clear expires the cookie and the chunks the request carried. The unchunked name is expired even
// when the request did not carry it, because a cookie scoped to another path is never sent here.
func (c cookieSpec) clear(w http.ResponseWriter, r *http.Request) {
	if _, err := r.Cookie(c.name); err != nil {
		http.SetCookie(w, c.cookie(c.name, "", -1))
	}
	c.expireExcept(w, r, nil)
}

func (c cookieSpec) expireExcept(w http.ResponseWriter, r *http.Request, keep map[string]bool) {
	var stale []string
	for _, cookie := range r.Cookies() {
		if keep[cookie.Name] {
			continue
		}
		if cookie.Name == c.name || c.isChunk(cookie.Name) {
			stale = append(stale, cookie.Name)
		}
	}
	sort.Strings(stale)
	for _, name := range stale {
		http.SetCookie(w, c.cookie(name, "", -1))
	}
}

func (c cookieSpec) chunkName(i int) string {
	return c.name + "." + strconv.Itoa(i)
}

func (c cookieSpec) isChunk(name string) bool {
	suffix, ok := strings.CutPrefix(name, c.name+".")
	if !ok {
		return false
	}
	_, err := strconv.Atoi(suffix)
	return err == nil
}
