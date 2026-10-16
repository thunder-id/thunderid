// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// LifecycleBoundaryTestSuite checks who may run suspend, unsuspend and unlock, on whom and from
// which flow. Each refusal checks the answer that refused it, that the target's state is unchanged and
// that its tokens still work. The cross-unit and undecidable-authorization branches are
// unit-tested in identitygovernance: the administration gate admits only root, which is outside every
// boundary, so no caller a test can mint reaches them. It runs under the shipped lockout policy.
type LifecycleBoundaryTestSuite struct {
	suite.Suite
	suffix        string
	ouA, ouB      string
	typeA, typeB  string
	authFlowID    string
	appID         string
	roleIDs       []string
	scopedRSID    string
	flowIDs       []string
	userIDs       []string
	rootToken     string
	rootID        string
	rootPassword  string
	rootUsername  string
	scopedToken   string
	recoveryAppID string
}

func TestLifecycleBoundaryTestSuite(t *testing.T) {
	suite.Run(t, new(LifecycleBoundaryTestSuite))
}

const (
	boundaryClientID     = "governance_boundary_client"
	boundaryClientSecret = "governance_boundary_secret" //nolint:gosec // test credential
	boundaryRedirectURI  = "https://localhost:3000/callback"
	boundaryConsoleURI   = "https://localhost:8095/console"

	// The bootstrap organization unit and system resource server: the root permission lives there.
	bootstrapOUID           = "01900000-0000-7000-8000-000000000001"
	systemResourceServerID  = "01900000-0000-7000-8000-000000000020"
	boundaryScopedRSHandle  = "https://boundary-test.example.com/system"
	boundaryRecoveryHandle  = "governance-boundary-recovery-suspend"
	boundaryNoPlanHandle    = "governance-boundary-suspend-without-plan"
	boundaryAuthFlowHandle  = "governance-boundary-auth"
	boundaryRecoveryAppName = "Governance Boundary Recovery App"

	// governanceRefusalCode is the one failure a refused lifecycle step answers with.
	governanceRefusalCode = "FET-1112"
	// adminAuthenticationRequiredCode is the administration gate's answer to a caller without root.
	// /flow/execute is a public path, so the middleware drops a token that fails its root check and
	// the gate sees an anonymous caller.
	adminAuthenticationRequiredCode = "FES-1017"
)

// governanceRefusal is the answer a lifecycle step gives when the authority refuses it.
var governanceRefusal = flowAnswer{status: http.StatusOK, flowStatus: "ERROR", code: governanceRefusalCode}

// rootRequired is the answer the administration gate gives a caller without the root permission.
var rootRequired = flowAnswer{status: http.StatusUnauthorized, code: adminAuthenticationRequiredCode}

// flowAnswer is what one flow execution answered: the HTTP status, the flow status if a step ran, and
// the error code from either the step or the engine.
type flowAnswer struct {
	status     int
	flowStatus string
	code       string
}

