// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package group

import (
	"context"
	"errors"

	"github.com/stretchr/testify/mock"

	flowcommon "github.com/thunder-id/thunderid/internal/flow/common"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
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
	suite.Error(SetSystemFlowProvider(nil, provider))
}

// memberRemoval returns a fixture around grp-001 with the given members, usr-001 alone when none are
// given, and the order in which a removal writes. Every user it is asked about exists.
func (suite *GroupServiceTestSuite) memberRemoval(members ...Member) (*groupChangeFixture, *[]string) {
	f := newGroupChangeFixture(suite.T())
	order := []string{}
	f.mayUpdateGroup()
	if len(members) == 0 {
		members = []Member{{ID: "usr-001", Type: memberTypeEntity}}
	}
	for _, member := range members {
		f.store.On("GetGroupMember", mock.Anything, "grp-001", member.ID).Return(&member, nil).Maybe()
	}
	f.store.On("GetGroupMember", mock.Anything, "grp-001", mock.Anything).Return((*Member)(nil), nil).Maybe()
	f.entities.On("GetEntitiesByIDs", mock.Anything, mock.Anything).Return(
		func(_ context.Context, ids []string) []providers.Entity {
			found := make([]providers.Entity, 0, len(ids))
			for _, id := range ids {
				found = append(found, providers.Entity{ID: id, Category: providers.EntityCategoryUser})
			}
			return found
		}, nil).Maybe()
	f.store.On("RemoveGroupMembers", mock.Anything, "grp-001", mock.Anything).
		Run(func(mock.Arguments) { order = append(order, "write") }).Return(nil).Maybe()
	return f, &order
}

// recordFlows answers every run with err, or else shares the run's handle as its plan, and records each
// run. When order is given, each run appends its handle to it.
func (suite *GroupServiceTestSuite) recordFlows(err *tidcommon.ServiceError, order *[]string) (
	*systemFlowProviderMock, *[]*flowcommon.SystemFlowContext) {
	runs := &[]*flowcommon.SystemFlowContext{}
	flows := newSystemFlowProviderMock(suite.T())
	flows.EXPECT().ExecuteSystemFlow(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, sysCtx *flowcommon.SystemFlowContext) (*flowcommon.SystemFlowResult,
			*tidcommon.ServiceError) {
			*runs = append(*runs, sysCtx)
			if order != nil {
				*order = append(*order, sysCtx.Handle)
			}
			if err != nil {
				return nil, err
			}
			return &flowcommon.SystemFlowResult{RuntimeData: map[string]string{"plan": sysCtx.Handle}}, nil
		}).Maybe()
	return flows, runs
}

// Removing a member runs the pre flow with the group and the member before the write, and the post flow,
// with what the pre flow shared, after.
func (suite *GroupServiceTestSuite) TestRemoveGroupMembers_RunsThePreAndPostFlowsAroundTheWrite() {
	f, order := suite.memberRemoval()
	flows, runs := suite.recordFlows(nil, order)
	f.service.systemFlows = flows

	_, svcErr := f.service.RemoveGroupMembers(context.Background(), "grp-001",
		[]Member{{ID: "usr-001", Type: MemberTypeUser}})

	suite.Require().Nil(svcErr)
	suite.Equal([]string{systemFlowGroupMembershipRemovalPre, "write", systemFlowRevocationPost}, *order)
	inputs := map[string]string{systemFlowInputGroup: "grp-001", systemFlowInputMember: "usr-001"}
	suite.Equal([]*flowcommon.SystemFlowContext{
		{Handle: systemFlowGroupMembershipRemovalPre, Inputs: inputs},
		{Handle: systemFlowRevocationPost, Inputs: inputs,
			RuntimeData: map[string]string{"plan": systemFlowGroupMembershipRemovalPre}},
	}, *runs)
}

