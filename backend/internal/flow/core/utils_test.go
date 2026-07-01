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
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
)

type UtilsTestSuite struct {
	suite.Suite
}

func TestUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(UtilsTestSuite))
}

// newAuthenticatedAuthUser creates an AuthUser that returns true for IsAuthenticated()
// by unmarshaling JSON with both entityReferenceToken and attributeToken set.
func newAuthenticatedAuthUser() providers.AuthUser {
	var authUser providers.AuthUser
	data := `{"entityReferenceToken":"token","attributeToken":"token"}`
	_ = json.Unmarshal([]byte(data), &authUser)
	return authUser
}

func (s *UtilsTestSuite) TestResolvePlaceholderWithNilContext() {
	result := ResolvePlaceholder(nil, "test value", nil, nil, nil)
	s.Equal("test value", result)
}

func (s *UtilsTestSuite) TestResolvePlaceholderNoPlaceholder() {
	ctx := &providers.NodeContext{
		RuntimeData: map[string]string{"key1": "value1"},
		UserInputs:  map[string]string{"key2": "value2"},
	}

	result := ResolvePlaceholder(ctx, "plain text without placeholders", nil, nil, nil)
	s.Equal("plain text without placeholders", result)
}

func (s *UtilsTestSuite) TestResolvePlaceholderFromRuntimeData() {
	ctx := &providers.NodeContext{
		RuntimeData: map[string]string{"status": "active", "role": "admin"},
		UserInputs:  map[string]string{},
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Single placeholder", "{{ctx(status)}}", "active"},
		{"Placeholder with text", "User role is {{ctx(role)}}", "User role is admin"},
		{"Multiple placeholders", "{{ctx(status)}}-{{ctx(role)}}", "active-admin"},
		{"No whitespace", "{{ctx(status)}}", "active"},
		{"Extra whitespace", "{{ctx(status)}}", "active"},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			result := ResolvePlaceholder(ctx, tt.input, nil, nil, nil)
			s.Equal(tt.expected, result)
		})
	}
}

func (s *UtilsTestSuite) TestResolvePlaceholderFromUserInputs() {
	ctx := &providers.NodeContext{
		RuntimeData: map[string]string{},
		UserInputs:  map[string]string{"username": "john_doe", "email": "john@example.com"},
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Resolve username", "{{ctx(username)}}", "john_doe"},
		{"Resolve email", "{{ctx(email)}}", "john@example.com"},
		{"Multiple from user input", "{{ctx(username)}} - {{ctx(email)}}", "john_doe - john@example.com"},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			result := ResolvePlaceholder(ctx, tt.input, nil, nil, nil)
			s.Equal(tt.expected, result)
		})
	}
}

func (s *UtilsTestSuite) TestResolvePlaceholderRuntimeTakesPrecedence() {
	ctx := &providers.NodeContext{
		RuntimeData: map[string]string{"key": "runtime_value"},
		UserInputs:  map[string]string{"key": "user_input_value"},
	}

	result := ResolvePlaceholder(ctx, "{{ctx(key)}}", nil, nil, nil)
	s.Equal("runtime_value", result, "RuntimeData should take precedence over UserInputs")
}

func (s *UtilsTestSuite) TestResolvePlaceholderUserIDFromAuthnProvider() {
	mockProvider := managermock.NewAuthnProviderManagerMock(s.T())
	authUser := newAuthenticatedAuthUser()
	ctx := &providers.NodeContext{
		Context:     context.Background(),
		RuntimeData: map[string]string{},
		AuthUser:    authUser,
	}
	execResp := &providers.ExecutorResponse{}
	logger := log.GetLogger()

	mockProvider.On("GetEntityReference", mock.Anything, authUser).
		Return(authUser, &providers.EntityReference{
			EntityID: "user-123",
			OUID:     "ou-456",
		}, nil)

	result := ResolvePlaceholder(ctx, "{{ctx(userId)}}", execResp, mockProvider, logger)
	s.Equal("user-123", result)
}

func (s *UtilsTestSuite) TestResolvePlaceholderUserIDFromRuntimeData() {
	ctx := &providers.NodeContext{
		RuntimeData: map[string]string{"userId": "runtime-user-456"},
	}

	result := ResolvePlaceholder(ctx, "{{ctx(userId)}}", nil, nil, nil)
	s.Equal("runtime-user-456", result)
}

