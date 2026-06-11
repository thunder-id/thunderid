/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package agent_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/agent"
	"github.com/thunder-id/thunderid/internal/agent/model"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	i18ncore "github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/agentmock"
)

type AgentExporterTestSuite struct {
	suite.Suite
	mockService *agentmock.AgentServiceInterfaceMock
	exporter    declarativeresource.ResourceExporter
	logger      *log.Logger
}

func TestAgentExporterTestSuite(t *testing.T) {
	suite.Run(t, new(AgentExporterTestSuite))
}

func (s *AgentExporterTestSuite) SetupTest() {
	s.mockService = agentmock.NewAgentServiceInterfaceMock(s.T())
	s.exporter = agent.NewAgentExporterForTest(s.mockService)
	s.logger = log.GetLogger()
}

func (s *AgentExporterTestSuite) TestGetResourceType() {
	assert.Equal(s.T(), "agent", s.exporter.GetResourceType())
}

func (s *AgentExporterTestSuite) TestGetParameterizerType() {
	assert.Equal(s.T(), "Agent", s.exporter.GetParameterizerType())
}

func (s *AgentExporterTestSuite) TestGetAllResourceIDs_Success() {
	s.mockService.EXPECT().GetAgentList(mock.Anything, mock.Anything, mock.Anything, mock.Anything, false).
		Return(&model.AgentListResponse{
			Agents: []model.BasicAgentResponse{
				{ID: "agent1", IsReadOnly: false},
				{ID: "agent2", IsReadOnly: false},
			},
		}, nil).Once()
	s.mockService.EXPECT().GetAgentList(mock.Anything, mock.Anything, mock.Anything, mock.Anything, false).
		Return(&model.AgentListResponse{Agents: []model.BasicAgentResponse{}}, nil).Once()

	ids, err := s.exporter.GetAllResourceIDs(context.Background())

	assert.Nil(s.T(), err)
	assert.Len(s.T(), ids, 2)
	assert.ElementsMatch(s.T(), []string{"agent1", "agent2"}, ids)
}

func (s *AgentExporterTestSuite) TestGetAllResourceIDs_SkipsDeclarative() {
	s.mockService.EXPECT().GetAgentList(mock.Anything, mock.Anything, mock.Anything, mock.Anything, false).
		Return(&model.AgentListResponse{
			Agents: []model.BasicAgentResponse{
				{ID: "agent-db", IsReadOnly: false},
				{ID: "agent-decl", IsReadOnly: true},
			},
		}, nil).Once()
	s.mockService.EXPECT().GetAgentList(mock.Anything, mock.Anything, mock.Anything, mock.Anything, false).
		Return(&model.AgentListResponse{Agents: []model.BasicAgentResponse{}}, nil).Once()

	ids, err := s.exporter.GetAllResourceIDs(context.Background())

	assert.Nil(s.T(), err)
	assert.Len(s.T(), ids, 1)
	assert.Equal(s.T(), "agent-db", ids[0])
}

func (s *AgentExporterTestSuite) TestGetAllResourceIDs_EntityNotFound_Included() {
	s.mockService.EXPECT().GetAgentList(mock.Anything, mock.Anything, mock.Anything, mock.Anything, false).
		Return(&model.AgentListResponse{
			Agents: []model.BasicAgentResponse{{ID: "agent-orphan", IsReadOnly: false}},
		}, nil).Once()
	s.mockService.EXPECT().GetAgentList(mock.Anything, mock.Anything, mock.Anything, mock.Anything, false).
		Return(&model.AgentListResponse{Agents: []model.BasicAgentResponse{}}, nil).Once()

	ids, err := s.exporter.GetAllResourceIDs(context.Background())

	assert.Nil(s.T(), err)
	assert.Len(s.T(), ids, 1)
}

func (s *AgentExporterTestSuite) TestGetAllResourceIDs_Error() {
	svcErr := &serviceerror.ServiceError{
		Code:  "ERR_CODE",
		Error: i18ncore.I18nMessage{DefaultValue: "test error"},
	}
	s.mockService.EXPECT().GetAgentList(mock.Anything, mock.Anything, mock.Anything, mock.Anything, false).
		Return(nil, svcErr)

	ids, err := s.exporter.GetAllResourceIDs(context.Background())

	assert.Nil(s.T(), ids)
	assert.NotNil(s.T(), err)
}

func (s *AgentExporterTestSuite) TestGetAllResourceIDs_EmptyList() {
	s.mockService.EXPECT().GetAgentList(mock.Anything, mock.Anything, mock.Anything, mock.Anything, false).
		Return(&model.AgentListResponse{Agents: []model.BasicAgentResponse{}}, nil)

	ids, err := s.exporter.GetAllResourceIDs(context.Background())

	assert.Nil(s.T(), err)
	assert.Len(s.T(), ids, 0)
}

