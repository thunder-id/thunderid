// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"slices"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	entitypkg "github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/identitygovernance"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/identitygovernancemock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
	"github.com/thunder-id/thunderid/tests/mocks/sharingmock"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

const (
	svcTestUserID1            = "user-1"
	svcTestUserID123          = "user-123"
	svcTestDeclarativeUserID1 = "declarative-user-1"
	testUserType              = "employee"
)
const testOrgID = "11111111-1111-1111-1111-111111111111"

// newAllowAllAuthz returns a mock SystemAuthorizationServiceInterface that allows all actions.
func newAllowAllAuthz(t interface {
	mock.TestingT
	Cleanup(func())
}) *sysauthzmock.SystemAuthorizationServiceInterfaceMock {
	authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
	authzMock.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
		Return(true, nil).Maybe()
	authzMock.On("GetAccessibleResources", mock.Anything, mock.Anything, mock.Anything).
		Return(&sysauthz.AccessibleResources{AllAllowed: true}, nil).Maybe()
	return authzMock
}

func TestOUStore_ValidateOrganizationUnitForUserType(t *testing.T) {
	type testMocks struct {
		ouService         *oumock.OrganizationUnitServiceInterfaceMock
		entityTypeService *entitytypemock.EntityTypeServiceInterfaceMock
	}

	setupParentCheckError := func(t *testing.T, errCode string) (*userService, testMocks) {
		parentOU := "0a08d914-d223-48c2-8939-55d719739a17"
		ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
		ouServiceMock.On("IsOrganizationUnitExists",
			mock.Anything, "d9e12416-58d3-4c17-a4e4-cc4d96122598").
			Return(true, (*tidcommon.ServiceError)(nil)).
			Once()
		ouServiceMock.On("IsParent", mock.Anything, parentOU,
			"d9e12416-58d3-4c17-a4e4-cc4d96122598").Return(false, &tidcommon.ServiceError{
			Code: errCode,
		}).Once()

		entityTypeMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
		entityTypeMock.
			On("GetEntityTypeByHandle", mock.Anything, mock.Anything, testUserType).
			Return(&entitytype.EntityType{
				OUID: parentOU,
			}, (*tidcommon.ServiceError)(nil)).
			Once()

		return &userService{
			ouService:         ouServiceMock,
			entityTypeService: entityTypeMock,
		}, testMocks{
			ouService:         ouServiceMock,
			entityTypeService: entityTypeMock,
		}
	}

	testCases := []struct {
		name        string
		userType    string
		ouID        string
		setup       func(t *testing.T) (*userService, testMocks)
		expectedErr *tidcommon.ServiceError
	}{
		{
			name:     "ReturnsErrorWhenIDEmpty",
			userType: testUserType,
			ouID:     "",
			setup: func(t *testing.T) (*userService, testMocks) {
				return &userService{}, testMocks{}
			},
			expectedErr: &ErrorInvalidOUID,
		},
		{
			name:     "ReturnsInternalErrorWhenOUServiceMissing",
			userType: testUserType,
			ouID:     "invalid-id",
			setup: func(t *testing.T) (*userService, testMocks) {
				return &userService{}, testMocks{}
			},
			expectedErr: &tidcommon.InternalServerError,
		},
		{
			name:     "ReturnsErrorWhenOrganizationUnitMissing",
			userType: testUserType,
			ouID:     "4d8b40d6-3a17-4c19-9a94-5866df9b6bf5",
			setup: func(t *testing.T) (*userService, testMocks) {
				ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
				ouServiceMock.On("IsOrganizationUnitExists",
					mock.Anything, "4d8b40d6-3a17-4c19-9a94-5866df9b6bf5").
					Return(false, (*tidcommon.ServiceError)(nil)).
					Once()

				return &userService{
					ouService: ouServiceMock,
				}, testMocks{
					ouService: ouServiceMock,
				}
			},
			expectedErr: &ErrorOrganizationUnitNotFound,
		},
		{
			name:     "HandlesClientErrorWhenOrganizationUnitMissing",
			userType: testUserType,
			ouID:     "6c8f5afd-8884-4ea0-a317-3d8579346d86",
			setup: func(t *testing.T) (*userService, testMocks) {
				ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
				ouServiceMock.On("IsOrganizationUnitExists",
					mock.Anything, "6c8f5afd-8884-4ea0-a317-3d8579346d86").Return(false, &tidcommon.ServiceError{
					Type: tidcommon.ClientErrorType,
					Code: oupkg.ErrorOrganizationUnitNotFound.Code,
				}).Once()

				return &userService{
					ouService: ouServiceMock,
				}, testMocks{
					ouService: ouServiceMock,
				}
			},
			expectedErr: &ErrorOrganizationUnitNotFound,
		},
		{
			name:     "HandlesClientErrorWhenOUIDInvalid",
			userType: testUserType,
			ouID:     "8d0c2f4e-8bb1-40bc-a0e1-ca5c4aacff63",
			setup: func(t *testing.T) (*userService, testMocks) {
				ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
				ouServiceMock.On("IsOrganizationUnitExists",
					mock.Anything, "8d0c2f4e-8bb1-40bc-a0e1-ca5c4aacff63").Return(false, &tidcommon.ServiceError{
					Type: tidcommon.ClientErrorType,
					Code: oupkg.ErrorInvalidRequestFormat.Code,
				}).Once()

				return &userService{
					ouService: ouServiceMock,
				}, testMocks{
					ouService: ouServiceMock,
				}
			},
			expectedErr: &ErrorInvalidOUID,
		},
		{
			name:     "ReturnsMismatchWhenSchemaDoesNotMatchOU",
			userType: testUserType,
			ouID:     "f4e7c7b2-0b11-46a4-83be-4b43a7f69c7e",
			setup: func(t *testing.T) (*userService, testMocks) {
				parentOU := "a88cbecc-53a3-4c3e-958f-7ee4bf2d7a28"
				ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
				ouServiceMock.On("IsOrganizationUnitExists",
					mock.Anything, "f4e7c7b2-0b11-46a4-83be-4b43a7f69c7e").
					Return(true, (*tidcommon.ServiceError)(nil)).
					Once()
				ouServiceMock.
					On("IsParent", mock.Anything, parentOU, "f4e7c7b2-0b11-46a4-83be-4b43a7f69c7e").
					Return(false, (*tidcommon.ServiceError)(nil)).
					Once()

				entityTypeMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
				entityTypeMock.
					On("GetEntityTypeByHandle", mock.Anything, mock.Anything, testUserType).
					Return(&entitytype.EntityType{
						OUID: parentOU,
					}, (*tidcommon.ServiceError)(nil)).
					Once()

				return &userService{
					ouService:         ouServiceMock,
					entityTypeService: entityTypeMock,
				}, testMocks{
					ouService:         ouServiceMock,
					entityTypeService: entityTypeMock,
				}
			},
			expectedErr: &ErrorOrganizationUnitMismatch,
		},
		{
			name:     "AllowsChildOrganizationUnit",
			userType: testUserType,
			ouID:     "1b5c7208-0d6f-4d5d-8fb9-6e8573549533",
			setup: func(t *testing.T) (*userService, testMocks) {
				parentOU := "c7e99c3b-e563-4c47-981f-1f7f755c8c68"
				ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
				ouServiceMock.On("IsOrganizationUnitExists",
					mock.Anything, "1b5c7208-0d6f-4d5d-8fb9-6e8573549533").
					Return(true, (*tidcommon.ServiceError)(nil)).
					Once()
				ouServiceMock.On("IsParent", mock.Anything, parentOU,
					"1b5c7208-0d6f-4d5d-8fb9-6e8573549533").Return(true, (*tidcommon.ServiceError)(nil)).Once()

				entityTypeMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
				entityTypeMock.
					On("GetEntityTypeByHandle", mock.Anything, mock.Anything, testUserType).
					Return(&entitytype.EntityType{
						OUID: parentOU,
					}, (*tidcommon.ServiceError)(nil)).
					Once()

				return &userService{
					ouService:         ouServiceMock,
					entityTypeService: entityTypeMock,
				}, testMocks{
					ouService:         ouServiceMock,
					entityTypeService: entityTypeMock,
				}
			},
			expectedErr: nil,
		},
		{
			name:     "HandlesParentCheckErrorsOrganizationUnitNotFound",
			userType: testUserType,
			ouID:     "d9e12416-58d3-4c17-a4e4-cc4d96122598",
			setup: func(t *testing.T) (*userService, testMocks) {
				return setupParentCheckError(t, oupkg.ErrorOrganizationUnitNotFound.Code)
			},
			expectedErr: &ErrorOrganizationUnitNotFound,
		},
		{
			name:     "HandlesParentCheckErrorsInternalServerError",
			userType: testUserType,
			ouID:     "d9e12416-58d3-4c17-a4e4-cc4d96122598",
			setup: func(t *testing.T) (*userService, testMocks) {
				return setupParentCheckError(t, tidcommon.InternalServerError.Code)
			},
			expectedErr: &tidcommon.InternalServerError,
		},
		{
			name:     "ReturnsNilWhenValid",
			userType: testUserType,
			ouID:     "e5c3aa8a-d7df-46f8-9f3f-bb3245c95d7c",
			setup: func(t *testing.T) (*userService, testMocks) {
				ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
				ouServiceMock.On("IsOrganizationUnitExists",
					mock.Anything, "e5c3aa8a-d7df-46f8-9f3f-bb3245c95d7c").
					Return(true, (*tidcommon.ServiceError)(nil)).
					Once()

				entityTypeMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
				entityTypeMock.
					On("GetEntityTypeByHandle", mock.Anything, mock.Anything, testUserType).
					Return(&entitytype.EntityType{
						OUID: "e5c3aa8a-d7df-46f8-9f3f-bb3245c95d7c",
					}, (*tidcommon.ServiceError)(nil)).
					Once()

				return &userService{
					ouService:         ouServiceMock,
					entityTypeService: entityTypeMock,
				}, testMocks{
					ouService:         ouServiceMock,
					entityTypeService: entityTypeMock,
				}
			},
			expectedErr: nil,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			service, _ := tc.setup(t)
			logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "UserServiceTest"))

			err := service.validateOrganizationUnitForUserType(context.Background(), tc.userType, tc.ouID, logger)
			if tc.expectedErr == nil {
				require.Nil(t, err)
				return
			}

			require.NotNil(t, err)
			require.Equal(t, *tc.expectedErr, *err)
		})
	}
}

// A filter on the tree path is refused, an organization unit filter and an attribute filter alike,
// before the organization unit is resolved.
func TestUserService_GetUsersByPath_RefusesFilters(t *testing.T) {
	for name, filters := range map[string]map[string]interface{}{
		"organization unit": {"ouId": "ou-1"},
		"attribute":         {"username": "alice"},
	} {
		t.Run(name, func(t *testing.T) {
			service := &userService{ouService: oumock.NewOrganizationUnitServiceInterfaceMock(t)}

			resp, err := service.GetUsersByPath(context.Background(), "root", 10, 0, filters, false)

			require.Nil(t, resp)
			require.Equal(t, &ErrorInvalidFilter, err)
		})
	}
}

func TestUserService_GetUsersByPath_HandlesOUServiceErrors(t *testing.T) {
	testCases := []struct {
		name        string
		setup       func(t *testing.T) *userService
		expectedErr *tidcommon.ServiceError
	}{
		{
			name: "ReturnsInvalidHandlePathWhenResolverFails",
			setup: func(t *testing.T) *userService {
				ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
				ouServiceMock.
					On("GetOrganizationUnitByPath", mock.Anything, "root").
					Return(oupkg.OrganizationUnit{}, &tidcommon.ServiceError{
						Type: tidcommon.ClientErrorType,
						Code: oupkg.ErrorInvalidHandlePath.Code,
					}).
					Once()

				return &userService{
					ouService: ouServiceMock,
				}
			},
			expectedErr: &ErrorInvalidHandlePath,
		},
		{
			name: "ReturnsServerErrorWhenListingUsersFails",
			setup: func(t *testing.T) *userService {
				ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
				ouServiceMock.
					On("GetOrganizationUnitByPath", mock.Anything, "root").
					Return(oupkg.OrganizationUnit{ID: "ou-id"}, (*tidcommon.ServiceError)(nil)).
					Once()
				storeMock := entitymock.NewEntityServiceInterfaceMock(t)
				storeMock.On("GetEntityPageInOU", mock.Anything, providers.EntityCategoryUser, "ou-id",
					entitypkg.OUEntityScopeAll, 10, 0, (map[string]interface{})(nil)).
					Return(nil, errors.New("db error")).Once()

				return &userService{
					ouService:     ouServiceMock,
					authzService:  newAllowAllAuthz(t),
					entityService: storeMock,
				}
			},
			expectedErr: &tidcommon.InternalServerError,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			service := tc.setup(t)

			resp, err := service.GetUsersByPath(context.Background(), "root", 10, 0, nil, false)
			require.Nil(t, resp)
			require.NotNil(t, err)
			require.Equal(t, *tc.expectedErr, *err)
		})
	}
}

func TestUserService_CreateUserByPath_HandlesOUServiceErrors(t *testing.T) {
	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	ouServiceMock.
		On("GetOrganizationUnitByPath", mock.Anything, "root/engineering").
		Return(oupkg.OrganizationUnit{}, &tidcommon.ServiceError{
			Type: tidcommon.ClientErrorType,
			Code: oupkg.ErrorInvalidHandlePath.Code,
		}).
		Once()

	service := &userService{
		ouService: ouServiceMock,
	}

	resp, err := service.CreateUserByPath(context.Background(), "root/engineering", CreateUserByPathRequest{
		Type: testUserType,
	})
	require.Nil(t, resp)
	require.NotNil(t, err)
	require.Equal(t, ErrorInvalidHandlePath, *err)
}

func TestUserService_CreateUser_CallsCreateEntity(t *testing.T) {
	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	ouServiceMock.On("IsOrganizationUnitExists", mock.Anything, testOrgID).
		Return(true, (*tidcommon.ServiceError)(nil)).
		Once()

	entityTypeMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	entityTypeMock.On("GetEntityTypeByHandle", mock.Anything, mock.Anything, testUserType).
		Return(&entitytype.EntityType{OUID: testOrgID}, (*tidcommon.ServiceError)(nil)).
		Once()

	storeMock := entitymock.NewEntityServiceInterfaceMock(t)
	storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
	storeMock.
		On("CreateEntity", mock.Anything, mock.MatchedBy(func(e *providers.Entity) bool {
			return e.OUID == testOrgID && e.Type == testUserType && e.ID != ""
		}), mock.Anything, mock.Anything).
		Return(&providers.Entity{
			OUID: testOrgID, Type: testUserType,
			Attributes: json.RawMessage(`{}`),
		}, nil).
		Once()

	service := &userService{
		entityService:     storeMock,
		ouService:         ouServiceMock,
		entityTypeService: entityTypeMock,
		authzService:      newAllowAllAuthz(t),
		uuidGenerator:     utils.GenerateUUIDv7,
	}

	user := &providers.User{
		Type:       testUserType,
		OUID:       testOrgID,
		Attributes: json.RawMessage(`{}`),
	}

	created, err := service.CreateUser(context.Background(), user)
	require.Nil(t, err)
	require.NotNil(t, created)
	require.Equal(t, testOrgID, created.OUID)
	require.NotEmpty(t, created.ID)
	storeMock.AssertNumberOfCalls(t, "CreateEntity", 1)
}

// A user created with a federated link carries it to the entity layer as the linkedIds system
// attribute, so the user and its link are written together.
func TestUserService_CreateUser_WritesLinkedAccount(t *testing.T) {
	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	ouServiceMock.On("IsOrganizationUnitExists", mock.Anything, testOrgID).
		Return(true, (*tidcommon.ServiceError)(nil)).Once()
	entityTypeMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	entityTypeMock.On("GetEntityTypeByHandle", mock.Anything, mock.Anything, testUserType).
		Return(&entitytype.EntityType{OUID: testOrgID}, (*tidcommon.ServiceError)(nil)).Once()
	storeMock := entitymock.NewEntityServiceInterfaceMock(t)
	storeMock.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything,
		&providers.LinkedAccount{IdpID: "idp-a", Sub: "sub-1"}).
		Return(&providers.Entity{Attributes: json.RawMessage(`{}`)}, nil).Once()

	service := &userService{
		entityService:     storeMock,
		ouService:         ouServiceMock,
		entityTypeService: entityTypeMock,
		authzService:      newAllowAllAuthz(t),
		uuidGenerator:     utils.GenerateUUIDv7,
	}

	created, svcErr := service.CreateUser(context.Background(), &providers.User{
		Type:          testUserType,
		OUID:          testOrgID,
		Attributes:    json.RawMessage(`{}`),
		LinkedAccount: &providers.LinkedAccount{IdpID: "idp-a", Sub: "sub-1"},
	})
	require.Nil(t, svcErr)
	require.NotNil(t, created)
}

// A federated link missing its connection or subject names nothing, so the create is rejected
// before anything is written.
func TestUserService_CreateUser_RejectsIncompleteLinkedAccount(t *testing.T) {
	for name, link := range map[string]providers.LinkedAccount{
		"no connection": {Sub: "sub-1"},
		"no subject":    {IdpID: "idp-a"},
	} {
		t.Run(name, func(t *testing.T) {
			service := &userService{}

			created, svcErr := service.CreateUser(context.Background(), &providers.User{
				Type: testUserType, OUID: testOrgID, LinkedAccount: &link,
			})
			require.Nil(t, created)
			require.NotNil(t, svcErr)
			require.Equal(t, ErrorInvalidRequestFormat.Code, svcErr.Code)
		})
	}
}

func TestUserService_CreateUser_UUIDGenerationError(t *testing.T) {
	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	ouServiceMock.On("IsOrganizationUnitExists", mock.Anything, testOrgID).
		Return(true, (*tidcommon.ServiceError)(nil)).Once()

	entityTypeMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	entityTypeMock.On("GetEntityTypeByHandle", mock.Anything, mock.Anything, testUserType).
		Return(&entitytype.EntityType{OUID: testOrgID}, (*tidcommon.ServiceError)(nil)).Once()

	service := &userService{
		ouService:         ouServiceMock,
		entityTypeService: entityTypeMock,
		authzService:      newAllowAllAuthz(t),
		uuidGenerator: func() (string, error) {
			return "", errors.New("entropy source failed")
		},
	}

	user := &providers.User{Type: testUserType, OUID: testOrgID}

	created, svcErr := service.CreateUser(context.Background(), user)
	require.Nil(t, created)
	require.NotNil(t, svcErr)
	require.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)
}

func TestUserService_CreateUser_PropagatesStoreError(t *testing.T) {
	storeErr := errors.New("store failure")

	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	ouServiceMock.On("IsOrganizationUnitExists", mock.Anything, testOrgID).
		Return(true, (*tidcommon.ServiceError)(nil)).
		Once()

	entityTypeMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	entityTypeMock.On("GetEntityTypeByHandle", mock.Anything, mock.Anything, testUserType).
		Return(&entitytype.EntityType{OUID: testOrgID}, (*tidcommon.ServiceError)(nil)).
		Once()

	storeMock := entitymock.NewEntityServiceInterfaceMock(t)
	storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
	storeMock.
		On("CreateEntity", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return((*providers.Entity)(nil), storeErr).
		Once()

	service := &userService{
		entityService:     storeMock,
		ouService:         ouServiceMock,
		entityTypeService: entityTypeMock,
		authzService:      newAllowAllAuthz(t),
		uuidGenerator:     utils.GenerateUUIDv7,
	}

	user := &providers.User{
		Type:       testUserType,
		OUID:       testOrgID,
		Attributes: json.RawMessage(`{}`),
	}

	created, svcErr := service.CreateUser(context.Background(), user)
	require.Nil(t, created)
	require.NotNil(t, svcErr)
	require.Equal(t, tidcommon.InternalServerError, *svcErr)
	storeMock.AssertNumberOfCalls(t, "CreateEntity", 1)
}

func TestUserService_UpdateUserCredentials_Validation(t *testing.T) {
	t.Run("ReturnsAuthErrorWhenUserIDMissing", func(t *testing.T) {
		service := &userService{}

		err := service.UpdateUserCredentials(context.Background(), "", json.RawMessage(`{"password":"newpass"}`))
		require.NotNil(t, err)
		require.Equal(t, ErrorAuthenticationFailed, *err)
	})

	t.Run("ReturnsMissingCredentialsWhenPayloadEmpty", func(t *testing.T) {
		service := &userService{}

		err := service.UpdateUserCredentials(context.Background(), svcTestUserID1, json.RawMessage(``))
		require.NotNil(t, err)
		require.Equal(t, ErrorMissingCredentials, *err)
	})

	t.Run("ReturnsInvalidRequestFormatWhenInvalidJSON", func(t *testing.T) {
		service := &userService{}

		err := service.UpdateUserCredentials(context.Background(), svcTestUserID1, json.RawMessage(`invalid json`))
		require.NotNil(t, err)
		require.Equal(t, ErrorInvalidRequestFormat, *err)
	})

	t.Run("ReturnsInvalidCredentialForUnsupportedType", func(t *testing.T) {
		userStoreMock := entitymock.NewEntityServiceInterfaceMock(t)
		userStoreMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
		userStoreMock.
			On("GetEntityProfile", mock.Anything, svcTestUserID1).
			Return(&providers.Entity{
				Category: providers.EntityCategoryUser, ID: svcTestUserID1, Type: "Person",
			}, nil).
			Once()
		userStoreMock.
			On("UpdateCredentials", mock.Anything, svcTestUserID1, mock.Anything).
			Return(entitypkg.ErrInvalidCredential).
			Once()

		service := &userService{
			entityService: userStoreMock,
			authzService:  newAllowAllAuthz(t),
		}

		err := service.UpdateUserCredentials(context.Background(), svcTestUserID1,
			json.RawMessage(`{"invalidtype":"value"}`))
		require.NotNil(t, err)
		require.Equal(t, ErrorInvalidCredential.Code, err.Code)
	})

	t.Run("ReturnsMissingCredentialsWhenMapEmpty", func(t *testing.T) {
		service := &userService{}

		err := service.UpdateUserCredentials(context.Background(), svcTestUserID1, json.RawMessage(`{}`))
		require.NotNil(t, err)
		require.Equal(t, ErrorMissingCredentials, *err)
	})
}

func TestUserService_UpdateUserCredentials_UserNotFound(t *testing.T) {
	userStoreMock := entitymock.NewEntityServiceInterfaceMock(t)
	userStoreMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
	userStoreMock.
		On("GetEntityProfile", mock.Anything, svcTestUserID1).
		Return((*providers.Entity)(nil), entitypkg.ErrEntityNotFound).
		Once()

	service := &userService{
		entityService: userStoreMock,
	}

	credentialsJSON := json.RawMessage(`{"password":"newpassword"}`)
	svcErr := service.UpdateUserCredentials(context.Background(), svcTestUserID1, credentialsJSON)
	require.NotNil(t, svcErr)
	require.Equal(t, ErrorUserNotFound, *svcErr)
	userStoreMock.AssertNotCalled(t, "UpdateSystemCredentials", mock.Anything, mock.Anything, mock.Anything)
}

func TestUserService_UpdateUserCredentials_Succeeds(t *testing.T) {
	userStoreMock := entitymock.NewEntityServiceInterfaceMock(t)
	userStoreMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
	userStoreMock.
		On("GetEntityProfile", mock.Anything, svcTestUserID1).
		Return(&providers.Entity{
			Category: providers.EntityCategoryUser, ID: svcTestUserID1, Type: "Person",
		}, nil).
		Once()

	var capturedJSON json.RawMessage
	userStoreMock.
		On("UpdateCredentials", mock.Anything, svcTestUserID1, mock.Anything).
		Run(func(args mock.Arguments) {
			capturedJSON = args.Get(2).(json.RawMessage)
		}).
		Return(nil).
		Once()

	service := &userService{
		entityService: userStoreMock,
		authzService:  newAllowAllAuthz(t),
	}

	// Send plain text password - entity service will hash it
	credentialsJSON := json.RawMessage(`{"password":"newpassword"}`)
	svcErr := service.UpdateUserCredentials(context.Background(), svcTestUserID1, credentialsJSON)
	require.Nil(t, svcErr)

	// Verify plaintext was passed to UpdateCredentials (schema credentials column)
	var plaintextMap map[string]interface{}
	require.NoError(t, json.Unmarshal(capturedJSON, &plaintextMap))
	require.Equal(t, "newpassword", plaintextMap["password"])

	userStoreMock.AssertNumberOfCalls(t, "UpdateCredentials", 1)
}

func TestUserService_UpdateUserCredentials_Rejections(t *testing.T) {
	tests := []struct {
		name          string
		payload       string
		mockEntityErr error
		wantErrCode   string
	}{
		{
			// Passkeys are system-managed credentials and must be rejected explicitly
			// at the user service level before reaching the entity service.
			name:        "RejectsPasskeys",
			payload:     `{"passkey":"passkey-credential-1"}`,
			wantErrCode: ErrorInvalidCredential.Code,
		},
		{
			// Schema credentials must be plain strings; arrays/objects fail JSON unmarshal
			// to string in the user service before reaching entity service.
			name:        "RejectsStructuredPasswordValues",
			payload:     `{"password":[{"value":"password1"}, {"value":"password2"}]}`,
			wantErrCode: ErrorInvalidRequestFormat.Code,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			userStoreMock := entitymock.NewEntityServiceInterfaceMock(t)
			userStoreMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
			userStoreMock.
				On("GetEntityProfile", mock.Anything, svcTestUserID1).
				Return(&providers.Entity{
					Category: providers.EntityCategoryUser, ID: svcTestUserID1, Type: "Person",
				}, nil).
				Maybe()

			service := &userService{
				entityService: userStoreMock,
				authzService:  newAllowAllAuthz(t),
			}

			svcErr := service.UpdateUserCredentials(
				context.Background(), svcTestUserID1, json.RawMessage(tc.payload))
			require.NotNil(t, svcErr)
			require.Equal(t, tc.wantErrCode, svcErr.Code)
		})
	}
}

