// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	flowconfig "github.com/thunder-id/thunderid/internal/flow/config"
	"github.com/thunder-id/thunderid/internal/serverconfig"
	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
	"github.com/thunder-id/thunderid/tests/mocks/serverconfigmock"
	"github.com/thunder-id/thunderid/tests/mocks/transactionmock"
)

type InitTestSuite struct {
	suite.Suite
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

// Initialize builds the service over the session section and registers the management routes.
func (s *InitTestSuite) TestInitializeRegistersTheManagementRoutes() {
	dbProvider := providermock.NewDBProviderInterfaceMock(s.T())
	dbProvider.EXPECT().GetRuntimePersistentDBTransactioner().
		Return(transactionmock.NewTransactionerMock(s.T()), nil)
	reader := serverconfigmock.NewServerConfigServiceMock(s.T())
	reader.EXPECT().GetMergedConfig(mock.Anything, string(serverconfig.ConfigNameSession)).Return(Config{}, nil)
	mux := http.NewServeMux()

	svc, hook, err := Initialize(mux, dbProvider, testDeploymentID, reader, flowconfig.Config{}, nil, nil, nil, nil,
		eventbusmock.NewEventBusInterfaceMock(s.T()))

	s.Require().NoError(err)
	s.NotNil(hook)
	s.Equal(newTimeouts(Config{}, deploymentLimits{}).Absolute, svc.AbsoluteTimeout(context.Background()))
	for _, path := range []string{"/sessions", "/users/me/sessions"} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(http.MethodOptions, path, nil))
		s.Equal(http.StatusNoContent, w.Code, path)
	}
}

func (s *InitTestSuite) TestSetApplicationProvider() {
	svc := &service{}
	apps := newApplicationProviderMock(s.T())

	s.Require().NoError(SetApplicationProvider(svc, apps))
	s.Same(apps, svc.apps)
}

// A Service that can't take the provider, such as a mock of the interface, is reported.
func (s *InitTestSuite) TestSetApplicationProvider_UnsupportedService() {
	err := SetApplicationProvider(NewServiceMock(s.T()), newApplicationProviderMock(s.T()))

	s.ErrorContains(err, "does not support application provider injection")
}

func (s *InitTestSuite) TestInitializeTransactionerError() {
	dbProvider := providermock.NewDBProviderInterfaceMock(s.T())
	dbProvider.EXPECT().GetRuntimePersistentDBTransactioner().Return(nil, errors.New("db down"))

	_, _, err := Initialize(http.NewServeMux(), dbProvider, testDeploymentID, nil, flowconfig.Config{}, nil, nil,
		nil, nil, eventbusmock.NewEventBusInterfaceMock(s.T()))

	s.ErrorContains(err, "failed to get runtime persistent DB transactioner")
}

func (s *InitTestSuite) TestInitialize_RefusesANilEventBus() {
	svc, hook, err := Initialize(http.NewServeMux(), providermock.NewDBProviderInterfaceMock(s.T()), testDeploymentID,
		nil, flowconfig.Config{}, nil, nil, nil, nil, nil)

	s.ErrorIs(err, errNilEventBus)
	s.Nil(svc)
	s.Nil(hook)
}
