// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package outboundauth

import (
	"errors"
	"fmt"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
)

// ErrUnsupportedType reports a configuration naming an authentication method this build does
// not implement. It is the caller's mistake rather than a server fault, so a transport mapping
// a ToProperties failure onto a status code must tell it apart from a genuine failure to build
// a property, which is not the caller's doing.
var ErrUnsupportedType = errors.New("unsupported authentication type")

// PropertyKeyPrefix namespaces every property this package owns inside a consumer's flat
// property bag, keeping an authentication field distinct from a transport property of the same
// name. The prefix is deliberately longer than "auth_": the Twilio sender already stores a
// transport secret called "auth_token", which a shorter prefix would silently claim.
const PropertyKeyPrefix = "authentication_"

// PropertyKeyType is the property key holding the authentication type discriminator. It is
// written even for TypeNone, so a reader can tell "authenticates nothing" from "was configured
// before outbound authentication existed".
const PropertyKeyType = PropertyKeyPrefix + "type"

// PropertyKey returns the property key a field's value is stored under, for example
// "authentication_password" for the basic method's password field.
func PropertyKey(fieldName string) string {
	return PropertyKeyPrefix + fieldName
}

// OwnsPropertyKey reports whether name is a property key this package owns. Consumers use it to
// skip authentication properties in their own property loops, so a parser that warns about an
// unrecognized property does not warn about a field it has no reason to know.
func OwnsPropertyKey(name string) bool {
	return strings.HasPrefix(name, PropertyKeyPrefix)
}

// ToProperties renders cfg for a consumer's property bag. The registered method decides which
// values are secret, so cmodels.NewProperty encrypts them on construction. A field with a blank
// value is skipped, matching how the sender services treat a missing property.
//
// A field the method does not declare is dropped rather than stored, which guarantees nothing
// can reach storage unencrypted by being named something the registry does not know. That makes
// an undeclared property silently ignored rather than rejected, consistent with how the rest of
// the API treats unknown request fields.
func ToProperties(cfg Config) ([]cmodels.Property, error) {
	authType := cfg.Type
	if authType == "" {
		authType = TypeNone
	}

	method, ok := GetMethod(authType)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedType, cfg.Type)
	}

	props := make([]cmodels.Property, 0, len(method.Fields)+1)
	typeProp, err := cmodels.NewProperty(PropertyKeyType, string(authType), false)
	if err != nil {
		return nil, fmt.Errorf("failed to build property %s: %w", PropertyKeyType, err)
	}
	props = append(props, *typeProp)

	for _, field := range method.Fields {
		// A secret is stored exactly as given, since trimming could silently alter a valid
		// value. Everything else is trimmed, matching how the sender validators read values.
		value := cfg.Get(field.Name)
		if !field.Credential {
			value = strings.TrimSpace(value)
		}
		if value == "" {
			continue
		}
		prop, err := cmodels.NewProperty(PropertyKey(field.Name), value, field.Credential)
		if err != nil {
			return nil, fmt.Errorf("failed to build property %s: %w", PropertyKey(field.Name), err)
		}
		props = append(props, *prop)
	}

	return props, nil
}

// FromProperties reads a configuration back out of a property bag, ignoring every key this
// package does not own. Secret values are decrypted, so the returned Config carries plaintext
// and must not reach a response unmasked; see FromValues for the masked path.
func FromProperties(props []cmodels.Property) (Config, error) {
	values := make(map[string]string, len(props))
	for i := range props {
		prop := &props[i]
		if !OwnsPropertyKey(prop.GetName()) {
			continue
		}
		value, err := prop.GetValue()
		if err != nil {
			return Config{}, fmt.Errorf("failed to read property %s: %w", prop.GetName(), err)
		}
		values[prop.GetName()] = value
	}

	return FromValues(values), nil
}

// FromValues builds a Config from an already-resolved property-key to value map. Callers that
// have masked their secrets, such as the connection read API, pass the masked map here rather
// than decrypting again. Only keys this package owns, and only fields the resolved type
// declares, are read.
func FromValues(values map[string]string) Config {
	authType, ok := ParseType(values[PropertyKeyType])
	if !ok {
		// An unrecognized stored type is surfaced verbatim so Validate can reject it with a
		// descriptive message rather than silently downgrading to "none".
		return Config{Type: Type(values[PropertyKeyType]), Properties: map[string]string{}}
	}

	cfg := Config{Type: authType, Properties: map[string]string{}}
	method, ok := GetMethod(authType)
	if !ok {
		return cfg
	}

	for _, field := range method.Fields {
		if value, exists := values[PropertyKey(field.Name)]; exists {
			cfg.Properties[field.Name] = value
		}
	}

	return cfg
}
