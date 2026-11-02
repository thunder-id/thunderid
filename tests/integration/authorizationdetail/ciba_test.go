// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authorizationdetail

import (
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	cibaClientID     = "authz_details_ciba_client"
	cibaClientSecret = "authz_details_ciba_secret"
	cibaResource     = "https://authz-details-ciba.example.com"
	cibaUserType     = "authz_details_ciba_user"
	cibaUsername     = "authzdetailscibauser"
	cibaPassword     = "testpassword"

	cibaGrantType    = "urn:openid:params:grant-type:ciba"
	cibaBackchannel  = "/oauth2/bc-authorize"
	cibaCallbackPath = "/oauth2/auth/callback"
)

// cibaDetailsFlow identifies the user from the login hint and sends the invite link, then, once the user
// follows it, authenticates them, runs the type processors and asks for consent before asserting.
func cibaDetailsFlow(senderID string) testutils.Flow {
	credentials := []map[string]interface{}{
		{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
		{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
	}
	return testutils.Flow{
		Name:     "Authorization Details CIBA Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "authz_details_ciba_flow",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "identify_user"},
			{
				"id":         "identify_user",
				"type":       "TASK_EXECUTION",
				"executor":   map[string]interface{}{"name": "IdentifyingExecutor", "mode": "identify"},
				"properties": map[string]interface{}{"loginHintAttribute": "username"},
				"onSuccess":  "generate_invite",
			},
			{
				"id":        "generate_invite",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "InviteExecutor", "mode": "generate"},
				"onSuccess": "send_notification",
			},
			{
				"id":         "send_notification",
				"type":       "TASK_EXECUTION",
				"executor":   map[string]interface{}{"name": "SMSExecutor"},
				"properties": map[string]interface{}{"senderId": senderID, "smsTemplate": "ciba-notification"},
				"onSuccess":  "notification_sent",
			},
			{"id": "notification_sent", "type": "PROMPT", "next": "verify_invite"},
			{
				"id":   "verify_invite",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "InviteExecutor",
					"mode": "verify",
					"inputs": []map[string]interface{}{
						{"ref": "input_invite_token", "identifier": "inviteToken", "type": "HIDDEN", "required": true},
					},
				},
				"onSuccess": "prompt_credentials",
			},
			{
				"id":   "prompt_credentials",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{{
					"inputs": credentials,
					"action": map[string]interface{}{"ref": "action_001", "nextNode": "credentials_auth"},
				}},
			},
			{
				"id":        "credentials_auth",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "CredentialsAuthExecutor", "inputs": credentials},
				"onSuccess": "authorization_details",
			},
			{
				"id":        "authorization_details",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "AuthorizationDetailExecutor"},
				"onSuccess": "consent",
			},
			{
				"id":        "consent",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "ConsentExecutor"},
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

type CIBATestSuite struct {
	suite.Suite
	notifications    *testutils.MockNotificationServer
	senderID         string
	ouID             string
	userTypeID       string
	userID           string
	resourceServerID string
	flowID           string
	appID            string
	flightTypeID     string
	flightType       string
}

func TestCIBATestSuite(t *testing.T) {
	suite.Run(t, new(CIBATestSuite))
}

