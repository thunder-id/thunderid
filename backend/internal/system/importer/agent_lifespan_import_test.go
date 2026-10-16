// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/agent"
	agentmodel "github.com/thunder-id/thunderid/internal/agent/model"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/agentmock"
)

func agentDoc(t *testing.T, body string) parsedDocument {
	t.Helper()
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(body), &node); err != nil {
		t.Fatalf("parsing the document: %v", err)
	}
	return parsedDocument{ResourceType: resourceTypeAgent, Node: node.Content[0]}
}

func noUpsert() *ImportOptions {
	upsert := false
	return &ImportOptions{Upsert: &upsert}
}

// An exported expired agent is restored as it was, so its past validUntil reaches the agent service.
func TestImportAgent_PastLifespanIsRestored(t *testing.T) {
	agentSvc := agentmock.NewAgentServiceInterfaceMock(t)
	agentSvc.On("CreateAgent", mock.Anything, mock.MatchedBy(func(a *providers.Agent) bool {
		return a.ValidUntil != nil && *a.ValidUntil == "2020-01-01T00:00:00Z"
	})).Return(&agentmodel.AgentCompleteResponse{ID: "agent-1"}, nil).Once()
	svc := &importService{agentService: agentSvc}

	outcome := svc.importAgent(context.Background(), agentDoc(t,
		"id: agent-1\ntype: default\nouId: ou-1\nvalidUntil: \"2020-01-01T00:00:00Z\"\n"),
		noUpsert(), false, nil)

	assert.Equal(t, statusSuccess, outcome.Status)
}

// A malformed validUntil is reported by the dry run, rather than only by the import it previews.
func TestImportAgent_MalformedLifespanFailsDryRun(t *testing.T) {
	agentSvc := agentmock.NewAgentServiceInterfaceMock(t)
	svc := &importService{agentService: agentSvc}

	outcome := svc.importAgent(context.Background(), agentDoc(t,
		"id: agent-1\ntype: default\nouId: ou-1\nvalidUntil: \"2027-03-31\"\n"),
		noUpsert(), true, nil)

	assert.Equal(t, statusFailed, outcome.Status)
	assert.Equal(t, agent.ErrorInvalidLifespan.Code, outcome.Code)
	agentSvc.AssertNotCalled(t, "CreateAgent", mock.Anything, mock.Anything)
}
