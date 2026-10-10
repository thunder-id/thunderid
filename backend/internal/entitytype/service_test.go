// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entitytype

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entitytype/model"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"

	"github.com/thunder-id/thunderid/tests/mocks/oumock"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

const (
	testOUID1 = "00000000-0000-0000-0000-000000000001"
	testOUID2 = "00000000-0000-0000-0000-000000000002"
	testOUID3 = "00000000-0000-0000-0000-000000000003"
)

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

func TestCreateEntityTypeReturnsErrorWhenOrganizationUnitMissing(t *testing.T) {
	// Initialize server runtime with default config
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(t, err)
	defer config.ResetServerRuntime()

	storeMock := newEntityTypeStoreInterfaceMock(t)
	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)

	ouID := testOUID1
	ouServiceMock.On("IsOrganizationUnitExists", mock.Anything, ouID).
		Return(false, (*tidcommon.ServiceError)(nil)).
		Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		ouService:       ouServiceMock,
		transactioner:   &mockTransactioner{},
	}

	request := CreateEntityTypeRequestWithID{
		Handle:      "test-schema",
		DisplayName: "test-schema",
		OUID:        ouID,
		Schema:      json.RawMessage(`{"email":{"type":"string"}}`),
	}

	createdSchema, svcErr := service.CreateEntityType(context.Background(), TypeCategoryUser, request)

	require.Nil(t, createdSchema)
	require.NotNil(t, svcErr)
	require.Equal(t, ErrorInvalidEntityTypeRequest.Code, svcErr.Code)
	require.Contains(t, svcErr.ErrorDescription.DefaultValue, "organization unit id does not exist")
}

func TestCreateEntityTypeReturnsInternalErrorWhenOUValidationFails(t *testing.T) {
	// Initialize server runtime with default config
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(t, err)
	defer config.ResetServerRuntime()

	storeMock := newEntityTypeStoreInterfaceMock(t)
	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)

	ouID := testOUID2
	ouServiceMock.
		On("IsOrganizationUnitExists", mock.Anything, ouID).
		Return(false, &tidcommon.ServiceError{Code: "OUS-5000"}).
		Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		ouService:       ouServiceMock,
		transactioner:   &mockTransactioner{},
	}

	request := CreateEntityTypeRequestWithID{
		Handle:      "test-schema",
		DisplayName: "test-schema",
		OUID:        ouID,
		Schema:      json.RawMessage(`{"email":{"type":"string"}}`),
	}

	createdSchema, svcErr := service.CreateEntityType(context.Background(), TypeCategoryUser, request)

	require.Nil(t, createdSchema)
	require.NotNil(t, svcErr)
	require.Equal(t, tidcommon.InternalServerError, *svcErr)
}

func TestUpdateEntityTypeReturnsErrorWhenOrganizationUnitMissing(t *testing.T) {
	// Initialize server runtime with default config
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(t, err)
	defer config.ResetServerRuntime()

	storeMock := newEntityTypeStoreInterfaceMock(t)
	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)

	ouID := testOUID3
	storeMock.On("IsEntityTypeDeclarative", TypeCategoryUser, "schema-id").Return(false).Once()
	ouServiceMock.On("IsOrganizationUnitExists", mock.Anything, ouID).
		Return(false, (*tidcommon.ServiceError)(nil)).
		Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		ouService:       ouServiceMock,
		transactioner:   &mockTransactioner{},
	}

	request := UpdateEntityTypeRequest{
		Handle:      "test-schema",
		DisplayName: "test-schema",
		OUID:        ouID,
		Schema:      json.RawMessage(`{"email":{"type":"string"}}`),
	}

	updatedSchema, svcErr := service.UpdateEntityType(context.Background(), TypeCategoryUser, "schema-id", request)

	require.Nil(t, updatedSchema)
	require.NotNil(t, svcErr)
	require.Equal(t, ErrorInvalidEntityTypeRequest.Code, svcErr.Code)
}

func TestCreateEntityTypeResolvesOUHandleToID(t *testing.T) {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{Enabled: false},
	}
	config.ResetServerRuntime()
	require.NoError(t, config.InitializeServerRuntime("/tmp/test", testConfig))
	defer config.ResetServerRuntime()

	storeMock := newEntityTypeStoreInterfaceMock(t)
	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)

	ouServiceMock.On("GetOrganizationUnitByPath", mock.Anything, "default").
		Return(ou.OrganizationUnit{ID: testOUID1}, (*tidcommon.ServiceError)(nil)).Once()
	ouServiceMock.On("IsOrganizationUnitExists", mock.Anything, testOUID1).
		Return(true, (*tidcommon.ServiceError)(nil)).Once()
	storeMock.On("GetEntityTypeByHandle", mock.Anything, TypeCategoryUser, "test-schema").
		Return(EntityType{}, ErrEntityTypeNotFound).Once()
	storeMock.On("CreateEntityType", mock.Anything, mock.Anything).Return(nil).Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		ouService:       ouServiceMock,
		transactioner:   &mockTransactioner{},
	}

	result, svcErr := service.CreateEntityType(context.Background(), TypeCategoryUser, CreateEntityTypeRequestWithID{
		Handle:      "test-schema",
		DisplayName: "test-schema",
		OUHandle:    "default",
		Schema:      json.RawMessage(`{"email":{"type":"string"}}`),
	})

	require.Nil(t, svcErr)
	require.NotNil(t, result)
	require.Equal(t, testOUID1, result.OUID)
}

func TestCreateEntityTypeReturnsErrorWhenOUHandleNotFound(t *testing.T) {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{Enabled: false},
	}
	config.ResetServerRuntime()
	require.NoError(t, config.InitializeServerRuntime("/tmp/test", testConfig))
	defer config.ResetServerRuntime()

	storeMock := newEntityTypeStoreInterfaceMock(t)
	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)

	ouServiceMock.On("GetOrganizationUnitByPath", mock.Anything, "missing").
		Return(ou.OrganizationUnit{}, &tidcommon.ServiceError{Code: "OUS-4004"}).Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		ouService:       ouServiceMock,
		transactioner:   &mockTransactioner{},
	}

	result, svcErr := service.CreateEntityType(context.Background(), TypeCategoryUser, CreateEntityTypeRequestWithID{
		Handle:      "test-schema",
		DisplayName: "test-schema",
		OUHandle:    "missing",
		Schema:      json.RawMessage(`{"email":{"type":"string"}}`),
	})

	require.Nil(t, result)
	require.NotNil(t, svcErr)
	require.Equal(t, ErrorInvalidEntityTypeRequest.Code, svcErr.Code)
}

func TestUpdateEntityTypeResolvesOUHandleToID(t *testing.T) {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{Enabled: false},
	}
	config.ResetServerRuntime()
	require.NoError(t, config.InitializeServerRuntime("/tmp/test", testConfig))
	defer config.ResetServerRuntime()

	storeMock := newEntityTypeStoreInterfaceMock(t)
	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)

	storeMock.On("IsEntityTypeDeclarative", TypeCategoryUser, "schema-id").Return(false).Once()
	ouServiceMock.On("GetOrganizationUnitByPath", mock.Anything, "default").
		Return(ou.OrganizationUnit{ID: testOUID1}, (*tidcommon.ServiceError)(nil)).Once()
	ouServiceMock.On("IsOrganizationUnitExists", mock.Anything, testOUID1).
		Return(true, (*tidcommon.ServiceError)(nil)).Once()
	storeMock.On("GetEntityTypeByID", mock.Anything, TypeCategoryUser, "schema-id").
		Return(EntityType{
			ID: "schema-id", Handle: "test-schema", DisplayName: "test-schema", OUID: testOUID1,
		}, nil).Once()
	storeMock.On("UpdateEntityTypeByID", mock.Anything, TypeCategoryUser, "schema-id", mock.Anything).Return(nil).Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		ouService:       ouServiceMock,
		transactioner:   &mockTransactioner{},
	}

	req := UpdateEntityTypeRequest{
		Handle:      "test-schema",
		DisplayName: "test-schema",
		OUHandle:    "default",
		Schema:      json.RawMessage(`{"email":{"type":"string"}}`),
	}
	result, svcErr := service.UpdateEntityType(context.Background(), TypeCategoryUser, "schema-id", req)

	require.Nil(t, svcErr)
	require.NotNil(t, result)
	require.Equal(t, testOUID1, result.OUID)
}

func TestUpdateEntityTypeReturnsErrorWhenOUHandleNotFound(t *testing.T) {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{Enabled: false},
	}
	config.ResetServerRuntime()
	require.NoError(t, config.InitializeServerRuntime("/tmp/test", testConfig))
	defer config.ResetServerRuntime()

	storeMock := newEntityTypeStoreInterfaceMock(t)
	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)

	storeMock.On("IsEntityTypeDeclarative", TypeCategoryUser, "schema-id").Return(false).Once()
	ouServiceMock.On("GetOrganizationUnitByPath", mock.Anything, "missing").
		Return(ou.OrganizationUnit{}, &tidcommon.ServiceError{Code: "OUS-4004"}).Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		ouService:       ouServiceMock,
		transactioner:   &mockTransactioner{},
	}

	req := UpdateEntityTypeRequest{
		Handle:      "test-schema",
		DisplayName: "test-schema",
		OUHandle:    "missing",
		Schema:      json.RawMessage(`{"email":{"type":"string"}}`),
	}
	result, svcErr := service.UpdateEntityType(context.Background(), TypeCategoryUser, "schema-id", req)

	require.Nil(t, result)
	require.NotNil(t, svcErr)
	require.Equal(t, ErrorInvalidEntityTypeRequest.Code, svcErr.Code)
}

// TestCreateEntityTypeOUIDWinsWhenBothOUIDAndOUHandleProvided verifies that when both
// ou_id and ou_handle are supplied to CreateEntityType, ou_id wins and no handle
// resolution is attempted (the absence of a GetOrganizationUnitByPath mock expectation
// asserts that). This covers the WARN-on-collision branch used by the importer/REST path.
func TestCreateEntityTypeOUIDWinsWhenBothOUIDAndOUHandleProvided(t *testing.T) {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{Enabled: false},
	}
	config.ResetServerRuntime()
	require.NoError(t, config.InitializeServerRuntime("/tmp/test", testConfig))
	defer config.ResetServerRuntime()

	storeMock := newEntityTypeStoreInterfaceMock(t)
	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)

	ouServiceMock.On("IsOrganizationUnitExists", mock.Anything, testOUID1).
		Return(true, (*tidcommon.ServiceError)(nil)).Once()
	storeMock.On("GetEntityTypeByHandle", mock.Anything, TypeCategoryUser, "test-schema").
		Return(EntityType{}, ErrEntityTypeNotFound).Once()
	storeMock.On("CreateEntityType", mock.Anything, mock.Anything).Return(nil).Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		ouService:       ouServiceMock,
		transactioner:   &mockTransactioner{},
	}

	result, svcErr := service.CreateEntityType(context.Background(), TypeCategoryUser, CreateEntityTypeRequestWithID{
		Handle:      "test-schema",
		DisplayName: "test-schema",
		OUID:        testOUID1,
		OUHandle:    "some-handle",
		Schema:      json.RawMessage(`{"email":{"type":"string"}}`),
	})

	require.Nil(t, svcErr)
	require.NotNil(t, result)
	require.Equal(t, testOUID1, result.OUID)
}

