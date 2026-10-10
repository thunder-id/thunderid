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

// TestResolveUserTypeForSchemaURN_Found_ReturnsUserType tests that the user type with the given
// handle is returned.
func (suite *UsertypeResolverTestSuite) TestResolveUserTypeForSchemaURN_Found_ReturnsUserType() {
	t := suite.T()
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeByHandle", mock.Anything, entitytype.TypeCategoryUser, "employee").
		Return(&entitytype.EntityType{Handle: "employee"}, (*tidcommon.ServiceError)(nil))

	et, svcErr := ResolveUserTypeForSchemaURN(context.Background(), mockET, "employee")

	require.Nil(t, svcErr)
	require.NotNil(t, et)
	require.Equal(t, "employee", et.Handle)
}

// TestResolveUserTypeForSchemaURN_NotFound_ReturnsNilNil tests that an unknown handle yields no
// user type and no error.
func (suite *UsertypeResolverTestSuite) TestResolveUserTypeForSchemaURN_NotFound_ReturnsNilNil() {
	t := suite.T()
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeByHandle", mock.Anything, entitytype.TypeCategoryUser, "missing").
		Return((*entitytype.EntityType)(nil), &entitytype.ErrorEntityTypeNotFound)

	et, svcErr := ResolveUserTypeForSchemaURN(context.Background(), mockET, "missing")

	require.Nil(t, svcErr)
	require.Nil(t, et)
}

// TestResolveUserTypeForSchemaURN_ServerError_ReturnsInternalServerError tests that a server
// error from the user type service surfaces as an internal server error.
func (suite *UsertypeResolverTestSuite) TestResolveUserTypeForSchemaURN_ServerError_ReturnsInternalServerError() {
	t := suite.T()
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeByHandle", mock.Anything, entitytype.TypeCategoryUser, "employee").
		Return((*entitytype.EntityType)(nil), &tidcommon.ServiceError{Type: tidcommon.ServerErrorType})

	et, svcErr := ResolveUserTypeForSchemaURN(context.Background(), mockET, "employee")

	require.NotNil(t, svcErr)
	require.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)
	require.Nil(t, et)
}

// TestResolveUserTypeForSchemaURN_OtherClientError_ReturnsSchemaNotFound tests that any other
// client error maps to a schema-not-found error.
func (suite *UsertypeResolverTestSuite) TestResolveUserTypeForSchemaURN_OtherClientError_ReturnsSchemaNotFound() {
	t := suite.T()
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeByHandle", mock.Anything, entitytype.TypeCategoryUser, "employee").
		Return((*entitytype.EntityType)(nil), &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "X"})

	et, svcErr := ResolveUserTypeForSchemaURN(context.Background(), mockET, "employee")

	require.NotNil(t, svcErr)
	require.Equal(t, ErrorSchemaNotFound.Code, svcErr.Code)
	require.Nil(t, et)
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
				{Handle: "contractor"},
				{Handle: "employee", SystemAttributes: &entitytype.SystemAttributes{IsScimCoreType: true}},
			},
		}, (*tidcommon.ServiceError)(nil))

	handle, svcErr := ResolveCoreUserType(context.Background(), mockET)

	require.Nil(t, svcErr)
	require.Equal(t, "employee", handle)
}

// TestResolveCoreUserType_DesignatedOnLaterPage_ResolvesToThatType tests that the flagged type is
// found when it is not on the first page of the user type list.
func (suite *UsertypeResolverTestSuite) TestResolveCoreUserType_DesignatedOnLaterPage_ResolvesToThatType() {
	t := suite.T()
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, 0, false).
		Return(&entitytype.EntityTypeListResponse{
			TotalResults: 2,
			Types:        []entitytype.EntityTypeListItem{{Handle: "contractor"}},
		}, (*tidcommon.ServiceError)(nil))
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, 1, false).
		Return(&entitytype.EntityTypeListResponse{
			TotalResults: 2,
			Types: []entitytype.EntityTypeListItem{
				{Handle: "employee", SystemAttributes: &entitytype.SystemAttributes{IsScimCoreType: true}},
			},
		}, (*tidcommon.ServiceError)(nil))

	handle, svcErr := ResolveCoreUserType(context.Background(), mockET)

	require.Nil(t, svcErr)
	require.Equal(t, "employee", handle)
}

// TestResolveCoreUserType_ListServerError_ReturnsInternalServerError tests that a server error
// while listing user types surfaces as an internal server error.
func (suite *UsertypeResolverTestSuite) TestResolveCoreUserType_ListServerError_ReturnsInternalServerError() {
	t := suite.T()
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, 0, false).
		Return((*entitytype.EntityTypeListResponse)(nil), &tidcommon.ServiceError{Type: tidcommon.ServerErrorType})

	handle, svcErr := ResolveCoreUserType(context.Background(), mockET)

	require.NotNil(t, svcErr)
	require.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)
	require.Empty(t, handle)
}

