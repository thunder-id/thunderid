// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/entity"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
)

// resolverOverPage returns a resolver whose entity service answers the page request for ou-1 with page.
func resolverOverPage(
	t *testing.T, page *entity.OUEntityPage, schema *entitytypemock.EntityTypeServiceInterfaceMock,
) oupkg.OUUserResolver {
	svc := entitymock.NewEntityServiceInterfaceMock(t)
	svc.On("GetEntityPageInOU", mock.Anything, providers.EntityCategoryUser, "ou-1", entity.OUEntityScopeAll,
		10, 0, (map[string]interface{})(nil)).Return(page, nil).Once()
	if schema == nil {
		return newOUUserResolver(svc, nil)
	}
	return newOUUserResolver(svc, schema)
}

func TestOUUserResolver_GetUserListByOUID(t *testing.T) {
	t.Run("owned users come first, then shared ones", func(t *testing.T) {
		resolver := resolverOverPage(t, &entity.OUEntityPage{
			Owned:        []providers.Entity{{ID: "user-1"}, {ID: "user-2"}},
			Shared:       []providers.Entity{{ID: "user-3"}},
			TotalResults: 7,
		}, nil)

		users, total, err := resolver.GetUserListByOUID(context.Background(), "ou-1", 10, 0, false)

		require.NoError(t, err)
		require.Equal(t, 7, total)
		require.Equal(t, []oupkg.User{
			{ID: "user-1", IsOwnedByOU: true},
			{ID: "user-2", IsOwnedByOU: true},
			{ID: "user-3", IsOwnedByOU: false},
		}, users)
	})

	t.Run("store error", func(t *testing.T) {
		svc := entitymock.NewEntityServiceInterfaceMock(t)
		svc.On("GetEntityPageInOU", mock.Anything, providers.EntityCategoryUser, "ou-1", entity.OUEntityScopeAll,
			10, 0, (map[string]interface{})(nil)).Return(nil, errors.New("db error")).Once()
		resolver := newOUUserResolver(svc, nil)

		users, _, err := resolver.GetUserListByOUID(context.Background(), "ou-1", 10, 0, false)

		require.Error(t, err)
		require.Nil(t, users)
	})

	t.Run("with display resolution", func(t *testing.T) {
		schemaMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
		schemaMock.On("GetDisplayAttributesByHandles", mock.Anything, mock.Anything,
			mock.MatchedBy(func(names []string) bool {
				if len(names) != 2 {
					return false
				}
				has := map[string]bool{names[0]: true, names[1]: true}
				return has["employee"] && has["contractor"]
			})).Return(map[string]string{
			"employee":   "email",
			"contractor": "profile.fullName",
		}, (*tidcommon.ServiceError)(nil)).Once()
		resolver := resolverOverPage(t, &entity.OUEntityPage{
			Owned: []providers.Entity{
				{ID: "user-1", Type: "employee", Attributes: json.RawMessage(`{"email":"alice@example.com"}`)},
			},
			Shared: []providers.Entity{
				{ID: "user-2", Type: "contractor", Attributes: json.RawMessage(`{"profile":{"fullName":"Bob Smith"}}`)},
			},
			TotalResults: 2,
		}, schemaMock)

		users, _, err := resolver.GetUserListByOUID(context.Background(), "ou-1", 10, 0, true)

		require.NoError(t, err)
		require.Len(t, users, 2)
		require.Equal(t, "employee", users[0].Type)
		require.Equal(t, "alice@example.com", users[0].Display)
		require.Equal(t, "contractor", users[1].Type)
		require.Equal(t, "Bob Smith", users[1].Display)
	})

	t.Run("display fallback to ID on schema error", func(t *testing.T) {
		schemaMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
		schemaErr := &tidcommon.ServiceError{
			Code:  "500",
			Error: tidcommon.I18nMessage{DefaultValue: "schema unavailable"},
		}
		schemaMock.On("GetDisplayAttributesByHandles", mock.Anything, mock.Anything, mock.Anything).
			Return((map[string]string)(nil), schemaErr).Once()
		resolver := resolverOverPage(t, &entity.OUEntityPage{
			Owned: []providers.Entity{
				{ID: "user-1", Type: "employee", Attributes: json.RawMessage(`{"email":"alice@example.com"}`)},
			},
			Shared:       []providers.Entity{},
			TotalResults: 1,
		}, schemaMock)

		users, _, err := resolver.GetUserListByOUID(context.Background(), "ou-1", 10, 0, true)

		require.NoError(t, err)
		require.Len(t, users, 1)
		// Falls back to user ID when schema service fails
		require.Equal(t, "user-1", users[0].Display)
	})

	t.Run("display fallback to ID on attribute mismatch", func(t *testing.T) {
		schemaMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
		schemaMock.On("GetDisplayAttributesByHandles", mock.Anything, mock.Anything, []string{"employee"}).
			Return(map[string]string{"employee": "email"}, (*tidcommon.ServiceError)(nil)).Once()
		resolver := resolverOverPage(t, &entity.OUEntityPage{
			Owned: []providers.Entity{
				{ID: "user-1", Type: "employee", Attributes: json.RawMessage(`{"name":"Alice"}`)},
			},
			Shared:       []providers.Entity{},
			TotalResults: 1,
		}, schemaMock)

		users, _, err := resolver.GetUserListByOUID(context.Background(), "ou-1", 10, 0, true)

		require.NoError(t, err)
		require.Len(t, users, 1)
		// Falls back to user ID when attribute path doesn't match
		require.Equal(t, "user-1", users[0].Display)
	})
}

func TestOUUserResolver_GetResourceDependencies(t *testing.T) {
	t.Run("reports users in the organization unit as restrict", func(t *testing.T) {
		svc := entitymock.NewEntityServiceInterfaceMock(t)
		svc.On("GetEntityListByOUIDs", context.Background(),
			providers.EntityCategoryUser, []string{"ou-1"},
			serverconst.MaxCompositeStoreRecords, 0, (map[string]interface{})(nil)).
			Return([]providers.Entity{{ID: "user-1"}}, nil).Once()

		resolver := newOUUserResolver(svc, nil)
		deps, err := resolver.GetResourceDependencies(
			context.Background(), resourcedependency.ResourceTypeOU, "ou-1")

		require.NoError(t, err)
		require.Len(t, deps, 1)
		require.Equal(t, resourcedependency.ResourceTypeUser, deps[0].ResourceType)
		require.Equal(t, "user-1", deps[0].ID)
		require.Equal(t, resourcedependency.BehaviorRestrict, deps[0].BehaviorOnDelete)
	})

	t.Run("ignores non-organization-unit targets", func(t *testing.T) {
		svc := entitymock.NewEntityServiceInterfaceMock(t)

		resolver := newOUUserResolver(svc, nil)
		deps, err := resolver.GetResourceDependencies(
			context.Background(), resourcedependency.ResourceTypeGroup, "group-1")

		require.NoError(t, err)
		require.Empty(t, deps)
	})

	t.Run("store error", func(t *testing.T) {
		svc := entitymock.NewEntityServiceInterfaceMock(t)
		svc.On("GetEntityListByOUIDs", context.Background(),
			providers.EntityCategoryUser, []string{"ou-1"},
			serverconst.MaxCompositeStoreRecords, 0, (map[string]interface{})(nil)).
			Return([]providers.Entity(nil), errors.New("db error")).Once()

		resolver := newOUUserResolver(svc, nil)
		deps, err := resolver.GetResourceDependencies(
			context.Background(), resourcedependency.ResourceTypeOU, "ou-1")

		require.Error(t, err)
		require.Nil(t, deps)
	})
}
