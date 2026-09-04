// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package users

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/entitytype"
	scim "github.com/thunder-id/thunderid/internal/scim/common"
	scimconfig "github.com/thunder-id/thunderid/internal/scim/config"
	"github.com/thunder-id/thunderid/internal/user"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/usermock"
)

const testUserTypeEmployee = "employee"
const testOUID = "ou-abc"

// TestGetUser_Success tests Get User for Success.
func TestGetUser_Success(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)

	service := newSCIMUsersService(
		mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

	internalUser := &user.User{
		ID:         "user-123",
		Type:       testUserTypeEmployee,
		Attributes: []byte(`{"given_name": "John"}`),
	}
	mockUserService.On("GetUser", mock.Anything, "user-123", false).Return(internalUser, (*tidcommon.ServiceError)(nil))
	mockUserTypeService.On(

		"GetAttributes", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
		entitytype.AttributeFilter{AllowCredential: true, AllowNonCredential: false, RequiredOnly: false},
	).Return([]entitytype.AttributeInfo{{Attribute: "password"}}, (*tidcommon.ServiceError)(nil))

	scimUser, err := service.GetUser(context.Background(), "user-123", testBaseURL)

	require.Nil(t, err)
	require.NotNil(t, scimUser)
	require.Equal(t, "user-123", scimUser.ID)
	require.Contains(t, scimUser.Schemas, "urn:thunderid:params:scim:schemas:employee:2.0:User")
}

// TestGetUser_CredentialKeyLookupFailure_DoesNotLeakAttributes tests Get User for Credential Key Lookup
// Failure Does Not Leak Attributes.
func TestGetUser_CredentialKeyLookupFailure_DoesNotLeakAttributes(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)

	service := newSCIMUsersService(
		mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

	internalUser := &user.User{
		ID:         "user-123",
		Type:       testUserTypeEmployee,
		Attributes: []byte(`{"given_name": "John", "password": "secret"}`),
	}
	mockUserService.On("GetUser", mock.Anything, "user-123", false).Return(internalUser, (*tidcommon.ServiceError)(nil))
	mockUserTypeService.On(
		"GetAttributes", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
		entitytype.AttributeFilter{AllowCredential: true, AllowNonCredential: false, RequiredOnly: false},
	).Return(nil, &tidcommon.ServiceError{Code: "SVC-500", Type: tidcommon.ServerErrorType})

	scimUser, err := service.GetUser(context.Background(), "user-123", testBaseURL)

	require.Nil(t, scimUser)
	require.NotNil(t, err)
	require.Equal(t, scim.ErrorInternalServer.Code, err.Code)
}

// TestGetUser_NotFound tests Get User for Not Found.
func TestGetUser_NotFound(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)

	service := newSCIMUsersService(
		mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

	mockUserService.On("GetUser", mock.Anything, "user-123", false).Return((*user.User)(nil), &user.ErrorUserNotFound)

	scimUser, err := service.GetUser(context.Background(), "user-123", testBaseURL)

	require.NotNil(t, err)
	require.Equal(t, scim.ErrorUserNotFound.Code, err.Code)
	require.Nil(t, scimUser)
}

// TestDeleteUser_Success tests Delete User for Success.
func TestDeleteUser_Success(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)

	service := newSCIMUsersService(
		mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

	mockUserService.On("DeleteUser", mock.Anything, "user-123").Return((*tidcommon.ServiceError)(nil))

	err := service.DeleteUser(context.Background(), "user-123", "")

	require.Nil(t, err)
}

// TestDeleteUser_NotFound tests Delete User for Not Found.
func TestDeleteUser_NotFound(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)

	service := newSCIMUsersService(
		mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

	mockUserService.On("DeleteUser", mock.Anything, "user-123").Return(&user.ErrorUserNotFound)

	err := service.DeleteUser(context.Background(), "user-123", "")

	require.NotNil(t, err)
	require.Equal(t, scim.ErrorUserNotFound.Code, err.Code)
}

// TestDeleteUser_MutabilityViolation_MapsToSCIM tests Delete User for Mutability Violation Maps To SCIM.
func TestDeleteUser_MutabilityViolation_MapsToSCIM(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)

	service := newSCIMUsersService(
		mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

	mockUserService.On("DeleteUser", mock.Anything, "user-123").
		Return(&user.ErrorCannotModifyDeclarativeResource)

	err := service.DeleteUser(context.Background(), "user-123", "")

	require.NotNil(t, err)
	require.Equal(t, scim.ErrorMutabilityViolation.Code, err.Code)
}

// TestGetUser_UniquenessConflict_MapsToSCIM tests Get User for Uniqueness Conflict Maps To SCIM.
func TestGetUser_UniquenessConflict_MapsToSCIM(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)

	service := newSCIMUsersService(
		mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

	mockUserService.On("GetUser", mock.Anything, "user-123", false).
		Return((*user.User)(nil), &user.ErrorAttributeConflict)

	scimUser, err := service.GetUser(context.Background(), "user-123", testBaseURL)

	require.NotNil(t, err)
	require.Equal(t, scim.ErrorUniquenessConflict.Code, err.Code)
	require.Nil(t, scimUser)
}

// TestGetUser_SchemaValidationError_MapsToSCIM tests Get User for Schema Validation Error Maps To SCIM.
func TestGetUser_SchemaValidationError_MapsToSCIM(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)

	service := newSCIMUsersService(
		mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

	mockUserService.On("GetUser", mock.Anything, "user-123", false).
		Return((*user.User)(nil), &user.ErrorSchemaValidationFailed)

	scimUser, err := service.GetUser(context.Background(), "user-123", testBaseURL)

	require.NotNil(t, err)
	require.Equal(t, scim.ErrorSchemaValidationFailed.Code, err.Code)
	require.Nil(t, scimUser)
}

// TestListUsers_Success tests List Users for Success.
func TestListUsers_Success(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)

	service := newSCIMUsersService(
		mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

	internalUser := user.User{
		ID:         "user-1",
		Type:       testUserTypeEmployee,
		Attributes: []byte(`{"given_name":"Alice"}`),
	}
	mockUserService.On("GetUserList", mock.Anything, 20, 0, (map[string]interface{})(nil), false).
		Return(&user.UserListResponse{
			TotalResults: 1,
			Users:        []user.User{internalUser},
		}, (*tidcommon.ServiceError)(nil))
	mockUserTypeService.On(
		"GetAttributes", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
		entitytype.AttributeFilter{AllowCredential: true, AllowNonCredential: false, RequiredOnly: false},
	).Return([]entitytype.AttributeInfo{}, (*tidcommon.ServiceError)(nil))

	resp, err := service.ListUsers(context.Background(), 1, 20, nil, testBaseURL)

	require.Nil(t, err)
	require.Equal(t, 1, resp.TotalResults)
	require.Len(t, resp.Resources, 1)
	require.Equal(t, "user-1", resp.Resources[0].ID)
}

