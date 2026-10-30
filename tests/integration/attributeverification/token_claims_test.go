// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package attributeverification

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// The OIDC application the suite signs users in through, and the claims it releases.
const (
	tokenClaimsRedirectURI  = "http://localhost:3000/callback"
	tokenClaimsClientSecret = "Verification-Token-Claims-Secret1!"
	tokenClaimsScope        = "openid email phone"

	emailVerifiedClaim  = "email_verified"
	mobileVerifiedClaim = "mobile_number_verified"
)

// The consent prompt a sign-in pauses on, and the error a refused application attribute gets.
const (
	consentDecisionsInput     = "consent_decisions"
	consentPromptDataKey      = "consentPrompt"
	errCodeInvalidAppUserAttr = "APP-1035"
)

// federatedIdPPort is where the test deployment points the server for the Google endpoints, so the mock
// IdP cannot take a free port.
const federatedIdPPort = 8093

// TokenClaimsTestSuite checks the <contact>_verified claims in the ID token, the access token, a refreshed
// access token, the UserInfo response and a sign-in flow's assertion: only the server's verification of
// the released value sets them, and an application and its consent prompt offer them only beside a
// contact.
type TokenClaimsTestSuite struct {
	suite.Suite
	fixture *verificationFixture
	sms     *smsProvider
	// declaredType is a user type whose schema defines email_verified itself.
	declaredType string
	flowID       string
	clientID     string
}

func TestTokenClaimsTestSuite(t *testing.T) {
	suite.Run(t, new(TokenClaimsTestSuite))
}

func (ts *TokenClaimsTestSuite) SetupSuite() {
	t := ts.T()
	ts.fixture = newVerificationFixture(t, "token_claims")
	ts.sms = newSMSProvider(t, "Token Claims")
	ts.fixture.nameSender(t, mobileSendNode, ts.sms.senderID)

	schema := verificationUserTypeSchema()
	schema[emailVerifiedClaim] = map[string]interface{}{"type": "boolean"}
	_, ts.declaredType = ts.fixture.newUserType(t, "token_claims_declared", schema)

	flowID := createFlow(t, testutils.Flow{
		Name:     "Attribute Verification Token Claims Sign-in",
		FlowType: "AUTHENTICATION",
		Handle:   "verification-token-claims-sign-in-" + ts.fixture.suffix,
		Nodes: linearNodes(
			promptNode("prompt_credentials", "action_001", "username", "password"),
			taskNode("credentials_auth", "CredentialsAuthExecutor", ""),
			taskNode("auth_assert", "AuthAssertExecutor", "")),
	})

	ts.flowID = flowID
	ts.clientID = "verification_token_claims_" + ts.fixture.suffix
	ts.oauthApp(t, "Token Claims", ts.clientID, map[string][]string{
		"email": {emailAttribute, emailVerifiedClaim},
		"phone": {mobileAttribute, mobileVerifiedClaim},
	})
}

// oauthApp creates an application for the suite's user types that signs in through the suite's flow and
// releases the claims scopeClaims lists in every token and the UserInfo response, until t ends.
func (ts *TokenClaimsTestSuite) oauthApp(t *testing.T, name, clientID string, scopeClaims map[string][]string) {
	t.Helper()

	released := []string{}
	scopes := []string{"openid"}
	for scope, claims := range scopeClaims {
		scopes = append(scopes, scope)
		released = append(released, claims...)
	}
	createApp(t, testutils.Application{
		OUID:             ts.fixture.ouID,
		Name:             "Attribute Verification " + name + " App " + ts.fixture.suffix,
		Description:      "Application that releases contacts and their verification claims",
		AuthFlowID:       ts.flowID,
		AllowedUserTypes: []string{ts.fixture.userType, ts.declaredType},
		InboundAuthConfig: []map[string]interface{}{{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                clientID,
				"clientSecret":            tokenClaimsClientSecret,
				"redirectUris":            []string{tokenClaimsRedirectURI},
				"grantTypes":              []string{"authorization_code", "refresh_token"},
				"responseTypes":           []string{"code"},
				"tokenEndpointAuthMethod": "client_secret_post",
				"pkceRequired":            true,
				"scopes":                  scopes,
				"scopeClaims":             scopeClaims,
				"token": map[string]interface{}{
					"accessToken": map[string]interface{}{"userConfig": map[string]interface{}{"attributes": released}},
					"idToken":     map[string]interface{}{"userAttributes": released},
				},
				"userInfo": map[string]interface{}{"userAttributes": released},
			},
		}},
	})
}

