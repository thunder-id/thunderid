// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

type HandlerTestSuite struct {
	suite.Suite
	store *sessionStoreMock
	mux   *http.ServeMux
}

func TestHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(HandlerTestSuite))
}

func (s *HandlerTestSuite) SetupSuite() {
	security.InitDataPlanePermissions("")
}

func (s *HandlerTestSuite) SetupTest() {
	s.store = newSessionStoreMock(s.T())
	s.mux = http.NewServeMux()
	registerRoutes(s.mux, newSessionMgtHandler(&service{store: s.store, timeouts: defaultTimeouts(s.T()),
		logger: log.GetLogger()}))
}

func (s *HandlerTestSuite) serve(r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, r)
	return w
}

func userRequest(target string) *http.Request {
	ctx := security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest("user-1", "", "", nil, nil))
	return httptest.NewRequest(http.MethodGet, target, nil).WithContext(ctx)
}

// adminRequest is a request from an administrator holding the system permission.
func adminRequest(target string) *http.Request {
	return httptest.NewRequest(http.MethodGet, target, nil).WithContext(adminContext("", "system"))
}

// selfRequest is a request from user-1 whose token holds the token family tfid-1.
func selfRequest(target string) *http.Request {
	return httptest.NewRequest(http.MethodGet, target, nil).WithContext(selfContext("tfid-1"))
}

func (s *HandlerTestSuite) TestList_PassesTheFilterAndPage() {
	s.store.EXPECT().CountLiveByApp(mock.Anything, "app-a", noOU, mock.Anything).Return(45, nil)
	s.store.EXPECT().ListLiveByApp(mock.Anything, "app-a", noOU, mock.Anything, 20, 20).Return([]Session{}, nil)
	s.store.EXPECT().ListBySessionIDs(mock.Anything, []string{}).Return(nil, nil)

	w := s.serve(adminRequest("/sessions?appId=app-a&limit=20&offset=20"))

	s.Equal(http.StatusOK, w.Code)
	var body sessionListResponse
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
	s.Equal(45, body.TotalResults)
	s.Equal(21, body.StartIndex)
}

func (s *HandlerTestSuite) TestList_DefaultPage() {
	s.store.EXPECT().CountLiveBySubject(mock.Anything, "user-1", noOU, mock.Anything).Return(1, nil)
	s.store.EXPECT().ListLiveBySubject(mock.Anything, "user-1", noOU, mock.Anything, 30, 0).Return([]Session{}, nil)
	s.store.EXPECT().ListBySessionIDs(mock.Anything, []string{}).Return(nil, nil)

	w := s.serve(adminRequest("/sessions?userId=user-1"))

	s.Equal(http.StatusOK, w.Code)
}

func (s *HandlerTestSuite) TestList_InvalidPagination() {
	cases := []struct {
		name     string
		query    string
		wantCode string
	}{
		{"limit not a number", "?userId=u&limit=x", ErrorInvalidLimit.Code},
		{"limit zero", "?userId=u&limit=0", ErrorInvalidLimit.Code},
		{"limit over max", "?userId=u&limit=101", ErrorInvalidLimit.Code},
		{"negative offset", "?userId=u&offset=-1", ErrorInvalidOffset.Code},
		{"offset not a number", "?userId=u&offset=x", ErrorInvalidOffset.Code},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			w := s.serve(userRequest("/sessions" + tc.query))

			s.Equal(http.StatusBadRequest, w.Code)
			s.Contains(w.Body.String(), tc.wantCode)
		})
	}
}

func (s *HandlerTestSuite) TestList_ServiceErrors() {
	s.Run("client error", func() {
		w := s.serve(adminRequest("/sessions?userId=u&appId=a"))
		s.Equal(http.StatusBadRequest, w.Code)
		s.Contains(w.Body.String(), ErrorInvalidListFilter.Code)
	})
	s.Run("unauthorized", func() {
		w := s.serve(userRequest("/sessions?userId=u"))
		s.Equal(http.StatusForbidden, w.Code)
		s.Contains(w.Body.String(), tidcommon.ErrorUnauthorized.Code)
	})
	s.Run("server error", func() {
		s.store.EXPECT().CountLiveBySubject(mock.Anything, "u", noOU, mock.Anything).
			Return(0, errors.New("db down")).Once()
		w := s.serve(adminRequest("/sessions?userId=u"))
		s.Equal(http.StatusInternalServerError, w.Code)
		s.Contains(w.Body.String(), tidcommon.InternalServerError.Code)
	})
}

func (s *HandlerTestSuite) TestSelfList() {
	s.store.EXPECT().GetSubjectParticipantByTokenFamily(mock.Anything, "user-1", "tfid-1").
		Return(&Participant{OUID: "ou-1"}, nil)
	s.store.EXPECT().CountLiveBySubject(mock.Anything, "user-1", new("ou-1"), mock.Anything).Return(1, nil)
	s.store.EXPECT().ListLiveBySubject(mock.Anything, "user-1", new("ou-1"), mock.Anything, 10, 0).
		Return([]Session{{SessionID: "sess-1"}}, nil)
	s.store.EXPECT().ListBySessionIDs(mock.Anything, []string{"sess-1"}).Return(nil, nil)

	w := s.serve(selfRequest("/users/me/sessions?limit=10"))

	s.Equal(http.StatusOK, w.Code)
	s.Contains(w.Body.String(), `"id":"sess-1"`)
}

