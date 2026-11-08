// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/agent/model"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/tests/mocks/agentmock"
)

// asRuntime matches a context marked as the server's own caller.
var asRuntime = mock.MatchedBy(security.IsRuntimeContext)

type AgentSharingTestSuite struct {
	suite.Suite
	service *agentmock.AgentServiceInterfaceMock
	decl    *agentSharingDeclaration
}

func TestAgentSharingTestSuite(t *testing.T) {
	suite.Run(t, new(AgentSharingTestSuite))
}

func (s *AgentSharingTestSuite) SetupTest() {
	s.service = agentmock.NewAgentServiceInterfaceMock(s.T())
	s.decl = newAgentSharingDeclaration(s.service)
}

func (s *AgentSharingTestSuite) TestDeclaresNoFields() {
	s.Equal(agentSharingType, s.decl.ResourceType())
	s.Empty(s.decl.Fields())
}

// The owner is read as the server, so the answer does not depend on who asked.
func (s *AgentSharingTestSuite) TestOwnerIsTheAgentsUnit() {
	s.service.EXPECT().GetAgent(asRuntime, "agent-1", false).
		Return(&model.AgentGetResponse{ID: "agent-1", OUID: "ou-1"}, nil).Once()

	ouID, err := s.decl.OwningOUID(context.Background(), "agent-1")

	s.Require().Nil(err)
	s.Equal("ou-1", ouID)
}

func (s *AgentSharingTestSuite) TestUnknownAgent() {
	s.service.EXPECT().GetAgent(asRuntime, "missing", false).Return(nil, &ErrorAgentNotFound).Once()

	_, err := s.decl.OwningOUID(context.Background(), "missing")

	s.Require().NotNil(err)
	s.Equal(ErrorAgentNotFound.Code, err.Code)
}