func (s *AgentExporterTestSuite) TestGetResourceByID_Success() {
	expected := &model.AgentGetResponse{ID: "agent1", Name: "My Agent"}
	s.mockService.EXPECT().GetAgent(mock.Anything, "agent1", false).Return(expected, nil)

	resource, name, err := s.exporter.GetResourceByID(context.Background(), "agent1")

	assert.Nil(s.T(), err)
	assert.Equal(s.T(), "My Agent", name)
	assert.Equal(s.T(), expected, resource)
}

func (s *AgentExporterTestSuite) TestGetResourceByID_Error() {
	svcErr := &serviceerror.ServiceError{
		Code:  "ERR_CODE",
		Error: i18ncore.I18nMessage{DefaultValue: "not found"},
	}
	s.mockService.EXPECT().GetAgent(mock.Anything, "agent1", false).Return(nil, svcErr)

	resource, name, err := s.exporter.GetResourceByID(context.Background(), "agent1")

	assert.Nil(s.T(), resource)
	assert.Empty(s.T(), name)
	assert.Equal(s.T(), svcErr, err)
}

func (s *AgentExporterTestSuite) TestValidateResource_Success() {
	a := &model.AgentGetResponse{ID: "agent1", Name: "Valid Agent"}

	name, err := s.exporter.ValidateResource(context.Background(), a, "agent1", s.logger)

	assert.Nil(s.T(), err)
	assert.Equal(s.T(), "Valid Agent", name)
}

func (s *AgentExporterTestSuite) TestValidateResource_InvalidType() {
	name, err := s.exporter.ValidateResource(context.Background(), "not-an-agent", "agent1", s.logger)

	assert.Empty(s.T(), name)
	assert.NotNil(s.T(), err)
	assert.Equal(s.T(), "agent", err.ResourceType)
	assert.Equal(s.T(), "agent1", err.ResourceID)
	assert.Equal(s.T(), "INVALID_TYPE", err.Code)
}

func (s *AgentExporterTestSuite) TestValidateResource_EmptyName() {
	a := &model.AgentGetResponse{ID: "agent1", Name: ""}

	name, err := s.exporter.ValidateResource(context.Background(), a, "agent1", s.logger)

	assert.Empty(s.T(), name)
	assert.NotNil(s.T(), err)
	assert.Equal(s.T(), "agent", err.ResourceType)
	assert.Equal(s.T(), "agent1", err.ResourceID)
}

func (s *AgentExporterTestSuite) TestGetResourceRulesForResource_PublicClientNoRedirectURIs() {
	pr, ok := s.exporter.(declarativeresource.PerResourceRuler)
	assert.True(s.T(), ok, "exporter should implement PerResourceRuler")

	a := &model.AgentGetResponse{
		ID:   "agent1",
		Name: "Public Agent",
		InboundAuthConfig: []inboundmodel.InboundAuthConfigWithSecret{
			{
				OAuthConfig: &inboundmodel.OAuthConfigWithSecret{
					ClientID:     "client-id-1",
					PublicClient: true,
				},
			},
		},
	}

	rules := pr.GetResourceRulesForResource(a)

	assert.NotNil(s.T(), rules)
	assert.Contains(s.T(), rules.Variables, "InboundAuthConfig[].OAuthConfig.ClientID")
	assert.NotContains(s.T(), rules.Variables, "InboundAuthConfig[].OAuthConfig.ClientSecret")
	assert.Empty(s.T(), rules.ArrayVariables, "no redirect URIs means no array variables")
}

func (s *AgentExporterTestSuite) TestGetResourceRulesForResource_PublicClientWithRedirectURIs() {
	pr, ok := s.exporter.(declarativeresource.PerResourceRuler)
	assert.True(s.T(), ok, "exporter should implement PerResourceRuler")

	a := &model.AgentGetResponse{
		ID:   "agent1",
		Name: "Public Agent",
		InboundAuthConfig: []inboundmodel.InboundAuthConfigWithSecret{
			{
				OAuthConfig: &inboundmodel.OAuthConfigWithSecret{
					ClientID:     "client-id-1",
					PublicClient: true,
					RedirectURIs: []string{"https://app.example.com/callback"},
				},
			},
		},
	}

	rules := pr.GetResourceRulesForResource(a)

	assert.NotNil(s.T(), rules)
	assert.Contains(s.T(), rules.Variables, "InboundAuthConfig[].OAuthConfig.ClientID")
	assert.NotContains(s.T(), rules.Variables, "InboundAuthConfig[].OAuthConfig.ClientSecret")
	assert.Contains(s.T(), rules.ArrayVariables, "InboundAuthConfig[].OAuthConfig.RedirectURIs")
}

