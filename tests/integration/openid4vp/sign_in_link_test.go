// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package openid4vp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	signInLinkVCT          = "https://credentials.thunderid.local/SignInLinkTestCredential"
	signInLinkSubjectClaim = "national_id"
	// signInLinkRefreshAction polls the verifier from the wallet wait view.
	signInLinkRefreshAction = "action_refresh"
)

var signInLinkTestOU = testutils.OrganizationUnit{
	Handle:      "openid4vp-sign-in-link-ou",
	Name:        "OpenID4VP Sign-in Link OU",
	Description: "Organization unit for OpenID4VP sign-in link testing",
}

// OpenID4VPSignInLinkTestSuite signs holders in through an OpenID4VP flow and checks that the link
// recorded on provisioning is scoped to the presentation definition and issuer.
type OpenID4VPSignInLinkTestSuite struct {
	suite.Suite
	wallet *testutils.VPWallet

	ouID       string
	userTypeID string

	definitionAID string
	appA          string
	appB          string
	appUnenforced string

	definitionIDs []string
	flowIDs       []string
	appIDs        []string
	userIDs       []string
}

func TestOpenID4VPSignInLinkTestSuite(t *testing.T) {
	suite.Run(t, new(OpenID4VPSignInLinkTestSuite))
}

func (ts *OpenID4VPSignInLinkTestSuite) SetupSuite() {
	wallet, err := testutils.NewVPWallet()
	ts.Require().NoError(err)
	ts.wallet = wallet

	ts.ouID, err = testutils.CreateOrganizationUnit(signInLinkTestOU)
	ts.Require().NoError(err, "create test OU")

	userTypeHandle := "openid4vp-sign-in-link-holder"
	ts.userTypeID, err = testutils.CreateUserType(testutils.UserType{
		Handle:                userTypeHandle,
		DisplayName:           "OpenID4VP Holder",
		OUID:                  ts.ouID,
		AllowSelfRegistration: true,
		Schema: map[string]interface{}{
			"given_name":           map[string]interface{}{"type": "string"},
			signInLinkSubjectClaim: map[string]interface{}{"type": "string"},
		},
	})
	ts.Require().NoError(err, "create user type")

	trusted := []testutils.TrustedAuthority{{Name: "wallet-issuer", Certificate: wallet.IssuerCertificatePEM()}}
	ts.definitionAID = ts.createDefinition("sign_in_link_a", true, trusted)
	ts.createDefinition("sign_in_link_b", true, trusted)
	ts.createDefinition("sign_in_link_unenforced", false, nil)

	ts.appA = ts.createApp("sign_in_link_a", userTypeHandle)
	ts.appB = ts.createApp("sign_in_link_b", userTypeHandle)
	ts.appUnenforced = ts.createApp("sign_in_link_unenforced", userTypeHandle)
}

