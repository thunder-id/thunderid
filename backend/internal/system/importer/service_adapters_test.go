// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/agent"
	agentmodel "github.com/thunder-id/thunderid/internal/agent/model"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/agentmock"
)

type ServiceAdaptersTestSuite struct {
	suite.Suite
}

func TestServiceAdaptersTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceAdaptersTestSuite))
}

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
func (suite *ServiceAdaptersTestSuite) TestImportAgent_PastLifespanIsRestored() {
	t := suite.T()
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

// An exported agent's project reaches the agent service, so the import keeps it in the project.
func (suite *ServiceAdaptersTestSuite) TestImportAgent_KeepsProject() {
	t := suite.T()
	agentSvc := agentmock.NewAgentServiceInterfaceMock(t)
	agentSvc.On("CreateAgent", mock.Anything, mock.MatchedBy(func(a *providers.Agent) bool {
		return a.ProjectID == "project-1"
	})).Return(&agentmodel.AgentCompleteResponse{ID: "agent-1"}, nil).Once()
	svc := &importService{agentService: agentSvc}

	outcome := svc.importAgent(context.Background(), agentDoc(t,
		"id: agent-1\ntype: default\nouId: ou-1\nprojectId: project-1\n"), noUpsert(), false, nil)

	assert.Equal(t, statusSuccess, outcome.Status)
}

// An upsert onto an existing agent keeps the project the document names.
func (suite *ServiceAdaptersTestSuite) TestImportAgent_UpsertKeepsProject() {
	t := suite.T()
	agentSvc := agentmock.NewAgentServiceInterfaceMock(t)
	agentSvc.On("GetAgent", mock.Anything, "agent-1", false).
		Return(&agentmodel.AgentGetResponse{ID: "agent-1"}, nil).Once()
	agentSvc.On("UpdateAgent", mock.Anything, "agent-1", mock.MatchedBy(func(r *agentmodel.UpdateAgentRequest) bool {
		return r.ProjectID == "project-1"
	})).Return(&agentmodel.AgentCompleteResponse{ID: "agent-1"}, nil).Once()
	svc := &importService{agentService: agentSvc}
	upsert := true

	outcome := svc.importAgent(context.Background(), agentDoc(t,
		"id: agent-1\ntype: default\nouId: ou-1\nprojectId: project-1\n"),
		&ImportOptions{Upsert: &upsert}, false, nil)

	assert.Equal(t, statusSuccess, outcome.Status)
}

// A malformed validUntil is reported by the dry run, rather than only by the import it previews.
func (suite *ServiceAdaptersTestSuite) TestImportAgent_MalformedLifespanFailsDryRun() {
	t := suite.T()
	agentSvc := agentmock.NewAgentServiceInterfaceMock(t)
	svc := &importService{agentService: agentSvc}

	outcome := svc.importAgent(context.Background(), agentDoc(t,
		"id: agent-1\ntype: default\nouId: ou-1\nvalidUntil: \"2027-03-31\"\n"),
		noUpsert(), true, nil)

	assert.Equal(t, statusFailed, outcome.Status)
	assert.Equal(t, agent.ErrorInvalidLifespan.Code, outcome.Code)
	agentSvc.AssertNotCalled(t, "CreateAgent", mock.Anything, mock.Anything)
}
