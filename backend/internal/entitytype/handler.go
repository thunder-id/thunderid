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

package entitytype

import (
	"net/http"
	"strconv"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

const entityTypeHandlerLoggerComponentName = "EntityTypeHandler"

// entityTypeHandler is the handler for entity type management operations. Each handler instance
// is bound to a single TypeCategory so the same code path serves both /user-types and
// /agent-types with the category injected at construction time.
type entityTypeHandler struct {
	entityTypeService EntityTypeServiceInterface
	category          TypeCategory
}

// newEntityTypeHandler creates a new instance of entityTypeHandler bound to the given category.
func newEntityTypeHandler(entityTypeService EntityTypeServiceInterface,
	category TypeCategory) *entityTypeHandler {
	return &entityTypeHandler{
		entityTypeService: entityTypeService,
		category:          category,
	}
}

// HandleEntityTypeListRequest handles the entity type list request.
func (h *entityTypeHandler) HandleEntityTypeListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, entityTypeHandlerLoggerComponentName))

	limit, offset, svcErr := parsePaginationParams(r.URL.Query())
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	if limit == 0 {
		limit = serverconst.DefaultPageSize
	}

	includeDisplay := r.URL.Query().Get(sysutils.QueryParamInclude) == sysutils.IncludeValueDisplay

	entityTypeListResponse, svcErr := h.entityTypeService.GetEntityTypeList(
		ctx, h.category, limit, offset, includeDisplay)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, entityTypeListResponse)

	logger.Debug("Successfully listed entity types with pagination",
		log.String("category", string(h.category)),
		log.Int("limit", limit), log.Int("offset", offset),
		log.Int("totalResults", entityTypeListResponse.TotalResults),
		log.Int("count", entityTypeListResponse.Count))
}

// HandleEntityTypePostRequest handles the entity type creation request.
func (h *entityTypeHandler) HandleEntityTypePostRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, entityTypeHandlerLoggerComponentName))

	createRequest, err := sysutils.DecodeJSONBody[CreateEntityTypeRequest](r)
	if err != nil {
		errResp := apierror.ErrorResponse{
			Code:    ErrorInvalidRequestFormat.Code,
			Message: ErrorInvalidRequestFormat.Error,
			Description: core.I18nMessage{
				Key:          "error.entitytypeservice.create_schema_request_parse_failed_description",
				DefaultValue: "Failed to parse request body"},
		}

		sysutils.WriteErrorResponse(w, http.StatusBadRequest, errResp)
		return
	}

	sanitizedRequest := h.sanitizeCreateEntityTypeRequest(*createRequest)

	createdEntityType, svcErr := h.entityTypeService.CreateEntityType(ctx, h.category,
		CreateEntityTypeRequestWithID{
			Name:                  sanitizedRequest.Name,
			OUID:                  sanitizedRequest.OUID,
			AllowSelfRegistration: sanitizedRequest.AllowSelfRegistration,
			SystemAttributes:      sanitizedRequest.SystemAttributes,
			Schema:                sanitizedRequest.Schema,
		})
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusCreated, createdEntityType)

	logger.Debug("Successfully created entity type",
		log.String("category", string(h.category)),
		log.String("entityTypeID", createdEntityType.ID), log.String("name", createdEntityType.Name))
}

// HandleEntityTypeGetRequest handles the entity type get request.
func (h *entityTypeHandler) HandleEntityTypeGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, entityTypeHandlerLoggerComponentName))

	schemaID, idValidationFailed := extractAndValidateSchemaID(w, r)
	if idValidationFailed {
		return
	}

	includeDisplay := r.URL.Query().Get(sysutils.QueryParamInclude) == sysutils.IncludeValueDisplay

	entityType, svcErr := h.entityTypeService.GetEntityType(ctx, h.category, schemaID, includeDisplay)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, entityType)

	logger.Debug("Successfully retrieved entity type",
		log.String("category", string(h.category)), log.String("entityTypeID", schemaID))
}

// HandleEntityTypePutRequest handles the entity type update request.
func (h *entityTypeHandler) HandleEntityTypePutRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, entityTypeHandlerLoggerComponentName))

	schemaID, idValidationFailed := extractAndValidateSchemaID(w, r)
	if idValidationFailed {
		return
	}

	sanitizedRequest, requestValidationFailed := validateUpdateEntityTypeRequest(w, r, h)
	if requestValidationFailed {
		return
	}

	updatedEntityType, svcErr := h.entityTypeService.UpdateEntityType(
		ctx, h.category, schemaID, sanitizedRequest)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, updatedEntityType)

	logger.Debug("Successfully updated entity type",
		log.String("category", string(h.category)),
		log.String("entityTypeID", schemaID), log.String("name", updatedEntityType.Name))
}

// HandleEntityTypeDeleteRequest handles the entity type delete request.
func (h *entityTypeHandler) HandleEntityTypeDeleteRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, entityTypeHandlerLoggerComponentName))

	schemaID, idValidationFailed := extractAndValidateSchemaID(w, r)
	if idValidationFailed {
		return
	}

	svcErr := h.entityTypeService.DeleteEntityType(ctx, h.category, schemaID)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusNoContent, nil)
	logger.Debug("Successfully deleted entity type",
		log.String("category", string(h.category)), log.String("entityTypeID", schemaID))
}

