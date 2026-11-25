// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/inboundclientmock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/revocationmock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
	"github.com/thunder-id/thunderid/tests/mocks/rolemock"
	"github.com/thunder-id/thunderid/tests/mocks/sharingmock"
)

func setupAgentConfig(t *testing.T, agentStore string, declarativeEnabled bool) {
	t.Helper()
	config.ResetServerRuntime()
	t.Cleanup(config.ResetServerRuntime)
	require.NoError(t, config.InitializeServerRuntime("", &config.Config{
		Agent:                config.AgentConfig{Store: agentStore},
		DeclarativeResources: config.DeclarativeResources{Enabled: declarativeEnabled},
	}))
}

func TestGetAgentStoreMode_ExplicitMutable(t *testing.T) {
	setupAgentConfig(t, "mutable", false)
	assert.Equal(t, serverconst.StoreModeMutable, getAgentStoreMode())
}

func TestGetAgentStoreMode_ExplicitDeclarative(t *testing.T) {
	setupAgentConfig(t, "declarative", false)
	assert.Equal(t, serverconst.StoreModeDeclarative, getAgentStoreMode())
}

func TestGetAgentStoreMode_ExplicitComposite(t *testing.T) {
	setupAgentConfig(t, "composite", false)
	assert.Equal(t, serverconst.StoreModeComposite, getAgentStoreMode())
}

func TestGetAgentStoreMode_ExplicitCaseInsensitive(t *testing.T) {
	setupAgentConfig(t, "  Mutable  ", false)
	assert.Equal(t, serverconst.StoreModeMutable, getAgentStoreMode())
}

func TestGetAgentStoreMode_FallbackDeclarativeModeEnabled(t *testing.T) {
	setupAgentConfig(t, "", true)
	assert.Equal(t, serverconst.StoreModeDeclarative, getAgentStoreMode())
}

func TestGetAgentStoreMode_FallbackDeclarativeModeDisabled(t *testing.T) {
	setupAgentConfig(t, "", false)
	assert.Equal(t, serverconst.StoreModeMutable, getAgentStoreMode())
}

func TestGetAgentStoreMode_InvalidStoreFallbackDeclarativeEnabled(t *testing.T) {
	setupAgentConfig(t, "unknown", true)
	assert.Equal(t, serverconst.StoreModeDeclarative, getAgentStoreMode())
}

func TestGetAgentStoreMode_InvalidStoreFallbackDeclarativeDisabled(t *testing.T) {
	setupAgentConfig(t, "unknown", false)
	assert.Equal(t, serverconst.StoreModeMutable, getAgentStoreMode())
}

// InitializeTestSuite tests the Initialize function's declarative resource loading paths.
type InitializeTestSuite struct {
	suite.Suite
}

func TestInitializeTestSuite(t *testing.T) {
	suite.Run(t, new(InitializeTestSuite))
}

func (suite *InitializeTestSuite) TestInitialize_EntityLoadError_AllDeclarativeModes() {
	for _, storeMode := range []serverconst.StoreMode{
		serverconst.StoreModeDeclarative,
		serverconst.StoreModeComposite,
	} {
		suite.Run(string(storeMode), func() {
			setupAgentConfig(suite.T(), string(storeMode), false)

			mockEntity := entitymock.NewEntityServiceInterfaceMock(suite.T())
			mockEntity.On("LoadIndexedAttributes", []string{"name"}).Return(nil).Once()
			mockInbound := inboundclientmock.NewInboundClientServiceInterfaceMock(suite.T())
			mockOU := oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())
			mockRole := rolemock.NewRoleServiceInterfaceMock(suite.T())
			mockSharing := newAgentSharingRegistry(suite.T(), mockEntity)

			mockEntity.On("LoadDeclarativeResources", mock.Anything).
				Return(errors.New("entity load error")).Once()

			svc, exporter, err := Initialize(http.NewServeMux(), nil, mockEntity, mockSharing, mockInbound, mockOU,
				mockRole, nil, nil, newAllowAllAuthz(suite.T()), nil, nil, nil,
				newEntityDBProvider(suite.T()), nil, nil, discardingBus{}, nil, nil)

			suite.EqualError(err, "entity load error")
			suite.Nil(svc)
			suite.Nil(exporter)
			mockEntity.AssertExpectations(suite.T())
		})
	}
}

