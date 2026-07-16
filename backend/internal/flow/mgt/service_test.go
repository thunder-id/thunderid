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

package flowmgt

import (
	"context"
	"errors"
	"testing"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/executor"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/tests/mocks/flow/executormock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/interceptormock"
)

const testFlowIDService = "test-flow-id"

type FlowMgtServiceTestSuite struct {
	suite.Suite
	service                 FlowMgtServiceInterface
	mockStore               *flowStoreInterfaceMock
	mockInference           *flowInferenceServiceInterfaceMock
	mockGraphBuilder        *graphBuilderInterfaceMock
	mockExecutorRegistry    *executormock.ExecutorRegistryInterfaceMock
	mockInterceptorRegistry *interceptormock.InterceptorRegistryInterfaceMock
	mockValidator           *FlowValidatorInterfaceMock
}

func TestFlowMgtServiceTestSuite(t *testing.T) {
	suite.Run(t, new(FlowMgtServiceTestSuite))
}

// stubTransactioner is a stub implementation of Transactioner for testing.
type stubTransactioner struct{}

func (s *stubTransactioner) Transact(ctx context.Context, txFunc func(context.Context) error) error {
	return txFunc(ctx)
}

// stubDependencyRegistry is a stub implementation of resourcedependency.Registry for testing.
// The real registry never returns an error from GetDependencies, so a stub is required to
// exercise the error branch of GetFlowUsages.
type stubDependencyRegistry struct {
	resp *resourcedependency.DependenciesResponse
	err  error
}

func (r *stubDependencyRegistry) RegisterProvider(resourcedependency.Provider) {}

func (r *stubDependencyRegistry) GetDependencies(
	context.Context, string, string) (*resourcedependency.DependenciesResponse, error) {
	return r.resp, r.err
}

func (r *stubDependencyRegistry) CascadeDelete(context.Context, string, string) (int, error) {
	return 0, nil
}

func (s *FlowMgtServiceTestSuite) SetupTest() {
	s.mockStore = newFlowStoreInterfaceMock(s.T())
	s.mockInference = newFlowInferenceServiceInterfaceMock(s.T())
	s.mockGraphBuilder = newGraphBuilderInterfaceMock(s.T())
	s.mockExecutorRegistry = executormock.NewExecutorRegistryInterfaceMock(s.T())
	s.mockInterceptorRegistry = interceptormock.NewInterceptorRegistryInterfaceMock(s.T())
	s.mockValidator = NewFlowValidatorInterfaceMock(s.T())
	s.service = newFlowMgtService(s.mockStore, s.mockInference, s.mockGraphBuilder,
		s.mockExecutorRegistry, s.mockInterceptorRegistry, s.mockValidator, nil, &stubTransactioner{})

	testConfig := &config.Config{
		Flow: engineconfig.FlowConfig{
			AutoInferRegistration: false,
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("test", testConfig)
}

func (s *FlowMgtServiceTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func validFlowNodes() []providers.NodeDefinition {
	return []providers.NodeDefinition{
		{ID: "start", Type: "START", OnSuccess: "prompt"},
		{ID: "prompt", Type: "PROMPT", Next: "end"},
		{ID: "end", Type: "END"},
	}
}

// authCapableNodes returns nodes that satisfy the Authentication flow's required executor constraint.
func authCapableNodes() []providers.NodeDefinition {
	return []providers.NodeDefinition{
		{ID: "start", Type: "START", OnSuccess: "assert"},
		{
			ID: "assert", Type: string(common.NodeTypeTaskExecution),
			Executor:  &providers.ExecutorDefinition{Name: executor.ExecutorNameAuthAssert},
			OnSuccess: "end",
		},
		{ID: "end", Type: "END"},
	}
}

// ListFlows tests

func (s *FlowMgtServiceTestSuite) TestListFlows_Success() {
	expectedFlows := []BasicFlowDefinition{
		{ID: "flow1", Handle: "test-handle", Name: "Flow 1", FlowType: providers.FlowTypeAuthentication},
	}
	s.mockStore.EXPECT().ListFlows(mock.Anything, 30, 0, "").Return(expectedFlows, 1, nil)

	result, err := s.service.ListFlows(context.Background(), 30, 0, "")

	s.Nil(err)
	s.NotNil(result)
	s.Equal(1, result.Count)
	s.Equal(1, result.TotalResults)
	s.Len(result.Flows, 1)
}

func (s *FlowMgtServiceTestSuite) TestListFlows_DefaultLimit() {
	s.mockStore.EXPECT().ListFlows(mock.Anything, defaultPageSize, 0, "").Return([]BasicFlowDefinition{}, 0, nil)

	result, err := s.service.ListFlows(context.Background(), 0, 0, "")

	s.Nil(err)
	s.NotNil(result)
}

func (s *FlowMgtServiceTestSuite) TestListFlows_MaxLimitExceeded() {
	s.mockStore.EXPECT().ListFlows(mock.Anything, maxPageSize, 0, "").Return([]BasicFlowDefinition{}, 0, nil)

	result, err := s.service.ListFlows(context.Background(), 1000, 0, "")

	s.Nil(err)
	s.NotNil(result)
}

func (s *FlowMgtServiceTestSuite) TestListFlows_NegativeOffset() {
	s.mockStore.EXPECT().ListFlows(mock.Anything, 30, 0, "").Return([]BasicFlowDefinition{}, 0, nil)

	result, err := s.service.ListFlows(context.Background(), 30, -10, "")

	s.Nil(err)
	s.NotNil(result)
}

func (s *FlowMgtServiceTestSuite) TestListFlows_WithFlowType() {
	s.mockStore.EXPECT().ListFlows(mock.Anything, 30, 0, string(providers.FlowTypeAuthentication)).
		Return([]BasicFlowDefinition{}, 0, nil)

	result, err := s.service.ListFlows(context.Background(), 30, 0, providers.FlowTypeAuthentication)

	s.Nil(err)
	s.NotNil(result)
}

func (s *FlowMgtServiceTestSuite) TestListFlows_InvalidFlowType() {
	result, err := s.service.ListFlows(context.Background(), 30, 0, "invalid")

	s.Nil(result)
	s.Equal(&ErrorInvalidFlowType, err)
}

func (s *FlowMgtServiceTestSuite) TestListFlows_StoreError() {
	s.mockStore.EXPECT().ListFlows(mock.Anything, 30, 0, "").Return(nil, 0, errors.New("db error"))

	result, err := s.service.ListFlows(context.Background(), 30, 0, "")

	s.Nil(result)
	s.Equal(&tidcommon.InternalServerError, err)
}

func (s *FlowMgtServiceTestSuite) TestListFlows_PaginationLinks() {
	s.mockStore.EXPECT().ListFlows(mock.Anything, 10, 20, "").Return([]BasicFlowDefinition{}, 100, nil)

	result, err := s.service.ListFlows(context.Background(), 10, 20, "")

	s.Nil(err)
	s.NotNil(result)
	// Should have first, prev, next, last links
	s.Len(result.Links, 4)
}

func (s *FlowMgtServiceTestSuite) TestListFlows_PaginationLinksFirstPage() {
	s.mockStore.EXPECT().ListFlows(mock.Anything, 10, 0, "").Return([]BasicFlowDefinition{}, 100, nil)

	result, err := s.service.ListFlows(context.Background(), 10, 0, "")

	s.Nil(err)
	s.NotNil(result)
	// Should only have next and last links (no first/prev on first page)
	s.Len(result.Links, 2)
}

func (s *FlowMgtServiceTestSuite) TestListFlows_PaginationLinksLastPage() {
	s.mockStore.EXPECT().ListFlows(mock.Anything, 10, 90, "").Return([]BasicFlowDefinition{}, 100, nil)

	result, err := s.service.ListFlows(context.Background(), 10, 90, "")

	s.Nil(err)
	s.NotNil(result)
	// Should only have first and prev links (no next/last on last page)
	s.Len(result.Links, 2)
}

// CreateFlow tests

func (s *FlowMgtServiceTestSuite) TestCreateFlow_Success() {
	flowDef := &FlowDefinition{
		Handle:   "test-handle",
		Name:     "Test Flow",
		FlowType: providers.FlowTypeAuthentication,
		Nodes:    validFlowNodes(),
	}
	expectedFlow := &providers.CompleteFlowDefinition{
		Handle:        "test-handle",
		Name:          "Test Flow",
		FlowType:      providers.FlowTypeAuthentication,
		ActiveVersion: 1,
	}
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)
	s.mockStore.EXPECT().IsFlowExistsByHandle(mock.Anything, "test-handle",
		providers.FlowTypeAuthentication).Return(false, nil)
	s.mockStore.EXPECT().CreateFlow(mock.Anything, mock.Anything, flowDef).Return(expectedFlow, nil)

	result, err := s.service.CreateFlow(context.Background(), flowDef)

	s.Nil(err)
	s.NotNil(result)
	s.Equal("Test Flow", result.Name)
}

func (s *FlowMgtServiceTestSuite) TestCreateFlow_ValidationError() {
	flowDef := &FlowDefinition{
		Handle:   "",
		Name:     "",
		FlowType: providers.FlowTypeAuthentication,
		Nodes:    []providers.NodeDefinition{{Type: "start"}, {Type: "end"}},
	}

	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(&ErrorMissingFlowHandle)

	result, err := s.service.CreateFlow(context.Background(), flowDef)

	s.Nil(result)
	s.Equal(&ErrorMissingFlowHandle, err)
}

func (s *FlowMgtServiceTestSuite) TestCreateFlow_InvalidProvidedFlowID() {
	flowDef := &FlowDefinition{
		ID:       "not-a-uuid",
		Handle:   "test-handle",
		Name:     "Test Flow",
		FlowType: providers.FlowTypeAuthentication,
		Nodes:    validFlowNodes(),
	}

	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(&ErrorInvalidFlowIDFormat)

	result, err := s.service.CreateFlow(context.Background(), flowDef)

	s.Nil(result)
	s.Equal(&ErrorInvalidFlowIDFormat, err)
}

func (s *FlowMgtServiceTestSuite) TestCreateFlow_DuplicateProvidedFlowID() {
	flowID := "550e8400-e29b-41d4-a716-446655440000"
	flowDef := &FlowDefinition{
		ID:       flowID,
		Handle:   "test-handle",
		Name:     "Test Flow",
		FlowType: providers.FlowTypeAuthentication,
		Nodes:    validFlowNodes(),
	}

	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, flowID).Return(&providers.CompleteFlowDefinition{ID: flowID}, nil)

	result, err := s.service.CreateFlow(context.Background(), flowDef)

	s.Nil(result)
	s.Equal(&ErrorDuplicateFlowID, err)
}

func (s *FlowMgtServiceTestSuite) TestCreateFlow_InvalidHandleFormat_Uppercase() {
	flowDef := &FlowDefinition{
		Handle:   "Test-Handle",
		Name:     "Test",
		FlowType: providers.FlowTypeAuthentication,
		Nodes:    validFlowNodes(),
	}

	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(&ErrorInvalidFlowHandleFormat)

	result, err := s.service.CreateFlow(context.Background(), flowDef)

	s.Nil(result)
	s.Equal(&ErrorInvalidFlowHandleFormat, err)
}

func (s *FlowMgtServiceTestSuite) TestCreateFlow_InvalidHandleFormat_Spaces() {
	flowDef := &FlowDefinition{
		Handle:   "test handle",
		Name:     "Test",
		FlowType: providers.FlowTypeAuthentication,
		Nodes:    validFlowNodes(),
	}

	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(&ErrorInvalidFlowHandleFormat)

	result, err := s.service.CreateFlow(context.Background(), flowDef)

	s.Nil(result)
	s.Equal(&ErrorInvalidFlowHandleFormat, err)
}

func (s *FlowMgtServiceTestSuite) TestCreateFlow_InvalidHandleFormat_SpecialChars() {
	flowDef := &FlowDefinition{
		Handle:   "test@handle",
		Name:     "Test",
		FlowType: providers.FlowTypeAuthentication,
		Nodes:    validFlowNodes(),
	}

	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(&ErrorInvalidFlowHandleFormat)

	result, err := s.service.CreateFlow(context.Background(), flowDef)

	s.Nil(result)
	s.Equal(&ErrorInvalidFlowHandleFormat, err)
}

func (s *FlowMgtServiceTestSuite) TestCreateFlow_InvalidHandleFormat_StartsWithDash() {
	flowDef := &FlowDefinition{
		Handle:   "-test-handle",
		Name:     "Test",
		FlowType: providers.FlowTypeAuthentication,
		Nodes:    validFlowNodes(),
	}

	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(&ErrorInvalidFlowHandleFormat)

	result, err := s.service.CreateFlow(context.Background(), flowDef)

	s.Nil(result)
	s.Equal(&ErrorInvalidFlowHandleFormat, err)
}

func (s *FlowMgtServiceTestSuite) TestCreateFlow_InvalidHandleFormat_EndsWithUnderscore() {
	flowDef := &FlowDefinition{
		Handle:   "test_handle_",
		Name:     "Test",
		FlowType: providers.FlowTypeAuthentication,
		Nodes:    validFlowNodes(),
	}

	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(&ErrorInvalidFlowHandleFormat)

	result, err := s.service.CreateFlow(context.Background(), flowDef)

	s.Nil(result)
	s.Equal(&ErrorInvalidFlowHandleFormat, err)
}

func (s *FlowMgtServiceTestSuite) TestCreateFlow_ValidHandleFormats() {
	testCases := []struct {
		name   string
		handle string
	}{
		{
			name:   "With dashes and numbers",
			handle: "test-handle-123",
		},
		{
			name:   "With underscores",
			handle: "test_handle_456",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			flowDef := &FlowDefinition{
				Handle:   tc.handle,
				Name:     "Test",
				FlowType: providers.FlowTypeAuthentication,
				Nodes:    validFlowNodes(),
			}

			flowID, _ := utils.GenerateUUIDv7()
			expectedFlow := &providers.CompleteFlowDefinition{
				ID:            flowID,
				Handle:        flowDef.Handle,
				Name:          flowDef.Name,
				FlowType:      flowDef.FlowType,
				ActiveVersion: 1,
				Nodes:         flowDef.Nodes,
			}

			s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)
			s.mockStore.EXPECT().IsFlowExistsByHandle(mock.Anything, tc.handle,
				providers.FlowTypeAuthentication).Return(false, nil)
			s.mockStore.EXPECT().CreateFlow(mock.Anything, mock.Anything, flowDef).Return(expectedFlow, nil)

			result, err := s.service.CreateFlow(context.Background(), flowDef)

			s.Nil(err)
			s.NotNil(result)
			s.Equal(tc.handle, result.Handle)
		})
	}
}

