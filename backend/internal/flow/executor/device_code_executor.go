// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"errors"
	"fmt"
	"strings"

	flowcommon "github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/deviceauth"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const deviceUserCodeInput = "user_code"

// DeviceCodeVerifier verifies a Device Grant user code and returns its trusted request context.
type DeviceCodeVerifier interface {
	VerifyUserCode(context.Context, string) (*deviceauth.DeviceVerificationResponse, error)
}

type deviceCodeExecutor struct {
	providers.Executor
	verifier DeviceCodeVerifier
}

var _ providers.Executor = (*deviceCodeExecutor)(nil)

func newDeviceCodeExecutor(factory core.FlowFactoryInterface, verifier DeviceCodeVerifier) *deviceCodeExecutor {
	base := factory.CreateExecutor(ExecutorNameDeviceCode, providers.ExecutorTypeUtility,
		[]providers.Input{{Identifier: deviceUserCodeInput, Type: providers.InputTypeText, Required: true}},
		nil, &providers.ExecutorMeta{SupportedFlowTypes: []providers.FlowType{providers.FlowTypeSystem}})
	return &deviceCodeExecutor{Executor: base, verifier: verifier}
}

// Execute asks for a user code or binds the system flow to the request identified by a valid code.
func (e *deviceCodeExecutor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	input := providers.Input{Identifier: deviceUserCodeInput, Type: providers.InputTypeText, Required: true}
	code, ok := ctx.ConsumeInput(deviceUserCodeInput)
	if !ok || strings.TrimSpace(code) == "" {
		return &providers.ExecutorResponse{Status: providers.ExecUserInputRequired,
			Inputs: []providers.Input{input}}, nil
	}
	if e.verifier == nil {
		return nil, fmt.Errorf("device code verifier is not configured")
	}

	verified, err := e.verifier.VerifyUserCode(ctx.Context, code)
	if err != nil {
		var deviceErr *deviceauth.DeviceAuthError
		if !errors.As(err, &deviceErr) || deviceErr.Code != oauth2const.ErrorInvalidRequest {
			return nil, fmt.Errorf("verify device user code: %w", err)
		}
		return &providers.ExecutorResponse{Status: providers.ExecUserInputRequired,
			Inputs: []providers.Input{input},
			Error: &tidcommon.ServiceError{
				Code: "invalid_user_code", Type: tidcommon.ClientErrorType,
				Error: tidcommon.I18nMessage{Key: "device_authorization.invalid_user_code",
					DefaultValue: "This code is invalid or no longer available. Try again."},
			}}, nil
	}
	if verified == nil || verified.ApplicationID == "" || verified.ApplicationAuthFlowID == "" ||
		verified.AuthID == "" {
		return nil, fmt.Errorf("device code verifier returned an incomplete request")
	}
	return &providers.ExecutorResponse{Status: providers.ExecComplete,
		EngineData: map[string]string{
			flowcommon.RuntimeKeyDeviceApplicationID: verified.ApplicationID,
		},
		RuntimeData: map[string]string{
			flowcommon.RuntimeKeyDeviceApplicationID:                   verified.ApplicationID,
			flowcommon.RuntimeKeyDeviceApplicationAuthenticationFlowID: verified.ApplicationAuthFlowID,
			flowcommon.RuntimeKeyAuthorizationRequestID:                verified.AuthID,
			flowcommon.RuntimeKeyCallbackType:                          string(providers.GrantTypeDeviceCode),
			flowcommon.RuntimeKeyRequestedPermissions:                  verified.RequestedPermissions,
			flowcommon.RuntimeKeyResourceServerIdentifier:              verified.ResourceServerIdentifier,
			flowcommon.RuntimeKeyRequiredOptionalAttributes:            verified.RequiredOptionalAttributes,
		}}, nil
}
