// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent_test

import (
	"context"
	"testing"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/agent"
	"github.com/thunder-id/thunderid/internal/agent/model"
	"github.com/thunder-id/thunderid/internal/entity"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/agentmock"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"

	"gopkg.in/yaml.v3"
)

type AgentExporterTestSuite struct {
	suite.Suite
	mockService *agentmock.AgentServiceInterfaceMock
	mockEntity  *entitymock.EntityServiceInterfaceMock
	exporter    declarativeresource.ResourceExporter
	logger      *log.Logger
}

func TestAgentExporterTestSuite(t *testing.T) {
	suite.Run(t, new(AgentExporterTestSuite))
}

func (s *AgentExporterTestSuite) SetupTest() {
	s.mockService = agentmock.NewAgentServiceInterfaceMock(s.T())
	s.mockEntity = entitymock.NewEntityServiceInterfaceMock(s.T())
	s.exporter = agent.NewAgentExporterForTest(s.mockService, s.mockEntity)
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
	svcErr := &tidcommon.ServiceError{
		Code:  "ERR_CODE",
		Error: tidcommon.I18nMessage{DefaultValue: "test error"},
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
	expected := &model.AgentGetResponse{ID: "agent1", Display: "My Agent"}
	s.mockService.EXPECT().GetAgent(mock.Anything, "agent1", true).Return(expected, nil)
	s.mockEntity.EXPECT().GetHeldCredentialTypes(mock.Anything, "agent1").Return(nil, nil)

	resource, name, err := s.exporter.GetResourceByID(context.Background(), "agent1")

	assert.Nil(s.T(), err)
	assert.Equal(s.T(), "My Agent", name)
	assert.Equal(s.T(), expected, resource)
	assert.Nil(s.T(), expected.Credentials, "an agent with no credential set exports none")
}

// Each credential the agent holds is exported as a placeholder named after the agent, never as its
// stored hash.
func (s *AgentExporterTestSuite) TestGetResourceByID_ExportsHeldCredentialsAsPlaceholders() {
	s.mockService.EXPECT().GetAgent(mock.Anything, "agent1", true).
		Return(&model.AgentGetResponse{ID: "agent1", Display: "My Agent"}, nil)
	s.mockEntity.EXPECT().GetHeldCredentialTypes(mock.Anything, "agent1").Return([]string{"password"}, nil)

	resource, _, err := s.exporter.GetResourceByID(context.Background(), "agent1")

	assert.Nil(s.T(), err)
	assert.Equal(s.T(), map[string]interface{}{"password": "{{.AGENT_MY_AGENT_PASSWORD}}"},
		resource.(*model.AgentGetResponse).Credentials)
}

// A control plane's export writes each credential as a reference to the secret the gateway holds.
func (s *AgentExporterTestSuite) TestGetResourceByID_WritesCredentialReferences() {
	writer, ok := s.exporter.(declarativeresource.ReferenceWriter)
	s.Require().True(ok, "the exporter writes its credential placeholders itself")
	writer.WriteValueReferences(true)
	s.mockService.EXPECT().GetAgent(mock.Anything, "agent1", true).
		Return(&model.AgentGetResponse{ID: "agent1", Display: "My Agent"}, nil)
	s.mockEntity.EXPECT().GetHeldCredentialTypes(mock.Anything, "agent1").Return([]string{"password"}, nil)

	resource, _, err := s.exporter.GetResourceByID(context.Background(), "agent1")

	assert.Nil(s.T(), err)
	assert.Equal(s.T(), map[string]interface{}{"password": "sec:AGENT_MY_AGENT_PASSWORD"},
		resource.(*model.AgentGetResponse).Credentials)
}

func (s *AgentExporterTestSuite) TestGetResourceByID_CredentialReadFailure() {
	s.mockService.EXPECT().GetAgent(mock.Anything, "agent1", true).
		Return(&model.AgentGetResponse{ID: "agent1", Display: "My Agent"}, nil)
	s.mockEntity.EXPECT().GetHeldCredentialTypes(mock.Anything, "agent1").Return(nil, assert.AnError)

	resource, name, err := s.exporter.GetResourceByID(context.Background(), "agent1")

	assert.Nil(s.T(), resource)
	assert.Empty(s.T(), name)
	s.Require().NotNil(err)
	assert.Equal(s.T(), tidcommon.InternalServerError.Code, err.Code)
}

// An agent whose type is gone fails its export as a client error, as a write of it would.
func (s *AgentExporterTestSuite) TestGetResourceByID_TypeGoneFailsAsSchemaValidation() {
	s.mockService.EXPECT().GetAgent(mock.Anything, "agent1", true).
		Return(&model.AgentGetResponse{ID: "agent1", Display: "My Agent"}, nil)
	s.mockEntity.EXPECT().GetHeldCredentialTypes(mock.Anything, "agent1").
		Return(nil, entity.ErrSchemaValidationFailed)

	_, _, err := s.exporter.GetResourceByID(context.Background(), "agent1")

	s.Require().NotNil(err)
	assert.Equal(s.T(), tidcommon.ClientErrorType, err.Type)
}

func (s *AgentExporterTestSuite) TestSecretFieldsNameTheCredentials() {
	lister, ok := s.exporter.(declarativeresource.SecretFieldLister)
	s.Require().True(ok)
	assert.ElementsMatch(s.T(), []string{"inboundAuthConfig[].config.clientSecret", "credentials.*"},
		lister.SecretFields())
}

// An unbounded lifespan is exported as its sentinel, so importing it onto an agent with a bound
// removes the bound instead of leaving it in place.
func (s *AgentExporterTestSuite) TestGetResourceByID_ExportsUnboundedLifespan() {
	s.mockService.EXPECT().GetAgent(mock.Anything, "agent1", true).Return(&model.AgentGetResponse{
		ID: "agent1", Lifespan: &model.Lifespan{Phase: model.LifespanPhaseUnbounded},
	}, nil)
	s.mockEntity.EXPECT().GetHeldCredentialTypes(mock.Anything, "agent1").Return(nil, nil)

	resource, _, err := s.exporter.GetResourceByID(context.Background(), "agent1")

	assert.Nil(s.T(), err)
	assert.Equal(s.T(), "9999-12-31T23:59:59Z", resource.(*model.AgentGetResponse).ValidUntil)
}

// An exported agent keeps its project, so an import places it in the same project.
func (s *AgentExporterTestSuite) TestGetResourceByID_ExportsProject() {
	s.mockService.EXPECT().GetAgent(mock.Anything, "agent1", true).Return(
		&model.AgentGetResponse{ID: "agent1", ProjectID: "project-1"}, nil)
	s.mockEntity.EXPECT().GetHeldCredentialTypes(mock.Anything, "agent1").Return(nil, nil)

	resource, _, err := s.exporter.GetResourceByID(context.Background(), "agent1")
	s.Require().Nil(err)

	document, marshalErr := yaml.Marshal(resource)
	s.Require().NoError(marshalErr)
	assert.Contains(s.T(), string(document), "projectId: project-1")
}

func (s *AgentExporterTestSuite) TestGetResourceByID_Error() {
	svcErr := &tidcommon.ServiceError{
		Code:  "ERR_CODE",
		Error: tidcommon.I18nMessage{DefaultValue: "not found"},
	}
	s.mockService.EXPECT().GetAgent(mock.Anything, "agent1", true).Return(nil, svcErr)

	resource, name, err := s.exporter.GetResourceByID(context.Background(), "agent1")

	assert.Nil(s.T(), resource)
	assert.Empty(s.T(), name)
	assert.Equal(s.T(), svcErr, err)
}

func (s *AgentExporterTestSuite) TestValidateResource_Success() {
	a := &model.AgentGetResponse{ID: "agent1", Display: "Valid Agent"}

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
	a := &model.AgentGetResponse{ID: "agent1"}

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
		ID:      "agent1",
		Display: "Public Agent",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:     "client-id-1",
					PublicClient: true,
				},
			},
		},
	}

	rules := pr.GetResourceRulesForResource(a)

	assert.NotNil(s.T(), rules)
	assert.Contains(s.T(), rules.Variables, "InboundAuthConfig[].OAuthConfig.ClientID")
	assert.NotContains(s.T(), rules.SecretVariables, "InboundAuthConfig[].OAuthConfig.ClientSecret")
	assert.Empty(s.T(), rules.ArrayVariables, "no redirect URIs means no array variables")
}

