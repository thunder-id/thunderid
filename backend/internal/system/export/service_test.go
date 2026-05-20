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

package export

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/thunder-id/thunderid/internal/application"
	appmodel "github.com/thunder-id/thunderid/internal/application/model"
	"github.com/thunder-id/thunderid/internal/entitytype"
	flowcommon "github.com/thunder-id/thunderid/internal/flow/common"
	flowmgt "github.com/thunder-id/thunderid/internal/flow/mgt"
	"github.com/thunder-id/thunderid/internal/idp"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/notification"
	"github.com/thunder-id/thunderid/internal/notification/common"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	i18ncore "github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/tests/mocks/applicationmock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/flowmgtmock"
	"github.com/thunder-id/thunderid/tests/mocks/idp/idpmock"
	"github.com/thunder-id/thunderid/tests/mocks/notification/notificationmock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

const (
	testAppID     = "test-app-id"
	testIDPID     = "test-idp-id"
	testApp1ID    = "app1"
	testApp2ID    = "app2"
	testApp3ID    = "app3"
	testAppTestID = "app-test-id"
	testFlowID    = "test-flow-id"
	testFlow1ID   = "flow1"
	testFlow2ID   = "flow2"
)

// ExportServiceTestSuite defines the test suite for the export service.
type ExportServiceTestSuite struct {
	suite.Suite
	appServiceMock          *applicationmock.ApplicationServiceInterfaceMock
	idpServiceMock          *idpmock.IDPServiceInterfaceMock
	mockNotificationService *notificationmock.NotificationSenderMgtSvcInterfaceMock
	mockEntityTypeService   *entitytypemock.EntityTypeServiceInterfaceMock
	mockFlowService         *flowmgtmock.FlowMgtServiceInterfaceMock
	exportService           ExportServiceInterface
}

// SetupTest sets up the test environment before each test.
func (suite *ExportServiceTestSuite) SetupTest() {
	// Create temporary directory
	tempDir := suite.T().TempDir()

	// Initialize server runtime with declarative mode disabled
	// Use just the filename since InitializeServerRuntime will prepend the base path
	config.ResetServerRuntime()
	testConfig := &config.Config{
		Crypto: config.CryptoConfig{
			Encryption: config.EncryptionConfig{
				Key: "0579f866ac7c9273580d0ff163fa01a7b2401a7ff3ddc3e3b14ae3136fa6025e",
			},
		},
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	_ = config.InitializeServerRuntime(tempDir, testConfig)

	suite.appServiceMock = applicationmock.NewApplicationServiceInterfaceMock(suite.T())
	suite.idpServiceMock = idpmock.NewIDPServiceInterfaceMock(suite.T())
	suite.mockNotificationService = notificationmock.NewNotificationSenderMgtSvcInterfaceMock(suite.T())
	suite.mockEntityTypeService = entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	suite.mockFlowService = flowmgtmock.NewFlowMgtServiceInterfaceMock(suite.T())

	// Create exporters
	exporters := []declarativeresource.ResourceExporter{
		application.NewApplicationExporterForTest(suite.appServiceMock),
		idp.NewIDPExporterForTest(suite.idpServiceMock),
		notification.NewNotificationSenderExporterForTest(suite.mockNotificationService),
		entitytype.NewEntityTypeExporterForTest(suite.mockEntityTypeService),
		flowmgt.NewFlowGraphExporterForTest(suite.mockFlowService),
	}

	// Create parameterizer instance
	parameterizer := newParameterizer(templatingRules{})

	suite.exportService = newExportService(exporters, parameterizer)
}

func (suite *ExportServiceTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

// TestExportServiceTestSuite runs the test suite.
func TestExportServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ExportServiceTestSuite))
}

// TestExportResources_NilRequest tests ExportResources with nil request.
func (suite *ExportServiceTestSuite) TestExportResources_NilRequest() {
	result, err := suite.exportService.ExportResources(context.Background(), nil)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorInvalidRequest.Code, err.Code)
	assert.Equal(suite.T(), "Invalid export request", err.Error.DefaultValue)
}

// TestExportResources_EmptyRequest tests ExportResources with empty request.
func (suite *ExportServiceTestSuite) TestExportResources_EmptyRequest() {
	request := &ExportRequest{}

	result, err := suite.exportService.ExportResources(context.Background(), request)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorNoResourcesFound.Code, err.Code)
	assert.Equal(suite.T(), "No resources found", err.Error.DefaultValue)
}

// TestExportResources_DefaultOptions tests ExportResources with default options.
func (suite *ExportServiceTestSuite) TestExportResources_DefaultOptions() {
	appID := testAppID
	request := &ExportRequest{
		Applications: []string{appID},
	}

	mockApp := &appmodel.Application{
		ID:          appID,
		Name:        "Test App",
		Description: "Test Description",
	}

	suite.appServiceMock.EXPECT().GetApplication(mock.Anything, appID).Return(mockApp, nil)

	result, err := suite.exportService.ExportResources(context.Background(), request)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Len(suite.T(), result.Files, 1)
	assert.Nil(suite.T(), result.EnvFile)
	assert.Equal(suite.T(), 1, result.Summary.TotalFiles)
	assert.Contains(suite.T(), result.Summary.ResourceTypes, "application")
	assert.Contains(suite.T(), result.Files[0].Content, "# resource_type: application")
}

func (suite *ExportServiceTestSuite) TestAddResourceTypeComment() {
	content := "name: sample\n"
	annotated := addResourceTypeComment(content, "application")

	assert.Equal(suite.T(), "# resource_type: application\nname: sample\n", annotated)

	annotatedAgain := addResourceTypeComment(annotated, "application")
	assert.Equal(suite.T(), annotated, annotatedAgain)
}

// TestExportResources_ApplicationNotFound tests ExportResources when application is not found.
func (suite *ExportServiceTestSuite) TestExportResources_ApplicationNotFound() {
	appID := "non-existent-app"
	request := &ExportRequest{
		Applications: []string{appID},
		Options: &ExportOptions{
			Format: "yaml",
		},
	}

	appError := &serviceerror.ServiceError{
		Code:  "APP_NOT_FOUND",
		Error: i18ncore.I18nMessage{DefaultValue: "Application not found"},
	}

	suite.appServiceMock.EXPECT().GetApplication(mock.Anything, appID).Return(nil, appError)

	result, err := suite.exportService.ExportResources(context.Background(), request)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorNoResourcesFound.Code, err.Code)
}

// TestExportResources_CompleteOAuthApplication tests exporting an application with OAuth config.
func (suite *ExportServiceTestSuite) TestExportResources_CompleteOAuthApplication() {
	appID := "oauth-app-id"
	request := &ExportRequest{
		Applications: []string{appID},
		Options: &ExportOptions{
			Format: "yaml",
			FolderStructure: &FolderStructureOptions{
				GroupByType:       true,
				FileNamingPattern: "${name}_${id}",
			},
		},
	}

	mockOAuthConfig := &inboundmodel.OAuthConfigWithSecret{
		ClientID:                "client123",
		RedirectURIs:            []string{"http://localhost:3000/callback"},
		GrantTypes:              []oauth2const.GrantType{oauth2const.GrantTypeAuthorizationCode},
		ResponseTypes:           []oauth2const.ResponseType{oauth2const.ResponseTypeCode},
		TokenEndpointAuthMethod: oauth2const.TokenEndpointAuthMethodClientSecretPost,
		PKCERequired:            true,
		PublicClient:            false,
		Scopes:                  []string{"openid", "profile"},
		Token: &inboundmodel.OAuthTokenConfig{
			AccessToken: &inboundmodel.AccessTokenConfig{
				ValidityPeriod: 3600,
				UserAttributes: []string{"email", "username"},
			},
			IDToken: &inboundmodel.IDTokenConfig{
				ValidityPeriod: 1800,
				UserAttributes: []string{"email"},
			},
		},
		ScopeClaims: map[string][]string{
			"profile": {"name", "picture"},
		},
	}

	mockApp := &appmodel.Application{
		ID:          appID,
		Name:        "OAuth Test App",
		Description: "OAuth Test Description",
		URL:         "https://example.com",
		InboundAuthConfig: []inboundmodel.InboundAuthConfigWithSecret{
			{
				Type:        inboundmodel.OAuthInboundAuthType,
				OAuthConfig: mockOAuthConfig,
			},
		},
		InboundAuthProfile: inboundmodel.InboundAuthProfile{
			Assertion: &inboundmodel.AssertionConfig{
				UserAttributes: []string{"email", "username"},
			},
		},
	}

	suite.appServiceMock.EXPECT().GetApplication(mock.Anything, appID).Return(mockApp, nil)

	result, err := suite.exportService.ExportResources(context.Background(), request)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Len(suite.T(), result.Files, 1)

	file := result.Files[0]
	assert.Equal(suite.T(), "OAuth_Test_App_oauth-app-id.yaml", file.FileName)
	assert.Equal(suite.T(), "applications", file.FolderPath)
	assert.Contains(suite.T(), file.Content, "name: OAuth Test App")
	assert.Contains(suite.T(), file.Content, "client_id: {{.O_AUTH_TEST_APP_CLIENT_ID}}")
	assert.Contains(suite.T(), file.Content, "client_secret: {{.O_AUTH_TEST_APP_CLIENT_SECRET}}")
	assert.Contains(suite.T(), file.Content, "redirect_uris:")
	assert.Contains(suite.T(), file.Content, "{{- range .O_AUTH_TEST_APP_REDIRECT_URIS}}")
	assert.NotNil(suite.T(), result.EnvFile)
	assert.Equal(suite.T(), ".env", result.EnvFile.FileName)
	assert.Contains(suite.T(), result.EnvFile.Content, "O_AUTH_TEST_APP_CLIENT_ID=client123\n")
	assert.Contains(suite.T(), result.EnvFile.Content, "O_AUTH_TEST_APP_CLIENT_SECRET=\n")
	expectedRedirectURIs := "O_AUTH_TEST_APP_REDIRECT_URIS=[\"http://localhost:3000/callback\"]\n"
	assert.Contains(suite.T(), result.EnvFile.Content, expectedRedirectURIs)

	assert.Equal(suite.T(), 1, result.Summary.ResourceTypes["application"])
	assert.Equal(suite.T(), int64(len(file.Content)), file.Size)
}

// TestExportResources_MultipleApplications tests exporting multiple applications.
func (suite *ExportServiceTestSuite) TestExportResources_MultipleApplications() {
	request := &ExportRequest{
		Applications: []string{testApp1ID, testApp2ID},
		Options: &ExportOptions{
			Format: "yaml",
		},
	}

	mockApp1 := &appmodel.Application{
		ID:          testApp1ID,
		Name:        "App One",
		Description: "First App",
	}

	mockApp2 := &appmodel.Application{
		ID:          testApp2ID,
		Name:        "App Two",
		Description: "Second App",
	}

	suite.appServiceMock.EXPECT().GetApplication(mock.Anything, testApp1ID).Return(mockApp1, nil)
	suite.appServiceMock.EXPECT().GetApplication(mock.Anything, testApp2ID).Return(mockApp2, nil)

	result, err := suite.exportService.ExportResources(context.Background(), request)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Len(suite.T(), result.Files, 2)
	assert.Equal(suite.T(), 2, result.Summary.TotalFiles)
	assert.Equal(suite.T(), 2, result.Summary.ResourceTypes["application"])
}

// TestExportResources_PartialFailure tests exporting when some applications fail.
func (suite *ExportServiceTestSuite) TestExportResources_PartialFailure() {
	app1ID := "valid-app"
	app2ID := "invalid-app"
	request := &ExportRequest{
		Applications: []string{app1ID, app2ID},
		Options: &ExportOptions{
			Format: "yaml",
		},
	}

	mockApp1 := &appmodel.Application{
		ID:   app1ID,
		Name: "Valid App",
	}

	appError := &serviceerror.ServiceError{
		Code:  "APP_NOT_FOUND",
		Error: i18ncore.I18nMessage{DefaultValue: "Application not found"},
	}

	suite.appServiceMock.EXPECT().GetApplication(mock.Anything, app1ID).Return(mockApp1, nil)
	suite.appServiceMock.EXPECT().GetApplication(mock.Anything, app2ID).Return(nil, appError)

	result, err := suite.exportService.ExportResources(context.Background(), request)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Len(suite.T(), result.Files, 1) // Only one successful export
	assert.Equal(suite.T(), 1, result.Summary.TotalFiles)
	assert.Len(suite.T(), result.Summary.Errors, 1) // One error recorded
	assert.Equal(suite.T(), "application", result.Summary.Errors[0].ResourceType)
	assert.Equal(suite.T(), app2ID, result.Summary.Errors[0].ResourceID)
}

