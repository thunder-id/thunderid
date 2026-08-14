// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package consent

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	consentPromptAdditionalDataKey = "consentPrompt"
	consentDecisionsInputKey       = "consent_decisions"

	// shortConsentValidityPeriod is the consent validity period configured on the short-validity
	// application, small enough that a test can wait it out.
	shortConsentValidityPeriod = 3 * time.Second

	consentTestUsername  = "consentuser"
	consentTestPassword  = "testpassword"
	consentTestGivenName = "Consent"
	consentTestEmail     = "consent@example.com"

	// The OIDC application below is driven through the authorization endpoint, which is the only
	// path that can mark an attribute essential (via the claims request parameter).
	consentOIDCClientID     = "consent_oidc_test_client"
	consentOIDCClientSecret = "consent_oidc_test_secret"
	consentOIDCRedirectURI  = "https://localhost:3000"
	consentOIDCResource     = "https://consent-oidc-test.example.com"

	// consentEssentialClaimsParam requests email as an essential claim and given_name as an optional
	// one, so a single consent prompt covers both classifications.
	consentEssentialClaimsParam = `{"id_token":{"email":{"essential":true},"given_name":null}}`

	// consentEmailOnlyClaimsParam and consentBothAttributesClaimsParam request a narrow and then a
	// wider attribute set, so a login can be made to ask about an element that is already consented
	// alongside one that is not.
	consentEmailOnlyClaimsParam      = `{"id_token":{"email":null}}`
	consentBothAttributesClaimsParam = `{"id_token":{"email":null,"given_name":null}}`

	// consentUnconfiguredAttributeClaimsParam asks for family_name, which the OIDC application does
	// not configure, alongside email, which it does.
	consentUnconfiguredAttributeClaimsParam = `{"id_token":{"email":null,"family_name":null}}`

	// consentDeniedErrorCode is the executor error returned when an essential attribute is denied.
	consentDeniedErrorCode = "FET-1066"
	// consentPromptTimedOutErrorCode is the executor error returned when a consent prompt is answered
	// after its configured timeout has elapsed.
	consentPromptTimedOutErrorCode = "FET-1069"

	// consentPromptTimeout is the prompt timeout configured on the timeout test flow's consent node,
	// short enough that a test can wait it out.
	consentPromptTimeout = 2 * time.Second

	// The permission strings below are action handles on the test resource server. "documents" is a
	// prefix of the other two followed by a permission delimiter, so a prompt covering more than one
	// of them is expected to report the rollup linkage between them.
	consentPermDocuments      = "documents"
	consentPermDocumentsRead  = "documents.read"
	consentPermDocumentsWrite = "documents.write"

	// authorizedPermissionsClaim carries the permissions the assertion releases, which the consent
	// step narrows to the ones the user approved.
	authorizedPermissionsClaim = "authorized_permissions"

	requestedPermissionsInputKey = "requested_permissions"
	resourceServerInputKey       = "resource_server_identifier"
)

var (
	consentExecutorFlow = testutils.Flow{
		Name:     "Consent Executor Flow Test",
		FlowType: "AUTHENTICATION",
		Handle:   "consent_executor_test_1",
		Nodes: []map[string]interface{}{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "credentials_auth",
			},
			{
				"id":   "credentials_auth",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "CredentialsAuthExecutor",
				},
				"onSuccess": "consent",
			},
			{
				"id":   "consent",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "ConsentExecutor",
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

	consentTestApp = testutils.Application{
		Name:             "Consent Executor Flow Test Application",
		Description:      "Application for testing the consent executor flow end-to-end",
		ClientID:         "consent_flow_test_client",
		ClientSecret:     "consent_flow_test_secret",
		RedirectURIs:     []string{"http://localhost:3000/callback"},
		AllowedUserTypes: []string{"consent_flow_user"},
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{"given_name", "email"},
		},
		// A validity period long enough that it cannot elapse during the test run, so the reuse
		// case exercises consent reuse rather than expiry.
		LoginConsent: map[string]interface{}{
			"validityPeriod": 3600,
		},
	}

	// consentShortValidityApp mirrors consentTestApp but with a consent validity period short enough
	// to elapse mid-test, so re-prompting after expiry can be exercised.
	consentShortValidityApp = testutils.Application{
		Name:             "Consent Executor Short Validity Test Application",
		Description:      "Application for testing consent expiry in the consent executor flow",
		ClientID:         "consent_short_validity_test_client",
		ClientSecret:     "consent_short_validity_test_secret",
		RedirectURIs:     []string{"http://localhost:3000/callback"},
		AllowedUserTypes: []string{"consent_flow_user"},
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{"given_name", "email"},
		},
		LoginConsent: map[string]interface{}{
			"validityPeriod": int(shortConsentValidityPeriod.Seconds()),
		},
	}

	// consentNoValidityApp mirrors consentTestApp but leaves the consent validity period unset, so its
	// consent is recorded with no expiry at all rather than with a bounded lifetime.
	consentNoValidityApp = testutils.Application{
		Name:             "Consent Executor No Validity Test Application",
		Description:      "Application for testing consent recorded without a validity period",
		ClientID:         "consent_no_validity_test_client",
		ClientSecret:     "consent_no_validity_test_secret",
		RedirectURIs:     []string{"http://localhost:3000/callback"},
		AllowedUserTypes: []string{"consent_flow_user"},
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{"given_name", "email"},
		},
	}

	// consentNoAttributesApp is embedded, so it carries no OAuth profile whose token configuration
	// could contribute attributes, and it configures no assertion attributes either. With no attribute
	// list anywhere on the application there is no attribute consent purpose to derive.
	consentNoAttributesApp = testutils.Application{
		Name:             "Consent Executor No Attributes Test Application",
		Description:      "Application for testing consent with no configured attributes",
		Embedded:         true,
		AllowedUserTypes: []string{"consent_flow_user"},
	}

	// consentProfileFilterApp configures an attribute the test user does not have a value for, so the
	// consent prompt can be shown to omit it.
	consentProfileFilterApp = testutils.Application{
		Name:             "Consent Executor Profile Filter Test Application",
		Description:      "Application for testing the consent profile presence filter",
		ClientID:         "consent_profile_filter_test_client",
		ClientSecret:     "consent_profile_filter_test_secret",
		RedirectURIs:     []string{"http://localhost:3000/callback"},
		AllowedUserTypes: []string{"consent_flow_user"},
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{"given_name", "email", "family_name"},
		},
		LoginConsent: map[string]interface{}{
			"validityPeriod": 3600,
		},
	}

	// consentTimeoutFlow mirrors consentExecutorFlow but bounds how long the consent prompt stays
	// answerable, so an abandoned prompt can be exercised.
	consentTimeoutFlow = testutils.Flow{
		Name:     "Consent Executor Timeout Test Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "consent_executor_timeout_test_1",
		Nodes: []map[string]interface{}{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "credentials_auth",
			},
			{
				"id":   "credentials_auth",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "CredentialsAuthExecutor",
				},
				"onSuccess": "consent",
			},
			{
				"id":   "consent",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "ConsentExecutor",
				},
				"properties": map[string]interface{}{
					"timeout": strconv.Itoa(int(consentPromptTimeout.Seconds())),
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

	// consentTimeoutApp mirrors consentTestApp but runs the timeout-bounded consent flow.
	consentTimeoutApp = testutils.Application{
		Name:             "Consent Executor Timeout Test Application",
		Description:      "Application for testing consent prompt timeouts",
		ClientID:         "consent_timeout_test_client",
		ClientSecret:     "consent_timeout_test_secret",
		RedirectURIs:     []string{"http://localhost:3000/callback"},
		AllowedUserTypes: []string{"consent_flow_user"},
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{"given_name", "email"},
		},
		LoginConsent: map[string]interface{}{
			"validityPeriod": 3600,
		},
	}

	// consentPermissionFlow runs the authorization executor ahead of the consent executor, which is
	// what turns the user's authorized permissions into a consent purpose. Permission consent is
	// unreachable without that node.
	consentPermissionFlow = testutils.Flow{
		Name:     "Consent Executor Permission Test Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "consent_executor_permission_test_1",
		Nodes: []map[string]interface{}{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "credentials_auth",
			},
			{
				"id":   "credentials_auth",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "CredentialsAuthExecutor",
				},
				"onSuccess": "authorization_check",
			},
			{
				"id":   "authorization_check",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "AuthorizationExecutor",
				},
				"onSuccess": "consent",
			},
			{
				"id":   "consent",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "ConsentExecutor",
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

	// consentPermissionApp configures no user attributes, so it yields no attribute consent purpose
	// at all and its prompts cover permissions alone.
	consentPermissionApp = testutils.Application{
		Name:             "Consent Executor Permission Test Application",
		Description:      "Application for testing permission consent",
		ClientID:         "consent_permission_test_client",
		ClientSecret:     "consent_permission_test_secret",
		RedirectURIs:     []string{"http://localhost:3000/callback"},
		AllowedUserTypes: []string{"consent_flow_user"},
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{},
		},
		LoginConsent: map[string]interface{}{
			"validityPeriod": 3600,
		},
	}

	// consentPermissionAttributeApp configures an attribute alongside the permissions it requests, so
	// a single consent record ends up holding both an attribute purpose and a permission purpose.
	consentPermissionAttributeApp = testutils.Application{
		Name:             "Consent Executor Permission And Attribute Test Application",
		Description:      "Application for testing consent spanning both purpose types",
		ClientID:         "consent_permission_attribute_test_client",
		ClientSecret:     "consent_permission_attribute_test_secret",
		RedirectURIs:     []string{"http://localhost:3000/callback"},
		AllowedUserTypes: []string{"consent_flow_user"},
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{"email"},
		},
		LoginConsent: map[string]interface{}{
			"validityPeriod": 3600,
		},
	}

	// consentEssentialFlow collects consent the same way consentExecutorFlow does, but prompts for
	// credentials with a PROMPT node because it is started by the authorization endpoint rather than
	// with the credentials already supplied.
	consentEssentialFlow = testutils.Flow{
		Name:     "Consent Executor Essential Attributes Test Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "consent_executor_essential_test_1",
		Nodes: []map[string]interface{}{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "prompt_credentials",
			},
			{
				"id":   "prompt_credentials",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{
								"ref":        "input_001",
								"identifier": "username",
								"type":       "TEXT_INPUT",
								"required":   true,
							},
							{
								"ref":        "input_002",
								"identifier": "password",
								"type":       "PASSWORD_INPUT",
								"required":   true,
							},
						},
						"action": map[string]interface{}{
							"ref":      "action_001",
							"nextNode": "credentials_auth",
						},
					},
				},
			},
			{
				"id":   "credentials_auth",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "CredentialsAuthExecutor",
					"inputs": []map[string]interface{}{
						{
							"ref":        "input_001",
							"identifier": "username",
							"type":       "TEXT_INPUT",
							"required":   true,
						},
						{
							"ref":        "input_002",
							"identifier": "password",
							"type":       "PASSWORD_INPUT",
							"required":   true,
						},
					},
				},
				"onSuccess": "consent",
			},
			{
				"id":   "consent",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "ConsentExecutor",
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

	// consentOIDCApp is driven through the authorization endpoint so that the claims request
	// parameter can mark an attribute essential. The assertion and ID token attribute lists are both
	// pinned to the same pair so the consent purpose covers exactly those two elements regardless of
	// any deployment level defaults.
	consentOIDCApp = testutils.Application{
		Name:             "Consent Executor OIDC Test Application",
		Description:      "Application for testing essential and optional consent attributes",
		Type:             "fullstack",
		AllowedUserTypes: []string{"consent_flow_user"},
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{"email", "given_name"},
		},
		LoginConsent: map[string]interface{}{
			"validityPeriod": 3600,
		},
		InboundAuthConfig: []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                consentOIDCClientID,
					"clientSecret":            consentOIDCClientSecret,
					"redirectUris":            []string{consentOIDCRedirectURI},
					"grantTypes":              []string{"authorization_code", "refresh_token"},
					"responseTypes":           []string{"code"},
					"tokenEndpointAuthMethod": "client_secret_basic",
					"scopes":                  []string{"openid"},
					"token": map[string]interface{}{
						// An application that leaves accessToken.userConfig unset inherits the
						// assertion attribute list there, and every access token attribute is
						// requested as optional on each authorization regardless of the claims
						// parameter. Pinning it empty keeps the claims parameter authoritative over
						// what a given login asks consent for.
						"accessToken": map[string]interface{}{
							"userConfig": map[string]interface{}{
								"attributes": []string{},
							},
						},
						"idToken": map[string]interface{}{
							"userAttributes": []string{"email", "given_name"},
						},
					},
				},
			},
		},
	}

	consentTestOU = testutils.OrganizationUnit{
		Handle:      "consent-flow-test-ou",
		Name:        "Consent Executor Flow Test Organization Unit",
		Description: "Organization unit for consent executor flow testing",
		Parent:      nil,
	}

	consentEntityType = testutils.UserType{
		Name: "consent_flow_user",
		Schema: map[string]interface{}{
			"username": map[string]interface{}{
				"type": "string",
			},
			"password": map[string]interface{}{
				"type":       "string",
				"credential": true,
			},
			"given_name": map[string]interface{}{
				"type": "string",
			},
			"email": map[string]interface{}{
				"type": "string",
			},
			// family_name is part of the schema but is deliberately left unset on the test user, so
			// an application can configure it and still have it filtered out of consent prompts.
			"family_name": map[string]interface{}{
				"type": "string",
			},
		},
	}

	consentTestUser = testutils.User{
		Type: consentEntityType.Name,
		Attributes: json.RawMessage(`{
			"username": "` + consentTestUsername + `",
			"password": "` + consentTestPassword + `",
			"given_name": "` + consentTestGivenName + `",
			"email": "` + consentTestEmail + `"
		}`),
	}

	consentTestCredentials = map[string]string{
		"username": consentTestUsername,
		"password": consentTestPassword,
	}
)

