// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authentication

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

const (
	errCodeInvalidOrganizationUnit = "FES-1023"
	errCodeOUNotResolved           = "FET-1124"
	errCodeNoAccessToApplication   = "FET-1122"
	errCodeOUIDNotAllowed          = "FES-1026"
	errCodeInvalidCredentials      = "FET-1005"

	ouBoundaryPassword     = "OuBoundary@123"
	ouBoundaryRedirectURI  = "https://localhost:3000/ou-boundary/callback"
	ouBoundaryRSIdentifier = "https://ou-boundary.example.com/documents"

	ouBoundaryParentHandle     = "ou-boundary-parent"
	ouBoundaryChildHandle      = "ou-boundary-child"
	ouBoundaryChildTwoHandle   = "ou-boundary-child-two"
	ouBoundaryForeignHandle    = "ou-boundary-foreign"
	ouBoundaryGrandchildHandle = "ou-boundary-grandchild"

	ouBoundarySharedUser     = "ou_boundary_shared"
	ouBoundaryChildTwoUser   = "ou_boundary_child_two"
	ouBoundaryForeignUser    = "ou_boundary_foreign"
	ouBoundaryDeepUser       = "ou_boundary_deep"
	ouBoundaryParentOnlyUser = "ou_boundary_parent_only"
	ouBoundaryTwinUser       = "ou_boundary_twin"
	ouBoundaryUnknownUser    = "ou_boundary_nobody"
	ouBoundaryWrongPassword  = "NotThePassword@123"

	ouBoundaryParentRole = "OU Boundary Reader"
	ouBoundaryChildRole  = "OU Boundary Writer"

	ouBoundaryParentGroup = "OU Boundary Parent Group"
	ouBoundaryChildGroup  = "OU Boundary Child Group"

	// An organization unit that does not exist.
	ouBoundaryUnknownOUID = "0195ffff-0000-7000-8000-000000000000"
)

var ouBoundarySchema = map[string]interface{}{
	"username": map[string]interface{}{"type": "string", "unique": true},
	"password": map[string]interface{}{"type": "string", "credential": true},
	"email":    map[string]interface{}{"type": "string"},
}

// ouBoundaryPromptAfterSignIn decides the OU from the signed-in user's eligible ones at or below parentOUID.
func ouBoundaryPromptAfterSignIn(parentOUID string) map[string]interface{} {
	return map[string]interface{}{"resolveFrom": "prompt", "parentOuId": parentOUID}
}

// ouBoundaryTwinSchema lets two users share a username.
var ouBoundaryTwinSchema = map[string]interface{}{
	"username": map[string]interface{}{"type": "string"},
	"password": map[string]interface{}{"type": "string", "credential": true},
	"email":    map[string]interface{}{"type": "string"},
}

// OUBoundedSignInTestSuite covers sign-in bounded to the organization units where both the user and
// the application are found.
//
// Fixture topology:
//
//	parent (ou-boundary-parent)            ← owns the shared applications, the shared and parent-only users
//	├── child (ou-boundary-child)          ← owns the unshared applications; holds the deep user and a twin;
//	│   │                                    the shared user is shared in
//	│   └── grandchild                     ← the deep user is shared in
//	└── child two (ou-boundary-child-two)  ← holds the child-two user and the other twin
//	foreign (ou-boundary-foreign)          ← another tree root, holding the foreign user
type OUBoundedSignInTestSuite struct {
	suite.Suite

	parentOUID     string
	childOUID      string
	childTwoOUID   string
	foreignOUID    string
	grandchildOUID string

	userTypeIDs []string
	flowIDs     []string
	appIDs      []string
	roleIDs     []string
	rsID        string

	parentType        string
	allowedTypes      []string
	credentialsFlowID string
	ssoFlowID         string

	sharedUserID     string
	childTwoUserID   string
	foreignUserID    string
	deepUserID       string
	parentOnlyUserID string
	childTwinID      string
	childTwoTwinID   string

	// App-native applications.
	nativeChildApp            string
	nativeSharedApp           string
	nativeSharedNoResolverApp string

	// Redirect applications.
	oauthSharedClientID string
	ssoChildClientID    string
	ssoForeignClientID  string

	// Linked account scenarios.
	mockGoogle       *testutils.MockGoogleOIDCServer
	googleIDPID      string
	federatedType    string
	federatedUserIDs []string
	// refreshUserID is the user the refresh test unshares, kept apart from the users other tests rely on.
	refreshUserID     string
	googleSharedApp   string
	googleLinkingApp  string
	googleJITChildApp string
}

func TestOUBoundedSignInTestSuite(t *testing.T) {
	suite.Run(t, new(OUBoundedSignInTestSuite))
}

// ouBoundaryFlow returns a sign-in flow: credentials, then an OU resolver with the given properties when they are
// set, then authorization and the assertion.
func ouBoundaryFlow(handle string, resolver map[string]interface{}) testutils.Flow {
	credentialInputs := []map[string]interface{}{
		{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
		{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
	}
	afterCredentials := "authorization_check"
	if resolver != nil {
		afterCredentials = "resolve_ou"
	}

	nodes := []map[string]interface{}{
		{"id": "start", "type": "START", "onSuccess": "prompt_credentials"},
		{
			"id":   "prompt_credentials",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{
				{
					"inputs": credentialInputs,
					"action": map[string]interface{}{"ref": "action_001", "nextNode": "credentials_auth"},
				},
			},
		},
		{
			"id":           "credentials_auth",
			"type":         "TASK_EXECUTION",
			"executor":     map[string]interface{}{"name": "CredentialsAuthExecutor", "inputs": credentialInputs},
			"onSuccess":    afterCredentials,
			"onIncomplete": "prompt_credentials",
		},
	}
	if resolver != nil {
		nodes = append(nodes,
			map[string]interface{}{
				"id":           "resolve_ou",
				"type":         "TASK_EXECUTION",
				"executor":     map[string]interface{}{"name": "OUResolverExecutor"},
				"properties":   resolver,
				"onSuccess":    "authorization_check",
				"onIncomplete": "prompt_ou",
			},
			map[string]interface{}{
				"id":   "prompt_ou",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{"ref": "input_ou", "identifier": "ouId", "type": "SELECT", "required": true},
						},
						"action": map[string]interface{}{"ref": "action_ou", "nextNode": "resolve_ou"},
					},
				},
			},
		)
	}
	nodes = append(nodes,
		map[string]interface{}{
			"id":        "authorization_check",
			"type":      "TASK_EXECUTION",
			"executor":  map[string]interface{}{"name": "AuthorizationExecutor"},
			"onSuccess": "auth_assert",
		},
		map[string]interface{}{
			"id":        "auth_assert",
			"type":      "TASK_EXECUTION",
			"executor":  map[string]interface{}{"name": "AuthAssertExecutor"},
			"onSuccess": "end",
		},
		map[string]interface{}{"id": "end", "type": "END"},
	)

	return testutils.Flow{Name: "OU Boundary " + handle, FlowType: "AUTHENTICATION", Handle: handle, Nodes: nodes}
}

