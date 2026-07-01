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

// Package oauthconfig holds OAuth-specific configuration injected at initialization.
package oauthconfig

import (
	"github.com/thunder-id/thunderid/internal/system/config"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// Config holds configuration values required by OAuth services.
type Config struct {
	DeploymentID  string
	RuntimeDBType string
	BaseURL       string
	JWT           engineconfig.JWTConfig
	OAuth         engineconfig.OAuthConfig
	GateClient    engineconfig.GateClientConfig
}

// FromServerRuntime builds OAuth configuration from the global server runtime.
func FromServerRuntime() Config {
	runtime := config.GetServerRuntime()
	return Config{
		DeploymentID:  runtime.Config.Server.Identifier,
		RuntimeDBType: runtime.Config.Database.Runtime.Type,
		BaseURL:       config.GetServerURL(&runtime.Config.Server),
		JWT:           runtime.Config.JWT,
		OAuth:         runtime.Config.OAuth,
		GateClient:    runtime.Config.GateClient,
	}
}