func (s *FlowMgtServiceTestSuite) TestCreateFlow_InvalidFlowType() {
	flowDef := &FlowDefinition{
		Handle:   "test-handle",
		Name:     "Test",
		FlowType: "invalid",
		Nodes:    validFlowNodes(),
	}

	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(&ErrorInvalidFlowType)

	result, err := s.service.CreateFlow(context.Background(), flowDef)

	s.Nil(result)
	s.Equal(&ErrorInvalidFlowType, err)
}

func (s *FlowMgtServiceTestSuite) TestCreateFlow_InsufficientNodes() {
	flowDef := &FlowDefinition{
		Handle:   "test-handle",
		Name:     "Test",
		FlowType: providers.FlowTypeAuthentication,
		Nodes:    []providers.NodeDefinition{{Type: "start"}},
	}

	validationErr := tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
		DefaultValue: "insufficient nodes",
	})
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(validationErr)

	result, err := s.service.CreateFlow(context.Background(), flowDef)

	s.Nil(result)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *FlowMgtServiceTestSuite) TestCreateFlow_OnlyStartAndEnd() {
	flowDef := &FlowDefinition{
		Handle:   "test-handle",
		Name:     "Test",
		FlowType: providers.FlowTypeAuthentication,
		Nodes:    []providers.NodeDefinition{{Type: "start"}, {Type: "end"}},
	}

	validationErr := tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
		DefaultValue: "only start and end",
	})
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(validationErr)

	result, err := s.service.CreateFlow(context.Background(), flowDef)

	s.Nil(result)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *FlowMgtServiceTestSuite) TestCreateFlow_StoreError() {
	flowDef := &FlowDefinition{
		Handle:   "test-handle",
		Name:     "Test",
		FlowType: providers.FlowTypeAuthentication,
		Nodes:    validFlowNodes(),
	}
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)
	s.mockStore.EXPECT().IsFlowExistsByHandle(mock.Anything, "test-handle",
		providers.FlowTypeAuthentication).Return(false, nil)
	s.mockStore.EXPECT().CreateFlow(mock.Anything, mock.Anything, flowDef).Return(nil, errors.New("db error"))

	result, err := s.service.CreateFlow(context.Background(), flowDef)

	s.Nil(result)
	s.Equal(&tidcommon.InternalServerError, err)
}

// Interceptor validation tests (CreateFlow)

func (s *FlowMgtServiceTestSuite) TestCreateFlow_WithInterceptors_Success() {
	flowDef := &FlowDefinition{
		Handle:   "test-handle",
		Name:     "Test",
		FlowType: providers.FlowTypeAuthentication,
		Nodes:    validFlowNodes(),
		Interceptors: []providers.InterceptorDefinition{
			{
				Name:  "CaptchaInterceptor",
				Mode:  providers.InterceptorModePreRequest,
				Scope: providers.InterceptorScopeAll,
			},
		},
	}
	expectedFlow := &providers.CompleteFlowDefinition{
		Handle:        "test-handle",
		Name:          "Test",
		FlowType:      providers.FlowTypeAuthentication,
		ActiveVersion: 1,
		Interceptors:  flowDef.Interceptors,
	}
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)
	s.mockStore.EXPECT().IsFlowExistsByHandle(mock.Anything, "test-handle",
		providers.FlowTypeAuthentication).Return(false, nil)
	s.mockStore.EXPECT().CreateFlow(mock.Anything, mock.Anything, flowDef).Return(expectedFlow, nil)

	result, err := s.service.CreateFlow(context.Background(), flowDef)

	s.Nil(err)
	s.NotNil(result)
	s.Len(result.Interceptors, 1)
	s.Equal("CaptchaInterceptor", result.Interceptors[0].Name)
}

