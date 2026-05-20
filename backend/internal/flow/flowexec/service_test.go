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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	flowmgt "github.com/thunder-id/thunderid/internal/flow/mgt"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/cryptolab"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	i18ncore "github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/kmprovider"
	"github.com/thunder-id/thunderid/internal/system/kmprovider/defaultkm"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/crypto/cryptomock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/flowmgtmock"
	"github.com/thunder-id/thunderid/tests/mocks/inboundclientmock"
)

// txMarkerKey is an unexported type used as a context key for the transaction marker in tests.
type txMarkerKey struct{}

// stubTransactioner is a stub implementation of Transactioner for testing.
type stubTransactioner struct{}

func (s *stubTransactioner) Transact(ctx context.Context, txFunc func(context.Context) error) error {
	txCtx := context.WithValue(ctx, txMarkerKey{}, "tx")
	return txFunc(txCtx)
}
func TestInitiateFlowNilContext(t *testing.T) {
	// Setup
	service := &flowExecService{}

	// Execute
	executionID, err := service.InitiateFlow(context.Background(), nil)

	// Assert
	assert.NotNil(t, err)
	assert.Empty(t, executionID)
	assert.Equal(t, "FES-1008", err.Code)
}

func TestInitiateFlowEmptyApplicationID(t *testing.T) {
	// Setup
	service := &flowExecService{}

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
	service := &flowExecService{}

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
	service := &flowExecService{}

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

	flowFactory, _ := core.Initialize(cache.Initialize())
	testGraph := flowFactory.CreateGraph("auth-graph-1", common.FlowTypeAuthentication)

	// Mock inbound client + entity for the flow's owning entity (shared across test cases).
	mockClient := &inboundmodel.InboundClient{
		ID:         "app-id-123",
		AuthFlowID: "auth-graph-1",
	}
	mockEntity := &entityprovider.Entity{ID: appID, Category: entityprovider.EntityCategoryApp}

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
			mockFlowMgtSvc := flowmgtmock.NewFlowMgtServiceInterfaceMock(t)
			mockCrypto := cryptomock.NewRuntimeCryptoProviderMock(t)
			mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return([]byte("encrypted-ctx"), nil, nil)

			// Create service with mocked dependencies
			service := &flowExecService{
				flowMgtService:       mockFlowMgtSvc,
				flowStore:            mockStore,
				inboundClientService: mockInboundClient,
				entityProvider:       mockEntityProvider,
				flowEngine:           nil,
				transactioner:        &stubTransactioner{},
				cryptoSvc:            mockCrypto,
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
				initContext.FlowType = string(common.FlowTypeUserOnboarding)
				initContext.ApplicationID = "" // System flows don't need app ID

				// Mock flow management service to return flow by handle
				mockFlow := &flowmgt.CompleteFlowDefinition{ID: "onboarding-flow-123"}
				mockFlowMgtSvc.EXPECT().GetFlowByHandle(mock.Anything,
					mock.Anything, common.FlowTypeUserOnboarding).Return(mockFlow, nil)

				// Mock GetGraph call which is made during initContext
				inviteGraph := flowFactory.CreateGraph("onboarding-flow-123", common.FlowTypeUserOnboarding)
				mockFlowMgtSvc.EXPECT().GetGraph(mock.Anything, "onboarding-flow-123").Return(inviteGraph, nil)

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
				mockFlowMgtSvc.EXPECT().GetGraph(mock.Anything, "auth-graph-1").Return(testGraph, nil)
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

	flowFactory, _ := core.Initialize(cache.Initialize())

	tests := []struct {
		name       string
		setupMocks func(
			*flowStoreInterfaceMock,
			*inboundclientmock.InboundClientServiceInterfaceMock,
			*entityprovidermock.EntityProviderInterfaceMock,
			*flowmgtmock.FlowMgtServiceInterfaceMock,
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
				mockFlowMgtSvc *flowmgtmock.FlowMgtServiceInterfaceMock,
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
				mockFlowMgtSvc *flowmgtmock.FlowMgtServiceInterfaceMock,
			) {
				mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, appID).
					Return(nil, assert.AnError)
			},
			expectedErrorCode: serviceerror.InternalServerError.Code,
		},
		{
			name: "error from flowMgtService.GetGraph - graph not found",
			setupMocks: func(
				mockStore *flowStoreInterfaceMock,
				mockInboundClient *inboundclientmock.InboundClientServiceInterfaceMock,
				mockEntityProvider *entityprovidermock.EntityProviderInterfaceMock,
				mockFlowMgtSvc *flowmgtmock.FlowMgtServiceInterfaceMock,
			) {
				mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, appID).
					Return(&inboundmodel.InboundClient{ID: appID, AuthFlowID: "auth-graph-1"}, nil)

				// Mock flow management service to return error (graph not found)
				mockFlowMgtSvc.EXPECT().GetGraph(mock.Anything, "auth-graph-1").
					Return(nil, &serviceerror.InternalServerError)
			},
			expectedErrorCode: serviceerror.InternalServerError.Code,
		},
		{
			name: "error from storeContext - store failure",
			setupMocks: func(
				mockStore *flowStoreInterfaceMock,
				mockInboundClient *inboundclientmock.InboundClientServiceInterfaceMock,
				mockEntityProvider *entityprovidermock.EntityProviderInterfaceMock,
				mockFlowMgtSvc *flowmgtmock.FlowMgtServiceInterfaceMock,
			) {
				mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, appID).
					Return(&inboundmodel.InboundClient{ID: appID, AuthFlowID: "auth-graph-1"}, nil)
				mockEntityProvider.EXPECT().GetEntity(appID).Return(
					&entityprovider.Entity{ID: appID, Category: entityprovider.EntityCategoryApp},
					(*entityprovider.EntityProviderError)(nil))

				// Mock flow management service to return valid graph
				testGraph := flowFactory.CreateGraph("auth-graph-1", common.FlowTypeAuthentication)
				mockFlowMgtSvc.EXPECT().GetGraph(mock.Anything, "auth-graph-1").Return(testGraph, nil)

				// Mock store to return error
				mockStore.EXPECT().StoreFlowContext(
					mock.MatchedBy(func(ctx context.Context) bool {
						return ctx.Value(txMarkerKey{}) == "tx"
					}),
					mock.AnythingOfType("FlowContextDB"), mock.Anything).Return(assert.AnError)
			},
			expectedErrorCode: serviceerror.InternalServerError.Code,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockStore := newFlowStoreInterfaceMock(t)
			mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(t)
			mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)
			mockFlowMgtSvc := flowmgtmock.NewFlowMgtServiceInterfaceMock(t)
			mockCrypto := cryptomock.NewRuntimeCryptoProviderMock(t)
			mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return([]byte("encrypted-ctx"), nil, nil).Maybe()

			// Create service with mocked dependencies
			service := &flowExecService{
				flowMgtService:       mockFlowMgtSvc,
				flowStore:            mockStore,
				inboundClientService: mockInboundClient,
				entityProvider:       mockEntityProvider,
				flowEngine:           nil,
				transactioner:        &stubTransactioner{},
				cryptoSvc:            mockCrypto,
			}

			initContext := &FlowInitContext{
				ApplicationID: appID,
				FlowType:      "AUTHENTICATION",
				RuntimeData: map[string]string{
					"test": "data",
				},
			}

			// Setup test-specific mocks
			tt.setupMocks(mockStore, mockInboundClient, mockEntityProvider, mockFlowMgtSvc)

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
	service := &flowExecService{}

	tests := []struct {
		name     string
		flowType common.FlowType
		expected int64
	}{
		{
			name:     "Authentication flow",
			flowType: common.FlowTypeAuthentication,
			expected: defaultAuthFlowExpiry,
		},
		{
			name:     "Registration flow",
			flowType: common.FlowTypeRegistration,
			expected: defaultRegistrationFlowExpiry,
		},
		{
			name:     "User onboarding flow",
			flowType: common.FlowTypeUserOnboarding,
			expected: defaultUserOnboardingFlowExpiry,
		},
		{
			name:     "Unknown flow type (fallback)",
			flowType: common.FlowType("UNKNOWN_FLOW"),
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

	flowFactory, _ := core.Initialize(cache.Initialize())
	testGraph := flowFactory.CreateGraph("auth-graph-1", common.FlowTypeAuthentication)

	mockStore := newFlowStoreInterfaceMock(t)
	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(t)
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)
	mockFlowMgtSvc := flowmgtmock.NewFlowMgtServiceInterfaceMock(t)
	mockCrypto := cryptomock.NewRuntimeCryptoProviderMock(t)
	mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]byte(encryptedPayload), nil, nil)

	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, "test-app").Return(
		&inboundmodel.InboundClient{ID: "test-app", AuthFlowID: "auth-graph-1"}, nil)
	mockEntityProvider.EXPECT().GetEntity("test-app").Return(
		&entityprovider.Entity{ID: "test-app", Category: entityprovider.EntityCategoryApp},
		(*entityprovider.EntityProviderError)(nil))
	mockFlowMgtSvc.EXPECT().GetGraph(mock.Anything, "auth-graph-1").Return(testGraph, nil)
	mockStore.EXPECT().StoreFlowContext(
		mock.MatchedBy(func(ctx context.Context) bool { return ctx.Value(txMarkerKey{}) == "tx" }),
		mock.MatchedBy(func(dbModel FlowContextDB) bool {
			return dbModel.Context == encryptedPayload
		}),
		mock.Anything).Return(nil)

	service := &flowExecService{
		flowMgtService:       mockFlowMgtSvc,
		flowStore:            mockStore,
		inboundClientService: mockInboundClient,
		entityProvider:       mockEntityProvider,
		transactioner:        &stubTransactioner{},
		cryptoSvc:            mockCrypto,
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
	flowFactory, _ := core.Initialize(cache.Initialize())
	testGraph := flowFactory.CreateGraph("test-graph-id", common.FlowTypeAuthentication)

	engineCtx := EngineContext{
		ExecutionID:       "existing-execution-id",
		AppID:             "test-app-id",
		FlowType:          common.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{Attributes: map[string]interface{}{}},
		UserInputs:        map[string]string{},
		RuntimeData:       map[string]string{},
		ExecutionHistory:  map[string]*common.NodeExecutionRecord{},
		Graph:             testGraph,
	}
	plainCtx, err := FromEngineContext(engineCtx)
	assert.NoError(t, err)

	// Simulate what the store returns: an encrypted blob
	encryptedStoredCtx := &FlowContextDB{
		ExecutionID: "existing-execution-id",
		Context:     `{"alg":"AES-GCM","ct":"c2VjcmV0","kid":"k1"}`,
	}

	mockStore := newFlowStoreInterfaceMock(t)
	mockFlowMgtSvc := flowmgtmock.NewFlowMgtServiceInterfaceMock(t)
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

	mockStore.EXPECT().GetFlowContext(mock.Anything, "existing-execution-id").Return(encryptedStoredCtx, nil)
	mockFlowMgtSvc.EXPECT().GetGraph(mock.Anything, "test-graph-id").Return(testGraph, nil)
	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, "test-app-id").Return(
		&inboundmodel.InboundClient{ID: "test-app-id", AuthFlowID: "test-graph-id"}, nil)
	mockEntityProvider.EXPECT().GetEntity("test-app-id").Return(
		&entityprovider.Entity{ID: "test-app-id", Category: entityprovider.EntityCategoryApp},
		(*entityprovider.EntityProviderError)(nil))

	// Engine receives a properly restored context — not the raw encrypted bytes
	mockEngine.EXPECT().Execute(mock.MatchedBy(func(ctx *EngineContext) bool {
		return ctx != nil && ctx.AppID == "test-app-id" && ctx.ExecutionID == "existing-execution-id"
	})).Return(FlowStep{Status: common.FlowStatusIncomplete}, nil)

	mockStore.EXPECT().UpdateFlowContext(
		mock.MatchedBy(func(ctx context.Context) bool { return ctx.Value(txMarkerKey{}) == "tx" }),
		mock.AnythingOfType("FlowContextDB")).Return(nil)

	service := &flowExecService{
		flowStore:            mockStore,
		flowMgtService:       mockFlowMgtSvc,
		flowEngine:           mockEngine,
		inboundClientService: mockInboundClient,
		entityProvider:       mockEntityProvider,
		transactioner:        &stubTransactioner{},
		cryptoSvc:            mockCrypto,
	}

	flowStep, svcErr := service.Execute(context.Background(), "test-app", "existing-execution-id",
		string(common.FlowTypeAuthentication), false, "submit", map[string]string{}, "")

	assert.Nil(t, svcErr)
	assert.NotNil(t, flowStep)
	assert.Equal(t, common.FlowStatusIncomplete, flowStep.Status)
}

