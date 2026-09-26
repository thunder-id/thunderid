// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package outboundauthn provides authentication strategies for outbound HTTP requests.
package outboundauthn

import (
	"fmt"
	"net/http"
	"strings"
)

const (
	// SchemeNone sends no authentication headers.
	SchemeNone = "NONE"
	// SchemeBearer sends an OAuth bearer token in the Authorization header.
	SchemeBearer = "BEARER"
	// SchemeAPIKey sends an API key in a configured header.
	SchemeAPIKey = "API_KEY"
)

// Config configures authentication for an outbound HTTP connection.
type Config struct {
	Scheme        string
	BearerToken   string
	APIKeyHeaders map[string]string
}

// APIKeyHeader represents one API-key HTTP header.
type APIKeyHeader struct {
	Name  string `json:"name" yaml:"name"`
	Value string `json:"value,omitempty" yaml:"value,omitempty"`
}

// ValidateAPIKeyHeader validates and canonicalizes an API-key HTTP header.
func ValidateAPIKeyHeader(name, value string) (string, error) {
	name = http.CanonicalHeaderKey(strings.TrimSpace(name))
	if !isValidHeaderName(name) {
		return "", fmt.Errorf("invalid API key header name")
	}
	if isReservedHeaderName(name) {
		return "", fmt.Errorf("API key header %q is reserved", name)
	}
	if strings.TrimSpace(value) == "" || strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("invalid API key header value")
	}
	return name, nil
}

// RequestAuthenticator applies configured authentication to an HTTP request.
type RequestAuthenticator interface {
	ApplyAuthentication(*http.Request)
}

type requestAuthenticator struct {
	scheme        string
	bearerToken   string
	apiKeyHeaders map[string]string
}

// NewRequestAuthenticator creates an outbound HTTP request authenticator.
func NewRequestAuthenticator(config Config) (RequestAuthenticator, error) {
	configuredScheme := strings.ToUpper(strings.TrimSpace(config.Scheme))
	if configuredScheme != "" &&
		configuredScheme != SchemeNone && configuredScheme != SchemeBearer && configuredScheme != SchemeAPIKey {
		return nil, fmt.Errorf("unsupported outbound authentication scheme %q", config.Scheme)
	}
	scheme := normalizeScheme(config.Scheme, config.BearerToken, config.APIKeyHeaders)
	if scheme == SchemeBearer && strings.TrimSpace(config.BearerToken) == "" {
		return nil, fmt.Errorf("bearer token is required")
	}
	if scheme == SchemeAPIKey && len(config.APIKeyHeaders) == 0 {
		return nil, fmt.Errorf("at least one API key header is required")
	}
	headers := make(map[string]string, len(config.APIKeyHeaders))
	for name, value := range config.APIKeyHeaders {
		canonicalName, err := ValidateAPIKeyHeader(name, value)
		if err != nil {
			return nil, err
		}
		headers[canonicalName] = value
	}

	return &requestAuthenticator{
		scheme:        scheme,
		bearerToken:   config.BearerToken,
		apiKeyHeaders: headers,
	}, nil
}

// ApplyAuthentication adds the configured authentication headers to req.
func (a *requestAuthenticator) ApplyAuthentication(req *http.Request) {
	if req == nil {
		return
	}

	switch a.scheme {
	case SchemeBearer:
		if a.bearerToken != "" {
			req.Header.Set("Authorization", "Bearer "+a.bearerToken)
		}
	case SchemeAPIKey:
		for name, value := range a.apiKeyHeaders {
			if value != "" {
				req.Header.Set(name, value)
			}
		}
	}
}

func normalizeScheme(value, bearerToken string, apiKeyHeaders map[string]string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case SchemeNone, SchemeBearer, SchemeAPIKey:
		return strings.ToUpper(strings.TrimSpace(value))
	default:
		if strings.TrimSpace(bearerToken) != "" {
			return SchemeBearer
		}
		if len(apiKeyHeaders) > 0 {
			return SchemeAPIKey
		}
		return SchemeNone
	}
}

func isValidHeaderName(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		character := name[i]
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			strings.ContainsRune("!#$%&'*+-.^_`|~", rune(character)) {
			continue
		}
		return false
	}
	return true
}

func isReservedHeaderName(name string) bool {
	switch name {
	case "Content-Type", "Content-Length", "Host":
		return true
	default:
		return false
	}
}
