/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package model provides data structures for entity type attribute types and validation.
package model

import (
	"encoding/json"
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/log"
)

type array struct {
	required    bool
	displayName string
	items       property
}

func (p *array) isRequired() bool {
	return p.required
}

func (p *array) isCredential() bool {
	return false
}

func (p *array) isDisplayable() bool {
	return false
}

func (p *array) getDisplayName() string {
	return p.displayName
}

func (p *array) isUnique() bool {
	return false
}

func (p *array) validateValue(value interface{}, path string, logger *log.Logger) (bool, error) {
	arrayValue, ok := value.([]interface{})
	if !ok {
		logger.Debug("Expected array but got different type",
			log.String("property", path), log.String("value", fmt.Sprintf("%v", value)))
		return false, nil
	}

	if p.required && len(arrayValue) == 0 {
		logger.Debug("Array property is required but empty", log.String("property", path))
		return false, nil
	}

	if p.items == nil {
		return true, nil
	}

	for index, item := range arrayValue {
		itemPath := fmt.Sprintf("%s[%d]", path, index)
		isValid, err := p.items.validateValue(item, itemPath, logger)
		if err != nil {
			return false, err
		}
		if !isValid {
			return false, nil
		}
	}

	return true, nil
}

func (p *array) validateUniqueness(
	value interface{},
	path string,
	exists func(map[string]interface{}) (bool, error),
	logger *log.Logger,
) (bool, error) {
	// Arrays are not supported for uniqueness validation
	return true, nil
}

func compileArrayProperty(propName string, propMap map[string]json.RawMessage) (property, error) {
	allowedFields := map[string]struct{}{
		"type":        {},
		"items":       {},
		"required":    {},
		"displayName": {},
	}

	for field := range propMap {
		if _, ok := allowedFields[field]; !ok {
			return nil, fmt.Errorf("invalid field '%s' for array property", field)
		}
	}

	prop := &array{}

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

	itemsRaw, exists := propMap["items"]
	if !exists {
		return nil, fmt.Errorf("missing required 'items' field for array type")
	}

	compiledItems, err := compileProperty(propName, itemsRaw)
	if err != nil {
		return nil, fmt.Errorf("invalid 'items' definition: %w", err)
	}

	prop.items = compiledItems
	return prop, nil
}
