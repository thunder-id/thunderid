// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
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
				{Property: propertyKeyApprovedActions},
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

	approved, err := approvedActions(ctx)
	if err != nil {
		return nil, err
	}
	if approved != "" {
		execResp.EngineData = map[string]string{common.EngineDataKeyApprovedActions: approved}
	}

	inviteToken, err := e.getOrGenerateToken(ctx)
	if err != nil {
		logger.Debug(ctx.Context, "Failed to get or generate invite token", log.Error(err))
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrInviteTokenGenerationFailed
		return execResp, nil
	}

	inviteLink := e.generateInviteLink(ctx, inviteToken)

	execResp.RuntimeData[common.RuntimeKeyStoredInviteToken] = inviteToken
	execResp.RuntimeData[common.RuntimeKeyInviteLink] = inviteLink

	execResp.ForwardedData[common.ForwardedDataKeyTemplateData] = map[string]interface{}{
		"inviteLink": inviteLink,
		"appName":    ctx.Application.Name,
	}

	// The onboarding admin sees the link, for a screen that shows it when no email can be sent. An invitation's admin
	// does not: with it they could accept for an existing user, so the email step surfaces it only when sending fails.
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

	if !hasStoredToken || storedToken == "" {
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
		proof.Passed = true
		shared, err := sharedVerificationEvidence(*proof)
		if err != nil {
			return nil, err
		}

		execResp.SharedRuntimeData = shared
	}

	logger.Debug(ctx.Context, "Invite token validated successfully")
	execResp.Status = providers.ExecComplete
	return execResp, nil
}

// getOrGenerateToken retrieves the existing invite token from runtime data or generates a new one.
func (e *inviteExecutor) getOrGenerateToken(ctx *providers.NodeContext) (string, error) {
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

// approvedActions resolves, as JSON, the writes the node approves for the steps its flow's caller does not send,
// or "" when it approves none. Each entry names an action and the organization unit it acts in, and may name the
// only resources it acts on, space-separated. Units and resources resolve from the runtime data alone. A unit may
// list candidates, of which the first that resolves is taken. An entry whose resources resolve to none approves
// nothing.
func approvedActions(ctx *providers.NodeContext) (string, error) {
	raw, ok := ctx.NodeProperties[propertyKeyApprovedActions]
	if !ok || raw == nil {
		return "", nil
	}
	entries, ok := raw.([]interface{})
	if !ok {
		return "", fmt.Errorf("property %q must be a list", propertyKeyApprovedActions)
	}

	approved := make([]security.ApprovedAction, 0, len(entries))
	for _, entry := range entries {
		fields, ok := entry.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("property %q must list objects", propertyKeyApprovedActions)
		}
		action, _ := fields["action"].(string)
		if action == "" {
			return "", fmt.Errorf("property %q has an entry without an action", propertyKeyApprovedActions)
		}
		ouID := firstResolved(ctx, fields["ouId"])
		if ouID == "" {
			return "", fmt.Errorf("property %q: the unit of %q did not resolve from the runtime data",
				propertyKeyApprovedActions, action)
		}
		approval := security.ApprovedAction{Action: security.Action(action), OUID: ouID}
		if resources, named := fields["resourceIds"]; named {
			if approval.ResourceIDs = uniqueFields(firstResolved(ctx, resources)); len(approval.ResourceIDs) == 0 {
				continue
			}
		}
		approved = append(approved, approval)
	}
	if len(approved) == 0 {
		return "", nil
	}

	encoded, err := json.Marshal(approved)
	if err != nil {
		return "", fmt.Errorf("failed to encode the approved actions: %w", err)
	}
	return string(encoded), nil
}

// firstResolved returns the first of a value or list of values that resolves from the runtime data alone to
// something other than blank, or "".
func firstResolved(ctx *providers.NodeContext, value interface{}) string {
	candidates := []interface{}{value}
	if list, ok := value.([]interface{}); ok {
		candidates = list
	}
	for _, candidate := range candidates {
		text, _ := candidate.(string)
		resolved := core.ResolvePlaceholder(&providers.NodeContext{RuntimeData: ctx.RuntimeData},
			text, nil, nil, false, log.GetLogger())
		if !strings.Contains(resolved, "{{") && strings.TrimSpace(resolved) != "" {
			return resolved
		}
	}
	return ""
}