// Removing several members looks each one up on its own and never lists the membership, which the
// composite store truncates at its record limit.
func (suite *GroupServiceTestSuite) TestRemoveGroupMembers_LooksUpEachMemberWithoutListing() {
	f, _ := suite.memberRemoval(Member{ID: "usr-001", Type: memberTypeEntity},
		Member{ID: "usr-002", Type: memberTypeEntity}, Member{ID: "usr-003", Type: memberTypeEntity})
	flows, runs := suite.recordFlows(nil, nil)
	f.service.systemFlows = flows

	_, svcErr := f.service.RemoveGroupMembers(context.Background(), "grp-001", []Member{
		{ID: "usr-001", Type: MemberTypeUser}, {ID: "usr-002", Type: MemberTypeUser},
		{ID: "usr-003", Type: MemberTypeUser}})

	suite.Require().Nil(svcErr)
	suite.Len(*runs, 6)
	f.store.AssertNumberOfCalls(suite.T(), "GetGroupMember", 3)
	f.store.AssertNotCalled(suite.T(), "GetGroupMembers", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything)
}

// A post flow that fails after the removal is written is reported, not passed over.
func (suite *GroupServiceTestSuite) TestRemoveGroupMembers_FailedPostFlowIsReported() {
	flows := newSystemFlowProviderMock(suite.T())
	f, _ := suite.memberRemoval()
	f.service.systemFlows = flows
	flows.EXPECT().ExecuteSystemFlow(mock.Anything, mock.MatchedBy(func(c *flowcommon.SystemFlowContext) bool {
		return c.Handle == systemFlowGroupMembershipRemovalPre
	})).Return(&flowcommon.SystemFlowResult{}, nil).Once()
	flows.EXPECT().ExecuteSystemFlow(mock.Anything, mock.MatchedBy(func(c *flowcommon.SystemFlowContext) bool {
		return c.Handle == systemFlowRevocationPost
	})).Return(nil, &tidcommon.InternalServerError).Once()

	_, svcErr := f.service.RemoveGroupMembers(context.Background(), "grp-001",
		[]Member{{ID: "usr-001", Type: MemberTypeUser}})

	suite.Equal(&tidcommon.InternalServerError, svcErr)
	f.store.AssertCalled(suite.T(), "RemoveGroupMembers", mock.Anything, "grp-001", mock.Anything)
}

// Removing a non-member changes nothing, so no flow runs.
func (suite *GroupServiceTestSuite) TestRemoveGroupMembers_ANonMemberRunsNoFlow() {
	f, _ := suite.memberRemoval()
	flows, runs := suite.recordFlows(nil, nil)
	f.service.systemFlows = flows

	_, svcErr := f.service.RemoveGroupMembers(context.Background(), "grp-001",
		[]Member{{ID: "usr-002", Type: MemberTypeUser}})

	suite.Require().Nil(svcErr)
	suite.Empty(*runs)
}

// A refused pre flow leaves every membership as it was.
func (suite *GroupServiceTestSuite) TestRemoveGroupMembers_RefusedFlowRemovesNothing() {
	f, _ := suite.memberRemoval()
	flows, runs := suite.recordFlows(&tidcommon.InternalServerError, nil)
	f.service.systemFlows = flows

	_, svcErr := f.service.RemoveGroupMembers(context.Background(), "grp-001",
		[]Member{{ID: "usr-001", Type: MemberTypeUser}})

	suite.Equal(&tidcommon.InternalServerError, svcErr)
	suite.Len(*runs, 1)
	f.store.AssertNotCalled(suite.T(), "RemoveGroupMembers", mock.Anything, mock.Anything, mock.Anything)
}

// flowFor matches a run of the given flow for the given member.
func flowFor(handle, memberID string) interface{} {
	return mock.MatchedBy(func(c *flowcommon.SystemFlowContext) bool {
		return c.Handle == handle && c.Inputs[systemFlowInputMember] == memberID
	})
}

