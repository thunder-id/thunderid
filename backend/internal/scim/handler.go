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

package scim

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// scimHandler handles SCIM HTTP requests.
type scimHandler struct {
	svc     SCIMServiceInterface
	baseURL string
	logger  *log.Logger
}

// newSCIMHandler creates a new scimHandler instance.
func newSCIMHandler(svc SCIMServiceInterface, baseURL string) *scimHandler {
	return &scimHandler{
		svc:     svc,
		baseURL: baseURL,
		logger:  log.GetLogger().With(log.String(log.LoggerKeyComponentName, "SCIMHandler")),
	}
}

// HandleServiceProviderConfigGetRequest handles GET /scim/v2/ServiceProviderConfig.
func (sh *scimHandler) HandleServiceProviderConfigGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := sh.logger

	config := sh.svc.GetServiceProviderConfig(ctx, sh.baseURL)
	writeSCIMSuccessResponse(ctx, w, config)

	logger.Debug(ctx, "SCIM ServiceProviderConfig GET response sent")
}

// HandleSchemaListRequest handles GET /scim/v2/Schemas.
// Returns all SCIM schemas: the core User schema plus one per ThunderID user-type, entity type.
func (sh *scimHandler) HandleSchemaListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := sh.logger

	listResp, svcErr := sh.svc.ListSchemas(ctx, sh.baseURL)
	if svcErr != nil {
		sh.handleSCIMError(w, r, svcErr)
		return
	}

	writeSCIMSuccessResponse(ctx, w, listResp)
	logger.Debug(ctx, "SCIM Schemas list response sent",
		log.Int("totalResults", listResp.TotalResults))
}

// HandleSchemaGetRequest handles GET /scim/v2/Schemas/{id}.
// The {id} path value is the full SCIM schema URN (e.g.
// urn:ietf:params:scim:schemas:core:2.0:User or a ThunderID extension URN).
func (sh *scimHandler) HandleSchemaGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := sh.logger
	schemaURN := r.PathValue("id")
	if schemaURN == "" {
		sh.handleSCIMError(w, r, &ErrorSchemaNotFound)
		return
	}

	schema, svcErr := sh.svc.GetSchema(ctx, schemaURN, sh.baseURL)
	if svcErr != nil {
		sh.handleSCIMError(w, r, svcErr)
		return
	}

	writeSCIMSuccessResponse(ctx, w, schema)
	logger.Debug(ctx, "SCIM Schema GET response sent", log.String("urn", schemaURN))
}

// HandleResourceTypeListRequest handles GET /scim/v2/ResourceTypes.
// Returns all SCIM resource types. ThunderID only exposes a single "User" resource type.
func (sh *scimHandler) HandleResourceTypeListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := sh.logger
	listResp, svcErr := sh.svc.ListResourceTypes(ctx, sh.baseURL)
	if svcErr != nil {
		sh.handleSCIMError(w, r, svcErr)
		return
	}

	writeSCIMSuccessResponse(ctx, w, listResp)
	logger.Debug(ctx, "SCIM ResourceTypes list response sent",
		log.Int("totalResults", listResp.TotalResults))
}

// HandleResourceTypeGetRequest handles GET /scim/v2/ResourceTypes/{id}.
// The {id} path value is the resource type name — "User" is the only supported value.
func (sh *scimHandler) HandleResourceTypeGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := sh.logger

	resourceTypeID := r.PathValue("id")
	if resourceTypeID == "" {
		sh.handleSCIMError(w, r, &ErrorResourceTypeNotFound)
		return
	}

	rt, svcErr := sh.svc.GetResourceType(ctx, resourceTypeID, sh.baseURL)
	if svcErr != nil {
		sh.handleSCIMError(w, r, svcErr)
		return
	}

	writeSCIMSuccessResponse(ctx, w, rt)
	logger.Debug(ctx, "SCIM ResourceType GET response sent", log.String("id", resourceTypeID))
}

// HandleUnsupportedRequest returns a SCIM-standard 501 for unimplemented endpoints.
// Delegates to handleSCIMError so that all error paths go through the same translator.
func (sh *scimHandler) handleUnsupportedRequest(w http.ResponseWriter, r *http.Request) {
	sh.handleSCIMError(w, r, &ErrorUnsupportedOperation)
}

