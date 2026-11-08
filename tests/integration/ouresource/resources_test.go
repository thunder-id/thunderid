// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ouresource

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

func (ts *OUResourceTestSuite) TestApplications() {
	own, err := testutils.CreateApplication(testutils.Application{Name: "ou-resource-own-app", OUID: ts.ou1ID})
	ts.Require().NoError(err)
	ts.cleanup("/applications/" + own)
	other, err := testutils.CreateApplication(testutils.Application{Name: "ou-resource-other-app", OUID: ts.ou2ID})
	ts.Require().NoError(err)
	ts.cleanup("/applications/" + other)

	ts.requireConfinedToOwnOU("/applications", own, other)
	ts.requireStatus(http.StatusForbidden, http.MethodPost, "/applications",
		map[string]interface{}{"name": "ou-resource-refused-app", "type": "custom", "ouId": ts.ou2ID})

	body := ts.requireStatus(http.StatusOK, http.MethodGet, "/applications", nil)
	ts.requireListedOnlyOwn(listedIDs(body, "applications"), own, other)
}

func (ts *OUResourceTestSuite) TestRoles() {
	own, err := testutils.CreateRole(testutils.Role{Name: "ou-resource-own-role", OUID: ts.ou1ID})
	ts.Require().NoError(err)
	ts.cleanup("/roles/" + own)
	other, err := testutils.CreateRole(testutils.Role{Name: "ou-resource-other-role", OUID: ts.ou2ID})
	ts.Require().NoError(err)
	ts.cleanup("/roles/" + other)

	ts.requireConfinedToOwnOU("/roles", own, other)
	ts.requireStatus(http.StatusForbidden, http.MethodPut, "/roles/"+other,
		map[string]interface{}{"name": "ou-resource-renamed-role"})
	ts.requireStatus(http.StatusForbidden, http.MethodPost, "/roles",
		map[string]interface{}{"name": "ou-resource-refused-role", "ouId": ts.ou2ID})
	ts.requireStatus(http.StatusForbidden, http.MethodGet, "/roles/"+other+"/assignments", nil)

	body := ts.requireStatus(http.StatusOK, http.MethodGet, "/roles?limit=100", nil)
	ts.requireListedOnlyOwn(listedIDs(body, "roles"), own, other)

	ts.requireStatus(http.StatusOK, http.MethodGet, "/organization-units/"+ts.ou1ID+"/roles", nil)
	ts.requireStatus(http.StatusForbidden, http.MethodGet, "/organization-units/"+ts.ou2ID+"/roles", nil)
}

func (ts *OUResourceTestSuite) TestRoleListInChildOU() {
	own, err := testutils.CreateRole(testutils.Role{Name: "ou-resource-shared-role", OUID: ts.ou1ID})
	ts.Require().NoError(err)
	ts.cleanup("/roles/" + own)
	child, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "ou-resource-roles-child", Name: "OU Resource Roles Child", Parent: &ts.ou1ID,
	})
	ts.Require().NoError(err)
	ts.cleanups = append([]func(){func() { _ = testutils.DeleteOrganizationUnit(child) }}, ts.cleanups...)
	created := ts.requireStatus(http.StatusCreated, http.MethodPost, "/roles/"+own+"/sharing-policies",
		map[string]interface{}{
			"childUnits": map[string]interface{}{
				"mode": "selected", "units": []map[string]string{{"ouId": child, "reach": "unit"}},
			},
		})
	ts.cleanup("/roles/" + own + "/sharing-policies/" + idOf(created))

	// The administrator may not list the child's roles, so the list holds only the roles shared into it.
	body := ts.requireStatus(http.StatusOK, http.MethodGet,
		"/roles?limit=100&filter="+url.QueryEscape(`ouId eq "`+child+`"`), nil)
	var list struct {
		Roles []struct {
			ID          string `json:"id"`
			OUID        string `json:"ouId"`
			IsReadOnly  bool   `json:"isReadOnly"`
			IsOwnedByOU bool   `json:"isOwnedByOU"`
		} `json:"roles"`
	}
	ts.Require().NoError(json.Unmarshal(body, &list))
	ts.Require().Len(list.Roles, 1)
	ts.Equal(own, list.Roles[0].ID)
	ts.Equal(child, list.Roles[0].OUID)
	ts.True(list.Roles[0].IsReadOnly)
	ts.False(list.Roles[0].IsOwnedByOU)
}

