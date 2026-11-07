// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importexport

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// The machine client and the scopes of the role permission revocation test.
const (
	importRevocationClientID     = "import_revocation_client"
	importRevocationClientSecret = "import_revocation_secret" // #nosec G101 -- test fixture, not a credential
	importRevocationAudience     = "https://import-revocation.example.com/api"
	importRevocationScopeKept    = "import_revocation_kept"
	importRevocationScopeDropped = "import_revocation_dropped"
)

// TestImportRoleDroppingAPermission_RevokesOnlyTheDroppedPermission covers an import upsert that drops a
// permission from a role: a token carrying the dropped permission is rejected, and one carrying only the
// kept permission stays accepted.
func (suite *ImportExportErrorSuite) TestImportRoleDroppingAPermission_RevokesOnlyTheDroppedPermission() {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "import_revocation_ou", Name: "Import Revocation OU",
		Description: "Organization unit created for import revocation testing",
	})
	suite.Require().NoError(err)
	suite.T().Cleanup(func() { _ = testutils.DeleteOrganizationUnit(ouID) })

	rsID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name: "Import Revocation API", Identifier: importRevocationAudience, OUID: ouID,
	}, nil)
	suite.Require().NoError(err)
	suite.T().Cleanup(func() { _ = testutils.DeleteResourceServer(rsID) })
	for _, scope := range []string{importRevocationScopeKept, importRevocationScopeDropped} {
		resourceID, resErr := testutils.CreateResource(rsID, scope, scope, "")
		suite.Require().NoError(resErr)
		suite.T().Cleanup(func() { _ = testutils.DeleteResource(rsID, resourceID) })
	}

	appID, err := testutils.CreateApplication(testutils.Application{
		Name: "Import Revocation Client", OUID: ouID,
		InboundAuthConfig: []map[string]interface{}{{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                importRevocationClientID,
				"clientSecret":            importRevocationClientSecret,
				"grantTypes":              []string{"client_credentials"},
				"tokenEndpointAuthMethod": "client_secret_basic",
			},
		}},
	})
	suite.Require().NoError(err)
	suite.T().Cleanup(func() { _ = testutils.DeleteApplication(appID) })

	roleID := newImportUUID(suite)
	defer suite.deleteImportedRole(roleID)
	result := suite.importRole(importRevocationRoleDocument(roleID, ouID, rsID, appID,
		importRevocationScopeKept, importRevocationScopeDropped))
	suite.Require().Equal("success", result.Status, "message: %s", result.Message)
	dropped := suite.activeToken(importRevocationScopeDropped)
	kept := suite.activeToken(importRevocationScopeKept)

	result = suite.importRole(importRevocationRoleDocument(roleID, ouID, rsID, appID, importRevocationScopeKept))
	suite.Require().Equal("success", result.Status, "message: %s", result.Message)
	suite.Equal("update", result.Operation)

	suite.False(suite.tokenIsActive(dropped), "the token carrying the dropped permission must be rejected")
	suite.True(suite.tokenIsActive(kept), "the token carrying only the kept permission must stay accepted")
}

// activeToken obtains a client credentials token for the scope and requires it to carry the scope and be
// active.
func (suite *ImportExportErrorSuite) activeToken(scope string) string {
	result, err := testutils.RequestClientCredentialsToken(importRevocationClientID, importRevocationClientSecret,
		scope, importRevocationAudience)
	suite.Require().NoError(err)
	suite.Require().Equal(http.StatusOK, result.StatusCode, "body: %s", string(result.Body))
	suite.Require().Contains(strings.Fields(result.Token.Scope), scope)
	suite.Require().True(suite.tokenIsActive(result.Token.AccessToken))
	return result.Token.AccessToken
}

// tokenIsActive reports whether introspection accepts the token.
func (suite *ImportExportErrorSuite) tokenIsActive(token string) bool {
	active, err := testutils.IntrospectToken(token, importRevocationClientID, importRevocationClientSecret)
	suite.Require().NoError(err)
	return active
}

// importRevocationRoleDocument builds a role import document granting the permissions on the resource
// server and assigning the role to the application.
func importRevocationRoleDocument(id, ouID, rsID, appID string, permissions ...string) string {
	return fmt.Sprintf("resource_type: role\nid: %s\nname: %q\nouId: %s\n"+
		"permissions:\n  - resourceServerId: %s\n    permissions: [%s]\n"+
		"assignments:\n  - id: %s\n    type: app\n",
		id, "Import Revocation Role "+id[:8], ouID, rsID, strings.Join(permissions, ", "), appID)
}
