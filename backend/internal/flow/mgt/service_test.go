// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowmgt

import (
	"context"
	"errors"
	"fmt"
	"testing"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/connection"
	"github.com/thunder-id/thunderid/internal/flow/common"
	flowconfig "github.com/thunder-id/thunderid/internal/flow/config"
	"github.com/thunder-id/thunderid/internal/flow/executor"
	"github.com/thunder-id/thunderid/internal/notificationtemplate"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/tests/mocks/flow/executormock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/interceptormock"
	"github.com/thunder-id/thunderid/tests/mocks/sharingmock"
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
	mockResolver            *notificationTemplateProviderMock
	mockOU                  *ouProviderMock
	mockSharing             *sharingmock.SharingServiceInterfaceMock
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
	resp        *resourcedependency.DependenciesResponse
	err         error
	validateErr *tidcommon.ServiceError
}

func (r *stubDependencyRegistry) RegisterProvider(resourcedependency.Provider) {}

func (r *stubDependencyRegistry) GetDependencies(
	context.Context, string, string) (*resourcedependency.DependenciesResponse, error) {
	return r.resp, r.err
}

func (r *stubDependencyRegistry) CascadeDelete(context.Context, string, string) (int, error) {
	return 0, nil
}

func (r *stubDependencyRegistry) ValidateReferenceUpdate(
	context.Context, string, string) *tidcommon.ServiceError {
	return r.validateErr
}

func (s *FlowMgtServiceTestSuite) SetupTest() {
	s.mockStore = newFlowStoreInterfaceMock(s.T())
	s.mockInference = newFlowInferenceServiceInterfaceMock(s.T())
	s.mockGraphBuilder = newGraphBuilderInterfaceMock(s.T())
	s.mockExecutorRegistry = executormock.NewExecutorRegistryInterfaceMock(s.T())
	s.mockInterceptorRegistry = interceptormock.NewInterceptorRegistryInterfaceMock(s.T())
	s.mockValidator = NewFlowValidatorInterfaceMock(s.T())
	s.mockResolver = newNotificationTemplateProviderMock(s.T())
	s.mockOU = newOuProviderMock(s.T())
	s.mockOU.EXPECT().ResolveOwner(mock.Anything, "ou-1", "", mock.Anything).Return("ou-1", nil).Maybe()
	s.mockOU.EXPECT().ResolveOwnerForUpdate(mock.Anything, "ou-1", "", "ou-1", mock.Anything).
		Return("ou-1", nil).Maybe()
	s.mockOU.EXPECT().ResolveOwnerForUpdate(mock.Anything, "", "", "ou-1", mock.Anything).
		Return("ou-1", nil).Maybe()
	// A flow that was never shared has no policies, saved values or sharees, which is what most of these tests are
	// about. The tests of sharing replace the mock with their own.
	s.mockSharing = sharingmock.NewSharingServiceInterfaceMock(s.T())
	s.mockSharing.EXPECT().ListPolicies(mock.Anything, FlowSharingType, mock.Anything).
		Return([]sharing.Policy{}, nil).Maybe()
	s.mockSharing.EXPECT().ListOverlayValues(mock.Anything, FlowSharingType, mock.Anything).
		Return(map[string]map[string][]string{}, nil).Maybe()
	s.mockSharing.EXPECT().DeletePoliciesForResource(mock.Anything, FlowSharingType, mock.Anything).
		Return(nil).Maybe()
	s.mockSharing.EXPECT().IsVisible(mock.Anything, FlowSharingType, mock.Anything, mock.Anything).
		Return(false, nil).Maybe()
	s.service = newFlowMgtService(s.mockStore, s.mockInference, s.mockGraphBuilder,
		s.mockExecutorRegistry, s.mockInterceptorRegistry, s.mockValidator, nil, &stubTransactioner{}, nil, s.mockOU,
		s.mockResolver, nil, s.mockSharing, nil)

	// UpdateFlow / DeleteFlow / RestoreFlowVersion invalidate the store cache post-transaction.
	// The mock is applied here so individual tests don't need to repeat the expectation.
	s.mockStore.EXPECT().InvalidateCache(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()

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
		OUID:     "ou-1",
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
		OUID:     "ou-1",
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
				OUID:     "ou-1",
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
		OUID:     "ou-1",
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
		OUID:     "ou-1",
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
		OUID:          "ou-1",
	}
	flowDef := &FlowDefinition{
		Handle:   "test-handle",
		Name:     "Updated Flow",
		FlowType: providers.FlowTypeAuthentication,
		OUID:     "ou-1",
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
		OUID:     "ou-1",
		Interceptors: []providers.InterceptorDefinition{
			{Name: "OldInterceptor", Mode: providers.InterceptorModePreRequest},
		},
	}
	flowDef := &FlowDefinition{
		Handle:       "test-handle",
		Name:         "Updated Flow",
		FlowType:     providers.FlowTypeAuthentication,
		OUID:         "ou-1",
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
		OUID:     "ou-1",
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
		OUID:     "ou-1",
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
		OUID:     "ou-1",
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
		OUID:     "ou-1",
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
		OUID:     "ou-1",
	}
	flowDef := &FlowDefinition{
		Handle:   "test-handle",
		Name:     "Updated",
		FlowType: providers.FlowTypeAuthentication,
		OUID:     "ou-1",
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

func (s *FlowMgtServiceTestSuite) TestUpdateFlow_DependencyRegistryRejectsUpdate() {
	existingFlow := &providers.CompleteFlowDefinition{
		ID:       testFlowIDService,
		Handle:   "test-handle",
		FlowType: providers.FlowTypeAuthentication,
		OUID:     "ou-1",
	}
	flowDef := &FlowDefinition{
		Handle:   "test-handle",
		Name:     "Updated",
		FlowType: providers.FlowTypeAuthentication,
		OUID:     "ou-1",
		Nodes:    validFlowNodes(),
	}
	updatedFlow := &providers.CompleteFlowDefinition{Handle: "test-handle", Name: "Updated"}
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(existingFlow, nil)
	s.mockStore.EXPECT().UpdateFlow(mock.Anything, testFlowIDService, flowDef).Return(updatedFlow, nil)
	s.mockGraphBuilder.EXPECT().InvalidateCache(mock.Anything, testFlowIDService)

	validationErr := &tidcommon.ServiceError{Code: "X", Type: tidcommon.ClientErrorType}
	s.service.(*flowMgtService).dependencyRegistry = &stubDependencyRegistry{validateErr: validationErr}

	result, err := s.service.UpdateFlow(context.Background(), testFlowIDService, flowDef)

	s.Nil(result)
	// The dependent resource's error code must not leak; the caller sees a flow-scoped error.
	s.Require().NotNil(err)
	s.Equal(ErrorFlowUpdateBlockedByDependent.Code, err.Code)
}

func (s *FlowMgtServiceTestSuite) TestUpdateFlow_DependencyRegistryServerErrorMapsToInternal() {
	// A server-error from the dependency registry is not a client-side conflict, so it must not be
	// translated into ErrorFlowUpdateBlockedByDependent. It surfaces as an internal server error.
	existingFlow := &providers.CompleteFlowDefinition{
		ID:       testFlowIDService,
		Handle:   "test-handle",
		FlowType: providers.FlowTypeAuthentication,
		OUID:     "ou-1",
	}
	flowDef := &FlowDefinition{
		Handle:   "test-handle",
		Name:     "Updated",
		FlowType: providers.FlowTypeAuthentication,
		OUID:     "ou-1",
		Nodes:    validFlowNodes(),
	}
	updatedFlow := &providers.CompleteFlowDefinition{Handle: "test-handle", Name: "Updated"}
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(existingFlow, nil)
	s.mockStore.EXPECT().UpdateFlow(mock.Anything, testFlowIDService, flowDef).Return(updatedFlow, nil)
	s.mockGraphBuilder.EXPECT().InvalidateCache(mock.Anything, testFlowIDService)

	serverErr := &tidcommon.ServiceError{Code: "INTERNAL", Type: tidcommon.ServerErrorType}
	s.service.(*flowMgtService).dependencyRegistry = &stubDependencyRegistry{validateErr: serverErr}

	result, err := s.service.UpdateFlow(context.Background(), testFlowIDService, flowDef)

	s.Nil(result)
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (s *FlowMgtServiceTestSuite) TestUpdateFlow_DependencyRegistryPasses() {
	existingFlow := &providers.CompleteFlowDefinition{
		ID:       testFlowIDService,
		Handle:   "test-handle",
		FlowType: providers.FlowTypeAuthentication,
		OUID:     "ou-1",
	}
	flowDef := &FlowDefinition{
		Handle:   "test-handle",
		Name:     "Updated",
		FlowType: providers.FlowTypeAuthentication,
		OUID:     "ou-1",
		Nodes:    validFlowNodes(),
	}
	updatedFlow := &providers.CompleteFlowDefinition{Handle: "test-handle", Name: "Updated"}
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(existingFlow, nil)
	s.mockStore.EXPECT().UpdateFlow(mock.Anything, testFlowIDService, flowDef).Return(updatedFlow, nil)
	s.mockGraphBuilder.EXPECT().InvalidateCache(mock.Anything, testFlowIDService)

	s.service.(*flowMgtService).dependencyRegistry = &stubDependencyRegistry{}

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
		OUID:     "ou-1",
	}
	flowDef := &FlowDefinition{
		Handle:   "test-handle",
		Name:     "Test",
		FlowType: providers.FlowTypeAuthentication,
		OUID:     "ou-1",
		Nodes:    validFlowNodes(),
	}
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(existingFlow, nil)
	s.mockStore.EXPECT().UpdateFlow(mock.Anything, testFlowIDService, flowDef).Return(nil, errors.New("db error"))
	// Store write was attempted (and may have applied partial writes rolled back by the tx), so
	// the graph cache is invalidated regardless of the store error.
	s.mockGraphBuilder.EXPECT().InvalidateCache(mock.Anything, testFlowIDService)

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
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).
		Return(&providers.CompleteFlowDefinition{ID: testFlowIDService}, nil)
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
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).
		Return(&providers.CompleteFlowDefinition{ID: testFlowIDService}, nil)
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
		mockExecutorRegistry, mockInterceptorRegistry, mockValidator,
		nil, &stubTransactioner{}, nil, nil, nil, nil, s.mockSharing, nil)

	authFlowDef := &FlowDefinition{
		Handle:   "auth-flow",
		Name:     "Auth Flow",
		FlowType: providers.FlowTypeAuthentication,
		OUID:     "ou-1",
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

	service.tryInferRegistrationFlow(context.Background(), "auth-flow-id", authFlowDef)

	s.mockInference.AssertExpectations(s.T())
	s.mockStore.AssertExpectations(s.T())
	mockExecutorRegistry.AssertNotCalled(s.T(), "GetExecutor")
	s.Equal("ou-1", inferredRegFlow.OUID, "the inferred flow belongs to the OU of its authentication flow")
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
		mockExecutorRegistry, mockInterceptorRegistry, mockValidator,
		nil, &stubTransactioner{}, nil, nil, nil, nil, s.mockSharing, nil)

	regFlowDef := &FlowDefinition{
		Handle:   "reg-flow",
		Name:     "Registration Flow",
		FlowType: providers.FlowTypeRegistration,
		Nodes:    []providers.NodeDefinition{},
	}

	service.tryInferRegistrationFlow(context.Background(), "reg-flow-id", regFlowDef)

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
		mockExecutorRegistry, mockInterceptorRegistry, mockValidator,
		nil, &stubTransactioner{}, nil, nil, nil, nil, s.mockSharing, nil)

	authFlowDef := &FlowDefinition{
		Handle:   "auth-flow",
		Name:     "Auth Flow",
		FlowType: providers.FlowTypeAuthentication,
		Nodes:    []providers.NodeDefinition{},
	}

	s.mockInference.On("InferRegistrationFlow", mock.Anything, authFlowDef).Return(nil, errors.New("inference failed"))

	service.tryInferRegistrationFlow(context.Background(), "auth-flow-id", authFlowDef)

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
		mockExecutorRegistry, mockInterceptorRegistry, mockValidator,
		nil, &stubTransactioner{}, nil, nil, nil, nil, s.mockSharing, nil)

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

	service.tryInferRegistrationFlow(context.Background(), "auth-flow-id", authFlowDef)

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
		mockExecutorRegistry, mockInterceptorRegistry, mockValidator,
		nil, &stubTransactioner{}, nil, nil, nil, nil, s.mockSharing, nil)

	authFlowDef := &FlowDefinition{
		Handle:   "auth-flow",
		Name:     "Auth Flow",
		FlowType: providers.FlowTypeAuthentication,
		Nodes:    []providers.NodeDefinition{},
	}

	service.tryInferRegistrationFlow(context.Background(), "auth-flow-id", authFlowDef)

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
		mockExecutorRegistry, mockInterceptorRegistry, mockValidator,
		nil, &stubTransactioner{}, nil, nil, nil, nil, s.mockSharing, nil)

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

	service.tryInferRegistrationFlow(context.Background(), "auth-flow-id", authFlowDef)

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
		OUID:     "ou-1",
		Nodes:    validFlowNodes(),
	}

	existingFlow := &providers.CompleteFlowDefinition{
		ID:            flowID,
		Handle:        "test-flow",
		Name:          "Test Flow",
		FlowType:      providers.FlowTypeAuthentication,
		ActiveVersion: 1,
		OUID:          "ou-1",
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
		OUID:     "ou-1",
		Nodes:    validFlowNodes(),
	}

	existingFlow := &providers.CompleteFlowDefinition{
		ID:            flowID,
		Handle:        "test-flow",
		Name:          "Test Flow",
		FlowType:      providers.FlowTypeAuthentication,
		ActiveVersion: 1,
		OUID:          "ou-1",
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

func connectionVariableFlow(id, defaultConnection string) *providers.CompleteFlowDefinition {
	return &providers.CompleteFlowDefinition{
		ID: id, Name: "Flow " + id, OUID: "ou-1",
		Nodes: []providers.NodeDefinition{
			{ID: "start", Type: "START", OnSuccess: "vars"},
			{ID: "vars", Type: "VARIABLES", OnSuccess: "end", Inputs: []providers.VariableDefinition{{
				Identifier: "social", Type: providers.InputTypeConnectionSelect, Category: "IDENTITY_PROVIDER",
				DefaultValue: defaultConnection,
			}}},
			{ID: "end", Type: "END"},
		},
	}
}

func connectionPolicy(terms sharing.OverlayRule) sharing.Policy {
	return sharing.Policy{
		ID: "p1", InitiatingOUID: "ou-1", ChildUnits: &sharing.ChildUnits{Mode: sharing.ChildUnitsAll},
		Rules: []sharing.StoredRule{{
			FieldKey: sharingKey("social"), Grant: sharing.GrantRef{Section: sharing.SectionChildUnits},
			Requested: terms, Resolved: terms,
		}},
	}
}

func (s *FlowMgtServiceTestSuite) connectionUsages() []resourcedependency.ResourceDependency {
	usages, err := s.service.GetResourceDependencies(
		context.Background(), resourcedependency.ResourceTypeIDP, "idp-1")
	s.Require().NoError(err)
	return usages
}

// A flow that names a connection as the default of a variable depends on it, as one that names it in a node does.
func (s *FlowMgtServiceTestSuite) TestGetResourceDependencies_FlowVariableDefaultNamesTheConnection() {
	s.mockStore.EXPECT().ListActiveFlowsWithNodes(mock.Anything).Return([]*providers.CompleteFlowDefinition{
		connectionVariableFlow("flow-1", "idp-1"), connectionVariableFlow("flow-2", "idp-2"),
	}, nil)
	sharingSvc := sharingmock.NewSharingServiceInterfaceMock(s.T())
	s.withSharing(sharingSvc)
	sharingSvc.EXPECT().ListPolicies(mock.Anything, FlowSharingType, "flow-2").Return([]sharing.Policy{}, nil)
	sharingSvc.EXPECT().ListOverlayValues(mock.Anything, FlowSharingType, "flow-2").
		Return(map[string]map[string][]string{}, nil)

	usages := s.connectionUsages()

	s.Require().Len(usages, 1)
	s.Equal("flow-1", usages[0].ID)
	s.Equal(resourcedependency.ResourceTypeFlow, usages[0].ResourceType)
	s.Equal(resourcedependency.BehaviorRestrict, usages[0].BehaviorOnDelete)
}

// A connection a sharing policy of the flow gives as a value, or as one to choose from, is in use. One it withholds is
// not.
func (s *FlowMgtServiceTestSuite) TestGetResourceDependencies_PolicyValuesNameTheConnection() {
	cases := map[string]struct {
		terms sharing.OverlayRule
		used  bool
	}{
		"as the value":         {sharing.OverlayRule{Editable: false, Value: strs("idp-1")}, true},
		"as an allowed value":  {sharing.OverlayRule{Editable: true, AllowedValues: strs("idp-2", "idp-1")}, true},
		"as an excluded value": {sharing.OverlayRule{Editable: true, ExcludedValues: strs("idp-1")}, false},
		"another connection":   {sharing.OverlayRule{Editable: true, Value: strs("idp-2")}, false},
		"no value in the rule": {sharing.OverlayRule{Editable: true}, false},
	}
	for name, tc := range cases {
		s.Run(name, func() {
			s.SetupTest()
			sharingSvc := sharingmock.NewSharingServiceInterfaceMock(s.T())
			s.withSharing(sharingSvc)
			s.mockStore.EXPECT().ListActiveFlowsWithNodes(mock.Anything).
				Return([]*providers.CompleteFlowDefinition{connectionVariableFlow("flow-1", "idp-9")}, nil)
			sharingSvc.EXPECT().ListPolicies(mock.Anything, FlowSharingType, "flow-1").
				Return([]sharing.Policy{connectionPolicy(tc.terms)}, nil)
			sharingSvc.EXPECT().ListOverlayValues(mock.Anything, FlowSharingType, "flow-1").
				Return(map[string]map[string][]string{}, nil).Maybe()

			s.Equal(tc.used, len(s.connectionUsages()) == 1)
		})
	}
}

// A value an organization unit saved for the variable is in use, and the flow is listed once however it uses it.
func (s *FlowMgtServiceTestSuite) TestGetResourceDependencies_SavedValueNamesTheConnection() {
	sharingSvc := sharingmock.NewSharingServiceInterfaceMock(s.T())
	s.withSharing(sharingSvc)
	s.mockStore.EXPECT().ListActiveFlowsWithNodes(mock.Anything).
		Return([]*providers.CompleteFlowDefinition{connectionVariableFlow("flow-1", "idp-9")}, nil)
	sharingSvc.EXPECT().ListPolicies(mock.Anything, FlowSharingType, "flow-1").
		Return([]sharing.Policy{connectionPolicy(sharing.OverlayRule{Editable: true})}, nil)
	sharingSvc.EXPECT().ListOverlayValues(mock.Anything, FlowSharingType, "flow-1").
		Return(map[string]map[string][]string{
			"ou-child": {sharingKey("social"): {"idp-1"}, sharingKey("other"): {"idp-1"}},
		}, nil)

	s.Len(s.connectionUsages(), 1)
}

func (s *FlowMgtServiceTestSuite) TestGetResourceDependencies_ListedOnceWhateverWaysTheFlowUsesTheConnection() {
	sharingSvc := sharingmock.NewSharingServiceInterfaceMock(s.T())
	s.withSharing(sharingSvc)
	flow := connectionVariableFlow("flow-1", "idp-1")
	flow.Nodes[0].Properties = map[string]interface{}{"idpId": "idp-1"}
	s.mockStore.EXPECT().ListActiveFlowsWithNodes(mock.Anything).
		Return([]*providers.CompleteFlowDefinition{flow}, nil)

	s.Len(s.connectionUsages(), 1)
}

// Usage that cannot be read is an error, so the delete is blocked rather than allowed on a guess.
func (s *FlowMgtServiceTestSuite) TestGetResourceDependencies_FailsWhenWhatUnitsSavedCannotBeRead() {
	sharingSvc := sharingmock.NewSharingServiceInterfaceMock(s.T())
	s.withSharing(sharingSvc)
	s.mockStore.EXPECT().ListActiveFlowsWithNodes(mock.Anything).
		Return([]*providers.CompleteFlowDefinition{connectionVariableFlow("flow-1", "idp-9")}, nil)
	sharingSvc.EXPECT().ListPolicies(mock.Anything, FlowSharingType, "flow-1").
		Return([]sharing.Policy{}, nil)
	sharingSvc.EXPECT().ListOverlayValues(mock.Anything, FlowSharingType, "flow-1").
		Return(nil, &tidcommon.InternalServerError)

	usages, err := s.service.GetResourceDependencies(
		context.Background(), resourcedependency.ResourceTypeIDP, "idp-1")

	s.Error(err)
	s.Nil(usages)
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

func (s *FlowMgtServiceTestSuite) TestGetResourceDependencies_CountsAConnectionIDWithSurroundingWhitespace() {
	s.mockStore.EXPECT().ListActiveFlowsWithNodes(mock.Anything).Return([]*providers.CompleteFlowDefinition{
		{
			ID:   "flow1",
			Name: "Padded reference",
			Nodes: []providers.NodeDefinition{
				{ID: "risk", Type: "TASK_EXECUTION", Executor: &providers.ExecutorDefinition{Name: "ExtensionExecutor"},
					Properties: map[string]interface{}{"connectionId": "  http-conn-1\t"}},
			},
		},
	}, nil)

	usages, err := s.service.GetResourceDependencies(
		context.Background(), resourcedependency.ResourceTypeHTTPConnection, "http-conn-1")

	s.NoError(err)
	s.Require().Len(usages, 1)
	s.Equal("flow1", usages[0].ID)
}

func (s *FlowMgtServiceTestSuite) TestGetResourceDependencies_ExtensionStepReferencesHTTPConnection() {
	connectionID := "http-conn-1"
	s.mockStore.EXPECT().ListActiveFlowsWithNodes(mock.Anything).Return([]*providers.CompleteFlowDefinition{
		{
			ID:   "flow1",
			Name: "Default sign-in",
			Nodes: []providers.NodeDefinition{
				{ID: "risk", Type: "TASK_EXECUTION", Executor: &providers.ExecutorDefinition{Name: "ExtensionExecutor"},
					Properties: map[string]interface{}{"connectionId": connectionID}},
			},
		},
		{
			ID:   "flow3",
			Name: "Not an extension step",
			Nodes: []providers.NodeDefinition{
				{ID: "call", Type: "TASK_EXECUTION",
					Executor:   &providers.ExecutorDefinition{Name: "HTTPRequestExecutor"},
					Properties: map[string]interface{}{"connectionId": connectionID}},
				{ID: "bare", Type: "TASK_EXECUTION", Properties: map[string]interface{}{"connectionId": connectionID}},
			},
		},
		{
			ID:   "flow2",
			Name: "Other extension",
			Nodes: []providers.NodeDefinition{
				{ID: "risk", Type: "TASK_EXECUTION", Executor: &providers.ExecutorDefinition{Name: "ExtensionExecutor"},
					Properties: map[string]interface{}{"connectionId": "other-conn"}},
			},
		},
	}, nil)

	usages, err := s.service.GetResourceDependencies(
		context.Background(), resourcedependency.ResourceTypeHTTPConnection, connectionID)

	s.NoError(err)
	s.Equal([]resourcedependency.ResourceDependency{{
		ResourceType: resourcedependency.ResourceTypeFlow, ID: "flow1", DisplayName: "Default sign-in",
		BehaviorOnDelete: resourcedependency.BehaviorRestrict,
	}}, usages)
}

func (s *FlowMgtServiceTestSuite) TestGetResourceDependencies_FlowReferencesOU() {
	ouID := "ou-parent"
	s.mockStore.EXPECT().ListFlows(mock.Anything, maxPageSize, 0, "").Return([]BasicFlowDefinition{}, 0, nil)
	s.mockStore.EXPECT().ListActiveFlowsWithNodes(mock.Anything).Return([]*providers.CompleteFlowDefinition{
		{
			ID:   "flow1",
			Name: "Sign In",
			Nodes: []providers.NodeDefinition{
				{ID: "ou", Type: "TASK_EXECUTION", Properties: map[string]interface{}{"parentOuId": ouID}},
			},
		},
		{
			ID:   "flow2",
			Name: "Other Sign In",
			Nodes: []providers.NodeDefinition{
				{ID: "ou", Type: "TASK_EXECUTION", Properties: map[string]interface{}{"parentOuId": "other-ou"}},
			},
		},
	}, nil)

	usages, err := s.service.GetResourceDependencies(
		context.Background(), resourcedependency.ResourceTypeOU, ouID)

	s.NoError(err)
	s.Len(usages, 1)
	s.Equal("flow1", usages[0].ID)
	s.Equal(resourcedependency.ResourceTypeFlow, usages[0].ResourceType)
	s.Equal(resourcedependency.BehaviorRestrict, usages[0].BehaviorOnDelete)
}

func (s *FlowMgtServiceTestSuite) TestGetResourceDependencies_OUListsOwnedAndReferencingFlowsOnce() {
	ouID := "ou-parent"
	s.mockStore.EXPECT().ListFlows(mock.Anything, maxPageSize, 0, "").Return([]BasicFlowDefinition{
		{ID: "owned-only", Name: "Owned", OUID: ouID},
		{ID: "both", Name: "Both", OUID: ouID},
	}, 2, nil)
	s.mockStore.EXPECT().ListActiveFlowsWithNodes(mock.Anything).Return([]*providers.CompleteFlowDefinition{
		{ID: "both", Name: "Both", Nodes: []providers.NodeDefinition{
			{ID: "ou", Type: "TASK_EXECUTION", Properties: map[string]interface{}{"parentOuId": ouID}}}},
		{ID: "referencing-only", Name: "Referencing", Nodes: []providers.NodeDefinition{
			{ID: "ou", Type: "TASK_EXECUTION", Properties: map[string]interface{}{"parentOuId": ouID}}}},
	}, nil)

	usages, err := s.service.GetResourceDependencies(
		context.Background(), resourcedependency.ResourceTypeOU, ouID)

	s.NoError(err)
	ids := make([]string, 0, len(usages))
	for _, usage := range usages {
		ids = append(ids, usage.ID)
	}
	s.Equal([]string{"owned-only", "both", "referencing-only"}, ids)
}

func (s *FlowMgtServiceTestSuite) TestGetResourceDependencies_OUReportsAFailureToScanReferences() {
	s.mockStore.EXPECT().ListFlows(mock.Anything, maxPageSize, 0, "").Return([]BasicFlowDefinition{}, 0, nil)
	s.mockStore.EXPECT().ListActiveFlowsWithNodes(mock.Anything).Return(nil, errors.New("db down"))

	usages, err := s.service.GetResourceDependencies(
		context.Background(), resourcedependency.ResourceTypeOU, "ou-parent")

	s.Nil(usages)
	s.Error(err)
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

func (s *FlowMgtServiceTestSuite) TestGetResourceDependencies_FlowReferencesEmailTemplate() {
	s.mockResolver.EXPECT().GetTemplateByHandle(mock.Anything, notificationtemplate.ChannelTypeEmail, "user-invite").
		Return(&notificationtemplate.Template{ID: "tmpl-1", Handle: "user-invite"}, nil)
	s.mockStore.EXPECT().ListActiveFlowsWithNodes(mock.Anything).Return([]*providers.CompleteFlowDefinition{
		{
			ID:   "flow1",
			Name: "Invite",
			Nodes: []providers.NodeDefinition{
				{
					ID:         "email",
					Type:       "TASK_EXECUTION",
					Properties: map[string]interface{}{"emailTemplate": "user-invite"},
				},
			},
		},
	}, nil)

	usages, err := s.service.GetResourceDependencies(
		context.Background(), resourcedependency.ResourceTypeNotificationTemplate, "tmpl-1")

	s.NoError(err)
	s.Len(usages, 1)
	s.Equal("flow1", usages[0].ID)
	s.Equal(resourcedependency.BehaviorRestrict, usages[0].BehaviorOnDelete)
}

// A node referencing the handle in the other channel must not match: email/otp and sms/otp are
// distinct templates, disambiguated by the emailTemplate vs smsTemplate property.
func (s *FlowMgtServiceTestSuite) TestGetResourceDependencies_TemplateChannelDisambiguation() {
	// The flow node only carries smsTemplate, so only the SMS handle is resolved.
	s.mockResolver.EXPECT().GetTemplateByHandle(mock.Anything, notificationtemplate.ChannelTypeSMS, "otp").
		Return(&notificationtemplate.Template{ID: "sms-otp-id", Handle: "otp"}, nil)
	s.mockStore.EXPECT().ListActiveFlowsWithNodes(mock.Anything).Return([]*providers.CompleteFlowDefinition{
		{
			ID:   "flow1",
			Name: "SMS OTP",
			Nodes: []providers.NodeDefinition{
				{ID: "sms", Type: "TASK_EXECUTION", Properties: map[string]interface{}{"smsTemplate": "otp"}},
			},
		},
	}, nil)

	// Deleting the email/otp template is not blocked by a flow that only uses the sms/otp template.
	usages, err := s.service.GetResourceDependencies(
		context.Background(), resourcedependency.ResourceTypeNotificationTemplate, "email-otp-id")

	s.NoError(err)
	s.Empty(usages)
}

func (s *FlowMgtServiceTestSuite) TestGetResourceDependencies_TemplateNoResolverFailsClosed() {
	// A missing resolver is a wiring fault: fail closed (error) so the delete is blocked rather than
	// letting a flow-referenced template be removed.
	s.service.(*flowMgtService).templateResolver = nil

	usages, err := s.service.GetResourceDependencies(
		context.Background(), resourcedependency.ResourceTypeNotificationTemplate, "tmpl-1")

	s.Error(err)
	s.Nil(usages)
}

// buildFlowDef builds a minimal CompleteFlowDefinition with the given ID, type, and CALL-node
// target IDs. Non-CALL structure (START/END) is omitted since the walker doesn't need it.
func buildFlowDef(id string, flowType providers.FlowType, callTargets ...string) *providers.CompleteFlowDefinition {
	nodes := make([]providers.NodeDefinition, 0, len(callTargets))
	for i, targetID := range callTargets {
		nodes = append(nodes, providers.NodeDefinition{
			ID:   "call-" + string(rune('a'+i)),
			Type: string(common.NodeTypeCall),
			Flow: &providers.FlowReferenceDefinition{Ref: targetID},
		})
	}
	return &providers.CompleteFlowDefinition{ID: id, FlowType: flowType, Nodes: nodes}
}

func (s *FlowMgtServiceTestSuite) TestGetReachableCallTargets_Empty() {
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, "auth").
		Return(buildFlowDef("auth", providers.FlowTypeAuthentication), nil)

	targets, err := s.service.GetReachableCallTargets(context.Background(), "auth")
	s.Require().Nil(err)
	s.Empty(targets)
}

func (s *FlowMgtServiceTestSuite) TestGetReachableCallTargets_DirectRegistration() {
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, "auth").
		Return(buildFlowDef("auth", providers.FlowTypeAuthentication, "reg"), nil)
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, "reg").
		Return(buildFlowDef("reg", providers.FlowTypeRegistration), nil)

	targets, err := s.service.GetReachableCallTargets(context.Background(), "auth")
	s.Require().Nil(err)
	s.Equal([]CallTarget{{FlowID: "reg", FlowType: providers.FlowTypeRegistration}}, targets)
}