func (ts *TokenClaimsTestSuite) SetupTest() {
	ts.fixture.smtp.ClearEmails()
}

func (ts *TokenClaimsTestSuite) TestUnverifiedContactsReleaseFalse() {
	user := ts.fixture.newUser(ts.T(), "claims_unverified")

	for source, claims := range ts.claims(ts.clientID, user) {
		ts.Equal(user.email, claims[emailAttribute], source)
		ts.Equal(false, claims[emailVerifiedClaim], source)
		ts.Equal(user.mobile, claims[mobileAttribute], source)
		ts.Equal(false, claims[mobileVerifiedClaim], source)
	}
}

func (ts *TokenClaimsTestSuite) TestVerifiedEmailReleasesTrue() {
	user := ts.fixture.newUser(ts.T(), "claims_verified")
	ts.fixture.verifyEmail(ts.T(), user)

	for source, claims := range ts.claims(ts.clientID, user) {
		ts.Equal(user.email, claims[emailAttribute], source)
		ts.Equal(true, claims[emailVerifiedClaim], source)
		ts.Equal(false, claims[mobileVerifiedClaim], source)
	}
}

func (ts *TokenClaimsTestSuite) TestVerifiedMobileNumberReleasesTrue() {
	user := ts.fixture.newUser(ts.T(), "claims_verified_mobile")
	ts.fixture.verifyMobile(ts.T(), ts.sms, user)

	for source, claims := range ts.claims(ts.clientID, user) {
		ts.Equal(user.mobile, claims[mobileAttribute], source)
		ts.Equal(true, claims[mobileVerifiedClaim], "%s: a verified mobile number must release true", source)
		ts.Equal(false, claims[emailVerifiedClaim], "%s: verifying the mobile number must not verify the email",
			source)
	}
}

func (ts *TokenClaimsTestSuite) TestChangedEmailReleasesFalse() {
	user := ts.fixture.newUser(ts.T(), "claims_changed")
	ts.fixture.verifyEmail(ts.T(), user)
	changed := otherAddress(user)
	attributes := profile(user, changed)
	attributes[mobileAttribute] = user.mobile
	ts.fixture.adminUpdate(ts.T(), user, ts.fixture.userType, attributes)

	for source, claims := range ts.claims(ts.clientID, user) {
		ts.Equal(changed, claims[emailAttribute], source)
		ts.Equal(false, claims[emailVerifiedClaim], source)
	}
}

// A schema that defines email_verified releases the stored value, even against a server verification.
func (ts *TokenClaimsTestSuite) TestSchemaDefinedClaimWins() {
	user := ts.fixture.newUserOfType(ts.T(), "claims_declared", ts.declaredType)
	ts.fixture.verifyEmail(ts.T(), user)
	ts.Run("no stored value", func() {
		for source, claims := range ts.claims(ts.clientID, user) {
			ts.Equal(user.email, claims[emailAttribute], source)
			ts.NotContains(claims, emailVerifiedClaim, source)
		}
	})

	attributes := profile(user, user.email)
	attributes[mobileAttribute] = user.mobile
	attributes[emailVerifiedClaim] = false
	ts.fixture.adminUpdate(ts.T(), user, ts.declaredType, attributes)
	ts.Require().NotEmpty(verifiedAt(ts.T(), user.id, emailAttribute), "precondition: the email stays verified")

	ts.Run("stored value", func() {
		for source, claims := range ts.claims(ts.clientID, user) {
			ts.Equal(false, claims[emailVerifiedClaim], source)
			ts.Equal(false, claims[mobileVerifiedClaim], source)
		}
	})
}