func (ts *OUResourceTestSuite) TestFlows() {
	own, err := testutils.CreateFlow(ouResourceFlow("ou-resource-own-flow", ts.ou1ID))
	ts.Require().NoError(err)
	ts.cleanup("/flows/" + own)
	other, err := testutils.CreateFlow(ouResourceFlow("ou-resource-other-flow", ts.ou2ID))
	ts.Require().NoError(err)
	ts.cleanup("/flows/" + other)

	ts.requireConfinedToOwnOU("/flows", own, other)
	ts.requireStatus(http.StatusForbidden, http.MethodPut, "/flows/"+other,
		ouResourceFlow("ou-resource-other-flow", ts.ou2ID))
	ts.requireStatus(http.StatusForbidden, http.MethodPost, "/flows",
		ouResourceFlow("ou-resource-refused-flow", ts.ou2ID))

	body := ts.requireStatus(http.StatusOK, http.MethodGet, "/flows?limit=100", nil)
	ts.requireListedOnlyOwn(listedIDs(body, "flows"), own, other)
	body = ts.requireStatus(http.StatusOK, http.MethodGet,
		"/flows?filter="+url.QueryEscape(`ouId eq "`+ts.ou2ID+`"`), nil)
	ts.Empty(listedIDs(body, "flows"), "a list of another OU's flows must be empty")

	ts.requireStatus(http.StatusForbidden, http.MethodGet, "/flows/"+other+"?ouId="+ts.ou2ID, nil)
	ts.requireStatus(http.StatusForbidden, http.MethodGet, "/flows/"+other+"/versions", nil)
	ts.requireStatus(http.StatusForbidden, http.MethodGet, "/flows/"+other+"/versions/1", nil)
	ts.requireStatus(http.StatusForbidden, http.MethodPost, "/flows/"+other+"/restore",
		map[string]interface{}{"version": 1})
	ts.requireStatus(http.StatusForbidden, http.MethodGet, "/flows/"+other+"/variables?ouId="+ts.ou2ID, nil)
	ts.requireStatus(http.StatusForbidden, http.MethodGet, "/flows/"+other+"/overlay-rules?ouId="+ts.ou2ID, nil)
	ts.requireStatus(http.StatusForbidden, http.MethodGet, "/flows/"+other+"/sharing-policies", nil)
	ts.requireStatus(http.StatusForbidden, http.MethodPost, "/flows/"+other+"/sharing-policies",
		map[string]interface{}{"childUnits": map[string]interface{}{"mode": "all"}})
}

func (ts *OUResourceTestSuite) TestResourceServers() {
	own := ts.createAsRoot("/resource-servers", resourceServerIn("ou-resource-own-rs", ts.ou1ID))
	other := ts.createAsRoot("/resource-servers", resourceServerIn("ou-resource-other-rs", ts.ou2ID))

	ts.requireConfinedToOwnOU("/resource-servers", own, other)
	ts.requireStatus(http.StatusForbidden, http.MethodGet, "/resource-servers/"+other+"/resources", nil)
	ts.requireStatus(http.StatusForbidden, http.MethodPost, "/resource-servers/"+other+"/resources",
		map[string]interface{}{"name": "Refused", "handle": "refused"})
	ts.requireStatus(http.StatusForbidden, http.MethodPost, "/resource-servers",
		resourceServerIn("ou-resource-refused-rs", ts.ou2ID))

	body := ts.requireStatus(http.StatusOK, http.MethodGet, "/resource-servers?limit=100", nil)
	ts.requireListedOnlyOwn(listedIDs(body, "resourceServers"), own, other)
}

