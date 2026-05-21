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

package enginebridge

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/thunder-id/thunderid/pkg/thunderidengine"
)

func testProviders() thunderidengine.Providers {
	return thunderidengine.Providers{
		Client:         &testClientProvider{},
		Authn:          &testAuthnProvider{},
		Authz:          &testAuthzProvider{},
		Resource:       &testResourceProvider{},
		OU:             &testOUProvider{},
		IDP:            &testIDPProvider{},
		FlowDefinition: &testFlowDefinitionProvider{},
		Design:         &testDesignProvider{},
		I18n:           &testI18nProvider{},
		Role:           &testRoleProvider{},
		RuntimeStore:   newTestMemoryRuntimeStore(),
	}
}

type testDesignProvider struct{}

func (p *testDesignProvider) ResolveDesign(
	_ context.Context, _, _ string,
) (*thunderidengine.DesignResponse, error) {
	return &thunderidengine.DesignResponse{
		Theme:  json.RawMessage(`{}`),
		Layout: json.RawMessage(`{}`),
	}, nil
}

type testI18nProvider struct{}

func (p *testI18nProvider) ResolveTranslations(
	_ context.Context, language, namespace string,
) (*thunderidengine.TranslationsResponse, error) {
	return &thunderidengine.TranslationsResponse{
		Language:     language,
		TotalResults: 1,
		Translations: map[string]map[string]string{namespace: {"key": "value"}},
	}, nil
}

func (p *testI18nProvider) ListLanguages(_ context.Context) ([]string, error) {
	return []string{"en"}, nil
}

type testRoleProvider struct{}

func (p *testRoleProvider) GetUserRoles(
	_ context.Context, _ string, _ []string,
) ([]thunderidengine.Role, error) {
	return []thunderidengine.Role{{Name: "user"}}, nil
}

type testClientProvider struct{}

func (p *testClientProvider) GetOAuthClientByClientID(
	_ context.Context, _ string,
) (*thunderidengine.OAuthClient, error) {
	return &thunderidengine.OAuthClient{ID: "app-1", ClientID: "test-client", OUID: "ou-1"}, nil
}

func (p *testClientProvider) GetTransitiveEntityGroups(
	_ context.Context, _ string,
) ([]thunderidengine.EntityGroup, error) {
	return nil, nil
}

func (p *testClientProvider) GetApplicationByID(_ context.Context, _ string) (*thunderidengine.Application, error) {
	return &thunderidengine.Application{ID: "app-1", OUID: "ou-1"}, nil
}

type testAuthnProvider struct{}

func (p *testAuthnProvider) AuthenticateUser(
	_ context.Context, _ thunderidengine.Credentials,
) (*thunderidengine.AuthnResult, error) {
	return &thunderidengine.AuthnResult{UserID: "user-1", OUID: "ou-1"}, nil
}

func (p *testAuthnProvider) GetUserAttributes(_ context.Context, _ string, _ []string) (map[string]interface{}, error) {
	return map[string]interface{}{"email": "user@example.com"}, nil
}

type testAuthzProvider struct{}

func (p *testAuthzProvider) IsAuthorized(_ context.Context, _, _, _ string) (bool, error) {
	return true, nil
}

type testResourceProvider struct{}

func (p *testResourceProvider) GetResource(_ context.Context, _ string) (*thunderidengine.Resource, error) {
	return &thunderidengine.Resource{ID: "res-1", Permission: "read"}, nil
}

type testOUProvider struct{}

func (p *testOUProvider) GetOU(_ context.Context, _ string) (*thunderidengine.OrganizationUnit, error) {
	return &thunderidengine.OrganizationUnit{ID: "ou-1", Name: "Root"}, nil
}

func (p *testOUProvider) GetOUAncestors(_ context.Context, _ string) ([]thunderidengine.OrganizationUnit, error) {
	return nil, nil
}

type testIDPProvider struct{}

func (p *testIDPProvider) GetIDPByID(_ context.Context, _ string) (*thunderidengine.IdentityProvider, error) {
	return &thunderidengine.IdentityProvider{ID: "idp-1", Name: "local", Type: "LOCAL"}, nil
}

