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

package export

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/log"
)

type templatingRules struct {
	Application        *resourceRules `yaml:"Application,omitempty"`
	IdentityProvider   *resourceRules `yaml:"IdentityProvider,omitempty"`
	NotificationSender *resourceRules `yaml:"NotificationSender,omitempty"`
	EntityType         *resourceRules `yaml:"EntityType,omitempty"`
}

// ResourceRules defines variables and array variables to parameterize
type resourceRules struct {
	Variables             []string `yaml:"Variables,omitempty"`
	ArrayVariables        []string `yaml:"ArrayVariables,omitempty"`
	DynamicPropertyFields []string `yaml:"DynamicPropertyFields,omitempty"`
}

const (
	yamlTagOmitEmpty = "omitempty"
	yamlTagInline    = "inline"
)

// Parameterizer handles the templating logic
type parameterizer struct {
	rules templatingRules
}

// newParameterizer creates a new Parameterizer instance with the given templating rules
func newParameterizer(rules templatingRules) *parameterizer {
	return &parameterizer{rules: rules}
}

// ToParameterizedYAML converts an object directly to parameterized YAML.
// It returns the template string and a map of variable names to their original values.
func (p *parameterizer) ToParameterizedYAML(obj interface{},
	resourceType string, resourceName string,
	rules *declarativeresource.ResourceRules) (string, map[string]string, error) {
	// Convert imported type to local type for compatibility
	var localRules *resourceRules
	if rules != nil {
		localRules = &resourceRules{
			Variables:             rules.Variables,
			ArrayVariables:        rules.ArrayVariables,
			DynamicPropertyFields: rules.DynamicPropertyFields,
		}
	}

	// Convert object to yaml.Node directly to preserve field order and handle omitempty
	// Pass rules so fields in parameterization rules bypass omitempty
	var node yaml.Node
	if err := p.structToNodeIgnoringOmitempty(obj, &node, localRules, "", resourceName); err != nil {
		return "", nil, fmt.Errorf("failed to convert object to node: %w", err)
	}

	if localRules == nil {
		// No rules, just marshal the node as-is
		var buf bytes.Buffer
		encoder := yaml.NewEncoder(&buf)
		encoder.SetIndent(2)
		if err := encoder.Encode(&node); err != nil {
			return "", nil, fmt.Errorf("failed to marshal data: %w", err)
		}
		err := encoder.Close()
		if err != nil {
			return "", nil, fmt.Errorf("failed to close encoder: %w", err)
		}
		return buf.String(), nil, nil
	}

	// Convert struct field paths to YAML field paths
	rulesWithYAMLPaths := p.convertStructPathsToYAMLPaths(obj, localRules)

	// Capture original values before parameterization replaces them.
	// Dynamic property values must be extracted from the original struct here because
	// structToNodeIgnoringOmitempty already baked template placeholders into their nodes.
	variableValues := p.extractValuesFromNode(&node, rulesWithYAMLPaths, resourceName)
	for k, v := range p.extractDynamicPropertyValues(obj, localRules, resourceName) {
		variableValues[k] = v
	}

	// Apply parameterization to the node tree
	if err := p.parameterizeNode(&node, rulesWithYAMLPaths, resourceName); err != nil {
		return "", nil, err
	}

	// Marshal back to YAML with preserved indentation
	// Use custom renderer to handle template syntax properly
	var buf bytes.Buffer
	if err := p.renderNode(&buf, &node, 0); err != nil {
		return "", nil, fmt.Errorf("failed to render parameterized YAML: %w", err)
	}

	return buf.String(), variableValues, nil
}

// extractValuesFromNode reads the original values of parameterization variables from the node
// tree before they are replaced with template placeholders.
func (p *parameterizer) extractValuesFromNode(
	node *yaml.Node, rules *resourceRules, resourceName string,
) map[string]string {
	values := make(map[string]string)
	if node.Kind != yaml.DocumentNode || len(node.Content) == 0 {
		return values
	}
	root := node.Content[0]

	for _, path := range rules.Variables {
		varName := p.pathToVariableName(resourceName, path)
		if val := p.getScalarFromNode(root, path); val != "" {
			values[varName] = val
		}
	}

	for _, path := range rules.ArrayVariables {
		varName := p.pathToVariableName(resourceName, path)
		if vals := p.getArrayFromNode(root, path); vals != nil {
			jsonBytes, err := json.Marshal(vals)
			if err == nil {
				values[varName] = string(jsonBytes)
			}
		}
	}

	return values
}

// extractDynamicPropertyValues reads actual property values from the original struct before
// structToNodeIgnoringOmitempty converts them to template placeholders.
// It navigates each DynamicPropertyFields path, iterates the []cmodels.Property slice found
// there, and maps generatePropertyVarName(resourceName, propName) → actual value.
func (p *parameterizer) extractDynamicPropertyValues(
	obj interface{}, rules *resourceRules, resourceName string,
) map[string]string {
	values := make(map[string]string)
	if rules == nil || len(rules.DynamicPropertyFields) == 0 {
		return values
	}

	v := reflect.ValueOf(obj)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return values
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return values
	}

	for _, fieldPath := range rules.DynamicPropertyFields {
		field, found := p.findFieldByNameCaseInsensitive(v.Type(), fieldPath)
		if !found {
			continue
		}
		fieldVal := v.FieldByName(field.Name)
		if !fieldVal.IsValid() || fieldVal.Kind() != reflect.Slice {
			continue
		}

		for i := 0; i < fieldVal.Len(); i++ {
			propVal := fieldVal.Index(i)
			if propVal.CanAddr() {
				propVal = propVal.Addr()
			}

			nameMethod := propVal.MethodByName("GetName")
			if !nameMethod.IsValid() {
				continue
			}
			nameResults := nameMethod.Call(nil)
			if len(nameResults) == 0 {
				continue
			}
			propName := nameResults[0].String()

			valueMethod := propVal.MethodByName("GetValue")
			if !valueMethod.IsValid() {
				continue
			}
			valueResults := valueMethod.Call(nil)
			// GetValue returns (string, error); skip on error
			if len(valueResults) < 2 || !valueResults[1].IsNil() {
				continue
			}
			propValue := valueResults[0].String()

			varName := p.generatePropertyVarName(resourceName, propName)
			values[varName] = propValue
		}
	}

	return values
}

