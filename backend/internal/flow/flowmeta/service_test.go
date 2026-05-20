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

package flowmeta

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/design/common"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	i18nmgt "github.com/thunder-id/thunderid/internal/system/i18n/mgt"
	"github.com/thunder-id/thunderid/tests/mocks/design/resolvemock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/i18n/mgtmock"
	"github.com/thunder-id/thunderid/tests/mocks/inboundclientmock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
)

const (
	testAppID = "app-123"
	testOUID  = "ou-456"
)

// Test Suite

type FlowMetaServiceTestSuite struct {
	suite.Suite
	mockInboundClient  *inboundclientmock.InboundClientServiceInterfaceMock
	mockEntityProvider *entityprovidermock.EntityProviderInterfaceMock
	mockOUService      *oumock.OrganizationUnitServiceInterfaceMock
	mockDesignResolve  *resolvemock.DesignResolveServiceInterfaceMock
	mockI18nService    *mgtmock.I18nServiceInterfaceMock
	service            FlowMetaServiceInterface
	ctx                context.Context
}

func TestFlowMetaServiceTestSuite(t *testing.T) {
	suite.Run(t, new(FlowMetaServiceTestSuite))
}

func (suite *FlowMetaServiceTestSuite) SetupTest() {
	suite.mockInboundClient = inboundclientmock.NewInboundClientServiceInterfaceMock(suite.T())
	suite.mockEntityProvider = entityprovidermock.NewEntityProviderInterfaceMock(suite.T())
	suite.mockOUService = oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())
	suite.mockDesignResolve = resolvemock.NewDesignResolveServiceInterfaceMock(suite.T())
	suite.mockI18nService = mgtmock.NewI18nServiceInterfaceMock(suite.T())
	suite.service = newFlowMetaService(
		suite.mockInboundClient,
		suite.mockEntityProvider,
		suite.mockOUService,
		suite.mockDesignResolve,
		suite.mockI18nService,
	)
	suite.ctx = context.Background()
}

func (suite *FlowMetaServiceTestSuite) TearDownTest() {
	// Mockery-generated mocks automatically assert expectations
}

// expectInboundLookup wires the inbound + entity mocks for an APP-type lookup. The synthesized
// inbound client carries the fields populateTypeMetadata reads onto ApplicationMetadata.
func (suite *FlowMetaServiceTestSuite) expectInboundLookup(
	appID string, name string, isRegEnabled bool, props map[string]interface{},
) {
	client := &inboundmodel.InboundClient{
		ID:                        appID,
		IsRegistrationFlowEnabled: isRegEnabled,
		Properties:                props,
	}
	sysAttrs, _ := json.Marshal(map[string]interface{}{"name": name})
	entity := &entityprovider.Entity{
		ID:               appID,
		Category:         entityprovider.EntityCategoryApp,
		SystemAttributes: sysAttrs,
	}
	suite.mockInboundClient.On("GetInboundClientByEntityID", mock.Anything, appID).Return(client, nil)
	suite.mockEntityProvider.On("GetEntity", appID).
		Return(entity, (*entityprovider.EntityProviderError)(nil))
}

func (suite *FlowMetaServiceTestSuite) TestGetFlowMetadata_APP_Success() {
	// Arrange
	appID := testAppID
	ouID := testOUID
	metaType := MetaTypeAPP
	language := "en"
	namespace := "auth"

	suite.expectInboundLookup(appID, "Test App", true, map[string]interface{}{
		"logo_url":   "https://example.com/logo.png",
		"url":        "https://example.com",
		"tos_uri":    "https://example.com/tos",
		"policy_uri": "https://example.com/policy",
	})

	mockOUList := &ou.OrganizationUnitListResponse{
		TotalResults: 1,
		OrganizationUnits: []ou.OrganizationUnitBasic{
			{ID: ouID, Handle: "default", Name: "Default OU"},
		},
	}

	mockOU := ou.OrganizationUnit{
		ID:          ouID,
		Handle:      "default",
		Name:        "Default OU",
		Description: "Default organization",
		LogoURL:     "https://example.com/ou-logo.png",
	}

	mockDesign := &common.DesignResponse{
		Theme:  json.RawMessage(`{"primary":"#000000"}`),
		Layout: json.RawMessage(`{"header":"simple"}`),
	}

	mockTranslations := &i18nmgt.LanguageTranslationsResponse{
		Language:     language,
		TotalResults: 2,
		Translations: map[string]map[string]string{
			"auth": {
				"login.button": "Login",
				"login.title":  "Welcome",
			},
		},
	}

	suite.mockOUService.On("GetOrganizationUnitList", mock.Anything, 1, 0, mock.Anything).Return(mockOUList, nil)
	suite.mockOUService.On("GetOrganizationUnit", mock.Anything, ouID).Return(mockOU, nil)
	suite.mockDesignResolve.On("ResolveDesign", mock.Anything, common.DesignResolveTypeAPP, appID).
		Return(mockDesign, nil)
	suite.mockI18nService.On("ResolveTranslations", language, namespace).Return(mockTranslations, nil)
	suite.mockI18nService.On("ListLanguages").Return([]string{"en", "es"}, nil)

	// Act
	result, svcErr := suite.service.GetFlowMetadata(suite.ctx, metaType, appID, &language, &namespace)

	// Assert
	assert.Nil(suite.T(), svcErr)
	assert.NotNil(suite.T(), result)
	assert.True(suite.T(), result.IsRegistrationFlowEnabled)
	assert.NotNil(suite.T(), result.Application)
	assert.Equal(suite.T(), appID, result.Application.ID)
	assert.Equal(suite.T(), "Test App", result.Application.Name)
	assert.Equal(suite.T(), "https://example.com/logo.png", result.Application.LogoURL)
	assert.Equal(suite.T(), "https://example.com", result.Application.URL)
	assert.NotNil(suite.T(), result.OU)
	assert.Equal(suite.T(), ouID, result.OU.ID)
	assert.NotNil(suite.T(), result.Design.Theme)
	assert.NotNil(suite.T(), result.Design.Layout)
	assert.Equal(suite.T(), 2, len(result.I18n.Languages))
	assert.Equal(suite.T(), 2, result.I18n.TotalResults)
}

