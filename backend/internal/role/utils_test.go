// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/revocation"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/groupmock"
	"github.com/thunder-id/thunderid/tests/mocks/resourcemock"
)

const (
	targetRoleID   = "role-1"
	targetUserID   = "user-1"
	targetGroupID  = "group-1"
	californiaRSID = "rs-ca"
	ohioRSID       = "rs-oh"
	californiaAud  = "https://api.dmv.ca.gov"
	ohioAud        = "https://api.dmv.oh.gov"
)

func stubDirectMember(groups *groupmock.GroupServiceInterfaceMock, memberID string, member *group.Member) {
	groups.On("GetDirectGroupMember", mock.Anything, targetGroupID, memberID).Return(member, nil)
}

func stubGroupEntities(groups *groupmock.GroupServiceInterfaceMock, groupID string, entityIDs ...string) {
	groups.On("GetTransitiveMemberEntities", mock.Anything, groupID).Return(entityIDs, nil)
}

func stubResourceServer(resources *resourcemock.ResourceServiceInterfaceMock, id, identifier string) {
	resources.On("GetResourceServer", mock.Anything, id).
		Return(&providers.ResourceServer{ID: id, Identifier: identifier}, nil)
}

type RoleUtilsTestSuite struct {
	suite.Suite
	groups    *groupmock.GroupServiceInterfaceMock
	resources *resourcemock.ResourceServiceInterfaceMock
}

func TestRoleUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(RoleUtilsTestSuite))
}

func (s *RoleUtilsTestSuite) SetupTest() {
	s.groups = groupmock.NewGroupServiceInterfaceMock(s.T())
	s.resources = resourcemock.NewResourceServiceInterfaceMock(s.T())
}

func (s *RoleUtilsTestSuite) TestResolveScopes_PairsEachPermissionWithItsAudience() {
	stubResourceServer(s.resources, californiaRSID, californiaAud)
	stubResourceServer(s.resources, ohioRSID, ohioAud)

	scopes, svcErr := resolveScopes(context.Background(), s.resources, []ResourcePermissions{
		{ResourceServerID: californiaRSID, Permissions: []string{"license", "vehicle-registration"}},
		{ResourceServerID: ohioRSID, Permissions: []string{"license"}},
		{ResourceServerID: "rs-empty"},
	})

	s.Require().Nil(svcErr)
	s.ElementsMatch([]revocation.AudienceScope{
		{Audience: californiaAud, Scope: "license"},
		{Audience: californiaAud, Scope: "vehicle-registration"},
		{Audience: ohioAud, Scope: "license"},
	}, scopes)
}

func (s *RoleUtilsTestSuite) TestDroppedPermissions_KeysByResourceServer() {
	dropped := droppedPermissions(
		[]ResourcePermissions{
			{ResourceServerID: californiaRSID, Permissions: []string{"license", "permits"}},
			{ResourceServerID: ohioRSID, Permissions: []string{"license"}},
		},
		[]ResourcePermissions{
			{ResourceServerID: californiaRSID, Permissions: []string{"permits"}},
			{ResourceServerID: ohioRSID, Permissions: []string{"permits"}},
		})

	s.Equal([]ResourcePermissions{
		{ResourceServerID: californiaRSID, Permissions: []string{"license"}},
		{ResourceServerID: ohioRSID, Permissions: []string{"license"}},
	}, dropped)
}

func (s *RoleUtilsTestSuite) TestDroppedPermissions_NothingDropped() {
	current := []ResourcePermissions{{ResourceServerID: californiaRSID, Permissions: []string{"license"}}}
	updated := []ResourcePermissions{current[0], {ResourceServerID: ohioRSID, Permissions: []string{"license"}}}

	s.Empty(droppedPermissions(current, updated))
}

func (s *RoleUtilsTestSuite) TestResolveScopes_SkipsResourceServerWithoutIdentifier() {
	s.resources.On("GetResourceServer", mock.Anything, californiaRSID).
		Return(&providers.ResourceServer{ID: californiaRSID}, nil)

	scopes, svcErr := resolveScopes(context.Background(), s.resources, []ResourcePermissions{
		{ResourceServerID: californiaRSID, Permissions: []string{"license"}}})

	s.Require().Nil(svcErr)
	s.Empty(scopes)
}

func (s *RoleUtilsTestSuite) TestResolveScopes_CarriesTheLookupError() {
	s.resources.On("GetResourceServer", mock.Anything, californiaRSID).
		Return(nil, &tidcommon.InternalServerError)

	_, svcErr := resolveScopes(context.Background(), s.resources, []ResourcePermissions{
		{ResourceServerID: californiaRSID, Permissions: []string{"license"}}})

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *RoleUtilsTestSuite) TestExpandPrincipal_AnEntityIsItself() {
	entityIDs, svcErr := expandPrincipal(context.Background(), s.groups, targetUserID, false)

	s.Require().Nil(svcErr)
	s.Equal([]string{targetUserID}, entityIDs)
}

func (s *RoleUtilsTestSuite) TestExpandPrincipal_ExpandsAGroupThroughTheGroupService() {
	stubGroupEntities(s.groups, targetGroupID, "member-1", "member-2")

	entityIDs, svcErr := expandPrincipal(context.Background(), s.groups, targetGroupID, true)

	s.Require().Nil(svcErr)
	s.Equal([]string{"member-1", "member-2"}, entityIDs)
}

func (s *RoleUtilsTestSuite) TestExpandPrincipal_CarriesTheExpansionError() {
	s.groups.On("GetTransitiveMemberEntities", mock.Anything, targetGroupID).
		Return(nil, &group.ErrorGroupNestingTooDeep)

	_, svcErr := expandPrincipal(context.Background(), s.groups, targetGroupID, true)

	s.Require().NotNil(svcErr)
	s.Equal(group.ErrorGroupNestingTooDeep.Code, svcErr.Code)
}
