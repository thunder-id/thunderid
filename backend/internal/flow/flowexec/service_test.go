// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowexec

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	flowconfig "github.com/thunder-id/thunderid/internal/flow/config"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/interceptor"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
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
	"github.com/thunder-id/thunderid/tests/mocks/inboundclientmock"
	"github.com/thunder-id/thunderid/tests/mocks/observability/observabilitymock"
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
				graphBuilder:  mockGraphBuilder,
				flowProvider:  mockFlowProvider,
				flowStore:     mockStore,
				actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil),
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
				initContext.FlowType = string(providers.FlowTypeUserOnboarding)
				initContext.ApplicationID = "" // System flows don't need app ID

				// Mock flow management service to return flow by handle
				mockFlow := &providers.CompleteFlowDefinition{
					ID:       "onboarding-flow-123",
					FlowType: providers.FlowTypeUserOnboarding,
				}
				mockFlowProvider.EXPECT().GetFlowByHandle(mock.Anything,
					testUserOnboardingFlowHandle, providers.FlowTypeUserOnboarding).Return(mockFlow, nil)

				// Mock GetGraph call which is made during initContext
				inviteGraph := flowFactory.CreateGraph("onboarding-flow-123", providers.FlowTypeUserOnboarding, 1)
				mockFlowProvider.EXPECT().
					GetFlow(mock.Anything, "onboarding-flow-123").
					Return(&providers.CompleteFlowDefinition{
						ID:       "onboarding-flow-123",
						FlowType: providers.FlowTypeUserOnboarding,
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
				mockEntityProvider.EXPECT().GetEntity(mock.Anything, appID).
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
			executionID, svcErr := service.InitiateFlow(context.Background(), initContext)

			// Assert
			assert.NotEmpty(t, executionID)
			assert.Nil(t, svcErr)

			// All mocks automatically verified by mockery
		})
	}
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
				mockEntityProvider.EXPECT().GetEntity(mock.Anything, appID).Return(
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
				graphBuilder:  mockGraphBuilder,
				flowProvider:  mockFlowProvider,
				flowStore:     mockStore,
				actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil),
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

		service := &flowExecService{
			graphBuilder:  mockGraphBuilder,
			flowProvider:  mockFlowProvider,
			flowStore:     mockStore,
			actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil),
			transactioner: &stubTransactioner{},
			cryptoSvc:     mockCrypto,
			cfg:           testFlowExecCfg,
			serverConfigSvc: stubServerConfig{cfg: flowconfig.FlowSectionConfig{
				AuthFlow: flowconfig.FlowTypeConfig{DefaultHandle: testDefaultAuthFlowHandle},
			}},
		}

		mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, appID).
			Return(&inboundmodel.InboundClient{ID: appID, AuthFlowID: "stale-auth-graph"}, nil)
		mockEntityProvider.EXPECT().GetEntity(mock.Anything, appID).
			Return(&providers.Entity{ID: appID, Category: providers.EntityCategoryApp},
				(*entityprovider.EntityProviderError)(nil))

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
			graphBuilder:  mockGraphBuilder,
			flowProvider:  mockFlowProvider,
			flowStore:     mockStore,
			actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil),
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
			graphBuilder:  mockGraphBuilder,
			flowProvider:  mockFlowProvider,
			flowStore:     mockStore,
			actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil),
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
	mockEntityProvider.EXPECT().GetEntity(mock.Anything, "test-app").Return(
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
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil),
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
	mockEntityProvider.EXPECT().GetEntity(mock.Anything, "test-app-id").Return(
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
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil),
		transactioner: &stubTransactioner{},
		cryptoSvc:     mockCrypto,
		cfg:           testFlowExecCfg,
	}

	flowStep, svcErr := service.Execute(context.Background(), "test-app", existingExecutionID,
		string(providers.FlowTypeAuthentication), false, "submit", map[string]string{}, "", "", "")

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
		string(providers.FlowTypeAuthentication), false, "submit", map[string]string{}, "", "", "")

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
	mockEntityProvider.EXPECT().GetEntity(mock.Anything, "test-app-id").Return(
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
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil),
		transactioner: &stubTransactioner{},
		cryptoSvc:     mockCrypto,
		cfg:           testFlowExecCfg,
	}

	flowStep, svcErr := service.Execute(context.Background(), "test-app", existingExecutionID,
		string(providers.FlowTypeAuthentication), false, "submit", map[string]string{}, challengeToken, "", "")

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
	mockEntityProvider.EXPECT().GetEntity(mock.Anything, "test-app-id").Return(
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
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil),
		transactioner: &stubTransactioner{},
		cryptoSvc:     mockCrypto,
		cfg:           testFlowExecCfg,
	}

	// Execute with empty challenge token
	flowStep, svcErr := service.Execute(context.Background(), "test-app", existingExecutionID,
		string(providers.FlowTypeAuthentication), false, "submit", map[string]string{}, "", "", "")

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
			mockEntityProvider.EXPECT().GetEntity(mock.Anything, "test-app-id").Return(
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
				actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil),
				transactioner: &stubTransactioner{},
				cryptoSvc:     mockCrypto,
				cfg:           testFlowExecCfg,
			}

			flowStep, svcErr := service.Execute(context.Background(), "test-app", existingExecutionID,
				string(providers.FlowTypeAuthentication), false, "submit", map[string]string{},
				tt.challengeToken, "", "")

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
	mockEntityProvider.EXPECT().GetEntity(mock.Anything, "test-app-id").Return(
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
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil),
		transactioner: &stubTransactioner{},
		cryptoSvc:     mockCrypto,
		cfg:           testFlowExecCfg,
	}

	flowStep, svcErr := service.Execute(context.Background(), "test-app", existingExecutionID,
		string(providers.FlowTypeAuthentication), false, "submit", map[string]string{}, "wrong-token", "", "")

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
	mockEntityProvider.EXPECT().GetEntity(mock.Anything, "test-app-id").Return(
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
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil),
		transactioner: &stubTransactioner{},
		cfg:           testFlowExecCfg,
	}

	flowStep, svcErr := service.Execute(context.Background(), "test-app", existingExecutionID,
		string(providers.FlowTypeAuthentication), false, "submit", map[string]string{}, "valid-token", "", "")

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
	mockEntityProvider.EXPECT().GetEntity(mock.Anything, "test-app").Return(
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
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, mockAuthn, nil),
		transactioner: &stubTransactioner{},
		cryptoSvc:     mockCrypto,
		cfg:           testFlowExecCfg,
	}

	// Pass empty executionID to indicate a new flow
	flowStep, svcErr := service.Execute(context.Background(), "test-app", "",
		string(providers.FlowTypeAuthentication), false, "submit", map[string]string{}, "", "valid-secret", "")

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
	return actorprovider.Initialize(mockInbound, mockEP, noopAuthnMgr(), nil), mockInbound, mockEP
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
	mockEP.EXPECT().GetEntity(mock.Anything, "app-x").Return(
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
	mockEP.EXPECT().GetEntity(mock.Anything, "app-x").Return(
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
	mockEP.EXPECT().GetEntity(mock.Anything, "app-x").Return(
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
	mockEntityProvider.EXPECT().GetEntity(mock.Anything, appID).Return(
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
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil),
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
	mockEntityProvider.EXPECT().GetEntity(mock.Anything, appID).Return(
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
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil),
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
	mockEntityProvider.EXPECT().GetEntity(mock.Anything, appID).Return(
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
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil),
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
	mockEntityProvider.EXPECT().GetEntity(mock.Anything, appID).Return(
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
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil),
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
	mockEntityProvider.EXPECT().GetEntity(mock.Anything, appID).Return(
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
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil),
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
	mockEntityProvider.EXPECT().GetEntity(mock.Anything, appID).Return(
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
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil),
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
			name:         "empty app id",
			flowType:     providers.FlowTypeAuthentication,
			client:       nil,
			expectedCode: ErrorInvalidAppID.Code,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(s.T())
			mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(s.T())
			service := &flowExecService{
				actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil),
				cfg:           testFlowExecCfg,
			}

			lookupID := appID
			if tt.name == "empty app id" {
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
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil),
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
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil),
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
	mockEntityProvider.EXPECT().GetEntity(mock.Anything, appID).Return(
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
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, mockAuthn, nil),
		transactioner: &stubTransactioner{},
		cryptoSvc:     mockCrypto,
		cfg:           testFlowExecCfg,
	}

	flowStep, svcErr := service.Execute(context.Background(), appID, "",
		string(providers.FlowTypeAuthentication), false, "submit", map[string]string{}, "", "valid-secret", "")

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
	mockEntityProvider.EXPECT().GetEntity(mock.Anything, "test-app-id").Return(
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
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil),
		transactioner: &stubTransactioner{},
		cfg:           testFlowExecCfg,
	}

	flowStep, svcErr := service.Execute(context.Background(), "test-app", "existing-execution-id",
		string(providers.FlowTypeAuthentication), false, "submit", map[string]string{}, "", "", "")

	s.Nil(svcErr)
	s.NotNil(flowStep)
	s.Equal(providers.FlowStatusComplete, flowStep.Status)
}

