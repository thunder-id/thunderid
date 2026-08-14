// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authentication

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	subAttrRedirectURI = "https://localhost:3000"
	subAttrUserType    = "subattr-test-person"
	subAttrUsername    = "subattr_user"
	subAttrPassword    = "SecurePass123!"
	subAttrExternalID  = "ext-98765"

	mappedClientID     = "subattr_mapped_client"     // maps external_id and releases it
	defaultClientID    = "subattr_default_client"    // no mapping (control)
	unreleasedClientID = "subattr_unreleased_client" // maps external_id but does not release it
)

type SubjectAttributeTestSuite struct {
	suite.Suite
	ouID            string
	entityTypeID    string
	userID          string
	flowID          string
	mappedAppID     string
	defaultAppID    string
	unreleasedAppID string
}

func TestSubjectAttributeTestSuite(t *testing.T) {
	// The subject attribute mapping is an internal-only attribute. It is no longer accepted from the
	// management API or from declarative configuration, so this suite has no supported way to
	// provision the mapping on the applications it creates. Re-enable once an input path exists.
	t.Skip("subject attribute mapping is internal-only and has no supported input path to configure")

	suite.Run(t, new(SubjectAttributeTestSuite))
}

func (ts *SubjectAttributeTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "subattr-test-ou",
		Name:        "Subject Attribute Test OU",
		Description: "Organization unit for subject-attribute mapping integration testing",
	})
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

	// external_id is unique + required + string, so it is a valid subject-attribute candidate.
	entityTypeID, err := testutils.CreateUserType(testutils.UserType{
		Name: subAttrUserType,
		OUID: ts.ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{
				"type":     "string",
				"required": true,
				"unique":   true,
			},
			"password": map[string]interface{}{
				"type":       "string",
				"credential": true,
			},
			"external_id": map[string]interface{}{
				"type":     "string",
				"required": true,
				"unique":   true,
			},
			"email": map[string]interface{}{
				"type": "string",
			},
		},
	})
	ts.Require().NoError(err, "Failed to create test user type")
	ts.entityTypeID = entityTypeID

	ts.userID = ts.createTestUser()
	ts.flowID = ts.createTestAuthenticationFlow()

	// App 1: maps external_id as sub AND releases it (assertion.userAttributes).
	appID, err := testutils.CreateApplication(ts.appConfig(mappedClientID,
		map[string]string{subAttrUserType: "external_id"}, []string{"external_id"}))
	ts.Require().NoError(err, "mapped app should be created")
	ts.mappedAppID = appID

	// App 2: no mapping — the control for default behaviour.
	appID, err = testutils.CreateApplication(ts.appConfig(defaultClientID, nil, []string{"external_id"}))
	ts.Require().NoError(err, "default app should be created")
	ts.defaultAppID = appID

	// App 3: maps external_id but does NOT release it. Resolution still reads the fetched value.
	appID, err = testutils.CreateApplication(ts.appConfig(unreleasedClientID,
		map[string]string{subAttrUserType: "external_id"}, nil))
	ts.Require().NoError(err, "unreleased-mapping app should be created")
	ts.unreleasedAppID = appID
}