// ouBoundarySSOFlow returns a sign-in flow that reuses a live SSO session in place of the credentials.
func ouBoundarySSOFlow(handle string) testutils.Flow {
	credentialInputs := []map[string]interface{}{
		{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
		{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
	}
	return testutils.Flow{
		Name:     "OU Boundary " + handle,
		FlowType: "AUTHENTICATION",
		Handle:   handle,
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "sso_check"},
			{
				"id":         "sso_check",
				"type":       "TASK_EXECUTION",
				"executor":   map[string]interface{}{"name": "SSOCheckExecutor"},
				"properties": map[string]interface{}{"checkpointRef": "session_main"},
				"onSuccess":  "session_main",
				"onFailure":  "prompt_credentials",
			},
			{
				"id":   "prompt_credentials",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": credentialInputs,
						"action": map[string]interface{}{"ref": "action_001", "nextNode": "credentials_auth"},
					},
				},
			},
			{
				"id":           "credentials_auth",
				"type":         "TASK_EXECUTION",
				"executor":     map[string]interface{}{"name": "CredentialsAuthExecutor", "inputs": credentialInputs},
				"onSuccess":    "session_main",
				"onIncomplete": "prompt_credentials",
			},
			{
				"id":        "session_main",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "SessionExecutor"},
				"onSuccess": "auth_assert",
			},
			{
				"id":        "auth_assert",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "AuthAssertExecutor"},
				"onSuccess": "end",
			},
			{"id": "end", "type": "END"},
		},
	}
}

func (ts *OUBoundedSignInTestSuite) SetupSuite() {
	ts.parentOUID = ts.createOU(ouBoundaryParentHandle, nil)
	ts.childOUID = ts.createOU(ouBoundaryChildHandle, &ts.parentOUID)
	ts.childTwoOUID = ts.createOU(ouBoundaryChildTwoHandle, &ts.parentOUID)
	ts.foreignOUID = ts.createOU(ouBoundaryForeignHandle, nil)
	ts.grandchildOUID = ts.createOU(ouBoundaryGrandchildHandle, &ts.childOUID)

	ts.parentType = ts.createUserType("ou-boundary-person", ts.parentOUID, ouBoundarySchema)
	foreignType := ts.createUserType("ou-boundary-foreign-person", ts.foreignOUID, ouBoundarySchema)
	twinType := ts.createUserType("ou-boundary-twin-person", ts.parentOUID, ouBoundaryTwinSchema)
	ts.allowedTypes = []string{ts.parentType, foreignType, twinType}
	allowedTypes := ts.allowedTypes

	ts.sharedUserID = ts.createUser(ts.parentType, ts.parentOUID, ouBoundarySharedUser)
	ts.childTwoUserID = ts.createUser(ts.parentType, ts.childTwoOUID, ouBoundaryChildTwoUser)
	ts.foreignUserID = ts.createUser(foreignType, ts.foreignOUID, ouBoundaryForeignUser)
	ts.deepUserID = ts.createUser(ts.parentType, ts.childOUID, ouBoundaryDeepUser)
	ts.parentOnlyUserID = ts.createUser(ts.parentType, ts.parentOUID, ouBoundaryParentOnlyUser)
	ts.childTwinID = ts.createUser(twinType, ts.childOUID, ouBoundaryTwinUser)
	ts.childTwoTwinID = ts.createUser(twinType, ts.childTwoOUID, ouBoundaryTwinUser)
	ts.shareUser(ts.sharedUserID, ts.childOUID)
	ts.shareUser(ts.deepUserID, ts.grandchildOUID)

	credentialsFlow := ts.createFlow(ouBoundaryFlow("ou_boundary_credentials", nil))
	resolverFlow := ts.createFlow(ouBoundaryFlow("ou_boundary_prompt_resolver",
		ouBoundaryPromptAfterSignIn(ts.parentOUID)))
	ssoFlow := ts.createFlow(ouBoundarySSOFlow("ou_boundary_sso"))
	ts.credentialsFlowID = credentialsFlow
	ts.ssoFlowID = ssoFlow

	assertion := map[string]interface{}{
		"userAttributes": []string{"ouId", "ouHandle", "roles", "groups"},
	}
	ts.nativeChildApp = ts.createNativeApp("OU Boundary Native Child", "ou_boundary_native_child",
		ts.childOUID, credentialsFlow, allowedTypes, assertion)
	ts.nativeSharedApp = ts.createNativeApp("OU Boundary Native Shared", "ou_boundary_native_shared",
		ts.parentOUID, resolverFlow, allowedTypes, assertion)
	ts.nativeSharedNoResolverApp = ts.createNativeApp("OU Boundary Native Shared No Resolver",
		"ou_boundary_native_shared_no_resolver", ts.parentOUID, credentialsFlow, allowedTypes, assertion)

	ts.oauthSharedClientID = "ou_boundary_oauth_shared"
	oauthSharedApp := ts.createRedirectApp("OU Boundary OAuth Shared", ts.oauthSharedClientID, ts.parentOUID,
		resolverFlow, allowedTypes)
	ts.ssoChildClientID = "ou_boundary_sso_child"
	ts.createRedirectApp("OU Boundary SSO Child", ts.ssoChildClientID, ts.childOUID, ssoFlow, allowedTypes)
	ts.ssoForeignClientID = "ou_boundary_sso_foreign"
	ts.createRedirectApp("OU Boundary SSO Foreign", ts.ssoForeignClientID, ts.foreignOUID, ssoFlow, allowedTypes)

	for _, appID := range []string{ts.nativeSharedApp, ts.nativeSharedNoResolverApp, oauthSharedApp} {
		ts.Require().NoError(testutils.ShareApplicationWithChildUnits(appID), "share application %s", appID)
	}

	// The shared user reads in the parent, where it lives, and writes in the child, where it is shared in.
	var err error
	ts.rsID, err = testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name: "OU Boundary Documents", Identifier: ouBoundaryRSIdentifier, OUID: ts.parentOUID,
	}, []testutils.Action{
		{Name: "Read", Handle: "read"},
		{Name: "Write", Handle: "write"},
	})
	ts.Require().NoError(err, "create resource server")
	ts.createRole(ouBoundaryParentRole, ts.parentOUID, "read")
	ts.createRole(ouBoundaryChildRole, ts.childOUID, "write")

	ts.setUpLinkedAccountFixture()
}

func (ts *OUBoundedSignInTestSuite) TearDownSuite() {
	for _, id := range ts.roleIDs {
		if err := testutils.DeleteRole(id); err != nil {
			ts.T().Logf("teardown: delete role %s: %v", id, err)
		}
	}
	if ts.rsID != "" {
		if err := testutils.DeleteResourceServerWithChildren(ts.rsID); err != nil {
			ts.T().Logf("teardown: delete resource server: %v", err)
		}
	}
	for _, id := range ts.appIDs {
		if err := testutils.DeleteApplication(id); err != nil {
			ts.T().Logf("teardown: delete application %s: %v", id, err)
		}
	}
	for _, id := range append([]string{ts.sharedUserID, ts.childTwoUserID, ts.foreignUserID, ts.deepUserID,
		ts.parentOnlyUserID, ts.childTwinID, ts.childTwoTwinID, ts.refreshUserID}, ts.federatedUserIDs...) {
		if id == "" {
			continue
		}
		if err := testutils.DeleteUser(id); err != nil {
			ts.T().Logf("teardown: delete user %s: %v", id, err)
		}
	}
	for _, id := range ts.flowIDs {
		if err := testutils.DeleteFlow(id); err != nil {
			ts.T().Logf("teardown: delete flow %s: %v", id, err)
		}
	}
	for _, id := range ts.userTypeIDs {
		if err := testutils.DeleteUserType(id); err != nil {
			ts.T().Logf("teardown: delete user type %s: %v", id, err)
		}
	}
	for _, id := range []string{ts.grandchildOUID, ts.childOUID, ts.childTwoOUID, ts.parentOUID, ts.foreignOUID} {
		if id == "" {
			continue
		}
		if err := testutils.DeleteOrganizationUnit(id); err != nil {
			ts.T().Logf("teardown: delete OU %s: %v", id, err)
		}
	}
	ts.tearDownLinkedAccountFixture()
}

