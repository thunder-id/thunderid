// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/application/model"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/mcp/tool"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type ApplicationToolsTestSuite struct {
	suite.Suite
}

func TestApplicationToolsTestSuite(t *testing.T) {
	suite.Run(t, new(ApplicationToolsTestSuite))
}

func (suite *ApplicationToolsTestSuite) SetupTest() {
	config.ResetServerRuntime()
	cfg := &config.Config{
		Server: engineconfig.ServerConfig{
			Identifier: "test-dep",
			Hostname:   "thunderid.io",
			Port:       443,
			PublicURL:  "https://thunderid.io",
		},
		Database: config.DatabaseConfig{
			RuntimeTransient: config.DataSource{Type: "sqlite"},
		},
		JWT: engineconfig.JWTConfig{
			Issuer:         "https://thunderid.io",
			ValidityPeriod: 3600,
		},
	}
	err := config.InitializeServerRuntime("/tmp/test-application-tools", cfg)
	suite.Require().NoError(err)
}

func (suite *ApplicationToolsTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (suite *ApplicationToolsTestSuite) TestNewApplicationTools() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	tools := &applicationTools{appService: mockService}

	assert.NotNil(suite.T(), tools)
	assert.Equal(suite.T(), mockService, tools.appService)
}

func (suite *ApplicationToolsTestSuite) TestListApplications_Success() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	tools := &applicationTools{appService: mockService}

	expectedApps := []model.BasicApplicationResponse{
		{
			ID:   "app1",
			Name: "App 1",
		},
		{
			ID:   "app2",
			Name: "App 2",
		},
	}

	mockService.On("GetApplicationList", mock.Anything, mock.Anything).Return(&model.ApplicationListResponse{
		TotalResults: 2,
		Applications: expectedApps,
	}, nil)

	ctx := context.Background()
	req := &mcp.CallToolRequest{}

	result, output, err := tools.listApplications(ctx, req, model.ApplicationListInput{})

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), 2, output.TotalCount)
	assert.Len(suite.T(), output.Applications, 2)
	assert.Equal(suite.T(), "app1", output.Applications[0].ID)

	mockService.AssertExpectations(suite.T())
}

func (suite *ApplicationToolsTestSuite) TestListApplications_Error() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	tools := &applicationTools{appService: mockService}

	mockService.On("GetApplicationList", mock.Anything, mock.Anything).Return(nil, &tidcommon.ServiceError{
		ErrorDescription: tidcommon.I18nMessage{DefaultValue: "database error"},
	})

	ctx := context.Background()
	req := &mcp.CallToolRequest{}

	result, output, err := tools.listApplications(ctx, req, model.ApplicationListInput{})

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), model.ApplicationListOutput{}, output)
	assert.Contains(suite.T(), err.Error(), "failed to list applications")

	mockService.AssertExpectations(suite.T())
}

func (suite *ApplicationToolsTestSuite) TestListApplications_Scoped() {
	listed := &model.ApplicationListResponse{
		TotalResults: 1,
		Applications: []model.BasicApplicationResponse{{ID: "app1"}},
	}
	ouFilter := map[string]interface{}{"ouId": "ou-1"}

	for name, tc := range map[string]struct {
		input  model.ApplicationListInput
		expect func(*ApplicationServiceInterfaceMock)
	}{
		"an organization unit": {
			input: model.ApplicationListInput{OUID: "ou-1"},
			expect: func(m *ApplicationServiceInterfaceMock) {
				m.On("GetApplicationList", mock.Anything, ouFilter).Return(listed, nil)
			},
		},
		"a project": {
			input: model.ApplicationListInput{ProjectID: "project-1"},
			expect: func(m *ApplicationServiceInterfaceMock) {
				m.On("GetApplicationListByProject", mock.Anything, "project-1",
					map[string]interface{}(nil)).Return(listed, nil)
			},
		},
		"an organization unit of a project": {
			input: model.ApplicationListInput{ProjectID: "project-1", OUID: "ou-1"},
			expect: func(m *ApplicationServiceInterfaceMock) {
				m.On("GetApplicationListByProject", mock.Anything, "project-1", ouFilter).Return(listed, nil)
			},
		},
	} {
		suite.Run(name, func() {
			mockService := NewApplicationServiceInterfaceMock(suite.T())
			tc.expect(mockService)
			tools := &applicationTools{appService: mockService}

			_, output, err := tools.listApplications(context.Background(), &mcp.CallToolRequest{}, tc.input)

			assert.NoError(suite.T(), err)
			assert.Equal(suite.T(), 1, output.TotalCount)
			assert.Equal(suite.T(), "app1", output.Applications[0].ID)
		})
	}
}