// Only the requested principals that are members are removed, each once, with one pre and one post flow.
func (suite *GroupServiceTestSuite) TestRemoveGroupMembers_RemovesEachMemberOnce() {
	for name, requested := range map[string][]Member{
		"listed twice":      {{ID: "usr-001", Type: MemberTypeUser}, {ID: "usr-001", Type: MemberTypeUser}},
		"with a non-member": {{ID: "usr-001", Type: MemberTypeUser}, {ID: "usr-002", Type: MemberTypeUser}},
	} {
		suite.Run(name, func() {
			f, _ := suite.memberRemoval()
			flows := newSystemFlowProviderMock(suite.T())
			f.service.systemFlows = flows
			flows.EXPECT().ExecuteSystemFlow(mock.Anything, flowFor(systemFlowGroupMembershipRemovalPre, "usr-001")).
				Return(&flowcommon.SystemFlowResult{}, nil).Once()
			flows.EXPECT().ExecuteSystemFlow(mock.Anything, flowFor(systemFlowRevocationPost, "usr-001")).
				Return(&flowcommon.SystemFlowResult{}, nil).Once()

			_, svcErr := f.service.RemoveGroupMembers(context.Background(), "grp-001", requested)

			suite.Require().Nil(svcErr)
			f.store.AssertCalled(suite.T(), "RemoveGroupMembers", mock.Anything, "grp-001",
				[]Member{{ID: "usr-001", Type: memberTypeEntity}})
		})
	}
}

// A member that cannot be looked up runs no flow and removes nothing.
func (suite *GroupServiceTestSuite) TestRemoveGroupMembers_FailedMemberLookupRemovesNothing() {
	f := newGroupChangeFixture(suite.T())
	f.mayUpdateGroup()
	f.knownUsers("usr-001")
	f.store.On("GetGroupMember", mock.Anything, "grp-001", "usr-001").Return(nil, errors.New("db down")).Once()
	f.service.systemFlows = newSystemFlowProviderMock(suite.T())

	_, svcErr := f.service.RemoveGroupMembers(context.Background(), "grp-001",
		[]Member{{ID: "usr-001", Type: MemberTypeUser}})

	suite.Equal(&tidcommon.InternalServerError, svcErr)
	f.store.AssertNotCalled(suite.T(), "RemoveGroupMembers", mock.Anything, mock.Anything, mock.Anything)
}

// A later member's refused pre flow removes nothing and runs no post flow.
func (suite *GroupServiceTestSuite) TestRemoveGroupMembers_LaterRefusalRemovesNothing() {
	f, _ := suite.memberRemoval(Member{ID: "usr-001", Type: memberTypeEntity},
		Member{ID: "usr-002", Type: memberTypeEntity})
	flows := newSystemFlowProviderMock(suite.T())
	f.service.systemFlows = flows
	flows.EXPECT().ExecuteSystemFlow(mock.Anything, flowFor(systemFlowGroupMembershipRemovalPre, "usr-001")).
		Return(&flowcommon.SystemFlowResult{}, nil).Once()
	flows.EXPECT().ExecuteSystemFlow(mock.Anything, flowFor(systemFlowGroupMembershipRemovalPre, "usr-002")).
		Return(nil, &ErrorGroupNestingTooDeep).Once()

	_, svcErr := f.service.RemoveGroupMembers(context.Background(), "grp-001",
		[]Member{{ID: "usr-001", Type: MemberTypeUser}, {ID: "usr-002", Type: MemberTypeUser}})

	suite.Equal(&ErrorGroupNestingTooDeep, svcErr)
	f.store.AssertNotCalled(suite.T(), "RemoveGroupMembers", mock.Anything, mock.Anything, mock.Anything)
}

