// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package projects

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	mockGooglePort       = 8093
	mockGithubPort       = 8092
	googleClientID       = "orgs_test_google_client"
	googleClientSecret   = "orgs_test_google_secret"
	githubClientID       = "orgs_test_github_client"
	githubClientSecret   = "orgs_test_github_secret"
	customerTypeHandle   = "federated_customer"
	loginFlowHandle      = "federated-orgs-login-test"
	onboardingFlowHandle = "federated-orgs-user-onboarding-test"
	googleAction         = "action_google"
	githubAction         = "action_github"
	sendInviteAction     = "action_send_invite"
	selectOUAction       = "action_select_ou"
	createOUAction       = "action_create_org"
	selectUserTypeAction = "action_select_user_type"
	selectInviteOUAction = "action_select_invite_ou"

	providerGoogle = "Google"
	providerGithub = "GitHub"
)

var (
	// loginFlow signs a user in with Google or GitHub. A user with no account creates an organization
	// and is provisioned into it, and a user with an account in several organizations picks one.
	loginFlow = testutils.Flow{
		Name:     "Federated Organizations Login Flow",
		Handle:   loginFlowHandle,
		FlowType: "AUTHENTICATION",
		Nodes: []map[string]interface{}{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "choose_auth",
			},
			{
				"id":   "choose_auth",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{"action": map[string]interface{}{"ref": googleAction, "nextNode": "google_auth"}},
					{"action": map[string]interface{}{"ref": githubAction, "nextNode": "github_auth"}},
				},
			},
			{
				"id":   "google_auth",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"idpId":                               "",
					"allowAuthenticationWithoutLocalUser": true,
				},
				"executor": map[string]interface{}{
					"name": "GoogleOIDCAuthExecutor",
				},
				"onSuccess": "disambiguate",
			},
			{
				"id":   "github_auth",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"idpId":                               "",
					"allowAuthenticationWithoutLocalUser": true,
				},
				"executor": map[string]interface{}{
					"name": "GithubOAuthExecutor",
				},
				"onSuccess": "disambiguate",
			},
			{
				"id":   "disambiguate",
				"type": "TASK_EXECUTION",
				"condition": map[string]interface{}{
					"key":    "{{ctx(entityState)}}",
					"value":  "ambiguous",
					"onSkip": "prompt_create_org",
				},
				"executor": map[string]interface{}{
					"name": "IdentifyingExecutor",
					"mode": "resolve",
				},
				"onSuccess":    "auth_assert",
				"onIncomplete": "prompt_select_org",
			},
			{
				"id":   "prompt_select_org",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{"ref": "select_ou", "identifier": "ouHandle", "type": "SELECT", "required": true},
						},
						"action": map[string]interface{}{
							"ref":      selectOUAction,
							"nextNode": "disambiguate",
						},
					},
				},
			},
			{
				"id":   "prompt_create_org",
				"type": "PROMPT",
				"condition": map[string]interface{}{
					"key":    "{{ctx(entityState)}}",
					"value":  "not_exists",
					"onSkip": "auth_assert",
				},
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{"ref": "input_ou_name", "identifier": "ouName", "type": "TEXT_INPUT", "required": true},
							{"ref": "input_ou_handle", "identifier": "ouHandle", "type": "TEXT_INPUT", "required": true},
						},
						"action": map[string]interface{}{
							"ref":      createOUAction,
							"nextNode": "create_org",
						},
					},
				},
			},
			{
				"id":   "create_org",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"parentOuId": "",
				},
				"executor": map[string]interface{}{
					"name": "OUExecutor",
					"inputs": []map[string]interface{}{
						{"ref": "input_ou_name", "identifier": "ouName", "type": "TEXT_INPUT", "required": true},
						{"ref": "input_ou_handle", "identifier": "ouHandle", "type": "TEXT_INPUT", "required": true},
					},
				},
				"onSuccess": "provisioning",
			},
			{
				"id":   "provisioning",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"allowCrossOUProvisioning": true,
				},
				"executor": map[string]interface{}{
					"name": "ProvisioningExecutor",
				},
				"onSuccess": "auth_assert",
			},
			{
				"id":   "auth_assert",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "AuthAssertExecutor",
				},
				"onSuccess": "end",
			},
			{
				"id":   "end",
				"type": "END",
			},
		},
	}

	// onboardingFlow invites a user to an organization by email. The invitee accepts by signing up
	// with Google or GitHub, and gets an account in that organization even when they already hold one
	// elsewhere.
	onboardingFlow = testutils.Flow{
		Name:     "Federated Organizations User Onboarding Flow",
		Handle:   onboardingFlowHandle,
		FlowType: "USER_ONBOARDING",
		Nodes: []map[string]interface{}{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "permission_validator",
			},
			{
				"id":   "permission_validator",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "PermissionValidator",
				},
				"onSuccess": "user_type_resolver",
			},
			{
				"id":   "user_type_resolver",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "UserTypeResolver",
				},
				"onSuccess":    "ou_resolver",
				"onIncomplete": "prompt_user_type",
			},
			{
				"id":   "prompt_user_type",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{"ref": "input_user_type", "identifier": "userType", "type": "SELECT", "required": true},
						},
						"action": map[string]interface{}{
							"ref":      selectUserTypeAction,
							"nextNode": "user_type_resolver",
						},
					},
				},
			},
			{
				"id":   "ou_resolver",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"resolveFrom": "prompt",
				},
				"executor": map[string]interface{}{
					"name": "OUResolverExecutor",
				},
				"onSuccess":    "prompt_email",
				"onIncomplete": "prompt_ou",
			},
			{
				"id":   "prompt_ou",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{"ref": "input_ou", "identifier": "ouId", "type": "OU_SELECT", "required": true},
						},
						"action": map[string]interface{}{
							"ref":      selectInviteOUAction,
							"nextNode": "ou_resolver",
						},
					},
				},
			},
			{
				"id":   "prompt_email",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{"ref": "input_email", "identifier": "email", "type": "EMAIL_INPUT", "required": true},
						},
						"action": map[string]interface{}{
							"ref":      sendInviteAction,
							"nextNode": "invite_generate",
						},
					},
				},
			},
			{
				"id":   "invite_generate",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "InviteExecutor",
					"mode": "generate",
				},
				"onSuccess": "send_invite_email",
			},
			{
				"id":   "send_invite_email",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"emailTemplate": "user-invite",
					"senderId":      "",
				},
				"executor": map[string]interface{}{
					"name": "EmailExecutor",
					"mode": "send",
					"inputs": []map[string]interface{}{
						{"ref": "input_email", "identifier": "email", "type": "EMAIL_INPUT", "required": true},
					},
				},
				"onSuccess": "invitation_sent",
			},
			{
				"id":   "invitation_sent",
				"type": "PROMPT",
				"next": "invite_verify",
			},
			{
				"id":   "invite_verify",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "InviteExecutor",
					"mode": "verify",
					"inputs": []map[string]interface{}{
						{"identifier": "inviteToken", "type": "HIDDEN", "required": true},
					},
				},
				"onSuccess": "choose_signup",
			},
			{
				"id":   "choose_signup",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{"action": map[string]interface{}{"ref": googleAction, "nextNode": "google_signup"}},
					{"action": map[string]interface{}{"ref": githubAction, "nextNode": "github_signup"}},
				},
			},
			{
				"id":   "google_signup",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"idpId":                             "",
					"allowRegistrationWithExistingUser": true,
				},
				"executor": map[string]interface{}{
					"name": "GoogleOIDCAuthExecutor",
				},
				"onSuccess": "provisioning",
			},
			{
				"id":   "github_signup",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"idpId":                             "",
					"allowRegistrationWithExistingUser": true,
				},
				"executor": map[string]interface{}{
					"name": "GithubOAuthExecutor",
				},
				"onSuccess": "provisioning",
			},
			{
				"id":   "provisioning",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"allowCrossOUProvisioning": true,
				},
				"executor": map[string]interface{}{
					"name": "ProvisioningExecutor",
				},
				"onSuccess": "invitation_accepted",
			},
			{
				"id":   "invitation_accepted",
				"type": "PROMPT",
				"next": "end",
			},
			{
				"id":   "end",
				"type": "END",
			},
		},
	}
)

