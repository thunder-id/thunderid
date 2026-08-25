// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// The shipped application administration flows. Executing by handle keeps the tests independent of the
// seeded flow ids, which are a bootstrap detail.
const (
	applicationDeletionFlowHandle = "default-application-deletion-flow"
	secretRegenerationFlowHandle  = "default-client-secret-regeneration-flow"
)

// applicationTargetInput is the input identifier the application executors read their target from. It is
// deliberately not "applicationId": that name is already taken at the top level of the execution request.
const applicationTargetInput = "targetApplicationId"

// clientSecretData is the key the regeneration flow returns the new secret under, its only readable
// moment.
const clientSecretData = "clientSecret" // #nosec G101 -- response field name, not a secret

// The refusal codes the preparatory node carries out of the application validators. They are the
// validators' own codes rather than the executor's generic one, which is what lets the console tell a
// public client apart from a declarative one or from an application that is simply gone.
const (
	errCodeApplicationNotFound             = "APP-1001"
	errCodeApplicationHasNoClientSecret    = "APP-1047"
	errCodeCannotModifyDeclarativeResource = "APP-1030"
)

// declarativeAppID is the confidential application loaded from
// tests/integration/resources/declarative_resources/applications/app-declarative-confidential.yaml before
// the suites run. It is confidential on purpose: a declarative public client would be refused for having
// no rotatable secret, which would hide whether the declarative refusal itself is reached.
const (
	declarativeAppID       = "decl-app-confidential-1"
	declarativeAppClientID = "decl-conf-client-1"
	declarativeAppSecret   = "decl-conf-secret-1" // #nosec G101 -- declarative test fixture value
)

// appAdminFlowTestOU owns the applications these tests create, so they never touch a shared one.
var appAdminFlowTestOU = testutils.OrganizationUnit{
	Handle:      "app_admin_flow_test_ou",
	Name:        "Test OU for Application Administration Flows",
	Description: "Organization unit created for application administration flow testing",
	Parent:      nil,
}

// appAdminFlowTestUserType backs the user that stands in for a target that exists but is not an
// application, so the flows can be pointed at a real id of the wrong kind.
var appAdminFlowTestUserType = testutils.UserType{
	Name: "app_admin_flow_test_user",
	Schema: map[string]interface{}{
		"username": map[string]interface{}{"type": "string"},
		"password": map[string]interface{}{"type": "string", "credential": true},
	},
}

type ApplicationAdministrationFlowTestSuite struct {
	suite.Suite
	ouID           string
	userTypeID     string
	userID         string
	deletionFlowID string
	rotationFlowID string
	createdAppIDs  []string
}

func TestApplicationAdministrationFlowTestSuite(t *testing.T) {
	suite.Run(t, new(ApplicationAdministrationFlowTestSuite))
}

func (ts *ApplicationAdministrationFlowTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(appAdminFlowTestOU)
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

	deletionFlowID, err := testutils.GetFlowIDByHandle(applicationDeletionFlowHandle, administrationFlowType)
	ts.Require().NoError(err, "Failed to resolve the shipped application deletion flow")
	ts.Require().NotEmpty(deletionFlowID, "The shipped application deletion flow must be present")
	ts.deletionFlowID = deletionFlowID

	rotationFlowID, err := testutils.GetFlowIDByHandle(secretRegenerationFlowHandle, administrationFlowType)
	ts.Require().NoError(err, "Failed to resolve the shipped client secret regeneration flow")
	ts.Require().NotEmpty(rotationFlowID, "The shipped client secret regeneration flow must be present")
	ts.rotationFlowID = rotationFlowID

	appAdminFlowTestUserType.OUID = ts.ouID
	userTypeID, err := testutils.CreateUserType(appAdminFlowTestUserType)
	ts.Require().NoError(err, "Failed to create test user type")
	ts.userTypeID = userTypeID

	attributes, err := json.Marshal(map[string]string{
		"username": common.GenerateUniqueUsername("app_admin_flow_user"),
		"password": "Testpass1",
	})
	ts.Require().NoError(err)
	userID, err := testutils.CreateUser(testutils.User{
		Type:       appAdminFlowTestUserType.Name,
		OUID:       ts.ouID,
		Attributes: attributes,
	})
	ts.Require().NoError(err, "Failed to create test user")
	ts.Require().NotEmpty(userID)
	ts.userID = userID
}

