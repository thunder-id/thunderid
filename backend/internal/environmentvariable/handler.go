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

package environmentvariable

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

const environmentVariableHandlerLoggerComponentName = "EnvironmentVariableHandler"

// environmentVariableHandler serves the environment variable management HTTP endpoints.
type environmentVariableHandler struct {
	environmentVariableService EnvironmentVariableServiceInterface
}

// newEnvironmentVariableHandler creates a new environmentVariableHandler.
func newEnvironmentVariableHandler(service EnvironmentVariableServiceInterface) *environmentVariableHandler {
	return &environmentVariableHandler{environmentVariableService: service}
}

// HandleEnvironmentVariablePostRequest handles environment variable creation.
func (h *environmentVariableHandler) HandleEnvironmentVariablePostRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(
		log.String(log.LoggerKeyComponentName, environmentVariableHandlerLoggerComponentName))

	createRequest, err := sysutils.DecodeJSONBody[CreateEnvironmentVariableRequest](r)
	if err != nil {
		writeParseError(ctx, w, err)
		return
	}

	createRequest.Key = sysutils.SanitizeString(createRequest.Key)
	createRequest.Description = sysutils.SanitizeString(createRequest.Description)

	envID, failed := extractAndValidateEnvID(w, r)
	if failed {
		return
	}

	created, svcErr := h.environmentVariableService.CreateEnvironmentVariable(ctx, envID, *createRequest)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusCreated, created)
	logger.Debug(ctx, "Successfully created environment variable",
		log.String("environmentVariableID", created.ID))
}

// HandleEnvironmentVariableListRequest handles listing environment variables.
func (h *environmentVariableHandler) HandleEnvironmentVariableListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	limit, offset, svcErr := parsePaginationParams(r.URL.Query())
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	if limit == 0 {
		limit = serverconst.DefaultPageSize
	}

	envID, failed := extractAndValidateEnvID(w, r)
	if failed {
		return
	}

	listResponse, svcErr := h.environmentVariableService.GetEnvironmentVariableList(ctx, envID, limit, offset)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, listResponse)
}

// HandleEnvironmentVariableResolveRequest returns every key mapped to its value. It backs the config
// export/apply tooling that substitutes declarative placeholders for a Data Plane.
func (h *environmentVariableHandler) HandleEnvironmentVariableResolveRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	envID, failed := extractAndValidateEnvID(w, r)
	if failed {
		return
	}

	values, svcErr := h.environmentVariableService.ResolveEnvironmentVariables(ctx, envID)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, EnvironmentVariableResolveResponse{Variables: values})
}

// HandleEnvironmentVariableGetRequest handles retrieving a single environment variable.
func (h *environmentVariableHandler) HandleEnvironmentVariableGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	envID, failed := extractAndValidateEnvID(w, r)
	if failed {
		return
	}
	id, failed := extractAndValidateID(w, r)
	if failed {
		return
	}

	result, svcErr := h.environmentVariableService.GetEnvironmentVariable(ctx, envID, id)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, result)
}

// HandleEnvironmentVariablePutRequest handles updating an environment variable.
func (h *environmentVariableHandler) HandleEnvironmentVariablePutRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(
		log.String(log.LoggerKeyComponentName, environmentVariableHandlerLoggerComponentName))

	envID, failed := extractAndValidateEnvID(w, r)
	if failed {
		return
	}
	id, failed := extractAndValidateID(w, r)
	if failed {
		return
	}

	updateRequest, err := sysutils.DecodeJSONBody[UpdateEnvironmentVariableRequest](r)
	if err != nil {
		writeParseError(ctx, w, err)
		return
	}
	updateRequest.Description = sysutils.SanitizeString(updateRequest.Description)

	updated, svcErr := h.environmentVariableService.UpdateEnvironmentVariable(ctx, envID, id, *updateRequest)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, updated)
	logger.Debug(ctx, "Successfully updated environment variable", log.String("environmentVariableID", id))
}

