// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"
	"strings"
)

// DeepCopyMapOfStrings creates a deep copy of a map with strings.
func DeepCopyMapOfStrings(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// DeepCopyMapOfStringSlices creates a deep copy of a map of string slices.
func DeepCopyMapOfStringSlices(src map[string][]string) map[string][]string {
	if src == nil {
		return nil
	}
	dst := make(map[string][]string, len(src))
	for k, v := range src {
		copied := append([]string(nil), v...)
		dst[k] = copied
	}
	return dst
}

// ClonableInterface defines an interface for clonable types.
type ClonableInterface interface {
	Clone() (ClonableInterface, error)
}

// DeepCopyMapOfClonables creates a deep copy of a map with clonable values.
func DeepCopyMapOfClonables[T ClonableInterface](src map[string]T) (map[string]T, error) {
	if src == nil {
		return nil, nil
	}
	dst := make(map[string]T, len(src))
	for k, v := range src {
		cloned, err := v.Clone()
		if err != nil {
			return nil, fmt.Errorf("failed to clone value for key %s: %w", k, err)
		}
		if _, ok := cloned.(T); !ok {
			return nil, fmt.Errorf("cloned value for key %s is not of type: %T", k, cloned)
		}
		dst[k] = cloned.(T)
	}
	return dst, nil
}

// DeepCopyMap creates a deep copy of a map with interface{} values.
// It recursively copies nested maps and slices.
func DeepCopyMap(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return nil
	}
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = DeepCopyInterface(v)
	}
	return dst
}

// DeepCopyInterface creates a deep copy of an interface{} value.
// It recursively copies nested maps and slices. For primitive types, a direct copy is returned.
func DeepCopyInterface(v interface{}) interface{} {
	if v == nil {
		return nil
	}

	switch val := v.(type) {
	case map[string]interface{}:
		return DeepCopyMap(val)
	case []interface{}:
		copied := make([]interface{}, len(val))
		for i, item := range val {
			copied[i] = DeepCopyInterface(item)
		}
		return copied
	case []string:
		return append([]string(nil), val...)
	default:
		// For primitive types (string, int, float, bool, etc.), direct assignment is sufficient
		return v
	}
}

// MergeInterfaceMaps merges two maps of interface{} and returns the result.
func MergeInterfaceMaps(dst, src map[string]interface{}) map[string]interface{} {
	if dst == nil {
		dst = make(map[string]interface{})
	}
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// GetNestedValue resolves a value by exact key first, then by dot-notation path through nested maps.
func GetNestedValue(data map[string]interface{}, path string) (interface{}, bool) {
	if value, ok := data[path]; ok {
		return value, true
	}
	if !strings.Contains(path, ".") {
		return nil, false
	}

	current := interface{}(data)
	for _, segment := range strings.Split(path, ".") {
		obj, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		value, exists := obj[segment]
		if !exists {
			return nil, false
		}
		current = value
	}
	return current, true
}

// UniqueStrings returns a slice containing only unique values from the input slice.
// The order of elements is not guaranteed.
func UniqueStrings(input []string) []string {
	if input == nil {
		return nil
	}

	seen := make(map[string]bool, len(input))
	result := make([]string, 0, len(input))

	for _, item := range input {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}

	return result
}

// UniqueNonEmptyStrings returns a slice containing only unique, non-empty values from the input slice.
func UniqueNonEmptyStrings(input []string) []string {
	seen := make(map[string]struct{}, len(input))
	var result []string
	for _, s := range input {
		if s != "" {
			if _, ok := seen[s]; !ok {
				seen[s] = struct{}{}
				result = append(result, s)
			}
		}
	}

	return result
}

// MergeUniqueStrings returns the union of a and b with duplicates removed, preserving a's order
// followed by any new values from b.
func MergeUniqueStrings(a, b []string) []string {
	if len(b) == 0 {
		return a
	}
	seen := make(map[string]bool, len(a)+len(b))
	merged := make([]string, 0, len(a)+len(b))
	for _, v := range a {
		if !seen[v] {
			seen[v] = true
			merged = append(merged, v)
		}
	}
	for _, v := range b {
		if !seen[v] {
			seen[v] = true
			merged = append(merged, v)
		}
	}
	return merged
}
