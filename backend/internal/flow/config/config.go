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

// Package flowconfig holds flow-specific configuration injected at initialization.
package flowconfig

import (
	flowsession "github.com/thunder-id/thunderid/internal/flow/session"
	"github.com/thunder-id/thunderid/internal/system/config"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// Config holds configuration values required by flow services.
type Config struct {
	Flow engineconfig.FlowConfig
	// SecureCookies marks SSO cookies Secure; it is derived from the deployment's HTTP-only setting.
	SecureCookies bool
	// Session holds the SSO session lifetime configuration used for both server-side timeouts and the
	// cookie lifetime. It is sourced from the server-config "session" section at the composition root,
	// not the static server runtime, so FromServerRuntime leaves it zero for the caller to populate.
	Session flowsession.Config
}

// FromServerRuntime builds flow configuration from the global server runtime.
func FromServerRuntime() Config {
	runtime := config.GetServerRuntime()
	return Config{
		Flow:          runtime.Config.Flow,
		SecureCookies: !runtime.Config.Server.HTTPOnly,
	}
}
