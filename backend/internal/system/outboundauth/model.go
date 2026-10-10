// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package outboundauth models how ThunderID authenticates itself on an outbound call. It owns
// the transport-neutral configuration model and the Authenticator contract every transport
// implements; each transport presents the credentials in its own subpackage, so a caller only
// ever calls Authenticate on its target.
package outboundauth

import (
	"strings"
)

// Type is the authentication method used on an outbound call, carried on the wire as the
// authentication.type discriminator.
type Type string

const (
	// TypeNone sends no credentials.
	TypeNone Type = "none"
	// TypeBasic sends a username and a password, presented as SASL PLAIN over SMTP.
	TypeBasic Type = "basic"
)

// FieldType is the value type of a credential field.
type FieldType string

// fieldTypeText is the value type of a free-text field.
const fieldTypeText FieldType = "string"

// Field names of the built-in methods. They are the keys inside authentication.properties on
// the wire and, prefixed by propertyKeyPrefix, the keys a value is stored under.
const (
	// FieldBasicUsername is the basic method's username field.
	FieldBasicUsername = "username"
	// FieldBasicPassword is the basic method's password field.
	FieldBasicPassword = "password"
)

// Field describes one credential field of an authentication method.
type Field struct {
	// Key is the field's key inside authentication.properties.
	Key string `json:"key"`
	// Type is the field's value type.
	Type FieldType `json:"type"`
	// Required reports whether the field must carry a non-blank value.
	Required bool `json:"required,omitempty"`
	// Credential marks a secret field: stored encrypted, masked on read, and preserved on update
	// when the request omits it.
	Credential bool `json:"credential,omitempty"`
	// DisplayName is the field label, either plain text or an i18n template of the form
	// "{{t(namespace:key)}}".
	DisplayName string `json:"displayName"`
}

// Method describes one authentication method: its discriminator and its credential fields.
type Method struct {
	// Type is the discriminator carried as authentication.type.
	Type Type `json:"type"`
	// DisplayName is the method label.
	DisplayName string `json:"displayName"`
	// Fields are the method's credential fields, in display order. Empty for TypeNone.
	Fields []Field `json:"fields"`
}

// Config is a resolved outbound authentication configuration: a method plus its field values
// keyed by Field.Key. A zero Config authenticates nothing.
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