func (s *FlowMgtServiceTestSuite) TestCreateFlow_InterceptorMissingName() {
	flowDef := &FlowDefinition{
		Handle:   "test-handle",
		Name:     "Test",
		FlowType: providers.FlowTypeAuthentication,
		Nodes:    validFlowNodes(),
		Interceptors: []providers.InterceptorDefinition{
			{Name: "", Mode: providers.InterceptorModePreRequest},
		},
	}

	validationErr := tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
		DefaultValue: "interceptor missing name",
	})
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(validationErr)

	result, err := s.service.CreateFlow(context.Background(), flowDef)

	s.Nil(result)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *FlowMgtServiceTestSuite) TestCreateFlow_InterceptorInvalidMode() {
	flowDef := &FlowDefinition{
		Handle:   "test-handle",
		Name:     "Test",
		FlowType: providers.FlowTypeAuthentication,
		Nodes:    validFlowNodes(),
		Interceptors: []providers.InterceptorDefinition{
			{Name: "MyInterceptor", Mode: "INVALID_MODE"},
		},
	}

	validationErr := tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
		DefaultValue: "invalid interceptor mode",
	})
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(validationErr)

	result, err := s.service.CreateFlow(context.Background(), flowDef)

	s.Nil(result)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *FlowMgtServiceTestSuite) TestCreateFlow_InterceptorEmptyMode() {
	flowDef := &FlowDefinition{
		Handle:   "test-handle",
		Name:     "Test",
		FlowType: providers.FlowTypeAuthentication,
		Nodes:    validFlowNodes(),
		Interceptors: []providers.InterceptorDefinition{
			{Name: "MyInterceptor", Mode: ""},
		},
	}

	validationErr := tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
		DefaultValue: "empty interceptor mode",
	})
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(validationErr)

	result, err := s.service.CreateFlow(context.Background(), flowDef)

	s.Nil(result)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *FlowMgtServiceTestSuite) TestCreateFlow_InterceptorInvalidScope() {
	flowDef := &FlowDefinition{
		Handle:   "test-handle",
		Name:     "Test",
		FlowType: providers.FlowTypeAuthentication,
		Nodes:    validFlowNodes(),
		Interceptors: []providers.InterceptorDefinition{
			{Name: "MyInterceptor", Mode: providers.InterceptorModePreNode, Scope: "INVALID_SCOPE"},
		},
	}

	validationErr := tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
		DefaultValue: "invalid interceptor scope",
	})
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(validationErr)

	result, err := s.service.CreateFlow(context.Background(), flowDef)

	s.Nil(result)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *FlowMgtServiceTestSuite) TestCreateFlow_InterceptorSelectedScopeEmptyApplyTo() {
	flowDef := &FlowDefinition{
		Handle:   "test-handle",
		Name:     "Test",
		FlowType: providers.FlowTypeAuthentication,
		Nodes:    validFlowNodes(),
		Interceptors: []providers.InterceptorDefinition{
			{
				Name:    "MyInterceptor",
				Mode:    providers.InterceptorModePreNode,
				Scope:   providers.InterceptorScopeSelected,
				ApplyTo: []string{},
			},
		},
	}

	validationErr := tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
		DefaultValue: "selected scope requires applyTo",
	})
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(validationErr)

	result, err := s.service.CreateFlow(context.Background(), flowDef)

	s.Nil(result)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *FlowMgtServiceTestSuite) TestCreateFlow_InterceptorDefaultNotConfigurable() {
	flowDef := &FlowDefinition{
		Handle:   "test-handle",
		Name:     "Test",
		FlowType: providers.FlowTypeAuthentication,
		Nodes:    validFlowNodes(),
		Interceptors: []providers.InterceptorDefinition{
			{Name: "DefaultIC", Mode: providers.InterceptorModePreRequest},
		},
	}

	validationErr := tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
		DefaultValue: "default interceptor not configurable",
	})
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(validationErr)

	result, err := s.service.CreateFlow(context.Background(), flowDef)

	s.Nil(result)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *FlowMgtServiceTestSuite) TestCreateFlow_InterceptorNotRegistered() {
	flowDef := &FlowDefinition{
		Handle:   "test-handle",
		Name:     "Test",
		FlowType: providers.FlowTypeAuthentication,
		Nodes:    validFlowNodes(),
		Interceptors: []providers.InterceptorDefinition{
			{Name: "UnknownInterceptor", Mode: providers.InterceptorModePreRequest},
		},
	}

	validationErr := tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
		DefaultValue: "interceptor not registered",
	})
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(validationErr)

	result, err := s.service.CreateFlow(context.Background(), flowDef)

	s.Nil(result)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

// Interceptor update tests (UpdateFlow)

func (s *FlowMgtServiceTestSuite) TestUpdateFlow_WithInterceptors_Success() {
	existingFlow := &providers.CompleteFlowDefinition{
		ID:            testFlowIDService,
		Handle:        "test-handle",
		FlowType:      providers.FlowTypeAuthentication,
		ActiveVersion: 1,
	}
	flowDef := &FlowDefinition{
		Handle:   "test-handle",
		Name:     "Updated Flow",
		FlowType: providers.FlowTypeAuthentication,
		Nodes:    validFlowNodes(),
		Interceptors: []providers.InterceptorDefinition{
			{
				Name:    "RateLimitInterceptor",
				Mode:    providers.InterceptorModePreNode,
				Scope:   providers.InterceptorScopeSelected,
				ApplyTo: []string{"prompt"},
			},
		},
	}
	expectedFlow := &providers.CompleteFlowDefinition{
		Handle:        "test-handle",
		Name:          "Updated Flow",
		FlowType:      providers.FlowTypeAuthentication,
		ActiveVersion: 2,
		Interceptors:  flowDef.Interceptors,
	}
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(existingFlow, nil)
	s.mockStore.EXPECT().UpdateFlow(mock.Anything, testFlowIDService, flowDef).Return(expectedFlow, nil)
	s.mockGraphBuilder.EXPECT().InvalidateCache(mock.Anything, testFlowIDService)

	result, err := s.service.UpdateFlow(context.Background(), testFlowIDService, flowDef)

	s.Nil(err)
	s.NotNil(result)
	s.Len(result.Interceptors, 1)
	s.Equal("RateLimitInterceptor", result.Interceptors[0].Name)
}

func (s *FlowMgtServiceTestSuite) TestUpdateFlow_RemoveInterceptors() {
	existingFlow := &providers.CompleteFlowDefinition{
		ID:       testFlowIDService,
		Handle:   "test-handle",
		FlowType: providers.FlowTypeAuthentication,
		Interceptors: []providers.InterceptorDefinition{
			{Name: "OldInterceptor", Mode: providers.InterceptorModePreRequest},
		},
	}
	flowDef := &FlowDefinition{
		Handle:       "test-handle",
		Name:         "Updated Flow",
		FlowType:     providers.FlowTypeAuthentication,
		Nodes:        validFlowNodes(),
		Interceptors: nil,
	}
	expectedFlow := &providers.CompleteFlowDefinition{
		Handle:        "test-handle",
		Name:          "Updated Flow",
		FlowType:      providers.FlowTypeAuthentication,
		ActiveVersion: 2,
	}
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(existingFlow, nil)
	s.mockStore.EXPECT().UpdateFlow(mock.Anything, testFlowIDService, flowDef).Return(expectedFlow, nil)
	s.mockGraphBuilder.EXPECT().InvalidateCache(mock.Anything, testFlowIDService)

	result, err := s.service.UpdateFlow(context.Background(), testFlowIDService, flowDef)

	s.Nil(err)
	s.NotNil(result)
	s.Empty(result.Interceptors)
}

