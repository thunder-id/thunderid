// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/log"
)

// boolean represents a boolean property in the entity type schema.
type boolean struct {
	required    bool
	displayName string
}

func (p *boolean) isRequired() bool {
	return p.required
}

func (p *boolean) isCredential() bool {
	return false
}

func (p *boolean) isDisplayable() bool {
	return false
}

func (p *boolean) getDisplayName() string {
	return p.displayName
}

// getEnum returns no values: boolean properties do not constrain their value to a fixed set.
func (p *boolean) getEnum() []string {
	return nil
}

func (p *boolean) isUnique() bool {
	return false
}

func (p *boolean) getType() string {
	return TypeBoolean
}

func (p *boolean) validateValue(ctx context.Context, value interface{}, path string, logger *log.Logger) (bool, error) {
	_, ok := value.(bool)
	if !ok {
		logger.Debug(ctx, "Expected boolean but got different type",
			log.String("property", path), log.String("value", fmt.Sprintf("%v", value)))
		return false, nil
	}
	return true, nil
}

func (p *boolean) validateUniqueness(ctx context.Context,
	value interface{},
	path string,
	exists func(map[string]interface{}) (bool, error),
	logger *log.Logger,
) (bool, error) {
	return true, nil
}

func compileBooleanProperty(propMap map[string]json.RawMessage) (property, error) {
	allowedFields := map[string]struct{}{
		"type":        {},
		"required":    {},
		"displayName": {},
	}

	for field := range propMap {
		if _, ok := allowedFields[field]; !ok {
			return nil, fmt.Errorf("invalid field '%s' for leaf property", field)
		}
	}

	prop := &boolean{}

	if raw, exists := propMap["required"]; exists {
		if err := json.Unmarshal(raw, &prop.required); err != nil {
			return nil, fmt.Errorf("'required' field must be a boolean")
		}
	}

	if raw, exists := propMap["displayName"]; exists {
		if err := json.Unmarshal(raw, &prop.displayName); err != nil {
			return nil, fmt.Errorf("'displayName' field must be a string")
		}
	}

	return prop, nil
}
