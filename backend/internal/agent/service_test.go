// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/identitygovernance"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/identitygovernancemock"
	"github.com/thunder-id/thunderid/tests/mocks/inboundclientmock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
	"github.com/thunder-id/thunderid/tests/mocks/rolemock"
	"github.com/thunder-id/thunderid/tests/mocks/serverconfigmock"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

const (
	testAgentID   = "agent-id-123"
	testAgentName = "test-agent"
	testAgentType = "employee"
	testOUID      = "ou-id-abc"
	testAgentLogo = "avatar:shape=circle,variant=anonymous_entity,content=bot_head,colors=0"
)

// newAllowAllAuthz returns a mock SystemAuthorizationServiceInterface that allows all actions.
func newAllowAllAuthz(t interface {
	mock.TestingT
	Cleanup(func())
}) *sysauthzmock.SystemAuthorizationServiceInterfaceMock {
	authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
	authzMock.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
		Return(true, nil).Maybe()
	authzMock.On("GetAccessibleResources", mock.Anything, mock.Anything, mock.Anything).
		Return(&sysauthz.AccessibleResources{AllAllowed: true}, nil).Maybe()
	return authzMock
}

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
	mockEntity.On("GetEntityProfile", mock.Anything, mock.Anything).
		Maybe().Return((*providers.Entity)(nil), entity.ErrEntityNotFound)
	mockEntity.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
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
	mockInbound.On("Validate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
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
		Maybe().Return(oupkg.OrganizationUnit{ID: testOUID}, (*tidcommon.ServiceError)(nil))
	mockOU.On("GetOrganizationUnitHandlesByIDs", mock.Anything, mock.Anything).
		Maybe().Return(map[string]string{}, (*tidcommon.ServiceError)(nil))

	governance := identitygovernancemock.NewIdentityGovernanceServiceInterfaceMock(suite.T())
	governance.EXPECT().ReadAccess(mock.Anything, mock.Anything).RunAndReturn(activeReads).Maybe()
	governance.EXPECT().ReadEntityAccess(mock.Anything, mock.Anything).
		Return(identitygovernance.AccessRead{
			Status: providers.AccessStatusReport{Value: providers.AccessStatusActive},
		}).Maybe()

	svc := &agentService{
		logger:               log.GetLogger().With(log.String(log.LoggerKeyComponentName, "AgentService")),
		authzService:         newAllowAllAuthz(suite.T()),
		entityService:        mockEntity,
		inboundClientService: mockInbound,
		ouService:            mockOU,
		dependencyRegistry:   noopDepRegistry{},
		roleService:          mockRole,
		governance:           governance,
	}
	return svc, mockEntity, mockInbound, mockOU, mockRole
}

// activeReads answers a governance read with ACTIVE for every entity.
func activeReads(_ context.Context, entities []providers.Entity) []identitygovernance.AccessRead {
	reads := make([]identitygovernance.AccessRead, len(entities))
	for i := range reads {
		reads[i].Status.Value = providers.AccessStatusActive
	}
	return reads
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

func (noopDepRegistry) ValidateReferenceUpdate(
	context.Context, string, string) *tidcommon.ServiceError {
	return nil
}

// buildAgentEntityFixture returns an providers.Entity carrying the name as a public attribute and
// the remaining fields as system attributes.
func buildAgentEntityFixture(name, description, owner, clientID string) *providers.Entity {
	attrs := map[string]interface{}{}
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
	var publicAttrs json.RawMessage
	if name != "" {
		publicAttrs, _ = json.Marshal(map[string]interface{}{"name": name})
	}
	return &providers.Entity{
		ID:               testAgentID,
		Category:         providers.EntityCategoryAgent,
		Type:             testAgentType,
		State:            providers.EntityStateActive,
		OUID:             testOUID,
		Attributes:       publicAttrs,
		SystemAttributes: sysAttrs,
	}
}

// withKind returns the fixture with the given agent kind added to its system attributes.
func withKind(e *providers.Entity, kind providers.AgentKind) *providers.Entity {
	attrs := map[string]interface{}{}
	if len(e.SystemAttributes) > 0 {
		_ = json.Unmarshal(e.SystemAttributes, &attrs)
	}
	attrs[fieldKind] = kind
	e.SystemAttributes, _ = json.Marshal(attrs)
	return e
}

// --- pure helper tests ---

func (suite *AgentServiceTestSuite) TestNeedsInboundClient_NilRequest() {
	assert.False(suite.T(), needsInboundClient(nil))
}

func (suite *AgentServiceTestSuite) TestNeedsInboundClient_EmptyRequest() {
	assert.False(suite.T(), needsInboundClient(&providers.Agent{}))
}

func (suite *AgentServiceTestSuite) TestNeedsInboundClient_WithAuthFlowID() {
	req := &providers.Agent{InboundAuthProfile: providers.InboundAuthProfile{AuthFlowID: "flow-1"}}
	assert.True(suite.T(), needsInboundClient(req))
}

func (suite *AgentServiceTestSuite) TestNeedsInboundClient_WithInboundAuthConfig() {
	req := &providers.Agent{
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{Type: providers.OAuthInboundAuthType, OAuthConfig: &providers.OAuthConfigWithSecret{}},
		},
	}
	assert.True(suite.T(), needsInboundClient(req))
}

func (suite *AgentServiceTestSuite) TestNeedsInboundClient_WithAllowedUserTypes() {
	req := &providers.Agent{
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
	req := &model.UpdateAgentRequest{InboundAuthProfileReq: inboundmodel.InboundAuthProfileReq{ThemeID: "theme-abc"}}
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
	assert.Equal(suite.T(), agentSystemAttributes{}, readSystemAttributes(nil))
}

func (suite *AgentServiceTestSuite) TestReadSystemAttributes_InvalidJSON() {
	assert.Equal(suite.T(), agentSystemAttributes{}, readSystemAttributes(json.RawMessage("{")))
}

func (suite *AgentServiceTestSuite) TestReadSystemAttributes_AllFields() {
	raw, _ := json.Marshal(map[string]interface{}{
		"description": "desc",
		"owner":       "alice",
		"clientId":    "cid-123",
		"kind":        providers.AgentKindPersonalAssistant,
	})
	assert.Equal(suite.T(), agentSystemAttributes{
		description: "desc",
		owner:       "alice",
		clientID:    "cid-123",
		kind:        providers.AgentKindPersonalAssistant,
	}, readSystemAttributes(raw))
}

func (suite *AgentServiceTestSuite) TestBuildSystemAttributesJSON_AllFields() {
	sysAttrs := agentSystemAttributes{description: "d", owner: "o", clientID: "c", kind: "x"}
	raw, err := buildSystemAttributesJSON(sysAttrs)
	suite.Require().NoError(err)
	suite.Require().NotNil(raw)
	assert.Equal(suite.T(), sysAttrs, readSystemAttributes(raw))
}

func (suite *AgentServiceTestSuite) TestBuildSystemAttributesJSON_EmptyFields() {
	raw, err := buildSystemAttributesJSON(agentSystemAttributes{})
	suite.Require().NoError(err)
	assert.Nil(suite.T(), raw)
}

// --- validateBaseFields ---

func (suite *AgentServiceTestSuite) TestValidateBaseFields_MissingType() {
	svcErr := validateBaseFields("")
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorInvalidAgentType.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestValidateBaseFields_Valid() {
	svcErr := validateBaseFields("type")
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

func (suite *AgentServiceTestSuite) TestCreateAgent_MissingType() {
	svc, _, _, _, _ := suite.setupService()
	req := &providers.Agent{OUID: testOUID}
	resp, svcErr := svc.CreateAgent(context.Background(), req)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorInvalidAgentType.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestCreateAgent_OUNotFound() {
	svc, _, _, mockOU, _ := suite.setupService()
	clearMockCalls(mockOU, "IsOrganizationUnitExists")
	mockOU.On("IsOrganizationUnitExists", mock.Anything, testOUID).Return(false, (*tidcommon.ServiceError)(nil))

	req := &providers.Agent{Type: testAgentType, OUID: testOUID}
	resp, svcErr := svc.CreateAgent(context.Background(), req)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorOrganizationUnitNotFound.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestCreateAgent_NameAlreadyExists() {
	svc, mockEntity, _, _, _ := suite.setupService()
	clearMockCalls(mockEntity, "CreateEntity")
	mockEntity.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return((*providers.Entity)(nil), entity.ErrAttributeConflict)

	req := &providers.Agent{
		Type: testAgentType, OUID: testOUID, Attributes: json.RawMessage(`{"name":"test-agent"}`),
	}
	resp, svcErr := svc.CreateAgent(context.Background(), req)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorAttributeConflict.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestCreateAgent_EntityOnly_Success() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

	createdEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "CreateEntity")
	mockEntity.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(createdEntity, nil)

	req := &providers.Agent{
		Type: testAgentType,
		OUID: testOUID,
	}
	resp, svcErr := svc.CreateAgent(context.Background(), req)
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.JSONEq(suite.T(), `{"name":"test-agent"}`, string(resp.Attributes))
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
	}), mock.Anything, mock.Anything).Return(createdEntity, nil)

	req := &providers.Agent{Type: testAgentType, OUID: testOUID}
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
	}), mock.Anything, mock.Anything).Return(createdEntity, nil)

	req := &providers.Agent{ID: presetID, Type: testAgentType, OUID: testOUID}
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
	mockEntity.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(createdEntity, nil)

	clearMockCalls(mockInbound, "CreateInboundClient")
	mockInbound.On("CreateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	req := &providers.Agent{
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

func (suite *AgentServiceTestSuite) TestCreateAgent_WithLogo_PersistsLogoProperty() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

	createdEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "CreateEntity")
	mockEntity.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(createdEntity, nil)

	var capturedClient *inboundmodel.InboundClient
	clearMockCalls(mockInbound, "CreateInboundClient")
	mockInbound.On("CreateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			capturedClient = args.Get(1).(*inboundmodel.InboundClient)
		}).Return(nil)

	req := &providers.Agent{
		Type:               testAgentType,
		OUID:               testOUID,
		LogoURL:            testAgentLogo,
		InboundAuthProfile: providers.InboundAuthProfile{AuthFlowID: "flow-1"},
	}
	resp, svcErr := svc.CreateAgent(context.Background(), req)
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Equal(suite.T(), testAgentLogo, resp.LogoURL)
	suite.Require().NotNil(capturedClient)
	assert.Equal(suite.T(), testAgentLogo, capturedClient.Properties[propLogoURL])
}

