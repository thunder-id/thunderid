package scim

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/entitytype"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/user"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/usermock"
)

const testUserTypeEmployee = "employee"
const testOUID = "ou-abc"

func TestGetUser_Success(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockEntityService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)

	service := newSCIMUsersService(mockUserService, mockEntityService)

	internalUser := &user.User{
		ID:         "user-123",
		Type:       testUserTypeEmployee,
		Attributes: []byte(`{"given_name": "John"}`),
	}
	mockUserService.On("GetUser", mock.Anything, "user-123", false).Return(internalUser, (*tidcommon.ServiceError)(nil))
	mockEntityService.On(
		"GetAttributes", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee, true, false, false,
	).Return([]entitytype.AttributeInfo{{Attribute: "password"}}, (*tidcommon.ServiceError)(nil))

	scimUser, err := service.GetUser(context.Background(), "user-123", testBaseURL)

	require.Nil(t, err)
	require.NotNil(t, scimUser)
	require.Equal(t, "user-123", scimUser.ID)
	require.Contains(t, scimUser.Schemas, "urn:thunderid:params:scim:schemas:employee:2.0:User")
}

func TestGetUser_NotFound(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockEntityService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)

	service := newSCIMUsersService(mockUserService, mockEntityService)

	mockUserService.On("GetUser", mock.Anything, "user-123", false).Return((*user.User)(nil), &user.ErrorUserNotFound)

	scimUser, err := service.GetUser(context.Background(), "user-123", testBaseURL)

	require.NotNil(t, err)
	require.Equal(t, ErrorUserNotFound.Code, err.Code)
	require.Nil(t, scimUser)
}

func TestDeleteUser_Success(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockEntityService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)

	service := newSCIMUsersService(mockUserService, mockEntityService)

	mockUserService.On("DeleteUser", mock.Anything, "user-123").Return((*tidcommon.ServiceError)(nil))

	err := service.DeleteUser(context.Background(), "user-123", "")

	require.Nil(t, err)
}

func TestDeleteUser_NotFound(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockEntityService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)

	service := newSCIMUsersService(mockUserService, mockEntityService)

	mockUserService.On("DeleteUser", mock.Anything, "user-123").Return(&user.ErrorUserNotFound)

	err := service.DeleteUser(context.Background(), "user-123", "")

	require.NotNil(t, err)
	require.Equal(t, ErrorUserNotFound.Code, err.Code)
}

func TestDeleteUser_MutabilityViolation_MapsToSCIM(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockEntityService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)

	service := newSCIMUsersService(mockUserService, mockEntityService)

	mockUserService.On("DeleteUser", mock.Anything, "user-123").
		Return(&user.ErrorCannotModifyDeclarativeResource)

	err := service.DeleteUser(context.Background(), "user-123", "")

	require.NotNil(t, err)
	require.Equal(t, ErrorMutabilityViolation.Code, err.Code)
}

func TestGetUser_UniquenessConflict_MapsToSCIM(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockEntityService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)

	service := newSCIMUsersService(mockUserService, mockEntityService)

	mockUserService.On("GetUser", mock.Anything, "user-123", false).
		Return((*user.User)(nil), &user.ErrorAttributeConflict)

	scimUser, err := service.GetUser(context.Background(), "user-123", testBaseURL)

	require.NotNil(t, err)
	require.Equal(t, ErrorUniquenessConflict.Code, err.Code)
	require.Nil(t, scimUser)
}

func TestGetUser_SchemaValidationError_MapsToSCIM(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockEntityService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)

	service := newSCIMUsersService(mockUserService, mockEntityService)

	mockUserService.On("GetUser", mock.Anything, "user-123", false).
		Return((*user.User)(nil), &user.ErrorSchemaValidationFailed)

	scimUser, err := service.GetUser(context.Background(), "user-123", testBaseURL)

	require.NotNil(t, err)
	require.Equal(t, ErrorSchemaValidationFailed.Code, err.Code)
	require.Nil(t, scimUser)
}

func TestListUsers_Success(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockEntityService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)

	service := newSCIMUsersService(mockUserService, mockEntityService)

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
	mockEntityService.On(
		"GetAttributes", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee, true, false, false,
	).Return([]entitytype.AttributeInfo{}, (*tidcommon.ServiceError)(nil))

	resp, err := service.ListUsers(context.Background(), 1, 20, nil, testBaseURL)

	require.Nil(t, err)
	require.Equal(t, 1, resp.TotalResults)
	require.Len(t, resp.Resources, 1)
	require.Equal(t, "user-1", resp.Resources[0].ID)
}

