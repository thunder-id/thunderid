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

package core

import (
	"testing"

	"github.com/stretchr/testify/suite"

	authncm "github.com/thunder-id/thunderid/internal/authn/common"
)

type UtilsTestSuite struct {
	suite.Suite
}

func TestUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(UtilsTestSuite))
}

func (s *UtilsTestSuite) TestResolvePlaceholderWithNilContext() {
	result := ResolvePlaceholder(nil, "test value")
	s.Equal("test value", result)
}

func (s *UtilsTestSuite) TestResolvePlaceholderNoPlaceholder() {
	ctx := &NodeContext{
		RuntimeData: map[string]string{"key1": "value1"},
		UserInputs:  map[string]string{"key2": "value2"},
	}

	result := ResolvePlaceholder(ctx, "plain text without placeholders")
	s.Equal("plain text without placeholders", result)
}

func (s *UtilsTestSuite) TestResolvePlaceholderFromRuntimeData() {
	ctx := &NodeContext{
		RuntimeData: map[string]string{"status": "active", "role": "admin"},
		UserInputs:  map[string]string{},
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Single placeholder", "{{ context.status }}", "active"},
		{"Placeholder with text", "User role is {{ context.role }}", "User role is admin"},
		{"Multiple placeholders", "{{ context.status }}-{{ context.role }}", "active-admin"},
		{"No whitespace", "{{context.status}}", "active"},
		{"Extra whitespace", "{{  context.status  }}", "active"},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			result := ResolvePlaceholder(ctx, tt.input)
			s.Equal(tt.expected, result)
		})
	}
}

func (s *UtilsTestSuite) TestResolvePlaceholderFromUserInputs() {
	ctx := &NodeContext{
		RuntimeData: map[string]string{},
		UserInputs:  map[string]string{"username": "john_doe", "email": "john@example.com"},
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Resolve username", "{{ context.username }}", "john_doe"},
		{"Resolve email", "{{ context.email }}", "john@example.com"},
		{"Multiple from user input", "{{ context.username }} - {{ context.email }}", "john_doe - john@example.com"},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			result := ResolvePlaceholder(ctx, tt.input)
			s.Equal(tt.expected, result)
		})
	}
}

func (s *UtilsTestSuite) TestResolvePlaceholderRuntimeTakesPrecedence() {
	ctx := &NodeContext{
		RuntimeData: map[string]string{"key": "runtime_value"},
		UserInputs:  map[string]string{"key": "user_input_value"},
	}

	result := ResolvePlaceholder(ctx, "{{ context.key }}")
	s.Equal("runtime_value", result, "RuntimeData should take precedence over UserInputs")
}

func (s *UtilsTestSuite) TestResolvePlaceholderUserIDFromAuthenticatedUser() {
	ctx := &NodeContext{
		RuntimeData: map[string]string{},
		AuthenticatedUser: authncm.AuthenticatedUser{
			UserID: "user-123",
		},
	}

	result := ResolvePlaceholder(ctx, "{{ context.userId }}")
	s.Equal("user-123", result)
}

func (s *UtilsTestSuite) TestResolvePlaceholderUserIDFromRuntimeData() {
	ctx := &NodeContext{
		RuntimeData: map[string]string{"userId": "runtime-user-456"},
		AuthenticatedUser: authncm.AuthenticatedUser{
			UserID: "",
		},
	}

	result := ResolvePlaceholder(ctx, "{{ context.userId }}")
	s.Equal("runtime-user-456", result)
}

func (s *UtilsTestSuite) TestResolvePlaceholderUserIDAuthenticatedUserTakesPrecedence() {
	ctx := &NodeContext{
		RuntimeData: map[string]string{"userId": "runtime-user-id"},
		AuthenticatedUser: authncm.AuthenticatedUser{
			UserID: "auth-user-id",
		},
	}

	result := ResolvePlaceholder(ctx, "{{ context.userId }}")
	s.Equal("auth-user-id", result, "AuthenticatedUser.UserID should take precedence over RuntimeData")
}

func (s *UtilsTestSuite) TestResolvePlaceholderUserIDNotFromUserInputs() {
	ctx := &NodeContext{
		UserInputs:  map[string]string{"userId": "input-user-id"},
		RuntimeData: map[string]string{},
		AuthenticatedUser: authncm.AuthenticatedUser{
			UserID: "",
		},
	}

	result := ResolvePlaceholder(ctx, "{{ context.userId }}")
	s.Equal("{{ context.userId }}", result, "userId should NOT be resolved from UserInputs")
}

func (s *UtilsTestSuite) TestResolvePlaceholderOUIDFromAuthenticatedUser() {
	ctx := &NodeContext{
		RuntimeData: map[string]string{},
		AuthenticatedUser: authncm.AuthenticatedUser{
			OUID: "ou-123",
		},
	}

	result := ResolvePlaceholder(ctx, "{{ context.ouId }}")
	s.Equal("ou-123", result)
}