// TestListUsers_UnresolvableUserType_OmitsUserButReturnsRest guards against a single user
// with a stale/unregistered entity type (e.g. leftover declarative fixture data) taking down
// an entire unfiltered listing. The user with the bad type is silently omitted; other users
// are still returned.
// TestListUsers_UnresolvableUserType_OmitsUserButReturnsRest tests List Users for Unresolvable User Type
// Omits User But Returns Rest.
func TestListUsers_UnresolvableUserType_OmitsUserButReturnsRest(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)

	service := newSCIMUsersService(
		mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

	const testUnknownUserType = "ghost-type"
	ghostUser := user.User{
		ID:         "user-ghost",
		Type:       testUnknownUserType,
		Attributes: []byte(`{"given_name":"Ghost"}`),
	}
	goodUser := user.User{
		ID:         "user-1",
		Type:       testUserTypeEmployee,
		Attributes: []byte(`{"given_name":"Alice"}`),
	}
	mockUserService.On("GetUserList", mock.Anything, 20, 0, (map[string]interface{})(nil), false).
		Return(&user.UserListResponse{
			TotalResults: 2,
			Users:        []user.User{ghostUser, goodUser},
		}, (*tidcommon.ServiceError)(nil))
	mockUserTypeService.On(
		"GetAttributes", mock.Anything, entitytype.TypeCategoryUser, testUnknownUserType,
		entitytype.AttributeFilter{AllowCredential: true, AllowNonCredential: false, RequiredOnly: false},
	).Return(nil, &tidcommon.ServiceError{Code: "USRS-1002", Type: tidcommon.ClientErrorType})
	mockUserTypeService.On(
		"GetAttributes", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
		entitytype.AttributeFilter{AllowCredential: true, AllowNonCredential: false, RequiredOnly: false},
	).Return([]entitytype.AttributeInfo{}, (*tidcommon.ServiceError)(nil))

	resp, err := service.ListUsers(context.Background(), 1, 20, nil, testBaseURL)

	require.Nil(t, err)
	require.Len(t, resp.Resources, 1)
	require.Equal(t, "user-1", resp.Resources[0].ID)
}

// TestListUsers_ServiceError tests List Users for Service Error.
func TestListUsers_ServiceError(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)

	service := newSCIMUsersService(
		mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

	mockUserService.On("GetUserList", mock.Anything, 20, 0, (map[string]interface{})(nil), false).
		Return((*user.UserListResponse)(nil), &user.ErrorUserNotFound)

	resp, err := service.ListUsers(context.Background(), 1, 20, nil, testBaseURL)

	require.NotNil(t, err)
	require.Equal(t, scim.ErrorUserNotFound.Code, err.Code)
	require.Empty(t, resp.Resources)
}

// TestListUsers_ExplicitZeroCountReturnsNoResources tests List Users for Explicit Zero Count Returns No Resources.
func TestListUsers_ExplicitZeroCountReturnsNoResources(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)

	service := newSCIMUsersService(
		mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

	mockUserService.On("GetUserList",
		mock.Anything, 1, 0, (map[string]interface{})(nil), false).
		Return(&user.UserListResponse{TotalResults: 5, Users: []user.User{{ID: "user-1", Type: "employee"}}},
			(*tidcommon.ServiceError)(nil))

	resp, err := service.ListUsers(context.Background(), 0, 0, nil, testBaseURL)

	require.Nil(t, err)
	require.Equal(t, 5, resp.TotalResults)
	require.Empty(t, resp.Resources)
	require.Equal(t, 0, resp.ItemsPerPage)
}

// TestMapUserServiceErrorToSCIM_AllCodes tests Map User Service Error To SCIM for All Codes.
func TestMapUserServiceErrorToSCIM_AllCodes(t *testing.T) {
	tests := []struct {
		input    *tidcommon.ServiceError
		wantCode string
	}{
		{&user.ErrorUserNotFound, scim.ErrorUserNotFound.Code},
		{&user.ErrorAttributeConflict, scim.ErrorUniquenessConflict.Code},
		{&user.ErrorSchemaValidationFailed, scim.ErrorSchemaValidationFailed.Code},
		{&user.ErrorEntityTypeNotFound, scim.ErrorUnknownUserType.Code},
		{&user.ErrorCannotModifyDeclarativeResource, scim.ErrorMutabilityViolation.Code},
		{&tidcommon.ErrorUnauthorized, tidcommon.ErrorUnauthorized.Code},
		// Unknown client error → invalidRequestBody
		{&user.ErrorInvalidRequestFormat, scim.ErrorInvalidRequestBody.Code},
	}

	for _, tc := range tests {
		t.Run(tc.input.Code, func(t *testing.T) {
			got := mapUserServiceErrorToSCIM(tc.input)
			require.NotNil(t, got)
			require.Equal(t, tc.wantCode, got.Code)
		})
	}
}

// TestMapUserServiceErrorToSCIM_ServerError_MapsToInternalServer tests Map User Service Error To SCIM for
// Server Error Maps To Internal Server.
func TestMapUserServiceErrorToSCIM_ServerError_MapsToInternalServer(t *testing.T) {
	serverErr := &tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		Code: "SRV-9999",
	}
	got := mapUserServiceErrorToSCIM(serverErr)
	require.NotNil(t, got)
	require.Equal(t, tidcommon.InternalServerError.Code, got.Code)
}

// TestMapUserServiceErrorToSCIM_Nil_ReturnsNil tests Map User Service Error To SCIM for Nil Returns Nil.
func TestMapUserServiceErrorToSCIM_Nil_ReturnsNil(t *testing.T) {
	require.Nil(t, mapUserServiceErrorToSCIM(nil))
}

// scim.ResolveUserTypeNameForSchemaURN is called with the user type name extracted
// from the schema URN. It pages through GetEntityTypeList and matches by name
// (case-insensitive). The tests below set up the minimal mock chain:
//   GetEntityTypeList  →  list containing the type  →  GetEntityTypeByName
//   GetEntityTypeByName → EntityType with OUID
//   userService.CreateUser / UpdateUser → created/updated User

// makeEntityTypeListPage handles make entity type list page.
func makeEntityTypeListPage() *entitytype.EntityTypeListResponse {
	return &entitytype.EntityTypeListResponse{
		TotalResults: 1,
		Types: []entitytype.EntityTypeListItem{
			{Name: testUserTypeEmployee, OUID: testOUID},
		},
	}
}

// --- CreateUser ---

// TestCreateUser_Success tests Create User for Success.
func TestCreateUser_Success(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(
		mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

	payload := &SCIMUserPayload{
		UserTypeName:   testUserTypeEmployee,
		ExtensionURN:   "urn:thunderid:params:scim:schemas:employee:2.0:User",
		ExtensionAttrs: map[string]json.RawMessage{"given_name": json.RawMessage(`"Alice"`)},
	}
	createdUser := &user.User{
		ID:         "user-new",
		Type:       testUserTypeEmployee,
		OUID:       testOUID,
		Attributes: []byte(`{"given_name":"Alice"}`),
	}

	// scim.ResolveUserTypeNameForSchemaURN pages GetEntityTypeList
	mockUserTypeService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))

	// GetEntityTypeByName after resolution
	mockUserTypeService.On(
		"GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
	).Return(&entitytype.EntityType{Name: testUserTypeEmployee, OUID: testOUID}, (*tidcommon.ServiceError)(nil))

	mockUserService.On("CreateUser", mock.Anything, mock.MatchedBy(func(u *user.User) bool {
		return u.Type == testUserTypeEmployee && u.OUID == testOUID
	})).Return(createdUser, (*tidcommon.ServiceError)(nil))
	mockUserTypeService.On(
		"GetAttributes", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
		entitytype.AttributeFilter{AllowCredential: true, AllowNonCredential: false, RequiredOnly: false},
	).Return([]entitytype.AttributeInfo{}, (*tidcommon.ServiceError)(nil))

	scimUser, err := service.CreateUser(context.Background(), payload, testBaseURL)

	require.Nil(t, err)
	require.NotNil(t, scimUser)
	require.Equal(t, "user-new", scimUser.ID)
	require.Contains(t, scimUser.Schemas, "urn:thunderid:params:scim:schemas:employee:2.0:User")
}

