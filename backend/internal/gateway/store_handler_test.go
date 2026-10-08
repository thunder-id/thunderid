// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// serveStore serves one call to the store routes of a service administering a stand-in gateway that
// answers every call with status and body.
func serveStore(t *testing.T, status int, body, method, path, payload string) (*httptest.ResponseRecorder,
	*[]storeCall) {
	t.Helper()
	svc, calls := newStoreFixture(t, status, body)
	mux := http.NewServeMux()
	registerStoreRoutes(mux, newStoreHandler(svc))
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(method, path, strings.NewReader(payload)))
	return recorder, calls
}

// Each route reaches its own call on the gateway's store and answers with what the gateway returned.
func TestStoreRoutesReachTheGatewaysStore(t *testing.T) {
	for _, call := range []struct {
		status                       int
		method, path, payload        string
		wantStatus                   int
		wantMethod, wantPath, wantQS string
	}{
		{http.StatusOK, http.MethodGet, "/gateways/gw-1/variables?names=A,B&limit=5", ``, http.StatusOK,
			http.MethodGet, "/variables", "limit=5&names=A%2CB"},
		{http.StatusCreated, http.MethodPost, "/gateways/gw-1/variables", `{"name":"A","value":"1"}`,
			http.StatusCreated, http.MethodPost, "/variables", ""},
		{http.StatusOK, http.MethodGet, "/gateways/gw-1/variables/A", ``, http.StatusOK,
			http.MethodGet, "/variables/A", ""},
		{http.StatusCreated, http.MethodPut, "/gateways/gw-1/variables/A", `{"value":"2"}`, http.StatusCreated,
			http.MethodPut, "/variables/A", ""},
		{http.StatusOK, http.MethodPut, "/gateways/gw-1/secrets/S", `{"value":"2"}`, http.StatusOK,
			http.MethodPut, "/secrets/S", ""},
		{http.StatusNoContent, http.MethodDelete, "/gateways/gw-1/secrets/S", ``, http.StatusNoContent,
			http.MethodDelete, "/secrets/S", ""},
		{http.StatusOK, http.MethodGet, "/gateways/gw-1/secrets", ``, http.StatusOK,
			http.MethodGet, "/secrets", ""},
	} {
		recorder, calls := serveStore(t, call.status, `{"name":"A"}`, call.method, call.path, call.payload)
		assert.Equal(t, call.wantStatus, recorder.Code, "%s %s", call.method, call.path)
		if assert.Len(t, *calls, 1, "%s %s", call.method, call.path) {
			sent := (*calls)[0]
			assert.Equal(t, call.wantMethod+" "+call.wantPath+"?"+call.wantQS,
				sent.method+" "+sent.path+"?"+sent.query)
		}
	}
}

// A call the gateway refuses is answered with the gateway's status and error, unchanged.
func TestStoreRoutesAnswerWithTheGatewaysRefusal(t *testing.T) {
	recorder, _ := serveStore(t, http.StatusConflict, `{"code":"VAR-1005"}`, http.MethodPost,
		"/gateways/gw-1/secrets", `{"name":"S","value":"v"}`)

	assert.Equal(t, http.StatusConflict, recorder.Code)
	assert.JSONEq(t, `{"code":"VAR-1005"}`, recorder.Body.String())
	assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
}

// A call to a gateway this plane cannot reach is answered with the reason.
func TestStoreRoutesReportAnUnreachableGateway(t *testing.T) {
	recorder, _ := serveStore(t, http.StatusBadGateway, `<html></html>`, http.MethodGet, "/gateways/gw-1/secrets", ``)

	assert.Equal(t, http.StatusBadGateway, recorder.Code)
}

// A body that is not one value, or larger than one, is refused before it reaches the gateway.
func TestStoreRoutesRefuseABadBody(t *testing.T) {
	oversized := `{"value":"` + strings.Repeat("x", maxStoreBody) + `"}`
	for _, call := range []struct{ method, path, payload string }{
		{http.MethodPost, "/gateways/gw-1/secrets", `{"name":`},
		{http.MethodPost, "/gateways/gw-1/secrets", `{"name":"S"} {}`},
		{http.MethodPost, "/gateways/gw-1/variables", oversized},
		{http.MethodPut, "/gateways/gw-1/variables/A", `{"value":`},
		{http.MethodPut, "/gateways/gw-1/secrets/S", oversized},
	} {
		recorder, calls := serveStore(t, http.StatusCreated, `{}`, call.method, call.path, call.payload)
		assert.Equal(t, http.StatusBadRequest, recorder.Code, "%s %s", call.method, call.path)
		assert.Empty(t, *calls)
	}
}
