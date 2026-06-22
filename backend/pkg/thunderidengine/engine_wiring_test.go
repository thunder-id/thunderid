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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/authz"
	flowcore "github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/executor"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
	"github.com/thunder-id/thunderid/tests/mocks/rolemock"
)

type stubHostRoleProvider struct{}

func (stubHostRoleProvider) GetAuthorizedPermissions(
	_ context.Context, _ string, _ []string, requested []string,
) ([]string, error) {
	return requested, nil
}

func (stubHostRoleProvider) GetUserRoles(_ context.Context, _ string, _ []string) ([]string, error) {
	return []string{"user"}, nil
}

func TestBuildExecutorRegistry_FillsRoleService(t *testing.T) {
	config.ResetServerRuntime()
	defer config.ResetServerRuntime()
	require.NoError(t, config.InitializeServerRuntime(t.TempDir(), &config.Config{}))

	mockRole := rolemock.NewRoleServiceInterfaceMock(t)
	cacheManager := cache.Initialize(config.CacheConfig{}, "test-dep")
	flowFactory, _ := flowcore.Initialize(cacheManager)

	c := &engineConfig{
		roleService:      mockRole,
		jwtService:       jwtmock.NewJWTServiceInterfaceMock(t),
		executorDeps:     &ExecutorDependencies{},
		enabledExecutors: []string{"AuthAssertExecutor"},
	}

	reg, err := c.buildExecutorRegistry(flowFactory)
	require.NoError(t, err)
	require.NotNil(t, reg)

	_, err = reg.GetExecutor("AuthAssertExecutor")
	require.NoError(t, err)
}

func TestBuildExecutorRegistry_AutoFillsConsentEnforcer(t *testing.T) {
	config.ResetServerRuntime()
	defer config.ResetServerRuntime()
	require.NoError(t, config.InitializeServerRuntime(t.TempDir(), &config.Config{}))

	cacheManager := cache.Initialize(config.CacheConfig{}, "test-dep")
	flowFactory, _ := flowcore.Initialize(cacheManager)

	c := &engineConfig{
		jwtService:       jwtmock.NewJWTServiceInterfaceMock(t),
		executorDeps:     &ExecutorDependencies{},
		enabledExecutors: []string{executor.ExecutorNameConsent},
	}

	reg, err := c.buildExecutorRegistry(flowFactory)
	require.NoError(t, err)

	_, err = reg.GetExecutor(executor.ExecutorNameConsent)
	require.NoError(t, err)
}

func TestHostRoleProvider_DerivesAuthZService(t *testing.T) {
	roleSvc := newRoleAdapter(stubHostRoleProvider{})
	authZSvc := authz.Initialize(roleSvc)
	assert.NotNil(t, authZSvc)
}

func TestWillRegisterConsentExecutor(t *testing.T) {
	assert.True(t, willRegisterConsentExecutor(nil))
	assert.True(t, willRegisterConsentExecutor([]string{}))
	assert.True(t, willRegisterConsentExecutor([]string{executor.ExecutorNameConsent}))
	assert.False(t, willRegisterConsentExecutor([]string{"CredentialsAuthExecutor"}))
}

func TestLoadEngineConfig_OmitsDatabaseDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	writeMinimalDeployment(t, tmpDir)
	cfg, err := LoadEngineConfig(tmpDir)
	require.NoError(t, err)
	assert.Empty(t, cfg.Database.Config.Type)
	assert.Empty(t, cfg.Database.User.Type)
	assert.Empty(t, cfg.Database.Runtime.Type)
}

func writeMinimalDeployment(t *testing.T, serverHome string) {
	t.Helper()
	err := os.WriteFile(
		filepath.Join(serverHome, "deployment.yaml"),
		[]byte("server:\n  hostname: localhost\n  port: 8090\n"),
		0o600,
	)
	require.NoError(t, err)
}
