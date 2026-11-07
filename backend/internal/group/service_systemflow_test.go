// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package group

import (
	"context"
	"errors"

	"github.com/stretchr/testify/mock"

	flowcommon "github.com/thunder-id/thunderid/internal/flow/common"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// deletableGroup returns a service around a group that may be deleted, with flows as its provider and
// cascadeErr from the cascade, so a test sees whether the deletion went past the pre flow.
func (suite *GroupServiceTestSuite) deletableGroup(flows systemFlowProvider, cascadeErr error) (
	*groupService, *groupStoreInterfaceMock) {
	storeMock := newGroupStoreInterfaceMock(suite.T())
	storeMock.On("IsGroupDeclarative", mock.Anything, "grp-001").Return(false, nil).Once()
	storeMock.On("GetGroup", mock.Anything, "grp-001").
		Return(GroupDAO{ID: "grp-001", OUID: testOUID1}, nil).Once()
	service := &groupService{
		authzService:       newAllowAllAuthz(suite.T()),
		groupStore:         storeMock,
		transactioner:      &stubTransactioner{},
		dependencyRegistry: noopDepRegistry{cascadeErr: cascadeErr},
		systemFlows:        flows,
		bus:                discardingBus{},
	}
	return service, storeMock
}

// isHandle matches a run of the named system flow.
func isHandle(handle string) interface{} {
	return mock.MatchedBy(func(c *flowcommon.SystemFlowContext) bool { return c.Handle == handle })
}

// A group deletion runs the pre flow with the group as input before the cascade, and the revocation post
// flow, with what the pre flow shared, after the cascade and before the group is deleted.
func (suite *GroupServiceTestSuite) TestDeleteGroup_RunsThePostFlowBeforeTheGroupDelete() {
	var order []string
	inputs := map[string]string{systemFlowInputGroup: "grp-001"}
	shared := map[string]string{"revocationPlan": "plan"}
	flows := newSystemFlowProviderMock(suite.T())
	flows.EXPECT().ExecuteSystemFlow(mock.Anything,
		&flowcommon.SystemFlowContext{Handle: systemFlowGroupDeletePre, Inputs: inputs}).
		Run(func(context.Context, *flowcommon.SystemFlowContext) { order = append(order, "pre") }).
		Return(&flowcommon.SystemFlowResult{RuntimeData: shared}, nil).Once()
	flows.EXPECT().ExecuteSystemFlow(mock.Anything,
		&flowcommon.SystemFlowContext{Handle: systemFlowRevocationPost, Inputs: inputs, RuntimeData: shared}).
		Run(func(context.Context, *flowcommon.SystemFlowContext) { order = append(order, "post") }).
		Return(&flowcommon.SystemFlowResult{}, nil).Once()
	service, storeMock := suite.deletableGroup(flows, nil)
	storeMock.On("DeleteGroup", mock.Anything, "grp-001").
		Run(func(mock.Arguments) { order = append(order, "group") }).Return(nil).Once()

	err := service.DeleteGroup(context.Background(), "grp-001")

	suite.Nil(err)
	suite.Equal([]string{"pre", "post", "group"}, order)
}

// The pre flow runs before the cascade, which removes the group's role assignments.
func (suite *GroupServiceTestSuite) TestDeleteGroup_RunsThePreFlowBeforeTheCascade() {
	flows := newSystemFlowProviderMock(suite.T())
	flows.EXPECT().ExecuteSystemFlow(mock.Anything, isHandle(systemFlowGroupDeletePre)).
		Return(&flowcommon.SystemFlowResult{}, nil).Once()
	service, storeMock := suite.deletableGroup(flows, errors.New("cascade reached"))

	err := service.DeleteGroup(context.Background(), "grp-001")

	suite.Equal(&tidcommon.InternalServerError, err, "the cascade must run after the pre flow")
	storeMock.AssertNotCalled(suite.T(), "DeleteGroup", mock.Anything, mock.Anything)
}

// A failed post flow is reported and leaves the group in place, so the delete can be retried.
func (suite *GroupServiceTestSuite) TestDeleteGroup_FailedPostFlowKeepsTheGroup() {
	flows := newSystemFlowProviderMock(suite.T())
	flows.EXPECT().ExecuteSystemFlow(mock.Anything, isHandle(systemFlowGroupDeletePre)).
		Return(&flowcommon.SystemFlowResult{}, nil).Once()
	flows.EXPECT().ExecuteSystemFlow(mock.Anything, isHandle(systemFlowRevocationPost)).
		Return(nil, &tidcommon.InternalServerError).Once()
	service, storeMock := suite.deletableGroup(flows, nil)

	err := service.DeleteGroup(context.Background(), "grp-001")

	suite.Equal(&tidcommon.InternalServerError, err)
	storeMock.AssertNotCalled(suite.T(), "DeleteGroup", mock.Anything, mock.Anything)
}

// A refused pre flow leaves the group, its role assignments and its memberships as they were, and runs no
// post flow.
func (suite *GroupServiceTestSuite) TestDeleteGroup_RefusedFlowRemovesNothing() {
	refusal := &tidcommon.ServiceError{Code: "FEX-1", Type: tidcommon.ClientErrorType}
	flows := newSystemFlowProviderMock(suite.T())
	flows.EXPECT().ExecuteSystemFlow(mock.Anything, mock.Anything).Return(nil, refusal).Once()
	// A cascade error would surface as a server error, so the refusal coming back proves it never ran.
	service, storeMock := suite.deletableGroup(flows, errors.New("cascade reached"))

	err := service.DeleteGroup(context.Background(), "grp-001")

	suite.Same(refusal, err)
	storeMock.AssertNotCalled(suite.T(), "DeleteGroup", mock.Anything, mock.Anything)
}

// A group that is not there has nothing to revoke, so no flow runs.
func (suite *GroupServiceTestSuite) TestDeleteGroup_MissingGroupRunsNoFlow() {
	storeMock := newGroupStoreInterfaceMock(suite.T())
	storeMock.On("IsGroupDeclarative", mock.Anything, "gone").Return(false, nil).Once()
	storeMock.On("GetGroup", mock.Anything, "gone").Return(GroupDAO{}, ErrGroupNotFound).Once()
	service := &groupService{
		authzService:       newAllowAllAuthz(suite.T()),
		groupStore:         storeMock,
		transactioner:      &stubTransactioner{},
		dependencyRegistry: noopDepRegistry{},
		systemFlows:        newSystemFlowProviderMock(suite.T()),
		bus:                discardingBus{},
	}

	err := service.DeleteGroup(context.Background(), "gone")

	suite.Equal(&ErrorGroupNotFound, err)
}

// A service with no provider runs no system flows and still deletes the group.
func (suite *GroupServiceTestSuite) TestDeleteGroup_NoProviderRunsNoFlow() {
	service, storeMock := suite.deletableGroup(nil, nil)
	storeMock.On("DeleteGroup", mock.Anything, "grp-001").Return(nil).Once()

	err := service.DeleteGroup(context.Background(), "grp-001")

	suite.Nil(err)
}

// The server injects the flow engine once it exists; a service that cannot take it is reported.
func (suite *GroupServiceTestSuite) TestSetSystemFlowProvider() {
	service := &groupService{}
	provider := newSystemFlowProviderMock(suite.T())

	suite.NoError(SetSystemFlowProvider(service, provider))
	suite.Same(provider, service.systemFlows)
	suite.Error(SetSystemFlowProvider(NewGroupServiceInterfaceMock(suite.T()), provider))
}
