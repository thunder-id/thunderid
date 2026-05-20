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

package authn

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
)

type AuthenticationInitTestSuite struct {
	suite.Suite
}

var (
	initRuntimeMutex sync.Mutex
)

func TestAuthenticationInitTestSuite(t *testing.T) {
	suite.Run(t, new(AuthenticationInitTestSuite))
}

func initializeTestRuntime(root string) error {
	testConfig := &config.Config{
		Server: config.ServerConfig{
			Hostname: "localhost",
			Port:     8090,
		},
		JWT: config.JWTConfig{
			Issuer: "test-issuer",
		},
	}
	return config.InitializeServerRuntime(root, testConfig)
}

func (suite *AuthenticationInitTestSuite) SetupSuite() {
	initRuntimeMutex.Lock()
	config.ResetServerRuntime()
	suite.Require().NoError(initializeTestRuntime(suite.T().TempDir()))
}

func (suite *AuthenticationInitTestSuite) TearDownSuite() {
	config.ResetServerRuntime()
	initRuntimeMutex.Unlock()
}