func (ts *LifecycleBoundaryTestSuite) SetupSuite() {
	ts.suffix = uniqueSuffix()

	ts.ouA = ts.createOU("a")
	ts.ouB = ts.createOU("b")
	ts.typeA = ts.createUserType("a", ts.ouA)
	ts.typeB = ts.createUserType("b", ts.ouB)

	// Only the recovery application uses this one: a custom recovery flow beside the default
	// authentication flow fails cross-type reference validation.
	authFlowID, err := testutils.CreateIsolatedAuthFlow(boundaryAuthFlowHandle + "-" + ts.suffix)
	ts.Require().NoError(err, "failed to create the authentication flow")
	ts.authFlowID = authFlowID
	ts.flowIDs = append(ts.flowIDs, authFlowID)

	appID, err := testutils.CreateApplication(testutils.Application{
		OUID:             ts.ouA,
		Name:             "Governance Boundary Test App",
		Description:      "Application the lifecycle boundary tests sign targets in through",
		AllowedUserTypes: []string{ts.typeHandle("a"), ts.typeHandle("b")},
		InboundAuthConfig: []map[string]interface{}{{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                boundaryClientID,
				"clientSecret":            boundaryClientSecret,
				"redirectUris":            []string{boundaryRedirectURI},
				"grantTypes":              []string{"authorization_code", "refresh_token"},
				"responseTypes":           []string{"code"},
				"tokenEndpointAuthMethod": "client_secret_post",
				"scopes":                  []string{"openid"},
			},
		}},
	})
	ts.Require().NoError(err, "failed to create the test application")
	ts.appID = appID

	// A second root operator, so a self-targeting bug suspends a throwaway account rather than the
	// administrator every other suite runs as.
	ts.rootID, ts.rootUsername, ts.rootPassword = ts.newUser("root", "a")
	roleID, err := testutils.CreateRole(testutils.Role{
		Name: "Boundary Root " + ts.suffix,
		OUID: bootstrapOUID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: systemResourceServerID, Permissions: []string{"system"}},
		},
		Assignments: []testutils.Assignment{{ID: ts.rootID, Type: "user"}},
	})
	ts.Require().NoError(err, "failed to grant the root permission")
	ts.roleIDs = append(ts.roleIDs, roleID)
	root, err := testutils.ObtainAccessTokenWithPassword("CONSOLE", boundaryConsoleURI, "system",
		ts.rootUsername, ts.rootPassword, true)
	ts.Require().NoError(err, "the root operator must sign in to the console")
	ts.rootToken = root.AccessToken

	// An operator holding user management in unit A only, the closest thing to a delegated
	// administrator the product can express.
	scopedID, scopedUsername, scopedPassword := ts.newUser("scoped", "a")
	rsID, err := testutils.CreateSystemScopedResourceServer(ts.ouA, "Governance Boundary RS "+ts.suffix,
		boundaryScopedRSHandle, "user")
	ts.Require().NoError(err, "failed to create the scoped resource server")
	ts.scopedRSID = rsID
	roleID, err = testutils.CreateRole(testutils.Role{
		Name: "Boundary User Manager " + ts.suffix,
		OUID: ts.ouA,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: rsID, Permissions: []string{"system:user", "system:user:view"}},
		},
		Assignments: []testutils.Assignment{{ID: scopedID, Type: "user"}},
	})
	ts.Require().NoError(err, "failed to grant user management")
	ts.roleIDs = append(ts.roleIDs, roleID)
	scoped, err := testutils.ObtainAccessTokenWithPassword("CONSOLE", boundaryConsoleURI,
		"system:user system:user:view", scopedUsername, scopedPassword, true, "", boundaryScopedRSHandle)
	ts.Require().NoError(err, "the scoped operator must sign in to the console")
	ts.scopedToken = scoped.AccessToken
}

func (ts *LifecycleBoundaryTestSuite) TearDownSuite() {
	for _, id := range []string{ts.recoveryAppID, ts.appID} {
		if id != "" {
			if err := testutils.DeleteApplication(id); err != nil {
				ts.T().Logf("teardown: failed to delete application %s: %v", id, err)
			}
		}
	}
	for _, id := range ts.roleIDs {
		if err := testutils.DeleteRole(id); err != nil {
			ts.T().Logf("teardown: failed to delete role %s: %v", id, err)
		}
	}
	if ts.scopedRSID != "" {
		if err := testutils.DeleteResourceServerWithChildren(ts.scopedRSID); err != nil {
			ts.T().Logf("teardown: failed to delete resource server: %v", err)
		}
	}
	for _, id := range ts.userIDs {
		if err := testutils.DeleteUser(id); err != nil {
			ts.T().Logf("teardown: failed to delete user %s: %v", id, err)
		}
	}
	for _, id := range ts.flowIDs {
		if err := testutils.DeleteFlow(id); err != nil {
			ts.T().Logf("teardown: failed to delete flow %s: %v", id, err)
		}
	}
	for _, id := range []string{ts.typeA, ts.typeB} {
		if id != "" {
			if err := testutils.DeleteUserType(id); err != nil {
				ts.T().Logf("teardown: failed to delete user type %s: %v", id, err)
			}
		}
	}
	for _, id := range []string{ts.ouA, ts.ouB} {
		if id != "" {
			if err := testutils.DeleteOrganizationUnit(id); err != nil {
				ts.T().Logf("teardown: failed to delete organization unit %s: %v", id, err)
			}
		}
	}
}

