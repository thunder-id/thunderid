// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ouresource

import (
	"encoding/json"
	"fmt"
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

// The administrator may not list the child's roles, so a list filtered to the child is empty, even of the
// administrator's own role shared into it.
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

	body := ts.requireStatus(http.StatusOK, http.MethodGet,
		"/roles?limit=100&filter="+url.QueryEscape(`ouId eq "`+child+`"`), nil)
	ts.Empty(listedIDs(body, "roles"), "a list of a unit the administrator may not list must be empty")
}

// A list filtered to a unit the administrator may not list is empty for every resource, whatever that unit
// owns or is shared.
func (ts *OUResourceTestSuite) TestListFilteredToAnotherOUIsEmpty() {
	app, err := testutils.CreateApplication(testutils.Application{Name: "ou-resource-filter-app", OUID: ts.parentID})
	ts.Require().NoError(err)
	ts.cleanup("/applications/" + app)
	role, err := testutils.CreateRole(testutils.Role{Name: "ou-resource-filter-role", OUID: ts.parentID})
	ts.Require().NoError(err)
	ts.cleanup("/roles/" + role)
	flow, err := testutils.CreateFlow(ouResourceFlow("ou-resource-filter-flow", ts.parentID))
	ts.Require().NoError(err)
	ts.cleanup("/flows/" + flow)
	ts.createAsRoot("/connections/http", httpConnectionIn("ou-resource-filter-http", ts.parentID))

	for _, ouID := range []string{ts.parentID, ts.ou2ID} {
		filter := url.QueryEscape(`ouId eq "` + ouID + `"`)
		for _, r := range []struct{ path, key string }{
			{"/applications?filter=" + filter, "applications"},
			{"/roles?limit=100&filter=" + filter, "roles"},
			{"/flows?limit=100&filter=" + filter, "flows"},
			{"/connections?limit=100&filter=" + filter, "connections"},
		} {
			body := ts.requireStatus(http.StatusOK, http.MethodGet, r.path, nil)
			ts.Emptyf(listedIDs(body, r.key), "%s must list nothing", r.path)
		}
	}
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

func (ts *OUResourceTestSuite) TestVCInParentOU() {
	for _, vc := range []struct {
		path string
		body func(handle, ouID string) map[string]interface{}
	}{
		{"/openid4vci/credential-configurations", credentialConfigurationIn},
		{"/openid4vp/presentation-definitions", presentationDefinitionIn},
	} {
		id := ts.createAsRoot(vc.path, vc.body("ou_resource_parent_vc", ts.parentID))

		ts.requireStatus(http.StatusOK, http.MethodGet, vc.path+"/"+id, nil)
		ts.requireStatus(http.StatusForbidden, http.MethodPut, vc.path+"/"+id,
			vc.body("ou_resource_parent_vc", ts.parentID))
		ts.requireStatus(http.StatusForbidden, http.MethodDelete, vc.path+"/"+id, nil)

		body := ts.requireStatus(http.StatusOK, http.MethodGet, vc.path, nil)
		ts.Truef(listedArrayIDs(body)[id], "%s must list the parent OU's resource", vc.path)
	}
}

// A configuration version holds every OU's resources, so capturing and reading one needs the system
// permission. The org root does not reach them.
func (ts *OUResourceTestSuite) TestConfigurationVersionsNeedSystem() {
	ts.requireStatus(http.StatusForbidden, http.MethodPost, "/configuration-versions", map[string]interface{}{})
	ts.requireStatus(http.StatusForbidden, http.MethodGet, "/configuration-versions/current", nil)
}

// The administrator reads and lists the resource servers of the OU above its own, which its roles take
// permissions from, and can't change them.
func (ts *OUResourceTestSuite) TestResourceServerInParentOU() {
	id := ts.createAsRoot("/resource-servers", resourceServerIn("ou-resource-parent-rs", ts.parentID))

	ts.requireStatus(http.StatusOK, http.MethodGet, "/resource-servers/"+id, nil)
	ts.requireStatus(http.StatusOK, http.MethodGet, "/resource-servers/"+id+"/resources", nil)
	ts.requireStatus(http.StatusForbidden, http.MethodPut, "/resource-servers/"+id,
		resourceServerIn("ou-resource-parent-rs", ts.parentID))
	ts.requireStatus(http.StatusForbidden, http.MethodDelete, "/resource-servers/"+id, nil)
	ts.requireStatus(http.StatusForbidden, http.MethodPost, "/resource-servers/"+id+"/resources",
		map[string]interface{}{"name": "Refused", "handle": "refused"})

	body := ts.requireStatus(http.StatusOK, http.MethodGet, "/resource-servers?limit=100", nil)
	ids := listedIDs(body, "resourceServers")
	ts.True(ids[id], "the list must include the parent OU's resource server")
	ts.True(ids[ts.rsID], "the list must include the administrator's own resource server")
}

// A resource shared into the administrator's OU is listed with an OU filter and without one, and opening it
// reads it in that OU rather than refusing it.
func (ts *OUResourceTestSuite) TestSharedResourcesListedAndRead() {
	app, err := testutils.CreateApplication(testutils.Application{Name: "ou-resource-shared-app", OUID: ts.parentID})
	ts.Require().NoError(err)
	ts.cleanup("/applications/" + app)
	flow, err := testutils.CreateFlow(ouResourceFlow("ou-resource-shared-flow", ts.parentID))
	ts.Require().NoError(err)
	ts.cleanup("/flows/" + flow)
	role, err := testutils.CreateRole(testutils.Role{Name: "ou-resource-shared-list-role", OUID: ts.parentID})
	ts.Require().NoError(err)
	ts.cleanup("/roles/" + role)
	conn := ts.createAsRoot("/connections/http", httpConnectionIn("ou-resource-shared-http", ts.parentID))

	ownOU := url.QueryEscape(`ouId eq "` + ts.ou1ID + `"`)
	for _, r := range []struct {
		item, list, filtered, key string
	}{
		{"/applications/" + app, "/applications", "/applications?filter=" + ownOU, "applications"},
		{"/flows/" + flow, "/flows?limit=100", "/flows?limit=100&filter=" + ownOU, "flows"},
		{"/roles/" + role, "/roles?limit=100", "/roles?limit=100&filter=" + ownOU, "roles"},
		{"/connections/http/" + conn, "/connections?limit=100", "/connections?limit=100&filter=" + ownOU,
			"connections"},
	} {
		status, body := ts.root(http.MethodPost, r.item+"/sharing-policies",
			map[string]interface{}{"childUnits": map[string]interface{}{"mode": "all"}})
		ts.Require().Equalf(http.StatusCreated, status, "share %s: %s", r.item, body)
		ts.cleanup(r.item + "/sharing-policies/" + idOf(body))
		id := r.item[strings.LastIndex(r.item, "/")+1:]

		body = ts.requireStatus(http.StatusOK, http.MethodGet, r.list, nil)
		ts.Truef(listedIDs(body, r.key)[id], "%s without a filter must list the shared resource", r.list)
		body = ts.requireStatus(http.StatusOK, http.MethodGet, r.filtered, nil)
		ts.Truef(listedIDs(body, r.key)[id], "%s must list the shared resource", r.filtered)
		ts.requireStatus(http.StatusOK, http.MethodGet, r.item, nil)
	}
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

// A token that names no organization unit is refused on the Data Plane unless it holds system, which reaches every
// unit: org and its children act only in the unit a token names, so a token naming none reaches nothing.
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

	own, err := testutils.CreateFlow(ouResourceFlow("ou-resource-no-ou-flow", ts.ou1ID))
	ts.Require().NoError(err)
	ts.cleanup("/flows/" + own)

	signIn := func(username, scope string) *http.Client {
		token, err := testutils.ObtainAccessTokenWithPassword(clientID, testutils.OUSignInRedirectURI, scope,
			username, adminPassword, true, "", rsIdentifier)
		ts.Require().NoError(err)
		claims, err := testutils.DecodeJWT(token.AccessToken)
		ts.Require().NoError(err)
		ts.Require().Empty(claims.OUID, "the token must name no organization unit")
		return testutils.GetHTTPClientWithToken(token.AccessToken)
	}

	org := signIn(adminUsername, "org")
	for _, path := range []string{"/flows?limit=100", "/flows/" + own, "/applications", "/connections/http",
		"/openid4vci/credential-configurations", "/openid4vp/presentation-definitions"} {
		status, body := ts.send(org, http.MethodGet, path, nil)
		ts.Equalf(http.StatusForbidden, status, "%s: %s", path, body)
	}

	system := signIn(ts.createSystemAdmin("ou-resource-no-ou-system-admin"), "system")
	status, body := ts.send(system, http.MethodGet, "/flows?limit=100", nil)
	ts.Require().Equal(http.StatusOK, status, string(body))
	status, body = ts.send(system, http.MethodGet, "/flows/"+own, nil)
	ts.Equal(http.StatusOK, status, string(body))
}

// A system token taken in OU1 reaches every unit. Without a filter it lists another tree's resources, and a filter
// lists what the unit owns, then what is shared into it, or what another unit owns.
func (ts *OUResourceTestSuite) TestSystemTokenInAUnitListsEveryUnit() {
	client := ts.systemAdminClient()

	app, err := testutils.CreateApplication(testutils.Application{Name: "ou-resource-system-app", OUID: ts.parentID})
	ts.Require().NoError(err)
	ts.cleanup("/applications/" + app)
	otherApp, err := testutils.CreateApplication(testutils.Application{Name: "ou-resource-system-other-app",
		OUID: ts.ou2ID})
	ts.Require().NoError(err)
	ts.cleanup("/applications/" + otherApp)
	role, err := testutils.CreateRole(testutils.Role{Name: "ou-resource-system-role", OUID: ts.parentID})
	ts.Require().NoError(err)
	ts.cleanup("/roles/" + role)
	otherRole, err := testutils.CreateRole(testutils.Role{Name: "ou-resource-system-other-role", OUID: ts.ou2ID})
	ts.Require().NoError(err)
	ts.cleanup("/roles/" + otherRole)
	flow, err := testutils.CreateFlow(ouResourceFlow("ou-resource-system-flow", ts.parentID))
	ts.Require().NoError(err)
	ts.cleanup("/flows/" + flow)
	otherFlow, err := testutils.CreateFlow(ouResourceFlow("ou-resource-system-other-flow", ts.ou2ID))
	ts.Require().NoError(err)
	ts.cleanup("/flows/" + otherFlow)
	conn := ts.createAsRoot("/connections/http", httpConnectionIn("ou-resource-system-http", ts.parentID))
	otherConn := ts.createAsRoot("/connections/http", httpConnectionIn("ou-resource-system-other-http", ts.ou2ID))

	for _, r := range []struct {
		item, list, key, shared, other string
	}{
		{"/applications/", "/applications", "applications", app, otherApp},
		{"/roles/", "/roles", "roles", role, otherRole},
		{"/flows/", "/flows", "flows", flow, otherFlow},
		{"/connections/http/", "/connections", "connections", conn, otherConn},
	} {
		ts.Run(r.key, func() {
			status, body := ts.root(http.MethodPost, r.item+r.shared+"/sharing-policies",
				map[string]interface{}{"childUnits": map[string]interface{}{"mode": "all"}})
			ts.Require().Equalf(http.StatusCreated, status, "share %s: %s", r.shared, body)
			ts.cleanup(r.item + r.shared + "/sharing-policies/" + idOf(body))

			every := ts.listOwnership(client, r.list, r.key, "")
			ts.Contains(every, r.shared, "the unfiltered list holds the parent's resource")
			ts.Contains(every, r.other, "the unfiltered list holds the other tree's resource")

			inOU1 := ts.listOwnership(client, r.list, r.key, ts.ou1ID)
			ts.Require().Contains(inOU1, r.shared, "OU1 lists the resource shared into it")
			ts.False(inOU1[r.shared], "OU1 lists the parent's resource as shared")
			ts.NotContains(inOU1, r.other)

			inOU2 := ts.listOwnership(client, r.list, r.key, ts.ou2ID)
			ts.Require().Contains(inOU2, r.other, "a filter on another tree lists what it owns")
			ts.True(inOU2[r.other])
			ts.NotContains(inOU2, r.shared)
		})
	}
}

// systemAdminClient creates an administrator in OU1 holding system, and returns a client carrying its token.
func (ts *OUResourceTestSuite) systemAdminClient() *http.Client {
	ts.T().Helper()
	username := ts.createSystemAdmin("ou-resource-system-admin")
	token, err := ts.signIn.ObtainAccessToken(ts.ou1ID, "system", username, adminPassword, rsIdentifier)
	ts.Require().NoError(err, "obtain the system administrator's token")
	ts.Require().NotEmpty(token.AccessToken)
	return testutils.GetHTTPClientWithToken(token.AccessToken)
}

// createSystemAdmin creates an administrator in OU1 holding system, and returns its username.
func (ts *OUResourceTestSuite) createSystemAdmin(username string) string {
	ts.T().Helper()
	userID, err := testutils.CreateUser(testutils.User{
		Type: "ou-resource-admin-type",
		OUID: ts.ou1ID,
		Attributes: json.RawMessage(fmt.Sprintf(`{"username": %q, "password": %q}`,
			username, adminPassword)),
	})
	ts.Require().NoError(err, "create the system administrator %s", username)
	ts.cleanup("/users/" + userID)
	roleID, err := testutils.CreateRole(testutils.Role{
		Name: username + "-role",
		OUID: ts.ou1ID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: ts.rsID, Permissions: []string{"system"}},
		},
		Assignments: []testutils.Assignment{{ID: userID, Type: "user"}},
	})
	ts.Require().NoError(err, "create the system administrator's role")
	ts.cleanup("/roles/" + roleID)
	return username
}

