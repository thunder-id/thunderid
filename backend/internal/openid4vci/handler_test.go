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

package openid4vci

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dpop"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/dpopmock"
)

type OpenID4VCIHandlerTestSuite struct {
	suite.Suite
}

func TestOpenID4VCIHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(OpenID4VCIHandlerTestSuite))
}

// errReader is an io.Reader that always fails, to exercise request-body read errors.
type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

func makeToken(t *testing.T, payload map[string]any) string {
	t.Helper()
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return "e30." + base64.RawURLEncoding.EncodeToString(b) + ".sig"
}

func (s *OpenID4VCIHandlerTestSuite) TestNewOpenID4VCIHandler() {
	svc := NewOpenID4VCIServiceInterfaceMock(s.T())
	h := newOpenID4VCIHandler(svc, nil, "https://i/credential")
	s.Equal(svc, h.service)
	s.Equal("https://i/credential", h.credentialEndpoint)
}

func (s *OpenID4VCIHandlerTestSuite) TestBearerToken() {
	cases := []struct {
		header string
		want   string
	}{
		{"Bearer abc", "abc"},
		{"bearer abc", "abc"},
		{"DPoP xyz", "xyz"},
		{"Basic zzz", ""},
		{"", ""},
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodPost, "/openid4vci/credential", nil)
		if c.header != "" {
			req.Header.Set("Authorization", c.header)
		}
		s.Equal(c.want, bearerToken(req))
	}
}

func (s *OpenID4VCIHandlerTestSuite) TestHandleMetadata() {
	svc := NewOpenID4VCIServiceInterfaceMock(s.T())
	svc.EXPECT().GetMetadata(mock.Anything).Return(map[string]interface{}{"credential_issuer": "https://i"})
	h := newOpenID4VCIHandler(svc, nil, "")

	rr := httptest.NewRecorder()
	h.HandleMetadata(rr, httptest.NewRequest(http.MethodGet, metadataPath, nil))
	s.Equal(http.StatusOK, rr.Code)
	s.Contains(rr.Body.String(), "credential_issuer")
}

func (s *OpenID4VCIHandlerTestSuite) TestHandleOffer() {
	s.Run("MissingConfig", func() {
		svc := NewOpenID4VCIServiceInterfaceMock(s.T())
		h := newOpenID4VCIHandler(svc, nil, "")
		rr := httptest.NewRecorder()
		h.HandleOffer(rr, httptest.NewRequest(http.MethodGet, offerPath, nil))
		s.Equal(http.StatusBadRequest, rr.Code)
	})

	s.Run("Success", func() {
		svc := NewOpenID4VCIServiceInterfaceMock(s.T())
		svc.EXPECT().GenerateCredentialOffer(mock.Anything, "eudi-pid").
			Return(map[string]interface{}{"credential_issuer": "https://i"}, "openid-credential-offer://x", nil)
		h := newOpenID4VCIHandler(svc, nil, "")
		rr := httptest.NewRecorder()
		h.HandleOffer(rr, httptest.NewRequest(http.MethodGet, offerPath+"?credential_configuration_id=eudi-pid", nil))
		s.Equal(http.StatusOK, rr.Code)
		s.Contains(rr.Body.String(), "credential_offer_uri")
	})

	s.Run("ServiceError", func() {
		svc := NewOpenID4VCIServiceInterfaceMock(s.T())
		svc.EXPECT().GenerateCredentialOffer(mock.Anything, "x").
			Return(nil, "", ErrUnsupportedCredential)
		h := newOpenID4VCIHandler(svc, nil, "")
		rr := httptest.NewRecorder()
		h.HandleOffer(rr, httptest.NewRequest(http.MethodGet, offerPath+"?credential_configuration_id=x", nil))
		s.Equal(http.StatusBadRequest, rr.Code)
	})
}

func (s *OpenID4VCIHandlerTestSuite) TestHandleCredentialOffer() {
	s.Run("MissingID", func() {
		svc := NewOpenID4VCIServiceInterfaceMock(s.T())
		h := newOpenID4VCIHandler(svc, nil, "")
		rr := httptest.NewRecorder()
		h.HandleCredentialOffer(rr, httptest.NewRequest(http.MethodGet, credentialOfferPath+"/", nil))
		s.Equal(http.StatusBadRequest, rr.Code)
	})

	s.Run("Success", func() {
		svc := NewOpenID4VCIServiceInterfaceMock(s.T())
		svc.EXPECT().GetCredentialOffer(mock.Anything, "o1").
			Return(map[string]interface{}{"credential_issuer": "https://i"}, nil)
		h := newOpenID4VCIHandler(svc, nil, "")
		req := httptest.NewRequest(http.MethodGet, credentialOfferPath+"/o1", nil)
		req.SetPathValue("id", "o1")
		rr := httptest.NewRecorder()
		h.HandleCredentialOffer(rr, req)
		s.Equal(http.StatusOK, rr.Code)
		s.Equal("no-store", rr.Header().Get("Cache-Control"))
	})

	s.Run("NotFound", func() {
		svc := NewOpenID4VCIServiceInterfaceMock(s.T())
		svc.EXPECT().GetCredentialOffer(mock.Anything, "missing").Return(nil, ErrUnsupportedCredential)
		h := newOpenID4VCIHandler(svc, nil, "")
		req := httptest.NewRequest(http.MethodGet, credentialOfferPath+"/missing", nil)
		req.SetPathValue("id", "missing")
		rr := httptest.NewRecorder()
		h.HandleCredentialOffer(rr, req)
		s.Equal(http.StatusBadRequest, rr.Code)
	})
}

