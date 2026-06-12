/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/inboundclientmock"
)

const (
	testAppIDGraph               = "test-app"
	testUserOnboardingFlowHandle = "onboarding-flow"
)

type FlowExecServiceGraphTestSuite struct {
	suite.Suite
	testGraph core.GraphInterface
	logger    *log.Logger
}

func TestFlowExecServiceGraphTestSuite(t *testing.T) {
	suite.Run(t, new(FlowExecServiceGraphTestSuite))
}

func (s *FlowExecServiceGraphTestSuite) SetupSuite() {
	s.testGraph = core.NewFlowFactory().CreateGraph("flow-1", common.FlowTypeAuthentication)
	s.logger = log.GetLogger()
}

func minimalFlowDefinition(id string, flowType common.FlowType) *common.CompleteFlowDefinition {
	return &common.CompleteFlowDefinition{
		ID:       id,
		Handle:   "test-handle",
		Name:     "Test Flow",
		FlowType: flowType,
		Nodes: []common.NodeDefinition{
			{ID: "start", Type: "START", OnSuccess: "end"},
			{ID: "end", Type: "END"},
		},
	}
}

func (s *FlowExecServiceGraphTestSuite) TestLoadGraph_GetFlowReturnsError() {
	mockProvider := NewFlowProviderInterfaceMock(s.T())
	mockProvider.EXPECT().GetFlow(mock.Anything, "flow-1").Return(nil, &serviceerror.InternalServerError)

	service := &flowExecService{flowProvider: mockProvider}

	graph, svcErr := service.loadGraph(context.Background(), "flow-1", s.logger)

	s.Nil(graph)
	s.NotNil(svcErr)
	s.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
}

func (s *FlowExecServiceGraphTestSuite) TestLoadGraph_GetFlowReturnsClientError() {
	clientErr := &serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "FLM-1003",
	}
	mockProvider := NewFlowProviderInterfaceMock(s.T())
	mockProvider.EXPECT().GetFlow(mock.Anything, "flow-1").Return(nil, clientErr)

	service := &flowExecService{flowProvider: mockProvider}

	graph, svcErr := service.loadGraph(context.Background(), "flow-1", s.logger)

	s.Nil(graph)
	s.Equal(clientErr, svcErr)
}

func (s *FlowExecServiceGraphTestSuite) TestLoadGraph_GetFlowReturnsNilFlowDefinition() {
	mockProvider := NewFlowProviderInterfaceMock(s.T())
	mockProvider.EXPECT().GetFlow(mock.Anything, "flow-1").Return(nil, nil)

	service := &flowExecService{flowProvider: mockProvider}

	graph, svcErr := service.loadGraph(context.Background(), "flow-1", s.logger)

	s.Nil(graph)
	s.NotNil(svcErr)
	s.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
}

func (s *FlowExecServiceGraphTestSuite) TestLoadGraph_GetGraphReturnsError() {
	mockProvider := NewFlowProviderInterfaceMock(s.T())
	mockProvider.EXPECT().GetFlow(mock.Anything, "flow-1").
		Return(minimalFlowDefinition("flow-1", common.FlowTypeAuthentication), nil)

	mockBuilder := coremock.NewGraphBuilderInterfaceMock(s.T())
	mockBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).
		Return(nil, &serviceerror.InternalServerError)

	service := &flowExecService{flowProvider: mockProvider, graphBuilder: mockBuilder}

	graph, svcErr := service.loadGraph(context.Background(), "flow-1", s.logger)

	s.Nil(graph)
	s.NotNil(svcErr)
	s.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
}

func (s *FlowExecServiceGraphTestSuite) TestLoadGraph_GetGraphReturnsNilGraph() {
	mockProvider := NewFlowProviderInterfaceMock(s.T())
	mockProvider.EXPECT().GetFlow(mock.Anything, "flow-1").
		Return(minimalFlowDefinition("flow-1", common.FlowTypeAuthentication), nil)

	mockBuilder := coremock.NewGraphBuilderInterfaceMock(s.T())
	mockBuilder.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(nil, nil)

	service := &flowExecService{flowProvider: mockProvider, graphBuilder: mockBuilder}

	graph, svcErr := service.loadGraph(context.Background(), "flow-1", s.logger)

	s.Nil(graph)
	s.NotNil(svcErr)
	s.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
}

func (s *FlowExecServiceGraphTestSuite) TestLoadGraph_Success() {
	flowDef := minimalFlowDefinition("flow-1", common.FlowTypeAuthentication)
	mockProvider := NewFlowProviderInterfaceMock(s.T())
	mockProvider.EXPECT().GetFlow(mock.Anything, "flow-1").Return(flowDef, nil)

	mockBuilder := coremock.NewGraphBuilderInterfaceMock(s.T())
	mockBuilder.EXPECT().GetGraph(mock.Anything, flowDef).Return(s.testGraph, nil)

	service := &flowExecService{flowProvider: mockProvider, graphBuilder: mockBuilder}

	graph, svcErr := service.loadGraph(context.Background(), "flow-1", s.logger)

	s.Nil(svcErr)
	s.Equal(s.testGraph, graph)
}