func (s *AgentExporterTestSuite) TestGetResourceRulesForResource_PublicClientWithRedirectURIs() {
	pr, ok := s.exporter.(declarativeresource.PerResourceRuler)
	assert.True(s.T(), ok, "exporter should implement PerResourceRuler")

	a := &model.AgentGetResponse{
		ID:      "agent1",
		Display: "Public Agent",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				OAuthConfig: &providers.OAuthConfigWithSecret{
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
	assert.NotContains(s.T(), rules.SecretVariables, "InboundAuthConfig[].OAuthConfig.ClientSecret")
	assert.Contains(s.T(), rules.ArrayVariables, "InboundAuthConfig[].OAuthConfig.RedirectURIs")
}

func (s *AgentExporterTestSuite) TestGetResourceRulesForResource_ConfidentialClientNoRedirectURIs() {
	pr, ok := s.exporter.(declarativeresource.PerResourceRuler)
	assert.True(s.T(), ok, "exporter should implement PerResourceRuler")

	a := &model.AgentGetResponse{
		ID:      "agent2",
		Display: "Confidential Agent",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:     "client-id-2",
					PublicClient: false,
				},
			},
		},
	}

	rules := pr.GetResourceRulesForResource(a)

	assert.NotNil(s.T(), rules)
	assert.Contains(s.T(), rules.Variables, "InboundAuthConfig[].OAuthConfig.ClientID")
	assert.Contains(s.T(), rules.SecretVariables, "InboundAuthConfig[].OAuthConfig.ClientSecret")
	assert.Empty(s.T(), rules.ArrayVariables, "M2M agents have no redirect URIs")
}

