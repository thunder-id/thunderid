// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	flowcommon "github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type DeviceCodeExecutorTestSuite struct {
	suite.Suite
}

func TestDeviceCodeExecutorTestSuite(t *testing.T) {
	suite.Run(t, new(DeviceCodeExecutorTestSuite))
}

func (s *DeviceCodeExecutorTestSuite) TestAsksForCode() {
	executor := &deviceCodeExecutor{}
	response, err := executor.Execute(&providers.NodeContext{Context: context.Background(),
		UserInputs: map[string]string{}})
	s.Require().NoError(err)
	s.Equal(providers.ExecUserInputRequired, response.Status)
	s.Equal(deviceUserCodeInput, response.Inputs[0].Identifier)
}

func (s *DeviceCodeExecutorTestSuite) TestUsesStoredRequestIdentity() {
	verifier := NewDeviceCodeVerifierMock(s.T())
	verifier.EXPECT().VerifyUserCode(mock.Anything, "stored-app", "BCDF-GHJK").Return(
		&providers.DeviceVerificationResult{ApplicationID: "stored-app", AuthID: "stored-request",
			RequestedPermissions: "booking:read", ResourceServerIdentifier: "https://api.example.com",
			RequiredOptionalAttributes: "email"}, nil)
	executor := &deviceCodeExecutor{
		verifier: verifier,
	}
	response, err := executor.Execute(&providers.NodeContext{Context: context.Background(),
		Application: providers.Application{ID: "stored-app"},
		UserInputs:  map[string]string{deviceUserCodeInput: "BCDF-GHJK"}})
	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, response.Status)
	s.Equal("stored-request", response.SharedRuntimeData[flowcommon.RuntimeKeyAuthorizationRequestID])
	s.Equal(string(providers.GrantTypeDeviceCode), response.SharedRuntimeData[flowcommon.RuntimeKeyCallbackType])
	s.Equal("booking:read", response.SharedRuntimeData[flowcommon.RuntimeKeyRequestedPermissions])
	s.Equal("https://api.example.com", response.SharedRuntimeData[flowcommon.RuntimeKeyResourceServerIdentifier])
	s.Equal("email", response.SharedRuntimeData[flowcommon.RuntimeKeyRequiredOptionalAttributes])
}

func (s *DeviceCodeExecutorTestSuite) TestRepeatsPromptForUnavailableCode() {
	verifier := NewDeviceCodeVerifierMock(s.T())
	verifier.EXPECT().VerifyUserCode(mock.Anything, "", "INVALID").Return(
		&providers.DeviceVerificationResult{InvalidCode: true}, nil)
	executor := &deviceCodeExecutor{
		verifier: verifier,
	}
	response, err := executor.Execute(&providers.NodeContext{Context: context.Background(),
		UserInputs: map[string]string{deviceUserCodeInput: "INVALID"}})
	s.Require().NoError(err)
	s.Equal(providers.ExecUserInputRequired, response.Status)
	s.Equal("FET-1131", response.Error.Code)
	s.Equal("flows.executor.errors.invalid_device_user_code", response.Error.Error.Key)
	s.Equal("flows.executor.errors.invalid_device_user_code_desc", response.Error.ErrorDescription.Key)
	s.Empty(response.RuntimeData)
}

func (s *DeviceCodeExecutorTestSuite) TestReturnsStoreFailure() {
	storeErr := errors.New("store unavailable")
	verifier := NewDeviceCodeVerifierMock(s.T())
	verifier.EXPECT().VerifyUserCode(mock.Anything, "stored-app", "BCDF-GHJK").Return(nil, storeErr)
	executor := &deviceCodeExecutor{
		verifier: verifier,
	}
	response, err := executor.Execute(&providers.NodeContext{Context: context.Background(),
		Application: providers.Application{ID: "stored-app"},
		UserInputs:  map[string]string{deviceUserCodeInput: "BCDF-GHJK"}})
	s.Nil(response)
	s.ErrorIs(err, storeErr)
}
