// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowexec

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/actorprovider"
	"github.com/thunder-id/thunderid/internal/application/model"
	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/common"
	flowconfig "github.com/thunder-id/thunderid/internal/flow/config"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/interceptor"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	sysContext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/kmprovider/defaultkm"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/actorprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/attestationprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/crypto/cryptomock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/identitygovernanceprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/inboundclientmock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
	"github.com/thunder-id/thunderid/tests/mocks/observability/observabilitymock"
	"github.com/thunder-id/thunderid/tests/mocks/ouprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/permissionprovidermock"
)

const existingExecutionID = "existing-execution-id"

const testAppID = "test-app-123"

// txMarkerKey is an unexported type used as a context key for the transaction marker in tests.
type txMarkerKey struct{}

// stubTransactioner is a stub implementation of Transactioner for testing.
type stubTransactioner struct{}

func (s *stubTransactioner) Transact(ctx context.Context, txFunc func(context.Context) error) error {
	txCtx := context.WithValue(ctx, txMarkerKey{}, "tx")
	return txFunc(txCtx)
}

const testUserOnboardingFlowHandle = "onboarding-handle"

const testDefaultAuthFlowHandle = "default-auth-handle"

var testFlowConfig = engineconfig.FlowConfig{}

var testFlowExecCfg = flowconfig.Config{
	Flow: testFlowConfig,
}

// stubServerConfig is a test implementation of serverConfigProvider that returns
// a pre-configured FlowSectionConfig for the "flow" section.
type stubServerConfig struct {
	cfg flowconfig.FlowSectionConfig
}

func (s stubServerConfig) GetMergedConfig(_ context.Context, name string) (any, *tidcommon.ServiceError) {
	if name == "flow" {
		return s.cfg, nil
	}
	return nil, nil
}

type ServiceTestSuite struct {
	suite.Suite
}

func TestServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceTestSuite))
}

func (s *ServiceTestSuite) TestFlowErrorCallbackDataIncludesCallbackTypeAndAuthID() {
	engineCtx := &EngineContext{sharedRuntimeData: map[string]string{
		common.RuntimeKeyCallbackType:           string(providers.GrantTypeDeviceCode),
		common.RuntimeKeyAuthorizationRequestID: "device-request-1",
	}}

	s.Equal(map[string]string{
		common.DataCallbackType: string(providers.GrantTypeDeviceCode),
		oauth2const.AuthID:      "device-request-1",
	}, flowErrorCallbackData(engineCtx))
	s.Equal(map[string]string{
		common.DataCallbackType: string(providers.GrantTypeAuthorizationCode),
		oauth2const.AuthID:      "authorization-request-1",
	}, flowErrorCallbackData(&EngineContext{RuntimeData: map[string]string{
		common.RuntimeKeyCallbackType:           string(providers.GrantTypeAuthorizationCode),
		common.RuntimeKeyAuthorizationRequestID: "authorization-request-1",
	}}))
	s.Nil(flowErrorCallbackData(nil))
	s.Nil(flowErrorCallbackData(&EngineContext{RuntimeData: map[string]string{
		common.RuntimeKeyCallbackType: string(providers.GrantTypeAuthorizationCode),
	}}))
	s.Nil(flowErrorCallbackData(&EngineContext{RuntimeData: map[string]string{
		common.RuntimeKeyCallbackType: string(providers.GrantTypeDeviceCode),
	}}))
}

func (s *ServiceTestSuite) TestBuildErrorAssertionUsesSharedAuthorizationRequestID() {
	jwtService := jwtmock.NewJWTServiceInterfaceMock(s.T())
	jwtService.EXPECT().GenerateJWT(mock.Anything, "", "", int64(0), mock.MatchedBy(
		func(claims map[string]interface{}) bool {
			return claims[common.ClaimAuthorizationRequestID] == "device-request-1"
		}), jwt.TokenTypeJWT, "").Return("signed-error", int64(60), (*tidcommon.ServiceError)(nil))
	service := &flowExecService{jwtService: jwtService}
	engineCtx := &EngineContext{
		AppID: "app-1",
		sharedRuntimeData: map[string]string{
			common.RuntimeKeyAuthorizationRequestID: "device-request-1",
		},
	}

	assertion := service.buildErrorAssertion(context.Background(), engineCtx,
		common.FlowErrorTypeServer, "flow failed", log.GetLogger())
	s.Equal("signed-error", assertion)
}

func TestInitiateFlowNilContext(t *testing.T) {
	// Setup
	service := &flowExecService{cfg: testFlowExecCfg}

	// Execute
	executionID, err := service.InitiateFlow(context.Background(), nil)

	// Assert
	assert.NotNil(t, err)
	assert.Empty(t, executionID)
	assert.Equal(t, "FES-1008", err.Code)
}

func TestInitiateFlowEmptyApplicationID(t *testing.T) {
	// Setup
	service := &flowExecService{cfg: testFlowExecCfg}

	initContext := &FlowInitContext{
		ApplicationID: "",
		FlowType:      "AUTHENTICATION",
		RuntimeData:   map[string]string{},
	}

	// Execute
	executionID, err := service.InitiateFlow(context.Background(), initContext)

	// Assert
	assert.NotNil(t, err)
	assert.Empty(t, executionID)
	assert.Equal(t, "FES-1008", err.Code)
}

func TestInitiateFlowEmptyFlowType(t *testing.T) {
	// Setup
	service := &flowExecService{cfg: testFlowExecCfg}

	initContext := &FlowInitContext{
		ApplicationID: "test-app",
		FlowType:      "",
		RuntimeData:   map[string]string{},
	}

	// Execute
	executionID, err := service.InitiateFlow(context.Background(), initContext)

	// Assert
	assert.NotNil(t, err)
	assert.Empty(t, executionID)
	assert.Equal(t, "FES-1008", err.Code)
}

func TestInitiateFlowInvalidFlowType(t *testing.T) {
	// Setup
	service := &flowExecService{cfg: testFlowExecCfg}

	initContext := &FlowInitContext{
		ApplicationID: "test-app",
		FlowType:      "INVALID_TYPE",
		RuntimeData:   map[string]string{},
	}

	// Execute
	executionID, err := service.InitiateFlow(context.Background(), initContext)

	// Assert
	assert.NotNil(t, err)
	assert.Empty(t, executionID)
	assert.Equal(t, "FES-1005", err.Code) // ErrorInvalidFlowType
}

func TestInitiateFlowSuccessScenarios(t *testing.T) {
	appID := testAppID
	security.InitControlPlanePermissions("")

	testConfig := &config.Config{}
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)

	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	testGraph := flowFactory.CreateGraph("auth-graph-1", providers.FlowTypeAuthentication, 1)

	// Mock inbound client + entity for the flow's owning entity (shared across test cases).
	mockClient := &inboundmodel.InboundClient{
		ID:         "app-id-123",
		AuthFlowID: "auth-graph-1",
	}
	mockEntity := &providers.Entity{ID: appID, Category: providers.EntityCategoryApp}

	tests := []struct {
		name                     string
		runtimeData              map[string]string
		setRuntimeDataField      bool // whether to explicitly set the RuntimeData field
		expectedRuntimeDataCheck func(ctx EngineContext) bool
	}{
		{
			name: "with runtime data",
			runtimeData: map[string]string{
				"permissions": "perm1 perm2 perm3",
				"state":       "random-state-value",
				"type":        "code",
			},
			setRuntimeDataField: true,
			expectedRuntimeDataCheck: func(ctx EngineContext) bool {
				// Verify RuntimeData is preserved
				return ctx.RuntimeData != nil &&
					ctx.RuntimeData["permissions"] == "perm1 perm2 perm3" &&
					ctx.RuntimeData["state"] == "random-state-value" &&
					ctx.RuntimeData["type"] == "code"
			},
		},
		{
			name:                "with nil runtime data",
			runtimeData:         nil,
			setRuntimeDataField: true,
			expectedRuntimeDataCheck: func(ctx EngineContext) bool {
				// Verify RuntimeData is nil (since initContext.RuntimeData is nil and len > 0 check fails)
				return ctx.RuntimeData == nil
			},
		},
		{
			name:                "with empty runtime data",
			runtimeData:         map[string]string{},
			setRuntimeDataField: true,
			expectedRuntimeDataCheck: func(ctx EngineContext) bool {
				// Verify RuntimeData is not nil and empty
				return ctx.RuntimeData != nil && len(ctx.RuntimeData) == 0
			},
		},
		{
			name:                "without runtime data field",
			runtimeData:         nil, // This won't be used since setRuntimeDataField is false
			setRuntimeDataField: false,
			expectedRuntimeDataCheck: func(ctx EngineContext) bool {
				// Verify RuntimeData is nil (since initContext.RuntimeData is nil and len > 0 check fails)
				return ctx.RuntimeData == nil
			},
		},
		{
			name: "user onboarding flow (system flow)",
			runtimeData: map[string]string{
				"email": "test@example.com",
			},
			setRuntimeDataField: true,
			expectedRuntimeDataCheck: func(ctx EngineContext) bool {
				return ctx.RuntimeData != nil && ctx.RuntimeData["email"] == "test@example.com"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockStore := newFlowStoreInterfaceMock(t)
			mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(t)
			mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)
			mockFlowProvider := NewFlowProviderMock(t)
			mockGraphBuilder := NewGraphBuilderInterfaceMock(t)
			mockCrypto := cryptomock.NewRuntimeCryptoProviderMock(t)
			mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return([]byte("encrypted-ctx"), nil, nil)

			// Create service with mocked dependencies
			service := &flowExecService{
				graphBuilder: mockGraphBuilder,
				flowProvider: mockFlowProvider,
				flowStore:    mockStore,
				actorProvider: actorprovider.Initialize(
					mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil, nil, nil),
				flowEngine:    nil,
				transactioner: &stubTransactioner{},
				cryptoSvc:     mockCrypto,
				cfg:           testFlowExecCfg,
				serverConfigSvc: stubServerConfig{cfg: flowconfig.FlowSectionConfig{
					AuthFlow:           flowconfig.FlowTypeConfig{DefaultHandle: testDefaultAuthFlowHandle},
					UserOnboardingFlow: flowconfig.FlowTypeConfig{DefaultHandle: testUserOnboardingFlowHandle},
				}},
			}

			initContext := &FlowInitContext{
				ApplicationID: appID,
				FlowType:      "AUTHENTICATION",
			}

			// Set RuntimeData field only if specified in test case
			if tt.setRuntimeDataField {
				initContext.RuntimeData = tt.runtimeData
			}

			// Setup expectations
			if tt.name == "user onboarding flow (system flow)" {
				systemFlowType := providers.FlowTypeUserOnboarding
				systemHandle := testUserOnboardingFlowHandle
				initContext.FlowType = string(systemFlowType)
				initContext.ApplicationID = "" // System flows don't need app ID

				// Mock flow management service to return flow by handle
				mockFlow := &providers.CompleteFlowDefinition{
					ID:       "onboarding-flow-123",
					FlowType: systemFlowType,
				}
				mockFlowProvider.EXPECT().GetFlowByHandle(mock.Anything,
					systemHandle, systemFlowType).Return(mockFlow, nil)

				// Mock GetGraph call which is made during initContext
				inviteGraph := flowFactory.CreateGraph("onboarding-flow-123", systemFlowType, 1)
				mockFlowProvider.EXPECT().
					GetFlow(mock.Anything, "onboarding-flow-123").
					Return(&providers.CompleteFlowDefinition{
						ID:       "onboarding-flow-123",
						FlowType: systemFlowType,
					}, nil)
				mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(inviteGraph, nil)

				mockStore.EXPECT().StoreFlowContext(mock.MatchedBy(func(ctx context.Context) bool {
					return ctx.Value(txMarkerKey{}) == "tx"
				}), mock.MatchedBy(func(encryptedEngineCtx FlowContextDB) bool {
					return encryptedEngineCtx.ExecutionID != ""
				}), mock.Anything).Return(nil)
			} else {
				mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, appID).
					Return(mockClient, nil)
				mockEntityProvider.EXPECT().GetEntityProfile(mock.Anything, appID).
					Return(mockEntity, (*entityprovider.EntityProviderError)(nil))
				mockFlowProvider.EXPECT().GetFlow(mock.Anything, "auth-graph-1").
					Return(&providers.CompleteFlowDefinition{ID: "auth-graph-1"}, nil)
				mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(testGraph, nil)
				mockStore.EXPECT().StoreFlowContext(mock.MatchedBy(func(ctx context.Context) bool {
					return ctx.Value(txMarkerKey{}) == "tx"
				}), mock.MatchedBy(func(encryptedEngineCtx FlowContextDB) bool {
					return encryptedEngineCtx.ExecutionID != ""
				}), mock.Anything).Return(nil)
			}

			// Execute
			executionID, svcErr := service.InitiateFlow(authenticatedAdminContext([]string{"system"}), initContext)

			// Assert
			assert.NotEmpty(t, executionID)
			assert.Nil(t, svcErr)

			// All mocks automatically verified by mockery
		})
	}
}

// An onboarding flow loads no application, so the caller-supplied ID is not checked.
func (s *ServiceTestSuite) TestInitiateFlowChecksOnlyALoadedApplication() {
	config.ResetServerRuntime()
	defer config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), &config.Config{}))
	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))

	mockStore := newFlowStoreInterfaceMock(s.T())
	mockFlowProvider := NewFlowProviderMock(s.T())
	mockGraphBuilder := NewGraphBuilderInterfaceMock(s.T())
	mockCrypto := cryptomock.NewRuntimeCryptoProviderMock(s.T())
	mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]byte("encrypted-ctx"), nil, nil)
	governance := identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(s.T())

	service := &flowExecService{
		graphBuilder:  mockGraphBuilder,
		flowProvider:  mockFlowProvider,
		flowStore:     mockStore,
		transactioner: &stubTransactioner{},
		cryptoSvc:     mockCrypto,
		cfg:           testFlowExecCfg,
		governance:    governance,
		serverConfigSvc: stubServerConfig{cfg: flowconfig.FlowSectionConfig{
			UserOnboardingFlow: flowconfig.FlowTypeConfig{DefaultHandle: testUserOnboardingFlowHandle},
		}},
	}

	onboarding := &providers.CompleteFlowDefinition{
		ID: "onboarding-flow-123", FlowType: providers.FlowTypeUserOnboarding}
	mockFlowProvider.EXPECT().GetFlowByHandle(mock.Anything, testUserOnboardingFlowHandle,
		providers.FlowTypeUserOnboarding).Return(onboarding, nil)
	mockFlowProvider.EXPECT().GetFlow(mock.Anything, "onboarding-flow-123").Return(onboarding, nil)
	mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).
		Return(flowFactory.CreateGraph("onboarding-flow-123", providers.FlowTypeUserOnboarding, 1), nil)
	mockStore.EXPECT().StoreFlowContext(mock.Anything, mock.Anything, mock.Anything).Return(nil)

	security.InitControlPlanePermissions("")
	executionID, svcErr := service.InitiateFlow(authenticatedAdminContext([]string{"system"}), &FlowInitContext{
		ApplicationID: "not-an-application",
		FlowType:      string(providers.FlowTypeUserOnboarding),
	})

	s.Nil(svcErr)
	s.NotEmpty(executionID)
	governance.AssertNotCalled(s.T(), "EvaluateAccess", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestInitiateFlowErrorScenarios(t *testing.T) {
	appID := testAppID

	testConfig := &config.Config{}
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)

	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))

	tests := []struct {
		name       string
		setupMocks func(
			*flowStoreInterfaceMock,
			*inboundclientmock.InboundClientServiceInterfaceMock,
			*entityprovidermock.EntityProviderInterfaceMock,
			*FlowProviderMock,
			*GraphBuilderInterfaceMock,
		)
		expectedErrorCode        string
		expectedErrorDescription string
	}{
		{
			name: "error from inbound client lookup - not found",
			setupMocks: func(
				mockStore *flowStoreInterfaceMock,
				mockInboundClient *inboundclientmock.InboundClientServiceInterfaceMock,
				mockEntityProvider *entityprovidermock.EntityProviderInterfaceMock,
				mockFlowProvider *FlowProviderMock,
				_ *GraphBuilderInterfaceMock,
			) {
				mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, appID).
					Return(nil, inboundclient.ErrInboundClientNotFound)
			},
			expectedErrorCode: "FES-1003", // ErrorInvalidAppID
		},
		{
			name: "error from inbound client lookup - server error",
			setupMocks: func(
				mockStore *flowStoreInterfaceMock,
				mockInboundClient *inboundclientmock.InboundClientServiceInterfaceMock,
				mockEntityProvider *entityprovidermock.EntityProviderInterfaceMock,
				mockFlowProvider *FlowProviderMock,
				_ *GraphBuilderInterfaceMock,
			) {
				mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, appID).
					Return(nil, assert.AnError)
			},
			expectedErrorCode: tidcommon.InternalServerError.Code,
		},
		{
			name: "error from flowProvider.GetGraph - graph not found",
			setupMocks: func(
				mockStore *flowStoreInterfaceMock,
				mockInboundClient *inboundclientmock.InboundClientServiceInterfaceMock,
				mockEntityProvider *entityprovidermock.EntityProviderInterfaceMock,
				mockFlowProvider *FlowProviderMock,
				_ *GraphBuilderInterfaceMock,
			) {
				mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, appID).
					Return(&inboundmodel.InboundClient{ID: appID, AuthFlowID: "auth-graph-1"}, nil)

				// Mock flow provider to return error (flow not found)
				mockFlowProvider.EXPECT().GetFlow(mock.Anything, "auth-graph-1").
					Return(nil, &tidcommon.InternalServerError)
			},
			expectedErrorCode: tidcommon.InternalServerError.Code,
		},
		{
			name: "error from storeContext - store failure",
			setupMocks: func(
				mockStore *flowStoreInterfaceMock,
				mockInboundClient *inboundclientmock.InboundClientServiceInterfaceMock,
				mockEntityProvider *entityprovidermock.EntityProviderInterfaceMock,
				mockFlowProvider *FlowProviderMock,
				mockGraphBuilder *GraphBuilderInterfaceMock,
			) {
				mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, appID).
					Return(&inboundmodel.InboundClient{ID: appID, AuthFlowID: "auth-graph-1"}, nil)
				mockEntityProvider.EXPECT().GetEntityProfile(mock.Anything, appID).Return(
					&providers.Entity{ID: appID, Category: providers.EntityCategoryApp},
					(*entityprovider.EntityProviderError)(nil))

				// Mock flow management service to return valid graph
				testGraph := flowFactory.CreateGraph("auth-graph-1", providers.FlowTypeAuthentication, 1)
				mockFlowProvider.EXPECT().
					GetFlow(mock.Anything, "auth-graph-1").
					Return(&providers.CompleteFlowDefinition{ID: "auth-graph-1"}, nil)
				mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(testGraph, nil)

				// Mock store to return error
				mockStore.EXPECT().StoreFlowContext(
					mock.MatchedBy(func(ctx context.Context) bool {
						return ctx.Value(txMarkerKey{}) == "tx"
					}),
					mock.AnythingOfType("FlowContextDB"), mock.Anything).Return(assert.AnError)
			},
			expectedErrorCode: tidcommon.InternalServerError.Code,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockStore := newFlowStoreInterfaceMock(t)
			mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(t)
			mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)
			mockFlowProvider := NewFlowProviderMock(t)
			mockGraphBuilder := NewGraphBuilderInterfaceMock(t)
			mockCrypto := cryptomock.NewRuntimeCryptoProviderMock(t)
			mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return([]byte("encrypted-ctx"), nil, nil).Maybe()

			// Create service with mocked dependencies
			service := &flowExecService{
				graphBuilder: mockGraphBuilder,
				flowProvider: mockFlowProvider,
				flowStore:    mockStore,
				actorProvider: actorprovider.Initialize(
					mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil, nil, nil),
				flowEngine:    nil,
				transactioner: &stubTransactioner{},
				cryptoSvc:     mockCrypto,
				cfg:           testFlowExecCfg,
				serverConfigSvc: stubServerConfig{cfg: flowconfig.FlowSectionConfig{
					AuthFlow:           flowconfig.FlowTypeConfig{DefaultHandle: testDefaultAuthFlowHandle},
					UserOnboardingFlow: flowconfig.FlowTypeConfig{DefaultHandle: testUserOnboardingFlowHandle},
				}},
			}

			initContext := &FlowInitContext{
				ApplicationID: appID,
				FlowType:      "AUTHENTICATION",
				RuntimeData: map[string]string{
					"test": "data",
				},
			}

			// Setup test-specific mocks
			tt.setupMocks(mockStore, mockInboundClient, mockEntityProvider, mockFlowProvider, mockGraphBuilder)

			// Execute
			executionID, svcErr := service.InitiateFlow(context.Background(), initContext)

			// Assert
			assert.Empty(t, executionID)
			assert.NotNil(t, svcErr)
			assert.Equal(t, tt.expectedErrorCode, svcErr.Code)

			// All mocks automatically verified by mockery
		})
	}
}

func TestInitiateFlowFallsBackToDefaultFlow(t *testing.T) {
	appID := testAppID

	testConfig := &config.Config{}
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)

	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	defaultGraph := flowFactory.CreateGraph("default-auth-graph", providers.FlowTypeAuthentication, 1)

	flowNotFound := &tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "FLM-1003",
		Error: tidcommon.I18nMessage{DefaultValue: "Flow not found"},
	}

	t.Run("auth flow deleted - falls back to default", func(t *testing.T) {
		mockStore := newFlowStoreInterfaceMock(t)
		mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(t)
		mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)
		mockFlowProvider := NewFlowProviderMock(t)
		mockGraphBuilder := NewGraphBuilderInterfaceMock(t)
		mockCrypto := cryptomock.NewRuntimeCryptoProviderMock(t)
		mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return([]byte("encrypted-ctx"), nil, nil)
		mockOU := ouprovidermock.NewOrganizationUnitProviderMock(t)

		service := &flowExecService{
			graphBuilder: mockGraphBuilder,
			flowProvider: mockFlowProvider,
			flowStore:    mockStore,
			actorProvider: actorprovider.Initialize(
				mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil, nil, nil),
			ouProvider:    mockOU,
			transactioner: &stubTransactioner{},
			cryptoSvc:     mockCrypto,
			cfg:           testFlowExecCfg,
			serverConfigSvc: stubServerConfig{cfg: flowconfig.FlowSectionConfig{
				AuthFlow: flowconfig.FlowTypeConfig{DefaultHandle: testDefaultAuthFlowHandle},
			}},
		}

		mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, appID).
			Return(&inboundmodel.InboundClient{ID: appID, AuthFlowID: "stale-auth-graph"}, nil)
		mockEntityProvider.EXPECT().GetEntityProfile(mock.Anything, appID).
			Return(&providers.Entity{ID: appID, Category: providers.EntityCategoryApp},
				(*entityprovider.EntityProviderError)(nil))
		mockOU.EXPECT().IsApplicationShared(mock.Anything).Return(true, nil)

		// The referenced flow was deleted.
		mockFlowProvider.EXPECT().GetFlow(mock.Anything, "stale-auth-graph").
			Return(nil, flowNotFound)
		// Fallback resolves the default authentication flow by handle.
		mockFlowProvider.EXPECT().
			GetFlowByHandle(mock.Anything, testDefaultAuthFlowHandle, providers.FlowTypeAuthentication).
			Return(&providers.CompleteFlowDefinition{
				ID:       "default-auth-graph",
				FlowType: providers.FlowTypeAuthentication,
			}, nil)
		mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(defaultGraph, nil)
		mockStore.EXPECT().StoreFlowContext(mock.Anything, mock.MatchedBy(
			func(encryptedEngineCtx FlowContextDB) bool {
				return encryptedEngineCtx.ExecutionID != ""
			}), mock.Anything).Return(nil)

		executionID, svcErr := service.InitiateFlow(context.Background(), &FlowInitContext{
			ApplicationID: appID,
			FlowType:      "AUTHENTICATION",
		})

		assert.Nil(t, svcErr)
		assert.NotEmpty(t, executionID)
	})

	t.Run("server error retrieving flow - no fallback", func(t *testing.T) {
		mockStore := newFlowStoreInterfaceMock(t)
		mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(t)
		mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)
		mockFlowProvider := NewFlowProviderMock(t)
		mockGraphBuilder := NewGraphBuilderInterfaceMock(t)

		service := &flowExecService{
			graphBuilder: mockGraphBuilder,
			flowProvider: mockFlowProvider,
			flowStore:    mockStore,
			actorProvider: actorprovider.Initialize(
				mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil, nil, nil),
			transactioner: &stubTransactioner{},
			cfg:           testFlowExecCfg,
		}

		mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, appID).
			Return(&inboundmodel.InboundClient{ID: appID, AuthFlowID: "stale-auth-graph"}, nil)
		mockFlowProvider.EXPECT().GetFlow(mock.Anything, "stale-auth-graph").
			Return(nil, &tidcommon.InternalServerError)

		executionID, svcErr := service.InitiateFlow(context.Background(), &FlowInitContext{
			ApplicationID: appID,
			FlowType:      "AUTHENTICATION",
		})

		assert.NotNil(t, svcErr)
		assert.Empty(t, executionID)
		assert.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)
	})

	t.Run("default flow retrieval fails during fallback", func(t *testing.T) {
		mockStore := newFlowStoreInterfaceMock(t)
		mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(t)
		mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)
		mockFlowProvider := NewFlowProviderMock(t)
		mockGraphBuilder := NewGraphBuilderInterfaceMock(t)

		service := &flowExecService{
			graphBuilder: mockGraphBuilder,
			flowProvider: mockFlowProvider,
			flowStore:    mockStore,
			actorProvider: actorprovider.Initialize(
				mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil, nil, nil),
			transactioner: &stubTransactioner{},
			cfg:           testFlowExecCfg,
			serverConfigSvc: stubServerConfig{cfg: flowconfig.FlowSectionConfig{
				AuthFlow: flowconfig.FlowTypeConfig{DefaultHandle: testDefaultAuthFlowHandle},
			}},
		}

		mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, appID).
			Return(&inboundmodel.InboundClient{ID: appID, AuthFlowID: "stale-auth-graph"}, nil)
		// The referenced flow was deleted, triggering the fallback.
		mockFlowProvider.EXPECT().GetFlow(mock.Anything, "stale-auth-graph").
			Return(nil, flowNotFound)
		// Resolving the default authentication flow also fails.
		mockFlowProvider.EXPECT().
			GetFlowByHandle(mock.Anything, testDefaultAuthFlowHandle, providers.FlowTypeAuthentication).
			Return(nil, &tidcommon.InternalServerError)

		executionID, svcErr := service.InitiateFlow(context.Background(), &FlowInitContext{
			ApplicationID: appID,
			FlowType:      "AUTHENTICATION",
		})

		assert.NotNil(t, svcErr)
		assert.Empty(t, executionID)
		assert.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)
	})
}

func TestGetFlowExpirySeconds(t *testing.T) {
	service := &flowExecService{cfg: testFlowExecCfg}

	tests := []struct {
		name     string
		flowType providers.FlowType
		expected int64
	}{
		{
			name:     "Authentication flow",
			flowType: providers.FlowTypeAuthentication,
			expected: 3600,
		},
		{
			name:     "Registration flow",
			flowType: providers.FlowTypeRegistration,
			expected: 3600,
		},
		{
			name:     "User onboarding flow",
			flowType: providers.FlowTypeUserOnboarding,
			expected: 86400,
		},
		{
			name:     "Invitation flow",
			flowType: providers.FlowTypeInvitation,
			expected: 86400,
		},
		{
			name:     "Unknown flow type (fallback)",
			flowType: providers.FlowType("UNKNOWN_FLOW"),
			expected: 3600,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.getFlowExpirySeconds(context.Background(), tt.flowType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEncryptedPayloadStoredBeforeWrite(t *testing.T) {
	// Verifies that the context passed to StoreFlowContext is the encrypted payload
	// returned by cryptoSvc.Encrypt, not the plain serialized JSON.
	const encryptedPayload = `{"alg":"AES-GCM","ct":"c2VjcmV0","kid":"k1"}`

	testConfig := &config.Config{}
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)

	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	testGraph := flowFactory.CreateGraph("auth-graph-1", providers.FlowTypeAuthentication, 1)

	mockStore := newFlowStoreInterfaceMock(t)
	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(t)
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)
	mockFlowProvider := NewFlowProviderMock(t)
	mockGraphBuilder := NewGraphBuilderInterfaceMock(t)
	mockCrypto := cryptomock.NewRuntimeCryptoProviderMock(t)
	mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]byte(encryptedPayload), nil, nil)

	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, "test-app").Return(
		&inboundmodel.InboundClient{ID: "test-app", AuthFlowID: "auth-graph-1"}, nil)
	mockEntityProvider.EXPECT().GetEntityProfile(mock.Anything, "test-app").Return(
		&providers.Entity{ID: "test-app", Category: providers.EntityCategoryApp},
		(*entityprovider.EntityProviderError)(nil))
	mockFlowProvider.EXPECT().
		GetFlow(mock.Anything, "auth-graph-1").
		Return(&providers.CompleteFlowDefinition{ID: "auth-graph-1"}, nil)
	mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(testGraph, nil)
	mockStore.EXPECT().StoreFlowContext(
		mock.MatchedBy(func(ctx context.Context) bool { return ctx.Value(txMarkerKey{}) == "tx" }),
		mock.MatchedBy(func(dbModel FlowContextDB) bool {
			return dbModel.Context == encryptedPayload
		}),
		mock.Anything).Return(nil)

	service := &flowExecService{
		graphBuilder:  mockGraphBuilder,
		flowProvider:  mockFlowProvider,
		flowStore:     mockStore,
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil, nil, nil),
		transactioner: &stubTransactioner{},
		cryptoSvc:     mockCrypto,
		cfg:           testFlowExecCfg,
	}

	executionID, svcErr := service.InitiateFlow(context.Background(), &FlowInitContext{
		ApplicationID: "test-app",
		FlowType:      "AUTHENTICATION",
	})

	assert.NotEmpty(t, executionID)
	assert.Nil(t, svcErr)
}

