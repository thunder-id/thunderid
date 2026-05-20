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

package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/agent/model"
	"github.com/thunder-id/thunderid/internal/cert"
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/inboundclientmock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
)

const (
	testAgentID   = "agent-id-123"
	testAgentName = "test-agent"
	testAgentType = "employee"
	testOUID      = "ou-id-abc"
)

// AgentServiceTestSuite groups all agent service unit tests.
type AgentServiceTestSuite struct {
	suite.Suite
}

func TestAgentServiceTestSuite(t *testing.T) {
	suite.Run(t, new(AgentServiceTestSuite))
}

// setupService wires a service with permissive default mocks. Tests override specific
// expectations as needed.
func (suite *AgentServiceTestSuite) setupService() (
	*agentService,
	*entitymock.EntityServiceInterfaceMock,
	*inboundclientmock.InboundClientServiceInterfaceMock,
	*oumock.OrganizationUnitServiceInterfaceMock,
) {
	mockEntity := entitymock.NewEntityServiceInterfaceMock(suite.T())
	mockInbound := inboundclientmock.NewInboundClientServiceInterfaceMock(suite.T())
	mockOU := oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())

	// Permissive defaults — tests narrow these as needed.
	mockEntity.On("GetEntity", mock.Anything, mock.Anything).
		Maybe().Return((*entity.Entity)(nil), entity.ErrEntityNotFound)
	mockEntity.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything).
		Maybe().Return(&entity.Entity{ID: testAgentID}, nil)
	mockEntity.On("DeleteEntity", mock.Anything, mock.Anything).
		Maybe().Return(nil)
	mockEntity.On("IdentifyEntity", mock.Anything, mock.Anything).
		Maybe().Return((*string)(nil), entity.ErrEntityNotFound)
	mockEntity.On("UpdateEntity", mock.Anything, mock.Anything, mock.Anything).
		Maybe().Return(&entity.Entity{}, nil)
	mockEntity.On("UpdateSystemCredentials", mock.Anything, mock.Anything, mock.Anything).
		Maybe().Return(nil)
	mockEntity.On("GetEntityList", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Maybe().Return([]entity.Entity{}, nil)
	mockEntity.On("GetEntityListCount", mock.Anything, mock.Anything, mock.Anything).
		Maybe().Return(0, nil)
	mockEntity.On("GetEntityListByOUIDs", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Maybe().Return([]entity.Entity{}, nil)
	mockEntity.On("GetEntityListCountByOUIDs", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Maybe().Return(0, nil)
	mockEntity.On("GetEntityGroups", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Maybe().Return([]entity.EntityGroup{}, nil)
	mockEntity.On("GetGroupCountForEntity", mock.Anything, mock.Anything).
		Maybe().Return(0, nil)

	mockInbound.On("GetInboundClientByEntityID", mock.Anything, mock.Anything).
		Maybe().Return((*inboundmodel.InboundClient)(nil), inboundclient.ErrInboundClientNotFound)
	mockInbound.On("GetOAuthProfileByEntityID", mock.Anything, mock.Anything).
		Maybe().Return((*inboundmodel.OAuthProfile)(nil), inboundclient.ErrInboundClientNotFound)
	mockInbound.On("GetCertificate", mock.Anything, mock.Anything, mock.Anything).
		Maybe().Return((*inboundmodel.Certificate)(nil), (*inboundclient.CertOperationError)(nil))
	mockInbound.On("CreateInboundClient", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Maybe().Return(nil)
	mockInbound.On("UpdateInboundClient", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Maybe().Return(nil)
	mockInbound.On("DeleteInboundClient", mock.Anything, mock.Anything).
		Maybe().Return(nil)

	mockOU.On("IsOrganizationUnitExists", mock.Anything, mock.Anything).
		Maybe().Return(true, (*serviceerror.ServiceError)(nil))
	mockOU.On("GetOrganizationUnitByPath", mock.Anything, mock.Anything).
		Maybe().Return(oupkg.OrganizationUnit{ID: testOUID}, (*serviceerror.ServiceError)(nil))
	mockOU.On("GetOrganizationUnitHandlesByIDs", mock.Anything, mock.Anything).
		Maybe().Return(map[string]string{}, (*serviceerror.ServiceError)(nil))

	svc := &agentService{
		logger:               log.GetLogger().With(log.String(log.LoggerKeyComponentName, "AgentService")),
		entityService:        mockEntity,
		inboundClientService: mockInbound,
		ouService:            mockOU,
	}
	return svc, mockEntity, mockInbound, mockOU
}

// buildAgentEntityFixture returns an entity.Entity with system attributes for the given fields.
func buildAgentEntityFixture(name, description, owner, clientID string) *entity.Entity {
	attrs := map[string]interface{}{}
	if name != "" {
		attrs[fieldName] = name
	}
	if description != "" {
		attrs[fieldDescription] = description
	}
	if owner != "" {
		attrs[fieldOwner] = owner
	}
	if clientID != "" {
		attrs[fieldClientID] = clientID
	}
	sysAttrs, _ := json.Marshal(attrs)
	return &entity.Entity{
		ID:               testAgentID,
		Category:         entity.EntityCategoryAgent,
		Type:             testAgentType,
		State:            entity.EntityStateActive,
		OUID:             testOUID,
		SystemAttributes: sysAttrs,
	}
}

// --- pure helper tests ---

func (suite *AgentServiceTestSuite) TestNeedsInboundClient_NilRequest() {
	assert.False(suite.T(), needsInboundClient(nil))
}

func (suite *AgentServiceTestSuite) TestNeedsInboundClient_EmptyRequest() {
	assert.False(suite.T(), needsInboundClient(&model.CreateAgentRequest{}))
}

func (suite *AgentServiceTestSuite) TestNeedsInboundClient_WithAuthFlowID() {
	req := &model.CreateAgentRequest{InboundAuthProfile: inboundmodel.InboundAuthProfile{AuthFlowID: "flow-1"}}
	assert.True(suite.T(), needsInboundClient(req))
}

func (suite *AgentServiceTestSuite) TestNeedsInboundClient_WithInboundAuthConfig() {
	req := &model.CreateAgentRequest{
		InboundAuthConfig: []inboundmodel.InboundAuthConfigWithSecret{
			{Type: inboundmodel.OAuthInboundAuthType, OAuthConfig: &inboundmodel.OAuthConfigWithSecret{}},
		},
	}
	assert.True(suite.T(), needsInboundClient(req))
}

func (suite *AgentServiceTestSuite) TestNeedsInboundClient_WithAllowedUserTypes() {
	req := &model.CreateAgentRequest{
		InboundAuthProfile: inboundmodel.InboundAuthProfile{AllowedUserTypes: []string{"employee"}},
	}
	assert.True(suite.T(), needsInboundClient(req))
}

func (suite *AgentServiceTestSuite) TestUpdateNeedsInboundClient_NilRequest() {
	assert.False(suite.T(), updateNeedsInboundClient(nil))
}

func (suite *AgentServiceTestSuite) TestUpdateNeedsInboundClient_EmptyRequest() {
	assert.False(suite.T(), updateNeedsInboundClient(&model.UpdateAgentRequest{}))
}

func (suite *AgentServiceTestSuite) TestUpdateNeedsInboundClient_WithThemeID() {
	req := &model.UpdateAgentRequest{InboundAuthProfile: inboundmodel.InboundAuthProfile{ThemeID: "theme-abc"}}
	assert.True(suite.T(), updateNeedsInboundClient(req))
}

func (suite *AgentServiceTestSuite) TestRequiresClientSecret_NilConfig() {
	assert.False(suite.T(), requiresClientSecret(nil))
}

func (suite *AgentServiceTestSuite) TestRequiresClientSecret_PublicClient() {
	cfg := &inboundmodel.OAuthConfigWithSecret{PublicClient: true}
	assert.False(suite.T(), requiresClientSecret(cfg))
}

func (suite *AgentServiceTestSuite) TestRequiresClientSecret_ClientSecretBasic() {
	cfg := &inboundmodel.OAuthConfigWithSecret{
		TokenEndpointAuthMethod: oauth2const.TokenEndpointAuthMethodClientSecretBasic,
	}
	assert.True(suite.T(), requiresClientSecret(cfg))
}

func (suite *AgentServiceTestSuite) TestRequiresClientSecret_NoneMethod() {
	cfg := &inboundmodel.OAuthConfigWithSecret{
		TokenEndpointAuthMethod: oauth2const.TokenEndpointAuthMethodNone,
	}
	assert.False(suite.T(), requiresClientSecret(cfg))
}

func (suite *AgentServiceTestSuite) TestRequiresClientSecret_DefaultIsTrue() {
	cfg := &inboundmodel.OAuthConfigWithSecret{}
	assert.True(suite.T(), requiresClientSecret(cfg))
}

// --- readSystemAttributes / buildSystemAttributesJSON ---

func (suite *AgentServiceTestSuite) TestReadSystemAttributes_Empty() {
	name, desc, owner, clientID := readSystemAttributes(nil)
	assert.Empty(suite.T(), name)
	assert.Empty(suite.T(), desc)
	assert.Empty(suite.T(), owner)
	assert.Empty(suite.T(), clientID)
}

func (suite *AgentServiceTestSuite) TestReadSystemAttributes_AllFields() {
	raw, _ := json.Marshal(map[string]interface{}{
		"name":        "my-agent",
		"description": "desc",
		"owner":       "alice",
		"clientId":    "cid-123",
	})
	name, desc, owner, clientID := readSystemAttributes(raw)
	assert.Equal(suite.T(), "my-agent", name)
	assert.Equal(suite.T(), "desc", desc)
	assert.Equal(suite.T(), "alice", owner)
	assert.Equal(suite.T(), "cid-123", clientID)
}

func (suite *AgentServiceTestSuite) TestBuildSystemAttributesJSON_AllFields() {
	raw, err := buildSystemAttributesJSON("n", "d", "o", "c")
	suite.Require().NoError(err)
	suite.Require().NotNil(raw)
	name, desc, owner, clientID := readSystemAttributes(raw)
	assert.Equal(suite.T(), "n", name)
	assert.Equal(suite.T(), "d", desc)
	assert.Equal(suite.T(), "o", owner)
	assert.Equal(suite.T(), "c", clientID)
}

func (suite *AgentServiceTestSuite) TestBuildSystemAttributesJSON_EmptyFields() {
	raw, err := buildSystemAttributesJSON("", "", "", "")
	suite.Require().NoError(err)
	assert.Nil(suite.T(), raw)
}

// --- validateBaseFields ---

func (suite *AgentServiceTestSuite) TestValidateBaseFields_MissingName() {
	svcErr := validateBaseFields("", "type")
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorInvalidAgentName.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestValidateBaseFields_MissingType() {
	svcErr := validateBaseFields("name", "")
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorInvalidAgentType.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestValidateBaseFields_Valid() {
	svcErr := validateBaseFields("name", "type")
	assert.Nil(suite.T(), svcErr)
}

// --- validatePaginationParams ---

func (suite *AgentServiceTestSuite) TestValidatePaginationParams_NegativeLimit() {
	svcErr := validatePaginationParams(-1, 0)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorInvalidLimit.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestValidatePaginationParams_LimitOver100() {
	svcErr := validatePaginationParams(101, 0)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorInvalidLimit.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestValidatePaginationParams_NegativeOffset() {
	svcErr := validatePaginationParams(10, -1)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorInvalidOffset.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestValidatePaginationParams_Valid() {
	svcErr := validatePaginationParams(10, 0)
	assert.Nil(suite.T(), svcErr)
}

// --- CreateAgent ---

func (suite *AgentServiceTestSuite) TestCreateAgent_NilRequest() {
	svc, _, _, _ := suite.setupService()
	resp, svcErr := svc.CreateAgent(context.Background(), nil)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorInvalidRequestFormat.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestCreateAgent_MissingName() {
	svc, _, _, _ := suite.setupService()
	req := &model.CreateAgentRequest{Type: testAgentType, OUID: testOUID}
	resp, svcErr := svc.CreateAgent(context.Background(), req)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorInvalidAgentName.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestCreateAgent_MissingType() {
	svc, _, _, _ := suite.setupService()
	req := &model.CreateAgentRequest{Name: testAgentName, OUID: testOUID}
	resp, svcErr := svc.CreateAgent(context.Background(), req)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorInvalidAgentType.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestCreateAgent_OUNotFound() {
	svc, _, _, mockOU := suite.setupService()
	clearMockCalls(mockOU, "IsOrganizationUnitExists")
	mockOU.On("IsOrganizationUnitExists", mock.Anything, testOUID).Return(false, (*serviceerror.ServiceError)(nil))

	req := &model.CreateAgentRequest{Name: testAgentName, Type: testAgentType, OUID: testOUID}
	resp, svcErr := svc.CreateAgent(context.Background(), req)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorOrganizationUnitNotFound.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestCreateAgent_NameAlreadyExists() {
	svc, mockEntity, _, _ := suite.setupService()
	existingID := "existing-agent-id"
	clearMockCalls(mockEntity, "IdentifyEntity")
	mockEntity.On("IdentifyEntity", mock.Anything, mock.Anything).Return(&existingID, nil)
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, existingID).Return(
		&entity.Entity{ID: existingID, Category: entity.EntityCategoryAgent}, nil)

	req := &model.CreateAgentRequest{Name: testAgentName, Type: testAgentType, OUID: testOUID}
	resp, svcErr := svc.CreateAgent(context.Background(), req)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorAgentAlreadyExistsWithName.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestCreateAgent_EntityOnly_Success() {
	svc, mockEntity, mockInbound, _ := suite.setupService()

	createdEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "CreateEntity")
	mockEntity.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything).
		Return(createdEntity, nil)

	req := &model.CreateAgentRequest{
		Name: testAgentName,
		Type: testAgentType,
		OUID: testOUID,
	}
	resp, svcErr := svc.CreateAgent(context.Background(), req)
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Equal(suite.T(), testAgentName, resp.Name)
	assert.Equal(suite.T(), testAgentType, resp.Type)

	// No inbound client should be created for entity-only agents.
	mockInbound.AssertNotCalled(suite.T(), "CreateInboundClient")
}

func (suite *AgentServiceTestSuite) TestCreateAgent_WithInboundAuth_Success() {
	svc, mockEntity, mockInbound, _ := suite.setupService()

	createdEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "CreateEntity")
	mockEntity.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything).
		Return(createdEntity, nil)

	clearMockCalls(mockInbound, "CreateInboundClient")
	mockInbound.On("CreateInboundClient", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return(nil)

	req := &model.CreateAgentRequest{
		Name:               testAgentName,
		Type:               testAgentType,
		OUID:               testOUID,
		InboundAuthProfile: inboundmodel.InboundAuthProfile{AuthFlowID: "flow-1"},
	}
	resp, svcErr := svc.CreateAgent(context.Background(), req)
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Equal(suite.T(), "flow-1", resp.AuthFlowID)
	mockInbound.AssertCalled(suite.T(), "CreateInboundClient", mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (suite *AgentServiceTestSuite) TestCreateAgent_FlowIDResolvedToDefault() {
	svc, mockEntity, mockInbound, _ := suite.setupService()

	createdEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "CreateEntity")
	mockEntity.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything).
		Return(createdEntity, nil)

	clearMockCalls(mockInbound, "CreateInboundClient")
	mockInbound.On("CreateInboundClient", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			client := args.Get(1).(*inboundmodel.InboundClient)
			client.AuthFlowID = "default-flow-id"
			client.RegistrationFlowID = "default-reg-flow-id"
		}).Return(nil)

	req := &model.CreateAgentRequest{
		Name: testAgentName,
		Type: testAgentType,
		OUID: testOUID,
		InboundAuthConfig: []inboundmodel.InboundAuthConfigWithSecret{
			{
				Type: inboundmodel.OAuthInboundAuthType,
				OAuthConfig: &inboundmodel.OAuthConfigWithSecret{
					GrantTypes:              []oauth2const.GrantType{oauth2const.GrantTypeClientCredentials},
					TokenEndpointAuthMethod: oauth2const.TokenEndpointAuthMethodClientSecretBasic,
				},
			},
		},
	}
	resp, svcErr := svc.CreateAgent(context.Background(), req)
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Equal(suite.T(), "default-flow-id", resp.AuthFlowID)
	assert.Equal(suite.T(), "default-reg-flow-id", resp.RegistrationFlowID)
}

func (suite *AgentServiceTestSuite) TestCreateAgent_WithOAuth_Success() {
	svc, mockEntity, mockInbound, _ := suite.setupService()

	createdEntity := buildAgentEntityFixture(testAgentName, "", "", "cid-xxx")
	clearMockCalls(mockEntity, "CreateEntity")
	mockEntity.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything).
		Return(createdEntity, nil)

	clearMockCalls(mockInbound, "CreateInboundClient")
	mockInbound.On("CreateInboundClient", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return(nil)

	req := &model.CreateAgentRequest{
		Name:               testAgentName,
		Type:               testAgentType,
		OUID:               testOUID,
		InboundAuthProfile: inboundmodel.InboundAuthProfile{AuthFlowID: "flow-1"},
		InboundAuthConfig: []inboundmodel.InboundAuthConfigWithSecret{
			{
				Type: inboundmodel.OAuthInboundAuthType,
				OAuthConfig: &inboundmodel.OAuthConfigWithSecret{
					GrantTypes:              []oauth2const.GrantType{oauth2const.GrantTypeClientCredentials},
					TokenEndpointAuthMethod: oauth2const.TokenEndpointAuthMethodClientSecretBasic,
				},
			},
		},
	}
	resp, svcErr := svc.CreateAgent(context.Background(), req)
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	suite.Require().Len(resp.InboundAuthConfig, 1)
	assert.Equal(suite.T(), inboundmodel.OAuthInboundAuthType, resp.InboundAuthConfig[0].Type)
	assert.NotEmpty(suite.T(), resp.InboundAuthConfig[0].OAuthConfig.ClientID)
	assert.NotEmpty(suite.T(), resp.InboundAuthConfig[0].OAuthConfig.ClientSecret)
}

func (suite *AgentServiceTestSuite) TestCreateAgent_EntityCreationFails() {
	svc, mockEntity, _, _ := suite.setupService()

	clearMockCalls(mockEntity, "CreateEntity")
	mockEntity.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything).
		Return((*entity.Entity)(nil), entity.ErrSchemaValidationFailed)

	req := &model.CreateAgentRequest{Name: testAgentName, Type: testAgentType, OUID: testOUID}
	resp, svcErr := svc.CreateAgent(context.Background(), req)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorSchemaValidationFailed.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestCreateAgent_InboundCreationFails_CompensatesEntity() {
	svc, mockEntity, mockInbound, _ := suite.setupService()

	createdEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "CreateEntity")
	mockEntity.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything).
		Return(createdEntity, nil)

	clearMockCalls(mockInbound, "CreateInboundClient")
	mockInbound.On("CreateInboundClient", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(inboundclient.ErrOAuthInvalidGrantType)

	clearMockCalls(mockEntity, "DeleteEntity")
	mockEntity.On("DeleteEntity", mock.Anything, mock.Anything).Return(nil)

	req := &model.CreateAgentRequest{
		Name:               testAgentName,
		Type:               testAgentType,
		OUID:               testOUID,
		InboundAuthProfile: inboundmodel.InboundAuthProfile{AuthFlowID: "flow-1"},
	}
	resp, svcErr := svc.CreateAgent(context.Background(), req)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorInvalidGrantType.Code, svcErr.Code)
	mockEntity.AssertCalled(suite.T(), "DeleteEntity", mock.Anything, mock.Anything)
}

// --- GetAgent ---

func (suite *AgentServiceTestSuite) TestGetAgent_EmptyID() {
	svc, _, _, _ := suite.setupService()
	resp, svcErr := svc.GetAgent(context.Background(), "", false)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorMissingAgentID.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestGetAgent_NotFound() {
	svc, _, _, _ := suite.setupService()
	// Default mock returns ErrEntityNotFound.
	resp, svcErr := svc.GetAgent(context.Background(), testAgentID, false)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorAgentNotFound.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestGetAgent_WrongCategory() {
	svc, mockEntity, _, _ := suite.setupService()

	wrongCatEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	wrongCatEntity.Category = entity.EntityCategoryUser

	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(wrongCatEntity, nil)

	resp, svcErr := svc.GetAgent(context.Background(), testAgentID, false)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorAgentNotFound.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestGetAgent_Success_NoInbound() {
	svc, mockEntity, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "desc", "alice", "")

	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	resp, svcErr := svc.GetAgent(context.Background(), testAgentID, false)
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Equal(suite.T(), testAgentID, resp.ID)
	assert.Equal(suite.T(), testAgentName, resp.Name)
	assert.Equal(suite.T(), "desc", resp.Description)
	assert.Equal(suite.T(), "alice", resp.Owner)
	assert.Nil(suite.T(), resp.InboundAuthConfig)
}

func (suite *AgentServiceTestSuite) TestGetAgent_Success_WithOAuth() {
	svc, mockEntity, mockInbound, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "cid-123")

	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	inboundRec := &inboundmodel.InboundClient{ID: testAgentID, AuthFlowID: "flow-1"}
	clearMockCalls(mockInbound, "GetInboundClientByEntityID")
	mockInbound.On("GetInboundClientByEntityID", mock.Anything, testAgentID).Return(inboundRec, nil)

	oauthProfile := &inboundmodel.OAuthProfile{
		GrantTypes: []string{"client_credentials"},
	}
	clearMockCalls(mockInbound, "GetOAuthProfileByEntityID")
	mockInbound.On("GetOAuthProfileByEntityID", mock.Anything, testAgentID).Return(oauthProfile, nil)

	resp, svcErr := svc.GetAgent(context.Background(), testAgentID, false)
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Equal(suite.T(), "flow-1", resp.AuthFlowID)
	suite.Require().Len(resp.InboundAuthConfig, 1)
	// ClientSecret is structurally absent on the GET response (OAuthConfig has no field).
	assert.Equal(suite.T(), "cid-123", resp.InboundAuthConfig[0].OAuthConfig.ClientID)
}

// --- DeleteAgent ---

func (suite *AgentServiceTestSuite) TestDeleteAgent_EmptyID() {
	svc, _, _, _ := suite.setupService()
	svcErr := svc.DeleteAgent(context.Background(), "")
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorMissingAgentID.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestDeleteAgent_NotFound() {
	svc, _, _, _ := suite.setupService()
	svcErr := svc.DeleteAgent(context.Background(), testAgentID)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorAgentNotFound.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestDeleteAgent_Success_NoInboundClient() {
	svc, mockEntity, mockInbound, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockInbound, "DeleteInboundClient")
	mockInbound.On("DeleteInboundClient", mock.Anything, testAgentID).
		Return(inboundclient.ErrInboundClientNotFound)

	clearMockCalls(mockEntity, "DeleteEntity")
	mockEntity.On("DeleteEntity", mock.Anything, testAgentID).Return(nil)

	svcErr := svc.DeleteAgent(context.Background(), testAgentID)
	assert.Nil(suite.T(), svcErr)
	mockEntity.AssertCalled(suite.T(), "DeleteEntity", mock.Anything, testAgentID)
}

func (suite *AgentServiceTestSuite) TestDeleteAgent_Success_WithInboundClient() {
	svc, mockEntity, mockInbound, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "cid-abc")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockInbound, "DeleteInboundClient")
	mockInbound.On("DeleteInboundClient", mock.Anything, testAgentID).Return(nil)

	clearMockCalls(mockEntity, "DeleteEntity")
	mockEntity.On("DeleteEntity", mock.Anything, testAgentID).Return(nil)

	svcErr := svc.DeleteAgent(context.Background(), testAgentID)
	assert.Nil(suite.T(), svcErr)
	mockInbound.AssertCalled(suite.T(), "DeleteInboundClient", mock.Anything, testAgentID)
	mockEntity.AssertCalled(suite.T(), "DeleteEntity", mock.Anything, testAgentID)
}

// --- GetAgentList ---

func (suite *AgentServiceTestSuite) TestGetAgentList_InvalidLimit() {
	svc, _, _, _ := suite.setupService()
	resp, svcErr := svc.GetAgentList(context.Background(), -1, 0, nil, false)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorInvalidLimit.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestGetAgentList_Success() {
	svc, mockEntity, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "desc", "alice", "")
	clearMockCalls(mockEntity, "GetEntityList")
	mockEntity.On("GetEntityList", mock.Anything, entity.EntityCategoryAgent, 30, 0, mock.Anything).
		Return([]entity.Entity{*agentEntity}, nil)
	clearMockCalls(mockEntity, "GetEntityListCount")
	mockEntity.On("GetEntityListCount", mock.Anything, entity.EntityCategoryAgent, mock.Anything).
		Return(1, nil)

	resp, svcErr := svc.GetAgentList(context.Background(), 0, 0, nil, false)
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Equal(suite.T(), 1, resp.TotalResults)
	suite.Require().Len(resp.Agents, 1)
	assert.Equal(suite.T(), testAgentName, resp.Agents[0].Name)
}

// --- GetAgentGroups ---

func (suite *AgentServiceTestSuite) TestGetAgentGroups_EmptyID() {
	svc, _, _, _ := suite.setupService()
	resp, svcErr := svc.GetAgentGroups(context.Background(), "", 10, 0)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorMissingAgentID.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestGetAgentGroups_AgentNotFound() {
	svc, _, _, _ := suite.setupService()
	resp, svcErr := svc.GetAgentGroups(context.Background(), testAgentID, 10, 0)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorAgentNotFound.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestGetAgentGroups_Success() {
	svc, mockEntity, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockEntity, "GetGroupCountForEntity")
	mockEntity.On("GetGroupCountForEntity", mock.Anything, testAgentID).Return(2, nil)

	clearMockCalls(mockEntity, "GetEntityGroups")
	mockEntity.On("GetEntityGroups", mock.Anything, testAgentID, 10, 0).
		Return([]entity.EntityGroup{
			{ID: "g1", Name: "group-one", OUID: testOUID},
			{ID: "g2", Name: "group-two", OUID: testOUID},
		}, nil)

	resp, svcErr := svc.GetAgentGroups(context.Background(), testAgentID, 10, 0)
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Equal(suite.T(), 2, resp.TotalResults)
	assert.Len(suite.T(), resp.Groups, 2)
}

// --- UpdateAgent ---

func (suite *AgentServiceTestSuite) TestUpdateAgent_EmptyID() {
	svc, _, _, _ := suite.setupService()
	resp, svcErr := svc.UpdateAgent(context.Background(), "", &model.UpdateAgentRequest{
		Name: testAgentName, Type: testAgentType,
	})
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorMissingAgentID.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_NilRequest() {
	svc, _, _, _ := suite.setupService()
	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, nil)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorInvalidRequestFormat.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_AgentNotFound() {
	svc, _, _, _ := suite.setupService()
	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Name: testAgentName, Type: testAgentType,
	})
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorAgentNotFound.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_Success_EntityOnly() {
	svc, mockEntity, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture("old-name", "", "", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockEntity, "UpdateEntity")
	mockEntity.On("UpdateEntity", mock.Anything, testAgentID, mock.Anything).
		Return(&entity.Entity{}, nil)

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Name: testAgentName, Type: testAgentType,
	})
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Equal(suite.T(), testAgentName, resp.Name)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_FlowIDResolvedToDefault() {
	svc, mockEntity, mockInbound, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture("old-name", "", "", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockEntity, "UpdateEntity")
	mockEntity.On("UpdateEntity", mock.Anything, testAgentID, mock.Anything).
		Return(&entity.Entity{}, nil)

	clearMockCalls(mockInbound, "GetInboundClientByEntityID")
	mockInbound.On("GetInboundClientByEntityID", mock.Anything, mock.Anything).
		Return(&inboundmodel.InboundClient{}, nil)

	clearMockCalls(mockInbound, "UpdateInboundClient")
	mockInbound.On("UpdateInboundClient", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			client := args.Get(1).(*inboundmodel.InboundClient)
			client.AuthFlowID = "default-flow-id"
			client.RegistrationFlowID = "default-reg-flow-id"
		}).Return(nil)

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Name: testAgentName,
		Type: testAgentType,
		InboundAuthConfig: []inboundmodel.InboundAuthConfigWithSecret{
			{
				Type: inboundmodel.OAuthInboundAuthType,
				OAuthConfig: &inboundmodel.OAuthConfigWithSecret{
					GrantTypes:              []oauth2const.GrantType{oauth2const.GrantTypeClientCredentials},
					TokenEndpointAuthMethod: oauth2const.TokenEndpointAuthMethodClientSecretBasic,
				},
			},
		},
	})
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Equal(suite.T(), "default-flow-id", resp.AuthFlowID)
	assert.Equal(suite.T(), "default-reg-flow-id", resp.RegistrationFlowID)
}

// --- owner validation ---

func (suite *AgentServiceTestSuite) TestValidateOwnerExists_Empty() {
	svc, _, _, _ := suite.setupService()
	assert.Nil(suite.T(), svc.validateOwnerExists(context.Background(), ""))
}

func (suite *AgentServiceTestSuite) TestValidateOwnerExists_NotFound() {
	svc, mockEntity, _, _ := suite.setupService()
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, "missing-owner").
		Return((*entity.Entity)(nil), entity.ErrEntityNotFound)

	svcErr := svc.validateOwnerExists(context.Background(), "missing-owner")
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorOwnerNotFound.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestValidateOwnerExists_StoreError() {
	svc, mockEntity, _, _ := suite.setupService()
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, "owner-x").
		Return((*entity.Entity)(nil), errors.New("db error"))

	svcErr := svc.validateOwnerExists(context.Background(), "owner-x")
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), serviceerror.InternalServerError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestValidateOwnerExists_Success() {
	svc, mockEntity, _, _ := suite.setupService()
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, "owner-y").
		Return(&entity.Entity{ID: "owner-y"}, nil)

	assert.Nil(suite.T(), svc.validateOwnerExists(context.Background(), "owner-y"))
}

func (suite *AgentServiceTestSuite) TestCreateAgent_OwnerNotFound() {
	svc, mockEntity, _, _ := suite.setupService()
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, "ghost").
		Return((*entity.Entity)(nil), entity.ErrEntityNotFound)

	resp, svcErr := svc.CreateAgent(context.Background(), &model.CreateAgentRequest{
		Name: testAgentName, Type: testAgentType, OUID: testOUID, Owner: "ghost",
	})
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorOwnerNotFound.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_OwnerChanged_OwnerNotFound() {
	svc, mockEntity, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture("old-name", "", "current-owner", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)
	mockEntity.On("GetEntity", mock.Anything, "new-owner").
		Return((*entity.Entity)(nil), entity.ErrEntityNotFound)

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Name: testAgentName, Type: testAgentType, Owner: "new-owner",
	})
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorOwnerNotFound.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_OwnerChanged_Success() {
	svc, mockEntity, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture("old-name", "", "current-owner", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)
	mockEntity.On("GetEntity", mock.Anything, "new-owner").
		Return(&entity.Entity{ID: "new-owner"}, nil)

	clearMockCalls(mockEntity, "UpdateEntity")
	mockEntity.On("UpdateEntity", mock.Anything, testAgentID, mock.Anything).
		Return(&entity.Entity{}, nil)

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Name: testAgentName, Type: testAgentType, Owner: "new-owner",
	})
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Equal(suite.T(), "new-owner", resp.Owner)
}

