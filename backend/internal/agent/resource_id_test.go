// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/thunder-id/thunderid/internal/agent/model"
)

// A capture names an agent's values from the agent it is handed, so the exporter must read the ID from it.
func TestAgentExporterReadsTheResourceID(t *testing.T) {
	exporter := newAgentExporter(nil, nil, nil)

	assert.Equal(t, "agent-1", exporter.GetResourceID(&model.AgentGetResponse{ID: "agent-1"}))
	assert.Empty(t, exporter.GetResourceID("not an agent"))
	assert.Empty(t, exporter.GetResourceID(nil))
}