func (s *FlowMgtServiceTestSuite) TestUpdateFlow_InvalidInterceptorMode() {
	flowDef := &FlowDefinition{
		Handle:   "test-handle",
		Name:     "Test",
		FlowType: providers.FlowTypeAuthentication,
		Nodes:    validFlowNodes(),
		Interceptors: []providers.InterceptorDefinition{
			{Name: "MyInterceptor", Mode: "BAD_MODE"},
		},
	}

	validationErr := tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
		DefaultValue: "invalid interceptor mode",
	})
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(validationErr)

	result, err := s.service.UpdateFlow(context.Background(), testFlowIDService, flowDef)

	s.Nil(result)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *FlowMgtServiceTestSuite) TestCreateFlow_WithAutoInference() {
	// Enable auto-inference for this test
	config.ResetServerRuntime()
	testConfig := &config.Config{
		Flow: engineconfig.FlowConfig{
			AutoInferRegistration: true,
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("test", testConfig)

	authNodes := authCapableNodes()
	flowDef := &FlowDefinition{
		Handle:   "test-handle",
		Name:     "Auth Flow",
		FlowType: providers.FlowTypeAuthentication,
		Nodes:    authNodes,
	}
	expectedFlow := &providers.CompleteFlowDefinition{
		Handle:        "test-handle",
		Name:          "Auth Flow",
		FlowType:      providers.FlowTypeAuthentication,
		ActiveVersion: 1,
	}
	inferredRegFlow := &FlowDefinition{
		Handle:   "test-handle-reg",
		Name:     "Auth Flow - Registration",
		FlowType: providers.FlowTypeRegistration,
		Nodes:    validFlowNodes(),
	}

	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)
	s.mockStore.EXPECT().IsFlowExistsByHandle(mock.Anything, "test-handle",
		providers.FlowTypeAuthentication).Return(false, nil)
	s.mockStore.EXPECT().CreateFlow(mock.Anything, mock.Anything, flowDef).Return(expectedFlow, nil)
	s.mockInference.EXPECT().InferRegistrationFlow(mock.Anything, flowDef).Return(inferredRegFlow, nil)
	s.mockStore.EXPECT().CreateFlow(mock.Anything, mock.Anything, inferredRegFlow).Return(nil, nil)

	result, err := s.service.CreateFlow(context.Background(), flowDef)

	s.Nil(err)
	s.NotNil(result)
}

func (s *FlowMgtServiceTestSuite) TestCreateFlow_AutoInferenceFailure() {
	// Enable auto-inference for this test
	config.ResetServerRuntime()
	testConfig := &config.Config{
		Flow: engineconfig.FlowConfig{
			AutoInferRegistration: true,
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("test", testConfig)

	authNodes := authCapableNodes()
	flowDef := &FlowDefinition{
		Handle:   "test-handle",
		Name:     "Auth Flow",
		FlowType: providers.FlowTypeAuthentication,
		Nodes:    authNodes,
	}
	expectedFlow := &providers.CompleteFlowDefinition{
		Handle:        "test-handle",
		Name:          "Auth Flow",
		FlowType:      providers.FlowTypeAuthentication,
		ActiveVersion: 1,
	}

	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)
	s.mockStore.EXPECT().IsFlowExistsByHandle(mock.Anything, "test-handle",
		providers.FlowTypeAuthentication).Return(false, nil)
	s.mockStore.EXPECT().CreateFlow(mock.Anything, mock.Anything, flowDef).Return(expectedFlow, nil)
	s.mockInference.EXPECT().InferRegistrationFlow(mock.Anything, flowDef).Return(nil, errors.New("inference error"))

	// Should still succeed even if inference fails
	result, err := s.service.CreateFlow(context.Background(), flowDef)

	s.Nil(err)
	s.NotNil(result)
}

func (s *FlowMgtServiceTestSuite) TestCreateFlow_DuplicateHandle() {
	flowDef := &FlowDefinition{
		Handle:   "existing-handle",
		Name:     "Test Flow",
		FlowType: providers.FlowTypeAuthentication,
		Nodes:    validFlowNodes(),
	}
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)
	s.mockStore.EXPECT().
		IsFlowExistsByHandle(mock.Anything, "existing-handle", providers.FlowTypeAuthentication).
		Return(
			true, nil)

	result, err := s.service.CreateFlow(context.Background(), flowDef)

	s.Nil(result)
	s.Equal(&ErrorDuplicateFlowHandle, err)
}

func (s *FlowMgtServiceTestSuite) TestCreateFlow_DuplicateHandleCheckError() {
	flowDef := &FlowDefinition{
		Handle:   "test-handle",
		Name:     "Test Flow",
		FlowType: providers.FlowTypeAuthentication,
		Nodes:    validFlowNodes(),
	}
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)
	s.mockStore.EXPECT().IsFlowExistsByHandle(mock.Anything, "test-handle", providers.FlowTypeAuthentication).Return(
		false, errors.New("db error"))

	result, err := s.service.CreateFlow(context.Background(), flowDef)

	s.Nil(result)
	s.Equal(&tidcommon.InternalServerError, err)
}

// GetFlow tests

func (s *FlowMgtServiceTestSuite) TestGetFlow_Success() {
	expectedFlow := &providers.CompleteFlowDefinition{
		ID:     testFlowIDService,
		Handle: "test-handle",
		Name:   "Test",
	}
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(expectedFlow, nil)

	result, err := s.service.GetFlow(context.Background(), testFlowIDService)

	s.Nil(err)
	s.Equal(expectedFlow, result)
}

func (s *FlowMgtServiceTestSuite) TestGetFlow_EmptyID() {
	result, err := s.service.GetFlow(context.Background(), "")

	s.Nil(result)
	s.Equal(&ErrorMissingFlowID, err)
}

func (s *FlowMgtServiceTestSuite) TestGetFlow_NotFound() {
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(nil, errFlowNotFound)

	result, err := s.service.GetFlow(context.Background(), testFlowIDService)

	s.Nil(result)
	s.Equal(&ErrorFlowNotFound, err)
}

func (s *FlowMgtServiceTestSuite) TestGetFlow_StoreError() {
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(nil, errors.New("db error"))

	result, err := s.service.GetFlow(context.Background(), testFlowIDService)

	s.Nil(result)
	s.Equal(&tidcommon.InternalServerError, err)
}

// GetFlowUsages tests

func (s *FlowMgtServiceTestSuite) TestGetFlowUsages_EmptyID() {
	result, err := s.service.GetFlowUsages(context.Background(), "")

	s.Nil(result)
	s.Equal(&ErrorMissingFlowID, err)
}

func (s *FlowMgtServiceTestSuite) TestGetFlowUsages_NotFound() {
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(nil, errFlowNotFound)

	result, err := s.service.GetFlowUsages(context.Background(), testFlowIDService)

	s.Nil(result)
	s.Equal(&ErrorFlowNotFound, err)
}

func (s *FlowMgtServiceTestSuite) TestGetFlowUsages_StoreError() {
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(nil, errors.New("db error"))

	result, err := s.service.GetFlowUsages(context.Background(), testFlowIDService)

	s.Nil(result)
	s.Equal(&tidcommon.InternalServerError, err)
}

func (s *FlowMgtServiceTestSuite) TestGetFlowUsages_NoRegistrySet() {
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).
		Return(&providers.CompleteFlowDefinition{ID: testFlowIDService}, nil)

	result, err := s.service.GetFlowUsages(context.Background(), testFlowIDService)

	s.Nil(err)
	s.NotNil(result)
	s.Nil(result.TotalResults)
	s.Nil(result.Summary)
	s.Empty(result.Usages)
}

func (s *FlowMgtServiceTestSuite) TestGetFlowUsages_RegistryError() {
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).
		Return(&providers.CompleteFlowDefinition{ID: testFlowIDService}, nil)
	s.service.SetDependencyRegistry(&stubDependencyRegistry{err: errors.New("registry error")})

	result, err := s.service.GetFlowUsages(context.Background(), testFlowIDService)

	s.Nil(result)
	s.Equal(&tidcommon.InternalServerError, err)
}

func (s *FlowMgtServiceTestSuite) TestGetFlowUsages_Success() {
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).
		Return(&providers.CompleteFlowDefinition{ID: testFlowIDService}, nil)
	total := 1
	expected := &resourcedependency.DependenciesResponse{
		TotalResults: &total,
		Count:        1,
		Summary:      map[string]int{resourcedependency.ResourceTypeApplication: 1},
		Usages: []resourcedependency.ResourceDependency{
			{ResourceType: resourcedependency.ResourceTypeApplication, ID: "app-1",
				DisplayName: "App One", BehaviorOnDelete: resourcedependency.BehaviorFallback},
		},
	}
	s.service.SetDependencyRegistry(&stubDependencyRegistry{resp: expected})

	result, err := s.service.GetFlowUsages(context.Background(), testFlowIDService)

	s.Nil(err)
	s.Equal(expected, result)
}

func (s *FlowMgtServiceTestSuite) TestGetFlowByHandle_Success() {
	expectedFlow := &providers.CompleteFlowDefinition{
		ID:       testFlowIDService,
		Handle:   "test-auth-flow",
		Name:     "Test Auth Flow",
		FlowType: providers.FlowTypeAuthentication,
	}
	s.mockStore.EXPECT().GetFlowByHandle(mock.Anything, "test-auth-flow", providers.FlowTypeAuthentication).
		Return(expectedFlow, nil)

	result, err := s.service.GetFlowByHandle(context.Background(), "test-auth-flow", providers.FlowTypeAuthentication)

	s.Nil(err)
	s.Equal(expectedFlow, result)
	s.Equal("test-auth-flow", result.Handle)
	s.Equal(providers.FlowTypeAuthentication, result.FlowType)
}

func (s *FlowMgtServiceTestSuite) TestGetFlowByHandle_SuccessRegistrationFlow() {
	expectedFlow := &providers.CompleteFlowDefinition{
		ID:       "flow-reg-id",
		Handle:   "test-reg-flow",
		Name:     "Test Registration Flow",
		FlowType: providers.FlowTypeRegistration,
	}
	s.mockStore.EXPECT().GetFlowByHandle(mock.Anything, "test-reg-flow", providers.FlowTypeRegistration).
		Return(expectedFlow, nil)

	result, err := s.service.GetFlowByHandle(context.Background(), "test-reg-flow", providers.FlowTypeRegistration)

	s.Nil(err)
	s.Equal(expectedFlow, result)
	s.Equal("test-reg-flow", result.Handle)
	s.Equal(providers.FlowTypeRegistration, result.FlowType)
}

func (s *FlowMgtServiceTestSuite) TestGetFlowByHandle_EmptyHandle() {
	result, err := s.service.GetFlowByHandle(context.Background(), "", providers.FlowTypeAuthentication)

	s.Nil(result)
	s.Equal(&ErrorMissingFlowHandle, err)
}

func (s *FlowMgtServiceTestSuite) TestGetFlowByHandle_InvalidFlowType() {
	result, err := s.service.GetFlowByHandle(context.Background(), "test-handle", "INVALID_TYPE")

	s.Nil(result)
	s.Equal(&ErrorInvalidFlowType, err)
}

func (s *FlowMgtServiceTestSuite) TestGetFlowByHandle_EmptyFlowType() {
	result, err := s.service.GetFlowByHandle(context.Background(), "test-handle", "")

	s.Nil(result)
	s.Equal(&ErrorInvalidFlowType, err)
}

