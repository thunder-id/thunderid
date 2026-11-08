// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package capability

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

type InitTestSuite struct {
	suite.Suite
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (s *InitTestSuite) TestInitializeRegistersRoutes() {
	security.InitSystemPermissions("")
	authz := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	authz.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).Return(false, nil).Maybe()
	authz.On("GetBoundary", mock.Anything).Return(sysauthz.Boundary{Type: sysauthz.BoundaryTypeDeployment}).Maybe()
	mux := http.NewServeMux()
	Initialize(mux, authz)

	tests := []struct {
		method string
		want   int
	}{
		{http.MethodGet, http.StatusOK},
		{http.MethodOptions, http.StatusNoContent},
	}
	for _, tc := range tests {
		s.Run(tc.method, func() {
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(tc.method, "/capabilities", nil))
			s.Equal(tc.want, rec.Code)
		})
	}
}
