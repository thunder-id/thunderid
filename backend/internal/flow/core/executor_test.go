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

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
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
	defaultInputs := []common.Input{{Identifier: testInputName, Required: true}}
	prerequisites := []common.Input{{Identifier: userAttributeUserID, Required: true}}

	exec := newExecutor(testExecutorName, common.ExecutorTypeAuthentication, defaultInputs, prerequisites)

	s.NotNil(exec)
	s.Equal(testExecutorName, exec.GetName())
	s.Equal(common.ExecutorTypeAuthentication, exec.GetType())
	s.Equal(defaultInputs, exec.GetDefaultInputs())
	s.Equal(prerequisites, exec.GetPrerequisites())
}

func (s *ExecutorTestSuite) TestGetName() {
	exec := newExecutor(testExecutorName, common.ExecutorTypeAuthentication, nil, nil)
	s.Equal(testExecutorName, exec.GetName())
}

func (s *ExecutorTestSuite) TestGetType() {
	exec := newExecutor(testExecutorName, common.ExecutorTypeAuthentication, nil, nil)
	s.Equal(common.ExecutorTypeAuthentication, exec.GetType())
}

func (s *ExecutorTestSuite) TestExecute() {
	exec := newExecutor(testExecutorName, common.ExecutorTypeAuthentication, nil, nil)
	ctx := &NodeContext{ExecutionID: "test-flow"}

	resp, err := exec.Execute(ctx)

	s.Nil(err)
	s.Nil(resp)
}

func (s *ExecutorTestSuite) TestGetDefaultInputs() {
	defaultInputs := []common.Input{
		{Identifier: testInputName, Required: true},
		{Identifier: "password", Required: true},
	}
	exec := newExecutor(testExecutorName, common.ExecutorTypeAuthentication, defaultInputs, nil)

	result := exec.GetDefaultInputs()

	s.Equal(defaultInputs, result)
}

func (s *ExecutorTestSuite) TestGetPrerequisites() {
	prerequisites := []common.Input{{Identifier: userAttributeUserID, Required: true}}
	exec := newExecutor(testExecutorName, common.ExecutorTypeAuthentication, nil, prerequisites)

	result := exec.GetPrerequisites()

	s.Equal(prerequisites, result)
}

