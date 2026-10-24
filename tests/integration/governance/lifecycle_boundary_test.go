// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	boundaryClientID     = "governance_boundary_client"
	boundaryClientSecret = "governance_boundary_secret" //nolint:gosec // test credential
	boundaryRedirectURI  = "https://localhost:3000/callback"

	boundaryScopedRSHandle  = "https://boundary-test.example.com/system"
	boundaryRecoveryHandle  = "governance-boundary-recovery-suspend"
	boundaryNoPlanHandle    = "governance-boundary-suspend-without-plan"
	boundaryAuthFlowHandle  = "governance-boundary-auth"
	boundaryRecoveryAppName = "Governance Boundary Recovery App"

	// The actions the boundary suite's recovery flow submits a subject with.
	submitSubjectAction    = "action_submit_subject"
	unsuspendSubjectAction = "action_unsuspend_subject"

	// adminAuthenticationRequiredCode is the administration gate's answer to a caller without root.
	// /flow/execute is a public path, so the middleware drops a token that fails its root check and
	// the gate sees an anonymous caller.
	adminAuthenticationRequiredCode = "FES-1017"
)

// rootRequired is the answer the administration gate gives a caller without the root permission.
var rootRequired = flowAnswer{status: http.StatusUnauthorized, code: adminAuthenticationRequiredCode}

// LifecycleBoundaryTestSuite checks who may run suspend, unsuspend and unlock, on whom and from
// which flow. Each refusal checks the answer that refused it, that the target's state is unchanged and
// that its tokens still work. The cross-unit and undecidable-authorization branches are
// unit-tested in identitygovernance: the administration gate admits only root, which is outside every
// boundary, so no caller a test can mint reaches them. It runs under the shipped lockout policy.
type LifecycleBoundaryTestSuite struct {
	suite.Suite
	unitA, unitB *userFixture
	authFlowID   string
	appID        string
	rootToken    string
	rootID       string
	rootPassword string
	rootUsername string
	scopedToken  string
}

func TestLifecycleBoundaryTestSuite(t *testing.T) {
	suite.Run(t, new(LifecycleBoundaryTestSuite))
}

