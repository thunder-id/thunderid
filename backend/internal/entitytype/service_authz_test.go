/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package entitytype

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/tests/mocks/consentmock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

// ---------------------------------------------------------------------------
// Helper: create a deny-all authz mock
// ---------------------------------------------------------------------------

func newAuthzError(t interface {
	mock.TestingT
	Cleanup(func())
}) *sysauthzmock.SystemAuthorizationServiceInterfaceMock {
	svcErr := &serviceerror.ServiceError{
		Type: serviceerror.ServerErrorType,
		Code: "SSE-5000",
		Error: core.I18nMessage{
			Key:          "error.sysauthz.authorization_failure",
			DefaultValue: "authz failure",
		},
	}
	m := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
	m.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
		Return(false, svcErr).Maybe()
	m.On("GetAccessibleResources", mock.Anything, mock.Anything, mock.Anything).
		Return((*sysauthz.AccessibleResources)(nil), svcErr).Maybe()
	return m
}

func initTestRuntime(t *testing.T) {
	t.Helper()
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{Enabled: false},
	}
	config.ResetServerRuntime()
	require.NoError(t, config.InitializeServerRuntime("/tmp/test", testConfig))
	t.Cleanup(config.ResetServerRuntime)
}

// ---------------------------------------------------------------------------
// Suite for authorization tests
// ---------------------------------------------------------------------------

type AuthzTestSuite struct {
	suite.Suite
}

func TestAuthzTestSuite(t *testing.T) {
	suite.Run(t, new(AuthzTestSuite))
}

// ---- GetEntityTypeList ----

func (s *AuthzTestSuite) TestGetEntityTypeList_AllAllowed() {
	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	storeMock.On("GetEntityTypeListCount", mock.Anything, mock.Anything).Return(2, nil)
	storeMock.On("GetEntityTypeList", mock.Anything, mock.Anything, 10, 0).Return([]EntityTypeListItem{
		{ID: "s1", Name: "schema1", OUID: testOUID1},
		{ID: "s2", Name: "schema2", OUID: testOUID2},
	}, nil)

	svc := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
		authzService:    newAllowAllAuthz(s.T()),
	}

	resp, svcErr := svc.GetEntityTypeList(context.Background(), TypeCategoryUser, 10, 0, false)
	s.Require().Nil(svcErr)
	s.Require().NotNil(resp)
	s.Equal(2, resp.TotalResults)
	s.Len(resp.Types, 2)
}

func (s *AuthzTestSuite) TestGetEntityTypeList_FilteredByOUIDs() {
	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	storeMock.On("GetEntityTypeListCountByOUIDs", mock.Anything, mock.Anything, []string{testOUID1}).Return(1, nil)
	storeMock.On("GetEntityTypeListByOUIDs", mock.Anything, mock.Anything, []string{testOUID1}, 10, 0).
		Return([]EntityTypeListItem{
			{ID: "s1", Name: "schema1", OUID: testOUID1},
		}, nil)

	authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	authzMock.On("GetAccessibleResources", mock.Anything, security.ActionListUserTypes,
		security.ResourceTypeUserType).
		Return(&sysauthz.AccessibleResources{AllAllowed: false, IDs: []string{testOUID1}}, nil)

	svc := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
		authzService:    authzMock,
	}

	resp, svcErr := svc.GetEntityTypeList(context.Background(), TypeCategoryUser, 10, 0, false)
	s.Require().Nil(svcErr)
	s.Require().NotNil(resp)
	s.Equal(1, resp.TotalResults)
	s.Len(resp.Types, 1)
	s.Equal("s1", resp.Types[0].ID)
}