// TestUpdateEntityTypeOUIDWinsWhenBothOUIDAndOUHandleProvided verifies that when both
// ou_id and ou_handle are supplied to UpdateEntityType, ou_id wins and no handle
// resolution is attempted (the absence of a GetOrganizationUnitByPath mock expectation
// asserts that). This covers the WARN-on-collision branch used by the importer/REST path.
func TestUpdateEntityTypeOUIDWinsWhenBothOUIDAndOUHandleProvided(t *testing.T) {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{Enabled: false},
	}
	config.ResetServerRuntime()
	require.NoError(t, config.InitializeServerRuntime("/tmp/test", testConfig))
	defer config.ResetServerRuntime()

	storeMock := newEntityTypeStoreInterfaceMock(t)
	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)

	storeMock.On("IsEntityTypeDeclarative", TypeCategoryUser, "schema-id").Return(false).Once()
	ouServiceMock.On("IsOrganizationUnitExists", mock.Anything, testOUID1).
		Return(true, (*tidcommon.ServiceError)(nil)).Once()
	storeMock.On("GetEntityTypeByID", mock.Anything, TypeCategoryUser, "schema-id").
		Return(EntityType{
			ID: "schema-id", Handle: "test-schema", DisplayName: "test-schema", OUID: testOUID1,
		}, nil).Once()
	storeMock.On("UpdateEntityTypeByID", mock.Anything, TypeCategoryUser, "schema-id", mock.Anything).
		Return(nil).Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		ouService:       ouServiceMock,
		transactioner:   &mockTransactioner{},
	}

	req := UpdateEntityTypeRequest{
		Handle:      "test-schema",
		DisplayName: "test-schema",
		OUID:        testOUID1,
		OUHandle:    "some-handle",
		Schema:      json.RawMessage(`{"email":{"type":"string"}}`),
	}
	result, svcErr := service.UpdateEntityType(context.Background(), TypeCategoryUser, "schema-id", req)

	require.Nil(t, svcErr)
	require.NotNil(t, result)
	require.Equal(t, testOUID1, result.OUID)
}

func TestResolveEntityTypeHandles_OUHandleResolved(t *testing.T) {
	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	ouServiceMock.On("GetOrganizationUnitByPath", mock.Anything, "default").
		Return(ou.OrganizationUnit{ID: testOUID1}, (*tidcommon.ServiceError)(nil)).Once()

	svc := &entityTypeService{ouService: ouServiceMock}
	et := &EntityType{OUHandle: "default"}

	svcErr := svc.ResolveEntityTypeHandles(context.Background(), et)

	require.Nil(t, svcErr)
	require.Equal(t, testOUID1, et.OUID)
}

func TestResolveEntityTypeHandles_OUIDAlreadySet(t *testing.T) {
	svc := &entityTypeService{}
	et := &EntityType{OUID: testOUID1}

	svcErr := svc.ResolveEntityTypeHandles(context.Background(), et)

	require.Nil(t, svcErr)
	require.Equal(t, testOUID1, et.OUID)
}

func TestResolveEntityTypeHandles_OUHandleNotFound(t *testing.T) {
	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	ouServiceMock.On("GetOrganizationUnitByPath", mock.Anything, "bad").
		Return(ou.OrganizationUnit{}, &tidcommon.ServiceError{Code: "OUS-4004"}).Once()

	svc := &entityTypeService{ouService: ouServiceMock}
	et := &EntityType{OUHandle: "bad"}

	svcErr := svc.ResolveEntityTypeHandles(context.Background(), et)

	require.NotNil(t, svcErr)
	require.Equal(t, ErrorInvalidRequestFormat.Code, svcErr.Code)
}

func TestResolveEntityTypeHandles_NilOUService(t *testing.T) {
	svc := &entityTypeService{ouService: nil}
	et := &EntityType{OUHandle: "default"}

	svcErr := svc.ResolveEntityTypeHandles(context.Background(), et)

	require.NotNil(t, svcErr)
	require.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)
}

// TestResolveEntityTypeHandles_DeclarativeLoaderUsesRuntimeContext verifies the public
// ResolveEntityTypeHandles entry point (used by the startup-time declarative loader, which
// has no user context) elevates the caller context to a runtime context before invoking the
// OU lookup. This is what allows file-based entity types to be loaded without an authenticated
// subject.
func TestResolveEntityTypeHandles_DeclarativeLoaderUsesRuntimeContext(t *testing.T) {
	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	var capturedCtx context.Context
	ouServiceMock.On("GetOrganizationUnitByPath", mock.Anything, "default").
		Run(func(args mock.Arguments) {
			capturedCtx = args.Get(0).(context.Context)
		}).
		Return(ou.OrganizationUnit{ID: testOUID1}, (*tidcommon.ServiceError)(nil)).Once()

	svc := &entityTypeService{ouService: ouServiceMock}
	et := &EntityType{OUHandle: "default"}

	svcErr := svc.ResolveEntityTypeHandles(context.Background(), et)

	require.Nil(t, svcErr)
	require.NotNil(t, capturedCtx)
	require.True(t, security.IsRuntimeContext(capturedCtx),
		"declarative loader path should elevate to runtime context")
}

// TestCreateEntityType_OUHandleLookupUsesCallerContext verifies that the API/REST path
// (CreateEntityType) does NOT elevate to a runtime context when resolving ou_handle. The
// caller's context must propagate so that the underlying OU lookup is still subject to the
// caller's ou:read authorization. Without this, an API caller without ou:read could bypass
// authorization by supplying ou_handle instead of ou_id.
func TestCreateEntityType_OUHandleLookupUsesCallerContext(t *testing.T) {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{Enabled: false},
	}
	config.ResetServerRuntime()
	require.NoError(t, config.InitializeServerRuntime("/tmp/test", testConfig))
	defer config.ResetServerRuntime()

	storeMock := newEntityTypeStoreInterfaceMock(t)
	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)

	var capturedCtx context.Context
	ouServiceMock.On("GetOrganizationUnitByPath", mock.Anything, "default").
		Run(func(args mock.Arguments) {
			capturedCtx = args.Get(0).(context.Context)
		}).
		Return(ou.OrganizationUnit{ID: testOUID1}, (*tidcommon.ServiceError)(nil)).Once()
	ouServiceMock.On("IsOrganizationUnitExists", mock.Anything, testOUID1).
		Return(true, (*tidcommon.ServiceError)(nil)).Once()
	storeMock.On("GetEntityTypeByHandle", mock.Anything, TypeCategoryUser, "test-schema").
		Return(EntityType{}, ErrEntityTypeNotFound).Once()
	storeMock.On("CreateEntityType", mock.Anything, mock.Anything).Return(nil).Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		ouService:       ouServiceMock,
		transactioner:   &mockTransactioner{},
	}

	result, svcErr := service.CreateEntityType(context.Background(), TypeCategoryUser, CreateEntityTypeRequestWithID{
		Handle:      "test-schema",
		DisplayName: "test-schema",
		OUHandle:    "default",
		Schema:      json.RawMessage(`{"email":{"type":"string"}}`),
	})

	require.Nil(t, svcErr)
	require.NotNil(t, result)
	require.NotNil(t, capturedCtx)
	require.False(t, security.IsRuntimeContext(capturedCtx),
		"API path must propagate the caller's context (not runtime) so ou:read is enforced")
}

// TestUpdateEntityType_OUHandleLookupUsesCallerContext is the UpdateEntityType analog of
// TestCreateEntityType_OUHandleLookupUsesCallerContext.
func TestUpdateEntityType_OUHandleLookupUsesCallerContext(t *testing.T) {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{Enabled: false},
	}
	config.ResetServerRuntime()
	require.NoError(t, config.InitializeServerRuntime("/tmp/test", testConfig))
	defer config.ResetServerRuntime()

	storeMock := newEntityTypeStoreInterfaceMock(t)
	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)

	var capturedCtx context.Context
	storeMock.On("IsEntityTypeDeclarative", TypeCategoryUser, "schema-id").Return(false).Once()
	ouServiceMock.On("GetOrganizationUnitByPath", mock.Anything, "default").
		Run(func(args mock.Arguments) {
			capturedCtx = args.Get(0).(context.Context)
		}).
		Return(ou.OrganizationUnit{ID: testOUID1}, (*tidcommon.ServiceError)(nil)).Once()
	ouServiceMock.On("IsOrganizationUnitExists", mock.Anything, testOUID1).
		Return(true, (*tidcommon.ServiceError)(nil)).Once()
	storeMock.On("GetEntityTypeByID", mock.Anything, TypeCategoryUser, "schema-id").
		Return(EntityType{
			ID: "schema-id", Handle: "test-schema", DisplayName: "test-schema", OUID: testOUID1,
		}, nil).Once()
	storeMock.On("UpdateEntityTypeByID", mock.Anything, TypeCategoryUser, "schema-id", mock.Anything).
		Return(nil).Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		ouService:       ouServiceMock,
		transactioner:   &mockTransactioner{},
	}

	req := UpdateEntityTypeRequest{
		Handle:      "test-schema",
		DisplayName: "test-schema",
		OUHandle:    "default",
		Schema:      json.RawMessage(`{"email":{"type":"string"}}`),
	}
	result, svcErr := service.UpdateEntityType(context.Background(), TypeCategoryUser, "schema-id", req)

	require.Nil(t, svcErr)
	require.NotNil(t, result)
	require.NotNil(t, capturedCtx)
	require.False(t, security.IsRuntimeContext(capturedCtx),
		"API path must propagate the caller's context (not runtime) so ou:read is enforced")
}

func TestGetEntityTypeByHandleReturnsSchema(t *testing.T) {
	storeMock := newEntityTypeStoreInterfaceMock(t)
	expectedSchema := EntityType{
		ID:          "schema-id",
		Handle:      "employee",
		DisplayName: "employee",
	}
	storeMock.
		On("GetEntityTypeByHandle", context.Background(), TypeCategoryUser, "employee").
		Return(expectedSchema, nil).
		Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
		authzService:    newAllowAllAuthz(t),
	}

	entityType, svcErr := service.GetEntityTypeByHandle(context.Background(), TypeCategoryUser, "employee")

	require.Nil(t, svcErr)
	require.NotNil(t, entityType)
	require.Equal(t, &expectedSchema, entityType)
}

func TestGetEntityTypeByHandleReturnsNotFound(t *testing.T) {
	storeMock := newEntityTypeStoreInterfaceMock(t)
	storeMock.
		On("GetEntityTypeByHandle", context.Background(), TypeCategoryUser, "employee").
		Return(EntityType{}, ErrEntityTypeNotFound).
		Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
	}

	entityType, svcErr := service.GetEntityTypeByHandle(context.Background(), TypeCategoryUser, "employee")

	require.Nil(t, entityType)
	require.NotNil(t, svcErr)
	require.Equal(t, ErrorEntityTypeNotFound.Code, svcErr.Code)
}

