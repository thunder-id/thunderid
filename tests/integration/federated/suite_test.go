// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/*
Federated attribute-mapping and user-type-resolution integration tests.

These exercise a generic OIDC connection end to end: a mock IdP returns claims, a registration flow
provisions a local user from them, and the assertions read the provisioned user back through the API.
Mapped claim *values* are not observable on the direct /auth endpoints, whose response carries only
{id, type, ouId, assertion}, so provisioning is the only surface that shows what a mapping produced.

One connection, one flow and one application are created for the whole suite; each test PUTs the
attributeConfiguration it needs onto that connection and drives the flow with its own mock user. That
avoids standing up a flow per scenario, and keeps the mock claim set and the configuration under test
next to each other in the test body.
*/
package federated

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// mockOIDCPort is 8100 because 8092-8099, 9091 and 9092 are already bound by other suites, and
// 8092/8093 are additionally pinned to the GitHub and Google mocks by the deployment.yaml override in
// testutils/test_utils.go.
const (
	mockOIDCPort  = 8100
	mockOAuthPort = 8101

	// The GitHub mock must bind 8092: Google and GitHub connections carry no configurable endpoints,
	// so the harness rewrites the scheme and host of their hardcoded defaults to the base URLs in
	// deployment.yaml, and github_base_url is pinned to this port. Paths are preserved, which is why
	// the mock mirrors GitHub's real ones.
	mockGitHubPort = 8092
)

const (
	oidcClientID     = "federated-test-client"
	oidcClientSecret = "federated-test-secret"
)

// fedPersonType is the type identities provision into. It carries every mapping target the scenarios
// use. email is required and unique so this type does not suppress the deployment-wide email
// account-linking default for other suites; costCenter is deliberately not unique, since a linking
// attribute that allows duplicates is what the ambiguity scenarios need in a later phase.
var fedPersonType = testutils.UserType{
	Name:                  "fed_person",
	AllowSelfRegistration: true,
	Schema: map[string]interface{}{
		"username":   map[string]interface{}{"type": "string", "required": true, "unique": true},
		"email":      map[string]interface{}{"type": "string", "required": true, "unique": true},
		"firstName":  map[string]interface{}{"type": "string"},
		"lastName":   map[string]interface{}{"type": "string"},
		"city":       map[string]interface{}{"type": "string"},
		"costCenter": map[string]interface{}{"type": "string"},
		"sub":        map[string]interface{}{"type": "string"},
	},
}

// fedContractorType exists so a second mapping profile can be keyed to it. Nothing provisions into it:
// userTypeResolution selects which profile applies, not which type the identity becomes (G17), so this
// type is never a provisioning target and deliberately does not allow self registration — that also
// keeps the flow's user-type resolution unambiguous.
var fedContractorType = testutils.UserType{
	Name:                  "fed_contractor",
	AllowSelfRegistration: false,
	Schema: map[string]interface{}{
		"username":       map[string]interface{}{"type": "string", "required": true, "unique": true},
		"email":          map[string]interface{}{"type": "string", "required": true, "unique": true},
		"firstName":      map[string]interface{}{"type": "string"},
		"employeeNumber": map[string]interface{}{"type": "string"},
		"sub":            map[string]interface{}{"type": "string"},
	},
}

var fedTestOU = testutils.OrganizationUnit{
	Handle:      "federated-mapping-ou",
	Name:        "Federated Mapping Test OU",
	Description: "Organization unit for federated attribute mapping tests",
	Parent:      nil,
}

