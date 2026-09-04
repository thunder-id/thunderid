// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/*
ID-JAG consumption-leg AuthorizationMapping integration tests.

These exercise the connection's authorizationMappings section on the ID-JAG consumption leg (the
jwt-bearer grant): a mock external issuer signs an ID-JAG assertion carrying claims, the jwt-bearer
grant resolves the assertion issuer's connection, applies its authorization mappings, and the issued
access token's granted scopes are read back. This mirrors
tests/integration/oauth/token/token_exchange_authorization_mapping_test.go, which covers the same
resolution logic on the token exchange path; see that file's scenarios for the full claim-shape and
operator matrix this one deliberately keeps in parity with.

Runs its own mock external issuer, application, resource servers, and role/group, independent of
IDJAGConsumptionTestSuite, so its connection's authorizationMappings can be rewritten per scenario
without disturbing the other ID-JAG tests in this package.
*/
package idjag

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
	idjagAuthzMockIssuerPort = 8103
	idjagAuthzClientID       = "idjag_authz_mapping_client"
	idjagAuthzClientSecret   = "idjag_authz_mapping_secret"
	idjagAuthzAppName        = "IDJAGAuthorizationMappingApp"
	idjagAuthzAudience       = "https://localhost:8095"

	idjagAuthzRSIdentifier      = "https://idjag-authz-mapping.example.com"
	idjagAuthzOtherRSIdentifier = "https://idjag-authz-mapping-other.example.com"
)

// IDJAGAuthorizationMappingTestSuite exercises the ID-JAG consumption leg's AuthorizationMapping
// resolution: the assertion's issuing connection resolves mapped roles/groups/permissions from the
// assertion's claims, the same single-authority rule token exchange applies to its subject token.
type IDJAGAuthorizationMappingTestSuite struct {
	suite.Suite
	client *http.Client

	mockIssuer *testutils.MockOIDCServer

	ouID                  string
	appID                 string
	resourceServerID      string
	otherResourceServerID string
	mappedRoleID          string
	mappedGroupID         string
	groupRoleID           string
	idpID                 string
	jtiCounter            int
}

func TestIDJAGAuthorizationMappingTestSuite(t *testing.T) {
	suite.Run(t, new(IDJAGAuthorizationMappingTestSuite))
}

func (ts *IDJAGAuthorizationMappingTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "idjag-authz-mapping-ou",
		Name:        "ID-JAG Authorization Mapping OU",
		Description: "Organization unit for ID-JAG authorization mapping integration tests",
	})
	ts.Require().NoError(err)
	ts.ouID = ouID

	rsID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:       "ID-JAG Authorization Mapping API",
		Identifier: idjagAuthzRSIdentifier,
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
		Name:       "ID-JAG Authorization Mapping Other API",
		Identifier: idjagAuthzOtherRSIdentifier,
		OUID:       ouID,
	}, []testutils.Action{
		{Name: "Read", Handle: "read", Description: "Read access"},
	})
	ts.Require().NoError(err)
	ts.otherResourceServerID = otherRSID

	// No assignments: reachable only by a mapping naming it directly.
	mappedRoleID, err := testutils.CreateRole(testutils.Role{
		Name: "ID-JAG Mapped Reader",
		OUID: ouID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: rsID, Permissions: []string{"read"}},
		},
	})
	ts.Require().NoError(err)
	ts.mappedRoleID = mappedRoleID

	groupID, err := testutils.CreateGroup(testutils.Group{Name: "ID-JAG Mapped Writers", OUID: ouID})
	ts.Require().NoError(err)
	ts.mappedGroupID = groupID

	groupRoleID, err := testutils.CreateRole(testutils.Role{
		Name: "ID-JAG Group Writer",
		OUID: ouID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: rsID, Permissions: []string{"write"}},
		},
		Assignments: []testutils.Assignment{{ID: groupID, Type: "group"}},
	})
	ts.Require().NoError(err)
	ts.groupRoleID = groupRoleID

	mockIssuer, err := testutils.NewMockOIDCServer(
		idjagAuthzMockIssuerPort, "idjag-authz-mapping-client", "idjag-authz-mapping-secret")
	ts.Require().NoError(err)
	ts.mockIssuer = mockIssuer
	ts.Require().NoError(ts.mockIssuer.Start())

	idpID, err := testutils.CreateIDP(testutils.IDP{
		Name:        "ID-JAG Authorization Mapping IdP",
		Description: "Mock external issuer for ID-JAG authorization mapping tests",
		Type:        "OIDC",
		Properties: []testutils.IDPProperty{
			{Name: "issuer", Value: ts.mockIssuer.GetURL()},
			{Name: "jwks_endpoint", Value: ts.mockIssuer.GetJWKSURL()},
			{Name: "id_jag_enabled", Value: "true"},
		},
	})
	ts.Require().NoError(err)
	ts.idpID = idpID

	ts.appID = ts.createApp()
}