func TestUserService_UpdateUserAttributes_Validation(t *testing.T) {
	service := &userService{}

	resp, err := service.UpdateUserAttributes(context.Background(), "", json.RawMessage(`{"email":"a@b.com"}`))
	require.Nil(t, resp)
	require.NotNil(t, err)
	require.Equal(t, ErrorMissingUserID, *err)

	resp, err = service.UpdateUserAttributes(context.Background(), svcTestUserID1, json.RawMessage{})
	require.Nil(t, resp)
	require.NotNil(t, err)
	require.Equal(t, ErrorInvalidRequestFormat, *err)
}

func TestUserService_UpdateUserAttributes_UserNotFound(t *testing.T) {
	storeMock := entitymock.NewEntityServiceInterfaceMock(t)
	storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
	storeMock.On("GetEntity", mock.Anything, svcTestUserID1).
		Return((*providers.Entity)(nil), entitypkg.ErrEntityNotFound).Once()

	service := &userService{
		entityService: storeMock,
	}

	resp, err := service.UpdateUserAttributes(context.Background(), svcTestUserID1,
		json.RawMessage(`{"email":"a@b.com"}`))
	require.Nil(t, resp)
	require.NotNil(t, err)
	require.Equal(t, ErrorUserNotFound, *err)
	storeMock.AssertNotCalled(t, "UpdateEntity", mock.Anything, mock.Anything, mock.Anything)
}

func TestUserService_UpdateUserAttributes_SchemaValidationFails(t *testing.T) {
	storeMock := entitymock.NewEntityServiceInterfaceMock(t)
	storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
	storeMock.
		On("GetEntity", mock.Anything, svcTestUserID1).
		Return(&providers.Entity{
			Category: providers.EntityCategoryUser, ID: svcTestUserID1, Type: testUserType,
			Attributes: json.RawMessage(`{"email":"old"}`),
		}, nil)
	storeMock.
		On("UpdateAttributes", mock.Anything, svcTestUserID1, mock.Anything).
		Return(entitypkg.ErrSchemaValidationFailed).
		Once()

	schemaMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	schemaMock.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		entitytype.AttributeFilter{AllowCredential: true}).
		Return([]entitytype.AttributeInfo{{Attribute: "password"}}, (*tidcommon.ServiceError)(nil)).Once()

	service := &userService{
		entityService:     storeMock,
		entityTypeService: schemaMock,
		authzService:      newAllowAllAuthz(t),
	}

	resp, err := service.UpdateUserAttributes(context.Background(), svcTestUserID1,
		json.RawMessage(`{"email":"new@example.com"}`))
	require.Nil(t, resp)
	require.NotNil(t, err)
	require.Equal(t, ErrorSchemaValidationFailed, *err)
}

func TestUserService_UpdateUserAttributes_Succeeds(t *testing.T) {
	storeMock := entitymock.NewEntityServiceInterfaceMock(t)
	storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
	storeMock.
		On("GetEntity", mock.Anything, svcTestUserID1).
		Return(&providers.Entity{Category: providers.EntityCategoryUser, ID: svcTestUserID1, Type: testUserType,
			Attributes: json.RawMessage(`{"email":"old@example.com"}`)}, nil)
	storeMock.
		On("UpdateAttributes", mock.Anything, svcTestUserID1, mock.Anything).
		Return(nil).
		Once()

	schemaMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	schemaMock.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		entitytype.AttributeFilter{AllowCredential: true}).
		Return([]entitytype.AttributeInfo{{Attribute: "password"}}, (*tidcommon.ServiceError)(nil)).Once()

	service := &userService{
		entityService:     storeMock,
		entityTypeService: schemaMock,
		authzService:      newAllowAllAuthz(t),
		governance:        activeGovernance(t),
	}

	newAttrs := json.RawMessage(`{"email":"new@example.com"}`)
	resp, err := service.UpdateUserAttributes(context.Background(), svcTestUserID1, newAttrs)
	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Equal(t, svcTestUserID1, resp.ID)
	require.JSONEq(t, string(newAttrs), string(resp.Attributes))
}

func TestUserService_UpdateUserAttributes_RejectsCredentialAttributes(t *testing.T) {
	storeMock := entitymock.NewEntityServiceInterfaceMock(t)
	storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
	storeMock.
		On("GetEntity", mock.Anything, svcTestUserID1).
		Return(&providers.Entity{Category: providers.EntityCategoryUser, ID: svcTestUserID1, Type: testUserType,
			Attributes: json.RawMessage(`{"email":"old@example.com"}`)}, nil).Once()

	schemaMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	schemaMock.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		entitytype.AttributeFilter{AllowCredential: true}).
		Return([]entitytype.AttributeInfo{{Attribute: "password"}}, (*tidcommon.ServiceError)(nil)).Once()

	service := &userService{
		entityService:     storeMock,
		entityTypeService: schemaMock,
		authzService:      newAllowAllAuthz(t),
	}

	resp, err := service.UpdateUserAttributes(context.Background(), svcTestUserID1,
		json.RawMessage(`{"email":"new@example.com","password":"secret"}`))
	require.Nil(t, resp)
	require.NotNil(t, err)
	require.Equal(t, ErrorInvalidRequestFormat.Code, err.Code)
}

func TestUserService_UpdateUserAttributes_NilSchemaService(t *testing.T) {
	storeMock := entitymock.NewEntityServiceInterfaceMock(t)
	storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
	storeMock.
		On("GetEntity", mock.Anything, svcTestUserID1).
		Return(&providers.Entity{Category: providers.EntityCategoryUser, ID: svcTestUserID1, Type: testUserType,
			Attributes: json.RawMessage(`{"email":"old@example.com"}`)}, nil).Once()

	service := &userService{
		entityService: storeMock,
		authzService:  newAllowAllAuthz(t),
	}

	resp, err := service.UpdateUserAttributes(context.Background(), svcTestUserID1,
		json.RawMessage(`{"email":"new@example.com"}`))
	require.Nil(t, resp)
	require.NotNil(t, err)
	require.Equal(t, tidcommon.InternalServerError.Code, err.Code)
}

func TestUserService_GetUser_ReturnsUser(t *testing.T) {
	userID := svcTestUserID1
	expectedEntity := &providers.Entity{
		Category: providers.EntityCategoryUser, ID: userID, OUID: testOrgID,
	}

	storeMock := entitymock.NewEntityServiceInterfaceMock(t)
	storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
	storeMock.On("GetEntity", mock.Anything, userID).Return(expectedEntity, nil).Once()

	service := &userService{
		entityService: storeMock,
		authzService:  newAllowAllAuthz(t),
		governance:    activeGovernance(t),
	}

	user, err := service.GetUser(context.Background(), userID, false)
	require.Nil(t, err)
	require.Equal(t, userID, user.ID)
	require.Equal(t, testOrgID, user.OUID)
}

func TestUserService_GetUser_WithIncludeDisplay(t *testing.T) {
	userID := svcTestUserID1
	expectedEntity := &providers.Entity{
		Category:   providers.EntityCategoryUser,
		ID:         userID,
		OUID:       testOrgID,
		Type:       "employee",
		Attributes: json.RawMessage(`{"email":"alice@example.com"}`),
	}

	storeMock := entitymock.NewEntityServiceInterfaceMock(t)
	storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
	storeMock.On("GetEntity", mock.Anything, userID).Return(expectedEntity, nil).Once()

	mockSchema := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockSchema.On("GetDisplayAttributesByHandles", mock.Anything, mock.Anything, []string{"employee"}).
		Return(map[string]string{"employee": "email"}, nil).Once()

	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	ouServiceMock.On("GetOrganizationUnitHandlesByIDs", mock.Anything, []string{testOrgID}).
		Return(map[string]string{testOrgID: "test-ou"}, nil).Once()

	service := &userService{
		entityService:     storeMock,
		authzService:      newAllowAllAuthz(t),
		entityTypeService: mockSchema,
		ouService:         ouServiceMock,
		governance:        activeGovernance(t),
	}

	user, err := service.GetUser(context.Background(), userID, true)
	require.Nil(t, err)
	require.Equal(t, "alice@example.com", user.Display)
	require.Equal(t, "test-ou", user.OUHandle)
}

func TestUserService_DeleteUser(t *testing.T) {
	userID := svcTestUserID1

	storeMock := entitymock.NewEntityServiceInterfaceMock(t)
	storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
	storeMock.On("GetEntityProfile", mock.Anything, userID).
		Return(&providers.Entity{
			Category: providers.EntityCategoryUser, ID: userID, OUID: testOrgID,
		}, nil).Once()
	storeMock.On("DeleteEntity", mock.Anything, userID).Return(nil).Once()

	service := &userService{
		entityService:      storeMock,
		authzService:       newAllowAllAuthz(t),
		dependencyRegistry: newNoBlockingDepsRegistry(),
		sharingService:     newPurgingSharing(t, userID),
	}

	err := service.DeleteUser(context.Background(), userID)
	require.Nil(t, err)
	storeMock.AssertNumberOfCalls(t, "DeleteEntity", 1)
}

func TestUserService_UpdateUser(t *testing.T) {
	userID := svcTestUserID1
	updatedUser := providers.User{ID: userID, OUID: testOrgID, Type: testUserType,
		Attributes: json.RawMessage(`{"updated":"true"}`),
		Status:     &providers.AccessStatusReport{Value: providers.AccessStatusSuspended}}

	storeMock := entitymock.NewEntityServiceInterfaceMock(t)
	storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()

	// Mock GetUser pre-fetch for authz check
	storeMock.On("GetEntityProfile", mock.Anything, userID).
		Return(&providers.Entity{
			Category: providers.EntityCategoryUser, ID: userID, OUID: testOrgID, Type: testUserType,
		}, nil).Once()

	// Mock UpdateEntity call
	storeMock.On("UpdateEntity", mock.Anything, userID, mock.MatchedBy(func(e *providers.Entity) bool {
		return e.ID == userID
	})).Return(&providers.Entity{
		ID:         userID,
		Attributes: updatedUser.Attributes,
	}, nil).Once()

	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	ouServiceMock.On("IsOrganizationUnitExists", mock.Anything, testOrgID).
		Return(true, (*tidcommon.ServiceError)(nil)).
		Once()

	entityTypeMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	entityTypeMock.On("GetEntityTypeByHandle", mock.Anything, mock.Anything, testUserType).
		Return(&entitytype.EntityType{OUID: testOrgID}, (*tidcommon.ServiceError)(nil)).
		Once()
	entityTypeMock.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		entitytype.AttributeFilter{AllowCredential: true}).
		Return([]entitytype.AttributeInfo{}, (*tidcommon.ServiceError)(nil)).Once()

	service := &userService{
		entityService:     storeMock,
		ouService:         ouServiceMock,
		entityTypeService: entityTypeMock,
		authzService:      newAllowAllAuthz(t),
		governance:        activeGovernance(t),
	}

	resp, err := service.UpdateUser(context.Background(), userID, &updatedUser)
	require.Nil(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Status)
	require.Equal(t, providers.AccessStatusActive, resp.Status.Value, "a status in the request must not be echoed")
	storeMock.AssertNumberOfCalls(t, "UpdateEntity", 1)
}

func TestUserService_UpdateUser_ReportsTheGovernedStatus(t *testing.T) {
	held := identitygovernance.AccessRead{
		Status: providers.AccessStatusReport{Value: providers.AccessStatusSuspended,
			Details: &providers.AccessStatusDetails{Reason: providers.AccessHoldReasonAdministrativeSuspension,
				Since: "2026-10-21T08:00:00Z", OperatorNote: "Under investigation"}},
		LastLoginAt: "2026-10-20T08:00:00Z",
	}
	active := identitygovernance.AccessRead{Status: providers.AccessStatusReport{Value: providers.AccessStatusActive}}
	cases := []struct {
		name      string
		requested providers.AccessStatus
		read      identitygovernance.AccessRead
	}{
		{name: "a held user", requested: providers.AccessStatusActive, read: held},
		{name: "an active user", requested: providers.AccessStatusSuspended, read: active},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			userID := svcTestUserID1
			request := providers.User{ID: userID, OUID: testOrgID, Type: testUserType,
				Attributes: json.RawMessage(`{"updated":"true"}`),
				Status:     &providers.AccessStatusReport{Value: tc.requested}}
			updated := &providers.Entity{Category: providers.EntityCategoryUser, ID: userID, OUID: testOrgID,
				Type: testUserType, Attributes: request.Attributes}

			storeMock := entitymock.NewEntityServiceInterfaceMock(t)
			storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
			storeMock.On("GetEntityProfile", mock.Anything, userID).Return(&providers.Entity{
				Category: providers.EntityCategoryUser, ID: userID, OUID: testOrgID, Type: testUserType,
			}, nil).Once()
			storeMock.On("UpdateEntity", mock.Anything, userID, mock.Anything).Return(updated, nil).Once()
			ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
			ouServiceMock.On("IsOrganizationUnitExists", mock.Anything, testOrgID).
				Return(true, (*tidcommon.ServiceError)(nil)).Once()
			entityTypeMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
			entityTypeMock.On("GetEntityTypeByHandle", mock.Anything, mock.Anything, testUserType).
				Return(&entitytype.EntityType{OUID: testOrgID}, (*tidcommon.ServiceError)(nil)).Once()
			entityTypeMock.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
				entitytype.AttributeFilter{AllowCredential: true}).
				Return([]entitytype.AttributeInfo{}, (*tidcommon.ServiceError)(nil)).Once()
			governance := identitygovernancemock.NewIdentityGovernanceServiceInterfaceMock(t)
			governance.EXPECT().ReadEntityAccess(mock.Anything, *updated).
				Return(tc.read).Once()
			service := &userService{entityService: storeMock, ouService: ouServiceMock,
				entityTypeService: entityTypeMock, authzService: newAllowAllAuthz(t), governance: governance}

			resp, svcErr := service.UpdateUser(context.Background(), userID, &request)

			require.Nil(t, svcErr)
			require.NotNil(t, resp.Status)
			require.Equal(t, tc.read.Status, *resp.Status)
			require.Equal(t, tc.read.LastLoginAt, resp.LastLoginAt)
		})
	}
}

func TestUserService_UpdateUser_RejectsCredentialAttributes(t *testing.T) {
	userID := svcTestUserID1

	updatedUser := providers.User{
		ID:         userID,
		OUID:       testOrgID,
		Type:       testUserType,
		Attributes: json.RawMessage(`{"email":"test@example.com","password":"newPassword123"}`),
	}

	storeMock := entitymock.NewEntityServiceInterfaceMock(t)
	storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	entityTypeMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)

	storeMock.On("GetEntityProfile", mock.Anything, userID).
		Return(&providers.Entity{
			Category: providers.EntityCategoryUser, ID: userID, OUID: testOrgID, Type: testUserType,
		}, nil).Once()

	ouServiceMock.On("IsOrganizationUnitExists", mock.Anything, testOrgID).
		Return(true, (*tidcommon.ServiceError)(nil)).Once()
	entityTypeMock.On("GetEntityTypeByHandle", mock.Anything, mock.Anything, testUserType).
		Return(&entitytype.EntityType{OUID: testOrgID}, (*tidcommon.ServiceError)(nil)).Once()
	entityTypeMock.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		entitytype.AttributeFilter{AllowCredential: true}).
		Return([]entitytype.AttributeInfo{{Attribute: "password"}}, (*tidcommon.ServiceError)(nil)).Once()

	service := &userService{
		entityService:     storeMock,
		ouService:         ouServiceMock,
		entityTypeService: entityTypeMock,
		authzService:      newAllowAllAuthz(t),
	}

	resp, err := service.UpdateUser(context.Background(), userID, &updatedUser)

	require.Nil(t, resp)
	require.NotNil(t, err)
	require.Equal(t, ErrorCredentialUpdateNotAllowed.Code, err.Code)
}

func TestUserService_UpdateUser_ErrorPaths(t *testing.T) {
	userID := svcTestUserID1
	ctx := context.Background()

	tests := []struct {
		name       string
		attributes string
		setupMocks func(
			storeMock *entitymock.EntityServiceInterfaceMock,
			ouServiceMock *oumock.OrganizationUnitServiceInterfaceMock,
			entityTypeMock *entitytypemock.EntityTypeServiceInterfaceMock,
		)
		expectedError *tidcommon.ServiceError
	}{
		{
			name:       "UpdateEntity_EntityNotFound",
			attributes: `{"email":"test@example.com"}`,
			setupMocks: func(
				storeMock *entitymock.EntityServiceInterfaceMock,
				ouServiceMock *oumock.OrganizationUnitServiceInterfaceMock,
				entityTypeMock *entitytypemock.EntityTypeServiceInterfaceMock,
			) {
				ouServiceMock.On("IsOrganizationUnitExists", mock.Anything, testOrgID).
					Return(true, (*tidcommon.ServiceError)(nil)).Maybe()
				entityTypeMock.On("GetEntityTypeByHandle", mock.Anything, mock.Anything, testUserType).
					Return(&entitytype.EntityType{OUID: testOrgID},
						(*tidcommon.ServiceError)(nil)).Maybe()
				entityTypeMock.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
					entitytype.AttributeFilter{AllowCredential: true}).
					Return([]entitytype.AttributeInfo{}, (*tidcommon.ServiceError)(nil)).Maybe()
				storeMock.On("GetEntityProfile", mock.Anything, userID).
					Return(&providers.Entity{
						Category: providers.EntityCategoryUser,
						ID:       userID,
						OUID:     testOrgID,
						Type:     testUserType,
					}, nil).Once()
				storeMock.On("UpdateEntity", mock.Anything, userID, mock.Anything).
					Return((*providers.Entity)(nil), entitypkg.ErrEntityNotFound).Once()
			},
			expectedError: &ErrorUserNotFound,
		},
		{
			name:       "UpdateEntity_StoreError",
			attributes: `{"email":"test@example.com"}`,
			setupMocks: func(
				storeMock *entitymock.EntityServiceInterfaceMock,
				ouServiceMock *oumock.OrganizationUnitServiceInterfaceMock,
				entityTypeMock *entitytypemock.EntityTypeServiceInterfaceMock,
			) {
				ouServiceMock.On("IsOrganizationUnitExists", mock.Anything, testOrgID).
					Return(true, (*tidcommon.ServiceError)(nil)).Maybe()
				entityTypeMock.On("GetEntityTypeByHandle", mock.Anything, mock.Anything, testUserType).
					Return(&entitytype.EntityType{OUID: testOrgID},
						(*tidcommon.ServiceError)(nil)).Maybe()
				entityTypeMock.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
					entitytype.AttributeFilter{AllowCredential: true}).
					Return([]entitytype.AttributeInfo{}, (*tidcommon.ServiceError)(nil)).Maybe()
				storeMock.On("GetEntityProfile", mock.Anything, userID).
					Return(&providers.Entity{
						Category: providers.EntityCategoryUser,
						ID:       userID,
						OUID:     testOrgID,
						Type:     testUserType,
					}, nil).Once()
				storeMock.On("UpdateEntity", mock.Anything, userID, mock.Anything).
					Return((*providers.Entity)(nil), errors.New("db connection lost")).Once()
			},
			expectedError: &tidcommon.InternalServerError,
		},
		{
			name:       "UpdateUser_WithoutCredentials_Success",
			attributes: `{"email":"updated@example.com"}`,
			setupMocks: func(
				storeMock *entitymock.EntityServiceInterfaceMock,
				ouServiceMock *oumock.OrganizationUnitServiceInterfaceMock,
				entityTypeMock *entitytypemock.EntityTypeServiceInterfaceMock,
			) {
				ouServiceMock.On("IsOrganizationUnitExists", mock.Anything, testOrgID).
					Return(true, (*tidcommon.ServiceError)(nil)).Once()
				entityTypeMock.On("GetEntityTypeByHandle", mock.Anything, mock.Anything, testUserType).
					Return(&entitytype.EntityType{OUID: testOrgID},
						(*tidcommon.ServiceError)(nil)).Once()
				entityTypeMock.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
					entitytype.AttributeFilter{AllowCredential: true}).
					Return([]entitytype.AttributeInfo{}, (*tidcommon.ServiceError)(nil)).Once()
				storeMock.On("GetEntityProfile", mock.Anything, userID).
					Return(&providers.Entity{
						Category: providers.EntityCategoryUser,
						ID:       userID,
						OUID:     testOrgID,
						Type:     testUserType,
					}, nil).Once()
				storeMock.On("UpdateEntity", mock.Anything, userID, mock.Anything).
					Return(&providers.Entity{
						Category:   providers.EntityCategoryUser,
						ID:         userID,
						OUID:       testOrgID,
						Type:       testUserType,
						Attributes: json.RawMessage(`{"email":"updated@example.com"}`),
					}, nil).Once()
			},
			expectedError: nil,
		},
		{
			name:       "GetAttributes_EntityTypeNotFound",
			attributes: `{"email":"test@example.com"}`,
			setupMocks: func(
				storeMock *entitymock.EntityServiceInterfaceMock,
				ouServiceMock *oumock.OrganizationUnitServiceInterfaceMock,
				entityTypeMock *entitytypemock.EntityTypeServiceInterfaceMock,
			) {
				storeMock.On("GetEntityProfile", mock.Anything, userID).
					Return(&providers.Entity{
						Category: providers.EntityCategoryUser,
						ID:       userID,
						OUID:     testOrgID,
						Type:     testUserType,
					}, nil).Once()
				ouServiceMock.On("IsOrganizationUnitExists", mock.Anything, testOrgID).
					Return(true, (*tidcommon.ServiceError)(nil)).Once()
				entityTypeMock.On("GetEntityTypeByHandle", mock.Anything, mock.Anything, testUserType).
					Return(&entitytype.EntityType{OUID: testOrgID},
						(*tidcommon.ServiceError)(nil)).Once()
				entityTypeMock.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
					entitytype.AttributeFilter{AllowCredential: true}).
					Return(nil, &entitytype.ErrorEntityTypeNotFound).Once()
			},
			expectedError: &ErrorEntityTypeNotFound,
		},
		{
			name:       "GetAttributes_GenericError",
			attributes: `{"email":"test@example.com"}`,
			setupMocks: func(
				storeMock *entitymock.EntityServiceInterfaceMock,
				ouServiceMock *oumock.OrganizationUnitServiceInterfaceMock,
				entityTypeMock *entitytypemock.EntityTypeServiceInterfaceMock,
			) {
				storeMock.On("GetEntityProfile", mock.Anything, userID).
					Return(&providers.Entity{
						Category: providers.EntityCategoryUser,
						ID:       userID,
						OUID:     testOrgID,
						Type:     testUserType,
					}, nil).Once()
				ouServiceMock.On("IsOrganizationUnitExists", mock.Anything, testOrgID).
					Return(true, (*tidcommon.ServiceError)(nil)).Once()
				entityTypeMock.On("GetEntityTypeByHandle", mock.Anything, mock.Anything, testUserType).
					Return(&entitytype.EntityType{OUID: testOrgID},
						(*tidcommon.ServiceError)(nil)).Once()
				entityTypeMock.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
					entitytype.AttributeFilter{AllowCredential: true}).
					Return(nil, &tidcommon.ServiceError{
						Code:             "USRS-9999",
						ErrorDescription: tidcommon.I18nMessage{DefaultValue: "unexpected schema error"},
					}).Once()
			},
			expectedError: &tidcommon.InternalServerError,
		},
		{
			name:       "CredentialCheck_InvalidJSON",
			attributes: `{not valid json`,
			setupMocks: func(
				storeMock *entitymock.EntityServiceInterfaceMock,
				ouServiceMock *oumock.OrganizationUnitServiceInterfaceMock,
				entityTypeMock *entitytypemock.EntityTypeServiceInterfaceMock,
			) {
				storeMock.On("GetEntityProfile", mock.Anything, userID).
					Return(&providers.Entity{
						Category: providers.EntityCategoryUser,
						ID:       userID,
						OUID:     testOrgID,
						Type:     testUserType,
					}, nil).Once()
				ouServiceMock.On("IsOrganizationUnitExists", mock.Anything, testOrgID).
					Return(true, (*tidcommon.ServiceError)(nil)).Once()
				entityTypeMock.On("GetEntityTypeByHandle", mock.Anything, mock.Anything, testUserType).
					Return(&entitytype.EntityType{OUID: testOrgID},
						(*tidcommon.ServiceError)(nil)).Once()
				entityTypeMock.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
					entitytype.AttributeFilter{AllowCredential: true}).
					Return([]entitytype.AttributeInfo{{Attribute: "password"}},
						(*tidcommon.ServiceError)(nil)).Once()
			},
			expectedError: &ErrorInvalidRequestFormat,
		},
		{
			name:       "CredentialCheck_NoMatchingCredentialFields",
			attributes: `{"email":"test@example.com","username":"john"}`,
			setupMocks: func(
				storeMock *entitymock.EntityServiceInterfaceMock,
				ouServiceMock *oumock.OrganizationUnitServiceInterfaceMock,
				entityTypeMock *entitytypemock.EntityTypeServiceInterfaceMock,
			) {
				ouServiceMock.On("IsOrganizationUnitExists", mock.Anything, testOrgID).
					Return(true, (*tidcommon.ServiceError)(nil)).Once()
				entityTypeMock.On("GetEntityTypeByHandle", mock.Anything, mock.Anything, testUserType).
					Return(&entitytype.EntityType{OUID: testOrgID},
						(*tidcommon.ServiceError)(nil)).Once()
				entityTypeMock.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
					entitytype.AttributeFilter{AllowCredential: true}).
					Return([]entitytype.AttributeInfo{{Attribute: "password"}},
						(*tidcommon.ServiceError)(nil)).Once()
				storeMock.On("GetEntityProfile", mock.Anything, userID).
					Return(&providers.Entity{
						Category: providers.EntityCategoryUser,
						ID:       userID,
						OUID:     testOrgID,
						Type:     testUserType,
					}, nil).Once()
				storeMock.On("UpdateEntity", mock.Anything, userID, mock.Anything).
					Return(&providers.Entity{
						ID:         userID,
						Attributes: json.RawMessage(`{"email":"updated@example.com"}`),
					}, nil).Once()
			},
			expectedError: nil,
		},
		{
			name:       "GetUser_UserNotFound",
			attributes: `{"email":"test@example.com"}`,
			setupMocks: func(
				storeMock *entitymock.EntityServiceInterfaceMock,
				_ *oumock.OrganizationUnitServiceInterfaceMock,
				_ *entitytypemock.EntityTypeServiceInterfaceMock,
			) {
				storeMock.On("GetEntityProfile", mock.Anything, userID).
					Return((*providers.Entity)(nil), entitypkg.ErrEntityNotFound).Once()
			},
			expectedError: &ErrorUserNotFound,
		},
		{
			name:       "GetUser_GenericError",
			attributes: `{"email":"test@example.com"}`,
			setupMocks: func(
				storeMock *entitymock.EntityServiceInterfaceMock,
				_ *oumock.OrganizationUnitServiceInterfaceMock,
				_ *entitytypemock.EntityTypeServiceInterfaceMock,
			) {
				storeMock.On("GetEntityProfile", mock.Anything, userID).
					Return((*providers.Entity)(nil), errors.New("db connection lost")).Once()
			},
			expectedError: &tidcommon.InternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updatedUser := providers.User{
				ID:         userID,
				OUID:       testOrgID,
				Type:       testUserType,
				Attributes: json.RawMessage(tt.attributes),
			}

			storeMock := entitymock.NewEntityServiceInterfaceMock(t)
			storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
			ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
			entityTypeMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
			if tt.setupMocks != nil {
				tt.setupMocks(storeMock, ouServiceMock, entityTypeMock)
			}

			service := &userService{
				entityService:     storeMock,
				ouService:         ouServiceMock,
				entityTypeService: entityTypeMock,
				authzService:      newAllowAllAuthz(t),
				governance:        activeGovernance(t),
			}

			resp, err := service.UpdateUser(ctx, userID, &updatedUser)
			if tt.expectedError != nil {
				require.NotNil(t, err)
				require.Nil(t, resp)
				require.Equal(t, tt.expectedError.Code, err.Code)
			} else {
				require.Nil(t, err)
				require.NotNil(t, resp)
				require.Equal(t, userID, resp.ID)
			}
		})
	}
}