// --- mapEntityError ---

func (suite *AgentServiceTestSuite) TestMapEntityError_NotFound() {
	svcErr := mapEntityError(entity.ErrEntityNotFound)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorAgentNotFound.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestMapEntityError_SchemaValidation() {
	svcErr := mapEntityError(entity.ErrSchemaValidationFailed)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorSchemaValidationFailed.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestMapEntityError_Unknown() {
	svcErr := mapEntityError(entity.ErrAmbiguousEntity)
	assert.Nil(suite.T(), svcErr)
}

// --- translateInboundClientError ---

func (suite *AgentServiceTestSuite) TestTranslateInboundClientError_InvalidRedirectURI() {
	svc, _, _, _ := suite.setupService()
	svcErr := svc.translateInboundClientError(inboundclient.ErrOAuthInvalidRedirectURI)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorInvalidRedirectURI.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestTranslateInboundClientError_InvalidGrantType() {
	svc, _, _, _ := suite.setupService()
	svcErr := svc.translateInboundClientError(inboundclient.ErrOAuthInvalidGrantType)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorInvalidGrantType.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestTranslateInboundClientError_Unknown() {
	svc, _, _, _ := suite.setupService()
	svcErr := svc.translateInboundClientError(errors.New("unknown error"))
	assert.Nil(suite.T(), svcErr)
}

// --- translateOAuthValidationError ---

func (suite *AgentServiceTestSuite) TestTranslateOAuthValidationError() {
	cases := []struct {
		name        string
		err         error
		wantCode    string
		wantDescKey string
	}{
		{"InvalidRedirectURI", inboundclient.ErrOAuthInvalidRedirectURI, ErrorInvalidRedirectURI.Code, ""},
		{"RedirectURIFragmentNotAllowed", inboundclient.ErrOAuthRedirectURIFragmentNotAllowed,
			ErrorInvalidRedirectURI.Code,
			"error.agentservice.redirect_uri_fragment_not_allowed_description"},
		{"AuthCodeRequiresRedirectURIs", inboundclient.ErrOAuthAuthCodeRequiresRedirectURIs,
			ErrorInvalidOAuthConfiguration.Code,
			"error.agentservice.auth_code_requires_redirect_uris_description"},
		{"InvalidGrantType", inboundclient.ErrOAuthInvalidGrantType, ErrorInvalidGrantType.Code, ""},
		{"InvalidResponseType", inboundclient.ErrOAuthInvalidResponseType, ErrorInvalidResponseType.Code, ""},
		{"ClientCredentialsCannotUseResponseTypes", inboundclient.ErrOAuthClientCredentialsCannotUseResponseTypes,
			ErrorInvalidOAuthConfiguration.Code,
			"error.agentservice.client_credentials_cannot_use_response_types_description"},
		{"AuthCodeRequiresCodeResponseType", inboundclient.ErrOAuthAuthCodeRequiresCodeResponseType,
			ErrorInvalidOAuthConfiguration.Code,
			"error.agentservice.auth_code_requires_code_response_type_description"},
		{"RefreshTokenCannotBeSoleGrant", inboundclient.ErrOAuthRefreshTokenCannotBeSoleGrant,
			ErrorInvalidOAuthConfiguration.Code,
			"error.agentservice.refresh_token_cannot_be_sole_grant_description"},
		{"PKCERequiresAuthCode", inboundclient.ErrOAuthPKCERequiresAuthCode,
			ErrorInvalidOAuthConfiguration.Code,
			"error.agentservice.pkce_requires_authorization_code_description"},
		{"ResponseTypesRequireAuthCode", inboundclient.ErrOAuthResponseTypesRequireAuthCode,
			ErrorInvalidOAuthConfiguration.Code,
			"error.agentservice.response_types_require_authorization_code_description"},
		{"InvalidTokenEndpointAuthMethod", inboundclient.ErrOAuthInvalidTokenEndpointAuthMethod,
			ErrorInvalidTokenEndpointAuthMethod.Code, ""},
		{"PrivateKeyJWTRequiresCertificate", inboundclient.ErrOAuthPrivateKeyJWTRequiresCertificate,
			ErrorInvalidOAuthConfiguration.Code,
			"error.agentservice.private_key_jwt_requires_certificate_description"},
		{"PrivateKeyJWTCannotHaveClientSecret", inboundclient.ErrOAuthPrivateKeyJWTCannotHaveClientSecret,
			ErrorInvalidOAuthConfiguration.Code,
			"error.agentservice.private_key_jwt_cannot_have_client_secret_description"},
		{"ClientSecretCannotHaveCertificate", inboundclient.ErrOAuthClientSecretCannotHaveCertificate,
			ErrorInvalidOAuthConfiguration.Code,
			"error.agentservice.client_secret_cannot_have_certificate_description"},
		{"NoneAuthRequiresPublicClient", inboundclient.ErrOAuthNoneAuthRequiresPublicClient,
			ErrorInvalidOAuthConfiguration.Code,
			"error.agentservice.none_auth_method_requires_public_client_description"},
		{"NoneAuthCannotHaveCertOrSecret", inboundclient.ErrOAuthNoneAuthCannotHaveCertOrSecret,
			ErrorInvalidOAuthConfiguration.Code,
			"error.agentservice.none_auth_method_cannot_have_cert_or_secret_description"},
		{"ClientCredentialsCannotUseNoneAuth", inboundclient.ErrOAuthClientCredentialsCannotUseNoneAuth,
			ErrorInvalidOAuthConfiguration.Code,
			"error.agentservice.client_credentials_cannot_use_none_auth_description"},
		{"PublicClientMustUseNoneAuth", inboundclient.ErrOAuthPublicClientMustUseNoneAuth,
			ErrorInvalidPublicClientConfiguration.Code,
			"error.agentservice.public_client_must_use_none_auth_description"},
		{"PublicClientMustHavePKCE", inboundclient.ErrOAuthPublicClientMustHavePKCE,
			ErrorInvalidPublicClientConfiguration.Code,
			"error.agentservice.public_client_must_have_pkce_description"},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			svcErr := translateOAuthValidationError(tc.err)
			suite.Require().NotNil(svcErr)
			suite.Equal(tc.wantCode, svcErr.Code)
			if tc.wantDescKey != "" {
				suite.Equal(tc.wantDescKey, svcErr.ErrorDescription.Key)
			}
		})
	}
	suite.Nil(translateOAuthValidationError(errors.New("unknown")))
}

// --- translateUserInfoValidationError ---

func (suite *AgentServiceTestSuite) TestTranslateUserInfoValidationError() {
	cases := []struct {
		name        string
		err         error
		wantDescKey string
	}{
		{"UnsupportedSigningAlg", inboundclient.ErrOAuthUserInfoUnsupportedSigningAlg,
			"error.agentservice.userinfo_unsupported_signing_alg_description"},
		{"UnsupportedEncryptionAlg", inboundclient.ErrOAuthUserInfoUnsupportedEncryptionAlg,
			"error.agentservice.userinfo_unsupported_encryption_alg_description"},
		{"UnsupportedEncryptionEnc", inboundclient.ErrOAuthUserInfoUnsupportedEncryptionEnc,
			"error.agentservice.userinfo_unsupported_encryption_enc_description"},
		{"EncryptionAlgRequiresEnc", inboundclient.ErrOAuthUserInfoEncryptionAlgRequiresEnc,
			"error.agentservice.userinfo_encryption_alg_requires_enc_description"},
		{"EncryptionEncRequiresAlg", inboundclient.ErrOAuthUserInfoEncryptionEncRequiresAlg,
			"error.agentservice.userinfo_encryption_enc_requires_alg_description"},
		{"EncryptionRequiresCertificate", inboundclient.ErrOAuthUserInfoEncryptionRequiresCertificate,
			"error.agentservice.userinfo_encryption_requires_certificate_description"},
		{"JWKSURINotSSRFSafe", inboundclient.ErrOAuthUserInfoJWKSURINotSSRFSafe,
			"error.agentservice.userinfo_jwks_uri_not_ssrf_safe_description"},
		{"UnsupportedResponseType", inboundclient.ErrOAuthUserInfoUnsupportedResponseType,
			"error.agentservice.userinfo_unsupported_response_type_description"},
		{"JWSRequiresSigningAlg", inboundclient.ErrOAuthUserInfoJWSRequiresSigningAlg,
			"error.agentservice.userinfo_jws_requires_signing_alg_description"},
		{"JWERequiresEncryption", inboundclient.ErrOAuthUserInfoJWERequiresEncryption,
			"error.agentservice.userinfo_jwe_requires_encryption_description"},
		{"NestedJWTRequiresAll", inboundclient.ErrOAuthUserInfoNestedJWTRequiresAll,
			"error.agentservice.userinfo_nested_jwt_requires_all_description"},
		{"AlgRequiresResponseType", inboundclient.ErrOAuthUserInfoAlgRequiresResponseType,
			"error.agentservice.userinfo_alg_requires_response_type_description"},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			svcErr := translateUserInfoValidationError(tc.err)
			suite.Require().NotNil(svcErr)
			suite.Equal(ErrorInvalidOAuthConfiguration.Code, svcErr.Code)
			suite.Equal(tc.wantDescKey, svcErr.ErrorDescription.Key)
		})
	}
	suite.Nil(translateUserInfoValidationError(errors.New("unknown")))
}