func (ts *OUResourceTestSuite) TestConnections() {
	own := ts.createAsRoot("/connections/http", httpConnectionIn("ou-resource-own-http", ts.ou1ID))
	other := ts.createAsRoot("/connections/http", httpConnectionIn("ou-resource-other-http", ts.ou2ID))

	ts.requireConfinedToOwnOU("/connections/http", own, other)
	ts.requireStatus(http.StatusForbidden, http.MethodPost, "/connections/http",
		httpConnectionIn("ou-resource-refused-http", ts.ou2ID))

	body := ts.requireStatus(http.StatusOK, http.MethodGet, "/connections?limit=100", nil)
	ts.requireListedOnlyOwn(listedIDs(body, "connections"), own, other)
	body = ts.requireStatus(http.StatusOK, http.MethodGet, "/connections/http", nil)
	ts.requireListedOnlyOwn(listedArrayIDs(body), own, other)
}

func (ts *OUResourceTestSuite) TestCredentialConfigurations() {
	const path = "/openid4vci/credential-configurations"
	own := ts.createAsRoot(path, credentialConfigurationIn("ou_resource_own_cfg", ts.ou1ID))
	other := ts.createAsRoot(path, credentialConfigurationIn("ou_resource_other_cfg", ts.ou2ID))

	ts.requireConfinedToOwnOU(path, own, other)
	ts.requireStatus(http.StatusForbidden, http.MethodPost, path,
		credentialConfigurationIn("ou_resource_refused_cfg", ts.ou2ID))
	ts.requireStatus(http.StatusForbidden, http.MethodPut, path+"/"+other,
		credentialConfigurationIn("ou_resource_other_cfg", ts.ou2ID))
	// Moving the administrator's own configuration into another OU is refused too.
	ts.requireStatus(http.StatusForbidden, http.MethodPut, path+"/"+own,
		credentialConfigurationIn("ou_resource_own_cfg", ts.ou2ID))

	body := ts.requireStatus(http.StatusOK, http.MethodGet, path, nil)
	ts.requireListedOnlyOwn(listedArrayIDs(body), own, other)
}

func ouResourceFlow(handle, ouID string) testutils.Flow {
	return testutils.Flow{
		Name:     "OU resource " + handle,
		FlowType: "AUTHENTICATION",
		Handle:   handle,
		OUID:     ouID,
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "auth_assert"},
			{"id": "auth_assert", "type": "TASK_EXECUTION",
				"executor": map[string]interface{}{"name": "AuthAssertExecutor"}, "onSuccess": "end"},
			{"id": "end", "type": "END"},
		},
	}
}

func resourceServerIn(name, ouID string) map[string]interface{} {
	return map[string]interface{}{
		"name":       name,
		"identifier": "https://" + name + ".example.com/api",
		"ouId":       ouID,
	}
}

func httpConnectionIn(name, ouID string) map[string]interface{} {
	return map[string]interface{}{
		"name": name,
		"ouId": ouID,
		"url":  "https://" + name + ".example.com/hook",
	}
}

func credentialConfigurationIn(handle, ouID string) map[string]interface{} {
	return map[string]interface{}{
		"handle": handle,
		"ouId":   ouID,
		"name":   "OU resource " + handle,
		"format": "dc+sd-jwt",
		"vct":    "https://credentials.example.com/" + handle,
		"claims": []map[string]string{{"name": "given_name"}},
	}
}

func (ts *OUResourceTestSuite) TestApplicationSharingPolicies() {
	own, err := testutils.CreateApplication(testutils.Application{Name: "ou-resource-own-shared-app", OUID: ts.ou1ID})
	ts.Require().NoError(err)
	ts.cleanup("/applications/" + own)
	other, err := testutils.CreateApplication(testutils.Application{Name: "ou-resource-other-shared-app",
		OUID: ts.ou2ID})
	ts.Require().NoError(err)
	ts.cleanup("/applications/" + other)
	child, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "ou-resource-ou1-child", Name: "OU Resource OU1 Child", Parent: &ts.ou1ID,
	})
	ts.Require().NoError(err)
	ts.cleanups = append([]func(){func() { _ = testutils.DeleteOrganizationUnit(child) }}, ts.cleanups...)
	share := map[string]interface{}{
		"childUnits": map[string]interface{}{
			"mode": "selected", "units": []map[string]string{{"ouId": child, "reach": "unit"}},
		},
	}

	ts.requireStatus(http.StatusForbidden, http.MethodPost, "/applications/"+other+"/sharing-policies", share)
	ts.requireStatus(http.StatusForbidden, http.MethodGet, "/applications/"+other+"/sharing-policies", nil)
	created := ts.requireStatus(http.StatusCreated, http.MethodPost, "/applications/"+own+"/sharing-policies", share)
	ts.cleanup("/applications/" + own + "/sharing-policies/" + idOf(created))
	ts.requireStatus(http.StatusOK, http.MethodGet, "/applications/"+own+"/sharing-policies", nil)
}

