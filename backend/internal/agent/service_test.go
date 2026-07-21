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

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/agent/model"
	"github.com/thunder-id/thunderid/internal/cert"
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/inboundclientmock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
	"github.com/thunder-id/thunderid/tests/mocks/rolemock"
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
	*rolemock.RoleServiceInterfaceMock,
) {
	mockEntity := entitymock.NewEntityServiceInterfaceMock(suite.T())
	mockInbound := inboundclientmock.NewInboundClientServiceInterfaceMock(suite.T())
	mockOU := oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())
	mockRole := rolemock.NewRoleServiceInterfaceMock(suite.T())

	// Permissive defaults — tests narrow these as needed.
	mockEntity.On("GetEntity", mock.Anything, mock.Anything).
		Maybe().Return((*providers.Entity)(nil), entity.ErrEntityNotFound)
	mockEntity.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything).
		Maybe().Return(&providers.Entity{ID: testAgentID}, nil)
	mockEntity.On("DeleteEntity", mock.Anything, mock.Anything).
		Maybe().Return(nil)
	mockEntity.On("IdentifyEntity", mock.Anything, mock.Anything).
		Maybe().Return((*string)(nil), entity.ErrEntityNotFound)
	mockEntity.On("UpdateEntity", mock.Anything, mock.Anything, mock.Anything).
		Maybe().Return(&providers.Entity{}, nil)
	mockEntity.On("UpdateSystemCredentials", mock.Anything, mock.Anything, mock.Anything).
		Maybe().Return(nil)
	mockEntity.On("GetEntityList", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Maybe().Return([]providers.Entity{}, nil)
	mockEntity.On("GetEntityListCount", mock.Anything, mock.Anything, mock.Anything).
		Maybe().Return(0, nil)
	mockEntity.On("GetEntityListByOUIDs", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Maybe().Return([]providers.Entity{}, nil)
	mockEntity.On("GetEntityListCountByOUIDs", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Maybe().Return(0, nil)
	mockEntity.On("GetEntityGroups", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Maybe().Return([]providers.EntityGroup{}, nil)
	mockEntity.On("GetGroupCountForEntity", mock.Anything, mock.Anything).
		Maybe().Return(0, nil)

	mockInbound.On("GetInboundClientByEntityID", mock.Anything, mock.Anything).
		Maybe().Return((*inboundmodel.InboundClient)(nil), inboundclient.ErrInboundClientNotFound)
	mockInbound.On("GetOAuthProfileByEntityID", mock.Anything, mock.Anything).
		Maybe().Return((*providers.OAuthProfile)(nil), inboundclient.ErrInboundClientNotFound)
	mockInbound.On("GetCertificate", mock.Anything, mock.Anything, mock.Anything).
		Maybe().Return((*inboundmodel.Certificate)(nil), (*inboundclient.CertOperationError)(nil))
	mockInbound.On("ResolveInboundAuthProfileHandles", mock.Anything, mock.Anything).
		Maybe().Return(nil)
	mockInbound.On("Validate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Maybe().Return(nil)
	mockInbound.On("CreateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Maybe().Return(nil)
	mockInbound.On("UpdateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Maybe().Return(nil)
	mockInbound.On("DeleteInboundClient", mock.Anything, mock.Anything).
		Maybe().Return(nil)

	mockOU.On("IsOrganizationUnitExists", mock.Anything, mock.Anything).
		Maybe().Return(true, (*tidcommon.ServiceError)(nil))
	mockOU.On("GetOrganizationUnitByPath", mock.Anything, mock.Anything).
		Maybe().Return(providers.OrganizationUnit{ID: testOUID}, (*tidcommon.ServiceError)(nil))
	mockOU.On("GetOrganizationUnitHandlesByIDs", mock.Anything, mock.Anything).
		Maybe().Return(map[string]string{}, (*tidcommon.ServiceError)(nil))

	svc := &agentService{
		logger:               log.GetLogger().With(log.String(log.LoggerKeyComponentName, "AgentService")),
		entityService:        mockEntity,
		inboundClientService: mockInbound,
		ouService:            mockOU,
		dependencyRegistry:   noopDepRegistry{},
		roleService:          mockRole,
	}
	return svc, mockEntity, mockInbound, mockOU, mockRole
}

// noopDepRegistry is a no-op resourcedependency.Registry for tests that don't exercise cascade.
type noopDepRegistry struct{ cascadeErr error }

func (noopDepRegistry) RegisterProvider(resourcedependency.Provider) {}

func (noopDepRegistry) GetDependencies(
	context.Context, string, string) (*resourcedependency.DependenciesResponse, error) {
	return &resourcedependency.DependenciesResponse{}, nil
}

func (r noopDepRegistry) CascadeDelete(context.Context, string, string) (int, error) {
	return 0, r.cascadeErr
}

// buildAgentEntityFixture returns an providers.Entity with system attributes for the given fields.
func buildAgentEntityFixture(name, description, owner, clientID string) *providers.Entity {
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
	return &providers.Entity{
		ID:               testAgentID,
		Category:         providers.EntityCategoryAgent,
		Type:             testAgentType,
		State:            providers.EntityStateActive,
		OUID:             testOUID,
		SystemAttributes: sysAttrs,
	}
}

// --- pure helper tests ---

func (suite *AgentServiceTestSuite) TestNeedsInboundClient_NilRequest() {
	assert.False(suite.T(), needsInboundClient(nil))
}

func (suite *AgentServiceTestSuite) TestNeedsInboundClient_EmptyRequest() {
	assert.False(suite.T(), needsInboundClient(&model.Agent{}))
}

func (suite *AgentServiceTestSuite) TestNeedsInboundClient_WithAuthFlowID() {
	req := &model.Agent{InboundAuthProfile: providers.InboundAuthProfile{AuthFlowID: "flow-1"}}
	assert.True(suite.T(), needsInboundClient(req))
}

func (suite *AgentServiceTestSuite) TestNeedsInboundClient_WithInboundAuthConfig() {
	req := &model.Agent{
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{Type: providers.OAuthInboundAuthType, OAuthConfig: &providers.OAuthConfigWithSecret{}},
		},
	}
	assert.True(suite.T(), needsInboundClient(req))
}

func (suite *AgentServiceTestSuite) TestNeedsInboundClient_WithAllowedUserTypes() {
	req := &model.Agent{
		InboundAuthProfile: providers.InboundAuthProfile{AllowedUserTypes: []string{"employee"}},
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
	req := &model.UpdateAgentRequest{InboundAuthProfile: providers.InboundAuthProfile{ThemeID: "theme-abc"}}
	assert.True(suite.T(), updateNeedsInboundClient(req))
}

func (suite *AgentServiceTestSuite) TestRequiresClientSecret_NilConfig() {
	assert.False(suite.T(), requiresClientSecret(nil))
}

func (suite *AgentServiceTestSuite) TestRequiresClientSecret_PublicClient() {
	cfg := &providers.OAuthConfigWithSecret{PublicClient: true}
	assert.False(suite.T(), requiresClientSecret(cfg))
}

func (suite *AgentServiceTestSuite) TestRequiresClientSecret_ClientSecretBasic() {
	cfg := &providers.OAuthConfigWithSecret{
		TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
	}
	assert.True(suite.T(), requiresClientSecret(cfg))
}

func (suite *AgentServiceTestSuite) TestRequiresClientSecret_NoneMethod() {
	cfg := &providers.OAuthConfigWithSecret{
		TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodNone,
	}
	assert.False(suite.T(), requiresClientSecret(cfg))
}

func (suite *AgentServiceTestSuite) TestRequiresClientSecret_DefaultIsTrue() {
	cfg := &providers.OAuthConfigWithSecret{}
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
	svc, _, _, _, _ := suite.setupService()
	resp, svcErr := svc.CreateAgent(context.Background(), nil)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorInvalidRequestFormat.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestCreateAgent_MissingName() {
	svc, _, _, _, _ := suite.setupService()
	req := &model.Agent{Type: testAgentType, OUID: testOUID}
	resp, svcErr := svc.CreateAgent(context.Background(), req)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorInvalidAgentName.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestCreateAgent_MissingType() {
	svc, _, _, _, _ := suite.setupService()
	req := &model.Agent{Name: testAgentName, OUID: testOUID}
	resp, svcErr := svc.CreateAgent(context.Background(), req)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorInvalidAgentType.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestCreateAgent_OUNotFound() {
	svc, _, _, mockOU, _ := suite.setupService()
	clearMockCalls(mockOU, "IsOrganizationUnitExists")
	mockOU.On("IsOrganizationUnitExists", mock.Anything, testOUID).Return(false, (*tidcommon.ServiceError)(nil))

	req := &model.Agent{Name: testAgentName, Type: testAgentType, OUID: testOUID}
	resp, svcErr := svc.CreateAgent(context.Background(), req)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorOrganizationUnitNotFound.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestCreateAgent_NameAlreadyExists() {
	svc, mockEntity, _, _, _ := suite.setupService()
	existingID := "existing-agent-id"
	clearMockCalls(mockEntity, "IdentifyEntity")
	mockEntity.On("IdentifyEntity", mock.Anything, mock.Anything).Return(&existingID, nil)
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, existingID).Return(
		&providers.Entity{ID: existingID, Category: providers.EntityCategoryAgent}, nil)

	req := &model.Agent{Name: testAgentName, Type: testAgentType, OUID: testOUID}
	resp, svcErr := svc.CreateAgent(context.Background(), req)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorAgentAlreadyExistsWithName.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestCreateAgent_EntityOnly_Success() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

	createdEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "CreateEntity")
	mockEntity.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything).
		Return(createdEntity, nil)

	req := &model.Agent{
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

func (suite *AgentServiceTestSuite) TestCreateAgent_GeneratesUUIDWhenNoID() {
	svc, mockEntity, _, _, _ := suite.setupService()

	createdEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "CreateEntity")
	mockEntity.On("CreateEntity", mock.Anything, mock.MatchedBy(func(e *providers.Entity) bool {
		return e.ID != ""
	}), mock.Anything).Return(createdEntity, nil)

	req := &model.Agent{Name: testAgentName, Type: testAgentType, OUID: testOUID}
	resp, svcErr := svc.CreateAgent(context.Background(), req)
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.NotEmpty(suite.T(), resp.ID)
	mockEntity.AssertExpectations(suite.T())
}

func (suite *AgentServiceTestSuite) TestCreateAgent_PresetIDSkipsGeneration() {
	const presetID = "preset-agent-id-abc123"

	svc, mockEntity, _, _, _ := suite.setupService()

	createdEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	createdEntity.ID = presetID
	clearMockCalls(mockEntity, "CreateEntity")
	mockEntity.On("CreateEntity", mock.Anything, mock.MatchedBy(func(e *providers.Entity) bool {
		return e.ID == presetID
	}), mock.Anything).Return(createdEntity, nil)

	req := &model.Agent{ID: presetID, Name: testAgentName, Type: testAgentType, OUID: testOUID}
	resp, svcErr := svc.CreateAgent(context.Background(), req)
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Equal(suite.T(), presetID, resp.ID)
	mockEntity.AssertExpectations(suite.T())
}

func (suite *AgentServiceTestSuite) TestCreateAgent_WithInboundAuth_Success() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

	createdEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "CreateEntity")
	mockEntity.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything).
		Return(createdEntity, nil)

	clearMockCalls(mockInbound, "CreateInboundClient")
	mockInbound.On("CreateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	req := &model.Agent{
		Name:               testAgentName,
		Type:               testAgentType,
		OUID:               testOUID,
		InboundAuthProfile: providers.InboundAuthProfile{AuthFlowID: "flow-1"},
	}
	resp, svcErr := svc.CreateAgent(context.Background(), req)
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Equal(suite.T(), "flow-1", resp.AuthFlowID)
	mockInbound.AssertCalled(suite.T(), "CreateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (suite *AgentServiceTestSuite) TestCreateAgent_FlowIDResolvedToDefault() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

	createdEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "CreateEntity")
	mockEntity.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything).
		Return(createdEntity, nil)

	clearMockCalls(mockInbound, "CreateInboundClient")
	mockInbound.On("CreateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			client := args.Get(1).(*inboundmodel.InboundClient)
			client.AuthFlowID = "default-flow-id"
			client.RegistrationFlowID = "default-reg-flow-id"
		}).Return(nil)

	req := &model.Agent{
		Name: testAgentName,
		Type: testAgentType,
		OUID: testOUID,
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					GrantTypes:              []providers.GrantType{providers.GrantTypeClientCredentials},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
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
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

	createdEntity := buildAgentEntityFixture(testAgentName, "", "", "cid-xxx")
	clearMockCalls(mockEntity, "CreateEntity")
	mockEntity.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything).
		Return(createdEntity, nil)

	clearMockCalls(mockInbound, "CreateInboundClient")
	mockInbound.On("CreateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	req := &model.Agent{
		Name:               testAgentName,
		Type:               testAgentType,
		OUID:               testOUID,
		InboundAuthProfile: providers.InboundAuthProfile{AuthFlowID: "flow-1"},
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					GrantTypes:              []providers.GrantType{providers.GrantTypeClientCredentials},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
				},
			},
		},
	}
	resp, svcErr := svc.CreateAgent(context.Background(), req)
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	suite.Require().Len(resp.InboundAuthConfig, 1)
	assert.Equal(suite.T(), providers.OAuthInboundAuthType, resp.InboundAuthConfig[0].Type)
	assert.NotEmpty(suite.T(), resp.InboundAuthConfig[0].OAuthConfig.ClientID)
	assert.NotEmpty(suite.T(), resp.InboundAuthConfig[0].OAuthConfig.ClientSecret)
}