func TestListUsers_ServiceError(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockEntityService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)

	service := newSCIMUsersService(mockUserService, mockEntityService)

	mockUserService.On("GetUserList", mock.Anything, 20, 0, (map[string]interface{})(nil), false).
		Return((*user.UserListResponse)(nil), &user.ErrorUserNotFound)

	resp, err := service.ListUsers(context.Background(), 1, 20, nil, testBaseURL)

	require.NotNil(t, err)
	require.Equal(t, ErrorUserNotFound.Code, err.Code)
	require.Empty(t, resp.Resources)
}

func TestListUsers_DefaultsInvalidPagination(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockEntityService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)

	service := newSCIMUsersService(mockUserService, mockEntityService)

	mockUserService.On("GetUserList",
		mock.Anything, serverconst.DefaultPageSize, 0, (map[string]interface{})(nil), false).
		Return(&user.UserListResponse{TotalResults: 0, Users: []user.User{}}, (*tidcommon.ServiceError)(nil))

	resp, err := service.ListUsers(context.Background(), 0, 0, nil, testBaseURL)

	require.Nil(t, err)
	require.Equal(t, 0, resp.TotalResults)
}

func TestMapUserServiceErrorToSCIM_AllCodes(t *testing.T) {
	tests := []struct {
		input    *tidcommon.ServiceError
		wantCode string
	}{
		{&user.ErrorUserNotFound, ErrorUserNotFound.Code},
		{&user.ErrorAttributeConflict, ErrorUniquenessConflict.Code},
		{&user.ErrorSchemaValidationFailed, ErrorSchemaValidationFailed.Code},
		{&user.ErrorEntityTypeNotFound, ErrorUnknownUserType.Code},
		{&user.ErrorCannotModifyDeclarativeResource, ErrorMutabilityViolation.Code},
		{&tidcommon.ErrorUnauthorized, tidcommon.ErrorUnauthorized.Code},
		// Unknown client error → invalidRequestBody
		{&user.ErrorInvalidRequestFormat, ErrorInvalidRequestBody.Code},
	}

	for _, tc := range tests {
		t.Run(tc.input.Code, func(t *testing.T) {
			got := mapUserServiceErrorToSCIM(tc.input)
			require.NotNil(t, got)
			require.Equal(t, tc.wantCode, got.Code)
		})
	}
}

func TestMapUserServiceErrorToSCIM_ServerError_MapsToInternalServer(t *testing.T) {
	serverErr := &tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		Code: "SRV-9999",
	}
	got := mapUserServiceErrorToSCIM(serverErr)
	require.NotNil(t, got)
	require.Equal(t, tidcommon.InternalServerError.Code, got.Code)
}

func TestMapUserServiceErrorToSCIM_Nil_ReturnsNil(t *testing.T) {
	require.Nil(t, mapUserServiceErrorToSCIM(nil))
}

// resolveEntityTypeNameForSchemaURN is called with the user type name extracted
// from the schema URN. It pages through GetEntityTypeList and matches by name
// (case-insensitive). The tests below set up the minimal mock chain:
//   GetEntityTypeList  →  list containing the type  →  GetEntityTypeByName
//   GetEntityTypeByName → EntityType with OUID
//   userService.CreateUser / UpdateUser → created/updated User

func makeEntityTypeListPage() *entitytype.EntityTypeListResponse {
	return &entitytype.EntityTypeListResponse{
		TotalResults: 1,
		Types: []entitytype.EntityTypeListItem{
			{Name: testUserTypeEmployee, OUID: testOUID},
		},
	}
}

// --- CreateUser ---

func TestCreateUser_Success(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockEntityService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(mockUserService, mockEntityService)

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

	// resolveEntityTypeNameForSchemaURN pages GetEntityTypeList
	mockEntityService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))

	// GetEntityTypeByName after resolution
	mockEntityService.On(
		"GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
	).Return(&entitytype.EntityType{Name: testUserTypeEmployee, OUID: testOUID}, (*tidcommon.ServiceError)(nil))

	mockUserService.On("CreateUser", mock.Anything, mock.MatchedBy(func(u *user.User) bool {
		return u.Type == testUserTypeEmployee && u.OUID == testOUID
	})).Return(createdUser, (*tidcommon.ServiceError)(nil))
	mockEntityService.On(
		"GetAttributes", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee, true, false, false,
	).Return([]entitytype.AttributeInfo{}, (*tidcommon.ServiceError)(nil))

	scimUser, err := service.CreateUser(context.Background(), payload, testBaseURL)

	require.Nil(t, err)
	require.NotNil(t, scimUser)
	require.Equal(t, "user-new", scimUser.ID)
	require.Contains(t, scimUser.Schemas, "urn:thunderid:params:scim:schemas:employee:2.0:User")
}

