// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package smtpauth authenticates an SMTP session with an outbound authentication configuration.
// A caller builds an Authenticator once and calls Authenticate on the session; the SASL mechanism
// is chosen and presented here, and nothing here is reachable from another transport.
package smtpauth

import (
	"context"
	"fmt"
	"net/smtp"

	"github.com/thunder-id/thunderid/internal/system/outboundauth"
)

// Session is the part of an SMTP session an Authenticator needs. *smtp.Client satisfies it.
type Session interface {
	// Auth runs the AUTH exchange with a.
	Auth(a smtp.Auth) error
}

// Authenticator authenticates an SMTP session.
type Authenticator = outboundauth.Authenticator[Session]

// target is what a binding authenticates: the session plus the server name the credentials are
// bound to. *smtp.Client does not expose the name it was created with, so it travels alongside.
type target struct {
	session Session
	host    string
}

// basicAuthenticator presents SASL PLAIN.
type basicAuthenticator struct {
	username string
	password string
}

// noopAuthenticator presents nothing.
type noopAuthenticator struct{}

// hostBoundAuthenticator binds a host to the authenticator a binding built, so that the
// exported contract takes the session alone.
type hostBoundAuthenticator struct {
	inner outboundauth.Authenticator[target]
	host  string
}

// bindings is the single source for both New and SupportedTypes.
var bindings = outboundauth.Bindings[target]{
	{
		Type: outboundauth.TypeNone,
		New: func(outboundauth.Config) outboundauth.Authenticator[target] {
			return &noopAuthenticator{}
		},
	},
	{
		Type: outboundauth.TypeBasic,
		New: func(authConfig outboundauth.Config) outboundauth.Authenticator[target] {
			return &basicAuthenticator{
				username: authConfig.Get(outboundauth.FieldBasicUsername),
				password: authConfig.Get(outboundauth.FieldBasicPassword),
			}
		},
	},
}

// SupportedTypes returns the methods SMTP can carry, in binding order.
func SupportedTypes() []outboundauth.Type {
	return bindings.SupportedTypes()
}

// New returns the Authenticator for cfg, with its credentials bound to host. It is built with
// its client, and a client is built for every send, so an Authenticator must not hold state that
// has to outlive one send.
func New(authConfig outboundauth.Config, host string) (Authenticator, error) {
	inner, err := bindings.New(authConfig)
	if err != nil {
		return nil, fmt.Errorf("smtp: %w", err)
	}
	return &hostBoundAuthenticator{inner: inner, host: host}, nil
}

// Authenticate authenticates session with the bound host.
func (a *hostBoundAuthenticator) Authenticate(ctx context.Context, session Session) error {
	return a.inner.Authenticate(ctx, target{session: session, host: a.host})
}

// Authenticate presents the PLAIN mechanism bound to the target host. PlainAuth itself refuses
// to send the credentials to a server of another name or over a plaintext connection.
func (a *basicAuthenticator) Authenticate(_ context.Context, authTarget target) error {
	return authTarget.session.Auth(smtp.PlainAuth("", a.username, a.password, authTarget.host))
}

// Authenticate presents nothing.
func (a *noopAuthenticator) Authenticate(_ context.Context, _ target) error {
	return nil
}
