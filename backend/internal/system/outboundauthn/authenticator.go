// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package outboundauthn provides authentication strategies for outbound HTTP requests.
package outboundauthn

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
)

const (
	// MaskedSecretValue is the placeholder returned for stored credential values.
	MaskedSecretValue = "******"
	// SchemeNone sends no authentication headers.
	SchemeNone = "NONE"
	// SchemeBearer sends an OAuth bearer token in the Authorization header.
	SchemeBearer = "BEARER"
	// SchemeBasic sends HTTP Basic credentials in the Authorization header.
	SchemeBasic = "BASIC"
	// SchemeAPIKey sends an API key in a configured header.
	SchemeAPIKey = "API_KEY"
)

// Config configures authentication for an outbound HTTP connection.
type Config struct {
	Scheme        string
	BearerToken   string
	BasicUsername string
	BasicPassword string
	APIKeyHeaders map[string]string
}

// Authentication configures one supported outbound HTTP authentication scheme.
type Authentication struct {
	Scheme string             `json:"scheme" yaml:"scheme"`
	Bearer *BearerCredentials `json:"bearer,omitempty" yaml:"bearer,omitempty"`
	Basic  *BasicCredentials  `json:"basic,omitempty" yaml:"basic,omitempty"`
	APIKey *APIKeyCredentials `json:"apiKey,omitempty" yaml:"apiKey,omitempty"`
}

// BearerCredentials configures an OAuth bearer token.
type BearerCredentials struct {
	Token string `json:"token" yaml:"token"`
}

// BasicCredentials configures HTTP Basic credentials.
type BasicCredentials struct {
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
}

// APIKeyCredentials configures API-key HTTP headers.
type APIKeyCredentials struct {
	Headers []APIKeyHeader `json:"headers" yaml:"headers"`
}

// APIKeyHeader represents one API-key HTTP header.
type APIKeyHeader struct {
	Name  string `json:"name" yaml:"name"`
	Value string `json:"value,omitempty" yaml:"value,omitempty"`
}

// ConfigFromAuthentication validates an authentication request and converts it to runtime configuration.
func ConfigFromAuthentication(authentication *Authentication) (Config, error) {
	if authentication == nil {
		return Config{Scheme: SchemeNone}, nil
	}
	scheme := strings.ToUpper(strings.TrimSpace(authentication.Scheme))
	if scheme == "" {
		scheme = SchemeNone
	}
	config := Config{Scheme: scheme}
	switch scheme {
	case SchemeNone:
		if authentication.Bearer != nil || authentication.Basic != nil || authentication.APIKey != nil {
			return Config{}, fmt.Errorf("only the selected authentication credentials are allowed")
		}
	case SchemeBearer:
		if authentication.Bearer == nil || authentication.Basic != nil || authentication.APIKey != nil {
			return Config{}, fmt.Errorf("only the selected authentication credentials are allowed")
		}
		config.BearerToken = authentication.Bearer.Token
	case SchemeBasic:
		if authentication.Bearer != nil || authentication.Basic == nil || authentication.APIKey != nil {
			return Config{}, fmt.Errorf("only the selected authentication credentials are allowed")
		}
		config.BasicUsername = authentication.Basic.Username
		config.BasicPassword = authentication.Basic.Password
	case SchemeAPIKey:
		if authentication.Bearer != nil || authentication.Basic != nil || authentication.APIKey == nil {
			return Config{}, fmt.Errorf("only the selected authentication credentials are allowed")
		}
		headers, err := canonicalAPIKeyHeaders(authentication.APIKey.Headers, false)
		if err != nil {
			return Config{}, err
		}
		config.APIKeyHeaders = make(map[string]string, len(headers))
		for _, header := range headers {
			config.APIKeyHeaders[header.Name] = header.Value
		}
	default:
		return Config{}, fmt.Errorf("unsupported outbound authentication scheme %q", authentication.Scheme)
	}
	if _, err := NewRequestAuthenticator(config); err != nil {
		return Config{}, err
	}
	return config, nil
}

// NewAPIKeyHeadersProperty validates, canonicalizes, and encrypts API-key headers for storage.
func NewAPIKeyHeadersProperty(name string, headers []APIKeyHeader) (*cmodels.Property, error) {
	return newAPIKeyHeadersProperty(name, headers, false)
}

// NewAPIKeyHeadersPropertyForUpdate stores API-key headers that can include masked values.
// Call MergeAPIKeyHeaders before persisting the resulting property.
func NewAPIKeyHeadersPropertyForUpdate(name string, headers []APIKeyHeader) (*cmodels.Property, error) {
	return newAPIKeyHeadersProperty(name, headers, true)
}

func newAPIKeyHeadersProperty(name string, headers []APIKeyHeader, allowMaskedValues bool) (*cmodels.Property, error) {
	canonicalHeaders, err := canonicalAPIKeyHeaders(headers, allowMaskedValues)
	if err != nil {
		return nil, err
	}
	value, err := json.Marshal(canonicalHeaders)
	if err != nil {
		return nil, fmt.Errorf("failed to encode API key headers: %w", err)
	}
	property, err := cmodels.NewProperty(name, string(value), true)
	if err != nil {
		return nil, err
	}
	return property, nil
}

