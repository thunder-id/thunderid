/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package application

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/application/model"
	"github.com/thunder-id/thunderid/internal/cert"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/crypto/cryptomock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/i18n/mgtmock"
	"github.com/thunder-id/thunderid/tests/mocks/inboundclientmock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
)

const testServiceAppID = "app123"
const testClientID = "test-client-id"
const testOUID = "default-ou"
const testConflictingAppID = "app456"

type ServiceTestSuite struct {
	suite.Suite
}

func TestServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceTestSuite))
}

func (suite *ServiceTestSuite) TestBuildBasicApplicationResponse() {
	cfg := inboundmodel.InboundClient{
		ID:                        "app-123",
		AuthFlowID:                "auth_flow_1",
		RegistrationFlowID:        "reg_flow_1",
		IsRegistrationFlowEnabled: true,
	}
	sysAttrs, _ := json.Marshal(map[string]interface{}{
		"name":        "Test App",
		"description": "Test Description",
		"clientId":    "client-123",
	})
	entity := &providers.Entity{SystemAttributes: sysAttrs}

	result := buildBasicApplicationResponse(cfg, entity)

	assert.Equal(suite.T(), "app-123", result.ID)
	assert.Equal(suite.T(), "Test App", result.Name)
	assert.Equal(suite.T(), "Test Description", result.Description)
	assert.Equal(suite.T(), "auth_flow_1", result.AuthFlowID)
	assert.Equal(suite.T(), "reg_flow_1", result.RegistrationFlowID)
	assert.True(suite.T(), result.IsRegistrationFlowEnabled)
	assert.Equal(suite.T(), "client-123", result.ClientID)
}

func (suite *ServiceTestSuite) TestBuildBasicApplicationResponse_WithTemplate() {
	cfg := inboundmodel.InboundClient{
		ID:                        "app-123",
		AuthFlowID:                "auth_flow_1",
		RegistrationFlowID:        "reg_flow_1",
		IsRegistrationFlowEnabled: true,
		ThemeID:                   "theme-123",
		LayoutID:                  "layout-456",
		Properties: map[string]interface{}{
			"template": "spa",
			"logo_url": "https://example.com/logo.png",
		},
	}
	sysAttrs, _ := json.Marshal(map[string]interface{}{
		"name":     "Test App",
		"clientId": "client-123",
	})
	entity := &providers.Entity{SystemAttributes: sysAttrs}

	result := buildBasicApplicationResponse(cfg, entity)

	assert.Equal(suite.T(), "app-123", result.ID)
	assert.Equal(suite.T(), "Test App", result.Name)
	assert.Equal(suite.T(), "theme-123", result.ThemeID)
	assert.Equal(suite.T(), "layout-456", result.LayoutID)
	assert.Equal(suite.T(), "spa", result.Template)
	assert.Equal(suite.T(), "client-123", result.ClientID)
	assert.Equal(suite.T(), "https://example.com/logo.png", result.LogoURL)
}

func (suite *ServiceTestSuite) TestBuildBasicApplicationResponse_WithEmptyTemplate() {
	cfg := inboundmodel.InboundClient{
		ID:                        "app-123",
		AuthFlowID:                "auth_flow_1",
		RegistrationFlowID:        "reg_flow_1",
		IsRegistrationFlowEnabled: true,
	}
	sysAttrs, _ := json.Marshal(map[string]interface{}{
		"name":     "Test App",
		"clientId": "client-123",
	})
	entity := &providers.Entity{SystemAttributes: sysAttrs}

	result := buildBasicApplicationResponse(cfg, entity)

	assert.Equal(suite.T(), "app-123", result.ID)
	assert.Equal(suite.T(), "", result.Template)
}

// setupTestService wires a service with permissive entity-provider / OU mocks and a
// no-op transactioner. Returns the service plus the inbound-client mock
// that tests typically need to extend.
func (suite *ServiceTestSuite) setupTestService() (
	*applicationService,
	*inboundclientmock.InboundClientServiceInterfaceMock,
) {
	mockStore := inboundclientmock.NewInboundClientServiceInterfaceMock(suite.T())
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(suite.T())
	epNotFound := entityprovider.NewEntityProviderError(
		entityprovider.ErrorCodeEntityNotFound, "not found", "")
	var noEPErr *entityprovider.EntityProviderError
	mockEntityProvider.On("IdentifyEntity", mock.Anything).Maybe().Return((*string)(nil), epNotFound)
	mockEntityProvider.On("GetEntity", mock.Anything).Maybe().Return((*providers.Entity)(nil), epNotFound)
	mockEntityProvider.On("GetEntitiesByIDs", mock.Anything).Maybe().Return([]providers.Entity{}, noEPErr)
	mockEntityProvider.On("CreateEntity", mock.Anything, mock.Anything).
		Maybe().Return(&providers.Entity{}, noEPErr)
	mockEntityProvider.On("DeleteEntity", mock.Anything).Maybe().Return(noEPErr)
	mockEntityProvider.On("UpdateSystemAttributes", mock.Anything, mock.Anything).Maybe().Return(noEPErr)
	mockEntityProvider.On("UpdateSystemCredentials", mock.Anything, mock.Anything).Maybe().Return(noEPErr)
	mockStore.On("Validate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe().Return(nil)
	mockStore.On("ResolveInboundAuthProfileHandles", mock.Anything, mock.Anything).Maybe().Return(nil)
	mockOUService := oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())
	mockOUService.On("IsOrganizationUnitExists", mock.Anything, mock.Anything).Maybe().Return(true, nil)
	service := &applicationService{
		logger:               log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ApplicationService")),
		inboundClientService: mockStore,
		entityProvider:       mockEntityProvider,
		ouService:            mockOUService,
		dependencyRegistry:   noopDepRegistry{},
	}
	return service, mockStore
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

// recordingDepRegistry is a resourcedependency.Registry that records CascadeDelete invocations so
// tests can assert dependents were removed. cascadeCalls is incremented on each CascadeDelete.
type recordingDepRegistry struct{ cascadeCalls *int }

func (recordingDepRegistry) RegisterProvider(resourcedependency.Provider) {}

func (recordingDepRegistry) GetDependencies(
	context.Context, string, string) (*resourcedependency.DependenciesResponse, error) {
	return &resourcedependency.DependenciesResponse{}, nil
}

func (r recordingDepRegistry) CascadeDelete(context.Context, string, string) (int, error) {
	*r.cascadeCalls++
	return 1, nil
}

func (recordingDepRegistry) ValidateReferenceUpdate(
	context.Context, string, string) *tidcommon.ServiceError {
	return nil
}

// resetIdentifyEntity removes broad IdentifyEntity expectations from the entity provider mock
// so a test can register a specific expectation without conflict.
func resetIdentifyEntity(service *applicationService) *entityprovidermock.EntityProviderInterfaceMock {
	return resetEntityProviderMethod(service, "IdentifyEntity")
}

// resetEntityProviderMethod removes any broad expectation for the named method on the
// entity provider mock attached to the service.
func resetEntityProviderMethod(
	service *applicationService, method string,
) *entityprovidermock.EntityProviderInterfaceMock {
	ep := service.entityProvider.(*entityprovidermock.EntityProviderInterfaceMock)
	var kept []*mock.Call
	for _, c := range ep.ExpectedCalls {
		if c.Method != method {
			kept = append(kept, c)
		}
	}
	ep.ExpectedCalls = kept
	return ep
}

// mockLoadFullApplication sets up the inbound-client + entity-provider mocks so that
// applicationService.getApplication(ctx, dto.ID) returns a result equivalent to the given
// ApplicationProcessedDTO. Builds the InboundClient (with Properties), OAuthProfile, and
// entity system attributes via the same helpers production code uses.
func mockLoadFullApplication(
	mockStore *inboundclientmock.InboundClientServiceInterfaceMock,
	service *applicationService,
	dto *model.ApplicationProcessedDTO,
) {
	inboundClient := toInboundClient(dto)
	mockStore.On("GetInboundClientByEntityID", mock.Anything, dto.ID).Return(&inboundClient, nil)

	var oauthProfile *providers.OAuthProfile
	if oauthProcessed := getOAuthInboundAuthConfigProcessedDTO(dto.InboundAuthConfig); oauthProcessed != nil {
		oauthProfile = buildOAuthProfileFromProcessed(*oauthProcessed)
	}
	if oauthProfile != nil {
		mockStore.On("GetOAuthProfileByEntityID", mock.Anything, dto.ID).Return(oauthProfile, nil)
	} else {
		mockStore.On("GetOAuthProfileByEntityID", mock.Anything, dto.ID).
			Return((*providers.OAuthProfile)(nil), inboundclient.ErrInboundClientNotFound)
	}

	sysAttrs := map[string]interface{}{}
	if dto.Name != "" {
		sysAttrs["name"] = dto.Name
	}
	if dto.Description != "" {
		sysAttrs["description"] = dto.Description
	}
	if oauthProcessed := getOAuthInboundAuthConfigProcessedDTO(dto.InboundAuthConfig); oauthProcessed != nil &&
		oauthProcessed.OAuthConfig != nil && oauthProcessed.OAuthConfig.ClientID != "" {
		sysAttrs["clientId"] = oauthProcessed.OAuthConfig.ClientID
	}
	sysAttrsJSON, _ := json.Marshal(sysAttrs)
	ep := resetEntityProviderMethod(service, "GetEntity")
	ep.On("GetEntity", dto.ID).Return(
		&providers.Entity{
			ID:               dto.ID,
			Category:         providers.EntityCategoryApp,
			OUID:             dto.OUID,
			SystemAttributes: sysAttrsJSON,
		},
		(*entityprovider.EntityProviderError)(nil),
	)
}

func (suite *ServiceTestSuite) TestGetOAuthApplication_EmptyClientID() {
	service, _ := suite.setupTestService()

	result, svcErr := service.GetOAuthApplication(context.Background(), "")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), svcErr)
}

func (suite *ServiceTestSuite) TestGetOAuthApplication_NotFound() {
	service, mockStore := suite.setupTestService()

	mockStore.EXPECT().GetOAuthClientByClientID(mock.Anything, "client123").Return(nil, nil)

	result, svcErr := service.GetOAuthApplication(context.Background(), "client123")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), svcErr)
}

func (suite *ServiceTestSuite) TestGetOAuthApplication_StoreError() {
	service, mockStore := suite.setupTestService()

	mockStore.EXPECT().GetOAuthClientByClientID(mock.Anything, "client123").
		Return(nil, errors.New("store error"))

	result, svcErr := service.GetOAuthApplication(context.Background(), "client123")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), svcErr)
}

func (suite *ServiceTestSuite) TestGetOAuthApplication_Success() {
	service, mockStore := suite.setupTestService()

	mockStore.EXPECT().GetOAuthClientByClientID(mock.Anything, "client123").
		Return(&providers.OAuthClient{ClientID: "client123", ID: "app123"}, nil)
	resetEntityProviderMethod(service, "GetEntity")
	service.entityProvider.(*entityprovidermock.EntityProviderInterfaceMock).
		On("GetEntity", "app123").Return(
		&providers.Entity{ID: "app123", Category: providers.EntityCategoryApp},
		(*entityprovider.EntityProviderError)(nil))

	result, svcErr := service.GetOAuthApplication(context.Background(), "client123")

	assert.NotNil(suite.T(), result)
	assert.Nil(suite.T(), svcErr)
	assert.Equal(suite.T(), "client123", result.ClientID)
}

// TestGetOAuthApplication_AgentEntity verifies GetOAuthApplication rejects an OAuth client
// whose owning entity is an agent (the OAuth client_id namespace is shared with agents).
func (suite *ServiceTestSuite) TestGetOAuthApplication_AgentEntity() {
	service, mockStore := suite.setupTestService()

	mockStore.EXPECT().GetOAuthClientByClientID(mock.Anything, "agent-client").
		Return(&providers.OAuthClient{ClientID: "agent-client", ID: "agent-id"}, nil)
	resetEntityProviderMethod(service, "GetEntity")
	service.entityProvider.(*entityprovidermock.EntityProviderInterfaceMock).
		On("GetEntity", "agent-id").Return(
		&providers.Entity{ID: "agent-id", Category: providers.EntityCategoryAgent},
		(*entityprovider.EntityProviderError)(nil))

	result, svcErr := service.GetOAuthApplication(context.Background(), "agent-client")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), ErrorApplicationNotFound.Code, svcErr.Code)
}

// TestGetOAuthApplication_EntityNotFound covers the path where the OAuth client exists but the
// owning entity has been deleted; GetOAuthApplication must surface ErrorApplicationNotFound.
func (suite *ServiceTestSuite) TestGetOAuthApplication_EntityNotFound() {
	service, mockStore := suite.setupTestService()

	mockStore.EXPECT().GetOAuthClientByClientID(mock.Anything, "client-x").
		Return(&providers.OAuthClient{ClientID: "client-x", ID: "missing-app"}, nil)

	result, svcErr := service.GetOAuthApplication(context.Background(), "client-x")

	assert.Nil(suite.T(), result)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorApplicationNotFound.Code, svcErr.Code)
}

// TestGetOAuthApplication_EntityLoadError covers the non-NotFound entity-provider error branch.
func (suite *ServiceTestSuite) TestGetOAuthApplication_EntityLoadError() {
	service, mockStore := suite.setupTestService()

	mockStore.EXPECT().GetOAuthClientByClientID(mock.Anything, "client-y").
		Return(&providers.OAuthClient{ClientID: "client-y", ID: "app-y"}, nil)
	ep := resetEntityProviderMethod(service, "GetEntity")
	ep.On("GetEntity", "app-y").Return(
		(*providers.Entity)(nil),
		entityprovider.NewEntityProviderError("INTERNAL_ERROR", "boom", ""))

	result, svcErr := service.GetOAuthApplication(context.Background(), "client-y")

	assert.Nil(suite.T(), result)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *ServiceTestSuite) TestGetApplication_EmptyAppID() {
	service, _ := suite.setupTestService()

	result, svcErr := service.GetApplication(context.Background(), "")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), svcErr)
}

func (suite *ServiceTestSuite) TestGetApplication_NotFound() {
	service, mockStore := suite.setupTestService()

	mockStore.On("GetInboundClientByEntityID", mock.Anything, testServiceAppID).
		Return(nil, model.ApplicationNotFoundError)

	result, svcErr := service.GetApplication(context.Background(), testServiceAppID)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), svcErr)
}

func (suite *ServiceTestSuite) TestGetApplication_StoreError() {
	service, mockStore := suite.setupTestService()

	mockStore.On("GetInboundClientByEntityID", mock.Anything, testServiceAppID).Return(nil, errors.New("store error"))

	result, svcErr := service.GetApplication(context.Background(), testServiceAppID)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), svcErr)
}

func (suite *ServiceTestSuite) TestGetApplication_Success() {
	service, mockStore := suite.setupTestService()

	app := &model.ApplicationProcessedDTO{
		ID:       testServiceAppID,
		Name:     "Test App",
		Metadata: map[string]interface{}{"service_key": "service_val"},
	}

	mockLoadFullApplication(mockStore, service, app)

	result, svcErr := service.GetApplication(context.Background(), testServiceAppID)

	assert.NotNil(suite.T(), result)
	assert.Nil(suite.T(), svcErr)
	assert.Equal(suite.T(), testServiceAppID, result.ID)
	assert.Equal(suite.T(), map[string]interface{}{"service_key": "service_val"}, result.Metadata)
}

// TestGetApplication_AgentEntity verifies getApplication rejects an entity that exists but is
// in the agent category — the application API must not leak agent records.
func (suite *ServiceTestSuite) TestGetApplication_AgentEntity() {
	service, mockStore := suite.setupTestService()

	mockStore.On("GetInboundClientByEntityID", mock.Anything, testServiceAppID).
		Return(&inboundmodel.InboundClient{ID: testServiceAppID}, nil)
	ep := resetEntityProviderMethod(service, "GetEntity")
	ep.On("GetEntity", testServiceAppID).Return(
		&providers.Entity{ID: testServiceAppID, Category: providers.EntityCategoryAgent},
		(*entityprovider.EntityProviderError)(nil))

	result, svcErr := service.GetApplication(context.Background(), testServiceAppID)

	assert.Nil(suite.T(), result)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorApplicationNotFound.Code, svcErr.Code)
}

