// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/entitytype"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/export"
	"github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
)

// recordingCapturer records every resource handed to it.
type recordingCapturer struct {
	resourceTypes []string
	resources     []interface{}
}

func (r *recordingCapturer) CaptureValues(_ context.Context, resourceType string, resource interface{}) {
	r.resourceTypes = append(r.resourceTypes, resourceType)
	r.resources = append(r.resources, resource)
}

// captured returns the one user handed to the capturer, in the shape its exporter reads back.
func (r *recordingCapturer) captured(t *testing.T) *userDeclarativeResource {
	t.Helper()
	require.Len(t, r.resources, 1)
	assert.Equal(t, resourceTypeUser, r.resourceTypes[0])
	user, ok := r.resources[0].(*userDeclarativeResource)
	require.True(t, ok, "the capture was not handed the shape the exporter reads back")
	return user
}

func declaresPassword(t *testing.T) *entitytypemock.EntityTypeServiceInterfaceMock {
	t.Helper()
	entityTypes := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	entityTypes.On("GetEntityTypeByHandle", mock.Anything, mock.Anything, testUserType).
		Return(&entitytype.EntityType{OUID: testOrgID}, (*tidcommon.ServiceError)(nil)).Maybe()
	entityTypes.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, testUserType,
		entitytype.AttributeFilter{AllowCredential: true}).
		Return([]entitytype.AttributeInfo{{Attribute: "password", Credential: true}},
			(*tidcommon.ServiceError)(nil)).Maybe()
	return entityTypes
}

func newCapturingUserService(t *testing.T, capturer declarativeresource.ValueCapturer,
	entities *entitymock.EntityServiceInterfaceMock) *userService {
	t.Helper()
	ouService := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	ouService.On("IsOrganizationUnitExists", mock.Anything, testOrgID).
		Return(true, (*tidcommon.ServiceError)(nil)).Maybe()
	return &userService{
		entityService:     entities,
		ouService:         ouService,
		entityTypeService: declaresPassword(t),
		authzService:      newAllowAllAuthz(t),
		uuidGenerator:     utils.GenerateUUIDv7,
		valueCapturer:     capturer,
	}
}

// A created user is captured with the credentials it was created with, which creation strips from
// what it returns.
func TestCreateUser_CapturesTheSubmittedCredentials(t *testing.T) {
	entities := entitymock.NewEntityServiceInterfaceMock(t)
	entities.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything).
		Return(&providers.Entity{Attributes: json.RawMessage(`{"username":"alice","email":"a@x.test"}`)}, nil)
	capturer := &recordingCapturer{}
	service := newCapturingUserService(t, capturer, entities)

	created, svcErr := service.CreateUser(context.Background(), &providers.User{
		Type: testUserType, OUID: testOrgID,
		Attributes: json.RawMessage(`{"username":"alice","email":"a@x.test","password":"s3cret"}`),
	})
	require.Nil(t, svcErr)

	user := capturer.captured(t)
	assert.Equal(t, created.ID, user.ID)
	assert.Equal(t, map[string]interface{}{"username": "alice", "email": "a@x.test"}, user.Attributes)
	assert.Equal(t, map[string]interface{}{"password": "s3cret"}, user.Credentials)
}

// A user created without a credential has nothing to capture.
func TestCreateUser_WithoutCredentialsCapturesNothing(t *testing.T) {
	entities := entitymock.NewEntityServiceInterfaceMock(t)
	entities.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything).
		Return(&providers.Entity{Attributes: json.RawMessage(`{"username":"alice"}`)}, nil)
	capturer := &recordingCapturer{}
	service := newCapturingUserService(t, capturer, entities)

	_, svcErr := service.CreateUser(context.Background(), &providers.User{
		Type: testUserType, OUID: testOrgID, Attributes: json.RawMessage(`{"username":"alice"}`),
	})
	require.Nil(t, svcErr)

	assert.Empty(t, capturer.resources)
}

// Credentials are named after the username, so a user without one has nothing captured.
func TestCreateUser_WithoutAUsernameCapturesNothing(t *testing.T) {
	entities := entitymock.NewEntityServiceInterfaceMock(t)
	entities.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything).
		Return(&providers.Entity{Attributes: json.RawMessage(`{"email":"a@x.test"}`)}, nil)
	capturer := &recordingCapturer{}
	service := newCapturingUserService(t, capturer, entities)

	_, svcErr := service.CreateUser(context.Background(), &providers.User{
		Type: testUserType, OUID: testOrgID,
		Attributes: json.RawMessage(`{"email":"a@x.test","password":"s3cret"}`),
	})
	require.Nil(t, svcErr)

	assert.Empty(t, capturer.resources)
}

// A rotated credential is captured, so the store does not keep serving the one from creation.
func TestUpdateUserCredentials_CapturesTheNewCredential(t *testing.T) {
	entities := entitymock.NewEntityServiceInterfaceMock(t)
	entities.On("IsEntityDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()
	entities.On("GetEntity", mock.Anything, svcTestUserID1).Return(&providers.Entity{
		Category: providers.EntityCategoryUser, ID: svcTestUserID1, Type: testUserType, OUID: testOrgID,
		Attributes: json.RawMessage(`{"username":"alice"}`),
	}, nil)
	entities.On("UpdateCredentials", mock.Anything, svcTestUserID1, mock.Anything).Return(nil)
	capturer := &recordingCapturer{}
	service := newCapturingUserService(t, capturer, entities)

	svcErr := service.UpdateUserCredentials(context.Background(), svcTestUserID1,
		json.RawMessage(`{"password":"rotated"}`))
	require.Nil(t, svcErr)

	user := capturer.captured(t)
	assert.Equal(t, svcTestUserID1, user.ID)
	assert.Equal(t, map[string]interface{}{"username": "alice"}, user.Attributes)
	assert.Equal(t, map[string]interface{}{"password": "rotated"}, user.Credentials)
}

// Without a capturer, which is every plane but a control plane, nothing is captured and the user
// type is not asked which attributes are credentials.
func TestCaptureWithoutACapturerDoesNothing(t *testing.T) {
	service := &userService{}

	assert.NotPanics(t, func() {
		service.captureSubmittedCredentials(context.Background(), &providers.User{Type: testUserType},
			json.RawMessage(`{"username":"alice","password":"s3cret"}`))
	})
}

// The name a user's credential is captured under is the one its export writes the placeholder with.
func TestACapturedCredentialIsNamedAsTheExportNamesIt(t *testing.T) {
	c := newCredentialExport(t, "alice")
	c.typeDeclaresCredentials("password")
	c.userHasCredential("password", true)
	exported := c.exportUser(t)

	capturer := &recordingCapturer{}
	(&userService{valueCapturer: capturer}).captureCredentials(context.Background(),
		&providers.User{ID: "user-1", Type: "person"}, json.RawMessage(`{"username":"alice"}`),
		map[string]string{"password": "s3cret"})
	values := export.Initialize(http.NewServeMux(), []declarativeresource.ResourceExporter{c.exporter},
		export.ValueReferences)

	variables, secrets, err := values.PlaceholderValues(context.Background(), resourceTypeUser,
		capturer.captured(t))
	require.NoError(t, err)

	assert.Empty(t, variables)
	require.Len(t, secrets, 1)
	for name, value := range secrets {
		assert.Equal(t, "{{."+name+"}}", exported.Credentials["password"])
		assert.Equal(t, "s3cret", value)
	}
}