// claims signs the user in through the application with clientID and returns the claims of the ID token,
// the access token, the access token a refresh issues and the UserInfo response, keyed by where they were
// read.
func (ts *TokenClaimsTestSuite) claims(clientID string, u testUser) map[string]map[string]interface{} {
	ts.T().Helper()

	tokens, err := testutils.ObtainAccessTokenWithPassword(clientID, tokenClaimsRedirectURI, tokenClaimsScope,
		u.username, u.password, true, tokenClaimsClientSecret)
	ts.Require().NoError(err, "failed to sign %s in", u.username)
	ts.Require().NotEmpty(tokens.RefreshToken, "expected a refresh token")
	refreshed, err := testutils.RefreshAccessTokenWithClientCredentialsInBody(clientID, tokenClaimsClientSecret,
		tokens.RefreshToken)
	ts.Require().NoError(err, "failed to refresh the tokens of %s", u.username)

	claims := map[string]map[string]interface{}{"UserInfo": ts.userInfo(tokens.AccessToken)}
	for source, token := range map[string]string{"ID token": tokens.IDToken, "access token": tokens.AccessToken,
		"refreshed access token": refreshed.AccessToken} {
		claims[source], err = testutils.DecodeJWTPayloadMap(token)
		ts.Require().NoError(err, "failed to decode the %s", source)
	}
	return claims
}

// userInfo reads the UserInfo response for the access token. The default test client treats the OAuth
// endpoints as public and sends no token, so the header is set here.
func (ts *TokenClaimsTestSuite) userInfo(accessToken string) map[string]interface{} {
	ts.T().Helper()

	req, err := http.NewRequest(http.MethodGet, testServerURL+"/oauth2/userinfo", nil)
	ts.Require().NoError(err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, resp.StatusCode, "userinfo failed: %s", body)

	var userInfo map[string]interface{}
	ts.Require().NoError(json.Unmarshal(body, &userInfo))
	return userInfo
}

// An application that releases email_verified but not email, and mobile_number_verified but not
// mobile_number, issues neither claim, for a verified email as for an unverified one.
func (ts *TokenClaimsTestSuite) TestAClaimReleasedWithoutItsContactIsNotIssued() {
	clientID := "verification_claim_only_" + ts.fixture.suffix
	ts.oauthApp(ts.T(), "Claim Only", clientID, map[string][]string{
		"email": {emailVerifiedClaim}, "phone": {mobileVerifiedClaim}})
	verified := ts.fixture.newUser(ts.T(), "claim_only_verified")
	ts.fixture.verifyEmail(ts.T(), verified)

	for name, user := range map[string]testUser{
		"verified":   verified,
		"unverified": ts.fixture.newUser(ts.T(), "claim_only_unverified"),
	} {
		for source, claims := range ts.claims(clientID, user) {
			ts.NotContains(claims, emailAttribute, "precondition: the %s does not release the email", source)
			ts.NotContains(claims, mobileAttribute, "precondition: the %s does not release the number", source)
			ts.NotContains(claims, emailVerifiedClaim,
				"the %s of a %s user must not carry a claim without its contact", source, name)
			ts.NotContains(claims, mobileVerifiedClaim,
				"the %s of a %s user must not carry a claim without its contact", source, name)
		}
	}
}

