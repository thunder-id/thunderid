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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	appmodel "github.com/asgardeo/thunder/internal/application/model"
	"github.com/asgardeo/thunder/internal/flow/common"
	"github.com/asgardeo/thunder/internal/flow/core"
	flowmgt "github.com/asgardeo/thunder/internal/flow/mgt"
	"github.com/asgardeo/thunder/internal/system/config"
	"github.com/asgardeo/thunder/internal/system/error/serviceerror"
	i18ncore "github.com/asgardeo/thunder/internal/system/i18n/core"
	"github.com/asgardeo/thunder/tests/mocks/applicationmock"
	"github.com/asgardeo/thunder/tests/mocks/flow/flowmgtmock"
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

	flowFactory, _ := core.Initialize()
	testGraph := flowFactory.CreateGraph("auth-graph-1", common.FlowTypeAuthentication)

	// Mock application and graph - shared across all test cases
	mockApp := &appmodel.Application{
		ID:         "app-id-123",
		AuthFlowID: "auth-graph-1",
	}

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
			mockAppService := applicationmock.NewApplicationServiceInterfaceMock(t)
			mockFlowMgtSvc := flowmgtmock.NewFlowMgtServiceInterfaceMock(t)

			// Create service with mocked dependencies
			service := &flowExecService{
				flowMgtService: mockFlowMgtSvc,
				flowStore:      mockStore,
				appService:     mockAppService,
				flowEngine:     nil,
				transactioner:  &stubTransactioner{},
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

				// For system flows, StoreFlowContext is called with empty AppID
				mockStore.EXPECT().StoreFlowContext(mock.MatchedBy(func(ctx context.Context) bool {
					return ctx.Value(txMarkerKey{}) == "tx"
				}), mock.MatchedBy(func(ctx EngineContext) bool {
					// Verify executionID is generated
					if ctx.ExecutionID == "" {
						return false
					}
					// Verify runtime data according to test case expectation
					if !tt.expectedRuntimeDataCheck(ctx) {
						return false
					}
					// Verify AppID is empty for system flow
					if ctx.AppID != "" {
						return false
					}
					if ctx.FlowType != common.FlowTypeUserOnboarding {
						return false
					}
					return true
				}), mock.Anything).Return(nil)
			} else {
				mockAppService.EXPECT().GetApplication(mock.Anything, appID).Return(mockApp, nil)
				mockFlowMgtSvc.EXPECT().GetGraph(mock.Anything, "auth-graph-1").Return(testGraph, nil)
				mockStore.EXPECT().StoreFlowContext(mock.MatchedBy(func(ctx context.Context) bool {
					return ctx.Value(txMarkerKey{}) == "tx"
				}), mock.MatchedBy(func(ctx EngineContext) bool {
					// Verify executionID is generated
					if ctx.ExecutionID == "" {
						return false
					}
					// Verify runtime data according to test case expectation
					if !tt.expectedRuntimeDataCheck(ctx) {
						return false
					}
					// Verify AppID and FlowType
					if ctx.AppID != appID {
						return false
					}
					if ctx.FlowType != common.FlowTypeAuthentication {
						return false
					}
					return true
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

	flowFactory, _ := core.Initialize()

	tests := []struct {
		name       string
		setupMocks func(
			*flowStoreInterfaceMock,
			*applicationmock.ApplicationServiceInterfaceMock,
			*flowmgtmock.FlowMgtServiceInterfaceMock,
		)
		expectedErrorCode        string
		expectedErrorDescription string
	}{
		{
			name: "error from getApplication - application not found",
			setupMocks: func(
				mockStore *flowStoreInterfaceMock,
				mockAppService *applicationmock.ApplicationServiceInterfaceMock,
				mockFlowMgtSvc *flowmgtmock.FlowMgtServiceInterfaceMock,
			) {
				// Import application package for its error constants
				appNotFoundErr := &serviceerror.ServiceError{
					Type:  serviceerror.ClientErrorType,
					Code:  "APP-1001", // ErrorApplicationNotFound.Code
					Error: i18ncore.I18nMessage{DefaultValue: "Application not found"},
					ErrorDescription: i18ncore.I18nMessage{
						DefaultValue: "The requested application could not be found",
					},
				}
				mockAppService.EXPECT().GetApplication(mock.Anything, appID).Return(nil, appNotFoundErr)
				// No other mocks needed as it fails early
			},
			expectedErrorCode: "FES-1003", // ErrorInvalidAppID (converted from application not found)
		},
		{
			name: "error from getApplication - other client error",
			setupMocks: func(
				mockStore *flowStoreInterfaceMock,
				mockAppService *applicationmock.ApplicationServiceInterfaceMock,
				mockFlowMgtSvc *flowmgtmock.FlowMgtServiceInterfaceMock,
			) {
				// Mock application service to return a different client error
				mockAppService.EXPECT().GetApplication(mock.Anything, appID).
					Return(nil, &ErrorApplicationRetrievalClientError)
				// No other mocks needed as it fails early
			},
			expectedErrorCode: "FES-1007", // ErrorApplicationRetrievalClientError
		},
		{
			name: "error from flowMgtService.GetGraph - graph not found",
			setupMocks: func(
				mockStore *flowStoreInterfaceMock,
				mockAppService *applicationmock.ApplicationServiceInterfaceMock,
				mockFlowMgtSvc *flowmgtmock.FlowMgtServiceInterfaceMock,
			) {
				// Mock application service to return valid app
				mockApp := &appmodel.Application{
					ID:         "app-id-123",
					AuthFlowID: "auth-graph-1",
				}
				mockAppService.EXPECT().GetApplication(mock.Anything, appID).Return(mockApp, nil)

				// Mock flow management service to return error (graph not found)
				mockFlowMgtSvc.EXPECT().GetGraph(mock.Anything, "auth-graph-1").
					Return(nil, &serviceerror.InternalServerError)
				// No store mock needed as it fails before storing
			},
			expectedErrorCode: serviceerror.InternalServerError.Code,
		},
		{
			name: "error from storeContext - store failure",
			setupMocks: func(
				mockStore *flowStoreInterfaceMock,
				mockAppService *applicationmock.ApplicationServiceInterfaceMock,
				mockFlowMgtSvc *flowmgtmock.FlowMgtServiceInterfaceMock,
			) {
				// Mock application service to return valid app
				mockApp := &appmodel.Application{
					ID:         "app-id-123",
					AuthFlowID: "auth-graph-1",
				}
				mockAppService.EXPECT().GetApplication(mock.Anything, appID).Return(mockApp, nil)

				// Mock flow management service to return valid graph
				testGraph := flowFactory.CreateGraph("auth-graph-1", common.FlowTypeAuthentication)
				mockFlowMgtSvc.EXPECT().GetGraph(mock.Anything, "auth-graph-1").Return(testGraph, nil)

				// Mock store to return error
				mockStore.EXPECT().StoreFlowContext(
					mock.MatchedBy(func(ctx context.Context) bool {
						return ctx.Value(txMarkerKey{}) == "tx"
					}),
					mock.AnythingOfType("EngineContext"), mock.Anything).Return(assert.AnError)
			},
			expectedErrorCode: serviceerror.InternalServerError.Code,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockStore := newFlowStoreInterfaceMock(t)
			mockAppService := applicationmock.NewApplicationServiceInterfaceMock(t)
			mockFlowMgtSvc := flowmgtmock.NewFlowMgtServiceInterfaceMock(t)

			// Create service with mocked dependencies
			service := &flowExecService{
				flowMgtService: mockFlowMgtSvc,
				flowStore:      mockStore,
				appService:     mockAppService,
				flowEngine:     nil,
				transactioner:  &stubTransactioner{},
			}

			initContext := &FlowInitContext{
				ApplicationID: appID,
				FlowType:      "AUTHENTICATION",
				RuntimeData: map[string]string{
					"test": "data",
				},
			}

			// Setup test-specific mocks
			tt.setupMocks(mockStore, mockAppService, mockFlowMgtSvc)

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

func TestExecute_ContextDecryptionFailure(t *testing.T) {
	// Tests that when the stored flow context cannot be decrypted,
	// Execute returns an InternalServerError without proceeding further.
	testConfig := &config.Config{
		Crypto: config.CryptoConfig{
			Encryption: config.EncryptionConfig{
				Key: "2729a7928c79371e5f312167269294a14bb0660fd166b02a408a20fa73271580",
			},
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)

	mockStore := newFlowStoreInterfaceMock(t)

	// Context looks encrypted (has "alg" field) but the ciphertext is invalid
	invalidCtx := &FlowContextDB{
		ExecutionID: "existing-execution-id",
		Context:     `{"alg":"AES-GCM","ct":"not-valid-ciphertext!!!","kid":"key-1"}`,
	}
	mockStore.EXPECT().GetFlowContext(mock.Anything, "existing-execution-id").Return(invalidCtx, nil)

	service := &flowExecService{
		flowStore: mockStore,
	}

	_, svcErr := service.Execute(context.Background(), "test-app", "existing-execution-id",
		string(common.FlowTypeAuthentication), false, "submit", map[string]string{}, "")

	assert.NotNil(t, svcErr)
	assert.Equal(t, serviceerror.InternalServerError.Code, svcErr.Code)
}

func TestExecute_ContextDecryptionSuccess(t *testing.T) {
	// Tests that a properly encrypted stored context is decrypted and used
	// to continue flow execution without error.
	testConfig := &config.Config{
		Crypto: config.CryptoConfig{
			Encryption: config.EncryptionConfig{
				Key: "2729a7928c79371e5f312167269294a14bb0660fd166b02a408a20fa73271580",
			},
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)

	flowFactory, _ := core.Initialize()
	testGraph := flowFactory.CreateGraph("test-graph-id", common.FlowTypeAuthentication)

	// Build a properly encrypted FlowContextDB
	engineCtx := EngineContext{
		ExecutionID:      "existing-execution-id",
		AppID:            "test-app-id",
		FlowType:         common.FlowTypeAuthentication,
		UserInputs:       map[string]string{},
		RuntimeData:      map[string]string{},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Graph:            testGraph,
	}
	dbModel, err := FromEngineContext(engineCtx)
	assert.NoError(t, err)
	assert.Contains(t, dbModel.Context, `"ct"`, "context should be encrypted before retrieval")

	mockStore := newFlowStoreInterfaceMock(t)
	mockFlowMgtSvc := flowmgtmock.NewFlowMgtServiceInterfaceMock(t)
	mockEngine := newFlowEngineInterfaceMock(t)
	mockAppService := applicationmock.NewApplicationServiceInterfaceMock(t)

	mockStore.EXPECT().GetFlowContext(mock.Anything, "existing-execution-id").Return(dbModel, nil)
	mockFlowMgtSvc.EXPECT().GetGraph(mock.Anything, "test-graph-id").Return(testGraph, nil)
	mockAppService.EXPECT().GetApplication(mock.Anything, "test-app-id").Return(
		&appmodel.Application{ID: "test-app-id", AuthFlowID: "test-graph-id"}, nil)
	challengeToken := "test-challenge-token"
	mockEngine.EXPECT().Execute(mock.MatchedBy(func(ctx *EngineContext) bool {
		return ctx != nil && ctx.ChallengeTokenIn == challengeToken
	})).Return(FlowStep{Status: common.FlowStatusIncomplete}, nil)
	mockStore.EXPECT().UpdateFlowContext(
		mock.MatchedBy(func(ctx context.Context) bool { return ctx.Value(txMarkerKey{}) == "tx" }),
		mock.AnythingOfType("EngineContext")).Return(nil)

	service := &flowExecService{
		flowStore:      mockStore,
		flowMgtService: mockFlowMgtSvc,
		flowEngine:     mockEngine,
		appService:     mockAppService,
		transactioner:  &stubTransactioner{},
	}

	flowStep, svcErr := service.Execute(context.Background(), "test-app", "existing-execution-id",
		string(common.FlowTypeAuthentication), false, "submit", map[string]string{}, challengeToken)

	assert.Nil(t, svcErr)
	assert.NotNil(t, flowStep)
	assert.Equal(t, common.FlowStatusIncomplete, flowStep.Status)
}

func TestExecute_ExistingFlowWithoutChallengeToken(t *testing.T) {
	testConfig := &config.Config{
		Crypto: config.CryptoConfig{
			Encryption: config.EncryptionConfig{
				Key: "2729a7928c79371e5f312167269294a14bb0660fd166b02a408a20fa73271580",
			},
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)

	flowFactory, _ := core.Initialize()
	testGraph := flowFactory.CreateGraph("test-graph-id", common.FlowTypeAuthentication)

	engineCtx := EngineContext{
		ExecutionID:      "existing-execution-id",
		AppID:            "test-app-id",
		FlowType:         common.FlowTypeAuthentication,
		UserInputs:       map[string]string{},
		RuntimeData:      map[string]string{},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Graph:            testGraph,
	}
	dbModel, err := FromEngineContext(engineCtx)
	assert.NoError(t, err)

	mockStore := newFlowStoreInterfaceMock(t)
	mockFlowMgtSvc := flowmgtmock.NewFlowMgtServiceInterfaceMock(t)
	mockEngine := newFlowEngineInterfaceMock(t)
	mockAppService := applicationmock.NewApplicationServiceInterfaceMock(t)

	mockStore.EXPECT().GetFlowContext(mock.Anything, "existing-execution-id").Return(dbModel, nil)
	mockFlowMgtSvc.EXPECT().GetGraph(mock.Anything, "test-graph-id").Return(testGraph, nil)
	mockAppService.EXPECT().GetApplication(mock.Anything, "test-app-id").Return(
		&appmodel.Application{ID: "test-app-id", AuthFlowID: "test-graph-id"}, nil)

	mockEngine.EXPECT().Execute(mock.MatchedBy(func(ctx *EngineContext) bool {
		return ctx != nil && ctx.ChallengeTokenIn == ""
	})).Return(FlowStep{Status: common.FlowStatusIncomplete}, nil)
	mockStore.EXPECT().UpdateFlowContext(
		mock.MatchedBy(func(ctx context.Context) bool { return ctx.Value(txMarkerKey{}) == "tx" }),
		mock.AnythingOfType("EngineContext")).Return(nil)

	service := &flowExecService{
		flowStore:      mockStore,
		flowMgtService: mockFlowMgtSvc,
		flowEngine:     mockEngine,
		appService:     mockAppService,
		transactioner:  &stubTransactioner{},
	}

	// Execute with empty challenge token
	flowStep, svcErr := service.Execute(context.Background(), "test-app", "existing-execution-id",
		string(common.FlowTypeAuthentication), false, "submit", map[string]string{}, "")

	assert.Nil(t, svcErr)
	assert.NotNil(t, flowStep)
	assert.Equal(t, common.FlowStatusIncomplete, flowStep.Status)
}

func TestExecute_ExistingFlowWithDifferentChallengeTokens(t *testing.T) {
	testConfig := &config.Config{
		Crypto: config.CryptoConfig{
			Encryption: config.EncryptionConfig{
				Key: "2729a7928c79371e5f312167269294a14bb0660fd166b02a408a20fa73271580",
			},
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)

	flowFactory, _ := core.Initialize()
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
				ExecutionID:      "existing-execution-id",
				AppID:            "test-app-id",
				FlowType:         common.FlowTypeAuthentication,
				UserInputs:       map[string]string{},
				RuntimeData:      map[string]string{},
				ExecutionHistory: map[string]*common.NodeExecutionRecord{},
				Graph:            testGraph,
			}
			dbModel, err := FromEngineContext(engineCtx)
			assert.NoError(t, err)

			mockStore := newFlowStoreInterfaceMock(t)
			mockFlowMgtSvc := flowmgtmock.NewFlowMgtServiceInterfaceMock(t)
			mockEngine := newFlowEngineInterfaceMock(t)
			mockAppService := applicationmock.NewApplicationServiceInterfaceMock(t)

			mockStore.EXPECT().GetFlowContext(mock.Anything, "existing-execution-id").Return(dbModel, nil)
			mockFlowMgtSvc.EXPECT().GetGraph(mock.Anything, "test-graph-id").Return(testGraph, nil)
			mockAppService.EXPECT().GetApplication(mock.Anything, "test-app-id").Return(
				&appmodel.Application{ID: "test-app-id", AuthFlowID: "test-graph-id"}, nil)

			expectedToken := tt.expectInContext
			mockEngine.EXPECT().Execute(mock.MatchedBy(func(ctx *EngineContext) bool {
				return ctx != nil && ctx.ChallengeTokenIn == expectedToken
			})).Return(FlowStep{Status: common.FlowStatusIncomplete}, nil)
			mockStore.EXPECT().UpdateFlowContext(
				mock.MatchedBy(func(ctx context.Context) bool { return ctx.Value(txMarkerKey{}) == "tx" }),
				mock.AnythingOfType("EngineContext")).Return(nil)

			service := &flowExecService{
				flowStore:      mockStore,
				flowMgtService: mockFlowMgtSvc,
				flowEngine:     mockEngine,
				appService:     mockAppService,
				transactioner:  &stubTransactioner{},
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
	testConfig := &config.Config{
		Crypto: config.CryptoConfig{
			Encryption: config.EncryptionConfig{
				Key: "2729a7928c79371e5f312167269294a14bb0660fd166b02a408a20fa73271580",
			},
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)

	flowFactory, _ := core.Initialize()
	testGraph := flowFactory.CreateGraph("test-graph-id", common.FlowTypeAuthentication)

	engineCtx := EngineContext{
		ExecutionID:      "existing-execution-id",
		AppID:            "test-app-id",
		FlowType:         common.FlowTypeAuthentication,
		UserInputs:       map[string]string{},
		RuntimeData:      map[string]string{},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Graph:            testGraph,
	}
	dbModel, err := FromEngineContext(engineCtx)
	assert.NoError(t, err)

	mockStore := newFlowStoreInterfaceMock(t)
	mockFlowMgtSvc := flowmgtmock.NewFlowMgtServiceInterfaceMock(t)
	mockEngine := newFlowEngineInterfaceMock(t)
	mockAppService := applicationmock.NewApplicationServiceInterfaceMock(t)

	mockStore.EXPECT().GetFlowContext(mock.Anything, "existing-execution-id").Return(dbModel, nil)
	mockFlowMgtSvc.EXPECT().GetGraph(mock.Anything, "test-graph-id").Return(testGraph, nil)
	mockAppService.EXPECT().GetApplication(mock.Anything, "test-app-id").Return(
		&appmodel.Application{ID: "test-app-id", AuthFlowID: "test-graph-id"}, nil)

	// Engine returns invalid challenge token error
	mockEngine.EXPECT().Execute(mock.Anything).Return(FlowStep{}, &ErrorInvalidChallengeToken)
	// DeleteFlowContext must NOT be called — flow must be preserved for retry

	service := &flowExecService{
		flowStore:      mockStore,
		flowMgtService: mockFlowMgtSvc,
		flowEngine:     mockEngine,
		appService:     mockAppService,
		transactioner:  &stubTransactioner{},
	}

	flowStep, svcErr := service.Execute(context.Background(), "test-app", "existing-execution-id",
		string(common.FlowTypeAuthentication), false, "submit", map[string]string{}, "wrong-token")

	assert.NotNil(t, svcErr)
	assert.Equal(t, ErrorInvalidChallengeToken.Code, svcErr.Code)
	assert.Nil(t, flowStep)
}

func TestExecute_EngineError_NonChallengeToken_RemovesContext(t *testing.T) {
	testConfig := &config.Config{
		Crypto: config.CryptoConfig{
			Encryption: config.EncryptionConfig{
				Key: "2729a7928c79371e5f312167269294a14bb0660fd166b02a408a20fa73271580",
			},
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)

	flowFactory, _ := core.Initialize()
	testGraph := flowFactory.CreateGraph("test-graph-id", common.FlowTypeAuthentication)

	engineCtx := EngineContext{
		ExecutionID:      "existing-execution-id",
		AppID:            "test-app-id",
		FlowType:         common.FlowTypeAuthentication,
		UserInputs:       map[string]string{},
		RuntimeData:      map[string]string{},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Graph:            testGraph,
	}
	dbModel, err := FromEngineContext(engineCtx)
	assert.NoError(t, err)

	mockStore := newFlowStoreInterfaceMock(t)
	mockFlowMgtSvc := flowmgtmock.NewFlowMgtServiceInterfaceMock(t)
	mockEngine := newFlowEngineInterfaceMock(t)
	mockAppService := applicationmock.NewApplicationServiceInterfaceMock(t)

	mockStore.EXPECT().GetFlowContext(mock.Anything, "existing-execution-id").Return(dbModel, nil)
	mockFlowMgtSvc.EXPECT().GetGraph(mock.Anything, "test-graph-id").Return(testGraph, nil)
	mockAppService.EXPECT().GetApplication(mock.Anything, "test-app-id").Return(
		&appmodel.Application{ID: "test-app-id", AuthFlowID: "test-graph-id"}, nil)

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
		flowStore:      mockStore,
		flowMgtService: mockFlowMgtSvc,
		flowEngine:     mockEngine,
		appService:     mockAppService,
		transactioner:  &stubTransactioner{},
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

	flowFactory, _ := core.Initialize()
	testGraph := flowFactory.CreateGraph("auth-graph-1", common.FlowTypeAuthentication)

	mockStore := newFlowStoreInterfaceMock(t)
	mockFlowMgtSvc := flowmgtmock.NewFlowMgtServiceInterfaceMock(t)
	mockEngine := newFlowEngineInterfaceMock(t)
	mockAppService := applicationmock.NewApplicationServiceInterfaceMock(t)

	mockAppService.EXPECT().GetApplication(mock.Anything, "test-app").Return(
		&appmodel.Application{ID: "test-app", AuthFlowID: "auth-graph-1"}, nil).Times(2)
	mockFlowMgtSvc.EXPECT().GetGraph(mock.Anything, "auth-graph-1").Return(testGraph, nil)
	mockEngine.EXPECT().Execute(mock.Anything).Return(FlowStep{}, &ErrorInvalidChallengeToken)
	// DeleteFlowContext must NOT be called — new flows have no persisted context to clean up

	service := &flowExecService{
		flowStore:      mockStore,
		flowMgtService: mockFlowMgtSvc,
		flowEngine:     mockEngine,
		appService:     mockAppService,
		transactioner:  &stubTransactioner{},
	}

	// Pass empty executionID to indicate a new flow
	flowStep, svcErr := service.Execute(context.Background(), "test-app", "",
		string(common.FlowTypeAuthentication), false, "submit", map[string]string{}, "")

	assert.NotNil(t, svcErr)
	assert.Equal(t, ErrorInvalidChallengeToken.Code, svcErr.Code)
	assert.Nil(t, flowStep)
}
