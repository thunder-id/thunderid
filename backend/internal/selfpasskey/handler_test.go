// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package selfpasskey

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/authn/passkey"
	"github.com/thunder-id/thunderid/internal/system/security"
)

type HandlerTestSuite struct {
	suite.Suite
	service *selfPasskeyServiceInterfaceMock
	mux     *http.ServeMux
}

func TestHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(HandlerTestSuite))
}

func (s *HandlerTestSuite) SetupTest() {
	s.service = newSelfPasskeyServiceInterfaceMock(s.T())
	s.mux = http.NewServeMux()
	registerRoutes(s.mux, newSelfPasskeyHandler(s.service))
}

// serve sends a request as the given caller, whose token carries the given claims.
func (s *HandlerTestSuite) serve(method, path, body, userID string,
	claims map[string]interface{}) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req = req.WithContext(security.WithSecurityContextTest(req.Context(),
		security.NewSecurityContextForTest(userID, "", "", nil, claims)))
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	return rec
}

func (s *HandlerTestSuite) TestRegistrationStart() {
	data := &passkey.PasskeyRegistrationStartData{SessionToken: "token"}
	s.service.On("StartRegistration", mock.Anything, testUserID, testClientID, "example.com").
		Return(data, nil).Once()

	rec := s.serve(http.MethodPost, "/users/me/passkeys/register/start", `{"relyingPartyId":"example.com"}`,
		testUserID, map[string]interface{}{claimClientID: testClientID})

	s.Equal(http.StatusOK, rec.Code)
	var body passkey.PasskeyRegistrationStartData
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &body))
	s.Equal("token", body.SessionToken)
}

func (s *HandlerTestSuite) TestRegistrationStart_InvalidBody() {
	rec := s.serve(http.MethodPost, "/users/me/passkeys/register/start", `{`, testUserID, nil)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Contains(rec.Body.String(), ErrorInvalidRequestFormat.Code)
}

func (s *HandlerTestSuite) TestRegistrationStart_ServiceError() {
	s.service.On("StartRegistration", mock.Anything, testUserID, "", "evil.com").
		Return(nil, &ErrorRelyingPartyNotAllowed).Once()

	rec := s.serve(http.MethodPost, "/users/me/passkeys/register/start", `{"relyingPartyId":"evil.com"}`,
		testUserID, nil)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Contains(rec.Body.String(), ErrorRelyingPartyNotAllowed.Code)
}

func (s *HandlerTestSuite) TestRegistrationFinish() {
	s.service.On("FinishRegistration", mock.Anything, testUserID, testClientID,
		mock.MatchedBy(func(req *registrationFinishRequest) bool {
			return req.SessionToken == "token" && req.Name == "Work laptop" &&
				req.PublicKeyCredential.Response.AttestationObject == "attestation"
		})).Return(&passkey.PasskeyInfo{ID: "cred-1", Name: "Work laptop"}, nil).Once()

	body := `{"publicKeyCredential":{"id":"cred-1","type":"public-key","response":` +
		`{"clientDataJSON":"client-data","attestationObject":"attestation"}},` +
		`"sessionToken":"token","name":"Work laptop"}`
	rec := s.serve(http.MethodPost, "/users/me/passkeys/register/finish", body,
		testUserID, map[string]interface{}{claimClientID: testClientID})

	s.Equal(http.StatusCreated, rec.Code)
	s.JSONEq(`{"id":"cred-1","name":"Work laptop"}`, rec.Body.String())
}

func (s *HandlerTestSuite) TestRegistrationFinish_InvalidBody() {
	rec := s.serve(http.MethodPost, "/users/me/passkeys/register/finish", `[]`, testUserID, nil)

	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *HandlerTestSuite) TestRegistrationFinish_HeldAccount() {
	for _, svcErr := range []*tidcommon.ServiceError{&common.ErrorAccountSuspended, &common.ErrorAccountExpired} {
		s.service.On("FinishRegistration", mock.Anything, testUserID, "", mock.Anything).Return(nil, svcErr).Once()

		rec := s.serve(http.MethodPost, "/users/me/passkeys/register/finish", `{}`, testUserID, nil)

		s.Equal(http.StatusForbidden, rec.Code, svcErr.Code)
	}
}

