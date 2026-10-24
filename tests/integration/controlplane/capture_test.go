// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// captureGatewayKey is the key the control plane presents to the gateway that receives captures.
const captureGatewayKey = "capture-gateway-key"

// capturingGateway stands in for a data plane's variable store: it keeps every value the control
// plane sets in it, by collection and name, removes one the control plane deletes, and refuses a call
// without the key it was registered with.
type capturingGateway struct {
	server *httptest.Server
	mu     sync.Mutex
	values map[string]string
	// refuseFirst makes the first write of each value fail, as a store that is briefly down would.
	refuseFirst bool
	attempts    map[string]int
	// writes is the method of every accepted write of each value, in the order they arrived.
	writes map[string][]string
}

func newCapturingGateway() *capturingGateway {
	g := &capturingGateway{values: map[string]string{}, attempts: map[string]int{}, writes: map[string][]string{}}
	g.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("API-Key") != captureGatewayKey {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) != 2 || (r.Method != http.MethodPut && r.Method != http.MethodDelete) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method == http.MethodDelete {
			g.mu.Lock()
			key := parts[0] + "/" + parts[1]
			delete(g.values, key)
			g.writes[key] = append(g.writes[key], http.MethodDelete)
			g.mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
			return
		}
		var body struct {
			Value string `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		g.mu.Lock()
		key := parts[0] + "/" + parts[1]
		g.attempts[key]++
		if g.refuseFirst && g.attempts[key] == 1 {
			g.mu.Unlock()
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		g.values[key] = body.Value
		g.writes[key] = append(g.writes[key], http.MethodPut)
		g.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"` + parts[1] + `"}`))
	}))
	return g
}

