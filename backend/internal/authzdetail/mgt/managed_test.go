// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzdetailmgt

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/thunder-id/thunderid/internal/system/managedresource"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// A declared type is refused as declarative; a deployed one is refused to all but the managed import.
func TestARefusedChangeSaysWhyTheTypeIsReadOnly(t *testing.T) {
	ctx := context.Background()
	declared := &providers.AuthorizationDetailType{IsReadOnly: true}
	plain := &providers.AuthorizationDetailType{}

	assert.Equal(t, ErrorImmutableType.Code, refuseReadOnly(ctx, declared, false).Code)
	assert.Equal(t, ErrorManagedType.Code, refuseReadOnly(ctx, plain, true).Code)
	assert.Nil(t, refuseReadOnly(managedresource.WithManagedImport(ctx), plain, true))
	assert.Nil(t, refuseReadOnly(ctx, plain, false))
}