func TestDecryptCalledForEncryptedStoredContext(t *testing.T) {
	// Verifies that when GetFlowContext returns an encrypted context (has "alg" field),
	// Decrypt is called and the engine receives the properly restored EngineContext.
	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	testGraph := flowFactory.CreateGraph("test-graph-id", providers.FlowTypeAuthentication, 1)

	engineCtx := EngineContext{
		ExecutionID:       existingExecutionID,
		AppID:             "test-app-id",
		FlowType:          providers.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{Attributes: map[string]interface{}{}},
		UserInputs:        map[string]string{},
		RuntimeData:       map[string]string{},
		ExecutionHistory:  map[string]*providers.NodeExecutionRecord{},
		Graph:             testGraph,
	}
	plainCtx := &FlowContextDB{}
	err := plainCtx.FromEngineContext(engineCtx)
	assert.NoError(t, err)

	// Simulate what the store returns: an encrypted blob
	encryptedStoredCtx := &FlowContextDB{
		ExecutionID: existingExecutionID,
		Context:     `{"alg":"AES-GCM","ct":"c2VjcmV0","kid":"k1"}`,
	}

	mockStore := newFlowStoreInterfaceMock(t)
	mockFlowProvider := NewFlowProviderMock(t)
	mockGraphBuilder := NewGraphBuilderInterfaceMock(t)
	mockEngine := newFlowEngineInterfaceMock(t)
	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(t)
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)
	mockCrypto := cryptomock.NewRuntimeCryptoProviderMock(t)

	// Decrypt should be called with the encrypted blob and return the plain JSON
	mockCrypto.EXPECT().Decrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		[]byte(encryptedStoredCtx.Context)).
		Return([]byte(plainCtx.Context), nil)
	// Encrypt called when updating context after engine runs
	mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]byte("re-encrypted"), nil, nil)

	mockStore.EXPECT().GetFlowContext(mock.Anything, existingExecutionID).Return(encryptedStoredCtx, nil)
	mockFlowProvider.EXPECT().
		GetFlow(mock.Anything, "test-graph-id").
		Return(&providers.CompleteFlowDefinition{ID: "test-graph-id"}, nil)
	mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(testGraph, nil)
	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, "test-app-id").Return(
		&inboundmodel.InboundClient{ID: "test-app-id", AuthFlowID: "test-graph-id"}, nil)
	mockEntityProvider.EXPECT().GetEntityProfile(mock.Anything, "test-app-id").Return(
		&providers.Entity{ID: "test-app-id", Category: providers.EntityCategoryApp},
		(*entityprovider.EntityProviderError)(nil))

	// Engine receives a properly restored context — not the raw encrypted bytes
	mockEngine.EXPECT().Execute(mock.MatchedBy(func(ctx *EngineContext) bool {
		return ctx != nil && ctx.AppID == "test-app-id" && ctx.ExecutionID == existingExecutionID
	})).Return(FlowStep{Status: providers.FlowStatusIncomplete}, nil)

	mockStore.EXPECT().UpdateFlowContext(
		mock.MatchedBy(func(ctx context.Context) bool { return ctx.Value(txMarkerKey{}) == "tx" }),
		mock.AnythingOfType("FlowContextDB")).Return(nil)

	service := &flowExecService{
		flowStore:     mockStore,
		graphBuilder:  mockGraphBuilder,
		flowProvider:  mockFlowProvider,
		flowEngine:    mockEngine,
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil, nil, nil),
		transactioner: &stubTransactioner{},
		cryptoSvc:     mockCrypto,
		cfg:           testFlowExecCfg,
	}

	flowStep, svcErr := service.Execute(context.Background(), "test-app", existingExecutionID,
		string(providers.FlowTypeAuthentication), "", false, "submit", map[string]string{}, "", "", "", nil)

	assert.Nil(t, svcErr)
	assert.NotNil(t, flowStep)
	assert.Equal(t, providers.FlowStatusIncomplete, flowStep.Status)
}

func TestEncryptedContext_SensitiveFieldsHidden(t *testing.T) {
	// Verifies that after encryptEngineContext, sensitive fields (appId, userId, token, inputs)
	// are not visible in the encrypted bytes stored — matching the protection guarantee.
	_ = config.InitializeServerRuntime("/tmp/test", &config.Config{})

	testKey, _ := hex.DecodeString("2729a7928c79371e5f312167269294a14bb0660fd166b02a408a20fa73271580")

	mockConfigCryptoService := cryptomock.NewConfigCryptoProviderMock(t)
	mockConfigCryptoService.EXPECT().Encrypt(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, plaintext []byte) ([]byte, error) {
			ciphertext, _, err := cryptolib.Encrypt(
				testKey,
				&cryptolib.AlgorithmParams{Algorithm: cryptolib.AlgorithmAESGCM},
				plaintext,
			)
			if err != nil {
				return nil, err
			}
			encData := defaultkm.EncryptedData{
				Algorithm:  defaultkm.AESGCM,
				Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
				KeyID:      "test-kid",
			}
			return json.Marshal(encData)
		})

	mockCrypto := cryptomock.NewRuntimeCryptoProviderMock(t)
	mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(
			func(
				ctx context.Context,
				_ *providers.KeyRef,
				_ string,
				_ map[string]interface{},
				content []byte) ([]byte, *providers.CryptoDetails, error) {
				encrypted, encErr := mockConfigCryptoService.Encrypt(ctx, content)
				return encrypted, nil, encErr
			})

	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	testGraph := flowFactory.CreateGraph("test-graph-id", providers.FlowTypeAuthentication, 1)

	sensitiveAppID := "app-sensitive-99999"
	sensitiveUserID := "user-sensitive-88888"
	sensitiveInput := "sensitive-password-value"
	sensitiveRuntimeData := "sensitive-state-value"

	engineCtx := EngineContext{
		ExecutionID: "test-flow-id",
		AppID:       sensitiveAppID,
		FlowType:    providers.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          sensitiveUserID,
			Attributes:      map[string]interface{}{},
		},
		UserInputs:       map[string]string{"password": sensitiveInput},
		RuntimeData:      map[string]string{"state": sensitiveRuntimeData},
		ExecutionHistory: map[string]*providers.NodeExecutionRecord{},
		Graph:            testGraph,
	}

	svc := &flowExecService{cryptoSvc: mockCrypto, cfg: testFlowExecCfg}
	encryptedEngineCtx, err := svc.encryptEngineContext(context.Background(), &engineCtx)
	assert.NoError(t, err)

	// Stored context must be encrypted
	assert.True(t, isContextEncrypted(encryptedEngineCtx.Context),
		"stored context should have alg field indicating encryption")

	// Sensitive fields must not be visible in the raw stored bytes
	assert.NotContains(t, encryptedEngineCtx.Context, sensitiveAppID,
		"appId must not appear in encrypted context")
	assert.NotContains(t, encryptedEngineCtx.Context, sensitiveUserID,
		"userId must not appear in encrypted context")
	assert.NotContains(t, encryptedEngineCtx.Context, sensitiveInput,
		"user input must not appear in encrypted context")
	assert.NotContains(t, encryptedEngineCtx.Context, sensitiveRuntimeData,
		"runtime data must not appear in encrypted context")
}

func TestEncryptDecryptRoundTrip_AllFieldsPreserved(t *testing.T) {
	// Full encrypt → decrypt round trip through encryptEngineContext / getFlowContext decrypt path.
	// Verifies all context fields — including the auth token — survive the cycle intact.
	_ = config.InitializeServerRuntime("/tmp/test", &config.Config{})

	testKey, _ := hex.DecodeString("2729a7928c79371e5f312167269294a14bb0660fd166b02a408a20fa73271580")

	mockConfigCryptoService := cryptomock.NewConfigCryptoProviderMock(t)
	mockConfigCryptoService.EXPECT().Encrypt(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, plaintext []byte) ([]byte, error) {
			ciphertext, _, err := cryptolib.Encrypt(
				testKey,
				&cryptolib.AlgorithmParams{Algorithm: cryptolib.AlgorithmAESGCM},
				plaintext,
			)
			if err != nil {
				return nil, err
			}
			encData := defaultkm.EncryptedData{
				Algorithm:  defaultkm.AESGCM,
				Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
				KeyID:      "test-kid",
			}
			return json.Marshal(encData)
		})
	mockConfigCryptoService.EXPECT().Decrypt(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, encodedData []byte) ([]byte, error) {
			var encData defaultkm.EncryptedData
			if err := json.Unmarshal(encodedData, &encData); err != nil {
				return nil, err
			}
			ciphertext, err := base64.StdEncoding.DecodeString(encData.Ciphertext)
			if err != nil {
				return nil, err
			}
			return cryptolib.Decrypt(
				testKey,
				cryptolib.AlgorithmParams{Algorithm: cryptolib.AlgorithmAESGCM},
				ciphertext,
			)
		})

	mockCrypto := cryptomock.NewRuntimeCryptoProviderMock(t)
	mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(
			ctx context.Context,
			_ *providers.KeyRef,
			_ string,
			_ map[string]interface{},
			content []byte) ([]byte, *providers.CryptoDetails, error) {
			encrypted, encErr := mockConfigCryptoService.Encrypt(ctx, content)
			return encrypted, nil, encErr
		})
	mockCrypto.EXPECT().Decrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(
			ctx context.Context,
			_ *providers.KeyRef,
			_ string, _ map[string]interface{}, content []byte) ([]byte, error) {
			return mockConfigCryptoService.Decrypt(ctx, content)
		})

	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	testGraph := flowFactory.CreateGraph("test-graph-id", providers.FlowTypeAuthentication, 1)

	originalToken := "original-secret-token-value-xyz789"

	engineCtx := EngineContext{
		ExecutionID: "round-trip-flow-id",
		AppID:       "round-trip-app-id",
		FlowType:    providers.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          "round-trip-user-id",
			OUID:            "round-trip-org-id",
			UserType:        "standard",
			Token:           originalToken,
			Attributes:      map[string]interface{}{"email": "test@example.com"},
		},
		UserInputs:       map[string]string{"username": "testuser"},
		RuntimeData:      map[string]string{"state": "abc123"},
		ExecutionHistory: map[string]*providers.NodeExecutionRecord{},
		Graph:            testGraph,
	}

	svc := &flowExecService{cryptoSvc: mockCrypto, cfg: testFlowExecCfg}

	// Step 1: Encrypt (as storeContext / updateContext would)
	encryptedEngineCtx, err := svc.encryptEngineContext(context.Background(), &engineCtx)
	assert.NoError(t, err)
	assert.True(t, isContextEncrypted(encryptedEngineCtx.Context))

	// Step 2: Simulate getFlowContext decrypt path — call through the mock so RunAndReturn fires
	decryptedBytes, err := mockCrypto.Decrypt(
		context.Background(), nil,
		string(cryptolib.AlgorithmAESGCM), nil,
		[]byte(encryptedEngineCtx.Context))
	assert.NoError(t, err)

	restoredDB := &FlowContextDB{
		ExecutionID: encryptedEngineCtx.ExecutionID,
		Context:     string(decryptedBytes),
	}

	// Step 3: Convert back to EngineContext
	resultCtx, err := restoredDB.ToEngineContext(context.Background(), testGraph, nil)
	assert.NoError(t, err)

	// Verify all fields survived the round trip
	assert.Equal(t, engineCtx.ExecutionID, resultCtx.ExecutionID)
	assert.Equal(t, engineCtx.AppID, resultCtx.AppID)
	assert.True(t, resultCtx.AuthenticatedUser.IsAuthenticated)
	assert.Equal(t, engineCtx.AuthenticatedUser.UserID, resultCtx.AuthenticatedUser.UserID)
	assert.Equal(t, engineCtx.AuthenticatedUser.OUID, resultCtx.AuthenticatedUser.OUID)
	assert.Equal(t, engineCtx.AuthenticatedUser.UserType, resultCtx.AuthenticatedUser.UserType)
	assert.Equal(t, originalToken, resultCtx.AuthenticatedUser.Token,
		"token must survive the full encrypt-decrypt round trip")
	assert.Equal(t, len(engineCtx.UserInputs), len(resultCtx.UserInputs))
	assert.Equal(t, len(engineCtx.RuntimeData), len(resultCtx.RuntimeData))
}

func TestExecute_ContextDecryptionFailure(t *testing.T) {
	// Tests that when the stored flow context cannot be decrypted,
	// Execute returns an InternalServerError without proceeding further.
	mockStore := newFlowStoreInterfaceMock(t)
	mockCrypto := cryptomock.NewRuntimeCryptoProviderMock(t)
	mockCrypto.EXPECT().Decrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("decryption failed"))

	// Context looks encrypted (has "alg" field) but the ciphertext is invalid
	invalidCtx := &FlowContextDB{
		ExecutionID: existingExecutionID,
		Context:     `{"alg":"AES-GCM","ct":"not-valid-ciphertext!!!","kid":"key-1"}`,
	}
	mockStore.EXPECT().GetFlowContext(mock.Anything, existingExecutionID).Return(invalidCtx, nil)

	service := &flowExecService{
		flowStore: mockStore,
		cryptoSvc: mockCrypto,
		cfg:       testFlowExecCfg,
	}

	_, svcErr := service.Execute(context.Background(), "test-app", existingExecutionID,
		string(providers.FlowTypeAuthentication), "", false, "submit", map[string]string{}, "", "", "", nil)

	assert.NotNil(t, svcErr)
	assert.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)
}

func TestExecute_ContextDecryptionSuccess(t *testing.T) {
	// Tests that a plain-text stored context (decryption already handled by service before store)
	// is loaded and used to continue flow execution without error.
	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	testGraph := flowFactory.CreateGraph("test-graph-id", providers.FlowTypeAuthentication, 1)

	engineCtx := EngineContext{
		ExecutionID: existingExecutionID,
		AppID:       "test-app-id",
		FlowType:    providers.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{
			Attributes: map[string]interface{}{},
		},
		UserInputs:       map[string]string{},
		RuntimeData:      map[string]string{},
		ExecutionHistory: map[string]*providers.NodeExecutionRecord{},
		Graph:            testGraph,
	}
	storedCtx := &FlowContextDB{}
	err := storedCtx.FromEngineContext(engineCtx)
	assert.NoError(t, err)

	mockStore := newFlowStoreInterfaceMock(t)
	mockFlowProvider := NewFlowProviderMock(t)
	mockGraphBuilder := NewGraphBuilderInterfaceMock(t)
	mockEngine := newFlowEngineInterfaceMock(t)
	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(t)
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)
	mockCrypto := cryptomock.NewRuntimeCryptoProviderMock(t)
	mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]byte("encrypted-ctx"), nil, nil)

	mockStore.EXPECT().GetFlowContext(mock.Anything, existingExecutionID).Return(storedCtx, nil)
	mockFlowProvider.EXPECT().
		GetFlow(mock.Anything, "test-graph-id").
		Return(&providers.CompleteFlowDefinition{ID: "test-graph-id"}, nil)
	mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(testGraph, nil)
	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, "test-app-id").Return(
		&inboundmodel.InboundClient{ID: "test-app-id", AuthFlowID: "test-graph-id"}, nil)
	mockEntityProvider.EXPECT().GetEntityProfile(mock.Anything, "test-app-id").Return(
		&providers.Entity{ID: "test-app-id", Category: providers.EntityCategoryApp},
		(*entityprovider.EntityProviderError)(nil))
	challengeToken := "test-challenge-token"
	mockEngine.EXPECT().Execute(mock.MatchedBy(func(ctx *EngineContext) bool {
		return ctx != nil && ctx.ExecutionID == existingExecutionID
	})).Return(FlowStep{Status: providers.FlowStatusIncomplete}, nil)
	mockStore.EXPECT().UpdateFlowContext(
		mock.MatchedBy(func(ctx context.Context) bool { return ctx.Value(txMarkerKey{}) == "tx" }),
		mock.AnythingOfType("FlowContextDB")).Return(nil)

	service := &flowExecService{
		flowStore:     mockStore,
		graphBuilder:  mockGraphBuilder,
		flowProvider:  mockFlowProvider,
		flowEngine:    mockEngine,
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil, nil, nil),
		transactioner: &stubTransactioner{},
		cryptoSvc:     mockCrypto,
		cfg:           testFlowExecCfg,
	}

	flowStep, svcErr := service.Execute(context.Background(), "test-app", existingExecutionID,
		string(providers.FlowTypeAuthentication), "", false, "submit", map[string]string{}, challengeToken, "", "", nil)

	assert.Nil(t, svcErr)
	assert.NotNil(t, flowStep)
	assert.Equal(t, providers.FlowStatusIncomplete, flowStep.Status)
}

func TestExecute_ExistingFlowWithoutChallengeToken(t *testing.T) {
	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	testGraph := flowFactory.CreateGraph("test-graph-id", providers.FlowTypeAuthentication, 1)

	engineCtx := EngineContext{
		ExecutionID: existingExecutionID,
		AppID:       "test-app-id",
		FlowType:    providers.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{
			Attributes: map[string]interface{}{},
		},
		UserInputs:       map[string]string{},
		RuntimeData:      map[string]string{},
		ExecutionHistory: map[string]*providers.NodeExecutionRecord{},
		Graph:            testGraph,
	}
	storedCtx := &FlowContextDB{}
	err := storedCtx.FromEngineContext(engineCtx)
	assert.NoError(t, err)

	mockStore := newFlowStoreInterfaceMock(t)
	mockFlowProvider := NewFlowProviderMock(t)
	mockGraphBuilder := NewGraphBuilderInterfaceMock(t)
	mockEngine := newFlowEngineInterfaceMock(t)
	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(t)
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)

	mockStore.EXPECT().GetFlowContext(mock.Anything, existingExecutionID).Return(storedCtx, nil)
	mockFlowProvider.EXPECT().
		GetFlow(mock.Anything, "test-graph-id").
		Return(&providers.CompleteFlowDefinition{ID: "test-graph-id"}, nil)
	mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(testGraph, nil)
	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, "test-app-id").Return(
		&inboundmodel.InboundClient{ID: "test-app-id", AuthFlowID: "test-graph-id"}, nil)
	mockEntityProvider.EXPECT().GetEntityProfile(mock.Anything, "test-app-id").Return(
		&providers.Entity{ID: "test-app-id", Category: providers.EntityCategoryApp},
		(*entityprovider.EntityProviderError)(nil))

	mockCrypto := cryptomock.NewRuntimeCryptoProviderMock(t)
	mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]byte("encrypted-ctx"), nil, nil)

	mockEngine.EXPECT().Execute(mock.MatchedBy(func(ctx *EngineContext) bool {
		return ctx != nil && ctx.ExecutionID == existingExecutionID
	})).Return(FlowStep{Status: providers.FlowStatusIncomplete}, nil)
	mockStore.EXPECT().UpdateFlowContext(
		mock.MatchedBy(func(ctx context.Context) bool { return ctx.Value(txMarkerKey{}) == "tx" }),
		mock.AnythingOfType("FlowContextDB")).Return(nil)

	service := &flowExecService{
		flowStore:     mockStore,
		graphBuilder:  mockGraphBuilder,
		flowProvider:  mockFlowProvider,
		flowEngine:    mockEngine,
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil, nil, nil),
		transactioner: &stubTransactioner{},
		cryptoSvc:     mockCrypto,
		cfg:           testFlowExecCfg,
	}

	// Execute with empty challenge token
	flowStep, svcErr := service.Execute(context.Background(), "test-app", existingExecutionID,
		string(providers.FlowTypeAuthentication), "", false, "submit", map[string]string{}, "", "", "", nil)

	assert.Nil(t, svcErr)
	assert.NotNil(t, flowStep)
	assert.Equal(t, providers.FlowStatusIncomplete, flowStep.Status)
}

func TestExecute_ExistingFlowWithDifferentChallengeTokens(t *testing.T) {
	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	testGraph := flowFactory.CreateGraph("test-graph-id", providers.FlowTypeAuthentication, 1)

	tests := []struct {
		name            string
		challengeToken  string
		expectInContext string
	}{
		{
			name:            "with short token",
			challengeToken:  "abc123",
			expectInContext: "abc123",
		},
		{
			name: "with long token",
			challengeToken: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0." +
				"dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U",
			expectInContext: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0." +
				"dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U",
		},
		{
			name:            "with empty token",
			challengeToken:  "",
			expectInContext: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engineCtx := EngineContext{
				ExecutionID: existingExecutionID,
				AppID:       "test-app-id",
				FlowType:    providers.FlowTypeAuthentication,
				AuthenticatedUser: authncm.AuthenticatedUser{
					Attributes: map[string]interface{}{},
				},
				UserInputs:       map[string]string{},
				RuntimeData:      map[string]string{},
				ExecutionHistory: map[string]*providers.NodeExecutionRecord{},
				Graph:            testGraph,
			}
			storedCtx := &FlowContextDB{}
			err := storedCtx.FromEngineContext(engineCtx)
			assert.NoError(t, err)

			mockStore := newFlowStoreInterfaceMock(t)
			mockFlowProvider := NewFlowProviderMock(t)
			mockGraphBuilder := NewGraphBuilderInterfaceMock(t)
			mockEngine := newFlowEngineInterfaceMock(t)
			mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(t)
			mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)

			mockStore.EXPECT().GetFlowContext(mock.Anything, existingExecutionID).Return(storedCtx, nil)
			mockFlowProvider.EXPECT().
				GetFlow(mock.Anything, "test-graph-id").
				Return(&providers.CompleteFlowDefinition{ID: "test-graph-id"}, nil)
			mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(testGraph, nil)
			mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, "test-app-id").Return(
				&inboundmodel.InboundClient{ID: "test-app-id", AuthFlowID: "test-graph-id"}, nil)
			mockEntityProvider.EXPECT().GetEntityProfile(mock.Anything, "test-app-id").Return(
				&providers.Entity{ID: "test-app-id", Category: providers.EntityCategoryApp},
				(*entityprovider.EntityProviderError)(nil))

			mockCrypto := cryptomock.NewRuntimeCryptoProviderMock(t)
			mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return([]byte("encrypted-ctx"), nil, nil)

			mockEngine.EXPECT().Execute(mock.MatchedBy(func(ctx *EngineContext) bool {
				return ctx != nil && ctx.ExecutionID == existingExecutionID
			})).Return(FlowStep{Status: providers.FlowStatusIncomplete}, nil)
			mockStore.EXPECT().UpdateFlowContext(
				mock.MatchedBy(func(ctx context.Context) bool { return ctx.Value(txMarkerKey{}) == "tx" }),
				mock.AnythingOfType("FlowContextDB")).Return(nil)

			service := &flowExecService{
				flowStore:    mockStore,
				graphBuilder: mockGraphBuilder,
				flowProvider: mockFlowProvider,
				flowEngine:   mockEngine,
				actorProvider: actorprovider.Initialize(
					mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil, nil, nil),
				transactioner: &stubTransactioner{},
				cryptoSvc:     mockCrypto,
				cfg:           testFlowExecCfg,
			}

			flowStep, svcErr := service.Execute(context.Background(), "test-app", existingExecutionID,
				string(providers.FlowTypeAuthentication), "", false, "submit", map[string]string{},
				tt.challengeToken, "", "", nil)

			assert.Nil(t, svcErr)
			assert.NotNil(t, flowStep)
			assert.Equal(t, providers.FlowStatusIncomplete, flowStep.Status)
		})
	}
}

func TestExecute_EngineError_InvalidChallengeToken_PreservesContext(t *testing.T) {
	testConfig := &config.Config{}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)

	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	testGraph := flowFactory.CreateGraph("test-graph-id", providers.FlowTypeAuthentication, 1)

	engineCtx := EngineContext{
		ExecutionID: existingExecutionID,
		AppID:       "test-app-id",
		FlowType:    providers.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{
			Attributes: map[string]interface{}{},
		},
		UserInputs:       map[string]string{},
		RuntimeData:      map[string]string{},
		ExecutionHistory: map[string]*providers.NodeExecutionRecord{},
		Graph:            testGraph,
	}
	storedCtx := &FlowContextDB{}
	err := storedCtx.FromEngineContext(engineCtx)
	assert.NoError(t, err)

	mockStore := newFlowStoreInterfaceMock(t)
	mockFlowProvider := NewFlowProviderMock(t)
	mockGraphBuilder := NewGraphBuilderInterfaceMock(t)
	mockEngine := newFlowEngineInterfaceMock(t)
	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(t)
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)
	mockCrypto := cryptomock.NewRuntimeCryptoProviderMock(t)

	mockStore.EXPECT().GetFlowContext(mock.Anything, existingExecutionID).Return(storedCtx, nil)
	mockFlowProvider.EXPECT().
		GetFlow(mock.Anything, "test-graph-id").
		Return(&providers.CompleteFlowDefinition{ID: "test-graph-id"}, nil)
	mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(testGraph, nil)
	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, "test-app-id").Return(
		&inboundmodel.InboundClient{ID: "test-app-id", AuthFlowID: "test-graph-id"}, nil)
	mockEntityProvider.EXPECT().GetEntityProfile(mock.Anything, "test-app-id").Return(
		&providers.Entity{ID: "test-app-id", Category: providers.EntityCategoryApp},
		(*entityprovider.EntityProviderError)(nil))
	mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]byte("encrypted-ctx"), nil, nil)

	// Engine returns challenge token error as a FlowStep with ERROR status (interceptor-based).
	challengeTokenErr := interceptor.ErrorChallengeTokenInvalid
	mockEngine.EXPECT().Execute(mock.Anything).Return(
		FlowStep{Status: providers.FlowStatusIncomplete, Error: &challengeTokenErr}, nil)
	// DeleteFlowContext must NOT be called — flow must be preserved for retry.
	// UpdateFlowContext IS called because the engine returned successfully.
	mockStore.EXPECT().UpdateFlowContext(
		mock.MatchedBy(func(ctx context.Context) bool { return ctx.Value(txMarkerKey{}) == "tx" }),
		mock.AnythingOfType("FlowContextDB")).Return(nil)

	service := &flowExecService{
		flowStore:     mockStore,
		graphBuilder:  mockGraphBuilder,
		flowProvider:  mockFlowProvider,
		flowEngine:    mockEngine,
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil, nil, nil),
		transactioner: &stubTransactioner{},
		cryptoSvc:     mockCrypto,
		cfg:           testFlowExecCfg,
	}

	flowStep, svcErr := service.Execute(context.Background(), "test-app", existingExecutionID,
		string(providers.FlowTypeAuthentication), "", false, "submit", map[string]string{}, "wrong-token", "", "", nil)

	assert.Nil(t, svcErr)
	assert.NotNil(t, flowStep)
	assert.Equal(t, providers.FlowStatusIncomplete, flowStep.Status)
	assert.NotNil(t, flowStep.Error)
	assert.Equal(t, interceptor.ErrorChallengeTokenInvalid.Code, flowStep.Error.Code)
}

func TestExecute_EngineError_NonChallengeToken_RemovesContext(t *testing.T) {
	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	testGraph := flowFactory.CreateGraph("test-graph-id", providers.FlowTypeAuthentication, 1)

	engineCtx := EngineContext{
		ExecutionID: existingExecutionID,
		AppID:       "test-app-id",
		FlowType:    providers.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{
			Attributes: map[string]interface{}{},
		},
		UserInputs:       map[string]string{},
		RuntimeData:      map[string]string{},
		ExecutionHistory: map[string]*providers.NodeExecutionRecord{},
		Graph:            testGraph,
	}
	storedCtx := &FlowContextDB{}
	err := storedCtx.FromEngineContext(engineCtx)
	assert.NoError(t, err)

	mockStore := newFlowStoreInterfaceMock(t)
	mockFlowProvider := NewFlowProviderMock(t)
	mockGraphBuilder := NewGraphBuilderInterfaceMock(t)
	mockEngine := newFlowEngineInterfaceMock(t)
	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(t)
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)

	mockStore.EXPECT().GetFlowContext(mock.Anything, existingExecutionID).Return(storedCtx, nil)
	mockFlowProvider.EXPECT().
		GetFlow(mock.Anything, "test-graph-id").
		Return(&providers.CompleteFlowDefinition{ID: "test-graph-id"}, nil)
	mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(testGraph, nil)
	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, "test-app-id").Return(
		&inboundmodel.InboundClient{ID: "test-app-id", AuthFlowID: "test-graph-id"}, nil)
	mockEntityProvider.EXPECT().GetEntityProfile(mock.Anything, "test-app-id").Return(
		&providers.Entity{ID: "test-app-id", Category: providers.EntityCategoryApp},
		(*entityprovider.EntityProviderError)(nil))

	otherErr := &tidcommon.ServiceError{
		Code: "FES-9999",
		Type: tidcommon.ServerErrorType,
		Error: tidcommon.I18nMessage{
			Key:          "error.flowexecservice.engine_error",
			DefaultValue: "some other engine error",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowexecservice.engine_error_description",
			DefaultValue: "some other engine error",
		},
	}
	mockEngine.EXPECT().Execute(mock.Anything).Return(FlowStep{}, otherErr)
	// DeleteFlowContext MUST be called — non-challenge-token errors remove the context
	mockStore.EXPECT().DeleteFlowContext(
		mock.MatchedBy(func(ctx context.Context) bool { return ctx.Value(txMarkerKey{}) == "tx" }),
		existingExecutionID).Return(nil)

	service := &flowExecService{
		flowStore:     mockStore,
		graphBuilder:  mockGraphBuilder,
		flowProvider:  mockFlowProvider,
		flowEngine:    mockEngine,
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil, nil, nil),
		transactioner: &stubTransactioner{},
		cfg:           testFlowExecCfg,
	}

	flowStep, svcErr := service.Execute(context.Background(), "test-app", existingExecutionID,
		string(providers.FlowTypeAuthentication), "", false, "submit", map[string]string{}, "valid-token", "", "", nil)

	assert.NotNil(t, svcErr)
	assert.Equal(t, otherErr.Code, svcErr.Code)
	assert.Nil(t, flowStep)
}

func TestExecute_EngineError_NewFlow_ContextNeverRemoved(t *testing.T) {
	testConfig := &config.Config{}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)

	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	testGraph := flowFactory.CreateGraph("auth-graph-1", providers.FlowTypeAuthentication, 1)

	mockStore := newFlowStoreInterfaceMock(t)
	mockFlowProvider := NewFlowProviderMock(t)
	mockGraphBuilder := NewGraphBuilderInterfaceMock(t)
	mockEngine := newFlowEngineInterfaceMock(t)
	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(t)
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)
	mockCrypto := cryptomock.NewRuntimeCryptoProviderMock(t)

	mockAuthn := managermock.NewAuthnProviderManagerMock(t)
	mockInboundClient.EXPECT().GetOAuthProfileByEntityID(mock.Anything, "test-app").Return(nil, nil)
	mockAuthn.EXPECT().AuthenticateUser(mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, nil, nil)
	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, "test-app").Return(
		&inboundmodel.InboundClient{ID: "test-app", AuthFlowID: "auth-graph-1"}, nil).Times(3)
	mockEntityProvider.EXPECT().GetEntityProfile(mock.Anything, "test-app").Return(
		&providers.Entity{ID: "test-app", Category: providers.EntityCategoryApp},
		(*entityprovider.EntityProviderError)(nil))
	mockFlowProvider.EXPECT().
		GetFlow(mock.Anything, "auth-graph-1").
		Return(&providers.CompleteFlowDefinition{ID: "auth-graph-1"}, nil)
	mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(testGraph, nil)
	mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]byte("encrypted-ctx"), nil, nil)

	// Engine returns challenge token error as a FlowStep with ERROR status (interceptor-based).
	challengeTokenErr := interceptor.ErrorChallengeTokenInvalid
	mockEngine.EXPECT().Execute(mock.Anything).Return(
		FlowStep{Status: providers.FlowStatusIncomplete, Error: &challengeTokenErr}, nil)
	// DeleteFlowContext must NOT be called — new flows have no persisted context to clean up.
	// StoreFlowContext IS called because the engine returned a non-complete status.
	mockStore.EXPECT().StoreFlowContext(
		mock.MatchedBy(func(ctx context.Context) bool { return ctx.Value(txMarkerKey{}) == "tx" }),
		mock.AnythingOfType("FlowContextDB"), mock.Anything).Return(nil)

	service := &flowExecService{
		flowStore:     mockStore,
		graphBuilder:  mockGraphBuilder,
		flowProvider:  mockFlowProvider,
		flowEngine:    mockEngine,
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, mockAuthn, nil, nil, nil),
		transactioner: &stubTransactioner{},
		cryptoSvc:     mockCrypto,
		cfg:           testFlowExecCfg,
	}

	// Pass empty executionID to indicate a new flow
	flowStep, svcErr := service.Execute(context.Background(), "test-app", "",
		string(providers.FlowTypeAuthentication), "", false, "submit", map[string]string{}, "", "valid-secret", "", nil)

	assert.Nil(t, svcErr)
	assert.NotNil(t, flowStep)
	assert.Equal(t, providers.FlowStatusIncomplete, flowStep.Status)
	assert.NotNil(t, flowStep.Error)
	assert.Equal(t, interceptor.ErrorChallengeTokenInvalid.Code, flowStep.Error.Code)
}

