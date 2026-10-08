// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// stubVersionService answers every call with err, or with an empty success when err is nil.
type stubVersionService struct {
	err      *tidcommon.ServiceError
	lastRef  string
	lastApp  ApplyRequest
	forgot   string
	captured CaptureRequest
}

func (s *stubVersionService) Capture(_ context.Context, req CaptureRequest) (*Version, *tidcommon.ServiceError) {
	s.captured = req
	return &Version{Seq: 1}, s.err
}
func (s *stubVersionService) ListVersions(context.Context) ([]Version, *tidcommon.ServiceError) {
	return []Version{}, s.err
}
func (s *stubVersionService) GetVersion(_ context.Context, ref string) (*Version, *tidcommon.ServiceError) {
	s.lastRef = ref
	return &Version{Seq: 1}, s.err
}
func (s *stubVersionService) GetApplied(_ context.Context, id string) (*AppliedVersion, *tidcommon.ServiceError) {
	return &AppliedVersion{GatewayID: id}, s.err
}
func (s *stubVersionService) Diff(_ context.Context, _, ref string) (*Diff, *tidcommon.ServiceError) {
	s.lastRef = ref
	return &Diff{}, s.err
}
func (s *stubVersionService) Apply(_ context.Context, _ string,
	req ApplyRequest) (*ApplyResult, *tidcommon.ServiceError) {
	s.lastApp = req
	return &ApplyResult{}, s.err
}
func (s *stubVersionService) Revert(context.Context, string, RevertRequest) (*ApplyResult, *tidcommon.ServiceError) {
	return &ApplyResult{}, s.err
}
func (s *stubVersionService) Forget(_ context.Context, id string) { s.forgot = id }
func serveVersions(stub *stubVersionService, method, path, body string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	registerVersionRoutes(mux, newVersionHandler(stub))
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(method, path, strings.NewReader(body)))
	return recorder
}

func TestVersionRoutesAnswerEachCall(t *testing.T) {
	stub := &stubVersionService{}
	for _, call := range []struct {
		method, path, body string
		want               int
	}{
		{http.MethodPost, "/configuration-versions", ``, http.StatusCreated},
		{http.MethodPost, "/configuration-versions", `{"note":"n"}`, http.StatusCreated},
		{http.MethodGet, "/configuration-versions", ``, http.StatusOK},
		{http.MethodGet, "/configuration-versions/latest", ``, http.StatusOK},
		{http.MethodGet, "/gateways/gw-1/applied-version", ``, http.StatusOK},
		{http.MethodGet, "/gateways/gw-1/diff?version=2", ``, http.StatusOK},
		{http.MethodPost, "/gateways/gw-1/apply", `{"version":"2","dryRun":true}`, http.StatusOK},
		{http.MethodPost, "/gateways/gw-1/revert", `{}`, http.StatusOK},
		{http.MethodOptions, "/gateways/gw-1/apply", ``, http.StatusNoContent},
	} {
		assert.Equal(t, call.want, serveVersions(stub, call.method, call.path, call.body).Code,
			"%s %s", call.method, call.path)
	}
	assert.Equal(t, ApplyRequest{Version: "2", DryRun: true}, stub.lastApp)
	assert.Equal(t, "n", stub.captured.Note)
}

// Each failure is answered with the status that says what went wrong.
func TestVersionRoutesMapErrorsToStatuses(t *testing.T) {
	for svcErr, want := range map[*tidcommon.ServiceError]int{
		&ErrorVersionNotFound:          http.StatusNotFound,
		&ErrorGatewayNotFound:          http.StatusNotFound,
		&ErrorInvalidVersion:           http.StatusBadRequest,
		&ErrorGatewayUnreachable:       http.StatusBadGateway,
		&ErrorMissingValues:            http.StatusConflict,
		&tidcommon.InternalServerError: http.StatusInternalServerError,
	} {
		stub := &stubVersionService{err: svcErr}
		for _, call := range [][2]string{
			{http.MethodPost, "/configuration-versions"},
			{http.MethodGet, "/configuration-versions"},
			{http.MethodGet, "/configuration-versions/1"},
			{http.MethodGet, "/gateways/gw-1/applied-version"},
			{http.MethodGet, "/gateways/gw-1/diff"},
			{http.MethodPost, "/gateways/gw-1/apply"},
			{http.MethodPost, "/gateways/gw-1/revert"},
		} {
			assert.Equal(t, want, serveVersions(stub, call[0], call[1], "").Code, "%s %s", call[0], call[1])
		}
	}
}

// A body that is not one JSON object is refused before anything runs.
func TestVersionRoutesRefuseABrokenBody(t *testing.T) {
	stub := &stubVersionService{}
	for _, path := range []string{"/configuration-versions", "/gateways/gw-1/apply", "/gateways/gw-1/revert"} {
		assert.Equal(t, http.StatusBadRequest, serveVersions(stub, http.MethodPost, path, `{"x":`).Code, path)
	}
}

// Removing a gateway forgets what was applied to it.
func TestRemovingAGatewayForgetsWhatItHeld(t *testing.T) {
	stub := &stubVersionService{}
	h := newHandler(newTestService(t, &fakeStore{gateways: []Gateway{{ID: "gw-1"}}}))
	h.afterDelete = stub.Forget
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/gateways/gw-1", nil)
	request.SetPathValue("id", "gw-1")

	h.handleDelete(recorder, request)

	assert.Equal(t, http.StatusNoContent, recorder.Code)
	assert.Equal(t, "gw-1", stub.forgot)
}