// createOU creates an organization unit and returns its ID.
func (ts *OUBoundedSignInTestSuite) createOU(handle string, parent *string) string {
	ts.T().Helper()
	id, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: handle, Name: handle, Parent: parent,
	})
	ts.Require().NoError(err, "create OU %s", handle)
	return id
}

// createUserType creates a user type owned by ouID and returns its handle.
func (ts *OUBoundedSignInTestSuite) createUserType(handle, ouID string, schema map[string]interface{}) string {
	ts.T().Helper()
	id, err := testutils.CreateUserType(testutils.UserType{
		Handle: handle, DisplayName: handle, OUID: ouID, AllowSelfRegistration: true, Schema: schema,
	})
	ts.Require().NoError(err, "create user type %s", handle)
	ts.userTypeIDs = append(ts.userTypeIDs, id)
	return handle
}

// createUser creates a user living in ouID and returns its ID.
func (ts *OUBoundedSignInTestSuite) createUser(userType, ouID, username string) string {
	ts.T().Helper()
	id, err := testutils.CreateUser(testutils.User{
		Type: userType,
		OUID: ouID,
		Attributes: json.RawMessage(fmt.Sprintf(`{"username": %q, "password": %q, "email": %q}`,
			username, ouBoundaryPassword, username+"@ou-boundary.test")),
	})
	ts.Require().NoError(err, "create user %s", username)
	return id
}

// shareUser shares a user into one organization unit.
func (ts *OUBoundedSignInTestSuite) shareUser(userID, ouID string) {
	ts.T().Helper()
	status, raw := ts.request(testutils.GetHTTPClient(), http.MethodPost, "/users/"+userID+"/sharing-policies",
		map[string]interface{}{
			"childUnits": map[string]interface{}{
				"mode":  "selected",
				"units": []map[string]string{{"ouId": ouID, "reach": "unit"}},
			},
		})
	ts.Require().Equal(http.StatusCreated, status, "share user %s into %s: %s", userID, ouID, string(raw))
}

// unshareUser deletes every sharing policy of a user.
func (ts *OUBoundedSignInTestSuite) unshareUser(userID string) {
	ts.T().Helper()
	status, raw := ts.request(testutils.GetHTTPClient(), http.MethodGet, "/users/"+userID+"/sharing-policies", nil)
	ts.Require().Equal(http.StatusOK, status, "list the sharing policies of %s: %s", userID, string(raw))
	var list struct {
		Policies []struct {
			ID string `json:"id"`
		} `json:"policies"`
	}
	ts.Require().NoError(json.Unmarshal(raw, &list), string(raw))
	ts.Require().NotEmpty(list.Policies, "the user has a sharing policy")
	for _, p := range list.Policies {
		status, raw = ts.request(testutils.GetHTTPClient(), http.MethodDelete,
			"/users/"+userID+"/sharing-policies/"+p.ID, nil)
		ts.Require().Equal(http.StatusNoContent, status, "delete sharing policy %s: %s", p.ID, string(raw))
	}
}

// shareApplicationWith shares an application with the named child units only.
func (ts *OUBoundedSignInTestSuite) shareApplicationWith(appID string, ouIDs ...string) {
	ts.T().Helper()
	units := make([]map[string]string, 0, len(ouIDs))
	for _, ouID := range ouIDs {
		units = append(units, map[string]string{"ouId": ouID, "reach": "unit"})
	}
	status, raw := ts.request(testutils.GetHTTPClient(), http.MethodPost, "/applications/"+appID+"/sharing-policies",
		map[string]interface{}{"childUnits": map[string]interface{}{"mode": "selected", "units": units}})
	ts.Require().Equal(http.StatusCreated, status, "share application %s: %s", appID, string(raw))
}

// createFlow creates a flow and returns its ID.
func (ts *OUBoundedSignInTestSuite) createFlow(flow testutils.Flow) string {
	ts.T().Helper()
	id, err := testutils.CreateFlow(flow)
	ts.Require().NoError(err, "create flow %s", flow.Handle)
	ts.flowIDs = append(ts.flowIDs, id)
	return id
}

// createNativeApp creates an application that starts its sign-in on /flow/execute.
func (ts *OUBoundedSignInTestSuite) createNativeApp(name, clientID, ouID, flowID string, userTypes []string,
	assertion map[string]interface{}) string {
	ts.T().Helper()
	id, err := testutils.CreateApplication(testutils.Application{
		Name:             name,
		OUID:             ouID,
		ClientID:         clientID,
		ClientSecret:     clientID + "_secret",
		AuthFlowID:       flowID,
		AllowedUserTypes: userTypes,
		AssertionConfig:  assertion,
	})
	ts.Require().NoError(err, "create application %s", name)
	ts.appIDs = append(ts.appIDs, id)
	return id
}

// createRedirectApp creates an authorization code client whose access tokens carry the sign-in's OU.
func (ts *OUBoundedSignInTestSuite) createRedirectApp(name, clientID, ouID, flowID string,
	userTypes []string, extraGrantTypes ...string) string {
	ts.T().Helper()
	id, err := testutils.CreateApplication(testutils.Application{
		Name:             name,
		OUID:             ouID,
		AuthFlowID:       flowID,
		AllowedUserTypes: userTypes,
		InboundAuthConfig: []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                clientID,
					"clientSecret":            clientID + "_secret",
					"redirectUris":            []string{ouBoundaryRedirectURI},
					"grantTypes":              append([]string{"authorization_code"}, extraGrantTypes...),
					"responseTypes":           []string{"code"},
					"tokenEndpointAuthMethod": "client_secret_basic",
					"token": map[string]interface{}{
						"accessToken": map[string]interface{}{
							"userConfig": map[string]interface{}{"attributes": []string{"ouId"}},
						},
					},
				},
			},
		},
	})
	ts.Require().NoError(err, "create application %s", name)
	ts.appIDs = append(ts.appIDs, id)
	return id
}

// createRole creates a role owned by ouID granting one permission on the suite's resource server to the
// shared user.
func (ts *OUBoundedSignInTestSuite) createRole(name, ouID, permission string) {
	ts.T().Helper()
	id, err := testutils.CreateRole(testutils.Role{
		Name:        name,
		OUID:        ouID,
		Permissions: []testutils.ResourcePermissions{{ResourceServerID: ts.rsID, Permissions: []string{permission}}},
		Assignments: []testutils.Assignment{{ID: ts.sharedUserID, Type: "user"}},
	})
	ts.Require().NoError(err, "create role %s", name)
	ts.roleIDs = append(ts.roleIDs, id)
}

// request sends a JSON request through client and returns the status and body.
func (ts *OUBoundedSignInTestSuite) request(client *http.Client, method, path string,
	body interface{}) (int, []byte) {
	ts.T().Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		ts.Require().NoError(err)
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, testutils.TestServerURL+path, reader)
	ts.Require().NoError(err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	return resp.StatusCode, raw
}

// startSignIn starts an app-native sign-in, naming ouID when it is not empty.
func (ts *OUBoundedSignInTestSuite) startSignIn(appID, ouID string, inputs map[string]string) (int, []byte) {
	ts.T().Helper()
	body := map[string]interface{}{"applicationId": appID, "flowType": "AUTHENTICATION"}
	if ouID != "" {
		body["ouId"] = ouID
	}
	if len(inputs) > 0 {
		body["inputs"] = inputs
	}
	payload, err := json.Marshal(body)
	ts.Require().NoError(err)
	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+"/flow/execute", bytes.NewReader(payload))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	if secret := testutils.GetFlowSecret(appID); secret != "" {
		req.Header.Set(testutils.FlowSecretHeaderName, secret)
	}
	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	return resp.StatusCode, raw
}

