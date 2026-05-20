/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package executor

import (
	"fmt"
	"net/url"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// inviteExecutor generates an invite link for the user to complete registration.
type inviteExecutor struct {
	core.ExecutorInterface
	logger *log.Logger
}

// newInviteExecutor creates a new instance of the invite executor.
func newInviteExecutor(flowFactory core.FlowFactoryInterface) *inviteExecutor {
	defaultInputs := []common.Input{
		{
			Identifier: userInputInviteToken,
			Type:       "HIDDEN",
			Required:   true,
		},
	}
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "InviteExecutor"))
	base := flowFactory.CreateExecutor(
		ExecutorNameInviteExecutor,
		common.ExecutorTypeUtility,
		defaultInputs,
		[]common.Input{},
	)
	return &inviteExecutor{
		ExecutorInterface: base,
		logger:            logger,
	}
}

// GetExecutionPolicy returns the execution policy for the given mode.
// The verify mode skips challenge token validation because the invite token itself serves as the challenge.
func (e *inviteExecutor) GetExecutionPolicy(mode string) *core.ExecutionPolicy {
	if mode == ExecutorModeVerify {
		return &core.ExecutionPolicy{
			SkipChallengeValidation: true,
			AllowSegmentRestart:     true,
		}
	}
	return nil
}

// Execute delegates to the appropriate mode handler based on the executor mode.
func (e *inviteExecutor) Execute(ctx *core.NodeContext) (*common.ExecutorResponse, error) {
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
func (e *inviteExecutor) executeGenerate(ctx *core.NodeContext) (*common.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug("Executing invite executor in generate mode")

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
		ForwardedData:  make(map[string]interface{}),
	}

	inviteToken, err := e.getOrGenerateToken(ctx)
	if err != nil {
		logger.Debug("Failed to get or generate invite token", log.Error(err))
		execResp.Status = common.ExecFailure
		return execResp, nil
	}

	inviteLink := e.generateInviteLink(ctx, inviteToken)

	execResp.RuntimeData[common.RuntimeKeyStoredInviteToken] = inviteToken
	execResp.RuntimeData[common.RuntimeKeyInviteLink] = inviteLink

	execResp.ForwardedData[common.ForwardedDataKeyTemplateData] = map[string]interface{}{
		"inviteLink": inviteLink,
		"appName":    ctx.Application.Name,
	}

	if ctx.FlowType == common.FlowTypeUserOnboarding {
		execResp.AdditionalData[common.DataInviteLink] = inviteLink
	}

	execResp.Status = common.ExecComplete
	return execResp, nil
}

// executeVerify validates the user-provided invite token against the stored token.
func (e *inviteExecutor) executeVerify(ctx *core.NodeContext) (*common.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug("Executing invite executor in verify mode")

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	// If the user has not yet provided the invite token, request it
	if !e.HasRequiredInputs(ctx, execResp) {
		execResp.Status = common.ExecUserInputRequired
		return execResp, nil
	}

	// User has provided the invite token, validate it against stored token
	inviteTokenInput := ctx.UserInputs[userInputInviteToken]
	storedToken, hasStoredToken := ctx.RuntimeData[common.RuntimeKeyStoredInviteToken]

	if !hasStoredToken {
		logger.Debug("No invite token found in runtime data")
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "Invalid invite token"
		return execResp, nil
	}

	if inviteTokenInput != storedToken {
		logger.Debug("Invite token mismatch", log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "Invalid invite token"
		return execResp, nil
	}

	logger.Debug("Invite token validated successfully")
	execResp.Status = common.ExecComplete
	return execResp, nil
}

// getOrGenerateToken retrieves the existing invite token from runtime data or generates a new one.
func (e *inviteExecutor) getOrGenerateToken(ctx *core.NodeContext) (string, error) {
	if storedToken, exists := ctx.RuntimeData[common.RuntimeKeyStoredInviteToken]; exists && storedToken != "" {
		return storedToken, nil
	}

	return utils.GenerateUUIDv7()
}

// generateInviteLink constructs the invite link using the GateClient configuration.
func (e *inviteExecutor) generateInviteLink(ctx *core.NodeContext, inviteToken string) string {
	gateConfig := config.GetServerRuntime().Config.GateClient
	gateAppURL := fmt.Sprintf("%s://%s:%d%s",
		gateConfig.Scheme,
		gateConfig.Hostname,
		gateConfig.Port,
		gateConfig.Path)
	queryParams := url.Values{
		"executionId": []string{ctx.ExecutionID},
		"inviteToken": []string{inviteToken},
	}

	if ctx.EntityID != "" {
		queryParams.Set(oauth2const.AppID, ctx.EntityID)
	}

	return fmt.Sprintf("%s/invite?%s", gateAppURL, queryParams.Encode())
}