func (ts *IDJAGAuthorizationMappingTestSuite) TearDownSuite() {
	if ts.appID != "" {
		_ = testutils.DeleteApplication(ts.appID)
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
	if ts.mockIssuer != nil {
		_ = ts.mockIssuer.Stop()
	}
}

// createApp creates the confidential, jwt-bearer-only application the assertions are presented to.
func (ts *IDJAGAuthorizationMappingTestSuite) createApp() string {
	ts.T().Helper()
	app := map[string]any{
		"name":                      idjagAuthzAppName,
		"description":               "Application for ID-JAG authorization mapping tests",
		"ouId":                      ts.ouID,
		"type":                      "fullstack",
		"isRegistrationFlowEnabled": false,
		"inboundAuthConfig": []map[string]any{{
			"type": "oauth2",
			"config": map[string]any{
				"clientId":                idjagAuthzClientID,
				"clientSecret":            idjagAuthzClientSecret,
				"grantTypes":              []string{"urn:ietf:params:oauth:grant-type:jwt-bearer"},
				"tokenEndpointAuthMethod": "client_secret_basic",
			},
		}},
	}
	jsonData, err := json.Marshal(app)
	ts.Require().NoError(err)
	req, err := http.NewRequest("POST", testutils.TestServerURL+"/applications", strings.NewReader(string(jsonData)))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	ts.Require().Equalf(http.StatusCreated, resp.StatusCode, "create app failed: %s", string(body))
	var respData map[string]any
	ts.Require().NoError(json.Unmarshal(body, &respData))
	id, _ := respData["id"].(string)
	ts.Require().NotEmpty(id)
	return id
}

// applyAuthorizationMapping replaces the connection's authorizationMappings for one claim. values is
// a claim-value-to-targets map, converted into the equivalent equals rules (sorted by key, so the
// resulting configuration is deterministic) since most scenarios in this file are exact-match cases.
func (ts *IDJAGAuthorizationMappingTestSuite) applyAuthorizationMapping(
	claim, delimiter string, values map[string][]testutils.AuthorizationTarget,
) {
	ts.T().Helper()
	current, err := testutils.GetIDP("oidc", ts.idpID)
	ts.Require().NoError(err)
	ts.Require().NotNil(current)
	current.Properties = ensureIDJagEnabled(current.Properties)
	current.AttributeConfiguration = &testutils.AttributeConfiguration{
		AuthorizationMappings: []testutils.AuthorizationMapping{
			{Claim: claim, Delimiter: delimiter, Values: idjagAuthzEqualsRules(values)},
		},
	}
	ts.Require().NoError(testutils.UpdateIDP(ts.idpID, *current))
}

// applyAuthorizationMappingRules replaces the connection's authorizationMappings for one claim from
// explicit rules, for scenarios that need an operator other than equals.
func (ts *IDJAGAuthorizationMappingTestSuite) applyAuthorizationMappingRules(
	claim, valueType string, rules []testutils.AuthorizationRule,
) {
	ts.applyAuthorizationMappingRulesAndDelimiter(claim, valueType, "", rules)
}

// applyAuthorizationMappingRulesAndDelimiter is applyAuthorizationMappingRules with an explicit
// delimiter, for a multi-valued string claim (a delimiter is only meaningful for the string value
// type).
func (ts *IDJAGAuthorizationMappingTestSuite) applyAuthorizationMappingRulesAndDelimiter(
	claim, valueType, delimiter string, rules []testutils.AuthorizationRule,
) {
	ts.T().Helper()
	current, err := testutils.GetIDP("oidc", ts.idpID)
	ts.Require().NoError(err)
	ts.Require().NotNil(current)
	current.Properties = ensureIDJagEnabled(current.Properties)
	current.AttributeConfiguration = &testutils.AttributeConfiguration{
		AuthorizationMappings: []testutils.AuthorizationMapping{
			{Claim: claim, ValueType: valueType, Delimiter: delimiter, Values: rules},
		},
	}
	ts.Require().NoError(testutils.UpdateIDP(ts.idpID, *current))
}

// ensureIDJagEnabled returns props with id_jag_enabled set to true, adding it if absent.
// testutils.GetIDP cannot round-trip this flag (it has no generic property-to-field mapping, only a
// write-side special case in idpToConnectionBody), so every update must re-assert it or the trust-only
// connection's reduced required-property set never activates and the update is rejected for missing
// client credentials this connection was never given.
func ensureIDJagEnabled(props []testutils.IDPProperty) []testutils.IDPProperty {
	for i, p := range props {
		if p.Name == "id_jag_enabled" {
			props[i].Value = "true"
			return props
		}
	}
	return append(props, testutils.IDPProperty{Name: "id_jag_enabled", Value: "true"})
}

// idjagAuthzEqualsRules converts a claim-value-to-targets map into the equivalent equals rules, one
// per key, sorted by key for a deterministic result.
func idjagAuthzEqualsRules(values map[string][]testutils.AuthorizationTarget) []testutils.AuthorizationRule {
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

// nextJTI returns a jti unique to this test run.
func (ts *IDJAGAuthorizationMappingTestSuite) nextJTI() string {
	ts.jtiCounter++
	return fmt.Sprintf("idjag-authz-jti-%d-%d", ts.jtiCounter, time.Now().UnixNano())
}

// buildAssertion signs an ID-JAG assertion from the mock issuer. The assertion's own scope claim is
// deliberately something no scenario in this file requests ("irrelevant_scope"): every mapped
// scenario proves the mapping is the sole authority precisely by requesting a scope this claim does
// not carry.
func (ts *IDJAGAuthorizationMappingTestSuite) buildAssertion(customClaims map[string]interface{}) string {
	ts.T().Helper()

	header := map[string]interface{}{
		"typ": "oauth-id-jag+jwt",
		"alg": "RS256",
		"kid": "oidc-key-1",
	}
	now := time.Now()
	claims := map[string]interface{}{
		"iss":       ts.mockIssuer.GetURL(),
		"sub":       "idjag-authz-mapping-subject",
		"aud":       idjagAuthzAudience,
		"client_id": idjagAuthzClientID,
		"exp":       now.Add(5 * time.Minute).Unix(),
		"iat":       now.Unix(),
		"nbf":       now.Unix(),
		"jti":       ts.nextJTI(),
		"scope":     "irrelevant_scope",
	}
	for k, v := range customClaims {
		claims[k] = v
	}

	token, err := ts.mockIssuer.SignJWT(header, claims)
	ts.Require().NoError(err)
	return token
}

// consumeAssertion sends a jwt-bearer grant request to /oauth2/token with the given assertion,
// requesting scope against the given resource.
func (ts *IDJAGAuthorizationMappingTestSuite) consumeAssertion(assertion, scope, resource string) (
	IDJAGTokenResponse, int) {
	ts.T().Helper()

	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	formData.Set("assertion", assertion)
	if scope != "" {
		formData.Set("scope", scope)
	}
	if resource != "" {
		formData.Set("resource", resource)
	}

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/oauth2/token", strings.NewReader(formData.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Basic "+basicAuth(idjagAuthzClientID, idjagAuthzClientSecret))

	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	var result IDJAGTokenResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&result))
	return result, resp.StatusCode
}