func TestGetEntityTypeByHandleReturnsInternalErrorOnStoreFailure(t *testing.T) {
	storeMock := newEntityTypeStoreInterfaceMock(t)
	storeMock.
		On("GetEntityTypeByHandle", context.Background(), TypeCategoryUser, "employee").
		Return(EntityType{}, errors.New("db failure")).
		Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
	}

	entityType, svcErr := service.GetEntityTypeByHandle(context.Background(), TypeCategoryUser, "employee")

	require.Nil(t, entityType)
	require.NotNil(t, svcErr)
	require.Equal(t, tidcommon.InternalServerError, *svcErr)
}

func TestGetEntityTypeByHandleRequiresHandle(t *testing.T) {
	storeMock := newEntityTypeStoreInterfaceMock(t)

	service := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
	}

	entityType, svcErr := service.GetEntityTypeByHandle(context.Background(), TypeCategoryUser, "")

	require.Nil(t, entityType)
	require.NotNil(t, svcErr)
	require.Equal(t, ErrorInvalidEntityTypeRequest.Code, svcErr.Code)
}

func TestValidateEntityReturnsTrueWhenValidationPasses(t *testing.T) {
	storeMock := newEntityTypeStoreInterfaceMock(t)
	storeMock.
		On("GetEntityTypeByHandle", context.Background(), TypeCategoryUser, "employee").
		Return(EntityType{
			Handle:      "employee",
			DisplayName: "employee",
			Schema:      json.RawMessage(`{"email":{"type":"string","required":true}}`),
		}, nil).
		Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
	}

	ok, svcErr := service.ValidateEntity(
		context.Background(), TypeCategoryUser,
		"employee",
		json.RawMessage(`{"email":"employee@example.com"}`),
		false,
	)

	require.True(t, ok)
	require.Nil(t, svcErr)
}

func TestValidateEntityReturnsInternalErrorWhenSchemaLoadFails(t *testing.T) {
	storeMock := newEntityTypeStoreInterfaceMock(t)
	storeMock.
		On("GetEntityTypeByHandle", context.Background(), TypeCategoryUser, "employee").
		Return(EntityType{}, errors.New("db failure")).
		Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
	}

	ok, svcErr := service.ValidateEntity(
		context.Background(), TypeCategoryUser, "employee", json.RawMessage(`{}`), false)

	require.False(t, ok)
	require.NotNil(t, svcErr)
	require.Equal(t, tidcommon.InternalServerError, *svcErr)
}

func TestValidateEntityUniquenessReturnsTrueWhenNoConflicts(t *testing.T) {
	storeMock := newEntityTypeStoreInterfaceMock(t)
	storeMock.
		On("GetEntityTypeByHandle", context.Background(), TypeCategoryUser, "employee").
		Return(EntityType{
			Handle:      "employee",
			DisplayName: "employee",
			Schema:      json.RawMessage(`{"email":{"type":"string","unique":true}}`),
		}, nil).
		Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
	}

	ok, svcErr := service.ValidateEntityUniqueness(
		context.Background(), TypeCategoryUser,
		"employee",
		json.RawMessage(`{"email":"unique@example.com"}`),
		func(filters map[string]interface{}) (bool, error) {
			require.Equal(t, map[string]interface{}{"email": "unique@example.com"}, filters)
			return false, nil
		},
	)

	require.True(t, ok)
	require.Nil(t, svcErr)
}

func TestValidateEntityReturnsSchemaNotFoundWhenSchemaMissing(t *testing.T) {
	storeMock := newEntityTypeStoreInterfaceMock(t)
	storeMock.
		On("GetEntityTypeByHandle", context.Background(), TypeCategoryUser, "employee").
		Return(EntityType{}, ErrEntityTypeNotFound).
		Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
	}

	ok, svcErr := service.ValidateEntity(
		context.Background(), TypeCategoryUser,
		"employee",
		json.RawMessage(`{"email":"employee@example.com"}`),
		false,
	)

	require.False(t, ok)
	require.NotNil(t, svcErr)
	require.Equal(t, ErrorEntityTypeNotFound.Code, svcErr.Code)
}

func TestValidateEntityUniquenessReturnsSchemaNotFoundWhenSchemaMissing(t *testing.T) {
	storeMock := newEntityTypeStoreInterfaceMock(t)
	storeMock.
		On("GetEntityTypeByHandle", context.Background(), TypeCategoryUser, "employee").
		Return(EntityType{}, ErrEntityTypeNotFound).
		Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
	}

	ok, svcErr := service.ValidateEntityUniqueness(
		context.Background(), TypeCategoryUser,
		"employee",
		json.RawMessage(`{}`),
		func(map[string]interface{}) (bool, error) { return false, nil },
	)

	require.False(t, ok)
	require.NotNil(t, svcErr)
	require.Equal(t, ErrorEntityTypeNotFound.Code, svcErr.Code)
}

func TestValidateEntityUniquenessReturnsInternalErrorWhenSchemaLoadFails(t *testing.T) {
	storeMock := newEntityTypeStoreInterfaceMock(t)
	storeMock.
		On("GetEntityTypeByHandle", context.Background(), TypeCategoryUser, "employee").
		Return(EntityType{}, errors.New("db failure")).
		Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
	}

	ok, svcErr := service.ValidateEntityUniqueness(
		context.Background(), TypeCategoryUser,
		"employee",
		json.RawMessage(`{}`),
		func(map[string]interface{}) (bool, error) { return false, nil },
	)

	require.False(t, ok)
	require.NotNil(t, svcErr)
	require.Equal(t, tidcommon.InternalServerError, *svcErr)
}

func TestValidateEntityTypeDefinitionSuccess(t *testing.T) {
	validOUID := testOUID1
	validSchema := json.RawMessage(`{"email":{"type":"string","required":true}}`)

	schema := EntityType{
		Handle:      "test-schema",
		DisplayName: "test-schema",
		OUID:        validOUID,
		Schema:      validSchema,
	}

	err := validateEntityTypeDefinition(context.Background(), TypeCategoryUser, schema)

	require.Nil(t, err)
}

func TestValidateEntityTypeDefinitionReturnsErrorWhenDisplayNameIsEmpty(t *testing.T) {
	validOUID := testOUID1
	validSchema := json.RawMessage(`{"email":{"type":"string"}}`)

	schema := EntityType{
		Handle:      "test-schema",
		DisplayName: "",
		OUID:        validOUID,
		Schema:      validSchema,
	}

	err := validateEntityTypeDefinition(context.Background(), TypeCategoryUser, schema)

	require.NotNil(t, err)
	require.Equal(t, ErrorInvalidEntityTypeRequest.Code, err.Code)
	require.Contains(t, err.ErrorDescription.DefaultValue, "entity type display name must not be empty")
}

func TestValidateEntityTypeDefinitionReturnsErrorWhenOUIDIsEmpty(t *testing.T) {
	validSchema := json.RawMessage(`{"email":{"type":"string"}}`)

	schema := EntityType{
		Handle:      "test-schema",
		DisplayName: "test-schema",
		OUID:        "",
		Schema:      validSchema,
	}

	err := validateEntityTypeDefinition(context.Background(), TypeCategoryUser, schema)

	require.NotNil(t, err)
	require.Equal(t, ErrorInvalidEntityTypeRequest.Code, err.Code)
	require.Contains(t, err.ErrorDescription.DefaultValue, "organization unit id must not be empty")
}

func TestValidateEntityTypeDefinitionAllowsNonUUIDOUID(t *testing.T) {
	validSchema := json.RawMessage(`{"email":{"type":"string"}}`)

	schema := EntityType{
		Handle:      "test-schema",
		DisplayName: "test-schema",
		OUID:        "not-a-uuid",
		Schema:      validSchema,
	}

	err := validateEntityTypeDefinition(context.Background(), TypeCategoryUser, schema)

	require.Nil(t, err)
}

func TestValidateEntityTypeDefinitionReturnsErrorWhenSchemaIsEmpty(t *testing.T) {
	validOUID := testOUID1

	schema := EntityType{
		Handle:      "test-schema",
		DisplayName: "test-schema",
		OUID:        validOUID,
		Schema:      json.RawMessage{},
	}

	err := validateEntityTypeDefinition(context.Background(), TypeCategoryUser, schema)

	require.NotNil(t, err)
	require.Equal(t, ErrorInvalidEntityTypeRequest.Code, err.Code)
	require.Contains(t, err.ErrorDescription.DefaultValue, "schema definition must not be empty")
}

func TestValidateEntityTypeDefinitionReturnsErrorWhenSchemaIsNil(t *testing.T) {
	validOUID := testOUID1

	schema := EntityType{
		Handle:      "test-schema",
		DisplayName: "test-schema",
		OUID:        validOUID,
		Schema:      nil,
	}

	err := validateEntityTypeDefinition(context.Background(), TypeCategoryUser, schema)

	require.NotNil(t, err)
	require.Equal(t, ErrorInvalidEntityTypeRequest.Code, err.Code)
	require.Contains(t, err.ErrorDescription.DefaultValue, "schema definition must not be empty")
}

func TestValidateEntityTypeDefinitionReturnsErrorWhenSchemaCompilationFails(t *testing.T) {
	validOUID := testOUID1
	invalidSchema := json.RawMessage(`{"email":"invalid"}`)

	schema := EntityType{
		Handle:      "test-schema",
		DisplayName: "test-schema",
		OUID:        validOUID,
		Schema:      invalidSchema,
	}

	err := validateEntityTypeDefinition(context.Background(), TypeCategoryUser, schema)

	require.NotNil(t, err)
	require.Equal(t, ErrorInvalidEntityTypeRequest.Code, err.Code)
	require.Contains(t, err.ErrorDescription.DefaultValue, "property definition must be an object")
}

func TestValidateEntityTypeDefinitionReturnsErrorForInvalidJSON(t *testing.T) {
	validOUID := testOUID1
	invalidSchema := json.RawMessage(`{invalid json}`)

	schema := EntityType{
		Handle:      "test-schema",
		DisplayName: "test-schema",
		OUID:        validOUID,
		Schema:      invalidSchema,
	}

	err := validateEntityTypeDefinition(context.Background(), TypeCategoryUser, schema)

	require.NotNil(t, err)
	require.Equal(t, ErrorInvalidEntityTypeRequest.Code, err.Code)
}

func TestValidateEntityTypeDefinitionReturnsErrorForEmptySchemaObject(t *testing.T) {
	validOUID := testOUID1
	emptySchema := json.RawMessage(`{}`)

	schema := EntityType{
		Handle:      "test-schema",
		DisplayName: "test-schema",
		OUID:        validOUID,
		Schema:      emptySchema,
	}

	err := validateEntityTypeDefinition(context.Background(), TypeCategoryUser, schema)

	require.NotNil(t, err)
	require.Equal(t, ErrorInvalidEntityTypeRequest.Code, err.Code)
	require.Contains(t, err.ErrorDescription.DefaultValue, "schema cannot be empty")
}

