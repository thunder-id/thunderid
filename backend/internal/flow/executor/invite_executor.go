// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"fmt"
	"net/url"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// inviteExecutor generates an invite link for the user to complete registration.
type inviteExecutor struct {
	providers.Executor
	logger *log.Logger
}

// newInviteExecutor creates a new instance of the invite executor.
func newInviteExecutor(flowFactory core.FlowFactoryInterface) *inviteExecutor {
	defaultInputs := []providers.Input{
		{
			Identifier: userInputInviteToken,
			Type:       "HIDDEN",
			Required:   true,
		},
	}
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "InviteExecutor"))
	base := flowFactory.CreateExecutor(
		ExecutorNameInviteExecutor,
		providers.ExecutorTypeUtility,
		defaultInputs,
		[]providers.Input{},
		&providers.ExecutorMeta{
			SupportedModes: []string{ExecutorModeGenerate, ExecutorModeVerify},
			SupportedProperties: []providers.ExecutorSupportedProperties{
				{Property: propertyKeyInviteBaseURL},
			},
		},
	)
	return &inviteExecutor{
		Executor: base,
		logger:   logger,
	}
}

// GetExecutionPolicy returns the execution policy for the given mode.
// The verify mode skips challenge token validation because the invite token itself serves as the challenge.
func (e *inviteExecutor) GetExecutionPolicy(mode string) *providers.ExecutionPolicy {
	if mode == ExecutorModeVerify {
		return &providers.ExecutionPolicy{
			SkipChallengeValidation: true,
			AllowSegmentRestart:     true,
		}
	}
	return nil
}

// Execute delegates to the appropriate mode handler based on the executor mode.
func (e *inviteExecutor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	switch ctx.ExecutorMode {
	case ExecutorModeGenerate:
		return e.executeGenerate(ctx)
	case ExecutorModeVerify:
		return e.executeVerify(ctx)
	default:
		return nil, fmt.Errorf("invalid executor mode for InviteExecutor: %s", ctx.ExecutorMode)
	}
}

// executeGenerate generates the invite token and link.
func (e *inviteExecutor) executeGenerate(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Executing invite executor in generate mode")

	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
		ForwardedData:  make(map[string]interface{}),
	}

	proof, err := pinnedVerificationEvidence(ctx)
	if err != nil {
		return nil, err
	}

	inviteToken, err := e.getOrGenerateToken(ctx, proof)
	if err != nil {
		logger.Debug(ctx.Context, "Failed to get or generate invite token", log.Error(err))
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrInviteTokenGenerationFailed
		return execResp, nil
	}

	if proof != nil {
		proof.Challenge, proof.Passed, proof.Receipts = inviteToken, false, nil
		execResp.SharedRuntimeData = sharedVerificationEvidence(*proof)
	}
	inviteLink := e.generateInviteLink(ctx, inviteToken)

	execResp.RuntimeData[common.RuntimeKeyStoredInviteToken] = inviteToken
	execResp.RuntimeData[common.RuntimeKeyInviteLink] = inviteLink

	execResp.ForwardedData[common.ForwardedDataKeyTemplateData] = map[string]interface{}{
		"inviteLink": inviteLink,
		"appName":    ctx.Application.Name,
	}

	if ctx.FlowType == providers.FlowTypeUserOnboarding {
		execResp.AdditionalData[common.DataInviteLink] = inviteLink
	}

	execResp.Status = providers.ExecComplete
	return execResp, nil
}

// executeVerify validates the user-provided invite token against the stored token.
func (e *inviteExecutor) executeVerify(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Executing invite executor in verify mode")

	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	proof, err := pinnedVerificationEvidence(ctx)
	if err != nil {
		return nil, err
	}

	// If the user has not yet provided the invite token, request it
	if !e.HasRequiredInputs(ctx, execResp) {
		execResp.Status = providers.ExecUserInputRequired
		return execResp, nil
	}

	// User has provided the invite token, validate it against stored token
	inviteTokenInput := ctx.UserInputs[userInputInviteToken]
	storedToken, hasStoredToken := ctx.RuntimeData[common.RuntimeKeyStoredInviteToken]

	if !hasStoredToken {
		logger.Debug(ctx.Context, "No invite token found in runtime data")
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrInvalidInviteToken
		return execResp, nil
	}

	if inviteTokenInput != storedToken {
		logger.Debug(ctx.Context, "Invite token mismatch",
			log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrInvalidInviteToken
		return execResp, nil
	}

	if proof != nil {
		if proof.Challenge == "" || proof.Challenge != storedToken {
			execResp.Status, execResp.Error = providers.ExecFailure, &ErrInvalidInviteToken
			return execResp, nil
		}
		proof.Passed = true
		execResp.SharedRuntimeData = sharedVerificationEvidence(*proof)
	}
	logger.Debug(ctx.Context, "Invite token validated successfully")
	execResp.Status = providers.ExecComplete
	return execResp, nil
}

// getOrGenerateToken retrieves the existing invite token from runtime data or generates a new one. A
// pinned contact always gets a new token.
func (e *inviteExecutor) getOrGenerateToken(ctx *providers.NodeContext,
	proof *verificationEvidence) (string, error) {
	if proof != nil {
		return utils.GenerateUUIDv7()
	}
	if storedToken, exists := ctx.RuntimeData[common.RuntimeKeyStoredInviteToken]; exists && storedToken != "" {
		return storedToken, nil
	}

	return utils.GenerateUUIDv7()
}

// generateInviteLink constructs the invite link. If the node declares an inviteBaseURL
// property it is used as the base; otherwise the GateClient configuration is the fallback.
func (e *inviteExecutor) generateInviteLink(ctx *providers.NodeContext, inviteToken string) string {
	queryParams := url.Values{
		"executionId": []string{ctx.ExecutionID},
		"inviteToken": []string{inviteToken},
	}

	if ctx.EntityID != "" {
		queryParams.Set(oauth2const.AppID, ctx.EntityID)
	}
	if authReqID, ok := ctx.RuntimeData[common.RuntimeKeyAuthorizationRequestID]; ok && authReqID != "" {
		queryParams.Set(oauth2const.RequestParamAuthReqID, authReqID)
	}

	if baseURL, ok := ctx.NodeProperties[propertyKeyInviteBaseURL].(string); ok && baseURL != "" {
		if u, err := url.Parse(baseURL); err == nil {
			q := u.Query()
			for k, vals := range queryParams {
				q[k] = vals
			}
			u.RawQuery = q.Encode()
			return u.String()
		}
	}

	gateConfig := config.GetServerRuntime().Config.GateClient
	gateAppURL := fmt.Sprintf("%s://%s:%d%s",
		gateConfig.Scheme,
		gateConfig.Hostname,
		gateConfig.Port,
		gateConfig.Path)

	return fmt.Sprintf("%s/invite?%s", gateAppURL, queryParams.Encode())
}