func (s *CIBATestSuite) SetupSuite() {
	s.notifications = testutils.NewMockNotificationServer(0)
	s.Require().NoError(s.notifications.Start())

	senderID, err := testutils.CreateNotificationSender(testutils.NotificationSender{
		Name:     "Authorization Details CIBA Sender",
		Provider: "custom",
		Properties: []testutils.SenderProperty{
			{Name: "url", Value: s.notifications.GetSendSMSURL()},
			{Name: "http_method", Value: "POST"},
			{Name: "content_type", Value: "JSON"},
		},
	})
	s.Require().NoError(err)
	s.senderID = senderID

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "authz-details-ciba-ou",
		Name:        "Authorization Details CIBA OU",
		Description: "Organization unit for authorization details CIBA tests",
	})
	s.Require().NoError(err)
	s.ouID = ouID

	userTypeID, err := testutils.CreateUserType(testutils.UserType{
		Handle:      cibaUserType,
		DisplayName: "Authorization Details CIBA User",
		OUID:        s.ouID,
		Schema: map[string]interface{}{
			"username":      map[string]interface{}{"type": "string"},
			"password":      map[string]interface{}{"type": "string", "credential": true},
			"mobile_number": map[string]interface{}{"type": "string"},
		},
	})
	s.Require().NoError(err)
	s.userTypeID = userTypeID

	userID, err := testutils.CreateUser(testutils.User{
		Type: cibaUserType,
		OUID: s.ouID,
		Attributes: json.RawMessage(`{"username":"` + cibaUsername + `","password":"` + cibaPassword +
			`","mobile_number":"+1987654322"}`),
	})
	s.Require().NoError(err)
	s.userID = userID

	rsID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:       "Authorization Details CIBA API",
		Identifier: cibaResource,
		OUID:       s.ouID,
	}, []testutils.Action{})
	s.Require().NoError(err)
	s.resourceServerID = rsID

	flowID, err := testutils.CreateFlow(cibaDetailsFlow(s.senderID))
	s.Require().NoError(err)
	s.flowID = flowID

	appID, err := testutils.CreateApplication(testutils.Application{
		Name:             "Authorization Details CIBA App",
		Type:             "fullstack",
		OUID:             s.ouID,
		AuthFlowID:       s.flowID,
		AllowedUserTypes: []string{cibaUserType},
		LoginConsent:     map[string]interface{}{"validityPeriod": 3600},
		InboundAuthConfig: []map[string]interface{}{{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                cibaClientID,
				"clientSecret":            cibaClientSecret,
				"redirectUris":            []string{acRedirectURI},
				"grantTypes":              []string{cibaGrantType},
				"tokenEndpointAuthMethod": "client_secret_basic",
			},
		}},
	})
	s.Require().NoError(err)
	s.appID = appID

	s.flightType = uniqueTypeValue("ciba-flight")
	s.flightTypeID = createType(s.T(), typeRequest{Type: s.flightType, Name: "Flight", Schema: flightSchema,
		ConsentTemplate: "Book {{route}}"}).ID
}

func (s *CIBATestSuite) TearDownSuite() {
	if s.flightTypeID != "" {
		deleteType(s.T(), s.flightTypeID)
	}
	if s.appID != "" {
		_ = testutils.DeleteApplication(s.appID)
	}
	if s.flowID != "" {
		_ = testutils.DeleteFlow(s.flowID)
	}
	if s.resourceServerID != "" {
		_ = testutils.DeleteResourceServer(s.resourceServerID)
	}
	if s.userID != "" {
		_ = testutils.DeleteUser(s.userID)
	}
	if s.userTypeID != "" {
		_ = testutils.DeleteUserType(s.userTypeID)
	}
	if s.ouID != "" {
		_ = testutils.DeleteOrganizationUnit(s.ouID)
	}
	if s.senderID != "" {
		_ = testutils.DeleteNotificationSender(s.senderID)
	}
	if s.notifications != nil {
		_ = s.notifications.Stop()
	}
}

// Details sent on the backchannel request are prompted to the user who follows the invite, and the token the
// client then polls for carries the details the user approved.
func (s *CIBATestSuite) TestApprovedDetailsReachThePolledToken() {
	details := `[{"type":"` + s.flightType + `","route":"CMB-SIN","maxFare":400}]`
	authReqID, executionID, step := s.startAndSignIn(details)

	purpose := detailsPurpose(s.T(), step)
	s.Require().Len(purpose.Optional, 1, "the detail is prompted")
	s.Equal(s.flightType, purpose.Optional[0].DetailType)
	s.Equal("Book CMB-SIN", purpose.Optional[0].Summary)

	token := s.approveAndPoll(authReqID, executionID, step, purpose)
	s.Require().Equal(http.StatusOK, token.status, string(token.body))
	expected := `[{"type":"` + s.flightType + `","route":"CMB-SIN","maxFare":400}]`
	s.JSONEq(expected, rawJSON(s.T(), token.details))
	assertNoGrantClaims(s.T(), token.body, "the token response")
	assertTokenDetails(s.T(), token.accessToken, expected)
}

// A CIBA request always prompts, so an earlier approval of a reusable type does not cover its details.
func (s *CIBATestSuite) TestEarlierApprovalDoesNotCoverACIBARequest() {
	value := uniqueTypeValue("ciba-reuse")
	reusable := createType(s.T(), typeRequest{Type: value, Name: "Reusable", Schema: flightSchema,
		ConsentTemplate: "Book {{route}}", Reuse: true})
	defer deleteType(s.T(), reusable.ID)

	details := `[{"type":"` + value + `","route":"CMB-SIN","maxFare":400}]`
	authReqID, executionID, step := s.startAndSignIn(details)
	token := s.approveAndPoll(authReqID, executionID, step, detailsPurpose(s.T(), step))
	s.Require().Equal(http.StatusOK, token.status, string(token.body))
	s.JSONEq(details, rawJSON(s.T(), token.details))

	_, _, step = s.startAndSignIn(details)
	s.Len(detailsPurpose(s.T(), step).Optional, 1, "the approved detail is prompted again")
}

