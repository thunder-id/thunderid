// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowmgt

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	"github.com/stretchr/testify/mock"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// linkingFlowNodesFixtures are the sample flows shipped for federated account linking. They are not
// wired into any e2e/Console harness, so this test is their only regression coverage: it catches a
// NodeDefinition schema change, an executor constant rename, or a PromptDefinition/ActionDefinition
// field rename that would otherwise silently break them.
var linkingFlowNodesFixtures = map[string]string{
	"verified": "../../../../tests/e2e/utils/server-setup/verified-linking-flow-nodes.json",
	"verify":   "../../../../tests/e2e/utils/server-setup/verified-linking-verify-flow-nodes.json",
}

// TestLinkingFlowNodesFixtures_ValidateSuccessfully loads each committed sample flow, substitutes the
// same placeholders the e2e setup scripts would, and runs it through the real ValidateFlowDefinition
// entry point.
func (s *ValidatorTestSuite) TestLinkingFlowNodesFixtures_ValidateSuccessfully() {
	for mode, path := range linkingFlowNodesFixtures {
		s.Run(mode, func() {
			raw, err := os.ReadFile(path) //nolint:gosec // path is one of the constants above
			s.Require().NoError(err)

			substituted := strings.NewReplacer(
				"{{IDP_ID}}", "test-idp",
				"{{EXECUTOR_NAME}}", "OAuthExecutor",
				"{{VERIFY_FLOW_ID}}", "verify-flow",
			).Replace(string(raw))

			var nodes []providers.NodeDefinition
			s.Require().NoError(json.Unmarshal([]byte(substituted), &nodes))

			flowDef := &FlowDefinition{
				Handle:   mode + "-linking",
				Name:     mode + " Linking",
				FlowType: providers.FlowTypeAuthentication,
				Nodes:    nodes,
			}

			s.mockExecutorRegistry.EXPECT().IsRegistered(mock.Anything).Return(true)
			s.mockExecutorRegistry.EXPECT().GetExecutorMeta(mock.Anything).Return(nil, nil)
			s.mockGraphBuilder.EXPECT().ValidateGraph(mock.Anything, mock.Anything).Return(nil)

			s.Nil(s.v.ValidateFlowDefinition(context.Background(), flowDef))
		})
	}
}
