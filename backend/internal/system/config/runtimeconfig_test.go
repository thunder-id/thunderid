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
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type RuntimeConfigTestSuite struct {
	suite.Suite
}

func TestRuntimeConfigSuite(t *testing.T) {
	suite.Run(t, new(RuntimeConfigTestSuite))
}

func (suite *RuntimeConfigTestSuite) BeforeTest(suiteName, testName string) {
	runtimeConfig = nil
	once = sync.Once{}
}

func (suite *RuntimeConfigTestSuite) TestInitializeServerRuntime() {
	config := &Config{
		Server: ServerConfig{
			Hostname: "testhost",
			Port:     9000,
		},
		TLS: TLSConfig{
			CertFile: "test-cert.pem",
			KeyFile:  "test-key.pem",
		},
	}

	err := InitializeServerRuntime("/test/thunderid/home", config)

	assert.NoError(suite.T(), err)

	runtime := runtimeConfig
	assert.NotNil(suite.T(), runtime)
	assert.Equal(suite.T(), "/test/thunderid/home", runtime.ServerHome)
	assert.Equal(suite.T(), config.Server.Hostname, runtime.Config.Server.Hostname)
	assert.Equal(suite.T(), config.Server.Port, runtime.Config.Server.Port)
	assert.Equal(suite.T(), config.TLS.CertFile, runtime.Config.TLS.CertFile)
}

func (suite *RuntimeConfigTestSuite) TestInitializeServerRuntimeOnlyOnce() {
	// First initialization
	firstConfig := &Config{
		Server: ServerConfig{
			Hostname: "firsthost",
			Port:     8000,
		},
	}

	err := InitializeServerRuntime("/first/path", firstConfig)
	assert.NoError(suite.T(), err)

	// Try second initialization
	secondConfig := &Config{
		Server: ServerConfig{
			Hostname: "secondhost",
			Port:     9000,
		},
	}

	err = InitializeServerRuntime("/second/path", secondConfig)
	assert.NoError(suite.T(), err) // Should not return error

	// Verify that the first initialization remains
	runtime := GetServerRuntime()
	assert.Equal(suite.T(), "/first/path", runtime.ServerHome)
	assert.Equal(suite.T(), "firsthost", runtime.Config.Server.Hostname)
	assert.Equal(suite.T(), 8000, runtime.Config.Server.Port)
}

func (suite *RuntimeConfigTestSuite) TestGetServerRuntime() {
	config := &Config{
		Server: ServerConfig{
			Hostname: "gettest",
			Port:     8888,
		},
	}

	err := InitializeServerRuntime("/get/test/path", config)
	assert.NoError(suite.T(), err)

	runtime := GetServerRuntime()

	assert.NotNil(suite.T(), runtime)
	assert.Equal(suite.T(), "/get/test/path", runtime.ServerHome)
	assert.Equal(suite.T(), "gettest", runtime.Config.Server.Hostname)
}

func (suite *RuntimeConfigTestSuite) TestGetServerRuntimePanic() {
	runtimeConfig = nil

	assert.Panics(suite.T(), func() {
		GetServerRuntime()
	})
}

func (suite *RuntimeConfigTestSuite) TestInitializeServerRuntime_InvalidLoginPathFallback() {
	// Setup a config with an intentionally broken LoginPath
	config := &Config{}
	config.GateClient.Scheme = schemeHTTPS
	config.GateClient.Hostname = localhost
	config.GateClient.Port = 8443
	config.GateClient.LoginPath = "/login%ZZ"

	err := InitializeServerRuntime("/test/thunderid/home", config)

	assert.NoError(suite.T(), err)

	runtime := GetServerRuntime()
	assert.NotNil(suite.T(), runtime)
	assert.NotNil(suite.T(), runtime.GateClientLoginURL)

	assert.Equal(suite.T(), "/signin", runtime.GateClientLoginURL.Path)
	assert.Equal(suite.T(), "https://localhost:8443/signin", runtime.GateClientLoginURL.String())
}

func (suite *RuntimeConfigTestSuite) TestInitializeServerRuntime_InvalidCallbackPathFallback() {
	// Setup a config with an intentionally broken CallbackPath
	config := &Config{}
	config.GateClient.Scheme = schemeHTTPS
	config.GateClient.Hostname = localhost
	config.GateClient.Port = 8443
	config.GateClient.CallbackPath = "/callback%ZZ"

	err := InitializeServerRuntime("/test/thunderid/home", config)

	assert.NoError(suite.T(), err)

	runtime := GetServerRuntime()
	assert.NotNil(suite.T(), runtime)
	assert.NotNil(suite.T(), runtime.GateClientCallbackURL)

	assert.Equal(suite.T(), "/callback", runtime.GateClientCallbackURL.Path)
	assert.Equal(suite.T(), "https://localhost:8443/callback", runtime.GateClientCallbackURL.String())
}