func (ts *OpenID4VPSignInLinkTestSuite) TearDownSuite() {
	if err := testutils.CleanupUsers(ts.userIDs); err != nil {
		ts.T().Logf("Failed to clean up users: %v", err)
	}
	for _, id := range ts.appIDs {
		if err := testutils.DeleteApplication(id); err != nil {
			ts.T().Logf("Failed to delete application %s: %v", id, err)
		}
	}
	for _, id := range ts.flowIDs {
		if err := testutils.DeleteFlow(id); err != nil {
			ts.T().Logf("Failed to delete flow %s: %v", id, err)
		}
	}
	for _, id := range ts.definitionIDs {
		if err := testutils.DeletePresentationDefinition(id); err != nil {
			ts.T().Logf("Failed to delete presentation definition %s: %v", id, err)
		}
	}
	if ts.userTypeID != "" {
		if err := testutils.DeleteUserType(ts.userTypeID); err != nil {
			ts.T().Logf("Failed to delete user type: %v", err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete test organization unit: %v", err)
		}
	}
}

// createDefinition creates a definition that identifies the holder by the national_id claim.
func (ts *OpenID4VPSignInLinkTestSuite) createDefinition(
	handle string, enforce bool, authorities []testutils.TrustedAuthority,
) string {
	id, err := testutils.CreatePresentationDefinition(testutils.PresentationDefinition{
		Handle:               handle,
		OUID:                 ts.ouID,
		Name:                 "Sign-in Link Test Presentation",
		VCT:                  signInLinkVCT,
		Format:               "dc+sd-jwt",
		MandatoryClaims:      []string{"given_name", signInLinkSubjectClaim},
		EnforceTrustedIssuer: &enforce,
		TrustedAuthorities:   authorities,
		SubjectClaim:         signInLinkSubjectClaim,
	})
	ts.Require().NoError(err, "create presentation definition %s", handle)
	ts.definitionIDs = append(ts.definitionIDs, id)
	return id
}

// createApp creates an application whose sign-in flow verifies a presentation for definitionHandle and
// provisions an unknown holder.
func (ts *OpenID4VPSignInLinkTestSuite) createApp(definitionHandle, userTypeHandle string) string {
	flowID, err := testutils.CreateFlow(testutils.Flow{
		Name:     "OpenID4VP Sign-in Link Flow " + definitionHandle,
		FlowType: "AUTHENTICATION",
		Handle:   "flow_" + definitionHandle,
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "verify_presentation"},
			{
				"id":   "verify_presentation",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"presentation_definition_id":          definitionHandle,
					"allowAuthenticationWithoutLocalUser": true,
				},
				"executor":     map[string]interface{}{"name": "OpenID4VPVerifyExecutor"},
				"onSuccess":    "provision_user",
				"onIncomplete": "wait_for_wallet",
			},
			{
				"id":   "wait_for_wallet",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{"action": map[string]interface{}{"ref": signInLinkRefreshAction, "nextNode": "verify_presentation"}},
				},
			},
			{
				"id":   "provision_user",
				"type": "TASK_EXECUTION",
				"condition": map[string]interface{}{
					"key":    "{{ctx(userEligibleForProvisioning)}}",
					"value":  "true",
					"onSkip": "auth_assert",
				},
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
	})
	ts.Require().NoError(err, "create flow for %s", definitionHandle)
	ts.flowIDs = append(ts.flowIDs, flowID)

	clientID := "openid4vp_" + definitionHandle + "_client"
	appID, err := testutils.CreateApplication(testutils.Application{
		OUID:             ts.ouID,
		Name:             "OpenID4VP " + definitionHandle,
		ClientID:         clientID,
		ClientSecret:     clientID + "_secret",
		RedirectURIs:     []string{"http://localhost:3000/callback"},
		AllowedUserTypes: []string{userTypeHandle},
		AuthFlowID:       flowID,
	})
	ts.Require().NoError(err, "create application for %s", definitionHandle)
	ts.appIDs = append(ts.appIDs, appID)
	return appID
}

// signIn runs appID's sign-in flow, presenting a credential that identifies the holder by nationalID,
// and returns the final flow step.
func (ts *OpenID4VPSignInLinkTestSuite) signIn(appID, nationalID string) *common.FlowStep {
	step, err := common.InitiateAuthenticationFlow(appID, false, nil, "")
	ts.Require().NoError(err, "initiate sign-in")
	requestURI := step.Data.AdditionalData["openid4vpRequestUri"]
	ts.Require().NotEmptyf(requestURI, "flow did not hand out a request URI: %+v", step)
	parsed, err := url.Parse(requestURI)
	ts.Require().NoError(err)
	state := parsed.Query().Get("state")
	ts.Require().NotEmpty(state, "request URI carries no state")

	_, jar, _, err := testutils.FetchVPRequestObject(state)
	ts.Require().NoError(err)
	req, err := testutils.ParseVPRequestObject(jar)
	ts.Require().NoError(err)
	presentation, err := ts.wallet.BuildPresentation(req,
		map[string]any{"given_name": "Ada", signInLinkSubjectClaim: nationalID})
	ts.Require().NoError(err)
	response, err := ts.wallet.EncryptResponse(req, presentation)
	ts.Require().NoError(err)
	submit, err := testutils.SubmitVPResponse(state, response)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, submit.StatusCode, "submit presentation: %s", string(submit.Body))

	for i := 0; i < 10; i++ {
		step, err = common.CompleteFlow(step.ExecutionID, map[string]string{}, signInLinkRefreshAction,
			step.ChallengeToken)
		ts.Require().NoError(err, "poll sign-in")
		if step.FlowStatus != "INCOMPLETE" {
			return step
		}
		time.Sleep(200 * time.Millisecond)
	}
	ts.FailNow("sign-in did not finish", "last step: %+v", step)
	return nil
}

