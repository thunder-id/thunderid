// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowexec

import (
	"context"
	"math"
	"net/http"
	"time"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/session"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	sysContext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// FlowExecutionHandler handles flow execution requests.
type flowExecutionHandler struct {
	flowExecService FlowExecServiceInterface
	// ssoTransport carries the SSO handle to and from the client. It is nil when the deployment has
	// no SSO session store.
	ssoTransport session.HandleTransportInterface
	// ssoSessions bounds the per-flow SSO handle to the session's absolute lifetime. It is nil where the
	// deployment keeps no SSO sessions.
	ssoSessions ssoSessionLifetime
}

func newFlowExecutionHandler(flowExecService FlowExecServiceInterface, ssoTransport session.HandleTransportInterface,
	ssoSessions ssoSessionLifetime) *flowExecutionHandler {
	return &flowExecutionHandler{
		flowExecService: flowExecService,
		ssoTransport:    ssoTransport,
		ssoSessions:     ssoSessions,
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
	handle := sysutils.SanitizeString(flowR.Handle)
	verbose := flowR.Verbose
	action := sysutils.SanitizeString(flowR.Action)
	inputs := sysutils.SanitizeStringMap(flowR.Inputs)
	challengeToken := sysutils.SanitizeString(flowR.ChallengeToken)
	ouID := sysutils.SanitizeString(flowR.OUID)
	flowSecret := sysutils.SanitizeString(r.Header.Get(serverconst.FlowSecretHeaderName))
	attestationToken := sysutils.SanitizeString(r.Header.Get(serverconst.AttestationTokenHeaderName))
	sessionData := make([]SessionData, 0, len(flowR.SessionData))
	for _, entry := range flowR.SessionData {
		flowIdentifier := sysutils.SanitizeString(entry.FlowIdentifier)
		sessionHandle := sysutils.SanitizeString(entry.SessionHandle)
		if flowIdentifier == "" || sessionHandle == "" {
			sysutils.WriteErrorResponse(r.Context(), w, http.StatusBadRequest, APIErrorInvalidSessionData)
			return
		}
		sessionData = append(sessionData, SessionData{FlowIdentifier: flowIdentifier, SessionHandle: sessionHandle})
	}

	// An organization unit is named only to start a sign-in; a flow selected by ID is an administration flow.
	ctx := r.Context()
	if ouID != "" {
		if executionID != "" || flowID != "" || flowTypeStr != string(providers.FlowTypeAuthentication) {
			sysutils.WriteErrorResponse(ctx, w, http.StatusBadRequest, APIErrorOUIDNotAllowed)
			return
		}
		ctx = sysContext.WithAccessingOUID(ctx, ouID)
	}

	// Read the inbound SSO transport inputs and make them available to the flow service, which
	// selects the handle once the flow is known.
	ssoExchange := &session.Exchange{Request: r, Response: w}
	if h.ssoTransport != nil {
		ctx = session.WithInbound(ctx, h.ssoTransport.Read(ssoExchange))
	}

	// Carry the current step's request headers and query params so flow executors can publish them
	// via {{request(current.*)}} placeholders. Credential-bearing headers and query params are
	// stripped at this boundary; the Host is folded in so {{request(current.header.Host)}} resolves.
	ctx = common.WithCurrentRequest(ctx, &providers.CurrentRequest{
		Headers:     sysutils.FilterSensitiveHeaders(sysutils.HeadersWithHost(r)),
		QueryParams: sysutils.FilterSensitiveQueryParams(r.URL.Query()),
	})

	var flowStep *FlowStep
	var flowErr *tidcommon.ServiceError
	if flowID != "" {
		flowStep, flowErr = h.flowExecService.ExecuteByID(
			ctx, flowID, executionID, verbose, action, inputs, challengeToken, sessionData)
	} else {
		flowStep, flowErr = h.flowExecService.Execute(
			ctx, appID, executionID, flowTypeStr, handle, verbose, action, inputs, challengeToken,
			flowSecret, attestationToken, sessionData)
	}

	if flowErr != nil {
		handleFlowError(r.Context(), w, flowErr, flowStep)
		return
	}

	// Convert service error to API error if present in the flow step response
	var stepErrorResp *apierror.ErrorResponse
	if flowStep.Error != nil {
		resp := convertToAPIError(flowStep.Error)
		stepErrorResp = &resp
	}

	if h.ssoTransport != nil {
		// Emit the per-flow SSO handle when the flow minted a new session handle. This must happen
		// before the response body is written.
		if flowStep.SSOHandleOut != "" && flowStep.SSOFlowID != "" {
			// The handle has no TTL of its own; bound it to the session's absolute lifetime.
			h.ssoTransport.Write(ssoExchange, flowStep.SSOFlowID, flowStep.SSOHandleOut,
				ssoHandleTTL(ctx, h.ssoSessions))
		}

		// Clear the per-flow SSO handle when the flow terminated the session (sign-out).
		if flowStep.SSOClearFlowID != "" {
			h.ssoTransport.Clear(ssoExchange, flowStep.SSOClearFlowID)
		}
	}

	flowResp := FlowResponse{
		ExecutionID:     flowStep.ExecutionID,
		StepID:          flowStep.StepID,
		FlowStatus:      string(flowStep.Status),
		Type:            string(flowStep.Type),
		Data:            flowStep.Data,
		Assertion:       flowStep.Assertion,
		ErrorAssertion:  flowStep.ErrorAssertion,
		Error:           stepErrorResp,
		ChallengeToken:  flowStep.ChallengeToken,
		SessionData:     flowStep.SessionData,
		ClearedSessions: flowStep.ClearedSessions,
	}
	if !flowStep.ExpiresAt.IsZero() {
		if expiresIn := int64(math.Ceil(time.Until(flowStep.ExpiresAt).Seconds())); expiresIn > 0 {
			flowResp.ExpiresIn = expiresIn
		}
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
	ErrorAssertion string    `json:"errorAssertion,omitempty"`
	Data           *FlowData `json:"data,omitempty"`
}

// handleFlowError handles errors that occur during flow execution as an API error response.
func handleFlowError(ctx context.Context, w http.ResponseWriter, flowErr *tidcommon.ServiceError,
	flowStep *FlowStep) {
	errResp := apierror.ErrorResponse{
		Code:        flowErr.Code,
		Message:     flowErr.Error,
		Description: flowErr.ErrorDescription,
	}

	statusCode := http.StatusInternalServerError
	if flowErr.Type == tidcommon.ClientErrorType {
		switch flowErr.Code {
		case ErrorDirectFlowInitiationNotPermitted.Code, ErrorFlowIDExecutionNotPermitted.Code,
			ErrorAdministrationPermissionRequired.Code, ErrorFlowApplicationNotUsable.Code,
			ErrorInvitationOUMissing.Code, ErrorFlowNotSharedWithOU.Code, ErrorFlowStartedByAnotherCaller.Code:
			statusCode = http.StatusForbidden
		case ErrorFlowSecretRequired.Code, ErrorFlowSecretInvalid.Code,
			ErrorAttestationRequired.Code, ErrorAttestationInvalid.Code,
			ErrorFlowAuthenticationRequired.Code:
			statusCode = http.StatusUnauthorized
		default:
			statusCode = http.StatusBadRequest
		}
	}

	if flowStep != nil && flowStep.ErrorAssertion != "" {
		response := flowErrorResponse{
			ErrorResponse:  errResp,
			ErrorAssertion: flowStep.ErrorAssertion,
		}
		if len(flowStep.Data.AdditionalData) > 0 {
			response.Data = &flowStep.Data
		}
		sysutils.WriteSuccessResponse(ctx, w, statusCode,
			response)
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