func TestEncryptedContext_SensitiveFieldsHidden(t *testing.T) {
	// Verifies that after encryptEngineContext, sensitive fields (appId, userId, token, inputs)
	// are not visible in the encrypted bytes stored — matching the protection guarantee.
	testConfig := &config.Config{
		Crypto: config.CryptoConfig{
			Encryption: config.EncryptionConfig{
				Key: "2729a7928c79371e5f312167269294a14bb0660fd166b02a408a20fa73271580",
			},
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)

	cfgSvc, err := defaultkm.InitConfigProvider()
	assert.NoError(t, err)

	mockCrypto := cryptomock.NewRuntimeCryptoProviderMock(t)
	mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(
			func(
				ctx context.Context,
				_ *kmprovider.KeyRef,
				_ cryptolab.AlgorithmParams,
				content []byte) ([]byte, *cryptolab.CryptoDetails, error) {
				encrypted, encErr := cfgSvc.Encrypt(ctx, content)
				return encrypted, nil, encErr
			})

	flowFactory, _ := core.Initialize(cache.Initialize())
	testGraph := flowFactory.CreateGraph("test-graph-id", common.FlowTypeAuthentication)

	sensitiveAppID := "app-sensitive-99999"
	sensitiveUserID := "user-sensitive-88888"
	sensitiveInput := "sensitive-password-value"
	sensitiveRuntimeData := "sensitive-state-value"

	engineCtx := EngineContext{
		ExecutionID: "test-flow-id",
		AppID:       sensitiveAppID,
		FlowType:    common.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          sensitiveUserID,
			Attributes:      map[string]interface{}{},
		},
		UserInputs:       map[string]string{"password": sensitiveInput},
		RuntimeData:      map[string]string{"state": sensitiveRuntimeData},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Graph:            testGraph,
	}

	svc := &flowExecService{cryptoSvc: mockCrypto}
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
	testConfig := &config.Config{
		Crypto: config.CryptoConfig{
			Encryption: config.EncryptionConfig{
				Key: "2729a7928c79371e5f312167269294a14bb0660fd166b02a408a20fa73271580",
			},
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)

	cfgSvc, err := defaultkm.InitConfigProvider()
	assert.NoError(t, err)

	mockCrypto := cryptomock.NewRuntimeCryptoProviderMock(t)
	mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(
			ctx context.Context,
			_ *kmprovider.KeyRef,
			_ cryptolab.AlgorithmParams,
			content []byte) ([]byte, *cryptolab.CryptoDetails, error) {
			encrypted, encErr := cfgSvc.Encrypt(ctx, content)
			return encrypted, nil, encErr
		})
	mockCrypto.EXPECT().Decrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(
			ctx context.Context,
			_ *kmprovider.KeyRef,
			_ cryptolab.AlgorithmParams, content []byte) ([]byte, error) {
			return cfgSvc.Decrypt(ctx, content)
		})

	flowFactory, _ := core.Initialize(cache.Initialize())
	testGraph := flowFactory.CreateGraph("test-graph-id", common.FlowTypeAuthentication)

	originalToken := "original-secret-token-value-xyz789"

	engineCtx := EngineContext{
		ExecutionID: "round-trip-flow-id",
		AppID:       "round-trip-app-id",
		FlowType:    common.FlowTypeAuthentication,
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
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Graph:            testGraph,
	}

	svc := &flowExecService{cryptoSvc: mockCrypto}

	// Step 1: Encrypt (as storeContext / updateContext would)
	encryptedEngineCtx, err := svc.encryptEngineContext(context.Background(), &engineCtx)
	assert.NoError(t, err)
	assert.True(t, isContextEncrypted(encryptedEngineCtx.Context))

	// Step 2: Simulate getFlowContext decrypt path — call through the mock so RunAndReturn fires
	decryptedBytes, err := mockCrypto.Decrypt(
		context.Background(), nil,
		cryptolab.AlgorithmParams{Algorithm: cryptolab.AlgorithmAESGCM},
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
		ExecutionID: "existing-execution-id",
		Context:     `{"alg":"AES-GCM","ct":"not-valid-ciphertext!!!","kid":"key-1"}`,
	}
	mockStore.EXPECT().GetFlowContext(mock.Anything, "existing-execution-id").Return(invalidCtx, nil)

	service := &flowExecService{
		flowStore: mockStore,
		cryptoSvc: mockCrypto,
	}

	_, svcErr := service.Execute(context.Background(), "test-app", "existing-execution-id",
		string(common.FlowTypeAuthentication), false, "submit", map[string]string{}, "")

	assert.NotNil(t, svcErr)
	assert.Equal(t, serviceerror.InternalServerError.Code, svcErr.Code)
}

