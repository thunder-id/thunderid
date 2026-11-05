// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent_test

import (
	"context"
	"errors"
	"testing"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/agent"
	"github.com/thunder-id/thunderid/internal/agent/model"
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entitytype"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/agentmock"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
)

type AgentExporterTestSuite struct {
	suite.Suite
	mockService     *agentmock.AgentServiceInterfaceMock
	mockEntity      *entitymock.EntityServiceInterfaceMock
	mockEntityTypes *entitytypemock.EntityTypeServiceInterfaceMock
	exporter        declarativeresource.ResourceExporter
	logger          *log.Logger
}

func TestAgentExporterTestSuite(t *testing.T) {
	suite.Run(t, new(AgentExporterTestSuite))
}

func (s *AgentExporterTestSuite) SetupTest() {
	s.mockService = agentmock.NewAgentServiceInterfaceMock(s.T())
	s.mockEntity = entitymock.NewEntityServiceInterfaceMock(s.T())
	s.mockEntityTypes = entitytypemock.NewEntityTypeServiceInterfaceMock(s.T())
	s.exporter = agent.NewAgentExporterForTest(s.mockService, s.mockEntity, s.mockEntityTypes)
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
	expected := &model.AgentGetResponse{ID: "agent1", Type: "default", Display: "My Agent"}
	s.mockService.EXPECT().GetAgent(mock.Anything, "agent1", true).Return(expected, nil)
	s.typeDeclaresCredentials()

	resource, name, err := s.exporter.GetResourceByID(context.Background(), "agent1")

	assert.Nil(s.T(), err)
	assert.Equal(s.T(), "My Agent", name)
	assert.Equal(s.T(), expected, resource)
}

// An unbounded lifespan is exported as its sentinel, so importing it onto an agent with a bound
// removes the bound instead of leaving it in place.
func (s *AgentExporterTestSuite) TestGetResourceByID_ExportsUnboundedLifespan() {
	s.mockService.EXPECT().GetAgent(mock.Anything, "agent1", true).Return(&model.AgentGetResponse{
		ID: "agent1", Lifespan: &model.Lifespan{Phase: model.LifespanPhaseUnbounded},
	}, nil)

	resource, _, err := s.exporter.GetResourceByID(context.Background(), "agent1")

	assert.Nil(s.T(), err)
	assert.Equal(s.T(), "9999-12-31T23:59:59Z", resource.(*model.AgentGetResponse).ValidUntil)
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
}

func (s *AgentExporterTestSuite) TestGetResourceRulesForResource_NoInboundAuthConfig() {
	pr, ok := s.exporter.(declarativeresource.PerResourceRuler)
	assert.True(s.T(), ok, "exporter should implement PerResourceRuler")

	a := &model.AgentGetResponse{ID: "agent3", Display: "Entity-only Agent"}

	rules := pr.GetResourceRulesForResource(a)

	assert.NotNil(s.T(), rules)
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
}

// typeDeclaresCredentials sets the credential attributes the agents' type declares.
func (s *AgentExporterTestSuite) typeDeclaresCredentials(attributes ...string) {
	infos := make([]entitytype.AttributeInfo, 0, len(attributes))
	for _, attribute := range attributes {
		infos = append(infos, entitytype.AttributeInfo{Attribute: attribute, Credential: true})
	}
	s.mockEntityTypes.EXPECT().GetAttributes(mock.Anything, entitytype.TypeCategoryAgent, "default",
		entitytype.AttributeFilter{AllowCredential: true}).Return(infos, nil)
}

// agentHasCredential sets whether the agent has a stored credential for an attribute.
func (s *AgentExporterTestSuite) agentHasCredential(attribute string, isSet bool) {
	var stored []entity.StoredCredential
	if isSet {
		stored = []entity.StoredCredential{{Value: "a-hash"}}
	}
	s.mockEntity.EXPECT().GetCredentialsByType(mock.Anything, "agent1", attribute).Return(stored, nil)
}

func (s *AgentExporterTestSuite) exportAgent() *model.AgentGetResponse {
	s.mockService.EXPECT().GetAgent(mock.Anything, "agent1", true).
		Return(&model.AgentGetResponse{ID: "agent1", Type: "default", Display: "My Agent"}, nil)
	resource, _, err := s.exporter.GetResourceByID(context.Background(), "agent1")
	s.Require().Nil(err)
	return resource.(*model.AgentGetResponse)
}