// TearDownSuite removes any application a test created but did not delete through a flow.
func (ts *ApplicationAdministrationFlowTestSuite) TearDownSuite() {
	if ts.userID != "" {
		if err := testutils.DeleteUser(ts.userID); err != nil {
			ts.T().Logf("Failed to delete test user during teardown: %v", err)
		}
	}
	if ts.userTypeID != "" {
		if err := testutils.DeleteUserType(ts.userTypeID); err != nil {
			ts.T().Logf("Failed to delete test user type during teardown: %v", err)
		}
	}
	for _, appID := range ts.createdAppIDs {
		if err := testutils.DeleteApplication(appID); err != nil {
			ts.T().Logf("Failed to delete test application %s during teardown: %v", appID, err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}
}

// executeAdminFlow runs a flow by id as an administrator and returns the resulting step.
//
// The bearer token is set on a raw client because the shared test clients treat /flow/execute as a public
// endpoint and skip token injection, which would make every administration request anonymous.
func (ts *ApplicationAdministrationFlowTestSuite) executeAdminFlow(
	flowID string, inputs map[string]string) (int, common.FlowStep, []byte) {
	ts.T().Helper()

	token, err := testutils.GetAccessToken()
	ts.Require().NoError(err, "Failed to obtain admin access token")

	reqBody, err := json.Marshal(map[string]interface{}{"flowId": flowID, "inputs": inputs})
	ts.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost, testServerURL+"/flow/execute", bytes.NewReader(reqBody))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err, "Failed to execute administration flow")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)

	var step common.FlowStep
	_ = json.Unmarshal(body, &step)

	return resp.StatusCode, step, body
}

// createTestApp creates an application and registers it for teardown.
func (ts *ApplicationAdministrationFlowTestSuite) createTestApp(
	name, clientID, clientSecret string, embedded bool) string {
	ts.T().Helper()

	appID, err := testutils.CreateApplication(testutils.Application{
		Name:         name,
		Description:  "Application created for administration flow testing",
		OUID:         ts.ouID,
		Type:         "fullstack",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Embedded:     embedded,
	})
	ts.Require().NoError(err, "Failed to create test application")
	ts.Require().NotEmpty(appID)
	ts.createdAppIDs = append(ts.createdAppIDs, appID)

	return appID
}

// applicationExists reports whether the application record is still retrievable.
func (ts *ApplicationAdministrationFlowTestSuite) applicationExists(appID string) bool {
	ts.T().Helper()

	req, err := http.NewRequest(http.MethodGet, testServerURL+"/applications/"+appID, nil)
	ts.Require().NoError(err)

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err, "Failed to read application")
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	return resp.StatusCode == http.StatusOK
}

// clientCredentialsAccepted reports whether the client id and secret authenticate at the token endpoint.
// It is the observable proof that a rotation took effect: the old secret stops working and the new one
// starts.
func (ts *ApplicationAdministrationFlowTestSuite) clientCredentialsAccepted(
	clientID, clientSecret string) bool {
	ts.T().Helper()

	form := url.Values{}
	form.Set("grant_type", "client_credentials")

	req, err := http.NewRequest(http.MethodPost, testServerURL+"/oauth2/token",
		strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// The test applications default to client_secret_basic, so the credentials go in the header.
	req.SetBasicAuth(clientID, clientSecret)

	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err, "Failed to call the token endpoint")
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	return resp.StatusCode == http.StatusOK
}

// Running the shipped deletion flow exercises the whole chain in one execution: the permission
// validator, the validation that publishes the trusted plan, the criteria write, the session
// detachment, and the record deletion.
func (ts *ApplicationAdministrationFlowTestSuite) TestApplicationDeletionFlow_CompletesAndDeletesApp() {
	appID := ts.createTestApp("admin_flow_delete_app", "admin-flow-delete-client", "admin-flow-delete-secret", false)
	ts.Require().True(ts.applicationExists(appID), "The application should exist before deletion")

	status, step, body := ts.executeAdminFlow(ts.deletionFlowID,
		map[string]string{applicationTargetInput: appID})

	ts.Require().Equal(http.StatusOK, status, "Deletion flow execution failed: %s", string(body))
	ts.Equal("COMPLETE", step.FlowStatus, "Deletion flow should run to completion: %s", string(body))
	ts.False(ts.applicationExists(appID), "The application record should be gone after the deletion flow")
}

