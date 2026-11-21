// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sysauthz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/security"
)

// ApprovalTestSuite tests the writes a caller rebuilt for a step it did not send may make.
type ApprovalTestSuite struct {
	suite.Suite
}

func TestApprovalTestSuite(t *testing.T) {
	suite.Run(t, new(ApprovalTestSuite))
}

// approvedCaller rebuilds admin-1 of ou-1 holding permissions, limited to the approved writes.
func approvedCaller(permissions []string, approved ...security.ApprovedAction) context.Context {
	return security.WithApprovedActions(context.Background(), "admin-1", "ou-1", permissions, approved, nil)
}

// A caller rebuilt for a step it did not send writes only what it approved, and still needs the permission.
// Reads keep the permissions it holds. The root permission does not lift the limit.
func (s *ApprovalTestSuite) TestIsActionAllowed_LimitedToTheApprovedWrites() {
	t := s.T()
	createInOU2 := security.ApprovedAction{Action: security.ActionCreateUser, OUID: "ou-2"}
	tests := []struct {
		name        string
		ctx         context.Context
		action      security.Action
		actionCtx   *ActionContext
		wantAllowed bool
	}{
		{"approved write", approvedCaller([]string{"system"}, createInOU2), security.ActionCreateUser,
			&ActionContext{ResourceType: security.ResourceTypeUser, OUID: "ou-2"}, true},
		{"write in another unit", approvedCaller([]string{"system"}, createInOU2), security.ActionCreateUser,
			&ActionContext{ResourceType: security.ResourceTypeUser, OUID: "ou-3"}, false},
		{"another write", approvedCaller([]string{"system"}, createInOU2), security.ActionDeleteUser,
			&ActionContext{ResourceType: security.ResourceTypeUser, OUID: "ou-2", ResourceID: "user-1"}, false},
		{"no action context", approvedCaller([]string{"system"}, createInOU2), security.ActionCreateUser, nil,
			false},
		{"approved without the permission", approvedCaller([]string{"system:group"}, createInOU2),
			security.ActionCreateUser, &ActionContext{ResourceType: security.ResourceTypeUser, OUID: "ou-2"}, false},
		{"read with the permission", approvedCaller([]string{"system"}), security.ActionReadUser,
			&ActionContext{ResourceType: security.ResourceTypeUser, OUID: "ou-3", ResourceID: "user-1"}, true},
		{"read without the permission", approvedCaller([]string{"system:group"}), security.ActionReadUser,
			&ActionContext{ResourceType: security.ResourceTypeUser, OUID: "ou-1", ResourceID: "user-1"}, false},
	}
	for boundaryName, service := range map[string]SystemAuthorizationServiceInterface{
		"ou": newOUBoundedAuthzService(), "deployment": newDeploymentBoundedAuthzService(),
	} {
		for _, tc := range tests {
			t.Run(boundaryName+"/"+tc.name, func(t *testing.T) {
				allowed, svcErr := service.IsActionAllowed(tc.ctx, tc.action, tc.actionCtx)

				require.Nil(t, svcErr)
				assert.Equal(t, tc.wantAllowed, allowed)
			})
		}
	}
}

// A caller limited to its approved writes changes only the memberships it approved, whatever it holds.
func (s *ApprovalTestSuite) TestCanGrantMembership_LimitedToTheApprovedMemberships() {
	t := s.T()
	svc := newGuardService(&stubPermissionResolver{})
	ctx := approvedCaller([]string{"system"},
		security.ApprovedAction{Action: security.ActionUpdateRole, OUID: "ou-2", ResourceIDs: []string{"role-1"}})

	assert.Nil(t, svc.CanGrantMembership(ctx, PrincipalTypeRole, "role-1"))

	svcErr := svc.CanGrantMembership(ctx, PrincipalTypeRole, "role-2")
	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorGrantNotPermitted.Code, svcErr.Code)

	svcErr = svc.CanGrantMembership(ctx, PrincipalTypeGroup, "role-1")
	require.NotNil(t, svcErr, "a role's approval does not cover a group of the same ID")
	assert.Equal(t, ErrorGrantNotPermitted.Code, svcErr.Code)

	ids, svcErr := svc.GrantableMemberships(ctx, PrincipalTypeRole, []string{"role-2", "role-1"})
	require.Nil(t, svcErr)
	assert.Equal(t, []string{"role-1"}, ids)

	ids, svcErr = svc.GrantableMemberships(ctx, PrincipalTypeGroup, []string{"group-1"})
	require.Nil(t, svcErr)
	assert.Empty(t, ids)
}

// A caller limited to its approved writes updates the user it created earlier in the flow without an approval for
// it, and still needs the permission; it neither deletes that user nor updates another.
func (s *ApprovalTestSuite) TestIsActionAllowed_UpdatesWhatTheFlowCreated() {
	t := s.T()
	createdBy := func(permissions []string) context.Context {
		ctx := approvedCaller(permissions)
		security.RecordCreated(ctx, security.ResourceTypeUser, "user-1")
		return ctx
	}
	userIn := func(id string) *ActionContext {
		return &ActionContext{ResourceType: security.ResourceTypeUser, OUID: "ou-2", ResourceID: id}
	}
	tests := []struct {
		name        string
		ctx         context.Context
		action      security.Action
		actionCtx   *ActionContext
		wantAllowed bool
	}{
		{"update the created user", createdBy([]string{"system"}), security.ActionUpdateUser, userIn("user-1"), true},
		{"update another user", createdBy([]string{"system"}), security.ActionUpdateUser, userIn("user-2"), false},
		{"delete the created user", createdBy([]string{"system"}), security.ActionDeleteUser, userIn("user-1"),
			false},
		{"update the created user without the permission", createdBy([]string{"system:group"}),
			security.ActionUpdateUser, userIn("user-1"), false},
	}
	for boundaryName, service := range map[string]SystemAuthorizationServiceInterface{
		"ou": newOUBoundedAuthzService(), "deployment": newDeploymentBoundedAuthzService(),
	} {
		for _, tc := range tests {
			t.Run(boundaryName+"/"+tc.name, func(t *testing.T) {
				allowed, svcErr := service.IsActionAllowed(tc.ctx, tc.action, tc.actionCtx)

				require.Nil(t, svcErr)
				assert.Equal(t, tc.wantAllowed, allowed)
			})
		}
	}
}