func (s *AuthzTestSuite) TestGetEntityTypeList_EmptyAccessibleOUIDs() {
	authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	authzMock.On("GetAccessibleResources", mock.Anything, security.ActionListUserTypes,
		security.ResourceTypeUserType).
		Return(&sysauthz.AccessibleResources{AllAllowed: false, IDs: []string{}}, nil)

	svc := &entityTypeService{
		entityTypeStore: newEntityTypeStoreInterfaceMock(s.T()),
		transactioner:   &mockTransactioner{},
		authzService:    authzMock,
	}

	resp, svcErr := svc.GetEntityTypeList(context.Background(), TypeCategoryUser, 10, 0, false)
	s.Require().Nil(svcErr)
	s.Require().NotNil(resp)
	s.Equal(0, resp.TotalResults)
	s.Empty(resp.Types)
}

func (s *AuthzTestSuite) TestGetEntityTypeList_AuthzServiceError() {
	svc := &entityTypeService{
		entityTypeStore: newEntityTypeStoreInterfaceMock(s.T()),
		transactioner:   &mockTransactioner{},
		authzService:    newAuthzError(s.T()),
	}

	resp, svcErr := svc.GetEntityTypeList(context.Background(), TypeCategoryUser, 10, 0, false)
	s.Nil(resp)
	s.Require().NotNil(svcErr)
	s.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
}

func (s *AuthzTestSuite) TestGetEntityTypeList_NilAuthzService() {
	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	storeMock.On("GetEntityTypeListCount", mock.Anything, mock.Anything).Return(1, nil)
	storeMock.On("GetEntityTypeList", mock.Anything, mock.Anything, 10, 0).Return([]EntityTypeListItem{
		{ID: "s1", Name: "schema1", OUID: testOUID1},
	}, nil)

	svc := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
		authzService:    nil,
	}

	resp, svcErr := svc.GetEntityTypeList(context.Background(), TypeCategoryUser, 10, 0, false)
	s.Require().Nil(svcErr)
	s.Require().NotNil(resp)
	s.Equal(1, resp.TotalResults)
}

// ---- CreateEntityType ----

func (s *AuthzTestSuite) TestCreateEntityType_Denied() {
	initTestRuntime(s.T())

	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	ouMock := oumock.NewOrganizationUnitServiceInterfaceMock(s.T())
	ouMock.On("IsOrganizationUnitExists", mock.Anything, testOUID1).
		Return(true, (*serviceerror.ServiceError)(nil))

	authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	authzMock.On("IsActionAllowed", mock.Anything, security.ActionCreateUserType,
		&sysauthz.ActionContext{ResourceType: security.ResourceTypeUserType, OUID: testOUID1}).
		Return(false, nil)

	svc := &entityTypeService{
		entityTypeStore: storeMock,
		ouService:       ouMock,
		transactioner:   &mockTransactioner{},
		authzService:    authzMock,
	}

	result, svcErr := svc.CreateEntityType(context.Background(), TypeCategoryUser, CreateEntityTypeRequestWithID{
		Name:   "test-schema",
		OUID:   testOUID1,
		Schema: json.RawMessage(`{"email":{"type":"string"}}`),
	})
	s.Nil(result)
	s.Require().NotNil(svcErr)
	s.Equal(serviceerror.ErrorUnauthorized.Code, svcErr.Code)
}

func (s *AuthzTestSuite) TestCreateEntityType_AuthzError() {
	initTestRuntime(s.T())

	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	ouMock := oumock.NewOrganizationUnitServiceInterfaceMock(s.T())
	ouMock.On("IsOrganizationUnitExists", mock.Anything, testOUID1).
		Return(true, (*serviceerror.ServiceError)(nil))

	svc := &entityTypeService{
		entityTypeStore: storeMock,
		ouService:       ouMock,
		transactioner:   &mockTransactioner{},
		authzService:    newAuthzError(s.T()),
	}

	result, svcErr := svc.CreateEntityType(context.Background(), TypeCategoryUser, CreateEntityTypeRequestWithID{
		Name:   "test-schema",
		OUID:   testOUID1,
		Schema: json.RawMessage(`{"email":{"type":"string"}}`),
	})
	s.Nil(result)
	s.Require().NotNil(svcErr)
	s.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
}

// ---- GetEntityType ----

