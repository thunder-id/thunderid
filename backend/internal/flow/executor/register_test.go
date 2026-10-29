// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	yaml "gopkg.in/yaml.v3"

	flowconfig "github.com/thunder-id/thunderid/internal/flow/config"
	"github.com/thunder-id/thunderid/tests/mocks/actorprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/agentmgtprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/authn/githubmock"
	"github.com/thunder-id/thunderid/tests/mocks/authn/googlemock"
	"github.com/thunder-id/thunderid/tests/mocks/authn/oauthmock"
	"github.com/thunder-id/thunderid/tests/mocks/authn/oidcmock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/identitygovernanceprovidermock"
)

type BuiltInExecutorRegistrationTestSuite struct {
	suite.Suite
	registry ExecutorRegistryInterface
}

func TestBuiltInExecutorRegistrationSuite(t *testing.T) {
	suite.Run(t, new(BuiltInExecutorRegistrationTestSuite))
}

func (suite *BuiltInExecutorRegistrationTestSuite) SetupTest() {
	suite.registry = newExecutorRegistry()
}

func (suite *BuiltInExecutorRegistrationTestSuite) mockFlowFactory() *coremock.FlowFactoryInterfaceMock {
	mockFactory := coremock.NewFlowFactoryInterfaceMock(suite.T())
	mockBase := coremock.NewExecutorInterfaceMock(suite.T())
	mockBase.On("GetName").Return("").Maybe()
	mockBase.On("GetType").Return(providers.ExecutorTypeUtility).Maybe()
	mockFactory.On("CreateExecutor", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(mockBase).Maybe()
	return mockFactory
}

func (suite *BuiltInExecutorRegistrationTestSuite) depsForBuiltInRegistration() ExecutorDependencies {
	return ExecutorDependencies{
		FlowFactory:    suite.mockFlowFactory(),
		EntityProvider: entityprovidermock.NewEntityProviderInterfaceMock(suite.T()),
		OAuthSvc:       oauthmock.NewOAuthAuthnServiceInterfaceMock(suite.T()),
		OIDCSvc:        oidcmock.NewOIDCAuthnServiceInterfaceMock(suite.T()),
		GithubSvc:      githubmock.NewGithubOAuthAuthnServiceInterfaceMock(suite.T()),
		GoogleSvc:      googlemock.NewGoogleOIDCAuthnServiceInterfaceMock(suite.T()),
	}
}

func (suite *BuiltInExecutorRegistrationTestSuite) TestRegisterBuiltInExecutors_AllDefaultNamesRegister() {
	err := registerBuiltInExecutors(suite.registry, suite.depsForBuiltInRegistration(), nil)
	require.NoError(suite.T(), err)

	for _, name := range defaultBuiltInExecutorNames() {
		assert.True(suite.T(), suite.registry.IsRegistered(name), "executor %q should be registered", name)
	}
}

func (suite *BuiltInExecutorRegistrationTestSuite) TestRegisterBuiltInExecutor_EachDefaultNameRegisters() {
	deps := suite.depsForBuiltInRegistration()
	catalog := newBuiltInExecutorRegistrars()
	for _, name := range defaultBuiltInExecutorNames() {
		reg := newExecutorRegistry()
		err := registerBuiltInExecutor(reg, catalog, deps, name)
		require.NoError(suite.T(), err, "registering %q", name)
		assert.True(suite.T(), reg.IsRegistered(name), "executor %q should be registered", name)
	}
}

func (suite *BuiltInExecutorRegistrationTestSuite) TestRegisterBuiltInExecutor_PassesIdentityGovernance() {
	deps := suite.depsForBuiltInRegistration()
	governance := identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(suite.T())
	deps.IdentityGovernance = governance
	governanceOf := map[string]func(providers.Executor) providers.IdentityGovernanceProvider{
		ExecutorNameAuthAssert: func(e providers.Executor) providers.IdentityGovernanceProvider {
			return e.(*authAssertExecutor).accessGuard.governance
		},
		ExecutorNameIdentityGovernance: func(e providers.Executor) providers.IdentityGovernanceProvider {
			return e.(*identityGovernanceExecutor).governance
		},
		ExecutorNamePreSuspend: func(e providers.Executor) providers.IdentityGovernanceProvider {
			return e.(*preSuspendExecutor).governance
		},
		ExecutorNameCredentialSetter: func(e providers.Executor) providers.IdentityGovernanceProvider {
			return e.(*credentialSetter).accessGuard.governance
		},
		ExecutorNameIdentifying: func(e providers.Executor) providers.IdentityGovernanceProvider {
			return e.(*identifyingExecutor).accessGuard.governance
		},
		ExecutorNameSession: func(e providers.Executor) providers.IdentityGovernanceProvider {
			return e.(*sessionExecutor).accessGuard.governance
		},
		ExecutorNameOTPExecutor: func(e providers.Executor) providers.IdentityGovernanceProvider {
			return e.(*otpExecutor).accessGuard.governance
		},
		ExecutorNamePasskeyAuth: func(e providers.Executor) providers.IdentityGovernanceProvider {
			return e.(*passkeyAuthExecutor).accessGuard.governance
		},
		ExecutorNameMagicLink: func(e providers.Executor) providers.IdentityGovernanceProvider {
			return e.(*magicLinkExecutor).accessGuard.governance
		},
	}
	for name, governanceOfExecutor := range governanceOf {
		reg := newExecutorRegistry()
		require.NoError(suite.T(), registerBuiltInExecutor(reg, newBuiltInExecutorRegistrars(), deps, name))
		registered, err := reg.GetExecutor(name)
		require.NoError(suite.T(), err)
		assert.Same(suite.T(), governance, governanceOfExecutor(registered), name)
	}
}

func (suite *BuiltInExecutorRegistrationTestSuite) TestRegisterBuiltInExecutor_RequiresFlowFactory() {
	reg := newExecutorRegistry()
	suite.Require().Panics(func() {
		_ = registerBuiltInExecutor(
			reg, newBuiltInExecutorRegistrars(), ExecutorDependencies{}, ExecutorNameInviteExecutor)
	})
	assert.False(suite.T(), reg.IsRegistered(ExecutorNameInviteExecutor))
}

func (suite *BuiltInExecutorRegistrationTestSuite) TestRegisterBuiltInExecutors_UnknownName() {
	err := registerBuiltInExecutors(suite.registry, ExecutorDependencies{}, []string{"NotARealExecutor"})
	require.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "unknown built-in executor")
	assert.False(suite.T(), suite.registry.IsRegistered("NotARealExecutor"))
}