func (s *FlowMgtServiceTestSuite) TestGetFlowByHandle_NotFound() {
	s.mockStore.EXPECT().GetFlowByHandle(mock.Anything, "non-existent-handle", providers.FlowTypeAuthentication).
		Return(nil, errFlowNotFound)

	result, err := s.service.GetFlowByHandle(
		context.Background(),
		"non-existent-handle",
		providers.FlowTypeAuthentication,
	)

	s.Nil(result)
	s.Equal(&ErrorFlowNotFound, err)
}

func (s *FlowMgtServiceTestSuite) TestGetFlowByHandle_StoreError() {
	s.mockStore.EXPECT().GetFlowByHandle(mock.Anything, "test-handle", providers.FlowTypeAuthentication).
		Return(nil, errors.New("database connection error"))

	result, err := s.service.GetFlowByHandle(context.Background(), "test-handle", providers.FlowTypeAuthentication)

	s.Nil(result)
	s.Equal(&tidcommon.InternalServerError, err)
}

// UpdateFlow tests

func (s *FlowMgtServiceTestSuite) TestUpdateFlow_Success() {
	existingFlow := &providers.CompleteFlowDefinition{
		ID:       testFlowIDService,
		Handle:   "test-handle",
		FlowType: providers.FlowTypeAuthentication,
	}
	flowDef := &FlowDefinition{
		Handle:   "test-handle",
		Name:     "Updated",
		FlowType: providers.FlowTypeAuthentication,
		Nodes:    validFlowNodes(),
	}
	updatedFlow := &providers.CompleteFlowDefinition{
		Handle:        "test-handle",
		Name:          "Updated",
		ActiveVersion: 2,
	}
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(existingFlow, nil)
	s.mockStore.EXPECT().UpdateFlow(mock.Anything, testFlowIDService, flowDef).Return(updatedFlow, nil)
	s.mockGraphBuilder.EXPECT().InvalidateCache(mock.Anything, testFlowIDService)

	result, err := s.service.UpdateFlow(context.Background(), testFlowIDService, flowDef)

	s.Nil(err)
	s.Equal(updatedFlow, result)
}

func (s *FlowMgtServiceTestSuite) TestUpdateFlow_EmptyID() {
	flowDef := &FlowDefinition{Name: "Test", FlowType: providers.FlowTypeAuthentication}

	result, err := s.service.UpdateFlow(context.Background(), "", flowDef)

	s.Nil(result)
	s.Equal(&ErrorMissingFlowID, err)
}

func (s *FlowMgtServiceTestSuite) TestUpdateFlow_ValidationError() {
	flowDef := &FlowDefinition{Handle: "", Name: "", FlowType: providers.FlowTypeAuthentication}

	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(&ErrorMissingFlowHandle)

	result, err := s.service.UpdateFlow(context.Background(), testFlowIDService, flowDef)

	s.Nil(result)
	s.Equal(&ErrorMissingFlowHandle, err)
}

func (s *FlowMgtServiceTestSuite) TestUpdateFlow_FlowNotFound() {
	flowDef := &FlowDefinition{
		Handle:   "test-handle",
		Name:     "Test",
		FlowType: providers.FlowTypeAuthentication,
		Nodes:    validFlowNodes(),
	}
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(nil, errFlowNotFound)

	result, err := s.service.UpdateFlow(context.Background(), testFlowIDService, flowDef)

	s.Nil(result)
	s.Equal(&ErrorFlowNotFound, err)
}

func (s *FlowMgtServiceTestSuite) TestUpdateFlow_CannotChangeFlowType() {
	existingFlow := &providers.CompleteFlowDefinition{
		ID:       testFlowIDService,
		Handle:   "test-handle",
		FlowType: providers.FlowTypeAuthentication,
	}
	flowDef := &FlowDefinition{
		Handle:   "test-handle",
		Name:     "Test",
		FlowType: providers.FlowTypeRegistration,
		Nodes:    validFlowNodes(),
	}
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(existingFlow, nil)

	result, err := s.service.UpdateFlow(context.Background(), testFlowIDService, flowDef)

	s.Nil(result)
	s.Equal(&ErrorCannotUpdateFlowType, err)
}

func (s *FlowMgtServiceTestSuite) TestUpdateFlow_CannotChangeHandle() {
	existingFlow := &providers.CompleteFlowDefinition{
		ID:       testFlowIDService,
		Handle:   "original-handle",
		FlowType: providers.FlowTypeAuthentication,
	}
	flowDef := &FlowDefinition{
		Handle:   "new-handle",
		Name:     "Test",
		FlowType: providers.FlowTypeAuthentication,
		Nodes:    validFlowNodes(),
	}
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(existingFlow, nil)

	result, err := s.service.UpdateFlow(context.Background(), testFlowIDService, flowDef)

	s.Nil(result)
	s.Equal(&ErrorHandleUpdateNotAllowed, err)
}

func (s *FlowMgtServiceTestSuite) TestUpdateFlow_StoreError() {
	existingFlow := &providers.CompleteFlowDefinition{
		ID:       testFlowIDService,
		Handle:   "test-handle",
		FlowType: providers.FlowTypeAuthentication,
	}
	flowDef := &FlowDefinition{
		Handle:   "test-handle",
		Name:     "Test",
		FlowType: providers.FlowTypeAuthentication,
		Nodes:    validFlowNodes(),
	}
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(existingFlow, nil)
	s.mockStore.EXPECT().UpdateFlow(mock.Anything, testFlowIDService, flowDef).Return(nil, errors.New("db error"))

	result, err := s.service.UpdateFlow(context.Background(), testFlowIDService, flowDef)

	s.Nil(result)
	s.Equal(&tidcommon.InternalServerError, err)
}

// DeleteFlow tests

func (s *FlowMgtServiceTestSuite) TestDeleteFlow_Success() {
	existingFlow := &providers.CompleteFlowDefinition{ID: testFlowIDService, Handle: "test-handle"}
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(existingFlow, nil)
	s.mockStore.EXPECT().DeleteFlow(mock.Anything, testFlowIDService).Return(nil)
	s.mockGraphBuilder.EXPECT().InvalidateCache(mock.Anything, testFlowIDService)

	err := s.service.DeleteFlow(context.Background(), testFlowIDService)

	s.Nil(err)
}

func (s *FlowMgtServiceTestSuite) TestDeleteFlow_EmptyID() {
	err := s.service.DeleteFlow(context.Background(), "")

	s.Equal(&ErrorMissingFlowID, err)
}

func (s *FlowMgtServiceTestSuite) TestDeleteFlow_NotFound() {
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(nil, errFlowNotFound)

	err := s.service.DeleteFlow(context.Background(), testFlowIDService)

	s.Nil(err)
}

func (s *FlowMgtServiceTestSuite) TestDeleteFlow_GetError() {
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(nil, errors.New("db error"))

	err := s.service.DeleteFlow(context.Background(), testFlowIDService)

	s.Equal(&tidcommon.InternalServerError, err)
}

func (s *FlowMgtServiceTestSuite) TestDeleteFlow_StoreError() {
	existingFlow := &providers.CompleteFlowDefinition{ID: testFlowIDService, Handle: "test-handle"}
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(existingFlow, nil)
	s.mockStore.EXPECT().DeleteFlow(mock.Anything, testFlowIDService).Return(errors.New("db error"))

	err := s.service.DeleteFlow(context.Background(), testFlowIDService)

	s.Equal(&tidcommon.InternalServerError, err)
}

// ListFlowVersions tests

func (s *FlowMgtServiceTestSuite) TestListFlowVersions_Success() {
	existingFlow := &providers.CompleteFlowDefinition{ID: testFlowIDService, Handle: "test-handle"}
	versions := []BasicFlowVersion{{Version: 1}, {Version: 2}}
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(existingFlow, nil)
	s.mockStore.EXPECT().ListFlowVersions(mock.Anything, testFlowIDService).Return(versions, nil)

	result, err := s.service.ListFlowVersions(context.Background(), testFlowIDService)

	s.Nil(err)
	s.NotNil(result)
	s.Equal(2, result.TotalVersions)
	s.Len(result.Versions, 2)
}

func (s *FlowMgtServiceTestSuite) TestListFlowVersions_EmptyID() {
	result, err := s.service.ListFlowVersions(context.Background(), "")

	s.Nil(result)
	s.Equal(&ErrorMissingFlowID, err)
}

func (s *FlowMgtServiceTestSuite) TestListFlowVersions_FlowNotFound() {
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(nil, errFlowNotFound)

	result, err := s.service.ListFlowVersions(context.Background(), testFlowIDService)

	s.Nil(result)
	s.Equal(&ErrorFlowNotFound, err)
}

func (s *FlowMgtServiceTestSuite) TestListFlowVersions_StoreError() {
	existingFlow := &providers.CompleteFlowDefinition{ID: testFlowIDService, Handle: "test-handle"}
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(existingFlow, nil)
	s.mockStore.EXPECT().ListFlowVersions(mock.Anything, testFlowIDService).Return(nil, errors.New("db error"))

	result, err := s.service.ListFlowVersions(context.Background(), testFlowIDService)

	s.Nil(result)
	s.Equal(&tidcommon.InternalServerError, err)
}

// GetFlowVersion tests

func (s *FlowMgtServiceTestSuite) TestGetFlowVersion_Success() {
	expectedVersion := &FlowVersion{Version: 1}
	s.mockStore.EXPECT().GetFlowVersion(mock.Anything, testFlowIDService, 1).Return(expectedVersion, nil)

	result, err := s.service.GetFlowVersion(context.Background(), testFlowIDService, 1)

	s.Nil(err)
	s.Equal(expectedVersion, result)
}