// APIKeyHeadersFromProperty decrypts stored API-key headers and optionally masks their values.
func APIKeyHeadersFromProperty(property cmodels.Property, maskValues bool) ([]APIKeyHeader, error) {
	value, err := property.GetValue()
	if err != nil {
		return nil, err
	}
	var headers []APIKeyHeader
	if err := json.Unmarshal([]byte(value), &headers); err != nil {
		return nil, fmt.Errorf("failed to decode API key headers: %w", err)
	}
	canonicalHeaders, err := canonicalAPIKeyHeaders(headers, true)
	if err != nil {
		return nil, err
	}
	if maskValues {
		return MaskAPIKeyHeaders(canonicalHeaders), nil
	}
	return canonicalHeaders, nil
}

// MaskAPIKeyHeaders returns API-key headers with credential values replaced by a mask.
func MaskAPIKeyHeaders(headers []APIKeyHeader) []APIKeyHeader {
	masked := make([]APIKeyHeader, len(headers))
	for index, header := range headers {
		masked[index] = APIKeyHeader{Name: header.Name, Value: MaskedSecretValue}
	}
	return masked
}

// MergeAPIKeyHeaders replaces incoming masked values with matching stored values.
func MergeAPIKeyHeaders(incoming, existing []APIKeyHeader) ([]APIKeyHeader, error) {
	if incoming == nil {
		return canonicalAPIKeyHeaders(existing, false)
	}
	existingHeaders, err := canonicalAPIKeyHeaders(existing, false)
	if err != nil {
		return nil, err
	}
	existingValues := make(map[string]string, len(existingHeaders))
	for _, header := range existingHeaders {
		existingValues[header.Name] = header.Value
	}
	merged := make([]APIKeyHeader, len(incoming))
	for index, header := range incoming {
		name := http.CanonicalHeaderKey(strings.TrimSpace(header.Name))
		if header.Value == MaskedSecretValue {
			value, found := existingValues[name]
			if !found {
				return nil, fmt.Errorf("stored API key header %q was not found", header.Name)
			}
			header.Value = value
		}
		merged[index] = header
	}
	return canonicalAPIKeyHeaders(merged, false)
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
	basicUsername string
	basicPassword string
	apiKeyHeaders map[string]string
}

// NewRequestAuthenticator creates an outbound HTTP request authenticator.
func NewRequestAuthenticator(config Config) (RequestAuthenticator, error) {
	configuredScheme := strings.ToUpper(strings.TrimSpace(config.Scheme))
	if configuredScheme != "" &&
		configuredScheme != SchemeNone && configuredScheme != SchemeBearer && configuredScheme != SchemeBasic &&
		configuredScheme != SchemeAPIKey {
		return nil, fmt.Errorf("unsupported outbound authentication scheme %q", config.Scheme)
	}
	scheme := normalizeScheme(config.Scheme, config.BearerToken, config.APIKeyHeaders)
	if scheme == SchemeBearer && strings.TrimSpace(config.BearerToken) == "" {
		return nil, fmt.Errorf("bearer token is required")
	}
	if scheme == SchemeBasic {
		if strings.TrimSpace(config.BasicUsername) == "" || strings.Contains(config.BasicUsername, ":") {
			return nil, fmt.Errorf("basic username is required and cannot contain a colon")
		}
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
		basicUsername: config.BasicUsername,
		basicPassword: config.BasicPassword,
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
	case SchemeBasic:
		req.SetBasicAuth(a.basicUsername, a.basicPassword)
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
	case SchemeNone, SchemeBearer, SchemeBasic, SchemeAPIKey:
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
	case "Authorization", "Content-Type", "Content-Length", "Host":
		return true
	default:
		return false
	}
}

func canonicalAPIKeyHeaders(headers []APIKeyHeader, allowMaskedValues bool) ([]APIKeyHeader, error) {
	canonicalHeaders := make([]APIKeyHeader, len(headers))
	seen := make(map[string]struct{}, len(headers))
	for index, header := range headers {
		if !allowMaskedValues && strings.TrimSpace(header.Value) == MaskedSecretValue {
			return nil, fmt.Errorf("masked API key header values are only allowed on update")
		}
		if allowMaskedValues && header.Value == MaskedSecretValue {
			header.Name = http.CanonicalHeaderKey(strings.TrimSpace(header.Name))
			if !isValidHeaderName(header.Name) || isReservedHeaderName(header.Name) {
				return nil, fmt.Errorf("invalid API key header name")
			}
		} else {
			name, err := ValidateAPIKeyHeader(header.Name, header.Value)
			if err != nil {
				return nil, err
			}
			header.Name = name
		}
		if _, found := seen[header.Name]; found {
			return nil, fmt.Errorf("duplicate API key header %q", header.Name)
		}
		seen[header.Name] = struct{}{}
		canonicalHeaders[index] = header
	}
	return canonicalHeaders, nil
}
