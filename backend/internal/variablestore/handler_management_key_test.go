// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package variablestore

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/security"
)

// storeRequest is one call to the variable and secret APIs.
type storeRequest struct {
	method string
	target string
	body   string
	// serviceMethod is what the handler calls when it serves the request.
	serviceMethod string
}

// everyStoreRequest is a call to each API the store serves over HTTP.
var everyStoreRequest = []storeRequest{
	{http.MethodGet, pathVariables, "", "ListVariables"},
	{http.MethodPost, pathVariables, `{"name":"API_URL","value":"x"}`, "CreateVariable"},
	{http.MethodGet, pathVariables + "/API_URL", "", "GetVariable"},
	{http.MethodPut, pathVariables + "/API_URL", `{"value":"x"}`, "UpdateVariable"},
	{http.MethodDelete, pathVariables + "/API_URL", "", "DeleteVariable"},
	{http.MethodGet, pathSecrets, "", "ListSecrets"},
	{http.MethodPost, pathSecrets, `{"name":"API_KEY","value":"x"}`, "CreateSecret"},
	{http.MethodGet, pathSecrets + "/API_KEY", "", "GetSecret"},
	{http.MethodPut, pathSecrets + "/API_KEY", `{"value":"x"}`, "UpdateSecret"},
	{http.MethodDelete, pathSecrets + "/API_KEY", "", "DeleteSecret"},
}

type ManagementAPIKeyTestSuite struct {
	suite.Suite
}

func TestManagementAPIKeySuite(t *testing.T) {
	suite.Run(t, new(ManagementAPIKeyTestSuite))
}

// routes registers the store's routes on a handler that does or does not require the key.
func (s *ManagementAPIKeyTestSuite) routes(service ServiceInterface, keyOnly bool) *http.ServeMux {
	mux := http.NewServeMux()
	registerRoutes(mux, newHandler(service, keyOnly))
	return mux
}

// answering makes every service method answer not found, so a served request is told apart from a
// refused one by its status.
func answering(service *ServiceInterfaceMock) {
	for _, method := range []string{"GetVariable", "CreateVariable", "ListVariables", "GetSecret",
		"CreateSecret", "ListSecrets"} {
		service.On(method, mock.Anything, mock.Anything).Return(nil, &ErrorNotFound).Maybe()
	}
	for _, method := range []string{"UpdateVariable", "UpdateSecret"} {
		service.On(method, mock.Anything, mock.Anything, mock.Anything).Return(nil, false, &ErrorNotFound).Maybe()
	}
	for _, method := range []string{"DeleteVariable", "DeleteSecret"} {
		service.On(method, mock.Anything, mock.Anything).Return(&ErrorNotFound).Maybe()
	}
}

// called reports whether the service was asked to run the method.
func called(service *ServiceInterfaceMock, method string) bool {
	for _, call := range service.Calls {
		if call.Method == method {
			return true
		}
	}
	return false
}

func send(mux *http.ServeMux, call storeRequest, caller *security.SecurityContext) *httptest.ResponseRecorder {
	request := httptest.NewRequest(call.method, call.target, strings.NewReader(call.body))
	request.Header.Set("Content-Type", "application/json")
	if caller != nil {
		request = request.WithContext(security.WithSecurityContextTest(request.Context(), caller))
	}
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	return recorder
}

// rootPermission is the permission over every management API.
const rootPermission = "system"

// rootToken is a user's token with the root permission, the most an administrator can hold.
func rootToken() *security.SecurityContext {
	return security.NewSecurityContextForTest("admin", "ou-1", "token",
		[]string{rootPermission}, nil)
}

// managementAPIKey is a caller the management API key authenticated.
func managementAPIKey() *security.SecurityContext {
	return security.NewSecurityContextForTest("management-api-key", "", "",
		[]string{rootPermission}, nil)
}

// Where the key is required, a token is refused on every API, however privileged, and the store is
// never reached.
func (s *ManagementAPIKeyTestSuite) TestATokenIsRefusedWhereTheKeyIsRequired() {
	for _, call := range everyStoreRequest {
		service := NewServiceInterfaceMock(s.T())
		response := send(s.routes(service, true), call, rootToken())

		s.Equal(http.StatusForbidden, response.Code, "%s %s", call.method, call.target)
		s.Contains(response.Body.String(), ErrorManagementAPIKeyRequired.Code, "%s %s", call.method, call.target)
		service.AssertNotCalled(s.T(), call.serviceMethod)
	}
}

// Where the key is required, a request no one authenticated is refused too.
func (s *ManagementAPIKeyTestSuite) TestAnUnauthenticatedRequestIsRefusedWhereTheKeyIsRequired() {
	for _, call := range everyStoreRequest {
		service := NewServiceInterfaceMock(s.T())
		response := send(s.routes(service, true), call, nil)

		s.Equal(http.StatusForbidden, response.Code, "%s %s", call.method, call.target)
	}
}

// Where the key is required, the key is served on every API.
func (s *ManagementAPIKeyTestSuite) TestTheKeyIsServedWhereItIsRequired() {
	for _, call := range everyStoreRequest {
		service := NewServiceInterfaceMock(s.T())
		answering(service)
		response := send(s.routes(service, true), call, managementAPIKey())

		s.Equal(http.StatusNotFound, response.Code, "%s %s", call.method, call.target)
		s.True(called(service, call.serviceMethod), "%s %s did not reach the store", call.method, call.target)
	}
}

// Where no key is configured, an administrator's token keeps managing the store.
func (s *ManagementAPIKeyTestSuite) TestATokenIsServedWhereNoKeyIsRequired() {
	for _, call := range everyStoreRequest {
		service := NewServiceInterfaceMock(s.T())
		answering(service)
		response := send(s.routes(service, false), call, rootToken())

		s.Equal(http.StatusNotFound, response.Code, "%s %s", call.method, call.target)
	}
}

// A preflight carries no credentials, so it is answered either way.
func (s *ManagementAPIKeyTestSuite) TestAPreflightIsAnswered() {
	mux := s.routes(NewServiceInterfaceMock(s.T()), true)
	for _, target := range []string{pathVariables, pathVariables + "/API_URL", pathSecrets, pathSecrets + "/API_KEY"} {
		response := send(mux, storeRequest{method: http.MethodOptions, target: target}, nil)

		s.Equal(http.StatusNoContent, response.Code, target)
	}
}
