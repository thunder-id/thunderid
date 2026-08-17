// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authentication

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// PermissionConsentFlowTestSuite covers consent over resource-server permissions, as opposed to the
// user-attribute consent ConsentFlowTestSuite covers.
//
// The permission purpose is not stored: it is built at prompt time from the permissions the
// authorization executor actually authorized, so it can only be reached by running an
// AuthorizationExecutor node ahead of the consent node and asking for permission scopes bound to a
// resource server. Everything the suite asserts follows from that:
//
//   - only authorized permissions are prompted, never everything requested;
//   - the purpose is typed as permissions, so the Console renders it in its own section;
//   - each element carries the rollup parent the server computes, which is what lets the Console
//     group a permission under the broader one that implies it.
//
// The rollup parent is the part with real logic: a permission's parent is the longest other prompted
// permission it extends across a delimiter. The fixture therefore declares a nested resource, whose
// permission has a parent, alongside a sibling whose handle merely starts with the same letters,
// whose permission must not.
type PermissionConsentFlowTestSuite struct {
	suite.Suite
	config *common.TestSuiteConfig

	ouID       string
	userTypeID string
	appID      string
	rsID       string
	roleID     string

	// Resource IDs in creation order; teardown removes them leaf first so the resource server can
	// then be deleted.
	resourceIDs []string

	// A user per behaviour. The tests that only read the prompt share one, but the tests that
	// record a decision each need their own: a recorded consent suppresses the prompt every later
	// authentication of that user depends on.
	promptUserID  string
	approveUserID string
	denyUserID    string
}

const (
	permConsentOUHandle     = "permission-consent-ou"
	permConsentUserTypeName = "permission-consent-person"

	permConsentPromptUsername  = "permission_consent_prompt_user"
	permConsentApproveUsername = "permission_consent_approve_user"
	permConsentDenyUsername    = "permission_consent_deny_user"
	permConsentPassword        = "SecurePass123!"

	permConsentRSIdentifier = "permission-consent-docs"

	// The permission a top-level resource contributes. It is the rollup parent of the nested one.
	permConsentParentPermission = "docs"

	// The permission a resource nested under "docs" contributes. Its parent must resolve to "docs".
	permConsentChildPermission = "docs:reports"

	// A sibling top-level permission that starts with "docs" but does not extend it across a
	// delimiter, so it must have no parent. Without it a prefix check that ignored the delimiter
	// would pass.
	permConsentDecoyPermission = "docsx"

	// A permission the user is never granted. It is requested anyway, so the prompt is shown to
	// carry only what was authorized.
	permConsentUnheldPermission = "docs:payroll"

	permConsentPurposeType = "permissions"
)

func TestPermissionConsentFlowTestSuite(t *testing.T) {
	suite.Run(t, new(PermissionConsentFlowTestSuite))
}

func (ts *PermissionConsentFlowTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      permConsentOUHandle,
		Name:        "Permission Consent Test Organization Unit",
		Description: "Organization unit for permission consent flow testing",
	})
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

	ts.userTypeID, err = testutils.CreateUserType(testutils.UserType{
		Name: permConsentUserTypeName,
		OUID: ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
			"email":    map[string]interface{}{"type": "string"},
		},
	})
	ts.Require().NoError(err, "Failed to create test user type")

	// The resource tree is what the permission strings are derived from: a resource's permission is
	// its handle path, so nesting "reports" under "docs" produces "docs:reports".
	ts.rsID, err = testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:        "Permission Consent Document Store",
		Description: "Resource server for permission consent flow testing",
		Identifier:  permConsentRSIdentifier,
		OUID:        ouID,
	}, nil)
	ts.Require().NoError(err, "Failed to create test resource server")

	docsID := ts.createResource("Documents", permConsentParentPermission, "")
	ts.createResource("Reports", "reports", docsID)
	ts.createResource("Payroll", "payroll", docsID)
	ts.createResource("Docs Extended", permConsentDecoyPermission, "")

	flowID, err := testutils.CreateFlow(testutils.Flow{
		Name:     "Permission Consent Test Auth Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_permission_consent_test",
		Nodes:    permissionConsentFlowNodes(),
	})
	ts.Require().NoError(err, "Failed to create permission consent test flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)

	// No assertion config, so no attribute purpose applies and the permission purpose is the only
	// one prompted.
	ts.appID, err = testutils.CreateApplication(testutils.Application{
		Name:             "Permission Consent Flow Test Application",
		Description:      "Application for testing permission consent collection in flows",
		ClientID:         "permission_consent_flow_test_client",
		ClientSecret:     "permission_consent_flow_test_secret",
		RedirectURIs:     []string{"http://localhost:3000/callback"},
		OUID:             ouID,
		AllowedUserTypes: []string{permConsentUserTypeName},
		AuthFlowID:       flowID,
	})
	ts.Require().NoError(err, "Failed to create test application")

	ts.promptUserID = ts.createPermConsentUser(permConsentPromptUsername, "permission.prompt@test.com")
	ts.approveUserID = ts.createPermConsentUser(permConsentApproveUsername, "permission.approve@test.com")
	ts.denyUserID = ts.createPermConsentUser(permConsentDenyUsername, "permission.deny@test.com")

	// Granted to every user, and deliberately excluding docs:payroll.
	ts.roleID, err = testutils.CreateRole(testutils.Role{
		Name:        "permission-consent-reader",
		Description: "Role granting the permissions the consent prompt is expected to carry",
		OUID:        ouID,
		Permissions: []testutils.ResourcePermissions{
			{
				ResourceServerID: ts.rsID,
				Permissions: []string{
					permConsentParentPermission,
					permConsentChildPermission,
					permConsentDecoyPermission,
				},
			},
		},
		Assignments: []testutils.Assignment{
			{ID: ts.promptUserID, Type: "user"},
			{ID: ts.approveUserID, Type: "user"},
			{ID: ts.denyUserID, Type: "user"},
		},
	})
	ts.Require().NoError(err, "Failed to create test role")
}

