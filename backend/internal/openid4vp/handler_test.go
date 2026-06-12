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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authncommon "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
)

const (
	apiBaseURL            = "https://verifier.example"
	resultRedirectURIBase = apiBaseURL + "/result"
)

// =============================================================================
// Wallet-facing handler tests
// =============================================================================

func TestHandleRequestObject(t *testing.T) {
	b := newPIDBuilder(t)
	svc, _ := newTestService(t, b)
	h := newOpenID4VPHandler(svc, nil, "", 0, 0)

	init, err := svc.Initiate(context.Background(), testDefinitionID)
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/openid4vp/request?state="+url.QueryEscape(init.State), nil)
		rec := httptest.NewRecorder()
		h.HandleRequestObject(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, requestObjectContentType, rec.Header().Get("Content-Type"))
		assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
		assert.Len(t, strings.Split(rec.Body.String(), "."), 3)
	})

	t.Run("missing state", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/openid4vp/request", nil)
		rec := httptest.NewRecorder()
		h.HandleRequestObject(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Equal(t, ErrorInvalidRequest.Code, decodeErrorCode(t, rec))
	})

	t.Run("unknown state", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/openid4vp/request?state=nope", nil)
		rec := httptest.NewRecorder()
		h.HandleRequestObject(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.Equal(t, ErrorUnknownState.Code, decodeErrorCode(t, rec))
	})
}

func TestHandleResponse(t *testing.T) {
	b := newPIDBuilder(t)
	svc, store := newTestService(t, b)
	svc.cfg.ResultRedirectURIBase = resultRedirectURIBase
	h := newOpenID4VPHandler(svc, nil, "", 0, 0)

	init, err := svc.Initiate(context.Background(), testDefinitionID)
	require.NoError(t, err)
	rs := store.m[init.State]

	presentation := b.build(rs.Nonce, map[string]interface{}{
		"given_name": "Erika", "family_name": "Mustermann",
	})
	body, err := json.Marshal(map[string]interface{}{
		"state":    init.State,
		"vp_token": map[string]interface{}{credentialID: []string{presentation}},
	})
	require.NoError(t, err)
	jweToken := fabricateResponseJWE(t, &rs.EphemeralKey.PublicKey, body)

	t.Run("success returns redirect_uri", func(t *testing.T) {
		form := url.Values{"state": {init.State}, "response": {jweToken}}
		rec := postForm(h, form)
		assert.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]string
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.Contains(t, resp["redirect_uri"], "state=")
	})

	t.Run("missing fields", func(t *testing.T) {
		rec := postForm(h, url.Values{"state": {init.State}})
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Equal(t, ErrorInvalidRequest.Code, decodeErrorCode(t, rec))
	})
}

func TestHandleResponseVerificationFailure(t *testing.T) {
	b := newPIDBuilder(t)
	svc, store := newTestService(t, b)
	h := newOpenID4VPHandler(svc, nil, "", 0, 0)

	init, err := svc.Initiate(context.Background(), testDefinitionID)
	require.NoError(t, err)
	rs := store.m[init.State]

	// Presentation bound to the wrong nonce -> verification fails.
	presentation := b.build("wrong-nonce", map[string]interface{}{"given_name": "Erika", "family_name": "M"})
	body, err := json.Marshal(map[string]interface{}{
		"state":    init.State,
		"vp_token": map[string]interface{}{credentialID: []string{presentation}},
	})
	require.NoError(t, err)
	jweToken := fabricateResponseJWE(t, &rs.EphemeralKey.PublicKey, body)

	rec := postForm(h, url.Values{"state": {init.State}, "response": {jweToken}})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, ErrorVerificationFailed.Code, decodeErrorCode(t, rec))
}

// =============================================================================
// RP-facing handler tests
// =============================================================================

// newTestRPHandler builds an openID4VPHandler over the standard test service
// plus the deterministic resultTokenIssuerFake. Tests that need to assert on
// the issuer's recorded calls construct the handler inline (see
// TestAPIInitiateAndStatusCompleted).
func newTestRPHandler(t *testing.T) (*openID4VPHandler, *fakeStore) {
	t.Helper()
	b := newPIDBuilder(t)
	svc, store := newTestService(t, b)
	issuer := &resultTokenIssuerFake{}
	h := newOpenID4VPHandler(svc, issuer, apiBaseURL, 300*time.Second, 0)
	return h, store
}