// heldSoon waits for a value to be set under a name in a collection, since the control plane writes
// captured values in the background, and returns it and whether one arrived.
func (g *capturingGateway) heldSoon(collection, name string) (string, bool) {
	deadline := time.Now().Add(10 * time.Second)
	for {
		if value, ok := g.held(collection, name); ok || time.Now().After(deadline) {
			return value, ok
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// holdsValueSoon waits for any name in a collection to hold the value.
func (g *capturingGateway) holdsValueSoon(collection, value string) bool {
	deadline := time.Now().Add(10 * time.Second)
	for {
		if g.holdsValue(collection, value) || time.Now().After(deadline) {
			return g.holdsValue(collection, value)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// goneSoon waits for a name in a collection to hold no value, since the control plane removes released
// values in the background, and reports whether it was removed.
func (g *capturingGateway) goneSoon(collection, name string) bool {
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, ok := g.held(collection, name); !ok {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// writesAt returns the method of every accepted write of a value, in order.
func (g *capturingGateway) writesAt(collection, name string) []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]string(nil), g.writes[collection+"/"+name]...)
}

// writesSoon waits for a value to have received the given number of accepted writes.
func (g *capturingGateway) writesSoon(collection, name string, count int) []string {
	deadline := time.Now().Add(10 * time.Second)
	for {
		if writes := g.writesAt(collection, name); len(writes) >= count || time.Now().After(deadline) {
			return writes
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// attemptsAt counts the writes a value received, failed ones included.
func (g *capturingGateway) attemptsAt(collection, name string) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.attempts[collection+"/"+name]
}

// held returns the value set under a name in a collection, and whether one was.
func (g *capturingGateway) held(collection, name string) (string, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	value, ok := g.values[collection+"/"+name]
	return value, ok
}

// holdsValue reports whether any name in a collection holds the value.
func (g *capturingGateway) holdsValue(collection, value string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	for key, held := range g.values {
		if strings.HasPrefix(key, collection+"/") && held == value {
			return true
		}
	}
	return false
}

// withDefaultGateway registers a capturing gateway as this control plane's default for one test.
func (ts *ControlPlaneTestSuite) withDefaultGateway() *capturingGateway {
	ts.T().Helper()
	g := newCapturingGateway()
	ts.T().Cleanup(g.server.Close)
	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: g.server.Certificate().Raw})

	var registered struct {
		ID        string `json:"id"`
		IsDefault bool   `json:"isDefault"`
	}
	status, raw := ts.call(http.MethodPost, "/gateways", map[string]any{
		"name":          unique("capture-gateway"),
		"baseUrl":       g.server.URL,
		"key":           captureGatewayKey,
		"caCertificate": string(certificate),
		"isDefault":     true,
	}, &registered)
	ts.Require().Equal(http.StatusCreated, status, string(raw))
	ts.Require().True(registered.IsDefault, "the capturing gateway is not the default")
	ts.T().Cleanup(func() { ts.call(http.MethodDelete, "/gateways/"+registered.ID, nil, nil) })
	return g
}

// variableName is how an export names a resource's value: its type, its name and the field.
func variableName(resourceType, name, field string) string {
	return resourceType + "_" + strings.ToUpper(strings.NewReplacer(" ", "_", "-", "_").Replace(name)) + "_" + field
}

// An application written on the control plane leaves its client ID and secret in the default
// gateway's store, under the names its export refers to, so applying a version finds them.
func (ts *ControlPlaneTestSuite) TestAWrittenApplicationsValuesReachTheDefaultGateway() {
	g := ts.withDefaultGateway()
	name := unique("CP Capture App")
	clientID := unique("cp-capture-client")
	clientSecret := unique("cp-capture-secret")

	appID, err := testutils.CreateApplication(testutils.Application{
		OUID:         ts.ouID,
		Name:         name,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURIs: []string{"https://cp-capture.example.com/callback"},
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteApplication(appID) }()

	held, ok := g.heldSoon("variables", variableName("APPLICATION", name, "CLIENT_ID"))
	ts.True(ok, "the client ID was not captured")
	ts.Equal(clientID, held)
	held, ok = g.heldSoon("secrets", variableName("APPLICATION", name, "CLIENT_SECRET"))
	ts.True(ok, "the client secret was not captured")
	ts.Equal(clientSecret, held)
	held, ok = g.heldSoon("variables", variableName("APPLICATION", name, "REDIRECT_URIS"))
	ts.True(ok, "the redirect URIs were not captured")
	ts.JSONEq(`["https://cp-capture.example.com/callback"]`, held)
}

// A value the gateway's store refuses is written again a little later, so a store that is briefly
// down loses nothing.
func (ts *ControlPlaneTestSuite) TestACapturedValueRefusedOnceIsWrittenAgain() {
	g := ts.withDefaultGateway()
	g.refuseFirst = true
	name := unique("CP Retry App")
	clientID := unique("cp-retry-client")

	appID, err := testutils.CreateApplication(testutils.Application{
		OUID:         ts.ouID,
		Name:         name,
		ClientID:     clientID,
		ClientSecret: unique("cp-retry-secret"),
		RedirectURIs: []string{"https://cp-retry.example.com/callback"},
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteApplication(appID) }()

	held, ok := g.heldSoon("variables", variableName("APPLICATION", name, "CLIENT_ID"))
	ts.True(ok, "the client ID was not captured after the store refused it once")
	ts.Equal(clientID, held)
	ts.GreaterOrEqual(g.attemptsAt("variables", variableName("APPLICATION", name, "CLIENT_ID")), 2,
		"the refused write was not tried again")
}

// An application that signs no user in has no redirect URIs. Its export leaves the empty list as it
// is, so no gateway is asked for a value, and nothing is captured for it.
func (ts *ControlPlaneTestSuite) TestAnApplicationWithoutRedirectURIsAsksNoGatewayForThem() {
	g := ts.withDefaultGateway()
	name := unique("CP Machine App")
	clientID := unique("cp-machine-client")

	appID, err := testutils.CreateApplication(testutils.Application{
		OUID: ts.ouID,
		Name: name,
		Type: "m2m",
		InboundAuthConfig: []map[string]interface{}{{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                clientID,
				"clientSecret":            unique("cp-machine-secret"),
				"grantTypes":              []string{"client_credentials"},
				"tokenEndpointAuthMethod": "client_secret_basic",
			},
		}},
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteApplication(appID) }()

	held, ok := g.heldSoon("variables", variableName("APPLICATION", name, "CLIENT_ID"))
	ts.True(ok, "the client ID was not captured")
	ts.Equal(clientID, held)
	_, ok = g.held("variables", variableName("APPLICATION", name, "REDIRECT_URIS"))
	ts.False(ok, "a value was captured for redirect URIs the application does not have")

	var exported struct {
		Resources string `json:"resources"`
	}
	status, raw := ts.call(http.MethodPost, "/export", map[string]any{"applications": []string{appID}}, &exported)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	ts.NotContains(exported.Resources, variableName("APPLICATION", name, "REDIRECT_URIS"),
		"the export refers to redirect URIs the application does not have")
}

// A user written with a password leaves the password in the default gateway's secrets, never in its
// variables.
func (ts *ControlPlaneTestSuite) TestAWrittenUsersPasswordReachesTheDefaultGatewaysSecrets() {
	g := ts.withDefaultGateway()
	userType := unique("cp-capture-person")
	userTypeID, err := testutils.CreateUserType(testutils.UserType{
		Handle:      userType,
		DisplayName: "Capture person",
		OUID:        ts.ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string", "required": true, "unique": true},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteUserType(userTypeID) }()
	password := unique("Cp-Capture-Pass1!")
	attributes, err := json.Marshal(map[string]any{"username": unique("cp-capture-user"), "password": password})
	ts.Require().NoError(err)

	userID, err := testutils.CreateUser(testutils.User{OUID: ts.ouID, Type: userType, Attributes: attributes})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteUser(userID) }()

	ts.True(g.holdsValueSoon("secrets", password), "the password was not captured as a secret")
	ts.False(g.holdsValue("variables", password), "the password was captured as a variable")
}

// A connection backed by an identity provider leaves its client secret in the default gateway's
// secrets under the name its export refers to.
func (ts *ControlPlaneTestSuite) TestAWrittenConnectionsSecretReachesTheDefaultGateway() {
	g := ts.withDefaultGateway()
	name := unique("CP Capture Google")
	clientSecret := unique("cp-capture-google-secret")

	idpID, err := testutils.CreateIDP(testutils.IDP{
		Name: name,
		Type: "GOOGLE",
		Properties: []testutils.IDPProperty{
			{Name: "client_id", Value: unique("cp-capture-google-client")},
			{Name: "client_secret", Value: clientSecret, IsSecret: true},
			{Name: "scopes", Value: "openid email profile"},
			{Name: "redirect_uri", Value: "https://cp-capture.example.com/callback"},
		},
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteIDP(idpID) }()

	ts.True(g.holdsValueSoon("secrets", clientSecret), "the connection's client secret was not captured")
}

// An agent written with client credentials leaves them in the default gateway's store, under the
// names its export refers to.
func (ts *ControlPlaneTestSuite) TestAWrittenAgentsCredentialsReachTheDefaultGateway() {
	g := ts.withDefaultGateway()
	// The default agent type is shared, so it is pointed at this suite's unit for the test and put back.
	snapshot, err := testutils.SnapshotAgentType()
	ts.Require().NoError(err)
	defer func() { ts.NoError(testutils.RestoreAgentType(snapshot)) }()
	_, err = testutils.CreateAgentType(testutils.UserType{
		Handle:           "default",
		DisplayName:      "Default",
		OUID:             ts.ouID,
		SystemAttributes: &testutils.UserTypeSystemAttributes{Display: "name"},
		Schema: map[string]interface{}{
			"name":        map[string]interface{}{"type": "string"},
			"description": map[string]interface{}{"type": "string"},
		},
	})
	ts.Require().NoError(err)
	name := unique("CP Capture Agent")
	clientID := unique("cp-capture-agent-client")
	clientSecret := unique("cp-capture-agent-secret")

	agentID, err := testutils.CreateAgent(testutils.Agent{
		OUID:       ts.ouID,
		Type:       "default",
		Attributes: map[string]interface{}{"name": name, "description": "captures its credentials"},
		InboundAuthConfig: []testutils.AgentInboundAuthConfig{{
			Type: "oauth2",
			Config: &testutils.AgentOAuthConfig{
				ClientID:                clientID,
				ClientSecret:            clientSecret,
				GrantTypes:              []string{"client_credentials"},
				TokenEndpointAuthMethod: "client_secret_basic",
			},
		}},
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteAgent(agentID) }()

	held, ok := g.heldSoon("variables", variableName("AGENT", name, "CLIENT_ID"))
	ts.True(ok, "the agent's client ID was not captured")
	ts.Equal(clientID, held)
	held, ok = g.heldSoon("secrets", variableName("AGENT", name, "CLIENT_SECRET"))
	ts.True(ok, "the agent's client secret was not captured")
	ts.Equal(clientSecret, held)
}

// createResourceServer creates a resource server on the control plane and returns its id.
func (ts *ControlPlaneTestSuite) createResourceServer(name, identifier string) string {
	ts.T().Helper()
	var created struct {
		ID string `json:"id"`
	}
	status, raw := ts.call(http.MethodPost, "/resource-servers", map[string]any{
		"name":       name,
		"identifier": identifier,
		"ouId":       ts.ouID,
		"delimiter":  ".",
	}, &created)
	ts.Require().Equal(http.StatusCreated, status, string(raw))
	ts.T().Cleanup(func() { _ = testutils.DeleteResourceServer(created.ID) })
	return created.ID
}

// A resource server written on the control plane leaves its identifier in the default gateway's
// variables, under the name its export refers to, and a changed identifier replaces it there.
func (ts *ControlPlaneTestSuite) TestAWrittenResourceServersIdentifierReachesTheDefaultGateway() {
	g := ts.withDefaultGateway()
	name := unique("CP Capture Orders")
	identifier := "https://" + unique("cp-capture-orders") + ".example.com"
	variable := variableName("RESOURCE_SERVER", name, "IDENTIFIER")

	rsID := ts.createResourceServer(name, identifier)

	held, ok := g.heldSoon("variables", variable)
	ts.True(ok, "the identifier was not captured")
	ts.Equal(identifier, held)

	var exported struct {
		Resources string `json:"resources"`
	}
	status, raw := ts.call(http.MethodPost, "/export", map[string]any{"resourceServers": []string{rsID}}, &exported)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	ts.Contains(exported.Resources, "identifier: var:"+variable)
	ts.NotContains(exported.Resources, identifier, "the export carried the identifier")
	ts.Contains(exported.Resources, `delimiter: "."`)

	changed := "https://" + unique("cp-capture-orders-moved") + ".example.com"
	status, raw = ts.call(http.MethodPut, "/resource-servers/"+rsID, map[string]any{
		"name":       name,
		"identifier": changed,
		"ouId":       ts.ouID,
	}, nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	ts.True(g.holdsValueSoon("variables", changed), "the changed identifier did not replace the captured one")
	held, _ = g.held("variables", variable)
	ts.Equal(changed, held, "the changed identifier was captured under another name")
}

// createHTTPConnection creates an HTTP endpoint connection on the control plane and returns its id,
// removing it at the end of the test unless the test already did.
func (ts *ControlPlaneTestSuite) createHTTPConnection(body map[string]any) string {
	ts.T().Helper()
	var created struct {
		ID string `json:"id"`
	}
	status, raw := ts.call(http.MethodPost, "/connections/http", body, &created)
	ts.Require().Equal(http.StatusCreated, status, string(raw))
	ts.Require().NotEmpty(created.ID, string(raw))
	ts.T().Cleanup(func() { ts.call(http.MethodDelete, "/connections/http/"+created.ID, nil, nil) })
	return created.ID
}

// importHTTPConnections posts an import to the control plane and returns its single outcome.
func (ts *ControlPlaneTestSuite) importHTTPConnections(request testutils.ImportRequest) testutils.ImportItem {
	ts.T().Helper()
	var imported testutils.ImportResponse
	status, raw := ts.call(http.MethodPost, "/import", request, &imported)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	ts.Require().Len(imported.Results, 1, string(raw))
	return imported.Results[0]
}

// AC2.12: an API update that keeps the password captures it again, and one that drops it under the
// retention rule removes the copy, so no reference resolves to a password the connection no longer has.
func (ts *ControlPlaneTestSuite) TestAnHTTPConnectionsPasswordCopyIsRemovedWhenAnUpdateDropsIt() {
	g := ts.withDefaultGateway()
	name := unique("cp-http-basic")
	password := unique("cp-http-basic-secret")
	const url = "https://cp-http-basic.example.com/hook"
	basic := func(properties map[string]any) map[string]any {
		return map[string]any{"type": "basic", "properties": properties}
	}
	id := ts.createHTTPConnection(map[string]any{"name": name, "url": url,
		"authentication": basic(map[string]any{"username": "cp-http-user", "password": password})})
	secret := variableName("CONNECTION", name, "PASSWORD")

	held, ok := g.heldSoon("secrets", secret)
	ts.Require().True(ok, "the password was not captured")
	ts.Require().Equal(password, held, "the captured password is not the connection's")

	// Same origin and username, so the omitted password is kept; the copy must stay.
	status, raw := ts.call(http.MethodPut, "/connections/http/"+id, map[string]any{"name": name, "url": url + "/v2",
		"authentication": basic(map[string]any{"username": "cp-http-user"})}, nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	ts.Equal([]string{http.MethodPut, http.MethodPut}, g.writesSoon("secrets", secret, 2),
		"a kept password should be captured again, not released")
	held, ok = g.held("secrets", secret)
	ts.True(ok, "the kept password's copy was removed")
	ts.Equal(password, held, "the kept password's copy changed")

	status, raw = ts.call(http.MethodPut, "/connections/http/"+id, map[string]any{"name": name, "url": url,
		"authentication": map[string]any{"type": "none"}}, nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	ts.True(g.goneSoon("secrets", secret), "the dropped password's copy is still in the gateway's store")
	ts.Equal([]string{http.MethodPut, http.MethodPut, http.MethodDelete}, g.writesAt("secrets", secret),
		"the dropped password should be removed exactly once, after its captures")
}

// AC2.12: deleting a connection removes the copy of its token.
func (ts *ControlPlaneTestSuite) TestDeletingAnHTTPConnectionRemovesItsTokenCopy() {
	g := ts.withDefaultGateway()
	name := unique("cp-http-bearer")
	token := unique("cp-http-bearer-token")
	id := ts.createHTTPConnection(map[string]any{"name": name, "url": "https://cp-http-bearer.example.com/hook",
		"authentication": map[string]any{"type": "bearer", "properties": map[string]any{"token": token}}})
	secret := variableName("CONNECTION", name, "TOKEN")

	held, ok := g.heldSoon("secrets", secret)
	ts.Require().True(ok, "the token was not captured")
	ts.Require().Equal(token, held, "the captured token is not the connection's")

	status, raw := ts.call(http.MethodDelete, "/connections/http/"+id, nil, nil)
	ts.Require().Equal(http.StatusNoContent, status, string(raw))
	ts.True(g.goneSoon("secrets", secret), "the deleted connection's token is still in the gateway's store")
	ts.Equal([]string{http.MethodPut, http.MethodDelete}, g.writesAt("secrets", secret),
		"the token should be removed once, after its capture")
}

// AC2.12: an import writes HTTP connections outside the connection API, and keeps the copies in step
// as the API does: an upsert that drops one API-key header removes that header's copy and keeps the
// one it retains, and a deletion removes what is left.
func (ts *ControlPlaneTestSuite) TestAnImportRemovesTheCopiesOfSecretsAnHTTPConnectionNoLongerHolds() {
	g := ts.withDefaultGateway()
	name := unique("cp-http-apikey")
	tenantKey := unique("cp-http-tenant-key")
	regionKey := unique("cp-http-region-key")
	const url = "https://cp-http-apikey.example.com/hook"
	id := ts.createHTTPConnection(map[string]any{"name": name, "url": url, "authentication": map[string]any{
		"type": "api_key", "properties": map[string]any{"X-Tenant-Key": tenantKey, "X-Region-Key": regionKey}}})
	tenantSecret := variableName("CONNECTION", name, "X_TENANT_KEY")
	regionSecret := variableName("CONNECTION", name, "X_REGION_KEY")
	for secret, value := range map[string]string{tenantSecret: tenantKey, regionSecret: regionKey} {
		held, ok := g.heldSoon("secrets", secret)
		ts.Require().True(ok, "%s was not captured", secret)
		ts.Require().Equal(value, held, "%s was captured with another value", secret)
	}

	// The mask keeps the tenant key's stored value on the same origin; the region key is left out.
	document := "resource_type: connection\nid: " + id + "\ntype: http\nname: " + name + "\nurl: " + url +
		"\nauthentication:\n  type: api_key\n  properties:\n    X-Tenant-Key: \"******\"\n"
	outcome := ts.importHTTPConnections(testutils.ImportRequest{Content: document,
		Options: testutils.ImportOptions{Upsert: true, ContinueOnError: true, Target: "runtime"}})
	ts.Require().Equal("success", outcome.Status, "the upsert failed: %s %s", outcome.Code, outcome.Message)
	ts.Require().Equal("update", outcome.Operation, "the upsert should update the connection")
	ts.True(g.goneSoon("secrets", regionSecret), "the dropped header's copy is still in the gateway's store")
	ts.Equal([]string{http.MethodPut, http.MethodPut}, g.writesSoon("secrets", tenantSecret, 2),
		"the kept header should be captured again, not released")
	held, ok := g.held("secrets", tenantSecret)
	ts.True(ok, "the kept header's copy was removed")
	ts.Equal(tenantKey, held, "the kept header's copy changed")

	outcome = ts.importHTTPConnections(testutils.ImportRequest{
		Options:   testutils.ImportOptions{ContinueOnError: true, Target: "runtime"},
		Deletions: []testutils.ResourceDeletion{{ResourceType: "connection", ID: id}},
	})
	ts.Require().Equal("success", outcome.Status, "the deletion failed: %s %s", outcome.Code, outcome.Message)
	ts.True(g.goneSoon("secrets", tenantSecret), "the pruned connection's header is still in the gateway's store")
	ts.Equal([]string{http.MethodPut, http.MethodDelete}, g.writesAt("secrets", regionSecret),
		"the dropped header should be removed once")
	ts.Equal([]string{http.MethodPut, http.MethodPut, http.MethodDelete}, g.writesAt("secrets", tenantSecret),
		"the pruned header should be removed once, after its captures")
	status, raw := ts.call(http.MethodGet, "/connections/http/"+id, nil, nil)
	ts.Equal(http.StatusNotFound, status, "the pruned connection is still there: %s", raw)
}
