// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	// organizationTemplates is the designer's template file, so the flows under test are the shipped templates.
	organizationTemplates = "../../../../frontend/packages/configure-flows/src/data/templates.json"

	orgTemplatesUserType = "org-templates-member"
	orgTemplatesPassword = "OrgTemplates@123"
	orgSSOClientID       = "org_templates_oidc_client"
	orgSSOClientSecret   = "org_templates_oidc_secret"
)

// OrganizationTemplatesTestSuite runs the organization templates as the designer ships them. The parent stands for
// the unit customers sign up under: it owns the user type, the role and the applications, and every organization
// the sign-ups create is its direct child.
type OrganizationTemplatesTestSuite struct {
	suite.Suite

	parentOUID string
	userTypeID string
	// roleID is the parent's role, shared with every child with its assignments editable there.
	roleID string

	signUpAppID              string
	sharedAccountSignUpAppID string
	signInAppID              string
	// signInFlowID is the enterprise SSO template's flow, which the sign-up applications sign in with too: it calls
	// no sign-up flow, so it never conflicts with theirs.
	signInFlowID string

	// acmeOUID is an organization made ahead of the sign-in scenarios, with its own OpenID Connect connection
	// chosen as the sign-in flow's enterpriseIdp.
	acmeOUID string
	mockOIDC *testutils.MockOIDCServer
	idpID    string

	flowIDs []string
	appIDs  []string
	roleIDs []string
	userIDs []string
	// orgIDs are the organizations the sign-ups created, deleted before the parent.
	orgIDs []string
}

func TestOrganizationTemplatesTestSuite(t *testing.T) {
	suite.Run(t, new(OrganizationTemplatesTestSuite))
}

func (ts *OrganizationTemplatesTestSuite) SetupSuite() {
	var err error
	ts.parentOUID, err = testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "org-templates-parent", Name: "Organization Templates Parent",
	})
	ts.Require().NoError(err, "create the parent OU")
	ts.userTypeID, err = testutils.CreateUserType(testutils.UserType{
		Handle:                orgTemplatesUserType,
		DisplayName:           "Organization Templates Member",
		OUID:                  ts.parentOUID,
		AllowSelfRegistration: true,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"email":    map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	})
	ts.Require().NoError(err, "create the user type")

	ts.roleID = createRole(&ts.Suite, &ts.roleIDs, ts.parentOUID, "", "Organization Templates Admin", nil)
	status, raw := ts.request(http.MethodPost, "/roles/"+ts.roleID+"/sharing-policies", map[string]interface{}{
		"childUnits": map[string]interface{}{
			"mode":         "all",
			"overlayRules": map[string]interface{}{"assignments": map[string]interface{}{"editable": true}},
		},
	})
	ts.Require().Equal(http.StatusCreated, status, "share the role with every child: %s", raw)

	ts.setUpEnterpriseSSO()

	ts.signUpAppID = ts.createSignUpApp("Organization Sign-up", "org_templates_signup",
		ts.createFlow("ORGANIZATION_SIGNUP", "REGISTRATION", "org-templates-signup"))
	ts.sharedAccountSignUpAppID = ts.createSignUpApp("Organization Sign-up Shared Account", "org_templates_shared_signup",
		ts.createFlow("ORGANIZATION_SIGNUP_SHARED_ACCOUNT", "REGISTRATION", "org-templates-shared-signup"))
}