func TestUserService_UpdateUser_AuthzBranches(t *testing.T) {
	ctx := context.Background()
	userID := svcTestUserID1
	existingOU := "11111111-1111-1111-1111-111111111111"
	destinationOU := "22222222-2222-2222-2222-222222222222"

	tests := []struct {
		name            string
		userOU          string // OrganizationUnit in the update request
		setupAuthzMock  func(authzMock *sysauthzmock.SystemAuthorizationServiceInterfaceMock)
		setupExtraMocks func(
			storeMock *entitymock.EntityServiceInterfaceMock,
			ouMock *oumock.OrganizationUnitServiceInterfaceMock,
			schemaMock *entitytypemock.EntityTypeServiceInterfaceMock)
		expectedErrorCode string
	}{
		{
			name:   "Denied_on_existing_user_OU",
			userOU: existingOU, // same OU, so only one authz check should occur
			setupAuthzMock: func(authzMock *sysauthzmock.SystemAuthorizationServiceInterfaceMock) {
				// First check on existing OU → denied.
				authzMock.On("IsActionAllowed", mock.Anything, security.ActionUpdateUser,
					&sysauthz.ActionContext{
						ResourceType: security.ResourceTypeUser,
						OUID:         existingOU,
						ResourceID:   userID,
					}).Return(false, nil).Once()
			},
			expectedErrorCode: tidcommon.ErrorUnauthorized.Code,
		},
		{
			name:   "Authz_service_error_on_existing_user_OU",
			userOU: existingOU,
			setupAuthzMock: func(authzMock *sysauthzmock.SystemAuthorizationServiceInterfaceMock) {
				// First check on existing OU → service error.
				authzMock.On("IsActionAllowed", mock.Anything, security.ActionUpdateUser,
					&sysauthz.ActionContext{
						ResourceType: security.ResourceTypeUser,
						OUID:         existingOU,
						ResourceID:   userID,
					}).Return(false, &tidcommon.InternalServerError).Once()
			},
			expectedErrorCode: tidcommon.InternalServerError.Code,
		},
		{
			name:   "Same_OU_skips_destination_check",
			userOU: existingOU, // same OU → no second authz check
			setupAuthzMock: func(authzMock *sysauthzmock.SystemAuthorizationServiceInterfaceMock) {
				// Only the first check on existing OU → allowed. No second call expected.
				authzMock.On("IsActionAllowed", mock.Anything, security.ActionUpdateUser,
					&sysauthz.ActionContext{
						ResourceType: security.ResourceTypeUser,
						OUID:         existingOU,
						ResourceID:   userID,
					}).Return(true, nil).Once()
			},
			expectedErrorCode: "", // success path (no authz error)
		},
		{
			name:   "Empty_OU_triggers_destination_check",
			userOU: "", // empty OU differs from existingOU → second authz check is triggered
			setupAuthzMock: func(authzMock *sysauthzmock.SystemAuthorizationServiceInterfaceMock) {
				// First check on existing OU → allowed.
				authzMock.On("IsActionAllowed", mock.Anything, security.ActionUpdateUser,
					&sysauthz.ActionContext{
						ResourceType: security.ResourceTypeUser,
						OUID:         existingOU,
						ResourceID:   userID,
					}).Return(true, nil).Once()
				// Second check on empty destination OU → allowed.
				authzMock.On("IsActionAllowed", mock.Anything, security.ActionUpdateUser,
					&sysauthz.ActionContext{
						ResourceType: security.ResourceTypeUser,
						OUID:         "",
						ResourceID:   userID,
					}).Return(true, nil).Once()
			},
			// Downstream validation rejects empty OU after both authz checks pass.
			expectedErrorCode: ErrorInvalidOUID.Code,
		},
		{
			name:   "Whitespace_OU_triggers_destination_check",
			userOU: "   ", // whitespace OU differs from existingOU → second authz check is triggered
			setupAuthzMock: func(authzMock *sysauthzmock.SystemAuthorizationServiceInterfaceMock) {
				// First check on existing OU → allowed.
				authzMock.On("IsActionAllowed", mock.Anything, security.ActionUpdateUser,
					&sysauthz.ActionContext{
						ResourceType: security.ResourceTypeUser,
						OUID:         existingOU,
						ResourceID:   userID,
					}).Return(true, nil).Once()
				// Second check on whitespace destination OU → allowed.
				authzMock.On("IsActionAllowed", mock.Anything, security.ActionUpdateUser,
					&sysauthz.ActionContext{
						ResourceType: security.ResourceTypeUser,
						OUID:         "   ",
						ResourceID:   userID,
					}).Return(true, nil).Once()
			},
			// Downstream validation rejects whitespace OU after both authz checks pass.
			expectedErrorCode: ErrorInvalidOUID.Code,
		},
		{
			name:   "Different_OU_destination_denied",
			userOU: destinationOU,
			setupAuthzMock: func(authzMock *sysauthzmock.SystemAuthorizationServiceInterfaceMock) {
				// First check on existing OU → allowed.
				authzMock.On("IsActionAllowed", mock.Anything, security.ActionUpdateUser,
					&sysauthz.ActionContext{
						ResourceType: security.ResourceTypeUser,
						OUID:         existingOU,
						ResourceID:   userID,
					}).Return(true, nil).Once()
				// Second check on destination OU → denied.
				authzMock.On("IsActionAllowed", mock.Anything, security.ActionUpdateUser,
					&sysauthz.ActionContext{
						ResourceType: security.ResourceTypeUser,
						OUID:         destinationOU,
						ResourceID:   userID,
					}).Return(false, nil).Once()
			},
			expectedErrorCode: tidcommon.ErrorUnauthorized.Code,
		},
		{
			name:   "Different_OU_destination_authz_error",
			userOU: destinationOU,
			setupAuthzMock: func(authzMock *sysauthzmock.SystemAuthorizationServiceInterfaceMock) {
				// First check on existing OU → allowed.
				authzMock.On("IsActionAllowed", mock.Anything, security.ActionUpdateUser,
					&sysauthz.ActionContext{
						ResourceType: security.ResourceTypeUser,
						OUID:         existingOU,
						ResourceID:   userID,
					}).Return(true, nil).Once()
				// Second check on destination OU → service error.
				authzMock.On("IsActionAllowed", mock.Anything, security.ActionUpdateUser,
					&sysauthz.ActionContext{
						ResourceType: security.ResourceTypeUser,
						OUID:         destinationOU,
						ResourceID:   userID,
					}).Return(false, &tidcommon.InternalServerError).Once()
			},
			expectedErrorCode: tidcommon.InternalServerError.Code,
		},
		{
			name:   "Different_OU_both_allowed",
			userOU: destinationOU,
			setupAuthzMock: func(authzMock *sysauthzmock.SystemAuthorizationServiceInterfaceMock) {
				// First check on existing OU → allowed.
				authzMock.On("IsActionAllowed", mock.Anything, security.ActionUpdateUser,
					&sysauthz.ActionContext{
						ResourceType: security.ResourceTypeUser,
						OUID:         existingOU,
						ResourceID:   userID,
					}).Return(true, nil).Once()
				// Second check on destination OU → allowed.
				authzMock.On("IsActionAllowed", mock.Anything, security.ActionUpdateUser,
					&sysauthz.ActionContext{
						ResourceType: security.ResourceTypeUser,
						OUID:         destinationOU,
						ResourceID:   userID,
					}).Return(true, nil).Once()
			},
			setupExtraMocks: func(
				_ *entitymock.EntityServiceInterfaceMock,
				ouMock *oumock.OrganizationUnitServiceInterfaceMock,
				_ *entitytypemock.EntityTypeServiceInterfaceMock,
			) {
				// Destination OU differs from the schema OU, so IsParent is called.
				ouMock.On("IsParent", mock.Anything, existingOU, destinationOU).
					Return(true, (*tidcommon.ServiceError)(nil)).Maybe()
			},
			expectedErrorCode: "", // success path
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storeMock := entitymock.NewEntityServiceInterfaceMock(t)
			storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
			ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
			entityTypeMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
			authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
			// The existing user always lives in existingOU.
			storeMock.On("GetEntityProfile", mock.Anything, userID).
				Return(&providers.Entity{
					Category: providers.EntityCategoryUser, ID: userID,
					OUID: existingOU, Type: testUserType,
				}, nil).Once()

			tt.setupAuthzMock(authzMock)

			// For success-path cases, set up the remaining mocks so the method completes.
			if tt.expectedErrorCode == "" {
				ouServiceMock.On("IsOrganizationUnitExists", mock.Anything, mock.Anything).
					Return(true, (*tidcommon.ServiceError)(nil)).Maybe()
				entityTypeMock.On("GetEntityTypeByHandle", mock.Anything, mock.Anything, testUserType).
					Return(&entitytype.EntityType{OUID: existingOU},
						(*tidcommon.ServiceError)(nil)).Maybe()
				entityTypeMock.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
					entitytype.AttributeFilter{AllowCredential: true}).
					Return([]entitytype.AttributeInfo{}, (*tidcommon.ServiceError)(nil)).Maybe()
				storeMock.On("UpdateEntity", mock.Anything, userID, mock.Anything).
					Return(&providers.Entity{
						ID:         userID,
						Attributes: json.RawMessage(`{"email":"test@example.com"}`),
					}, nil).Maybe()
			}

			if tt.setupExtraMocks != nil {
				tt.setupExtraMocks(storeMock, ouServiceMock, entityTypeMock)
			}

			service := &userService{
				entityService:     storeMock,
				ouService:         ouServiceMock,
				entityTypeService: entityTypeMock,
				authzService:      authzMock,
				governance:        activeGovernance(t),
				sharingService:    unsharedSharing(t),
			}

			updatedUser := providers.User{
				ID:         userID,
				OUID:       tt.userOU,
				Type:       testUserType,
				Attributes: json.RawMessage(`{"email":"test@example.com"}`),
			}

			resp, svcErr := service.UpdateUser(ctx, userID, &updatedUser)
			if tt.expectedErrorCode != "" {
				require.NotNil(t, svcErr)
				require.Nil(t, resp)
				require.Equal(t, tt.expectedErrorCode, svcErr.Code)
			} else {
				require.Nil(t, svcErr)
				require.NotNil(t, resp)
				require.Equal(t, userID, resp.ID)
			}

			storeMock.AssertExpectations(t)
			authzMock.AssertExpectations(t)
		})
	}
}

func TestUserService_UpdateUser_RejectsCredentialInMixedAttributes(t *testing.T) {
	ctx := context.Background()
	userID := svcTestUserID123
	testOU := testOrgID

	updatedUser := providers.User{
		ID:   userID,
		Type: testUserType,
		OUID: testOU,
		Attributes: json.RawMessage(`{
			"username": "john.doe",
			"email": "john.updated@example.com",
			"password": "NewPassword456!"
		}`),
	}

	storeMock := entitymock.NewEntityServiceInterfaceMock(t)
	storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	entityTypeMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)

	storeMock.On("GetEntityProfile", mock.Anything, userID).
		Return(&providers.Entity{
			Category: providers.EntityCategoryUser, ID: userID, OUID: testOU, Type: testUserType,
		}, nil).Once()

	ouServiceMock.On("IsOrganizationUnitExists", mock.Anything, testOU).
		Return(true, (*tidcommon.ServiceError)(nil)).Once()
	entityTypeMock.On("GetEntityTypeByHandle", mock.Anything, mock.Anything, testUserType).
		Return(&entitytype.EntityType{
			Handle: testUserType,
			OUID:   testOU,
		}, (*tidcommon.ServiceError)(nil)).Once()
	entityTypeMock.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		entitytype.AttributeFilter{AllowCredential: true}).
		Return([]entitytype.AttributeInfo{{Attribute: "password"}}, (*tidcommon.ServiceError)(nil)).Once()

	// Create service
	service := &userService{
		entityService:     storeMock,
		ouService:         ouServiceMock,
		entityTypeService: entityTypeMock,
		authzService:      newAllowAllAuthz(t),
	}

	result, svcErr := service.UpdateUser(ctx, userID, &updatedUser)

	require.Nil(t, result)
	require.NotNil(t, svcErr)
	require.Equal(t, ErrorCredentialUpdateNotAllowed.Code, svcErr.Code)
}

func TestUserService_GetUserList(t *testing.T) {
	limit := 10
	offset := 0
	filters := map[string]interface{}{}

	storeMock := entitymock.NewEntityServiceInterfaceMock(t)
	storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
	storeMock.On("GetEntityListCount", mock.Anything, providers.EntityCategoryUser, filters).Return(5, nil).Once()
	storeMock.On("GetEntityList", mock.Anything, providers.EntityCategoryUser, limit, offset, filters).
		Return([]providers.Entity{{ID: svcTestUserID1}}, nil).
		Once()

	service := &userService{
		entityService: storeMock,
		authzService:  newAllowAllAuthz(t),
		governance:    activeGovernance(t),
	}

	resp, err := service.GetUserList(context.Background(), limit, offset, filters, false)
	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Equal(t, 5, resp.TotalResults)
	require.Len(t, resp.Users, 1)
	require.True(t, *resp.Users[0].IsOwnedByOU, "an unfiltered list holds every user in the unit that owns it")
}

func TestUserService_GetUserList_ScopedByOUIDs(t *testing.T) {
	limit := 10
	offset := 0
	filters := map[string]interface{}{}
	ouIDs := []string{testOrgID}

	storeMock := entitymock.NewEntityServiceInterfaceMock(t)
	storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
	storeMock.On("GetEntityListCountByOUIDs", mock.Anything, providers.EntityCategoryUser, ouIDs, filters).
		Return(3, nil).Once()
	storeMock.On("GetEntityListByOUIDs", mock.Anything, providers.EntityCategoryUser, ouIDs, limit, offset, filters).
		Return([]providers.Entity{{ID: svcTestUserID1, OUID: testOrgID}}, nil).
		Once()

	authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
	authzMock.On("GetAccessibleResources", mock.Anything, mock.Anything, mock.Anything).
		Return(&sysauthz.AccessibleResources{AllAllowed: false, IDs: ouIDs}, nil).Once()

	service := &userService{
		entityService: storeMock,
		authzService:  authzMock,
		governance:    activeGovernance(t),
	}

	resp, err := service.GetUserList(context.Background(), limit, offset, filters, false)
	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Equal(t, 3, resp.TotalResults)
	require.Len(t, resp.Users, 1)
	require.True(t, *resp.Users[0].IsOwnedByOU)
}

func TestUserService_GetUserList_EmptyOUIDs(t *testing.T) {
	limit := 10
	offset := 0
	filters := map[string]interface{}{}

	authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
	authzMock.On("GetAccessibleResources", mock.Anything, mock.Anything, mock.Anything).
		Return(&sysauthz.AccessibleResources{AllAllowed: false, IDs: []string{}}, nil).Once()

	service := &userService{
		entityService: entitymock.NewEntityServiceInterfaceMock(t),
		authzService:  authzMock,
	}

	resp, err := service.GetUserList(context.Background(), limit, offset, filters, false)
	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Equal(t, 0, resp.TotalResults)
	require.Empty(t, resp.Users)
}

// ouFilteredService returns a service whose entity service answers the page request for testOrgID with
// page.
func ouFilteredService(
	t *testing.T, authz sysauthz.SystemAuthorizationServiceInterface, page *entitypkg.OUEntityPage,
) *userService {
	storeMock := entitymock.NewEntityServiceInterfaceMock(t)
	storeMock.On("GetEntityPageInOU", mock.Anything, providers.EntityCategoryUser, testOrgID,
		entitypkg.OUEntityScopeAll, 2, 0, map[string]interface{}{}).Return(page, nil).Once()
	return &userService{
		entityService: storeMock,
		authzService:  authz,
		governance:    activeGovernance(t),
	}
}

func TestUserService_GetUserList_FilteredByOU_ListsOwnedThenShared(t *testing.T) {
	service := ouFilteredService(t, newAllowAllAuthz(t), &entitypkg.OUEntityPage{
		Owned:        []providers.Entity{{ID: "owned-1", OUID: testOrgID}},
		Shared:       []providers.Entity{{ID: "shared-1", OUID: "other-ou"}},
		TotalResults: 3,
	})

	resp, err := service.GetUserList(context.Background(), 2, 0,
		map[string]interface{}{"ouId": testOrgID}, false)

	require.Nil(t, err)
	require.Equal(t, 3, resp.TotalResults)
	require.Len(t, resp.Users, 2)
	require.Equal(t, "owned-1", resp.Users[0].ID)
	require.True(t, *resp.Users[0].IsOwnedByOU)
	require.Equal(t, "shared-1", resp.Users[1].ID)
	require.False(t, *resp.Users[1].IsOwnedByOU)
	require.Len(t, resp.Links, 2)
	for _, link := range resp.Links {
		require.Contains(t, link.Href, "filter=ouId+eq+%22"+testOrgID+"%22",
			"link %q has to list the same organization unit", link.Rel)
	}
}

func TestUserService_GetUserList_FilteredByOU_ScopedCallerWithAccess(t *testing.T) {
	authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
	authzMock.On("GetAccessibleResources", mock.Anything, mock.Anything, mock.Anything).
		Return(&sysauthz.AccessibleResources{IDs: []string{"another-ou", testOrgID}}, nil).Once()
	service := ouFilteredService(t, authzMock, &entitypkg.OUEntityPage{
		Owned:        []providers.Entity{{ID: "owned-1", OUID: testOrgID}},
		Shared:       []providers.Entity{},
		TotalResults: 1,
	})

	resp, err := service.GetUserList(context.Background(), 2, 0,
		map[string]interface{}{"ouId": testOrgID}, false)

	require.Nil(t, err)
	require.Equal(t, 1, resp.TotalResults)
	require.Len(t, resp.Users, 1)
}

func TestUserService_GetUserList_FilteredByOU_CallerWithoutAccessSeesNone(t *testing.T) {
	authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
	authzMock.On("GetAccessibleResources", mock.Anything, mock.Anything, mock.Anything).
		Return(&sysauthz.AccessibleResources{IDs: []string{"another-ou"}}, nil).Once()
	service := &userService{
		entityService: entitymock.NewEntityServiceInterfaceMock(t),
		authzService:  authzMock,
	}

	resp, err := service.GetUserList(context.Background(), 10, 0,
		map[string]interface{}{"ouId": testOrgID}, false)

	require.Nil(t, err)
	require.Zero(t, resp.TotalResults)
	require.Empty(t, resp.Users)
}

func TestUserService_GetUserList_FilteredByOU_Errors(t *testing.T) {
	tests := []struct {
		name    string
		filters map[string]interface{}
		setup   func(t *testing.T) *userService
		wantErr *tidcommon.ServiceError
	}{
		{
			name:    "an organization unit that is not a string",
			filters: map[string]interface{}{"ouId": int64(5)},
			setup: func(t *testing.T) *userService {
				return &userService{authzService: sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)}
			},
			wantErr: &ErrorInvalidFilter,
		},
		{
			name:    "an organization unit with another condition",
			filters: map[string]interface{}{"ouId": testOrgID, "username": "alice"},
			setup: func(t *testing.T) *userService {
				return &userService{authzService: sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)}
			},
			wantErr: &ErrorInvalidFilter,
		},
		{
			name:    "the page read fails",
			filters: map[string]interface{}{"ouId": testOrgID},
			setup: func(t *testing.T) *userService {
				storeMock := entitymock.NewEntityServiceInterfaceMock(t)
				storeMock.On("GetEntityPageInOU", mock.Anything, providers.EntityCategoryUser, testOrgID,
					entitypkg.OUEntityScopeAll, 10, 0, map[string]interface{}{}).
					Return(nil, errors.New("db error")).Once()
				return &userService{entityService: storeMock, authzService: newAllowAllAuthz(t)}
			},
			wantErr: &tidcommon.InternalServerError,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := tc.setup(t).GetUserList(context.Background(), 10, 0, tc.filters, false)

			require.Nil(t, resp)
			require.NotNil(t, err)
			require.Equal(t, tc.wantErr.Code, err.Code)
		})
	}
}

func TestUserService_GetUserGroups(t *testing.T) {
	mockStore := entitymock.NewEntityServiceInterfaceMock(t)
	mockStore.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
	userID := svcTestUserID123
	limit, offset := 10, 0

	mockStore.On("GetEntityProfile", mock.Anything, userID).
		Return(&providers.Entity{
			Category: providers.EntityCategoryUser, ID: userID, OUID: testOrgID,
		}, nil).Once()
	mockStore.On("GetGroupCountForEntity", mock.Anything, userID).Return(5, nil)
	mockStore.On("GetEntityGroups", mock.Anything, userID, limit, offset).
		Return([]providers.EntityGroup{{ID: "g1", Name: "Group 1"}}, nil)

	service := &userService{
		entityService: mockStore,
		authzService:  newAllowAllAuthz(t),
	}
	resp, err := service.GetUserGroups(context.Background(), userID, limit, offset)

	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Equal(t, 5, resp.TotalResults)
	require.Len(t, resp.Groups, 1)
}