func TestValidateEntityTypeDefinitionWithComplexSchema(t *testing.T) {
	validOUID := testOUID1
	complexSchema := json.RawMessage(`{
		"email": {
			"type": "string",
			"required": true,
			"unique": true,
			"pattern": "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"
		},
		"age": {
			"type": "number",
			"required": false
		},
		"isActive": {
			"type": "boolean",
			"required": true
		},
		"address": {
			"type": "object",
			"properties": {
				"street": {"type": "string"},
				"city": {"type": "string"}
			}
		},
		"tags": {
			"type": "array",
			"items": {"type": "string"}
		}
	}`)

	schema := EntityType{
		Handle:      "complex-schema",
		DisplayName: "complex-schema",
		OUID:        validOUID,
		Schema:      complexSchema,
	}

	err := validateEntityTypeDefinition(context.Background(), TypeCategoryUser, schema)

	require.Nil(t, err)
}

func TestValidateEntityTypeDefinitionReturnsErrorForMissingTypeField(t *testing.T) {
	validOUID := testOUID1
	schemaWithoutType := json.RawMessage(`{"email":{"required":true}}`)

	schema := EntityType{
		Handle:      "test-schema",
		DisplayName: "test-schema",
		OUID:        validOUID,
		Schema:      schemaWithoutType,
	}

	err := validateEntityTypeDefinition(context.Background(), TypeCategoryUser, schema)

	require.NotNil(t, err)
	require.Equal(t, ErrorInvalidEntityTypeRequest.Code, err.Code)
	require.Contains(t, err.ErrorDescription.DefaultValue, "missing required 'type' field")
}

func TestValidateEntityTypeDefinitionReturnsErrorForInvalidType(t *testing.T) {
	validOUID := testOUID1
	schemaWithInvalidType := json.RawMessage(`{"email":{"type":"invalid-type"}}`)

	schema := EntityType{
		Handle:      "test-schema",
		DisplayName: "test-schema",
		OUID:        validOUID,
		Schema:      schemaWithInvalidType,
	}

	err := validateEntityTypeDefinition(context.Background(), TypeCategoryUser, schema)

	require.NotNil(t, err)
	require.Equal(t, ErrorInvalidEntityTypeRequest.Code, err.Code)
}

func TestValidateEntityTypeDefinitionWithMultipleValidationErrors(t *testing.T) {
	testCases := []struct {
		name          string
		schema        EntityType
		expectedError string
	}{
		{
			name: "Empty display name and empty OU ID",
			schema: EntityType{
				Handle:      "test",
				DisplayName: "",
				OUID:        "",
				Schema:      json.RawMessage(`{"email":{"type":"string"}}`),
			},
			expectedError: "entity type display name must not be empty",
		},
		{
			name: "Display name exceeds max length",
			schema: EntityType{
				Handle:      "test",
				DisplayName: strings.Repeat("a", 101),
				OUID:        "123",
				Schema:      json.RawMessage(`{"email":{"type":"string"}}`),
			},
			expectedError: "entity type display name must not exceed 100 characters",
		},
		{
			name: "Non-UUID OU ID still validates schema payload",
			schema: EntityType{
				Handle:      "test",
				DisplayName: "test",
				OUID:        "123",
				Schema:      json.RawMessage{},
			},
			expectedError: "schema definition must not be empty",
		},
		{
			name: "Valid OU ID but empty schema",
			schema: EntityType{
				Handle:      "test",
				DisplayName: "test",
				OUID:        testOUID1,
				Schema:      json.RawMessage{},
			},
			expectedError: "schema definition must not be empty",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateEntityTypeDefinition(context.Background(), TypeCategoryUser, tc.schema)

			require.NotNil(t, err)
			require.Equal(t, ErrorInvalidEntityTypeRequest.Code, err.Code)
			require.Contains(t, err.ErrorDescription.DefaultValue, tc.expectedError)
		})
	}
}

func TestValidateEntityTypeDefinitionCountsDisplayNameCharacters(t *testing.T) {
	schema := EntityType{
		Handle:      "test-schema",
		DisplayName: strings.Repeat("é", 100),
		OUID:        testOUID1,
		Schema:      json.RawMessage(`{"email":{"type":"string"}}`),
	}

	err := validateEntityTypeDefinition(context.Background(), TypeCategoryUser, schema)

	require.Nil(t, err)
}

func TestValidateEntityTypeDefinitionWithValidDisplayAttribute(t *testing.T) {
	schema := EntityType{
		Handle:           "test-schema",
		DisplayName:      "test-schema",
		OUID:             testOUID1,
		SystemAttributes: &SystemAttributes{Display: "email"},
		Schema:           json.RawMessage(`{"email":{"type":"string"}}`),
	}

	err := validateEntityTypeDefinition(context.Background(), TypeCategoryUser, schema)

	require.Nil(t, err)
}

func TestValidateEntityTypeDefinitionRejectsNonExistentDisplayAttribute(t *testing.T) {
	schema := EntityType{
		Handle:           "test-schema",
		DisplayName:      "test-schema",
		OUID:             testOUID1,
		SystemAttributes: &SystemAttributes{Display: "unknown"},
		Schema:           json.RawMessage(`{"email":{"type":"string"}}`),
	}

	err := validateEntityTypeDefinition(context.Background(), TypeCategoryUser, schema)

	require.NotNil(t, err)
	require.Equal(t, ErrorInvalidDisplayAttribute.Code, err.Code)
}

func TestValidateEntityTypeDefinitionRejectsNonDisplayableDisplayAttribute(t *testing.T) {
	schema := EntityType{
		Handle:           "test-schema",
		DisplayName:      "test-schema",
		OUID:             testOUID1,
		SystemAttributes: &SystemAttributes{Display: "active"},
		Schema:           json.RawMessage(`{"active":{"type":"boolean"}}`),
	}

	err := validateEntityTypeDefinition(context.Background(), TypeCategoryUser, schema)

	require.NotNil(t, err)
	require.Equal(t, ErrorNonDisplayableAttribute.Code, err.Code)
}

func TestValidateEntityTypeDefinitionRejectsCredentialDisplayAttribute(t *testing.T) {
	schema := EntityType{
		Handle:           "test-schema",
		DisplayName:      "test-schema",
		OUID:             testOUID1,
		SystemAttributes: &SystemAttributes{Display: "password"},
		Schema:           json.RawMessage(`{"password":{"type":"string","credential":true}}`),
	}

	err := validateEntityTypeDefinition(context.Background(), TypeCategoryUser, schema)

	require.NotNil(t, err)
	require.Equal(t, ErrorCredentialDisplayAttribute.Code, err.Code)
}

func TestValidateEntityTypeDefinitionWithNilSystemAttributes(t *testing.T) {
	schema := EntityType{
		Handle:      "test-schema",
		DisplayName: "test-schema",
		OUID:        testOUID1,
		Schema:      json.RawMessage(`{"email":{"type":"string"}}`),
	}

	err := validateEntityTypeDefinition(context.Background(), TypeCategoryUser, schema)

	require.Nil(t, err)
}

type EntityTypeServiceTestSuite struct {
	suite.Suite
}

func TestEntityTypeServiceTestSuite(t *testing.T) {
	suite.Run(t, new(EntityTypeServiceTestSuite))
}

func (s *EntityTypeServiceTestSuite) TestGetAttributes_Credential_ReturnsCredentialFieldInfos() {
	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	storeMock.
		On("GetEntityTypeByHandle", context.Background(), TypeCategoryUser, "customer").
		Return(EntityType{
			Schema: json.RawMessage(
				`{"password":{"type":"string","credential":true},` +
					`"apiKey":{"type":"string","credential":true},` +
					`"email":{"type":"string","unique":true}}`,
			),
		}, nil).
		Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
	}

	attrs, svcErr := service.GetAttributes(
		context.Background(), TypeCategoryUser, "customer", AttributeFilter{AllowCredential: true},
	)

	s.Require().Nil(svcErr)
	s.Require().Len(attrs, 2)

	attrMap := make(map[string]AttributeInfo, len(attrs))
	for _, a := range attrs {
		attrMap[a.Attribute] = a
	}
	_, hasPassword := attrMap["password"]
	s.True(hasPassword)
	_, hasAPIKey := attrMap["apiKey"]
	s.True(hasAPIKey)
}

func (s *EntityTypeServiceTestSuite) TestGetAttributes_Credential_NoCredentials_ReturnsEmpty() {
	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	storeMock.
		On("GetEntityTypeByHandle", context.Background(), TypeCategoryUser, "customer").
		Return(EntityType{
			Schema: json.RawMessage(
				`{"email":{"type":"string"},"age":{"type":"number"}}`,
			),
		}, nil).
		Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
	}

	attrs, svcErr := service.GetAttributes(
		context.Background(), TypeCategoryUser, "customer", AttributeFilter{AllowCredential: true},
	)

	s.Require().Nil(svcErr)
	s.Require().Empty(attrs)
}

func (s *EntityTypeServiceTestSuite) TestGetAttributes_SchemaNotFound_ReturnsError() {
	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	storeMock.
		On("GetEntityTypeByHandle", context.Background(), TypeCategoryUser, "unknown").
		Return(EntityType{}, ErrEntityTypeNotFound).
		Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
	}

	attrs, svcErr := service.GetAttributes(
		context.Background(), TypeCategoryUser, "unknown", AttributeFilter{AllowCredential: true},
	)

	s.Require().Nil(attrs)
	s.Require().NotNil(svcErr)
	s.Require().Equal(ErrorEntityTypeNotFound.Code, svcErr.Code)
}

func (s *EntityTypeServiceTestSuite) TestGetAttributesForEntityType_ReturnsAttributesByCategory() {
	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	storeMock.
		On("GetEntityTypeByHandle", context.Background(), TypeCategoryUser, "employee").
		Return(EntityType{Schema: json.RawMessage(`{"email":{"type":"string"}}`)}, nil).
		Once()
	storeMock.
		On("GetEntityTypeByHandle", context.Background(), TypeCategoryAgent, "employee").
		Return(EntityType{Schema: json.RawMessage(`{"clientId":{"type":"string"}}`)}, nil).
		Once()
	service := &entityTypeService{entityTypeStore: storeMock, transactioner: &mockTransactioner{}}

	attributes, svcErr := service.GetAttributesForEntityType(
		context.Background(), "employee", AttributeFilter{AllowNonCredential: true},
	)

	s.Require().Nil(svcErr)
	s.Require().Equal([]AttributeInfo{{Attribute: "email", Type: "string"}}, attributes[TypeCategoryUser])
	s.Require().Equal([]AttributeInfo{{Attribute: "clientId", Type: "string"}}, attributes[TypeCategoryAgent])
}

func (s *EntityTypeServiceTestSuite) TestGetAttributes_EmptyEntityType_ReturnsError() {
	storeMock := newEntityTypeStoreInterfaceMock(s.T())

	service := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
	}

	attrs, svcErr := service.GetAttributes(
		context.Background(), TypeCategoryUser, "", AttributeFilter{AllowCredential: true},
	)

	s.Require().Nil(attrs)
	s.Require().NotNil(svcErr)
	s.Require().Equal(ErrorEntityTypeNotFound.Code, svcErr.Code)
}