func (suite *FlowMetaServiceTestSuite) TestGetFlowMetadata_OU_Success() {
	// Arrange
	ouID := testOUID
	metaType := MetaTypeOU

	mockOU := ou.OrganizationUnit{
		ID:          ouID,
		Handle:      "engineering",
		Name:        "Engineering OU",
		Description: "Engineering unit",
		LogoURL:     "https://example.com/eng-logo.png",
	}

	mockDesign := &common.DesignResponse{
		Theme:  json.RawMessage(`{}`),
		Layout: json.RawMessage(`{}`),
	}

	mockTranslations := &i18nmgt.LanguageTranslationsResponse{
		Language:     "en",
		TotalResults: 0,
		Translations: map[string]map[string]string{},
	}

	suite.mockOUService.On("GetOrganizationUnit", mock.Anything, ouID).Return(mockOU, nil)
	suite.mockDesignResolve.On("ResolveDesign", mock.Anything, common.DesignResolveTypeOU, ouID).Return(mockDesign, nil)
	suite.mockI18nService.On("ResolveTranslations", "en-US", "").Return(mockTranslations, nil)
	suite.mockI18nService.On("ListLanguages").Return([]string{"en"}, nil)

	// Act
	result, svcErr := suite.service.GetFlowMetadata(suite.ctx, metaType, ouID, nil, nil)

	// Assert
	assert.Nil(suite.T(), svcErr)
	assert.NotNil(suite.T(), result)
	assert.False(suite.T(), result.IsRegistrationFlowEnabled)
	assert.Nil(suite.T(), result.Application)
	assert.NotNil(suite.T(), result.OU)
	assert.Equal(suite.T(), ouID, result.OU.ID)
	assert.Equal(suite.T(), "engineering", result.OU.Handle)
}

func (suite *FlowMetaServiceTestSuite) TestGetFlowMetadata_InvalidType() {
	// Arrange
	metaType := MetaType("INVALID")
	id := "some-id"

	// Act
	result, svcErr := suite.service.GetFlowMetadata(suite.ctx, metaType, id, nil, nil)

	// Assert
	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), ErrorInvalidType.Code, svcErr.Code)
}

func (suite *FlowMetaServiceTestSuite) TestGetFlowMetadata_ApplicationNotFound() {
	// Arrange
	metaType := MetaTypeAPP
	appID := "non-existent"

	suite.mockInboundClient.On("GetInboundClientByEntityID", mock.Anything, appID).
		Return(nil, inboundclient.ErrInboundClientNotFound)

	// Act
	result, svcErr := suite.service.GetFlowMetadata(suite.ctx, metaType, appID, nil, nil)

	// Assert
	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), ErrorApplicationNotFound.Code, svcErr.Code)
}

func (suite *FlowMetaServiceTestSuite) TestGetFlowMetadata_OUNotFound() {
	// Arrange
	metaType := MetaTypeOU
	ouID := "non-existent"

	suite.mockOUService.On("GetOrganizationUnit", mock.Anything, ouID).
		Return(ou.OrganizationUnit{}, &ou.ErrorOrganizationUnitNotFound)

	// Act
	result, svcErr := suite.service.GetFlowMetadata(suite.ctx, metaType, ouID, nil, nil)

	// Assert
	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), ErrorOUNotFound.Code, svcErr.Code)
}