func (s *FlowExecServiceGraphTestSuite) TestGetFlowGraph_EmptyAppIDForAuthentication() {
	service := &flowExecService{}

	flowID, svcErr := service.getFlowGraph(
		context.Background(), "", common.FlowTypeAuthentication, s.logger)

	s.Empty(flowID)
	s.NotNil(svcErr)
	s.Equal(ErrorInvalidAppID.Code, svcErr.Code)
}

func (s *FlowExecServiceGraphTestSuite) TestGetFlowGraph_InboundClientNotFound() {
	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(s.T())
	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, testAppIDGraph).
		Return(nil, inboundclient.ErrInboundClientNotFound)

	service := &flowExecService{inboundClientService: mockInboundClient}

	flowID, svcErr := service.getFlowGraph(
		context.Background(), testAppIDGraph, common.FlowTypeAuthentication, s.logger)

	s.Empty(flowID)
	s.NotNil(svcErr)
	s.Equal(ErrorInvalidAppID.Code, svcErr.Code)
}

func (s *FlowExecServiceGraphTestSuite) TestGetFlowGraph_RegistrationFlowDisabled() {
	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(s.T())
	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, testAppIDGraph).
		Return(&inboundmodel.InboundClient{ID: testAppIDGraph, IsRegistrationFlowEnabled: false}, nil)

	service := &flowExecService{inboundClientService: mockInboundClient}

	flowID, svcErr := service.getFlowGraph(
		context.Background(), testAppIDGraph, common.FlowTypeRegistration, s.logger)

	s.Empty(flowID)
	s.NotNil(svcErr)
	s.Equal(ErrorRegistrationFlowDisabled.Code, svcErr.Code)
}

func (s *FlowExecServiceGraphTestSuite) TestGetFlowGraph_RecoveryFlowDisabled() {
	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(s.T())
	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, testAppIDGraph).
		Return(&inboundmodel.InboundClient{ID: testAppIDGraph, IsRecoveryFlowEnabled: false}, nil)

	service := &flowExecService{inboundClientService: mockInboundClient}

	flowID, svcErr := service.getFlowGraph(
		context.Background(), testAppIDGraph, common.FlowTypeRecovery, s.logger)

	s.Empty(flowID)
	s.NotNil(svcErr)
	s.Equal(ErrorRecoveryFlowDisabled.Code, svcErr.Code)
}

func (s *FlowExecServiceGraphTestSuite) TestGetFlowGraph_ReturnsAuthenticationFlowID() {
	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(s.T())
	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, testAppIDGraph).
		Return(&inboundmodel.InboundClient{ID: testAppIDGraph, AuthFlowID: "auth-flow-1"}, nil)

	service := &flowExecService{inboundClientService: mockInboundClient}

	flowID, svcErr := service.getFlowGraph(
		context.Background(), testAppIDGraph, common.FlowTypeAuthentication, s.logger)

	s.Nil(svcErr)
	s.Equal("auth-flow-1", flowID)
}

func (s *FlowExecServiceGraphTestSuite) TestGetFlowGraph_ReturnsRegistrationFlowID() {
	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(s.T())
	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, testAppIDGraph).
		Return(&inboundmodel.InboundClient{
			ID: testAppIDGraph, IsRegistrationFlowEnabled: true, RegistrationFlowID: "reg-flow-1",
		}, nil)

	service := &flowExecService{inboundClientService: mockInboundClient}

	flowID, svcErr := service.getFlowGraph(
		context.Background(), testAppIDGraph, common.FlowTypeRegistration, s.logger)

	s.Nil(svcErr)
	s.Equal("reg-flow-1", flowID)
}

func (s *FlowExecServiceGraphTestSuite) TestGetFlowGraph_ReturnsRecoveryFlowID() {
	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(s.T())
	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, testAppIDGraph).
		Return(&inboundmodel.InboundClient{
			ID: testAppIDGraph, IsRecoveryFlowEnabled: true, RecoveryFlowID: "recovery-flow-1",
		}, nil)

	service := &flowExecService{inboundClientService: mockInboundClient}

	flowID, svcErr := service.getFlowGraph(
		context.Background(), testAppIDGraph, common.FlowTypeRecovery, s.logger)

	s.Nil(svcErr)
	s.Equal("recovery-flow-1", flowID)
}

func (s *FlowExecServiceGraphTestSuite) TestGetFlowGraph_UserOnboardingResolvesSystemFlowByHandle() {
	s.setupUserOnboardingConfig()

	mockProvider := NewFlowProviderInterfaceMock(s.T())
	mockProvider.EXPECT().GetFlowByHandle(
		mock.Anything, testUserOnboardingFlowHandle, common.FlowTypeUserOnboarding).
		Return(&common.CompleteFlowDefinition{ID: "onboarding-flow-id"}, nil)

	service := &flowExecService{flowProvider: mockProvider}

	flowID, svcErr := service.getFlowGraph(
		context.Background(), "", common.FlowTypeUserOnboarding, s.logger)

	s.Nil(svcErr)
	s.Equal("onboarding-flow-id", flowID)
}