// getScalarFromNode traverses the YAML node tree and returns the scalar value at the given path.
// Returns an empty string when the path does not exist or the target is not a scalar.
func (p *parameterizer) getScalarFromNode(node *yaml.Node, path string) string {
	parts := strings.Split(path, ".")
	current := node

	for i, part := range parts {
		isArrayAccess := strings.HasSuffix(part, "[]")
		fieldName := strings.TrimSuffix(part, "[]")

		if current.Kind != yaml.MappingNode {
			return ""
		}

		found := false
		for j := 0; j < len(current.Content); j += 2 {
			if current.Content[j].Value != fieldName {
				continue
			}
			valueNode := current.Content[j+1]
			if isArrayAccess {
				if valueNode.Kind == yaml.SequenceNode && i < len(parts)-1 {
					remainingPath := strings.Join(parts[i+1:], ".")
					for _, elem := range valueNode.Content {
						if val := p.getScalarFromNode(elem, remainingPath); val != "" {
							return val
						}
					}
				}
				return ""
			}
			if i == len(parts)-1 {
				if valueNode.Kind == yaml.ScalarNode {
					return valueNode.Value
				}
				return ""
			}
			current = valueNode
			found = true
			break
		}
		if !found {
			return ""
		}
	}
	return ""
}

// getArrayFromNode traverses the YAML node tree and returns the values of the sequence at the
// given path. Returns nil when the path does not exist or the target is not a sequence.
func (p *parameterizer) getArrayFromNode(node *yaml.Node, path string) []string {
	parts := strings.Split(path, ".")
	current := node

	for i, part := range parts {
		isArrayAccess := strings.HasSuffix(part, "[]")
		fieldName := strings.TrimSuffix(part, "[]")

		if current.Kind != yaml.MappingNode {
			return nil
		}

		for j := 0; j < len(current.Content); j += 2 {
			if current.Content[j].Value != fieldName {
				continue
			}
			valueNode := current.Content[j+1]
			if isArrayAccess {
				if valueNode.Kind == yaml.SequenceNode && i < len(parts)-1 {
					remainingPath := strings.Join(parts[i+1:], ".")
					var results []string
					for _, elem := range valueNode.Content {
						results = append(results, p.getArrayFromNode(elem, remainingPath)...)
					}
					return results
				}
				return nil
			}
			if i == len(parts)-1 {
				if valueNode.Kind == yaml.SequenceNode {
					vals := make([]string, 0)
					for _, elem := range valueNode.Content {
						if elem.Kind == yaml.ScalarNode && elem.Value != "" {
							vals = append(vals, elem.Value)
						}
					}
					return vals
				}
				if valueNode.Kind == yaml.ScalarNode && valueNode.Value != "" {
					return []string{valueNode.Value}
				}
				return nil
			}
			current = valueNode
			break
		}
	}
	return nil
}

// structToNodeIgnoringOmitempty converts a struct to yaml.Node while preserving field order
// and respecting omitempty tags (skip empty fields unless they're in parameterization rules)
func (p *parameterizer) structToNodeIgnoringOmitempty(
	obj interface{}, node *yaml.Node, rules *resourceRules, currentPath string, resourceName string,
) error {
	v := reflect.ValueOf(obj)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			node.Kind = yaml.ScalarNode
			node.Tag = "!!null"
			node.Value = "null"
			return nil
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return fmt.Errorf("expected struct, got %v", v.Kind())
	}

	// Create document node
	node.Kind = yaml.DocumentNode

	// Create mapping node for the struct
	mappingNode := &yaml.Node{
		Kind: yaml.MappingNode,
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Get the yaml tag
		yamlTag := field.Tag.Get("yaml")
		if yamlTag == "" || yamlTag == "-" {
			continue
		}

		// Parse yaml tag to detect omitempty and inline options
		tagParts := strings.Split(yamlTag, ",")
		yamlFieldName := tagParts[0]
		hasOmitEmpty := false
		isInline := false
		for _, part := range tagParts[1:] {
			switch part {
			case yamlTagOmitEmpty:
				hasOmitEmpty = true
			case yamlTagInline:
				isInline = true
			}
		}

		// Mirror yaml.v3's `yaml:",inline"` semantics: flatten the embedded struct's fields
		// into the current mapping rather than nesting them under an empty key.
		if isInline {
			err := p.appendInlineStructFields(mappingNode, fieldValue, rules, currentPath, resourceName)
			if err != nil {
				return err
			}
			continue
		}

		// Check for forced quoting via yamlfmt tag
		forceQuoted := field.Tag.Get("yamlfmt") == "quoted"

		// Build the current field path (using struct field names)
		fieldPath := field.Name
		if currentPath != "" {
			fieldPath = currentPath + "." + field.Name
		}

		// Skip empty fields if omitempty is set AND field is NOT in parameterization rules
		if hasOmitEmpty && p.isEmptyValue(fieldValue) && !p.isFieldInRules(rules, fieldPath) {
			continue
		}

		// Create key node
		keyNode := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: yamlFieldName,
		}

		// Create value node
		valueNode, err := p.fieldToNode(fieldValue, rules, fieldPath, resourceName)
		if err != nil {
			return err
		}

		// Apply forced quoting if requested
		if forceQuoted && valueNode.Kind == yaml.ScalarNode {
			valueNode.Style = yaml.DoubleQuotedStyle
		}

		// Add key-value pair to mapping
		mappingNode.Content = append(mappingNode.Content, keyNode, valueNode)
	}

	node.Content = []*yaml.Node{mappingNode}
	return nil
}

