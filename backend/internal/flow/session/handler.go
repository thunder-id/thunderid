// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/security"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// sessionMgtHandler serves the session management endpoints.
type sessionMgtHandler struct {
	svc *service
}

// newSessionMgtHandler creates the session management handler.
func newSessionMgtHandler(svc *service) *sessionMgtHandler {
	return &sessionMgtHandler{svc: svc}
}

// HandleSessionListRequest handles GET /sessions?userId= or GET /sessions?appId=.
func (h *sessionMgtHandler) HandleSessionListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()
	limit, offset, svcErr := parsePaginationParams(r)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	resp, svcErr := h.svc.ListSessions(ctx, query.Get("userId"), query.Get("appId"), limit, offset)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, resp)
}

// HandleSelfSessionListRequest handles GET /users/me/sessions: the caller's own sessions, with the one
// behind the caller's access token marked as current.
func (h *sessionMgtHandler) HandleSelfSessionListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if strings.TrimSpace(security.GetSubject(ctx)) == "" {
		sysutils.WriteErrorResponse(ctx, w, http.StatusUnauthorized, apierror.ErrUnauthorized)
		return
	}
	limit, offset, svcErr := parsePaginationParams(r)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	resp, svcErr := h.svc.ListOwnSessions(ctx, limit, offset)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, resp)
}

// parsePaginationParams parses limit and offset. An omitted limit is the default page size; a limit outside 1 to
// the maximum page size, or a negative offset, is rejected.
func parsePaginationParams(r *http.Request) (int, int, *tidcommon.ServiceError) {
	query := r.URL.Query()
	limit := serverconst.DefaultPageSize
	offset := 0
	if limitStr := query.Get("limit"); limitStr != "" {
		parsed, err := strconv.Atoi(limitStr)
		if err != nil || parsed < 1 || parsed > serverconst.MaxPageSize {
			return 0, 0, &ErrorInvalidLimit
		}
		limit = parsed
	}
	if offsetStr := query.Get("offset"); offsetStr != "" {
		parsed, err := strconv.Atoi(offsetStr)
		if err != nil || parsed < 0 {
			return 0, 0, &ErrorInvalidOffset
		}
		offset = parsed
	}
	return limit, offset, nil
}

// handleError writes the error response for a service error.
func handleError(ctx context.Context, w http.ResponseWriter, svcErr *tidcommon.ServiceError) {
	statusCode := http.StatusInternalServerError
	switch {
	case svcErr.Code == tidcommon.ErrorUnauthorized.Code:
		statusCode = http.StatusForbidden
	case svcErr.Type == tidcommon.ClientErrorType:
		statusCode = http.StatusBadRequest
	}

	sysutils.WriteErrorResponse(ctx, w, statusCode, apierror.ErrorResponse{
		Code:        svcErr.Code,
		Message:     svcErr.Error,
		Description: svcErr.ErrorDescription,
	})
}