// --- BuildApplication (via actorprovider) ---

func newBuildAppProvider(
	t *testing.T,
) (providers.ActorProvider, *inboundclientmock.InboundClientServiceInterfaceMock,
	*entityprovidermock.EntityProviderInterfaceMock) {
	mockInbound := inboundclientmock.NewInboundClientServiceInterfaceMock(t)
	mockEP := entityprovidermock.NewEntityProviderInterfaceMock(t)
	return actorprovider.Initialize(mockInbound, mockEP, noopAuthnMgr(), nil, nil, nil), mockInbound, mockEP
}

func TestBuildApplication_InboundClientNotFound(t *testing.T) {
	provider, mockInbound, _ := newBuildAppProvider(t)
	mockInbound.EXPECT().GetInboundClientByEntityID(mock.Anything, "app-x").
		Return((*inboundmodel.InboundClient)(nil), inboundclient.ErrInboundClientNotFound)

	app, svcErr := actorprovider.BuildApplication(context.Background(), provider, "app-x")

	assert.Nil(t, app)
	assert.Equal(t, actorprovider.ErrorActorNotFound.Code, svcErr.Code)
}

func TestBuildApplication_InboundClientStoreError(t *testing.T) {
	provider, mockInbound, _ := newBuildAppProvider(t)
	mockInbound.EXPECT().GetInboundClientByEntityID(mock.Anything, "app-x").
		Return((*inboundmodel.InboundClient)(nil), errors.New("boom"))

	app, svcErr := actorprovider.BuildApplication(context.Background(), provider, "app-x")

	assert.Nil(t, app)
	assert.NotNil(t, svcErr)
	assert.NotEqual(t, actorprovider.ErrorActorNotFound.Code, svcErr.Code)
}

func TestBuildApplication_EntityLoadError(t *testing.T) {
	provider, mockInbound, mockEP := newBuildAppProvider(t)
	mockInbound.EXPECT().GetInboundClientByEntityID(mock.Anything, "app-x").
		Return(&inboundmodel.InboundClient{ID: "app-x"}, nil)
	mockEP.EXPECT().GetEntityProfile(mock.Anything, "app-x").Return(
		(*providers.Entity)(nil),
		entityprovider.NewEntityProviderError("INTERNAL_ERROR", "boom", ""))

	app, svcErr := actorprovider.BuildApplication(context.Background(), provider, "app-x")

	assert.Nil(t, app)
	assert.NotNil(t, svcErr)
}

func TestBuildApplication_EntityNotFound_ReturnsAppWithoutEntityFields(t *testing.T) {
	provider, mockInbound, mockEP := newBuildAppProvider(t)
	mockInbound.EXPECT().GetInboundClientByEntityID(mock.Anything, "app-x").
		Return(&inboundmodel.InboundClient{
			ID:               "app-x",
			AllowedUserTypes: []string{"customer"},
		}, nil)
	mockEP.EXPECT().GetEntityProfile(mock.Anything, "app-x").Return(
		(*providers.Entity)(nil),
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "missing", ""))

	app, svcErr := actorprovider.BuildApplication(context.Background(), provider, "app-x")

	assert.Nil(t, svcErr)
	assert.NotNil(t, app)
	assert.Equal(t, "app-x", app.ID)
	assert.Equal(t, "", app.Name)
	assert.Equal(t, []string{"customer"}, app.AllowedUserTypes)
	assert.Empty(t, app.InboundAuthConfig)
}

func TestBuildApplication_Success_WithMetadataAndClientID(t *testing.T) {
	provider, mockInbound, mockEP := newBuildAppProvider(t)
	mockInbound.EXPECT().GetInboundClientByEntityID(mock.Anything, "app-x").
		Return(&inboundmodel.InboundClient{
			ID: "app-x",
			Properties: map[string]interface{}{
				"metadata": map[string]interface{}{"tier": "gold"},
			},
		}, nil)
	sysAttrs := []byte(`{"name":"Acme","clientId":"client-1"}`)
	mockEP.EXPECT().GetEntityProfile(mock.Anything, "app-x").Return(
		&providers.Entity{
			ID:               "app-x",
			Category:         providers.EntityCategoryApp,
			SystemAttributes: sysAttrs,
		},
		(*entityprovider.EntityProviderError)(nil))

	app, svcErr := actorprovider.BuildApplication(context.Background(), provider, "app-x")

	assert.Nil(t, svcErr)
	assert.NotNil(t, app)
	assert.Equal(t, "Acme", app.Name)
	assert.Equal(t, map[string]interface{}{"tier": "gold"}, app.Metadata)
	assert.Len(t, app.InboundAuthConfig, 1)
	assert.Equal(t, providers.OAuthInboundAuthType, app.InboundAuthConfig[0].Type)
	assert.Equal(t, "client-1", app.InboundAuthConfig[0].OAuthConfig.ClientID)
}

func TestEncryptEngineContext_SerializeError(t *testing.T) {
	// Triggers line 478: FromEngineContext fails because Attributes contains an
	// unjsonifiable value (channel), wrapping the error with "failed to serialize engine context".
	engineCtx := &EngineContext{
		AuthenticatedUser: authncm.AuthenticatedUser{
			Attributes: map[string]interface{}{
				"bad": make(chan int), // channels cannot be marshaled to JSON
			},
		},
		UserInputs:       map[string]string{},
		RuntimeData:      map[string]string{},
		ExecutionHistory: map[string]*providers.NodeExecutionRecord{},
	}

	svc := &flowExecService{cfg: testFlowExecCfg}
	_, err := svc.encryptEngineContext(context.Background(), engineCtx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to serialize engine context")
}

func TestEncryptEngineContext_EncryptError(t *testing.T) {
	// Triggers line 483: serialization succeeds but cryptoSvc.Encrypt returns an error,
	// wrapping it with "failed to encrypt context".
	testConfig := &config.Config{}
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)

	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	testGraph := flowFactory.CreateGraph("test-graph-id", providers.FlowTypeAuthentication, 1)

	engineCtx := &EngineContext{
		ExecutionID: "exec-id",
		AppID:       "app-id",
		FlowType:    providers.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{
			Attributes: map[string]interface{}{},
		},
		UserInputs:       map[string]string{},
		RuntimeData:      map[string]string{},
		ExecutionHistory: map[string]*providers.NodeExecutionRecord{},
		Graph:            testGraph,
	}

	mockCrypto := cryptomock.NewRuntimeCryptoProviderMock(t)
	mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil, errors.New("encryption backend unavailable"))

	svc := &flowExecService{cryptoSvc: mockCrypto, cfg: testFlowExecCfg}
	_, err := svc.encryptEngineContext(context.Background(), engineCtx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to encrypt context")
}

func TestInitiateAndExecute_NilContext(t *testing.T) {
	svc := &flowExecService{cfg: testFlowExecCfg}
	step, err := svc.InitiateAndExecute(context.Background(), nil)
	assert.NotNil(t, err)
	assert.Nil(t, step)
	assert.Equal(t, "FES-1008", err.Code)
}

func TestInitiateAndExecute_EmptyFlowType(t *testing.T) {
	svc := &flowExecService{cfg: testFlowExecCfg}
	step, err := svc.InitiateAndExecute(context.Background(), &FlowInitContext{
		ApplicationID: "app-1",
		FlowType:      "",
	})
	assert.NotNil(t, err)
	assert.Nil(t, step)
	assert.Equal(t, "FES-1008", err.Code)
}

func TestInitiateAndExecute_InvalidFlowType(t *testing.T) {
	svc := &flowExecService{cfg: testFlowExecCfg}
	step, err := svc.InitiateAndExecute(context.Background(), &FlowInitContext{
		ApplicationID: "app-1",
		FlowType:      "INVALID",
	})
	assert.NotNil(t, err)
	assert.Nil(t, step)
}

func TestInitiateAndExecute_CustomExpiryUsed(t *testing.T) {
	appID := "test-app-custom-expiry"

	testConfig := &config.Config{}
	_ = config.InitializeServerRuntime("/tmp/test-ia-expiry", testConfig)
	defer config.ResetServerRuntime()

	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	testGraph := flowFactory.CreateGraph("auth-graph-expiry", providers.FlowTypeAuthentication, 1)

	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(t)
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)
	mockFlowProvider := NewFlowProviderMock(t)
	mockGraphBuilder := NewGraphBuilderInterfaceMock(t)
	mockCrypto := cryptomock.NewRuntimeCryptoProviderMock(t)
	mockStore := newFlowStoreInterfaceMock(t)
	mockEngineInner := newFlowEngineInterfaceMock(t)

	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, appID).Return(
		&inboundmodel.InboundClient{ID: appID, AuthFlowID: "auth-graph-expiry"}, nil)
	mockEntityProvider.EXPECT().GetEntityProfile(mock.Anything, appID).Return(
		&providers.Entity{ID: appID, Category: providers.EntityCategoryApp}, nil)
	mockFlowProvider.EXPECT().
		GetFlow(mock.Anything, "auth-graph-expiry").
		Return(&providers.CompleteFlowDefinition{ID: "auth-graph-expiry"}, nil)
	mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(testGraph, nil)
	mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]byte("encrypted"), nil, nil)

	const customExpiry int64 = 300
	mockStore.EXPECT().StoreFlowContext(mock.Anything, mock.Anything,
		mock.MatchedBy(func(exp int64) bool { return exp == customExpiry })).
		Return(nil)
	mockEngineInner.EXPECT().Execute(mock.Anything).
		Return(FlowStep{Status: providers.FlowStatusIncomplete}, nil)

	svc := &flowExecService{
		flowStore:     mockStore,
		graphBuilder:  mockGraphBuilder,
		flowProvider:  mockFlowProvider,
		flowEngine:    mockEngineInner,
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil, nil, nil),
		transactioner: &stubTransactioner{},
		cryptoSvc:     mockCrypto,
		cfg:           testFlowExecCfg,
	}

	step, err := svc.InitiateAndExecute(context.Background(), &FlowInitContext{
		ApplicationID: appID,
		FlowType:      "AUTHENTICATION",
		ExpirySeconds: customExpiry,
	})

	assert.Nil(t, err)
	assert.NotNil(t, step)
}

func (s *ServiceTestSuite) TestInitiationRefusesAHeldApplication() {
	appID := "test-app-held"

	_ = config.InitializeServerRuntime("/tmp/test-ia-held", &config.Config{})
	defer config.ResetServerRuntime()

	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	newService := func() *flowExecService {
		mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(s.T())
		mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(s.T())
		mockFlowProvider := NewFlowProviderMock(s.T())
		mockGraphBuilder := NewGraphBuilderInterfaceMock(s.T())
		governance := identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(s.T())

		mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, appID).Return(
			&inboundmodel.InboundClient{ID: appID, AuthFlowID: "auth-graph-held"}, nil)
		mockEntityProvider.EXPECT().GetEntityProfile(mock.Anything, appID).Return(
			&providers.Entity{ID: appID, Category: providers.EntityCategoryApp}, nil)
		mockFlowProvider.EXPECT().GetFlow(mock.Anything, "auth-graph-held").
			Return(&providers.CompleteFlowDefinition{ID: "auth-graph-held"}, nil)
		mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).
			Return(flowFactory.CreateGraph("auth-graph-held", providers.FlowTypeAuthentication, 1), nil)
		governance.EXPECT().EvaluateAccess(mock.Anything, appID, providers.AccessPlaneApplication,
			providers.AuthenticationMethod("")).
			Return(providers.AccessDecision{Admitted: false}, nil).Once()

		// No store, crypto or engine mock: a held application must stop before any of them.
		return &flowExecService{
			graphBuilder: mockGraphBuilder,
			flowProvider: mockFlowProvider,
			actorProvider: actorprovider.Initialize(
				mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil, nil, nil),
			transactioner: &stubTransactioner{},
			cfg:           testFlowExecCfg,
			governance:    governance,
		}
	}
	initContext := &FlowInitContext{ApplicationID: appID, FlowType: "AUTHENTICATION"}

	s.Run("InitiateFlow", func() {
		executionID, svcErr := newService().InitiateFlow(context.Background(), initContext)
		s.Empty(executionID)
		s.Require().NotNil(svcErr)
		s.Equal(ErrorFlowApplicationNotUsable.Code, svcErr.Code)
	})

	s.Run("InitiateAndExecute", func() {
		step, svcErr := newService().InitiateAndExecute(context.Background(), initContext)
		s.Nil(step)
		s.Require().NotNil(svcErr)
		s.Equal(ErrorFlowApplicationNotUsable.Code, svcErr.Code)
	})
}

func TestInitiateAndExecute_ZeroExpiryUsesDefault(t *testing.T) {
	appID := "test-app-default-expiry"

	testConfig := &config.Config{}
	_ = config.InitializeServerRuntime("/tmp/test-ia-defexp", testConfig)
	defer config.ResetServerRuntime()

	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	testGraph := flowFactory.CreateGraph("auth-graph-defexp", providers.FlowTypeAuthentication, 1)

	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(t)
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)
	mockFlowProvider := NewFlowProviderMock(t)
	mockGraphBuilder := NewGraphBuilderInterfaceMock(t)
	mockCrypto := cryptomock.NewRuntimeCryptoProviderMock(t)
	mockStore := newFlowStoreInterfaceMock(t)
	mockEngineInner := newFlowEngineInterfaceMock(t)

	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, appID).Return(
		&inboundmodel.InboundClient{ID: appID, AuthFlowID: "auth-graph-defexp"}, nil)
	mockEntityProvider.EXPECT().GetEntityProfile(mock.Anything, appID).Return(
		&providers.Entity{ID: appID, Category: providers.EntityCategoryApp}, nil)
	mockFlowProvider.EXPECT().
		GetFlow(mock.Anything, "auth-graph-defexp").
		Return(&providers.CompleteFlowDefinition{ID: "auth-graph-defexp"}, nil)
	mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(testGraph, nil)
	mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]byte("encrypted"), nil, nil)
	mockStore.EXPECT().StoreFlowContext(mock.Anything, mock.Anything,
		mock.MatchedBy(func(exp int64) bool { return exp == int64(3600) })).
		Return(nil)
	mockEngineInner.EXPECT().Execute(mock.Anything).
		Return(FlowStep{Status: providers.FlowStatusIncomplete}, nil)

	svc := &flowExecService{
		flowStore:     mockStore,
		graphBuilder:  mockGraphBuilder,
		flowProvider:  mockFlowProvider,
		flowEngine:    mockEngineInner,
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil, nil, nil),
		transactioner: &stubTransactioner{},
		cryptoSvc:     mockCrypto,
		cfg:           testFlowExecCfg,
	}

	step, err := svc.InitiateAndExecute(context.Background(), &FlowInitContext{
		ApplicationID: appID,
		FlowType:      "AUTHENTICATION",
	})

	assert.Nil(t, err)
	assert.NotNil(t, step)
}

func TestInitiateAndExecute_EmptyAppID(t *testing.T) {
	svc := &flowExecService{cfg: testFlowExecCfg}
	step, err := svc.InitiateAndExecute(context.Background(), &FlowInitContext{
		ApplicationID: "",
		FlowType:      "AUTHENTICATION",
	})
	assert.NotNil(t, err)
	assert.Nil(t, step)
	assert.Equal(t, "FES-1008", err.Code)
}

func TestInitiateAndExecute_InitialInputsAndRuntimeData(t *testing.T) {
	appID := "test-app-ia"

	testConfig := &config.Config{}
	_ = config.InitializeServerRuntime("/tmp/test-ia", testConfig)
	defer config.ResetServerRuntime()

	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	testGraph := flowFactory.CreateGraph("auth-graph-ia", providers.FlowTypeAuthentication, 1)

	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(t)
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)
	mockFlowProvider := NewFlowProviderMock(t)
	mockGraphBuilder := NewGraphBuilderInterfaceMock(t)
	mockCrypto := cryptomock.NewRuntimeCryptoProviderMock(t)
	mockStore := newFlowStoreInterfaceMock(t)
	mockEngineInner := newFlowEngineInterfaceMock(t)

	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, appID).Return(
		&inboundmodel.InboundClient{ID: appID, AuthFlowID: "auth-graph-ia"}, nil)
	mockEntityProvider.EXPECT().GetEntityProfile(mock.Anything, appID).Return(
		&providers.Entity{ID: appID, Category: providers.EntityCategoryApp}, nil)
	mockFlowProvider.EXPECT().
		GetFlow(mock.Anything, "auth-graph-ia").
		Return(&providers.CompleteFlowDefinition{ID: "auth-graph-ia"}, nil)
	mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(testGraph, nil)
	mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]byte("encrypted"), nil, nil)
	mockStore.EXPECT().StoreFlowContext(mock.Anything, mock.Anything, mock.Anything).Return(nil)

	var capturedCtx *EngineContext
	mockEngineInner.EXPECT().Execute(mock.MatchedBy(func(ctx *EngineContext) bool {
		capturedCtx = ctx
		return true
	})).Return(FlowStep{Status: providers.FlowStatusIncomplete}, nil)

	svc := &flowExecService{
		flowStore:     mockStore,
		graphBuilder:  mockGraphBuilder,
		flowProvider:  mockFlowProvider,
		flowEngine:    mockEngineInner,
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil, nil, nil),
		transactioner: &stubTransactioner{},
		cryptoSvc:     mockCrypto,
		cfg:           testFlowExecCfg,
	}

	step, err := svc.InitiateAndExecute(context.Background(), &FlowInitContext{
		ApplicationID: appID,
		FlowType:      "AUTHENTICATION",
		RuntimeData:   map[string]string{"clientId": "c1"},
		InitialInputs: map[string]string{"username": "alice"},
	})

	assert.Nil(t, err)
	assert.NotNil(t, step)
	assert.Equal(t, providers.FlowStatusIncomplete, step.Status)
	assert.Equal(t, "alice", capturedCtx.UserInputs["username"])
	assert.Equal(t, "c1", capturedCtx.RuntimeData["clientId"])
}

func TestInitiateAndExecute_FlowComplete_ContextNotStored(t *testing.T) {
	appID := "test-app-complete"

	testConfig := &config.Config{}
	_ = config.InitializeServerRuntime("/tmp/test-ia2", testConfig)
	defer config.ResetServerRuntime()

	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	testGraph := flowFactory.CreateGraph("auth-graph-complete", providers.FlowTypeAuthentication, 1)

	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(t)
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)
	mockFlowProvider := NewFlowProviderMock(t)
	mockGraphBuilder := NewGraphBuilderInterfaceMock(t)
	mockCrypto := cryptomock.NewRuntimeCryptoProviderMock(t)
	mockStore := newFlowStoreInterfaceMock(t)
	mockEngineInner := newFlowEngineInterfaceMock(t)

	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, appID).Return(
		&inboundmodel.InboundClient{ID: appID, AuthFlowID: "auth-graph-complete"}, nil)
	mockEntityProvider.EXPECT().GetEntityProfile(mock.Anything, appID).Return(
		&providers.Entity{ID: appID, Category: providers.EntityCategoryApp}, nil)
	mockFlowProvider.EXPECT().
		GetFlow(mock.Anything, "auth-graph-complete").
		Return(&providers.CompleteFlowDefinition{ID: "auth-graph-complete"}, nil)
	mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(testGraph, nil)

	mockEngineInner.EXPECT().Execute(mock.Anything).
		Return(FlowStep{Status: providers.FlowStatusComplete}, nil)

	svc := &flowExecService{
		flowStore:     mockStore,
		graphBuilder:  mockGraphBuilder,
		flowProvider:  mockFlowProvider,
		flowEngine:    mockEngineInner,
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil, nil, nil),
		transactioner: &stubTransactioner{},
		cryptoSvc:     mockCrypto,
		cfg:           testFlowExecCfg,
	}

	step, err := svc.InitiateAndExecute(context.Background(), &FlowInitContext{
		ApplicationID: appID,
		FlowType:      "AUTHENTICATION",
	})

	assert.Nil(t, err)
	assert.NotNil(t, step)
	assert.Equal(t, providers.FlowStatusComplete, step.Status)
	// StoreFlowContext must NOT be called when flow completes
	mockStore.AssertNotCalled(t, "StoreFlowContext")
}

func TestInitiateAndExecute_EngineError(t *testing.T) {
	appID := "test-app-eng-err"

	testConfig := &config.Config{}
	_ = config.InitializeServerRuntime("/tmp/test-ia3", testConfig)
	defer config.ResetServerRuntime()

	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	testGraph := flowFactory.CreateGraph("auth-graph-ee", providers.FlowTypeAuthentication, 1)

	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(t)
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)
	mockFlowProvider := NewFlowProviderMock(t)
	mockGraphBuilder := NewGraphBuilderInterfaceMock(t)
	mockCrypto := cryptomock.NewRuntimeCryptoProviderMock(t)
	mockStore := newFlowStoreInterfaceMock(t)
	mockEngineInner := newFlowEngineInterfaceMock(t)

	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, appID).Return(
		&inboundmodel.InboundClient{ID: appID, AuthFlowID: "auth-graph-ee"}, nil)
	mockEntityProvider.EXPECT().GetEntityProfile(mock.Anything, appID).Return(
		&providers.Entity{ID: appID, Category: providers.EntityCategoryApp}, nil)
	mockFlowProvider.EXPECT().
		GetFlow(mock.Anything, "auth-graph-ee").
		Return(&providers.CompleteFlowDefinition{ID: "auth-graph-ee"}, nil)
	mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(testGraph, nil)

	engineErr := &tidcommon.ServiceError{Code: "ENG-1"}
	mockEngineInner.EXPECT().Execute(mock.Anything).Return(FlowStep{}, engineErr)

	svc := &flowExecService{
		flowStore:     mockStore,
		graphBuilder:  mockGraphBuilder,
		flowProvider:  mockFlowProvider,
		flowEngine:    mockEngineInner,
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil, nil, nil),
		transactioner: &stubTransactioner{},
		cryptoSvc:     mockCrypto,
		cfg:           testFlowExecCfg,
	}

	step, err := svc.InitiateAndExecute(context.Background(), &FlowInitContext{
		ApplicationID: appID,
		FlowType:      "AUTHENTICATION",
	})

	assert.NotNil(t, err)
	assert.Nil(t, step)
	assert.Equal(t, "ENG-1", err.Code)
}

func TestInitiateAndExecute_StoreError_ReturnsError(t *testing.T) {
	appID := "test-app-store-err"

	testConfig := &config.Config{}
	_ = config.InitializeServerRuntime("/tmp/test-ia-store", testConfig)
	defer config.ResetServerRuntime()

	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	testGraph := flowFactory.CreateGraph("auth-graph-se", providers.FlowTypeAuthentication, 1)

	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(t)
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)
	mockFlowProvider := NewFlowProviderMock(t)
	mockGraphBuilder := NewGraphBuilderInterfaceMock(t)
	mockCrypto := cryptomock.NewRuntimeCryptoProviderMock(t)
	mockStore := newFlowStoreInterfaceMock(t)
	mockEngineInner := newFlowEngineInterfaceMock(t)

	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, appID).Return(
		&inboundmodel.InboundClient{ID: appID, AuthFlowID: "auth-graph-se"}, nil)
	mockEntityProvider.EXPECT().GetEntityProfile(mock.Anything, appID).Return(
		&providers.Entity{ID: appID, Category: providers.EntityCategoryApp}, nil)
	mockFlowProvider.EXPECT().
		GetFlow(mock.Anything, "auth-graph-se").
		Return(&providers.CompleteFlowDefinition{ID: "auth-graph-se"}, nil)
	mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(testGraph, nil)
	mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]byte("encrypted"), nil, nil)
	mockStore.EXPECT().StoreFlowContext(mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("store failed"))

	mockEngineInner.EXPECT().Execute(mock.Anything).
		Return(FlowStep{Status: providers.FlowStatusIncomplete}, nil)

	svc := &flowExecService{
		flowStore:     mockStore,
		graphBuilder:  mockGraphBuilder,
		flowProvider:  mockFlowProvider,
		flowEngine:    mockEngineInner,
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil, nil, nil),
		transactioner: &stubTransactioner{},
		cryptoSvc:     mockCrypto,
		cfg:           testFlowExecCfg,
	}

	step, err := svc.InitiateAndExecute(context.Background(), &FlowInitContext{
		ApplicationID: appID,
		FlowType:      "AUTHENTICATION",
	})

	assert.NotNil(t, err)
	assert.Nil(t, step)
}

func (s *ServiceTestSuite) TestGetFlowGraph_RegistrationAndRecovery() {
	appID := "test-app-flows"

	testConfig := &config.Config{}
	_ = config.InitializeServerRuntime("/tmp/test-flow-graph", testConfig)

	tests := []struct {
		name          string
		flowType      providers.FlowType
		client        *inboundmodel.InboundClient
		expectedGraph string
		expectedCode  string
	}{
		{
			name:     "registration flow enabled",
			flowType: providers.FlowTypeRegistration,
			client: &inboundmodel.InboundClient{
				ID:                        appID,
				IsRegistrationFlowEnabled: true,
				RegistrationFlowID:        "reg-graph-1",
			},
			expectedGraph: "reg-graph-1",
		},
		{
			name:     "registration flow disabled",
			flowType: providers.FlowTypeRegistration,
			client: &inboundmodel.InboundClient{
				ID:                        appID,
				IsRegistrationFlowEnabled: false,
			},
			expectedCode: ErrorRegistrationFlowDisabled.Code,
		},
		{
			name:     "recovery flow enabled",
			flowType: providers.FlowTypeRecovery,
			client: &inboundmodel.InboundClient{
				ID:                    appID,
				IsRecoveryFlowEnabled: true,
				RecoveryFlowID:        "recovery-graph-1",
			},
			expectedGraph: "recovery-graph-1",
		},
		{
			name:     "recovery flow disabled",
			flowType: providers.FlowTypeRecovery,
			client: &inboundmodel.InboundClient{
				ID:                    appID,
				IsRecoveryFlowEnabled: false,
			},
			expectedCode: ErrorRecoveryFlowDisabled.Code,
		},
		{
			name:     "signout flow configured",
			flowType: providers.FlowTypeSignOut,
			client: &inboundmodel.InboundClient{
				ID:            appID,
				SignOutFlowID: "signout-graph-1",
			},
			expectedGraph: "signout-graph-1",
		},
		{
			name:     "signout flow not configured",
			flowType: providers.FlowTypeSignOut,
			client: &inboundmodel.InboundClient{
				ID: appID,
			},
			expectedCode: tidcommon.InternalServerError.Code,
		},
		{
			name:     "device grant flow configured",
			flowType: providers.FlowTypeDeviceGrant,
			client: &inboundmodel.InboundClient{
				ID:                appID,
				DeviceGrantFlowID: "device-grant-graph-1",
			},
			expectedGraph: "device-grant-graph-1",
		},
		{
			name:         "device grant flow not configured",
			flowType:     providers.FlowTypeDeviceGrant,
			client:       &inboundmodel.InboundClient{ID: appID},
			expectedCode: ErrorDeviceGrantFlowDisabled.Code,
		},
		{
			name:         "empty app id",
			flowType:     providers.FlowTypeAuthentication,
			client:       nil,
			expectedCode: ErrorInvalidAppID.Code,
		},
		{
			name:         "device grant with empty app id",
			flowType:     providers.FlowTypeDeviceGrant,
			client:       nil,
			expectedCode: ErrorInvalidAppID.Code,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(s.T())
			mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(s.T())
			service := &flowExecService{
				actorProvider: actorprovider.Initialize(
					mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil, nil, nil),
				cfg: testFlowExecCfg,
			}

			lookupID := appID
			if tt.client == nil {
				lookupID = ""
			}

			if tt.client != nil {
				mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, lookupID).Return(tt.client, nil)
			}

			graphID, svcErr := service.getFlowGraph(context.Background(), lookupID, tt.flowType, log.GetLogger())

			if tt.expectedCode != "" {
				s.NotNil(svcErr)
				s.Equal(tt.expectedCode, svcErr.Code)
				s.Empty(graphID)
				return
			}

			s.Nil(svcErr)
			s.Equal(tt.expectedGraph, graphID)
		})
	}
}