func (s *UtilsTestSuite) TestResolvePlaceholderOUIDFromRuntimeData() {
	ctx := &NodeContext{
		RuntimeData: map[string]string{"ouId": "runtime-ou-456"},
		AuthenticatedUser: authncm.AuthenticatedUser{
			OUID: "",
		},
	}

	result := ResolvePlaceholder(ctx, "{{ context.ouId }}")
	s.Equal("runtime-ou-456", result)
}

func (s *UtilsTestSuite) TestResolvePlaceholderOUIDAuthenticatedUserTakesPrecedence() {
	ctx := &NodeContext{
		RuntimeData: map[string]string{"ouId": "runtime-ou-id"},
		AuthenticatedUser: authncm.AuthenticatedUser{
			OUID: "auth-ou-id",
		},
	}

	result := ResolvePlaceholder(ctx, "{{ context.ouId }}")
	s.Equal("auth-ou-id", result, "AuthenticatedUser.OUID should take precedence over RuntimeData")
}

func (s *UtilsTestSuite) TestResolvePlaceholderOUIDNotFromUserInputs() {
	ctx := &NodeContext{
		UserInputs:  map[string]string{"ouId": "input-ou-id"},
		RuntimeData: map[string]string{},
		AuthenticatedUser: authncm.AuthenticatedUser{
			OUID: "",
		},
	}

	result := ResolvePlaceholder(ctx, "{{ context.ouId }}")
	s.Equal("{{ context.ouId }}", result, "ouId should NOT be resolved from UserInputs")
}

func (s *UtilsTestSuite) TestResolvePlaceholderKeyNotFound() {
	ctx := &NodeContext{
		RuntimeData: map[string]string{"existing": "value"},
		UserInputs:  map[string]string{},
	}

	result := ResolvePlaceholder(ctx, "{{ context.nonexistent }}")
	s.Equal("{{ context.nonexistent }}", result, "Non-existent key should keep placeholder as-is")
}

func (s *UtilsTestSuite) TestResolvePlaceholderEmptyValue() {
	ctx := &NodeContext{
		RuntimeData: map[string]string{"empty": ""},
		UserInputs:  map[string]string{"nonempty": "value"},
	}

	// Empty runtime value should fall through to user input (but since key doesn't match, keeps placeholder)
	result := ResolvePlaceholder(ctx, "{{ context.empty }}")
	s.Equal("{{ context.empty }}", result, "Empty value should not resolve, keeps placeholder")

	// Non-empty user input should be used
	result = ResolvePlaceholder(ctx, "{{ context.nonempty }}")
	s.Equal("value", result)
}

func (s *UtilsTestSuite) TestResolvePlaceholderMixedStaticAndDynamic() {
	ctx := &NodeContext{
		RuntimeData: map[string]string{"name": "John"},
		UserInputs:  map[string]string{"action": "login"},
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Prefix static", "User: {{ context.name }}", "User: John"},
		{"Suffix static", "{{ context.name }} performed action", "John performed action"},
		{"Both ends static", "User {{ context.name }} did {{ context.action }}", "User John did login"},
		{"URL template", "https://api.example.com/users/{{ context.name }}/{{ context.action }}",
			"https://api.example.com/users/John/login"},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			result := ResolvePlaceholder(ctx, tt.input)
			s.Equal(tt.expected, result)
		})
	}
}

func (s *UtilsTestSuite) TestResolvePlaceholderWithNilMaps() {
	ctx := &NodeContext{
		RuntimeData: nil,
		UserInputs:  nil,
	}

	// Should not panic with nil maps
	result := ResolvePlaceholder(ctx, "{{ context.key }}")
	s.Equal("{{ context.key }}", result)
}

func (s *UtilsTestSuite) TestResolvePlaceholderEmptyString() {
	ctx := &NodeContext{
		RuntimeData: map[string]string{"key": "value"},
	}

	result := ResolvePlaceholder(ctx, "")
	s.Equal("", result)
}

func (s *UtilsTestSuite) TestResolvePlaceholderSpecialCharactersInValue() {
	ctx := &NodeContext{
		RuntimeData: map[string]string{
			"url":   "https://example.com?foo=bar&baz=qux",
			"json":  `{"key": "value"}`,
			"regex": `^[a-z]+$`,
		},
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"URL with special chars", "{{ context.url }}", "https://example.com?foo=bar&baz=qux"},
		{"JSON string", "{{ context.json }}", `{"key": "value"}`},
		{"Regex pattern", "{{ context.regex }}", `^[a-z]+$`},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			result := ResolvePlaceholder(ctx, tt.input)
			s.Equal(tt.expected, result)
		})
	}
}