// federatedUser is an identity at one of the providers. A Google and a GitHub identity of the same
// person are separate users, since nothing links them.
type federatedUser struct {
	provider string
	sub      string
	email    string
	name     string
	// login is the GitHub username the mock signs in as.
	login string
}

// scenarioUsers holds the identities each scenario signs in with at one provider, and the
// organizations they create at sign up.
type scenarioUsers struct {
	// invited is invited to two organizations.
	invited federatedUser
	// newUser signs in without an account and creates newUserOrg.
	newUser    federatedUser
	newUserOrg testutils.OrganizationUnit
	// selfSignedUp creates selfSignedUpOrg and is then invited to another organization.
	selfSignedUp    federatedUser
	selfSignedUpOrg testutils.OrganizationUnit
}

var (
	providers = []string{providerGoogle, providerGithub}

	usersByProvider = map[string]scenarioUsers{
		providerGoogle: {
			invited:      googleUser("google-invited-user", "invited@gmail.test", "Invited User"),
			newUser:      googleUser("google-new-user", "new@gmail.test", "New User"),
			newUserOrg:   testutils.OrganizationUnit{Handle: "test-google-new-user-org", Name: "Google New User Org"},
			selfSignedUp: googleUser("google-self-signed-up-user", "self-signed-up@gmail.test", "Self User"),
			selfSignedUpOrg: testutils.OrganizationUnit{
				Handle: "test-google-self-signed-up-org",
				Name:   "Google Self Signed Up Org",
			},
		},
		providerGithub: {
			invited:      githubUser(91001, "invited-octocat", "invited@github.test", "Invited User"),
			newUser:      githubUser(91002, "new-octocat", "new@github.test", "New User"),
			newUserOrg:   testutils.OrganizationUnit{Handle: "test-github-new-user-org", Name: "GitHub New User Org"},
			selfSignedUp: githubUser(91003, "self-signed-up-octocat", "self-signed-up@github.test", "Self User"),
			selfSignedUpOrg: testutils.OrganizationUnit{
				Handle: "test-github-self-signed-up-org",
				Name:   "GitHub Self Signed Up Org",
			},
		},
	}

	rootOU = testutils.OrganizationUnit{
		Handle:      "test-orgs-root",
		Name:        "Test Organizations Root",
		Description: "Parent of the organizations users sign up to or are invited to",
	}
	orgA = testutils.OrganizationUnit{
		Handle: "test-org-a",
		Name:   "Test Org A",
	}
	orgB = testutils.OrganizationUnit{
		Handle: "test-org-b",
		Name:   "Test Org B",
	}

	customerType = testutils.UserType{
		Handle:      customerTypeHandle,
		DisplayName: "Federated Customer",
		Schema: map[string]interface{}{
			"email":          map[string]interface{}{"type": "string"},
			"email_verified": map[string]interface{}{"type": "string"},
			"name":           map[string]interface{}{"type": "string"},
			"given_name":     map[string]interface{}{"type": "string"},
			"family_name":    map[string]interface{}{"type": "string"},
			"picture":        map[string]interface{}{"type": "string"},
			"locale":         map[string]interface{}{"type": "string"},
		},
	}
)

