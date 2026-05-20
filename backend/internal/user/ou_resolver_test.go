/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package user

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	entitypkg "github.com/thunder-id/thunderid/internal/entity"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	i18ncore "github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
)

func TestOUUserResolver_GetUserCountByOUID(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := entitymock.NewEntityServiceInterfaceMock(t)
		svc.On("GetEntityListCountByOUIDs", context.Background(),
			entitypkg.EntityCategoryUser, []string{"ou-1"}, (map[string]interface{})(nil)).
			Return(5, nil).Once()

		resolver := newOUUserResolver(svc, nil)
		count, err := resolver.GetUserCountByOUID(context.Background(), "ou-1")

		require.NoError(t, err)
		require.Equal(t, 5, count)
	})

	t.Run("store error", func(t *testing.T) {
		svc := entitymock.NewEntityServiceInterfaceMock(t)
		svc.On("GetEntityListCountByOUIDs", context.Background(),
			entitypkg.EntityCategoryUser, []string{"ou-1"}, (map[string]interface{})(nil)).
			Return(0, errors.New("db error")).Once()

		resolver := newOUUserResolver(svc, nil)
		count, err := resolver.GetUserCountByOUID(context.Background(), "ou-1")

		require.Error(t, err)
		require.Equal(t, 0, count)
	})
}

func TestOUUserResolver_GetUserListByOUID(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := entitymock.NewEntityServiceInterfaceMock(t)
		svc.On("GetEntityListByOUIDs", context.Background(),
			entitypkg.EntityCategoryUser, []string{"ou-1"}, 10, 0, (map[string]interface{})(nil)).
			Return([]entitypkg.Entity{
				{ID: "user-1"},
				{ID: "user-2"},
			}, nil).Once()

		resolver := newOUUserResolver(svc, nil)
		users, err := resolver.GetUserListByOUID(context.Background(), "ou-1", 10, 0, false)

		require.NoError(t, err)
		require.Len(t, users, 2)
		require.Equal(t, oupkg.User{ID: "user-1"}, users[0])
		require.Equal(t, oupkg.User{ID: "user-2"}, users[1])
	})

	t.Run("store error", func(t *testing.T) {
		svc := entitymock.NewEntityServiceInterfaceMock(t)
		svc.On("GetEntityListByOUIDs", context.Background(),
			entitypkg.EntityCategoryUser, []string{"ou-1"}, 10, 0, (map[string]interface{})(nil)).
			Return([]entitypkg.Entity(nil), errors.New("db error")).Once()

		resolver := newOUUserResolver(svc, nil)
		users, err := resolver.GetUserListByOUID(context.Background(), "ou-1", 10, 0, false)

		require.Error(t, err)
		require.Nil(t, users)
	})

	t.Run("empty results", func(t *testing.T) {
		svc := entitymock.NewEntityServiceInterfaceMock(t)
		svc.On("GetEntityListByOUIDs", context.Background(),
			entitypkg.EntityCategoryUser, []string{"ou-1"}, 10, 0, (map[string]interface{})(nil)).
			Return([]entitypkg.Entity{}, nil).Once()

		resolver := newOUUserResolver(svc, nil)
		users, err := resolver.GetUserListByOUID(context.Background(), "ou-1", 10, 0, false)

		require.NoError(t, err)
		require.Empty(t, users)
	})

	t.Run("with display resolution", func(t *testing.T) {
		svc := entitymock.NewEntityServiceInterfaceMock(t)
		svc.On("GetEntityListByOUIDs", context.Background(),
			entitypkg.EntityCategoryUser, []string{"ou-1"}, 10, 0, (map[string]interface{})(nil)).
			Return([]entitypkg.Entity{
				{ID: "user-1", Type: "employee", Attributes: json.RawMessage(`{"email":"alice@example.com"}`)},
				{ID: "user-2", Type: "contractor", Attributes: json.RawMessage(`{"profile":{"fullName":"Bob Smith"}}`)},
			}, nil).Once()

		schemaMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
		schemaMock.On("GetDisplayAttributesByNames", mock.Anything, mock.Anything,
			mock.MatchedBy(func(names []string) bool {
				if len(names) != 2 {
					return false
				}
				has := map[string]bool{names[0]: true, names[1]: true}
				return has["employee"] && has["contractor"]
			})).Return(map[string]string{
			"employee":   "email",
			"contractor": "profile.fullName",
		}, (*serviceerror.ServiceError)(nil)).Once()

		resolver := newOUUserResolver(svc, schemaMock)
		users, err := resolver.GetUserListByOUID(context.Background(), "ou-1", 10, 0, true)

		require.NoError(t, err)
		require.Len(t, users, 2)
		require.Equal(t, "employee", users[0].Type)
		require.Equal(t, "alice@example.com", users[0].Display)
		require.Equal(t, "contractor", users[1].Type)
		require.Equal(t, "Bob Smith", users[1].Display)
	})

	t.Run("display fallback to ID on schema error", func(t *testing.T) {
		svc := entitymock.NewEntityServiceInterfaceMock(t)
		svc.On("GetEntityListByOUIDs", context.Background(),
			entitypkg.EntityCategoryUser, []string{"ou-1"}, 10, 0, (map[string]interface{})(nil)).
			Return([]entitypkg.Entity{
				{ID: "user-1", Type: "employee", Attributes: json.RawMessage(`{"email":"alice@example.com"}`)},
			}, nil).Once()

		schemaMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
		schemaErr := &serviceerror.ServiceError{
			Code:  "500",
			Error: i18ncore.I18nMessage{DefaultValue: "schema unavailable"},
		}
		schemaMock.On("GetDisplayAttributesByNames", mock.Anything, mock.Anything, mock.Anything).
			Return((map[string]string)(nil), schemaErr).Once()

		resolver := newOUUserResolver(svc, schemaMock)
		users, err := resolver.GetUserListByOUID(context.Background(), "ou-1", 10, 0, true)

		require.NoError(t, err)
		require.Len(t, users, 1)
		require.Equal(t, "employee", users[0].Type)
		// Falls back to user ID when schema service fails
		require.Equal(t, "user-1", users[0].Display)
	})

	t.Run("display fallback to ID on attribute mismatch", func(t *testing.T) {
		svc := entitymock.NewEntityServiceInterfaceMock(t)
		svc.On("GetEntityListByOUIDs", context.Background(),
			entitypkg.EntityCategoryUser, []string{"ou-1"}, 10, 0, (map[string]interface{})(nil)).
			Return([]entitypkg.Entity{
				{ID: "user-1", Type: "employee", Attributes: json.RawMessage(`{"name":"Alice"}`)},
			}, nil).Once()

		schemaMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
		schemaMock.On("GetDisplayAttributesByNames", mock.Anything, mock.Anything, []string{"employee"}).
			Return(map[string]string{"employee": "email"}, (*serviceerror.ServiceError)(nil)).Once()

		resolver := newOUUserResolver(svc, schemaMock)
		users, err := resolver.GetUserListByOUID(context.Background(), "ou-1", 10, 0, true)

		require.NoError(t, err)
		require.Len(t, users, 1)
		require.Equal(t, "employee", users[0].Type)
		// Falls back to user ID when attribute path doesn't match
		require.Equal(t, "user-1", users[0].Display)
	})
}
