/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

/*
Composite Mode Integration Test Suite

This suite validates the server behavior in composite mode, where declarative
resources (YAML-based, file-backed) coexist with runtime resources (database-backed).

Declarative resources are immutable and cannot be modified or deleted.
Runtime resources can be freely modified.
*/
package composite

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

type CompositeModeSuite struct {
	suite.Suite
	createdResources map[string][]string
}

func (suite *CompositeModeSuite) SetupSuite() {
	// Initialize suite
	suite.createdResources = make(map[string][]string)
}

func (suite *CompositeModeSuite) TearDownSuite() {
	// Delete only runtime resources (not declarative)
	for module, ids := range suite.createdResources {
		for _, id := range ids {
			switch module {
			case "application":
				suite.deleteResource(fmt.Sprintf("%s/applications/%s", testutils.TestServerURL, id))
			case "user":
				suite.deleteResource(fmt.Sprintf("%s/users/%s", testutils.TestServerURL, id))
			case "role":
				suite.deleteResource(fmt.Sprintf("%s/roles/%s", testutils.TestServerURL, id))
			case "organization_unit":
				suite.deleteResource(fmt.Sprintf("%s/organization-units/%s", testutils.TestServerURL, id))
			case "identity_provider":
				suite.deleteResource(fmt.Sprintf("%s/identity-providers/%s", testutils.TestServerURL, id))
			case "resource_server":
				suite.deleteResource(fmt.Sprintf("%s/resource-servers/%s", testutils.TestServerURL, id))
			case "flow":
				suite.deleteResource(fmt.Sprintf("%s/flows/%s", testutils.TestServerURL, id))
			case "user_type":
				suite.deleteResource(fmt.Sprintf("%s/user-types/%s", testutils.TestServerURL, id))
			case "theme":
				suite.deleteResource(fmt.Sprintf("%s/design/themes/%s", testutils.TestServerURL, id))
			case "layout":
				suite.deleteResource(fmt.Sprintf("%s/design/layouts/%s", testutils.TestServerURL, id))
			}
		}
	}
}

func (suite *CompositeModeSuite) deleteResource(url string) {
	client := testutils.GetHTTPClient()
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return
	}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}

// Helper function to track created resources
func (suite *CompositeModeSuite) trackResource(module, id string) {
	suite.createdResources[module] = append(suite.createdResources[module], id)
}

// Helper function to extract error code from response
func (suite *CompositeModeSuite) extractErrorCode(resp *http.Response) string {
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var errResp map[string]interface{}
	json.Unmarshal(body, &errResp)

	if code, ok := errResp["code"]; ok {
		return code.(string)
	}
	return ""
}

func (suite *CompositeModeSuite) getCollectionItems(path, field string) []map[string]interface{} {
	client := testutils.GetHTTPClient()
	resp, err := client.Get(fmt.Sprintf("%s%s", testutils.TestServerURL, path))
	suite.Require().NoError(err)
	suite.Require().Equal(http.StatusOK, resp.StatusCode, "collection endpoint should be accessible: %s", path)
	defer resp.Body.Close()

	if field == "" {
		var items []map[string]interface{}
		suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&items))
		return items
	}

	var listResp map[string]interface{}
	suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&listResp))

	rawItems, ok := listResp[field].([]interface{})
	suite.Require().True(ok, "collection response should include %s", field)

	items := make([]map[string]interface{}, 0, len(rawItems))
	for _, rawItem := range rawItems {
		item, ok := rawItem.(map[string]interface{})
		suite.Require().True(ok, "collection item should be an object in %s", path)
		items = append(items, item)
	}

	return items
}

func (suite *CompositeModeSuite) extractCollectionIDs(items []map[string]interface{}) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		if id, ok := item["id"].(string); ok && id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func (suite *CompositeModeSuite) findCollectionItemByID(items []map[string]interface{}, id string) map[string]interface{} {
	for _, item := range items {
		itemID, ok := item["id"].(string)
		if ok && itemID == id {
			return item
		}
	}
	return nil
}

func (suite *CompositeModeSuite) assertMergedCollectionContainsIDs(path, field, declarativeID, runtimeModule string) []map[string]interface{} {
	runtimeIDs := suite.createdResources[runtimeModule]
	suite.Require().NotEmpty(runtimeIDs, "runtime %s resource should be created before verifying merged collection", runtimeModule)

	items := suite.getCollectionItems(path, field)
	collectionIDs := suite.extractCollectionIDs(items)

	expectedIDs := append([]string{declarativeID}, runtimeIDs...)
	for _, expectedID := range expectedIDs {
		suite.Contains(collectionIDs, expectedID,
			"collection %s should include merged resource %s; got IDs: %v", path, expectedID, collectionIDs)
	}

	return items
}

