// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/thunder-id/thunderid/internal/system/log"
)

type str struct {
	required    bool
	unique      bool
	credential  bool
	displayName string
	enum        map[string]struct{}
	enumOrder   []string
	pattern     *regexp.Regexp
}

func (p *str) isUnique() bool {
	return p.unique
}

func (p *str) getType() string {
	return TypeString
}

func (p *str) isRequired() bool {
	return p.required
}

func (p *str) isCredential() bool {
	return p.credential
}

func (p *str) isDisplayable() bool {
	return true
}

func (p *str) getDisplayName() string {
	return p.displayName
}

// getEnum returns the permitted values in the order the schema declared them.
func (p *str) getEnum() []string {
	return p.enumOrder
}

func (p *str) validateValue(ctx context.Context, value interface{}, path string, logger *log.Logger) (bool, error) {
	strValue, ok := value.(string)
	if !ok {
		logger.Debug(ctx, "Expected string but got different type",
			log.String("property", path), log.String("value", fmt.Sprintf("%v", value)))
		return false, nil
	}

	if p.enum != nil {
		if _, exists := p.enum[strValue]; !exists {
			logger.Debug(ctx, "Value not in enum",
				log.String("property", path), log.String("value", strValue))
			return false, nil
		}
	}

	if p.pattern != nil && !p.pattern.MatchString(strValue) {
		logger.Debug(ctx, "Regex pattern mismatch",
			log.String("property", path), log.String("value", strValue))
		return false, nil
	}

	return true, nil
}

func (p *str) validateUniqueness(ctx context.Context,
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

func compileStringProperty(propMap map[string]json.RawMessage) (property, error) {
	allowedFields := map[string]struct{}{
		"type":        {},
		"required":    {},
		"unique":      {},
		"credential":  {},
		"displayName": {},
		"enum":        {},
		"regex":       {},
		"pattern":     {},
	}

	for field := range propMap {
		if _, ok := allowedFields[field]; !ok {
			return nil, fmt.Errorf("invalid field '%s' for leaf property", field)
		}
	}

	prop := &str{}

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

		prop.enum = make(map[string]struct{}, len(enumRaw))
		prop.enumOrder = make([]string, 0, len(enumRaw))
		for i, itemRaw := range enumRaw {
			var value string
			if err := json.Unmarshal(itemRaw, &value); err != nil {
				return nil, fmt.Errorf("'enum' array item at index %d must be a string to match property type", i)
			}
			if _, seen := prop.enum[value]; seen {
				continue
			}
			prop.enum[value] = struct{}{}
			prop.enumOrder = append(prop.enumOrder, value)
		}
	}

	pattern, err := compilePattern(propMap)
	if err != nil {
		return nil, err
	}
	prop.pattern = pattern

	return prop, nil
}

func compilePattern(propMap map[string]json.RawMessage) (*regexp.Regexp, error) {
	var patternStr string
	if raw, exists := propMap["regex"]; exists {
		if _, dup := propMap["pattern"]; dup {
			return nil, fmt.Errorf("cannot define both 'regex' and 'pattern' fields for the same property")
		}
		if err := json.Unmarshal(raw, &patternStr); err != nil {
			return nil, fmt.Errorf("'regex' field must be a string")
		}
	} else if raw, exists := propMap["pattern"]; exists {
		if err := json.Unmarshal(raw, &patternStr); err != nil {
			return nil, fmt.Errorf("'pattern' field must be a string")
		}
	}

	if patternStr == "" {
		return nil, nil
	}

	compiled, err := regexp.Compile(patternStr)
	if err != nil {
		return nil, fmt.Errorf("failed to compile regex pattern: %w", err)
	}

	return compiled, nil
}
