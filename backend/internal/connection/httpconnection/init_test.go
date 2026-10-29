// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package httpconnection

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
)

type HTTPConnectionInitTestSuite struct {
	suite.Suite
}

func TestHTTPConnectionInitTestSuite(t *testing.T) {
	suite.Run(t, new(HTTPConnectionInitTestSuite))
}

func (s *HTTPConnectionInitTestSuite) SetupTest() {
	initStoreRuntime(s.T(), "", false)
}

func (s *HTTPConnectionInitTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (s *HTTPConnectionInitTestSuite) TestStoreFollowsTheMode() {
	s.IsType(&fileBasedStore{}, initializeStore(serverconst.StoreModeDeclarative))
	s.IsType(&compositeStore{}, initializeStore(serverconst.StoreModeComposite))
	s.IsType(&httpConnectionStore{}, initializeStore(serverconst.StoreModeMutable))
}

func (s *HTTPConnectionInitTestSuite) TestAnUnknownStoreModeFailsStartup() {
	service, err := Initialize(config.HTTPConnectionConfig{Store: "file"}, nil)

	s.Require().ErrorContains(err, `invalid http_connection store mode "file"`)
	s.Nil(service)
}

func (s *HTTPConnectionInitTestSuite) TestInitializeUsesTheResolvedStore() {
	service, err := Initialize(config.HTTPConnectionConfig{Store: "Declarative"}, nil)

	s.Require().NoError(err)
	s.IsType(&fileBasedStore{}, service.(*httpConnectionService).store)
}