func (s *AgentExporterTestSuite) TestGetResourceRulesForResource_ConfidentialClientNoRedirectURIs() {
	pr, ok := s.exporter.(declarativeresource.PerResourceRuler)
	assert.True(s.T(), ok, "exporter should implement PerResourceRuler")

	a := &model.AgentGetResponse{
		ID:   "agent2",
		Name: "Confidential Agent",
		InboundAuthConfig: []inboundmodel.InboundAuthConfigWithSecret{
			{
				OAuthConfig: &inboundmodel.OAuthConfigWithSecret{
					ClientID:     "client-id-2",
					PublicClient: false,
				},
			},
		},
	}

	rules := pr.GetResourceRulesForResource(a)

	assert.NotNil(s.T(), rules)
	assert.Contains(s.T(), rules.Variables, "InboundAuthConfig[].OAuthConfig.ClientID")
	assert.Contains(s.T(), rules.Variables, "InboundAuthConfig[].OAuthConfig.ClientSecret")
	assert.Empty(s.T(), rules.ArrayVariables, "M2M agents have no redirect URIs")
}

func (s *AgentExporterTestSuite) TestGetResourceRulesForResource_ConfidentialClientWithRedirectURIs() {
	pr, ok := s.exporter.(declarativeresource.PerResourceRuler)
	assert.True(s.T(), ok, "exporter should implement PerResourceRuler")

	a := &model.AgentGetResponse{
		ID:   "agent2",
		Name: "Confidential Agent",
		InboundAuthConfig: []inboundmodel.InboundAuthConfigWithSecret{
			{
				OAuthConfig: &inboundmodel.OAuthConfigWithSecret{
					ClientID:     "client-id-2",
					PublicClient: false,
					RedirectURIs: []string{"https://srv.example.com/callback"},
				},
			},
		},
	}

	rules := pr.GetResourceRulesForResource(a)

	assert.NotNil(s.T(), rules)
	assert.Contains(s.T(), rules.Variables, "InboundAuthConfig[].OAuthConfig.ClientID")
	assert.Contains(s.T(), rules.Variables, "InboundAuthConfig[].OAuthConfig.ClientSecret")
	assert.Contains(s.T(), rules.ArrayVariables, "InboundAuthConfig[].OAuthConfig.RedirectURIs")
}

func (s *AgentExporterTestSuite) TestGetResourceRulesForResource_NoInboundAuthConfig() {
	pr, ok := s.exporter.(declarativeresource.PerResourceRuler)
	assert.True(s.T(), ok, "exporter should implement PerResourceRuler")

	a := &model.AgentGetResponse{ID: "agent3", Name: "Entity-only Agent"}

	rules := pr.GetResourceRulesForResource(a)

	assert.NotNil(s.T(), rules)
	assert.Contains(s.T(), rules.Variables, "InboundAuthConfig[].OAuthConfig.ClientID")
	assert.Contains(s.T(), rules.Variables, "InboundAuthConfig[].OAuthConfig.ClientSecret")
	assert.Empty(s.T(), rules.ArrayVariables)
}

func (s *AgentExporterTestSuite) TestGetResourceRulesForResource_NilOAuthConfig() {
	pr, ok := s.exporter.(declarativeresource.PerResourceRuler)
	assert.True(s.T(), ok, "exporter should implement PerResourceRuler")

	a := &model.AgentGetResponse{
		ID:   "agent4",
		Name: "Agent With Nil OAuth",
		InboundAuthConfig: []inboundmodel.InboundAuthConfigWithSecret{
			{OAuthConfig: nil},
		},
	}

	rules := pr.GetResourceRulesForResource(a)

	assert.NotNil(s.T(), rules)
	assert.Contains(s.T(), rules.Variables, "InboundAuthConfig[].OAuthConfig.ClientID")
	assert.Contains(s.T(), rules.Variables, "InboundAuthConfig[].OAuthConfig.ClientSecret")
	assert.Empty(s.T(), rules.ArrayVariables)
}

func (s *AgentExporterTestSuite) TestGetResourceRulesForResource_NonAgentType() {
	pr, ok := s.exporter.(declarativeresource.PerResourceRuler)
	assert.True(s.T(), ok, "exporter should implement PerResourceRuler")

	rules := pr.GetResourceRulesForResource("not-an-agent")

	assert.NotNil(s.T(), rules)
	assert.Contains(s.T(), rules.Variables, "InboundAuthConfig[].OAuthConfig.ClientID")
	assert.Contains(s.T(), rules.Variables, "InboundAuthConfig[].OAuthConfig.ClientSecret")
	assert.Contains(s.T(), rules.ArrayVariables, "InboundAuthConfig[].OAuthConfig.RedirectURIs")
}