// HandleEnvironmentVariableDeleteRequest handles deleting an environment variable.
func (h *environmentVariableHandler) HandleEnvironmentVariableDeleteRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(
		log.String(log.LoggerKeyComponentName, environmentVariableHandlerLoggerComponentName))

	envID, failed := extractAndValidateEnvID(w, r)
	if failed {
		return
	}
	id, failed := extractAndValidateID(w, r)
	if failed {
		return
	}

	svcErr := h.environmentVariableService.DeleteEnvironmentVariable(ctx, envID, id)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusNoContent, nil)
	logger.Debug(ctx, "Successfully deleted environment variable", log.String("environmentVariableID", id))
}

// parsePaginationParams parses limit and offset from query parameters.
func parsePaginationParams(query map[string][]string) (int, int, *tidcommon.ServiceError) {
	var limit, offset int
	var err error

	if limitStr := query["limit"]; len(limitStr) > 0 && limitStr[0] != "" {
		limit, err = strconv.Atoi(sysutils.SanitizeString(limitStr[0]))
		if err != nil || limit <= 0 {
			return 0, 0, &ErrorInvalidEnvironmentVariableRequest
		}
	}

	if offsetStr := query["offset"]; len(offsetStr) > 0 && offsetStr[0] != "" {
		offset, err = strconv.Atoi(sysutils.SanitizeString(offsetStr[0]))
		if err != nil || offset < 0 {
			return 0, 0, &ErrorInvalidEnvironmentVariableRequest
		}
	}

	return limit, offset, nil
}

// extractAndValidateID extracts and validates the environment variable id from the URL path.
func extractAndValidateID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if id == "" {
		errResp := apierror.ErrorResponse{
			Code:        ErrorInvalidEnvironmentVariableRequest.Code,
			Message:     ErrorInvalidEnvironmentVariableRequest.Error,
			Description: ErrorInvalidEnvironmentVariableRequest.ErrorDescription,
		}
		sysutils.WriteErrorResponse(r.Context(), w, http.StatusBadRequest, errResp)
		return "", true
	}
	return id, false
}

// extractAndValidateEnvID reads the environment a variable belongs to from the path.
func extractAndValidateEnvID(w http.ResponseWriter, r *http.Request) (string, bool) {
	envID := r.PathValue("envId")
	if envID == "" {
		errResp := apierror.ErrorResponse{
			Code:        ErrorInvalidEnvironmentVariableRequest.Code,
			Message:     ErrorInvalidEnvironmentVariableRequest.Error,
			Description: ErrorInvalidEnvironmentVariableRequest.ErrorDescription,
		}
		sysutils.WriteErrorResponse(r.Context(), w, http.StatusBadRequest, errResp)
		return "", true
	}
	return envID, false
}

// writeParseError writes a validation or parse-failure response for a malformed request body.
func writeParseError(ctx context.Context, w http.ResponseWriter, err error) {
	var valErr *sysutils.ValidationError
	if errors.As(err, &valErr) {
		sysutils.WriteStructuredErrorResponse(w, http.StatusBadRequest, "Validation Failed", valErr.Errors)
		return
	}
	errResp := apierror.ErrorResponse{
		Code:        ErrorInvalidEnvironmentVariableRequest.Code,
		Message:     ErrorInvalidEnvironmentVariableRequest.Error,
		Description: ErrorInvalidEnvironmentVariableRequest.ErrorDescription,
	}
	sysutils.WriteErrorResponse(ctx, w, http.StatusBadRequest, errResp)
}

// handleError converts a ServiceError to the appropriate HTTP response.
func handleError(ctx context.Context, w http.ResponseWriter, svcErr *tidcommon.ServiceError) {
	statusCode := http.StatusInternalServerError
	if svcErr.Type == tidcommon.ClientErrorType {
		statusCode = http.StatusBadRequest
		switch svcErr.Code {
		case ErrorEnvironmentVariableNotFound.Code:
			statusCode = http.StatusNotFound
		case ErrorEnvironmentVariableKeyConflict.Code:
			statusCode = http.StatusConflict
		}
	}

	errResp := apierror.ErrorResponse{
		Code:        svcErr.Code,
		Message:     svcErr.Error,
		Description: svcErr.ErrorDescription,
	}
	sysutils.WriteErrorResponse(ctx, w, statusCode, errResp)
}