// createResource creates a resource under the suite's resource server, recording it for teardown.
func (ts *PermissionConsentFlowTestSuite) createResource(name, handle, parentID string) string {
	ts.T().Helper()

	resourceID, err := testutils.CreateResource(ts.rsID, name, handle, parentID)
	ts.Require().NoError(err, "Failed to create the %s resource", handle)
	ts.resourceIDs = append(ts.resourceIDs, resourceID)
	return resourceID
}

// createPermConsentUser creates a user of the suite's type and returns its ID.
func (ts *PermissionConsentFlowTestSuite) createPermConsentUser(username, email string) string {
	ts.T().Helper()

	attributes, err := json.Marshal(map[string]string{
		"username": username,
		"password": permConsentPassword,
		"email":    email,
	})
	ts.Require().NoError(err)

	userID, err := testutils.CreateUser(testutils.User{
		Type:       permConsentUserTypeName,
		OUID:       ts.ouID,
		Attributes: attributes,
	})
	ts.Require().NoError(err, "Failed to create user %s", username)
	return userID
}

func (ts *PermissionConsentFlowTestSuite) TearDownSuite() {
	if ts.roleID != "" {
		if err := testutils.DeleteRole(ts.roleID); err != nil {
			ts.T().Logf("Failed to delete test role: %v", err)
		}
	}
	for _, id := range []string{ts.promptUserID, ts.approveUserID, ts.denyUserID} {
		if id != "" {
			if err := testutils.DeleteUser(id); err != nil {
				ts.T().Logf("Failed to delete test user %s: %v", id, err)
			}
		}
	}
	if ts.appID != "" {
		if err := testutils.DeleteApplication(ts.appID); err != nil {
			ts.T().Logf("Failed to delete test application: %v", err)
		}
	}
	for _, flowID := range ts.config.CreatedFlowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete created flow %s: %v", flowID, err)
		}
	}
	// Leaf first: a resource server with resources still attached refuses deletion, and a parent
	// resource refuses it while it still has children.
	for i := len(ts.resourceIDs) - 1; i >= 0; i-- {
		if err := testutils.DeleteResource(ts.rsID, ts.resourceIDs[i]); err != nil {
			ts.T().Logf("Failed to delete test resource %s: %v", ts.resourceIDs[i], err)
		}
	}
	if ts.rsID != "" {
		if err := testutils.DeleteResourceServer(ts.rsID); err != nil {
			ts.T().Logf("Failed to delete test resource server: %v", err)
		}
	}
	if ts.userTypeID != "" {
		if err := testutils.DeleteUserType(ts.userTypeID); err != nil {
			ts.T().Logf("Failed to delete test user type: %v", err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete test organization unit: %v", err)
		}
	}
}

