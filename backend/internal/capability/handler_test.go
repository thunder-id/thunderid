// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package capability

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

type HandlerTestSuite struct {
	suite.Suite
	authz *sysauthzmock.SystemAuthorizationServiceInterfaceMock
}

func TestHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(HandlerTestSuite))
}

func (s *HandlerTestSuite) SetupTest() {
	security.InitSystemPermissions("")
	s.authz = sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
}

// serve calls the handler and returns the recorded response.
func (s *HandlerTestSuite) serve() *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	newCapabilityHandler(newCapabilityService(s.authz)).
		HandleCapabilitiesGetRequest(rec, httptest.NewRequest(http.MethodGet, "/capabilities", nil))
	return rec
}

func (s *HandlerTestSuite) TestWritesTheCapabilities() {
	s.authz.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
	s.authz.On("GetBoundary", mock.Anything).Return(sysauthz.Boundary{Type: sysauthz.BoundaryTypeOU, OUID: "ou-1"})

	rec := s.serve()

	s.Equal(http.StatusOK, rec.Code)
	var body map[string]any
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &body))
	s.Equal(map[string]any{"type": "ou", "ouId": "ou-1"}, body["scope"])
	s.Equal(map[string]any{"manage": false}, body["root"])
	s.Contains(body["resources"], "app")
}

func (s *HandlerTestSuite) TestWritesAServerErrorWhenEvaluationFails() {
	s.authz.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
		Return(false, &tidcommon.InternalServerError).Once()

	rec := s.serve()

	s.Equal(http.StatusInternalServerError, rec.Code)
}
