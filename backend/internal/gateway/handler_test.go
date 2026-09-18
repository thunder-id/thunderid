// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// stubService answers whatever a handler test needs, and records what it was asked.
type stubService struct {
	ServiceInterface
	registered *RegisterRequest
	result     *Gateway
	listed     []Gateway
	fetchedID  string
	deletedID  string
	svcErr     *tidcommon.ServiceError
}

func (s *stubService) Register(_ context.Context, req RegisterRequest) (*Gateway, *tidcommon.ServiceError) {
	s.registered = &req
	if s.svcErr != nil {
		return nil, s.svcErr
	}
	return s.result, nil
}

func postRegistration(t *testing.T, svc ServiceInterface, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/gateways", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	newHandler(svc).handleRegister(recorder, request)
	return recorder
}

// A body is one JSON object and nothing else. Anything after it means the caller sent something
// this server did not understand, and accepting it would report that it had.
func TestRegistrationRefusesAnythingAfterTheObject(t *testing.T) {
	for name, body := range map[string]string{
		"a second object": `{"name":"dev","dataPlaneId":"dp-1","baseUrl":"https://dp","key":"k"} {}`,
		"trailing junk":   `{"name":"dev","dataPlaneId":"dp-1","baseUrl":"https://dp","key":"k"} not-json`,
		"two objects":     `{"name":"a","dataPlaneId":"dp-1","baseUrl":"https://dp","key":"k"}{"name":"b"}`,
	} {
		svc := &stubService{result: &Gateway{ID: "gw-1"}}
		response := postRegistration(t, svc, body)

		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d", name, response.Code)
		}
		if svc.registered != nil {
			t.Fatalf("%s: the request reached the service despite the surplus data", name)
		}
	}
}

func TestRegistrationAcceptsOneObject(t *testing.T) {
	svc := &stubService{result: &Gateway{ID: "gw-1", Name: "dev"}}

	response := postRegistration(t, svc,
		`{"name":"dev","dataPlaneId":"dp-1","baseUrl":"https://dp","key":"k"}`)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", response.Code, response.Body.String())
	}
	if svc.registered == nil || svc.registered.DataPlaneID != "dp-1" {
		t.Fatalf("the registration did not reach the service intact: %+v", svc.registered)
	}
}

// Trailing whitespace and a newline are what an editor or a curl heredoc leaves behind, and are not
// surplus data.
func TestRegistrationAcceptsTrailingWhitespace(t *testing.T) {
	svc := &stubService{result: &Gateway{ID: "gw-1"}}

	response := postRegistration(t, svc,
		"{\"name\":\"dev\",\"dataPlaneId\":\"dp-1\",\"baseUrl\":\"https://dp\",\"key\":\"k\"}\n  \n")

	if response.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", response.Code, response.Body.String())
	}
}

// An already-registered data plane is a conflict, like a duplicate name, not a bad request.
func TestRegistrationConflictIsReportedAsConflict(t *testing.T) {
	svc := &stubService{svcErr: &ErrorDataPlaneAlreadyRegistered}

	response := postRegistration(t, svc,
		`{"name":"dev","dataPlaneId":"dp-1","baseUrl":"https://dp","key":"k"}`)

	if response.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", response.Code)
	}
}

func (s *stubService) List(_ context.Context) ([]Gateway, *tidcommon.ServiceError) {
	if s.svcErr != nil {
		return nil, s.svcErr
	}
	return s.listed, nil
}

func (s *stubService) Get(_ context.Context, id string) (*Gateway, *tidcommon.ServiceError) {
	s.fetchedID = id
	if s.svcErr != nil {
		return nil, s.svcErr
	}
	return s.result, nil
}

func (s *stubService) Delete(_ context.Context, id string) *tidcommon.ServiceError {
	s.deletedID = id
	return s.svcErr
}

