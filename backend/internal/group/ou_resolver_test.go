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

package group

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	oupkg "github.com/thunder-id/thunderid/internal/ou"
)

func TestOUGroupResolver_GetGroupCountByOUID(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		store := newGroupStoreInterfaceMock(t)
		store.On("GetGroupsByOrganizationUnitCount", context.Background(), "ou-1").
			Return(3, nil).Once()

		resolver := newOUGroupResolver(store)
		count, err := resolver.GetGroupCountByOUID(context.Background(), "ou-1")

		require.NoError(t, err)
		require.Equal(t, 3, count)
	})

	t.Run("store error", func(t *testing.T) {
		store := newGroupStoreInterfaceMock(t)
		store.On("GetGroupsByOrganizationUnitCount", context.Background(), "ou-1").
			Return(0, errors.New("db error")).Once()

		resolver := newOUGroupResolver(store)
		count, err := resolver.GetGroupCountByOUID(context.Background(), "ou-1")

		require.Error(t, err)
		require.Equal(t, 0, count)
	})
}

func TestOUGroupResolver_GetGroupListByOUID(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		store := newGroupStoreInterfaceMock(t)
		store.On("GetGroupsByOrganizationUnit", context.Background(), "ou-1", 10, 0).
			Return([]GroupBasicDAO{
				{ID: "g1", Name: "Group 1"},
				{ID: "g2", Name: "Group 2"},
			}, nil).Once()

		resolver := newOUGroupResolver(store)
		groups, err := resolver.GetGroupListByOUID(context.Background(), "ou-1", 10, 0)

		require.NoError(t, err)
		require.Len(t, groups, 2)
		require.Equal(t, oupkg.Group{ID: "g1", Name: "Group 1"}, groups[0])
		require.Equal(t, oupkg.Group{ID: "g2", Name: "Group 2"}, groups[1])
	})

	t.Run("store error", func(t *testing.T) {
		store := newGroupStoreInterfaceMock(t)
		store.On("GetGroupsByOrganizationUnit", context.Background(), "ou-1", 10, 0).
			Return([]GroupBasicDAO(nil), errors.New("db error")).Once()

		resolver := newOUGroupResolver(store)
		groups, err := resolver.GetGroupListByOUID(context.Background(), "ou-1", 10, 0)

		require.Error(t, err)
		require.Nil(t, groups)
	})

	t.Run("empty results", func(t *testing.T) {
		store := newGroupStoreInterfaceMock(t)
		store.On("GetGroupsByOrganizationUnit", context.Background(), "ou-1", 10, 0).
			Return([]GroupBasicDAO{}, nil).Once()

		resolver := newOUGroupResolver(store)
		groups, err := resolver.GetGroupListByOUID(context.Background(), "ou-1", 10, 0)

		require.NoError(t, err)
		require.Empty(t, groups)
	})
}