func (suite *CompositeModeSuite) TestApplicationDeclarativeVisibility() {
	client := testutils.GetHTTPClient()
	resp, err := client.Get(fmt.Sprintf("%s/applications/decl-app-1", testutils.TestServerURL))
	suite.Require().NoError(err)
	suite.Equal(http.StatusOK, resp.StatusCode, "declarative application should be visible")
	resp.Body.Close()

	suite.assertMergedCollectionContainsIDs("/applications", "applications", "decl-app-1", "application")
}

func (suite *CompositeModeSuite) TestUserDeclarativeVisibility() {
	client := testutils.GetHTTPClient()
	resp, err := client.Get(fmt.Sprintf("%s/users/decl-user-1", testutils.TestServerURL))
	suite.Require().NoError(err)
	suite.Equal(http.StatusOK, resp.StatusCode, "declarative user should be visible")
	resp.Body.Close()

	suite.assertMergedCollectionContainsIDs("/users", "users", "decl-user-1", "user")
}

func (suite *CompositeModeSuite) TestRoleDeclarativeVisibility() {
	client := testutils.GetHTTPClient()
	resp, err := client.Get(fmt.Sprintf("%s/roles/decl-role-1", testutils.TestServerURL))
	suite.Require().NoError(err)
	suite.Equal(http.StatusOK, resp.StatusCode, "declarative role should be visible")
	resp.Body.Close()

	suite.assertMergedCollectionContainsIDs("/roles", "roles", "decl-role-1", "role")
}

func (suite *CompositeModeSuite) TestOrganizationUnitDeclarativeVisibility() {
	client := testutils.GetHTTPClient()
	resp, err := client.Get(fmt.Sprintf("%s/organization-units/decl-ou-1", testutils.TestServerURL))
	suite.Require().NoError(err)
	suite.Equal(http.StatusOK, resp.StatusCode, "declarative organization unit should be visible")
	resp.Body.Close()

	suite.assertMergedCollectionContainsIDs("/organization-units", "organizationUnits", "decl-ou-1", "organization_unit")
}

func (suite *CompositeModeSuite) TestIdentityProviderDeclarativeVisibility() {
	client := testutils.GetHTTPClient()
	resp, err := client.Get(fmt.Sprintf("%s/identity-providers/decl-idp-1", testutils.TestServerURL))
	suite.Require().NoError(err)
	suite.Equal(http.StatusOK, resp.StatusCode, "declarative identity provider should be visible")
	resp.Body.Close()

	suite.assertMergedCollectionContainsIDs("/identity-providers", "", "decl-idp-1", "identity_provider")
}

func (suite *CompositeModeSuite) TestResourceServerDeclarativeVisibility() {
	client := testutils.GetHTTPClient()
	resp, err := client.Get(fmt.Sprintf("%s/resource-servers/decl-rs-1", testutils.TestServerURL))
	suite.Require().NoError(err)
	suite.Equal(http.StatusOK, resp.StatusCode, "declarative resource server should be visible")
	resp.Body.Close()

	suite.assertMergedCollectionContainsIDs("/resource-servers", "resourceServers", "decl-rs-1", "resource_server")
}

func (suite *CompositeModeSuite) TestFlowDeclarativeVisibility() {
	client := testutils.GetHTTPClient()
	resp, err := client.Get(fmt.Sprintf("%s/flows/decl-flow-1", testutils.TestServerURL))
	suite.Require().NoError(err)
	suite.Equal(http.StatusOK, resp.StatusCode, "declarative flow should be visible")

	defer resp.Body.Close()
	var flowResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&flowResp)
	suite.True(flowResp["isReadOnly"].(bool), "declarative flow should be marked as read-only")

	items := suite.assertMergedCollectionContainsIDs("/flows?limit=50", "flows", "decl-flow-1", "flow")
	declFlow := suite.findCollectionItemByID(items, "decl-flow-1")
	suite.Require().NotNil(declFlow, "declarative flow should be present in merged collection")
	isReadOnly, ok := declFlow["isReadOnly"].(bool)
	suite.Require().True(ok, "merged flow item should include isReadOnly")
	suite.True(isReadOnly, "declarative flow should be marked as read-only in merged collection")
}

func (suite *CompositeModeSuite) TestEntityTypeDeclarativeVisibility() {
	client := testutils.GetHTTPClient()
	resp, err := client.Get(fmt.Sprintf("%s/user-types/decl-schema-1", testutils.TestServerURL))
	suite.Require().NoError(err)
	suite.Equal(http.StatusOK, resp.StatusCode, "declarative user type should be visible")
	resp.Body.Close()

	suite.assertMergedCollectionContainsIDs("/user-types", "types", "decl-schema-1", "user_type")
}