func (suite *AgentServiceTestSuite) TestCreateAgent_InvalidLogoURL() {
	svc, _, _, _, _ := suite.setupService()

	req := &providers.Agent{
		Type:    testAgentType,
		OUID:    testOUID,
		LogoURL: "javascript:alert(1)",
	}
	resp, svcErr := svc.CreateAgent(context.Background(), req)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorInvalidLogoURL.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestCreateAgent_FlowIDResolvedToDefault() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

	createdEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "CreateEntity")
	mockEntity.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(createdEntity, nil)

	clearMockCalls(mockInbound, "CreateInboundClient")
	mockInbound.On("CreateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			client := args.Get(1).(*inboundmodel.InboundClient)
			client.AuthFlowID = "default-flow-id"
			client.RegistrationFlowID = "default-reg-flow-id"
		}).Return(nil)

	req := &providers.Agent{
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
	mockEntity.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(createdEntity, nil)

	clearMockCalls(mockInbound, "CreateInboundClient")
	mockInbound.On("CreateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	req := &providers.Agent{
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

// captureCreatedOAuthProfile stubs inbound client creation and returns a pointer to the OAuth profile
// the service hands it.
func (suite *AgentServiceTestSuite) captureCreatedOAuthProfile(
	mockEntity *entitymock.EntityServiceInterfaceMock,
	mockInbound *inboundclientmock.InboundClientServiceInterfaceMock,
) **providers.OAuthProfile {
	clearMockCalls(mockEntity, "CreateEntity")
	mockEntity.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(buildAgentEntityFixture(testAgentName, "", "", "cid-xxx"), nil)

	captured := new(*providers.OAuthProfile)
	clearMockCalls(mockInbound, "CreateInboundClient")
	mockInbound.On("CreateInboundClient", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { *captured = args.Get(2).(*providers.OAuthProfile) }).
		Return(nil)
	return captured
}

func (suite *AgentServiceTestSuite) TestCreateAgent_DelegatedGetsTheDefaultGrants() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()
	profile := suite.captureCreatedOAuthProfile(mockEntity, mockInbound)

	req := &providers.Agent{
		Type:      testAgentType,
		OUID:      testOUID,
		Delegated: true,
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type:        providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{RedirectURIs: []string{"https://app.example.com/cb"}},
			},
		},
	}
	resp, svcErr := svc.CreateAgent(context.Background(), req)

	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	suite.Require().NotNil(*profile)
	assert.Equal(suite.T(), []string{"client_credentials", "authorization_code", "refresh_token"},
		(*profile).GrantTypes)
	assert.Equal(suite.T(), []string{"code"}, (*profile).ResponseTypes)
	assert.True(suite.T(), (*profile).PKCERequired)
	assert.Equal(suite.T(), []string{"https://app.example.com/cb"}, (*profile).RedirectURIs)
}

func (suite *AgentServiceTestSuite) TestCreateAgent_NotDelegatedAlwaysHoldsClientCredentials() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()
	profile := suite.captureCreatedOAuthProfile(mockEntity, mockInbound)

	req := &providers.Agent{
		Type: testAgentType,
		OUID: testOUID,
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					GrantTypes: []providers.GrantType{providers.GrantTypeRefreshToken},
				},
			},
		},
	}
	_, svcErr := svc.CreateAgent(context.Background(), req)

	suite.Require().Nil(svcErr)
	suite.Require().NotNil(*profile)
	assert.Equal(suite.T(), []string{"client_credentials", "refresh_token"}, (*profile).GrantTypes)
}

func (suite *AgentServiceTestSuite) TestCreateAgent_DelegatedWithoutADelegationGrantIsRejected() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

	req := &providers.Agent{
		Type:      testAgentType,
		OUID:      testOUID,
		Delegated: true,
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					GrantTypes: []providers.GrantType{providers.GrantTypeRefreshToken},
				},
			},
		},
	}
	resp, svcErr := svc.CreateAgent(context.Background(), req)

	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorMissingDelegationGrantType.Code, svcErr.Code)
	mockEntity.AssertNotCalled(suite.T(), "CreateEntity", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	mockInbound.AssertNotCalled(suite.T(), "Validate", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything)
}

func (suite *AgentServiceTestSuite) TestCreateAgent_EntityCreationFails() {
	svc, mockEntity, _, _, _ := suite.setupService()

	clearMockCalls(mockEntity, "CreateEntity")
	mockEntity.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return((*providers.Entity)(nil), entity.ErrSchemaValidationFailed)

	req := &providers.Agent{Type: testAgentType, OUID: testOUID}
	resp, svcErr := svc.CreateAgent(context.Background(), req)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorSchemaValidationFailed.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestCreateAgent_InboundCreationFails_CompensatesEntity() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

	createdEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "CreateEntity")
	mockEntity.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(createdEntity, nil)

	clearMockCalls(mockInbound, "CreateInboundClient")
	mockInbound.On("CreateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(inboundclient.ErrOAuthInvalidGrantType)

	clearMockCalls(mockEntity, "DeleteEntity")
	mockEntity.On("DeleteEntity", mock.Anything, mock.Anything).Return(nil)

	req := &providers.Agent{
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

	agentEntity := withKind(buildAgentEntityFixture(testAgentName, "desc", "alice", ""),
		providers.AgentKindPersonalAssistant)

	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	resp, svcErr := svc.GetAgent(context.Background(), testAgentID, false)
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Equal(suite.T(), testAgentID, resp.ID)
	assert.JSONEq(suite.T(), `{"name":"test-agent"}`, string(resp.Attributes))
	assert.Equal(suite.T(), "desc", resp.Description)
	assert.Equal(suite.T(), "alice", resp.Owner)
	assert.Equal(suite.T(), providers.AgentKindPersonalAssistant, resp.Kind)
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

func (suite *AgentServiceTestSuite) TestGetAgent_ReturnsLogoFromProperties() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	inboundRec := &inboundmodel.InboundClient{
		ID:         testAgentID,
		AuthFlowID: "flow-1",
		Properties: map[string]interface{}{propLogoURL: testAgentLogo},
	}
	clearMockCalls(mockInbound, "GetInboundClientByEntityID")
	mockInbound.On("GetInboundClientByEntityID", mock.Anything, testAgentID).Return(inboundRec, nil)

	resp, svcErr := svc.GetAgent(context.Background(), testAgentID, false)
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Equal(suite.T(), testAgentLogo, resp.LogoURL)
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
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

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
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

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

	agentEntity := withKind(buildAgentEntityFixture(testAgentName, "desc", "alice", ""),
		providers.AgentKindPersonalAssistant)
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
	assert.JSONEq(suite.T(), `{"name":"test-agent"}`, string(resp.Agents[0].Attributes))
	assert.Equal(suite.T(), providers.AgentKindPersonalAssistant, resp.Agents[0].Kind)
}

func (suite *AgentServiceTestSuite) TestGetAgentList_ReturnsLogoFromInboundClients() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "GetEntityList")
	mockEntity.On("GetEntityList", mock.Anything, providers.EntityCategoryAgent, 30, 0, mock.Anything).
		Return([]providers.Entity{*agentEntity}, nil)
	clearMockCalls(mockEntity, "GetEntityListCount")
	mockEntity.On("GetEntityListCount", mock.Anything, providers.EntityCategoryAgent, mock.Anything).
		Return(1, nil)

	clearMockCalls(mockInbound, "GetInboundClientByEntityID")
	mockInbound.On("GetInboundClientByEntityID", mock.Anything, testAgentID).
		Return(&inboundmodel.InboundClient{
			ID:         testAgentID,
			Properties: map[string]interface{}{propLogoURL: testAgentLogo},
		}, nil)

	resp, svcErr := svc.GetAgentList(context.Background(), 0, 0, nil, false)
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	suite.Require().Len(resp.Agents, 1)
	assert.Equal(suite.T(), testAgentLogo, resp.Agents[0].LogoURL)
}

func (suite *AgentServiceTestSuite) TestGetAgentList_LogoLookupError_StillReturns() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "GetEntityList")
	mockEntity.On("GetEntityList", mock.Anything, providers.EntityCategoryAgent, 30, 0, mock.Anything).
		Return([]providers.Entity{*agentEntity}, nil)
	clearMockCalls(mockEntity, "GetEntityListCount")
	mockEntity.On("GetEntityListCount", mock.Anything, providers.EntityCategoryAgent, mock.Anything).
		Return(1, nil)

	clearMockCalls(mockInbound, "GetInboundClientByEntityID")
	mockInbound.On("GetInboundClientByEntityID", mock.Anything, testAgentID).
		Return((*inboundmodel.InboundClient)(nil), assert.AnError)

	resp, svcErr := svc.GetAgentList(context.Background(), 0, 0, nil, false)
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	suite.Require().Len(resp.Agents, 1)
	assert.Empty(suite.T(), resp.Agents[0].LogoURL)
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
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

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
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

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
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

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
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

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
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

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
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).
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
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(wrongCatEntity, nil)

	resp, svcErr := svc.GetAgentRoles(context.Background(), testAgentID, 10, 0)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorAgentNotFound.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestGetAgentRoles_CountError() {
	svc, mockEntity, _, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

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
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

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
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

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
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

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
		Type: testAgentType,
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
		Type: testAgentType,
	})
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorAgentNotFound.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_Success_EntityOnly() {
	svc, mockEntity, _, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture("old-name", "", "", "")
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockEntity, "UpdateEntity")
	mockEntity.On("UpdateEntity", mock.Anything, testAgentID, mock.Anything).
		Return(&providers.Entity{}, nil)

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Type: testAgentType, Attributes: json.RawMessage(`{"name":"test-agent"}`),
	})
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.JSONEq(suite.T(), `{"name":"test-agent"}`, string(resp.Attributes))
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_FlowIDResolvedToDefault() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture("old-name", "", "", "")
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

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

func (suite *AgentServiceTestSuite) TestUpdateAgent_ResolvesAuthFlowHandle() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture("old-name", "", "", "")
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockEntity, "UpdateEntity")
	mockEntity.On("UpdateEntity", mock.Anything, testAgentID, mock.Anything).
		Return(&providers.Entity{}, nil)

	clearMockCalls(mockInbound, "GetInboundClientByEntityID")
	mockInbound.On("GetInboundClientByEntityID", mock.Anything, mock.Anything).
		Return(&inboundmodel.InboundClient{}, nil)

	clearMockCalls(mockInbound, "ResolveInboundAuthProfileHandles")
	mockInbound.On("ResolveInboundAuthProfileHandles", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			profile := args.Get(1).(*providers.InboundAuthProfile)
			assert.Equal(suite.T(), "wayfinder-agent-auth-flow", profile.AuthFlowHandle)
			profile.AuthFlowID = "resolved-auth-flow-id"
		}).Return(nil)

	var capturedAuthFlowID string
	clearMockCalls(mockInbound, "UpdateInboundClient")
	mockInbound.On("UpdateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			client := args.Get(1).(*inboundmodel.InboundClient)
			capturedAuthFlowID = client.AuthFlowID
		}).Return(nil)

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Type: testAgentType,
		InboundAuthProfileReq: inboundmodel.InboundAuthProfileReq{
			AuthFlowHandle: "wayfinder-agent-auth-flow",
		},
	})
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Equal(suite.T(), "resolved-auth-flow-id", capturedAuthFlowID)
	assert.Equal(suite.T(), "resolved-auth-flow-id", resp.AuthFlowID)
	mockInbound.AssertNotCalled(suite.T(), "DeleteInboundClient", mock.Anything, mock.Anything)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_ResolveHandlesFails_NonFKError() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture("old-name", "", "", "")
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockInbound, "GetInboundClientByEntityID")
	mockInbound.On("GetInboundClientByEntityID", mock.Anything, mock.Anything).
		Return(&inboundmodel.InboundClient{}, nil)

	clearMockCalls(mockInbound, "ResolveInboundAuthProfileHandles")
	mockInbound.On("ResolveInboundAuthProfileHandles", mock.Anything, mock.Anything).
		Return(errors.New("resolve boom"))

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Type: testAgentType,
		InboundAuthProfileReq: inboundmodel.InboundAuthProfileReq{
			AuthFlowHandle: "wayfinder-agent-auth-flow",
		},
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
	suite.Require().Nil(resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

// --- owner validation ---

func (suite *AgentServiceTestSuite) TestValidateOwnerExists_Empty() {
	svc, _, _, _, _ := suite.setupService()
	owner, svcErr := svc.validateOwnerExists(context.Background(), "")
	assert.Nil(suite.T(), owner)
	assert.Nil(suite.T(), svcErr)
}

func (suite *AgentServiceTestSuite) TestValidateOwnerExists_NotFound() {
	svc, mockEntity, _, _, _ := suite.setupService()
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, "missing-owner").
		Return((*providers.Entity)(nil), entity.ErrEntityNotFound)

	_, svcErr := svc.validateOwnerExists(context.Background(), "missing-owner")
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorOwnerNotFound.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestValidateOwnerExists_StoreError() {
	svc, mockEntity, _, _, _ := suite.setupService()
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, "owner-x").
		Return((*providers.Entity)(nil), errors.New("db error"))

	_, svcErr := svc.validateOwnerExists(context.Background(), "owner-x")
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestValidateOwnerExists_Success() {
	svc, mockEntity, _, _, _ := suite.setupService()
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, "owner-y").
		Return(&providers.Entity{ID: "owner-y"}, nil)

	owner, svcErr := svc.validateOwnerExists(context.Background(), "owner-y")
	assert.Nil(suite.T(), svcErr)
	assert.NotNil(suite.T(), owner)
}