// An operator cannot run a lifecycle operation on themselves, even with root, and a refused
// suspend revokes nothing they hold. The refusal is the authority's, after the permission check passed.
func (ts *LifecycleBoundaryTestSuite) TestAnOperatorCannotGovernThemselves() {
	own := ts.signIn(ts.rootUsername, ts.rootPassword)

	for _, handle := range []string{suspendFlowHandle, unlockFlowHandle} {
		ts.refused(ts.rootToken, handle, ts.rootID, governanceRefusal, "self-target: %s must be refused", handle)
	}

	ts.Equal("ACTIVE", readStatus(ts.T(), ts.rootID).Value, "a refused self-suspend must hold nothing")
	ts.intact(own, "a refused self-suspend must revoke nothing")
}

// Administration is root-only, so an operator holding user management in one unit is
// refused at the administration gate for a target in another unit and in their own, and revokes
// nothing in either.
func (ts *LifecycleBoundaryTestSuite) TestAnOperatorWithoutRootIsRefusedInAndOutsideTheirUnit() {
	for _, unit := range []string{"a", "b"} {
		targetID, username, password := ts.newUser("scoped_target_"+unit, unit)
		held := ts.signIn(username, password)

		for _, handle := range []string{suspendFlowHandle, unlockFlowHandle} {
			ts.refused(ts.scopedToken, handle, targetID, rootRequired,
				"unit %s: %s must be refused without root", unit, handle)
		}

		ts.Equal("ACTIVE", readStatus(ts.T(), targetID).Value, "unit %s: nothing may be held", unit)
		ts.intact(held, fmt.Sprintf("unit %s: a refused suspend must revoke nothing", unit))
	}
}

// Owning the account does not authorize a lifecycle operation on it. A locked holder signed in
// to the Console with no permission cannot unlock, suspend or unsuspend themselves: the
// administration gate refuses each, the lock stays, and the holder's own grant is untouched.
func (ts *LifecycleBoundaryTestSuite) TestOwningTheAccountDoesNotAuthorizeItsLifecycle() {
	ownerID, username, password := ts.newUser("owner", "a")
	held := ts.signIn(username, password)
	console, err := testutils.ObtainAccessTokenWithPassword("CONSOLE", boundaryConsoleURI, "openid",
		username, password, true)
	ts.Require().NoError(err, "the holder must be able to sign in to the console")

	failAuthentication(ts.T(), "username", username, shippedUserLockThreshold)
	ts.Require().Equal("LOCKED", readStatus(ts.T(), ownerID).Value, "the holder must be locked first")

	for _, handle := range []string{unlockFlowHandle, suspendFlowHandle, unsuspendFlowHandle} {
		ts.refused(console.AccessToken, handle, ownerID, rootRequired, "the holder's own %s", handle)
	}

	ts.Equal("LOCKED", readStatus(ts.T(), ownerID).Value, "the holder must not have released their lock")
	ts.intact(held, "a refused self-service suspend must revoke nothing")
}

// The cycle in order: suspend revokes, unlock does not release a suspension,
// unsuspend lands a user on a lock, a second unsuspend does not apply, and unlock returns the account.
func (ts *LifecycleBoundaryTestSuite) TestTheLifecycleRunsInOrder() {
	targetID, username, password := ts.newUser("cycle", "a")
	held := ts.signIn(username, password)

	ts.run(suspendFlowHandle, targetID)
	ts.Equal("SUSPENDED", readStatus(ts.T(), targetID).Value)
	ts.False(ts.active(held.AccessToken), "suspend must revoke the access token")

	ts.run(unlockFlowHandle, targetID)
	ts.Equal("SUSPENDED", readStatus(ts.T(), targetID).Value, "unlock must not release a suspension")

	ts.run(unsuspendFlowHandle, targetID)
	assertTheRecoveryLock(ts.T(), readStatus(ts.T(), targetID), "unsuspended")
	ts.refused(ts.rootToken, unsuspendFlowHandle, targetID, governanceRefusal,
		"unsuspend must not apply to an unsuspended user")
	assertTheRecoveryLock(ts.T(), readStatus(ts.T(), targetID), "a refused unsuspend changes nothing")

	ts.run(unlockFlowHandle, targetID)
	ts.Equal("ACTIVE", readStatus(ts.T(), targetID).Value, "unlock must return the account")
}