// appendInlineStructFields flattens a `yaml:",inline"` embedded struct's fields into the
// given mapping node, honoring per-field omitempty / rules / yamlfmt as the regular walkers do.
func (p *parameterizer) appendInlineStructFields(
	mapping *yaml.Node, fieldValue reflect.Value,
	rules *resourceRules, currentPath, resourceName string,
) error {
	v := fieldValue
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil
	}
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		yamlTag := field.Tag.Get("yaml")
		if yamlTag == "" || yamlTag == "-" {
			continue
		}
		tagParts := strings.Split(yamlTag, ",")
		yamlFieldName := tagParts[0]
		hasOmitEmpty := false
		isInline := false
		for _, part := range tagParts[1:] {
			switch part {
			case yamlTagOmitEmpty:
				hasOmitEmpty = true
			case yamlTagInline:
				isInline = true
			}
		}

		innerValue := v.Field(i)
		nestedPath := field.Name
		if currentPath != "" {
			nestedPath = currentPath + "." + field.Name
		}

		if isInline {
			if err := p.appendInlineStructFields(mapping, innerValue, rules, currentPath, resourceName); err != nil {
				return err
			}
			continue
		}

		if hasOmitEmpty && p.isEmptyValue(innerValue) && !p.isFieldInRules(rules, nestedPath) {
			continue
		}

		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: yamlFieldName}
		valueNode, err := p.fieldToNode(innerValue, rules, nestedPath, resourceName)
		if err != nil {
			return err
		}
		if field.Tag.Get("yamlfmt") == "quoted" && valueNode.Kind == yaml.ScalarNode {
			valueNode.Style = yaml.DoubleQuotedStyle
		}
		mapping.Content = append(mapping.Content, keyNode, valueNode)
	}
	return nil
}

// isEmptyValue checks if a reflect.Value is considered empty for omitempty purposes
func (p *parameterizer) isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	}
	return false
}

// isFieldInRules checks if a field path exists in the parameterization rules
func (p *parameterizer) isFieldInRules(rules *resourceRules, fieldPath string) bool {
	if rules == nil {
		return false
	}

	// Normalize the field path to lowercase for case-insensitive matching
	normalizedPath := strings.ToLower(fieldPath)

	// Check Variables
	for _, varPath := range rules.Variables {
		// Strip [] slice notation before comparing: rules use "Foo[].Bar" but traversal
		// produces "Foo.Bar" (index not tracked in path).
		normalizedVarPath := strings.ToLower(strings.ReplaceAll(varPath, "[]", ""))
		if normalizedVarPath == normalizedPath {
			return true
		}
	}

	// Check ArrayVariables
	for _, arrPath := range rules.ArrayVariables {
		normalizedArrPath := strings.ToLower(strings.ReplaceAll(arrPath, "[]", ""))
		if normalizedArrPath == normalizedPath {
			return true
		}
	}

	return false
}

// findFieldByNameCaseInsensitive finds a struct field by name, case-insensitively
func (p *parameterizer) findFieldByNameCaseInsensitive(t reflect.Type, name string) (reflect.StructField, bool) {
	// First try exact match
	if field, found := t.FieldByName(name); found {
		return field, true
	}

	// Try case-insensitive match
	lowerName := strings.ToLower(name)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if strings.ToLower(field.Name) == lowerName {
			return field, true
		}
	}

	return reflect.StructField{}, false
}

// isPropertySlice checks if a type is a slice of cmodels.Property
func (p *parameterizer) isPropertySlice(t reflect.Type) bool {
	if t.Kind() != reflect.Slice {
		return false
	}
	elemType := t.Elem()
	// Check if element type name contains "Property" and is from cmodels package
	return strings.Contains(elemType.String(), "cmodels.Property")
}

// isFieldDynamicProperty checks if a field path is in the DynamicPropertyFields list
func (p *parameterizer) isFieldDynamicProperty(rules *resourceRules, fieldPath string) bool {
	if rules == nil || len(rules.DynamicPropertyFields) == 0 {
		return false
	}

	// Normalize the field path to lowercase for case-insensitive matching
	normalizedPath := strings.ToLower(fieldPath)

	for _, dynField := range rules.DynamicPropertyFields {
		normalizedDynField := strings.ToLower(dynField)
		if normalizedDynField == normalizedPath {
			return true
		}
	}

	return false
}

