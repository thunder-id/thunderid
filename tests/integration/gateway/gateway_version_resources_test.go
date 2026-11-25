// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// document is a resource as a read returns it, decoded without a type of its own so that the read of
// the live resource and the applied configuration can be compared field by field.
type document = map[string]any

type appliedConfiguration struct {
	GatewayID string            `json:"gatewayId"`
	Version   string            `json:"version"`
	AppliedAt *time.Time        `json:"appliedAt"`
	Resources []appliedResource `json:"resources"`
	Skipped   []skippedResource `json:"skipped"`
}

type appliedResource struct {
	ResourceType string              `json:"resourceType"`
	ID           string              `json:"id"`
	Resource     document            `json:"resource"`
	Parts        map[string]document `json:"parts"`
}

type skippedResource struct {
	ResourceType string `json:"resourceType"`
	ID           string `json:"id"`
	Code         string `json:"code"`
	Reason       string `json:"reason"`
}

// A gateway's applied configuration shows each resource as the version holds it, not as this plane
// holds it now, and so does a part of it such as a group's members.
func (ts *GatewayVersionsTestSuite) TestTheAppliedConfigurationShowsTheVersionAsApplied() {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: fmt.Sprintf("viewed-%d", time.Now().UnixNano()), Name: "As Captured",
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteOrganizationUnit(ouID) }()
	memberID, err := testutils.CreateGroup(testutils.Group{Name: fmt.Sprintf("member-%d", time.Now().UnixNano()),
		OUID: ouID})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteGroup(memberID) }()
	groupID, err := testutils.CreateGroup(testutils.Group{Name: fmt.Sprintf("viewed-%d", time.Now().UnixNano()),
		OUID: ouID, Members: []testutils.Member{{Id: memberID, Type: "group"}}})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteGroup(groupID) }()

	captured := ts.capture("to view")
	ts.Require().True(ts.apply(captured.Version, false).Recorded)

	var live testutils.OrganizationUnit
	ts.Require().Equal(http.StatusOK, ts.call(http.MethodGet, "/organization-units/"+ouID, nil, &live))
	live.Name = "Changed Since"
	changed, err := json.Marshal(live)
	ts.Require().NoError(err)
	var changedDocument map[string]any
	ts.Require().NoError(json.Unmarshal(changed, &changedDocument))
	changedDocument["resource_type"] = "organization_unit"
	content, err := json.Marshal(changedDocument)
	ts.Require().NoError(err)
	ts.managedImport(testutils.ImportRequest{
		Content: string(content), Options: testutils.ImportOptions{Upsert: true},
	})

	applied := ts.appliedConfiguration(ts.gatewayID)
	ts.Equal(ts.gatewayID, applied.GatewayID)
	ts.Equal(captured.Version, applied.Version)
	ts.NotNil(applied.AppliedAt)

	unit := ts.shown(applied, "organization_unit", ouID)
	ts.Equal(ouID, unit.Resource["id"])
	ts.Equal("As Captured", unit.Resource["name"], "the gateway should show the unit as it was applied")
	ts.Equal(live.Handle, unit.Resource["handle"])
	ts.Empty(unit.Parts)

	members := ts.shown(applied, "group", groupID).Parts["members"]
	ts.EqualValues(1, members["totalResults"])
	ts.Equal([]any{memberID}, ts.ids(members["members"]))
}

// A gateway nothing was applied to runs nothing, and a gateway that is not there is not found.
func (ts *GatewayVersionsTestSuite) TestAGatewayNothingWasAppliedToShowsNothing() {
	applied := ts.appliedConfiguration(ts.gatewayID)
	ts.Equal(ts.gatewayID, applied.GatewayID)
	ts.Empty(applied.Version)
	ts.Nil(applied.AppliedAt)
	ts.NotNil(applied.Resources, "resources should be an empty list, not absent")
	ts.Empty(applied.Resources)
	ts.Empty(applied.Skipped)

	ts.Equal(http.StatusNotFound,
		ts.call(http.MethodGet, "/gateways/not-a-gateway/applied-configuration", nil, nil))
}