func (suite *AgentServiceTestSuite) TestCreateAgent_EntityCreationFails() {
	svc, mockEntity, _, _, _ := suite.setupService()

	clearMockCalls(mockEntity, "CreateEntity")
	mockEntity.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything).
		Return((*providers.Entity)(nil), entity.ErrSchemaValidationFailed)

	req := &model.Agent{Name: testAgentName, Type: testAgentType, OUID: testOUID}
	resp, svcErr := svc.CreateAgent(context.Background(), req)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorSchemaValidationFailed.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestCreateAgent_InboundCreationFails_CompensatesEntity() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

	createdEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "CreateEntity")
	mockEntity.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything).
		Return(createdEntity, nil)

	clearMockCalls(mockInbound, "CreateInboundClient")
	mockInbound.On("CreateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(inboundclient.ErrOAuthInvalidGrantType)

	clearMockCalls(mockEntity, "DeleteEntity")
	mockEntity.On("DeleteEntity", mock.Anything, mock.Anything).Return(nil)

	req := &model.Agent{
		Name:               testAgentName,
		Type:               testAgentType,
		OUID:               testOUID,
		InboundAuthProfile: providers.InboundAuthProfile{AuthFlowID: "flow-1"},
	}
	resp, svcErr := svc.CreateAgent(context.Background(), req)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorInvalidGrantType.Code, svcErr.Code)
	mockEntity.AssertCalled(suite.T(), "DeleteEntity", mock.Anything, mock.Anything)
}

// --- GetAgent ---