// TestExportResources_WildcardApplications tests exporting all applications using wildcard.
func (suite *ExportServiceTestSuite) TestExportResources_WildcardApplications() {
	request := &ExportRequest{
		Applications: []string{"*"},
		Options: &ExportOptions{
			Format: "yaml",
		},
	}

	// Mock GetApplicationList to return 3 applications
	mockAppList := &appmodel.ApplicationListResponse{
		TotalResults: 3,
		Count:        3,
		Applications: []appmodel.BasicApplicationResponse{
			{ID: testApp1ID, Name: "Application One"},
			{ID: testApp2ID, Name: "Application Two"},
			{ID: testApp3ID, Name: "Application Three"},
		},
	}

	mockApp1 := &appmodel.Application{
		ID:          testApp1ID,
		Name:        "Application One",
		Description: "First App",
	}

	mockApp2 := &appmodel.Application{
		ID:          testApp2ID,
		Name:        "Application Two",
		Description: "Second App",
	}

	mockApp3 := &appmodel.Application{
		ID:          testApp3ID,
		Name:        "Application Three",
		Description: "Third App",
	}

	suite.appServiceMock.EXPECT().GetApplicationList(mock.Anything).Return(mockAppList, nil)
	suite.appServiceMock.EXPECT().GetApplication(mock.Anything, testApp1ID).Return(mockApp1, nil)
	suite.appServiceMock.EXPECT().GetApplication(mock.Anything, testApp2ID).Return(mockApp2, nil)
	suite.appServiceMock.EXPECT().GetApplication(mock.Anything, testApp3ID).Return(mockApp3, nil)

	result, err := suite.exportService.ExportResources(context.Background(), request)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Len(suite.T(), result.Files, 3) // All 3 applications exported
	assert.Equal(suite.T(), 3, result.Summary.TotalFiles)
	assert.Equal(suite.T(), 3, result.Summary.ResourceTypes["application"])
	assert.Len(suite.T(), result.Summary.Errors, 0) // No errors
}

// TestExportResources_WildcardApplications_ListFailure tests wildcard export when GetApplicationList fails.
func (suite *ExportServiceTestSuite) TestExportResources_WildcardApplications_ListFailure() {
	request := &ExportRequest{
		Applications: []string{"*"},
		Options: &ExportOptions{
			Format: "yaml",
		},
	}

	listError := &serviceerror.ServiceError{
		Code:  "LIST_FAILED",
		Error: i18ncore.I18nMessage{DefaultValue: "Failed to list applications"},
	}

	suite.appServiceMock.EXPECT().GetApplicationList(mock.Anything).Return(nil, listError)

	result, err := suite.exportService.ExportResources(context.Background(), request)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorNoResourcesFound.Code, err.Code)
}

// TestExportResources_WildcardApplications_EmptyList tests wildcard export with empty application list.
func (suite *ExportServiceTestSuite) TestExportResources_WildcardApplications_EmptyList() {
	request := &ExportRequest{
		Applications: []string{"*"},
		Options: &ExportOptions{
			Format: "yaml",
		},
	}

	// Mock GetApplicationList to return empty list
	mockAppList := &appmodel.ApplicationListResponse{
		TotalResults: 0,
		Count:        0,
		Applications: []appmodel.BasicApplicationResponse{},
	}

	suite.appServiceMock.EXPECT().GetApplicationList(mock.Anything).Return(mockAppList, nil)

	result, err := suite.exportService.ExportResources(context.Background(), request)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorNoResourcesFound.Code, err.Code)
}

// TestExportResources_WildcardApplications_PartialFailure tests wildcard export with partial failures.
func (suite *ExportServiceTestSuite) TestExportResources_WildcardApplications_PartialFailure() {
	request := &ExportRequest{
		Applications: []string{"*"},
		Options: &ExportOptions{
			Format: "yaml",
		},
	}

	// Mock GetApplicationList to return 3 applications
	mockAppList := &appmodel.ApplicationListResponse{
		TotalResults: 3,
		Count:        3,
		Applications: []appmodel.BasicApplicationResponse{
			{ID: testApp1ID, Name: "Application One"},
			{ID: testApp2ID, Name: "Application Two"},
			{ID: testApp3ID, Name: "Application Three"},
		},
	}

	mockApp1 := &appmodel.Application{
		ID:          testApp1ID,
		Name:        "Application One",
		Description: "First App",
	}

	mockApp3 := &appmodel.Application{
		ID:          testApp3ID,
		Name:        "Application Three",
		Description: "Third App",
	}

	appError := &serviceerror.ServiceError{
		Code:  "APP_NOT_FOUND",
		Error: i18ncore.I18nMessage{DefaultValue: "Application not found"},
	}

	suite.appServiceMock.EXPECT().GetApplicationList(mock.Anything).Return(mockAppList, nil)
	suite.appServiceMock.EXPECT().GetApplication(mock.Anything, testApp1ID).Return(mockApp1, nil)
	suite.appServiceMock.EXPECT().GetApplication(mock.Anything, testApp2ID).Return(nil, appError)
	suite.appServiceMock.EXPECT().GetApplication(mock.Anything, testApp3ID).Return(mockApp3, nil)

	result, err := suite.exportService.ExportResources(context.Background(), request)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Len(suite.T(), result.Files, 2) // 2 successful exports
	assert.Equal(suite.T(), 2, result.Summary.TotalFiles)
	assert.Equal(suite.T(), 2, result.Summary.ResourceTypes["application"])
	assert.Len(suite.T(), result.Summary.Errors, 1) // One error recorded
	assert.Equal(suite.T(), "application", result.Summary.Errors[0].ResourceType)
	assert.Equal(suite.T(), testApp2ID, result.Summary.Errors[0].ResourceID)
}

// TestExportResources_IdentityProvider_Success tests exporting a single IDP successfully.
func (suite *ExportServiceTestSuite) TestExportResources_IdentityProvider_Success() {
	idpID := testIDPID
	request := &ExportRequest{
		IdentityProviders: []string{idpID},
		Options: &ExportOptions{
			Format: "yaml",
		},
	}

	mockProperty, _ := cmodels.NewProperty("client_id", "test-client-id", false)
	mockIDP := &idp.IDPDTO{
		ID:          idpID,
		Name:        "Test IDP",
		Description: "Test Identity Provider",
		Type:        idp.IDPTypeGoogle,
		Properties:  []cmodels.Property{*mockProperty},
	}

	suite.idpServiceMock.EXPECT().GetIdentityProvider(mock.Anything, idpID).Return(mockIDP, nil)

	result, err := suite.exportService.ExportResources(context.Background(), request)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Len(suite.T(), result.Files, 1)
	assert.NotNil(suite.T(), result.EnvFile)
	assert.Equal(suite.T(), 2, result.Summary.TotalFiles)
	assert.Contains(suite.T(), result.Summary.ResourceTypes, "identity_provider")
	assert.Equal(suite.T(), "Test_IDP.yaml", result.Files[0].FileName)
	assert.Equal(suite.T(), "identity_provider", result.Files[0].ResourceType)
	assert.Contains(suite.T(), result.Files[0].Content, "name: Test IDP")
}

// TestExportResources_IdentityProvider_Multiple tests exporting multiple IDPs.
// nolint:dupl // Similar test pattern for different resource types
func (suite *ExportServiceTestSuite) TestExportResources_IdentityProvider_Multiple() {
	mockProperty1, _ := cmodels.NewProperty("client_id", "client1", false)
	mockIDP1 := &idp.IDPDTO{
		ID:         "idp1",
		Name:       "Google IDP",
		Type:       idp.IDPTypeGoogle,
		Properties: []cmodels.Property{*mockProperty1},
	}

	mockProperty2, _ := cmodels.NewProperty("client_id", "client2", false)
	mockIDP2 := &idp.IDPDTO{
		ID:         "idp2",
		Name:       "GitHub IDP",
		Type:       idp.IDPTypeGitHub,
		Properties: []cmodels.Property{*mockProperty2},
	}

	suite.idpServiceMock.EXPECT().GetIdentityProvider(mock.Anything, "idp1").Return(mockIDP1, nil)
	suite.idpServiceMock.EXPECT().GetIdentityProvider(mock.Anything, "idp2").Return(mockIDP2, nil)

	request := &ExportRequest{
		IdentityProviders: []string{"idp1", "idp2"},
		Options:           &ExportOptions{Format: "yaml"},
	}
	result, err := suite.exportService.ExportResources(context.Background(), request)

	suite.assertMultipleResourcesExport(result, err, 2, "identity_provider")
}

// TestExportResources_IdentityProvider_Wildcard tests exporting all IDPs using wildcard.
func (suite *ExportServiceTestSuite) TestExportResources_IdentityProvider_Wildcard() {
	request := &ExportRequest{
		IdentityProviders: []string{"*"},
		Options: &ExportOptions{
			Format: "yaml",
		},
	}

	mockIDPList := []idp.BasicIDPDTO{
		{ID: "idp1", Name: "Google IDP"},
		{ID: "idp2", Name: "GitHub IDP"},
	}

	mockProperty1, _ := cmodels.NewProperty("client_id", "client1", false)
	mockIDP1 := &idp.IDPDTO{
		ID:         "idp1",
		Name:       "Google IDP",
		Type:       idp.IDPTypeGoogle,
		Properties: []cmodels.Property{*mockProperty1},
	}

	mockProperty2, _ := cmodels.NewProperty("client_id", "client2", false)
	mockIDP2 := &idp.IDPDTO{
		ID:         "idp2",
		Name:       "GitHub IDP",
		Type:       idp.IDPTypeGitHub,
		Properties: []cmodels.Property{*mockProperty2},
	}

	suite.idpServiceMock.EXPECT().GetIdentityProviderList(mock.Anything).Return(mockIDPList, nil)
	suite.idpServiceMock.EXPECT().GetIdentityProvider(mock.Anything, "idp1").Return(mockIDP1, nil)
	suite.idpServiceMock.EXPECT().GetIdentityProvider(mock.Anything, "idp2").Return(mockIDP2, nil)

	result, err := suite.exportService.ExportResources(context.Background(), request)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Len(suite.T(), result.Files, 2)
	assert.NotNil(suite.T(), result.EnvFile)
	assert.Equal(suite.T(), 3, result.Summary.TotalFiles)
}

// TestExportResources_Mixed_ApplicationsAndIDPs tests exporting both applications and IDPs.
func (suite *ExportServiceTestSuite) TestExportResources_Mixed_ApplicationsAndIDPs() {
	request := &ExportRequest{
		Applications:      []string{testAppID},
		IdentityProviders: []string{testIDPID},
		Options: &ExportOptions{
			Format: "yaml",
		},
	}

	mockApp := &appmodel.Application{
		ID:          testAppID,
		Name:        "Test App",
		Description: "Test Description",
	}

	mockProperty, _ := cmodels.NewProperty("client_id", "test-client-id", false)
	mockIDP := &idp.IDPDTO{
		ID:         testIDPID,
		Name:       "Test IDP",
		Type:       idp.IDPTypeGoogle,
		Properties: []cmodels.Property{*mockProperty},
	}

	suite.appServiceMock.EXPECT().GetApplication(mock.Anything, testAppID).Return(mockApp, nil)
	suite.idpServiceMock.EXPECT().GetIdentityProvider(mock.Anything, testIDPID).Return(mockIDP, nil)

	result, err := suite.exportService.ExportResources(context.Background(), request)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Len(suite.T(), result.Files, 2) // 1 app + 1 IDP
	assert.NotNil(suite.T(), result.EnvFile)
	assert.Equal(suite.T(), 3, result.Summary.TotalFiles)
	assert.Contains(suite.T(), result.Summary.ResourceTypes, "application")
	assert.Contains(suite.T(), result.Summary.ResourceTypes, "identity_provider")
	assert.Equal(suite.T(), 1, result.Summary.ResourceTypes["application"])
	assert.Equal(suite.T(), 1, result.Summary.ResourceTypes["identity_provider"])
}

// TestExportResources_IdentityProvider_NotFound tests error handling when IDP not found.
func (suite *ExportServiceTestSuite) TestExportResources_IdentityProvider_NotFound() {
	request := &ExportRequest{
		IdentityProviders: []string{"non-existent-idp"},
		Options: &ExportOptions{
			Format: "yaml",
		},
	}

	idpError := &serviceerror.ServiceError{
		Code:  "IDP_NOT_FOUND",
		Error: i18ncore.I18nMessage{DefaultValue: "Identity provider not found"},
	}

	suite.idpServiceMock.EXPECT().GetIdentityProvider(mock.Anything, "non-existent-idp").Return(nil, idpError)

	result, err := suite.exportService.ExportResources(context.Background(), request)

	// Should return error since no valid resources found
	assert.NotNil(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), ErrorNoResourcesFound.Code, err.Code)
}