func (ts *SubjectAttributeTestSuite) TearDownSuite() {
	for _, id := range []string{ts.mappedAppID, ts.defaultAppID, ts.unreleasedAppID} {
		if id != "" {
			if err := testutils.DeleteApplication(id); err != nil {
				ts.T().Logf("Failed to delete application during teardown: %v", err)
			}
		}
	}
	if ts.flowID != "" {
		if err := testutils.DeleteFlow(ts.flowID); err != nil {
			ts.T().Logf("Failed to delete flow during teardown: %v", err)
		}
	}
	if ts.userID != "" {
		if err := testutils.DeleteUser(ts.userID); err != nil {
			ts.T().Logf("Failed to delete user during teardown: %v", err)
		}
	}
	if ts.entityTypeID != "" {
		if err := testutils.DeleteUserType(ts.entityTypeID); err != nil {
			ts.T().Logf("Failed to delete user type during teardown: %v", err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete organization unit during teardown: %v", err)
		}
	}
}

// TestMappedSubject verifies that both the access token and ID token carry the mapped attribute
// value as `sub`, and that a refresh preserves it.
func (ts *SubjectAttributeTestSuite) TestMappedSubject() {
	resp, err := testutils.ObtainAccessTokenWithPassword(
		mappedClientID, subAttrRedirectURI, "openid", subAttrUsername, subAttrPassword, true)
	ts.Require().NoError(err, "Failed to obtain token for mapped app")

	at, err := testutils.DecodeJWT(resp.AccessToken)
	ts.Require().NoError(err)
	ts.Equal(subAttrExternalID, at.Sub, "access token sub should be the mapped external_id")

	it, err := testutils.DecodeJWT(resp.IDToken)
	ts.Require().NoError(err)
	ts.Equal(subAttrExternalID, it.Sub, "id token sub should be the mapped external_id")

	ts.NotEqual(ts.userID, at.Sub, "mapped sub must differ from the entity ID")

	// Refresh preserves the mapped sub.
	ts.Require().NotEmpty(resp.RefreshToken, "expected a refresh token")
	refreshed, err := testutils.RefreshAccessTokenWithClientCredentialsInBody(mappedClientID, "", resp.RefreshToken)
	ts.Require().NoError(err, "Failed to refresh token")
	rat, err := testutils.DecodeJWT(refreshed.AccessToken)
	ts.Require().NoError(err)
	ts.Equal(subAttrExternalID, rat.Sub, "refreshed access token sub should preserve the mapped value")
}

// TestDefaultSubjectWhenNoMapping verifies that, without a mapping, sub is the entity ID.
func (ts *SubjectAttributeTestSuite) TestDefaultSubjectWhenNoMapping() {
	resp, err := testutils.ObtainAccessTokenWithPassword(
		defaultClientID, subAttrRedirectURI, "openid", subAttrUsername, subAttrPassword, true)
	ts.Require().NoError(err, "Failed to obtain token for default app")

	at, err := testutils.DecodeJWT(resp.AccessToken)
	ts.Require().NoError(err)
	ts.Equal(ts.userID, at.Sub, "sub should default to the entity ID when no mapping is configured")
}

// TestMappedSubjectWhenAttributeNotReleased verifies resolve-then-drop: the mapped attribute is used
// as `sub` even though it is not released to the client, and it does not appear as a token claim.
func (ts *SubjectAttributeTestSuite) TestMappedSubjectWhenAttributeNotReleased() {
	resp, err := testutils.ObtainAccessTokenWithPassword(
		unreleasedClientID, subAttrRedirectURI, "openid", subAttrUsername, subAttrPassword, true)
	ts.Require().NoError(err, "Failed to obtain token for unreleased-mapping app")

	at, err := testutils.DecodeJWT(resp.AccessToken)
	ts.Require().NoError(err)
	ts.Equal(subAttrExternalID, at.Sub, "sub should be the mapped external_id even when it is not released")

	// The mapped attribute must not leak into the released claims of either token.
	atClaims, err := testutils.DecodeJWTPayloadMap(resp.AccessToken)
	ts.Require().NoError(err)
	ts.NotContains(atClaims, "external_id", "unreleased mapped attribute must not appear in access token claims")

	itClaims, err := testutils.DecodeJWTPayloadMap(resp.IDToken)
	ts.Require().NoError(err)
	ts.NotContains(itClaims, "external_id", "unreleased mapped attribute must not appear in id token claims")
}

// TestValidationRejectsInvalidMapping verifies that app creation rejects a subject-attribute mapping
// pointing at an attribute that is not unique+required+string (email here is neither unique nor required).
func (ts *SubjectAttributeTestSuite) TestValidationRejectsInvalidMapping() {
	_, err := testutils.CreateApplication(ts.appConfig("subattr_invalid_client",
		map[string]string{subAttrUserType: "email"}, []string{"email"}))
	ts.Error(err, "app creation should reject a non-unique/non-required subject attribute")
}

// --- helpers ---

func (ts *SubjectAttributeTestSuite) createTestUser() string {
	attributes, err := json.Marshal(map[string]interface{}{
		"username":    subAttrUsername,
		"password":    subAttrPassword,
		"external_id": subAttrExternalID,
		"email":       "subattr@example.com",
	})
	ts.Require().NoError(err)

	userID, err := testutils.CreateUser(testutils.User{
		Type:       subAttrUserType,
		OUID:       ts.ouID,
		Attributes: json.RawMessage(attributes),
	})
	ts.Require().NoError(err, "Failed to create test user")
	return userID
}

func (ts *SubjectAttributeTestSuite) createTestAuthenticationFlow() string {
	flow := testutils.Flow{
		Name:     "Subject Attribute Test Auth Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "subattr_test_auth_flow",
		Nodes: []map[string]interface{}{
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
				"onSuccess": "auth_assert",
			},
			{"id": "auth_assert", "type": "TASK_EXECUTION",
				"executor": map[string]interface{}{"name": "AuthAssertExecutor"}, "onSuccess": "end"},
			{"id": "end", "type": "END"},
		},
	}
	flowID, err := testutils.CreateFlow(flow)
	ts.Require().NoError(err, "Failed to create test authentication flow")
	return flowID
}

// appConfig builds the application for testutils.CreateApplication. subjectAttribute (when non-nil)
// is set as a first-class application config. releasedAttrs (when non-empty) are released via the
// assertion and ID-token allow-list; leaving it empty keeps the attribute out of the released claims.
func (ts *SubjectAttributeTestSuite) appConfig(
	clientID string, subjectAttribute map[string]string, releasedAttrs []string,
) testutils.Application {
	tokenConfig := map[string]interface{}{}
	if len(releasedAttrs) > 0 {
		tokenConfig["idToken"] = map[string]interface{}{"userAttributes": releasedAttrs}
	}

	app := testutils.Application{
		Name:             "SubAttr-" + clientID,
		Description:      "Subject-attribute mapping integration test app",
		OUID:             ts.ouID,
		Type:             "fullstack",
		AuthFlowID:       ts.flowID,
		AllowedUserTypes: []string{subAttrUserType},
		SubjectAttribute: subjectAttribute,
		InboundAuthConfig: []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                clientID,
					"redirectUris":            []string{subAttrRedirectURI},
					"grantTypes":              []string{"authorization_code", "refresh_token"},
					"responseTypes":           []string{"code"},
					"tokenEndpointAuthMethod": "none",
					"publicClient":            true,
					"pkceRequired":            true,
					"scopes":                  []string{"openid", "email"},
					"token":                   tokenConfig,
				},
			},
		},
	}
	if len(releasedAttrs) > 0 {
		app.AssertionConfig = map[string]interface{}{"userAttributes": releasedAttrs}
	}
	return app
}