func (s *FlowMgtServiceTestSuite) TestGetReachableCallTargets_TransitiveRecovery() {
	// auth -> reg -> recovery. Walker must reach recovery via reg.
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, "auth").
		Return(buildFlowDef("auth", providers.FlowTypeAuthentication, "reg"), nil)
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, "reg").
		Return(buildFlowDef("reg", providers.FlowTypeRegistration, "rec"), nil)
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, "rec").
		Return(buildFlowDef("rec", providers.FlowTypeRecovery), nil)

	targets, err := s.service.GetReachableCallTargets(context.Background(), "auth")
	s.Require().Nil(err)
	s.Len(targets, 2)
	byID := map[string]providers.FlowType{}
	for _, t := range targets {
		byID[t.FlowID] = t.FlowType
	}
	s.Equal(providers.FlowTypeRegistration, byID["reg"])
	s.Equal(providers.FlowTypeRecovery, byID["rec"])
}

func (s *FlowMgtServiceTestSuite) TestGetReachableCallTargets_CycleSafe() {
	// auth -> reg -> auth (cycle). Walker must terminate and not double-emit auth.
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, "auth").
		Return(buildFlowDef("auth", providers.FlowTypeAuthentication, "reg"), nil)
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, "reg").
		Return(buildFlowDef("reg", providers.FlowTypeRegistration, "auth"), nil)

	targets, err := s.service.GetReachableCallTargets(context.Background(), "auth")
	s.Require().Nil(err)
	// Only reg is in the results (starting flow "auth" is excluded, and the cycle re-visit is skipped).
	s.Equal([]CallTarget{{FlowID: "reg", FlowType: providers.FlowTypeRegistration}}, targets)
}

func (s *FlowMgtServiceTestSuite) TestGetReachableCallTargets_MissingStart() {
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, "missing").
		Return(nil, errFlowNotFound)

	targets, err := s.service.GetReachableCallTargets(context.Background(), "missing")
	s.Nil(targets)
	s.Require().NotNil(err)
	s.Equal(ErrorFlowNotFound.Code, err.Code)
}

func (s *FlowMgtServiceTestSuite) TestGetReachableCallTargets_MissingCallTarget() {
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, "auth").
		Return(buildFlowDef("auth", providers.FlowTypeAuthentication, "ghost"), nil)
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, "ghost").
		Return(nil, errFlowNotFound)

	targets, err := s.service.GetReachableCallTargets(context.Background(), "auth")
	s.Nil(targets)
	s.Require().NotNil(err)
	s.Equal(ErrorCallTargetFlowNotFound.Code, err.Code)
}

func (s *FlowMgtServiceTestSuite) TestGetReachableCallTargets_IgnoresBlankRefs() {
	def := buildFlowDef("auth", providers.FlowTypeAuthentication)
	def.Nodes = append(def.Nodes, providers.NodeDefinition{
		ID:   "call-empty",
		Type: string(common.NodeTypeCall),
		Flow: &providers.FlowReferenceDefinition{Ref: ""},
	})
	def.Nodes = append(def.Nodes, providers.NodeDefinition{
		ID:   "call-noflow",
		Type: string(common.NodeTypeCall),
		Flow: nil,
	})

	s.mockStore.EXPECT().GetFlowByID(mock.Anything, "auth").Return(def, nil)

	targets, err := s.service.GetReachableCallTargets(context.Background(), "auth")
	s.Require().Nil(err)
	s.Empty(targets)
}

func (s *FlowMgtServiceTestSuite) TestGetReachableCallTargets_EmptyFlowID() {
	targets, err := s.service.GetReachableCallTargets(context.Background(), "")
	s.Nil(targets)
	s.Require().NotNil(err)
	s.Equal(ErrorMissingFlowID.Code, err.Code)
}

func (s *FlowMgtServiceTestSuite) TestGetReachableCallTargets_StoreErrorMapsToInternal() {
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, "auth").Return(nil, errors.New("db down"))

	targets, err := s.service.GetReachableCallTargets(context.Background(), "auth")
	s.Nil(targets)
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

// ----- ResolveEffectiveFlowID -----

func (s *FlowMgtServiceTestSuite) TestResolveEffectiveFlowID_OverriddenIDWins() {
	svc := newFlowMgtService(s.mockStore, s.mockInference, s.mockGraphBuilder,
		s.mockExecutorRegistry, s.mockInterceptorRegistry, s.mockValidator, nil, &stubTransactioner{},
		nil, nil, nil, nil, s.mockSharing, nil)

	id, svcErr := svc.ResolveEffectiveFlowID(
		context.Background(), "override-id", "ou-1", providers.FlowTypeAuthentication)

	s.Nil(svcErr)
	s.Equal("override-id", id)
}

func (s *FlowMgtServiceTestSuite) TestResolveEffectiveFlowID_OUFlowIDUsedWhenNoOverride() {
	mockOU := newOuProviderMock(s.T())
	svc := newFlowMgtService(s.mockStore, s.mockInference, s.mockGraphBuilder,
		s.mockExecutorRegistry, s.mockInterceptorRegistry, s.mockValidator, nil, &stubTransactioner{},
		nil, mockOU, nil, nil, s.mockSharing, nil)

	ou := ou.OrganizationUnit{AuthFlowID: "ou-auth-flow"}
	mockOU.EXPECT().GetOrganizationUnit(mock.Anything, "ou-1").Return(ou, nil)

	id, svcErr := svc.ResolveEffectiveFlowID(context.Background(), "", "ou-1", providers.FlowTypeAuthentication)

	s.Nil(svcErr)
	s.Equal("ou-auth-flow", id)
}

func (s *FlowMgtServiceTestSuite) TestResolveEffectiveFlowID_ServerDefaultUsedWhenOUHasNoFlowID() {
	mockOU := newOuProviderMock(s.T())
	mockSC := newServerConfigProviderMock(s.T())
	svc := newFlowMgtService(s.mockStore, s.mockInference, s.mockGraphBuilder,
		s.mockExecutorRegistry, s.mockInterceptorRegistry, s.mockValidator, nil, &stubTransactioner{},
		mockSC, mockOU, nil, nil, s.mockSharing, nil)

	mockOU.EXPECT().GetOrganizationUnit(mock.Anything, "ou-1").Return(ou.OrganizationUnit{}, nil)

	flowconfig := flowconfig.FlowSectionConfig{
		AuthFlow: flowconfig.FlowTypeConfig{DefaultHandle: "default-auth"},
	}
	mockSC.EXPECT().GetMergedConfig(mock.Anything, "flow").Return(flowconfig, nil)

	completeFlow := &providers.CompleteFlowDefinition{
		ID: "server-default-id", Handle: "default-auth", FlowType: providers.FlowTypeAuthentication,
	}
	s.mockStore.EXPECT().GetFlowByHandle(
		mock.Anything, "default-auth", providers.FlowTypeAuthentication,
	).Return(completeFlow, nil)

	id, svcErr := svc.ResolveEffectiveFlowID(context.Background(), "", "ou-1", providers.FlowTypeAuthentication)

	s.Nil(svcErr)
	s.Equal("server-default-id", id)
}

