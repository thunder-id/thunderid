// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/thunder-id/thunderid/internal/entitytype"
	scimconfig "github.com/thunder-id/thunderid/internal/scim/config"
	"github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// MapEntityTypeServiceErrorToSCIM translates an entitytype service error into a SCIM
// package error. Shared by the discovery and users services, which both call
// into entitytype.EntityTypeServiceInterface scoped to TypeCategoryUser.
func MapEntityTypeServiceErrorToSCIM(svcErr *tidcommon.ServiceError) *tidcommon.ServiceError {
	if svcErr == nil {
		return nil
	}
	switch svcErr.Code {
	case entitytype.ErrorUserTypeNotFound.Code:
		return &ErrorUnknownUserType
	case tidcommon.ErrorUnauthorized.Code:
		return svcErr
	default:
		if svcErr.Type == tidcommon.ServerErrorType {
			return &ErrorInternalServer
		}
		return &ErrorUnknownUserType
	}
}

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
	case ErrorInvalidPatchPath.Code, errorCustomAttributeRequiresURN.Code, errorUnrecognizedSchemaURN.Code:
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

	case ErrorInternalServer.Code:
		return http.StatusInternalServerError, ""

	// 412 — If-Match precondition failed (optimistic concurrency, RFC 7644 §3.14).
	case ErrorPreconditionFailed.Code:
		return http.StatusPreconditionFailed, ""

	default:
		return http.StatusBadRequest, ScimErrorTypeInvalidValue
	}
}

// WriteSCIMSuccessResponse writes a SCIM-compliant success response.
// Uses application/scim+json as required by RFC 7644, and uses a
// buffer-first pattern to avoid sending headers before encoding succeeds.
// componentName identifies the calling handler for log attribution.
func WriteSCIMSuccessResponse(
	ctx context.Context, w http.ResponseWriter, statusCode int, data any, componentName string,
) {
	if statusCode == http.StatusNoContent {
		w.WriteHeader(statusCode)
		return
	}

	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, componentName))

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
// componentName identifies the calling handler for log attribution.
func WriteSCIMErrorResponse(
	ctx context.Context, w http.ResponseWriter, statusCode int, scimErr SCIMErrorResponse, componentName string,
) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, componentName))

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

// ValidateSCIMContentType enforces RFC 7644 §3.1 — write requests must carry
// Content-Type: application/scim+json.
func ValidateSCIMContentType(r *http.Request) *tidcommon.ServiceError {
	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if !strings.EqualFold(mediaType, constants.SCIMContentType) {
		return &errorInvalidContentType
	}
	return nil
}

// HandleSCIMError translates an internal ThunderID ServiceError into the
// SCIM-standard wire error response (RFC 7644 §3.12).
// Internal codes (SCIM-1001 etc.) are NEVER sent to the client.
// componentName identifies the calling handler for log attribution.
func HandleSCIMError(w http.ResponseWriter, r *http.Request, svcErr *tidcommon.ServiceError, componentName string) {
	ctx := r.Context()

	if svcErr.Type == tidcommon.ServerErrorType {
		WriteSCIMErrorResponse(ctx, w, http.StatusInternalServerError, SCIMErrorResponse{
			Schemas: []string{SCIMErrorSchemaURN},
			Status:  "500",
			Detail:  svcErr.ErrorDescription.DefaultValue,
		}, componentName)
		return
	}

	httpStatus, scimType := mapSCIMError(svcErr)
	WriteSCIMErrorResponse(ctx, w, httpStatus, SCIMErrorResponse{
		Schemas:  []string{SCIMErrorSchemaURN},
		Status:   fmt.Sprintf("%d", httpStatus),
		ScimType: scimType,
		Detail:   svcErr.ErrorDescription.DefaultValue,
	}, componentName)
}

// ParseSCIMPaginationQueryParams extracts and clamps startIndex and count query parameters
// per RFC 7644 §3.4.2.4.
func ParseSCIMPaginationQueryParams(r *http.Request) (int, int) {
	startIndex := 1
	if v := strings.TrimSpace(r.URL.Query().Get("startIndex")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			startIndex = n
		}
	}
	var count *int
	if v := strings.TrimSpace(r.URL.Query().Get("count")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			count = &n
		}
	}
	return NormalizeSCIMPagination(startIndex, count)
}

// NormalizeSCIMPagination clamps startIndex and count per RFC 7644 §3.4.2.4. A nil count means
// count was not specified, and falls back to the default page size; a negative count is
// interpreted as 0 (no resources, totalResults only), matching an explicit 0.
func NormalizeSCIMPagination(startIndex int, count *int) (int, int) {
	if startIndex < 1 {
		startIndex = 1
	}
	resolvedCount := constants.DefaultPageSize
	if count != nil {
		resolvedCount = *count
		if resolvedCount < 0 {
			resolvedCount = 0
		}
	}
	if resolvedCount > scimconfig.FilterMaxResults {
		resolvedCount = scimconfig.FilterMaxResults
	}
	return startIndex, resolvedCount
}

// HandleUnsupportedRequest handles unimplemented endpoints by returning a SCIM-standard 501.
// Delegates to HandleSCIMError so that all error paths go through the same translator.
func HandleUnsupportedRequest(w http.ResponseWriter, r *http.Request, componentName string) {
	HandleSCIMError(w, r, &ErrorUnsupportedOperation, componentName)
}