// A failed post flow does not stop the post flows of the other removed members.
func (suite *GroupServiceTestSuite) TestRemoveGroupMembers_EveryPostFlowRunsAfterOneFails() {
	f, _ := suite.memberRemoval(Member{ID: "usr-001", Type: memberTypeEntity},
		Member{ID: "usr-002", Type: memberTypeEntity})
	flows := newSystemFlowProviderMock(suite.T())
	f.service.systemFlows = flows
	flows.EXPECT().ExecuteSystemFlow(mock.Anything, mock.MatchedBy(func(c *flowcommon.SystemFlowContext) bool {
		return c.Handle == systemFlowGroupMembershipRemovalPre
	})).Return(&flowcommon.SystemFlowResult{}, nil).Times(2)
	flows.EXPECT().ExecuteSystemFlow(mock.Anything, flowFor(systemFlowRevocationPost, "usr-001")).
		Return(nil, &tidcommon.InternalServerError).Once()
	flows.EXPECT().ExecuteSystemFlow(mock.Anything, flowFor(systemFlowRevocationPost, "usr-002")).
		Return(&flowcommon.SystemFlowResult{}, nil).Once()

	_, svcErr := f.service.RemoveGroupMembers(context.Background(), "grp-001",
		[]Member{{ID: "usr-001", Type: MemberTypeUser}, {ID: "usr-002", Type: MemberTypeUser}})

	suite.Equal(&tidcommon.InternalServerError, svcErr)
}

// declarativeGroupRemoval returns a fixture whose grp-001 is declarative and declares usr-001 only.
func (suite *GroupServiceTestSuite) declarativeGroupRemoval() *groupChangeFixture {
	f := newGroupChangeFixture(suite.T())
	f.store.On("IsGroupDeclarative", mock.Anything, "grp-001").Return(true, nil)
	f.mayUpdateGroup()
	f.knownUsers("usr-001")
	f.store.On("IsGroupMemberDeclarative", mock.Anything, "grp-001", "usr-001").Return(true, nil).Maybe()
	f.service.systemFlows = newSystemFlowProviderMock(suite.T())
	return f
}

// A member a declarative group declares cannot be removed, so the removal is refused before any flow
// revokes what the member holds.
func (suite *GroupServiceTestSuite) TestRemoveGroupMembers_RefusesADeclaredMember() {
	f := suite.declarativeGroupRemoval()

	_, svcErr := f.service.RemoveGroupMembers(context.Background(), "grp-001",
		[]Member{{ID: "usr-001", Type: MemberTypeUser}})

	suite.Equal(&ErrorImmutableGroup, svcErr)
	f.store.AssertNotCalled(suite.T(), "GetGroupMember", mock.Anything, mock.Anything, mock.Anything)
	f.store.AssertNotCalled(suite.T(), "RemoveGroupMembers", mock.Anything, mock.Anything, mock.Anything)
}

// The validation the pre flow runs refuses a declared member the same way.
func (suite *GroupServiceTestSuite) TestValidateRemoveGroupMembers_RefusesADeclaredMember() {
	f := suite.declarativeGroupRemoval()

	svcErr := f.service.ValidateRemoveGroupMembers(context.Background(), "grp-001",
		[]Member{{ID: "usr-001", Type: MemberTypeUser}})

	suite.Equal(&ErrorImmutableGroup, svcErr)
}

// A failure to tell whether the group or the member is declared refuses the removal.
func (suite *GroupServiceTestSuite) TestRemoveGroupMembers_DeclarationCheckFailureRemovesNothing() {
	for name, arrange := range map[string]func(*groupChangeFixture){
		"group": func(f *groupChangeFixture) {
			f.store.On("IsGroupDeclarative", mock.Anything, "grp-001").Return(false, errors.New("file error"))
		},
		"member": func(f *groupChangeFixture) {
			f.store.On("IsGroupDeclarative", mock.Anything, "grp-001").Return(true, nil)
			f.store.On("IsGroupMemberDeclarative", mock.Anything, "grp-001", "usr-001").
				Return(false, errors.New("file error"))
		},
	} {
		suite.Run(name, func() {
			f := newGroupChangeFixture(suite.T())
			arrange(f)
			f.mayUpdateGroup()
			f.knownUsers("usr-001")
			f.service.systemFlows = newSystemFlowProviderMock(suite.T())

			_, svcErr := f.service.RemoveGroupMembers(context.Background(), "grp-001",
				[]Member{{ID: "usr-001", Type: MemberTypeUser}})

			suite.Equal(&tidcommon.InternalServerError, svcErr)
			f.store.AssertNotCalled(suite.T(), "RemoveGroupMembers", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}