func TestUserService_GetUserGroups_ErrorCases(t *testing.T) {
	mockStore := entitymock.NewEntityServiceInterfaceMock(t)
	mockStore.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
	service := &userService{
		entityService: mockStore,
		authzService:  newAllowAllAuthz(t),
	}
	ctx := context.Background()

	t.Run("MissingUserID", func(t *testing.T) {
		_, err := service.GetUserGroups(ctx, "", 10, 0)
		require.NotNil(t, err)
		require.Equal(t, ErrorMissingUserID.Code, err.Code)
	})

	t.Run("InvalidPagination", func(t *testing.T) {
		_, err := service.GetUserGroups(ctx, "u1", -1, 0)
		require.NotNil(t, err)
	})

	t.Run("UserNotFound", func(t *testing.T) {
		mockStore.On("GetEntityProfile", mock.Anything, "u1").
			Return((*providers.Entity)(nil), entitypkg.ErrEntityNotFound).Once()
		_, err := service.GetUserGroups(ctx, "u1", 10, 0)
		require.NotNil(t, err)
		require.Equal(t, ErrorUserNotFound.Code, err.Code)
	})

	t.Run("StoreErrorOnGetUser", func(t *testing.T) {
		mockStore.On("GetEntityProfile", mock.Anything, "u1").
			Return((*providers.Entity)(nil), errors.New("db error")).Once()
		_, err := service.GetUserGroups(ctx, "u1", 10, 0)
		require.NotNil(t, err)
		require.Equal(t, tidcommon.InternalServerError.Code, err.Code)
	})

	t.Run("StoreErrorOnCount", func(t *testing.T) {
		mockStore.On("GetEntityProfile", mock.Anything, "u1").
			Return(&providers.Entity{
				Category: providers.EntityCategoryUser, ID: "u1", OUID: testOrgID,
			}, nil).Once()
		mockStore.On("GetGroupCountForEntity", mock.Anything, "u1").
			Return(0, errors.New("db error")).Once()
		_, err := service.GetUserGroups(ctx, "u1", 10, 0)
		require.NotNil(t, err)
		require.Equal(t, tidcommon.InternalServerError.Code, err.Code)
	})
}

func TestBuildPaginationLinks(t *testing.T) {
	links := utils.BuildPaginationLinks("/users", 10, 20, 55, "")
	// totalResults 55, limit 10
	// 0-9, 10-19, 20-29, 30-39, 40-49, 50-54
	// offset 20 (3rd page)
	// first: 0
	// prev: 10
	// next: 30
	// last: 50
	require.Len(t, links, 4)

	relMap := make(map[string]string)
	for _, l := range links {
		relMap[l.Rel] = l.Href
	}

	require.Equal(t, "/users?offset=0&limit=10", relMap["first"])
	require.Equal(t, "/users?offset=30&limit=10", relMap["next"])
	require.Equal(t, "/users?offset=50&limit=10", relMap["last"])
}

func TestUserService_CRUD_ErrorCases(t *testing.T) {
	mockStore := entitymock.NewEntityServiceInterfaceMock(t)
	mockStore.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
	service := &userService{
		entityService: mockStore,
		authzService:  newAllowAllAuthz(t),
	}
	ctx := context.Background()

	t.Run("GetUser_MissingID", func(t *testing.T) {
		_, err := service.GetUser(ctx, "", false)
		require.NotNil(t, err)
		require.Equal(t, ErrorMissingUserID.Code, err.Code)
	})

	t.Run("GetUser_NotFound", func(t *testing.T) {
		mockStore.On("GetEntity", mock.Anything, "u1").
			Return((*providers.Entity)(nil), entitypkg.ErrEntityNotFound).Once()
		_, err := service.GetUser(ctx, "u1", false)
		require.NotNil(t, err)
		require.Equal(t, ErrorUserNotFound.Code, err.Code)
	})

	t.Run("DeleteUser_MissingID", func(t *testing.T) {
		err := service.DeleteUser(ctx, "")
		require.NotNil(t, err)
		require.Equal(t, ErrorMissingUserID.Code, err.Code)
	})

	t.Run("DeleteUser_NotFound", func(t *testing.T) {
		mockStore.On("GetEntityProfile", mock.Anything, "u1").
			Return((*providers.Entity)(nil), entitypkg.ErrEntityNotFound).Once()
		err := service.DeleteUser(ctx, "u1")
		require.NotNil(t, err)
		require.Equal(t, ErrorUserNotFound.Code, err.Code)
	})

	t.Run("CreateUser_MissingType", func(t *testing.T) {
		_, err := service.CreateUser(ctx, &providers.User{ID: "u1"})
		require.NotNil(t, err)
		require.Equal(t, ErrorEntityTypeNotFound.Code, err.Code)
	})

	t.Run("UpdateUser_MissingID", func(t *testing.T) {
		_, err := service.UpdateUser(ctx, "", &providers.User{})
		require.NotNil(t, err)
		require.Equal(t, ErrorMissingUserID.Code, err.Code)
	})
}

func TestUserService_GetUsersByPath(t *testing.T) {
	mockOU := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	mockOU.On("GetOrganizationUnitByPath", mock.Anything, "root").
		Return(oupkg.OrganizationUnit{ID: "ou-1"}, nil).
		Once()
	storeMock := entitymock.NewEntityServiceInterfaceMock(t)
	storeMock.On("GetEntityPageInOU", mock.Anything, providers.EntityCategoryUser, "ou-1",
		entitypkg.OUEntityScopeAll, 10, 0, (map[string]interface{})(nil)).
		Return(&entitypkg.OUEntityPage{
			Owned:        []providers.Entity{{ID: "u1", OUID: "ou-1"}},
			Shared:       []providers.Entity{{ID: "u2", OUID: "ou-2"}},
			TotalResults: 20,
		}, nil).Once()
	service := &userService{
		ouService:     mockOU,
		authzService:  newAllowAllAuthz(t),
		entityService: storeMock,
		governance:    activeGovernance(t),
	}

	resp, err := service.GetUsersByPath(context.Background(), "root", 10, 0, nil, false)

	require.Nil(t, err)
	require.Equal(t, 20, resp.TotalResults)
	require.Equal(t, 2, resp.Count)
	require.Equal(t, "u1", resp.Users[0].ID)
	require.True(t, *resp.Users[0].IsOwnedByOU)
	require.Equal(t, "u2", resp.Users[1].ID)
	require.Equal(t, "ou-1", resp.Users[1].OUID, "a shared user is listed under the unit it is shared into")
	require.False(t, *resp.Users[1].IsOwnedByOU)
	require.NotEmpty(t, resp.Links)
	require.Contains(t, resp.Links[0].Href, "/users/tree/root")
}

func TestUserService_GetUsersByPath_WithIncludeDisplay(t *testing.T) {
	mockOU := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	mockOU.On("GetOrganizationUnitByPath", mock.Anything, "root").
		Return(oupkg.OrganizationUnit{ID: "ou-1"}, nil).Once()
	mockOU.On("GetOrganizationUnitHandlesByIDs", mock.Anything, []string{"ou-1"}).
		Return(map[string]string{"ou-1": "root"}, nil).Once()
	storeMock := entitymock.NewEntityServiceInterfaceMock(t)
	storeMock.On("GetEntityPageInOU", mock.Anything, providers.EntityCategoryUser, "ou-1",
		entitypkg.OUEntityScopeAll, 10, 0, (map[string]interface{})(nil)).
		Return(&entitypkg.OUEntityPage{
			Owned: []providers.Entity{{ID: "u1", OUID: "ou-1", Type: "employee",
				Attributes: json.RawMessage(`{"email":"alice@example.com"}`)}},
			Shared: []providers.Entity{{ID: "u2", OUID: "ou-2", Type: "employee",
				Attributes: json.RawMessage(`{"email":"bob@example.com"}`)}},
			TotalResults: 2,
		}, nil).Once()
	mockSchema := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockSchema.On("GetDisplayAttributesByHandles", mock.Anything, mock.Anything, []string{"employee"}).
		Return(map[string]string{"employee": "email"}, nil).Once()
	service := &userService{
		ouService:         mockOU,
		authzService:      newAllowAllAuthz(t),
		entityService:     storeMock,
		entityTypeService: mockSchema,
		governance:        activeGovernance(t),
	}

	resp, err := service.GetUsersByPath(context.Background(), "root", 10, 0, nil, true)

	require.Nil(t, err)
	require.Equal(t, 2, resp.TotalResults)
	require.Equal(t, "alice@example.com", resp.Users[0].Display)
	require.Equal(t, "root", resp.Users[0].OUHandle)
	require.Equal(t, "bob@example.com", resp.Users[1].Display)
	require.Equal(t, "ou-1", resp.Users[1].OUID, "a shared user shows the unit it is shared into")
	require.Equal(t, "root", resp.Users[1].OUHandle)
}

func TestNewFunctions(t *testing.T) {
	svc := newUserService(nil, nil, nil, nil, nil, nil, nil)
	require.NotNil(t, svc)

	handler := newUserHandler(svc)
	require.NotNil(t, handler)
}

func TestUserService_Validation_EdgeCases(t *testing.T) {
	service := &userService{}
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "UserServiceTest"))

	t.Run("ValidateOU_InvalidUUID", func(t *testing.T) {
		err := service.validateOrganizationUnitForUserType(context.Background(), "customer", "invalid-uuid", logger)
		require.NotNil(t, err)
		require.Equal(t, tidcommon.InternalServerError.Code, err.Code)
	})

	t.Run("ValidateOU_EmptyOU", func(t *testing.T) {
		err := service.validateOrganizationUnitForUserType(context.Background(), "customer", "", logger)
		require.NotNil(t, err)
		require.Equal(t, ErrorInvalidOUID.Code, err.Code)
	})
}

func TestUserService_MoreErrorCases(t *testing.T) {
	storeMock := &entitymock.EntityServiceInterfaceMock{}
	storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	entityTypeMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	authzMock := newAllowAllAuthz(t)
	service := &userService{
		entityService:      storeMock,
		ouService:          ouServiceMock,
		entityTypeService:  entityTypeMock,
		authzService:       authzMock,
		dependencyRegistry: newNoBlockingDepsRegistry(),
		sharingService:     newPurgingSharing(t, "u1"),
	}
	ctx := context.Background()

	t.Run("UpdateUser_StoreError", func(t *testing.T) {
		userIn := &providers.User{Type: "customer", OUID: testOrgID}
		storeMock.On("GetEntityProfile", mock.Anything, "u1").
			Return(&providers.Entity{
				Category: providers.EntityCategoryUser, ID: "u1", OUID: testOrgID,
			}, nil).Once()
		storeMock.On("UpdateEntity", mock.Anything, "u1", mock.Anything).
			Return((*providers.Entity)(nil), errors.New("db error")).Once()

		// Mock all validation steps with broad matches to ensure they hit
		entityTypeMock.On("GetAttributes", mock.Anything, mock.Anything, mock.Anything,
			entitytype.AttributeFilter{AllowCredential: true}).
			Return([]entitytype.AttributeInfo{}, (*tidcommon.ServiceError)(nil)).Maybe()
		ouServiceMock.On("IsOrganizationUnitExists", mock.Anything, mock.Anything).Return(true, nil).Maybe()
		ouServiceMock.On("IsParent", mock.Anything, mock.Anything, mock.Anything).Return(true, nil).Maybe()
		entityTypeMock.On("GetEntityTypeByHandle", mock.Anything, mock.Anything, mock.Anything).
			Return(&entitytype.EntityType{}, nil).Maybe()
		entityTypeMock.On(
			"ValidateEntity", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		).Return(true, nil).Maybe()
		entityTypeMock.On(
			"ValidateEntityUniqueness", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		).
			Return(true, nil).Maybe()
		storeMock.On("IdentifyEntity", mock.Anything, mock.Anything).
			Return((*string)(nil), entitypkg.ErrEntityNotFound).Maybe()

		_, err := service.UpdateUser(ctx, "u1", userIn)
		require.NotNil(t, err)
		require.Equal(t, tidcommon.InternalServerError.Code, err.Code)
	})

	t.Run("DeleteUser_StoreError", func(t *testing.T) {
		storeMock.On("GetEntityProfile", mock.Anything, "u1").
			Return(&providers.Entity{
				Category: providers.EntityCategoryUser, ID: "u1", OUID: testOrgID,
			}, nil).Once()
		storeMock.On("DeleteEntity", mock.Anything, "u1").Return(errors.New("db error")).Once()
		err := service.DeleteUser(ctx, "u1")
		require.NotNil(t, err)
		require.Equal(t, tidcommon.InternalServerError.Code, err.Code)
	})

	t.Run("CreateUserByPath_MissingPath", func(t *testing.T) {
		_, err := service.CreateUserByPath(ctx, "", CreateUserByPathRequest{})
		require.NotNil(t, err)
		require.Equal(t, ErrorInvalidHandlePath.Code, err.Code)
	})
}

func TestCredentialType_Methods(t *testing.T) {
	t.Run("IsSystemManaged", func(t *testing.T) {
		require.True(t, CredentialTypePasskey.IsSystemManaged())
		require.False(t, CredentialType("password").IsSystemManaged())
		require.False(t, CredentialType("pin").IsSystemManaged())
		require.False(t, CredentialType("invalid").IsSystemManaged())
	})

	t.Run("String", func(t *testing.T) {
		require.Equal(t, "password", CredentialType("password").String())
		require.Equal(t, "passkey", CredentialTypePasskey.String())
	})
}

func TestUserService_CreateUser_EntityErrors(t *testing.T) {
	tests := []struct {
		name        string
		entityErr   error
		expectedErr tidcommon.ServiceError
	}{
		{"SchemaNotFound", entitypkg.ErrSchemaValidationFailed, ErrorSchemaValidationFailed},
		{"AttributeConflict", entitypkg.ErrAttributeConflict, ErrorAttributeConflict},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
			ouServiceMock.On("IsOrganizationUnitExists", mock.Anything, testOrgID).
				Return(true, (*tidcommon.ServiceError)(nil)).Once()

			entityTypeMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
			entityTypeMock.On("GetEntityTypeByHandle", mock.Anything, mock.Anything, testUserType).
				Return(&entitytype.EntityType{OUID: testOrgID}, (*tidcommon.ServiceError)(nil)).Once()

			storeMock := entitymock.NewEntityServiceInterfaceMock(t)
			storeMock.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return((*providers.Entity)(nil), tc.entityErr).Once()

			service := &userService{
				entityService:     storeMock,
				ouService:         ouServiceMock,
				entityTypeService: entityTypeMock,
				authzService:      newAllowAllAuthz(t),
				uuidGenerator:     utils.GenerateUUIDv7,
			}

			user := &providers.User{
				Type:       testUserType,
				OUID:       testOrgID,
				Attributes: json.RawMessage(`{}`),
			}

			created, svcErr := service.CreateUser(context.Background(), user)
			require.Nil(t, created)
			require.NotNil(t, svcErr)
			require.Equal(t, tc.expectedErr, *svcErr)
		})
	}
}

func TestUserService_UpdateUser_NilSchemaService(t *testing.T) {
	storeMock := entitymock.NewEntityServiceInterfaceMock(t)
	storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
	storeMock.On("GetEntityProfile", mock.Anything, svcTestUserID1).
		Return(&providers.Entity{
			Category: providers.EntityCategoryUser, ID: svcTestUserID1,
			OUID: testOrgID, Type: testUserType,
		}, nil).Once()

	service := &userService{
		entityService: storeMock,
		authzService:  newAllowAllAuthz(t),
	}

	user := &providers.User{
		ID:         svcTestUserID1,
		Type:       testUserType,
		OUID:       testOrgID,
		Attributes: json.RawMessage(`{"email":"test@example.com"}`),
	}

	resp, svcErr := service.UpdateUser(context.Background(), svcTestUserID1, user)
	require.Nil(t, resp)
	require.NotNil(t, svcErr)
	require.Equal(t, tidcommon.InternalServerError, *svcErr)
}

func TestUserService_UpdateUser_SchemaNotFound(t *testing.T) {
	storeMock := entitymock.NewEntityServiceInterfaceMock(t)
	storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
	storeMock.On("GetEntityProfile", mock.Anything, svcTestUserID1).
		Return(&providers.Entity{
			Category: providers.EntityCategoryUser, ID: svcTestUserID1,
			OUID: testOrgID, Type: testUserType,
		}, nil).Once()

	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	ouServiceMock.On("IsOrganizationUnitExists", mock.Anything, testOrgID).
		Return(true, (*tidcommon.ServiceError)(nil)).Once()

	entityTypeMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	entityTypeMock.On("GetEntityTypeByHandle", mock.Anything, mock.Anything, testUserType).
		Return(nil, &entitytype.ErrorEntityTypeNotFound).Once()

	service := &userService{
		entityService:     storeMock,
		ouService:         ouServiceMock,
		entityTypeService: entityTypeMock,
		authzService:      newAllowAllAuthz(t),
	}

	user := &providers.User{
		ID:         svcTestUserID1,
		Type:       testUserType,
		OUID:       testOrgID,
		Attributes: json.RawMessage(`{"email":"test@example.com"}`),
	}

	resp, svcErr := service.UpdateUser(context.Background(), svcTestUserID1, user)
	require.Nil(t, resp)
	require.NotNil(t, svcErr)
	// The error can come from either validateOrganizationUnitForUserType or the credential check.
	// Both map entitytype.ErrorEntityTypeNotFound to ErrorEntityTypeNotFound.
	require.Equal(t, ErrorEntityTypeNotFound, *svcErr)
}

// ---------------------------------------------------------------------------
// checkUserAccess
// ---------------------------------------------------------------------------

func TestUserService_CheckUserAccess(t *testing.T) {
	someAuthzErr := &tidcommon.ServiceError{
		Code:  "SVC-5000",
		Error: tidcommon.I18nMessage{DefaultValue: "authz error"},
	}

	tests := []struct {
		name        string
		isAllowed   bool
		authzSvcErr *tidcommon.ServiceError
		wantErrCode string
	}{
		{
			name:        "Allowed_ReturnsNil",
			isAllowed:   true,
			authzSvcErr: nil,
			wantErrCode: "",
		},
		{
			name:        "Denied_ReturnsUnauthorized",
			isAllowed:   false,
			authzSvcErr: nil,
			wantErrCode: tidcommon.ErrorUnauthorized.Code,
		},
		{
			name:        "AuthzServiceError_ReturnsInternalServerError",
			isAllowed:   false,
			authzSvcErr: someAuthzErr,
			wantErrCode: tidcommon.InternalServerError.Code,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
			authzMock.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
				Return(tc.isAllowed, tc.authzSvcErr).Once()

			svc := &userService{authzService: authzMock}
			err := svc.checkUserAccess(context.Background(), security.ActionReadUser, testOrgID, svcTestUserID1)

			if tc.wantErrCode == "" {
				require.Nil(t, err)
			} else {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.Code)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GetUserList – error paths
// ---------------------------------------------------------------------------

func TestUserService_GetUserList_ErrorCases(t *testing.T) {
	limit, offset := 10, 0
	filters := map[string]interface{}{}
	ouIDs := []string{testOrgID}
	storeErr := errors.New("db error")
	authzErr := &tidcommon.ServiceError{
		Code:  "SVC-5000",
		Error: tidcommon.I18nMessage{DefaultValue: "authz error"},
	}

	tests := []struct {
		name        string
		setup       func(t *testing.T) *userService
		wantErrCode string
	}{
		{
			name: "GetAccessibleResources_Error_ReturnsInternalServerError",
			setup: func(t *testing.T) *userService {
				authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
				authzMock.On("GetAccessibleResources", mock.Anything, mock.Anything, mock.Anything).
					Return((*sysauthz.AccessibleResources)(nil), authzErr).Once()
				return &userService{
					entityService: entitymock.NewEntityServiceInterfaceMock(t),
					authzService:  authzMock,
				}
			},
			wantErrCode: tidcommon.InternalServerError.Code,
		},
		{
			name: "AllAllowed_GetUserListCount_Error_ReturnsInternalServerError",
			setup: func(t *testing.T) *userService {
				storeMock := entitymock.NewEntityServiceInterfaceMock(t)
				storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
				storeMock.On("GetEntityListCount", mock.Anything, providers.EntityCategoryUser, filters).
					Return(0, storeErr).Once()
				return &userService{
					entityService: storeMock,
					authzService:  newAllowAllAuthz(t),
				}
			},
			wantErrCode: tidcommon.InternalServerError.Code,
		},
		{
			name: "AllAllowed_GetUserList_Error_ReturnsInternalServerError",
			setup: func(t *testing.T) *userService {
				storeMock := entitymock.NewEntityServiceInterfaceMock(t)
				storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
				storeMock.On("GetEntityListCount", mock.Anything, providers.EntityCategoryUser, filters).
					Return(5, nil).Once()
				storeMock.On("GetEntityList", mock.Anything, providers.EntityCategoryUser, limit, offset, filters).
					Return([]providers.Entity(nil), storeErr).Once()
				return &userService{
					entityService: storeMock,
					authzService:  newAllowAllAuthz(t),
				}
			},
			wantErrCode: tidcommon.InternalServerError.Code,
		},
		{
			name: "ScopedOUIDs_GetUserListCountByOUIDs_Error_ReturnsInternalServerError",
			setup: func(t *testing.T) *userService {
				storeMock := entitymock.NewEntityServiceInterfaceMock(t)
				storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
				storeMock.On("GetEntityListCountByOUIDs", mock.Anything, providers.EntityCategoryUser, ouIDs, filters).
					Return(0, storeErr).Once()
				authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
				authzMock.On("GetAccessibleResources", mock.Anything, mock.Anything, mock.Anything).
					Return(&sysauthz.AccessibleResources{AllAllowed: false, IDs: ouIDs}, nil).Once()
				return &userService{
					entityService: storeMock,
					authzService:  authzMock,
				}
			},
			wantErrCode: tidcommon.InternalServerError.Code,
		},
		{
			name: "ScopedOUIDs_GetUserListByOUIDs_Error_ReturnsInternalServerError",
			setup: func(t *testing.T) *userService {
				storeMock := entitymock.NewEntityServiceInterfaceMock(t)
				storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
				storeMock.On("GetEntityListCountByOUIDs", mock.Anything, providers.EntityCategoryUser, ouIDs, filters).
					Return(3, nil).Once()
				storeMock.On("GetEntityListByOUIDs",
					mock.Anything, providers.EntityCategoryUser, ouIDs, limit, offset, filters).
					Return([]providers.Entity(nil), storeErr).Once()
				authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
				authzMock.On("GetAccessibleResources", mock.Anything, mock.Anything, mock.Anything).
					Return(&sysauthz.AccessibleResources{AllAllowed: false, IDs: ouIDs}, nil).Once()
				return &userService{
					entityService: storeMock,
					authzService:  authzMock,
				}
			},
			wantErrCode: tidcommon.InternalServerError.Code,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := tc.setup(t)
			resp, err := svc.GetUserList(context.Background(), limit, offset, filters, false)
			require.Nil(t, resp)
			require.NotNil(t, err)
			require.Equal(t, tc.wantErrCode, err.Code)
		})
	}
}

// ---------------------------------------------------------------------------
// GetUsersByPath – authz checks
// ---------------------------------------------------------------------------

func TestUserService_GetUsersByPath_AuthzChecks(t *testing.T) {
	ouID := "ou-1"
	authzErr := &tidcommon.ServiceError{
		Code:  "SVC-5000",
		Error: tidcommon.I18nMessage{DefaultValue: "authz error"},
	}

	tests := []struct {
		name        string
		setup       func(t *testing.T) *userService
		wantErrCode string
	}{
		{
			name: "AuthzDenied_ReturnsUnauthorized",
			setup: func(t *testing.T) *userService {
				ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
				ouServiceMock.On("GetOrganizationUnitByPath", mock.Anything, "root").
					Return(oupkg.OrganizationUnit{ID: ouID}, (*tidcommon.ServiceError)(nil)).Once()

				authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
				authzMock.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
					Return(false, (*tidcommon.ServiceError)(nil)).Once()

				return &userService{
					ouService:    ouServiceMock,
					authzService: authzMock,
				}
			},
			wantErrCode: tidcommon.ErrorUnauthorized.Code,
		},
		{
			name: "AuthzServiceError_ReturnsInternalServerError",
			setup: func(t *testing.T) *userService {
				ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
				ouServiceMock.On("GetOrganizationUnitByPath", mock.Anything, "root").
					Return(oupkg.OrganizationUnit{ID: ouID}, (*tidcommon.ServiceError)(nil)).Once()

				authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
				authzMock.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
					Return(false, authzErr).Once()

				return &userService{
					ouService:    ouServiceMock,
					authzService: authzMock,
				}
			},
			wantErrCode: tidcommon.InternalServerError.Code,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := tc.setup(t)
			resp, err := svc.GetUsersByPath(context.Background(), "root", 10, 0, nil, false)
			require.Nil(t, resp)
			require.NotNil(t, err)
			require.Equal(t, tc.wantErrCode, err.Code)
		})
	}
}

// ---------------------------------------------------------------------------
// CreateUser – authz checks
// ---------------------------------------------------------------------------

func TestUserService_CreateUser_AuthzChecks(t *testing.T) {
	authzErr := &tidcommon.ServiceError{
		Code:  "SVC-5000",
		Error: tidcommon.I18nMessage{DefaultValue: "authz error"},
	}

	tests := []struct {
		name        string
		setup       func(t *testing.T) *userService
		wantErrCode string
	}{
		{
			name: "AuthzDenied_ReturnsUnauthorized",
			setup: func(t *testing.T) *userService {
				authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
				authzMock.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
					Return(false, (*tidcommon.ServiceError)(nil)).Once()
				return &userService{authzService: authzMock}
			},
			wantErrCode: tidcommon.ErrorUnauthorized.Code,
		},
		{
			name: "AuthzServiceError_ReturnsInternalServerError",
			setup: func(t *testing.T) *userService {
				authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
				authzMock.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
					Return(false, authzErr).Once()
				return &userService{authzService: authzMock}
			},
			wantErrCode: tidcommon.InternalServerError.Code,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := tc.setup(t)
			user := &providers.User{Type: testUserType, OUID: testOrgID}
			resp, err := svc.CreateUser(context.Background(), user)
			require.Nil(t, resp)
			require.NotNil(t, err)
			require.Equal(t, tc.wantErrCode, err.Code)
		})
	}
}

