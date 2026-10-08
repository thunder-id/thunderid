// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package outboundauth

import (
	"slices"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	i18nmgt "github.com/thunder-id/thunderid/internal/system/i18n/mgt"
)

// Display labels of the registered methods and fields. They are served as references to the
// system i18n namespace, so their default translations ship with the server.
var (
	labelMethodNone    = tidcommon.I18nMessage{Key: "outboundauth.method.none", DefaultValue: "None"}
	labelMethodBasic   = tidcommon.I18nMessage{Key: "outboundauth.method.basic", DefaultValue: "Username and Password"}
	labelFieldUsername = tidcommon.I18nMessage{Key: "outboundauth.field.username", DefaultValue: "Username"}
	labelFieldPassword = tidcommon.I18nMessage{Key: "outboundauth.field.password", DefaultValue: "Password"}
)

// registeredMethods is the single source of truth for every supported authentication method.
// Adding a method here is all that is needed to make it storable, validatable and servable to
// clients: no other code enumerates methods or field names.
//
// The table is compile-time by design. There is no plugin system in the product, and runtime
// registration would make the supported set depend on initialization order.
var registeredMethods = []Method{
	{
		Type:        TypeNone,
		DisplayName: systemI18nRef(labelMethodNone),
	},
	{
		Type:        TypeBasic,
		DisplayName: systemI18nRef(labelMethodBasic),
		Fields: []Field{
			{
				Key:         FieldBasicUsername,
				Type:        fieldTypeText,
				Required:    true,
				DisplayName: systemI18nRef(labelFieldUsername),
			},
			{
				Key:         FieldBasicPassword,
				Type:        fieldTypeText,
				Required:    true,
				Credential:  true,
				DisplayName: systemI18nRef(labelFieldPassword),
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

// systemI18nRef returns the i18n template reference to msg in the system namespace.
func systemI18nRef(msg tidcommon.I18nMessage) string {
	return "{{t(" + i18nmgt.SystemNamespace + ":" + msg.Key + ")}}"
}