func (suite *InitializeTestSuite) TestInitialize_InboundLoadError_AllDeclarativeModes() {
	for _, storeMode := range []serverconst.StoreMode{
		serverconst.StoreModeDeclarative,
		serverconst.StoreModeComposite,
	} {
		suite.Run(string(storeMode), func() {
			setupAgentConfig(suite.T(), string(storeMode), false)

			mockEntity := entitymock.NewEntityServiceInterfaceMock(suite.T())
			mockEntity.On("LoadIndexedAttributes", []string{"name"}).Return(nil).Once()
			mockInbound := inboundclientmock.NewInboundClientServiceInterfaceMock(suite.T())
			mockOU := oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())
			mockRole := rolemock.NewRoleServiceInterfaceMock(suite.T())
			mockSharing := newAgentSharingRegistry(suite.T(), mockEntity)

			mockEntity.On("LoadDeclarativeResources", mock.Anything).Return(nil).Once()
			mockInbound.On("LoadDeclarativeResources", mock.Anything, mock.Anything).
				Return(errors.New("inbound load error")).Once()

			mux := http.NewServeMux()
			svc, exporter, err := Initialize(
				mux, nil, mockEntity, mockSharing, mockInbound, mockOU, mockRole, nil, nil, newAllowAllAuthz(suite.T()),
				nil, nil, nil, newEntityDBProvider(suite.T()), nil, nil, discardingBus{}, nil, nil)

			suite.Error(err)
			suite.Equal("inbound load error", err.Error())
			suite.Nil(svc)
			suite.Nil(exporter)
			mockEntity.AssertExpectations(suite.T())
			mockInbound.AssertExpectations(suite.T())
		})
	}
}

func (suite *InitializeTestSuite) TestInitialize_DeclarativeMode_Success() {
	setupAgentConfig(suite.T(), string(serverconst.StoreModeDeclarative), false)

	mockEntity := entitymock.NewEntityServiceInterfaceMock(suite.T())
	mockEntity.On("LoadIndexedAttributes", []string{"name"}).Return(nil).Once()
	mockInbound := inboundclientmock.NewInboundClientServiceInterfaceMock(suite.T())
	mockOU := oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())
	mockRole := rolemock.NewRoleServiceInterfaceMock(suite.T())
	mockSharing := newAgentSharingRegistry(suite.T(), mockEntity)

	mockEntity.On("LoadDeclarativeResources", mock.Anything).Return(nil).Once()
	mockInbound.On("LoadDeclarativeResources", mock.Anything, mock.Anything).Return(nil).Once()
	mockSharing.On("LoadDeclarativeResources", mock.Anything, mock.MatchedBy(isAgentSharingConfig)).
		Return(nil).Once()

	mux := http.NewServeMux()
	svc, exporter, err := Initialize(
		mux, nil, mockEntity, mockSharing, mockInbound, mockOU, mockRole, nil, nil, newAllowAllAuthz(suite.T()),
		nil, nil, nil, newEntityDBProvider(suite.T()), nil, nil, discardingBus{}, nil, nil)

	suite.NoError(err)
	suite.NotNil(svc)
	suite.NotNil(exporter)
	mockEntity.AssertExpectations(suite.T())
	mockInbound.AssertExpectations(suite.T())
	mockSharing.AssertExpectations(suite.T())
}