// An application with no OAuth component issues no artifacts, so there is nothing to revoke. The flow
// must still delete it rather than refusing on the absent criterion.
func (ts *ApplicationAdministrationFlowTestSuite) TestApplicationDeletionFlow_DeletesAppWithoutOAuthComponent() {
	appID := ts.createTestApp("admin_flow_delete_embedded", "", "", true)
	ts.Require().True(ts.applicationExists(appID), "The application should exist before deletion")

	status, step, body := ts.executeAdminFlow(ts.deletionFlowID,
		map[string]string{applicationTargetInput: appID})

	ts.Require().Equal(http.StatusOK, status, "Deletion flow execution failed: %s", string(body))
	ts.Equal("COMPLETE", step.FlowStatus,
		"An application with no OAuth component should still be deleted: %s", string(body))
	ts.False(ts.applicationExists(appID), "The application record should be gone after the deletion flow")
}

// The rotation flow returns the new secret exactly once, and the rotation is real: the previous secret
// stops authenticating and the returned one starts.
func (ts *ApplicationAdministrationFlowTestSuite) TestSecretRegenerationFlow_RotatesAndReturnsNewSecret() {
	const clientID = "admin-flow-rotate-client"
	const oldSecret = "admin-flow-rotate-secret" // #nosec G101 -- test fixture
	appID := ts.createTestApp("admin_flow_rotate_app", clientID, oldSecret, false)
	ts.Require().True(ts.clientCredentialsAccepted(clientID, oldSecret),
		"The original secret should authenticate before rotation")

	status, step, body := ts.executeAdminFlow(ts.rotationFlowID,
		map[string]string{applicationTargetInput: appID})

	ts.Require().Equal(http.StatusOK, status, "Regeneration flow execution failed: %s", string(body))
	ts.Require().Equal("COMPLETE", step.FlowStatus,
		"Regeneration flow should run to completion: %s", string(body))

	newSecret := step.Data.AdditionalData[clientSecretData]
	ts.Require().NotEmpty(newSecret, "The flow should return the new client secret: %s", string(body))
	ts.NotEqual(oldSecret, newSecret, "The returned secret should be a new value")
	ts.False(ts.clientCredentialsAccepted(clientID, oldSecret),
		"The previous secret must stop authenticating after rotation")
	ts.True(ts.clientCredentialsAccepted(clientID, newSecret),
		"The returned secret must authenticate after rotation")
}

// Validation runs before anything destructive, so an application that does not exist is refused rather
// than producing a completed flow that deleted nothing.
func (ts *ApplicationAdministrationFlowTestSuite) TestApplicationDeletionFlow_UnknownApplicationDoesNotComplete() {
	status, step, body := ts.executeAdminFlow(ts.deletionFlowID,
		map[string]string{applicationTargetInput: "01900000-0000-7000-8000-0000000000fe"})

	if status == http.StatusOK {
		ts.NotEqual("COMPLETE", step.FlowStatus,
			"Deleting an unknown application must not report success: %s", string(body))
		return
	}
	ts.GreaterOrEqual(status, http.StatusBadRequest,
		"Deleting an unknown application should be reported as an error: %s", string(body))
}

// The flows declare their target as a required input, so omitting it must not act on anything.
func (ts *ApplicationAdministrationFlowTestSuite) TestApplicationFlows_MissingTargetDoesNotComplete() {
	for name, flowID := range map[string]string{
		"deletion": ts.deletionFlowID,
		"rotation": ts.rotationFlowID,
	} {
		status, step, body := ts.executeAdminFlow(flowID, map[string]string{})

		if status == http.StatusOK {
			ts.NotEqual("COMPLETE", step.FlowStatus,
				"The %s flow with no target must not report success: %s", name, string(body))
			continue
		}
		ts.GreaterOrEqual(status, http.StatusBadRequest,
			"The %s flow with no target should be reported as an error: %s", name, string(body))
	}
}

