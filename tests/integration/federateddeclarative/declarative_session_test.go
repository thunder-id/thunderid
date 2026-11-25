// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package federateddeclarative signs federated identities in while groups and roles are kept only in
// declarative files. It restarts the server to switch the stores, so it runs apart from the federated
// suite.
package federateddeclarative

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	oidcClientID     = "federated-declarative-idp-client"
	oidcClientSecret = "federated-declarative-idp-secret"
	appClientID      = "federated-declarative-token-client"
	appClientSecret  = "federated-declarative-token-client-secret"
	redirectURI      = "http://localhost:3000/callback"

	// The declarative fixtures: decl-role-1 grants declarativeScope on declarativeResource, and is
	// assigned to decl-group-1. Both belong to decl-ou-1.
	declarativeOUID     = "decl-ou-1"
	declarativeRoleID   = "decl-role-1"
	declarativeGroupID  = "decl-group-1"
	declarativeResource = "https://localhost:8090/decl-rs-1"
	declarativeScope    = "test-resource:read"
)

var declarativeUserType = testutils.UserType{
	Handle:                "fed_decl_person",
	DisplayName:           "Fed Declarative Person",
	OUID:                  declarativeOUID,
	AllowSelfRegistration: true,
	Schema: map[string]interface{}{
		"username": map[string]interface{}{"type": "string", "required": true, "unique": true},
		"email":    map[string]interface{}{"type": "string", "required": true, "unique": true},
		"sub":      map[string]interface{}{"type": "string"},
	},
}

type DeclarativeSessionTestSuite struct {
	suite.Suite
	mockOIDC   *testutils.MockOIDCServer
	activeSub  string
	typeID     string
	idpID      string
	flowIDs    []string
	appID      string
	userIDs    []string
	subCounter int
}

func TestDeclarativeSessionTestSuite(t *testing.T) {
	suite.Run(t, new(DeclarativeSessionTestSuite))
}

// SetupSuite creates, in the declarative fixtures' organization unit, a user type, a connection that
// maps groups claims onto the declarative role and group, and an application whose flow provisions and
// checks authorization after the federated login. A connection maps only the roles and groups its unit
// owns or has shared with it.
func (s *DeclarativeSessionTestSuite) SetupSuite() {
	mock, err := testutils.NewMockOIDCServer(oidcClientID, oidcClientSecret)
	s.Require().NoError(err, "failed to create the mock OIDC server")
	s.mockOIDC = mock
	s.mockOIDC.SetAuthorizeFunc(func(string) (string, error) {
		if s.activeSub == "" {
			return "", fmt.Errorf("no active subject selected by the test")
		}
		return s.activeSub, nil
	})

	s.typeID, err = testutils.CreateUserType(declarativeUserType)
	s.Require().NoError(err, "failed to create the user type")

	s.idpID, err = testutils.CreateIDP(testutils.IDP{
		Name: "Federated Declarative Connection",
		OUID: declarativeOUID,
		Type: "OIDC",
		Properties: []testutils.IDPProperty{
			{Name: "client_id", Value: oidcClientID},
			{Name: "client_secret", Value: oidcClientSecret, IsSecret: true},
			{Name: "redirect_uri", Value: redirectURI},
			{Name: "authorization_endpoint", Value: s.mockOIDC.GetAuthorizeURL()},
			{Name: "token_endpoint", Value: s.mockOIDC.GetTokenURL()},
			{Name: "userinfo_endpoint", Value: s.mockOIDC.GetUserInfoURL()},
			{Name: "jwks_endpoint", Value: s.mockOIDC.GetJWKSURL()},
			{Name: "scopes", Value: "openid,email,profile"},
		},
		AttributeConfiguration: &testutils.AttributeConfiguration{
			UserTypeResolution: &testutils.UserTypeResolution{Default: declarativeUserType.Handle},
			UserTypeAttributeMappings: []testutils.UserTypeAttributeMapping{{
				UserType: declarativeUserType.Handle,
				Attributes: []testutils.AttributeMapping{
					{ExternalAttribute: "email", LocalAttribute: "email"},
					{ExternalAttribute: "email", LocalAttribute: "username"},
				},
			}},
			AuthorizationMapping: &testutils.AuthorizationMapping{
				Rules: []testutils.AuthorizationRuleMapping{{
					Claim: "groups",
					Values: []testutils.AuthorizationRule{
						{Operator: testutils.AuthorizationOperatorEquals, Value: "decl-readers",
							Targets: []testutils.AuthorizationTarget{
								{Type: testutils.AuthorizationTargetRole, ID: declarativeRoleID}}},
						{Operator: testutils.AuthorizationOperatorEquals, Value: "decl-members",
							Targets: []testutils.AuthorizationTarget{
								{Type: testutils.AuthorizationTargetGroup, ID: declarativeGroupID}}},
					},
				}},
			},
		},
	})
	s.Require().NoError(err, "failed to create the connection")

	flowID, err := testutils.CreateFlow(testutils.Flow{
		Name:     "Federated Declarative Token Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_federated_declarative_tokens",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "oidc_auth"},
			{
				"id":   "oidc_auth",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"idpId":                               s.idpID,
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
	})
	s.Require().NoError(err, "failed to create the authentication flow")
	s.flowIDs = append(s.flowIDs, flowID)

	regFlowID, err := testutils.CreateIsolatedRegistrationFlow(appClientID + "-reg")
	s.Require().NoError(err, "failed to create the isolated registration flow")
	s.flowIDs = append(s.flowIDs, regFlowID)

	s.appID, err = testutils.CreateApplication(testutils.Application{
		Name:               "Federated Declarative Token App",
		OUID:               declarativeOUID,
		AuthFlowID:         flowID,
		RegistrationFlowID: regFlowID,
		AllowedUserTypes:   []string{declarativeUserType.Handle},
		InboundAuthConfig: []map[string]interface{}{{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                appClientID,
				"clientSecret":            appClientSecret,
				"redirectUris":            []string{redirectURI},
				"grantTypes":              []string{"authorization_code", "refresh_token"},
				"responseTypes":           []string{"code"},
				"tokenEndpointAuthMethod": "client_secret_basic",
			},
		}},
	})
	s.Require().NoError(err, "failed to create the application")
}