// An application and an agent with no address list show as their own reads return them: with no
// redirect URIs the export writes no template range, so the document is YAML. An application with
// an OAuth profile writes its address list as a template range, so it is skipped as not readable.
func (ts *GatewayVersionsTestSuite) TestAppliedApplicationsAndAgentsShowAsTheirReads() {
	suffix := uniqueSuffix()
	ouID := ts.createUnit("viewed-clients-" + suffix)
	embeddedID, err := testutils.CreateApplication(testutils.Application{
		Name: "viewed-embedded-" + suffix, Description: "Embedded", OUID: ouID, Embedded: true,
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteApplication(embeddedID) }()
	m2mID, err := testutils.CreateApplication(testutils.Application{
		Name: "viewed-m2m-" + suffix, Description: "Machine to machine", OUID: ouID, Type: "m2m",
		InboundAuthConfig: []map[string]interface{}{{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                "viewed-m2m-client-" + suffix,
				"clientSecret":            "viewed-m2m-secret-" + suffix,
				"grantTypes":              []string{"client_credentials"},
				"tokenEndpointAuthMethod": "client_secret_basic",
			},
		}},
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteApplication(m2mID) }()
	// An agent is placed within its agent type's organization unit tree, so it is of a type of the unit.
	agentTypeID, err := testutils.CreateNewAgentType(testutils.UserType{
		Handle: "viewed-agents-" + suffix, DisplayName: "Viewed Agents", OUID: ouID,
		Schema: map[string]interface{}{
			"name":          map[string]interface{}{"type": "string", "required": true},
			"modelProvider": map[string]interface{}{"type": "string"},
			"model":         map[string]interface{}{"type": "string"},
		},
	})
	ts.Require().NoError(err)
	// What an apply deployed here, which is this server, is removed only by the managed import.
	defer ts.removeManaged("agent_type", agentTypeID)
	agentID, err := testutils.CreateAgent(testutils.Agent{
		Type: "viewed-agents-" + suffix, OUID: ouID, Description: "Viewed agent",
		Attributes: map[string]any{"name": "viewed-agent-" + suffix, "modelProvider": "anthropic", "model": "viewed-model"},
		InboundAuthConfig: []testutils.AgentInboundAuthConfig{{
			Type: "oauth2",
			Config: &testutils.AgentOAuthConfig{
				ClientID:                "viewed-agent-client-" + suffix,
				ClientSecret:            "viewed-agent-secret-" + suffix,
				GrantTypes:              []string{"client_credentials"},
				TokenEndpointAuthMethod: "client_secret_basic",
			},
		}},
	})
	ts.Require().NoError(err)
	defer ts.removeManaged("agent", agentID)

	// The agent is read as it was captured: an import defaults its ID token's response type, which
	// its own create leaves unset, so the read after the apply has a field the version does not.
	live := ts.read("/agents/" + agentID)
	applied := ts.captureAndApply("applications and agents")

	embedded := ts.shown(applied, "application", embeddedID)
	embeddedRead := ts.read("/applications/" + embeddedID)
	ts.Contains(embeddedRead, "status", "an application read carries its access status")
	ts.Equal(withoutAccessState(embeddedRead), embedded.Resource,
		"an application with no OAuth profile shows as its own read")
	ts.assertNoAccessState(embedded.Resource, "application")
	ts.Empty(embedded.Parts)

	// An agent writes its address list only when it has one, so the agent's document is YAML. Its
	// client ID stands as the placeholder the export wrote, and its secret is not shown.
	agent := ts.shown(applied, "agent", agentID)
	viewed := agent.Resource
	ts.Equal(live["id"], viewed["id"])
	ts.Equal(live["name"], viewed["name"])
	ts.Equal(live["ouId"], viewed["ouId"])
	ts.Equal(live["attributes"], viewed["attributes"])
	ts.NotEmpty(viewed["attributes"])
	ts.Regexp(`^\{\{\.AGENT_[A-Z0-9_]+_CLIENT_ID\}\}$`, viewed["clientId"])
	// The export writes an empty list as no value, so a scope claim with no claims reads as null.
	ts.Contains(live, "status", "an agent read carries its access status")
	ts.Equal(emptyAsNil(withoutClient(withoutAccessState(live))), emptyAsNil(withoutClient(viewed)))
	ts.assertNoAccessState(viewed, "agent")
	ts.NotContains(fmt.Sprint(viewed), "-secret-"+suffix, "a client secret was shown")
	ts.Empty(agent.Parts)

	ts.Equal("GTW-1022", ts.skipped(applied, "application", m2mID).Code)
}

// A user shows as its own read returns it.
func (ts *GatewayVersionsTestSuite) TestAnAppliedUserShowsAsItsRead() {
	applied := ts.captureAndApply("users")

	var users struct {
		Users []testutils.User `json:"users"`
	}
	ts.Require().Equal(http.StatusOK, ts.call(http.MethodGet, "/users", nil, &users))
	ts.Require().NotEmpty(users.Users)
	var live testutils.User
	ts.Require().Equal(http.StatusOK, ts.call(http.MethodGet, "/users/"+users.Users[0].ID, nil, &live))
	viewed := ts.shown(applied, "user", live.ID).Resource
	ts.Equal(live.ID, viewed["id"])
	ts.Equal(live.Type, viewed["type"])
	ts.Equal(live.OUID, viewed["ouId"])
	var liveAttributes map[string]any
	ts.Require().NoError(json.Unmarshal(live.Attributes, &liveAttributes))
	ts.Equal(liveAttributes, viewed["attributes"], "the user's attributes should show as its own read gives them")
}

// A user type, agent type, user, group and role show as their own reads return them, and so do a
// group's members and a role's assignments.
func (ts *GatewayVersionsTestSuite) TestTheAppliedDirectoryShowsAsItsReads() {
	suffix := uniqueSuffix()
	ouID := ts.createUnit("viewed-directory-" + suffix)
	userTypeID, err := testutils.CreateUserType(testutils.UserType{
		Handle: "viewed_type_" + suffix, DisplayName: "Viewed Type", OUID: ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string", "required": true, "unique": true},
			"nickname": map[string]interface{}{"type": "string"},
		},
	})
	ts.Require().NoError(err)
	// What the apply deployed here, which is this server, is removed only by the managed import, whether or
	// not the test got as far as the apply.
	defer ts.removeManaged("user_type", userTypeID)
	// An agent type of this test's own: the default one, once deployed here, reads as read only and so is left
	// out of the next capture, which a control plane, that never holds what it deploys, does not do.
	agentTypeID, err := testutils.CreateNewAgentType(testutils.UserType{
		Handle: "viewed-agent-type-" + suffix, DisplayName: "Viewed Agent Type", OUID: ouID,
		Schema: map[string]interface{}{"name": map[string]interface{}{"type": "string"}},
	})
	ts.Require().NoError(err)
	defer ts.removeManaged("agent_type", agentTypeID)
	userID, err := testutils.CreateUser(testutils.User{
		OUID: ouID, Type: "viewed_type_" + suffix,
		Attributes: []byte(fmt.Sprintf(`{"username":"viewed-%s","nickname":"Viewed"}`, suffix)),
	})
	ts.Require().NoError(err)
	defer ts.removeManaged("user", userID)
	groupID, err := testutils.CreateGroup(testutils.Group{
		Name: "viewed-group-" + suffix, Description: "Viewed group", OUID: ouID,
		Members: []testutils.Member{{Id: userID, Type: "user"}},
	})
	ts.Require().NoError(err)
	defer ts.removeManaged("group", groupID)
	roleID, err := testutils.CreateRole(testutils.Role{
		Name: "viewed-role-" + suffix, Description: "Viewed role", OUID: ouID,
		Assignments: []testutils.Assignment{{ID: userID, Type: "user"}, {ID: groupID, Type: "group"}},
	})
	ts.Require().NoError(err)
	defer ts.removeManaged("role", roleID)

	applied := ts.captureAndApply("directory")

	userType := ts.shown(applied, "user_type", userTypeID)
	ts.Equal(ts.read("/user-types/"+userTypeID), userType.Resource)
	ts.Empty(userType.Parts)
	ts.Equal(ts.read("/agent-types/"+agentTypeID), ts.shown(applied, "agent_type", agentTypeID).Resource)
	user := ts.shown(applied, "user", userID)
	userRead := ts.read("/users/" + userID)
	ts.Contains(userRead, "status", "a user read carries its access status")
	ts.Equal(withoutAccessState(userRead), user.Resource)
	ts.assertNoAccessState(user.Resource, "user")
	ts.Empty(user.Parts)

	group := ts.shown(applied, "group", groupID)
	liveGroup := ts.read("/groups/" + groupID)
	for _, field := range []string{"id", "name", "description", "ouId"} {
		ts.Equal(liveGroup[field], group.Resource[field], field)
	}
	ts.Equal([]string{"members"}, partNames(group))
	members := group.Parts["members"]
	ts.EqualValues(1, members["totalResults"])
	ts.Equal(ts.ids(ts.read("/groups/" + groupID + "/members")["members"]), ts.ids(members["members"]))

	role := ts.shown(applied, "role", roleID)
	liveRole := ts.read("/roles/" + roleID)
	for _, field := range []string{"id", "name", "description", "ouId", "permissions"} {
		ts.Equal(liveRole[field], role.Resource[field], field)
	}
	ts.Equal([]string{"assignments"}, partNames(role))
	assignments := role.Parts["assignments"]
	ts.EqualValues(2, assignments["totalResults"])
	ts.ElementsMatch(ts.ids(ts.read("/roles/" + roleID + "/assignments")["assignments"]),
		ts.ids(assignments["assignments"]))
}