func (suite *AgentServiceTestSuite) TestCreateAgent_OwnerNotFound() {
	svc, mockEntity, _, _, _ := suite.setupService()
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, "ghost").
		Return((*providers.Entity)(nil), entity.ErrEntityNotFound)

	resp, svcErr := svc.CreateAgent(context.Background(), &providers.Agent{
		Type: testAgentType, OUID: testOUID, Owner: "ghost",
	})
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorOwnerNotFound.Code, svcErr.Code)
}

// personalAgentCreateRequest returns a personal agent create request with an authorization code
// OAuth profile, as an import would pass it.
func personalAgentCreateRequest(owner string, grants ...providers.GrantType) *providers.Agent {
	return &providers.Agent{
		Type:               testAgentType,
		OUID:               testOUID,
		Owner:              owner,
		Kind:               providers.AgentKindPersonalAssistant,
		InboundAuthProfile: providers.InboundAuthProfile{AuthFlowID: "flow-1"},
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					GrantTypes:              grants,
					ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
					RedirectURIs:            []string{"https://agent.example.com/callback"},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
				},
			},
		},
	}
}

// enablePersonalAgents makes the service read personal agents as enabled.
func (suite *AgentServiceTestSuite) enablePersonalAgents(svc *agentService) {
	mockServerConfig := serverconfigmock.NewServerConfigServiceMock(suite.T())
	mockServerConfig.EXPECT().GetMergedConfig(mock.Anything, "personalAgent").
		Return(PersonalAgentConfig{Enabled: boolPtr(true)}, nil).Maybe()
	svc.serverConfigService = mockServerConfig
}

func (suite *AgentServiceTestSuite) TestCreateAgent_PersonalAgent_Success() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()
	suite.enablePersonalAgents(svc)
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, "user-1").
		Return(&providers.Entity{ID: "user-1", Category: providers.EntityCategoryUser}, nil)

	var storedSysAttrs agentSystemAttributes
	clearMockCalls(mockEntity, "CreateEntity")
	mockEntity.On("CreateEntity", mock.Anything, mock.MatchedBy(func(e *providers.Entity) bool {
		storedSysAttrs = readSystemAttributes(e.SystemAttributes)
		return true
	}), mock.Anything, mock.Anything).Return(buildAgentEntityFixture(testAgentName, "", "user-1", ""), nil)
	clearMockCalls(mockInbound, "CreateInboundClient")
	mockInbound.On("CreateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	resp, svcErr := svc.CreateAgent(context.Background(), personalAgentCreateRequest("user-1",
		providers.GrantTypeAuthorizationCode, providers.GrantTypeCIBA, providers.GrantTypeRefreshToken))
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Equal(suite.T(), providers.AgentKindPersonalAssistant, resp.Kind)
	assert.Equal(suite.T(), "user-1", storedSysAttrs.owner)
	assert.Equal(suite.T(), providers.AgentKindPersonalAssistant, storedSysAttrs.kind)
}