func (suite *AgentServiceTestSuite) TestGetAgent_EmptyID() {
	svc, _, _, _, _ := suite.setupService()
	resp, svcErr := svc.GetAgent(context.Background(), "", false)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorMissingAgentID.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestGetAgent_NotFound() {
	svc, _, _, _, _ := suite.setupService()
	// Default mock returns ErrEntityNotFound.
	resp, svcErr := svc.GetAgent(context.Background(), testAgentID, false)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorAgentNotFound.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestGetAgent_WrongCategory() {
	svc, mockEntity, _, _, _ := suite.setupService()

	wrongCatEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	wrongCatEntity.Category = providers.EntityCategoryUser

	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(wrongCatEntity, nil)

	resp, svcErr := svc.GetAgent(context.Background(), testAgentID, false)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorAgentNotFound.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestGetAgent_Success_NoInbound() {
	svc, mockEntity, _, _, _ := suite.setupService()

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
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "cid-123")

	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	inboundRec := &inboundmodel.InboundClient{ID: testAgentID, AuthFlowID: "flow-1"}
	clearMockCalls(mockInbound, "GetInboundClientByEntityID")
	mockInbound.On("GetInboundClientByEntityID", mock.Anything, testAgentID).Return(inboundRec, nil)

	oauthProfile := &providers.OAuthProfile{
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
	svc, _, _, _, _ := suite.setupService()
	svcErr := svc.DeleteAgent(context.Background(), "")
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorMissingAgentID.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestDeleteAgent_NotFound() {
	svc, _, _, _, _ := suite.setupService()
	svcErr := svc.DeleteAgent(context.Background(), testAgentID)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorAgentNotFound.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestDeleteAgent_Success_NoInboundClient() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

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
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

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
	svc, _, _, _, _ := suite.setupService()
	resp, svcErr := svc.GetAgentList(context.Background(), -1, 0, nil, false)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorInvalidLimit.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestGetAgentList_Success() {
	svc, mockEntity, _, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "desc", "alice", "")
	clearMockCalls(mockEntity, "GetEntityList")
	mockEntity.On("GetEntityList", mock.Anything, providers.EntityCategoryAgent, 30, 0, mock.Anything).
		Return([]providers.Entity{*agentEntity}, nil)
	clearMockCalls(mockEntity, "GetEntityListCount")
	mockEntity.On("GetEntityListCount", mock.Anything, providers.EntityCategoryAgent, mock.Anything).
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
	svc, _, _, _, _ := suite.setupService()
	resp, svcErr := svc.GetAgentGroups(context.Background(), "", 10, 0)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorMissingAgentID.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestGetAgentGroups_AgentNotFound() {
	svc, _, _, _, _ := suite.setupService()
	resp, svcErr := svc.GetAgentGroups(context.Background(), testAgentID, 10, 0)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorAgentNotFound.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestGetAgentGroups_Success() {
	svc, mockEntity, _, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockEntity, "GetGroupCountForEntity")
	mockEntity.On("GetGroupCountForEntity", mock.Anything, testAgentID).Return(2, nil)

	clearMockCalls(mockEntity, "GetEntityGroups")
	mockEntity.On("GetEntityGroups", mock.Anything, testAgentID, 10, 0).
		Return([]providers.EntityGroup{
			{ID: "g1", Name: "group-one", OUID: testOUID},
			{ID: "g2", Name: "group-two", OUID: testOUID},
		}, nil)

	resp, svcErr := svc.GetAgentGroups(context.Background(), testAgentID, 10, 0)
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Equal(suite.T(), 2, resp.TotalResults)
	assert.Len(suite.T(), resp.Groups, 2)
}

// --- GetAgentRoles ---

func (suite *AgentServiceTestSuite) TestGetAgentRoles_EmptyID() {
	svc, _, _, _, _ := suite.setupService()
	resp, svcErr := svc.GetAgentRoles(context.Background(), "", 10, 0)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorMissingAgentID.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestGetAgentRoles_AgentNotFound() {
	svc, _, _, _, _ := suite.setupService()
	resp, svcErr := svc.GetAgentRoles(context.Background(), testAgentID, 10, 0)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorAgentNotFound.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestGetAgentRoles_DirectAndGroupInherited() {
	svc, mockEntity, _, _, mockRole := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockEntity, "GetGroupCountForEntity")
	mockEntity.On("GetGroupCountForEntity", mock.Anything, testAgentID).Return(1, nil)

	clearMockCalls(mockEntity, "GetEntityGroups")
	mockEntity.On("GetEntityGroups", mock.Anything, testAgentID, 1, 0).
		Return([]providers.EntityGroup{{ID: "g1", Name: "group-one", OUID: testOUID}}, nil)

	mockRole.On("GetUserRoles", mock.Anything, testAgentID, []string{"g1"}).
		Return([]string{"order-service-reader", "platform-agent"}, nil)

	resp, svcErr := svc.GetAgentRoles(context.Background(), testAgentID, 10, 0)
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Equal(suite.T(), 2, resp.TotalResults)
	assert.Equal(suite.T(), []string{"order-service-reader", "platform-agent"}, resp.Roles)
}

func (suite *AgentServiceTestSuite) TestGetAgentRoles_NoGroups() {
	svc, mockEntity, _, _, mockRole := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockEntity, "GetGroupCountForEntity")
	mockEntity.On("GetGroupCountForEntity", mock.Anything, testAgentID).Return(0, nil)

	mockRole.On("GetUserRoles", mock.Anything, testAgentID, []string{}).
		Return([]string{"order-service-reader"}, nil)

	resp, svcErr := svc.GetAgentRoles(context.Background(), testAgentID, 10, 0)
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Equal(suite.T(), 1, resp.TotalResults)
	assert.Equal(suite.T(), []string{"order-service-reader"}, resp.Roles)
}

func (suite *AgentServiceTestSuite) TestGetAgentRoles_Pagination() {
	svc, mockEntity, _, _, mockRole := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockEntity, "GetGroupCountForEntity")
	mockEntity.On("GetGroupCountForEntity", mock.Anything, testAgentID).Return(0, nil)

	mockRole.On("GetUserRoles", mock.Anything, testAgentID, []string{}).
		Return([]string{"role-a", "role-b", "role-c"}, nil)

	resp, svcErr := svc.GetAgentRoles(context.Background(), testAgentID, 1, 1)
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Equal(suite.T(), 3, resp.TotalResults)
	assert.Equal(suite.T(), []string{"role-b"}, resp.Roles)
}

func (suite *AgentServiceTestSuite) TestGetAgentRoles_InvalidPagination() {
	svc, _, _, _, _ := suite.setupService()
	resp, svcErr := svc.GetAgentRoles(context.Background(), testAgentID, -1, 0)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorInvalidLimit.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestGetAgentRoles_DefaultLimit() {
	svc, mockEntity, _, _, mockRole := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockEntity, "GetGroupCountForEntity")
	mockEntity.On("GetGroupCountForEntity", mock.Anything, testAgentID).Return(0, nil)

	mockRole.On("GetUserRoles", mock.Anything, testAgentID, []string{}).
		Return([]string{}, nil)

	resp, svcErr := svc.GetAgentRoles(context.Background(), testAgentID, 0, 0)
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
}

func (suite *AgentServiceTestSuite) TestGetAgentRoles_EntityStoreError() {
	svc, mockEntity, _, _, _ := suite.setupService()
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).
		Return((*providers.Entity)(nil), errors.New("db error"))

	resp, svcErr := svc.GetAgentRoles(context.Background(), testAgentID, 10, 0)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestGetAgentRoles_WrongCategory() {
	svc, mockEntity, _, _, _ := suite.setupService()

	wrongCatEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	wrongCatEntity.Category = providers.EntityCategoryUser
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(wrongCatEntity, nil)

	resp, svcErr := svc.GetAgentRoles(context.Background(), testAgentID, 10, 0)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorAgentNotFound.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestGetAgentRoles_CountError() {
	svc, mockEntity, _, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockEntity, "GetGroupCountForEntity")
	mockEntity.On("GetGroupCountForEntity", mock.Anything, testAgentID).
		Return(0, errors.New("db error"))

	resp, svcErr := svc.GetAgentRoles(context.Background(), testAgentID, 10, 0)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestGetAgentRoles_GroupListError() {
	svc, mockEntity, _, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockEntity, "GetGroupCountForEntity")
	mockEntity.On("GetGroupCountForEntity", mock.Anything, testAgentID).Return(1, nil)

	clearMockCalls(mockEntity, "GetEntityGroups")
	mockEntity.On("GetEntityGroups", mock.Anything, testAgentID, 1, 0).
		Return(nil, errors.New("db error"))

	resp, svcErr := svc.GetAgentRoles(context.Background(), testAgentID, 10, 0)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestGetAgentRoles_RoleServiceError() {
	svc, mockEntity, _, _, mockRole := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockEntity, "GetGroupCountForEntity")
	mockEntity.On("GetGroupCountForEntity", mock.Anything, testAgentID).Return(0, nil)

	mockRole.On("GetUserRoles", mock.Anything, testAgentID, []string{}).
		Return(nil, &tidcommon.InternalServerError)

	resp, svcErr := svc.GetAgentRoles(context.Background(), testAgentID, 10, 0)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestGetAgentRoles_OffsetBeyondTotal() {
	svc, mockEntity, _, _, mockRole := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockEntity, "GetGroupCountForEntity")
	mockEntity.On("GetGroupCountForEntity", mock.Anything, testAgentID).Return(0, nil)

	mockRole.On("GetUserRoles", mock.Anything, testAgentID, []string{}).
		Return([]string{"role-a"}, nil)

	resp, svcErr := svc.GetAgentRoles(context.Background(), testAgentID, 10, 50)
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Equal(suite.T(), 1, resp.TotalResults)
	assert.Empty(suite.T(), resp.Roles)
}

// --- UpdateAgent ---

func (suite *AgentServiceTestSuite) TestUpdateAgent_EmptyID() {
	svc, _, _, _, _ := suite.setupService()
	resp, svcErr := svc.UpdateAgent(context.Background(), "", &model.UpdateAgentRequest{
		Name: testAgentName, Type: testAgentType,
	})
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorMissingAgentID.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_NilRequest() {
	svc, _, _, _, _ := suite.setupService()
	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, nil)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorInvalidRequestFormat.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_AgentNotFound() {
	svc, _, _, _, _ := suite.setupService()
	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Name: testAgentName, Type: testAgentType,
	})
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorAgentNotFound.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_Success_EntityOnly() {
	svc, mockEntity, _, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture("old-name", "", "", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockEntity, "UpdateEntity")
	mockEntity.On("UpdateEntity", mock.Anything, testAgentID, mock.Anything).
		Return(&providers.Entity{}, nil)

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Name: testAgentName, Type: testAgentType,
	})
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Equal(suite.T(), testAgentName, resp.Name)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_FlowIDResolvedToDefault() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture("old-name", "", "", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockEntity, "UpdateEntity")
	mockEntity.On("UpdateEntity", mock.Anything, testAgentID, mock.Anything).
		Return(&providers.Entity{}, nil)

	clearMockCalls(mockInbound, "GetInboundClientByEntityID")
	mockInbound.On("GetInboundClientByEntityID", mock.Anything, mock.Anything).
		Return(&inboundmodel.InboundClient{}, nil)

	clearMockCalls(mockInbound, "UpdateInboundClient")
	mockInbound.On("UpdateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			client := args.Get(1).(*inboundmodel.InboundClient)
			client.AuthFlowID = "default-flow-id"
			client.RegistrationFlowID = "default-reg-flow-id"
		}).Return(nil)

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Name: testAgentName,
		Type: testAgentType,
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					GrantTypes:              []providers.GrantType{providers.GrantTypeClientCredentials},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
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
	svc, _, _, _, _ := suite.setupService()
	assert.Nil(suite.T(), svc.validateOwnerExists(context.Background(), ""))
}

func (suite *AgentServiceTestSuite) TestValidateOwnerExists_NotFound() {
	svc, mockEntity, _, _, _ := suite.setupService()
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, "missing-owner").
		Return((*providers.Entity)(nil), entity.ErrEntityNotFound)

	svcErr := svc.validateOwnerExists(context.Background(), "missing-owner")
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorOwnerNotFound.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestValidateOwnerExists_StoreError() {
	svc, mockEntity, _, _, _ := suite.setupService()
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, "owner-x").
		Return((*providers.Entity)(nil), errors.New("db error"))

	svcErr := svc.validateOwnerExists(context.Background(), "owner-x")
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestValidateOwnerExists_Success() {
	svc, mockEntity, _, _, _ := suite.setupService()
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, "owner-y").
		Return(&providers.Entity{ID: "owner-y"}, nil)

	assert.Nil(suite.T(), svc.validateOwnerExists(context.Background(), "owner-y"))
}