// TestIDJAGConsumption_MappedRoleGrantsScopeBeyondAssertionScope covers a mapped role (no local
// assignees) resolved from the assertion's claims: the assertion's own scope claim carries nothing
// ThunderID recognizes ("irrelevant_scope"), so only the mapping resolves the requested scope.
func (ts *IDJAGAuthorizationMappingTestSuite) TestIDJAGConsumption_MappedRoleGrantsScopeBeyondAssertionScope() {
	ts.applyAuthorizationMapping("groups", "", map[string][]testutils.AuthorizationTarget{
		"idjag-admins": {{Type: testutils.AuthorizationTargetRole, ID: ts.mappedRoleID}},
	})
	assertion := ts.buildAssertion(map[string]interface{}{"groups": []interface{}{"idjag-admins"}})

	resp, status := ts.consumeAssertion(assertion, "read", idjagAuthzRSIdentifier)
	ts.Require().Equal(http.StatusOK, status, "error=%s description=%s", resp.Error, resp.ErrorDescription)
	ts.Contains(strings.Fields(resp.Scope), "read")
}

// TestIDJAGConsumption_MappedGroupGrantsScope covers a mapped group target: the assertion's claim
// places the entity in the group, and the group's own role assignment grants the permission.
func (ts *IDJAGAuthorizationMappingTestSuite) TestIDJAGConsumption_MappedGroupGrantsScope() {
	ts.applyAuthorizationMapping("department", "", map[string][]testutils.AuthorizationTarget{
		"platform": {{Type: testutils.AuthorizationTargetGroup, ID: ts.mappedGroupID}},
	})
	assertion := ts.buildAssertion(map[string]interface{}{"department": "platform"})

	resp, status := ts.consumeAssertion(assertion, "write", idjagAuthzRSIdentifier)
	ts.Require().Equal(http.StatusOK, status, "error=%s description=%s", resp.Error, resp.ErrorDescription)
	ts.Contains(strings.Fields(resp.Scope), "write")
}

