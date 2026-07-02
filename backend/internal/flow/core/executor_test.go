/*
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
)

const (
	testExecutorName = "test-executor"
	testInputName    = "username"
	testInputValue   = "testuser"
)

type ExecutorTestSuite struct {
	suite.Suite
}

func TestExecutorTestSuite(t *testing.T) {
	suite.Run(t, new(ExecutorTestSuite))
}

func (s *ExecutorTestSuite) TestNewExecutor() {
	defaultInputs := []providers.Input{{Identifier: testInputName, Required: true}}
	prerequisites := []providers.Input{{Identifier: userAttributeUserID, Required: true}}

	exec := newExecutor(testExecutorName, providers.ExecutorTypeAuthentication,
		defaultInputs, prerequisites, &providers.ExecutorMeta{})

	s.NotNil(exec)
	s.Equal(testExecutorName, exec.GetName())
	s.Equal(providers.ExecutorTypeAuthentication, exec.GetType())
	s.Equal(defaultInputs, exec.GetDefaultInputs())
	s.Equal(prerequisites, exec.GetPrerequisites())
}

func (s *ExecutorTestSuite) TestGetName() {
	exec := newExecutor(testExecutorName, providers.ExecutorTypeAuthentication,
		nil, nil, &providers.ExecutorMeta{})
	s.Equal(testExecutorName, exec.GetName())
}

func (s *ExecutorTestSuite) TestGetType() {
	exec := newExecutor(testExecutorName, providers.ExecutorTypeAuthentication,
		nil, nil, &providers.ExecutorMeta{})
	s.Equal(providers.ExecutorTypeAuthentication, exec.GetType())
}

func (s *ExecutorTestSuite) TestExecute() {
	exec := newExecutor(testExecutorName, providers.ExecutorTypeAuthentication,
		nil, nil, &providers.ExecutorMeta{})
	ctx := &providers.NodeContext{ExecutionID: "test-flow"}

	resp, err := exec.Execute(ctx)

	s.Nil(err)
	s.Nil(resp)
}

func (s *ExecutorTestSuite) TestGetDefaultInputs() {
	defaultInputs := []providers.Input{
		{Identifier: testInputName, Required: true},
		{Identifier: "password", Required: true},
	}
	exec := newExecutor(testExecutorName, providers.ExecutorTypeAuthentication,
		defaultInputs, nil, &providers.ExecutorMeta{})

	result := exec.GetDefaultInputs()

	s.Equal(defaultInputs, result)
}

func (s *ExecutorTestSuite) TestGetPrerequisites() {
	prerequisites := []providers.Input{{Identifier: userAttributeUserID, Required: true}}
	exec := newExecutor(testExecutorName, providers.ExecutorTypeAuthentication,
		nil, prerequisites, &providers.ExecutorMeta{})

	result := exec.GetPrerequisites()

	s.Equal(prerequisites, result)
}

func (s *ExecutorTestSuite) TestHasRequiredInputs() {
	tests := []struct {
		name              string
		defaultInputs     []providers.Input
		userInputs        map[string]string
		runtimeData       map[string]string
		expectedHasInputs bool
		expectedDataCount int
	}{
		{
			"No inputs provided",
			[]providers.Input{{Identifier: testInputName, Required: true}},
			map[string]string{},
			map[string]string{},
			false,
			1,
		},
		{
			"All data in user input",
			[]providers.Input{{Identifier: testInputName, Required: true}},
			map[string]string{testInputName: testInputValue},
			map[string]string{},
			true,
			0,
		},
		{
			"Data in runtime data",
			[]providers.Input{{Identifier: testInputName, Required: true}},
			map[string]string{},
			map[string]string{testInputName: testInputValue},
			true,
			0,
		},
		{
			"Partial data in user input",
			[]providers.Input{
				{Identifier: testInputName, Required: true},
				{Identifier: "password", Required: true},
			},
			map[string]string{testInputName: testInputValue},
			map[string]string{},
			false,
			1,
		},
		{
			"Empty inputs and empty context",
			[]providers.Input{},
			map[string]string{},
			map[string]string{},
			false,
			0,
		},
		{
			"Data in forwarded data (string)",
			[]providers.Input{{Identifier: testInputName, Required: true}},
			map[string]string{},
			map[string]string{},
			true,
			0,
		},
		{
			"Data in forwarded data (non-string)",
			[]providers.Input{{Identifier: testInputName, Required: true}},
			map[string]string{},
			map[string]string{},
			false,
			1,
		},
		{
			"Partial data with forwarded data",
			[]providers.Input{
				{Identifier: testInputName, Required: true},
				{Identifier: "password", Required: true},
			},
			map[string]string{testInputName: testInputValue},
			map[string]string{},
			true,
			0,
		},
		{
			"All sources empty",
			[]providers.Input{{Identifier: testInputName, Required: true}},
			map[string]string{},
			map[string]string{},
			false,
			1,
		},
		{
			"Optional input prompts once",
			[]providers.Input{{Identifier: "nickname", Required: false}},
			map[string]string{},
			map[string]string{},
			false,
			1,
		},
		{
			"Optional input already prompted",
			[]providers.Input{{Identifier: "nickname", Required: false}},
			map[string]string{},
			map[string]string{common.RuntimeKeyPresentedOptionalInputs: "nickname"},
			true,
			0,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			exec := newExecutor(testExecutorName, providers.ExecutorTypeAuthentication,
				tt.defaultInputs, nil, &providers.ExecutorMeta{})
			ctx := &providers.NodeContext{
				ExecutionID: "test-flow",
				UserInputs:  tt.userInputs,
				RuntimeData: tt.runtimeData,
			}

			if tt.name == "Data in forwarded data (string)" {
				ctx.ForwardedData = map[string]interface{}{
					testInputName: testInputValue,
				}
			} else if tt.name == "Data in forwarded data (non-string)" {
				ctx.ForwardedData = map[string]interface{}{
					testInputName: 123,
				}
			} else if tt.name == "Partial data with forwarded data" {
				ctx.ForwardedData = map[string]interface{}{
					"password": "pass123",
				}
			} else if tt.name == "All sources empty" {
				ctx.ForwardedData = map[string]interface{}{}
			}

			execResp := &providers.ExecutorResponse{}

			result := exec.HasRequiredInputs(ctx, execResp)

			s.Equal(tt.expectedHasInputs, result)
			s.Len(execResp.Inputs, tt.expectedDataCount)
		})
	}
}

func (s *ExecutorTestSuite) newAuthenticatedAuthUser() providers.AuthUser {
	raw := `{"entityReferenceToken":"tok","entityReference":{"entityId":"user-123","entityCategory":"","entityType":"","ouId":""},"attributeToken":"atok","attributes":{"attributes":{"email":{"value":"test@example.com"}}}}` //nolint:lll
	var authUser providers.AuthUser
	err := json.Unmarshal([]byte(raw), &authUser)
	s.Require().NoError(err)
	return authUser
}

func (s *ExecutorTestSuite) TestValidatePrerequisites() {
	tests := []struct {
		name           string
		prerequisites  []providers.Input
		authUser       providers.AuthUser
		setupMock      func(*managermock.AuthnProviderManagerMock)
		userInputs     map[string]string
		runtimeData    map[string]string
		forwardedData  map[string]interface{}
		expectedValid  bool
		expectedStatus providers.ExecutorStatus
		expectError    bool
	}{
		{
			"No prerequisites",
			[]providers.Input{},
			providers.AuthUser{},
			nil,
			map[string]string{},
			map[string]string{},
			nil,
			true,
			"",
			false,
		},
		{
			"UserID prerequisite met via authenticated user",
			[]providers.Input{{Identifier: userAttributeUserID, Required: true}},
			providers.AuthUser{},
			func(m *managermock.AuthnProviderManagerMock) {
				m.EXPECT().GetEntityReference(mock.Anything, mock.Anything).
					Return(providers.AuthUser{}, &providers.EntityReference{EntityID: "user-123"}, nil)
				m.EXPECT().GetUserAttributes(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(providers.AuthUser{}, &providers.AttributesResponse{}, nil)
			},
			map[string]string{},
			map[string]string{},
			nil,
			true,
			"",
			false,
		},
		{
			"UserID prerequisite not met - no authn provider",
			[]providers.Input{{Identifier: userAttributeUserID, Required: true}},
			providers.AuthUser{},
			nil,
			map[string]string{},
			map[string]string{},
			nil,
			false,
			providers.ExecFailure,
			true,
		},
		{
			"Other prerequisite met via user input",
			[]providers.Input{{Identifier: "email", Required: true}},
			providers.AuthUser{},
			nil,
			map[string]string{"email": "test@example.com"},
			map[string]string{},
			nil,
			true,
			"",
			false,
		},
		{
			"Other prerequisite met via runtime data",
			[]providers.Input{{Identifier: "token", Required: true}},
			providers.AuthUser{},
			nil,
			map[string]string{},
			map[string]string{"token": "abc123"},
			nil,
			true,
			"",
			false,
		},
		{
			"Prerequisite not met",
			[]providers.Input{{Identifier: "apiKey", Required: true}},
			providers.AuthUser{},
			nil,
			map[string]string{},
			map[string]string{},
			nil,
			false,
			providers.ExecFailure,
			true,
		},
		{
			"Optional prerequisite not met",
			[]providers.Input{{Identifier: "optionalKey", Required: false}},
			providers.AuthUser{},
			nil,
			map[string]string{},
			map[string]string{},
			nil,
			true,
			"",
			false,
		},
		{
			"Prerequisite met via forwarded data (string)",
			[]providers.Input{{Identifier: "email", Required: true}},
			providers.AuthUser{},
			nil,
			map[string]string{},
			map[string]string{},
			map[string]interface{}{"email": "test@example.com"},
			true,
			"",
			false,
		},
		{
			"Prerequisite not met via forwarded data (non-string)",
			[]providers.Input{{Identifier: "email", Required: true}},
			providers.AuthUser{},
			nil,
			map[string]string{},
			map[string]string{},
			map[string]interface{}{"email": 12345},
			false,
			providers.ExecFailure,
			true,
		},
		{
			"UserID prerequisite met via authenticated user attributes",
			[]providers.Input{{Identifier: "email", Required: true}},
			providers.AuthUser{},
			func(m *managermock.AuthnProviderManagerMock) {
				m.EXPECT().GetEntityReference(mock.Anything, mock.Anything).
					Return(providers.AuthUser{}, &providers.EntityReference{EntityID: "user-123"}, nil)
				m.EXPECT().GetUserAttributes(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(providers.AuthUser{}, &providers.AttributesResponse{
						Attributes: map[string]*providers.AttributeResponse{
							"email": {Value: "test@example.com"},
						},
					}, nil)
			},
			map[string]string{},
			map[string]string{},
			nil,
			true,
			"",
			false,
		},
		{
			"GetEntityReference fails - prerequisite not met",
			[]providers.Input{{Identifier: userAttributeUserID, Required: true}},
			providers.AuthUser{},
			func(m *managermock.AuthnProviderManagerMock) {
				m.EXPECT().GetEntityReference(mock.Anything, mock.Anything).
					Return(providers.AuthUser{}, nil, &tidcommon.InternalServerError)
				m.EXPECT().GetUserAttributes(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(providers.AuthUser{}, &providers.AttributesResponse{}, nil)
			},
			map[string]string{},
			map[string]string{},
			nil,
			false,
			providers.ExecFailure,
			true,
		},
		{
			"GetUserAttributes fails - falls back to other sources",
			[]providers.Input{{Identifier: "email", Required: true}},
			providers.AuthUser{},
			func(m *managermock.AuthnProviderManagerMock) {
				m.EXPECT().GetEntityReference(mock.Anything, mock.Anything).
					Return(providers.AuthUser{}, &providers.EntityReference{EntityID: "user-123"}, nil)
				m.EXPECT().GetUserAttributes(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(providers.AuthUser{}, nil, &tidcommon.InternalServerError)
			},
			map[string]string{"email": "test@example.com"},
			map[string]string{},
			nil,
			true,
			"",
			false,
		},
		{
			"Entity reference empty ID - attribute still checked",
			[]providers.Input{{Identifier: userAttributeUserID, Required: true}},
			providers.AuthUser{},
			func(m *managermock.AuthnProviderManagerMock) {
				m.EXPECT().GetEntityReference(mock.Anything, mock.Anything).
					Return(providers.AuthUser{}, &providers.EntityReference{EntityID: ""}, nil)
				m.EXPECT().GetUserAttributes(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(providers.AuthUser{}, &providers.AttributesResponse{}, nil)
			},
			map[string]string{},
			map[string]string{},
			nil,
			false,
			providers.ExecFailure,
			true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			exec := newExecutor(testExecutorName, providers.ExecutorTypeAuthentication,
				nil, tt.prerequisites, &providers.ExecutorMeta{})

			var authnProvider providers.AuthnProviderManager
			if tt.setupMock != nil {
				authUser := s.newAuthenticatedAuthUser()
				tt.authUser = authUser
				mockProvider := managermock.NewAuthnProviderManagerMock(s.T())
				tt.setupMock(mockProvider)
				authnProvider = mockProvider
			}

			ctx := &providers.NodeContext{
				Context:       context.Background(),
				ExecutionID:   "test-flow",
				AuthUser:      tt.authUser,
				UserInputs:    tt.userInputs,
				RuntimeData:   tt.runtimeData,
				ForwardedData: tt.forwardedData,
			}

			execResp := &providers.ExecutorResponse{}

			result := exec.ValidatePrerequisites(ctx, execResp, authnProvider)

			s.Equal(tt.expectedValid, result)
			s.Equal(tt.expectedStatus, execResp.Status)
			s.Equal(tt.expectError, execResp.Error != nil)
		})
	}
}

func (s *ExecutorTestSuite) TestGetUserIDFromContext() {
	tests := []struct {
		name           string
		authUser       providers.AuthUser
		setupMock      func(*managermock.AuthnProviderManagerMock)
		runtimeData    map[string]string
		userInputs     map[string]string
		expectedUserID string
	}{
		{
			"UserID from runtime data",
			providers.AuthUser{},
			nil,
			map[string]string{userAttributeUserID: "user-456"},
			map[string]string{},
			"user-456",
		},
		{
			"UserID from authenticated user via authn provider",
			providers.AuthUser{},
			func(m *managermock.AuthnProviderManagerMock) {
				m.EXPECT().GetEntityReference(mock.Anything, mock.Anything).
					Return(providers.AuthUser{}, &providers.EntityReference{EntityID: "user-123"}, nil)
			},
			map[string]string{},
			map[string]string{},
			"user-123",
		},
		{
			"Priority: runtime data over authenticated user",
			providers.AuthUser{},
			nil,
			map[string]string{userAttributeUserID: "user-runtime"},
			map[string]string{},
			"user-runtime",
		},
		{
			"No userID available - no authn provider",
			providers.AuthUser{},
			nil,
			map[string]string{},
			map[string]string{},
			"",
		},
		{
			"GetEntityReference fails - returns empty",
			providers.AuthUser{},
			func(m *managermock.AuthnProviderManagerMock) {
				m.EXPECT().GetEntityReference(mock.Anything, mock.Anything).
					Return(providers.AuthUser{}, nil, &tidcommon.InternalServerError)
			},
			map[string]string{},
			map[string]string{},
			"",
		},
		{
			"Entity reference with empty ID - returns empty",
			providers.AuthUser{},
			func(m *managermock.AuthnProviderManagerMock) {
				m.EXPECT().GetEntityReference(mock.Anything, mock.Anything).
					Return(providers.AuthUser{}, &providers.EntityReference{EntityID: ""}, nil)
			},
			map[string]string{},
			map[string]string{},
			"",
		},
		{
			"Nil authn provider with unauthenticated user",
			providers.AuthUser{},
			nil,
			map[string]string{},
			map[string]string{},
			"",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			exec := newExecutor(testExecutorName, providers.ExecutorTypeAuthentication,
				nil, nil, &providers.ExecutorMeta{})

			var authnProvider providers.AuthnProviderManager
			if tt.setupMock != nil {
				authUser := s.newAuthenticatedAuthUser()
				tt.authUser = authUser
				mockProvider := managermock.NewAuthnProviderManagerMock(s.T())
				tt.setupMock(mockProvider)
				authnProvider = mockProvider
			}

			ctx := &providers.NodeContext{
				Context:     context.Background(),
				ExecutionID: "test-flow",
				AuthUser:    tt.authUser,
				RuntimeData: tt.runtimeData,
				UserInputs:  tt.userInputs,
			}

			execResp := &providers.ExecutorResponse{}

			result := exec.GetUserIDFromContext(ctx, execResp, authnProvider)

			s.Equal(tt.expectedUserID, result)
		})
	}
}

func (s *ExecutorTestSuite) TestGetRequiredInputs() {
	tests := []struct {
		name              string
		defaultInputs     []providers.Input
		nodeInputs        []providers.Input
		expectedDataCount int
		expectedContains  []string
	}{
		{
			"No node input, use default only",
			[]providers.Input{{Identifier: testInputName, Required: true}},
			[]providers.Input{},
			1,
			[]string{testInputName},
		},
		{
			"Node input provided, replaces default",
			[]providers.Input{{Identifier: testInputName, Required: true}},
			[]providers.Input{{Identifier: "email", Required: true}},
			1,
			[]string{"email"},
		},
		{
			"Duplicate in node input, no duplication in result",
			[]providers.Input{{Identifier: testInputName, Required: true}},
			[]providers.Input{{Identifier: testInputName, Required: true}},
			1,
			[]string{testInputName},
		},
		{
			"No default inputs, use node input",
			[]providers.Input{},
			[]providers.Input{{Identifier: "custom", Required: false}},
			1,
			[]string{"custom"},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			exec := newExecutor(testExecutorName, providers.ExecutorTypeAuthentication,
				tt.defaultInputs, nil, &providers.ExecutorMeta{})
			ctx := &providers.NodeContext{ExecutionID: "test-flow", NodeInputs: tt.nodeInputs}

			result := exec.GetRequiredInputs(ctx)

			s.Len(result, tt.expectedDataCount)
			for _, name := range tt.expectedContains {
				found := false
				for _, input := range result {
					if input.Identifier == name {
						found = true
						break
					}
				}
				s.True(found)
			}
		})
	}
}

func (s *ExecutorTestSuite) TestGetExecutionPolicy() {
	exec := newExecutor(testExecutorName, providers.ExecutorTypeAuthentication,
		nil, nil, &providers.ExecutorMeta{})

	s.Nil(exec.GetExecutionPolicy("default"))
	s.Nil(exec.GetExecutionPolicy(""))
	s.Nil(exec.GetExecutionPolicy("custom"))
}
