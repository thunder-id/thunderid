/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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
	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	flowconfig "github.com/thunder-id/thunderid/internal/flow/config"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/interceptor"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	kmprovider "github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	"github.com/thunder-id/thunderid/internal/system/kmprovider/defaultkm"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/actorprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/crypto/cryptomock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/inboundclientmock"
	"github.com/thunder-id/thunderid/tests/mocks/observability/observabilitymock"
)

const existingExecutionID = "existing-execution-id"

// txMarkerKey is an unexported type used as a context key for the transaction marker in tests.
type txMarkerKey struct{}

// stubTransactioner is a stub implementation of Transactioner for testing.
type stubTransactioner struct{}

func (s *stubTransactioner) Transact(ctx context.Context, txFunc func(context.Context) error) error {
	txCtx := context.WithValue(ctx, txMarkerKey{}, "tx")
	return txFunc(txCtx)
}

const testUserOnboardingFlowHandle = "onboarding-handle"

var testFlowConfig = engineconfig.FlowConfig{
	UserOnboardingFlowHandle: testUserOnboardingFlowHandle,
}

var testFlowExecCfg = flowconfig.Config{
	Flow: testFlowConfig,
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
	appID := "test-app-123"

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
			mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return([]byte("encrypted-ctx"), nil, nil)

			// Create service with mocked dependencies
			service := &flowExecService{
				graphBuilder:  mockGraphBuilder,
				flowProvider:  mockFlowProvider,
				flowStore:     mockStore,
				actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider),
				flowEngine:    nil,
				transactioner: &stubTransactioner{},
				cryptoSvc:     mockCrypto,
				cfg:           testFlowExecCfg,
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
				mockEntityProvider.EXPECT().GetEntity(appID).
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
	appID := "test-app-123"

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
				mockEntityProvider.EXPECT().GetEntity(appID).Return(
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
			mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return([]byte("encrypted-ctx"), nil, nil).Maybe()

			// Create service with mocked dependencies
			service := &flowExecService{
				graphBuilder:  mockGraphBuilder,
				flowProvider:  mockFlowProvider,
				flowStore:     mockStore,
				actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider),
				flowEngine:    nil,
				transactioner: &stubTransactioner{},
				cryptoSvc:     mockCrypto,
				cfg:           testFlowExecCfg,
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
			expected: defaultAuthFlowExpiry,
		},
		{
			name:     "Registration flow",
			flowType: providers.FlowTypeRegistration,
			expected: defaultRegistrationFlowExpiry,
		},
		{
			name:     "User onboarding flow",
			flowType: providers.FlowTypeUserOnboarding,
			expected: defaultUserOnboardingFlowExpiry,
		},
		{
			name:     "Unknown flow type (fallback)",
			flowType: providers.FlowType("UNKNOWN_FLOW"),
			expected: defaultAuthFlowExpiry,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.getFlowExpirySeconds(tt.flowType)
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
	mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]byte(encryptedPayload), nil, nil)

	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, "test-app").Return(
		&inboundmodel.InboundClient{ID: "test-app", AuthFlowID: "auth-graph-1"}, nil)
	mockEntityProvider.EXPECT().GetEntity("test-app").Return(
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
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider),
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
	plainCtx, err := FromEngineContext(engineCtx)
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
	mockCrypto.EXPECT().Decrypt(mock.Anything, mock.Anything, mock.Anything,
		[]byte(encryptedStoredCtx.Context)).
		Return([]byte(plainCtx.Context), nil)
	// Encrypt called when updating context after engine runs
	mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]byte("re-encrypted"), nil, nil)

	mockStore.EXPECT().GetFlowContext(mock.Anything, existingExecutionID).Return(encryptedStoredCtx, nil)
	mockFlowProvider.EXPECT().
		GetFlow(mock.Anything, "test-graph-id").
		Return(&providers.CompleteFlowDefinition{ID: "test-graph-id"}, nil)
	mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(testGraph, nil)
	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, "test-app-id").Return(
		&inboundmodel.InboundClient{ID: "test-app-id", AuthFlowID: "test-graph-id"}, nil)
	mockEntityProvider.EXPECT().GetEntity("test-app-id").Return(
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
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider),
		transactioner: &stubTransactioner{},
		cryptoSvc:     mockCrypto,
		cfg:           testFlowExecCfg,
	}

	flowStep, svcErr := service.Execute(context.Background(), "test-app", existingExecutionID,
		string(providers.FlowTypeAuthentication), false, "submit", map[string]string{}, "")

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
	mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(
			func(
				ctx context.Context,
				_ *kmprovider.KeyRef,
				_ cryptolib.AlgorithmParams,
				content []byte) ([]byte, *cryptolib.CryptoDetails, error) {
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
	mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(
			ctx context.Context,
			_ *kmprovider.KeyRef,
			_ cryptolib.AlgorithmParams,
			content []byte) ([]byte, *cryptolib.CryptoDetails, error) {
			encrypted, encErr := mockConfigCryptoService.Encrypt(ctx, content)
			return encrypted, nil, encErr
		})
	mockCrypto.EXPECT().Decrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(
			ctx context.Context,
			_ *kmprovider.KeyRef,
			_ cryptolib.AlgorithmParams, content []byte) ([]byte, error) {
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
		cryptolib.AlgorithmParams{Algorithm: cryptolib.AlgorithmAESGCM},
		[]byte(encryptedEngineCtx.Context))
	assert.NoError(t, err)

	restoredDB := &FlowContextDB{
		ExecutionID: encryptedEngineCtx.ExecutionID,
		Context:     string(decryptedBytes),
	}

	// Step 3: Convert back to EngineContext
	resultCtx, err := restoredDB.ToEngineContext(context.Background(), testGraph)
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
	mockCrypto.EXPECT().Decrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
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
		string(providers.FlowTypeAuthentication), false, "submit", map[string]string{}, "")

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
	storedCtx, err := FromEngineContext(engineCtx)
	assert.NoError(t, err)

	mockStore := newFlowStoreInterfaceMock(t)
	mockFlowProvider := NewFlowProviderMock(t)
	mockGraphBuilder := NewGraphBuilderInterfaceMock(t)
	mockEngine := newFlowEngineInterfaceMock(t)
	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(t)
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)
	mockCrypto := cryptomock.NewRuntimeCryptoProviderMock(t)
	mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]byte("encrypted-ctx"), nil, nil)

	mockStore.EXPECT().GetFlowContext(mock.Anything, existingExecutionID).Return(storedCtx, nil)
	mockFlowProvider.EXPECT().
		GetFlow(mock.Anything, "test-graph-id").
		Return(&providers.CompleteFlowDefinition{ID: "test-graph-id"}, nil)
	mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(testGraph, nil)
	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, "test-app-id").Return(
		&inboundmodel.InboundClient{ID: "test-app-id", AuthFlowID: "test-graph-id"}, nil)
	mockEntityProvider.EXPECT().GetEntity("test-app-id").Return(
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
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider),
		transactioner: &stubTransactioner{},
		cryptoSvc:     mockCrypto,
		cfg:           testFlowExecCfg,
	}

	flowStep, svcErr := service.Execute(context.Background(), "test-app", existingExecutionID,
		string(providers.FlowTypeAuthentication), false, "submit", map[string]string{}, challengeToken)

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
	storedCtx, err := FromEngineContext(engineCtx)
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
	mockEntityProvider.EXPECT().GetEntity("test-app-id").Return(
		&providers.Entity{ID: "test-app-id", Category: providers.EntityCategoryApp},
		(*entityprovider.EntityProviderError)(nil))

	mockCrypto := cryptomock.NewRuntimeCryptoProviderMock(t)
	mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
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
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider),
		transactioner: &stubTransactioner{},
		cryptoSvc:     mockCrypto,
		cfg:           testFlowExecCfg,
	}

	// Execute with empty challenge token
	flowStep, svcErr := service.Execute(context.Background(), "test-app", existingExecutionID,
		string(providers.FlowTypeAuthentication), false, "submit", map[string]string{}, "")

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
			storedCtx, err := FromEngineContext(engineCtx)
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
			mockEntityProvider.EXPECT().GetEntity("test-app-id").Return(
				&providers.Entity{ID: "test-app-id", Category: providers.EntityCategoryApp},
				(*entityprovider.EntityProviderError)(nil))

			mockCrypto := cryptomock.NewRuntimeCryptoProviderMock(t)
			mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
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
				actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider),
				transactioner: &stubTransactioner{},
				cryptoSvc:     mockCrypto,
				cfg:           testFlowExecCfg,
			}

			flowStep, svcErr := service.Execute(context.Background(), "test-app", existingExecutionID,
				string(providers.FlowTypeAuthentication), false, "submit", map[string]string{}, tt.challengeToken)

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
	storedCtx, err := FromEngineContext(engineCtx)
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
	mockEntityProvider.EXPECT().GetEntity("test-app-id").Return(
		&providers.Entity{ID: "test-app-id", Category: providers.EntityCategoryApp},
		(*entityprovider.EntityProviderError)(nil))
	mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
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
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider),
		transactioner: &stubTransactioner{},
		cryptoSvc:     mockCrypto,
		cfg:           testFlowExecCfg,
	}

	flowStep, svcErr := service.Execute(context.Background(), "test-app", existingExecutionID,
		string(providers.FlowTypeAuthentication), false, "submit", map[string]string{}, "wrong-token")

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
	storedCtx, err := FromEngineContext(engineCtx)
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
	mockEntityProvider.EXPECT().GetEntity("test-app-id").Return(
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
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider),
		transactioner: &stubTransactioner{},
		cfg:           testFlowExecCfg,
	}

	flowStep, svcErr := service.Execute(context.Background(), "test-app", existingExecutionID,
		string(providers.FlowTypeAuthentication), false, "submit", map[string]string{}, "valid-token")

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

	mockInboundClient.EXPECT().GetOAuthProfileByEntityID(mock.Anything, "test-app").Return(nil, nil)
	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, "test-app").Return(
		&inboundmodel.InboundClient{ID: "test-app", AuthFlowID: "auth-graph-1"}, nil).Times(2)
	mockEntityProvider.EXPECT().GetEntity("test-app").Return(
		&providers.Entity{ID: "test-app", Category: providers.EntityCategoryApp},
		(*entityprovider.EntityProviderError)(nil))
	mockFlowProvider.EXPECT().
		GetFlow(mock.Anything, "auth-graph-1").
		Return(&providers.CompleteFlowDefinition{ID: "auth-graph-1"}, nil)
	mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(testGraph, nil)
	mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
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
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider),
		transactioner: &stubTransactioner{},
		cryptoSvc:     mockCrypto,
		cfg:           testFlowExecCfg,
	}

	// Pass empty executionID to indicate a new flow
	flowStep, svcErr := service.Execute(context.Background(), "test-app", "",
		string(providers.FlowTypeAuthentication), false, "submit", map[string]string{}, "")

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
	return actorprovider.Initialize(mockInbound, mockEP), mockInbound, mockEP
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
	mockEP.EXPECT().GetEntity("app-x").Return(
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
	mockEP.EXPECT().GetEntity("app-x").Return(
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
	mockEP.EXPECT().GetEntity("app-x").Return(
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
	mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
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
	mockEntityProvider.EXPECT().GetEntity(appID).Return(
		&providers.Entity{ID: appID, Category: providers.EntityCategoryApp}, nil)
	mockFlowProvider.EXPECT().
		GetFlow(mock.Anything, "auth-graph-expiry").
		Return(&providers.CompleteFlowDefinition{ID: "auth-graph-expiry"}, nil)
	mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(testGraph, nil)
	mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
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
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider),
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
	mockEntityProvider.EXPECT().GetEntity(appID).Return(
		&providers.Entity{ID: appID, Category: providers.EntityCategoryApp}, nil)
	mockFlowProvider.EXPECT().
		GetFlow(mock.Anything, "auth-graph-defexp").
		Return(&providers.CompleteFlowDefinition{ID: "auth-graph-defexp"}, nil)
	mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(testGraph, nil)
	mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]byte("encrypted"), nil, nil)
	mockStore.EXPECT().StoreFlowContext(mock.Anything, mock.Anything,
		mock.MatchedBy(func(exp int64) bool { return exp == defaultAuthFlowExpiry })).
		Return(nil)
	mockEngineInner.EXPECT().Execute(mock.Anything).
		Return(FlowStep{Status: providers.FlowStatusIncomplete}, nil)

	svc := &flowExecService{
		flowStore:     mockStore,
		graphBuilder:  mockGraphBuilder,
		flowProvider:  mockFlowProvider,
		flowEngine:    mockEngineInner,
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider),
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
	mockEntityProvider.EXPECT().GetEntity(appID).Return(
		&providers.Entity{ID: appID, Category: providers.EntityCategoryApp}, nil)
	mockFlowProvider.EXPECT().
		GetFlow(mock.Anything, "auth-graph-ia").
		Return(&providers.CompleteFlowDefinition{ID: "auth-graph-ia"}, nil)
	mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(testGraph, nil)
	mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
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
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider),
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
	mockEntityProvider.EXPECT().GetEntity(appID).Return(
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
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider),
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
	mockEntityProvider.EXPECT().GetEntity(appID).Return(
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
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider),
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
	mockEntityProvider.EXPECT().GetEntity(appID).Return(
		&providers.Entity{ID: appID, Category: providers.EntityCategoryApp}, nil)
	mockFlowProvider.EXPECT().
		GetFlow(mock.Anything, "auth-graph-se").
		Return(&providers.CompleteFlowDefinition{ID: "auth-graph-se"}, nil)
	mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(testGraph, nil)
	mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
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
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider),
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
				actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider),
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
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider),
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
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider),
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

	mockInboundClient.EXPECT().GetOAuthProfileByEntityID(mock.Anything, appID).Return(nil, nil)
	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, appID).Return(
		&inboundmodel.InboundClient{ID: appID, AuthFlowID: "auth-graph-new"}, nil)
	mockEntityProvider.EXPECT().GetEntity(appID).Return(
		&providers.Entity{ID: appID, Category: providers.EntityCategoryApp}, nil)
	mockFlowProvider.EXPECT().
		GetFlow(mock.Anything, "auth-graph-new").
		Return(&providers.CompleteFlowDefinition{ID: "auth-graph-new"}, nil)
	mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(testGraph, nil)
	mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
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
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider),
		transactioner: &stubTransactioner{},
		cryptoSvc:     mockCrypto,
		cfg:           testFlowExecCfg,
	}

	flowStep, svcErr := service.Execute(context.Background(), appID, "",
		string(providers.FlowTypeAuthentication), false, "submit", map[string]string{}, "")

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
	storedCtx, err := FromEngineContext(engineCtx)
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
	mockEntityProvider.EXPECT().GetEntity("test-app-id").Return(
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
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider),
		transactioner: &stubTransactioner{},
		cfg:           testFlowExecCfg,
	}

	flowStep, svcErr := service.Execute(context.Background(), "test-app", "existing-execution-id",
		string(providers.FlowTypeAuthentication), false, "submit", map[string]string{}, "")

	s.Nil(svcErr)
	s.NotNil(flowStep)
	s.Equal(providers.FlowStatusComplete, flowStep.Status)
}

