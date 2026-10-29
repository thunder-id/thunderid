// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/security"
)

const testOwnerOU = "ou-owner"

type RoleSharingTestSuite struct {
	suite.Suite
	mockService *RoleServiceInterfaceMock
	ctx         context.Context
}

func TestRoleSharingTestSuite(t *testing.T) {
	suite.Run(t, new(RoleSharingTestSuite))
}

func (suite *RoleSharingTestSuite) SetupTest() {
	suite.mockService = NewRoleServiceInterfaceMock(suite.T())
	suite.ctx = context.Background()
}

// A role declares its assignments alone, open to every organization unit it reaches unless a
// policy says otherwise, so a policy need not mention them to allow them. A policy only switches
// them on or off.
func (suite *RoleSharingTestSuite) TestDeclaration_NamesRolesAndDeclaresAssignments() {
	decl := newRoleSharingDeclaration(suite.mockService)

	suite.Equal(RoleSharingType, decl.ResourceType())
	suite.Equal([]sharing.FieldDeclaration{{
		Key:          "assignments",
		Kind:         sharing.FieldReferenceSet,
		EditableOnly: true,
		Default:      &sharing.OverlayRule{Editable: true},
	}}, decl.Fields())
}

func (suite *RoleSharingTestSuite) TestDeclaration_OwningOUID() {
	decl := newRoleSharingDeclaration(suite.mockService)

	suite.mockService.EXPECT().GetRoleWithPermissions(mock.MatchedBy(security.IsRuntimeContext), "role1").
		Return(&RoleWithPermissions{ID: "role1", OUID: testOwnerOU}, nil).Once()
	ouID, svcErr := decl.OwningOUID(suite.ctx, "role1")
	suite.Nil(svcErr)
	suite.Equal(testOwnerOU, ouID)

	suite.mockService.EXPECT().GetRoleWithPermissions(mock.Anything, "missing").
		Return(nil, &ErrorRoleNotFound).Once()
	_, svcErr = decl.OwningOUID(suite.ctx, "missing")
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorRoleNotFound.Code, svcErr.Code)

	suite.mockService.EXPECT().GetRoleWithPermissions(mock.Anything, "broken").
		Return(nil, &tidcommon.InternalServerError).Once()
	_, svcErr = decl.OwningOUID(suite.ctx, "broken")
	suite.Require().NotNil(svcErr)
	suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}