// TestIDJAGConsumption_MappedPermissionGrantsDirectly covers a mapped permission target: it must
// grant the request directly, with no role or group involved.
func (ts *IDJAGAuthorizationMappingTestSuite) TestIDJAGConsumption_MappedPermissionGrantsDirectly() {
	ts.applyAuthorizationMapping("scope_claim", "", map[string][]testutils.AuthorizationTarget{
		"delegate": {{
			Type: testutils.AuthorizationTargetPermission, ResourceServerID: ts.resourceServerID, Permission: "read",
		}},
	})
	assertion := ts.buildAssertion(map[string]interface{}{"scope_claim": "delegate"})

	resp, status := ts.consumeAssertion(assertion, "read", idjagAuthzRSIdentifier)
	ts.Require().Equal(http.StatusOK, status, "error=%s description=%s", resp.Error, resp.ErrorDescription)
	ts.Contains(strings.Fields(resp.Scope), "read")
}

// An attribute value that matches none of the connection's mapping rules is not the same as no
// mapping being configured: the mapping is still the sole authority, so the request succeeds rather
// than falling back to the assertion's own scope claim, it just grants nothing for the requested
// scope.
func (ts *IDJAGAuthorizationMappingTestSuite) TestIDJAGConsumption_UnmappedValueGrantsNoScope() {
	ts.applyAuthorizationMapping("groups", "", map[string][]testutils.AuthorizationTarget{
		"idjag-admins": {{Type: testutils.AuthorizationTargetRole, ID: ts.mappedRoleID}},
	})
	assertion := ts.buildAssertion(map[string]interface{}{"groups": []interface{}{"marketing"}})

	resp, status := ts.consumeAssertion(assertion, "read", idjagAuthzRSIdentifier)
	ts.Require().Equal(http.StatusOK, status, "error=%s description=%s", resp.Error, resp.ErrorDescription)
	ts.NotContains(strings.Fields(resp.Scope), "read", "an unmapped value must not grant the requested scope")
}