func (s *ServiceTestSuite) TestGetFlowGraph_MissingConfiguredFlowID() {
	appID := "test-app-missing-flow"
	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(s.T())
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(s.T())
	service := &flowExecService{
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil, nil, nil),
		cfg:           testFlowExecCfg,
	}

	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, appID).Return(
		&inboundmodel.InboundClient{
			ID:                        appID,
			IsRegistrationFlowEnabled: true,
			RegistrationFlowID:        "",
		}, nil)

	graphID, svcErr := service.getFlowGraph(context.Background(), appID,
		providers.FlowTypeRegistration, log.GetLogger())

	s.Empty(graphID)
	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestGetFlowGraph_NilClient() {
	appID := "test-app-nil"
	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(s.T())
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(s.T())
	service := &flowExecService{
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil, nil, nil),
		cfg:           testFlowExecCfg,
	}

	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, appID).
		Return((*inboundmodel.InboundClient)(nil), nil)

	graphID, svcErr := service.getFlowGraph(context.Background(), appID,
		providers.FlowTypeAuthentication, log.GetLogger())

	s.Empty(graphID)
	s.NotNil(svcErr)
	s.Equal(ErrorInvalidAppID.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestExecute_NewFlow_IncompleteStoresContext() {
	appID := "test-app-new-flow"

	testConfig := &config.Config{}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test-new-flow", testConfig)

	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	testGraph := flowFactory.CreateGraph("auth-graph-new", providers.FlowTypeAuthentication, 1)

	mockStore := newFlowStoreInterfaceMock(s.T())
	mockFlowProvider := NewFlowProviderMock(s.T())
	mockGraphBuilder := NewGraphBuilderInterfaceMock(s.T())
	mockEngine := newFlowEngineInterfaceMock(s.T())
	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(s.T())
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(s.T())
	mockCrypto := cryptomock.NewRuntimeCryptoProviderMock(s.T())

	mockAuthn := managermock.NewAuthnProviderManagerMock(s.T())
	mockInboundClient.EXPECT().GetOAuthProfileByEntityID(mock.Anything, appID).Return(nil, nil)
	mockAuthn.EXPECT().AuthenticateUser(mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, nil, nil)
	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, appID).Return(
		&inboundmodel.InboundClient{ID: appID, AuthFlowID: "auth-graph-new"}, nil)
	mockEntityProvider.EXPECT().GetEntityProfile(mock.Anything, appID).Return(
		&providers.Entity{ID: appID, Category: providers.EntityCategoryApp}, nil)
	mockFlowProvider.EXPECT().
		GetFlow(mock.Anything, "auth-graph-new").
		Return(&providers.CompleteFlowDefinition{ID: "auth-graph-new"}, nil)
	mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(testGraph, nil)
	mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]byte("encrypted-ctx"), nil, nil)
	mockEngine.EXPECT().Execute(mock.Anything).
		Return(FlowStep{Status: providers.FlowStatusIncomplete}, nil)
	mockStore.EXPECT().StoreFlowContext(
		mock.MatchedBy(func(ctx context.Context) bool { return ctx.Value(txMarkerKey{}) == "tx" }),
		mock.AnythingOfType("FlowContextDB"), mock.Anything).Return(nil)

	service := &flowExecService{
		flowStore:     mockStore,
		graphBuilder:  mockGraphBuilder,
		flowProvider:  mockFlowProvider,
		flowEngine:    mockEngine,
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, mockAuthn, nil, nil, nil),
		transactioner: &stubTransactioner{},
		cryptoSvc:     mockCrypto,
		cfg:           testFlowExecCfg,
	}

	flowStep, svcErr := service.Execute(context.Background(), appID, "",
		string(providers.FlowTypeAuthentication), "", false, "submit", map[string]string{}, "", "valid-secret", "", nil)

	s.Nil(svcErr)
	s.NotNil(flowStep)
	s.Equal(providers.FlowStatusIncomplete, flowStep.Status)
}

func (s *ServiceTestSuite) TestExecute_ExistingFlow_CompleteRemovesContext() {
	testConfig := &config.Config{}
	_ = config.InitializeServerRuntime("/tmp/test-existing-flow-complete", testConfig)

	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	testGraph := flowFactory.CreateGraph("test-graph-id", providers.FlowTypeAuthentication, 1)

	engineCtx := EngineContext{
		ExecutionID:       "existing-execution-id",
		AppID:             "test-app-id",
		FlowType:          providers.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{Attributes: map[string]interface{}{}},
		UserInputs:        map[string]string{},
		RuntimeData:       map[string]string{},
		ExecutionHistory:  map[string]*providers.NodeExecutionRecord{},
		Graph:             testGraph,
	}
	storedCtx := &FlowContextDB{}
	err := storedCtx.FromEngineContext(engineCtx)
	s.NoError(err)

	mockStore := newFlowStoreInterfaceMock(s.T())
	mockFlowProvider := NewFlowProviderMock(s.T())
	mockGraphBuilder := NewGraphBuilderInterfaceMock(s.T())
	mockEngine := newFlowEngineInterfaceMock(s.T())
	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(s.T())
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(s.T())

	mockStore.EXPECT().GetFlowContext(mock.Anything, "existing-execution-id").Return(storedCtx, nil)
	mockFlowProvider.EXPECT().
		GetFlow(mock.Anything, "test-graph-id").
		Return(&providers.CompleteFlowDefinition{ID: "test-graph-id"}, nil)
	mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(testGraph, nil)
	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, "test-app-id").Return(
		&inboundmodel.InboundClient{ID: "test-app-id", AuthFlowID: "test-graph-id"}, nil)
	mockEntityProvider.EXPECT().GetEntityProfile(mock.Anything, "test-app-id").Return(
		&providers.Entity{ID: "test-app-id", Category: providers.EntityCategoryApp}, nil)
	mockEngine.EXPECT().Execute(mock.Anything).
		Return(FlowStep{Status: providers.FlowStatusComplete}, nil)
	mockStore.EXPECT().DeleteFlowContext(
		mock.MatchedBy(func(ctx context.Context) bool { return ctx.Value(txMarkerKey{}) == "tx" }),
		"existing-execution-id").Return(nil)

	service := &flowExecService{
		flowStore:     mockStore,
		graphBuilder:  mockGraphBuilder,
		flowProvider:  mockFlowProvider,
		flowEngine:    mockEngine,
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil, nil, nil),
		transactioner: &stubTransactioner{},
		cfg:           testFlowExecCfg,
	}

	flowStep, svcErr := service.Execute(context.Background(), "test-app", "existing-execution-id",
		string(providers.FlowTypeAuthentication), "", false, "submit", map[string]string{}, "", "", "", nil)

	s.Nil(svcErr)
	s.NotNil(flowStep)
	s.Equal(providers.FlowStatusComplete, flowStep.Status)
}

func (s *ServiceTestSuite) TestLoadNewContext_InvalidFlowType() {
	service := &flowExecService{cfg: testFlowExecCfg}

	engineCtx, svcErr := service.loadNewContext(context.Background(), "", "test-app", "INVALID_TYPE",
		"", false, "submit", map[string]string{}, "", "", log.GetLogger())

	s.Nil(engineCtx)
	s.NotNil(svcErr)
	s.Equal(ErrorInvalidFlowType.Code, svcErr.Code)
}

// The caller here is a legitimate administrator, so the rejection is about the target flow's type
// rather than the caller's identity.
func (s *ServiceTestSuite) TestLoadNewContext_ByIDRejectsInteractiveFlow() {
	security.InitControlPlanePermissions("")
	mockFlowProvider := NewFlowProviderMock(s.T())
	mockFlowProvider.EXPECT().GetFlow(mock.Anything, "authentication-flow").Return(
		&providers.CompleteFlowDefinition{
			ID:       "authentication-flow",
			FlowType: providers.FlowTypeAuthentication,
		}, nil)
	service := &flowExecService{flowProvider: mockFlowProvider, cfg: testFlowExecCfg}

	engineCtx, svcErr := service.loadNewContext(authenticatedAdminContext([]string{"system"}),
		"authentication-flow", "", "", "", false, "", map[string]string{}, "", "", log.GetLogger())

	s.Nil(engineCtx)
	s.NotNil(svcErr)
	s.Equal(ErrorFlowIDExecutionNotPermitted.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestLoadNewContext_ByIDLoadsAdministrationFlow() {
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), &config.Config{}))
	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	graph := flowFactory.CreateGraph("administration-1", providers.FlowTypeAdministration, 1)
	flow := &providers.CompleteFlowDefinition{
		ID: "administration-1", FlowType: providers.FlowTypeAdministration, ActiveVersion: 1,
	}
	mockFlowProvider := NewFlowProviderMock(s.T())
	mockFlowProvider.EXPECT().GetFlow(mock.Anything, "administration-1").Return(flow, nil).Once()
	mockGraphBuilder := NewGraphBuilderInterfaceMock(s.T())
	mockGraphBuilder.EXPECT().GetGraph(mock.Anything, flow).Return(graph, nil)
	service := &flowExecService{
		flowProvider: mockFlowProvider, graphBuilder: mockGraphBuilder, cfg: testFlowExecCfg,
		flowVisibility: s.administrationFlowVisible(),
	}

	engineCtx, svcErr := service.loadNewContext(authenticatedAdminContext([]string{"system:user"}),
		"administration-1", "", "", "", true, "", map[string]string{}, "", "", log.GetLogger())

	s.Nil(svcErr)
	s.Require().NotNil(engineCtx)
	s.Equal(providers.FlowTypeAdministration, engineCtx.FlowType)
	s.True(engineCtx.Verbose)
}

// A caller that already knows the flow's inputs can supply them on the initiating request and get
// the finished result back, instead of being told which inputs are required and having to send a
// second call. The inputs must therefore be on the context the very first time the engine runs.
func (s *ServiceTestSuite) TestLoadNewContext_ByIDCarriesInitiatingInputs() {
	security.InitControlPlanePermissions("")
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), &config.Config{}))
	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	graph := flowFactory.CreateGraph("administration-1", providers.FlowTypeAdministration, 1)
	flow := &providers.CompleteFlowDefinition{
		ID: "administration-1", FlowType: providers.FlowTypeAdministration, ActiveVersion: 1,
	}
	mockFlowProvider := NewFlowProviderMock(s.T())
	mockFlowProvider.EXPECT().GetFlow(mock.Anything, "administration-1").Return(flow, nil).Once()
	mockGraphBuilder := NewGraphBuilderInterfaceMock(s.T())
	mockGraphBuilder.EXPECT().GetGraph(mock.Anything, flow).Return(graph, nil)
	service := &flowExecService{
		flowProvider: mockFlowProvider, graphBuilder: mockGraphBuilder, cfg: testFlowExecCfg,
		flowVisibility: s.administrationFlowVisible(),
	}

	engineCtx, svcErr := service.loadNewContext(authenticatedAdminContext([]string{"system:user"}),
		"administration-1", "", "", "", true, "",
		map[string]string{"subject": "019fd0e4-de6c-7ea5-9541-7982f40beeb9"}, "", "", log.GetLogger())

	s.Nil(svcErr)
	s.Require().NotNil(engineCtx)
	s.Equal("019fd0e4-de6c-7ea5-9541-7982f40beeb9", engineCtx.UserInputs["subject"])
}

// Omitting the inputs must stay valid: that is the first leg of the two-request exchange, where the
// response tells the caller which inputs to send back with the execution ID.
func (s *ServiceTestSuite) TestLoadNewContext_ByIDWithoutInputsStartsEmpty() {
	security.InitControlPlanePermissions("")
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), &config.Config{}))
	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	graph := flowFactory.CreateGraph("administration-1", providers.FlowTypeAdministration, 1)
	flow := &providers.CompleteFlowDefinition{
		ID: "administration-1", FlowType: providers.FlowTypeAdministration, ActiveVersion: 1,
	}
	mockFlowProvider := NewFlowProviderMock(s.T())
	mockFlowProvider.EXPECT().GetFlow(mock.Anything, "administration-1").Return(flow, nil).Once()
	mockGraphBuilder := NewGraphBuilderInterfaceMock(s.T())
	mockGraphBuilder.EXPECT().GetGraph(mock.Anything, flow).Return(graph, nil)
	service := &flowExecService{
		flowProvider: mockFlowProvider, graphBuilder: mockGraphBuilder, cfg: testFlowExecCfg,
		flowVisibility: s.administrationFlowVisible(),
	}

	engineCtx, svcErr := service.loadNewContext(authenticatedAdminContext([]string{"system:user"}),
		"administration-1", "", "", "", true, "", nil, "", "", log.GetLogger())

	s.Nil(svcErr)
	s.Require().NotNil(engineCtx)
	s.NotNil(engineCtx.UserInputs, "an empty input map must still be initialized for the engine")
	s.Empty(engineCtx.UserInputs)
}

// An invitation flow acts for the caller's own unit: an ouId sent as an input must not move it.
func (s *ServiceTestSuite) TestLoadNewContext_ByIDLoadsInvitationFlowInTheTokensOU() {
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), &config.Config{}))
	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	graph := flowFactory.CreateGraph("invitation-1", providers.FlowTypeInvitation, 1)
	flow := &providers.CompleteFlowDefinition{
		ID: "invitation-1", FlowType: providers.FlowTypeInvitation, ActiveVersion: 1,
	}
	mockFlowProvider := NewFlowProviderMock(s.T())
	mockFlowProvider.EXPECT().GetFlow(mock.Anything, "invitation-1").Return(flow, nil).Once()
	mockGraphBuilder := NewGraphBuilderInterfaceMock(s.T())
	mockGraphBuilder.EXPECT().GetGraph(mock.Anything, flow).Return(graph, nil)
	mockVisibility := newFlowVisibilityProviderMock(s.T())
	mockVisibility.EXPECT().IsFlowVisible(mock.Anything, "invitation-1", "ou-1").Return(true, nil).Once()
	service := &flowExecService{
		flowProvider: mockFlowProvider, graphBuilder: mockGraphBuilder, flowVisibility: mockVisibility,
		cfg: testFlowExecCfg,
		actorProvider: actorprovider.Initialize(inboundclientmock.NewInboundClientServiceInterfaceMock(s.T()),
			entityprovidermock.NewEntityProviderInterfaceMock(s.T()), noopAuthnMgr(), nil, nil, nil),
	}

	engineCtx, svcErr := service.loadNewContext(authenticatedAdminContext([]string{"system:user"}),
		"invitation-1", "", "", "", false, "", map[string]string{"ouId": "ou-x"}, "", "", log.GetLogger())

	s.Nil(svcErr)
	s.Require().NotNil(engineCtx)
	s.Equal(providers.FlowTypeInvitation, engineCtx.FlowType)
	s.Equal("ou-1", engineCtx.RuntimeData[common.RuntimeKeyOUID])
	s.Equal("admin-1", engineCtx.RuntimeData[common.RuntimeKeyInviter])
	s.Nil(engineCtx.authority, "an invitation flow records no authority; its role step records the admin")
	s.Empty(engineCtx.Application.ID)
}

// An organization unit runs an invitation flow only when it owns the flow or was shared it.
func (s *ServiceTestSuite) TestLoadNewContext_ByIDRefusesInvitationFlowNotSharedWithTheTokensOU() {
	security.InitControlPlanePermissions("")
	mockFlowProvider := NewFlowProviderMock(s.T())
	mockFlowProvider.EXPECT().GetFlow(mock.Anything, "invitation-1").Return(
		&providers.CompleteFlowDefinition{ID: "invitation-1", FlowType: providers.FlowTypeInvitation, OUID: "ou-root"},
		nil)
	mockVisibility := newFlowVisibilityProviderMock(s.T())
	mockVisibility.EXPECT().IsFlowVisible(mock.Anything, "invitation-1", "ou-1").Return(false, nil).Once()
	service := &flowExecService{flowProvider: mockFlowProvider, flowVisibility: mockVisibility, cfg: testFlowExecCfg}

	engineCtx, svcErr := service.loadNewContext(authenticatedAdminContext([]string{"system:user"}),
		"invitation-1", "", "", "", false, "", nil, "", "", log.GetLogger())

	s.Nil(engineCtx)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorFlowNotSharedWithOU.Code, svcErr.Code)
}

// An invitation flow is resolved as the server: the unit the caller's token names must own the flow or have
// been shared it, which an organization administrator below the owning unit could not show by reading the flow
// itself.
func (s *ServiceTestSuite) TestLoadNewContext_ByIDResolvesInvitationFlowAsTheServer() {
	security.InitDataPlanePermissions("")
	defer security.InitControlPlanePermissions("")
	mockFlowProvider := NewFlowProviderMock(s.T())
	mockFlowProvider.EXPECT().GetFlow(mock.MatchedBy(security.IsRuntimeContext), "invitation-1").Return(
		&providers.CompleteFlowDefinition{ID: "invitation-1", FlowType: providers.FlowTypeInvitation, OUID: "ou-root"},
		nil).Once()
	mockVisibility := newFlowVisibilityProviderMock(s.T())
	mockVisibility.EXPECT().IsFlowVisible(mock.Anything, "invitation-1", "ou-1").Return(false, nil).Once()
	service := &flowExecService{flowProvider: mockFlowProvider, flowVisibility: mockVisibility, cfg: testFlowExecCfg}

	engineCtx, svcErr := service.loadNewContext(authenticatedAdminContext([]string{"org"}),
		"invitation-1", "", "", "", false, "", nil, "", "", log.GetLogger())

	s.Nil(engineCtx)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorFlowNotSharedWithOU.Code, svcErr.Code)
}

// For a caller without the root permission, an administration flow starts only in a unit that owns it or was
// shared it.
func (s *ServiceTestSuite) TestLoadNewContext_ByIDRefusesAdministrationFlowNotSharedWithTheCaller() {
	security.InitControlPlanePermissions("")
	mockFlowProvider := NewFlowProviderMock(s.T())
	mockFlowProvider.EXPECT().GetFlow(mock.MatchedBy(security.IsRuntimeContext), "administration-1").Return(
		&providers.CompleteFlowDefinition{ID: "administration-1", FlowType: providers.FlowTypeAdministration},
		nil).Once()
	mockVisibility := newFlowVisibilityProviderMock(s.T())
	mockVisibility.EXPECT().IsFlowVisible(mock.Anything, "administration-1", "ou-1").Return(false, nil).Once()
	service := &flowExecService{flowProvider: mockFlowProvider, flowVisibility: mockVisibility, cfg: testFlowExecCfg}

	engineCtx, svcErr := service.loadNewContext(authenticatedAdminContext([]string{"system:user"}),
		"administration-1", "", "", "", false, "", nil, "", "", log.GetLogger())

	s.Nil(engineCtx)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorFlowNotSharedWithOU.Code, svcErr.Code)
}

// A caller holding the root permission reaches every organization unit, so an administration flow starts
// without asking whether it is shared with the token's unit.
func (s *ServiceTestSuite) TestValidateFlowSharedWithCaller_RootSkipsTheShareCheck() {
	security.InitControlPlanePermissions("")
	mockVisibility := newFlowVisibilityProviderMock(s.T())
	service := &flowExecService{flowVisibility: mockVisibility, cfg: testFlowExecCfg}

	s.Nil(service.validateFlowSharedWithCaller(authenticatedAdminContext([]string{"system"}), "administration-1"))
	mockVisibility.AssertNotCalled(s.T(), "IsFlowVisible", mock.Anything, mock.Anything, mock.Anything)
}

// administrationFlowVisible returns a visibility provider that reports administration-1 as shared with ou-1.
func (s *ServiceTestSuite) administrationFlowVisible() *flowVisibilityProviderMock {
	visibility := newFlowVisibilityProviderMock(s.T())
	visibility.EXPECT().IsFlowVisible(mock.Anything, "administration-1", "ou-1").Return(true, nil).Once()
	return visibility
}

// Without flow management to say whether the flow is shared, an invitation flow is refused.
func (s *ServiceTestSuite) TestLoadNewContext_ByIDRefusesInvitationWithoutAVisibilityProvider() {
	security.InitControlPlanePermissions("")
	mockFlowProvider := NewFlowProviderMock(s.T())
	mockFlowProvider.EXPECT().GetFlow(mock.Anything, "invitation-1").Return(
		&providers.CompleteFlowDefinition{ID: "invitation-1", FlowType: providers.FlowTypeInvitation}, nil)
	service := &flowExecService{flowProvider: mockFlowProvider, cfg: testFlowExecCfg}

	engineCtx, svcErr := service.loadNewContext(authenticatedAdminContext([]string{"system:user"}),
		"invitation-1", "", "", "", false, "", nil, "", "", log.GetLogger())

	s.Nil(engineCtx)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorFlowNotSharedWithOU.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestLoadNewContext_ByIDReturnsTheErrorOfTheShareLookup() {
	security.InitControlPlanePermissions("")
	mockFlowProvider := NewFlowProviderMock(s.T())
	mockFlowProvider.EXPECT().GetFlow(mock.Anything, "invitation-1").Return(
		&providers.CompleteFlowDefinition{ID: "invitation-1", FlowType: providers.FlowTypeInvitation}, nil)
	mockVisibility := newFlowVisibilityProviderMock(s.T())
	mockVisibility.EXPECT().IsFlowVisible(mock.Anything, "invitation-1", "ou-1").
		Return(false, &tidcommon.InternalServerError).Once()
	service := &flowExecService{flowProvider: mockFlowProvider, flowVisibility: mockVisibility, cfg: testFlowExecCfg}

	engineCtx, svcErr := service.loadNewContext(authenticatedAdminContext([]string{"system:user"}),
		"invitation-1", "", "", "", false, "", nil, "", "", log.GetLogger())

	s.Nil(engineCtx)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

// A client error from the share lookup, such as the token's OU no longer existing, is a refusal like an unshared
// flow, not a sharing policy error.
func (s *ServiceTestSuite) TestLoadNewContext_ByIDRefusesAShareLookupClientErrorAsNotShared() {
	security.InitControlPlanePermissions("")
	mockFlowProvider := NewFlowProviderMock(s.T())
	mockFlowProvider.EXPECT().GetFlow(mock.Anything, "invitation-1").Return(
		&providers.CompleteFlowDefinition{ID: "invitation-1", FlowType: providers.FlowTypeInvitation}, nil)
	mockVisibility := newFlowVisibilityProviderMock(s.T())
	mockVisibility.EXPECT().IsFlowVisible(mock.Anything, "invitation-1", "ou-1").
		Return(false, &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "FLM-1034"}).Once()
	service := &flowExecService{flowProvider: mockFlowProvider, flowVisibility: mockVisibility, cfg: testFlowExecCfg}

	engineCtx, svcErr := service.loadNewContext(authenticatedAdminContext([]string{"system:user"}),
		"invitation-1", "", "", "", false, "", nil, "", "", log.GetLogger())

	s.Nil(engineCtx)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorFlowNotSharedWithOU.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestLoadNewContext_ByIDRefusesInvitationWithoutTokenOU() {
	security.InitControlPlanePermissions("")
	mockFlowProvider := NewFlowProviderMock(s.T())
	mockFlowProvider.EXPECT().GetFlow(mock.Anything, "invitation-1").Return(
		&providers.CompleteFlowDefinition{ID: "invitation-1", FlowType: providers.FlowTypeInvitation}, nil)
	service := &flowExecService{flowProvider: mockFlowProvider, cfg: testFlowExecCfg}
	ctx := security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest("admin-1", "", "token", []string{"system"}, map[string]interface{}{}))

	engineCtx, svcErr := service.loadNewContext(ctx,
		"invitation-1", "", "", "", false, "", map[string]string{"ouId": "ou-x"}, "", "", log.GetLogger())

	s.Nil(engineCtx)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorInvitationOUMissing.Code, svcErr.Code)
}

// Every flow started by ID needs a system permission, checked before the flow is resolved.
func (s *ServiceTestSuite) TestLoadNewContext_ByIDRefusesCallerWithoutASystemPermission() {
	security.InitControlPlanePermissions("")
	mockFlowProvider := NewFlowProviderMock(s.T())
	service := &flowExecService{flowProvider: mockFlowProvider, cfg: testFlowExecCfg}

	for _, flowID := range []string{"administration-1", "invitation-1"} {
		engineCtx, svcErr := service.loadNewContext(authenticatedAdminContext([]string{"openid", "systemx"}),
			flowID, "", "", "", false, "", nil, "", "", log.GetLogger())

		s.Nil(engineCtx)
		s.Require().NotNil(svcErr)
		s.Equal(ErrorAdministrationPermissionRequired.Code, svcErr.Code)
	}
	mockFlowProvider.AssertNotCalled(s.T(), "GetFlow", mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestLoadNewContext_ByTypeRefusesInvitationFlow() {
	service := &flowExecService{cfg: testFlowExecCfg}

	engineCtx, svcErr := service.loadNewContext(authenticatedAdminContext(nil), "", "",
		string(providers.FlowTypeInvitation), "", false, "", nil, "", "", log.GetLogger())

	s.Nil(engineCtx)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorInvalidFlowType.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestValidateInvitationCaller() {
	s.Run("RejectsTokenWithoutOU", func() {
		ctx := security.WithSecurityContextTest(context.Background(),
			security.NewSecurityContextForTest("admin-1", "", "token", nil, map[string]interface{}{}))

		svcErr := validateInvitationCaller(ctx)

		s.Require().NotNil(svcErr)
		s.Equal(ErrorInvitationOUMissing.Code, svcErr.Code)
	})

	s.Run("AllowsCallerWithTokenOU", func() {
		s.Nil(validateInvitationCaller(authenticatedAdminContext([]string{"system"})))
	})
}

// authenticatedAdminContext returns a context carrying an authenticated caller with the permissions
// supplied, as the security middleware would produce for a request bearing a valid token.
func authenticatedAdminContext(permissions []string) context.Context {
	authCtx := security.NewSecurityContextForTest(
		"admin-1", "ou-1", "token", permissions, map[string]interface{}{})
	return security.WithSecurityContextTest(context.Background(), authCtx)
}

// Initiation entry points must apply the same gate as execution, so neither becomes a way around it.
func (s *ServiceTestSuite) TestInitiateFlow_RejectsUnauthenticatedAdministrationFlow() {
	security.InitControlPlanePermissions("")
	service := &flowExecService{cfg: testFlowExecCfg}

	executionID, svcErr := service.InitiateFlow(security.WithRuntimeContext(context.Background()),
		&FlowInitContext{FlowType: string(providers.FlowTypeAdministration), ApplicationID: "app-1"})

	s.Empty(executionID)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorFlowAuthenticationRequired.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestInitiateAndExecute_RejectsUnauthenticatedAdministrationFlow() {
	security.InitControlPlanePermissions("")
	service := &flowExecService{cfg: testFlowExecCfg}

	flowStep, svcErr := service.InitiateAndExecute(security.WithRuntimeContext(context.Background()),
		&FlowInitContext{FlowType: string(providers.FlowTypeAdministration), ApplicationID: "app-1"})

	s.Nil(flowStep)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorFlowAuthenticationRequired.Code, svcErr.Code)
}

// An unauthenticated caller is rejected before the flow is resolved, so the error cannot reveal
// whether the flow ID exists or what type it is, and no lookup work is done on its behalf.
func (s *ServiceTestSuite) TestExecuteByID_RejectsUnauthenticatedPublicRequest() {
	security.InitControlPlanePermissions("")
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), &config.Config{}))
	mockFlowProvider := NewFlowProviderMock(s.T())
	mockObservability := observabilitymock.NewObservabilityServiceInterfaceMock(s.T())
	mockObservability.EXPECT().IsEnabled().Return(false)
	service := &flowExecService{
		flowProvider: mockFlowProvider, observabilitySvc: mockObservability, cfg: testFlowExecCfg,
	}

	flowStep, svcErr := service.ExecuteByID(security.WithRuntimeContext(context.Background()),
		"does-not-matter", "", false, "", map[string]string{}, "", nil)

	s.Nil(flowStep)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorFlowAuthenticationRequired.Code, svcErr.Code)
	mockFlowProvider.AssertNotCalled(s.T(), "GetFlow", mock.Anything, mock.Anything)
}

// An authenticated administrator still cannot use the flow-ID entry point to start a flow of any
// other type, which would bypass that type's application binding and initiation guards.
func (s *ServiceTestSuite) TestExecuteByID_RejectsNonAdministrationFlowType() {
	security.InitControlPlanePermissions("")
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), &config.Config{}))

	for _, flowType := range []providers.FlowType{
		providers.FlowTypeAuthentication,
		providers.FlowTypeRegistration,
		providers.FlowTypeRecovery,
		providers.FlowTypeSignOut,
		providers.FlowTypeUserOnboarding,
		providers.FlowTypeSelf,
	} {
		s.Run(string(flowType), func() {
			flow := &providers.CompleteFlowDefinition{ID: "flow-1", FlowType: flowType, ActiveVersion: 1}
			mockFlowProvider := NewFlowProviderMock(s.T())
			mockFlowProvider.EXPECT().GetFlow(mock.Anything, "flow-1").Return(flow, nil).Once()
			mockObservability := observabilitymock.NewObservabilityServiceInterfaceMock(s.T())
			mockObservability.EXPECT().IsEnabled().Return(false)
			service := &flowExecService{
				flowProvider: mockFlowProvider, observabilitySvc: mockObservability, cfg: testFlowExecCfg,
			}

			flowStep, svcErr := service.ExecuteByID(authenticatedAdminContext([]string{"system"}),
				"flow-1", "", true, "", map[string]string{}, "", nil)

			s.Nil(flowStep)
			s.Require().NotNil(svcErr)
			s.Equal(ErrorFlowIDExecutionNotPermitted.Code, svcErr.Code)
		})
	}
}