// propertyToYAMLNode converts a Property interface to a YAML node
func (p *parameterizer) propertyToYAMLNode(propValue reflect.Value, resourceName string) *yaml.Node {
	node := &yaml.Node{Kind: yaml.MappingNode}

	// Property has methods: GetName(), GetValue(), IsSecret()
	// We need to call these methods via reflection

	// Get the name
	nameMethod := propValue.MethodByName("GetName")
	if !nameMethod.IsValid() {
		return node
	}
	nameResults := nameMethod.Call(nil)
	if len(nameResults) == 0 {
		return node
	}
	propName := nameResults[0].String()

	// Get is_secret
	isSecretMethod := propValue.MethodByName("IsSecret")
	isSecret := false
	if isSecretMethod.IsValid() {
		isSecretResults := isSecretMethod.Call(nil)
		if len(isSecretResults) > 0 {
			isSecret = isSecretResults[0].Bool()
		}
	}

	// Generate template variable name
	propValueStr := fmt.Sprintf("{{.%s}}", p.generatePropertyVarName(resourceName, propName))

	// Build the YAML node: {name: "...", value: "...", is_secret: true/false}
	// Add name
	node.Content = append(node.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "name"},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: propName},
	)

	// Add value
	node.Content = append(node.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "value"},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: propValueStr},
	)

	// Add is_secret if true (omit if false for cleaner YAML)
	if isSecret {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "is_secret"},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"},
		)
	}

	return node
}

// generatePropertyVarName generates a context-aware variable name for a property
// e.g., "Export Test IDP" + "client_id" -> "EXPORT_TEST_IDP_CLIENT_ID"
func (p *parameterizer) generatePropertyVarName(resourceName, propertyName string) string {
	// Convert resource name: replace spaces with underscores and convert to snake_case
	resourcePrefix := strings.ReplaceAll(resourceName, " ", "_")
	resourcePrefix = p.toSnakeCase(resourcePrefix)

	// Convert property name to snake_case
	propName := p.toSnakeCase(propertyName)

	// Combine them
	return resourcePrefix + "_" + propName
}

// handleInterfaceValue handles interface{} types by JSON-encoding them.
func (p *parameterizer) handleInterfaceValue(v reflect.Value) *yaml.Node {
	// Get the actual value from the interface
	actualValue := v.Elem()

	// If the actual value is nil, return null
	if !actualValue.IsValid() {
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!null",
			Value: "null",
		}
	}

	// JSON-encode the interface value to preserve its structure
	jsonBytes, err := json.Marshal(actualValue.Interface())
	if err != nil {
		// If JSON encoding fails, fall back to string representation
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: fmt.Sprintf("%v", actualValue.Interface()),
		}
	}

	// Return the JSON string as a YAML scalar
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: string(jsonBytes),
	}
}

// handleStructNode converts a struct reflect.Value to a YAML mapping node.
func (p *parameterizer) handleStructNode(
	v reflect.Value, rules *resourceRules, currentPath string, resourceName string) (*yaml.Node, error) {
	node := &yaml.Node{Kind: yaml.MappingNode}
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		yamlTag := field.Tag.Get("yaml")
		if yamlTag == "" || yamlTag == "-" {
			continue
		}

		tagParts := strings.Split(yamlTag, ",")
		yamlFieldName := tagParts[0]
		hasOmitEmpty := false
		isInline := false
		for _, part := range tagParts[1:] {
			switch part {
			case yamlTagOmitEmpty:
				hasOmitEmpty = true
			case yamlTagInline:
				isInline = true
			}
		}

		fieldValue := v.Field(i)

		if isInline {
			if err := p.appendInlineStructFields(node, fieldValue, rules, currentPath, resourceName); err != nil {
				return nil, err
			}
			continue
		}

		// Build the nested field path
		nestedFieldPath := field.Name
		if currentPath != "" {
			nestedFieldPath = currentPath + "." + field.Name
		}

		// Skip empty fields if omitempty is set AND field is NOT in parameterization rules
		if hasOmitEmpty && p.isEmptyValue(fieldValue) && !p.isFieldInRules(rules, nestedFieldPath) {
			continue
		}

		keyNode := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: yamlFieldName,
		}
		valueNode, err := p.fieldToNode(fieldValue, rules, nestedFieldPath, resourceName)
		if err != nil {
			return nil, err
		}
		node.Content = append(node.Content, keyNode, valueNode)
	}
	return node, nil
}

// handleSliceOrArrayNode converts a slice or array reflect.Value to a YAML node.
func (p *parameterizer) handleSliceOrArrayNode(
	v reflect.Value, rules *resourceRules, currentPath string, resourceName string) (*yaml.Node, error) {
	// Check if this is json.RawMessage ([]byte) and handle it specially
	if v.Type() == reflect.TypeOf(json.RawMessage{}) {
		return p.handleJSONRawMessage(v)
	}

	// Check if this is a Property slice that should be dynamically parameterized
	if p.isPropertySlice(v.Type()) && p.isFieldDynamicProperty(rules, currentPath) {
		// Handle Property slices with dynamic parameterization
		node := &yaml.Node{Kind: yaml.SequenceNode}
		for i := 0; i < v.Len(); i++ {
			propValue := v.Index(i)
			// If the property value is addressable, get its address for method calls
			if propValue.CanAddr() {
				propValue = propValue.Addr()
			}
			propNode := p.propertyToYAMLNode(propValue, resourceName)
			node.Content = append(node.Content, propNode)
		}
		return node, nil
	}

	// Convert regular slices/arrays
	node := &yaml.Node{Kind: yaml.SequenceNode}
	for i := 0; i < v.Len(); i++ {
		itemNode, err := p.fieldToNode(v.Index(i), rules, currentPath, resourceName)
		if err != nil {
			return nil, err
		}
		node.Content = append(node.Content, itemNode)
	}
	return node, nil
}