func TestAPIInitiateHappyPath(t *testing.T) {
	h, _ := newTestRPHandler(t)

	body, _ := json.Marshal(initiateRequest{
		DefinitionID: testDefinitionID,
		RPID:         "scholarbooks",
	})
	rec := postJSON(h.HandleInitiate, body)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp initiateResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.TxnID)
	assert.Contains(t, resp.WalletURL, "openid4vp://")
	assert.Contains(t, resp.WalletURL, "request_uri=")
	assert.Equal(t, apiBaseURL+"/openid4vp/status/"+resp.TxnID, resp.StatusURL)
	parsed, err := time.Parse(time.RFC3339, resp.ExpiresAt)
	require.NoError(t, err)
	assert.True(t, parsed.After(time.Now().Add(-time.Second)))
}

func TestAPIInitiateRejectsUnknownDefinition(t *testing.T) {
	h, _ := newTestRPHandler(t)

	body, _ := json.Marshal(initiateRequest{DefinitionID: "no-such-def", RPID: "rp"})
	rec := postJSON(h.HandleInitiate, body)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, ErrorUnknownDefinition.Code, decodeErrorCode(t, rec))
}

func TestAPIInitiateRejectsMissingFields(t *testing.T) {
	h, _ := newTestRPHandler(t)

	cases := []initiateRequest{
		{},
		{DefinitionID: testDefinitionID},
		{RPID: "scholarbooks"},
	}
	for _, c := range cases {
		body, _ := json.Marshal(c)
		rec := postJSON(h.HandleInitiate, body)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Equal(t, ErrorInvalidRequest.Code, decodeErrorCode(t, rec))
	}
}

func TestAPIInitiateRejectsInvalidJSON(t *testing.T) {
	h, _ := newTestRPHandler(t)
	rec := postJSON(h.HandleInitiate, []byte("not json"))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, ErrorInvalidRequest.Code, decodeErrorCode(t, rec))
}

func TestAPIStatusUnknownTxn(t *testing.T) {
	h, _ := newTestRPHandler(t)
	rec := getStatus(h.HandleStatus, "nope")
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, ErrorUnknownState.Code, decodeErrorCode(t, rec))
}

func TestAPIStatusPending(t *testing.T) {
	h, _ := newTestRPHandler(t)

	body, _ := json.Marshal(initiateRequest{DefinitionID: testDefinitionID, RPID: "rp-1"})
	postJSON(h.HandleInitiate, body)
	var resp initiateResponse
	require.NoError(t, json.Unmarshal(postJSON(h.HandleInitiate, body).Body.Bytes(), &resp))

	rec := getStatus(h.HandleStatus, resp.TxnID)
	require.Equal(t, http.StatusOK, rec.Code)
	var s statusResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &s))
	assert.Equal(t, "PENDING", s.Status)
	assert.Empty(t, s.ResultToken)
}

func TestAPIStatusFailed(t *testing.T) {
	h, store := newTestRPHandler(t)

	rec := postJSON(h.HandleInitiate, mustJSON(t, initiateRequest{
		DefinitionID: testDefinitionID, RPID: "rp",
	}))
	require.Equal(t, http.StatusOK, rec.Code)
	var ir initiateResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &ir))

	// Flip the stored state to FAILED.
	rs := store.m[ir.TxnID]
	rs.Status = StatusFailed
	rs.FailureReason = "untrusted_issuer"

	srec := getStatus(h.HandleStatus, ir.TxnID)
	require.Equal(t, http.StatusOK, srec.Code)
	var sr statusResponse
	require.NoError(t, json.Unmarshal(srec.Body.Bytes(), &sr))
	assert.Equal(t, "FAILED", sr.Status)
	assert.Equal(t, "untrusted_issuer", sr.Error)
}

func TestAPIStatusExpired(t *testing.T) {
	h, store := newTestRPHandler(t)

	rec := postJSON(h.HandleInitiate, mustJSON(t, initiateRequest{
		DefinitionID: testDefinitionID, RPID: "rp",
	}))
	require.Equal(t, http.StatusOK, rec.Code)
	var ir initiateResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &ir))

	store.m[ir.TxnID].ExpiresAt = time.Now().Add(-time.Minute)

	srec := getStatus(h.HandleStatus, ir.TxnID)
	require.Equal(t, http.StatusOK, srec.Code)
	var sr statusResponse
	require.NoError(t, json.Unmarshal(srec.Body.Bytes(), &sr))
	assert.Equal(t, "EXPIRED", sr.Status)
}