// listOwnership reads every page of a list with the client, filtered to ouID when it is set, and returns whether
// each listed resource is listed as owned.
func (ts *OUResourceTestSuite) listOwnership(client *http.Client, list, key, ouID string) map[string]bool {
	ts.T().Helper()
	query := ""
	if ouID != "" {
		query = "&filter=" + url.QueryEscape(`ouId eq "`+ouID+`"`)
	}
	listed := map[string]bool{}
	for offset := 0; ; offset += 100 {
		status, body := ts.send(client, http.MethodGet, fmt.Sprintf("%s?limit=100&offset=%d%s", list, offset, query),
			nil)
		ts.Require().Equalf(http.StatusOK, status, "%s: %s", list, body)
		var page map[string]json.RawMessage
		ts.Require().NoError(json.Unmarshal(body, &page))
		var total int
		_ = json.Unmarshal(page["totalResults"], &total)
		var items []struct {
			ID          string `json:"id"`
			IsOwnedByOU *bool  `json:"isOwnedByOU"`
		}
		ts.Require().NoError(json.Unmarshal(page[key], &items))
		for _, item := range items {
			listed[item.ID] = item.IsOwnedByOU == nil || *item.IsOwnedByOU
		}
		if len(items) < 100 || offset+100 >= total {
			return listed
		}
	}
}