// ---------------------------------------------------------------------------
// GetUser – error paths (store error + authz checks)
// ---------------------------------------------------------------------------

func TestUserService_GetUser_ErrorCases(t *testing.T) {
	userID := svcTestUserID1
	storeErr := errors.New("db error")
	authzErr := &tidcommon.ServiceError{
		Code:  "SVC-5000",
		Error: tidcommon.I18nMessage{DefaultValue: "authz error"},
	}

	tests := []struct {
		name        string
		setup       func(t *testing.T) *userService
		wantErrCode string
	}{
		{
			// GetUser validates that userID is non-empty before calling the store.
			name: "MissingUserID_ReturnsMissingUserIDError",
			setup: func(t *testing.T) *userService {
				return &userService{
					entityService: entitymock.NewEntityServiceInterfaceMock(t),
					authzService:  newAllowAllAuthz(t),
				}
			},
			wantErrCode: ErrorMissingUserID.Code,
		},
		{
			name: "StoreError_ReturnsInternalServerError",
			setup: func(t *testing.T) *userService {
				storeMock := entitymock.NewEntityServiceInterfaceMock(t)
				storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
				storeMock.On("GetEntity", mock.Anything, userID).Return((*providers.Entity)(nil), storeErr).Once()
				return &userService{
					entityService: storeMock,
					authzService:  newAllowAllAuthz(t),
				}
			},
			wantErrCode: tidcommon.InternalServerError.Code,
		},
		{
			name: "AuthzDenied_ReturnsUnauthorized",
			setup: func(t *testing.T) *userService {
				storeMock := entitymock.NewEntityServiceInterfaceMock(t)
				storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
				storeMock.On("GetEntity", mock.Anything, userID).
					Return(&providers.Entity{
						Category: providers.EntityCategoryUser, ID: userID, OUID: testOrgID,
					}, nil).Once()

				authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
				authzMock.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
					Return(false, (*tidcommon.ServiceError)(nil)).Once()

				return &userService{
					entityService: storeMock,
					authzService:  authzMock,
				}
			},
			wantErrCode: tidcommon.ErrorUnauthorized.Code,
		},
		{
			name: "AuthzServiceError_ReturnsInternalServerError",
			setup: func(t *testing.T) *userService {
				storeMock := entitymock.NewEntityServiceInterfaceMock(t)
				storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
				storeMock.On("GetEntity", mock.Anything, userID).
					Return(&providers.Entity{
						Category: providers.EntityCategoryUser, ID: userID, OUID: testOrgID,
					}, nil).Once()

				authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
				authzMock.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
					Return(false, authzErr).Once()

				return &userService{
					entityService: storeMock,
					authzService:  authzMock,
				}
			},
			wantErrCode: tidcommon.InternalServerError.Code,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := tc.setup(t)
			id := userID
			if tc.name == "MissingUserID_ReturnsMissingUserIDError" {
				id = ""
			}
			user, err := svc.GetUser(context.Background(), id, false)
			require.Nil(t, user)
			require.NotNil(t, err)
			require.Equal(t, tc.wantErrCode, err.Code)
		})
	}
}

// ---------------------------------------------------------------------------
// GetUserGroups – authz checks
// ---------------------------------------------------------------------------

func TestUserService_GetUserGroups_AuthzChecks(t *testing.T) {
	userID := svcTestUserID1
	authzErr := &tidcommon.ServiceError{
		Code:  "SVC-5000",
		Error: tidcommon.I18nMessage{DefaultValue: "authz error"},
	}

	tests := []struct {
		name        string
		setup       func(t *testing.T) *userService
		wantErrCode string
	}{
		{
			name: "AuthzDenied_ReturnsUnauthorized",
			setup: func(t *testing.T) *userService {
				storeMock := entitymock.NewEntityServiceInterfaceMock(t)
				storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
				storeMock.On("GetEntityProfile", mock.Anything, userID).
					Return(&providers.Entity{
						Category: providers.EntityCategoryUser, ID: userID, OUID: testOrgID,
					}, nil).Once()

				authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
				authzMock.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
					Return(false, (*tidcommon.ServiceError)(nil)).Once()

				return &userService{
					entityService: storeMock,
					authzService:  authzMock,
				}
			},
			wantErrCode: tidcommon.ErrorUnauthorized.Code,
		},
		{
			name: "AuthzServiceError_ReturnsInternalServerError",
			setup: func(t *testing.T) *userService {
				storeMock := entitymock.NewEntityServiceInterfaceMock(t)
				storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
				storeMock.On("GetEntityProfile", mock.Anything, userID).
					Return(&providers.Entity{
						Category: providers.EntityCategoryUser, ID: userID, OUID: testOrgID,
					}, nil).Once()

				authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
				authzMock.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
					Return(false, authzErr).Once()

				return &userService{
					entityService: storeMock,
					authzService:  authzMock,
				}
			},
			wantErrCode: tidcommon.InternalServerError.Code,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := tc.setup(t)
			resp, err := svc.GetUserGroups(context.Background(), userID, 10, 0)
			require.Nil(t, resp)
			require.NotNil(t, err)
			require.Equal(t, tc.wantErrCode, err.Code)
		})
	}
}

// ---------------------------------------------------------------------------
// UpdateUser – pre-fetch and authz checks
// ---------------------------------------------------------------------------

func TestUserService_UpdateUser_PreFetchAndAuthzChecks(t *testing.T) {
	userID := svcTestUserID1
	storeErr := errors.New("db error")
	authzErr := &tidcommon.ServiceError{
		Code:  "SVC-5000",
		Error: tidcommon.I18nMessage{DefaultValue: "authz error"},
	}
	updatedUser := &providers.User{Type: testUserType, OUID: testOrgID,
		Attributes: json.RawMessage(`{"email":"test@example.com"}`)}

	tests := []struct {
		name        string
		setup       func(t *testing.T) *userService
		wantErrCode string
	}{
		{
			name: "GetUser_NotFound_ReturnsUserNotFound",
			setup: func(t *testing.T) *userService {
				storeMock := entitymock.NewEntityServiceInterfaceMock(t)
				storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
				storeMock.On("GetEntityProfile", mock.Anything, userID).
					Return((*providers.Entity)(nil), entitypkg.ErrEntityNotFound).Once()
				return &userService{
					entityService: storeMock,
					authzService:  newAllowAllAuthz(t),
				}
			},
			wantErrCode: ErrorUserNotFound.Code,
		},
		{
			name: "GetUser_StoreError_ReturnsInternalServerError",
			setup: func(t *testing.T) *userService {
				storeMock := entitymock.NewEntityServiceInterfaceMock(t)
				storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
				storeMock.On("GetEntityProfile", mock.Anything, userID).
					Return((*providers.Entity)(nil), storeErr).Once()
				return &userService{
					entityService: storeMock,
					authzService:  newAllowAllAuthz(t),
				}
			},
			wantErrCode: tidcommon.InternalServerError.Code,
		},
		{
			name: "AuthzDenied_ReturnsUnauthorized",
			setup: func(t *testing.T) *userService {
				storeMock := entitymock.NewEntityServiceInterfaceMock(t)
				storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
				storeMock.On("GetEntityProfile", mock.Anything, userID).
					Return(&providers.Entity{
						Category: providers.EntityCategoryUser, ID: userID, OUID: testOrgID,
					}, nil).Once()

				authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
				authzMock.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
					Return(false, (*tidcommon.ServiceError)(nil)).Once()

				return &userService{
					entityService: storeMock,
					authzService:  authzMock,
				}
			},
			wantErrCode: tidcommon.ErrorUnauthorized.Code,
		},
		{
			name: "AuthzServiceError_ReturnsInternalServerError",
			setup: func(t *testing.T) *userService {
				storeMock := entitymock.NewEntityServiceInterfaceMock(t)
				storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
				storeMock.On("GetEntityProfile", mock.Anything, userID).
					Return(&providers.Entity{
						Category: providers.EntityCategoryUser, ID: userID, OUID: testOrgID,
					}, nil).Once()

				authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
				authzMock.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
					Return(false, authzErr).Once()

				return &userService{
					entityService: storeMock,
					authzService:  authzMock,
				}
			},
			wantErrCode: tidcommon.InternalServerError.Code,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := tc.setup(t)
			resp, err := svc.UpdateUser(context.Background(), userID, updatedUser)
			require.Nil(t, resp)
			require.NotNil(t, err)
			require.Equal(t, tc.wantErrCode, err.Code)
		})
	}
}

// ---------------------------------------------------------------------------
// UpdateUserAttributes – pre-fetch and authz checks
// ---------------------------------------------------------------------------

func TestUserService_UpdateUserAttributes_PreFetchAndAuthzChecks(t *testing.T) {
	userID := svcTestUserID1
	storeErr := errors.New("db error")
	authzErr := &tidcommon.ServiceError{
		Code:  "SVC-5000",
		Error: tidcommon.I18nMessage{DefaultValue: "authz error"},
	}
	attrs := json.RawMessage(`{"email":"new@example.com"}`)

	tests := []struct {
		name        string
		setup       func(t *testing.T) *userService
		wantErrCode string
	}{
		{
			// The first GetUser call (for schema lookup) fails.
			name: "GetUser_StoreError_ReturnsInternalServerError",
			setup: func(t *testing.T) *userService {
				storeMock := entitymock.NewEntityServiceInterfaceMock(t)
				storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
				storeMock.On("GetEntity", mock.Anything, userID).Return((*providers.Entity)(nil), storeErr).Once()
				return &userService{
					entityService: storeMock,
					authzService:  newAllowAllAuthz(t),
				}
			},
			wantErrCode: tidcommon.InternalServerError.Code,
		},
		{
			// GetUser succeeds → authz check reuses the pre-fetched user's OU → authz denies.
			name: "AuthzDenied_ReturnsUnauthorized",
			setup: func(t *testing.T) *userService {
				storeMock := entitymock.NewEntityServiceInterfaceMock(t)
				storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
				storeMock.On("GetEntity", mock.Anything, userID).
					Return(&providers.Entity{
						Category: providers.EntityCategoryUser, ID: userID,
						Type: testUserType, OUID: testOrgID,
					}, nil).Once()

				schemaMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
				schemaMock.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
					entitytype.AttributeFilter{AllowCredential: true}).
					Return([]entitytype.AttributeInfo{}, (*tidcommon.ServiceError)(nil)).Once()

				authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
				authzMock.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
					Return(false, (*tidcommon.ServiceError)(nil)).Once()

				return &userService{
					entityService:     storeMock,
					entityTypeService: schemaMock,
					authzService:      authzMock,
				}
			},
			wantErrCode: tidcommon.ErrorUnauthorized.Code,
		},
		{
			// Same flow as above but authz service returns an error.
			name: "AuthzServiceError_ReturnsInternalServerError",
			setup: func(t *testing.T) *userService {
				storeMock := entitymock.NewEntityServiceInterfaceMock(t)
				storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
				storeMock.On("GetEntity", mock.Anything, userID).
					Return(&providers.Entity{
						Category: providers.EntityCategoryUser, ID: userID,
						Type: testUserType, OUID: testOrgID,
					}, nil).Once()

				schemaMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
				schemaMock.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
					entitytype.AttributeFilter{AllowCredential: true}).
					Return([]entitytype.AttributeInfo{}, (*tidcommon.ServiceError)(nil)).Once()

				authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
				authzMock.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
					Return(false, authzErr).Once()

				return &userService{
					entityService:     storeMock,
					entityTypeService: schemaMock,
					authzService:      authzMock,
				}
			},
			wantErrCode: tidcommon.InternalServerError.Code,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := tc.setup(t)
			resp, err := svc.UpdateUserAttributes(context.Background(), userID, attrs)
			require.Nil(t, resp)
			require.NotNil(t, err)
			require.Equal(t, tc.wantErrCode, err.Code)
		})
	}
}

// ---------------------------------------------------------------------------
// UpdateUserCredentials (batchUpdateUserCredentials) – pre-fetch and authz checks
// ---------------------------------------------------------------------------

func TestUserService_UpdateUserCredentials_PreFetchAndAuthzChecks(t *testing.T) {
	userID := svcTestUserID1
	storeErr := errors.New("db error")
	authzErr := &tidcommon.ServiceError{
		Code:  "SVC-5000",
		Error: tidcommon.I18nMessage{DefaultValue: "authz error"},
	}
	creds := json.RawMessage(`{"password":"newPass"}`)

	tests := []struct { //nolint:dupl
		name        string
		setup       func(t *testing.T) *userService
		wantErrCode string
	}{
		{
			name: "GetUser_StoreError_ReturnsInternalServerError",
			setup: func(t *testing.T) *userService {
				storeMock := entitymock.NewEntityServiceInterfaceMock(t)
				storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
				storeMock.On("GetEntityProfile", mock.Anything, userID).
					Return((*providers.Entity)(nil), storeErr).Once()
				return &userService{
					entityService: storeMock,
					authzService:  newAllowAllAuthz(t),
				}
			},
			wantErrCode: tidcommon.InternalServerError.Code,
		},
		{
			name: "AuthzDenied_ReturnsUnauthorized",
			setup: func(t *testing.T) *userService {
				storeMock := entitymock.NewEntityServiceInterfaceMock(t)
				storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
				storeMock.On("GetEntityProfile", mock.Anything, userID).
					Return(&providers.Entity{
						Category: providers.EntityCategoryUser, ID: userID, OUID: testOrgID,
					}, nil).Once()

				authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
				authzMock.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
					Return(false, (*tidcommon.ServiceError)(nil)).Once()

				return &userService{
					entityService: storeMock,
					authzService:  authzMock,
				}
			},
			wantErrCode: tidcommon.ErrorUnauthorized.Code,
		},
		{
			name: "AuthzServiceError_ReturnsInternalServerError",
			setup: func(t *testing.T) *userService {
				storeMock := entitymock.NewEntityServiceInterfaceMock(t)
				storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
				storeMock.On("GetEntityProfile", mock.Anything, userID).
					Return(&providers.Entity{
						Category: providers.EntityCategoryUser, ID: userID, OUID: testOrgID,
					}, nil).Once()

				authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
				authzMock.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
					Return(false, authzErr).Once()

				return &userService{
					entityService: storeMock,
					authzService:  authzMock,
				}
			},
			wantErrCode: tidcommon.InternalServerError.Code,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := tc.setup(t)
			err := svc.UpdateUserCredentials(context.Background(), userID, creds)
			require.NotNil(t, err)
			require.Equal(t, tc.wantErrCode, err.Code)
		})
	}
}

// ---------------------------------------------------------------------------
// DeleteUser – pre-fetch and authz checks
// ---------------------------------------------------------------------------

func TestUserService_DeleteUser_PreFetchAndAuthzChecks(t *testing.T) {
	userID := svcTestUserID1
	storeErr := errors.New("db error")
	authzErr := &tidcommon.ServiceError{
		Code:  "SVC-5000",
		Error: tidcommon.I18nMessage{DefaultValue: "authz error"},
	}

	tests := []struct { //nolint:dupl
		name        string
		setup       func(t *testing.T) *userService
		wantErrCode string
	}{
		{
			name: "GetUser_StoreError_ReturnsInternalServerError",
			setup: func(t *testing.T) *userService {
				storeMock := entitymock.NewEntityServiceInterfaceMock(t)
				storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
				storeMock.On("GetEntityProfile", mock.Anything, userID).
					Return((*providers.Entity)(nil), storeErr).Once()
				return &userService{
					entityService: storeMock,
					authzService:  newAllowAllAuthz(t),
				}
			},
			wantErrCode: tidcommon.InternalServerError.Code,
		},
		{
			name: "AuthzDenied_ReturnsUnauthorized",
			setup: func(t *testing.T) *userService {
				storeMock := entitymock.NewEntityServiceInterfaceMock(t)
				storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
				storeMock.On("GetEntityProfile", mock.Anything, userID).
					Return(&providers.Entity{
						Category: providers.EntityCategoryUser, ID: userID, OUID: testOrgID,
					}, nil).Once()

				authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
				authzMock.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
					Return(false, (*tidcommon.ServiceError)(nil)).Once()

				return &userService{
					entityService: storeMock,
					authzService:  authzMock,
				}
			},
			wantErrCode: tidcommon.ErrorUnauthorized.Code,
		},
		{
			name: "AuthzServiceError_ReturnsInternalServerError",
			setup: func(t *testing.T) *userService {
				storeMock := entitymock.NewEntityServiceInterfaceMock(t)
				storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
				storeMock.On("GetEntityProfile", mock.Anything, userID).
					Return(&providers.Entity{
						Category: providers.EntityCategoryUser, ID: userID, OUID: testOrgID,
					}, nil).Once()

				authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
				authzMock.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
					Return(false, authzErr).Once()

				return &userService{
					entityService: storeMock,
					authzService:  authzMock,
				}
			},
			wantErrCode: tidcommon.InternalServerError.Code,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := tc.setup(t)
			err := svc.DeleteUser(context.Background(), userID)
			require.NotNil(t, err)
			require.Equal(t, tc.wantErrCode, err.Code)
		})
	}
}

// ---------------------------------------------------------------------------
// DeleteUser – blocking dependency guard
// ---------------------------------------------------------------------------

// newDeletableUserStore builds an entity mock for a user that passes the pre-delete checks.
func newDeletableUserStore(t *testing.T) *entitymock.EntityServiceInterfaceMock {
	t.Helper()
	storeMock := entitymock.NewEntityServiceInterfaceMock(t)
	storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
	storeMock.On("GetEntityProfile", mock.Anything, svcTestUserID1).
		Return(&providers.Entity{
			Category: providers.EntityCategoryUser, ID: svcTestUserID1, OUID: testOrgID,
		}, nil).Once()
	return storeMock
}

func TestUserService_DeleteUser_BlockedByOwnedAgent(t *testing.T) {
	userID := svcTestUserID1
	storeMock := newDeletableUserStore(t)

	service := &userService{
		entityService:      storeMock,
		authzService:       newAllowAllAuthz(t),
		dependencyRegistry: newBlockingDepsRegistry(),
	}

	err := service.DeleteUser(context.Background(), userID)
	require.NotNil(t, err)
	require.Equal(t, ErrorUserHasBlockingDependencies.Code, err.Code)
	require.NotEmpty(t, err.ErrorDescription.Params["dependencies"])
	storeMock.AssertNotCalled(t, "DeleteEntity", mock.Anything, mock.Anything)
}

func TestUserService_DeleteUser_AllowedWhenOnlyNonBlockingDependencies(t *testing.T) {
	userID := svcTestUserID1
	storeMock := newDeletableUserStore(t)
	storeMock.On("DeleteEntity", mock.Anything, userID).Return(nil).Once()

	total := 1
	registry := &stubUsageRegistry{resp: &resourcedependency.DependenciesResponse{
		TotalResults: &total,
		Count:        1,
		Usages: []resourcedependency.ResourceDependency{
			{ResourceType: resourcedependency.ResourceTypeApplication, ID: "app-1",
				DisplayName: "Portal", BehaviorOnDelete: resourcedependency.BehaviorFallback},
		},
	}}

	service := &userService{
		entityService:      storeMock,
		authzService:       newAllowAllAuthz(t),
		dependencyRegistry: registry,
		sharingService:     newPurgingSharing(t, userID),
	}

	err := service.DeleteUser(context.Background(), userID)
	require.Nil(t, err)
	storeMock.AssertNumberOfCalls(t, "DeleteEntity", 1)
}

func TestUserService_DeleteUser_RefusedWhenDependenciesUnknown(t *testing.T) {
	userID := svcTestUserID1
	storeMock := newDeletableUserStore(t)

	// TotalResults nil signals a provider failed to report; deletion must fail closed.
	registry := &stubUsageRegistry{resp: &resourcedependency.DependenciesResponse{
		TotalResults: nil,
		Usages:       []resourcedependency.ResourceDependency{},
	}}

	service := &userService{
		entityService:      storeMock,
		authzService:       newAllowAllAuthz(t),
		dependencyRegistry: registry,
	}

	err := service.DeleteUser(context.Background(), userID)
	require.NotNil(t, err)
	require.Equal(t, tidcommon.InternalServerError.Code, err.Code)
	storeMock.AssertNotCalled(t, "DeleteEntity", mock.Anything, mock.Anything)
}

func TestUserService_DeleteUser_RefusedWhenRegistryUnset(t *testing.T) {
	userID := svcTestUserID1
	storeMock := newDeletableUserStore(t)

	service := &userService{
		entityService: storeMock,
		authzService:  newAllowAllAuthz(t),
	}

	err := service.DeleteUser(context.Background(), userID)
	require.NotNil(t, err)
	require.Equal(t, tidcommon.InternalServerError.Code, err.Code)
	storeMock.AssertNotCalled(t, "DeleteEntity", mock.Anything, mock.Anything)
}

func TestUserService_DeleteUser_AbortedWhenCascadeFails(t *testing.T) {
	userID := svcTestUserID1
	storeMock := newDeletableUserStore(t)

	total := 0
	registry := &stubUsageRegistry{
		resp: &resourcedependency.DependenciesResponse{
			TotalResults: &total, Usages: []resourcedependency.ResourceDependency{},
		},
		cascadeErr: errors.New("cascade delete failed"),
	}

	service := &userService{
		entityService:      storeMock,
		authzService:       newAllowAllAuthz(t),
		dependencyRegistry: registry,
	}

	err := service.DeleteUser(context.Background(), userID)
	require.NotNil(t, err)
	require.Equal(t, tidcommon.InternalServerError.Code, err.Code)
	storeMock.AssertNotCalled(t, "DeleteEntity", mock.Anything, mock.Anything)
}

// TestUpdateUser_DeclarativeResource tests that UpdateUser returns ErrorCannotModifyDeclarativeResource
// when the user is declarative.
func TestUpdateUser_DeclarativeResource(t *testing.T) {
	userID := svcTestDeclarativeUserID1
	updatedUser := providers.User{
		ID:         userID,
		OUID:       "ou1",
		Type:       "employee",
		Attributes: json.RawMessage(`{"name":"test"}`),
	}

	storeMock := entitymock.NewEntityServiceInterfaceMock(t)
	// Mock GetUser for pre-fetch
	storeMock.On("GetEntityProfile", mock.Anything, userID).
		Return(&providers.Entity{
			Category: providers.EntityCategoryUser, ID: userID, OUID: "ou1", Type: "employee",
		}, nil).Once()

	// Mock IsUserDeclarative to return true
	storeMock.On("IsEntityDeclarative", mock.Anything, userID).Return(true, nil).Once()

	service := &userService{
		entityService: storeMock,
		authzService:  newAllowAllAuthz(t),
	}

	_, err := service.UpdateUser(context.Background(), userID, &updatedUser)
	require.NotNil(t, err)
	require.Equal(t, ErrorCannotModifyDeclarativeResource.Code, err.Code)
}

// TestUpdateUser_DeclarativeCheckError tests that UpdateUser surfaces errors from IsUserDeclarative.
func TestUpdateUser_DeclarativeCheckError(t *testing.T) {
	userID := svcTestUserID1
	updatedUser := providers.User{
		ID:         userID,
		OUID:       "ou1",
		Type:       "employee",
		Attributes: json.RawMessage(`{"name":"test"}`),
	}

	storeMock := entitymock.NewEntityServiceInterfaceMock(t)
	// Mock GetUser for pre-fetch
	storeMock.On("GetEntityProfile", mock.Anything, userID).
		Return(&providers.Entity{
			Category: providers.EntityCategoryUser, ID: userID, OUID: "ou1", Type: "employee",
		}, nil).Once()

	// Mock IsUserDeclarative to return an error
	storeErr := errors.New("database connection failed")
	storeMock.On("IsEntityDeclarative", mock.Anything, userID).Return(false, storeErr).Once()

	service := &userService{
		entityService: storeMock,
		authzService:  newAllowAllAuthz(t),
	}

	_, err := service.UpdateUser(context.Background(), userID, &updatedUser)
	require.NotNil(t, err)
	require.Equal(t, tidcommon.InternalServerError.Code, err.Code)
}