// signIn starts an app-native sign-in and submits the user's credentials.
func (ts *OUBoundedSignInTestSuite) signIn(appID, ouID, username string,
	inputs map[string]string) *common.FlowStep {
	ts.T().Helper()
	return ts.signInWithPassword(appID, ouID, username, ouBoundaryPassword, inputs)
}

// signInWithPassword is signIn with the given password.
func (ts *OUBoundedSignInTestSuite) signInWithPassword(appID, ouID, username, password string,
	inputs map[string]string) *common.FlowStep {
	ts.T().Helper()
	status, raw := ts.startSignIn(appID, ouID, inputs)
	ts.Require().Equal(http.StatusOK, status, "start the sign-in: %s", string(raw))
	var step common.FlowStep
	ts.Require().NoError(json.Unmarshal(raw, &step))
	ts.Require().True(common.HasInput(step.Data.Inputs, "username"), "expected the credentials prompt: %s", raw)

	next, err := common.CompleteFlow(step.ExecutionID,
		map[string]string{"username": username, "password": password}, "action_001", step.ChallengeToken)
	ts.Require().NoError(err, "submit the credentials")
	return next
}

// requireSameRefusal checks that every step was refused as not found with the same error.
func (ts *OUBoundedSignInTestSuite) requireSameRefusal(steps map[string]*common.FlowStep, reference string) {
	ts.T().Helper()
	want := steps[reference]
	ts.requireNotFound(want)
	for name, step := range steps {
		ts.requireNotFound(step)
		ts.Equal(*want.Error, *step.Error, "%s gets the same answer as %s", name, reference)
	}
}

// requireSignedInto checks that the step completed with an assertion for ouID and returns its claims.
func (ts *OUBoundedSignInTestSuite) requireSignedInto(step *common.FlowStep, ouID string) *testutils.JWTClaims {
	ts.T().Helper()
	ts.Require().Equal("COMPLETE", step.FlowStatus, "the sign-in should complete: %+v", step.Error)
	ts.Require().NotEmpty(step.Assertion, "a completed sign-in carries an assertion")
	claims, err := testutils.DecodeJWT(step.Assertion)
	ts.Require().NoError(err)
	ts.Equal(ouID, claims.OUID, "the assertion names the OU the sign-in landed in")
	return claims
}

// requireNotFound checks that the credentials were refused because the user is outside the boundary.
func (ts *OUBoundedSignInTestSuite) requireNotFound(step *common.FlowStep) {
	ts.T().Helper()
	ts.Empty(step.Assertion, "a user outside the boundary gets no assertion")
	ts.Require().NotNil(step.Error, "a user outside the boundary is refused")
	ts.Equal(errCodeUserNotFound, step.Error.Code, "a user outside the boundary is not found")
}

// authorize sends an authorization request through client, to the organization unit path when ouID is set.
func (ts *OUBoundedSignInTestSuite) authorize(client *http.Client, ouID, clientID, state string) *http.Response {
	ts.T().Helper()
	return ts.authorizeWith(client, ouID, clientID, state, nil)
}

// authorizeWith is authorize with extra request parameters.
func (ts *OUBoundedSignInTestSuite) authorizeWith(client *http.Client, ouID, clientID, state string,
	extra map[string]string) *http.Response {
	ts.T().Helper()
	params := url.Values{}
	params.Set("client_id", clientID)
	params.Set("redirect_uri", ouBoundaryRedirectURI)
	params.Set("response_type", "code")
	params.Set("scope", "openid")
	params.Set("state", state)
	for k, v := range extra {
		params.Set(k, v)
	}
	path := "/oauth2/authorize"
	if ouID != "" {
		path = "/ou/" + ouID + path
	}
	req, err := http.NewRequest(http.MethodGet, testutils.TestServerURL+path+"?"+params.Encode(), nil)
	ts.Require().NoError(err)
	resp, err := client.Do(req)
	ts.Require().NoError(err)
	return resp
}

// startAuthorization authorizes through client and returns the authId and executionId the gate redirect carries.
func (ts *OUBoundedSignInTestSuite) startAuthorization(client *http.Client, ouID, clientID,
	state string) (string, string) {
	ts.T().Helper()
	resp := ts.authorize(client, ouID, clientID, state)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusFound, resp.StatusCode, "authorize redirects to the gate")
	authID, executionID, err := testutils.ExtractAuthData(resp.Header.Get("Location"))
	ts.Require().NoError(err, "the gate redirect carries the flow: %s", resp.Header.Get("Location"))
	return authID, executionID
}

// executeWith posts a flow step through client, so SSO cookies are carried, and returns the step.
func (ts *OUBoundedSignInTestSuite) executeWith(client *http.Client, body map[string]interface{}) *common.FlowStep {
	ts.T().Helper()
	status, raw, err := testutils.ExecuteFlowRaw(client, "", body)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, status, "flow step: %s", string(raw))
	var step common.FlowStep
	ts.Require().NoError(json.Unmarshal(raw, &step))
	return &step
}

// redirectSignIn completes a redirect sign-in with the user's credentials and returns the access token.
func (ts *OUBoundedSignInTestSuite) redirectSignIn(ouID, clientID, username string) string {
	ts.T().Helper()
	return ts.redeemCode(ts.redirectSignInForCode(ouID, clientID, username), clientID)
}

// redirectSignInForCode signs username in through the authorization endpoint, on the OU path when ouID is set, and
// returns the client redirect carrying the code.
func (ts *OUBoundedSignInTestSuite) redirectSignInForCode(ouID, clientID, username string) string {
	ts.T().Helper()
	client, err := testutils.NewSessionClient()
	ts.Require().NoError(err)
	authID, executionID := ts.startAuthorization(client, ouID, clientID, "ou-boundary-state")

	prompt := ts.executeWith(client, map[string]interface{}{"executionId": executionID})
	ts.Require().True(common.HasInput(prompt.Data.Inputs, "username"), "expected the credentials prompt")
	step := ts.executeWith(client, map[string]interface{}{
		"executionId":    executionID,
		"inputs":         map[string]string{"username": username, "password": ouBoundaryPassword},
		"action":         "action_001",
		"challengeToken": prompt.ChallengeToken,
	})
	ts.Require().Equal("COMPLETE", step.FlowStatus, "the OU in the path decides the OU, so nothing is asked: %+v",
		step.Data.Inputs)
	authz, err := testutils.CompleteAuthorizationWithClient(client, authID, step.Assertion)
	ts.Require().NoError(err)
	return authz.RedirectURI
}

// completeToAccessToken posts the assertion to the authorize callback and exchanges the code for an access token.
func (ts *OUBoundedSignInTestSuite) completeToAccessToken(client *http.Client, authID, assertion,
	clientID string) string {
	ts.T().Helper()
	authz, err := testutils.CompleteAuthorizationWithClient(client, authID, assertion)
	ts.Require().NoError(err)
	return ts.redeemCode(authz.RedirectURI, clientID)
}

// redeemCode exchanges the code a client redirect carries for an access token.
func (ts *OUBoundedSignInTestSuite) redeemCode(redirect, clientID string) string {
	ts.T().Helper()
	code, err := testutils.ExtractAuthorizationCode(redirect)
	ts.Require().NoError(err)
	token, err := testutils.RequestToken(clientID, clientID+"_secret", code, ouBoundaryRedirectURI,
		"authorization_code")
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, token.StatusCode, "token: %s", string(token.Body))
	return token.Token.AccessToken
}

