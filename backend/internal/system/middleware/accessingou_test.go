// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package middleware

import (
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

// serve runs one request through the middleware, reporting what reached the handler and whether the
// caller's own refusal was invoked instead.
func (s *AccessingOUMiddlewareTestSuite) serve(
	ouService providers.OrganizationUnitProvider, route, target string,
) (reached bool, seen string, refusedID string) {
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		reached = true
		seen = syscontext.GetAccessingOUID(r.Context())
	})
	refuse := func(w http.ResponseWriter, _ *http.Request, ouID string) {
		refusedID = ouID
		w.WriteHeader(http.StatusBadRequest)
	}

	mux := http.NewServeMux()
	mux.Handle(route, AccessingOUMiddleware(ouService, refuse)(next))
	mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, target, nil))
	return reached, seen, refusedID
}

// A route carrying no organization unit must not pay for the one that does: no lookup is made, and
// nothing is recorded. This is what lets one handler serve both forms of an endpoint.
func (s *AccessingOUMiddlewareTestSuite) TestNoPrefixPassesThroughUntouched() {
	ouService := ouprovidermock.NewOrganizationUnitProviderMock(s.T())

	reached, seen, refused := s.serve(ouService, "POST "+unprefixedPath, unprefixedPath)

	s.True(reached, "the handler runs")
	s.Empty(seen, "and is told of no accessing organization unit")
	s.Empty(refused)
	ouService.AssertNotCalled(s.T(), "GetOrganizationUnit", mock.Anything, mock.Anything)
}

// A resolvable organization unit reaches the handler on the context, which is where admission and
// claim resolution both read it from.
func (s *AccessingOUMiddlewareTestSuite) TestResolvableOUIsRecordedOnTheContext() {
	ouService := ouprovidermock.NewOrganizationUnitProviderMock(s.T())
	ouService.EXPECT().GetOrganizationUnit(mock.Anything, accessingOU).
		Return(providers.OrganizationUnit{ID: accessingOU}, nil).Once()

	reached, seen, refused := s.serve(ouService, prefixedRoute, "/ou/"+accessingOU+"/oauth2/token")

	s.True(reached)
	s.Equal(accessingOU, seen)
	s.Empty(refused)
}

// An id that names nothing is refused before the handler runs. A policy check alone could not do
// this: a deployment-wide policy matches every organization unit without consulting the tree, so an
// unknown id would satisfy it and fail much later as an internal error.
func (s *AccessingOUMiddlewareTestSuite) TestUnresolvableOUIsRefusedBeforeTheHandler() {
	ouService := ouprovidermock.NewOrganizationUnitProviderMock(s.T())
	ouService.EXPECT().GetOrganizationUnit(mock.Anything, "no-such-ou").
		Return(providers.OrganizationUnit{}, &tidcommon.ErrorUnauthorized).Once()

	reached, _, refused := s.serve(ouService, prefixedRoute, "/ou/no-such-ou/oauth2/token")

	s.False(reached, "the request never reaches the handler")
	s.Equal("no-such-ou", refused, "and the refusal is told which id was asked for")
}

// Without a resolver there is no way to establish that the organization unit exists, and admitting
// it would let an unknown id through to a blanket policy. Failing closed is the only safe answer.
func (s *AccessingOUMiddlewareTestSuite) TestMissingResolverRefuses() {
	reached, _, refused := s.serve(nil, prefixedRoute, "/ou/"+accessingOU+"/oauth2/token")

	s.False(reached)
	s.Equal(accessingOU, refused)
}
