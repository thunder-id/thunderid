// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowmgt

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

func viewDocument(t *testing.T, content string) *yaml.Node {
	t.Helper()
	var root yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(content), &root))
	return root.Content[0]
}

// An exported flow is shown as a read returns it, with its references as they are.
func TestViewResourceShowsAnExportedFlowAsItsRead(t *testing.T) {
	flow := &providers.CompleteFlowDefinition{
		ID: "flow-1", Handle: "basic-login", Name: "Basic login", FlowType: providers.FlowTypeAuthentication,
		ActiveVersion: 2,
		Nodes: []providers.NodeDefinition{
			{ID: "start", Type: "START", OnSuccess: "login"},
			{
				ID: "login", Type: "TASK_EXECUTION", OnSuccess: "end",
				Executor:   &providers.ExecutorDefinition{Name: "UsernamePasswordAuthenticator"},
				Properties: map[string]interface{}{"clientSecret": "sec:FLOW_BASIC_LOGIN_SECRET"},
				Meta:       map[string]interface{}{"title": "Sign in"},
			},
			{ID: "end", Type: "END"},
		},
		CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z",
	}
	exported, err := yaml.Marshal(flow)
	require.NoError(t, err)

	view, err := newFlowGraphExporter(nil).ViewResource(context.Background(),
		viewDocument(t, "resource_type: flow\n"+string(exported)))

	require.NoError(t, err)
	assert.Equal(t, flow, view)
}

// A document that is not a flow is refused.
func TestViewResourceRefusesAnUnreadableDocument(t *testing.T) {
	exporter := newFlowGraphExporter(nil)

	_, err := exporter.ViewResource(context.Background(), viewDocument(t, "name: [not, a, name]"))
	assert.Error(t, err)
}