func (s *ServiceTestSuite) TestLoadNewContext_InvalidFlowType() {
	service := &flowExecService{cfg: testFlowExecCfg}

	engineCtx, svcErr := service.loadNewContext(context.Background(), "", "test-app", "INVALID_TYPE",
		false, "submit", map[string]string{}, "", "", log.GetLogger())

	s.Nil(engineCtx)
	s.NotNil(svcErr)
	s.Equal(ErrorInvalidFlowType.Code, svcErr.Code)
}

// The caller here is a legitimate administrator, so the rejection is about the target flow's type
// rather than the caller's identity.
func (s *ServiceTestSuite) TestLoadNewContext_ByIDRejectsInteractiveFlow() {
	security.InitSystemPermissions("")
	mockFlowProvider := NewFlowProviderMock(s.T())
	mockFlowProvider.EXPECT().GetFlow(mock.Anything, "authentication-flow").Return(
		&providers.CompleteFlowDefinition{
			ID:       "authentication-flow",
			FlowType: providers.FlowTypeAuthentication,
		}, nil)
	service := &flowExecService{flowProvider: mockFlowProvider, cfg: testFlowExecCfg}

	engineCtx, svcErr := service.loadNewContext(authenticatedAdminContext([]string{"system"}),
		"authentication-flow", "", "", false, "", map[string]string{}, "", "", log.GetLogger())

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
	}

	engineCtx, svcErr := service.loadNewContext(authenticatedAdminContext([]string{"system"}),
		"administration-1", "", "", true, "", map[string]string{}, "", "", log.GetLogger())

	s.Nil(svcErr)
	s.Require().NotNil(engineCtx)
	s.Equal(providers.FlowTypeAdministration, engineCtx.FlowType)
	s.True(engineCtx.Verbose)
}