func (s *ServiceTestSuite) TestSetApplicationToContext_ActorNotFound() {
	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(s.T())
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(s.T())
	service := &flowExecService{
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil, nil, nil),
		cfg:           testFlowExecCfg,
	}

	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, "missing-app").
		Return(nil, inboundclient.ErrInboundClientNotFound)

	engineCtx := &EngineContext{
		Context:  context.Background(),
		AppID:    "missing-app",
		FlowType: providers.FlowTypeAuthentication,
	}

	svcErr := service.setApplicationToContext(engineCtx, log.GetLogger())

	s.NotNil(svcErr)
	s.Equal(ErrorInvalidAppID.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestSetApplicationToContext_UserOnboardingSkipped() {
	service := &flowExecService{cfg: testFlowExecCfg}
	engineCtx := &EngineContext{
		Context:  context.Background(),
		FlowType: providers.FlowTypeUserOnboarding,
	}

	svcErr := service.setApplicationToContext(engineCtx, log.GetLogger())

	s.Nil(svcErr)
}

func (s *ServiceTestSuite) TestGetFlowExpirySeconds_RecoveryFlow() {
	service := &flowExecService{cfg: testFlowExecCfg}
	s.Equal(int64(1800), service.getFlowExpirySeconds(context.Background(), providers.FlowTypeRecovery))
}

func (s *ServiceTestSuite) TestLoadContextFromStore_EmptyExecutionID() {
	service := &flowExecService{cfg: testFlowExecCfg}

	engineCtx, svcErr := service.loadContextFromStore(context.Background(), "", log.GetLogger())

	s.Nil(engineCtx)
	s.NotNil(svcErr)
	s.Equal(ErrorInvalidExecutionID.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestRemoveContext_EmptyExecutionID() {
	service := &flowExecService{cfg: testFlowExecCfg}

	err := service.removeContext(context.Background(), "", log.GetLogger())

	s.Error(err)
}

func (s *ServiceTestSuite) TestUpdateContext_CompleteStatusRemovesContext() {
	mockStore := newFlowStoreInterfaceMock(s.T())
	service := &flowExecService{
		flowStore:     mockStore,
		transactioner: &stubTransactioner{},
		cfg:           testFlowExecCfg,
	}

	mockStore.EXPECT().DeleteFlowContext(
		mock.MatchedBy(func(ctx context.Context) bool { return ctx.Value(txMarkerKey{}) == "tx" }),
		"exec-1").Return(nil)

	engineCtx := &EngineContext{ExecutionID: "exec-1"}
	flowStep := &FlowStep{Status: providers.FlowStatusComplete}

	err := service.updateContext(context.Background(), engineCtx, flowStep, log.GetLogger())

	s.NoError(err)
}

func (s *ServiceTestSuite) TestGetSystemFlowGraph_GetFlowByHandleError() {
	mockFlowProvider := NewFlowProviderMock(s.T())
	mockGraphBuilder := NewGraphBuilderInterfaceMock(s.T())
	service := &flowExecService{
		graphBuilder: mockGraphBuilder,
		flowProvider: mockFlowProvider,
		cfg:          testFlowExecCfg,
		serverConfigSvc: stubServerConfig{cfg: flowconfig.FlowSectionConfig{
			UserOnboardingFlow: flowconfig.FlowTypeConfig{DefaultHandle: testUserOnboardingFlowHandle},
		}},
	}

	mockFlowProvider.EXPECT().GetFlowByHandle(mock.Anything, testUserOnboardingFlowHandle,
		providers.FlowTypeUserOnboarding).Return(nil, &tidcommon.InternalServerError)

	graphID, svcErr := service.getSystemFlowGraph(context.Background(),
		providers.FlowTypeUserOnboarding, log.GetLogger())

	s.Empty(graphID)
	s.NotNil(svcErr)
}

func (s *ServiceTestSuite) TestSetApplicationToContext_BuildApplicationError() {
	mockProvider := actorprovidermock.NewActorProviderMock(s.T())
	mockProvider.EXPECT().GetInboundClientByID(mock.Anything, "app-1").
		Return((*inboundmodel.InboundClient)(nil), &tidcommon.InternalServerError)

	service := &flowExecService{
		actorProvider: mockProvider,
		cfg:           testFlowExecCfg,
	}
	engineCtx := &EngineContext{
		Context:  context.Background(),
		AppID:    "app-1",
		FlowType: providers.FlowTypeAuthentication,
	}

	svcErr := service.setApplicationToContext(engineCtx, log.GetLogger())

	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

// newAgentApplicationService returns a service whose actor provider resolves app-1 as an entity of the
// given category, and the actor provider mock for further expectations.
func (s *ServiceTestSuite) newAgentApplicationService(
	category providers.EntityCategory) (*flowExecService, *actorprovidermock.ActorProviderMock) {
	mockProvider := actorprovidermock.NewActorProviderMock(s.T())
	mockProvider.EXPECT().GetInboundClientByID(mock.Anything, "app-1").
		Return(&providers.InboundClient{ID: "app-1"}, nil)
	mockProvider.EXPECT().GetActor(mock.Anything, "app-1").
		Return(&providers.Entity{ID: "app-1", Category: category}, nil)
	return &flowExecService{actorProvider: mockProvider, cfg: testFlowExecCfg}, mockProvider
}

// An agent's required subject comes from its OAuth client's rules, for any flow type, so a CALL into an
// authentication flow is covered.
func (s *ServiceTestSuite) TestSetApplicationToContext_AgentClientRules() {
	for name, tc := range map[string]struct {
		client   *providers.OAuthClient
		expected string
	}{
		"RequiredSubject": {client: &providers.OAuthClient{ID: "app-1",
			Rules: providers.ClientRules{RequiredSubjectID: "owner-1"}}, expected: "owner-1"},
		"NoRules":       {client: &providers.OAuthClient{ID: "app-1"}, expected: ""},
		"NoOAuthClient": {client: nil, expected: ""},
	} {
		s.Run(name, func() {
			service, mockProvider := s.newAgentApplicationService(providers.EntityCategoryAgent)
			mockProvider.EXPECT().GetOAuthClientByID(mock.Anything, "app-1").Return(tc.client, nil)
			engineCtx := &EngineContext{
				Context: context.Background(), AppID: "app-1", FlowType: providers.FlowTypeRegistration,
			}

			s.Nil(service.setApplicationToContext(engineCtx, log.GetLogger()))
			s.Equal(tc.expected, engineCtx.RequiredSubjectID)
		})
	}
}

// An application's OAuth client is not read for rules.
func (s *ServiceTestSuite) TestSetApplicationToContext_ApplicationSkipsClientRules() {
	service, mockProvider := s.newAgentApplicationService(providers.EntityCategoryApp)
	engineCtx := &EngineContext{
		Context: context.Background(), AppID: "app-1", FlowType: providers.FlowTypeAuthentication,
	}

	s.Nil(service.setApplicationToContext(engineCtx, log.GetLogger()))
	s.Empty(engineCtx.RequiredSubjectID)
	mockProvider.AssertNotCalled(s.T(), "GetOAuthClientByID", mock.Anything, mock.Anything)
}

// A failed lookup fails the flow rather than running it without the owner constraint.
func (s *ServiceTestSuite) TestSetApplicationToContext_AgentClientRulesLookupFails() {
	service, mockProvider := s.newAgentApplicationService(providers.EntityCategoryAgent)
	mockProvider.EXPECT().GetOAuthClientByID(mock.Anything, "app-1").Return(nil, &tidcommon.InternalServerError)
	engineCtx := &EngineContext{
		Context: context.Background(), AppID: "app-1", FlowType: providers.FlowTypeAuthentication,
	}

	svcErr := service.setApplicationToContext(engineCtx, log.GetLogger())
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

// --- resolveFlowInitiationMode (type-driven) ---

// The flow-initiation mode is resolved from the application type. Machine-to-machine, browser, and
// mobile (without attestation) apps may not initiate a flow directly and never consult the OAuth
// profile. Full-stack, custom, and mcp apps derive the mode from their profile.
func (s *ServiceTestSuite) TestResolveFlowInitiationMode_ByType() {
	const appID = "test-app"
	tokenExchange := string(providers.GrantTypeTokenExchange)
	jwtBearer := string(providers.GrantTypeJWTBearer)

	cases := []struct {
		name          string
		appType       model.ApplicationType
		attestation   *providers.AttestationConfig
		profile       *providers.OAuthProfile
		profileErr    *tidcommon.ServiceError
		expectMode    flowInitiationMode
		expectErrCode string
	}{
		{name: "m2m not permitted", appType: model.ApplicationTypeM2M, expectMode: flowInitiationNotPermitted},
		{
			name: "browser not permitted", appType: model.ApplicationTypeBrowser,
			expectMode: flowInitiationNotPermitted,
		},
		{
			name: "mobile without attestation errors", appType: model.ApplicationTypeMobile,
			expectErrCode: ErrorAttestationNotConfigured.Code,
		},
		{
			name: "mobile with attestation uses attestation", appType: model.ApplicationTypeMobile,
			attestation: &providers.AttestationConfig{Apple: &providers.AppleAttestationConfig{}},
			expectMode:  flowInitiationAttestation,
		},
		{
			name: "mobile with dev mode skips attestation", appType: model.ApplicationTypeMobile,
			attestation: &providers.AttestationConfig{DevMode: true},
			expectMode:  flowInitiationDevMode,
		},
		{
			name: "mcp embedded uses flow secret", appType: model.ApplicationTypeMCP,
			profile:    &providers.OAuthProfile{GrantTypes: []string{"client_credentials", tokenExchange}},
			expectMode: flowInitiationFlowSecret,
		},
		{
			name: "mcp redirect not permitted", appType: model.ApplicationTypeMCP,
			profile:    &providers.OAuthProfile{GrantTypes: []string{"authorization_code"}},
			expectMode: flowInitiationNotPermitted,
		},
		{
			name: "fullstack redirect not permitted", appType: model.ApplicationTypeFullStack,
			profile:    &providers.OAuthProfile{GrantTypes: []string{"authorization_code"}},
			expectMode: flowInitiationNotPermitted,
		},
		{
			name: "fullstack embedded uses flow secret", appType: model.ApplicationTypeFullStack,
			profile:    &providers.OAuthProfile{GrantTypes: []string{"client_credentials", tokenExchange}},
			expectMode: flowInitiationFlowSecret,
		},
		{
			name: "fullstack without profile uses flow secret", appType: model.ApplicationTypeFullStack,
			profileErr: &actorprovider.ErrorActorNotFound, expectMode: flowInitiationFlowSecret,
		},
		{
			name: "custom embedded uses flow secret", appType: model.ApplicationTypeCustom,
			profile:    &providers.OAuthProfile{GrantTypes: []string{"client_credentials", tokenExchange}},
			expectMode: flowInitiationFlowSecret,
		},
		{
			name: "custom redirect not permitted", appType: model.ApplicationTypeCustom,
			profile:    &providers.OAuthProfile{GrantTypes: []string{"authorization_code"}},
			expectMode: flowInitiationNotPermitted,
		},
		{
			name: "custom m2m-shaped not permitted", appType: model.ApplicationTypeCustom,
			profile:    &providers.OAuthProfile{GrantTypes: []string{"client_credentials"}},
			expectMode: flowInitiationNotPermitted,
		},
		{
			name: "mcp jwt-bearer only not permitted", appType: model.ApplicationTypeMCP,
			profile:    &providers.OAuthProfile{GrantTypes: []string{jwtBearer}},
			expectMode: flowInitiationNotPermitted,
		},
		{
			name: "mcp client_credentials and jwt-bearer not permitted", appType: model.ApplicationTypeMCP,
			profile:    &providers.OAuthProfile{GrantTypes: []string{"client_credentials", jwtBearer}},
			expectMode: flowInitiationNotPermitted,
		},
		{
			name: "mcp jwt-bearer with token exchange uses flow secret", appType: model.ApplicationTypeMCP,
			profile:    &providers.OAuthProfile{GrantTypes: []string{jwtBearer, tokenExchange}},
			expectMode: flowInitiationFlowSecret,
		},
		{
			name: "fullstack ciba only not permitted", appType: model.ApplicationTypeFullStack,
			profile:    &providers.OAuthProfile{GrantTypes: []string{string(providers.GrantTypeCIBA)}},
			expectMode: flowInitiationNotPermitted,
		},
		{
			name: "fullstack redirect with token exchange not permitted", appType: model.ApplicationTypeFullStack,
			profile:    &providers.OAuthProfile{GrantTypes: []string{"authorization_code", tokenExchange}},
			expectMode: flowInitiationNotPermitted,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			t := s.T()
			mockActorProvider := actorprovidermock.NewActorProviderMock(t)

			client := &providers.InboundClient{ID: appID, Attestation: tc.attestation}
			if tc.appType != "" {
				client.Properties = map[string]interface{}{
					applicationTypePropertyKey: string(tc.appType),
				}
			}
			mockActorProvider.EXPECT().GetInboundClientByID(mock.Anything, appID).Return(client, nil)

			// The OAuth profile is consulted for full-stack, custom, and mcp apps.
			if tc.appType == model.ApplicationTypeFullStack || tc.appType == model.ApplicationTypeCustom ||
				tc.appType == model.ApplicationTypeMCP {
				mockActorProvider.EXPECT().GetOAuthProfileByID(mock.Anything, appID).Return(tc.profile, tc.profileErr)
			}

			service := &flowExecService{actorProvider: mockActorProvider}

			mode, attestation, svcErr := service.resolveFlowInitiationMode(context.Background(), appID)

			if tc.expectErrCode != "" {
				s.NotNil(svcErr)
				s.Equal(tc.expectErrCode, svcErr.Code)
				return
			}
			s.Nil(svcErr)
			s.Equal(tc.expectMode, mode)
			if tc.expectMode == flowInitiationAttestation {
				s.NotNil(attestation)
			}
		})
	}
}

// --- checkDirectFlowInitiationAllowed ---

// A new flow is rejected at initiation when the app is classified as RedirectOnly (an
// authorization_code app that must use the OAuth component) or as a backend app without a secret
// being presented.
func (s *ServiceTestSuite) TestExecute_NewFlow_GuardRejections() {
	cases := []struct {
		name       string
		flowType   providers.FlowType
		grantTypes []string
		flowSecret string
		wantCode   string
	}{
		{"redirect-based app blocked", providers.FlowTypeAuthentication, []string{"authorization_code"}, "",
			ErrorDirectFlowInitiationNotPermitted.Code},
		{"m2m app blocked", providers.FlowTypeAuthentication, []string{"client_credentials"}, "",
			ErrorDirectFlowInitiationNotPermitted.Code},
		{"jwt-bearer only app blocked", providers.FlowTypeAuthentication,
			[]string{"urn:ietf:params:oauth:grant-type:jwt-bearer"}, "",
			ErrorDirectFlowInitiationNotPermitted.Code},
		{"flow-native app without secret", providers.FlowTypeAuthentication,
			[]string{"client_credentials", "urn:ietf:params:oauth:grant-type:token-exchange"}, "",
			ErrorFlowSecretRequired.Code},
		{"sign-out redirect-based app blocked", providers.FlowTypeSignOut, []string{"authorization_code"}, "",
			ErrorDirectFlowInitiationNotPermitted.Code},
		{"sign-out flow-native app without secret", providers.FlowTypeSignOut,
			[]string{"client_credentials", "urn:ietf:params:oauth:grant-type:token-exchange"}, "",
			ErrorFlowSecretRequired.Code},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			t := s.T()
			testConfig := &config.Config{}
			config.ResetServerRuntime()
			_ = config.InitializeServerRuntime("/tmp/test", testConfig)

			mockActorProvider := actorprovidermock.NewActorProviderMock(t)
			mockObservability := observabilitymock.NewObservabilityServiceInterfaceMock(t)
			mockActorProvider.EXPECT().GetInboundClientByID(mock.Anything, "test-app").Return(
				&providers.InboundClient{ID: "test-app"}, nil)
			mockActorProvider.EXPECT().GetOAuthProfileByID(mock.Anything, "test-app").Return(
				&providers.OAuthProfile{GrantTypes: tc.grantTypes}, nil)
			mockObservability.EXPECT().IsEnabled().Return(false)

			service := &flowExecService{
				actorProvider:    mockActorProvider,
				observabilitySvc: mockObservability,
				transactioner:    &stubTransactioner{},
			}

			flowStep, svcErr := service.Execute(context.Background(), "test-app", "",
				string(tc.flowType), "", false, "submit", map[string]string{}, "", tc.flowSecret, "", nil)

			s.Nil(flowStep)
			s.NotNil(svcErr)
			s.Equal(tc.wantCode, svcErr.Code)
			s.Equal(tidcommon.ClientErrorType, svcErr.Type)
		})
	}
}

func (s *ServiceTestSuite) TestExecute_NewFlow_BackendApp_ValidSecret_Allowed() {
	t := s.T()
	testConfig := &config.Config{}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)

	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	testGraph := flowFactory.CreateGraph("auth-graph-1", providers.FlowTypeAuthentication, 1)

	mockStore := newFlowStoreInterfaceMock(t)
	mockFlowProvider := NewFlowProviderMock(t)
	mockGraphBuilder := NewGraphBuilderInterfaceMock(t)
	mockEngine := newFlowEngineInterfaceMock(t)
	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(t)
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)
	mockAuthn := managermock.NewAuthnProviderManagerMock(t)

	mockInboundClient.EXPECT().GetOAuthProfileByEntityID(mock.Anything, "test-app").Return(
		&providers.OAuthProfile{
			GrantTypes:   []string{"client_credentials", "urn:ietf:params:oauth:grant-type:token-exchange"},
			PublicClient: false,
		}, nil)
	mockAuthn.EXPECT().AuthenticateUser(mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, nil, nil)
	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, "test-app").Return(
		&inboundmodel.InboundClient{ID: "test-app", AuthFlowID: "auth-graph-1"}, nil).Times(3)
	mockEntityProvider.EXPECT().GetEntityProfile(mock.Anything, "test-app").Return(
		&providers.Entity{ID: "test-app", Category: providers.EntityCategoryApp},
		(*entityprovider.EntityProviderError)(nil))
	mockFlowProvider.EXPECT().
		GetFlow(mock.Anything, "auth-graph-1").
		Return(&providers.CompleteFlowDefinition{ID: "auth-graph-1"}, nil)
	mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(testGraph, nil)

	completedStep := FlowStep{Status: providers.FlowStatusComplete}
	mockEngine.EXPECT().Execute(mock.Anything).Return(completedStep, (*tidcommon.ServiceError)(nil))

	service := &flowExecService{
		flowStore:     mockStore,
		graphBuilder:  mockGraphBuilder,
		flowProvider:  mockFlowProvider,
		flowEngine:    mockEngine,
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, mockAuthn, nil, nil, nil),
		transactioner: &stubTransactioner{},
	}

	flowStep, svcErr := service.Execute(context.Background(), "test-app", "",
		string(providers.FlowTypeAuthentication), "", false, "submit", map[string]string{}, "", "valid-secret", "", nil)

	s.Nil(svcErr)
	s.NotNil(flowStep)
}

func (s *ServiceTestSuite) TestExecute_NewFlow_BackendApp_InvalidSecret_Rejected() {
	t := s.T()
	testConfig := &config.Config{}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)

	mockActorProvider := actorprovidermock.NewActorProviderMock(t)
	mockObservability := observabilitymock.NewObservabilityServiceInterfaceMock(t)
	mockActorProvider.EXPECT().GetInboundClientByID(mock.Anything, "test-app").Return(
		&providers.InboundClient{ID: "test-app"}, nil)
	mockActorProvider.EXPECT().GetOAuthProfileByID(mock.Anything, "test-app").Return(
		&providers.OAuthProfile{
			GrantTypes: []string{"client_credentials", "urn:ietf:params:oauth:grant-type:token-exchange"},
		}, nil)
	mockActorProvider.EXPECT().AuthenticateActor(mock.Anything,
		map[string]interface{}{authnprovidercm.UserAttributeUserID: "test-app"},
		map[string]interface{}{fieldFlowSecret: "wrong-secret"}).
		Return(&tidcommon.ServiceError{Code: "AUTH-FAIL", Type: tidcommon.ClientErrorType})
	mockObservability.EXPECT().IsEnabled().Return(false)

	service := &flowExecService{
		actorProvider:    mockActorProvider,
		observabilitySvc: mockObservability,
		transactioner:    &stubTransactioner{},
	}

	flowStep, svcErr := service.Execute(context.Background(), "test-app", "",
		string(providers.FlowTypeAuthentication), "", false, "submit", map[string]string{}, "", "wrong-secret", "", nil)

	s.Nil(flowStep)
	s.NotNil(svcErr)
	s.Equal(ErrorFlowSecretInvalid.Code, svcErr.Code)
	s.Equal(tidcommon.ClientErrorType, svcErr.Type)
}

func (s *ServiceTestSuite) TestExecute_NewFlow_InitiationModeError_InternalError() {
	t := s.T()
	testConfig := &config.Config{}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)

	mockActorProvider := actorprovidermock.NewActorProviderMock(t)
	mockObservability := observabilitymock.NewObservabilityServiceInterfaceMock(t)
	mockActorProvider.EXPECT().GetInboundClientByID(mock.Anything, "test-app").Return(
		&providers.InboundClient{ID: "test-app"}, nil)
	mockActorProvider.EXPECT().GetOAuthProfileByID(mock.Anything, "test-app").Return(
		(*providers.OAuthProfile)(nil), &tidcommon.InternalServerError)
	mockObservability.EXPECT().IsEnabled().Return(false)

	service := &flowExecService{
		actorProvider:    mockActorProvider,
		observabilitySvc: mockObservability,
		transactioner:    &stubTransactioner{},
	}

	flowStep, svcErr := service.Execute(context.Background(), "test-app", "",
		string(providers.FlowTypeAuthentication), "", false, "submit", map[string]string{}, "", "", "", nil)

	s.Nil(flowStep)
	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

// An embedded server-side app has no OAuth profile. It must present a valid Flow Secret, and when
// it does the flow initiates normally.
func (s *ServiceTestSuite) TestExecute_NewFlow_EmbeddedApp_ValidSecret_Allowed() {
	t := s.T()
	testConfig := &config.Config{}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)

	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	testGraph := flowFactory.CreateGraph("auth-graph-1", providers.FlowTypeAuthentication, 1)

	mockStore := newFlowStoreInterfaceMock(t)
	mockFlowProvider := NewFlowProviderMock(t)
	mockGraphBuilder := NewGraphBuilderInterfaceMock(t)
	mockEngine := newFlowEngineInterfaceMock(t)
	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(t)
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)
	mockAuthn := managermock.NewAuthnProviderManagerMock(t)

	// No OAuth profile — the embedded app case.
	mockInboundClient.EXPECT().GetOAuthProfileByEntityID(mock.Anything, "test-app").Return(nil, nil)
	mockAuthn.EXPECT().AuthenticateUser(mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, nil, nil)
	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, "test-app").Return(
		&inboundmodel.InboundClient{ID: "test-app", AuthFlowID: "auth-graph-1"}, nil).Times(3)
	mockEntityProvider.EXPECT().GetEntityProfile(mock.Anything, "test-app").Return(
		&providers.Entity{ID: "test-app", Category: providers.EntityCategoryApp},
		(*entityprovider.EntityProviderError)(nil))
	mockFlowProvider.EXPECT().
		GetFlow(mock.Anything, "auth-graph-1").
		Return(&providers.CompleteFlowDefinition{ID: "auth-graph-1"}, nil)
	mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(testGraph, nil)

	completedStep := FlowStep{Status: providers.FlowStatusComplete}
	mockEngine.EXPECT().Execute(mock.Anything).Return(completedStep, (*tidcommon.ServiceError)(nil))

	service := &flowExecService{
		flowStore:     mockStore,
		graphBuilder:  mockGraphBuilder,
		flowProvider:  mockFlowProvider,
		flowEngine:    mockEngine,
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, mockAuthn, nil, nil, nil),
		transactioner: &stubTransactioner{},
	}

	flowStep, svcErr := service.Execute(context.Background(), "test-app", "",
		string(providers.FlowTypeAuthentication), "", false, "submit", map[string]string{}, "", "valid-secret", "", nil)

	s.Nil(svcErr)
	s.NotNil(flowStep)
}

// An embedded server-side app (no OAuth profile) that omits the Flow Secret is rejected.
func (s *ServiceTestSuite) TestExecute_NewFlow_EmbeddedApp_MissingSecret_Rejected() {
	t := s.T()
	testConfig := &config.Config{}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)

	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(t)
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)
	mockObservability := observabilitymock.NewObservabilityServiceInterfaceMock(t)

	mockInboundClient.EXPECT().GetOAuthProfileByEntityID(mock.Anything, "test-app").Return(nil, nil)
	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, "test-app").Return(
		&inboundmodel.InboundClient{ID: "test-app", AuthFlowID: "auth-graph-1"}, nil)
	mockObservability.EXPECT().IsEnabled().Return(false)

	service := &flowExecService{
		actorProvider: actorprovider.Initialize(
			mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil, nil, nil),
		observabilitySvc: mockObservability,
		transactioner:    &stubTransactioner{},
	}

	flowStep, svcErr := service.Execute(context.Background(), "test-app", "",
		string(providers.FlowTypeAuthentication), "", false, "submit", map[string]string{}, "", "", "", nil)

	s.Nil(flowStep)
	s.NotNil(svcErr)
	s.Equal(ErrorFlowSecretRequired.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestExecute_ContinuationFlow_AuthCodeApp_NotBlocked() {
	t := s.T()
	testConfig := &config.Config{}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)

	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	testGraph := flowFactory.CreateGraph("auth-graph-1", providers.FlowTypeAuthentication, 1)

	mockStore := newFlowStoreInterfaceMock(t)
	mockFlowProvider := NewFlowProviderMock(t)
	mockGraphBuilder := NewGraphBuilderInterfaceMock(t)
	mockEngine := newFlowEngineInterfaceMock(t)
	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(t)
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)

	engineCtx := EngineContext{
		ExecutionID:      "existing-execution-id",
		AppID:            "test-app",
		FlowType:         providers.FlowTypeAuthentication,
		Graph:            testGraph,
		UserInputs:       map[string]string{},
		RuntimeData:      map[string]string{},
		ExecutionHistory: map[string]*providers.NodeExecutionRecord{},
	}
	storedCtx := &FlowContextDB{}
	err := storedCtx.FromEngineContext(engineCtx)
	s.NoError(err)

	mockStore.EXPECT().GetFlowContext(mock.Anything, "existing-execution-id").Return(storedCtx, nil)
	mockFlowProvider.EXPECT().
		GetFlow(mock.Anything, "auth-graph-1").
		Return(&providers.CompleteFlowDefinition{ID: "auth-graph-1"}, nil)
	mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(testGraph, nil)
	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, "test-app").Return(
		&inboundmodel.InboundClient{ID: "test-app", AuthFlowID: "auth-graph-1"}, nil)
	mockEntityProvider.EXPECT().GetEntityProfile(mock.Anything, "test-app").Return(
		&providers.Entity{ID: "test-app", Category: providers.EntityCategoryApp},
		(*entityprovider.EntityProviderError)(nil))

	completedStep := FlowStep{Status: providers.FlowStatusComplete}
	mockEngine.EXPECT().Execute(mock.Anything).Return(completedStep, (*tidcommon.ServiceError)(nil))
	mockStore.EXPECT().DeleteFlowContext(mock.Anything, "existing-execution-id").Return(nil)

	service := &flowExecService{
		flowStore:     mockStore,
		graphBuilder:  mockGraphBuilder,
		flowProvider:  mockFlowProvider,
		flowEngine:    mockEngine,
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil, nil, nil),
		transactioner: &stubTransactioner{},
	}

	flowStep, svcErr := service.Execute(context.Background(), "test-app", "existing-execution-id",
		string(providers.FlowTypeAuthentication), "", false, "submit", map[string]string{}, "valid-token", "", "", nil)

	s.Nil(svcErr)
	s.NotNil(flowStep)
}

// The invitee continues an invitation flow without a token; the invite token carries the step instead, and the
// step runs with the server's authority. The role step assigns the roles as the admin it recorded.
func (s *ServiceTestSuite) TestExecute_ContinuationOfInvitationFlowWithoutToken() {
	t := s.T()
	security.InitControlPlanePermissions("")
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime(t.TempDir(), &config.Config{}))
	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	testGraph := flowFactory.CreateGraph("invitation-1", providers.FlowTypeInvitation, 1)

	mockStore := newFlowStoreInterfaceMock(t)
	mockFlowProvider := NewFlowProviderMock(t)
	mockGraphBuilder := NewGraphBuilderInterfaceMock(t)
	mockEngine := newFlowEngineInterfaceMock(t)

	storedCtx := &FlowContextDB{}
	s.NoError(storedCtx.FromEngineContext(EngineContext{
		ExecutionID:      "existing-execution-id",
		FlowType:         providers.FlowTypeInvitation,
		Graph:            testGraph,
		UserInputs:       map[string]string{},
		RuntimeData:      map[string]string{common.RuntimeKeyOUID: "ou-1"},
		ExecutionHistory: map[string]*providers.NodeExecutionRecord{},
	}))
	mockStore.EXPECT().GetFlowContext(mock.Anything, "existing-execution-id").Return(storedCtx, nil)
	mockFlowProvider.EXPECT().GetFlow(mock.Anything, "invitation-1").Return(
		&providers.CompleteFlowDefinition{ID: "invitation-1", FlowType: providers.FlowTypeInvitation}, nil)
	mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(testGraph, nil)
	mockEngine.EXPECT().Execute(mock.MatchedBy(func(engineCtx *EngineContext) bool {
		return security.IsRuntimeContext(engineCtx.Context) && !security.IsRecordedCaller(engineCtx.Context)
	})).Return(FlowStep{Status: providers.FlowStatusComplete}, (*tidcommon.ServiceError)(nil))
	mockStore.EXPECT().DeleteFlowContext(mock.Anything, "existing-execution-id").Return(nil)

	service := &flowExecService{
		flowStore:    mockStore,
		graphBuilder: mockGraphBuilder,
		flowProvider: mockFlowProvider,
		flowEngine:   mockEngine,
		actorProvider: actorprovider.Initialize(inboundclientmock.NewInboundClientServiceInterfaceMock(t),
			entityprovidermock.NewEntityProviderInterfaceMock(t), noopAuthnMgr(), nil, nil, nil),
		transactioner: &stubTransactioner{},
	}

	flowStep, svcErr := service.Execute(security.WithRuntimeContext(context.Background()), "",
		"existing-execution-id", "", "", false, "accept", map[string]string{}, "", "", "", nil)

	s.Nil(svcErr)
	s.NotNil(flowStep)
}

// A continuation reloads its flow as the server: the flow was authorized when the execution started, and an
// organization administrator below the unit that owns it could not read it again as itself.
func (s *ServiceTestSuite) TestExecute_ContinuationLoadsTheFlowAsTheServer() {
	t := s.T()
	security.InitDataPlanePermissions("")
	defer security.InitControlPlanePermissions("")
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime(t.TempDir(), &config.Config{}))
	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	testGraph := flowFactory.CreateGraph("invitation-1", providers.FlowTypeInvitation, 1)

	mockStore := newFlowStoreInterfaceMock(t)
	mockFlowProvider := NewFlowProviderMock(t)
	mockGraphBuilder := NewGraphBuilderInterfaceMock(t)
	mockEngine := newFlowEngineInterfaceMock(t)

	storedCtx := &FlowContextDB{}
	s.NoError(storedCtx.FromEngineContext(EngineContext{
		ExecutionID:      "existing-execution-id",
		FlowType:         providers.FlowTypeInvitation,
		Graph:            testGraph,
		UserInputs:       map[string]string{},
		RuntimeData:      map[string]string{common.RuntimeKeyOUID: "ou-1"},
		ExecutionHistory: map[string]*providers.NodeExecutionRecord{},
	}))
	mockStore.EXPECT().GetFlowContext(mock.Anything, "existing-execution-id").Return(storedCtx, nil)
	mockFlowProvider.EXPECT().GetFlow(mock.MatchedBy(security.IsRuntimeContext), "invitation-1").Return(
		&providers.CompleteFlowDefinition{ID: "invitation-1", FlowType: providers.FlowTypeInvitation}, nil)
	mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(testGraph, nil)
	mockEngine.EXPECT().Execute(mock.Anything).Return(
		FlowStep{Status: providers.FlowStatusComplete}, (*tidcommon.ServiceError)(nil))
	mockStore.EXPECT().DeleteFlowContext(mock.Anything, "existing-execution-id").Return(nil)

	service := &flowExecService{
		flowStore:    mockStore,
		graphBuilder: mockGraphBuilder,
		flowProvider: mockFlowProvider,
		flowEngine:   mockEngine,
		actorProvider: actorprovider.Initialize(inboundclientmock.NewInboundClientServiceInterfaceMock(t),
			entityprovidermock.NewEntityProviderInterfaceMock(t), noopAuthnMgr(), nil, nil, nil),
		transactioner: &stubTransactioner{},
	}

	flowStep, svcErr := service.Execute(authenticatedAdminContext([]string{"org"}), "",
		"existing-execution-id", "", "", false, "action_roles", map[string]string{}, "", "", "", nil)

	s.Nil(svcErr)
	s.NotNil(flowStep)
}

