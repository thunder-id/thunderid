// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
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
	getType() string
	getDisplayName() string
	validateValue(ctx context.Context, value interface{}, path string, logger *log.Logger) (bool, error)
	validateUniqueness(ctx context.Context, value interface{}, path string,
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
	Type        string
	Required    bool
	Credential  bool
	Unique      bool
}

// AttributeFilter selects top-level schema properties by their characteristics. Credential and
// non-credential properties are included independently via AllowCredential / AllowNonCredential.
// RequiredOnly and UniqueOnly further restrict the result to required / unique properties, and Type
// (when non-empty) restricts to properties of that type (e.g. "string").
type AttributeFilter struct {
	AllowCredential    bool
	AllowNonCredential bool
	RequiredOnly       bool
	UniqueOnly         bool
	Type               string
}

// GetAttributes returns top-level properties matching the given filter.
func (cs *Schema) GetAttributes(filter AttributeFilter) []AttributeInfo {
	result := make([]AttributeInfo, 0, len(cs.properties))
	for attr, prop := range cs.properties {
		isCredential := prop.isCredential()
		if isCredential && !filter.AllowCredential {
			continue
		}
		if !isCredential && !filter.AllowNonCredential {
			continue
		}
		if filter.RequiredOnly && !prop.isRequired() {
			continue
		}
		if filter.UniqueOnly && !prop.isUnique() {
			continue
		}
		if filter.Type != "" && prop.getType() != filter.Type {
			continue
		}
		result = append(result, AttributeInfo{
			Attribute:   attr,
			DisplayName: prop.getDisplayName(),
			Type:        prop.getType(),
			Required:    prop.isRequired(),
			Credential:  isCredential,
			Unique:      prop.isUnique(),
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
func (cs *Schema) Validate(
	ctx context.Context, attributes json.RawMessage, logger *log.Logger, skipCredentialRequired bool) (bool, error) {
	if len(attributes) == 0 {
		logger.Debug(ctx, "User has no attributes to validate")
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

		isValid, err := prop.validateValue(ctx, value, propName, logger)
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
			logger.Debug(ctx, "Attribute not defined in schema", log.String("attribute", key))
			return false, nil
		}
	}

	return true, nil
}

// ValidateUniqueness checks uniqueness constraints for the schema properties.
func (cs *Schema) ValidateUniqueness(
	ctx context.Context,
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

		isValid, err := prop.validateUniqueness(ctx, value, propName, exists, logger)
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

// propertyNamePattern is the SCIM 2.0 attribute-name production (RFC 7643 section 2.1):
// a letter, followed by any number of letters, digits, '$', '-' and '_'.
//
// '.' is excluded deliberately. Filter keys built from a schema join nested property names with
// '.', and the query layer splits them back on '.', so a literal dot inside a name cannot be told
// apart from traversal into a nested object.
var propertyNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9$_-]*$`)

// validatePropertyName reports whether name may be used as a schema property name. Property names
// are machine names; use the 'displayName' field for a human-readable label.
func validatePropertyName(name string) error {
	if !propertyNamePattern.MatchString(name) {
		return fmt.Errorf("invalid property name '%s': must start with a letter and contain only "+
			"letters, digits, '-', '_' and '$'", name)
	}
	return nil
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
		if err := validatePropertyName(propName); err != nil {
			return nil, err
		}
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
