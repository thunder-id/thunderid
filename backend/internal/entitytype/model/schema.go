// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// JSON Schema type constants.
const (
	// TypeString represents the string type in JSON Schema.
	TypeString = "string"
	// TypeEmail represents an email contact.
	TypeEmail = "email"
	// TypeMobile represents a mobile contact.
	TypeMobile = "mobile"
	// TypeNumber represents the number type in JSON Schema.
	TypeNumber = "number"
	// TypeBoolean represents the boolean type in JSON Schema.
	TypeBoolean = "boolean"
	// TypeObject represents the object type in JSON Schema.
	TypeObject = "object"
	// TypeArray represents the array type in JSON Schema.
	TypeArray = "array"
)

// verificationClaimSuffix ends the name of the claim that carries a contact attribute's verification status.
const verificationClaimSuffix = "_verified"

type property interface {
	isRequired() bool
	isCredential() bool
	isDisplayable() bool
	isUnique() bool
	getType() string
	getDisplayName() string
	getEnum() []string
	validateValue(ctx context.Context, value interface{}, path string, logger *log.Logger) (bool, error)
	validateUniqueness(ctx context.Context, value interface{}, path string,
		exists func(map[string]interface{}) (bool, error), logger *log.Logger) (bool, error)
}

// Keys of the display settings a top-level property can set. They are read here rather than by
// each property type, so nested properties do not accept them.
const (
	keyDisplay  = "display"
	keyOrder    = "order"
	keyOnCreate = "onCreate"
	keyHidden   = "hidden"
)

// attributeHints holds a top-level property's display order and whether it is hidden on create.
type attributeHints struct {
	displayOrder   *int
	hiddenOnCreate bool
}