// permissionConsentFlowNodes builds credentials, then authorization, then consent. The
// authorization node is what puts authorized permissions into runtime data, which is the only
// source the permission purpose is built from.
func permissionConsentFlowNodes() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"id":        "start",
			"type":      "START",
			"onSuccess": "prompt_credentials",
		},
		{
			"id":   "prompt_credentials",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{
				{
					"inputs": []map[string]interface{}{
						{
							"ref":        "input_001",
							"identifier": "username",
							"type":       "TEXT_INPUT",
							"required":   true,
						},
						{
							"ref":        "input_002",
							"identifier": "password",
							"type":       "PASSWORD_INPUT",
							"required":   true,
						},
					},
					"action": map[string]interface{}{
						"ref":      "action_001",
						"nextNode": "credentials_auth",
					},
				},
			},
		},
		{
			"id":   "credentials_auth",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{
				"name": "CredentialsAuthExecutor",
			},
			"onSuccess":    "authorization_check",
			"onIncomplete": "prompt_credentials",
		},
		{
			"id":   "authorization_check",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{
				"name": "AuthorizationExecutor",
			},
			"onSuccess": "consent_check",
		},
		{
			"id":   "consent_check",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{
				"name": "ConsentExecutor",
			},
			"onSuccess":    "auth_assert",
			"onIncomplete": "prompt_consent",
		},
		{
			"id":   "prompt_consent",
			"type": "PROMPT",
			"meta": map[string]interface{}{
				"components": []map[string]interface{}{
					{
						"type": "BLOCK",
						"id":   "consent_block",
						"components": []map[string]interface{}{
							{
								"id":       "consent_input",
								"ref":      consentInputIdentifier,
								"type":     "CONSENT_INPUT",
								"required": true,
							},
							{
								"type":      "ACTION",
								"id":        consentApproveAction,
								"label":     "Approve",
								"eventType": "SUBMIT",
							},
						},
					},
				},
			},
			"prompts": []map[string]interface{}{
				{
					"inputs": []map[string]interface{}{
						{
							"ref":        "consent_input",
							"identifier": consentInputIdentifier,
							"type":       "CONSENT_INPUT",
							"required":   true,
						},
					},
					"action": map[string]interface{}{
						"ref":      consentApproveAction,
						"nextNode": "consent_check",
					},
				},
			},
		},
		{
			"id":   "auth_assert",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{
				"name": "AuthAssertExecutor",
			},
			"onSuccess": "end",
		},
		{
			"id":   "end",
			"type": "END",
		},
	}
}

// authenticateToPermissionConsent runs the flow to the consent prompt, requesting every permission
// including the one the user does not hold.
func (ts *PermissionConsentFlowTestSuite) authenticateToPermissionConsent(username string) *common.FlowStep {
	ts.T().Helper()

	step, err := common.InitiateAuthenticationFlow(ts.appID, false, map[string]string{
		"applicationId": ts.appID,
		"requested_permissions": permConsentParentPermission + " " + permConsentChildPermission +
			" " + permConsentDecoyPermission + " " + permConsentUnheldPermission,
		"resource_server_identifier": permConsentRSIdentifier,
	}, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "Flow should pause at the credentials prompt")

	step, err = common.CompleteFlow(step.ExecutionID, map[string]string{
		"username": username,
		"password": permConsentPassword,
	}, "action_001", step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit credentials")
	return step
}

// requirePermissionPurpose asserts the step is a consent prompt carrying exactly one permission
// purpose, and returns it.
func (ts *PermissionConsentFlowTestSuite) requirePermissionPurpose(
	step *common.FlowStep,
) consentPromptPurpose {
	ts.T().Helper()

	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "Consent should pause the flow for input")
	ts.Require().True(common.HasInput(step.Data.Inputs, consentInputIdentifier),
		"The consent prompt must request the consent decisions input")

	promptJSON, ok := step.Data.AdditionalData[consentPromptDataKey]
	ts.Require().True(ok, "The consent prompt must carry the purposes to render")

	var purposes []consentPromptPurpose
	ts.Require().NoError(json.Unmarshal([]byte(promptJSON), &purposes),
		"Consent prompt data should be a purposes array")
	ts.Require().Len(purposes, 1,
		"Only the permission purpose applies; the application requests no attributes")
	ts.Require().Equal(permConsentPurposeType, purposes[0].Type,
		"The purpose must be typed as permissions so the Console renders it as such")
	return purposes[0]
}

// TestPermissionConsent_ElementsCarryRollupParents verifies the rollup linkage the Console groups by.
// The nested permission is linked to the one it extends; the sibling that merely shares a prefix is
// not, which is what the delimiter check in the parent computation exists for.
func (ts *PermissionConsentFlowTestSuite) TestPermissionConsent_ElementsCarryRollupParents() {
	step := ts.authenticateToPermissionConsent(permConsentPromptUsername)
	purpose := ts.requirePermissionPurpose(step)

	parents := make(map[string]string, len(purpose.Optional))
	for _, element := range purpose.Optional {
		parents[element.Name] = element.Parent
	}

	ts.Equal(permConsentParentPermission, parents[permConsentChildPermission],
		"a permission nested under another must roll up to it")
	ts.Empty(parents[permConsentParentPermission],
		"a top-level permission has nothing to roll up to")
	ts.Empty(parents[permConsentDecoyPermission],
		"sharing a prefix without a delimiter is not a parent relationship")

	// The application requests this permission but the user was never granted it. Offering it for
	// consent would record a decision about access the user does not hold.
	ts.NotContains(parents, permConsentUnheldPermission,
		"a requested but unauthorized permission must never be prompted")
}

// TestPermissionConsent_PurposeIsNamedForTheApplication verifies the purpose name identifies the
// application, which is what scopes the recorded consent to it.
func (ts *PermissionConsentFlowTestSuite) TestPermissionConsent_PurposeIsNamedForTheApplication() {
	step := ts.authenticateToPermissionConsent(permConsentPromptUsername)
	purpose := ts.requirePermissionPurpose(step)

	ts.Equal("permissions:"+ts.appID, purpose.PurposeName,
		"the permission purpose is derived from the application it belongs to")
}