// A caller that already knows the flow's inputs can supply them on the initiating request and get
// the finished result back, instead of being told which inputs are required and having to send a
// second call. The inputs must therefore be on the context the very first time the engine runs.
func (s *ServiceTestSuite) TestLoadNewContext_ByIDCarriesInitiatingInputs() {
	security.InitSystemPermissions("")
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
	}

	engineCtx, svcErr := service.loadNewContext(authenticatedAdminContext([]string{"system"}),
		"administration-1", "", "", true, "",
		map[string]string{"subject": "019fd0e4-de6c-7ea5-9541-7982f40beeb9"}, "", "", log.GetLogger())

	s.Nil(svcErr)
	s.Require().NotNil(engineCtx)
	s.Equal("019fd0e4-de6c-7ea5-9541-7982f40beeb9", engineCtx.UserInputs["subject"])
}

// Omitting the inputs must stay valid: that is the first leg of the two-request exchange, where the
// response tells the caller which inputs to send back with the execution ID.
func (s *ServiceTestSuite) TestLoadNewContext_ByIDWithoutInputsStartsEmpty() {
	security.InitSystemPermissions("")
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
	}

	engineCtx, svcErr := service.loadNewContext(authenticatedAdminContext([]string{"system"}),
		"administration-1", "", "", true, "", nil, "", "", log.GetLogger())

	s.Nil(svcErr)
	s.Require().NotNil(engineCtx)
	s.NotNil(engineCtx.UserInputs, "an empty input map must still be initialized for the engine")
	s.Empty(engineCtx.UserInputs)
}