func (suite *AgentServiceTestSuite) TestCreateAgent_OwnerNotFound() {
	svc, mockEntity, _, _, _ := suite.setupService()
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, "ghost").
		Return((*providers.Entity)(nil), entity.ErrEntityNotFound)

	resp, svcErr := svc.CreateAgent(context.Background(), &model.Agent{
		Name: testAgentName, Type: testAgentType, OUID: testOUID, Owner: "ghost",
	})
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorOwnerNotFound.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_OwnerChanged_OwnerNotFound() {
	svc, mockEntity, _, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture("old-name", "", "current-owner", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)
	mockEntity.On("GetEntity", mock.Anything, "new-owner").
		Return((*providers.Entity)(nil), entity.ErrEntityNotFound)

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Name: testAgentName, Type: testAgentType, Owner: "new-owner",
	})
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorOwnerNotFound.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_OwnerChanged_Success() {
	svc, mockEntity, _, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture("old-name", "", "current-owner", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)
	mockEntity.On("GetEntity", mock.Anything, "new-owner").
		Return(&providers.Entity{ID: "new-owner"}, nil)

	clearMockCalls(mockEntity, "UpdateEntity")
	mockEntity.On("UpdateEntity", mock.Anything, testAgentID, mock.Anything).
		Return(&providers.Entity{}, nil)

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
	svc, _, _, _, _ := suite.setupService()
	svcErr := svc.translateInboundClientError(context.Background(), inboundclient.ErrOAuthInvalidRedirectURI)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorInvalidRedirectURI.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestTranslateInboundClientError_InvalidGrantType() {
	svc, _, _, _, _ := suite.setupService()
	svcErr := svc.translateInboundClientError(context.Background(), inboundclient.ErrOAuthInvalidGrantType)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorInvalidGrantType.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestTranslateInboundClientError_Unknown() {
	svc, _, _, _, _ := suite.setupService()
	svcErr := svc.translateInboundClientError(context.Background(), errors.New("unknown error"))
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
		{"CertificateRequiresClientID", inboundclient.ErrOAuthCertificateRequiresClientID,
			ErrorInvalidOAuthConfiguration.Code,
			"error.agentservice.certificate_requires_client_id_description"},
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
		{"FlowServerError", inboundclient.ErrFKFlowServerError, tidcommon.InternalServerError.Code},
		{"ThemeNotFound", inboundclient.ErrFKThemeNotFound, ErrorThemeNotFound.Code},
		{"LayoutNotFound", inboundclient.ErrFKLayoutNotFound, ErrorLayoutNotFound.Code},
		{"InvalidUserType", inboundclient.ErrFKInvalidUserType, ErrorInvalidUserType.Code},
		{"UserSchemaLookupFailed", inboundclient.ErrUserSchemaLookupFailed, tidcommon.InternalServerError.Code},
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
		underlying  *tidcommon.ServiceError
		wantCode    string
		wantDescKey string
	}{
		{"CreateClientErr", inboundclient.CertOpCreate, cert.CertificateReferenceTypeOAuthApp,
			&tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "X-1",
				ErrorDescription: tidcommon.I18nMessage{DefaultValue: "underlying"}},
			ErrorCertificateClientError.Code, "error.agentservice.create_certificate_failed_description"},
		{"UpdateClientErr", inboundclient.CertOpUpdate, cert.CertificateReferenceTypeOAuthApp,
			&tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "X-2",
				ErrorDescription: tidcommon.I18nMessage{DefaultValue: "underlying"}},
			ErrorCertificateClientError.Code, "error.agentservice.update_certificate_failed_description"},
		{"RetrieveClientErr", inboundclient.CertOpRetrieve, cert.CertificateReferenceTypeOAuthApp,
			&tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "X-3",
				ErrorDescription: tidcommon.I18nMessage{DefaultValue: "underlying"}},
			ErrorCertificateClientError.Code, "error.agentservice.retrieve_certificate_failed_description"},
		{"DeleteOAuthRefClientErr", inboundclient.CertOpDelete, cert.CertificateReferenceTypeOAuthApp,
			&tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "X-5",
				ErrorDescription: tidcommon.I18nMessage{DefaultValue: "underlying"}},
			ErrorCertificateClientError.Code, "error.agentservice.delete_oauth_certificate_failed_description"},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			opErr := &inboundclient.CertOperationError{
				Operation: tc.op, RefType: tc.refType, Underlying: tc.underlying,
			}
			svcErr := s.translateCertOperationError(context.Background(), opErr)
			suite.Require().NotNil(svcErr)
			suite.Equal(tc.wantCode, svcErr.Code)
			suite.Equal(tc.wantDescKey, svcErr.ErrorDescription.Key)
			suite.Contains(svcErr.ErrorDescription.DefaultValue, "underlying")
		})
	}

	// Server error returns InternalServerError.
	serverErrOp := &inboundclient.CertOperationError{
		Operation:  inboundclient.CertOpCreate,
		RefType:    cert.CertificateReferenceTypeOAuthApp,
		Underlying: &tidcommon.ServiceError{Type: tidcommon.ServerErrorType, Code: "X-S"},
	}
	suite.Equal(
		tidcommon.InternalServerError.Code,
		s.translateCertOperationError(context.Background(), serverErrOp).Code)

	// Unknown operation returns InternalServerError.
	unknownOp := &inboundclient.CertOperationError{
		Operation:  "weird",
		Underlying: &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "X-?"},
	}
	suite.Equal(
		tidcommon.InternalServerError.Code,
		s.translateCertOperationError(context.Background(), unknownOp).Code)
}

// --- reconcileInboundForUpdate not-found short-circuit ---

// When the update request has no inbound config and the existing inbound client delete returns
// ErrInboundClientNotFound, the update should still succeed (no-op).
func (suite *AgentServiceTestSuite) TestUpdateAgent_NoInboundWanted_DeleteNotFound_Succeeds() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture("old-name", "", "", "cid-abc")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockEntity, "UpdateEntity")
	mockEntity.On("UpdateEntity", mock.Anything, testAgentID, mock.Anything).
		Return(&providers.Entity{}, nil)

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

// --- GetAgent additional error paths ---

func (suite *AgentServiceTestSuite) TestGetAgent_EntityStoreError() {
	svc, mockEntity, _, _, _ := suite.setupService()
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).
		Return((*providers.Entity)(nil), errors.New("db error"))

	resp, svcErr := svc.GetAgent(context.Background(), testAgentID, false)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestGetAgent_InboundClientError() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockInbound, "GetInboundClientByEntityID")
	mockInbound.On("GetInboundClientByEntityID", mock.Anything, testAgentID).
		Return((*inboundmodel.InboundClient)(nil), errors.New("db error"))

	resp, svcErr := svc.GetAgent(context.Background(), testAgentID, false)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestGetAgent_OAuthProfileError() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockInbound, "GetInboundClientByEntityID")
	mockInbound.On("GetInboundClientByEntityID", mock.Anything, testAgentID).
		Return(&inboundmodel.InboundClient{ID: testAgentID}, nil)

	clearMockCalls(mockInbound, "GetOAuthProfileByEntityID")
	mockInbound.On("GetOAuthProfileByEntityID", mock.Anything, testAgentID).
		Return((*providers.OAuthProfile)(nil), errors.New("db error"))

	resp, svcErr := svc.GetAgent(context.Background(), testAgentID, false)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestGetAgent_OAuthCertError() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "cid-123")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockInbound, "GetInboundClientByEntityID")
	mockInbound.On("GetInboundClientByEntityID", mock.Anything, testAgentID).
		Return(&inboundmodel.InboundClient{ID: testAgentID}, nil)

	clearMockCalls(mockInbound, "GetOAuthProfileByEntityID")
	mockInbound.On("GetOAuthProfileByEntityID", mock.Anything, testAgentID).
		Return(&providers.OAuthProfile{GrantTypes: []string{"client_credentials"}}, nil)

	certOpErr := &inboundclient.CertOperationError{
		Operation:  inboundclient.CertOpRetrieve,
		RefType:    cert.CertificateReferenceTypeOAuthApp,
		Underlying: &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "CERT-2"},
	}
	clearMockCalls(mockInbound, "GetCertificate")
	mockInbound.On("GetCertificate", mock.Anything, cert.CertificateReferenceTypeOAuthApp, "cid-123").
		Return((*inboundmodel.Certificate)(nil), certOpErr)

	resp, svcErr := svc.GetAgent(context.Background(), testAgentID, false)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorCertificateClientError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestGetAgent_IncludeDisplay_PopulatesOUHandle() {
	svc, mockEntity, _, mockOU, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockOU, "GetOrganizationUnitHandlesByIDs")
	mockOU.On("GetOrganizationUnitHandlesByIDs", mock.Anything, []string{testOUID}).
		Return(map[string]string{testOUID: "test-ou"}, (*tidcommon.ServiceError)(nil))

	resp, svcErr := svc.GetAgent(context.Background(), testAgentID, true)
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Equal(suite.T(), "test-ou", resp.OUHandle)
}

