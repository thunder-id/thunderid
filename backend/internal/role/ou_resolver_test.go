// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	oupkg "github.com/thunder-id/thunderid/internal/ou"
)

func TestOURoleResolver_GetRoleListByOUID(t *testing.T) {
	ouFilter := map[string]interface{}{"ouId": "ou-1"}

	t.Run("owned roles come first, then shared ones", func(t *testing.T) {
		service := NewRoleServiceInterfaceMock(t)
		service.On("GetRoleList", context.Background(), 10, 0, ouFilter).
			Return(&RoleList{
				TotalResults: 7,
				Roles: []Role{
					{ID: "r1", Name: "Admin", Description: "Admin role", IsOwnedByOU: true},
					{ID: "r2", Name: "Viewer", Description: "Viewer role", IsReadOnly: true, IsOwnedByOU: true},
					{ID: "r3", Name: "Shared", IsReadOnly: true, IsOwnedByOU: false},
				},
			}, (*tidcommon.ServiceError)(nil)).Once()

		resolver := newOURoleResolver(service)
		roles, total, err := resolver.GetRoleListByOUID(context.Background(), "ou-1", 10, 0)

		require.NoError(t, err)
		require.Equal(t, 7, total)
		require.Equal(t, []oupkg.Role{
			{ID: "r1", Name: "Admin", Description: "Admin role", IsOwnedByOU: true},
			{ID: "r2", Name: "Viewer", Description: "Viewer role", IsReadOnly: true, IsOwnedByOU: true},
			{ID: "r3", Name: "Shared", IsReadOnly: true, IsOwnedByOU: false},
		}, roles)
	})

	t.Run("service error", func(t *testing.T) {
		service := NewRoleServiceInterfaceMock(t)
		service.On("GetRoleList", context.Background(), 10, 0, ouFilter).
			Return((*RoleList)(nil), &tidcommon.InternalServerError).Once()

		resolver := newOURoleResolver(service)
		roles, _, err := resolver.GetRoleListByOUID(context.Background(), "ou-1", 10, 0)

		require.Error(t, err)
		require.Nil(t, roles)
	})

	t.Run("empty results", func(t *testing.T) {
		service := NewRoleServiceInterfaceMock(t)
		service.On("GetRoleList", context.Background(), 10, 0, ouFilter).
			Return(&RoleList{Roles: []Role{}}, (*tidcommon.ServiceError)(nil)).Once()

		resolver := newOURoleResolver(service)
		roles, total, err := resolver.GetRoleListByOUID(context.Background(), "ou-1", 10, 0)

		require.NoError(t, err)
		require.Zero(t, total)
		require.NotNil(t, roles)
		require.Empty(t, roles)
	})
}