func (ts *LifecycleBoundaryTestSuite) SetupSuite() {
	t := ts.T()
	schema := testutils.UserType{Schema: credentialsSchema(map[string]interface{}{"email": stringAttribute})}
	ts.unitA = newUserFixture(t, "boundary_a", schema)
	ts.unitB = newUserFixture(t, "boundary_b", schema)

	// Only the recovery application uses this one: a custom recovery flow beside the default
	// authentication flow fails cross-type reference validation.
	authFlowID, err := testutils.CreateIsolatedAuthFlow(boundaryAuthFlowHandle + "-" + ts.unitA.suffix)
	ts.Require().NoError(err, "failed to create the authentication flow")
	cleanupFlow(t, authFlowID)
	ts.authFlowID = authFlowID

	appID, err := testutils.CreateApplication(testutils.Application{
		OUID:             ts.unitA.ouID,
		Name:             "Governance Boundary Test App",
		Description:      "Application the lifecycle boundary tests sign targets in through",
		AllowedUserTypes: []string{ts.unitA.userType, ts.unitB.userType},
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
	cleanupApplication(t, appID)
	ts.appID = appID

	// A second root operator, so a self-targeting bug suspends a throwaway account rather than the
	// administrator every other suite runs as.
	ts.rootID, ts.rootUsername, ts.rootPassword, ts.rootToken = newRootOperator(t, ts.unitA)

	// An operator holding user management in unit A only, the closest thing to a delegated
	// administrator the product can express.
	scopedID, scopedUsername, scopedPassword := ts.unitA.newUser(t, "scoped", nil)
	rsID, err := testutils.CreateSystemScopedResourceServer(ts.unitA.ouID,
		"Governance Boundary RS "+ts.unitA.suffix, boundaryScopedRSHandle, "user")
	ts.Require().NoError(err, "failed to create the scoped resource server")
	t.Cleanup(func() {
		if err := testutils.DeleteResourceServerWithChildren(rsID); err != nil {
			t.Logf("teardown: failed to delete resource server: %v", err)
		}
	})
	roleID, err := testutils.CreateRole(testutils.Role{
		Name: "Boundary User Manager " + ts.unitA.suffix,
		OUID: ts.unitA.ouID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: rsID, Permissions: []string{"system:user", "system:user:view"}},
		},
		Assignments: []testutils.Assignment{{ID: scopedID, Type: "user"}},
	})
	ts.Require().NoError(err, "failed to grant user management")
	cleanupRole(t, roleID)
	scoped, err := testutils.ObtainAccessTokenWithPassword("CONSOLE", boundaryConsoleURI,
		"system:user system:user:view", scopedUsername, scopedPassword, true, "", boundaryScopedRSHandle)
	ts.Require().NoError(err, "the scoped operator must sign in to the console")
	ts.scopedToken = scoped.AccessToken
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
// nothing in either. The target is locked first, so a refused unlock is seen to release nothing.
func (ts *LifecycleBoundaryTestSuite) TestAnOperatorWithoutRootIsRefusedInAndOutsideTheirUnit() {
	for _, target := range []struct {
		name string
		unit *userFixture
	}{{"a", ts.unitA}, {"b", ts.unitB}} {
		targetID, username, password := target.unit.newUser(ts.T(), "scoped_target", nil)
		held := ts.signIn(username, password)
		failAuthentication(ts.T(), "username", username, shippedUserLockThreshold)
		ts.Require().Equal("LOCKED", readStatus(ts.T(), targetID).Value,
			"unit %s: the target must be locked first", target.name)

		for _, handle := range []string{suspendFlowHandle, unlockFlowHandle} {
			ts.refused(ts.scopedToken, handle, targetID, rootRequired,
				"unit %s: %s must be refused without root", target.name, handle)
		}

		ts.Equal("LOCKED", readStatus(ts.T(), targetID).Value, "unit %s: the lock must stay", target.name)
		ts.intact(held, fmt.Sprintf("unit %s: a refused suspend must revoke nothing", target.name))
	}
}

// An unlock started without a subject asks for one and releases nothing.
func (ts *LifecycleBoundaryTestSuite) TestAnUnlockWithoutASubjectAsksForOne() {
	lockedID, username, _ := ts.unitA.newUser(ts.T(), "no_subject", nil)
	failAuthentication(ts.T(), "username", username, shippedUserLockThreshold)
	ts.Require().Equal("LOCKED", readStatus(ts.T(), lockedID).Value, "the account must be locked first")

	status, raw := executeAdministrationFlowAs(ts.T(), ts.rootToken, unlockFlowHandle, map[string]string{})
	step := flowStepOf(ts.T(), status, raw)

	ts.Equal("INCOMPLETE", step.FlowStatus, "an unlock without a subject must ask for one: %s", raw)
	ts.True(common.HasInput(step.Data.Inputs, subjectInput), "the flow must ask for the subject: %s", raw)
	ts.Equal("LOCKED", readStatus(ts.T(), lockedID).Value, "nothing may be released without a subject")
}

// Owning the account does not authorize a lifecycle operation on it. A locked holder signed in
// to the Console with no permission cannot unlock, suspend or unsuspend themselves: the
// administration gate refuses each, the lock stays, and the holder's own grant is untouched.
func (ts *LifecycleBoundaryTestSuite) TestOwningTheAccountDoesNotAuthorizeItsLifecycle() {
	ownerID, username, password := ts.unitA.newUser(ts.T(), "owner", nil)
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
	targetID, username, password := ts.unitA.newUser(ts.T(), "cycle", nil)
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
func (ts *LifecycleBoundaryTestSuite) TestSuspendAndUnsuspendFaultOutsideAnAdministrationFlow() {
	flowID, err := testutils.CreateFlow(buildRecoverySuspendFlow())
	ts.Require().NoError(err, "a recovery flow with lifecycle nodes is creatable; the nodes refuse at run time")
	cleanupFlow(ts.T(), flowID)
	appID, err := testutils.CreateApplication(testutils.Application{
		OUID:                  ts.unitA.ouID,
		Name:                  boundaryRecoveryAppName,
		Description:           "Application whose recovery flow carries suspend and unsuspend nodes",
		IsRecoveryFlowEnabled: true,
		ClientID:              "governance_boundary_recovery_client",
		ClientSecret:          "governance_boundary_recovery_secret",
		RedirectURIs:          []string{boundaryRedirectURI},
		AllowedUserTypes:      []string{ts.unitA.userType},
		AuthFlowID:            ts.authFlowID,
		RecoveryFlowID:        flowID,
	})
	ts.Require().NoError(err, "failed to create the recovery application")
	cleanupApplication(ts.T(), appID)

	activeID, username, password := ts.unitA.newUser(ts.T(), "recovery_target", nil)
	held := ts.signIn(username, password)
	ts.Equal(serverFault, ts.recoverWith(appID, activeID, submitSubjectAction),
		"a suspend node in a recovery flow must fault")
	ts.Equal("ACTIVE", readStatus(ts.T(), activeID).Value, "a recovery flow must not suspend anyone")
	ts.intact(held, "a recovery flow must revoke nothing")

	suspendedID, _, _ := ts.unitA.newUser(ts.T(), "recovery_suspended", nil)
	ts.run(suspendFlowHandle, suspendedID)
	ts.Equal(serverFault, ts.recoverWith(appID, suspendedID, unsuspendSubjectAction),
		"an unsuspend node in a recovery flow must fault")
	ts.Equal("SUSPENDED", readStatus(ts.T(), suspendedID).Value, "a recovery flow must not unsuspend anyone")
}

// recoverWith starts the suite's recovery flow and submits a target through the given action.
func (ts *LifecycleBoundaryTestSuite) recoverWith(appID, subject, action string) flowAnswer {
	ts.T().Helper()
	step, err := common.InitiateRecoveryFlow(appID, false, nil, "")
	ts.Require().NoError(err)
	ts.Require().Equal("INCOMPLETE", step.FlowStatus)
	return answerOf(postFlow(ts.T(), nil, "", map[string]interface{}{
		"executionId":    step.ExecutionID,
		"inputs":         map[string]string{subjectInput: subject},
		"action":         action,
		"challengeToken": step.ChallengeToken,
	}))
}

// A suspend node with no pre-suspend plan before it faults and holds nothing: the plan is the
// only place its target can come from.
func (ts *LifecycleBoundaryTestSuite) TestASuspendNodeWithoutAPlanHoldsNothing() {
	flowID, err := testutils.CreateFlow(buildSuspendFlowWithoutPlan())
	ts.Require().NoError(err, "failed to create the plan-free suspend flow")
	cleanupFlow(ts.T(), flowID)
	targetID, username, password := ts.unitA.newUser(ts.T(), "no_plan", nil)
	held := ts.signIn(username, password)

	answer := answerOf(postFlow(ts.T(), nil, ts.rootToken, map[string]interface{}{
		"flowId": flowID, "inputs": map[string]string{subjectInput: targetID},
	}))

	ts.Equal(serverFault, answer, "a suspend node without a plan must fault")
	ts.Equal("ACTIVE", readStatus(ts.T(), targetID).Value, "nothing may be held without a plan")
	ts.intact(held, "a faulted suspend without containment must revoke nothing")
}

// refused runs a shipped flow as the given operator and requires the given refusal.
func (ts *LifecycleBoundaryTestSuite) refused(token, handle, subject string, want flowAnswer, msg string,
	args ...interface{}) {
	ts.T().Helper()
	got := attemptAdministrationFlowAs(ts.T(), token, handle, map[string]string{subjectInput: subject})
	ts.Equal(want, got, append([]interface{}{msg}, args...)...)
}

// run runs a shipped flow as this suite's root operator and requires it to complete.
func (ts *LifecycleBoundaryTestSuite) run(handle, subject string) {
	ts.T().Helper()
	runAdministrationFlowAs(ts.T(), ts.rootToken, handle, map[string]string{subjectInput: subject})
}

// intact requires the access a refusal must not have touched to still work.
func (ts *LifecycleBoundaryTestSuite) intact(token *testutils.TokenResponse, msg string) {
	ts.T().Helper()
	ts.True(ts.active(token.AccessToken), "%s: the access token", msg)
	_, err := testutils.RefreshAccessTokenWithClientCredentialsInBody(boundaryClientID, boundaryClientSecret,
		token.RefreshToken)
	ts.NoError(err, "%s: the refresh grant", msg)
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

// active introspects a token as the suite application, which authenticates in the request body.
func (ts *LifecycleBoundaryTestSuite) active(token string) bool {
	ts.T().Helper()
	result, err := testutils.IntrospectTokenResult(token, boundaryClientID, boundaryClientSecret, true)
	ts.Require().NoError(err, "introspection itself must answer")
	return result.Active
}

// buildRecoverySuspendFlow is a recovery flow that asks for a subject and runs a suspend or an
// unsuspend node on it, by the action submitted.
func buildRecoverySuspendFlow() testutils.Flow {
	subject := flowInput("input_subject", subjectInput, "TEXT_INPUT")
	prompt := promptNode("prompt_subject", submitSubjectAction, "access_governance", subject)
	prompt["prompts"] = append(prompt["prompts"].([]map[string]interface{}), map[string]interface{}{
		"inputs": []map[string]interface{}{flowInput("input_unsuspend_subject", subjectInput, "TEXT_INPUT")},
		"action": map[string]interface{}{"ref": unsuspendSubjectAction, "nextNode": "release_hold"},
	})
	return testutils.Flow{
		Name:     "Governance Boundary Recovery Suspend Flow",
		Handle:   boundaryRecoveryHandle + "-" + uniqueSuffix(),
		FlowType: "RECOVERY",
		Nodes: []map[string]interface{}{
			startNode("prompt_subject"),
			prompt,
			taskNode("access_governance", "IdentityGovernanceExecutor", "suspend", "end", nil),
			taskNode("release_hold", "IdentityGovernanceExecutor", "unsuspend", "end", nil),
			endNode(),
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
			startNode("permission_validator"),
			taskNode("permission_validator", "PermissionValidator", "", "access_governance", map[string]interface{}{
				"properties": map[string]interface{}{"requiredScopes": []string{"system:user"}},
			}),
			taskNode("access_governance", "IdentityGovernanceExecutor", "suspend", "end", nil),
			endNode(),
		},
	}
}