// parsePaginationParams parses limit and offset from query parameters.
func parsePaginationParams(query map[string][]string) (int, int, *serviceerror.ServiceError) {
	var limit, offset int
	var err error

	if limitStr := query["limit"]; len(limitStr) > 0 && limitStr[0] != "" {
		sanitizedLimit := sysutils.SanitizeString(limitStr[0])
		limit, err = strconv.Atoi(sanitizedLimit)
		if err != nil || limit <= 0 {
			return 0, 0, &ErrorInvalidLimit
		}
	}

	if offsetStr := query["offset"]; len(offsetStr) > 0 && offsetStr[0] != "" {
		sanitizedOffset := sysutils.SanitizeString(offsetStr[0])
		offset, err = strconv.Atoi(sanitizedOffset)
		if err != nil || offset < 0 {
			return 0, 0, &ErrorInvalidOffset
		}
	}

	return limit, offset, nil
}

// handleError handles service errors and converts them to appropriate HTTP responses.
func handleError(w http.ResponseWriter, svcErr *serviceerror.ServiceError) {
	var statusCode int
	if svcErr.Type == serviceerror.ClientErrorType {
		statusCode = http.StatusBadRequest
		if svcErr.Code == ErrorEntityTypeNotFound.Code {
			statusCode = http.StatusNotFound
		} else if svcErr.Code == ErrorEntityTypeNameConflict.Code {
			statusCode = http.StatusConflict
		} else if svcErr.Code == ErrorCannotModifyDeclarativeResource.Code {
			statusCode = http.StatusForbidden
		} else if svcErr.Code == ErrorResultLimitExceededInCompositeMode.Code {
			statusCode = http.StatusBadRequest
		} else if svcErr.Code == serviceerror.ErrorUnauthorized.Code {
			statusCode = http.StatusForbidden
		}
	} else {
		statusCode = http.StatusInternalServerError
	}

	errResp := apierror.ErrorResponse{
		Code:        svcErr.Code,
		Message:     svcErr.Error,
		Description: svcErr.ErrorDescription,
	}

	sysutils.WriteErrorResponse(w, statusCode, errResp)
}

// extractAndValidateSchemaID extracts and validates the schema ID from the URL path.
func extractAndValidateSchemaID(w http.ResponseWriter, r *http.Request) (string, bool) {
	schemaID := r.PathValue("id")
	if schemaID == "" {
		errResp := apierror.ErrorResponse{
			Code:        ErrorInvalidEntityTypeRequest.Code,
			Message:     ErrorInvalidEntityTypeRequest.Error,
			Description: ErrorInvalidEntityTypeRequest.ErrorDescription,
		}
		sysutils.WriteErrorResponse(w, http.StatusBadRequest, errResp)
		return "", true
	}

	return schemaID, false
}

func validateUpdateEntityTypeRequest(
	w http.ResponseWriter, r *http.Request, h *entityTypeHandler,
) (UpdateEntityTypeRequest, bool) {
	updateRequest, err := sysutils.DecodeJSONBody[UpdateEntityTypeRequest](r)
	if err != nil {
		errResp := apierror.ErrorResponse{
			Code:    ErrorInvalidRequestFormat.Code,
			Message: ErrorInvalidRequestFormat.Error,
			Description: core.I18nMessage{
				Key:          "error.entitytypeservice.update_schema_request_parse_failed_description",
				DefaultValue: "Failed to parse request body"},
		}
		sysutils.WriteErrorResponse(w, http.StatusBadRequest, errResp)
		return UpdateEntityTypeRequest{}, true
	}

	sanitizedRequest := h.sanitizeUpdateEntityTypeRequest(*updateRequest)
	return sanitizedRequest, false
}

// sanitizeCreateEntityTypeRequest sanitizes the create entity type request input.
func (h *entityTypeHandler) sanitizeCreateEntityTypeRequest(
	request CreateEntityTypeRequest,
) CreateEntityTypeRequest {
	sanitizedName := sysutils.SanitizeString(request.Name)
	sanitizedOUID := sysutils.SanitizeString(request.OUID)

	return CreateEntityTypeRequest{
		Name:                  sanitizedName,
		OUID:                  sanitizedOUID,
		AllowSelfRegistration: request.AllowSelfRegistration,
		SystemAttributes:      sanitizeSystemAttributes(request.SystemAttributes),
		Schema:                request.Schema,
	}
}

// sanitizeUpdateEntityTypeRequest sanitizes the update entity type request input.
func (h *entityTypeHandler) sanitizeUpdateEntityTypeRequest(
	request UpdateEntityTypeRequest,
) UpdateEntityTypeRequest {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, entityTypeHandlerLoggerComponentName))

	originalName := request.Name
	sanitizedName := sysutils.SanitizeString(request.Name)
	sanitizedOUID := sysutils.SanitizeString(request.OUID)

	if originalName != sanitizedName {
		logger.Debug("Sanitized entity type name in update request",
			log.MaskedString("original", originalName),
			log.MaskedString("sanitized", sanitizedName))
	}

	return UpdateEntityTypeRequest{
		Name:                  sanitizedName,
		OUID:                  sanitizedOUID,
		AllowSelfRegistration: request.AllowSelfRegistration,
		SystemAttributes:      sanitizeSystemAttributes(request.SystemAttributes),
		Schema:                request.Schema,
	}
}

// sanitizeSystemAttributes sanitizes the SystemAttributes fields.
func sanitizeSystemAttributes(sa *SystemAttributes) *SystemAttributes {
	if sa == nil {
		return nil
	}
	return &SystemAttributes{
		Display: sysutils.SanitizeString(sa.Display),
	}
}