// A resource server shows as its own read returns it, and so do its resources, their children and
// their actions, each named by the id a declarative resource server serves it by.
func (ts *GatewayVersionsTestSuite) TestAnAppliedResourceServerShowsWithItsResourcesAndActions() {
	suffix := uniqueSuffix()
	ouID := ts.createUnit("viewed-rs-" + suffix)
	rsID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name: "viewed-rs-" + suffix, Description: "Viewed", Identifier: "urn:viewed:" + suffix, OUID: ouID,
	}, []testutils.Action{{Name: "Audit", Handle: "audit"}})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteResourceServerWithChildren(rsID) }()
	docsID, err := testutils.CreateResource(rsID, "Documents", "docs", "")
	ts.Require().NoError(err)
	draftsID, err := testutils.CreateResource(rsID, "Drafts", "drafts", docsID)
	ts.Require().NoError(err)
	rsBase := "/resource-servers/" + rsID
	ts.Require().Equal(http.StatusCreated, ts.call(http.MethodPost, rsBase+"/resources/"+docsID+"/actions",
		testutils.Action{Name: "Read", Handle: "read"}, nil))
	ts.Require().Equal(http.StatusCreated, ts.call(http.MethodPost, rsBase+"/resources/"+draftsID+"/actions",
		testutils.Action{Name: "Edit", Handle: "edit"}, nil))

	applied := ts.captureAndApply("resource server")
	server := ts.shown(applied, "resource_server", rsID)

	liveServer := ts.read(rsBase)
	for _, field := range []string{"id", "name", "description", "ouId", "delimiter"} {
		ts.Equal(liveServer[field], server.Resource[field], field)
	}
	// Each gateway holds its own identifier, so it stands as the placeholder the export wrote.
	ts.Regexp(`^\{\{\.RESOURCE_SERVER_[A-Z0-9_]+_IDENTIFIER\}\}$`, server.Resource["identifier"])

	delimiter, _ := liveServer["delimiter"].(string)
	builtDocsID := rsID + delimiter + "docs"
	builtDraftsID := builtDocsID + delimiter + "drafts"
	ts.ElementsMatch([]string{"resources", "actions",
		"resources/" + builtDocsID, "resources/" + builtDocsID + "/resources", "resources/" + builtDocsID + "/actions",
		"resources/" + builtDraftsID, "resources/" + builtDraftsID + "/resources",
		"resources/" + builtDraftsID + "/actions",
	}, partNames(server))

	topLevel := server.Parts["resources"]
	ts.EqualValues(1, topLevel["totalResults"])
	ts.Equal(ts.handles(ts.read(rsBase + "/resources")["resources"]), ts.handles(topLevel["resources"]))

	liveDocs, viewedDocs := ts.read(rsBase+"/resources/"+docsID), server.Parts["resources/"+builtDocsID]
	ts.Equal(builtDocsID, viewedDocs["id"])
	for _, field := range []string{"name", "handle", "permission"} {
		ts.Equal(liveDocs[field], viewedDocs[field], field)
	}
	liveDrafts, viewedDrafts := ts.read(rsBase+"/resources/"+draftsID), server.Parts["resources/"+builtDraftsID]
	ts.Equal(liveDrafts["permission"], viewedDrafts["permission"])
	ts.Equal(builtDocsID, viewedDrafts["parent"], "a child names its parent by the parent's built id")

	// A resource's children show as the live API lists them by parent.
	children := server.Parts["resources/"+builtDocsID+"/resources"]
	ts.EqualValues(1, children["totalResults"])
	ts.Equal(ts.handles(ts.read(rsBase + "/resources?parentId=" + docsID)["resources"]),
		ts.handles(children["resources"]))

	actions := server.Parts["resources/"+builtDocsID+"/actions"]
	ts.EqualValues(1, actions["totalResults"])
	ts.Equal(ts.handles(ts.read(rsBase + "/resources/" + docsID + "/actions")["actions"]),
		ts.handles(actions["actions"]))
	liveEdit := ts.list(ts.read(rsBase + "/resources/" + draftsID + "/actions")["actions"])[0]
	viewedEdits := ts.list(server.Parts["resources/"+builtDraftsID+"/actions"]["actions"])
	ts.Require().Len(viewedEdits, 1)
	ts.Equal(builtDraftsID+delimiter+"edit", viewedEdits[0]["id"])
	for _, field := range []string{"name", "handle", "permission"} {
		ts.Equal(liveEdit[field], viewedEdits[0][field], field)
	}

	// The resource server's own actions show as the live API lists them.
	serverActions := server.Parts["actions"]
	ts.EqualValues(1, serverActions["totalResults"])
	liveAudits := ts.list(ts.read(rsBase + "/actions")["actions"])
	ts.Require().Len(liveAudits, 1)
	liveAudit := liveAudits[0]
	viewedAudits := ts.list(serverActions["actions"])
	ts.Require().Len(viewedAudits, 1)
	ts.Equal(rsID+delimiter+"audit", viewedAudits[0]["id"])
	for _, field := range []string{"name", "handle", "permission"} {
		ts.Equal(liveAudit[field], viewedAudits[0][field], field)
	}
}

