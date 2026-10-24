// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package userlookup puts names to users recorded by id. A plane that records who did something, such
// as who captured a configuration version, keeps the user's id only; the name is read from the
// trusted issuer's user API when the record is shown, as server.security.trusted_issuer.user_lookup
// configures it, and is never stored.
package userlookup

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/thunder-id/thunderid/internal/system/config"
	syshttp "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/internal/system/log"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// Timeout bounds one call to the issuer, for a token or a user.
const Timeout = 10 * time.Second

// tokenMargin is how long before a token expires it stops being used, so a call does not set out with
// a token that expires on the way.
const tokenMargin = 30 * time.Second

// maxNamesKept bounds the names kept in memory; the oldest are dropped when more are read.
const maxNamesKept = 1000

// failureTTL is how long a lookup that failed is not tried again, for that user or, when the token
// could not be had, for any. An issuer that is down then costs one round of calls and one log line,
// not a call for every read.
const failureTTL = 30 * time.Second

// defaultTokenLifetime is how long a token is used when the issuer does not say: RFC 6749 only
// recommends expires_in.
const defaultTokenLifetime = 5 * time.Minute

// maxConcurrentLookups bounds the users looked up at once for one read.
const maxConcurrentLookups = 8

// Lookup puts names to users recorded by id, from the trusted issuer's user API. The token the
// issuer's token endpoint grants the configured client is kept until shortly before it expires, and a
// name once read is kept for the configured time, so a listing that names the same few users many
// times asks the issuer once for each. A user the issuer does not know is kept as nameless for the
// same time, and a lookup that fails for a short while. One Lookup serves the whole process; it is
// safe for concurrent use.
type Lookup struct {
	config func() engineconfig.UserLookupConfig
	do     func(*http.Request) (*http.Response, error)
	logger *log.Logger
	now    func() time.Time

	// mu guards the names; tokenMu guards the token and is held across the request for one, so a
	// slow token endpoint holds up the lookups that need a token and not the reads of kept names.
	mu      sync.Mutex
	names   map[string]namedUntil
	tokenMu sync.Mutex
	token   string
	expires time.Time
	// tokenFailedUntil is when a token is next asked for after a request for one failed.
	tokenFailedUntil time.Time
}

// namedUntil is a name kept until a moment.
type namedUntil struct {
	name  string
	until time.Time
}

// New reads the lookup from the server configuration and calls the issuer with the server's outbound
// client.
func New() *Lookup {
	var (
		once   sync.Once
		client syshttp.HTTPClientInterface
	)
	return &Lookup{
		config: configured,
		do: func(req *http.Request) (*http.Response, error) {
			// The client reads the TLS settings from the runtime, which is loaded by the time a lookup runs.
			once.Do(func() { client = syshttp.NewHTTPClientWithTimeout(Timeout) })
			return client.Do(req)
		},
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "UserLookup")),
		now:    time.Now,
		names:  map[string]namedUntil{},
	}
}

// configured reads server.security.trusted_issuer.user_lookup.
func configured() engineconfig.UserLookupConfig {
	if !config.IsServerRuntimeInitialized() {
		return engineconfig.UserLookupConfig{}
	}
	return config.GetServerRuntime().Config.Server.SecurityConfig.TrustedIssuer.UserLookup
}

// Enabled reports whether a lookup is configured at all.
func (l *Lookup) Enabled() bool {
	return l.config().Enabled()
}

// Name is the name of the user with the id, or "" when no lookup is configured, the issuer does not
// know the user, or the lookup failed.
func (l *Lookup) Name(ctx context.Context, id string) string {
	return l.Names(ctx, id)[id]
}

// Names is the name of each of the users with the ids, under their id; a user with no name is absent.
// Each id is looked up once, the users not kept are looked up concurrently, and the whole read is
// bounded by one call's timeout, so a slow issuer delays a read by that long at most. The failures of
// one read are logged once.
func (l *Lookup) Names(ctx context.Context, ids ...string) map[string]string {
	names := map[string]string{}
	lookup := l.config()
	if !lookup.Enabled() {
		return names
	}
	var wanted []string
	seen := map[string]bool{}
	for _, id := range ids {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		if name, ok := l.kept(id); ok {
			if name != "" {
				names[id] = name
			}
			continue
		}
		wanted = append(wanted, id)
	}
	if len(wanted) == 0 {
		return names
	}

	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		failed   int
		firstErr error
	)
	slots := make(chan struct{}, maxConcurrentLookups)
	for _, id := range wanted {
		wg.Add(1)
		slots <- struct{}{}
		go func(id string) {
			defer func() { <-slots; wg.Done() }()
			name, err := l.fetch(ctx, lookup, id)
			ttl := lookup.CacheTTLOrDefault()
			if err != nil {
				ttl = failureTTL
			}
			l.keep(id, name, ttl)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err != nil:
				failed++
				if firstErr == nil {
					firstErr = err
				}
			case name != "":
				names[id] = name
			}
		}(id)
	}
	wg.Wait()
	if failed > 0 {
		l.logger.Warn(ctx, "Failed to look up the names of recorded users, who are left unnamed for now",
			log.Int("failed", failed), log.Error(firstErr))
	}
	return names
}