// authenticatedAdminContext returns a context carrying an authenticated caller with the permissions
// supplied, as the security middleware would produce for a request bearing a valid token.
func authenticatedAdminContext(permissions []string) context.Context {
	authCtx := security.NewSecurityContextForTest(
		"admin-1", "ou-1", "token", permissions, map[string]interface{}{})
	return security.WithSecurityContextTest(context.Background(), authCtx)
}

func (s *ServiceTestSuite) TestValidateAdministrationCaller() {
	security.InitSystemPermissions("")

	s.Run("RejectsPublicRuntimeContext", func() {
		svcErr := validateAdministrationCaller(
			security.WithRuntimeContext(context.Background()), providers.FlowTypeAdministration)

		s.Require().NotNil(svcErr)
		s.Equal(ErrorAdministrationAuthenticationRequired.Code, svcErr.Code)
	})

	// A bare context carries no authenticated subject. The gate must assert the caller's identity
	// positively rather than inferring it from the absence of the runtime marker.
	s.Run("RejectsContextWithoutAuthenticatedSubject", func() {
		svcErr := validateAdministrationCaller(context.Background(), providers.FlowTypeAdministration)

		s.Require().NotNil(svcErr)
		s.Equal(ErrorAdministrationAuthenticationRequired.Code, svcErr.Code)
	})

	// Authenticated but unprivileged callers are rejected here, not merely by the API permission
	// table. /flow/execute is a public path, so this boundary must hold on its own.
	s.Run("RejectsAuthenticatedCallerWithoutSystemPermission", func() {
		svcErr := validateAdministrationCaller(
			authenticatedAdminContext([]string{"system:user:view"}), providers.FlowTypeAdministration)

		s.Require().NotNil(svcErr)
		s.Equal(ErrorAdministrationPermissionRequired.Code, svcErr.Code)
	})

	s.Run("AllowsAuthenticatedCallerWithSystemPermission", func() {
		s.Nil(validateAdministrationCaller(
			authenticatedAdminContext([]string{"system"}), providers.FlowTypeAdministration))
	})

	s.Run("AllowsPublicInteractiveFlow", func() {
		s.Nil(validateAdministrationCaller(
			security.WithRuntimeContext(context.Background()), providers.FlowTypeAuthentication))
	})

	// The gate must never fire for a non-administration flow, whatever the caller holds.
	s.Run("IgnoresNonAdministrationFlowForAuthenticatedCaller", func() {
		s.Nil(validateAdministrationCaller(
			authenticatedAdminContext(nil), providers.FlowTypeRegistration))
	})
}

// Initiation entry points must apply the same gate as execution, so neither becomes a way around it.
func (s *ServiceTestSuite) TestInitiateFlow_RejectsUnauthenticatedAdministrationFlow() {
	security.InitSystemPermissions("")
	service := &flowExecService{cfg: testFlowExecCfg}

	executionID, svcErr := service.InitiateFlow(security.WithRuntimeContext(context.Background()),
		&FlowInitContext{FlowType: string(providers.FlowTypeAdministration), ApplicationID: "app-1"})

	s.Empty(executionID)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorAdministrationAuthenticationRequired.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestInitiateAndExecute_RejectsUnauthenticatedAdministrationFlow() {
	security.InitSystemPermissions("")
	service := &flowExecService{cfg: testFlowExecCfg}

	flowStep, svcErr := service.InitiateAndExecute(security.WithRuntimeContext(context.Background()),
		&FlowInitContext{FlowType: string(providers.FlowTypeAdministration), ApplicationID: "app-1"})

	s.Nil(flowStep)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorAdministrationAuthenticationRequired.Code, svcErr.Code)
}