// TestIDJAGConsumption_ListValuedClaimGrantsRole: a list-valued claim resolves every element,
// matching the multi-valued claims table.
func (ts *IDJAGAuthorizationMappingTestSuite) TestIDJAGConsumption_ListValuedClaimGrantsRole() {
	ts.applyAuthorizationMapping("groups", "", map[string][]testutils.AuthorizationTarget{
		"idjag-admins": {{Type: testutils.AuthorizationTargetRole, ID: ts.mappedRoleID}},
	})
	assertion := ts.buildAssertion(map[string]interface{}{
		"groups": []interface{}{"engineering", "idjag-admins"},
	})

	resp, status := ts.consumeAssertion(assertion, "read", idjagAuthzRSIdentifier)
	ts.Require().Equal(http.StatusOK, status, "error=%s description=%s", resp.Error, resp.ErrorDescription)
	ts.Contains(strings.Fields(resp.Scope), "read")
}

// A list-valued claim combines with a non-equals operator the same way it does with equals: a rule
// contributes if any one of the claim's elements satisfies it.
func (ts *IDJAGAuthorizationMappingTestSuite) TestIDJAGConsumption_NotEqualsOperatorMatchesElementInListClaim() {
	ts.applyAuthorizationMappingRules("groups", "", []testutils.AuthorizationRule{
		{
			Operator: testutils.AuthorizationOperatorNotEquals,
			Value:    "guest",
			Targets:  []testutils.AuthorizationTarget{{Type: testutils.AuthorizationTargetRole, ID: ts.mappedRoleID}},
		},
	})
	assertion := ts.buildAssertion(map[string]interface{}{
		"groups": []interface{}{"guest", "engineering"},
	})

	resp, status := ts.consumeAssertion(assertion, "read", idjagAuthzRSIdentifier)
	ts.Require().Equal(http.StatusOK, status, "error=%s description=%s", resp.Error, resp.ErrorDescription)
	ts.Contains(strings.Fields(resp.Scope), "read")
}

// not_includes tests absence from the whole set, unlike not_equals above: on identical claim data (the
// subject is a "guest" among other things), not_equals incorrectly grants access meant for
// non-guests; not_includes correctly withholds it, since "guest" is present in the set.
func (ts *IDJAGAuthorizationMappingTestSuite) TestIDJAGConsumption_NotIncludesWithholdsWhenExcludedValuePresentInListClaim() {
	ts.applyAuthorizationMappingRules("groups", testutils.AuthorizationValueTypeArray, []testutils.AuthorizationRule{
		{
			Operator: testutils.AuthorizationOperatorNotIncludes,
			Value:    "guest",
			Targets:  []testutils.AuthorizationTarget{{Type: testutils.AuthorizationTargetRole, ID: ts.mappedRoleID}},
		},
	})

	guestAssertion := ts.buildAssertion(map[string]interface{}{
		"groups": []interface{}{"guest", "engineering"},
	})
	guestResp, guestStatus := ts.consumeAssertion(guestAssertion, "read", idjagAuthzRSIdentifier)
	ts.Require().Equal(http.StatusOK, guestStatus,
		"error=%s description=%s", guestResp.Error, guestResp.ErrorDescription)
	ts.NotContains(strings.Fields(guestResp.Scope), "read",
		"not_includes must not grant when the excluded value is present in the set, even alongside others")

	nonGuestAssertion := ts.buildAssertion(map[string]interface{}{
		"groups": []interface{}{"engineering"},
	})
	nonGuestResp, nonGuestStatus := ts.consumeAssertion(nonGuestAssertion, "read", idjagAuthzRSIdentifier)
	ts.Require().Equal(http.StatusOK, nonGuestStatus,
		"error=%s description=%s", nonGuestResp.Error, nonGuestResp.ErrorDescription)
	ts.Contains(strings.Fields(nonGuestResp.Scope), "read", "not_includes must grant when the excluded value is absent")
}