func (s *EntityTypeServiceTestSuite) TestGetAttributes_StoreError_ReturnsInternalError() {
	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	storeMock.
		On("GetEntityTypeByHandle", context.Background(), TypeCategoryUser, "customer").
		Return(EntityType{}, errors.New("db failure")).
		Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
	}

	attrs, svcErr := service.GetAttributes(
		context.Background(), TypeCategoryUser, "customer", AttributeFilter{AllowCredential: true},
	)

	s.Require().Nil(attrs)
	s.Require().NotNil(svcErr)
	s.Require().Equal(tidcommon.InternalServerError, *svcErr)
}

func (s *EntityTypeServiceTestSuite) TestGetAttributes_CredentialRequiredOnly_ReturnsOnlyRequiredCredential() {
	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	storeMock.
		On("GetEntityTypeByHandle", context.Background(), TypeCategoryUser, "customer").
		Return(EntityType{
			Schema: json.RawMessage(
				`{"password":{"type":"string","required":true,"credential":true,"displayName":"Password"},` +
					`"pin":{"type":"string","credential":true,"displayName":"PIN"},` +
					`"email":{"type":"string","required":true}}`,
			),
		}, nil).
		Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
	}

	attrs, svcErr := service.GetAttributes(
		context.Background(), TypeCategoryUser, "customer", AttributeFilter{AllowCredential: true, RequiredOnly: true},
	)

	s.Require().Nil(svcErr)
	s.Require().Len(attrs, 1)
	s.Equal("password", attrs[0].Attribute)
	s.Equal("Password", attrs[0].DisplayName)
	s.True(attrs[0].Required)
}

func (s *EntityTypeServiceTestSuite) TestGetAttributes_CredentialAllAttrs_IncludesOptional() {
	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	storeMock.
		On("GetEntityTypeByHandle", context.Background(), TypeCategoryUser, "customer").
		Return(EntityType{
			Schema: json.RawMessage(
				`{"password":{"type":"string","required":true,"credential":true,"displayName":"Password"},` +
					`"pin":{"type":"string","credential":true,"displayName":"PIN"},` +
					`"email":{"type":"string","required":true}}`,
			),
		}, nil).
		Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
	}

	attrs, svcErr := service.GetAttributes(
		context.Background(), TypeCategoryUser, "customer", AttributeFilter{AllowCredential: true},
	)

	s.Require().Nil(svcErr)
	s.Require().Len(attrs, 2)

	attrMap := make(map[string]AttributeInfo, len(attrs))
	for _, a := range attrs {
		attrMap[a.Attribute] = a
	}

	s.True(attrMap["password"].Required)
	s.Equal("Password", attrMap["password"].DisplayName)
	s.False(attrMap["pin"].Required)
	s.Equal("PIN", attrMap["pin"].DisplayName)
	_, hasEmail := attrMap["email"]
	s.False(hasEmail, "non-credential attribute must be excluded")
}

func (s *EntityTypeServiceTestSuite) TestGetUniqueAttributes_ReturnsUniqueFieldNames() {
	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	storeMock.
		On("GetEntityTypeByHandle", context.Background(), TypeCategoryUser, "customer").
		Return(EntityType{
			Schema: json.RawMessage(
				`{"email":{"type":"string","unique":true},` +
					`"username":{"type":"string","unique":true},` +
					`"given_name":{"type":"string"}}`,
			),
		}, nil).
		Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
	}

	fields, svcErr := service.GetUniqueAttributes(context.Background(), TypeCategoryUser, "customer")

	s.Require().Nil(svcErr)
	sort.Strings(fields)
	s.Require().Equal([]string{"email", "username"}, fields)
}

func (s *EntityTypeServiceTestSuite) TestGetUniqueAttributes_TestNoUniqueAttributes_ReturnsEmpty() {
	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	storeMock.
		On("GetEntityTypeByHandle", context.Background(), TypeCategoryUser, "customer").
		Return(EntityType{
			Schema: json.RawMessage(`{"given_name":{"type":"string"},"age":{"type":"number"}}`),
		}, nil).
		Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
	}

	fields, svcErr := service.GetUniqueAttributes(context.Background(), TypeCategoryUser, "customer")

	s.Require().Nil(svcErr)
	s.Require().Empty(fields)
}

func (s *EntityTypeServiceTestSuite) TestGetUniqueAttributes_TestSchemaNotFound_ReturnsError() {
	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	storeMock.
		On("GetEntityTypeByHandle", context.Background(), TypeCategoryUser, "unknown").
		Return(EntityType{}, ErrEntityTypeNotFound).
		Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
	}

	fields, svcErr := service.GetUniqueAttributes(context.Background(), TypeCategoryUser, "unknown")

	s.Require().Nil(fields)
	s.Require().NotNil(svcErr)
	s.Require().Equal(ErrorEntityTypeNotFound.Code, svcErr.Code)
}

func (s *EntityTypeServiceTestSuite) TestGetUniqueAttributes_TestEmptyUserType_ReturnsError() {
	storeMock := newEntityTypeStoreInterfaceMock(s.T())

	service := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
	}

	fields, svcErr := service.GetUniqueAttributes(context.Background(), TypeCategoryUser, "")

	s.Require().Nil(fields)
	s.Require().NotNil(svcErr)
	s.Require().Equal(ErrorEntityTypeNotFound.Code, svcErr.Code)
}

// ----- DeleteEntityType Tests -----

func TestDeleteEntityType(t *testing.T) {
	const schemaID = "schema-456"

	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(t, err)
	defer config.ResetServerRuntime()

	storeMock := newEntityTypeStoreInterfaceMock(t)
	storeMock.On("GetEntityTypeByID", mock.Anything, mock.Anything, schemaID).Return(EntityType{
		ID:     schemaID,
		OUID:   testOUID1,
		Schema: json.RawMessage(`{"email":{"type":"string"}}`),
	}, nil).Once()
	storeMock.On("IsEntityTypeDeclarative", TypeCategoryUser, schemaID).Return(false).Once()
	storeMock.On("DeleteEntityTypeByID", mock.Anything, mock.Anything, schemaID).Return(nil).Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
		authzService:    newAllowAllAuthz(t),
	}

	svcErr := service.DeleteEntityType(context.Background(), TypeCategoryUser, schemaID)

	require.Nil(t, svcErr)
	storeMock.AssertExpectations(t)
}

func TestCreateEntityType_AgentTypeRejectsNonDefaultHandle(t *testing.T) {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{Enabled: false},
	}
	config.ResetServerRuntime()
	require.NoError(t, config.InitializeServerRuntime("/tmp/test", testConfig))
	defer config.ResetServerRuntime()

	service := &entityTypeService{
		entityTypeStore: newEntityTypeStoreInterfaceMock(t),
		transactioner:   &mockTransactioner{},
		authzService:    newAllowAllAuthz(t),
	}

	req := CreateEntityTypeRequestWithID{
		Handle:      "tool-agent",
		DisplayName: "tool-agent",
		OUID:        testOUID1,
		Schema:      json.RawMessage(`{"name":{"type":"string"}}`),
	}
	_, svcErr := service.CreateEntityType(context.Background(), TypeCategoryAgent, req)
	require.NotNil(t, svcErr)
	require.Equal(t, ErrorAgentTypeOnlyDefaultAllowed.Code, svcErr.Code)
}

func TestUpdateEntityType_AgentTypeRejectsNonDefaultHandle(t *testing.T) {
	initEntityTypeTestRuntime(t)

	service := &entityTypeService{
		entityTypeStore: newEntityTypeStoreInterfaceMock(t),
		transactioner:   &mockTransactioner{},
		authzService:    newAllowAllAuthz(t),
	}

	req := UpdateEntityTypeRequest{
		Handle:      "tool-agent",
		DisplayName: "Tool Agent",
		OUID:        testOUID1,
		Schema:      json.RawMessage(`{"name":{"type":"string"}}`),
	}
	_, svcErr := service.UpdateEntityType(context.Background(), TypeCategoryAgent, "schema-1", req)
	require.NotNil(t, svcErr)
	require.Equal(t, ErrorAgentTypeOnlyDefaultAllowed.Code, svcErr.Code)
}

func TestUpdateEntityType_AgentTypeOmittedHandleKeepsDefault(t *testing.T) {
	initEntityTypeTestRuntime(t)

	storeMock := newEntityTypeStoreInterfaceMock(t)
	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	storeMock.On("IsEntityTypeDeclarative", TypeCategoryAgent, "schema-id").Return(false).Once()
	ouServiceMock.On("IsOrganizationUnitExists", mock.Anything, testOUID1).
		Return(true, (*tidcommon.ServiceError)(nil)).Once()
	storeMock.On("GetEntityTypeByID", mock.Anything, TypeCategoryAgent, "schema-id").
		Return(EntityType{ID: "schema-id", Handle: DefaultAgentTypeHandle, DisplayName: "Default",
			OUID: testOUID1}, nil).Once()
	storeMock.On("UpdateEntityTypeByID", mock.Anything, TypeCategoryAgent, "schema-id",
		mock.MatchedBy(func(e EntityType) bool {
			return e.Handle == DefaultAgentTypeHandle && e.DisplayName == "Assistants"
		})).Return(nil).Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		ouService:       ouServiceMock,
		transactioner:   &mockTransactioner{},
		authzService:    newAllowAllAuthz(t),
	}

	result, svcErr := service.UpdateEntityType(context.Background(), TypeCategoryAgent, "schema-id",
		UpdateEntityTypeRequest{
			DisplayName: "Assistants",
			OUID:        testOUID1,
			Schema:      json.RawMessage(`{"name":{"type":"string"}}`),
		})

	require.Nil(t, svcErr)
	require.Equal(t, DefaultAgentTypeHandle, result.Handle)
}

func TestDeleteEntityType_AgentTypeAlwaysRejected(t *testing.T) {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{Enabled: false},
	}
	config.ResetServerRuntime()
	require.NoError(t, config.InitializeServerRuntime("/tmp/test", testConfig))
	defer config.ResetServerRuntime()

	service := &entityTypeService{
		entityTypeStore: newEntityTypeStoreInterfaceMock(t),
		authzService:    newAllowAllAuthz(t),
	}

	svcErr := service.DeleteEntityType(context.Background(), TypeCategoryAgent, "schema-1")
	require.NotNil(t, svcErr)
	require.Equal(t, ErrorAgentTypeCannotDelete.Code, svcErr.Code)
}

func TestValidateDisplayAttribute_NilSystemAttributes(t *testing.T) {
	compiled, err := model.CompileSchema(json.RawMessage(`{"email":{"type":"string"}}`))
	require.NoError(t, err)

	svcErr := validateDisplayAttribute(compiled, "")
	require.Nil(t, svcErr)
}

func TestValidateDisplayAttribute_EmptyDisplay(t *testing.T) {
	compiled, err := model.CompileSchema(json.RawMessage(`{"email":{"type":"string"}}`))
	require.NoError(t, err)

	svcErr := validateDisplayAttribute(compiled, "")
	require.Nil(t, svcErr)
}

func TestValidateDisplayAttribute_ValidStringAttribute(t *testing.T) {
	compiled, err := model.CompileSchema(json.RawMessage(`{
		"email":{"type":"string"},
		"password":{"type":"string","credential":true}
	}`))
	require.NoError(t, err)

	svcErr := validateDisplayAttribute(compiled, "email")
	require.Nil(t, svcErr)
}