func googleUser(sub, email, name string) federatedUser {
	return federatedUser{provider: providerGoogle, sub: sub, email: email, name: name}
}

func githubUser(id int64, login, email, name string) federatedUser {
	return federatedUser{provider: providerGithub, sub: strconv.FormatInt(id, 10), email: email, name: name,
		login: login}
}

// MultiOUSocialSignupTestSuite covers a project where every user signs in with Google or GitHub and
// can belong to several organizations. An invited user gets one account per organization, and the
// login flow asks which organization to continue with when the identity matches more than one account.
type MultiOUSocialSignupTestSuite struct {
	suite.Suite
	mockGoogle   *testutils.MockGoogleOIDCServer
	mockGithub   *testutils.MockGithubOAuthServer
	mockSMTP     *testutils.MockSMTPServer
	senderID     string
	idpIDs       []string
	userTypeID   string
	loginFlowID  string
	onboardingID string
	appID        string
	createdOUIDs []string
	flowConfig   json.RawMessage
}

func TestMultiOUSocialSignupTestSuite(t *testing.T) {
	suite.Run(t, new(MultiOUSocialSignupTestSuite))
}

func (ts *MultiOUSocialSignupTestSuite) SetupSuite() {
	ts.mockSMTP = testutils.NewMockSMTPServer(0)
	ts.Require().NoError(ts.mockSMTP.Start(), "Failed to start mock SMTP server")
	time.Sleep(100 * time.Millisecond)

	senderID, err := testutils.CreateSMTPEmailProvider("Organizations Test Email Provider", "localhost",
		ts.mockSMTP.GetPort(), "noreply@thunderid.test")
	ts.Require().NoError(err, "Failed to create the SMTP email provider")
	ts.senderID = senderID

	ts.startMockProviders()

	rootOUID, err := testutils.CreateOrganizationUnit(rootOU)
	ts.Require().NoError(err, "Failed to create root organization unit")
	rootOU.ID = rootOUID

	for _, org := range []*testutils.OrganizationUnit{&orgA, &orgB} {
		org.Parent = &rootOUID
		orgID, err := testutils.CreateOrganizationUnit(*org)
		ts.Require().NoError(err, "Failed to create organization unit %s", org.Handle)
		org.ID = orgID
	}

	customerType.OUID = rootOUID
	userTypeID, err := testutils.CreateUserType(customerType)
	ts.Require().NoError(err, "Failed to create user type")
	ts.userTypeID = userTypeID

	googleIDPID := ts.createIDP("GOOGLE", googleClientID, googleClientSecret, "openid email profile")
	githubIDPID := ts.createIDP("GITHUB", githubClientID, githubClientSecret, "user:email,read:user")

	setNodeProperty(loginFlow, "google_auth", "idpId", googleIDPID)
	setNodeProperty(loginFlow, "github_auth", "idpId", githubIDPID)
	setNodeProperty(loginFlow, "create_org", "parentOuId", rootOUID)
	loginFlowID, err := testutils.CreateFlow(loginFlow)
	ts.Require().NoError(err, "Failed to create login flow")
	ts.loginFlowID = loginFlowID

	setNodeProperty(onboardingFlow, "google_signup", "idpId", googleIDPID)
	setNodeProperty(onboardingFlow, "github_signup", "idpId", githubIDPID)
	setNodeProperty(onboardingFlow, "send_invite_email", "senderId", ts.senderID)
	onboardingID, err := testutils.CreateFlow(onboardingFlow)
	ts.Require().NoError(err, "Failed to create user onboarding flow")
	ts.onboardingID = onboardingID

	flowConfig, err := testutils.MergeWritableServerConfig("flow", map[string]interface{}{
		"userOnboardingFlow": map[string]interface{}{"defaultHandle": onboardingFlowHandle},
	})
	ts.Require().NoError(err, "Failed to set the default user onboarding flow")
	ts.flowConfig = flowConfig

	appID, err := testutils.CreateApplication(testutils.Application{
		OUID:                      rootOUID,
		Name:                      "Organizations Test Application",
		Description:               "Application users of every organization sign in to",
		IsRegistrationFlowEnabled: false,
		ClientID:                  "orgs_test_client",
		ClientSecret:              "orgs_test_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{customerTypeHandle},
		AuthFlowID:                loginFlowID,
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{"userTypeHandle", "ouId", "ouName", "ouHandle"},
		},
	})
	ts.Require().NoError(err, "Failed to create application")
	ts.appID = appID
}