// TestResolveCoreUserType_ListClientError_ReturnsMissingCustomSchema tests that a client error
// while listing user types means no core type can be determined.
func (suite *UsertypeResolverTestSuite) TestResolveCoreUserType_ListClientError_ReturnsMissingCustomSchema() {
	t := suite.T()
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, 0, false).
		Return((*entitytype.EntityTypeListResponse)(nil), &tidcommon.ServiceError{Type: tidcommon.ClientErrorType})

	handle, svcErr := ResolveCoreUserType(context.Background(), mockET)

	require.NotNil(t, svcErr)
	require.Equal(t, ErrorMissingCustomSchema.Code, svcErr.Code)
	require.Empty(t, handle)
}

// TestResolveCoreUserType_Unset_SingleUserType_FallsBack tests that with no user type flagged
// as the SCIM core type, it falls back to the sole configured user type.
func (suite *UsertypeResolverTestSuite) TestResolveCoreUserType_Unset_SingleUserType_FallsBack() {
	t := suite.T()
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, 0, false).
		Return(&entitytype.EntityTypeListResponse{
			TotalResults: 1,
			Types: []entitytype.EntityTypeListItem{
				{Handle: "employee", DisplayName: "Employee Staff", OUID: "ou-1"},
			},
		}, (*tidcommon.ServiceError)(nil))

	handle, svcErr := ResolveCoreUserType(context.Background(), mockET)

	require.Nil(t, svcErr)
	require.Equal(t, "employee", handle)
}

// TestResolveCoreUserType_Unset_MultipleUserTypes_ReturnsMissingCustomSchema tests that no
// flagged SCIM core type with 2+ configured user types is ambiguous and errors rather than
// silently guessing.
func (suite *UsertypeResolverTestSuite) TestResolveCoreUserType_Unset_MultipleUserTypes_ReturnsMissingCustomSchema() {
	t := suite.T()
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, 0, false).
		Return(&entitytype.EntityTypeListResponse{
			TotalResults: 2,
			Types: []entitytype.EntityTypeListItem{
				{Handle: "employee", OUID: "ou-1"},
				{Handle: "contractor", OUID: "ou-2"},
			},
		}, (*tidcommon.ServiceError)(nil))

	handle, svcErr := ResolveCoreUserType(context.Background(), mockET)

	require.NotNil(t, svcErr)
	require.Equal(t, ErrorMissingCustomSchema.Code, svcErr.Code)
	require.Empty(t, handle)
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
			Types:        []entitytype.EntityTypeListItem{{Handle: "employee", SystemAttributes: coreAttrs}},
		}, (*tidcommon.ServiceError)(nil))
	mockET.On("GetEntityTypeByHandle", mock.Anything, entitytype.TypeCategoryUser, "employee").
		Return(&entitytype.EntityType{Handle: "employee", SystemAttributes: coreAttrs}, (*tidcommon.ServiceError)(nil))

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
			Types:        []entitytype.EntityTypeListItem{{Handle: "employee"}},
		}, (*tidcommon.ServiceError)(nil))
	mockET.On("GetEntityTypeByHandle", mock.Anything, entitytype.TypeCategoryUser, "employee").
		Return(&entitytype.EntityType{Handle: "employee"}, (*tidcommon.ServiceError)(nil))

	core, enterprise, svcErr := ResolveCoreUserTypeRules(context.Background(), mockET)

	require.Nil(t, svcErr)
	require.Empty(t, core)
	require.Empty(t, enterprise)
}

// TestResolveCoreUserTypeRules_NoCoreType_ReturnsError tests that rules cannot be resolved when
// no core user type can be determined.
func (suite *UsertypeResolverTestSuite) TestResolveCoreUserTypeRules_NoCoreType_ReturnsError() {
	t := suite.T()
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, 0, false).
		Return(&entitytype.EntityTypeListResponse{TotalResults: 0}, (*tidcommon.ServiceError)(nil))

	core, enterprise, svcErr := ResolveCoreUserTypeRules(context.Background(), mockET)

	require.NotNil(t, svcErr)
	require.Equal(t, ErrorMissingCustomSchema.Code, svcErr.Code)
	require.Empty(t, core)
	require.Empty(t, enterprise)
}

// TestResolveCoreUserTypeRules_LoadError_ReturnsError tests that a failure loading the core
// user type surfaces as an error.
func (suite *UsertypeResolverTestSuite) TestResolveCoreUserTypeRules_LoadError_ReturnsError() {
	t := suite.T()
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, 0, false).
		Return(&entitytype.EntityTypeListResponse{
			TotalResults: 1,
			Types:        []entitytype.EntityTypeListItem{{Handle: "employee"}},
		}, (*tidcommon.ServiceError)(nil))
	mockET.On("GetEntityTypeByHandle", mock.Anything, entitytype.TypeCategoryUser, "employee").
		Return((*entitytype.EntityType)(nil), &tidcommon.ServiceError{Type: tidcommon.ServerErrorType})

	core, enterprise, svcErr := ResolveCoreUserTypeRules(context.Background(), mockET)

	require.NotNil(t, svcErr)
	require.Empty(t, core)
	require.Empty(t, enterprise)
}