// Runtime data a flow sets cannot set email_verified: the assertion carries the server's status, false
// for an unverified email whatever the flow set, and true for a verified one.
func (ts *TokenClaimsTestSuite) TestRuntimeDataCannotSetTheVerificationClaim() {
	verified := ts.fixture.newUser(ts.T(), "runtime_verified")
	ts.fixture.verifyEmail(ts.T(), verified)

	for name, tc := range map[string]struct {
		user     testUser
		supplied bool
	}{
		"unverified email, runtime true": {ts.fixture.newUser(ts.T(), "runtime_unverified"), true},
		"verified email, runtime false":  {verified, false},
	} {
		ts.Run(name, func() {
			claims := ts.assertionWith(tc.user, map[string]interface{}{emailVerifiedClaim: tc.supplied})

			ts.Equal(tc.user.email, claims[emailAttribute], "precondition: the stored email is released")
			ts.Equal(!tc.supplied, claims[emailVerifiedClaim],
				"the assertion must carry the server's status, not the runtime value")
		})
	}
}

// A flow that releases an email other than the stored, verified one releases email_verified false.
func (ts *TokenClaimsTestSuite) TestAReleasedEmailOtherThanTheStoredOneIsUnverified() {
	user := ts.fixture.newUser(ts.T(), "released_other")
	ts.fixture.verifyEmail(ts.T(), user)
	other := otherAddress(user)

	claims := ts.assertionWith(user, map[string]interface{}{emailAttribute: other})

	ts.Equal(other, claims[emailAttribute], "precondition: the flow releases another email")
	ts.Equal(false, claims[emailVerifiedClaim], "an email other than the verified one must be released unverified")
}

// assertionWith signs the user in through a flow whose HTTP request step maps response's email and
// email_verified into runtime data before the assertion is built, and returns the assertion's claims.
func (ts *TokenClaimsTestSuite) assertionWith(u testUser, response map[string]interface{}) map[string]interface{} {
	t := ts.T()
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	t.Cleanup(server.Close)
	serverURL, err := url.Parse(server.URL)
	ts.Require().NoError(err)

	request := taskNode("http_request", "HTTPRequestExecutor", "")
	request["properties"] = map[string]interface{}{
		"url":    "http://localhost:" + serverURL.Port() + "/claims",
		"method": "GET",
		"responseMapping": map[string]interface{}{
			emailAttribute:     "response.data." + emailAttribute,
			emailVerifiedClaim: "response.data." + emailVerifiedClaim,
		},
		"errorHandling": map[string]interface{}{"failOnError": true},
	}
	flowID := createFlow(t, testutils.Flow{
		Name:     "Attribute Verification Runtime Claims Sign-in",
		FlowType: "AUTHENTICATION",
		Handle:   "verification-runtime-claims-" + u.username,
		Nodes: linearNodes(taskNode("credentials_auth", "CredentialsAuthExecutor", ""), request,
			taskNode("auth_assert", "AuthAssertExecutor", "")),
	})
	appID := createApp(t, testutils.Application{
		OUID:             ts.fixture.ouID,
		Name:             "Attribute Verification Runtime Claims App " + u.username,
		ClientID:         "verification_runtime_claims_" + u.username,
		ClientSecret:     tokenClaimsClientSecret,
		AllowedUserTypes: []string{ts.fixture.userType},
		AuthFlowID:       flowID,
		AssertionConfig:  map[string]interface{}{"userAttributes": []string{emailAttribute, emailVerifiedClaim}},
	})

	done, err := common.InitiateAuthenticationFlow(appID, false,
		map[string]string{"username": u.username, "password": u.password}, "")
	ts.Require().NoError(err)
	return ts.requireAssertion(done)
}

// requireAssertion requires the sign-in to have completed and returns its assertion's claims.
func (ts *TokenClaimsTestSuite) requireAssertion(done *common.FlowStep) map[string]interface{} {
	ts.T().Helper()

	ts.Require().Equal(flowStatusComplete, done.FlowStatus, "the sign-in must complete: %+v", done)
	ts.Require().NotEmpty(done.Assertion, "the sign-in must issue an assertion")
	claims, err := testutils.DecodeJWTPayloadMap(done.Assertion)
	ts.Require().NoError(err)
	return claims
}