func (suite *AgentServiceTestSuite) TestGetAgent_IncludeDisplay_SkipsWhenOUIDEmpty() {
	svc, mockEntity, _, mockOU, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	agentEntity.OUID = ""
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	resp, svcErr := svc.GetAgent(context.Background(), testAgentID, true)
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Empty(suite.T(), resp.OUHandle)
	mockOU.AssertNotCalled(suite.T(), "GetOrganizationUnitHandlesByIDs")
}

func (suite *AgentServiceTestSuite) TestGetAgent_IncludeDisplay_SkipsWhenHandleAlreadySet() {
	svc, mockEntity, _, mockOU, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	agentEntity.OUHandle = "pre-set-handle"
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	resp, svcErr := svc.GetAgent(context.Background(), testAgentID, true)
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Equal(suite.T(), "pre-set-handle", resp.OUHandle)
	mockOU.AssertNotCalled(suite.T(), "GetOrganizationUnitHandlesByIDs")
}

func (suite *AgentServiceTestSuite) TestGetAgent_IncludeDisplay_LookupError() {
	svc, mockEntity, _, mockOU, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	ouErr := &tidcommon.ServiceError{Code: "OU_ERR"}
	clearMockCalls(mockOU, "GetOrganizationUnitHandlesByIDs")
	mockOU.On("GetOrganizationUnitHandlesByIDs", mock.Anything, mock.Anything).
		Return(map[string]string(nil), ouErr)

	resp, svcErr := svc.GetAgent(context.Background(), testAgentID, true)
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Empty(suite.T(), resp.OUHandle)
}

// --- GetAgentList additional paths ---

func (suite *AgentServiceTestSuite) TestGetAgentList_CountError() {
	svc, mockEntity, _, _, _ := suite.setupService()
	clearMockCalls(mockEntity, "GetEntityListCount")
	mockEntity.On("GetEntityListCount", mock.Anything, providers.EntityCategoryAgent, mock.Anything).
		Return(0, errors.New("db error"))

	resp, svcErr := svc.GetAgentList(context.Background(), 10, 0, nil, false)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestGetAgentList_ListError() {
	svc, mockEntity, _, _, _ := suite.setupService()
	clearMockCalls(mockEntity, "GetEntityList")
	mockEntity.On("GetEntityList", mock.Anything, providers.EntityCategoryAgent,
		mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("db error"))

	resp, svcErr := svc.GetAgentList(context.Background(), 10, 0, nil, false)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestGetAgentList_DefaultLimit() {
	svc, mockEntity, _, _, _ := suite.setupService()
	clearMockCalls(mockEntity, "GetEntityList")
	mockEntity.On("GetEntityList", mock.Anything, providers.EntityCategoryAgent, 30, 0, mock.Anything).
		Return([]providers.Entity{}, nil)

	resp, svcErr := svc.GetAgentList(context.Background(), 0, 0, nil, false)
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Equal(suite.T(), 0, resp.TotalResults)
}

func (suite *AgentServiceTestSuite) TestGetAgentList_IncludeDisplay() {
	svc, mockEntity, _, mockOU, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "GetEntityList")
	mockEntity.On("GetEntityList", mock.Anything, providers.EntityCategoryAgent, 10, 0, mock.Anything).
		Return([]providers.Entity{*agentEntity}, nil)
	clearMockCalls(mockEntity, "GetEntityListCount")
	mockEntity.On("GetEntityListCount", mock.Anything, providers.EntityCategoryAgent, mock.Anything).
		Return(1, nil)

	clearMockCalls(mockOU, "GetOrganizationUnitHandlesByIDs")
	mockOU.On("GetOrganizationUnitHandlesByIDs", mock.Anything, []string{testOUID}).
		Return(map[string]string{testOUID: "test-ou"}, (*tidcommon.ServiceError)(nil))

	resp, svcErr := svc.GetAgentList(context.Background(), 10, 0, nil, true)
	suite.Require().Nil(svcErr)
	suite.Require().Len(resp.Agents, 1)
	assert.Equal(suite.T(), "test-ou", resp.Agents[0].OUHandle)
}

// --- UpdateAgent additional paths ---

func (suite *AgentServiceTestSuite) TestUpdateAgent_MissingName() {
	svc, _, _, _, _ := suite.setupService()
	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Type: testAgentType,
	})
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorInvalidAgentName.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_EntityStoreError() {
	svc, mockEntity, _, _, _ := suite.setupService()
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).
		Return((*providers.Entity)(nil), errors.New("db error"))

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Name: testAgentName, Type: testAgentType,
	})
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_WrongCategory() {
	svc, mockEntity, _, _, _ := suite.setupService()

	wrongCatEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	wrongCatEntity.Category = providers.EntityCategoryUser
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(wrongCatEntity, nil)

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Name: testAgentName, Type: testAgentType,
	})
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorAgentNotFound.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_IsReadOnly() {
	svc, mockEntity, _, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	agentEntity.IsReadOnly = true
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Name: testAgentName, Type: testAgentType,
	})
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorCannotModifyDeclarativeResource.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_OUHandleResolution() {
	svc, mockEntity, _, mockOU, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture("old-name", "", "", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockEntity, "UpdateEntity")
	mockEntity.On("UpdateEntity", mock.Anything, testAgentID, mock.Anything).
		Return(&providers.Entity{}, nil)

	newOUID := "new-ou-id"
	clearMockCalls(mockOU, "GetOrganizationUnitByPath")
	mockOU.On("GetOrganizationUnitByPath", mock.Anything, "new-handle").
		Return(providers.OrganizationUnit{ID: newOUID}, (*tidcommon.ServiceError)(nil))

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Name: testAgentName, Type: testAgentType, OUHandle: "new-handle",
	})
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Equal(suite.T(), newOUID, resp.OUID)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_ExplicitOUIDChanged() {
	svc, mockEntity, _, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockEntity, "UpdateEntity")
	mockEntity.On("UpdateEntity", mock.Anything, testAgentID, mock.Anything).
		Return(&providers.Entity{}, nil)

	newOUID := "different-ou-id"
	// Default IsOrganizationUnitExists mock returns true for any ID.

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Name: testAgentName, Type: testAgentType, OUID: newOUID,
	})
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Equal(suite.T(), newOUID, resp.OUID)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_WantsInbound_NoExisting_CreatesInbound() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture("old-name", "", "", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockEntity, "UpdateEntity")
	mockEntity.On("UpdateEntity", mock.Anything, testAgentID, mock.Anything).
		Return(&providers.Entity{}, nil)

	clearMockCalls(mockInbound, "CreateInboundClient")
	mockInbound.On("CreateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			client := args.Get(1).(*inboundmodel.InboundClient)
			client.AuthFlowID = "new-flow-id"
		}).Return(nil)

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Name:               testAgentName,
		Type:               testAgentType,
		InboundAuthProfile: providers.InboundAuthProfile{AuthFlowID: "new-flow-id"},
	})
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Equal(suite.T(), "new-flow-id", resp.AuthFlowID)
	mockInbound.AssertCalled(suite.T(), "CreateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_PopulatesOUHandle_SkipsWhenOUIDEmpty() {
	svc, mockEntity, _, mockOU, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	agentEntity.OUID = ""
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockEntity, "UpdateEntity")
	mockEntity.On("UpdateEntity", mock.Anything, testAgentID, mock.Anything).
		Return(&providers.Entity{}, nil)

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Name: testAgentName, Type: testAgentType,
	})
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Empty(suite.T(), resp.OUID)
	mockOU.AssertNotCalled(suite.T(), "GetOrganizationUnitHandlesByIDs")
}

// --- DeleteAgent additional paths ---

func (suite *AgentServiceTestSuite) TestDeleteAgent_EntityStoreError() {
	svc, mockEntity, _, _, _ := suite.setupService()
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).
		Return((*providers.Entity)(nil), errors.New("db error"))

	svcErr := svc.DeleteAgent(context.Background(), testAgentID)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestDeleteAgent_WrongCategory() {
	svc, mockEntity, _, _, _ := suite.setupService()

	wrongCatEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	wrongCatEntity.Category = providers.EntityCategoryUser
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(wrongCatEntity, nil)

	svcErr := svc.DeleteAgent(context.Background(), testAgentID)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorAgentNotFound.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestDeleteAgent_IsReadOnly() {
	svc, mockEntity, _, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	agentEntity.IsReadOnly = true
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	svcErr := svc.DeleteAgent(context.Background(), testAgentID)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorCannotModifyDeclarativeResource.Code, svcErr.Code)
}

// --- GetAgentGroups additional paths ---

func (suite *AgentServiceTestSuite) TestGetAgentGroups_InvalidPagination() {
	svc, _, _, _, _ := suite.setupService()
	resp, svcErr := svc.GetAgentGroups(context.Background(), testAgentID, -1, 0)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorInvalidLimit.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestGetAgentGroups_DefaultLimit() {
	svc, mockEntity, _, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockEntity, "GetGroupCountForEntity")
	mockEntity.On("GetGroupCountForEntity", mock.Anything, testAgentID).Return(0, nil)

	clearMockCalls(mockEntity, "GetEntityGroups")
	mockEntity.On("GetEntityGroups", mock.Anything, testAgentID, 30, 0).
		Return([]providers.EntityGroup{}, nil)

	resp, svcErr := svc.GetAgentGroups(context.Background(), testAgentID, 0, 0)
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
}

