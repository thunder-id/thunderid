// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package security

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// ApprovalTestSuite tests the writes a caller rebuilt for a step it did not send may make.
type ApprovalTestSuite struct {
	suite.Suite
}

func TestApprovalTestSuite(t *testing.T) {
	suite.Run(t, new(ApprovalTestSuite))
}

func (s *ApprovalTestSuite) TestWithApprovedActions_RebuildsTheCallerLimitedToTheApprovedWrites() {
	t := s.T()
	ctx := WithApprovedActions(WithRuntimeContext(context.Background()), "admin-1", "ou-a",
		[]string{"system:user:create"}, []ApprovedAction{
			{Action: ActionCreateUser, OUID: "ou-b"},
			{Action: ActionUpdateRole, OUID: "ou-b", ResourceIDs: []string{"role-1"}},
		}, nil)

	assert.Equal(t, "admin-1", GetSubject(ctx))
	assert.Equal(t, "ou-a", GetOUID(ctx))
	assert.Equal(t, []string{"system:user:create"}, GetPermissions(ctx))
	assert.False(t, IsRuntimeContext(ctx))
	assert.True(t, IsRecordedCaller(ctx))
	assert.True(t, IsLimitedToApprovedActions(ctx))

	assert.True(t, IsApproved(ctx, ActionCreateUser, "ou-b", ""))
	assert.False(t, IsApproved(ctx, ActionCreateUser, "ou-a", ""), "another unit")
	assert.False(t, IsApproved(ctx, ActionDeleteUser, "ou-b", "user-1"), "another action")
	assert.True(t, IsApproved(ctx, ActionUpdateRole, "ou-b", "role-1"))
	assert.False(t, IsApproved(ctx, ActionUpdateRole, "ou-b", "role-2"), "another resource")

	assert.True(t, IsApprovedOn(ctx, ActionUpdateRole, "role-1"))
	assert.False(t, IsApprovedOn(ctx, ActionUpdateRole, "role-2"))
	assert.False(t, IsApprovedOn(ctx, ActionCreateUser, ""), "names no resource")
}

func (s *ApprovalTestSuite) TestIsApproved_ResourcesWithoutAUnitMatchInAnyUnit() {
	t := s.T()
	ctx := WithApprovedActions(context.Background(), "admin-1", "ou-a", nil,
		[]ApprovedAction{{Action: ActionUpdateUser, ResourceIDs: []string{"user-1"}}}, nil)

	assert.True(t, IsApproved(ctx, ActionUpdateUser, "ou-b", "user-1"))
	assert.False(t, IsApproved(ctx, ActionUpdateUser, "ou-b", "user-2"))
	assert.False(t, IsApproved(ctx, ActionUpdateUser, "ou-b", ""), "names no resource")
}

func (s *ApprovalTestSuite) TestWithApprovedActions_NoneApprovedStillLimits() {
	t := s.T()
	ctx := WithApprovedActions(context.Background(), "admin-1", "ou-a", nil, nil, nil)

	assert.True(t, IsLimitedToApprovedActions(ctx))
	assert.False(t, IsApproved(ctx, ActionCreateUser, "ou-a", ""))
}

func (s *ApprovalTestSuite) TestIsLimitedToApprovedActions_FalseForOtherCallers() {
	t := s.T()
	assert.False(t, IsLimitedToApprovedActions(context.Background()))
	assert.False(t, IsLimitedToApprovedActions(WithRecordedCaller(context.Background(), "admin-1", "ou-a", nil)))
	assert.False(t, IsLimitedToApprovedActions(withSecurityContext(context.Background(),
		newSecurityContext("admin-1", "ou-a", "token", []string{"system"}, nil))))
	assert.False(t, IsApproved(context.Background(), ActionCreateUser, "", ""))
}

func (s *ApprovalTestSuite) TestIsReadAction() {
	t := s.T()
	for _, action := range []Action{ActionReadUser, ActionListUsers, ActionListChildOUs} {
		assert.True(t, IsReadAction(action), action)
	}
	for _, action := range []Action{ActionCreateUser, ActionUpdateRole, ActionDeleteGroup, Action("")} {
		assert.False(t, IsReadAction(action), action)
	}
}

// A caller limited to its approved writes records what it creates and may then update exactly that entity.
func (s *ApprovalTestSuite) TestRecordCreated_LetsTheCallerUpdateOnlyWhatItCreated() {
	t := s.T()
	ctx := WithApprovedActions(context.Background(), "admin-1", "ou-a", nil, nil,
		[]CreatedEntity{{ResourceType: ResourceTypeAgent, ID: "agent-1"}})

	RecordCreated(ctx, ResourceTypeUser, "user-1")

	assert.Equal(t, []CreatedEntity{{ResourceType: ResourceTypeAgent, ID: "agent-1"},
		{ResourceType: ResourceTypeUser, ID: "user-1"}}, CreatedEntities(ctx))
	assert.True(t, IsUpdateOfCreated(ctx, ActionUpdateUser, ResourceTypeUser, "user-1"))
	assert.True(t, IsUpdateOfCreated(ctx, ActionUpdateAgent, ResourceTypeAgent, "agent-1"))
	assert.False(t, IsUpdateOfCreated(ctx, ActionDeleteUser, ResourceTypeUser, "user-1"), "not an update")
	assert.False(t, IsUpdateOfCreated(ctx, ActionUpdateUser, ResourceTypeUser, "user-2"), "another user")
	assert.False(t, IsUpdateOfCreated(ctx, ActionUpdateGroup, ResourceTypeGroup, "user-1"), "another type")
	assert.False(t, IsUpdateOfCreated(ctx, ActionUpdateUser, ResourceTypeUser, ""), "names no resource")
}

// Only a caller limited to its approved writes records what it creates.
func (s *ApprovalTestSuite) TestRecordCreated_IgnoredForOtherCallers() {
	t := s.T()
	for _, ctx := range []context.Context{
		context.Background(),
		WithRuntimeContext(context.Background()),
		WithRecordedCaller(context.Background(), "admin-1", "ou-a", nil),
		withSecurityContext(context.Background(), newSecurityContext("admin-1", "ou-a", "token", nil, nil)),
	} {
		RecordCreated(ctx, ResourceTypeUser, "user-1")

		assert.Empty(t, CreatedEntities(ctx))
		assert.False(t, IsUpdateOfCreated(ctx, ActionUpdateUser, ResourceTypeUser, "user-1"))
	}
}