func (s *AuthzTestSuite) TestGetEntityType_Denied() {
	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	storeMock.On("GetEntityTypeByID", mock.Anything, mock.Anything, "schema-1").
		Return(EntityType{ID: "schema-1", OUID: testOUID1}, nil)

	authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	authzMock.On("IsActionAllowed", mock.Anything, security.ActionReadUserType,
		&sysauthz.ActionContext{ResourceType: security.ResourceTypeUserType, OUID: testOUID1}).
		Return(false, nil)

	svc := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
		authzService:    authzMock,
	}

	result, svcErr := svc.GetEntityType(context.Background(), TypeCategoryUser, "schema-1", false)
	s.Nil(result)
	s.Require().NotNil(svcErr)
	s.Equal(serviceerror.ErrorUnauthorized.Code, svcErr.Code)
}

func (s *AuthzTestSuite) TestGetEntityType_AuthzError() {
	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	storeMock.On("GetEntityTypeByID", mock.Anything, mock.Anything, "schema-1").
		Return(EntityType{ID: "schema-1", OUID: testOUID1}, nil)

	svc := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
		authzService:    newAuthzError(s.T()),
	}

	result, svcErr := svc.GetEntityType(context.Background(), TypeCategoryUser, "schema-1", false)
	s.Nil(result)
	s.Require().NotNil(svcErr)
	s.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
}

// ---- GetEntityTypeByName ----

func (s *AuthzTestSuite) TestGetEntityTypeByName_Denied() {
	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	storeMock.On("GetEntityTypeByName", mock.Anything, mock.Anything, "employee").
		Return(EntityType{ID: "schema-1", Name: "employee", OUID: testOUID2}, nil)

	authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	authzMock.On("IsActionAllowed", mock.Anything, security.ActionReadUserType,
		&sysauthz.ActionContext{ResourceType: security.ResourceTypeUserType, OUID: testOUID2}).
		Return(false, nil)

	svc := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
		authzService:    authzMock,
	}

	result, svcErr := svc.GetEntityTypeByName(context.Background(), TypeCategoryUser, "employee")
	s.Nil(result)
	s.Require().NotNil(svcErr)
	s.Equal(serviceerror.ErrorUnauthorized.Code, svcErr.Code)
}

func (s *AuthzTestSuite) TestGetEntityTypeByName_AuthzError() {
	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	storeMock.On("GetEntityTypeByName", mock.Anything, mock.Anything, "employee").
		Return(EntityType{ID: "schema-1", Name: "employee", OUID: testOUID2}, nil)

	svc := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
		authzService:    newAuthzError(s.T()),
	}

	result, svcErr := svc.GetEntityTypeByName(context.Background(), TypeCategoryUser, "employee")
	s.Nil(result)
	s.Require().NotNil(svcErr)
	s.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
}

// ---- UpdateEntityType ----

func (s *AuthzTestSuite) TestUpdateEntityType_Denied() {
	initTestRuntime(s.T())

	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	storeMock.On("IsEntityTypeDeclarative", mock.Anything, mock.Anything).Return(false).Maybe()
	storeMock.On("GetEntityTypeByID", mock.Anything, mock.Anything, "schema-1").
		Return(EntityType{
			ID:     "schema-1",
			Name:   "employee",
			OUID:   testOUID1,
			Schema: json.RawMessage(`{"email":{"type":"string"}}`),
		}, nil)

	ouMock := oumock.NewOrganizationUnitServiceInterfaceMock(s.T())
	ouMock.On("IsOrganizationUnitExists", mock.Anything, testOUID1).
		Return(true, (*serviceerror.ServiceError)(nil))

	authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	authzMock.On("IsActionAllowed", mock.Anything, security.ActionUpdateUserType,
		&sysauthz.ActionContext{ResourceType: security.ResourceTypeUserType, OUID: testOUID1}).
		Return(false, nil)

	svc := &entityTypeService{
		entityTypeStore: storeMock,
		ouService:       ouMock,
		transactioner:   &mockTransactioner{},
		authzService:    authzMock,
	}

	result, svcErr := svc.UpdateEntityType(context.Background(), TypeCategoryUser, "schema-1", UpdateEntityTypeRequest{
		Name:   "employee",
		OUID:   testOUID1,
		Schema: json.RawMessage(`{"email":{"type":"string"}}`),
	})
	s.Nil(result)
	s.Require().NotNil(svcErr)
	s.Equal(serviceerror.ErrorUnauthorized.Code, svcErr.Code)
}

