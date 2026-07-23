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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	systemconfig "github.com/thunder-id/thunderid/internal/system/config"
	joseconfig "github.com/thunder-id/thunderid/internal/system/jose/config"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/actorprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/authzmock"
	"github.com/thunder-id/thunderid/tests/mocks/consentprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/designprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/executormock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/flowexecmock"
	"github.com/thunder-id/thunderid/tests/mocks/i18nprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/idpprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/observabilityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/ouprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/resourceserverprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/runtimestoreprovidermock"
)

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

func newTestIDPProvider(t *testing.T) providers.IDPProvider {
	return idpprovidermock.NewIDPProviderMock(t)
}

func newTestRuntimeStoreProvider(t *testing.T) providers.RuntimeStoreProvider {
	return runtimestoreprovidermock.NewRuntimeStoreProviderMock(t)
}

func validEngineContext(t *testing.T) *engineContext {
	return &engineContext{
		serverHome:            "/tmp/server",
		serverConfig:          engineconfig.ServerConfig{Identifier: "test-server"},
		observabilitySvc:      newTestObservabilityProvider(t),
		authzProvider:         newTestAuthzProvider(t),
		actorProvider:         actorprovidermock.NewActorProviderMock(t),
		authnProvider:         managermock.NewAuthnProviderManagerMock(t),
		resourceProvider:      resourceserverprovidermock.NewResourceServerProviderMock(t),
		ouProvider:            ouprovidermock.NewOrganizationUnitProviderMock(t),
		designResolveProvider: designprovidermock.NewDesignProviderMock(t),
		flowProvider:          flowexecmock.NewFlowProviderMock(t),
		i18nProvider:          i18nprovidermock.NewI18nProviderMock(t),
		idpProvider:           idpprovidermock.NewIDPProviderMock(t),
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

	suite.T().Run("missing organization unit provider", func(t *testing.T) {
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
	logCfg := engineconfig.LogConfig{Level: "debug", Format: "json"}
	gateClientCfg := engineconfig.GateClientConfig{Hostname: "localhost", Port: 9090}
	keyConfigs := []engineconfig.KeyConfig{{ID: "key-1", CertFile: "cert.pem", KeyFile: "key.pem"}}
	encryptionCfg := engineconfig.EncryptionConfig{Key: "secret"}
	idpProvider := newTestIDPProvider(suite.T())
	runtimeStoreProvider := newTestRuntimeStoreProvider(suite.T())
	customExec := map[string]providers.Executor{"custom": newTestExecutor(suite.T(), "custom")}

	opts := []Option{
		WithServerHome("/home"),
		WithServerConfig(serverCfg),
		WithCacheConfig(cacheCfg),
		WithOAuthConfig(oauthCfg),
		WithJWTConfig(jwtCfg),
		WithFlowConfig(flowCfg),
		WithObservabilityConfig(obsCfg),
		WithLogConfig(logCfg),
		WithGateClientConfig(gateClientCfg),
		WithRuntimeTransientDBType("redis"),
		WithKeyConfigs(keyConfigs),
		WithEncryptionConfig(encryptionCfg),
		WithActorProvider(nil),
		WithAuthnProvider(nil),
		WithResourceProvider(nil),
		WithOUProvider(nil),
		WithDesignResolveProvider(nil),
		WithFlowProvider(nil),
		WithI18nProvider(nil),
		WithIDPProvider(idpProvider),
		WithConsentProvider(nil),
		WithCustomExecutors(customExec),
		WithObservabilityProvider(newTestObservabilityProvider(suite.T())),
		WithAuthorizationProvider(newTestAuthzProvider(suite.T())),
		WithRuntimeStoreProvider(runtimeStoreProvider),
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
	assert.Equal(suite.T(), logCfg, ctx.logConfig)
	assert.Equal(suite.T(), gateClientCfg, ctx.gateClientConfig)
	assert.Equal(suite.T(), "redis", ctx.runtimeTransientDBType)
	assert.Equal(suite.T(), keyConfigs, ctx.keyConfigs)
	assert.Equal(suite.T(), encryptionCfg, ctx.encryptionConfig)
	assert.Equal(suite.T(), idpProvider, ctx.idpProvider)
	assert.Equal(suite.T(), runtimeStoreProvider, ctx.runtimeStoreProvider)
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

// generateSelfSignedCertFiles writes a throwaway self-signed cert/key pair to dir and returns
// their PEM-encoded certificate contents, so callers can also seed it as a trust anchor
// (e.g. the Apple App Attest root) without needing a real one.
func generateSelfSignedCertFiles(t *testing.T, dir, certFile, keyFile string) []byte {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{CommonName: "engine-test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})

	require.NoError(t, os.WriteFile(filepath.Join(dir, certFile), certPEM, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, keyFile), keyPEM, 0o600))

	return certPEM
}

// TestNew_HappyPath drives New() through its full initialization sequence, using a real
// self-signed key pair for the default crypto provider and an injected runtime store to avoid
// needing a real database. The global system runtime config is seeded directly (rather than
// through New()'s Option API) because New() does not yet expose a way to configure Apple App
// Attest, which attestation.Initialize requires; seeding it first makes New()'s own
// (sync.Once-guarded) call to InitializeServerRuntime a no-op that keeps this pre-seeded config.
func (suite *EngineTestSuite) TestNew_HappyPath() {
	t := suite.T()
	tempDir := t.TempDir()

	keyID := "test-key"
	certPEM := generateSelfSignedCertFiles(t, tempDir, "cert.pem", "key.pem")
	keyConfigs := []engineconfig.KeyConfig{{ID: keyID, CertFile: "cert.pem", KeyFile: "key.pem"}}

	systemconfig.ResetServerRuntime()
	t.Cleanup(systemconfig.ResetServerRuntime)
	require.NoError(t, systemconfig.InitializeServerRuntime(tempDir, &systemconfig.Config{
		GateClient: engineconfig.GateClientConfig{Hostname: "localhost", Port: 8080, Scheme: "https"},
		Crypto: systemconfig.CryptoConfig{
			Keys:       keyConfigs,
			Encryption: engineconfig.EncryptionConfig{Key: "000102030405060708090a0b0c0d0e0f"},
		},
		Attestation: systemconfig.AttestationConfig{
			Apple: systemconfig.AppleAttestationConfig{RootCertificate: string(certPEM)},
		},
	}))

	runtimeStoreProvider := newTestRuntimeStoreProvider(t)

	mux := http.NewServeMux()
	eng := New(mux,
		WithServerHome(tempDir),
		WithServerConfig(engineconfig.ServerConfig{Identifier: "test-engine"}),
		WithObservabilityProvider(newTestObservabilityProvider(t)),
		WithAuthorizationProvider(newTestAuthzProvider(t)),
		WithKeyConfigs(keyConfigs),
		WithJWTConfig(engineconfig.JWTConfig{Issuer: "test-issuer", PreferredKeyID: keyID, ValidityPeriod: 3600}),
		WithRuntimeStoreProvider(runtimeStoreProvider),
		WithRuntimeTransientDBType("memory"),
		WithEncryptionConfig(engineconfig.EncryptionConfig{Key: "test-encryption-key"}),
		WithGateClientConfig(engineconfig.GateClientConfig{Hostname: "localhost", Port: 8080, Scheme: "https"}),
		WithCacheConfig(engineconfig.CacheConfig{Disabled: true}),
		WithLogConfig(engineconfig.LogConfig{Level: "info", Format: "json"}),
		WithIDPProvider(newTestIDPProvider(t)),
		WithActorProvider(actorprovidermock.NewActorProviderMock(t)),
		WithAuthnProvider(managermock.NewAuthnProviderManagerMock(t)),
		WithResourceProvider(resourceserverprovidermock.NewResourceServerProviderMock(t)),
		WithOUProvider(ouprovidermock.NewOrganizationUnitProviderMock(t)),
		WithDesignResolveProvider(designprovidermock.NewDesignProviderMock(t)),
		WithFlowProvider(flowexecmock.NewFlowProviderMock(t)),
		WithI18nProvider(i18nprovidermock.NewI18nProviderMock(t)),
		WithConsentProvider(consentprovidermock.NewConsentProviderMock(t)),
		// Restrict to built-in executors that only depend on FlowFactory; the others assume
		// non-nil typed provider dependencies (e.g. the GitHub/Google/OIDC auth executors) that
		// this minimal test setup does not wire up.
		WithFlowConfig(engineconfig.FlowConfig{Executors: []string{"InviteExecutor", "PermissionValidator"}}),
	)

	require.NotNil(t, eng)
	require.NotNil(t, eng.engineCtx)
	assert.NotNil(t, eng.engineCtx.runtimeCryptoSvc)
	assert.NotNil(t, eng.engineCtx.jwtService)
	assert.NotNil(t, eng.engineCtx.jweService)
	assert.NotNil(t, eng.engineCtx.flowFactory)
	assert.NotNil(t, eng.engineCtx.execRegistry)
	assert.NotNil(t, eng.engineCtx.interceptorRegistry)
	assert.NotNil(t, eng.engineCtx.graphBuilder)
	assert.NotNil(t, eng.engineCtx.flowExecService)
	assert.NotNil(t, eng.engineCtx.dpopVerifier)
	assert.NotNil(t, eng.engineCtx.attributeCacheService)
	assert.NotNil(t, eng.engineCtx.authAssertGen)
	assert.Equal(t, runtimeStoreProvider, eng.engineCtx.runtimeStoreProvider)
}

// TestNew_InitializesRuntimeStoreWhenNotInjected drives New() without WithRuntimeStoreProvider,
// exercising the branch that falls back to runtimestore.Initialize() when no provider was
// supplied by the caller. A SQLite-backed runtime transient datasource is seeded so the real
// dbstore implementation can initialize against a throwaway file in the test's temp dir.
func (suite *EngineTestSuite) TestNew_InitializesRuntimeStoreWhenNotInjected() {
	t := suite.T()
	tempDir := t.TempDir()

	keyID := "test-key"
	certPEM := generateSelfSignedCertFiles(t, tempDir, "cert.pem", "key.pem")
	keyConfigs := []engineconfig.KeyConfig{{ID: keyID, CertFile: "cert.pem", KeyFile: "key.pem"}}

	systemconfig.ResetServerRuntime()
	t.Cleanup(systemconfig.ResetServerRuntime)
	require.NoError(t, systemconfig.InitializeServerRuntime(tempDir, &systemconfig.Config{
		GateClient: engineconfig.GateClientConfig{Hostname: "localhost", Port: 8080, Scheme: "https"},
		Crypto: systemconfig.CryptoConfig{
			Keys:       keyConfigs,
			Encryption: engineconfig.EncryptionConfig{Key: "000102030405060708090a0b0c0d0e0f"},
		},
		Attestation: systemconfig.AttestationConfig{
			Apple: systemconfig.AppleAttestationConfig{RootCertificate: string(certPEM)},
		},
		Database: systemconfig.DatabaseConfig{
			RuntimeTransient: systemconfig.DataSource{
				Type:   "sqlite",
				SQLite: systemconfig.SQLiteDataSource{Path: "runtime.db"},
			},
		},
	}))

	mux := http.NewServeMux()
	eng := New(mux,
		WithServerHome(tempDir),
		WithServerConfig(engineconfig.ServerConfig{Identifier: "test-engine"}),
		WithObservabilityProvider(newTestObservabilityProvider(t)),
		WithAuthorizationProvider(newTestAuthzProvider(t)),
		WithKeyConfigs(keyConfigs),
		WithJWTConfig(engineconfig.JWTConfig{Issuer: "test-issuer", PreferredKeyID: keyID, ValidityPeriod: 3600}),
		WithEncryptionConfig(engineconfig.EncryptionConfig{Key: "test-encryption-key"}),
		WithGateClientConfig(engineconfig.GateClientConfig{Hostname: "localhost", Port: 8080, Scheme: "https"}),
		WithCacheConfig(engineconfig.CacheConfig{Disabled: true}),
		WithIDPProvider(newTestIDPProvider(t)),
		WithActorProvider(actorprovidermock.NewActorProviderMock(t)),
		WithAuthnProvider(managermock.NewAuthnProviderManagerMock(t)),
		WithResourceProvider(resourceserverprovidermock.NewResourceServerProviderMock(t)),
		WithOUProvider(ouprovidermock.NewOrganizationUnitProviderMock(t)),
		WithDesignResolveProvider(designprovidermock.NewDesignProviderMock(t)),
		WithFlowProvider(flowexecmock.NewFlowProviderMock(t)),
		WithI18nProvider(i18nprovidermock.NewI18nProviderMock(t)),
		WithConsentProvider(consentprovidermock.NewConsentProviderMock(t)),
		// Restrict to built-in executors that only depend on FlowFactory; the others assume
		// non-nil typed provider dependencies (e.g. the GitHub/Google/OIDC auth executors) that
		// this minimal test setup does not wire up.
		WithFlowConfig(engineconfig.FlowConfig{Executors: []string{"InviteExecutor", "PermissionValidator"}}),
	)

	require.NotNil(t, eng)
	require.NotNil(t, eng.engineCtx)
	assert.NotNil(t, eng.engineCtx.runtimeStoreProvider)
	assert.NotNil(t, eng.engineCtx.transactioner)
}
