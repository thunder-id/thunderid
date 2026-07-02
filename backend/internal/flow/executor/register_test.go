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

package executor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	yaml "gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/tests/mocks/authn/githubmock"
	"github.com/thunder-id/thunderid/tests/mocks/authn/googlemock"
	"github.com/thunder-id/thunderid/tests/mocks/authn/oauthmock"
	"github.com/thunder-id/thunderid/tests/mocks/authn/oidcmock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
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
	reg, err := Initialize(suite.depsForBuiltInRegistration(), engineconfig.FlowConfig{
		Executors: []string{""},
	})
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
	reg, err := Initialize(suite.depsForBuiltInRegistration(), engineconfig.FlowConfig{})
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), reg)

	for _, name := range defaultBuiltInExecutorNames() {
		assert.True(suite.T(), reg.IsRegistered(name), "executor %q should be registered", name)
	}
}

func (suite *BuiltInExecutorRegistrationTestSuite) TestInitialize_SubsetConfig() {
	reg, err := Initialize(suite.depsForBuiltInRegistration(), engineconfig.FlowConfig{
		Executors: []string{ExecutorNameInviteExecutor},
	})
	require.NoError(suite.T(), err)

	assert.True(suite.T(), reg.IsRegistered(ExecutorNameInviteExecutor))
	assert.False(suite.T(), reg.IsRegistered(ExecutorNameCredentialsAuth))
}

func (suite *BuiltInExecutorRegistrationTestSuite) TestInitialize_UnknownExecutorReturnsError() {
	reg, err := Initialize(ExecutorDependencies{}, engineconfig.FlowConfig{
		Executors: []string{"NotARealExecutor"},
	})
	require.Error(suite.T(), err)
	assert.Nil(suite.T(), reg)
	assert.Contains(suite.T(), err.Error(), "unknown built-in executor")
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

	reg, err := Initialize(suite.depsForBuiltInRegistration(), cfg.Flow)
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

	reg, err := Initialize(suite.depsForBuiltInRegistration(), cfg.Flow)
	require.NoError(suite.T(), err)
	for _, name := range defaultBuiltInExecutorNames() {
		assert.True(suite.T(), reg.IsRegistered(name), "executor %q should be registered", name)
	}
}
