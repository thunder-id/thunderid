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

func (s *ListOUTestSuite) TestFilterByListOUs() {
	items := []string{"ou-1:a", "ou-2:b", "ou-3:c"}
	ouOf := func(item string) string { return item[:4] }

	s.Equal(items, FilterByListOUs(items, nil, true, ouOf))
	s.Equal([]string{"ou-1:a", "ou-3:c"}, FilterByListOUs(items, []string{"ou-1", "ou-3"}, false, ouOf))
	s.Empty(FilterByListOUs(items, []string{}, false, ouOf))
}

func (s *ListOUTestSuite) TestBoundaryOUOfTheCaller() {
	ctx := context.Background()
	s.Equal("ou-1", boundaryOU(ctx, &stubAuthz{boundary: Boundary{Type: BoundaryTypeOU, OUID: "ou-1"}}))
	s.Empty(boundaryOU(ctx, &stubAuthz{boundary: Boundary{Type: BoundaryTypeDeployment}}))
	s.Empty(boundaryOU(security.WithRuntimeContext(ctx),
		&stubAuthz{boundary: Boundary{Type: BoundaryTypeOU, OUID: "ou-1"}}))
}

func (s *ListOUTestSuite) TestSharedAccessOU() {
	ctx := context.Background()
	confined := &stubAuthz{boundary: Boundary{Type: BoundaryTypeOU, OUID: "ou-1"}}
	tests := []struct {
		name    string
		authz   *stubAuthz
		owner   string
		refusal *tidcommon.ServiceError
		want    string
	}{
		{name: "refused in another unit", authz: confined, owner: "ou-2", refusal: &tidcommon.ErrorUnauthorized,
			want: "ou-1"},
		{name: "refused in its own unit", authz: confined, owner: "ou-1", refusal: &tidcommon.ErrorUnauthorized},
		{name: "not refused", authz: confined, owner: "ou-2"},
		{name: "another refusal", authz: confined, owner: "ou-2", refusal: &tidcommon.InternalServerError},
		{name: "deployment wide", authz: &stubAuthz{boundary: Boundary{Type: BoundaryTypeDeployment}},
			owner: "ou-2", refusal: &tidcommon.ErrorUnauthorized},
	}
	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.Equal(tc.want, SharedAccessOU(ctx, tc.authz, tc.owner, tc.refusal))
		})
	}
}

// stubAuthz answers the calls the list, any-unit and boundary helpers make.
type stubAuthz struct {
	SystemAuthorizationServiceInterface
	allowedOUs []string
	accessible *AccessibleResources
	boundary   Boundary
	err        *tidcommon.ServiceError
}

func (a *stubAuthz) GetBoundary(_ context.Context) Boundary {
	return a.boundary
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

func (s *ListOUTestSuite) TestResolveListOUs() {
	logger := log.GetLogger()
	ctx := context.Background()
	several := &stubAuthz{accessible: &AccessibleResources{IDs: []string{"ou-1", "ou-2"}}}
	every := &stubAuthz{accessible: &AccessibleResources{AllAllowed: true}}

	ouIDs, all, svcErr := ResolveListOUs(ctx, several, logger, security.ActionListVCs, "")
	s.Nil(svcErr)
	s.False(all)
	s.Equal([]string{"ou-1", "ou-2"}, ouIDs)

	ouIDs, all, svcErr = ResolveListOUs(ctx, several, logger, security.ActionListVCs, "ou-2")
	s.Nil(svcErr)
	s.False(all)
	s.Equal([]string{"ou-2"}, ouIDs)

	ouIDs, all, svcErr = ResolveListOUs(ctx, several, logger, security.ActionListVCs, "ou-3")
	s.Nil(svcErr)
	s.False(all)
	s.Empty(ouIDs)

	ouIDs, all, svcErr = ResolveListOUs(ctx, every, logger, security.ActionListVCs, "")
	s.Nil(svcErr)
	s.True(all)
	s.Nil(ouIDs)

	ouIDs, all, svcErr = ResolveListOUs(ctx, every, logger, security.ActionListVCs, "ou-3")
	s.Nil(svcErr)
	s.False(all)
	s.Equal([]string{"ou-3"}, ouIDs)

	_, _, svcErr = ResolveListOUs(ctx, &stubAuthz{err: &tidcommon.InternalServerError}, logger,
		security.ActionListVCs, "")
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
