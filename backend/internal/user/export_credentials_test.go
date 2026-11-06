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
	"github.com/thunder-id/thunderid/internal/sharing"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/sharingmock"
)

// credentialExport wires a user exporter over mocks for one user of type "person" named alice.
type credentialExport struct {
	users    *UserServiceInterfaceMock
	entities *entitymock.EntityServiceInterfaceMock
	sharing  *sharingmock.SharingServiceInterfaceMock
	exporter *userExporter
}

func newCredentialExport(t *testing.T, username string) *credentialExport {
	t.Helper()
	c := &credentialExport{
		users:    NewUserServiceInterfaceMock(t),
		entities: entitymock.NewEntityServiceInterfaceMock(t),
		sharing:  sharingmock.NewSharingServiceInterfaceMock(t),
	}
	c.exporter = newUserExporter(c.users, c.entities, c.sharing)
	// The user is shared nowhere unless a test says otherwise.
	c.sharing.On("ExportPolicies", context.Background(), UserSharingType, "user-1").
		Return([]sharing.ReplayablePolicy(nil), (*tidcommon.ServiceError)(nil)).Maybe()

	attrs, err := json.Marshal(map[string]interface{}{"username": username})
	require.NoError(t, err)
	c.users.On("GetUser", context.Background(), "user-1", false).
		Return(&providers.User{ID: "user-1", Type: "person", OUID: "ou-1", Attributes: attrs}, nil)
	return c
}

// userHolds sets the credentials the user's type declares that the user has set.
func (c *credentialExport) userHolds(credentials ...string) {
	c.entities.On("GetHeldCredentialTypes", context.Background(), "user-1").Return(credentials, nil)
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
	c.userHolds("password")

	user := c.exportUser(t)

	assert.Equal(t, map[string]interface{}{"password": "{{.USER_ALICE_PASSWORD}}"}, user.Credentials)
}

// A user who never set a password is still exported, with no credentials. Exporting one anyway
// would demand a value on import for a credential the user never had.
func TestAUserWithoutAPasswordIsExportedWithoutCredentials(t *testing.T) {
	c := newCredentialExport(t, "alice")
	c.userHolds()

	user := c.exportUser(t)

	assert.Equal(t, "user-1", user.ID, "the user was not exported")
	assert.Empty(t, user.Credentials)
}

// Each credential the user holds is exported under its own variable.
func TestEveryCredentialAUserHoldsIsExported(t *testing.T) {
	c := newCredentialExport(t, "alice")
	c.userHolds("password", "pin")

	user := c.exportUser(t)

	assert.Equal(t, map[string]interface{}{
		"password": "{{.USER_ALICE_PASSWORD}}",
		"pin":      "{{.USER_ALICE_PIN}}",
	}, user.Credentials)
}

// Without a username there is nothing to name the variable after, so no credential is exported and
// the store is not consulted.
func TestAUserWithoutAUsernameExportsNoCredentials(t *testing.T) {
	c := newCredentialExport(t, "")

	user := c.exportUser(t)

	assert.Empty(t, user.Credentials)
}

// A failure to read which credentials the user holds is reported rather than exporting the user as
// if it had none, which would import a user who can no longer sign in.
func TestUnreadableCredentialsFailTheUsersExport(t *testing.T) {
	c := newCredentialExport(t, "alice")
	c.entities.On("GetHeldCredentialTypes", context.Background(), "user-1").
		Return(nil, errors.New("store unavailable"))

	_, _, svcErr := c.exporter.GetResourceByID(context.Background(), "user-1")

	require.NotNil(t, svcErr)
	assert.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)
}

// A user whose type is gone fails its export as a client error, as a write of the user would.
func TestAUserWhoseTypeIsGoneFailsItsExportAsAClientError(t *testing.T) {
	c := newCredentialExport(t, "alice")
	c.entities.On("GetHeldCredentialTypes", context.Background(), "user-1").
		Return(nil, entity.ErrSchemaValidationFailed)

	_, _, svcErr := c.exporter.GetResourceByID(context.Background(), "user-1")

	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorSchemaValidationFailed.Code, svcErr.Code)
}

// A control plane's export names the secret the gateway holds instead, as it does for every other
// credential, so the gateway's import fills it in and hashes it.
func TestAReferenceExportNamesAUsersPasswordAsASecret(t *testing.T) {
	c := newCredentialExport(t, "alice")
	c.exporter.WriteValueReferences(true)
	c.userHolds("password")

	user := c.exportUser(t)

	assert.Equal(t, map[string]interface{}{"password": "sec:USER_ALICE_PASSWORD"}, user.Credentials)
}