func TestExecute_ContextDecryptionSuccess(t *testing.T) {
	// Tests that a plain-text stored context (decryption already handled by service before store)
	// is loaded and used to continue flow execution without error.
	flowFactory, _ := core.Initialize(cache.Initialize())
	testGraph := flowFactory.CreateGraph("test-graph-id", common.FlowTypeAuthentication)

	engineCtx := EngineContext{
		ExecutionID: "existing-execution-id",
		AppID:       "test-app-id",
		FlowType:    common.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{
			Attributes: map[string]interface{}{},
		},
		UserInputs:       map[string]string{},
		RuntimeData:      map[string]string{},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Graph:            testGraph,
	}
	storedCtx, err := FromEngineContext(engineCtx)
	assert.NoError(t, err)

	mockStore := newFlowStoreInterfaceMock(t)
	mockFlowMgtSvc := flowmgtmock.NewFlowMgtServiceInterfaceMock(t)
	mockEngine := newFlowEngineInterfaceMock(t)
	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(t)
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)
	mockCrypto := cryptomock.NewRuntimeCryptoProviderMock(t)
	mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]byte("encrypted-ctx"), nil, nil)

	mockStore.EXPECT().GetFlowContext(mock.Anything, "existing-execution-id").Return(storedCtx, nil)
	mockFlowMgtSvc.EXPECT().GetGraph(mock.Anything, "test-graph-id").Return(testGraph, nil)
	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, "test-app-id").Return(
		&inboundmodel.InboundClient{ID: "test-app-id", AuthFlowID: "test-graph-id"}, nil)
	mockEntityProvider.EXPECT().GetEntity("test-app-id").Return(
		&entityprovider.Entity{ID: "test-app-id", Category: entityprovider.EntityCategoryApp},
		(*entityprovider.EntityProviderError)(nil))
	challengeToken := "test-challenge-token"
	mockEngine.EXPECT().Execute(mock.MatchedBy(func(ctx *EngineContext) bool {
		return ctx != nil && ctx.ChallengeTokenIn == challengeToken
	})).Return(FlowStep{Status: common.FlowStatusIncomplete}, nil)
	mockStore.EXPECT().UpdateFlowContext(
		mock.MatchedBy(func(ctx context.Context) bool { return ctx.Value(txMarkerKey{}) == "tx" }),
		mock.AnythingOfType("FlowContextDB")).Return(nil)

	service := &flowExecService{
		flowStore:            mockStore,
		flowMgtService:       mockFlowMgtSvc,
		flowEngine:           mockEngine,
		inboundClientService: mockInboundClient,
		entityProvider:       mockEntityProvider,
		transactioner:        &stubTransactioner{},
		cryptoSvc:            mockCrypto,
	}

	flowStep, svcErr := service.Execute(context.Background(), "test-app", "existing-execution-id",
		string(common.FlowTypeAuthentication), false, "submit", map[string]string{}, challengeToken)

	assert.Nil(t, svcErr)
	assert.NotNil(t, flowStep)
	assert.Equal(t, common.FlowStatusIncomplete, flowStep.Status)
}