func (suite *ApplicationToolsTestSuite) TestListApplications_ProjectError() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	mockService.On("GetApplicationListByProject", mock.Anything, "project-1", mock.Anything).
		Return(nil, &ErrorInvalidFilter)
	tools := &applicationTools{appService: mockService}

	_, output, err := tools.listApplications(context.Background(), &mcp.CallToolRequest{},
		model.ApplicationListInput{ProjectID: "project-1"})

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), model.ApplicationListOutput{}, output)
	assert.Contains(suite.T(), err.Error(), "failed to list applications")
}

func (suite *ApplicationToolsTestSuite) TestGetApplicationByID_Success() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	tools := &applicationTools{appService: mockService}

	expectedApp := &providers.Application{
		ID:          "app123",
		Name:        "Test App",
		Description: "Test Description",
	}

	mockService.On("GetApplication", mock.Anything, "app123").Return(expectedApp, nil)

	ctx := context.Background()
	req := &mcp.CallToolRequest{}
	input := tool.IDInput{ID: "app123"}

	result, output, err := tools.getApplicationByID(ctx, req, input)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), expectedApp, output)

	mockService.AssertExpectations(suite.T())
}

func (suite *ApplicationToolsTestSuite) TestGetApplicationByID_Error() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	tools := &applicationTools{appService: mockService}

	mockService.On("GetApplication", mock.Anything, "app123").Return(nil, &tidcommon.ServiceError{
		ErrorDescription: tidcommon.I18nMessage{DefaultValue: "application not found"},
	})

	ctx := context.Background()
	req := &mcp.CallToolRequest{}
	input := tool.IDInput{ID: "app123"}

	result, output, err := tools.getApplicationByID(ctx, req, input)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Nil(suite.T(), output)
	assert.Contains(suite.T(), err.Error(), "failed to get application")

	mockService.AssertExpectations(suite.T())
}

func (suite *ApplicationToolsTestSuite) TestGetApplicationByClientID_Success() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	tools := &applicationTools{appService: mockService}

	oauthApp := &providers.OAuthClient{
		ID:       "app123",
		ClientID: "client123",
	}

	expectedApp := &providers.Application{
		ID:   "app123",
		Name: "Test App",
	}

	mockService.On("GetOAuthApplication", mock.Anything, "client123").Return(oauthApp, nil)
	mockService.On("GetApplication", mock.Anything, "app123").Return(expectedApp, nil)

	ctx := context.Background()
	req := &mcp.CallToolRequest{}
	input := model.ClientIDInput{ClientID: "client123"}

	result, output, err := tools.getApplicationByClientID(ctx, req, input)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), expectedApp, output)

	mockService.AssertExpectations(suite.T())
}

func (suite *ApplicationToolsTestSuite) TestGetApplicationByClientID_OAuthError() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	tools := &applicationTools{appService: mockService}

	mockService.On("GetOAuthApplication", mock.Anything, "client123").Return(nil, &tidcommon.ServiceError{
		ErrorDescription: tidcommon.I18nMessage{DefaultValue: "OAuth application not found"},
	})

	ctx := context.Background()
	req := &mcp.CallToolRequest{}
	input := model.ClientIDInput{ClientID: "client123"}

	result, output, err := tools.getApplicationByClientID(ctx, req, input)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Nil(suite.T(), output)
	assert.Contains(suite.T(), err.Error(), "failed to get OAuth application")

	mockService.AssertExpectations(suite.T())
}

