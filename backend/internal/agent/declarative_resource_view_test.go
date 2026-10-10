// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/agent/model"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

func viewDocument(t *testing.T, content string) *yaml.Node {
	t.Helper()
	var root yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(content), &root))
	return root.Content[0]
}

// An exported agent is shown as a read returns it, with its references as they are and no secret.
func TestViewResourceShowsAnExportedAgentAsItsRead(t *testing.T) {
	exported, err := yaml.Marshal(&model.AgentGetResponse{
		ID: "agent-1", OUID: "ou-1", Type: "default", Name: "Planner",
		AttributesYAML: map[string]interface{}{"team": "ops"},
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{{
			Type: providers.OAuthInboundAuthType,
			OAuthConfig: &providers.OAuthConfigWithSecret{
				ClientID:     "var:AGENT_PLANNER_CLIENT_ID",
				ClientSecret: "sec:AGENT_PLANNER_CLIENT_SECRET",
			},
		}},
	})
	require.NoError(t, err)

	view, err := newAgentExporter(nil).ViewResource(context.Background(),
		viewDocument(t, "resource_type: agent\n"+string(exported)))

	require.NoError(t, err)
	a, ok := view.(*model.AgentGetResponse)
	require.True(t, ok)
	assert.Equal(t, "agent-1", a.ID)
	assert.Equal(t, "Planner", a.Name)
	assert.Equal(t, "ou-1", a.OUID)
	assert.Equal(t, "var:AGENT_PLANNER_CLIENT_ID", a.ClientID)
	assert.JSONEq(t, `{"team":"ops"}`, string(a.Attributes))
	require.Len(t, a.InboundAuthConfig, 1)
	assert.Equal(t, "var:AGENT_PLANNER_CLIENT_ID", a.InboundAuthConfig[0].OAuthConfig.ClientID)
	assert.Empty(t, a.InboundAuthConfig[0].OAuthConfig.ClientSecret)
}

// A document that is not an agent is refused.
func TestViewResourceRefusesAnUnreadableDocument(t *testing.T) {
	exporter := newAgentExporter(nil)

	_, err := exporter.ViewResource(context.Background(), viewDocument(t, "name: [not, a, name]"))
	assert.Error(t, err)
}