func (s *FlowMgtServiceTestSuite) TestGetFlowVersion_EmptyID() {
	result, err := s.service.GetFlowVersion(context.Background(), "", 1)

	s.Nil(result)
	s.Equal(&ErrorMissingFlowID, err)
}

func (s *FlowMgtServiceTestSuite) TestGetFlowVersion_InvalidVersion() {
	result, err := s.service.GetFlowVersion(context.Background(), testFlowIDService, 0)

	s.Nil(result)
	s.Equal(&ErrorInvalidVersion, err)
}

func (s *FlowMgtServiceTestSuite) TestGetFlowVersion_FlowNotFound() {
	s.mockStore.EXPECT().GetFlowVersion(mock.Anything, testFlowIDService, 1).Return(nil, errFlowNotFound)

	result, err := s.service.GetFlowVersion(context.Background(), testFlowIDService, 1)

	s.Nil(result)
	s.Equal(&ErrorFlowNotFound, err)
}

func (s *FlowMgtServiceTestSuite) TestGetFlowVersion_VersionNotFound() {
	s.mockStore.EXPECT().GetFlowVersion(mock.Anything, testFlowIDService, 1).Return(nil, errVersionNotFound)

	result, err := s.service.GetFlowVersion(context.Background(), testFlowIDService, 1)

	s.Nil(result)
	s.Equal(&ErrorVersionNotFound, err)
}

func (s *FlowMgtServiceTestSuite) TestGetFlowVersion_StoreError() {
	s.mockStore.EXPECT().GetFlowVersion(mock.Anything, testFlowIDService, 1).Return(nil, errors.New("db error"))

	result, err := s.service.GetFlowVersion(context.Background(), testFlowIDService, 1)

	s.Nil(result)
	s.Equal(&tidcommon.InternalServerError, err)
}

// RestoreFlowVersion tests

func (s *FlowMgtServiceTestSuite) TestRestoreFlowVersion_Success() {
	version := &FlowVersion{Version: 1}
	restoredFlow := &providers.CompleteFlowDefinition{ActiveVersion: 2}
	s.mockStore.EXPECT().GetFlowVersion(mock.Anything, testFlowIDService, 1).Return(version, nil)
	s.mockStore.EXPECT().RestoreFlowVersion(mock.Anything, testFlowIDService, 1).Return(restoredFlow, nil)
	s.mockGraphBuilder.EXPECT().InvalidateCache(mock.Anything, testFlowIDService)

	result, err := s.service.RestoreFlowVersion(context.Background(), testFlowIDService, 1)

	s.Nil(err)
	s.Equal(restoredFlow, result)
}

func (s *FlowMgtServiceTestSuite) TestRestoreFlowVersion_EmptyID() {
	result, err := s.service.RestoreFlowVersion(context.Background(), "", 1)

	s.Nil(result)
	s.Equal(&ErrorMissingFlowID, err)
}

func (s *FlowMgtServiceTestSuite) TestRestoreFlowVersion_InvalidVersion() {
	result, err := s.service.RestoreFlowVersion(context.Background(), testFlowIDService, 0)

	s.Nil(result)
	s.Equal(&ErrorInvalidVersion, err)
}

func (s *FlowMgtServiceTestSuite) TestRestoreFlowVersion_FlowNotFound() {
	s.mockStore.EXPECT().GetFlowVersion(mock.Anything, testFlowIDService, 1).Return(nil, errFlowNotFound)

	result, err := s.service.RestoreFlowVersion(context.Background(), testFlowIDService, 1)

	s.Nil(result)
	s.Equal(&ErrorFlowNotFound, err)
}

func (s *FlowMgtServiceTestSuite) TestRestoreFlowVersion_VersionNotFound() {
	s.mockStore.EXPECT().GetFlowVersion(mock.Anything, testFlowIDService, 1).Return(nil, errVersionNotFound)

	result, err := s.service.RestoreFlowVersion(context.Background(), testFlowIDService, 1)

	s.Nil(result)
	s.Equal(&ErrorVersionNotFound, err)
}

func (s *FlowMgtServiceTestSuite) TestRestoreFlowVersion_StoreError() {
	version := &FlowVersion{Version: 1}
	s.mockStore.EXPECT().GetFlowVersion(mock.Anything, testFlowIDService, 1).Return(version, nil)
	s.mockStore.EXPECT().RestoreFlowVersion(mock.Anything, testFlowIDService, 1).Return(nil, errors.New("db error"))

	result, err := s.service.RestoreFlowVersion(context.Background(), testFlowIDService, 1)

	s.Nil(result)
	s.Equal(&tidcommon.InternalServerError, err)
}

// GetGraph tests

func (s *FlowMgtServiceTestSuite) TestGetGraph_Success() {
	flow := &providers.CompleteFlowDefinition{ID: testFlowIDService}
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(flow, nil)
	s.mockGraphBuilder.EXPECT().GetGraph(mock.Anything, flow).Return(nil, nil)

	result, err := s.service.GetGraph(context.Background(), testFlowIDService)

	s.Nil(err)
	s.Nil(result)
}

func (s *FlowMgtServiceTestSuite) TestGetGraph_EmptyID() {
	result, err := s.service.GetGraph(context.Background(), "")

	s.Nil(result)
	s.Equal(&ErrorMissingFlowID, err)
}

func (s *FlowMgtServiceTestSuite) TestGetGraph_FlowNotFound() {
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(nil, errFlowNotFound)

	result, err := s.service.GetGraph(context.Background(), testFlowIDService)

	s.Nil(result)
	s.Equal(&ErrorFlowNotFound, err)
}

func (s *FlowMgtServiceTestSuite) TestGetGraph_StoreError() {
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(nil, errors.New("db error"))

	result, err := s.service.GetGraph(context.Background(), testFlowIDService)

	s.Nil(result)
	s.Equal(&tidcommon.InternalServerError, err)
}

// IsValidFlow tests

func (s *FlowMgtServiceTestSuite) TestIsValidFlow_Success() {
	expectedFlow := &providers.CompleteFlowDefinition{
		ID:       testFlowIDService,
		FlowType: providers.FlowTypeAuthentication,
	}
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(expectedFlow, nil)

	result, svcErr := s.service.IsValidFlow(context.Background(), testFlowIDService, providers.FlowTypeAuthentication)

	s.Nil(svcErr)
	s.True(result)
}

func (s *FlowMgtServiceTestSuite) TestIsValidFlow_TypeMismatch() {
	expectedFlow := &providers.CompleteFlowDefinition{
		ID:       testFlowIDService,
		FlowType: providers.FlowTypeAuthentication,
	}
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(expectedFlow, nil)

	result, svcErr := s.service.IsValidFlow(context.Background(), testFlowIDService, providers.FlowTypeRegistration)

	s.Nil(svcErr)
	s.False(result)
}

func (s *FlowMgtServiceTestSuite) TestIsValidFlow_EmptyID() {
	result, svcErr := s.service.IsValidFlow(context.Background(), "", providers.FlowTypeAuthentication)

	s.Nil(svcErr)
	s.False(result)
}

func (s *FlowMgtServiceTestSuite) TestIsValidFlow_NotFound() {
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(nil, errFlowNotFound)

	result, svcErr := s.service.IsValidFlow(context.Background(), testFlowIDService, providers.FlowTypeAuthentication)

	s.Nil(svcErr)
	s.False(result)
}

func (s *FlowMgtServiceTestSuite) TestIsValidFlow_StoreError() {
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(nil, errors.New("db error"))

	result, svcErr := s.service.IsValidFlow(context.Background(), testFlowIDService, providers.FlowTypeAuthentication)

	s.NotNil(svcErr)
	s.Equal(tidcommon.ServerErrorType, svcErr.Type)
	s.False(result)
}

// TryInferRegistrationFlow Tests

