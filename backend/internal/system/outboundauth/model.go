// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package outboundauth models how ThunderID authenticates itself on an outbound call, over
// SMTP, HTTP, or any other transport it dials out on. It is the outbound counterpart to
// internal/system/security, which authenticates inbound callers of ThunderID's own API.
//
// The package owns the configuration model and stays transport-neutral; how credentials reach
// the wire differs per transport, so each binding lives in its own leaf subpackage that imports
// this package and nothing else. Binding shapes differ on purpose: smtpauth returns an
// smtp.Auth, while an HTTP binding would mutate a request and a client-certificate one would
// return a tls.Certificate. A new transport brings its own signature rather than being forced
// through a common one.
//
// Outbound transport hardening that is not authentication, such as the trust store, TLS minimum
// version, egress proxy and SSRF policy, stays out of this package.
//
// The method registry is the single source of truth. A registered method decides the property
// key a value is stored under, whether it is required, whether it is a secret, and how a console
// renders it, so adding a method means adding a registry entry rather than touching every caller.
//
// Field values are strings. A method needing a list encodes it in a single space-delimited
// value, as OAuth scopes already are.
package outboundauth

import "strings"

// Type is the authentication method used on an outbound call, carried on the wire as the
// authentication.type discriminator.
type Type string

const (
	// TypeNone sends no credentials.
	TypeNone Type = "none"
	// TypeBasic sends a username and a password (HTTP Basic, SMTP PLAIN, and so on).
	TypeBasic Type = "basic"
)

// FieldTypeString is the console-facing value type of every credential field today. It matches
// the "type" discriminator of the property definition schema the console already renders.
const FieldTypeString = "string"

// Field names of the built-in methods. They are the keys inside authentication.properties on
// the wire and, prefixed by PropertyKeyPrefix, the keys a value is stored under.
const (
	// FieldBasicUsername is the basic method's username field.
	FieldBasicUsername = "username"
	// FieldBasicPassword is the basic method's password field.
	FieldBasicPassword = "password"
)

// Field describes one credential field of an authentication method. It is the single source of
// truth for that field: the property key it is stored under, whether a value is required,
// whether the value is a secret, and how a console renders it. The JSON tags speak the console's
// property definition vocabulary so a metadata endpoint can serve registered methods verbatim.
type Field struct {
	// Name is the field's key inside authentication.properties.
	Name string `json:"name"`
	// Type is the console-facing value type; FieldTypeString today.
	Type string `json:"type"`
	// Required reports whether the field must carry a non-blank value.
	Required bool `json:"required,omitempty"`
	// Credential marks a secret field: stored through cmodels.NewProperty with isSecret set,
	// masked on read, and preserved on update when the request omits it. The JSON name matches
	// the console's own credential flag.
	Credential bool `json:"credential,omitempty"`
	// Enum, when set, restricts the value to one of the listed choices. Nothing uses it yet;
	// it exists because a method that offers a choice, such as where an API key is placed,
	// cannot be described without it.
	Enum []string `json:"enum,omitempty"`
	// Regex, when set, is the pattern a value must match. It is compiled by the validator and
	// sent verbatim to the console, which compiles it too.
	Regex string `json:"regex,omitempty"`
	// DisplayName is the console label, either plain text or an i18n template pattern of the
	// form "{{t(namespace:key)}}" that the console resolves.
	DisplayName string `json:"displayName"`
}

// Method describes one authentication method end to end: its discriminator and its credential
// fields in the order a console should render them.
type Method struct {
	// Type is the discriminator carried as authentication.type.
	Type Type `json:"type"`
	// DisplayName is the console label for the method itself.
	DisplayName string `json:"displayName"`
	// Fields are the method's credential fields, in render order. Empty for TypeNone.
	Fields []Field `json:"properties"`
	// Validate, when set, checks rules internal to this method that the fields cannot express,
	// such as requiring exactly one of two alternative credentials. Rules that depend on the
	// transport belong to the caller instead.
	Validate func(cfg Config) error `json:"-"`
}

// Config is a resolved outbound authentication configuration: a method plus its field values
// keyed by Field.Name. A zero Config authenticates nothing.
type Config struct {
	Type       Type
	Properties map[string]string
}

// Enabled reports whether c calls for credentials to be sent at all.
func (c Config) Enabled() bool {
	return c.Type != "" && c.Type != TypeNone
}

// Get returns the value of the named field, or the empty string when it is absent.
func (c Config) Get(name string) string {
	return c.Properties[name]
}

// ParseType returns the Type named by value, normalizing case and surrounding whitespace. An
// empty value yields TypeNone. The second return reports whether value named a registered method
// at all; whether that method is permitted in a given context is Validate's business.
//
// The registry is consulted rather than a list of its own, so a method added there is parsable
// without a second edit here.
func ParseType(value string) (Type, bool) {
	authType := Type(strings.ToLower(strings.TrimSpace(value)))
	if authType == "" {
		authType = TypeNone
	}
	if _, ok := GetMethod(authType); !ok {
		return "", false
	}
	return authType, true
}