// No personal agent is created while personal agents are disabled, whether a registration flow, the agent
// API or an import creates it.
func (suite *AgentServiceTestSuite) TestCreateAgent_PersonalAgent_RefusedWhileDisabled() {
	for name, ctx := range map[string]context.Context{
		"RegistrationFlow": security.WithRuntimeContext(context.Background()),
		"AgentAPIOrImport": context.Background(),
	} {
		suite.Run(name, func() {
			svc, mockEntity, _, _, _ := suite.setupService()
			mockServerConfig := serverconfigmock.NewServerConfigServiceMock(suite.T())
			mockServerConfig.EXPECT().GetMergedConfig(mock.Anything, "personalAgent").
				Return(PersonalAgentConfig{Enabled: boolPtr(false)}, nil)
			svc.serverConfigService = mockServerConfig

			resp, svcErr := svc.CreateAgent(ctx,
				personalAgentCreateRequest("user-1", providers.GrantTypeAuthorizationCode))

			assert.Nil(suite.T(), resp)
			suite.Require().NotNil(svcErr)
			assert.Equal(suite.T(), ErrorPersonalAgentsDisabled.Code, svcErr.Code)
			mockEntity.AssertNotCalled(suite.T(), "CreateEntity",
				mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

// serviceWithPersonalAgentLoginFlow returns a service whose personalAgent section names the given login flow.
func (suite *AgentServiceTestSuite) serviceWithPersonalAgentLoginFlow(handle string) *agentService {
	svc, _, _, _, _ := suite.setupService()
	mockServerConfig := serverconfigmock.NewServerConfigServiceMock(suite.T())
	mockServerConfig.EXPECT().GetMergedConfig(mock.Anything, "personalAgent").
		Return(PersonalAgentConfig{
			Enabled:  boolPtr(true),
			AuthFlow: PersonalAgentAuthFlowConfig{DefaultHandle: handle},
		}, nil).Maybe()
	svc.serverConfigService = mockServerConfig
	return svc
}

// A personal agent's login flow is the one given; with none, the organization's personal agent login flow.
func (suite *AgentServiceTestSuite) TestApplyPersonalAgentSetup_LoginFlow() {
	for name, tc := range map[string]struct {
		flowID, flowHandle, configured string
		wantID, wantHandle             string
	}{
		"given flow ID kept":     {flowID: "flow-1", wantID: "flow-1"},
		"given flow handle kept": {flowHandle: "caller-flow", wantHandle: "caller-flow"},
		"configured fallback":    {configured: "personal-login", wantHandle: "personal-login"},
		"built-in fallback":      {wantHandle: defaultPersonalAgentAuthFlowHandle},
	} {
		suite.Run(name, func() {
			svc := suite.serviceWithPersonalAgentLoginFlow(tc.configured)
			agent := personalAgentCreateRequest("user-1", providers.GrantTypeAuthorizationCode)
			agent.AuthFlowID, agent.AuthFlowHandle = tc.flowID, tc.flowHandle

			suite.Require().Nil(svc.applyPersonalAgentSetup(context.Background(), agent))
			assert.Equal(suite.T(), tc.wantID, agent.AuthFlowID)
			assert.Equal(suite.T(), tc.wantHandle, agent.AuthFlowHandle)
		})
	}
}

// Consent is asked on every authorization unless the caller says otherwise.
func (suite *AgentServiceTestSuite) TestApplyPersonalAgentSetup_AlwaysPrompt() {
	svc := suite.serviceWithPersonalAgentLoginFlow("")
	notGiven := personalAgentCreateRequest("user-1", providers.GrantTypeAuthorizationCode)
	suite.Require().Nil(svc.applyPersonalAgentSetup(context.Background(), notGiven))
	suite.Require().NotNil(notGiven.LoginConsent.AlwaysPrompt)
	assert.True(suite.T(), *notGiven.LoginConsent.AlwaysPrompt)

	off := false
	given := personalAgentCreateRequest("user-1", providers.GrantTypeAuthorizationCode)
	given.LoginConsent = &providers.LoginConsentConfig{ValidityPeriod: 60, AlwaysPrompt: &off}
	suite.Require().Nil(svc.applyPersonalAgentSetup(context.Background(), given))
	assert.False(suite.T(), *given.LoginConsent.AlwaysPrompt, "an explicit false is kept")
	assert.Equal(suite.T(), int64(60), given.LoginConsent.ValidityPeriod)
}

// The OAuth client is completed: added when missing, the authorization code grant by default, response type
// code and PKCE with that grant, and the authentication method from the key.
func (suite *AgentServiceTestSuite) TestApplyPersonalAgentSetup_OAuthConfig() {
	svc := suite.serviceWithPersonalAgentLoginFlow("")
	withKey := func(certificate *providers.Certificate) *providers.Agent {
		agent := personalAgentCreateRequest("user-1")
		agent.InboundAuthConfig[0].OAuthConfig.Certificate = certificate
		return agent
	}
	for name, tc := range map[string]struct {
		agent      *providers.Agent
		wantGrants []providers.GrantType
		wantMethod providers.TokenEndpointAuthMethod
		wantPKCE   bool
	}{
		"no OAuth config": {agent: &providers.Agent{Type: testAgentType, OUID: testOUID, Owner: "user-1",
			Kind: providers.AgentKindPersonalAssistant},
			wantGrants: []providers.GrantType{providers.GrantTypeAuthorizationCode},
			wantMethod: providers.TokenEndpointAuthMethodClientSecretBasic, wantPKCE: true},
		"given grants kept": {agent: personalAgentCreateRequest("user-1", providers.GrantTypeAuthorizationCode,
			providers.GrantTypeRefreshToken),
			wantGrants: []providers.GrantType{providers.GrantTypeAuthorizationCode, providers.GrantTypeRefreshToken},
			wantMethod: providers.TokenEndpointAuthMethodClientSecretBasic, wantPKCE: true},
		"CIBA only": {agent: personalAgentCreateRequest("user-1", providers.GrantTypeCIBA),
			wantGrants: []providers.GrantType{providers.GrantTypeCIBA},
			wantMethod: providers.TokenEndpointAuthMethodClientSecretBasic},
		"public key": {agent: withKey(&providers.Certificate{Type: providers.CertificateTypeJWKS,
			Value: `{"keys":[{"kty":"EC"}]}`}),
			wantGrants: []providers.GrantType{providers.GrantTypeAuthorizationCode},
			wantMethod: providers.TokenEndpointAuthMethodPrivateKeyJWT, wantPKCE: true},
		"JWKS URI": {agent: withKey(&providers.Certificate{Type: providers.CertificateTypeJWKSURI,
			Value: "https://agent.example.com/jwks.json"}),
			wantGrants: []providers.GrantType{providers.GrantTypeAuthorizationCode},
			wantMethod: providers.TokenEndpointAuthMethodPrivateKeyJWT, wantPKCE: true},
	} {
		suite.Run(name, func() {
			suite.Require().Nil(svc.applyPersonalAgentSetup(context.Background(), tc.agent))
			suite.Require().Len(tc.agent.InboundAuthConfig, 1)
			cfg := tc.agent.InboundAuthConfig[0].OAuthConfig
			suite.Require().NotNil(cfg)
			assert.Equal(suite.T(), tc.wantGrants, cfg.GrantTypes)
			assert.Equal(suite.T(), tc.wantMethod, cfg.TokenEndpointAuthMethod)
			assert.Equal(suite.T(), tc.wantPKCE, cfg.PKCERequired)
			if tc.wantPKCE {
				assert.Equal(suite.T(), []providers.ResponseType{providers.ResponseTypeCode}, cfg.ResponseTypes)
			}
		})
	}
}

// A key that cannot be used, or a grant outside the personal set, is refused.
func (suite *AgentServiceTestSuite) TestApplyPersonalAgentSetup_Refusals() {
	svc := suite.serviceWithPersonalAgentLoginFlow("")
	withKey := func(certificate *providers.Certificate) *providers.Agent {
		agent := personalAgentCreateRequest("user-1", providers.GrantTypeAuthorizationCode)
		agent.InboundAuthConfig[0].OAuthConfig.Certificate = certificate
		return agent
	}
	for name, tc := range map[string]struct {
		agent    *providers.Agent
		wantCode string
	}{
		"public key without keys": {agent: withKey(&providers.Certificate{Type: providers.CertificateTypeJWKS,
			Value: `{"keys":[]}`}), wantCode: ErrorInvalidPublicKey.Code},
		"public key that is not JSON": {agent: withKey(&providers.Certificate{Type: providers.CertificateTypeJWKS,
			Value: "not-json"}), wantCode: ErrorInvalidPublicKey.Code},
		"plain HTTP JWKS URI": {agent: withKey(&providers.Certificate{Type: providers.CertificateTypeJWKSURI,
			Value: "http://agent.example.com/jwks.json"}), wantCode: ErrorInvalidJWKSURI.Code},
		"private address JWKS URI": {agent: withKey(&providers.Certificate{Type: providers.CertificateTypeJWKSURI,
			Value: "https://127.0.0.1/jwks.json"}), wantCode: ErrorInvalidJWKSURI.Code},
		"client credentials": {agent: personalAgentCreateRequest("user-1", providers.GrantTypeClientCredentials),
			wantCode: ErrorGrantTypeNotAllowedForPersonalAgent.Code},
		"two OAuth configs": {agent: &providers.Agent{Kind: providers.AgentKindPersonalAssistant,
			InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
				{Type: providers.OAuthInboundAuthType}, {Type: providers.OAuthInboundAuthType},
			}}, wantCode: ErrorMultipleOAuthConfigs.Code},
	} {
		suite.Run(name, func() {
			svcErr := svc.applyPersonalAgentSetup(context.Background(), tc.agent)
			suite.Require().NotNil(svcErr)
			assert.Equal(suite.T(), tc.wantCode, svcErr.Code)
		})
	}
}

func (suite *AgentServiceTestSuite) TestCreateAgent_PersonalAgent_OwnerRefusals() {
	tests := []struct {
		name     string
		owner    string
		ownerEnt *providers.Entity
		ownerErr error
		wantCode string
	}{
		{name: "missing owner", owner: "", wantCode: ErrorOwnerNotFound.Code},
		{name: "owner not found", owner: "ghost", ownerErr: entity.ErrEntityNotFound,
			wantCode: ErrorOwnerNotFound.Code},
		{name: "owner is an agent", owner: "agent-2",
			ownerEnt: &providers.Entity{ID: "agent-2", Category: providers.EntityCategoryAgent},
			wantCode: ErrorOwnerNotFound.Code},
		{name: "owner lookup fails", owner: "user-1", ownerErr: errors.New("db down"),
			wantCode: tidcommon.InternalServerError.Code},
	}
	for _, tc := range tests {
		suite.Run(tc.name, func() {
			svc, mockEntity, _, _, _ := suite.setupService()
			suite.enablePersonalAgents(svc)
			if tc.owner != "" {
				clearMockCalls(mockEntity, "GetEntityProfile")
				mockEntity.On("GetEntityProfile", mock.Anything, tc.owner).Return(tc.ownerEnt, tc.ownerErr)
			}

			resp, svcErr := svc.CreateAgent(context.Background(),
				personalAgentCreateRequest(tc.owner, providers.GrantTypeAuthorizationCode))
			assert.Nil(suite.T(), resp)
			suite.Require().NotNil(svcErr)
			assert.Equal(suite.T(), tc.wantCode, svcErr.Code)
			mockEntity.AssertNotCalled(suite.T(), "CreateEntity",
				mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func (suite *AgentServiceTestSuite) TestCreateAgent_PersonalAgent_GrantRefusals() {
	tests := []struct {
		name   string
		grants []providers.GrantType
	}{
		{name: "client credentials", grants: []providers.GrantType{providers.GrantTypeClientCredentials}},
		{name: "token exchange", grants: []providers.GrantType{
			providers.GrantTypeAuthorizationCode, providers.GrantTypeTokenExchange}},
		{name: "jwt bearer", grants: []providers.GrantType{providers.GrantTypeJWTBearer}},
	}
	for _, tc := range tests {
		suite.Run(tc.name, func() {
			svc, mockEntity, _, _, _ := suite.setupService()
			suite.enablePersonalAgents(svc)

			resp, svcErr := svc.CreateAgent(context.Background(),
				personalAgentCreateRequest("user-1", tc.grants...))
			assert.Nil(suite.T(), resp)
			suite.Require().NotNil(svcErr)
			assert.Equal(suite.T(), ErrorGrantTypeNotAllowedForPersonalAgent.Code, svcErr.Code)
			mockEntity.AssertNotCalled(suite.T(), "CreateEntity",
				mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

// An unknown kind is refused before anything is created, so a typo cannot create an ordinary agent.
func (suite *AgentServiceTestSuite) TestCreateAgent_UnknownKindRefused() {
	svc, mockEntity, _, _, _ := suite.setupService()

	resp, svcErr := svc.CreateAgent(context.Background(), &providers.Agent{
		Type: testAgentType, OUID: testOUID, Kind: "personal_assistant",
	})
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorInvalidKind.Code, svcErr.Code)
	mockEntity.AssertNotCalled(suite.T(), "CreateEntity", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_OwnerChanged_OwnerNotFound() {
	svc, mockEntity, _, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture("old-name", "", "current-owner", "")
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)
	mockEntity.On("GetEntityProfile", mock.Anything, "new-owner").
		Return((*providers.Entity)(nil), entity.ErrEntityNotFound)

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Type: testAgentType, Owner: "new-owner",
	})
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorOwnerNotFound.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_OwnerChanged_Success() {
	svc, mockEntity, _, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture("old-name", "", "current-owner", "")
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)
	mockEntity.On("GetEntityProfile", mock.Anything, "new-owner").
		Return(&providers.Entity{ID: "new-owner"}, nil)

	clearMockCalls(mockEntity, "UpdateEntity")
	mockEntity.On("UpdateEntity", mock.Anything, testAgentID, mock.Anything).
		Return(&providers.Entity{}, nil)

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Type: testAgentType, Owner: "new-owner",
	})
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Equal(suite.T(), "new-owner", resp.Owner)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_CarriesKindForward() {
	svc, mockEntity, _, _, _ := suite.setupService()

	agentEntity := withKind(buildAgentEntityFixture("old-name", "", "owner-1", ""), "future-kind")
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

	var storedKind providers.AgentKind
	clearMockCalls(mockEntity, "UpdateEntity")
	mockEntity.On("UpdateEntity", mock.Anything, testAgentID, mock.MatchedBy(func(e *providers.Entity) bool {
		storedKind = readSystemAttributes(e.SystemAttributes).kind
		return true
	})).Return(&providers.Entity{}, nil)

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Type: testAgentType,
	})
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Equal(suite.T(), providers.AgentKind("future-kind"), storedKind)
	assert.Equal(suite.T(), providers.AgentKind("future-kind"), resp.Kind)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_PersonalAgentRefused() {
	svc, mockEntity, _, _, _ := suite.setupService()

	agentEntity := withKind(buildAgentEntityFixture("old-name", "", "owner-1", ""),
		providers.AgentKindPersonalAssistant)
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Type: testAgentType, Owner: "owner-1",
	})
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorCannotUpdatePersonalAgent.Code, svcErr.Code)
	mockEntity.AssertNotCalled(suite.T(), "UpdateEntity", mock.Anything, mock.Anything, mock.Anything)
}

// --- GetClientRules ---

func (suite *AgentServiceTestSuite) TestGetClientRules_PersonalAgent() {
	svc, mockEntity, _, _, _ := suite.setupService()
	mockServerConfig := serverconfigmock.NewServerConfigServiceMock(suite.T())
	mockServerConfig.EXPECT().GetMergedConfig(mock.Anything, "personalAgent").
		Return(PersonalAgentConfig{Enabled: boolPtr(true), MaxAccessTokenValidity: 900}, nil)
	svc.serverConfigService = mockServerConfig

	agentEntity := withKind(buildAgentEntityFixture(testAgentName, "", "owner-1", ""),
		providers.AgentKindPersonalAssistant)
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

	rules, svcErr := svc.GetClientRules(context.Background(), testAgentID)
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(rules)
	assert.Equal(suite.T(), providers.ClientRules{
		RequiredSubjectID: "owner-1", Disabled: false, MaxAccessTokenValidity: 900,
	}, *rules)
}

// A failed or missing settings read falls back to the defaults: a personal agent is disabled, and its
// access tokens get the default cap.
func (suite *AgentServiceTestSuite) TestGetClientRules_SettingsFallBackToDisabled() {
	agentEntity := withKind(buildAgentEntityFixture(testAgentName, "", "owner-1", ""),
		providers.AgentKindPersonalAssistant)

	for name, merged := range map[string]struct {
		value any
		err   *tidcommon.ServiceError
	}{
		"read failure": {err: &tidcommon.InternalServerError},
		"wrong type":   {value: "not a config"},
		"no service":   {},
	} {
		suite.Run(name, func() {
			svc, mockEntity, _, _, _ := suite.setupService()
			if name != "no service" {
				mockServerConfig := serverconfigmock.NewServerConfigServiceMock(suite.T())
				mockServerConfig.EXPECT().GetMergedConfig(mock.Anything, "personalAgent").
					Return(merged.value, merged.err)
				svc.serverConfigService = mockServerConfig
			}
			clearMockCalls(mockEntity, "GetEntityProfile")
			mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

			rules, svcErr := svc.GetClientRules(context.Background(), testAgentID)
			suite.Require().Nil(svcErr)
			suite.Require().NotNil(rules)
			assert.True(suite.T(), rules.Disabled)
			assert.Equal(suite.T(), defaultPersonalAgentMaxAccessTokenValidity, rules.MaxAccessTokenValidity)
		})
	}
}

func (suite *AgentServiceTestSuite) TestGetClientRules_NotPersonal() {
	svc, mockEntity, _, _, _ := suite.setupService()

	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).
		Return(buildAgentEntityFixture(testAgentName, "", "owner-1", ""), nil)

	rules, svcErr := svc.GetClientRules(context.Background(), testAgentID)
	assert.Nil(suite.T(), svcErr)
	assert.Nil(suite.T(), rules)
}

// A personal agent without an owner must never read as an agent anyone may sign in to.
func (suite *AgentServiceTestSuite) TestGetClientRules_MissingOwnerIsServerError() {
	svc, mockEntity, _, _, _ := suite.setupService()

	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(withKind(
		buildAgentEntityFixture(testAgentName, "", "", ""), providers.AgentKindPersonalAssistant), nil)

	rules, svcErr := svc.GetClientRules(context.Background(), testAgentID)
	assert.Nil(suite.T(), rules)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestGetClientRules_Errors() {
	wrongCategory := buildAgentEntityFixture(testAgentName, "", "owner-1", "")
	wrongCategory.Category = providers.EntityCategoryUser

	testCases := []struct {
		name      string
		agentID   string
		entity    *providers.Entity
		entityErr error
		wantCode  string
	}{
		{"missing ID", "", nil, nil, ErrorMissingAgentID.Code},
		{"not found", testAgentID, nil, entity.ErrEntityNotFound, ErrorAgentNotFound.Code},
		{"not an agent", testAgentID, wrongCategory, nil, ErrorAgentNotFound.Code},
		{"store failure", testAgentID, nil, errors.New("db down"), tidcommon.InternalServerError.Code},
	}
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			svc, mockEntity, _, _, _ := suite.setupService()
			clearMockCalls(mockEntity, "GetEntityProfile")
			mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Maybe().Return(tc.entity, tc.entityErr)

			rules, svcErr := svc.GetClientRules(context.Background(), tc.agentID)
			assert.Nil(suite.T(), rules)
			suite.Require().NotNil(svcErr)
			assert.Equal(suite.T(), tc.wantCode, svcErr.Code)
		})
	}
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
		{"RefreshTokenRequiresTokenIssuingGrant", inboundclient.ErrOAuthRefreshTokenRequiresTokenIssuingGrant,
			ErrorInvalidOAuthConfiguration.Code,
			"error.agentservice.refresh_token_requires_token_issuing_grant_description"},
		{"PKCERequiresAuthCode", inboundclient.ErrOAuthPKCERequiresAuthCode,
			ErrorInvalidOAuthConfiguration.Code,
			"error.agentservice.pkce_requires_authorization_code_description"},
		{"ResponseTypesRequireAuthCode", inboundclient.ErrOAuthResponseTypesRequireAuthCode,
			ErrorInvalidOAuthConfiguration.Code,
			"error.agentservice.response_types_require_authorization_code_description"},
		{"IDJAGRequiresTokenExchange", inboundclient.ErrOAuthClientIDJAGRequiresTokenExchange,
			ErrorInvalidOAuthConfiguration.Code,
			"error.agentservice.idjag_requires_token_exchange_description"},
		{"IDJAGRequiresAllowedAudience", inboundclient.ErrOAuthClientIDJAGRequiresAllowedAudience,
			ErrorInvalidOAuthConfiguration.Code,
			"error.agentservice.idjag_requires_allowed_audience_description"},
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
		{"NoneAuthRequiresPublicClient", inboundclient.ErrOAuthNoneAuthRequiresPublicClient,
			ErrorInvalidOAuthConfiguration.Code,
			"error.agentservice.none_auth_method_requires_public_client_description"},
		{"NoneAuthCannotHaveSecret", inboundclient.ErrOAuthNoneAuthCannotHaveSecret,
			ErrorInvalidOAuthConfiguration.Code,
			"error.agentservice.none_auth_method_cannot_have_secret_description"},
		{"ClientCredentialsCannotUseNoneAuth", inboundclient.ErrOAuthClientCredentialsCannotUseNoneAuth,
			ErrorInvalidOAuthConfiguration.Code,
			"error.agentservice.client_credentials_cannot_use_none_auth_description"},
		{"JWTBearerCannotUseNoneAuth", inboundclient.ErrOAuthClientJWTBearerCannotUseNoneAuth,
			ErrorInvalidOAuthConfiguration.Code,
			"error.agentservice.jwt_bearer_cannot_use_none_auth_description"},
		{"IDJAGCannotUseNoneAuth", inboundclient.ErrOAuthClientIDJAGCannotUseNoneAuth,
			ErrorInvalidOAuthConfiguration.Code,
			"error.agentservice.idjag_cannot_use_none_auth_description"},
		{"PublicClientMustUseNoneAuth", inboundclient.ErrOAuthPublicClientMustUseNoneAuth,
			ErrorInvalidPublicClientConfiguration.Code,
			"error.agentservice.public_client_must_use_none_auth_description"},
		{"PublicClientMustHavePKCE", inboundclient.ErrOAuthPublicClientMustHavePKCE,
			ErrorInvalidPublicClientConfiguration.Code,
			"error.agentservice.public_client_must_have_pkce_description"},
		{"InvalidBackchannelLogoutURI", inboundclient.ErrOAuthInvalidBackchannelLogoutURI,
			ErrorInvalidOAuthConfiguration.Code,
			"error.agentservice.invalid_backchannel_logout_uri_description"},
		{"BackchannelLogoutURIRequiresHTTPS", inboundclient.ErrOAuthBackchannelLogoutURIRequiresHTTPS,
			ErrorInvalidOAuthConfiguration.Code,
			"error.agentservice.backchannel_logout_uri_requires_https_description"},
		{"BackchannelLogoutURIPrivateHost", inboundclient.ErrOAuthBackchannelLogoutURIPrivateHost,
			ErrorInvalidOAuthConfiguration.Code,
			"error.agentservice.backchannel_logout_uri_private_host_description"},
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
		{"UnsupportedSigningAlg", inboundclient.ErrOAuthIDTokenUnsupportedSigningAlg,
			"error.agentservice.idtoken_unsupported_signing_alg_description"},
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
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

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
		Type: testAgentType,
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

func (suite *AgentServiceTestSuite) TestUpdateAgent_EntityStoreError() {
	svc, mockEntity, _, _, _ := suite.setupService()
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).
		Return((*providers.Entity)(nil), errors.New("db error"))

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Type: testAgentType,
	})
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_WrongCategory() {
	svc, mockEntity, _, _, _ := suite.setupService()

	wrongCatEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	wrongCatEntity.Category = providers.EntityCategoryUser
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(wrongCatEntity, nil)

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Type: testAgentType,
	})
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorAgentNotFound.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_IsReadOnly() {
	svc, mockEntity, _, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	agentEntity.IsReadOnly = true
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Type: testAgentType,
	})
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorCannotModifyDeclarativeResource.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_OUHandleResolution() {
	svc, mockEntity, _, mockOU, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture("old-name", "", "", "")
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockEntity, "UpdateEntity")
	mockEntity.On("UpdateEntity", mock.Anything, testAgentID, mock.Anything).
		Return(&providers.Entity{}, nil)

	newOUID := "new-ou-id"
	clearMockCalls(mockOU, "GetOrganizationUnitByPath")
	mockOU.On("GetOrganizationUnitByPath", mock.Anything, "new-handle").
		Return(oupkg.OrganizationUnit{ID: newOUID}, (*tidcommon.ServiceError)(nil))

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Type: testAgentType, OUHandle: "new-handle",
	})
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Equal(suite.T(), newOUID, resp.OUID)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_ExplicitOUIDChanged() {
	svc, mockEntity, _, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockEntity, "UpdateEntity")
	mockEntity.On("UpdateEntity", mock.Anything, testAgentID, mock.Anything).
		Return(&providers.Entity{}, nil)

	newOUID := "different-ou-id"
	// Default IsOrganizationUnitExists mock returns true for any ID.

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Type: testAgentType, OUID: newOUID,
	})
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Equal(suite.T(), newOUID, resp.OUID)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_WantsInbound_NoExisting_CreatesInbound() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture("old-name", "", "", "")
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

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
		Type:                  testAgentType,
		InboundAuthProfileReq: inboundmodel.InboundAuthProfileReq{AuthFlowID: "new-flow-id"},
	})
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Equal(suite.T(), "new-flow-id", resp.AuthFlowID)
	mockInbound.AssertCalled(suite.T(), "CreateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// captureUpdatedOAuthProfile stubs the entity read and update and the inbound client creation, and
// returns a pointer to the OAuth profile the service hands the inbound client service.
func (suite *AgentServiceTestSuite) captureUpdatedOAuthProfile(
	mockEntity *entitymock.EntityServiceInterfaceMock,
	mockInbound *inboundclientmock.InboundClientServiceInterfaceMock,
) **providers.OAuthProfile {
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).
		Return(buildAgentEntityFixture("old-name", "", "", ""), nil)
	clearMockCalls(mockEntity, "UpdateEntity")
	mockEntity.On("UpdateEntity", mock.Anything, testAgentID, mock.Anything).
		Return(&providers.Entity{}, nil)

	captured := new(*providers.OAuthProfile)
	clearMockCalls(mockInbound, "CreateInboundClient")
	mockInbound.On("CreateInboundClient", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { *captured = args.Get(2).(*providers.OAuthProfile) }).
		Return(nil)
	return captured
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_DelegatedGetsTheDefaultGrants() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()
	profile := suite.captureUpdatedOAuthProfile(mockEntity, mockInbound)

	_, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Type:      testAgentType,
		Delegated: true,
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type:        providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{RedirectURIs: []string{"https://app.example.com/cb"}},
			},
		},
	})

	suite.Require().Nil(svcErr)
	suite.Require().NotNil(*profile)
	assert.Equal(suite.T(), []string{"client_credentials", "authorization_code", "refresh_token"},
		(*profile).GrantTypes)
	assert.Equal(suite.T(), []string{"code"}, (*profile).ResponseTypes)
	assert.True(suite.T(), (*profile).PKCERequired)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_NotDelegatedAlwaysHoldsClientCredentials() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()
	profile := suite.captureUpdatedOAuthProfile(mockEntity, mockInbound)

	_, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Type: testAgentType,
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					GrantTypes: []providers.GrantType{providers.GrantTypeRefreshToken},
				},
			},
		},
	})

	suite.Require().Nil(svcErr)
	suite.Require().NotNil(*profile)
	assert.Equal(suite.T(), []string{"client_credentials", "refresh_token"}, (*profile).GrantTypes)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_DelegatedWithoutADelegationGrantIsRejectedBeforeAnyWrite() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).
		Return(buildAgentEntityFixture("old-name", "", "", ""), nil)

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Type:      testAgentType,
		Delegated: true,
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					GrantTypes: []providers.GrantType{providers.GrantTypeRefreshToken},
				},
			},
		},
	})

	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorMissingDelegationGrantType.Code, svcErr.Code)
	mockEntity.AssertNotCalled(suite.T(), "UpdateEntity", mock.Anything, mock.Anything, mock.Anything)
	mockInbound.AssertNotCalled(suite.T(), "CreateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	mockInbound.AssertNotCalled(suite.T(), "UpdateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_SetsLogoProperty() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture("old-name", "", "", "")
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockEntity, "UpdateEntity")
	mockEntity.On("UpdateEntity", mock.Anything, testAgentID, mock.Anything).
		Return(&providers.Entity{}, nil)

	var capturedClient *inboundmodel.InboundClient
	clearMockCalls(mockInbound, "CreateInboundClient")
	mockInbound.On("CreateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			capturedClient = args.Get(1).(*inboundmodel.InboundClient)
		}).Return(nil)

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Type:                  testAgentType,
		LogoURL:               testAgentLogo,
		InboundAuthProfileReq: inboundmodel.InboundAuthProfileReq{AuthFlowID: "new-flow-id"},
	})
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Equal(suite.T(), testAgentLogo, resp.LogoURL)
	suite.Require().NotNil(capturedClient)
	assert.Equal(suite.T(), testAgentLogo, capturedClient.Properties[propLogoURL])
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_PopulatesOUHandle_SkipsWhenOUIDEmpty() {
	svc, mockEntity, _, mockOU, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	agentEntity.OUID = ""
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockEntity, "UpdateEntity")
	mockEntity.On("UpdateEntity", mock.Anything, testAgentID, mock.Anything).
		Return(&providers.Entity{}, nil)

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Type: testAgentType,
	})
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Empty(suite.T(), resp.OUID)
	mockOU.AssertNotCalled(suite.T(), "GetOrganizationUnitHandlesByIDs")
}

