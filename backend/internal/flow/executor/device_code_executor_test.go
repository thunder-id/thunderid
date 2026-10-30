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
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/deviceauth"
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
	verifier.EXPECT().VerifyUserCode(mock.Anything, "BCDF-GHJK").Return(
		&deviceauth.DeviceVerificationResponse{ApplicationID: "stored-app", AuthID: "stored-request",
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
	s.Equal("stored-request", response.RuntimeData[flowcommon.RuntimeKeyAuthorizationRequestID])
	s.Equal(string(providers.GrantTypeDeviceCode), response.RuntimeData[flowcommon.RuntimeKeyCallbackType])
	s.Equal("booking:read", response.RuntimeData[flowcommon.RuntimeKeyRequestedPermissions])
	s.Equal("https://api.example.com", response.RuntimeData[flowcommon.RuntimeKeyResourceServerIdentifier])
	s.Equal("email", response.RuntimeData[flowcommon.RuntimeKeyRequiredOptionalAttributes])
}

func (s *DeviceCodeExecutorTestSuite) TestRepeatsPromptForUnavailableCode() {
	verifier := NewDeviceCodeVerifierMock(s.T())
	verifier.EXPECT().VerifyUserCode(mock.Anything, "INVALID").Return(nil,
		&deviceauth.DeviceAuthError{Code: oauth2const.ErrorInvalidRequest})
	executor := &deviceCodeExecutor{
		verifier: verifier,
	}
	response, err := executor.Execute(&providers.NodeContext{Context: context.Background(),
		UserInputs: map[string]string{deviceUserCodeInput: "INVALID"}})
	s.Require().NoError(err)
	s.Equal(providers.ExecUserInputRequired, response.Status)
	s.Equal("invalid_user_code", response.Error.Code)
	s.Empty(response.RuntimeData)
}

func (s *DeviceCodeExecutorTestSuite) TestReturnsStoreFailure() {
	storeErr := errors.New("store unavailable")
	verifier := NewDeviceCodeVerifierMock(s.T())
	verifier.EXPECT().VerifyUserCode(mock.Anything, "BCDF-GHJK").Return(nil, storeErr)
	executor := &deviceCodeExecutor{
		verifier: verifier,
	}
	response, err := executor.Execute(&providers.NodeContext{Context: context.Background(),
		Application: providers.Application{ID: "stored-app"},
		UserInputs:  map[string]string{deviceUserCodeInput: "BCDF-GHJK"}})
	s.Nil(response)
	s.ErrorIs(err, storeErr)
}