func (s *FlowMgtServiceTestSuite) TestResolveEffectiveFlowID_EmptyWhenNoDefaultConfigured() {
	mockSC := newServerConfigProviderMock(s.T())
	svc := newFlowMgtService(s.mockStore, s.mockInference, s.mockGraphBuilder,
		s.mockExecutorRegistry, s.mockInterceptorRegistry, s.mockValidator, nil, &stubTransactioner{},
		mockSC, nil, nil, nil, s.mockSharing, nil)

	mockSC.EXPECT().GetMergedConfig(mock.Anything, "flow").Return(flowconfig.FlowSectionConfig{}, nil)

	id, svcErr := svc.ResolveEffectiveFlowID(context.Background(), "", "", providers.FlowTypeRegistration)

	s.Nil(svcErr)
	s.Empty(id)
}

func (s *FlowMgtServiceTestSuite) TestResolveEffectiveFlowID_OULookupErrorFallsThrough() {
	mockOU := newOuProviderMock(s.T())
	mockSC := newServerConfigProviderMock(s.T())
	svc := newFlowMgtService(s.mockStore, s.mockInference, s.mockGraphBuilder,
		s.mockExecutorRegistry, s.mockInterceptorRegistry, s.mockValidator, nil, &stubTransactioner{},
		mockSC, mockOU, nil, nil, s.mockSharing, nil)

	mockOU.EXPECT().GetOrganizationUnit(mock.Anything, "ou-1").
		Return(ou.OrganizationUnit{}, &tidcommon.InternalServerError)
	mockSC.EXPECT().GetMergedConfig(mock.Anything, "flow").Return(flowconfig.FlowSectionConfig{}, nil)

	id, svcErr := svc.ResolveEffectiveFlowID(context.Background(), "", "ou-1", providers.FlowTypeAuthentication)

	s.Nil(svcErr)
	s.Empty(id)
}

// ----- ouFlowIDForType -----

func (s *FlowMgtServiceTestSuite) TestOUFlowIDForType_AllTypes() {
	ou := ou.OrganizationUnit{
		AuthFlowID:           "auth-id",
		RegistrationFlowID:   "reg-id",
		UserOnboardingFlowID: "onboard-id",
		RecoveryFlowID:       "recovery-id",
		SignOutFlowID:        "signout-id",
	}

	s.Equal("auth-id", ouFlowIDForType(ou, providers.FlowTypeAuthentication))
	s.Equal("reg-id", ouFlowIDForType(ou, providers.FlowTypeRegistration))
	s.Equal("onboard-id", ouFlowIDForType(ou, providers.FlowTypeUserOnboarding))
	s.Equal("recovery-id", ouFlowIDForType(ou, providers.FlowTypeRecovery))
	s.Equal("signout-id", ouFlowIDForType(ou, providers.FlowTypeSignOut))
	s.Empty(ouFlowIDForType(ou, providers.FlowType("UNKNOWN")))
}

// ----- getFlowSectionConfig -----

func (s *FlowMgtServiceTestSuite) TestGetFlowSectionConfig_NilServerConfigSvc() {
	svc := newFlowMgtService(s.mockStore, s.mockInference, s.mockGraphBuilder,
		s.mockExecutorRegistry, s.mockInterceptorRegistry, s.mockValidator, nil, &stubTransactioner{},
		nil, nil, nil, nil, s.mockSharing, nil)

	cfg := svc.getFlowSectionConfig(context.Background())

	s.Equal(flowconfig.FlowSectionConfig{}, cfg)
}

func (s *FlowMgtServiceTestSuite) TestGetFlowSectionConfig_ServerConfigError() {
	mockSC := newServerConfigProviderMock(s.T())
	svc := newFlowMgtService(s.mockStore, s.mockInference, s.mockGraphBuilder,
		s.mockExecutorRegistry, s.mockInterceptorRegistry, s.mockValidator, nil, &stubTransactioner{},
		mockSC, nil, nil, nil, s.mockSharing, nil)

	mockSC.EXPECT().GetMergedConfig(mock.Anything, "flow").Return(nil, &tidcommon.InternalServerError)

	cfg := svc.getFlowSectionConfig(context.Background())

	s.Equal(flowconfig.FlowSectionConfig{}, cfg)
}

func (s *FlowMgtServiceTestSuite) TestGetFlowSectionConfig_WrongType() {
	mockSC := newServerConfigProviderMock(s.T())
	svc := newFlowMgtService(s.mockStore, s.mockInference, s.mockGraphBuilder,
		s.mockExecutorRegistry, s.mockInterceptorRegistry, s.mockValidator, nil, &stubTransactioner{},
		mockSC, nil, nil, nil, s.mockSharing, nil)

	mockSC.EXPECT().GetMergedConfig(mock.Anything, "flow").Return("not-a-flow-section-config", nil)

	cfg := svc.getFlowSectionConfig(context.Background())

	s.Equal(flowconfig.FlowSectionConfig{}, cfg)
}

// ----- resolveDefaultFlowHandle -----

func (s *FlowMgtServiceTestSuite) TestResolveDefaultFlowHandle_AllFlowTypes() {
	mockSC := newServerConfigProviderMock(s.T())
	svc := newFlowMgtService(s.mockStore, s.mockInference, s.mockGraphBuilder,
		s.mockExecutorRegistry, s.mockInterceptorRegistry, s.mockValidator, nil, &stubTransactioner{},
		mockSC, nil, nil, nil, s.mockSharing, nil)

	section := flowconfig.FlowSectionConfig{
		AuthFlow:           flowconfig.FlowTypeConfig{DefaultHandle: "h-auth"},
		RegistrationFlow:   flowconfig.FlowTypeConfig{DefaultHandle: "h-reg"},
		UserOnboardingFlow: flowconfig.FlowTypeConfig{DefaultHandle: "h-onboard"},
		RecoveryFlow:       flowconfig.FlowTypeConfig{DefaultHandle: "h-recovery"},
		SignOutFlow:        flowconfig.FlowTypeConfig{DefaultHandle: "h-signout"},
	}
	mockSC.EXPECT().GetMergedConfig(mock.Anything, "flow").Return(section, nil).Times(6)

	testCases := []struct {
		flowType providers.FlowType
		expected string
	}{
		{providers.FlowTypeAuthentication, "h-auth"},
		{providers.FlowTypeRegistration, "h-reg"},
		{providers.FlowTypeUserOnboarding, "h-onboard"},
		{providers.FlowTypeRecovery, "h-recovery"},
		{providers.FlowTypeSignOut, "h-signout"},
		{providers.FlowType("UNKNOWN"), ""},
	}

	for _, tc := range testCases {
		handle := svc.resolveDefaultFlowHandle(context.Background(), tc.flowType)
		s.Equal(tc.expected, handle)
	}
}

// Flow organization unit tests

const testOUHandle = "default"

func (s *FlowMgtServiceTestSuite) newFlowDef(ouID string) *FlowDefinition {
	return &FlowDefinition{
		Handle: "test-handle", Name: "Test Flow", FlowType: providers.FlowTypeAuthentication,
		Nodes: validFlowNodes(), OUID: ouID,
	}
}

func (s *FlowMgtServiceTestSuite) TestCreateFlow_OwnedByTheOUItNames() {
	s.mockOU.EXPECT().ResolveOwner(mock.Anything, "ou-named", "", mock.Anything).Return("ou-named", nil)
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)
	s.mockStore.EXPECT().IsFlowExistsByHandle(mock.Anything, "test-handle", providers.FlowTypeAuthentication).
		Return(false, nil)
	s.mockStore.EXPECT().CreateFlow(mock.Anything, mock.Anything, mock.MatchedBy(func(def *FlowDefinition) bool {
		return def.OUID == "ou-named"
	})).Return(&providers.CompleteFlowDefinition{OUID: "ou-named"}, nil)

	result, err := s.service.CreateFlow(context.Background(), s.newFlowDef("ou-named"))

	s.Nil(err)
	s.Equal("ou-named", result.OUID)
}

func (s *FlowMgtServiceTestSuite) TestCreateFlow_ResolvesTheOUHandleToItsID() {
	flowDef := s.newFlowDef("")
	flowDef.OUHandle = testOUHandle
	s.mockOU.EXPECT().ResolveOwner(mock.Anything, "", testOUHandle, mock.Anything).Return("ou-default", nil)
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)
	s.mockStore.EXPECT().IsFlowExistsByHandle(mock.Anything, "test-handle", providers.FlowTypeAuthentication).
		Return(false, nil)
	s.mockStore.EXPECT().CreateFlow(mock.Anything, mock.Anything, mock.MatchedBy(func(def *FlowDefinition) bool {
		return def.OUID == "ou-default" && def.OUHandle == ""
	})).Return(&providers.CompleteFlowDefinition{OUID: "ou-default"}, nil)

	result, err := s.service.CreateFlow(context.Background(), flowDef)

	s.Nil(err)
	s.Equal("ou-default", result.OUID)
}

func (s *FlowMgtServiceTestSuite) TestCreateFlow_TheIDWinsOverTheHandle() {
	flowDef := s.newFlowDef("ou-named")
	flowDef.OUHandle = testOUHandle
	s.mockOU.EXPECT().ResolveOwner(mock.Anything, "ou-named", testOUHandle, mock.Anything).Return("ou-named", nil)
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)
	s.mockStore.EXPECT().IsFlowExistsByHandle(mock.Anything, "test-handle", providers.FlowTypeAuthentication).
		Return(false, nil)
	s.mockStore.EXPECT().CreateFlow(mock.Anything, mock.Anything, mock.MatchedBy(func(def *FlowDefinition) bool {
		return def.OUID == "ou-named"
	})).Return(&providers.CompleteFlowDefinition{OUID: "ou-named"}, nil)

	result, err := s.service.CreateFlow(context.Background(), flowDef)

	s.Nil(err)
	s.Equal("ou-named", result.OUID)
}

func (s *FlowMgtServiceTestSuite) TestCreateFlow_RequiresAnOU() {
	s.mockOU.EXPECT().ResolveOwner(mock.Anything, "", "", mock.Anything).Return("", &ErrorFlowOURequired)
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)

	result, err := s.service.CreateFlow(context.Background(), s.newFlowDef(""))

	s.Nil(result)
	s.Equal(&ErrorFlowOURequired, err)
}

func (s *FlowMgtServiceTestSuite) TestCreateFlow_DoesNotDefaultToTheCallersOU() {
	s.mockOU.EXPECT().ResolveOwner(mock.Anything, "", "", mock.Anything).Return("", &ErrorFlowOURequired)
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)
	ctx := security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest("user-1", "ou-caller", "", nil, nil))

	result, err := s.service.CreateFlow(ctx, s.newFlowDef(""))

	s.Nil(result)
	s.Equal(&ErrorFlowOURequired, err)
}

func (s *FlowMgtServiceTestSuite) TestCreateFlow_RejectsAnOUThatDoesNotExist() {
	s.mockOU.EXPECT().ResolveOwner(mock.Anything, "ou-missing", "", mock.Anything).Return("", &ErrorInvalidFlowOU)
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)

	result, err := s.service.CreateFlow(context.Background(), s.newFlowDef("ou-missing"))

	s.Nil(result)
	s.Equal(&ErrorInvalidFlowOU, err)
}

func (s *FlowMgtServiceTestSuite) TestCreateFlow_RejectsAnOUHandleThatDoesNotExist() {
	flowDef := s.newFlowDef("")
	flowDef.OUHandle = "nowhere"
	s.mockOU.EXPECT().ResolveOwner(mock.Anything, "", "nowhere", mock.Anything).Return("", &ErrorInvalidFlowOU)
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)

	result, err := s.service.CreateFlow(context.Background(), flowDef)

	s.Nil(result)
	s.Equal(&ErrorInvalidFlowOU, err)
}

func (s *FlowMgtServiceTestSuite) TestCreateFlow_FailsWhenTheOUCannotBeLookedUp() {
	s.mockOU.EXPECT().ResolveOwner(mock.Anything, "ou-broken", "", mock.Anything).
		Return("", &tidcommon.InternalServerError)
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)

	result, err := s.service.CreateFlow(context.Background(), s.newFlowDef("ou-broken"))

	s.Nil(result)
	s.Equal(&tidcommon.InternalServerError, err)
}

func (s *FlowMgtServiceTestSuite) TestUpdateFlow_RefusesToChangeTheOU() {
	existingFlow := &providers.CompleteFlowDefinition{
		ID: testFlowIDService, Handle: "test-handle", FlowType: providers.FlowTypeAuthentication, OUID: "ou-1",
	}
	s.mockOU.EXPECT().ResolveOwnerForUpdate(mock.Anything, "ou-2", "", "ou-1", mock.Anything).
		Return("", &ErrorOUUpdateNotAllowed)
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(existingFlow, nil)

	result, err := s.service.UpdateFlow(context.Background(), testFlowIDService, s.newFlowDef("ou-2"))

	s.Nil(result)
	s.Equal(&ErrorOUUpdateNotAllowed, err)
}

func (s *FlowMgtServiceTestSuite) TestUpdateFlow_RefusesToChangeTheOUByHandle() {
	existingFlow := &providers.CompleteFlowDefinition{
		ID: testFlowIDService, Handle: "test-handle", FlowType: providers.FlowTypeAuthentication, OUID: "ou-1",
	}
	flowDef := s.newFlowDef("")
	flowDef.OUHandle = "other"
	s.mockOU.EXPECT().ResolveOwnerForUpdate(mock.Anything, "", "other", "ou-1", mock.Anything).
		Return("", &ErrorOUUpdateNotAllowed)
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(existingFlow, nil)

	result, err := s.service.UpdateFlow(context.Background(), testFlowIDService, flowDef)

	s.Nil(result)
	s.Equal(&ErrorOUUpdateNotAllowed, err)
}

func (s *FlowMgtServiceTestSuite) TestUpdateFlow_FailsWhenTheOUHandleCannotBeLookedUp() {
	existingFlow := &providers.CompleteFlowDefinition{
		ID: testFlowIDService, Handle: "test-handle", FlowType: providers.FlowTypeAuthentication, OUID: "ou-1",
	}
	flowDef := s.newFlowDef("")
	flowDef.OUHandle = "other"
	s.mockOU.EXPECT().ResolveOwnerForUpdate(mock.Anything, "", "other", "ou-1", mock.Anything).
		Return("", &tidcommon.InternalServerError)
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(existingFlow, nil)

	result, err := s.service.UpdateFlow(context.Background(), testFlowIDService, flowDef)

	s.Nil(result)
	s.Equal(&tidcommon.InternalServerError, err)
}

func (s *FlowMgtServiceTestSuite) TestUpdateFlow_KeepsTheOUWhenOmitted() {
	existingFlow := &providers.CompleteFlowDefinition{
		ID: testFlowIDService, Handle: "test-handle", FlowType: providers.FlowTypeAuthentication, OUID: "ou-1",
	}
	flowDef := s.newFlowDef("")
	updated := &providers.CompleteFlowDefinition{Handle: "test-handle", OUID: "ou-1"}
	s.mockOU.EXPECT().ResolveOwnerForUpdate(mock.Anything, "", "", "ou-1", mock.Anything).Return("ou-1", nil)
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(existingFlow, nil)
	s.mockStore.EXPECT().UpdateFlow(mock.Anything, testFlowIDService, flowDef).Return(updated, nil)
	s.mockGraphBuilder.EXPECT().InvalidateCache(mock.Anything, testFlowIDService)

	result, err := s.service.UpdateFlow(context.Background(), testFlowIDService, flowDef)

	s.Nil(err)
	s.Equal("ou-1", result.OUID)
}

func (s *FlowMgtServiceTestSuite) TestUpdateFlow_KeepsTheOUWhenRepeated() {
	existingFlow := &providers.CompleteFlowDefinition{
		ID: testFlowIDService, Handle: "test-handle", FlowType: providers.FlowTypeAuthentication, OUID: "ou-1",
	}
	flowDef := s.newFlowDef("ou-1")
	updated := &providers.CompleteFlowDefinition{Handle: "test-handle", OUID: "ou-1"}
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(existingFlow, nil)
	s.mockStore.EXPECT().UpdateFlow(mock.Anything, testFlowIDService, flowDef).Return(updated, nil)
	s.mockGraphBuilder.EXPECT().InvalidateCache(mock.Anything, testFlowIDService)

	result, err := s.service.UpdateFlow(context.Background(), testFlowIDService, flowDef)

	s.Nil(err)
	s.Equal("ou-1", result.OUID)
	s.Equal("ou-1", flowDef.OUID)
}

func (s *FlowMgtServiceTestSuite) TestResolveFlowOU_ReplacesTheHandleWithTheID() {
	flowDef := s.newFlowDef("")
	flowDef.OUHandle = testOUHandle
	s.mockOU.EXPECT().ResolveOwner(mock.Anything, "", testOUHandle, mock.Anything).Return("ou-default", nil)

	err := s.service.ResolveFlowOU(context.Background(), flowDef)

	s.Nil(err)
	s.Equal("ou-default", flowDef.OUID)
	s.Empty(flowDef.OUHandle)
}

func (s *FlowMgtServiceTestSuite) TestGetResourceDependencies_ListsTheFlowsAnOUOwns() {
	s.mockStore.EXPECT().ListFlows(mock.Anything, maxPageSize, 0, "").Return([]BasicFlowDefinition{
		{ID: "flow-1", Name: "Owned", OUID: "ou-1"},
		{ID: "flow-2", Name: "Other", OUID: "ou-2"},
	}, 2, nil)
	s.mockStore.EXPECT().ListActiveFlowsWithNodes(mock.Anything).Return(nil, nil)

	deps, err := s.service.GetResourceDependencies(context.Background(), resourcedependency.ResourceTypeOU, "ou-1")

	s.NoError(err)
	s.Equal([]resourcedependency.ResourceDependency{{
		ResourceType: resourcedependency.ResourceTypeFlow, ID: "flow-1", DisplayName: "Owned",
		BehaviorOnDelete: resourcedependency.BehaviorRestrict,
	}}, deps)
}

func (s *FlowMgtServiceTestSuite) TestGetResourceDependencies_PagesThroughEveryFlowOfAnOU() {
	first := make([]BasicFlowDefinition, maxPageSize)
	for i := range first {
		first[i] = BasicFlowDefinition{ID: fmt.Sprintf("flow-%d", i), OUID: "ou-2"}
	}
	s.mockStore.EXPECT().ListFlows(mock.Anything, maxPageSize, 0, "").Return(first, maxPageSize+1, nil)
	s.mockStore.EXPECT().ListFlows(mock.Anything, maxPageSize, maxPageSize, "").
		Return([]BasicFlowDefinition{{ID: "flow-last", Name: "Last", OUID: "ou-1"}}, maxPageSize+1, nil)
	s.mockStore.EXPECT().ListActiveFlowsWithNodes(mock.Anything).Return(nil, nil)

	deps, err := s.service.GetResourceDependencies(context.Background(), resourcedependency.ResourceTypeOU, "ou-1")

	s.NoError(err)
	s.Len(deps, 1)
	s.Equal("flow-last", deps[0].ID)
}