// setUpEnterpriseSSO shares the sign-in flow and its application with every child, and lets acme sign in with its
// own OpenID Connect connection.
func (ts *OrganizationTemplatesTestSuite) setUpEnterpriseSSO() {
	var err error
	ts.acmeOUID, err = testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "org-templates-acme", Name: "Organization Templates Acme", Parent: &ts.parentOUID,
	})
	ts.Require().NoError(err, "create acme")

	ts.mockOIDC, err = testutils.NewMockOIDCServer(orgSSOClientID, orgSSOClientSecret)
	ts.Require().NoError(err, "start the mock OpenID Connect provider")
	ts.idpID, err = testutils.CreateIDP(testutils.IDP{
		Name:        "Organization Templates Acme IdP",
		Description: "Acme's own OpenID Connect provider",
		Type:        "OIDC",
		OUID:        ts.acmeOUID,
		Properties: []testutils.IDPProperty{
			{Name: "client_id", Value: orgSSOClientID},
			{Name: "client_secret", Value: orgSSOClientSecret, IsSecret: true},
			{Name: "redirect_uri", Value: "http://localhost:3000/callback"},
			{Name: "authorization_endpoint", Value: ts.mockOIDC.GetAuthorizeURL()},
			{Name: "token_endpoint", Value: ts.mockOIDC.GetTokenURL()},
			{Name: "userinfo_endpoint", Value: ts.mockOIDC.GetUserInfoURL()},
			{Name: "jwks_endpoint", Value: ts.mockOIDC.GetJWKSURL()},
			{Name: "scopes", Value: "openid,email,profile"},
		},
	})
	ts.Require().NoError(err, "create acme's OpenID Connect connection")

	flowID := ts.createFlow("BASIC_ENTERPRISE_SSO", "AUTHENTICATION", "org-templates-sso")
	status, raw := ts.request(http.MethodPost, "/flows/"+flowID+"/sharing-policies", map[string]interface{}{
		"childUnits": map[string]interface{}{
			"mode": "all",
			"overlayRules": map[string]interface{}{
				"variables.enterpriseIdp": map[string]interface{}{"editable": true},
			},
		},
	})
	ts.Require().Equal(http.StatusCreated, status, "share the sign-in flow with every child: %s", raw)
	status, raw = ts.request(http.MethodPut, "/flows/"+flowID+"/variables", map[string]interface{}{
		"ouId": ts.acmeOUID, "values": map[string]interface{}{"enterpriseIdp": ts.idpID},
	})
	ts.Require().Equal(http.StatusOK, status, "choose acme's connection: %s", raw)

	ts.signInFlowID = flowID
	ts.signInAppID, err = testutils.CreateApplication(testutils.Application{
		Name:             "Organization Templates Sign-in",
		OUID:             ts.parentOUID,
		ClientID:         "org_templates_signin",
		ClientSecret:     "org_templates_signin_secret",
		AuthFlowID:       flowID,
		AllowedUserTypes: []string{orgTemplatesUserType},
		AssertionConfig:  map[string]interface{}{"userAttributes": []string{"ouId"}},
	})
	ts.Require().NoError(err, "create the sign-in application")
	ts.appIDs = append(ts.appIDs, ts.signInAppID)
	ts.Require().NoError(testutils.ShareApplicationWithChildUnits(ts.signInAppID), "share the sign-in application")
}

func (ts *OrganizationTemplatesTestSuite) TearDownSuite() {
	for _, id := range ts.appIDs {
		if err := testutils.DeleteApplication(id); err != nil {
			ts.T().Logf("teardown: delete application %s: %v", id, err)
		}
	}
	for _, id := range ts.userIDs {
		if err := testutils.DeleteUser(id); err != nil {
			ts.T().Logf("teardown: delete user %s: %v", id, err)
		}
	}
	for _, id := range ts.flowIDs {
		if err := testutils.DeleteFlow(id); err != nil {
			ts.T().Logf("teardown: delete flow %s: %v", id, err)
		}
	}
	if ts.idpID != "" {
		if err := testutils.DeleteIDP(ts.idpID); err != nil {
			ts.T().Logf("teardown: delete acme's connection: %v", err)
		}
	}
	if ts.mockOIDC != nil {
		if err := ts.mockOIDC.Stop(); err != nil {
			ts.T().Logf("teardown: stop the mock OpenID Connect provider: %v", err)
		}
	}
	for _, id := range ts.roleIDs {
		if err := testutils.DeleteRole(id); err != nil {
			ts.T().Logf("teardown: delete role %s: %v", id, err)
		}
	}
	if ts.userTypeID != "" {
		if err := testutils.DeleteUserType(ts.userTypeID); err != nil {
			ts.T().Logf("teardown: delete user type: %v", err)
		}
	}
	for _, id := range append(ts.orgIDs, ts.acmeOUID, ts.parentOUID) {
		if id != "" {
			if err := testutils.DeleteOrganizationUnit(id); err != nil {
				ts.T().Logf("teardown: delete OU %s: %v", id, err)
			}
		}
	}
}

