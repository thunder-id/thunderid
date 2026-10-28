// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	"github.com/stretchr/testify/suite"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
)

type UtilsTestSuite struct {
	suite.Suite
}

func TestUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(UtilsTestSuite))
}

func (s *UtilsTestSuite) TestParseExtensionAuthResolverProperties_ReadsEveryProperty() {
	props, invalid := ParseExtensionAuthResolverProperties(map[string]interface{}{
		NodePropertySourceKey:      " extension.pin_check.verifiedUsername ",
		NodePropertyMatchAttribute: " username ",
		NodePropertyAttributeMapping: map[string]interface{}{
			" email ": " extension.pin_check.email ",
		},
	})

	s.Empty(invalid)
	s.Equal(ExtensionAuthResolverProperties{
		SourceKey:        "extension.pin_check.verifiedUsername",
		MatchAttribute:   UserAttributeUsername,
		AttributeMapping: map[string]string{"email": "extension.pin_check.email"},
	}, props)

	props, invalid = ParseExtensionAuthResolverProperties(map[string]interface{}{
		NodePropertySourceKey:                           "extension.pin_check.verifiedUserId",
		NodePropertyMatchAttribute:                      "userID",
		NodePropertyAllowAuthenticationWithoutLocalUser: false,
		NodePropertyAttributeMapping:                    map[string]interface{}{},
	})

	s.Empty(invalid)
	s.Equal(authnprovidercm.UserAttributeUserID, props.MatchAttribute)
}

func (s *UtilsTestSuite) TestParseExtensionAuthResolverProperties_Defaults() {
	for name, properties := range map[string]map[string]interface{}{
		"absent": {NodePropertySourceKey: "extension.pin_check.username"},
		"null": {NodePropertySourceKey: "extension.pin_check.username", NodePropertyMatchAttribute: nil,
			NodePropertyAttributeMapping: nil},
		"blank": {NodePropertySourceKey: "extension.pin_check.username", NodePropertyMatchAttribute: " "},
	} {
		s.Run(name, func() {
			props, invalid := ParseExtensionAuthResolverProperties(properties)

			s.Empty(invalid)
			s.Equal(UserAttributeUsername, props.MatchAttribute)
			s.Nil(props.AttributeMapping)
		})
	}
}

func (s *UtilsTestSuite) TestParseExtensionAuthResolverProperties_RefusesUnusableValues() {
	const sourceKey = "extension.pin_check.username"
	cases := []struct {
		name       string
		properties map[string]interface{}
		invalid    string
	}{
		{"no sourceKey", map[string]interface{}{}, NodePropertySourceKey},
		{"blank sourceKey", map[string]interface{}{NodePropertySourceKey: " "}, NodePropertySourceKey},
		{"non-string sourceKey", map[string]interface{}{NodePropertySourceKey: 1}, NodePropertySourceKey},
		{"bare sourceKey", map[string]interface{}{NodePropertySourceKey: "username"}, NodePropertySourceKey},
		{"sourceKey without a field", map[string]interface{}{NodePropertySourceKey: "extension.pin_check."},
			NodePropertySourceKey},
		{"sourceKey without a node", map[string]interface{}{NodePropertySourceKey: "extension..username"},
			NodePropertySourceKey},
		{"sourceKey of the namespace only", map[string]interface{}{NodePropertySourceKey: "extension.pin_check"},
			NodePropertySourceKey},
		{"non-string matchAttribute", map[string]interface{}{NodePropertySourceKey: sourceKey,
			NodePropertyMatchAttribute: true}, NodePropertyMatchAttribute},
		{"attribute other than username or userID", map[string]interface{}{NodePropertySourceKey: sourceKey,
			NodePropertyMatchAttribute: "email"}, NodePropertyMatchAttribute},
		{"attributeMapping not a map", map[string]interface{}{NodePropertySourceKey: sourceKey,
			NodePropertyAttributeMapping: "email"}, NodePropertyAttributeMapping},
		{"attributeMapping bare key", map[string]interface{}{NodePropertySourceKey: sourceKey,
			NodePropertyAttributeMapping: map[string]interface{}{"email": "email"}}, NodePropertyAttributeMapping},
		{"attributeMapping non-string key", map[string]interface{}{NodePropertySourceKey: sourceKey,
			NodePropertyAttributeMapping: map[string]interface{}{"email": 1}}, NodePropertyAttributeMapping},
		{"attributeMapping blank attribute", map[string]interface{}{NodePropertySourceKey: sourceKey,
			NodePropertyAttributeMapping: map[string]interface{}{" ": sourceKey}}, NodePropertyAttributeMapping},
		{"allowAuthenticationWithoutLocalUser with userID", map[string]interface{}{NodePropertySourceKey: sourceKey,
			NodePropertyMatchAttribute: "userID", NodePropertyAllowAuthenticationWithoutLocalUser: true},
			NodePropertyAllowAuthenticationWithoutLocalUser},
		{"attributeMapping with userID", map[string]interface{}{NodePropertySourceKey: sourceKey,
			NodePropertyMatchAttribute:   "userID",
			NodePropertyAttributeMapping: map[string]interface{}{"email": sourceKey}}, NodePropertyAttributeMapping},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			_, invalid := ParseExtensionAuthResolverProperties(tc.properties)

			s.Equal(tc.invalid, invalid)
		})
	}
}