// --- DeleteAgent additional paths ---

func (suite *AgentServiceTestSuite) TestDeleteAgent_EntityStoreError() {
	svc, mockEntity, _, _, _ := suite.setupService()
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).
		Return((*providers.Entity)(nil), errors.New("db error"))

	svcErr := svc.DeleteAgent(context.Background(), testAgentID)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestDeleteAgent_WrongCategory() {
	svc, mockEntity, _, _, _ := suite.setupService()

	wrongCatEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	wrongCatEntity.Category = providers.EntityCategoryUser
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(wrongCatEntity, nil)

	svcErr := svc.DeleteAgent(context.Background(), testAgentID)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorAgentNotFound.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestDeleteAgent_IsReadOnly() {
	svc, mockEntity, _, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	agentEntity.IsReadOnly = true
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

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
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

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
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).
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
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(wrongCatEntity, nil)

	resp, svcErr := svc.GetAgentGroups(context.Background(), testAgentID, 10, 0)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorAgentNotFound.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestGetAgentGroups_CountError() {
	svc, mockEntity, _, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

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
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

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
		Return(oupkg.OrganizationUnit{}, notFound)

	req := &providers.Agent{
		Type: testAgentType, OUHandle: "missing-handle",
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
		Return(oupkg.OrganizationUnit{}, internalErr)

	req := &providers.Agent{
		Type: testAgentType, OUHandle: "bad-handle",
	}
	_, _, _, svcErr := svc.ValidateAgent(context.Background(), req, "")
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

// --- resolveUpdateOUID via UpdateAgent ---

func (suite *AgentServiceTestSuite) TestUpdateAgent_OUHandleResolution_OUNotFound() {
	svc, mockEntity, _, mockOU, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

	notFound := &oupkg.ErrorOrganizationUnitNotFound
	clearMockCalls(mockOU, "GetOrganizationUnitByPath")
	mockOU.On("GetOrganizationUnitByPath", mock.Anything, "missing-handle").
		Return(oupkg.OrganizationUnit{}, notFound)

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Type: testAgentType, OUHandle: "missing-handle",
	})
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorOrganizationUnitNotFound.Code, svcErr.Code)
	assert.Nil(suite.T(), resp)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_OUHandleResolution_InternalError() {
	svc, mockEntity, _, mockOU, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

	internalErr := &tidcommon.ServiceError{Code: "SOME_OTHER_ERROR"}
	clearMockCalls(mockOU, "GetOrganizationUnitByPath")
	mockOU.On("GetOrganizationUnitByPath", mock.Anything, "bad-handle").
		Return(oupkg.OrganizationUnit{}, internalErr)

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Type: testAgentType, OUHandle: "bad-handle",
	})
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
	assert.Nil(suite.T(), resp)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_ExplicitOUIDChanged_ValidateOUFails() {
	svc, mockEntity, _, mockOU, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

	ouNotFound := &oupkg.ErrorOrganizationUnitNotFound
	clearMockCalls(mockOU, "IsOrganizationUnitExists")
	mockOU.On("IsOrganizationUnitExists", mock.Anything, "nonexistent-ou").
		Return(false, ouNotFound)

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Type: testAgentType, OUID: "nonexistent-ou",
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
	suite.withDisplayPaths(svc, "name")
	mockInbound.On("GetEntityIDsByReference", mock.Anything, resourcedependency.ResourceTypeTheme, "theme-1",
		serverconst.MaxCompositeStoreRecords, 0).
		Return([]string{"agent-1"}, 1, nil)

	attrs, _ := json.Marshal(map[string]interface{}{"name": "Agent One"})
	mockEntity.On("GetEntitiesByIDs", mock.Anything, []string{"agent-1"}).Return([]providers.Entity{
		{ID: "agent-1", Category: providers.EntityCategoryAgent, Type: testAgentType, Attributes: attrs},
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

// ownerQuery is the system attribute query that finds the agents a user owns.
func ownerQuery(ownerID string) entity.SystemAttributeQuery {
	return entity.SystemAttributeQuery{SystemAttributes: map[string]interface{}{"owner": ownerID}}
}

// Owner dependencies are resolved through the owner in the agents' system attributes.
func (suite *AgentServiceTestSuite) TestGetResourceDependencies_ByOwner_Success() {
	svc, mockEntity, _, _, _ := suite.setupService()
	suite.withDisplayPaths(svc, "name")

	ownedSysAttrs, _ := json.Marshal(map[string]interface{}{"owner": "user-1"})
	ownedAttrs, _ := json.Marshal(map[string]interface{}{"name": "Agent One"})
	mockEntity.On("GetEntityListBySystemAttributes", mock.Anything, providers.EntityCategoryAgent,
		ownerQuery("user-1"), serverconst.MaxCompositeStoreRecords, 0).
		Return([]providers.Entity{
			{ID: "agent-1", Category: providers.EntityCategoryAgent, Type: testAgentType,
				Attributes: ownedAttrs, SystemAttributes: ownedSysAttrs},
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
	mockEntity.On("GetEntityListBySystemAttributes", mock.Anything, providers.EntityCategoryAgent,
		ownerQuery("user-1"), serverconst.MaxCompositeStoreRecords, 0).
		Return([]providers.Entity{}, nil)

	result, err := svc.GetResourceDependencies(context.Background(), resourcedependency.ResourceTypeUser, "user-1")
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), result)
}

func (suite *AgentServiceTestSuite) TestGetResourceDependencies_ByOwner_Error() {
	svc, mockEntity, _, _, _ := suite.setupService()
	mockEntity.On("GetEntityListBySystemAttributes", mock.Anything, providers.EntityCategoryAgent,
		ownerQuery("user-1"), serverconst.MaxCompositeStoreRecords, 0).
		Return(nil, errors.New("store error"))

	result, err := svc.GetResourceDependencies(context.Background(), resourcedependency.ResourceTypeUser, "user-1")
	assert.Nil(suite.T(), result)
	assert.Error(suite.T(), err)
}

// --- error-branch coverage ---

func (suite *AgentServiceTestSuite) TestCreateAgent_EntityCreationFails_NonMappableError() {
	svc, mockEntity, _, _, _ := suite.setupService()

	clearMockCalls(mockEntity, "CreateEntity")
	mockEntity.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return((*providers.Entity)(nil), errors.New("db error"))

	req := &providers.Agent{Type: testAgentType, OUID: testOUID}
	resp, svcErr := svc.CreateAgent(context.Background(), req)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestCreateAgent_InboundCreationFails_NonTranslatableError() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

	createdEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "CreateEntity")
	mockEntity.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(createdEntity, nil)

	clearMockCalls(mockInbound, "CreateInboundClient")
	mockInbound.On("CreateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("inbound boom"))

	clearMockCalls(mockEntity, "DeleteEntity")
	mockEntity.On("DeleteEntity", mock.Anything, mock.Anything).Return(nil)

	req := &providers.Agent{
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
	mockEntity.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(createdEntity, nil)

	clearMockCalls(mockInbound, "CreateInboundClient")
	mockInbound.On("CreateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("inbound boom"))

	clearMockCalls(mockEntity, "DeleteEntity")
	mockEntity.On("DeleteEntity", mock.Anything, mock.Anything).
		Return(errors.New("compensation delete failed"))

	req := &providers.Agent{
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
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockInbound, "GetOAuthProfileByEntityID")
	mockInbound.On("GetOAuthProfileByEntityID", mock.Anything, testAgentID).
		Return((*providers.OAuthProfile)(nil), errors.New("db error"))

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Type: testAgentType,
	})
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_UpdateEntityFails_NonMappableError() {
	svc, mockEntity, _, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockEntity, "UpdateEntity")
	mockEntity.On("UpdateEntity", mock.Anything, testAgentID, mock.Anything).
		Return((*providers.Entity)(nil), errors.New("db error"))

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Type: testAgentType,
	})
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_UpdateSystemCredentialsFails() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture("old-name", "", "", "")
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

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
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockInbound, "GetInboundClientByEntityID")
	mockInbound.On("GetInboundClientByEntityID", mock.Anything, testAgentID).
		Return(&inboundmodel.InboundClient{ID: testAgentID}, nil)

	clearMockCalls(mockInbound, "UpdateInboundClient")
	mockInbound.On("UpdateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("update boom"))

	req := &model.UpdateAgentRequest{
		Type:                  testAgentType,
		InboundAuthProfileReq: inboundmodel.InboundAuthProfileReq{AuthFlowID: "flow-1"},
	}
	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, req)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_ReconcileCreateInboundFails_NonTranslatableError() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture("old-name", "", "", "")
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockInbound, "GetInboundClientByEntityID")
	mockInbound.On("GetInboundClientByEntityID", mock.Anything, testAgentID).
		Return((*inboundmodel.InboundClient)(nil), inboundclient.ErrInboundClientNotFound)

	clearMockCalls(mockInbound, "CreateInboundClient")
	mockInbound.On("CreateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("create boom"))

	req := &model.UpdateAgentRequest{
		Type:                  testAgentType,
		InboundAuthProfileReq: inboundmodel.InboundAuthProfileReq{AuthFlowID: "flow-1"},
	}
	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, req)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_ReconcileDeleteInboundFails_NonTranslatableError() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture("old-name", "", "", "")
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

	clearMockCalls(mockInbound, "GetInboundClientByEntityID")
	mockInbound.On("GetInboundClientByEntityID", mock.Anything, testAgentID).
		Return(&inboundmodel.InboundClient{ID: testAgentID}, nil)

	clearMockCalls(mockInbound, "DeleteInboundClient")
	mockInbound.On("DeleteInboundClient", mock.Anything, testAgentID).
		Return(errors.New("delete boom"))

	req := &model.UpdateAgentRequest{Type: testAgentType}
	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, req)
	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestDeleteAgent_DeleteInboundClientFails_NonTranslatableError() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()

	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

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
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

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

	req := &providers.Agent{Type: testAgentType, OUID: testOUID}
	_, _, _, svcErr := svc.ValidateAgent(context.Background(), req, "")
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestValidateAgent_InboundValidateFails_NonTranslatableError() {
	svc, _, mockInbound, _, _ := suite.setupService()

	clearMockCalls(mockInbound, "Validate")
	mockInbound.On("Validate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("validate boom"))

	req := &providers.Agent{
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
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Return(agentEntity, nil)

	svcErr := svc.DeleteAgent(context.Background(), testAgentID)

	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
	mockInbound.AssertNotCalled(suite.T(), "DeleteInboundClient", mock.Anything, mock.Anything)
}

// configsWithGrants builds a one-entry inbound auth config carrying the given grant types.
func configsWithGrants(grantTypes []providers.GrantType) []providers.InboundAuthConfigWithSecret {
	return []providers.InboundAuthConfigWithSecret{
		{
			Type:        providers.OAuthInboundAuthType,
			OAuthConfig: &providers.OAuthConfigWithSecret{GrantTypes: grantTypes},
		},
	}
}

// clientAttributesOf reads back the client-token attribute selection, or nil when unset.
func clientAttributesOf(configs []providers.InboundAuthConfigWithSecret) []string {
	token := configs[0].OAuthConfig.Token
	if token == nil || token.AccessToken == nil || token.AccessToken.ClientConfig == nil {
		return nil
	}
	return token.AccessToken.ClientConfig.Attributes
}

func (suite *AgentServiceTestSuite) TestSeedClientSubTypeAttribute_ClientCredentials() {
	configs := configsWithGrants([]providers.GrantType{providers.GrantTypeClientCredentials})

	seedClientSubTypeAttribute(configs)

	assert.Equal(suite.T(), []string{"sub_type"}, clientAttributesOf(configs))
}

// An agent with no grant types receives client_credentials by default, so it is seeded too.
func (suite *AgentServiceTestSuite) TestSeedClientSubTypeAttribute_EmptyGrantTypes() {
	configs := configsWithGrants(nil)

	seedClientSubTypeAttribute(configs)

	assert.Equal(suite.T(), []string{"sub_type"}, clientAttributesOf(configs))
}

// An agent that never receives a token for itself gets no selection.
func (suite *AgentServiceTestSuite) TestSeedClientSubTypeAttribute_SkipsNonClientGrants() {
	configs := configsWithGrants([]providers.GrantType{providers.GrantTypeAuthorizationCode})

	seedClientSubTypeAttribute(configs)

	assert.Nil(suite.T(), clientAttributesOf(configs))
}

// Seeding adds to the caller's selection rather than replacing it.
func (suite *AgentServiceTestSuite) TestSeedClientSubTypeAttribute_AppendsToCallerSelection() {
	configs := configsWithGrants([]providers.GrantType{providers.GrantTypeClientCredentials})
	configs[0].OAuthConfig.Token = &providers.OAuthTokenConfig{
		AccessToken: &providers.AccessTokenConfig{
			ClientConfig: &providers.AccessTokenSubConfig{Attributes: []string{"modelProvider"}},
		},
	}

	seedClientSubTypeAttribute(configs)

	assert.Equal(suite.T(), []string{"modelProvider", "sub_type"}, clientAttributesOf(configs))
}

// An agent with no OAuth inbound config has no client token to carry the claim.
func (suite *AgentServiceTestSuite) TestSeedClientSubTypeAttribute_NoOAuthConfig() {
	configs := []providers.InboundAuthConfigWithSecret{}

	seedClientSubTypeAttribute(configs)

	assert.Empty(suite.T(), configs)
}

// --- Authorization ---

// setupServiceWithAuthz wires a service with permissive entity/inbound/OU mocks and the given
// authorization mock, so authz-specific tests control only the authorization decision.
func (suite *AgentServiceTestSuite) setupServiceWithAuthz(
	authzMock *sysauthzmock.SystemAuthorizationServiceInterfaceMock,
) (*agentService, *entitymock.EntityServiceInterfaceMock) {
	svc, mockEntity, _, _, _ := suite.setupService()
	svc.authzService = authzMock
	return svc, mockEntity
}

// newDenyAuthz returns a mock that denies the given action and allows everything else, so a test
// can pin the denial to one specific check.
func (suite *AgentServiceTestSuite) newDenyAuthz(
	action security.Action, ouID string,
) *sysauthzmock.SystemAuthorizationServiceInterfaceMock {
	authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(suite.T())
	authzMock.On("IsActionAllowed", mock.Anything, action,
		&sysauthz.ActionContext{
			ResourceType: security.ResourceTypeAgent,
			OUID:         ouID,
			ResourceID:   testAgentID,
		}).Return(false, nil).Maybe()
	authzMock.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
		Return(true, nil).Maybe()
	return authzMock
}

func (suite *AgentServiceTestSuite) TestCreateAgent_AuthzDenied() {
	authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(suite.T())
	authzMock.On("IsActionAllowed", mock.Anything, security.ActionCreateAgent,
		&sysauthz.ActionContext{ResourceType: security.ResourceTypeAgent, OUID: testOUID}).
		Return(false, nil).Once()
	svc, mockEntity := suite.setupServiceWithAuthz(authzMock)

	resp, svcErr := svc.CreateAgent(context.Background(), &providers.Agent{
		Type: testAgentType, OUID: testOUID,
	})

	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	mockEntity.AssertNotCalled(suite.T(), "CreateEntity", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (suite *AgentServiceTestSuite) TestGetAgent_AuthzDenied() {
	svc, mockEntity := suite.setupServiceWithAuthz(
		suite.newDenyAuthz(security.ActionReadAgent, testOUID))

	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).
		Return(buildAgentEntityFixture(testAgentName, "", "", ""), nil)

	resp, svcErr := svc.GetAgent(context.Background(), testAgentID, false)

	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.ErrorUnauthorized.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_AuthzDeniedOnExistingOU() {
	svc, mockEntity := suite.setupServiceWithAuthz(
		suite.newDenyAuthz(security.ActionUpdateAgent, testOUID))

	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).
		Return(buildAgentEntityFixture(testAgentName, "", "", ""), nil)

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Type: testAgentType,
	})

	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	mockEntity.AssertNotCalled(suite.T(), "UpdateEntity", mock.Anything, mock.Anything, mock.Anything)
}

// A caller authorized on the agent's current OU must still be refused when moving the agent into an
// OU it does not control, and the agent must not be written.
func (suite *AgentServiceTestSuite) TestUpdateAgent_AuthzDeniedOnDestinationOU() {
	const destinationOUID = "ou-id-destination"

	svc, mockEntity := suite.setupServiceWithAuthz(
		suite.newDenyAuthz(security.ActionUpdateAgent, destinationOUID))

	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).
		Return(buildAgentEntityFixture(testAgentName, "", "", ""), nil)

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Type: testAgentType, OUID: destinationOUID,
	})

	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	mockEntity.AssertNotCalled(suite.T(), "UpdateEntity", mock.Anything, mock.Anything, mock.Anything)
}

