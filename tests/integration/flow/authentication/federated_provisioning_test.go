// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authentication

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	// fedProvMockGooglePort must be 8093: testutils.GoogleMockBaseURL hardcodes
	// http://localhost:8093 as the Google endpoint the server resolves, so the port is load-bearing
	// rather than arbitrary. Suites in this package run serially and each stops its own mock, so
	// sharing the port with the conditional-exec and google-auth suites is safe.
	fedProvMockGooglePort = 8093
	fedProvGoogleClientID = "fedprov_google_client"
	fedProvGoogleSecret   = "fedprov_google_secret"
)

// fedProvSchema carries the attributes the Google executor maps, none of them required, so
// provisioning never stops to prompt.
var fedProvSchema = map[string]interface{}{
	"username":   map[string]interface{}{"type": "string"},
	"password":   map[string]interface{}{"type": "string", "credential": true},
	"sub":        map[string]interface{}{"type": "string"},
	"email":      map[string]interface{}{"type": "string"},
	"givenName":  map[string]interface{}{"type": "string"},
	"familyName": map[string]interface{}{"type": "string"},
}

type FederatedProvisioningTestSuite struct {
	suite.Suite
	mockGoogle *testutils.MockGoogleOIDCServer

	parentOUID string
	ouAID      string
	ouBID      string
	ouAName    string
	ouAHandle  string

	primaryTypeID   string
	primaryTypeName string
	altTypeID       string
	altTypeName     string
	noSelfRegTypeID string
	noSelfRegName   string

	idpID          string
	noTypesAppID   string
	noSelfRegAppID string
	ambiguousAppID string
	singleAppID    string
	promptAllAppID string

	createdFlowIDs []string
	createdUserIDs []string
}

func TestFederatedProvisioningTestSuite(t *testing.T) {
	suite.Run(t, new(FederatedProvisioningTestSuite))
}

// buildFederatedProvisioningFlow returns a fresh AUTHENTICATION graph modelled on the proven
// conditionalExecFlow: federated login, then provisioning gated on userEligibleForProvisioning, then
// AuthAssert. A builder rather than a package var, because conditionalExecFlow is mutated in place by
// its own suite and sharing it would race. When ouResolveFrom is non-empty an OUResolverExecutor is
// inserted ahead of provisioning.
func buildFederatedProvisioningFlow(handle, idpID, ouResolveFrom string) testutils.Flow {
	provisionNext := "auth_assert"
	nodes := []map[string]interface{}{
		{"id": "start", "type": "START", "onSuccess": "google_auth"},
		{
			"id":   "google_auth",
			"type": "TASK_EXECUTION",
			"properties": map[string]interface{}{
				"idpId":                               idpID,
				"allowAuthenticationWithoutLocalUser": true,
			},
			"executor":  map[string]interface{}{"name": "GoogleOIDCAuthExecutor"},
			"onSuccess": "provision_user",
		},
	}

	if ouResolveFrom != "" {
		// promptAll shows the whole OU tree and, unlike prompt, does not need the defaultOUID that
		// UserTypeResolver would have set (ou_resolver_executor.go:210-211). This flow has no
		// UserTypeResolver, so promptAll is the only workable strategy.
		nodes[1]["onSuccess"] = "resolve_ou"
		nodes = append(nodes, map[string]interface{}{
			"id":         "resolve_ou",
			"type":       "TASK_EXECUTION",
			"properties": map[string]interface{}{"resolveFrom": ouResolveFrom},
			"condition": map[string]interface{}{
				"key":    "{{ctx(userEligibleForProvisioning)}}",
				"value":  "true",
				"onSkip": "auth_assert",
			},
			"executor":     map[string]interface{}{"name": "OUResolverExecutor"},
			"onSuccess":    "provision_user",
			"onIncomplete": "prompt_ou",
		})
		nodes = append(nodes, map[string]interface{}{
			"id":   "prompt_ou",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{
				{
					"inputs": []map[string]interface{}{
						{"ref": "input_ou", "identifier": "ouId", "type": "TEXT_INPUT", "required": true},
					},
					"action": map[string]interface{}{"ref": "action_ou", "nextNode": "resolve_ou"},
				},
			},
		})
	}

	nodes = append(nodes,
		map[string]interface{}{
			"id":   "provision_user",
			"type": "TASK_EXECUTION",
			"condition": map[string]interface{}{
				"key":    "{{ctx(userEligibleForProvisioning)}}",
				"value":  "true",
				"onSkip": "auth_assert",
			},
			"executor":  map[string]interface{}{"name": "ProvisioningExecutor"},
			"onSuccess": provisionNext,
		},
		map[string]interface{}{
			"id":        "auth_assert",
			"type":      "TASK_EXECUTION",
			"executor":  map[string]interface{}{"name": "AuthAssertExecutor"},
			"onSuccess": "end",
		},
		map[string]interface{}{"id": "end", "type": "END"},
	)

	return testutils.Flow{
		Name:     "Federated Provisioning Flow " + handle,
		FlowType: "AUTHENTICATION",
		Handle:   handle,
		Nodes:    nodes,
	}
}