// TestExportResources_IdentityProvider_WildcardPartialFailure tests wildcard IDP export with partial failures.
func (suite *ExportServiceTestSuite) TestExportResources_IdentityProvider_WildcardPartialFailure() {
	request := &ExportRequest{
		IdentityProviders: []string{"*"},
		Options: &ExportOptions{
			Format: "yaml",
		},
	}

	mockIDPList := []idp.BasicIDPDTO{
		{ID: "idp1", Name: "Google IDP"},
		{ID: "idp2", Name: "GitHub IDP"},
		{ID: "idp3", Name: "OIDC IDP"},
	}

	mockProperty1, _ := cmodels.NewProperty("client_id", "client1", false)
	mockIDP1 := &idp.IDPDTO{
		ID:         "idp1",
		Name:       "Google IDP",
		Type:       idp.IDPTypeGoogle,
		Properties: []cmodels.Property{*mockProperty1},
	}

	mockProperty3, _ := cmodels.NewProperty("client_id", "client3", false)
	mockIDP3 := &idp.IDPDTO{
		ID:         "idp3",
		Name:       "OIDC IDP",
		Type:       idp.IDPTypeOIDC,
		Properties: []cmodels.Property{*mockProperty3},
	}

	idpError := &serviceerror.ServiceError{
		Code:  "IDP_NOT_FOUND",
		Error: i18ncore.I18nMessage{DefaultValue: "Identity provider not found"},
	}

	suite.idpServiceMock.EXPECT().GetIdentityProviderList(mock.Anything).Return(mockIDPList, nil)
	suite.idpServiceMock.EXPECT().GetIdentityProvider(mock.Anything, "idp1").Return(mockIDP1, nil)
	suite.idpServiceMock.EXPECT().GetIdentityProvider(mock.Anything, "idp2").Return(nil, idpError)
	suite.idpServiceMock.EXPECT().GetIdentityProvider(mock.Anything, "idp3").Return(mockIDP3, nil)

	result, err := suite.exportService.ExportResources(context.Background(), request)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Len(suite.T(), result.Files, 2) // 2 successful exports
	assert.NotNil(suite.T(), result.EnvFile)
	assert.Equal(suite.T(), 3, result.Summary.TotalFiles)
	assert.Equal(suite.T(), 2, result.Summary.ResourceTypes["identity_provider"])
	assert.Len(suite.T(), result.Summary.Errors, 1) // One error recorded
	assert.Equal(suite.T(), "identity_provider", result.Summary.Errors[0].ResourceType)
	assert.Equal(suite.T(), "idp2", result.Summary.Errors[0].ResourceID)
}

func (suite *ExportServiceTestSuite) assertExportNoProperties(request *ExportRequest, expectedContent string) {
	result, err := suite.exportService.ExportResources(context.Background(), request)

	// Should succeed even with no properties
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Len(suite.T(), result.Files, 1)
	assert.Equal(suite.T(), 1, result.Summary.TotalFiles)
	assert.Contains(suite.T(), result.Files[0].Content, expectedContent)
}

// TestExportResources_IdentityProvider_NoProperties tests exporting IDP with no properties.
func (suite *ExportServiceTestSuite) TestExportResources_IdentityProvider_NoProperties() {
	request := &ExportRequest{
		IdentityProviders: []string{"idp-no-props"},
		Options: &ExportOptions{
			Format: "yaml",
		},
	}

	// IDP with no properties
	mockIDP := &idp.IDPDTO{
		ID:         "idp-no-props",
		Name:       "Empty IDP",
		Type:       idp.IDPTypeOIDC,
		Properties: []cmodels.Property{}, // Empty properties
	}

	suite.idpServiceMock.EXPECT().GetIdentityProvider(mock.Anything, "idp-no-props").Return(mockIDP, nil)

	suite.assertExportNoProperties(request, "name: Empty IDP")
}

// TestExportResources_IdentityProvider_EmptyName tests validation for IDP with empty name.
func (suite *ExportServiceTestSuite) TestExportResources_IdentityProvider_EmptyName() {
	request := &ExportRequest{
		IdentityProviders: []string{"idp-no-name"},
		Options: &ExportOptions{
			Format: "yaml",
		},
	}

	mockProperty, _ := cmodels.NewProperty("key", "value", false)
	mockIDP := &idp.IDPDTO{
		ID:         "idp-no-name",
		Name:       "", // Empty name
		Type:       idp.IDPTypeOIDC,
		Properties: []cmodels.Property{*mockProperty},
	}

	suite.idpServiceMock.EXPECT().GetIdentityProvider(mock.Anything, "idp-no-name").Return(mockIDP, nil)

	result, err := suite.exportService.ExportResources(context.Background(), request)

	// Should return error since name is required
	assert.NotNil(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), ErrorNoResourcesFound.Code, err.Code)
}

// TestExportResources_IdentityProvider_PropertyParameterization verifies that IDP properties
// are correctly parameterized with context-aware variable names.
func (suite *ExportServiceTestSuite) TestExportResources_IdentityProvider_PropertyParameterization() {
	idpID := "test-parameterization-idp"
	request := &ExportRequest{
		IdentityProviders: []string{idpID},
		Options: &ExportOptions{
			Format: "yaml",
		},
	}

	// Create properties with various names
	clientIDProp, _ := cmodels.NewProperty("client_id", "test-client-123", true)
	clientSecretProp, _ := cmodels.NewProperty("client_secret", "super-secret", true)
	redirectURIProp, _ := cmodels.NewProperty("redirect_uri", "http://localhost:3000", false)

	mockIDP := &idp.IDPDTO{
		ID:          idpID,
		Name:        "Export Test IDP",
		Description: "Test IDP for parameterization",
		Type:        idp.IDPTypeGoogle,
		Properties: []cmodels.Property{
			*clientIDProp,
			*clientSecretProp,
			*redirectURIProp,
		},
	}

	suite.idpServiceMock.EXPECT().GetIdentityProvider(mock.Anything, idpID).Return(mockIDP, nil)

	result, err := suite.exportService.ExportResources(context.Background(), request)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Len(suite.T(), result.Files, 1)

	yamlContent := result.Files[0].Content

	// Verify the YAML contains parameterized property values with context-aware variable names
	// Variable names should be: IDP_NAME + PROPERTY_NAME in UPPER_SNAKE_CASE
	assert.Contains(suite.T(), yamlContent, "{{.EXPORT_TEST_IDP_CLIENT_ID}}")
	assert.Contains(suite.T(), yamlContent, "{{.EXPORT_TEST_IDP_CLIENT_SECRET}}")
	assert.Contains(suite.T(), yamlContent, "{{.EXPORT_TEST_IDP_REDIRECT_URI}}")

	// Verify property names are preserved
	assert.Contains(suite.T(), yamlContent, "name: client_id")
	assert.Contains(suite.T(), yamlContent, "name: client_secret")
	assert.Contains(suite.T(), yamlContent, "name: redirect_uri")

	// Verify secret flags are preserved (YAML uses 'is_secret' field name)
	assert.Contains(suite.T(), yamlContent, "is_secret: true")

	// Verify basic IDP fields
	assert.Contains(suite.T(), yamlContent, "name: Export Test IDP")
	assert.Contains(suite.T(), yamlContent, "type: GOOGLE")
}

// TestExportResources_IdentityProvider_PropertyStructure verifies that IDP properties
// are exported with correct YAML structure including name, value, and is_secret fields.
func (suite *ExportServiceTestSuite) TestExportResources_IdentityProvider_PropertyStructure() {
	idpID := "test-property-structure"
	request := &ExportRequest{
		IdentityProviders: []string{idpID},
		Options: &ExportOptions{
			Format: "yaml",
		},
	}

	// Create properties with various combinations - some secret, some not
	clientIDProp, _ := cmodels.NewProperty("client_id", "test-client-123", false)
	clientSecretProp, _ := cmodels.NewProperty("client_secret", "super-secret-value", true)
	apiKeyProp, _ := cmodels.NewProperty("api_key", "api-key-xyz", true)
	callbackURLProp, _ := cmodels.NewProperty("callback_url", "https://example.com/callback", false)

	mockIDP := &idp.IDPDTO{
		ID:          idpID,
		Name:        "Property Structure Test",
		Description: "Test IDP for property YAML structure validation",
		Type:        idp.IDPTypeOIDC,
		Properties: []cmodels.Property{
			*clientIDProp,
			*clientSecretProp,
			*apiKeyProp,
			*callbackURLProp,
		},
	}

	suite.idpServiceMock.EXPECT().GetIdentityProvider(mock.Anything, idpID).Return(mockIDP, nil)

	result, err := suite.exportService.ExportResources(context.Background(), request)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Len(suite.T(), result.Files, 1)

	yamlContent := result.Files[0].Content

	// Verify property names are preserved in the YAML
	assert.Contains(suite.T(), yamlContent, "name: client_id")
	assert.Contains(suite.T(), yamlContent, "name: client_secret")
	assert.Contains(suite.T(), yamlContent, "name: api_key")
	assert.Contains(suite.T(), yamlContent, "name: callback_url")

	// Verify all properties have value fields (template variables due to DynamicPropertyFields)
	assert.Contains(suite.T(), yamlContent, "value:")

	// Verify secret flags are preserved for secret properties
	// Count occurrences of "is_secret: true" - should be 2 (client_secret and api_key)
	secretCount := strings.Count(yamlContent, "is_secret: true")
	assert.Equal(suite.T(), 2, secretCount, "Should have exactly 2 secret properties")

	// Verify the properties section exists and has proper structure
	assert.Contains(suite.T(), yamlContent, "properties:")

	// Verify basic IDP fields
	assert.Contains(suite.T(), yamlContent, "name: Property Structure Test")
	assert.Contains(suite.T(), yamlContent, "description: Test IDP for property YAML structure validation")
	assert.Contains(suite.T(), yamlContent, "type: OIDC")

	// Verify proper indentation and YAML list structure for properties
	assert.Contains(suite.T(), yamlContent, "properties:\n  - name:")
}

// TestExportResources_PartialFailure_DetailedErrorValidation enhances the existing partial failure test
// with detailed error field validation.
func (suite *ExportServiceTestSuite) TestExportResources_PartialFailure_DetailedErrorValidation() {
	app1ID := "app1"
	app2ID := "app2-not-found"

	request := &ExportRequest{
		Applications: []string{app1ID, app2ID},
		Options: &ExportOptions{
			Format: "yaml",
		},
	}

	mockApp1 := &appmodel.Application{
		ID:   app1ID,
		Name: "Valid App",
	}

	appError := &serviceerror.ServiceError{
		Code:  "APP_NOT_FOUND",
		Error: i18ncore.I18nMessage{DefaultValue: "Application not found"},
	}

	suite.appServiceMock.EXPECT().GetApplication(mock.Anything, app1ID).Return(mockApp1, nil)
	suite.appServiceMock.EXPECT().GetApplication(mock.Anything, app2ID).Return(nil, appError)

	result, err := suite.exportService.ExportResources(context.Background(), request)

	// Verify successful export
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Len(suite.T(), result.Files, 1)
	assert.Equal(suite.T(), 1, result.Summary.TotalFiles)

	// Verify error details
	assert.Len(suite.T(), result.Summary.Errors, 1)
	exportError := result.Summary.Errors[0]
	assert.Equal(suite.T(), "application", exportError.ResourceType)
	assert.Equal(suite.T(), app2ID, exportError.ResourceID)
	assert.Equal(suite.T(), "APP_NOT_FOUND", exportError.Code)
	assert.Equal(suite.T(), "Application not found", exportError.Error)

	// Verify file size calculation
	assert.Equal(suite.T(), int64(len(result.Files[0].Content)), result.Files[0].Size)
	assert.Greater(suite.T(), result.Summary.TotalSize, int64(0))
}