// An unauthenticated caller is rejected before the flow is resolved, so the error cannot reveal
// whether the flow ID exists or what type it is, and no lookup work is done on its behalf.
func (s *ServiceTestSuite) TestExecuteByID_RejectsUnauthenticatedPublicRequest() {
	security.InitSystemPermissions("")
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), &config.Config{}))
	mockFlowProvider := NewFlowProviderMock(s.T())
	mockObservability := observabilitymock.NewObservabilityServiceInterfaceMock(s.T())
	mockObservability.EXPECT().IsEnabled().Return(false)
	service := &flowExecService{
		flowProvider: mockFlowProvider, observabilitySvc: mockObservability, cfg: testFlowExecCfg,
	}

	flowStep, svcErr := service.ExecuteByID(security.WithRuntimeContext(context.Background()),
		"does-not-matter", "", false, "", map[string]string{}, "")

	s.Nil(flowStep)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorAdministrationAuthenticationRequired.Code, svcErr.Code)
	mockFlowProvider.AssertNotCalled(s.T(), "GetFlow", mock.Anything, mock.Anything)
}

// An authenticated administrator still cannot use the flow-ID entry point to start a flow of any
// other type, which would bypass that type's application binding and initiation guards.
func (s *ServiceTestSuite) TestExecuteByID_RejectsNonAdministrationFlowType() {
	security.InitSystemPermissions("")
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), &config.Config{}))

	for _, flowType := range []providers.FlowType{
		providers.FlowTypeAuthentication,
		providers.FlowTypeRegistration,
		providers.FlowTypeRecovery,
		providers.FlowTypeSignOut,
		providers.FlowTypeUserOnboarding,
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
				"flow-1", "", true, "", map[string]string{}, "")

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
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil),
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

// --- resolveFlowInitiationMode (type-driven) ---

// The flow-initiation mode is resolved from the application type. Machine-to-machine, browser, and
// mobile (without attestation) apps may not initiate a flow directly and never consult the OAuth
// profile. Full-stack, custom, and mcp apps derive the mode from their profile.
func (s *ServiceTestSuite) TestResolveFlowInitiationMode_ByType() {
	const appID = "test-app"
	tokenExchange := string(providers.GrantTypeTokenExchange)

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
				string(tc.flowType), false, "submit", map[string]string{}, "", tc.flowSecret, "")

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
	mockEntityProvider.EXPECT().GetEntity(mock.Anything, "test-app").Return(
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
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, mockAuthn, nil),
		transactioner: &stubTransactioner{},
	}

	flowStep, svcErr := service.Execute(context.Background(), "test-app", "",
		string(providers.FlowTypeAuthentication), false, "submit", map[string]string{}, "", "valid-secret", "")

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
		string(providers.FlowTypeAuthentication), false, "submit", map[string]string{}, "", "wrong-secret", "")

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
		string(providers.FlowTypeAuthentication), false, "submit", map[string]string{}, "", "", "")

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
	mockEntityProvider.EXPECT().GetEntity(mock.Anything, "test-app").Return(
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
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, mockAuthn, nil),
		transactioner: &stubTransactioner{},
	}

	flowStep, svcErr := service.Execute(context.Background(), "test-app", "",
		string(providers.FlowTypeAuthentication), false, "submit", map[string]string{}, "", "valid-secret", "")

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
		actorProvider:    actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil),
		observabilitySvc: mockObservability,
		transactioner:    &stubTransactioner{},
	}

	flowStep, svcErr := service.Execute(context.Background(), "test-app", "",
		string(providers.FlowTypeAuthentication), false, "submit", map[string]string{}, "", "", "")

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
	mockEntityProvider.EXPECT().GetEntity(mock.Anything, "test-app").Return(
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
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider, noopAuthnMgr(), nil),
		transactioner: &stubTransactioner{},
	}

	flowStep, svcErr := service.Execute(context.Background(), "test-app", "existing-execution-id",
		string(providers.FlowTypeAuthentication), false, "submit", map[string]string{}, "valid-token", "", "")

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
