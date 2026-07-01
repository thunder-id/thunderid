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

package serverconfig

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

type ConfigTestSuite struct {
	suite.Suite
}

func TestConfigTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigTestSuite))
}

func (suite *ConfigTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

// setRuntime initializes a fresh runtime with the given server-config store knob and global flag.
func (suite *ConfigTestSuite) setRuntime(store string, declarativeEnabled bool) {
	config.ResetServerRuntime()
	cfg := &config.Config{Server: engineconfig.ServerConfig{Identifier: "test-deployment"}}
	cfg.ServerConfig.Store = store
	cfg.DeclarativeResources.Enabled = declarativeEnabled
	suite.Require().NoError(config.InitializeServerRuntime("", cfg))
}

func (suite *ConfigTestSuite) TestGetServerConfigStoreMode() {
	testCases := []struct {
		name               string
		store              string
		declarativeEnabled bool
		expected           serverconst.StoreMode
		expectErr          bool
	}{
		{"explicit mutable", "mutable", false, serverconst.StoreModeMutable, false},
		{"explicit declarative", "declarative", false, serverconst.StoreModeDeclarative, false},
		{"explicit composite", "composite", false, serverconst.StoreModeComposite, false},
		{"case insensitive", "Composite", false, serverconst.StoreModeComposite, false},
		{"trimmed whitespace", "  composite  ", false, serverconst.StoreModeComposite, false},
		{"explicit takes precedence over global", "mutable", true, serverconst.StoreModeMutable, false},
		{"fallback composite when declarative enabled", "", true, serverconst.StoreModeComposite, false},
		{"fallback mutable when declarative disabled", "", false, serverconst.StoreModeMutable, false},
		{"invalid value is rejected", "bogus", true, "", true},
	}
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.setRuntime(tc.store, tc.declarativeEnabled)
			mode, err := getServerConfigStoreMode()
			if tc.expectErr {
				suite.Error(err)
				return
			}
			suite.NoError(err)
			suite.Equal(tc.expected, mode)
		})
	}
}