func (s *FlowMgtServiceTestSuite) TestGetResourceDependencies_ReportsAFailureToListFlowsOfAnOU() {
	s.mockStore.EXPECT().ListFlows(mock.Anything, maxPageSize, 0, "").Return(nil, 0, errors.New("db down"))

	deps, err := s.service.GetResourceDependencies(context.Background(), resourcedependency.ResourceTypeOU, "ou-1")

	s.Nil(deps)
	s.Error(err)
}

// ---- Sharing and connection variables ----

func (s *FlowMgtServiceTestSuite) withSharing(sharingSvc *sharingmock.SharingServiceInterfaceMock) {
	impl := s.service.(*flowMgtService)
	impl.sharingSvc = sharingSvc
}

func (s *FlowMgtServiceTestSuite) TestUpdateFlow_AFlowSharedWithTheUnitInTheRequestIsReadOnlyThroughIt() {
	sharingSvc := sharingmock.NewSharingServiceInterfaceMock(s.T())
	s.withSharing(sharingSvc)
	sharingSvc.EXPECT().IsVisible(mock.Anything, FlowSharingType, testFlowIDService, "ou-child").Return(true, nil)
	existing := &providers.CompleteFlowDefinition{
		ID: testFlowIDService, Handle: "test-handle", FlowType: providers.FlowTypeAuthentication, OUID: "ou-1",
	}
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(existing, nil)

	result, err := s.service.UpdateFlow(context.Background(), testFlowIDService, s.newFlowDef("ou-child"))

	s.Nil(result)
	s.Equal(&ErrorSharedFlowReadOnly, err)
}

// brokenPolicy is a policy that names a variable no flow of these tests declares.
func brokenPolicy() sharing.Policy {
	return sharing.Policy{
		ID: "p1", InitiatingOUID: "ou-1", ChildUnits: &sharing.ChildUnits{Mode: sharing.ChildUnitsAll},
		Rules: []sharing.StoredRule{{
			FieldKey: sharingKey("gone"), Grant: sharing.GrantRef{Section: sharing.SectionChildUnits},
			Requested: sharing.OverlayRule{Editable: true},
		}},
	}
}

func (s *FlowMgtServiceTestSuite) TestUpdateFlow_RefusesAChangeThatBreaksASharingPolicy() {
	sharingSvc := sharingmock.NewSharingServiceInterfaceMock(s.T())
	s.withSharing(sharingSvc)
	sharingSvc.EXPECT().ListPolicies(mock.Anything, FlowSharingType, testFlowIDService).
		Return([]sharing.Policy{brokenPolicy()}, nil)
	existing := &providers.CompleteFlowDefinition{
		ID: testFlowIDService, Handle: "test-handle", FlowType: providers.FlowTypeAuthentication, OUID: "ou-1",
	}
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(existing, nil)

	result, err := s.service.UpdateFlow(context.Background(), testFlowIDService, s.newFlowDef(""))

	s.Nil(result)
	s.Require().NotNil(err)
	s.Equal(ErrorFlowConflictsWithSharingPolicy.Code, err.Code)
	s.Equal("p1", err.ErrorDescription.Params["policy"])
}

func (s *FlowMgtServiceTestSuite) TestRestoreFlowVersion_RefusesAVersionThatBreaksASharingPolicy() {
	sharingSvc := sharingmock.NewSharingServiceInterfaceMock(s.T())
	s.withSharing(sharingSvc)
	sharingSvc.EXPECT().ListPolicies(mock.Anything, FlowSharingType, testFlowIDService).
		Return([]sharing.Policy{brokenPolicy()}, nil)
	s.mockStore.EXPECT().GetFlowVersion(mock.Anything, testFlowIDService, 1).
		Return(&FlowVersion{Version: 1, Nodes: validFlowNodes()}, nil)
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).
		Return(&providers.CompleteFlowDefinition{ID: testFlowIDService}, nil)

	result, err := s.service.RestoreFlowVersion(context.Background(), testFlowIDService, 1)

	s.Nil(result)
	s.Require().NotNil(err)
	s.Equal(ErrorFlowConflictsWithSharingPolicy.Code, err.Code)
}

func (s *FlowMgtServiceTestSuite) TestUpdateFlow_RefusesAChangeThatInvalidatesAValueAUnitSaved() {
	sharingSvc := sharingmock.NewSharingServiceInterfaceMock(s.T())
	s.withSharing(sharingSvc)
	sharingSvc.EXPECT().ListPolicies(mock.Anything, FlowSharingType, testFlowIDService).
		Return([]sharing.Policy{}, nil)
	sharingSvc.EXPECT().ListOverlayValues(mock.Anything, FlowSharingType, testFlowIDService).
		Return(map[string]map[string][]string{"ou-child": {sharingKey("gone"): {"x"}}}, nil)
	existing := &providers.CompleteFlowDefinition{
		ID: testFlowIDService, Handle: "test-handle", FlowType: providers.FlowTypeAuthentication, OUID: "ou-1",
	}
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(existing, nil)

	result, err := s.service.UpdateFlow(context.Background(), testFlowIDService, s.newFlowDef(""))

	s.Nil(result)
	s.Require().NotNil(err)
	s.Equal(ErrorFlowConflictsWithSavedValues.Code, err.Code)
	s.Contains(err.ErrorDescription.Params["details"], "ou-child")
}

func (s *FlowMgtServiceTestSuite) TestRestoreFlowVersion_RefusesAVersionThatInvalidatesAValueAUnitSaved() {
	sharingSvc := sharingmock.NewSharingServiceInterfaceMock(s.T())
	s.withSharing(sharingSvc)
	sharingSvc.EXPECT().ListPolicies(mock.Anything, FlowSharingType, testFlowIDService).
		Return([]sharing.Policy{}, nil)
	sharingSvc.EXPECT().ListOverlayValues(mock.Anything, FlowSharingType, testFlowIDService).
		Return(map[string]map[string][]string{"ou-child": {sharingKey("gone"): {"x"}}}, nil)
	s.mockStore.EXPECT().GetFlowVersion(mock.Anything, testFlowIDService, 1).
		Return(&FlowVersion{Version: 1, Nodes: validFlowNodes()}, nil)
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).
		Return(&providers.CompleteFlowDefinition{ID: testFlowIDService}, nil)

	result, err := s.service.RestoreFlowVersion(context.Background(), testFlowIDService, 1)

	s.Nil(result)
	s.Require().NotNil(err)
	s.Equal(ErrorFlowConflictsWithSavedValues.Code, err.Code)
}

func (s *FlowMgtServiceTestSuite) TestUpdateFlow_AnotherUnitTheFlowWasNotSharedWithIsAnOwnerChange() {
	sharingSvc := sharingmock.NewSharingServiceInterfaceMock(s.T())
	s.withSharing(sharingSvc)
	sharingSvc.EXPECT().IsVisible(mock.Anything, FlowSharingType, testFlowIDService, "ou-2").Return(false, nil)
	existing := &providers.CompleteFlowDefinition{
		ID: testFlowIDService, Handle: "test-handle", FlowType: providers.FlowTypeAuthentication, OUID: "ou-1",
	}
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(existing, nil)
	s.mockOU.EXPECT().ResolveOwnerForUpdate(mock.Anything, "ou-2", "", "ou-1", mock.Anything).
		Return("", &ErrorOUUpdateNotAllowed)

	result, err := s.service.UpdateFlow(context.Background(), testFlowIDService, s.newFlowDef("ou-2"))

	s.Nil(result)
	s.Equal(&ErrorOUUpdateNotAllowed, err)
}

func variableNodes(identifiers ...string) []providers.NodeDefinition {
	inputs := make([]providers.VariableDefinition, 0, len(identifiers))
	for _, id := range identifiers {
		inputs = append(inputs, providers.VariableDefinition{
			Identifier: id, Type: providers.InputTypeText, DefaultValue: "x",
		})
	}
	return []providers.NodeDefinition{
		{ID: "start", Type: "START", OnSuccess: "vars"},
		{ID: "vars", Type: "VARIABLES", OnSuccess: "end", Inputs: inputs},
		{ID: "end", Type: "END"},
	}
}

// A variable an update takes out of the flow takes its rules and every unit's saved value with it, and the ones it
// keeps are left alone.
func (s *FlowMgtServiceTestSuite) TestUpdateFlow_DropsTheSharingOfAVariableItRemoves() {
	sharingSvc := sharingmock.NewSharingServiceInterfaceMock(s.T())
	s.withSharing(sharingSvc)
	existing := &providers.CompleteFlowDefinition{
		ID: testFlowIDService, Handle: "test-handle", FlowType: providers.FlowTypeAuthentication, OUID: "ou-1",
		Nodes: variableNodes("brand", "locale"),
	}
	flowDef := s.newFlowDef("ou-1")
	flowDef.Nodes = variableNodes("locale")
	sharingSvc.EXPECT().RemoveField(mock.Anything, FlowSharingType, testFlowIDService, sharingKey("brand")).
		Return(nil).Once()
	sharingSvc.EXPECT().ListPolicies(mock.Anything, FlowSharingType, testFlowIDService).
		Return([]sharing.Policy{}, nil)
	sharingSvc.EXPECT().ListOverlayValues(mock.Anything, FlowSharingType, testFlowIDService).
		Return(map[string]map[string][]string{}, nil)
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(existing, nil)
	s.mockStore.EXPECT().UpdateFlow(mock.Anything, testFlowIDService, flowDef).
		Return(&providers.CompleteFlowDefinition{ActiveVersion: 2}, nil)
	s.mockGraphBuilder.EXPECT().InvalidateCache(mock.Anything, testFlowIDService)

	result, err := s.service.UpdateFlow(context.Background(), testFlowIDService, flowDef)

	s.Nil(err)
	s.NotNil(result)
}

// A failure to drop the sharing of a removed variable fails the update, which is then rolled back.
func (s *FlowMgtServiceTestSuite) TestUpdateFlow_FailsWhenTheSharingOfARemovedVariableCannotBeDropped() {
	sharingSvc := sharingmock.NewSharingServiceInterfaceMock(s.T())
	s.withSharing(sharingSvc)
	existing := &providers.CompleteFlowDefinition{
		ID: testFlowIDService, Handle: "test-handle", FlowType: providers.FlowTypeAuthentication, OUID: "ou-1",
		Nodes: variableNodes("brand"),
	}
	sharingSvc.EXPECT().RemoveField(mock.Anything, FlowSharingType, testFlowIDService, sharingKey("brand")).
		Return(&tidcommon.InternalServerError)
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(existing, nil)

	result, err := s.service.UpdateFlow(context.Background(), testFlowIDService, s.newFlowDef("ou-1"))

	s.Nil(result)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

// Restoring a version that lacks a variable of the current one drops that variable's sharing the same way.
func (s *FlowMgtServiceTestSuite) TestRestoreFlowVersion_DropsTheSharingOfAVariableTheVersionLacks() {
	sharingSvc := sharingmock.NewSharingServiceInterfaceMock(s.T())
	s.withSharing(sharingSvc)
	current := &providers.CompleteFlowDefinition{
		ID: testFlowIDService, Handle: "test-handle", FlowType: providers.FlowTypeAuthentication, OUID: "ou-1",
		Nodes: variableNodes("brand"),
	}
	sharingSvc.EXPECT().RemoveField(mock.Anything, FlowSharingType, testFlowIDService, sharingKey("brand")).
		Return(nil).Once()
	sharingSvc.EXPECT().ListPolicies(mock.Anything, FlowSharingType, testFlowIDService).
		Return([]sharing.Policy{}, nil)
	sharingSvc.EXPECT().ListOverlayValues(mock.Anything, FlowSharingType, testFlowIDService).
		Return(map[string]map[string][]string{}, nil)
	s.mockStore.EXPECT().GetFlowVersion(mock.Anything, testFlowIDService, 1).
		Return(&FlowVersion{Version: 1, Nodes: validFlowNodes()}, nil)
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(current, nil)
	s.mockStore.EXPECT().RestoreFlowVersion(mock.Anything, testFlowIDService, 1).
		Return(&providers.CompleteFlowDefinition{ActiveVersion: 3}, nil)
	s.mockGraphBuilder.EXPECT().InvalidateCache(mock.Anything, testFlowIDService)

	result, err := s.service.RestoreFlowVersion(context.Background(), testFlowIDService, 1)

	s.Nil(err)
	s.NotNil(result)
}

func documentPolicy(variable string) sharing.PolicyRequest {
	return sharing.PolicyRequest{ChildUnits: &sharing.ChildUnitsRequest{
		Mode:         sharing.ChildUnitsAll,
		OverlayRules: map[string]sharing.OverlayRule{sharingKey(variable): {Editable: true}},
	}}
}

func (s *FlowMgtServiceTestSuite) expectDocumentUpdate(
	existing *providers.CompleteFlowDefinition, flowDef *FlowDefinition,
) {
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil)
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(existing, nil)
	s.mockStore.EXPECT().UpdateFlow(mock.Anything, testFlowIDService, flowDef).
		Return(&providers.CompleteFlowDefinition{ActiveVersion: 2}, nil)
	s.mockGraphBuilder.EXPECT().InvalidateCache(mock.Anything, testFlowIDService)
}

// A document that states its policies is checked against its own definition, not against the policies the flow held:
// the stored policy names a variable the document drops, and the document's policy replaces it.
func (s *FlowMgtServiceTestSuite) TestUpdateFlowWithSharing_ChecksTheDocumentAgainstItself() {
	sharingSvc := sharingmock.NewSharingServiceInterfaceMock(s.T())
	s.withSharing(sharingSvc)
	existing := &providers.CompleteFlowDefinition{
		ID: testFlowIDService, Handle: "test-handle", FlowType: providers.FlowTypeAuthentication, OUID: "ou-1",
		Nodes: variableNodes("brand", "locale"),
	}
	flowDef := s.newFlowDef("ou-1")
	flowDef.Nodes = variableNodes("locale")
	policies := []sharing.PolicyRequest{documentPolicy("locale")}
	s.expectDocumentUpdate(existing, flowDef)
	sharingSvc.EXPECT().RemoveField(mock.Anything, FlowSharingType, testFlowIDService, sharingKey("brand")).
		Return(nil)
	sharingSvc.EXPECT().ReplacePoliciesForResource(mock.Anything, FlowSharingType, testFlowIDService, "ou-1", policies).
		Return(sharing.PolicyReplay{}, nil)
	// Only what is left after the replace is checked, and it holds nothing that conflicts.
	sharingSvc.EXPECT().ListPolicies(mock.Anything, FlowSharingType, testFlowIDService).
		Return([]sharing.Policy{}, nil).Once()
	sharingSvc.EXPECT().ListOverlayValues(mock.Anything, FlowSharingType, testFlowIDService).
		Return(map[string]map[string][]string{}, nil).Once()

	result, err := s.service.UpdateFlowWithSharing(context.Background(), testFlowIDService, flowDef, policies)

	s.Nil(err)
	s.NotNil(result)
}

// A policy of the document that does not fit the new definition is refused before anything is replaced.
func (s *FlowMgtServiceTestSuite) TestUpdateFlowWithSharing_RefusesAPolicyThatDoesNotFitTheDefinition() {
	sharingSvc := sharingmock.NewSharingServiceInterfaceMock(s.T())
	s.withSharing(sharingSvc)
	existing := &providers.CompleteFlowDefinition{
		ID: testFlowIDService, Handle: "test-handle", FlowType: providers.FlowTypeAuthentication, OUID: "ou-1",
		Nodes: variableNodes("locale"),
	}
	flowDef := s.newFlowDef("ou-1")
	flowDef.Nodes = variableNodes("locale")
	s.expectDocumentUpdate(existing, flowDef)

	result, err := s.service.UpdateFlowWithSharing(context.Background(), testFlowIDService, flowDef,
		[]sharing.PolicyRequest{documentPolicy("gone")})

	s.Nil(result)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidSharingPolicy.Code, err.Code)
}

// A refusal by the sharing framework names the policy of the document, and the update goes with it.
func (s *FlowMgtServiceTestSuite) TestUpdateFlowWithSharing_NamesThePolicyTheFrameworkRefused() {
	sharingSvc := sharingmock.NewSharingServiceInterfaceMock(s.T())
	s.withSharing(sharingSvc)
	existing := &providers.CompleteFlowDefinition{
		ID: testFlowIDService, Handle: "test-handle", FlowType: providers.FlowTypeAuthentication, OUID: "ou-1",
		Nodes: variableNodes("locale"),
	}
	flowDef := s.newFlowDef("ou-1")
	flowDef.Nodes = variableNodes("locale")
	policies := []sharing.PolicyRequest{documentPolicy("locale")}
	s.expectDocumentUpdate(existing, flowDef)
	sharingSvc.EXPECT().ReplacePoliciesForResource(mock.Anything, FlowSharingType, testFlowIDService, "ou-1", policies).
		Return(sharing.PolicyReplay{}, &sharing.PolicyReplayError{
			Index: 0, InitiatingOUID: "ou-9", Err: &sharing.ErrorInvalidRequestFormat})

	result, err := s.service.UpdateFlowWithSharing(context.Background(), testFlowIDService, flowDef, policies)

	s.Nil(result)
	s.Require().NotNil(err)
	s.Equal(sharing.ErrorInvalidRequestFormat.Code, err.Code)
	s.Contains(err.ErrorDescription.DefaultValue, "sharing policy 1 (organization unit ou-9)")
}

// A policy the document does not name is kept, so it is checked against the new definition after the replace.
func (s *FlowMgtServiceTestSuite) TestUpdateFlowWithSharing_ChecksWhatIsLeftAfterTheReplace() {
	sharingSvc := sharingmock.NewSharingServiceInterfaceMock(s.T())
	s.withSharing(sharingSvc)
	existing := &providers.CompleteFlowDefinition{
		ID: testFlowIDService, Handle: "test-handle", FlowType: providers.FlowTypeAuthentication, OUID: "ou-1",
		Nodes: variableNodes("locale"),
	}
	flowDef := s.newFlowDef("ou-1")
	flowDef.Nodes = variableNodes("locale")
	policies := []sharing.PolicyRequest{documentPolicy("locale")}
	s.expectDocumentUpdate(existing, flowDef)
	sharingSvc.EXPECT().ReplacePoliciesForResource(mock.Anything, FlowSharingType, testFlowIDService, "ou-1", policies).
		Return(sharing.PolicyReplay{}, nil)
	sharingSvc.EXPECT().ListPolicies(mock.Anything, FlowSharingType, testFlowIDService).
		Return([]sharing.Policy{brokenPolicy()}, nil)

	result, err := s.service.UpdateFlowWithSharing(context.Background(), testFlowIDService, flowDef, policies)

	s.Nil(result)
	s.Require().NotNil(err)
	s.Equal(ErrorFlowConflictsWithSharingPolicy.Code, err.Code)
}