// --- translateIDTokenValidationError ---

func (suite *AgentServiceTestSuite) TestTranslateIDTokenValidationError() {
	cases := []struct {
		name        string
		err         error
		wantDescKey string
	}{
		{"EncryptionFieldsNotAllowed", inboundclient.ErrOAuthIDTokenEncryptionFieldsNotAllowed,
			"error.agentservice.idtoken_encryption_fields_not_allowed_description"},
		{"UnsupportedResponseType", inboundclient.ErrOAuthIDTokenUnsupportedResponseType,
			"error.agentservice.idtoken_unsupported_response_type_description"},
		{"UnsupportedEncryptionAlg", inboundclient.ErrOAuthIDTokenUnsupportedEncryptionAlg,
			"error.agentservice.idtoken_unsupported_encryption_alg_description"},
		{"UnsupportedEncryptionEnc", inboundclient.ErrOAuthIDTokenUnsupportedEncryptionEnc,
			"error.agentservice.idtoken_unsupported_encryption_enc_description"},
		{"EncryptionAlgRequiresEnc", inboundclient.ErrOAuthIDTokenEncryptionAlgRequiresEnc,
			"error.agentservice.idtoken_encryption_alg_requires_enc_description"},
		{"EncryptionEncRequiresAlg", inboundclient.ErrOAuthIDTokenEncryptionEncRequiresAlg,
			"error.agentservice.idtoken_encryption_enc_requires_alg_description"},
		{"EncryptionRequiresCertificate", inboundclient.ErrOAuthIDTokenEncryptionRequiresCertificate,
			"error.agentservice.idtoken_encryption_requires_certificate_description"},
		{"JWKSURINotSSRFSafe", inboundclient.ErrOAuthIDTokenJWKSURINotSSRFSafe,
			"error.agentservice.idtoken_jwks_uri_not_ssrf_safe_description"},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			svcErr := translateIDTokenValidationError(tc.err)
			suite.Require().NotNil(svcErr)
			suite.Equal(ErrorInvalidOAuthConfiguration.Code, svcErr.Code)
			suite.Equal(tc.wantDescKey, svcErr.ErrorDescription.Key)
		})
	}
	suite.Nil(translateIDTokenValidationError(errors.New("unknown")))
}