// includes is a straightforward membership test, end to end through a real signed assertion carrying
// a genuine JSON array.
func (ts *IDJAGAuthorizationMappingTestSuite) TestIDJAGConsumption_IncludesOperatorTestsSetMembershipInListClaim() {
	ts.applyAuthorizationMappingRules("groups", testutils.AuthorizationValueTypeArray, []testutils.AuthorizationRule{
		{
			Operator: testutils.AuthorizationOperatorIncludes,
			Value:    "idjag-admins",
			Targets:  []testutils.AuthorizationTarget{{Type: testutils.AuthorizationTargetRole, ID: ts.mappedRoleID}},
		},
	})
	assertion := ts.buildAssertion(map[string]interface{}{
		"groups": []interface{}{"engineering", "idjag-admins"},
	})

	resp, status := ts.consumeAssertion(assertion, "read", idjagAuthzRSIdentifier)
	ts.Require().Equal(http.StatusOK, status, "error=%s description=%s", resp.Error, resp.ErrorDescription)
	ts.Contains(strings.Fields(resp.Scope), "read")
}

// TestIDJAGConsumption_SpaceDelimitedClaimGrantsRole: a space-delimited claim is split on the
// configured delimiter and each token is tried against the mapping's values.
func (ts *IDJAGAuthorizationMappingTestSuite) TestIDJAGConsumption_SpaceDelimitedClaimGrantsRole() {
	ts.applyAuthorizationMappingRulesAndDelimiter("scope_claim", testutils.AuthorizationValueTypeString, " ",
		[]testutils.AuthorizationRule{
			{
				Operator: testutils.AuthorizationOperatorIncludes,
				Value:    "orders.write",
				Targets:  []testutils.AuthorizationTarget{{Type: testutils.AuthorizationTargetRole, ID: ts.mappedRoleID}},
			},
		})
	assertion := ts.buildAssertion(map[string]interface{}{"scope_claim": "orders.read orders.write"})

	resp, status := ts.consumeAssertion(assertion, "read", idjagAuthzRSIdentifier)
	ts.Require().Equal(http.StatusOK, status, "error=%s description=%s", resp.Error, resp.ErrorDescription)
	ts.Contains(strings.Fields(resp.Scope), "read")
}

// A mapped permission target is scoped to the resource server it names: requesting the same
// permission handle against a different resource server is not authorized by it (the request still
// succeeds, since it's narrowed rather than rejected), while requesting it against the resource
// server it does name is authorized.
func (ts *IDJAGAuthorizationMappingTestSuite) TestIDJAGConsumption_PermissionTargetScopedToItsOwnResourceServer() {
	ts.applyAuthorizationMapping("groups", "", map[string][]testutils.AuthorizationTarget{
		"idjag-admins": {{
			Type: testutils.AuthorizationTargetPermission, ResourceServerID: ts.resourceServerID, Permission: "read",
		}},
	})

	otherAssertion := ts.buildAssertion(map[string]interface{}{"groups": []interface{}{"idjag-admins"}})
	otherResp, otherStatus := ts.consumeAssertion(otherAssertion, "read", idjagAuthzOtherRSIdentifier)
	ts.Require().Equal(http.StatusOK, otherStatus, "error=%s description=%s", otherResp.Error, otherResp.ErrorDescription)
	ts.NotContains(strings.Fields(otherResp.Scope), "read",
		"a permission target scoped to one resource server must not authorize the same handle on another")

	ownAssertion := ts.buildAssertion(map[string]interface{}{"groups": []interface{}{"idjag-admins"}})
	ownResp, ownStatus := ts.consumeAssertion(ownAssertion, "read", idjagAuthzRSIdentifier)
	ts.Require().Equal(http.StatusOK, ownStatus, "error=%s description=%s", ownResp.Error, ownResp.ErrorDescription)
	ts.Contains(strings.Fields(ownResp.Scope), "read", "a permission target must authorize its own resource server")
}