func (suite *ApplicationToolsTestSuite) TestGetApplicationByClientID_AppError() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	tools := &applicationTools{appService: mockService}

	oauthApp := &providers.OAuthClient{
		ID:       "app123",
		ClientID: "client123",
	}

	mockService.On("GetOAuthApplication", mock.Anything, "client123").Return(oauthApp, nil)
	mockService.On("GetApplication", mock.Anything, "app123").Return(nil, &tidcommon.ServiceError{
		ErrorDescription: tidcommon.I18nMessage{DefaultValue: "application not found"},
	})

	ctx := context.Background()
	req := &mcp.CallToolRequest{}
	input := model.ClientIDInput{ClientID: "client123"}

	result, output, err := tools.getApplicationByClientID(ctx, req, input)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Nil(suite.T(), output)
	assert.Contains(suite.T(), err.Error(), "failed to get application")

	mockService.AssertExpectations(suite.T())
}

func (suite *ApplicationToolsTestSuite) TestCreateApplication_Success() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	tools := &applicationTools{appService: mockService}

	inputApp := model.ApplicationDTO{
		Name:        "New App",
		Description: "New Description",
	}

	createdApp := &model.ApplicationDTO{
		ID:          "new-app-id",
		Name:        "New App",
		Description: "New Description",
	}

	mockService.On("CreateApplication", mock.Anything, &inputApp).Return(createdApp, nil)

	ctx := context.Background()
	req := &mcp.CallToolRequest{}

	result, output, err := tools.createApplication(ctx, req, inputApp)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), createdApp, output)

	mockService.AssertExpectations(suite.T())
}

func (suite *ApplicationToolsTestSuite) TestCreateApplication_Error() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	tools := &applicationTools{appService: mockService}

	inputApp := model.ApplicationDTO{
		Name: "New App",
	}

	mockService.On("CreateApplication", mock.Anything, &inputApp).Return(nil, &tidcommon.ServiceError{
		ErrorDescription: tidcommon.I18nMessage{DefaultValue: "validation error"},
	})

	ctx := context.Background()
	req := &mcp.CallToolRequest{}

	result, output, err := tools.createApplication(ctx, req, inputApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Nil(suite.T(), output)
	assert.Contains(suite.T(), err.Error(), "failed to create application")

	mockService.AssertExpectations(suite.T())
}

func (suite *ApplicationToolsTestSuite) TestUpdateApplication_Success() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	tools := &applicationTools{appService: mockService}

	inputApp := model.ApplicationDTO{
		ID:          "app123",
		Name:        "Updated App",
		Description: "Updated Description",
	}

	updatedApp := &model.ApplicationDTO{
		ID:          "app123",
		Name:        "Updated App",
		Description: "Updated Description",
	}

	mockService.On("UpdateApplication", mock.Anything, "app123", &inputApp).Return(updatedApp, nil)

	ctx := context.Background()
	req := &mcp.CallToolRequest{}

	result, output, err := tools.updateApplication(ctx, req, inputApp)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), updatedApp, output)

	mockService.AssertExpectations(suite.T())
}

func (suite *ApplicationToolsTestSuite) TestUpdateApplication_Error() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	tools := &applicationTools{appService: mockService}

	inputApp := model.ApplicationDTO{
		ID:   "app123",
		Name: "Updated App",
	}

	mockService.On("UpdateApplication", mock.Anything, "app123", &inputApp).Return(nil, &tidcommon.ServiceError{
		ErrorDescription: tidcommon.I18nMessage{DefaultValue: "application not found"},
	})

	ctx := context.Background()
	req := &mcp.CallToolRequest{}

	result, output, err := tools.updateApplication(ctx, req, inputApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Nil(suite.T(), output)
	assert.Contains(suite.T(), err.Error(), "failed to update application")

	mockService.AssertExpectations(suite.T())
}

// The notice says whether the tokens the old secret issued were revoked, as it happened.
func (suite *ApplicationToolsTestSuite) TestRegenerateApplicationSecret_Success() {
	cases := map[string]struct {
		response *model.ClientSecretResponse
		says     string
	}{
		"revoked": {&model.ClientSecretResponse{ClientSecret: "new-secret", Revoked: true},
			"the tokens it issued are revoked"},
		"revocation incomplete": {&model.ClientSecretResponse{ClientSecret: "new-secret", RevocationIncomplete: true},
			"revoking the tokens issued while it was replaced failed"},
		"nothing revoked": {&model.ClientSecretResponse{ClientSecret: "new-secret"},
			"were not revoked"},
	}
	for name, tc := range cases {
		suite.Run(name, func() {
			mockService := NewApplicationServiceInterfaceMock(suite.T())
			tools := &applicationTools{appService: mockService}
			mockService.On("RegenerateClientSecret", mock.Anything, "app123").Return(tc.response, nil).Once()

			result, output, err := tools.regenerateApplicationSecret(context.Background(), &mcp.CallToolRequest{},
				tool.IDInput{ID: "app123"})

			assert.NoError(suite.T(), err)
			assert.Nil(suite.T(), result)
			suite.Require().NotNil(output)
			assert.Equal(suite.T(), "new-secret", output.ClientSecret)
			assert.Equal(suite.T(), tc.response.RevocationIncomplete, output.RevocationIncomplete)
			assert.Contains(suite.T(), output.Notice, tc.says)
			assert.Contains(suite.T(), output.Notice, clientSecretNotice)
		})
	}
}