func (suite *ServiceTestSuite) TestGetApplication_WithInboundAuthConfig_Success() {
	service, mockStore := suite.setupTestService()

	app := &model.ApplicationProcessedDTO{
		ID:          testServiceAppID,
		Name:        "OAuth Test App",
		Description: "App with OAuth config",
		InboundAuthConfig: []inboundmodel.InboundAuthConfigProcessed{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthClient{
					ClientID:                "client-id-123",
					RedirectURIs:            []string{"https://example.com/callback"},
					GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
					ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
					PKCERequired:            true,
					PublicClient:            false,
					Scopes:                  []string{"openid", "profile"},
				},
			},
		},
	}

	mockLoadFullApplication(mockStore, service, app)
	mockStore.EXPECT().GetCertificate(mock.Anything,
		cert.CertificateReferenceTypeOAuthApp, "client-id-123").Return(nil, nil)

	result, svcErr := service.GetApplication(context.Background(), testServiceAppID)

	assert.Nil(suite.T(), svcErr)
	require.NotNil(suite.T(), result)
	assert.Equal(suite.T(), testServiceAppID, result.ID)
	assert.Equal(suite.T(), "OAuth Test App", result.Name)

	require.Len(suite.T(), result.InboundAuthConfig, 1)
	inboundAuth := result.InboundAuthConfig[0]
	assert.Equal(suite.T(), providers.OAuthInboundAuthType, inboundAuth.Type)
	require.NotNil(suite.T(), inboundAuth.OAuthConfig)
	assert.Equal(suite.T(), "client-id-123", inboundAuth.OAuthConfig.ClientID)
	assert.Equal(suite.T(), []string{"https://example.com/callback"}, inboundAuth.OAuthConfig.RedirectURIs)
	assert.Equal(suite.T(), []providers.GrantType{providers.GrantTypeAuthorizationCode},
		inboundAuth.OAuthConfig.GrantTypes)
	assert.Equal(suite.T(), []providers.ResponseType{providers.ResponseTypeCode},
		inboundAuth.OAuthConfig.ResponseTypes)
	assert.Equal(suite.T(), providers.TokenEndpointAuthMethodClientSecretBasic,
		inboundAuth.OAuthConfig.TokenEndpointAuthMethod)
	assert.True(suite.T(), inboundAuth.OAuthConfig.PKCERequired)
	assert.False(suite.T(), inboundAuth.OAuthConfig.PublicClient)
	assert.Equal(suite.T(), []string{"openid", "profile"}, inboundAuth.OAuthConfig.Scopes)
	assert.Nil(suite.T(), inboundAuth.OAuthConfig.Certificate)
}

func (suite *ServiceTestSuite) TestGetApplicationList_Success() {
	service, mockStore := suite.setupTestService()

	sysAttrs1, _ := json.Marshal(map[string]interface{}{"name": "App 1"})
	sysAttrs2, _ := json.Marshal(map[string]interface{}{"name": "App 2"})
	entities := []providers.Entity{
		{ID: "app1", Category: providers.EntityCategoryApp, SystemAttributes: sysAttrs1},
		{ID: "app2", Category: providers.EntityCategoryApp, SystemAttributes: sysAttrs2},
	}
	cfg1 := inboundmodel.InboundClient{ID: "app1"}
	cfg2 := inboundmodel.InboundClient{ID: "app2"}

	ep := resetEntityProviderMethod(service, "GetEntityList")
	ep.On("GetEntityList", providers.EntityCategoryApp,
		mock.AnythingOfType("int"), mock.AnythingOfType("int"), mock.Anything).
		Return(entities, (*entityprovider.EntityProviderError)(nil))
	resetEntityProviderMethod(service, "GetEntityListCount").
		On("GetEntityListCount", providers.EntityCategoryApp, mock.Anything).
		Return(2, (*entityprovider.EntityProviderError)(nil))

	mockStore.On("GetInboundClientList", mock.Anything).
		Return([]inboundmodel.InboundClient{cfg1, cfg2}, nil)

	result, svcErr := service.GetApplicationList(context.Background())

	assert.NotNil(suite.T(), result)
	assert.Nil(suite.T(), svcErr)
	assert.Equal(suite.T(), 2, result.TotalResults)
	assert.Equal(suite.T(), 2, result.Count)
	assert.Len(suite.T(), result.Applications, 2)
}

func (suite *ServiceTestSuite) TestGetApplicationList_ListError() {
	service, _ := suite.setupTestService()

	resetEntityProviderMethod(service, "GetEntityListCount").
		On("GetEntityListCount", providers.EntityCategoryApp, mock.Anything).
		Return(0, (*entityprovider.EntityProviderError)(nil))
	ep := resetEntityProviderMethod(service, "GetEntityList")
	epErr := &entityprovider.EntityProviderError{Code: "INTERNAL_ERROR"}
	ep.On("GetEntityList", providers.EntityCategoryApp,
		mock.AnythingOfType("int"), mock.AnythingOfType("int"), mock.Anything).
		Return(([]providers.Entity)(nil), epErr)

	result, svcErr := service.GetApplicationList(context.Background())

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), svcErr)
}

func (suite *ServiceTestSuite) TestGetApplicationList_InboundFetchError() {
	service, mockStore := suite.setupTestService()

	entities := []providers.Entity{
		{ID: "app1", Category: providers.EntityCategoryApp},
	}
	resetEntityProviderMethod(service, "GetEntityListCount").
		On("GetEntityListCount", providers.EntityCategoryApp, mock.Anything).
		Return(1, (*entityprovider.EntityProviderError)(nil))
	ep := resetEntityProviderMethod(service, "GetEntityList")
	ep.On("GetEntityList", providers.EntityCategoryApp,
		mock.AnythingOfType("int"), mock.AnythingOfType("int"), mock.Anything).
		Return(entities, (*entityprovider.EntityProviderError)(nil))

	mockStore.On("GetInboundClientList", mock.Anything).
		Return(([]inboundmodel.InboundClient)(nil), errors.New("db error"))

	result, svcErr := service.GetApplicationList(context.Background())

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), svcErr)
}

func (suite *ServiceTestSuite) TestValidateApplication_NilApp() {
	service, _ := suite.setupTestService()

	result, inboundAuth, svcErr := service.ValidateApplication(context.Background(), nil)

	assert.Nil(suite.T(), result)
	assert.Nil(suite.T(), inboundAuth)
	assert.NotNil(suite.T(), svcErr)
}

func (suite *ServiceTestSuite) TestValidateApplication_EmptyName() {
	service, _ := suite.setupTestService()

	app := &model.ApplicationDTO{
		Name: "",
		OUID: testOUID,
	}

	result, inboundAuth, svcErr := service.ValidateApplication(context.Background(), app)

	assert.Nil(suite.T(), result)
	assert.Nil(suite.T(), inboundAuth)
	assert.NotNil(suite.T(), svcErr)
}

func (suite *ServiceTestSuite) TestValidateApplication_ExistingName() {
	service, _ := suite.setupTestService()

	app := &model.ApplicationDTO{
		Name: "Existing App",
		OUID: testOUID,
	}

	mockEP := resetIdentifyEntity(service)
	existingID := "existing-id"
	mockEP.On("IdentifyEntity",
		map[string]interface{}{"name": "Existing App"}).
		Return(
			&existingID, (*entityprovider.EntityProviderError)(nil))

	result, inboundAuth, svcErr := service.ValidateApplication(context.Background(), app)

	assert.Nil(suite.T(), result)
	assert.Nil(suite.T(), inboundAuth)
	assert.NotNil(suite.T(), svcErr)
}

func (suite *ServiceTestSuite) TestValidateApplicationForUpdate_EmptyAppID() {
	service, _ := suite.setupTestService()

	app := &model.ApplicationDTO{
		Name: "Test App",
		OUID: testOUID,
	}

	result, inboundAuth, svcErr := service.validateApplicationForUpdate(context.Background(), "", app)

	assert.Nil(suite.T(), result)
	assert.Nil(suite.T(), inboundAuth)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), &ErrorInvalidApplicationID, svcErr)
}

func (suite *ServiceTestSuite) TestValidateApplicationForUpdate_NilApp() {
	service, _ := suite.setupTestService()

	result, inboundAuth, svcErr := service.validateApplicationForUpdate(context.Background(), testServiceAppID, nil)

	assert.Nil(suite.T(), result)
	assert.Nil(suite.T(), inboundAuth)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), &ErrorApplicationNil, svcErr)
}

func (suite *ServiceTestSuite) TestValidateApplicationForUpdate_EmptyName() {
	service, _ := suite.setupTestService()

	app := &model.ApplicationDTO{
		Name: "",
		OUID: testOUID,
	}

	result, inboundAuth, svcErr := service.validateApplicationForUpdate(context.Background(), testServiceAppID, app)

	assert.Nil(suite.T(), result)
	assert.Nil(suite.T(), inboundAuth)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), &ErrorInvalidApplicationName, svcErr)
}

func (suite *ServiceTestSuite) TestValidateApplicationForUpdate_ApplicationNotFound() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()

	app := &model.ApplicationDTO{
		Name: "Test App",
		OUID: testOUID,
	}

	mockStore.On("IsDeclarative", mock.Anything, testServiceAppID).Maybe().Return(false)
	mockStore.On("GetInboundClientByEntityID", mock.Anything, testServiceAppID).
		Return(nil, inboundclient.ErrInboundClientNotFound)

	result, inboundAuth, svcErr := service.validateApplicationForUpdate(context.Background(), testServiceAppID, app)

	assert.Nil(suite.T(), result)
	assert.Nil(suite.T(), inboundAuth)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), &ErrorApplicationNotFound, svcErr)
}

func (suite *ServiceTestSuite) TestValidateApplicationForUpdate_ApplicationNilFromStore() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()

	app := &model.ApplicationDTO{
		Name: "Test App",
		OUID: testOUID,
	}

	mockStore.On("IsDeclarative", mock.Anything, testServiceAppID).Maybe().Return(false)
	mockStore.On("GetInboundClientByEntityID", mock.Anything, testServiceAppID).Return(nil, nil)

	result, inboundAuth, svcErr := service.validateApplicationForUpdate(context.Background(), testServiceAppID, app)

	assert.Nil(suite.T(), result)
	assert.Nil(suite.T(), inboundAuth)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), &ErrorApplicationNotFound, svcErr)
}

func (suite *ServiceTestSuite) TestValidateApplicationForUpdate_StoreError() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()

	app := &model.ApplicationDTO{
		Name: "Test App",
		OUID: testOUID,
	}

	mockStore.On("IsDeclarative", mock.Anything, testServiceAppID).Maybe().Return(false)
	mockStore.On("GetInboundClientByEntityID", mock.Anything, testServiceAppID).
		Return(nil, errors.New("database error"))

	result, inboundAuth, svcErr := service.validateApplicationForUpdate(context.Background(), testServiceAppID, app)

	assert.Nil(suite.T(), result)
	assert.Nil(suite.T(), inboundAuth)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), &tidcommon.InternalServerError, svcErr)
}

func (suite *ServiceTestSuite) TestValidateApplicationForUpdate_NameConflict() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()

	app := &model.ApplicationDTO{
		Name: "New Name",
		OUID: testOUID,
	}

	sysAttrs, _ := json.Marshal(map[string]interface{}{"name": "Old Name"})

	mockStore.On("IsDeclarative", mock.Anything, testServiceAppID).Maybe().Return(false)
	mockStore.On("GetInboundClientByEntityID", mock.Anything, testServiceAppID).
		Return(&inboundmodel.InboundClient{ID: testServiceAppID}, nil)
	mockStore.On("GetOAuthProfileByEntityID", mock.Anything, testServiceAppID).
		Return((*providers.OAuthProfile)(nil), nil)
	mockEP := resetIdentifyEntity(service)
	mockEP.On("GetEntity", testServiceAppID).Unset()
	mockEP.On("GetEntity", testServiceAppID).Return(
		&providers.Entity{
			ID:               testServiceAppID,
			Category:         providers.EntityCategoryApp,
			SystemAttributes: sysAttrs,
		}, (*entityprovider.EntityProviderError)(nil))
	conflictingID := testConflictingAppID
	mockEP.On("IdentifyEntity",
		map[string]interface{}{"name": "New Name"}).
		Return(
			&conflictingID, (*entityprovider.EntityProviderError)(nil))

	result, inboundAuth, svcErr := service.validateApplicationForUpdate(context.Background(), testServiceAppID, app)

	assert.Nil(suite.T(), result)
	assert.Nil(suite.T(), inboundAuth)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), &ErrorApplicationAlreadyExistsWithName, svcErr)
}

func (suite *ServiceTestSuite) TestValidateApplicationForUpdate_NameCheckStoreError() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()

	existingApp := &model.ApplicationProcessedDTO{
		ID:   testServiceAppID,
		Name: "Old Name",
	}

	app := &model.ApplicationDTO{
		Name: "New Name",
		OUID: testOUID,
	}

	mockStore.On("IsDeclarative", mock.Anything, testServiceAppID).Maybe().Return(false)
	mockLoadFullApplication(mockStore, service, existingApp)
	mockEP := resetIdentifyEntity(service)
	mockEP.On("IdentifyEntity",
		map[string]interface{}{"name": "New Name"}).
		Return((*string)(nil),
			entityprovider.NewEntityProviderError(
				entityprovider.ErrorCodeSystemError, "database error", ""))

	result, inboundAuth, svcErr := service.validateApplicationForUpdate(context.Background(), testServiceAppID, app)

	assert.Nil(suite.T(), result)
	assert.Nil(suite.T(), inboundAuth)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), &tidcommon.InternalServerError, svcErr)
}

func (suite *ServiceTestSuite) TestValidateApplicationForUpdate_Success() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()

	existingApp := &model.ApplicationProcessedDTO{
		ID:   testServiceAppID,
		Name: "Test App",
	}

	app := &model.ApplicationDTO{
		Name:    "Test App",
		OUID:    testOUID,
		URL:     "https://example.com",
		LogoURL: "https://example.com/logo.png",
	}

	mockStore.On("IsDeclarative", mock.Anything, testServiceAppID).Maybe().Return(false)
	mockLoadFullApplication(mockStore, service, existingApp)

	result, inboundAuth, svcErr := service.validateApplicationForUpdate(context.Background(), testServiceAppID, app)

	assert.NotNil(suite.T(), result)
	assert.Nil(suite.T(), inboundAuth)
	assert.Nil(suite.T(), svcErr)
	assert.Equal(suite.T(), testServiceAppID, result.ID)
	assert.Equal(suite.T(), "Test App", result.Name)
}

func (suite *ServiceTestSuite) TestDeleteApplication_EmptyAppID() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, _ := suite.setupTestService()

	svcErr := service.DeleteApplication(context.Background(), "")

	assert.NotNil(suite.T(), svcErr)
}

func (suite *ServiceTestSuite) TestDeleteApplication_NotFound() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()

	mockStore.On("DeleteInboundClient", mock.Anything, testServiceAppID).
		Return(inboundclient.ErrInboundClientNotFound)

	svcErr := service.DeleteApplication(context.Background(), testServiceAppID)

	// Should return nil (not error) when app not found
	assert.Nil(suite.T(), svcErr)
}

func (suite *ServiceTestSuite) TestDeleteApplication_StoreError() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()

	mockStore.On("DeleteInboundClient", mock.Anything, testServiceAppID).
		Return(errors.New("internal server error"))

	svcErr := service.DeleteApplication(context.Background(), testServiceAppID)

	assert.NotNil(suite.T(), svcErr)
}

func (suite *ServiceTestSuite) TestDeleteApplication_Success() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()

	mockStore.On("DeleteInboundClient", mock.Anything, testServiceAppID).Return(nil)

	svcErr := service.DeleteApplication(context.Background(), testServiceAppID)

	assert.Nil(suite.T(), svcErr)
}

// TestDeleteApplication_OAuthCertError verifies that when the inbound-client layer reports an
// internal error from a certificate operation, DeleteApplication surfaces an internal server error.
func (suite *ServiceTestSuite) TestDeleteApplication_OAuthCertError() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()

	mockStore.On("DeleteInboundClient", mock.Anything, testServiceAppID).
		Return(errors.New("internal server error"))

	svcErr := service.DeleteApplication(context.Background(), testServiceAppID)

	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), &tidcommon.InternalServerError, svcErr)
}

// TestDeleteApplication_OAuthCertError_ClientError verifies that when the inbound-client layer
// surfaces a cert operation client error, DeleteApplication maps it to ErrorCertificateClientError.
func (suite *ServiceTestSuite) TestDeleteApplication_OAuthCertError_ClientError() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()

	certOpErr := &inboundclient.CertOperationError{
		Operation: inboundclient.CertOpDelete,
		RefType:   cert.CertificateReferenceTypeOAuthApp,
		Underlying: &tidcommon.ServiceError{
			Type: tidcommon.ClientErrorType,
			ErrorDescription: tidcommon.I18nMessage{
				Key:          "error.test.invalid_client_id",
				DefaultValue: "Invalid client ID",
			},
		},
	}
	mockStore.On("DeleteInboundClient", mock.Anything, testServiceAppID).Return(certOpErr)

	svcErr := service.DeleteApplication(context.Background(), testServiceAppID)

	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), ErrorCertificateClientError.Code, svcErr.Code)
	assert.Contains(suite.T(), svcErr.ErrorDescription.DefaultValue, "Failed to delete OAuth app certificate")
}

// TestDeleteApplication_WithOAuthCert_Success verifies successful deletion of an application with OAuth certificate.
// This test covers deleteOAuthAppCertificate's success path (return nil).
func (suite *ServiceTestSuite) TestDeleteApplication_WithOAuthCert_Success() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()

	mockStore.On("DeleteInboundClient", mock.Anything, testServiceAppID).Return(nil)

	svcErr := service.DeleteApplication(context.Background(), testServiceAppID)

	assert.Nil(suite.T(), svcErr)
}

// TestDeleteApplication_AgentEntity verifies DeleteApplication refuses to delete when the
// targeted entity exists but is an agent — application delete must not affect agent records.
func (suite *ServiceTestSuite) TestDeleteApplication_AgentEntity() {
	testConfig := &config.Config{DeclarativeResources: config.DeclarativeResources{Enabled: false}}
	config.ResetServerRuntime()
	require.NoError(suite.T(), config.InitializeServerRuntime("/tmp/test", testConfig))
	defer config.ResetServerRuntime()

	service, _ := suite.setupTestService()
	ep := resetEntityProviderMethod(service, "GetEntity")
	ep.On("GetEntity", testServiceAppID).Return(
		&providers.Entity{ID: testServiceAppID, Category: providers.EntityCategoryAgent},
		(*entityprovider.EntityProviderError)(nil))

	svcErr := service.DeleteApplication(context.Background(), testServiceAppID)

	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorApplicationNotFound.Code, svcErr.Code)
}

