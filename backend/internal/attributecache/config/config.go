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

// Package attributecache holds attribute cache-specific configuration injected at initialization.
package attributecache

import (
	"github.com/thunder-id/thunderid/internal/system/config"
)

// Config holds configuration values required by the attribute cache services.
type Config struct {
	DeploymentID string
	StoreConfig  config.DatabaseConfig
}

// FromServerRuntime builds attribute cache configuration from the global server runtime.
func FromServerRuntime() Config {
	runtime := config.GetServerRuntime()
	return Config{
		DeploymentID: runtime.Config.Server.Identifier,
		StoreConfig:  runtime.Config.Database,
	}
}
