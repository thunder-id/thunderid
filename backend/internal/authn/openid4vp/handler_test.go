// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package openid4vp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const (
	resultRedirectURIBase = "https://verifier.example/result"
)

type OpenID4VPHandlerTestSuite struct {
	suite.Suite
}

func TestOpenID4VPHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(OpenID4VPHandlerTestSuite))
}

func (suite *OpenID4VPHandlerTestSuite) TestHandleRequestObject() {
	t := suite.T()
	svc, _ := newTestService(t)
	h := newOpenID4VPHandler(svc, svc)

	init, svcErr := svc.Initiate(context.Background(), testDefinitionID)
	suite.Require().Nil(svcErr)

	suite.Run("success", func() {
		req := httptest.NewRequest(http.MethodGet, "/openid4vp/request?state="+url.QueryEscape(init.State), nil)
		rec := httptest.NewRecorder()
		h.HandleRequestObject(rec, req)

		suite.Equal(http.StatusOK, rec.Code)
		suite.Equal(requestObjectContentType, rec.Header().Get("Content-Type"))
		suite.Equal("no-store", rec.Header().Get("Cache-Control"))
		suite.Len(strings.Split(rec.Body.String(), "."), 3)
	})

	suite.Run("missing state", func() {
		req := httptest.NewRequest(http.MethodGet, "/openid4vp/request", nil)
		rec := httptest.NewRecorder()
		h.HandleRequestObject(rec, req)
		suite.Equal(http.StatusBadRequest, rec.Code)
		suite.Equal(ErrorInvalidRequest.Code, decodeErrorCode(suite.T(), rec))
	})

	suite.Run("unknown state", func() {
		req := httptest.NewRequest(http.MethodGet, "/openid4vp/request?state=nope", nil)
		rec := httptest.NewRecorder()
		h.HandleRequestObject(rec, req)
		suite.Equal(http.StatusNotFound, rec.Code)
		suite.Equal(ErrorUnknownState.Code, decodeErrorCode(suite.T(), rec))
	})
}

func (suite *OpenID4VPHandlerTestSuite) TestHandleResponse() {
	t := suite.T()
	b := newPIDBuilder(t)
	svc, store := newTestService(t)
	svc.cfg.ResultRedirectURIBase = resultRedirectURIBase
	h := newOpenID4VPHandler(svc, svc)

	init, svcErr := svc.Initiate(context.Background(), testDefinitionID)
	suite.Require().Nil(svcErr)
	rs := store[init.State]

	presentation := b.build(rs.Nonce, map[string]interface{}{
		"given_name": "Erika", "family_name": "Mustermann",
	})
	body, err := json.Marshal(map[string]interface{}{
		"state":    init.State,
		"vp_token": map[string]interface{}{credentialID: []string{presentation}},
	})
	suite.Require().NoError(err)
	jweToken := fabricateResponseJWE(t, &rs.EphemeralKey.PublicKey, body)

	suite.Run("success returns redirect_uri", func() {
		form := url.Values{"state": {init.State}, "response": {jweToken}}
		rec := postForm(h, form)
		suite.Equal(http.StatusOK, rec.Code)

		var resp map[string]string
		suite.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &resp))
		suite.Contains(resp["redirect_uri"], "state=")
	})

	suite.Run("missing fields", func() {
		rec := postForm(h, url.Values{"state": {init.State}})
		suite.Equal(http.StatusBadRequest, rec.Code)
		suite.Equal(ErrorInvalidRequest.Code, decodeErrorCode(suite.T(), rec))
	})
}

func (suite *OpenID4VPHandlerTestSuite) TestHandleResponseVerificationFailure() {
	t := suite.T()
	b := newPIDBuilder(t)
	svc, store := newTestService(t)
	h := newOpenID4VPHandler(svc, svc)

	init, svcErr := svc.Initiate(context.Background(), testDefinitionID)
	suite.Require().Nil(svcErr)
	rs := store[init.State]

	// Presentation bound to the wrong nonce -> verification fails.
	presentation := b.build("wrong-nonce", map[string]interface{}{"given_name": "Erika", "family_name": "M"})
	body, err := json.Marshal(map[string]interface{}{
		"state":    init.State,
		"vp_token": map[string]interface{}{credentialID: []string{presentation}},
	})
	suite.Require().NoError(err)
	jweToken := fabricateResponseJWE(t, &rs.EphemeralKey.PublicKey, body)

	rec := postForm(h, url.Values{"state": {init.State}, "response": {jweToken}})
	suite.Equal(http.StatusBadRequest, rec.Code)
	suite.Equal(ErrorVerificationFailed.Code, decodeErrorCode(suite.T(), rec))
}

func (suite *OpenID4VPHandlerTestSuite) TestHandleRequestObjectWriteError() {
	t := suite.T()
	svc, _ := newTestService(t)
	h := newOpenID4VPHandler(svc, svc)

	init, svcErr := svc.Initiate(context.Background(), testDefinitionID)
	suite.Require().Nil(svcErr)

	req := httptest.NewRequest(http.MethodGet, "/openid4vp/request?state="+url.QueryEscape(init.State), nil)
	rec := &failingResponseWriter{header: http.Header{}}
	h.HandleRequestObject(rec, req)

	suite.Equal(http.StatusOK, rec.status)
	suite.True(rec.writeCalled)
}

func (suite *OpenID4VPHandlerTestSuite) TestHandleResponseParseFormError() {
	t := suite.T()
	svc, _ := newTestService(t)
	h := newOpenID4VPHandler(svc, svc)

	req := httptest.NewRequest(http.MethodPost, "/openid4vp/response", strings.NewReader("%zz"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.HandleResponse(rec, req)

	suite.Equal(http.StatusBadRequest, rec.Code)
	suite.Equal(ErrorInvalidRequest.Code, decodeErrorCode(suite.T(), rec))
}

// failingResponseWriter records the written status code and fails every Write.
type failingResponseWriter struct {
	header      http.Header
	status      int
	writeCalled bool
}

func (w *failingResponseWriter) Header() http.Header { return w.header }

func (w *failingResponseWriter) Write([]byte) (int, error) {
	w.writeCalled = true
	return 0, errors.New("write failed")
}

func (w *failingResponseWriter) WriteHeader(code int) { w.status = code }

func postForm(h *openID4VPHandler, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/openid4vp/response", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.HandleResponse(rec, req)
	return rec
}

func decodeErrorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var resp struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	return resp.Code
}