// Connections show as their vendors' reads return them, with the secrets masked as those reads mask
// them, never the secret itself.
func (ts *GatewayVersionsTestSuite) TestAppliedConnectionsShowWithTheirSecretsMasked() {
	suffix := uniqueSuffix()
	const secret = "viewed-connection-secret"
	ouID := ts.createUnit("viewed-connections-" + suffix)
	connections := []struct {
		vendor string
		body   map[string]any
		masked string
	}{
		{"google", map[string]any{"clientId": "g-client", "clientSecret": secret,
			"redirectUri": "https://localhost:3000/google/callback", "scopes": []string{"openid"}}, "clientSecret"},
		{"github", map[string]any{"clientId": "gh-client", "clientSecret": secret,
			"redirectUri": "https://localhost:3000/github/callback"}, "clientSecret"},
		{"oidc", map[string]any{"clientId": "oidc-client", "clientSecret": secret,
			"redirectUri":           "https://localhost:3000/oidc/callback",
			"authorizationEndpoint": "https://issuer.example.com/authorize",
			"tokenEndpoint":         "https://issuer.example.com/token"}, "clientSecret"},
		{"oauth", map[string]any{"clientId": "oauth-client", "clientSecret": secret,
			"redirectUri":           "https://localhost:3000/oauth/callback",
			"authorizationEndpoint": "https://issuer.example.com/authorize",
			"tokenEndpoint":         "https://issuer.example.com/token",
			"userInfoEndpoint":      "https://issuer.example.com/userinfo"}, "clientSecret"},
		{"twilio", map[string]any{"accountSid": "AC00000000000000000000000000000000", "authToken": secret,
			"senderId": "+15005550006"}, "authToken"},
		{"vonage", map[string]any{"apiKey": "vo-key", "apiSecret": secret, "senderId": "Viewed"}, "apiSecret"},
		{"sms-gateway", map[string]any{"url": "https://sms.example.com/send", "httpMethod": "POST"}, ""},
	}
	ids := make([]string, len(connections))
	for i, connection := range connections {
		connection.body["name"] = fmt.Sprintf("viewed-%s-%s", connection.vendor, suffix)
		connection.body["ouId"] = ouID
		ids[i] = ts.create("/connections/"+connection.vendor, connection.body)
		vendorPath := "/connections/" + connection.vendor + "/" + ids[i]
		ts.T().Cleanup(func() { ts.call(http.MethodDelete, vendorPath, nil, nil) })
	}

	applied := ts.captureAndApply("connections")
	ts.NotContains(ts.rawBody(http.MethodGet, "/gateways/"+ts.gatewayID+"/applied-configuration"), secret,
		"a connection's secret was shown")

	for i, connection := range connections {
		shown := ts.shown(applied, "connection", ids[i])
		ts.Equal(ts.read("/connections/"+connection.vendor+"/"+ids[i]), shown.Resource, connection.vendor)
		if connection.masked != "" {
			ts.Equal("******", shown.Resource[connection.masked], connection.vendor)
		}
		ts.Empty(shown.Parts, connection.vendor)
	}
}

