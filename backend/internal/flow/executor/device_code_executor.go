// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"fmt"
	"strings"

	flowcommon "github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const deviceUserCodeInput = "user_code"

type deviceCodeExecutor struct {
	providers.Executor
	verifier providers.DeviceCodeVerifier
}

var _ providers.Executor = (*deviceCodeExecutor)(nil)

// newDeviceCodeExecutor creates the executor that verifies a submitted Device Grant user code.
func newDeviceCodeExecutor(
	factory core.FlowFactoryInterface, verifier providers.DeviceCodeVerifier,
) *deviceCodeExecutor {
	base := factory.CreateExecutor(ExecutorNameDeviceCode, providers.ExecutorTypeUtility,
		[]providers.Input{{Identifier: deviceUserCodeInput, Type: providers.InputTypeText, Required: true}},
		nil, &providers.ExecutorMeta{SupportedFlowTypes: []providers.FlowType{providers.FlowTypeDeviceGrant}})
	return &deviceCodeExecutor{Executor: base, verifier: verifier}
}

// Execute verifies the submitted user code, returns an input prompt for an invalid code, or stores
// the verified authorization request details in shared runtime data for the rest of the flow.
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

	verified, err := e.verifier.VerifyUserCode(ctx.Context, ctx.Application.ID, code)
	if err != nil {
		return nil, fmt.Errorf("verify device user code: %w", err)
	}
	if verified != nil && verified.InvalidCode {
		return invalidDeviceCodeResponse(input), nil
	}
	if verified == nil || verified.ApplicationID == "" || verified.AuthID == "" {
		return nil, fmt.Errorf("device code verifier returned an incomplete request")
	}
	if verified.ApplicationID != ctx.Application.ID {
		return invalidDeviceCodeResponse(input), nil
	}
	return &providers.ExecutorResponse{Status: providers.ExecComplete,
		SharedRuntimeData: map[string]string{
			flowcommon.RuntimeKeyAuthorizationRequestID:     verified.AuthID,
			flowcommon.RuntimeKeyCallbackType:               string(providers.GrantTypeDeviceCode),
			flowcommon.RuntimeKeyRequestedPermissions:       verified.RequestedPermissions,
			flowcommon.RuntimeKeyResourceServerIdentifier:   verified.ResourceServerIdentifier,
			flowcommon.RuntimeKeyRequiredOptionalAttributes: verified.RequiredOptionalAttributes,
		}}, nil
}

// invalidDeviceCodeResponse asks the user to re-enter a code that cannot be used for this flow.
func invalidDeviceCodeResponse(input providers.Input) *providers.ExecutorResponse {
	return &providers.ExecutorResponse{Status: providers.ExecUserInputRequired,
		Inputs: []providers.Input{input},
		Error:  &ErrInvalidDeviceUserCode,
	}
}