// handleJSONRawMessage converts a json.RawMessage to a YAML scalar node containing the JSON string.
func (p *parameterizer) handleJSONRawMessage(v reflect.Value) (*yaml.Node, error) {
	// For json.RawMessage, export as a JSON string (not parsed YAML structure)
	if v.Len() == 0 {
		// Empty json.RawMessage is valid - return null
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!null",
			Value: "null",
		}, nil
	}

	rawBytes := v.Bytes()
	// Validate it's valid JSON
	if !json.Valid(rawBytes) {
		return nil, fmt.Errorf("invalid JSON in RawMessage: %s", string(rawBytes))
	}

	// Return as a string containing the JSON
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: string(rawBytes),
	}, nil
}

// handleMapNode converts a map reflect.Value to a YAML mapping node.
func (p *parameterizer) handleMapNode(
	v reflect.Value, rules *resourceRules, currentPath string, resourceName string) (*yaml.Node, error) {
	node := &yaml.Node{Kind: yaml.MappingNode}
	iter := v.MapRange()
	for iter.Next() {
		keyNode := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: fmt.Sprintf("%v", iter.Key().Interface()),
		}
		valueNode, err := p.fieldToNode(iter.Value(), rules, currentPath, resourceName)
		if err != nil {
			return nil, err
		}
		node.Content = append(node.Content, keyNode, valueNode)
	}
	return node, nil
}

// fieldToNode converts a reflect.Value to yaml.Node
func (p *parameterizer) fieldToNode(
	v reflect.Value, rules *resourceRules, currentPath string, resourceName string) (*yaml.Node, error) {
	// Handle nil pointers
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!null",
			Value: "null",
		}, nil
	}

	// Dereference pointers
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	// Handle interface{} types by JSON-encoding them
	if v.Kind() == reflect.Interface && !v.IsNil() {
		return p.handleInterfaceValue(v), nil
	}

	switch v.Kind() {
	case reflect.Struct:
		return p.handleStructNode(v, rules, currentPath, resourceName)

	case reflect.Slice, reflect.Array:
		return p.handleSliceOrArrayNode(v, rules, currentPath, resourceName)

	case reflect.Map:
		return p.handleMapNode(v, rules, currentPath, resourceName)

	case reflect.String:
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: v.String(),
		}, nil

	case reflect.Bool:
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!bool",
			Value: fmt.Sprintf("%t", v.Bool()),
		}, nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!int",
			Value: fmt.Sprintf("%d", v.Int()),
		}, nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!int",
			Value: fmt.Sprintf("%d", v.Uint()),
		}, nil

	case reflect.Float32, reflect.Float64:
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!float",
			Value: fmt.Sprintf("%g", v.Float()),
		}, nil

	default:
		// For other types, use interface conversion
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: fmt.Sprintf("%v", v.Interface()),
		}, nil
	}
}

// structToMapIgnoringOmitempty converts a struct to a map while including all fields
// regardless of omitempty tags. This ensures fields to be parameterized are present
// even if they're empty/nil in the original struct.
func (p *parameterizer) structToMapIgnoringOmitempty(obj interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	v := reflect.ValueOf(obj)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected struct, got %v", v.Kind())
	}

	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Get the yaml tag
		yamlTag := field.Tag.Get("yaml")
		if yamlTag == "" || yamlTag == "-" {
			continue
		}

		// Parse yaml tag to get field name (ignore omitempty and other options)
		tagParts := strings.Split(yamlTag, ",")
		yamlFieldName := tagParts[0]

		// Convert the field value to interface
		fieldInterface := p.convertFieldToInterface(fieldValue)
		result[yamlFieldName] = fieldInterface
	}

	return result, nil
}

// convertFieldToInterface recursively converts reflect.Value to interface{}
// handling nested structs, maps, slices, and pointers
func (p *parameterizer) convertFieldToInterface(v reflect.Value) interface{} {
	// Handle nil pointers
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return nil
	}

	// Dereference pointers
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Struct:
		// Recursively convert nested structs
		result := make(map[string]interface{})
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			field := t.Field(i)
			if !field.IsExported() {
				continue
			}

			yamlTag := field.Tag.Get("yaml")
			if yamlTag == "" || yamlTag == "-" {
				continue
			}

			tagParts := strings.Split(yamlTag, ",")
			yamlFieldName := tagParts[0]
			result[yamlFieldName] = p.convertFieldToInterface(v.Field(i))
		}
		return result

	case reflect.Slice, reflect.Array:
		// Convert slices/arrays
		result := make([]interface{}, v.Len())
		for i := 0; i < v.Len(); i++ {
			result[i] = p.convertFieldToInterface(v.Index(i))
		}
		return result

	case reflect.Map:
		// Convert maps
		result := make(map[string]interface{})
		iter := v.MapRange()
		for iter.Next() {
			key := iter.Key()
			val := iter.Value()
			keyStr := fmt.Sprintf("%v", key.Interface())
			result[keyStr] = p.convertFieldToInterface(val)
		}
		return result

	default:
		// For basic types, return the interface value
		return v.Interface()
	}
}