func (suite *BuiltInExecutorRegistrationTestSuite) TestResolveBuiltInExecutorNames_EmptyUsesDefaults() {
	catalog := newBuiltInExecutorRegistrars()
	names, err := resolveBuiltInExecutorNames(catalog, nil)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), defaultBuiltInExecutorNames(), names)

	names, err = resolveBuiltInExecutorNames(catalog, []string{})
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), defaultBuiltInExecutorNames(), names)
}

func (suite *BuiltInExecutorRegistrationTestSuite) TestResolveBuiltInExecutorNames_UnknownName() {
	_, err := resolveBuiltInExecutorNames(newBuiltInExecutorRegistrars(), []string{"NotARealExecutor"})
	require.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "unknown built-in executor")
}

func (suite *BuiltInExecutorRegistrationTestSuite) TestResolveBuiltInExecutorNames_UnknownAfterValidName() {
	_, err := resolveBuiltInExecutorNames(newBuiltInExecutorRegistrars(), []string{
		ExecutorNameInviteExecutor,
		"NotARealExecutor",
	})
	require.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "unknown built-in executor")
}

func (suite *BuiltInExecutorRegistrationTestSuite) TestResolveBuiltInExecutorNames_WhitespaceNameRejected() {
	_, err := resolveBuiltInExecutorNames(newBuiltInExecutorRegistrars(), []string{" " + ExecutorNameCredentialsAuth})
	require.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "unknown built-in executor")
}

