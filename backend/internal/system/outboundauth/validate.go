// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package outboundauth

import (
	"fmt"
	"slices"
	"strings"
)

// Validate checks cfg against the registered method for its type and the set of types the caller
// supports. It enforces exactly what the registry declares: a known and permitted type, a
// non-blank value for every required field, and no field the method does not declare.
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
	if !slices.Contains(supported, authType) {
		return fmt.Errorf("authentication type is not supported by this provider: %s", authType)
	}

	for name := range cfg.Properties {
		if !slices.ContainsFunc(method.Fields, func(field Field) bool { return field.Key == name }) {
			return fmt.Errorf("unknown authentication property for type %s: %s", authType, name)
		}
	}

	for _, field := range method.Fields {
		if field.Required && strings.TrimSpace(cfg.Get(field.Key)) == "" {
			return fmt.Errorf("authentication property %s is required for type %s",
				field.Key, authType)
		}
	}

	return nil
}