// convertStructPathsToYAMLPaths converts Go struct field paths to YAML field paths
// e.g., "InboundAuthConfig[].OAuthConfig.ClientID" -> "inbound_auth_config[].config.client_id"
func (p *parameterizer) convertStructPathsToYAMLPaths(obj interface{}, rules *resourceRules) *resourceRules {
	logger := log.GetLogger().With(log.String("component", "Parameterizer"))

	converted := &resourceRules{
		Variables:      make([]string, len(rules.Variables)),
		ArrayVariables: make([]string, len(rules.ArrayVariables)),
	}

	objType := reflect.TypeOf(obj)
	if objType.Kind() == reflect.Ptr {
		objType = objType.Elem()
	}

	for i, path := range rules.Variables {
		yamlPath := p.convertPathToYAMLPath(objType, path)
		converted.Variables[i] = yamlPath
		// Debug log to help troubleshoot path resolution
		logger.Debug("Converted variable path",
			log.String("original", path),
			log.String("yaml", yamlPath))
	}

	for i, path := range rules.ArrayVariables {
		yamlPath := p.convertPathToYAMLPath(objType, path)
		converted.ArrayVariables[i] = yamlPath
		// Debug log to help troubleshoot path resolution
		logger.Debug("Converted array variable path",
			log.String("original", path),
			log.String("yaml", yamlPath))
	}

	return converted
}

// convertPathToYAMLPath converts a single struct field path to YAML field path
// Handles array notation (e.g., "FieldName[]" or "FieldName[].SubField")
func (p *parameterizer) convertPathToYAMLPath(objType reflect.Type, path string) string {
	parts := strings.Split(path, ".")
	yamlParts := make([]string, 0, len(parts))

	currentType := objType
	for _, part := range parts {
		if currentType.Kind() == reflect.Ptr {
			currentType = currentType.Elem()
		}

		// Check if this part indicates an array access (e.g., "FieldName[]")
		isArrayAccess := strings.HasSuffix(part, "[]")
		fieldName := part
		if isArrayAccess {
			// Remove the [] suffix to get the actual field name
			fieldName = strings.TrimSuffix(part, "[]")
		}

		if currentType.Kind() != reflect.Struct {
			// Can't continue, just use the original part
			yamlParts = append(yamlParts, part)
			continue
		}

		// Find the field by name (case-insensitive)
		field, found := p.findFieldByNameCaseInsensitive(currentType, fieldName)
		if !found {
			// Field not found, use original part
			yamlParts = append(yamlParts, part)
			continue
		}

		// Get YAML tag
		yamlTag := field.Tag.Get("yaml")
		if yamlTag == "" || yamlTag == "-" {
			// No yaml tag, use field name
			yamlParts = append(yamlParts, part)
			currentType = field.Type
			continue
		}

		// Parse yaml tag (format: "name,omitempty,flow" etc.)
		yamlName := strings.Split(yamlTag, ",")[0]

		// Preserve the array notation in the YAML path
		if isArrayAccess {
			yamlParts = append(yamlParts, yamlName+"[]")
			// For arrays/slices, get the element type
			if field.Type.Kind() == reflect.Slice || field.Type.Kind() == reflect.Array {
				currentType = field.Type.Elem()
			} else {
				currentType = field.Type
			}
		} else {
			yamlParts = append(yamlParts, yamlName)
			currentType = field.Type
		}
	}

	return strings.Join(yamlParts, ".")
}

// parameterizeNode applies templating rules to yaml.Node tree
func (p *parameterizer) parameterizeNode(node *yaml.Node, rules *resourceRules, resourceName string) error {
	if node.Kind != yaml.DocumentNode || len(node.Content) == 0 {
		return nil
	}

	root := node.Content[0]

	// Process simple variables
	for _, path := range rules.Variables {
		varName := p.pathToVariableName(resourceName, path)
		if err := p.replaceNodeValue(root, path, fmt.Sprintf("{{.%s}}", varName)); err != nil {
			return err
		}
	}

	// Process array variables
	for _, path := range rules.ArrayVariables {
		varName := p.pathToVariableName(resourceName, path)
		if err := p.replaceArrayNode(root, path, varName); err != nil {
			return err
		}
	}

	return nil
}

// pathToVariableName converts path to uppercase variable name with app name prefix
// e.g., "My App", "InboundAuthConfig.oauth.RedirectURIs" -> "MY_APP_REDIRECT_URIS"
func (p *parameterizer) pathToVariableName(appName, path string) string {
	parts := strings.Split(path, ".")
	lastPart := parts[len(parts)-1]

	// Convert appName: replace spaces with underscores, convert camelCase to snake_case, then uppercase
	appPrefix := strings.ReplaceAll(appName, " ", "_")
	appPrefix = p.toSnakeCase(appPrefix)

	// Convert field name from camelCase/PascalCase to snake_case
	fieldName := p.toSnakeCase(lastPart)

	// Prepend app prefix to field name
	return appPrefix + "_" + fieldName
}

// toSnakeCase converts camelCase/PascalCase to UPPER_SNAKE_CASE
func (p *parameterizer) toSnakeCase(s string) string {
	var result strings.Builder
	runes := []rune(s)

	for i := 0; i < len(runes); i++ {
		r := runes[i]

		// Add underscore before uppercase letter if:
		// 1. Not at the beginning (i > 0)
		// 2. Previous character is lowercase (not underscore or uppercase) OR
		// 3. Next character exists and is lowercase (handles acronyms like "ClientID" -> "CLIENT_ID")
		if i > 0 && r >= 'A' && r <= 'Z' {
			prev := runes[i-1]
			// Check if previous char is lowercase (skip if it's underscore or uppercase)
			if prev >= 'a' && prev <= 'z' {
				result.WriteRune('_')
			} else if prev != '_' && i+1 < len(runes) {
				// Previous is uppercase (but not underscore), check next
				next := runes[i+1]
				if next >= 'a' && next <= 'z' {
					result.WriteRune('_')
				}
			}
		}

		result.WriteRune(r)
	}

	return strings.ToUpper(result.String())
}