// TestDeleteApplication_EntityLoadError covers the non-NotFound entity-provider error branch
// in the pre-delete category check.
func (suite *ServiceTestSuite) TestDeleteApplication_EntityLoadError() {
	testConfig := &config.Config{DeclarativeResources: config.DeclarativeResources{Enabled: false}}
	config.ResetServerRuntime()
	require.NoError(suite.T(), config.InitializeServerRuntime("/tmp/test", testConfig))
	defer config.ResetServerRuntime()

	service, _ := suite.setupTestService()
	ep := resetEntityProviderMethod(service, "GetEntity")
	ep.On("GetEntity", testServiceAppID).Return(
		(*providers.Entity)(nil),
		entityprovider.NewEntityProviderError("INTERNAL_ERROR", "boom", ""))

	svcErr := service.DeleteApplication(context.Background(), testServiceAppID)

	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *ServiceTestSuite) TestValidateOAuthParamsForCreateAndUpdate_EmptyInboundAuth() {
	app := &model.ApplicationDTO{
		Name: "Test App",
		OUID: testOUID,
	}

	result, svcErr := validateOAuthParamsForCreateAndUpdate(app)

	assert.Nil(suite.T(), result)
	assert.Nil(suite.T(), svcErr)
}

func (suite *ServiceTestSuite) TestValidateOAuthParamsForCreateAndUpdate_InvalidType() {
	app := &model.ApplicationDTO{
		Name: "Test App",
		OUID: testOUID,
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: "invalid_type",
			},
		},
	}

	result, svcErr := validateOAuthParamsForCreateAndUpdate(app)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), svcErr)
}

func (suite *ServiceTestSuite) TestValidateOAuthParamsForCreateAndUpdate_NilOAuthConfig() {
	app := &model.ApplicationDTO{
		Name: "Test App",
		OUID: testOUID,
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type:        providers.OAuthInboundAuthType,
				OAuthConfig: nil,
			},
		},
	}

	result, svcErr := validateOAuthParamsForCreateAndUpdate(app)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), svcErr)
}

func (suite *ServiceTestSuite) TestValidateOAuthParamsForCreateAndUpdate_WithDefaults() {
	config.ResetServerRuntime()
	require.NoError(suite.T(), config.InitializeServerRuntime("/tmp/test", &config.Config{}))
	defer config.ResetServerRuntime()

	app := &model.ApplicationDTO{
		Name: "Test App",
		OUID: testOUID,
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					RedirectURIs:            []string{"https://example.com/callback"},
					GrantTypes:              []providers.GrantType{},
					ResponseTypes:           []providers.ResponseType{},
					TokenEndpointAuthMethod: "",
				},
			},
		},
	}

	result, svcErr := validateOAuthParamsForCreateAndUpdate(app)

	assert.NotNil(suite.T(), result)
	assert.Nil(suite.T(), svcErr)
	assert.Len(suite.T(), result.OAuthConfig.GrantTypes, 1)
	assert.Equal(suite.T(), providers.GrantTypeAuthorizationCode, result.OAuthConfig.GrantTypes[0])
	assert.Equal(
		suite.T(),
		providers.TokenEndpointAuthMethodClientSecretBasic,
		result.OAuthConfig.TokenEndpointAuthMethod,
	)
}

func (suite *ServiceTestSuite) TestValidateOAuthParamsForCreateAndUpdate_WithResponseTypeDefault() {
	config.ResetServerRuntime()
	require.NoError(suite.T(), config.InitializeServerRuntime("/tmp/test", &config.Config{}))
	defer config.ResetServerRuntime()

	app := &model.ApplicationDTO{
		Name: "Test App",
		OUID: testOUID,
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					RedirectURIs:            []string{"https://example.com/callback"},
					GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
					ResponseTypes:           []providers.ResponseType{},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
				},
			},
		},
	}

	result, svcErr := validateOAuthParamsForCreateAndUpdate(app)

	assert.NotNil(suite.T(), result)
	assert.Nil(suite.T(), svcErr)
	assert.Len(suite.T(), result.OAuthConfig.ResponseTypes, 1)
	assert.Equal(suite.T(), providers.ResponseTypeCode, result.OAuthConfig.ResponseTypes[0])
}

func (suite *ServiceTestSuite) TestValidateOAuthParamsForCreateAndUpdate_WithGrantTypeButNoResponseType() {
	config.ResetServerRuntime()
	require.NoError(suite.T(), config.InitializeServerRuntime("/tmp/test", &config.Config{}))
	defer config.ResetServerRuntime()

	app := &model.ApplicationDTO{
		Name: "Test App",
		OUID: testOUID,
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					RedirectURIs:            []string{"https://example.com/callback"},
					GrantTypes:              []providers.GrantType{providers.GrantTypeClientCredentials},
					ResponseTypes:           []providers.ResponseType{},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
				},
			},
		},
	}

	result, svcErr := validateOAuthParamsForCreateAndUpdate(app)

	assert.NotNil(suite.T(), result)
	assert.Nil(suite.T(), svcErr)
	assert.Len(suite.T(), result.OAuthConfig.ResponseTypes, 0)
}

func (suite *ServiceTestSuite) TestEnrichApplicationWithCertificate_Error() {
	service, mockStore := suite.setupTestService()

	app := &providers.Application{
		ID:   testServiceAppID,
		Name: "Test App",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID: "client-id-123",
				},
			},
		},
	}

	svcErr := &tidcommon.ServiceError{
		Type:             tidcommon.ClientErrorType,
		ErrorDescription: tidcommon.I18nMessage{DefaultValue: "Invalid certificate"},
	}

	mockStore.EXPECT().
		GetCertificate(mock.Anything, cert.CertificateReferenceTypeOAuthApp, "client-id-123").
		Return(nil, &inboundclient.CertOperationError{
			Operation:  inboundclient.CertOpRetrieve,
			RefType:    cert.CertificateReferenceTypeOAuthApp,
			Underlying: svcErr,
		})

	result, err := service.enrichApplicationWithCertificate(context.Background(), app)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
}

func (suite *ServiceTestSuite) TestEnrichApplicationWithCertificate_Success() {
	service, mockStore := suite.setupTestService()

	app := &providers.Application{
		ID:   testServiceAppID,
		Name: "Test App",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID: "client-id-123",
				},
			},
		},
	}

	mockStore.EXPECT().
		GetCertificate(mock.Anything, cert.CertificateReferenceTypeOAuthApp, "client-id-123").
		Return(&inboundmodel.Certificate{
			Type:  cert.CertificateTypeJWKS,
			Value: `{"keys":[]}`,
		}, nil)

	result, err := service.enrichApplicationWithCertificate(context.Background(), app)

	assert.NotNil(suite.T(), result)
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), cert.CertificateTypeJWKS, result.InboundAuthConfig[0].OAuthConfig.Certificate.Type)
}

func (suite *ServiceTestSuite) TestValidateOAuthParamsForCreateAndUpdate_PublicClientSuccess() {
	config.ResetServerRuntime()
	require.NoError(suite.T(), config.InitializeServerRuntime("/tmp/test", &config.Config{}))
	defer config.ResetServerRuntime()

	app := &model.ApplicationDTO{
		Name: "Test App",
		OUID: testOUID,
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					RedirectURIs:            []string{"https://example.com/callback"},
					GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
					ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodNone,
					PublicClient:            true,
					PKCERequired:            true,
				},
			},
		},
	}

	result, svcErr := validateOAuthParamsForCreateAndUpdate(app)

	assert.NotNil(suite.T(), result)
	assert.Nil(suite.T(), svcErr)
	assert.True(suite.T(), result.OAuthConfig.PublicClient)
}

func (suite *ServiceTestSuite) TestValidateOAuthParamsForCreateAndUpdate_PublicClientNativeFlowRejected() {
	app := &model.ApplicationDTO{
		Name: "Test App",
		OUID: testOUID,
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					GrantTypes:              []providers.GrantType{providers.GrantTypeClientCredentials},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodNone,
					PublicClient:            true,
				},
			},
		},
	}

	result, svcErr := validateOAuthParamsForCreateAndUpdate(app)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), ErrorNativeFlowNotAllowedForSPA.Code, svcErr.Code)
}

func (suite *ServiceTestSuite) TestValidateApplication_StoreErrorNonNotFound() {
	service, _ := suite.setupTestService()

	app := &model.ApplicationDTO{
		Name: "Test App",
		OUID: testOUID,
	}

	// Return an entity provider error that's not EntityNotFound
	mockEP := resetIdentifyEntity(service)
	mockEP.On("IdentifyEntity",
		map[string]interface{}{"name": "Test App"}).
		Return((*string)(nil),
			entityprovider.NewEntityProviderError(
				entityprovider.ErrorCodeSystemError, "database connection error", ""))

	result, inboundAuth, svcErr := service.ValidateApplication(context.Background(), app)

	assert.Nil(suite.T(), result)
	assert.Nil(suite.T(), inboundAuth)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), &tidcommon.InternalServerError, svcErr)
}

//nolint:dupl // Testing different URL validation scenarios
func (suite *ServiceTestSuite) TestValidateApplication_InvalidURL() {
	testConfig := &config.Config{}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, _ := suite.setupTestService()

	app := &model.ApplicationDTO{
		Name: "Test App",
		OUID: testOUID,
		URL:  "not-a-valid-uri",
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID: "edc013d0-e893-4dc0-990c-3e1d203e005b",
		},
	}

	result, inboundAuth, svcErr := service.ValidateApplication(context.Background(), app)

	assert.Nil(suite.T(), result)
	assert.Nil(suite.T(), inboundAuth)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), &ErrorInvalidApplicationURL, svcErr)
}

func (suite *ServiceTestSuite) TestValidateApplication_AmbiguousAttestationConfig() {
	testConfig := &config.Config{}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, _ := suite.setupTestService()

	app := &model.ApplicationDTO{
		Name: "Test App",
		OUID: testOUID,
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID: "edc013d0-e893-4dc0-990c-3e1d203e005b",
			Attestation: &providers.AttestationConfig{
				Android: &providers.AndroidAttestationConfig{PackageName: "com.example.app"},
				Apple:   &providers.AppleAttestationConfig{TeamID: "TEAM123", BundleID: "com.example.app"},
			},
		},
	}

	result, inboundAuth, svcErr := service.ValidateApplication(context.Background(), app)

	assert.Nil(suite.T(), result)
	assert.Nil(suite.T(), inboundAuth)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), &ErrorAmbiguousAttestationConfig, svcErr)
}

func (suite *ServiceTestSuite) TestValidateApplication_InvalidURLs() {
	testConfig := &config.Config{}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, _ := suite.setupTestService()

	cases := []struct {
		name    string
		apply   func(*model.ApplicationDTO)
		wantErr *tidcommon.ServiceError
	}{
		{"InvalidLogoURL", func(a *model.ApplicationDTO) { a.LogoURL = "://invalid" }, &ErrorInvalidLogoURL},
		{"InvalidTosURI", func(a *model.ApplicationDTO) { a.TosURI = "not-a-valid-uri" }, &ErrorInvalidTosURI},
		{"InvalidPolicyURI", func(a *model.ApplicationDTO) { a.PolicyURI = "not-a-valid-uri" }, &ErrorInvalidPolicyURI},
	}

	for _, tc := range cases {
		suite.Run(tc.name, func() {
			app := &model.ApplicationDTO{
				Name: "Test App",
				OUID: testOUID,
				InboundAuthProfile: providers.InboundAuthProfile{
					AuthFlowID: "edc013d0-e893-4dc0-990c-3e1d203e005b",
				},
			}
			tc.apply(app)

			result, inboundAuth, svcErr := service.ValidateApplication(context.Background(), app)

			assert.Nil(suite.T(), result)
			assert.Nil(suite.T(), inboundAuth)
			assert.Equal(suite.T(), tc.wantErr, svcErr)
		})
	}
}

func (suite *ServiceTestSuite) TestCreateApplication_StoreErrorWithRollback() {
	suite.runCreateApplicationStoreErrorTest()
}

func (suite *ServiceTestSuite) TestCreateApplication_StoreErrorWithRollbackFailure() {
	// Currently identical to success case as rollback behavior is internal
	suite.runCreateApplicationStoreErrorTest()
}

func (suite *ServiceTestSuite) runCreateApplicationStoreErrorTest() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()

	app := &model.ApplicationDTO{
		Name: "Test App",
		OUID: testOUID,
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID:         "edc013d0-e893-4dc0-990c-3e1d203e005b",
			RegistrationFlowID: "80024fb3-29ed-4c33-aa48-8aee5e96d522",
		},
	}

	mockStore.On("CreateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("internal server error"))

	result, svcErr := service.CreateApplication(context.Background(), app)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), &tidcommon.InternalServerError, svcErr)
}

func (suite *ServiceTestSuite) TestUpdateApplication_StoreErrorNonNotFound() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()

	app := &model.ApplicationDTO{
		Name: "Updated App",
		OUID: testOUID,
	}

	mockStore.On("IsDeclarative", mock.Anything, testServiceAppID).Maybe().Return(false)
	// Return an error that's not ApplicationNotFoundError
	mockStore.On("GetInboundClientByEntityID", mock.Anything, testServiceAppID).
		Return(nil, errors.New("database connection error"))

	result, svcErr := service.UpdateApplication(context.Background(), testServiceAppID, app)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), &tidcommon.InternalServerError, svcErr)
}

func (suite *ServiceTestSuite) TestUpdateApplication_StoreErrorWhenCheckingName() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()

	existingApp := &model.ApplicationProcessedDTO{
		ID:   testServiceAppID,
		Name: "Old App",
	}

	app := &model.ApplicationDTO{
		Name: "New App",
		OUID: testOUID,
	}

	mockStore.On("IsDeclarative", mock.Anything, testServiceAppID).Maybe().Return(false)
	mockLoadFullApplication(mockStore, service, existingApp)
	// Return an entity provider error when checking name uniqueness
	mockEP := resetIdentifyEntity(service)
	mockEP.On("IdentifyEntity",
		map[string]interface{}{"name": "New App"}).
		Return((*string)(nil),
			entityprovider.NewEntityProviderError(
				entityprovider.ErrorCodeSystemError, "database connection error", ""))

	result, svcErr := service.UpdateApplication(context.Background(), testServiceAppID, app)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), &tidcommon.InternalServerError, svcErr)
}

func (suite *ServiceTestSuite) TestUpdateApplication_StoreErrorWhenCheckingClientID() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()

	existingApp := &model.ApplicationProcessedDTO{
		ID:   testServiceAppID,
		Name: "Test App",
		InboundAuthConfig: []inboundmodel.InboundAuthConfigProcessed{
			{
				OAuthConfig: &providers.OAuthClient{
					ClientID: "old-client-id",
				},
			},
		},
	}

	app := &model.ApplicationDTO{
		Name: "Test App",
		OUID: testOUID,
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:                "new-client-id",
					RedirectURIs:            []string{"https://example.com/callback"},
					GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
					ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
				},
			},
		},
	}

	mockStore.On("IsDeclarative", mock.Anything, testServiceAppID).Maybe().Return(false)
	mockLoadFullApplication(mockStore, service, existingApp)
	// Return an entity provider error when checking client ID uniqueness
	mockEP := resetIdentifyEntity(service)
	mockEP.On("IdentifyEntity",
		map[string]interface{}{"clientId": "new-client-id"}).
		Return((*string)(nil),
			entityprovider.NewEntityProviderError(
				entityprovider.ErrorCodeSystemError, "database connection error", ""))

	result, svcErr := service.UpdateApplication(context.Background(), testServiceAppID, app)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), &tidcommon.InternalServerError, svcErr)
}

func (suite *ServiceTestSuite) TestUpdateApplication_StoreErrorWithRollback() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()

	existingApp := &model.ApplicationProcessedDTO{
		ID:   testServiceAppID,
		Name: "Test App",
	}

	app := &model.ApplicationDTO{
		ID:   testServiceAppID,
		Name: "Test App",
		OUID: testOUID,
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID:         "edc013d0-e893-4dc0-990c-3e1d203e005b",
			RegistrationFlowID: "80024fb3-29ed-4c33-aa48-8aee5e96d522",
		},
	}

	mockStore.On("IsDeclarative", mock.Anything, testServiceAppID).Maybe().Return(false)
	mockLoadFullApplication(mockStore, service, existingApp)
	mockStore.On("UpdateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("internal server error"))

	result, svcErr := service.UpdateApplication(context.Background(), testServiceAppID, app)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), &tidcommon.InternalServerError, svcErr)
}

