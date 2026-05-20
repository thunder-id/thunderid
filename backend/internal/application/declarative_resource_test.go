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

package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/application"
	"github.com/thunder-id/thunderid/internal/application/model"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	i18ncore "github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/applicationmock"
)

// ApplicationExporterTestSuite tests the applicationExporter.
type ApplicationExporterTestSuite struct {
	suite.Suite
	mockService *applicationmock.ApplicationServiceInterfaceMock
	exporter    declarativeresource.ResourceExporter
	logger      *log.Logger
}

func TestApplicationExporterTestSuite(t *testing.T) {
	suite.Run(t, new(ApplicationExporterTestSuite))
}

func (s *ApplicationExporterTestSuite) SetupTest() {
	s.mockService = applicationmock.NewApplicationServiceInterfaceMock(s.T())
	s.exporter = application.NewApplicationExporterForTest(s.mockService)
	s.logger = log.GetLogger()
}

func (s *ApplicationExporterTestSuite) TestNewApplicationExporter() {
	assert.NotNil(s.T(), s.exporter)
}

func (s *ApplicationExporterTestSuite) TestGetResourceType() {
	assert.Equal(s.T(), "application", s.exporter.GetResourceType())
}

func (s *ApplicationExporterTestSuite) TestGetParameterizerType() {
	assert.Equal(s.T(), "Application", s.exporter.GetParameterizerType())
}

func (s *ApplicationExporterTestSuite) TestGetAllResourceIDs_Success() {
	expectedApps := &model.ApplicationListResponse{
		Applications: []model.BasicApplicationResponse{
			{ID: "app1", Name: "App 1"},
			{ID: "app2", Name: "App 2"},
		},
	}

	s.mockService.EXPECT().GetApplicationList(mock.Anything).Return(expectedApps, nil)

	ids, err := s.exporter.GetAllResourceIDs(context.Background())

	assert.Nil(s.T(), err)
	assert.Len(s.T(), ids, 2)
	assert.Equal(s.T(), "app1", ids[0])
	assert.Equal(s.T(), "app2", ids[1])
}

func (s *ApplicationExporterTestSuite) TestGetAllResourceIDs_Error() {
	serviceError := &serviceerror.ServiceError{
		Code:  "ERR_CODE",
		Error: i18ncore.I18nMessage{DefaultValue: "test error"},
	}

	s.mockService.EXPECT().GetApplicationList(mock.Anything).Return(nil, serviceError)

	ids, err := s.exporter.GetAllResourceIDs(context.Background())

	assert.Nil(s.T(), ids)
	assert.Equal(s.T(), &serviceerror.InternalServerError, err)
}

func (s *ApplicationExporterTestSuite) TestGetAllResourceIDs_EmptyList() {
	expectedApps := &model.ApplicationListResponse{
		Applications: []model.BasicApplicationResponse{},
	}

	s.mockService.EXPECT().GetApplicationList(mock.Anything).Return(expectedApps, nil)

	ids, err := s.exporter.GetAllResourceIDs(context.Background())

	assert.Nil(s.T(), err)
	assert.Len(s.T(), ids, 0)
}

func (s *ApplicationExporterTestSuite) TestGetResourceByID_Success() {
	expectedApp := &model.Application{
		ID:   "app1",
		Name: "Test App",
	}

	s.mockService.EXPECT().GetApplication(mock.Anything, "app1").Return(expectedApp, nil)

	resource, name, err := s.exporter.GetResourceByID(context.Background(), "app1")

	assert.Nil(s.T(), err)
	assert.Equal(s.T(), "Test App", name)
	assert.Equal(s.T(), expectedApp, resource)
}

func (s *ApplicationExporterTestSuite) TestGetResourceByID_Error() {
	serviceError := &serviceerror.ServiceError{
		Code:  "ERR_CODE",
		Error: i18ncore.I18nMessage{DefaultValue: "test error"},
	}

	s.mockService.EXPECT().GetApplication(mock.Anything, "app1").Return(nil, serviceError)

	resource, name, err := s.exporter.GetResourceByID(context.Background(), "app1")

	assert.Nil(s.T(), resource)
	assert.Empty(s.T(), name)
	assert.Equal(s.T(), serviceError, err)
}