func (suite *ApplicationToolsTestSuite) TestRegenerateApplicationSecret_Error() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	tools := &applicationTools{appService: mockService}
	mockService.On("RegenerateClientSecret", mock.Anything, "app123").Return(nil, &ErrorApplicationHasNoClientSecret).
		Once()

	result, output, err := tools.regenerateApplicationSecret(context.Background(), &mcp.CallToolRequest{},
		tool.IDInput{ID: "app123"})

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Nil(suite.T(), output)
	assert.Contains(suite.T(), err.Error(), "failed to regenerate application secret")
}

func (suite *ApplicationToolsTestSuite) TestGetApplicationTemplates_Success() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	tools := &applicationTools{appService: mockService}

	ctx := context.Background()
	req := &mcp.CallToolRequest{}

	result, output, err := tools.getApplicationTemplates(ctx, req, nil)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), output)
	assert.Contains(suite.T(), output, "spa")
	assert.Contains(suite.T(), output, "mobile")
	assert.Contains(suite.T(), output, "server")
	assert.Contains(suite.T(), output, "m2m")

	// Verify SPA template structure
	spaTemplate := output["spa"]
	assert.Equal(suite.T(), "<APP_NAME>", spaTemplate.Name)
	assert.NotEmpty(suite.T(), spaTemplate.InboundAuthConfig)
	assert.Equal(suite.T(), "<THEME_ID>", spaTemplate.InboundAuthProfile.ThemeID)

	mobileTemplate := output["mobile"]
	assert.Equal(suite.T(), "<THEME_ID>", mobileTemplate.InboundAuthProfile.ThemeID)

	serverTemplate := output["server"]
	assert.Equal(suite.T(), "<THEME_ID>", serverTemplate.InboundAuthProfile.ThemeID)

	m2mTemplate := output["m2m"]
	assert.Empty(suite.T(), m2mTemplate.InboundAuthProfile.ThemeID)

	mockService.AssertExpectations(suite.T())
}

func (suite *ApplicationToolsTestSuite) TestRegisterMCPTools() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "test-server",
		Version: "1.0.0",
	}, nil)

	// Register tools
	registerMCPTools(server, mockService)

	// Verify tools are registered by checking server has tools
	// Note: We can't directly verify the tools list without accessing internal server state,
	// but we can verify the function doesn't panic
	assert.NotNil(suite.T(), server)
}

// TestRegisteredToolsRequireApplicationPermissions verifies that every application MCP tool is mapped
// to an application catalog permission, so none falls back to root by accident.
func (suite *ApplicationToolsTestSuite) TestRegisteredToolsRequireApplicationPermissions() {
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "1.0.0"}, nil)
	registerMCPTools(server, NewApplicationServiceInterfaceMock(suite.T()))

	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	_, err := server.Connect(ctx, serverTransport, nil)
	suite.Require().NoError(err)
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil).
		Connect(ctx, clientTransport, nil)
	suite.Require().NoError(err)
	defer func() { _ = session.Close() }()
	tools, err := session.ListTools(ctx, nil)
	suite.Require().NoError(err)

	suite.Require().NotEmpty(tools.Tools)
	security.ForEachPermissionPlaneForTest(func(plane string) {
		suite.Run(plane, func() {
			p := security.GetSystemPermissions()
			for _, registered := range tools.Tools {
				assert.Contains(suite.T(), []string{p.AppView, p.AppCreate, p.AppUpdate, p.AppDelete},
					security.ResolveMCPToolPermission(registered.Name),
					"tool %q must require an application permission", registered.Name)
			}
		})
	})
}