// An authorization details type shows as its read returns it, with its processor credential masked.
func (ts *GatewayVersionsTestSuite) TestAnAppliedAuthorizationDetailTypeShowsWithItsCredentialMasked() {
	const secret = "viewed-processor-secret"
	typeID := ts.importResource("authorization_detail_type", map[string]any{
		"type": "https://viewed.example.com/" + uniqueSuffix(), "name": "Viewed",
		"schema": map[string]any{"type": "object",
			"properties": map[string]any{"amount": map[string]any{"type": "number"}}},
		"processor": map[string]any{"endpoint": "https://processor.example.com/rar",
			"authentication": map[string]any{"type": "bearer", "properties": map[string]any{"token": secret}}},
	})

	applied := ts.captureAndApply("authorization details type")
	ts.NotContains(ts.rawBody(http.MethodGet, "/gateways/"+ts.gatewayID+"/applied-configuration"), secret,
		"the processor credential was shown")

	shown := ts.shown(applied, "authorization_detail_type", typeID)
	ts.Equal(ts.read("/authorization-detail-types/"+typeID), shown.Resource)
	ts.Empty(shown.Parts)
}

// A flow, theme, layout, translations and server configuration show as their own reads return them.
// A language's translations show one namespace at a time as a part, as its resolve read filters.
func (ts *GatewayVersionsTestSuite) TestAppliedFlowsDesignTranslationsAndServerConfigShow() {
	suffix := uniqueSuffix()
	flowID, err := testutils.CreateIsolatedAuthFlow("viewed-flow-" + suffix)
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteFlow(flowID) }()
	themeID := ts.importResource("theme", map[string]any{
		"handle": "viewed-theme-" + suffix, "displayName": "Viewed Theme", "description": "Viewed",
		"theme": map[string]any{"direction": "ltr"},
	})
	layoutID := ts.importResource("layout", map[string]any{
		"handle": "viewed-layout-" + suffix, "displayName": "Viewed Layout", "description": "Viewed",
		"layout": map[string]any{"type": "centered"},
	})
	const language, namespace = "de-AT", "integration-version-view"
	translations := "/i18n/languages/" + language + "/translations"
	ts.importResource("translation", map[string]any{
		"language":     language,
		"translations": map[string]map[string]string{namespace: {"greeting": "Servus", "farewell": "Pfiat di"}},
	})

	applied := ts.captureAndApply("flows, design, translations and server configuration")

	flow := ts.shown(applied, "flow", flowID)
	liveFlow := ts.read("/flows/" + flowID)
	for _, field := range []string{"id", "handle", "name", "flowType", "nodes"} {
		ts.Equal(liveFlow[field], flow.Resource[field], field)
	}
	ts.Empty(flow.Parts)
	theme, layout := ts.shown(applied, "theme", themeID), ts.shown(applied, "layout", layoutID)
	// The gateway here is this server, so the apply writes the theme and the layout back and moves
	// their updatedAt past the version's; everything else reads the same.
	ts.Equal(withoutUpdatedAt(ts.read("/design/themes/"+themeID)), withoutUpdatedAt(theme.Resource))
	ts.Empty(theme.Parts)
	ts.Equal(withoutUpdatedAt(ts.read("/design/layouts/"+layoutID)), withoutUpdatedAt(layout.Resource))
	ts.Empty(layout.Parts)

	translation := ts.shown(applied, "translation", language)
	ts.Equal([]string{"ns/" + namespace}, partNames(translation))
	ts.Equal(ts.read(translations+"/resolve?namespace="+url.QueryEscape(namespace)),
		translation.Parts["ns/"+namespace])
	ts.Equal(language, translation.Resource["language"])
	ts.EqualValues(2, translation.Resource["totalResults"], "a language holds only what its document says")

	var sections []string
	ts.Require().Equal(http.StatusOK, ts.call(http.MethodGet, "/server-config", nil, &sections))
	ts.Require().NotEmpty(sections)
	for _, section := range sections {
		// The gateway section sets how the control plane itself deploys, so it is not exported or
		// applied.
		if section == "gateway" {
			continue
		}
		shown := ts.shown(applied, "server_config", section)
		ts.Equal(ts.read("/server-config/" + section)["merged"], shown.Resource["merged"], section)
		ts.Equal(shown.Resource["merged"], shown.Resource["writable"], "an import writes a section to its writable layer")
		ts.Empty(shown.Parts, section)
	}
}