func (s *OpenID4VCIHandlerTestSuite) TestHandleNonce() {
	s.Run("Success", func() {
		svc := NewOpenID4VCIServiceInterfaceMock(s.T())
		svc.EXPECT().GenerateNonce(mock.Anything).Return("the-nonce", nil)
		h := newOpenID4VCIHandler(svc, nil, "")
		rr := httptest.NewRecorder()
		h.HandleNonce(rr, httptest.NewRequest(http.MethodPost, noncePath, nil))
		s.Equal(http.StatusOK, rr.Code)
		s.Contains(rr.Body.String(), "the-nonce")
	})

	s.Run("Error", func() {
		svc := NewOpenID4VCIServiceInterfaceMock(s.T())
		svc.EXPECT().GenerateNonce(mock.Anything).Return("", errors.New("boom"))
		h := newOpenID4VCIHandler(svc, nil, "")
		rr := httptest.NewRecorder()
		h.HandleNonce(rr, httptest.NewRequest(http.MethodPost, noncePath, nil))
		s.Equal(http.StatusInternalServerError, rr.Code)
	})
}

func (s *OpenID4VCIHandlerTestSuite) TestHandleCredential() {
	token := makeToken(s.T(), map[string]any{"sub": "u1"})

	s.Run("AccessTokenInQuery", func() {
		svc := NewOpenID4VCIServiceInterfaceMock(s.T())
		h := newOpenID4VCIHandler(svc, nil, "")
		rr := httptest.NewRecorder()
		h.HandleCredential(rr, httptest.NewRequest(http.MethodPost, credentialPath+"?access_token=x", nil))
		s.Equal(http.StatusUnauthorized, rr.Code)
	})

	s.Run("MissingToken", func() {
		svc := NewOpenID4VCIServiceInterfaceMock(s.T())
		h := newOpenID4VCIHandler(svc, nil, "")
		rr := httptest.NewRecorder()
		h.HandleCredential(rr, httptest.NewRequest(http.MethodPost, credentialPath, nil))
		s.Equal(http.StatusUnauthorized, rr.Code)
	})

	s.Run("Success", func() {
		svc := NewOpenID4VCIServiceInterfaceMock(s.T())
		svc.EXPECT().IssueCredential(mock.Anything, token, mock.Anything).
			Return(&CredentialResponse{Credentials: []IssuedCredential{{Credential: "vc"}}}, nil)
		h := newOpenID4VCIHandler(svc, nil, "")
		req := httptest.NewRequest(http.MethodPost, credentialPath, strings.NewReader("{}"))
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		h.HandleCredential(rr, req)
		s.Equal(http.StatusOK, rr.Code)
		s.Contains(rr.Body.String(), "vc")
	})

	s.Run("IssueErrorAddsNonce", func() {
		svc := NewOpenID4VCIServiceInterfaceMock(s.T())
		svc.EXPECT().IssueCredential(mock.Anything, token, mock.Anything).Return(nil, ErrInvalidProof)
		svc.EXPECT().GenerateNonce(mock.Anything).Return("fresh", nil)
		h := newOpenID4VCIHandler(svc, nil, "")
		req := httptest.NewRequest(http.MethodPost, credentialPath, strings.NewReader("{}"))
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		h.HandleCredential(rr, req)
		s.Equal(http.StatusBadRequest, rr.Code)
		s.Contains(rr.Body.String(), "fresh")
	})

	s.Run("DPoPRequiredButMissing", func() {
		svc := NewOpenID4VCIServiceInterfaceMock(s.T())
		boundToken := makeToken(s.T(), map[string]any{"sub": "u1", "cnf": map[string]any{"jkt": "abc"}})
		h := newOpenID4VCIHandler(svc, nil, "https://i/credential")
		req := httptest.NewRequest(http.MethodPost, credentialPath, strings.NewReader("{}"))
		req.Header.Set("Authorization", "Bearer "+boundToken)
		rr := httptest.NewRecorder()
		h.HandleCredential(rr, req)
		s.Equal(http.StatusUnauthorized, rr.Code)
		s.Contains(rr.Header().Get("WWW-Authenticate"), "DPoP")
	})

	s.Run("BodyReadError", func() {
		svc := NewOpenID4VCIServiceInterfaceMock(s.T())
		h := newOpenID4VCIHandler(svc, nil, "")
		req := httptest.NewRequest(http.MethodPost, credentialPath, errReader{})
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		h.HandleCredential(rr, req)
		s.Equal(http.StatusBadRequest, rr.Code)
	})

	s.Run("IssueErrorOther", func() {
		svc := NewOpenID4VCIServiceInterfaceMock(s.T())
		svc.EXPECT().IssueCredential(mock.Anything, token, mock.Anything).Return(nil, ErrInvalidToken)
		h := newOpenID4VCIHandler(svc, nil, "")
		req := httptest.NewRequest(http.MethodPost, credentialPath, strings.NewReader("{}"))
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		h.HandleCredential(rr, req)
		s.Equal(http.StatusUnauthorized, rr.Code)
	})
}