func (suite *AgentServiceTestSuite) TestGetAgentGroups_EntityStoreError() {
	svc, mockEntity, _, _, _ := suite.setupService()
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).
		Return((*providers.Entity)(nil), errors.New("db error"))

	resp, svcErr := svc.GetAgentGroups(context.Background(), testAgentID, 10, 0)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestGetAgentGroups_WrongCategory() {
	svc, mockEntity, _, _, _ := suite.setupService()

	wrongCatEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	wrongCatEntity.Category = providers.EntityCategoryUser
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(wrongCatEntity, nil)

	resp, svcErr := svc.GetAgentGroups(context.Background(), testAgentID, 10, 0)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorAgentNotFound.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestGetAgentGroups_CountError() {
	svc, mockEntity, _, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockEntity, "GetGroupCountForEntity")
	mockEntity.On("GetGroupCountForEntity", mock.Anything, testAgentID).
		Return(0, errors.New("db error"))

	resp, svcErr := svc.GetAgentGroups(context.Background(), testAgentID, 10, 0)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestGetAgentGroups_ListError() {
	svc, mockEntity, _, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockEntity, "GetGroupCountForEntity")
	mockEntity.On("GetGroupCountForEntity", mock.Anything, testAgentID).Return(2, nil)

	clearMockCalls(mockEntity, "GetEntityGroups")
	mockEntity.On("GetEntityGroups", mock.Anything, testAgentID, 10, 0).
		Return(nil, errors.New("db error"))

	resp, svcErr := svc.GetAgentGroups(context.Background(), testAgentID, 10, 0)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

// --- validateNameUnique direct tests ---

func (suite *AgentServiceTestSuite) TestValidateNameUnique_AmbiguousEntity() {
	svc, mockEntity, _, _, _ := suite.setupService()
	clearMockCalls(mockEntity, "IdentifyEntity")
	mockEntity.On("IdentifyEntity", mock.Anything, mock.Anything).
		Return((*string)(nil), entity.ErrAmbiguousEntity)

	svcErr := svc.validateNameUnique(context.Background(), testAgentName, "")
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorAgentAlreadyExistsWithName.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestValidateNameUnique_StoreError() {
	svc, mockEntity, _, _, _ := suite.setupService()
	clearMockCalls(mockEntity, "IdentifyEntity")
	mockEntity.On("IdentifyEntity", mock.Anything, mock.Anything).
		Return((*string)(nil), errors.New("db error"))

	svcErr := svc.validateNameUnique(context.Background(), testAgentName, "")
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestValidateNameUnique_NilID() {
	svc, mockEntity, _, _, _ := suite.setupService()
	clearMockCalls(mockEntity, "IdentifyEntity")
	mockEntity.On("IdentifyEntity", mock.Anything, mock.Anything).
		Return((*string)(nil), nil)

	svcErr := svc.validateNameUnique(context.Background(), testAgentName, "")
	assert.Nil(suite.T(), svcErr)
}

func (suite *AgentServiceTestSuite) TestValidateNameUnique_ExcludeIDMatch() {
	svc, mockEntity, _, _, _ := suite.setupService()

	foundID := testAgentID
	clearMockCalls(mockEntity, "IdentifyEntity")
	mockEntity.On("IdentifyEntity", mock.Anything, mock.Anything).Return(&foundID, nil)

	svcErr := svc.validateNameUnique(context.Background(), testAgentName, testAgentID)
	assert.Nil(suite.T(), svcErr)
}

func (suite *AgentServiceTestSuite) TestValidateNameUnique_NonAgentEntity() {
	svc, mockEntity, _, _, _ := suite.setupService()

	foundID := "some-app-id"
	clearMockCalls(mockEntity, "IdentifyEntity")
	mockEntity.On("IdentifyEntity", mock.Anything, mock.Anything).Return(&foundID, nil)

	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, foundID).
		Return(&providers.Entity{ID: foundID, Category: providers.EntityCategoryUser}, nil)

	svcErr := svc.validateNameUnique(context.Background(), testAgentName, "")
	assert.Nil(suite.T(), svcErr)
}

// --- isClientIDTaken direct tests ---

func (suite *AgentServiceTestSuite) TestIsClientIDTaken_StoreError() {
	svc, mockEntity, _, _, _ := suite.setupService()
	clearMockCalls(mockEntity, "IdentifyEntity")
	mockEntity.On("IdentifyEntity", mock.Anything, mock.Anything).
		Return((*string)(nil), errors.New("db error"))

	taken, svcErr := svc.isClientIDTaken(context.Background(), "client-x", "")
	assert.False(suite.T(), taken)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestIsClientIDTaken_NilID() {
	svc, mockEntity, _, _, _ := suite.setupService()
	clearMockCalls(mockEntity, "IdentifyEntity")
	mockEntity.On("IdentifyEntity", mock.Anything, mock.Anything).
		Return((*string)(nil), nil)

	taken, svcErr := svc.isClientIDTaken(context.Background(), "client-x", "")
	assert.False(suite.T(), taken)
	assert.Nil(suite.T(), svcErr)
}

func (suite *AgentServiceTestSuite) TestIsClientIDTaken_ExcludeIDMatch() {
	svc, mockEntity, _, _, _ := suite.setupService()

	foundID := "exclude-agent"
	clearMockCalls(mockEntity, "IdentifyEntity")
	mockEntity.On("IdentifyEntity", mock.Anything, mock.Anything).Return(&foundID, nil)

	taken, svcErr := svc.isClientIDTaken(context.Background(), "client-x", "exclude-agent")
	assert.False(suite.T(), taken)
	assert.Nil(suite.T(), svcErr)
}

func (suite *AgentServiceTestSuite) TestIsClientIDTaken_Taken() {
	svc, mockEntity, _, _, _ := suite.setupService()

	foundID := "other-agent"
	clearMockCalls(mockEntity, "IdentifyEntity")
	mockEntity.On("IdentifyEntity", mock.Anything, mock.Anything).Return(&foundID, nil)

	taken, svcErr := svc.isClientIDTaken(context.Background(), "client-x", "")
	assert.True(suite.T(), taken)
	assert.Nil(suite.T(), svcErr)
}

// --- populateOUHandlesForList direct tests ---

func (suite *AgentServiceTestSuite) TestPopulateOUHandlesForList_Empty() {
	svc, _, _, mockOU, _ := suite.setupService()
	agents := []model.BasicAgentResponse{}
	svc.populateOUHandlesForList(context.Background(), agents)
	mockOU.AssertNotCalled(suite.T(), "GetOrganizationUnitHandlesByIDs")
}

func (suite *AgentServiceTestSuite) TestPopulateOUHandlesForList_AllEmptyOUIDs() {
	svc, _, _, mockOU, _ := suite.setupService()
	agents := []model.BasicAgentResponse{
		{ID: "a1", OUID: ""},
		{ID: "a2", OUID: ""},
	}
	svc.populateOUHandlesForList(context.Background(), agents)
	mockOU.AssertNotCalled(suite.T(), "GetOrganizationUnitHandlesByIDs")
}

func (suite *AgentServiceTestSuite) TestPopulateOUHandlesForList_LookupError() {
	svc, _, _, mockOU, _ := suite.setupService()

	ouErr := &tidcommon.ServiceError{Code: "LOOKUP_ERR"}
	clearMockCalls(mockOU, "GetOrganizationUnitHandlesByIDs")
	mockOU.On("GetOrganizationUnitHandlesByIDs", mock.Anything, mock.Anything).
		Return(map[string]string(nil), ouErr)

	agents := []model.BasicAgentResponse{{ID: "a1", OUID: testOUID}}
	svc.populateOUHandlesForList(context.Background(), agents)
	assert.Empty(suite.T(), agents[0].OUHandle)
}

func (suite *AgentServiceTestSuite) TestPopulateOUHandlesForList_Success() {
	svc, _, _, mockOU, _ := suite.setupService()

	clearMockCalls(mockOU, "GetOrganizationUnitHandlesByIDs")
	mockOU.On("GetOrganizationUnitHandlesByIDs", mock.Anything, []string{testOUID}).
		Return(map[string]string{testOUID: "my-ou"}, (*tidcommon.ServiceError)(nil))

	agents := []model.BasicAgentResponse{{ID: "a1", OUID: testOUID}}
	svc.populateOUHandlesForList(context.Background(), agents)
	assert.Equal(suite.T(), "my-ou", agents[0].OUHandle)
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

// --- ValidateAgent ---

func (suite *AgentServiceTestSuite) TestValidateAgent_NilRequest() {
	svc, _, _, _, _ := suite.setupService()
	_, _, _, svcErr := svc.ValidateAgent(context.Background(), nil, "")
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorInvalidRequestFormat.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestValidateAgent_OUHandleNotFound() {
	svc, _, _, mockOU, _ := suite.setupService()

	notFound := &oupkg.ErrorOrganizationUnitNotFound
	clearMockCalls(mockOU, "GetOrganizationUnitByPath")
	mockOU.On("GetOrganizationUnitByPath", mock.Anything, "missing-handle").
		Return(providers.OrganizationUnit{}, notFound)

	req := &model.Agent{
		Name: testAgentName, Type: testAgentType, OUHandle: "missing-handle",
	}
	_, _, _, svcErr := svc.ValidateAgent(context.Background(), req, "")
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorOrganizationUnitNotFound.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestValidateAgent_OUHandleInternalError() {
	svc, _, _, mockOU, _ := suite.setupService()

	internalErr := &tidcommon.ServiceError{Code: "SOME_OTHER_ERROR"}
	clearMockCalls(mockOU, "GetOrganizationUnitByPath")
	mockOU.On("GetOrganizationUnitByPath", mock.Anything, "bad-handle").
		Return(providers.OrganizationUnit{}, internalErr)

	req := &model.Agent{
		Name: testAgentName, Type: testAgentType, OUHandle: "bad-handle",
	}
	_, _, _, svcErr := svc.ValidateAgent(context.Background(), req, "")
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

// --- resolveUpdateOUID via UpdateAgent ---

func (suite *AgentServiceTestSuite) TestUpdateAgent_OUHandleResolution_OUNotFound() {
	svc, mockEntity, _, mockOU, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	notFound := &oupkg.ErrorOrganizationUnitNotFound
	clearMockCalls(mockOU, "GetOrganizationUnitByPath")
	mockOU.On("GetOrganizationUnitByPath", mock.Anything, "missing-handle").
		Return(providers.OrganizationUnit{}, notFound)

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Name: testAgentName, Type: testAgentType, OUHandle: "missing-handle",
	})
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorOrganizationUnitNotFound.Code, svcErr.Code)
	assert.Nil(suite.T(), resp)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_OUHandleResolution_InternalError() {
	svc, mockEntity, _, mockOU, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	internalErr := &tidcommon.ServiceError{Code: "SOME_OTHER_ERROR"}
	clearMockCalls(mockOU, "GetOrganizationUnitByPath")
	mockOU.On("GetOrganizationUnitByPath", mock.Anything, "bad-handle").
		Return(providers.OrganizationUnit{}, internalErr)

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Name: testAgentName, Type: testAgentType, OUHandle: "bad-handle",
	})
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
	assert.Nil(suite.T(), resp)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_ExplicitOUIDChanged_ValidateOUFails() {
	svc, mockEntity, _, mockOU, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	ouNotFound := &oupkg.ErrorOrganizationUnitNotFound
	clearMockCalls(mockOU, "IsOrganizationUnitExists")
	mockOU.On("IsOrganizationUnitExists", mock.Anything, "nonexistent-ou").
		Return(false, ouNotFound)

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Name: testAgentName, Type: testAgentType, OUID: "nonexistent-ou",
	})
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorOrganizationUnitNotFound.Code, svcErr.Code)
	assert.Nil(suite.T(), resp)
}

// --- GetResourceDependencies tests ---

func (suite *AgentServiceTestSuite) TestGetResourceDependencies_UnknownResourceType() {
	svc, _, mockInbound, _, _ := suite.setupService()
	mockInbound.On("GetEntityIDsByReference", mock.Anything, "unknown", "id-1",
		serverconst.MaxCompositeStoreRecords, 0).Return([]string{}, 0, nil)

	result, err := svc.GetResourceDependencies(context.Background(), "unknown", "id-1")
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), result)
}