func (s *AgentExporterTestSuite) TestGetResourceRulesForResource_ClientWithoutSecret() {
	pr, ok := s.exporter.(declarativeresource.PerResourceRuler)
	assert.True(s.T(), ok, "exporter should implement PerResourceRuler")

	for _, method := range []providers.TokenEndpointAuthMethod{
		providers.TokenEndpointAuthMethodPrivateKeyJWT,
		providers.TokenEndpointAuthMethodNone,
	} {
		s.Run(string(method), func() {
			a := &model.AgentGetResponse{
				ID:      "agent1",
				Display: "Secretless Agent",
				InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
					{
						OAuthConfig: &providers.OAuthConfigWithSecret{
							ClientID:                "client-id-1",
							TokenEndpointAuthMethod: method,
						},
					},
				},
			}

			rules := pr.GetResourceRulesForResource(a)

			assert.NotNil(s.T(), rules)
			assert.Contains(s.T(), rules.Variables, "InboundAuthConfig[].OAuthConfig.ClientID")
			assert.Empty(s.T(), rules.SecretVariables)
		})
	}
}

func (s *AgentExporterTestSuite) TestGetResourceRulesForResource_ConfidentialClientWithRedirectURIs() {
	pr, ok := s.exporter.(declarativeresource.PerResourceRuler)
	assert.True(s.T(), ok, "exporter should implement PerResourceRuler")

	a := &model.AgentGetResponse{
		ID:      "agent2",
		Display: "Confidential Agent",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				OAuthConfig: &providers.OAuthConfigWithSecret{
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
	assert.Contains(s.T(), rules.SecretVariables, "InboundAuthConfig[].OAuthConfig.ClientSecret")
	assert.Contains(s.T(), rules.ArrayVariables, "InboundAuthConfig[].OAuthConfig.RedirectURIs")
	assert.Equal(s.T(), []string{"Credentials"}, rules.DynamicPropertyFields)
}

func (s *AgentExporterTestSuite) TestGetResourceRulesForResource_NoInboundAuthConfig() {
	pr, ok := s.exporter.(declarativeresource.PerResourceRuler)
	assert.True(s.T(), ok, "exporter should implement PerResourceRuler")

	a := &model.AgentGetResponse{ID: "agent3", Display: "Entity-only Agent"}

	rules := pr.GetResourceRulesForResource(a)

	assert.NotNil(s.T(), rules)
	assert.Equal(s.T(), []string{"Credentials"}, rules.DynamicPropertyFields,
		"a capture names the credentials of an agent without a client as the export does")
	assert.Contains(s.T(), rules.Variables, "InboundAuthConfig[].OAuthConfig.ClientID")
	assert.Contains(s.T(), rules.SecretVariables, "InboundAuthConfig[].OAuthConfig.ClientSecret")
	assert.Empty(s.T(), rules.ArrayVariables)
}

func (s *AgentExporterTestSuite) TestGetResourceRulesForResource_NilOAuthConfig() {
	pr, ok := s.exporter.(declarativeresource.PerResourceRuler)
	assert.True(s.T(), ok, "exporter should implement PerResourceRuler")

	a := &model.AgentGetResponse{
		ID:      "agent4",
		Display: "Agent With Nil OAuth",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{OAuthConfig: nil},
		},
	}

	rules := pr.GetResourceRulesForResource(a)

	assert.NotNil(s.T(), rules)
	assert.Contains(s.T(), rules.Variables, "InboundAuthConfig[].OAuthConfig.ClientID")
	assert.Contains(s.T(), rules.SecretVariables, "InboundAuthConfig[].OAuthConfig.ClientSecret")
	assert.Empty(s.T(), rules.ArrayVariables)
}

func (s *AgentExporterTestSuite) TestGetResourceRulesForResource_NonAgentType() {
	pr, ok := s.exporter.(declarativeresource.PerResourceRuler)
	assert.True(s.T(), ok, "exporter should implement PerResourceRuler")

	rules := pr.GetResourceRulesForResource("not-an-agent")

	assert.NotNil(s.T(), rules)
	assert.Contains(s.T(), rules.Variables, "InboundAuthConfig[].OAuthConfig.ClientID")
	assert.Contains(s.T(), rules.SecretVariables, "InboundAuthConfig[].OAuthConfig.ClientSecret")
	assert.Contains(s.T(), rules.ArrayVariables, "InboundAuthConfig[].OAuthConfig.RedirectURIs")
	assert.Equal(s.T(), []string{"Credentials"}, rules.DynamicPropertyFields)
}