var (
	consentExecFlowAppID               string
	consentExecFlowShortValidityAppID  string
	consentExecFlowNoValidityAppID     string
	consentExecFlowNoAttributesAppID   string
	consentExecFlowProfileFilterAppID  string
	consentExecFlowTimeoutAppID        string
	consentExecFlowPermissionAppID     string
	consentExecFlowPermissionAttrAppID string
	consentExecFlowOIDCAppID           string
	consentExecFlowResourceServerID    string
	consentExecFlowOUID                string
	consentExecFlowEntityTypeID        string
	consentExecFlowUserID              string
	consentExecFlowRoleID              string
)

// ConsentExecutorFlowTestSuite validates the ConsentExecutor node end-to-end: prompting, recording
// grant/deny decisions, applying them to the auth assertion, reusing an active consent within its
// validity period, and re-prompting once that period has elapsed.
type ConsentExecutorFlowTestSuite struct {
	suite.Suite
	config *common.TestSuiteConfig
}

func TestConsentExecutorFlowTestSuite(t *testing.T) {
	suite.Run(t, new(ConsentExecutorFlowTestSuite))
}

func (ts *ConsentExecutorFlowTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}

	ouID, err := testutils.CreateOrganizationUnit(consentTestOU)
	ts.Require().NoError(err, "Failed to create test organization unit during setup")
	consentExecFlowOUID = ouID

	consentEntityType.OUID = consentExecFlowOUID
	entityTypeID, err := testutils.CreateUserType(consentEntityType)
	ts.Require().NoError(err, "Failed to create test user type during setup")
	consentExecFlowEntityTypeID = entityTypeID

	flowID, err := testutils.CreateFlow(consentExecutorFlow)
	ts.Require().NoError(err, "Failed to create consent executor flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)

	consentTestApp.AuthFlowID = flowID
	consentTestApp.OUID = consentExecFlowOUID
	appID, err := testutils.CreateApplication(consentTestApp)
	ts.Require().NoError(err, "Failed to create test application during setup")
	consentExecFlowAppID = appID

	consentShortValidityApp.AuthFlowID = flowID
	consentShortValidityApp.OUID = consentExecFlowOUID
	shortValidityAppID, err := testutils.CreateApplication(consentShortValidityApp)
	ts.Require().NoError(err, "Failed to create short validity test application during setup")
	consentExecFlowShortValidityAppID = shortValidityAppID

	consentNoValidityApp.AuthFlowID = flowID
	consentNoValidityApp.OUID = consentExecFlowOUID
	noValidityAppID, err := testutils.CreateApplication(consentNoValidityApp)
	ts.Require().NoError(err, "Failed to create no validity test application during setup")
	consentExecFlowNoValidityAppID = noValidityAppID

	consentNoAttributesApp.AuthFlowID = flowID
	consentNoAttributesApp.OUID = consentExecFlowOUID
	noAttributesAppID, err := testutils.CreateApplication(consentNoAttributesApp)
	ts.Require().NoError(err, "Failed to create no attributes test application during setup")
	consentExecFlowNoAttributesAppID = noAttributesAppID

	consentProfileFilterApp.AuthFlowID = flowID
	consentProfileFilterApp.OUID = consentExecFlowOUID
	profileFilterAppID, err := testutils.CreateApplication(consentProfileFilterApp)
	ts.Require().NoError(err, "Failed to create profile filter test application during setup")
	consentExecFlowProfileFilterAppID = profileFilterAppID

	timeoutFlowID, err := testutils.CreateFlow(consentTimeoutFlow)
	ts.Require().NoError(err, "Failed to create consent timeout flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, timeoutFlowID)

	consentTimeoutApp.AuthFlowID = timeoutFlowID
	consentTimeoutApp.OUID = consentExecFlowOUID
	timeoutAppID, err := testutils.CreateApplication(consentTimeoutApp)
	ts.Require().NoError(err, "Failed to create timeout test application during setup")
	consentExecFlowTimeoutAppID = timeoutAppID

	// The OIDC application below needs a resource server so its access tokens can be requested for a
	// concrete audience. Its actions double as the permissions the authorization executor evaluates,
	// which in turn become the elements of the permission consent purpose.
	resourceServerID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:        "Consent Executor Test Resource Server",
		Description: "Resource server for consent executor OIDC and permission tests",
		Identifier:  consentOIDCResource,
		OUID:        consentExecFlowOUID,
	}, []testutils.Action{
		{Name: "Documents", Handle: consentPermDocuments, Description: "Access documents"},
		{Name: "Read Documents", Handle: consentPermDocumentsRead, Description: "Read documents"},
		{Name: "Write Documents", Handle: consentPermDocumentsWrite, Description: "Write documents"},
	})
	ts.Require().NoError(err, "Failed to create test resource server during setup")
	consentExecFlowResourceServerID = resourceServerID

	permissionFlowID, err := testutils.CreateFlow(consentPermissionFlow)
	ts.Require().NoError(err, "Failed to create consent permission flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, permissionFlowID)

	consentPermissionApp.AuthFlowID = permissionFlowID
	consentPermissionApp.OUID = consentExecFlowOUID
	permissionAppID, err := testutils.CreateApplication(consentPermissionApp)
	ts.Require().NoError(err, "Failed to create permission test application during setup")
	consentExecFlowPermissionAppID = permissionAppID

	consentPermissionAttributeApp.AuthFlowID = permissionFlowID
	consentPermissionAttributeApp.OUID = consentExecFlowOUID
	permissionAttrAppID, err := testutils.CreateApplication(consentPermissionAttributeApp)
	ts.Require().NoError(err, "Failed to create permission and attribute test application during setup")
	consentExecFlowPermissionAttrAppID = permissionAttrAppID

	essentialFlowID, err := testutils.CreateFlow(consentEssentialFlow)
	ts.Require().NoError(err, "Failed to create consent essential attributes flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, essentialFlowID)

	consentOIDCApp.AuthFlowID = essentialFlowID
	consentOIDCApp.OUID = consentExecFlowOUID
	oidcAppID, err := testutils.CreateApplication(consentOIDCApp)
	ts.Require().NoError(err, "Failed to create OIDC test application during setup")
	consentExecFlowOIDCAppID = oidcAppID
}

// SetupTest recreates the test user before each test. Consent records are keyed by user ID, so a
// freshly created user guarantees a clean consent state and keeps the tests order independent.
func (ts *ConsentExecutorFlowTestSuite) SetupTest() {
	user := consentTestUser
	user.OUID = consentExecFlowOUID

	userID, err := testutils.CreateUser(user)
	ts.Require().NoError(err, "Failed to create test user before test")
	consentExecFlowUserID = userID
}

// TearDownTest deletes the test user, discarding the consent it accumulated during the test, along
// with any role a permission test granted it. The role goes first because it is assigned to the user.
func (ts *ConsentExecutorFlowTestSuite) TearDownTest() {
	if consentExecFlowRoleID != "" {
		if err := testutils.DeleteRole(consentExecFlowRoleID); err != nil {
			ts.T().Logf("Failed to delete test role after test: %v", err)
		}
		consentExecFlowRoleID = ""
	}

	if consentExecFlowUserID == "" {
		return
	}
	if err := testutils.DeleteUser(consentExecFlowUserID); err != nil {
		ts.T().Logf("Failed to delete test user after test: %v", err)
	}
	consentExecFlowUserID = ""
}

func (ts *ConsentExecutorFlowTestSuite) TearDownSuite() {
	for _, appID := range []string{
		consentExecFlowAppID, consentExecFlowShortValidityAppID,
		consentExecFlowNoValidityAppID, consentExecFlowNoAttributesAppID,
		consentExecFlowProfileFilterAppID, consentExecFlowTimeoutAppID,
		consentExecFlowPermissionAppID, consentExecFlowPermissionAttrAppID,
		consentExecFlowOIDCAppID,
	} {
		if appID == "" {
			continue
		}
		if err := testutils.DeleteApplication(appID); err != nil {
			ts.T().Logf("Failed to delete test application during teardown: %v", err)
		}
	}

	if consentExecFlowResourceServerID != "" {
		// A resource server cannot be removed while it still owns actions.
		actionIDs, err := testutils.GetActionsByResourceServer(consentExecFlowResourceServerID)
		if err != nil {
			ts.T().Logf("Failed to list test resource server actions during teardown: %v", err)
		}
		for _, actionID := range actionIDs {
			if err := testutils.DeleteAction(consentExecFlowResourceServerID, actionID); err != nil {
				ts.T().Logf("Failed to delete test action during teardown: %v", err)
			}
		}

		if err := testutils.DeleteResourceServer(consentExecFlowResourceServerID); err != nil {
			ts.T().Logf("Failed to delete test resource server during teardown: %v", err)
		}
	}

	if consentExecFlowOUID != "" {
		if err := testutils.DeleteOrganizationUnit(consentExecFlowOUID); err != nil {
			ts.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}

	if consentExecFlowEntityTypeID != "" {
		if err := testutils.DeleteUserType(consentExecFlowEntityTypeID); err != nil {
			ts.T().Logf("Failed to delete test user type during teardown: %v", err)
		}
	}

	for _, flowID := range ts.config.CreatedFlowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete test flow during teardown: %v", err)
		}
	}
}