// TestCreateUser_MissingRequiredAttribute_ReturnsSchemaValidationError tests Create User for Missing Required
// Attribute Returns Schema Validation Error.
func TestCreateUser_MissingRequiredAttribute_ReturnsSchemaValidationError(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(
		mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

	payload := &SCIMUserPayload{
		UserTypeName:   testUserTypeEmployee,
		ExtensionURN:   "urn:thunderid:params:scim:schemas:employee:2.0:User",
		ExtensionAttrs: map[string]json.RawMessage{"given_name": json.RawMessage(`"Alice"`)},
	}

	mockUserTypeService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))
	mockUserTypeService.On(
		"GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
	).Return(&entitytype.EntityType{
		Name: testUserTypeEmployee, OUID: testOUID,
		Schema: json.RawMessage(`{"department":{"required":true}}`),
	}, (*tidcommon.ServiceError)(nil))

	scimUser, err := service.CreateUser(context.Background(), payload, testBaseURL)

	require.NotNil(t, err)
	require.Equal(t, scim.ErrorSchemaValidationFailed.Code, err.Code)
	require.Contains(t, err.ErrorDescription.DefaultValue, "department")
	require.Nil(t, scimUser)
}

// TestCreateUser_UndeclaredAttribute_ReturnsSchemaValidationError tests Create User for Undeclared Attribute
// Returns Schema Validation Error.
func TestCreateUser_UndeclaredAttribute_ReturnsSchemaValidationError(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(
		mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

	payload := &SCIMUserPayload{
		UserTypeName: testUserTypeEmployee,
		ExtensionURN: "urn:thunderid:params:scim:schemas:employee:2.0:User",
		ExtensionAttrs: map[string]json.RawMessage{
			"department": json.RawMessage(`"Eng"`),
			"undeclared": json.RawMessage(`"bad"`),
		},
	}

	mockUserTypeService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))
	mockUserTypeService.On(
		"GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
	).Return(&entitytype.EntityType{
		Name: testUserTypeEmployee, OUID: testOUID,
		Schema: json.RawMessage(`{"department":{"required":true}}`),
	}, (*tidcommon.ServiceError)(nil))

	scimUser, err := service.CreateUser(context.Background(), payload, testBaseURL)

	require.NotNil(t, err)
	require.Equal(t, scim.ErrorSchemaValidationFailed.Code, err.Code)
	require.Contains(t, err.ErrorDescription.DefaultValue, "undeclared")
	require.Nil(t, scimUser)
}

// TestCreateUser_MalformedSchemaJSON_ReturnsInternalServerError tests Create User for Malformed Schema JSON
// Returns Internal Server Error.
func TestCreateUser_MalformedSchemaJSON_ReturnsInternalServerError(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(
		mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

	payload := &SCIMUserPayload{
		UserTypeName: testUserTypeEmployee,
		ExtensionURN: "urn:thunderid:params:scim:schemas:employee:2.0:User",
		CoreAttrs: map[string]json.RawMessage{
			"userName": json.RawMessage(`"alice"`),
		},
	}

	mockUserTypeService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))
	mockUserTypeService.On(
		"GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
	).Return(&entitytype.EntityType{
		Name: testUserTypeEmployee, OUID: testOUID,
		Schema: json.RawMessage(`invalid json`),
	}, (*tidcommon.ServiceError)(nil))

	scimUser, err := service.CreateUser(context.Background(), payload, testBaseURL)

	require.NotNil(t, err)
	require.Equal(t, scim.ErrorInternalServer.Code, err.Code)
	require.Nil(t, scimUser)
}

// TestReplaceUser_MalformedSchemaJSON_ReturnsInternalServerError tests Replace User for Malformed Schema JSON
// Returns Internal Server Error.
func TestReplaceUser_MalformedSchemaJSON_ReturnsInternalServerError(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(
		mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

	payload := &SCIMUserPayload{
		UserTypeName: testUserTypeEmployee,
		ExtensionURN: "urn:thunderid:params:scim:schemas:employee:2.0:User",
		CoreAttrs: map[string]json.RawMessage{
			"userName": json.RawMessage(`"alice"`),
		},
	}

	mockUserTypeService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))
	mockUserTypeService.On(
		"GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
	).Return(&entitytype.EntityType{
		Name: testUserTypeEmployee, OUID: testOUID,
		Schema: json.RawMessage(`invalid json`),
	}, (*tidcommon.ServiceError)(nil))
	mockUserService.On("GetUser", mock.Anything, "user-123", false).
		Return(&user.User{ID: "user-123", Type: testUserTypeEmployee}, (*tidcommon.ServiceError)(nil))

	scimUser, err := service.ReplaceUser(context.Background(), "user-123", payload, "", testBaseURL, false)

	require.NotNil(t, err)
	require.Equal(t, scim.ErrorInternalServer.Code, err.Code)
	require.Nil(t, scimUser)
}

// TestCreateUser_SchemaAttributeErrors tests Create User for Schema Attribute Errors.
func TestCreateUser_SchemaAttributeErrors(t *testing.T) {
	tests := []struct {
		name             string
		payload          *SCIMUserPayload
		schema           string
		wantErrorCode    string
		wantDescContains string
	}{
		{
			name: "ConflictingCoreAndCustomValue_ReturnsConflictError",
			payload: &SCIMUserPayload{
				UserTypeName:   testUserTypeEmployee,
				ExtensionURN:   "urn:thunderid:params:scim:schemas:employee:2.0:User",
				CoreAttrs:      map[string]json.RawMessage{"userName": json.RawMessage(`"alice"`)},
				ExtensionAttrs: map[string]json.RawMessage{"username": json.RawMessage(`"bob"`)},
			},
			schema:           `{"username":{"type":"string"}}`,
			wantErrorCode:    scim.ErrorConflictingAttributeValue.Code,
			wantDescContains: "username",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUserService := usermock.NewUserServiceInterfaceMock(t)
			mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
			service := newSCIMUsersService(
				mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

			mockUserTypeService.On(
				"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
			).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))
			mockUserTypeService.On(
				"GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
			).Return(&entitytype.EntityType{
				Name: testUserTypeEmployee, OUID: testOUID,
				Schema: json.RawMessage(tt.schema),
			}, (*tidcommon.ServiceError)(nil))

			scimUser, err := service.CreateUser(context.Background(), tt.payload, testBaseURL)

			require.NotNil(t, err)
			require.Equal(t, tt.wantErrorCode, err.Code)
			require.Contains(t, err.ErrorDescription.DefaultValue, tt.wantDescContains)
			require.Nil(t, scimUser)
		})
	}
}

// TestCreateUser_MatchingCoreAndCustomValue_Succeeds tests Create User for Matching Core And Custom Value Succeeds.
func TestCreateUser_MatchingCoreAndCustomValue_Succeeds(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(
		mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

	payload := &SCIMUserPayload{
		UserTypeName:   testUserTypeEmployee,
		ExtensionURN:   "urn:thunderid:params:scim:schemas:employee:2.0:User",
		CoreAttrs:      map[string]json.RawMessage{"userName": json.RawMessage(`"alice"`)},
		ExtensionAttrs: map[string]json.RawMessage{"username": json.RawMessage(`"alice"`)},
	}
	createdUser := &user.User{
		ID:         "user-new",
		Type:       testUserTypeEmployee,
		OUID:       testOUID,
		Attributes: []byte(`{"username":"alice"}`),
	}

	mockUserTypeService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))
	mockUserTypeService.On(
		"GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
	).Return(&entitytype.EntityType{
		Name: testUserTypeEmployee, OUID: testOUID,
		Schema: json.RawMessage(`{"username":{"type":"string"}}`),
	}, (*tidcommon.ServiceError)(nil))
	mockUserService.On("CreateUser", mock.Anything, mock.MatchedBy(func(u *user.User) bool {
		return u.Type == testUserTypeEmployee && u.OUID == testOUID
	})).Return(createdUser, (*tidcommon.ServiceError)(nil))
	mockUserTypeService.On(
		"GetAttributes", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
		entitytype.AttributeFilter{AllowCredential: true, AllowNonCredential: false, RequiredOnly: false},
	).Return([]entitytype.AttributeInfo{}, (*tidcommon.ServiceError)(nil))

	scimUser, err := service.CreateUser(context.Background(), payload, testBaseURL)

	require.Nil(t, err)
	require.NotNil(t, scimUser)
	require.Equal(t, "user-new", scimUser.ID)
}