func (s *DeclarativeSessionTestSuite) TearDownSuite() {
	if err := testutils.CleanupUsers(s.userIDs); err != nil {
		s.T().Logf("failed to clean up users: %v", err)
	}
	if s.appID != "" {
		if err := testutils.DeleteApplication(s.appID); err != nil {
			s.T().Logf("failed to delete the application: %v", err)
		}
	}
	for _, flowID := range s.flowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			s.T().Logf("failed to delete flow %s: %v", flowID, err)
		}
	}
	if s.idpID != "" {
		if err := testutils.DeleteIDP(s.idpID); err != nil {
			s.T().Logf("failed to delete the connection: %v", err)
		}
	}
	if s.typeID != "" {
		if err := testutils.DeleteUserType(s.typeID); err != nil {
			s.T().Logf("failed to delete the user type: %v", err)
		}
	}
	if s.mockOIDC != nil {
		if err := s.mockOIDC.Stop(); err != nil {
			s.T().Logf("failed to stop the mock OIDC server: %v", err)
		}
	}
}

// With roles and groups kept only in declarative files, the account can't hold a mapped role or group, so
// either applies to the session whatever the sync mode: it grants its scope at sign-in and after a
// refresh, and nothing is stored.
func (s *DeclarativeSessionTestSuite) TestDeclarativeGroupsAndRolesApplyToTheSession() {
	restore := s.restartWithDeclarativeGroupsAndRoles()
	defer restore()
	var signedIn []string
	for _, group := range []string{"decl-readers", "decl-members"} {
		tokens, userID := s.signIn(group)
		signedIn = append(signedIn, userID)
		s.Contains(s.accessTokenScopes(tokens.AccessToken), declarativeScope,
			"the declarative mapping should grant its scope at sign-in")

		refreshed, err := testutils.RefreshAccessToken(appClientID, appClientSecret, tokens.RefreshToken)
		s.Require().NoError(err, "the refresh token grant should succeed")
		s.Contains(s.accessTokenScopes(refreshed.AccessToken), declarativeScope,
			"the declarative mapping should grant its scope after a refresh")
	}

	// Back on the composite stores, an assignment the sign-ins had written would be readable.
	restore()
	assignments, err := testutils.GetRoleAssignments(declarativeRoleID)
	s.Require().NoError(err, "failed to read the role assignments")
	members, err := testutils.GetGroupMembers(declarativeGroupID)
	s.Require().NoError(err, "failed to read the group members")
	for _, userID := range signedIn {
		s.False(slices.ContainsFunc(assignments, func(a testutils.Assignment) bool { return a.ID == userID }),
			"the declarative role should not be assigned")
		s.False(slices.ContainsFunc(members, func(m testutils.GroupMember) bool { return m.ID == userID }),
			"the declarative group should not be assigned")
	}
}