// End-to-end happy path: initiate -> wallet response -> status returns a
// COMPLETED payload with a result_token whose claims match the contract.
func TestAPIInitiateAndStatusCompleted(t *testing.T) {
	b := newPIDBuilder(t)
	svc, store := newTestService(t, b)
	issuer := &resultTokenIssuerFake{}
	h := newOpenID4VPHandler(svc, issuer, apiBaseURL, 300*time.Second, 0)

	rec := postJSON(h.HandleInitiate, mustJSON(t, initiateRequest{
		DefinitionID: testDefinitionID, RPID: "scholarbooks",
	}))
	require.Equal(t, http.StatusOK, rec.Code)
	var ir initiateResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &ir))

	rs := store.m[ir.TxnID]
	require.NotNil(t, rs)
	assert.Equal(t, "scholarbooks", rs.RPID)

	// Simulate a successful wallet response.
	presentation := b.build(rs.Nonce, map[string]interface{}{
		"given_name": "Erika", "family_name": "Mustermann",
	})
	body, err := json.Marshal(map[string]interface{}{
		"state":    ir.TxnID,
		"vp_token": map[string]interface{}{credentialID: []string{presentation}},
	})
	require.NoError(t, err)
	jweToken := fabricateResponseJWE(t, &rs.EphemeralKey.PublicKey, body)
	_, err = svc.SubmitResponse(context.Background(), ir.TxnID, []byte(jweToken))
	require.NoError(t, err)

	srec := getStatus(h.HandleStatus, ir.TxnID)
	require.Equal(t, http.StatusOK, srec.Code)
	var sr statusResponse
	require.NoError(t, json.Unmarshal(srec.Body.Bytes(), &sr))
	assert.Equal(t, "COMPLETED", sr.Status)
	assert.NotEmpty(t, sr.ResultToken)

	// The issuer received the RP id from initiate, and the synthesized token
	// carries the contract-required claims. Subject and verified_claims are
	// only available inside the signed result_token; they are not echoed
	// unsigned on the response envelope.
	assert.Equal(t, "scholarbooks", issuer.lastRPID)
	assert.EqualValues(t, 300, issuer.lastValid)
	claims := decodeFakeToken(t, sr.ResultToken)
	assert.Equal(t, "scholarbooks", claims["aud"])
	assert.Equal(t, ir.TxnID, claims["txn"])
	assert.Equal(t, testDefinitionID, claims["definition_id"])
	assert.Contains(t, claims, "subject")
	vc := claims["verified_claims"].(map[string]interface{})
	assert.Equal(t, "Erika", vc["given_name"])
}

func TestAPIRegisterRoutesAddsInitiateAndStatus(t *testing.T) {
	h, _ := newTestRPHandler(t)
	mux := http.NewServeMux()
	registerRoutes(mux, h)

	// POST initiate is routed.
	body := mustJSON(t, initiateRequest{DefinitionID: testDefinitionID, RPID: "rp"})
	req := httptest.NewRequest(http.MethodPost, apiInitiatePath, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var ir initiateResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &ir))

	// GET status/{txn_id} resolves the path value.
	statusReq := httptest.NewRequest(http.MethodGet, "/openid4vp/status/"+ir.TxnID, nil)
	statusRec := httptest.NewRecorder()
	mux.ServeHTTP(statusRec, statusReq)
	require.Equal(t, http.StatusOK, statusRec.Code, statusRec.Body.String())
	var sr statusResponse
	require.NoError(t, json.Unmarshal(statusRec.Body.Bytes(), &sr))
	assert.Equal(t, "PENDING", sr.Status)
}

func TestHandleRequestObjectWriteError(t *testing.T) {
	b := newPIDBuilder(t)
	svc, _ := newTestService(t, b)
	h := newOpenID4VPHandler(svc, nil, "", 0, 0)

	init, err := svc.Initiate(context.Background(), testDefinitionID)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/openid4vp/request?state="+url.QueryEscape(init.State), nil)
	rec := &failingResponseWriter{header: http.Header{}}
	h.HandleRequestObject(rec, req)

	assert.Equal(t, http.StatusOK, rec.status)
	assert.True(t, rec.writeCalled)
}