func TestExecute_ExistingFlowWithoutChallengeToken(t *testing.T) {
	flowFactory, _ := core.Initialize(cache.Initialize())
	testGraph := flowFactory.CreateGraph("test-graph-id", common.FlowTypeAuthentication)

	engineCtx := EngineContext{
		ExecutionID: "existing-execution-id",
		AppID:       "test-app-id",
		FlowType:    common.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{
			Attributes: map[string]interface{}{},
		},
		UserInputs:       map[string]string{},
		RuntimeData:      map[string]string{},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Graph:            testGraph,
	}
	storedCtx, err := FromEngineContext(engineCtx)
	assert.NoError(t, err)

	mockStore := newFlowStoreInterfaceMock(t)
	mockFlowMgtSvc := flowmgtmock.NewFlowMgtServiceInterfaceMock(t)
	mockEngine := newFlowEngineInterfaceMock(t)
	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(t)
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)

	mockStore.EXPECT().GetFlowContext(mock.Anything, "existing-execution-id").Return(storedCtx, nil)
	mockFlowMgtSvc.EXPECT().GetGraph(mock.Anything, "test-graph-id").Return(testGraph, nil)
	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, "test-app-id").Return(
		&inboundmodel.InboundClient{ID: "test-app-id", AuthFlowID: "test-graph-id"}, nil)
	mockEntityProvider.EXPECT().GetEntity("test-app-id").Return(
		&entityprovider.Entity{ID: "test-app-id", Category: entityprovider.EntityCategoryApp},
		(*entityprovider.EntityProviderError)(nil))

	mockCrypto := cryptomock.NewRuntimeCryptoProviderMock(t)
	mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]byte("encrypted-ctx"), nil, nil)

	mockEngine.EXPECT().Execute(mock.MatchedBy(func(ctx *EngineContext) bool {
		return ctx != nil && ctx.ChallengeTokenIn == ""
	})).Return(FlowStep{Status: common.FlowStatusIncomplete}, nil)
	mockStore.EXPECT().UpdateFlowContext(
		mock.MatchedBy(func(ctx context.Context) bool { return ctx.Value(txMarkerKey{}) == "tx" }),
		mock.AnythingOfType("FlowContextDB")).Return(nil)

	service := &flowExecService{
		flowStore:            mockStore,
		flowMgtService:       mockFlowMgtSvc,
		flowEngine:           mockEngine,
		inboundClientService: mockInboundClient,
		entityProvider:       mockEntityProvider,
		transactioner:        &stubTransactioner{},
		cryptoSvc:            mockCrypto,
	}

	// Execute with empty challenge token
	flowStep, svcErr := service.Execute(context.Background(), "test-app", "existing-execution-id",
		string(common.FlowTypeAuthentication), false, "submit", map[string]string{}, "")

	assert.Nil(t, svcErr)
	assert.NotNil(t, flowStep)
	assert.Equal(t, common.FlowStatusIncomplete, flowStep.Status)
}

