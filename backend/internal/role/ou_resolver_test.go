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

package role

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	oupkg "github.com/thunder-id/thunderid/internal/ou"
)

func TestOURoleResolver_GetRoleCountByOUID(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		store := newRoleStoreInterfaceMock(t)
		store.On("GetRoleListCountByOUID", context.Background(), "ou-1").
			Return(3, nil).Once()

		resolver := newOURoleResolver(store)
		count, err := resolver.GetRoleCountByOUID(context.Background(), "ou-1")

		require.NoError(t, err)
		require.Equal(t, 3, count)
	})

	t.Run("store error", func(t *testing.T) {
		store := newRoleStoreInterfaceMock(t)
		store.On("GetRoleListCountByOUID", context.Background(), "ou-1").
			Return(0, errors.New("db error")).Once()

		resolver := newOURoleResolver(store)
		count, err := resolver.GetRoleCountByOUID(context.Background(), "ou-1")

		require.Error(t, err)
		require.Equal(t, 0, count)
	})
}

func TestOURoleResolver_GetRoleListByOUID(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		store := newRoleStoreInterfaceMock(t)
		store.On("GetRoleListByOUID", context.Background(), "ou-1", 10, 0).
			Return([]Role{
				{ID: "r1", Name: "Admin", Description: "Admin role", IsReadOnly: false},
				{ID: "r2", Name: "Viewer", Description: "Viewer role", IsReadOnly: true},
			}, nil).Once()

		resolver := newOURoleResolver(store)
		roles, err := resolver.GetRoleListByOUID(context.Background(), "ou-1", 10, 0)

		require.NoError(t, err)
		require.Len(t, roles, 2)
		require.Equal(t, oupkg.Role{ID: "r1", Name: "Admin", Description: "Admin role", IsReadOnly: false}, roles[0])
		require.Equal(t, oupkg.Role{ID: "r2", Name: "Viewer", Description: "Viewer role", IsReadOnly: true}, roles[1])
	})

	t.Run("store error", func(t *testing.T) {
		store := newRoleStoreInterfaceMock(t)
		store.On("GetRoleListByOUID", context.Background(), "ou-1", 10, 0).
			Return([]Role(nil), errors.New("db error")).Once()

		resolver := newOURoleResolver(store)
		roles, err := resolver.GetRoleListByOUID(context.Background(), "ou-1", 10, 0)

		require.Error(t, err)
		require.Nil(t, roles)
	})

	t.Run("empty results", func(t *testing.T) {
		store := newRoleStoreInterfaceMock(t)
		store.On("GetRoleListByOUID", context.Background(), "ou-1", 10, 0).
			Return([]Role{}, nil).Once()

		resolver := newOURoleResolver(store)
		roles, err := resolver.GetRoleListByOUID(context.Background(), "ou-1", 10, 0)

		require.NoError(t, err)
		require.NotNil(t, roles)
		require.Empty(t, roles)
	})
}