func (suite *CompositeModeSuite) TestThemeDeclarativeVisibility() {
	client := testutils.GetHTTPClient()
	resp, err := client.Get(fmt.Sprintf("%s/design/themes/decl-theme-1", testutils.TestServerURL))
	suite.Require().NoError(err)
	suite.Equal(http.StatusOK, resp.StatusCode, "declarative theme should be visible")
	resp.Body.Close()

	suite.assertMergedCollectionContainsIDs("/design/themes", "themes", "decl-theme-1", "theme")
}

func (suite *CompositeModeSuite) TestLayoutDeclarativeVisibility() {
	client := testutils.GetHTTPClient()
	resp, err := client.Get(fmt.Sprintf("%s/design/layouts/decl-layout-1", testutils.TestServerURL))
	suite.Require().NoError(err)
	suite.Equal(http.StatusOK, resp.StatusCode, "declarative layout should be visible")
	resp.Body.Close()

	suite.assertMergedCollectionContainsIDs("/design/layouts", "layouts", "decl-layout-1", "layout")
}

func (suite *CompositeModeSuite) TestApplicationCreate() {
	app := map[string]interface{}{
		"name":     "Test Runtime Application",
		"ouId":     "decl-ou-1",
		"template": "web",
		"url":      "https://test.example.com",
	}

	payload, _ := json.Marshal(app)
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/applications", testutils.TestServerURL),
		strings.NewReader(string(payload)),
	)
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	suite.Equal(http.StatusCreated, resp.StatusCode, "application should be created")

	// Parse server-assigned ID from response
	var appResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&appResp)
	resp.Body.Close()

	appID, ok := appResp["id"].(string)
	suite.Require().True(ok, "response should contain application ID")
	suite.Require().NotEmpty(appID, "application ID should not be empty")

	// Verify retrieval
	getReq, _ := http.NewRequest("GET", fmt.Sprintf("%s/applications/%s", testutils.TestServerURL, appID), nil)
	resp, err = client.Do(getReq)
	suite.Require().NoError(err)
	suite.Equal(http.StatusOK, resp.StatusCode, "created application should be retrievable")
	resp.Body.Close()

	suite.trackResource("application", appID)
}

func (suite *CompositeModeSuite) TestUserCreate() {
	user := map[string]interface{}{
		"type": "Declarative Test Schema",
		"ouId": "decl-ou-1",
		"attributes": map[string]interface{}{
			"username": fmt.Sprintf("runtime-user-%d", time.Now().Unix()),
			"email":    fmt.Sprintf("runtime-user-%d@example.com", time.Now().Unix()),
		},
	}

	payload, _ := json.Marshal(user)
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/users", testutils.TestServerURL),
		strings.NewReader(string(payload)),
	)
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	suite.Equal(http.StatusCreated, resp.StatusCode, "user should be created")

	// Parse server-assigned ID from response
	var userResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&userResp)
	resp.Body.Close()

	userID, ok := userResp["id"].(string)
	suite.Require().True(ok, "response should contain user ID")
	suite.Require().NotEmpty(userID, "user ID should not be empty")

	// Verify retrieval
	getReq, _ := http.NewRequest("GET", fmt.Sprintf("%s/users/%s", testutils.TestServerURL, userID), nil)
	resp, err = client.Do(getReq)
	suite.Require().NoError(err)
	suite.Equal(http.StatusOK, resp.StatusCode, "created user should be retrievable")
	resp.Body.Close()

	suite.trackResource("user", userID)
}

func (suite *CompositeModeSuite) TestRoleCreate() {
	timestamp := time.Now().Unix()
	roleID := fmt.Sprintf("runtime-role-%d", timestamp)

	role := map[string]interface{}{
		"name": "Test Runtime Role",
		"ouId": "decl-ou-1",
	}

	payload, _ := json.Marshal(role)
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/roles", testutils.TestServerURL),
		strings.NewReader(string(payload)),
	)
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	suite.Equal(http.StatusCreated, resp.StatusCode, "role should be created")

	// Parse server-assigned ID from response
	var roleResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&roleResp)
	resp.Body.Close()

	roleID, ok := roleResp["id"].(string)
	suite.Require().True(ok, "response should contain role ID")
	suite.Require().NotEmpty(roleID, "role ID should not be empty")

	// Verify retrieval
	getReq, _ := http.NewRequest("GET", fmt.Sprintf("%s/roles/%s", testutils.TestServerURL, roleID), nil)
	resp, err = client.Do(getReq)
	suite.Require().NoError(err)
	suite.Equal(http.StatusOK, resp.StatusCode, "created role should be retrievable")
	resp.Body.Close()

	suite.trackResource("role", roleID)
}

