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
	"strings"

	"github.com/thunder-id/thunderid/internal/system/log"
)

// JSON Schema type constants.
const (
	// TypeString represents the string type in JSON Schema.
	TypeString = "string"
	// TypeNumber represents the number type in JSON Schema.
	TypeNumber = "number"
	// TypeBoolean represents the boolean type in JSON Schema.
	TypeBoolean = "boolean"
	// TypeObject represents the object type in JSON Schema.
	TypeObject = "object"
	// TypeArray represents the array type in JSON Schema.
	TypeArray = "array"
)

type property interface {
	isRequired() bool
	isCredential() bool
	isDisplayable() bool
	isUnique() bool
	getDisplayName() string
	validateValue(value interface{}, path string, logger *log.Logger) (bool, error)
	validateUniqueness(value interface{}, path string,
		exists func(map[string]interface{}) (bool, error), logger *log.Logger) (bool, error)
}

// Schema represents an entity type schema with a set of properties.
type Schema struct {
	properties map[string]property
}

// getPropertyByPath returns the property at the given dot-notation path
// (e.g. "address.city") by walking through nested object types. For a simple
// (non-dotted) name, it returns the top-level property directly.
func (cs *Schema) getPropertyByPath(path string) (property, bool) {
	segments := strings.Split(path, ".")
	currentProps := cs.properties

	for i, segment := range segments {
		prop, exists := currentProps[segment]
		if !exists {
			return nil, false
		}

		if i == len(segments)-1 {
			return prop, true
		}

		obj, ok := prop.(*object)
		if !ok {
			return nil, false
		}
		currentProps = obj.properties
	}

	return nil, false
}

// DisplayAttributeStatus represents the result of validating an attribute as a display attribute.
type DisplayAttributeStatus int

const (
	// DisplayAttributeValid indicates the attribute is valid for use as a display attribute.
	DisplayAttributeValid DisplayAttributeStatus = iota
	// DisplayAttributeNotFound indicates the attribute does not exist in the schema.
	DisplayAttributeNotFound
	// DisplayAttributeNotDisplayable indicates the attribute type is not displayable.
	DisplayAttributeNotDisplayable
	// DisplayAttributeIsCredential indicates the attribute is marked as a credential.
	DisplayAttributeIsCredential
)

// ValidateAsDisplayAttribute resolves the path once and checks existence, displayability,
// and credential status in a single pass.
func (cs *Schema) ValidateAsDisplayAttribute(name string) DisplayAttributeStatus {
	prop, exists := cs.getPropertyByPath(name)
	if !exists {
		return DisplayAttributeNotFound
	}
	if !prop.isDisplayable() {
		return DisplayAttributeNotDisplayable
	}
	if prop.isCredential() {
		return DisplayAttributeIsCredential
	}
	return DisplayAttributeValid
}

// AttributeInfo holds an attribute name, its required and credential status, and its human-readable
// display label. DisplayName may be empty when the schema definition omits the `displayName` field;
// callers should fall back to Attribute when rendering a label.
type AttributeInfo struct {
	Attribute   string
	DisplayName string
	Required    bool
	Credential  bool
}

// GetAttributes returns top-level properties filtered by the provided flags.
// allowCredential includes credential properties; allowNonCredential includes non-credential
// properties. When requiredOnly is true, only required properties are included.
func (cs *Schema) GetAttributes(allowCredential, allowNonCredential, requiredOnly bool) []AttributeInfo {
	result := make([]AttributeInfo, 0, len(cs.properties))
	for attr, prop := range cs.properties {
		isCredential := prop.isCredential()
		if isCredential && !allowCredential {
			continue
		}
		if !isCredential && !allowNonCredential {
			continue
		}
		if requiredOnly && !prop.isRequired() {
			continue
		}
		result = append(result, AttributeInfo{
			Attribute:   attr,
			DisplayName: prop.getDisplayName(),
			Required:    prop.isRequired(),
			Credential:  isCredential,
		})
	}
	return result
}