func (suite *AgentServiceTestSuite) TestDeleteAgent_AuthzDenied() {
	svc, mockEntity := suite.setupServiceWithAuthz(
		suite.newDenyAuthz(security.ActionDeleteAgent, testOUID))

	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).
		Return(buildAgentEntityFixture(testAgentName, "", "", ""), nil)

	svcErr := svc.DeleteAgent(context.Background(), testAgentID)

	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	mockEntity.AssertNotCalled(suite.T(), "DeleteEntity", mock.Anything, mock.Anything)
}

func (suite *AgentServiceTestSuite) TestGetAgentGroups_AuthzDenied() {
	svc, mockEntity := suite.setupServiceWithAuthz(
		suite.newDenyAuthz(security.ActionReadAgent, testOUID))

	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).
		Return(buildAgentEntityFixture(testAgentName, "", "", ""), nil)

	resp, svcErr := svc.GetAgentGroups(context.Background(), testAgentID, 10, 0)

	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.ErrorUnauthorized.Code, svcErr.Code)
}

func (suite *AgentServiceTestSuite) TestGetAgentRoles_AuthzDenied() {
	svc, mockEntity := suite.setupServiceWithAuthz(
		suite.newDenyAuthz(security.ActionReadAgent, testOUID))

	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).
		Return(buildAgentEntityFixture(testAgentName, "", "", ""), nil)

	resp, svcErr := svc.GetAgentRoles(context.Background(), testAgentID, 10, 0)

	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.ErrorUnauthorized.Code, svcErr.Code)
}