// TestCreateUser_CoreOnly_SingleUserType_DefaultsToType tests Create User for Core Only Single User Type
// Defaults To Type.
func TestCreateUser_CoreOnly_SingleUserType_DefaultsToType(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(
		mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

	// No UserTypeName/ExtensionURN: the request carried only core attributes.
	payload := &SCIMUserPayload{
		CoreAttrs:      map[string]json.RawMessage{"userName": json.RawMessage(`"alice"`)},
		ExtensionAttrs: map[string]json.RawMessage{},
	}
	createdUser := &user.User{
		ID:         "user-new",
		Type:       testUserTypeEmployee,
		OUID:       testOUID,
		Attributes: []byte(`{"username":"alice"}`),
	}

	// scim.ResolveDefaultUserTypeName pages GetEntityTypeList and, finding
	// exactly one configured user type, defaults to it.
	mockUserTypeService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))
	mockUserTypeService.On(
		"GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
	).Return(&entitytype.EntityType{
		Name: testUserTypeEmployee, OUID: testOUID,
		Schema: json.RawMessage(`{"username":{"type":"string"}}`),
	}, (*tidcommon.ServiceError)(nil))
	mockUserService.On("CreateUser", mock.Anything, mock.MatchedBy(func(u *user.User) bool {
		return u.Type == testUserTypeEmployee && u.OUID == testOUID
	})).Return(createdUser, (*tidcommon.ServiceError)(nil))
	mockUserTypeService.On(
		"GetAttributes", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
		entitytype.AttributeFilter{AllowCredential: true, AllowNonCredential: false, RequiredOnly: false},
	).Return([]entitytype.AttributeInfo{}, (*tidcommon.ServiceError)(nil))

	scimUser, err := service.CreateUser(context.Background(), payload, testBaseURL)

	require.Nil(t, err)
	require.NotNil(t, scimUser)
	require.Equal(t, "user-new", scimUser.ID)
	require.Contains(t, scimUser.Schemas, "urn:thunderid:params:scim:schemas:employee:2.0:User")
}

// TestCreateUser_CoreOnly_MultipleUserTypes_ReturnsMissingCustomSchema tests Create User for Core Only
// Multiple User Types Returns Missing Custom Schema.
func TestCreateUser_CoreOnly_MultipleUserTypes_ReturnsMissingCustomSchema(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(
		mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

	payload := &SCIMUserPayload{
		CoreAttrs: map[string]json.RawMessage{"userName": json.RawMessage(`"alice"`)},
	}

	// Two configured user types: which one to default to is ambiguous.
	mockUserTypeService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(&entitytype.EntityTypeListResponse{
		TotalResults: 2,
		Types: []entitytype.EntityTypeListItem{
			{Name: testUserTypeEmployee, OUID: testOUID},
			{Name: "person", OUID: "ou-def"},
		},
	}, (*tidcommon.ServiceError)(nil))

	scimUser, err := service.CreateUser(context.Background(), payload, testBaseURL)

	require.NotNil(t, err)
	require.Equal(t, scim.ErrorMissingCustomSchema.Code, err.Code)
	require.Nil(t, scimUser)
}

// TestCreateUser_CoreOnly_ConfiguredCoreUserTypeID_ResolvesUnambiguously tests that a configured
// CoreUserTypeID resolves the target user type directly even with 2+ user types configured,
// where the sole-user-type fallback would otherwise be ambiguous.
func TestCreateUser_CoreOnly_ConfiguredCoreUserTypeID_ResolvesUnambiguously(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(
		mockUserService, mockUserTypeService,
		scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true, CoreUserTypeID: "type-employee-id"})

	payload := &SCIMUserPayload{
		CoreAttrs:      map[string]json.RawMessage{"userName": json.RawMessage(`"alice"`)},
		ExtensionAttrs: map[string]json.RawMessage{},
	}
	createdUser := &user.User{
		ID:         "user-new",
		Type:       testUserTypeEmployee,
		OUID:       testOUID,
		Attributes: []byte(`{"username":"alice"}`),
	}

	mockUserTypeService.On(
		"GetEntityType", mock.Anything, entitytype.TypeCategoryUser, "type-employee-id", false,
	).Return(&entitytype.EntityType{
		ID: "type-employee-id", Name: testUserTypeEmployee, OUID: testOUID,
		Schema: json.RawMessage(`{"username":{"type":"string"}}`),
	}, (*tidcommon.ServiceError)(nil))
	mockUserTypeService.On(
		"GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
	).Return(&entitytype.EntityType{
		Name: testUserTypeEmployee, OUID: testOUID,
		Schema: json.RawMessage(`{"username":{"type":"string"}}`),
	}, (*tidcommon.ServiceError)(nil))
	mockUserService.On("CreateUser", mock.Anything, mock.MatchedBy(func(u *user.User) bool {
		return u.Type == testUserTypeEmployee && u.OUID == testOUID
	})).Return(createdUser, (*tidcommon.ServiceError)(nil))
	mockUserTypeService.On(
		"GetAttributes", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
		entitytype.AttributeFilter{AllowCredential: true, AllowNonCredential: false, RequiredOnly: false},
	).Return([]entitytype.AttributeInfo{}, (*tidcommon.ServiceError)(nil))

	scimUser, err := service.CreateUser(context.Background(), payload, testBaseURL)

	require.Nil(t, err)
	require.NotNil(t, scimUser)
	require.Equal(t, "user-new", scimUser.ID)
}