func TestCreateUser_MissingRequiredAttribute_ReturnsSchemaValidationError(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockEntityService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(mockUserService, mockEntityService)

	payload := &SCIMUserPayload{
		UserTypeName:   testUserTypeEmployee,
		ExtensionURN:   "urn:thunderid:params:scim:schemas:employee:2.0:User",
		ExtensionAttrs: map[string]json.RawMessage{"given_name": json.RawMessage(`"Alice"`)},
	}

	mockEntityService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))
	mockEntityService.On(
		"GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
	).Return(&entitytype.EntityType{
		Name: testUserTypeEmployee, OUID: testOUID,
		Schema: json.RawMessage(`{"department":{"required":true}}`),
	}, (*tidcommon.ServiceError)(nil))

	scimUser, err := service.CreateUser(context.Background(), payload, testBaseURL)

	require.NotNil(t, err)
	require.Equal(t, ErrorSchemaValidationFailed.Code, err.Code)
	require.Contains(t, err.ErrorDescription.DefaultValue, "department")
	require.Nil(t, scimUser)
}

func TestCreateUser_UndeclaredAttribute_ReturnsSchemaValidationError(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockEntityService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(mockUserService, mockEntityService)

	payload := &SCIMUserPayload{
		UserTypeName: testUserTypeEmployee,
		ExtensionURN: "urn:thunderid:params:scim:schemas:employee:2.0:User",
		ExtensionAttrs: map[string]json.RawMessage{
			"department": json.RawMessage(`"Eng"`),
			"undeclared": json.RawMessage(`"bad"`),
		},
	}

	mockEntityService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))
	mockEntityService.On(
		"GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
	).Return(&entitytype.EntityType{
		Name: testUserTypeEmployee, OUID: testOUID,
		Schema: json.RawMessage(`{"department":{"required":true}}`),
	}, (*tidcommon.ServiceError)(nil))

	scimUser, err := service.CreateUser(context.Background(), payload, testBaseURL)

	require.NotNil(t, err)
	require.Equal(t, ErrorSchemaValidationFailed.Code, err.Code)
	require.Contains(t, err.ErrorDescription.DefaultValue, "undeclared")
	require.Nil(t, scimUser)
}

func TestCreateUser_MalformedSchemaJSON_ReturnsInternalServerError(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockEntityService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(mockUserService, mockEntityService)

	payload := &SCIMUserPayload{
		UserTypeName: testUserTypeEmployee,
		ExtensionURN: "urn:thunderid:params:scim:schemas:employee:2.0:User",
		CoreAttrs: map[string]json.RawMessage{
			"userName": json.RawMessage(`"alice"`),
		},
	}

	mockEntityService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))
	mockEntityService.On(
		"GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
	).Return(&entitytype.EntityType{
		Name: testUserTypeEmployee, OUID: testOUID,
		Schema: json.RawMessage(`invalid json`),
	}, (*tidcommon.ServiceError)(nil))

	scimUser, err := service.CreateUser(context.Background(), payload, testBaseURL)

	require.NotNil(t, err)
	require.Equal(t, ErrorInternalServer.Code, err.Code)
	require.Nil(t, scimUser)
}

func TestReplaceUser_MalformedSchemaJSON_ReturnsInternalServerError(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockEntityService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(mockUserService, mockEntityService)

	payload := &SCIMUserPayload{
		UserTypeName: testUserTypeEmployee,
		ExtensionURN: "urn:thunderid:params:scim:schemas:employee:2.0:User",
		CoreAttrs: map[string]json.RawMessage{
			"userName": json.RawMessage(`"alice"`),
		},
	}

	mockEntityService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))
	mockEntityService.On(
		"GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
	).Return(&entitytype.EntityType{
		Name: testUserTypeEmployee, OUID: testOUID,
		Schema: json.RawMessage(`invalid json`),
	}, (*tidcommon.ServiceError)(nil))
	mockUserService.On("GetUser", mock.Anything, "user-123", false).
		Return(&user.User{ID: "user-123", Type: testUserTypeEmployee}, (*tidcommon.ServiceError)(nil))

	scimUser, err := service.ReplaceUser(context.Background(), "user-123", payload, "", testBaseURL)

	require.NotNil(t, err)
	require.Equal(t, ErrorInternalServer.Code, err.Code)
	require.Nil(t, scimUser)
}

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
			wantErrorCode:    ErrorConflictingAttributeValue.Code,
			wantDescContains: "username",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUserService := usermock.NewUserServiceInterfaceMock(t)
			mockEntityService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
			service := newSCIMUsersService(mockUserService, mockEntityService)

			mockEntityService.On(
				"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
			).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))
			mockEntityService.On(
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

func TestCreateUser_MatchingCoreAndCustomValue_Succeeds(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockEntityService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(mockUserService, mockEntityService)

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

	mockEntityService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))
	mockEntityService.On(
		"GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
	).Return(&entitytype.EntityType{
		Name: testUserTypeEmployee, OUID: testOUID,
		Schema: json.RawMessage(`{"username":{"type":"string"}}`),
	}, (*tidcommon.ServiceError)(nil))
	mockUserService.On("CreateUser", mock.Anything, mock.MatchedBy(func(u *user.User) bool {
		return u.Type == testUserTypeEmployee && u.OUID == testOUID
	})).Return(createdUser, (*tidcommon.ServiceError)(nil))
	mockEntityService.On(
		"GetAttributes", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee, true, false, false,
	).Return([]entitytype.AttributeInfo{}, (*tidcommon.ServiceError)(nil))

	scimUser, err := service.CreateUser(context.Background(), payload, testBaseURL)

	require.Nil(t, err)
	require.NotNil(t, scimUser)
	require.Equal(t, "user-new", scimUser.ID)
}