func (s *HandlerTestSuite) TestSelfList_Errors() {
	s.Run("no subject", func() {
		w := s.serve(httptest.NewRequest(http.MethodGet, "/users/me/sessions", nil))
		s.Equal(http.StatusUnauthorized, w.Code)
	})
	s.Run("invalid limit", func() {
		w := s.serve(userRequest("/users/me/sessions?limit=0"))
		s.Equal(http.StatusBadRequest, w.Code)
	})
	s.Run("service error", func() {
		s.store.EXPECT().GetSubjectParticipantByTokenFamily(mock.Anything, "user-1", "tfid-1").
			Return(nil, errors.New("db down")).Once()
		w := s.serve(selfRequest("/users/me/sessions"))
		s.Equal(http.StatusInternalServerError, w.Code)
	})
}

func (s *HandlerTestSuite) TestPreflight() {
	for _, path := range []string{"/sessions", "/users/me/sessions", "/sessions/sess-1", "/users/me/sessions/sess-1"} {
		w := s.serve(httptest.NewRequest(http.MethodOptions, path, nil))
		s.Equal(http.StatusNoContent, w.Code, path)
	}
}

// deleteRequest is r with the DELETE method.
func deleteRequest(r *http.Request) *http.Request {
	r.Method = http.MethodDelete
	return r
}

func (s *HandlerTestSuite) TestRevoke_ByID() {
	s.Run("already gone is a 204", func() {
		s.store.EXPECT().GetBySessionID(mock.Anything, "sess-gone").Return(nil, nil).Once()
		w := s.serve(deleteRequest(adminRequest("/sessions/sess-gone")))
		s.Equal(http.StatusNoContent, w.Code)
	})
	s.Run("unauthorized", func() {
		w := s.serve(deleteRequest(userRequest("/sessions/sess-1")))
		s.Equal(http.StatusForbidden, w.Code)
	})
	s.Run("server error", func() {
		s.store.EXPECT().GetBySessionID(mock.Anything, "sess-1").Return(nil, errors.New("db down")).Once()
		w := s.serve(deleteRequest(adminRequest("/sessions/sess-1")))
		s.Equal(http.StatusInternalServerError, w.Code)
	})
}

func (s *HandlerTestSuite) TestRevoke_ByUser() {
	s.Run("revoked", func() {
		s.store.EXPECT().ListAllLiveBySubject(mock.Anything, "user-1", mock.Anything).Return([]Session{}, nil).Once()
		w := s.serve(deleteRequest(adminRequest("/sessions?userId=user-1")))
		s.Equal(http.StatusNoContent, w.Code)
	})
	s.Run("missing userId", func() {
		w := s.serve(deleteRequest(adminRequest("/sessions?userId=%20")))
		s.Equal(http.StatusBadRequest, w.Code)
		s.Contains(w.Body.String(), ErrorMissingUserID.Code)
	})
}

func (s *HandlerTestSuite) TestRevokeSelf_ByID() {
	s.Run("no such session is a 204", func() {
		s.store.EXPECT().GetSubjectParticipantByTokenFamily(mock.Anything, "user-1", "tfid-1").
			Return(nil, nil).Once()
		w := s.serve(deleteRequest(selfRequest("/users/me/sessions/sess-1")))
		s.Equal(http.StatusNoContent, w.Code)
	})
	s.Run("server error", func() {
		s.store.EXPECT().GetSubjectParticipantByTokenFamily(mock.Anything, "user-1", "tfid-1").
			Return(nil, errors.New("db down")).Once()
		w := s.serve(deleteRequest(selfRequest("/users/me/sessions/sess-1")))
		s.Equal(http.StatusInternalServerError, w.Code)
	})
	s.Run("no subject", func() {
		w := s.serve(httptest.NewRequest(http.MethodDelete, "/users/me/sessions/sess-1", nil))
		s.Equal(http.StatusUnauthorized, w.Code)
	})
}

// A delegated token, issued to a client acting for the user, can't revoke the user's sessions.
func (s *HandlerTestSuite) TestRevokeSelf_RefusesADelegatedToken() {
	ctx := security.WithSecurityContextTest(context.Background(), security.NewSecurityContextForTest("user-1", "",
		"", nil, map[string]interface{}{"tfid": "tfid-1", "act": map[string]interface{}{"sub": "client-1"}}))
	for _, path := range []string{"/users/me/sessions", "/users/me/sessions/sess-1"} {
		w := s.serve(httptest.NewRequest(http.MethodDelete, path, nil).WithContext(ctx))
		s.Equal(http.StatusForbidden, w.Code, path)
		s.Contains(w.Body.String(), ErrorDelegatedTokenNotAllowed.Code, path)
	}
}

func (s *HandlerTestSuite) TestRevokeSelf_All() {
	s.Run("revoked", func() {
		s.store.EXPECT().GetSubjectParticipantByTokenFamily(mock.Anything, "user-1", "tfid-1").
			Return(nil, nil).Once()
		w := s.serve(deleteRequest(selfRequest("/users/me/sessions?exceptCurrent=false")))
		s.Equal(http.StatusNoContent, w.Code)
	})
	s.Run("current session unknown", func() {
		w := s.serve(deleteRequest(userRequest("/users/me/sessions?exceptCurrent=true")))
		s.Equal(http.StatusBadRequest, w.Code)
		s.Contains(w.Body.String(), ErrorCurrentSessionUnknown.Code)
	})
	for _, value := range []string{"maybe", "1", "TRUE"} {
		s.Run("invalid exceptCurrent "+value, func() {
			w := s.serve(deleteRequest(selfRequest("/users/me/sessions?exceptCurrent=" + value)))
			s.Equal(http.StatusBadRequest, w.Code)
			s.Contains(w.Body.String(), ErrorInvalidExceptCurrent.Code)
		})
	}
	s.Run("no subject", func() {
		w := s.serve(httptest.NewRequest(http.MethodDelete, "/users/me/sessions", nil))
		s.Equal(http.StatusUnauthorized, w.Code)
	})
}