// --- translateInboundClientFKError ---

func (suite *AgentServiceTestSuite) TestTranslateInboundClientFKError() {
	cases := []struct {
		name     string
		err      error
		wantCode string
	}{
		{"InvalidAuthFlow", inboundclient.ErrFKInvalidAuthFlow, ErrorInvalidAuthFlowID.Code},
		{"InvalidRegistrationFlow", inboundclient.ErrFKInvalidRegistrationFlow, ErrorInvalidRegistrationFlowID.Code},
		{"FlowDefinitionRetrievalFailed", inboundclient.ErrFKFlowDefinitionRetrievalFailed,
			ErrorWhileRetrievingFlowDefinition.Code},
		{"FlowServerError", inboundclient.ErrFKFlowServerError, serviceerror.InternalServerError.Code},
		{"ThemeNotFound", inboundclient.ErrFKThemeNotFound, ErrorThemeNotFound.Code},
		{"LayoutNotFound", inboundclient.ErrFKLayoutNotFound, ErrorLayoutNotFound.Code},
		{"InvalidUserType", inboundclient.ErrFKInvalidUserType, ErrorInvalidUserType.Code},
		{"UserSchemaLookupFailed", inboundclient.ErrUserSchemaLookupFailed, serviceerror.InternalServerError.Code},
		{"InvalidUserAttribute", inboundclient.ErrInvalidUserAttribute, ErrorInvalidUserAttribute.Code},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			svcErr := translateInboundClientFKError(tc.err)
			suite.Require().NotNil(svcErr)
			suite.Equal(tc.wantCode, svcErr.Code)
		})
	}
	suite.Nil(translateInboundClientFKError(errors.New("unknown")))
}