func (ts *MultiOUSocialSignupTestSuite) startMockProviders() {
	mockGoogle, err := testutils.NewMockGoogleOIDCServer(mockGooglePort, googleClientID, googleClientSecret)
	ts.Require().NoError(err, "Failed to create mock Google server")
	ts.mockGoogle = mockGoogle

	ts.mockGithub = testutils.NewMockGithubOAuthServer(mockGithubPort, githubClientID, githubClientSecret)

	for _, users := range usersByProvider {
		for _, user := range []federatedUser{users.invited, users.newUser, users.selfSignedUp} {
			switch user.provider {
			case providerGoogle:
				ts.mockGoogle.AddUser(&testutils.GoogleUserInfo{
					Sub:           user.sub,
					Email:         user.email,
					EmailVerified: true,
					Name:          user.name,
				})
			case providerGithub:
				id, err := strconv.ParseInt(user.sub, 10, 64)
				ts.Require().NoError(err)
				email := user.email
				ts.mockGithub.AddUser(&testutils.GithubUserInfo{
					Login: user.login,
					ID:    id,
					Email: &email,
					Name:  user.name,
					Type:  "User",
				}, []*testutils.GithubEmail{{Email: email, Primary: true, Verified: true}})
			}
		}
	}

	ts.Require().NoError(ts.mockGoogle.Start(), "Failed to start mock Google server")
	ts.Require().NoError(ts.mockGithub.Start(), "Failed to start mock GitHub server")
}

