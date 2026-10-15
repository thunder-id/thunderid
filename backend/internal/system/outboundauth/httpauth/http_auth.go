// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package httpauth authenticates an outbound HTTP request with an outbound authentication
// configuration. A caller builds an Authenticator and calls Authenticate on the request's headers;
// the scheme is chosen and presented here, and nothing here is reachable from another transport.
package httpauth

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
)

// Authenticator authenticates an outbound HTTP request by setting its headers.
type Authenticator = outboundauth.Authenticator[http.Header]

// basicAuthenticator presents HTTP Basic credentials (RFC 7617).
type basicAuthenticator struct {
	username string
	password string
}

// noopAuthenticator presents nothing.
type noopAuthenticator struct{}

// bindings is the single source for both New and SupportedTypes.
var bindings = outboundauth.Bindings[http.Header]{
	{
		Type: outboundauth.TypeNone,
		New: func(outboundauth.Config) outboundauth.Authenticator[http.Header] {
			return &noopAuthenticator{}
		},
	},
	{
		Type: outboundauth.TypeBasic,
		New: func(authConfig outboundauth.Config) outboundauth.Authenticator[http.Header] {
			return &basicAuthenticator{
				username: authConfig.Get(outboundauth.FieldBasicUsername),
				password: authConfig.Get(outboundauth.FieldBasicPassword),
			}
		},
	},
}

// SupportedTypes returns the methods HTTP can carry, in binding order.
func SupportedTypes() []outboundauth.Type {
	return bindings.SupportedTypes()
}

// New returns the Authenticator for authConfig.
func New(authConfig outboundauth.Config) (Authenticator, error) {
	authenticator, err := bindings.New(authConfig)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	return authenticator, nil
}

// Authenticate sets the Authorization header, replacing any value already present.
func (a *basicAuthenticator) Authenticate(_ context.Context, header http.Header) error {
	credentials := base64.StdEncoding.EncodeToString([]byte(a.username + ":" + a.password))
	header.Set(serverconst.AuthorizationHeaderName, serverconst.AuthSchemeBasic+credentials)
	return nil
}

// Authenticate presents nothing.
func (a *noopAuthenticator) Authenticate(_ context.Context, _ http.Header) error {
	return nil
}