// --- updateContext ---

func (s *ServiceTestSuite) TestUpdateContext_IncompleteEmptyExecutionID() {
	service := &flowExecService{cfg: testFlowExecCfg}
	engineCtx := &EngineContext{ExecutionID: ""}
	flowStep := &FlowStep{Status: providers.FlowStatusIncomplete}

	err := service.updateContext(context.Background(), engineCtx, flowStep, log.GetLogger())
	s.Error(err)
}

// --- checkDirectFlowInitiationAllowed ---

// An App-Secret-mode app (a backend / embedded server-side app) with no secret supplied is
// rejected rather than allowed through.
func (s *ServiceTestSuite) TestCheckDirectFlowInitiationAllowed_NoProfileRequiresFlowSecret() {
	t := s.T()
	mockActorProvider := actorprovidermock.NewActorProviderMock(t)
	// No OAuth profile, but the application exists (embedded server-side app) → FlowSecret mode.
	mockActorProvider.EXPECT().GetOAuthProfileByID(mock.Anything, "embedded-app").Return(
		(*providers.OAuthProfile)(nil), &actorprovider.ErrorActorNotFound)
	mockActorProvider.EXPECT().GetInboundClientByID(mock.Anything, "embedded-app").Return(
		&providers.InboundClient{ID: "embedded-app"}, nil)

	service := &flowExecService{
		actorProvider: mockActorProvider,
		cfg:           testFlowExecCfg,
	}

	svcErr := service.checkDirectFlowInitiationAllowed(context.Background(), "embedded-app",
		providers.FlowTypeAuthentication, "", "", log.GetLogger())
	s.NotNil(svcErr)
	s.Equal(ErrorFlowSecretRequired.Code, svcErr.Code)
}

// A non-existent application resolves to ErrorActorNotFound; the guard must surface that as an
// invalid-app-ID error rather than demanding an Flow Secret.
func (s *ServiceTestSuite) TestCheckDirectFlowInitiationAllowed_UnknownAppInvalidAppID() {
	t := s.T()
	mockActorProvider := actorprovidermock.NewActorProviderMock(t)
	mockActorProvider.EXPECT().GetInboundClientByID(mock.Anything, "app-notfound").Return(
		(*providers.InboundClient)(nil), &actorprovider.ErrorActorNotFound)

	service := &flowExecService{
		actorProvider: mockActorProvider,
		cfg:           testFlowExecCfg,
	}

	svcErr := service.checkDirectFlowInitiationAllowed(context.Background(), "app-notfound",
		providers.FlowTypeAuthentication, "", "", log.GetLogger())
	s.NotNil(svcErr)
	s.Equal(ErrorInvalidAppID.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestCheckDirectFlowInitiationAllowed_NonAuthFlowAllowed() {
	service := &flowExecService{cfg: testFlowExecCfg}

	svcErr := service.checkDirectFlowInitiationAllowed(context.Background(), "test-app",
		providers.FlowTypeRegistration, "", "", log.GetLogger())
	s.Nil(svcErr)
}

// attestationClient returns an inbound client configured with Android attestation, holding an
// already-encrypted service account credential.
func attestationClient() *providers.InboundClient {
	client := &providers.InboundClient{
		ID: "mobile-app",
		Properties: map[string]interface{}{
			applicationTypePropertyKey: string(model.ApplicationTypeMobile),
		},
		Attestation: &providers.AttestationConfig{
			Android: &providers.AndroidAttestationConfig{
				PackageName:               "com.example.app",
				ServiceAccountCredentials: "encrypted-creds",
			},
		},
	}
	return client
}

// A mobile app that has not configured attestation cannot initiate a flow.
func (s *ServiceTestSuite) TestCheckDirectFlowInitiationAllowed_AttestationNotConfigured() {
	t := s.T()
	mockActorProvider := actorprovidermock.NewActorProviderMock(t)
	mobileClient := &providers.InboundClient{
		ID: "mobile-app",
		Properties: map[string]interface{}{
			applicationTypePropertyKey: string(model.ApplicationTypeMobile),
		},
	}
	mockActorProvider.EXPECT().GetInboundClientByID(mock.Anything, "mobile-app").Return(mobileClient, nil)

	service := &flowExecService{
		actorProvider:       mockActorProvider,
		attestationVerifier: attestationprovidermock.NewAttestationProviderMock(t),
		cfg:                 testFlowExecCfg,
	}

	svcErr := service.checkDirectFlowInitiationAllowed(context.Background(), "mobile-app",
		providers.FlowTypeAuthentication, "", "", log.GetLogger())
	s.NotNil(svcErr)
	s.Equal(ErrorAttestationNotConfigured.Code, svcErr.Code)
}

// A mobile app with attestation configured but no token presented is rejected before any
// verification is attempted.
func (s *ServiceTestSuite) TestCheckDirectFlowInitiationAllowed_AttestationMissingToken() {
	t := s.T()
	mockActorProvider := actorprovidermock.NewActorProviderMock(t)
	mockActorProvider.EXPECT().GetInboundClientByID(mock.Anything, "mobile-app").Return(
		attestationClient(), nil)

	service := &flowExecService{
		actorProvider:       mockActorProvider,
		attestationVerifier: attestationprovidermock.NewAttestationProviderMock(t),
		cfg:                 testFlowExecCfg,
	}

	svcErr := service.checkDirectFlowInitiationAllowed(context.Background(), "mobile-app",
		providers.FlowTypeAuthentication, "", "", log.GetLogger())
	s.NotNil(svcErr)
	s.Equal(ErrorAttestationRequired.Code, svcErr.Code)
}

// A token that is definitively rejected by the provider (identity mismatch, unrecognized app, etc.)
// surfaces as an invalid attestation error.
func (s *ServiceTestSuite) TestCheckDirectFlowInitiationAllowed_AttestationInvalid() {
	t := s.T()
	mockActorProvider := actorprovidermock.NewActorProviderMock(t)
	mockActorProvider.EXPECT().GetInboundClientByID(mock.Anything, "mobile-app").Return(
		attestationClient(), nil)
	mockProvider := attestationprovidermock.NewAttestationProviderMock(t)
	mockProvider.EXPECT().Verify(mock.Anything, mock.Anything, "bad-token").
		Return(false, nil)

	service := &flowExecService{
		actorProvider:       mockActorProvider,
		attestationVerifier: mockProvider,
		cfg:                 testFlowExecCfg,
	}

	svcErr := service.checkDirectFlowInitiationAllowed(context.Background(), "mobile-app",
		providers.FlowTypeAuthentication, "", "bad-token", log.GetLogger())
	s.NotNil(svcErr)
	s.Equal(ErrorAttestationInvalid.Code, svcErr.Code)
}

// An operational provider failure (provider outage, decrypt failure, misconfiguration) is surfaced
// as a retriable server error, not a 401 — the caller should not treat a provider outage as an
// authentication failure.
func (s *ServiceTestSuite) TestCheckDirectFlowInitiationAllowed_AttestationVerifierUnavailable() {
	t := s.T()
	mockActorProvider := actorprovidermock.NewActorProviderMock(t)
	mockActorProvider.EXPECT().GetInboundClientByID(mock.Anything, "mobile-app").Return(
		attestationClient(), nil)
	mockProvider := attestationprovidermock.NewAttestationProviderMock(t)
	mockProvider.EXPECT().Verify(mock.Anything, mock.Anything, "some-token").
		Return(false, &tidcommon.InternalServerError)

	service := &flowExecService{
		actorProvider:       mockActorProvider,
		attestationVerifier: mockProvider,
		cfg:                 testFlowExecCfg,
	}

	svcErr := service.checkDirectFlowInitiationAllowed(context.Background(), "mobile-app",
		providers.FlowTypeAuthentication, "", "some-token", log.GetLogger())
	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

// A token verified by the attestation provider permits direct flow initiation.
func (s *ServiceTestSuite) TestCheckDirectFlowInitiationAllowed_AttestationValid() {
	t := s.T()
	mockActorProvider := actorprovidermock.NewActorProviderMock(t)
	mockActorProvider.EXPECT().GetInboundClientByID(mock.Anything, "mobile-app").Return(
		attestationClient(), nil)
	mockProvider := attestationprovidermock.NewAttestationProviderMock(t)
	mockProvider.EXPECT().Verify(mock.Anything, mock.MatchedBy(func(cfg *providers.AttestationConfig) bool {
		return cfg != nil && cfg.Android != nil && cfg.Android.ServiceAccountCredentials == "encrypted-creds"
	}), "good-token").Return(true, nil)

	service := &flowExecService{
		actorProvider:       mockActorProvider,
		attestationVerifier: mockProvider,
		cfg:                 testFlowExecCfg,
	}

	svcErr := service.checkDirectFlowInitiationAllowed(context.Background(), "mobile-app",
		providers.FlowTypeAuthentication, "", "good-token", log.GetLogger())
	s.Nil(svcErr)
}

// appleAttestationClient returns an inbound client configured with Apple App Attest attestation.
func appleAttestationClient() *providers.InboundClient {
	client := &providers.InboundClient{
		ID: "mobile-app",
		Properties: map[string]interface{}{
			applicationTypePropertyKey: string(model.ApplicationTypeMobile),
		},
		Attestation: &providers.AttestationConfig{
			Apple: &providers.AppleAttestationConfig{TeamID: "TEAM123", BundleID: "com.example.app"},
		},
	}
	return client
}

// An Apple-configured client also resolves to attestation-based flow initiation, and a token verified
// by the provider permits direct flow initiation.
func (s *ServiceTestSuite) TestCheckDirectFlowInitiationAllowed_AppleAttestationValid() {
	t := s.T()
	mockActorProvider := actorprovidermock.NewActorProviderMock(t)
	mockActorProvider.EXPECT().GetInboundClientByID(mock.Anything, "mobile-app").Return(
		appleAttestationClient(), nil)
	mockProvider := attestationprovidermock.NewAttestationProviderMock(t)
	mockProvider.EXPECT().Verify(mock.Anything, mock.MatchedBy(func(cfg *providers.AttestationConfig) bool {
		return cfg != nil && cfg.Apple != nil && cfg.Apple.TeamID == "TEAM123"
	}), "good-token").Return(true, nil)

	service := &flowExecService{
		actorProvider:       mockActorProvider,
		attestationVerifier: mockProvider,
		cfg:                 testFlowExecCfg,
	}

	svcErr := service.checkDirectFlowInitiationAllowed(context.Background(), "mobile-app",
		providers.FlowTypeAuthentication, "", "good-token", log.GetLogger())
	s.Nil(svcErr)
}

// A mobile app with attestation dev mode enabled may initiate a flow directly without presenting an
// attestation token, and no verification is attempted.
func (s *ServiceTestSuite) TestCheckDirectFlowInitiationAllowed_DevModeSkipsAttestation() {
	t := s.T()
	mockActorProvider := actorprovidermock.NewActorProviderMock(t)
	devModeClient := &providers.InboundClient{
		ID: "mobile-app",
		Properties: map[string]interface{}{
			applicationTypePropertyKey: string(model.ApplicationTypeMobile),
		},
		Attestation: &providers.AttestationConfig{DevMode: true},
	}
	mockActorProvider.EXPECT().GetInboundClientByID(mock.Anything, "mobile-app").Return(devModeClient, nil)

	service := &flowExecService{
		actorProvider: mockActorProvider,
		cfg:           testFlowExecCfg,
	}

	svcErr := service.checkDirectFlowInitiationAllowed(context.Background(), "mobile-app",
		providers.FlowTypeAuthentication, "", "", log.GetLogger())
	s.Nil(svcErr)
}

// --- getFlowContext ---

func (s *ServiceTestSuite) TestGetFlowContext_NilDbModel() {
	t := s.T()
	mockStore := newFlowStoreInterfaceMock(t)
	mockStore.EXPECT().GetFlowContext(mock.Anything, "exec-nil").Return(nil, nil)

	service := &flowExecService{
		flowStore: mockStore,
		cfg:       testFlowExecCfg,
	}

	result, svcErr := service.getFlowContext(context.Background(), "exec-nil", log.GetLogger())
	s.Nil(result)
	s.NotNil(svcErr)
	s.Equal(ErrorInvalidExecutionID.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestGetFlowContext_StoreError() {
	t := s.T()
	mockStore := newFlowStoreInterfaceMock(t)
	mockStore.EXPECT().GetFlowContext(mock.Anything, "exec-err").Return(nil, errors.New("store failure"))

	service := &flowExecService{
		flowStore: mockStore,
		cfg:       testFlowExecCfg,
	}

	result, svcErr := service.getFlowContext(context.Background(), "exec-err", log.GetLogger())
	s.Nil(result)
	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestLoadContextFromStore_ToEngineContextError() {
	t := s.T()
	mockStore := newFlowStoreInterfaceMock(t)
	mockFlowProvider := NewFlowProviderMock(t)
	mockGraphBuilder := NewGraphBuilderInterfaceMock(t)

	// Context has invalid JSON for userInputs to force ToEngineContext error.
	rawCtx := "{\"executionID\":\"exec-2\",\"appID\":\"app-1\",\"flowType\":\"AUTHENTICATION\"," +
		"\"graphID\":\"graph-1\",\"currentNodeID\":\"node-1\",\"userInputs\":\"not-valid-json\"," +
		"\"runtimeData\":\"{}\",\"executionHistory\":\"{}\"}"
	dbModel := &FlowContextDB{
		ExecutionID: "exec-2",
		Context:     rawCtx,
	}
	mockStore.EXPECT().GetFlowContext(mock.Anything, "exec-2").Return(dbModel, nil)

	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.EXPECT().GetNode(mock.Anything).Return(nil, false).Maybe()
	mockGraph.EXPECT().GetType().Return(providers.FlowType("AUTHENTICATION")).Maybe()
	mockFlowProvider.EXPECT().GetFlow(mock.Anything, mock.Anything).Return(&providers.CompleteFlowDefinition{}, nil)
	mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(mockGraph, nil)

	service := &flowExecService{
		flowStore:    mockStore,
		graphBuilder: mockGraphBuilder,
		flowProvider: mockFlowProvider,
		cfg:          testFlowExecCfg,
	}

	result, svcErr := service.loadContextFromStore(context.Background(), "exec-2", log.GetLogger())
	s.Nil(result)
	s.NotNil(svcErr)
}

func (s *ServiceTestSuite) TestLoadContextFromStore_GetFlowGraphError() {
	t := s.T()
	mockStore := newFlowStoreInterfaceMock(t)
	mockFlowProvider := NewFlowProviderMock(t)

	rawCtx := "{\"executionID\":\"exec-1\",\"appID\":\"app-1\",\"flowType\":\"AUTHENTICATION\"," +
		"\"graphID\":\"graph-1\",\"currentNodeID\":\"node-1\",\"userInputs\":\"{}\"," +
		"\"runtimeData\":\"{}\",\"executionHistory\":\"{}\"}"
	dbModel := &FlowContextDB{
		ExecutionID: "exec-1",
		Context:     rawCtx,
	}
	mockStore.EXPECT().GetFlowContext(mock.Anything, "exec-1").Return(dbModel, nil)
	mockFlowProvider.EXPECT().GetFlow(mock.Anything, mock.Anything).Return(nil, &tidcommon.InternalServerError)

	service := &flowExecService{
		flowStore:    mockStore,
		flowProvider: mockFlowProvider,
		cfg:          testFlowExecCfg,
	}

	result, svcErr := service.loadContextFromStore(context.Background(), "exec-1", log.GetLogger())
	s.Nil(result)
	s.NotNil(svcErr)
}

// ----- withoutConstantInputs -----

func (s *ServiceTestSuite) TestWithoutConstantInputs_DropsOnlyConstants() {
	graph := coremock.NewGraphInterfaceMock(s.T())
	graph.EXPECT().IsConstantInput("username").Return(false)
	graph.EXPECT().IsConstantInput("faculty").Return(true)

	accepted := withoutConstantInputs(graph, map[string]string{"username": "alice", "faculty": "Arts"})

	s.Equal(map[string]string{"username": "alice"}, accepted)
}

func (s *ServiceTestSuite) TestWithoutConstantInputs_WithoutAGraphKeepsEverything() {
	inputs := map[string]string{"username": "alice"}

	s.Equal(inputs, withoutConstantInputs(nil, inputs))
}

func (s *ServiceTestSuite) TestWithoutConstantInputs_NothingSubmitted() {
	graph := coremock.NewGraphInterfaceMock(s.T())

	s.Empty(withoutConstantInputs(graph, nil))
}

// graphWithConstantInput returns a graph whose only node holds the locale input constant.
func (s *ServiceTestSuite) graphWithConstantInput(flowType providers.FlowType) core.GraphInterface {
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), &config.Config{}))
	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	graph := flowFactory.CreateGraph("graph-1", flowType, 1)
	node, err := flowFactory.CreateNode("node-1", string(common.NodeTypeTaskExecution),
		map[string]interface{}{}, false, false)
	s.Require().NoError(err)
	node.(core.ExecutorBackedNodeInterface).SetInputs(
		[]providers.Input{{Identifier: "locale", DefaultValue: "en-US", Constant: true}})
	s.Require().NoError(graph.AddNode(node))
	return graph
}

// A client that starts a flow cannot set an input the flow holds constant.
func (s *ServiceTestSuite) TestLoadNewContext_IgnoresConstantInputs() {
	security.InitControlPlanePermissions("")
	graph := s.graphWithConstantInput(providers.FlowTypeAdministration)
	flow := &providers.CompleteFlowDefinition{
		ID: "administration-1", FlowType: providers.FlowTypeAdministration, ActiveVersion: 1,
	}
	mockFlowProvider := NewFlowProviderMock(s.T())
	mockFlowProvider.EXPECT().GetFlow(mock.Anything, "administration-1").Return(flow, nil).Once()
	mockGraphBuilder := NewGraphBuilderInterfaceMock(s.T())
	mockGraphBuilder.EXPECT().GetGraph(mock.Anything, flow).Return(graph, nil)
	service := &flowExecService{
		flowProvider: mockFlowProvider, graphBuilder: mockGraphBuilder, cfg: testFlowExecCfg,
		flowVisibility: s.administrationFlowVisible(),
	}

	engineCtx, svcErr := service.loadNewContext(authenticatedAdminContext([]string{"system:user"}),
		"administration-1", "", "", "", true, "",
		map[string]string{"subject": "user-1", "locale": "fr-FR"}, "", "", log.GetLogger())

	s.Nil(svcErr)
	s.Require().NotNil(engineCtx)
	s.Equal("user-1", engineCtx.UserInputs["subject"])
	s.NotContains(engineCtx.UserInputs, "locale")
}

// A client that continues a flow cannot set an input the flow holds constant either.
func (s *ServiceTestSuite) TestLoadPrevContext_IgnoresConstantInputs() {
	t := s.T()
	graph := s.graphWithConstantInput(providers.FlowTypeAdministration)
	rawCtx := "{\"executionID\":\"exec-1\",\"appID\":\"app-1\",\"flowType\":\"ADMINISTRATION\"," +
		"\"graphID\":\"graph-1\",\"currentNodeID\":\"node-1\",\"userInputs\":\"{}\"," +
		"\"runtimeData\":\"{}\",\"executionHistory\":\"{}\"}"
	mockStore := newFlowStoreInterfaceMock(t)
	mockStore.EXPECT().GetFlowContext(mock.Anything, "exec-1").
		Return(&FlowContextDB{ExecutionID: "exec-1", Context: rawCtx}, nil)
	mockFlowProvider := NewFlowProviderMock(t)
	mockFlowProvider.EXPECT().GetFlow(mock.Anything, mock.Anything).Return(&providers.CompleteFlowDefinition{}, nil)
	mockGraphBuilder := NewGraphBuilderInterfaceMock(t)
	mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(graph, nil)
	service := &flowExecService{
		flowStore: mockStore, graphBuilder: mockGraphBuilder, flowProvider: mockFlowProvider, cfg: testFlowExecCfg,
	}

	engineCtx, svcErr := service.loadPrevContext(context.Background(), "exec-1", "",
		map[string]string{"username": "alice", "locale": "fr-FR"}, log.GetLogger())

	s.Nil(svcErr)
	s.Require().NotNil(engineCtx)
	s.Equal("alice", engineCtx.UserInputs["username"])
	s.NotContains(engineCtx.UserInputs, "locale")
}

// The initial inputs of a flow started by the server, such as the login hint of a CIBA request, come
// from a client, so they cannot set an input the flow holds constant either.
func (s *ServiceTestSuite) TestInitiateAndExecute_IgnoresConstantInitialInputs() {
	t := s.T()
	appID := "test-app-constant"
	graph := s.graphWithConstantInput(providers.FlowTypeAuthentication)

	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(t)
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)
	mockFlowProvider := NewFlowProviderMock(t)
	mockGraphBuilder := NewGraphBuilderInterfaceMock(t)
	mockCrypto := cryptomock.NewRuntimeCryptoProviderMock(t)
	mockStore := newFlowStoreInterfaceMock(t)
	mockEngine := newFlowEngineInterfaceMock(t)

	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, appID).Return(
		&inboundmodel.InboundClient{ID: appID, AuthFlowID: "auth-graph-constant"}, nil)
	mockEntityProvider.EXPECT().GetEntityProfile(mock.Anything, appID).Return(
		&providers.Entity{ID: appID, Category: providers.EntityCategoryApp}, nil)
	mockFlowProvider.EXPECT().GetFlow(mock.Anything, "auth-graph-constant").
		Return(&providers.CompleteFlowDefinition{ID: "auth-graph-constant"}, nil)
	mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(graph, nil)
	mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]byte("encrypted"), nil, nil)
	mockStore.EXPECT().StoreFlowContext(mock.Anything, mock.Anything, mock.Anything).Return(nil)

	var capturedCtx *EngineContext
	mockEngine.EXPECT().Execute(mock.MatchedBy(func(ctx *EngineContext) bool {
		capturedCtx = ctx
		return true
	})).Return(FlowStep{Status: providers.FlowStatusIncomplete}, nil)

	service := &flowExecService{
		flowStore:     mockStore,
		graphBuilder:  mockGraphBuilder,
		flowProvider:  mockFlowProvider,
		flowEngine:    mockEngine,
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil, nil, nil),
		transactioner: &stubTransactioner{},
		cryptoSvc:     mockCrypto,
		cfg:           testFlowExecCfg,
	}

	step, svcErr := service.InitiateAndExecute(context.Background(), &FlowInitContext{
		ApplicationID: appID,
		FlowType:      "AUTHENTICATION",
		InitialInputs: map[string]string{"username": "alice", "locale": "fr-FR"},
	})

	s.Nil(svcErr)
	s.NotNil(step)
	s.Require().NotNil(capturedCtx)
	s.Equal("alice", capturedCtx.UserInputs["username"])
	s.NotContains(capturedCtx.UserInputs, "locale")
}

// ----- firstPositiveExpiry -----

func (s *ServiceTestSuite) TestFirstPositiveExpiry_PositiveVWins() {
	s.Equal(int64(300), firstPositiveExpiry(300, 1800))
}

func (s *ServiceTestSuite) TestFirstPositiveExpiry_ZeroVFallsBack() {
	s.Equal(int64(1800), firstPositiveExpiry(0, 1800))
}

func (s *ServiceTestSuite) TestFirstPositiveExpiry_NegativeVFallsBack() {
	s.Equal(int64(1800), firstPositiveExpiry(-5, 1800))
}

// ----- resolveDefaultFlowHandle -----

func (s *ServiceTestSuite) TestResolveDefaultFlowHandle_Authentication() {
	svc := &flowExecService{
		serverConfigSvc: stubServerConfig{cfg: flowconfig.FlowSectionConfig{
			AuthFlow: flowconfig.FlowTypeConfig{DefaultHandle: "h-auth"},
		}},
		cfg: testFlowExecCfg,
	}
	s.Equal("h-auth", svc.resolveDefaultFlowHandle(context.Background(), providers.FlowTypeAuthentication))
}

func (s *ServiceTestSuite) TestResolveDefaultFlowHandle_Registration() {
	svc := &flowExecService{
		serverConfigSvc: stubServerConfig{cfg: flowconfig.FlowSectionConfig{
			RegistrationFlow: flowconfig.FlowTypeConfig{DefaultHandle: "h-reg"},
		}},
		cfg: testFlowExecCfg,
	}
	s.Equal("h-reg", svc.resolveDefaultFlowHandle(context.Background(), providers.FlowTypeRegistration))
}

func (s *ServiceTestSuite) TestResolveDefaultFlowHandle_UserOnboarding() {
	svc := &flowExecService{
		serverConfigSvc: stubServerConfig{cfg: flowconfig.FlowSectionConfig{
			UserOnboardingFlow: flowconfig.FlowTypeConfig{DefaultHandle: "h-onboard"},
		}},
		cfg: testFlowExecCfg,
	}
	s.Equal("h-onboard", svc.resolveDefaultFlowHandle(context.Background(), providers.FlowTypeUserOnboarding))
}

func (s *ServiceTestSuite) TestResolveDefaultFlowHandle_Recovery() {
	svc := &flowExecService{
		serverConfigSvc: stubServerConfig{cfg: flowconfig.FlowSectionConfig{
			RecoveryFlow: flowconfig.FlowTypeConfig{DefaultHandle: "h-recovery"},
		}},
		cfg: testFlowExecCfg,
	}
	s.Equal("h-recovery", svc.resolveDefaultFlowHandle(context.Background(), providers.FlowTypeRecovery))
}

func (s *ServiceTestSuite) TestResolveDefaultFlowHandle_SignOut() {
	svc := &flowExecService{
		serverConfigSvc: stubServerConfig{cfg: flowconfig.FlowSectionConfig{
			SignOutFlow: flowconfig.FlowTypeConfig{DefaultHandle: "h-signout"},
		}},
		cfg: testFlowExecCfg,
	}
	s.Equal("h-signout", svc.resolveDefaultFlowHandle(context.Background(), providers.FlowTypeSignOut))
}

func (s *ServiceTestSuite) TestResolveDefaultFlowHandle_UnknownFlowTypeReturnsEmpty() {
	svc := &flowExecService{
		serverConfigSvc: stubServerConfig{cfg: flowconfig.FlowSectionConfig{}},
		cfg:             testFlowExecCfg,
	}
	s.Empty(svc.resolveDefaultFlowHandle(context.Background(), providers.FlowType("UNKNOWN")))
}

func (s *ServiceTestSuite) TestResolveDefaultFlowHandle_NilServerConfig() {
	svc := &flowExecService{cfg: testFlowExecCfg}
	s.Empty(svc.resolveDefaultFlowHandle(context.Background(), providers.FlowTypeAuthentication))
}

// ----- getFlowExpirySeconds with serverconfig -----

func (s *ServiceTestSuite) TestGetFlowExpirySeconds_AuthFlowServerConfigOverridesDefault() {
	svc := &flowExecService{
		serverConfigSvc: stubServerConfig{cfg: flowconfig.FlowSectionConfig{
			AuthFlow: flowconfig.FlowTypeConfig{ExpirySeconds: 600},
		}},
		cfg: testFlowExecCfg,
	}
	s.Equal(int64(600), svc.getFlowExpirySeconds(context.Background(), providers.FlowTypeAuthentication))
}

func (s *ServiceTestSuite) TestGetFlowExpirySeconds_RegistrationFlowServerConfigOverridesDefault() {
	svc := &flowExecService{
		serverConfigSvc: stubServerConfig{cfg: flowconfig.FlowSectionConfig{
			RegistrationFlow: flowconfig.FlowTypeConfig{ExpirySeconds: 7200},
		}},
		cfg: testFlowExecCfg,
	}
	s.Equal(int64(7200), svc.getFlowExpirySeconds(context.Background(), providers.FlowTypeRegistration))
}

func (s *ServiceTestSuite) TestGetFlowExpirySeconds_UserOnboardingFlowServerConfigOverridesDefault() {
	svc := &flowExecService{
		serverConfigSvc: stubServerConfig{cfg: flowconfig.FlowSectionConfig{
			UserOnboardingFlow: flowconfig.FlowTypeConfig{ExpirySeconds: 43200},
		}},
		cfg: testFlowExecCfg,
	}
	s.Equal(int64(43200), svc.getFlowExpirySeconds(context.Background(), providers.FlowTypeUserOnboarding))
}

func (s *ServiceTestSuite) TestGetFlowExpirySeconds_SignOutFlowServerConfigOverridesDefault() {
	svc := &flowExecService{
		serverConfigSvc: stubServerConfig{cfg: flowconfig.FlowSectionConfig{
			SignOutFlow: flowconfig.FlowTypeConfig{ExpirySeconds: 120},
		}},
		cfg: testFlowExecCfg,
	}
	s.Equal(int64(120), svc.getFlowExpirySeconds(context.Background(), providers.FlowTypeSignOut))
}

func (s *ServiceTestSuite) TestGetFlowExpirySeconds_SignOutFallsBackToDefault() {
	svc := &flowExecService{
		serverConfigSvc: stubServerConfig{cfg: flowconfig.FlowSectionConfig{}}, cfg: testFlowExecCfg,
	}
	s.Equal(defaultSignOutFlowExpiry, svc.getFlowExpirySeconds(context.Background(), providers.FlowTypeSignOut))
}

// noopAuthnMgr returns an authentication-provider mock with no expectations, for tests that
// build a real actor provider but never exercise actor authentication.
func noopAuthnMgr() *managermock.AuthnProviderManagerMock {
	return &managermock.AuthnProviderManagerMock{}
}

func (s *ServiceTestSuite) TestRefuseHeldApplicationSeparatesAFaultFromAHold() {
	faulty := identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(s.T())
	faulty.EXPECT().EvaluateAccess(mock.Anything, "app-1", providers.AccessPlaneApplication,
		providers.AuthenticationMethod("")).
		Return(providers.AccessDecision{}, &tidcommon.InternalServerError).Once()
	svc := &flowExecService{governance: faulty}
	svcErr := svc.refuseHeldApplication(context.Background(), "app-1", log.GetLogger())
	s.Require().NotNil(svcErr)
	s.Require().Equal(tidcommon.InternalServerError.Code, svcErr.Code)

	held := identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(s.T())
	held.EXPECT().EvaluateAccess(mock.Anything, "app-1", providers.AccessPlaneApplication,
		providers.AuthenticationMethod("")).
		Return(providers.AccessDecision{Admitted: false}, nil).Once()
	svc = &flowExecService{governance: held}
	svcErr = svc.refuseHeldApplication(context.Background(), "app-1", log.GetLogger())
	s.Require().NotNil(svcErr)
	s.Require().Equal(ErrorFlowApplicationNotUsable.Code, svcErr.Code)

	admitted := identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(s.T())
	admitted.EXPECT().EvaluateAccess(mock.Anything, "app-1", providers.AccessPlaneApplication,
		providers.AuthenticationMethod("")).
		Return(providers.AccessDecision{Admitted: true}, nil).Once()
	svc = &flowExecService{governance: admitted}
	s.Nil(svc.refuseHeldApplication(context.Background(), "app-1", log.GetLogger()))

	// A flow with no application has nothing to check.
	unchecked := identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(s.T())
	svc = &flowExecService{governance: unchecked}
	s.Nil(svc.refuseHeldApplication(context.Background(), "", log.GetLogger()))
}