func (s *AuthzTestSuite) TestUpdateEntityType_AuthzError() {
	initTestRuntime(s.T())

	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	storeMock.On("IsEntityTypeDeclarative", mock.Anything, mock.Anything).Return(false).Maybe()
	storeMock.On("GetEntityTypeByID", mock.Anything, mock.Anything, "schema-1").
		Return(EntityType{
			ID:     "schema-1",
			Name:   "employee",
			OUID:   testOUID1,
			Schema: json.RawMessage(`{"email":{"type":"string"}}`),
		}, nil)

	ouMock := oumock.NewOrganizationUnitServiceInterfaceMock(s.T())
	ouMock.On("IsOrganizationUnitExists", mock.Anything, testOUID1).
		Return(true, (*serviceerror.ServiceError)(nil))

	svc := &entityTypeService{
		entityTypeStore: storeMock,
		ouService:       ouMock,
		transactioner:   &mockTransactioner{},
		authzService:    newAuthzError(s.T()),
	}

	result, svcErr := svc.UpdateEntityType(context.Background(), TypeCategoryUser, "schema-1", UpdateEntityTypeRequest{
		Name:   "employee",
		OUID:   testOUID1,
		Schema: json.RawMessage(`{"email":{"type":"string"}}`),
	})
	s.Nil(result)
	s.Require().NotNil(svcErr)
	s.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
}

// ---- DeleteEntityType ----

func (s *AuthzTestSuite) TestDeleteEntityType_Denied() {
	initTestRuntime(s.T())

	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	storeMock.On("IsEntityTypeDeclarative", mock.Anything, mock.Anything).Return(false).Maybe()
	storeMock.On("GetEntityTypeByID", mock.Anything, mock.Anything, "schema-1").
		Return(EntityType{
			ID:   "schema-1",
			OUID: testOUID1,
		}, nil)

	authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	authzMock.On("IsActionAllowed", mock.Anything, security.ActionDeleteUserType,
		&sysauthz.ActionContext{ResourceType: security.ResourceTypeUserType, OUID: testOUID1}).
		Return(false, nil)

	svc := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
		authzService:    authzMock,
	}

	svcErr := svc.DeleteEntityType(context.Background(), TypeCategoryUser, "schema-1")
	s.Require().NotNil(svcErr)
	s.Equal(serviceerror.ErrorUnauthorized.Code, svcErr.Code)
}

func (s *AuthzTestSuite) TestDeleteEntityType_AuthzError() {
	initTestRuntime(s.T())

	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	storeMock.On("IsEntityTypeDeclarative", mock.Anything, mock.Anything).Return(false).Maybe()
	storeMock.On("GetEntityTypeByID", mock.Anything, mock.Anything, "schema-1").
		Return(EntityType{
			ID:   "schema-1",
			OUID: testOUID1,
		}, nil)

	svc := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
		authzService:    newAuthzError(s.T()),
	}

	svcErr := svc.DeleteEntityType(context.Background(), TypeCategoryUser, "schema-1")
	s.Require().NotNil(svcErr)
	s.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
}

func (s *AuthzTestSuite) TestDeleteEntityType_NotFound_StillChecksAuthz() {
	initTestRuntime(s.T())

	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	storeMock.On("IsEntityTypeDeclarative", mock.Anything, mock.Anything).Return(false).Maybe()
	storeMock.On("GetEntityTypeByID", mock.Anything, mock.Anything, "nonexistent").
		Return(EntityType{}, ErrEntityTypeNotFound)

	authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	// Expect delete authz check with empty OU (schema doesn't exist, so no OU to check against).
	authzMock.On("IsActionAllowed", mock.Anything, security.ActionDeleteUserType,
		&sysauthz.ActionContext{ResourceType: security.ResourceTypeUserType, OUID: ""}).
		Return(false, nil)

	svc := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
		authzService:    authzMock,
	}

	svcErr := svc.DeleteEntityType(context.Background(), TypeCategoryUser, "nonexistent")
	s.Require().NotNil(svcErr)
	s.Equal(serviceerror.ErrorUnauthorized.Code, svcErr.Code,
		"delete of nonexistent schema should still return unauthorized for denied callers")
}