// signedInUser returns the user a completed sign-in asserts, and registers it for cleanup.
func (ts *OpenID4VPSignInLinkTestSuite) signedInUser(step *common.FlowStep) string {
	ts.Require().Equalf("COMPLETE", step.FlowStatus, "sign-in failed: %+v", step.Error)
	claims, err := testutils.DecodeJWT(step.Assertion)
	ts.Require().NoError(err)
	ts.Require().NotEmpty(claims.Sub)
	if !slices.Contains(ts.userIDs, claims.Sub) {
		ts.userIDs = append(ts.userIDs, claims.Sub)
	}
	return claims.Sub
}

func uniqueNationalID() string {
	return fmt.Sprintf("DE-%d", time.Now().UnixNano())
}

func (ts *OpenID4VPSignInLinkTestSuite) TestDefinitionReturnsSubjectClaim() {
	req, err := http.NewRequest(http.MethodGet,
		testutils.TestServerURL+"/openid4vp/presentation-definitions/"+ts.definitionAID, nil)
	ts.Require().NoError(err)
	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode)

	var fetched map[string]any
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&fetched))
	ts.Equal(signInLinkSubjectClaim, fetched["subjectClaim"])
}

// The first sign-in provisions the holder and records the link, and the next one resolves to that user.
func (ts *OpenID4VPSignInLinkTestSuite) TestSignInRecordsAndResolvesLink() {
	nationalID := uniqueNationalID()

	first := ts.signedInUser(ts.signIn(ts.appA, nationalID))
	second := ts.signedInUser(ts.signIn(ts.appA, nationalID))
	ts.Equal(first, second, "a returning holder must resolve to the user provisioned on first sign-in")

	other := ts.signedInUser(ts.signIn(ts.appA, uniqueNationalID()))
	ts.NotEqual(first, other, "another holder of the same issuer must not resolve to the first user")
}

// A link belongs to the definition that trusted the issuer, so another definition that trusts the same
// issuer does not resolve it.
func (ts *OpenID4VPSignInLinkTestSuite) TestLinkIsScopedToDefinition() {
	nationalID := uniqueNationalID()

	throughA := ts.signedInUser(ts.signIn(ts.appA, nationalID))
	throughB := ts.signedInUser(ts.signIn(ts.appB, nationalID))
	ts.NotEqual(throughA, throughB, "definition B must not resolve a link recorded through definition A")
}

// A definition that does not enforce trusted issuers verifies the credential but cannot sign anyone in.
func (ts *OpenID4VPSignInLinkTestSuite) TestUnenforcedDefinitionCannotSignIn() {
	step := ts.signIn(ts.appUnenforced, uniqueNationalID())

	ts.Equal("ERROR", step.FlowStatus)
	ts.Empty(step.Assertion)
	ts.Require().NotNil(step.Error)
	// The flow reports the verifier failure. The provider's EUDI-1006 reason is not passed through.
	ts.Equal("FET-1079", step.Error.Code)
}