func (s *ServiceTestSuite) TestExecuteRefusesAHeldApplicationOnAContinuingStep() {
	t := s.T()
	s.Require().NoError(config.InitializeServerRuntime(t.TempDir(), &config.Config{}))
	defer config.ResetServerRuntime()
	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	testGraph := flowFactory.CreateGraph("test-graph-id", providers.FlowTypeAuthentication, 1)

	storedCtx := &FlowContextDB{}
	s.Require().NoError(storedCtx.FromEngineContext(EngineContext{
		ExecutionID:      existingExecutionID,
		AppID:            "test-app-id",
		FlowType:         providers.FlowTypeAuthentication,
		UserInputs:       map[string]string{},
		RuntimeData:      map[string]string{},
		ExecutionHistory: map[string]*providers.NodeExecutionRecord{},
		Graph:            testGraph,
	}))

	mockStore := newFlowStoreInterfaceMock(t)
	mockFlowProvider := NewFlowProviderMock(t)
	mockGraphBuilder := NewGraphBuilderInterfaceMock(t)
	mockEngine := newFlowEngineInterfaceMock(t)
	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(t)
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)
	governance := identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(t)

	mockStore.EXPECT().GetFlowContext(mock.Anything, existingExecutionID).Return(storedCtx, nil)
	mockFlowProvider.EXPECT().GetFlow(mock.Anything, "test-graph-id").
		Return(&providers.CompleteFlowDefinition{ID: "test-graph-id"}, nil)
	mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(testGraph, nil)
	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, "test-app-id").Return(
		&inboundmodel.InboundClient{ID: "test-app-id", AuthFlowID: "test-graph-id"}, nil)
	mockEntityProvider.EXPECT().GetEntityProfile(mock.Anything, "test-app-id").Return(
		&providers.Entity{ID: "test-app-id", Category: providers.EntityCategoryApp},
		(*entityprovider.EntityProviderError)(nil))
	governance.EXPECT().EvaluateAccess(mock.Anything, "test-app-id", providers.AccessPlaneApplication,
		providers.AuthenticationMethod("")).
		Return(providers.AccessDecision{Admitted: false}, nil).Once()

	service := &flowExecService{
		flowStore:     mockStore,
		graphBuilder:  mockGraphBuilder,
		flowProvider:  mockFlowProvider,
		flowEngine:    mockEngine,
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil, nil, nil),
		transactioner: &stubTransactioner{},
		cfg:           testFlowExecCfg,
		governance:    governance,
	}

	flowStep, svcErr := service.Execute(context.Background(), "test-app", existingExecutionID,
		string(providers.FlowTypeAuthentication), "", false, "submit", map[string]string{}, "", "", "", nil)

	s.Nil(flowStep)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorFlowApplicationNotUsable.Code, svcErr.Code)
	mockEngine.AssertNotCalled(t, "Execute", mock.Anything)
	mockStore.AssertNotCalled(t, "UpdateFlowContext", mock.Anything, mock.Anything)
	mockStore.AssertNotCalled(t, "DeleteFlowContext", mock.Anything, mock.Anything)
}

// systemFlowService is an execution service with a flow provider, a graph builder and an engine, and
// nothing that loads an application or stores a context.
func (s *ServiceTestSuite) systemFlowService() (*flowExecService, *FlowProviderMock, *flowEngineInterfaceMock) {
	flows := NewFlowProviderMock(s.T())
	graphs := NewGraphBuilderInterfaceMock(s.T())
	graphs.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	engine := newFlowEngineInterfaceMock(s.T())
	return &flowExecService{flowProvider: flows, graphBuilder: graphs, flowEngine: engine, cfg: testFlowExecCfg},
		flows, engine
}

func systemFlow() *providers.CompleteFlowDefinition {
	return &providers.CompleteFlowDefinition{
		ID: "019a1f40-6a1c-7000-8000-5e5713d00001", Handle: "system-application-delete-pre",
		FlowType: providers.FlowTypeSystem,
	}
}

// A system flow runs to completion as one, with the inputs it was given and no application loaded.
func (s *ServiceTestSuite) TestExecuteSystemFlow_RunsToCompletion() {
	service, flows, engine := s.systemFlowService()
	flows.EXPECT().GetSystemFlow(mock.Anything, "system-application-delete-pre").Return(systemFlow(), nil)
	engine.EXPECT().Execute(mock.MatchedBy(func(ctx *EngineContext) bool {
		return ctx.FlowType == providers.FlowTypeSystem && ctx.UserInputs["targetApplicationId"] == "app-1" &&
			ctx.Application.ID == ""
	})).Run(func(ctx *EngineContext) {
		ctx.setSharedRuntimeData("revocationPlan", "plan")
	}).Return(FlowStep{Status: providers.FlowStatusComplete}, nil)

	result, svcErr := service.ExecuteSystemFlow(context.Background(), &common.SystemFlowContext{
		Handle: "system-application-delete-pre", Inputs: map[string]string{"targetApplicationId": "app-1"}})

	s.Nil(svcErr)
	s.NotEmpty(result.ExecutionID)
	// What the steps shared comes back, for the caller to hand to the flow that runs after the change.
	s.Equal(map[string]string{"revocationPlan": "plan"}, result.RuntimeData)
}

// A post flow starts with what its pre flow shared, so its steps finish what that run started.
func (s *ServiceTestSuite) TestExecuteSystemFlow_StartsWithTheRuntimeDataItIsGiven() {
	service, flows, engine := s.systemFlowService()
	flows.EXPECT().GetSystemFlow(mock.Anything, "system-revocation-post").Return(systemFlow(), nil)
	engine.EXPECT().Execute(mock.MatchedBy(func(ctx *EngineContext) bool {
		plan, ok := ctx.getSharedRuntimeData("revocationPlan")
		return ok && plan == "plan"
	})).Return(FlowStep{Status: providers.FlowStatusComplete}, nil)

	result, svcErr := service.ExecuteSystemFlow(context.Background(), &common.SystemFlowContext{
		Handle: "system-revocation-post", RuntimeData: map[string]string{"revocationPlan": "plan"}})

	s.Nil(svcErr)
	s.NotEmpty(result.ExecutionID)
	s.Equal(map[string]string{"revocationPlan": "plan"}, result.RuntimeData)
}

// A step's refusal is returned as it is, so the caller reports the real reason.
func (s *ServiceTestSuite) TestExecuteSystemFlow_ReturnsAStepsRefusal() {
	service, flows, engine := s.systemFlowService()
	flows.EXPECT().GetSystemFlow(mock.Anything, mock.Anything).Return(systemFlow(), nil)
	refusal := &tidcommon.ServiceError{Code: "APP-1030", Type: tidcommon.ClientErrorType}
	engine.EXPECT().Execute(mock.Anything).Return(FlowStep{Status: providers.FlowStatusError, Error: refusal}, nil)

	result, svcErr := service.ExecuteSystemFlow(context.Background(),
		&common.SystemFlowContext{Handle: "system-application-delete-pre"})
	s.Nil(result)
	s.Same(refusal, svcErr)
}

// A system flow that fails without a reason, stops for input, or cannot be found is a server error,
// and its context is never stored.
func (s *ServiceTestSuite) TestExecuteSystemFlow_AnythingButCompletionIsAServerError() {
	for name, step := range map[string]FlowStep{
		"failed without a reason": {Status: providers.FlowStatusError},
		"stopped for input":       {Status: providers.FlowStatusIncomplete},
	} {
		s.Run(name, func() {
			service, flows, engine := s.systemFlowService()
			flows.EXPECT().GetSystemFlow(mock.Anything, mock.Anything).Return(systemFlow(), nil)
			engine.EXPECT().Execute(mock.Anything).Return(step, nil)

			result, svcErr := service.ExecuteSystemFlow(context.Background(),
				&common.SystemFlowContext{Handle: "system-application-delete-pre"})
			s.Nil(result)
			s.Equal(&tidcommon.InternalServerError, svcErr)
		})
	}

	service, flows, engine := s.systemFlowService()
	flows.EXPECT().GetSystemFlow(mock.Anything, "unknown").Return(nil, &tidcommon.ServiceError{Code: "FMS-1003"})
	result, svcErr := service.ExecuteSystemFlow(context.Background(), &common.SystemFlowContext{Handle: "unknown"})
	s.Nil(result)
	s.Equal(&tidcommon.InternalServerError, svcErr)
	engine.AssertNotCalled(s.T(), "Execute", mock.Anything)
}

// The flow execution API cannot start a system flow: its type is not one a request may name.
func (s *ServiceTestSuite) TestExecutionAPIRefusesSystemFlows() {
	_, svcErr := validateFlowType(string(providers.FlowTypeSystem))

	s.Equal(&ErrorInvalidFlowType, svcErr)
}

const (
	boundaryAppID      = "boundary-app"
	boundaryAppOUID    = "boundary-app-ou"
	boundaryNamedOUID  = "boundary-named-ou"
	boundaryAuthFlowID = "boundary-auth-flow"
	boundaryRegFlowID  = "boundary-registration-flow"
	boundaryUserID     = "boundary-user"
)

// inStartingSignInBoundary matches a context carrying the sign-in boundary of boundaryAppID before an ouId or a
// subject is set.
func inStartingSignInBoundary() any {
	return mock.MatchedBy(func(c context.Context) bool {
		b, ok := authnprovidercm.SignInBoundaryFrom(c)
		return ok && b == authnprovidercm.SignInBoundary{ApplicationID: boundaryAppID}
	})
}

// boundaryFixture is a service that starts flows for boundaryAppID, an application owned by
// boundaryAppOUID, with the mocks a test states the sharing answers and outcomes on.
type boundaryFixture struct {
	service *flowExecService
	actor   *actorprovidermock.ActorProviderMock
	ou      *ouprovidermock.OrganizationUnitProviderMock
	engine  *flowEngineInterfaceMock
	crypto  *cryptomock.RuntimeCryptoProviderMock
	jwt     *jwtmock.JWTServiceInterfaceMock
	store   *flowStoreInterfaceMock
	graph   core.GraphInterface
}

// newBoundaryFixture returns a fixture whose application starts a flow of the given type. Its store
// has no expectations, so a test fails if any flow context is stored it did not allow.
func (s *ServiceTestSuite) newBoundaryFixture(flowType providers.FlowType) *boundaryFixture {
	config.ResetServerRuntime()
	s.T().Cleanup(config.ResetServerRuntime)
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), &config.Config{}))
	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))

	flowID := boundaryAuthFlowID
	if flowType == providers.FlowTypeRegistration {
		flowID = boundaryRegFlowID
	}
	actor := actorprovidermock.NewActorProviderMock(s.T())
	actor.EXPECT().GetInboundClientByID(mock.Anything, boundaryAppID).Return(&providers.InboundClient{
		ID: boundaryAppID, AuthFlowID: boundaryAuthFlowID,
		RegistrationFlowID: boundaryRegFlowID, IsRegistrationFlowEnabled: true,
	}, nil)
	actor.EXPECT().GetActor(mock.Anything, boundaryAppID).Return(&providers.Entity{
		ID: boundaryAppID, OUID: boundaryAppOUID, Category: providers.EntityCategoryApp,
	}, nil)
	flowProvider := NewFlowProviderMock(s.T())
	flowProvider.EXPECT().GetFlow(mock.Anything, flowID).
		Return(&providers.CompleteFlowDefinition{ID: flowID, FlowType: flowType}, nil)
	graph := flowFactory.CreateGraph(flowID, flowType, 1)
	graphBuilder := NewGraphBuilderInterfaceMock(s.T())
	graphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(graph, nil)
	observability := observabilitymock.NewObservabilityServiceInterfaceMock(s.T())
	observability.EXPECT().IsEnabled().Return(false).Maybe()

	f := &boundaryFixture{
		actor:  actor,
		ou:     ouprovidermock.NewOrganizationUnitProviderMock(s.T()),
		engine: newFlowEngineInterfaceMock(s.T()),
		crypto: cryptomock.NewRuntimeCryptoProviderMock(s.T()),
		jwt:    jwtmock.NewJWTServiceInterfaceMock(s.T()),
		store:  newFlowStoreInterfaceMock(s.T()),
		graph:  graph,
	}
	f.service = &flowExecService{
		flowProvider:     flowProvider,
		graphBuilder:     graphBuilder,
		flowStore:        f.store,
		flowEngine:       f.engine,
		actorProvider:    actor,
		ouProvider:       f.ou,
		observabilitySvc: observability,
		transactioner:    &stubTransactioner{},
		cryptoSvc:        f.crypto,
		jwtService:       f.jwt,
		cfg:              testFlowExecCfg,
	}
	return f
}

// initContext starts a flow of the fixture's type the way every entry point does.
func (f *boundaryFixture) initContext(ctx context.Context, flowType providers.FlowType) (
	*EngineContext, *tidcommon.ServiceError) {
	return f.service.initContext(ctx, "", boundaryAppID, flowType, nil, nil, true, log.GetLogger())
}

// The start fixes the organization unit when it can: the one the request names, or the application's
// own when it is not shared. A shared application with no unit named leaves it open.
func (s *ServiceTestSuite) TestBoundSignIn_FixesTheOrganizationUnitAtStart() {
	cases := []struct {
		name      string
		namedOUID string
		arrange   func(ou *ouprovidermock.OrganizationUnitProviderMock)
		wantOUID  string
	}{
		{
			name:      "a named unit the application is accessible from",
			namedOUID: boundaryNamedOUID,
			arrange: func(ou *ouprovidermock.OrganizationUnitProviderMock) {
				ou.EXPECT().IsEligibleOU(inStartingSignInBoundary(), boundaryNamedOUID).Return(true, nil).Once()
			},
			wantOUID: boundaryNamedOUID,
		},
		{
			name: "no unit named and the application not shared",
			arrange: func(ou *ouprovidermock.OrganizationUnitProviderMock) {
				ou.EXPECT().IsApplicationShared(inStartingSignInBoundary()).Return(false, nil).Once()
			},
			wantOUID: boundaryAppOUID,
		},
		{
			name: "no unit named and the application shared",
			arrange: func(ou *ouprovidermock.OrganizationUnitProviderMock) {
				ou.EXPECT().IsApplicationShared(inStartingSignInBoundary()).Return(true, nil).Once()
			},
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			f := s.newBoundaryFixture(providers.FlowTypeAuthentication)
			tc.arrange(f.ou)
			ctx := context.Background()
			if tc.namedOUID != "" {
				ctx = sysContext.WithAccessingOUID(ctx, tc.namedOUID)
			}

			engineCtx, svcErr := f.initContext(ctx, providers.FlowTypeAuthentication)

			s.Require().Nil(svcErr)
			s.True(engineCtx.SignInBounded)
			ouID, fixed := engineCtx.RuntimeData[common.RuntimeKeyOUID]
			s.Equal(tc.wantOUID != "", fixed)
			s.Equal(tc.wantOUID, ouID)
		})
	}
}

// A unit the application is not accessible from is refused at every entry point before any flow
// context exists, with one answer whether the unit is not shared with or does not exist.
func (s *ServiceTestSuite) TestBoundSignIn_RefusesAUnitTheApplicationIsNotAccessibleFrom() {
	answers := map[string]*tidcommon.ServiceError{
		"not shared with the unit": nil,
		"unit does not exist":      {Type: tidcommon.ClientErrorType, Code: "OU-1003"},
	}
	for name, lookupErr := range answers {
		refuse := func(f *boundaryFixture) {
			f.ou.EXPECT().IsEligibleOU(inStartingSignInBoundary(), boundaryNamedOUID).Return(false, lookupErr).Once()
		}
		ctx := sysContext.WithAccessingOUID(context.Background(), boundaryNamedOUID)

		s.Run(name+" via authorize", func() {
			f := s.newBoundaryFixture(providers.FlowTypeAuthentication)
			refuse(f)

			executionID, svcErr := f.service.InitiateFlow(ctx, &FlowInitContext{
				ApplicationID: boundaryAppID, FlowType: string(providers.FlowTypeAuthentication),
			})

			s.Empty(executionID)
			s.Equal(&ErrorInvalidOrganizationUnit, svcErr)
		})

		s.Run(name+" via flow execute", func() {
			f := s.newBoundaryFixture(providers.FlowTypeAuthentication)
			refuse(f)
			f.actor.EXPECT().GetOAuthProfileByID(mock.Anything, boundaryAppID).Return(nil, nil)
			f.actor.EXPECT().AuthenticateActor(mock.Anything, mock.Anything, mock.Anything).Return(nil)

			step, svcErr := f.service.Execute(ctx, boundaryAppID, "", string(providers.FlowTypeAuthentication),
				"", false, "", nil, "", "flow-secret", "", nil)

			s.Nil(step)
			s.Equal(&ErrorInvalidOrganizationUnit, svcErr)
		})
	}
}

// The unit a flow execution request names reaches the start rule, and its refusal is the request's answer.
func (s *ServiceTestSuite) TestBoundSignIn_FlowExecuteRequestNamingAnInaccessibleUnitIsRefused() {
	f := s.newBoundaryFixture(providers.FlowTypeAuthentication)
	f.ou.EXPECT().IsEligibleOU(inStartingSignInBoundary(), boundaryNamedOUID).Return(false, nil).Once()
	f.actor.EXPECT().GetOAuthProfileByID(mock.Anything, boundaryAppID).Return(nil, nil)
	f.actor.EXPECT().AuthenticateActor(mock.Anything, mock.Anything, mock.Anything).Return(nil)
	body := `{"applicationId":"` + boundaryAppID + `","flowType":"AUTHENTICATION","ouId":"` + boundaryNamedOUID + `"}`
	req := httptest.NewRequest(http.MethodPost, "/flow/execute", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(serverconst.FlowSecretHeaderName, "flow-secret")
	w := httptest.NewRecorder()

	newFlowExecutionHandler(f.service, nil, 0).HandleFlowExecutionRequest(w, req)

	s.Equal(http.StatusBadRequest, w.Code)
	s.Contains(w.Body.String(), ErrorInvalidOrganizationUnit.Code)
}

// A failed sharing lookup is a server error, not a refusal or an open boundary.
func (s *ServiceTestSuite) TestBoundSignIn_LookupFailureIsAServerError() {
	s.Run("access from the named unit", func() {
		f := s.newBoundaryFixture(providers.FlowTypeAuthentication)
		f.ou.EXPECT().IsEligibleOU(mock.Anything, boundaryNamedOUID).
			Return(false, &tidcommon.InternalServerError).Once()

		engineCtx, svcErr := f.initContext(
			sysContext.WithAccessingOUID(context.Background(), boundaryNamedOUID), providers.FlowTypeAuthentication)

		s.Nil(engineCtx)
		s.Equal(&tidcommon.InternalServerError, svcErr)
	})

	s.Run("whether the application is shared", func() {
		f := s.newBoundaryFixture(providers.FlowTypeAuthentication)
		f.ou.EXPECT().IsApplicationShared(mock.Anything).Return(false, &tidcommon.InternalServerError).Once()

		engineCtx, svcErr := f.initContext(context.Background(), providers.FlowTypeAuthentication)

		s.Nil(engineCtx)
		s.Equal(&tidcommon.InternalServerError, svcErr)
	})
}

// Only sign-in is bounded: another flow type asks nothing about sharing, even with a unit named.
func (s *ServiceTestSuite) TestBoundSignIn_OtherFlowTypesAreNotBounded() {
	f := s.newBoundaryFixture(providers.FlowTypeRegistration)

	engineCtx, svcErr := f.initContext(
		sysContext.WithAccessingOUID(context.Background(), boundaryNamedOUID), providers.FlowTypeRegistration)

	s.Require().Nil(svcErr)
	s.False(engineCtx.SignInBounded)
	s.NotContains(engineCtx.RuntimeData, common.RuntimeKeyOUID)
}

// The authorize entry point stores the boundary with the context, beside the runtime data it passed.
func (s *ServiceTestSuite) TestBoundSignIn_InitiateFlowStoresTheBoundary() {
	f := s.newBoundaryFixture(providers.FlowTypeAuthentication)
	f.ou.EXPECT().IsApplicationShared(inStartingSignInBoundary()).Return(false, nil).Once()
	var stored flowContextContent
	f.crypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(_ context.Context, _ *providers.KeyRef, _ string, _ map[string]interface{}, content []byte) {
			s.Require().NoError(json.Unmarshal(content, &stored))
		}).Return([]byte("encrypted-ctx"), nil, nil).Once()
	f.service.flowStore.(*flowStoreInterfaceMock).EXPECT().
		StoreFlowContext(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	executionID, svcErr := f.service.InitiateFlow(context.Background(), &FlowInitContext{
		ApplicationID: boundaryAppID,
		FlowType:      string(providers.FlowTypeAuthentication),
		RuntimeData:   map[string]string{common.RuntimeKeyClientID: "boundary-client"},
	})

	s.Require().Nil(svcErr)
	s.NotEmpty(executionID)
	s.True(stored.SignInBounded)
	s.Require().NotNil(stored.RuntimeData)
	var runtimeData map[string]string
	s.Require().NoError(json.Unmarshal([]byte(*stored.RuntimeData), &runtimeData))
	s.Equal(map[string]string{
		common.RuntimeKeyClientID: "boundary-client",
		common.RuntimeKeyOUID:     boundaryAppOUID,
	}, runtimeData)
}

// The flow execution entry point runs the flow with the boundary and the unit it named.
func (s *ServiceTestSuite) TestBoundSignIn_ExecuteRunsTheFlowBounded() {
	f := s.newBoundaryFixture(providers.FlowTypeAuthentication)
	f.ou.EXPECT().IsEligibleOU(inStartingSignInBoundary(), boundaryNamedOUID).Return(true, nil).Once()
	f.actor.EXPECT().GetOAuthProfileByID(mock.Anything, boundaryAppID).Return(nil, nil)
	f.actor.EXPECT().AuthenticateActor(mock.Anything, mock.Anything, mock.Anything).Return(nil)
	var ran *EngineContext
	f.engine.EXPECT().Execute(mock.Anything).Run(func(ctx *EngineContext) { ran = ctx }).
		Return(FlowStep{Status: providers.FlowStatusComplete}, nil).Once()

	_, svcErr := f.service.Execute(sysContext.WithAccessingOUID(context.Background(), boundaryNamedOUID),
		boundaryAppID, "", string(providers.FlowTypeAuthentication), "", false, "", nil, "", "flow-secret", "", nil)

	s.Require().Nil(svcErr)
	s.Require().NotNil(ran)
	s.True(ran.SignInBounded)
	s.Equal(boundaryNamedOUID, ran.RuntimeData[common.RuntimeKeyOUID])
}

// boundaryAuthUser is boundaryUserID as the default provider signed it in.
func boundaryAuthUser() providers.AuthUser {
	var authUser providers.AuthUser
	authUser.SetStateFor("default", providers.AuthState{
		EntityReference: &providers.EntityReference{EntityID: boundaryUserID, EntityCategory: "user"},
		AttributeToken:  "attributes",
	})
	return authUser
}

// completedSignIn is the step the engine completes a sign-in with: an assertion and a new SSO session.
func completedSignIn() FlowStep {
	return FlowStep{
		Status:       providers.FlowStatusComplete,
		Assertion:    "success-assertion",
		SSOHandleOut: "sso-handle",
		SSOFlowID:    boundaryAuthFlowID,
	}
}

// continueStoredFlow stores a flow the engine ends with step on the next request, and expects its context removed.
func (s *ServiceTestSuite) continueStoredFlow(f *boundaryFixture, flowType providers.FlowType, bounded bool,
	runtimeData map[string]string, step FlowStep) {
	stored := &FlowContextDB{}
	s.Require().NoError(stored.FromEngineContext(EngineContext{
		ExecutionID:       existingExecutionID,
		AppID:             boundaryAppID,
		FlowType:          flowType,
		SignInBounded:     bounded,
		AuthenticatedUser: authncm.AuthenticatedUser{Attributes: map[string]interface{}{}},
		AuthUser:          boundaryAuthUser(),
		UserInputs:        map[string]string{},
		RuntimeData:       runtimeData,
		ExecutionHistory:  map[string]*providers.NodeExecutionRecord{},
		Graph:             f.graph,
	}))
	f.store.EXPECT().GetFlowContext(mock.Anything, existingExecutionID).Return(stored, nil).Once()
	f.engine.EXPECT().Execute(mock.Anything).Return(step, nil).Once()
	f.store.EXPECT().DeleteFlowContext(mock.Anything, existingExecutionID).Return(nil).Once()
}

// execute continues the stored flow.
func (f *boundaryFixture) execute(flowType providers.FlowType) (*FlowStep, *tidcommon.ServiceError) {
	return f.service.Execute(context.Background(), boundaryAppID, existingExecutionID, string(flowType),
		"", false, "", nil, "", "", "", nil)
}

// The service returns a bounded completion as the engine made it, since the assertion step checks the boundary.
func (s *ServiceTestSuite) TestExecute_BoundedCompletionIsNotCheckedAgain() {
	f := s.newBoundaryFixture(providers.FlowTypeAuthentication)
	s.continueStoredFlow(f, providers.FlowTypeAuthentication, true,
		map[string]string{common.RuntimeKeyOUID: boundaryNamedOUID}, completedSignIn())

	step, svcErr := f.execute(providers.FlowTypeAuthentication)

	s.Require().Nil(svcErr)
	s.Require().NotNil(step)
	s.Equal(providers.FlowStatusComplete, step.Status)
	s.Nil(step.Error)
	s.Equal("success-assertion", step.Assertion)
	s.Require().Len(step.SessionData, 1)
	s.Equal("sso-handle", step.SessionData[0].SessionHandle)
}

func TestInitiateAndExecute_SetsTraceID(t *testing.T) {
	appID := "test-app-trace-id"

	testConfig := &config.Config{}
	_ = config.InitializeServerRuntime("/tmp/test-ia-trace", testConfig)
	defer config.ResetServerRuntime()

	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	testGraph := flowFactory.CreateGraph("auth-graph-trace", providers.FlowTypeAuthentication, 1)

	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(t)
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)
	mockFlowProvider := NewFlowProviderMock(t)
	mockGraphBuilder := NewGraphBuilderInterfaceMock(t)
	mockCrypto := cryptomock.NewRuntimeCryptoProviderMock(t)
	mockStore := newFlowStoreInterfaceMock(t)
	mockEngineInner := newFlowEngineInterfaceMock(t)

	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, appID).Return(
		&inboundmodel.InboundClient{ID: appID, AuthFlowID: "auth-graph-trace"}, nil)
	mockEntityProvider.EXPECT().GetEntityProfile(mock.Anything, appID).Return(
		&providers.Entity{ID: appID, Category: providers.EntityCategoryApp}, nil)
	mockFlowProvider.EXPECT().
		GetFlow(mock.Anything, "auth-graph-trace").
		Return(&providers.CompleteFlowDefinition{ID: "auth-graph-trace"}, nil)
	mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(testGraph, nil)
	mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]byte("encrypted"), nil, nil)

	mockStore.EXPECT().StoreFlowContext(mock.Anything, mock.Anything, mock.Anything).Return(nil)
	var capturedTraceID string
	mockEngineInner.EXPECT().Execute(mock.Anything).
		RunAndReturn(func(engineCtx *EngineContext) (FlowStep, *tidcommon.ServiceError) {
			capturedTraceID = engineCtx.TraceID
			return FlowStep{Status: providers.FlowStatusIncomplete}, nil
		})

	svc := &flowExecService{
		flowStore:     mockStore,
		graphBuilder:  mockGraphBuilder,
		flowProvider:  mockFlowProvider,
		flowEngine:    mockEngineInner,
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil, nil, nil),
		transactioner: &stubTransactioner{},
		cryptoSvc:     mockCrypto,
		cfg:           testFlowExecCfg,
	}

	step, err := svc.InitiateAndExecute(sysContext.WithTraceID(context.Background(), "trace-abcd"), &FlowInitContext{
		ApplicationID: appID,
		FlowType:      "AUTHENTICATION",
	})

	assert.Nil(t, err)
	assert.NotNil(t, step)
	assert.Equal(t, "trace-abcd", capturedTraceID)
}

const (
	testSelfFlowIssuer = "https://localhost:8090"
	testSelfFlowHandle = "default-self-attribute-verification-flow"
	testSelfFlowUser   = "user-1"
)

// selfIssuedTokenContext is the request context the security gate builds for an access token this
// deployment issued to the subject.
func (s *ServiceTestSuite) selfIssuedTokenContext(subject string) context.Context {
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), &config.Config{
		JWT: engineconfig.JWTConfig{Issuer: testSelfFlowIssuer},
	}))
	s.T().Cleanup(config.ResetServerRuntime)
	header, _ := json.Marshal(map[string]interface{}{"alg": "RS256", "typ": jwt.TokenTypeAccessToken})
	payload, _ := json.Marshal(map[string]interface{}{"sub": subject, "iss": testSelfFlowIssuer})
	token := base64.RawURLEncoding.EncodeToString(header) + "." +
		base64.RawURLEncoding.EncodeToString(payload) + ".signature"
	jwtService := jwtmock.NewJWTServiceInterfaceMock(s.T())
	jwtService.On("VerifyJWT", mock.Anything, token, "", "").Return(nil).Once()
	securityCtx, err := security.AuthenticateBearerToken(context.Background(), jwtService, token, "")
	s.Require().NoError(err)
	return security.WithSecurityContext(context.Background(), securityCtx)
}

// selfFlowService returns a service whose flow provider resolves the handle to a self flow.
func (s *ServiceTestSuite) selfFlowService(flowProvider *FlowProviderMock) (*flowExecService,
	*providers.CompleteFlowDefinition) {
	flowFactory, _ := core.Initialize(cache.Initialize(engineconfig.CacheConfig{}, "self-test"))
	flow := &providers.CompleteFlowDefinition{ID: "self-flow", Handle: testSelfFlowHandle,
		FlowType: providers.FlowTypeSelf, ActiveVersion: 1}
	builder := NewGraphBuilderInterfaceMock(s.T())
	builder.EXPECT().GetGraph(mock.Anything, flow).Return(flowFactory.CreateGraph(flow.ID, flow.FlowType, 1), nil).
		Maybe()
	return &flowExecService{flowProvider: flowProvider, graphBuilder: builder, cfg: testFlowExecCfg}, flow
}

