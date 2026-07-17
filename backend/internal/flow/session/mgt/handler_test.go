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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	flowsession "github.com/thunder-id/thunderid/internal/flow/session"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/tests/mocks/flow/sessionmock"
)

// stubNameResolver is a test NameResolver returning names from its maps, or "" (falls back to id)
// when an id is absent.
type stubNameResolver struct {
	users map[string]string
	apps  map[string]string
}

func (s stubNameResolver) UserName(_ context.Context, id string) string { return s.users[id] }
func (s stubNameResolver) AppName(_ context.Context, id string) string  { return s.apps[id] }

// TestHandleSessionListRequest_Success verifies GET /sessions?userId= returns 200 with a
// paginated payload whose sessions include their participants, and that the session handle
// (the cookie credential) is never present in the response body.
func TestHandleSessionListRequest_Success(t *testing.T) {
	mockSvc := sessionmock.NewManagementServiceMock(t)
	now := time.Now().UTC()
	sess := flowsession.Session{
		SessionID:       "sess-1",
		SubjectID:       "u1",
		FlowID:          "flow-1",
		HandleID:        "secret-handle",
		AuthenticatedAt: now,
		CreatedAt:       now,
		LastActiveAt:    now,
	}
	participant := flowsession.Participant{
		SessionID:     "sess-1",
		AppID:         "app-1",
		FirstJoinedAt: now,
		LastActiveAt:  now,
	}
	mockSvc.EXPECT().
		ListBySubject(mock.Anything, "u1", 30, 0, mock.Anything).
		Return(&flowsession.SessionPage{Sessions: []flowsession.Session{sess}, TotalResults: 1}, nil)
	mockSvc.EXPECT().
		ListParticipants(mock.Anything, "sess-1").
		Return([]flowsession.Participant{participant}, nil)

	handler := newSessionMgtHandler(mockSvc, stubNameResolver{})
	req := httptest.NewRequest(http.MethodGet, "/sessions?userId=u1", nil)
	rr := httptest.NewRecorder()

	handler.HandleSessionListRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()
	require.Contains(t, body, `"totalResults":1`)
	require.Contains(t, body, `"startIndex":1`)
	require.Contains(t, body, `"count":1`)
	require.Contains(t, body, `"sessions"`)
	require.Contains(t, body, `"links"`)
	require.Contains(t, body, `"appId":"app-1"`)
	require.NotContains(t, body, "secret-handle")
}

// TestHandleSessionListRequest_ResolvesNames verifies the listing embeds the server-resolved user
// and application display names, and omits the name (client falls back to the id) when unresolved.
func TestHandleSessionListRequest_ResolvesNames(t *testing.T) {
	mockSvc := sessionmock.NewManagementServiceMock(t)
	now := time.Now().UTC()
	sess := flowsession.Session{SessionID: "sess-1", SubjectID: "u1", FlowID: "flow-1",
		HandleID: "secret-handle", AuthenticatedAt: now, CreatedAt: now, LastActiveAt: now}
	mockSvc.EXPECT().
		ListBySubject(mock.Anything, "u1", 30, 0, mock.Anything).
		Return(&flowsession.SessionPage{Sessions: []flowsession.Session{sess}, TotalResults: 1}, nil)
	mockSvc.EXPECT().
		ListParticipants(mock.Anything, "sess-1").
		Return([]flowsession.Participant{
			{SessionID: "sess-1", AppID: "app-1", FirstJoinedAt: now, LastActiveAt: now},
			{SessionID: "sess-1", AppID: "app-2", FirstJoinedAt: now, LastActiveAt: now},
		}, nil)

	names := stubNameResolver{
		users: map[string]string{"u1": "Alice Doe"},
		apps:  map[string]string{"app-1": "My App"}, // app-2 intentionally absent
	}
	handler := newSessionMgtHandler(mockSvc, names)
	req := httptest.NewRequest(http.MethodGet, "/sessions?userId=u1", nil)
	rr := httptest.NewRecorder()

	handler.HandleSessionListRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()
	require.Contains(t, body, `"userName":"Alice Doe"`)
	require.Contains(t, body, `"appName":"My App"`)
	// app-2 has no resolved name, so appName is omitted (client falls back to appId).
	require.Contains(t, body, `"appId":"app-2"`)
	require.NotContains(t, body, `"appName":"app-2"`)
}

// TestHandleSessionListRequest_EmptyPage verifies that an empty page serializes "sessions" as
// an empty JSON array, never null, since buildListResponse always allocates the slice.
func TestHandleSessionListRequest_EmptyPage(t *testing.T) {
	mockSvc := sessionmock.NewManagementServiceMock(t)
	mockSvc.EXPECT().
		ListBySubject(mock.Anything, "u1", 30, 0, mock.Anything).
		Return(&flowsession.SessionPage{Sessions: nil, TotalResults: 0}, nil)

	handler := newSessionMgtHandler(mockSvc, stubNameResolver{})
	req := httptest.NewRequest(http.MethodGet, "/sessions?userId=u1", nil)
	rr := httptest.NewRecorder()

	handler.HandleSessionListRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Contains(t, rr.Body.String(), `"sessions":[]`)
}

// TestHandleSessionListRequest_BothFilters verifies that supplying both userId and appId is
// rejected with SSM-1002 before the service is ever consulted.
func TestHandleSessionListRequest_BothFilters(t *testing.T) {
	mockSvc := sessionmock.NewManagementServiceMock(t)
	handler := newSessionMgtHandler(mockSvc, stubNameResolver{})
	req := httptest.NewRequest(http.MethodGet, "/sessions?userId=u1&appId=a1", nil)
	rr := httptest.NewRecorder()

	handler.HandleSessionListRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.Contains(t, rr.Body.String(), ErrorInvalidListFilter.Code)
}

