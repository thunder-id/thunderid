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

	mockService.On("GetApplicationList", mock.Anything).Return(&model.ApplicationListResponse{
		TotalResults: 2,
		Applications: expectedApps,
	}, nil)

	ctx := context.Background()
	req := &mcp.CallToolRequest{}

	result, output, err := tools.listApplications(ctx, req, nil)

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

	mockService.On("GetApplicationList", mock.Anything).Return(nil, &tidcommon.ServiceError{
		ErrorDescription: tidcommon.I18nMessage{DefaultValue: "database error"},
	})

	ctx := context.Background()
	req := &mcp.CallToolRequest{}

	result, output, err := tools.listApplications(ctx, req, nil)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), model.ApplicationListOutput{}, output)
	assert.Contains(suite.T(), err.Error(), "failed to list applications")

	mockService.AssertExpectations(suite.T())
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
