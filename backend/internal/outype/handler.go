// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package outype

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// ouTypeHandler is the handler for organization unit type management operations.
type ouTypeHandler struct {
	service OUTypeServiceInterface
}

// newOUTypeHandler creates a new organization unit type handler.
func newOUTypeHandler(service OUTypeServiceInterface) *ouTypeHandler {
	return &ouTypeHandler{service: service}
}

// HandleOUTypeListRequest handles the organization unit type list request.
func (h *ouTypeHandler) HandleOUTypeListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	limit, offset, svcErr := parsePaginationParams(r)
	if svcErr != nil {
		writeOUTypeError(ctx, w, svcErr)
		return
	}

	list, svcErr := h.service.GetOUTypeList(ctx, limit, offset, includeDisplay(r))
	if svcErr != nil {
		writeOUTypeError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, list)
}

// HandleOUTypePostRequest handles the organization unit type creation request.
func (h *ouTypeHandler) HandleOUTypePostRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	request, ok := decodeRequest[CreateOUTypeRequest](w, r)
	if !ok {
		return
	}

	created, svcErr := h.service.CreateOUType(ctx, CreateOUTypeRequest{
		Handle:           sysutils.SanitizeString(request.Handle),
		DisplayName:      sysutils.SanitizeString(request.DisplayName),
		OUID:             sysutils.SanitizeString(request.OUID),
		SystemAttributes: sanitizeSystemAttributes(request.SystemAttributes),
		Schema:           request.Schema,
	})
	if svcErr != nil {
		writeOUTypeError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusCreated, created)
}

// HandleOUTypeGetRequest handles the organization unit type get request.
func (h *ouTypeHandler) HandleOUTypeGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	ouType, svcErr := h.service.GetOUType(ctx, r.PathValue("id"), includeDisplay(r))
	if svcErr != nil {
		writeOUTypeError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, ouType)
}

// HandleOUTypePutRequest handles the organization unit type update request.
func (h *ouTypeHandler) HandleOUTypePutRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	request, ok := decodeRequest[UpdateOUTypeRequest](w, r)
	if !ok {
		return
	}

	updated, svcErr := h.service.UpdateOUType(ctx, r.PathValue("id"), UpdateOUTypeRequest{
		Handle:           sysutils.SanitizeString(request.Handle),
		DisplayName:      sysutils.SanitizeString(request.DisplayName),
		OUID:             sysutils.SanitizeString(request.OUID),
		SystemAttributes: sanitizeSystemAttributes(request.SystemAttributes),
		Schema:           request.Schema,
	})
	if svcErr != nil {
		writeOUTypeError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, updated)
}

// HandleOUTypeDeleteRequest handles the organization unit type delete request.
func (h *ouTypeHandler) HandleOUTypeDeleteRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if svcErr := h.service.DeleteOUType(ctx, r.PathValue("id")); svcErr != nil {
		writeOUTypeError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusNoContent, nil)
}

// decodeRequest decodes a request body and refuses a field the API does not define, so a setting
// organization unit types do not support, such as allowSelfRegistration, is never silently dropped.
func decodeRequest[T any](w http.ResponseWriter, r *http.Request) (*T, bool) {
	request, err := sysutils.DecodeJSONBodyStrict[T](r)
	if err != nil {
		var valErr *sysutils.ValidationError
		if errors.As(err, &valErr) {
			sysutils.WriteStructuredErrorResponse(w, http.StatusBadRequest, "Validation Failed", valErr.Errors)
			return nil, false
		}
		writeOUTypeError(r.Context(), w, &ErrorInvalidRequestFormat)
		return nil, false
	}
	return request, true
}

// includeDisplay reports whether the request asks for the handle of the owning organization unit.
func includeDisplay(r *http.Request) bool {
	return r.URL.Query().Get(sysutils.QueryParamInclude) == sysutils.IncludeValueDisplay
}

// parsePaginationParams parses the limit and offset query parameters.
func parsePaginationParams(r *http.Request) (int, int, *tidcommon.ServiceError) {
	query := r.URL.Query()
	limit := serverconst.DefaultPageSize
	offset := 0
	var err error

	if limitStr := sysutils.SanitizeString(query.Get("limit")); limitStr != "" {
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit <= 0 {
			return 0, 0, &ErrorInvalidLimit
		}
	}
	if offsetStr := sysutils.SanitizeString(query.Get("offset")); offsetStr != "" {
		offset, err = strconv.Atoi(offsetStr)
		if err != nil || offset < 0 {
			return 0, 0, &ErrorInvalidOffset
		}
	}
	return limit, offset, nil
}

// sanitizeSystemAttributes sanitizes the system attributes of a request.
func sanitizeSystemAttributes(attrs *SystemAttributes) *SystemAttributes {
	if attrs == nil {
		return nil
	}
	return &SystemAttributes{Display: sysutils.SanitizeString(attrs.Display)}
}

// writeOUTypeError writes a service error as an HTTP error response.
func writeOUTypeError(ctx context.Context, w http.ResponseWriter, svcErr *tidcommon.ServiceError) {
	status := http.StatusInternalServerError
	if svcErr.Type == tidcommon.ClientErrorType {
		switch svcErr.Code {
		case ErrorOUTypeNotFound.Code:
			status = http.StatusNotFound
		case ErrorOUTypeHandleConflict.Code:
			status = http.StatusConflict
		case ErrorCannotModifyDeclarativeOUType.Code, ErrorCannotModifyManagedOUType.Code,
			tidcommon.ErrorUnauthorized.Code:
			status = http.StatusForbidden
		default:
			status = http.StatusBadRequest
		}
	}

	sysutils.WriteErrorResponse(ctx, w, status, apierror.ErrorResponse{
		Code:        svcErr.Code,
		Message:     svcErr.Error,
		Description: svcErr.ErrorDescription,
	})
}