func (suite *CompositeModeSuite) TestOrganizationUnitCreate() {
	timestamp := time.Now().Unix()
	ouHandle := fmt.Sprintf("runtime-ou-%d", timestamp)

	ou := map[string]interface{}{
		"handle": ouHandle,
		"name":   "Test Runtime OU",
	}

	payload, _ := json.Marshal(ou)
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/organization-units", testutils.TestServerURL),
		strings.NewReader(string(payload)),
	)
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	suite.Equal(http.StatusCreated, resp.StatusCode, "organization unit should be created")

	var ouResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&ouResp)
	resp.Body.Close()

	ouID, ok := ouResp["id"].(string)
	suite.Require().True(ok, "response should contain organization unit ID")
	suite.Require().NotEmpty(ouID, "organization unit ID should not be empty")

	// Verify retrieval
	getReq, _ := http.NewRequest("GET", fmt.Sprintf("%s/organization-units/%s", testutils.TestServerURL, ouID), nil)
	resp, err = client.Do(getReq)
	suite.Require().NoError(err)
	suite.Equal(http.StatusOK, resp.StatusCode, "created organization unit should be retrievable")
	resp.Body.Close()

	suite.trackResource("organization_unit", ouID)
}

func (suite *CompositeModeSuite) TestIdentityProviderCreate() {
	idp := map[string]interface{}{
		"name": "Test Runtime IDP",
		"type": "OAUTH",
		"properties": []map[string]interface{}{
			{
				"name":      "client_id",
				"value":     "test-client-id",
				"is_secret": false,
			},
			{
				"name":      "client_secret",
				"value":     "test-client-secret",
				"is_secret": true,
			},
			{
				"name":      "redirect_uri",
				"value":     "https://localhost:3000/oidc/callback",
				"is_secret": false,
			},
			{
				"name":      "authorization_endpoint",
				"value":     "https://example.com/oauth2/authorize",
				"is_secret": false,
			},
			{
				"name":      "token_endpoint",
				"value":     "https://example.com/oauth2/token",
				"is_secret": false,
			},
			{
				"name":      "userinfo_endpoint",
				"value":     "https://example.com/oauth2/userinfo",
				"is_secret": false,
			},
			{
				"name":      "scopes",
				"value":     "openid,email,profile",
				"is_secret": false,
			},
		},
	}

	payload, _ := json.Marshal(idp)
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/identity-providers", testutils.TestServerURL),
		strings.NewReader(string(payload)),
	)
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	suite.Equal(http.StatusCreated, resp.StatusCode, "identity provider should be created")

	var idpResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&idpResp)
	resp.Body.Close()

	idpID, ok := idpResp["id"].(string)
	suite.Require().True(ok, "response should contain identity provider ID")
	suite.Require().NotEmpty(idpID, "identity provider ID should not be empty")

	// Verify retrieval
	getReq, _ := http.NewRequest("GET", fmt.Sprintf("%s/identity-providers/%s", testutils.TestServerURL, idpID), nil)
	resp, err = client.Do(getReq)
	suite.Require().NoError(err)
	suite.Equal(http.StatusOK, resp.StatusCode, "created identity provider should be retrievable")
	resp.Body.Close()

	suite.trackResource("identity_provider", idpID)
}

func (suite *CompositeModeSuite) TestResourceServerCreate() {
	timestamp := time.Now().Unix()
	rsID := fmt.Sprintf("runtime-rs-%d", timestamp)

	rs := map[string]interface{}{
		"name":       "Test Runtime Resource Server",
		"ouId":       "decl-ou-1",
		"identifier": rsID,
		"resources": []map[string]interface{}{
			{
				"name":   "Test Resource",
				"handle": "test",
				"actions": []map[string]interface{}{
					{
						"name":   "Read",
						"handle": "read",
					},
				},
			},
		},
	}

	payload, _ := json.Marshal(rs)
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/resource-servers", testutils.TestServerURL),
		strings.NewReader(string(payload)),
	)
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	suite.Equal(http.StatusCreated, resp.StatusCode, "resource server should be created")

	// Parse server-assigned ID from response
	var rsResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&rsResp)
	resp.Body.Close()

	rsID, ok := rsResp["id"].(string)
	suite.Require().True(ok, "response should contain resource server ID")
	suite.Require().NotEmpty(rsID, "resource server ID should not be empty")

	// Verify retrieval
	getReq, _ := http.NewRequest("GET", fmt.Sprintf("%s/resource-servers/%s", testutils.TestServerURL, rsID), nil)
	resp, err = client.Do(getReq)
	suite.Require().NoError(err)
	suite.Equal(http.StatusOK, resp.StatusCode, "created resource server should be retrievable")
	resp.Body.Close()

	suite.trackResource("resource_server", rsID)
}