// TestExportResources_IdentityProvider_PartialFailure_DetailedErrorValidation tests IDP partial failure.
func (suite *ExportServiceTestSuite) TestExportResources_IdentityProvider_PartialFailure_DetailedErrorValidation() {
	request := &ExportRequest{
		IdentityProviders: []string{"idp1", "idp2-not-found", "idp3"},
		Options: &ExportOptions{
			Format: "yaml",
		},
	}

	mockProperty1, _ := cmodels.NewProperty("client_id", "client1", false)
	mockIDP1 := &idp.IDPDTO{
		ID:         "idp1",
		Name:       "Google IDP",
		Type:       idp.IDPTypeGoogle,
		Properties: []cmodels.Property{*mockProperty1},
	}

	mockProperty3, _ := cmodels.NewProperty("client_id", "client3", false)
	mockIDP3 := &idp.IDPDTO{
		ID:         "idp3",
		Name:       "GitHub IDP",
		Type:       idp.IDPTypeGitHub,
		Properties: []cmodels.Property{*mockProperty3},
	}

	idpError := &serviceerror.ServiceError{
		Code:  "IDP_NOT_FOUND",
		Error: i18ncore.I18nMessage{DefaultValue: "Identity provider not found"},
	}

	suite.idpServiceMock.EXPECT().GetIdentityProvider(mock.Anything, "idp1").Return(mockIDP1, nil)
	suite.idpServiceMock.EXPECT().GetIdentityProvider(mock.Anything, "idp2-not-found").Return(nil, idpError)
	suite.idpServiceMock.EXPECT().GetIdentityProvider(mock.Anything, "idp3").Return(mockIDP3, nil)

	result, err := suite.exportService.ExportResources(context.Background(), request)

	// Verify partial success
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Len(suite.T(), result.Files, 2) // Two successful exports
	assert.NotNil(suite.T(), result.EnvFile)
	assert.Equal(suite.T(), 3, result.Summary.TotalFiles)

	// Verify error details
	assert.Len(suite.T(), result.Summary.Errors, 1)
	exportError := result.Summary.Errors[0]
	assert.Equal(suite.T(), "identity_provider", exportError.ResourceType)
	assert.Equal(suite.T(), "idp2-not-found", exportError.ResourceID)
	assert.Equal(suite.T(), "IDP_NOT_FOUND", exportError.Code)
	assert.Equal(suite.T(), "Identity provider not found", exportError.Error)

	// Verify file sizes
	for _, file := range result.Files {
		assert.Equal(suite.T(), int64(len(file.Content)), file.Size)
	}
	assert.Greater(suite.T(), result.Summary.TotalSize, int64(0))
}

// TestExportResources_MixedResources_WithErrors tests exporting both apps and IDPs with some failures.
func (suite *ExportServiceTestSuite) TestExportResources_MixedResources_WithErrors() {
	request := &ExportRequest{
		Applications:      []string{"app1", "app2-not-found"},
		IdentityProviders: []string{"idp1", "idp2-not-found"},
		Options: &ExportOptions{
			Format: "yaml",
		},
	}

	// Setup successful app
	mockApp1 := &appmodel.Application{
		ID:   "app1",
		Name: "Valid App",
	}

	// Setup app error
	appError := &serviceerror.ServiceError{
		Code:  "APP_NOT_FOUND",
		Error: i18ncore.I18nMessage{DefaultValue: "Application not found"},
	}

	// Setup successful IDP
	mockProperty1, _ := cmodels.NewProperty("client_id", "client1", false)
	mockIDP1 := &idp.IDPDTO{
		ID:         "idp1",
		Name:       "Google IDP",
		Type:       idp.IDPTypeGoogle,
		Properties: []cmodels.Property{*mockProperty1},
	}

	// Setup IDP error
	idpError := &serviceerror.ServiceError{
		Code:  "IDP_NOT_FOUND",
		Error: i18ncore.I18nMessage{DefaultValue: "Identity provider not found"},
	}

	suite.appServiceMock.EXPECT().GetApplication(mock.Anything, "app1").Return(mockApp1, nil)
	suite.appServiceMock.EXPECT().GetApplication(mock.Anything, "app2-not-found").Return(nil, appError)
	suite.idpServiceMock.EXPECT().GetIdentityProvider(mock.Anything, "idp1").Return(mockIDP1, nil)
	suite.idpServiceMock.EXPECT().GetIdentityProvider(mock.Anything, "idp2-not-found").Return(nil, idpError)

	result, err := suite.exportService.ExportResources(context.Background(), request)

	// Verify partial success
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Len(suite.T(), result.Files, 2) // One app + one IDP
	assert.NotNil(suite.T(), result.EnvFile)
	assert.Equal(suite.T(), 3, result.Summary.TotalFiles)

	// Verify resource type counts
	assert.Equal(suite.T(), 1, result.Summary.ResourceTypes["application"])
	assert.Equal(suite.T(), 1, result.Summary.ResourceTypes["identity_provider"])

	// Verify errors - should have 2 errors (1 app, 1 IDP)
	assert.Len(suite.T(), result.Summary.Errors, 2)

	// Verify app error
	var appErrorFound bool
	var idpErrorFound bool
	for _, e := range result.Summary.Errors {
		if e.ResourceType == resourceTypeApplication {
			appErrorFound = true
			assert.Equal(suite.T(), "app2-not-found", e.ResourceID)
			assert.Equal(suite.T(), "APP_NOT_FOUND", e.Code)
		}
		if e.ResourceType == resourceTypeIdentityProvider {
			idpErrorFound = true
			assert.Equal(suite.T(), "idp2-not-found", e.ResourceID)
			assert.Equal(suite.T(), "IDP_NOT_FOUND", e.Code)
		}
	}
	assert.True(suite.T(), appErrorFound, "Application error not found in Summary.Errors")
	assert.True(suite.T(), idpErrorFound, "IDP error not found in Summary.Errors")
}

// TestExportResources_FileSizeCalculation tests that file sizes are calculated correctly.
func (suite *ExportServiceTestSuite) TestExportResources_FileSizeCalculation() {
	request := &ExportRequest{
		Applications: []string{testApp1ID, testApp2ID},
		Options: &ExportOptions{
			Format: "yaml",
		},
	}

	mockApp1 := &appmodel.Application{
		ID:          testApp1ID,
		Name:        "Application One",
		Description: "First application",
	}

	mockApp2 := &appmodel.Application{
		ID:          testApp2ID,
		Name:        "Application Two",
		Description: "Second application with longer description",
	}

	suite.appServiceMock.EXPECT().GetApplication(mock.Anything, testApp1ID).Return(mockApp1, nil)
	suite.appServiceMock.EXPECT().GetApplication(mock.Anything, testApp2ID).Return(mockApp2, nil)

	result, err := suite.exportService.ExportResources(context.Background(), request)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Len(suite.T(), result.Files, 2)

	// Verify each file's size matches its content length
	var totalCalculatedSize int64
	for _, file := range result.Files {
		expectedSize := int64(len(file.Content))
		assert.Equal(suite.T(), expectedSize, file.Size, "File size mismatch for %s", file.FileName)
		assert.Greater(suite.T(), file.Size, int64(0), "File size should be greater than 0")
		totalCalculatedSize += file.Size
	}

	// Verify total size matches sum of individual file sizes
	assert.Equal(suite.T(), totalCalculatedSize, result.Summary.TotalSize)
	assert.Greater(suite.T(), result.Summary.TotalSize, int64(0))
}

// MockParameterizer is a mock implementation of ParameterizerInterface for testing.
type MockParameterizer struct {
	shouldFail bool
	errorMsg   string
}

func (m *MockParameterizer) ToParameterizedYAML(obj interface{},
	resourceType string, resourceName string,
	rules *declarativeresource.ResourceRules) (string, map[string]string, error) {
	if m.shouldFail {
		return "", nil, fmt.Errorf("%s", m.errorMsg)
	}
	// Return minimal valid YAML
	return "id: test\nname: test\n", nil, nil
}

// TestExportResources_TemplateGenerationError tests the error path in generateTemplateFromStruct.
func (suite *ExportServiceTestSuite) TestExportResources_TemplateGenerationError() {
	request := &ExportRequest{
		Applications: []string{testApp1ID, testApp2ID},
		Options: &ExportOptions{
			Format: "yaml",
		},
	}

	mockApp1 := &appmodel.Application{
		ID:   testApp1ID,
		Name: "Valid App",
	}

	mockApp2 := &appmodel.Application{
		ID:   testApp2ID,
		Name: "App That Fails Template Generation",
	}

	suite.appServiceMock.EXPECT().GetApplication(mock.Anything, testApp1ID).Return(mockApp1, nil)
	suite.appServiceMock.EXPECT().GetApplication(mock.Anything, testApp2ID).Return(mockApp2, nil)

	// Create a mock parameterizer that returns errors
	mockParameterizer := &MockParameterizer{
		shouldFail: true,
		errorMsg:   "template generation failed: unknown resource type",
	}

	// Create exporters with the test services
	exporters := []declarativeresource.ResourceExporter{
		application.NewApplicationExporterForTest(suite.appServiceMock),
		idp.NewIDPExporterForTest(suite.idpServiceMock),
		notification.NewNotificationSenderExporterForTest(suite.mockNotificationService),
		entitytype.NewEntityTypeExporterForTest(suite.mockEntityTypeService),
	}

	// Create a new export service with the mock parameterizer
	exportServiceWithMock := newExportService(exporters, mockParameterizer)

	result, err := exportServiceWithMock.ExportResources(context.Background(), request)

	// When all resources fail template generation, service returns error
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorNoResourcesFound.Code, err.Code)

	// Result should be nil when error is returned
	assert.Nil(suite.T(), result)
}

// TestExportResources_WithCustomFolderStructure tests the CustomStructure path in generateFolderPath.
func (suite *ExportServiceTestSuite) TestExportResources_WithCustomFolderStructure() {
	request := &ExportRequest{
		Applications: []string{testApp1ID},
		Options: &ExportOptions{
			Format: "yaml",
			FolderStructure: &FolderStructureOptions{
				CustomStructure: map[string]string{
					"application": "custom/apps/folder",
				},
			},
		},
	}

	mockApp := &appmodel.Application{
		ID:   testApp1ID,
		Name: "Test Application",
	}

	suite.appServiceMock.EXPECT().GetApplication(mock.Anything, testApp1ID).Return(mockApp, nil)

	result, err := suite.exportService.ExportResources(context.Background(), request)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Len(suite.T(), result.Files, 1)

	// Verify the file has custom folder path
	assert.Equal(suite.T(), "custom/apps/folder", result.Files[0].FolderPath)
}

// TestExportResources_WithGroupByTypeStructure tests the GroupByType path in generateFolderPath.
func (suite *ExportServiceTestSuite) TestExportResources_WithGroupByTypeStructure() {
	request := &ExportRequest{
		Applications:      []string{testApp1ID, testApp2ID},
		IdentityProviders: []string{"idp1"},
		Options: &ExportOptions{
			Format: "yaml",
			FolderStructure: &FolderStructureOptions{
				GroupByType: true,
			},
		},
	}

	mockApp1 := &appmodel.Application{
		ID:   testApp1ID,
		Name: "Application One",
	}

	mockApp2 := &appmodel.Application{
		ID:   testApp2ID,
		Name: "Application Two",
	}

	mockProperty, _ := cmodels.NewProperty("client_id", "test-client", false)
	mockIDP := &idp.IDPDTO{
		ID:         "idp1",
		Name:       "Test IDP",
		Type:       idp.IDPTypeGoogle,
		Properties: []cmodels.Property{*mockProperty},
	}

	suite.appServiceMock.EXPECT().GetApplication(mock.Anything, testApp1ID).Return(mockApp1, nil)
	suite.appServiceMock.EXPECT().GetApplication(mock.Anything, testApp2ID).Return(mockApp2, nil)
	suite.idpServiceMock.EXPECT().GetIdentityProvider(mock.Anything, "idp1").Return(mockIDP, nil)

	result, err := suite.exportService.ExportResources(context.Background(), request)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Len(suite.T(), result.Files, 3) // 2 apps + 1 IDP

	// Verify applications are in "applications" folder
	appFiles := 0
	idpFiles := 0
	for _, file := range result.Files {
		if file.ResourceType == "application" {
			assert.Equal(suite.T(), "applications", file.FolderPath)
			appFiles++
		} else if file.ResourceType == "identity_provider" {
			assert.Equal(suite.T(), "identity_providers", file.FolderPath)
			idpFiles++
		}
	}

	assert.Equal(suite.T(), 2, appFiles, "Should have 2 application files")
	assert.Equal(suite.T(), 1, idpFiles, "Should have 1 IDP file")
}

// TestExportNotificationSenders_Success tests successful export of notification senders.
func (suite *ExportServiceTestSuite) TestExportNotificationSenders_Success() {
	request := &ExportRequest{
		NotificationSenders: []string{"sender1"},
		Options: &ExportOptions{
			Format: "yaml",
		},
	}

	mockProperty, _ := cmodels.NewProperty("api_key", "test-api-key", true)
	mockSender := &common.NotificationSenderDTO{
		ID:          "sender1",
		Name:        "Test Sender",
		Description: "Test notification sender",
		Provider:    common.MessageProviderTypeTwilio,
		Properties:  []cmodels.Property{*mockProperty},
	}

	suite.mockNotificationService.EXPECT().GetSender(mock.Anything, "sender1").Return(mockSender, nil)

	result, err := suite.exportService.ExportResources(context.Background(), request)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Len(suite.T(), result.Files, 1)
	assert.NotNil(suite.T(), result.EnvFile)
	assert.Equal(suite.T(), 2, result.Summary.TotalFiles)
	assert.Contains(suite.T(), result.Summary.ResourceTypes, "notification_sender")
	assert.Equal(suite.T(), "Test_Sender.yaml", result.Files[0].FileName)
	assert.Equal(suite.T(), "notification_sender", result.Files[0].ResourceType)
	assert.Contains(suite.T(), result.Files[0].Content, "name: Test Sender")
}