func TestCreateUser_CoreOnly_SingleUserType_DefaultsToType(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockEntityService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(mockUserService, mockEntityService)

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

	// resolveDefaultEntityTypeName pages GetEntityTypeList and, finding
	// exactly one configured user type, defaults to it.
	mockEntityService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))
	mockEntityService.On(
		"GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
	).Return(&entitytype.EntityType{
		Name: testUserTypeEmployee, OUID: testOUID,
		Schema: json.RawMessage(`{"username":{"type":"string"}}`),
	}, (*tidcommon.ServiceError)(nil))
	mockUserService.On("CreateUser", mock.Anything, mock.MatchedBy(func(u *user.User) bool {
		return u.Type == testUserTypeEmployee && u.OUID == testOUID
	})).Return(createdUser, (*tidcommon.ServiceError)(nil))
	mockEntityService.On(
		"GetAttributes", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee, true, false, false,
	).Return([]entitytype.AttributeInfo{}, (*tidcommon.ServiceError)(nil))

	scimUser, err := service.CreateUser(context.Background(), payload, testBaseURL)

	require.Nil(t, err)
	require.NotNil(t, scimUser)
	require.Equal(t, "user-new", scimUser.ID)
	require.Contains(t, scimUser.Schemas, "urn:thunderid:params:scim:schemas:employee:2.0:User")
}

func TestCreateUser_CoreOnly_MultipleUserTypes_ReturnsMissingCustomSchema(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockEntityService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(mockUserService, mockEntityService)

	payload := &SCIMUserPayload{
		CoreAttrs: map[string]json.RawMessage{"userName": json.RawMessage(`"alice"`)},
	}

	// Two configured user types: which one to default to is ambiguous.
	mockEntityService.On(
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
	require.Equal(t, ErrorMissingCustomSchema.Code, err.Code)
	require.Nil(t, scimUser)
}

func TestCreateUser_CoreOnly_ZeroUserTypes_ReturnsMissingCustomSchema(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockEntityService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(mockUserService, mockEntityService)

	payload := &SCIMUserPayload{
		CoreAttrs: map[string]json.RawMessage{"userName": json.RawMessage(`"alice"`)},
	}

	mockEntityService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(&entitytype.EntityTypeListResponse{TotalResults: 0, Types: []entitytype.EntityTypeListItem{}},
		(*tidcommon.ServiceError)(nil))

	scimUser, err := service.CreateUser(context.Background(), payload, testBaseURL)

	require.NotNil(t, err)
	require.Equal(t, ErrorMissingCustomSchema.Code, err.Code)
	require.Nil(t, scimUser)
}

func TestCreateUser_EntityTypeNotFound_ReturnsUnknownUserType(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockEntityService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(mockUserService, mockEntityService)

	payload := &SCIMUserPayload{
		UserTypeName: "ghost",
		ExtensionURN: "urn:thunderid:params:scim:schemas:ghost:2.0:User",
	}

	// resolver finds no match — returns empty list
	mockEntityService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(&entitytype.EntityTypeListResponse{TotalResults: 0, Types: []entitytype.EntityTypeListItem{}},
		(*tidcommon.ServiceError)(nil))

	scimUser, err := service.CreateUser(context.Background(), payload, testBaseURL)

	require.NotNil(t, err)
	require.Equal(t, ErrorUnknownUserType.Code, err.Code)
	require.Nil(t, scimUser)
}

func TestCreateUser_EntityTypeListError_ReturnsUnknownUserType(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockEntityService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(mockUserService, mockEntityService)

	payload := &SCIMUserPayload{
		UserTypeName: testUserTypeEmployee,
		ExtensionURN: "urn:thunderid:params:scim:schemas:employee:2.0:User",
	}

	mockEntityService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return((*entitytype.EntityTypeListResponse)(nil), &tidcommon.ErrorUnauthorized)

	scimUser, err := service.CreateUser(context.Background(), payload, testBaseURL)

	require.NotNil(t, err)
	require.Equal(t, ErrorUnknownUserType.Code, err.Code)
	require.Nil(t, scimUser)
}

func TestCreateUser_GetEntityTypeByNameError_ReturnsUnknownUserType(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockEntityService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(mockUserService, mockEntityService)

	payload := &SCIMUserPayload{
		UserTypeName: testUserTypeEmployee,
		ExtensionURN: "urn:thunderid:params:scim:schemas:employee:2.0:User",
	}

	mockEntityService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))

	mockEntityService.On(
		"GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
	).Return((*entitytype.EntityType)(nil), &user.ErrorEntityTypeNotFound)

	scimUser, err := service.CreateUser(context.Background(), payload, testBaseURL)

	require.NotNil(t, err)
	require.Equal(t, ErrorUnknownUserType.Code, err.Code)
	require.Nil(t, scimUser)
}