// The user is created in the organization the sign-up creates beneath the user type's unit, and gets the named
// role there. The handle is left empty, so the organization's is made from its name.
func (ts *OrganizationTemplatesTestSuite) TestOrganizationSignUp_CreatesTheUserInTheNewOrganizationWithItsRoles() {
	username := common.GenerateUniqueUsername("orgsignup")

	ts.signUp(ts.signUpAppID, username, "Organization Templates Globex", "")

	user := ts.findUser(username)
	ts.Require().NotNil(user, "the sign-up creates the user")
	ts.userIDs = append(ts.userIDs, user.ID)
	ts.orgIDs = append(ts.orgIDs, user.OUID)
	ts.requireChildOfParent(user.OUID, "organization-templates-globex")
	ts.Equal([]string{user.ID}, ts.assignees(user.OUID), "the role is assigned in the new organization")
}

// The user is created in the user type's unit and shared into the organization the sign-up creates, where it gets
// the named role.
func (ts *OrganizationTemplatesTestSuite) TestSharedAccountSignUp_CreatesTheUserInTheParentAndSharesIt() {
	username := common.GenerateUniqueUsername("orgsharedsignup")

	ts.signUp(ts.sharedAccountSignUpAppID, username, "Organization Templates Initech", "org-templates-initech")

	user := ts.findUser(username)
	ts.Require().NotNil(user, "the sign-up creates the user")
	ts.userIDs = append(ts.userIDs, user.ID)
	ts.Equal(ts.parentOUID, user.OUID, "the user lives in the user type's unit")
	units := ts.policyUnits(user.ID)
	ts.Require().Len(units, 1, "the user is shared into one organization")
	ts.orgIDs = append(ts.orgIDs, units[0])
	ts.requireChildOfParent(units[0], "org-templates-initech")
	ts.Equal([]string{user.ID}, ts.assignees(units[0]), "the role is assigned in the new organization")
	ts.Empty(ts.assignees(ts.parentOUID), "the role is not assigned in the user type's unit")
}

// Enterprise SSO decides the organization from its handle before sign-in, signs in through the connection the
// organization chose and creates the user there.
func (ts *OrganizationTemplatesTestSuite) TestEnterpriseSSO_SignsInThroughTheOrganizationsConnection() {
	sub := common.GenerateUniqueUsername("acmesso")
	ts.mockOIDC.AddUser(&testutils.OIDCUserInfo{Sub: sub, Email: sub + "@acme.test", EmailVerified: true})
	ts.mockOIDC.SetAuthorizeFunc(func(string) (string, error) { return sub, nil })

	step := ts.startSignIn()
	step = ts.next(step, nil, "action_enterprise_sso")
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "the flow asks for the organization: %+v", step.Error)
	step = ts.next(step, map[string]string{"ouHandle": "org-templates-acme"}, "action_organization")
	ts.Require().Equal("REDIRECTION", step.Type, "the flow redirects to acme's provider: %+v", step.Error)

	code, state, err := testutils.SimulateFederatedOAuthFlow(step.Data.RedirectURL)
	ts.Require().NoError(err, "authorize at acme's provider")
	step = ts.next(step, map[string]string{"code": code, "state": state}, "")

	claims := ts.requireSignedInto(step, ts.acmeOUID)
	ts.userIDs = append(ts.userIDs, claims.Sub)
	user, err := testutils.GetUserFromAssertion(step.Assertion)
	ts.Require().NoError(err)
	ts.Equal(ts.acmeOUID, user.OUID, "the first sign-in creates the user in acme")
}

// A handle that names no organization beneath the parent is asked for again, and nothing redirects.
func (ts *OrganizationTemplatesTestSuite) TestEnterpriseSSO_AnUnknownOrganizationIsAskedForAgain() {
	step := ts.startSignIn()
	step = ts.next(step, nil, "action_enterprise_sso")
	step = ts.next(step, map[string]string{"ouHandle": "org-templates-unknown"}, "action_organization")

	ts.Equal("INCOMPLETE", step.FlowStatus, "%+v", step.Error)
	ts.NotEqual("REDIRECTION", step.Type)
	ts.True(common.HasInput(step.Data.Inputs, "ouHandle"), "the organization is asked for again: %+v", step.Data)
}