func (ts *FederatedProvisioningTestSuite) SetupSuite() {
	mock, err := testutils.NewMockGoogleOIDCServer(
		fedProvMockGooglePort, fedProvGoogleClientID, fedProvGoogleSecret)
	ts.Require().NoError(err, "Failed to create the mock Google server")
	ts.mockGoogle = mock
	ts.Require().NoError(ts.mockGoogle.Start(), "Failed to start the mock Google server")

	parentOUID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "fedprov-test-ou",
		Name:        "Federated Provisioning Test OU",
		Description: "Parent OU for federated auto-provisioning entity ref tests",
	})
	ts.Require().NoError(err, "Failed to create parent OU")
	ts.parentOUID = parentOUID

	ts.ouAHandle = "fedprov-ou-a"
	ts.ouAName = "Federated Provisioning OU A"
	ts.ouAID, err = testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: ts.ouAHandle, Name: ts.ouAName, Parent: &ts.parentOUID,
	})
	ts.Require().NoError(err, "Failed to create OU A")

	ts.ouBID, err = testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "fedprov-ou-b", Name: "Federated Provisioning OU B", Parent: &ts.parentOUID,
	})
	ts.Require().NoError(err, "Failed to create OU B")

	// OU-A owns the only self-registration-enabled type used by scenario 10, so the resolved OU is
	// distinguishable from the one scenario 11 selects.
	ts.primaryTypeName = "fedprov-selfreg-primary"
	ts.primaryTypeID, err = testutils.CreateUserType(testutils.UserType{
		Name: ts.primaryTypeName, OUID: ts.ouAID, AllowSelfRegistration: true, Schema: fedProvSchema,
	})
	ts.Require().NoError(err, "Failed to create the primary self-registration type")

	ts.altTypeName = "fedprov-selfreg-secondary"
	ts.altTypeID, err = testutils.CreateUserType(testutils.UserType{
		Name: ts.altTypeName, OUID: ts.ouBID, AllowSelfRegistration: true, Schema: fedProvSchema,
	})
	ts.Require().NoError(err, "Failed to create the secondary self-registration type")

	// testutils.CreateUserType rewrites AllowSelfRegistration false to true
	// (testutils/api_utils.go:68-70), so a type created through it is always self-registration-enabled.
	// This type must be created with an explicit false, so it is posted as raw JSON.
	ts.noSelfRegName = "fedprov-noselfreg"
	ts.noSelfRegTypeID = ts.createUserTypeWithSelfRegDisabled(ts.noSelfRegName, ts.ouBID)

	ts.idpID, err = testutils.CreateIDP(testutils.IDP{
		Name:        "Federated Provisioning Google IDP",
		Description: "Google IDP for federated auto-provisioning tests",
		Type:        "GOOGLE",
		Properties: []testutils.IDPProperty{
			{Name: "client_id", Value: fedProvGoogleClientID},
			{Name: "client_secret", Value: fedProvGoogleSecret, IsSecret: true},
			{Name: "scopes", Value: "openid email profile"},
			{Name: "redirect_uri", Value: "http://localhost:3000/callback"},
		},
	})
	ts.Require().NoError(err, "Failed to create the Google IDP")

	ts.noTypesAppID = ts.appForFlow("fedprov_flow_no_types", "", "FedProv No Types App",
		"fedprov_notypes_client", nil)
	ts.noSelfRegAppID = ts.appForFlow("fedprov_flow_noselfreg", "", "FedProv No Self Reg App",
		"fedprov_noselfreg_client", []string{ts.noSelfRegName})
	ts.ambiguousAppID = ts.appForFlow("fedprov_flow_ambiguous", "", "FedProv Ambiguous App",
		"fedprov_ambiguous_client", []string{ts.primaryTypeName, ts.altTypeName})
	ts.singleAppID = ts.appForFlow("fedprov_flow_single", "", "FedProv Single App",
		"fedprov_single_client", []string{ts.primaryTypeName, ts.noSelfRegName})
	ts.promptAllAppID = ts.appForFlow("fedprov_flow_promptall", "promptAll", "FedProv PromptAll App",
		"fedprov_promptall_client", []string{ts.primaryTypeName, ts.noSelfRegName})
}

