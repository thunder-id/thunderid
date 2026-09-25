// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	syscontext "github.com/thunder-id/thunderid/internal/system/context"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const accessingOU = "ou-customer"

// ouStub resolves one known organization unit and records whether it was asked at all.
type ouStub struct {
	providers.OrganizationUnitProvider
	known  bool
	called bool
}

func (s *ouStub) GetOrganizationUnit(
	_ context.Context, id string,
) (providers.OrganizationUnit, *tidcommon.ServiceError) {
	s.called = true
	if !s.known {
		return providers.OrganizationUnit{},
			&tidcommon.ServiceError{Code: "OU-1003", Type: tidcommon.ClientErrorType}
	}
	return providers.OrganizationUnit{ID: id}, nil
}

type AccessingOUMiddlewareTestSuite struct {
	suite.Suite
}

func TestAccessingOUMiddlewareTestSuite(t *testing.T) {
	suite.Run(t, new(AccessingOUMiddlewareTestSuite))
}

// serve runs the middleware over a request, reporting whether the handler was reached and what it
// saw on the context.
func (suite *AccessingOUMiddlewareTestSuite) serve(
	ou *ouStub, pathValue string,
) (recorder *httptest.ResponseRecorder, reached bool, seen string) {
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		reached = true
		seen = syscontext.GetAccessingOUID(r.Context())
	})

	req := httptest.NewRequest(http.MethodPost, "/oauth2/token", nil)
	if pathValue != "" {
		req.SetPathValue(constants.PathParamOUID, pathValue)
	}
	recorder = httptest.NewRecorder()
	accessingOUMiddleware(ou)(next).ServeHTTP(recorder, req)
	return recorder, reached, seen
}

// The bare endpoint must be completely untouched, including making no organization unit lookup: it
// is the path every existing client already uses.
func (suite *AccessingOUMiddlewareTestSuite) TestNoPrefixPassesThroughUntouched() {
	ou := &ouStub{known: true}

	recorder, reached, seen := suite.serve(ou, "")

	suite.True(reached)
	suite.Equal(http.StatusOK, recorder.Code)
	suite.Empty(seen)
	suite.False(ou.called, "the bare endpoint must not resolve an organization unit")
}

func (suite *AccessingOUMiddlewareTestSuite) TestResolvableOUIsRecordedOnTheContext() {
	_, reached, seen := suite.serve(&ouStub{known: true}, accessingOU)

	suite.True(reached)
	suite.Equal(accessingOU, seen)
}

// An id naming nothing still satisfies a blanket allOus policy, so it has to be rejected here rather
// than left to fail much later during claim resolution.
func (suite *AccessingOUMiddlewareTestSuite) TestUnresolvableOUIsRefusedBeforeTheHandler() {
	recorder, reached, _ := suite.serve(&ouStub{known: false}, accessingOU)

	suite.False(reached, "the request must not reach client authentication")
	suite.Equal(http.StatusBadRequest, recorder.Code)

	var body map[string]any
	require.NoError(suite.T(), json.Unmarshal(recorder.Body.Bytes(), &body))
	suite.Equal(constants.ErrorInvalidRequest, body["error"])
	suite.Equal(constants.OUAccessRefusalDescription(accessingOU), body["error_description"])
}

// Fail closed: without a resolver there is no way to establish the organization unit exists, and
// admitting it would let an unknown id through to a blanket policy.
func (suite *AccessingOUMiddlewareTestSuite) TestMissingResolverRefuses() {
	next := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		suite.Fail("the request must not reach client authentication")
	})
	req := httptest.NewRequest(http.MethodPost, "/oauth2/token", nil)
	req.SetPathValue(constants.PathParamOUID, accessingOU)
	recorder := httptest.NewRecorder()

	accessingOUMiddleware(nil)(next).ServeHTTP(recorder, req)

	suite.Equal(http.StatusBadRequest, recorder.Code)
}
