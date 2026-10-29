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
	errCodeOUNotDirectChild = "FET-1121"
	errCodeInvalidOU        = "FET-1029"

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
	ouAChildID string
	ouBID      string
	ouAName    string
	ouAHandle  string

	ouAChildHandle      string
	ouAGrandchildID     string
	ouAGrandchildHandle string

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

	promptParentAppID       string
	directChildrenIDAppID   string
	directChildrenHandleApp string

	createdFlowIDs []string
	createdUserIDs []string
}

func TestFederatedProvisioningTestSuite(t *testing.T) {
	suite.Run(t, new(FederatedProvisioningTestSuite))
}

// buildFederatedProvisioningFlow returns a fresh AUTHENTICATION graph modelled on the proven
// conditionalExecFlow: federated login, then provisioning gated on userEligibleForProvisioning, then
// AuthAssert. A builder rather than a package var, because conditionalExecFlow is mutated in place by
// its own suite and sharing it would race. When ouProperties is non-nil an OUResolverExecutor with those
// properties is inserted ahead of provisioning. When ouInput is set the resolver declares it as its single
// required input, "ouHandle" to name the OU by handle. The OU prompt asks for ouInput, or ouId when it is empty.
func buildFederatedProvisioningFlow(
	handle, idpID string, ouProperties map[string]interface{}, ouInput string) testutils.Flow {
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

	if ouProperties != nil {
		// promptAll shows the whole OU tree and needs no default OU. Plain prompt selects below a parent
		// OU, and this flow has no UserTypeResolver to set a default one, so it needs parentOuId.
		promptInput := "ouId"
		resolver := map[string]interface{}{"name": "OUResolverExecutor"}
		if ouInput != "" {
			promptInput = ouInput
			resolver["inputs"] = []map[string]interface{}{
				{"ref": "input_ou_resolver", "identifier": ouInput, "type": "TEXT_INPUT", "required": true},
			}
		}
		nodes[1]["onSuccess"] = "resolve_ou"
		nodes = append(nodes, map[string]interface{}{
			"id":         "resolve_ou",
			"type":       "TASK_EXECUTION",
			"properties": ouProperties,
			"condition": map[string]interface{}{
				"key":    "{{ctx(userEligibleForProvisioning)}}",
				"value":  "true",
				"onSkip": "auth_assert",
			},
			"executor":     resolver,
			"onSuccess":    "provision_user",
			"onIncomplete": "prompt_ou",
		})
		nodes = append(nodes, map[string]interface{}{
			"id":   "prompt_ou",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{
				{
					"inputs": []map[string]interface{}{
						{"ref": "input_ou", "identifier": promptInput, "type": "TEXT_INPUT", "required": true},
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

	// A child of OU-A, holding no user types of its own. Scenario 11 provisions into it to show the
	// resolved OU overriding the user type's own OU. It must sit inside OU-A's subtree because a
	// user type may only be used in its own organization unit or one beneath it; a sibling of OU-A
	// would be rejected by the user service. It is kept out of OU-B so the self-registration type
	// counts the other scenarios rely on are unaffected.
	ts.ouAChildID, err = testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "fedprov-ou-a-child", Name: "Federated Provisioning OU A Child", Parent: &ts.ouAID,
	})
	ts.Require().NoError(err, "Failed to create OU A child")
	ts.ouAChildHandle = "fedprov-ou-a-child"

	// A grandchild of OU-A, to tell a direct child of OU-A from a deeper descendant.
	ts.ouAGrandchildHandle = "fedprov-ou-a-grandchild"
	ts.ouAGrandchildID, err = testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: ts.ouAGrandchildHandle, Name: "Federated Provisioning OU A Grandchild", Parent: &ts.ouAChildID,
	})
	ts.Require().NoError(err, "Failed to create OU A grandchild")

	// OU-A owns the only self-registration-enabled type used by scenario 10, so the resolved OU is
	// distinguishable from the one scenario 11 selects.
	ts.primaryTypeName = "fedprov-selfreg-primary"
	ts.primaryTypeID, err = testutils.CreateUserType(testutils.UserType{
		Handle: ts.primaryTypeName, DisplayName: "Primary Type", OUID: ts.ouAID, AllowSelfRegistration: true, Schema: fedProvSchema,
	})
	ts.Require().NoError(err, "Failed to create the primary self-registration type")

	ts.altTypeName = "fedprov-selfreg-secondary"
	ts.altTypeID, err = testutils.CreateUserType(testutils.UserType{
		Handle: ts.altTypeName, DisplayName: "Alt Type", OUID: ts.ouBID, AllowSelfRegistration: true, Schema: fedProvSchema,
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

	ts.noTypesAppID = ts.appForFlow("fedprov_flow_no_types", nil, "", "FedProv No Types App",
		"fedprov_notypes_client", nil)
	ts.noSelfRegAppID = ts.appForFlow("fedprov_flow_noselfreg", nil, "", "FedProv No Self Reg App",
		"fedprov_noselfreg_client", []string{ts.noSelfRegName})
	ts.ambiguousAppID = ts.appForFlow("fedprov_flow_ambiguous", nil, "", "FedProv Ambiguous App",
		"fedprov_ambiguous_client", []string{ts.primaryTypeName, ts.altTypeName})
	ts.singleAppID = ts.appForFlow("fedprov_flow_single", nil, "", "FedProv Single App",
		"fedprov_single_client", []string{ts.primaryTypeName, ts.noSelfRegName})
	ts.promptAllAppID = ts.appForFlow("fedprov_flow_promptall", map[string]interface{}{"resolveFrom": "promptAll"}, "",
		"FedProv PromptAll App", "fedprov_promptall_client", []string{ts.primaryTypeName, ts.noSelfRegName})

	types := []string{ts.primaryTypeName, ts.noSelfRegName}
	ts.promptParentAppID = ts.appForFlow("fedprov_flow_prompt_parent",
		map[string]interface{}{"resolveFrom": "prompt", "parentOuId": ts.ouAID}, "",
		"FedProv Prompt Parent App", "fedprov_prompt_parent_client", types)
	ts.directChildrenIDAppID = ts.appForFlow("fedprov_flow_direct_id",
		map[string]interface{}{"resolveFrom": "prompt", "parentOuId": ts.ouAID, "directChildrenOnly": true},
		"ouId", "FedProv Direct Children By ID App", "fedprov_direct_id_client", types)
	ts.directChildrenHandleApp = ts.appForFlow("fedprov_flow_direct_handle",
		map[string]interface{}{"resolveFrom": "prompt", "parentOuId": ts.ouAID, "directChildrenOnly": true},
		"ouHandle", "FedProv Direct Children By Handle App", "fedprov_direct_handle_client", types)
}

// createUserTypeWithSelfRegDisabled posts a user type with allowSelfRegistration explicitly false.
// testutils.CreateUserType cannot express this: it actively rewrites a false to true
// (testutils/api_utils.go:68-70), so a type created through it is always self-registration-enabled.
func (ts *FederatedProvisioningTestSuite) createUserTypeWithSelfRegDisabled(
	name, ouID string) string {
	payload := map[string]interface{}{
		"handle":                name,
		"displayName":           name,
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
	flowHandle string, ouProperties map[string]interface{}, ouInput, appName, clientID string,
	allowedTypes []string) string {
	flowID, err := testutils.CreateFlow(
		buildFederatedProvisioningFlow(flowHandle, ts.idpID, ouProperties, ouInput))
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
			"userAttributes": []string{"userTypeHandle", "ouId", "ouName", "ouHandle"},
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
		ts.singleAppID, ts.promptAllAppID, ts.promptParentAppID, ts.directChildrenIDAppID,
		ts.directChildrenHandleApp} {
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
	// Children before parents: an organization unit cannot be removed while it still has children.
	for _, ouID := range []string{ts.ouAGrandchildID, ts.ouAChildID, ts.ouAID, ts.ouBID, ts.parentOUID} {
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

// Scenario 12: a user type may only be used in its own organization unit or one beneath it. When a
// flow resolves an OU outside that subtree the user service rejects the create, and the executor
// must surface that error rather than flattening it to the generic provisioning failure, so the
// caller can tell a misconfigured flow from an outage.
func (ts *FederatedProvisioningTestSuite) TestProvisioningSurfacesUserServiceErrorForCrossOUType() {
	sub := "fedprov-crossou-" + common.GenerateUniqueUsername("sub")
	email := sub + "@example.com"

	flowStep := ts.federatedLogin(ts.promptAllAppID, sub, email)
	ts.Require().True(common.HasInput(flowStep.Data.Inputs, "ouId"),
		"promptAll must prompt for an OU selection, got inputs %+v", flowStep.Data.Inputs)

	// OU-B is a sibling of OU-A, so the primary user type defined in OU-A is not usable there.
	flowStep, err := common.CompleteFlow(flowStep.ExecutionID,
		map[string]string{"ouId": ts.ouBID}, "action_ou", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the OU selection")

	ts.Require().Equal("ERROR", flowStep.FlowStatus, "Provisioning must fail for a cross-OU user type")
	ts.Require().NotNil(flowStep.Error, "The failure must carry an error")
	ts.Equal("USR-1023", flowStep.Error.Code,
		"The user service error must reach the flow unchanged, not be flattened to FET-1021")

	ts.Empty(ts.usersWithEmail(email), "A rejected provisioning must not persist a user")
}

// Scenario 11: when OUResolverExecutor supplies an OU, getTargetEntityRef keeps it and takes only the
// user type from the default entity ref (provisioning_executor.go:691-696). promptAll is used because it
// needs no parent OU; the scenarios below cover plain prompt, which selects below a parentOuId.
func (ts *FederatedProvisioningTestSuite) TestResolvedOUKeptWhileUserTypeComesFromDefaultRef() {
	sub := "fedprov-promptall-" + common.GenerateUniqueUsername("sub")
	email := sub + "@example.com"

	flowStep := ts.federatedLogin(ts.promptAllAppID, sub, email)

	// promptAll offers the OU tree, so the flow stops for a selection before provisioning.
	ts.Require().NotEqual("ERROR", flowStep.FlowStatus,
		"promptAll must offer an OU selection rather than failing, got %+v", flowStep.Error)
	ts.Require().True(common.HasInput(flowStep.Data.Inputs, "ouId"),
		"promptAll must prompt for an OU selection, got inputs %+v", flowStep.Data.Inputs)

	// Select OU-A's child, which is NOT the primary user type's own OU but sits inside its subtree.
	// A sibling such as OU-B would be rejected: a user type may only be used in its own
	// organization unit or one beneath it.
	flowStep, err := common.CompleteFlow(flowStep.ExecutionID,
		map[string]string{"ouId": ts.ouAChildID}, "action_ou", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the OU selection")
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus,
		"Provisioning must complete with a resolved OU and a defaulted user type")

	created := ts.usersWithEmail(email)
	ts.Require().Len(created, 1, "Exactly one user must be provisioned")
	ts.createdUserIDs = append(ts.createdUserIDs, created[0].ID)

	ts.Equal(ts.ouAChildID, created[0].OUID,
		"The explicitly resolved OU must win over the user type's own OU")
	ts.Equal(ts.primaryTypeName, created[0].Type,
		"The user type must still come from the default entity ref")
}

// completeOUSelection submits the user's OU choice at the prompt and returns the next step.
func (ts *FederatedProvisioningTestSuite) completeOUSelection(
	flowStep *common.FlowStep, input, value string) *common.FlowStep {
	next, err := common.CompleteFlow(flowStep.ExecutionID,
		map[string]string{input: value}, "action_ou", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the OU selection")
	return next
}

// assertProvisionedInto checks that exactly one user was provisioned for the email, into the given OU.
func (ts *FederatedProvisioningTestSuite) assertProvisionedInto(email, ouID string) {
	created := ts.usersWithEmail(email)
	ts.Require().Len(created, 1, "Exactly one user must be provisioned")
	ts.createdUserIDs = append(ts.createdUserIDs, created[0].ID)
	ts.Equal(ouID, created[0].OUID, "The user must be provisioned into the selected OU")
}

// Scenario 12: a sign-in flow has no UserTypeResolver and so no default OU. With parentOuId set, the
// prompt strategy offers a selection below that OU and provisions the user into the chosen one.
func (ts *FederatedProvisioningTestSuite) TestSignInPromptSelectsBelowParentOU() {
	sub := "fedprov-parent-" + common.GenerateUniqueUsername("sub")
	email := sub + "@example.com"

	flowStep := ts.federatedLogin(ts.promptParentAppID, sub, email)
	ts.Require().NotEqual("ERROR", flowStep.FlowStatus,
		"prompt with parentOuId must offer a selection, got %+v", flowStep.Error)
	ts.Require().True(common.HasInput(flowStep.Data.Inputs, "ouId"),
		"prompt must ask for an OU below the parent, got inputs %+v", flowStep.Data.Inputs)

	flowStep = ts.completeOUSelection(flowStep, "ouId", ts.ouAChildID)
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus, "Provisioning must complete in the selected OU")
	ts.assertProvisionedInto(email, ts.ouAChildID)
}

// Scenario 13: with directChildrenOnly and an ID, a deeper descendant of the parent OU is refused and
// the selection is asked for again, while a direct child is accepted.
func (ts *FederatedProvisioningTestSuite) TestDirectChildrenOnlyByID() {
	sub := "fedprov-direct-id-" + common.GenerateUniqueUsername("sub")
	email := sub + "@example.com"

	flowStep := ts.federatedLogin(ts.directChildrenIDAppID, sub, email)
	ts.Require().True(common.HasInput(flowStep.Data.Inputs, "ouId"),
		"The flow must ask for an OU ID, got inputs %+v", flowStep.Data.Inputs)

	rejected := ts.completeOUSelection(flowStep, "ouId", ts.ouAGrandchildID)
	ts.Require().NotNil(rejected.Error, "A grandchild must be refused")
	ts.Equal(errCodeOUNotDirectChild, rejected.Error.Code, "The refusal must say it is not a direct child")
	ts.Require().True(common.HasInput(rejected.Data.Inputs, "ouId"), "The selection must be asked for again")
	ts.Empty(ts.usersWithEmail(email), "A refused selection must not provision a user")

	accepted := ts.completeOUSelection(rejected, "ouId", ts.ouAChildID)
	ts.Require().Equal("COMPLETE", accepted.FlowStatus, "A direct child must be accepted")
	ts.assertProvisionedInto(email, ts.ouAChildID)
}

// Scenario 14: with an ouHandle input on the resolver the user names the OU by handle, which is looked up among the
// parent OU's direct children only. A handle that exists only deeper in the tree is refused.
func (ts *FederatedProvisioningTestSuite) TestDirectChildrenOnlyByHandle() {
	sub := "fedprov-direct-handle-" + common.GenerateUniqueUsername("sub")
	email := sub + "@example.com"

	flowStep := ts.federatedLogin(ts.directChildrenHandleApp, sub, email)
	ts.Require().True(common.HasInput(flowStep.Data.Inputs, "ouHandle"),
		"The flow must ask for an OU handle, got inputs %+v", flowStep.Data.Inputs)

	rejected := ts.completeOUSelection(flowStep, "ouHandle", ts.ouAGrandchildHandle)
	ts.Require().NotNil(rejected.Error, "A handle that is not a direct child must be refused")
	ts.Equal(errCodeInvalidOU, rejected.Error.Code, "The refusal must say the OU is invalid")
	ts.Require().True(common.HasInput(rejected.Data.Inputs, "ouHandle"), "The selection must be asked for again")
	ts.Empty(ts.usersWithEmail(email), "A refused selection must not provision a user")

	accepted := ts.completeOUSelection(rejected, "ouHandle", ts.ouAChildHandle)
	ts.Require().Equal("COMPLETE", accepted.FlowStatus, "A direct child's handle must be accepted")
	ts.assertProvisionedInto(email, ts.ouAChildID)
}