func (s *ServiceTestSuite) TestLoadNewContext_InvalidFlowType() {
	service := &flowExecService{cfg: testFlowExecCfg}

	engineCtx, svcErr := service.loadNewContext(context.Background(), "test-app", "INVALID_TYPE",
		false, "submit", map[string]string{}, log.GetLogger())

	s.Nil(engineCtx)
	s.NotNil(svcErr)
	s.Equal(ErrorInvalidFlowType.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestSetApplicationToContext_ActorNotFound() {
	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(s.T())
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(s.T())
	service := &flowExecService{
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider),
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
	s.Equal(defaultRecoveryFlowExpiry, service.getFlowExpirySeconds(providers.FlowTypeRecovery))
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

// --- checkDirectFlowInitiationAllowed ---

func (s *ServiceTestSuite) TestExecute_NewFlow_AuthCodeApp_Blocked() {
	t := s.T()
	testConfig := &config.Config{}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)

	mockActorProvider := actorprovidermock.NewActorProviderMock(t)
	mockObservability := observabilitymock.NewObservabilityServiceInterfaceMock(t)
	mockActorProvider.EXPECT().GetOAuthProfileByID(mock.Anything, "test-app").Return(
		&providers.OAuthProfile{GrantTypes: []string{"authorization_code"}}, nil)
	mockObservability.EXPECT().IsEnabled().Return(false)

	service := &flowExecService{
		actorProvider:    mockActorProvider,
		observabilitySvc: mockObservability,
		transactioner:    &stubTransactioner{},
	}

	flowStep, svcErr := service.Execute(context.Background(), "test-app", "",
		string(providers.FlowTypeAuthentication), false, "submit", map[string]string{}, "")

	s.Nil(flowStep)
	s.NotNil(svcErr)
	s.Equal(ErrorDirectFlowInitiationNotPermitted.Code, svcErr.Code)
	s.Equal(tidcommon.ClientErrorType, svcErr.Type)
}

func (s *ServiceTestSuite) TestExecute_NewFlow_NonAuthCodeApp_Allowed() {
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

	mockInboundClient.EXPECT().GetOAuthProfileByEntityID(mock.Anything, "test-app").Return(
		&providers.OAuthProfile{GrantTypes: []string{"client_credentials"}}, nil)
	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, "test-app").Return(
		&inboundmodel.InboundClient{ID: "test-app", AuthFlowID: "auth-graph-1"}, nil).Times(2)
	mockEntityProvider.EXPECT().GetEntity("test-app").Return(
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
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider),
		transactioner: &stubTransactioner{},
	}

	flowStep, svcErr := service.Execute(context.Background(), "test-app", "",
		string(providers.FlowTypeAuthentication), false, "submit", map[string]string{}, "")

	s.Nil(svcErr)
	s.NotNil(flowStep)
}

