// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

func serveMasters(stub *stubVersionService, method, path string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	registerMasterRoutes(mux, newVersionHandler(stub))
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(method, path, nil))
	return recorder
}

type MasterHandlerTestSuite struct {
	suite.Suite
}

func TestMasterHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(MasterHandlerTestSuite))
}

// Each master route answers, and a resource's history is asked for by its key.
func (s *MasterHandlerTestSuite) TestMasterRoutesAnswerEachCall() {
	stub := &stubVersionService{}
	for _, call := range []struct {
		method, path string
		want         int
	}{
		{http.MethodGet, "/gateways/gw-1/master", http.StatusOK},
		{http.MethodGet, "/gateways/gw-1/master/history", http.StatusOK},
		{http.MethodGet, "/gateways/gw-1/master/resources/application/app-1", http.StatusOK},
		{http.MethodOptions, "/gateways/gw-1/master", http.StatusNoContent},
		{http.MethodOptions, "/gateways/gw-1/master/resources/application/app-1", http.StatusNoContent},
	} {
		s.Equal(call.want, serveMasters(stub, call.method, call.path).Code, "%s %s", call.method, call.path)
	}
	s.Equal("application/app-1", stub.lastRef)
}

// A gateway or a resource that is not found is answered as such, and any other failure with its status.
func (s *MasterHandlerTestSuite) TestMasterRoutesMapErrorsToStatuses() {
	for svcErr, want := range map[*tidcommon.ServiceError]int{
		&ErrorGatewayNotFound:          http.StatusNotFound,
		&ErrorResourceNotDeployed:      http.StatusNotFound,
		&tidcommon.InternalServerError: http.StatusInternalServerError,
	} {
		stub := &stubVersionService{err: svcErr}
		for _, path := range []string{"/gateways/gw-1/master", "/gateways/gw-1/master/history",
			"/gateways/gw-1/master/resources/application/app-1"} {
			s.Equal(want, serveMasters(stub, http.MethodGet, path).Code, path)
		}
	}
}
