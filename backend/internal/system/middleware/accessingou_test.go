// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	syscontext "github.com/thunder-id/thunderid/internal/system/context"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/ouprovidermock"
)

const (
	accessingOU    = "m2m-child-a"
	prefixedRoute  = "POST /ou/{" + PathParamOUID + "}/oauth2/token"
	unprefixedPath = "/oauth2/token"
)

// AccessingOUMiddlewareTestSuite covers the middleware that turns an /ou/{ouId}/... request into one
// carrying the organization unit it is for. The refusal's shape belongs to whoever mounts it, so
// what is asserted here is that a refusal happened and the handler did not run.
type AccessingOUMiddlewareTestSuite struct {
	suite.Suite
}

func TestAccessingOUMiddlewareTestSuite(t *testing.T) {
	suite.Run(t, new(AccessingOUMiddlewareTestSuite))
}

// serve runs one request through the middleware, reporting what reached the handler and, when
// nothing did, which of the two answers the middleware wrote instead.
func (s *AccessingOUMiddlewareTestSuite) serve(
	ouService providers.OrganizationUnitProvider, route, target string,
) (outcome serveOutcome) {
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		outcome.reached = true
		outcome.seen = syscontext.GetAccessingOUID(r.Context())
	})

	mux := http.NewServeMux()
	mux.Handle(route, AccessingOUMiddleware(ouService, testRefusal, testFailure)(next))

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, target, nil))

	outcome.status = recorder.Code
	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err == nil {
		outcome.errorCode = body["error"]
	}
	return outcome
}

// The two answers the tests mount the middleware with. They are deliberately unlike each other, so
// that reading the response alone says which one the middleware chose.
var (
	testRefusal = AccessingOUResponse{
		Code:        "refused",
		Description: "the request may not act for that organization unit",
		StatusCode:  http.StatusBadRequest,
	}
	testFailure = AccessingOUResponse{
		Code:        "lookup_failed",
		Description: "the organization unit could not be resolved",
		StatusCode:  http.StatusInternalServerError,
	}
)

// serveOutcome records which of the three endings one request reached.
type serveOutcome struct {
	reached bool
	seen    string
	// status and errorCode come back from the response the middleware wrote, which is how a test
	// tells a refusal from a failure.
	status    int
	errorCode string
}

// refused reports whether the middleware answered with the refusal it was given.
func (o serveOutcome) refused() bool {
	return o.errorCode == testRefusal.Code && o.status == testRefusal.StatusCode
}

// failed reports whether the middleware answered with the failure it was given.
func (o serveOutcome) failed() bool {
	return o.errorCode == testFailure.Code && o.status == testFailure.StatusCode
}

// A route carrying no organization unit must not pay for the one that does: no lookup is made, and
// nothing is recorded. This is what lets one handler serve both forms of an endpoint.
func (s *AccessingOUMiddlewareTestSuite) TestNoPrefixPassesThroughUntouched() {
	ouService := ouprovidermock.NewOrganizationUnitProviderMock(s.T())

	got := s.serve(ouService, "POST "+unprefixedPath, unprefixedPath)

	s.True(got.reached, "the handler runs")
	s.Empty(got.seen, "and is told of no accessing organization unit")
	s.False(got.refused())
	ouService.AssertNotCalled(s.T(), "GetOrganizationUnit", mock.Anything, mock.Anything)
}

// A resolvable organization unit reaches the handler on the context, which is where admission and
// claim resolution both read it from.
func (s *AccessingOUMiddlewareTestSuite) TestResolvableOUIsRecordedOnTheContext() {
	ouService := ouprovidermock.NewOrganizationUnitProviderMock(s.T())
	ouService.EXPECT().GetOrganizationUnit(mock.Anything, accessingOU).
		Return(providers.OrganizationUnit{ID: accessingOU}, nil).Once()

	got := s.serve(ouService, prefixedRoute, "/ou/"+accessingOU+"/oauth2/token")

	s.True(got.reached)
	s.Equal(accessingOU, got.seen)
	s.False(got.refused())
}

// An id that names nothing is refused before the handler runs. A policy check alone could not do
// this: a deployment-wide policy matches every organization unit without consulting the tree, so an
// unknown id would satisfy it and fail much later as an internal error.
func (s *AccessingOUMiddlewareTestSuite) TestUnresolvableOUIsRefusedBeforeTheHandler() {
	ouService := ouprovidermock.NewOrganizationUnitProviderMock(s.T())
	ouService.EXPECT().GetOrganizationUnit(mock.Anything, "no-such-ou").
		Return(providers.OrganizationUnit{}, &tidcommon.ErrorUnauthorized).Once()

	got := s.serve(ouService, prefixedRoute, "/ou/no-such-ou/oauth2/token")

	s.False(got.reached, "the request never reaches the handler")
	s.True(got.refused(), "and the refusal the endpoint supplied is what the caller is told")
	s.False(got.failed(), "a missing organization unit is the caller's problem, not the server's")
}

// A lookup that fails because the store is unreachable says nothing about the organization unit.
// Reporting it as a refusal would tell the caller they named something they may not act for, when
// in fact the deployment is broken, and would hide the fault from whoever watches for server
// errors. It leaks nothing to separate the two: a refusal still reads the same whether the
// organization unit is absent or merely out of reach.
func (s *AccessingOUMiddlewareTestSuite) TestAFailedLookupIsNotARefusal() {
	ouService := ouprovidermock.NewOrganizationUnitProviderMock(s.T())
	ouService.EXPECT().GetOrganizationUnit(mock.Anything, accessingOU).
		Return(providers.OrganizationUnit{}, &tidcommon.InternalServerError).Once()

	got := s.serve(ouService, prefixedRoute, "/ou/"+accessingOU+"/oauth2/token")

	s.False(got.reached, "the request never reaches the handler either way")
	s.True(got.failed(), "a broken store was reported as a bad request")
	s.False(got.refused())
}

// Without a resolver there is no way to establish that the organization unit exists, and admitting
// it would let an unknown id through to a blanket policy. Failing closed is the only safe answer.
func (s *AccessingOUMiddlewareTestSuite) TestMissingResolverIsAServerFault() {
	got := s.serve(nil, prefixedRoute, "/ou/"+accessingOU+"/oauth2/token")

	s.False(got.reached)
	s.True(got.failed(), "a deployment with no resolver is a server fault, not a bad request")
	s.False(got.refused())
}
