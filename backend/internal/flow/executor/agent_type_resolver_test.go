// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
)

const agentTypeExecutionID = "flow-agent-type-1"

// The resolution is shared with the user resolver, whose suite covers the filtering and the
// none/one/many outcomes. These tests cover what is category dependent: the runtime key written,
// the category queried, and the wording of the errors raised.
type AgentTypeResolverTestSuite struct {
	suite.Suite
	mockFlowFactory       *coremock.FlowFactoryInterfaceMock
	mockEntityTypeService *entitytypemock.EntityTypeServiceInterfaceMock
	mockOUService         *oumock.OrganizationUnitServiceInterfaceMock
	executor              *agentTypeResolver
}

func TestAgentTypeResolverSuite(t *testing.T) {
	suite.Run(t, new(AgentTypeResolverTestSuite))
}

func (suite *AgentTypeResolverTestSuite) SetupTest() {
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())
	suite.mockEntityTypeService = entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	suite.mockOUService = oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())

	mockExec := coremock.NewExecutorInterfaceMock(suite.T())
	mockExec.On("GetName").Return(ExecutorNameAgentTypeResolver).Maybe()
	mockExec.On("GetType").Return(providers.ExecutorTypeUtility).Maybe()
	mockExec.On("GetDefaultInputs").Return([]providers.Input{
		{Ref: "agenttype_input", Identifier: agentTypeKey, Type: providers.InputTypeSelect, Required: true},
	}).Maybe()
	mockExec.On("GetPrerequisites").Return([]providers.Input{}).Maybe()

	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameAgentTypeResolver, providers.ExecutorTypeUtility,
		mock.Anything, mock.Anything, mock.Anything).Return(mockExec)

	suite.executor = newAgentTypeResolver(suite.mockFlowFactory, suite.mockEntityTypeService, suite.mockOUService)
}

func agentTypeNodeContext() *providers.NodeContext {
	return &providers.NodeContext{
		ExecutionID: agentTypeExecutionID,
		FlowType:    providers.FlowTypeAdministration,
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{},
	}
}

func (suite *AgentTypeResolverTestSuite) listReturns(types ...entitytype.EntityTypeListItem) {
	suite.mockEntityTypeService.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryAgent,
		entityTypeCandidateLimit, 0, false).
		Return(&entitytype.EntityTypeListResponse{Types: types}, nil).Once()
}

// One candidate is no choice, so it is taken without a prompt.
func (suite *AgentTypeResolverTestSuite) TestSingleTypeResolvesWithoutPrompting() {
	suite.listReturns(entitytype.EntityTypeListItem{Name: "default", OUID: "ou-agents"})

	resp, err := suite.executor.Execute(agentTypeNodeContext())

	suite.NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
	suite.Equal("default", resp.RuntimeData[agentTypeKey], "the agent runtime key, not the user one")
	suite.Empty(resp.RuntimeData[userTypeKey])
	suite.Equal("ou-agents", resp.RuntimeData[defaultOUIDKey])
	suite.Empty(resp.Inputs)
}

// Several candidates are offered rather than narrowed, so the count is not assumed anywhere.
func (suite *AgentTypeResolverTestSuite) TestMultipleTypesArePrompted() {
	suite.listReturns(
		entitytype.EntityTypeListItem{Name: "default", OUID: "ou-agents"},
		entitytype.EntityTypeListItem{Name: "worker", OUID: "ou-agents"},
	)

	resp, err := suite.executor.Execute(agentTypeNodeContext())

	suite.NoError(err)
	suite.Equal(providers.ExecUserInputRequired, resp.Status)
	suite.Require().Len(resp.Inputs, 1)
	suite.Equal(agentTypeKey, resp.Inputs[0].Identifier)
	suite.ElementsMatch([]string{"default", "worker"}, resp.Inputs[0].Options)
	suite.Equal(resp.Inputs, resp.ForwardedData[common.ForwardedDataKeyInputs])
}

// The node property narrows the candidates before the count decides the outcome.
func (suite *AgentTypeResolverTestSuite) TestAllowedAgentTypesNarrowsTheChoice() {
	ctx := agentTypeNodeContext()
	ctx.NodeProperties = map[string]interface{}{
		propertyKeyAllowedAgentTypes: []interface{}{"default"},
	}
	suite.listReturns(
		entitytype.EntityTypeListItem{Name: "default", OUID: "ou-agents"},
		entitytype.EntityTypeListItem{Name: "worker", OUID: "ou-other"},
	)

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status, "narrowing to one removes the prompt")
	suite.Equal("default", resp.RuntimeData[agentTypeKey])
	suite.Equal("ou-agents", resp.RuntimeData[defaultOUIDKey])
}

// The errors are shared across categories, so the category has to reach the rendered message.
func (suite *AgentTypeResolverTestSuite) TestErrorsNameTheAgentCategory() {
	suite.mockEntityTypeService.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryAgent,
		entityTypeCandidateLimit, 0, false).
		Return(&entitytype.EntityTypeListResponse{Types: []entitytype.EntityTypeListItem{}}, nil).Once()

	resp, err := suite.executor.Execute(agentTypeNodeContext())

	suite.NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Require().NotNil(resp.Error)
	suite.Equal(ErrNoUserTypesAvailable.Code, resp.Error.Code, "the code is shared across categories")
	suite.Equal(string(entitytype.TypeCategoryAgent), resp.Error.Error.Params["entity"],
		"the message renders as agent types, not user types")
}

// The supported flow types are declared, so the validator rejects a node placed elsewhere.
func (suite *AgentTypeResolverTestSuite) TestSupportedFlowTypes() {
	mockFactory := coremock.NewFlowFactoryInterfaceMock(suite.T())
	mockExec := coremock.NewExecutorInterfaceMock(suite.T())
	var meta *providers.ExecutorMeta

	mockFactory.On("CreateExecutor", ExecutorNameAgentTypeResolver, providers.ExecutorTypeUtility,
		mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			meta = args.Get(4).(*providers.ExecutorMeta)
		}).Return(mockExec)

	newAgentTypeResolver(mockFactory, suite.mockEntityTypeService, suite.mockOUService)

	suite.Require().NotNil(meta)
	suite.Equal([]providers.FlowType{providers.FlowTypeAdministration}, meta.SupportedFlowTypes)
	suite.Require().Len(meta.SupportedProperties, 1)
	suite.Equal(propertyKeyAllowedAgentTypes, meta.SupportedProperties[0].Property)
}
