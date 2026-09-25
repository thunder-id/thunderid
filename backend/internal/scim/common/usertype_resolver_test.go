// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entitytype"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
)

// UsertypeResolverTestSuite groups the tests in usertype_resolver_test.go.
type UsertypeResolverTestSuite struct {
	suite.Suite
}

// TestUsertypeResolverTestSuite runs UsertypeResolverTestSuite.
func TestUsertypeResolverTestSuite(t *testing.T) {
	suite.Run(t, new(UsertypeResolverTestSuite))
}

// TestResolveCoreUserType_ConfiguredID_ResolvesToThatType tests that a configured
// CoreUserTypeID resolves directly via GetEntityType, without listing user types.
func (suite *UsertypeResolverTestSuite) TestResolveCoreUserType_ConfiguredID_ResolvesToThatType() {
	t := suite.T()
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityType", mock.Anything, entitytype.TypeCategoryUser, "type-employee", false).
		Return(&entitytype.EntityType{ID: "type-employee", Name: "Employee"}, (*tidcommon.ServiceError)(nil))

	name, svcErr := ResolveCoreUserType(context.Background(), mockET, "type-employee")

	require.Nil(t, svcErr)
	require.Equal(t, "Employee", name)
}

// TestResolveCoreUserType_ConfiguredID_NotFound_ReturnsError tests that a CoreUserTypeID
// pointing at a nonexistent user type surfaces an error instead of silently falling back.
func (suite *UsertypeResolverTestSuite) TestResolveCoreUserType_ConfiguredID_NotFound_ReturnsError() {
	t := suite.T()
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityType", mock.Anything, entitytype.TypeCategoryUser, "missing-id", false).
		Return((*entitytype.EntityType)(nil), &entitytype.ErrorUserTypeNotFound)

	name, svcErr := ResolveCoreUserType(context.Background(), mockET, "missing-id")

	require.NotNil(t, svcErr)
	require.Empty(t, name)
}

// TestResolveCoreUserType_Unset_SingleUserType_FallsBack tests that an empty CoreUserTypeID
// falls back to the sole configured user type, preserving today's implicit behavior.
func (suite *UsertypeResolverTestSuite) TestResolveCoreUserType_Unset_SingleUserType_FallsBack() {
	t := suite.T()
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, 0, false).
		Return(&entitytype.EntityTypeListResponse{
			TotalResults: 1,
			Types:        []entitytype.EntityTypeListItem{{Name: "Employee", OUID: "ou-1"}},
		}, (*tidcommon.ServiceError)(nil))

	name, svcErr := ResolveCoreUserType(context.Background(), mockET, "")

	require.Nil(t, svcErr)
	require.Equal(t, "Employee", name)
}

// TestResolveCoreUserType_Unset_MultipleUserTypes_ReturnsMissingCustomSchema tests that an
// empty CoreUserTypeID with 2+ configured user types is ambiguous and errors rather than
// silently guessing.
func (suite *UsertypeResolverTestSuite) TestResolveCoreUserType_Unset_MultipleUserTypes_ReturnsMissingCustomSchema() {
	t := suite.T()
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, 0, false).
		Return(&entitytype.EntityTypeListResponse{
			TotalResults: 2,
			Types: []entitytype.EntityTypeListItem{
				{Name: "Employee", OUID: "ou-1"},
				{Name: "Contractor", OUID: "ou-2"},
			},
		}, (*tidcommon.ServiceError)(nil))

	name, svcErr := ResolveCoreUserType(context.Background(), mockET, "")

	require.NotNil(t, svcErr)
	require.Equal(t, ErrorMissingCustomSchema.Code, svcErr.Code)
	require.Empty(t, name)
}
