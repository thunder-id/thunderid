// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/managedresource"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/managedresourcemock"
)

// What a control plane deployed is changed only by a deploy from it: anything else is refused, the managed
// import is let through, and a failed read of the marks refuses the change.
func TestAManagedResourceIsChangedOnlyByTheManagedImport(t *testing.T) {
	ctx := context.Background()
	store := managedresourcemock.NewStoreInterfaceMock(t)
	svc := &agentService{managed: store, logger: log.GetLogger()}

	store.EXPECT().IsManaged(ctx, "id-1", managedresource.TypeAgent).Return(true, nil).Once()
	svcErr := svc.refuseManaged(ctx, "id-1")
	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorManagedAgent.Code, svcErr.Code)

	importCtx := managedresource.WithManagedImport(ctx)
	assert.Nil(t, svc.refuseManaged(importCtx, "id-1"))

	store.EXPECT().IsManaged(ctx, "id-1", managedresource.TypeAgent).
		Return(false, errors.New("marker store unavailable")).Once()
	svcErr = svc.refuseManaged(ctx, "id-1")
	require.NotNil(t, svcErr)
	assert.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)

	store.EXPECT().IsManaged(ctx, "id-1", managedresource.TypeAgent).Return(false, nil).Once()
	assert.Nil(t, svc.refuseManaged(ctx, "id-1"))
}

// A declared agent is refused as declarative before the marks are read.
func TestADeclaredAgentIsRefusedAsDeclarative(t *testing.T) {
	svc := &agentService{}
	svcErr := svc.refuseReadOnly(context.Background(), &providers.Entity{ID: "a-1", IsReadOnly: true})
	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorCannotModifyDeclarativeResource.Code, svcErr.Code)
}