// TestExportNotificationSenders_Multiple tests exporting multiple notification senders.
// nolint:dupl // Similar test pattern for different resource types
func (suite *ExportServiceTestSuite) TestExportNotificationSenders_Multiple() {
	mockProperty1, _ := cmodels.NewProperty("api_key", "key1", true)
	mockSender1 := &common.NotificationSenderDTO{
		ID:         "sender1",
		Name:       "Twilio Sender",
		Provider:   common.MessageProviderTypeTwilio,
		Properties: []cmodels.Property{*mockProperty1},
	}

	mockProperty2, _ := cmodels.NewProperty("api_key", "key2", true)
	mockSender2 := &common.NotificationSenderDTO{
		ID:         "sender2",
		Name:       "Vonage Sender",
		Provider:   common.MessageProviderTypeVonage,
		Properties: []cmodels.Property{*mockProperty2},
	}

	suite.mockNotificationService.EXPECT().GetSender(mock.Anything, "sender1").Return(mockSender1, nil)
	suite.mockNotificationService.EXPECT().GetSender(mock.Anything, "sender2").Return(mockSender2, nil)

	request := &ExportRequest{
		NotificationSenders: []string{"sender1", "sender2"},
		Options:             &ExportOptions{Format: "yaml"},
	}
	result, err := suite.exportService.ExportResources(context.Background(), request)

	suite.assertMultipleResourcesExport(result, err, 2, "notification_sender")
}

// TestExportNotificationSenders_Wildcard tests exporting all notification senders using wildcard.
func (suite *ExportServiceTestSuite) TestExportNotificationSenders_Wildcard() {
	request := &ExportRequest{
		NotificationSenders: []string{"*"},
		Options: &ExportOptions{
			Format: "yaml",
		},
	}

	mockProperty1, _ := cmodels.NewProperty("api_key", "key1", true)
	mockSender1 := &common.NotificationSenderDTO{
		ID:         "sender1",
		Name:       "Twilio Sender",
		Provider:   common.MessageProviderTypeTwilio,
		Properties: []cmodels.Property{*mockProperty1},
	}

	mockProperty2, _ := cmodels.NewProperty("api_key", "key2", true)
	mockSender2 := &common.NotificationSenderDTO{
		ID:         "sender2",
		Name:       "Vonage Sender",
		Provider:   common.MessageProviderTypeVonage,
		Properties: []cmodels.Property{*mockProperty2},
	}

	mockSenderList := []common.NotificationSenderDTO{*mockSender1, *mockSender2}

	suite.mockNotificationService.EXPECT().ListSenders(mock.Anything).Return(mockSenderList, nil)
	suite.mockNotificationService.EXPECT().GetSender(mock.Anything, "sender1").Return(mockSender1, nil)
	suite.mockNotificationService.EXPECT().GetSender(mock.Anything, "sender2").Return(mockSender2, nil)

	result, err := suite.exportService.ExportResources(context.Background(), request)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Len(suite.T(), result.Files, 2)
	assert.NotNil(suite.T(), result.EnvFile)
	assert.Equal(suite.T(), 3, result.Summary.TotalFiles)
}

// TestExportNotificationSenders_NotFound tests error handling when sender not found.
func (suite *ExportServiceTestSuite) TestExportNotificationSenders_NotFound() {
	request := &ExportRequest{
		NotificationSenders: []string{"non-existent-sender"},
		Options: &ExportOptions{
			Format: "yaml",
		},
	}

	senderError := &serviceerror.ServiceError{
		Code:  "SENDER_NOT_FOUND",
		Error: i18ncore.I18nMessage{DefaultValue: "Notification sender not found"},
	}

	suite.mockNotificationService.EXPECT().GetSender(mock.Anything, "non-existent-sender").Return(nil, senderError)

	result, err := suite.exportService.ExportResources(context.Background(), request)

	assert.NotNil(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), ErrorNoResourcesFound.Code, err.Code)
}

// TestExportNotificationSenders_EmptyName tests validation for sender with empty name.
func (suite *ExportServiceTestSuite) TestExportNotificationSenders_EmptyName() {
	request := &ExportRequest{
		NotificationSenders: []string{"sender-no-name"},
		Options: &ExportOptions{
			Format: "yaml",
		},
	}

	mockProperty, _ := cmodels.NewProperty("api_key", "test-key", true)
	mockSender := &common.NotificationSenderDTO{
		ID:         "sender-no-name",
		Name:       "", // Empty name
		Provider:   common.MessageProviderTypeTwilio,
		Properties: []cmodels.Property{*mockProperty},
	}

	suite.mockNotificationService.EXPECT().GetSender(mock.Anything, "sender-no-name").Return(mockSender, nil)

	result, err := suite.exportService.ExportResources(context.Background(), request)

	assert.NotNil(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), ErrorNoResourcesFound.Code, err.Code)
}

// TestExportNotificationSenders_NoProperties tests exporting sender with no properties.
func (suite *ExportServiceTestSuite) TestExportNotificationSenders_NoProperties() {
	request := &ExportRequest{
		NotificationSenders: []string{"sender-no-props"},
		Options: &ExportOptions{
			Format: "yaml",
		},
	}

	mockSender := &common.NotificationSenderDTO{
		ID:         "sender-no-props",
		Name:       "Empty Sender",
		Provider:   common.MessageProviderTypeTwilio,
		Properties: []cmodels.Property{}, // Empty properties
	}

	suite.mockNotificationService.EXPECT().GetSender(mock.Anything, "sender-no-props").Return(mockSender, nil)

	suite.assertExportNoProperties(request, "name: Empty Sender")
}

// TestExportNotificationSenders_WildcardPartialFailure tests wildcard export with partial failures.
func (suite *ExportServiceTestSuite) TestExportNotificationSenders_WildcardPartialFailure() {
	request := &ExportRequest{
		NotificationSenders: []string{"*"},
		Options: &ExportOptions{
			Format: "yaml",
		},
	}

	mockProperty1, _ := cmodels.NewProperty("api_key", "key1", true)
	mockSender1 := &common.NotificationSenderDTO{
		ID:         "sender1",
		Name:       "Twilio Sender",
		Provider:   common.MessageProviderTypeTwilio,
		Properties: []cmodels.Property{*mockProperty1},
	}

	mockProperty3, _ := cmodels.NewProperty("api_key", "key3", true)
	mockSender3 := &common.NotificationSenderDTO{
		ID:         "sender3",
		Name:       "Vonage Sender",
		Provider:   common.MessageProviderTypeVonage,
		Properties: []cmodels.Property{*mockProperty3},
	}

	// Create list with 3 senders but sender2 will fail to retrieve
	mockSenderList := []common.NotificationSenderDTO{*mockSender1, *mockSender3}

	suite.mockNotificationService.EXPECT().ListSenders(mock.Anything).Return(mockSenderList, nil)
	suite.mockNotificationService.EXPECT().GetSender(mock.Anything, "sender1").Return(mockSender1, nil)
	suite.mockNotificationService.EXPECT().GetSender(mock.Anything, "sender3").Return(mockSender3, nil)

	result, err := suite.exportService.ExportResources(context.Background(), request)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Len(suite.T(), result.Files, 2)
	assert.NotNil(suite.T(), result.EnvFile)
	assert.Equal(suite.T(), 3, result.Summary.TotalFiles)
	assert.Equal(suite.T(), 2, result.Summary.ResourceTypes["notification_sender"])
}

// TestExportEntityTypes_Success tests successful export of entity types.
func (suite *ExportServiceTestSuite) TestExportEntityTypes_Success() {
	request := &ExportRequest{
		UserTypes: []string{"schema1"},
		Options: &ExportOptions{
			Format: "yaml",
		},
	}

	mockSchema := &entitytype.EntityType{
		ID:                    "schema1",
		Name:                  "Test Schema",
		OUID:                  "ou1",
		AllowSelfRegistration: true,
		Schema:                []byte(`{"type":"object","properties":{"email":{"type":"string"}}}`),
	}

	suite.mockEntityTypeService.EXPECT().
		GetEntityType(mock.Anything, mock.Anything, "schema1", mock.Anything).
		Return(mockSchema, nil)

	result, err := suite.exportService.ExportResources(context.Background(), request)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Len(suite.T(), result.Files, 1)
	assert.Equal(suite.T(), 1, result.Summary.TotalFiles)
	assert.Contains(suite.T(), result.Summary.ResourceTypes, "user_type")
	assert.Equal(suite.T(), "Test_Schema.yaml", result.Files[0].FileName)
	assert.Equal(suite.T(), "user_type", result.Files[0].ResourceType)
	assert.Contains(suite.T(), result.Files[0].Content, "name: Test Schema")
}

// TestExportEntityTypes_Multiple tests exporting multiple entity types.
func (suite *ExportServiceTestSuite) TestExportEntityTypes_Multiple() {
	request := &ExportRequest{
		UserTypes: []string{"schema1", "schema2"},
		Options: &ExportOptions{
			Format: "yaml",
		},
	}

	mockSchema1 := &entitytype.EntityType{
		ID:                    "schema1",
		Name:                  "Customer Schema",
		OUID:                  "ou1",
		AllowSelfRegistration: true,
		Schema:                []byte(`{"type":"object","properties":{"email":{"type":"string"}}}`),
	}

	mockSchema2 := &entitytype.EntityType{
		ID:                    "schema2",
		Name:                  "Employee Schema",
		OUID:                  "ou1",
		AllowSelfRegistration: false,
		Schema:                []byte(`{"type":"object","properties":{"empId":{"type":"string"}}}`),
	}

	suite.mockEntityTypeService.EXPECT().
		GetEntityType(mock.Anything, mock.Anything, "schema1", mock.Anything).
		Return(mockSchema1, nil)
	suite.mockEntityTypeService.EXPECT().
		GetEntityType(mock.Anything, mock.Anything, "schema2", mock.Anything).
		Return(mockSchema2, nil)

	result, err := suite.exportService.ExportResources(context.Background(), request)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Len(suite.T(), result.Files, 2)
	assert.Equal(suite.T(), 2, result.Summary.TotalFiles)
	assert.Equal(suite.T(), 2, result.Summary.ResourceTypes["user_type"])
}

// TestExportEntityTypes_Wildcard tests exporting all entity types using wildcard.
func (suite *ExportServiceTestSuite) TestExportEntityTypes_Wildcard() {
	request := &ExportRequest{
		UserTypes: []string{"*"},
		Options: &ExportOptions{
			Format: "yaml",
		},
	}

	mockSchema1 := &entitytype.EntityType{
		ID:                    "schema1",
		Name:                  "Customer Schema",
		OUID:                  "ou1",
		AllowSelfRegistration: true,
		Schema:                []byte(`{"type":"object","properties":{"email":{"type":"string"}}}`),
	}

	mockSchema2 := &entitytype.EntityType{
		ID:                    "schema2",
		Name:                  "Employee Schema",
		OUID:                  "ou1",
		AllowSelfRegistration: false,
		Schema:                []byte(`{"type":"object","properties":{"empId":{"type":"string"}}}`),
	}

	mockSchemaList := &entitytype.EntityTypeListResponse{
		TotalResults: 2,
		Count:        2,
		Types: []entitytype.EntityTypeListItem{
			{ID: "schema1", Name: "Customer Schema", OUID: "ou1"},
			{ID: "schema2", Name: "Employee Schema", OUID: "ou1"},
		},
	}

	suite.mockEntityTypeService.EXPECT().
		GetEntityTypeList(mock.Anything, mock.Anything, 100, 0, mock.Anything).Return(mockSchemaList, nil)
	suite.mockEntityTypeService.EXPECT().
		GetEntityType(mock.Anything, mock.Anything, "schema1", mock.Anything).
		Return(mockSchema1, nil)
	suite.mockEntityTypeService.EXPECT().
		GetEntityType(mock.Anything, mock.Anything, "schema2", mock.Anything).
		Return(mockSchema2, nil)

	result, err := suite.exportService.ExportResources(context.Background(), request)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Len(suite.T(), result.Files, 2)
	assert.Equal(suite.T(), 2, result.Summary.TotalFiles)
}