// An identity provider that asserts email_verified true does not verify a local user's email: the
// federated sign-in's assertion carries the server's status, false until the user verifies it.
func (ts *TokenClaimsTestSuite) TestAFederatedClaimCannotSetTheVerificationClaim() {
	t := ts.T()
	const googleClientID, googleClientSecret = "verification_google_client", "verification_google_secret"
	server, err := testutils.NewMockGoogleOIDCServer(federatedIdPPort, googleClientID, googleClientSecret)
	ts.Require().NoError(err)
	ts.Require().NoError(server.Start(), "failed to start the mock IdP")
	t.Cleanup(func() { _ = server.Stop() })
	user := ts.fixture.newUser(t, "federated")
	server.AddUser(&testutils.GoogleUserInfo{Sub: "verification-" + user.username, Email: user.email,
		EmailVerified: true, Name: user.username})
	server.SetAuthorizeFunc(func(string) (string, error) { return user.email, nil })

	idpID, err := testutils.CreateIDP(testutils.IDP{
		Name: "Attribute Verification Google " + ts.fixture.suffix,
		Type: "GOOGLE",
		Properties: []testutils.IDPProperty{
			{Name: "client_id", Value: googleClientID},
			{Name: "client_secret", Value: googleClientSecret, IsSecret: true},
			{Name: "redirect_uri", Value: tokenClaimsRedirectURI},
			{Name: "scopes", Value: "openid email profile"},
		},
		AttributeConfiguration: &testutils.AttributeConfiguration{
			AccountLinking: &testutils.AccountLinking{Attributes: []string{emailAttribute}},
		},
	})
	ts.Require().NoError(err, "failed to create the IdP")
	t.Cleanup(func() {
		if err := testutils.DeleteIDP(idpID); err != nil {
			t.Errorf("cleanup: failed to delete the IdP: %v", err)
		}
	})
	ts.Require().NoError(common.LinkAccount(common.LinkRequest{
		Handle:       "verification-link-" + ts.fixture.suffix,
		ExecutorName: "GoogleOIDCAuthExecutor",
		IDPID:        idpID,
		OUID:         ts.fixture.ouID,
		UserType:     ts.fixture.userType,
		Username:     user.username,
		Password:     user.password,
	}), "precondition: the IdP identity is linked to the user")

	federated := taskNode("google_auth", "GoogleOIDCAuthExecutor", "")
	federated["properties"] = map[string]interface{}{"idpId": idpID}
	flowID := createFlow(t, testutils.Flow{
		Name:     "Attribute Verification Federated Sign-in",
		FlowType: "AUTHENTICATION",
		Handle:   "verification-federated-" + ts.fixture.suffix,
		Nodes:    linearNodes(federated, taskNode("auth_assert", "AuthAssertExecutor", "")),
	})
	appID := createApp(t, testutils.Application{
		OUID:             ts.fixture.ouID,
		Name:             "Attribute Verification Federated App " + ts.fixture.suffix,
		ClientID:         "verification_federated_" + ts.fixture.suffix,
		ClientSecret:     tokenClaimsClientSecret,
		RedirectURIs:     []string{tokenClaimsRedirectURI},
		AllowedUserTypes: []string{ts.fixture.userType},
		AuthFlowID:       flowID,
		AssertionConfig:  map[string]interface{}{"userAttributes": []string{emailAttribute, emailVerifiedClaim}},
	})

	step, err := common.InitiateAuthenticationFlow(appID, false, nil, "")
	ts.Require().NoError(err)
	ts.Require().Equal("REDIRECTION", step.Type, "precondition: the flow redirects to the IdP: %+v", step)
	code, state, err := testutils.SimulateFederatedOAuthFlow(step.Data.RedirectURL)
	ts.Require().NoError(err, "precondition: the IdP authorizes the user")
	done, err := common.CompleteFlow(step.ExecutionID, map[string]string{"code": code, "state": state}, "",
		step.ChallengeToken)
	ts.Require().NoError(err)

	claims := ts.requireAssertion(done)
	ts.Equal(user.id, claims["sub"], "precondition: the IdP identity signs the local user in")
	ts.Equal(user.email, claims[emailAttribute], "precondition: the email is released")
	ts.Equal(false, claims[emailVerifiedClaim], "the IdP's email_verified must not verify the local email")
}