// A self flow start binds the token's subject, resolves the flow by handle and ignores the
// application. A user's token on this public path arrives as a runtime context that keeps its subject.
func (s *ServiceTestSuite) TestLoadNewContext_SelfFlowBindsTheCaller() {
	for name, wrap := range map[string]func(context.Context) context.Context{
		"token":                func(ctx context.Context) context.Context { return ctx },
		"token on the runtime": security.WithRuntimeContext,
	} {
		s.Run(name, func() {
			flowProvider := NewFlowProviderMock(s.T())
			service, flow := s.selfFlowService(flowProvider)
			flowProvider.EXPECT().GetFlowByHandle(mock.Anything, testSelfFlowHandle,
				providers.FlowTypeSelf).Return(flow, nil).Once()

			engineCtx, svcErr := service.loadNewContext(wrap(s.selfIssuedTokenContext(testSelfFlowUser)), "",
				"some-app", string(providers.FlowTypeSelf), testSelfFlowHandle, false, "",
				map[string]string{"attribute": "email"}, "", "", log.GetLogger())

			s.Require().Nil(svcErr)
			s.Equal(testSelfFlowUser, engineCtx.boundSubject)
			s.Equal(flow.ID, engineCtx.Graph.GetID())
			s.Equal(providers.FlowTypeSelf, engineCtx.FlowType)
			s.Empty(engineCtx.AppID)
			s.Empty(engineCtx.Application.ID)
			s.Equal("email", engineCtx.UserInputs["attribute"])
		})
	}
}

// The caller is checked before the handle, so an unauthenticated caller cannot learn which handles exist.
func (s *ServiceTestSuite) TestLoadNewContext_SelfFlowRequiresASelfIssuedToken() {
	for name, ctx := range map[string]func() context.Context{
		"no token":            context.Background,
		"runtime without one": func() context.Context { return security.WithRuntimeContext(context.Background()) },
		"token without a subject": func() context.Context {
			return s.selfIssuedTokenContext("")
		},
		"caller without a self-issued token": func() context.Context {
			return authenticatedAdminContext([]string{"system"})
		},
	} {
		s.Run(name, func() {
			flowProvider := NewFlowProviderMock(s.T())
			service, _ := s.selfFlowService(flowProvider)

			engineCtx, svcErr := service.loadNewContext(ctx(), "", "", string(providers.FlowTypeSelf),
				"", false, "", map[string]string{"attribute": "email"}, "", "", log.GetLogger())

			s.Nil(engineCtx)
			s.Require().NotNil(svcErr)
			s.Equal(ErrorFlowAuthenticationRequired.Code, svcErr.Code)
			s.Equal("error.flowexecservice.flow_access_token_required_description",
				svcErr.ErrorDescription.Key)
			flowProvider.AssertNotCalled(s.T(), "GetFlowByHandle", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func (s *ServiceTestSuite) TestLoadNewContext_SelfFlowRequiresAHandle() {
	flowProvider := NewFlowProviderMock(s.T())
	service, _ := s.selfFlowService(flowProvider)

	service.serverConfigSvc = stubServerConfig{cfg: flowconfig.FlowSectionConfig{
		SelfFlow: flowconfig.FlowTypeConfig{DefaultHandle: testSelfFlowHandle},
	}}

	engineCtx, svcErr := service.loadNewContext(s.selfIssuedTokenContext(testSelfFlowUser), "", "",
		string(providers.FlowTypeSelf), "", false, "", map[string]string{"attribute": "email"}, "", "",
		log.GetLogger())

	s.Nil(engineCtx)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorFlowHandleRequired.Code, svcErr.Code)
	flowProvider.AssertNotCalled(s.T(), "GetFlowByHandle", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestLoadNewContext_SelfFlowUnknownHandle() {
	notFound := &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "FLM-1003"}
	flowProvider := NewFlowProviderMock(s.T())
	service, _ := s.selfFlowService(flowProvider)
	flowProvider.EXPECT().GetFlowByHandle(mock.Anything, "unknown", providers.FlowTypeSelf).
		Return(nil, notFound).Once()

	engineCtx, svcErr := service.loadNewContext(s.selfIssuedTokenContext(testSelfFlowUser), "", "",
		string(providers.FlowTypeSelf), "unknown", false, "", map[string]string{}, "", "", log.GetLogger())

	s.Nil(engineCtx)
	s.Equal(&ErrorFlowHandleNotFound, svcErr)
}

// A provider's server error is not reported as an unknown handle.
func (s *ServiceTestSuite) TestLoadNewContext_SelfFlowHandleLookupServerError() {
	flowProvider := NewFlowProviderMock(s.T())
	service, _ := s.selfFlowService(flowProvider)
	flowProvider.EXPECT().GetFlowByHandle(mock.Anything, testSelfFlowHandle, providers.FlowTypeSelf).
		Return(nil, &tidcommon.InternalServerError).Once()

	engineCtx, svcErr := service.loadNewContext(s.selfIssuedTokenContext(testSelfFlowUser), "", "",
		string(providers.FlowTypeSelf), testSelfFlowHandle, false, "", map[string]string{}, "", "",
		log.GetLogger())

	s.Nil(engineCtx)
	s.Equal(&tidcommon.InternalServerError, svcErr)
}

// Another flow type keeps its own selection and ignores a handle.
func (s *ServiceTestSuite) TestLoadNewContext_OtherFlowTypesIgnoreTheHandle() {
	flowFactory, _ := core.Initialize(cache.Initialize(engineconfig.CacheConfig{}, "handle-ignored-test"))
	flow := &providers.CompleteFlowDefinition{ID: "onboarding-flow", FlowType: providers.FlowTypeUserOnboarding}
	flowProvider := NewFlowProviderMock(s.T())
	flowProvider.EXPECT().GetFlowByHandle(mock.Anything, testUserOnboardingFlowHandle,
		providers.FlowTypeUserOnboarding).Return(flow, nil).Once()
	flowProvider.EXPECT().GetFlow(mock.Anything, flow.ID).Return(flow, nil).Once()
	builder := NewGraphBuilderInterfaceMock(s.T())
	builder.EXPECT().GetGraph(mock.Anything, flow).Return(flowFactory.CreateGraph(flow.ID, flow.FlowType, 1), nil)
	service := &flowExecService{flowProvider: flowProvider, graphBuilder: builder, cfg: testFlowExecCfg,
		serverConfigSvc: stubServerConfig{cfg: flowconfig.FlowSectionConfig{
			UserOnboardingFlow: flowconfig.FlowTypeConfig{DefaultHandle: testUserOnboardingFlowHandle},
		}}}

	security.InitControlPlanePermissions("")
	engineCtx, svcErr := service.loadNewContext(authenticatedAdminContext([]string{"system:user:create"}), "", "",
		string(providers.FlowTypeUserOnboarding), "other-handle", false, "", map[string]string{}, "", "",
		log.GetLogger())

	s.Require().Nil(svcErr)
	s.Equal(flow.ID, engineCtx.Graph.GetID())
	s.Empty(engineCtx.boundSubject)
}

// A continuation keeps the subject bound at the start, whatever token a later request carries, and
// needs none.
func (s *ServiceTestSuite) TestExecute_SelfFlowContinuationKeepsTheBoundSubject() {
	for name, ctx := range map[string]func() context.Context{
		"no token":           context.Background,
		"another user token": func() context.Context { return s.selfIssuedTokenContext("user-2") },
	} {
		s.Run(name, func() {
			flowFactory, _ := core.Initialize(cache.Initialize(engineconfig.CacheConfig{}, "self-continue-test"))
			flow := &providers.CompleteFlowDefinition{ID: "self-flow", FlowType: providers.FlowTypeSelf}
			graph := flowFactory.CreateGraph(flow.ID, flow.FlowType, 1)
			stored := &FlowContextDB{}
			s.Require().NoError(stored.FromEngineContext(EngineContext{
				ExecutionID: existingExecutionID, FlowType: flow.FlowType, Graph: graph,
				UserInputs: map[string]string{}, RuntimeData: map[string]string{},
				boundSubject: testSelfFlowUser,
			}))
			store := newFlowStoreInterfaceMock(s.T())
			store.EXPECT().GetFlowContext(mock.Anything, existingExecutionID).Return(stored, nil)
			store.EXPECT().DeleteFlowContext(mock.Anything, existingExecutionID).Return(nil)
			flowProvider := NewFlowProviderMock(s.T())
			flowProvider.EXPECT().GetFlow(mock.Anything, flow.ID).Return(flow, nil)
			builder := NewGraphBuilderInterfaceMock(s.T())
			builder.EXPECT().GetGraph(mock.Anything, flow).Return(graph, nil)
			engine := newFlowEngineInterfaceMock(s.T())
			engine.EXPECT().Execute(mock.MatchedBy(func(engineCtx *EngineContext) bool {
				return engineCtx.boundSubject == testSelfFlowUser
			})).Return(FlowStep{Status: providers.FlowStatusComplete}, nil).Once()
			service := &flowExecService{flowStore: store, flowProvider: flowProvider, graphBuilder: builder,
				flowEngine: engine, transactioner: &stubTransactioner{}}

			step, svcErr := service.Execute(ctx(), "", existingExecutionID, "", "", false, "verify",
				map[string]string{"otp": "123456"}, "", "", "", nil)

			s.Require().Nil(svcErr)
			s.Equal(providers.FlowStatusComplete, step.Status)
		})
	}
}

// A new self flow execution that pauses is stored with the type's configured expiry.
func (s *ServiceTestSuite) TestExecute_StoresANewSelfFlowExecutionWithItsExpiry() {
	flowProvider := NewFlowProviderMock(s.T())
	service, flow := s.selfFlowService(flowProvider)
	flowProvider.EXPECT().GetFlowByHandle(mock.Anything, testSelfFlowHandle, providers.FlowTypeSelf).
		Return(flow, nil).Once()
	engine := newFlowEngineInterfaceMock(s.T())
	engine.EXPECT().Execute(mock.Anything).Return(FlowStep{Status: providers.FlowStatusIncomplete}, nil).Once()
	crypto := cryptomock.NewRuntimeCryptoProviderMock(s.T())
	crypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]byte("encrypted-ctx"), nil, nil)
	store := newFlowStoreInterfaceMock(s.T())
	store.EXPECT().StoreFlowContext(mock.Anything, mock.Anything, int64(900)).Return(nil).Once()
	service.flowEngine, service.cryptoSvc, service.flowStore = engine, crypto, store
	service.transactioner = &stubTransactioner{}
	service.serverConfigSvc = stubServerConfig{cfg: flowconfig.FlowSectionConfig{
		SelfFlow: flowconfig.FlowTypeConfig{ExpirySeconds: 900},
	}}

	step, svcErr := service.Execute(s.selfIssuedTokenContext(testSelfFlowUser), "", "",
		string(providers.FlowTypeSelf), testSelfFlowHandle, false, "",
		map[string]string{"attribute": "email"}, "", "", "", nil)

	s.Require().Nil(svcErr)
	s.Equal(providers.FlowStatusIncomplete, step.Status)
}

func (s *ServiceTestSuite) TestGetFlowExpirySeconds_SelfFlow() {
	service := &flowExecService{}
	s.Equal(defaultSelfFlowExpiry, service.getFlowExpirySeconds(context.Background(),
		providers.FlowTypeSelf))

	service.serverConfigSvc = stubServerConfig{cfg: flowconfig.FlowSectionConfig{
		SelfFlow: flowconfig.FlowTypeConfig{ExpirySeconds: 600},
	}}
	s.Equal(int64(600), service.getFlowExpirySeconds(context.Background(), providers.FlowTypeSelf))
}

// A self flow is started only on the execute path, so the initiation entry points refuse its
// type instead of starting the application's sign-in flow.
func (s *ServiceTestSuite) TestInitiationEntryPointsRefuseSelfFlow() {
	service := &flowExecService{cfg: testFlowExecCfg}
	initContext := &FlowInitContext{ApplicationID: "test-app",
		FlowType: string(providers.FlowTypeSelf), RuntimeData: map[string]string{}}

	executionID, err := service.InitiateFlow(context.Background(), initContext)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowType.Code, err.Code)
	s.Empty(executionID)

	step, err := service.InitiateAndExecute(context.Background(), initContext)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowType.Code, err.Code)
	s.Nil(step)
}

func (s *ServiceTestSuite) TestSelfFlowHasNoDefaultFlowHandle() {
	service := &flowExecService{serverConfigSvc: stubServerConfig{cfg: flowconfig.FlowSectionConfig{
		SelfFlow: flowconfig.FlowTypeConfig{DefaultHandle: testSelfFlowHandle},
	}}}
	s.Empty(service.resolveDefaultFlowHandle(context.Background(), providers.FlowTypeSelf))
}

func (s *ServiceTestSuite) TestGetExecutionPosition() {
	s.Run("returns the flow a paused execution runs and its node", func() {
		mockStore := newFlowStoreInterfaceMock(s.T())
		mockStore.On("GetFlowContext", mock.Anything, "exec-1").Return(&FlowContextDB{ExecutionID: "exec-1",
			Context: `{"graphId":"flow-1","currentNodeId":"prompt_owner"}`}, nil).Once()
		svc := &flowExecService{flowStore: mockStore}

		flowID, nodeID, svcErr := svc.GetExecutionPosition(context.Background(), "exec-1")

		s.Nil(svcErr)
		s.Equal("flow-1", flowID)
		s.Equal("prompt_owner", nodeID)
	})

	s.Run("refuses an execution that isn't stored", func() {
		mockStore := newFlowStoreInterfaceMock(s.T())
		mockStore.On("GetFlowContext", mock.Anything, "gone").Return((*FlowContextDB)(nil), nil).Once()
		svc := &flowExecService{flowStore: mockStore}

		_, _, svcErr := svc.GetExecutionPosition(context.Background(), "gone")

		s.Equal(&ErrorInvalidExecutionID, svcErr)
	})

	s.Run("reports a context it can't read", func() {
		mockStore := newFlowStoreInterfaceMock(s.T())
		mockStore.On("GetFlowContext", mock.Anything, "bad").
			Return(&FlowContextDB{ExecutionID: "bad", Context: "{"}, nil).Once()
		svc := &flowExecService{flowStore: mockStore}

		_, _, svcErr := svc.GetExecutionPosition(context.Background(), "bad")

		s.Equal(&tidcommon.InternalServerError, svcErr)
	})
}

// --- flow authority ---

func (s *ServiceTestSuite) TestStartAuthority_RunsEachFlowTypeWithItsAuthority() {
	security.InitControlPlanePermissions("")
	caller := authenticatedAdminContext([]string{"system:user:create"})

	for _, flowType := range []providers.FlowType{
		providers.FlowTypeAuthentication, providers.FlowTypeRegistration, providers.FlowTypeRecovery,
		providers.FlowTypeSignOut, providers.FlowTypeDeviceGrant, providers.FlowTypeSelf, providers.FlowTypeSystem,
	} {
		s.Run(string(flowType), func() {
			ctx, svcErr := (&flowExecService{}).startAuthority(caller, flowType)

			s.Nil(svcErr)
			s.True(security.IsRuntimeContext(ctx))
			s.Equal("admin-1", security.GetSubject(ctx), "the subject stays for audit")
		})
	}
	for _, flowType := range []providers.FlowType{
		providers.FlowTypeAdministration, providers.FlowTypeUserOnboarding, providers.FlowTypeInvitation,
	} {
		s.Run(string(flowType), func() {
			ctx, svcErr := (&flowExecService{}).startAuthority(caller, flowType)

			s.Nil(svcErr)
			s.False(security.IsRuntimeContext(ctx))
			s.Equal([]string{"system:user:create"}, security.GetPermissions(ctx))
		})
	}
}

func (s *ServiceTestSuite) TestStartAuthority_FloorForAFlowThatActsForItsCaller() {
	security.InitControlPlanePermissions("")
	tests := []struct {
		name    string
		ctx     context.Context
		errCode string
	}{
		{"NoToken", security.WithRuntimeContext(context.Background()), ErrorFlowAuthenticationRequired.Code},
		{"NoSubject", context.Background(), ErrorFlowAuthenticationRequired.Code},
		{"TokenWithoutASystemPermission", authenticatedAdminContext([]string{"openid"}),
			ErrorAdministrationPermissionRequired.Code},
		{"NarrowPermission", authenticatedAdminContext([]string{"system:user:view"}), ""},
		{"RootPermission", authenticatedAdminContext([]string{"system"}), ""},
	}
	for _, tt := range tests {
		for _, flowType := range []providers.FlowType{
			providers.FlowTypeAdministration, providers.FlowTypeUserOnboarding,
		} {
			s.Run(tt.name+"/"+string(flowType), func() {
				_, svcErr := (&flowExecService{}).startAuthority(tt.ctx, flowType)

				if tt.errCode == "" {
					s.Nil(svcErr)
					return
				}
				s.Require().NotNil(svcErr)
				s.Equal(tt.errCode, svcErr.Code)
			})
		}
	}
}

func (s *ServiceTestSuite) TestLoadNewContext_ByTypeRefusesUserOnboardingWithoutAToken() {
	security.InitControlPlanePermissions("")
	flowProvider := NewFlowProviderMock(s.T())
	service := &flowExecService{flowProvider: flowProvider, cfg: testFlowExecCfg}

	engineCtx, svcErr := service.loadNewContext(security.WithRuntimeContext(context.Background()), "", "",
		string(providers.FlowTypeUserOnboarding), "", false, "", nil, "", "", log.GetLogger())

	s.Nil(engineCtx)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorFlowAuthenticationRequired.Code, svcErr.Code)
	flowProvider.AssertNotCalled(s.T(), "GetFlowByHandle", mock.Anything, mock.Anything, mock.Anything)
}

// A narrow administrator starts an administration flow by ID, and the flow records their authority.
func (s *ServiceTestSuite) TestLoadNewContext_ByIDRecordsANarrowAdminsAuthority() {
	security.InitControlPlanePermissions("")
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), &config.Config{}))
	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	flow := &providers.CompleteFlowDefinition{
		ID: "administration-1", FlowType: providers.FlowTypeAdministration, ActiveVersion: 1,
	}
	flowProvider := NewFlowProviderMock(s.T())
	flowProvider.EXPECT().GetFlow(mock.Anything, "administration-1").Return(flow, nil).Once()
	graphBuilder := NewGraphBuilderInterfaceMock(s.T())
	graphBuilder.EXPECT().GetGraph(mock.Anything, flow).Return(
		flowFactory.CreateGraph("administration-1", providers.FlowTypeAdministration, 1), nil)
	service := &flowExecService{flowProvider: flowProvider, graphBuilder: graphBuilder, cfg: testFlowExecCfg,
		flowVisibility: s.administrationFlowVisible()}

	engineCtx, svcErr := service.loadNewContext(authenticatedAdminContext([]string{"system:user:delete"}),
		"administration-1", "", "", "", false, "", nil, "", "", log.GetLogger())

	s.Require().Nil(svcErr)
	s.False(security.IsRuntimeContext(engineCtx.Context))
	s.Equal(&flowAuthority{Subject: "admin-1", OUID: "ou-1", Permissions: []string{"system:user:delete"}},
		engineCtx.authority)
}

func (s *ServiceTestSuite) TestAuthority_PersistedWithTheContext() {
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), &config.Config{}))
	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	graph := flowFactory.CreateGraph("onboarding-1", providers.FlowTypeUserOnboarding, 1)
	authority := &flowAuthority{Subject: "admin-1", OUID: "ou-1", Permissions: []string{"system:user:create"}}

	stored := &FlowContextDB{}
	s.Require().NoError(stored.FromEngineContext(EngineContext{
		ExecutionID: "execution-1", FlowType: providers.FlowTypeUserOnboarding, Graph: graph, authority: authority,
	}))
	loaded, err := stored.ToEngineContext(context.Background(), graph, nil)

	s.Require().NoError(err)
	s.Equal(authority, loaded.authority)
}

// continuationService returns a service whose permission provider must not be asked.
func (s *ServiceTestSuite) continuationService() *flowExecService {
	return &flowExecService{permissionProvider: permissionprovidermock.NewPermissionProviderMock(s.T()),
		cfg: testFlowExecCfg}
}

// resolvingService returns a service whose permission provider is asked once what admin-1 holds now.
func (s *ServiceTestSuite) resolvingService(current security.PermissionSet) *flowExecService {
	provider := permissionprovidermock.NewPermissionProviderMock(s.T())
	provider.EXPECT().GetEntityPermissions(mock.Anything, "admin-1").Return(current, nil).Once()
	return &flowExecService{permissionProvider: provider, cfg: testFlowExecCfg}
}

// startedContext returns a flow of flowType that admin-1 of ou-1 started with permissions.
func startedContext(flowType providers.FlowType, permissions ...string) *EngineContext {
	return &EngineContext{ExecutionID: "execution-1", FlowType: flowType,
		authority: &flowAuthority{Subject: "admin-1", OUID: "ou-1", Permissions: permissions}}
}

func (s *ServiceTestSuite) TestContinuationAuthority_RunsAFlowForWhoeverDrivesItWithTheServersAuthority() {
	service := s.continuationService()

	for _, flowType := range []providers.FlowType{
		providers.FlowTypeAuthentication, providers.FlowTypeRecovery, providers.FlowTypeSelf,
	} {
		ctx, svcErr := service.continuationAuthority(authenticatedAdminContext(nil),
			&EngineContext{FlowType: flowType}, log.GetLogger())

		s.Nil(svcErr)
		s.True(security.IsRuntimeContext(ctx))
		s.Equal("admin-1", security.GetSubject(ctx))
	}
}

func (s *ServiceTestSuite) TestContinuationAuthority_WithATokenRunsAsTheCaller() {
	security.InitControlPlanePermissions("")
	service := s.continuationService()
	caller := authenticatedAdminContext([]string{"system:user:delete"})

	ctx, svcErr := service.continuationAuthority(caller,
		startedContext(providers.FlowTypeAdministration, "system:user:delete"), log.GetLogger())

	s.Nil(svcErr)
	s.Equal(caller, ctx)
}

func (s *ServiceTestSuite) TestContinuationAuthority_RefusesAnotherCaller() {
	security.InitControlPlanePermissions("")
	service := s.continuationService()
	other := security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest("admin-2", "ou-1", "token", []string{"system"}, nil))

	for _, flowType := range []providers.FlowType{
		providers.FlowTypeAdministration, providers.FlowTypeUserOnboarding,
	} {
		_, svcErr := service.continuationAuthority(other, startedContext(flowType, "system"), log.GetLogger())

		s.Require().NotNil(svcErr)
		s.Equal(ErrorFlowStartedByAnotherCaller.Code, svcErr.Code)
	}
}

// The token of the caller who started the flow is checked on every step, so one that no longer carries a
// system permission is refused.
func (s *ServiceTestSuite) TestContinuationAuthority_ReappliesTheFloorToTheCallersToken() {
	security.InitControlPlanePermissions("")
	service := s.continuationService()

	_, svcErr := service.continuationAuthority(authenticatedAdminContext([]string{"openid"}),
		startedContext(providers.FlowTypeAdministration, "system"), log.GetLogger())

	s.Require().NotNil(svcErr)
	s.Equal(ErrorAdministrationPermissionRequired.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestContinuationAuthority_RefusesATokenlessAdministrationStep() {
	security.InitControlPlanePermissions("")
	service := s.continuationService()

	for name, ctx := range map[string]context.Context{
		"runtime": security.WithRuntimeContext(context.Background()), "bare": context.Background(),
	} {
		s.Run(name, func() {
			_, svcErr := service.continuationAuthority(ctx,
				startedContext(providers.FlowTypeAdministration, "system"), log.GetLogger())

			s.Require().NotNil(svcErr)
			s.Equal(ErrorFlowAuthenticationRequired.Code, svcErr.Code)
		})
	}
}

// A flow that called another flow is decided by the flow the execution started, not the callee it paused in.
func (s *ServiceTestSuite) TestContinuationAuthority_DecidesByTheRootFlow() {
	security.InitControlPlanePermissions("")
	service := s.continuationService()
	engineCtx := startedContext(providers.FlowTypeAuthentication, "system")
	engineCtx.frameStack = []*frame{{flowType: providers.FlowTypeAdministration}}

	_, svcErr := service.continuationAuthority(security.WithRuntimeContext(context.Background()), engineCtx,
		log.GetLogger())

	s.Require().NotNil(svcErr)
	s.Equal(ErrorFlowAuthenticationRequired.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestContinuationAuthority_TokenlessStepRunsWithTheRecordedAuthority() {
	security.InitControlPlanePermissions("")
	service := s.resolvingService(security.PermissionSet{"rs-1": {"system:user:create"}})

	ctx, svcErr := service.continuationAuthority(security.WithRuntimeContext(context.Background()),
		startedContext(providers.FlowTypeUserOnboarding, "openid", "system:user:create", "system:group:view"),
		log.GetLogger())

	s.Require().Nil(svcErr)
	s.False(security.IsRuntimeContext(ctx))
	s.True(security.IsRecordedCaller(ctx))
	s.Equal("admin-1", security.GetSubject(ctx))
	s.Equal("ou-1", security.GetOUID(ctx))
	s.Equal([]string{"system:user:create"}, security.GetPermissions(ctx), "only what admin-1 still holds")
}

// An invitation flow runs each step with the request's own context: the admin's token on the admin's steps, and
// the server's authority on the invitee's. It records no authority, so none is looked up.
func (s *ServiceTestSuite) TestContinuationAuthority_InvitationRunsWithTheRequestsContext() {
	security.InitControlPlanePermissions("")
	service := s.continuationService()

	for name, ctx := range map[string]context.Context{
		"admin":   authenticatedAdminContext([]string{"system"}),
		"invitee": security.WithRuntimeContext(context.Background()),
	} {
		s.Run(name, func() {
			got, svcErr := service.continuationAuthority(ctx, &EngineContext{FlowType: providers.FlowTypeInvitation},
				log.GetLogger())

			s.Require().Nil(svcErr)
			s.Equal(ctx, got)
		})
	}
}

func (s *ServiceTestSuite) TestContinuationAuthority_RefusesATokenlessStepOnceTheCallerHoldsNoSystemPermission() {
	security.InitControlPlanePermissions("")
	service := s.resolvingService(security.PermissionSet{"rs-1": {"orders:read"}})

	_, svcErr := service.continuationAuthority(security.WithRuntimeContext(context.Background()),
		startedContext(providers.FlowTypeUserOnboarding, "system:user:create", "orders:read"), log.GetLogger())

	s.Require().NotNil(svcErr)
	s.Equal(ErrorAdministrationPermissionRequired.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestContinuationAuthority_RefusesAFlowWithNoRecordedAuthority() {
	service := s.continuationService()

	_, svcErr := service.continuationAuthority(security.WithRuntimeContext(context.Background()),
		&EngineContext{FlowType: providers.FlowTypeUserOnboarding}, log.GetLogger())

	s.Require().NotNil(svcErr)
	s.Equal(ErrorFlowRestartRequired.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestContinuationAuthority_ResolverFailureIsAServerError() {
	provider := permissionprovidermock.NewPermissionProviderMock(s.T())
	provider.EXPECT().GetEntityPermissions(mock.Anything, "admin-1").Return(nil, errors.New("store down"))
	service := &flowExecService{permissionProvider: provider, cfg: testFlowExecCfg}

	_, svcErr := service.continuationAuthority(security.WithRuntimeContext(context.Background()),
		startedContext(providers.FlowTypeUserOnboarding, "system"), log.GetLogger())

	s.Equal(&tidcommon.InternalServerError, svcErr)
}

func (s *ServiceTestSuite) TestContinuationAuthority_WithoutAResolverRefusesATokenlessStep() {
	service := &flowExecService{cfg: testFlowExecCfg}

	_, svcErr := service.continuationAuthority(security.WithRuntimeContext(context.Background()),
		startedContext(providers.FlowTypeUserOnboarding, "system"), log.GetLogger())

	s.Require().NotNil(svcErr)
	s.Equal(ErrorFlowAuthenticationRequired.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestExecuteSystemFlow_RunsWithTheServersAuthorityKeepingTheSubject() {
	service, flows, engine := s.systemFlowService()
	flows.EXPECT().GetSystemFlow(mock.Anything, mock.Anything).Return(systemFlow(), nil)
	engine.EXPECT().Execute(mock.MatchedBy(func(ctx *EngineContext) bool {
		return security.IsRuntimeContext(ctx.Context) && security.GetSubject(ctx.Context) == "admin-1"
	})).Return(FlowStep{Status: providers.FlowStatusComplete}, nil)

	_, svcErr := service.ExecuteSystemFlow(authenticatedAdminContext([]string{"system:app:delete"}),
		&common.SystemFlowContext{Handle: "system-application-delete-pre"})

	s.Nil(svcErr)
}

// heldService returns a resolving service whose governance answers decision, or svcErr, for admin-1.
func (s *ServiceTestSuite) heldService(decision providers.AccessDecision,
	svcErr *tidcommon.ServiceError) *flowExecService {
	service := s.resolvingService(security.PermissionSet{"rs-1": {"system:user:create"}})
	governance := identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(s.T())
	governance.EXPECT().EvaluateAccess(mock.Anything, "admin-1", providers.AccessPlaneSession,
		providers.AuthenticationMethod("")).Return(decision, svcErr).Once()
	service.governance = governance
	return service
}

func (s *ServiceTestSuite) TestContinuationAuthority_TokenlessStepRunsForAnAdmittedCaller() {
	security.InitControlPlanePermissions("")
	service := s.heldService(providers.AccessDecision{Admitted: true}, nil)

	ctx, svcErr := service.continuationAuthority(security.WithRuntimeContext(context.Background()),
		startedContext(providers.FlowTypeUserOnboarding, "system:user:create"), log.GetLogger())

	s.Require().Nil(svcErr)
	s.True(security.IsRecordedCaller(ctx))
}

// A caller suspended after starting the flow no longer lends it their authority, as their tokens no longer would.
func (s *ServiceTestSuite) TestContinuationAuthority_RefusesATokenlessStepOnceTheCallerIsHeld() {
	security.InitControlPlanePermissions("")
	service := s.heldService(providers.AccessDecision{Admitted: false}, nil)

	_, svcErr := service.continuationAuthority(security.WithRuntimeContext(context.Background()),
		startedContext(providers.FlowTypeUserOnboarding, "system:user:create"), log.GetLogger())

	s.Require().NotNil(svcErr)
	s.Equal(ErrorAdministrationPermissionRequired.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestContinuationAuthority_UnreadableHoldIsAServerError() {
	security.InitControlPlanePermissions("")
	service := s.heldService(providers.AccessDecision{}, &tidcommon.InternalServerError)

	_, svcErr := service.continuationAuthority(security.WithRuntimeContext(context.Background()),
		startedContext(providers.FlowTypeUserOnboarding, "system:user:create"), log.GetLogger())

	s.Equal(&tidcommon.InternalServerError, svcErr)
}
