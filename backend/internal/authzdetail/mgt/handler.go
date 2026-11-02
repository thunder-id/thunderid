// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzdetailmgt

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// authorizationDetailHandler handles HTTP requests for authorization details type management.
type authorizationDetailHandler struct {
	service AuthorizationDetailMgtServiceInterface
}

// newAuthorizationDetailHandler creates a new authorization details type handler.
func newAuthorizationDetailHandler(service AuthorizationDetailMgtServiceInterface) *authorizationDetailHandler {
	return &authorizationDetailHandler{service: service}
}

// HandleTypeListRequest lists the authorization details types, a page at a time.
func (h *authorizationDetailHandler) HandleTypeListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	limit, offset, ok := parsePagination(r.URL.Query())
	if !ok {
		handleError(ctx, w, &ErrorInvalidPagination)
		return
	}
	page, svcErr := h.service.ListAuthorizationDetailTypes(ctx, limit, offset)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	response := AuthorizationDetailTypeListResponse{
		TotalResults: page.TotalResults,
		StartIndex:   offset + 1,
		Count:        len(page.Types),
		Types:        make([]AuthorizationDetailTypeResponse, 0, len(page.Types)),
		Links: sysutils.BuildPaginationLinks("/authorization-detail-types", limit, offset,
			page.TotalResults, ""),
	}
	for _, t := range page.Types {
		response.Types = append(response.Types, toTypeResponse(t))
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, response)
}

// parsePagination reads the limit and offset query parameters, defaulting to the first page.
func parsePagination(query url.Values) (int, int, bool) {
	limit, offset := serverconst.DefaultPageSize, 0
	var err error
	if v := query.Get("limit"); v != "" {
		if limit, err = strconv.Atoi(v); err != nil {
			return 0, 0, false
		}
	}
	if v := query.Get("offset"); v != "" {
		if offset, err = strconv.Atoi(v); err != nil {
			return 0, 0, false
		}
	}
	return limit, offset, true
}

// HandleTypePostRequest registers an authorization details type.
func (h *authorizationDetailHandler) HandleTypePostRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req, ok := decodeTypeRequest(ctx, w, r)
	if !ok {
		return
	}
	t, svcErr := toType(req)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	created, svcErr := h.service.CreateAuthorizationDetailType(ctx, t)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusCreated, toTypeResponse(*created))
}

// HandleTypeGetRequest retrieves an authorization details type.
func (h *authorizationDetailHandler) HandleTypeGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	t, svcErr := h.service.GetAuthorizationDetailType(ctx, r.PathValue("id"))
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, toTypeResponse(*t))
}

// HandleTypePutRequest updates an authorization details type.
func (h *authorizationDetailHandler) HandleTypePutRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req, ok := decodeTypeRequest(ctx, w, r)
	if !ok {
		return
	}
	t, svcErr := toType(req)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	updated, svcErr := h.service.UpdateAuthorizationDetailType(ctx, r.PathValue("id"), t)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, toTypeResponse(*updated))
}

// HandleTypeDeleteRequest deletes an authorization details type.
func (h *authorizationDetailHandler) HandleTypeDeleteRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if svcErr := h.service.DeleteAuthorizationDetailType(ctx, r.PathValue("id")); svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// maxTypeRequestBodyBytes bounds the body of a create or update request.
const maxTypeRequestBodyBytes = 1 << 20

// decodeTypeRequest decodes a type request body, writing the error response when it cannot.
func decodeTypeRequest(ctx context.Context, w http.ResponseWriter,
	r *http.Request) (*AuthorizationDetailTypeRequest, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxTypeRequestBodyBytes)
	// A misspelled key, such as one for the processor, is refused rather than dropped.
	body, err := sysutils.DecodeJSONBodyStrict[typeRequestWire](r)
	if err != nil {
		handleError(ctx, w, &ErrorInvalidRequestFormat)
		return nil, false
	}
	return &body.AuthorizationDetailTypeRequest, true
}

// toType converts a request body to a type, carrying the processor credential as sent for the service to
// resolve.
func toType(req *AuthorizationDetailTypeRequest) (providers.AuthorizationDetailType, *tidcommon.ServiceError) {
	t := typeFromRequest(req)
	if req.Processor != nil {
		properties, svcErr := sentAuthenticationProperties(req.Processor.Authentication)
		if svcErr != nil {
			return providers.AuthorizationDetailType{}, svcErr
		}
		t.Processor.AuthenticationProperties = properties
	}
	return t, nil
}

// typeFromRequest converts a request body to a type, without its processor credential.
func typeFromRequest(req *AuthorizationDetailTypeRequest) providers.AuthorizationDetailType {
	t := providers.AuthorizationDetailType{
		Type:            req.Type,
		Name:            sysutils.SanitizeString(req.Name),
		Description:     sysutils.SanitizeString(req.Description),
		Schema:          req.Schema,
		ConsentTemplate: req.ConsentTemplate,
		Reuse:           req.Reuse != nil && *req.Reuse,
	}
	if req.Processor != nil {
		t.Processor = &providers.AuthorizationDetailProcessor{Endpoint: req.Processor.Endpoint}
	}
	return t
}

// toTypeResponse converts a type to its response body with processor credentials masked.
func toTypeResponse(t providers.AuthorizationDetailType) AuthorizationDetailTypeResponse {
	return AuthorizationDetailTypeResponse{
		ID:              t.ID,
		Type:            t.Type,
		Name:            t.Name,
		Description:     t.Description,
		Version:         t.Version,
		Schema:          t.Schema,
		ConsentTemplate: t.ConsentTemplate,
		Processor:       processorResponse(t.Processor),
		Reuse:           t.Reuse,
		IsReadOnly:      t.IsReadOnly,
	}
}

// processorResponse returns a processor for the management API with credentials masked.
func processorResponse(processor *providers.AuthorizationDetailProcessor) *ProcessorResponse {
	if processor == nil {
		return nil
	}
	authentication, err := outboundauth.AuthenticationFromProperties(processor.AuthenticationProperties)
	if err != nil {
		authentication = outboundauth.Authentication{Type: string(outboundauth.TypeNone)}
	}
	return &ProcessorResponse{Endpoint: processor.Endpoint, Authentication: authentication}
}

// handleError writes the error response for a service error.
func handleError(ctx context.Context, w http.ResponseWriter, svcErr *tidcommon.ServiceError) {
	statusCode := http.StatusInternalServerError
	if svcErr.Type == tidcommon.ClientErrorType {
		switch svcErr.Code {
		case ErrorTypeNotFound.Code:
			statusCode = http.StatusNotFound
		case ErrorTypeConflict.Code, ErrorTypeIDConflict.Code, ErrorTypeChanged.Code:
			statusCode = http.StatusConflict
		default:
			statusCode = http.StatusBadRequest
		}
	}

	sysutils.WriteErrorResponse(ctx, w, statusCode, apierror.ErrorResponse{
		Code:        svcErr.Code,
		Message:     svcErr.Error,
		Description: svcErr.ErrorDescription,
	})
}