func (s *UtilsTestSuite) TestResolvePlaceholderUserIDRuntimeDataTakesPrecedence() {
	mockProvider := managermock.NewAuthnProviderManagerMock(s.T())
	authUser := newAuthenticatedAuthUser()
	ctx := &providers.NodeContext{
		Context:     context.Background(),
		RuntimeData: map[string]string{"userId": "runtime-user-id"},
		AuthUser:    authUser,
	}
	execResp := &providers.ExecutorResponse{}
	logger := log.GetLogger()

	result := ResolvePlaceholder(ctx, "{{ctx(userId)}}", execResp, mockProvider, logger)
	s.Equal("runtime-user-id", result, "RuntimeData should take precedence over authn provider")
}

func (s *UtilsTestSuite) TestResolvePlaceholderUserIDNotFromUserInputs() {
	ctx := &providers.NodeContext{
		UserInputs:  map[string]string{"userId": "input-user-id"},
		RuntimeData: map[string]string{},
	}

	result := ResolvePlaceholder(ctx, "{{ctx(userId)}}", nil, nil, nil)
	s.Equal("{{ctx(userId)}}", result, "userId should NOT be resolved from UserInputs")
}

func (s *UtilsTestSuite) TestResolvePlaceholderOUIDFromAuthnProvider() {
	mockProvider := managermock.NewAuthnProviderManagerMock(s.T())
	authUser := newAuthenticatedAuthUser()
	ctx := &providers.NodeContext{
		Context:     context.Background(),
		RuntimeData: map[string]string{},
		AuthUser:    authUser,
	}
	execResp := &providers.ExecutorResponse{}
	logger := log.GetLogger()

	mockProvider.On("GetEntityReference", mock.Anything, authUser).
		Return(authUser, &providers.EntityReference{
			EntityID: "user-123",
			OUID:     "ou-123",
		}, nil)

	result := ResolvePlaceholder(ctx, "{{ctx(ouId)}}", execResp, mockProvider, logger)
	s.Equal("ou-123", result)
}

func (s *UtilsTestSuite) TestResolvePlaceholderOUIDFromAuthnProviderWithoutEntityID() {
	mockProvider := managermock.NewAuthnProviderManagerMock(s.T())
	authUser := newAuthenticatedAuthUser()
	ctx := &providers.NodeContext{
		Context:     context.Background(),
		RuntimeData: map[string]string{},
		AuthUser:    authUser,
	}
	execResp := &providers.ExecutorResponse{}
	logger := log.GetLogger()

	mockProvider.On("GetEntityReference", mock.Anything, authUser).
		Return(authUser, &providers.EntityReference{
			OUID: "ou-123",
		}, nil)

	result := ResolvePlaceholder(ctx, "{{ctx(ouId)}}", execResp, mockProvider, logger)
	s.Equal("ou-123", result)
}

func (s *UtilsTestSuite) TestResolvePlaceholderOUIDFromRuntimeData() {
	ctx := &providers.NodeContext{
		RuntimeData: map[string]string{"ouId": "runtime-ou-456"},
	}

	result := ResolvePlaceholder(ctx, "{{ctx(ouId)}}", nil, nil, nil)
	s.Equal("runtime-ou-456", result)
}

func (s *UtilsTestSuite) TestResolvePlaceholderOUIDRuntimeDataTakesPrecedence() {
	mockProvider := managermock.NewAuthnProviderManagerMock(s.T())
	authUser := newAuthenticatedAuthUser()
	ctx := &providers.NodeContext{
		Context:     context.Background(),
		RuntimeData: map[string]string{"ouId": "runtime-ou-id"},
		AuthUser:    authUser,
	}
	execResp := &providers.ExecutorResponse{}
	logger := log.GetLogger()

	result := ResolvePlaceholder(ctx, "{{ctx(ouId)}}", execResp, mockProvider, logger)
	s.Equal("runtime-ou-id", result, "RuntimeData should take precedence over authn provider")
}

func (s *UtilsTestSuite) TestResolvePlaceholderOUIDNotFromUserInputs() {
	ctx := &providers.NodeContext{
		UserInputs:  map[string]string{"ouId": "input-ou-id"},
		RuntimeData: map[string]string{},
	}

	result := ResolvePlaceholder(ctx, "{{ctx(ouId)}}", nil, nil, nil)
	s.Equal("{{ctx(ouId)}}", result, "ouId should NOT be resolved from UserInputs")
}

func (s *UtilsTestSuite) TestResolvePlaceholderUserIDAndOUIDShareSingleFetch() {
	mockProvider := managermock.NewAuthnProviderManagerMock(s.T())
	authUser := newAuthenticatedAuthUser()
	ctx := &providers.NodeContext{
		Context:     context.Background(),
		RuntimeData: map[string]string{},
		AuthUser:    authUser,
	}
	execResp := &providers.ExecutorResponse{}
	logger := log.GetLogger()

	mockProvider.On("GetEntityReference", mock.Anything, authUser).
		Return(authUser, &providers.EntityReference{
			EntityID: "user-789",
			OUID:     "ou-789",
		}, nil).Once()

	result := ResolvePlaceholder(ctx, "{{ctx(userId)}}-{{ctx(ouId)}}", execResp, mockProvider, logger)
	s.Equal("user-789-ou-789", result)
	mockProvider.AssertNumberOfCalls(s.T(), "GetEntityReference", 1)
}