func (suite *BuiltInExecutorRegistrationTestSuite) TestResolveBuiltInExecutorNames_EmptyStringRejected() {
	_, err := resolveBuiltInExecutorNames(newBuiltInExecutorRegistrars(), []string{""})
	require.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "unknown built-in executor")
}

func (suite *BuiltInExecutorRegistrationTestSuite) TestInitialize_EmptyStringExecutorRejected() {
	reg, err := Initialize(suite.depsForBuiltInRegistration(), flowconfig.Config{Flow: engineconfig.FlowConfig{
		Executors: []string{""},
	}})
	require.Error(suite.T(), err)
	assert.Nil(suite.T(), reg)
	assert.Contains(suite.T(), err.Error(), "unknown built-in executor")
}

func (suite *BuiltInExecutorRegistrationTestSuite) TestRegisterBuiltInExecutors_InvalidListDoesNotPartiallyRegister() {
	err := registerBuiltInExecutors(suite.registry, suite.depsForBuiltInRegistration(), []string{
		ExecutorNameInviteExecutor,
		"NotARealExecutor",
	})
	require.Error(suite.T(), err)
	assert.False(suite.T(), suite.registry.IsRegistered(ExecutorNameInviteExecutor))
}

func (suite *BuiltInExecutorRegistrationTestSuite) TestRegisterBuiltInExecutors_Subset() {
	err := registerBuiltInExecutors(suite.registry, suite.depsForBuiltInRegistration(), []string{
		ExecutorNamePermissionValidator,
		ExecutorNameInviteExecutor,
	})
	require.NoError(suite.T(), err)

	assert.True(suite.T(), suite.registry.IsRegistered(ExecutorNamePermissionValidator))
	assert.True(suite.T(), suite.registry.IsRegistered(ExecutorNameInviteExecutor))
	assert.False(suite.T(), suite.registry.IsRegistered(ExecutorNameCredentialsAuth))
}

func (suite *BuiltInExecutorRegistrationTestSuite) TestRegisterBuiltInExecutors_DedupesNames() {
	err := registerBuiltInExecutors(suite.registry, suite.depsForBuiltInRegistration(), []string{
		ExecutorNamePermissionValidator,
		ExecutorNamePermissionValidator,
	})
	require.NoError(suite.T(), err)
	assert.True(suite.T(), suite.registry.IsRegistered(ExecutorNamePermissionValidator))
}

func (suite *BuiltInExecutorRegistrationTestSuite) TestRegisterBuiltInExecutors_CustomExecutorAfterBuiltIn() {
	err := registerBuiltInExecutors(suite.registry, suite.depsForBuiltInRegistration(),
		[]string{ExecutorNamePermissionValidator})
	require.NoError(suite.T(), err)

	custom := createMockExecutorForRegistry(suite.T(), "CustomExecutor", providers.ExecutorTypeUtility)
	suite.registry.RegisterExecutor("CustomExecutor", custom)

	assert.True(suite.T(), suite.registry.IsRegistered("CustomExecutor"))
	assert.True(suite.T(), suite.registry.IsRegistered(ExecutorNamePermissionValidator))
	assert.False(suite.T(), suite.registry.IsRegistered(ExecutorNameCredentialsAuth))
}

func (suite *BuiltInExecutorRegistrationTestSuite) TestInitialize_EmptyConfigRegistersAllBuiltIns() {
	reg, err := Initialize(suite.depsForBuiltInRegistration(), flowconfig.Config{})
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), reg)

	for _, name := range defaultBuiltInExecutorNames() {
		assert.True(suite.T(), reg.IsRegistered(name), "executor %q should be registered", name)
	}
}

