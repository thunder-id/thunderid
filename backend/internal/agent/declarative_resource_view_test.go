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
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
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
		ID: "agent-1", OUID: "ou-1", Type: "default",
		AttributesYAML: map[string]interface{}{"name": "Planner", "team": "ops"},
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
	assert.Equal(t, "ou-1", a.OUID)
	assert.Equal(t, "var:AGENT_PLANNER_CLIENT_ID", a.ClientID)
	assert.JSONEq(t, `{"name":"Planner","team":"ops"}`, string(a.Attributes))
	require.Len(t, a.InboundAuthConfig, 1)
	assert.Equal(t, "var:AGENT_PLANNER_CLIENT_ID", a.InboundAuthConfig[0].OAuthConfig.ClientID)
	assert.Empty(t, a.InboundAuthConfig[0].OAuthConfig.ClientSecret)
}

// The lifespan is shown as a read reports it: the unbounded sentinel export writes reads as an unbounded
// phase with no date, and a bounded value keeps its date with its phase.
func TestViewResourceReportsTheLifespanAsItsRead(t *testing.T) {
	cases := []struct {
		name       string
		validUntil string
		wantDate   string
		wantPhase  model.LifespanPhase
	}{
		{"unbounded sentinel", authnprovidercm.UnboundedValidUntil, "", model.LifespanPhaseUnbounded},
		{"future", "2099-01-01T00:00:00Z", "2099-01-01T00:00:00Z", model.LifespanPhaseActive},
		{"past", "2020-01-01T00:00:00Z", "2020-01-01T00:00:00Z", model.LifespanPhaseExpired},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			view, err := newAgentExporter(nil).ViewResource(context.Background(), viewDocument(t,
				"resource_type: agent\nid: agent-1\nvalidUntil: \""+tc.validUntil+"\"\n"))

			require.NoError(t, err)
			a := view.(*model.AgentGetResponse)
			assert.Equal(t, tc.wantDate, a.ValidUntil)
			require.NotNil(t, a.Lifespan)
			assert.Equal(t, tc.wantPhase, a.Lifespan.Phase)
		})
	}
}

// An exported agent with no validUntil reports no lifespan, as nothing says what it is.
func TestViewResourceWithoutALifespanReportsNone(t *testing.T) {
	view, err := newAgentExporter(nil).ViewResource(context.Background(),
		viewDocument(t, "resource_type: agent\nid: agent-1\n"))

	require.NoError(t, err)
	assert.Nil(t, view.(*model.AgentGetResponse).Lifespan)
}

// A document that is not an agent is refused.
func TestViewResourceRefusesAnUnreadableDocument(t *testing.T) {
	exporter := newAgentExporter(nil)

	_, err := exporter.ViewResource(context.Background(), viewDocument(t, "id: [not, an, id]"))
	assert.Error(t, err)
}
