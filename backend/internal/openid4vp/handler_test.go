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

package openid4vp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const (
	apiBaseURL            = "https://verifier.example"
	resultRedirectURIBase = apiBaseURL + "/result"
)

type OpenID4VPHandlerTestSuite struct {
	suite.Suite
}

func TestOpenID4VPHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(OpenID4VPHandlerTestSuite))
}

// =============================================================================
// Wallet-facing handler tests
// =============================================================================

func (suite *OpenID4VPHandlerTestSuite) TestHandleRequestObject() {
	t := suite.T()
	b := newPIDBuilder(t)
	svc, _ := newTestService(t, b)
	h := newOpenID4VPHandler(svc, nil, "", 0, 0)

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
	svc, store := newTestService(t, b)
	svc.cfg.ResultRedirectURIBase = resultRedirectURIBase
	h := newOpenID4VPHandler(svc, nil, "", 0, 0)

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
	svc, store := newTestService(t, b)
	h := newOpenID4VPHandler(svc, nil, "", 0, 0)

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

// =============================================================================
// RP-facing handler tests
// =============================================================================

// newTestRPHandler builds an openID4VPHandler over the standard test service
// plus a deterministic result-token issuer. Tests that need to assert on
// the issuer's recorded calls construct the handler inline (see
// TestAPIInitiateAndStatusCompleted).
func newTestRPHandler(t *testing.T) (*openID4VPHandler, map[string]*RequestState) {
	t.Helper()
	b := newPIDBuilder(t)
	svc, store := newTestService(t, b)
	issuer := stubResultTokenIssuer(t)
	h := newOpenID4VPHandler(svc, issuer, apiBaseURL, 300*time.Second, 0)
	return h, store
}

func (suite *OpenID4VPHandlerTestSuite) TestAPIInitiateHappyPath() {
	h, _ := newTestRPHandler(suite.T())

	body, _ := json.Marshal(initiateRequest{
		DefinitionID: testDefinitionID,
		RPID:         "scholarbooks",
	})
	rec := postJSON(h.HandleInitiate, body)
	suite.Require().Equal(http.StatusOK, rec.Code)

	var resp initiateResponse
	suite.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &resp))
	suite.NotEmpty(resp.TxnID)
	suite.Contains(resp.WalletURL, "openid4vp://")
	suite.Contains(resp.WalletURL, "request_uri=")
	suite.Equal(apiBaseURL+"/openid4vp/status/"+resp.TxnID, resp.StatusURL)
	parsed, err := time.Parse(time.RFC3339, resp.ExpiresAt)
	suite.Require().NoError(err)
	suite.True(parsed.After(time.Now().Add(-time.Second)))
}

func (suite *OpenID4VPHandlerTestSuite) TestAPIInitiateRejectsUnknownDefinition() {
	h, _ := newTestRPHandler(suite.T())

	body, _ := json.Marshal(initiateRequest{DefinitionID: "no-such-def", RPID: "rp"})
	rec := postJSON(h.HandleInitiate, body)
	suite.Equal(http.StatusBadRequest, rec.Code)
	suite.Equal(ErrorUnknownDefinition.Code, decodeErrorCode(suite.T(), rec))
}

func (suite *OpenID4VPHandlerTestSuite) TestAPIInitiateRejectsMissingFields() {
	h, _ := newTestRPHandler(suite.T())

	cases := []initiateRequest{
		{},
		{DefinitionID: testDefinitionID},
		{RPID: "scholarbooks"},
	}
	for _, c := range cases {
		body, _ := json.Marshal(c)
		rec := postJSON(h.HandleInitiate, body)
		suite.Equal(http.StatusBadRequest, rec.Code)
		suite.Equal(ErrorInvalidRequest.Code, decodeErrorCode(suite.T(), rec))
	}
}

func (suite *OpenID4VPHandlerTestSuite) TestAPIInitiateRejectsInvalidJSON() {
	h, _ := newTestRPHandler(suite.T())
	rec := postJSON(h.HandleInitiate, []byte("not json"))
	suite.Equal(http.StatusBadRequest, rec.Code)
	suite.Equal(ErrorInvalidRequest.Code, decodeErrorCode(suite.T(), rec))
}