func (suite *CompositeModeSuite) TestFlowCreate() {
	timestamp := time.Now().Unix()
	flowID := fmt.Sprintf("runtime-flow-%d", timestamp)

	flow := map[string]interface{}{
		"handle":   flowID,
		"name":     "Test Runtime Flow",
		"flowType": "AUTHENTICATION",
		"nodes": []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "prompt_credentials"},
			{
				"id":   "prompt_credentials",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{"identifier": "username", "type": "TEXT_INPUT", "required": true},
							{"identifier": "password", "type": "PASSWORD", "required": true},
						},
						"onSuccess": "end",
					},
				},
			},
			{"id": "end", "type": "END"},
		},
	}

	payload, _ := json.Marshal(flow)
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/flows", testutils.TestServerURL),
		strings.NewReader(string(payload)),
	)
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	suite.Equal(http.StatusCreated, resp.StatusCode, "flow should be created")

	// Parse server-assigned ID from response
	var flowResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&flowResp)
	resp.Body.Close()

	flowID, ok := flowResp["id"].(string)
	suite.Require().True(ok, "response should contain flow ID")
	suite.Require().NotEmpty(flowID, "flow ID should not be empty")

	// Verify retrieval
	getReq, _ := http.NewRequest("GET", fmt.Sprintf("%s/flows/%s", testutils.TestServerURL, flowID), nil)
	resp, err = client.Do(getReq)
	suite.Require().NoError(err)
	suite.Equal(http.StatusOK, resp.StatusCode, "created flow should be retrievable")
	resp.Body.Close()

	suite.trackResource("flow", flowID)
}

func (suite *CompositeModeSuite) TestEntityTypeCreate() {
	timestamp := time.Now().Unix()
	schemaID := fmt.Sprintf("runtime-schema-%d", timestamp)
	client := testutils.GetHTTPClient()

	// User type API requires ouId as a UUID; resolve the declarative OU handle first.
	ouResp, err := client.Get(fmt.Sprintf("%s/organization-units/decl-ou-1", testutils.TestServerURL))
	suite.Require().NoError(err)
	suite.Equal(http.StatusOK, ouResp.StatusCode)

	var ouBody map[string]interface{}
	json.NewDecoder(ouResp.Body).Decode(&ouBody)
	ouResp.Body.Close()

	schema := map[string]interface{}{
		"name": schemaID,
		"ouId": "decl-ou-1",
		"schema": map[string]interface{}{
			"email":    map[string]interface{}{"type": "string"},
			"username": map[string]interface{}{"type": "string", "required": true},
		},
	}

	payload, _ := json.Marshal(schema)

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/user-types", testutils.TestServerURL),
		strings.NewReader(string(payload)),
	)
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	suite.Equal(http.StatusCreated, resp.StatusCode, "user type should be created")
	location := resp.Header.Get("Location")

	// Parse server-assigned ID from response
	var schemaResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&schemaResp)
	resp.Body.Close()

	schemaID, ok := schemaResp["id"].(string)
	if !ok || schemaID == "" {
		parts := strings.Split(strings.TrimRight(location, "/"), "/")
		if len(parts) > 0 {
			schemaID = parts[len(parts)-1]
		}
	}
	suite.Require().NotEmpty(schemaID, "response should contain schema ID", schemaResp)
	suite.Require().NotEmpty(schemaID, "schema ID should not be empty")

	// Verify retrieval
	getReq, _ := http.NewRequest("GET", fmt.Sprintf("%s/user-types/%s", testutils.TestServerURL, schemaID), nil)
	resp, err = client.Do(getReq)
	suite.Require().NoError(err)
	suite.Equal(http.StatusOK, resp.StatusCode, "created user type should be retrievable")
	resp.Body.Close()

	suite.trackResource("user_type", schemaID)
}

func (suite *CompositeModeSuite) TestThemeCreate() {
	theme := map[string]interface{}{
		"handle":      fmt.Sprintf("runtime-theme-%d", time.Now().UnixNano()),
		"displayName": "Test Runtime Theme",
		"theme": map[string]interface{}{
			"primaryColor":   "#1976d2",
			"secondaryColor": "#dc004e",
		},
	}

	payload, _ := json.Marshal(theme)
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/design/themes", testutils.TestServerURL),
		strings.NewReader(string(payload)),
	)
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	suite.Equal(http.StatusCreated, resp.StatusCode, "theme should be created")

	var themeResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&themeResp)
	resp.Body.Close()

	themeID, ok := themeResp["id"].(string)
	suite.Require().True(ok, "response should contain theme ID")
	suite.Require().NotEmpty(themeID, "theme ID should not be empty")

	// Verify retrieval
	getReq, _ := http.NewRequest("GET", fmt.Sprintf("%s/design/themes/%s", testutils.TestServerURL, themeID), nil)
	resp, err = client.Do(getReq)
	suite.Require().NoError(err)
	suite.Equal(http.StatusOK, resp.StatusCode, "created theme should be retrievable")
	resp.Body.Close()

	suite.trackResource("theme", themeID)
}

