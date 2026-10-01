// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

// InitTestSuite covers wiring the service: that the store and its transactioner come from the
// configuration database, that the caches are optional, and that a database failure stops startup
// rather than yielding a half-built service.
type InitTestSuite struct {
	suite.Suite
	originalProvider func() provider.DBProviderInterface
	provider         *providermock.DBProviderInterfaceMock
	client           *providermock.DBClientInterfaceMock
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (s *InitTestSuite) SetupTest() {
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("", &config.Config{
		Server: engineconfig.ServerConfig{Identifier: storeDeploymentID},
	})
	s.originalProvider = getDBProvider
	s.provider = providermock.NewDBProviderInterfaceMock(s.T())
	s.client = providermock.NewDBClientInterfaceMock(s.T())
	getDBProvider = func() provider.DBProviderInterface { return s.provider }
}

func (s *InitTestSuite) TearDownTest() {
	getDBProvider = s.originalProvider
}

// With no cache manager the service still initializes; caching is an optimization, not a dependency.
func (s *InitTestSuite) TestInitializeWithoutCaches() {
	s.provider.On("GetConfigDBClient").Return(s.client, nil)
	s.client.On("GetTransactioner").Return(nil, nil).Once()
	hierarchy, enumerator := testResolver(s.T())

	svc, err := Initialize(nil, hierarchy, enumerator, false)

	s.Require().NoError(err)
	s.Require().NotNil(svc)
	concrete, ok := svc.(*sharingService)
	s.Require().True(ok)
	s.Nil(concrete.visibilityCache, "no cache manager means no visibility cache")
	s.Nil(concrete.overlayRuleCache)
	s.NotNil(concrete.fileStore, "the declarative store is always created")
	s.False(concrete.allowChildOUCrossTreeSharing)
}

// The cross-tree setting is carried through to the service, since it gates a decision the service
// makes rather than one the caller makes.
func (s *InitTestSuite) TestInitializeCarriesTheCrossTreeSetting() {
	s.provider.On("GetConfigDBClient").Return(s.client, nil)
	s.client.On("GetTransactioner").Return(nil, nil).Once()
	hierarchy, enumerator := testResolver(s.T())

	svc, err := Initialize(nil, hierarchy, enumerator, true)

	s.Require().NoError(err)
	concrete, ok := svc.(*sharingService)
	s.Require().True(ok)
	s.True(concrete.allowChildOUCrossTreeSharing)
}

// A cache manager produces both caches, which is what keeps a visibility check off the database on
// the common path.
func (s *InitTestSuite) TestInitializeWithCaches() {
	s.provider.On("GetConfigDBClient").Return(s.client, nil)
	s.client.On("GetTransactioner").Return(nil, nil).Once()
	manager := cache.Initialize(engineconfig.CacheConfig{Disabled: false, Size: 10, TTL: 60}, storeDeploymentID)
	hierarchy, enumerator := testResolver(s.T())

	svc, err := Initialize(manager, hierarchy, enumerator, false)

	s.Require().NoError(err)
	concrete, ok := svc.(*sharingService)
	s.Require().True(ok)
	s.NotNil(concrete.visibilityCache)
	s.NotNil(concrete.overlayRuleCache)
}

// A database that cannot be reached has to stop startup. Returning a service without a store would
// fail later, on the first request, with nothing pointing at the cause.
func (s *InitTestSuite) TestInitializeFailsWhenTheDatabaseIsUnavailable() {
	failure := errors.New("no database")
	s.provider.On("GetConfigDBClient").Return(nil, failure).Once()
	hierarchy, enumerator := testResolver(s.T())

	svc, err := Initialize(nil, hierarchy, enumerator, false)

	s.Require().Error(err)
	s.ErrorIs(err, failure)
	s.Nil(svc, "a failed initialization must not hand back a half-built service")
}

// The transactioner is fetched at construction rather than per write, so its failure surfaces here.
func (s *InitTestSuite) TestInitializeFailsWhenTheTransactionerIsUnavailable() {
	failure := errors.New("no transactioner")
	s.provider.On("GetConfigDBClient").Return(s.client, nil)
	s.client.On("GetTransactioner").Return(nil, failure).Once()
	hierarchy, enumerator := testResolver(s.T())

	svc, err := Initialize(nil, hierarchy, enumerator, false)

	s.Require().Error(err)
	s.ErrorIs(err, failure)
	s.Nil(svc)
}

// The store reads no configuration at construction: the deployment a query is scoped by is resolved
// from the request, so the store holds nothing but its provider.
func (s *InitTestSuite) TestNewSharingStoreHoldsNoDeploymentOfItsOwn() {
	s.provider.On("GetConfigDBClient").Return(s.client, nil)
	s.client.On("GetTransactioner").Return(nil, nil).Once()

	store, transactioner, err := newSharingStore()

	s.Require().NoError(err)
	s.Nil(transactioner, "the mock client yields no transactioner, which is what was asked of it")
	concrete, ok := store.(*sharingStore)
	s.Require().True(ok)
	s.Equal(s.provider, concrete.dbProvider)
}