// A password sign-in decides the organization after sign-in: the only one beneath the parent the user is in.
func (ts *OrganizationTemplatesTestSuite) TestPasswordSignIn_LandsInTheUsersOrganization() {
	username := common.GenerateUniqueUsername("acmepassword")
	attributes, err := json.Marshal(map[string]interface{}{"username": username, "password": orgTemplatesPassword})
	ts.Require().NoError(err)
	userID, err := testutils.CreateUser(testutils.User{
		OUID: ts.acmeOUID, Type: orgTemplatesUserType, Attributes: attributes,
	})
	ts.Require().NoError(err, "create acme's user")
	ts.userIDs = append(ts.userIDs, userID)

	step := ts.startSignIn()
	step = ts.next(step, map[string]string{"username": username, "password": orgTemplatesPassword},
		"action_credentials")

	claims := ts.requireSignedInto(step, ts.acmeOUID)
	ts.Equal(userID, claims.Sub)
}

// An organization handle already taken is asked for again, and the sign-up creates no user before it gets one.
func (ts *OrganizationTemplatesTestSuite) TestOrganizationSignUp_ATakenHandleIsAskedForAgain() {
	username := common.GenerateUniqueUsername("orgtakenhandle")
	step, err := common.InitiateRegistrationFlow(ts.signUpAppID, false, nil, "")
	ts.Require().NoError(err, "start the sign-up")
	step = ts.next(step, map[string]string{"username": username, "password": orgTemplatesPassword},
		"action_credentials")

	step = ts.next(step, map[string]string{
		"ouName": "Organization Templates Acme Again", "ouHandle": "org-templates-acme",
	}, "action_organization")

	ts.Equal("INCOMPLETE", step.FlowStatus, "%+v", step.Error)
	ts.True(common.HasInput(step.Data.Inputs, "ouHandle"), "the handle is asked for again: %+v", step.Data)
	ts.Nil(ts.findUser(username), "no user is created before the organization is")
}

// A role the new organization cannot assign, here one the parent shares with no one, fails the sign-up.
func (ts *OrganizationTemplatesTestSuite) TestOrganizationSignUp_ARoleTheOrganizationCannotAssignFails() {
	closedRoleID := createRole(&ts.Suite, &ts.roleIDs, ts.parentOUID, "", "Organization Templates Closed", nil)
	appID := ts.createSignUpApp("Organization Sign-up Closed Role", "org_templates_closed_signup",
		ts.createFlowAssigning("ORGANIZATION_SIGNUP", "REGISTRATION", "org-templates-closed-signup", closedRoleID))
	username := common.GenerateUniqueUsername("orgclosedrole")

	step, err := common.InitiateRegistrationFlow(appID, false, nil, "")
	ts.Require().NoError(err, "start the sign-up")
	step = ts.next(step, map[string]string{"username": username, "password": orgTemplatesPassword},
		"action_credentials")
	step = ts.next(step, map[string]string{
		"ouName": "Organization Templates Hooli", "ouHandle": "org-templates-hooli",
	}, "action_organization")
	if user := ts.findUser(username); user != nil {
		ts.userIDs = append(ts.userIDs, user.ID)
		ts.orgIDs = append(ts.orgIDs, user.OUID)
	}

	ts.Equal("ERROR", step.FlowStatus, "the sign-up fails on the role")
	ts.Require().NotNil(step.Error)
	ts.Equal("FET-1134", step.Error.Code)
}

// A password sign-in of a user in several organizations beneath the parent asks which one, and lands there.
func (ts *OrganizationTemplatesTestSuite) TestPasswordSignIn_AUserInSeveralOrganizationsPicksOne() {
	betaOUID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "org-templates-beta", Name: "Organization Templates Beta", Parent: &ts.parentOUID,
	})
	ts.Require().NoError(err, "create beta")
	ts.orgIDs = append(ts.orgIDs, betaOUID)
	username := common.GenerateUniqueUsername("orgseveral")
	attributes, err := json.Marshal(map[string]interface{}{"username": username, "password": orgTemplatesPassword})
	ts.Require().NoError(err)
	userID, err := testutils.CreateUser(testutils.User{
		OUID: ts.parentOUID, Type: orgTemplatesUserType, Attributes: attributes,
	})
	ts.Require().NoError(err, "create the parent's user")
	ts.userIDs = append(ts.userIDs, userID)
	status, raw := ts.request(http.MethodPost, "/users/"+userID+"/sharing-policies", map[string]interface{}{
		"childUnits": map[string]interface{}{
			"mode": "selected",
			"units": []map[string]string{
				{"ouId": ts.acmeOUID, "reach": "unit"},
				{"ouId": betaOUID, "reach": "unit"},
			},
		},
	})
	ts.Require().Equal(http.StatusCreated, status, "share the user into acme and beta: %s", raw)

	step := ts.startSignIn()
	step = ts.next(step, map[string]string{"username": username, "password": orgTemplatesPassword},
		"action_credentials")
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "the sign-in asks for the organization: %+v", step.Error)
	idx := slices.IndexFunc(step.Data.Inputs, func(in common.Inputs) bool { return in.Identifier == "ouId" })
	ts.Require().GreaterOrEqual(idx, 0, "%+v", step.Data)
	// Organizations directly beneath one parent are told apart by their handles, which the user picks by.
	ts.Equal([]string{"org-templates-acme", "org-templates-beta"}, step.Data.Inputs[idx].Options)

	step = ts.next(step, map[string]string{"ouId": "org-templates-beta"}, "action_user_organization")

	claims := ts.requireSignedInto(step, betaOUID)
	ts.Equal(userID, claims.Sub)
}

