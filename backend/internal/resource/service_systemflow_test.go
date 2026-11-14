// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"

	"github.com/stretchr/testify/mock"

	flowcommon "github.com/thunder-id/thunderid/internal/flow/common"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// expectSystemFlows gives the service a mocked system flow runner and returns it.
func (suite *ResourceServiceTestSuite) expectSystemFlows() *systemFlowRunnerMock {
	flows := newSystemFlowRunnerMock(suite.T())
	suite.Require().NoError(SetSystemFlowRunner(suite.service, flows))
	return flows
}

// existingAction readies action-123 on rs-123 to be deleted.
func (suite *ResourceServiceTestSuite) existingAction() {
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything, "rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("IsActionExist", mock.Anything, "action-123", "rs-123", (*string)(nil)).Return(true, nil)
}

// isHandle matches a run of the named system flow.
func isHandle(handle string) interface{} {
	return mock.MatchedBy(func(c *flowcommon.SystemFlowContext) bool { return c.Handle == handle })
}

func (suite *ResourceServiceTestSuite) TestSetSystemFlowRunner() {
	flows := newSystemFlowRunnerMock(suite.T())

	suite.NoError(SetSystemFlowRunner(suite.service, flows))
	suite.Same(flows, suite.service.(*resourceService).systemFlows)
	suite.Error(SetSystemFlowRunner(NewResourceServiceInterfaceMock(suite.T()), flows))
}

// An action deletion runs the pre flow with the action as input before anything is removed, and the
// revocation post flow, with what the pre flow shared, once the action is gone.
func (suite *ResourceServiceTestSuite) TestDeleteAction_RunsThePreAndPostFlowsAroundTheWrite() {
	flows := suite.expectSystemFlows()
	suite.existingAction()
	var order []string
	inputs := map[string]string{systemFlowInputResourceServer: "rs-123", systemFlowInputAction: "action-123"}
	shared := map[string]string{"revocationPlan": "plan"}
	flows.EXPECT().ExecuteSystemFlow(mock.Anything,
		&flowcommon.SystemFlowContext{Handle: systemFlowActionDeletePre, Inputs: inputs}).
		Run(func(context.Context, *flowcommon.SystemFlowContext) { order = append(order, "pre") }).
		Return(&flowcommon.SystemFlowResult{RuntimeData: shared}, nil).Once()
	flows.EXPECT().ExecuteSystemFlow(mock.Anything,
		&flowcommon.SystemFlowContext{Handle: systemFlowRevocationPost, Inputs: inputs, RuntimeData: shared}).
		Run(func(context.Context, *flowcommon.SystemFlowContext) { order = append(order, "post") }).
		Return(&flowcommon.SystemFlowResult{}, nil).Once()
	suite.mockStore.On("DeleteAction", mock.Anything, "action-123", "rs-123", (*string)(nil)).Return(nil).
		Run(func(mock.Arguments) { order = append(order, "delete") })

	err := suite.service.DeleteAction(context.Background(), "rs-123", nil, "action-123")

	suite.Nil(err)
	suite.Equal([]string{"pre", "delete", "post"}, order)
}

// A post flow that fails after the action is deleted is reported, and the deletion stays committed.
func (suite *ResourceServiceTestSuite) TestDeleteAction_FailedPostFlowIsReported() {
	flows := suite.expectSystemFlows()
	suite.existingAction()
	flows.EXPECT().ExecuteSystemFlow(mock.Anything, isHandle(systemFlowActionDeletePre)).
		Return(&flowcommon.SystemFlowResult{}, nil).Once()
	flows.EXPECT().ExecuteSystemFlow(mock.Anything, isHandle(systemFlowRevocationPost)).
		Return(nil, &tidcommon.InternalServerError).Once()
	suite.mockStore.On("DeleteAction", mock.Anything, "action-123", "rs-123", (*string)(nil)).Return(nil)

	err := suite.service.DeleteAction(context.Background(), "rs-123", nil, "action-123")

	suite.Equal(&tidcommon.InternalServerError, err)
	suite.mockStore.AssertCalled(suite.T(), "DeleteAction", mock.Anything, "action-123", "rs-123", (*string)(nil))
}

// An action under a resource names the resource to the flows as well.
func (suite *ResourceServiceTestSuite) TestDeleteAction_UnderAResourceNamesTheResource() {
	flows := suite.expectSystemFlows()
	resourceID := testResourceID
	inputs := map[string]string{
		systemFlowInputResourceServer: "rs-123", systemFlowInputResource: resourceID,
		systemFlowInputAction: "action-123",
	}
	flows.EXPECT().ExecuteSystemFlow(mock.Anything,
		&flowcommon.SystemFlowContext{Handle: systemFlowActionDeletePre, Inputs: inputs}).
		Return(&flowcommon.SystemFlowResult{}, nil).Once()
	flows.EXPECT().ExecuteSystemFlow(mock.Anything,
		&flowcommon.SystemFlowContext{Handle: systemFlowRevocationPost, Inputs: inputs}).
		Return(&flowcommon.SystemFlowResult{}, nil).Once()
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything, "rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("GetResource", mock.Anything, resourceID, "rs-123").Return(providers.Resource{}, nil)
	suite.mockStore.On("IsActionExist", mock.Anything, "action-123", "rs-123", &resourceID).Return(true, nil)
	suite.mockStore.On("DeleteAction", mock.Anything, "action-123", "rs-123", &resourceID).Return(nil)

	err := suite.service.DeleteAction(context.Background(), "rs-123", &resourceID, "action-123")

	suite.Nil(err)
}

// A refused pre flow leaves the action as it was.
func (suite *ResourceServiceTestSuite) TestDeleteAction_RefusedFlowRemovesNothing() {
	flows := suite.expectSystemFlows()
	suite.existingAction()
	flows.EXPECT().ExecuteSystemFlow(mock.Anything, isHandle(systemFlowActionDeletePre)).
		Return(nil, &tidcommon.InternalServerError).Once()

	err := suite.service.DeleteAction(context.Background(), "rs-123", nil, "action-123")

	suite.Equal(&tidcommon.InternalServerError, err)
	suite.mockStore.AssertNotCalled(suite.T(), "DeleteAction", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything)
}

// An action that is already gone runs no flow, so a retry does not revoke again.
func (suite *ResourceServiceTestSuite) TestDeleteAction_MissingActionRunsNoFlow() {
	suite.expectSystemFlows()
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything, "rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("IsActionExist", mock.Anything, "action-123", "rs-123", (*string)(nil)).Return(false, nil)

	err := suite.service.DeleteAction(context.Background(), "rs-123", nil, "action-123")

	suite.Nil(err)
}