func (s *FlowMgtServiceTestSuite) TestTryInferRegistrationFlow_Success() {
	// Enable auto-inference for this test
	testConfig := &config.Config{
		Flow: engineconfig.FlowConfig{
			AutoInferRegistration: true,
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("test", testConfig)

	mockExecutorRegistry := executormock.NewExecutorRegistryInterfaceMock(s.T())
	mockInterceptorRegistry := interceptormock.NewInterceptorRegistryInterfaceMock(s.T())
	mockValidator := NewFlowValidatorInterfaceMock(s.T())
	service := newFlowMgtService(s.mockStore, s.mockInference, s.mockGraphBuilder,
		mockExecutorRegistry, mockInterceptorRegistry, mockValidator, nil, &stubTransactioner{})

	authFlowDef := &FlowDefinition{
		Handle:   "auth-flow",
		Name:     "Auth Flow",
		FlowType: providers.FlowTypeAuthentication,
		Nodes: []providers.NodeDefinition{
			{ID: "start", Type: "START", OnSuccess: "end"},
			{ID: "end", Type: "END"},
		},
	}

	inferredRegFlow := &FlowDefinition{
		Handle:   "auth-flow-registration",
		Name:     "Auth Flow Registration",
		FlowType: providers.FlowTypeRegistration,
		Nodes: []providers.NodeDefinition{
			{ID: "start", Type: "START", OnSuccess: "executor"},
			{
				ID:   "executor",
				Type: "TASK_EXECUTION",
				Executor: &providers.ExecutorDefinition{
					Name: "UserTypeResolver",
				},
				OnSuccess: "end",
			},
			{ID: "end", Type: "END"},
		},
	}

	s.mockInference.On("InferRegistrationFlow", mock.Anything, authFlowDef).Return(inferredRegFlow, nil)

	completeFlow := &providers.CompleteFlowDefinition{
		Handle:   inferredRegFlow.Handle,
		Name:     inferredRegFlow.Name,
		FlowType: inferredRegFlow.FlowType,
		Nodes:    inferredRegFlow.Nodes,
	}
	s.mockStore.On("CreateFlow", mock.Anything, mock.AnythingOfType("string"),
		inferredRegFlow).Return(completeFlow, nil)

	service.(*flowMgtService).tryInferRegistrationFlow(context.Background(), "auth-flow-id", authFlowDef)

	s.mockInference.AssertExpectations(s.T())
	s.mockStore.AssertExpectations(s.T())
	mockExecutorRegistry.AssertNotCalled(s.T(), "GetExecutor")
}

func (s *FlowMgtServiceTestSuite) TestTryInferRegistrationFlow_SkipsNonAuthFlow() {
	// Enable auto-inference
	testConfig := &config.Config{
		Flow: engineconfig.FlowConfig{
			AutoInferRegistration: true,
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("test", testConfig)

	mockExecutorRegistry := executormock.NewExecutorRegistryInterfaceMock(s.T())
	mockInterceptorRegistry := interceptormock.NewInterceptorRegistryInterfaceMock(s.T())
	mockValidator := NewFlowValidatorInterfaceMock(s.T())
	service := newFlowMgtService(s.mockStore, s.mockInference, s.mockGraphBuilder,
		mockExecutorRegistry, mockInterceptorRegistry, mockValidator, nil, &stubTransactioner{})

	regFlowDef := &FlowDefinition{
		Handle:   "reg-flow",
		Name:     "Registration Flow",
		FlowType: providers.FlowTypeRegistration,
		Nodes:    []providers.NodeDefinition{},
	}

	service.(*flowMgtService).tryInferRegistrationFlow(context.Background(), "reg-flow-id", regFlowDef)

	s.mockInference.AssertNotCalled(s.T(), "InferRegistrationFlow")
	s.mockStore.AssertNotCalled(s.T(), "CreateFlow")
	mockExecutorRegistry.AssertNotCalled(s.T(), "GetExecutor")
}

func (s *FlowMgtServiceTestSuite) TestTryInferRegistrationFlow_HandlesInferenceError() {
	// Enable auto-inference
	testConfig := &config.Config{
		Flow: engineconfig.FlowConfig{
			AutoInferRegistration: true,
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("test", testConfig)

	mockExecutorRegistry := executormock.NewExecutorRegistryInterfaceMock(s.T())
	mockInterceptorRegistry := interceptormock.NewInterceptorRegistryInterfaceMock(s.T())
	mockValidator := NewFlowValidatorInterfaceMock(s.T())
	service := newFlowMgtService(s.mockStore, s.mockInference, s.mockGraphBuilder,
		mockExecutorRegistry, mockInterceptorRegistry, mockValidator, nil, &stubTransactioner{})

	authFlowDef := &FlowDefinition{
		Handle:   "auth-flow",
		Name:     "Auth Flow",
		FlowType: providers.FlowTypeAuthentication,
		Nodes:    []providers.NodeDefinition{},
	}

	s.mockInference.On("InferRegistrationFlow", mock.Anything, authFlowDef).Return(nil, errors.New("inference failed"))

	service.(*flowMgtService).tryInferRegistrationFlow(context.Background(), "auth-flow-id", authFlowDef)

	s.mockInference.AssertExpectations(s.T())
	s.mockStore.AssertNotCalled(s.T(), "CreateFlow")
	mockExecutorRegistry.AssertNotCalled(s.T(), "GetExecutor")
}

func (s *FlowMgtServiceTestSuite) TestTryInferRegistrationFlow_HandlesStoreError() {
	// Enable auto-inference
	testConfig := &config.Config{
		Flow: engineconfig.FlowConfig{
			AutoInferRegistration: true,
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("test", testConfig)

	mockExecutorRegistry := executormock.NewExecutorRegistryInterfaceMock(s.T())
	mockInterceptorRegistry := interceptormock.NewInterceptorRegistryInterfaceMock(s.T())
	mockValidator := NewFlowValidatorInterfaceMock(s.T())
	service := newFlowMgtService(s.mockStore, s.mockInference, s.mockGraphBuilder,
		mockExecutorRegistry, mockInterceptorRegistry, mockValidator, nil, &stubTransactioner{})

	authFlowDef := &FlowDefinition{
		Handle:   "auth-flow",
		Name:     "Auth Flow",
		FlowType: providers.FlowTypeAuthentication,
		Nodes:    []providers.NodeDefinition{},
	}

	inferredRegFlow := &FlowDefinition{
		Handle:   "auth-flow-registration",
		Name:     "Auth Flow Registration",
		FlowType: providers.FlowTypeRegistration,
		Nodes: []providers.NodeDefinition{
			{ID: "start", Type: "START", OnSuccess: "executor"},
			{
				ID:        "executor",
				Type:      "TASK_EXECUTION",
				Executor:  &providers.ExecutorDefinition{Name: "UserTypeResolver"},
				OnSuccess: "end",
			},
			{ID: "end", Type: "END"},
		},
	}

	s.mockInference.On("InferRegistrationFlow", mock.Anything, authFlowDef).Return(inferredRegFlow, nil)
	s.mockStore.On("CreateFlow", mock.Anything, mock.AnythingOfType("string"),
		inferredRegFlow).Return(nil, errors.New("store error"))

	service.(*flowMgtService).tryInferRegistrationFlow(context.Background(), "auth-flow-id", authFlowDef)

	s.mockInference.AssertExpectations(s.T())
	s.mockStore.AssertExpectations(s.T())
	mockExecutorRegistry.AssertNotCalled(s.T(), "GetExecutor")
}

func (s *FlowMgtServiceTestSuite) TestTryInferRegistrationFlow_DisabledAutoInference() {
	// Auto-inference is disabled in SetupTest, so just verify early return
	mockExecutorRegistry := executormock.NewExecutorRegistryInterfaceMock(s.T())
	mockInterceptorRegistry := interceptormock.NewInterceptorRegistryInterfaceMock(s.T())
	mockValidator := NewFlowValidatorInterfaceMock(s.T())
	service := newFlowMgtService(s.mockStore, s.mockInference, s.mockGraphBuilder,
		mockExecutorRegistry, mockInterceptorRegistry, mockValidator, nil, &stubTransactioner{})

	authFlowDef := &FlowDefinition{
		Handle:   "auth-flow",
		Name:     "Auth Flow",
		FlowType: providers.FlowTypeAuthentication,
		Nodes:    []providers.NodeDefinition{},
	}

	service.(*flowMgtService).tryInferRegistrationFlow(context.Background(), "auth-flow-id", authFlowDef)

	s.mockInference.AssertNotCalled(s.T(), "InferRegistrationFlow")
	s.mockStore.AssertNotCalled(s.T(), "CreateFlow")
	mockExecutorRegistry.AssertNotCalled(s.T(), "GetExecutor")
}

func (s *FlowMgtServiceTestSuite) TestTryInferRegistrationFlow_SkipsPasskeyRegistrationModes() {
	// Enable auto-inference for this test
	testConfig := &config.Config{
		Flow: engineconfig.FlowConfig{
			AutoInferRegistration: true,
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("test", testConfig)

	mockExecutorRegistry := executormock.NewExecutorRegistryInterfaceMock(s.T())
	mockInterceptorRegistry := interceptormock.NewInterceptorRegistryInterfaceMock(s.T())
	mockValidator := NewFlowValidatorInterfaceMock(s.T())
	service := newFlowMgtService(s.mockStore, s.mockInference, s.mockGraphBuilder,
		mockExecutorRegistry, mockInterceptorRegistry, mockValidator, nil, &stubTransactioner{})

	// Auth flow with PasskeyAuthExecutor in register_start and register_finish modes
	authFlowDef := &FlowDefinition{
		Handle:   "auth-flow-passkey",
		Name:     "Auth Flow With Passkey Registration",
		FlowType: providers.FlowTypeAuthentication,
		Nodes: []providers.NodeDefinition{
			{ID: "start", Type: "START", OnSuccess: "basic_auth"},
			{
				ID:        "basic_auth",
				Type:      "TASK_EXECUTION",
				Executor:  &providers.ExecutorDefinition{Name: "CredentialsAuthExecutor"},
				OnSuccess: "passkey_register_start",
			},
			{
				ID:   "passkey_register_start",
				Type: "TASK_EXECUTION",
				Executor: &providers.ExecutorDefinition{
					Name: "PasskeyAuthExecutor",
					Mode: "register_start",
				},
				OnSuccess: "passkey_register_finish",
			},
			{
				ID:   "passkey_register_finish",
				Type: "TASK_EXECUTION",
				Executor: &providers.ExecutorDefinition{
					Name: "PasskeyAuthExecutor",
					Mode: "register_finish",
				},
				OnSuccess: "end",
			},
			{ID: "end", Type: "END"},
		},
	}

	service.(*flowMgtService).tryInferRegistrationFlow(context.Background(), "auth-flow-id", authFlowDef)

	// InferRegistrationFlow and CreateFlow should NOT be called because
	// the auth flow already contains passkey registration modes
	s.mockInference.AssertNotCalled(s.T(), "InferRegistrationFlow")
	s.mockStore.AssertNotCalled(s.T(), "CreateFlow")
	mockExecutorRegistry.AssertNotCalled(s.T(), "GetExecutor")
}

// Immutability enforcement tests with composite mode disabled
// Note: These tests verify behavior when compositeStore is nil (composite mode not enabled).
// When composite mode is disabled, flows are always treated as mutable.
// For full declarative flow immutability testing, composite store mode must be enabled with
// proper file store configuration.

func (s *FlowMgtServiceTestSuite) TestUpdateFlow_CompositeDisabled_AllowsUpdate() {
	flowID := "declarative-flow"
	flowDef := &FlowDefinition{
		Handle:   "test-flow",
		Name:     "Test Flow",
		FlowType: providers.FlowTypeAuthentication,
		Nodes:    validFlowNodes(),
	}

	existingFlow := &providers.CompleteFlowDefinition{
		ID:            flowID,
		Handle:        "test-flow",
		Name:          "Test Flow",
		FlowType:      providers.FlowTypeAuthentication,
		ActiveVersion: 1,
		Nodes:         flowDef.Nodes,
	}

	// Mock the store to return the existing flow
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, flowID).Return(existingFlow, nil).Once()
	s.mockStore.EXPECT().UpdateFlow(mock.Anything, flowID, mock.Anything).Return(existingFlow, nil).Once()
	s.mockGraphBuilder.EXPECT().InvalidateCache(mock.Anything, flowID).Once()

	// Since compositeStore is nil in this test setup, isFlowDeclarative returns false
	// and the flow is treated as mutable, allowing the update
	result, err := s.service.UpdateFlow(context.Background(), flowID, flowDef)

	// Should succeed because compositeStore is nil (composite mode not enabled)
	s.Nil(err)
	s.NotNil(result)
}

func (s *FlowMgtServiceTestSuite) TestDeleteFlow_CompositeDisabled_AllowsDelete() {
	flowID := "declarative-flow"

	existingFlow := &providers.CompleteFlowDefinition{
		ID:            flowID,
		Handle:        "test-flow",
		Name:          "Test Flow",
		FlowType:      providers.FlowTypeAuthentication,
		ActiveVersion: 1,
		Nodes: []providers.NodeDefinition{
			{ID: "start", Type: "START"},
			{ID: "login", Type: "BASIC_AUTHENTICATION", OnSuccess: "end"},
			{ID: "end", Type: "END"},
		},
	}

	// Mock the store to return the existing flow
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, flowID).Return(existingFlow, nil).Once()
	s.mockStore.EXPECT().DeleteFlow(mock.Anything, flowID).Return(nil).Once()
	s.mockGraphBuilder.EXPECT().InvalidateCache(mock.Anything, flowID).Once()

	// Since compositeStore is nil in this test setup, isFlowDeclarative returns false
	// and the flow is treated as mutable, allowing the delete
	err := s.service.DeleteFlow(context.Background(), flowID)

	// Should succeed because compositeStore is nil (composite mode not enabled)
	s.Nil(err)
	s.mockStore.AssertExpectations(s.T())
}

func (s *FlowMgtServiceTestSuite) TestUpdateFlow_MutableFlowAllowed() {
	flowID := "mutable-flow"
	flowDef := &FlowDefinition{
		Handle:   "test-flow",
		Name:     "Updated Flow",
		FlowType: providers.FlowTypeAuthentication,
		Nodes:    validFlowNodes(),
	}

	existingFlow := &providers.CompleteFlowDefinition{
		ID:            flowID,
		Handle:        "test-flow",
		Name:          "Test Flow",
		FlowType:      providers.FlowTypeAuthentication,
		ActiveVersion: 1,
		Nodes:         flowDef.Nodes,
	}

	updatedFlow := &providers.CompleteFlowDefinition{
		ID:            flowID,
		Handle:        "test-flow",
		Name:          "Updated Flow",
		FlowType:      providers.FlowTypeAuthentication,
		ActiveVersion: 2,
		Nodes:         flowDef.Nodes,
	}

	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, flowID).Return(existingFlow, nil).Once()
	s.mockStore.EXPECT().UpdateFlow(mock.Anything, flowID, mock.MatchedBy(func(fd *FlowDefinition) bool {
		return fd.Name == "Updated Flow"
	})).Return(updatedFlow, nil).Once()
	s.mockGraphBuilder.EXPECT().InvalidateCache(mock.Anything, flowID).Once()

	result, err := s.service.UpdateFlow(context.Background(), flowID, flowDef)

	s.Nil(err)
	s.NotNil(result)
	s.Equal("Updated Flow", result.Name)
}

func (s *FlowMgtServiceTestSuite) TestDeleteFlow_MutableFlowAllowed() {
	flowID := "mutable-flow"

	existingFlow := &providers.CompleteFlowDefinition{
		ID:            flowID,
		Handle:        "test-flow",
		Name:          "Test Flow",
		FlowType:      providers.FlowTypeAuthentication,
		ActiveVersion: 1,
		Nodes: []providers.NodeDefinition{
			{ID: "start", Type: "START"},
			{ID: "login", Type: "BASIC_AUTHENTICATION", OnSuccess: "end"},
			{ID: "end", Type: "END"},
		},
	}

	s.mockStore.EXPECT().GetFlowByID(mock.Anything, flowID).Return(existingFlow, nil).Once()
	s.mockStore.EXPECT().DeleteFlow(mock.Anything, flowID).Return(nil).Once()
	s.mockGraphBuilder.EXPECT().InvalidateCache(mock.Anything, flowID).Return().Once()

	err := s.service.DeleteFlow(context.Background(), flowID)

	s.Nil(err)
	s.mockStore.AssertExpectations(s.T())
	s.mockGraphBuilder.AssertExpectations(s.T())
}

// GetResourceDependencies tests

func (s *FlowMgtServiceTestSuite) TestGetResourceDependencies_UnrelatedResourceTypeReturnsEmpty() {
	usages, err := s.service.GetResourceDependencies(
		context.Background(), resourcedependency.ResourceTypeUser, "user-1")

	s.NoError(err)
	s.Empty(usages)
}

func (s *FlowMgtServiceTestSuite) TestGetResourceDependencies_FlowReferencesIDP() {
	idpID := "idp-1"
	s.mockStore.EXPECT().ListActiveFlowsWithNodes(mock.Anything).Return([]*providers.CompleteFlowDefinition{
		{
			ID:   "flow1",
			Name: "Google Login",
			Nodes: []providers.NodeDefinition{
				{ID: "start", Type: "START"},
				{ID: "oidc", Type: "TASK_EXECUTION", Properties: map[string]interface{}{"idpId": idpID}},
			},
		},
		{
			ID:   "flow2",
			Name: "Basic Login",
			Nodes: []providers.NodeDefinition{
				{ID: "start", Type: "START"},
				{ID: "pw", Type: "TASK_EXECUTION", Properties: map[string]interface{}{"idpId": "other-idp"}},
			},
		},
	}, nil)

	usages, err := s.service.GetResourceDependencies(
		context.Background(), resourcedependency.ResourceTypeIDP, idpID)

	s.NoError(err)
	s.Len(usages, 1)
	s.Equal("flow1", usages[0].ID)
	s.Equal(resourcedependency.ResourceTypeFlow, usages[0].ResourceType)
	s.Equal("Google Login", usages[0].DisplayName)
	s.Equal(resourcedependency.BehaviorRestrict, usages[0].BehaviorOnDelete)
}

func (s *FlowMgtServiceTestSuite) TestGetResourceDependencies_FlowReferencesNotificationSender() {
	senderID := "sender-1"
	s.mockStore.EXPECT().ListActiveFlowsWithNodes(mock.Anything).Return([]*providers.CompleteFlowDefinition{
		{
			ID:   "flow1",
			Name: "SMS OTP",
			Nodes: []providers.NodeDefinition{
				{ID: "sms", Type: "TASK_EXECUTION", Properties: map[string]interface{}{"senderId": senderID}},
			},
		},
	}, nil)

	usages, err := s.service.GetResourceDependencies(
		context.Background(), resourcedependency.ResourceTypeNotificationSender, senderID)

	s.NoError(err)
	s.Len(usages, 1)
	s.Equal("flow1", usages[0].ID)
}

func (s *FlowMgtServiceTestSuite) TestGetResourceDependencies_NoReferencingFlows() {
	s.mockStore.EXPECT().ListActiveFlowsWithNodes(mock.Anything).Return([]*providers.CompleteFlowDefinition{
		{ID: "flow1", Nodes: []providers.NodeDefinition{{ID: "start", Type: "START"}}},
	}, nil)

	usages, err := s.service.GetResourceDependencies(
		context.Background(), resourcedependency.ResourceTypeIDP, "idp-1")

	s.NoError(err)
	s.Empty(usages)
}

func (s *FlowMgtServiceTestSuite) TestGetResourceDependencies_ListError() {
	s.mockStore.EXPECT().ListActiveFlowsWithNodes(mock.Anything).Return(nil, errors.New("db error"))

	usages, err := s.service.GetResourceDependencies(
		context.Background(), resourcedependency.ResourceTypeIDP, "idp-1")

	s.Error(err)
	s.Nil(usages)
}