// TestExportEntityTypes_NotFound tests error handling when schema not found.
func (suite *ExportServiceTestSuite) TestExportEntityTypes_NotFound() {
	request := &ExportRequest{
		UserTypes: []string{"non-existent-schema"},
		Options: &ExportOptions{
			Format: "yaml",
		},
	}

	schemaError := &serviceerror.ServiceError{
		Code:  "SCHEMA_NOT_FOUND",
		Error: i18ncore.I18nMessage{DefaultValue: "User type not found"},
	}

	suite.mockEntityTypeService.EXPECT().
		GetEntityType(
			mock.Anything, mock.Anything, "non-existent-schema", mock.Anything).
		Return(nil, schemaError)

	result, err := suite.exportService.ExportResources(context.Background(), request)

	assert.NotNil(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), ErrorNoResourcesFound.Code, err.Code)
}

// TestExportEntityTypes_EmptyName tests validation for schema with empty name.
func (suite *ExportServiceTestSuite) TestExportEntityTypes_EmptyName() {
	request := &ExportRequest{
		UserTypes: []string{"schema-no-name"},
		Options: &ExportOptions{
			Format: "yaml",
		},
	}

	mockSchema := &entitytype.EntityType{
		ID:                    "schema-no-name",
		Name:                  "", // Empty name
		OUID:                  "ou1",
		AllowSelfRegistration: true,
		Schema:                []byte(`{"type":"object"}`),
	}

	suite.mockEntityTypeService.EXPECT().
		GetEntityType(
			mock.Anything, mock.Anything, "schema-no-name", mock.Anything).
		Return(mockSchema, nil)

	result, err := suite.exportService.ExportResources(context.Background(), request)

	assert.NotNil(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), ErrorNoResourcesFound.Code, err.Code)
}

// TestExportEntityTypes_NoSchema tests exporting schema with no schema definition.
func (suite *ExportServiceTestSuite) TestExportEntityTypes_NoSchema() {
	request := &ExportRequest{
		UserTypes: []string{"schema-no-def"},
		Options: &ExportOptions{
			Format: "yaml",
		},
	}

	mockSchema := &entitytype.EntityType{
		ID:                    "schema-no-def",
		Name:                  "Empty Schema",
		OUID:                  "ou1",
		AllowSelfRegistration: true,
		Schema:                []byte{}, // Empty schema
	}

	suite.mockEntityTypeService.EXPECT().
		GetEntityType(
			mock.Anything, mock.Anything, "schema-no-def", mock.Anything).
		Return(mockSchema, nil)

	result, err := suite.exportService.ExportResources(context.Background(), request)

	// Should succeed even with no schema definition
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Len(suite.T(), result.Files, 1)
	assert.Equal(suite.T(), 1, result.Summary.TotalFiles)
	assert.Contains(suite.T(), result.Files[0].Content, "name: Empty Schema")
}

// TestExportEntityTypes_WildcardPartialFailure tests wildcard export with partial failures.
func (suite *ExportServiceTestSuite) TestExportEntityTypes_WildcardPartialFailure() {
	request := &ExportRequest{
		UserTypes: []string{"*"},
		Options: &ExportOptions{
			Format: "yaml",
		},
	}

	mockSchema1 := &entitytype.EntityType{
		ID:                    "schema1",
		Name:                  "Customer Schema",
		OUID:                  "ou1",
		AllowSelfRegistration: true,
		Schema:                []byte(`{"type":"object"}`),
	}

	mockSchema3 := &entitytype.EntityType{
		ID:                    "schema3",
		Name:                  "Partner Schema",
		OUID:                  "ou1",
		AllowSelfRegistration: false,
		Schema:                []byte(`{"type":"object"}`),
	}

	mockSchemaList := &entitytype.EntityTypeListResponse{
		TotalResults: 3,
		Count:        3,
		Types: []entitytype.EntityTypeListItem{
			{ID: "schema1", Name: "Customer Schema"},
			{ID: "schema2", Name: "Employee Schema"},
			{ID: "schema3", Name: "Partner Schema"},
		},
	}

	schemaError := &serviceerror.ServiceError{
		Code:  "SCHEMA_NOT_FOUND",
		Error: i18ncore.I18nMessage{DefaultValue: "User type not found"},
	}

	suite.mockEntityTypeService.EXPECT().
		GetEntityTypeList(mock.Anything, mock.Anything, 100, 0, mock.Anything).Return(mockSchemaList, nil)
	suite.mockEntityTypeService.EXPECT().
		GetEntityType(mock.Anything, mock.Anything, "schema1", mock.Anything).
		Return(mockSchema1, nil)
	suite.mockEntityTypeService.EXPECT().
		GetEntityType(mock.Anything, mock.Anything, "schema2", mock.Anything).
		Return(nil, schemaError)
	suite.mockEntityTypeService.EXPECT().
		GetEntityType(mock.Anything, mock.Anything, "schema3", mock.Anything).
		Return(mockSchema3, nil)

	result, err := suite.exportService.ExportResources(context.Background(), request)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Len(suite.T(), result.Files, 2) // 2 successful exports
	assert.Equal(suite.T(), 2, result.Summary.TotalFiles)
	assert.Equal(suite.T(), 2, result.Summary.ResourceTypes["user_type"])
	assert.Len(suite.T(), result.Summary.Errors, 1) // One error recorded
	assert.Equal(suite.T(), "user_type", result.Summary.Errors[0].ResourceType)
	assert.Equal(suite.T(), "schema2", result.Summary.Errors[0].ResourceID)
}

// Helper functions for test assertions

// assertMultipleResourcesExport is a helper function to assert multiple resource export results.
func (suite *ExportServiceTestSuite) assertMultipleResourcesExport(
	result *ExportResponse, err *serviceerror.ServiceError, expectedCount int, resourceTypeKey string) {
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Len(suite.T(), result.Files, expectedCount)
	expectedTotalFiles := expectedCount
	if result.EnvFile != nil {
		expectedTotalFiles++
	}
	assert.Equal(suite.T(), expectedTotalFiles, result.Summary.TotalFiles)
	assert.Equal(suite.T(), expectedCount, result.Summary.ResourceTypes[resourceTypeKey])
}

// Tests for exportResourcesWithExporter

// TestExportResourcesWithExporter_Success tests successful export with a valid exporter.
func (suite *ExportServiceTestSuite) TestExportResourcesWithExporter_Success() {
	appID := testAppTestID
	mockApp := &appmodel.Application{
		ID:          appID,
		Name:        "Test Application",
		Description: "Test Description",
	}

	suite.appServiceMock.EXPECT().GetApplication(mock.Anything, appID).Return(mockApp, nil)

	// Get the exporter from the service's registry
	exporter, exists := suite.exportService.(*exportService).registry.Get(resourceTypeApplication)
	assert.True(suite.T(), exists, "Application exporter should be registered")

	options := &ExportOptions{
		Format: formatYAML,
	}

	files, variables, errors := suite.exportService.(*exportService).exportResourcesWithExporter(context.Background(),
		exporter, []string{appID}, options)

	assert.Len(suite.T(), files, 1)
	assert.Len(suite.T(), errors, 0)
	assert.Equal(suite.T(), "Test_Application.yaml", files[0].FileName)
	assert.Equal(suite.T(), resourceTypeApplication, files[0].ResourceType)
	assert.Equal(suite.T(), appID, files[0].ResourceID)
	assert.Contains(suite.T(), files[0].Content, "name: Test Application")
	assert.Empty(suite.T(), variables)
}

// TestExportResourcesWithExporter_MultipleResources tests exporting multiple resources.
func (suite *ExportServiceTestSuite) TestExportResourcesWithExporter_MultipleResources() {
	app1ID := "app-1"
	app2ID := "app-2"
	app3ID := "app-3"

	mockApp1 := &appmodel.Application{
		ID:          app1ID,
		Name:        "Application One",
		Description: "First App",
	}

	mockApp2 := &appmodel.Application{
		ID:          app2ID,
		Name:        "Application Two",
		Description: "Second App",
	}

	mockApp3 := &appmodel.Application{
		ID:          app3ID,
		Name:        "Application Three",
		Description: "Third App",
	}

	suite.appServiceMock.EXPECT().GetApplication(mock.Anything, app1ID).Return(mockApp1, nil)
	suite.appServiceMock.EXPECT().GetApplication(mock.Anything, app2ID).Return(mockApp2, nil)
	suite.appServiceMock.EXPECT().GetApplication(mock.Anything, app3ID).Return(mockApp3, nil)

	exporter, _ := suite.exportService.(*exportService).registry.Get(resourceTypeApplication)
	options := &ExportOptions{Format: formatYAML}

	files, variables, errors := suite.exportService.(*exportService).exportResourcesWithExporter(context.Background(),
		exporter, []string{app1ID, app2ID, app3ID}, options)

	assert.Len(suite.T(), files, 3)
	assert.Len(suite.T(), errors, 0)
	assert.Equal(suite.T(), "Application_One.yaml", files[0].FileName)
	assert.Equal(suite.T(), "Application_Two.yaml", files[1].FileName)
	assert.Equal(suite.T(), "Application_Three.yaml", files[2].FileName)
	assert.Empty(suite.T(), variables)
}

// TestExportResourcesWithExporter_ResourceNotFound tests when a resource is not found.
func (suite *ExportServiceTestSuite) TestExportResourcesWithExporter_ResourceNotFound() {
	appID := "non-existent-app"
	appError := &serviceerror.ServiceError{
		Code:  "APP_NOT_FOUND",
		Error: i18ncore.I18nMessage{DefaultValue: "Application not found"},
	}

	suite.appServiceMock.EXPECT().GetApplication(mock.Anything, appID).Return(nil, appError)

	exporter, _ := suite.exportService.(*exportService).registry.Get(resourceTypeApplication)
	options := &ExportOptions{Format: formatYAML}

	files, variables, errors := suite.exportService.(*exportService).exportResourcesWithExporter(context.Background(),
		exporter, []string{appID}, options)

	assert.Len(suite.T(), files, 0)
	assert.Len(suite.T(), errors, 1)
	assert.Equal(suite.T(), resourceTypeApplication, errors[0].ResourceType)
	assert.Equal(suite.T(), appID, errors[0].ResourceID)
	assert.Equal(suite.T(), "APP_NOT_FOUND", errors[0].Code)
	assert.Equal(suite.T(), "Application not found", errors[0].Error)
	assert.Empty(suite.T(), variables)
}

// TestExportResourcesWithExporter_PartialSuccess tests when some resources succeed and some fail.
func (suite *ExportServiceTestSuite) TestExportResourcesWithExporter_PartialSuccess() {
	validAppID := "valid-app"
	invalidAppID := "invalid-app"

	mockApp := &appmodel.Application{
		ID:          validAppID,
		Name:        "Valid Application",
		Description: "Valid Description",
	}

	appError := &serviceerror.ServiceError{
		Code:  "APP_NOT_FOUND",
		Error: i18ncore.I18nMessage{DefaultValue: "Application not found"},
	}

	suite.appServiceMock.EXPECT().GetApplication(mock.Anything, validAppID).Return(mockApp, nil)
	suite.appServiceMock.EXPECT().GetApplication(mock.Anything, invalidAppID).Return(nil, appError)

	exporter, _ := suite.exportService.(*exportService).registry.Get(resourceTypeApplication)
	options := &ExportOptions{Format: formatYAML}

	files, variables, errors := suite.exportService.(*exportService).exportResourcesWithExporter(context.Background(),
		exporter, []string{validAppID, invalidAppID}, options)

	assert.Len(suite.T(), files, 1)
	assert.Len(suite.T(), errors, 1)
	assert.Equal(suite.T(), "Valid_Application.yaml", files[0].FileName)
	assert.Equal(suite.T(), resourceTypeApplication, errors[0].ResourceType)
	assert.Equal(suite.T(), invalidAppID, errors[0].ResourceID)
	assert.Empty(suite.T(), variables)
}

// TestExportResourcesWithExporter_WildcardSuccess tests wildcard export.
func (suite *ExportServiceTestSuite) TestExportResourcesWithExporter_WildcardSuccess() {
	mockAppList := &appmodel.ApplicationListResponse{
		TotalResults: 2,
		Count:        2,
		Applications: []appmodel.BasicApplicationResponse{
			{ID: testApp1ID, Name: "App One"},
			{ID: testApp2ID, Name: "App Two"},
		},
	}

	mockApp1 := &appmodel.Application{
		ID:          testApp1ID,
		Name:        "App One",
		Description: "First App",
	}

	mockApp2 := &appmodel.Application{
		ID:          testApp2ID,
		Name:        "App Two",
		Description: "Second App",
	}

	suite.appServiceMock.EXPECT().GetApplicationList(mock.Anything).Return(mockAppList, nil)
	suite.appServiceMock.EXPECT().GetApplication(mock.Anything, testApp1ID).Return(mockApp1, nil)
	suite.appServiceMock.EXPECT().GetApplication(mock.Anything, testApp2ID).Return(mockApp2, nil)

	exporter, _ := suite.exportService.(*exportService).registry.Get(resourceTypeApplication)
	options := &ExportOptions{Format: formatYAML}

	files, variables, errors := suite.exportService.(*exportService).exportResourcesWithExporter(context.Background(),
		exporter, []string{"*"}, options)

	assert.Len(suite.T(), files, 2)
	assert.Len(suite.T(), errors, 0)
	assert.Equal(suite.T(), "App_One.yaml", files[0].FileName)
	assert.Equal(suite.T(), "App_Two.yaml", files[1].FileName)
	assert.Empty(suite.T(), variables)
}