func (suite *CompositeModeSuite) TestLayoutCreate() {
	layout := map[string]interface{}{
		"handle":      fmt.Sprintf("runtime-layout-%d", time.Now().UnixNano()),
		"displayName": "Test Runtime Layout",
		"layout": map[string]interface{}{
			"type": "centered",
		},
	}

	payload, _ := json.Marshal(layout)
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/design/layouts", testutils.TestServerURL),
		strings.NewReader(string(payload)),
	)
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	suite.Equal(http.StatusCreated, resp.StatusCode, "layout should be created")

	var layoutResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&layoutResp)
	resp.Body.Close()

	layoutID, ok := layoutResp["id"].(string)
	suite.Require().True(ok, "response should contain layout ID")
	suite.Require().NotEmpty(layoutID, "layout ID should not be empty")

	// Verify retrieval
	getReq, _ := http.NewRequest("GET", fmt.Sprintf("%s/design/layouts/%s", testutils.TestServerURL, layoutID), nil)
	resp, err = client.Do(getReq)
	suite.Require().NoError(err)
	suite.Equal(http.StatusOK, resp.StatusCode, "created layout should be retrievable")
	resp.Body.Close()

	suite.trackResource("layout", layoutID)
}

func (suite *CompositeModeSuite) TestApplicationDeclarativeUpdateReject() {
	client := testutils.GetHTTPClient()
	payload := map[string]interface{}{"name": "Updated"}
	jsonPayload, _ := json.Marshal(payload)

	req, _ := http.NewRequest("PUT", fmt.Sprintf("%s/applications/decl-app-1", testutils.TestServerURL), strings.NewReader(string(jsonPayload)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	suite.Equal(http.StatusBadRequest, resp.StatusCode, "update of declarative application should be rejected")

	errCode := suite.extractErrorCode(resp)
	suite.Equal("APP-1030", errCode, "error code should be APP-1030 for immutable application")
}

func (suite *CompositeModeSuite) TestApplicationDeclarativeDeleteReject() {
	client := testutils.GetHTTPClient()
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/applications/decl-app-1", testutils.TestServerURL), nil)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	suite.Equal(http.StatusBadRequest, resp.StatusCode, "delete of declarative application should be rejected")

	errCode := suite.extractErrorCode(resp)
	suite.Equal("APP-1030", errCode, "error code should be APP-1030 for immutable application")
}

func (suite *CompositeModeSuite) TestUserDeclarativeUpdateReject() {
	client := testutils.GetHTTPClient()
	payload := map[string]interface{}{
		"attributes": map[string]interface{}{
			"given_name": "Updated",
		},
	}
	jsonPayload, _ := json.Marshal(payload)

	req, _ := http.NewRequest("PUT", fmt.Sprintf("%s/users/decl-user-1", testutils.TestServerURL), strings.NewReader(string(jsonPayload)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	suite.Equal(http.StatusBadRequest, resp.StatusCode, "update of declarative user should be rejected")

	errCode := suite.extractErrorCode(resp)
	suite.Equal("USR-1025", errCode, "error code should be USR-1025 for immutable user")
}

func (suite *CompositeModeSuite) TestUserDeclarativeDeleteReject() {
	client := testutils.GetHTTPClient()
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/users/decl-user-1", testutils.TestServerURL), nil)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	suite.Equal(http.StatusBadRequest, resp.StatusCode, "delete of declarative user should be rejected")

	errCode := suite.extractErrorCode(resp)
	suite.Equal("USR-1025", errCode, "error code should be USR-1025 for immutable user")
}

func (suite *CompositeModeSuite) TestRoleDeclarativeUpdateReject() {
	client := testutils.GetHTTPClient()
	payload := map[string]interface{}{"name": "Updated", "ouId": "decl-ou-1"}
	jsonPayload, _ := json.Marshal(payload)

	req, _ := http.NewRequest("PUT", fmt.Sprintf("%s/roles/decl-role-1", testutils.TestServerURL), strings.NewReader(string(jsonPayload)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	suite.Equal(http.StatusBadRequest, resp.StatusCode, "update of declarative role should be rejected")

	errCode := suite.extractErrorCode(resp)
	suite.Equal("ROL-1013", errCode, "error code should be ROL-1013 for immutable role")
}

func (suite *CompositeModeSuite) TestRoleDeclarativeDeleteReject() {
	client := testutils.GetHTTPClient()
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/roles/decl-role-1", testutils.TestServerURL), nil)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	suite.Equal(http.StatusBadRequest, resp.StatusCode, "delete of declarative role should be rejected")

	errCode := suite.extractErrorCode(resp)
	suite.Equal("ROL-1013", errCode, "error code should be ROL-1013 for immutable role")
}

func (suite *CompositeModeSuite) TestOrganizationUnitDeclarativeUpdateReject() {
	client := testutils.GetHTTPClient()
	payload := map[string]interface{}{"name": "Updated"}
	jsonPayload, _ := json.Marshal(payload)

	req, _ := http.NewRequest("PUT", fmt.Sprintf("%s/organization-units/decl-ou-1", testutils.TestServerURL), strings.NewReader(string(jsonPayload)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	suite.Equal(http.StatusBadRequest, resp.StatusCode, "update of declarative organization unit should be rejected")

	errCode := suite.extractErrorCode(resp)
	suite.Equal("OU-1012", errCode, "error code should be OU-1012 for immutable organization unit")
}

func (suite *CompositeModeSuite) TestOrganizationUnitDeclarativeDeleteReject() {
	client := testutils.GetHTTPClient()
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/organization-units/decl-ou-1", testutils.TestServerURL), nil)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	suite.Equal(http.StatusBadRequest, resp.StatusCode, "delete of declarative organization unit should be rejected")

	errCode := suite.extractErrorCode(resp)
	suite.Equal("OU-1012", errCode, "error code should be OU-1012 for immutable organization unit")
}

func (suite *CompositeModeSuite) TestIdentityProviderDeclarativeUpdateReject() {
	client := testutils.GetHTTPClient()
	payload := map[string]interface{}{
		"name": "Updated",
		"type": "OAUTH",
		"properties": []map[string]interface{}{
			{"name": "client_id", "value": "test-client-id", "is_secret": false},
			{"name": "client_secret", "value": "test-client-secret", "is_secret": true},
			{"name": "redirect_uri", "value": "https://localhost:3000/oidc/callback", "is_secret": false},
			{"name": "authorization_endpoint", "value": "https://example.com/oauth2/authorize", "is_secret": false},
			{"name": "token_endpoint", "value": "https://example.com/oauth2/token", "is_secret": false},
			{"name": "userinfo_endpoint", "value": "https://example.com/oauth2/userinfo", "is_secret": false},
			{"name": "scopes", "value": "openid,email,profile", "is_secret": false},
		},
	}
	jsonPayload, _ := json.Marshal(payload)

	req, _ := http.NewRequest("PUT", fmt.Sprintf("%s/identity-providers/decl-idp-1", testutils.TestServerURL), strings.NewReader(string(jsonPayload)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	suite.Equal(http.StatusBadRequest, resp.StatusCode, "update of declarative identity provider should be rejected")

	errCode := suite.extractErrorCode(resp)
	suite.Equal("IDP-1010", errCode, "error code should be IDP-1010 for immutable identity provider")
}

func (suite *CompositeModeSuite) TestIdentityProviderDeclarativeDeleteReject() {
	client := testutils.GetHTTPClient()
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/identity-providers/decl-idp-1", testutils.TestServerURL), nil)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	suite.Equal(http.StatusBadRequest, resp.StatusCode, "delete of declarative identity provider should be rejected")

	errCode := suite.extractErrorCode(resp)
	suite.Equal("IDP-1010", errCode, "error code should be IDP-1010 for immutable identity provider")
}

func (suite *CompositeModeSuite) TestResourceServerDeclarativeUpdateReject() {
	client := testutils.GetHTTPClient()
	payload := map[string]interface{}{"name": "Updated"}
	jsonPayload, _ := json.Marshal(payload)

	req, _ := http.NewRequest("PUT", fmt.Sprintf("%s/resource-servers/decl-rs-1", testutils.TestServerURL), strings.NewReader(string(jsonPayload)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	suite.Equal(http.StatusBadRequest, resp.StatusCode, "update of declarative resource server should be rejected")

	errCode := suite.extractErrorCode(resp)
	suite.Equal("RES-1001", errCode, "error code should be RES-1001 for immutable resource server")
}

func (suite *CompositeModeSuite) TestResourceServerDeclarativeDeleteReject() {
	client := testutils.GetHTTPClient()
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/resource-servers/decl-rs-1", testutils.TestServerURL), nil)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	suite.Equal(http.StatusBadRequest, resp.StatusCode, "delete of declarative resource server should be rejected")

	errCode := suite.extractErrorCode(resp)
	suite.Equal("RES-1018", errCode, "error code should be RES-1018 for immutable resource server")
}

func (suite *CompositeModeSuite) TestFlowDeclarativeUpdateReject() {
	client := testutils.GetHTTPClient()
	payload := map[string]interface{}{
		"handle":   "decl-flow-1",
		"name":     "Updated",
		"flowType": "AUTHENTICATION",
		"nodes": []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "prompt_credentials"},
			{
				"id":   "prompt_credentials",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{"identifier": "username", "type": "TEXT_INPUT", "required": true},
							{"identifier": "password", "type": "PASSWORD", "required": true},
						},
						"onSuccess": "end",
					},
				},
			},
			{"id": "end", "type": "END"},
		},
	}
	jsonPayload, _ := json.Marshal(payload)

	req, _ := http.NewRequest("PUT", fmt.Sprintf("%s/flows/decl-flow-1", testutils.TestServerURL), strings.NewReader(string(jsonPayload)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	suite.Equal(http.StatusBadRequest, resp.StatusCode, "update of declarative flow should be rejected")

	errCode := suite.extractErrorCode(resp)
	suite.Equal("FLM-1017", errCode, "error code should be FLM-1017 for immutable flow")
}

func (suite *CompositeModeSuite) TestFlowDeclarativeDeleteReject() {
	client := testutils.GetHTTPClient()
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/flows/decl-flow-1", testutils.TestServerURL), nil)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	suite.Equal(http.StatusBadRequest, resp.StatusCode, "delete of declarative flow should be rejected")

	errCode := suite.extractErrorCode(resp)
	suite.Equal("FLM-1017", errCode, "error code should be FLM-1017 for immutable flow")
}

func (suite *CompositeModeSuite) TestEntityTypeDeclarativeUpdateReject() {
	client := testutils.GetHTTPClient()
	payload := map[string]interface{}{"name": "Updated"}
	jsonPayload, _ := json.Marshal(payload)

	req, _ := http.NewRequest("PUT", fmt.Sprintf("%s/user-types/decl-schema-1", testutils.TestServerURL), strings.NewReader(string(jsonPayload)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	suite.Equal(http.StatusForbidden, resp.StatusCode, "update of declarative user type should be rejected")

	errCode := suite.extractErrorCode(resp)
	suite.Equal("USRS-1008", errCode, "error code should be USRS-1008 for immutable user type")
}

func (suite *CompositeModeSuite) TestEntityTypeDeclarativeDeleteReject() {
	client := testutils.GetHTTPClient()
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/user-types/decl-schema-1", testutils.TestServerURL), nil)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	suite.Equal(http.StatusForbidden, resp.StatusCode, "delete of declarative user type should be rejected")

	errCode := suite.extractErrorCode(resp)
	suite.Equal("USRS-1008", errCode, "error code should be USRS-1008 for immutable user type")
}

func (suite *CompositeModeSuite) TestThemeDeclarativeUpdateReject() {
	client := testutils.GetHTTPClient()
	payload := map[string]interface{}{"displayName": "Updated"}
	jsonPayload, _ := json.Marshal(payload)

	req, _ := http.NewRequest("PUT", fmt.Sprintf("%s/design/themes/decl-theme-1", testutils.TestServerURL), strings.NewReader(string(jsonPayload)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	suite.Equal(http.StatusBadRequest, resp.StatusCode, "update of declarative theme should be rejected")

	errCode := suite.extractErrorCode(resp)
	suite.Equal("THM-1014", errCode, "error code should be THM-1014 for immutable theme")
}

func (suite *CompositeModeSuite) TestThemeDeclarativeDeleteReject() {
	client := testutils.GetHTTPClient()
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/design/themes/decl-theme-1", testutils.TestServerURL), nil)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	suite.Equal(http.StatusBadRequest, resp.StatusCode, "delete of declarative theme should be rejected")

	errCode := suite.extractErrorCode(resp)
	suite.Equal("THM-1014", errCode, "error code should be THM-1014 for immutable theme")
}

func (suite *CompositeModeSuite) TestLayoutDeclarativeUpdateReject() {
	client := testutils.GetHTTPClient()
	payload := map[string]interface{}{"displayName": "Updated"}
	jsonPayload, _ := json.Marshal(payload)

	req, _ := http.NewRequest("PUT", fmt.Sprintf("%s/design/layouts/decl-layout-1", testutils.TestServerURL), strings.NewReader(string(jsonPayload)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	suite.Equal(http.StatusBadRequest, resp.StatusCode, "update of declarative layout should be rejected")

	errCode := suite.extractErrorCode(resp)
	suite.Equal("LAY-1015", errCode, "error code should be LAY-1015 for immutable layout")
}

func (suite *CompositeModeSuite) TestLayoutDeclarativeDeleteReject() {
	client := testutils.GetHTTPClient()
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/design/layouts/decl-layout-1", testutils.TestServerURL), nil)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	suite.Equal(http.StatusBadRequest, resp.StatusCode, "delete of declarative layout should be rejected")

	errCode := suite.extractErrorCode(resp)
	suite.Equal("LAY-1015", errCode, "error code should be LAY-1015 for immutable layout")
}

// Run the test suite
func TestCompositeModeSuite(t *testing.T) {
	suite.Run(t, new(CompositeModeSuite))
}