// redeemCodeAt exchanges the code a client redirect carries at the token endpoint of ouID, and returns the status
// and the decoded body.
func (ts *OUBoundedSignInTestSuite) redeemCodeAt(redirect, clientID, ouID string) (int, map[string]interface{}) {
	ts.T().Helper()
	code, err := testutils.ExtractAuthorizationCode(redirect)
	ts.Require().NoError(err)
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", ouBoundaryRedirectURI)
	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+"/ou/"+ouID+"/oauth2/token",
		strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientID+"_secret")
	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	body := map[string]interface{}{}
	ts.Require().NoError(json.Unmarshal(raw, &body), string(raw))
	return resp.StatusCode, body
}

// tokenOUID returns the ouId claim of an access token.
func (ts *OUBoundedSignInTestSuite) tokenOUID(accessToken string) string {
	ts.T().Helper()
	claims, err := testutils.DecodeJWT(accessToken)
	ts.Require().NoError(err)
	return claims.OUID
}

// credentialSignIn submits the user's credentials at the gate prompt of a redirect sign-in and returns the step.
func (ts *OUBoundedSignInTestSuite) credentialSignIn(client *http.Client, executionID,
	username string) *common.FlowStep {
	ts.T().Helper()
	prompt := ts.executeWith(client, map[string]interface{}{"executionId": executionID})
	ts.Require().True(common.HasInput(prompt.Data.Inputs, "username"), "expected the credentials prompt")
	return ts.executeWith(client, map[string]interface{}{
		"executionId":    executionID,
		"inputs":         map[string]string{"username": username, "password": ouBoundaryPassword},
		"action":         "action_001",
		"challengeToken": prompt.ChallengeToken,
	})
}

// handlePath returns the handle path prompt offers after sign-in for an OU, joining its handles from the root.
func handlePath(handles ...string) string {
	return strings.Join(handles, "/")
}

// An unshared application is used from its own OU only, so a user from elsewhere is not found. The answer
// is the one an unknown username gets, whether the password is right or wrong, so the lookup refuses the
// user before any credential is checked.
func (ts *OUBoundedSignInTestSuite) TestUnsharedApp_UserFromAnotherOUIsNotFound() {
	for name, username := range map[string]string{
		"AnotherTree": ouBoundaryForeignUser,
		"SiblingOU":   ouBoundaryChildTwoUser,
	} {
		ts.Run(name, func() {
			ts.requireSameRefusal(map[string]*common.FlowStep{
				"unknown username": ts.signIn(ts.nativeChildApp, "", ouBoundaryUnknownUser, nil),
				"right password":   ts.signIn(ts.nativeChildApp, "", username, nil),
				"wrong password": ts.signInWithPassword(ts.nativeChildApp, "", username,
					ouBoundaryWrongPassword, nil),
			}, "unknown username")
		})
	}
}

// A user living in an unshared application's OU signs in there.
func (ts *OUBoundedSignInTestSuite) TestUnsharedApp_UserOfTheAppsOUSignsInThere() {
	claims := ts.requireSignedInto(ts.signIn(ts.nativeChildApp, "", ouBoundaryDeepUser, nil), ts.childOUID)
	ts.Equal(ts.deepUserID, claims.Sub)
	ts.Equal(ouBoundaryChildHandle, claims.OuHandle)
}

// Of two users with the same username, the one in the unshared application's OU is found, with no
// disambiguation asked. Through a shared application, which reaches both, the same credentials name no single
// user and are refused.
func (ts *OUBoundedSignInTestSuite) TestUnsharedApp_SameUsernameElsewhereIsNotAmbiguous() {
	step := ts.signIn(ts.nativeChildApp, "", ouBoundaryTwinUser, nil)

	ts.Empty(step.Data.Inputs, "nothing is asked to tell the users apart: %+v", step.Data.Inputs)
	claims := ts.requireSignedInto(step, ts.childOUID)
	ts.Equal(ts.childTwinID, claims.Sub, "the user in the application's OU is the one signed in")

	ambiguous := ts.signIn(ts.nativeSharedApp, "", ouBoundaryTwinUser, nil)
	ts.NotEqual("COMPLETE", ambiguous.FlowStatus, "both users are reachable through the shared application")
	ts.Empty(ambiguous.Assertion)
	ts.Require().NotNil(ambiguous.Error, "%+v", ambiguous)
	ts.Equal(errCodeInvalidCredentials, ambiguous.Error.Code)
}

// A user shared into an unshared application's OU signs in there, and the assertion names that OU rather
// than the user's own.
func (ts *OUBoundedSignInTestSuite) TestUnsharedApp_SharedInUserSignsInToTheAppsOU() {
	claims := ts.requireSignedInto(ts.signIn(ts.nativeChildApp, "", ouBoundarySharedUser, nil), ts.childOUID)
	ts.Equal(ouBoundaryChildHandle, claims.OuHandle)

	user, err := testutils.FindUserByAttribute("username", ouBoundarySharedUser)
	ts.Require().NoError(err)
	ts.Require().NotNil(user)
	ts.Equal(ts.parentOUID, user.OUID, "the user still lives in its own OU")
}

// With sign in first, a user eligible in one OU only is signed in there without being asked.
func (ts *OUBoundedSignInTestSuite) TestSharedApp_OneEligibleOUIsDecided() {
	step := ts.signIn(ts.nativeSharedApp, "", ouBoundaryChildTwoUser, nil)
	ts.False(common.HasInput(step.Data.Inputs, "ouId"), "a single eligible OU is not offered as a choice")
	ts.requireSignedInto(step, ts.childTwoOUID)
}

// With sign in first, a user eligible in several OUs chooses one by its handle path, and a choice outside
// them is refused and asked for again.
func (ts *OUBoundedSignInTestSuite) TestSharedApp_SeveralEligibleOUsArePrompted() {
	step := ts.signIn(ts.nativeSharedApp, "", ouBoundarySharedUser, nil)
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "several eligible OUs need a choice: %+v", step.Error)
	ts.Empty(step.Assertion)
	input := findDisambiguationInput(step.Data.Inputs, "ouId")
	ts.Require().NotNil(input, "the OU is asked for: %+v", step.Data.Inputs)
	ts.Equal("SELECT", input.Type)
	ts.Equal([]string{
		handlePath(ouBoundaryParentHandle),
		handlePath(ouBoundaryParentHandle, ouBoundaryChildHandle),
	}, input.Options, "the user's own OU and the one it is shared into, in path order")

	rejected, err := common.CompleteFlow(step.ExecutionID,
		map[string]string{"ouId": handlePath(ouBoundaryForeignHandle)}, "action_ou", step.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().NotNil(rejected.Error, "an OU outside the eligible ones is refused")
	ts.Equal(errCodeInvalidOU, rejected.Error.Code)
	ts.Empty(rejected.Assertion)
	ts.True(common.HasInput(rejected.Data.Inputs, "ouId"), "the choice is asked for again")

	chosen, err := common.CompleteFlow(rejected.ExecutionID,
		map[string]string{"ouId": handlePath(ouBoundaryParentHandle, ouBoundaryChildHandle)}, "action_ou",
		rejected.ChallengeToken)
	ts.Require().NoError(err)
	ts.requireSignedInto(chosen, ts.childOUID)
}

// With sign in first no OU is fixed, so the user is looked up deployment-wide. A user in no OU the application
// reaches signs in and is then refused by the resolver, with no assertion.
func (ts *OUBoundedSignInTestSuite) TestSharedApp_NoEligibleOUIsRefusedAfterSignIn() {
	step := ts.signIn(ts.nativeSharedApp, "", ouBoundaryForeignUser, nil)

	ts.Empty(step.Assertion, "a refused sign-in carries no assertion")
	ts.Require().NotNil(step.Error, "a user in no OU the application reaches is refused")
	ts.Equal(errCodeNoAccessToApplication, step.Error.Code)
}

