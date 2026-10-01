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
const testGatewayID = "gw-1"

type stubService struct {
	ServiceInterface
	registered *RegisterRequest
	result     *Registration
	rotatedID  string
	rotatedKey string
	listed     []Gateway
	fetchedID  string
	deletedID  string
	svcErr     *tidcommon.ServiceError
}

func (s *stubService) Register(_ context.Context,
	req RegisterRequest) (*Registration, *tidcommon.ServiceError) {
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
		"a second object": `{"name":"dev","baseUrl":"https://dp"} {}`,
		"trailing junk":   `{"name":"dev","baseUrl":"https://dp"} not-json`,
		"two objects":     `{"name":"a","baseUrl":"https://dp"}{"name":"b"}`,
	} {
		svc := &stubService{result: &Registration{Gateway: Gateway{ID: testGatewayID}}}
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
	svc := &stubService{result: &Registration{Gateway: Gateway{ID: testGatewayID, Name: "dev"}}}

	response := postRegistration(t, svc,
		`{"name":"dev","baseUrl":"https://dp"}`)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", response.Code, response.Body.String())
	}
	if svc.registered == nil || svc.registered.BaseURL != "https://dp" {
		t.Fatalf("the registration did not reach the service intact: %+v", svc.registered)
	}
}

// Trailing whitespace and a newline are what an editor or a curl heredoc leaves behind, and are not
// surplus data.
func TestRegistrationAcceptsTrailingWhitespace(t *testing.T) {
	svc := &stubService{result: &Registration{Gateway: Gateway{ID: testGatewayID}}}

	response := postRegistration(t, svc,
		"{\"name\":\"dev\",\"baseUrl\":\"https://dp\"}\n  \n")

	if response.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", response.Code, response.Body.String())
	}
}

// An already-registered gateway is a conflict, like a duplicate name, not a bad request.
func TestRegistrationConflictIsReportedAsConflict(t *testing.T) {
	svc := &stubService{svcErr: &ErrorGatewayAlreadyRegistered}

	response := postRegistration(t, svc,
		`{"name":"dev","baseUrl":"https://dp"}`)

	if response.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", response.Code)
	}
}

func (s *stubService) RotateKey(_ context.Context, id string) (string, *tidcommon.ServiceError) {
	s.rotatedID = id
	if s.svcErr != nil {
		return "", s.svcErr
	}
	return s.rotatedKey, nil
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
	if s.result == nil {
		return nil, nil
	}
	return &s.result.Gateway, nil
}

func (s *stubService) Delete(_ context.Context, id string) *tidcommon.ServiceError {
	s.deletedID = id
	return s.svcErr
}

func TestListReturnsTheGateways(t *testing.T) {
	svc := &stubService{listed: []Gateway{
		{ID: testGatewayID, Name: "production"},
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
	svc := &stubService{result: &Registration{Gateway: Gateway{ID: testGatewayID, Name: "production"}}}

	request := httptest.NewRequest(http.MethodGet, "/gateways/gw-1", nil)
	request.SetPathValue("id", testGatewayID)
	recorder := httptest.NewRecorder()
	newHandler(svc).handleGet(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if svc.fetchedID != testGatewayID {
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
	request.SetPathValue("id", testGatewayID)
	recorder := httptest.NewRecorder()
	newHandler(svc).handleDelete(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", recorder.Code)
	}
	if svc.deletedID != testGatewayID {
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
	svc := &stubService{listed: []Gateway{}, result: &Registration{Gateway: Gateway{ID: testGatewayID}}}
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

// A delete the service refuses is reported with its own status, not swallowed into 204.
func TestDeleteReportsARefusal(t *testing.T) {
	for name, tc := range map[string]struct {
		svcErr     *tidcommon.ServiceError
		wantStatus int
	}{
		"not found":  {&ErrorGatewayNotFound, http.StatusNotFound},
		"invalid id": {&ErrorInvalidGatewayID, http.StatusBadRequest},
		"store down": {&tidcommon.InternalServerError, http.StatusInternalServerError},
	} {
		t.Run(name, func(t *testing.T) {
			svc := &stubService{svcErr: tc.svcErr}

			request := httptest.NewRequest(http.MethodDelete, "/gateways/gw-1", nil)
			request.SetPathValue("id", testGatewayID)
			recorder := httptest.NewRecorder()
			newHandler(svc).handleDelete(recorder, request)

			if recorder.Code != tc.wantStatus {
				t.Fatalf("expected %d, got %d", tc.wantStatus, recorder.Code)
			}
		})
	}
}

// A body that decodes but fails validation is a bad request, and the service never sees it.
func TestRegistrationRefusesAnUnreadableBody(t *testing.T) {
	for name, body := range map[string]string{
		"not json":      `this is not json`,
		"empty body":    ``,
		"a bare array":  `[]`,
		"a bare string": `"just a string"`,
	} {
		t.Run(name, func(t *testing.T) {
			svc := &stubService{result: &Registration{Gateway: Gateway{ID: testGatewayID}}}

			response := postRegistration(t, svc, body)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", response.Code)
			}
			if svc.registered != nil {
				t.Fatal("an unreadable body reached the service")
			}
		})
	}
}

// An update carries only the fields it changes, and the handler passes them through as given.
func TestHandleUpdatePassesThePartialThrough(t *testing.T) {
	svc := &updateStub{result: &Gateway{ID: testGatewayID, Name: "prod"}}
	request := httptest.NewRequest(http.MethodPut, "/gateways/gw-1",
		strings.NewReader(`{"name":"prod"}`))
	request.SetPathValue("id", testGatewayID)
	recorder := httptest.NewRecorder()
	newHandler(svc).handleUpdate(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if svc.updatedID != testGatewayID {
		t.Fatalf("the id did not reach the service: %q", svc.updatedID)
	}
	if svc.req.Name == nil || *svc.req.Name != "prod" {
		t.Fatal("the name did not reach the service")
	}
	// What the body omits must arrive as omitted, not as an empty string.
	if svc.req.BaseURL != nil || svc.req.CACertificate != nil {
		t.Fatal("fields the body did not name must stay nil")
	}
}

// updateStub records what an update was asked to change.
type updateStub struct {
	ServiceInterface
	updatedID string
	req       UpdateRequest
	result    *Gateway
	svcErr    *tidcommon.ServiceError
}

func (u *updateStub) Update(_ context.Context, id string,
	req UpdateRequest) (*Gateway, *tidcommon.ServiceError) {
	u.updatedID = id
	u.req = req
	if u.svcErr != nil {
		return nil, u.svcErr
	}
	return u.result, nil
}