func (suite *AgentServiceTestSuite) TestGetResourceDependencies_InboundClientError() {
	svc, _, mockInbound, _, _ := suite.setupService()
	mockInbound.On("GetEntityIDsByReference", mock.Anything, resourcedependency.ResourceTypeTheme, "theme-1",
		serverconst.MaxCompositeStoreRecords, 0).
		Return(nil, 0, errors.New("store error"))

	result, err := svc.GetResourceDependencies(context.Background(), resourcedependency.ResourceTypeTheme, "theme-1")
	assert.Nil(suite.T(), result)
	assert.Error(suite.T(), err)
}

func (suite *AgentServiceTestSuite) TestGetResourceDependencies_EmptyIDs() {
	svc, _, mockInbound, _, _ := suite.setupService()
	mockInbound.On("GetEntityIDsByReference", mock.Anything, resourcedependency.ResourceTypeTheme, "theme-1",
		serverconst.MaxCompositeStoreRecords, 0).
		Return([]string{}, 0, nil)

	result, err := svc.GetResourceDependencies(context.Background(), resourcedependency.ResourceTypeTheme, "theme-1")
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), result)
}

func (suite *AgentServiceTestSuite) TestGetResourceDependencies_Success() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()
	mockInbound.On("GetEntityIDsByReference", mock.Anything, resourcedependency.ResourceTypeTheme, "theme-1",
		serverconst.MaxCompositeStoreRecords, 0).
		Return([]string{"agent-1"}, 1, nil)

	sysAttrs, _ := json.Marshal(map[string]interface{}{"name": "Agent One"})
	mockEntity.On("GetEntitiesByIDs", mock.Anything, []string{"agent-1"}).Return([]providers.Entity{
		{ID: "agent-1", Category: providers.EntityCategoryAgent, SystemAttributes: sysAttrs},
	}, nil)

	result, err := svc.GetResourceDependencies(context.Background(), resourcedependency.ResourceTypeTheme, "theme-1")
	assert.NoError(suite.T(), err)
	suite.Require().Len(result, 1)
	assert.Equal(suite.T(), resourcedependency.ResourceTypeAgent, result[0].ResourceType)
	assert.Equal(suite.T(), resourcedependency.BehaviorFallback, result[0].BehaviorOnDelete)
	assert.Equal(suite.T(), "agent-1", result[0].ID)
	assert.Equal(suite.T(), "Agent One", result[0].DisplayName)
}

// Applications share the inbound-client store; the agent provider must skip non-agent entities.
func (suite *AgentServiceTestSuite) TestGetResourceDependencies_FiltersOutNonAgentEntities() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()
	mockInbound.On("GetEntityIDsByReference", mock.Anything, resourcedependency.ResourceTypeTheme, "theme-1",
		serverconst.MaxCompositeStoreRecords, 0).
		Return([]string{"agent-1", "app-1"}, 2, nil)

	sysAttrs, _ := json.Marshal(map[string]interface{}{"name": "Agent One"})
	mockEntity.On("GetEntitiesByIDs", mock.Anything, []string{"agent-1", "app-1"}).Return([]providers.Entity{
		{ID: "agent-1", Category: providers.EntityCategoryAgent, SystemAttributes: sysAttrs},
		{ID: "app-1", Category: providers.EntityCategoryApp},
	}, nil)

	result, err := svc.GetResourceDependencies(context.Background(), resourcedependency.ResourceTypeTheme, "theme-1")
	assert.NoError(suite.T(), err)
	suite.Require().Len(result, 1)
	assert.Equal(suite.T(), "agent-1", result[0].ID)
}

// Owner dependencies are resolved by listing agents and matching the owner system attribute in
// memory, because the entity list filter only searches the public attributes column.
func (suite *AgentServiceTestSuite) TestGetResourceDependencies_ByOwner_Success() {
	svc, mockEntity, _, _, _ := suite.setupService()
	clearMockCalls(mockEntity, "GetEntityList")

	ownedAttrs, _ := json.Marshal(map[string]interface{}{"name": "Agent One", "owner": "user-1"})
	otherAttrs, _ := json.Marshal(map[string]interface{}{"name": "Agent Two", "owner": "user-2"})
	mockEntity.On("GetEntityList", mock.Anything, providers.EntityCategoryAgent,
		serverconst.MaxCompositeStoreRecords, 0, mock.Anything).
		Return([]providers.Entity{
			{ID: "agent-1", Category: providers.EntityCategoryAgent, SystemAttributes: ownedAttrs},
			{ID: "agent-2", Category: providers.EntityCategoryAgent, SystemAttributes: otherAttrs},
		}, nil)

	result, err := svc.GetResourceDependencies(context.Background(), resourcedependency.ResourceTypeUser, "user-1")
	assert.NoError(suite.T(), err)
	suite.Require().Len(result, 1)
	assert.Equal(suite.T(), resourcedependency.ResourceTypeAgent, result[0].ResourceType)
	assert.Equal(suite.T(), resourcedependency.BehaviorRestrict, result[0].BehaviorOnDelete)
	assert.Equal(suite.T(), "agent-1", result[0].ID)
	assert.Equal(suite.T(), "Agent One", result[0].DisplayName)
}

func (suite *AgentServiceTestSuite) TestGetResourceDependencies_ByOwner_Empty() {
	svc, mockEntity, _, _, _ := suite.setupService()
	clearMockCalls(mockEntity, "GetEntityList")

	otherAttrs, _ := json.Marshal(map[string]interface{}{"name": "Agent Two", "owner": "user-2"})
	mockEntity.On("GetEntityList", mock.Anything, providers.EntityCategoryAgent,
		serverconst.MaxCompositeStoreRecords, 0, mock.Anything).
		Return([]providers.Entity{
			{ID: "agent-2", Category: providers.EntityCategoryAgent, SystemAttributes: otherAttrs},
		}, nil)

	result, err := svc.GetResourceDependencies(context.Background(), resourcedependency.ResourceTypeUser, "user-1")
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), result)
}