func (s *AuthzTestSuite) TestDeleteEntityType_NotFound_Authorized_ReturnsNil() {
	initTestRuntime(s.T())

	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	storeMock.On("IsEntityTypeDeclarative", mock.Anything, mock.Anything).Return(false).Maybe()
	storeMock.On("GetEntityTypeByID", mock.Anything, mock.Anything, "nonexistent").
		Return(EntityType{}, ErrEntityTypeNotFound)

	authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	authzMock.On("IsActionAllowed", mock.Anything, security.ActionDeleteUserType,
		&sysauthz.ActionContext{ResourceType: security.ResourceTypeUserType, OUID: ""}).
		Return(true, nil)

	svc := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
		authzService:    authzMock,
	}

	svcErr := svc.DeleteEntityType(context.Background(), TypeCategoryUser, "nonexistent")
	s.Nil(svcErr, "authorized caller deleting nonexistent schema should get nil (idempotent)")
}

// ---- Nil authzService (backward compatibility) ----

func (s *AuthzTestSuite) TestGetEntityType_NilAuthz_NoError() {
	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	storeMock.On("GetEntityTypeByID", mock.Anything, mock.Anything, "schema-1").
		Return(EntityType{ID: "schema-1", Name: "test", OUID: testOUID1}, nil)

	svc := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
		authzService:    nil,
	}

	result, svcErr := svc.GetEntityType(context.Background(), TypeCategoryUser, "schema-1", false)
	s.Require().Nil(svcErr)
	s.Require().NotNil(result)
	s.Equal("schema-1", result.ID)
}

func (s *AuthzTestSuite) TestGetEntityType_WithIncludeDisplay() {
	s.Run("populates OUHandle when includeDisplay is true", func() {
		storeMock := newEntityTypeStoreInterfaceMock(s.T())
		storeMock.On("GetEntityTypeByID", mock.Anything, mock.Anything, "schema-1").
			Return(EntityType{ID: "schema-1", Name: "test", OUID: testOUID1}, nil)

		ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(s.T())
		ouServiceMock.On(
			"GetOrganizationUnitHandlesByIDs",
			mock.Anything, []string{testOUID1},
		).Return(map[string]string{testOUID1: "root"}, nil).Once()

		svc := &entityTypeService{
			entityTypeStore: storeMock,
			transactioner:   &mockTransactioner{},
			authzService:    nil,
			ouService:       ouServiceMock,
		}

		result, svcErr := svc.GetEntityType(
			context.Background(), TypeCategoryUser, "schema-1", true)
		s.Require().Nil(svcErr)
		s.Require().NotNil(result)
		s.Equal("root", result.OUHandle)
		ouServiceMock.AssertExpectations(s.T())
	})

	s.Run("does not populate OUHandle when includeDisplay is false", func() {
		storeMock := newEntityTypeStoreInterfaceMock(s.T())
		storeMock.On("GetEntityTypeByID", mock.Anything, mock.Anything, "schema-1").
			Return(EntityType{ID: "schema-1", Name: "test", OUID: testOUID1}, nil)

		svc := &entityTypeService{
			entityTypeStore: storeMock,
			transactioner:   &mockTransactioner{},
			authzService:    nil,
		}

		result, svcErr := svc.GetEntityType(
			context.Background(), TypeCategoryUser, "schema-1", false)
		s.Require().Nil(svcErr)
		s.Require().NotNil(result)
		s.Equal("", result.OUHandle)
	})

	s.Run("returns schema with empty ouHandle when OU handle resolution fails", func() {
		storeMock := newEntityTypeStoreInterfaceMock(s.T())
		storeMock.On("GetEntityTypeByID", mock.Anything, mock.Anything, "schema-1").
			Return(EntityType{ID: "schema-1", Name: "test", OUID: testOUID1}, nil)

		ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(s.T())
		ouServiceMock.On(
			"GetOrganizationUnitHandlesByIDs",
			mock.Anything, []string{testOUID1},
		).Return(
			(map[string]string)(nil),
			&serviceerror.ServiceError{Code: "OU-5000"},
		).Once()

		svc := &entityTypeService{
			entityTypeStore: storeMock,
			transactioner:   &mockTransactioner{},
			authzService:    nil,
			ouService:       ouServiceMock,
		}

		result, svcErr := svc.GetEntityType(
			context.Background(), TypeCategoryUser, "schema-1", true)
		s.Require().Nil(svcErr)
		s.Require().NotNil(result)
		s.Equal("schema-1", result.ID)
		s.Empty(result.OUHandle)
	})
}