func (ts *OUResourceTestSuite) TestRoleAssignmentsInOtherOU() {
	own, err := testutils.CreateRole(testutils.Role{Name: "ou-resource-assigned-role", OUID: ts.ou1ID})
	ts.Require().NoError(err)
	ts.cleanup("/roles/" + own)
	assign := func(ouID string) map[string]interface{} {
		return map[string]interface{}{"ouId": ouID,
			"assignments": []map[string]string{{"id": ts.adminID, "type": "user"}}}
	}

	ts.requireStatus(http.StatusForbidden, http.MethodPost, "/roles/"+own+"/assignments/add", assign(ts.ou2ID))
	ts.requireStatus(http.StatusForbidden, http.MethodGet,
		"/roles/"+own+"/assignments?filter="+url.QueryEscape(`ouId eq "`+ts.ou2ID+`"`), nil)
	ts.requireStatus(http.StatusOK, http.MethodGet, "/roles/"+own+"/assignments", nil)
	ts.requireStatus(http.StatusOK, http.MethodGet, "/organization-units/tree/"+parentHandle+"/"+ou1Handle+"/roles",
		nil)
	ts.requireStatus(http.StatusForbidden, http.MethodGet, "/organization-units/tree/"+ou2Handle+"/roles", nil)
}

func (ts *OUResourceTestSuite) TestIdentityProviderSenderAndPDPConnections() {
	vendors := []struct {
		path string
		body func(name, ouID string) map[string]interface{}
	}{
		{"/connections/google", func(name, ouID string) map[string]interface{} {
			return map[string]interface{}{"name": name, "ouId": ouID, "clientId": "g-client",
				"clientSecret": "g-secret", "redirectUri": "https://localhost:3000/google/callback"}
		}},
		{"/connections/twilio", func(name, ouID string) map[string]interface{} {
			return map[string]interface{}{"name": name, "ouId": ouID, "accountSid": "AC00000000000000000000000000000000",
				"authToken": "tw-token", "senderId": "+15005550006"}
		}},
		{"/connections/authzen-pdp", func(name, ouID string) map[string]interface{} {
			return map[string]interface{}{"name": name, "ouId": ouID,
				"endpoint": "https://" + name + ".example.com/access/v1/evaluation"}
		}},
	}
	for _, v := range vendors {
		own := ts.createAsRoot(v.path, v.body("ou-resource-own"+strings.ReplaceAll(v.path, "/", "-"), ts.ou1ID))
		other := ts.createAsRoot(v.path, v.body("ou-resource-other"+strings.ReplaceAll(v.path, "/", "-"), ts.ou2ID))

		ts.requireConfinedToOwnOU(v.path, own, other)
		ts.requireStatus(http.StatusForbidden, http.MethodPost, v.path,
			v.body("ou-resource-refused"+strings.ReplaceAll(v.path, "/", "-"), ts.ou2ID))
	}
}

