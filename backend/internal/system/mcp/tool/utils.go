/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

// Package tool provides utility functions for MCP schema generation and manipulation.
package tool

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/thunder-id/thunderid/internal/system/log"
)

// GenerateSchema creates a JSON schema for a given type T and applies the provided modifiers.
func GenerateSchema[T any](modifiers ...func(*jsonschema.Schema)) *jsonschema.Schema {
	schema, err := jsonschema.For[T](&jsonschema.ForOptions{})
	if err != nil {
		log.GetLogger().Error("Failed to generate schema",
			log.String("type", fmt.Sprintf("%T", *new(T))),
			log.Error(err))
		return nil
	}

	for _, mod := range modifiers {
		mod(schema)
	}

	return schema
}

// WithDefault applies a default value to a property in the schema.
func WithDefault(parent, key string, value any) func(*jsonschema.Schema) {
	return func(root *jsonschema.Schema) {
		targetSchema := findSchemaByPath(root, parent)

		if targetSchema != nil && targetSchema.Properties != nil {
			if prop, ok := targetSchema.Properties[key]; ok {
				if raw, err := json.Marshal(value); err == nil {
					prop.Default = raw
				}
			}
		}
	}
}

// WithEnum applies enum constraints to a property in the schema.
func WithEnum(parent, child string, values []string) func(*jsonschema.Schema) {
	return func(root *jsonschema.Schema) {
		targetSchema := findSchemaByPath(root, parent)

		if targetSchema != nil && targetSchema.Properties != nil {
			if prop, ok := targetSchema.Properties[child]; ok {
				applyEnum(prop, values)
				// Reassign to ensure map update
				targetSchema.Properties[child] = prop
			}
		}
	}
}

// WithRequired marks the specified fields as required in the schema.
func WithRequired(parent string, fields ...string) func(*jsonschema.Schema) {
	return func(root *jsonschema.Schema) {
		targetSchema := findSchemaByPath(root, parent)

		if targetSchema != nil {
			for _, f := range fields {
				if _, ok := targetSchema.Properties[f]; ok && !slices.Contains(targetSchema.Required, f) {
					targetSchema.Required = append(targetSchema.Required, f)
				}
			}
		}
	}
}

// WithRemove removes specified fields from the schema properties.
func WithRemove(parent string, fields ...string) func(*jsonschema.Schema) {
	return func(root *jsonschema.Schema) {
		targetSchema := findSchemaByPath(root, parent)

		if targetSchema != nil && targetSchema.Properties != nil {
			for _, f := range fields {
				delete(targetSchema.Properties, f)
				targetSchema.Required = slices.DeleteFunc(targetSchema.Required, func(r string) bool {
					return r == f
				})
			}
		}
	}
}

// findSchemaByPath traverses the schema using a dot-separated path.
// Returns the schema node at the end of the path.
func findSchemaByPath(root *jsonschema.Schema, path string) *jsonschema.Schema {
	if path == "" {
		return root
	}

	parts := strings.Split(path, ".")
	current := root

	for _, part := range parts {
		if current == nil || current.Properties == nil {
			return nil
		}

		prop, ok := current.Properties[part]
		if !ok {
			return nil
		}

		// If property is an array/list
		if prop.Items != nil {
			current = prop.Items
		} else {
			current = prop
		}
	}

	return current
}

// Helper to apply enum values to a schema property
func applyEnum(prop *jsonschema.Schema, values []string) {
	if prop.Items != nil {
		prop.Items.Enum = stringSliceToAny(values)
	} else {
		prop.Enum = stringSliceToAny(values)
	}
}

// stringSliceToAny converts a slice of strings to a slice of empty interfaces.
func stringSliceToAny(strings []string) []any {
	anys := make([]any, len(strings))
	for i, s := range strings {
		anys[i] = s
	}
	return anys
}