// TestUpdateUser_DeclarativeCheckUserNotFound tests that UpdateUser returns ErrorUserNotFound
// when IsUserDeclarative encounters ErrEntityNotFound.
func TestUpdateUser_DeclarativeCheckUserNotFound(t *testing.T) {
	userID := "non-existent-user"
	updatedUser := providers.User{
		ID:         userID,
		OUID:       "ou1",
		Type:       "employee",
		Attributes: json.RawMessage(`{"name":"test"}`),
	}

	storeMock := entitymock.NewEntityServiceInterfaceMock(t)
	// Mock GetUser for pre-fetch
	storeMock.On("GetEntityProfile", mock.Anything, userID).
		Return(&providers.Entity{
			Category: providers.EntityCategoryUser, ID: userID, OUID: "ou1", Type: "employee",
		}, nil).Once()

	// Mock IsUserDeclarative to return ErrEntityNotFound
	storeMock.On("IsEntityDeclarative", mock.Anything, userID).Return(false, entitypkg.ErrEntityNotFound).Once()

	service := &userService{
		entityService: storeMock,
		authzService:  newAllowAllAuthz(t),
	}

	_, err := service.UpdateUser(context.Background(), userID, &updatedUser)
	require.NotNil(t, err)
	require.Equal(t, ErrorUserNotFound.Code, err.Code)
}

// TestUpdateUserAttributes_DeclarativeResource tests that UpdateUserAttributes returns
// ErrorCannotModifyDeclarativeResource when the user is declarative.
func TestUpdateUserAttributes_DeclarativeResource(t *testing.T) {
	userID := svcTestDeclarativeUserID1
	attributes := json.RawMessage(`{"name":"updated"}`)

	storeMock := entitymock.NewEntityServiceInterfaceMock(t)
	// Mock GetUser for pre-fetch
	storeMock.On("GetEntity", mock.Anything, userID).
		Return(&providers.Entity{
			Category: providers.EntityCategoryUser, ID: userID, OUID: "ou1", Type: "employee",
		}, nil).Once()

	// Mock IsUserDeclarative to return true
	storeMock.On("IsEntityDeclarative", mock.Anything, userID).Return(true, nil).Once()

	schemaMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	schemaMock.On("GetAttributes", mock.Anything, mock.Anything, "employee",
		entitytype.AttributeFilter{AllowCredential: true}).
		Return([]entitytype.AttributeInfo{}, (*tidcommon.ServiceError)(nil)).Once()

	service := &userService{
		entityService:     storeMock,
		entityTypeService: schemaMock,
		authzService:      newAllowAllAuthz(t),
	}

	_, err := service.UpdateUserAttributes(context.Background(), userID, attributes)
	require.NotNil(t, err)
	require.Equal(t, ErrorCannotModifyDeclarativeResource.Code, err.Code)
}

// TestUpdateUserAttributes_DeclarativeCheckError tests that UpdateUserAttributes surfaces errors
// from IsUserDeclarative.
func TestUpdateUserAttributes_DeclarativeCheckError(t *testing.T) {
	userID := svcTestUserID1
	attributes := json.RawMessage(`{"name":"updated"}`)

	storeMock := entitymock.NewEntityServiceInterfaceMock(t)
	// Mock GetUser for pre-fetch
	storeMock.On("GetEntity", mock.Anything, userID).
		Return(&providers.Entity{
			Category: providers.EntityCategoryUser, ID: userID, OUID: "ou1", Type: "employee",
		}, nil).Once()

	// Mock IsUserDeclarative to return an error
	storeErr := errors.New("database connection failed")
	storeMock.On("IsEntityDeclarative", mock.Anything, userID).Return(false, storeErr).Once()

	schemaMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	schemaMock.On("GetAttributes", mock.Anything, mock.Anything, "employee",
		entitytype.AttributeFilter{AllowCredential: true}).
		Return([]entitytype.AttributeInfo{}, (*tidcommon.ServiceError)(nil)).Once()

	service := &userService{
		entityService:     storeMock,
		entityTypeService: schemaMock,
		authzService:      newAllowAllAuthz(t),
	}

	_, err := service.UpdateUserAttributes(context.Background(), userID, attributes)
	require.NotNil(t, err)
	require.Equal(t, tidcommon.InternalServerError.Code, err.Code)
}

// TestUpdateUserCredentials_DeclarativeResource tests that UpdateUserCredentials returns
// ErrorCannotModifyDeclarativeResource when the user is declarative.
func TestUpdateUserCredentials_DeclarativeResource(t *testing.T) {
	userID := svcTestDeclarativeUserID1
	credentials := json.RawMessage(`{"password":"newpass123"}`)

	storeMock := entitymock.NewEntityServiceInterfaceMock(t)
	// Mock GetUser for pre-fetch
	storeMock.On("GetEntityProfile", mock.Anything, userID).
		Return(&providers.Entity{
			Category: providers.EntityCategoryUser, ID: userID, OUID: "ou1", Type: "employee",
		}, nil).Once()

	// Mock IsUserDeclarative to return true
	storeMock.On("IsEntityDeclarative", mock.Anything, userID).Return(true, nil).Once()

	entityTypeMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)

	service := &userService{
		entityService:     storeMock,
		entityTypeService: entityTypeMock,
		authzService:      newAllowAllAuthz(t),
	}

	err := service.UpdateUserCredentials(context.Background(), userID, credentials)
	require.NotNil(t, err)
	require.Equal(t, ErrorCannotModifyDeclarativeResource.Code, err.Code)
}

// TestUpdateUserCredentials_DeclarativeCheckError tests that UpdateUserCredentials surfaces errors
// from IsUserDeclarative.
func TestUpdateUserCredentials_DeclarativeCheckError(t *testing.T) {
	userID := svcTestUserID1
	credentials := json.RawMessage(`{"password":"newpass123"}`)

	storeMock := entitymock.NewEntityServiceInterfaceMock(t)
	// Mock GetUser for pre-fetch
	storeMock.On("GetEntityProfile", mock.Anything, userID).
		Return(&providers.Entity{
			Category: providers.EntityCategoryUser, ID: userID, OUID: "ou1", Type: "employee",
		}, nil).Once()

	// Mock IsUserDeclarative to return an error
	storeErr := errors.New("database connection failed")
	storeMock.On("IsEntityDeclarative", mock.Anything, userID).Return(false, storeErr).Once()

	entityTypeMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)

	service := &userService{
		entityService:     storeMock,
		entityTypeService: entityTypeMock,
		authzService:      newAllowAllAuthz(t),
	}

	err := service.UpdateUserCredentials(context.Background(), userID, credentials)
	require.NotNil(t, err)
	require.Equal(t, tidcommon.InternalServerError.Code, err.Code)
}

// TestDeleteUser_DeclarativeResource tests that DeleteUser returns ErrorCannotModifyDeclarativeResource
// when the user is declarative.
func TestDeleteUser_DeclarativeResource(t *testing.T) {
	userID := svcTestDeclarativeUserID1

	storeMock := entitymock.NewEntityServiceInterfaceMock(t)
	// Mock GetUser for pre-fetch
	storeMock.On("GetEntityProfile", mock.Anything, userID).
		Return(&providers.Entity{
			Category: providers.EntityCategoryUser, ID: userID, OUID: "ou1", Type: "employee",
		}, nil).Once()

	// Mock IsUserDeclarative to return true
	storeMock.On("IsEntityDeclarative", mock.Anything, userID).Return(true, nil).Once()

	service := &userService{
		entityService: storeMock,
		authzService:  newAllowAllAuthz(t),
	}

	err := service.DeleteUser(context.Background(), userID)
	require.NotNil(t, err)
	require.Equal(t, ErrorCannotModifyDeclarativeResource.Code, err.Code)
}

// TestDeleteUser_DeclarativeCheckError tests that DeleteUser surfaces errors from IsUserDeclarative.
func TestDeleteUser_DeclarativeCheckError(t *testing.T) {
	userID := svcTestUserID1

	storeMock := entitymock.NewEntityServiceInterfaceMock(t)
	// Mock GetUser for pre-fetch
	storeMock.On("GetEntityProfile", mock.Anything, userID).
		Return(&providers.Entity{
			Category: providers.EntityCategoryUser, ID: userID, OUID: "ou1", Type: "employee",
		}, nil).Once()

	// Mock IsUserDeclarative to return an error
	storeErr := errors.New("database connection failed")
	storeMock.On("IsEntityDeclarative", mock.Anything, userID).Return(false, storeErr).Once()

	service := &userService{
		entityService: storeMock,
		authzService:  newAllowAllAuthz(t),
	}

	err := service.DeleteUser(context.Background(), userID)
	require.NotNil(t, err)
	require.Equal(t, tidcommon.InternalServerError.Code, err.Code)
}

// populateUserDisplayNames Tests

func TestPopulateUserDisplayNames_Success(t *testing.T) {
	schemaMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	schemaMock.On("GetDisplayAttributesByHandles", mock.Anything, mock.Anything, []string{"employee"}).
		Return(map[string]string{"employee": "name"}, (*tidcommon.ServiceError)(nil)).Once()

	service := &userService{entityTypeService: schemaMock}
	users := []providers.User{
		{ID: "user-1", Type: "employee", Attributes: json.RawMessage(`{"name":"Alice"}`)},
		{ID: "user-2", Type: "employee", Attributes: json.RawMessage(`{"name":"Bob"}`)},
	}

	service.populateUserDisplayNames(context.Background(), users, nil)
	require.Equal(t, "Alice", users[0].Display)
	require.Equal(t, "Bob", users[1].Display)
}

func TestPopulateUserDisplayNames_FallbackToID(t *testing.T) {
	schemaMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	schemaMock.On("GetDisplayAttributesByHandles", mock.Anything, mock.Anything, []string{"employee"}).
		Return(map[string]string{"employee": "missing"}, (*tidcommon.ServiceError)(nil)).Once()

	service := &userService{entityTypeService: schemaMock}

	users := []providers.User{
		{ID: "user-1", Type: "employee", Attributes: json.RawMessage(`{"name":"Alice"}`)},
	}

	service.populateUserDisplayNames(context.Background(), users, nil)
	require.Equal(t, "user-1", users[0].Display)
}

func TestPopulateUserDisplayNames_EmptyUsers(t *testing.T) {
	service := &userService{}

	var users []providers.User
	service.populateUserDisplayNames(context.Background(), users, nil)
	// Should not panic.
}

func TestPopulateUserDisplayNames_NilSchemaService(t *testing.T) {
	service := &userService{entityTypeService: nil}

	users := []providers.User{
		{ID: "user-1", Type: "employee", Attributes: json.RawMessage(`{"name":"Alice"}`)},
	}

	service.populateUserDisplayNames(context.Background(), users, nil)
	// Display should fall back to user ID when schema service is nil.
	require.Equal(t, "user-1", users[0].Display)
}

func TestPopulateUserDisplayNames_SchemaServiceError(t *testing.T) {
	schemaMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	schemaMock.On("GetDisplayAttributesByHandles", mock.Anything, mock.Anything, []string{"employee"}).
		Return(map[string]string(nil), &tidcommon.ServiceError{
			Code:  "ERR",
			Error: tidcommon.I18nMessage{DefaultValue: "err"},
		}).Once()

	service := &userService{entityTypeService: schemaMock}

	users := []providers.User{
		{ID: "user-1", Type: "employee", Attributes: json.RawMessage(`{"name":"Alice"}`)},
	}

	service.populateUserDisplayNames(context.Background(), users, nil)
	// Display should fall back to user ID on schema service error.
	require.Equal(t, "user-1", users[0].Display)
}

func TestPopulateUserDisplayNames_MultipleTypes(t *testing.T) {
	schemaMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	schemaMock.On("GetDisplayAttributesByHandles", mock.Anything, mock.Anything,
		mock.MatchedBy(func(names []string) bool {
			if len(names) != 2 {
				return false
			}
			set := map[string]bool{}
			for _, n := range names {
				set[n] = true
			}
			return set["employee"] && set["customer"]
		})).
		Return(map[string]string{
			"employee": "name",
			"customer": "email",
		}, (*tidcommon.ServiceError)(nil)).Once()

	service := &userService{entityTypeService: schemaMock}

	users := []providers.User{
		{ID: "user-1", Type: "employee", Attributes: json.RawMessage(`{"name":"Alice"}`)},
		{ID: "user-2", Type: "customer", Attributes: json.RawMessage(`{"email":"bob@example.com"}`)},
	}

	service.populateUserDisplayNames(context.Background(), users, nil)
	require.Equal(t, "Alice", users[0].Display)
	require.Equal(t, "bob@example.com", users[1].Display)
}

// GetUserList with includeDisplay Tests

func TestUserService_GetUserList_WithIncludeDisplay(t *testing.T) {
	limit := 10
	offset := 0
	filters := map[string]interface{}{}

	storeMock := entitymock.NewEntityServiceInterfaceMock(t)
	storeMock.On("GetEntityListCount", mock.Anything, providers.EntityCategoryUser, filters).Return(2, nil).Once()
	storeMock.On("GetEntityList", mock.Anything, providers.EntityCategoryUser, limit, offset, filters).
		Return([]providers.Entity{
			{
				ID: "user-1", OUID: "ou-1", Type: "employee",
				Attributes: json.RawMessage(`{"name":"Alice"}`),
			},
			{
				ID: "user-2", OUID: "ou-2", Type: "employee",
				Attributes: json.RawMessage(`{"name":"Bob"}`),
			},
		}, nil).Once()

	schemaMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	schemaMock.On("GetDisplayAttributesByHandles", mock.Anything, mock.Anything, []string{"employee"}).
		Return(map[string]string{"employee": "name"}, (*tidcommon.ServiceError)(nil)).Once()

	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	ouServiceMock.On("GetOrganizationUnitHandlesByIDs", mock.Anything,
		mock.MatchedBy(func(ids []string) bool {
			if len(ids) != 2 {
				return false
			}
			expected := map[string]bool{"ou-1": true, "ou-2": true}
			return expected[ids[0]] && expected[ids[1]]
		}),
	).Return(map[string]string{"ou-1": "engineering", "ou-2": "sales"}, nil).Once()

	service := &userService{
		entityService:     storeMock,
		entityTypeService: schemaMock,
		ouService:         ouServiceMock,
		authzService:      newAllowAllAuthz(t),
		governance:        activeGovernance(t),
	}

	resp, err := service.GetUserList(context.Background(), limit, offset, filters, true)
	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Users, 2)
	require.Equal(t, "Alice", resp.Users[0].Display)
	require.Equal(t, "engineering", resp.Users[0].OUHandle)
	require.Equal(t, "Bob", resp.Users[1].Display)
	require.Equal(t, "sales", resp.Users[1].OUHandle)
}

// TestResolveUserOUHandle_OUHandleResolved verifies that when only ou_handle is set,
// it is resolved to ou_id via the OU service.
func TestResolveUserOUHandle_OUHandleResolved(t *testing.T) {
	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	ouServiceMock.On("GetOrganizationUnitByPath", mock.Anything, "default").
		Return(oupkg.OrganizationUnit{ID: "ou-resolved"}, (*tidcommon.ServiceError)(nil)).Once()

	svc := &userService{ouService: ouServiceMock}
	u := &providers.User{OUHandle: "default"}

	svcErr := svc.ResolveUserOUHandle(context.Background(), u)

	require.Nil(t, svcErr)
	require.Equal(t, "ou-resolved", u.OUID)
}

// TestResolveUserOUHandle_OUIDAlreadySet verifies that no resolution happens when
// ou_id is set and ou_handle is empty.
func TestResolveUserOUHandle_OUIDAlreadySet(t *testing.T) {
	svc := &userService{}
	u := &providers.User{OUID: "ou-direct"}

	svcErr := svc.ResolveUserOUHandle(context.Background(), u)

	require.Nil(t, svcErr)
	require.Equal(t, "ou-direct", u.OUID)
}

// TestResolveUserOUHandle_BothProvided verifies that when both ou_id and ou_handle are
// provided, ou_id is retained and the OU service is never called.
func TestResolveUserOUHandle_BothProvided(t *testing.T) {
	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)

	svc := &userService{ouService: ouServiceMock}
	u := &providers.User{ID: "u1", OUID: "ou-direct", OUHandle: "default"}

	svcErr := svc.ResolveUserOUHandle(context.Background(), u)

	require.Nil(t, svcErr)
	require.Equal(t, "ou-direct", u.OUID)
	// Cleanup-time AssertExpectations confirms GetOrganizationUnitByPath was never called.
}

// TestResolveUserOUHandle_OUHandleNotFound verifies that a not-found response from the OU
// service is surfaced as ErrorInvalidRequestFormat.
func TestResolveUserOUHandle_OUHandleNotFound(t *testing.T) {
	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	ouServiceMock.On("GetOrganizationUnitByPath", mock.Anything, "missing").
		Return(oupkg.OrganizationUnit{}, &oupkg.ErrorOrganizationUnitNotFound).Once()

	svc := &userService{ouService: ouServiceMock}
	u := &providers.User{OUHandle: "missing"}

	svcErr := svc.ResolveUserOUHandle(context.Background(), u)

	require.NotNil(t, svcErr)
	require.Equal(t, ErrorInvalidRequestFormat.Code, svcErr.Code)
}

// TestResolveUserOUHandle_NeitherProvided verifies the call is a no-op when neither
// ou_id nor ou_handle is provided.
func TestResolveUserOUHandle_NeitherProvided(t *testing.T) {
	svc := &userService{}
	u := &providers.User{}

	svcErr := svc.ResolveUserOUHandle(context.Background(), u)

	require.Nil(t, svcErr)
	require.Empty(t, u.OUID)
}

// TestResolveUserOUHandle_NilOUService verifies that a clear error is returned when the OU
// service is nil and ou_handle is supplied (no nil-pointer panic).
func TestResolveUserOUHandle_NilOUService(t *testing.T) {
	svc := &userService{ouService: nil}
	u := &providers.User{OUHandle: "default"}

	svcErr := svc.ResolveUserOUHandle(context.Background(), u)

	require.NotNil(t, svcErr)
	require.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)
}

// TestUserService_GetUser_DisplayOUHandleError verifies that GetUser falls back gracefully
// when resolving the OU handle for display fails.
func TestUserService_GetUser_DisplayOUHandleError(t *testing.T) {
	userID := svcTestUserID1
	expectedEntity := &providers.Entity{
		Category:   providers.EntityCategoryUser,
		ID:         userID,
		OUID:       testOrgID,
		Type:       "employee",
		Attributes: json.RawMessage(`{"email":"alice@example.com"}`),
	}

	storeMock := entitymock.NewEntityServiceInterfaceMock(t)
	storeMock.On("GetEntity", mock.Anything, userID).Return(expectedEntity, nil).Once()

	mockSchema := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockSchema.On("GetDisplayAttributesByHandles", mock.Anything, mock.Anything, []string{"employee"}).
		Return(map[string]string{"employee": "email"}, (*tidcommon.ServiceError)(nil)).Once()

	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	ouServiceMock.On("GetOrganizationUnitHandlesByIDs", mock.Anything, []string{testOrgID}).
		Return(map[string]string(nil), &tidcommon.InternalServerError).Once()

	service := &userService{
		entityService:     storeMock,
		authzService:      newAllowAllAuthz(t),
		entityTypeService: mockSchema,
		ouService:         ouServiceMock,
		governance:        activeGovernance(t),
	}

	user, err := service.GetUser(context.Background(), userID, true)
	require.Nil(t, err)
	require.Equal(t, "alice@example.com", user.Display)
	require.Empty(t, user.OUHandle)
}

// TestUserService_GetUserGroups_GroupsStoreError verifies that an error from GetEntityGroups
// surfaces as an internal server error.
func TestUserService_GetUserGroups_GroupsStoreError(t *testing.T) {
	userID := svcTestUserID123
	limit, offset := 10, 0

	mockStore := entitymock.NewEntityServiceInterfaceMock(t)
	mockStore.On("GetEntityProfile", mock.Anything, userID).
		Return(&providers.Entity{
			Category: providers.EntityCategoryUser, ID: userID, OUID: testOrgID,
		}, nil).Once()
	mockStore.On("GetGroupCountForEntity", mock.Anything, userID).Return(5, nil).Once()
	mockStore.On("GetEntityGroups", mock.Anything, userID, limit, offset).
		Return(([]providers.EntityGroup)(nil), errors.New("db error")).Once()

	service := &userService{
		entityService: mockStore,
		authzService:  newAllowAllAuthz(t),
	}

	_, err := service.GetUserGroups(context.Background(), userID, limit, offset)
	require.NotNil(t, err)
	require.Equal(t, tidcommon.InternalServerError.Code, err.Code)
}

// TestUserService_DeleteUser_NotFoundOnDelete verifies that a not-found error from the
// delete call is mapped to the user-not-found error.
func TestUserService_DeleteUser_NotFoundOnDelete(t *testing.T) {
	userID := svcTestUserID1

	storeMock := entitymock.NewEntityServiceInterfaceMock(t)
	storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
	storeMock.On("GetEntityProfile", mock.Anything, userID).
		Return(&providers.Entity{
			Category: providers.EntityCategoryUser, ID: userID, OUID: testOrgID,
		}, nil).Once()
	storeMock.On("DeleteEntity", mock.Anything, userID).Return(entitypkg.ErrEntityNotFound).Once()

	service := &userService{
		entityService:      storeMock,
		authzService:       newAllowAllAuthz(t),
		dependencyRegistry: newNoBlockingDepsRegistry(),
		sharingService:     newPurgingSharing(t, userID),
	}

	err := service.DeleteUser(context.Background(), userID)
	require.NotNil(t, err)
	require.Equal(t, ErrorUserNotFound.Code, err.Code)
}

// TestPopulateOUHandles_HandleResolutionError verifies that populateOUHandles returns early
// without setting handles when the OU service fails.
func TestPopulateOUHandles_HandleResolutionError(t *testing.T) {
	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	ouServiceMock.On("GetOrganizationUnitHandlesByIDs", mock.Anything, []string{testOrgID}).
		Return(map[string]string(nil), &tidcommon.InternalServerError).Once()

	service := &userService{ouService: ouServiceMock}
	users := []providers.User{{ID: "user-1", OUID: testOrgID}}

	service.populateOUHandles(context.Background(), users, log.GetLogger())
	require.Empty(t, users[0].OUHandle)
}

// TestValidateOrganizationUnitForUserType_NilEntityTypeService verifies that a missing entity
// type service yields an internal server error after the OU existence check passes.
func TestValidateOrganizationUnitForUserType_NilEntityTypeService(t *testing.T) {
	ouID := "2b4f9c1e-2222-4c19-9a94-5866df9b6bf5"
	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	ouServiceMock.On("IsOrganizationUnitExists", mock.Anything, ouID).
		Return(true, (*tidcommon.ServiceError)(nil)).Once()

	service := &userService{ouService: ouServiceMock, entityTypeService: nil}

	err := service.validateOrganizationUnitForUserType(context.Background(), testUserType, ouID, log.GetLogger())
	require.NotNil(t, err)
	require.Equal(t, tidcommon.InternalServerError.Code, err.Code)
}

// TestValidateOrganizationUnitForUserType_EntityTypeLookupError verifies that an unexpected
// entity type service error yields an internal server error.
func TestValidateOrganizationUnitForUserType_EntityTypeLookupError(t *testing.T) {
	ouID := "3c5fa02d-3333-4ea0-a317-3d8579346d86"
	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	ouServiceMock.On("IsOrganizationUnitExists", mock.Anything, ouID).
		Return(true, (*tidcommon.ServiceError)(nil)).Once()

	entityTypeMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	entityTypeMock.On("GetEntityTypeByHandle", mock.Anything, mock.Anything, testUserType).
		Return((*entitytype.EntityType)(nil), &tidcommon.InternalServerError).Once()

	service := &userService{ouService: ouServiceMock, entityTypeService: entityTypeMock}

	err := service.validateOrganizationUnitForUserType(context.Background(), testUserType, ouID, log.GetLogger())
	require.NotNil(t, err)
	require.Equal(t, tidcommon.InternalServerError.Code, err.Code)
}

// TestValidateOrganizationUnitForUserType_NilEntityType verifies that a nil entity type
// response yields an internal server error.
func TestValidateOrganizationUnitForUserType_NilEntityType(t *testing.T) {
	ouID := "4d60b13e-4444-4ea0-a317-3d8579346d86"
	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	ouServiceMock.On("IsOrganizationUnitExists", mock.Anything, ouID).
		Return(true, (*tidcommon.ServiceError)(nil)).Once()

	entityTypeMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	entityTypeMock.On("GetEntityTypeByHandle", mock.Anything, mock.Anything, testUserType).
		Return((*entitytype.EntityType)(nil), (*tidcommon.ServiceError)(nil)).Once()

	service := &userService{ouService: ouServiceMock, entityTypeService: entityTypeMock}

	err := service.validateOrganizationUnitForUserType(context.Background(), testUserType, ouID, log.GetLogger())
	require.NotNil(t, err)
	require.Equal(t, tidcommon.InternalServerError.Code, err.Code)
}

// TestMapOUServiceError_UnmappedClientError verifies that an unmapped client-type OU error is
// logged and mapped to an internal server error.
func TestMapOUServiceError_UnmappedClientError(t *testing.T) {
	clientErr := &tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "OU-UNMAPPED-1",
	}

	result := mapOUServiceError(context.Background(), clientErr, log.GetLogger(),
		"performing an operation", map[string]*tidcommon.ServiceError{})

	require.NotNil(t, result)
	require.Equal(t, tidcommon.InternalServerError.Code, result.Code)
}

