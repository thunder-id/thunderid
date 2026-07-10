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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	joseconfig "github.com/thunder-id/thunderid/internal/system/jose/config"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/actorprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/authzmock"
	"github.com/thunder-id/thunderid/tests/mocks/consentprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/executormock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/flowexecmock"
	"github.com/thunder-id/thunderid/tests/mocks/observabilityprovidermock"
)

// fakeResourceProvider, fakeOUProvider, fakeDesignProvider, fakeI18nProvider, and fakeIDPProvider
// satisfy their respective provider interfaces via embedding, purely to serve as non-nil values in
// validateEngineContext tests where no method calls are exercised.
type fakeResourceProvider struct {
	providers.ResourceServerProvider
}
type fakeOUProvider struct {
	providers.OrganizationUnitProvider
}
type fakeDesignProvider struct{ providers.DesignProvider }
type fakeI18nProvider struct{ providers.I18nProvider }
type fakeIDPProvider struct{ providers.IDPProvider }

type EngineTestSuite struct {
	suite.Suite
}

func TestEngineSuite(t *testing.T) {
	suite.Run(t, new(EngineTestSuite))
}

func newTestObservabilityProvider(t *testing.T) providers.ObservabilityProvider {
	mockObs := observabilityprovidermock.NewObservabilityProviderMock(t)
	mockObs.On("IsEnabled").Return(false).Maybe()
	return mockObs
}

func newTestAuthzProvider(t *testing.T) providers.AuthorizationProvider {
	return authzmock.NewAuthorizationProviderMock(t)
}

func newTestExecutor(t *testing.T, name string) providers.Executor {
	mockExec := coremock.NewExecutorInterfaceMock(t)
	mockExec.On("GetName").Return(name).Maybe()
	mockExec.On("GetType").Return(providers.ExecutorTypeUtility).Maybe()
	return mockExec
}

func validEngineContext(t *testing.T) *engineContext {
	return &engineContext{
		serverHome:            "/tmp/server",
		serverConfig:          engineconfig.ServerConfig{Identifier: "test-server"},
		observabilitySvc:      newTestObservabilityProvider(t),
		authzProvider:         newTestAuthzProvider(t),
		actorProvider:         actorprovidermock.NewActorProviderMock(t),
		authnProvider:         managermock.NewAuthnProviderManagerMock(t),
		resourceProvider:      fakeResourceProvider{},
		ouProvider:            fakeOUProvider{},
		designResolveProvider: fakeDesignProvider{},
		flowProvider:          flowexecmock.NewFlowProviderMock(t),
		i18nProvider:          fakeI18nProvider{},
		idpProvider:           fakeIDPProvider{},
		consentProvider:       consentprovidermock.NewConsentProviderMock(t),
	}
}

func (suite *EngineTestSuite) TestValidateEngineContext() {
	suite.T().Run("valid context passes", func(t *testing.T) {
		assert.NoError(t, validateEngineContext(validEngineContext(t)))
	})

	suite.T().Run("missing server home", func(t *testing.T) {
		ctx := validEngineContext(t)
		ctx.serverHome = ""
		assert.ErrorContains(t, validateEngineContext(ctx), "server home directory")
	})

	suite.T().Run("missing server identifier", func(t *testing.T) {
		ctx := validEngineContext(t)
		ctx.serverConfig.Identifier = ""
		assert.ErrorContains(t, validateEngineContext(ctx), "server identifier")
	})

	suite.T().Run("missing observability provider", func(t *testing.T) {
		ctx := validEngineContext(t)
		ctx.observabilitySvc = nil
		assert.ErrorContains(t, validateEngineContext(ctx), "observability provider")
	})

	suite.T().Run("missing authorization provider", func(t *testing.T) {
		ctx := validEngineContext(t)
		ctx.authzProvider = nil
		assert.ErrorContains(t, validateEngineContext(ctx), "authorization provider")
	})

	suite.T().Run("missing actor provider", func(t *testing.T) {
		ctx := validEngineContext(t)
		ctx.actorProvider = nil
		assert.ErrorContains(t, validateEngineContext(ctx), "actor provider")
	})

	suite.T().Run("missing authn provider", func(t *testing.T) {
		ctx := validEngineContext(t)
		ctx.authnProvider = nil
		assert.ErrorContains(t, validateEngineContext(ctx), "authn provider")
	})

	suite.T().Run("missing resource provider", func(t *testing.T) {
		ctx := validEngineContext(t)
		ctx.resourceProvider = nil
		assert.ErrorContains(t, validateEngineContext(ctx), "resource server provider")
	})

	suite.T().Run("missing ou provider", func(t *testing.T) {
		ctx := validEngineContext(t)
		ctx.ouProvider = nil
		assert.ErrorContains(t, validateEngineContext(ctx), "organization unit provider")
	})

	suite.T().Run("missing design provider", func(t *testing.T) {
		ctx := validEngineContext(t)
		ctx.designResolveProvider = nil
		assert.ErrorContains(t, validateEngineContext(ctx), "design provider")
	})

	suite.T().Run("missing flow provider", func(t *testing.T) {
		ctx := validEngineContext(t)
		ctx.flowProvider = nil
		assert.ErrorContains(t, validateEngineContext(ctx), "flow provider")
	})

	suite.T().Run("missing i18n provider", func(t *testing.T) {
		ctx := validEngineContext(t)
		ctx.i18nProvider = nil
		assert.ErrorContains(t, validateEngineContext(ctx), "i18n provider")
	})

	suite.T().Run("missing idp provider", func(t *testing.T) {
		ctx := validEngineContext(t)
		ctx.idpProvider = nil
		assert.ErrorContains(t, validateEngineContext(ctx), "idp provider")
	})

	suite.T().Run("missing consent provider", func(t *testing.T) {
		ctx := validEngineContext(t)
		ctx.consentProvider = nil
		assert.ErrorContains(t, validateEngineContext(ctx), "consent provider")
	})
}