// An unauthenticated caller must be refused before any application lookup, since /flow/execute is a
// public path and the administration gate is what protects these flows.
func (ts *ApplicationAdministrationFlowTestSuite) TestApplicationFlows_RejectAnonymousCaller() {
	appID := ts.createTestApp("admin_flow_anon_app", "admin-flow-anon-client", "admin-flow-anon-secret", false)

	reqBody, err := json.Marshal(map[string]interface{}{
		"flowId": ts.deletionFlowID,
		"inputs": map[string]string{applicationTargetInput: appID},
	})
	ts.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost, testServerURL+"/flow/execute", bytes.NewReader(reqBody))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	ts.Equal(http.StatusUnauthorized, resp.StatusCode,
		"An anonymous administration request should be unauthorized: %s", string(body))
	ts.True(ts.applicationExists(appID), "Nothing should have been deleted for an anonymous caller")
}

// createTestAppWithOAuthConfig creates an application with an explicit inbound OAuth configuration and
// registers it for teardown. It exists for the client shapes the plain helper cannot express, such as a
// public client that authenticates without a secret.
func (ts *ApplicationAdministrationFlowTestSuite) createTestAppWithOAuthConfig(
	name string, oauthConfig map[string]interface{}) string {
	ts.T().Helper()

	appID, err := testutils.CreateApplication(testutils.Application{
		Name:        name,
		Description: "Application created for administration flow testing",
		OUID:        ts.ouID,
		Type:        "fullstack",
		InboundAuthConfig: []map[string]interface{}{
			{"type": "oauth2", "config": oauthConfig},
		},
	})
	ts.Require().NoError(err, "Failed to create test application")
	ts.Require().NotEmpty(appID)
	ts.createdAppIDs = append(ts.createdAppIDs, appID)

	return appID
}

// assertFlowRefused asserts that a flow execution did not report success, whether it was refused with an
// error status or completed its request with a non-COMPLETE step.
//
// The expected code is the validator's own, not the executor's generic one: the refusals differ in what
// the operator has to do about them, so a caller that cannot tell them apart is shown the wrong reason.
func (ts *ApplicationAdministrationFlowTestSuite) assertFlowRefused(
	status int, step common.FlowStep, body []byte, expectedCode, msg string) {
	ts.T().Helper()

	if status == http.StatusOK {
		ts.NotEqual("COMPLETE", step.FlowStatus, "%s: %s", msg, string(body))
		ts.Empty(step.Data.AdditionalData[clientSecretData],
			"%s: a refused rotation must not return a secret: %s", msg, string(body))
		if ts.NotNil(step.Error, "%s: a refused step should carry an error: %s", msg, string(body)) {
			ts.Equal(expectedCode, step.Error.Code,
				"%s: the refusal should name its own reason: %s", msg, string(body))
		}
		return
	}
	ts.GreaterOrEqual(status, http.StatusBadRequest, "%s: %s", msg, string(body))
}

// An application with no OAuth component has no client secret to rotate. Refusing in the preparatory node
// is what keeps that from being discovered after the criteria node has already written a deny-list row
// against a client that does not exist.
func (ts *ApplicationAdministrationFlowTestSuite) TestSecretRegenerationFlow_RefusesAppWithoutOAuthComponent() {
	appID := ts.createTestApp("admin_flow_rotate_embedded", "", "", true)

	status, step, body := ts.executeAdminFlow(ts.rotationFlowID,
		map[string]string{applicationTargetInput: appID})

	ts.assertFlowRefused(status, step, body, errCodeApplicationHasNoClientSecret,
		"Rotating an application with no OAuth component must be refused")
	ts.True(ts.applicationExists(appID), "A refused rotation must leave the application untouched")
}