// replaceNodeValue finds and replaces a scalar value in the node tree
// Handles array notation (e.g., "field[]" means iterate all array elements)
func (p *parameterizer) replaceNodeValue(node *yaml.Node, path string, replacement string) error {
	parts := strings.Split(path, ".")
	current := node

	for i, part := range parts {
		// Check if this part indicates array access
		isArrayAccess := strings.HasSuffix(part, "[]")
		fieldName := part
		if isArrayAccess {
			fieldName = strings.TrimSuffix(part, "[]")
		}

		if current.Kind != yaml.MappingNode {
			return nil // Path doesn't exist
		}

		found := false
		for j := 0; j < len(current.Content); j += 2 {
			keyNode := current.Content[j]
			valueNode := current.Content[j+1]

			if keyNode.Value == fieldName {
				if isArrayAccess {
					// Handle array access - need to iterate through array elements
					if valueNode.Kind == yaml.SequenceNode {
						// Process each element in the sequence
						for _, elemNode := range valueNode.Content {
							if i == len(parts)-1 {
								// This shouldn't happen for array access without further path
								return nil
							}
							// Continue with remaining path on each element
							remainingPath := strings.Join(parts[i+1:], ".")
							if err := p.replaceNodeValue(elemNode, remainingPath, replacement); err != nil {
								return err
							}
						}
						return nil
					}
				} else if i == len(parts)-1 {
					// Replace the value
					valueNode.Kind = yaml.ScalarNode
					valueNode.Tag = "!!str"
					valueNode.Value = replacement
					valueNode.Style = yaml.LiteralStyle
					return nil
				}
				current = valueNode
				found = true
				break
			}
		}

		if !found {
			return nil // Path doesn't exist
		}
	}

	return nil
}

// replaceArrayNode finds and replaces an array with template range syntax
// Handles array notation (e.g., "field[]" means iterate all array elements)
func (p *parameterizer) replaceArrayNode(node *yaml.Node, path string, varName string) error {
	parts := strings.Split(path, ".")
	current := node

	for i, part := range parts {
		// Check if this part indicates array access
		isArrayAccess := strings.HasSuffix(part, "[]")
		fieldName := part
		if isArrayAccess {
			fieldName = strings.TrimSuffix(part, "[]")
		}

		if current.Kind != yaml.MappingNode {
			return nil // Path doesn't exist
		}

		found := false
		for j := 0; j < len(current.Content); j += 2 {
			keyNode := current.Content[j]
			valueNode := current.Content[j+1]

			if keyNode.Value == fieldName {
				if isArrayAccess {
					// Handle array access - need to iterate through array elements
					if valueNode.Kind == yaml.SequenceNode {
						// Process each element in the sequence
						for _, elemNode := range valueNode.Content {
							if i == len(parts)-1 {
								// This shouldn't happen for array access without further path
								return nil
							}
							// Continue with remaining path on each element
							remainingPath := strings.Join(parts[i+1:], ".")
							if err := p.replaceArrayNode(elemNode, remainingPath, varName); err != nil {
								return err
							}
						}
						return nil
					}
				} else if i == len(parts)-1 {
					// Replace array with template range syntax
					if valueNode.Kind == yaml.SequenceNode {
						// Create a new sequence node with template content
						// Instead of replacing with a scalar, we create template nodes within the sequence

						// Create template range start comment
						rangeStartNode := &yaml.Node{
							Kind:  yaml.ScalarNode,
							Tag:   "!!str",
							Value: fmt.Sprintf("{{- range .%s}}", varName),
							Style: yaml.FlowStyle,
						}

						// Create template item node
						itemNode := &yaml.Node{
							Kind:  yaml.ScalarNode,
							Tag:   "!!str",
							Value: "{{.}}",
							Style: yaml.FlowStyle,
						}

						// Create template range end comment
						rangeEndNode := &yaml.Node{
							Kind:  yaml.ScalarNode,
							Tag:   "!!str",
							Value: "{{- end}}",
							Style: yaml.FlowStyle,
						}

						// Replace the sequence content with template nodes
						valueNode.Content = []*yaml.Node{rangeStartNode, itemNode, rangeEndNode}
					}
					return nil
				}
				current = valueNode
				found = true
				break
			}
		}

		if !found {
			return nil // Path doesn't exist
		}
	}

	return nil
}

// isTemplateSequence checks if a sequence node contains template range syntax
func (p *parameterizer) isTemplateSequence(node *yaml.Node) bool {
	return node.Kind == yaml.SequenceNode &&
		len(node.Content) > 0 &&
		strings.HasPrefix(node.Content[0].Value, "{{- range")
}

// renderTemplateSequence renders a template sequence with range syntax
func (p *parameterizer) renderTemplateSequence(buf *bytes.Buffer, node *yaml.Node, indent int) {
	for _, item := range node.Content {
		if strings.HasPrefix(item.Value, "{{- range") || strings.HasPrefix(item.Value, "{{- end}}") {
			// Range start/end - write at same indent level
			buf.WriteString(strings.Repeat(" ", indent))
			buf.WriteString(item.Value)
			buf.WriteString("\n")
		} else {
			// Item template - write as list item
			buf.WriteString(strings.Repeat(" ", indent))
			buf.WriteString("- ")
			buf.WriteString(item.Value)
			buf.WriteString("\n")
		}
	}
}

