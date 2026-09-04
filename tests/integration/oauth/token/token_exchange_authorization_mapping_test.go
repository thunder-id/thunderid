// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/*
Token exchange AuthorizationMapping integration tests.

These exercise the connection's authorizationMappings section on the RFC 8693 token exchange path: a
mock external OIDC issuer mints a subject_token carrying claims, the exchange resolves the issuer's
connection, applies its authorization mappings, and the exchanged access token's granted scopes are
read back. Unlike the federated-login path (tests/integration/federated/authorization_mapping_test.go),
token exchange never resolves a local entity for the subject token's sub, so every scenario here is
implicitly a "no local record" scenario; TestTokenExchange_NoLocalRecordRequired makes that explicit.
*/
package token

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	teAuthzMockOIDCPort = 8102
	teAuthzExternalCID  = "te-authz-mapping-external-client"
	teAuthzExternalCSec = "te-authz-mapping-external-secret"
	teAuthzRedirectURI  = "https://localhost:3000/callback"
	teAuthzClientID     = "te_authz_mapping_client"
	teAuthzClientSecret = "te_authz_mapping_secret"

	teAuthzRSIdentifier      = "https://te-authz-mapping.example.com"
	teAuthzOtherRSIdentifier = "https://te-authz-mapping-other.example.com"
)

// TokenExchangeAuthorizationMappingTestSuite runs its own mock external OIDC issuer and its own
// resource servers/roles/group, independent of TokenExchangeTestSuite, so its connection's
// authorizationMappings can be rewritten per scenario without disturbing other token exchange tests.
type TokenExchangeAuthorizationMappingTestSuite struct {
	suite.Suite
	client *http.Client

	mockIDP *testutils.MockOIDCServer

	ouID                  string
	exchangeAppID         string
	resourceServerID      string
	otherResourceServerID string
	mappedRoleID          string
	mappedGroupID         string
	groupRoleID           string
	idpID                 string
	subCounter            int
	activeSub             string
}

func TestTokenExchangeAuthorizationMappingTestSuite(t *testing.T) {
	suite.Run(t, new(TokenExchangeAuthorizationMappingTestSuite))
}

func (ts *TokenExchangeAuthorizationMappingTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "te-authz-mapping-ou",
		Name:        "Token Exchange Authorization Mapping OU",
		Description: "Organization unit for token exchange authorization mapping integration tests",
	})
	ts.Require().NoError(err)
	ts.ouID = ouID

	rsID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:       "Token Exchange Authorization Mapping API",
		Identifier: teAuthzRSIdentifier,
		OUID:       ouID,
	}, []testutils.Action{
		{Name: "Read", Handle: "read", Description: "Read access"},
		{Name: "Write", Handle: "write", Description: "Write access"},
	})
	ts.Require().NoError(err)
	ts.resourceServerID = rsID

	// A second resource server sharing the "read" handle, so a mapped permission target scoped to the
	// first can be proven not to leak into an evaluation against this one.
	otherRSID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:       "Token Exchange Authorization Mapping Other API",
		Identifier: teAuthzOtherRSIdentifier,
		OUID:       ouID,
	}, []testutils.Action{
		{Name: "Read", Handle: "read", Description: "Read access"},
	})
	ts.Require().NoError(err)
	ts.otherResourceServerID = otherRSID

	// No assignments: reachable only by a mapping naming it directly.
	mappedRoleID, err := testutils.CreateRole(testutils.Role{
		Name: "Token Exchange Mapped Reader",
		OUID: ouID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: rsID, Permissions: []string{"read"}},
		},
	})
	ts.Require().NoError(err)
	ts.mappedRoleID = mappedRoleID

	groupID, err := testutils.CreateGroup(testutils.Group{Name: "Token Exchange Mapped Writers", OUID: ouID})
	ts.Require().NoError(err)
	ts.mappedGroupID = groupID

	groupRoleID, err := testutils.CreateRole(testutils.Role{
		Name: "Token Exchange Group Writer",
		OUID: ouID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: rsID, Permissions: []string{"write"}},
		},
		Assignments: []testutils.Assignment{{ID: groupID, Type: "group"}},
	})
	ts.Require().NoError(err)
	ts.groupRoleID = groupRoleID

	mockIDP, err := testutils.NewMockOIDCServer(teAuthzMockOIDCPort, teAuthzExternalCID, teAuthzExternalCSec)
	ts.Require().NoError(err)
	ts.mockIDP = mockIDP
	ts.mockIDP.SetAuthorizeFunc(func(string) (string, error) {
		if ts.activeSub == "" {
			return "", fmt.Errorf("no active subject selected by the test")
		}
		return ts.activeSub, nil
	})
	ts.Require().NoError(ts.mockIDP.Start())

	idpID, err := testutils.CreateIDP(testutils.IDP{
		Name:        "Token Exchange Authorization Mapping IdP",
		Description: "Mock external issuer for token exchange authorization mapping tests",
		Type:        "OIDC",
		Properties: []testutils.IDPProperty{
			{Name: "issuer", Value: ts.mockIDP.GetURL()},
			{Name: "jwks_endpoint", Value: ts.mockIDP.GetJWKSURL()},
			{Name: "token_exchange_enabled", Value: "true"},
			{Name: "trusted_token_audience", Value: teAuthzExternalCID},
		},
	})
	ts.Require().NoError(err)
	ts.idpID = idpID

	ts.exchangeAppID = ts.createExchangeApplication()
}

