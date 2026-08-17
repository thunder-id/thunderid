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

// This suite pins the consent side of AuthAssertExecutor.resolvePermissionsForClaim. The claim is
// resolved from a precedence chain:
//
//   1. consent ran and an authorization step ran: the consented set intersected with the currently
//      authorized set, so a stale consent record cannot re-grant a revoked permission.
//   2. consent ran without an authorization step: the consented set is used verbatim.
//   3. consent did not run: the authorized set is used (covered by FlowAuthzTestSuite).
//
// Each scenario uses its own user because consent records are keyed by (application, user) and an
// element that already has active consent is not prompted again, which would change the branch
// under test on a second run.

const (
	consentPermsResourceServer = "consent-perms-mgmt"
	consentPermsPassword       = "SecurePass123!"
)

var consentPermsOU = testutils.OrganizationUnit{
	Handle:      "consent-perms-test-ou",
	Name:        "Consent Permissions Test OU",
	Description: "Organization unit for consent permission claim tests",
	Parent:      nil,
}

var consentPermsUserType = testutils.UserType{
	Name: "consent-perms-person",
	Schema: map[string]interface{}{
		"username":    map[string]interface{}{"type": "string"},
		"password":    map[string]interface{}{"type": "string", "credential": true},
		"email":       map[string]interface{}{"type": "string"},
		"given_name":  map[string]interface{}{"type": "string"},
		"family_name": map[string]interface{}{"type": "string"},
	},
}

// consentPromptNodes are the consent executor node and its prompt view, shared by both test flows.
// The consent executor forwards the prompt on onIncomplete and is re-entered by both the approve
// and the deny action, exactly as the sample application flows wire it.
func consentPromptNodes(nextNodeAfterConsent string) []map[string]interface{} {
	return []map[string]interface{}{
		{
			"id":           "consent_check",
			"type":         "TASK_EXECUTION",
			"executor":     map[string]interface{}{"name": "ConsentExecutor"},
			"onSuccess":    nextNodeAfterConsent,
			"onIncomplete": "prompt_consent",
		},
		{
			"id":   "prompt_consent",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{
				{
					"inputs": []map[string]interface{}{
						{
							"ref":        "consent_input",
							"identifier": "consent_decisions",
							"type":       "CONSENT_INPUT",
							"required":   true,
						},
					},
					"action": map[string]interface{}{"ref": "consent_action_allow", "nextNode": "consent_check"},
				},
				{
					"inputs": []map[string]interface{}{
						{
							"ref":        "consent_input",
							"identifier": "consent_decisions",
							"type":       "CONSENT_INPUT",
							"required":   true,
						},
					},
					"action": map[string]interface{}{"ref": "consent_action_deny", "nextNode": "consent_check"},
				},
			},
		},
	}
}

