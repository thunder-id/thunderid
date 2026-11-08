// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sysauthz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
)

// ListOUTestSuite covers deciding the organization unit a list serves.
type ListOUTestSuite struct {
	suite.Suite
}

func TestListOUTestSuite(t *testing.T) {
	suite.Run(t, new(ListOUTestSuite))
}

func (s *ListOUTestSuite) TestListOU() {
	tests := []struct {
		name        string
		accessible  AccessibleResources
		requested   string
		wantOU      string
		wantVisible bool
		wantErr     error
	}{
		{name: "all allowed lists every unit", accessible: AccessibleResources{AllAllowed: true},
			wantOU: "", wantVisible: true},
		{name: "all allowed keeps the requested unit", accessible: AccessibleResources{AllAllowed: true},
			requested: "ou-2", wantOU: "ou-2", wantVisible: true},
		{name: "one unit lists it", accessible: AccessibleResources{IDs: []string{"ou-1"}},
			wantOU: "ou-1", wantVisible: true},
		{name: "requested accessible unit", accessible: AccessibleResources{IDs: []string{"ou-1"}},
			requested: "ou-1", wantOU: "ou-1", wantVisible: true},
		{name: "requested inaccessible unit lists nothing", accessible: AccessibleResources{IDs: []string{"ou-1"}},
			requested: "ou-2", wantOU: "ou-2", wantVisible: false},
		{name: "no unit lists nothing", accessible: AccessibleResources{IDs: []string{}},
			wantOU: "", wantVisible: false},
		{name: "several units with no requested unit", accessible: AccessibleResources{IDs: []string{"ou-1", "ou-2"}},
			wantOU: "", wantVisible: false, wantErr: errMultipleListOUs},
		{name: "several units with a requested unit", accessible: AccessibleResources{IDs: []string{"ou-1", "ou-2"}},
			requested: "ou-2", wantOU: "ou-2", wantVisible: true},
	}
	for _, tc := range tests {
		s.Run(tc.name, func() {
			ouID, visible, err := tc.accessible.listOU(tc.requested)
			s.Equal(tc.wantOU, ouID)
			s.Equal(tc.wantVisible, visible)
			s.ErrorIs(err, tc.wantErr)
		})
	}
}

func (s *ListOUTestSuite) TestFilterByListOU() {
	items := []string{"ou-1:a", "ou-2:b", "ou-1:c"}
	ouOf := func(item string) string { return item[:4] }

	s.Equal(items, FilterByListOU(items, "", true, ouOf))
	s.Equal([]string{"ou-1:a", "ou-1:c"}, FilterByListOU(items, "ou-1", true, ouOf))
	s.Empty(FilterByListOU(items, "ou-1", false, ouOf))
}

// stubAuthz answers the two calls the list and any-unit helpers make.
type stubAuthz struct {
	SystemAuthorizationServiceInterface
	allowedOUs []string
	accessible *AccessibleResources
	err        *tidcommon.ServiceError
}

func (a *stubAuthz) IsActionAllowed(_ context.Context, _ security.Action,
	actionCtx *ActionContext) (bool, *tidcommon.ServiceError) {
	if a.err != nil {
		return false, a.err
	}
	for _, ouID := range a.allowedOUs {
		if ouID == actionCtx.OUID {
			return true, nil
		}
	}
	return false, nil
}

func (a *stubAuthz) GetAccessibleResources(_ context.Context, _ security.Action,
	_ security.ResourceType) (*AccessibleResources, *tidcommon.ServiceError) {
	return a.accessible, a.err
}

func (s *ListOUTestSuite) TestResolveListOU() {
	logger := log.GetLogger()
	ctx := context.Background()

	ouID, visible, svcErr := ResolveListOU(ctx, &stubAuthz{accessible: &AccessibleResources{IDs: []string{"ou-1"}}},
		logger, security.ActionListRoles, "")
	s.Nil(svcErr)
	s.Equal("ou-1", ouID)
	s.True(visible)

	_, _, svcErr = ResolveListOU(ctx, &stubAuthz{accessible: &AccessibleResources{IDs: []string{"ou-1", "ou-2"}}},
		logger, security.ActionListRoles, "")
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)

	_, _, svcErr = ResolveListOU(ctx, &stubAuthz{err: &tidcommon.InternalServerError}, logger,
		security.ActionListRoles, "")
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ListOUTestSuite) TestCheckActionInAnyOU() {
	logger := log.GetLogger()
	ctx := context.Background()
	authz := &stubAuthz{allowedOUs: []string{"ou-2"}}

	s.Nil(CheckActionInAnyOU(ctx, authz, logger, security.ActionReadRole, security.ResourceTypeRole,
		"ou-1", "ou-2"))

	svcErr := CheckActionInAnyOU(ctx, authz, logger, security.ActionReadRole, security.ResourceTypeRole, "ou-1")
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)

	svcErr = CheckActionInAnyOU(ctx, &stubAuthz{err: &tidcommon.InternalServerError}, logger,
		security.ActionReadRole, security.ResourceTypeRole, "ou-1")
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}