// A shared application whose flow never decides the OU is refused at its assertion step, with no assertion and no
// session handed out, although the flow saved one.
func (ts *OUBoundedSignInTestSuite) TestSharedAppWithoutResolver_IsRefusedAtItsAssertion() {
	step := ts.signIn(ts.nativeSharedNoResolverApp, "", ouBoundarySharedUser, nil)

	ts.Equal("ERROR", step.FlowStatus)
	ts.Empty(step.Assertion, "a refused sign-in carries no assertion")
	ts.Require().NotNil(step.Error)
	ts.Equal(errCodeOUNotResolved, step.Error.Code)

	ssoApp := ts.createNativeApp("OU Boundary Native Shared SSO", "ou_boundary_native_shared_sso",
		ts.parentOUID, ts.ssoFlowID, ts.allowedTypes, map[string]interface{}{"userAttributes": []string{"ouId"}})
	ts.Require().NoError(testutils.ShareApplicationWithChildUnits(ssoApp))
	signInWithSession := func(ouID string) (*testutils.FlowStep, string) {
		status, raw := ts.startSignIn(ssoApp, ouID, nil)
		ts.Require().Equal(http.StatusOK, status, string(raw))
		var prompt common.FlowStep
		ts.Require().NoError(json.Unmarshal(raw, &prompt))
		ts.Require().True(common.HasInput(prompt.Data.Inputs, "username"), "no session yet, so credentials: %s", raw)
		_, body, err := common.CompleteFlowWithRawResponse(prompt.ExecutionID,
			map[string]string{"username": ouBoundarySharedUser, "password": ouBoundaryPassword}, "action_001",
			prompt.ChallengeToken)
		ts.Require().NoError(err)
		var step testutils.FlowStep
		ts.Require().NoError(json.Unmarshal([]byte(body), &step))
		return &step, body
	}

	refused, body := signInWithSession("")
	ts.Equal("ERROR", refused.FlowStatus, body)
	ts.Require().NotNil(refused.Error)
	ts.Equal(errCodeOUNotResolved, refused.Error.Code)
	ts.Empty(refused.Assertion, "a refused sign-in carries no assertion")
	ts.Empty(refused.SessionData, "a refused sign-in hands out no session: %s", body)

	completed, body := signInWithSession(ts.childOUID)
	ts.Equal("COMPLETE", completed.FlowStatus, body)
	ts.NotEmpty(completed.SessionData, "the same flow hands out its session once the OU is decided: %s", body)
}

// An OU named on /flow/execute bounds the sign-in to it, so the flow needs no resolver and a user who is
// not in it is not found.
func (ts *OUBoundedSignInTestSuite) TestAppNativeOUID_BoundsTheSignInToThatOU() {
	ts.requireSignedInto(ts.signIn(ts.nativeSharedNoResolverApp, ts.childOUID, ouBoundarySharedUser, nil),
		ts.childOUID)
	ts.requireNotFound(ts.signIn(ts.nativeSharedNoResolverApp, ts.childOUID, ouBoundaryChildTwoUser, nil))
}

// An OU named on /flow/execute that the application is not used from is refused, whether it exists or not,
// with the same answer.
func (ts *OUBoundedSignInTestSuite) TestAppNativeOUID_UnknownAndUnsharedOUGetTheSameAnswer() {
	unsharedStatus, unshared := ts.startSignIn(ts.nativeSharedApp, ts.foreignOUID, nil)
	unknownStatus, unknown := ts.startSignIn(ts.nativeSharedApp, ouBoundaryUnknownOUID, nil)

	ts.Equal(http.StatusBadRequest, unsharedStatus, string(unshared))
	var refusal common.ErrorResponse
	ts.Require().NoError(json.Unmarshal(unshared, &refusal))
	ts.Equal(errCodeInvalidOrganizationUnit, refusal.Code)
	ts.Equal(unsharedStatus, unknownStatus)
	ts.Equal(string(unshared), string(unknown), "an unknown OU is not told apart from an unshared one")
}

// An OU is named only to start a sign-in.
func (ts *OUBoundedSignInTestSuite) TestAppNativeOUID_RefusedOutsideASignInStart() {
	payloads := map[string]map[string]interface{}{
		"Registration": {"applicationId": ts.nativeSharedApp, "flowType": "REGISTRATION", "ouId": ts.childOUID},
		"Continuation": {"executionId": "0195ffff-0000-7000-8000-000000000001", "ouId": ts.childOUID},
		"FlowID":       {"flowId": ts.credentialsFlowID, "flowType": "AUTHENTICATION", "ouId": ts.childOUID},
	}
	for name, payload := range payloads {
		ts.Run(name, func() {
			status, raw, err := testutils.ExecuteFlowRaw(nil, "", payload)
			ts.Require().NoError(err)
			ts.Equal(http.StatusBadRequest, status, string(raw))
			var refusal common.ErrorResponse
			ts.Require().NoError(json.Unmarshal(raw, &refusal))
			ts.Equal(errCodeOUIDNotAllowed, refusal.Code)
		})
	}
}

// roleCase is a sign-in OU with the one permission and role the shared user holds there.
type roleCase struct {
	name       string
	ouID       string
	permission string
	role       string
}

// roleCases are the OUs the shared user holds a role in: the parent grants read and the child write.
func (ts *OUBoundedSignInTestSuite) roleCases() []roleCase {
	return []roleCase{
		{"Child", ts.childOUID, "write", ouBoundaryChildRole},
		{"Parent", ts.parentOUID, "read", ouBoundaryParentRole},
	}
}

// signInRequestingPermissions signs the shared user in to ouID, requesting both permissions.
func (ts *OUBoundedSignInTestSuite) signInRequestingPermissions(ouID string) *testutils.JWTClaims {
	ts.T().Helper()
	inputs := map[string]string{
		"requested_permissions":      "read write",
		"resource_server_identifier": ouBoundaryRSIdentifier,
	}
	return ts.requireSignedInto(ts.signIn(ts.nativeSharedApp, ouID, ouBoundarySharedUser, inputs), ouID)
}

// Authorization counts only the roles assigned in the OU the sign-in lands in.
func (ts *OUBoundedSignInTestSuite) TestRolesFromOtherOUsGrantNoPermission() {
	for _, tc := range ts.roleCases() {
		ts.Run(tc.name, func() {
			claims := ts.signInRequestingPermissions(tc.ouID)
			ts.Equal(tc.permission, claims.Additional["authorized_permissions"],
				"only the permission granted in the sign-in's OU is authorized")
		})
	}
}

// The roles claim lists only the roles assigned in the OU the sign-in lands in.
func (ts *OUBoundedSignInTestSuite) TestRolesFromOtherOUsAreNotListed() {
	for _, tc := range ts.roleCases() {
		ts.Run(tc.name, func() {
			claims := ts.signInRequestingPermissions(tc.ouID)
			ts.Equal([]interface{}{tc.role}, claims.Additional["roles"],
				"only the role assigned in the sign-in's OU is listed")
		})
	}
}