func (ts *TokenExchangeAuthorizationMappingTestSuite) TearDownSuite() {
	if ts.exchangeAppID != "" {
		_ = testutils.DeleteApplication(ts.exchangeAppID)
	}
	if ts.idpID != "" {
		_ = testutils.DeleteIDP(ts.idpID)
	}
	for _, roleID := range []string{ts.mappedRoleID, ts.groupRoleID} {
		if roleID != "" {
			_ = testutils.DeleteRole(roleID)
		}
	}
	if ts.mappedGroupID != "" {
		_ = testutils.DeleteGroup(ts.mappedGroupID)
	}
	for _, rsID := range []string{ts.resourceServerID, ts.otherResourceServerID} {
		if rsID != "" {
			_ = testutils.DeleteResourceServer(rsID)
		}
	}
	if ts.ouID != "" {
		_ = testutils.DeleteOrganizationUnit(ts.ouID)
	}
	if ts.mockIDP != nil {
		_ = ts.mockIDP.Stop()
	}
}

// applyAuthorizationMapping replaces the connection's authorizationMappings for one claim. values is
// a claim-value-to-targets map, converted into the equivalent equals rules (sorted by key, so the
// resulting configuration is deterministic) since every scenario in this file is an exact-match case.
func (ts *TokenExchangeAuthorizationMappingTestSuite) applyAuthorizationMapping(
	claim, delimiter string, values map[string][]testutils.AuthorizationTarget,
) {
	ts.T().Helper()
	current, err := testutils.GetIDP("oidc", ts.idpID)
	ts.Require().NoError(err)
	ts.Require().NotNil(current)
	current.AttributeConfiguration = &testutils.AttributeConfiguration{
		AuthorizationMappings: []testutils.AuthorizationMapping{
			{Claim: claim, Delimiter: delimiter, Values: equalsRules(values)},
		},
	}
	ts.Require().NoError(testutils.UpdateIDP(ts.idpID, *current))
}

// applyAuthorizationMappingRules replaces the connection's authorizationMappings for one claim from
// explicit rules, for scenarios that need an operator other than equals (applyAuthorizationMapping
// only ever builds equals rules).
func (ts *TokenExchangeAuthorizationMappingTestSuite) applyAuthorizationMappingRules(
	claim, valueType string, rules []testutils.AuthorizationRule,
) {
	ts.applyAuthorizationMappingRulesAndDelimiter(claim, valueType, "", rules)
}

// applyAuthorizationMappingRulesAndDelimiter is applyAuthorizationMappingRules with an explicit
// delimiter, for a multi-valued string claim (a delimiter is only meaningful for the string value
// type).
func (ts *TokenExchangeAuthorizationMappingTestSuite) applyAuthorizationMappingRulesAndDelimiter(
	claim, valueType, delimiter string, rules []testutils.AuthorizationRule,
) {
	ts.T().Helper()
	current, err := testutils.GetIDP("oidc", ts.idpID)
	ts.Require().NoError(err)
	ts.Require().NotNil(current)
	current.AttributeConfiguration = &testutils.AttributeConfiguration{
		AuthorizationMappings: []testutils.AuthorizationMapping{
			{Claim: claim, ValueType: valueType, Delimiter: delimiter, Values: rules},
		},
	}
	ts.Require().NoError(testutils.UpdateIDP(ts.idpID, *current))
}