// A credential configuration and a presentation definition show as their own reads return them. The
// presentation definition's trusted authorities show by name, without their certificates.
func (ts *GatewayVersionsTestSuite) TestAppliedCredentialsShowAsTheirReads() {
	suffix := uniqueSuffix()
	ouHandle := "viewed-credentials-" + suffix
	ouID := ts.createUnit(ouHandle)
	validity := 3600
	configurationID, err := testutils.CreateCredentialConfiguration(testutils.CredentialConfiguration{
		Handle: "viewed-credential-" + suffix, OUID: ouID, Name: "Viewed Credential", Description: "Viewed",
		Format: "dc+sd-jwt", VCT: "urn:viewed:credential:" + suffix,
		Claims:          []testutils.ClaimMapping{{Name: "given_name", DisplayName: "Given Name"}},
		Display:         &testutils.CredentialDisplay{Locale: "en-US", LogoURI: "https://example.com/logo.png"},
		ValiditySeconds: &validity,
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteCredentialConfiguration(configurationID) }()
	certificate, err := testutils.NewSelfSignedCertificatePEM("Viewed Issuer CA", time.Now().Add(24*time.Hour))
	ts.Require().NoError(err)
	enforce := true
	definitionID, err := testutils.CreatePresentationDefinition(testutils.PresentationDefinition{
		Handle: "viewed-presentation-" + suffix, OUID: ouID, Name: "Viewed Presentation", Description: "Viewed",
		VCT: "urn:viewed:credential:" + suffix, Format: "dc+sd-jwt",
		RequestedClaims: []string{"given_name", "email"}, MandatoryClaims: []string{"given_name"},
		OptionalClaims: []string{"email"}, EnforceTrustedIssuer: &enforce,
		TrustedAuthorities: []testutils.TrustedAuthority{{Name: "viewed-issuer", Certificate: certificate}},
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeletePresentationDefinition(definitionID) }()

	applied := ts.captureAndApply("credentials")

	// The read names the unit by its handle as well, which the export does not carry.
	live := ts.read("/openid4vci/credential-configurations/" + configurationID)
	ts.Equal(ouHandle, live["ouHandle"])
	delete(live, "ouHandle")
	configuration := ts.shown(applied, "credential_configuration", configurationID)
	ts.Equal(live, configuration.Resource)
	ts.Empty(configuration.Parts)

	liveDefinition := ts.read("/openid4vp/presentation-definitions/" + definitionID)
	ts.Equal(ouHandle, liveDefinition["ouHandle"])
	delete(liveDefinition, "ouHandle")
	definition := ts.shown(applied, "presentation_definition", definitionID)
	ts.Equal(liveDefinition, definition.Resource)
	ts.Equal([]any{"viewed-issuer"}, definition.Resource["trustedAuthorities"])
	ts.Empty(definition.Parts)
}

func uniqueSuffix() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// captureAndApply captures this plane's configuration, applies it to the gateway and returns what the
// gateway then runs.
func (ts *GatewayVersionsTestSuite) captureAndApply(note string) appliedConfiguration {
	ts.T().Helper()
	captured := ts.capture(note)
	result := ts.apply(captured.Version, false)
	ts.Require().True(result.Recorded, "the apply was not recorded: %s", result.Import)
	applied := ts.appliedConfiguration(ts.gatewayID)
	ts.Require().Equal(captured.Version, applied.Version)
	return applied
}

func (ts *GatewayVersionsTestSuite) appliedConfiguration(gatewayID string) appliedConfiguration {
	ts.T().Helper()
	var applied appliedConfiguration
	ts.Require().Equal(http.StatusOK,
		ts.call(http.MethodGet, "/gateways/"+gatewayID+"/applied-configuration", nil, &applied))
	return applied
}

// shown finds a resource of the applied configuration, requiring it there.
func (ts *GatewayVersionsTestSuite) shown(applied appliedConfiguration, resourceType, id string) appliedResource {
	ts.T().Helper()
	for _, resource := range applied.Resources {
		if resource.ResourceType == resourceType && resource.ID == id {
			return resource
		}
	}
	for _, skipped := range applied.Skipped {
		if skipped.ResourceType == resourceType && skipped.ID == id {
			ts.FailNow(fmt.Sprintf("%s %s was skipped: %s %s", resourceType, id, skipped.Code, skipped.Reason))
		}
	}
	var held []string
	for _, resource := range applied.Resources {
		held = append(held, resource.ResourceType+"/"+resource.ID)
	}
	var master struct {
		Entries []map[string]any `json:"entries"`
	}
	ts.call(http.MethodGet, "/gateways/"+ts.gatewayID+"/master", nil, &master)
	var entries []string
	for _, entry := range master.Entries {
		if entry["resourceType"] == resourceType {
			entries = append(entries, fmt.Sprintf("%v", entry))
		}
	}
	ts.FailNow(fmt.Sprintf("%s %s is not in the applied configuration, which holds %v and skipped %v; the master has %v",
		resourceType, id, held, applied.Skipped, entries))
	return appliedResource{}
}

// skipped finds a document the applied configuration could not show, requiring it there.
func (ts *GatewayVersionsTestSuite) skipped(applied appliedConfiguration, resourceType, id string) skippedResource {
	ts.T().Helper()
	for _, skipped := range applied.Skipped {
		if skipped.ResourceType == resourceType && skipped.ID == id {
			return skipped
		}
	}
	ts.FailNow(fmt.Sprintf("%s %s is not skipped", resourceType, id))
	return skippedResource{}
}

func partNames(resource appliedResource) []string {
	names := make([]string, 0, len(resource.Parts))
	for name := range resource.Parts {
		names = append(names, name)
	}
	return names
}

// read requires a successful read and returns its body.
func (ts *GatewayVersionsTestSuite) read(path string) document {
	ts.T().Helper()
	var body document
	status := ts.call(http.MethodGet, path, nil, &body)
	ts.Require().Equal(http.StatusOK, status, "GET %s", path)
	// The gateway here is this server, so what an apply deployed is read only on it, which the applied
	// configuration, as the apply sent it, does not show: it shows the resource as it was before.
	if _, ok := body["isReadOnly"]; ok {
		body["isReadOnly"] = false
	}
	// Whether the reading organization unit owns the resource belongs to the read, not to the resource, so a
	// version does not hold it.
	delete(body, "isOwnedByOU")
	return body
}

// managedImport deploys through the managed import with the management API key, as a control plane does,
// and requires every result of it to have succeeded. What an apply deployed to this server, which is the
// gateway here, is read only on it: only the managed import changes or removes it.
func (ts *GatewayVersionsTestSuite) managedImport(request testutils.ImportRequest) {
	ts.T().Helper()
	request.Options.Target = "runtime"
	encoded, err := json.Marshal(request)
	ts.Require().NoError(err)
	req, err := http.NewRequest(http.MethodPost, testServerURL+"/import/managed", bytes.NewReader(encoded))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := testutils.GetManagementAPIKeyHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()
	payload, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, resp.StatusCode, string(payload))
	var response testutils.ImportResponse
	ts.Require().NoError(json.Unmarshal(payload, &response), string(payload))
	for _, result := range response.Results {
		ts.Require().Equal("success", result.Status, string(payload))
	}
}

// tryRemoveManaged removes a resource an apply deployed here through the managed import, and leaves it if
// that is refused.
func (ts *GatewayVersionsTestSuite) tryRemoveManaged(resourceType, id string) {
	ts.T().Helper()
	encoded, err := json.Marshal(testutils.ImportRequest{
		Options:   testutils.ImportOptions{Target: "runtime"},
		Deletions: []testutils.ResourceDeletion{{ResourceType: resourceType, ID: id}},
	})
	if err != nil {
		return
	}
	req, err := http.NewRequest(http.MethodPost, testServerURL+"/import/managed", bytes.NewReader(encoded))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if resp, err := testutils.GetManagementAPIKeyHTTPClient().Do(req); err == nil {
		_ = resp.Body.Close()
	}
}

// removeManagedUnits removes organization units an apply deployed here, which the API no longer deletes.
func (ts *GatewayVersionsTestSuite) removeManagedUnits(ids ...string) {
	ts.T().Helper()
	ts.removeManaged("organization_unit", ids...)
}

// removeManaged removes resources of one type an apply deployed here, which the API no longer deletes.
func (ts *GatewayVersionsTestSuite) removeManaged(resourceType string, ids ...string) {
	ts.T().Helper()
	var deletions []testutils.ResourceDeletion
	for _, id := range ids {
		deletions = append(deletions, testutils.ResourceDeletion{ResourceType: resourceType, ID: id})
	}
	ts.managedImport(testutils.ImportRequest{Deletions: deletions})
}

// create posts a resource, requires it created and returns its id.
func (ts *GatewayVersionsTestSuite) create(path string, body any) string {
	ts.T().Helper()
	var created struct {
		ID string `json:"id"`
	}
	ts.Require().Equal(http.StatusCreated, ts.call(http.MethodPost, path, body, &created), "POST %s", path)
	return created.ID
}

// importResource imports a resource a data plane takes only from an import with the management API key,
// requires it imported and returns its id. It is removed the same way when the test ends.
func (ts *GatewayVersionsTestSuite) importResource(resourceType string, document map[string]any) string {
	ts.T().Helper()
	id, err := testutils.ImportResource(resourceType, document, true)
	ts.Require().NoError(err, "import %s", resourceType)
	if resourceType == "translation" {
		id = document["language"].(string)
	}
	ts.T().Cleanup(func() { _ = testutils.DeleteImportedResource(resourceType, id) })
	return id
}

// createUnit creates an organization unit the test's resources live in, removed when the test ends.
func (ts *GatewayVersionsTestSuite) createUnit(handle string) string {
	ts.T().Helper()
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{Handle: handle, Name: handle})
	ts.Require().NoError(err)
	// Best effort, as the cleanup of every test is: what other tests left in the unit keeps it from going.
	ts.T().Cleanup(func() { ts.tryRemoveManaged("organization_unit", ouID) })
	return ouID
}