// TestConsentGranted covers the grant path: the user is prompted for the configured attributes,
// grants them, and the granted attributes appear in the resulting assertion.
func (ts *ConsentExecutorFlowTestSuite) TestConsentGranted() {
	consentStep, purpose := ts.loginExpectingConsentPrompt(consentExecFlowAppID)
	ts.Require().Empty(purpose.Essential, "Assertion-derived attributes are prompted as optional")
	ts.Require().ElementsMatch([]string{"given_name", "email"}, promptElementNames(purpose.Optional))

	finalStep := ts.submitConsentDecisions(consentStep, purpose, true)
	ts.Require().Equal("COMPLETE", finalStep.FlowStatus, "Expected flow to complete after granting consent")
	ts.Require().NotEmpty(finalStep.Assertion, "Expected an assertion after granting consent")

	claims, err := testutils.DecodeJWT(finalStep.Assertion)
	ts.Require().NoError(err, "Failed to decode JWT assertion")
	ts.Require().Equal(consentTestGivenName, claims.Additional["given_name"],
		"Granted attribute should be present in the assertion")
	ts.Require().Equal(consentTestEmail, claims.Additional["email"],
		"Granted attribute should be present in the assertion")
}

// TestConsentDenied covers the deny path: the user is prompted, denies the optional attributes,
// the flow still completes (nothing essential was denied), the denial is recorded, and the denied
// attributes are excluded from the resulting assertion.
func (ts *ConsentExecutorFlowTestSuite) TestConsentDenied() {
	consentStep, purpose := ts.loginExpectingConsentPrompt(consentExecFlowAppID)

	finalStep := ts.submitConsentDecisions(consentStep, purpose, false)
	ts.Require().Equal("COMPLETE", finalStep.FlowStatus,
		"Expected flow to complete since only optional (non-essential) attributes were denied")
	ts.Require().NotEmpty(finalStep.Assertion, "Expected an assertion even when consent is denied")

	claims, err := testutils.DecodeJWT(finalStep.Assertion)
	ts.Require().NoError(err, "Failed to decode JWT assertion")
	_, hasGivenName := claims.Additional["given_name"]
	_, hasEmail := claims.Additional["email"]
	ts.Require().False(hasGivenName, "Denied attribute should be excluded from the assertion")
	ts.Require().False(hasEmail, "Denied attribute should be excluded from the assertion")
}