// credentialsNodes are the start and password authentication nodes shared by both test flows.
func credentialsNodes(nextNodeAfterAuth string) []map[string]interface{} {
	return []map[string]interface{}{
		{"id": "start", "type": "START", "onSuccess": "prompt_credentials"},
		{
			"id":   "prompt_credentials",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{
				{
					"inputs": []map[string]interface{}{
						{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
						{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
					},
					"action": map[string]interface{}{"ref": "action_001", "nextNode": "credentials_auth"},
				},
			},
		},
		{
			"id":   "credentials_auth",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{
				"name": "CredentialsAuthExecutor",
				"inputs": []map[string]interface{}{
					{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
					{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
				},
			},
			"onSuccess": nextNodeAfterAuth,
		},
	}
}

func assertAndConsentNodes() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"id":        "auth_assert",
			"type":      "TASK_EXECUTION",
			"executor":  map[string]interface{}{"name": "AuthAssertExecutor"},
			"onSuccess": "end",
		},
		{"id": "end", "type": "END"},
	}
}

func buildConsentFlowWithAuthz() testutils.Flow {
	nodes := credentialsNodes("authorization_check")
	nodes = append(nodes, map[string]interface{}{
		"id":        "authorization_check",
		"type":      "TASK_EXECUTION",
		"executor":  map[string]interface{}{"name": "AuthorizationExecutor"},
		"onSuccess": "consent_check",
	})
	nodes = append(nodes, consentPromptNodes("auth_assert")...)
	nodes = append(nodes, assertAndConsentNodes()...)

	return testutils.Flow{
		Name:     "Consent Permissions Auth Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_consent_perms",
		Nodes:    nodes,
	}
}

func buildConsentFlowWithoutAuthz() testutils.Flow {
	nodes := credentialsNodes("consent_check")
	nodes = append(nodes, consentPromptNodes("auth_assert")...)
	nodes = append(nodes, assertAndConsentNodes()...)

	return testutils.Flow{
		Name:     "Consent Without Authorization Auth Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_consent_no_authz",
		Nodes:    nodes,
	}
}

// consentPromptElement mirrors the element entries of the consent prompt payload the executor
// publishes under the consentPrompt key of the flow response additional data.
type consentPromptElement struct {
	Name string `json:"name"`
	// Parent is the element a nested permission rolls up to, absent for a top-level one.
	Parent string `json:"parent,omitempty"`
}

// consentPromptPurpose mirrors one purpose of the consent prompt payload.
type consentPromptPurpose struct {
	PurposeName string                 `json:"purposeName"`
	Type        string                 `json:"type"`
	Essential   []consentPromptElement `json:"essential"`
	Optional    []consentPromptElement `json:"optional"`
}

type consentElementDecision struct {
	Name     string `json:"name"`
	Approved bool   `json:"approved"`
}

type consentPurposeDecision struct {
	PurposeName string                   `json:"purposeName"`
	Approved    bool                     `json:"approved"`
	Elements    []consentElementDecision `json:"elements"`
}

type consentDecisions struct {
	Approved bool                     `json:"approved"`
	Reason   string                   `json:"reason,omitempty"`
	Purposes []consentPurposeDecision `json:"purposes"`
}

type ConsentPermissionsTestSuite struct {
	suite.Suite
	ouID             string
	userTypeID       string
	resourceServerID string
	authzFlowID      string
	noAuthzFlowID    string
	authzAppID       string
	noAuthzAppID     string
	subsetRoleID     string
	staleRoleID      string
	subsetUserID     string
	staleUserID      string
	denyAllUserID    string
	noAuthzUserID    string
}

func TestConsentPermissionsTestSuite(t *testing.T) {
	suite.Run(t, new(ConsentPermissionsTestSuite))
}

func (ts *ConsentPermissionsTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(consentPermsOU)
	ts.Require().NoError(err, "Failed to create organization unit")
	ts.ouID = ouID

	consentPermsUserType.OUID = ts.ouID
	userTypeID, err := testutils.CreateUserType(consentPermsUserType)
	ts.Require().NoError(err, "Failed to create user type")
	ts.userTypeID = userTypeID

	resourceServerID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:        "Consent Permissions Document Store",
		Description: "Resource server for consent permission claim tests",
		Identifier:  consentPermsResourceServer,
		OUID:        ts.ouID,
	}, []testutils.Action{
		{Name: "Read Documents", Handle: "read", Description: "Permission to read documents"},
		{Name: "Write Documents", Handle: "write", Description: "Permission to write documents"},
		{Name: "Delete Documents", Handle: "delete", Description: "Permission to delete documents"},
	})
	ts.Require().NoError(err, "Failed to create resource server")
	ts.resourceServerID = resourceServerID

	authzFlowID, err := testutils.CreateFlow(buildConsentFlowWithAuthz())
	ts.Require().NoError(err, "Failed to create the consent flow with an authorization step")
	ts.authzFlowID = authzFlowID

	noAuthzFlowID, err := testutils.CreateFlow(buildConsentFlowWithoutAuthz())
	ts.Require().NoError(err, "Failed to create the consent flow without an authorization step")
	ts.noAuthzFlowID = noAuthzFlowID

	// The application driving the permission scenarios releases no user attributes, so the consent
	// prompt contains only the permissions purpose.
	authzAppID, err := testutils.CreateApplication(testutils.Application{
		Name:             "Consent Permissions App",
		Description:      "Application for the consent permission claim tests",
		OUID:             ts.ouID,
		AuthFlowID:       authzFlowID,
		AllowedUserTypes: []string{consentPermsUserType.Name},
		Embedded:         true,
	})
	ts.Require().NoError(err, "Failed to create the consent permissions application")
	ts.authzAppID = authzAppID

	// The application driving the no-authorization scenario releases user attributes so the consent
	// prompt has an attribute purpose to show. Without a purpose to prompt, consent is skipped
	// entirely and the consented-permissions branch is never reached.
	noAuthzAppID, err := testutils.CreateApplication(testutils.Application{
		Name:             "Consent Without Authorization App",
		Description:      "Application for the consent without authorization test",
		OUID:             ts.ouID,
		AuthFlowID:       noAuthzFlowID,
		AllowedUserTypes: []string{consentPermsUserType.Name},
		Embedded:         true,
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{"email", "given_name", "family_name"},
		},
	})
	ts.Require().NoError(err, "Failed to create the consent without authorization application")
	ts.noAuthzAppID = noAuthzAppID

	ts.subsetUserID = ts.createUser("consent_subset_user")
	ts.staleUserID = ts.createUser("consent_stale_user")
	ts.denyAllUserID = ts.createUser("consent_deny_all_user")
	ts.noAuthzUserID = ts.createUser("consent_no_authz_user")

	subsetRoleID, err := testutils.CreateRole(testutils.Role{
		Name:        "ConsentSubsetEditor",
		Description: "Grants read, write and delete on the consent permissions resource server",
		OUID:        ts.ouID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: ts.resourceServerID, Permissions: []string{"read", "write", "delete"}},
		},
		Assignments: []testutils.Assignment{
			{ID: ts.subsetUserID, Type: "user"},
			{ID: ts.denyAllUserID, Type: "user"},
		},
	})
	ts.Require().NoError(err, "Failed to create the subset role")
	ts.subsetRoleID = subsetRoleID

	staleRoleID, err := testutils.CreateRole(testutils.Role{
		Name:        "ConsentStaleEditor",
		Description: "Grants the permissions that are later revoked to create a stale consent record",
		OUID:        ts.ouID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: ts.resourceServerID, Permissions: []string{"read", "write"}},
		},
		Assignments: []testutils.Assignment{
			{ID: ts.staleUserID, Type: "user"},
		},
	})
	ts.Require().NoError(err, "Failed to create the stale consent role")
	ts.staleRoleID = staleRoleID
}