// Invalid details are refused on the backchannel request, before the user is notified.
func (s *CIBATestSuite) TestInvalidDetailsAreRefusedAtTheBackchannelEndpoint() {
	for name, details := range map[string]string{
		"not JSON":            "not json",
		"an unknown type":     `[{"type":"` + uniqueTypeValue("unknown") + `"}]`,
		"an undeclared field": `[{"type":"` + s.flightType + `","route":"CMB-SIN","note":"x"}]`,
	} {
		status, body := s.backchannel(details)
		s.Equal(http.StatusBadRequest, status, name)
		s.Equal(errorInvalidAuthorizationDetails, oauthError(s.T(), body).Error, name)
	}
}

// startAndSignIn sends a backchannel request for the given details and signs the test user in through the
// invite. It returns the auth_req_id, the execution id and the step after sign-in.
func (s *CIBATestSuite) startAndSignIn(details string) (string, string, *testutils.FlowStep) {
	status, body := s.backchannel(details)
	s.Require().Equal(http.StatusOK, status, string(body))
	var started struct {
		AuthReqID string `json:"auth_req_id"`
	}
	s.Require().NoError(json.Unmarshal(body, &started))
	s.Require().NotEmpty(started.AuthReqID)

	executionID, inviteToken := s.invite(started.AuthReqID)
	resumed, err := testutils.ExecuteAuthenticationFlow(executionID, map[string]string{"inviteToken": inviteToken}, "")
	s.Require().NoError(err)
	step, err := testutils.ExecuteAuthenticationFlow(executionID,
		map[string]string{"username": cibaUsername, "password": cibaPassword}, "action_001", resumed.ChallengeToken)
	s.Require().NoError(err)
	return started.AuthReqID, executionID, step
}

// approveAndPoll approves every prompted detail, completes the CIBA callback and polls for the token.
func (s *CIBATestSuite) approveAndPoll(authReqID, executionID string, step *testutils.FlowStep,
	purpose consentPurposePrompt) tokenResult {
	final := decideConsent(s.T(), executionID, step.ChallengeToken, purpose, true)
	status, body := doJSON(s.T(), http.MethodPost, cibaCallbackPath,
		map[string]string{"authId": authReqID, "assertion": final.Assertion, "type": cibaGrantType})
	s.Require().Equal(http.StatusOK, status, string(body))

	form := url.Values{}
	form.Set("grant_type", cibaGrantType)
	form.Set("auth_req_id", authReqID)
	status, body = postForm(s.T(), "/oauth2/token", form, cibaClientID, cibaClientSecret)
	return readTokenResult(s.T(), status, body)
}

// backchannel sends a backchannel authentication request for the test user with the given details.
func (s *CIBATestSuite) backchannel(details string) (int, []byte) {
	form := url.Values{}
	form.Set("login_hint", cibaUsername)
	form.Set("scope", "openid")
	form.Set("resource", cibaResource)
	form.Set("authorization_details", details)
	return postForm(s.T(), cibaBackchannel, form, cibaClientID, cibaClientSecret)
}

// invite returns the execution id and invite token of the link the user was sent for the request.
func (s *CIBATestSuite) invite(authReqID string) (string, string) {
	var executionID, inviteToken string
	s.Require().Eventually(func() bool {
		msg := s.notifications.GetLastMessage()
		if msg == nil || linkParam(msg.Message, "auth_req_id") != authReqID {
			return false
		}
		executionID = linkParam(msg.Message, "executionId")
		inviteToken = linkParam(msg.Message, "inviteToken")
		return executionID != "" && inviteToken != ""
	}, 5*time.Second, 100*time.Millisecond, "expected the invite for %s", authReqID)
	return executionID, inviteToken
}

// linkParam returns a query parameter of the invite link in a notification.
func linkParam(text, param string) string {
	m := regexp.MustCompile(regexp.QuoteMeta(param) + `=([^&"\s\\]+)`).FindStringSubmatch(text)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}