// A public client authenticates without a secret, so rotating one would write a credential that is never
// used while revoking artifacts that are still legitimate. The refusal lands before anything is revoked.
func (ts *ApplicationAdministrationFlowTestSuite) TestSecretRegenerationFlow_RefusesPublicClient() {
	appID := ts.createTestAppWithOAuthConfig("admin_flow_rotate_public", map[string]interface{}{
		"clientId":                "admin-flow-rotate-public-client",
		"redirectUris":            []string{"https://localhost:3000"},
		"grantTypes":              []string{"authorization_code"},
		"responseTypes":           []string{"code"},
		"tokenEndpointAuthMethod": "none",
		"publicClient":            true,
		// A public client cannot keep a secret, so the server requires PKCE on it before it will
		// accept the registration at all.
		"pkceRequired": true,
	})

	status, step, body := ts.executeAdminFlow(ts.rotationFlowID,
		map[string]string{applicationTargetInput: appID})

	ts.assertFlowRefused(status, step, body, errCodeApplicationHasNoClientSecret,
		"Rotating a public client must be refused")
	ts.True(ts.applicationExists(appID), "A refused rotation must leave the application untouched")
}

// Validation runs before the rotation, so an application that does not exist is refused rather than
// producing a completed flow that rotated nothing.
func (ts *ApplicationAdministrationFlowTestSuite) TestSecretRegenerationFlow_UnknownApplicationDoesNotComplete() {
	status, step, body := ts.executeAdminFlow(ts.rotationFlowID,
		map[string]string{applicationTargetInput: "01900000-0000-7000-8000-0000000000fd"})

	ts.assertFlowRefused(status, step, body, errCodeApplicationNotFound,
		"Rotating an unknown application must be refused")
}

// A declarative application is owned by its file, so the deletion flow must refuse it in the preparatory
// node. Refusing there is what keeps the deny-list row from being written against a client the flow then
// cannot delete, leaving the application present but its artifacts revoked.
func (ts *ApplicationAdministrationFlowTestSuite) TestApplicationDeletionFlow_RefusesDeclarativeApplication() {
	ts.Require().True(ts.applicationExists(declarativeAppID),
		"The declarative application fixture must be loaded before this test runs")

	status, step, body := ts.executeAdminFlow(ts.deletionFlowID,
		map[string]string{applicationTargetInput: declarativeAppID})

	ts.assertFlowRefused(status, step, body, errCodeCannotModifyDeclarativeResource,
		"Deleting a declarative application must be refused")
	ts.True(ts.applicationExists(declarativeAppID),
		"A refused deletion must leave the declarative application in place")
}

// Rotating the secret of a declarative application would write a credential the file does not carry, so
// the next reload would silently restore the old one while the artifacts issued under it stay revoked.
// The refusal names the declarative reason rather than the generic one, and the fixture's own secret keeps
// authenticating, which is the observable proof that nothing was rotated.
func (ts *ApplicationAdministrationFlowTestSuite) TestSecretRegenerationFlow_RefusesDeclarativeApplication() {
	ts.Require().True(ts.clientCredentialsAccepted(declarativeAppClientID, declarativeAppSecret),
		"The declarative client's secret should authenticate before the refused rotation")

	status, step, body := ts.executeAdminFlow(ts.rotationFlowID,
		map[string]string{applicationTargetInput: declarativeAppID})

	ts.assertFlowRefused(status, step, body, errCodeCannotModifyDeclarativeResource,
		"Rotating the secret of a declarative application must be refused")
	ts.True(ts.clientCredentialsAccepted(declarativeAppClientID, declarativeAppSecret),
		"A refused rotation must leave the declarative client's secret usable")
}

// An id that resolves to a record of another kind is reported as an application that does not exist,
// rather than acted on. Every entity shares one id space, so a user id reaches the lookup; the category
// check is the only thing that stops an application flow from operating on it.
func (ts *ApplicationAdministrationFlowTestSuite) TestApplicationFlows_RefuseNonApplicationEntity() {
	for name, flowID := range map[string]string{
		"deletion": ts.deletionFlowID,
		"rotation": ts.rotationFlowID,
	} {
		status, step, body := ts.executeAdminFlow(flowID,
			map[string]string{applicationTargetInput: ts.userID})

		ts.assertFlowRefused(status, step, body, errCodeApplicationNotFound,
			"The "+name+" flow must refuse a target that is not an application")
	}
	ts.userStillExists()
}

