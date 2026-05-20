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

package hash

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
)

type InitTestSuite struct {
	suite.Suite
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (suite *InitTestSuite) TestInitialize() {
	testConfig := &config.Config{
		Crypto: config.CryptoConfig{
			PasswordHashing: config.PasswordHashingConfig{
				Algorithm: string(SHA256),
				SHA256: config.SHA256Config{
					SaltSize: 32,
				},
			},
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/test/thunderid/home", testConfig)

	service, err := Initialize(HashConfig{
		Algorithm: SHA256,
		SaltSize:  32,
	})

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), service)
	assert.Implements(suite.T(), (*HashServiceInterface)(nil), service)
}