func (ts *GatewayVersionsTestSuite) list(value any) []document {
	ts.T().Helper()
	items, ok := value.([]any)
	ts.Require().True(ok, "not a list: %v", value)
	documents := make([]document, len(items))
	for i, item := range items {
		documents[i], ok = item.(document)
		ts.Require().True(ok, "not an object: %v", item)
	}
	return documents
}

func (ts *GatewayVersionsTestSuite) ids(value any) []any {
	ts.T().Helper()
	return ts.field(value, "id")
}

func (ts *GatewayVersionsTestSuite) handles(value any) []any {
	ts.T().Helper()
	return ts.field(value, "handle")
}

func (ts *GatewayVersionsTestSuite) field(value any, name string) []any {
	ts.T().Helper()
	items := ts.list(value)
	values := make([]any, len(items))
	for i, item := range items {
		values[i] = item[name]
	}
	return values
}

// accessStateKeys are the fields of a read the runtime keeps and a version never holds: the access
// status and the last login.
var accessStateKeys = []string{"status", "lastLoginAt"}

// withoutAccessState leaves the access state out of a read.
func withoutAccessState(read document) document {
	trimmed := make(document, len(read))
	for key, value := range read {
		if !slices.Contains(accessStateKeys, key) {
			trimmed[key] = value
		}
	}
	return trimmed
}