func (ts *ConsentPermissionsTestSuite) TearDownSuite() {
	for _, roleID := range []string{ts.subsetRoleID, ts.staleRoleID} {
		if roleID == "" {
			continue
		}
		if err := testutils.DeleteRole(roleID); err != nil {
			ts.T().Logf("Failed to delete role during teardown: %v", err)
		}
	}
	for _, userID := range []string{ts.subsetUserID, ts.staleUserID, ts.denyAllUserID, ts.noAuthzUserID} {
		if userID == "" {
			continue
		}
		if err := testutils.DeleteUser(userID); err != nil {
			ts.T().Logf("Failed to delete user during teardown: %v", err)
		}
	}
	for _, appID := range []string{ts.authzAppID, ts.noAuthzAppID} {
		if appID == "" {
			continue
		}
		if err := testutils.DeleteApplication(appID); err != nil {
			ts.T().Logf("Failed to delete application during teardown: %v", err)
		}
	}
	for _, flowID := range []string{ts.authzFlowID, ts.noAuthzFlowID} {
		if flowID == "" {
			continue
		}
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete flow during teardown: %v", err)
		}
	}
	if ts.resourceServerID != "" {
		// A resource server that still defines actions is not deletable, so the actions go first.
		actionIDs, err := testutils.GetActionsByResourceServer(ts.resourceServerID)
		if err != nil {
			ts.T().Logf("Failed to list resource server actions during teardown: %v", err)
		}
		for _, actionID := range actionIDs {
			if err := testutils.DeleteAction(ts.resourceServerID, actionID); err != nil {
				ts.T().Logf("Failed to delete resource server action during teardown: %v", err)
			}
		}
		if err := testutils.DeleteResourceServer(ts.resourceServerID); err != nil {
			ts.T().Logf("Failed to delete resource server during teardown: %v", err)
		}
	}
	if ts.userTypeID != "" {
		if err := testutils.DeleteUserType(ts.userTypeID); err != nil {
			ts.T().Logf("Failed to delete user type during teardown: %v", err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete organization unit during teardown: %v", err)
		}
	}
}

func (ts *ConsentPermissionsTestSuite) createUser(username string) string {
	attributes, err := json.Marshal(map[string]interface{}{
		"username":    username,
		"password":    consentPermsPassword,
		"email":       username + "@test.com",
		"given_name":  "Consent",
		"family_name": "Tester",
	})
	ts.Require().NoError(err, "Failed to marshal user attributes")

	userID, err := testutils.CreateUser(testutils.User{
		Type:       consentPermsUserType.Name,
		OUID:       ts.ouID,
		Attributes: json.RawMessage(attributes),
	})
	ts.Require().NoError(err, "Failed to create user "+username)
	return userID
}

