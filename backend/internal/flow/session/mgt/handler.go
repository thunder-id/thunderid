// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sessionmgt

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/application"
	flowsession "github.com/thunder-id/thunderid/internal/flow/session"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/internal/user"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

const handlerLoggerComponentName = "SessionMgtHandler"

// sessionMgtHandler serves the session management endpoints.
type sessionMgtHandler struct {
	svc    flowsession.Service
	users  user.UserServiceInterface
	apps   application.ApplicationServiceInterface
	logger *log.Logger
}

// newSessionMgtHandler creates the session management handler.
func newSessionMgtHandler(svc flowsession.Service, users user.UserServiceInterface,
	apps application.ApplicationServiceInterface) *sessionMgtHandler {
	return &sessionMgtHandler{
		svc:    svc,
		users:  users,
		apps:   apps,
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName)),
	}
}

// HandleSessionListRequest handles GET /sessions?userId= or GET /sessions?appId=.
func (h *sessionMgtHandler) HandleSessionListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()
	userID := strings.TrimSpace(query.Get("userId"))
	appID := strings.TrimSpace(query.Get("appId"))
	if (userID == "") == (appID == "") {
		handleError(ctx, w, &ErrorInvalidListFilter)
		return
	}
	limit, offset, svcErr := parsePaginationParams(query)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	var (
		page       *flowsession.SessionPage
		err        error
		extraQuery string
	)
	now := time.Now().UTC()
	if userID != "" {
		page, err = h.svc.ListLiveBySubject(ctx, userID, now, limit, offset)
		extraQuery = "&userId=" + url.QueryEscape(userID)
	} else {
		page, err = h.svc.ListLiveByApp(ctx, appID, now, limit, offset)
		extraQuery = "&appId=" + url.QueryEscape(appID)
	}
	if err != nil {
		h.logger.Error(ctx, "Failed to list sessions", log.Error(err))
		handleError(ctx, w, &tidcommon.InternalServerError)
		return
	}

	userNames := h.userNames(ctx, page.Sessions)
	appNames := h.appNames(ctx, page.Sessions)
	sessions := make([]sessionResponse, 0, len(page.Sessions))
	for _, s := range page.Sessions {
		sessions = append(sessions, toSessionResponse(s, userNames, appNames))
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK,
		buildListResponse(sessions, page.TotalResults, "/sessions", limit, offset, extraQuery))
}

// HandleSelfSessionListRequest handles GET /users/me/sessions: the caller's own sessions, with the one
// behind the caller's access token marked as current.
func (h *sessionMgtHandler) HandleSelfSessionListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	subject := security.GetSubject(ctx)
	if strings.TrimSpace(subject) == "" {
		sysutils.WriteErrorResponse(ctx, w, http.StatusUnauthorized, apierror.ErrUnauthorized)
		return
	}
	limit, offset, svcErr := parsePaginationParams(r.URL.Query())
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	page, err := h.svc.ListLiveBySubject(ctx, subject, time.Now().UTC(), limit, offset)
	if err != nil {
		h.logger.Error(ctx, "Failed to list own sessions", log.Error(err))
		handleError(ctx, w, &tidcommon.InternalServerError)
		return
	}

	// The caller already knows who they are, so only the application names are resolved.
	appNames := h.appNames(ctx, page.Sessions)
	tokenFamilyID, _ := security.GetAttribute(ctx, oauth2const.ClaimTokenFamilyID).(string)
	sessions := make([]sessionResponse, 0, len(page.Sessions))
	for _, s := range page.Sessions {
		resp := toSessionResponse(s, nil, appNames)
		isCurrent := isCurrentSession(s, tokenFamilyID)
		resp.IsCurrent = &isCurrent
		sessions = append(sessions, resp)
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK,
		buildListResponse(sessions, page.TotalResults, "/users/me/sessions", limit, offset, ""))
}

// userNames resolves the display name of each distinct user across the sessions, once per user. A
// user that can't be found gets no name.
func (h *sessionMgtHandler) userNames(ctx context.Context, sessions []flowsession.SessionDetail) map[string]string {
	names := make(map[string]string)
	for _, s := range sessions {
		if _, seen := names[s.SubjectID]; seen {
			continue
		}
		names[s.SubjectID] = ""
		if u, svcErr := h.users.GetUser(ctx, s.SubjectID, true); svcErr == nil && u != nil {
			names[s.SubjectID] = u.Display
		}
	}
	return names
}

// appNames resolves the name of each distinct participating application across the sessions, once
// per application. An application that can't be found gets no name.
func (h *sessionMgtHandler) appNames(ctx context.Context, sessions []flowsession.SessionDetail) map[string]string {
	names := make(map[string]string)
	for _, s := range sessions {
		for _, p := range s.Participants {
			if _, seen := names[p.AppID]; seen {
				continue
			}
			names[p.AppID] = ""
			if a, svcErr := h.apps.GetApplication(ctx, p.AppID); svcErr == nil && a != nil {
				names[p.AppID] = a.Name
			}
		}
	}
	return names
}

// isCurrentSession reports whether the session issued the caller's access token: one of its
// participants holds the token's family. A token without a family matches no session.
func isCurrentSession(s flowsession.SessionDetail, tokenFamilyID string) bool {
	if tokenFamilyID == "" {
		return false
	}
	for _, p := range s.Participants {
		if p.TokenFamilyID == tokenFamilyID {
			return true
		}
	}
	return false
}

// buildListResponse assembles the paginated list body. basePath is the route the pagination links
// point back to, and extraQuery carries the list filter into them.
func buildListResponse(sessions []sessionResponse, total int, basePath string, limit, offset int,
	extraQuery string) *sessionListResponse {
	return &sessionListResponse{
		TotalResults: total,
		StartIndex:   offset + 1,
		Count:        len(sessions),
		Sessions:     sessions,
		Links:        sysutils.BuildPaginationLinks(basePath, limit, offset, total, extraQuery),
	}
}

// parsePaginationParams parses limit and offset. An omitted limit is the default page size; a limit
// outside 1 to the maximum page size, or a negative offset, is rejected.
func parsePaginationParams(query url.Values) (int, int, *tidcommon.ServiceError) {
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
	if svcErr.Type == tidcommon.ClientErrorType {
		statusCode = http.StatusBadRequest
	}

	sysutils.WriteErrorResponse(ctx, w, statusCode, apierror.ErrorResponse{
		Code:        svcErr.Code,
		Message:     svcErr.Error,
		Description: svcErr.ErrorDescription,
	})
}