// TestExportResourcesWithExporter_WildcardFailure tests wildcard when GetAllResourceIDs fails.
func (suite *ExportServiceTestSuite) TestExportResourcesWithExporter_WildcardFailure() {
	listError := &serviceerror.ServiceError{
		Code:  "LIST_FAILED",
		Error: i18ncore.I18nMessage{DefaultValue: "Failed to list applications"},
	}

	suite.appServiceMock.EXPECT().GetApplicationList(mock.Anything).Return(nil, listError)

	exporter, _ := suite.exportService.(*exportService).registry.Get(resourceTypeApplication)
	options := &ExportOptions{Format: formatYAML}

	files, variables, errors := suite.exportService.(*exportService).exportResourcesWithExporter(context.Background(),
		exporter, []string{"*"}, options)

	assert.Len(suite.T(), files, 0)
	assert.Len(suite.T(), errors, 0) // Returns empty slices on wildcard failure
	assert.Empty(suite.T(), variables)
}

// TestExportResourcesWithExporter_WildcardEmptyList tests wildcard with no resources.
func (suite *ExportServiceTestSuite) TestExportResourcesWithExporter_WildcardEmptyList() {
	mockAppList := &appmodel.ApplicationListResponse{
		TotalResults: 0,
		Count:        0,
		Applications: []appmodel.BasicApplicationResponse{},
	}

	suite.appServiceMock.EXPECT().GetApplicationList(mock.Anything).Return(mockAppList, nil)

	exporter, _ := suite.exportService.(*exportService).registry.Get(resourceTypeApplication)
	options := &ExportOptions{Format: formatYAML}

	files, variables, errors := suite.exportService.(*exportService).exportResourcesWithExporter(context.Background(),
		exporter, []string{"*"}, options)

	assert.Len(suite.T(), files, 0)
	assert.Len(suite.T(), errors, 0)
	assert.Empty(suite.T(), variables)
}

// TestExportResourcesWithExporter_WithGroupByType tests export with GroupByType option.
func (suite *ExportServiceTestSuite) TestExportResourcesWithExporter_WithGroupByType() {
	appID := testAppTestID
	mockApp := &appmodel.Application{
		ID:          appID,
		Name:        "Test App",
		Description: "Test Description",
	}

	suite.appServiceMock.EXPECT().GetApplication(mock.Anything, appID).Return(mockApp, nil)

	exporter, _ := suite.exportService.(*exportService).registry.Get(resourceTypeApplication)
	options := &ExportOptions{
		Format: formatYAML,
		FolderStructure: &FolderStructureOptions{
			GroupByType: true,
		},
	}

	files, variables, errors := suite.exportService.(*exportService).exportResourcesWithExporter(context.Background(),
		exporter, []string{appID}, options)

	assert.Len(suite.T(), files, 1)
	assert.Len(suite.T(), errors, 0)
	assert.Equal(suite.T(), "applications", files[0].FolderPath)
	assert.Empty(suite.T(), variables)
}

// TestExportResourcesWithExporter_WithCustomFileNaming tests export with custom file naming pattern.
func (suite *ExportServiceTestSuite) TestExportResourcesWithExporter_WithCustomFileNaming() {
	appID := "app-123"
	mockApp := &appmodel.Application{
		ID:          appID,
		Name:        "My Application",
		Description: "Test Description",
	}

	suite.appServiceMock.EXPECT().GetApplication(mock.Anything, appID).Return(mockApp, nil)

	exporter, _ := suite.exportService.(*exportService).registry.Get(resourceTypeApplication)
	options := &ExportOptions{
		Format: formatYAML,
		FolderStructure: &FolderStructureOptions{
			FileNamingPattern: "${name}_${id}",
		},
	}

	files, variables, errors := suite.exportService.(*exportService).exportResourcesWithExporter(context.Background(),
		exporter, []string{appID}, options)

	assert.Len(suite.T(), files, 1)
	assert.Len(suite.T(), errors, 0)
	assert.Equal(suite.T(), "My_Application_app-123.yaml", files[0].FileName)
	assert.Empty(suite.T(), variables)
}

// TestExportResourcesWithExporter_IdentityProvider tests export with IDP exporter.
func (suite *ExportServiceTestSuite) TestExportResourcesWithExporter_IdentityProvider() {
	idpID := "idp-test-id"
	mockIDP := &idp.IDPDTO{
		ID:          idpID,
		Name:        "Test IDP",
		Description: "Test IDP Description",
	}

	suite.idpServiceMock.EXPECT().GetIdentityProvider(mock.Anything, idpID).Return(mockIDP, nil)

	exporter, exists := suite.exportService.(*exportService).registry.Get(resourceTypeIdentityProvider)
	assert.True(suite.T(), exists, "IDP exporter should be registered")

	options := &ExportOptions{Format: formatYAML}

	files, variables, errors := suite.exportService.(*exportService).exportResourcesWithExporter(context.Background(),
		exporter, []string{idpID}, options)

	assert.Len(suite.T(), files, 1)
	assert.Len(suite.T(), errors, 0)
	assert.Equal(suite.T(), "Test_IDP.yaml", files[0].FileName)
	assert.Equal(suite.T(), resourceTypeIdentityProvider, files[0].ResourceType)
	assert.Equal(suite.T(), idpID, files[0].ResourceID)
	assert.Empty(suite.T(), variables)
}

// TestExportResourcesWithExporter_NotificationSender tests export with notification sender exporter.
func (suite *ExportServiceTestSuite) TestExportResourcesWithExporter_NotificationSender() {
	senderID := "sender-test-id"
	mockProperty, _ := cmodels.NewProperty("api_key", "key1", true)
	mockSender := &common.NotificationSenderDTO{
		ID:         senderID,
		Name:       "Test Sender",
		Provider:   common.MessageProviderTypeTwilio,
		Properties: []cmodels.Property{*mockProperty},
	}

	suite.mockNotificationService.EXPECT().GetSender(mock.Anything, senderID).Return(mockSender, nil)

	exporter, exists := suite.exportService.(*exportService).registry.Get(resourceTypeNotificationSender)
	assert.True(suite.T(), exists, "Notification sender exporter should be registered")

	options := &ExportOptions{Format: formatYAML}

	files, variables, errors := suite.exportService.(*exportService).exportResourcesWithExporter(context.Background(),
		exporter, []string{senderID}, options)

	assert.Len(suite.T(), files, 1)
	assert.Len(suite.T(), errors, 0)
	assert.Equal(suite.T(), "Test_Sender.yaml", files[0].FileName)
	assert.Equal(suite.T(), resourceTypeNotificationSender, files[0].ResourceType)
	assert.Equal(suite.T(), senderID, files[0].ResourceID)
	assert.NotEmpty(suite.T(), variables)
	assert.Equal(suite.T(), "key1", variables["TEST_SENDER_API_KEY"])
}

// TestExportResourcesWithExporter_EntityType tests export with entity type exporter.
func (suite *ExportServiceTestSuite) TestExportResourcesWithExporter_EntityType() {
	schemaID := "schema-test-id"
	mockSchema := &entitytype.EntityType{
		ID:                    schemaID,
		Name:                  "Test Schema",
		OUID:                  "ou1",
		AllowSelfRegistration: true,
		Schema:                []byte(`{"type":"object","properties":{"email":{"type":"string"}}}`),
	}

	suite.mockEntityTypeService.EXPECT().
		GetEntityType(mock.Anything, mock.Anything, schemaID, mock.Anything).
		Return(mockSchema, nil)

	exporter, exists := suite.exportService.(*exportService).registry.Get(resourceTypeUserType)
	assert.True(suite.T(), exists, "Entity type exporter should be registered")

	options := &ExportOptions{Format: formatYAML}

	files, variables, errors := suite.exportService.(*exportService).exportResourcesWithExporter(context.Background(),
		exporter, []string{schemaID}, options)

	assert.Len(suite.T(), files, 1)
	assert.Len(suite.T(), errors, 0)
	assert.Equal(suite.T(), "Test_Schema.yaml", files[0].FileName)
	assert.Equal(suite.T(), resourceTypeUserType, files[0].ResourceType)
	assert.Equal(suite.T(), schemaID, files[0].ResourceID)
	assert.Empty(suite.T(), variables)
}

// TestExportResourcesWithExporter_EmptyResourceIDs tests export with empty resource ID list.
func (suite *ExportServiceTestSuite) TestExportResourcesWithExporter_EmptyResourceIDs() {
	exporter, _ := suite.exportService.(*exportService).registry.Get(resourceTypeApplication)
	options := &ExportOptions{Format: formatYAML}

	files, variables, errors := suite.exportService.(*exportService).exportResourcesWithExporter(context.Background(),
		exporter, []string{}, options)

	assert.Len(suite.T(), files, 0)
	assert.Len(suite.T(), errors, 0)
	assert.Empty(suite.T(), variables)
}

// TestExportResourcesWithExporter_JSONFormatFallback tests that JSON format falls back to YAML.
func (suite *ExportServiceTestSuite) TestExportResourcesWithExporter_JSONFormatFallback() {
	appID := testAppTestID
	mockApp := &appmodel.Application{
		ID:          appID,
		Name:        "Test App",
		Description: "Test Description",
	}

	suite.appServiceMock.EXPECT().GetApplication(mock.Anything, appID).Return(mockApp, nil)

	exporter, _ := suite.exportService.(*exportService).registry.Get(resourceTypeApplication)
	options := &ExportOptions{
		Format: formatJSON, // JSON not yet implemented
	}

	files, variables, errors := suite.exportService.(*exportService).exportResourcesWithExporter(context.Background(),
		exporter, []string{appID}, options)

	assert.Len(suite.T(), files, 1)
	assert.Len(suite.T(), errors, 0)
	// Should fall back to YAML format
	assert.Equal(suite.T(), "Test_App.yaml", files[0].FileName)
	assert.Contains(suite.T(), files[0].Content, "name: Test App")
	assert.Empty(suite.T(), variables)
}

// TestExportResourcesWithExporter_Flow tests export with flow exporter.
func (suite *ExportServiceTestSuite) TestExportResourcesWithExporter_Flow() {
	flowID := testFlowID
	mockFlow := &flowmgt.CompleteFlowDefinition{
		ID:            flowID,
		Handle:        "basic-auth-flow",
		Name:          "Basic Authentication Flow",
		FlowType:      flowcommon.FlowType("AUTHENTICATION"),
		ActiveVersion: 1,
		Nodes: []flowmgt.NodeDefinition{
			{
				ID:        "start",
				Type:      "START",
				OnSuccess: "login",
			},
			{
				ID:   "login",
				Type: "BASIC_AUTHENTICATION",
			},
			{
				ID:   "end",
				Type: "END",
			},
		},
		CreatedAt: "2025-12-22 10:00:00",
		UpdatedAt: "2025-12-22 10:00:00",
	}

	suite.mockFlowService.EXPECT().GetFlow(mock.Anything, flowID).Return(mockFlow, nil)

	exporter, exists := suite.exportService.(*exportService).registry.Get("flow")
	assert.True(suite.T(), exists, "Flow exporter should be registered")

	options := &ExportOptions{Format: formatYAML}

	files, variables, errors := suite.exportService.(*exportService).exportResourcesWithExporter(context.Background(),
		exporter, []string{flowID}, options)

	assert.Len(suite.T(), files, 1)
	assert.Len(suite.T(), errors, 0)
	assert.Equal(suite.T(), "Basic_Authentication_Flow.yaml", files[0].FileName)
	assert.Equal(suite.T(), "flow", files[0].ResourceType)
	assert.Equal(suite.T(), flowID, files[0].ResourceID)
	assert.Contains(suite.T(), files[0].Content, "handle: basic-auth-flow")
	assert.Contains(suite.T(), files[0].Content, "flowType: AUTHENTICATION")
	assert.Empty(suite.T(), variables)
}