func (ts *MultiOUSocialSignupTestSuite) createIDP(idpType, clientID, clientSecret, scopes string) string {
	idpID, err := testutils.CreateIDP(testutils.IDP{
		Name:        "Organizations Test " + idpType + " IDP",
		Description: "Connection users of every organization sign in with",
		Type:        idpType,
		Properties: []testutils.IDPProperty{
			{Name: "client_id", Value: clientID},
			{Name: "client_secret", Value: clientSecret, IsSecret: true},
			{Name: "redirect_uri", Value: "http://localhost:3000/callback"},
			{Name: "scopes", Value: scopes},
		},
	})
	ts.Require().NoError(err, "Failed to create %s IDP", idpType)
	ts.idpIDs = append(ts.idpIDs, idpID)
	return idpID
}

func (ts *MultiOUSocialSignupTestSuite) TearDownSuite() {
	if ts.flowConfig != nil {
		if err := testutils.PutWritableServerConfig("flow", ts.flowConfig); err != nil {
			ts.T().Logf("teardown: failed to reset the flow server config: %v", err)
		}
	}
	if ts.appID != "" {
		if err := testutils.DeleteApplication(ts.appID); err != nil {
			ts.T().Logf("teardown: failed to delete application: %v", err)
		}
	}
	for _, flowID := range []string{ts.loginFlowID, ts.onboardingID} {
		if flowID == "" {
			continue
		}
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("teardown: failed to delete flow %s: %v", flowID, err)
		}
	}
	for _, users := range usersByProvider {
		for _, user := range []federatedUser{users.invited, users.newUser, users.selfSignedUp} {
			accounts, err := findUsersByEmail(user.email)
			if err != nil {
				ts.T().Logf("teardown: failed to find users for %s: %v", user.email, err)
				continue
			}
			for _, account := range accounts {
				if err := testutils.DeleteUser(account.ID); err != nil {
					ts.T().Logf("teardown: failed to delete user %s: %v", account.ID, err)
				}
			}
		}
	}
	if ts.userTypeID != "" {
		if err := testutils.DeleteUserType(ts.userTypeID); err != nil {
			ts.T().Logf("teardown: failed to delete user type: %v", err)
		}
	}
	ouIDs := append(ts.createdOUIDs, orgA.ID, orgB.ID, rootOU.ID)
	for _, ouID := range ouIDs {
		if ouID == "" {
			continue
		}
		if err := testutils.DeleteOrganizationUnit(ouID); err != nil {
			ts.T().Logf("teardown: failed to delete organization unit %s: %v", ouID, err)
		}
	}
	for _, idpID := range ts.idpIDs {
		if err := testutils.DeleteIDP(idpID); err != nil {
			ts.T().Logf("teardown: failed to delete IDP %s: %v", idpID, err)
		}
	}
	if ts.mockGoogle != nil {
		_ = ts.mockGoogle.Stop()
	}
	if ts.mockGithub != nil {
		_ = ts.mockGithub.Stop()
	}
	time.Sleep(200 * time.Millisecond)
	if ts.mockSMTP != nil {
		if err := ts.mockSMTP.Stop(); err != nil {
			ts.T().Logf("teardown: failed to stop mock SMTP server: %v", err)
		}
	}
	if ts.senderID != "" {
		if err := testutils.DeleteNotificationSender(ts.senderID); err != nil {
			ts.T().Logf("teardown: failed to delete the email provider: %v", err)
		}
	}
}