func (s *ExecutorTestSuite) TestHasRequiredInputs() {
	tests := []struct {
		name              string
		defaultInputs     []common.Input
		userInputs        map[string]string
		runtimeData       map[string]string
		expectedHasInputs bool
		expectedDataCount int
	}{
		{
			"No inputs provided",
			[]common.Input{{Identifier: testInputName, Required: true}},
			map[string]string{},
			map[string]string{},
			false,
			1,
		},
		{
			"All data in user input",
			[]common.Input{{Identifier: testInputName, Required: true}},
			map[string]string{testInputName: testInputValue},
			map[string]string{},
			true,
			0,
		},
		{
			"Data in runtime data",
			[]common.Input{{Identifier: testInputName, Required: true}},
			map[string]string{},
			map[string]string{testInputName: testInputValue},
			true,
			0,
		},
		{
			"Partial data in user input",
			[]common.Input{
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
			[]common.Input{},
			map[string]string{},
			map[string]string{},
			false,
			0,
		},
		{
			"Data in forwarded data (string)",
			[]common.Input{{Identifier: testInputName, Required: true}},
			map[string]string{},
			map[string]string{},
			true,
			0,
		},
		{
			"Data in forwarded data (non-string)",
			[]common.Input{{Identifier: testInputName, Required: true}},
			map[string]string{},
			map[string]string{},
			false,
			1,
		},
		{
			"Partial data with forwarded data",
			[]common.Input{
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
			[]common.Input{{Identifier: testInputName, Required: true}},
			map[string]string{},
			map[string]string{},
			false,
			1,
		},
		{
			"Optional input prompts once",
			[]common.Input{{Identifier: "nickname", Required: false}},
			map[string]string{},
			map[string]string{},
			false,
			1,
		},
		{
			"Optional input already prompted",
			[]common.Input{{Identifier: "nickname", Required: false}},
			map[string]string{},
			map[string]string{common.RuntimeKeyPresentedOptionalInputs: "nickname"},
			true,
			0,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			exec := newExecutor(testExecutorName, common.ExecutorTypeAuthentication, tt.defaultInputs, nil)
			ctx := &NodeContext{
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

			execResp := &common.ExecutorResponse{}

			result := exec.HasRequiredInputs(ctx, execResp)

			s.Equal(tt.expectedHasInputs, result)
			s.Len(execResp.Inputs, tt.expectedDataCount)
		})
	}
}

func (s *ExecutorTestSuite) newAuthenticatedAuthUser() manager.AuthUser {
	raw := `{"entityReferenceToken":"tok","entityReference":{"entityId":"user-123","entityCategory":"","entityType":"","ouId":""},"attributeToken":"atok","attributes":{"attributes":{"email":{"value":"test@example.com"}}}}` //nolint:lll
	var authUser manager.AuthUser
	err := json.Unmarshal([]byte(raw), &authUser)
	s.Require().NoError(err)
	return authUser
}

func (s *ExecutorTestSuite) TestValidatePrerequisites() {
	tests := []struct {
		name           string
		prerequisites  []common.Input
		authUser       manager.AuthUser
		setupMock      func(*managermock.AuthnProviderManagerInterfaceMock)
		userInputs     map[string]string
		runtimeData    map[string]string
		forwardedData  map[string]interface{}
		expectedValid  bool
		expectedStatus common.ExecutorStatus
		expectError    bool
	}{
		{
			"No prerequisites",
			[]common.Input{},
			manager.AuthUser{},
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
			[]common.Input{{Identifier: userAttributeUserID, Required: true}},
			manager.AuthUser{},
			func(m *managermock.AuthnProviderManagerInterfaceMock) {
				m.EXPECT().GetEntityReference(mock.Anything, mock.Anything).
					Return(manager.AuthUser{}, &authnprovidercm.EntityReference{EntityID: "user-123"}, nil)
				m.EXPECT().GetUserAttributes(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(manager.AuthUser{}, &authnprovidercm.AttributesResponse{}, nil)
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
			[]common.Input{{Identifier: userAttributeUserID, Required: true}},
			manager.AuthUser{},
			nil,
			map[string]string{},
			map[string]string{},
			nil,
			false,
			common.ExecFailure,
			true,
		},
		{
			"Other prerequisite met via user input",
			[]common.Input{{Identifier: "email", Required: true}},
			manager.AuthUser{},
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
			[]common.Input{{Identifier: "token", Required: true}},
			manager.AuthUser{},
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
			[]common.Input{{Identifier: "apiKey", Required: true}},
			manager.AuthUser{},
			nil,
			map[string]string{},
			map[string]string{},
			nil,
			false,
			common.ExecFailure,
			true,
		},
		{
			"Optional prerequisite not met",
			[]common.Input{{Identifier: "optionalKey", Required: false}},
			manager.AuthUser{},
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
			[]common.Input{{Identifier: "email", Required: true}},
			manager.AuthUser{},
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
			[]common.Input{{Identifier: "email", Required: true}},
			manager.AuthUser{},
			nil,
			map[string]string{},
			map[string]string{},
			map[string]interface{}{"email": 12345},
			false,
			common.ExecFailure,
			true,
		},
		{
			"UserID prerequisite met via authenticated user attributes",
			[]common.Input{{Identifier: "email", Required: true}},
			manager.AuthUser{},
			func(m *managermock.AuthnProviderManagerInterfaceMock) {
				m.EXPECT().GetEntityReference(mock.Anything, mock.Anything).
					Return(manager.AuthUser{}, &authnprovidercm.EntityReference{EntityID: "user-123"}, nil)
				m.EXPECT().GetUserAttributes(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(manager.AuthUser{}, &authnprovidercm.AttributesResponse{
						Attributes: map[string]*authnprovidercm.AttributeResponse{
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
			[]common.Input{{Identifier: userAttributeUserID, Required: true}},
			manager.AuthUser{},
			func(m *managermock.AuthnProviderManagerInterfaceMock) {
				m.EXPECT().GetEntityReference(mock.Anything, mock.Anything).
					Return(manager.AuthUser{}, nil, &serviceerror.InternalServerError)
				m.EXPECT().GetUserAttributes(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(manager.AuthUser{}, &authnprovidercm.AttributesResponse{}, nil)
			},
			map[string]string{},
			map[string]string{},
			nil,
			false,
			common.ExecFailure,
			true,
		},
		{
			"GetUserAttributes fails - falls back to other sources",
			[]common.Input{{Identifier: "email", Required: true}},
			manager.AuthUser{},
			func(m *managermock.AuthnProviderManagerInterfaceMock) {
				m.EXPECT().GetEntityReference(mock.Anything, mock.Anything).
					Return(manager.AuthUser{}, &authnprovidercm.EntityReference{EntityID: "user-123"}, nil)
				m.EXPECT().GetUserAttributes(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(manager.AuthUser{}, nil, &serviceerror.InternalServerError)
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
			[]common.Input{{Identifier: userAttributeUserID, Required: true}},
			manager.AuthUser{},
			func(m *managermock.AuthnProviderManagerInterfaceMock) {
				m.EXPECT().GetEntityReference(mock.Anything, mock.Anything).
					Return(manager.AuthUser{}, &authnprovidercm.EntityReference{EntityID: ""}, nil)
				m.EXPECT().GetUserAttributes(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(manager.AuthUser{}, &authnprovidercm.AttributesResponse{}, nil)
			},
			map[string]string{},
			map[string]string{},
			nil,
			false,
			common.ExecFailure,
			true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			exec := newExecutor(testExecutorName, common.ExecutorTypeAuthentication, nil, tt.prerequisites)

			var authnProvider manager.AuthnProviderManagerInterface
			if tt.setupMock != nil {
				authUser := s.newAuthenticatedAuthUser()
				tt.authUser = authUser
				mockProvider := managermock.NewAuthnProviderManagerInterfaceMock(s.T())
				tt.setupMock(mockProvider)
				authnProvider = mockProvider
			}

			ctx := &NodeContext{
				Context:       context.Background(),
				ExecutionID:   "test-flow",
				AuthUser:      tt.authUser,
				UserInputs:    tt.userInputs,
				RuntimeData:   tt.runtimeData,
				ForwardedData: tt.forwardedData,
			}

			execResp := &common.ExecutorResponse{}

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
		authUser       manager.AuthUser
		setupMock      func(*managermock.AuthnProviderManagerInterfaceMock)
		runtimeData    map[string]string
		userInputs     map[string]string
		expectedUserID string
	}{
		{
			"UserID from runtime data",
			manager.AuthUser{},
			nil,
			map[string]string{userAttributeUserID: "user-456"},
			map[string]string{},
			"user-456",
		},
		{
			"UserID from authenticated user via authn provider",
			manager.AuthUser{},
			func(m *managermock.AuthnProviderManagerInterfaceMock) {
				m.EXPECT().GetEntityReference(mock.Anything, mock.Anything).
					Return(manager.AuthUser{}, &authnprovidercm.EntityReference{EntityID: "user-123"}, nil)
			},
			map[string]string{},
			map[string]string{},
			"user-123",
		},
		{
			"Priority: runtime data over authenticated user",
			manager.AuthUser{},
			nil,
			map[string]string{userAttributeUserID: "user-runtime"},
			map[string]string{},
			"user-runtime",
		},
		{
			"No userID available - no authn provider",
			manager.AuthUser{},
			nil,
			map[string]string{},
			map[string]string{},
			"",
		},
		{
			"GetEntityReference fails - returns empty",
			manager.AuthUser{},
			func(m *managermock.AuthnProviderManagerInterfaceMock) {
				m.EXPECT().GetEntityReference(mock.Anything, mock.Anything).
					Return(manager.AuthUser{}, nil, &serviceerror.InternalServerError)
			},
			map[string]string{},
			map[string]string{},
			"",
		},
		{
			"Entity reference with empty ID - returns empty",
			manager.AuthUser{},
			func(m *managermock.AuthnProviderManagerInterfaceMock) {
				m.EXPECT().GetEntityReference(mock.Anything, mock.Anything).
					Return(manager.AuthUser{}, &authnprovidercm.EntityReference{EntityID: ""}, nil)
			},
			map[string]string{},
			map[string]string{},
			"",
		},
		{
			"Nil authn provider with unauthenticated user",
			manager.AuthUser{},
			nil,
			map[string]string{},
			map[string]string{},
			"",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			exec := newExecutor(testExecutorName, common.ExecutorTypeAuthentication, nil, nil)

			var authnProvider manager.AuthnProviderManagerInterface
			if tt.setupMock != nil {
				authUser := s.newAuthenticatedAuthUser()
				tt.authUser = authUser
				mockProvider := managermock.NewAuthnProviderManagerInterfaceMock(s.T())
				tt.setupMock(mockProvider)
				authnProvider = mockProvider
			}

			ctx := &NodeContext{
				Context:     context.Background(),
				ExecutionID: "test-flow",
				AuthUser:    tt.authUser,
				RuntimeData: tt.runtimeData,
				UserInputs:  tt.userInputs,
			}

			execResp := &common.ExecutorResponse{}

			result := exec.GetUserIDFromContext(ctx, execResp, authnProvider)

			s.Equal(tt.expectedUserID, result)
		})
	}
}

func (s *ExecutorTestSuite) TestGetRequiredInputs() {
	tests := []struct {
		name              string
		defaultInputs     []common.Input
		nodeInputs        []common.Input
		expectedDataCount int
		expectedContains  []string
	}{
		{
			"No node input, use default only",
			[]common.Input{{Identifier: testInputName, Required: true}},
			[]common.Input{},
			1,
			[]string{testInputName},
		},
		{
			"Node input provided, replaces default",
			[]common.Input{{Identifier: testInputName, Required: true}},
			[]common.Input{{Identifier: "email", Required: true}},
			1,
			[]string{"email"},
		},
		{
			"Duplicate in node input, no duplication in result",
			[]common.Input{{Identifier: testInputName, Required: true}},
			[]common.Input{{Identifier: testInputName, Required: true}},
			1,
			[]string{testInputName},
		},
		{
			"No default inputs, use node input",
			[]common.Input{},
			[]common.Input{{Identifier: "custom", Required: false}},
			1,
			[]string{"custom"},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			exec := newExecutor(testExecutorName, common.ExecutorTypeAuthentication, tt.defaultInputs, nil)
			ctx := &NodeContext{ExecutionID: "test-flow", NodeInputs: tt.nodeInputs}

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
	exec := newExecutor(testExecutorName, common.ExecutorTypeAuthentication, nil, nil)

	s.Nil(exec.GetExecutionPolicy("default"))
	s.Nil(exec.GetExecutionPolicy(""))
	s.Nil(exec.GetExecutionPolicy("custom"))
}
