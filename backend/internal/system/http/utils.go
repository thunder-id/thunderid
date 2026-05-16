/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package http

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/config"
)

// GetTLSVersion returns the appropriate TLS version constant based on the provided
// configuration. It defaults to TLS 1.3 if the configured version is not recognized
// or empty.
func GetTLSVersion(config config.Config) uint16 {
	var minTLSVersion uint16
	switch config.TLS.MinVersion {
	case "1.2":
		minTLSVersion = tls.VersionTLS12
	case "1.3":
		minTLSVersion = tls.VersionTLS13
	default:
		minTLSVersion = tls.VersionTLS13 // Default to TLS 1.3 for better security
	}
	return minTLSVersion
}

// CanonicalizeURL normalizes an absolute URL for spec-compliant comparison
// (RFC 3986 §6): lowercase scheme/host, elide default ports, resolve "."/".."
// segments without collapsing repeated slashes, and normalize percent-encoding.
// Query and fragment are dropped.
func CanonicalizeURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	if !u.IsAbs() {
		return "", errors.New("URL must be absolute")
	}
	if u.Host == "" {
		return "", errors.New("URL missing host")
	}

	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		port = ""
	}

	p := u.EscapedPath()
	if p == "" {
		p = "/"
	}
	p = removeDotSegments(p)
	p = normalizePercentEncoding(p)

	hostPort := host
	if port != "" {
		hostPort = host + ":" + port
	}
	return scheme + "://" + hostPort + p, nil
}

// removeDotSegments removes only "." and ".." segments while preserving repeated
// slashes (path.Clean would collapse "/a//b" to "/a/b", altering URI semantics).
func removeDotSegments(p string) string {
	if p == "" {
		return "/"
	}
	var output strings.Builder
	output.Grow(len(p))
	input := p
	for len(input) > 0 {
		switch {
		case strings.HasPrefix(input, "../"):
			input = input[3:]
		case strings.HasPrefix(input, "./"):
			input = input[2:]
		case strings.HasPrefix(input, "/./"):
			input = input[2:]
		case input == "/.":
			input = "/"
		case strings.HasPrefix(input, "/../"):
			input = input[3:]
			cur := output.String()
			output.Reset()
			if i := strings.LastIndexByte(cur, '/'); i >= 0 {
				output.WriteString(cur[:i])
			}
		case input == "/..":
			input = "/"
			cur := output.String()
			output.Reset()
			if i := strings.LastIndexByte(cur, '/'); i >= 0 {
				output.WriteString(cur[:i])
			}
		case input == "." || input == "..":
			input = ""
		default:
			start := 0
			if input[0] == '/' {
				start = 1
			}
			next := strings.IndexByte(input[start:], '/')
			if next == -1 {
				output.WriteString(input)
				input = ""
			} else {
				output.WriteString(input[:start+next])
				input = input[start+next:]
			}
		}
	}
	result := output.String()
	if !strings.HasPrefix(result, "/") {
		result = "/" + result
	}
	return result
}

func normalizePercentEncoding(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != '%' || i+2 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		hi, ok1 := fromHex(s[i+1])
		lo, ok2 := fromHex(s[i+2])
		if !ok1 || !ok2 {
			b.WriteByte(s[i])
			continue
		}
		decoded := hi<<4 | lo
		if isUnreserved(decoded) {
			b.WriteByte(decoded)
		} else {
			b.WriteByte('%')
			b.WriteByte(upperHex(s[i+1]))
			b.WriteByte(upperHex(s[i+2]))
		}
		i += 2
	}
	return b.String()
}

func isUnreserved(c byte) bool {
	switch {
	case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		return true
	case c == '-', c == '.', c == '_', c == '~':
		return true
	}
	return false
}

func fromHex(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

func upperHex(c byte) byte {
	if c >= 'a' && c <= 'f' {
		return c - 32
	}
	return c
}