// An application may list <attribute>_verified for an email or mobile attribute of an allowed user type,
// or for a name the type's schema defines. A create listing it for any other attribute is refused, and an
// update listing it does not store it, as an update does with every attribute no allowed type declares.
func (ts *TokenClaimsTestSuite) TestApplicationsListOnlyVerificationClaimsOfContacts() {
	body := func(name string, attributes ...string) map[string]interface{} {
		return map[string]interface{}{
			"name":             "Attribute Verification Claim Config " + name + " " + ts.fixture.suffix,
			"ouId":             ts.fixture.ouID,
			"type":             "fullstack",
			"authFlowId":       ts.flowID,
			"allowedUserTypes": []string{ts.fixture.userType, ts.declaredType},
			"assertion":        map[string]interface{}{"userAttributes": attributes},
			"inboundAuthConfig": []map[string]interface{}{{"type": "oauth2", "config": map[string]interface{}{
				"clientId":     "verification_claim_config_" + name + "_" + ts.fixture.suffix,
				"clientSecret": tokenClaimsClientSecret,
				"redirectUris": []string{tokenClaimsRedirectURI},
				"grantTypes":   []string{"authorization_code"},
			}}},
		}
	}
	contacts := []string{emailAttribute, emailVerifiedClaim, mobileAttribute, mobileVerifiedClaim}

	status, created := sendJSON(ts.T(), testutils.GetHTTPClient(), http.MethodPost, "/applications",
		body("accepted", contacts...))
	ts.Require().Equal(http.StatusCreated, status, "the verification claims of contacts must be accepted: %s",
		created)
	var app struct {
		ID        string `json:"id"`
		Assertion struct {
			UserAttributes []string `json:"userAttributes"`
		} `json:"assertion"`
	}
	ts.Require().NoError(json.Unmarshal(created, &app))
	ts.T().Cleanup(func() {
		if err := testutils.DeleteApplication(app.ID); err != nil {
			ts.T().Errorf("cleanup: failed to delete application %s: %v", app.ID, err)
		}
	})
	ts.ElementsMatch(contacts, app.Assertion.UserAttributes, "the verification claims of contacts must be stored")

	for name, attribute := range map[string]string{
		"non_contact": displayNameAttribute + "_verified",
		"username":    "username_verified",
	} {
		ts.Run(name, func() {
			status, refused := sendJSON(ts.T(), testutils.GetHTTPClient(), http.MethodPost, "/applications",
				body(name, emailAttribute, emailVerifiedClaim, attribute))

			ts.Require().Equal(http.StatusBadRequest, status, "a create listing %s must be refused: %s", attribute,
				refused)
			var apiErr struct {
				Code string `json:"code"`
			}
			ts.Require().NoError(json.Unmarshal(refused, &apiErr))
			ts.Equal(errCodeInvalidAppUserAttr, apiErr.Code, "%s must be refused as an invalid user attribute: %s",
				attribute, refused)

			status, updated := sendJSON(ts.T(), testutils.GetHTTPClient(), http.MethodPut, "/applications/"+app.ID,
				body("accepted", append(contacts, attribute)...))
			ts.Require().Equal(http.StatusOK, status, "%s", updated)
			var stored struct {
				Assertion struct {
					UserAttributes []string `json:"userAttributes"`
				} `json:"assertion"`
			}
			ts.Require().NoError(json.Unmarshal(updated, &stored))
			ts.NotContains(stored.Assertion.UserAttributes, attribute, "an update must not store %s", attribute)
		})
	}
}