// restartWithDeclarativeGroupsAndRoles restarts the server with the group and role stores declarative, and
// returns a function that puts them back to composite. Once a call succeeds, later calls do nothing.
// The admin token from before the restart is kept, since a declarative role store holds no admin role.
func (s *DeclarativeSessionTestSuite) restartWithDeclarativeGroupsAndRoles() func() {
	s.T().Helper()
	restored := false
	restore := func() {
		if restored {
			return
		}
		restored = s.Assert().NoError(testutils.PatchDeploymentConfig(map[string]interface{}{
			"group": map[string]interface{}{"store": "composite"},
			"role":  map[string]interface{}{"store": "composite"},
		})) && s.Assert().NoError(testutils.RestartServer()) && s.Assert().NoError(testutils.ObtainAdminAccessToken())
	}
	// Registered before the patch, so a failed restart still puts the stores back.
	s.T().Cleanup(restore)
	s.Require().NoError(testutils.PatchDeploymentConfig(map[string]interface{}{
		"group": map[string]interface{}{"store": "declarative"},
		"role":  map[string]interface{}{"store": "declarative"},
	}))
	if err := testutils.RestartServer(); err != nil {
		restore()
		s.Require().NoError(err)
	}
	return restore
}

// signIn signs a new identity whose groups claim holds group in through the application, and returns
// tokens for the declarative resource and the id of the user the sign-in provisioned.
func (s *DeclarativeSessionTestSuite) signIn(group string) (*testutils.TokenResponse, string) {
	s.T().Helper()
	s.subCounter++
	sub := fmt.Sprintf("fed-decl-sub-%d-%d", s.subCounter, testutils.GetServerPID())
	s.mockOIDC.AddUser(&testutils.OIDCUserInfo{
		Sub: sub, Email: sub + "@example.com", EmailVerified: true,
		Custom: map[string]interface{}{"groups": []interface{}{group}},
	})
	s.activeSub = sub

	resp, err := testutils.InitiateAuthorizationFlowWithResource(
		appClientID, redirectURI, "code", "openid "+declarativeScope, "test-state", declarativeResource)
	s.Require().NoError(err, "failed to initiate the authorization request")
	defer resp.Body.Close()
	s.Require().Equal(http.StatusFound, resp.StatusCode, "expected a redirect from the authorization endpoint")
	authID, executionID, err := testutils.ExtractAuthData(resp.Header.Get("Location"))
	s.Require().NoError(err, "failed to read the authorization redirect %s", resp.Header.Get("Location"))

	step, err := testutils.ExecuteAuthenticationFlow(executionID, nil, "")
	s.Require().NoError(err, "failed to start the authentication flow")
	s.Require().Equal("REDIRECTION", step.Type, "expected a redirection to the identity provider, got %+v", step)
	code, state, err := testutils.SimulateFederatedOAuthFlow(step.Data.RedirectURL)
	s.Require().NoError(err, "failed to simulate authorization at the identity provider")
	step, err = testutils.ExecuteAuthenticationFlow(
		executionID, map[string]string{"code": code, "state": state}, "", step.ChallengeToken)
	s.Require().NoError(err, "failed to complete the authentication flow")
	s.Require().Equal("COMPLETE", step.FlowStatus, "expected the flow to complete, got %+v", step)

	claims, err := testutils.DecodeJWT(step.Assertion)
	s.Require().NoError(err, "failed to decode the assertion")
	s.userIDs = append(s.userIDs, claims.Sub)

	authz, err := testutils.CompleteAuthorization(authID, step.Assertion)
	s.Require().NoError(err, "failed to complete the authorization")
	authCode, err := testutils.ExtractAuthorizationCode(authz.RedirectURI)
	s.Require().NoError(err, "failed to read the authorization code")
	result, err := testutils.RequestTokenWithResource(appClientID, appClientSecret, authCode,
		redirectURI, "authorization_code", declarativeResource)
	s.Require().NoError(err, "failed to request tokens")
	s.Require().Equal(http.StatusOK, result.StatusCode, "the token request should succeed: %s", string(result.Body))
	return result.Token, claims.Sub
}

func (s *DeclarativeSessionTestSuite) accessTokenScopes(accessToken string) []string {
	s.T().Helper()
	claims, err := testutils.DecodeJWT(accessToken)
	s.Require().NoError(err, "failed to decode the access token")
	scope, _ := claims.Additional["scope"].(string)
	return strings.Fields(scope)
}