func TestValidateDisplayAttribute_ValidNumberAttribute(t *testing.T) {
	compiled, err := model.CompileSchema(json.RawMessage(`{
		"email":{"type":"string"},
		"age":{"type":"number"}
	}`))
	require.NoError(t, err)

	svcErr := validateDisplayAttribute(compiled, "age")
	require.Nil(t, svcErr)
}

func TestValidateDisplayAttribute_BooleanAttributeRejected(t *testing.T) {
	compiled, err := model.CompileSchema(json.RawMessage(`{
		"email":{"type":"string"},
		"active":{"type":"boolean"}
	}`))
	require.NoError(t, err)

	svcErr := validateDisplayAttribute(compiled, "active")
	require.NotNil(t, svcErr)
	require.Equal(t, ErrorNonDisplayableAttribute.Code, svcErr.Code)
}

func TestValidateDisplayAttribute_ObjectAttributeRejected(t *testing.T) {
	compiled, err := model.CompileSchema(json.RawMessage(`{
		"address":{"type":"object","properties":{"city":{"type":"string"}}}
	}`))
	require.NoError(t, err)

	svcErr := validateDisplayAttribute(compiled, "address")
	require.NotNil(t, svcErr)
	require.Equal(t, ErrorNonDisplayableAttribute.Code, svcErr.Code)
}

func TestValidateDisplayAttribute_ArrayAttributeRejected(t *testing.T) {
	compiled, err := model.CompileSchema(json.RawMessage(`{
		"tags":{"type":"array","items":{"type":"string"}}
	}`))
	require.NoError(t, err)

	svcErr := validateDisplayAttribute(compiled, "tags")
	require.NotNil(t, svcErr)
	require.Equal(t, ErrorNonDisplayableAttribute.Code, svcErr.Code)
}

func TestValidateDisplayAttribute_CredentialAttributeRejected(t *testing.T) {
	compiled, err := model.CompileSchema(json.RawMessage(`{
		"password":{"type":"string","credential":true}
	}`))
	require.NoError(t, err)

	svcErr := validateDisplayAttribute(compiled, "password")
	require.NotNil(t, svcErr)
	require.Equal(t, ErrorCredentialDisplayAttribute.Code, svcErr.Code)
}

func TestValidateDisplayAttribute_NonExistentAttribute(t *testing.T) {
	compiled, err := model.CompileSchema(json.RawMessage(`{
		"email":{"type":"string"}
	}`))
	require.NoError(t, err)

	svcErr := validateDisplayAttribute(compiled, "username")
	require.NotNil(t, svcErr)
	require.Equal(t, ErrorInvalidDisplayAttribute.Code, svcErr.Code)
}

func TestCreateEntityTypeReturnsErrorForInvalidDisplayAttribute(t *testing.T) {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(t, err)
	defer config.ResetServerRuntime()

	storeMock := newEntityTypeStoreInterfaceMock(t)

	service := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
	}

	request := CreateEntityTypeRequestWithID{
		Handle:           "test-schema",
		DisplayName:      "test-schema",
		OUID:             testOUID1,
		Schema:           json.RawMessage(`{"email":{"type":"string"}}`),
		SystemAttributes: &SystemAttributes{Display: "nonexistent"},
	}

	createdSchema, svcErr := service.CreateEntityType(context.Background(), TypeCategoryUser, request)

	require.Nil(t, createdSchema)
	require.NotNil(t, svcErr)
	require.Equal(t, ErrorInvalidDisplayAttribute.Code, svcErr.Code)
}

func TestUpdateEntityTypeReturnsErrorForInvalidDisplayAttribute(t *testing.T) {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(t, err)
	defer config.ResetServerRuntime()

	storeMock := newEntityTypeStoreInterfaceMock(t)
	storeMock.On("IsEntityTypeDeclarative", TypeCategoryUser, "schema-id").Return(false).Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
	}

	request := UpdateEntityTypeRequest{
		Handle:           "test-schema",
		DisplayName:      "test-schema",
		OUID:             testOUID1,
		Schema:           json.RawMessage(`{"email":{"type":"string"}}`),
		SystemAttributes: &SystemAttributes{Display: "nonexistent"},
	}

	updatedSchema, svcErr := service.UpdateEntityType(context.Background(), TypeCategoryUser, "schema-id", request)

	require.Nil(t, updatedSchema)
	require.NotNil(t, svcErr)
	require.Equal(t, ErrorInvalidDisplayAttribute.Code, svcErr.Code)
}

func TestValidateDisplayAttribute_DottedPath_ValidNestedString(t *testing.T) {
	compiled, err := model.CompileSchema(json.RawMessage(`{
		"address":{"type":"object","properties":{"city":{"type":"string"}}}
	}`))
	require.NoError(t, err)

	svcErr := validateDisplayAttribute(compiled, "address.city")
	require.Nil(t, svcErr)
}

func TestValidateDisplayAttribute_DottedPath_NestedObjectRejected(t *testing.T) {
	compiled, err := model.CompileSchema(json.RawMessage(`{
		"profile":{"type":"object","properties":{
			"address":{"type":"object","properties":{"city":{"type":"string"}}}
		}}
	}`))
	require.NoError(t, err)

	svcErr := validateDisplayAttribute(compiled, "profile.address")
	require.NotNil(t, svcErr)
	require.Equal(t, ErrorNonDisplayableAttribute.Code, svcErr.Code)
}

func TestValidateDisplayAttribute_DottedPath_NestedCredentialRejected(t *testing.T) {
	compiled, err := model.CompileSchema(json.RawMessage(`{
		"auth":{"type":"object","properties":{
			"password":{"type":"string","credential":true}
		}}
	}`))
	require.NoError(t, err)

	svcErr := validateDisplayAttribute(compiled, "auth.password")
	require.NotNil(t, svcErr)
	require.Equal(t, ErrorCredentialDisplayAttribute.Code, svcErr.Code)
}

func TestValidateDisplayAttribute_DottedPath_NonExistentNested(t *testing.T) {
	compiled, err := model.CompileSchema(json.RawMessage(`{
		"address":{"type":"object","properties":{"city":{"type":"string"}}}
	}`))
	require.NoError(t, err)

	svcErr := validateDisplayAttribute(compiled, "address.zip")
	require.NotNil(t, svcErr)
	require.Equal(t, ErrorInvalidDisplayAttribute.Code, svcErr.Code)
}

func TestValidateDisplayAttribute_DottedPath_TraverseIntoNonObject(t *testing.T) {
	compiled, err := model.CompileSchema(json.RawMessage(`{
		"email":{"type":"string"}
	}`))
	require.NoError(t, err)

	svcErr := validateDisplayAttribute(compiled, "email.domain")
	require.NotNil(t, svcErr)
	require.Equal(t, ErrorInvalidDisplayAttribute.Code, svcErr.Code)
}

func TestValidateDisplayAttribute_DottedPath_DeeplyNestedValid(t *testing.T) {
	compiled, err := model.CompileSchema(json.RawMessage(`{
		"profile":{"type":"object","properties":{
			"name":{"type":"object","properties":{
				"first":{"type":"string"}
			}}
		}}
	}`))
	require.NoError(t, err)

	svcErr := validateDisplayAttribute(compiled, "profile.name.first")
	require.Nil(t, svcErr)
}

// GetDisplayAttributesByHandles tests

func (s *EntityTypeServiceTestSuite) TestGetDisplayAttributesByHandles_ReturnsDisplayAttributes() {
	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	expected := map[string]string{"SchemaA": "email", "SchemaB": "given_name"}
	storeMock.
		On("GetDisplayAttributesByHandles", mock.Anything, TypeCategoryUser, []string{"SchemaA", "SchemaB"}).
		Return(expected, nil).
		Once()

	service := &entityTypeService{entityTypeStore: storeMock}

	result, svcErr := service.GetDisplayAttributesByHandles(
		context.Background(), TypeCategoryUser, []string{"SchemaA", "SchemaB"})

	s.Require().Nil(svcErr)
	s.Require().Equal(expected, result)
}

func (s *EntityTypeServiceTestSuite) TestGetDisplayAttributesByHandles_TestEmptyInput_ReturnsEmptyMap() {
	service := &entityTypeService{}

	result, svcErr := service.GetDisplayAttributesByHandles(context.Background(), TypeCategoryUser, []string{})

	s.Require().Nil(svcErr)
	s.Require().Empty(result)
}

func (s *EntityTypeServiceTestSuite) TestGetDisplayAttributesByHandles_TestStoreError_ReturnsServerError() {
	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	storeMock.
		On("GetDisplayAttributesByHandles", mock.Anything, TypeCategoryUser, []string{"SchemaA"}).
		Return(map[string]string(nil), errors.New("db error")).
		Once()

	service := &entityTypeService{entityTypeStore: storeMock}

	_, svcErr := service.GetDisplayAttributesByHandles(context.Background(), TypeCategoryUser, []string{"SchemaA"})

	s.Require().NotNil(svcErr)
	s.Require().Equal(tidcommon.InternalServerError, *svcErr)
}

func (s *EntityTypeServiceTestSuite) TestGetAttributes_NonCredentialRequiredOnly_ReturnsAttributes() {
	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	storeMock.
		On("GetEntityTypeByHandle", context.Background(), TypeCategoryUser, "internal").
		Return(EntityType{
			Schema: json.RawMessage(
				`{"email":{"type":"string","required":true},` +
					`"firstName":{"type":"string","required":true,"displayName":"First Name"},` +
					`"password":{"type":"string","required":true,"credential":true},` +
					`"age":{"type":"number"}}`,
			),
		}, nil).
		Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
	}

	attrs, svcErr := service.GetAttributes(context.Background(), TypeCategoryUser, "internal",
		AttributeFilter{AllowNonCredential: true, RequiredOnly: true})

	s.Require().Nil(svcErr)
	s.Require().Len(attrs, 2)

	attrMap := make(map[string]AttributeInfo, len(attrs))
	for _, a := range attrs {
		attrMap[a.Attribute] = a
	}

	email, ok := attrMap["email"]
	s.Require().True(ok, "email should be returned")
	s.Equal("", email.DisplayName)

	firstName, ok := attrMap["firstName"]
	s.Require().True(ok, "firstName should be returned")
	s.Equal("First Name", firstName.DisplayName)

	_, hasPassword := attrMap["password"]
	s.False(hasPassword, "password is credential and must be excluded")
}

func (s *EntityTypeServiceTestSuite) TestGetAttributes_NonCredential_UnknownEntityType_ReturnsError() {
	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	storeMock.
		On("GetEntityTypeByHandle", context.Background(), TypeCategoryUser, "unknown").
		Return(EntityType{}, ErrEntityTypeNotFound).
		Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
	}

	attrs, svcErr := service.GetAttributes(context.Background(), TypeCategoryUser, "unknown",
		AttributeFilter{AllowNonCredential: true, RequiredOnly: true})

	s.Require().NotNil(svcErr)
	s.Require().Equal(ErrorEntityTypeNotFound.Code, svcErr.Code)
	s.Require().Nil(attrs)
}

