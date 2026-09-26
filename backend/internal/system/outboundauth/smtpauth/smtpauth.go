// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package smtpauth binds an outbound authentication configuration to the SASL mechanism
// net/smtp expects. It is the SMTP half of the outboundauth seam: outboundauth itself stays
// transport-neutral, and nothing here is reachable from another transport.
package smtpauth

import (
	"context"
	"fmt"
	"net/smtp"

	"github.com/thunder-id/thunderid/internal/system/outboundauth"
)

// Authenticator produces the SASL mechanism for an SMTP session.
type Authenticator interface {
	// Auth returns the mechanism to present to the server, or a nil smtp.Auth when the
	// configuration authenticates nothing, so a caller can hand the result straight to
	// smtp.Client.Auth after a nil check. host is the server name the credentials are bound to.
	Auth(ctx context.Context, host string) (smtp.Auth, error)
}

// basicAuthenticator presents SASL PLAIN.
type basicAuthenticator struct {
	username string
	password string
}

// noopAuthenticator presents nothing.
type noopAuthenticator struct{}

// binding pairs a method with the Authenticator this transport builds for it.
type binding struct {
	authType outboundauth.Type
	build    func(cfg outboundauth.Config) Authenticator
}

// bindings is the single source for both New and SupportedTypes, so the set a caller validates
// against cannot drift from the set New can actually build. It is a slice rather than a map
// because the order is the order a console offers the methods in.
var bindings = []binding{
	{
		authType: outboundauth.TypeNone,
		build: func(outboundauth.Config) Authenticator {
			return &noopAuthenticator{}
		},
	},
	{
		authType: outboundauth.TypeBasic,
		build: func(cfg outboundauth.Config) Authenticator {
			return &basicAuthenticator{
				username: cfg.Get(outboundauth.FieldBasicUsername),
				password: cfg.Get(outboundauth.FieldBasicPassword),
			}
		},
	},
}

// unbindable lists the registered methods SMTP deliberately cannot carry, against the reason it
// cannot. Together with bindings it has to account for every registered method, which is what
// makes an unconsidered new method fail this package's test rather than a send months later.
var unbindable = map[outboundauth.Type]string{}

// SupportedTypes returns the methods this transport can carry, in the order a console should
// offer them. Callers hand it to outboundauth.Validate so a configuration naming a method SMTP
// cannot carry is refused when it is configured rather than when mail is sent.
func SupportedTypes() []outboundauth.Type {
	types := make([]outboundauth.Type, 0, len(bindings))
	for _, b := range bindings {
		types = append(types, b.authType)
	}
	return types
}

// New returns the Authenticator for cfg. It is built once per client, so a method that has to
// acquire and cache a token has somewhere to keep it. An error means cfg names a method SMTP
// cannot carry, which validation should already have refused.
func New(cfg outboundauth.Config) (Authenticator, error) {
	authType := cfg.Type
	if authType == "" {
		authType = outboundauth.TypeNone
	}

	for _, b := range bindings {
		if b.authType == authType {
			return b.build(cfg), nil
		}
	}

	return nil, fmt.Errorf("authentication type is not supported over SMTP: %s", authType)
}

// Auth returns the PLAIN mechanism bound to host.
func (a *basicAuthenticator) Auth(_ context.Context, host string) (smtp.Auth, error) {
	return smtp.PlainAuth("", a.username, a.password, host), nil
}

// Auth returns a nil mechanism, signaling that no credentials are presented.
func (a *noopAuthenticator) Auth(_ context.Context, _ string) (smtp.Auth, error) {
	return nil, nil
}
