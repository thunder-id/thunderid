// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sessionmgt

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	flowsession "github.com/thunder-id/thunderid/internal/flow/session"
	"github.com/thunder-id/thunderid/internal/system/security"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/applicationmock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/sessionmock"
	"github.com/thunder-id/thunderid/tests/mocks/usermock"
)

type HandlerTestSuite struct {
	suite.Suite
	svc     *sessionmock.ServiceMock
	users   *usermock.UserServiceInterfaceMock
	apps    *applicationmock.ApplicationServiceInterfaceMock
	handler *sessionMgtHandler
	mux     *http.ServeMux
}

func TestHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(HandlerTestSuite))
}

func (s *HandlerTestSuite) SetupTest() {
	s.svc = sessionmock.NewServiceMock(s.T())
	s.users = usermock.NewUserServiceInterfaceMock(s.T())
	s.apps = applicationmock.NewApplicationServiceInterfaceMock(s.T())
	s.handler = newSessionMgtHandler(s.svc, s.users, s.apps)
	s.mux = http.NewServeMux()
	registerRoutes(s.mux, s.handler)
}

func sampleDetail(id string, participants ...flowsession.Participant) flowsession.SessionDetail {
	base := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	return flowsession.SessionDetail{
		Session: flowsession.Session{
			SessionID:         id,
			SubjectID:         "user-1",
			FlowID:            "flow-1",
			HandleID:          "handle-secret",
			AuthenticatedAt:   base,
			CreatedAt:         base,
			LastActiveAt:      base.Add(time.Minute),
			IdleExpiresAt:     base.Add(31 * time.Minute),
			AbsoluteExpiresAt: base.Add(8 * time.Hour),
			Properties: flowsession.SessionProperties{
				UserAgent:    "Mozilla/5.0",
				CreationIP:   "198.51.100.7",
				LastActiveIP: "203.0.113.10",
			},
			State: flowsession.StateActive,
		},
		Participants: participants,
	}
}

func (s *HandlerTestSuite) expectUser(id, display string) {
	s.users.EXPECT().GetUser(mock.Anything, id, true).Return(&providers.User{ID: id, Display: display}, nil).Once()
}

func (s *HandlerTestSuite) expectApp(name string) {
	s.apps.EXPECT().GetApplication(mock.Anything, "app-a").Return(&providers.Application{Name: name}, nil).Once()
}

func selfContext(attributes map[string]interface{}) context.Context {
	return security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest("user-1", "", "", nil, attributes))
}

func (s *HandlerTestSuite) serve(r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, r)
	return w
}

func decode[T any](s *HandlerTestSuite, w *httptest.ResponseRecorder) T {
	var out T
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &out))
	return out
}

func (s *HandlerTestSuite) TestList_ByUser() {
	detail := sampleDetail("sess-1",
		flowsession.Participant{SessionID: "sess-1", AppID: "app-a", TokenFamilyID: "tfid-secret"},
		flowsession.Participant{SessionID: "sess-1", AppID: "app-b"})
	s.svc.EXPECT().ListLiveBySubject(mock.Anything, "user-1", mock.AnythingOfType("time.Time"), 30, 0).
		Return(&flowsession.SessionPage{Sessions: []flowsession.SessionDetail{detail}, TotalResults: 1}, nil)
	s.expectUser("user-1", "Alice")
	s.expectApp("App A")
	s.apps.EXPECT().GetApplication(mock.Anything, "app-b").Return(nil, &tidcommon.InternalServerError).Once()

	w := s.serve(httptest.NewRequest(http.MethodGet, "/sessions?userId=user-1", nil))

	s.Equal(http.StatusOK, w.Code)
	s.NotContains(w.Body.String(), "handle-secret", "the handle is a credential and never returned")
	s.NotContains(w.Body.String(), "tfid-secret", "token family ids are never returned")
	body := decode[sessionListResponse](s, w)
	s.Equal(1, body.TotalResults)
	s.Equal(1, body.StartIndex)
	s.Equal(1, body.Count)
	s.Require().Len(body.Sessions, 1)
	got := body.Sessions[0]
	s.Equal("sess-1", got.ID)
	s.Equal("user-1", got.UserID)
	s.Equal("Alice", got.UserName)
	s.Equal("Mozilla/5.0", got.UserAgent)
	s.Equal("198.51.100.7", got.CreationIP)
	s.Equal("203.0.113.10", got.LastActiveIP)
	s.Equal("2026-06-16T10:01:00Z", got.LastActiveAt)
	s.Equal("2026-06-16T18:00:00Z", got.AbsoluteExpiresAt)
	s.Nil(got.IsCurrent, "admin listings don't mark a current session")
	s.Equal([]participantResponse{
		{AppID: "app-a", AppName: "App A", FirstJoinedAt: "", LastActiveAt: ""},
		{AppID: "app-b"},
	}, got.Participants)
}

