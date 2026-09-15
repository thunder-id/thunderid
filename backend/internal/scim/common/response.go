// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

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

// mapSCIMError translates a ServiceError code into an HTTP status code and
// SCIM scimType string. scimDiscoveryHandler, scimUsersHandler, and scimGroupsHandler all
// delegate to this function so that the mapping cannot drift between handlers.
func mapSCIMError(svcErr *tidcommon.ServiceError) (httpStatus int, scimType ScimErrorType) {
	switch svcErr.Code {
	// 400 invalidSyntax — body could not be parsed at all.
	case ErrorInvalidRequestBody.Code, errorInvalidContentType.Code:
		return http.StatusBadRequest, ScimErrorTypeInvalidSyntax

	// 400 invalidValue — missing or malformed fields/schemas/URNs.
	case ErrorMissingSchemas.Code,
		ErrorDuplicateSchemas.Code,
		ErrorMissingCoreUserSchema.Code,
		ErrorMissingCoreGroupSchema.Code,
		ErrorMissingCustomSchema.Code,
		ErrorMultipleCustomSchemas.Code,
		ErrorInvalidCustomSchemaURN.Code,
		ErrorMissingCustomSchemaObject.Code,
		ErrorUndeclaredCustomSchemaObject.Code,
		ErrorEnterpriseSchemaNotSupported.Code,
		ErrorUndeclaredEnterpriseSchemaObject.Code,
		ErrorUnknownUserType.Code,
		ErrorSchemaValidationFailed.Code,
		ErrorMutabilityViolation.Code,
		ErrorInvalidGroupMember.Code,
		ErrorInvalidPatchOp.Code,
		ErrorInvalidPatchValue.Code,
		ErrorConflictingAttributesParams.Code,
		ErrorSortNotSupported.Code,
		ErrorConflictingAttributeValue.Code,
		ErrorUnsupportedMemberType.Code:
		return http.StatusBadRequest, ScimErrorTypeInvalidValue

		// 400 invalidPath — PATCH "path", or an "attributes"/"excludedAttributes" entry,
		// is missing, unsupported, or malformed.
	case ErrorInvalidPatchPath.Code, errorCustomAttributeRequiresURN.Code, errorUnrecognizedSchemaURN.Code,
		errorSubAttributePathNotSupportedForProjection.Code:
		return http.StatusBadRequest, ScimErrorTypeInvalidPath

		// 400 mutability — request attempted to change an immutable attribute.
	case ErrorImmutableUserType.Code:
		return http.StatusBadRequest, ScimErrorTypeMutability

	// 400 invalidFilter — filter query parameter is not supported or not syntactically valid.
	case ErrorFilterNotSupported.Code, errorInvalidFilterSyntax.Code:
		return http.StatusBadRequest, ScimErrorTypeInvalidFilter

	// 404 — resource not found.
	case ErrorUserNotFound.Code,
		ErrorSchemaNotFound.Code,
		ErrorResourceTypeNotFound.Code,
		ErrorResourceNotFound.Code:
		return http.StatusNotFound, ""

	// 409 — uniqueness conflict.
	case ErrorUniquenessConflict.Code:
		return http.StatusConflict, ScimErrorTypeUniqueness

	// 501 — unsupported operation.
	case ErrorUnsupportedOperation.Code:
		return http.StatusNotImplemented, ScimErrorTypeNotImplemented

	// 401 — no authenticated subject present.
	case ErrorUnauthenticated.Code:
		return http.StatusUnauthorized, ""

	// 403 — authorization failure.
	case tidcommon.ErrorUnauthorized.Code:
		return http.StatusForbidden, ""

	case tidcommon.InternalServerError.Code:
		return http.StatusInternalServerError, ""

	default:
		return http.StatusBadRequest, ScimErrorTypeInvalidValue
	}
}

// WriteSCIMSuccessResponse writes a SCIM-compliant success response.
// Uses application/scim+json as required by RFC 7644, and uses a
// buffer-first pattern to avoid sending headers before encoding succeeds.
func WriteSCIMSuccessResponse(
	ctx context.Context, w http.ResponseWriter, statusCode int, data any, logger log.Logger,
) {
	if statusCode == http.StatusNoContent {
		w.WriteHeader(statusCode)
		return
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(data); err != nil {
		logger.Error(ctx, "Failed to encode SCIM response", log.Error(err))
		w.Header().Set("Content-Type", constants.SCIMContentType)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", constants.SCIMContentType)
	w.WriteHeader(statusCode)
	_, _ = w.Write(buf.Bytes())
}

// WriteSCIMErrorResponse writes a SCIM-standard error response.
// Uses the same buffer-first pattern as WriteSCIMSuccessResponse so that
// headers are never committed before encoding is confirmed to succeed.
// Always sends the SCIM wire format — never internal ThunderID error codes.
func WriteSCIMErrorResponse(
	ctx context.Context, w http.ResponseWriter, statusCode int, scimErr SCIMErrorResponse, logger log.Logger,
) {
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

// HandleSCIMError translates an internal ThunderID ServiceError into the
// SCIM-standard wire error response (RFC 7644 §3.12).
// Internal codes (SCIM-1001 etc.) are NEVER sent to the client.
func HandleSCIMError(w http.ResponseWriter, r *http.Request, svcErr *tidcommon.ServiceError, logger log.Logger) {
	ctx := r.Context()

	if svcErr.Type == tidcommon.ServerErrorType {
		WriteSCIMErrorResponse(ctx, w, http.StatusInternalServerError, SCIMErrorResponse{
			Schemas: []string{SCIMErrorSchemaURN},
			Status:  "500",
			Detail:  svcErr.ErrorDescription.DefaultValue,
		}, logger)
		return
	}

	httpStatus, scimType := mapSCIMError(svcErr)
	WriteSCIMErrorResponse(ctx, w, httpStatus, SCIMErrorResponse{
		Schemas:  []string{SCIMErrorSchemaURN},
		Status:   fmt.Sprintf("%d", httpStatus),
		ScimType: scimType,
		Detail:   svcErr.ErrorDescription.DefaultValue,
	}, logger)
}

// HandleUnsupportedRequest handles unimplemented endpoints by returning a SCIM-standard 501.
// Delegates to HandleSCIMError so that all error paths go through the same translator.
func HandleUnsupportedRequest(w http.ResponseWriter, r *http.Request, logger log.Logger) {
	HandleSCIMError(w, r, &ErrorUnsupportedOperation, logger)
}