// Suspend and unsuspend fault outside an administration flow, and change nothing. A
// recovery flow carrying either node, handed a target, ends in a server fault. For unsuspend that
// fault is the flow-type guard's alone: without it the node would look for recorded proof, find none
// and answer with an ordinary refusal. For suspend the missing pre-suspend plan faults the node as
// well, so the guard itself is pinned by the executor's unit test.
func (ts *LifecycleBoundaryTestSuite) TestSuspendFaultsOutsideAnAdministrationFlow() {
	flowID, err := testutils.CreateFlow(buildRecoverySuspendFlow())
	ts.Require().NoError(err, "a recovery flow with lifecycle nodes is creatable; the nodes refuse at run time")
	ts.flowIDs = append(ts.flowIDs, flowID)
	appID, err := testutils.CreateApplication(testutils.Application{
		OUID:                  ts.ouA,
		Name:                  boundaryRecoveryAppName,
		Description:           "Application whose recovery flow carries suspend and unsuspend nodes",
		IsRecoveryFlowEnabled: true,
		ClientID:              "governance_boundary_recovery_client",
		ClientSecret:          "governance_boundary_recovery_secret",
		RedirectURIs:          []string{boundaryRedirectURI},
		AllowedUserTypes:      []string{ts.typeHandle("a")},
		AuthFlowID:            ts.authFlowID,
		RecoveryFlowID:        flowID,
	})
	ts.Require().NoError(err, "failed to create the recovery application")
	ts.recoveryAppID = appID

	activeID, username, password := ts.newUser("recovery_target", "a")
	held := ts.signIn(username, password)
	answer := ts.recoverWith(appID, activeID, "action_submit_subject")
	ts.Equal(http.StatusInternalServerError, answer.status, "a suspend node in a recovery flow must fault")
	ts.Equal("ACTIVE", readStatus(ts.T(), activeID).Value, "a recovery flow must not suspend anyone")
	ts.intact(held, "a recovery flow must revoke nothing")

	suspendedID, _, _ := ts.newUser("recovery_suspended", "a")
	ts.run(suspendFlowHandle, suspendedID)
	answer = ts.recoverWith(appID, suspendedID, "action_unsuspend_subject")
	ts.Equal(http.StatusInternalServerError, answer.status, "an unsuspend node in a recovery flow must fault")
	ts.Equal("SUSPENDED", readStatus(ts.T(), suspendedID).Value, "a recovery flow must not unsuspend anyone")
}

// recoverWith starts the suite's recovery flow and submits a target through the given action.
func (ts *LifecycleBoundaryTestSuite) recoverWith(appID, subject, action string) flowAnswer {
	ts.T().Helper()
	step, err := common.InitiateRecoveryFlow(appID, false, nil, "")
	ts.Require().NoError(err)
	ts.Require().Equal("INCOMPLETE", step.FlowStatus)
	return ts.execute("", map[string]interface{}{
		"executionId":    step.ExecutionID,
		"inputs":         map[string]string{subjectInput: subject},
		"action":         action,
		"challengeToken": step.ChallengeToken,
	})
}

// A suspend node with no pre-suspend plan before it faults and holds nothing: the plan is the
// only place its target can come from.
func (ts *LifecycleBoundaryTestSuite) TestASuspendNodeWithoutAPlanHoldsNothing() {
	flowID, err := testutils.CreateFlow(buildSuspendFlowWithoutPlan())
	ts.Require().NoError(err, "failed to create the plan-free suspend flow")
	ts.flowIDs = append(ts.flowIDs, flowID)
	targetID, username, password := ts.newUser("no_plan", "a")
	held := ts.signIn(username, password)

	answer := ts.execute(ts.rootToken, map[string]interface{}{
		"flowId": flowID, "inputs": map[string]string{subjectInput: targetID},
	})

	ts.Equal(http.StatusInternalServerError, answer.status, "a suspend node without a plan must fault")
	ts.Equal("ACTIVE", readStatus(ts.T(), targetID).Value, "nothing may be held without a plan")
	ts.intact(held, "a faulted suspend without containment must revoke nothing")
}