// An agent with a password exports it as a template variable named after the agent. The value is a
// one-way hash and never leaves; the importing server fills the variable and hashes it on the way in.
func (s *AgentExporterTestSuite) TestGetResourceByID_ExportsACredentialAsAPlaceholder() {
	s.typeDeclaresCredentials("password")
	s.agentHasCredential("password", true)

	exported := s.exportAgent()

	assert.Equal(s.T(), map[string]interface{}{"password": "{{.AGENT_MY_AGENT_PASSWORD}}"}, exported.Credentials)
}

// A control plane's export names the secret the gateway holds instead, so the gateway's import fills
// it in and hashes it.
func (s *AgentExporterTestSuite) TestGetResourceByID_ExportsACredentialAsASecretReference() {
	s.exporter.(declarativeresource.ReferenceWriter).WriteValueReferences(true)
	s.typeDeclaresCredentials("password")
	s.agentHasCredential("password", true)

	exported := s.exportAgent()

	assert.Equal(s.T(), map[string]interface{}{"password": "sec:AGENT_MY_AGENT_PASSWORD"}, exported.Credentials)
}

// A credential the agent never set is left out, rather than demanding a value on import.
func (s *AgentExporterTestSuite) TestGetResourceByID_LeavesOutACredentialTheAgentNeverSet() {
	s.typeDeclaresCredentials("password", "pin")
	s.agentHasCredential("password", false)
	s.agentHasCredential("pin", true)

	exported := s.exportAgent()

	assert.Equal(s.T(), map[string]interface{}{"pin": "{{.AGENT_MY_AGENT_PIN}}"}, exported.Credentials)
}

// A type that cannot be read fails the export rather than exporting an agent that could no longer sign in.
func (s *AgentExporterTestSuite) TestGetResourceByID_FailsWhenTheTypeIsUnreadable() {
	s.mockService.EXPECT().GetAgent(mock.Anything, "agent1", true).
		Return(&model.AgentGetResponse{ID: "agent1", Type: "default", Display: "My Agent"}, nil)
	s.mockEntityTypes.EXPECT().GetAttributes(mock.Anything, entitytype.TypeCategoryAgent, "default",
		entitytype.AttributeFilter{AllowCredential: true}).Return(nil, &tidcommon.ServiceError{Code: "ETS-1001"})

	_, _, err := s.exporter.GetResourceByID(context.Background(), "agent1")

	s.Require().NotNil(err)
	assert.Equal(s.T(), "ETS-1001", err.Code)
}

// An unreadable stored credential fails the export for the same reason.
func (s *AgentExporterTestSuite) TestGetResourceByID_FailsWhenTheStoredCredentialsAreUnreadable() {
	s.mockService.EXPECT().GetAgent(mock.Anything, "agent1", true).
		Return(&model.AgentGetResponse{ID: "agent1", Type: "default", Display: "My Agent"}, nil)
	s.typeDeclaresCredentials("password")
	s.mockEntity.EXPECT().GetCredentialsByType(mock.Anything, "agent1", "password").
		Return(nil, errors.New("store unavailable"))

	_, _, err := s.exporter.GetResourceByID(context.Background(), "agent1")

	s.Require().NotNil(err)
	assert.Equal(s.T(), tidcommon.InternalServerError.Code, err.Code)
}

// An agent with no name has nothing to name a credential after, so none is exported and the type is
// not consulted.
func (s *AgentExporterTestSuite) TestGetResourceByID_ExportsNoCredentialsForAnUnnamedAgent() {
	s.mockService.EXPECT().GetAgent(mock.Anything, "agent1", true).
		Return(&model.AgentGetResponse{ID: "agent1", Type: "default"}, nil)

	resource, _, err := s.exporter.GetResourceByID(context.Background(), "agent1")

	s.Require().Nil(err)
	assert.Empty(s.T(), resource.(*model.AgentGetResponse).Credentials)
}

// The credentials are secret fields, so an import resolves a reference only where one is kept.
func (s *AgentExporterTestSuite) TestSecretFieldsIncludeTheCredentials() {
	lister := s.exporter.(declarativeresource.SecretFieldLister)

	assert.Contains(s.T(), lister.SecretFields(), "credentials.*")
	assert.Contains(s.T(), lister.SecretFields(), "inboundAuthConfig[].config.clientSecret")
}
