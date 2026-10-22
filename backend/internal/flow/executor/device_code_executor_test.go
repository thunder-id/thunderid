// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	flowcommon "github.com/thunder-id/thunderid/internal/flow/common"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/deviceauth"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type deviceCodeVerifierStub struct {
	verify func(context.Context, string) (*deviceauth.DeviceVerificationResponse, error)
}

func (s deviceCodeVerifierStub) VerifyUserCode(
	ctx context.Context, code string,
) (*deviceauth.DeviceVerificationResponse, error) {
	return s.verify(ctx, code)
}

func TestDeviceCodeExecutorAsksForCode(t *testing.T) {
	executor := &deviceCodeExecutor{}
	response, err := executor.Execute(&providers.NodeContext{Context: context.Background(),
		UserInputs: map[string]string{}})
	require.NoError(t, err)
	require.Equal(t, providers.ExecUserInputRequired, response.Status)
	require.Equal(t, deviceUserCodeInput, response.Inputs[0].Identifier)
}

func TestDeviceCodeExecutorUsesStoredRequestIdentity(t *testing.T) {
	executor := &deviceCodeExecutor{
		verifier: deviceCodeVerifierStub{
			verify: func(_ context.Context, code string) (*deviceauth.DeviceVerificationResponse, error) {
				require.Equal(t, "BCDF-GHJK", code)
				return &deviceauth.DeviceVerificationResponse{ApplicationID: "stored-app", AuthID: "stored-request",
					ApplicationAuthFlowID: "authentication-flow-1",
					RequestedPermissions:  "booking:read", ResourceServerIdentifier: "https://api.example.com",
					RequiredOptionalAttributes: "email"}, nil
			},
		},
	}
	response, err := executor.Execute(&providers.NodeContext{Context: context.Background(),
		UserInputs: map[string]string{deviceUserCodeInput: "BCDF-GHJK"}})
	require.NoError(t, err)
	require.Equal(t, providers.ExecComplete, response.Status)
	require.Equal(t, "stored-app", response.RuntimeData[flowcommon.RuntimeKeyDeviceApplicationID])
	require.Equal(t, "authentication-flow-1",
		response.RuntimeData[flowcommon.RuntimeKeyDeviceApplicationAuthenticationFlowID])
	require.Equal(t, "stored-app", response.EngineData[flowcommon.RuntimeKeyDeviceApplicationID])
	require.Equal(t, "stored-request", response.RuntimeData[flowcommon.RuntimeKeyAuthorizationRequestID])
	require.Equal(t, string(providers.GrantTypeDeviceCode), response.RuntimeData[flowcommon.RuntimeKeyCallbackType])
	require.Equal(t, "booking:read", response.RuntimeData[flowcommon.RuntimeKeyRequestedPermissions])
	require.Equal(t, "https://api.example.com", response.RuntimeData[flowcommon.RuntimeKeyResourceServerIdentifier])
	require.Equal(t, "email", response.RuntimeData[flowcommon.RuntimeKeyRequiredOptionalAttributes])
}

func TestDeviceCodeExecutorRepeatsPromptForUnavailableCode(t *testing.T) {
	executor := &deviceCodeExecutor{
		verifier: deviceCodeVerifierStub{
			verify: func(context.Context, string) (*deviceauth.DeviceVerificationResponse, error) {
				return nil, &deviceauth.DeviceAuthError{Code: oauth2const.ErrorInvalidRequest}
			},
		},
	}
	response, err := executor.Execute(&providers.NodeContext{Context: context.Background(),
		UserInputs: map[string]string{deviceUserCodeInput: "INVALID"}})
	require.NoError(t, err)
	require.Equal(t, providers.ExecUserInputRequired, response.Status)
	require.Equal(t, "invalid_user_code", response.Error.Code)
	require.Empty(t, response.RuntimeData)
}

func TestDeviceCodeExecutorReturnsStoreFailure(t *testing.T) {
	executor := &deviceCodeExecutor{
		verifier: deviceCodeVerifierStub{
			verify: func(context.Context, string) (*deviceauth.DeviceVerificationResponse, error) {
				return nil, errors.New("store unavailable")
			},
		},
	}
	response, err := executor.Execute(&providers.NodeContext{Context: context.Background(),
		UserInputs: map[string]string{deviceUserCodeInput: "BCDF-GHJK"}})
	require.Nil(t, response)
	require.ErrorContains(t, err, "store unavailable")
}