func (s *AuthzTestSuite) TestGetEntityTypeList_WithIncludeDisplay() {
	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	storeMock.On("GetEntityTypeListCount", mock.Anything, mock.Anything).Return(2, nil)
	storeMock.On("GetEntityTypeList", mock.Anything, mock.Anything, 10, 0).Return(
		[]EntityTypeListItem{
			{ID: "s1", Name: "schema1", OUID: testOUID1},
			{ID: "s2", Name: "schema2", OUID: testOUID2},
		}, nil)

	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(s.T())
	ouServiceMock.On(
		"GetOrganizationUnitHandlesByIDs",
		mock.Anything,
		mock.MatchedBy(func(ids []string) bool {
			if len(ids) != 2 {
				return false
			}
			expected := map[string]bool{testOUID1: true, testOUID2: true}
			return expected[ids[0]] && expected[ids[1]]
		}),
	).Return(map[string]string{
		testOUID1: "handle-1",
		testOUID2: "handle-2",
	}, nil).Once()

	svc := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
		authzService:    newAllowAllAuthz(s.T()),
		ouService:       ouServiceMock,
	}

	resp, svcErr := svc.GetEntityTypeList(
		context.Background(), TypeCategoryUser, 10, 0, true)
	s.Require().Nil(svcErr)
	s.Require().NotNil(resp)
	s.Require().Len(resp.Types, 2)
	s.Equal("handle-1", resp.Types[0].OUHandle)
	s.Equal("handle-2", resp.Types[1].OUHandle)
	ouServiceMock.AssertExpectations(s.T())
}

func (s *AuthzTestSuite) TestGetEntityTypeByName_NilAuthz_NoError() {
	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	storeMock.On("GetEntityTypeByName", mock.Anything, mock.Anything, "employee").
		Return(EntityType{ID: "schema-1", Name: "employee"}, nil)

	svc := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
		authzService:    nil,
	}

	result, svcErr := svc.GetEntityTypeByName(context.Background(), TypeCategoryUser, "employee")
	s.Require().Nil(svcErr)
	s.Require().NotNil(result)
}

func (s *AuthzTestSuite) TestDeleteEntityType_NilAuthz_NoError() {
	initTestRuntime(s.T())

	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	storeMock.On("IsEntityTypeDeclarative", mock.Anything, mock.Anything).Return(false).Maybe()
	storeMock.On("GetEntityTypeByID", mock.Anything, mock.Anything, "schema-1").
		Return(EntityType{ID: "schema-1", OUID: testOUID1}, nil)
	storeMock.On("DeleteEntityTypeByID", mock.Anything, mock.Anything, "schema-1").Return(nil)

	consentMock := consentmock.NewConsentServiceInterfaceMock(s.T())
	consentMock.On("IsEnabled").Return(false)

	svc := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
		authzService:    nil,
		consentService:  consentMock,
	}

	svcErr := svc.DeleteEntityType(context.Background(), TypeCategoryUser, "schema-1")
	s.Nil(svcErr)
}