// A restricted caller lists through the OU-filtered store query, not the unfiltered one.
func (suite *AgentServiceTestSuite) TestGetAgentList_FiltersByAccessibleOUs() {
	accessibleOUIDs := []string{testOUID}

	authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(suite.T())
	authzMock.On("GetAccessibleResources", mock.Anything, security.ActionListAgents,
		security.ResourceTypeOU).
		Return(&sysauthz.AccessibleResources{AllAllowed: false, IDs: accessibleOUIDs}, nil).Once()
	svc, mockEntity := suite.setupServiceWithAuthz(authzMock)

	clearMockCalls(mockEntity, "GetEntityListCountByOUIDs")
	mockEntity.On("GetEntityListCountByOUIDs", mock.Anything,
		providers.EntityCategoryAgent, accessibleOUIDs, mock.Anything).Return(1, nil)

	clearMockCalls(mockEntity, "GetEntityListByOUIDs")
	mockEntity.On("GetEntityListByOUIDs", mock.Anything, providers.EntityCategoryAgent,
		accessibleOUIDs, mock.Anything, mock.Anything, mock.Anything).
		Return([]providers.Entity{*buildAgentEntityFixture(testAgentName, "", "", "")}, nil)

	resp, svcErr := svc.GetAgentList(context.Background(), 10, 0, nil, false)

	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Equal(suite.T(), 1, resp.TotalResults)
	mockEntity.AssertNotCalled(suite.T(), "GetEntityList", mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything)
}