// renderMappingValue renders the value part of a mapping key-value pair
func (p *parameterizer) renderMappingValue(buf *bytes.Buffer, valueNode *yaml.Node, indent int) error {
	// Check if value needs special handling
	if valueNode.Kind == yaml.ScalarNode && strings.HasPrefix(valueNode.Value, "{{") {
		if templateVariablePattern.MatchString(valueNode.Value) {
			// Go template parameterization variable (e.g. {{.MY_VAR}}) - write inline without quotes.
			// These are replaced before the file is used as YAML, so quoting is not needed.
			buf.WriteString(" ")
			buf.WriteString(valueNode.Value)
			buf.WriteString("\n")
		} else {
			// Non-parameterization value starting with {{ (e.g. i18n refs like {{ t(key) }}).
			// Must be quoted because bare { is a YAML flow-mapping indicator.
			// Use single-quoted YAML strings to avoid escaping issues with backslashes.
			buf.WriteString(` '`)
			buf.WriteString(strings.ReplaceAll(valueNode.Value, `'`, `''`))
			buf.WriteString(`'`)
			buf.WriteString("\n")
		}
	} else if p.isTemplateSequence(valueNode) {
		// Template sequence - render template syntax
		buf.WriteString("\n")
		p.renderTemplateSequence(buf, valueNode, indent+2)
	} else if valueNode.Kind == yaml.SequenceNode {
		// Regular sequence
		buf.WriteString("\n")
		if err := p.renderNode(buf, valueNode, indent+2); err != nil {
			return err
		}
	} else if valueNode.Kind == yaml.MappingNode {
		// Nested mapping
		buf.WriteString("\n")
		if err := p.renderNode(buf, valueNode, indent+2); err != nil {
			return err
		}
	} else {
		// Regular scalar value
		buf.WriteString(" ")
		buf.WriteString(valueNode.Value)
		buf.WriteString("\n")
	}
	return nil
}

// renderSequenceItemMapping renders an inline mapping within a sequence item
func (p *parameterizer) renderSequenceItemMapping(buf *bytes.Buffer, item *yaml.Node, indent int) error {
	first := true
	for j := 0; j < len(item.Content); j += 2 {
		keyNode := item.Content[j]
		valueNode := item.Content[j+1]

		if !first {
			buf.WriteString(strings.Repeat(" ", indent+2))
		}
		first = false

		buf.WriteString(keyNode.Value)
		buf.WriteString(":")

		if valueNode.Kind == yaml.ScalarNode {
			buf.WriteString(" ")
			val := valueNode.Value
			// Quote values that begin with { or [ (YAML flow-collection indicators) unless
			// they are actual Go template parameterization variables like {{.MY_VAR}}.
			if (strings.HasPrefix(val, "{") || strings.HasPrefix(val, "[")) &&
				!templateVariablePattern.MatchString(val) {
				// Use single-quoted YAML strings to avoid escaping issues with backslashes
				// in values that contain JSON (e.g. meta field with \" sequences).
				buf.WriteString(`'`)
				buf.WriteString(strings.ReplaceAll(val, `'`, `''`))
				buf.WriteString(`'`)
			} else {
				buf.WriteString(val)
			}
			buf.WriteString("\n")
		} else if p.isTemplateSequence(valueNode) {
			// Template sequence
			buf.WriteString("\n")
			p.renderTemplateSequence(buf, valueNode, indent+4)
		} else {
			// Regular sequence, nested mapping, or other
			buf.WriteString("\n")
			if err := p.renderNode(buf, valueNode, indent+4); err != nil {
				return err
			}
		}
	}
	return nil
}

// renderMappingNode renders a mapping node (key-value pairs)
func (p *parameterizer) renderMappingNode(buf *bytes.Buffer, node *yaml.Node, indent int) error {
	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valueNode := node.Content[i+1]

		// Write indentation and key
		buf.WriteString(strings.Repeat(" ", indent))
		buf.WriteString(keyNode.Value)
		buf.WriteString(":")

		// Render the value
		if err := p.renderMappingValue(buf, valueNode, indent); err != nil {
			return err
		}
	}
	return nil
}

// renderSequenceNode renders a sequence node (array items)
func (p *parameterizer) renderSequenceNode(buf *bytes.Buffer, node *yaml.Node, indent int) error {
	for _, item := range node.Content {
		buf.WriteString(strings.Repeat(" ", indent))
		buf.WriteString("- ")

		if item.Kind == yaml.MappingNode {
			// Inline mapping for sequence item
			if err := p.renderSequenceItemMapping(buf, item, indent); err != nil {
				return err
			}
		} else {
			// Scalar value
			buf.WriteString(item.Value)
			buf.WriteString("\n")
		}
	}
	return nil
}

// renderNode renders a yaml.Node to a buffer with custom handling for template syntax
func (p *parameterizer) renderNode(buf *bytes.Buffer, node *yaml.Node, indent int) error {
	switch node.Kind {
	case yaml.DocumentNode:
		// Render document children
		for _, child := range node.Content {
			if err := p.renderNode(buf, child, indent); err != nil {
				return err
			}
		}
	case yaml.MappingNode:
		return p.renderMappingNode(buf, node, indent)
	case yaml.SequenceNode:
		return p.renderSequenceNode(buf, node, indent)
	case yaml.ScalarNode:
		// Just write the value
		buf.WriteString(node.Value)
	}
	return nil
}
