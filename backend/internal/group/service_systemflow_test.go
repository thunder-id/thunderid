// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package group

import (
	"context"
	"errors"

	"github.com/stretchr/testify/mock"

	flowcommon "github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// memberRemoval returns a fixture around grp-001 with the given members, usr-001 alone when none are
// given, and the order in which a removal writes. Every user it is asked about exists.
func (suite *GroupServiceTestSuite) memberRemoval(members ...Member) (*groupChangeFixture, *[]string) {
	f := newGroupChangeFixture(suite.T())
	order := []string{}
	f.mayUpdateGroup()
	if len(members) == 0 {
		members = []Member{{ID: "usr-001", Type: memberTypeEntity}}
	}
	f.readableGroup("grp-001", members...)
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

// Removing several members reads the membership once for the whole call, not once for each member.
func (suite *GroupServiceTestSuite) TestRemoveGroupMembers_ReadsTheMembershipOncePerCall() {
	f, _ := suite.memberRemoval(Member{ID: "usr-001", Type: memberTypeEntity},
		Member{ID: "usr-002", Type: memberTypeEntity}, Member{ID: "usr-003", Type: memberTypeEntity})
	flows, runs := suite.recordFlows(nil, nil)
	f.service.systemFlows = flows

	_, svcErr := f.service.RemoveGroupMembers(context.Background(), "grp-001", []Member{
		{ID: "usr-001", Type: MemberTypeUser}, {ID: "usr-002", Type: MemberTypeUser},
		{ID: "usr-003", Type: MemberTypeUser}})

	suite.Require().Nil(svcErr)
	suite.Len(*runs, 6)
	f.store.AssertNumberOfCalls(suite.T(), "GetGroupMembers", 1)
	f.store.AssertNotCalled(suite.T(), "GetGroupMember", mock.Anything, mock.Anything, mock.Anything)
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

// A membership that cannot be listed runs no flow and removes nothing.
func (suite *GroupServiceTestSuite) TestRemoveGroupMembers_FailedMembershipListingRemovesNothing() {
	f := newGroupChangeFixture(suite.T())
	f.mayUpdateGroup()
	f.knownUsers("usr-001")
	f.authz.On("IsActionAllowed", mock.Anything, security.ActionReadGroup, &sysauthz.ActionContext{
		ResourceType: security.ResourceTypeGroup, OUID: testOUID1, ResourceID: "grp-001",
	}).Return(true, (*tidcommon.ServiceError)(nil)).Once()
	f.store.On("GetGroupMemberCount", mock.Anything, "grp-001").Return(0, errors.New("db down")).Once()
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

func (suite *GroupServiceTestSuite) TestSetSystemFlowProvider() {
	service := &groupService{}
	flows := newSystemFlowProviderMock(suite.T())

	suite.Require().NoError(SetSystemFlowProvider(service, flows))
	suite.Same(flows, service.systemFlows)
	suite.Error(SetSystemFlowProvider(nil, flows))
}