func (suite *OpenID4VPHandlerTestSuite) TestAPIStatusUnknownTxn() {
	h, _ := newTestRPHandler(suite.T())
	rec := getStatus(h.HandleStatus, "nope")
	suite.Equal(http.StatusNotFound, rec.Code)
	suite.Equal(ErrorUnknownState.Code, decodeErrorCode(suite.T(), rec))
}

func (suite *OpenID4VPHandlerTestSuite) TestAPIStatusPending() {
	h, _ := newTestRPHandler(suite.T())

	body, _ := json.Marshal(initiateRequest{DefinitionID: testDefinitionID, RPID: "rp-1"})
	postJSON(h.HandleInitiate, body)
	var resp initiateResponse
	suite.Require().NoError(json.Unmarshal(postJSON(h.HandleInitiate, body).Body.Bytes(), &resp))

	rec := getStatus(h.HandleStatus, resp.TxnID)
	suite.Require().Equal(http.StatusOK, rec.Code)
	var s statusResponse
	suite.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &s))
	suite.Equal("PENDING", s.Status)
	suite.Empty(s.ResultToken)
}

func (suite *OpenID4VPHandlerTestSuite) TestAPIStatusFailed() {
	h, store := newTestRPHandler(suite.T())

	rec := postJSON(h.HandleInitiate, mustJSON(suite.T(), initiateRequest{
		DefinitionID: testDefinitionID, RPID: "rp",
	}))
	suite.Require().Equal(http.StatusOK, rec.Code)
	var ir initiateResponse
	suite.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &ir))

	// Flip the stored state to FAILED.
	rs := store[ir.TxnID]
	rs.Status = StatusFailed
	rs.FailureReason = "untrusted_issuer"

	srec := getStatus(h.HandleStatus, ir.TxnID)
	suite.Require().Equal(http.StatusOK, srec.Code)
	var sr statusResponse
	suite.Require().NoError(json.Unmarshal(srec.Body.Bytes(), &sr))
	suite.Equal("FAILED", sr.Status)
	suite.Equal("untrusted_issuer", sr.Error)
}

func (suite *OpenID4VPHandlerTestSuite) TestAPIStatusExpired() {
	h, store := newTestRPHandler(suite.T())

	rec := postJSON(h.HandleInitiate, mustJSON(suite.T(), initiateRequest{
		DefinitionID: testDefinitionID, RPID: "rp",
	}))
	suite.Require().Equal(http.StatusOK, rec.Code)
	var ir initiateResponse
	suite.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &ir))

	store[ir.TxnID].ExpiresAt = time.Now().Add(-time.Minute)

	srec := getStatus(h.HandleStatus, ir.TxnID)
	suite.Require().Equal(http.StatusOK, srec.Code)
	var sr statusResponse
	suite.Require().NoError(json.Unmarshal(srec.Body.Bytes(), &sr))
	suite.Equal("EXPIRED", sr.Status)
}

// End-to-end happy path: initiate -> wallet response -> status returns a
// COMPLETED payload with a result_token whose claims match the contract.
func (suite *OpenID4VPHandlerTestSuite) TestAPIInitiateAndStatusCompleted() {
	t := suite.T()
	b := newPIDBuilder(t)
	svc, store := newTestService(t, b)
	issuer := stubResultTokenIssuer(t)
	h := newOpenID4VPHandler(svc, issuer, apiBaseURL, 300*time.Second, 0)

	rec := postJSON(h.HandleInitiate, mustJSON(t, initiateRequest{
		DefinitionID: testDefinitionID, RPID: "scholarbooks",
	}))
	suite.Require().Equal(http.StatusOK, rec.Code)
	var ir initiateResponse
	suite.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &ir))

	rs := store[ir.TxnID]
	suite.Require().NotNil(rs)
	suite.Equal("scholarbooks", rs.RPID)

	// Simulate a successful wallet response.
	presentation := b.build(rs.Nonce, map[string]interface{}{
		"given_name": "Erika", "family_name": "Mustermann",
	})
	body, err := json.Marshal(map[string]interface{}{
		"state":    ir.TxnID,
		"vp_token": map[string]interface{}{credentialID: []string{presentation}},
	})
	suite.Require().NoError(err)
	jweToken := fabricateResponseJWE(t, &rs.EphemeralKey.PublicKey, body)
	_, svcErr := svc.SubmitResponse(context.Background(), ir.TxnID, []byte(jweToken))
	suite.Require().Nil(svcErr)

	srec := getStatus(h.HandleStatus, ir.TxnID)
	suite.Require().Equal(http.StatusOK, srec.Code)
	var sr statusResponse
	suite.Require().NoError(json.Unmarshal(srec.Body.Bytes(), &sr))
	suite.Equal("COMPLETED", sr.Status)
	suite.NotEmpty(sr.ResultToken)

	// The issuer received the RP id from initiate, and the synthesized token
	// carries the contract-required claims. Subject and verified_claims are
	// only available inside the signed result_token; they are not echoed
	// unsigned on the response envelope.
	issuer.AssertCalled(t, "issueResultToken", mock.Anything, "scholarbooks", mock.Anything, int64(300))
	claims := decodeFakeToken(t, sr.ResultToken)
	suite.Equal("scholarbooks", claims["aud"])
	suite.Equal(ir.TxnID, claims["txn"])
	suite.Equal(testDefinitionID, claims["definition_id"])
	suite.Contains(claims, "subject")
	vc := claims["verified_claims"].(map[string]interface{})
	suite.Equal("Erika", vc["given_name"])
}