// templateNodes returns the nodes of the shipped template, with the values the template leaves to the deployment
// filled in: the roles to assign and the unit organizations sit beneath.
func (ts *OrganizationTemplatesTestSuite) templateNodes(templateType, flowType,
	roleID string) []map[string]interface{} {
	raw, err := os.ReadFile(organizationTemplates)
	ts.Require().NoError(err, "read the designer templates")
	var templates []struct {
		Type     string `json:"type"`
		FlowType string `json:"flowType"`
		Config   struct {
			Nodes []map[string]interface{} `json:"nodes"`
		} `json:"config"`
	}
	ts.Require().NoError(json.Unmarshal(raw, &templates))
	for _, t := range templates {
		if t.Type != templateType || t.FlowType != flowType {
			continue
		}
		for _, node := range t.Config.Nodes {
			switch node["id"] {
			case "assign_roles":
				node["properties"].(map[string]interface{})["roles"] = roleID
			case "resolve_organization", "resolve_user_organization":
				node["properties"].(map[string]interface{})["parentOuId"] = ts.parentOUID
			}
		}
		return t.Config.Nodes
	}
	ts.FailNow("the designer has no " + templateType + " template for " + flowType)
	return nil
}

func (ts *OrganizationTemplatesTestSuite) createFlow(templateType, flowType, handle string) string {
	return ts.createFlowAssigning(templateType, flowType, handle, ts.roleID)
}

// createFlowAssigning creates a flow from the template whose role assignment names roleID.
func (ts *OrganizationTemplatesTestSuite) createFlowAssigning(templateType, flowType, handle, roleID string) string {
	id, err := testutils.CreateFlow(testutils.Flow{
		Name: "Organization Templates " + handle, FlowType: flowType, Handle: handle,
		Nodes: ts.templateNodes(templateType, flowType, roleID), OUID: ts.parentOUID,
	})
	ts.Require().NoError(err, "create the flow from the %s template", templateType)
	ts.flowIDs = append(ts.flowIDs, id)
	return id
}

func (ts *OrganizationTemplatesTestSuite) createSignUpApp(name, clientID, flowID string) string {
	id, err := testutils.CreateApplication(testutils.Application{
		Name:                      name,
		OUID:                      ts.parentOUID,
		ClientID:                  clientID,
		ClientSecret:              clientID + "_secret",
		AuthFlowID:                ts.signInFlowID,
		RegistrationFlowID:        flowID,
		IsRegistrationFlowEnabled: true,
		AllowedUserTypes:          []string{orgTemplatesUserType},
	})
	ts.Require().NoError(err, "create application %s", name)
	ts.appIDs = append(ts.appIDs, id)
	return id
}

// signUp runs a sign-up template: the account, then the organization, which completes it. An empty handle is left
// out, for the template to make one from the name.
func (ts *OrganizationTemplatesTestSuite) signUp(appID, username, orgName, orgHandle string) {
	ts.T().Helper()
	step, err := common.InitiateRegistrationFlow(appID, false, nil, "")
	ts.Require().NoError(err, "start the sign-up")
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "the sign-up asks for the account: %+v", step.Error)

	step = ts.next(step, map[string]string{"username": username, "password": orgTemplatesPassword},
		"action_credentials")
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "the sign-up asks for the organization: %+v", step.Error)
	ts.Require().True(common.HasInput(step.Data.Inputs, "ouName"), "%+v", step.Data)

	organization := map[string]string{"ouName": orgName}
	if orgHandle != "" {
		organization["ouHandle"] = orgHandle
	}
	step = ts.next(step, organization, "action_organization")
	ts.Require().Equal("COMPLETE", step.FlowStatus, "the sign-up completes: %+v", step.Error)
}

