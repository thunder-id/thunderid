// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type AgentCommonUtilsTestSuite struct {
	suite.Suite
}

func TestAgentCommonUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(AgentCommonUtilsTestSuite))
}

func (s *AgentCommonUtilsTestSuite) TestIsPersonalAgent() {
	tests := []struct {
		name   string
		entity providers.Entity
		want   bool
	}{
		{"agent without system attributes", providers.Entity{Category: providers.EntityCategoryAgent}, false},
		{"unreadable system attributes", providers.Entity{Category: providers.EntityCategoryAgent,
			SystemAttributes: json.RawMessage(`not json`)}, false},
		{"ordinary agent", providers.Entity{Category: providers.EntityCategoryAgent,
			SystemAttributes: json.RawMessage(`{"name":"assistant"}`)}, false},
		{"personal agent", providers.Entity{Category: providers.EntityCategoryAgent,
			SystemAttributes: json.RawMessage(`{"kind":"personal-assistant"}`)}, true},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.Equal(tt.want, IsPersonalAgent(tt.entity))
		})
	}
}