// TestUserDeclarativeYAML_OUHandleParsed verifies that ou_handle is parsed off the YAML
// document into the user declarative resource.
func TestUserDeclarativeYAML_OUHandleParsed(t *testing.T) {
	yamlData := []byte("" +
		"id: user-1\n" +
		"type: person\n" +
		"ouHandle: default\n" +
		"attributes:\n" +
		"  username: alice\n")

	user, _, err := parseToUser(yamlData)

	require.NoError(t, err)
	require.Equal(t, "default", user.OUHandle)
	require.Empty(t, user.OUID)
}

// --- GetUserUsages tests ---

// stubUsageRegistry is a minimal resourcedependency.Registry for tests.
type stubUsageRegistry struct {
	resp       *resourcedependency.DependenciesResponse
	err        error
	cascadeErr error
}

func (s *stubUsageRegistry) RegisterProvider(resourcedependency.Provider) {}

func (s *stubUsageRegistry) GetDependencies(
	_ context.Context, _, _ string) (*resourcedependency.DependenciesResponse, error) {
	return s.resp, s.err
}

func (s *stubUsageRegistry) CascadeDelete(_ context.Context, _, _ string) (int, error) {
	return 0, s.cascadeErr
}

func (s *stubUsageRegistry) ValidateReferenceUpdate(
	_ context.Context, _, _ string) *tidcommon.ServiceError {
	return nil
}

func newUserForUsages(id string) *providers.Entity {
	return &providers.Entity{ID: id, Category: providers.EntityCategoryUser, Type: "Person"}
}

// newNoBlockingDepsRegistry returns a registry reporting confirmed-empty dependencies, so that
// deletion is permitted by the blocking guard.
func newNoBlockingDepsRegistry() *stubUsageRegistry {
	total := 0
	return &stubUsageRegistry{resp: &resourcedependency.DependenciesResponse{
		TotalResults: &total,
		Usages:       []resourcedependency.ResourceDependency{},
	}}
}

// newBlockingDepsRegistry returns a registry reporting a single blocking (restrict) dependency.
func newBlockingDepsRegistry() *stubUsageRegistry {
	total := 1
	return &stubUsageRegistry{resp: &resourcedependency.DependenciesResponse{
		TotalResults: &total,
		Count:        1,
		Summary:      map[string]int{resourcedependency.ResourceTypeAgent: 1},
		Usages: []resourcedependency.ResourceDependency{
			{ResourceType: resourcedependency.ResourceTypeAgent, ID: "agent-1",
				DisplayName: "Support Agent", BehaviorOnDelete: resourcedependency.BehaviorRestrict},
		},
	}}
}

func TestUserService_GetUserUsages_MissingID(t *testing.T) {
	service := &userService{}

	result, err := service.GetUserUsages(context.Background(), "")
	require.Nil(t, result)
	require.NotNil(t, err)
	require.Equal(t, ErrorMissingUserID.Code, err.Code)
}

func TestUserService_GetUserUsages_NotFound(t *testing.T) {
	entityMock := entitymock.NewEntityServiceInterfaceMock(t)
	entityMock.On("GetEntityProfile", mock.Anything, svcTestUserID1).
		Return((*providers.Entity)(nil), entitypkg.ErrEntityNotFound).Once()

	service := &userService{entityService: entityMock}

	result, err := service.GetUserUsages(context.Background(), svcTestUserID1)
	require.Nil(t, result)
	require.NotNil(t, err)
	require.Equal(t, ErrorUserNotFound.Code, err.Code)
}

func TestUserService_GetUserUsages_WrongCategory(t *testing.T) {
	entityMock := entitymock.NewEntityServiceInterfaceMock(t)
	entityMock.On("GetEntityProfile", mock.Anything, svcTestUserID1).
		Return(&providers.Entity{ID: svcTestUserID1, Category: providers.EntityCategoryAgent}, nil).Once()

	service := &userService{entityService: entityMock}

	result, err := service.GetUserUsages(context.Background(), svcTestUserID1)
	require.Nil(t, result)
	require.NotNil(t, err)
	require.Equal(t, ErrorUserNotFound.Code, err.Code)
}

func TestUserService_GetUserUsages_RegistryNotSet(t *testing.T) {
	entityMock := entitymock.NewEntityServiceInterfaceMock(t)
	entityMock.On("GetEntityProfile", mock.Anything, svcTestUserID1).
		Return(newUserForUsages(svcTestUserID1), nil).Once()

	service := &userService{entityService: entityMock}

	result, err := service.GetUserUsages(context.Background(), svcTestUserID1)
	require.Nil(t, err)
	require.NotNil(t, result)
	require.Nil(t, result.TotalResults)
	require.Nil(t, result.Summary)
	require.Empty(t, result.Usages)
}

func TestUserService_GetUserUsages_WithUsages(t *testing.T) {
	entityMock := entitymock.NewEntityServiceInterfaceMock(t)
	entityMock.On("GetEntityProfile", mock.Anything, svcTestUserID1).
		Return(newUserForUsages(svcTestUserID1), nil).Once()

	total := 1
	service := &userService{
		entityService: entityMock,
		dependencyRegistry: &stubUsageRegistry{
			resp: &resourcedependency.DependenciesResponse{
				TotalResults: &total,
				Count:        1,
				Summary:      map[string]int{resourcedependency.ResourceTypeAgent: 1},
				Usages: []resourcedependency.ResourceDependency{
					{ResourceType: resourcedependency.ResourceTypeAgent, ID: "agent-1",
						DisplayName: "Support Agent", BehaviorOnDelete: resourcedependency.BehaviorFallback},
				},
			},
		},
	}

	result, err := service.GetUserUsages(context.Background(), svcTestUserID1)
	require.Nil(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.TotalResults)
	require.Equal(t, 1, *result.TotalResults)
	require.Equal(t, 1, result.Summary[resourcedependency.ResourceTypeAgent])
	require.Len(t, result.Usages, 1)
	require.Equal(t, resourcedependency.ResourceTypeAgent, result.Usages[0].ResourceType)
	require.Equal(t, "agent-1", result.Usages[0].ID)
}

func TestUserService_GetUserUsages_RegistryError(t *testing.T) {
	entityMock := entitymock.NewEntityServiceInterfaceMock(t)
	entityMock.On("GetEntityProfile", mock.Anything, svcTestUserID1).
		Return(newUserForUsages(svcTestUserID1), nil).Once()

	service := &userService{
		entityService:      entityMock,
		dependencyRegistry: &stubUsageRegistry{err: errors.New("registry error")},
	}

	result, err := service.GetUserUsages(context.Background(), svcTestUserID1)
	require.Nil(t, result)
	require.NotNil(t, err)
}

func TestGetUserMetadata_Success(t *testing.T) {
	entityTypeMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	expectedSchema := &entitytype.EntityType{
		Handle: testUserType,
		Schema: json.RawMessage(`{"email":{"type":"string"}}`),
	}
	entityTypeMock.On("GetEntityTypeSchema", mock.Anything, entitytype.TypeCategoryUser, testUserType).
		Return(expectedSchema, (*tidcommon.ServiceError)(nil)).Once()

	entityServiceMock := entitymock.NewEntityServiceInterfaceMock(t)
	entityUser := &providers.Entity{
		ID:         svcTestUserID123,
		Category:   providers.EntityCategoryUser,
		Type:       testUserType,
		Attributes: json.RawMessage(`{"username":"test"}`),
	}
	entityServiceMock.On("GetEntity", mock.Anything, svcTestUserID123).
		Return(entityUser, nil).Once()

	service := &userService{
		entityService:     entityServiceMock,
		entityTypeService: entityTypeMock,
		authzService:      newAllowAllAuthz(t),
		governance:        activeGovernance(t),
	}

	schema, svcErr := service.GetUserMetadata(context.Background(), svcTestUserID123)
	require.Nil(t, svcErr)
	require.NotNil(t, schema)
	require.Equal(t, testUserType, schema.Handle)
}

func TestGetUserMetadata_GetUserError(t *testing.T) {
	entityServiceMock := entitymock.NewEntityServiceInterfaceMock(t)
	entityServiceMock.On("GetEntity", mock.Anything, svcTestUserID123).
		Return((*providers.Entity)(nil), entitypkg.ErrEntityNotFound).Once()

	service := &userService{
		entityService: entityServiceMock,
		authzService:  newAllowAllAuthz(t),
	}

	schema, svcErr := service.GetUserMetadata(context.Background(), svcTestUserID123)
	require.Nil(t, schema)
	require.NotNil(t, svcErr)
	require.Equal(t, ErrorUserNotFound.Code, svcErr.Code)
}

func TestGetUserMetadata_GetEntityTypeSchemaError(t *testing.T) {
	entityServiceMock := entitymock.NewEntityServiceInterfaceMock(t)
	entityUser := &providers.Entity{
		ID:         svcTestUserID123,
		Category:   providers.EntityCategoryUser,
		Type:       testUserType,
		Attributes: json.RawMessage(`{"username":"test"}`),
	}
	entityServiceMock.On("GetEntity", mock.Anything, svcTestUserID123).
		Return(entityUser, nil).Once()

	entityTypeMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	entityTypeMock.On("GetEntityTypeSchema", mock.Anything, entitytype.TypeCategoryUser, testUserType).
		Return((*entitytype.EntityType)(nil), &entitytype.ErrorEntityTypeNotFound).Once()

	service := &userService{
		entityService:     entityServiceMock,
		entityTypeService: entityTypeMock,
		authzService:      newAllowAllAuthz(t),
		governance:        activeGovernance(t),
	}

	schema, svcErr := service.GetUserMetadata(context.Background(), svcTestUserID123)
	require.Nil(t, schema)
	require.NotNil(t, svcErr)
	require.Equal(t, entitytype.ErrorEntityTypeNotFound.Code, svcErr.Code)
}

// unsharedSharing returns a sharing service that reports the user holds no sharing policy.
func unsharedSharing(t *testing.T) *sharingmock.SharingServiceInterfaceMock {
	sharingMock := sharingmock.NewSharingServiceInterfaceMock(t)
	sharingMock.On("ListPolicies", mock.Anything, UserSharingType, mock.Anything).
		Return([]sharing.Policy{}, (*tidcommon.ServiceError)(nil)).Maybe()
	return sharingMock
}

// A shared user stays in its organization unit, since its policies were issued against that unit.
func TestSharingPolicy_ASharedUserCannotMove(t *testing.T) {
	storeMock := entitymock.NewEntityServiceInterfaceMock(t)
	storeMock.On("GetEntityProfile", mock.Anything, svcTestUserID1).Return(&providers.Entity{
		Category: providers.EntityCategoryUser, ID: svcTestUserID1, OUID: shareOwnerOU, Type: testUserType,
	}, nil).Once()
	storeMock.On("IsEntityDeclarative", mock.Anything, svcTestUserID1).Return(false, nil).Maybe()
	sharingMock := sharingmock.NewSharingServiceInterfaceMock(t)
	sharingMock.On("ListPolicies", mock.Anything, UserSharingType, svcTestUserID1).
		Return([]sharing.Policy{userPolicy(shareOwnerOU)}, (*tidcommon.ServiceError)(nil)).Once()
	service := &userService{
		entityService:  storeMock,
		authzService:   newAllowAllAuthz(t),
		sharingService: sharingMock,
		governance:     activeGovernance(t),
	}

	_, err := service.UpdateUser(context.Background(), svcTestUserID1,
		&providers.User{OUID: shareChildOU, Type: testUserType})

	require.NotNil(t, err)
	require.Equal(t, ErrorSharedUserCannotMove.Code, err.Code)
}

// newPurgingSharing returns a sharing service that expects the policy purge a user delete runs.
func newPurgingSharing(t *testing.T, userID string) *sharingmock.SharingServiceInterfaceMock {
	sharingMock := sharingmock.NewSharingServiceInterfaceMock(t)
	sharingMock.On("DeletePoliciesForResource", mock.Anything, UserSharingType, userID).
		Return((*tidcommon.ServiceError)(nil)).Once()
	return sharingMock
}

const (
	shareOwnerOU  = "ou-owner"
	shareChildOU  = "ou-child"
	shareOtherOU  = "ou-other"
	sharePolicyID = "policy-1"
)

// sharingFixture is a user service wired for sharing calls. Its caller may act only in the
// organization units listed per action, and every check it makes is recorded.
type sharingFixture struct {
	service *userService
	sharing *sharingmock.SharingServiceInterfaceMock
	checks  []*sysauthz.ActionContext
	// userDeclared says whether the fixture's user is declared in a resource file.
	userDeclared bool
}

// withUserStore runs the rest of the test with users in the given store mode; empty is the default.
func withUserStore(t *testing.T, mode string) {
	t.Helper()
	config.ResetServerRuntime()
	require.NoError(t, config.InitializeServerRuntime("test", &config.Config{User: config.UserConfig{Store: mode}}))
	t.Cleanup(config.ResetServerRuntime)
}

func newSharingFixture(t *testing.T, allowed map[security.Action][]string) *sharingFixture {
	f := &sharingFixture{sharing: sharingmock.NewSharingServiceInterfaceMock(t)}
	withUserStore(t, "")

	store := entitymock.NewEntityServiceInterfaceMock(t)
	store.On("GetEntity", mock.Anything, svcTestUserID1).Return(&providers.Entity{
		Category: providers.EntityCategoryUser, ID: svcTestUserID1, OUID: shareOwnerOU,
	}, nil).Maybe()
	store.On("GetEntity", mock.Anything, mock.Anything).Return(nil, entitypkg.ErrEntityNotFound).Maybe()
	store.EXPECT().IsEntityDeclarative(mock.Anything, svcTestUserID1).RunAndReturn(
		func(context.Context, string) (bool, error) { return f.userDeclared, nil }).Maybe()

	authz := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
	authz.EXPECT().IsActionAllowed(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, action security.Action, ac *sysauthz.ActionContext) (bool, *tidcommon.ServiceError) {
			if security.IsRuntimeContext(ctx) {
				return true, nil
			}
			f.checks = append(f.checks, ac)
			return slices.Contains(allowed[action], ac.OUID), nil
		}).Maybe()

	f.service = &userService{
		governance: activeGovernance(t), entityService: store, authzService: authz, sharingService: f.sharing}
	return f
}

// userPolicy is a policy over the fixture's user, made by the given organization unit.
func userPolicy(initiatingOUID string) sharing.Policy {
	return sharing.Policy{
		ID: sharePolicyID, ResourceType: UserSharingType, ResourceID: svcTestUserID1,
		OwningOUID: shareOwnerOU, InitiatingOUID: initiatingOUID, Version: 1,
	}
}

func TestSharingPolicy_CreateIsAuthorizedForTheInitiatingOU(t *testing.T) {
	cases := []struct {
		name       string
		allowedIn  string
		initiating string
		allowed    bool
	}{
		{name: "OwnerSharesItsUser", allowedIn: shareOwnerOU, allowed: true},
		{name: "ChildReshares", allowedIn: shareChildOU, initiating: shareChildOU, allowed: true},
		{name: "ChildCannotShareForTheOwner", allowedIn: shareChildOU},
		{name: "OwnerCannotReshareForTheChild", allowedIn: shareOwnerOU, initiating: shareChildOU},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newSharingFixture(t, map[security.Action][]string{security.ActionUpdateUser: {tc.allowedIn}})
			req := sharing.PolicyRequest{InitiatingOUID: tc.initiating}
			if tc.allowed {
				f.sharing.On("CreatePolicy", mock.Anything, UserSharingType, svcTestUserID1, shareOwnerOU, req).
					Return(userPolicy(tc.allowedIn), (*tidcommon.ServiceError)(nil)).Once()
			}

			_, err := f.service.CreateSharingPolicy(context.Background(), svcTestUserID1, req)

			if tc.allowed {
				require.Nil(t, err)
				return
			}
			require.NotNil(t, err)
			require.Equal(t, tidcommon.ErrorUnauthorized.Code, err.Code)
		})
	}
}

// A declared user's own sharing is declared with it, so the owner's policy is refused here.
func TestSharingPolicy_ADeclaredUsersOwnPolicyIsRefused(t *testing.T) {
	f := newSharingFixture(t, map[security.Action][]string{security.ActionUpdateUser: {shareOwnerOU}})
	f.userDeclared = true

	_, err := f.service.CreateSharingPolicy(context.Background(), svcTestUserID1, sharing.PolicyRequest{})

	require.NotNil(t, err)
	require.Equal(t, ErrorCannotModifyDeclarativeResource.Code, err.Code)
}

// A unit a declared user is shared into may still reshare: there is nowhere else to record that.
func TestSharingPolicy_AUnitMayReshareADeclaredUser(t *testing.T) {
	f := newSharingFixture(t, map[security.Action][]string{security.ActionUpdateUser: {shareChildOU}})
	f.userDeclared = true
	req := sharing.PolicyRequest{InitiatingOUID: shareChildOU}
	f.sharing.On("CreatePolicy", mock.Anything, UserSharingType, svcTestUserID1, shareOwnerOU, req).
		Return(userPolicy(shareChildOU), (*tidcommon.ServiceError)(nil)).Once()

	_, err := f.service.CreateSharingPolicy(context.Background(), svcTestUserID1, req)

	require.Nil(t, err)
}

// Users that load from resource files only have all their sharing declared there, so no policy is
// written, changed or removed here.
func TestSharingPolicy_UsersFromFilesOnlyRefuseEveryWrite(t *testing.T) {
	f := newSharingFixture(t, map[security.Action][]string{security.ActionUpdateUser: {shareOwnerOU}})
	withUserStore(t, "declarative")
	f.sharing.On("GetPolicy", mock.Anything, sharePolicyID).
		Return(userPolicy(shareOwnerOU), (*tidcommon.ServiceError)(nil))
	ctx := context.Background()

	_, createErr := f.service.CreateSharingPolicy(ctx, svcTestUserID1, sharing.PolicyRequest{})
	_, updateErr := f.service.UpdateSharingPolicy(ctx, svcTestUserID1, sharePolicyID, sharing.PolicyRequest{})
	deleteErr := f.service.DeleteSharingPolicy(ctx, svcTestUserID1, sharePolicyID)

	for _, err := range []*tidcommon.ServiceError{createErr, updateErr, deleteErr} {
		require.NotNil(t, err)
		require.Equal(t, ErrorCannotModifyDeclarativeResource.Code, err.Code)
	}
}

func TestSharingPolicy_ChecksNeverNameTheUser(t *testing.T) {
	f := newSharingFixture(t, map[security.Action][]string{security.ActionUpdateUser: {shareOwnerOU}})
	f.sharing.On("CreatePolicy", mock.Anything, UserSharingType, svcTestUserID1, shareOwnerOU, mock.Anything).
		Return(userPolicy(shareOwnerOU), (*tidcommon.ServiceError)(nil)).Once()

	_, err := f.service.CreateSharingPolicy(context.Background(), svcTestUserID1, sharing.PolicyRequest{})

	require.Nil(t, err)
	require.NotEmpty(t, f.checks)
	for _, check := range f.checks {
		require.Empty(t, check.ResourceID, "a sharing check must not let users act on their own record")
	}
}

func TestSharingPolicy_UnknownUserAnswersNotFound(t *testing.T) {
	f := newSharingFixture(t, map[security.Action][]string{security.ActionUpdateUser: {shareOwnerOU}})

	_, err := f.service.CreateSharingPolicy(context.Background(), "missing-user", sharing.PolicyRequest{})

	require.NotNil(t, err)
	require.Equal(t, ErrorUserNotFound.Code, err.Code)
}

func TestSharingPolicy_AnotherUsersPolicyIsNotFound(t *testing.T) {
	f := newSharingFixture(t, map[security.Action][]string{security.ActionReadUser: {shareOwnerOU}})
	foreign := userPolicy(shareOwnerOU)
	foreign.ResourceID = "user-2"
	f.sharing.On("GetPolicy", mock.Anything, sharePolicyID).Return(foreign, (*tidcommon.ServiceError)(nil)).Once()

	_, err := f.service.GetSharingPolicy(context.Background(), svcTestUserID1, sharePolicyID)

	require.NotNil(t, err)
	require.Equal(t, ErrorSharingPolicyNotFound.Code, err.Code)
}

func TestSharingPolicy_GetIsReadableByTheOwnerOrTheInitiator(t *testing.T) {
	cases := []struct {
		name      string
		allowedIn string
		readable  bool
	}{
		{name: "OwnerReadsAReshare", allowedIn: shareOwnerOU, readable: true},
		{name: "ChildReadsItsOwnReshare", allowedIn: shareChildOU, readable: true},
		{name: "UnrelatedUnitCannotRead", allowedIn: shareOtherOU},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newSharingFixture(t, map[security.Action][]string{security.ActionReadUser: {tc.allowedIn}})
			f.sharing.On("GetPolicy", mock.Anything, sharePolicyID).
				Return(userPolicy(shareChildOU), (*tidcommon.ServiceError)(nil)).Once()

			_, err := f.service.GetSharingPolicy(context.Background(), svcTestUserID1, sharePolicyID)

			if tc.readable {
				require.Nil(t, err)
				return
			}
			require.NotNil(t, err)
			require.Equal(t, tidcommon.ErrorUnauthorized.Code, err.Code)
		})
	}
}

func TestSharingPolicy_OnlyTheInitiatorChangesAPolicy(t *testing.T) {
	cases := []struct {
		name       string
		allowedIn  string
		initiating string
		allowed    bool
	}{
		{name: "ChildChangesItsReshare", allowedIn: shareChildOU, initiating: shareChildOU, allowed: true},
		{name: "OwnerCannotChangeTheChildsReshare", allowedIn: shareOwnerOU, initiating: shareChildOU},
		{name: "ChildCannotChangeTheOwnersShare", allowedIn: shareChildOU, initiating: shareOwnerOU},
	}
	for _, tc := range cases {
		t.Run(tc.name+"/Update", func(t *testing.T) {
			f := newSharingFixture(t, map[security.Action][]string{security.ActionUpdateUser: {tc.allowedIn}})
			f.sharing.On("GetPolicy", mock.Anything, sharePolicyID).
				Return(userPolicy(tc.initiating), (*tidcommon.ServiceError)(nil)).Once()
			if tc.allowed {
				f.sharing.On("UpdatePolicy", mock.Anything, sharePolicyID, mock.Anything).
					Return(userPolicy(tc.initiating), (*tidcommon.ServiceError)(nil)).Once()
			}

			_, err := f.service.UpdateSharingPolicy(
				context.Background(), svcTestUserID1, sharePolicyID, sharing.PolicyRequest{Version: 1})

			if tc.allowed {
				require.Nil(t, err)
				return
			}
			require.NotNil(t, err)
			require.Equal(t, tidcommon.ErrorUnauthorized.Code, err.Code)
		})
		t.Run(tc.name+"/Delete", func(t *testing.T) {
			f := newSharingFixture(t, map[security.Action][]string{security.ActionUpdateUser: {tc.allowedIn}})
			f.sharing.On("GetPolicy", mock.Anything, sharePolicyID).
				Return(userPolicy(tc.initiating), (*tidcommon.ServiceError)(nil)).Once()
			if tc.allowed {
				f.sharing.On("DeletePolicy", mock.Anything, sharePolicyID).Return((*tidcommon.ServiceError)(nil)).Once()
			}

			err := f.service.DeleteSharingPolicy(context.Background(), svcTestUserID1, sharePolicyID)

			if tc.allowed {
				require.Nil(t, err)
				return
			}
			require.NotNil(t, err)
			require.Equal(t, tidcommon.ErrorUnauthorized.Code, err.Code)
		})
	}
}

func TestSharingPolicy_DeletingAnAbsentPolicySucceeds(t *testing.T) {
	f := newSharingFixture(t, nil)
	f.sharing.On("GetPolicy", mock.Anything, sharePolicyID).
		Return(sharing.Policy{}, &sharing.ErrorPolicyNotFound).Once()

	err := f.service.DeleteSharingPolicy(context.Background(), svcTestUserID1, sharePolicyID)

	require.Nil(t, err)
}

func TestSharingPolicy_DeleteRefusesADeclaredPolicy(t *testing.T) {
	f := newSharingFixture(t, map[security.Action][]string{security.ActionUpdateUser: {shareOwnerOU}})
	declared := userPolicy(shareOwnerOU)
	declared.Declared = true
	f.sharing.On("GetPolicy", mock.Anything, sharePolicyID).Return(declared, (*tidcommon.ServiceError)(nil)).Once()

	err := f.service.DeleteSharingPolicy(context.Background(), svcTestUserID1, sharePolicyID)

	require.NotNil(t, err)
	require.Equal(t, ErrorSharingPolicyDeclared.Code, err.Code)
}

