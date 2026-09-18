// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
)

const (
	testOwnerUserID  = "user-owner-1"
	testOtherUserID  = "user-owner-2"
	ownerExecutionID = "flow-owner-1"
)

type OwnerResolverTestSuite struct {
	suite.Suite
	mockFlowFactory    *coremock.FlowFactoryInterfaceMock
	mockEntityProvider *entityprovidermock.EntityProviderInterfaceMock
	executor           *ownerResolver
}

func TestOwnerResolverSuite(t *testing.T) {
	suite.Run(t, new(OwnerResolverTestSuite))
}

func (suite *OwnerResolverTestSuite) SetupTest() {
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())
	suite.mockEntityProvider = entityprovidermock.NewEntityProviderInterfaceMock(suite.T())

	mockExec := coremock.NewExecutorInterfaceMock(suite.T())
	mockExec.On("GetName").Return(ExecutorNameOwnerResolver).Maybe()
	mockExec.On("GetType").Return(providers.ExecutorTypeUtility).Maybe()
	mockExec.On("GetDefaultInputs").Return([]providers.Input{
		{Ref: "owner_input", Identifier: ownerKey, Type: providers.InputTypeUserSelect, Required: false},
	}).Maybe()
	mockExec.On("GetPrerequisites").Return([]providers.Input{}).Maybe()

	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameOwnerResolver, providers.ExecutorTypeUtility,
		mock.Anything, mock.Anything, mock.Anything).Return(mockExec)

	suite.executor = newOwnerResolver(suite.mockFlowFactory, suite.mockEntityProvider)
}

// An owner picker is offered while an entity is being onboarded, so the executor must be usable in a
// registration flow as well as an administration one.
func (suite *OwnerResolverTestSuite) TestSupportedFlowTypes() {
	mockFactory := coremock.NewFlowFactoryInterfaceMock(suite.T())
	mockExec := coremock.NewExecutorInterfaceMock(suite.T())

	var meta *providers.ExecutorMeta
	mockFactory.On("CreateExecutor", ExecutorNameOwnerResolver, providers.ExecutorTypeUtility,
		mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			meta = args.Get(4).(*providers.ExecutorMeta)
		}).Return(mockExec)

	newOwnerResolver(mockFactory, suite.mockEntityProvider)

	suite.Require().NotNil(meta)
	suite.ElementsMatch([]providers.FlowType{
		providers.FlowTypeAdministration,
		providers.FlowTypeRegistration,
	}, meta.SupportedFlowTypes)
}

func ownerNodeContext() *providers.NodeContext {
	return &providers.NodeContext{
		ExecutionID: ownerExecutionID,
		FlowType:    providers.FlowTypeAdministration,
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{},
	}
}

// The candidates are the client's to fetch, the way OU_SELECT works, so the prompt carries the
// input alone and the executor never lists users.
func (suite *OwnerResolverTestSuite) TestPromptsForAUserSelection() {
	resp, err := suite.executor.Execute(ownerNodeContext())

	suite.NoError(err)
	suite.Equal(providers.ExecUserInputRequired, resp.Status)
	suite.Require().Len(resp.Inputs, 1)
	suite.Equal(ownerKey, resp.Inputs[0].Identifier)
	suite.Equal(providers.InputTypeUserSelect, resp.Inputs[0].Type)
	suite.Empty(resp.Inputs[0].Options, "the client sources the candidates, so none are sent")
	suite.Equal(resp.Inputs, resp.ForwardedData[common.ForwardedDataKeyInputs])
	suite.mockEntityProvider.AssertNotCalled(suite.T(), "GetEntityList")
}

// The input is optional so an administrator can accept the default. The engine records presented
// optional inputs, and the executor must treat that as "asked and declined" rather than asking
// again, which would loop the flow.
func (suite *OwnerResolverTestSuite) TestSkippedPromptCompletesWithoutOwner() {
	ctx := ownerNodeContext()
	ctx.RuntimeData[common.RuntimeKeyPresentedOptionalInputs] = ownerKey

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
	suite.Empty(resp.RuntimeData[ownerKey], "no owner leaves the provider to use the caller")
	suite.Empty(resp.Inputs, "the choice must not be offered a second time")
	suite.mockEntityProvider.AssertNotCalled(suite.T(), "GetEntity", mock.Anything)
}

// A selected owner is recorded for the provisioning node to send.
func (suite *OwnerResolverTestSuite) TestSelectedOwnerIsRecorded() {
	ctx := ownerNodeContext()
	ctx.UserInputs[ownerKey] = testOwnerUserID
	suite.mockEntityProvider.On("GetEntity", testOwnerUserID).
		Return(&providers.Entity{Category: providers.EntityCategoryUser, ID: testOwnerUserID}, nil).Once()

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
	suite.Equal(testOwnerUserID, resp.RuntimeData[ownerKey])
}

// An owner that does not exist is recoverable by choosing another, so the choice is offered again
// rather than failing the flow.
func (suite *OwnerResolverTestSuite) TestUnknownOwnerIsRePrompted() {
	ctx := ownerNodeContext()
	ctx.UserInputs[ownerKey] = "does-not-exist"
	suite.mockEntityProvider.On("GetEntity", "does-not-exist").
		Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", "")).Once()

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecUserInputRequired, resp.Status)
	suite.Require().NotNil(resp.Error)
	suite.Equal(ErrOwnerNotFound.Code, resp.Error.Code)
	suite.Empty(resp.RuntimeData[ownerKey])
}

// An entity that is not a user must not become an owner. Entity lookups are not scoped by
// category, so without the check an agent identifier would be accepted, and the client-side list
// is no guarantee of what a caller submits.
func (suite *OwnerResolverTestSuite) TestNonUserEntityIsRejected() {
	ctx := ownerNodeContext()
	ctx.UserInputs[ownerKey] = testOtherUserID
	suite.mockEntityProvider.On("GetEntity", testOtherUserID).
		Return(&providers.Entity{Category: providers.EntityCategoryAgent, ID: testOtherUserID}, nil).Once()

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecUserInputRequired, resp.Status)
	suite.Require().NotNil(resp.Error)
	suite.Equal(ErrOwnerNotFound.Code, resp.Error.Code,
		"an agent identifier is reported the same way a missing one is")
	suite.Empty(resp.RuntimeData[ownerKey])
}

// A lookup outage is not something the administrator can fix by choosing differently, so it
// surfaces as a server error instead of a re-prompt.
func (suite *OwnerResolverTestSuite) TestEntityLookupFailurePropagates() {
	ctx := ownerNodeContext()
	ctx.UserInputs[ownerKey] = testOwnerUserID
	suite.mockEntityProvider.On("GetEntity", testOwnerUserID).
		Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeSystemError, "", "")).Once()

	resp, err := suite.executor.Execute(ctx)

	suite.Error(err)
	suite.Nil(resp)
}