func (s *ServiceTestSuite) TestExecute_NewFlow_OAuthProfileError_InternalError() {
	t := s.T()
	testConfig := &config.Config{}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)

	mockActorProvider := actorprovidermock.NewActorProviderMock(t)
	mockObservability := observabilitymock.NewObservabilityServiceInterfaceMock(t)
	mockActorProvider.EXPECT().GetOAuthProfileByID(mock.Anything, "test-app").Return(
		nil, &tidcommon.InternalServerError)
	mockObservability.EXPECT().IsEnabled().Return(false)

	service := &flowExecService{
		actorProvider:    mockActorProvider,
		observabilitySvc: mockObservability,
		transactioner:    &stubTransactioner{},
	}

	flowStep, svcErr := service.Execute(context.Background(), "test-app", "",
		string(providers.FlowTypeAuthentication), false, "submit", map[string]string{}, "")

	s.Nil(flowStep)
	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestExecute_NewFlow_OAuthProfileNil_Allowed() {
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

	mockInboundClient.EXPECT().GetOAuthProfileByEntityID(mock.Anything, "test-app").Return(nil, nil)
	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, "test-app").Return(
		&inboundmodel.InboundClient{ID: "test-app", AuthFlowID: "auth-graph-1"}, nil).Times(2)
	mockEntityProvider.EXPECT().GetEntity("test-app").Return(
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
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider),
		transactioner: &stubTransactioner{},
	}

	flowStep, svcErr := service.Execute(context.Background(), "test-app", "",
		string(providers.FlowTypeAuthentication), false, "submit", map[string]string{}, "")

	s.Nil(svcErr)
	s.NotNil(flowStep)
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
	storedCtx, err := FromEngineContext(engineCtx)
	s.NoError(err)

	mockStore.EXPECT().GetFlowContext(mock.Anything, "existing-execution-id").Return(storedCtx, nil)
	mockFlowProvider.EXPECT().
		GetFlow(mock.Anything, "auth-graph-1").
		Return(&providers.CompleteFlowDefinition{ID: "auth-graph-1"}, nil)
	mockGraphBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(testGraph, nil)
	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, "test-app").Return(
		&inboundmodel.InboundClient{ID: "test-app", AuthFlowID: "auth-graph-1"}, nil)
	mockEntityProvider.EXPECT().GetEntity("test-app").Return(
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
		actorProvider: actorprovider.Initialize(mockInboundClient, mockEntityProvider),
		transactioner: &stubTransactioner{},
	}

	flowStep, svcErr := service.Execute(context.Background(), "test-app", "existing-execution-id",
		string(providers.FlowTypeAuthentication), false, "submit", map[string]string{}, "valid-token")

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

func (s *ServiceTestSuite) TestCheckDirectFlowInitiationAllowed_ClientNotFound() {
	t := s.T()
	mockActorProvider := actorprovidermock.NewActorProviderMock(t)
	mockActorProvider.EXPECT().GetOAuthProfileByID(mock.Anything, "app-notfound").Return(
		(*providers.OAuthProfile)(nil), &actorprovider.ErrorActorNotFound)

	service := &flowExecService{
		actorProvider: mockActorProvider,
		cfg:           testFlowExecCfg,
	}

	svcErr := service.checkDirectFlowInitiationAllowed(context.Background(), "app-notfound",
		providers.FlowTypeAuthentication, log.GetLogger())
	s.Nil(svcErr)
}

func (s *ServiceTestSuite) TestCheckDirectFlowInitiationAllowed_NonAuthFlowAllowed() {
	service := &flowExecService{cfg: testFlowExecCfg}

	svcErr := service.checkDirectFlowInitiationAllowed(context.Background(), "test-app",
		providers.FlowTypeRegistration, log.GetLogger())
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