func (s *HandlerTestSuite) TestList_ByAppResolvesEachNameOnce() {
	page := &flowsession.SessionPage{
		Sessions: []flowsession.SessionDetail{
			sampleDetail("sess-1", flowsession.Participant{AppID: "app-a"}),
			sampleDetail("sess-2", flowsession.Participant{AppID: "app-a"}),
		},
		TotalResults: 45,
	}
	s.svc.EXPECT().ListLiveByApp(mock.Anything, "app-a", mock.AnythingOfType("time.Time"), 20, 20).Return(page, nil)
	s.users.EXPECT().GetUser(mock.Anything, "user-1", true).Return(nil, &tidcommon.ErrorUnauthorized).Once()
	s.expectApp("App A")

	w := s.serve(httptest.NewRequest(http.MethodGet, "/sessions?appId=app-a&limit=20&offset=20", nil))

	s.Equal(http.StatusOK, w.Code)
	body := decode[sessionListResponse](s, w)
	s.Equal(45, body.TotalResults)
	s.Equal(21, body.StartIndex)
	s.Equal(2, body.Count)
	s.Contains(body.Links, sysutils.Link{Href: "/sessions?offset=40&limit=20&appId=app-a", Rel: "next"})
	s.Empty(body.Sessions[0].UserName, "a user the caller can't read gets no name")
}

func (s *HandlerTestSuite) TestList_InvalidRequests() {
	cases := []struct {
		name     string
		query    string
		wantCode string
	}{
		{"no filter", "", ErrorInvalidListFilter.Code},
		{"blank filter", "?userId=%20", ErrorInvalidListFilter.Code},
		{"both filters", "?userId=u&appId=a", ErrorInvalidListFilter.Code},
		{"limit not a number", "?userId=u&limit=x", ErrorInvalidLimit.Code},
		{"limit zero", "?userId=u&limit=0", ErrorInvalidLimit.Code},
		{"limit over max", "?userId=u&limit=101", ErrorInvalidLimit.Code},
		{"negative offset", "?userId=u&offset=-1", ErrorInvalidOffset.Code},
		{"offset not a number", "?userId=u&offset=x", ErrorInvalidOffset.Code},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			w := s.serve(httptest.NewRequest(http.MethodGet, "/sessions"+tc.query, nil))

			s.Equal(http.StatusBadRequest, w.Code)
			s.Contains(w.Body.String(), tc.wantCode)
		})
	}
}

func (s *HandlerTestSuite) TestList_ServiceError() {
	s.svc.EXPECT().ListLiveBySubject(mock.Anything, "user-1", mock.Anything, 30, 0).Return(nil, errors.New("db down"))

	w := s.serve(httptest.NewRequest(http.MethodGet, "/sessions?userId=user-1", nil))

	s.Equal(http.StatusInternalServerError, w.Code)
	s.NotContains(w.Body.String(), "db down")
}