func TestCreateUser_Error_Scenarios(t *testing.T) {
	testCases := []struct {
		name          string
		mockError     *tidcommon.ServiceError
		expectedError *tidcommon.ServiceError
	}{
		{
			name:          "UserServiceConflict_ReturnsUniqueness",
			mockError:     &user.ErrorAttributeConflict,
			expectedError: &ErrorUniquenessConflict,
		},
		{
			name:          "SchemaValidationFailed_ReturnsSCIMError",
			mockError:     &user.ErrorSchemaValidationFailed,
			expectedError: &ErrorSchemaValidationFailed,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockUserService := usermock.NewUserServiceInterfaceMock(t)
			mockEntityService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
			service := newSCIMUsersService(mockUserService, mockEntityService)

			payload := &SCIMUserPayload{
				UserTypeName:   testUserTypeEmployee,
				ExtensionURN:   "urn:thunderid:params:scim:schemas:employee:2.0:User",
				ExtensionAttrs: map[string]json.RawMessage{},
			}

			mockEntityService.On(
				"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
			).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))
			mockEntityService.On(
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

func TestReplaceUser_Success(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockEntityService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(mockUserService, mockEntityService)

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

	mockEntityService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))
	mockEntityService.On(
		"GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
	).Return(&entitytype.EntityType{Name: testUserTypeEmployee, OUID: testOUID}, (*tidcommon.ServiceError)(nil))
	mockUserService.On("GetUser", mock.Anything, "user-123", false).
		Return(&user.User{ID: "user-123", Type: testUserTypeEmployee}, (*tidcommon.ServiceError)(nil))
	mockUserService.On("UpdateUser", mock.Anything, "user-123", mock.MatchedBy(func(u *user.User) bool {
		return u.ID == "user-123" && u.Type == testUserTypeEmployee
	})).Return(updatedUser, (*tidcommon.ServiceError)(nil))
	mockEntityService.On(
		"GetAttributes", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee, true, false, false,
	).Return([]entitytype.AttributeInfo{}, (*tidcommon.ServiceError)(nil))

	scimUser, err := service.ReplaceUser(context.Background(), "user-123", payload, "", testBaseURL)

	require.Nil(t, err)
	require.NotNil(t, scimUser)
	require.Equal(t, "user-123", scimUser.ID)
	require.Contains(t, scimUser.Schemas, "urn:thunderid:params:scim:schemas:employee:2.0:User")
}

func TestReplaceUser_CoreOnly_NoExtensionURN_DefaultsToExistingType(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockEntityService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(mockUserService, mockEntityService)

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

	mockEntityService.On(
		"GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
	).Return(&entitytype.EntityType{Name: testUserTypeEmployee, OUID: testOUID}, (*tidcommon.ServiceError)(nil))
	mockUserService.On("GetUser", mock.Anything, "user-123", false).
		Return(&user.User{ID: "user-123", Type: testUserTypeEmployee}, (*tidcommon.ServiceError)(nil))
	mockUserService.On("UpdateUser", mock.Anything, "user-123", mock.MatchedBy(func(u *user.User) bool {
		return u.ID == "user-123" && u.Type == testUserTypeEmployee
	})).Return(updatedUser, (*tidcommon.ServiceError)(nil))
	mockEntityService.On(
		"GetAttributes", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee, true, false, false,
	).Return([]entitytype.AttributeInfo{}, (*tidcommon.ServiceError)(nil))

	scimUser, err := service.ReplaceUser(context.Background(), "user-123", payload, "", testBaseURL)

	require.Nil(t, err)
	require.NotNil(t, scimUser)
	require.Equal(t, "user-123", scimUser.ID)
}

func TestReplaceUser_MissingRequiredAttribute_ReturnsSchemaValidationError(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockEntityService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(mockUserService, mockEntityService)

	payload := &SCIMUserPayload{
		UserTypeName:   testUserTypeEmployee,
		ExtensionURN:   "urn:thunderid:params:scim:schemas:employee:2.0:User",
		ExtensionAttrs: map[string]json.RawMessage{"given_name": json.RawMessage(`"Charlie"`)},
	}

	mockEntityService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))
	mockEntityService.On(
		"GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
	).Return(&entitytype.EntityType{
		Name: testUserTypeEmployee, OUID: testOUID,
		Schema: json.RawMessage(`{"department":{"required":true}}`),
	}, (*tidcommon.ServiceError)(nil))
	mockUserService.On("GetUser", mock.Anything, "user-123", false).
		Return(&user.User{ID: "user-123", Type: testUserTypeEmployee}, (*tidcommon.ServiceError)(nil))

	scimUser, err := service.ReplaceUser(context.Background(), "user-123", payload, "", testBaseURL)

	require.NotNil(t, err)
	require.Equal(t, ErrorSchemaValidationFailed.Code, err.Code)
	require.Contains(t, err.ErrorDescription.DefaultValue, "department")
	require.Nil(t, scimUser)
}