// createUserTypeWithSelfRegDisabled posts a user type with allowSelfRegistration explicitly false.
// testutils.CreateUserType cannot express this: it actively rewrites a false to true
// (testutils/api_utils.go:68-70), so a type created through it is always self-registration-enabled.
func (ts *FederatedProvisioningTestSuite) createUserTypeWithSelfRegDisabled(
	name, ouID string) string {
	payload := map[string]interface{}{
		"name":                  name,
		"ouId":                  ouID,
		"allowSelfRegistration": false,
		"schema":                fedProvSchema,
	}
	body, err := json.Marshal(payload)
	ts.Require().NoError(err, "Failed to encode the user type payload")

	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+"/user-types",
		bytes.NewReader(body))
	ts.Require().NoError(err, "Failed to build the user type request")
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err, "POST /user-types failed")
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err, "Failed to read the user type response")
	ts.Require().Equal(http.StatusCreated, resp.StatusCode,
		"Expected 201 creating the non-self-registration type, got %s", string(respBody))

	// A *bool, not a bool: a missing field would unmarshal to false and silently satisfy the check.
	var created struct {
		ID                    string `json:"id"`
		AllowSelfRegistration *bool  `json:"allowSelfRegistration"`
	}
	ts.Require().NoError(json.Unmarshal(respBody, &created), "Failed to decode the user type")
	ts.Require().NotNil(created.AllowSelfRegistration,
		"The response must echo allowSelfRegistration; without it this fixture cannot be trusted")
	ts.Require().False(*created.AllowSelfRegistration,
		"The type must be created with self-registration disabled, or scenarios 8 and 10 prove nothing")
	return created.ID
}

// appForFlow creates a federated flow with the given OU resolution strategy plus an app bound to it.
func (ts *FederatedProvisioningTestSuite) appForFlow(
	flowHandle, ouResolveFrom, appName, clientID string, allowedTypes []string) string {
	flowID, err := testutils.CreateFlow(
		buildFederatedProvisioningFlow(flowHandle, ts.idpID, ouResolveFrom))
	ts.Require().NoError(err, "Failed to create flow %s", flowHandle)
	ts.createdFlowIDs = append(ts.createdFlowIDs, flowID)

	appID, err := testutils.CreateApplication(testutils.Application{
		OUID:             ts.parentOUID,
		Name:             appName,
		ClientID:         clientID,
		ClientSecret:     clientID + "_secret",
		RedirectURIs:     []string{"http://localhost:3000/callback"},
		AllowedUserTypes: allowedTypes,
		AuthFlowID:       flowID,
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{"userType", "ouId", "ouName", "ouHandle"},
		},
	})
	ts.Require().NoError(err, "Failed to create app %s", appName)
	return appID
}

func (ts *FederatedProvisioningTestSuite) TearDownSuite() {
	if err := testutils.CleanupUsers(ts.createdUserIDs); err != nil {
		ts.T().Logf("Failed to clean up users: %v", err)
	}
	for _, appID := range []string{ts.noTypesAppID, ts.noSelfRegAppID, ts.ambiguousAppID,
		ts.singleAppID, ts.promptAllAppID} {
		if appID == "" {
			continue
		}
		if err := testutils.DeleteApplication(appID); err != nil {
			ts.T().Logf("Failed to delete app %s: %v", appID, err)
		}
	}
	for _, flowID := range ts.createdFlowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete flow %s: %v", flowID, err)
		}
	}
	if ts.idpID != "" {
		if err := testutils.DeleteIDP(ts.idpID); err != nil {
			ts.T().Logf("Failed to delete IDP: %v", err)
		}
	}
	for _, typeID := range []string{ts.primaryTypeID, ts.altTypeID, ts.noSelfRegTypeID} {
		if err := testutils.DeleteUserType(typeID); err != nil {
			ts.T().Logf("Failed to delete user type %s: %v", typeID, err)
		}
	}
	for _, ouID := range []string{ts.ouAID, ts.ouBID, ts.parentOUID} {
		if err := testutils.DeleteOrganizationUnit(ouID); err != nil {
			ts.T().Logf("Failed to delete OU %s: %v", ouID, err)
		}
	}
	if ts.mockGoogle != nil {
		if err := ts.mockGoogle.Stop(); err != nil {
			ts.T().Logf("Failed to stop the mock Google server: %v", err)
		}
	}
}