// A mobile application configured with Play Integrity attestation must have its service account
// credentials encrypted before persistence and stripped from the response.
func (suite *ServiceTestSuite) TestCreateApplication_WithAttestation_EncryptsAndStripsCredentials() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{Enabled: false},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()
	mockCrypto := cryptomock.NewRuntimeCryptoProviderMock(suite.T())
	service.cryptoSvc = mockCrypto

	const rawCreds = `{"type":"service_account"}`
	mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, []byte(rawCreds)).
		Return([]byte("encrypted-creds"), nil, nil)

	var persistedClient *inboundmodel.InboundClient
	mockStore.On("CreateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			persistedClient, _ = args.Get(1).(*inboundmodel.InboundClient)
		}).Return(nil)

	app := &model.ApplicationDTO{
		Name: "Mobile App",
		OUID: testOUID,
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID: "auth-flow-id",
			// Attestation is a client-level setting, configured at the top level of the application
			// regardless of protocol.
			Attestation: &providers.AttestationConfig{
				Android: &providers.AndroidAttestationConfig{
					PackageName:               "com.example.app",
					CertificateSha256Digests:  []string{"AA:BB"},
					ServiceAccountCredentials: rawCreds,
				},
			},
		},
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:                testClientID,
					RedirectURIs:            []string{"myapp://callback"},
					GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
					ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodNone,
					PublicClient:            true,
					PKCERequired:            true,
				},
			},
		},
	}

	result, svcErr := service.CreateApplication(context.Background(), app)
	require.Nil(suite.T(), svcErr)
	require.NotNil(suite.T(), result)

	// The persisted inbound client carries the encrypted credentials, never the plaintext.
	require.NotNil(suite.T(), persistedClient)
	require.NotNil(suite.T(), persistedClient.Attestation)
	require.NotNil(suite.T(), persistedClient.Attestation.Android)
	assert.Equal(suite.T(), "encrypted-creds", persistedClient.Attestation.Android.ServiceAccountCredentials)
	assert.Equal(suite.T(), "com.example.app", persistedClient.Attestation.Android.PackageName)

	// The response echoes package name and digests at the top level but never the credentials.
	require.NotNil(suite.T(), result.Attestation)
	require.NotNil(suite.T(), result.Attestation.Android)
	assert.Equal(suite.T(), "com.example.app", result.Attestation.Android.PackageName)
	assert.Empty(suite.T(), result.Attestation.Android.ServiceAccountCredentials)
}

// When credentials are omitted (as on an update that does not change them), the previously stored
// encrypted value is preserved rather than overwritten with an empty value.
func (suite *ServiceTestSuite) TestResolveAttestationCredentials_PreservesExistingWhenOmitted() {
	service, mockStore := suite.setupTestService()

	const appID = "app-1"
	mockStore.On("GetInboundClientByEntityID", mock.Anything, appID).Return(
		&inboundmodel.InboundClient{
			Attestation: &providers.AttestationConfig{
				Android: &providers.AndroidAttestationConfig{ServiceAccountCredentials: "stored-encrypted"},
			},
		}, nil)

	inboundClient := &inboundmodel.InboundClient{
		Attestation: &providers.AttestationConfig{
			Android: &providers.AndroidAttestationConfig{PackageName: "com.example.app"},
		},
	}

	svcErr := service.resolveAttestationCredentialsForPersist(context.Background(), appID, inboundClient)
	require.Nil(suite.T(), svcErr)
	assert.Equal(suite.T(), "stored-encrypted", inboundClient.Attestation.Android.ServiceAccountCredentials)
	assert.Equal(suite.T(), "com.example.app", inboundClient.Attestation.Android.PackageName)
}

// A non-"not found" lookup failure while preserving omitted credentials is propagated as an internal
// error, so a transient store failure cannot silently overwrite stored credentials with an empty
// value.
func (suite *ServiceTestSuite) TestResolveAttestationCredentials_LookupErrorPropagates() {
	service, mockStore := suite.setupTestService()

	const appID = "app-1"
	mockStore.On("GetInboundClientByEntityID", mock.Anything, appID).Return(
		(*inboundmodel.InboundClient)(nil), errors.New("database unavailable"))

	inboundClient := &inboundmodel.InboundClient{
		Attestation: &providers.AttestationConfig{
			Android: &providers.AndroidAttestationConfig{PackageName: "com.example.app"},
		},
	}

	svcErr := service.resolveAttestationCredentialsForPersist(context.Background(), appID, inboundClient)
	require.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *ServiceTestSuite) TestCreateApplication_ValidateApplicationError() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, _ := suite.setupTestService()

	app := &model.ApplicationDTO{
		Name: "", // Invalid name to trigger ValidateApplication error
		OUID: testOUID,
	}

	result, svcErr := service.CreateApplication(context.Background(), app)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), &ErrorInvalidApplicationName, svcErr)
}

func (suite *ServiceTestSuite) TestCreateApplication_WithOAuthCertificate_Success() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()

	app := &model.ApplicationDTO{
		Name: "Test OAuth Cert App",
		OUID: testOUID,
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID:         "auth-flow-id",
			RegistrationFlowID: "reg-flow-id",
		},
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:                testClientID,
					RedirectURIs:            []string{"https://example.com/callback"},
					GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
					ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodPrivateKeyJWT,
					Certificate: &inboundmodel.Certificate{
						Type:  "JWKS",
						Value: `{"keys":[]}`,
					},
				},
			},
		},
	}

	mockStore.On("CreateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	result, svcErr := service.CreateApplication(context.Background(), app)

	assert.NotNil(suite.T(), result)
	assert.Nil(suite.T(), svcErr)
	assert.Equal(suite.T(), "Test OAuth Cert App", result.Name)
	require.Len(suite.T(), result.InboundAuthConfig, 1)
	assert.Equal(suite.T(), providers.OAuthInboundAuthType, result.InboundAuthConfig[0].Type)
	require.NotNil(suite.T(), result.InboundAuthConfig[0].OAuthConfig)
	require.NotNil(suite.T(), result.InboundAuthConfig[0].OAuthConfig.Certificate)
	assert.Equal(suite.T(), cert.CertificateType("JWKS"), result.InboundAuthConfig[0].OAuthConfig.Certificate.Type)
	assert.Equal(suite.T(), `{"keys":[]}`, result.InboundAuthConfig[0].OAuthConfig.Certificate.Value)
}

func (suite *ServiceTestSuite) TestCreateApplication_IssuesFlowSecretForEmbeddedApp() {
	testConfig := &config.Config{DeclarativeResources: config.DeclarativeResources{Enabled: false}}
	config.ResetServerRuntime()
	require.NoError(suite.T(), config.InitializeServerRuntime("/tmp/test", testConfig))
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()

	// An embedded server-side app: no OAuth config, so no OAuth profile.
	app := &model.ApplicationDTO{
		Name: "Embedded App",
		OUID: testOUID,
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID: "auth-flow-id",
		},
	}

	var capturedCreds json.RawMessage
	ep := resetEntityProviderMethod(service, "CreateEntity")
	ep.On("CreateEntity", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			capturedCreds = args.Get(1).(json.RawMessage)
		}).
		Return(&providers.Entity{}, (*entityprovider.EntityProviderError)(nil))
	mockStore.On("CreateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	result, svcErr := service.CreateApplication(context.Background(), app)

	require.Nil(suite.T(), svcErr)
	require.NotNil(suite.T(), result)
	// The Flow Secret is surfaced once on creation.
	assert.NotEmpty(suite.T(), result.FlowSecret)
	// It is persisted to system credentials under the flowSecret key.
	require.NotNil(suite.T(), capturedCreds)
	var creds map[string]interface{}
	require.NoError(suite.T(), json.Unmarshal(capturedCreds, &creds))
	assert.Equal(suite.T(), result.FlowSecret, creds[fieldFlowSecret])
}

func (suite *ServiceTestSuite) TestCreateApplication_NoFlowSecretForM2MClient() {
	testConfig := &config.Config{DeclarativeResources: config.DeclarativeResources{Enabled: false}}
	config.ResetServerRuntime()
	require.NoError(suite.T(), config.InitializeServerRuntime("/tmp/test", testConfig))
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()

	// A machine-to-machine app using client_credentials only. It obtains tokens directly and cannot
	// consume a flow assertion, so it gets no Flow Secret. A caller-supplied FlowSecret is ignored.
	app := &model.ApplicationDTO{
		Name:       "M2M App",
		OUID:       testOUID,
		FlowSecret: "caller-supplied-secret",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:                testClientID,
					GrantTypes:              []providers.GrantType{providers.GrantTypeClientCredentials},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
					PublicClient:            false,
				},
			},
		},
	}

	var capturedCreds json.RawMessage
	ep := resetEntityProviderMethod(service, "CreateEntity")
	ep.On("CreateEntity", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			capturedCreds = args.Get(1).(json.RawMessage)
		}).
		Return(&providers.Entity{}, (*entityprovider.EntityProviderError)(nil))
	mockStore.On("CreateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	result, svcErr := service.CreateApplication(context.Background(), app)

	require.Nil(suite.T(), svcErr)
	require.NotNil(suite.T(), result)
	// M2M apps never receive a Flow Secret.
	assert.Empty(suite.T(), result.FlowSecret)
	if capturedCreds != nil {
		var creds map[string]interface{}
		require.NoError(suite.T(), json.Unmarshal(capturedCreds, &creds))
		_, hasFlowSecret := creds[fieldFlowSecret]
		assert.False(suite.T(), hasFlowSecret)
	}
}

func (suite *ServiceTestSuite) TestCreateApplication_NoFlowSecretForRedirectClient() {
	testConfig := &config.Config{DeclarativeResources: config.DeclarativeResources{Enabled: false}}
	config.ResetServerRuntime()
	require.NoError(suite.T(), config.InitializeServerRuntime("/tmp/test", testConfig))
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()

	// A confidential full-stack app using the redirect-based authorization_code flow. It gets a
	// client secret but no Flow Secret, since it cannot initiate flows directly. A caller-supplied
	// FlowSecret must be ignored for such an ineligible app.
	app := &model.ApplicationDTO{
		Name:       "Full-stack App",
		OUID:       testOUID,
		FlowSecret: "caller-supplied-secret",
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID: "auth-flow-id",
		},
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:                testClientID,
					RedirectURIs:            []string{"https://example.com/callback"},
					GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
					ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
					PublicClient:            false,
				},
			},
		},
	}

	var capturedCreds json.RawMessage
	ep := resetEntityProviderMethod(service, "CreateEntity")
	ep.On("CreateEntity", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			capturedCreds = args.Get(1).(json.RawMessage)
		}).
		Return(&providers.Entity{}, (*entityprovider.EntityProviderError)(nil))
	mockStore.On("CreateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	result, svcErr := service.CreateApplication(context.Background(), app)

	require.Nil(suite.T(), svcErr)
	require.NotNil(suite.T(), result)
	// Redirect-based apps never receive an Flow Secret.
	assert.Empty(suite.T(), result.FlowSecret)
	if capturedCreds != nil {
		var creds map[string]interface{}
		require.NoError(suite.T(), json.Unmarshal(capturedCreds, &creds))
		_, hasFlowSecret := creds[fieldFlowSecret]
		assert.False(suite.T(), hasFlowSecret)
	}
}

func (suite *ServiceTestSuite) TestCreateApplication_NoFlowSecretForPublicClient() {
	testConfig := &config.Config{DeclarativeResources: config.DeclarativeResources{Enabled: false}}
	config.ResetServerRuntime()
	require.NoError(suite.T(), config.InitializeServerRuntime("/tmp/test", testConfig))
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()

	// A browser SPA: public client, no client secret. A caller-supplied FlowSecret must be ignored.
	app := &model.ApplicationDTO{
		Name:       "SPA App",
		OUID:       testOUID,
		FlowSecret: "caller-supplied-secret",
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID: "auth-flow-id",
		},
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:                testClientID,
					RedirectURIs:            []string{"https://example.com/callback"},
					GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
					ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodNone,
					PublicClient:            true,
					PKCERequired:            true,
				},
			},
		},
	}

	var capturedCreds json.RawMessage
	ep := resetEntityProviderMethod(service, "CreateEntity")
	ep.On("CreateEntity", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			capturedCreds = args.Get(1).(json.RawMessage)
		}).
		Return(&providers.Entity{}, (*entityprovider.EntityProviderError)(nil))
	mockStore.On("CreateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	result, svcErr := service.CreateApplication(context.Background(), app)

	require.Nil(suite.T(), svcErr)
	require.NotNil(suite.T(), result)
	// Public clients never receive an Flow Secret.
	assert.Empty(suite.T(), result.FlowSecret)
	if capturedCreds != nil {
		var creds map[string]interface{}
		require.NoError(suite.T(), json.Unmarshal(capturedCreds, &creds))
		_, hasFlowSecret := creds[fieldFlowSecret]
		assert.False(suite.T(), hasFlowSecret)
	}
}

func (suite *ServiceTestSuite) TestCreateApplication_StoreErrorWithOAuthCertRollback() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()

	app := &model.ApplicationDTO{
		Name: "Test OAuth Cert App",
		OUID: testOUID,
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID:         "auth-flow-id",
			RegistrationFlowID: "reg-flow-id",
		},
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:                testClientID,
					RedirectURIs:            []string{"https://example.com/callback"},
					GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
					ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodPrivateKeyJWT,
					Certificate: &inboundmodel.Certificate{
						Type:  "JWKS",
						Value: `{"keys":[]}`,
					},
				},
			},
		},
	}

	// Store creation fails
	mockStore.On("CreateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("internal server error"))

	result, svcErr := service.CreateApplication(context.Background(), app)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), &tidcommon.InternalServerError, svcErr)
}

func (suite *ServiceTestSuite) TestUpdateApplication_NotFound() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()

	app := &model.ApplicationDTO{
		Name: "New Name",
		OUID: testOUID,
	}

	mockStore.On("IsDeclarative", mock.Anything, testServiceAppID).Maybe().Return(false)
	mockStore.On("GetInboundClientByEntityID", mock.Anything, testServiceAppID).
		Return(nil, inboundclient.ErrInboundClientNotFound)

	result, svcErr := service.UpdateApplication(context.Background(), testServiceAppID, app)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), &ErrorApplicationNotFound, svcErr)
}

func (suite *ServiceTestSuite) TestUpdateApplication_NameConflict() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()

	existingApp := &model.ApplicationProcessedDTO{
		ID:   testServiceAppID,
		Name: "Old Name",
	}

	app := &model.ApplicationDTO{
		Name: "New Name",
		OUID: testOUID,
	}

	mockStore.On("IsDeclarative", mock.Anything, testServiceAppID).Maybe().Return(false)
	mockLoadFullApplication(mockStore, service, existingApp)
	mockEP := resetIdentifyEntity(service)
	conflictingID := testConflictingAppID
	mockEP.On("IdentifyEntity",
		map[string]interface{}{"name": "New Name"}).
		Return(
			&conflictingID, (*entityprovider.EntityProviderError)(nil))

	result, svcErr := service.UpdateApplication(context.Background(), testServiceAppID, app)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), &ErrorApplicationAlreadyExistsWithName, svcErr)
}

func (suite *ServiceTestSuite) TestUpdateApplication_MetadataUpdate() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()

	existingApp := &model.ApplicationProcessedDTO{
		ID:   testServiceAppID,
		Name: "Test App",
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID:         "default-auth-flow",
			RegistrationFlowID: "default-reg-flow",
		},
		Metadata: map[string]interface{}{
			"old_key": "old_value",
		},
	}

	updatedApp := &model.ApplicationDTO{
		Name: "Test App",
		OUID: testOUID,
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID:         "default-auth-flow",
			RegistrationFlowID: "default-reg-flow",
		},
		Metadata: map[string]interface{}{
			"new_key":     "new_value",
			"another_key": "another_value",
		},
	}

	mockStore.On("IsDeclarative", mock.Anything, testServiceAppID).Maybe().Return(false)
	mockLoadFullApplication(mockStore, service, existingApp)
	mockStore.On("UpdateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	result, svcErr := service.UpdateApplication(context.Background(), testServiceAppID, updatedApp)

	assert.NotNil(suite.T(), result)
	assert.Nil(suite.T(), svcErr)
	assert.Equal(suite.T(), "new_value", result.Metadata["new_key"])
	assert.Equal(suite.T(), "another_value", result.Metadata["another_key"])
	mockStore.AssertExpectations(suite.T())
}

// TestUpdateApplication_AppCertificateUpdateError verifies that when the app certificate update fails
// inside the transaction, UpdateApplication returns the certificate error.
func (suite *ServiceTestSuite) TestUpdateApplication_AppCertificateUpdateError() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{Enabled: false},
		JWT:                  engineconfig.JWTConfig{ValidityPeriod: 3600},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()

	existingApp := &model.ApplicationProcessedDTO{
		ID:   testServiceAppID,
		Name: "Test App",
	}
	app := &model.ApplicationDTO{
		Name: "Test App",
		OUID: testOUID,
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID:         "edc013d0-e893-4dc0-990c-3e1d203e005b",
			RegistrationFlowID: "80024fb3-29ed-4c33-aa48-8aee5e96d522",
		},
	}

	mockStore.On("IsDeclarative", mock.Anything, testServiceAppID).Maybe().Return(false)
	mockLoadFullApplication(mockStore, service, existingApp)
	mockStore.On("UpdateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("internal server error"))

	result, svcErr := service.UpdateApplication(context.Background(), testServiceAppID, app)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), &tidcommon.InternalServerError, svcErr)
}

// TestResolveClientSecret_PublicClient tests that no secret is generated for public clients.
func TestResolveClientSecret_PublicClient(t *testing.T) {
	inboundAuthConfig := &providers.InboundAuthConfigWithSecret{
		OAuthConfig: &providers.OAuthConfigWithSecret{
			TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodNone,
			ClientSecret:            "",
			PublicClient:            true,
		},
	}

	err := resolveClientSecret(context.Background(), inboundAuthConfig, nil)

	assert.Nil(t, err)
	assert.Equal(t, "", inboundAuthConfig.OAuthConfig.ClientSecret)
}