// authenticateUpToConsent drives the flow to the consent prompt and returns the flow step together
// with the parsed consent prompt purposes.
func (ts *ConsentPermissionsTestSuite) authenticateUpToConsent(
	appID, username string, requestedPermissions string,
) (*common.FlowStep, []consentPromptPurpose) {
	inputs := map[string]string{"applicationId": appID}
	if requestedPermissions != "" {
		inputs["requested_permissions"] = requestedPermissions
		inputs["resource_server_identifier"] = consentPermsResourceServer
	}

	flowStep, err := common.InitiateAuthenticationFlow(appID, false, inputs, "")
	ts.Require().NoError(err, "Failed to initiate the flow")
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus, "Flow should start incomplete")

	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, map[string]string{
		"username": username,
		"password": consentPermsPassword,
	}, "action_001", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit credentials")
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus,
		"Flow should pause on the consent prompt, got assertion %q", flowStep.Assertion)

	promptJSON, ok := flowStep.Data.AdditionalData["consentPrompt"]
	ts.Require().True(ok, "Consent prompt data should be present, got additional data %v",
		flowStep.Data.AdditionalData)

	var purposes []consentPromptPurpose
	ts.Require().NoError(json.Unmarshal([]byte(promptJSON), &purposes), "Failed to parse the consent prompt")
	ts.Require().NotEmpty(purposes, "Consent prompt should carry at least one purpose")

	return flowStep, purposes
}

// submitConsent answers the consent prompt, approving exactly the elements named in approved, and
// returns the resulting flow step. A nil approved map denies everything.
func (ts *ConsentPermissionsTestSuite) submitConsent(
	flowStep *common.FlowStep, purposes []consentPromptPurpose, approved map[string]bool,
) *common.FlowStep {
	decisions := consentDecisions{Approved: approved != nil}
	for _, purpose := range purposes {
		decision := consentPurposeDecision{PurposeName: purpose.PurposeName, Approved: approved != nil}
		for _, element := range append(append([]consentPromptElement{},
			purpose.Essential...), purpose.Optional...) {
			decision.Elements = append(decision.Elements, consentElementDecision{
				Name:     element.Name,
				Approved: approved[element.Name],
			})
		}
		decisions.Purposes = append(decisions.Purposes, decision)
	}

	decisionsJSON, err := json.Marshal(decisions)
	ts.Require().NoError(err, "Failed to marshal the consent decisions")

	action := "consent_action_allow"
	if approved == nil {
		action = "consent_action_deny"
	}

	completed, err := common.CompleteFlow(flowStep.ExecutionID,
		map[string]string{"consent_decisions": string(decisionsJSON)}, action, flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the consent decisions")
	return completed
}

// permissionsClaim returns the authorized_permissions claim of the assertion and whether it is present.
func (ts *ConsentPermissionsTestSuite) permissionsClaim(assertion string) (string, bool) {
	ts.Require().NotEmpty(assertion, "Flow should return an assertion")

	claims, err := testutils.DecodeJWTPayloadMap(assertion)
	ts.Require().NoError(err, "Failed to decode the assertion")

	raw, ok := claims["authorized_permissions"]
	if !ok {
		return "", false
	}
	value, isString := raw.(string)
	ts.Require().True(isString, "authorized_permissions claim should be a string")
	return value, true
}

// TestConsentedSubsetOfAuthorizedPermissions covers case 1: consenting to a subset of the authorized
// permissions narrows the claim to that subset, in the order the user consented to them.
func (ts *ConsentPermissionsTestSuite) TestConsentedSubsetOfAuthorizedPermissions() {
	flowStep, purposes := ts.authenticateUpToConsent(ts.authzAppID, "consent_subset_user", "read write delete")

	completed := ts.submitConsent(flowStep, purposes, map[string]bool{"read": true, "delete": true})
	ts.Require().Equal("COMPLETE", completed.FlowStatus, "Flow should complete after consent")

	permissions, present := ts.permissionsClaim(completed.Assertion)
	ts.Require().True(present, "authorized_permissions claim should be present")
	ts.Equal("read delete", permissions, "Claim should be exactly the consented subset, in order")
}

// TestStaleConsentIsIntersectedWithAuthorizedPermissions covers case 2: a permission that stays in
// the consent record after the user loses it is dropped from the claim, because the consented set is
// intersected with the currently authorized set. The second run also withholds consent for a
// permission the user is authorized for, so neither set on its own can produce the expected claim.
func (ts *ConsentPermissionsTestSuite) TestStaleConsentIsIntersectedWithAuthorizedPermissions() {
	// First run: the user holds read and write and consents to both.
	flowStep, purposes := ts.authenticateUpToConsent(ts.authzAppID, "consent_stale_user", "read write")
	completed := ts.submitConsent(flowStep, purposes, map[string]bool{"read": true, "write": true})
	ts.Require().Equal("COMPLETE", completed.FlowStatus, "First run should complete after consent")

	permissions, present := ts.permissionsClaim(completed.Assertion)
	ts.Require().True(present, "First run should carry the consented permissions")
	ts.Equal("read write", permissions, "First run should carry both consented permissions")

	// Revoke write and grant delete, leaving write approved in the consent record but no longer
	// authorized. Granting delete keeps a non-consented permission in the prompt so the consent step
	// still records a decision on the second run.
	ts.Require().NoError(testutils.UpdateRole(ts.staleRoleID, testutils.Role{
		Name:        "ConsentStaleEditor",
		Description: "Grants the permissions that are later revoked to create a stale consent record",
		OUID:        ts.ouID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: ts.resourceServerID, Permissions: []string{"read", "delete"}},
		},
		Assignments: []testutils.Assignment{
			{ID: ts.staleUserID, Type: "user"},
		},
	}), "Failed to revoke the write permission")

	// Second run: the user is authorized for read and delete, and the consent record holds read and
	// the now revoked write. Denying delete leaves the consented and the authorized sets differing in
	// both directions, so only their intersection, read, can satisfy the assertion.
	flowStep, purposes = ts.authenticateUpToConsent(ts.authzAppID, "consent_stale_user", "read write delete")
	completed = ts.submitConsent(flowStep, purposes, map[string]bool{})
	ts.Require().Equal("COMPLETE", completed.FlowStatus, "Second run should complete after consent")

	permissions, present = ts.permissionsClaim(completed.Assertion)
	ts.Require().True(present, "Second run should carry the intersected permissions")
	ts.Equal("read", permissions,
		"Claim should be the intersection: stale write is not authorized and delete was not consented")
}

