// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package graphbuilder

import (
	"context"
	"testing"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/executor"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/authn/githubmock"
	"github.com/thunder-id/thunderid/tests/mocks/authn/googlemock"
	"github.com/thunder-id/thunderid/tests/mocks/authn/oauthmock"
	"github.com/thunder-id/thunderid/tests/mocks/authn/oidcmock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
)

type GraphBuilderSubsetRegistryTestSuite struct {
	suite.Suite
}

func TestGraphBuilderSubsetRegistrySuite(t *testing.T) {
	suite.Run(t, new(GraphBuilderSubsetRegistryTestSuite))
}

func (s *GraphBuilderSubsetRegistryTestSuite) SetupSuite() {
	_ = config.InitializeServerRuntime("test", &config.Config{
		Server: engineconfig.ServerConfig{Identifier: "test-deployment"},
	})
}

func (s *GraphBuilderSubsetRegistryTestSuite) subsetExecutorRegistry() executor.ExecutorRegistryInterface {
	mockFactory := coremock.NewFlowFactoryInterfaceMock(s.T())
	mockBase := coremock.NewExecutorInterfaceMock(s.T())
	mockBase.On("GetName").Return("").Maybe()
	mockBase.On("GetType").Return(providers.ExecutorTypeUtility).Maybe()
	mockFactory.On("CreateExecutor", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(mockBase).Maybe()

	reg, err := executor.Initialize(executor.ExecutorDependencies{
		FlowFactory:    mockFactory,
		EntityProvider: entityprovidermock.NewEntityProviderInterfaceMock(s.T()),
		OAuthSvc:       oauthmock.NewOAuthAuthnServiceInterfaceMock(s.T()),
		OIDCSvc:        oidcmock.NewOIDCAuthnServiceInterfaceMock(s.T()),
		GithubSvc:      githubmock.NewGithubOAuthAuthnServiceInterfaceMock(s.T()),
		GoogleSvc:      googlemock.NewGoogleOIDCAuthnServiceInterfaceMock(s.T()),
	}, engineconfig.FlowConfig{
		Executors: []string{executor.ExecutorNameInviteExecutor},
	})
	require.NoError(s.T(), err)
	return reg
}

func (s *GraphBuilderSubsetRegistryTestSuite) TestBuildGraph_SubsetRegistryRejectsUnregisteredExecutor() {
	execRegistry := s.subsetExecutorRegistry()
	require.False(s.T(), execRegistry.IsRegistered(executor.ExecutorNameCredentialsAuth))

	mockFlowFactory := coremock.NewFlowFactoryInterfaceMock(s.T())
	builder := &graphBuilder{
		flowFactory:      mockFlowFactory,
		executorRegistry: execRegistry,
		logger:           log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowGraphBuilder")),
	}

	flow := &providers.CompleteFlowDefinition{
		ID:       "flow-1",
		Handle:   "test-handle",
		Name:     "Test Flow",
		FlowType: providers.FlowTypeAuthentication,
		Nodes: []providers.NodeDefinition{
			{
				ID:       "task",
				Type:     "TASK_EXECUTION",
				Executor: &providers.ExecutorDefinition{Name: executor.ExecutorNameCredentialsAuth},
			},
		},
	}

	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockTaskNode := coremock.NewExecutorBackedNodeInterfaceMock(s.T())

	mockFlowFactory.EXPECT().CreateGraph("flow-1", providers.FlowTypeAuthentication, 0).Return(mockGraph)
	mockFlowFactory.EXPECT().CreateNode(
		"task", "TASK_EXECUTION", map[string]interface{}(nil), false, true).Return(mockTaskNode, nil)
	mockTaskNode.EXPECT().SetInputs([]providers.Input{})
	mockTaskNode.EXPECT().SetIdentifierInputs([]providers.Input{})

	graph, err := builder.buildGraph(context.Background(), flow)

	s.Nil(graph)
	s.Require().Error(err)
	s.Contains(err.Error(), "executor with name "+executor.ExecutorNameCredentialsAuth+" not registered")
}