func TestExecute_ExistingFlowWithDifferentChallengeTokens(t *testing.T) {
	flowFactory, _ := core.Initialize(cache.Initialize())
	testGraph := flowFactory.CreateGraph("test-graph-id", common.FlowTypeAuthentication)

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
				ExecutionID: "existing-execution-id",
				AppID:       "test-app-id",
				FlowType:    common.FlowTypeAuthentication,
				AuthenticatedUser: authncm.AuthenticatedUser{
					Attributes: map[string]interface{}{},
				},
				UserInputs:       map[string]string{},
				RuntimeData:      map[string]string{},
				ExecutionHistory: map[string]*common.NodeExecutionRecord{},
				Graph:            testGraph,
			}
			storedCtx, err := FromEngineContext(engineCtx)
			assert.NoError(t, err)

			mockStore := newFlowStoreInterfaceMock(t)
			mockFlowMgtSvc := flowmgtmock.NewFlowMgtServiceInterfaceMock(t)
			mockEngine := newFlowEngineInterfaceMock(t)
			mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(t)
			mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)

			mockStore.EXPECT().GetFlowContext(mock.Anything, "existing-execution-id").Return(storedCtx, nil)
			mockFlowMgtSvc.EXPECT().GetGraph(mock.Anything, "test-graph-id").Return(testGraph, nil)
			mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, "test-app-id").Return(
				&inboundmodel.InboundClient{ID: "test-app-id", AuthFlowID: "test-graph-id"}, nil)
			mockEntityProvider.EXPECT().GetEntity("test-app-id").Return(
				&entityprovider.Entity{ID: "test-app-id", Category: entityprovider.EntityCategoryApp},
				(*entityprovider.EntityProviderError)(nil))

			expectedToken := tt.expectInContext
			mockCrypto := cryptomock.NewRuntimeCryptoProviderMock(t)
			mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return([]byte("encrypted-ctx"), nil, nil)

			mockEngine.EXPECT().Execute(mock.MatchedBy(func(ctx *EngineContext) bool {
				return ctx != nil && ctx.ChallengeTokenIn == expectedToken
			})).Return(FlowStep{Status: common.FlowStatusIncomplete}, nil)
			mockStore.EXPECT().UpdateFlowContext(
				mock.MatchedBy(func(ctx context.Context) bool { return ctx.Value(txMarkerKey{}) == "tx" }),
				mock.AnythingOfType("FlowContextDB")).Return(nil)

			service := &flowExecService{
				flowStore:            mockStore,
				flowMgtService:       mockFlowMgtSvc,
				flowEngine:           mockEngine,
				inboundClientService: mockInboundClient,
				entityProvider:       mockEntityProvider,
				transactioner:        &stubTransactioner{},
				cryptoSvc:            mockCrypto,
			}

			flowStep, svcErr := service.Execute(context.Background(), "test-app", "existing-execution-id",
				string(common.FlowTypeAuthentication), false, "submit", map[string]string{}, tt.challengeToken)

			assert.Nil(t, svcErr)
			assert.NotNil(t, flowStep)
			assert.Equal(t, common.FlowStatusIncomplete, flowStep.Status)
		})
	}
}