// TestResolveClientSecret_SecretAlreadyProvided tests that existing secrets are not overwritten.
func TestResolveClientSecret_SecretAlreadyProvided(t *testing.T) {
	providedSecret := "user-provided-secret"
	inboundAuthConfig := &providers.InboundAuthConfigWithSecret{
		OAuthConfig: &providers.OAuthConfigWithSecret{
			TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
			ClientSecret:            providedSecret,
			PublicClient:            false,
		},
	}

	err := resolveClientSecret(context.Background(), inboundAuthConfig, nil)

	assert.Nil(t, err)
	assert.Equal(t, providedSecret, inboundAuthConfig.OAuthConfig.ClientSecret)
}

// TestResolveClientSecret_GenerateForNewConfidentialClient tests secret generation for new clients.
func TestResolveClientSecret_GenerateForNewConfidentialClient(t *testing.T) {
	inboundAuthConfig := &providers.InboundAuthConfigWithSecret{
		OAuthConfig: &providers.OAuthConfigWithSecret{
			TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
			ClientSecret:            "",
			PublicClient:            false,
		},
	}

	err := resolveClientSecret(context.Background(), inboundAuthConfig, nil)

	assert.Nil(t, err)
	assert.NotEmpty(t, inboundAuthConfig.OAuthConfig.ClientSecret)
	// Verify it's a valid OAuth2 secret (should be non-empty and have sufficient length)
	assert.Greater(t, len(inboundAuthConfig.OAuthConfig.ClientSecret), 20)
}

// TestResolveClientSecret_PreserveExistingSecret tests that existing secrets are preserved during updates.
func TestResolveClientSecret_PreserveExistingSecret(t *testing.T) {
	existingApp := &model.ApplicationProcessedDTO{
		InboundAuthConfig: []inboundmodel.InboundAuthConfigProcessed{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthClient{
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
					PublicClient:            false,
				},
			},
		},
	}

	inboundAuthConfig := &providers.InboundAuthConfigWithSecret{
		OAuthConfig: &providers.OAuthConfigWithSecret{
			TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
			ClientSecret:            "",
			PublicClient:            false,
		},
	}

	err := resolveClientSecret(context.Background(), inboundAuthConfig, existingApp)

	assert.Nil(t, err)
	// Secret should remain empty (not generated) because existing app has a secret
	assert.Equal(t, "", inboundAuthConfig.OAuthConfig.ClientSecret)
}

// TestResolveClientSecret_NoExistingApp tests secret generation when no existing app.
func TestResolveClientSecret_NoExistingApp(t *testing.T) {
	inboundAuthConfig := &providers.InboundAuthConfigWithSecret{
		OAuthConfig: &providers.OAuthConfigWithSecret{
			TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
			ClientSecret:            "",
			PublicClient:            false,
		},
	}

	err := resolveClientSecret(context.Background(), inboundAuthConfig, nil)

	assert.Nil(t, err)
	assert.NotEmpty(t, inboundAuthConfig.OAuthConfig.ClientSecret)
}

// TestResolveClientSecret_ExistingAppWithoutSecret tests secret generation when existing app has no secret.
func TestResolveClientSecret_ExistingAppWithoutSecret(t *testing.T) {
	existingApp := &model.ApplicationProcessedDTO{
		InboundAuthConfig: []inboundmodel.InboundAuthConfigProcessed{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthClient{
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodNone,
					PublicClient:            false,
				},
			},
		},
	}

	inboundAuthConfig := &providers.InboundAuthConfigWithSecret{
		OAuthConfig: &providers.OAuthConfigWithSecret{
			TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
			ClientSecret:            "",
			PublicClient:            false,
		},
	}

	err := resolveClientSecret(context.Background(), inboundAuthConfig, existingApp)

	assert.Nil(t, err)
	// Should generate a new secret since existing app doesn't have one
	assert.NotEmpty(t, inboundAuthConfig.OAuthConfig.ClientSecret)
}

// TestUpdateApplication_StoreFails_RollbackCertFails verifies that when the store update fails
// and rolling back the certificate also fails, the rollback error is returned.
func (suite *ServiceTestSuite) TestUpdateApplication_StoreFails_RollbackCertFails() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{Enabled: false},
		JWT:                  engineconfig.JWTConfig{ValidityPeriod: 3600},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()
	existingApp := &model.ApplicationProcessedDTO{
		ID:   "app123",
		Name: "Test App",
	}
	app := &model.ApplicationDTO{
		ID:   "app123",
		Name: "Test App",
		OUID: testOUID,
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID:         "edc013d0-e893-4dc0-990c-3e1d203e005b",
			RegistrationFlowID: "80024fb3-29ed-4c33-aa48-8aee5e96d522",
		},
	}

	mockStore.On("IsDeclarative", mock.Anything, "app123").Maybe().Return(false)
	mockLoadFullApplication(mockStore, service, existingApp)
	mockStore.On("UpdateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("internal server error"))

	result, svcErr := service.UpdateApplication(context.Background(), "app123", app)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), &tidcommon.InternalServerError, svcErr)
}

// TestUpdateApplication_WithOAuthConfig_Success tests successful update of an application with OAuth configuration.
func (suite *ServiceTestSuite) TestUpdateApplication_WithOAuthConfig_Success() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
		JWT: engineconfig.JWTConfig{
			ValidityPeriod: 3600,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()

	existingApp := &model.ApplicationProcessedDTO{
		ID:   testServiceAppID,
		Name: "Test App",
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID:         "auth-flow-id",
			RegistrationFlowID: "reg-flow-id",
		},
		InboundAuthConfig: []inboundmodel.InboundAuthConfigProcessed{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthClient{
					ClientID:                testClientID,
					RedirectURIs:            []string{"https://example.com/callback"},
					GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
					ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
				},
			},
		},
	}

	updatedApp := &model.ApplicationDTO{
		ID:   testServiceAppID,
		Name: "Test App Updated",
		OUID: testOUID,
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID:         "auth-flow-id",
			RegistrationFlowID: "reg-flow-id",
		},
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID: testClientID,
					RedirectURIs: []string{"https://example.com/callback",
						"https://example.com/callback2"},
					GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
					ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
				},
			},
		},
	}

	mockStore.On("IsDeclarative", mock.Anything, testServiceAppID).Maybe().Return(false)
	mockLoadFullApplication(mockStore, service, existingApp)

	mockStore.On("UpdateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	result, svcErr := service.UpdateApplication(context.Background(), testServiceAppID, updatedApp)

	assert.NotNil(suite.T(), result)
	assert.Nil(suite.T(), svcErr)
	assert.Equal(suite.T(), "Test App Updated", result.Name)
	require.Len(suite.T(), result.InboundAuthConfig, 1)
	assert.Equal(suite.T(), testClientID, result.InboundAuthConfig[0].OAuthConfig.ClientID)
	assert.Len(suite.T(), result.InboundAuthConfig[0].OAuthConfig.RedirectURIs, 2)
	mockStore.AssertExpectations(suite.T())
}

// TestUpdateApplication_AddOAuthConfig_Success tests adding OAuth configuration to an app that didn't have it.
func (suite *ServiceTestSuite) TestUpdateApplication_AddOAuthConfig_Success() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
		JWT: engineconfig.JWTConfig{
			ValidityPeriod: 3600,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()

	existingApp := &model.ApplicationProcessedDTO{
		ID:   testServiceAppID,
		Name: "Test App",
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID:         "auth-flow-id",
			RegistrationFlowID: "reg-flow-id",
		},
		InboundAuthConfig: []inboundmodel.InboundAuthConfigProcessed{}, // No OAuth config initially
	}

	updatedApp := &model.ApplicationDTO{
		ID:   testServiceAppID,
		Name: "Test App",
		OUID: testOUID,
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID:         "auth-flow-id",
			RegistrationFlowID: "reg-flow-id",
		},
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:                "new-client-id",
					RedirectURIs:            []string{"https://example.com/callback"},
					GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
					ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
				},
			},
		},
	}

	mockStore.On("IsDeclarative", mock.Anything, testServiceAppID).Maybe().Return(false)
	mockLoadFullApplication(mockStore, service, existingApp)

	mockStore.On("UpdateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	result, svcErr := service.UpdateApplication(context.Background(), testServiceAppID, updatedApp)

	assert.NotNil(suite.T(), result)
	assert.Nil(suite.T(), svcErr)
	require.Len(suite.T(), result.InboundAuthConfig, 1)
	assert.Equal(suite.T(), "new-client-id", result.InboundAuthConfig[0].OAuthConfig.ClientID)
	mockStore.AssertExpectations(suite.T())
}

// TestUpdateApplication_UpdateOAuthClientID_Success tests changing the OAuth client ID.
func (suite *ServiceTestSuite) TestUpdateApplication_UpdateOAuthClientID_Success() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
		JWT: engineconfig.JWTConfig{
			ValidityPeriod: 3600,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()

	existingApp := &model.ApplicationProcessedDTO{
		ID:   testServiceAppID,
		Name: "Test App",
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID:         "auth-flow-id",
			RegistrationFlowID: "reg-flow-id",
		},
		InboundAuthConfig: []inboundmodel.InboundAuthConfigProcessed{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthClient{
					ClientID:                "old-client-id",
					RedirectURIs:            []string{"https://example.com/callback"},
					GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
					ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
				},
			},
		},
	}

	updatedApp := &model.ApplicationDTO{
		ID:   testServiceAppID,
		Name: "Test App",
		OUID: testOUID,
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID:         "auth-flow-id",
			RegistrationFlowID: "reg-flow-id",
		},
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:                "new-client-id",
					RedirectURIs:            []string{"https://example.com/callback"},
					GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
					ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
				},
			},
		},
	}

	mockStore.On("IsDeclarative", mock.Anything, testServiceAppID).Maybe().Return(false)
	mockLoadFullApplication(mockStore, service, existingApp)

	mockStore.On("UpdateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	result, svcErr := service.UpdateApplication(context.Background(), testServiceAppID, updatedApp)

	assert.NotNil(suite.T(), result)
	assert.Nil(suite.T(), svcErr)
	require.Len(suite.T(), result.InboundAuthConfig, 1)
	assert.Equal(suite.T(), "new-client-id", result.InboundAuthConfig[0].OAuthConfig.ClientID)
	mockStore.AssertExpectations(suite.T())
}

func (suite *ServiceTestSuite) runUpdateApplicationWithJWKSCert(jwksValue string) {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
		JWT: engineconfig.JWTConfig{
			ValidityPeriod: 3600,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()

	existingApp := &model.ApplicationProcessedDTO{
		ID:   testServiceAppID,
		Name: "Test App",
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID:         "auth-flow-id",
			RegistrationFlowID: "reg-flow-id",
		},
		InboundAuthConfig: []inboundmodel.InboundAuthConfigProcessed{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthClient{
					ClientID:                testClientID,
					RedirectURIs:            []string{"https://example.com/callback"},
					GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
					ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodPrivateKeyJWT,
				},
			},
		},
	}

	updatedApp := &model.ApplicationDTO{
		ID:   testServiceAppID,
		Name: "Test App",
		OUID: testOUID,
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID:         "auth-flow-id",
			RegistrationFlowID: "reg-flow-id",
		},
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:                testClientID,
					RedirectURIs:            []string{"https://example.com/callback"},
					GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
					ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodPrivateKeyJWT,
					Certificate: &inboundmodel.Certificate{
						Type:  cert.CertificateTypeJWKS,
						Value: jwksValue,
					},
				},
			},
		},
	}

	mockStore.On("IsDeclarative", mock.Anything, testServiceAppID).Maybe().Return(false)
	mockLoadFullApplication(mockStore, service, existingApp)

	mockStore.On("UpdateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	result, svcErr := service.UpdateApplication(context.Background(), testServiceAppID, updatedApp)

	assert.NotNil(suite.T(), result)
	assert.Nil(suite.T(), svcErr)
	require.Len(suite.T(), result.InboundAuthConfig, 1)
	assert.NotNil(suite.T(), result.InboundAuthConfig[0].OAuthConfig.Certificate)
	assert.Equal(suite.T(), cert.CertificateTypeJWKS, result.InboundAuthConfig[0].OAuthConfig.Certificate.Type)
	mockStore.AssertExpectations(suite.T())
}

// TestUpdateApplication_WithOAuthCertificate_Success tests updating an application with a new OAuth certificate.
func (suite *ServiceTestSuite) TestUpdateApplication_WithOAuthCertificate_Success() {
	suite.runUpdateApplicationWithJWKSCert(`{"keys":[{"kty":"RSA"}]}`)
}

// TestUpdateApplication_UpdateOAuthCertificate_Success tests updating an application with a replaced OAuth certificate.
func (suite *ServiceTestSuite) TestUpdateApplication_UpdateOAuthCertificate_Success() {
	suite.runUpdateApplicationWithJWKSCert(`{"keys":[{"kty":"RSA","n":"new-value"}]}`)
}

// TestUpdateApplication_OAuthClientIDConflict tests when the new client ID already exists.
func (suite *ServiceTestSuite) TestUpdateApplication_OAuthClientIDConflict() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()

	existingApp := &model.ApplicationProcessedDTO{
		ID:   testServiceAppID,
		Name: "Test App",
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID:         "auth-flow-id",
			RegistrationFlowID: "reg-flow-id",
		},
		InboundAuthConfig: []inboundmodel.InboundAuthConfigProcessed{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthClient{
					ClientID:                "old-client-id",
					RedirectURIs:            []string{"https://example.com/callback"},
					GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
					ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
				},
			},
		},
	}

	updatedApp := &model.ApplicationDTO{
		ID:   testServiceAppID,
		Name: "Test App",
		OUID: testOUID,
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID:         "auth-flow-id",
			RegistrationFlowID: "reg-flow-id",
		},
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:                "existing-client-id",
					RedirectURIs:            []string{"https://example.com/callback"},
					GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
					ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
				},
			},
		},
	}

	mockStore.On("IsDeclarative", mock.Anything, testServiceAppID).Maybe().Return(false)
	mockLoadFullApplication(mockStore, service, existingApp)

	// Mock that another app already has this client ID via entity provider.
	mockEP := resetIdentifyEntity(service)
	conflictingEntityID := testConflictingAppID
	mockEP.On("IdentifyEntity",
		map[string]interface{}{"clientId": "existing-client-id"}).
		Return(
			&conflictingEntityID, (*entityprovider.EntityProviderError)(nil))

	result, svcErr := service.UpdateApplication(context.Background(), testServiceAppID, updatedApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), &ErrorApplicationAlreadyExistsWithClientID, svcErr)
}

// TestUpdateApplication_OAuthInvalidRedirectURI tests updating with an invalid redirect URI.

// TestUpdateApplication_OAuthStoreErrorWithRollback tests when the inbound-client update fails for an
// OAuth application and the service surfaces an internal-server error.
func (suite *ServiceTestSuite) TestUpdateApplication_OAuthStoreErrorWithRollback() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
		JWT: engineconfig.JWTConfig{
			ValidityPeriod: 3600,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()

	existingApp := &model.ApplicationProcessedDTO{
		ID:   testServiceAppID,
		Name: "Test App",
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID:         "auth-flow-id",
			RegistrationFlowID: "reg-flow-id",
		},
		InboundAuthConfig: []inboundmodel.InboundAuthConfigProcessed{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthClient{
					ClientID:                testClientID,
					RedirectURIs:            []string{"https://example.com/callback"},
					GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
					ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodPrivateKeyJWT,
				},
			},
		},
	}

	updatedApp := &model.ApplicationDTO{
		ID:   testServiceAppID,
		Name: "Test App",
		OUID: testOUID,
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID:         "auth-flow-id",
			RegistrationFlowID: "reg-flow-id",
		},
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:                testClientID,
					RedirectURIs:            []string{"https://example.com/callback"},
					GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
					ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodPrivateKeyJWT,
					Certificate: &inboundmodel.Certificate{
						Type:  cert.CertificateTypeJWKS,
						Value: `{"keys":[{"kty":"RSA"}]}`,
					},
				},
			},
		},
	}

	mockStore.On("IsDeclarative", mock.Anything, testServiceAppID).Maybe().Return(false)
	mockLoadFullApplication(mockStore, service, existingApp)

	// Mock store update failure
	mockStore.On("UpdateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("internal server error"))

	result, svcErr := service.UpdateApplication(context.Background(), testServiceAppID, updatedApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), &tidcommon.InternalServerError, svcErr)
}

