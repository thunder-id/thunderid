// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importer

import (
	"context"
	"encoding/json"
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

// An agent's credentials are written with its attributes, since an agent takes them there, so the
// imported agent can sign in with the password the export named.
func (suite *ServiceAdaptersTestSuite) TestImportAgent_WritesCredentialsWithTheAttributes() {
	t := suite.T()
	agentSvc := agentmock.NewAgentServiceInterfaceMock(t)
	agentSvc.On("CreateAgent", mock.Anything, mock.MatchedBy(func(a *providers.Agent) bool {
		var attrs map[string]interface{}
		return json.Unmarshal(a.Attributes, &attrs) == nil &&
			attrs["username"] == "agent-1" && attrs["password"] == "s3cret"
	})).Return(&agentmodel.AgentCompleteResponse{ID: "agent-1"}, nil).Once()
	svc := &importService{agentService: agentSvc}

	outcome := svc.importAgent(context.Background(), agentDoc(t,
		"id: agent-1\ntype: default\nouId: ou-1\nattributes:\n  username: agent-1\ncredentials:\n  password: s3cret\n"),
		noUpsert(), false, nil)

	assert.Equal(t, statusSuccess, outcome.Status)
}

// A credential the document carries no value for is left out, so an agent keeps the one it has.
func (suite *ServiceAdaptersTestSuite) TestImportAgent_LeavesOutAnEmptyCredential() {
	t := suite.T()
	agentSvc := agentmock.NewAgentServiceInterfaceMock(t)
	agentSvc.On("CreateAgent", mock.Anything, mock.MatchedBy(func(a *providers.Agent) bool {
		var attrs map[string]interface{}
		if json.Unmarshal(a.Attributes, &attrs) != nil {
			return false
		}
		_, hasPassword := attrs["password"]
		return !hasPassword && attrs["username"] == "agent-1"
	})).Return(&agentmodel.AgentCompleteResponse{ID: "agent-1"}, nil).Once()
	svc := &importService{agentService: agentSvc}

	outcome := svc.importAgent(context.Background(), agentDoc(t,
		"id: agent-1\ntype: default\nouId: ou-1\nattributes:\n  username: agent-1\ncredentials:\n  password: \"\"\n"),
		noUpsert(), false, nil)

	assert.Equal(t, statusSuccess, outcome.Status)
}