// refused runs a shipped flow as the given operator and requires the given refusal.
func (ts *LifecycleBoundaryTestSuite) refused(token, handle, subject string, want flowAnswer, msg string,
	args ...interface{}) {
	ts.T().Helper()
	flowID, err := testutils.GetFlowIDByHandle(handle, administrationFlowType)
	ts.Require().NoError(err)
	got := ts.execute(token, map[string]interface{}{
		"flowId": flowID, "inputs": map[string]string{subjectInput: subject},
	})
	ts.Equal(want, got, append([]interface{}{msg}, args...)...)
}

// execute posts one flow request, with the bearer token if one is given, and reads its answer.
func (ts *LifecycleBoundaryTestSuite) execute(token string, body map[string]interface{}) flowAnswer {
	ts.T().Helper()
	payload, err := json.Marshal(body)
	ts.Require().NoError(err)
	req, err := http.NewRequest(http.MethodPost, testServerURL+"/flow/execute", bytes.NewReader(payload))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err, "the flow request must reach the server")
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)

	var read struct {
		FlowStatus string `json:"flowStatus"`
		Code       string `json:"code"`
		Error      *struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(raw, &read)
	answer := flowAnswer{status: resp.StatusCode, flowStatus: read.FlowStatus, code: read.Code}
	if read.Error != nil {
		answer.code = read.Error.Code
	}
	return answer
}

// run runs a shipped flow as this suite's root operator and requires it to complete.
func (ts *LifecycleBoundaryTestSuite) run(handle, subject string) {
	ts.T().Helper()
	status, flowStatus, err := testutils.RunAdministrationFlowWithToken(ts.rootToken, handle,
		map[string]string{subjectInput: subject})
	ts.Require().NoError(err, "failed to execute %s", handle)
	ts.Require().Equal(http.StatusOK, status, "%s was refused at the transport", handle)
	ts.Require().Equal("COMPLETE", flowStatus, "%s did not complete", handle)
}

// intact requires the access a refusal must not have touched to still work.
func (ts *LifecycleBoundaryTestSuite) intact(token *testutils.TokenResponse, msg string) {
	ts.T().Helper()
	ts.True(ts.active(token.AccessToken), "%s: the access token", msg)
	ts.Equal(http.StatusOK, ts.post("/oauth2/token", url.Values{
		"grant_type": {"refresh_token"}, "refresh_token": {token.RefreshToken},
	}).StatusCode, "%s: the refresh grant", msg)
}

// signIn obtains an access token and refresh grant through the suite application.
func (ts *LifecycleBoundaryTestSuite) signIn(username, password string) *testutils.TokenResponse {
	ts.T().Helper()
	token, err := testutils.ObtainAccessTokenWithPassword(boundaryClientID, boundaryRedirectURI, "openid",
		username, password, false, boundaryClientSecret)
	ts.Require().NoError(err, "failed to sign in %s", username)
	ts.Require().NotEmpty(token.RefreshToken, "the sign-in must produce a refresh grant")
	return token
}

// active introspects a token as the suite application.
func (ts *LifecycleBoundaryTestSuite) active(token string) bool {
	ts.T().Helper()
	resp := ts.post("/oauth2/introspect", url.Values{"token": {token}})
	defer resp.Body.Close()
	var result struct {
		Active bool `json:"active"`
	}
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&result))
	return result.Active
}

