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

package thunderidengine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type mockClientProvider struct{}

func (m *mockClientProvider) GetOAuthClientByClientID(_ context.Context, _ string) (*OAuthClient, error) {
	return &OAuthClient{ClientID: "client"}, nil
}

func (m *mockClientProvider) GetTransitiveEntityGroups(_ context.Context, _ string) ([]EntityGroup, error) {
	return nil, nil
}

func (m *mockClientProvider) GetApplicationByID(_ context.Context, _ string) (*Application, error) {
	return &Application{ID: "app"}, nil
}

type mockAuthnProvider struct{}

func (m *mockAuthnProvider) AuthenticateUser(_ context.Context, _ Credentials) (*AuthnResult, error) {
	return &AuthnResult{UserID: "user"}, nil
}

func (m *mockAuthnProvider) GetUserAttributes(_ context.Context, _ string, _ []string) (map[string]interface{}, error) {
	return map[string]interface{}{"email": "a@b.c"}, nil
}

type mockAuthzProvider struct{}

func (m *mockAuthzProvider) IsAuthorized(_ context.Context, _, _, _ string) (bool, error) {
	return true, nil
}

type mockResourceProvider struct{}

func (m *mockResourceProvider) GetResource(_ context.Context, _ string) (*Resource, error) {
	return &Resource{ID: "res"}, nil
}

type mockOUProvider struct{}

func (m *mockOUProvider) GetOU(_ context.Context, _ string) (*OrganizationUnit, error) {
	return &OrganizationUnit{ID: "ou"}, nil
}

func (m *mockOUProvider) GetOUAncestors(_ context.Context, _ string) ([]OrganizationUnit, error) {
	return nil, nil
}

type mockIDPProvider struct{}

func (m *mockIDPProvider) GetIDPByID(_ context.Context, _ string) (*IdentityProvider, error) {
	return &IdentityProvider{ID: "idp"}, nil
}

func (m *mockIDPProvider) GetIDPByName(_ context.Context, _ string) (*IdentityProvider, error) {
	return &IdentityProvider{Name: "idp"}, nil
}

type mockFlowDefinitionProvider struct{}

func (m *mockFlowDefinitionProvider) GetFlowByID(_ context.Context, _ string) (*FlowDefinition, error) {
	return &FlowDefinition{ID: "flow"}, nil
}

func (m *mockFlowDefinitionProvider) GetFlowByHandle(_ context.Context, _, _ string) (*FlowDefinition, error) {
	return &FlowDefinition{Handle: "login"}, nil
}

type mockDesignProvider struct{}

func (m *mockDesignProvider) ResolveDesign(_ context.Context, _, _ string) (*DesignResponse, error) {
	return &DesignResponse{}, nil
}

type mockI18nProvider struct{}

func (m *mockI18nProvider) ResolveTranslations(
	_ context.Context, language, _ string,
) (*TranslationsResponse, error) {
	return &TranslationsResponse{Language: language}, nil
}

func (m *mockI18nProvider) ListLanguages(_ context.Context) ([]string, error) {
	return []string{"en"}, nil
}

type mockRoleProvider struct{}

func (m *mockRoleProvider) GetUserRoles(_ context.Context, _ string, _ []string) ([]Role, error) {
	return []Role{{Name: "user"}}, nil
}

type mockObservabilityProvider struct{}

func (m *mockObservabilityProvider) IsEnabled() bool { return true }

func (m *mockObservabilityProvider) PublishEvent(_ *Event) {}

type mockRuntimeStore struct{}

func (m *mockRuntimeStore) Store(_ context.Context, _ PushedAuthorizationRequest, _ int64) (string, error) {
	return "key", nil
}

func (m *mockRuntimeStore) Consume(_ context.Context, _ string) (PushedAuthorizationRequest, bool, error) {
	return PushedAuthorizationRequest{}, true, nil
}

func (m *mockRuntimeStore) AddRequest(_ context.Context, _ AuthRequestContext) (string, error) {
	return "auth", nil
}

func (m *mockRuntimeStore) GetRequest(_ context.Context, _ string) (bool, AuthRequestContext, error) {
	return true, AuthRequestContext{}, nil
}

func (m *mockRuntimeStore) ClearRequest(_ context.Context, _ string) error { return nil }