func (s *HandlerTestSuite) TestList() {
	s.service.On("ListPasskeys", mock.Anything, testUserID).
		Return([]passkey.PasskeyInfo{{ID: "cred-1", Name: "Work laptop"}, {ID: "cred-2"}}, nil).Once()

	rec := s.serve(http.MethodGet, "/users/me/passkeys", "", testUserID, nil)

	s.Equal(http.StatusOK, rec.Code)
	s.JSONEq(`{"passkeys":[{"id":"cred-1","name":"Work laptop"},{"id":"cred-2"}]}`, rec.Body.String())
}

func (s *HandlerTestSuite) TestList_Empty() {
	s.service.On("ListPasskeys", mock.Anything, testUserID).Return([]passkey.PasskeyInfo{}, nil).Once()

	rec := s.serve(http.MethodGet, "/users/me/passkeys", "", testUserID, nil)

	s.Equal(http.StatusOK, rec.Code)
	s.JSONEq(`{"passkeys":[]}`, rec.Body.String())
}

func (s *HandlerTestSuite) TestList_ServerError() {
	s.service.On("ListPasskeys", mock.Anything, testUserID).Return(nil, &tidcommon.InternalServerError).Once()

	rec := s.serve(http.MethodGet, "/users/me/passkeys", "", testUserID, nil)

	s.Equal(http.StatusInternalServerError, rec.Code)
}

func (s *HandlerTestSuite) TestDelete() {
	s.service.On("DeletePasskey", mock.Anything, testUserID, "cred-1").Return(nil).Once()

	rec := s.serve(http.MethodDelete, "/users/me/passkeys/cred-1", "", testUserID, nil)

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *HandlerTestSuite) TestDelete_NotFound() {
	for _, svcErr := range []*tidcommon.ServiceError{&passkey.ErrorCredentialNotFound, &passkey.ErrorUserNotFound} {
		s.service.On("DeletePasskey", mock.Anything, testUserID, "cred-1").Return(svcErr).Once()

		rec := s.serve(http.MethodDelete, "/users/me/passkeys/cred-1", "", testUserID, nil)

		s.Equal(http.StatusNotFound, rec.Code)
	}
}

func (s *HandlerTestSuite) TestRefusesCaller() {
	cases := []struct {
		name   string
		userID string
		claims map[string]interface{}
		status int
	}{
		{"no user", "", nil, http.StatusUnauthorized},
		{"delegated token", testUserID, map[string]interface{}{claimActor: map[string]interface{}{"sub": "agent"}},
			http.StatusForbidden},
	}
	requests := []struct{ method, path, body string }{
		{http.MethodPost, "/users/me/passkeys/register/start", `{"relyingPartyId":"example.com"}`},
		{http.MethodPost, "/users/me/passkeys/register/finish", `{}`},
		{http.MethodGet, "/users/me/passkeys", ""},
		{http.MethodDelete, "/users/me/passkeys/cred-1", ""},
	}
	for _, tc := range cases {
		for _, r := range requests {
			rec := s.serve(r.method, r.path, r.body, tc.userID, tc.claims)

			s.Equal(tc.status, rec.Code, tc.name+" "+r.method+" "+r.path)
		}
	}
}

func (s *HandlerTestSuite) TestPreflight() {
	for _, path := range []string{"/users/me/passkeys/register/start", "/users/me/passkeys/register/finish",
		"/users/me/passkeys", "/users/me/passkeys/cred-1"} {
		rec := s.serve(http.MethodOptions, path, "", "", nil)

		s.Equal(http.StatusNoContent, rec.Code, path)
	}
}
