// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entitytype"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
)

// credentialExport wires a user exporter over mocks for one user of type "person" named alice.
type credentialExport struct {
	users       *UserServiceInterfaceMock
	entities    *entitymock.EntityServiceInterfaceMock
	entityTypes *entitytypemock.EntityTypeServiceInterfaceMock
	exporter    *userExporter
}

func newCredentialExport(t *testing.T, username string) *credentialExport {
	t.Helper()
	c := &credentialExport{
		users:       NewUserServiceInterfaceMock(t),
		entities:    entitymock.NewEntityServiceInterfaceMock(t),
		entityTypes: entitytypemock.NewEntityTypeServiceInterfaceMock(t),
	}
	c.exporter = newUserExporter(c.users, c.entities, c.entityTypes)

	attrs, err := json.Marshal(map[string]interface{}{"username": username})
	require.NoError(t, err)
	c.users.On("GetUser", context.Background(), "user-1", false).
		Return(&providers.User{ID: "user-1", Type: "person", OUID: "ou-1", Attributes: attrs}, nil)
	return c
}

// typeDeclaresCredentials sets the credential attributes the user's type declares.
func (c *credentialExport) typeDeclaresCredentials(attributes ...string) {
	infos := make([]entitytype.AttributeInfo, 0, len(attributes))
	for _, attribute := range attributes {
		infos = append(infos, entitytype.AttributeInfo{Attribute: attribute, Credential: true})
	}
	c.entityTypes.On("GetAttributes", context.Background(), entitytype.TypeCategoryUser, "person",
		entitytype.AttributeFilter{AllowCredential: true}).Return(infos, nil)
}

// userHasCredential sets whether the user has a stored credential for an attribute.
func (c *credentialExport) userHasCredential(attribute string, isSet bool) {
	var stored []entity.StoredCredential
	if isSet {
		stored = []entity.StoredCredential{{Value: "a-hash"}}
	}
	c.entities.On("GetCredentialsByType", context.Background(), "user-1", attribute).Return(stored, nil)
}

func (c *credentialExport) exportUser(t *testing.T) *userDeclarativeResource {
	t.Helper()
	resource, _, svcErr := c.exporter.GetResourceByID(context.Background(), "user-1")
	require.Nil(t, svcErr)
	user, ok := resource.(*userDeclarativeResource)
	require.True(t, ok)
	return user
}

// A user with a password exports it as a template variable named after the username. The value is a
// one-way hash and never leaves; the importing server fills the variable and hashes it on the way in.
func TestAUserWithAPasswordExportsIt(t *testing.T) {
	c := newCredentialExport(t, "alice")
	c.typeDeclaresCredentials("password")
	c.userHasCredential("password", true)

	user := c.exportUser(t)

	assert.Equal(t, map[string]interface{}{"password": "{{.USER_ALICE_PASSWORD}}"}, user.Credentials)
}

// A user who never set a password is still exported, with no credentials. Exporting one anyway
// would demand a value on import for a credential the user never had.
func TestAUserWithoutAPasswordIsExportedWithoutCredentials(t *testing.T) {
	c := newCredentialExport(t, "alice")
	c.typeDeclaresCredentials("password")
	c.userHasCredential("password", false)

	user := c.exportUser(t)

	assert.Equal(t, "user-1", user.ID, "the user was not exported")
	assert.Empty(t, user.Credentials)
}

// What is exported follows the user's type: each credential it declares, when the user has it.
func TestOnlyTheCredentialsAUserHasAreExported(t *testing.T) {
	c := newCredentialExport(t, "alice")
	c.typeDeclaresCredentials("password", "pin")
	c.userHasCredential("password", false)
	c.userHasCredential("pin", true)

	user := c.exportUser(t)

	assert.Equal(t, map[string]interface{}{"pin": "{{.USER_ALICE_PIN}}"}, user.Credentials)
}

// A type that declares no credentials exports its users with none, and asks nothing of the store.
func TestAUserTypeWithNoCredentialsExportsNone(t *testing.T) {
	c := newCredentialExport(t, "alice")
	c.typeDeclaresCredentials()

	user := c.exportUser(t)

	assert.Empty(t, user.Credentials)
}

// Without a username there is nothing to name the variable after, so no credential is exported and
// neither the type nor the store is consulted.
func TestAUserWithoutAUsernameExportsNoCredentials(t *testing.T) {
	c := newCredentialExport(t, "")

	user := c.exportUser(t)

	assert.Empty(t, user.Credentials)
}

// A failure to read the type is reported rather than exporting the user as if it had no
// credentials, which would import a user who can no longer sign in.
func TestAUnreadableUserTypeFailsTheUsersExport(t *testing.T) {
	c := newCredentialExport(t, "alice")
	c.entityTypes.On("GetAttributes", context.Background(), entitytype.TypeCategoryUser, "person",
		entitytype.AttributeFilter{AllowCredential: true}).
		Return(nil, &tidcommon.ServiceError{Code: "ETS-1001"})

	_, _, svcErr := c.exporter.GetResourceByID(context.Background(), "user-1")

	require.NotNil(t, svcErr)
	assert.Equal(t, "ETS-1001", svcErr.Code)
}

// The same holds for a failure to read the user's stored credentials.
func TestUnreadableCredentialsFailTheUsersExport(t *testing.T) {
	c := newCredentialExport(t, "alice")
	c.typeDeclaresCredentials("password")
	c.entities.On("GetCredentialsByType", context.Background(), "user-1", "password").
		Return(nil, errors.New("store unavailable"))

	_, _, svcErr := c.exporter.GetResourceByID(context.Background(), "user-1")

	require.NotNil(t, svcErr)
	assert.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)
}

// A control plane's export names the secret the gateway holds instead, as it does for every other
// credential, so the gateway's import fills it in and hashes it.
func TestAReferenceExportNamesAUsersPasswordAsASecret(t *testing.T) {
	c := newCredentialExport(t, "alice")
	c.exporter.WriteValueReferences(true)
	c.typeDeclaresCredentials("password")
	c.userHasCredential("password", true)

	user := c.exportUser(t)

	assert.Equal(t, map[string]interface{}{"password": "sec:USER_ALICE_PASSWORD"}, user.Credentials)
}