func (suite *ApplicationToolsTestSuite) TestGetCommonSchemaModifiers() {
	modifiers := getCommonSchemaModifiers()

	assert.NotNil(suite.T(), modifiers)
	assert.Len(suite.T(), modifiers, 5)
}

func (suite *ApplicationToolsTestSuite) TestGetCreateAppSchema() {
	schema := getCreateAppSchema()

	suite.Require().NotNil(schema)
	assert.Contains(suite.T(), schema.Properties, "allowedUserTypes")
	assert.NotContains(suite.T(), schema.Properties, "subjectAttribute")
	assert.NotContains(suite.T(), schema.Properties, "id")
	assert.NotContains(suite.T(), schema.Properties, "status")
}

func (suite *ApplicationToolsTestSuite) TestGetUpdateAppSchema() {
	schema := getUpdateAppSchema()

	suite.Require().NotNil(schema)
	assert.Contains(suite.T(), schema.Properties, "allowedUserTypes")
	assert.NotContains(suite.T(), schema.Properties, "subjectAttribute")
	assert.Contains(suite.T(), schema.Required, "id")
	suite.Require().Contains(schema.Properties, "status")
	assert.True(suite.T(), schema.Properties["status"].ReadOnly)
	assert.NotContains(suite.T(), schema.Required, "status")
}

func (suite *ApplicationToolsTestSuite) TestCreateApplication_IgnoresSubjectAttribute() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	tools := &applicationTools{appService: mockService}

	inputApp := model.ApplicationDTO{
		Name: "New App",
		InboundAuthProfile: providers.InboundAuthProfile{
			AllowedUserTypes: []string{"customer"},
			SubjectAttribute: map[string]string{"customer": "email"},
		},
	}
	createdApp := &model.ApplicationDTO{ID: "new-app-id", Name: "New App"}

	mockService.On("CreateApplication", mock.Anything, mock.MatchedBy(func(dto *model.ApplicationDTO) bool {
		return dto.SubjectAttribute == nil && len(dto.AllowedUserTypes) == 1
	})).Return(createdApp, nil)

	_, output, err := tools.createApplication(context.Background(), &mcp.CallToolRequest{}, inputApp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), createdApp, output)
	mockService.AssertExpectations(suite.T())
}

func (suite *ApplicationToolsTestSuite) TestUpdateApplication_IgnoresSubjectAttribute() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	tools := &applicationTools{appService: mockService}

	inputApp := model.ApplicationDTO{
		ID:   "app123",
		Name: "Updated App",
		InboundAuthProfile: providers.InboundAuthProfile{
			AllowedUserTypes: []string{"customer"},
			SubjectAttribute: map[string]string{"customer": "email"},
		},
	}
	updatedApp := &model.ApplicationDTO{ID: "app123", Name: "Updated App"}

	mockService.On("UpdateApplication", mock.Anything, "app123",
		mock.MatchedBy(func(dto *model.ApplicationDTO) bool {
			return dto.SubjectAttribute == nil && len(dto.AllowedUserTypes) == 1
		})).Return(updatedApp, nil)

	_, output, err := tools.updateApplication(context.Background(), &mcp.CallToolRequest{}, inputApp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), updatedApp, output)
	mockService.AssertExpectations(suite.T())
}

func (suite *ApplicationToolsTestSuite) TestGetApplicationTools_OmitSubjectAttribute() {
	stored := &providers.Application{
		ID:   "app123",
		Name: "Test App",
		InboundAuthProfile: providers.InboundAuthProfile{
			AllowedUserTypes: []string{"customer"},
			SubjectAttribute: map[string]string{"customer": "email"},
		},
	}
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	mockService.On("GetApplication", mock.Anything, "app123").Return(stored, nil)
	mockService.On("GetOAuthApplication", mock.Anything, "client123").
		Return(&providers.OAuthClient{ID: "app123", ClientID: "client123"}, nil)
	tools := &applicationTools{appService: mockService}

	byID, byIDOutput, err := tools.getApplicationByID(context.Background(), &mcp.CallToolRequest{},
		tool.IDInput{ID: "app123"})
	suite.Require().NoError(err)
	assert.Nil(suite.T(), byID)
	_, byClientIDOutput, err := tools.getApplicationByClientID(context.Background(), &mcp.CallToolRequest{},
		model.ClientIDInput{ClientID: "client123"})
	suite.Require().NoError(err)

	for _, output := range []*providers.Application{byIDOutput, byClientIDOutput} {
		assert.Nil(suite.T(), output.SubjectAttribute)
		assert.Equal(suite.T(), "Test App", output.Name)
		assert.Equal(suite.T(), []string{"customer"}, output.AllowedUserTypes)
	}
	assert.Equal(suite.T(), map[string]string{"customer": "email"}, stored.SubjectAttribute,
		"the service's application must not be modified")
}

