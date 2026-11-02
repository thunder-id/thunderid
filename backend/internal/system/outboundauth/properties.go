// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package outboundauth

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
)

// propertyKeyPrefix namespaces every property this package owns inside a consumer's flat
// property bag, keeping an authentication field distinct from a transport property of the same
// name.
const propertyKeyPrefix = "authentication_"

// PropertyKeyType is the property key holding the authentication type discriminator. It is
// written even for TypeNone, so a reader can tell "authenticates nothing" from "was configured
// before outbound authentication existed".
const PropertyKeyType = propertyKeyPrefix + "type"

const maskedCredential = "******"

// ErrAuthenticationRequiredForTargetChange means stored credentials cannot be reused for a new destination.
var ErrAuthenticationRequiredForTargetChange = errors.New("fresh authentication is required when the target changes")

// propertyKey returns the property key a field's value is stored under, for example
// "authentication_password" for the basic method's password field.
func propertyKey(fieldName string) string {
	return propertyKeyPrefix + fieldName
}

// OwnsPropertyKey reports whether name is a property key this package owns. Consumers use it to
// skip authentication properties in their own property loops, so a parser that warns about an
// unrecognized property does not warn about a field it has no reason to know.
func OwnsPropertyKey(name string) bool {
	return strings.HasPrefix(name, propertyKeyPrefix)
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
		return nil, fmt.Errorf("%w: %s", errUnsupportedType, cfg.Type)
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
		value := cfg.Get(field.Key)
		if !field.Credential {
			value = strings.TrimSpace(value)
		}
		if value == "" {
			continue
		}
		prop, err := cmodels.NewProperty(propertyKey(field.Key), value, field.Credential)
		if err != nil {
			return nil, fmt.Errorf("failed to build property %s: %w", propertyKey(field.Key), err)
		}
		props = append(props, *prop)
	}
	if method.AdditionalProperties != nil {
		for name, value := range cfg.Properties {
			prop, err := cmodels.NewProperty(propertyKey(name), value, method.AdditionalProperties.Credential)
			if err != nil {
				return nil, fmt.Errorf("failed to build property %s: %w", propertyKey(name), err)
			}
			props = append(props, *prop)
		}
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
		if value, exists := values[propertyKey(field.Key)]; exists {
			cfg.Properties[field.Key] = value
		}
	}
	if method.AdditionalProperties != nil {
		for name, value := range values {
			if name == PropertyKeyType || !OwnsPropertyKey(name) {
				continue
			}
			cfg.Properties[strings.TrimPrefix(name, propertyKeyPrefix)] = value
		}
	}

	return cfg
}

// UpdateProperties resolves an authentication update against stored credentials. A nil request
// retains authentication only while the destination is unchanged. Secret values omitted from a
// selected method, or sent back masked, are retained only for the same method and identity.
// The caller owns the connection's storage and supplies the fields identifying its destination.
func UpdateProperties(request *Authentication, stored []cmodels.Property, previousTarget, nextTarget []string,
	supported []Type, normalizeUpdate, normalize func(Config) (Config, error),
) ([]cmodels.Property, error) {
	current, err := FromProperties(stored)
	if err != nil {
		return nil, err
	}
	targetUnchanged := slices.Equal(previousTarget, nextTarget)
	if request == nil {
		if !targetUnchanged && current.Enabled() {
			return nil, ErrAuthenticationRequiredForTargetChange
		}
		if len(stored) > 0 {
			return slices.Clone(stored), nil
		}
		return ToProperties(Config{Type: TypeNone})
	}

	config := request.Config()
	if normalizeUpdate != nil && config.Type == TypeAPIKey {
		config, err = normalizeUpdate(config)
		if err != nil {
			return nil, err
		}
	}
	if config.Properties == nil {
		config.Properties = make(map[string]string)
	}
	if err := retainStoredCredentials(&config, current, targetUnchanged); err != nil {
		return nil, err
	}
	if normalize != nil {
		config, err = normalize(config)
		if err != nil {
			return nil, err
		}
	}
	if err := Validate(config, supported); err != nil {
		return nil, err
	}
	return ToProperties(config)
}

// AuthenticationFromProperties returns a response with every registered credential masked.
// A secret is masked even if an older stored property was not marked secret.
func AuthenticationFromProperties(properties []cmodels.Property) (Authentication, error) {
	values := make(map[string]string, len(properties))
	for _, property := range properties {
		if !OwnsPropertyKey(property.GetName()) {
			continue
		}
		if property.GetName() == PropertyKeyType {
			value, err := property.GetValue()
			if err != nil {
				return Authentication{}, err
			}
			values[PropertyKeyType] = value
		}
	}
	config := FromValues(values)
	method, known := GetMethod(config.Type)
	for _, property := range properties {
		name := property.GetName()
		if !OwnsPropertyKey(name) || name == PropertyKeyType {
			continue
		}
		fieldName := name[len(propertyKeyPrefix):]
		secret := property.IsSecret() || !known
		if known {
			for _, field := range method.Fields {
				if field.Key == fieldName && field.Credential {
					secret = true
				}
			}
			if method.AdditionalProperties != nil && method.AdditionalProperties.Credential {
				secret = true
			}
		}
		if secret {
			values[name] = maskedCredential
			continue
		}
		value, err := property.GetValue()
		if err != nil {
			return Authentication{}, err
		}
		values[name] = value
	}
	return AuthenticationFromValues(values), nil
}

// retainStoredCredentials merges omitted or masked values only when the destination and identity match.
func retainStoredCredentials(config *Config, current Config, targetUnchanged bool) error {
	method, ok := GetMethod(config.Type)
	if !ok {
		return nil
	}
	sameIdentity := targetUnchanged && config.Type == current.Type
	for _, field := range method.Fields {
		if !field.Credential && config.Get(field.Key) != current.Get(field.Key) {
			sameIdentity = false
		}
	}
	for _, field := range method.Fields {
		if !field.Credential {
			continue
		}
		value := config.Get(field.Key)
		if value != "" && value != maskedCredential {
			continue
		}
		if sameIdentity && current.Get(field.Key) != "" {
			config.Properties[field.Key] = current.Get(field.Key)
		} else if value == maskedCredential {
			return cannotRetainCredential(field.Key, targetUnchanged, current.Enabled())
		}
	}
	if method.AdditionalProperties != nil {
		for name, value := range config.Properties {
			if value != maskedCredential {
				continue
			}
			if !sameIdentity || current.Get(name) == "" {
				return cannotRetainCredential(name, targetUnchanged, current.Enabled())
			}
			config.Properties[name] = current.Get(name)
		}
	}
	return nil
}

func cannotRetainCredential(name string, targetUnchanged, currentEnabled bool) error {
	if targetUnchanged || !currentEnabled {
		return fmt.Errorf("cannot retain authentication property %q after its identity changed", name)
	}
	return fmt.Errorf("%w: cannot retain authentication property %q",
		ErrAuthenticationRequiredForTargetChange, name)
}