func (ts *OrganizationTemplatesTestSuite) startSignIn() *common.FlowStep {
	ts.T().Helper()
	step, err := common.InitiateAuthenticationFlow(ts.signInAppID, false, nil, "")
	ts.Require().NoError(err, "start the sign-in")
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "%+v", step.Error)
	return step
}

func (ts *OrganizationTemplatesTestSuite) next(step *common.FlowStep, inputs map[string]string,
	action string) *common.FlowStep {
	ts.T().Helper()
	next, err := common.CompleteFlow(step.ExecutionID, inputs, action, step.ChallengeToken)
	ts.Require().NoError(err, "continue the flow with %s", action)
	return next
}

// requireSignedInto checks the sign-in completed with an assertion for ouID and returns its claims.
func (ts *OrganizationTemplatesTestSuite) requireSignedInto(step *common.FlowStep, ouID string) *testutils.JWTClaims {
	ts.T().Helper()
	ts.Require().Equal("COMPLETE", step.FlowStatus, "the sign-in completes: %+v", step.Error)
	claims, err := testutils.DecodeJWT(step.Assertion)
	ts.Require().NoError(err)
	ts.Equal(ouID, claims.OUID, "the assertion names the organization the sign-in landed in")
	return claims
}

// requireChildOfParent checks the organization the sign-up created has the handle it was given and sits directly
// beneath the parent.
func (ts *OrganizationTemplatesTestSuite) requireChildOfParent(ouID, handle string) {
	ts.T().Helper()
	ou, err := testutils.GetOrganizationUnit(ouID)
	ts.Require().NoError(err, "read the new organization")
	ts.Equal(handle, ou.Handle)
	ts.Require().NotNil(ou.Parent)
	ts.Equal(ts.parentOUID, *ou.Parent, "the organization sits directly beneath the user type's unit")
}

// assignees returns the IDs the suite's role is assigned to in the organization unit.
func (ts *OrganizationTemplatesTestSuite) assignees(ouID string) []string {
	path := "/roles/" + ts.roleID + "/assignments?filter=" + url.QueryEscape(`ouId eq "`+ouID+`"`)
	status, raw := ts.request(http.MethodGet, path, nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	var list struct {
		Assignments []struct {
			ID string `json:"id"`
		} `json:"assignments"`
	}
	ts.Require().NoError(json.Unmarshal(raw, &list), string(raw))
	ids := []string{}
	for _, a := range list.Assignments {
		ids = append(ids, a.ID)
	}
	return ids
}

// policyUnits returns the units the user's sharing policies name.
func (ts *OrganizationTemplatesTestSuite) policyUnits(userID string) []string {
	status, raw := ts.request(http.MethodGet, "/users/"+userID+"/sharing-policies", nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	var list struct {
		Policies []struct {
			ChildUnits struct {
				Units []struct {
					OUID string `json:"ouId"`
				} `json:"units"`
			} `json:"childUnits"`
		} `json:"policies"`
	}
	ts.Require().NoError(json.Unmarshal(raw, &list), string(raw))
	var units []string
	for _, p := range list.Policies {
		for _, u := range p.ChildUnits.Units {
			units = append(units, u.OUID)
		}
	}
	return units
}

func (ts *OrganizationTemplatesTestSuite) findUser(username string) *testutils.User {
	path := "/users?filter=" + url.QueryEscape(`username eq "`+username+`"`)
	status, raw := ts.request(http.MethodGet, path, nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	var list struct {
		Users []testutils.User `json:"users"`
	}
	ts.Require().NoError(json.Unmarshal(raw, &list), string(raw))
	for i := range list.Users {
		if list.Users[i].Type == orgTemplatesUserType {
			return &list.Users[i]
		}
	}
	return nil
}

func (ts *OrganizationTemplatesTestSuite) request(method, path string, body interface{}) (int, []byte) {
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
	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	return resp.StatusCode, raw
}