// A caller with no accessible OUs gets an empty page without querying the store.
func (suite *AgentServiceTestSuite) TestGetAgentList_NoAccessibleOUs() {
	authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(suite.T())
	authzMock.On("GetAccessibleResources", mock.Anything, security.ActionListAgents,
		security.ResourceTypeOU).
		Return(&sysauthz.AccessibleResources{AllAllowed: false, IDs: []string{}}, nil).Once()
	svc, mockEntity := suite.setupServiceWithAuthz(authzMock)

	resp, svcErr := svc.GetAgentList(context.Background(), 10, 0, nil, false)

	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp)
	assert.Equal(suite.T(), 0, resp.TotalResults)
	assert.Empty(suite.T(), resp.Agents)
	mockEntity.AssertNotCalled(suite.T(), "GetEntityListByOUIDs", mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// A filter on the kind or the owner is matched against the system attributes, with the rest of the filter on
// the schema attributes.
func (suite *AgentServiceTestSuite) TestGetAgentList_FilterOnSystemAttributes() {
	svc, mockEntity, _, _, _ := suite.setupService()
	filters := map[string]interface{}{fieldKind: "personal-assistant", "name": "x"}
	want := entity.SystemAttributeQuery{
		SystemAttributes: map[string]interface{}{fieldKind: "personal-assistant"},
		Attributes:       map[string]interface{}{"name": "x"},
	}
	mockEntity.On("GetEntityListCountBySystemAttributes", mock.Anything, providers.EntityCategoryAgent, want).
		Return(1, nil).Once()
	mockEntity.On("GetEntityListBySystemAttributes", mock.Anything, providers.EntityCategoryAgent, want, 10, 0).
		Return([]providers.Entity{*buildAgentEntityFixture(testAgentName, "", "", "")}, nil).Once()

	resp, svcErr := svc.GetAgentList(context.Background(), 10, 0, filters, false)

	suite.Require().Nil(svcErr)
	assert.Equal(suite.T(), 1, resp.TotalResults)
	mockEntity.AssertNotCalled(suite.T(), "GetEntityList", mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything)
}

// The system attribute lookup keeps the caller's OU scope.
func (suite *AgentServiceTestSuite) TestGetAgentList_FilterOnSystemAttributesInAccessibleOUs() {
	accessibleOUIDs := []string{testOUID}
	authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(suite.T())
	authzMock.On("GetAccessibleResources", mock.Anything, security.ActionListAgents,
		security.ResourceTypeOU).
		Return(&sysauthz.AccessibleResources{AllAllowed: false, IDs: accessibleOUIDs}, nil).Once()
	svc, mockEntity := suite.setupServiceWithAuthz(authzMock)
	want := entity.SystemAttributeQuery{
		SystemAttributes: map[string]interface{}{fieldOwner: "user-1"},
		Attributes:       map[string]interface{}{},
	}
	mockEntity.On("GetEntityListCountBySystemAttributesAndOUIDs", mock.Anything, providers.EntityCategoryAgent,
		want, accessibleOUIDs).Return(1, nil).Once()
	mockEntity.On("GetEntityListBySystemAttributesAndOUIDs", mock.Anything, providers.EntityCategoryAgent,
		want, accessibleOUIDs, 10, 0).
		Return([]providers.Entity{*buildAgentEntityFixture(testAgentName, "", "user-1", "")}, nil).Once()

	resp, svcErr := svc.GetAgentList(context.Background(), 10, 0,
		map[string]interface{}{fieldOwner: "user-1"}, false)

	suite.Require().Nil(svcErr)
	assert.Equal(suite.T(), 1, resp.TotalResults)
	mockEntity.AssertNotCalled(suite.T(), "GetEntityListByOUIDs", mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (suite *AgentServiceTestSuite) TestSystemAttributeQuery() {
	assert.Nil(suite.T(), systemAttributeQuery(nil))
	assert.Nil(suite.T(), systemAttributeQuery(map[string]interface{}{"name": "x"}),
		"a filter on schema attributes only keeps the existing lookup")
	assert.Equal(suite.T(), &entity.SystemAttributeQuery{
		SystemAttributes: map[string]interface{}{fieldKind: "personal-assistant", fieldOwner: "user-1"},
		Attributes:       map[string]interface{}{"name": "x"},
	}, systemAttributeQuery(map[string]interface{}{
		fieldKind: "personal-assistant", fieldOwner: "user-1", "name": "x",
	}))
}

// A failed authorization check is a refusal, not a pass.
func (suite *AgentServiceTestSuite) TestGetAgent_AuthzCheckError() {
	authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(suite.T())
	authzMock.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
		Return(false, &tidcommon.InternalServerError).Once()
	svc, mockEntity := suite.setupServiceWithAuthz(authzMock)

	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).
		Return(buildAgentEntityFixture(testAgentName, "", "", ""), nil)

	resp, svcErr := svc.GetAgent(context.Background(), testAgentID, false)

	assert.Nil(suite.T(), resp)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

// withDisplayPaths gives the service an entity type service reporting the given display attribute for
// the test agent type; an empty path models a schema that has no display attribute.
func (suite *AgentServiceTestSuite) withDisplayPaths(svc *agentService, path string) {
	mockType := entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	mockType.On("GetDisplayAttributesByHandles", mock.Anything, entitytype.TypeCategoryAgent,
		mock.Anything).Return(map[string]string{testAgentType: path}, (*tidcommon.ServiceError)(nil)).Maybe()
	svc.entityTypeService = mockType
}

func (suite *AgentServiceTestSuite) TestGetAgent_IncludeDisplay_ResolvesDisplayAttribute() {
	svc, mockEntity, _, _, _ := suite.setupService()
	suite.withDisplayPaths(svc, "name")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).
		Return(buildAgentEntityFixture(testAgentName, "", "", ""), nil)

	resp, svcErr := svc.GetAgent(context.Background(), testAgentID, true)

	suite.Require().Nil(svcErr)
	assert.Equal(suite.T(), testAgentName, resp.Display)
}

func (suite *AgentServiceTestSuite) TestGetAgent_IncludeDisplay_FallsBackToIDWithoutDisplayAttribute() {
	svc, mockEntity, _, _, _ := suite.setupService()
	suite.withDisplayPaths(svc, "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).
		Return(buildAgentEntityFixture(testAgentName, "", "", ""), nil)

	resp, svcErr := svc.GetAgent(context.Background(), testAgentID, true)

	suite.Require().Nil(svcErr)
	assert.Equal(suite.T(), testAgentID, resp.Display)
}

func (suite *AgentServiceTestSuite) TestGetAgent_WithoutIncludeDisplay_LeavesDisplayEmpty() {
	svc, mockEntity, _, _, _ := suite.setupService()
	suite.withDisplayPaths(svc, "name")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).
		Return(buildAgentEntityFixture(testAgentName, "", "", ""), nil)

	resp, svcErr := svc.GetAgent(context.Background(), testAgentID, false)

	suite.Require().Nil(svcErr)
	assert.Empty(suite.T(), resp.Display)
}

func (suite *AgentServiceTestSuite) TestGetAgentList_IncludeDisplay_ResolvesDisplayAttribute() {
	svc, mockEntity, _, _, _ := suite.setupService()
	suite.withDisplayPaths(svc, "name")
	named := *buildAgentEntityFixture(testAgentName, "", "", "")
	unnamed := *buildAgentEntityFixture("", "", "", "")
	unnamed.ID = "agent-without-name"
	clearMockCalls(mockEntity, "GetEntityList")
	mockEntity.On("GetEntityList", mock.Anything, providers.EntityCategoryAgent, mock.Anything, mock.Anything,
		mock.Anything).Return([]providers.Entity{named, unnamed}, nil)
	clearMockCalls(mockEntity, "GetEntityListCount")
	mockEntity.On("GetEntityListCount", mock.Anything, providers.EntityCategoryAgent, mock.Anything).
		Return(2, nil)

	resp, svcErr := svc.GetAgentList(context.Background(), 10, 0, nil, true)

	suite.Require().Nil(svcErr)
	suite.Require().Len(resp.Agents, 2)
	assert.Equal(suite.T(), testAgentName, resp.Agents[0].Display)
	assert.Equal(suite.T(), "agent-without-name", resp.Agents[1].Display, "falls back to the ID")
}

func (suite *AgentServiceTestSuite) TestGetAgent_ReportsTheGovernedStatus() {
	svc, mockEntity, _, _, _ := suite.setupService()
	agentEntity := buildAgentEntityFixture(testAgentName, "", "", "")
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntity, nil)

	governance := identitygovernancemock.NewIdentityGovernanceServiceInterfaceMock(suite.T())
	governance.EXPECT().ReadEntityAccess(mock.Anything, *agentEntity).
		Return(identitygovernance.AccessRead{
			Status:      providers.AccessStatusReport{Value: providers.AccessStatusLocked},
			LastLoginAt: "2026-09-09T18:22:04Z",
		}).Once()
	svc.governance = governance

	resp, svcErr := svc.GetAgent(context.Background(), testAgentID, false)
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(resp.Status)
	suite.Equal(providers.AccessStatusLocked, resp.Status.Value)
	suite.Equal("2026-09-09T18:22:04Z", resp.LastLoginAt)
}

func (suite *AgentServiceTestSuite) TestGetAgentList_ReportsTheGovernedStatus() {
	svc, mockEntity, _, _, _ := suite.setupService()
	first := buildAgentEntityFixture(testAgentName, "", "", "")
	second := buildAgentEntityFixture("second-agent", "", "", "")
	second.ID = "agent-2"
	listed := []providers.Entity{*first, *second}
	clearMockCalls(mockEntity, "GetEntityList")
	mockEntity.On("GetEntityList", mock.Anything, providers.EntityCategoryAgent, 30, 0, mock.Anything).
		Return(listed, nil)
	clearMockCalls(mockEntity, "GetEntityListCount")
	mockEntity.On("GetEntityListCount", mock.Anything, providers.EntityCategoryAgent, mock.Anything).
		Return(2, nil)
	governance := identitygovernancemock.NewIdentityGovernanceServiceInterfaceMock(suite.T())
	governance.EXPECT().ReadAccess(mock.Anything, listed).Return([]identitygovernance.AccessRead{
		{
			Status:      providers.AccessStatusReport{Value: providers.AccessStatusLocked},
			LastLoginAt: "2026-09-09T18:22:04Z",
		},
		{Status: providers.AccessStatusReport{Value: providers.AccessStatusSuspended}},
	}).Once()
	svc.governance = governance

	resp, svcErr := svc.GetAgentList(context.Background(), 0, 0, nil, false)

	suite.Require().Nil(svcErr)
	suite.Require().Len(resp.Agents, 2)
	suite.Equal(testAgentID, resp.Agents[0].ID)
	suite.Equal(providers.AccessStatusLocked, resp.Agents[0].Status.Value)
	suite.Equal("2026-09-09T18:22:04Z", resp.Agents[0].LastLoginAt)
	suite.Equal("agent-2", resp.Agents[1].ID)
	suite.Equal(providers.AccessStatusSuspended, resp.Agents[1].Status.Value)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_ReportsTheGovernedStatus() {
	cases := []struct {
		name string
		read identitygovernance.AccessRead
	}{
		{name: "a held agent", read: identitygovernance.AccessRead{
			Status: providers.AccessStatusReport{Value: providers.AccessStatusLocked,
				Details: &providers.AccessStatusDetails{Reason: providers.AccessHoldReasonFailedAttempts,
					LockedMethods: []providers.AccessLockedMethod{
						{Method: "credential", ExpiresAt: "2026-10-21T09:00:00Z"}}}},
			LastLoginAt: "2026-10-20T08:00:00Z",
		}},
		{name: "an active agent", read: identitygovernance.AccessRead{
			Status: providers.AccessStatusReport{Value: providers.AccessStatusActive}}},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			svc, mockEntity, _, _, _ := suite.setupService()
			clearMockCalls(mockEntity, "GetEntityProfile")
			mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).
				Return(buildAgentEntityFixture("old-name", "", "", ""), nil)
			updated := buildAgentEntityFixture(testAgentName, "", "", "")
			clearMockCalls(mockEntity, "UpdateEntity")
			mockEntity.On("UpdateEntity", mock.Anything, testAgentID, mock.Anything).Return(updated, nil)
			governance := identitygovernancemock.NewIdentityGovernanceServiceInterfaceMock(suite.T())
			governance.EXPECT().ReadEntityAccess(mock.Anything, *updated).
				Return(tc.read).Once()
			svc.governance = governance

			resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
				Type: testAgentType, Attributes: json.RawMessage(`{"name":"test-agent"}`),
			})

			suite.Require().Nil(svcErr)
			suite.Require().NotNil(resp.Status)
			suite.Equal(tc.read.Status, *resp.Status)
			suite.Equal(tc.read.LastLoginAt, resp.LastLoginAt)
		})
	}
}

func (suite *AgentServiceTestSuite) TestCreateAgent_ReportsNoStatus() {
	svc, mockEntity, _, _, _ := suite.setupService()
	clearMockCalls(mockEntity, "CreateEntity")
	mockEntity.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(buildAgentEntityFixture(testAgentName, "", "", ""), nil)
	svc.governance = identitygovernancemock.NewIdentityGovernanceServiceInterfaceMock(suite.T())

	resp, svcErr := svc.CreateAgent(context.Background(), &providers.Agent{Type: testAgentType, OUID: testOUID})

	suite.Require().Nil(svcErr)
	suite.Nil(resp.Status)
	suite.Empty(resp.LastLoginAt)
}