func (suite *ApplicationToolsTestSuite) TestCallTool_SubjectAttributeRefused() {
	for _, tc := range []struct {
		tool   string
		method string
	}{
		{tool: "create_application", method: "CreateApplication"},
		{tool: "update_application", method: "UpdateApplication"},
	} {
		suite.Run(tc.tool, func() {
			mockService := suite.toolCallMock()
			args := applicationToolArgs(tc.tool)
			args["subjectAttribute"] = map[string]string{"customer": "email"}

			result, err := suite.toolSession(mockService).CallTool(context.Background(),
				&mcp.CallToolParams{Name: tc.tool, Arguments: args})

			suite.Require().NoError(err)
			assert.True(suite.T(), result.IsError, "the call must be refused")
			assert.Contains(suite.T(), toolResultText(result), "subjectAttribute")
			mockService.AssertNumberOfCalls(suite.T(), tc.method, 0)
		})
	}
}

func (suite *ApplicationToolsTestSuite) TestCallTool_ProfileWithoutSubjectAttribute() {
	for _, tc := range []struct {
		tool   string
		method string
	}{
		{tool: "create_application", method: "CreateApplication"},
		{tool: "update_application", method: "UpdateApplication"},
	} {
		suite.Run(tc.tool, func() {
			mockService := suite.toolCallMock()

			result, err := suite.toolSession(mockService).CallTool(context.Background(),
				&mcp.CallToolParams{Name: tc.tool, Arguments: applicationToolArgs(tc.tool)})

			suite.Require().NoError(err)
			assert.False(suite.T(), result.IsError, "content: %s", toolResultText(result))
			mockService.AssertNumberOfCalls(suite.T(), tc.method, 1)
		})
	}
}

// toolCallMock returns a service mock that answers create and update, so a call reaching the service is
// counted rather than failing inside the server.
func (suite *ApplicationToolsTestSuite) toolCallMock() *ApplicationServiceInterfaceMock {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	returned := &model.ApplicationDTO{ID: "app123", Name: "App"}
	mockService.On("CreateApplication", mock.Anything, mock.Anything).Return(returned, nil).Maybe()
	mockService.On("UpdateApplication", mock.Anything, "app123", mock.Anything).Return(returned, nil).Maybe()
	return mockService
}

// applicationToolArgs returns the minimal arguments the create_application or update_application tool accepts.
func applicationToolArgs(toolName string) map[string]any {
	args := map[string]any{
		"name":                      "App",
		"type":                      "fullstack",
		"allowedUserTypes":          []string{"customer"},
		"isRegistrationFlowEnabled": false,
		"isRecoveryFlowEnabled":     false,
	}
	if toolName == "update_application" {
		args["id"] = "app123"
	}
	return args
}