// TestUpdateApplication_OAuthTokenConfigUpdate tests updating OAuth token configuration.
func (suite *ServiceTestSuite) TestUpdateApplication_OAuthTokenConfigUpdate() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
		JWT: engineconfig.JWTConfig{
			ValidityPeriod: 3600,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()

	existingApp := &model.ApplicationProcessedDTO{
		ID:   testServiceAppID,
		Name: "Test App",
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID:         "auth-flow-id",
			RegistrationFlowID: "reg-flow-id",
		},
		InboundAuthConfig: []inboundmodel.InboundAuthConfigProcessed{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthClient{
					ClientID:                testClientID,
					RedirectURIs:            []string{"https://example.com/callback"},
					GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
					ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
				},
			},
		},
	}

	updatedApp := &model.ApplicationDTO{
		ID:   testServiceAppID,
		Name: "Test App",
		OUID: testOUID,
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID:         "auth-flow-id",
			RegistrationFlowID: "reg-flow-id",
		},
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:                testClientID,
					RedirectURIs:            []string{"https://example.com/callback"},
					GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
					ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
					Token: &providers.OAuthTokenConfig{
						AccessToken: &providers.AccessTokenConfig{
							UserConfig: &providers.AccessTokenSubConfig{
								ValidityPeriod: 7200,
								Attributes:     []string{"email", "name"},
							},
						},
						IDToken: &providers.IDTokenConfig{
							ValidityPeriod: 3600,
							UserAttributes: []string{"sub", "email"},
						},
					},
				},
			},
		},
	}

	mockStore.On("IsDeclarative", mock.Anything, testServiceAppID).Maybe().Return(false)
	mockLoadFullApplication(mockStore, service, existingApp)

	mockStore.On("UpdateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	result, svcErr := service.UpdateApplication(context.Background(), testServiceAppID, updatedApp)

	assert.NotNil(suite.T(), result)
	assert.Nil(suite.T(), svcErr)
	require.Len(suite.T(), result.InboundAuthConfig, 1)
	assert.NotNil(suite.T(), result.InboundAuthConfig[0].OAuthConfig.Token)
	assert.Equal(suite.T(), int64(7200),
		result.InboundAuthConfig[0].OAuthConfig.Token.AccessToken.UserConfig.ValidityPeriod)
	assert.Equal(suite.T(), int64(3600), result.InboundAuthConfig[0].OAuthConfig.Token.IDToken.ValidityPeriod)
	mockStore.AssertExpectations(suite.T())
}

func (suite *ServiceTestSuite) TestCreateApplication_NilApplication() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, _ := suite.setupTestService()

	result, svcErr := service.CreateApplication(context.Background(), nil)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), &ErrorApplicationNil, svcErr)
}

func (suite *ServiceTestSuite) TestCreateApplication_DeclarativeMode() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: true,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, _ := suite.setupTestService()

	app := &model.ApplicationDTO{
		Name: "Test App",
		OUID: testOUID,
	}

	result, svcErr := service.CreateApplication(context.Background(), app)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), &ErrorCannotModifyDeclarativeResource, svcErr)
}

// TestValidateApplication_ErrorFromProcessInboundAuthConfig tests error from
// processInboundAuthConfig when invalid inbound auth config is provided.
func (suite *ServiceTestSuite) TestValidateApplication_ErrorFromProcessInboundAuthConfig() {
	service, _ := suite.setupTestService()

	app := &model.ApplicationDTO{
		Name: "Test App",
		OUID: testOUID,
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: "InvalidType", // Invalid type, not OAuth
			},
		},
	}

	result, inboundAuth, svcErr := service.ValidateApplication(context.Background(), app)

	assert.Nil(suite.T(), result)
	assert.Nil(suite.T(), inboundAuth)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), &ErrorInvalidInboundAuthConfig, svcErr)
}

// TestTranslateIDTokenValidationError_UnsupportedEncryptionAlg tests the translation of
// ErrOAuthIDTokenUnsupportedEncryptionAlg to a ServiceError.
func (suite *ServiceTestSuite) TestTranslateIDTokenValidationError_UnsupportedEncryptionAlg() {
	svcErr := (&applicationService{}).
		translateInboundClientError(context.Background(), inboundclient.ErrOAuthIDTokenUnsupportedEncryptionAlg)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), ErrorInvalidOAuthConfiguration.Code, svcErr.Code)
}

// TestTranslateIDTokenValidationError_UnsupportedEncryptionEnc tests the translation of
// ErrOAuthIDTokenUnsupportedEncryptionEnc to a ServiceError.
func (suite *ServiceTestSuite) TestTranslateIDTokenValidationError_UnsupportedEncryptionEnc() {
	svcErr := (&applicationService{}).
		translateInboundClientError(context.Background(), inboundclient.ErrOAuthIDTokenUnsupportedEncryptionEnc)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), ErrorInvalidOAuthConfiguration.Code, svcErr.Code)
}

// TestTranslateIDTokenValidationError_EncryptionAlgRequiresEnc tests the translation of
// ErrOAuthIDTokenEncryptionAlgRequiresEnc to a ServiceError.
func (suite *ServiceTestSuite) TestTranslateIDTokenValidationError_EncryptionAlgRequiresEnc() {
	svcErr := (&applicationService{}).
		translateInboundClientError(context.Background(), inboundclient.ErrOAuthIDTokenEncryptionAlgRequiresEnc)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), ErrorInvalidOAuthConfiguration.Code, svcErr.Code)
}

// TestTranslateIDTokenValidationError_EncryptionEncRequiresAlg tests the translation of
// ErrOAuthIDTokenEncryptionEncRequiresAlg to a ServiceError.
func (suite *ServiceTestSuite) TestTranslateIDTokenValidationError_EncryptionEncRequiresAlg() {
	svcErr := (&applicationService{}).
		translateInboundClientError(context.Background(), inboundclient.ErrOAuthIDTokenEncryptionEncRequiresAlg)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), ErrorInvalidOAuthConfiguration.Code, svcErr.Code)
}

// TestTranslateIDTokenValidationError_EncryptionRequiresCertificate tests the translation of
// ErrOAuthIDTokenEncryptionRequiresCertificate to a ServiceError.
func (suite *ServiceTestSuite) TestTranslateIDTokenValidationError_EncryptionRequiresCertificate() {
	svcErr := (&applicationService{}).translateInboundClientError(
		context.Background(), inboundclient.ErrOAuthIDTokenEncryptionRequiresCertificate)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), ErrorInvalidOAuthConfiguration.Code, svcErr.Code)
}

// TestTranslateIDTokenValidationError_JWKSURINotSSRFSafe tests the translation of
// ErrOAuthIDTokenJWKSURINotSSRFSafe to a ServiceError.
func (suite *ServiceTestSuite) TestTranslateIDTokenValidationError_JWKSURINotSSRFSafe() {
	svcErr := (&applicationService{}).
		translateInboundClientError(context.Background(), inboundclient.ErrOAuthIDTokenJWKSURINotSSRFSafe)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), ErrorInvalidOAuthConfiguration.Code, svcErr.Code)
	assert.Equal(suite.T(),
		"error.applicationservice.idtoken_jwks_uri_not_ssrf_safe_description",
		svcErr.ErrorDescription.Key,
	)
}

// ----- translateInboundClientFKError: FlowMismatchError branch -----

func (suite *ServiceTestSuite) TestTranslateInboundClientFKError_FlowMismatchProducesParametricError() {
	err := &inboundclient.FlowMismatchError{
		SourceFlowType: providers.FlowTypeAuthentication,
		FlowType:       providers.FlowTypeRegistration,
	}
	svcErr := translateInboundClientFKError(err)
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorApplicationFlowMismatch.Code, svcErr.Code)
	assert.Equal(suite.T(), "authentication", svcErr.ErrorDescription.Params["sourceFlowType"])
	assert.Equal(suite.T(), "registration", svcErr.ErrorDescription.Params["flowType"])
}

func (suite *ServiceTestSuite) TestTranslateInboundClientFKError_UnknownReturnsNil() {
	svcErr := translateInboundClientFKError(errors.New("something else"))
	assert.Nil(suite.T(), svcErr)
}

// ----- applicationService.ValidateReferenceUpdate -----

func (suite *ServiceTestSuite) TestValidateReferenceUpdate_NonFlowResourceIsNoOp() {
	svc, _ := suite.setupTestService()
	svcErr := svc.ValidateReferenceUpdate(context.Background(), resourcedependency.ResourceTypeTheme, "t1")
	assert.Nil(suite.T(), svcErr)
}

func (suite *ServiceTestSuite) TestValidateReferenceUpdate_GetEntityIDsErrorMapsToInternal() {
	svc, mockStore := suite.setupTestService()
	mockStore.EXPECT().GetEntityIDsByReference(
		mock.Anything, resourcedependency.ResourceTypeFlow, "flow-1",
		serverconst.MaxCompositeStoreRecords, 0,
	).Return(nil, 0, errors.New("db failure"))
	svcErr := svc.ValidateReferenceUpdate(context.Background(), resourcedependency.ResourceTypeFlow, "flow-1")
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *ServiceTestSuite) TestValidateReferenceUpdate_NoReferencingAppsReturnsNil() {
	svc, mockStore := suite.setupTestService()
	mockStore.EXPECT().GetEntityIDsByReference(
		mock.Anything, resourcedependency.ResourceTypeFlow, "flow-1",
		serverconst.MaxCompositeStoreRecords, 0,
	).Return([]string{}, 0, nil)
	svcErr := svc.ValidateReferenceUpdate(context.Background(), resourcedependency.ResourceTypeFlow, "flow-1")
	assert.Nil(suite.T(), svcErr)
}

func (suite *ServiceTestSuite) TestValidateReferenceUpdate_RevalidateFlowMismatchTranslated() {
	svc, mockStore := suite.setupTestService()
	mockStore.EXPECT().GetEntityIDsByReference(
		mock.Anything, resourcedependency.ResourceTypeFlow, "flow-1",
		serverconst.MaxCompositeStoreRecords, 0,
	).Return([]string{"app-1"}, 1, nil)
	mockStore.EXPECT().RevalidateFKs(mock.Anything, "app-1").Return(&inboundclient.FlowMismatchError{
		SourceFlowType: providers.FlowTypeAuthentication,
		FlowType:       providers.FlowTypeRegistration,
	})
	svcErr := svc.ValidateReferenceUpdate(context.Background(), resourcedependency.ResourceTypeFlow, "flow-1")
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), ErrorApplicationFlowMismatch.Code, svcErr.Code)
}

func (suite *ServiceTestSuite) TestValidateReferenceUpdate_RevalidateUnknownErrorMapsToInternal() {
	svc, mockStore := suite.setupTestService()
	mockStore.EXPECT().GetEntityIDsByReference(
		mock.Anything, resourcedependency.ResourceTypeFlow, "flow-1",
		serverconst.MaxCompositeStoreRecords, 0,
	).Return([]string{"app-1"}, 1, nil)
	mockStore.EXPECT().RevalidateFKs(mock.Anything, "app-1").Return(errors.New("unmapped"))
	svcErr := svc.ValidateReferenceUpdate(context.Background(), resourcedependency.ResourceTypeFlow, "flow-1")
	suite.Require().NotNil(svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *ServiceTestSuite) TestValidateReferenceUpdate_AllRevalidationsSucceedReturnsNil() {
	svc, mockStore := suite.setupTestService()
	mockStore.EXPECT().GetEntityIDsByReference(
		mock.Anything, resourcedependency.ResourceTypeFlow, "flow-1",
		serverconst.MaxCompositeStoreRecords, 0,
	).Return([]string{"app-1", "app-2"}, 2, nil)
	mockStore.EXPECT().RevalidateFKs(mock.Anything, "app-1").Return(nil)
	mockStore.EXPECT().RevalidateFKs(mock.Anything, "app-2").Return(nil)
	svcErr := svc.ValidateReferenceUpdate(context.Background(), resourcedependency.ResourceTypeFlow, "flow-1")
	assert.Nil(suite.T(), svcErr)
}

var validAcrMapping = engineconfig.AuthClassConfig{
	Amrs: []string{"PWD", "OTP"},
	AcrAMR: map[string][]string{
		"urn:thunder:acr:password":       {"PWD"},
		"urn:thunder:acr:generated-code": {"OTP"},
	},
}

type AcrValidationTestSuite struct {
	suite.Suite
}

func TestAcrValidationTestSuite(t *testing.T) {
	suite.Run(t, new(AcrValidationTestSuite))
}

func (s *AcrValidationTestSuite) initRegistry(mapping engineconfig.AuthClassConfig) {
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime("", &config.Config{
		OAuth: engineconfig.OAuthConfig{
			AuthClass: mapping,
		},
	}))
	s.T().Cleanup(config.ResetServerRuntime)
}

func (s *AcrValidationTestSuite) TestValidateAcrValues_EmptyList() {
	err := validateAcrValues(nil)
	s.Nil(err)

	err = validateAcrValues([]string{})
	s.Nil(err)
}

func (s *AcrValidationTestSuite) TestValidateAcrValues_AllValid() {
	s.initRegistry(validAcrMapping)

	err := validateAcrValues([]string{
		"urn:thunder:acr:password",
		"urn:thunder:acr:generated-code",
	})

	s.Nil(err)
}

func (s *AcrValidationTestSuite) TestValidateAcrValues_SingleValid() {
	s.initRegistry(validAcrMapping)

	err := validateAcrValues([]string{"urn:thunder:acr:password"})

	s.Nil(err)
}

func (s *AcrValidationTestSuite) TestValidateAcrValues_UnknownACR() {
	s.initRegistry(validAcrMapping)

	svcErr := validateAcrValues([]string{
		"urn:thunder:acr:password",
		"urn:thunder:acr:unknown-method",
	})

	s.NotNil(svcErr)
	s.Equal("APP-1033", svcErr.Code)
	s.Contains(svcErr.ErrorDescription.String(), "urn:thunder:acr:unknown-method")
}

func (s *AcrValidationTestSuite) TestValidateAcrValues_FirstEntryInvalid() {
	s.initRegistry(validAcrMapping)

	svcErr := validateAcrValues([]string{"totally-invalid-acr"})

	s.NotNil(svcErr)
	s.Equal("APP-1033", svcErr.Code)
	s.Contains(svcErr.ErrorDescription.String(), "totally-invalid-acr")
}

func (s *AcrValidationTestSuite) TestIsValidACR_KnownACR() {
	s.initRegistry(validAcrMapping)

	s.True(isValidACR("urn:thunder:acr:password"))
}

func (s *AcrValidationTestSuite) TestIsValidACR_UnknownACR() {
	s.initRegistry(validAcrMapping)

	s.False(isValidACR("urn:thunder:acr:unknown"))
}

func (s *AcrValidationTestSuite) TestIsValidACR_EmptyString() {
	s.initRegistry(validAcrMapping)

	s.False(isValidACR(""))
}

func (s *AcrValidationTestSuite) TestIsValidACR_AllMappedACRs() {
	s.initRegistry(validAcrMapping)

	knownACRs := []string{
		"urn:thunder:acr:password",
		"urn:thunder:acr:generated-code",
	}
	for _, acr := range knownACRs {
		s.True(isValidACR(acr), "expected ACR %q to be valid", acr)
	}
}

func (s *AcrValidationTestSuite) TestIsValidACR_EmptyMapping() {
	s.initRegistry(engineconfig.AuthClassConfig{})

	s.False(isValidACR("urn:thunder:acr:password"))
}