// TestConsentReusedWithinValidityPeriod covers consent reuse: once consent is granted, a second
// login within the configured validity period completes without prompting again.
func (ts *ConsentExecutorFlowTestSuite) TestConsentReusedWithinValidityPeriod() {
	// The first login prompts for consent and grants it.
	consentStep, purpose := ts.loginExpectingConsentPrompt(consentExecFlowAppID)
	grantedStep := ts.submitConsentDecisions(consentStep, purpose, true)
	ts.Require().Equal("COMPLETE", grantedStep.FlowStatus, "Expected flow to complete after granting consent")

	// The second login should reuse the active consent and complete without prompting again.
	finalStep, err := common.InitiateAuthenticationFlow(consentExecFlowAppID, false, consentTestCredentials, "")
	ts.Require().NoError(err, "Failed to initiate second authentication flow")
	ts.Require().Equal("COMPLETE", finalStep.FlowStatus,
		"Expected the active consent to be reused without a second prompt")
	ts.Require().NotEmpty(finalStep.Assertion, "Expected an assertion on the second login")

	claims, err := testutils.DecodeJWT(finalStep.Assertion)
	ts.Require().NoError(err, "Failed to decode JWT assertion")
	ts.Require().Equal(consentTestGivenName, claims.Additional["given_name"],
		"Reused consent should still yield the granted attributes")
	ts.Require().Equal(consentTestEmail, claims.Additional["email"],
		"Reused consent should still yield the granted attributes")
}

// TestConsentWithoutValidityPeriodNeverExpires covers an application that configures no consent
// validity period: the consent is recorded with no expiry rather than with a bounded lifetime, and it
// stays active for later logins instead of being treated as already expired.
func (ts *ConsentExecutorFlowTestSuite) TestConsentWithoutValidityPeriodNeverExpires() {
	consentStep, purpose := ts.loginExpectingConsentPrompt(consentExecFlowNoValidityAppID)
	grantedStep := ts.submitConsentDecisions(consentStep, purpose, true)
	ts.Require().Equal("COMPLETE", grantedStep.FlowStatus, "Expected flow to complete after granting consent")

	// A consent with no validity period has no point at which it lapses, so the next login reuses it.
	finalStep, err := common.InitiateAuthenticationFlow(
		consentExecFlowNoValidityAppID, false, consentTestCredentials, "")
	ts.Require().NoError(err, "Failed to initiate second authentication flow")
	ts.Require().Equal("COMPLETE", finalStep.FlowStatus,
		"Expected a consent with no validity period to stay active without a second prompt")
	ts.Require().NotEmpty(finalStep.Assertion, "Expected an assertion on the second login")

	claims, err := testutils.DecodeJWT(finalStep.Assertion)
	ts.Require().NoError(err, "Failed to decode JWT assertion")
	ts.Require().Equal(consentTestGivenName, claims.Additional["given_name"],
		"Reused consent should still yield the granted attributes")
	ts.Require().Equal(consentTestEmail, claims.Additional["email"],
		"Reused consent should still yield the granted attributes")
}