func TestExecute_EngineError_InvalidChallengeToken_PreservesContext(t *testing.T) {
	flowFactory, _ := core.Initialize(cache.Initialize())
	testGraph := flowFactory.CreateGraph("test-graph-id", common.FlowTypeAuthentication)

	engineCtx := EngineContext{
		ExecutionID: "existing-execution-id",
		AppID:       "test-app-id",
		FlowType:    common.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{
			Attributes: map[string]interface{}{},
		},
		UserInputs:       map[string]string{},
		RuntimeData:      map[string]string{},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Graph:            testGraph,
	}
	storedCtx, err := FromEngineContext(engineCtx)
	assert.NoError(t, err)

	mockStore := newFlowStoreInterfaceMock(t)
	mockFlowMgtSvc := flowmgtmock.NewFlowMgtServiceInterfaceMock(t)
	mockEngine := newFlowEngineInterfaceMock(t)
	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(t)
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)

	mockStore.EXPECT().GetFlowContext(mock.Anything, "existing-execution-id").Return(storedCtx, nil)
	mockFlowMgtSvc.EXPECT().GetGraph(mock.Anything, "test-graph-id").Return(testGraph, nil)
	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, "test-app-id").Return(
		&inboundmodel.InboundClient{ID: "test-app-id", AuthFlowID: "test-graph-id"}, nil)
	mockEntityProvider.EXPECT().GetEntity("test-app-id").Return(
		&entityprovider.Entity{ID: "test-app-id", Category: entityprovider.EntityCategoryApp},
		(*entityprovider.EntityProviderError)(nil))

	// Engine returns invalid challenge token error
	mockEngine.EXPECT().Execute(mock.Anything).Return(FlowStep{}, &ErrorInvalidChallengeToken)
	// DeleteFlowContext must NOT be called — flow must be preserved for retry

	service := &flowExecService{
		flowStore:            mockStore,
		flowMgtService:       mockFlowMgtSvc,
		flowEngine:           mockEngine,
		inboundClientService: mockInboundClient,
		entityProvider:       mockEntityProvider,
		transactioner:        &stubTransactioner{},
	}

	flowStep, svcErr := service.Execute(context.Background(), "test-app", "existing-execution-id",
		string(common.FlowTypeAuthentication), false, "submit", map[string]string{}, "wrong-token")

	assert.NotNil(t, svcErr)
	assert.Equal(t, ErrorInvalidChallengeToken.Code, svcErr.Code)
	assert.Nil(t, flowStep)
}

func TestExecute_EngineError_NonChallengeToken_RemovesContext(t *testing.T) {
	flowFactory, _ := core.Initialize(cache.Initialize())
	testGraph := flowFactory.CreateGraph("test-graph-id", common.FlowTypeAuthentication)

	engineCtx := EngineContext{
		ExecutionID: "existing-execution-id",
		AppID:       "test-app-id",
		FlowType:    common.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{
			Attributes: map[string]interface{}{},
		},
		UserInputs:       map[string]string{},
		RuntimeData:      map[string]string{},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Graph:            testGraph,
	}
	storedCtx, err := FromEngineContext(engineCtx)
	assert.NoError(t, err)

	mockStore := newFlowStoreInterfaceMock(t)
	mockFlowMgtSvc := flowmgtmock.NewFlowMgtServiceInterfaceMock(t)
	mockEngine := newFlowEngineInterfaceMock(t)
	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(t)
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)

	mockStore.EXPECT().GetFlowContext(mock.Anything, "existing-execution-id").Return(storedCtx, nil)
	mockFlowMgtSvc.EXPECT().GetGraph(mock.Anything, "test-graph-id").Return(testGraph, nil)
	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, "test-app-id").Return(
		&inboundmodel.InboundClient{ID: "test-app-id", AuthFlowID: "test-graph-id"}, nil)
	mockEntityProvider.EXPECT().GetEntity("test-app-id").Return(
		&entityprovider.Entity{ID: "test-app-id", Category: entityprovider.EntityCategoryApp},
		(*entityprovider.EntityProviderError)(nil))

	otherErr := &serviceerror.ServiceError{
		Code: "FES-9999",
		Type: serviceerror.ServerErrorType,
		Error: i18ncore.I18nMessage{
			Key:          "error.flowexecservice.engine_error",
			DefaultValue: "some other engine error",
		},
		ErrorDescription: i18ncore.I18nMessage{
			Key:          "error.flowexecservice.engine_error_description",
			DefaultValue: "some other engine error",
		},
	}
	mockEngine.EXPECT().Execute(mock.Anything).Return(FlowStep{}, otherErr)
	// DeleteFlowContext MUST be called — non-challenge-token errors remove the context
	mockStore.EXPECT().DeleteFlowContext(
		mock.MatchedBy(func(ctx context.Context) bool { return ctx.Value(txMarkerKey{}) == "tx" }),
		"existing-execution-id").Return(nil)

	service := &flowExecService{
		flowStore:            mockStore,
		flowMgtService:       mockFlowMgtSvc,
		flowEngine:           mockEngine,
		inboundClientService: mockInboundClient,
		entityProvider:       mockEntityProvider,
		transactioner:        &stubTransactioner{},
	}

	flowStep, svcErr := service.Execute(context.Background(), "test-app", "existing-execution-id",
		string(common.FlowTypeAuthentication), false, "submit", map[string]string{}, "valid-token")

	assert.NotNil(t, svcErr)
	assert.Equal(t, otherErr.Code, svcErr.Code)
	assert.Nil(t, flowStep)
}