func TestHandleResponseParseFormError(t *testing.T) {
	b := newPIDBuilder(t)
	svc, _ := newTestService(t, b)
	h := newOpenID4VPHandler(svc, nil, "", 0, 0)

	req := httptest.NewRequest(http.MethodPost, "/openid4vp/response", strings.NewReader("%zz"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.HandleResponse(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, ErrorInvalidRequest.Code, decodeErrorCode(t, rec))
}

func TestAPIInitiateServiceError(t *testing.T) {
	svc := &stubService{initiateErr: errors.New("boom")}
	h := newOpenID4VPHandler(svc, &resultTokenIssuerFake{}, apiBaseURL, 0, 0)

	body, _ := json.Marshal(initiateRequest{DefinitionID: testDefinitionID, RPID: "rp"})
	rec := postJSON(h.HandleInitiate, body)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestAPIStatusLookupServiceError(t *testing.T) {
	svc := &stubService{lookupErr: errors.New("boom")}
	h := newOpenID4VPHandler(svc, &resultTokenIssuerFake{}, apiBaseURL, 0, 0)

	rec := getStatus(h.HandleStatus, "txn")
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestAPIStatusEmptyTxn(t *testing.T) {
	h, _ := newTestRPHandler(t)
	rec := getStatus(h.HandleStatus, "")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, ErrorInvalidRequest.Code, decodeErrorCode(t, rec))
}

func TestAPIStatusCompletedIssuerNotConfigured(t *testing.T) {
	b := newPIDBuilder(t)
	svc, store := newTestService(t, b)
	h := newOpenID4VPHandler(svc, nil, apiBaseURL, 300*time.Second, 0)

	ir := initiateForTest(t, h, store)
	rs := store.m[ir.TxnID]
	rs.Status = StatusCompleted
	rs.Result = &VerifiedPresentation{Subject: "sub"}

	rec := getStatus(h.HandleStatus, ir.TxnID)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestAPIStatusCompletedIssuerError(t *testing.T) {
	b := newPIDBuilder(t)
	svc, store := newTestService(t, b)
	issuer := &resultTokenIssuerFake{errToThrow: errors.New("sign failed")}
	h := newOpenID4VPHandler(svc, issuer, apiBaseURL, 300*time.Second, 0)

	ir := initiateForTest(t, h, store)
	rs := store.m[ir.TxnID]
	rs.Status = StatusCompleted
	rs.Result = &VerifiedPresentation{Subject: "sub"}

	rec := getStatus(h.HandleStatus, ir.TxnID)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestAPIStatusUnknownStatusValue(t *testing.T) {
	b := newPIDBuilder(t)
	svc, store := newTestService(t, b)
	issuer := &resultTokenIssuerFake{}
	h := newOpenID4VPHandler(svc, issuer, apiBaseURL, 300*time.Second, 0)

	ir := initiateForTest(t, h, store)
	store.m[ir.TxnID].Status = Status("BOGUS")

	rec := getStatus(h.HandleStatus, ir.TxnID)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// =============================================================================
// Shared test helpers
// =============================================================================

func initiateForTest(t *testing.T, h *openID4VPHandler, store *fakeStore) initiateResponse {
	t.Helper()
	rec := postJSON(h.HandleInitiate, mustJSON(t, initiateRequest{
		DefinitionID: testDefinitionID, RPID: "rp",
	}))
	require.Equal(t, http.StatusOK, rec.Code)
	var ir initiateResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &ir))
	require.NotNil(t, store.m[ir.TxnID])
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

// stubService implements OpenID4VPServiceInterface to drive handler error
// branches that a real *service does not reach in unit tests.
type stubService struct {
	initiateErr error
	lookupErr   error
}

func (s *stubService) Initiate(context.Context, string) (*Initiation, error) {
	return nil, errors.New("not implemented")
}

func (s *stubService) Result(context.Context, string) (*RequestState, error) {
	return nil, errors.New("not implemented")
}

func (s *stubService) RequestObject(context.Context, string) (string, error) {
	return "", errors.New("not implemented")
}

func (s *stubService) SubmitResponse(context.Context, string, []byte) (*VerifiedPresentation, error) {
	return nil, errors.New("not implemented")
}

func (s *stubService) ResultRedirectURI(string) string { return "" }

func (s *stubService) InitiateForRP(context.Context, string, string) (*Initiation, error) {
	if s.initiateErr != nil {
		return nil, s.initiateErr
	}
	return &Initiation{State: "stub-state"}, nil
}

func (s *stubService) LookupState(context.Context, string) (*RequestState, error) {
	if s.lookupErr != nil {
		return nil, s.lookupErr
	}
	return &RequestState{State: "stub-state", Status: StatusPending}, nil
}

func (s *stubService) Authenticate(context.Context, string) (*authncommon.AuthnResult, *serviceerror.ServiceError) {
	return nil, nil
}

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