// GetUniqueAttributes returns the names of top-level properties marked as unique.
func (cs *Schema) GetUniqueAttributes() []string {
	var fields []string
	for name, prop := range cs.properties {
		if prop.isUnique() {
			fields = append(fields, name)
		}
	}

	return fields
}

// Validate validates the user attributes against the schema.
// When skipCredentialRequired is true, missing credential properties do not fail
// the required check. This is used during updates where credentials are not
// included in the payload.
func (cs *Schema) Validate(attributes json.RawMessage, logger *log.Logger, skipCredentialRequired bool) (bool, error) {
	if len(attributes) == 0 {
		logger.Debug("User has no attributes to validate")
		return true, nil
	}

	var userAttrs map[string]interface{}
	if err := json.Unmarshal(attributes, &userAttrs); err != nil {
		return false, fmt.Errorf("failed to unmarshal user attributes: %w", err)
	}

	if len(cs.properties) == 0 {
		return true, nil
	}

	for propName, prop := range cs.properties {
		value, exists := userAttrs[propName]
		if !exists {
			if prop.isRequired() && !(skipCredentialRequired && prop.isCredential()) {
				return false, nil
			}
			continue
		}

		isValid, err := prop.validateValue(value, propName, logger)
		if err != nil {
			return false, err
		}
		if !isValid {
			return false, nil
		}
	}

	// Reject any user attributes not declared in the schema.
	for key := range userAttrs {
		if _, declared := cs.properties[key]; !declared {
			logger.Debug("Attribute not defined in schema", log.String("attribute", key))
			return false, nil
		}
	}

	return true, nil
}

// ValidateUniqueness checks uniqueness constraints for the schema properties.
func (cs *Schema) ValidateUniqueness(
	attrs map[string]interface{},
	exists func(map[string]interface{}) (bool, error),
	logger *log.Logger,
) (bool, error) {
	if len(cs.properties) == 0 {
		return true, nil
	}

	for propName, prop := range cs.properties {
		value, ok := attrs[propName]
		if !ok {
			continue
		}

		isValid, err := prop.validateUniqueness(value, propName, exists, logger)
		if err != nil {
			return false, err
		}
		if !isValid {
			return false, nil
		}
	}

	return true, nil
}

func convertToFloat64(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	default:
		return 0, false
	}
}

// CompileSchema compiles an entity type JSON Schema from the provided raw JSON.
func CompileSchema(schema json.RawMessage) (*Schema, error) {
	var schemaMap map[string]json.RawMessage
	if err := json.Unmarshal(schema, &schemaMap); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	if len(schemaMap) == 0 {
		return nil, fmt.Errorf("schema cannot be empty - must contain at least one property definition")
	}

	compiled := &Schema{
		properties: make(map[string]property, len(schemaMap)),
	}

	for propName, propRaw := range schemaMap {
		compiledProp, err := compileProperty(propName, propRaw)
		if err != nil {
			return nil, fmt.Errorf("invalid property '%s': %w", propName, err)
		}
		compiled.properties[propName] = compiledProp
	}

	return compiled, nil
}

func compileProperty(propName string, propRaw json.RawMessage) (property, error) {
	var propMap map[string]json.RawMessage
	if err := json.Unmarshal(propRaw, &propMap); err != nil {
		return nil, fmt.Errorf("property definition must be an object")
	}

	typeRaw, exists := propMap["type"]
	if !exists {
		return nil, fmt.Errorf("missing required 'type' field")
	}

	var typeStr string
	if err := json.Unmarshal(typeRaw, &typeStr); err != nil {
		return nil, fmt.Errorf("'type' field must be a string")
	}

	switch typeStr {
	case TypeString:
		return compileStringProperty(propMap)
	case TypeNumber:
		return compileNumberProperty(propMap)
	case TypeBoolean:
		return compileBooleanProperty(propMap)
	case TypeObject:
		return compileObjectProperty(propMap)
	case TypeArray:
		return compileArrayProperty(propName, propMap)
	default:
		return nil, fmt.Errorf("invalid type '%s', must be one of: string, number, boolean, object, array", typeStr)
	}
}