// TestCreateUser_CoreOnly_ZeroUserTypes_ReturnsMissingCustomSchema tests Create User for Core Only Zero User
// Types Returns Missing Custom Schema.
func TestCreateUser_CoreOnly_ZeroUserTypes_ReturnsMissingCustomSchema(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(
		mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

	payload := &SCIMUserPayload{
		CoreAttrs: map[string]json.RawMessage{"userName": json.RawMessage(`"alice"`)},
	}

	mockUserTypeService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(&entitytype.EntityTypeListResponse{TotalResults: 0, Types: []entitytype.EntityTypeListItem{}},
		(*tidcommon.ServiceError)(nil))

	scimUser, err := service.CreateUser(context.Background(), payload, testBaseURL)

	require.NotNil(t, err)
	require.Equal(t, scim.ErrorMissingCustomSchema.Code, err.Code)
	require.Nil(t, scimUser)
}

// TestCreateUser_EntityTypeNotFound_ReturnsUnknownUserType tests Create User for Entity Type Not Found
// Returns Unknown User Type.
func TestCreateUser_EntityTypeNotFound_ReturnsUnknownUserType(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(
		mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

	payload := &SCIMUserPayload{
		UserTypeName: "ghost",
		ExtensionURN: "urn:thunderid:params:scim:schemas:ghost:2.0:User",
	}

	// resolver finds no match — returns empty list
	mockUserTypeService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(&entitytype.EntityTypeListResponse{TotalResults: 0, Types: []entitytype.EntityTypeListItem{}},
		(*tidcommon.ServiceError)(nil))

	scimUser, err := service.CreateUser(context.Background(), payload, testBaseURL)

	require.NotNil(t, err)
	require.Equal(t, scim.ErrorUnknownUserType.Code, err.Code)
	require.Nil(t, scimUser)
}

// TestCreateUser_EntityTypeListError_ReturnsUnknownUserType tests Create User for Entity Type List Error
// Returns Unknown User Type.
func TestCreateUser_EntityTypeListError_ReturnsUnknownUserType(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(
		mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

	payload := &SCIMUserPayload{
		UserTypeName: testUserTypeEmployee,
		ExtensionURN: "urn:thunderid:params:scim:schemas:employee:2.0:User",
	}

	mockUserTypeService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return((*entitytype.EntityTypeListResponse)(nil), &tidcommon.ErrorUnauthorized)

	scimUser, err := service.CreateUser(context.Background(), payload, testBaseURL)

	require.NotNil(t, err)
	require.Equal(t, scim.ErrorUnknownUserType.Code, err.Code)
	require.Nil(t, scimUser)
}

// TestCreateUser_GetEntityTypeByNameError_ReturnsUnknownUserType tests Create User for Get Entity Type By
// Name Error Returns Unknown User Type.
func TestCreateUser_GetEntityTypeByNameError_ReturnsUnknownUserType(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(
		mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

	payload := &SCIMUserPayload{
		UserTypeName: testUserTypeEmployee,
		ExtensionURN: "urn:thunderid:params:scim:schemas:employee:2.0:User",
	}

	mockUserTypeService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))

	mockUserTypeService.On(
		"GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
	).Return((*entitytype.EntityType)(nil), &user.ErrorEntityTypeNotFound)

	scimUser, err := service.CreateUser(context.Background(), payload, testBaseURL)

	require.NotNil(t, err)
	require.Equal(t, scim.ErrorUnknownUserType.Code, err.Code)
	require.Nil(t, scimUser)
}

// TestCreateUser_Error_Scenarios tests Create User for Error Scenarios.
func TestCreateUser_Error_Scenarios(t *testing.T) {
	testCases := []struct {
		name          string
		mockError     *tidcommon.ServiceError
		expectedError *tidcommon.ServiceError
	}{
		{
			name:          "UserServiceConflict_ReturnsUniqueness",
			mockError:     &user.ErrorAttributeConflict,
			expectedError: &scim.ErrorUniquenessConflict,
		},
		{
			name:          "SchemaValidationFailed_ReturnsSCIMError",
			mockError:     &user.ErrorSchemaValidationFailed,
			expectedError: &scim.ErrorSchemaValidationFailed,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockUserService := usermock.NewUserServiceInterfaceMock(t)
			mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
			service := newSCIMUsersService(
				mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

			payload := &SCIMUserPayload{
				UserTypeName:   testUserTypeEmployee,
				ExtensionURN:   "urn:thunderid:params:scim:schemas:employee:2.0:User",
				ExtensionAttrs: map[string]json.RawMessage{},
			}

			mockUserTypeService.On(
				"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
			).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))
			mockUserTypeService.On(
				"GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
			).Return(&entitytype.EntityType{Name: testUserTypeEmployee, OUID: testOUID}, (*tidcommon.ServiceError)(nil))
			mockUserService.On("CreateUser", mock.Anything, mock.Anything).
				Return((*user.User)(nil), tc.mockError)

			scimUser, err := service.CreateUser(context.Background(), payload, testBaseURL)

			require.NotNil(t, err)
			require.Equal(t, tc.expectedError.Code, err.Code)
			require.Nil(t, scimUser)
		})
	}
}

// --- ReplaceUser ---

// TestReplaceUser_Success tests Replace User for Success.
func TestReplaceUser_Success(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(
		mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

	payload := &SCIMUserPayload{
		UserTypeName:   testUserTypeEmployee,
		ExtensionURN:   "urn:thunderid:params:scim:schemas:employee:2.0:User",
		ExtensionAttrs: map[string]json.RawMessage{"given_name": json.RawMessage(`"Charlie"`)},
	}
	updatedUser := &user.User{
		ID:         "user-123",
		Type:       testUserTypeEmployee,
		OUID:       testOUID,
		Attributes: []byte(`{"given_name":"Charlie"}`),
	}

	mockUserTypeService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))
	mockUserTypeService.On(
		"GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
	).Return(&entitytype.EntityType{Name: testUserTypeEmployee, OUID: testOUID}, (*tidcommon.ServiceError)(nil))
	mockUserService.On("GetUser", mock.Anything, "user-123", false).
		Return(&user.User{ID: "user-123", Type: testUserTypeEmployee}, (*tidcommon.ServiceError)(nil))
	mockUserService.On("UpdateUser", mock.Anything, "user-123", mock.MatchedBy(func(u *user.User) bool {
		return u.ID == "user-123" && u.Type == testUserTypeEmployee
	})).Return(updatedUser, (*tidcommon.ServiceError)(nil))
	mockUserTypeService.On(
		"GetAttributes", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
		entitytype.AttributeFilter{AllowCredential: true, AllowNonCredential: false, RequiredOnly: false},
	).Return([]entitytype.AttributeInfo{}, (*tidcommon.ServiceError)(nil))

	scimUser, err := service.ReplaceUser(context.Background(), "user-123", payload, "", testBaseURL, false)

	require.Nil(t, err)
	require.NotNil(t, scimUser)
	require.Equal(t, "user-123", scimUser.ID)
	require.Contains(t, scimUser.Schemas, "urn:thunderid:params:scim:schemas:employee:2.0:User")
}

// TestReplaceUser_IsSelf_UsesUpdateUserAttributes tests Replace User for Is Self Uses Update User Attributes.
func TestReplaceUser_IsSelf_UsesUpdateUserAttributes(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(
		mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

	payload := &SCIMUserPayload{
		UserTypeName:   testUserTypeEmployee,
		ExtensionURN:   "urn:thunderid:params:scim:schemas:employee:2.0:User",
		ExtensionAttrs: map[string]json.RawMessage{"given_name": json.RawMessage(`"Charlie"`)},
	}
	updatedUser := &user.User{
		ID:         "user-123",
		Type:       testUserTypeEmployee,
		OUID:       testOUID,
		Attributes: []byte(`{"given_name":"Charlie"}`),
	}

	mockUserTypeService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))
	mockUserTypeService.On(
		"GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
	).Return(&entitytype.EntityType{Name: testUserTypeEmployee, OUID: testOUID}, (*tidcommon.ServiceError)(nil))
	mockUserService.On("GetUser", mock.Anything, "user-123", false).
		Return(&user.User{ID: "user-123", Type: testUserTypeEmployee}, (*tidcommon.ServiceError)(nil))
	// Self-service replace must go through UpdateUserAttributes, not UpdateUser:
	// UpdateUser's OU/type validation requires system:usertype:view with no
	// self-access bypass, which would 500 for a self-service caller.
	mockUserService.On(
		"UpdateUserAttributes", mock.Anything, "user-123", json.RawMessage(`{"given_name":"Charlie"}`),
	).Return(updatedUser, (*tidcommon.ServiceError)(nil))
	mockUserTypeService.On(
		"GetAttributes", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
		entitytype.AttributeFilter{AllowCredential: true, AllowNonCredential: false, RequiredOnly: false},
	).Return([]entitytype.AttributeInfo{}, (*tidcommon.ServiceError)(nil))

	scimUser, err := service.ReplaceUser(context.Background(), "user-123", payload, "", testBaseURL, true)

	require.Nil(t, err)
	require.NotNil(t, scimUser)
	require.Equal(t, "user-123", scimUser.ID)
	mockUserService.AssertNotCalled(t, "UpdateUser", mock.Anything, mock.Anything, mock.Anything)
}