// assertNoAccessState requires a shown resource to carry none of the access state.
func (ts *GatewayVersionsTestSuite) assertNoAccessState(resource document, label string) {
	ts.T().Helper()
	for _, key := range accessStateKeys {
		ts.NotContains(resource, key, "%s: a version must not hold %s", label, key)
	}
}

// withoutUpdatedAt leaves out of a read the time it was last written, which an apply moves.
func withoutUpdatedAt(read document) document {
	trimmed := make(document, len(read))
	for key, value := range read {
		if key != "updatedAt" {
			trimmed[key] = value
		}
	}
	return trimmed
}

// withoutClient leaves out of a read the client credentials, which a version holds as placeholders.
func withoutClient(read document) document {
	trimmed := make(document, len(read))
	for key, value := range read {
		if key != "clientId" && key != "inboundAuthConfig" {
			trimmed[key] = value
		}
	}
	if inbound, ok := read["inboundAuthConfig"].([]any); ok {
		configs := make([]any, len(inbound))
		for i, entry := range inbound {
			entry, _ := entry.(document)
			config, _ := entry["config"].(document)
			stripped := make(document, len(config))
			for key, value := range config {
				if key != "clientId" && key != "clientSecret" {
					stripped[key] = value
				}
			}
			configs[i] = document{"type": entry["type"], "config": stripped}
		}
		trimmed["inboundAuthConfig"] = configs
	}
	return trimmed
}

// emptyAsNil reads an empty list as no value, at any depth, as an export writes it.
func emptyAsNil(value any) any {
	switch v := value.(type) {
	case document:
		normalized := make(document, len(v))
		for key, item := range v {
			normalized[key] = emptyAsNil(item)
		}
		return normalized
	case []any:
		if len(v) == 0 {
			return nil
		}
		normalized := make([]any, len(v))
		for i, item := range v {
			normalized[i] = emptyAsNil(item)
		}
		return normalized
	}
	return value
}