func (suite *ApplicationToolsTestSuite) TestUpdateApplication_AcceptsGetApplicationResult() {
	alwaysPrompt := true
	app := &providers.Application{
		ID:          "app123",
		OUID:        "ou1",
		ProjectID:   "project1",
		Name:        "App",
		Description: "An application",
		Type:        string(model.ApplicationTypeFullStack),
		Template:    "react",
		URL:         "https://app.example.com",
		LogoURL:     "https://app.example.com/logo.png",
		TosURI:      "https://app.example.com/tos",
		PolicyURI:   "https://app.example.com/policy",
		Contacts:    []string{"admin@example.com"},
		Status:      &providers.ApplicationStatusReport{Value: providers.AccessStatusActive},
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID:            "auth-flow",
			ThemeID:               "theme1",
			Assertion:             &providers.AssertionConfig{ValidityPeriod: 3600, UserAttributes: []string{"email"}},
			LoginConsent:          &providers.LoginConsentConfig{ValidityPeriod: 0, AlwaysPrompt: &alwaysPrompt},
			AllowedUserTypes:      []string{"customer"},
			SubjectAttribute:      map[string]string{"customer": "email"},
			PasskeyAllowedOrigins: []string{"https://app.example.com"},
			Attestation: &providers.AttestationConfig{
				Apple: &providers.AppleAttestationConfig{TeamID: "team", BundleID: "com.example.app"},
			},
		},
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{{
			Type: providers.OAuthInboundAuthType,
			OAuthConfig: &providers.OAuthConfigWithSecret{
				ClientID:                "client1",
				RedirectURIs:            []string{"https://app.example.com/callback"},
				GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
				ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
				TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
				PKCERequired:            true,
				Token: &providers.OAuthTokenConfig{
					AccessToken: &providers.AccessTokenConfig{
						UserConfig: &providers.AccessTokenSubConfig{ValidityPeriod: 600},
					},
					IDToken: &providers.IDTokenConfig{ValidityPeriod: 600, UserAttributes: []string{"email"}},
				},
				Scopes:      []string{"openid", "profile"},
				ScopeClaims: map[string][]string{"profile": {"name"}},
			},
		}},
		Metadata: map[string]interface{}{"owner": "team-a"},
	}
	for _, tc := range []struct {
		tool string
		args map[string]any
	}{
		{tool: "get_application_by_id", args: map[string]any{"id": "app123"}},
		{tool: "get_application_by_client_id", args: map[string]any{"client_id": "client1"}},
	} {
		suite.Run(tc.tool, func() {
			mockService := NewApplicationServiceInterfaceMock(suite.T())
			mockService.On("GetOAuthApplication", mock.Anything, "client1").
				Return(&providers.OAuthClient{ID: "app123", ClientID: "client1"}, nil).Maybe()
			mockService.On("GetApplication", mock.Anything, "app123").Return(app, nil).Maybe()
			var updated *model.ApplicationDTO
			mockService.On("UpdateApplication", mock.Anything, "app123", mock.Anything).
				Run(func(args mock.Arguments) { updated = args.Get(2).(*model.ApplicationDTO) }).
				Return(&model.ApplicationDTO{ID: "app123", Name: "App"}, nil).Maybe()
			session := suite.toolSession(mockService)
			ctx := context.Background()

			got, err := session.CallTool(ctx, &mcp.CallToolParams{Name: tc.tool, Arguments: tc.args})
			suite.Require().NoError(err)
			suite.Require().False(got.IsError, "content: %s", toolResultText(got))
			suite.Require().Contains(got.StructuredContent, "status")

			result, err := session.CallTool(ctx, &mcp.CallToolParams{
				Name: "update_application", Arguments: got.StructuredContent})

			suite.Require().NoError(err)
			suite.Require().False(result.IsError, "content: %s", toolResultText(result))
			mockService.AssertNumberOfCalls(suite.T(), "UpdateApplication", 1)
			suite.Require().NotNil(updated)
			suite.Nil(updated.Status)
			suite.Equal(app.ID, updated.ID)
			suite.Equal(app.Name, updated.Name)
			suite.Equal(app.Template, updated.Template)
			suite.Equal(app.Metadata, updated.Metadata)
			wantProfile := app.InboundAuthProfile
			wantProfile.SubjectAttribute = nil
			suite.Equal(wantProfile, updated.InboundAuthProfile)
			suite.Equal(app.InboundAuthConfig, updated.InboundAuthConfig)
		})
	}
}

// toolSession registers the application tools on a new server and returns a client session to it.
func (suite *ApplicationToolsTestSuite) toolSession(appService ApplicationServiceInterface) *mcp.ClientSession {
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "1.0.0"}, nil)
	registerMCPTools(server, appService)
	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	_, err := server.Connect(ctx, serverTransport, nil)
	suite.Require().NoError(err)
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil).
		Connect(ctx, clientTransport, nil)
	suite.Require().NoError(err)
	suite.T().Cleanup(func() { _ = session.Close() })
	return session
}

// toolResultText joins the text content of a tool result.
func toolResultText(result *mcp.CallToolResult) string {
	var text string
	for _, c := range result.Content {
		if t, ok := c.(*mcp.TextContent); ok {
			text += t.Text
		}
	}
	return text
}