func (suite *AgentServiceTestSuite) TestGetResourceDependencies_ByOwner_Error() {
	svc, mockEntity, _, _, _ := suite.setupService()
	clearMockCalls(mockEntity, "GetEntityList")
	mockEntity.On("GetEntityList", mock.Anything, providers.EntityCategoryAgent,
		serverconst.MaxCompositeStoreRecords, 0, mock.Anything).
		Return(nil, errors.New("store error"))

	result, err := svc.GetResourceDependencies(context.Background(), resourcedependency.ResourceTypeUser, "user-1")
	assert.Nil(suite.T(), result)
	assert.Error(suite.T(), err)
}

// --- error-branch coverage ---

func (suite *AgentServiceTestSuite) TestCreateAgent_EntityCreationFails_NonMappableError() {
	svc, mockEntity, _, _, _ := suite.setupService()

	clearMockCalls(mockEntity, "CreateEntity")
	mockEntity.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything).
		Return((*providers.Entity)(nil), errors.New("db error"))

	req := &model.Agent{Name: testAgentName, Type: testAgentType, OUID: testOUID}
	resp, svcErr := svc.CreateAgent(context.Background(), req)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestCreateAgent_InboundCreationFails_NonTranslatableError() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

	createdEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "CreateEntity")
	mockEntity.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything).
		Return(createdEntity, nil)

	clearMockCalls(mockInbound, "CreateInboundClient")
	mockInbound.On("CreateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("inbound boom"))

	clearMockCalls(mockEntity, "DeleteEntity")
	mockEntity.On("DeleteEntity", mock.Anything, mock.Anything).Return(nil)

	req := &model.Agent{
		Name:               testAgentName,
		Type:               testAgentType,
		OUID:               testOUID,
		InboundAuthProfile: providers.InboundAuthProfile{AuthFlowID: "flow-1"},
	}
	resp, svcErr := svc.CreateAgent(context.Background(), req)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
	mockEntity.AssertCalled(suite.T(), "DeleteEntity", mock.Anything, mock.Anything)
}

func (suite *AgentServiceTestSuite) TestCreateAgent_InboundFails_CompensationDeleteFails() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

	createdEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "CreateEntity")
	mockEntity.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything).
		Return(createdEntity, nil)

	clearMockCalls(mockInbound, "CreateInboundClient")
	mockInbound.On("CreateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("inbound boom"))

	clearMockCalls(mockEntity, "DeleteEntity")
	mockEntity.On("DeleteEntity", mock.Anything, mock.Anything).
		Return(errors.New("compensation delete failed"))

	req := &model.Agent{
		Name:               testAgentName,
		Type:               testAgentType,
		OUID:               testOUID,
		InboundAuthProfile: providers.InboundAuthProfile{AuthFlowID: "flow-1"},
	}
	resp, svcErr := svc.CreateAgent(context.Background(), req)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
	mockEntity.AssertCalled(suite.T(), "DeleteEntity", mock.Anything, mock.Anything)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_ExistingOAuthProfileLoadError() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockInbound, "GetOAuthProfileByEntityID")
	mockInbound.On("GetOAuthProfileByEntityID", mock.Anything, testAgentID).
		Return((*providers.OAuthProfile)(nil), errors.New("db error"))

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Name: testAgentName, Type: testAgentType,
	})
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_UpdateEntityFails_NonMappableError() {
	svc, mockEntity, _, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockEntity, "UpdateEntity")
	mockEntity.On("UpdateEntity", mock.Anything, testAgentID, mock.Anything).
		Return((*providers.Entity)(nil), errors.New("db error"))

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Name: testAgentName, Type: testAgentType,
	})
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_UpdateSystemCredentialsFails() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture("old-name", "", "", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockEntity, "UpdateEntity")
	mockEntity.On("UpdateEntity", mock.Anything, testAgentID, mock.Anything).
		Return(&providers.Entity{}, nil)

	clearMockCalls(mockEntity, "UpdateSystemCredentials")
	mockEntity.On("UpdateSystemCredentials", mock.Anything, testAgentID, mock.Anything).
		Return(errors.New("creds boom"))

	clearMockCalls(mockInbound, "UpdateInboundClient")
	mockInbound.On("UpdateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	clearMockCalls(mockInbound, "GetInboundClientByEntityID")
	mockInbound.On("GetInboundClientByEntityID", mock.Anything, testAgentID).
		Return(&inboundmodel.InboundClient{ID: testAgentID}, nil)

	req := &model.UpdateAgentRequest{
		Name: testAgentName,
		Type: testAgentType,
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					GrantTypes:              []providers.GrantType{providers.GrantTypeClientCredentials},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
				},
			},
		},
	}
	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, req)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_ReconcileUpdateInboundFails_NonTranslatableError() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture("old-name", "", "", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockInbound, "GetInboundClientByEntityID")
	mockInbound.On("GetInboundClientByEntityID", mock.Anything, testAgentID).
		Return(&inboundmodel.InboundClient{ID: testAgentID}, nil)

	clearMockCalls(mockInbound, "UpdateInboundClient")
	mockInbound.On("UpdateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("update boom"))

	req := &model.UpdateAgentRequest{
		Name:               testAgentName,
		Type:               testAgentType,
		InboundAuthProfile: providers.InboundAuthProfile{AuthFlowID: "flow-1"},
	}
	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, req)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_ReconcileCreateInboundFails_NonTranslatableError() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture("old-name", "", "", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockInbound, "GetInboundClientByEntityID")
	mockInbound.On("GetInboundClientByEntityID", mock.Anything, testAgentID).
		Return((*inboundmodel.InboundClient)(nil), inboundclient.ErrInboundClientNotFound)

	clearMockCalls(mockInbound, "CreateInboundClient")
	mockInbound.On("CreateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("create boom"))

	req := &model.UpdateAgentRequest{
		Name:               testAgentName,
		Type:               testAgentType,
		InboundAuthProfile: providers.InboundAuthProfile{AuthFlowID: "flow-1"},
	}
	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, req)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_ReconcileDeleteInboundFails_NonTranslatableError() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture("old-name", "", "", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockInbound, "GetInboundClientByEntityID")
	mockInbound.On("GetInboundClientByEntityID", mock.Anything, testAgentID).
		Return(&inboundmodel.InboundClient{ID: testAgentID}, nil)

	clearMockCalls(mockInbound, "DeleteInboundClient")
	mockInbound.On("DeleteInboundClient", mock.Anything, testAgentID).
		Return(errors.New("delete boom"))

	req := &model.UpdateAgentRequest{Name: testAgentName, Type: testAgentType}
	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, req)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestDeleteAgent_DeleteInboundClientFails_NonTranslatableError() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockInbound, "DeleteInboundClient")
	mockInbound.On("DeleteInboundClient", mock.Anything, testAgentID).
		Return(errors.New("delete boom"))

	svcErr := svc.DeleteAgent(context.Background(), testAgentID)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestDeleteAgent_DeleteEntityFails() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockInbound, "DeleteInboundClient")
	mockInbound.On("DeleteInboundClient", mock.Anything, testAgentID).
		Return(inboundclient.ErrInboundClientNotFound)

	clearMockCalls(mockEntity, "DeleteEntity")
	mockEntity.On("DeleteEntity", mock.Anything, testAgentID).
		Return(errors.New("delete entity boom"))

	svcErr := svc.DeleteAgent(context.Background(), testAgentID)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestValidateAgent_ResolveHandlesFails_NonFKError() {
	svc, _, mockInbound, _, _ := suite.setupService()

	clearMockCalls(mockInbound, "ResolveInboundAuthProfileHandles")
	mockInbound.On("ResolveInboundAuthProfileHandles", mock.Anything, mock.Anything).
		Return(errors.New("resolve boom"))

	req := &model.Agent{Name: testAgentName, Type: testAgentType, OUID: testOUID}
	_, _, _, svcErr := svc.ValidateAgent(context.Background(), req, "")
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestValidateAgent_InboundValidateFails_NonTranslatableError() {
	svc, _, mockInbound, _, _ := suite.setupService()

	clearMockCalls(mockInbound, "Validate")
	mockInbound.On("Validate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("validate boom"))

	req := &model.Agent{
		Name:               testAgentName,
		Type:               testAgentType,
		OUID:               testOUID,
		InboundAuthProfile: providers.InboundAuthProfile{AuthFlowID: "flow-1"},
	}
	_, _, _, svcErr := svc.ValidateAgent(context.Background(), req, "")
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestValidateOUExists_StoreError_NonNotFound() {
	svc, _, _, mockOU, _ := suite.setupService()

	otherErr := &tidcommon.ServiceError{Code: "SOME_OTHER_ERROR"}
	clearMockCalls(mockOU, "IsOrganizationUnitExists")
	mockOU.On("IsOrganizationUnitExists", mock.Anything, testOUID).
		Return(false, otherErr)

	svcErr := svc.validateOUExists(context.Background(), testOUID)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestDeleteAgent_AbortedWhenCascadeFails() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()
	svc.SetDependencyRegistry(noopDepRegistry{cascadeErr: errors.New("cascade failed")})

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	svcErr := svc.DeleteAgent(context.Background(), testAgentID)

	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
	mockInbound.AssertNotCalled(suite.T(), "DeleteInboundClient", mock.Anything, mock.Anything)
}
