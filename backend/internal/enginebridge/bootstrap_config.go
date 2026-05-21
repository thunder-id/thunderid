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
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/config"
)

func loadEngineConfig(configPath string) (string, *config.Config, error) {
	if strings.TrimSpace(configPath) == "" {
		return "", nil, fmt.Errorf("thunderidengine: ConfigPath is required")
	}

	serverHome, deploymentPath := resolveServerHomeAndDeployment(configPath)
	if _, err := os.Stat(deploymentPath); err != nil {
		return "", nil, fmt.Errorf("thunderidengine: deployment config not found at %s: %w", deploymentPath, err)
	}

	defaultConfigPath := path.Join(serverHome, "repository/resources/conf/default.json")
	cfg, err := config.LoadConfig(deploymentPath, defaultConfigPath, serverHome)
	if err != nil {
		return "", nil, fmt.Errorf("thunderidengine: load config: %w", err)
	}
	if err := config.InitializeServerRuntime(serverHome, cfg); err != nil {
		return "", nil, fmt.Errorf("thunderidengine: init server runtime: %w", err)
	}
	return serverHome, cfg, nil
}

func resolveServerHomeAndDeployment(configPath string) (serverHome, deploymentPath string) {
	clean := filepath.Clean(configPath)
	if strings.HasSuffix(clean, string(filepath.Separator)+"deployment.yaml") ||
		strings.HasSuffix(clean, "/deployment.yaml") {
		return filepath.Clean(filepath.Join(filepath.Dir(clean), "..", "..")), clean
	}
	info, err := os.Stat(clean)
	if err == nil && info.IsDir() {
		return clean, path.Join(clean, "repository/conf/deployment.yaml")
	}
	return clean, path.Join(clean, "repository/conf/deployment.yaml")
}