// TestReplaceUser_CoreOnly_NoExtensionURN_DefaultsToExistingType tests Replace User for Core Only No
// Extension URN Defaults To Existing Type.
func TestReplaceUser_CoreOnly_NoExtensionURN_DefaultsToExistingType(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(
		mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

	// The user's type is immutable, so an omitted extension URN resolves to
	// the existing user's type instead of requiring the client to echo it.
	payload := &SCIMUserPayload{
		CoreAttrs:      map[string]json.RawMessage{"userName": json.RawMessage(`"alice"`)},
		ExtensionAttrs: map[string]json.RawMessage{},
	}
	updatedUser := &user.User{
		ID:         "user-123",
		Type:       testUserTypeEmployee,
		OUID:       testOUID,
		Attributes: []byte(`{}`),
	}

	mockUserTypeService.On(
		"GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
	).Return(&entitytype.EntityType{Name: testUserTypeEmployee, OUID: testOUID}, (*tidcommon.ServiceError)(nil))
	mockUserService.On("GetUser", mock.Anything, "user-123", false).
		Return(&user.User{ID: "user-123", Type: testUserTypeEmployee}, (*tidcommon.ServiceError)(nil))
	mockUserService.On("UpdateUser", mock.Anything, "user-123", mock.MatchedBy(func(u *user.User) bool {
		return u.ID == "user-123" && u.Type == testUserTypeEmployee
	})).Return(updatedUser, (*tidcommon.ServiceError)(nil))
	mockUserTypeService.On(
		"GetAttributes", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
		entitytype.AttributeFilter{AllowCredential: true, AllowNonCredential: false, RequiredOnly: false},
	).Return([]entitytype.AttributeInfo{}, (*tidcommon.ServiceError)(nil))

	scimUser, err := service.ReplaceUser(context.Background(), "user-123", payload, "", testBaseURL, false)

	require.Nil(t, err)
	require.NotNil(t, scimUser)
	require.Equal(t, "user-123", scimUser.ID)
}

// TestReplaceUser_MissingRequiredAttribute_ReturnsSchemaValidationError tests Replace User for Missing
// Required Attribute Returns Schema Validation Error.
func TestReplaceUser_MissingRequiredAttribute_ReturnsSchemaValidationError(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(
		mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

	payload := &SCIMUserPayload{
		UserTypeName:   testUserTypeEmployee,
		ExtensionURN:   "urn:thunderid:params:scim:schemas:employee:2.0:User",
		ExtensionAttrs: map[string]json.RawMessage{"given_name": json.RawMessage(`"Charlie"`)},
	}

	mockUserTypeService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))
	mockUserTypeService.On(
		"GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
	).Return(&entitytype.EntityType{
		Name: testUserTypeEmployee, OUID: testOUID,
		Schema: json.RawMessage(`{"department":{"required":true}}`),
	}, (*tidcommon.ServiceError)(nil))
	mockUserService.On("GetUser", mock.Anything, "user-123", false).
		Return(&user.User{ID: "user-123", Type: testUserTypeEmployee}, (*tidcommon.ServiceError)(nil))

	scimUser, err := service.ReplaceUser(context.Background(), "user-123", payload, "", testBaseURL, false)

	require.NotNil(t, err)
	require.Equal(t, scim.ErrorSchemaValidationFailed.Code, err.Code)
	require.Contains(t, err.ErrorDescription.DefaultValue, "department")
	require.Nil(t, scimUser)
}

// TestReplaceUser_UndeclaredAttribute_ReturnsSchemaValidationError tests Replace User for Undeclared
// Attribute Returns Schema Validation Error.
func TestReplaceUser_UndeclaredAttribute_ReturnsSchemaValidationError(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(
		mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

	payload := &SCIMUserPayload{
		UserTypeName: testUserTypeEmployee,
		ExtensionURN: "urn:thunderid:params:scim:schemas:employee:2.0:User",
		ExtensionAttrs: map[string]json.RawMessage{
			"department": json.RawMessage(`"Eng"`),
			"undeclared": json.RawMessage(`"bad"`),
		},
	}

	mockUserTypeService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))
	mockUserTypeService.On(
		"GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
	).Return(&entitytype.EntityType{
		Name: testUserTypeEmployee, OUID: testOUID,
		Schema: json.RawMessage(`{"department":{"required":true}}`),
	}, (*tidcommon.ServiceError)(nil))
	mockUserService.On("GetUser", mock.Anything, "user-123", false).
		Return(&user.User{ID: "user-123", Type: testUserTypeEmployee}, (*tidcommon.ServiceError)(nil))

	scimUser, err := service.ReplaceUser(context.Background(), "user-123", payload, "", testBaseURL, false)

	require.NotNil(t, err)
	require.Equal(t, scim.ErrorSchemaValidationFailed.Code, err.Code)
	require.Contains(t, err.ErrorDescription.DefaultValue, "undeclared")
	require.Nil(t, scimUser)
}

// TestReplaceUser_ConflictingCoreAndCustomValue_ReturnsConflictError tests Replace User for Conflicting Core
// And Custom Value Returns Conflict Error.
func TestReplaceUser_ConflictingCoreAndCustomValue_ReturnsConflictError(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(
		mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

	payload := &SCIMUserPayload{
		UserTypeName:   testUserTypeEmployee,
		ExtensionURN:   "urn:thunderid:params:scim:schemas:employee:2.0:User",
		CoreAttrs:      map[string]json.RawMessage{"userName": json.RawMessage(`"alice"`)},
		ExtensionAttrs: map[string]json.RawMessage{"username": json.RawMessage(`"bob"`)},
	}

	mockUserTypeService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))
	mockUserTypeService.On(
		"GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
	).Return(&entitytype.EntityType{
		Name: testUserTypeEmployee, OUID: testOUID,
		Schema: json.RawMessage(`{"username":{"type":"string"}}`),
	}, (*tidcommon.ServiceError)(nil))
	mockUserService.On("GetUser", mock.Anything, "user-123", false).
		Return(&user.User{ID: "user-123", Type: testUserTypeEmployee}, (*tidcommon.ServiceError)(nil))

	scimUser, err := service.ReplaceUser(context.Background(), "user-123", payload, "", testBaseURL, false)

	require.NotNil(t, err)
	require.Equal(t, scim.ErrorConflictingAttributeValue.Code, err.Code)
	require.Contains(t, err.ErrorDescription.DefaultValue, "username")
	require.Nil(t, scimUser)
}