// post sends a form to an OAuth endpoint, authenticating as the suite application in the body.
func (ts *LifecycleBoundaryTestSuite) post(path string, form url.Values) *http.Response {
	ts.T().Helper()
	form.Set("client_id", boundaryClientID)
	form.Set("client_secret", boundaryClientSecret)
	req, err := http.NewRequest(http.MethodPost, testServerURL+path, strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// newUser creates an account in unit a or b that nobody else touches.
func (ts *LifecycleBoundaryTestSuite) newUser(name, unit string) (userID, username, password string) {
	ts.T().Helper()
	ou, userType := ts.ouA, ts.typeHandle("a")
	if unit == "b" {
		ou, userType = ts.ouB, ts.typeHandle("b")
	}
	username = fmt.Sprintf("boundary_%s_%s", name, ts.suffix)
	password = "Correct-Horse-" + name + "1!"
	attributes, err := json.Marshal(map[string]interface{}{
		"username": username, "password": password, "email": username + "@example.com",
	})
	ts.Require().NoError(err)
	userID, err = testutils.CreateUser(testutils.User{Type: userType, OUID: ou, Attributes: attributes})
	ts.Require().NoError(err, "failed to create %s", username)
	ts.userIDs = append(ts.userIDs, userID)
	return userID, username, password
}

func (ts *LifecycleBoundaryTestSuite) createOU(unit string) string {
	ts.T().Helper()
	id, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "governance_boundary_ou_" + unit + "_" + ts.suffix,
		Name:        "Governance Boundary OU " + strings.ToUpper(unit),
		Description: "Organization unit for the lifecycle boundary tests",
	})
	ts.Require().NoError(err, "failed to create organization unit %s", unit)
	return id
}

func (ts *LifecycleBoundaryTestSuite) createUserType(unit, ouID string) string {
	ts.T().Helper()
	id, err := testutils.CreateUserType(testutils.UserType{
		Handle:      ts.typeHandle(unit),
		DisplayName: ts.typeHandle(unit),
		OUID:        ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
			"email":    map[string]interface{}{"type": "string"},
		},
	})
	ts.Require().NoError(err, "failed to create user type %s", unit)
	return id
}

func (ts *LifecycleBoundaryTestSuite) typeHandle(unit string) string {
	return "governance_boundary_user_" + unit + "_" + ts.suffix
}

// buildRecoverySuspendFlow is a recovery flow that asks for a subject and runs a suspend or an
// unsuspend node on it, by the action submitted.
func buildRecoverySuspendFlow() testutils.Flow {
	return testutils.Flow{
		Name:     "Governance Boundary Recovery Suspend Flow",
		Handle:   boundaryRecoveryHandle + "-" + uniqueSuffix(),
		FlowType: "RECOVERY",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "prompt_subject"},
			{
				"id":   "prompt_subject",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{{
							"ref": "input_subject", "identifier": subjectInput, "type": "TEXT_INPUT", "required": true,
						}},
						"action": map[string]interface{}{
							"ref": "action_submit_subject", "nextNode": "access_governance",
						},
					},
					{
						"inputs": []map[string]interface{}{{
							"ref": "input_unsuspend_subject", "identifier": subjectInput, "type": "TEXT_INPUT",
							"required": true,
						}},
						"action": map[string]interface{}{"ref": "action_unsuspend_subject", "nextNode": "release_hold"},
					},
				},
			},
			{
				"id":        "access_governance",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "IdentityGovernanceExecutor", "mode": "suspend"},
				"onSuccess": "end",
			},
			{
				"id":        "release_hold",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "IdentityGovernanceExecutor", "mode": "unsuspend"},
				"onSuccess": "end",
			},
			{"id": "end", "type": "END"},
		},
	}
}

// buildSuspendFlowWithoutPlan is the shipped user suspend flow without its pre-suspend and
// containment nodes.
func buildSuspendFlowWithoutPlan() testutils.Flow {
	return testutils.Flow{
		Name:     "Governance Boundary Suspend Without Plan",
		Handle:   boundaryNoPlanHandle + "-" + uniqueSuffix(),
		FlowType: administrationFlowType,
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "permission_validator"},
			{
				"id":         "permission_validator",
				"type":       "TASK_EXECUTION",
				"properties": map[string]interface{}{"requiredScopes": []string{"system:user"}},
				"executor":   map[string]interface{}{"name": "PermissionValidator"},
				"onSuccess":  "access_governance",
			},
			{
				"id":        "access_governance",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "IdentityGovernanceExecutor", "mode": "suspend"},
				"onSuccess": "end",
			},
			{"id": "end", "type": "END"},
		},
	}
}