func (s *EntityTypeServiceTestSuite) TestGetAttributes_NonCredential_EmptyEntityType_ReturnsError() {
	service := &entityTypeService{
		entityTypeStore: newEntityTypeStoreInterfaceMock(s.T()),
		transactioner:   &mockTransactioner{},
	}

	attrs, svcErr := service.GetAttributes(context.Background(), TypeCategoryUser, "",
		AttributeFilter{AllowNonCredential: true, RequiredOnly: true})

	s.Require().NotNil(svcErr)
	s.Require().Equal(ErrorEntityTypeNotFound.Code, svcErr.Code)
	s.Require().Nil(attrs)
}

func (s *EntityTypeServiceTestSuite) TestGetAttributes_NonCredential_AllCredentials_ReturnsEmpty() {
	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	storeMock.
		On("GetEntityTypeByHandle", context.Background(), TypeCategoryUser, "internal").
		Return(EntityType{
			Schema: json.RawMessage(
				`{"password":{"type":"string","required":true,"credential":true}}`,
			),
		}, nil).
		Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
	}

	attrs, svcErr := service.GetAttributes(context.Background(), TypeCategoryUser, "internal",
		AttributeFilter{AllowNonCredential: true, RequiredOnly: true})

	s.Require().Nil(svcErr)
	s.Require().Empty(attrs)
}

func (s *EntityTypeServiceTestSuite) TestGetAttributes_NonCredentialAllAttrs_IncludesOptional() {
	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	storeMock.
		On("GetEntityTypeByHandle", context.Background(), TypeCategoryUser, "internal").
		Return(EntityType{
			Schema: json.RawMessage(
				`{"email":{"type":"string","required":true},` +
					`"mobile_number":{"type":"string"},` +
					`"password":{"type":"string","required":true,"credential":true}}`,
			),
		}, nil).
		Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
	}

	attrs, svcErr := service.GetAttributes(context.Background(), TypeCategoryUser, "internal",
		AttributeFilter{AllowNonCredential: true})

	s.Require().Nil(svcErr)
	s.Require().Len(attrs, 2, "email and mobile_number should be returned; password excluded as credential")

	attrMap := make(map[string]AttributeInfo, len(attrs))
	for _, a := range attrs {
		attrMap[a.Attribute] = a
	}
	s.True(attrMap["email"].Required)
	s.False(attrMap["mobile_number"].Required, "optional attribute must be included with Required=false")
	_, hasPassword := attrMap["password"]
	s.False(hasPassword, "credential must always be excluded")
}

func (s *EntityTypeServiceTestSuite) TestGetAttributes_NonCredential_StoreError_ReturnsServerError() {
	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	storeMock.
		On("GetEntityTypeByHandle", context.Background(), TypeCategoryUser, "internal").
		Return(EntityType{}, errors.New("db failure")).
		Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
	}

	attrs, svcErr := service.GetAttributes(context.Background(), TypeCategoryUser, "internal",
		AttributeFilter{AllowNonCredential: true})

	s.Require().NotNil(svcErr)
	s.Require().Equal(tidcommon.InternalServerError, *svcErr)
	s.Require().Nil(attrs)
}

func (s *EntityTypeServiceTestSuite) TestGetEntityTypeSchema_Success() {
	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	storeMock.
		On("GetEntityTypeByHandle", context.Background(), TypeCategoryUser, "employee").
		Return(EntityType{Handle: "employee", DisplayName: "employee", Schema: json.RawMessage(`{}`)}, nil).
		Once()

	service := &entityTypeService{entityTypeStore: storeMock}

	entityType, svcErr := service.GetEntityTypeSchema(context.Background(), TypeCategoryUser, "employee")
	s.Require().Nil(svcErr)
	s.Require().NotNil(entityType)
	s.Require().Equal("employee", entityType.Handle)
}

func (s *EntityTypeServiceTestSuite) TestGetEntityTypeSchema_InvalidCategory() {
	service := &entityTypeService{}
	entityType, svcErr := service.GetEntityTypeSchema(context.Background(), TypeCategory("invalid"), "employee")
	s.Require().Nil(entityType)
	s.Require().NotNil(svcErr)
	s.Require().Equal(ErrorInvalidEntityTypeRequest.Code, svcErr.Code)
}

func (s *EntityTypeServiceTestSuite) TestGetEntityTypeSchema_EmptyHandle() {
	service := &entityTypeService{}
	entityType, svcErr := service.GetEntityTypeSchema(context.Background(), TypeCategoryUser, "")
	s.Require().Nil(entityType)
	s.Require().NotNil(svcErr)
	s.Require().Equal(ErrorInvalidEntityTypeRequest.Code, svcErr.Code)
}

func (s *EntityTypeServiceTestSuite) TestGetEntityTypeSchema_NotFound() {
	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	storeMock.
		On("GetEntityTypeByHandle", context.Background(), TypeCategoryUser, "employee").
		Return(EntityType{}, ErrEntityTypeNotFound).
		Once()

	service := &entityTypeService{entityTypeStore: storeMock}

	entityType, svcErr := service.GetEntityTypeSchema(context.Background(), TypeCategoryUser, "employee")
	s.Require().Nil(entityType)
	s.Require().NotNil(svcErr)
	s.Require().Equal(ErrorEntityTypeNotFound.Code, svcErr.Code)
}

func (s *EntityTypeServiceTestSuite) TestGetEntityTypeSchema_StoreError() {
	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	storeMock.
		On("GetEntityTypeByHandle", context.Background(), TypeCategoryUser, "employee").
		Return(EntityType{}, errors.New("db error")).
		Once()

	service := &entityTypeService{entityTypeStore: storeMock}

	entityType, svcErr := service.GetEntityTypeSchema(context.Background(), TypeCategoryUser, "employee")
	s.Require().Nil(entityType)
	s.Require().NotNil(svcErr)
	s.Require().Equal(tidcommon.InternalServerError, *svcErr)
}

// TestGetCompiledSchemaForEntityType_CompileError verifies that a stored schema which fails to
// compile surfaces as an internal server error through ValidateEntity.
func TestGetCompiledSchemaForEntityType_CompileError(t *testing.T) {
	storeMock := newEntityTypeStoreInterfaceMock(t)
	storeMock.
		On("GetEntityTypeByHandle", context.Background(), TypeCategoryUser, "employee").
		Return(EntityType{
			Handle:      "employee",
			DisplayName: "employee",
			Schema:      json.RawMessage(`{"email":{"type":"banana"}}`),
		}, nil).
		Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
	}

	ok, svcErr := service.ValidateEntity(
		context.Background(), TypeCategoryUser, "employee", json.RawMessage(`{}`), false)

	require.False(t, ok)
	require.NotNil(t, svcErr)
	require.Equal(t, tidcommon.InternalServerError, *svcErr)
}

// TestEnsureOrganizationUnitExists_NilOUService verifies that a missing OU service yields an
// internal server error.
func TestEnsureOrganizationUnitExists_NilOUService(t *testing.T) {
	service := &entityTypeService{ouService: nil}

	svcErr := service.ensureOrganizationUnitExists(
		context.Background(), testOUID1, TypeCategoryUser, log.GetLogger())

	require.NotNil(t, svcErr)
	require.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)
}

// TestPopulateEntityTypeOUHandles_HandleResolutionError verifies that populateEntityTypeOUHandles
// returns early without setting handles when the OU service fails.
func TestPopulateEntityTypeOUHandles_HandleResolutionError(t *testing.T) {
	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	ouServiceMock.On("GetOrganizationUnitHandlesByIDs", mock.Anything, []string{testOUID1}).
		Return(map[string]string(nil), &tidcommon.InternalServerError).Once()

	service := &entityTypeService{ouService: ouServiceMock}
	schemas := []EntityTypeListItem{{ID: "s1", Handle: "schema1", DisplayName: "Schema1", OUID: testOUID1}}

	service.populateEntityTypeOUHandles(context.Background(), schemas, log.GetLogger())
	require.Empty(t, schemas[0].OUHandle)
}

func initEntityTypeTestRuntime(t *testing.T) {
	t.Helper()
	config.ResetServerRuntime()
	require.NoError(t, config.InitializeServerRuntime("/tmp/test", &config.Config{
		DeclarativeResources: config.DeclarativeResources{Enabled: false},
	}))
	t.Cleanup(config.ResetServerRuntime)
}

func TestCreateEntityType_HandleValidation(t *testing.T) {
	maxLenHandle := strings.Repeat("a", 100)

	testCases := []struct {
		name    string
		handle  string
		isValid bool
	}{
		{"lowercase letters", "employee", true},
		{"single character", "e", true},
		{"digits only", "123", true},
		{"hyphens and underscores", "my-type_1", true},
		{"max length", maxLenHandle, true},
		{"empty", "", false},
		{"uppercase", "Employee", false},
		{"space", "my type", false},
		{"leading hyphen", "-employee", false},
		{"trailing hyphen", "employee-", false},
		{"leading underscore", "_employee", false},
		{"trailing underscore", "employee_", false},
		{"dot", "my.type", false},
		{"unicode", "employé", false},
		{"too long", maxLenHandle + "a", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			initEntityTypeTestRuntime(t)

			storeMock := newEntityTypeStoreInterfaceMock(t)
			ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
			service := &entityTypeService{
				entityTypeStore: storeMock,
				ouService:       ouServiceMock,
				transactioner:   &mockTransactioner{},
				authzService:    newAllowAllAuthz(t),
			}

			if tc.isValid {
				ouServiceMock.On("IsOrganizationUnitExists", mock.Anything, testOUID1).
					Return(true, (*tidcommon.ServiceError)(nil)).Once()
				storeMock.On("GetEntityTypeByHandle", mock.Anything, TypeCategoryUser, tc.handle).
					Return(EntityType{}, ErrEntityTypeNotFound).Once()
				storeMock.On("CreateEntityType", mock.Anything, mock.Anything).Return(nil).Once()
			}

			result, svcErr := service.CreateEntityType(context.Background(), TypeCategoryUser,
				CreateEntityTypeRequestWithID{
					Handle:      tc.handle,
					DisplayName: "Employee",
					OUID:        testOUID1,
					Schema:      json.RawMessage(`{"email":{"type":"string"}}`),
				})

			if tc.isValid {
				require.Nil(t, svcErr)
				require.NotNil(t, result)
				require.Equal(t, tc.handle, result.Handle)
				require.Equal(t, "Employee", result.DisplayName)
			} else {
				require.Nil(t, result)
				require.NotNil(t, svcErr)
				require.Equal(t, "USRS-1016", svcErr.Code)
				require.Equal(t, ErrorInvalidEntityTypeHandle.Code, svcErr.Code)
			}
		})
	}
}