// TestUserInvitedToTwoOrganizationsSelectsOrganizationAtLogin invites one identity to two
// organizations, so it holds an account in each, and expects the login flow to ask which one to use.
func (ts *MultiOUSocialSignupTestSuite) TestUserInvitedToTwoOrganizationsSelectsOrganizationAtLogin() {
	for _, provider := range providers {
		user := usersByProvider[provider].invited
		ts.Run(provider, func() {
			ts.inviteUser(user, orgA)
			ts.inviteUser(user, orgB)

			accounts, err := findUsersByEmail(user.email)
			ts.Require().NoError(err)
			ts.Require().Len(accounts, 2, "Expected one account per organization")
			ts.ElementsMatch([]string{orgA.ID, orgB.ID}, []string{accounts[0].OUID, accounts[1].OUID})

			step := ts.signIn(user)
			ts.requireOrganizationPicker(step, orgA.Handle, orgB.Handle)

			step = ts.continueFlow(step, map[string]string{"ouHandle": orgB.Handle}, selectOUAction)
			ts.requireSignedInTo(step, orgB, userInOU(accounts, orgB.ID))
		})
	}
}

// TestNewUserCreatesOrganizationAtFirstLogin signs in an identity with no account. The login flow
// asks for an organization, creates it with the user, and signs the user straight in next time.
func (ts *MultiOUSocialSignupTestSuite) TestNewUserCreatesOrganizationAtFirstLogin() {
	for _, provider := range providers {
		user, newOrg := usersByProvider[provider].newUser, usersByProvider[provider].newUserOrg
		ts.Run(provider, func() {
			userID := ts.signUpWithNewOrganization(user, &newOrg)

			step := ts.signIn(user)
			ts.requireSignedInTo(step, newOrg, userID)
		})
	}
}

// TestSelfSignedUpUserInvitedToExistingOrganizationSelectsOrganizationAtLogin signs up an identity
// with its own organization, invites it to an existing one, and expects the login flow to ask which
// one to use.
func (ts *MultiOUSocialSignupTestSuite) TestSelfSignedUpUserInvitedToExistingOrganizationSelectsOrganizationAtLogin() {
	for _, provider := range providers {
		user, ownOrg := usersByProvider[provider].selfSignedUp, usersByProvider[provider].selfSignedUpOrg
		ts.Run(provider, func() {
			ts.signUpWithNewOrganization(user, &ownOrg)

			ts.inviteUser(user, orgA)

			accounts, err := findUsersByEmail(user.email)
			ts.Require().NoError(err)
			ts.Require().Len(accounts, 2, "Expected one account per organization")

			step := ts.signIn(user)
			ts.requireOrganizationPicker(step, ownOrg.Handle, orgA.Handle)

			step = ts.continueFlow(step, map[string]string{"ouHandle": orgA.Handle}, selectOUAction)
			ts.requireSignedInTo(step, orgA, userInOU(accounts, orgA.ID))
		})
	}
}

