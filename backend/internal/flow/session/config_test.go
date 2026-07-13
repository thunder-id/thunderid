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

package session

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ConfigTestSuite struct {
	suite.Suite
}

func TestConfigTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigTestSuite))
}

func (s *ConfigTestSuite) TestValidate_UnsetUsesDefaults() {
	s.Require().NoError(Config{}.Validate())
}

func (s *ConfigTestSuite) TestValidate_PositiveValues() {
	s.Require().NoError(Config{IdleTimeoutSeconds: 1800, AbsoluteTimeoutSeconds: 28800}.Validate())
}

func (s *ConfigTestSuite) TestValidate_NegativeRejected() {
	s.Require().Error(Config{IdleTimeoutSeconds: -1}.Validate())
	s.Require().Error(Config{AbsoluteTimeoutSeconds: -1}.Validate())
}

func (s *ConfigTestSuite) TestValidate_IdleExceedsAbsolute() {
	s.Require().Error(Config{IdleTimeoutSeconds: 28801, AbsoluteTimeoutSeconds: 28800}.Validate())
}

func (s *ConfigTestSuite) TestHandler_DecodeEmptyIsZero() {
	got, err := ConfigHandler{}.Decode(nil)
	s.Require().NoError(err)
	s.Equal(Config{}, got)
}

func (s *ConfigTestSuite) TestHandler_DecodeJSON() {
	got, err := ConfigHandler{}.Decode(json.RawMessage(`{"idleTimeoutSeconds":900,"absoluteTimeoutSeconds":3600}`))
	s.Require().NoError(err)
	s.Equal(Config{IdleTimeoutSeconds: 900, AbsoluteTimeoutSeconds: 3600}, got)
}

func (s *ConfigTestSuite) TestHandler_ValidateRejectsIncoherent() {
	s.Require().Error(ConfigHandler{}.Validate(Config{IdleTimeoutSeconds: -5}, nil, nil))
}

func (s *ConfigTestSuite) TestHandler_MergeWritableWins() {
	readOnly := Config{IdleTimeoutSeconds: 1800, AbsoluteTimeoutSeconds: 28800}
	writable := Config{IdleTimeoutSeconds: 600}
	merged := ConfigHandler{}.Merge(readOnly, writable).(Config)
	// A positive writable field overrides read-only; an unset writable field keeps read-only.
	s.Equal(int64(600), merged.IdleTimeoutSeconds)
	s.Equal(int64(28800), merged.AbsoluteTimeoutSeconds)
}