func (s *HandlerTestSuite) TestSelfList_MarksCurrentSession() {
	page := &flowsession.SessionPage{
		Sessions: []flowsession.SessionDetail{
			sampleDetail("sess-1", flowsession.Participant{AppID: "app-a", TokenFamilyID: "tfid-other"}),
			sampleDetail("sess-2", flowsession.Participant{AppID: "app-a", TokenFamilyID: "tfid-mine"}),
		},
		TotalResults: 2,
	}
	s.svc.EXPECT().ListLiveBySubject(mock.Anything, "user-1", mock.Anything, 30, 0).Return(page, nil)
	s.expectApp("App A")

	ctx := selfContext(map[string]interface{}{"tfid": "tfid-mine"})
	w := s.serve(httptest.NewRequest(http.MethodGet, "/users/me/sessions?userId=someone-else", nil).WithContext(ctx))

	s.Equal(http.StatusOK, w.Code)
	body := decode[sessionListResponse](s, w)
	s.Require().Len(body.Sessions, 2)
	s.Require().NotNil(body.Sessions[0].IsCurrent)
	s.False(*body.Sessions[0].IsCurrent)
	s.Require().NotNil(body.Sessions[1].IsCurrent)
	s.True(*body.Sessions[1].IsCurrent)
	s.Empty(body.Sessions[1].UserName, "own sessions don't resolve the user's name")
	s.Empty(body.Links)
}

func (s *HandlerTestSuite) TestSelfList_TokenWithoutFamilyMarksNothing() {
	page := &flowsession.SessionPage{
		Sessions: []flowsession.SessionDetail{
			sampleDetail("sess-1", flowsession.Participant{AppID: "app-a"}),
		},
		TotalResults: 1,
	}
	s.svc.EXPECT().ListLiveBySubject(mock.Anything, "user-1", mock.Anything, 30, 0).Return(page, nil)
	s.expectApp("")

	ctx := selfContext(nil)
	w := s.serve(httptest.NewRequest(http.MethodGet, "/users/me/sessions", nil).WithContext(ctx))

	s.Equal(http.StatusOK, w.Code)
	body := decode[sessionListResponse](s, w)
	s.Require().Len(body.Sessions, 1)
	s.Require().NotNil(body.Sessions[0].IsCurrent)
	s.False(*body.Sessions[0].IsCurrent, "an empty participant tfid must not match a token without one")
}

func (s *HandlerTestSuite) TestSelfList_Errors() {
	s.Run("no subject", func() {
		w := s.serve(httptest.NewRequest(http.MethodGet, "/users/me/sessions", nil))
		s.Equal(http.StatusUnauthorized, w.Code)
	})
	s.Run("invalid limit", func() {
		w := s.serve(httptest.NewRequest(http.MethodGet, "/users/me/sessions?limit=0", nil).
			WithContext(selfContext(nil)))
		s.Equal(http.StatusBadRequest, w.Code)
	})
	s.Run("service error", func() {
		s.svc.EXPECT().ListLiveBySubject(mock.Anything, "user-1", mock.Anything, 30, 0).
			Return(nil, errors.New("db down")).Once()
		w := s.serve(httptest.NewRequest(http.MethodGet, "/users/me/sessions", nil).
			WithContext(selfContext(nil)))
		s.Equal(http.StatusInternalServerError, w.Code)
	})
}

func (s *HandlerTestSuite) TestPreflight() {
	for _, path := range []string{"/sessions", "/users/me/sessions"} {
		w := s.serve(httptest.NewRequest(http.MethodOptions, path, nil))
		s.Equal(http.StatusNoContent, w.Code, path)
	}
}

func (s *HandlerTestSuite) TestFormatTime_ZeroIsEmpty() {
	s.Empty(formatTime(time.Time{}))
	s.Equal("2026-06-16T10:00:00Z",
		formatTime(time.Date(2026, 6, 16, 15, 30, 0, 0, time.FixedZone("IST", 5*3600+1800))))
}
