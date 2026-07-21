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

package oauthconfig

import (
	"testing"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
)

type OAuthConfigTestSuite struct {
	suite.Suite
}

func TestOAuthConfigTestSuite(t *testing.T) {
	suite.Run(t, new(OAuthConfigTestSuite))
}

func (s *OAuthConfigTestSuite) SetupTest() {
	config.ResetServerRuntime()
}

func (s *OAuthConfigTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (s *OAuthConfigTestSuite) TestFromServerRuntime() {
	cfg := &config.Config{
		Server: engineconfig.ServerConfig{
			Identifier: "dep-1",
			Hostname:   "thunder.io",
			Port:       443,
			PublicURL:  "https://thunder.io",
		},
		Database: config.DatabaseConfig{
			RuntimeTransient: config.DataSource{Type: "sqlite"},
		},
		JWT: engineconfig.JWTConfig{
			Issuer:         "https://thunder.io",
			ValidityPeriod: 3600,
		},
		OAuth: engineconfig.OAuthConfig{
			PAR: engineconfig.PARConfig{ExpiresIn: 600},
		},
		GateClient: engineconfig.GateClientConfig{
			Scheme:   "https",
			Hostname: "localhost",
			Port:     3000,
		},
	}
	err := config.InitializeServerRuntime("/tmp/test-oauth-config", cfg)
	s.Require().NoError(err)

	result := FromServerRuntime()

	s.Equal("dep-1", result.DeploymentID)
	s.Equal("sqlite", result.RuntimeTransientDBType)
	s.Equal("https://thunder.io", result.BaseURL)
	s.Equal("https://thunder.io", result.JWT.Issuer)
	s.Equal(int64(600), result.OAuth.PAR.ExpiresIn)
	s.Equal("localhost", result.GateClient.Hostname)
}