func (suite *OpenID4VPHandlerTestSuite) TestHandleTrustAnchors() {
	t := suite.T()
	b := newPIDBuilder(t)
	svc, _ := newTestService(t, b)
	h := newOpenID4VPHandler(svc, nil, "", 0, 0)

	req := httptest.NewRequest(http.MethodGet, apiTrustAnchorsPath, nil)
	rec := httptest.NewRecorder()
	h.HandleTrustAnchors(rec, req)

	suite.Require().Equal(http.StatusOK, rec.Code)
	var anchors []TrustAnchorInfo
	suite.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &anchors))
	suite.Require().Len(anchors, 1)
	suite.Equal("test-root", anchors[0].Name)
	suite.NotEmpty(anchors[0].Subject)
}

func (suite *OpenID4VPHandlerTestSuite) TestAPIRegisterRoutesAddsInitiateAndStatus() {
	t := suite.T()
	h, _ := newTestRPHandler(t)
	mux := http.NewServeMux()
	registerRoutes(mux, h)

	// POST initiate is routed.
	body := mustJSON(t, initiateRequest{DefinitionID: testDefinitionID, RPID: "rp"})
	req := httptest.NewRequest(http.MethodPost, apiInitiatePath, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	suite.Require().Equal(http.StatusOK, rec.Code, rec.Body.String())
	var ir initiateResponse
	suite.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &ir))

	// GET status/{txn_id} resolves the path value.
	statusReq := httptest.NewRequest(http.MethodGet, "/openid4vp/status/"+ir.TxnID, nil)
	statusRec := httptest.NewRecorder()
	mux.ServeHTTP(statusRec, statusReq)
	suite.Require().Equal(http.StatusOK, statusRec.Code, statusRec.Body.String())
	var sr statusResponse
	suite.Require().NoError(json.Unmarshal(statusRec.Body.Bytes(), &sr))
	suite.Equal("PENDING", sr.Status)
}