func TestExecute_EngineError_NewFlow_ContextNeverRemoved(t *testing.T) {
	testConfig := &config.Config{}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)

	flowFactory, _ := core.Initialize(cache.Initialize())
	testGraph := flowFactory.CreateGraph("auth-graph-1", common.FlowTypeAuthentication)

	mockStore := newFlowStoreInterfaceMock(t)
	mockFlowMgtSvc := flowmgtmock.NewFlowMgtServiceInterfaceMock(t)
	mockEngine := newFlowEngineInterfaceMock(t)
	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(t)
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)

	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, "test-app").Return(
		&inboundmodel.InboundClient{ID: "test-app", AuthFlowID: "auth-graph-1"}, nil).Times(2)
	mockEntityProvider.EXPECT().GetEntity("test-app").Return(
		&entityprovider.Entity{ID: "test-app", Category: entityprovider.EntityCategoryApp},
		(*entityprovider.EntityProviderError)(nil))
	mockFlowMgtSvc.EXPECT().GetGraph(mock.Anything, "auth-graph-1").Return(testGraph, nil)
	mockEngine.EXPECT().Execute(mock.Anything).Return(FlowStep{}, &ErrorInvalidChallengeToken)
	// DeleteFlowContext must NOT be called — new flows have no persisted context to clean up

	service := &flowExecService{
		flowStore:            mockStore,
		flowMgtService:       mockFlowMgtSvc,
		flowEngine:           mockEngine,
		inboundClientService: mockInboundClient,
		entityProvider:       mockEntityProvider,
		transactioner:        &stubTransactioner{},
	}

	// Pass empty executionID to indicate a new flow
	flowStep, svcErr := service.Execute(context.Background(), "test-app", "",
		string(common.FlowTypeAuthentication), false, "submit", map[string]string{}, "")

	assert.NotNil(t, svcErr)
	assert.Equal(t, ErrorInvalidChallengeToken.Code, svcErr.Code)
	assert.Nil(t, flowStep)
}

// --- buildFlowApplication / readEntitySystemAttributes ---

func newBuildAppService(
	t *testing.T,
) (*flowExecService, *inboundclientmock.InboundClientServiceInterfaceMock,
	*entityprovidermock.EntityProviderInterfaceMock) {
	mockInbound := inboundclientmock.NewInboundClientServiceInterfaceMock(t)
	mockEP := entityprovidermock.NewEntityProviderInterfaceMock(t)
	return &flowExecService{
		inboundClientService: mockInbound,
		entityProvider:       mockEP,
	}, mockInbound, mockEP
}

func TestBuildFlowApplication_InboundClientNotFound(t *testing.T) {
	svc, mockInbound, _ := newBuildAppService(t)
	mockInbound.EXPECT().GetInboundClientByEntityID(mock.Anything, "app-x").
		Return((*inboundmodel.InboundClient)(nil), inboundclient.ErrInboundClientNotFound)

	app, svcErr := svc.buildFlowApplication(context.Background(), "app-x", log.GetLogger())

	assert.Nil(t, app)
	assert.Equal(t, ErrorInvalidAppID.Code, svcErr.Code)
}

func TestBuildFlowApplication_InboundClientStoreError(t *testing.T) {
	svc, mockInbound, _ := newBuildAppService(t)
	mockInbound.EXPECT().GetInboundClientByEntityID(mock.Anything, "app-x").
		Return((*inboundmodel.InboundClient)(nil), errors.New("boom"))

	app, svcErr := svc.buildFlowApplication(context.Background(), "app-x", log.GetLogger())

	assert.Nil(t, app)
	assert.Equal(t, serviceerror.InternalServerError.Code, svcErr.Code)
}

