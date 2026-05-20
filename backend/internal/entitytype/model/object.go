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

type object struct {
	required    bool
	displayName string
	properties  map[string]property
}

func (p *object) isRequired() bool {
	return p.required
}

func (p *object) isCredential() bool {
	return false
}

func (p *object) isDisplayable() bool {
	return false
}

func (p *object) getDisplayName() string {
	return p.displayName
}

func (p *object) isUnique() bool {
	return false
}

func (p *object) validateValue(value interface{}, path string, logger *log.Logger) (bool, error) {
	valueMap, ok := value.(map[string]interface{})
	if !ok {
		logger.Debug("Expected object but got different type",
			log.String("property", path), log.String("value", fmt.Sprintf("%v", value)))
		return false, nil
	}

	for nestedName, nestedProp := range p.properties {
		nestedValue, exists := valueMap[nestedName]
		nestedPath := path + "." + nestedName

		if !exists {
			if nestedProp.isRequired() {
				return false, nil
			}
			continue
		}

		if nestedValue == nil {
			continue
		}

		isValid, err := nestedProp.validateValue(nestedValue, nestedPath, logger)
		if err != nil {
			return false, err
		}
		if !isValid {
			return false, nil
		}
	}

	// Reject any nested keys not declared in the object schema.
	for key := range valueMap {
		if _, declared := p.properties[key]; !declared {
			logger.Debug("Attribute not defined in schema",
				log.String("attribute", path+"."+key))
			return false, nil
		}
	}

	return true, nil
}

func (p *object) validateUniqueness(
	value interface{},
	path string,
	exists func(map[string]interface{}) (bool, error),
	logger *log.Logger,
) (bool, error) {
	valueMap, ok := value.(map[string]interface{})
	if !ok {
		logger.Debug("Expected object but got different type",
			log.String("property", path), log.String("value", fmt.Sprintf("%v", value)))
		return false, nil
	}

	for nestedName, nestedProp := range p.properties {
		nestedValue, ok := valueMap[nestedName]
		if !ok {
			continue
		}

		nestedPath := path + "." + nestedName
		isValid, err := nestedProp.validateUniqueness(nestedValue, nestedPath, exists, logger)
		if err != nil {
			return false, err
		}
		if !isValid {
			return false, nil
		}
	}

	return true, nil
}

func compileObjectProperty(propMap map[string]json.RawMessage) (property, error) {
	allowedFields := map[string]struct{}{
		"type":        {},
		"properties":  {},
		"required":    {},
		"displayName": {},
	}

	for field := range propMap {
		if _, ok := allowedFields[field]; !ok {
			return nil, fmt.Errorf("invalid field '%s' for object property", field)
		}
	}

	prop := &object{
		properties: make(map[string]property),
	}

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

	propertiesRaw, exists := propMap["properties"]
	if !exists {
		return nil, fmt.Errorf("missing required 'properties' field for object type")
	}

	var nestedProps map[string]json.RawMessage
	if err := json.Unmarshal(propertiesRaw, &nestedProps); err != nil {
		return nil, fmt.Errorf("'properties' field must be an object")
	}

	for nestedName, nestedRaw := range nestedProps {
		compiledNested, err := compileProperty(nestedName, nestedRaw)
		if err != nil {
			return nil, fmt.Errorf("invalid nested property '%s': %w", nestedName, err)
		}
		prop.properties[nestedName] = compiledNested
	}

	return prop, nil
}
