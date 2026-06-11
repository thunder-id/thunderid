/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package config

import (
	"context"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/thunder-id/thunderid/internal/system/log"
)

// ServerRuntime holds the runtime configuration for the server.
type ServerRuntime struct {
	ServerHome            string `yaml:"server_home"`
	GateClientLoginURL    *url.URL
	GateClientCallbackURL *url.URL
	Config                Config `yaml:"config"`
}

var (
	runtimeConfig *ServerRuntime
	once          sync.Once
)

// InitializeServerRuntime initializes the server runtime configurations.
func InitializeServerRuntime(serverHome string, config *Config) error {
	once.Do(func() {
		loginPath := config.GateClient.LoginPath
		if strings.TrimSpace(loginPath) == "" {
			loginPath = "/signin"
		}
		callbackPath := config.GateClient.CallbackPath
		if strings.TrimSpace(callbackPath) == "" {
			callbackPath = "/callback"
		}

		portStr := strconv.Itoa(config.GateClient.Port)
		hostWithPort := net.JoinHostPort(config.GateClient.Hostname, portStr)

		baseURL := &url.URL{
			Scheme: config.GateClient.Scheme,
			Host:   hostWithPort,
		}

		parsedPath, err := url.Parse(loginPath)
		if err != nil || parsedPath == nil {
			// Runtime initialization runs during application startup, outside any request.
			log.GetLogger().Warn(context.Background(),
				"Invalid gate client login path configured. Falling back to default '/signin'",
				log.String("configuredPath", loginPath),
				log.Error(err),
			)
			parsedPath = &url.URL{Path: "/signin"}
		}

		parsedCallbackPath, err := url.Parse(callbackPath)
		if err != nil || parsedCallbackPath == nil {
			// Runtime initialization runs during application startup, outside any request.
			log.GetLogger().Warn(context.Background(),
				"Invalid gate client callback path configured. Falling back to default '/callback'",
				log.String("configuredPath", callbackPath),
				log.Error(err),
			)
			parsedCallbackPath = &url.URL{Path: "/callback"}
		}

		parsedURL := baseURL.ResolveReference(parsedPath)
		parsedCallbackURL := baseURL.ResolveReference(parsedCallbackPath)

		runtimeConfig = &ServerRuntime{
			ServerHome:            serverHome,
			GateClientLoginURL:    parsedURL,
			GateClientCallbackURL: parsedCallbackURL,
			Config:                *config,
		}
	})
	return nil
}

// GetServerRuntime returns the server runtime configurations.
func GetServerRuntime() *ServerRuntime {
	if runtimeConfig == nil {
		panic("Server runtime is not initialized")
	}
	return runtimeConfig
}

// ResetServerRuntime resets the server runtime.
// This should only be used in tests to reset the singleton state.
func ResetServerRuntime() {
	runtimeConfig = nil
	once = sync.Once{}
}