func (s *FlowExecServiceGraphTestSuite) TestGetFlowGraph_InboundClientServerError() {
	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(s.T())
	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, testAppIDGraph).
		Return(nil, errors.New("database unavailable"))

	service := &flowExecService{inboundClientService: mockInboundClient}

	flowID, svcErr := service.getFlowGraph(
		context.Background(), testAppIDGraph, common.FlowTypeAuthentication, s.logger)

	s.Empty(flowID)
	s.NotNil(svcErr)
	s.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
}

func (s *FlowExecServiceGraphTestSuite) TestGetFlowGraph_NilInboundClient() {
	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(s.T())
	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, testAppIDGraph).
		Return(nil, nil)

	service := &flowExecService{inboundClientService: mockInboundClient}

	flowID, svcErr := service.getFlowGraph(
		context.Background(), testAppIDGraph, common.FlowTypeAuthentication, s.logger)

	s.Empty(flowID)
	s.NotNil(svcErr)
	s.Equal(ErrorInvalidAppID.Code, svcErr.Code)
}

func (s *FlowExecServiceGraphTestSuite) TestGetFlowGraph_EmptyAuthenticationFlowID() {
	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(s.T())
	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, testAppIDGraph).
		Return(&inboundmodel.InboundClient{ID: testAppIDGraph, AuthFlowID: ""}, nil)

	service := &flowExecService{inboundClientService: mockInboundClient}

	flowID, svcErr := service.getFlowGraph(
		context.Background(), testAppIDGraph, common.FlowTypeAuthentication, s.logger)

	s.Empty(flowID)
	s.NotNil(svcErr)
	s.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
}

func (s *FlowExecServiceGraphTestSuite) TestGetFlowGraph_RegistrationFlowNotConfigured() {
	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(s.T())
	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, testAppIDGraph).
		Return(&inboundmodel.InboundClient{
			ID: testAppIDGraph, IsRegistrationFlowEnabled: true, RegistrationFlowID: "",
		}, nil)

	service := &flowExecService{inboundClientService: mockInboundClient}

	flowID, svcErr := service.getFlowGraph(
		context.Background(), testAppIDGraph, common.FlowTypeRegistration, s.logger)

	s.Empty(flowID)
	s.NotNil(svcErr)
	s.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
}

func (s *FlowExecServiceGraphTestSuite) TestGetFlowGraph_RecoveryFlowNotConfigured() {
	mockInboundClient := inboundclientmock.NewInboundClientServiceInterfaceMock(s.T())
	mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, testAppIDGraph).
		Return(&inboundmodel.InboundClient{
			ID: testAppIDGraph, IsRecoveryFlowEnabled: true, RecoveryFlowID: "",
		}, nil)

	service := &flowExecService{inboundClientService: mockInboundClient}

	flowID, svcErr := service.getFlowGraph(
		context.Background(), testAppIDGraph, common.FlowTypeRecovery, s.logger)

	s.Empty(flowID)
	s.NotNil(svcErr)
	s.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
}

func (s *FlowExecServiceGraphTestSuite) setupUserOnboardingConfig() {
	testConfig := &config.Config{}
	testConfig.Flow.UserOnboardingFlowHandle = testUserOnboardingFlowHandle
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)
	s.T().Cleanup(config.ResetServerRuntime)
}

func (s *FlowExecServiceGraphTestSuite) TestGetFlowGraph_UserOnboardingGetFlowByHandleError() {
	s.setupUserOnboardingConfig()

	mockProvider := NewFlowProviderInterfaceMock(s.T())
	mockProvider.EXPECT().GetFlowByHandle(
		mock.Anything, testUserOnboardingFlowHandle, common.FlowTypeUserOnboarding).
		Return(nil, &serviceerror.InternalServerError)

	service := &flowExecService{flowProvider: mockProvider}

	flowID, svcErr := service.getFlowGraph(
		context.Background(), "", common.FlowTypeUserOnboarding, s.logger)

	s.Empty(flowID)
	s.NotNil(svcErr)
	s.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
}

func (s *FlowExecServiceGraphTestSuite) TestGetFlowGraph_UserOnboardingEmptyFlowDefinition() {
	s.setupUserOnboardingConfig()

	mockProvider := NewFlowProviderInterfaceMock(s.T())
	mockProvider.EXPECT().GetFlowByHandle(
		mock.Anything, testUserOnboardingFlowHandle, common.FlowTypeUserOnboarding).
		Return(nil, nil)

	service := &flowExecService{flowProvider: mockProvider}

	flowID, svcErr := service.getFlowGraph(
		context.Background(), "", common.FlowTypeUserOnboarding, s.logger)

	s.Empty(flowID)
	s.NotNil(svcErr)
	s.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
}