func (s *UtilsTestSuite) TestResolvePlaceholderUserIDWithNilAuthnProvider() {
	authUser := newAuthenticatedAuthUser()
	ctx := &providers.NodeContext{
		Context:     context.Background(),
		RuntimeData: map[string]string{},
		AuthUser:    authUser,
	}

	result := ResolvePlaceholder(ctx, "{{ctx(userId)}}", nil, nil, nil)
	s.Equal("{{ctx(userId)}}", result, "userId should keep placeholder when authnProvider is nil")
}

func (s *UtilsTestSuite) TestResolvePlaceholderUserIDWithUnauthenticatedUser() {
	mockProvider := managermock.NewAuthnProviderManagerMock(s.T())
	ctx := &providers.NodeContext{
		Context:     context.Background(),
		RuntimeData: map[string]string{},
	}
	execResp := &providers.ExecutorResponse{}
	logger := log.GetLogger()

	result := ResolvePlaceholder(ctx, "{{ctx(userId)}}", execResp, mockProvider, logger)
	s.Equal("{{ctx(userId)}}", result, "userId should keep placeholder when user is not authenticated")
}

func (s *UtilsTestSuite) TestResolvePlaceholderKeyNotFound() {
	ctx := &providers.NodeContext{
		RuntimeData: map[string]string{"existing": "value"},
		UserInputs:  map[string]string{},
	}

	result := ResolvePlaceholder(ctx, "{{ctx(nonexistent)}}", nil, nil, nil)
	s.Equal("{{ctx(nonexistent)}}", result, "Non-existent key should keep placeholder as-is")
}

func (s *UtilsTestSuite) TestResolvePlaceholderEmptyValue() {
	ctx := &providers.NodeContext{
		RuntimeData: map[string]string{"empty": ""},
		UserInputs:  map[string]string{"nonempty": "value"},
	}

	// Empty runtime value should fall through to user input (but since key doesn't match, keeps placeholder)
	result := ResolvePlaceholder(ctx, "{{ctx(empty)}}", nil, nil, nil)
	s.Equal("{{ctx(empty)}}", result, "Empty value should not resolve, keeps placeholder")

	// Non-empty user input should be used
	result = ResolvePlaceholder(ctx, "{{ctx(nonempty)}}", nil, nil, nil)
	s.Equal("value", result)
}

func (s *UtilsTestSuite) TestResolvePlaceholderMixedStaticAndDynamic() {
	ctx := &providers.NodeContext{
		RuntimeData: map[string]string{"name": "John"},
		UserInputs:  map[string]string{"action": "login"},
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Prefix static", "User: {{ctx(name)}}", "User: John"},
		{"Suffix static", "{{ctx(name)}} performed action", "John performed action"},
		{"Both ends static", "User {{ctx(name)}} did {{ctx(action)}}", "User John did login"},
		{"URL template", "https://api.example.com/users/{{ctx(name)}}/{{ctx(action)}}",
			"https://api.example.com/users/John/login"},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			result := ResolvePlaceholder(ctx, tt.input, nil, nil, nil)
			s.Equal(tt.expected, result)
		})
	}
}

func (s *UtilsTestSuite) TestResolvePlaceholderWithNilMaps() {
	ctx := &providers.NodeContext{
		RuntimeData: nil,
		UserInputs:  nil,
	}

	// Should not panic with nil maps
	result := ResolvePlaceholder(ctx, "{{ctx(key)}}", nil, nil, nil)
	s.Equal("{{ctx(key)}}", result)
}

func (s *UtilsTestSuite) TestResolvePlaceholderEmptyString() {
	ctx := &providers.NodeContext{
		RuntimeData: map[string]string{"key": "value"},
	}

	result := ResolvePlaceholder(ctx, "", nil, nil, nil)
	s.Equal("", result)
}

func (s *UtilsTestSuite) TestResolvePlaceholderSpecialCharactersInValue() {
	ctx := &providers.NodeContext{
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
		{"URL with special chars", "{{ctx(url)}}", "https://example.com?foo=bar&baz=qux"},
		{"JSON string", "{{ctx(json)}}", `{"key": "value"}`},
		{"Regex pattern", "{{ctx(regex)}}", `^[a-z]+$`},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			result := ResolvePlaceholder(ctx, tt.input, nil, nil, nil)
			s.Equal(tt.expected, result)
		})
	}
}