// Declared sharing is loaded after the agents it is declared on, and a failure stops startup.
func (suite *InitializeTestSuite) TestInitialize_SharingLoadError_AllDeclarativeModes() {
	for _, storeMode := range []serverconst.StoreMode{
		serverconst.StoreModeDeclarative,
		serverconst.StoreModeComposite,
	} {
		suite.Run(string(storeMode), func() {
			setupAgentConfig(suite.T(), string(storeMode), false)

			mockEntity := entitymock.NewEntityServiceInterfaceMock(suite.T())
			mockEntity.On("LoadIndexedAttributes", []string{"name"}).Return(nil).Once()
			mockInbound := inboundclientmock.NewInboundClientServiceInterfaceMock(suite.T())
			mockOU := oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())
			mockRole := rolemock.NewRoleServiceInterfaceMock(suite.T())
			mockSharing := newAgentSharingRegistry(suite.T(), mockEntity)

			mockEntity.On("LoadDeclarativeResources", mock.Anything).Return(nil).Once()
			mockInbound.On("LoadDeclarativeResources", mock.Anything, mock.Anything).Return(nil).Once()
			mockSharing.On("LoadDeclarativeResources", mock.Anything, mock.MatchedBy(isAgentSharingConfig)).
				Return(errors.New("sharing load error")).Once()

			svc, exporter, err := Initialize(
				http.NewServeMux(), nil, mockEntity, mockSharing, mockInbound, mockOU, mockRole, nil, nil,
				newAllowAllAuthz(suite.T()), nil, nil, nil, newEntityDBProvider(suite.T()), nil, nil, discardingBus{},
				nil, nil)

			suite.EqualError(err, "sharing load error")
			suite.Nil(svc)
			suite.Nil(exporter)
		})
	}
}

func (suite *InitializeTestSuite) TestInitialize_MutableMode_SkipsDeclarativeLoading() {
	setupAgentConfig(suite.T(), string(serverconst.StoreModeMutable), false)

	mockEntity := entitymock.NewEntityServiceInterfaceMock(suite.T())
	mockEntity.On("LoadIndexedAttributes", []string{"name"}).Return(nil).Once()
	mockInbound := inboundclientmock.NewInboundClientServiceInterfaceMock(suite.T())
	mockOU := oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())
	mockRole := rolemock.NewRoleServiceInterfaceMock(suite.T())
	mockSharing := newAgentSharingRegistry(suite.T(), mockEntity)

	mux := http.NewServeMux()
	svc, exporter, err := Initialize(
		mux, nil, mockEntity, mockSharing, mockInbound, mockOU, mockRole, nil, nil, newAllowAllAuthz(suite.T()),
		nil, nil, nil, newEntityDBProvider(suite.T()), nil, nil, discardingBus{}, nil, nil)

	suite.NoError(err)
	suite.NotNil(svc)
	suite.NotNil(exporter)
	// No LoadDeclarativeResources calls expected in mutable mode.
	mockEntity.AssertNotCalled(suite.T(), "LoadDeclarativeResources")
	mockInbound.AssertNotCalled(suite.T(), "LoadDeclarativeResources")
	mockSharing.AssertNotCalled(suite.T(), "LoadDeclarativeResources")
}

// The owner's suspension routes are registered only with a revoker, which the control plane does not have.
func (suite *InitializeTestSuite) TestInitialize_OwnerSuspensionRoutesNeedARevoker() {
	setupAgentConfig(suite.T(), string(serverconst.StoreModeMutable), false)
	for name, revoker := range map[string]revocation.CriteriaRevoker{
		"with a revoker": revocationmock.NewCriteriaRevokerInterfaceMock(suite.T()),
		"without one":    nil,
	} {
		suite.Run(name, func() {
			mockEntity := entitymock.NewEntityServiceInterfaceMock(suite.T())
			mockEntity.On("LoadIndexedAttributes", []string{"name"}).Return(nil).Once()
			mux := http.NewServeMux()
			_, _, err := Initialize(mux, nil, mockEntity, newAgentSharingRegistry(suite.T(), mockEntity),
				inboundclientmock.NewInboundClientServiceInterfaceMock(suite.T()),
				oumock.NewOrganizationUnitServiceInterfaceMock(suite.T()),
				rolemock.NewRoleServiceInterfaceMock(suite.T()), nil, nil,
				newAllowAllAuthz(suite.T()), nil, nil, nil, newEntityDBProvider(suite.T()), nil, nil, discardingBus{},
				revoker, nil)
			suite.Require().NoError(err)

			for _, target := range []string{"/users/me/agents/suspend", "/users/me/agents/agent-1/suspend",
				"/users/me/agents/agent-1/unsuspend"} {
				_, pattern := mux.Handler(httptest.NewRequest(http.MethodPost, target, nil))
				if revoker != nil {
					suite.NotEmpty(pattern, target)
				} else {
					suite.Empty(pattern, target)
				}
			}
		})
	}
}

