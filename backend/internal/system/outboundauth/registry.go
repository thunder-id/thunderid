// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package outboundauth

import "slices"

// registeredMethods is the single source of truth for every supported authentication method.
// Adding a method here is all that is needed to make it storable, validatable and
// console-renderable: no other code enumerates methods or field names.
//
// The table is compile-time by design. There is no plugin system in the product, and runtime
// registration would make the supported set depend on initialization order.
var registeredMethods = []Method{
	{
		Type:        TypeNone,
		DisplayName: "{{t(connections:outboundAuth.method.none)}}",
	},
	{
		Type:        TypeBasic,
		DisplayName: "{{t(connections:outboundAuth.method.basic)}}",
		Fields: []Field{
			{
				Name:        FieldBasicUsername,
				Type:        FieldTypeString,
				Required:    true,
				DisplayName: "{{t(connections:outboundAuth.field.username)}}",
			},
			{
				Name:        FieldBasicPassword,
				Type:        FieldTypeString,
				Required:    true,
				Credential:  true,
				DisplayName: "{{t(connections:outboundAuth.field.password)}}",
			},
		},
	},
}

// GetMethod returns the method registered for authType. Its fields are a copy, so a caller
// cannot alter the registry, and with it which values are encrypted, through the result.
func GetMethod(authType Type) (Method, bool) {
	for _, method := range registeredMethods {
		if method.Type == authType {
			method.Fields = slices.Clone(method.Fields)
			return method, true
		}
	}
	return Method{}, false
}

// GetMethods returns the methods for the given types, in the order given, skipping any that
// names no registered method. Callers pass the set of methods they support and serve the result.
//
// A method that takes no fields carries an empty slice rather than a nil one: this result is
// serialized straight to the API, where the field is declared as an array, and a nil slice would
// reach the client as null.
func GetMethods(types []Type) []Method {
	result := make([]Method, 0, len(types))
	for _, authType := range types {
		if method, ok := GetMethod(authType); ok {
			if method.Fields == nil {
				method.Fields = []Field{}
			}
			result = append(result, method)
		}
	}
	return result
}

// GetTypes returns the type of every registered method, in registry order. A transport binding
// uses it to prove it has considered every method, so a method it cannot carry is caught by that
// binding's own test rather than by a failed send.
func GetTypes() []Type {
	types := make([]Type, 0, len(registeredMethods))
	for _, method := range registeredMethods {
		types = append(types, method.Type)
	}
	return types
}

// getField returns the named field of authType.
func getField(authType Type, name string) (Field, bool) {
	method, ok := GetMethod(authType)
	if !ok {
		return Field{}, false
	}
	for _, field := range method.Fields {
		if field.Name == name {
			return field, true
		}
	}
	return Field{}, false
}