func (suite *BuiltInExecutorRegistrationTestSuite) TestInitialize_SubsetConfig() {
	reg, err := Initialize(suite.depsForBuiltInRegistration(), flowconfig.Config{Flow: engineconfig.FlowConfig{
		Executors: []string{ExecutorNameInviteExecutor},
	}})
	require.NoError(suite.T(), err)

	assert.True(suite.T(), reg.IsRegistered(ExecutorNameInviteExecutor))
	assert.False(suite.T(), reg.IsRegistered(ExecutorNameCredentialsAuth))
}

func (suite *BuiltInExecutorRegistrationTestSuite) TestInitialize_UnknownExecutorReturnsError() {
	reg, err := Initialize(ExecutorDependencies{}, flowconfig.Config{Flow: engineconfig.FlowConfig{
		Executors: []string{"NotARealExecutor"},
	}})
	require.Error(suite.T(), err)
	assert.Nil(suite.T(), reg)
	assert.Contains(suite.T(), err.Error(), "unknown built-in executor")
}

func (suite *BuiltInExecutorRegistrationTestSuite) TestInitialize_LeewayAndLifetimeReachThePreSuspendExecutor() {
	deps := suite.depsForBuiltInRegistration()
	deps.ArtifactLifetime = func(*providers.OAuthClient) time.Duration { return time.Hour }
	reg, err := Initialize(deps, flowconfig.Config{
		Flow:        engineconfig.FlowConfig{Executors: []string{ExecutorNamePreSuspend}},
		TokenLeeway: 30 * time.Second,
	})
	require.NoError(suite.T(), err)

	ex, err := reg.GetExecutor(ExecutorNamePreSuspend)
	require.NoError(suite.T(), err)
	preSuspend := ex.(*preSuspendExecutor)
	assert.Equal(suite.T(), 30*time.Second, preSuspend.cutoffLeeway)
	require.NotNil(suite.T(), preSuspend.artifactLifetime)
	assert.Equal(suite.T(), time.Hour, preSuspend.artifactLifetime(&providers.OAuthClient{}))
}

func (suite *BuiltInExecutorRegistrationTestSuite) TestSetActorProvider_ReachesThePreSuspendExecutor() {
	reg, err := Initialize(suite.depsForBuiltInRegistration(), flowconfig.Config{
		Flow: engineconfig.FlowConfig{Executors: []string{ExecutorNamePreSuspend, ExecutorNameInviteExecutor}},
	})
	require.NoError(suite.T(), err)
	actors := actorprovidermock.NewActorProviderMock(suite.T())

	require.NoError(suite.T(), SetActorProvider(reg, actors))

	ex, err := reg.GetExecutor(ExecutorNamePreSuspend)
	require.NoError(suite.T(), err)
	assert.Same(suite.T(), actors, ex.(*preSuspendExecutor).actors)
}

func (suite *BuiltInExecutorRegistrationTestSuite) TestSetActorProvider_RefusesAForeignRegistry() {
	reg := &nilSkippingRegistry{inner: newExecutorRegistry()}

	err := SetActorProvider(reg, actorprovidermock.NewActorProviderMock(suite.T()))

	require.Error(suite.T(), err)
}

// nilSkippingRegistry delegates to a real registry but forces nil executors, matching
// RegisterExecutor's silent skip behavior when construction would otherwise succeed.
type nilSkippingRegistry struct {
	inner ExecutorRegistryInterface
}

func (n *nilSkippingRegistry) GetExecutor(name string) (providers.Executor, error) {
	return n.inner.GetExecutor(name)
}

func (n *nilSkippingRegistry) RegisterExecutor(name string, _ providers.Executor) {
	n.inner.RegisterExecutor(name, nil)
}

func (n *nilSkippingRegistry) IsRegistered(name string) bool {
	return n.inner.IsRegistered(name)
}