func (m *mockRuntimeStore) InsertAuthorizationCode(_ context.Context, _ AuthorizationCode) error {
	return nil
}

func (m *mockRuntimeStore) ConsumeAuthorizationCode(_ context.Context, _ string) (bool, error) {
	return true, nil
}

func (m *mockRuntimeStore) GetAuthorizationCode(_ context.Context, _ string) (*AuthorizationCode, error) {
	return &AuthorizationCode{Code: "code"}, nil
}

func (m *mockRuntimeStore) StoreFlowContext(_ context.Context, _ FlowContext, _ int64) error {
	return nil
}

func (m *mockRuntimeStore) GetFlowContext(_ context.Context, _ string) (*FlowContext, error) {
	return &FlowContext{ExecutionID: "exec"}, nil
}

func (m *mockRuntimeStore) UpdateFlowContext(_ context.Context, _ FlowContext) error { return nil }

func (m *mockRuntimeStore) DeleteFlowContext(_ context.Context, _ string) error { return nil }

func TestProviderMocksSatisfyInterfaces(t *testing.T) {
	var (
		_ ClientProvider         = (*mockClientProvider)(nil)
		_ AuthnProvider          = (*mockAuthnProvider)(nil)
		_ AuthzProvider          = (*mockAuthzProvider)(nil)
		_ ResourceProvider       = (*mockResourceProvider)(nil)
		_ OUProvider             = (*mockOUProvider)(nil)
		_ IDPProvider            = (*mockIDPProvider)(nil)
		_ FlowDefinitionProvider = (*mockFlowDefinitionProvider)(nil)
		_ DesignProvider         = (*mockDesignProvider)(nil)
		_ I18nProvider           = (*mockI18nProvider)(nil)
		_ RoleProvider           = (*mockRoleProvider)(nil)
		_ ObservabilityProvider  = (*mockObservabilityProvider)(nil)
		_ RuntimeStore           = (*mockRuntimeStore)(nil)
	)
}

func TestProvidersComplete(t *testing.T) {
	require.False(t, ProvidersComplete(Providers{}))
	require.True(t, ProvidersComplete(Providers{
		Client:         &mockClientProvider{},
		Authn:          &mockAuthnProvider{},
		Authz:          &mockAuthzProvider{},
		Resource:       &mockResourceProvider{},
		OU:             &mockOUProvider{},
		IDP:            &mockIDPProvider{},
		FlowDefinition: &mockFlowDefinitionProvider{},
		Design:         &mockDesignProvider{},
		I18n:           &mockI18nProvider{},
		Role:           &mockRoleProvider{},
		RuntimeStore:   &mockRuntimeStore{},
	}))
}

func TestHostOnlyEnabled(t *testing.T) {
	hostOnly := true
	require.True(t, EngineConfig{HostOnly: &hostOnly}.HostOnlyEnabled())

	disabled := false
	cfg := EngineConfig{
		HostOnly: &disabled,
		Providers: Providers{
			Client:         &mockClientProvider{},
			Authn:          &mockAuthnProvider{},
			Authz:          &mockAuthzProvider{},
			Resource:       &mockResourceProvider{},
			OU:             &mockOUProvider{},
			IDP:            &mockIDPProvider{},
			FlowDefinition: &mockFlowDefinitionProvider{},
			Design:         &mockDesignProvider{},
			I18n:           &mockI18nProvider{},
			Role:           &mockRoleProvider{},
			RuntimeStore:   &mockRuntimeStore{},
		},
	}
	require.False(t, cfg.HostOnlyEnabled())
}

func TestEngineConfigDefaults(t *testing.T) {
	cfg := EngineConfig{}
	require.True(t, cfg.RegisterRoutesEnabled())

	disabled := false
	cfg.RegisterRoutes = &disabled
	require.False(t, cfg.RegisterRoutesEnabled())
}

func TestEngineInitializeWithoutBootstrapRegistration(t *testing.T) {
	prev := registeredBootstrap
	registeredBootstrap = nil
	t.Cleanup(func() { registeredBootstrap = prev })

	engine := New(EngineConfig{})
	err := engine.Initialize(nil)
	require.ErrorIs(t, err, ErrNotImplemented)
}