// kept reads a name read earlier that is still to be used: "" for a user the issuer does not know, or
// whose lookup failed a moment ago.
func (l *Lookup) kept(id string) (string, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	named, ok := l.names[id]
	if !ok || l.now().After(named.until) {
		return "", false
	}
	return named.name, true
}

// keep remembers a name, or that there is none, for ttl.
func (l *Lookup) keep(id, name string, ttl time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.names) >= maxNamesKept {
		// Starting over is cheap: a name is read again at most once per user shown.
		clear(l.names)
	}
	l.names[id] = namedUntil{name: name, until: l.now().Add(ttl)}
}

// fetch reads a user's name from the issuer, getting a fresh token once when the one held is refused.
func (l *Lookup) fetch(ctx context.Context, lookup engineconfig.UserLookupConfig, id string) (string, error) {
	token, err := l.accessToken(ctx, lookup, "")
	if err != nil {
		return "", err
	}
	name, status, err := l.user(ctx, lookup, id, token)
	if err == nil && status == http.StatusUnauthorized {
		if token, err = l.accessToken(ctx, lookup, token); err == nil {
			name, status, err = l.user(ctx, lookup, id, token)
		}
	}
	if err != nil {
		return "", err
	}
	switch status {
	case http.StatusOK:
		return name, nil
	case http.StatusNotFound:
		return "", nil
	default:
		return "", fmt.Errorf("the user lookup answered %d", status)
	}
}

// user reads one user from the issuer and returns the name attribute, with the status answered.
func (l *Lookup) user(ctx context.Context, lookup engineconfig.UserLookupConfig, id,
	token string) (string, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, lookup.UserURL(id), nil)
	if err != nil {
		return "", 0, fmt.Errorf("failed to build the user lookup request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	resp, err := l.do(req)
	if err != nil {
		return "", 0, fmt.Errorf("the user lookup failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", resp.StatusCode, nil
	}
	var user struct {
		Attributes map[string]interface{} `json:"attributes"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&user); err != nil {
		return "", 0, fmt.Errorf("the user lookup answered with something other than a user: %w", err)
	}
	name, _ := user.Attributes[lookup.NameAttributeOrDefault()].(string)
	return strings.TrimSpace(name), http.StatusOK, nil
}

// errTokenUnavailable is returned while a failed token request is not to be tried again.
var errTokenUnavailable = fmt.Errorf("the token endpoint failed a moment ago")

// accessToken returns the token the lookups present, asking the issuer for one when none is held, the
// one held is about to expire, or the one held is the token the issuer just rejected. Lookups that saw
// the same token rejected then share the one the first of them got. One request is in flight at a
// time, and a failed one is not tried again for a short while.
func (l *Lookup) accessToken(ctx context.Context, lookup engineconfig.UserLookupConfig,
	rejected string) (string, error) {
	l.tokenMu.Lock()
	defer l.tokenMu.Unlock()
	now := l.now()
	if l.token != "" && l.token != rejected && now.Before(l.expires) {
		return l.token, nil
	}
	if now.Before(l.tokenFailedUntil) {
		return "", errTokenUnavailable
	}
	token, lifetime, err := l.requestToken(ctx, lookup)
	if err != nil {
		l.tokenFailedUntil = now.Add(failureTTL)
		return "", err
	}
	l.token = token
	// A token that expires sooner than the margin is used for this call and asked for again next time.
	l.expires = now.Add(lifetime - tokenMargin)
	return token, nil
}

// requestToken asks the issuer for a client credentials token, and says how long it is good for.
func (l *Lookup) requestToken(ctx context.Context, lookup engineconfig.UserLookupConfig) (string,
	time.Duration, error) {
	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {lookup.ClientID},
		"client_secret": {lookup.ClientSecret},
	}
	if scope := strings.TrimSpace(strings.Join(lookup.Scopes, " ")); scope != "" {
		form.Set("scope", scope)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSpace(lookup.TokenURL),
		strings.NewReader(form.Encode()))
	if err != nil {
		return "", 0, fmt.Errorf("failed to build the token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := l.do(req)
	if err != nil {
		return "", 0, fmt.Errorf("the token request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("the token endpoint answered %d", resp.StatusCode)
	}
	var issued struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&issued); err != nil {
		return "", 0, fmt.Errorf("the token endpoint answered with something other than a token: %w", err)
	}
	if issued.AccessToken == "" {
		return "", 0, fmt.Errorf("the token endpoint granted no access token")
	}
	lifetime := defaultTokenLifetime
	if issued.ExpiresIn > 0 {
		lifetime = time.Duration(issued.ExpiresIn) * time.Second
	}
	return issued.AccessToken, lifetime, nil
}