func (suite *InitializeTestSuite) TestInitialize_IndexedAttributesLoadError() {
	setupAgentConfig(suite.T(), string(serverconst.StoreModeMutable), false)

	mockEntity := entitymock.NewEntityServiceInterfaceMock(suite.T())
	mockInbound := inboundclientmock.NewInboundClientServiceInterfaceMock(suite.T())
	mockOU := oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())
	mockRole := rolemock.NewRoleServiceInterfaceMock(suite.T())
	mockSharing := sharingmock.NewSharingServiceInterfaceMock(suite.T())

	mockEntity.On("LoadIndexedAttributes", []string{"name"}).Return(errors.New("indexed load error")).Once()

	svc, exporter, err := Initialize(
		http.NewServeMux(), nil, mockEntity, mockSharing, mockInbound, mockOU, mockRole, nil, nil,
		newAllowAllAuthz(suite.T()), nil, nil, nil, newEntityDBProvider(suite.T()), nil, nil, discardingBus{}, nil, nil)

	suite.EqualError(err, "indexed load error")
	suite.Nil(svc)
	suite.Nil(exporter)
}

// Without an entity transactioner the agent service cannot run its writes, so Initialize fails before
// anything is registered or loaded.
func (suite *InitializeTestSuite) TestInitialize_TransactionerError() {
	dbProvider := providermock.NewDBProviderInterfaceMock(suite.T())
	dbProvider.On("GetEntityDBTransactioner").Return(nil, errors.New("entity db unavailable")).Once()
	mockEntity := entitymock.NewEntityServiceInterfaceMock(suite.T())

	svc, exporter, err := Initialize(http.NewServeMux(), nil, mockEntity, nil,
		inboundclientmock.NewInboundClientServiceInterfaceMock(suite.T()),
		oumock.NewOrganizationUnitServiceInterfaceMock(suite.T()), rolemock.NewRoleServiceInterfaceMock(suite.T()), nil,
		nil, newAllowAllAuthz(suite.T()), nil, nil, nil, dbProvider, nil, nil, discardingBus{}, nil, nil)

	suite.EqualError(err, "entity db unavailable")
	suite.Nil(svc)
	suite.Nil(exporter)
	mockEntity.AssertNotCalled(suite.T(), "LoadIndexedAttributes", mock.Anything)
}

// newAgentSharingRegistry expects the agent declaration to be registered with the sharing framework and
// the agent category to be registered as shared with the entity service.
func newAgentSharingRegistry(
	t *testing.T, mockEntity *entitymock.EntityServiceInterfaceMock,
) *sharingmock.SharingServiceInterfaceMock {
	mockSharing := sharingmock.NewSharingServiceInterfaceMock(t)
	mockSharing.On("RegisterResourceType", mock.MatchedBy(func(decl sharing.ResourceOverlayFieldDeclaration) bool {
		return decl.ResourceType() == agentSharingType
	})).Once()
	mockEntity.On("RegisterSharedCategory", providers.EntityCategoryAgent, agentSharingType).Once()
	return mockSharing
}

// isAgentSharingConfig matches the loader configuration that reads agents' declared sharing.
func isAgentSharingConfig(cfg sharing.DeclarativeLoaderConfig) bool {
	return cfg.ResourceType == agentSharingType && cfg.DirectoryName == "agents"
}

// newEntityDBProvider returns a DB provider whose entity transactioner runs the work without a database.
func newEntityDBProvider(t *testing.T) *providermock.DBProviderInterfaceMock {
	dbProvider := providermock.NewDBProviderInterfaceMock(t)
	dbProvider.On("GetEntityDBTransactioner").Return(transaction.NewNoOpTransactioner(), nil).Maybe()
	return dbProvider
}

func (suite *InitializeTestSuite) TestInitialize_RefusesANilEventBus() {
	svc, exporter, err := Initialize(http.NewServeMux(), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil)

	suite.ErrorIs(err, errNilEventBus)
	suite.Nil(svc)
	suite.Nil(exporter)
}
