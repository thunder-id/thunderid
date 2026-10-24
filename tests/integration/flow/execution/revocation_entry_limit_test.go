// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	oauthConfigKey            = "oauth"
	entryLimitPerChange       = 1
	errCodeEntryLimitExceeded = "RVS-1001"
	entryLimitResourceID      = "https://entrylimit.dmv.test"
)

const (
	scopeOverLimitFirst  = "over-limit-first"
	scopeOverLimitSecond = "over-limit-second"
	scopeAtLimit         = "at-limit"
)

var entryLimitScopes = []string{scopeOverLimitFirst, scopeOverLimitSecond, scopeAtLimit}

var entryLimitClient = machineClient{
	id:       "entry_limit_flow_client",
	secret:   "entry_limit_flow_secret", // #nosec G101 -- test fixture, not a credential
	audience: entryLimitResourceID,
}

var entryLimitTestOU = testutils.OrganizationUnit{
	Handle:      "entry_limit_flow_test_ou",
	Name:        "Test OU for the Revocation Entry Limit",
	Description: "Organization unit created for revocation entry limit testing",
	Parent:      nil,
}

// The limit is read once when the revocation service is built, so the suite restarts the server with it.
type RevocationEntryLimitTestSuite struct {
	suite.Suite
	roleDeletionFlowID string
	ouID               string
	resourceServerID   string
	resourceIDs        []string
	appID              string
	createdRoleIDs     []string
}

func TestRevocationEntryLimitTestSuite(t *testing.T) {
	suite.Run(t, new(RevocationEntryLimitTestSuite))
}

func (ts *RevocationEntryLimitTestSuite) SetupSuite() {
	original, err := testutils.ReadDeploymentConfigKey(oauthConfigKey)
	ts.Require().NoError(err, "Failed to read the oauth section of deployment.yaml")
	// Registered before the patch: a failing SetupSuite skips TearDownSuite, but never a cleanup.
	t := ts.T()
	t.Cleanup(func() { restoreOAuthConfig(t, original) })

	ts.Require().NoError(testutils.PatchDeploymentConfig(map[string]interface{}{
		oauthConfigKey: oauthWithEntryLimit(original, entryLimitPerChange),
	}), "Failed to set the revocation entry limit")
	ts.Require().NoError(testutils.RestartServer(), "Failed to restart the server with the entry limit")
	ts.Require().NoError(testutils.ObtainAdminAccessToken(), "Failed to re-obtain the admin token after restart")

	flowID, err := testutils.GetFlowIDByHandle(roleDeletionFlowHandle, administrationFlowType)
	ts.Require().NoError(err, "Failed to resolve the shipped %s", roleDeletionFlowHandle)
	ts.Require().NotEmpty(flowID, "The shipped %s must be present", roleDeletionFlowHandle)
	ts.roleDeletionFlowID = flowID

	ouID, err := testutils.CreateOrganizationUnit(entryLimitTestOU)
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

	rsID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:        "Entry Limit DMV API",
		Description: "Resource server created for revocation entry limit testing",
		Identifier:  entryLimitResourceID,
		OUID:        ts.ouID,
	}, nil)
	ts.Require().NoError(err, "Failed to create test resource server")
	ts.resourceServerID = rsID

	for _, scope := range entryLimitScopes {
		resourceID, resErr := testutils.CreateResource(ts.resourceServerID, scope, scope, "")
		ts.Require().NoError(resErr, "Failed to create test resource %s", scope)
		ts.resourceIDs = append(ts.resourceIDs, resourceID)
	}

	appID, err := testutils.CreateApplication(testutils.Application{
		Name:        "Entry Limit Machine Client",
		Description: "Application created for revocation entry limit testing",
		OUID:        ts.ouID,
		InboundAuthConfig: []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                entryLimitClient.id,
					"clientSecret":            entryLimitClient.secret,
					"grantTypes":              []string{"client_credentials"},
					"tokenEndpointAuthMethod": "client_secret_basic",
				},
			},
		},
	})
	ts.Require().NoError(err, "Failed to create test application")
	ts.appID = appID
}

