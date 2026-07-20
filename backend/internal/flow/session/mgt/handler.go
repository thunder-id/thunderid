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

package mgt

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	flowsession "github.com/thunder-id/thunderid/internal/flow/session"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

const handlerLoggerComponentName = "SessionMgtHandler"

// NameResolver resolves the display names shown in the session listing. Implementations resolve
// with the server's own privileges, so names appear regardless of the caller's list permissions.
// A lookup that fails or finds nothing returns "", and the client falls back to the id.
type NameResolver interface {
	// UserName returns the display name for the given user (subject) id, or "" if unresolved.
	UserName(ctx context.Context, userID string) string
	// AppName returns the name for the given application id, or "" if unresolved.
	AppName(ctx context.Context, appID string) string
}

// sessionMgtHandler serves the read-only session listing endpoints.
type sessionMgtHandler struct {
	svc    flowsession.ManagementService
	names  NameResolver
	logger *log.Logger
}

// newSessionMgtHandler creates the session management handler.
func newSessionMgtHandler(svc flowsession.ManagementService, names NameResolver) *sessionMgtHandler {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))
	return &sessionMgtHandler{svc: svc, names: names, logger: logger}
}

// parsePaginationParams parses limit and offset, applying the default page size when limit is
// omitted and clamping to MaxPageSize so a caller cannot request an unbounded page (which would
// materialize every row plus one participant lookup each).
func parsePaginationParams(query url.Values) (int, int, *tidcommon.ServiceError) {
	limit := serverconst.DefaultPageSize
	offset := 0

	if limitStr := query.Get("limit"); limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err != nil || parsedLimit < 0 {
			return 0, 0, &ErrorInvalidPaginationParams
		}
		limit = parsedLimit
		if limit > serverconst.MaxPageSize {
			limit = serverconst.MaxPageSize
		}
	}
	if offsetStr := query.Get("offset"); offsetStr != "" {
		parsedOffset, err := strconv.Atoi(offsetStr)
		if err != nil || parsedOffset < 0 {
			return 0, 0, &ErrorInvalidPaginationParams
		}
		offset = parsedOffset
	}
	return limit, offset, nil
}

// HandleSessionListRequest handles GET /sessions?userId=|appId= (admin listing).
func (h *sessionMgtHandler) HandleSessionListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := strings.TrimSpace(r.URL.Query().Get("userId"))
	appID := strings.TrimSpace(r.URL.Query().Get("appId"))

	if (userID == "") == (appID == "") { // both or neither
		handleError(ctx, w, &ErrorInvalidListFilter)
		return
	}
	limit, offset, svcErr := parsePaginationParams(r.URL.Query())
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	now := time.Now().UTC()
	var (
		page     *flowsession.SessionPage
		err      error
		extraQry string
	)
	if userID != "" {
		page, err = h.svc.ListBySubject(ctx, userID, limit, offset, now)
		extraQry = "&userId=" + url.QueryEscape(userID)
	} else {
		page, err = h.svc.ListByApp(ctx, appID, limit, offset, now)
		extraQry = "&appId=" + url.QueryEscape(appID)
	}
	if err != nil {
		h.logger.Error(ctx, "Failed to list sessions", log.Error(err))
		handleError(ctx, w, &ErrorInternalServerError)
		return
	}

	resp, svcErr := h.buildListResponse(ctx, page, "/sessions", limit, offset, extraQry)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, resp)
}

// HandleSelfSessionListRequest handles GET /sessions/me (own sessions).
func (h *sessionMgtHandler) HandleSelfSessionListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	subject := security.GetSubject(ctx)
	if strings.TrimSpace(subject) == "" {
		handleError(ctx, w, &ErrorAuthenticationRequired)
		return
	}
	limit, offset, svcErr := parsePaginationParams(r.URL.Query())
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	page, err := h.svc.ListBySubject(ctx, subject, limit, offset, time.Now().UTC())
	if err != nil {
		h.logger.Error(ctx, "Failed to list own sessions", log.Error(err))
		handleError(ctx, w, &ErrorInternalServerError)
		return
	}

	resp, svcErr := h.buildListResponse(ctx, page, "/sessions/me", limit, offset, "")
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, resp)
}

// buildListResponse loads each session's participants, resolves display names, and assembles the
// paginated payload. basePath is the route the pagination links point back at (/sessions or
// /sessions/me). Names are resolved once per distinct id across the page (not per row) and
// server-side, so they appear regardless of the caller's user/application list permissions.
func (h *sessionMgtHandler) buildListResponse(ctx context.Context, page *flowsession.SessionPage,
	basePath string, limit, offset int, extraQuery string) (*sessionListResponse, *tidcommon.ServiceError) {
	partsBySession := make(map[string][]flowsession.Participant, len(page.Sessions))
	userNames := make(map[string]string)
	appNames := make(map[string]string)

	for _, s := range page.Sessions {
		parts, err := h.svc.ListParticipants(ctx, s.SessionID)
		if err != nil {
			h.logger.Error(ctx, "Failed to list session participants", log.Error(err))
			return nil, &ErrorInternalServerError
		}
		partsBySession[s.SessionID] = parts

		if _, seen := userNames[s.SubjectID]; !seen {
			userNames[s.SubjectID] = h.names.UserName(ctx, s.SubjectID)
		}
		for _, p := range parts {
			if _, seen := appNames[p.AppID]; !seen {
				appNames[p.AppID] = h.names.AppName(ctx, p.AppID)
			}
		}
	}

	sessions := make([]sessionResponse, 0, len(page.Sessions))
	for _, s := range page.Sessions {
		sessions = append(sessions, toSessionResponse(s, partsBySession[s.SessionID], userNames[s.SubjectID], appNames))
	}
	return &sessionListResponse{
		TotalResults: page.TotalResults,
		StartIndex:   offset + 1,
		Count:        len(sessions),
		Sessions:     sessions,
		Links:        sysutils.BuildPaginationLinks(basePath, limit, offset, page.TotalResults, extraQuery),
	}, nil
}

// handleError handles service errors and returns appropriate HTTP responses. Copied from
// backend/internal/design/theme/mgt/handler.go, dropping the not-found special case (this
// package only produces client and server errors).
func handleError(ctx context.Context, w http.ResponseWriter, svcErr *tidcommon.ServiceError) {
	statusCode := http.StatusInternalServerError
	if svcErr.Type == tidcommon.ClientErrorType {
		statusCode = http.StatusBadRequest
	}

	errResp := apierror.ErrorResponse{
		Code:        svcErr.Code,
		Message:     svcErr.Error,
		Description: svcErr.ErrorDescription,
	}

	sysutils.WriteErrorResponse(ctx, w, statusCode, errResp)
}