func (suite *EngineTestSuite) TestApplyCustomExecutors() {
	suite.T().Run("no custom executors is a no-op", func(t *testing.T) {
		ctx := &engineContext{execRegistry: executormock.NewExecutorRegistryInterfaceMock(t)}
		assert.NoError(t, ctx.applyCustomExecutors())
	})

	suite.T().Run("nil executor registry returns error", func(t *testing.T) {
		ctx := &engineContext{
			customExecutors: map[string]providers.Executor{
				"custom": newTestExecutor(t, "custom"),
			},
		}
		assert.ErrorContains(t, ctx.applyCustomExecutors(), "executor registry is nil")
	})

	suite.T().Run("registers custom executors", func(t *testing.T) {
		reg := executormock.NewExecutorRegistryInterfaceMock(t)
		ex := newTestExecutor(t, "MyExecutor")
		reg.On("RegisterExecutor", "MyExecutor", ex).Once()
		ctx := &engineContext{
			execRegistry:    reg,
			customExecutors: map[string]providers.Executor{"MyExecutor": ex},
		}
		require.NoError(t, ctx.applyCustomExecutors())
	})
}

func (suite *EngineTestSuite) TestEngineOptions() {
	var ctx engineContext

	serverCfg := engineconfig.ServerConfig{Identifier: "srv-1"}
	cacheCfg := engineconfig.CacheConfig{Type: "memory"}
	oauthCfg := engineconfig.OAuthConfig{}
	jwtCfg := engineconfig.JWTConfig{Issuer: "issuer"}
	flowCfg := engineconfig.FlowConfig{Store: "memory"}
	obsCfg := engineconfig.ObservabilityConfig{Enabled: true}
	customExec := map[string]providers.Executor{"custom": newTestExecutor(suite.T(), "custom")}

	opts := []Option{
		WithServerHome("/home"),
		WithServerConfig(serverCfg),
		WithCacheConfig(cacheCfg),
		WithOAuthConfig(oauthCfg),
		WithJWTConfig(jwtCfg),
		WithFlowConfig(flowCfg),
		WithObservabilityConfig(obsCfg),
		WithActorProvider(nil),
		WithAuthnProvider(nil),
		WithResourceProvider(nil),
		WithOUProvider(nil),
		WithDesignResolveProvider(nil),
		WithFlowProvider(nil),
		WithI18nProvider(nil),
		WithConsentProvider(nil),
		WithCustomExecutors(customExec),
		WithObservabilityProvider(newTestObservabilityProvider(suite.T())),
		WithAuthorizationProvider(newTestAuthzProvider(suite.T())),
	}
	for _, opt := range opts {
		opt(&ctx)
	}

	assert.Equal(suite.T(), "/home", ctx.serverHome)
	assert.Equal(suite.T(), serverCfg, ctx.serverConfig)
	assert.Equal(suite.T(), cacheCfg, ctx.cacheConfig)
	assert.Equal(suite.T(), oauthCfg, ctx.oauthConfig)
	assert.Equal(suite.T(), jwtCfg, ctx.jwtConfig)
	assert.Equal(suite.T(), flowCfg, ctx.flowConfig)
	assert.Equal(suite.T(), obsCfg, ctx.observabilityConfig)
	assert.Equal(suite.T(), customExec["custom"], ctx.customExecutors["custom"])
	assert.NotNil(suite.T(), ctx.observabilitySvc)
	assert.NotNil(suite.T(), ctx.authzProvider)
}

func (suite *EngineTestSuite) TestJOSEConfig() {
	ctx := &engineContext{
		jwtConfig: engineconfig.JWTConfig{
			Issuer:         "https://auth.example.com",
			ValidityPeriod: 3600,
			Audience:       "https://api.example.com",
			PreferredKeyID: "key-1",
			Leeway:         30,
		},
		serverConfig: engineconfig.ServerConfig{
			SecurityConfig: engineconfig.SecurityConfig{JWKSCacheTTL: 120},
		},
	}

	expected := joseconfig.Config{
		Issuer:         "https://auth.example.com",
		ValidityPeriod: 3600,
		Audience:       "https://api.example.com",
		PreferredKeyID: "key-1",
		Leeway:         30,
		JWKSCacheTTL:   120 * time.Second,
	}
	assert.Equal(suite.T(), expected, ctx.joseConfig())
}

func (suite *EngineTestSuite) TestWithCustomExecutors_MergesIntoExistingMap() {
	var ctx engineContext
	ctx.customExecutors = map[string]providers.Executor{
		"existing": newTestExecutor(suite.T(), "existing"),
	}

	WithCustomExecutors(map[string]providers.Executor{
		"new": newTestExecutor(suite.T(), "new"),
	})(&ctx)

	assert.Len(suite.T(), ctx.customExecutors, 2)
	assert.Equal(suite.T(), "existing", ctx.customExecutors["existing"].GetName())
	assert.Equal(suite.T(), "new", ctx.customExecutors["new"].GetName())
}
