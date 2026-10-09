// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package httpauth applies outbound authentication to HTTP requests.
package httpauth

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/outboundauth"
)

// Authenticator applies credentials to an HTTP request.
type Authenticator = outboundauth.Authenticator[*http.Request]

type noAuthentication struct{}
type bearerAuthentication struct{ token string }
type basicAuthentication struct{ username, password string }
type apiKeyAuthentication struct{ headers map[string]string }

var bindings = outboundauth.Bindings[*http.Request]{
	{Type: outboundauth.TypeNone, New: func(outboundauth.Config) Authenticator { return noAuthentication{} }},
	{Type: outboundauth.TypeBearer, New: func(config outboundauth.Config) Authenticator {
		return bearerAuthentication{token: config.Get(outboundauth.FieldBearerToken)}
	}},
	{Type: outboundauth.TypeBasic, New: func(config outboundauth.Config) Authenticator {
		return basicAuthentication{
			username: config.Get(outboundauth.FieldBasicUsername),
			password: config.Get(outboundauth.FieldBasicPassword),
		}
	}},
	{Type: outboundauth.TypeAPIKey, New: func(config outboundauth.Config) Authenticator {
		return apiKeyAuthentication{headers: config.Properties}
	}},
}

// SupportedTypes reports which registered methods HTTP can carry.
func SupportedTypes() []outboundauth.Type { return bindings.SupportedTypes() }

// Canonicalize validates cfg and normalizes API-key header names.
func Canonicalize(cfg outboundauth.Config) (outboundauth.Config, error) {
	return canonicalize(cfg, false)
}

// CanonicalizeForUpdate permits masked API-key values that will be replaced with stored values.
func CanonicalizeForUpdate(cfg outboundauth.Config) (outboundauth.Config, error) {
	return canonicalize(cfg, true)
}

func canonicalize(cfg outboundauth.Config, allowMasked bool) (outboundauth.Config, error) {
	if err := outboundauth.Validate(cfg, SupportedTypes()); err != nil {
		return outboundauth.Config{}, err
	}
	if cfg.Type != outboundauth.TypeAPIKey {
		return cfg, nil
	}
	headers := make(map[string]string, len(cfg.Properties))
	for name, value := range cfg.Properties {
		valueToValidate := value
		if allowMasked && value == "******" {
			valueToValidate = "stored-value"
		}
		canonical, err := ValidateAPIKeyHeader(name, valueToValidate)
		if err != nil {
			return outboundauth.Config{}, err
		}
		if _, found := headers[canonical]; found {
			return outboundauth.Config{}, fmt.Errorf("duplicate API-key header %q", canonical)
		}
		headers[canonical] = value
	}
	cfg.Properties = headers
	return cfg, nil
}

// ValidateAPIKeyHeader validates a configured HTTP header and returns its canonical name.
func ValidateAPIKeyHeader(name, value string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("API-key header name and value are required")
	}
	if value == "******" {
		return "", fmt.Errorf("masked API-key header value cannot be stored")
	}
	for _, char := range name {
		if !(char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z' ||
			char >= '0' && char <= '9' || strings.ContainsRune("!#$%&'*+-.^_`|~", char)) {
			return "", fmt.Errorf("invalid API-key header name %q", name)
		}
	}
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("API-key header value contains CR or LF")
	}
	canonical := http.CanonicalHeaderKey(name)
	switch strings.ToLower(canonical) {
	case "content-type", "content-length", "host", "transfer-encoding", "connection",
		"te", "trailer", "upgrade", "proxy-connection", "keep-alive", "proxy-authorization":
		return "", fmt.Errorf("API-key header %q is reserved", canonical)
	}
	return canonical, nil
}

// New builds an HTTP authenticator from a validated configuration.
func New(cfg outboundauth.Config) (Authenticator, error) {
	canonical, err := Canonicalize(cfg)
	if err != nil {
		return nil, err
	}
	return bindings.New(canonical)
}

func (noAuthentication) Authenticate(_ context.Context, _ *http.Request) error { return nil }

func (a bearerAuthentication) Authenticate(_ context.Context, request *http.Request) error {
	if request == nil {
		return fmt.Errorf("HTTP request is required")
	}
	request.Header.Set("Authorization", "Bearer "+a.token)
	return nil
}

func (a basicAuthentication) Authenticate(_ context.Context, request *http.Request) error {
	if request == nil {
		return fmt.Errorf("HTTP request is required")
	}
	request.SetBasicAuth(a.username, a.password)
	return nil
}

func (a apiKeyAuthentication) Authenticate(_ context.Context, request *http.Request) error {
	if request == nil {
		return fmt.Errorf("HTTP request is required")
	}
	for name, value := range a.headers {
		request.Header.Set(name, value)
	}
	return nil
}