// The groups claim lists only the groups the OU the sign-in lands in owns, though the user is in a group of each.
func (ts *OUBoundedSignInTestSuite) TestGroupsFromOtherOUsAreNotListed() {
	for name, ouID := range map[string]string{
		ouBoundaryParentGroup: ts.parentOUID,
		ouBoundaryChildGroup:  ts.childOUID,
	} {
		groupID, err := testutils.CreateGroup(testutils.Group{
			Name: name, OUID: ouID, Members: []testutils.Member{{Id: ts.sharedUserID, Type: "user"}},
		})
		ts.Require().NoError(err, "create group %s", name)
		defer func() {
			if err := testutils.DeleteGroup(groupID); err != nil {
				ts.T().Logf("delete group %s: %v", groupID, err)
			}
		}()
	}

	for _, tc := range []struct{ name, ouID, group string }{
		{"Child", ts.childOUID, ouBoundaryChildGroup},
		{"Parent", ts.parentOUID, ouBoundaryParentGroup},
	} {
		ts.Run(tc.name, func() {
			claims := ts.requireSignedInto(ts.signIn(ts.nativeSharedApp, tc.ouID, ouBoundarySharedUser, nil), tc.ouID)
			ts.Equal([]interface{}{tc.group}, claims.Additional["groups"],
				"only the group the sign-in's OU owns is listed")
		})
	}
}

// The OU path bounds a redirect sign-in to that OU: a shared-in user's token names it, and the user's
// self-service calls work although the token's OU is not the user's own.
func (ts *OUBoundedSignInTestSuite) TestOUPathAuthorize_SharedInUserActsInTheChild() {
	accessToken := ts.redirectSignIn(ts.childOUID, ts.oauthSharedClientID, ouBoundarySharedUser)
	claims, err := testutils.DecodeJWT(accessToken)
	ts.Require().NoError(err)
	ts.Equal(ts.childOUID, claims.OUID, "the token names the OU the user signed in to")

	client := testutils.GetHTTPClientWithToken(accessToken)
	status, raw := ts.request(client, http.MethodGet, "/users/me", nil)
	ts.Require().Equal(http.StatusOK, status, "read the own profile: %s", string(raw))
	var me testutils.User
	ts.Require().NoError(json.Unmarshal(raw, &me))
	ts.Equal(ts.sharedUserID, me.ID)
	ts.Equal(ts.parentOUID, me.OUID, "the profile is the user's own, in its own OU")

	email := "updated." + ouBoundarySharedUser + "@ou-boundary.test"
	status, raw = ts.request(client, http.MethodPut, "/users/me", map[string]interface{}{
		"attributes": map[string]interface{}{"username": ouBoundarySharedUser, "email": email},
	})
	ts.Require().Equal(http.StatusOK, status, "update the own profile: %s", string(raw))
	var updated testutils.User
	ts.Require().NoError(json.Unmarshal(raw, &updated))
	var attributes map[string]interface{}
	ts.Require().NoError(json.Unmarshal(updated.Attributes, &attributes))
	ts.Equal(email, attributes["email"])
}

// A refresh token issued in an OU keeps working while its user can sign in there, and is refused once the user's
// share into that OU is removed.
func (ts *OUBoundedSignInTestSuite) TestRefresh_RefusedOnceTheUserIsUnsharedFromItsOU() {
	clientID := "ou_boundary_oauth_refresh"
	appID := ts.createRedirectApp("OU Boundary OAuth Refresh", clientID, ts.parentOUID, ts.credentialsFlowID,
		ts.allowedTypes, "refresh_token")
	ts.shareApplicationWith(appID, ts.childOUID)
	username := "ou-boundary-refresh-user"
	ts.refreshUserID = ts.createUser(ts.parentType, ts.parentOUID, username)
	ts.shareUser(ts.refreshUserID, ts.childOUID)

	code, err := testutils.ExtractAuthorizationCode(ts.redirectSignInForCode(ts.childOUID, clientID, username))
	ts.Require().NoError(err)
	issued, err := testutils.RequestToken(clientID, clientID+"_secret", code, ouBoundaryRedirectURI,
		"authorization_code")
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, issued.StatusCode, "token: %s", string(issued.Body))
	ts.Require().Equal(ts.childOUID, ts.tokenOUID(issued.Token.AccessToken))
	ts.Require().NotEmpty(issued.Token.RefreshToken)

	refreshed, err := testutils.RefreshAccessTokenRaw(clientID, clientID+"_secret", issued.Token.RefreshToken, "")
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, refreshed.StatusCode, "the user is still shared into the child: %s",
		string(refreshed.Body))
	refreshToken := issued.Token.RefreshToken
	if refreshed.Token.RefreshToken != "" {
		refreshToken = refreshed.Token.RefreshToken
	}

	ts.unshareUser(ts.refreshUserID)
	refused, err := testutils.RefreshAccessTokenRaw(clientID, clientID+"_secret", refreshToken, "")
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, refused.StatusCode, "the user is no longer in the child: %s",
		string(refused.Body))
	var body map[string]interface{}
	ts.Require().NoError(json.Unmarshal(refused.Body, &body), string(refused.Body))
	ts.Equal("invalid_grant", body["error"])
}

// The OU token endpoint redeems a code whose sign-in landed in that OU, and refuses one for another OU.
func (ts *OUBoundedSignInTestSuite) TestOUPathAuthorize_CodeIsRedeemedOnlyAtItsOUsTokenEndpoint() {
	ts.Run("ItsOU", func() {
		redirect := ts.redirectSignInForCode(ts.childOUID, ts.oauthSharedClientID, ouBoundarySharedUser)
		status, body := ts.redeemCodeAt(redirect, ts.oauthSharedClientID, ts.childOUID)
		ts.Require().Equal(http.StatusOK, status, "token: %v", body)
		accessToken, _ := body["access_token"].(string)
		ts.Equal(ts.childOUID, ts.tokenOUID(accessToken))
	})
	ts.Run("AnotherOU", func() {
		redirect := ts.redirectSignInForCode(ts.childOUID, ts.oauthSharedClientID, ouBoundarySharedUser)
		status, body := ts.redeemCodeAt(redirect, ts.oauthSharedClientID, ts.childTwoOUID)
		ts.Equal(http.StatusBadRequest, status)
		ts.Equal("invalid_grant", body["error"])
		ts.Equal("The authorization code was not issued for the requested organization unit",
			body["error_description"])
	})
}

// The OU path leaves out a user who is not in that OU.
func (ts *OUBoundedSignInTestSuite) TestOUPathAuthorize_UserNotInTheOUIsNotFound() {
	client, err := testutils.NewSessionClient()
	ts.Require().NoError(err)
	_, executionID := ts.startAuthorization(client, ts.childOUID, ts.oauthSharedClientID, "ou-boundary-state")

	prompt := ts.executeWith(client, map[string]interface{}{"executionId": executionID})
	step := ts.executeWith(client, map[string]interface{}{
		"executionId":    executionID,
		"inputs":         map[string]string{"username": ouBoundaryChildTwoUser, "password": ouBoundaryPassword},
		"action":         "action_001",
		"challengeToken": prompt.ChallengeToken,
	})
	ts.requireNotFound(step)
}

// The OU path answers an unknown OU exactly as an OU the application is not used from.
func (ts *OUBoundedSignInTestSuite) TestOUPathAuthorize_UnknownAndUnsharedOUGetTheSameRedirect() {
	client, err := testutils.NewSessionClient()
	ts.Require().NoError(err)

	locations := make([]string, 0, 2)
	for _, ouID := range []string{ts.foreignOUID, ouBoundaryUnknownOUID} {
		resp := ts.authorize(client, ouID, ts.oauthSharedClientID, "ou-boundary-state")
		resp.Body.Close()
		ts.Require().Equal(http.StatusFound, resp.StatusCode)
		locations = append(locations, resp.Header.Get("Location"))
	}

	redirect, err := url.Parse(locations[0])
	ts.Require().NoError(err)
	ts.Equal(ouBoundaryRedirectURI, redirect.Scheme+"://"+redirect.Host+redirect.Path,
		"the refusal goes back to the client")
	ts.Equal("unauthorized_client", redirect.Query().Get("error"))
	ts.Equal("ou-boundary-state", redirect.Query().Get("state"))
	ts.Equal(locations[0], locations[1], "an unknown OU is not told apart from an unshared one")
}

