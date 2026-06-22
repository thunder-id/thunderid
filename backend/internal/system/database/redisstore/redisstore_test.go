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

package redisstore

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

// RedisProviderTestSuite tests the redisProvider struct methods directly.
//
// Note: initRedisProvider / GetRedisProvider / GetRedisProviderCloser rely on
// a package-level sync.Once and require a live Redis connection. Those paths
// are validated by integration tests against a real Redis server.
type RedisProviderTestSuite struct {
	suite.Suite
}

func TestRedisProviderTestSuite(t *testing.T) {
	suite.Run(t, new(RedisProviderTestSuite))
}

func (suite *RedisProviderTestSuite) TestGetKeyPrefix() {
	p := &redisProvider{keyPrefix: "thunderid"}
	suite.Equal("thunderid", p.GetKeyPrefix())
}

func (suite *RedisProviderTestSuite) TestGetRedisClient_Nil() {
	p := &redisProvider{client: nil}
	suite.Nil(p.GetRedisClient())
}

func (suite *RedisProviderTestSuite) TestClose_NilClient_NoError() {
	// Closing when the client was never initialized should be a no-op.
	p := &redisProvider{client: nil}
	err := p.Close()
	suite.NoError(err)
}