// inviteUser runs the user onboarding flow for an organization: the admin enters the invitee's
// email, the invitation is sent by email, and the invitee accepts it by signing up with their
// provider.
func (ts *MultiOUSocialSignupTestSuite) inviteUser(user federatedUser, org testutils.OrganizationUnit) {
	ts.mockSMTP.ClearEmails()

	step, err := initiateUserOnboardingFlow(map[string]string{
		"userType": customerTypeHandle,
		"ouId":     org.ID,
	})
	ts.Require().NoError(err, "Failed to initiate user onboarding flow")
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "Expected the flow to ask for the invitee: %+v",
		step.Error)
	ts.Require().True(common.HasInput(step.Data.Inputs, "email"), "Expected an email input")

	step = ts.continueFlow(step, map[string]string{"email": user.email}, sendInviteAction)
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "Expected the flow to wait for the invitee")

	time.Sleep(1 * time.Second)
	email := ts.mockSMTP.GetLastEmail()
	ts.Require().NotNil(email, "Expected an invitation email")
	ts.Require().Contains(email.To, user.email, "Expected the invitation to reach the invitee")
	inviteLink := email.ExtractRecoveryLink()
	ts.Require().NotEmpty(inviteLink, "Expected an invite link in the email")
	inviteToken := testutils.ExtractQueryParam(inviteLink, "inviteToken")
	ts.Require().NotEmpty(inviteToken, "Expected inviteToken in the invite link")
	executionID := testutils.ExtractQueryParam(inviteLink, "executionId")
	ts.Require().Equal(step.ExecutionID, executionID, "Expected the invite link to resume this execution")

	step = ts.continueFlow(step, map[string]string{"inviteToken": inviteToken}, "")
	step = ts.completeFederatedSignIn(ts.chooseProvider(step, user), user)
	ts.Require().Equal("COMPLETE", step.FlowStatus, "Expected the invitation to be accepted")
}

// signUpWithNewOrganization signs in an identity with no account, creates its organization and
// returns the provisioned user's ID.
func (ts *MultiOUSocialSignupTestSuite) signUpWithNewOrganization(user federatedUser,
	org *testutils.OrganizationUnit) string {
	step := ts.signIn(user)
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "Expected the flow to ask for an organization")
	ts.Require().True(common.HasInput(step.Data.Inputs, "ouName"), "Expected ouName input")
	ts.Require().True(common.HasInput(step.Data.Inputs, "ouHandle"), "Expected ouHandle input")

	step = ts.continueFlow(step, map[string]string{"ouName": org.Name, "ouHandle": org.Handle}, createOUAction)
	ts.Require().Equal("COMPLETE", step.FlowStatus, "Expected sign up to complete: %+v %+v", step.Error,
		step.Data.Inputs)

	claims, err := testutils.DecodeJWT(step.Assertion)
	ts.Require().NoError(err)
	ts.Require().NotEmpty(claims.OUID, "Expected the new organization in the assertion")
	org.ID = claims.OUID
	ts.createdOUIDs = append([]string{claims.OUID}, ts.createdOUIDs...)
	ts.requireSignedInTo(step, *org, "")

	accounts, err := findUsersByEmail(user.email)
	ts.Require().NoError(err)
	ts.Require().Len(accounts, 1, "Expected one account after sign up")
	ts.Require().Equal(org.ID, accounts[0].OUID, "Expected the account in the new organization")
	ts.Require().Equal(accounts[0].ID, claims.Sub, "Expected the assertion for the new account")

	return accounts[0].ID
}

// signIn starts the login flow, picks the user's provider and completes the sign-in there.
func (ts *MultiOUSocialSignupTestSuite) signIn(user federatedUser) *common.FlowStep {
	step, err := common.InitiateAuthenticationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate login flow")

	return ts.completeFederatedSignIn(ts.chooseProvider(step, user), user)
}

// chooseProvider picks the user's provider on a sign-in method prompt offering Google and GitHub.
func (ts *MultiOUSocialSignupTestSuite) chooseProvider(step *common.FlowStep, user federatedUser) *common.FlowStep {
	ts.Require().Equal("VIEW", step.Type, "Expected a prompt to choose the sign-in method")
	ts.Require().True(common.ValidateRequiredActions(step.Data.Actions, []string{googleAction, githubAction}),
		"Expected both Google and GitHub to be offered")

	action := googleAction
	if user.provider == providerGithub {
		action = githubAction
	}
	step = ts.continueFlow(step, nil, action)
	ts.Require().Equal("REDIRECTION", step.Type, "Expected a redirect to %s", user.provider)
	return step
}