func (s *ApplicationExporterTestSuite) TestValidateResource_Success() {
	app := &model.Application{
		ID:   "app1",
		Name: "Valid App",
	}

	name, err := s.exporter.ValidateResource(app, "app1", s.logger)

	assert.Nil(s.T(), err)
	assert.Equal(s.T(), "Valid App", name)
}

func (s *ApplicationExporterTestSuite) TestValidateResource_InvalidType() {
	invalidResource := "not an application"

	name, err := s.exporter.ValidateResource(invalidResource, "app1", s.logger)

	assert.Empty(s.T(), name)
	assert.NotNil(s.T(), err)
	assert.Equal(s.T(), "application", err.ResourceType)
	assert.Equal(s.T(), "app1", err.ResourceID)
	assert.Equal(s.T(), "INVALID_TYPE", err.Code)
}

func (s *ApplicationExporterTestSuite) TestValidateResource_EmptyName() {
	app := &model.Application{
		ID:   "app1",
		Name: "",
	}

	name, err := s.exporter.ValidateResource(app, "app1", s.logger)

	assert.Empty(s.T(), name)
	assert.NotNil(s.T(), err)
	assert.Equal(s.T(), "application", err.ResourceType)
	assert.Equal(s.T(), "app1", err.ResourceID)
	assert.Equal(s.T(), "APP_VALIDATION_ERROR", err.Code)
	assert.Contains(s.T(), err.Error, "name is empty")
}

func (s *ApplicationExporterTestSuite) TestGetResourceRulesForResource_PublicClient() {
	pr, ok := s.exporter.(declarativeresource.PerResourceRuler)
	assert.True(s.T(), ok, "exporter should implement PerResourceRuler")

	app := &model.Application{
		ID:   "app1",
		Name: "Public App",
		InboundAuthConfig: []inboundmodel.InboundAuthConfigWithSecret{
			{
				OAuthConfig: &inboundmodel.OAuthConfigWithSecret{
					ClientID:     "client-id-1",
					PublicClient: true,
				},
			},
		},
	}

	rules := pr.GetResourceRulesForResource(app)

	assert.NotNil(s.T(), rules)
	assert.Contains(s.T(), rules.Variables, "InboundAuthConfig[].OAuthConfig.ClientID")
	assert.NotContains(s.T(), rules.Variables, "InboundAuthConfig[].OAuthConfig.ClientSecret")
	assert.Contains(s.T(), rules.ArrayVariables, "InboundAuthConfig[].OAuthConfig.RedirectURIs")
}

func (s *ApplicationExporterTestSuite) TestGetResourceRulesForResource_ConfidentialClient() {
	pr, ok := s.exporter.(declarativeresource.PerResourceRuler)
	assert.True(s.T(), ok, "exporter should implement PerResourceRuler")

	app := &model.Application{
		ID:   "app2",
		Name: "Confidential App",
		InboundAuthConfig: []inboundmodel.InboundAuthConfigWithSecret{
			{
				OAuthConfig: &inboundmodel.OAuthConfigWithSecret{
					ClientID:     "client-id-2",
					PublicClient: false,
				},
			},
		},
	}

	rules := pr.GetResourceRulesForResource(app)

	assert.NotNil(s.T(), rules)
	assert.Contains(s.T(), rules.Variables, "InboundAuthConfig[].OAuthConfig.ClientID")
	assert.Contains(s.T(), rules.Variables, "InboundAuthConfig[].OAuthConfig.ClientSecret")
	assert.Contains(s.T(), rules.ArrayVariables, "InboundAuthConfig[].OAuthConfig.RedirectURIs")
}

func (s *ApplicationExporterTestSuite) TestGetResourceRulesForResource_NonApplicationType() {
	pr, ok := s.exporter.(declarativeresource.PerResourceRuler)
	assert.True(s.T(), ok, "exporter should implement PerResourceRuler")

	rules := pr.GetResourceRulesForResource("not-an-application")

	assert.NotNil(s.T(), rules)
	assert.Contains(s.T(), rules.Variables, "InboundAuthConfig[].OAuthConfig.ClientID")
	assert.Contains(s.T(), rules.Variables, "InboundAuthConfig[].OAuthConfig.ClientSecret")
}