// TestExportResourcesWithExporter_FlowWithComplexMeta tests export with flow containing complex meta.
func (suite *ExportServiceTestSuite) TestExportResourcesWithExporter_FlowWithComplexMeta() {
	flowID := "flow-with-meta"
	complexMeta := map[string]interface{}{
		"components": []interface{}{
			map[string]interface{}{
				"id":      "text_001",
				"label":   "{{ t(signin:heading) }}",
				"type":    "TEXT",
				"variant": "HEADING_1",
			},
			map[string]interface{}{
				"id":   "block_001",
				"type": "BLOCK",
				"components": []interface{}{
					map[string]interface{}{
						"id":          "input_001",
						"label":       "Username",
						"placeholder": "Enter username",
						"ref":         "username",
						"required":    true,
						"type":        "TEXT_INPUT",
					},
					map[string]interface{}{
						"id":          "input_002",
						"label":       "Password",
						"placeholder": "Enter password",
						"ref":         "password",
						"required":    true,
						"type":        "PASSWORD_INPUT",
					},
				},
			},
		},
		"theme": map[string]interface{}{
			"primaryColor":   "#0066cc",
			"secondaryColor": "#6c757d",
		},
	}

	mockFlow := &flowmgt.CompleteFlowDefinition{
		ID:            flowID,
		Handle:        "prompt-flow",
		Name:          "Flow with Complex Meta",
		FlowType:      flowcommon.FlowType("AUTHENTICATION"),
		ActiveVersion: 1,
		Nodes: []flowmgt.NodeDefinition{
			{
				ID:        "start",
				Type:      "START",
				OnSuccess: "prompt",
			},
			{
				ID:   "prompt",
				Type: "PROMPT",
				Meta: complexMeta,
				Prompts: []flowmgt.PromptDefinition{
					{
						Inputs: []flowmgt.InputDefinition{
							{Ref: "input_001", Type: "TEXT_INPUT", Identifier: "username", Required: true},
							{Ref: "input_002", Type: "PASSWORD_INPUT", Identifier: "password", Required: true},
						},
						Action: &flowmgt.ActionDefinition{Ref: "action_001", NextNode: "end"},
					},
				},
			},
			{
				ID:   "end",
				Type: "END",
			},
		},
		CreatedAt: "2025-12-22 10:00:00",
		UpdatedAt: "2025-12-22 10:00:00",
	}

	suite.mockFlowService.EXPECT().GetFlow(mock.Anything, flowID).Return(mockFlow, nil)

	exporter, exists := suite.exportService.(*exportService).registry.Get("flow")
	assert.True(suite.T(), exists)

	options := &ExportOptions{Format: formatYAML}

	files, variables, errors := suite.exportService.(*exportService).exportResourcesWithExporter(context.Background(),
		exporter, []string{flowID}, options)

	assert.Len(suite.T(), files, 1)
	assert.Len(suite.T(), errors, 0)
	assert.Equal(suite.T(), "Flow_with_Complex_Meta.yaml", files[0].FileName)
	assert.Contains(suite.T(), files[0].Content, "handle: prompt-flow")
	assert.Contains(suite.T(), files[0].Content, "meta:")
	// Meta should be present in some form (either as JSON string or YAML structure)
	assert.Contains(suite.T(), files[0].Content, "prompt")
	assert.Empty(suite.T(), variables)
}

// TestExportResourcesWithExporter_MultipleFlows tests exporting multiple flows.
func (suite *ExportServiceTestSuite) TestExportResourcesWithExporter_MultipleFlows() {
	flow1 := &flowmgt.CompleteFlowDefinition{
		ID:            testFlow1ID,
		Handle:        "flow-1",
		Name:          "Flow One",
		FlowType:      flowcommon.FlowType("AUTHENTICATION"),
		ActiveVersion: 1,
		Nodes: []flowmgt.NodeDefinition{
			{ID: "start", Type: "START"},
			{ID: "end", Type: "END"},
		},
	}

	flow2 := &flowmgt.CompleteFlowDefinition{
		ID:            testFlow2ID,
		Handle:        "flow-2",
		Name:          "Flow Two",
		FlowType:      flowcommon.FlowType("AUTHORIZATION"),
		ActiveVersion: 2,
		Nodes: []flowmgt.NodeDefinition{
			{ID: "start", Type: "START"},
			{ID: "check", Type: "AUTHORIZATION_CHECK"},
			{ID: "end", Type: "END"},
		},
	}

	suite.mockFlowService.EXPECT().GetFlow(mock.Anything, testFlow1ID).Return(flow1, nil)
	suite.mockFlowService.EXPECT().GetFlow(mock.Anything, testFlow2ID).Return(flow2, nil)

	exporter, _ := suite.exportService.(*exportService).registry.Get("flow")
	options := &ExportOptions{Format: formatYAML}

	files, variables, errors := suite.exportService.(*exportService).exportResourcesWithExporter(context.Background(),
		exporter, []string{testFlow1ID, testFlow2ID}, options)

	assert.Len(suite.T(), files, 2)
	assert.Len(suite.T(), errors, 0)
	assert.Equal(suite.T(), "Flow_One.yaml", files[0].FileName)
	assert.Equal(suite.T(), "Flow_Two.yaml", files[1].FileName)
	assert.Empty(suite.T(), variables)
}

// TestExportResourcesWithExporter_FlowNotFound tests export when flow is not found.
func (suite *ExportServiceTestSuite) TestExportResourcesWithExporter_FlowNotFound() {
	flowID := "non-existent-flow"
	flowError := &serviceerror.ServiceError{
		Code:  "FLOW_NOT_FOUND",
		Error: i18ncore.I18nMessage{DefaultValue: "Flow not found"},
	}
	suite.mockFlowService.EXPECT().GetFlow(mock.Anything, flowID).Return(nil, flowError)

	exporter, _ := suite.exportService.(*exportService).registry.Get("flow")
	options := &ExportOptions{Format: formatYAML}

	files, variables, errors := suite.exportService.(*exportService).exportResourcesWithExporter(context.Background(),
		exporter, []string{flowID}, options)

	assert.Len(suite.T(), files, 0)
	assert.Len(suite.T(), errors, 1)
	assert.Equal(suite.T(), "flow", errors[0].ResourceType)
	assert.Equal(suite.T(), flowID, errors[0].ResourceID)
	assert.Contains(suite.T(), errors[0].Error, "Flow not found")
	assert.Empty(suite.T(), variables)
}

// TestExportResourcesWithExporter_WildcardFlows tests wildcard export for flows.
func (suite *ExportServiceTestSuite) TestExportResourcesWithExporter_WildcardFlows() {
	flowList := &flowmgt.FlowListResponse{
		TotalResults: 2,
		StartIndex:   0,
		Count:        2,
		Flows: []flowmgt.BasicFlowDefinition{
			{
				ID:            testFlow1ID,
				Handle:        "flow-1",
				Name:          "Flow One",
				FlowType:      flowcommon.FlowType("AUTHENTICATION"),
				ActiveVersion: 1,
			},
			{
				ID:            testFlow2ID,
				Handle:        "flow-2",
				Name:          "Flow Two",
				FlowType:      flowcommon.FlowType("AUTHORIZATION"),
				ActiveVersion: 1,
			},
		},
	}

	flow1Complete := &flowmgt.CompleteFlowDefinition{
		ID:            testFlow1ID,
		Handle:        "flow-1",
		Name:          "Flow One",
		FlowType:      flowcommon.FlowType("AUTHENTICATION"),
		ActiveVersion: 1,
		Nodes: []flowmgt.NodeDefinition{
			{ID: "start", Type: "START"},
			{ID: "end", Type: "END"},
		},
	}

	flow2Complete := &flowmgt.CompleteFlowDefinition{
		ID:            testFlow2ID,
		Handle:        "flow-2",
		Name:          "Flow Two",
		FlowType:      flowcommon.FlowType("AUTHORIZATION"),
		ActiveVersion: 1,
		Nodes: []flowmgt.NodeDefinition{
			{ID: "start", Type: "START"},
			{ID: "end", Type: "END"},
		},
	}

	suite.mockFlowService.EXPECT().ListFlows(mock.Anything, 10000, 0, flowcommon.FlowType("")).Return(flowList, nil)
	suite.mockFlowService.EXPECT().GetFlow(mock.Anything, testFlow1ID).Return(flow1Complete, nil)
	suite.mockFlowService.EXPECT().GetFlow(mock.Anything, testFlow2ID).Return(flow2Complete, nil)

	exporter, _ := suite.exportService.(*exportService).registry.Get("flow")
	options := &ExportOptions{Format: formatYAML}

	files, variables, errors := suite.exportService.(*exportService).exportResourcesWithExporter(context.Background(),
		exporter, []string{"*"}, options)

	assert.Len(suite.T(), files, 2)
	assert.Len(suite.T(), errors, 0)
	assert.Empty(suite.T(), variables)
}

// TestExportResourcesWithExporter_WildcardFlows_ListFailure tests wildcard when ListFlows fails.
func (suite *ExportServiceTestSuite) TestExportResourcesWithExporter_WildcardFlows_ListFailure() {
	dbError := &serviceerror.ServiceError{
		Code:  "DB_ERROR",
		Error: i18ncore.I18nMessage{DefaultValue: "Database error"},
	}
	suite.mockFlowService.EXPECT().ListFlows(mock.Anything, 10000, 0, flowcommon.FlowType("")).Return(nil, dbError)

	exporter, _ := suite.exportService.(*exportService).registry.Get("flow")
	options := &ExportOptions{Format: formatYAML}

	files, variables, errors := suite.exportService.(*exportService).exportResourcesWithExporter(context.Background(),
		exporter, []string{"*"}, options)

	assert.Len(suite.T(), files, 0)
	assert.Len(suite.T(), errors, 0) // Empty list on error
	assert.Empty(suite.T(), variables)
}

// TestExportResources_FlowOnly tests exporting only flows via main ExportResources method.
func (suite *ExportServiceTestSuite) TestExportResources_FlowOnly() {
	flowID := testFlowID
	mockFlow := &flowmgt.CompleteFlowDefinition{
		ID:            flowID,
		Handle:        "test-flow",
		Name:          "Test Flow",
		FlowType:      flowcommon.FlowType("AUTHENTICATION"),
		ActiveVersion: 1,
		Nodes: []flowmgt.NodeDefinition{
			{ID: "start", Type: "START"},
			{ID: "end", Type: "END"},
		},
		CreatedAt: "2025-12-22 10:00:00",
		UpdatedAt: "2025-12-22 10:00:00",
	}

	suite.mockFlowService.EXPECT().GetFlow(mock.Anything, flowID).Return(mockFlow, nil)

	request := &ExportRequest{
		Flows: []string{flowID},
		Options: &ExportOptions{
			Format: "yaml",
		},
	}

	response, err := suite.exportService.ExportResources(context.Background(), request)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), response)
	assert.Len(suite.T(), response.Files, 1)
	assert.Equal(suite.T(), "Test_Flow.yaml", response.Files[0].FileName)
	assert.Contains(suite.T(), response.Files[0].Content, "handle: test-flow")
	assert.Equal(suite.T(), 1, response.Summary.TotalFiles)
	assert.Len(suite.T(), response.Summary.Errors, 0)
}

// TestExportResources_MixedWithFlows tests exporting flows along with other resources.
func (suite *ExportServiceTestSuite) TestExportResources_MixedWithFlows() {
	appID := testAppID
	flowID := testFlowID

	mockApp := &appmodel.Application{
		ID:          appID,
		Name:        "Test App",
		Description: "Test Description",
	}

	mockFlow := &flowmgt.CompleteFlowDefinition{
		ID:            flowID,
		Handle:        "test-flow",
		Name:          "Test Flow",
		FlowType:      flowcommon.FlowType("AUTHENTICATION"),
		ActiveVersion: 1,
		Nodes: []flowmgt.NodeDefinition{
			{ID: "start", Type: "START"},
			{ID: "end", Type: "END"},
		},
	}

	suite.appServiceMock.EXPECT().GetApplication(mock.Anything, appID).Return(mockApp, nil)
	suite.mockFlowService.EXPECT().GetFlow(mock.Anything, flowID).Return(mockFlow, nil)

	request := &ExportRequest{
		Applications: []string{appID},
		Flows:        []string{flowID},
		Options: &ExportOptions{
			Format: "yaml",
		},
	}

	response, err := suite.exportService.ExportResources(context.Background(), request)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), response)
	assert.Len(suite.T(), response.Files, 2)
	assert.Equal(suite.T(), 2, response.Summary.TotalFiles)
	assert.Len(suite.T(), response.Summary.Errors, 0)

	// Verify we have both types
	var hasApp, hasFlow bool
	for _, file := range response.Files {
		if file.ResourceType == resourceTypeApplication {
			hasApp = true
		}
		if file.ResourceType == resourceTypeFlow {
			hasFlow = true
		}
	}
	assert.True(suite.T(), hasApp, "Should have application export")
	assert.True(suite.T(), hasFlow, "Should have flow export")
}