func (s *FlowMgtServiceTestSuite) TestDeleteFlow_RemovesItsSharingPolicies() {
	sharingSvc := sharingmock.NewSharingServiceInterfaceMock(s.T())
	s.withSharing(sharingSvc)
	existing := &providers.CompleteFlowDefinition{ID: testFlowIDService, Handle: "test-handle"}
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testFlowIDService).Return(existing, nil)
	s.mockStore.EXPECT().DeleteFlow(mock.Anything, testFlowIDService).Return(nil)
	s.mockGraphBuilder.EXPECT().InvalidateCache(mock.Anything, testFlowIDService)
	sharingSvc.EXPECT().DeletePoliciesForResource(mock.Anything, FlowSharingType, testFlowIDService).Return(nil)

	s.Nil(s.service.DeleteFlow(context.Background(), testFlowIDService))
}

// ---------------------------------------------------------------------------
// Sharing, overlay rules and variables
// ---------------------------------------------------------------------------

const (
	ownerOU   = "ou-owner"
	childOU   = "ou-child"
	otherOU   = "ou-other"
	flowIDVar = "flow-vars"
)

type SharingVariablesTestSuite struct {
	suite.Suite
	store   *flowStoreInterfaceMock
	ous     *ouProviderMock
	sharing *sharingmock.SharingServiceInterfaceMock
	idps    *idpProviderMock
	service *flowMgtService
	flow    *providers.CompleteFlowDefinition
}

func TestSharingVariablesTestSuite(t *testing.T) {
	suite.Run(t, new(SharingVariablesTestSuite))
}

func strs(values ...string) *[]string { return &values }

func (s *SharingVariablesTestSuite) SetupTest() {
	s.store = newFlowStoreInterfaceMock(s.T())
	s.ous = newOuProviderMock(s.T())
	s.sharing = sharingmock.NewSharingServiceInterfaceMock(s.T())
	s.idps = newIdpProviderMock(s.T())

	s.service = newFlowMgtService(s.store, nil, nil, nil, nil, nil, nil, &stubTransactioner{}, nil, s.ous, nil, nil,
		s.sharing, s.idps)

	s.flow = &providers.CompleteFlowDefinition{
		ID: flowIDVar, Handle: "acme-login", Name: "Acme Login", FlowType: providers.FlowTypeAuthentication,
		OUID: ownerOU,
		Nodes: []providers.NodeDefinition{
			{ID: "start", Type: "START", OnSuccess: "vars"},
			{ID: "vars", Type: "VARIABLES", OnSuccess: "end", Inputs: []providers.VariableDefinition{
				{Identifier: "supportEmail", Type: providers.InputTypeEmail, DefaultValue: "support@example.com"},
				{Identifier: "otp", Type: providers.InputTypeNumber, DefaultValue: 120.0,
					Min: ptr(30.0), Max: ptr(600.0)},
				{Identifier: "mfa", Type: providers.InputTypeSelect, DefaultValue: "NONE",
					Options: []interface{}{"NONE", "SMS_OTP", "EMAIL_OTP"}},
				{Identifier: "social", Type: providers.InputTypeConnectionSelect, DefaultValue: "c-owner",
					Category: "IDENTITY_PROVIDER", ConnectionTypes: []string{"google"}},
			}},
			{ID: "end", Type: "END"},
		},
	}
	s.store.EXPECT().GetFlowByID(mock.Anything, flowIDVar).Return(s.flow, nil).Maybe()
}

func ptr[T any](v T) *T { return &v }

var assertAnError = errors.New("boom")

// shareWith has the framework answer that the child holds the flow, with the given rules per variable.
func (s *SharingVariablesTestSuite) shareWith(rules map[string]sharing.OverlayRule) {
	s.sharing.EXPECT().IsVisible(mock.Anything, FlowSharingType, flowIDVar, childOU).Return(true, nil).Maybe()
	byKey := map[string]sharing.FieldRule{}
	for _, def := range declaredVariables(s.flow) {
		key := sharingKey(def.Identifier)
		fieldRule := sharing.FieldRule{OUID: childOU, Visible: true}
		if rule, ok := rules[key]; ok {
			fieldRule.Source = sharing.SourcePolicy
			fieldRule.Rule = rule
		}
		byKey[key] = fieldRule
		s.sharing.EXPECT().ResolveFieldRule(mock.Anything, FlowSharingType, flowIDVar, childOU, key).
			Return(fieldRule, nil).Maybe()
	}
	s.sharing.EXPECT().ResolveFieldRules(mock.Anything, FlowSharingType, flowIDVar, childOU, mock.Anything).
		Return(byKey, nil).Maybe()
}

// ---- Access ----

func (s *SharingVariablesTestSuite) TestGetFlowForOURequiresAnOrganizationUnit() {
	_, svcErr := s.service.GetFlowForOU(context.Background(), flowIDVar, "")

	s.Require().NotNil(svcErr)
	s.Equal(ErrorMissingOUID.Code, svcErr.Code)
}

func (s *SharingVariablesTestSuite) TestGetFlowForOUOwner() {
	view, svcErr := s.service.GetFlowForOU(context.Background(), flowIDVar, ownerOU)

	s.Require().Nil(svcErr)
	s.Require().NotNil(view.Owned)
	s.Nil(view.Shared)
	s.True(view.Owned.IsOwnedByOU)
	s.Len(view.Owned.Nodes, 3)
}

func (s *SharingVariablesTestSuite) TestGetFlowForOUSharedShowsOnlyWhatAppliesToTheUnit() {
	s.shareWith(map[string]sharing.OverlayRule{
		sharingKey("supportEmail"): {Editable: true},
		sharingKey("otp"):          {Editable: false, Value: strs("60")},
		sharingKey("mfa"): {
			Editable: true, AllowedValues: strs("SMS_OTP", "EMAIL_OTP"), Value: strs("EMAIL_OTP"),
		},
	})

	view, svcErr := s.service.GetFlowForOU(context.Background(), flowIDVar, childOU)

	s.Require().Nil(svcErr)
	s.Require().NotNil(view.Shared)
	s.False(view.Shared.IsOwnedByOU)
	s.True(view.Shared.IsReadOnly)
	s.Equal(ownerOU, view.Shared.OUID)

	byID := variablesByID(view.Shared.Variables)
	s.Len(byID, 3, "a variable no policy names for the unit is not listed")
	s.NotContains(byID, "social")
	s.Equal("support@example.com", byID["supportEmail"].DefaultValue,
		"no value from the policy keeps the owner's default")
	s.Equal(60.0, byID["otp"].DefaultValue, "the value the policy pins, in the variable's own type")
	s.Equal("EMAIL_OTP", byID["mfa"].DefaultValue)
	s.Equal([]interface{}{"SMS_OTP", "EMAIL_OTP"}, byID["mfa"].Options, "only the options the unit may choose from")
}

func (s *SharingVariablesTestSuite) TestGetFlowForOUNotSharedIsNotFound() {
	s.sharing.EXPECT().IsVisible(mock.Anything, FlowSharingType, flowIDVar, otherOU).Return(false, nil)

	_, svcErr := s.service.GetFlowForOU(context.Background(), flowIDVar, otherOU)

	s.Require().NotNil(svcErr)
	s.Equal(ErrorFlowNotFound.Code, svcErr.Code)
}

func (s *SharingVariablesTestSuite) TestCheckFlowAccess() {
	s.shareWith(nil)

	s.Nil(s.service.CheckFlowAccess(context.Background(), flowIDVar, "", true), "no unit named, nothing to check")
	s.Nil(s.service.CheckFlowAccess(context.Background(), flowIDVar, ownerOU, true))

	svcErr := s.service.CheckFlowAccess(context.Background(), flowIDVar, childOU, false)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorFlowNotFound.Code, svcErr.Code, "a shared flow's definition is not there to read")

	svcErr = s.service.CheckFlowAccess(context.Background(), flowIDVar, childOU, true)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorSharedFlowReadOnly.Code, svcErr.Code)
}

func (s *SharingVariablesTestSuite) TestListFlowsForOU() {
	owned := BasicFlowDefinition{ID: "f-owned", Handle: "owned", Name: "Owned", OUID: childOU, ActiveVersion: 2,
		CreatedAt: "t1", UpdatedAt: "t2"}
	shared := BasicFlowDefinition{ID: flowIDVar, Handle: "acme-login", Name: "Acme", OUID: ownerOU, ActiveVersion: 3,
		CreatedAt: "t1", UpdatedAt: "t2"}
	s.store.EXPECT().ListFlowsByOU(mock.Anything, childOU, defaultPageSize, 0, "").
		Return([]BasicFlowDefinition{owned}, 1, nil)
	s.sharing.EXPECT().ListVisibleResourceIDs(mock.Anything, FlowSharingType, childOU).Return([]string{flowIDVar}, nil)
	s.store.EXPECT().ListFlowsByIDs(mock.Anything, []string{flowIDVar}, "").
		Return([]BasicFlowDefinition{shared}, nil)

	list, svcErr := s.service.ListFlowsForOU(context.Background(), childOU, "", 0, 0)

	s.Require().Nil(svcErr)
	s.Equal(2, list.TotalResults)
	s.Require().Len(list.Flows, 2)
	s.True(*list.Flows[0].IsOwnedByOU)
	s.Equal(2, list.Flows[0].ActiveVersion)
	s.False(*list.Flows[1].IsOwnedByOU)
	s.True(list.Flows[1].IsReadOnly)
	s.Equal(3, list.Flows[1].ActiveVersion, "a shared flow is listed as an owned one is")
	s.Equal("t1", list.Flows[1].CreatedAt)
	s.Equal(ownerOU, list.Flows[1].OUID, "the owner's, not the requested one")
}

func (s *SharingVariablesTestSuite) TestListFlowsForOUFiltersAndPages() {
	s.store.EXPECT().ListFlowsByOU(mock.Anything, childOU, 2, 1, "AUTHENTICATION").Return([]BasicFlowDefinition{
		{ID: "b", OUID: childOU}, {ID: "c", OUID: childOU},
	}, 3, nil)
	s.sharing.EXPECT().ListVisibleResourceIDs(mock.Anything, FlowSharingType, childOU).Return([]string{}, nil)

	list, svcErr := s.service.ListFlowsForOU(
		context.Background(), childOU, providers.FlowTypeAuthentication, 2, 1)

	s.Require().Nil(svcErr)
	s.Equal(3, list.TotalResults)
	s.Equal(2, list.StartIndex)
	s.Len(list.Flows, 2)
	s.Equal("b", list.Flows[0].ID)
	s.Require().Len(list.Links, 3)
	s.Equal(`/flows?offset=0&limit=2&filter=ouId+eq+%22`+childOU+`%22&flowType=AUTHENTICATION`, list.Links[0].Href,
		"the links keep the filter and the flow type")
}

func (s *SharingVariablesTestSuite) TestListFlowsForOUReportsAStoreFailure() {
	s.store.EXPECT().ListFlowsByOU(mock.Anything, childOU, defaultPageSize, 0, "").
		Return(nil, 0, errors.New("db down")).Once()
	_, svcErr := s.service.ListFlowsForOU(context.Background(), childOU, "", 0, 0)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)

	s.store.EXPECT().ListFlowsByOU(mock.Anything, childOU, defaultPageSize, 0, "").
		Return([]BasicFlowDefinition{}, 0, nil).Once()
	s.sharing.EXPECT().ListVisibleResourceIDs(mock.Anything, FlowSharingType, childOU).
		Return([]string{flowIDVar}, nil).Once()
	s.store.EXPECT().ListFlowsByIDs(mock.Anything, []string{flowIDVar}, "").
		Return(nil, errors.New("db down")).Once()
	_, svcErr = s.service.ListFlowsForOU(context.Background(), childOU, "", 0, 0)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *SharingVariablesTestSuite) TestListFlowsForOUReadsOnlyThePageItNeeds() {
	// A unit that owns more than a page lists no shared flow, and the shared ones are not read for a page they
	// do not appear on.
	s.store.EXPECT().ListFlowsByOU(mock.Anything, childOU, 2, 0, "").
		Return([]BasicFlowDefinition{{ID: "a", OUID: childOU}, {ID: "b", OUID: childOU}}, 5, nil)
	s.sharing.EXPECT().ListVisibleResourceIDs(mock.Anything, FlowSharingType, childOU).
		Return([]string{flowIDVar}, nil)
	s.store.EXPECT().ListFlowsByIDs(mock.Anything, []string{flowIDVar}, "").
		Return([]BasicFlowDefinition{{ID: flowIDVar, OUID: ownerOU}}, nil)

	list, svcErr := s.service.ListFlowsForOU(context.Background(), childOU, "", 2, 0)

	s.Require().Nil(svcErr)
	s.Equal(6, list.TotalResults)
	s.Require().Len(list.Flows, 2)
	s.Equal("a", list.Flows[0].ID)
	s.Equal("b", list.Flows[1].ID)
}

func (s *SharingVariablesTestSuite) TestListFlowsForOUValidatesItsInput() {
	_, svcErr := s.service.ListFlowsForOU(context.Background(), "", "", 0, 0)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorMissingOUID.Code, svcErr.Code)

	_, svcErr = s.service.ListFlowsForOU(context.Background(), childOU, "NOPE", 0, 0)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorInvalidFlowType.Code, svcErr.Code)
}

func (s *SharingVariablesTestSuite) TestListFlowsForOUListsOwnedThenSharedAcrossPages() {
	// The owned flows are two, so the page that starts at the second holds the second, then a shared one.
	s.store.EXPECT().ListFlowsByOU(mock.Anything, childOU, 2, 1, "").
		Return([]BasicFlowDefinition{{ID: "mine-2", OUID: childOU}}, 2, nil)
	s.sharing.EXPECT().ListVisibleResourceIDs(mock.Anything, FlowSharingType, childOU).
		Return([]string{"gone", "shared-1", "shared-2"}, nil)
	s.store.EXPECT().ListFlowsByIDs(mock.Anything, []string{"gone", "shared-1", "shared-2"}, "").
		Return([]BasicFlowDefinition{{ID: "shared-2", OUID: ownerOU}, {ID: "shared-1", OUID: ownerOU}}, nil)

	list, svcErr := s.service.ListFlowsForOU(context.Background(), childOU, "", 2, 1)

	s.Require().Nil(svcErr)
	s.Equal(4, list.TotalResults, "a shared flow that no longer exists is not counted")
	s.Require().Len(list.Flows, 2)
	s.Equal("mine-2", list.Flows[0].ID)
	s.True(*list.Flows[0].IsOwnedByOU)
	s.Equal("shared-1", list.Flows[1].ID)
	s.False(*list.Flows[1].IsOwnedByOU)
	s.True(list.Flows[1].IsReadOnly)
}

func (s *SharingVariablesTestSuite) policyWith(rules map[string]sharing.OverlayRule) {
	policy := sharing.Policy{
		ID: "policy-1", ResourceID: flowIDVar, InitiatingOUID: ownerOU,
		ChildUnits: &sharing.ChildUnits{Mode: sharing.ChildUnitsAll},
	}
	for key, rule := range rules {
		policy.Rules = append(policy.Rules, sharing.StoredRule{
			FieldKey: key, Grant: sharing.GrantRef{Section: sharing.SectionChildUnits},
			Requested: rule, Resolved: rule,
		})
	}
	s.sharing.EXPECT().ListPolicies(mock.Anything, FlowSharingType, flowIDVar).
		Return([]sharing.Policy{policy}, nil)
}

func (s *SharingVariablesTestSuite) TestAChangeToAFlowNoPolicyIsRecordedForIsNotChecked() {
	s.sharing.EXPECT().ListPolicies(mock.Anything, FlowSharingType, flowIDVar).Return([]sharing.Policy{}, nil)

	s.Nil(s.service.checkSharingPolicies(context.Background(), flowIDVar, nil))
}

func (s *SharingVariablesTestSuite) TestAChangeThatStillSatisfiesTheirPoliciesIsAccepted() {
	s.policyWith(map[string]sharing.OverlayRule{
		sharingKey("otp"): {Editable: true, Value: strs("60")},
		sharingKey("mfa"): {Editable: true, AllowedValues: strs("SMS_OTP")},
	})

	s.Nil(s.service.checkSharingPolicies(context.Background(), flowIDVar, s.flow.Nodes))
}

func (s *SharingVariablesTestSuite) TestAChangeThatBreaksAPolicyIsRefusedNamingIt() {
	changes := map[string]struct {
		rules  map[string]sharing.OverlayRule
		edit   func(nodes []providers.NodeDefinition)
		reason string
	}{
		"a variable it names is removed": {
			map[string]sharing.OverlayRule{sharingKey("otp"): {Editable: true}},
			func(nodes []providers.NodeDefinition) {
				nodes[1].Inputs = append(nodes[1].Inputs[:1], nodes[1].Inputs[2:]...)
			},
			"does not name a variable the flow declares",
		},
		"an option it offers is removed": {
			map[string]sharing.OverlayRule{sharingKey("mfa"): {Editable: true, AllowedValues: strs("SMS_OTP")}},
			func(nodes []providers.NodeDefinition) {
				nodes[1].Inputs[2].Options = []interface{}{"NONE", "EMAIL_OTP"}
			},
			"SMS_OTP",
		},
		"a limit it states is tightened": {
			map[string]sharing.OverlayRule{sharingKey("otp"): {Editable: true, Value: strs("500")}},
			func(nodes []providers.NodeDefinition) { nodes[1].Inputs[1].Max = ptr(300.0) },
			"500",
		},
		"a variable it names changes type": {
			map[string]sharing.OverlayRule{sharingKey("otp"): {Editable: true, Value: strs("120")}},
			func(nodes []providers.NodeDefinition) { nodes[1].Inputs[1].Type = providers.InputTypeBoolean },
			"120",
		},
	}
	for name, tc := range changes {
		s.Run(name, func() {
			s.SetupTest()
			s.policyWith(tc.rules)
			nodes := append([]providers.NodeDefinition(nil), s.flow.Nodes...)
			nodes[1].Inputs = append([]providers.VariableDefinition(nil), nodes[1].Inputs...)
			tc.edit(nodes)

			svcErr := s.service.checkSharingPolicies(context.Background(), flowIDVar, nodes)

			s.Require().NotNil(svcErr)
			s.Equal(ErrorFlowConflictsWithSharingPolicy.Code, svcErr.Code)
			s.Equal("policy-1", svcErr.ErrorDescription.Params["policy"])
			s.Contains(svcErr.ErrorDescription.Params["reason"], tc.reason)
		})
	}
}