func (n *nilSkippingRegistry) GetExecutorMeta(name string) (*providers.ExecutorMeta, error) {
	return n.inner.GetExecutorMeta(name)
}

func (suite *BuiltInExecutorRegistrationTestSuite) TestRegisterBuiltInExecutor_RegistryNilExecutorSkipFails() {
	reg := &nilSkippingRegistry{inner: newExecutorRegistry()}
	err := registerBuiltInExecutor(
		reg, newBuiltInExecutorRegistrars(), suite.depsForBuiltInRegistration(), ExecutorNamePermissionValidator)
	require.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to register built-in executor")
	assert.False(suite.T(), reg.IsRegistered(ExecutorNamePermissionValidator))
}

func (suite *BuiltInExecutorRegistrationTestSuite) TestSetAgentMgtProvider_InjectsIntoProvisioningExecutor() {
	err := registerBuiltInExecutors(suite.registry, suite.depsForBuiltInRegistration(),
		[]string{ExecutorNameProvisioning})
	require.NoError(suite.T(), err)
	provider := agentmgtprovidermock.NewAgentMgtProviderMock(suite.T())

	require.NoError(suite.T(), SetAgentMgtProvider(suite.registry, provider))

	exec, err := suite.registry.GetExecutor(ExecutorNameProvisioning)
	require.NoError(suite.T(), err)
	provisioningExec, ok := exec.(*provisioningExecutor)
	require.True(suite.T(), ok)
	assert.Same(suite.T(), provider, provisioningExec.agentMgtProvider)
}

func (suite *BuiltInExecutorRegistrationTestSuite) TestSetAgentMgtProvider_UnsupportedRegistry() {
	err := SetAgentMgtProvider(NewExecutorRegistryInterfaceMock(suite.T()),
		agentmgtprovidermock.NewAgentMgtProviderMock(suite.T()))

	require.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "does not support agent provider injection")
}

func (suite *BuiltInExecutorRegistrationTestSuite) TestFlowConfig_ExecutorsYAMLUnmarshal() {
	const yamlFragment = `
flow:
  executors:
    - CredentialsAuthExecutor
    - InviteExecutor
`
	var cfg struct {
		Flow engineconfig.FlowConfig `yaml:"flow"`
	}
	err := yaml.Unmarshal([]byte(yamlFragment), &cfg)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), []string{ExecutorNameCredentialsAuth, ExecutorNameInviteExecutor}, cfg.Flow.Executors)

	reg, err := Initialize(suite.depsForBuiltInRegistration(), flowconfig.Config{Flow: cfg.Flow})
	require.NoError(suite.T(), err)
	assert.True(suite.T(), reg.IsRegistered(ExecutorNameCredentialsAuth))
	assert.True(suite.T(), reg.IsRegistered(ExecutorNameInviteExecutor))
	assert.False(suite.T(), reg.IsRegistered(ExecutorNameOAuth))
}

func (suite *BuiltInExecutorRegistrationTestSuite) TestDeploymentResource_FlowExecutorsOmittedRegistersAllByDefault() {
	deploymentPath := filepath.Join("..", "..", "..", "tests", "resources", "deployment.yaml")
	data, err := os.ReadFile(deploymentPath) // #nosec G304 -- fixed test fixture path
	require.NoError(suite.T(), err)

	var cfg struct {
		Flow engineconfig.FlowConfig `yaml:"flow"`
	}
	require.NoError(suite.T(), yaml.Unmarshal(data, &cfg))
	assert.Empty(suite.T(), cfg.Flow.Executors)

	reg, err := Initialize(suite.depsForBuiltInRegistration(), flowconfig.Config{Flow: cfg.Flow})
	require.NoError(suite.T(), err)
	for _, name := range defaultBuiltInExecutorNames() {
		assert.True(suite.T(), reg.IsRegistered(name), "executor %q should be registered", name)
	}
}
