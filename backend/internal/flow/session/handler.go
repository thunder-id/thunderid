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

// HandleSessionRevokeRequest handles DELETE /sessions/{id}. It responds 204 even when the session is already
// gone or out of the caller's reach.
func (h *sessionMgtHandler) HandleSessionRevokeRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if svcErr := h.svc.RevokeSession(ctx, r.PathValue("id")); svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusNoContent, nil)
}

// HandleUserSessionsRevokeRequest handles DELETE /sessions?userId=.
func (h *sessionMgtHandler) HandleUserSessionsRevokeRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if svcErr := h.svc.RevokeUserSessions(ctx, r.URL.Query().Get("userId")); svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusNoContent, nil)
}

// HandleSelfSessionRevokeRequest handles DELETE /users/me/sessions/{id}. It responds 204 even when the caller has
// no such session.
func (h *sessionMgtHandler) HandleSelfSessionRevokeRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !selfServiceCaller(ctx, w) {
		return
	}
	if svcErr := h.svc.RevokeOwnSession(ctx, r.PathValue("id")); svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusNoContent, nil)
}

// HandleSelfSessionsRevokeRequest handles DELETE /users/me/sessions, optionally with exceptCurrent=true to keep
// the session the caller's access token was issued in.
func (h *sessionMgtHandler) HandleSelfSessionsRevokeRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !selfServiceCaller(ctx, w) {
		return
	}
	v := r.URL.Query().Get("exceptCurrent")
	if v != "" && v != "true" && v != "false" {
		handleError(ctx, w, &ErrorInvalidExceptCurrent)
		return
	}
	if svcErr := h.svc.RevokeOwnSessions(ctx, v == "true"); svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusNoContent, nil)
}

// selfServiceCaller reports whether the caller may revoke its own sessions, writing the error response when it
// may not: the token must name a subject, and a delegated token, issued to a client acting for the user, is
// refused.
func selfServiceCaller(ctx context.Context, w http.ResponseWriter) bool {
	if strings.TrimSpace(security.GetSubject(ctx)) == "" {
		sysutils.WriteErrorResponse(ctx, w, http.StatusUnauthorized, apierror.ErrUnauthorized)
		return false
	}
	if security.IsDelegated(ctx) {
		handleError(ctx, w, &ErrorDelegatedTokenNotAllowed)
		return false
	}
	return true
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
	case svcErr.Code == tidcommon.ErrorUnauthorized.Code, svcErr.Code == ErrorDelegatedTokenNotAllowed.Code:
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