func (s *SharingVariablesTestSuite) TestAFailureToReadThePoliciesIsNotAConflict() {
	s.sharing.EXPECT().ListPolicies(mock.Anything, FlowSharingType, flowIDVar).
		Return(nil, &tidcommon.InternalServerError)

	svcErr := s.service.checkSharingPolicies(context.Background(), flowIDVar, s.flow.Nodes)

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

// ---- A change to the flow against the values organizations units saved ----

// savedValues has the framework answer with what the organization units hold for the flow.
func (s *SharingVariablesTestSuite) savedValues(held map[string]map[string][]string) {
	s.sharing.EXPECT().ListOverlayValues(mock.Anything, FlowSharingType, flowIDVar).Return(held, nil)
}

func (s *SharingVariablesTestSuite) TestAChangeToAFlowNoUnitSavedValuesFor() {
	s.savedValues(map[string]map[string][]string{})

	s.Nil(s.service.checkSavedValues(context.Background(), flowIDVar, nil))
}

func (s *SharingVariablesTestSuite) TestAChangeThatKeepsTheSavedValuesValidIsAccepted() {
	s.idps.EXPECT().GetIdentityProvider(mock.Anything, "c-owner").
		Return(&providers.IDPDTO{ID: "c-owner", Type: providers.IDPTypeGoogle, OUID: ownerOU}, nil)
	s.savedValues(map[string]map[string][]string{childOU: {
		sharingKey("otp"):     {"60"},
		sharingKey("mfa"):     {"SMS_OTP"},
		sharingKey("social"):  {"c-owner"},
		sharingKey("cleared"): {},
	}})

	s.Nil(s.service.checkSavedValues(context.Background(), flowIDVar, s.flow.Nodes),
		"a field the unit cleared holds no value to check")
}

func (s *SharingVariablesTestSuite) TestAChangeThatInvalidatesASavedValueIsRefusedNamingTheUnitAndVariable() {
	changes := map[string]struct {
		key    string
		value  string
		edit   func(nodes []providers.NodeDefinition)
		reason string
	}{
		"a variable is removed": {
			sharingKey("otp"), "60",
			func(nodes []providers.NodeDefinition) {
				nodes[1].Inputs = append(nodes[1].Inputs[:1], nodes[1].Inputs[2:]...)
			},
			"no longer declared",
		},
		"an option is removed": {
			sharingKey("mfa"), "SMS_OTP",
			func(nodes []providers.NodeDefinition) {
				nodes[1].Inputs[2].Options = []interface{}{"NONE", "EMAIL_OTP"}
			},
			"SMS_OTP",
		},
		"a limit is tightened": {
			sharingKey("otp"), "500",
			func(nodes []providers.NodeDefinition) { nodes[1].Inputs[1].Max = ptr(300.0) },
			"500",
		},
		"a variable changes type": {
			sharingKey("otp"), "120",
			func(nodes []providers.NodeDefinition) { nodes[1].Inputs[1].Type = providers.InputTypeBoolean },
			"120",
		},
	}
	for name, tc := range changes {
		s.Run(name, func() {
			s.SetupTest()
			s.savedValues(map[string]map[string][]string{childOU: {tc.key: {tc.value}}})
			nodes := append([]providers.NodeDefinition(nil), s.flow.Nodes...)
			nodes[1].Inputs = append([]providers.VariableDefinition(nil), nodes[1].Inputs...)
			tc.edit(nodes)

			svcErr := s.service.checkSavedValues(context.Background(), flowIDVar, nodes)

			s.Require().NotNil(svcErr)
			s.Equal(ErrorFlowConflictsWithSavedValues.Code, svcErr.Code)
			details := svcErr.ErrorDescription.Params["details"]
			s.Contains(details, childOU)
			s.Contains(details, tc.reason)
		})
	}
}

func (s *SharingVariablesTestSuite) TestAConnectionASavedValueNamesHasToStillBeUsable() {
	s.idps.EXPECT().GetIdentityProvider(mock.Anything, "c-gone").Return(nil, &ErrorFlowNotFound)
	s.savedValues(map[string]map[string][]string{childOU: {sharingKey("social"): {"c-gone"}}})

	svcErr := s.service.checkSavedValues(context.Background(), flowIDVar, s.flow.Nodes)

	s.Require().NotNil(svcErr)
	s.Equal(ErrorFlowConflictsWithSavedValues.Code, svcErr.Code)
	s.Contains(svcErr.ErrorDescription.Params["details"], "not an existing connection")
}

func (s *SharingVariablesTestSuite) TestARefusalNamesOnlyTheFirstFewSavedValues() {
	held := map[string]map[string][]string{}
	for i := 0; i < maxReportedValueConflicts+2; i++ {
		held[fmt.Sprintf("ou-%02d", i)] = map[string][]string{sharingKey("gone"): {"x"}}
	}
	s.savedValues(held)

	svcErr := s.service.checkSavedValues(context.Background(), flowIDVar, s.flow.Nodes)

	s.Require().NotNil(svcErr)
	details := svcErr.ErrorDescription.Params["details"]
	s.Contains(details, "ou-00")
	s.NotContains(details, "ou-06")
	s.Contains(details, "and 2 more")
}

func (s *SharingVariablesTestSuite) TestAFailureToReadTheSavedValuesIsNotAConflict() {
	s.sharing.EXPECT().ListOverlayValues(mock.Anything, FlowSharingType, flowIDVar).
		Return(nil, &tidcommon.InternalServerError)

	svcErr := s.service.checkSavedValues(context.Background(), flowIDVar, s.flow.Nodes)

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

// ---- Sharing policies ----

func (s *SharingVariablesTestSuite) TestCreateSharingPolicy() {
	req := sharing.PolicyRequest{ChildUnits: &sharing.ChildUnitsRequest{
		Mode: sharing.ChildUnitsAll,
		OverlayRules: map[string]sharing.OverlayRule{
			sharingKey("supportEmail"): {Editable: true},
			sharingKey("otp"):          {Editable: true, AllowedValues: strs("60", "120")},
			sharingKey("social"): {
				Editable: true, AllowedValues: strs("c-owner"), ExcludedValues: strs("c-gone"),
			},
		},
	}}
	s.idps.EXPECT().GetIdentityProvider(mock.Anything, "c-owner").
		Return(&providers.IDPDTO{ID: "c-owner", Type: providers.IDPTypeGoogle, OUID: ownerOU}, nil)
	s.sharing.EXPECT().CreatePolicy(mock.Anything, FlowSharingType, flowIDVar, ownerOU, req).
		Return(sharing.Policy{ID: "p1"}, nil)

	policy, svcErr := s.service.CreateSharingPolicy(context.Background(), flowIDVar, req)

	s.Require().Nil(svcErr)
	s.Equal("p1", policy.ID)
}

func (s *SharingVariablesTestSuite) TestCreateSharingPolicyRefusesTermsThatDoNotFitTheFlow() {
	rules := func(r map[string]sharing.OverlayRule) sharing.PolicyRequest {
		return sharing.PolicyRequest{ChildUnits: &sharing.ChildUnitsRequest{
			Mode: sharing.ChildUnitsSelected, Units: []sharing.ChildUnitRequest{
				{OUID: childOU, Reach: sharing.ReachUnit, OverlayRules: r}},
		}}
	}
	cases := map[string]struct {
		rules  map[string]sharing.OverlayRule
		reason string
	}{
		"a rule that is not a variable": {
			map[string]sharing.OverlayRule{"nodes.start": {Editable: true}}, "does not name a variable"},
		"a variable the flow does not declare": {
			map[string]sharing.OverlayRule{"variables.nope": {Editable: true}}, "does not name a variable"},
		"two values for one variable": {
			map[string]sharing.OverlayRule{
				sharingKey("mfa"): {Value: strs("SMS_OTP", "EMAIL_OTP")}}, "holds one value"},
		"a value that is not valid": {
			map[string]sharing.OverlayRule{sharingKey("otp"): {Value: strs("1000")}}, "above the maximum"},
		"an allowed value that is not valid": {
			map[string]sharing.OverlayRule{
				sharingKey("mfa"): {AllowedValues: strs("PHONE_CALL")}}, "not one of the options"},
	}
	for name, tc := range cases {
		s.Run(name, func() {
			_, svcErr := s.service.CreateSharingPolicy(context.Background(), flowIDVar, rules(tc.rules))

			s.Require().NotNil(svcErr)
			s.Equal(ErrorInvalidSharingPolicy.Code, svcErr.Code)
			s.Contains(svcErr.ErrorDescription.Params["reason"], tc.reason)
		})
	}
}

func (s *SharingVariablesTestSuite) TestCreateSharingPolicyChecksTheKindAndExistenceOfAConnection() {
	req := sharing.PolicyRequest{
		InitiatingOUID: childOU,
		ChildUnits: &sharing.ChildUnitsRequest{
			Mode: sharing.ChildUnitsAll,
			OverlayRules: map[string]sharing.OverlayRule{
				sharingKey("social"): {Editable: true, AllowedValues: strs("c-owner")},
			},
		},
	}
	s.idps.EXPECT().GetIdentityProvider(mock.Anything, "c-owner").
		Return(&providers.IDPDTO{ID: "c-owner", Type: providers.IDPTypeGitHub, OUID: childOU}, nil)

	_, svcErr := s.service.CreateSharingPolicy(context.Background(), flowIDVar, req)

	s.Require().NotNil(svcErr)
	s.Equal(ErrorInvalidSharingPolicy.Code, svcErr.Code)
	s.Contains(svcErr.ErrorDescription.Params["reason"], "does not take")
}

// A flow declared in a file is not changed through the API, but the sharing of it is not frozen with it: a unit the
// flow was shared with may still reshare it, and only the policies a file declares are protected.
func (s *SharingVariablesTestSuite) TestAUnitMayReshareADeclarativeFlow() {
	s.flow.IsReadOnly = true
	req := sharing.PolicyRequest{
		InitiatingOUID: childOU,
		ChildUnits:     &sharing.ChildUnitsRequest{Mode: sharing.ChildUnitsAll},
	}
	s.sharing.EXPECT().CreatePolicy(mock.Anything, FlowSharingType, flowIDVar, ownerOU, req).
		Return(sharing.Policy{ID: "p1"}, nil)

	policy, svcErr := s.service.CreateSharingPolicy(context.Background(), flowIDVar, req)

	s.Require().Nil(svcErr)
	s.Equal("p1", policy.ID)
}

func (s *SharingVariablesTestSuite) TestAPolicyADeclarativeFlowDeclaresIsNotChangedThroughTheAPI() {
	s.flow.IsReadOnly = true
	declared := sharing.Policy{ID: "p1", ResourceType: FlowSharingType, ResourceID: flowIDVar, Declared: true}
	s.sharing.EXPECT().GetPolicy(mock.Anything, "p1").Return(declared, nil).Twice()
	s.sharing.EXPECT().UpdatePolicy(mock.Anything, "p1", mock.Anything).
		Return(sharing.Policy{}, &sharing.ErrorPolicyDeclared)

	_, svcErr := s.service.UpdateSharingPolicy(context.Background(), flowIDVar, "p1", sharing.PolicyRequest{})
	s.Require().NotNil(svcErr)
	s.Equal(ErrorSharingPolicyDeclared.Code, svcErr.Code)

	svcErr = s.service.DeleteSharingPolicy(context.Background(), flowIDVar, "p1")
	s.Require().NotNil(svcErr)
	s.Equal(ErrorSharingPolicyDeclared.Code, svcErr.Code)
}
func (s *SharingVariablesTestSuite) TestTheFrameworksRefusalsKeepTheirMeaning() {
	req := sharing.PolicyRequest{ChildUnits: &sharing.ChildUnitsRequest{Mode: sharing.ChildUnitsAll}}
	for frameworkErr, expected := range map[string]tidcommon.ServiceError{
		sharing.ErrorPolicyExists.Code:    ErrorSharingPolicyExists,
		sharing.ErrorPolicyDeclared.Code:  ErrorSharingPolicyDeclared,
		sharing.ErrorVersionMismatch.Code: ErrorSharingPolicyVersionMismatch,
		sharing.ErrorRuleWidens.Code:      ErrorInvalidSharingPolicy,
	} {
		s.Run(frameworkErr, func() {
			failure := tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: frameworkErr}
			s.sharing.EXPECT().CreatePolicy(mock.Anything, FlowSharingType, flowIDVar, ownerOU, req).
				Return(sharing.Policy{}, &failure).Once()

			_, svcErr := s.service.CreateSharingPolicy(context.Background(), flowIDVar, req)

			s.Require().NotNil(svcErr)
			s.Equal(expected.Code, svcErr.Code)
		})
	}

	s.sharing.EXPECT().CreatePolicy(mock.Anything, FlowSharingType, flowIDVar, ownerOU, req).
		Return(sharing.Policy{}, &tidcommon.InternalServerError).Once()
	_, svcErr := s.service.CreateSharingPolicy(context.Background(), flowIDVar, req)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *SharingVariablesTestSuite) TestPoliciesAreReachedThroughTheirFlowOnly() {
	s.sharing.EXPECT().GetPolicy(mock.Anything, "p-other").
		Return(sharing.Policy{ID: "p-other", ResourceType: FlowSharingType, ResourceID: "another-flow"}, nil)
	s.sharing.EXPECT().GetPolicy(mock.Anything, "p-app").
		Return(sharing.Policy{ID: "p-app", ResourceType: "application", ResourceID: flowIDVar}, nil)
	s.sharing.EXPECT().GetPolicy(mock.Anything, "p1").
		Return(sharing.Policy{ID: "p1", ResourceType: FlowSharingType, ResourceID: flowIDVar}, nil)

	_, svcErr := s.service.GetSharingPolicy(context.Background(), flowIDVar, "p-other")
	s.Require().NotNil(svcErr)
	s.Equal(ErrorSharingPolicyNotFound.Code, svcErr.Code)

	_, svcErr = s.service.GetSharingPolicy(context.Background(), flowIDVar, "p-app")
	s.Require().NotNil(svcErr)
	s.Equal(ErrorSharingPolicyNotFound.Code, svcErr.Code)

	policy, svcErr := s.service.GetSharingPolicy(context.Background(), flowIDVar, "p1")
	s.Require().Nil(svcErr)
	s.Equal("p1", policy.ID)
}

func (s *SharingVariablesTestSuite) TestListUpdateAndDeleteSharingPolicies() {
	s.sharing.EXPECT().GetPolicyList(mock.Anything, FlowSharingType, flowIDVar, 10, 0).
		Return(sharing.PolicyList{TotalResults: 1, Count: 1}, nil)
	list, svcErr := s.service.ListSharingPolicies(context.Background(), flowIDVar, 10, 0)
	s.Require().Nil(svcErr)
	s.Equal(1, list.TotalResults)

	stored := sharing.Policy{ID: "p1", ResourceType: FlowSharingType, ResourceID: flowIDVar, InitiatingOUID: ownerOU}
	s.sharing.EXPECT().GetPolicy(mock.Anything, "p1").Return(stored, nil)
	edit := sharing.PolicyRequest{Version: 1, ChildUnits: &sharing.ChildUnitsRequest{Mode: sharing.ChildUnitsAll}}
	sent := edit
	sent.InitiatingOUID = ownerOU
	s.sharing.EXPECT().UpdatePolicy(mock.Anything, "p1", sent).Return(sharing.Policy{ID: "p1", Version: 2}, nil)
	updated, svcErr := s.service.UpdateSharingPolicy(context.Background(), flowIDVar, "p1", edit)
	s.Require().Nil(svcErr)
	s.Equal(2, updated.Version)

	s.sharing.EXPECT().DeletePolicy(mock.Anything, "p1").Return(nil)
	s.Nil(s.service.DeleteSharingPolicy(context.Background(), flowIDVar, "p1"))
}

func (s *SharingVariablesTestSuite) TestDeletingAPolicyThatIsGoneSucceedsAndADeclaredOneIsRefused() {
	s.sharing.EXPECT().GetPolicy(mock.Anything, "gone").Return(sharing.Policy{}, &sharing.ErrorPolicyNotFound)
	s.Nil(s.service.DeleteSharingPolicy(context.Background(), flowIDVar, "gone"))

	s.sharing.EXPECT().GetPolicy(mock.Anything, "declared").Return(sharing.Policy{
		ID: "declared", ResourceType: FlowSharingType, ResourceID: flowIDVar, Declared: true}, nil)
	svcErr := s.service.DeleteSharingPolicy(context.Background(), flowIDVar, "declared")
	s.Require().NotNil(svcErr)
	s.Equal(ErrorSharingPolicyDeclared.Code, svcErr.Code)
}

func (s *SharingVariablesTestSuite) TestADeletedFlowTakesItsPoliciesWithIt() {
	s.sharing.EXPECT().DeletePoliciesForResource(mock.Anything, FlowSharingType, flowIDVar).Return(nil)
	s.Nil(s.service.removeSharing(context.Background(), flowIDVar))

	s.sharing.EXPECT().DeletePoliciesForResource(mock.Anything, FlowSharingType, "f2").
		Return(&tidcommon.InternalServerError)
	svcErr := s.service.removeSharing(context.Background(), "f2")
	s.Require().NotNil(svcErr, "the caller is told, so a retry can finish the clean up")
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *SharingVariablesTestSuite) TestPolicyConnectionsMustBeVisibleToTheInitiatingUnit() {
	decl := newFlowSharingDeclaration(s.store, s.sharing)
	s.sharing.EXPECT().IsVisible(mock.Anything, connection.ConnectionSharingType, "c-seen", "child-ou").
		Return(true, nil)
	s.sharing.EXPECT().IsVisible(mock.Anything, connection.ConnectionSharingType, "c-unrelated", "child-ou").
		Return(false, nil)

	s.Nil(decl.ValidateMembers(context.Background(), flowIDVar, "variables.social", "child-ou",
		[]string{"c-seen"}))

	svcErr := decl.ValidateMembers(context.Background(), flowIDVar, "variables.social", "child-ou",
		[]string{"c-seen", "c-unrelated"})
	s.Require().NotNil(svcErr)
	s.Equal(ErrorInvalidSharingPolicy.Code, svcErr.Code)
}

func (s *SharingVariablesTestSuite) TestOnlyConnectionVariablesAreCheckedForVisibility() {
	decl := newFlowSharingDeclaration(s.store, s.sharing)

	s.Nil(decl.ValidateMembers(context.Background(), flowIDVar, "other.social", "child-ou", []string{"x"}))
	s.Nil(decl.ValidateMembers(context.Background(), flowIDVar, "variables.mfa", "child-ou", []string{"SMS_OTP"}))
	s.Nil(decl.ValidateMembers(context.Background(), flowIDVar, "variables.gone", "child-ou", []string{"x"}))
}

func (s *SharingVariablesTestSuite) TestVisibilityCheckFailuresAreNotRefusals() {
	decl := newFlowSharingDeclaration(s.store, s.sharing)
	s.sharing.EXPECT().IsVisible(mock.Anything, connection.ConnectionSharingType, "c1", "child-ou").
		Return(false, &tidcommon.InternalServerError)
	svcErr := decl.ValidateMembers(context.Background(), flowIDVar, "variables.social", "child-ou", []string{"c1"})
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)

	s.store.EXPECT().GetFlowByID(mock.Anything, "missing").Return(nil, errFlowNotFound)
	svcErr = decl.ValidateMembers(context.Background(), "missing", "variables.social", "child-ou", []string{"c1"})
	s.Require().NotNil(svcErr)
	s.Equal(ErrorFlowNotFound.Code, svcErr.Code)
}

