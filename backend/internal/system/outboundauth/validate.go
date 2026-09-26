// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package outboundauth

import (
	"fmt"
	"regexp"
	"strings"
)

// Validate checks cfg against the registered method for its type and the set of types the caller
// supports. It enforces exactly what the registry declares: a known and permitted type, a
// non-blank value for every required field, values within any declared enum or pattern, and no
// field the method does not declare. It then runs the method's own rules, if it has any.
//
// Policy that depends on the transport, such as SMTP refusing to send credentials over a
// plaintext connection, belongs to the caller.
func Validate(cfg Config, supported []Type) error {
	authType := cfg.Type
	if authType == "" {
		authType = TypeNone
	}

	method, ok := GetMethod(authType)
	if !ok {
		return fmt.Errorf("unsupported authentication type: %s", cfg.Type)
	}
	if !containsType(supported, authType) {
		return fmt.Errorf("authentication type is not supported by this provider: %s", authType)
	}

	for name := range cfg.Properties {
		if _, ok := getField(authType, name); !ok {
			return fmt.Errorf("unknown authentication property for type %s: %s", authType, name)
		}
	}

	for _, field := range method.Fields {
		if err := validateField(field, cfg.Get(field.Name), authType); err != nil {
			return err
		}
	}

	if method.Validate != nil {
		return method.Validate(cfg)
	}

	return nil
}

// validateField checks one field's value against its declaration.
func validateField(field Field, value string, authType Type) error {
	if strings.TrimSpace(value) == "" {
		if field.Required {
			return fmt.Errorf("authentication property %s is required for type %s",
				field.Name, authType)
		}
		return nil
	}

	if len(field.Enum) > 0 && !containsString(field.Enum, value) {
		return fmt.Errorf("authentication property %s must be one of %s, got: %s",
			field.Name, strings.Join(field.Enum, ", "), value)
	}

	if field.Regex != "" {
		matched, err := regexp.MatchString(field.Regex, value)
		if err != nil {
			return fmt.Errorf("failed to validate authentication property %s: %w",
				field.Name, err)
		}
		if !matched {
			return fmt.Errorf("authentication property %s has an invalid format", field.Name)
		}
	}

	return nil
}

// containsType reports whether types contains authType.
func containsType(types []Type, authType Type) bool {
	for _, candidate := range types {
		if candidate == authType {
			return true
		}
	}
	return false
}

// containsString reports whether values contains value.
func containsString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