// A blank OU in the path is refused outright, without starting a sign-in or redirecting to the client.
func (ts *OUBoundedSignInTestSuite) TestOUPathAuthorize_BlankOUIsRefused() {
	client, err := testutils.NewSessionClient()
	ts.Require().NoError(err)

	resp := ts.authorize(client, "%20", ts.oauthSharedClientID, "ou-boundary-state")
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)

	ts.Equal(http.StatusBadRequest, resp.StatusCode, string(raw))
	ts.Empty(resp.Header.Get("Location"), "a blank OU is not redirected anywhere")
	var refusal map[string]interface{}
	ts.Require().NoError(json.Unmarshal(raw, &refusal), string(raw))
	ts.Equal("invalid_request", refusal["error"])
}

// A live SSO session is reused in an OU where its user is eligible. In one where it is not, the sign-in asks for
// credentials instead, so another user of that OU can still sign in.
func (ts *OUBoundedSignInTestSuite) TestSSOReuse_IntoAnOUTheUserIsNotInAsksForCredentials() {
	client, err := testutils.NewSessionClient()
	ts.Require().NoError(err)

	authID, executionID := ts.startAuthorization(client, "", ts.ssoChildClientID, "sso-first")
	prompt := ts.executeWith(client, map[string]interface{}{"executionId": executionID})
	ts.Require().True(common.HasInput(prompt.Data.Inputs, "username"), "the first sign-in asks for credentials")
	first := ts.executeWith(client, map[string]interface{}{
		"executionId":    executionID,
		"inputs":         map[string]string{"username": ouBoundarySharedUser, "password": ouBoundaryPassword},
		"action":         "action_001",
		"challengeToken": prompt.ChallengeToken,
	})
	ts.Require().Equal("COMPLETE", first.FlowStatus, "the user is shared into the child: %+v", first.Error)
	_, err = testutils.CompleteAuthorizationWithClient(client, authID, first.Assertion)
	ts.Require().NoError(err)
	cookies, err := testutils.SSOCookieNames(client)
	ts.Require().NoError(err)
	ts.Require().NotEmpty(cookies, "the first sign-in starts an SSO session")

	_, executionID = ts.startAuthorization(client, "", ts.ssoChildClientID, "sso-reuse")
	reused := ts.executeWith(client, map[string]interface{}{"executionId": executionID})
	ts.Equal("COMPLETE", reused.FlowStatus, "the session is reused in the child without credentials: %+v",
		reused.Error)
	ts.NotEmpty(reused.Assertion)

	_, executionID = ts.startAuthorization(client, "", ts.ssoForeignClientID, "sso-foreign")
	foreign := ts.executeWith(client, map[string]interface{}{"executionId": executionID})
	ts.NotEqual("COMPLETE", foreign.FlowStatus, "the session's user is not in the foreign OU")
	ts.Empty(foreign.Assertion, "the session is not reused there")
	ts.True(foreign.Error == nil || foreign.Error.Code != errCodeNoAccessToApplication,
		"the sign-in is not refused: %+v", foreign.Error)
	ts.True(common.HasInput(foreign.Data.Inputs, "username"), "the foreign OU's sign-in asks for credentials")
}

// A live SSO session is reused by an application in another OU where its user is eligible, and that sign-in
// lands in the joining application's OU, not the one the session started in.
func (ts *OUBoundedSignInTestSuite) TestSSOReuse_IntoAnEligibleOUKeepsTheJoiningOU() {
	parentClientID := "ou_boundary_sso_parent"
	ts.createRedirectApp("OU Boundary SSO Parent", parentClientID, ts.parentOUID, ts.ssoFlowID, ts.allowedTypes)
	client, err := testutils.NewSessionClient()
	ts.Require().NoError(err)

	authID, executionID := ts.startAuthorization(client, "", ts.ssoChildClientID, "sso-eligible-first")
	first := ts.credentialSignIn(client, executionID, ouBoundarySharedUser)
	ts.Require().Equal("COMPLETE", first.FlowStatus, "the user is shared into the child: %+v", first.Error)
	ts.Equal(ts.childOUID, ts.tokenOUID(ts.completeToAccessToken(client, authID, first.Assertion,
		ts.ssoChildClientID)), "the session starts in the child")

	authID, executionID = ts.startAuthorization(client, "", parentClientID, "sso-eligible-join")
	joined := ts.executeWith(client, map[string]interface{}{"executionId": executionID})
	ts.Require().Equal("COMPLETE", joined.FlowStatus, "the session is reused without credentials: %+v",
		joined.Error)
	ts.Equal(ts.parentOUID, ts.tokenOUID(ts.completeToAccessToken(client, authID, joined.Assertion,
		parentClientID)), "the joining sign-in lands in its own application's OU")
}

// prompt=none on the OU path answers from a live SSO session inside the path's OU only: each eligible OU gets a
// code whose token names it, and an OU the session's user is not eligible in gets no code.
func (ts *OUBoundedSignInTestSuite) TestOUPathAuthorize_PromptNoneIsBoundedToThePathOU() {
	sharedClientID := "ou_boundary_sso_shared"
	appID := ts.createRedirectApp("OU Boundary SSO Shared", sharedClientID, ts.parentOUID, ts.ssoFlowID,
		ts.allowedTypes)
	ts.Require().NoError(testutils.ShareApplicationWithChildUnits(appID))
	client, err := testutils.NewSessionClient()
	ts.Require().NoError(err)

	authID, executionID := ts.startAuthorization(client, "", ts.ssoChildClientID, "prompt-none-first")
	first := ts.credentialSignIn(client, executionID, ouBoundarySharedUser)
	ts.Require().Equal("COMPLETE", first.FlowStatus, "the user is shared into the child: %+v", first.Error)
	_, err = testutils.CompleteAuthorizationWithClient(client, authID, first.Assertion)
	ts.Require().NoError(err)

	silent := func(ouID, state string) (url.Values, string) {
		resp := ts.authorizeWith(client, ouID, sharedClientID, state, map[string]string{"prompt": "none"})
		resp.Body.Close()
		ts.Require().Equal(http.StatusFound, resp.StatusCode)
		location := resp.Header.Get("Location")
		ts.Require().True(strings.HasPrefix(location, ouBoundaryRedirectURI),
			"prompt=none answers the client directly: %s", location)
		redirect, err := url.Parse(location)
		ts.Require().NoError(err)
		ts.Equal(state, redirect.Query().Get("state"))
		return redirect.Query(), location
	}

	for _, ouID := range []string{ts.parentOUID, ts.childOUID} {
		query, location := silent(ouID, "prompt-none-eligible")
		ts.Require().Empty(query.Get("error"), "the session's user is eligible there: %s", location)
		ts.Equal(ouID, ts.tokenOUID(ts.redeemCode(location, sharedClientID)), "the token names the path's OU")
	}

	query, location := silent(ts.childTwoOUID, "prompt-none-ineligible")
	ts.Empty(query.Get("code"), "no code for an OU the session's user is not eligible in: %s", location)
	ts.Equal("login_required", query.Get("error"), "the session cannot be used there, so a sign-in is needed: %s",
		location)
}