func TestCreateEntityType_RejectsEmptyDisplayName(t *testing.T) {
	for _, displayName := range []string{""} {
		t.Run("display name '"+displayName+"'", func(t *testing.T) {
			initEntityTypeTestRuntime(t)

			service := &entityTypeService{
				entityTypeStore: newEntityTypeStoreInterfaceMock(t),
				transactioner:   &mockTransactioner{},
				authzService:    newAllowAllAuthz(t),
			}

			result, svcErr := service.CreateEntityType(context.Background(), TypeCategoryUser,
				CreateEntityTypeRequestWithID{
					Handle:      "employee",
					DisplayName: displayName,
					OUID:        testOUID1,
					Schema:      json.RawMessage(`{"email":{"type":"string"}}`),
				})

			require.Nil(t, result)
			require.NotNil(t, svcErr)
			require.Equal(t, ErrorInvalidUserTypeRequest.Code, svcErr.Code)
			require.Contains(t, svcErr.ErrorDescription.DefaultValue, "display name must not be empty")
		})
	}
}

func TestCreateEntityType_HandleConflictWithinCategory(t *testing.T) {
	for _, category := range []TypeCategory{TypeCategoryUser, TypeCategoryAgent} {
		t.Run(string(category), func(t *testing.T) {
			initEntityTypeTestRuntime(t)

			storeMock := newEntityTypeStoreInterfaceMock(t)
			ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
			ouServiceMock.On("IsOrganizationUnitExists", mock.Anything, testOUID1).
				Return(true, (*tidcommon.ServiceError)(nil)).Once()
			storeMock.On("GetEntityTypeByHandle", mock.Anything, category, "default").
				Return(EntityType{ID: "existing-id", Handle: "default", OUID: testOUID1}, nil).Once()

			service := &entityTypeService{
				entityTypeStore: storeMock,
				ouService:       ouServiceMock,
				transactioner:   &mockTransactioner{},
				authzService:    newAllowAllAuthz(t),
			}

			result, svcErr := service.CreateEntityType(context.Background(), category,
				CreateEntityTypeRequestWithID{
					Handle:      "default",
					DisplayName: "Default",
					OUID:        testOUID1,
					Schema:      json.RawMessage(`{"email":{"type":"string"}}`),
				})

			require.Nil(t, result)
			require.NotNil(t, svcErr)
			require.Equal(t, entityTypeHandleConflictErr(category).Error.Key, svcErr.Error.Key)
			storeMock.AssertNotCalled(t, "CreateEntityType", mock.Anything, mock.Anything)
		})
	}
}

func TestCreateEntityType_SameHandleInOtherCategoryAllowed(t *testing.T) {
	initEntityTypeTestRuntime(t)

	storeMock := newEntityTypeStoreInterfaceMock(t)
	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	ouServiceMock.On("IsOrganizationUnitExists", mock.Anything, testOUID1).
		Return(true, (*tidcommon.ServiceError)(nil)).Once()
	storeMock.On("GetEntityTypeByHandle", mock.Anything, TypeCategoryUser, "default").
		Return(EntityType{}, ErrEntityTypeNotFound).Once()
	storeMock.On("CreateEntityType", mock.Anything, mock.Anything).Return(nil).Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		ouService:       ouServiceMock,
		transactioner:   &mockTransactioner{},
		authzService:    newAllowAllAuthz(t),
	}

	result, svcErr := service.CreateEntityType(context.Background(), TypeCategoryUser,
		CreateEntityTypeRequestWithID{
			Handle:      "default",
			DisplayName: "Default",
			OUID:        testOUID1,
			Schema:      json.RawMessage(`{"email":{"type":"string"}}`),
		})

	require.Nil(t, svcErr)
	require.Equal(t, "default", result.Handle)
	storeMock.AssertNotCalled(t, "GetEntityTypeByHandle", mock.Anything, TypeCategoryAgent, mock.Anything)
}

func TestCreateEntityType_HandleLookupStoreError(t *testing.T) {
	initEntityTypeTestRuntime(t)

	storeMock := newEntityTypeStoreInterfaceMock(t)
	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	ouServiceMock.On("IsOrganizationUnitExists", mock.Anything, testOUID1).
		Return(true, (*tidcommon.ServiceError)(nil)).Once()
	storeMock.On("GetEntityTypeByHandle", mock.Anything, TypeCategoryUser, "employee").
		Return(EntityType{}, errors.New("db failure")).Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		ouService:       ouServiceMock,
		transactioner:   &mockTransactioner{},
		authzService:    newAllowAllAuthz(t),
	}

	result, svcErr := service.CreateEntityType(context.Background(), TypeCategoryUser,
		CreateEntityTypeRequestWithID{
			Handle:      "employee",
			DisplayName: "Employee",
			OUID:        testOUID1,
			Schema:      json.RawMessage(`{"email":{"type":"string"}}`),
		})

	require.Nil(t, result)
	require.NotNil(t, svcErr)
	require.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)
}

func TestUpdateEntityType_HandleChangeRejected(t *testing.T) {
	initEntityTypeTestRuntime(t)

	storeMock := newEntityTypeStoreInterfaceMock(t)
	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	storeMock.On("IsEntityTypeDeclarative", TypeCategoryUser, "schema-id").Return(false).Once()
	ouServiceMock.On("IsOrganizationUnitExists", mock.Anything, testOUID1).
		Return(true, (*tidcommon.ServiceError)(nil)).Once()
	storeMock.On("GetEntityTypeByID", mock.Anything, TypeCategoryUser, "schema-id").
		Return(EntityType{ID: "schema-id", Handle: "employee", DisplayName: "Employee", OUID: testOUID1}, nil).Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		ouService:       ouServiceMock,
		transactioner:   &mockTransactioner{},
		authzService:    newAllowAllAuthz(t),
	}

	result, svcErr := service.UpdateEntityType(context.Background(), TypeCategoryUser, "schema-id",
		UpdateEntityTypeRequest{
			Handle:      "staff",
			DisplayName: "Employee",
			OUID:        testOUID1,
			Schema:      json.RawMessage(`{"email":{"type":"string"}}`),
		})

	require.Nil(t, result)
	require.NotNil(t, svcErr)
	require.Equal(t, "USRS-1017", svcErr.Code)
	require.Equal(t, ErrorEntityTypeHandleUpdateNotAllowed.Code, svcErr.Code)
	storeMock.AssertNotCalled(t, "UpdateEntityTypeByID", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestUpdateEntityType_OmittedHandleKeepsExisting(t *testing.T) {
	initEntityTypeTestRuntime(t)

	storeMock := newEntityTypeStoreInterfaceMock(t)
	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	storeMock.On("IsEntityTypeDeclarative", TypeCategoryUser, "schema-id").Return(false).Once()
	ouServiceMock.On("IsOrganizationUnitExists", mock.Anything, testOUID1).
		Return(true, (*tidcommon.ServiceError)(nil)).Once()
	storeMock.On("GetEntityTypeByID", mock.Anything, TypeCategoryUser, "schema-id").
		Return(EntityType{ID: "schema-id", Handle: "employee", DisplayName: "Employee", OUID: testOUID1}, nil).Once()
	storeMock.On("UpdateEntityTypeByID", mock.Anything, TypeCategoryUser, "schema-id",
		mock.MatchedBy(func(e EntityType) bool {
			return e.Handle == "employee" && e.DisplayName == "Staff"
		})).Return(nil).Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		ouService:       ouServiceMock,
		transactioner:   &mockTransactioner{},
		authzService:    newAllowAllAuthz(t),
	}

	result, svcErr := service.UpdateEntityType(context.Background(), TypeCategoryUser, "schema-id",
		UpdateEntityTypeRequest{
			DisplayName: "Staff",
			OUID:        testOUID1,
			Schema:      json.RawMessage(`{"email":{"type":"string"}}`),
		})

	require.Nil(t, svcErr)
	require.Equal(t, "employee", result.Handle)
}

func TestUpdateEntityType_RejectsEmptyDisplayName(t *testing.T) {
	initEntityTypeTestRuntime(t)

	storeMock := newEntityTypeStoreInterfaceMock(t)
	storeMock.On("IsEntityTypeDeclarative", TypeCategoryUser, "schema-id").Return(false).Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
		authzService:    newAllowAllAuthz(t),
	}

	result, svcErr := service.UpdateEntityType(context.Background(), TypeCategoryUser, "schema-id",
		UpdateEntityTypeRequest{
			Handle:      "employee",
			DisplayName: "",
			OUID:        testOUID1,
			Schema:      json.RawMessage(`{"email":{"type":"string"}}`),
		})

	require.Nil(t, result)
	require.NotNil(t, svcErr)
	require.Equal(t, ErrorInvalidUserTypeRequest.Code, svcErr.Code)
	require.Contains(t, svcErr.ErrorDescription.DefaultValue, "display name must not be empty")
}

func TestUpdateEntityType_DisplayNameChangeSkipsHandleLookup(t *testing.T) {
	initEntityTypeTestRuntime(t)

	storeMock := newEntityTypeStoreInterfaceMock(t)
	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	storeMock.On("IsEntityTypeDeclarative", TypeCategoryUser, "schema-id").Return(false).Once()
	ouServiceMock.On("IsOrganizationUnitExists", mock.Anything, testOUID1).
		Return(true, (*tidcommon.ServiceError)(nil)).Once()
	storeMock.On("GetEntityTypeByID", mock.Anything, TypeCategoryUser, "schema-id").
		Return(EntityType{ID: "schema-id", Handle: "employee", DisplayName: "Employee", OUID: testOUID1}, nil).Once()
	storeMock.On("UpdateEntityTypeByID", mock.Anything, TypeCategoryUser, "schema-id",
		mock.MatchedBy(func(e EntityType) bool {
			return e.Handle == "employee" && e.DisplayName == "Staff Member" && e.Category == TypeCategoryUser
		})).Return(nil).Once()

	service := &entityTypeService{
		entityTypeStore: storeMock,
		ouService:       ouServiceMock,
		transactioner:   &mockTransactioner{},
		authzService:    newAllowAllAuthz(t),
	}

	result, svcErr := service.UpdateEntityType(context.Background(), TypeCategoryUser, "schema-id",
		UpdateEntityTypeRequest{
			Handle:      "employee",
			DisplayName: "Staff Member",
			OUID:        testOUID1,
			Schema:      json.RawMessage(`{"email":{"type":"string"}}`),
		})

	require.Nil(t, svcErr)
	require.NotNil(t, result)
	require.Equal(t, "employee", result.Handle)
	require.Equal(t, "Staff Member", result.DisplayName)
	storeMock.AssertNotCalled(t, "GetEntityTypeByHandle", mock.Anything, mock.Anything, mock.Anything)
}

func TestCreateEntityType_InvalidHandleRejected(t *testing.T) {
	initEntityTypeTestRuntime(t)

	service := &entityTypeService{
		entityTypeStore: newEntityTypeStoreInterfaceMock(t),
		transactioner:   &mockTransactioner{},
		authzService:    newAllowAllAuthz(t),
	}

	_, svcErr := service.CreateEntityType(context.Background(), TypeCategoryUser, CreateEntityTypeRequestWithID{
		Handle:      "Bad Handle",
		DisplayName: "Employee",
		OUID:        testOUID1,
		Schema:      json.RawMessage(`{"email":{"type":"string"}}`),
	})

	require.NotNil(t, svcErr)
	require.Equal(t, ErrorInvalidEntityTypeHandle.Code, svcErr.Code)
}