func TestBuildFlowApplication_EntityLoadError(t *testing.T) {
	svc, mockInbound, mockEP := newBuildAppService(t)
	mockInbound.EXPECT().GetInboundClientByEntityID(mock.Anything, "app-x").
		Return(&inboundmodel.InboundClient{ID: "app-x"}, nil)
	mockEP.EXPECT().GetEntity("app-x").Return(
		(*entityprovider.Entity)(nil),
		entityprovider.NewEntityProviderError("INTERNAL_ERROR", "boom", ""))

	app, svcErr := svc.buildFlowApplication(context.Background(), "app-x", log.GetLogger())

	assert.Nil(t, app)
	assert.Equal(t, serviceerror.InternalServerError.Code, svcErr.Code)
}

func TestBuildFlowApplication_EntityNotFound_ReturnsAppWithoutEntityFields(t *testing.T) {
	svc, mockInbound, mockEP := newBuildAppService(t)
	mockInbound.EXPECT().GetInboundClientByEntityID(mock.Anything, "app-x").
		Return(&inboundmodel.InboundClient{
			ID:               "app-x",
			AllowedUserTypes: []string{"customer"},
		}, nil)
	mockEP.EXPECT().GetEntity("app-x").Return(
		(*entityprovider.Entity)(nil),
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "missing", ""))

	app, svcErr := svc.buildFlowApplication(context.Background(), "app-x", log.GetLogger())

	assert.Nil(t, svcErr)
	assert.NotNil(t, app)
	assert.Equal(t, "app-x", app.ID)
	assert.Equal(t, "", app.Name)
	assert.Equal(t, []string{"customer"}, app.AllowedUserTypes)
	assert.Empty(t, app.InboundAuthConfig)
}

func TestBuildFlowApplication_Success_WithMetadataAndClientID(t *testing.T) {
	svc, mockInbound, mockEP := newBuildAppService(t)
	mockInbound.EXPECT().GetInboundClientByEntityID(mock.Anything, "app-x").
		Return(&inboundmodel.InboundClient{
			ID: "app-x",
			Properties: map[string]interface{}{
				"metadata": map[string]interface{}{"tier": "gold"},
			},
		}, nil)
	sysAttrs := []byte(`{"name":"Acme","clientId":"client-1"}`)
	mockEP.EXPECT().GetEntity("app-x").Return(
		&entityprovider.Entity{
			ID:               "app-x",
			Category:         entityprovider.EntityCategoryApp,
			SystemAttributes: sysAttrs,
		},
		(*entityprovider.EntityProviderError)(nil))

	app, svcErr := svc.buildFlowApplication(context.Background(), "app-x", log.GetLogger())

	assert.Nil(t, svcErr)
	assert.NotNil(t, app)
	assert.Equal(t, "Acme", app.Name)
	assert.Equal(t, map[string]interface{}{"tier": "gold"}, app.Metadata)
	assert.Len(t, app.InboundAuthConfig, 1)
	assert.Equal(t, inboundmodel.OAuthInboundAuthType, app.InboundAuthConfig[0].Type)
	assert.Equal(t, "client-1", app.InboundAuthConfig[0].OAuthConfig.ClientID)
}

func TestReadEntitySystemAttributes_NilEntity(t *testing.T) {
	assert.Empty(t, readEntitySystemAttributes(nil))
}

func TestReadEntitySystemAttributes_EmptyBlob(t *testing.T) {
	assert.Empty(t, readEntitySystemAttributes(&entityprovider.Entity{}))
}

func TestReadEntitySystemAttributes_InvalidJSON(t *testing.T) {
	e := &entityprovider.Entity{SystemAttributes: []byte("not-json")}
	assert.Empty(t, readEntitySystemAttributes(e))
}

func TestReadEntitySystemAttributes_Valid(t *testing.T) {
	e := &entityprovider.Entity{SystemAttributes: []byte(`{"name":"X"}`)}
	assert.Equal(t, map[string]interface{}{"name": "X"}, readEntitySystemAttributes(e))
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
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
	}

	svc := &flowExecService{}
	_, err := svc.encryptEngineContext(context.Background(), engineCtx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to serialize engine context")
}

func TestEncryptEngineContext_EncryptError(t *testing.T) {
	// Triggers line 483: serialization succeeds but cryptoSvc.Encrypt returns an error,
	// wrapping it with "failed to encrypt context".
	testConfig := &config.Config{}
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)

	flowFactory, _ := core.Initialize(cache.Initialize())
	testGraph := flowFactory.CreateGraph("test-graph-id", common.FlowTypeAuthentication)

	engineCtx := &EngineContext{
		ExecutionID: "exec-id",
		AppID:       "app-id",
		FlowType:    common.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{
			Attributes: map[string]interface{}{},
		},
		UserInputs:       map[string]string{},
		RuntimeData:      map[string]string{},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Graph:            testGraph,
	}

	mockCrypto := cryptomock.NewRuntimeCryptoProviderMock(t)
	mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil, errors.New("encryption backend unavailable"))

	svc := &flowExecService{cryptoSvc: mockCrypto}
	_, err := svc.encryptEngineContext(context.Background(), engineCtx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to encrypt context")
}