func (p *testIDPProvider) GetIDPByName(_ context.Context, _ string) (*thunderidengine.IdentityProvider, error) {
	return &thunderidengine.IdentityProvider{ID: "idp-1", Name: "local", Type: "LOCAL"}, nil
}

type testFlowDefinitionProvider struct{}

func (p *testFlowDefinitionProvider) GetFlowByID(
	_ context.Context, id string,
) (*thunderidengine.FlowDefinition, error) {
	return &thunderidengine.FlowDefinition{
		ID: id, Handle: "login", Name: "Login", FlowType: thunderidengine.FlowTypeAuthentication,
		Nodes: json.RawMessage(`[]`),
	}, nil
}

func (p *testFlowDefinitionProvider) GetFlowByHandle(
	_ context.Context, _, handle string,
) (*thunderidengine.FlowDefinition, error) {
	return &thunderidengine.FlowDefinition{
		ID: "flow-1", Handle: handle, Name: "Login", FlowType: thunderidengine.FlowTypeAuthentication,
		Nodes: json.RawMessage(`[]`),
	}, nil
}

type testMemoryRuntimeStore struct {
	mu    sync.Mutex
	par   map[string]thunderidengine.PushedAuthorizationRequest
	codes map[string]thunderidengine.AuthorizationCode
	flows map[string]thunderidengine.FlowContext
}

func newTestMemoryRuntimeStore() *testMemoryRuntimeStore {
	return &testMemoryRuntimeStore{
		par:   make(map[string]thunderidengine.PushedAuthorizationRequest),
		codes: make(map[string]thunderidengine.AuthorizationCode),
		flows: make(map[string]thunderidengine.FlowContext),
	}
}

func (m *testMemoryRuntimeStore) Store(
	_ context.Context, parRequest thunderidengine.PushedAuthorizationRequest, _ int64,
) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := "par-" + parRequest.ClientID
	m.par[parRequestURIPrefix+key] = parRequest
	return parRequestURIPrefix + key, nil
}

func (m *testMemoryRuntimeStore) Consume(
	_ context.Context, requestURI string,
) (thunderidengine.PushedAuthorizationRequest, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	req, ok := m.par[requestURI]
	if !ok {
		return thunderidengine.PushedAuthorizationRequest{}, false, nil
	}
	delete(m.par, requestURI)
	return req, true, nil
}

func (m *testMemoryRuntimeStore) AddRequest(context.Context, thunderidengine.AuthRequestContext) (string, error) {
	return "auth-req-key", nil
}

func (m *testMemoryRuntimeStore) GetRequest(context.Context, string) (bool, thunderidengine.AuthRequestContext, error) {
	return false, thunderidengine.AuthRequestContext{}, nil
}

func (m *testMemoryRuntimeStore) ClearRequest(context.Context, string) error { return nil }

func (m *testMemoryRuntimeStore) InsertAuthorizationCode(
	_ context.Context, code thunderidengine.AuthorizationCode,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.codes[code.Code] = code
	return nil
}

func (m *testMemoryRuntimeStore) ConsumeAuthorizationCode(_ context.Context, authCode string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.codes[authCode]; !ok {
		return false, nil
	}
	delete(m.codes, authCode)
	return true, nil
}

func (m *testMemoryRuntimeStore) GetAuthorizationCode(
	_ context.Context, authCode string,
) (*thunderidengine.AuthorizationCode, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	code, ok := m.codes[authCode]
	if !ok {
		return nil, nil
	}
	return &code, nil
}

func (m *testMemoryRuntimeStore) StoreFlowContext(
	_ context.Context, flowContext thunderidengine.FlowContext, _ int64,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.flows[flowContext.ExecutionID] = flowContext
	return nil
}

func (m *testMemoryRuntimeStore) GetFlowContext(
	_ context.Context, executionID string,
) (*thunderidengine.FlowContext, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	flow, ok := m.flows[executionID]
	if !ok {
		return nil, nil
	}
	return &flow, nil
}

func (m *testMemoryRuntimeStore) UpdateFlowContext(
	_ context.Context, flowContext thunderidengine.FlowContext,
) error {
	return m.StoreFlowContext(context.Background(), flowContext, 0)
}

func (m *testMemoryRuntimeStore) DeleteFlowContext(_ context.Context, executionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.flows, executionID)
	return nil
}
