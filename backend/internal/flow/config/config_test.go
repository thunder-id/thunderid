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

package flowconfig

import (
	"testing"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
)

type FlowConfigTestSuite struct {
	suite.Suite
}

func TestFlowConfigTestSuite(t *testing.T) {
	suite.Run(t, new(FlowConfigTestSuite))
}

func (s *FlowConfigTestSuite) SetupTest() {
	config.ResetServerRuntime()
}

func (s *FlowConfigTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (s *FlowConfigTestSuite) TestFromServerRuntime() {
	cfg := &config.Config{
		Flow: engineconfig.FlowConfig{UserOnboardingFlowHandle: "onboarding-handle"},
		Server: engineconfig.ServerConfig{
			Identifier: "dep-1",
		},
		Database: config.DatabaseConfig{
			Runtime: config.DataSource{Type: "postgres"},
		},
	}
	err := config.InitializeServerRuntime("/tmp/test-flow-config", cfg)
	s.Require().NoError(err)

	result := FromServerRuntime()

	s.Equal("onboarding-handle", result.Flow.UserOnboardingFlowHandle)
	s.Equal("dep-1", result.DeploymentID)
	s.Equal("postgres", result.RuntimeDBType)
}
