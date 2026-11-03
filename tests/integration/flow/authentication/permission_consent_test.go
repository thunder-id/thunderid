// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authentication

import (
	"encoding/json"
	"strings"
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

	// A subject granted the parent resource alone, and the role conferring it.
	subtreeUserID string
	subtreeRoleID string
	branchUserID  string
	branchRoleID  string

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
	permConsentSubtreeUsername = "permission_consent_subtree_user"
	permConsentBranchUsername  = "permission_consent_branch_user"
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

	// A second permission beneath the parent resource, granted only through it rather than in its
	// own right.
	permConsentSecondChildPermission = "docs:payroll"

	// A permission no role confers. It sits outside every granted resource, since a permission
	// beneath one would be conferred by the grant of that resource.
	permConsentUnheldPermission = "restricted"

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
		Handle:      permConsentUserTypeName,
		DisplayName: "Permission Consent Person",
		OUID:        ouID,
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
		// Declared, so a grant of "docs" carries what sits beneath it. Without this the cases below
		// would be asserting against a resource server whose permissions do not nest at all.
		HierarchicalPermissions: true,
	}, nil)
	ts.Require().NoError(err, "Failed to create test resource server")

	docsID := ts.createResource("Documents", permConsentParentPermission, "")
	ts.createResource("Reports", "reports", docsID)
	ts.createResource("Payroll", "payroll", docsID)
	ts.createResource("Docs Extended", permConsentDecoyPermission, "")
	ts.createResource("Restricted", permConsentUnheldPermission, "")

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

	// A subject granted the parent resource and nothing beneath it. The resource carries a permission
	// of its own, and granting it grants what it contains, including the child the role above omits.
	ts.subtreeUserID = ts.createPermConsentUser(permConsentSubtreeUsername, "permission.subtree@test.com")
	ts.subtreeRoleID, err = testutils.CreateRole(testutils.Role{
		Name:        "permission-consent-subtree",
		Description: "Role granting only the parent resource",
		OUID:        ouID,
		Permissions: []testutils.ResourcePermissions{
			{
				ResourceServerID: ts.rsID,
				Permissions:      []string{permConsentParentPermission},
			},
		},
		Assignments: []testutils.Assignment{
			{ID: ts.subtreeUserID, Type: "user"},
		},
	})
	ts.Require().NoError(err, "Failed to create the subtree role")

	ts.branchUserID = ts.createPermConsentUser(permConsentBranchUsername, "permission.branches@test.com")
	ts.branchRoleID, err = testutils.CreateRole(testutils.Role{
		Name: "permission-consent-branches",
		OUID: ouID,
		Permissions: []testutils.ResourcePermissions{{
			ResourceServerID: ts.rsID,
			Permissions:      []string{permConsentChildPermission, permConsentSecondChildPermission},
		}},
		Assignments: []testutils.Assignment{{ID: ts.branchUserID, Type: "user"}},
	})
	ts.Require().NoError(err)
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
	if ts.branchRoleID != "" {
		if err := testutils.DeleteRole(ts.branchRoleID); err != nil {
			ts.T().Logf("Failed to delete the branch role: %v", err)
		}
	}
	if ts.subtreeRoleID != "" {
		if err := testutils.DeleteRole(ts.subtreeRoleID); err != nil {
			ts.T().Logf("Failed to delete the subtree role: %v", err)
		}
	}
	if ts.roleID != "" {
		if err := testutils.DeleteRole(ts.roleID); err != nil {
			ts.T().Logf("Failed to delete test role: %v", err)
		}
	}
	for _, id := range []string{ts.promptUserID, ts.approveUserID, ts.denyUserID, ts.subtreeUserID, ts.branchUserID} {
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
			" " + permConsentSecondChildPermission + " " + permConsentDecoyPermission +
			" " + permConsentUnheldPermission,
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

// The prompt offers every authorized permission as a choice, with the hierarchy links the gate
// needs to show what each branch carries.
func (ts *PermissionConsentFlowTestSuite) TestPermissionConsent_OffersEveryAuthorizedPermission() {
	step := ts.authenticateToPermissionConsent(permConsentPromptUsername)
	purpose := ts.requirePermissionPurpose(step)

	ts.Equal(":", purpose.PermissionDelimiter,
		"the gate needs the delimiter to tell a branch root from what sits beneath it")

	offered := make([]string, 0, len(purpose.Optional))
	for _, element := range purpose.Optional {
		offered = append(offered, element.Name)
		if element.Name == permConsentChildPermission || element.Name == permConsentSecondChildPermission {
			ts.Equal(permConsentParentPermission, element.Parent)
		} else {
			ts.Empty(element.Parent)
		}
	}
	ts.ElementsMatch([]string{permConsentParentPermission, permConsentChildPermission,
		permConsentSecondChildPermission, permConsentDecoyPermission}, offered)
}

// TestPermissionConsent_PurposeIsNamedForTheApplication verifies the purpose name identifies the
// application, which is what scopes the recorded consent to it.
func (ts *PermissionConsentFlowTestSuite) TestPermissionConsent_PurposeIsNamedForTheApplication() {
	step := ts.authenticateToPermissionConsent(permConsentPromptUsername)
	purpose := ts.requirePermissionPurpose(step)

	ts.Equal("permissions:"+ts.appID, purpose.PurposeName,
		"the permission purpose is derived from the application it belongs to")
}

// A role granting the parent resource authorizes everything beneath it, including the child the
// explicitly-granted role deliberately omits, and nothing that merely shares its prefix.
func (ts *PermissionConsentFlowTestSuite) TestPermissionConsent_ParentGrantCarriesTheSubtree() {
	step := ts.authenticateToPermissionConsent(permConsentSubtreeUsername)
	purpose := ts.requirePermissionPurpose(step)

	offered := make([]string, 0, len(purpose.Optional))
	for _, element := range purpose.Optional {
		offered = append(offered, element.Name)
	}

	ts.ElementsMatch([]string{permConsentParentPermission, permConsentChildPermission,
		permConsentSecondChildPermission}, offered)
}

// Two permissions neither of which sits under the other are two separate grants, so taking one and
// refusing the other is an ordinary decision rather than a contradiction.
func (ts *PermissionConsentFlowTestSuite) TestPermissionConsent_EachBranchIsDecidedOnItsOwn() {
	step := ts.authenticateToPermissionConsent(permConsentBranchUsername)
	purpose := ts.requirePermissionPurpose(step)
	offered := make([]string, 0, len(purpose.Optional))
	for _, element := range purpose.Optional {
		offered = append(offered, element.Name)
	}
	ts.ElementsMatch([]string{permConsentChildPermission, permConsentSecondChildPermission}, offered)

	decisions, err := json.Marshal(consentDecisions{
		Approved: true,
		Purposes: []consentPurposeDecision{{
			PurposeName: purpose.PurposeName,
			Approved:    true,
			Elements: []consentElementDecision{
				{Name: permConsentChildPermission, Approved: true},
				{Name: permConsentSecondChildPermission, Approved: false},
			},
		}},
	})
	ts.Require().NoError(err)
	completed, err := common.CompleteFlow(step.ExecutionID,
		map[string]string{consentInputIdentifier: string(decisions)}, consentApproveAction, step.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("COMPLETE", completed.FlowStatus)
	claims, err := testutils.DecodeJWTPayloadMap(completed.Assertion)
	ts.Require().NoError(err)
	permissions, ok := claims["authorized_permissions"].(string)
	ts.Require().True(ok)
	ts.ElementsMatch([]string{permConsentChildPermission}, strings.Fields(permissions),
		"the refused branch is withheld and the approved one is issued")
}

func (ts *PermissionConsentFlowTestSuite) TestPermissionConsent_AcceptGrantsTheEntireRequestedSet() {
	step := ts.authenticateToPermissionConsent(permConsentApproveUsername)
	purpose := ts.requirePermissionPurpose(step)
	elements := make([]consentElementDecision, 0, len(purpose.Optional))
	for _, element := range purpose.Optional {
		elements = append(elements, consentElementDecision{Name: element.Name, Approved: true})
	}
	decisions, err := json.Marshal(consentDecisions{Approved: true, Purposes: []consentPurposeDecision{{
		PurposeName: purpose.PurposeName, Approved: true, Elements: elements,
	}}})
	ts.Require().NoError(err)
	completed, err := common.CompleteFlow(step.ExecutionID,
		map[string]string{consentInputIdentifier: string(decisions)}, consentApproveAction, step.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("COMPLETE", completed.FlowStatus)
	claims, err := testutils.DecodeJWTPayloadMap(completed.Assertion)
	ts.Require().NoError(err)
	permissions, ok := claims["authorized_permissions"].(string)
	ts.Require().True(ok)
	ts.ElementsMatch([]string{permConsentParentPermission, permConsentChildPermission,
		permConsentSecondChildPermission, permConsentDecoyPermission}, strings.Fields(permissions))
}

// Refusing everything is a decision the subject is allowed to make. Authentication succeeds and the
// assertion carries no permissions, which is how refusing an optional permission has always worked.
func (ts *PermissionConsentFlowTestSuite) TestPermissionConsent_DeclineGrantsNothingAndStillAuthenticates() {
	step := ts.authenticateToPermissionConsent(permConsentDenyUsername)
	purpose := ts.requirePermissionPurpose(step)
	ts.NotEmpty(purpose.Optional)
	completed, err := common.CompleteFlow(step.ExecutionID,
		map[string]string{consentInputIdentifier: `{"approved":false}`}, consentApproveAction, step.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("COMPLETE", completed.FlowStatus)
	claims, err := testutils.DecodeJWTPayloadMap(completed.Assertion)
	ts.Require().NoError(err)
	permissions, _ := claims["authorized_permissions"].(string)
	ts.Empty(strings.Fields(permissions))
}
