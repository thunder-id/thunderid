// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ou

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

// allowOnly returns an authorization mock that allows only the given actions.
func allowOnly(t *testing.T, allowed ...security.Action) *sysauthzmock.SystemAuthorizationServiceInterfaceMock {
	authz := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
	authz.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).Return(
		func(_ context.Context, action security.Action, _ *sysauthz.ActionContext) bool {
			for _, a := range allowed {
				if a == action {
					return true
				}
			}
			return false
		},
		func(context.Context, security.Action, *sysauthz.ActionContext) *tidcommon.ServiceError { return nil },
	).Maybe()
	return authz
}

// An admin confined to its own unit may create and list children there, and update and delete only a child.
func TestAllowedActionsReportWhatTheCallerMayDo(t *testing.T) {
	ctx := context.Background()

	svc := &organizationUnitService{authzService: allowOnly(t, security.ActionCreateOU, security.ActionListChildOUs)}
	actions, svcErr := svc.allowedActions(ctx, "own", nil, false)
	require.Nil(t, svcErr)
	assert.Equal(t, []string{"createChild", "listChildren"}, actions)

	svc = &organizationUnitService{authzService: allowOnly(t, security.ActionUpdateOU, security.ActionDeleteOU)}
	actions, svcErr = svc.allowedActions(ctx, "child", nil, false)
	require.Nil(t, svcErr)
	assert.Equal(t, []string{"update", "delete"}, actions)

	svc = &organizationUnitService{authzService: newAllowAllAuthz(t)}
	actions, svcErr = svc.allowedActions(ctx, "ou", nil, true)
	require.Nil(t, svcErr)
	assert.Equal(t, []string{"createChild", "listChildren"}, actions, "a read-only unit allows no update or delete")

	units := []OrganizationUnitBasic{{ID: "a"}, {ID: "b", IsReadOnly: true}}
	require.Nil(t, svc.markAllowedActions(ctx, units, nil))
	assert.Equal(t, []string{"update", "delete", "createChild", "listChildren"}, units[0].AllowedActions)
	assert.Equal(t, []string{"createChild", "listChildren"}, units[1].AllowedActions)
}

func TestAllowedActionsFailOnAnAuthorizationError(t *testing.T) {
	authz := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
	authz.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
		Return(false, &tidcommon.InternalServerError).Once()
	svc := &organizationUnitService{authzService: authz}

	svcErr := svc.markAllowedActions(context.Background(), []OrganizationUnitBasic{{ID: "a"}}, nil)
	require.NotNil(t, svcErr)
	assert.Equal(t, tidcommon.InternalServerError, *svcErr)
}