// userStillExists asserts the user targeted by an application flow was left untouched.
func (ts *ApplicationAdministrationFlowTestSuite) userStillExists() {
	ts.T().Helper()

	req, err := http.NewRequest(http.MethodGet, testServerURL+"/users/"+ts.userID, nil)
	ts.Require().NoError(err)

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err, "Failed to read user")
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	ts.Equal(http.StatusOK, resp.StatusCode,
		"An application flow must not delete the user it was wrongly pointed at")
}

// The deny-list row a deletion writes is sized from the longest-lived artifact the client can issue, and
// user-subject and client_credentials access tokens carry separate validity settings. This application
// configures the client_credentials one longer, so sizing from the user-subject setting alone would let a
// token it already issued outlive its own revocation. The observable outcome is that the flow accepts the
// shape and completes: the client authenticates before the deletion and is gone after it.
func (ts *ApplicationAdministrationFlowTestSuite) TestApplicationDeletionFlow_DeletesAppWithLongerClientValidity() {
	const clientID = "admin-flow-client-validity-client"
	const clientSecret = "admin-flow-client-validity-secret" // #nosec G101 -- test fixture
	appID := ts.createTestAppWithOAuthConfig("admin_flow_delete_client_validity", map[string]interface{}{
		"clientId":                clientID,
		"clientSecret":            clientSecret,
		"redirectUris":            []string{"https://localhost:3000"},
		"grantTypes":              []string{"client_credentials"},
		"tokenEndpointAuthMethod": "client_secret_basic",
		"token": map[string]interface{}{
			"accessToken": map[string]interface{}{
				"userConfig":   map[string]interface{}{"validityPeriod": 300},
				"clientConfig": map[string]interface{}{"validityPeriod": 7200},
			},
		},
	})
	ts.Require().True(ts.clientCredentialsAccepted(clientID, clientSecret),
		"The client should issue client_credentials tokens before deletion")

	status, step, body := ts.executeAdminFlow(ts.deletionFlowID,
		map[string]string{applicationTargetInput: appID})

	ts.Require().Equal(http.StatusOK, status, "Deletion flow execution failed: %s", string(body))
	ts.Equal("COMPLETE", step.FlowStatus,
		"An application whose client_credentials tokens outlive its user tokens should still be deleted: %s",
		string(body))
	ts.False(ts.applicationExists(appID), "The application record should be gone after the deletion flow")
	ts.False(ts.clientCredentialsAccepted(clientID, clientSecret),
		"The deleted client must stop issuing tokens")
}

// A refresh token outlives both access token settings, so a deletion against a refresh-token capable
// client must size its deny-list row from the refresh validity instead. The application pins both access
// validities to a minute and its refresh validity to a day, so the refresh token is unambiguously the
// longest-lived artifact the client can hold when it is deleted.
func (ts *ApplicationAdministrationFlowTestSuite) TestApplicationDeletionFlow_DeletesRefreshTokenCapableApp() {
	appID := ts.createTestAppWithOAuthConfig("admin_flow_delete_refresh", map[string]interface{}{
		"clientId":                "admin-flow-refresh-client",
		"clientSecret":            "admin-flow-refresh-secret",
		"redirectUris":            []string{"https://localhost:3000"},
		"grantTypes":              []string{"authorization_code", "refresh_token"},
		"responseTypes":           []string{"code"},
		"tokenEndpointAuthMethod": "client_secret_basic",
		"token": map[string]interface{}{
			"accessToken": map[string]interface{}{
				"userConfig":   map[string]interface{}{"validityPeriod": 60},
				"clientConfig": map[string]interface{}{"validityPeriod": 60},
			},
			"refreshToken": map[string]interface{}{"validityPeriod": 86400},
		},
	})
	ts.Require().True(ts.applicationExists(appID), "The application should exist before deletion")

	status, step, body := ts.executeAdminFlow(ts.deletionFlowID,
		map[string]string{applicationTargetInput: appID})

	ts.Require().Equal(http.StatusOK, status, "Deletion flow execution failed: %s", string(body))
	ts.Equal("COMPLETE", step.FlowStatus,
		"A refresh-token capable application should be deleted: %s", string(body))
	ts.False(ts.applicationExists(appID), "The application record should be gone after the deletion flow")
}