// User types and agent types are read down the hierarchy without being shared: a token from a child unit, holding
// org or only the view permission of a type, lists and reads the types its ancestors own, but not another tree's.
// Changing or deleting an ancestor's type stays with the unit that owns it.
func (ts *OUResourceTestSuite) TestTypesAreReadFromTheParent() {
	schema := map[string]interface{}{"name": map[string]interface{}{"type": "string"}}
	// A unit owns at most one agent type, and OU2 may own one already, so another tree gets a root of its own.
	otherRoot, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "ou-resource-types-other", Name: "OU Resource Types Other Tree",
	})
	ts.Require().NoError(err)
	ts.cleanups = append(ts.cleanups, func() { _ = testutils.DeleteOrganizationUnit(otherRoot) })
	parentAgentType := ts.createAsRoot("/agent-types", map[string]interface{}{"handle": "ou-resource-parent-agents",
		"displayName": "OU Resource Parent Agents", "ouId": ts.parentID, "schema": schema})
	otherAgentType := ts.createAsRoot("/agent-types", map[string]interface{}{"handle": "ou-resource-other-agents",
		"displayName": "OU Resource Other Agents", "ouId": otherRoot, "schema": schema})
	parentUserType := ts.createAsRoot("/user-types", map[string]interface{}{"handle": "ou-resource-parent-people",
		"displayName": "OU Resource Parent People", "ouId": ts.parentID, "schema": schema})
	otherUserType := ts.createAsRoot("/user-types", map[string]interface{}{"handle": "ou-resource-other-people",
		"displayName": "OU Resource Other People", "ouId": otherRoot, "schema": schema})

	clients := map[string]*http.Client{
		"org":                                  ts.adminClient,
		"org:usertype:view org:agenttype:view": ts.typeViewerClient(),
	}
	for scope, client := range clients {
		ts.Run(scope, func() {
			for _, r := range []struct{ path, parent, other string }{
				{"/user-types", parentUserType, otherUserType},
				{"/agent-types", parentAgentType, otherAgentType},
			} {
				listed := ts.listOwnership(client, r.path, "types", "")
				ts.Contains(listed, r.parent, "%s lists the parent's type", r.path)
				ts.NotContains(listed, r.other, "%s leaves out another tree's type", r.path)

				status, body := ts.send(client, http.MethodGet, r.path+"/"+r.parent, nil)
				ts.Equalf(http.StatusOK, status, "%s reads the parent's type: %s", r.path, body)
				status, body = ts.send(client, http.MethodGet, r.path+"/"+r.other, nil)
				ts.Equalf(http.StatusForbidden, status, "%s refuses another tree's type: %s", r.path, body)
			}
		})
	}

	ts.requireStatus(http.StatusForbidden, http.MethodPut, "/agent-types/"+parentAgentType,
		map[string]interface{}{"handle": "ou-resource-parent-agents", "displayName": "Renamed",
			"ouId": ts.parentID, "schema": schema})
	ts.requireStatus(http.StatusForbidden, http.MethodDelete, "/agent-types/"+parentAgentType, nil)
	ts.requireStatus(http.StatusForbidden, http.MethodPut, "/user-types/"+parentUserType,
		map[string]interface{}{"handle": "ou-resource-parent-people", "displayName": "Renamed",
			"ouId": ts.parentID, "schema": schema})
	ts.requireStatus(http.StatusForbidden, http.MethodDelete, "/user-types/"+parentUserType, nil)
}