func TestReplaceUser_UndeclaredAttribute_ReturnsSchemaValidationError(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockEntityService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(mockUserService, mockEntityService)

	payload := &SCIMUserPayload{
		UserTypeName: testUserTypeEmployee,
		ExtensionURN: "urn:thunderid:params:scim:schemas:employee:2.0:User",
		ExtensionAttrs: map[string]json.RawMessage{
			"department": json.RawMessage(`"Eng"`),
			"undeclared": json.RawMessage(`"bad"`),
		},
	}

	mockEntityService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))
	mockEntityService.On(
		"GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
	).Return(&entitytype.EntityType{
		Name: testUserTypeEmployee, OUID: testOUID,
		Schema: json.RawMessage(`{"department":{"required":true}}`),
	}, (*tidcommon.ServiceError)(nil))
	mockUserService.On("GetUser", mock.Anything, "user-123", false).
		Return(&user.User{ID: "user-123", Type: testUserTypeEmployee}, (*tidcommon.ServiceError)(nil))

	scimUser, err := service.ReplaceUser(context.Background(), "user-123", payload, "", testBaseURL)

	require.NotNil(t, err)
	require.Equal(t, ErrorSchemaValidationFailed.Code, err.Code)
	require.Contains(t, err.ErrorDescription.DefaultValue, "undeclared")
	require.Nil(t, scimUser)
}

func TestReplaceUser_ConflictingCoreAndCustomValue_ReturnsConflictError(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockEntityService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(mockUserService, mockEntityService)

	payload := &SCIMUserPayload{
		UserTypeName:   testUserTypeEmployee,
		ExtensionURN:   "urn:thunderid:params:scim:schemas:employee:2.0:User",
		CoreAttrs:      map[string]json.RawMessage{"userName": json.RawMessage(`"alice"`)},
		ExtensionAttrs: map[string]json.RawMessage{"username": json.RawMessage(`"bob"`)},
	}

	mockEntityService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))
	mockEntityService.On(
		"GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
	).Return(&entitytype.EntityType{
		Name: testUserTypeEmployee, OUID: testOUID,
		Schema: json.RawMessage(`{"username":{"type":"string"}}`),
	}, (*tidcommon.ServiceError)(nil))
	mockUserService.On("GetUser", mock.Anything, "user-123", false).
		Return(&user.User{ID: "user-123", Type: testUserTypeEmployee}, (*tidcommon.ServiceError)(nil))

	scimUser, err := service.ReplaceUser(context.Background(), "user-123", payload, "", testBaseURL)

	require.NotNil(t, err)
	require.Equal(t, ErrorConflictingAttributeValue.Code, err.Code)
	require.Contains(t, err.ErrorDescription.DefaultValue, "username")
	require.Nil(t, scimUser)
}

func TestReplaceUser_EntityTypeNotFound_ReturnsUnknownUserType(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockEntityService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(mockUserService, mockEntityService)

	payload := &SCIMUserPayload{
		UserTypeName: "ghost",
		ExtensionURN: "urn:thunderid:params:scim:schemas:ghost:2.0:User",
	}

	mockEntityService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(&entitytype.EntityTypeListResponse{TotalResults: 0, Types: []entitytype.EntityTypeListItem{}},
		(*tidcommon.ServiceError)(nil))
	mockUserService.On("GetUser", mock.Anything, "user-123", false).
		Return(&user.User{ID: "user-123", Type: testUserTypeEmployee}, (*tidcommon.ServiceError)(nil))

	scimUser, err := service.ReplaceUser(context.Background(), "user-123", payload, "", testBaseURL)

	require.NotNil(t, err)
	require.Equal(t, ErrorUnknownUserType.Code, err.Code)
	require.Nil(t, scimUser)
}

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
			expectedError: &ErrorUserNotFound,
		},
		{
			name:          "MutabilityViolation_Returns400",
			userID:        "readonly",
			mockError:     &user.ErrorCannotModifyDeclarativeResource,
			expectedError: &ErrorMutabilityViolation,
		},
		{
			name:          "SchemaValidationFailed_Returns400",
			userID:        "user-123",
			mockError:     &user.ErrorSchemaValidationFailed,
			expectedError: &ErrorSchemaValidationFailed,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockUserService := usermock.NewUserServiceInterfaceMock(t)
			mockEntityService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
			service := newSCIMUsersService(mockUserService, mockEntityService)

			payload := &SCIMUserPayload{
				UserTypeName:   testUserTypeEmployee,
				ExtensionURN:   "urn:thunderid:params:scim:schemas:employee:2.0:User",
				ExtensionAttrs: map[string]json.RawMessage{},
			}

			if tc.name == "UserNotFound_Returns404" {
				mockUserService.On("GetUser", mock.Anything, tc.userID, false).
					Return((*user.User)(nil), tc.mockError)
			} else {
				mockEntityService.On(
					"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
				).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))
				mockEntityService.On(
					"GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
				).Return(&entitytype.EntityType{
					Name: testUserTypeEmployee, OUID: testOUID,
				}, (*tidcommon.ServiceError)(nil))
				mockUserService.On("GetUser", mock.Anything, tc.userID, false).
					Return(&user.User{ID: tc.userID, Type: testUserTypeEmployee}, (*tidcommon.ServiceError)(nil))
				mockUserService.On("UpdateUser", mock.Anything, tc.userID, mock.Anything).
					Return((*user.User)(nil), tc.mockError)
			}

			scimUser, err := service.ReplaceUser(context.Background(), tc.userID, payload, "", testBaseURL)

			require.NotNil(t, err)
			require.Equal(t, tc.expectedError.Code, err.Code)
			require.Nil(t, scimUser)
		})
	}
}