// TestReplaceUser_EntityTypeNotFound_ReturnsUnknownUserType tests Replace User for Entity Type Not Found
// Returns Unknown User Type.
func TestReplaceUser_EntityTypeNotFound_ReturnsUnknownUserType(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(
		mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

	payload := &SCIMUserPayload{
		UserTypeName: "ghost",
		ExtensionURN: "urn:thunderid:params:scim:schemas:ghost:2.0:User",
	}

	mockUserTypeService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(&entitytype.EntityTypeListResponse{TotalResults: 0, Types: []entitytype.EntityTypeListItem{}},
		(*tidcommon.ServiceError)(nil))
	mockUserService.On("GetUser", mock.Anything, "user-123", false).
		Return(&user.User{ID: "user-123", Type: testUserTypeEmployee}, (*tidcommon.ServiceError)(nil))

	scimUser, err := service.ReplaceUser(context.Background(), "user-123", payload, "", testBaseURL, false)

	require.NotNil(t, err)
	require.Equal(t, scim.ErrorUnknownUserType.Code, err.Code)
	require.Nil(t, scimUser)
}

// TestReplaceUser_Error_Scenarios tests Replace User for Error Scenarios.
func TestReplaceUser_Error_Scenarios(t *testing.T) {
	testCases := []struct {
		name          string
		userID        string
		mockError     *tidcommon.ServiceError
		expectedError *tidcommon.ServiceError
	}{
		{
			name:          "UserNotFound_Returns404",
			userID:        "no-such",
			mockError:     &user.ErrorUserNotFound,
			expectedError: &scim.ErrorUserNotFound,
		},
		{
			name:          "MutabilityViolation_Returns400",
			userID:        "readonly",
			mockError:     &user.ErrorCannotModifyDeclarativeResource,
			expectedError: &scim.ErrorMutabilityViolation,
		},
		{
			name:          "SchemaValidationFailed_Returns400",
			userID:        "user-123",
			mockError:     &user.ErrorSchemaValidationFailed,
			expectedError: &scim.ErrorSchemaValidationFailed,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockUserService := usermock.NewUserServiceInterfaceMock(t)
			mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
			service := newSCIMUsersService(
				mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

			payload := &SCIMUserPayload{
				UserTypeName:   testUserTypeEmployee,
				ExtensionURN:   "urn:thunderid:params:scim:schemas:employee:2.0:User",
				ExtensionAttrs: map[string]json.RawMessage{},
			}

			if tc.name == "UserNotFound_Returns404" {
				mockUserService.On("GetUser", mock.Anything, tc.userID, false).
					Return((*user.User)(nil), tc.mockError)
			} else {
				mockUserTypeService.On(
					"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
				).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))
				mockUserTypeService.On(
					"GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
				).Return(&entitytype.EntityType{
					Name: testUserTypeEmployee, OUID: testOUID,
				}, (*tidcommon.ServiceError)(nil))
				mockUserService.On("GetUser", mock.Anything, tc.userID, false).
					Return(&user.User{ID: tc.userID, Type: testUserTypeEmployee}, (*tidcommon.ServiceError)(nil))
				mockUserService.On("UpdateUser", mock.Anything, tc.userID, mock.Anything).
					Return((*user.User)(nil), tc.mockError)
			}

			scimUser, err := service.ReplaceUser(context.Background(), tc.userID, payload, "", testBaseURL, false)

			require.NotNil(t, err)
			require.Equal(t, tc.expectedError.Code, err.Code)
			require.Nil(t, scimUser)
		})
	}
}

// TestReplaceUser_IfMatch_Match tests Replace User for If Match Match.
func TestReplaceUser_IfMatch_Match(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(
		mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

	payload := &SCIMUserPayload{
		UserTypeName:   testUserTypeEmployee,
		ExtensionURN:   "urn:thunderid:params:scim:schemas:employee:2.0:User",
		ExtensionAttrs: map[string]json.RawMessage{"given_name": json.RawMessage(`"Charlie"`)},
	}
	existingUser := &user.User{ID: "user-123", Type: testUserTypeEmployee, Attributes: []byte(`{"given_name":"Bob"}`)}
	currentVersion := scim.GenerateVersion(userVersionState(*existingUser))

	mockUserTypeService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))
	mockUserTypeService.On(
		"GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
	).Return(&entitytype.EntityType{Name: testUserTypeEmployee, OUID: testOUID}, (*tidcommon.ServiceError)(nil))
	mockUserService.On("GetUser", mock.Anything, "user-123", false).
		Return(existingUser, (*tidcommon.ServiceError)(nil))
	mockUserService.On("UpdateUser", mock.Anything, "user-123", mock.Anything).
		Return(&user.User{ID: "user-123", Type: testUserTypeEmployee, Attributes: []byte(`{"given_name":"Charlie"}`)},
			(*tidcommon.ServiceError)(nil))
	mockUserTypeService.On(
		"GetAttributes", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
		entitytype.AttributeFilter{AllowCredential: true, AllowNonCredential: false, RequiredOnly: false},
	).Return([]entitytype.AttributeInfo{}, (*tidcommon.ServiceError)(nil))

	scimUser, err := service.ReplaceUser(context.Background(), "user-123", payload, currentVersion, testBaseURL, false)

	require.Nil(t, err)
	require.NotNil(t, scimUser)
}

