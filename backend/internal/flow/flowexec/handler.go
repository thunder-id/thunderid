// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowexec

import (
	"context"
	"net/http"
	"time"

	"github.com/thunder-id/thunderid/internal/flow/session"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// FlowExecutionHandler handles flow execution requests.
type flowExecutionHandler struct {
	flowExecService FlowExecServiceInterface
	ssoTransport    session.HandleTransport
	// ssoHandleTTL bounds the per-flow SSO handle cookie to the session's configured absolute lifetime.
	ssoHandleTTL time.Duration
}

func newFlowExecutionHandler(flowExecService FlowExecServiceInterface, ssoTransport session.HandleTransport,
	ssoHandleTTL time.Duration) *flowExecutionHandler {
	return &flowExecutionHandler{
		flowExecService: flowExecService,
		ssoTransport:    ssoTransport,
		ssoHandleTTL:    ssoHandleTTL,
	}
}

// HandleFlowExecutionRequest handles the flow execution request.
func (h *flowExecutionHandler) HandleFlowExecutionRequest(w http.ResponseWriter, r *http.Request) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowExecutionHandler"))

	flowR, err := sysutils.DecodeJSONBody[FlowRequest](r)
	if err != nil {
		sysutils.WriteErrorResponse(r.Context(), w, http.StatusBadRequest, APIErrorFlowRequestJSONDecodeError)
		return
	}

	// Sanitize the input to prevent injection attacks
	appID := sysutils.SanitizeString(flowR.ApplicationID)
	flowID := sysutils.SanitizeString(flowR.FlowID)
	executionID := sysutils.SanitizeString(flowR.ExecutionID)
	flowTypeStr := sysutils.SanitizeString(flowR.FlowType)
	verbose := flowR.Verbose
	action := sysutils.SanitizeString(flowR.Action)
	inputs := sysutils.SanitizeStringMap(flowR.Inputs)
	challengeToken := sysutils.SanitizeString(flowR.ChallengeToken)
	flowSecret := sysutils.SanitizeString(r.Header.Get(serverconst.FlowSecretHeaderName))
	attestationToken := sysutils.SanitizeString(r.Header.Get(serverconst.AttestationTokenHeaderName))

	// Read the inbound SSO transport inputs (per-flow handle cookies) and make
	// them available to the flow service, which selects the handle once the flow is known.
	ctx := session.WithInbound(r.Context(), h.ssoTransport.Read(r))

	// Carry the current step's request headers and query params so flow executors can publish them
	// via {{request(flow.*)}} placeholders. Credential-bearing headers are stripped at this boundary.
	ctx = withCurrentRequest(ctx, &providers.InitiatorRequest{
		Headers:     sysutils.FilterSensitiveHeaders(r.Header),
		QueryParams: r.URL.Query(),
	})

	var flowStep *FlowStep
	var flowErr *tidcommon.ServiceError
	if flowID != "" {
		flowStep, flowErr = h.flowExecService.ExecuteByID(
			ctx, flowID, executionID, verbose, action, inputs, challengeToken)
	} else {
		flowStep, flowErr = h.flowExecService.Execute(
			ctx, appID, executionID, flowTypeStr, verbose, action, inputs, challengeToken,
			flowSecret, attestationToken)
	}

	if flowErr != nil {
		errorAssertion := ""
		if flowStep != nil {
			errorAssertion = flowStep.ErrorAssertion
		}
		handleFlowError(r.Context(), w, flowErr, errorAssertion)
		return
	}

	// Convert service error to API error if present in the flow step response
	var stepErrorResp *apierror.ErrorResponse
	if flowStep.Error != nil {
		resp := convertToAPIError(flowStep.Error)
		stepErrorResp = &resp
	}

	// Emit the per-flow SSO handle cookie when the flow minted a new session handle. This must
	// happen before the response body is written.
	if flowStep.SSOHandleOut != "" && flowStep.SSOFlowID != "" {
		// The handle has no TTL of its own; bound the cookie to the session's configured absolute
		// lifetime.
		h.ssoTransport.Write(w, session.CookieName(flowStep.SSOFlowID), flowStep.SSOHandleOut,
			h.ssoHandleTTL)
	}

	// Clear the per-flow SSO cookie when the flow terminated the session (sign-out).
	if flowStep.SSOClearFlowID != "" {
		h.ssoTransport.Clear(w, session.CookieName(flowStep.SSOClearFlowID))
	}

	flowResp := FlowResponse{
		ExecutionID:    flowStep.ExecutionID,
		StepID:         flowStep.StepID,
		FlowStatus:     string(flowStep.Status),
		Type:           string(flowStep.Type),
		Data:           flowStep.Data,
		Assertion:      flowStep.Assertion,
		ErrorAssertion: flowStep.ErrorAssertion,
		Error:          stepErrorResp,
		ChallengeToken: flowStep.ChallengeToken,
	}

	sysutils.WriteSuccessResponse(r.Context(), w, http.StatusOK, flowResp)

	logger.Debug(r.Context(), "Flow execution request handled successfully",
		log.String(log.LoggerKeyExecutionID, flowResp.ExecutionID))
}

// flowErrorResponse is the error body for a failed flow execution. It carries the standard API error
// plus a signed error assertion (when the flow was OAuth-initiated) that the Gate/SDK relays to the
// OAuth callback so the failure reaches the waiting authorization request.
type flowErrorResponse struct {
	apierror.ErrorResponse
	ErrorAssertion string `json:"errorAssertion,omitempty"`
}

// handleFlowError handles errors that occur during flow execution as an API error response.
func handleFlowError(ctx context.Context, w http.ResponseWriter, flowErr *tidcommon.ServiceError,
	errorAssertion string) {
	errResp := apierror.ErrorResponse{
		Code:        flowErr.Code,
		Message:     flowErr.Error,
		Description: flowErr.ErrorDescription,
	}

	statusCode := http.StatusInternalServerError
	if flowErr.Type == tidcommon.ClientErrorType {
		switch flowErr.Code {
		case ErrorDirectFlowInitiationNotPermitted.Code, ErrorFlowIDExecutionNotPermitted.Code,
			ErrorAdministrationPermissionRequired.Code:
			statusCode = http.StatusForbidden
		case ErrorFlowSecretRequired.Code, ErrorFlowSecretInvalid.Code,
			ErrorAttestationRequired.Code, ErrorAttestationInvalid.Code,
			ErrorAdministrationAuthenticationRequired.Code:
			statusCode = http.StatusUnauthorized
		default:
			statusCode = http.StatusBadRequest
		}
	}

	if errorAssertion != "" {
		sysutils.WriteSuccessResponse(ctx, w, statusCode,
			flowErrorResponse{ErrorResponse: errResp, ErrorAssertion: errorAssertion})
		return
	}

	sysutils.WriteErrorResponse(ctx, w, statusCode, errResp)
}

// convertToAPIError converts service errors that occur during flow step execution as an API error response.
func convertToAPIError(flowErr *tidcommon.ServiceError) apierror.ErrorResponse {
	errResp := apierror.ErrorResponse{
		Code:        flowErr.Code,
		Message:     flowErr.Error,
		Description: flowErr.ErrorDescription,
	}

	return errResp
}