func TestSharingPolicy_OwnerListsThroughTheFrameworkPage(t *testing.T) {
	f := newSharingFixture(t, map[security.Action][]string{security.ActionReadUser: {shareOwnerOU}})
	page := sharing.PolicyList{TotalResults: 1, StartIndex: 1, Count: 1, Policies: []sharing.Policy{
		userPolicy(shareOwnerOU),
	}}
	f.sharing.On("GetPolicyList", mock.Anything, UserSharingType, svcTestUserID1, 30, 0).
		Return(page, (*tidcommon.ServiceError)(nil)).Once()

	list, err := f.service.ListSharingPolicies(context.Background(), svcTestUserID1, 30, 0)

	require.Nil(t, err)
	require.Equal(t, page, list)
}

func TestSharingPolicy_OtherUnitsListOnlyTheirOwnPolicies(t *testing.T) {
	f := newSharingFixture(t, map[security.Action][]string{security.ActionReadUser: {shareChildOU, shareOtherOU}})
	f.sharing.On("ListPolicies", mock.Anything, UserSharingType, svcTestUserID1).Return([]sharing.Policy{
		userPolicy(shareOwnerOU), userPolicy(shareChildOU), userPolicy(shareOtherOU),
	}, (*tidcommon.ServiceError)(nil)).Once()

	list, err := f.service.ListSharingPolicies(context.Background(), svcTestUserID1, 1, 1)

	require.Nil(t, err)
	require.Equal(t, 2, list.TotalResults)
	require.Equal(t, 2, list.StartIndex)
	require.Equal(t, 1, list.Count)
	require.Equal(t, shareOtherOU, list.Policies[0].InitiatingOUID)
}

func TestSharingPolicy_InMemoryListingPastTheLastPageIsEmpty(t *testing.T) {
	f := newSharingFixture(t, map[security.Action][]string{security.ActionReadUser: {shareChildOU}})
	f.sharing.On("ListPolicies", mock.Anything, UserSharingType, svcTestUserID1).Return([]sharing.Policy{
		userPolicy(shareOwnerOU), userPolicy(shareChildOU),
	}, (*tidcommon.ServiceError)(nil)).Once()

	list, err := f.service.ListSharingPolicies(context.Background(), svcTestUserID1, 30, 5)

	require.Nil(t, err)
	require.Empty(t, list.Policies)
	require.Equal(t, 1, list.TotalResults)
	require.Equal(t, 6, list.StartIndex)
}

func TestSharingPolicy_AUnitWithNoPoliciesCannotList(t *testing.T) {
	f := newSharingFixture(t, map[security.Action][]string{security.ActionReadUser: {shareOtherOU}})
	f.sharing.On("ListPolicies", mock.Anything, UserSharingType, svcTestUserID1).Return([]sharing.Policy{
		userPolicy(shareOwnerOU),
	}, (*tidcommon.ServiceError)(nil)).Once()

	_, err := f.service.ListSharingPolicies(context.Background(), svcTestUserID1, 30, 0)

	require.NotNil(t, err)
	require.Equal(t, tidcommon.ErrorUnauthorized.Code, err.Code)
}

// Both listings check the page: the owner's, which the framework pages, and another unit's, paged in
// memory. The framework mock expects no call, so an unchecked page fails the test.
func TestSharingPolicy_ListingValidatesThePage(t *testing.T) {
	cases := []struct {
		name          string
		limit, offset int
		expected      string
	}{
		{name: "ZeroLimit", limit: 0, offset: 0, expected: ErrorInvalidLimit.Code},
		{name: "LimitAboveMaximum", limit: 1000000, offset: 0, expected: ErrorInvalidLimit.Code},
		{name: "NegativeOffset", limit: 10, offset: -1, expected: ErrorInvalidOffset.Code},
		{name: "OffsetThatWouldOverflow", limit: 10, offset: math.MaxInt, expected: ErrorInvalidOffset.Code},
	}
	callers := map[string]string{"Owner": shareOwnerOU, "OtherUnit": shareChildOU}
	for caller, ouID := range callers {
		for _, tc := range cases {
			t.Run(caller+"/"+tc.name, func(t *testing.T) {
				f := newSharingFixture(t, map[security.Action][]string{security.ActionReadUser: {ouID}})

				_, err := f.service.ListSharingPolicies(context.Background(), svcTestUserID1, tc.limit, tc.offset)

				require.NotNil(t, err)
				require.Equal(t, tc.expected, err.Code)
			})
		}
	}
}

func TestSharingPolicy_ResolveOverlay(t *testing.T) {
	visible := sharing.ResolvedOverlay{OUID: shareChildOU, Visible: true}
	cases := []struct {
		name      string
		allowedIn string
		ouID      string
		overlay   sharing.ResolvedOverlay
		svcErr    *tidcommon.ServiceError
		expected  string
	}{
		{name: "MissingOU", allowedIn: shareOwnerOU, expected: ErrorMissingOUIDParameter.Code},
		{name: "ChildAsksAboutItself", allowedIn: shareChildOU, ouID: shareChildOU, overlay: visible},
		{name: "OwnerAsksAboutAnUnreachedUnit", allowedIn: shareOwnerOU, ouID: shareOtherOU,
			overlay: sharing.ResolvedOverlay{OUID: shareOtherOU}, expected: ErrorUserNotSharedToOU.Code},
		{name: "OwnerAsksAboutAnUnknownUnit", allowedIn: shareOwnerOU, ouID: "ou-unknown",
			svcErr: &oupkg.ErrorOrganizationUnitNotFound, expected: ErrorUserNotSharedToOU.Code},
		{name: "UnrelatedUnitCannotAsk", allowedIn: shareOtherOU, ouID: shareChildOU,
			expected: tidcommon.ErrorUnauthorized.Code},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newSharingFixture(t, map[security.Action][]string{security.ActionReadUser: {tc.allowedIn}})
			if tc.ouID != "" && tc.expected != tidcommon.ErrorUnauthorized.Code {
				f.sharing.On("ResolveOverlayRules", mock.Anything, UserSharingType, svcTestUserID1, tc.ouID).
					Return(tc.overlay, tc.svcErr).Once()
			}

			overlay, err := f.service.ResolveSharingOverlay(context.Background(), svcTestUserID1, tc.ouID)

			if tc.expected == "" {
				require.Nil(t, err)
				require.True(t, overlay.Visible)
				return
			}
			require.NotNil(t, err)
			require.Equal(t, tc.expected, err.Code)
		})
	}
}

func TestTranslateSharingError(t *testing.T) {
	cases := []struct {
		name     string
		in       tidcommon.ServiceError
		expected string
	}{
		{name: "PolicyNotFound", in: sharing.ErrorPolicyNotFound, expected: ErrorSharingPolicyNotFound.Code},
		{name: "PolicyExists", in: sharing.ErrorPolicyExists, expected: ErrorSharingPolicyExists.Code},
		{name: "VersionMismatch", in: sharing.ErrorVersionMismatch, expected: ErrorSharingPolicyVersionMismatch.Code},
		{name: "PolicyDeclared", in: sharing.ErrorPolicyDeclared, expected: ErrorSharingPolicyDeclared.Code},
		{name: "InvalidLimit", in: sharing.ErrorInvalidLimit, expected: ErrorInvalidLimit.Code},
		{name: "InvalidOffset", in: sharing.ErrorInvalidOffset, expected: ErrorInvalidOffset.Code},
		{name: "AnyOtherRefusal", in: sharing.ErrorDeclaredPolicyIDConflict, expected: ErrorInvalidSharingPolicy.Code},
		{name: "ServerError", in: tidcommon.InternalServerError, expected: tidcommon.InternalServerError.Code},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := tc.in
			out := translateSharingError(&in)
			require.Equal(t, tc.expected, out.Code)
			if in.Type == tidcommon.ClientErrorType {
				require.Equal(t, in.ErrorDescription, out.ErrorDescription, "the framework's reason must survive")
			}
		})
	}
	require.Nil(t, translateSharingError(nil))
}

func TestUserService_DeleteUser_AbortedWhenSharingPurgeFails(t *testing.T) {
	userID := svcTestUserID1
	storeMock := newDeletableUserStore(t)
	sharingMock := sharingmock.NewSharingServiceInterfaceMock(t)
	sharingMock.On("DeletePoliciesForResource", mock.Anything, UserSharingType, userID).
		Return(&tidcommon.InternalServerError).Once()

	service := &userService{
		governance:         activeGovernance(t),
		entityService:      storeMock,
		authzService:       newAllowAllAuthz(t),
		dependencyRegistry: newNoBlockingDepsRegistry(),
		sharingService:     sharingMock,
	}

	err := service.DeleteUser(context.Background(), userID)

	require.NotNil(t, err)
	require.Equal(t, tidcommon.InternalServerError.Code, err.Code)
	storeMock.AssertNotCalled(t, "DeleteEntity", mock.Anything, mock.Anything)
}

// accessTree is the organization unit tree behind the accessible-units tests, laid out as the cloud
// case: users live in the Customers OU and are shared into the customers' OUs beneath it.
//
//	ou-r             (root, not accessible)
//	└── ou-p         (the user's own: the Customers OU)
//	    ├── ou-c     (shared: a customer's OU)
//	    │   └── ou-g (shared)
//	    └── ou-c2    (shared: the same customer's other OU)
//	ou-x             (another tree's root, shared)
//	ou-y             (another tree's root, shared)
var accessTree = map[string]*string{
	"ou-r":  nil,
	"ou-p":  strPtr("ou-r"),
	"ou-c":  strPtr("ou-p"),
	"ou-c2": strPtr("ou-p"),
	"ou-g":  strPtr("ou-c"),
	"ou-x":  nil,
	"ou-y":  nil,
}

func strPtr(s string) *string { return &s }

// newAccessFixture returns a user service whose user lives in ou-p and is shared into the given
// units, called as that user from the given organization unit.
func newAccessFixture(t *testing.T, tokenOU string, reached ...string) (*userService, context.Context) {
	store := entitymock.NewEntityServiceInterfaceMock(t)
	store.On("GetEntity", mock.Anything, svcTestUserID1).Return(&providers.Entity{
		Category: providers.EntityCategoryUser, ID: svcTestUserID1, OUID: "ou-p",
	}, nil).Maybe()

	sharingMock := sharingmock.NewSharingServiceInterfaceMock(t)
	sharingMock.On("ListResourceSharedOUIDs", mock.Anything, UserSharingType, svcTestUserID1).
		Return(reached, (*tidcommon.ServiceError)(nil)).Maybe()

	ouMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	ouMock.EXPECT().GetOrganizationUnit(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, id string) (oupkg.OrganizationUnit, *tidcommon.ServiceError) {
			parent, ok := accessTree[id]
			if !ok {
				return oupkg.OrganizationUnit{}, &oupkg.ErrorOrganizationUnitNotFound
			}
			return oupkg.OrganizationUnit{ID: id, Handle: id, Name: id, Parent: parent}, nil
		}).Maybe()

	service := &userService{
		governance:    activeGovernance(t),
		entityService: store, authzService: newAllowAllAuthz(t),
		sharingService: sharingMock, ouService: ouMock,
	}
	ctx := security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest(svcTestUserID1, tokenOU, "", nil, nil))
	return service, ctx
}

// unitIDs returns the ids of the listed units, in order.
func unitIDs(list *AccessibleOrganizationUnitListResponse) []string {
	ids := make([]string, 0, len(list.OrganizationUnits))
	for _, ou := range list.OrganizationUnits {
		ids = append(ids, ou.ID)
	}
	return ids
}

func TestGetAccessibleOrganizationUnits_Scopes(t *testing.T) {
	cases := []struct {
		name, tokenOU, scope string
		expected             []string
	}{
		{name: "EveryUnitWhenScopeIsOmitted", tokenOU: "ou-c", scope: "",
			expected: []string{"ou-p", "ou-c", "ou-c2", "ou-g", "ou-x", "ou-y"}},
		{name: "SiblingsOfACustomerUnitLeaveOutTheParent", tokenOU: "ou-c", scope: "siblings",
			expected: []string{"ou-c", "ou-c2"}},
		{name: "SiblingsOfTheOwnUnit", tokenOU: "ou-p", scope: "siblings", expected: []string{"ou-p"}},
		{name: "SiblingsOfATopLevelUnitAreTheOtherTopLevelUnits", tokenOU: "ou-x", scope: "siblings",
			expected: []string{"ou-x", "ou-y"}},
		{name: "AnOnlyChildIsItsOwnSibling", tokenOU: "ou-g", scope: "siblings", expected: []string{"ou-g"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, ctx := newAccessFixture(t, tc.tokenOU, "ou-c", "ou-c2", "ou-g", "ou-x", "ou-y")

			list, err := service.GetAccessibleOrganizationUnits(ctx, tc.scope, 30, 0)

			require.Nil(t, err)
			require.Equal(t, tc.tokenOU, list.Current)
			require.Equal(t, tc.expected, unitIDs(list))
			require.Equal(t, len(tc.expected), list.TotalResults)
		})
	}
}

func TestGetAccessibleOrganizationUnits_OwnUnitFirstThenTheReachByID(t *testing.T) {
	service, ctx := newAccessFixture(t, "ou-p", "ou-x", "ou-g", "ou-c")

	list, err := service.GetAccessibleOrganizationUnits(ctx, "", 30, 0)

	require.Nil(t, err)
	require.Equal(t, []string{"ou-p", "ou-c", "ou-g", "ou-x"}, unitIDs(list))
}

func TestGetAccessibleOrganizationUnits_CurrentFallsBackToTheUsersOwnUnit(t *testing.T) {
	service, ctx := newAccessFixture(t, "", "ou-c")

	list, err := service.GetAccessibleOrganizationUnits(ctx, "siblings", 30, 0)

	require.Nil(t, err)
	require.Equal(t, "ou-p", list.Current)
	require.Equal(t, []string{"ou-p"}, unitIDs(list))
}

func TestGetAccessibleOrganizationUnits_NoSiblingsWhenTheCurrentUnitIsGone(t *testing.T) {
	service, ctx := newAccessFixture(t, "ou-removed", "ou-c")

	list, err := service.GetAccessibleOrganizationUnits(ctx, "siblings", 30, 0)

	require.Nil(t, err)
	require.Empty(t, list.OrganizationUnits)
	require.Equal(t, 0, list.TotalResults)
}

func TestGetAccessibleOrganizationUnits_SkipsARemovedUnit(t *testing.T) {
	service, ctx := newAccessFixture(t, "ou-p", "ou-c", "ou-removed")

	list, err := service.GetAccessibleOrganizationUnits(ctx, "", 30, 0)

	require.Nil(t, err)
	require.Equal(t, []string{"ou-p", "ou-c"}, unitIDs(list))
}

func TestGetAccessibleOrganizationUnits_Pages(t *testing.T) {
	service, ctx := newAccessFixture(t, "ou-p", "ou-c", "ou-g", "ou-x")

	list, err := service.GetAccessibleOrganizationUnits(ctx, "", 2, 1)

	require.Nil(t, err)
	require.Equal(t, []string{"ou-c", "ou-g"}, unitIDs(list))
	require.Equal(t, 4, list.TotalResults)
	require.Equal(t, 2, list.StartIndex)
	require.Equal(t, 2, list.Count)
	rels := make([]string, 0, len(list.Links))
	for _, link := range list.Links {
		rels = append(rels, link.Rel)
	}
	require.Contains(t, rels, "next")
	require.Contains(t, rels, "prev")
}

func TestGetAccessibleOrganizationUnits_PastTheLastPageIsEmpty(t *testing.T) {
	service, ctx := newAccessFixture(t, "ou-p", "ou-c")

	list, err := service.GetAccessibleOrganizationUnits(ctx, "", 30, 5)

	require.Nil(t, err)
	require.Empty(t, list.OrganizationUnits)
	require.Equal(t, 2, list.TotalResults)
	require.Equal(t, 6, list.StartIndex)
}

func TestGetAccessibleOrganizationUnits_PageLinksKeepTheScope(t *testing.T) {
	service, ctx := newAccessFixture(t, "ou-c", "ou-c", "ou-c2")

	list, err := service.GetAccessibleOrganizationUnits(ctx, "siblings", 1, 0)

	require.Nil(t, err)
	require.Equal(t, []string{"ou-c"}, unitIDs(list))
	require.Equal(t, 2, list.TotalResults)
	var next string
	for _, link := range list.Links {
		if link.Rel == "next" {
			next = link.Href
		}
	}
	require.Contains(t, next, "/users/me/ous?")
	require.Contains(t, next, "scope=siblings")
}

func TestGetAccessibleOrganizationUnits_RefusesACallerWithNoSubject(t *testing.T) {
	service, _ := newAccessFixture(t, "ou-p")
	ctx := security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest("", "ou-p", "", nil, nil))

	_, err := service.GetAccessibleOrganizationUnits(ctx, "", 30, 0)

	require.NotNil(t, err)
	require.Equal(t, ErrorAuthenticationFailed.Code, err.Code)
}

func TestGetAccessibleOrganizationUnits_RefusesABadRequest(t *testing.T) {
	cases := []struct {
		name          string
		scope         string
		limit, offset int
		expected      string
	}{
		{name: "DroppedScopeChildren", scope: "children", limit: 30, expected: ErrorInvalidScope.Code},
		{name: "DroppedScopeAll", scope: "all", limit: 30, expected: ErrorInvalidScope.Code},
		{name: "ZeroLimit", limit: 0, expected: ErrorInvalidLimit.Code},
		{name: "NegativeOffset", limit: 30, offset: -1, expected: ErrorInvalidOffset.Code},
		{name: "OffsetThatWouldOverflow", limit: 30, offset: math.MaxInt, expected: ErrorInvalidOffset.Code},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, ctx := newAccessFixture(t, "ou-p")

			_, err := service.GetAccessibleOrganizationUnits(ctx, tc.scope, tc.limit, tc.offset)

			require.NotNil(t, err)
			require.Equal(t, tc.expected, err.Code)
		})
	}
}

// lockedRead returns an access read reporting LOCKED, without details.
func lockedRead() identitygovernance.AccessRead {
	return identitygovernance.AccessRead{
		Status:      providers.AccessStatusReport{Value: providers.AccessStatusLocked},
		LastLoginAt: "2026-09-09T18:22:04Z",
	}
}

// activeGovernance answers every governance read with ACTIVE.
func activeGovernance(t *testing.T) *identitygovernancemock.IdentityGovernanceServiceInterfaceMock {
	governance := identitygovernancemock.NewIdentityGovernanceServiceInterfaceMock(t)
	governance.EXPECT().ReadEntityAccess(mock.Anything, mock.Anything).
		Return(identitygovernance.AccessRead{
			Status: providers.AccessStatusReport{Value: providers.AccessStatusActive},
		}).Maybe()
	governance.EXPECT().ReadAccess(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, entities []providers.Entity) []identitygovernance.AccessRead {
			reads := make([]identitygovernance.AccessRead, len(entities))
			for i := range reads {
				reads[i].Status.Value = providers.AccessStatusActive
			}
			return reads
		}).Maybe()
	return governance
}

func TestUserService_GetUser_ReportsTheGovernedStatus(t *testing.T) {
	stored := &providers.Entity{Category: providers.EntityCategoryUser, ID: svcTestUserID1, OUID: testOrgID}
	storeMock := entitymock.NewEntityServiceInterfaceMock(t)
	storeMock.On("GetEntity", mock.Anything, svcTestUserID1).Return(stored, nil).Once()
	governance := identitygovernancemock.NewIdentityGovernanceServiceInterfaceMock(t)
	governance.EXPECT().ReadEntityAccess(mock.Anything, *stored).
		Return(lockedRead()).Once()
	service := &userService{entityService: storeMock, authzService: newAllowAllAuthz(t), governance: governance}

	user, svcErr := service.GetUser(context.Background(), svcTestUserID1, false)

	require.Nil(t, svcErr)
	require.NotNil(t, user.Status)
	require.Equal(t, providers.AccessStatusLocked, user.Status.Value)
	require.Equal(t, "2026-09-09T18:22:04Z", user.LastLoginAt)
}

func TestUserService_GetUser_RefusedReadDerivesNoStatus(t *testing.T) {
	storeMock := entitymock.NewEntityServiceInterfaceMock(t)
	storeMock.On("GetEntity", mock.Anything, svcTestUserID1).Return(&providers.Entity{
		Category: providers.EntityCategoryUser, ID: svcTestUserID1, OUID: testOrgID}, nil).Once()
	authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
	authzMock.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
		Return(false, (*tidcommon.ServiceError)(nil)).Once()
	governance := identitygovernancemock.NewIdentityGovernanceServiceInterfaceMock(t)
	service := &userService{entityService: storeMock, authzService: authzMock, governance: governance}

	user, svcErr := service.GetUser(context.Background(), svcTestUserID1, false)

	require.Nil(t, user)
	require.NotNil(t, svcErr)
	governance.AssertNotCalled(t, "ReadAccess", mock.Anything, mock.Anything)
	governance.AssertNotCalled(t, "ReadEntityAccess", mock.Anything, mock.Anything)
}

func TestUserService_UpdateUserAttributes_ReportsTheGovernedStatus(t *testing.T) {
	stored := &providers.Entity{Category: providers.EntityCategoryUser, ID: svcTestUserID1, Type: testUserType,
		Attributes: json.RawMessage(`{"email":"old@example.com"}`)}
	storeMock := entitymock.NewEntityServiceInterfaceMock(t)
	storeMock.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
	storeMock.On("GetEntity", mock.Anything, svcTestUserID1).Return(stored, nil).Once()
	storeMock.On("UpdateAttributes", mock.Anything, svcTestUserID1, mock.Anything).Return(nil).Once()
	schemaMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	schemaMock.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		entitytype.AttributeFilter{AllowCredential: true}).
		Return([]entitytype.AttributeInfo{}, (*tidcommon.ServiceError)(nil)).Once()
	governance := identitygovernancemock.NewIdentityGovernanceServiceInterfaceMock(t)
	governance.EXPECT().ReadEntityAccess(mock.Anything, *stored).
		Return(lockedRead()).Once()
	service := &userService{entityService: storeMock, entityTypeService: schemaMock,
		authzService: newAllowAllAuthz(t), governance: governance}

	user, svcErr := service.UpdateUserAttributes(context.Background(), svcTestUserID1,
		json.RawMessage(`{"email":"new@example.com"}`))

	require.Nil(t, svcErr)
	require.NotNil(t, user.Status)
	require.Equal(t, providers.AccessStatusLocked, user.Status.Value)
	require.Equal(t, "2026-09-09T18:22:04Z", user.LastLoginAt)
}

func TestUserService_GetUserList_ReportsTheGovernedStatus(t *testing.T) {
	listed := []providers.Entity{
		{ID: svcTestUserID1, OUID: testOrgID, Category: providers.EntityCategoryUser},
		{ID: svcTestUserID123, OUID: testOrgID, Category: providers.EntityCategoryUser},
	}
	reads := []identitygovernance.AccessRead{
		lockedRead(), {Status: providers.AccessStatusReport{Value: providers.AccessStatusSuspended}},
	}
	filters := map[string]interface{}{}
	type listSetup func(storeMock *entitymock.EntityServiceInterfaceMock) sysauthz.SystemAuthorizationServiceInterface
	cases := []struct {
		name  string
		setup listSetup
	}{
		{name: "all users", setup: func(
			storeMock *entitymock.EntityServiceInterfaceMock) sysauthz.SystemAuthorizationServiceInterface {
			storeMock.On("GetEntityListCount", mock.Anything, providers.EntityCategoryUser, filters).
				Return(2, nil).Once()
			storeMock.On("GetEntityList", mock.Anything, providers.EntityCategoryUser, 10, 0, filters).
				Return(listed, nil).Once()
			return newAllowAllAuthz(t)
		}},
		{name: "users in accessible units", setup: func(
			storeMock *entitymock.EntityServiceInterfaceMock) sysauthz.SystemAuthorizationServiceInterface {
			ouIDs := []string{testOrgID}
			storeMock.On("GetEntityListCountByOUIDs", mock.Anything, providers.EntityCategoryUser, ouIDs, filters).
				Return(2, nil).Once()
			storeMock.On("GetEntityListByOUIDs", mock.Anything, providers.EntityCategoryUser, ouIDs, 10, 0,
				filters).Return(listed, nil).Once()
			authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
			authzMock.On("GetAccessibleResources", mock.Anything, mock.Anything, mock.Anything).
				Return(&sysauthz.AccessibleResources{IDs: ouIDs}, nil).Once()
			return authzMock
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			storeMock := entitymock.NewEntityServiceInterfaceMock(t)
			authz := tc.setup(storeMock)
			governance := identitygovernancemock.NewIdentityGovernanceServiceInterfaceMock(t)
			governance.EXPECT().ReadAccess(mock.Anything, listed).Return(reads).Once()
			service := &userService{entityService: storeMock, authzService: authz, governance: governance}

			resp, svcErr := service.GetUserList(context.Background(), 10, 0, filters, false)

			require.Nil(t, svcErr)
			require.Len(t, resp.Users, 2)
			require.Equal(t, svcTestUserID1, resp.Users[0].ID)
			require.Equal(t, providers.AccessStatusLocked, resp.Users[0].Status.Value)
			require.Equal(t, "2026-09-09T18:22:04Z", resp.Users[0].LastLoginAt)
			require.Equal(t, svcTestUserID123, resp.Users[1].ID)
			require.Equal(t, providers.AccessStatusSuspended, resp.Users[1].Status.Value)
		})
	}
}