func TestListReturnsTheGateways(t *testing.T) {
	svc := &stubService{listed: []Gateway{
		{ID: "gw-1", Name: "production", DataPlaneID: "dp-1"},
	}}

	response := serve(t, svc, http.MethodGet, "/gateways", newHandler(svc).handleList)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "production") {
		t.Fatalf("the listing did not carry the gateway: %s", response.Body.String())
	}
	// Whatever a listing carries, it is not the credential.
	if strings.Contains(response.Body.String(), `"key"`) {
		t.Fatalf("the listing returned a key field: %s", response.Body.String())
	}
}

func TestGetReturnsOneGateway(t *testing.T) {
	svc := &stubService{result: &Gateway{ID: "gw-1", Name: "production"}}

	request := httptest.NewRequest(http.MethodGet, "/gateways/gw-1", nil)
	request.SetPathValue("id", "gw-1")
	recorder := httptest.NewRecorder()
	newHandler(svc).handleGet(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if svc.fetchedID != "gw-1" {
		t.Fatalf("the handler asked for %q", svc.fetchedID)
	}
}

func TestGetReportsAMissingGatewayAsNotFound(t *testing.T) {
	svc := &stubService{svcErr: &ErrorGatewayNotFound}

	request := httptest.NewRequest(http.MethodGet, "/gateways/absent", nil)
	request.SetPathValue("id", "absent")
	recorder := httptest.NewRecorder()
	newHandler(svc).handleGet(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", recorder.Code)
	}
}

func TestDeleteAnswersNoContent(t *testing.T) {
	svc := &stubService{}

	request := httptest.NewRequest(http.MethodDelete, "/gateways/gw-1", nil)
	request.SetPathValue("id", "gw-1")
	recorder := httptest.NewRecorder()
	newHandler(svc).handleDelete(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", recorder.Code)
	}
	if svc.deletedID != "gw-1" {
		t.Fatalf("the handler deleted %q", svc.deletedID)
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("204 carried a body: %s", recorder.Body.String())
	}
}

// A store failure is an internal error, not a bad request.
func TestAStoreFailureIsReportedAsInternal(t *testing.T) {
	svc := &stubService{svcErr: &tidcommon.InternalServerError}

	response := serve(t, svc, http.MethodGet, "/gateways", newHandler(svc).handleList)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", response.Code)
	}
}

// KeyConfigured reports whether a key is held, which is all a read of one may disclose.
func TestKeyConfigured(t *testing.T) {
	if (&Gateway{}).KeyConfigured() {
		t.Fatal("a gateway with no key reported one")
	}
	if !(&Gateway{Key: "sealed"}).KeyConfigured() {
		t.Fatal("a gateway holding a key reported none")
	}
}

func serve(t *testing.T, _ ServiceInterface, method, target string,
	handler func(http.ResponseWriter, *http.Request)) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(method, target, nil)
	recorder := httptest.NewRecorder()
	handler(recorder, request)
	return recorder
}

// The routes are registered where the API says they are, so a caller reaching /gateways is answered
// rather than 404ing on a pattern typo.
func TestRegisteredRoutesAnswer(t *testing.T) {
	mux := http.NewServeMux()
	svc := &stubService{listed: []Gateway{}, result: &Gateway{ID: "gw-1"}}
	registerRoutes(mux, newHandler(svc))

	for _, tc := range []struct {
		method, target string
		wantStatus     int
	}{
		{http.MethodGet, "/gateways", http.StatusOK},
		{http.MethodGet, "/gateways/gw-1", http.StatusOK},
		{http.MethodDelete, "/gateways/gw-1", http.StatusNoContent},
		{http.MethodOptions, "/gateways", http.StatusNoContent},
		{http.MethodOptions, "/gateways/gw-1", http.StatusNoContent},
	} {
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, httptest.NewRequest(tc.method, tc.target, nil))

		if recorder.Code != tc.wantStatus {
			t.Errorf("%s %s answered %d, want %d", tc.method, tc.target, recorder.Code, tc.wantStatus)
		}
	}
}
