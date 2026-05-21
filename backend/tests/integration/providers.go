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

package integration

import (
	"context"
	"encoding/json"

	"github.com/thunder-id/thunderid/pkg/thunderidengine"
)

// FakeProviders returns in-memory provider implementations for engine integration tests.
func FakeProviders() thunderidengine.Providers {
	return thunderidengine.Providers{
		Client:         &fakeClientProvider{},
		Authn:          &fakeAuthnProvider{},
		Authz:          &fakeAuthzProvider{},
		Resource:       &fakeResourceProvider{},
		OU:             &fakeOUProvider{},
		IDP:            &fakeIDPProvider{},
		FlowDefinition: &fakeFlowDefinitionProvider{},
		Observability:  &fakeObservabilityProvider{},
		RuntimeStore:   newMemoryRuntimeStore(),
	}
}

type fakeClientProvider struct{}

func (f *fakeClientProvider) GetOAuthClientByClientID(
	_ context.Context, _ string,
) (*thunderidengine.OAuthClient, error) {
	return &thunderidengine.OAuthClient{ID: "app-1", ClientID: "test-client", OUID: "ou-1"}, nil
}

func (f *fakeClientProvider) GetTransitiveEntityGroups(
	_ context.Context, _ string,
) ([]thunderidengine.EntityGroup, error) {
	return nil, nil
}

func (f *fakeClientProvider) GetApplicationByID(_ context.Context, _ string) (*thunderidengine.Application, error) {
	return &thunderidengine.Application{ID: "app-1", OUID: "ou-1"}, nil
}

type fakeAuthnProvider struct{}

func (f *fakeAuthnProvider) AuthenticateUser(
	_ context.Context, _ thunderidengine.Credentials,
) (*thunderidengine.AuthnResult, error) {
	return &thunderidengine.AuthnResult{UserID: "user-1", OUID: "ou-1"}, nil
}

func (f *fakeAuthnProvider) GetUserAttributes(_ context.Context, _ string, _ []string) (map[string]interface{}, error) {
	return map[string]interface{}{"email": "user@example.com"}, nil
}

type fakeAuthzProvider struct{}

func (f *fakeAuthzProvider) IsAuthorized(_ context.Context, _, _, _ string) (bool, error) {
	return true, nil
}

type fakeResourceProvider struct{}

func (f *fakeResourceProvider) GetResource(_ context.Context, _ string) (*thunderidengine.Resource, error) {
	return &thunderidengine.Resource{ID: "res-1", Permission: "read"}, nil
}

type fakeOUProvider struct{}

func (f *fakeOUProvider) GetOU(_ context.Context, _ string) (*thunderidengine.OrganizationUnit, error) {
	return &thunderidengine.OrganizationUnit{ID: "ou-1", Name: "Root"}, nil
}

func (f *fakeOUProvider) GetOUAncestors(_ context.Context, _ string) ([]thunderidengine.OrganizationUnit, error) {
	return nil, nil
}

type fakeIDPProvider struct{}

func (f *fakeIDPProvider) GetIDPByID(_ context.Context, _ string) (*thunderidengine.IdentityProvider, error) {
	return &thunderidengine.IdentityProvider{ID: "idp-1", Name: "local", Type: "LOCAL"}, nil
}

func (f *fakeIDPProvider) GetIDPByName(_ context.Context, _ string) (*thunderidengine.IdentityProvider, error) {
	return &thunderidengine.IdentityProvider{ID: "idp-1", Name: "local", Type: "LOCAL"}, nil
}

type fakeFlowDefinitionProvider struct{}

func (f *fakeFlowDefinitionProvider) GetFlowByID(
	_ context.Context, id string,
) (*thunderidengine.FlowDefinition, error) {
	return &thunderidengine.FlowDefinition{
		ID: id, Handle: "login", Name: "Login", FlowType: thunderidengine.FlowTypeAuthentication,
		Nodes: json.RawMessage(`[]`),
	}, nil
}

func (f *fakeFlowDefinitionProvider) GetFlowByHandle(
	_ context.Context, _, handle string,
) (*thunderidengine.FlowDefinition, error) {
	return &thunderidengine.FlowDefinition{
		ID: "flow-1", Handle: handle, Name: "Login", FlowType: thunderidengine.FlowTypeAuthentication,
		Nodes: json.RawMessage(`[]`),
	}, nil
}

type fakeObservabilityProvider struct{}

func (f *fakeObservabilityProvider) IsEnabled() bool { return false }

func (f *fakeObservabilityProvider) PublishEvent(_ *thunderidengine.Event) {}

type memoryRuntimeStore struct {
	par map[string]thunderidengine.PushedAuthorizationRequest
}

func newMemoryRuntimeStore() *memoryRuntimeStore {
	return &memoryRuntimeStore{par: make(map[string]thunderidengine.PushedAuthorizationRequest)}
}

func (m *memoryRuntimeStore) Store(
	_ context.Context, parRequest thunderidengine.PushedAuthorizationRequest, _ int64,
) (string, error) {
	key := "urn:par:" + parRequest.ClientID
	m.par[key] = parRequest
	return key, nil
}

func (m *memoryRuntimeStore) Consume(
	_ context.Context, requestURI string,
) (thunderidengine.PushedAuthorizationRequest, bool, error) {
	req, ok := m.par[requestURI]
	if !ok {
		return thunderidengine.PushedAuthorizationRequest{}, false, nil
	}
	delete(m.par, requestURI)
	return req, true, nil
}

func (m *memoryRuntimeStore) AddRequest(context.Context, thunderidengine.AuthRequestContext) (string, error) {
	return "auth-req", nil
}

func (m *memoryRuntimeStore) GetRequest(context.Context, string) (bool, thunderidengine.AuthRequestContext, error) {
	return false, thunderidengine.AuthRequestContext{}, nil
}

func (m *memoryRuntimeStore) ClearRequest(context.Context, string) error { return nil }

func (m *memoryRuntimeStore) InsertAuthorizationCode(context.Context, thunderidengine.AuthorizationCode) error {
	return nil
}

func (m *memoryRuntimeStore) ConsumeAuthorizationCode(context.Context, string) (bool, error) {
	return true, nil
}

func (m *memoryRuntimeStore) GetAuthorizationCode(context.Context, string) (*thunderidengine.AuthorizationCode, error) {
	return &thunderidengine.AuthorizationCode{Code: "code"}, nil
}

func (m *memoryRuntimeStore) StoreFlowContext(context.Context, thunderidengine.FlowContext, int64) error {
	return nil
}

func (m *memoryRuntimeStore) GetFlowContext(context.Context, string) (*thunderidengine.FlowContext, error) {
	return &thunderidengine.FlowContext{ExecutionID: "exec-1"}, nil
}

func (m *memoryRuntimeStore) UpdateFlowContext(context.Context, thunderidengine.FlowContext) error {
	return nil
}

func (m *memoryRuntimeStore) DeleteFlowContext(context.Context, string) error { return nil }