// federatedLogin registers a Google identity, then drives the redirect dance to the callback.
func (ts *FederatedProvisioningTestSuite) federatedLogin(appID, sub, email string) *common.FlowStep {
	ts.mockGoogle.AddUser(&testutils.GoogleUserInfo{
		Sub: sub, Email: email, EmailVerified: true,
		Name: "Fed Prov User", GivenName: "Fed", FamilyName: "Prov",
	})

	// handleAuthorize calls authorizeFunc("") with a hardcoded empty string
	// (mock_google_oidc_server.go:288), so the request cannot name the identity. Register a closure
	// capturing this login's email by value rather than reading suite state: the mock invokes it on
	// its own HTTP handler goroutine, so a shared field would be an unsynchronized cross-goroutine
	// access. SetAuthorizeFunc itself is mutex-guarded.
	ts.mockGoogle.SetAuthorizeFunc(func(string) (string, error) { return email, nil })

	flowStep, err := common.InitiateAuthenticationFlow(appID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate the federated flow")
	ts.Require().Equal("REDIRECTION", flowStep.Type, "Expected a redirection to the IdP")
	ts.Require().NotEmpty(flowStep.Data.RedirectURL, "Expected a redirect URL")

	authCode, state, err := testutils.SimulateFederatedOAuthFlow(flowStep.Data.RedirectURL)
	ts.Require().NoError(err, "Failed to simulate the Google authorization")

	flowStep, err = common.CompleteFlow(flowStep.ExecutionID,
		map[string]string{"code": authCode, "state": state}, "", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to complete the federated callback")
	return flowStep
}

// usersWithEmail returns the users matching an exact email via GET /users?filter=.
func (ts *FederatedProvisioningTestSuite) usersWithEmail(email string) []testutils.User {
	req, err := http.NewRequest(http.MethodGet, testutils.TestServerURL+"/users", nil)
	ts.Require().NoError(err, "Failed to build the user list request")

	q := url.Values{}
	q.Add("filter", `email eq "`+email+`"`)
	req.URL.RawQuery = q.Encode()

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err, "GET /users with an email filter failed")
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusOK, resp.StatusCode, "Expected 200 from the filtered user list")

	var listResp testutils.UserListResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&listResp),
		"Failed to decode the user list response")
	return listResp.Users
}

// assertProvisioningFailedWithNoAccount is the shared shape for scenarios 7-9. getDefaultEntityRef
// returns nil, so getTargetEntityRef yields an empty user type and fetchSchemaAttributes fails with
// "user type not found" (provisioning_executor.go:514-517) before any user is created.
//
// The contract asserted here is deliberately ERROR + no assertion + no account. It does NOT assert
// that flowStep.Error is nil: today the failure carries no structured error (HasRequiredInputs sets
// ExecFailure without setting execResp.Error at :385-390, and the engine copies that nil through
// flowexec/engine.go:840-841), but that is an implementation detail rather than a documented API
// contract. Adding a useful error code later would be an improvement and must not break these tests.
func (ts *FederatedProvisioningTestSuite) assertProvisioningFailedWithNoAccount(
	flowStep *common.FlowStep, email string) {
	ts.Equal("ERROR", flowStep.FlowStatus,
		"Provisioning must fail when no default entity ref can be resolved")
	ts.Empty(flowStep.Assertion, "No assertion may be issued when provisioning fails")
	leaked := ts.usersWithEmail(email)
	for _, u := range leaked {
		ts.createdUserIDs = append(ts.createdUserIDs, u.ID)
	}
	ts.Empty(leaked, "No account may be created when provisioning fails")
}

// Scenario 7: an app with no allowedUserTypes cannot resolve a default entity ref
// (provisioning_executor.go:822-825), so a federated login provisions nothing.
func (ts *FederatedProvisioningTestSuite) TestNoAllowedUserTypesDoesNotProvision() {
	sub := "fedprov-notypes-" + common.GenerateUniqueUsername("sub")
	email := sub + "@example.com"

	ts.assertProvisioningFailedWithNoAccount(
		ts.federatedLogin(ts.noTypesAppID, sub, email), email)
}