// A sign-in's consent prompt offers email_verified beside the email of a user whose type declares it an
// email, and not beside an email attribute that is a plain string.
func (ts *TokenClaimsTestSuite) TestConsentOffersTheClaimOnlyBesideAContact() {
	t := ts.T()
	plain := verificationUserTypeSchema()
	plain[emailAttribute] = map[string]interface{}{"type": "string"}
	plain[mobileAttribute] = map[string]interface{}{"type": "string"}
	_, plainType := ts.fixture.newUserType(t, "consent_plain", plain)

	consent := taskNode("consent_check", "ConsentExecutor", "")
	consent["onSuccess"] = "auth_assert"
	consent["onIncomplete"] = "prompt_consent"
	consentPrompt := promptNode("prompt_consent", "consent_action_allow")
	consentPrompt["prompts"].([]map[string]interface{})[0]["inputs"] = []map[string]interface{}{
		{"ref": "consent_input", "identifier": consentDecisionsInput, "type": "CONSENT_INPUT", "required": true}}
	consentPrompt["prompts"].([]map[string]interface{})[0]["action"].(map[string]interface{})["nextNode"] =
		"consent_check"
	flowID := createFlow(t, testutils.Flow{
		Name:     "Attribute Verification Consent Sign-in",
		FlowType: "AUTHENTICATION",
		Handle:   "verification-consent-" + ts.fixture.suffix,
		Nodes: linearNodes(taskNode("credentials_auth", "CredentialsAuthExecutor", ""), consent, consentPrompt,
			taskNode("auth_assert", "AuthAssertExecutor", "")),
	})
	appID := createApp(t, testutils.Application{
		OUID:             ts.fixture.ouID,
		Name:             "Attribute Verification Consent App " + ts.fixture.suffix,
		ClientID:         "verification_consent_" + ts.fixture.suffix,
		ClientSecret:     tokenClaimsClientSecret,
		AllowedUserTypes: []string{ts.fixture.userType, plainType},
		AuthFlowID:       flowID,
		AssertionConfig:  map[string]interface{}{"userAttributes": []string{emailAttribute, emailVerifiedClaim}},
	})

	for name, tc := range map[string]struct {
		user    testUser
		offered bool
	}{
		"email attribute":  {ts.fixture.newUser(t, "consent_contact"), true},
		"string attribute": {ts.fixture.newUserOfType(t, "consent_plain", plainType), false},
	} {
		ts.Run(name, func() {
			step, err := common.InitiateAuthenticationFlow(appID, false,
				map[string]string{"username": tc.user.username, "password": tc.user.password}, "")
			ts.Require().NoError(err)

			offered := consentElements(ts.T(), step)
			ts.Contains(offered, emailAttribute, "precondition: the email is offered")
			if tc.offered {
				ts.Contains(offered, emailVerifiedClaim, "the claim must be offered beside a contact")
			} else {
				ts.NotContains(offered, emailVerifiedClaim, "the claim must not be offered beside a non-contact")
			}
		})
	}
}

// consentElements requires the step to be a consent prompt and returns the names of the elements it
// offers.
func consentElements(t *testing.T, step *common.FlowStep) []string {
	t.Helper()

	if step.FlowStatus != flowStatusIncomplete || !common.HasInput(step.Data.Inputs, consentDecisionsInput) {
		t.Fatalf("the sign-in must pause for consent: %+v", step)
	}
	var purposes []struct {
		Essential []struct {
			Name string `json:"name"`
		} `json:"essential"`
		Optional []struct {
			Name string `json:"name"`
		} `json:"optional"`
	}
	if err := json.Unmarshal([]byte(step.Data.AdditionalData[consentPromptDataKey]), &purposes); err != nil {
		t.Fatalf("the consent prompt must carry its purposes: %v: %+v", err, step.Data)
	}
	var names []string
	for _, purpose := range purposes {
		for _, element := range purpose.Essential {
			names = append(names, element.Name)
		}
		for _, element := range purpose.Optional {
			names = append(names, element.Name)
		}
	}
	return names
}