func (s *SharingVariablesTestSuite) TestTheFlowDeclaresItsVariablesToTheFramework() {
	decl := newFlowSharingDeclaration(s.store, s.sharing)

	s.Equal(FlowSharingType, decl.ResourceType())
	fields := decl.Fields()
	s.Require().Len(fields, 1)
	s.Equal("variables", fields[0].Key)
	s.True(fields[0].DynamicKeys)
	s.Require().NotNil(fields[0].Default)
	s.True(fields[0].Default.Editable, "the owner may open any variable, and a policy narrows from there")

	owner, svcErr := decl.OwningOUID(context.Background(), flowIDVar)
	s.Require().Nil(svcErr)
	s.Equal(ownerOU, owner)

	s.store.EXPECT().GetFlowByID(mock.Anything, "missing").Return(nil, errFlowNotFound)
	_, svcErr = decl.OwningOUID(context.Background(), "missing")
	s.Require().NotNil(svcErr)
	s.Equal(ErrorFlowNotFound.Code, svcErr.Code)

	s.store.EXPECT().GetFlowByID(mock.Anything, "broken").Return(nil, assertAnError)
	_, svcErr = decl.OwningOUID(context.Background(), "broken")
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

// ---- Overlay rules and variables ----

func (s *SharingVariablesTestSuite) TestGetFlowOverlayRules() {
	s.shareWith(map[string]sharing.OverlayRule{
		sharingKey("supportEmail"): {Editable: true},
		sharingKey("mfa"):          {Editable: false, Value: strs("SMS_OTP")},
	})
	s.sharing.EXPECT().ResolveOverlayRules(mock.Anything, FlowSharingType, flowIDVar, childOU).
		Return(sharing.ResolvedOverlay{OUID: childOU, Visible: true, PolicyIDs: []string{"p1"}}, nil)

	overlay, svcErr := s.service.GetFlowOverlayRules(context.Background(), flowIDVar, childOU)

	s.Require().Nil(svcErr)
	s.Equal([]string{"p1"}, overlay.PolicyIDs)
	s.Len(overlay.Rules, 2, "a variable no policy names has no entry")
	s.True(overlay.Rules[sharingKey("supportEmail")].Editable)
	s.Equal([]string{"SMS_OTP"}, *overlay.Rules[sharingKey("mfa")].Value)
}

func (s *SharingVariablesTestSuite) TestGetFlowOverlayRulesOfTheOwnerNamesNothing() {
	s.sharing.EXPECT().ResolveOverlayRules(mock.Anything, FlowSharingType, flowIDVar, ownerOU).
		Return(sharing.ResolvedOverlay{OUID: ownerOU, Owned: true, Visible: true}, nil)
	for _, def := range declaredVariables(s.flow) {
		s.sharing.EXPECT().ResolveFieldRule(
			mock.Anything, FlowSharingType, flowIDVar, ownerOU, sharingKey(def.Identifier)).
			Return(sharing.FieldRule{OUID: ownerOU, Owned: true, Visible: true}, nil)
	}

	overlay, svcErr := s.service.GetFlowOverlayRules(context.Background(), flowIDVar, ownerOU)

	s.Require().Nil(svcErr)
	s.True(overlay.Owned)
	s.Empty(overlay.Rules)
}

func (s *SharingVariablesTestSuite) TestGetFlowOverlayRulesForAUnitNoPolicyReaches() {
	s.sharing.EXPECT().IsVisible(mock.Anything, FlowSharingType, flowIDVar, otherOU).Return(false, nil)

	_, svcErr := s.service.GetFlowOverlayRules(context.Background(), flowIDVar, otherOU)

	s.Require().NotNil(svcErr)
	s.Equal(ErrorFlowNotFound.Code, svcErr.Code)
}

func (s *SharingVariablesTestSuite) heldValues(values map[string][]string) {
	resolved := sharing.ResolvedValues{OUID: childOU, Visible: true, Values: map[string]sharing.ResolvedValue{}}
	for key, value := range values {
		resolved.Values[key] = sharing.ResolvedValue{Value: value, Source: sharing.SourceOverlay, Editable: true}
	}
	s.sharing.EXPECT().ResolveOverlayValues(mock.Anything, FlowSharingType, flowIDVar, childOU).Return(resolved, nil)
}

func (s *SharingVariablesTestSuite) TestGetVariablesReturnsTheUnitsOwnOverridesInTheirOwnType() {
	s.shareWith(map[string]sharing.OverlayRule{sharingKey("otp"): {Editable: true}})
	s.sharing.EXPECT().ResolveOverlayValues(mock.Anything, FlowSharingType, flowIDVar, childOU).Return(
		sharing.ResolvedValues{OUID: childOU, Visible: true, Values: map[string]sharing.ResolvedValue{
			sharingKey("otp"):          {Value: []string{"90"}, Source: sharing.SourceOverlay, Editable: true},
			sharingKey("mfa"):          {Value: []string{"SMS_OTP"}, Source: sharing.SourceRule},
			sharingKey("supportEmail"): {Value: []string{"a@b.co"}, Source: sharing.SourceOverlay},
		}}, nil)

	resp, svcErr := s.service.GetVariables(context.Background(), flowIDVar, childOU)

	s.Require().Nil(svcErr)
	s.Equal(map[string]interface{}{"otp": 90.0, "supportEmail": "a@b.co"}, resp.Values,
		"what a policy fixes is not an override")
}

func (s *SharingVariablesTestSuite) TestGetVariablesOfAUnitThatSetNothing() {
	s.shareWith(nil)
	s.heldValues(nil)

	resp, svcErr := s.service.GetVariables(context.Background(), flowIDVar, childOU)

	s.Require().Nil(svcErr)
	s.NotNil(resp.Values)
	s.Empty(resp.Values)
}

func (s *SharingVariablesTestSuite) TestSetVariablesStoresEachValueAndDropsTheOnesLeftOut() {
	s.shareWith(map[string]sharing.OverlayRule{
		sharingKey("supportEmail"): {Editable: true},
		sharingKey("otp"):          {Editable: true, AllowedValues: strs("60", "90", "120")},
		sharingKey("mfa"):          {Editable: true},
	})
	// What the unit held before: an email and an MFA choice. The request keeps the email and the otp, and leaves
	// MFA out.
	s.sharing.EXPECT().ResolveOverlayValues(mock.Anything, FlowSharingType, flowIDVar, childOU).Return(
		sharing.ResolvedValues{OUID: childOU, Visible: true, Values: map[string]sharing.ResolvedValue{
			sharingKey("supportEmail"): {Value: []string{"old@b.co"}, Source: sharing.SourceOverlay, Editable: true},
			sharingKey("mfa"):          {Value: []string{"SMS_OTP"}, Source: sharing.SourceOverlay, Editable: true},
		}}, nil).Once()
	// One call replaces the values: the two the request names are set, and the rest of the variables, MFA among
	// them, are named so that what the unit held for them goes back to the rule.
	s.sharing.EXPECT().ReplaceOverlayValues(mock.Anything, FlowSharingType, flowIDVar, childOU,
		[]string{sharingKey("supportEmail"), sharingKey("otp"), sharingKey("mfa"), sharingKey("social")},
		map[string][]string{
			sharingKey("supportEmail"): {"help@emea.example"},
			sharingKey("otp"):          {"90"},
		}).Return(nil)

	resp, svcErr := s.service.SetVariables(context.Background(), flowIDVar, childOU, SetVariablesRequest{
		Values: map[string]interface{}{"supportEmail": "help@emea.example", "otp": 90.0},
	})

	s.Require().Nil(svcErr)
	s.NotNil(resp)
}

// The values are replaced in one call to the sharing store, so a refusal of any of them leaves the unit with what it
// held, and the refusal names the variable.
func (s *SharingVariablesTestSuite) TestSetVariablesReportsTheVariableTheFrameworkRefused() {
	s.shareWith(map[string]sharing.OverlayRule{
		sharingKey("supportEmail"): {Editable: true},
		sharingKey("otp"):          {Editable: true, AllowedValues: strs("60", "90", "120")},
	})
	s.sharing.EXPECT().ReplaceOverlayValues(mock.Anything, FlowSharingType, flowIDVar, childOU, mock.Anything,
		mock.Anything).Return(&sharing.OverlayReplaceError{
		FieldKey: sharingKey("otp"), Err: &sharing.ErrorFieldNotEditable})

	resp, svcErr := s.service.SetVariables(context.Background(), flowIDVar, childOU, SetVariablesRequest{
		Values: map[string]interface{}{"supportEmail": "help@emea.example", "otp": 90.0},
	})

	s.Nil(resp)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorVariableNotEditable.Code, svcErr.Code)
	s.Contains(svcErr.ErrorDescription.Params["variable"], "otp")
}

func (s *SharingVariablesTestSuite) TestSetVariablesReportsAFailureOfTheStoreAsAServerError() {
	s.shareWith(map[string]sharing.OverlayRule{sharingKey("supportEmail"): {Editable: true}})
	s.sharing.EXPECT().ReplaceOverlayValues(mock.Anything, FlowSharingType, flowIDVar, childOU, mock.Anything,
		mock.Anything).Return(&sharing.OverlayReplaceError{Err: &tidcommon.InternalServerError})

	_, svcErr := s.service.SetVariables(context.Background(), flowIDVar, childOU, SetVariablesRequest{
		Values: map[string]interface{}{"supportEmail": "help@emea.example"},
	})

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *SharingVariablesTestSuite) TestSetVariablesChecksEveryValueBeforeWritingAny() {
	s.shareWith(map[string]sharing.OverlayRule{
		sharingKey("supportEmail"): {Editable: true},
		sharingKey("otp"):          {Editable: true, AllowedValues: strs("60", "90")},
		sharingKey("mfa"):          {Editable: false, Value: strs("SMS_OTP")},
		sharingKey("social"):       {Editable: true},
	})
	type refusal struct {
		name   string
		values map[string]interface{}
		code   string
	}
	cases := []refusal{
		{"a variable the flow does not declare", map[string]interface{}{"nope": "x"}, ErrorUnknownVariable.Code},
		{"a variable the unit may not set", map[string]interface{}{"mfa": "EMAIL_OTP"}, ErrorVariableNotEditable.Code},
		{"a null", map[string]interface{}{"supportEmail": nil}, ErrorInvalidVariableValue.Code},
		{"a value of the wrong type", map[string]interface{}{"otp": "ninety"}, ErrorInvalidVariableValue.Code},
		{"a value the owner does not allow", map[string]interface{}{"otp": 120.0}, ErrorInvalidVariableValue.Code},
		{"a value that is not valid", map[string]interface{}{"supportEmail": "not-an-email"},
			ErrorInvalidVariableValue.Code},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			_, svcErr := s.service.SetVariables(
				context.Background(), flowIDVar, childOU, SetVariablesRequest{Values: tc.values})

			s.Require().NotNil(svcErr)
			s.Equal(tc.code, svcErr.Code)
		})
	}
}

func (s *SharingVariablesTestSuite) TestSetVariablesRefusesAVariableNoPolicyNames() {
	s.shareWith(nil)

	_, svcErr := s.service.SetVariables(context.Background(), flowIDVar, childOU, SetVariablesRequest{
		Values: map[string]interface{}{"supportEmail": "help@emea.example"}})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorUnknownVariable.Code, svcErr.Code)
}

func (s *SharingVariablesTestSuite) TestSetVariablesChecksTheKindAndExistenceOfAConnection() {
	s.shareWith(map[string]sharing.OverlayRule{sharingKey("social"): {Editable: true}})
	s.idps.EXPECT().GetIdentityProvider(mock.Anything, "c-github").
		Return(&providers.IDPDTO{ID: "c-github", Type: providers.IDPTypeGitHub, OUID: childOU}, nil)
	s.idps.EXPECT().GetIdentityProvider(mock.Anything, "c-gone").Return(nil, &ErrorFlowNotFound)

	for id, reason := range map[string]string{
		"c-github": "does not take", "c-gone": "not an existing connection"} {
		_, svcErr := s.service.SetVariables(context.Background(), flowIDVar, childOU,
			SetVariablesRequest{Values: map[string]interface{}{"social": id}})

		s.Require().NotNil(svcErr, id)
		s.Equal(ErrorInvalidVariableValue.Code, svcErr.Code)
		s.Contains(svcErr.ErrorDescription.Params["reason"], reason)
	}
}

func (s *SharingVariablesTestSuite) expectConnection(id string) {
	s.idps.EXPECT().GetIdentityProvider(mock.Anything, id).
		Return(&providers.IDPDTO{ID: id, Type: providers.IDPTypeGoogle, OUID: otherOU}, nil)
}

// A unit sets a connection variable to one it owns or was shared, not to any connection that exists.
func (s *SharingVariablesTestSuite) TestSetVariablesRefusesAConnectionTheUnitCannotUse() {
	s.shareWith(map[string]sharing.OverlayRule{sharingKey("social"): {Editable: true}})
	s.expectConnection("c-other")
	s.sharing.EXPECT().IsVisible(mock.Anything, connection.ConnectionSharingType, "c-other", childOU).
		Return(false, nil)

	_, svcErr := s.service.SetVariables(context.Background(), flowIDVar, childOU,
		SetVariablesRequest{Values: map[string]interface{}{"social": "c-other"}})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorInvalidVariableValue.Code, svcErr.Code)
	s.Contains(svcErr.ErrorDescription.Params["reason"], "neither owns nor was shared")
}

func (s *SharingVariablesTestSuite) TestSetVariablesAcceptsAConnectionTheUnitCanUse() {
	s.shareWith(map[string]sharing.OverlayRule{sharingKey("social"): {Editable: true}})
	s.expectConnection("c-shared")
	s.sharing.EXPECT().IsVisible(mock.Anything, connection.ConnectionSharingType, "c-shared", childOU).
		Return(true, nil)
	s.sharing.EXPECT().ResolveOverlayValues(mock.Anything, FlowSharingType, flowIDVar, childOU).Return(
		sharing.ResolvedValues{OUID: childOU, Visible: true}, nil).Once()
	s.sharing.EXPECT().ReplaceOverlayValues(mock.Anything, FlowSharingType, flowIDVar, childOU, mock.Anything,
		map[string][]string{sharingKey("social"): {"c-shared"}}).Return(nil)

	resp, svcErr := s.service.SetVariables(context.Background(), flowIDVar, childOU,
		SetVariablesRequest{Values: map[string]interface{}{"social": "c-shared"}})

	s.Require().Nil(svcErr)
	s.NotNil(resp)
}

func (s *SharingVariablesTestSuite) TestSetVariablesReportsAFailureToCheckWhetherAConnectionIsVisible() {
	s.shareWith(map[string]sharing.OverlayRule{sharingKey("social"): {Editable: true}})
	s.expectConnection("c-shared")
	s.sharing.EXPECT().IsVisible(mock.Anything, connection.ConnectionSharingType, "c-shared", childOU).
		Return(false, &tidcommon.InternalServerError)

	_, svcErr := s.service.SetVariables(context.Background(), flowIDVar, childOU,
		SetVariablesRequest{Values: map[string]interface{}{"social": "c-shared"}})

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *SharingVariablesTestSuite) TestSetVariablesOfAUnitTheFlowWasNotSharedWith() {
	s.sharing.EXPECT().IsVisible(mock.Anything, FlowSharingType, flowIDVar, otherOU).Return(false, nil)

	_, svcErr := s.service.SetVariables(context.Background(), flowIDVar, otherOU, SetVariablesRequest{
		Values: map[string]interface{}{}})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorFlowNotFound.Code, svcErr.Code)
}

func (s *SharingVariablesTestSuite) TestEffectiveVariableValuesAreWhatIsNotTheOwnersDefault() {
	s.shareWith(map[string]sharing.OverlayRule{
		sharingKey("supportEmail"): {Editable: true},
		sharingKey("otp"):          {Editable: true, Value: strs("60")},
		sharingKey("mfa"):          {Editable: false, Value: strs("SMS_OTP")},
	})
	s.sharing.EXPECT().ResolveOverlayValues(mock.Anything, FlowSharingType, flowIDVar, childOU).Return(
		sharing.ResolvedValues{OUID: childOU, Visible: true, Values: map[string]sharing.ResolvedValue{
			sharingKey("supportEmail"): {
				Value: []string{"help@emea.example"}, Source: sharing.SourceOverlay, Editable: true},
		}}, nil)

	values, svcErr := s.service.GetEffectiveVariableValues(context.Background(), flowIDVar, ownApp(childOU), childOU)

	s.Require().Nil(svcErr)
	s.Equal(map[string]string{
		"supportEmail": "help@emea.example", // its own override
		"otp":          "60",                // the value the policy starts it with
		"mfa":          "SMS_OTP",           // the value the policy pins
	}, values, "a variable no policy names, and one the unit left alone, stay at the owner's default")
}

// ownApp is an application owned by the unit, which holds values for its own applications without being shared them.
func ownApp(ouID string) providers.Application {
	return providers.Application{ID: "app-1", OUID: ouID}
}

// An application of another unit runs on the values of a unit it was shared with, which is how a user who signs in
// under that unit gets its values.
func (s *SharingVariablesTestSuite) TestEffectiveVariableValuesOfAUnitAnApplicationWasSharedWithAreItsOwn() {
	s.shareWith(map[string]sharing.OverlayRule{sharingKey("otp"): {Editable: true, Value: strs("60")}})
	s.sharing.EXPECT().IsVisible(mock.Anything, applicationSharingType, "app-1", childOU).Return(true, nil)
	s.sharing.EXPECT().ResolveOverlayValues(mock.Anything, FlowSharingType, flowIDVar, childOU).
		Return(sharing.ResolvedValues{OUID: childOU, Visible: true}, nil)

	values, svcErr := s.service.GetEffectiveVariableValues(
		context.Background(), flowIDVar, ownApp(ownerOU), childOU)

	s.Require().Nil(svcErr)
	s.Equal(map[string]string{"otp": "60"}, values)
}

func (s *SharingVariablesTestSuite) TestEffectiveVariableValuesIgnoreAUnitAnApplicationWasNotSharedWith() {
	s.sharing.EXPECT().IsVisible(mock.Anything, applicationSharingType, "app-1", childOU).Return(false, nil)

	values, svcErr := s.service.GetEffectiveVariableValues(
		context.Background(), flowIDVar, ownApp(ownerOU), childOU)

	s.Require().Nil(svcErr)
	s.Empty(values, "naming a unit is not enough to run on its values")
	s.sharing.AssertNotCalled(s.T(), "ResolveFieldRules", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything)
}

func (s *SharingVariablesTestSuite) TestEffectiveVariableValuesReportAFailureToTellIfTheApplicationIsShared() {
	s.sharing.EXPECT().IsVisible(mock.Anything, applicationSharingType, "app-1", childOU).
		Return(false, &tidcommon.InternalServerError)

	_, svcErr := s.service.GetEffectiveVariableValues(context.Background(), flowIDVar, ownApp(ownerOU), childOU)

	s.Require().NotNil(svcErr)
}

func (s *SharingVariablesTestSuite) TestEffectiveVariableValuesOfTheOwnerOrAStrangerAreEmpty() {
	s.sharing.EXPECT().IsVisible(mock.Anything, FlowSharingType, flowIDVar, ownerOU).Return(true, nil)
	values, svcErr := s.service.GetEffectiveVariableValues(context.Background(), flowIDVar, ownApp(ownerOU), ownerOU)
	s.Require().Nil(svcErr)
	s.Empty(values, "the owner holds the defaults")

	// A unit the flow is not visible to is answered from the cached visibility, before anything is resolved.
	s.sharing.EXPECT().IsVisible(mock.Anything, FlowSharingType, flowIDVar, otherOU).Return(false, nil)
	values, svcErr = s.service.GetEffectiveVariableValues(context.Background(), flowIDVar, ownApp(otherOU), otherOU)
	s.Require().Nil(svcErr)
	s.Empty(values, "a flow that was not shared with the unit")

	values, svcErr = s.service.GetEffectiveVariableValues(context.Background(), flowIDVar, ownApp(""), "")
	s.Require().Nil(svcErr)
	s.Empty(values, "no unit known yet")
}

