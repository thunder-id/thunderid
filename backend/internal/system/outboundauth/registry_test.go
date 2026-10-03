// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package outboundauth

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
)

type RegistryTestSuite struct {
	suite.Suite
}

func TestRegistryTestSuite(t *testing.T) {
	suite.Run(t, new(RegistryTestSuite))
}

// The registry decides which values are encrypted, so a caller changing a returned method must
// not change the registry itself.
func (s *RegistryTestSuite) TestGetMethodReturnsACopyOfFields() {
	method, ok := GetMethod(TypeBasic)
	s.Require().True(ok)
	for i := range method.Fields {
		method.Fields[i].Credential = false
	}

	password, ok := getField(TypeBasic, FieldBasicPassword)
	s.Require().True(ok)
	s.True(password.Credential)
}

// The table is hand-maintained and nothing validates it at startup, by design. A malformed entry
// would otherwise surface only as a misbehaving API, so these invariants are its only guard.
func (s *RegistryTestSuite) TestRegisteredMethodsAreWellFormed() {
	seenTypes := make(map[Type]struct{}, len(registeredMethods))

	for _, method := range registeredMethods {
		s.NotEmpty(method.Type, "every method must declare a type")
		s.NotContains(seenTypes, method.Type, "method %q is registered twice", method.Type)
		seenTypes[method.Type] = struct{}{}
		s.NotEmpty(method.DisplayName, "method %q must have a display name", method.Type)

		seenFields := make(map[string]struct{}, len(method.Fields))
		for _, field := range method.Fields {
			s.NotEmpty(field.Key, "method %q declares a field with no name", method.Type)
			s.NotContains(seenFields, field.Key,
				"method %q declares field %q twice", method.Type, field.Key)
			seenFields[field.Key] = struct{}{}

			s.Equal(fieldTypeText, field.Type,
				"method %q field %q: only text fields are supported today", method.Type, field.Key)
			s.NotEmpty(field.DisplayName,
				"method %q field %q must have a display name", method.Type, field.Key)
		}
	}
}

func (s *RegistryTestSuite) TestGetMethod() {
	method, ok := GetMethod(TypeBasic)
	s.True(ok)
	s.Equal(TypeBasic, method.Type)
	s.Require().Len(method.Fields, 2)

	// Field order is part of the contract: the username comes first.
	s.Equal(FieldBasicUsername, method.Fields[0].Key)
	s.Equal(FieldBasicPassword, method.Fields[1].Key)
	s.False(method.Fields[0].Credential)
	s.True(method.Fields[1].Credential)

	_, ok = GetMethod(Type("bearer"))
	s.False(ok)
}

func (s *RegistryTestSuite) TestGetMethodsPreservesCallerOrderAndSkipsUnknown() {
	result := GetMethods([]Type{TypeBasic, Type("bearer"), TypeNone})
	s.Require().Len(result, 2)
	s.Equal(TypeBasic, result[0].Type)
	s.Equal(TypeNone, result[1].Type)
}

// The API declares the fields of a method as an array. A nil slice would serialize to null and
// break a client that iterates it.
func (s *RegistryTestSuite) TestGetMethodsServesFieldlessMethodAsEmptyArray() {
	result := GetMethods([]Type{TypeNone})
	s.Require().Len(result, 1)
	s.NotNil(result[0].Fields)
	s.Empty(result[0].Fields)

	encoded, err := json.Marshal(result[0])
	s.Require().NoError(err)
	s.Contains(string(encoded), `"properties":[]`)
}