func (ts *RevocationEntryLimitTestSuite) TearDownSuite() {
	for _, roleID := range ts.createdRoleIDs {
		if err := testutils.DeleteRole(roleID); err != nil {
			ts.T().Logf("Failed to delete test role %s during teardown: %v", roleID, err)
		}
	}
	if ts.appID != "" {
		if err := testutils.DeleteApplication(ts.appID); err != nil {
			ts.T().Logf("Failed to delete test application during teardown: %v", err)
		}
	}
	for _, resourceID := range ts.resourceIDs {
		if err := testutils.DeleteResource(ts.resourceServerID, resourceID); err != nil {
			ts.T().Logf("Failed to delete test resource %s during teardown: %v", resourceID, err)
		}
	}
	if ts.resourceServerID != "" {
		if err := testutils.DeleteResourceServer(ts.resourceServerID); err != nil {
			ts.T().Logf("Failed to delete test resource server during teardown: %v", err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}
}

func (ts *RevocationEntryLimitTestSuite) TestRoleDeletionFlow_OverLimitIsRefusedAndAppliesNothing() {
	holder := testutils.Assignment{ID: ts.appID, Type: "app"}
	// One holder times two scopes plans two entries, one more than the limit.
	roleID := createRole(&ts.Suite, &ts.createdRoleIDs, ts.ouID, ts.resourceServerID,
		"Entry Limit Over Administrator", []string{scopeOverLimitFirst, scopeOverLimitSecond}, holder)
	first := grantedToken(&ts.Suite, entryLimitClient, scopeOverLimitFirst)
	second := grantedToken(&ts.Suite, entryLimitClient, scopeOverLimitSecond)

	step := executeAsAdmin(&ts.Suite, ts.roleDeletionFlowID, map[string]string{roleTargetInput: roleID})

	ts.Equal("ERROR", step.FlowStatus, "A change over the entry limit must not complete")
	ts.Require().NotNil(step.Error, "The refusal must be reported in the error envelope")
	ts.Equal(errCodeEntryLimitExceeded, step.Error.Code, "The refusal must carry the entry limit code")
	ts.Equal("2", step.Error.Description.Params["entries"],
		"The refusal must report how many entries the change would have written")
	ts.Equal("1", step.Error.Description.Params["max"], "The refusal must report the configured limit")

	ts.True(tokenIsActive(&ts.Suite, entryLimitClient, first),
		"A refused change must revoke nothing, so the first scope's token must stay active")
	ts.True(tokenIsActive(&ts.Suite, entryLimitClient, second),
		"A refused change must revoke nothing, so the second scope's token must stay active")

	assignments, err := testutils.GetRoleAssignments(roleID)
	ts.Require().NoError(err, "The role must still exist after a refused deletion")
	holderIDs := make([]string, 0, len(assignments))
	for _, assignment := range assignments {
		holderIDs = append(holderIDs, assignment.ID)
	}
	ts.Contains(holderIDs, ts.appID, "The refused deletion must leave the role's holder assigned")
	// Fresh grants prove the role still confers both permissions, not just that its record survives.
	grantedToken(&ts.Suite, entryLimitClient, scopeOverLimitFirst)
	grantedToken(&ts.Suite, entryLimitClient, scopeOverLimitSecond)
}

func (ts *RevocationEntryLimitTestSuite) TestRoleDeletionFlow_AtLimitCompletes() {
	// Exactly as many entries as the limit is allowed: the limit refuses only a change that exceeds it.
	roleID := createRole(&ts.Suite, &ts.createdRoleIDs, ts.ouID, ts.resourceServerID,
		"Entry Limit At Administrator", []string{scopeAtLimit}, testutils.Assignment{ID: ts.appID, Type: "app"})
	token := grantedToken(&ts.Suite, entryLimitClient, scopeAtLimit)

	step := executeAsAdmin(&ts.Suite, ts.roleDeletionFlowID, map[string]string{roleTargetInput: roleID})

	requireCompleted(&ts.Suite, step)
	ts.False(tokenIsActive(&ts.Suite, entryLimitClient, token),
		"A change within the limit must revoke the deleted role's scope")
}

// oauthWithEntryLimit copies the oauth section along the limit's path, because PatchDeploymentConfig
// replaces a top-level key wholesale and the captured original must stay untouched for the restore.
func oauthWithEntryLimit(original interface{}, limit int) map[string]interface{} {
	oauth := copyConfigSection(original)
	revocation := copyConfigSection(oauth["revocation"])
	criteria := copyConfigSection(revocation["criteria"])
	criteria["max_entries_per_change"] = limit
	revocation["criteria"] = criteria
	oauth["revocation"] = revocation
	return oauth
}

func copyConfigSection(section interface{}) map[string]interface{} {
	source, _ := section.(map[string]interface{})
	copied := make(map[string]interface{}, len(source)+1)
	for key, value := range source {
		copied[key] = value
	}
	return copied
}

// restoreOAuthConfig only logs: a failed restore must not fail the suite that already reported its results.
func restoreOAuthConfig(t *testing.T, original interface{}) {
	if err := testutils.PatchDeploymentConfig(map[string]interface{}{
		oauthConfigKey: copyConfigSection(original),
	}); err != nil {
		t.Logf("Failed to restore the oauth section of deployment.yaml: %v", err)
		return
	}
	if err := testutils.RestartServer(); err != nil {
		t.Logf("Server did not restart cleanly after the oauth config restore: %v", err)
		return
	}
	if err := testutils.ObtainAdminAccessToken(); err != nil {
		t.Logf("Failed to re-obtain the admin token after the oauth config restore: %v", err)
	}
}