func (s *SharingVariablesTestSuite) TestEffectiveVariableValuesResolveTheRulesOfAllVariablesTogether() {
	s.shareWith(map[string]sharing.OverlayRule{sharingKey("otp"): {Editable: true, Value: strs("60")}})
	// The values the unit saved are read, as a policy names a variable. The rules are asked for once, for every
	// variable, and never one by one.
	s.sharing.EXPECT().ResolveOverlayValues(mock.Anything, FlowSharingType, flowIDVar, childOU).
		Return(sharing.ResolvedValues{OUID: childOU, Visible: true}, nil).Once()

	values, svcErr := s.service.GetEffectiveVariableValues(context.Background(), flowIDVar, ownApp(childOU), childOU)

	s.Require().Nil(svcErr)
	s.Equal(map[string]string{"otp": "60"}, values)
	s.sharing.AssertNotCalled(s.T(), "ResolveFieldRule", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything)
	s.sharing.AssertNumberOfCalls(s.T(), "ResolveFieldRules", 1)
}

func (s *SharingVariablesTestSuite) TestEffectiveVariableValuesSkipTheSavedValuesWhenNoPolicyNamesAVariable() {
	s.shareWith(map[string]sharing.OverlayRule{})

	values, svcErr := s.service.GetEffectiveVariableValues(context.Background(), flowIDVar, ownApp(childOU), childOU)

	s.Require().Nil(svcErr)
	s.Empty(values)
	s.sharing.AssertNotCalled(s.T(), "ResolveOverlayValues", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything)
}

func (s *SharingVariablesTestSuite) TestEffectiveVariableValuesReportAFailureToResolve() {
	s.sharing.EXPECT().IsVisible(mock.Anything, FlowSharingType, flowIDVar, childOU).
		Return(false, &tidcommon.InternalServerError).Once()
	_, svcErr := s.service.GetEffectiveVariableValues(context.Background(), flowIDVar, ownApp(childOU), childOU)
	s.Require().NotNil(svcErr)

	s.sharing.EXPECT().IsVisible(mock.Anything, FlowSharingType, flowIDVar, childOU).Return(true, nil).Once()
	s.sharing.EXPECT().ResolveFieldRules(mock.Anything, FlowSharingType, flowIDVar, childOU, mock.Anything).
		Return(nil, &tidcommon.InternalServerError).Once()
	_, svcErr = s.service.GetEffectiveVariableValues(context.Background(), flowIDVar, ownApp(childOU), childOU)
	s.Require().NotNil(svcErr)
}

// ---- Existence ----

// What an organization unit is told about a flow it neither owns nor was shared has to be what it is told about a flow
// that does not exist, or the answers say which flows exist.
func (s *SharingVariablesTestSuite) TestAFlowNoUnitCanSeeAnswersAsAMissingFlowDoes() {
	s.sharing.EXPECT().IsVisible(mock.Anything, FlowSharingType, flowIDVar, otherOU).Return(false, nil)
	s.store.EXPECT().GetFlowByID(mock.Anything, "f-missing").Return(nil, errFlowNotFound)
	ctx := context.Background()

	operations := map[string]func(flowID string) *tidcommon.ServiceError{
		"GetFlowForOU": func(flowID string) *tidcommon.ServiceError {
			_, svcErr := s.service.GetFlowForOU(ctx, flowID, otherOU)
			return svcErr
		},
		"GetVariables": func(flowID string) *tidcommon.ServiceError {
			_, svcErr := s.service.GetVariables(ctx, flowID, otherOU)
			return svcErr
		},
		"SetVariables": func(flowID string) *tidcommon.ServiceError {
			_, svcErr := s.service.SetVariables(ctx, flowID, otherOU, SetVariablesRequest{
				Values: map[string]interface{}{}})
			return svcErr
		},
		"GetFlowOverlayRules": func(flowID string) *tidcommon.ServiceError {
			_, svcErr := s.service.GetFlowOverlayRules(ctx, flowID, otherOU)
			return svcErr
		},
		"CheckFlowAccess read": func(flowID string) *tidcommon.ServiceError {
			return s.service.CheckFlowAccess(ctx, flowID, otherOU, false)
		},
		"CheckFlowAccess write": func(flowID string) *tidcommon.ServiceError {
			return s.service.CheckFlowAccess(ctx, flowID, otherOU, true)
		},
	}
	for name, operation := range operations {
		s.Run(name, func() {
			unshared := operation(flowIDVar)
			missing := operation("f-missing")

			s.Require().NotNil(unshared)
			s.Equal(&ErrorFlowNotFound, unshared)
			s.Equal(missing, unshared, "an unshared flow answers as a missing one does")
		})
	}
}

// A flow that is already gone has nothing left but its policies, and a delete that finds them removes them.
func (s *SharingVariablesTestSuite) TestPurgeOrphanedSharingRemovesWhatIsLeft() {
	s.sharing.EXPECT().ListPolicies(mock.Anything, FlowSharingType, "gone").
		Return([]sharing.Policy{{ID: "p1"}}, nil)
	s.sharing.EXPECT().DeletePoliciesForResource(mock.Anything, FlowSharingType, "gone").Return(nil)

	s.Nil(s.service.purgeOrphanedSharing(context.Background(), "gone"))
}

func (s *SharingVariablesTestSuite) TestPurgeOrphanedSharingWithNothingLeftDoesNothing() {
	s.sharing.EXPECT().ListPolicies(mock.Anything, FlowSharingType, "gone").Return(nil, nil)

	s.Nil(s.service.purgeOrphanedSharing(context.Background(), "gone"))
}

func (s *SharingVariablesTestSuite) TestPurgeOrphanedSharingReportsAFailureToRead() {
	s.sharing.EXPECT().ListPolicies(mock.Anything, FlowSharingType, "gone").
		Return(nil, &tidcommon.InternalServerError)

	svcErr := s.service.purgeOrphanedSharing(context.Background(), "gone")
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

// A policy left behind by a delete that could not clean up is not hidden: the delete fails, and a retry finishes it.
func (s *SharingVariablesTestSuite) TestDeleteFlowFailsWhenItsPoliciesCannotBeRemoved() {
	graphs := newGraphBuilderInterfaceMock(s.T())
	graphs.EXPECT().InvalidateCache(mock.Anything, flowIDVar).Return()
	s.service.graphBuilder = graphs
	s.store.EXPECT().DeleteFlow(mock.Anything, flowIDVar).Return(nil)
	s.sharing.EXPECT().DeletePoliciesForResource(mock.Anything, FlowSharingType, flowIDVar).
		Return(&tidcommon.InternalServerError)

	svcErr := s.service.DeleteFlow(context.Background(), flowIDVar)

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *SharingVariablesTestSuite) TestDeleteFlowThatIsAlreadyGoneFinishesTheCleanUp() {
	s.store.EXPECT().GetFlowByID(mock.Anything, "gone").Return(nil, errFlowNotFound)
	s.sharing.EXPECT().ListPolicies(mock.Anything, FlowSharingType, "gone").
		Return([]sharing.Policy{{ID: "p1"}}, nil)
	s.sharing.EXPECT().DeletePoliciesForResource(mock.Anything, FlowSharingType, "gone").Return(nil)

	s.Nil(s.service.DeleteFlow(context.Background(), "gone"))
}

// ---- Replaying policies from a document ----

func (s *SharingVariablesTestSuite) TestValidateSharingPoliciesChecksTheTermsAgainstTheVariables() {
	valid := sharing.PolicyRequest{ChildUnits: &sharing.ChildUnitsRequest{
		Mode: sharing.ChildUnitsAll,
		OverlayRules: map[string]sharing.OverlayRule{
			"variables.mfa": {Editable: true, AllowedValues: strs("NONE", "SMS_OTP")},
		}}}
	s.Nil(s.service.ValidateSharingPolicies(context.Background(), flowIDVar, []sharing.PolicyRequest{valid}))

	notAVariable := sharing.PolicyRequest{ChildUnits: &sharing.ChildUnitsRequest{
		Mode: sharing.ChildUnitsAll,
		OverlayRules: map[string]sharing.OverlayRule{
			"variables.supportEmail.label": {Editable: true},
		}}}
	svcErr := s.service.ValidateSharingPolicies(context.Background(), flowIDVar,
		[]sharing.PolicyRequest{valid, notAVariable})
	s.Require().NotNil(svcErr)
	s.Equal(ErrorInvalidSharingPolicy.Code, svcErr.Code)
}

func (s *SharingVariablesTestSuite) TestValidateSharingPoliciesOfAFlowThatDoesNotExist() {
	s.store.EXPECT().GetFlowByID(mock.Anything, "missing").Return(nil, errFlowNotFound)

	svcErr := s.service.ValidateSharingPolicies(context.Background(), "missing", nil)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorFlowNotFound.Code, svcErr.Code)
}

// ---- Declared in the flow's own document ----

const declaredFlowDocument = `id: flow-vars
handle: acme-login
name: Acme Login
flowType: AUTHENTICATION
ouId: ou-owner
nodes:
  - id: start
    type: START
    onSuccess: vars
  - id: vars
    type: VARIABLES
    onSuccess: end
    inputs:
      - identifier: mfa
        type: SELECT
        defaultValue: NONE
        options: [NONE, SMS_OTP]
  - id: end
    type: END
sharingPolicies:
  - childUnits:
      mode: all
      overlayRules:
        variables.mfa:
          editable: true
          allowedValues: [NONE]
`

func (s *SharingVariablesTestSuite) TestADocumentWithoutPoliciesDeclaresNone() {
	declared, err := s.service.parseFlowSharing([]byte("id: flow-vars\nhandle: h\nname: n\nflowType: AUTHENTICATION\n"))

	s.Require().NoError(err)
	s.Equal("flow-vars", declared.ResourceID)
	s.Empty(declared.Policies)
}

func (s *SharingVariablesTestSuite) TestADocumentDeclaresItsPoliciesAgainstTheUnitThatOwnsIt() {
	s.ous.EXPECT().ResolveOwner(mock.Anything, ownerOU, "", mock.Anything).Return(ownerOU, nil)

	declared, err := s.service.parseFlowSharing([]byte(declaredFlowDocument))

	s.Require().NoError(err)
	s.Equal("flow-vars", declared.ResourceID)
	s.Equal("Acme Login", declared.ResourceName)
	s.Equal(ownerOU, declared.OwningOUID)
	s.Len(declared.Policies, 1)
}

func (s *SharingVariablesTestSuite) TestADocumentWithPoliciesNeedsAnOwner() {
	_, err := s.service.parseFlowSharing([]byte(
		"id: f\nname: n\nsharingPolicies:\n  - childUnits:\n      mode: all\n"))

	s.Error(err)
}

func (s *SharingVariablesTestSuite) TestADocumentWhoseTermsDoNotFitItsVariablesFailsTheLoad() {
	s.ous.EXPECT().ResolveOwner(mock.Anything, ownerOU, "", mock.Anything).Return(ownerOU, nil)
	bad := declaredFlowDocumentWith("          allowedValues: [BOGUS]\n")

	_, err := s.service.parseFlowSharing([]byte(bad))

	s.Error(err)
}

func declaredFlowDocumentWith(rule string) string {
	return declaredFlowDocument[:len(declaredFlowDocument)-len("          allowedValues: [NONE]\n")] + rule
}

// withFlowStore runs a test with the flows held by the given store mode.
func (s *SharingVariablesTestSuite) withFlowStore(mode serverconst.StoreMode, run func()) {
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime("test", &config.Config{
		Flow: engineconfig.FlowConfig{Store: string(mode)},
	}))
	defer config.ResetServerRuntime()
	run()
}

// A mutable store keeps its flows in the database, so there are no flow files to read policies from.
func (s *SharingVariablesTestSuite) TestNoPoliciesAreLoadedWhenFlowsAreNotDeclared() {
	s.withFlowStore(serverconst.StoreModeMutable, func() {
		s.Nil(s.service.loadDeclarativeSharing())
	})
}

// Flows are read from files in declarative and composite mode, and the policies their documents carry with them.
func (s *SharingVariablesTestSuite) TestPoliciesAreLoadedWhenFlowsAreDeclared() {
	for _, mode := range []serverconst.StoreMode{serverconst.StoreModeDeclarative, serverconst.StoreModeComposite} {
		s.withFlowStore(mode, func() {
			s.sharing.EXPECT().LoadDeclarativeResources(mock.Anything, mock.MatchedBy(
				func(cfg sharing.DeclarativeLoaderConfig) bool {
					return cfg.ResourceType == FlowSharingType && cfg.DirectoryName == "flows"
				})).Return(nil).Once()

			s.Nil(s.service.loadDeclarativeSharing())
		})
	}
}

func (s *SharingVariablesTestSuite) TestAFailureToLoadDeclaredPoliciesIsReported() {
	s.withFlowStore(serverconst.StoreModeDeclarative, func() {
		s.sharing.EXPECT().LoadDeclarativeResources(mock.Anything, mock.Anything).Return(assertAnError).Once()

		s.Error(s.service.loadDeclarativeSharing())
	})
}

func (s *SharingVariablesTestSuite) TestTheSharingConfigNamesTheFlowsDirectory() {
	cfg := s.service.makeFlowSharingConfig()

	s.Equal(FlowSharingType, cfg.ResourceType)
	s.Equal("flows", cfg.DirectoryName)
	s.NotNil(cfg.Parser)
}

// ---- Export ----

func (s *SharingVariablesTestSuite) TestAnExportCarriesThePoliciesThatGovernTheFlow() {
	flows := NewFlowMgtServiceInterfaceMock(s.T())
	flows.EXPECT().GetFlow(mock.Anything, flowIDVar).Return(s.flow, nil)
	request := sharing.PolicyRequest{ChildUnits: &sharing.ChildUnitsRequest{Mode: sharing.ChildUnitsAll}}
	s.sharing.EXPECT().ExportPolicies(mock.Anything, FlowSharingType, flowIDVar).
		Return([]sharing.ReplayablePolicy{{InitiatingOUID: ownerOU, Request: request}}, nil)
	s.sharing.EXPECT().ListOverlayValues(mock.Anything, FlowSharingType, flowIDVar).
		Return(map[string]map[string][]string{
			childOU: {sharingKey("supportEmail"): {"help@child.example"}},
			otherOU: {},
		}, nil)
	flows.EXPECT().GetVariables(mock.Anything, flowIDVar, childOU).
		Return(&VariablesResponse{Values: map[string]interface{}{"supportEmail": "help@child.example"}}, nil)
	exporter := newFlowGraphExporter(flows, s.sharing)

	resource, name, svcErr := exporter.GetResourceByID(context.Background(), flowIDVar)

	s.Require().Nil(svcErr)
	s.Equal("Acme Login", name)
	exported, ok := resource.(*flowDeclarativeResource)
	s.Require().True(ok)
	s.Equal([]sharing.PolicyRequest{request}, exported.SharingPolicies)
	s.Equal([]FlowUnitValues{{OUID: childOU, Values: map[string]interface{}{"supportEmail": "help@child.example"}}},
		exported.VariableValues, "a unit holding nothing is left out")

	document, err := yaml.Marshal(exported)
	s.Require().NoError(err)
	var node yaml.Node
	s.Require().NoError(yaml.Unmarshal(document, &node))
	policies, values, err := ParseFlowSharing(node.Content[0])
	s.Require().NoError(err)
	s.Equal([]sharing.PolicyRequest{request}, policies, "an export reads back as the document it was written as")
	s.Equal(exported.VariableValues, values)
}

func (s *SharingVariablesTestSuite) TestAnExportOfAFlowNoUnitSavedValuesForCarriesNone() {
	flows := NewFlowMgtServiceInterfaceMock(s.T())
	flows.EXPECT().GetFlow(mock.Anything, flowIDVar).Return(s.flow, nil)
	s.sharing.EXPECT().ExportPolicies(mock.Anything, FlowSharingType, flowIDVar).
		Return([]sharing.ReplayablePolicy{}, nil)
	s.sharing.EXPECT().ListOverlayValues(mock.Anything, FlowSharingType, flowIDVar).
		Return(map[string]map[string][]string{}, nil)

	resource, _, svcErr := newFlowGraphExporter(flows, s.sharing).GetResourceByID(context.Background(), flowIDVar)

	s.Require().Nil(svcErr)
	s.Empty(resource.(*flowDeclarativeResource).VariableValues)
}

// The values are what a unit chose within its terms, so an export that cannot read them fails rather than thinning.
func (s *SharingVariablesTestSuite) TestAnExportFailsWhenTheValuesCannotBeRead() {
	flows := NewFlowMgtServiceInterfaceMock(s.T())
	flows.EXPECT().GetFlow(mock.Anything, flowIDVar).Return(s.flow, nil).Times(2)
	s.sharing.EXPECT().ExportPolicies(mock.Anything, FlowSharingType, flowIDVar).
		Return([]sharing.ReplayablePolicy{}, nil).Times(2)
	exporter := newFlowGraphExporter(flows, s.sharing)

	s.sharing.EXPECT().ListOverlayValues(mock.Anything, FlowSharingType, flowIDVar).
		Return(nil, &tidcommon.InternalServerError).Once()
	_, _, svcErr := exporter.GetResourceByID(context.Background(), flowIDVar)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)

	s.sharing.EXPECT().ListOverlayValues(mock.Anything, FlowSharingType, flowIDVar).
		Return(map[string]map[string][]string{childOU: {sharingKey("supportEmail"): {"x"}}}, nil).Once()
	flows.EXPECT().GetVariables(mock.Anything, flowIDVar, childOU).Return(nil, &tidcommon.InternalServerError)
	_, _, svcErr = exporter.GetResourceByID(context.Background(), flowIDVar)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

// A document that silently lost its sharing would re-import as a flow nobody else can use, so the export fails.
func (s *SharingVariablesTestSuite) TestAnExportFailsWhenItsPoliciesCannotBeRead() {
	flows := NewFlowMgtServiceInterfaceMock(s.T())
	flows.EXPECT().GetFlow(mock.Anything, flowIDVar).Return(s.flow, nil)
	s.sharing.EXPECT().ExportPolicies(mock.Anything, FlowSharingType, flowIDVar).
		Return(nil, &tidcommon.InternalServerError)
	exporter := newFlowGraphExporter(flows, s.sharing)

	_, _, svcErr := exporter.GetResourceByID(context.Background(), flowIDVar)

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *SharingVariablesTestSuite) TestAFlowExportsAsItsBareDefinitionToo() {
	exporter := newFlowGraphExporter(nil, nil)

	flow, ok := flowOf(&providers.CompleteFlowDefinition{ID: "f"})
	s.True(ok)
	s.Equal("f", flow.ID)
	_, ok = flowOf("not a flow")
	s.False(ok)
	s.NotNil(exporter)
}
