/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package flowexec

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// FlowExecutionHandler handles flow execution requests.
type flowExecutionHandler struct {
	flowExecService FlowExecServiceInterface
}

func newFlowExecutionHandler(flowExecService FlowExecServiceInterface) *flowExecutionHandler {
	return &flowExecutionHandler{
		flowExecService: flowExecService,
	}
}

// HandleFlowExecutionRequest handles the flow execution request.
func (h *flowExecutionHandler) HandleFlowExecutionRequest(w http.ResponseWriter, r *http.Request) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowExecutionHandler"))

	flowR, err := sysutils.DecodeJSONBody[FlowRequest](r)
	if err != nil {
		sysutils.WriteErrorResponse(w, http.StatusBadRequest, APIErrorFlowRequestJSONDecodeError)
		return
	}

	// Sanitize the input to prevent injection attacks
	appID := sysutils.SanitizeString(flowR.ApplicationID)
	executionID := sysutils.SanitizeString(flowR.ExecutionID)
	flowTypeStr := sysutils.SanitizeString(flowR.FlowType)
	verbose := flowR.Verbose
	action := sysutils.SanitizeString(flowR.Action)
	inputs := sysutils.SanitizeStringMap(flowR.Inputs)
	challengeToken := sysutils.SanitizeString(flowR.ChallengeToken)

	flowStep, flowErr := h.flowExecService.Execute(
		r.Context(), appID, executionID, flowTypeStr, verbose, action, inputs, challengeToken)

	if flowErr != nil {
		handleFlowError(w, flowErr)
		return
	}

	flowResp := FlowResponse{
		ExecutionID:    flowStep.ExecutionID,
		StepID:         flowStep.StepID,
		FlowStatus:     string(flowStep.Status),
		Type:           string(flowStep.Type),
		Data:           flowStep.Data,
		Assertion:      flowStep.Assertion,
		FailureReason:  flowStep.FailureReason,
		ChallengeToken: flowStep.ChallengeToken,
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, flowResp)

	logger.Debug("Flow execution request handled successfully",
		log.String(log.LoggerKeyExecutionID, flowResp.ExecutionID))
}

// handleFlowError handles errors that occur during flow execution as an API error response.
func handleFlowError(w http.ResponseWriter, flowErr *serviceerror.ServiceError) {
	errResp := apierror.ErrorResponse{
		Code:        flowErr.Code,
		Message:     flowErr.Error,
		Description: flowErr.ErrorDescription,
	}

	statusCode := http.StatusInternalServerError
	if flowErr.Type == serviceerror.ClientErrorType {
		statusCode = http.StatusBadRequest
	}

	sysutils.WriteErrorResponse(w, statusCode, errResp)
}