func (suite *ServiceTestSuite) TestTranslateOAuthValidationError() {
	cases := []struct {
		name        string
		err         error
		wantCode    string
		wantDescKey string
	}{
		{
			name:     "InvalidRedirectURI",
			err:      inboundclient.ErrOAuthInvalidRedirectURI,
			wantCode: ErrorInvalidRedirectURI.Code,
		},
		{
			name:        "RedirectURIFragmentNotAllowed",
			err:         inboundclient.ErrOAuthRedirectURIFragmentNotAllowed,
			wantCode:    ErrorInvalidRedirectURI.Code,
			wantDescKey: "error.applicationservice.redirect_uri_fragment_not_allowed_description",
		},
		{
			name:        "AuthCodeRequiresRedirectURIs",
			err:         inboundclient.ErrOAuthAuthCodeRequiresRedirectURIs,
			wantCode:    ErrorInvalidOAuthConfiguration.Code,
			wantDescKey: "error.applicationservice.auth_code_requires_redirect_uris_description",
		},
		{
			name:        "InvalidGrantType",
			err:         inboundclient.ErrOAuthInvalidGrantType,
			wantCode:    ErrorInvalidGrantType.Code,
			wantDescKey: "error.applicationservice.invalid_grant_type_description",
		},
		{
			name:        "InvalidResponseType",
			err:         inboundclient.ErrOAuthInvalidResponseType,
			wantCode:    ErrorInvalidResponseType.Code,
			wantDescKey: "error.applicationservice.invalid_response_type_description",
		},
		{
			name:        "ClientCredentialsCannotUseResponseTypes",
			err:         inboundclient.ErrOAuthClientCredentialsCannotUseResponseTypes,
			wantCode:    ErrorInvalidOAuthConfiguration.Code,
			wantDescKey: "error.applicationservice.client_credentials_cannot_use_response_types_description",
		},
		{
			name:        "AuthCodeRequiresCodeResponseType",
			err:         inboundclient.ErrOAuthAuthCodeRequiresCodeResponseType,
			wantCode:    ErrorInvalidOAuthConfiguration.Code,
			wantDescKey: "error.applicationservice.auth_code_requires_code_response_type_description",
		},
		{
			name:        "RefreshTokenRequiresTokenIssuingGrant",
			err:         inboundclient.ErrOAuthRefreshTokenRequiresTokenIssuingGrant,
			wantCode:    ErrorInvalidOAuthConfiguration.Code,
			wantDescKey: "error.applicationservice.refresh_token_requires_token_issuing_grant_description",
		},
		{
			name:        "PKCERequiresAuthCode",
			err:         inboundclient.ErrOAuthPKCERequiresAuthCode,
			wantCode:    ErrorInvalidOAuthConfiguration.Code,
			wantDescKey: "error.applicationservice.pkce_requires_authorization_code_description",
		},
		{
			name:        "ResponseTypesRequireAuthCode",
			err:         inboundclient.ErrOAuthResponseTypesRequireAuthCode,
			wantCode:    ErrorInvalidOAuthConfiguration.Code,
			wantDescKey: "error.applicationservice.response_types_require_authorization_code_description",
		},
		{
			name:        "InvalidTokenEndpointAuthMethod",
			err:         inboundclient.ErrOAuthInvalidTokenEndpointAuthMethod,
			wantCode:    ErrorInvalidTokenEndpointAuthMethod.Code,
			wantDescKey: "error.applicationservice.invalid_token_endpoint_auth_method_description",
		},
		{
			name:        "PrivateKeyJWTRequiresCertificate",
			err:         inboundclient.ErrOAuthPrivateKeyJWTRequiresCertificate,
			wantCode:    ErrorInvalidOAuthConfiguration.Code,
			wantDescKey: "error.applicationservice.private_key_jwt_requires_certificate_description",
		},
		{
			name:        "CertificateRequiresClientID",
			err:         inboundclient.ErrOAuthCertificateRequiresClientID,
			wantCode:    ErrorInvalidOAuthConfiguration.Code,
			wantDescKey: "error.applicationservice.certificate_requires_client_id_description",
		},
		{
			name:        "PrivateKeyJWTCannotHaveClientSecret",
			err:         inboundclient.ErrOAuthPrivateKeyJWTCannotHaveClientSecret,
			wantCode:    ErrorInvalidOAuthConfiguration.Code,
			wantDescKey: "error.applicationservice.private_key_jwt_cannot_have_client_secret_description",
		},
		{
			name:        "ClientSecretCannotHaveCertificate",
			err:         inboundclient.ErrOAuthClientSecretCannotHaveCertificate,
			wantCode:    ErrorInvalidOAuthConfiguration.Code,
			wantDescKey: "error.applicationservice.client_secret_cannot_have_certificate_description",
		},
		{
			name:        "NoneAuthRequiresPublicClient",
			err:         inboundclient.ErrOAuthNoneAuthRequiresPublicClient,
			wantCode:    ErrorInvalidOAuthConfiguration.Code,
			wantDescKey: "error.applicationservice.none_auth_method_requires_public_client_description",
		},
		{
			name:        "NoneAuthCannotHaveCertOrSecret",
			err:         inboundclient.ErrOAuthNoneAuthCannotHaveCertOrSecret,
			wantCode:    ErrorInvalidOAuthConfiguration.Code,
			wantDescKey: "error.applicationservice.none_auth_method_cannot_have_cert_or_secret_description",
		},
		{
			name:        "ClientCredentialsCannotUseNoneAuth",
			err:         inboundclient.ErrOAuthClientCredentialsCannotUseNoneAuth,
			wantCode:    ErrorInvalidOAuthConfiguration.Code,
			wantDescKey: "error.applicationservice.client_credentials_cannot_use_none_auth_description",
		},
		{
			name:        "PublicClientMustUseNoneAuth",
			err:         inboundclient.ErrOAuthPublicClientMustUseNoneAuth,
			wantCode:    ErrorInvalidPublicClientConfiguration.Code,
			wantDescKey: "error.applicationservice.public_client_must_use_none_auth_description",
		},
		{
			name:        "PublicClientMustHavePKCE",
			err:         inboundclient.ErrOAuthPublicClientMustHavePKCE,
			wantCode:    ErrorInvalidPublicClientConfiguration.Code,
			wantDescKey: "error.applicationservice.public_client_must_have_pkce_description",
		},
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

func (suite *ServiceTestSuite) TestTranslateUserInfoValidationError() {
	cases := []struct {
		name        string
		err         error
		wantDescKey string
	}{
		{
			name:        "UnsupportedSigningAlg",
			err:         inboundclient.ErrOAuthUserInfoUnsupportedSigningAlg,
			wantDescKey: "error.applicationservice.userinfo_unsupported_signing_alg_description",
		},
		{
			name:        "UnsupportedEncryptionAlg",
			err:         inboundclient.ErrOAuthUserInfoUnsupportedEncryptionAlg,
			wantDescKey: "error.applicationservice.userinfo_unsupported_encryption_alg_description",
		},
		{
			name:        "UnsupportedEncryptionEnc",
			err:         inboundclient.ErrOAuthUserInfoUnsupportedEncryptionEnc,
			wantDescKey: "error.applicationservice.userinfo_unsupported_encryption_enc_description",
		},
		{
			name:        "EncryptionAlgRequiresEnc",
			err:         inboundclient.ErrOAuthUserInfoEncryptionAlgRequiresEnc,
			wantDescKey: "error.applicationservice.userinfo_encryption_alg_requires_enc_description",
		},
		{
			name:        "EncryptionEncRequiresAlg",
			err:         inboundclient.ErrOAuthUserInfoEncryptionEncRequiresAlg,
			wantDescKey: "error.applicationservice.userinfo_encryption_enc_requires_alg_description",
		},
		{
			name:        "EncryptionRequiresCertificate",
			err:         inboundclient.ErrOAuthUserInfoEncryptionRequiresCertificate,
			wantDescKey: "error.applicationservice.userinfo_encryption_requires_certificate_description",
		},
		{
			name:        "JWKSURINotSSRFSafe",
			err:         inboundclient.ErrOAuthUserInfoJWKSURINotSSRFSafe,
			wantDescKey: "error.applicationservice.userinfo_jwks_uri_not_ssrf_safe_description",
		},
		{
			name:        "UnsupportedResponseType",
			err:         inboundclient.ErrOAuthUserInfoUnsupportedResponseType,
			wantDescKey: "error.applicationservice.userinfo_unsupported_response_type_description",
		},
		{
			name:        "JWSRequiresSigningAlg",
			err:         inboundclient.ErrOAuthUserInfoJWSRequiresSigningAlg,
			wantDescKey: "error.applicationservice.userinfo_jws_requires_signing_alg_description",
		},
		{
			name:        "JWERequiresEncryption",
			err:         inboundclient.ErrOAuthUserInfoJWERequiresEncryption,
			wantDescKey: "error.applicationservice.userinfo_jwe_requires_encryption_description",
		},
		{
			name:        "NestedJWTRequiresAll",
			err:         inboundclient.ErrOAuthUserInfoNestedJWTRequiresAll,
			wantDescKey: "error.applicationservice.userinfo_nested_jwt_requires_all_description",
		},
		{
			name:        "AlgRequiresResponseType",
			err:         inboundclient.ErrOAuthUserInfoAlgRequiresResponseType,
			wantDescKey: "error.applicationservice.userinfo_alg_requires_response_type_description",
		},
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

func (suite *ServiceTestSuite) TestTranslateIDTokenValidationError() {
	cases := []struct {
		name        string
		err         error
		wantDescKey string
	}{
		{
			name:        "EncryptionFieldsNotAllowed",
			err:         inboundclient.ErrOAuthIDTokenEncryptionFieldsNotAllowed,
			wantDescKey: "error.applicationservice.idtoken_encryption_fields_not_allowed_description",
		},
		{
			name:        "UnsupportedResponseType",
			err:         inboundclient.ErrOAuthIDTokenUnsupportedResponseType,
			wantDescKey: "error.applicationservice.idtoken_unsupported_response_type_description",
		},
		{
			name:        "UnsupportedEncryptionAlg",
			err:         inboundclient.ErrOAuthIDTokenUnsupportedEncryptionAlg,
			wantDescKey: "error.applicationservice.idtoken_unsupported_encryption_alg_description",
		},
		{
			name:        "UnsupportedEncryptionEnc",
			err:         inboundclient.ErrOAuthIDTokenUnsupportedEncryptionEnc,
			wantDescKey: "error.applicationservice.idtoken_unsupported_encryption_enc_description",
		},
		{
			name:        "EncryptionAlgRequiresEnc",
			err:         inboundclient.ErrOAuthIDTokenEncryptionAlgRequiresEnc,
			wantDescKey: "error.applicationservice.idtoken_encryption_alg_requires_enc_description",
		},
		{
			name:        "EncryptionEncRequiresAlg",
			err:         inboundclient.ErrOAuthIDTokenEncryptionEncRequiresAlg,
			wantDescKey: "error.applicationservice.idtoken_encryption_enc_requires_alg_description",
		},
		{
			name:        "EncryptionRequiresCertificate",
			err:         inboundclient.ErrOAuthIDTokenEncryptionRequiresCertificate,
			wantDescKey: "error.applicationservice.idtoken_encryption_requires_certificate_description",
		},
		{
			name:        "JWKSURINotSSRFSafe",
			err:         inboundclient.ErrOAuthIDTokenJWKSURINotSSRFSafe,
			wantDescKey: "error.applicationservice.idtoken_jwks_uri_not_ssrf_safe_description",
		},
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

func (suite *ServiceTestSuite) TestTranslateInboundClientFKError() {
	cases := []struct {
		name     string
		err      error
		wantCode string
	}{
		{
			name:     "InvalidAuthFlow",
			err:      inboundclient.ErrFKInvalidAuthFlow,
			wantCode: ErrorInvalidAuthFlowID.Code,
		},
		{
			name:     "InvalidRegistrationFlow",
			err:      inboundclient.ErrFKInvalidRegistrationFlow,
			wantCode: ErrorInvalidRegistrationFlowID.Code,
		},
		{
			name:     "FlowDefinitionRetrievalFailed",
			err:      inboundclient.ErrFKFlowDefinitionRetrievalFailed,
			wantCode: ErrorWhileRetrievingFlowDefinition.Code,
		},
		{
			name:     "FlowServerError",
			err:      inboundclient.ErrFKFlowServerError,
			wantCode: tidcommon.InternalServerError.Code,
		},
		{
			name:     "ThemeNotFound",
			err:      inboundclient.ErrFKThemeNotFound,
			wantCode: ErrorThemeNotFound.Code,
		},
		{
			name:     "LayoutNotFound",
			err:      inboundclient.ErrFKLayoutNotFound,
			wantCode: ErrorLayoutNotFound.Code,
		},
		{
			name:     "InvalidUserType",
			err:      inboundclient.ErrFKInvalidUserType,
			wantCode: ErrorInvalidUserType.Code,
		},
		{
			name:     "UserSchemaLookupFailed",
			err:      inboundclient.ErrUserSchemaLookupFailed,
			wantCode: tidcommon.InternalServerError.Code,
		},
		{
			name:     "InvalidUserAttribute",
			err:      inboundclient.ErrInvalidUserAttribute,
			wantCode: ErrorInvalidUserAttribute.Code,
		},
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

func (suite *ServiceTestSuite) TestTranslateCertValidationError() {
	cases := []struct {
		name     string
		err      error
		wantCode string
	}{
		{
			name:     "ValueRequired",
			err:      inboundclient.ErrCertValueRequired,
			wantCode: ErrorInvalidCertificateValue.Code,
		},
		{
			name:     "InvalidJWKSURI",
			err:      inboundclient.ErrCertInvalidJWKSURI,
			wantCode: ErrorInvalidJWKSURI.Code,
		},
		{
			name:     "InvalidType",
			err:      inboundclient.ErrCertInvalidType,
			wantCode: ErrorInvalidCertificateType.Code,
		},
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

// ----- validateApplicationFields handle resolution -----

func (suite *ServiceTestSuite) TestValidateApplicationFields_OUHandleResolved() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{Enabled: false},
	}
	config.ResetServerRuntime()
	require.NoError(suite.T(), config.InitializeServerRuntime("/tmp/test", testConfig))
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()

	ouMock := service.ouService.(*oumock.OrganizationUnitServiceInterfaceMock)
	ouMock.On("GetOrganizationUnitByPath", mock.Anything, "default").
		Return(providers.OrganizationUnit{ID: testOUID}, nil).Once()

	app := &model.ApplicationDTO{
		Name:     "test-app",
		OUHandle: "default",
	}

	mockStore.On("Validate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe().Return(nil)

	svcErr := service.validateApplicationFields(context.Background(), app)

	assert.Nil(suite.T(), svcErr)
	assert.Equal(suite.T(), testOUID, app.OUID)
}

func (suite *ServiceTestSuite) TestValidateApplicationFields_OUHandleNotFound() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{Enabled: false},
	}
	config.ResetServerRuntime()
	require.NoError(suite.T(), config.InitializeServerRuntime("/tmp/test", testConfig))
	defer config.ResetServerRuntime()

	service, _ := suite.setupTestService()

	ouMock := service.ouService.(*oumock.OrganizationUnitServiceInterfaceMock)
	ouMock.On("GetOrganizationUnitByPath", mock.Anything, "bad-handle").
		Return(providers.OrganizationUnit{}, &tidcommon.ServiceError{Code: "OUS-4004"}).Once()

	app := &model.ApplicationDTO{
		Name:     "test-app",
		OUHandle: "bad-handle",
	}

	svcErr := service.validateApplicationFields(context.Background(), app)

	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), ErrorInvalidRequestFormat.Code, svcErr.Code)
}

// TestValidateApplicationFields_OUIDWinsWhenBothProvided verifies that when both ou_id and
// ou_handle are supplied, ou_id wins and no handle resolution is attempted (the absence of a
// GetOrganizationUnitByPath mock expectation asserts that). This covers the WARN-on-collision
// branch in validateApplicationFields.
func (suite *ServiceTestSuite) TestValidateApplicationFields_OUIDWinsWhenBothProvided() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{Enabled: false},
	}
	config.ResetServerRuntime()
	require.NoError(suite.T(), config.InitializeServerRuntime("/tmp/test", testConfig))
	defer config.ResetServerRuntime()

	service, _ := suite.setupTestService()

	app := &model.ApplicationDTO{
		Name:     "test-app",
		OUID:     testOUID,
		OUHandle: "some-handle",
	}

	svcErr := service.validateApplicationFields(context.Background(), app)

	assert.Nil(suite.T(), svcErr)
	assert.Equal(suite.T(), testOUID, app.OUID)

	ouMock := service.ouService.(*oumock.OrganizationUnitServiceInterfaceMock)
	ouMock.AssertNotCalled(suite.T(), "GetOrganizationUnitByPath", mock.Anything, mock.Anything)
}

func (suite *ServiceTestSuite) TestValidateApplicationFields_FlowHandleResolutionError() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{Enabled: false},
	}
	config.ResetServerRuntime()
	require.NoError(suite.T(), config.InitializeServerRuntime("/tmp/test", testConfig))
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()

	// Remove the permissive ResolveInboundAuthProfileHandles mock and register an error one
	for i, c := range mockStore.ExpectedCalls {
		if c.Method == "ResolveInboundAuthProfileHandles" {
			mockStore.ExpectedCalls = append(mockStore.ExpectedCalls[:i], mockStore.ExpectedCalls[i+1:]...)
			break
		}
	}
	mockStore.On("ResolveInboundAuthProfileHandles", mock.Anything, mock.Anything).
		Return(inboundclient.ErrFKInvalidAuthFlow).Once()

	app := &model.ApplicationDTO{
		Name: "test-app",
		OUID: testOUID,
	}

	svcErr := service.validateApplicationFields(context.Background(), app)

	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), ErrorInvalidRequestFormat.Code, svcErr.Code)
}

// --- GetResourceDependencies tests ---

func (suite *ServiceTestSuite) TestGetResourceDependencies_UnknownResourceType() {
	service, mockStore := suite.setupTestService()
	mockStore.EXPECT().
		GetEntityIDsByReference(
			mock.Anything, "unknown", "id-1", serverconst.MaxCompositeStoreRecords, 0).
		Return([]string{}, 0, nil)

	result, err := service.GetResourceDependencies(context.Background(), "unknown", "id-1")
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), result)
}

func (suite *ServiceTestSuite) TestGetResourceDependencies_InboundClientError() {
	service, mockStore := suite.setupTestService()
	mockStore.EXPECT().
		GetEntityIDsByReference(
			mock.Anything, resourcedependency.ResourceTypeTheme, "theme-1", serverconst.MaxCompositeStoreRecords, 0).
		Return(nil, 0, errors.New("store error"))

	result, err := service.GetResourceDependencies(
		context.Background(), resourcedependency.ResourceTypeTheme, "theme-1")
	assert.Nil(suite.T(), result)
	assert.Error(suite.T(), err)
}

func (suite *ServiceTestSuite) TestGetResourceDependencies_EmptyIDs() {
	service, mockStore := suite.setupTestService()
	mockStore.EXPECT().
		GetEntityIDsByReference(
			mock.Anything, resourcedependency.ResourceTypeTheme, "theme-1", serverconst.MaxCompositeStoreRecords, 0).
		Return([]string{}, 0, nil)

	result, err := service.GetResourceDependencies(
		context.Background(), resourcedependency.ResourceTypeTheme, "theme-1")
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), result)
}

func (suite *ServiceTestSuite) TestGetResourceDependencies_EntityProviderError() {
	service, mockStore := suite.setupTestService()
	mockStore.EXPECT().
		GetEntityIDsByReference(
			mock.Anything, resourcedependency.ResourceTypeTheme, "theme-1", serverconst.MaxCompositeStoreRecords, 0).
		Return([]string{"app-1"}, 1, nil)

	ep := resetEntityProviderMethod(service, "GetEntitiesByIDs")
	epErr := entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "error", "")
	ep.On("GetEntitiesByIDs", []string{"app-1"}).Return([]providers.Entity{}, epErr)

	result, err := service.GetResourceDependencies(
		context.Background(), resourcedependency.ResourceTypeTheme, "theme-1")
	assert.Nil(suite.T(), result)
	assert.Error(suite.T(), err)
}