func (ts *MultiOUSocialSignupTestSuite) completeFederatedSignIn(step *common.FlowStep,
	user federatedUser) *common.FlowStep {
	switch user.provider {
	case providerGoogle:
		ts.mockGoogle.SetAuthorizeFunc(func(string) (string, error) { return user.email, nil })
	case providerGithub:
		ts.mockGithub.SetAuthorizeFunc(func(string) (string, error) { return user.login, nil })
	}

	code, _, err := testutils.SimulateFederatedOAuthFlow(step.Data.RedirectURL)
	ts.Require().NoError(err, "Failed to sign in at %s", user.provider)

	// Only the code is sent back, as the SDK's OAuth callback does.
	return ts.continueFlow(step, map[string]string{"code": code}, "")
}

func (ts *MultiOUSocialSignupTestSuite) continueFlow(step *common.FlowStep, inputs map[string]string,
	action string) *common.FlowStep {
	next, err := common.CompleteFlow(step.ExecutionID, inputs, action, step.ChallengeToken)
	ts.Require().NoError(err, "Failed to continue flow")
	ts.Require().NotEqual("ERROR", next.FlowStatus, "Flow failed: %+v", next.Error)
	return next
}

func (ts *MultiOUSocialSignupTestSuite) requireOrganizationPicker(step *common.FlowStep, handles ...string) {
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "Expected the flow to ask for an organization")
	for _, input := range step.Data.Inputs {
		if input.Identifier == "ouHandle" {
			ts.Require().ElementsMatch(handles, input.Options, "Expected one option per organization")
			return
		}
	}
	ts.Require().Fail("Expected an ouHandle input to pick the organization", "inputs: %+v",
		step.Data.Inputs)
}

func (ts *MultiOUSocialSignupTestSuite) requireSignedInTo(step *common.FlowStep, org testutils.OrganizationUnit,
	userID string) {
	ts.Require().Equal("COMPLETE", step.FlowStatus, "Expected the login to complete")
	claims, err := testutils.ValidateJWTAssertionFields(step.Assertion, ts.appID, customerTypeHandle,
		org.ID, org.Name, org.Handle)
	ts.Require().NoError(err, "Unexpected assertion")
	if userID != "" {
		ts.Require().Equal(userID, claims.Sub, "Expected the assertion for the selected account")
	}
}

func userInOU(users []testutils.User, ouID string) string {
	for _, u := range users {
		if u.OUID == ouID {
			return u.ID
		}
	}
	return ""
}

// initiateUserOnboardingFlow starts the server's default user onboarding flow as the admin. The
// flow endpoint is public, so the admin token is attached here rather than by the test client.
func initiateUserOnboardingFlow(inputs map[string]string) (*common.FlowStep, error) {
	body, err := json.Marshal(map[string]interface{}{
		"flowType": "USER_ONBOARDING",
		"inputs":   inputs,
	})
	if err != nil {
		return nil, err
	}

	token, err := testutils.GetAccessToken()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+"/flow/execute", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := testutils.GetHTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("expected status 200, got %d: %s", resp.StatusCode, respBody)
	}

	var step common.FlowStep
	if err := json.Unmarshal(respBody, &step); err != nil {
		return nil, err
	}
	return &step, nil
}

// findUsersByEmail returns every user with the given email.
func findUsersByEmail(email string) ([]testutils.User, error) {
	filter := url.QueryEscape(fmt.Sprintf("email eq %q", email))
	req, err := http.NewRequest(http.MethodGet, testutils.TestServerURL+"/users?filter="+filter, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("expected status 200, got %d: %s", resp.StatusCode, respBody)
	}

	var list testutils.UserListResponse
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, err
	}
	return list.Users, nil
}

// setNodeProperty sets a property on the flow node with the given ID.
func setNodeProperty(flow testutils.Flow, nodeID, key string, value interface{}) {
	for _, node := range flow.Nodes.([]map[string]interface{}) {
		if node["id"] == nodeID {
			node["properties"].(map[string]interface{})[key] = value
		}
	}
}
