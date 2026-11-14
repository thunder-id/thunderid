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

func (suite *ApplicationToolsTestSuite) TestRegenerateApplicationSecret_Success() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	tools := &applicationTools{appService: mockService}
	mockService.On("RegenerateClientSecret", mock.Anything, "app123").
		Return(&model.ClientSecretResponse{ClientSecret: "new-secret"}, nil).Once()

	result, output, err := tools.regenerateApplicationSecret(context.Background(), &mcp.CallToolRequest{},
		tool.IDInput{ID: "app123"})

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	suite.Require().NotNil(output)
	assert.Equal(suite.T(), "new-secret", output.ClientSecret)
	assert.Equal(suite.T(), clientSecretNotice, output.Notice)
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
	assert.Len(suite.T(), modifiers, 4)
}

func (suite *ApplicationToolsTestSuite) TestGetCreateAppSchema() {
	schema := getCreateAppSchema()

	assert.NotNil(suite.T(), schema)
}

func (suite *ApplicationToolsTestSuite) TestGetUpdateAppSchema() {
	schema := getUpdateAppSchema()

	assert.NotNil(suite.T(), schema)
}
