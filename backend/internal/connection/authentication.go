// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
)

// connectionAuthentication is the nested authentication object on a connection payload: a
// discriminator plus a property bag whose keys come from the registered method. Keeping the bag
// generic is what lets a new authentication method reach the console without a change here or a
// new schema. On a response the secret values are already masked.
type connectionAuthentication struct {
	Type       string            `json:"type"`
	Properties map[string]string `json:"properties,omitempty"`
}

// authConfigFromRequest maps the wire object onto an outboundauth configuration. A nil object
// means "authenticates nothing", which is what an omitted block and the console's None
// selection both produce.
func authConfigFromRequest(auth *connectionAuthentication) outboundauth.Config {
	if auth == nil {
		return outboundauth.Config{Type: outboundauth.TypeNone}
	}

	// An unrecognized type is passed through rather than normalized, so the sender service
	// rejects it with a descriptive error instead of silently storing "none".
	authType := outboundauth.Type(auth.Type)
	if parsed, ok := outboundauth.ParseType(auth.Type); ok {
		authType = parsed
	}

	properties := make(map[string]string, len(auth.Properties))
	for name, value := range auth.Properties {
		properties[name] = value
	}

	return outboundauth.Config{Type: authType, Properties: properties}
}

// authenticationFromValues rebuilds the wire object from an already-masked property-value map.
// It reads the map propertyValues produced rather than the properties themselves, so a secret
// cannot be read back in plain text on this path.
func authenticationFromValues(values map[string]string) connectionAuthentication {
	cfg := outboundauth.FromValues(values)

	authType := cfg.Type
	if authType == "" {
		authType = outboundauth.TypeNone
	}

	return connectionAuthentication{
		Type:       string(authType),
		Properties: cfg.Properties,
	}
}

// authenticationProperties renders the authentication block of a request into sender
// properties. The registered method decides which values are secret, so the password is
// encrypted on construction and masked on every later read.
func authenticationProperties(auth *connectionAuthentication) ([]cmodels.Property, error) {
	return outboundauth.ToProperties(authConfigFromRequest(auth))
}

// authenticationFromExportModel maps a declarative document's authentication block onto the
// request shape, so an imported document and an API payload take the same path.
func authenticationFromExportModel(auth *connectionAuthenticationExportModel) *connectionAuthentication {
	if auth == nil {
		return nil
	}
	return &connectionAuthentication{Type: auth.Type, Properties: auth.Properties}
}