// TestHandleSessionListRequest_NoFilter verifies that supplying neither userId nor appId is
// rejected with SSM-1002 before the service is ever consulted.
func TestHandleSessionListRequest_NoFilter(t *testing.T) {
	mockSvc := sessionmock.NewManagementServiceMock(t)
	handler := newSessionMgtHandler(mockSvc, stubNameResolver{})
	req := httptest.NewRequest(http.MethodGet, "/sessions", nil)
	rr := httptest.NewRecorder()

	handler.HandleSessionListRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.Contains(t, rr.Body.String(), ErrorInvalidListFilter.Code)
}

// TestHandleSessionListRequest_InvalidLimit verifies that a non-integer limit is rejected with
// SSM-1001.
func TestHandleSessionListRequest_InvalidLimit(t *testing.T) {
	mockSvc := sessionmock.NewManagementServiceMock(t)
	handler := newSessionMgtHandler(mockSvc, stubNameResolver{})
	req := httptest.NewRequest(http.MethodGet, "/sessions?userId=u1&limit=abc", nil)
	rr := httptest.NewRecorder()

	handler.HandleSessionListRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.Contains(t, rr.Body.String(), ErrorInvalidPaginationParams.Code)
}

// TestHandleSelfSessionListRequest_NoSubject verifies that GET /sessions/me without an
// authenticated subject in the security context is rejected with SSM-1003.
func TestHandleSelfSessionListRequest_NoSubject(t *testing.T) {
	mockSvc := sessionmock.NewManagementServiceMock(t)
	handler := newSessionMgtHandler(mockSvc, stubNameResolver{})
	req := httptest.NewRequest(http.MethodGet, "/sessions/me", nil)
	rr := httptest.NewRecorder()

	handler.HandleSelfSessionListRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.Contains(t, rr.Body.String(), ErrorAuthenticationRequired.Code)
}

// TestHandleSessionListRequest_PaginationLinks verifies that admin-listing pagination links
// beyond one page point at /sessions and carry the escaped userId filter.
func TestHandleSessionListRequest_PaginationLinks(t *testing.T) {
	mockSvc := sessionmock.NewManagementServiceMock(t)
	mockSvc.EXPECT().
		ListBySubject(mock.Anything, "u 1", 2, 2, mock.Anything).
		Return(&flowsession.SessionPage{Sessions: nil, TotalResults: 5}, nil)

	handler := newSessionMgtHandler(mockSvc, stubNameResolver{})
	req := httptest.NewRequest(http.MethodGet, "/sessions?userId=u+1&limit=2&offset=2", nil)
	rr := httptest.NewRecorder()

	handler.HandleSessionListRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp struct {
		Links []struct {
			Href string `json:"href"`
			Rel  string `json:"rel"`
		} `json:"links"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.Links)
	for _, link := range resp.Links {
		require.True(t, strings.HasPrefix(link.Href, "/sessions?"),
			"link %q (%s) must start with /sessions?", link.Href, link.Rel)
		require.Contains(t, link.Href, "userId=u+1")
	}
}

// TestHandleSelfSessionListRequest_PaginationLinks verifies that self-listing pagination links
// beyond one page point at /sessions/me and never leak a userId filter.
func TestHandleSelfSessionListRequest_PaginationLinks(t *testing.T) {
	mockSvc := sessionmock.NewManagementServiceMock(t)
	mockSvc.EXPECT().
		ListBySubject(mock.Anything, "user1", 2, 2, mock.Anything).
		Return(&flowsession.SessionPage{Sessions: nil, TotalResults: 5}, nil)

	authCtx := security.NewSecurityContextForTest("user1", "", "", nil, nil)
	handler := newSessionMgtHandler(mockSvc, stubNameResolver{})
	req := httptest.NewRequest(http.MethodGet, "/sessions/me?limit=2&offset=2", nil)
	req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
	rr := httptest.NewRecorder()

	handler.HandleSelfSessionListRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp struct {
		Links []struct {
			Href string `json:"href"`
			Rel  string `json:"rel"`
		} `json:"links"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.Links)
	for _, link := range resp.Links {
		require.True(t, strings.HasPrefix(link.Href, "/sessions/me?"),
			"link %q (%s) must start with /sessions/me?", link.Href, link.Rel)
		require.NotContains(t, link.Href, "userId=")
	}
}

// TestHandleSelfSessionListRequest_Success verifies that GET /sessions/me with an authenticated
// subject delegates to ListBySubject with that subject and returns 200.
func TestHandleSelfSessionListRequest_Success(t *testing.T) {
	mockSvc := sessionmock.NewManagementServiceMock(t)
	mockSvc.EXPECT().
		ListBySubject(mock.Anything, "user1", 30, 0, mock.Anything).
		Return(&flowsession.SessionPage{Sessions: nil, TotalResults: 0}, nil)

	authCtx := security.NewSecurityContextForTest("user1", "", "", nil, nil)
	handler := newSessionMgtHandler(mockSvc, stubNameResolver{})
	req := httptest.NewRequest(http.MethodGet, "/sessions/me", nil)
	req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
	rr := httptest.NewRecorder()

	handler.HandleSelfSessionListRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
}