func (suite *ServiceTestSuite) TestGetResourceDependencies_Success() {
	service, mockStore := suite.setupTestService()
	mockStore.EXPECT().
		GetEntityIDsByReference(
			mock.Anything, resourcedependency.ResourceTypeTheme, "theme-1", serverconst.MaxCompositeStoreRecords, 0).
		Return([]string{"app-1", "app-2"}, 2, nil)

	sysAttrs1, _ := json.Marshal(map[string]interface{}{"name": "Portal App"})
	sysAttrs2, _ := json.Marshal(map[string]interface{}{"name": "Admin App"})
	ep := resetEntityProviderMethod(service, "GetEntitiesByIDs")
	ep.On("GetEntitiesByIDs", []string{"app-1", "app-2"}).Return([]providers.Entity{
		{ID: "app-1", Category: providers.EntityCategoryApp, SystemAttributes: sysAttrs1},
		{ID: "app-2", Category: providers.EntityCategoryApp, SystemAttributes: sysAttrs2},
	}, (*entityprovider.EntityProviderError)(nil))

	result, err := service.GetResourceDependencies(
		context.Background(), resourcedependency.ResourceTypeTheme, "theme-1")
	assert.NoError(suite.T(), err)
	require.Len(suite.T(), result, 2)
	assert.Equal(suite.T(), resourcedependency.ResourceTypeApplication, result[0].ResourceType)
	assert.Equal(suite.T(), resourcedependency.BehaviorFallback, result[0].BehaviorOnDelete)
	assert.Equal(suite.T(), "app-1", result[0].ID)
	assert.Equal(suite.T(), "Portal App", result[0].DisplayName)
	assert.Equal(suite.T(), "app-2", result[1].ID)
	assert.Equal(suite.T(), "Admin App", result[1].DisplayName)
}

// Agents share the inbound-client store; the application provider must skip non-app entities.
func (suite *ServiceTestSuite) TestGetResourceDependencies_FiltersOutNonAppEntities() {
	service, mockStore := suite.setupTestService()
	mockStore.EXPECT().
		GetEntityIDsByReference(
			mock.Anything, resourcedependency.ResourceTypeTheme, "theme-1", serverconst.MaxCompositeStoreRecords, 0).
		Return([]string{"app-1", "agent-1"}, 2, nil)

	sysAttrs, _ := json.Marshal(map[string]interface{}{"name": "Portal App"})
	ep := resetEntityProviderMethod(service, "GetEntitiesByIDs")
	ep.On("GetEntitiesByIDs", []string{"app-1", "agent-1"}).Return([]providers.Entity{
		{ID: "app-1", Category: providers.EntityCategoryApp, SystemAttributes: sysAttrs},
		{ID: "agent-1", Category: providers.EntityCategoryAgent},
	}, (*entityprovider.EntityProviderError)(nil))

	result, err := service.GetResourceDependencies(
		context.Background(), resourcedependency.ResourceTypeTheme, "theme-1")
	assert.NoError(suite.T(), err)
	require.Len(suite.T(), result, 1)
	assert.Equal(suite.T(), "app-1", result[0].ID)
}

func (suite *ServiceTestSuite) TestGetResourceDependencies_NoSystemAttributes() {
	service, mockStore := suite.setupTestService()
	mockStore.EXPECT().
		GetEntityIDsByReference(
			mock.Anything, resourcedependency.ResourceTypeTheme, "theme-1", serverconst.MaxCompositeStoreRecords, 0).
		Return([]string{"app-1"}, 1, nil)

	ep := resetEntityProviderMethod(service, "GetEntitiesByIDs")
	ep.On("GetEntitiesByIDs", []string{"app-1"}).Return([]providers.Entity{
		{ID: "app-1", Category: providers.EntityCategoryApp},
	}, (*entityprovider.EntityProviderError)(nil))

	result, err := service.GetResourceDependencies(
		context.Background(), resourcedependency.ResourceTypeTheme, "theme-1")
	assert.NoError(suite.T(), err)
	require.Len(suite.T(), result, 1)
	assert.Equal(suite.T(), "app-1", result[0].ID)
	assert.Equal(suite.T(), "", result[0].DisplayName)
}

func (suite *ServiceTestSuite) TestGetResourceDependencies_SystemAttributesWithoutName() {
	service, mockStore := suite.setupTestService()
	mockStore.EXPECT().
		GetEntityIDsByReference(
			mock.Anything, resourcedependency.ResourceTypeTheme, "theme-1", serverconst.MaxCompositeStoreRecords, 0).
		Return([]string{"app-1"}, 1, nil)

	sysAttrs, _ := json.Marshal(map[string]interface{}{"description": "some desc"})
	ep := resetEntityProviderMethod(service, "GetEntitiesByIDs")
	ep.On("GetEntitiesByIDs", []string{"app-1"}).Return([]providers.Entity{
		{ID: "app-1", Category: providers.EntityCategoryApp, SystemAttributes: sysAttrs},
	}, (*entityprovider.EntityProviderError)(nil))

	result, err := service.GetResourceDependencies(
		context.Background(), resourcedependency.ResourceTypeTheme, "theme-1")
	assert.NoError(suite.T(), err)
	require.Len(suite.T(), result, 1)
	assert.Equal(suite.T(), "", result[0].DisplayName)
}

func (suite *ServiceTestSuite) TestCreateApplication_CreateInboundClientFailsAndCompensationFails() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()

	app := &model.ApplicationDTO{
		Name: "Test App",
		OUID: testOUID,
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID:         "edc013d0-e893-4dc0-990c-3e1d203e005b",
			RegistrationFlowID: "80024fb3-29ed-4c33-aa48-8aee5e96d522",
		},
	}

	mockStore.On("CreateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("internal server error"))

	ep := resetEntityProviderMethod(service, "DeleteEntity")
	ep.On("DeleteEntity", mock.Anything).
		Return(entityprovider.NewEntityProviderError(
			entityprovider.ErrorCodeSystemError, "delete failed", ""))

	result, svcErr := service.CreateApplication(context.Background(), app)

	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), &tidcommon.InternalServerError, svcErr)
}

func (suite *ServiceTestSuite) TestValidateApplication_InboundClientValidateError() {
	service, mockStore := suite.setupTestService()

	for i, c := range mockStore.ExpectedCalls {
		if c.Method == "Validate" {
			mockStore.ExpectedCalls = append(mockStore.ExpectedCalls[:i], mockStore.ExpectedCalls[i+1:]...)
			break
		}
	}
	mockStore.On("Validate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("validation failed"))

	app := &model.ApplicationDTO{
		Name: "Test App",
		OUID: testOUID,
	}

	processed, inboundAuthConfig, svcErr := service.ValidateApplication(context.Background(), app)

	assert.Nil(suite.T(), processed)
	assert.Nil(suite.T(), inboundAuthConfig)
	assert.Equal(suite.T(), &tidcommon.InternalServerError, svcErr)
}

func (suite *ServiceTestSuite) TestGetApplicationList_CountError() {
	service, _ := suite.setupTestService()

	resetEntityProviderMethod(service, "GetEntityListCount").
		On("GetEntityListCount", providers.EntityCategoryApp, mock.Anything).
		Return(0, entityprovider.NewEntityProviderError(
			entityprovider.ErrorCodeSystemError, "count failed", ""))

	result, svcErr := service.GetApplicationList(context.Background())

	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), &tidcommon.InternalServerError, svcErr)
}

func (suite *ServiceTestSuite) TestGetApplicationList_EntityWithoutInboundClient() {
	service, mockStore := suite.setupTestService()

	entities := []providers.Entity{
		{ID: "app1", Category: providers.EntityCategoryApp},
	}
	resetEntityProviderMethod(service, "GetEntityListCount").
		On("GetEntityListCount", providers.EntityCategoryApp, mock.Anything).
		Return(1, (*entityprovider.EntityProviderError)(nil))
	ep := resetEntityProviderMethod(service, "GetEntityList")
	ep.On("GetEntityList", providers.EntityCategoryApp,
		mock.AnythingOfType("int"), mock.AnythingOfType("int"), mock.Anything).
		Return(entities, (*entityprovider.EntityProviderError)(nil))

	mockStore.On("GetInboundClientList", mock.Anything).
		Return([]inboundmodel.InboundClient{}, nil)

	result, svcErr := service.GetApplicationList(context.Background())

	assert.Nil(suite.T(), svcErr)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), 0, result.Count)
}

func (suite *ServiceTestSuite) TestUpdateEntityDataForApplicationUpdate_UpdateSystemAttributesError() {
	service, _ := suite.setupTestService()

	app := &model.ApplicationDTO{Name: "Test App", OUID: testOUID}

	ep := resetEntityProviderMethod(service, "UpdateSystemAttributes")
	ep.On("UpdateSystemAttributes", mock.Anything, mock.Anything).
		Return(entityprovider.NewEntityProviderError(
			entityprovider.ErrorCodeSystemError, "update failed", ""))

	svcErr := service.updateEntityDataForApplicationUpdate(context.Background(), testServiceAppID, app, nil)

	assert.Equal(suite.T(), &tidcommon.InternalServerError, svcErr)
}

func (suite *ServiceTestSuite) TestUpdateEntityDataForApplicationUpdate_FlowSecretUpdateError() {
	service, _ := suite.setupTestService()

	app := &model.ApplicationDTO{Name: "Test App", OUID: testOUID, FlowSecret: "new-app-secret"}

	ep := resetEntityProviderMethod(service, "UpdateSystemCredentials")
	ep.On("UpdateSystemCredentials", mock.Anything, mock.Anything).
		Return(entityprovider.NewEntityProviderError(
			entityprovider.ErrorCodeSystemError, "update failed", ""))

	svcErr := service.updateEntityDataForApplicationUpdate(context.Background(), testServiceAppID, app, nil)

	assert.Equal(suite.T(), &tidcommon.InternalServerError, svcErr)
}

// A public or redirect-based app is not eligible for an Flow Secret, so a supplied FlowSecret must
// be ignored on update and the credential store must not be touched for it.
func (suite *ServiceTestSuite) TestUpdateEntityDataForApplicationUpdate_NoFlowSecretRotationForRedirectClient() {
	service, _ := suite.setupTestService()

	app := &model.ApplicationDTO{Name: "Test App", OUID: testOUID, FlowSecret: "new-app-secret"}
	inboundAuthConfig := &providers.InboundAuthConfigWithSecret{
		OAuthConfig: &providers.OAuthConfigWithSecret{
			ClientID:                testClientID,
			GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
			TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodNone,
			PublicClient:            true,
		},
	}

	ep := service.entityProvider.(*entityprovidermock.EntityProviderInterfaceMock)

	svcErr := service.updateEntityDataForApplicationUpdate(
		context.Background(), testServiceAppID, app, inboundAuthConfig)

	assert.Nil(suite.T(), svcErr)
	ep.AssertNotCalled(suite.T(), "UpdateSystemCredentials", mock.Anything, mock.Anything)
}

func (suite *ServiceTestSuite) TestUpdateEntityDataForApplicationUpdate_UpdateCredentialsError() {
	service, _ := suite.setupTestService()

	app := &model.ApplicationDTO{Name: "Test App", OUID: testOUID}
	inboundAuthConfig := &providers.InboundAuthConfigWithSecret{
		OAuthConfig: &providers.OAuthConfigWithSecret{
			ClientID:                testClientID,
			ClientSecret:            "secret-value",
			TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
		},
	}

	ep := resetEntityProviderMethod(service, "UpdateSystemCredentials")
	ep.On("UpdateSystemCredentials", mock.Anything, mock.Anything).
		Return(entityprovider.NewEntityProviderError(
			entityprovider.ErrorCodeSystemError, "update creds failed", ""))

	svcErr := service.updateEntityDataForApplicationUpdate(
		context.Background(), testServiceAppID, app, inboundAuthConfig)

	assert.Equal(suite.T(), &tidcommon.InternalServerError, svcErr)
}

func (suite *ServiceTestSuite) TestDeleteApplication_DeleteEntityError() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()

	mockStore.On("DeleteInboundClient", mock.Anything, testServiceAppID).Return(nil)
	ep := resetEntityProviderMethod(service, "DeleteEntity")
	ep.On("DeleteEntity", testServiceAppID).
		Return(entityprovider.NewEntityProviderError(
			entityprovider.ErrorCodeSystemError, "delete failed", ""))

	svcErr := service.DeleteApplication(context.Background(), testServiceAppID)

	assert.Equal(suite.T(), &tidcommon.InternalServerError, svcErr)
}

func (suite *ServiceTestSuite) TestGetApplication_GetEntityError() {
	service, mockStore := suite.setupTestService()

	inboundClient := &inboundmodel.InboundClient{ID: testServiceAppID}
	mockStore.On("GetInboundClientByEntityID", mock.Anything, testServiceAppID).Return(inboundClient, nil)

	ep := resetEntityProviderMethod(service, "GetEntity")
	ep.On("GetEntity", testServiceAppID).
		Return((*providers.Entity)(nil), entityprovider.NewEntityProviderError(
			entityprovider.ErrorCodeSystemError, "get failed", ""))

	result, svcErr := service.getApplication(context.Background(), testServiceAppID)

	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), &tidcommon.InternalServerError, svcErr)
}

func (suite *ServiceTestSuite) TestGetApplication_GetOAuthProfileError() {
	service, mockStore := suite.setupTestService()

	inboundClient := &inboundmodel.InboundClient{ID: testServiceAppID}
	mockStore.On("GetInboundClientByEntityID", mock.Anything, testServiceAppID).Return(inboundClient, nil)
	mockStore.On("GetOAuthProfileByEntityID", mock.Anything, testServiceAppID).
		Return((*providers.OAuthProfile)(nil), errors.New("oauth profile load failed"))

	ep := resetEntityProviderMethod(service, "GetEntity")
	ep.On("GetEntity", testServiceAppID).
		Return(&providers.Entity{ID: testServiceAppID, Category: providers.EntityCategoryApp},
			(*entityprovider.EntityProviderError)(nil))

	result, svcErr := service.getApplication(context.Background(), testServiceAppID)

	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), &tidcommon.InternalServerError, svcErr)
}

func (suite *ServiceTestSuite) TestDeleteLocalizedVariants_DeleteError() {
	service, _ := suite.setupTestService()

	i18nMock := mgtmock.NewI18nServiceInterfaceMock(suite.T())
	i18nMock.On("DeleteTranslationsByKey", mock.Anything, mock.Anything, mock.Anything).
		Return(&tidcommon.InternalServerError)
	service.i18nService = i18nMock

	svcErr := service.deleteLocalizedVariants(context.Background(), testServiceAppID)

	assert.Equal(suite.T(), &tidcommon.InternalServerError, svcErr)
}

func (suite *ServiceTestSuite) TestCleanupStaleI18nKeys_DeleteError() {
	service, _ := suite.setupTestService()

	i18nMock := mgtmock.NewI18nServiceInterfaceMock(suite.T())
	i18nMock.On("DeleteTranslationsByKey", mock.Anything, mock.Anything, mock.Anything).
		Return(&tidcommon.InternalServerError)
	service.i18nService = i18nMock

	existing := &model.ApplicationProcessedDTO{
		ID:   testServiceAppID,
		Name: AppI18nRef(testServiceAppID, "name"),
	}
	updated := &model.ApplicationDTO{
		Name: "Plain Name",
	}

	svcErr := service.cleanupStaleI18nKeys(context.Background(), testServiceAppID, existing, updated)

	assert.Equal(suite.T(), &tidcommon.InternalServerError, svcErr)
}

func (suite *ServiceTestSuite) TestDeleteApplication_AbortedWhenCascadeFails() {
	service, mockStore := suite.setupTestService()
	service.SetDependencyRegistry(noopDepRegistry{cascadeErr: errors.New("cascade failed")})

	svcErr := service.DeleteApplication(context.Background(), testServiceAppID)

	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
	mockStore.AssertNotCalled(suite.T(), "DeleteInboundClient", mock.Anything, mock.Anything)
}

// TestDeleteApplication_EntityDeleteFailsAfterCascade verifies that when dependency cleanup
// succeeds but the later entity delete fails, the service surfaces an error while the dependents
// have already been removed (partial-state, retriable).
func (suite *ServiceTestSuite) TestDeleteApplication_EntityDeleteFailsAfterCascade() {
	service, mockStore := suite.setupTestService()
	cascadeCalls := 0
	service.SetDependencyRegistry(recordingDepRegistry{cascadeCalls: &cascadeCalls})

	mockStore.On("DeleteInboundClient", mock.Anything, testServiceAppID).Return(nil)
	ep := resetEntityProviderMethod(service, "DeleteEntity")
	ep.On("DeleteEntity", mock.Anything).Return(
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeSystemError, "delete failed", ""))

	svcErr := service.DeleteApplication(context.Background(), testServiceAppID)

	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
	// Dependents were removed even though the application delete did not complete.
	assert.Equal(suite.T(), 1, cascadeCalls)
	ep.AssertCalled(suite.T(), "DeleteEntity", mock.Anything)
}