func TestReplaceUser_IfMatch_Match(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockEntityService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(mockUserService, mockEntityService)

	payload := &SCIMUserPayload{
		UserTypeName:   testUserTypeEmployee,
		ExtensionURN:   "urn:thunderid:params:scim:schemas:employee:2.0:User",
		ExtensionAttrs: map[string]json.RawMessage{"given_name": json.RawMessage(`"Charlie"`)},
	}
	existingUser := &user.User{ID: "user-123", Type: testUserTypeEmployee, Attributes: []byte(`{"given_name":"Bob"}`)}
	currentVersion := generateVersion(userVersionState(*existingUser))

	mockEntityService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))
	mockEntityService.On(
		"GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
	).Return(&entitytype.EntityType{Name: testUserTypeEmployee, OUID: testOUID}, (*tidcommon.ServiceError)(nil))
	mockUserService.On("GetUser", mock.Anything, "user-123", false).
		Return(existingUser, (*tidcommon.ServiceError)(nil))
	mockUserService.On("UpdateUser", mock.Anything, "user-123", mock.Anything).
		Return(&user.User{ID: "user-123", Type: testUserTypeEmployee, Attributes: []byte(`{"given_name":"Charlie"}`)},
			(*tidcommon.ServiceError)(nil))
	mockEntityService.On(
		"GetAttributes", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee, true, false, false,
	).Return([]entitytype.AttributeInfo{}, (*tidcommon.ServiceError)(nil))

	scimUser, err := service.ReplaceUser(context.Background(), "user-123", payload, currentVersion, testBaseURL)

	require.Nil(t, err)
	require.NotNil(t, scimUser)
}

func TestReplaceUser_IfMatch_Mismatch(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockEntityService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(mockUserService, mockEntityService)

	payload := &SCIMUserPayload{
		UserTypeName:   testUserTypeEmployee,
		ExtensionURN:   "urn:thunderid:params:scim:schemas:employee:2.0:User",
		ExtensionAttrs: map[string]json.RawMessage{},
	}

	mockUserService.On("GetUser", mock.Anything, "user-123", false).
		Return(&user.User{ID: "user-123", Type: testUserTypeEmployee, Attributes: []byte(`{"given_name":"Bob"}`)},
			(*tidcommon.ServiceError)(nil))

	scimUser, err := service.ReplaceUser(context.Background(), "user-123", payload, `W/"stale"`, testBaseURL)

	require.Nil(t, scimUser)
	require.Equal(t, ErrorPreconditionFailed.Code, err.Code)
	mockUserService.AssertNotCalled(t, "UpdateUser", mock.Anything, mock.Anything, mock.Anything)
}

func TestDeleteUser_IfMatch_Match(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockEntityService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(mockUserService, mockEntityService)

	existingUser := &user.User{ID: "user-123", Type: testUserTypeEmployee, Attributes: []byte(`{"given_name":"Bob"}`)}
	currentVersion := generateVersion(userVersionState(*existingUser))

	mockUserService.On("GetUser", mock.Anything, "user-123", false).
		Return(existingUser, (*tidcommon.ServiceError)(nil))
	mockUserService.On("DeleteUser", mock.Anything, "user-123").Return((*tidcommon.ServiceError)(nil))

	err := service.DeleteUser(context.Background(), "user-123", currentVersion)

	require.Nil(t, err)
}

