// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/log"
)

type number struct {
	required    bool
	unique      bool
	credential  bool
	displayName string
	enum        map[float64]struct{}
}

func (p *number) isUnique() bool {
	return p.unique
}

func (p *number) getType() string {
	return TypeNumber
}

func (p *number) isRequired() bool {
	return p.required
}

func (p *number) isCredential() bool {
	return p.credential
}

func (p *number) isDisplayable() bool {
	return true
}

func (p *number) getDisplayName() string {
	return p.displayName
}

// getEnum returns no values: number properties do not constrain their value to a fixed set.
func (p *number) getEnum() []string {
	return nil
}

func (p *number) validateValue(ctx context.Context, value interface{}, path string, logger *log.Logger) (bool, error) {
	numberValue, ok := convertToFloat64(value)
	if !ok {
		logger.Debug(ctx, "Expected number but got different type",
			log.String("property", path), log.String("value", fmt.Sprintf("%v", value)))
		return false, nil
	}

	if p.enum != nil {
		if _, exists := p.enum[numberValue]; !exists {
			logger.Debug(ctx, "Value not in enum", log.String("property", path),
				log.String("value", fmt.Sprintf("%v", value)))
			return false, nil
		}
	}

	return true, nil
}

func (p *number) validateUniqueness(ctx context.Context,
	value interface{},
	path string,
	exists func(map[string]interface{}) (bool, error),
	logger *log.Logger,
) (bool, error) {
	if !p.unique {
		return true, nil
	}

	found, err := exists(map[string]interface{}{path: value})
	if err != nil {
		return false, err
	}

	return !found, nil
}

func compileNumberProperty(propMap map[string]json.RawMessage) (property, error) {
	allowedFields := map[string]struct{}{
		"type":        {},
		"required":    {},
		"unique":      {},
		"credential":  {},
		"displayName": {},
		"enum":        {},
	}

	for field := range propMap {
		if _, ok := allowedFields[field]; !ok {
			return nil, fmt.Errorf("invalid field '%s' for leaf property", field)
		}
	}

	prop := &number{}

	if raw, exists := propMap["required"]; exists {
		if err := json.Unmarshal(raw, &prop.required); err != nil {
			return nil, fmt.Errorf("'required' field must be a boolean")
		}
	}

	if raw, exists := propMap["unique"]; exists {
		if err := json.Unmarshal(raw, &prop.unique); err != nil {
			return nil, fmt.Errorf("'unique' field must be a boolean")
		}
	}

	if raw, exists := propMap["credential"]; exists {
		if err := json.Unmarshal(raw, &prop.credential); err != nil {
			return nil, fmt.Errorf("'credential' field must be a boolean")
		}
	}

	if raw, exists := propMap["displayName"]; exists {
		if err := json.Unmarshal(raw, &prop.displayName); err != nil {
			return nil, fmt.Errorf("'displayName' field must be a string")
		}
	}

	if raw, exists := propMap["enum"]; exists {
		var enumRaw []json.RawMessage
		if err := json.Unmarshal(raw, &enumRaw); err != nil {
			return nil, fmt.Errorf("'enum' field must be an array")
		}
		if len(enumRaw) == 0 {
			return nil, fmt.Errorf("'enum' array cannot be empty")
		}

		prop.enum = make(map[float64]struct{}, len(enumRaw))
		for i, itemRaw := range enumRaw {
			var value float64
			if err := json.Unmarshal(itemRaw, &value); err != nil {
				return nil, fmt.Errorf("'enum' array item at index %d must be a number to match property type", i)
			}
			prop.enum[value] = struct{}{}
		}
	}

	return prop, nil
}