// TestApplicationWithNoAttributesRaisesNoConsentPrompt covers an application that configures no user
// attributes anywhere: there is no attribute consent purpose to derive from it, so the consent
// executor has nothing to ask about and the login completes without a prompt.
func (ts *ConsentExecutorFlowTestSuite) TestApplicationWithNoAttributesRaisesNoConsentPrompt() {
	finalStep, err := common.InitiateAuthenticationFlow(
		consentExecFlowNoAttributesAppID, false, consentTestCredentials, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")
	ts.Require().Equal("COMPLETE", finalStep.FlowStatus,
		"Expected the login to complete without a consent prompt")
	ts.Require().NotEmpty(finalStep.Assertion, "Expected an assertion on the completed flow")

	ts.Require().NotContains(finalStep.Data.AdditionalData, consentPromptAdditionalDataKey,
		"Expected no consent prompt for an application with no configured attributes")

	claims, err := testutils.DecodeJWT(finalStep.Assertion)
	ts.Require().NoError(err, "Failed to decode JWT assertion")
	ts.Require().NotContains(claims.Additional, "given_name",
		"An application configuring no attributes should release none")
	ts.Require().NotContains(claims.Additional, "email",
		"An application configuring no attributes should release none")
}

// TestConsentRePromptedAfterValidityPeriodExpires covers consent expiry: a granted consent is reused
// only while it is active, and once its validity period has elapsed the user is prompted again.
func (ts *ConsentExecutorFlowTestSuite) TestConsentRePromptedAfterValidityPeriodExpires() {
	// The first login prompts for consent and grants it.
	// An application with a short consent validity period is used so the test can wait it out
	consentStep, purpose := ts.loginExpectingConsentPrompt(consentExecFlowShortValidityAppID)
	grantedStep := ts.submitConsentDecisions(consentStep, purpose, true)
	ts.Require().Equal("COMPLETE", grantedStep.FlowStatus, "Expected flow to complete after granting consent")

	// Wait out the application's consent validity period. Consent search derives each record's
	// effective status from its validity time, so the record stops matching the active filter and
	// the executor has to prompt again.
	time.Sleep(shortConsentValidityPeriod + time.Second)

	// The third login should re-prompt for consent because the previous grant has expired
	repromptStep, repromptPurpose := ts.loginExpectingConsentPrompt(consentExecFlowShortValidityAppID)
	ts.Require().Empty(repromptPurpose.Essential, "Assertion-derived attributes are prompted as optional")
	ts.Require().ElementsMatch([]string{"given_name", "email"}, promptElementNames(repromptPurpose.Optional),
		"Expected the expired consent to be prompted again in full")
	// The expired record does not block a fresh grant.
	finalStep := ts.submitConsentDecisions(repromptStep, repromptPurpose, true)
	ts.Require().Equal("COMPLETE", finalStep.FlowStatus, "Expected flow to complete after re-granting consent")
	ts.Require().NotEmpty(finalStep.Assertion, "Expected an assertion after re-granting consent")
	claims, err := testutils.DecodeJWT(finalStep.Assertion)
	ts.Require().NoError(err, "Failed to decode JWT assertion")
	ts.Require().Equal(consentTestGivenName, claims.Additional["given_name"],
		"Re-granted consent should yield the granted attributes")
	ts.Require().Equal(consentTestEmail, claims.Additional["email"],
		"Re-granted consent should yield the granted attributes")
}

// TestEssentialGrantedAndOptionalDenied covers granting every essential attribute while denying every
// optional one. Nothing essential is denied, so the flow completes and only the essential attribute
// is released.
func (ts *ConsentExecutorFlowTestSuite) TestEssentialGrantedAndOptionalDenied() {
	authID, executionID, consentStep, purpose := ts.authorizeExpectingConsentPrompt(consentEssentialClaimsParam)
	ts.Require().ElementsMatch([]string{"email"}, promptElementNames(purpose.Essential),
		"Expected the claim requested as essential to be prompted as essential")
	ts.Require().ElementsMatch([]string{"given_name"}, promptElementNames(purpose.Optional),
		"Expected the claim requested without the essential marker to be prompted as optional")

	finalStep := ts.submitOIDCConsentDecisions(executionID, consentStep.ChallengeToken, purpose, true, false)
	ts.Require().Equal("COMPLETE", finalStep.FlowStatus,
		"Expected flow to complete when every essential attribute is granted")
	ts.Require().NotEmpty(finalStep.Assertion, "Expected an assertion after granting essential consent")

	idTokenClaims := ts.exchangeAssertionForIDTokenClaims(authID, finalStep.Assertion)
	ts.Require().Equal(consentTestEmail, idTokenClaims["email"],
		"Granted essential attribute should be present in the ID token")
	ts.Require().NotContains(idTokenClaims, "given_name",
		"Denied optional attribute should be absent from the ID token")
}

// TestEssentialDeniedAndOptionalGranted covers denying an essential attribute while granting the
// optional ones. Denying anything essential fails the flow, even though the decision is still
// recorded beforehand for audit purposes.
func (ts *ConsentExecutorFlowTestSuite) TestEssentialDeniedAndOptionalGranted() {
	_, executionID, consentStep, purpose := ts.authorizeExpectingConsentPrompt(consentEssentialClaimsParam)
	ts.Require().NotEmpty(purpose.Essential, "Expected at least one essential attribute to be prompted")
	ts.Require().ElementsMatch([]string{"email"}, promptElementNames(purpose.Essential),
		"Expected the claim requested as essential to be prompted as essential")
	ts.Require().ElementsMatch([]string{"given_name"}, promptElementNames(purpose.Optional),
		"Expected the claim requested without the essential marker to be prompted as optional")

	finalStep := ts.submitOIDCConsentDecisions(executionID, consentStep.ChallengeToken, purpose, false, true)
	ts.Require().Equal("ERROR", finalStep.FlowStatus,
		"Expected the flow to fail when an essential attribute is denied")
	ts.Require().NotNil(finalStep.Error, "Expected an error on the failed flow step")
	ts.Require().Equal(consentDeniedErrorCode, finalStep.Error.Code, "Expected the consent denied executor error")
	ts.Require().Empty(finalStep.Assertion, "No assertion should be issued when essential consent is denied")
}

// TestPartialConsentSkipsAlreadyConsentedElements covers per-element consent reuse: an element that
// already carries active consent is not prompted again, so a later login asking for a wider set of
// attributes only prompts for the ones still outstanding. The new decision is merged into the
// existing record, so both the earlier and the newly consented attribute end up released.
func (ts *ConsentExecutorFlowTestSuite) TestPartialConsentSkipsAlreadyConsentedElements() {
	// The first login asks for email only, and the user grants it.
	_, executionID, consentStep, purpose := ts.authorizeExpectingConsentPrompt(consentEmailOnlyClaimsParam)
	ts.Require().Empty(purpose.Essential, "Claims requested without the essential marker are optional")
	ts.Require().ElementsMatch([]string{"email"}, promptElementNames(purpose.Optional),
		"Expected only the requested attribute to be prompted")

	grantedStep := ts.submitOIDCConsentDecisions(executionID, consentStep.ChallengeToken, purpose, true, true)
	ts.Require().Equal("COMPLETE", grantedStep.FlowStatus, "Expected flow to complete after granting consent")

	// The second login asks for email and given_name. Email already has active consent, so only
	// given_name is outstanding and only it is prompted.
	authID, secondExecutionID, secondConsentStep, secondPurpose :=
		ts.authorizeExpectingConsentPrompt(consentBothAttributesClaimsParam)
	ts.Require().Empty(secondPurpose.Essential, "Claims requested without the essential marker are optional")
	ts.Require().ElementsMatch([]string{"given_name"}, promptElementNames(secondPurpose.Optional),
		"Expected the already consented element to be skipped and only given_name to be prompted")

	finalStep := ts.submitOIDCConsentDecisions(secondExecutionID, secondConsentStep.ChallengeToken,
		secondPurpose, true, true)
	ts.Require().Equal("COMPLETE", finalStep.FlowStatus, "Expected flow to complete after granting the remainder")
	ts.Require().NotEmpty(finalStep.Assertion, "Expected an assertion after granting the remainder")

	idTokenClaims := ts.exchangeAssertionForIDTokenClaims(authID, finalStep.Assertion)
	ts.Require().Equal(consentTestEmail, idTokenClaims["email"],
		"The attribute consented in the first login should still be released")
	ts.Require().Equal(consentTestGivenName, idTokenClaims["given_name"],
		"The attribute consented in the second login should be released")
}

// TestAttributeAbsentFromProfileIsNotPrompted covers the profile presence filter: an attribute the
// application configures is only prompted when the user actually has a value for it, so nothing is
// collected that could never be released.
func (ts *ConsentExecutorFlowTestSuite) TestAttributeAbsentFromProfileIsNotPrompted() {
	consentStep, purpose := ts.loginExpectingConsentPrompt(consentExecFlowProfileFilterAppID)

	promptedNames := promptElementNames(purpose.Optional)
	ts.Require().ElementsMatch([]string{"given_name", "email"}, promptedNames,
		"Expected only the attributes the user has values for to be prompted")
	ts.Require().NotContains(promptedNames, "family_name",
		"Expected the attribute the user has no value for to be filtered out of the prompt")

	// Granting everything that was prompted still leaves the absent attribute unreleased.
	finalStep := ts.submitConsentDecisions(consentStep, purpose, true)
	ts.Require().Equal("COMPLETE", finalStep.FlowStatus, "Expected flow to complete after granting consent")

	claims, err := testutils.DecodeJWT(finalStep.Assertion)
	ts.Require().NoError(err, "Failed to decode JWT assertion")
	ts.Require().NotContains(claims.Additional, "family_name",
		"An attribute that was never prompted should not be released")
}

// TestAttributeOutsideConfiguredPurposeIsNotPrompted covers the requested attribute filter: a claim
// the authorization request asks for is only prompted when it falls within the attributes the
// application is configured to release, so a request cannot widen the consent purpose.
func (ts *ConsentExecutorFlowTestSuite) TestAttributeOutsideConfiguredPurposeIsNotPrompted() {
	authID, executionID, consentStep, purpose := ts.authorizeExpectingConsentPrompt(
		consentUnconfiguredAttributeClaimsParam)

	promptedNames := append(promptElementNames(purpose.Essential), promptElementNames(purpose.Optional)...)
	ts.Require().ElementsMatch([]string{"email"}, promptedNames,
		"Expected only the requested attribute that the application configures to be prompted")
	ts.Require().NotContains(promptedNames, "family_name",
		"Expected the requested attribute outside the configured purpose to be absent from the prompt")

	finalStep := ts.submitOIDCConsentDecisions(executionID, consentStep.ChallengeToken, purpose, true, true)
	ts.Require().Equal("COMPLETE", finalStep.FlowStatus, "Expected flow to complete after granting consent")

	idTokenClaims := ts.exchangeAssertionForIDTokenClaims(authID, finalStep.Assertion)
	ts.Require().Equal(consentTestEmail, idTokenClaims["email"],
		"The configured attribute that was granted should be released")
	ts.Require().NotContains(idTokenClaims, "family_name",
		"An attribute outside the configured purpose should never be released")
}

// TestPromptConsentForcesRePrompt covers forced re-prompting: prompt=consent makes the executor ask
// again for every requested attribute, bypassing the active consent that would otherwise be reused.
func (ts *ConsentExecutorFlowTestSuite) TestPromptConsentForcesRePrompt() {
	// Establish active consent for the requested attribute.
	_, executionID, consentStep, purpose := ts.authorizeExpectingConsentPrompt(consentEmailOnlyClaimsParam)
	ts.Require().ElementsMatch([]string{"email"}, promptElementNames(purpose.Optional),
		"Expected the requested attribute to be prompted on the first login")

	grantedStep := ts.submitOIDCConsentDecisions(executionID, consentStep.ChallengeToken, purpose, true, true)
	ts.Require().Equal("COMPLETE", grantedStep.FlowStatus, "Expected flow to complete after granting consent")

	// Without prompt=consent the active consent is reused and no prompt is raised, so reaching a
	// consent prompt at all is what this asserts.
	authID, repromptExecutionID, repromptStep, repromptPurpose := ts.authorizeExpectingConsentPromptWithPrompt(
		consentEmailOnlyClaimsParam, "consent")
	ts.Require().ElementsMatch([]string{"email"}, promptElementNames(repromptPurpose.Optional),
		"Expected prompt=consent to re-prompt the already consented attribute")

	finalStep := ts.submitOIDCConsentDecisions(repromptExecutionID, repromptStep.ChallengeToken,
		repromptPurpose, true, true)
	ts.Require().Equal("COMPLETE", finalStep.FlowStatus, "Expected flow to complete after re-granting consent")

	idTokenClaims := ts.exchangeAssertionForIDTokenClaims(authID, finalStep.Assertion)
	ts.Require().Equal(consentTestEmail, idTokenClaims["email"],
		"The re-granted attribute should be released")
}

// TestRecordedGrantIsHonoredOnNextAuthorize covers consent reuse over the authorization endpoint: the
// grant recorded by a first authorization request is honored by the next one, which completes without
// raising a consent prompt and still releases the consented attribute on the ID token.
func (ts *ConsentExecutorFlowTestSuite) TestRecordedGrantIsHonoredOnNextAuthorize() {
	// The first authorization request prompts for the attribute and records the grant.
	_, executionID, consentStep, purpose := ts.authorizeExpectingConsentPrompt(consentEmailOnlyClaimsParam)
	ts.Require().ElementsMatch([]string{"email"}, promptElementNames(purpose.Optional),
		"Expected the requested attribute to be prompted on the first authorization request")

	grantedStep := ts.submitOIDCConsentDecisions(executionID, consentStep.ChallengeToken, purpose, true, true)
	ts.Require().Equal("COMPLETE", grantedStep.FlowStatus, "Expected flow to complete after granting consent")

	// A second authorization request for the same attribute reuses the recorded grant.
	authID, finalStep := ts.authorizeExpectingNoConsentPrompt(consentEmailOnlyClaimsParam)

	idTokenClaims := ts.exchangeAssertionForIDTokenClaims(authID, finalStep.Assertion)
	ts.Require().Equal(consentTestEmail, idTokenClaims["email"],
		"The attribute consented on the first authorization request should be released on the second")
}

// TestPromptNoneIsRejectedWithLoginRequired covers the prompt=none error path: the server holds no
// server-side session, so an authorization request that forbids user interaction cannot be satisfied
// and the client is redirected back with `login_required`.
func (ts *ConsentExecutorFlowTestSuite) TestPromptNoneIsRejectedWithLoginRequired() {
	const promptNoneState = "consent_prompt_none_state"

	authzResp, err := testutils.InitiateAuthorizationFlowWithClaimsAndPrompt(consentOIDCClientID,
		consentOIDCRedirectURI, "code", "openid", promptNoneState, consentEmailOnlyClaimsParam, "none")
	ts.Require().NoError(err, "Failed to initiate authorization request")
	defer ts.closeBody(authzResp.Body)

	ts.Require().Equal(http.StatusFound, authzResp.StatusCode,
		"Expected the authorization request to be redirected")

	location := authzResp.Header.Get("Location")
	ts.Require().NotEmpty(location, "Expected a Location header on the authorization response")

	redirect, err := url.Parse(location)
	ts.Require().NoError(err, "Failed to parse the authorization redirect")

	// Both client_id and redirect_uri were valid, so the error belongs to the client rather than to a
	// server error page.
	clientRedirect, err := url.Parse(consentOIDCRedirectURI)
	ts.Require().NoError(err, "Failed to parse the client redirect URI")
	ts.Require().Equal(clientRedirect.Host, redirect.Host,
		"Expected the error to be delivered to the client redirect URI")

	params := redirect.Query()
	ts.Require().Equal("login_required", params.Get("error"),
		"Expected prompt=none to be rejected with login_required")
	ts.Require().NotEmpty(params.Get("error_description"), "Expected an error description for the client")
	ts.Require().Equal(promptNoneState, params.Get("state"),
		"Expected the state to be echoed so the client can correlate the error")
	ts.Require().Empty(params.Get("code"), "No authorization code should be issued for a rejected request")
}

// TestConsentIsScopedPerApplication covers consent scoping: consent purposes are grouped by
// application, so consent granted to one application does not suppress the prompt for another even
// for the same user.
func (ts *ConsentExecutorFlowTestSuite) TestConsentIsScopedPerApplication() {
	consentStep, purpose := ts.loginExpectingConsentPrompt(consentExecFlowAppID)
	grantedStep := ts.submitConsentDecisions(consentStep, purpose, true)
	ts.Require().Equal("COMPLETE", grantedStep.FlowStatus,
		"Expected flow to complete after granting consent to the first application")

	// A second application requesting the same attributes for the same user still prompts.
	otherConsentStep, otherPurpose := ts.loginExpectingConsentPrompt(consentExecFlowShortValidityAppID)
	ts.Require().ElementsMatch([]string{"given_name", "email"}, promptElementNames(otherPurpose.Optional),
		"Expected the second application to prompt despite consent existing for the first")
	ts.Require().NotEqual(purpose.PurposeName, otherPurpose.PurposeName,
		"Expected each application to have its own consent purpose")

	finalStep := ts.submitConsentDecisions(otherConsentStep, otherPurpose, true)
	ts.Require().Equal("COMPLETE", finalStep.FlowStatus,
		"Expected flow to complete after granting consent to the second application")
}

// TestPermissionConsentGranted covers the grant path for permissions: the permissions the user is
// authorized for are prompted as a permission purpose, the prompt reports how they roll up into one
// another, and the ones the user approves are the ones the assertion releases.
func (ts *ConsentExecutorFlowTestSuite) TestPermissionConsentGranted() {
	ts.grantTestUserPermissions()

	requested := []string{consentPermDocuments, consentPermDocumentsRead, consentPermDocumentsWrite}
	consentStep, purpose := ts.loginExpectingPermissionPrompt(consentExecFlowPermissionAppID, requested)

	ts.Require().Empty(purpose.Essential, "Permissions are always prompted as optional")
	ts.Require().ElementsMatch(requested, promptElementNames(purpose.Optional),
		"Expected every authorized permission to be prompted")

	// A permission is a rollup child of the longest other prompted permission it extends, so the
	// consent UI can present the narrower permissions underneath the one that encompasses them.
	parent, ok := promptElementParent(purpose.Optional, consentPermDocumentsRead)
	ts.Require().True(ok, "Expected %q to be prompted", consentPermDocumentsRead)
	ts.Require().Equal(consentPermDocuments, parent, "Expected the narrower permission to roll up")
	parent, ok = promptElementParent(purpose.Optional, consentPermDocumentsWrite)
	ts.Require().True(ok, "Expected %q to be prompted", consentPermDocumentsWrite)
	ts.Require().Equal(consentPermDocuments, parent, "Expected the narrower permission to roll up")
	parent, ok = promptElementParent(purpose.Optional, consentPermDocuments)
	ts.Require().True(ok, "Expected %q to be prompted", consentPermDocuments)
	ts.Require().Empty(parent, "The broadest prompted permission has no rollup parent")

	finalStep := ts.submitConsentDecisions(consentStep, purpose, true)
	ts.Require().Equal("COMPLETE", finalStep.FlowStatus, "Expected flow to complete after granting consent")
	ts.Require().NotEmpty(finalStep.Assertion, "Expected an assertion after granting consent")

	ts.Require().ElementsMatch(requested, ts.assertionPermissions(finalStep.Assertion),
		"Granted permissions should be released in the assertion")
}

// TestPermissionConsentDenied covers the deny path for permissions: the flow still completes because
// no permission is essential, but an application the user refuses gets no permissions at all.
func (ts *ConsentExecutorFlowTestSuite) TestPermissionConsentDenied() {
	ts.grantTestUserPermissions()

	requested := []string{consentPermDocuments, consentPermDocumentsRead}
	consentStep, purpose := ts.loginExpectingPermissionPrompt(consentExecFlowPermissionAppID, requested)

	finalStep := ts.submitConsentDecisions(consentStep, purpose, false)
	ts.Require().Equal("COMPLETE", finalStep.FlowStatus,
		"Expected flow to complete since permissions are never essential")
	ts.Require().NotEmpty(finalStep.Assertion, "Expected an assertion even when consent is denied")

	ts.Require().Empty(ts.assertionPermissions(finalStep.Assertion),
		"Denied permissions should be withheld from the assertion")
}

// TestAlreadyConsentedPermissionIsNotPromptedAgain covers per-element consent reuse for permissions:
// widening the permissions an application asks for only prompts for the addition, and the earlier
// grant is carried through to the assertion alongside it.
func (ts *ConsentExecutorFlowTestSuite) TestAlreadyConsentedPermissionIsNotPromptedAgain() {
	ts.grantTestUserPermissions()

	// The first login asks for two permissions and the user grants both.
	firstRequested := []string{consentPermDocuments, consentPermDocumentsRead}
	consentStep, purpose := ts.loginExpectingPermissionPrompt(consentExecFlowPermissionAppID, firstRequested)
	ts.Require().ElementsMatch(firstRequested, promptElementNames(purpose.Optional),
		"Expected both requested permissions to be prompted on the first login")

	grantedStep := ts.submitConsentDecisions(consentStep, purpose, true)
	ts.Require().Equal("COMPLETE", grantedStep.FlowStatus, "Expected flow to complete after granting consent")

	// The second login widens the request. Only the addition is outstanding, so only it is prompted.
	secondRequested := append(append([]string{}, firstRequested...), consentPermDocumentsWrite)
	secondStep, secondPurpose := ts.loginExpectingPermissionPrompt(
		consentExecFlowPermissionAppID, secondRequested)
	ts.Require().ElementsMatch([]string{consentPermDocumentsWrite}, promptElementNames(secondPurpose.Optional),
		"Expected the already consented permissions to be skipped")

	// The rollup parent is computed over the prompted set alone, so a permission whose broader
	// counterpart was not prompted stands on its own.
	parent, ok := promptElementParent(secondPurpose.Optional, consentPermDocumentsWrite)
	ts.Require().True(ok, "Expected %q to be prompted", consentPermDocumentsWrite)
	ts.Require().Empty(parent, "Expected no rollup parent when the broader permission is not prompted")

	finalStep := ts.submitConsentDecisions(secondStep, secondPurpose, true)
	ts.Require().Equal("COMPLETE", finalStep.FlowStatus, "Expected flow to complete after granting the remainder")

	ts.Require().ElementsMatch(secondRequested, ts.assertionPermissions(finalStep.Assertion),
		"Both the earlier and the newly granted permissions should be released")
}

// TestPermissionAndAttributeConsentAreMerged covers consent spanning both purpose types: a login that
// only prompts for permissions must not discard the attribute consent an earlier login recorded, so
// the user keeps what they already agreed to instead of being asked for it again.
func (ts *ConsentExecutorFlowTestSuite) TestPermissionAndAttributeConsentAreMerged() {
	ts.grantTestUserPermissions()

	// The first login requests no permissions, so only the configured attribute is prompted.
	attributeStep, attributePurpose := ts.loginExpectingConsentPrompt(consentExecFlowPermissionAttrAppID)
	ts.Require().ElementsMatch([]string{"email"}, promptElementNames(attributePurpose.Optional),
		"Expected the configured attribute to be prompted")

	grantedStep := ts.submitConsentDecisions(attributeStep, attributePurpose, true)
	ts.Require().Equal("COMPLETE", grantedStep.FlowStatus, "Expected flow to complete after granting consent")

	// The second login requests permissions. The attribute already carries active consent, so the
	// prompt covers the permission purpose alone.
	requested := []string{consentPermDocuments}
	permissionStep, permissionPurpose := ts.loginExpectingPermissionPrompt(
		consentExecFlowPermissionAttrAppID, requested)

	promptedPurposes, err := parsePromptPurposes(
		permissionStep.Data.AdditionalData[consentPromptAdditionalDataKey])
	ts.Require().NoError(err, "Failed to parse the consent prompt data")
	ts.Require().Len(promptedPurposes, 1, "Expected the already consented attribute purpose to be skipped")

	finalStep := ts.submitConsentDecisions(permissionStep, permissionPurpose, true)
	ts.Require().Equal("COMPLETE", finalStep.FlowStatus, "Expected flow to complete after granting consent")
	ts.Require().NotEmpty(finalStep.Assertion, "Expected an assertion after granting consent")

	claims, err := testutils.DecodeJWT(finalStep.Assertion)
	ts.Require().NoError(err, "Failed to decode JWT assertion")
	ts.Require().Equal(consentTestEmail, claims.Additional["email"],
		"The attribute consented in the first login should survive the permission consent")
	ts.Require().ElementsMatch(requested, ts.assertionPermissions(finalStep.Assertion),
		"The permission consented in the second login should be released")
}

// TestConsentSubmittedAfterTimeoutIsRejected covers an abandoned consent prompt: once the configured
// timeout has elapsed the decisions are no longer accepted, so a stale screen cannot be submitted
// later and no consent is recorded from it.
func (ts *ConsentExecutorFlowTestSuite) TestConsentSubmittedAfterTimeoutIsRejected() {
	consentStep, purpose := ts.loginExpectingConsentPrompt(consentExecFlowTimeoutAppID)

	time.Sleep(consentPromptTimeout + time.Second)

	finalStep := ts.submitConsentDecisions(consentStep, purpose, true)
	ts.Require().Equal("ERROR", finalStep.FlowStatus,
		"Expected the flow to fail when the consent prompt has timed out")
	ts.Require().NotNil(finalStep.Error, "Expected an error on the failed flow step")
	ts.Require().Equal(consentPromptTimedOutErrorCode, finalStep.Error.Code,
		"Expected the consent prompt timed out executor error")
	ts.Require().Empty(finalStep.Assertion, "No assertion should be issued after the prompt times out")

	// Nothing was recorded, so the next login is prompted for the same attributes again.
	repromptStep, repromptPurpose := ts.loginExpectingConsentPrompt(consentExecFlowTimeoutAppID)
	ts.Require().ElementsMatch([]string{"given_name", "email"}, promptElementNames(repromptPurpose.Optional),
		"Expected a timed out prompt to leave no consent behind")
	ts.Require().Equal("COMPLETE", ts.submitConsentDecisions(repromptStep, repromptPurpose, true).FlowStatus,
		"Expected the re-raised prompt to be answerable")
}

// TestMissingPurposeDecisionIsTreatedAsDenied covers an incomplete submission: a prompted purpose the
// response leaves out is recorded as denied rather than accepted, so a client that drops part of the
// prompt cannot have consent inferred on the user's behalf.
func (ts *ConsentExecutorFlowTestSuite) TestMissingPurposeDecisionIsTreatedAsDenied() {
	consentStep, _ := ts.loginExpectingConsentPrompt(consentExecFlowAppID)

	decisionsJSON, err := marshalConsentDecisions(consentDecisions{
		Approved: true,
		Purposes: []consentPurposeDecision{},
	})
	ts.Require().NoError(err, "Failed to build consent decisions payload")

	finalStep, err := common.CompleteFlow(consentStep.ExecutionID,
		map[string]string{consentDecisionsInputKey: decisionsJSON}, "", consentStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit consent decisions")
	ts.Require().Equal("COMPLETE", finalStep.FlowStatus,
		"Expected the flow to complete since only optional attributes went unanswered")
	ts.Require().NotEmpty(finalStep.Assertion, "Expected an assertion on the completed flow")

	claims, err := testutils.DecodeJWT(finalStep.Assertion)
	ts.Require().NoError(err, "Failed to decode JWT assertion")
	ts.Require().NotContains(claims.Additional, "given_name",
		"An unanswered purpose should be treated as denied")
	ts.Require().NotContains(claims.Additional, "email",
		"An unanswered purpose should be treated as denied")

	// The denial was recorded, so a second login does not silently reuse it as a grant.
	repromptStep, repromptPurpose := ts.loginExpectingConsentPrompt(consentExecFlowAppID)
	ts.Require().ElementsMatch([]string{"given_name", "email"}, promptElementNames(repromptPurpose.Optional),
		"Expected the denied attributes to be prompted again")
	ts.Require().Equal("COMPLETE", ts.submitConsentDecisions(repromptStep, repromptPurpose, true).FlowStatus,
		"Expected the re-raised prompt to be answerable")
}

// --- helpers ---

// loginExpectingConsentPrompt authenticates the test user against the given application and requires
// that the flow stops at a consent prompt, returning that step and its attribute consent purpose.
func (ts *ConsentExecutorFlowTestSuite) loginExpectingConsentPrompt(appID string) (
	*common.FlowStep, consentPurposePrompt) {
	consentStep, err := common.InitiateAuthenticationFlow(appID, false, consentTestCredentials, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")
	ts.Require().Equal("INCOMPLETE", consentStep.FlowStatus, "Expected a consent prompt to be shown")
	ts.Require().Equal("VIEW", consentStep.Type, "Expected flow type to be VIEW")
	ts.Require().True(common.ValidateRequiredInputs(consentStep.Data.Inputs, []string{consentDecisionsInputKey}),
		"Expected "+consentDecisionsInputKey+" input to be requested")

	purpose, err := requireAttributePurpose(consentStep.Data.AdditionalData)
	ts.Require().NoError(err, "Failed to extract the attribute consent purpose")

	return consentStep, purpose
}

// grantTestUserPermissions assigns the test user a role holding every test permission, so the
// authorization executor has something to authorize and the consent executor something to prompt
// for. The role is removed with the user in TearDownTest.
func (ts *ConsentExecutorFlowTestSuite) grantTestUserPermissions() {
	roleID, err := testutils.CreateRole(testutils.Role{
		Name:        "Consent Executor Permission Test Role",
		Description: "Grants the consent executor test permissions to the test user",
		OUID:        consentExecFlowOUID,
		Permissions: []testutils.ResourcePermissions{
			{
				ResourceServerID: consentExecFlowResourceServerID,
				Permissions: []string{
					consentPermDocuments, consentPermDocumentsRead, consentPermDocumentsWrite,
				},
			},
		},
		Assignments: []testutils.Assignment{
			{ID: consentExecFlowUserID, Type: "user"},
		},
	})
	ts.Require().NoError(err, "Failed to grant the test user its permissions")
	consentExecFlowRoleID = roleID
}

// loginExpectingPermissionPrompt authenticates the test user against the given application while
// asking for the given permissions, and requires that the flow stops at a consent prompt carrying a
// permission purpose.
func (ts *ConsentExecutorFlowTestSuite) loginExpectingPermissionPrompt(
	appID string, requestedPermissions []string) (*common.FlowStep, consentPurposePrompt) {
	inputs := map[string]string{
		requestedPermissionsInputKey: strings.Join(requestedPermissions, " "),
		resourceServerInputKey:       consentOIDCResource,
	}
	for key, value := range consentTestCredentials {
		inputs[key] = value
	}

	consentStep, err := common.InitiateAuthenticationFlow(appID, false, inputs, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")
	ts.Require().Equal("INCOMPLETE", consentStep.FlowStatus, "Expected a consent prompt to be shown")
	ts.Require().True(common.ValidateRequiredInputs(consentStep.Data.Inputs, []string{consentDecisionsInputKey}),
		"Expected "+consentDecisionsInputKey+" input to be requested")

	rawPrompt, ok := consentStep.Data.AdditionalData[consentPromptAdditionalDataKey]
	ts.Require().True(ok, "Expected %q in the flow additional data", consentPromptAdditionalDataKey)

	purpose, err := parsePermissionPurpose(rawPrompt)
	ts.Require().NoError(err, "Failed to extract the permission consent purpose")

	return consentStep, purpose
}

// assertionPermissions returns the permissions the assertion released, which is the consented set
// intersected with the set the user is authorized for.
func (ts *ConsentExecutorFlowTestSuite) assertionPermissions(assertion string) []string {
	claims, err := testutils.DecodeJWT(assertion)
	ts.Require().NoError(err, "Failed to decode JWT assertion")

	raw, ok := claims.Additional[authorizedPermissionsClaim]
	if !ok {
		return nil
	}

	permissions, ok := raw.(string)
	ts.Require().True(ok, "Expected %q to be a string claim", authorizedPermissionsClaim)

	return strings.Fields(permissions)
}

// submitConsentDecisions answers a consent prompt, approving or denying every prompted element.
func (ts *ConsentExecutorFlowTestSuite) submitConsentDecisions(consentStep *common.FlowStep,
	purpose consentPurposePrompt, approve bool) *common.FlowStep {
	decisionsJSON, err := buildConsentDecisionsInput(purpose, approve)
	ts.Require().NoError(err, "Failed to build consent decisions payload")

	nextStep, err := common.CompleteFlow(consentStep.ExecutionID,
		map[string]string{consentDecisionsInputKey: decisionsJSON}, "", consentStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit consent decisions")

	return nextStep
}

// authorizeExpectingConsentPrompt starts an authorization request for the attributes named in the
// given claims request parameter, authenticates the test user, and requires that the flow stops at a
// consent prompt. It returns the authorization id, the flow execution id, the consent step, and its
// attribute consent purpose.
func (ts *ConsentExecutorFlowTestSuite) authorizeExpectingConsentPrompt(claimsParam string) (
	string, string, *testutils.FlowStep, consentPurposePrompt) {
	return ts.authorizeExpectingConsentPromptWithPrompt(claimsParam, "")
}

// authorizeExpectingConsentPromptWithPrompt is authorizeExpectingConsentPrompt with control over the
// OAuth prompt parameter, so forced re-prompting can be exercised.
func (ts *ConsentExecutorFlowTestSuite) authorizeExpectingConsentPromptWithPrompt(claimsParam, prompt string) (
	string, string, *testutils.FlowStep, consentPurposePrompt) {
	authzResp, err := testutils.InitiateAuthorizationFlowWithClaimsAndPrompt(consentOIDCClientID,
		consentOIDCRedirectURI, "code", "openid", "consent_test_state", claimsParam, prompt)
	ts.Require().NoError(err, "Failed to initiate authorization request")
	defer ts.closeBody(authzResp.Body)

	location := authzResp.Header.Get("Location")
	ts.Require().NotEmpty(location, "Expected a Location header on the authorization response")

	authID, executionID, err := testutils.ExtractAuthData(location)
	ts.Require().NoError(err, "Failed to extract authorization data from the redirect")

	initialStep, err := testutils.ExecuteAuthenticationFlow(executionID, nil, "")
	ts.Require().NoError(err, "Failed to initiate the authentication flow")

	consentStep, err := testutils.ExecuteAuthenticationFlow(
		executionID, consentTestCredentials, "action_001", initialStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to authenticate with credentials")
	ts.Require().Equal("INCOMPLETE", consentStep.FlowStatus, "Expected a consent prompt to be shown")
	ts.Require().NotNil(consentStep.Data, "Expected flow data on the consent prompt step")

	rawPrompt, ok := consentStep.Data.AdditionalData[consentPromptAdditionalDataKey].(string)
	ts.Require().True(ok, "Expected %q in the flow additional data", consentPromptAdditionalDataKey)

	purpose, err := parseAttributePurpose(rawPrompt)
	ts.Require().NoError(err, "Failed to extract the attribute consent purpose")

	return authID, executionID, consentStep, purpose
}

// authorizeExpectingNoConsentPrompt starts an authorization request for the attributes named in the
// given claims request parameter and authenticates the test user, requiring that the flow completes
// without stopping at a consent prompt. It returns the authorization id and the completed step.
func (ts *ConsentExecutorFlowTestSuite) authorizeExpectingNoConsentPrompt(claimsParam string) (
	string, *testutils.FlowStep) {
	authzResp, err := testutils.InitiateAuthorizationFlowWithClaims(consentOIDCClientID,
		consentOIDCRedirectURI, "code", "openid", "consent_test_state", claimsParam)
	ts.Require().NoError(err, "Failed to initiate authorization request")
	defer ts.closeBody(authzResp.Body)

	location := authzResp.Header.Get("Location")
	ts.Require().NotEmpty(location, "Expected a Location header on the authorization response")

	authID, executionID, err := testutils.ExtractAuthData(location)
	ts.Require().NoError(err, "Failed to extract authorization data from the redirect")

	initialStep, err := testutils.ExecuteAuthenticationFlow(executionID, nil, "")
	ts.Require().NoError(err, "Failed to initiate the authentication flow")

	finalStep, err := testutils.ExecuteAuthenticationFlow(
		executionID, consentTestCredentials, "action_001", initialStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to authenticate with credentials")
	ts.Require().Equal("COMPLETE", finalStep.FlowStatus,
		"Expected the recorded grant to be reused without raising a consent prompt")
	ts.Require().NotEmpty(finalStep.Assertion, "Expected an assertion on the completed flow")
	if finalStep.Data != nil {
		ts.Require().NotContains(finalStep.Data.AdditionalData, consentPromptAdditionalDataKey,
			"Expected no consent prompt when an active grant already covers the request")
	}

	return authID, finalStep
}

// submitOIDCConsentDecisions answers a consent prompt raised by the OIDC application, deciding the
// essential and optional elements separately.
func (ts *ConsentExecutorFlowTestSuite) submitOIDCConsentDecisions(executionID, challengeToken string,
	purpose consentPurposePrompt, approveEssential, approveOptional bool) *testutils.FlowStep {
	decisionsJSON, err := buildConsentDecisionsInputFor(purpose, approveEssential, approveOptional)
	ts.Require().NoError(err, "Failed to build consent decisions payload")

	nextStep, err := testutils.ExecuteAuthenticationFlow(executionID,
		map[string]string{consentDecisionsInputKey: decisionsJSON}, "", challengeToken)
	ts.Require().NoError(err, "Failed to submit consent decisions")

	return nextStep
}

// exchangeAssertionForIDTokenClaims completes the authorization with the flow assertion and trades
// the resulting code for tokens, returning the decoded ID token claims. Attributes released by
// consent are asserted on the ID token because an authorization endpoint flow caches user attributes
// rather than embedding them in the flow assertion.
func (ts *ConsentExecutorFlowTestSuite) exchangeAssertionForIDTokenClaims(
	authID, assertion string) map[string]interface{} {
	authzResponse, err := testutils.CompleteAuthorization(authID, assertion)
	ts.Require().NoError(err, "Failed to complete the authorization")

	code, err := testutils.ExtractAuthorizationCode(authzResponse.RedirectURI)
	ts.Require().NoError(err, "Failed to extract the authorization code")

	tokenResult, err := testutils.RequestTokenWithResource(consentOIDCClientID, consentOIDCClientSecret,
		code, consentOIDCRedirectURI, "authorization_code", consentOIDCResource)
	ts.Require().NoError(err, "Failed to request tokens")
	ts.Require().Equal(http.StatusOK, tokenResult.StatusCode,
		"Token request failed: %s", string(tokenResult.Body))
	ts.Require().NotNil(tokenResult.Token, "Expected a token response")
	ts.Require().NotEmpty(tokenResult.Token.IDToken, "Expected an ID token")

	claims, err := testutils.DecodeJWT(tokenResult.Token.IDToken)
	ts.Require().NoError(err, "Failed to decode the ID token")

	return claims.Additional
}

// requireAttributePurpose decodes the consent prompt data forwarded via flow additional data and
// returns the single "attributes" purpose it must contain for this test's flow/application setup.
func requireAttributePurpose(additionalData map[string]string) (consentPurposePrompt, error) {
	raw, ok := additionalData[consentPromptAdditionalDataKey]
	if !ok || raw == "" {
		return consentPurposePrompt{}, fmt.Errorf("%q missing from flow additional data", consentPromptAdditionalDataKey)
	}

	return parseAttributePurpose(raw)
}

func (ts *ConsentExecutorFlowTestSuite) closeBody(body io.ReadCloser) {
	if body == nil {
		return
	}
	if err := body.Close(); err != nil {
		ts.T().Logf("Failed to close response body: %v", err)
	}
}