func (suite *FlowMetaServiceTestSuite) TestGetFlowMetadata_DesignResolveError_ContinuesWithEmptyDesign() {
	// Arrange
	appID := testAppID
	ouID := testOUID
	metaType := MetaTypeAPP

	suite.expectInboundLookup(appID, "Test App", false, nil)

	mockOUList := &ou.OrganizationUnitListResponse{
		TotalResults: 1,
		OrganizationUnits: []ou.OrganizationUnitBasic{
			{ID: ouID, Handle: "default", Name: "Default OU"},
		},
	}

	mockOU := ou.OrganizationUnit{
		ID:     ouID,
		Handle: "default",
		Name:   "Default OU",
	}

	suite.mockOUService.On("GetOrganizationUnitList", mock.Anything, 1, 0, mock.Anything).Return(mockOUList, nil)
	suite.mockOUService.On("GetOrganizationUnit", mock.Anything, ouID).Return(mockOU, nil)
	suite.mockDesignResolve.On("ResolveDesign", mock.Anything, common.DesignResolveTypeAPP, appID).
		Return(nil, &serviceerror.InternalServerError)
	suite.mockI18nService.On("ResolveTranslations", "en-US", "").
		Return(&i18nmgt.LanguageTranslationsResponse{
			Language:     "en-US",
			TotalResults: 0,
			Translations: map[string]map[string]string{},
		}, nil)
	suite.mockI18nService.On("ListLanguages").Return([]string{"en"}, nil)

	// Act
	result, svcErr := suite.service.GetFlowMetadata(suite.ctx, metaType, appID, nil, nil)

	// Assert - Should succeed with empty design
	assert.Nil(suite.T(), svcErr)
	assert.NotNil(suite.T(), result)
	assert.NotNil(suite.T(), result.Design.Theme)
	assert.NotNil(suite.T(), result.Design.Layout)
}

func (suite *FlowMetaServiceTestSuite) TestGetFlowMetadata_I18nError_ContinuesWithEmptyTranslations() {
	// Arrange
	ouID := testOUID
	metaType := MetaTypeOU

	mockOU := ou.OrganizationUnit{
		ID:     ouID,
		Handle: "default",
		Name:   "Default OU",
	}

	suite.mockOUService.On("GetOrganizationUnit", mock.Anything, ouID).Return(mockOU, nil)
	suite.mockDesignResolve.On("ResolveDesign", mock.Anything, common.DesignResolveTypeOU, ouID).
		Return(&common.DesignResponse{
			Theme:  json.RawMessage(`{}`),
			Layout: json.RawMessage(`{}`),
		}, nil)
	suite.mockI18nService.On("ResolveTranslations", "en-US", "").
		Return(nil, &serviceerror.ServiceError{Code: "I18N-5000", Type: serviceerror.ServerErrorType})
	suite.mockI18nService.On("ListLanguages").Return([]string{"en"}, nil)

	// Act
	result, svcErr := suite.service.GetFlowMetadata(suite.ctx, metaType, ouID, nil, nil)

	// Assert - Should succeed with empty translations
	assert.Nil(suite.T(), svcErr)
	assert.NotNil(suite.T(), result)
	assert.NotNil(suite.T(), result.I18n.Translations)
	assert.Equal(suite.T(), 0, len(result.I18n.Translations))
}

func (suite *FlowMetaServiceTestSuite) TestGetFlowMetadata_SystemFlow_NoTypeOrID() {
	// Arrange: no type or id — system flow returns i18n only, skips app/OU/design
	mockTranslations := &i18nmgt.LanguageTranslationsResponse{
		Language:     "en-US",
		TotalResults: 3,
		Translations: map[string]map[string]string{
			"system": {"error.internal": "Internal error"},
		},
	}

	suite.mockI18nService.On("ResolveTranslations", "en-US", "").Return(mockTranslations, nil)
	suite.mockI18nService.On("ListLanguages").Return([]string{"en-US"}, nil)

	// Act
	result, svcErr := suite.service.GetFlowMetadata(suite.ctx, MetaType(""), "", nil, nil)

	// Assert
	assert.Nil(suite.T(), svcErr)
	assert.NotNil(suite.T(), result)
	assert.False(suite.T(), result.IsRegistrationFlowEnabled)
	assert.Nil(suite.T(), result.Application)
	assert.Nil(suite.T(), result.OU)
	assert.Equal(suite.T(), "en-US", result.I18n.Language)
	assert.Equal(suite.T(), 3, result.I18n.TotalResults)
	assert.Equal(suite.T(), []string{"en-US"}, result.I18n.Languages)
	assert.Contains(suite.T(), result.I18n.Translations, "system")
}
