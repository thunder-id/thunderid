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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	flowcore "github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
)

func TestBuildSDKDeclarativeServices_NoDatabase(t *testing.T) {
	config.ResetServerRuntime()
	defer config.ResetServerRuntime()

	tmpDir := t.TempDir()
	writeMinimalDeployment(t, tmpDir)
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "config", "resources"), 0o750))

	cfg, err := LoadEngineConfig(tmpDir)
	require.NoError(t, err)
	cfg.DeclarativeResources.Enabled = true
	require.NoError(t, config.InitializeServerRuntime(tmpDir, cfg))

	cacheManager := cache.Initialize(cfg.Cache, cfg.Server.Identifier)
	flowFactory, graphCache := flowcore.Initialize(cacheManager)
	require.NotNil(t, flowFactory)
	require.NotNil(t, graphCache)

	c := &engineConfig{}
	design, err := c.buildSDKDeclarativeServices(cacheManager)
	require.NoError(t, err)
	require.NotNil(t, design)

	assert.NotNil(t, c.ouService)
	assert.NotNil(t, c.resourceService)
	assert.NotNil(t, c.idpService)
	assert.NotNil(t, c.attributeCacheSvc)
	assert.NotNil(t, c.authZService)
	assert.NotNil(t, c.roleService)
}