// handleSCIMError translates an internal ThunderID ServiceError into the
// SCIM-standard wire error response (RFC 7644 §3.12).
// Internal codes (SCIM-1001 etc.) are NEVER sent to the client.
func (sh *scimHandler) handleSCIMError(w http.ResponseWriter, r *http.Request, svcErr *tidcommon.ServiceError) {
	ctx := r.Context()

	// Server errors always map to 500 with no scimType.
	if svcErr.Type == tidcommon.ServerErrorType {
		writeSCIMErrorResponse(ctx, w, http.StatusInternalServerError, SCIMErrorResponse{
			Schemas: []string{SCIMErrorSchemaURN},
			Status:  "500",
			Detail:  "An unexpected error occurred",
		})
		return
	}

	// Map internal client error codes → SCIM standard HTTP status + scimType.
	var httpStatus int
	var scimType string

	switch svcErr.Code {
	// 400 invalidSyntax — body could not be parsed at all.
	case ErrorInvalidRequestBody.Code:
		httpStatus = http.StatusBadRequest
		scimType = "invalidSyntax"

	// 400 invalidValue — missing or malformed fields/schemas/URNs.
	case ErrorMissingSchemas.Code,
		ErrorDuplicateSchemas.Code,
		ErrorMissingCoreUserSchema.Code,
		ErrorMissingCustomSchema.Code,
		ErrorMultipleCustomSchemas.Code,
		ErrorInvalidCustomSchemaURN.Code,
		ErrorMissingCustomSchemaObject.Code,
		ErrorUnknownUserType.Code:
		httpStatus = http.StatusBadRequest
		scimType = "invalidValue"

	// 404 — resource not found.
	case ErrorUserNotFound.Code,
		ErrorSchemaNotFound.Code,
		ErrorResourceTypeNotFound.Code:
		httpStatus = http.StatusNotFound
		scimType = ""

	// 501 — unsupported operation.
	case ErrorUnsupportedOperation.Code:
		httpStatus = http.StatusNotImplemented
		scimType = "notImplemented"

	// 403 — authorization failure.
	case tidcommon.ErrorUnauthorized.Code:
		httpStatus = http.StatusForbidden
		scimType = ""

	case ErrorInternalServer.Code:
		httpStatus = http.StatusInternalServerError
		scimType = ""

	default:
		httpStatus = http.StatusBadRequest
		scimType = "invalidValue"
	}

	writeSCIMErrorResponse(ctx, w, httpStatus, SCIMErrorResponse{
		Schemas:  []string{SCIMErrorSchemaURN},
		Status:   fmt.Sprintf("%d", httpStatus),
		ScimType: scimType,
		Detail:   svcErr.ErrorDescription.DefaultValue,
	})
}

// writeSCIMSuccessResponse writes a SCIM-compliant success response.
// Uses application/scim+json as required by RFC 7644, and uses a
// buffer-first pattern to avoid sending headers before encoding succeeds.
func writeSCIMSuccessResponse(ctx context.Context, w http.ResponseWriter, data any) {
	logger := log.GetLogger()

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(data); err != nil {
		logger.Error(ctx, "Failed to encode SCIM response", log.Error(err))
		w.Header().Set("Content-Type", constants.SCIMContentType)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", constants.SCIMContentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}

// writeSCIMErrorResponse writes a SCIM-standard error response.
// Uses the same buffer-first pattern as writeSCIMSuccessResponse so that
// headers are never committed before encoding is confirmed to succeed.
// Always sends the SCIM wire format — never internal ThunderID error codes.
func writeSCIMErrorResponse(ctx context.Context, w http.ResponseWriter, statusCode int, scimErr SCIMErrorResponse) {
	logger := log.GetLogger()

	if len(scimErr.Schemas) == 0 {
		scimErr.Schemas = []string{SCIMErrorSchemaURN}
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(scimErr); err != nil {
		logger.Error(ctx, "Failed to encode SCIM error response", log.Error(err))
		w.Header().Set("Content-Type", constants.SCIMContentType)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", constants.SCIMContentType)
	w.WriteHeader(statusCode)
	_, _ = w.Write(buf.Bytes())
}
