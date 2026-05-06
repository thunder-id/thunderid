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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/asgardeo/thunder/internal/authnprovider/manager"
	"github.com/asgardeo/thunder/internal/flow/common"
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
			true, // Both inputs satisfied (username from UserInputs, password from ForwardedData)
			0,    // No missing inputs
		},
		{
			"All sources empty",
			[]common.Input{{Identifier: testInputName, Required: true}},
			map[string]string{},
			map[string]string{},
			false,
			1,
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

			// Add ForwardedData for specific test cases
			if tt.name == "Data in forwarded data (string)" {
				ctx.ForwardedData = map[string]interface{}{
					testInputName: testInputValue,
				}
			} else if tt.name == "Data in forwarded data (non-string)" {
				ctx.ForwardedData = map[string]interface{}{
					testInputName: 123, // Non-string value
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

func (s *ExecutorTestSuite) TestValidatePrerequisites() {
	tests := []struct {
		name               string
		prerequisites      []common.Input
		authUserID         string
		userInputs         map[string]string
		runtimeData        map[string]string
		expectedValid      bool
		expectedStatus     common.ExecutorStatus
		expectedFailReason string
	}{
		{
			"No prerequisites",
			[]common.Input{},
			"",
			map[string]string{},
			map[string]string{},
			true,
			"",
			"",
		},
		{
			"UserID prerequisite met via authenticated user",
			[]common.Input{{Identifier: userAttributeUserID, Required: true}},
			"user-123",
			map[string]string{},
			map[string]string{},
			true,
			"",
			"",
		},
		{
			"UserID prerequisite not met",
			[]common.Input{{Identifier: userAttributeUserID, Required: true}},
			"",
			map[string]string{},
			map[string]string{},
			false,
			common.ExecFailure,
			"Prerequisite not met: userID",
		},
		{
			"Other prerequisite met via user input",
			[]common.Input{{Identifier: "email", Required: true}},
			"",
			map[string]string{"email": "test@example.com"},
			map[string]string{},
			true,
			"",
			"",
		},
		{
			"Other prerequisite met via runtime data",
			[]common.Input{{Identifier: "token", Required: true}},
			"",
			map[string]string{},
			map[string]string{"token": "abc123"},
			true,
			"",
			"",
		},
		{
			"Prerequisite not met",
			[]common.Input{{Identifier: "apiKey", Required: true}},
			"",
			map[string]string{},
			map[string]string{},
			false,
			common.ExecFailure,
			"Prerequisite not met: apiKey",
		},
		{
			"Optional prerequisite not met",
			[]common.Input{{Identifier: "optionalKey", Required: false}},
			"",
			map[string]string{},
			map[string]string{},
			true,
			"",
			"",
		},
		{
			"Prerequisite met via forwarded data (string)",
			[]common.Input{{Identifier: "email", Required: true}},
			"",
			map[string]string{},
			map[string]string{},
			true,
			"",
			"",
		},
		{
			"Prerequisite not met via forwarded data (non-string)",
			[]common.Input{{Identifier: "email", Required: true}},
			"",
			map[string]string{},
			map[string]string{},
			false,
			common.ExecFailure,
			"Prerequisite not met: email",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			exec := newExecutor(testExecutorName, common.ExecutorTypeAuthentication, nil, tt.prerequisites)
			var authUser manager.AuthUser
			if tt.authUserID != "" {
				_ = json.Unmarshal([]byte(`{"authHistory":[],"userHistory":[{"userId":"`+tt.authUserID+
					`","isValuesIncluded":true}],"userState":"exists"}`), &authUser)
			}
			ctx := &NodeContext{
				ExecutionID: "test-flow",
				AuthUser:    authUser,
				UserInputs:  tt.userInputs,
				RuntimeData: tt.runtimeData,
			}

			// Add ForwardedData for specific test cases
			if tt.name == "Prerequisite met via forwarded data (string)" {
				ctx.ForwardedData = map[string]interface{}{
					"email": "test@example.com",
				}
			} else if tt.name == "Prerequisite not met via forwarded data (non-string)" {
				ctx.ForwardedData = map[string]interface{}{
					"email": 12345, // Non-string value
				}
			}

			execResp := &common.ExecutorResponse{}

			result := exec.ValidatePrerequisites(ctx, execResp)

			s.Equal(tt.expectedValid, result)
			s.Equal(tt.expectedStatus, execResp.Status)
			s.Equal(tt.expectedFailReason, execResp.FailureReason)
		})
	}
}

func (s *ExecutorTestSuite) TestGetUserIDFromContext() {
	tests := []struct {
		name           string
		authUserID     string
		runtimeData    map[string]string
		userInputs     map[string]string
		expectedUserID string
	}{
		{
			"UserID from authenticated user",
			"user-123",
			map[string]string{},
			map[string]string{},
			"user-123",
		},
		{
			"UserID from runtime data",
			"",
			map[string]string{userAttributeUserID: "user-456"},
			map[string]string{},
			"user-456",
		},
		{
			"Priority: authenticated user over runtime data",
			"user-auth",
			map[string]string{userAttributeUserID: "user-runtime"},
			map[string]string{},
			"user-auth",
		},
		{
			"No userID available",
			"",
			map[string]string{},
			map[string]string{},
			"",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			exec := newExecutor(testExecutorName, common.ExecutorTypeAuthentication, nil, nil)
			var authUser manager.AuthUser
			if tt.authUserID != "" {
				_ = json.Unmarshal([]byte(`{"authHistory":[],"userHistory":[{"userId":"`+tt.authUserID+
					`","isValuesIncluded":true}],"userState":"exists"}`), &authUser)
			}
			ctx := &NodeContext{
				AuthUser:    authUser,
				RuntimeData: tt.runtimeData,
				UserInputs:  tt.userInputs,
			}

			result := exec.GetUserIDFromContext(ctx)

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