// fedRegistrationFlow provisions a local user from the federated identity. The node list mirrors the
// proven Google registration flow, with the generic OIDCAuthExecutor in place of the Google one.
var fedRegistrationFlow = testutils.Flow{
	Name:     "Federated Mapping Registration Flow",
	FlowType: "REGISTRATION",
	Handle:   "registration_flow_federated_mapping",
	Nodes: []map[string]interface{}{
		{"id": "start", "type": "START", "onSuccess": "user_type_resolver"},
		{
			// REGISTRATION flows are required to carry a UserTypeResolver. The application allows a single
			// user type, so it resolves without input and the prompt below is never reached; it exists
			// because onIncomplete must name a node.
			"id":           "user_type_resolver",
			"type":         "TASK_EXECUTION",
			"executor":     map[string]interface{}{"name": "UserTypeResolver"},
			"onSuccess":    "oidc_auth",
			"onIncomplete": "prompt_usertype",
		},
		{
			"id":   "prompt_usertype",
			"type": "PROMPT",
			"meta": map[string]interface{}{
				"components": []map[string]interface{}{
					{
						"type": "BLOCK",
						"id":   "block_usertype",
						"components": []map[string]interface{}{
							{
								"type": "SELECT", "id": "usertype_input", "ref": "userType",
								"label": "User Type", "required": true, "options": []interface{}{},
							},
							{
								"type": "ACTION", "id": "action_usertype",
								"label": "Continue", "variant": "PRIMARY", "eventType": "SUBMIT",
							},
						},
					},
				},
			},
			"prompts": []map[string]interface{}{
				{
					"inputs": []map[string]interface{}{
						{"ref": "usertype_input", "identifier": "userType", "type": "SELECT", "required": true},
					},
					"action": map[string]interface{}{"ref": "action_usertype", "nextNode": "user_type_resolver"},
				},
			},
		},
		{
			"id":         "oidc_auth",
			"type":       "TASK_EXECUTION",
			"properties": map[string]interface{}{"idpId": "placeholder-idp-id"},
			"executor":   map[string]interface{}{"name": "OIDCAuthExecutor"},
			"onSuccess":  "provisioning",
		},
		{
			"id":        "provisioning",
			"type":      "TASK_EXECUTION",
			"executor":  map[string]interface{}{"name": "ProvisioningExecutor"},
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

// fedAuthFlow authenticates an existing local user through the federated connection. The federated node
// allows authentication without a local user, so an unmatched identity proceeds instead of failing —
// that branch is the one piece of executor behaviour that depends on the flow type.
var fedAuthFlow = testutils.Flow{
	Name:     "Federated Mapping Authentication Flow",
	FlowType: "AUTHENTICATION",
	Handle:   "auth_flow_federated_mapping",
	Nodes: []map[string]interface{}{
		{"id": "start", "type": "START", "onSuccess": "oidc_auth"},
		{
			"id":   "oidc_auth",
			"type": "TASK_EXECUTION",
			"properties": map[string]interface{}{
				"idpId":                               "placeholder-idp-id",
				"allowAuthenticationWithoutLocalUser": true,
			},
			"executor":  map[string]interface{}{"name": "OIDCAuthExecutor"},
			"onSuccess": "provisioning",
		},
		{
			// The allowance only marks the identity eligible for provisioning; a flow still has to carry
			// a provisioning step for an unmatched identity to become a user. Without one the flow reaches
			// the assertion with nothing to assert about.
			"id":        "provisioning",
			"type":      "TASK_EXECUTION",
			"executor":  map[string]interface{}{"name": "ProvisioningExecutor"},
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

// fedStrictAuthFlow is the same graph without the property, so an unmatched identity has nothing to
// authenticate and the flow cannot complete.
var fedStrictAuthFlow = testutils.Flow{
	Name:     "Federated Mapping Strict Authentication Flow",
	FlowType: "AUTHENTICATION",
	Handle:   "auth_flow_federated_mapping_strict",
	Nodes: []map[string]interface{}{
		{"id": "start", "type": "START", "onSuccess": "oidc_auth"},
		{
			"id":         "oidc_auth",
			"type":       "TASK_EXECUTION",
			"properties": map[string]interface{}{"idpId": "placeholder-idp-id"},
			"executor":   map[string]interface{}{"name": "OIDCAuthExecutor"},
			"onSuccess":  "auth_assert",
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

// fedOAuthFlow is the generic OAuth counterpart of the strict OIDC authentication flow, so the
// OAuthExecutor is driven as a flow node rather than only through the direct endpoints.
var fedOAuthFlow = testutils.Flow{
	Name:     "Federated Mapping OAuth Authentication Flow",
	FlowType: "AUTHENTICATION",
	Handle:   "auth_flow_federated_mapping_oauth",
	Nodes: []map[string]interface{}{
		{"id": "start", "type": "START", "onSuccess": "oidc_auth"},
		{
			// The node id stays oidc_auth so the shared connection-id patcher finds it.
			"id":         "oidc_auth",
			"type":       "TASK_EXECUTION",
			"properties": map[string]interface{}{"idpId": "placeholder-idp-id"},
			"executor":   map[string]interface{}{"name": "OAuthExecutor"},
			"onSuccess":  "auth_assert",
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

// fedAuthzFlow authenticates a federated identity and then runs an authorization check against
// whatever roles/groups/permissions its claims mapped to (plus any it holds directly), so mapped
// access can be observed on the resulting assertion's authorized_permissions claim. Provisioning
// precedes the check so a just-in-time-created entity's own assignments are visible to it, matching
// the architecture's Fed -> Prov -> Authz -> Assert ordering.
var fedAuthzFlow = testutils.Flow{
	Name:     "Federated Authorization Mapping Auth Flow",
	FlowType: "AUTHENTICATION",
	Handle:   "auth_flow_federated_authz_mapping",
	Nodes: []map[string]interface{}{
		{"id": "start", "type": "START", "onSuccess": "oidc_auth"},
		{
			"id":   "oidc_auth",
			"type": "TASK_EXECUTION",
			"properties": map[string]interface{}{
				"idpId":                               "placeholder-idp-id",
				"allowAuthenticationWithoutLocalUser": true,
			},
			"executor":  map[string]interface{}{"name": "OIDCAuthExecutor"},
			"onSuccess": "provisioning",
		},
		{
			"id":        "provisioning",
			"type":      "TASK_EXECUTION",
			"executor":  map[string]interface{}{"name": "ProvisioningExecutor"},
			"onSuccess": "authorization_check",
		},
		{
			"id":        "authorization_check",
			"type":      "TASK_EXECUTION",
			"executor":  map[string]interface{}{"name": "AuthorizationExecutor"},
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

var fedAuthzApp = testutils.Application{
	Name:             "Federated Authorization Mapping Test Application",
	Description:      "Application whose authentication flow checks authorization after a federated login",
	ClientID:         "federated_authz_mapping_test_client",
	ClientSecret:     "federated_authz_mapping_test_secret",
	RedirectURIs:     []string{"http://localhost:3000/callback"},
	AllowedUserTypes: []string{fedPersonType.Name},
	AssertionConfig: map[string]interface{}{
		"userAttributes": []string{"userType", "ouId", "ouName", "ouHandle"},
	},
}

var fedOAuthApp = testutils.Application{
	Name:             "Federated OAuth Authentication Test Application",
	Description:      "Application whose authentication flow uses the generic OAuth executor",
	ClientID:         "federated_oauth_auth_test_client",
	ClientSecret:     "federated_oauth_auth_test_secret",
	RedirectURIs:     []string{"http://localhost:3000/callback"},
	AllowedUserTypes: []string{fedPersonType.Name},
	AssertionConfig: map[string]interface{}{
		"userAttributes": []string{"userType", "ouId", "ouName", "ouHandle"},
	},
}

var fedAuthApp = testutils.Application{
	Name:             "Federated Authentication Test Application",
	Description:      "Application whose authentication flow allows an unmatched federated identity",
	ClientID:         "federated_auth_test_client",
	ClientSecret:     "federated_auth_test_secret",
	RedirectURIs:     []string{"http://localhost:3000/callback"},
	AllowedUserTypes: []string{fedPersonType.Name},
	AssertionConfig: map[string]interface{}{
		"userAttributes": []string{"userType", "ouId", "ouName", "ouHandle"},
	},
}

var fedStrictAuthApp = testutils.Application{
	Name:             "Federated Strict Authentication Test Application",
	Description:      "Application whose authentication flow requires a local user",
	ClientID:         "federated_strict_auth_test_client",
	ClientSecret:     "federated_strict_auth_test_secret",
	RedirectURIs:     []string{"http://localhost:3000/callback"},
	AllowedUserTypes: []string{fedPersonType.Name},
	AssertionConfig: map[string]interface{}{
		"userAttributes": []string{"userType", "ouId", "ouName", "ouHandle"},
	},
}

var fedTestApp = testutils.Application{
	Name:                      "Federated Mapping Test Application",
	Description:               "Application for federated attribute mapping tests",
	IsRegistrationFlowEnabled: true,
	ClientID:                  "federated_mapping_test_client",
	ClientSecret:              "federated_mapping_test_secret",
	RedirectURIs:              []string{"http://localhost:3000/callback"},
	AllowedUserTypes:          []string{fedPersonType.Name},
	AssertionConfig: map[string]interface{}{
		"userAttributes": []string{"userType", "ouId", "ouName", "ouHandle"},
	},
}

type FederatedMappingSuite struct {
	suite.Suite
	mockOIDC        *testutils.MockOIDCServer
	ouID            string
	typeIDs         []string
	idpID           string
	appID           string
	authAppID       string
	strictAuthAppID string
	config          *common.TestSuiteConfig
	activeSub       string
	subCounter      int
	jwksSuffix      string
	mockOAuth       *testutils.MockOAuthServer
	oauthIDPID      string
	oauthAppID      string
	mockGitHub      *testutils.MockGithubOAuthServer
	githubIDPID     string
	perTestAppIDs   []string
	perTestFlowIDs  []string

	// Authorization mapping fixtures: an application whose auth flow runs an AuthorizationExecutor
	// after the federated login, two resource servers (to prove a mapped permission stays scoped to
	// its own resource server), a role with no DB assignees (to prove the LEFT JOIN fix resolves a
	// mapped role by identifier alone), and a group whose own role assignment grants access to
	// whoever a mapping places in the group.
	authzAppID                 string
	authzResourceServerID      string
	authzOtherResourceServerID string
	authzMappedRoleID          string
	authzMappedGroupID         string
	authzGroupRoleID           string
}

func TestFederatedMappingSuite(t *testing.T) {
	suite.Run(t, new(FederatedMappingSuite))
}

func (s *FederatedMappingSuite) SetupSuite() {
	s.config = &common.TestSuiteConfig{}

	mock, err := testutils.NewMockOIDCServer(mockOIDCPort, oidcClientID, oidcClientSecret)
	s.Require().NoError(err, "failed to create mock OIDC server")
	s.mockOIDC = mock

	// The mock authorizes whichever subject the running test selected, so each scenario controls the
	// exact claim set it is asserting on.
	s.mockOIDC.SetAuthorizeFunc(func(string) (string, error) {
		if s.activeSub == "" {
			return "", fmt.Errorf("no active subject selected by the test")
		}
		return s.activeSub, nil
	})
	s.Require().NoError(s.mockOIDC.Start(), "failed to start mock OIDC server")

	ouID, err := testutils.CreateOrganizationUnit(fedTestOU)
	s.Require().NoError(err, "failed to create organization unit")
	s.ouID = ouID

	for _, userType := range []testutils.UserType{fedPersonType, fedContractorType} {
		userType.OUID = ouID
		typeID, err := testutils.CreateUserType(userType)
		s.Require().NoError(err, "failed to create user type %s", userType.Name)
		s.typeIDs = append(s.typeIDs, typeID)
	}

	idpID, err := testutils.CreateIDP(s.idpFixture(nil))
	s.Require().NoError(err, "failed to create OIDC connection")
	s.idpID = idpID
	s.config.CreatedIdpIDs = append(s.config.CreatedIdpIDs, idpID)

	// Located by node id rather than index: the node list has already gained a required executor once,
	// and an index would have silently written the connection id onto the wrong node.
	nodes := fedRegistrationFlow.Nodes.([]map[string]interface{})
	var patched bool
	for _, node := range nodes {
		if node["id"] == "oidc_auth" {
			node["properties"].(map[string]interface{})["idpId"] = idpID
			patched = true
		}
	}
	s.Require().True(patched, "the flow must contain an oidc_auth node to carry the connection id")
	fedRegistrationFlow.Nodes = nodes

	flowID, err := testutils.CreateFlow(fedRegistrationFlow)
	s.Require().NoError(err, "failed to create registration flow")
	s.config.CreatedFlowIDs = append(s.config.CreatedFlowIDs, flowID)
	fedTestApp.RegistrationFlowID = flowID

	authFlowID, err := testutils.CreateIsolatedAuthFlow("federated-mapping-isolated-auth")
	s.Require().NoError(err, "failed to create isolated auth flow")
	s.config.CreatedFlowIDs = append(s.config.CreatedFlowIDs, authFlowID)
	fedTestApp.AuthFlowID = authFlowID

	fedTestApp.OUID = ouID
	appID, err := testutils.CreateApplication(fedTestApp)
	s.Require().NoError(err, "failed to create application")
	s.appID = appID

	oauthMock := testutils.NewMockOAuthServer(mockOAuthPort, oidcClientID, oidcClientSecret)
	s.Require().NoError(oauthMock.Start(), "failed to start mock OAuth server")
	s.mockOAuth = oauthMock
	s.mockOAuth.SetAuthorizeFunc(func(string) (string, error) {
		if s.activeSub == "" {
			return "", fmt.Errorf("no active subject selected by the test")
		}
		return s.activeSub, nil
	})

	oauthIDPID, err := testutils.CreateIDP(testutils.IDP{
		Name: "Federated Mapping OAuth Connection", Type: "OAUTH",
		Properties: []testutils.IDPProperty{
			{Name: "client_id", Value: oidcClientID},
			{Name: "client_secret", Value: oidcClientSecret, IsSecret: true},
			{Name: "redirect_uri", Value: "http://localhost:3000/callback"},
			{Name: "authorization_endpoint", Value: s.mockOAuth.GetAuthorizeURL()},
			{Name: "token_endpoint", Value: s.mockOAuth.GetTokenURL()},
			{Name: "userinfo_endpoint", Value: s.mockOAuth.GetUserInfoURL()},
			{Name: "scopes", Value: "openid,email,profile"},
		},
	})
	s.Require().NoError(err, "failed to create OAuth connection")
	s.oauthIDPID = oauthIDPID

	githubMock := testutils.NewMockGithubOAuthServer(mockGitHubPort, oidcClientID, oidcClientSecret)
	s.Require().NoError(githubMock.Start(), "failed to start mock GitHub server")
	s.mockGitHub = githubMock
	// Without this the mock authorizes whichever user it happens to hold first, so a scenario's
	// selection is ignored and its identity never matches.
	s.mockGitHub.SetAuthorizeFunc(func(string) (string, error) {
		if s.activeSub == "" {
			return "", fmt.Errorf("no active subject selected by the test")
		}
		return s.activeSub, nil
	})

	// No endpoints are set: a GitHub connection has none to set, and supplying them would bypass the
	// base-URL override that points the defaults at the mock.
	githubIDPID, err := testutils.CreateIDP(testutils.IDP{
		Name: "Federated Mapping GitHub Connection", Type: "GITHUB",
		Properties: []testutils.IDPProperty{
			{Name: "client_id", Value: oidcClientID},
			{Name: "client_secret", Value: oidcClientSecret, IsSecret: true},
			{Name: "redirect_uri", Value: "http://localhost:3000/callback"},
		},
	})
	s.Require().NoError(err, "failed to create GitHub connection")
	s.githubIDPID = githubIDPID

	s.authAppID = s.createAuthApp(&fedAuthFlow, fedAuthApp, idpID, ouID, "federated-auth-isolated-reg")
	s.strictAuthAppID = s.createAuthApp(
		&fedStrictAuthFlow, fedStrictAuthApp, idpID, ouID, "federated-strict-auth-isolated-reg")
	s.oauthAppID = s.createAuthApp(
		&fedOAuthFlow, fedOAuthApp, oauthIDPID, ouID, "federated-oauth-auth-isolated-reg")

	s.setupAuthzFixtures(idpID, ouID)
}

// setupAuthzFixtures creates the resource servers, roles, group, and authorization-checking
// application shared by the AuthorizationMapping scenarios.
func (s *FederatedMappingSuite) setupAuthzFixtures(idpID, ouID string) {
	s.T().Helper()

	rsID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:       "Federated Authorization Mapping API",
		Identifier: "federated-authz-mapping-api",
		OUID:       ouID,
	}, []testutils.Action{
		{Name: "Read", Handle: "read", Description: "Read access"},
		{Name: "Write", Handle: "write", Description: "Write access"},
		{Name: "Delete", Handle: "delete", Description: "Delete access"},
	})
	s.Require().NoError(err, "failed to create the authorization mapping resource server")
	s.authzResourceServerID = rsID

	// A second resource server with the same permission handle, so a mapped permission target scoped
	// to the first resource server can be proven not to leak into an evaluation against this one.
	otherRSID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:       "Federated Authorization Mapping Other API",
		Identifier: "federated-authz-mapping-other-api",
		OUID:       ouID,
	}, []testutils.Action{
		{Name: "Read", Handle: "read", Description: "Read access"},
	})
	s.Require().NoError(err, "failed to create the second authorization mapping resource server")
	s.authzOtherResourceServerID = otherRSID

	// No assignments: this role is reachable only by a mapping naming it directly, exercising the
	// LEFT JOIN fix that lets a role with zero assignment rows still resolve by identifier.
	mappedRoleID, err := testutils.CreateRole(testutils.Role{
		Name: "Federated Mapped Reader",
		OUID: ouID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: rsID, Permissions: []string{"read"}},
		},
	})
	s.Require().NoError(err, "failed to create the mapped role with no assignees")
	s.authzMappedRoleID = mappedRoleID

	groupID, err := testutils.CreateGroup(testutils.Group{Name: "Federated Mapped Editors", OUID: ouID})
	s.Require().NoError(err, "failed to create the mapped group")
	s.authzMappedGroupID = groupID

	groupRoleID, err := testutils.CreateRole(testutils.Role{
		Name: "Federated Group Writer",
		OUID: ouID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: rsID, Permissions: []string{"write"}},
		},
		Assignments: []testutils.Assignment{{ID: groupID, Type: "group"}},
	})
	s.Require().NoError(err, "failed to create the role assigned to the mapped group")
	s.authzGroupRoleID = groupRoleID

	s.authzAppID = s.createAuthApp(&fedAuthzFlow, fedAuthzApp, idpID, ouID, "federated-authz-isolated-reg")
}

// createAuthApp creates one authentication flow and the application that runs it. Both applications
// need a registration flow of their own, because an application referencing another application's flow
// fails cross-type reference validation.
func (s *FederatedMappingSuite) createAuthApp(
	flow *testutils.Flow, app testutils.Application, idpID, ouID, regFlowHandle string) string {
	s.T().Helper()

	nodes := flow.Nodes.([]map[string]interface{})
	var patched bool
	for _, node := range nodes {
		if node["id"] == "oidc_auth" {
			node["properties"].(map[string]interface{})["idpId"] = idpID
			patched = true
		}
	}
	s.Require().True(patched, "the authentication flow must contain an oidc_auth node")
	flow.Nodes = nodes

	flowID, err := testutils.CreateFlow(*flow)
	s.Require().NoError(err, "failed to create authentication flow %s", flow.Handle)
	s.config.CreatedFlowIDs = append(s.config.CreatedFlowIDs, flowID)

	regFlowID, err := testutils.CreateIsolatedRegistrationFlow(regFlowHandle)
	s.Require().NoError(err, "failed to create isolated registration flow %s", regFlowHandle)
	s.config.CreatedFlowIDs = append(s.config.CreatedFlowIDs, regFlowID)

	app.OUID = ouID
	app.AuthFlowID = flowID
	app.RegistrationFlowID = regFlowID
	appID, err := testutils.CreateApplication(app)
	s.Require().NoError(err, "failed to create application %s", app.Name)
	return appID
}

// createScenarioApp creates a flow and an application that runs it, for scenarios whose whole point is a
// graph the shared flows cannot express. Both are torn down after the test.
func (s *FederatedMappingSuite) createScenarioApp(flow testutils.Flow, clientID string) string {
	s.T().Helper()
	flowID, err := testutils.CreateFlow(flow)
	s.Require().NoError(err, "failed to create flow %s", flow.Handle)
	s.perTestFlowIDs = append(s.perTestFlowIDs, flowID)

	regFlowID, err := testutils.CreateIsolatedRegistrationFlow(clientID + "-reg")
	s.Require().NoError(err, "failed to create the isolated registration flow")
	s.perTestFlowIDs = append(s.perTestFlowIDs, regFlowID)

	appID, err := testutils.CreateApplication(testutils.Application{
		Name:               "Scenario App " + clientID,
		ClientID:           clientID,
		ClientSecret:       clientID + "-secret",
		RedirectURIs:       []string{"http://localhost:3000/callback"},
		AllowedUserTypes:   []string{fedPersonType.Name},
		OUID:               s.ouID,
		AuthFlowID:         flowID,
		RegistrationFlowID: regFlowID,
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{"userType", "ouId"},
		},
	})
	s.Require().NoError(err, "failed to create the scenario application")
	s.perTestAppIDs = append(s.perTestAppIDs, appID)
	return appID
}

func (s *FederatedMappingSuite) TearDownTest() {
	s.jwksSuffix = ""
	for _, appID := range s.perTestAppIDs {
		if err := testutils.DeleteApplication(appID); err != nil {
			s.T().Logf("failed to delete scenario application: %v", err)
		}
	}
	s.perTestAppIDs = nil
	for _, flowID := range s.perTestFlowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			s.T().Logf("failed to delete scenario flow: %v", err)
		}
	}
	s.perTestFlowIDs = nil
	if len(s.config.CreatedUserIDs) > 0 {
		if err := testutils.CleanupUsers(s.config.CreatedUserIDs); err != nil {
			s.T().Logf("failed to clean up users: %v", err)
		}
		s.config.CreatedUserIDs = nil
	}
}

func (s *FederatedMappingSuite) TearDownSuite() {
	for _, appID := range []string{s.appID, s.authAppID, s.strictAuthAppID, s.oauthAppID, s.authzAppID} {
		if appID == "" {
			continue
		}
		if err := testutils.DeleteApplication(appID); err != nil {
			s.T().Logf("failed to delete application %s: %v", appID, err)
		}
	}
	for _, flowID := range s.config.CreatedFlowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			s.T().Logf("failed to delete flow %s: %v", flowID, err)
		}
	}
	for _, roleID := range []string{s.authzMappedRoleID, s.authzGroupRoleID} {
		if roleID == "" {
			continue
		}
		if err := testutils.DeleteRole(roleID); err != nil {
			s.T().Logf("failed to delete role %s: %v", roleID, err)
		}
	}
	if s.authzMappedGroupID != "" {
		if err := testutils.DeleteGroup(s.authzMappedGroupID); err != nil {
			s.T().Logf("failed to delete group %s: %v", s.authzMappedGroupID, err)
		}
	}
	for _, rsID := range []string{s.authzResourceServerID, s.authzOtherResourceServerID} {
		if rsID == "" {
			continue
		}
		if err := testutils.DeleteResourceServer(rsID); err != nil {
			s.T().Logf("failed to delete resource server %s: %v", rsID, err)
		}
	}
	if s.idpID != "" {
		if err := testutils.DeleteIDP(s.idpID); err != nil {
			s.T().Logf("failed to delete connection: %v", err)
		}
	}
	for _, typeID := range s.typeIDs {
		if err := testutils.DeleteUserType(typeID); err != nil {
			s.T().Logf("failed to delete user type %s: %v", typeID, err)
		}
	}
	if s.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(s.ouID); err != nil {
			s.T().Logf("failed to delete organization unit: %v", err)
		}
	}
	for _, id := range []string{s.oauthIDPID, s.githubIDPID} {
		if id == "" {
			continue
		}
		if err := testutils.DeleteIDP(id); err != nil {
			s.T().Logf("failed to delete connection %s: %v", id, err)
		}
	}
	if s.mockOAuth != nil {
		if err := s.mockOAuth.Stop(); err != nil {
			s.T().Logf("failed to stop mock OAuth server: %v", err)
		}
	}
	if s.mockGitHub != nil {
		if err := s.mockGitHub.Stop(); err != nil {
			s.T().Logf("failed to stop mock GitHub server: %v", err)
		}
	}
	if s.mockOIDC != nil {
		if err := s.mockOIDC.Stop(); err != nil {
			s.T().Logf("failed to stop mock OIDC server: %v", err)
		}
	}
}

// idpFixture builds the connection body. The JWKS endpoint is configured so ID-token signatures are
// actually verified rather than skipped.
func (s *FederatedMappingSuite) idpFixture(config *testutils.AttributeConfiguration) testutils.IDP {
	return testutils.IDP{
		Name:        "Federated Mapping OIDC Connection",
		Description: "Generic OIDC connection for federated attribute mapping tests",
		Type:        "OIDC",
		Properties: []testutils.IDPProperty{
			{Name: "client_id", Value: oidcClientID},
			{Name: "client_secret", Value: oidcClientSecret, IsSecret: true},
			{Name: "redirect_uri", Value: "http://localhost:3000/callback"},
			{Name: "authorization_endpoint", Value: s.mockOIDC.GetAuthorizeURL()},
			{Name: "token_endpoint", Value: s.mockOIDC.GetTokenURL()},
			{Name: "userinfo_endpoint", Value: s.mockOIDC.GetUserInfoURL()},
			{Name: "jwks_endpoint", Value: s.mockOIDC.GetJWKSURL() + s.jwksSuffix},
			{Name: "scopes", Value: "openid,email,profile"},
		},
		AttributeConfiguration: config,
	}
}

// setOAuthScopes rewrites just the scopes on the OAuth connection, for the delimiter scenarios.
func (s *FederatedMappingSuite) setOAuthScopes(scopes string) {
	s.T().Helper()
	current, err := testutils.GetIDP("oauth", s.oauthIDPID)
	s.Require().NoError(err, "failed to read the OAuth connection")
	s.Require().NotNil(current)

	// The masked secret must not be echoed back, and the scopes property is replaced rather than appended.
	kept := current.Properties[:0]
	for _, property := range current.Properties {
		if property.Name == "client_secret" || property.Name == "scopes" {
			continue
		}
		kept = append(kept, property)
	}
	if scopes != "" {
		kept = append(kept, testutils.IDPProperty{Name: "scopes", Value: scopes})
	}
	current.Properties = kept
	s.Require().NoError(testutils.UpdateIDP(s.oauthIDPID, *current), "failed to set scopes")
}

// applyConfigTo replaces the attribute configuration of a connection, which must be addressed under its
// own vendor path. Errors are asserted rather than swallowed: an earlier version returned silently on a
// failed read, which turned a wrong-vendor 404 into "the configuration simply never applied" and made
// the GitHub scenarios fail somewhere unrelated.
func (s *FederatedMappingSuite) applyConfigTo(
	vendor, idpID string, config *testutils.AttributeConfiguration) {
	s.T().Helper()
	current, err := testutils.GetIDP(vendor, idpID)
	s.Require().NoError(err, "failed to read the %s connection", vendor)
	s.Require().NotNil(current, "the %s connection should exist", vendor)
	current.AttributeConfiguration = config

	// The read-back carries the secret masked as ******. Sending that straight back would overwrite the
	// stored credential with the mask and every later token exchange would fail with invalid_client —
	// which is exactly what happened before this filter existed. The API preserves the stored secret
	// when the field is omitted, so the masked property is dropped rather than echoed.
	kept := current.Properties[:0]
	for _, property := range current.Properties {
		if property.Name == "client_secret" {
			continue
		}
		kept = append(kept, property)
	}
	current.Properties = kept

	s.Require().NoError(testutils.UpdateIDP(idpID, *current), "failed to apply the configuration")
}

// jsonUnmarshal is a thin wrapper so callers can decode without importing encoding/json twice.
func jsonUnmarshal(data []byte, target interface{}) error {
	return json.Unmarshal(data, target)
}

// applyConfig replaces the connection's attribute configuration. Update never re-seeds, so what is sent
// here is exactly what the runtime reads.
func (s *FederatedMappingSuite) applyConfig(config *testutils.AttributeConfiguration) {
	s.T().Helper()
	s.Require().NoError(testutils.UpdateIDP(s.idpID, s.idpFixture(config)),
		"failed to apply the attribute configuration under test")
}

// useFreshJWKS points the connection at a JWKS URL nothing has fetched yet.
//
// The verifier caches key sets by URL with a TTL (jwt/service.go:381), so a suite that has already
// authenticated once holds a warm entry and never re-fetches. Without this, a test that breaks or
// rotates the published keys would still authenticate from the cache — which is exactly what happened
// before this existed, and it made four scenarios pass for the wrong reason. The query string only
// changes the cache key; the mock routes on path and ignores it.
func (s *FederatedMappingSuite) useFreshJWKS() {
	s.T().Helper()
	s.subCounter++
	s.jwksSuffix = fmt.Sprintf("?cache=%d", s.subCounter)
}

// nextSubject returns a subject unique to this test run, so provisioned users never collide on the
// unique username and email attributes.
func (s *FederatedMappingSuite) nextSubject() string {
	s.subCounter++
	return fmt.Sprintf("fed-sub-%d-%d", s.subCounter, testutils.GetServerPID())
}

// register drives the registration flow for a mock identity carrying claims, and returns the
// attributes of the user it provisioned. It is the workhorse of this suite: every mapping and
// resolution scenario differs only in the claims it supplies and the configuration it applies.
func (s *FederatedMappingSuite) register(
	config *testutils.AttributeConfiguration, user *testutils.OIDCUserInfo) map[string]interface{} {
	s.T().Helper()
	s.applyConfig(config)
	s.mockOIDC.AddUser(user)
	s.activeSub = user.Sub

	flowStep, err := common.InitiateRegistrationFlow(s.appID, false, nil, "")
	s.Require().NoError(err, "failed to initiate the registration flow")
	s.Require().Equal("REDIRECTION", flowStep.Type,
		"expected a redirection to the identity provider, got %+v", flowStep)

	code, state, err := testutils.SimulateFederatedOAuthFlow(flowStep.Data.RedirectURL)
	s.Require().NoError(err, "failed to simulate authorization at the identity provider")

	completed, err := common.CompleteFlow(
		flowStep.ExecutionID, map[string]string{"code": code, "state": state}, "", flowStep.ChallengeToken)
	s.Require().NoError(err, "failed to complete the registration flow")
	s.Require().Equal("COMPLETE", completed.FlowStatus,
		"expected the flow to complete, got %+v", completed)

	provisioned, err := testutils.FindUserByAttribute("sub", user.Sub)
	s.Require().NoError(err, "failed to look up the provisioned user")
	s.Require().NotNil(provisioned, "no user was provisioned for subject %s", user.Sub)
	s.config.CreatedUserIDs = append(s.config.CreatedUserIDs, provisioned.ID)

	var attributes map[string]interface{}
	s.Require().NoError(json.Unmarshal(provisioned.Attributes, &attributes),
		"failed to decode the provisioned user's attributes")
	return attributes
}

// registerExpectingPrompt drives the flow for a configuration under which no mapping supplies a
// required local attribute, and returns the step the flow stops on. That is the user-visible
// consequence of a mapping profile not matching: provisioning has no username, because no provider
// emits a claim under that name, so it has to ask.
func (s *FederatedMappingSuite) registerExpectingPrompt(
	config *testutils.AttributeConfiguration, user *testutils.OIDCUserInfo) *common.FlowStep {
	s.T().Helper()
	s.applyConfig(config)
	s.mockOIDC.AddUser(user)
	s.activeSub = user.Sub

	flowStep, err := common.InitiateRegistrationFlow(s.appID, false, nil, "")
	s.Require().NoError(err, "failed to initiate the registration flow")

	code, state, err := testutils.SimulateFederatedOAuthFlow(flowStep.Data.RedirectURL)
	s.Require().NoError(err, "failed to simulate authorization at the identity provider")

	step, err := common.CompleteFlow(
		flowStep.ExecutionID, map[string]string{"code": code, "state": state}, "", flowStep.ChallengeToken)
	s.Require().NoError(err, "failed to advance the registration flow")
	return step
}

// assertPromptsFor asserts the flow stopped to collect the named input.
func (s *FederatedMappingSuite) assertPromptsFor(step *common.FlowStep, identifier string) {
	s.T().Helper()
	s.Require().Equal("INCOMPLETE", step.FlowStatus,
		"expected the flow to stop for input, got %+v", step)
	for _, input := range step.Data.Inputs {
		if input.Identifier == identifier {
			return
		}
	}
	s.Failf("missing expected prompt", "expected the flow to ask for %q, got inputs %+v",
		identifier, step.Data.Inputs)
}

// mustJSON marshals fixture attributes, failing the build rather than the assertion if they are invalid.
func mustJSON(value map[string]interface{}) []byte {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return encoded
}

// mapping is shorthand for a single-profile configuration keyed to the given user type.
func mapping(userType string, pairs ...testutils.AttributeMapping) *testutils.AttributeConfiguration {
	return &testutils.AttributeConfiguration{
		UserTypeResolution:        &testutils.UserTypeResolution{Default: userType},
		UserTypeAttributeMappings: []testutils.UserTypeAttributeMapping{{UserType: userType, Attributes: pairs}},
	}
}

// pair is shorthand for one external-to-local mapping.
func pair(external, local string) testutils.AttributeMapping {
	return testutils.AttributeMapping{ExternalAttribute: external, LocalAttribute: local}
}

// OIDCUser aliases the mock's user type so the ID-token override signatures stay readable.
type OIDCUser = testutils.OIDCUserInfo

// signedIDToken signs the given claims with the mock's key, using the kid the published JWKS advertises.
// Tests use it to mint tokens the mock would never produce on its own.
func (s *FederatedMappingSuite) signedIDToken(claims map[string]interface{}) (string, error) {
	return s.mockOIDC.SignJWT(
		map[string]interface{}{"alg": "RS256", "typ": "JWT", "kid": "oidc-key-1"}, claims)
}

// baseUser returns a mock identity whose claims satisfy the required local attributes, so a scenario
// only has to add the claims it is actually testing.
func (s *FederatedMappingSuite) baseUser(sub string) *testutils.OIDCUserInfo {
	return &testutils.OIDCUserInfo{
		Sub:           sub,
		Email:         sub + "@example.com",
		EmailVerified: true,
		Name:          "Federated Test User",
		GivenName:     "Federated",
		FamilyName:    "User",
		Custom:        map[string]interface{}{},
	}
}