// --- translateCertValidationError ---

func (suite *AgentServiceTestSuite) TestTranslateCertValidationError() {
	cases := []struct {
		name     string
		err      error
		wantCode string
	}{
		{"ValueRequired", inboundclient.ErrCertValueRequired, ErrorInvalidCertificateValue.Code},
		{"InvalidJWKSURI", inboundclient.ErrCertInvalidJWKSURI, ErrorInvalidJWKSURI.Code},
		{"InvalidType", inboundclient.ErrCertInvalidType, ErrorInvalidCertificateType.Code},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			svcErr := translateCertValidationError(tc.err)
			suite.Require().NotNil(svcErr)
			suite.Equal(tc.wantCode, svcErr.Code)
		})
	}
	suite.Nil(translateCertValidationError(errors.New("unknown")))
}

// --- translateCertOperationError ---

func (suite *AgentServiceTestSuite) TestTranslateCertOperationError() {
	s := &agentService{logger: log.GetLogger().With(log.String("component", "test"))}

	cases := []struct {
		name        string
		op          string
		refType     cert.CertificateReferenceType
		underlying  *serviceerror.ServiceError
		wantCode    string
		wantDescKey string
	}{
		{"CreateClientErr", inboundclient.CertOpCreate, cert.CertificateReferenceTypeApplication,
			&serviceerror.ServiceError{Type: serviceerror.ClientErrorType, Code: "X-1",
				ErrorDescription: core.I18nMessage{DefaultValue: "underlying"}},
			ErrorCertificateClientError.Code, "error.agentservice.create_certificate_failed_description"},
		{"UpdateClientErr", inboundclient.CertOpUpdate, cert.CertificateReferenceTypeApplication,
			&serviceerror.ServiceError{Type: serviceerror.ClientErrorType, Code: "X-2",
				ErrorDescription: core.I18nMessage{DefaultValue: "underlying"}},
			ErrorCertificateClientError.Code, "error.agentservice.update_certificate_failed_description"},
		{"RetrieveClientErr", inboundclient.CertOpRetrieve, cert.CertificateReferenceTypeApplication,
			&serviceerror.ServiceError{Type: serviceerror.ClientErrorType, Code: "X-3",
				ErrorDescription: core.I18nMessage{DefaultValue: "underlying"}},
			ErrorCertificateClientError.Code, "error.agentservice.retrieve_certificate_failed_description"},
		{"DeleteAppRefClientErr", inboundclient.CertOpDelete, cert.CertificateReferenceTypeApplication,
			&serviceerror.ServiceError{Type: serviceerror.ClientErrorType, Code: "X-4",
				ErrorDescription: core.I18nMessage{DefaultValue: "underlying"}},
			ErrorCertificateClientError.Code, "error.agentservice.delete_certificate_failed_description"},
		{"DeleteOAuthRefClientErr", inboundclient.CertOpDelete, cert.CertificateReferenceTypeOAuthApp,
			&serviceerror.ServiceError{Type: serviceerror.ClientErrorType, Code: "X-5",
				ErrorDescription: core.I18nMessage{DefaultValue: "underlying"}},
			ErrorCertificateClientError.Code, "error.agentservice.delete_oauth_certificate_failed_description"},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			opErr := &inboundclient.CertOperationError{
				Operation: tc.op, RefType: tc.refType, Underlying: tc.underlying,
			}
			svcErr := s.translateCertOperationError(opErr)
			suite.Require().NotNil(svcErr)
			suite.Equal(tc.wantCode, svcErr.Code)
			suite.Equal(tc.wantDescKey, svcErr.ErrorDescription.Key)
			suite.Contains(svcErr.ErrorDescription.DefaultValue, "underlying")
		})
	}

	// Server error returns InternalServerError.
	serverErrOp := &inboundclient.CertOperationError{
		Operation:  inboundclient.CertOpCreate,
		RefType:    cert.CertificateReferenceTypeApplication,
		Underlying: &serviceerror.ServiceError{Type: serviceerror.ServerErrorType, Code: "X-S"},
	}
	suite.Equal(serviceerror.InternalServerError.Code, s.translateCertOperationError(serverErrOp).Code)

	// Unknown operation returns InternalServerError.
	unknownOp := &inboundclient.CertOperationError{
		Operation:  "weird",
		Underlying: &serviceerror.ServiceError{Type: serviceerror.ClientErrorType, Code: "X-?"},
	}
	suite.Equal(serviceerror.InternalServerError.Code, s.translateCertOperationError(unknownOp).Code)
}