// equalsRules converts a claim-value-to-targets map into the equivalent equals rules, one per key,
// sorted by key for a deterministic result.
func equalsRules(values map[string][]testutils.AuthorizationTarget) []testutils.AuthorizationRule {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	rules := make([]testutils.AuthorizationRule, 0, len(keys))
	for _, key := range keys {
		rules = append(rules, testutils.AuthorizationRule{
			Operator: testutils.AuthorizationOperatorEquals,
			Value:    key,
			Targets:  values[key],
		})
	}
	return rules
}

// createExchangeApplication creates the application the exchange grant is authenticated as.
func (ts *TokenExchangeAuthorizationMappingTestSuite) createExchangeApplication() string {
	app := map[string]interface{}{
		"name":                      "TokenExchangeAuthzMappingApp",
		"description":               "Application for token exchange authorization mapping tests",
		"ouId":                      ts.ouID,
		"type":                      "fullstack",
		"isRegistrationFlowEnabled": false,
		"inboundAuthConfig": []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                teAuthzClientID,
					"clientSecret":            teAuthzClientSecret,
					"grantTypes":              []string{"urn:ietf:params:oauth:grant-type:token-exchange"},
					"tokenEndpointAuthMethod": "client_secret_basic",
				},
			},
		},
	}
	jsonData, err := json.Marshal(app)
	ts.Require().NoError(err)

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/applications", strings.NewReader(string(jsonData)))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	ts.Require().Equal(http.StatusCreated, resp.StatusCode, string(bodyBytes))

	var respData map[string]interface{}
	ts.Require().NoError(json.Unmarshal(bodyBytes, &respData))
	return respData["id"].(string)
}

// nextSubject returns a subject unique to this test run.
func (ts *TokenExchangeAuthorizationMappingTestSuite) nextSubject() string {
	ts.subCounter++
	return fmt.Sprintf("te-authz-sub-%d-%d", ts.subCounter, time.Now().UnixNano())
}

// mintExternalIDToken drives the mock IdP's authorize+token endpoints and returns the raw id_token,
// which carries whatever claims were registered for the subject via AddUser (including Custom).
func (ts *TokenExchangeAuthorizationMappingTestSuite) mintExternalIDToken(sub string) string {
	ts.T().Helper()
	ts.activeSub = sub

	authURL, err := url.Parse(ts.mockIDP.GetAuthorizeURL())
	ts.Require().NoError(err)
	query := authURL.Query()
	query.Set("client_id", teAuthzExternalCID)
	query.Set("redirect_uri", teAuthzRedirectURI)
	query.Set("response_type", "code")
	query.Set("scope", "openid")
	query.Set("state", "te-authz-mapping-state")
	authURL.RawQuery = query.Encode()

	noRedirectClient := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}
	authResp, err := noRedirectClient.Get(authURL.String())
	ts.Require().NoError(err)
	defer authResp.Body.Close()
	ts.Require().Equal(http.StatusFound, authResp.StatusCode)

	location := authResp.Header.Get("Location")
	ts.Require().NotEmpty(location)
	redirectedURL, err := url.Parse(location)
	ts.Require().NoError(err)
	code := redirectedURL.Query().Get("code")
	ts.Require().NotEmpty(code)

	formData := url.Values{}
	formData.Set("grant_type", "authorization_code")
	formData.Set("code", code)
	formData.Set("client_id", teAuthzExternalCID)
	formData.Set("client_secret", teAuthzExternalCSec)
	formData.Set("redirect_uri", teAuthzRedirectURI)

	tokenResp, err := http.PostForm(ts.mockIDP.GetTokenURL(), formData)
	ts.Require().NoError(err)
	defer tokenResp.Body.Close()
	ts.Require().Equal(http.StatusOK, tokenResp.StatusCode)

	var tokenData map[string]interface{}
	ts.Require().NoError(json.NewDecoder(tokenResp.Body).Decode(&tokenData))
	idToken, ok := tokenData["id_token"].(string)
	ts.Require().True(ok)
	ts.Require().NotEmpty(idToken)
	return idToken
}