// TestReplaceUser_IfMatch_Mismatch tests Replace User for If Match Mismatch.
func TestReplaceUser_IfMatch_Mismatch(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(
		mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

	payload := &SCIMUserPayload{
		UserTypeName:   testUserTypeEmployee,
		ExtensionURN:   "urn:thunderid:params:scim:schemas:employee:2.0:User",
		ExtensionAttrs: map[string]json.RawMessage{},
	}

	mockUserService.On("GetUser", mock.Anything, "user-123", false).
		Return(&user.User{ID: "user-123", Type: testUserTypeEmployee, Attributes: []byte(`{"given_name":"Bob"}`)},
			(*tidcommon.ServiceError)(nil))

	scimUser, err := service.ReplaceUser(context.Background(), "user-123", payload, `W/"stale"`, testBaseURL, false)

	require.Nil(t, scimUser)
	require.Equal(t, scim.ErrorPreconditionFailed.Code, err.Code)
	mockUserService.AssertNotCalled(t, "UpdateUser", mock.Anything, mock.Anything, mock.Anything)
}

// TestDeleteUser_IfMatch_Match tests Delete User for If Match Match.
func TestDeleteUser_IfMatch_Match(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(
		mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

	existingUser := &user.User{ID: "user-123", Type: testUserTypeEmployee, Attributes: []byte(`{"given_name":"Bob"}`)}
	currentVersion := scim.GenerateVersion(userVersionState(*existingUser))

	mockUserService.On("GetUser", mock.Anything, "user-123", false).
		Return(existingUser, (*tidcommon.ServiceError)(nil))
	mockUserService.On("DeleteUser", mock.Anything, "user-123").Return((*tidcommon.ServiceError)(nil))

	err := service.DeleteUser(context.Background(), "user-123", currentVersion)

	require.Nil(t, err)
}

// TestDeleteUser_IfMatch_Mismatch tests Delete User for If Match Mismatch.
func TestDeleteUser_IfMatch_Mismatch(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(
		mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

	mockUserService.On("GetUser", mock.Anything, "user-123", false).
		Return(&user.User{ID: "user-123", Type: testUserTypeEmployee, Attributes: []byte(`{"given_name":"Bob"}`)},
			(*tidcommon.ServiceError)(nil))

	err := service.DeleteUser(context.Background(), "user-123", `W/"stale"`)

	require.Equal(t, scim.ErrorPreconditionFailed.Code, err.Code)
	mockUserService.AssertNotCalled(t, "DeleteUser", mock.Anything, mock.Anything)
}

// TestReplaceUser_TypeMismatch tests Replace User for Type Mismatch.
func TestReplaceUser_TypeMismatch(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(
		mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

	payload := &SCIMUserPayload{
		UserTypeName: testUserTypeEmployee,
		ExtensionURN: "urn:thunderid:params:scim:schemas:employee:2.0:User",
	}

	mockUserTypeService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))

	mockUserService.On("GetUser", mock.Anything, "user-123", false).
		Return(&user.User{ID: "user-123", Type: "customer"}, (*tidcommon.ServiceError)(nil))

	scimUser, err := service.ReplaceUser(context.Background(), "user-123", payload, "", testBaseURL, false)

	require.Nil(t, scimUser)
	require.Equal(t, scim.ErrorImmutableUserType.Code, err.Code)
}

// TestReplaceUser_GetEntityTypeByNameError tests Replace User for Get Entity Type By Name Error.
func TestReplaceUser_GetEntityTypeByNameError(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(
		mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

	payload := &SCIMUserPayload{
		UserTypeName: testUserTypeEmployee,
		ExtensionURN: "urn:thunderid:params:scim:schemas:employee:2.0:User",
	}

	mockUserTypeService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))

	mockUserService.On("GetUser", mock.Anything, "user-123", false).
		Return(&user.User{ID: "user-123", Type: testUserTypeEmployee}, (*tidcommon.ServiceError)(nil))

	mockUserTypeService.On(
		"GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
	).Return((*entitytype.EntityType)(nil), &user.ErrorEntityTypeNotFound)

	scimUser, err := service.ReplaceUser(context.Background(), "user-123", payload, "", testBaseURL, false)

	require.Nil(t, scimUser)
	require.Equal(t, scim.ErrorUnknownUserType.Code, err.Code)
}

// TestDeleteUser_IfMatch_GetUserError tests Delete User for If Match Get User Error.
func TestDeleteUser_IfMatch_GetUserError(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(
		mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

	mockUserService.On("GetUser", mock.Anything, "user-123", false).
		Return((*user.User)(nil), &user.ErrorUserNotFound)

	err := service.DeleteUser(context.Background(), "user-123", `W/"version1"`)

	require.Equal(t, scim.ErrorUserNotFound.Code, err.Code)
}

// TestCreateUser_MarshalExtensionAttrsError tests Create User for Marshal Extension Attrs Error.
func TestCreateUser_MarshalExtensionAttrsError(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(
		mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

	payload := &SCIMUserPayload{
		UserTypeName:   testUserTypeEmployee,
		ExtensionURN:   "urn:thunderid:params:scim:schemas:employee:2.0:User",
		ExtensionAttrs: map[string]json.RawMessage{"empty": []byte("")},
	}

	mockUserTypeService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))

	mockUserTypeService.On(
		"GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
	).Return(&entitytype.EntityType{Name: testUserTypeEmployee, OUID: testOUID}, (*tidcommon.ServiceError)(nil))

	scimUser, err := service.CreateUser(context.Background(), payload, testBaseURL)

	require.Nil(t, scimUser)
	require.Equal(t, scim.ErrorInvalidRequestBody.Code, err.Code)
}

// TestReplaceUser_MarshalExtensionAttrsError tests Replace User for Marshal Extension Attrs Error.
func TestReplaceUser_MarshalExtensionAttrsError(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockUserTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(
		mockUserService, mockUserTypeService, scimconfig.SCIMConfig{ReturnMappedCoreAttrsOnGet: true})

	payload := &SCIMUserPayload{
		UserTypeName:   testUserTypeEmployee,
		ExtensionURN:   "urn:thunderid:params:scim:schemas:employee:2.0:User",
		ExtensionAttrs: map[string]json.RawMessage{"empty": []byte("")},
	}

	mockUserTypeService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))

	mockUserService.On("GetUser", mock.Anything, "user-123", false).
		Return(&user.User{ID: "user-123", Type: testUserTypeEmployee}, (*tidcommon.ServiceError)(nil))

	mockUserTypeService.On(
		"GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
	).Return(&entitytype.EntityType{Name: testUserTypeEmployee, OUID: testOUID}, (*tidcommon.ServiceError)(nil))

	scimUser, err := service.ReplaceUser(context.Background(), "user-123", payload, "", testBaseURL, false)

	require.Nil(t, scimUser)
	require.Equal(t, scim.ErrorInvalidRequestBody.Code, err.Code)
}

// TestMergeReverseMappedCoreAttrs tests Merge Reverse Mapped Core Attrs.
func TestMergeReverseMappedCoreAttrs(t *testing.T) {
	testCases := []struct {
		name           string
		extensionAttrs map[string]json.RawMessage
		reverseMapped  map[string]json.RawMessage
		expectedError  *tidcommon.ServiceError
		expectedFinal  map[string]json.RawMessage
	}{
		{
			name: "NoConflict_AddsNew",
			extensionAttrs: map[string]json.RawMessage{
				"department": json.RawMessage(`"Eng"`),
			},
			reverseMapped: map[string]json.RawMessage{
				"username": json.RawMessage(`"alice"`),
			},
			expectedError: nil,
			expectedFinal: map[string]json.RawMessage{
				"department": json.RawMessage(`"Eng"`),
				"username":   json.RawMessage(`"alice"`),
			},
		},
		{
			name: "MatchExisting_Success",
			extensionAttrs: map[string]json.RawMessage{
				"username": json.RawMessage(`"alice"`),
			},
			reverseMapped: map[string]json.RawMessage{
				"username": json.RawMessage(`"alice"`),
			},
			expectedError: nil,
			expectedFinal: map[string]json.RawMessage{
				"username": json.RawMessage(`"alice"`),
			},
		},
		{
			name: "ConflictExisting_ReturnsError",
			extensionAttrs: map[string]json.RawMessage{
				"username": json.RawMessage(`"bob"`),
			},
			reverseMapped: map[string]json.RawMessage{
				"username": json.RawMessage(`"alice"`),
			},
			expectedError: &scim.ErrorConflictingAttributeValue,
			expectedFinal: map[string]json.RawMessage{
				"username": json.RawMessage(`"bob"`),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := mergeReverseMappedCoreAttrs(tc.extensionAttrs, tc.reverseMapped)
			if tc.expectedError != nil {
				require.NotNil(t, err)
				require.Equal(t, tc.expectedError.Code, err.Code)
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.expectedFinal, tc.extensionAttrs)
			}
		})
	}
}

// TestJsonRawValuesEqual tests Json Raw Values Equal.
func TestJsonRawValuesEqual(t *testing.T) {
	testCases := []struct {
		name     string
		a        json.RawMessage
		b        json.RawMessage
		expected bool
	}{
		{
			name:     "StringMatch",
			a:        json.RawMessage(`"alice"`),
			b:        json.RawMessage(`"alice"`),
			expected: true,
		},
		{
			name:     "StringMismatch",
			a:        json.RawMessage(`"alice"`),
			b:        json.RawMessage(`"bob"`),
			expected: false,
		},
		{
			name:     "ObjectMatchWhitespaceDifferences",
			a:        json.RawMessage(`{"a": 1, "b": 2}`),
			b:        json.RawMessage(`{ "a":1,"b":2 }`),
			expected: true,
		},
		{
			name:     "ObjectMatchKeyOrderDifferences",
			a:        json.RawMessage(`{"a": 1, "b": 2}`),
			b:        json.RawMessage(`{"b": 2, "a": 1}`),
			expected: true,
		},
		{
			name:     "ObjectMismatch",
			a:        json.RawMessage(`{"a": 1}`),
			b:        json.RawMessage(`{"a": 2}`),
			expected: false,
		},
		{
			name:     "InvalidJson_FallbackToByteCompare_Match",
			a:        json.RawMessage(`invalid`),
			b:        json.RawMessage(`invalid`),
			expected: true,
		},
		{
			name:     "InvalidJson_FallbackToByteCompare_Mismatch",
			a:        json.RawMessage(`invalid1`),
			b:        json.RawMessage(`invalid2`),
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := jsonRawValuesEqual(tc.a, tc.b)
			require.Equal(t, tc.expected, result)
		})
	}
}