func (s *OpenID4VCIHandlerTestSuite) TestWriteOID4VCIError() {
	s.Run("UnauthorizedBearer", func() {
		rr := httptest.NewRecorder()
		writeOID4VCIError(rr, oid4vciError{
			Status: http.StatusUnauthorized, Code: errCodeInvalidToken, Description: "d",
		})
		s.Equal(http.StatusUnauthorized, rr.Code)
		s.Contains(rr.Header().Get("WWW-Authenticate"), "Bearer")
		s.Equal("no-store", rr.Header().Get("Cache-Control"))
	})

	s.Run("UnauthorizedDPoP", func() {
		rr := httptest.NewRecorder()
		writeOID4VCIError(rr, oid4vciError{
			Status: http.StatusUnauthorized, Code: errCodeInvalidDPoPProof, Description: "d",
		})
		s.Contains(rr.Header().Get("WWW-Authenticate"), "DPoP")
	})

	s.Run("NonAuth", func() {
		rr := httptest.NewRecorder()
		writeOID4VCIError(rr, oid4vciError{Status: http.StatusBadRequest, Code: errCodeInvalidProof})
		s.Equal(http.StatusBadRequest, rr.Code)
		s.Empty(rr.Header().Get("WWW-Authenticate"))
	})
}

func (s *OpenID4VCIHandlerTestSuite) TestVerifyDPoPBearerTokenSkipped() {
	h := &openID4VCIHandler{}
	token := makeToken(s.T(), map[string]any{"sub": "u1"})
	req := httptest.NewRequest(http.MethodPost, "/openid4vci/credential", nil)
	s.NoError(h.verifyDPoP(req, token), "bearer (unbound) token should skip DPoP")
}

func (s *OpenID4VCIHandlerTestSuite) TestVerifyDPoPBadCnfClaim() {
	h := &openID4VCIHandler{}
	token := makeToken(s.T(), map[string]any{"sub": "u1", "cnf": "not-an-object"})
	req := httptest.NewRequest(http.MethodPost, "/openid4vci/credential", nil)
	s.ErrorIs(h.verifyDPoP(req, token), ErrInvalidToken)
}

func (s *OpenID4VCIHandlerTestSuite) TestVerifyDPoPBoundTokenRequiresProof() {
	h := &openID4VCIHandler{}
	token := makeToken(s.T(), map[string]any{"sub": "u1", "cnf": map[string]any{"jkt": "abc"}})
	req := httptest.NewRequest(http.MethodPost, "/openid4vci/credential", nil)
	s.ErrorIs(h.verifyDPoP(req, token), ErrInvalidDPoP, "DPoP-bound token without proof should fail")
}

func (s *OpenID4VCIHandlerTestSuite) TestVerifyDPoPBadToken() {
	h := &openID4VCIHandler{}
	req := httptest.NewRequest(http.MethodPost, "/openid4vci/credential", nil)
	s.ErrorIs(h.verifyDPoP(req, "not-a-jwt"), ErrInvalidToken)
}

func (s *OpenID4VCIHandlerTestSuite) TestVerifyDPoPBoundTokenSuccess() {
	verifier := dpopmock.NewVerifierInterfaceMock(s.T())
	verifier.EXPECT().Verify(mock.Anything, mock.Anything).Return(&dpop.ProofResult{}, nil)
	h := &openID4VCIHandler{dpopVerifier: verifier, credentialEndpoint: "https://i/credential"}
	token := makeToken(s.T(), map[string]any{"sub": "u1", "cnf": map[string]any{"jkt": "abc"}})
	req := httptest.NewRequest(http.MethodPost, "/openid4vci/credential", nil)
	req.Header.Set("DPoP", "proof")
	s.NoError(h.verifyDPoP(req, token))
}

func (s *OpenID4VCIHandlerTestSuite) TestVerifyDPoPBoundTokenVerifyFails() {
	verifier := dpopmock.NewVerifierInterfaceMock(s.T())
	verifier.EXPECT().Verify(mock.Anything, mock.Anything).Return(nil, errors.New("bad proof"))
	h := &openID4VCIHandler{dpopVerifier: verifier, credentialEndpoint: "https://i/credential"}
	token := makeToken(s.T(), map[string]any{"sub": "u1", "cnf": map[string]any{"jkt": "abc"}})
	req := httptest.NewRequest(http.MethodPost, "/openid4vci/credential", nil)
	req.Header.Set("DPoP", "proof")
	s.ErrorIs(h.verifyDPoP(req, token), ErrInvalidDPoP)
}
