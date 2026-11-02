// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entitytype

import (
	"context"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

func TestResolveDisplayAttributePaths(t *testing.T) {
	ctx := context.Background()
	logger := log.GetLogger()

	t.Run("NoServiceOrTypes", func(t *testing.T) {
		assert.Nil(t, ResolveDisplayAttributePaths(ctx, TypeCategoryAgent, []string{"default"}, nil, logger))

		svc := NewEntityTypeServiceInterfaceMock(t)
		assert.Nil(t, ResolveDisplayAttributePaths(ctx, TypeCategoryAgent, nil, svc, logger))
		assert.Nil(t, ResolveDisplayAttributePaths(ctx, TypeCategoryAgent, []string{"", ""}, svc, logger))
	})

	t.Run("ResolvesUniqueTypesInTheCategory", func(t *testing.T) {
		svc := NewEntityTypeServiceInterfaceMock(t)
		svc.On("GetDisplayAttributesByHandles", ctx, TypeCategoryAgent, []string{"default"}).
			Return(map[string]string{"default": "name"}, (*tidcommon.ServiceError)(nil)).Once()

		got := ResolveDisplayAttributePaths(ctx, TypeCategoryAgent, []string{"default", "default"}, svc, logger)

		assert.Equal(t, map[string]string{"default": "name"}, got)
	})

	t.Run("LookupFailureFallsBackToNil", func(t *testing.T) {
		svc := NewEntityTypeServiceInterfaceMock(t)
		svc.On("GetDisplayAttributesByHandles", ctx, TypeCategoryAgent, mock.Anything).
			Return(map[string]string(nil), &tidcommon.InternalServerError).Once()

		assert.Nil(t, ResolveDisplayAttributePaths(ctx, TypeCategoryAgent, []string{"default"}, svc, logger))
	})
}

func TestResolveEntityDisplays(t *testing.T) {
	ctx := context.Background()
	logger := log.GetLogger()
	attrs := func(json string) []byte { return []byte(json) }

	user := providers.Entity{ID: "u1", Category: providers.EntityCategoryUser, Type: "shared",
		Attributes: attrs(`{"username":"alice","name":"wrong"}`)}
	agent := providers.Entity{ID: "a1", Category: providers.EntityCategoryAgent, Type: "shared",
		Attributes: attrs(`{"username":"wrong","name":"Ledger Agent"}`)}

	t.Run("ResolvesEachCategoryIndependentlyWithOneLookupEach", func(t *testing.T) {
		svc := NewEntityTypeServiceInterfaceMock(t)
		svc.On("GetDisplayAttributesByHandles", ctx, TypeCategoryUser, []string{"shared"}).
			Return(map[string]string{"shared": "username"}, (*tidcommon.ServiceError)(nil)).Once()
		svc.On("GetDisplayAttributesByHandles", ctx, TypeCategoryAgent, []string{"shared"}).
			Return(map[string]string{"shared": "name"}, (*tidcommon.ServiceError)(nil)).Once()
		another := user
		another.ID = "u2"

		got := ResolveEntityDisplays(ctx, []providers.Entity{user, agent, another}, svc, logger)

		assert.Equal(t, map[string]string{"u1": "alice", "u2": "alice", "a1": "Ledger Agent"}, got)
	})

	t.Run("FailureInOneCategoryDoesNotBlockTheOther", func(t *testing.T) {
		svc := NewEntityTypeServiceInterfaceMock(t)
		svc.On("GetDisplayAttributesByHandles", ctx, TypeCategoryUser, mock.Anything).
			Return(map[string]string(nil), &tidcommon.InternalServerError).Once()
		svc.On("GetDisplayAttributesByHandles", ctx, TypeCategoryAgent, mock.Anything).
			Return(map[string]string{"shared": "name"}, (*tidcommon.ServiceError)(nil)).Once()

		got := ResolveEntityDisplays(ctx, []providers.Entity{user, agent}, svc, logger)

		assert.Equal(t, "u1", got["u1"])
		assert.Equal(t, "Ledger Agent", got["a1"])
	})

	t.Run("SkipsOtherCategoriesAndMakesNoLookupWhenNothingApplies", func(t *testing.T) {
		svc := NewEntityTypeServiceInterfaceMock(t)
		app := providers.Entity{ID: "app1", Category: providers.EntityCategoryApp, Type: "x"}

		got := ResolveEntityDisplays(ctx, []providers.Entity{app}, svc, logger)

		assert.Empty(t, got)
		svc.AssertNotCalled(t, "GetDisplayAttributesByHandles", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("FallsBackToTheIDWithoutAService", func(t *testing.T) {
		got := ResolveEntityDisplays(ctx, []providers.Entity{user, agent}, nil, logger)

		assert.Equal(t, map[string]string{"u1": "u1", "a1": "a1"}, got)
	})
}