// Schema represents an entity type schema with a set of properties.
type Schema struct {
	properties map[string]property
	hints      map[string]attributeHints
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

// PrimaryContactStatus represents the result of validating an attribute as a primary contact.
type PrimaryContactStatus int

const (
	// PrimaryContactValid indicates the attribute is valid for use as a primary contact.
	PrimaryContactValid PrimaryContactStatus = iota
	// PrimaryContactNotFound indicates the attribute is not a top-level attribute in the schema.
	PrimaryContactNotFound
	// PrimaryContactTypeMismatch indicates the attribute is not of the expected contact type.
	PrimaryContactTypeMismatch
)

// ValidateAsPrimaryContact checks that the name references a top-level attribute of the contact type.
func (cs *Schema) ValidateAsPrimaryContact(name, contactType string) PrimaryContactStatus {
	prop, exists := cs.properties[name]
	if !exists {
		return PrimaryContactNotFound
	}
	if prop.getType() != contactType {
		return PrimaryContactTypeMismatch
	}
	return PrimaryContactValid
}

// IsStringBackedType reports whether values of the attribute type are JSON strings.
func IsStringBackedType(attrType string) bool {
	return attrType == TypeString || attrType == TypeEmail || attrType == TypeMobile
}

// IsContactType reports whether the attribute type is a verifiable contact: an email or a mobile number.
func IsContactType(attrType string) bool {
	return attrType == TypeEmail || attrType == TypeMobile
}

// ContactChannel returns the verification channel of a contact type: SMS for a mobile number, email
// otherwise.
func ContactChannel(attrType string) providers.VerificationChannel {
	if attrType == TypeMobile {
		return providers.VerificationChannelSMS
	}

	return providers.VerificationChannelEmail
}

// ContactTypeForChannel returns the contact attribute type a channel delivers to.
func ContactTypeForChannel(channel providers.VerificationChannel) string {
	if channel == providers.VerificationChannelSMS {
		return TypeMobile
	}

	return TypeEmail
}

// CanonicalContactValue returns the comparable form of a contact value: an email address is trimmed and
// its domain lowercased. A value of any other type is returned unchanged.
func CanonicalContactValue(attrType, value string) string {
	if attrType != TypeEmail {
		return value
	}

	value = strings.TrimSpace(value)
	if at := strings.LastIndexByte(value, '@'); at >= 0 {
		value = value[:at+1] + strings.ToLower(value[at+1:])
	}

	return value
}

// VerificationClaims returns the verification claims the server issues for a type's attributes, mapped to
// their contact attribute: <attribute>_verified for each email or mobile attribute, unless the schema
// declares that name itself.
func VerificationClaims(attributes []AttributeInfo) map[string]string {
	declared := make(map[string]bool, len(attributes))
	for _, attribute := range attributes {
		declared[attribute.Attribute] = true
	}

	claims := make(map[string]string)
	for _, attribute := range attributes {
		name := attribute.Attribute + verificationClaimSuffix
		if IsContactType(attribute.Type) && !declared[name] {
			claims[name] = attribute.Attribute
		}
	}

	return claims
}

// IsVerificationClaimName reports whether a claim name has the form of a verification claim.
func IsVerificationClaimName(name string) bool {
	return strings.HasSuffix(name, verificationClaimSuffix)
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
	// Enum holds the permitted values in schema order when the property constrains them, and is
	// empty otherwise.
	Enum []string
	// DisplayOrder is the attribute's position among the others when it is collected, lower first
	// and attribute name for the same order, and nil when the schema does not set one.
	DisplayOrder *int
	// HiddenOnCreate is true when an optional attribute is not asked for when an account is created.
	HiddenOnCreate bool
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
		attrHints := cs.hints[attr]
		result = append(result, AttributeInfo{
			Attribute:      attr,
			DisplayName:    prop.getDisplayName(),
			Type:           prop.getType(),
			Required:       prop.isRequired(),
			Credential:     isCredential,
			Unique:         prop.isUnique(),
			Enum:           prop.getEnum(),
			DisplayOrder:   attrHints.displayOrder,
			HiddenOnCreate: attrHints.hiddenOnCreate,
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
		if _, exists := userAttrs[propName]; !exists && prop.isRequired() &&
			(!skipCredentialRequired || !prop.isCredential()) {
			return false, nil
		}
	}

	return cs.validateValues(ctx, userAttrs, logger)
}

// ValidateValues validates only the given attributes against their schema properties, so a partial
// payload such as a credential update is checked without requiring the rest of the entity.
func (cs *Schema) ValidateValues(
	ctx context.Context, attributes json.RawMessage, logger *log.Logger) (bool, error) {
	if len(attributes) == 0 {
		return true, nil
	}

	var attrs map[string]any
	if err := json.Unmarshal(attributes, &attrs); err != nil {
		return false, fmt.Errorf("failed to unmarshal attributes: %w", err)
	}

	return cs.validateValues(ctx, attrs, logger)
}

// validateValues checks each attribute against its schema property and rejects attributes the
// schema does not declare.
func (cs *Schema) validateValues(
	ctx context.Context, attrs map[string]any, logger *log.Logger) (bool, error) {
	for name, value := range attrs {
		prop, declared := cs.properties[name]
		if !declared {
			logger.Debug(ctx, "Attribute not defined in schema", log.String("attribute", name))
			return false, nil
		}
		isValid, err := prop.validateValue(ctx, value, name, logger)
		if err != nil {
			return false, err
		}
		if !isValid {
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
		hints:      make(map[string]attributeHints, len(schemaMap)),
	}

	for propName, propRaw := range schemaMap {
		var propMap map[string]json.RawMessage
		if err := json.Unmarshal(propRaw, &propMap); err != nil {
			return nil, fmt.Errorf("invalid property '%s': property definition must be an object", propName)
		}
		propHints, err := extractDisplay(propMap)
		if err != nil {
			return nil, fmt.Errorf("invalid property '%s': %w", propName, err)
		}
		compiledProp, err := compilePropertyMap(propName, propMap)
		if err != nil {
			return nil, fmt.Errorf("invalid property '%s': %w", propName, err)
		}
		compiled.properties[propName] = compiledProp
		compiled.hints[propName] = propHints
	}

	return compiled, nil
}

// extractDisplay reads the display settings of a top-level property and removes them from propMap.
func extractDisplay(propMap map[string]json.RawMessage) (attributeHints, error) {
	var result attributeHints
	raw, ok := propMap[keyDisplay]
	if !ok {
		return result, nil
	}
	delete(propMap, keyDisplay)

	var display map[string]json.RawMessage
	if err := json.Unmarshal(raw, &display); err != nil || display == nil {
		return result, fmt.Errorf("'%s' field must be an object", keyDisplay)
	}
	for key, value := range display {
		switch key {
		case keyOrder:
			var order *int
			if err := json.Unmarshal(value, &order); err != nil || order == nil || *order < 0 {
				return result, fmt.Errorf("'%s.%s' field must be a non-negative integer", keyDisplay, keyOrder)
			}
			result.displayOrder = order
		case keyOnCreate:
			hidden, err := extractHidden(value)
			if err != nil {
				return result, err
			}
			result.hiddenOnCreate = hidden
		default:
			return result, fmt.Errorf("invalid field '%s.%s'", keyDisplay, key)
		}
	}
	return result, nil
}

// extractHidden reads the hidden setting of display.onCreate.
func extractHidden(raw json.RawMessage) (bool, error) {
	var onCreate map[string]json.RawMessage
	if err := json.Unmarshal(raw, &onCreate); err != nil || onCreate == nil {
		return false, fmt.Errorf("'%s.%s' field must be an object", keyDisplay, keyOnCreate)
	}
	hidden := false
	for key, value := range onCreate {
		if key != keyHidden {
			return false, fmt.Errorf("invalid field '%s.%s.%s'", keyDisplay, keyOnCreate, key)
		}
		var val *bool
		if err := json.Unmarshal(value, &val); err != nil || val == nil {
			return false, fmt.Errorf("'%s.%s.%s' field must be a boolean", keyDisplay, keyOnCreate, keyHidden)
		}
		hidden = *val
	}
	return hidden, nil
}

func compileProperty(propName string, propRaw json.RawMessage) (property, error) {
	var propMap map[string]json.RawMessage
	if err := json.Unmarshal(propRaw, &propMap); err != nil {
		return nil, fmt.Errorf("property definition must be an object")
	}
	return compilePropertyMap(propName, propMap)
}

func compilePropertyMap(propName string, propMap map[string]json.RawMessage) (property, error) {
	typeRaw, exists := propMap["type"]
	if !exists {
		return nil, fmt.Errorf("missing required 'type' field")
	}

	var typeStr string
	if err := json.Unmarshal(typeRaw, &typeStr); err != nil {
		return nil, fmt.Errorf("'type' field must be a string")
	}

	switch typeStr {
	case TypeString, TypeEmail, TypeMobile:
		return compileStringProperty(propMap, typeStr)
	case TypeNumber:
		return compileNumberProperty(propMap)
	case TypeBoolean:
		return compileBooleanProperty(propMap)
	case TypeObject:
		return compileObjectProperty(propMap)
	case TypeArray:
		return compileArrayProperty(propName, propMap)
	default:
		return nil, fmt.Errorf(
			"invalid type '%s', must be one of: string, email, mobile, number, boolean, object, array", typeStr)
	}
}