func (suite *OpenID4VPHandlerTestSuite) TestHandleRequestObjectWriteError() {
	t := suite.T()
	b := newPIDBuilder(t)
	svc, _ := newTestService(t, b)
	h := newOpenID4VPHandler(svc, nil, "", 0, 0)

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
	b := newPIDBuilder(t)
	svc, _ := newTestService(t, b)
	h := newOpenID4VPHandler(svc, nil, "", 0, 0)

	req := httptest.NewRequest(http.MethodPost, "/openid4vp/response", strings.NewReader("%zz"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.HandleResponse(rec, req)

	suite.Equal(http.StatusBadRequest, rec.Code)
	suite.Equal(ErrorInvalidRequest.Code, decodeErrorCode(suite.T(), rec))
}

func (suite *OpenID4VPHandlerTestSuite) TestAPIInitiateServiceError() {
	svc := NewOpenID4VPServiceInterfaceMock(suite.T())
	svc.On("InitiateForRP", mock.Anything, testDefinitionID, "rp").
		Return(nil, &tidcommon.InternalServerError)
	h := newOpenID4VPHandler(svc, stubResultTokenIssuer(suite.T()), apiBaseURL, 0, 0)

	body, _ := json.Marshal(initiateRequest{DefinitionID: testDefinitionID, RPID: "rp"})
	rec := postJSON(h.HandleInitiate, body)
	suite.Equal(http.StatusInternalServerError, rec.Code)
}

func (suite *OpenID4VPHandlerTestSuite) TestAPIStatusLookupServiceError() {
	svc := NewOpenID4VPServiceInterfaceMock(suite.T())
	svc.On("LookupState", mock.Anything, "txn").Return(nil, &tidcommon.InternalServerError)
	h := newOpenID4VPHandler(svc, stubResultTokenIssuer(suite.T()), apiBaseURL, 0, 0)

	rec := getStatus(h.HandleStatus, "txn")
	suite.Equal(http.StatusInternalServerError, rec.Code)
}

func (suite *OpenID4VPHandlerTestSuite) TestAPIStatusEmptyTxn() {
	h, _ := newTestRPHandler(suite.T())
	rec := getStatus(h.HandleStatus, "")
	suite.Equal(http.StatusBadRequest, rec.Code)
	suite.Equal(ErrorInvalidRequest.Code, decodeErrorCode(suite.T(), rec))
}

func (suite *OpenID4VPHandlerTestSuite) TestAPIStatusCompletedIssuerNotConfigured() {
	t := suite.T()
	b := newPIDBuilder(t)
	svc, store := newTestService(t, b)
	h := newOpenID4VPHandler(svc, nil, apiBaseURL, 300*time.Second, 0)

	ir := initiateForTest(t, h, store)
	rs := store[ir.TxnID]
	rs.Status = StatusCompleted
	rs.Result = &VerifiedPresentation{Subject: "sub"}

	rec := getStatus(h.HandleStatus, ir.TxnID)
	suite.Equal(http.StatusInternalServerError, rec.Code)
}

func (suite *OpenID4VPHandlerTestSuite) TestAPIStatusCompletedIssuerError() {
	t := suite.T()
	b := newPIDBuilder(t)
	svc, store := newTestService(t, b)
	issuer := failingResultTokenIssuer(t, errors.New("sign failed"))
	h := newOpenID4VPHandler(svc, issuer, apiBaseURL, 300*time.Second, 0)

	ir := initiateForTest(t, h, store)
	rs := store[ir.TxnID]
	rs.Status = StatusCompleted
	rs.Result = &VerifiedPresentation{Subject: "sub"}

	rec := getStatus(h.HandleStatus, ir.TxnID)
	suite.Equal(http.StatusInternalServerError, rec.Code)
}

func (suite *OpenID4VPHandlerTestSuite) TestAPIStatusUnknownStatusValue() {
	t := suite.T()
	b := newPIDBuilder(t)
	svc, store := newTestService(t, b)
	issuer := stubResultTokenIssuer(t)
	h := newOpenID4VPHandler(svc, issuer, apiBaseURL, 300*time.Second, 0)

	ir := initiateForTest(t, h, store)
	store[ir.TxnID].Status = Status("BOGUS")

	rec := getStatus(h.HandleStatus, ir.TxnID)
	suite.Equal(http.StatusInternalServerError, rec.Code)
}

// =============================================================================
// Shared test helpers
// =============================================================================

func initiateForTest(t *testing.T, h *openID4VPHandler, store map[string]*RequestState) initiateResponse {
	t.Helper()
	rec := postJSON(h.HandleInitiate, mustJSON(t, initiateRequest{
		DefinitionID: testDefinitionID, RPID: "rp",
	}))
	require.Equal(t, http.StatusOK, rec.Code)
	var ir initiateResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &ir))
	require.NotNil(t, store[ir.TxnID])
	return ir
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

func postJSON(handler http.HandlerFunc, body []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, apiInitiatePath, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func getStatus(handler http.HandlerFunc, txnID string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/openid4vp/status/"+txnID, nil)
	req.SetPathValue("txn_id", txnID)
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func mustJSON(t *testing.T, v interface{}) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}

func decodeErrorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var resp struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	return resp.Code
}