// typeViewerClient creates an administrator in OU1 holding only the view permissions of user types and agent types,
// on a resource server of its own that declares them, and returns a client carrying its token.
func (ts *OUResourceTestSuite) typeViewerClient() *http.Client {
	ts.T().Helper()
	const username = "ou-resource-type-viewer"
	const identifier = "https://ou-resource.example.com/types"
	rsID, err := testutils.CreateOrgScopedResourceServer(ts.ou1ID, "OU Resource Types RS", identifier,
		"usertype", "agenttype")
	ts.Require().NoError(err)
	ts.cleanups = append(ts.cleanups, func() { _ = testutils.DeleteResourceServerWithChildren(rsID) })

	userID, err := testutils.CreateUser(testutils.User{
		Type: "ou-resource-admin-type",
		OUID: ts.ou1ID,
		Attributes: json.RawMessage(fmt.Sprintf(`{"username": %q, "password": %q}`,
			username, adminPassword)),
	})
	ts.Require().NoError(err)
	ts.cleanup("/users/" + userID)
	roleID, err := testutils.CreateRole(testutils.Role{
		Name: username + "-role",
		OUID: ts.ou1ID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: rsID, Permissions: []string{"org:usertype:view", "org:agenttype:view"}},
		},
		Assignments: []testutils.Assignment{{ID: userID, Type: "user"}},
	})
	ts.Require().NoError(err)
	ts.cleanup("/roles/" + roleID)

	token, err := ts.signIn.ObtainAccessToken(ts.ou1ID, "org:usertype:view org:agenttype:view", username,
		adminPassword, identifier)
	ts.Require().NoError(err)
	ts.Require().ElementsMatch([]string{"org:usertype:view", "org:agenttype:view"}, strings.Fields(token.Scope),
		"the token carries only the view scopes")
	return testutils.GetHTTPClientWithToken(token.AccessToken)
}