// TestConsentDeniedForAllPermissions covers case 3: denying every permission leaves the claim out of
// the assertion even though the user is authorized for all of them.
func (ts *ConsentPermissionsTestSuite) TestConsentDeniedForAllPermissions() {
	flowStep, purposes := ts.authenticateUpToConsent(ts.authzAppID, "consent_deny_all_user", "read write delete")

	completed := ts.submitConsent(flowStep, purposes, nil)
	ts.Require().Equal("COMPLETE", completed.FlowStatus, "Flow should complete after the denial")

	permissions, present := ts.permissionsClaim(completed.Assertion)
	ts.False(present, "authorized_permissions claim should be absent, got %q", permissions)
}

// TestConsentWithoutAuthorizationStep covers case 4: when consent runs in a flow that has no
// authorization step there is no authorized set to intersect against, so the consented set is used
// verbatim. No permission can be consented to in such a flow (the permissions purpose is built from
// the authorized set), so the consented set is empty and the claim is omitted, while the consented
// attributes still reach the assertion, proving the consent step did run.
func (ts *ConsentPermissionsTestSuite) TestConsentWithoutAuthorizationStep() {
	flowStep, purposes := ts.authenticateUpToConsent(ts.noAuthzAppID, "consent_no_authz_user", "")

	for _, purpose := range purposes {
		ts.NotEqual("permissions", purpose.Type,
			"A flow without an authorization step cannot prompt for permissions")
	}

	completed := ts.submitConsent(flowStep, purposes, map[string]bool{
		"email": true, "given_name": true, "family_name": true,
	})
	ts.Require().Equal("COMPLETE", completed.FlowStatus, "Flow should complete after consent")

	claims, err := testutils.DecodeJWTPayloadMap(completed.Assertion)
	ts.Require().NoError(err, "Failed to decode the assertion")
	ts.Equal("consent_no_authz_user@test.com", claims["email"], "Consented attributes should be released")

	_, present := claims["authorized_permissions"]
	ts.False(present, "authorized_permissions claim should be absent, got claims %v", claims)
}