type teAuthzTokenResponse struct {
	AccessToken      string `json:"access_token,omitempty"`
	Scope            string `json:"scope,omitempty"`
	Error            string `json:"error,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`
}

// exchangeToken submits a token-exchange grant request for the external ID token, requesting the
// given scope and resource.
func (ts *TokenExchangeAuthorizationMappingTestSuite) exchangeToken(idToken, scope, resource string) (
	*teAuthzTokenResponse, int) {
	ts.T().Helper()

	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	formData.Set("subject_token", idToken)
	formData.Set("subject_token_type", "urn:ietf:params:oauth:token-type:jwt")
	if scope != "" {
		formData.Set("scope", scope)
	}
	if resource != "" {
		formData.Set("resource", resource)
	}

	req, err := http.NewRequest(
		"POST", testutils.TestServerURL+"/oauth2/token", strings.NewReader(formData.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(teAuthzClientID, teAuthzClientSecret)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	var tokenResp teAuthzTokenResponse
	ts.Require().NoError(json.Unmarshal(bodyBytes, &tokenResp), "body: %s", string(bodyBytes))
	return &tokenResp, resp.StatusCode
}

// TestTokenExchange_MappedRoleGrantsScopeBeyondNativeScope covers a federated subject token whose own
// scope claim carries nothing ThunderID recognizes, but whose issuer resolved a mapped role from the
// token's claims. The role has no local assignees, so only the mapping resolves it.
func (ts *TokenExchangeAuthorizationMappingTestSuite) TestTokenExchange_MappedRoleGrantsScopeBeyondNativeScope() {
	sub := ts.nextSubject()
	ts.mockIDP.AddUser(&testutils.OIDCUserInfo{
		Sub:    sub,
		Custom: map[string]interface{}{"groups": []interface{}{"te-admins"}},
	})
	ts.applyAuthorizationMapping("groups", "", map[string][]testutils.AuthorizationTarget{
		"te-admins": {{Type: testutils.AuthorizationTargetRole, ID: ts.mappedRoleID}},
	})

	idToken := ts.mintExternalIDToken(sub)
	resp, status := ts.exchangeToken(idToken, "read", teAuthzRSIdentifier)
	ts.Require().Equal(http.StatusOK, status, "error=%s description=%s", resp.Error, resp.ErrorDescription)
	ts.Require().NotEmpty(resp.AccessToken)

	claims, err := testutils.DecodeJWT(resp.AccessToken)
	ts.Require().NoError(err)
	ts.Equal(sub, claims.Sub, "the exchanged token's subject should be the raw external subject")
	ts.Contains(strings.Fields(resp.Scope), "read")
}

// TestTokenExchange_MappedGroupGrantsScope covers a mapped group target: the federated entity is
// placed in the group by the mapping, and the group's own role assignment grants the permission.
func (ts *TokenExchangeAuthorizationMappingTestSuite) TestTokenExchange_MappedGroupGrantsScope() {
	sub := ts.nextSubject()
	ts.mockIDP.AddUser(&testutils.OIDCUserInfo{
		Sub:    sub,
		Custom: map[string]interface{}{"department": "platform"},
	})
	ts.applyAuthorizationMapping("department", "", map[string][]testutils.AuthorizationTarget{
		"platform": {{Type: testutils.AuthorizationTargetGroup, ID: ts.mappedGroupID}},
	})

	idToken := ts.mintExternalIDToken(sub)
	resp, status := ts.exchangeToken(idToken, "write", teAuthzRSIdentifier)
	ts.Require().Equal(http.StatusOK, status, "error=%s description=%s", resp.Error, resp.ErrorDescription)
	ts.Contains(strings.Fields(resp.Scope), "write")
}

// TestTokenExchange_MappedPermissionTargetGrantsDirectly covers a mapped permission target: it must
// grant the exchange directly, with no role or group involved.
func (ts *TokenExchangeAuthorizationMappingTestSuite) TestTokenExchange_MappedPermissionTargetGrantsDirectly() {
	sub := ts.nextSubject()
	ts.mockIDP.AddUser(&testutils.OIDCUserInfo{
		Sub:    sub,
		Custom: map[string]interface{}{"scope_claim": "delegate"},
	})
	ts.applyAuthorizationMapping("scope_claim", "", map[string][]testutils.AuthorizationTarget{
		"delegate": {{
			Type: testutils.AuthorizationTargetPermission, ResourceServerID: ts.resourceServerID, Permission: "read",
		}},
	})

	idToken := ts.mintExternalIDToken(sub)
	resp, status := ts.exchangeToken(idToken, "read", teAuthzRSIdentifier)
	ts.Require().Equal(http.StatusOK, status, "error=%s description=%s", resp.Error, resp.ErrorDescription)
	ts.Contains(strings.Fields(resp.Scope), "read")
}

// An attribute value that matches none of the connection's mapping rules is not the same as no
// mapping being configured at all: the mapping is still the sole authority, so the exchange succeeds
// rather than falling back to the subject token's own (empty) scope claim, it just grants nothing for
// the requested scope, since nothing matched.
func (ts *TokenExchangeAuthorizationMappingTestSuite) TestTokenExchange_UnmappedValueGrantsNoScope() {
	sub := ts.nextSubject()
	ts.mockIDP.AddUser(&testutils.OIDCUserInfo{
		Sub:    sub,
		Custom: map[string]interface{}{"groups": []interface{}{"marketing"}},
	})
	ts.applyAuthorizationMapping("groups", "", map[string][]testutils.AuthorizationTarget{
		"te-admins": {{Type: testutils.AuthorizationTargetRole, ID: ts.mappedRoleID}},
	})

	idToken := ts.mintExternalIDToken(sub)
	resp, status := ts.exchangeToken(idToken, "read", teAuthzRSIdentifier)
	ts.Require().Equal(http.StatusOK, status, "error=%s description=%s", resp.Error, resp.ErrorDescription)
	ts.NotContains(strings.Fields(resp.Scope), "read", "an unmapped value must not grant the requested scope")
}

// TestTokenExchange_ListValuedClaimGrantsRole: a list-valued claim resolves every element, matching
// the multi-valued claims table.
func (ts *TokenExchangeAuthorizationMappingTestSuite) TestTokenExchange_ListValuedClaimGrantsRole() {
	sub := ts.nextSubject()
	ts.mockIDP.AddUser(&testutils.OIDCUserInfo{
		Sub:    sub,
		Custom: map[string]interface{}{"groups": []interface{}{"engineering", "te-admins"}},
	})
	ts.applyAuthorizationMapping("groups", "", map[string][]testutils.AuthorizationTarget{
		"te-admins": {{Type: testutils.AuthorizationTargetRole, ID: ts.mappedRoleID}},
	})

	idToken := ts.mintExternalIDToken(sub)
	resp, status := ts.exchangeToken(idToken, "read", teAuthzRSIdentifier)
	ts.Require().Equal(http.StatusOK, status, "error=%s description=%s", resp.Error, resp.ErrorDescription)
	ts.Contains(strings.Fields(resp.Scope), "read")
}

// A list-valued claim combines with a non-equals operator the same way it does with equals: a rule
// contributes if any one of the claim's elements satisfies it. Here "engineering" (not "guest")
// satisfies not_equals "guest", end to end through a real signed subject token carrying a genuine
// JSON array, not a synthetic in-memory shortcut.
func (ts *TokenExchangeAuthorizationMappingTestSuite) TestTokenExchange_NotEqualsOperatorMatchesElementInListClaim() {
	sub := ts.nextSubject()
	ts.mockIDP.AddUser(&testutils.OIDCUserInfo{
		Sub:    sub,
		Custom: map[string]interface{}{"groups": []interface{}{"guest", "engineering"}},
	})
	ts.applyAuthorizationMappingRules("groups", "", []testutils.AuthorizationRule{
		{
			Operator: testutils.AuthorizationOperatorNotEquals,
			Value:    "guest",
			Targets:  []testutils.AuthorizationTarget{{Type: testutils.AuthorizationTargetRole, ID: ts.mappedRoleID}},
		},
	})

	idToken := ts.mintExternalIDToken(sub)
	resp, status := ts.exchangeToken(idToken, "read", teAuthzRSIdentifier)
	ts.Require().Equal(http.StatusOK, status, "error=%s description=%s", resp.Error, resp.ErrorDescription)
	ts.Contains(strings.Fields(resp.Scope), "read")
}

// not_includes tests absence from the whole set, unlike not_equals above: on the identical claim data
// (the subject is a "guest" among other things), not_equals incorrectly granted access meant for
// non-guests, since "engineering" differs from "guest". not_includes correctly withholds it, since
// "guest" is present in the set, end to end through a real signed subject token.
func (ts *TokenExchangeAuthorizationMappingTestSuite) TestTokenExchange_NotIncludesWithholdsWhenExcludedValuePresentInListClaim() {
	ts.applyAuthorizationMappingRules("groups", testutils.AuthorizationValueTypeArray, []testutils.AuthorizationRule{
		{
			Operator: testutils.AuthorizationOperatorNotIncludes,
			Value:    "guest",
			Targets:  []testutils.AuthorizationTarget{{Type: testutils.AuthorizationTargetRole, ID: ts.mappedRoleID}},
		},
	})

	// When "guest" is present, not_includes contributes nothing, exactly like an unmapped value (see
	// TestTokenExchange_UnmappedValueGrantsNoScope): the mapping is still the sole authority, so the
	// exchange succeeds but grants nothing for the requested scope.
	guestSub := ts.nextSubject()
	ts.mockIDP.AddUser(&testutils.OIDCUserInfo{
		Sub:    guestSub,
		Custom: map[string]interface{}{"groups": []interface{}{"guest", "engineering"}},
	})
	guestResp, guestStatus := ts.exchangeToken(ts.mintExternalIDToken(guestSub), "read", teAuthzRSIdentifier)
	ts.Require().Equal(http.StatusOK, guestStatus,
		"error=%s description=%s", guestResp.Error, guestResp.ErrorDescription)
	ts.NotContains(strings.Fields(guestResp.Scope), "read",
		"not_includes must not grant when the excluded value is present in the set, even alongside others")

	nonGuestSub := ts.nextSubject()
	ts.mockIDP.AddUser(&testutils.OIDCUserInfo{
		Sub:    nonGuestSub,
		Custom: map[string]interface{}{"groups": []interface{}{"engineering"}},
	})
	nonGuestResp, nonGuestStatus := ts.exchangeToken(ts.mintExternalIDToken(nonGuestSub), "read", teAuthzRSIdentifier)
	ts.Require().Equal(http.StatusOK, nonGuestStatus,
		"error=%s description=%s", nonGuestResp.Error, nonGuestResp.ErrorDescription)
	ts.Contains(strings.Fields(nonGuestResp.Scope), "read", "not_includes must grant when the excluded value is absent")
}

// includes is a straightforward membership test, end to end through a real signed subject token
// carrying a genuine JSON array.
func (ts *TokenExchangeAuthorizationMappingTestSuite) TestTokenExchange_IncludesOperatorTestsSetMembershipInListClaim() {
	sub := ts.nextSubject()
	ts.mockIDP.AddUser(&testutils.OIDCUserInfo{
		Sub:    sub,
		Custom: map[string]interface{}{"groups": []interface{}{"engineering", "te-admins"}},
	})
	ts.applyAuthorizationMappingRules("groups", testutils.AuthorizationValueTypeArray, []testutils.AuthorizationRule{
		{
			Operator: testutils.AuthorizationOperatorIncludes,
			Value:    "te-admins",
			Targets:  []testutils.AuthorizationTarget{{Type: testutils.AuthorizationTargetRole, ID: ts.mappedRoleID}},
		},
	})

	idToken := ts.mintExternalIDToken(sub)
	resp, status := ts.exchangeToken(idToken, "read", teAuthzRSIdentifier)
	ts.Require().Equal(http.StatusOK, status, "error=%s description=%s", resp.Error, resp.ErrorDescription)
	ts.Contains(strings.Fields(resp.Scope), "read")
}

// TestTokenExchange_SpaceDelimitedClaimGrantsRole: a space-delimited claim is split on the configured
// delimiter and each token is tried against the mapping's values.
func (ts *TokenExchangeAuthorizationMappingTestSuite) TestTokenExchange_SpaceDelimitedClaimGrantsRole() {
	sub := ts.nextSubject()
	ts.mockIDP.AddUser(&testutils.OIDCUserInfo{
		Sub:    sub,
		Custom: map[string]interface{}{"scope_claim": "orders.read orders.write"},
	})
	ts.applyAuthorizationMappingRulesAndDelimiter("scope_claim", testutils.AuthorizationValueTypeString, " ",
		[]testutils.AuthorizationRule{
			{
				Operator: testutils.AuthorizationOperatorIncludes,
				Value:    "orders.write",
				Targets:  []testutils.AuthorizationTarget{{Type: testutils.AuthorizationTargetRole, ID: ts.mappedRoleID}},
			},
		})

	idToken := ts.mintExternalIDToken(sub)
	resp, status := ts.exchangeToken(idToken, "read", teAuthzRSIdentifier)
	ts.Require().Equal(http.StatusOK, status, "error=%s description=%s", resp.Error, resp.ErrorDescription)
	ts.Contains(strings.Fields(resp.Scope), "read")
}

// A mapped permission target is scoped to the resource server it names: requesting the same
// permission handle against a different resource server is not authorized by it (the exchange still
// succeeds, since the client's request is narrowed rather than rejected, exactly as a partial direct
// authorization narrows rather than fails elsewhere in the codebase), while requesting it against the
// resource server it does name is authorized.
func (ts *TokenExchangeAuthorizationMappingTestSuite) TestTokenExchange_PermissionTargetScopedToItsOwnResourceServer() {
	config := func(sub string) {
		ts.mockIDP.AddUser(&testutils.OIDCUserInfo{
			Sub:    sub,
			Custom: map[string]interface{}{"groups": []interface{}{"te-admins"}},
		})
	}
	ts.applyAuthorizationMapping("groups", "", map[string][]testutils.AuthorizationTarget{
		"te-admins": {{
			Type: testutils.AuthorizationTargetPermission, ResourceServerID: ts.resourceServerID, Permission: "read",
		}},
	})

	otherSub := ts.nextSubject()
	config(otherSub)
	otherResp, otherStatus := ts.exchangeToken(ts.mintExternalIDToken(otherSub), "read", teAuthzOtherRSIdentifier)
	ts.Require().Equal(http.StatusOK, otherStatus, "error=%s description=%s", otherResp.Error, otherResp.ErrorDescription)
	ts.NotContains(strings.Fields(otherResp.Scope), "read",
		"a permission target scoped to one resource server must not authorize the same handle on another")

	ownSub := ts.nextSubject()
	config(ownSub)
	ownResp, ownStatus := ts.exchangeToken(ts.mintExternalIDToken(ownSub), "read", teAuthzRSIdentifier)
	ts.Require().Equal(http.StatusOK, ownStatus, "error=%s description=%s", ownResp.Error, ownResp.ErrorDescription)
	ts.Contains(strings.Fields(ownResp.Scope), "read", "a permission target must authorize its own resource server")
}

// TestTokenExchange_NoLocalRecordRequired makes explicit what every other scenario in this file
// depends on: token exchange authorizes a mapped, purely-external entity end to end, with no local
// user ever created for its subject.
func (ts *TokenExchangeAuthorizationMappingTestSuite) TestTokenExchange_NoLocalRecordRequired() {
	sub := ts.nextSubject()
	ts.mockIDP.AddUser(&testutils.OIDCUserInfo{
		Sub:    sub,
		Custom: map[string]interface{}{"groups": []interface{}{"te-admins"}},
	})
	ts.applyAuthorizationMapping("groups", "", map[string][]testutils.AuthorizationTarget{
		"te-admins": {{Type: testutils.AuthorizationTargetRole, ID: ts.mappedRoleID}},
	})

	before, err := testutils.FindUserByAttribute("sub", sub)
	ts.Require().NoError(err)
	ts.Require().Nil(before, "no local record should exist before the exchange")

	idToken := ts.mintExternalIDToken(sub)
	resp, status := ts.exchangeToken(idToken, "read", teAuthzRSIdentifier)
	ts.Require().Equal(http.StatusOK, status, "error=%s description=%s", resp.Error, resp.ErrorDescription)
	ts.Contains(strings.Fields(resp.Scope), "read")

	claims, err := testutils.DecodeJWT(resp.AccessToken)
	ts.Require().NoError(err)
	ts.Equal(sub, claims.Sub)

	after, err := testutils.FindUserByAttribute("sub", sub)
	ts.Require().NoError(err)
	ts.Nil(after, "authorizing the exchange must not have created a local record")
}
