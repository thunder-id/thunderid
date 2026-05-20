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

package model

import (
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

func (p *boolean) isUnique() bool {
	return false
}

func (p *boolean) validateValue(value interface{}, path string, logger *log.Logger) (bool, error) {
	_, ok := value.(bool)
	if !ok {
		logger.Debug("Expected boolean but got different type",
			log.String("property", path), log.String("value", fmt.Sprintf("%v", value)))
		return false, nil
	}
	return true, nil
}

func (p *boolean) validateUniqueness(
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
