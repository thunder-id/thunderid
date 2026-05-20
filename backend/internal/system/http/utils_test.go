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

package http

import (
	"crypto/tls"
	"testing"

	"github.com/thunder-id/thunderid/internal/system/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// UtilsTestSuite defines the test suite for HTTP utils.
type UtilsTestSuite struct {
	suite.Suite
}

// TestUtilsSuite runs the HTTP utils test suite.
func TestUtilsSuite(t *testing.T) {
	suite.Run(t, new(UtilsTestSuite))
}

func (suite *UtilsTestSuite) TestGetTLSVersion_TLS12() {
	cfg := config.Config{
		TLS: config.TLSConfig{
			MinVersion: "1.2",
		},
	}

	version := GetTLSVersion(cfg)
	assert.Equal(suite.T(), uint16(tls.VersionTLS12), version)
}

func (suite *UtilsTestSuite) TestGetTLSVersion_TLS13() {
	cfg := config.Config{
		TLS: config.TLSConfig{
			MinVersion: "1.3",
		},
	}

	version := GetTLSVersion(cfg)
	assert.Equal(suite.T(), uint16(tls.VersionTLS13), version)
}

func (suite *UtilsTestSuite) TestGetTLSVersion_DefaultToTLS13() {
	cfg := config.Config{
		TLS: config.TLSConfig{
			MinVersion: "",
		},
	}

	version := GetTLSVersion(cfg)
	assert.Equal(suite.T(), uint16(tls.VersionTLS13), version)
}

func (suite *UtilsTestSuite) TestGetTLSVersion_InvalidVersionDefaultsToTLS13() {
	cfg := config.Config{
		TLS: config.TLSConfig{
			MinVersion: "1.1",
		},
	}

	version := GetTLSVersion(cfg)
	assert.Equal(suite.T(), uint16(tls.VersionTLS13), version)
}

func (suite *UtilsTestSuite) TestGetTLSVersion_UnknownVersionDefaultsToTLS13() {
	cfg := config.Config{
		TLS: config.TLSConfig{
			MinVersion: "invalid",
		},
	}

	version := GetTLSVersion(cfg)
	assert.Equal(suite.T(), uint16(tls.VersionTLS13), version)
}