// Scenario 8: an app whose allowed user types all forbid self-registration must not auto-provision a
// federated user (provisioning_executor.go:842-845). Security property.
func (ts *FederatedProvisioningTestSuite) TestNoSelfRegistrationTypeDoesNotProvision() {
	sub := "fedprov-noselfreg-" + common.GenerateUniqueUsername("sub")
	email := sub + "@example.com"

	ts.assertProvisioningFailedWithNoAccount(
		ts.federatedLogin(ts.noSelfRegAppID, sub, email), email)
}

// Scenario 9: two allowed types both permit self-registration, so the choice is ambiguous and must
// fail closed rather than picking one (provisioning_executor.go:848-852). Security property.
func (ts *FederatedProvisioningTestSuite) TestAmbiguousSelfRegistrationDoesNotProvision() {
	sub := "fedprov-ambiguous-" + common.GenerateUniqueUsername("sub")
	email := sub + "@example.com"

	ts.assertProvisioningFailedWithNoAccount(
		ts.federatedLogin(ts.ambiguousAppID, sub, email), email)
}

// Scenario 10: exactly one allowed type permits self-registration, so the user is provisioned into
// that type and its own OU (provisioning_executor.go:854-857).
func (ts *FederatedProvisioningTestSuite) TestSingleSelfRegistrationTypeProvisionsIntoThatType() {
	sub := "fedprov-single-" + common.GenerateUniqueUsername("sub")
	email := sub + "@example.com"

	flowStep := ts.federatedLogin(ts.singleAppID, sub, email)

	ts.Require().Equal("COMPLETE", flowStep.FlowStatus,
		"Provisioning must complete when exactly one allowed type permits self-registration")
	ts.Require().NotEmpty(flowStep.Assertion, "A successful provisioning must issue an assertion")

	created := ts.usersWithEmail(email)
	ts.Require().Len(created, 1, "Exactly one user must be provisioned")
	ts.createdUserIDs = append(ts.createdUserIDs, created[0].ID)

	ts.Equal(ts.primaryTypeName, created[0].Type,
		"The user must be provisioned into the only self-registration-enabled type")
	ts.Equal(ts.ouAID, created[0].OUID,
		"The user must land in that user type's own OU")

	// Second, independent view of the same contract: the assertion's own claims.
	_, err := testutils.ValidateJWTAssertionFields(flowStep.Assertion, ts.singleAppID,
		ts.primaryTypeName, ts.ouAID, ts.ouAName, ts.ouAHandle)
	ts.Require().NoError(err, "The assertion must carry the provisioned user type and OU")
}

// Scenario 11: when OUResolverExecutor supplies an OU, getTargetEntityRef keeps it and takes only the
// user type from the default entity ref (provisioning_executor.go:691-696). promptAll is used because
// plain prompt needs the defaultOUID that UserTypeResolver would have set, and this flow has none.
func (ts *FederatedProvisioningTestSuite) TestResolvedOUKeptWhileUserTypeComesFromDefaultRef() {
	sub := "fedprov-promptall-" + common.GenerateUniqueUsername("sub")
	email := sub + "@example.com"

	flowStep := ts.federatedLogin(ts.promptAllAppID, sub, email)

	// promptAll offers the OU tree, so the flow stops for a selection before provisioning.
	ts.Require().NotEqual("ERROR", flowStep.FlowStatus,
		"promptAll must offer an OU selection rather than failing, got %+v", flowStep.Error)
	ts.Require().True(common.HasInput(flowStep.Data.Inputs, "ouId"),
		"promptAll must prompt for an OU selection, got inputs %+v", flowStep.Data.Inputs)

	// Select OU-B, which is NOT the primary user type's own OU.
	flowStep, err := common.CompleteFlow(flowStep.ExecutionID,
		map[string]string{"ouId": ts.ouBID}, "action_ou", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the OU selection")
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus,
		"Provisioning must complete with a resolved OU and a defaulted user type")

	created := ts.usersWithEmail(email)
	ts.Require().Len(created, 1, "Exactly one user must be provisioned")
	ts.createdUserIDs = append(ts.createdUserIDs, created[0].ID)

	ts.Equal(ts.ouBID, created[0].OUID,
		"The explicitly resolved OU must win over the user type's own OU")
	ts.Equal(ts.primaryTypeName, created[0].Type,
		"The user type must still come from the default entity ref")
}