func (ts *OUResourceTestSuite) TestPresentationDefinitions() {
	const path = "/openid4vp/presentation-definitions"
	own := ts.createAsRoot(path, presentationDefinitionIn("ou_resource_own_pd", ts.ou1ID))
	other := ts.createAsRoot(path, presentationDefinitionIn("ou_resource_other_pd", ts.ou2ID))

	ts.requireConfinedToOwnOU(path, own, other)
	ts.requireStatus(http.StatusForbidden, http.MethodPost, path,
		presentationDefinitionIn("ou_resource_refused_pd", ts.ou2ID))
	ts.requireStatus(http.StatusForbidden, http.MethodPut, path+"/"+other,
		presentationDefinitionIn("ou_resource_other_pd", ts.ou2ID))
	// Moving the administrator's own definition into another OU is refused too.
	ts.requireStatus(http.StatusForbidden, http.MethodPut, path+"/"+own,
		presentationDefinitionIn("ou_resource_own_pd", ts.ou2ID))

	body := ts.requireStatus(http.StatusOK, http.MethodGet, path, nil)
	ts.requireListedOnlyOwn(listedArrayIDs(body), own, other)
}

func (ts *OUResourceTestSuite) TestAgentTypes() {
	schema := map[string]interface{}{"name": map[string]interface{}{"type": "string"}}
	own := ts.createAsRoot("/agent-types", map[string]interface{}{"handle": "ou-resource-own-agent-type",
		"displayName": "OU Resource Own Agent Type", "ouId": ts.ou1ID, "schema": schema})
	other := ts.createAsRoot("/agent-types", map[string]interface{}{"handle": "ou-resource-other-agent-type",
		"displayName": "OU Resource Other Agent Type", "ouId": ts.ou2ID, "schema": schema})

	body := ts.requireStatus(http.StatusOK, http.MethodGet, "/agent-types?limit=100", nil)
	ts.requireListedOnlyOwn(listedIDs(body, "types"), own, other)
}

func presentationDefinitionIn(handle, ouID string) map[string]interface{} {
	return map[string]interface{}{
		"handle":          handle,
		"ouId":            ouID,
		"name":            "OU resource " + handle,
		"format":          "dc+sd-jwt",
		"vct":             "https://credentials.example.com/" + handle,
		"requestedClaims": []string{"given_name"},
	}
}

// A token that names no organization unit lists nothing and reads nothing, whatever permissions it carries.
func (ts *OUResourceTestSuite) TestTokenWithoutOU() {
	const clientID = "ou-resource-no-ou-sign-in"
	appID, err := testutils.CreateApplication(testutils.Application{
		Name: "OU Resource No OU Sign-in",
		OUID: ts.ou1ID,
		Type: "browser",
		InboundAuthConfig: []map[string]interface{}{{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                clientID,
				"redirectUris":            []string{testutils.OUSignInRedirectURI},
				"grantTypes":              []string{"authorization_code"},
				"responseTypes":           []string{"code"},
				"pkceRequired":            true,
				"tokenEndpointAuthMethod": "none",
				"publicClient":            true,
			},
		}},
	})
	ts.Require().NoError(err)
	ts.cleanup("/applications/" + appID)

	scope := "system"
	for _, p := range adminPermissions {
		scope += " " + p
	}
	token, err := testutils.ObtainAccessTokenWithPassword(clientID, testutils.OUSignInRedirectURI, scope,
		adminUsername, adminPassword, true, "", rsIdentifier)
	ts.Require().NoError(err)
	client := testutils.GetHTTPClientWithToken(token.AccessToken)

	own, err := testutils.CreateFlow(ouResourceFlow("ou-resource-no-ou-flow", ts.ou1ID))
	ts.Require().NoError(err)
	ts.cleanup("/flows/" + own)

	status, body := ts.send(client, http.MethodGet, "/flows?limit=100", nil)
	ts.Require().Equal(http.StatusOK, status, string(body))
	ts.Empty(listedIDs(body, "flows"))
	for _, path := range []string{"/connections/http", "/openid4vci/credential-configurations",
		"/openid4vp/presentation-definitions"} {
		status, body = ts.send(client, http.MethodGet, path, nil)
		ts.Require().Equalf(http.StatusOK, status, "%s: %s", path, body)
		ts.Emptyf(listedArrayIDs(body), "%s must list nothing", path)
	}
	status, body = ts.send(client, http.MethodGet, "/flows/"+own, nil)
	ts.Equal(http.StatusForbidden, status, string(body))
}