// --- translateConsentSyncError ---

func (suite *AgentServiceTestSuite) TestTranslateConsentSyncError() {
	clientErr := &inboundclient.ConsentSyncError{
		Underlying: &serviceerror.ServiceError{Type: serviceerror.ClientErrorType, Code: "CONSENT-1234"},
	}
	svcErr := translateConsentSyncError(clientErr)
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorConsentSyncFailed.Code, svcErr.Code)
	suite.Equal("error.agentservice.consent_sync_failed_description", svcErr.ErrorDescription.Key)
	suite.Contains(svcErr.ErrorDescription.DefaultValue, "CONSENT-1234")

	serverErr := &inboundclient.ConsentSyncError{
		Underlying: &serviceerror.ServiceError{Type: serviceerror.ServerErrorType, Code: "CONSENT-9000"},
	}
	suite.Equal(serviceerror.InternalServerError.Code, translateConsentSyncError(serverErr).Code)
}

// --- reconcileInboundForUpdate not-found short-circuit ---

// When the update request has no inbound config and the existing inbound client delete returns
// ErrInboundClientNotFound, the update should still succeed (no-op).
func (suite *AgentServiceTestSuite) TestUpdateAgent_NoInboundWanted_DeleteNotFound_Succeeds() {
	svc, mockEntity, mockInbound, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture("old-name", "", "", "cid-abc")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockEntity, "UpdateEntity")
	mockEntity.On("UpdateEntity", mock.Anything, testAgentID, mock.Anything).
		Return(&entity.Entity{}, nil)

	// Existing inbound client present (so hasExisting=true), but delete reports not-found.
	clearMockCalls(mockInbound, "GetInboundClientByEntityID")
	mockInbound.On("GetInboundClientByEntityID", mock.Anything, testAgentID).
		Return(&inboundmodel.InboundClient{ID: testAgentID}, nil)

	clearMockCalls(mockInbound, "DeleteInboundClient")
	mockInbound.On("DeleteInboundClient", mock.Anything, testAgentID).
		Return(inboundclient.ErrInboundClientNotFound)

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Name: testAgentName, Type: testAgentType,
	})
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	mockInbound.AssertCalled(suite.T(), "DeleteInboundClient", mock.Anything, testAgentID)
}

// --- helpers ---

// clearMockCalls removes all expectations for the named method from the mock, so a test
// can register a more specific expectation without conflicting with the permissive default.
func clearMockCalls(m any, method string) {
	var mockObj *mock.Mock
	switch v := m.(type) {
	case *entitymock.EntityServiceInterfaceMock:
		mockObj = &v.Mock
	case *inboundclientmock.InboundClientServiceInterfaceMock:
		mockObj = &v.Mock
	case *oumock.OrganizationUnitServiceInterfaceMock:
		mockObj = &v.Mock
	}
	if mockObj == nil {
		return
	}
	var kept []*mock.Call
	for _, c := range mockObj.ExpectedCalls {
		if c.Method != method {
			kept = append(kept, c)
		}
	}
	mockObj.ExpectedCalls = kept
}
