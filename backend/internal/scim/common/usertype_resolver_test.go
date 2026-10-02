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

// TestResolveCoreUserType_Designated_ResolvesToThatType tests that the user type flagged as the
// SCIM core type is chosen among several configured user types.
func (suite *UsertypeResolverTestSuite) TestResolveCoreUserType_Designated_ResolvesToThatType() {
	t := suite.T()
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, 0, false).
		Return(&entitytype.EntityTypeListResponse{
			TotalResults: 2,
			Types: []entitytype.EntityTypeListItem{
				{Name: "Contractor"},
				{Name: "Employee", SystemAttributes: &entitytype.SystemAttributes{IsScimCoreType: true}},
			},
		}, (*tidcommon.ServiceError)(nil))

	name, svcErr := ResolveCoreUserType(context.Background(), mockET)

	require.Nil(t, svcErr)
	require.Equal(t, "Employee", name)
}

// TestResolveCoreUserTypeRules_BuildsRulesFromStoredMapping tests that rules come from the
// core type's stored SCIM mapping.
func (suite *UsertypeResolverTestSuite) TestResolveCoreUserTypeRules_BuildsRulesFromStoredMapping() {
	t := suite.T()
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	coreAttrs := &entitytype.SystemAttributes{
		IsScimCoreType: true,
		ScimMapping:    &entitytype.ScimMapping{AttributeMap: map[string]string{"login": "userName"}},
	}
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, 0, false).
		Return(&entitytype.EntityTypeListResponse{
			TotalResults: 1,
			Types:        []entitytype.EntityTypeListItem{{Name: "Employee", SystemAttributes: coreAttrs}},
		}, (*tidcommon.ServiceError)(nil))
	mockET.On("GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, "Employee").
		Return(&entitytype.EntityType{Name: "Employee", SystemAttributes: coreAttrs}, (*tidcommon.ServiceError)(nil))

	core, enterprise, svcErr := ResolveCoreUserTypeRules(context.Background(), mockET)

	require.Nil(t, svcErr)
	require.Equal(t, []CoreAttrRule{{Candidate: "login", SCIMField: fieldUserName, Kind: KindSimpleString}}, core)
	require.Empty(t, enterprise)
}

// TestResolveCoreUserTypeRules_NoMapping_ReturnsNoRules tests that a core type without a stored
// mapping has no rules.
func (suite *UsertypeResolverTestSuite) TestResolveCoreUserTypeRules_NoMapping_ReturnsNoRules() {
	t := suite.T()
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, 0, false).
		Return(&entitytype.EntityTypeListResponse{
			TotalResults: 1,
			Types:        []entitytype.EntityTypeListItem{{Name: "Employee"}},
		}, (*tidcommon.ServiceError)(nil))
	mockET.On("GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, "Employee").
		Return(&entitytype.EntityType{Name: "Employee"}, (*tidcommon.ServiceError)(nil))

	core, enterprise, svcErr := ResolveCoreUserTypeRules(context.Background(), mockET)

	require.Nil(t, svcErr)
	require.Empty(t, core)
	require.Empty(t, enterprise)
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

	name, svcErr := ResolveCoreUserType(context.Background(), mockET)

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

	name, svcErr := ResolveCoreUserType(context.Background(), mockET)

	require.NotNil(t, svcErr)
	require.Equal(t, ErrorMissingCustomSchema.Code, svcErr.Code)
	require.Empty(t, name)
}