func TestDeleteUser_IfMatch_Mismatch(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockEntityService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(mockUserService, mockEntityService)

	mockUserService.On("GetUser", mock.Anything, "user-123", false).
		Return(&user.User{ID: "user-123", Type: testUserTypeEmployee, Attributes: []byte(`{"given_name":"Bob"}`)},
			(*tidcommon.ServiceError)(nil))

	err := service.DeleteUser(context.Background(), "user-123", `W/"stale"`)

	require.Equal(t, ErrorPreconditionFailed.Code, err.Code)
	mockUserService.AssertNotCalled(t, "DeleteUser", mock.Anything, mock.Anything)
}

func TestReplaceUser_TypeMismatch(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockEntityService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(mockUserService, mockEntityService)

	payload := &SCIMUserPayload{
		UserTypeName: testUserTypeEmployee,
		ExtensionURN: "urn:thunderid:params:scim:schemas:employee:2.0:User",
	}

	mockEntityService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))

	mockUserService.On("GetUser", mock.Anything, "user-123", false).
		Return(&user.User{ID: "user-123", Type: "customer"}, (*tidcommon.ServiceError)(nil))

	scimUser, err := service.ReplaceUser(context.Background(), "user-123", payload, "", testBaseURL)

	require.Nil(t, scimUser)
	require.Equal(t, ErrorImmutableUserType.Code, err.Code)
}

func TestReplaceUser_GetEntityTypeByNameError(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockEntityService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(mockUserService, mockEntityService)

	payload := &SCIMUserPayload{
		UserTypeName: testUserTypeEmployee,
		ExtensionURN: "urn:thunderid:params:scim:schemas:employee:2.0:User",
	}

	mockEntityService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))

	mockUserService.On("GetUser", mock.Anything, "user-123", false).
		Return(&user.User{ID: "user-123", Type: testUserTypeEmployee}, (*tidcommon.ServiceError)(nil))

	mockEntityService.On(
		"GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
	).Return((*entitytype.EntityType)(nil), &user.ErrorEntityTypeNotFound)

	scimUser, err := service.ReplaceUser(context.Background(), "user-123", payload, "", testBaseURL)

	require.Nil(t, scimUser)
	require.Equal(t, ErrorUnknownUserType.Code, err.Code)
}

func TestDeleteUser_IfMatch_GetUserError(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockEntityService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(mockUserService, mockEntityService)

	mockUserService.On("GetUser", mock.Anything, "user-123", false).
		Return((*user.User)(nil), &user.ErrorUserNotFound)

	err := service.DeleteUser(context.Background(), "user-123", `W/"version1"`)

	require.Equal(t, ErrorUserNotFound.Code, err.Code)
}

func TestCreateUser_MarshalExtensionAttrsError(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockEntityService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(mockUserService, mockEntityService)

	payload := &SCIMUserPayload{
		UserTypeName:   testUserTypeEmployee,
		ExtensionURN:   "urn:thunderid:params:scim:schemas:employee:2.0:User",
		ExtensionAttrs: map[string]json.RawMessage{"empty": []byte("")},
	}

	mockEntityService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))

	mockEntityService.On(
		"GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
	).Return(&entitytype.EntityType{Name: testUserTypeEmployee, OUID: testOUID}, (*tidcommon.ServiceError)(nil))

	scimUser, err := service.CreateUser(context.Background(), payload, testBaseURL)

	require.Nil(t, scimUser)
	require.Equal(t, ErrorInvalidRequestBody.Code, err.Code)
}

func TestReplaceUser_MarshalExtensionAttrsError(t *testing.T) {
	mockUserService := usermock.NewUserServiceInterfaceMock(t)
	mockEntityService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	service := newSCIMUsersService(mockUserService, mockEntityService)

	payload := &SCIMUserPayload{
		UserTypeName:   testUserTypeEmployee,
		ExtensionURN:   "urn:thunderid:params:scim:schemas:employee:2.0:User",
		ExtensionAttrs: map[string]json.RawMessage{"empty": []byte("")},
	}

	mockEntityService.On(
		"GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, 100, 0, false,
	).Return(makeEntityTypeListPage(), (*tidcommon.ServiceError)(nil))

	mockUserService.On("GetUser", mock.Anything, "user-123", false).
		Return(&user.User{ID: "user-123", Type: testUserTypeEmployee}, (*tidcommon.ServiceError)(nil))

	mockEntityService.On(
		"GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, testUserTypeEmployee,
	).Return(&entitytype.EntityType{Name: testUserTypeEmployee, OUID: testOUID}, (*tidcommon.ServiceError)(nil))

	scimUser, err := service.ReplaceUser(context.Background(), "user-123", payload, "", testBaseURL)

	require.Nil(t, scimUser)
	require.Equal(t, ErrorInvalidRequestBody.Code, err.Code)
}

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
			expectedError: &ErrorConflictingAttributeValue,
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
