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

package resolve

import (
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"

	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/application"
	appmodel "github.com/thunder-id/thunderid/internal/application/model"
	"github.com/thunder-id/thunderid/internal/design/common"
	layoutmgt "github.com/thunder-id/thunderid/internal/design/layout/mgt"
	thememgt "github.com/thunder-id/thunderid/internal/design/theme/mgt"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/tests/mocks/applicationmock"
	"github.com/thunder-id/thunderid/tests/mocks/design/layoutmock"
	"github.com/thunder-id/thunderid/tests/mocks/design/thememock"
)

// Test Suite
type ResolveServiceTestSuite struct {
	suite.Suite
	mockThemeService  *thememock.ThemeMgtServiceInterfaceMock
	mockLayoutService *layoutmock.LayoutMgtServiceInterfaceMock
	mockAppService    *applicationmock.ApplicationServiceInterfaceMock
	service           DesignResolveServiceInterface
}

func TestResolveServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ResolveServiceTestSuite))
}

func (suite *ResolveServiceTestSuite) SetupTest() {
	suite.mockThemeService = thememock.NewThemeMgtServiceInterfaceMock(suite.T())
	suite.mockLayoutService = layoutmock.NewLayoutMgtServiceInterfaceMock(suite.T())
	suite.mockAppService = applicationmock.NewApplicationServiceInterfaceMock(suite.T())
	suite.service = newDesignResolveService(suite.mockThemeService, suite.mockLayoutService, suite.mockAppService)
}

// Test ResolveDesign - Empty resolve type
func (suite *ResolveServiceTestSuite) TestResolveDesign_EmptyResolveType() {
	result, err := suite.service.ResolveDesign(context.Background(), "", "00000000-0000-0000-0000-000000000001")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), common.ErrorInvalidResolveType.Code, err.Code)
}

// Test ResolveDesign - Empty ID
func (suite *ResolveServiceTestSuite) TestResolveDesign_EmptyID() {
	result, err := suite.service.ResolveDesign(context.Background(), common.DesignResolveTypeAPP, "")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), common.ErrorMissingResolveID.Code, err.Code)
}

// Test ResolveDesign - Unsupported resolve type
func (suite *ResolveServiceTestSuite) TestResolveDesign_UnsupportedType() {
	result, err := suite.service.ResolveDesign(context.Background(), common.DesignResolveTypeOU,
		"00000000-0000-0000-0000-000000000002")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), common.ErrorUnsupportedResolveType.Code, err.Code)
}

// Test ResolveDesign - Nil application service
func (suite *ResolveServiceTestSuite) TestResolveDesign_NilApplicationService() {
	service := newDesignResolveService(suite.mockThemeService, suite.mockLayoutService, nil)

	result, err := service.ResolveDesign(context.Background(), common.DesignResolveTypeAPP,
		"00000000-0000-0000-0000-000000000001")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), serviceerror.InternalServerError.Code, err.Code)
}

// Test ResolveDesign - Application not found
func (suite *ResolveServiceTestSuite) TestResolveDesign_ApplicationNotFound() {
	suite.mockAppService.On("GetApplication", mock.Anything, "00000000-0000-0000-0000-000000000099").
		Return(nil, &application.ErrorApplicationNotFound)

	result, err := suite.service.ResolveDesign(context.Background(), common.DesignResolveTypeAPP,
		"00000000-0000-0000-0000-000000000099")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), common.ErrorApplicationNotFound.Code, err.Code)
}

// Test ResolveDesign - Invalid application ID (passed through to app service)
func (suite *ResolveServiceTestSuite) TestResolveDesign_InvalidApplicationID() {
	suite.mockAppService.On("GetApplication", mock.Anything, "invalid").
		Return(nil, &application.ErrorInvalidApplicationID)

	result, err := suite.service.ResolveDesign(context.Background(), common.DesignResolveTypeAPP, "invalid")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), common.ErrorMissingResolveID.Code, err.Code)
}

// Test ResolveDesign - Application service error propagation
func (suite *ResolveServiceTestSuite) TestResolveDesign_ApplicationServiceError() {
	svcErr := &serviceerror.ServiceError{
		Code:  "APP-9999",
		Error: core.I18nMessage{Key: "error.test.unexpected_error", DefaultValue: "unexpected error"},
	}
	suite.mockAppService.On("GetApplication", mock.Anything, "00000000-0000-0000-0000-000000000001").Return(nil, svcErr)

	result, err := suite.service.ResolveDesign(context.Background(), common.DesignResolveTypeAPP,
		"00000000-0000-0000-0000-000000000001")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), svcErr.Code, err.Code)
}

// Test ResolveDesign - Application has no design
func (suite *ResolveServiceTestSuite) TestResolveDesign_ApplicationHasNoDesign() {
	app := &appmodel.Application{
		ID:   "00000000-0000-0000-0000-000000000001",
		Name: "Test App",
		InboundAuthProfile: inboundmodel.InboundAuthProfile{
			ThemeID:  "",
			LayoutID: "",
		},
	}
	suite.mockAppService.On("GetApplication", mock.Anything, "00000000-0000-0000-0000-000000000001").Return(app, nil)

	result, err := suite.service.ResolveDesign(context.Background(), common.DesignResolveTypeAPP,
		"00000000-0000-0000-0000-000000000001")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), common.ErrorApplicationHasNoDesign.Code, err.Code)
}

// Test ResolveDesign - Success with theme only
func (suite *ResolveServiceTestSuite) TestResolveDesign_SuccessWithThemeOnly() {
	app := &appmodel.Application{
		ID:   "00000000-0000-0000-0000-000000000001",
		Name: "Test App",
		InboundAuthProfile: inboundmodel.InboundAuthProfile{
			ThemeID:  "theme-123",
			LayoutID: "",
		},
	}
	themeConfig := &thememgt.Theme{
		ID:          "theme-123",
		DisplayName: "Test Theme",
		Theme:       json.RawMessage(`{"colors": {"primary": "#007bff"}}`),
	}

	suite.mockAppService.On("GetApplication", mock.Anything, "00000000-0000-0000-0000-000000000001").Return(app, nil)
	suite.mockThemeService.On("GetTheme", "theme-123").Return(themeConfig, nil)

	result, err := suite.service.ResolveDesign(context.Background(), common.DesignResolveTypeAPP,
		"00000000-0000-0000-0000-000000000001")

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.NotNil(suite.T(), result.Theme)
	assert.Nil(suite.T(), result.Layout)
}

// Test ResolveDesign - Success with layout only
func (suite *ResolveServiceTestSuite) TestResolveDesign_SuccessWithLayoutOnly() {
	app := &appmodel.Application{
		ID:   "00000000-0000-0000-0000-000000000001",
		Name: "Test App",
		InboundAuthProfile: inboundmodel.InboundAuthProfile{
			ThemeID:  "",
			LayoutID: "layout-123",
		},
	}
	layoutConfig := &layoutmgt.Layout{
		ID:          "layout-123",
		DisplayName: "Test Layout",
		Layout:      json.RawMessage(`{"structure": "centered"}`),
	}

	suite.mockAppService.On("GetApplication", mock.Anything, "00000000-0000-0000-0000-000000000001").Return(app, nil)
	suite.mockLayoutService.On("GetLayout", "layout-123").Return(layoutConfig, nil)

	result, err := suite.service.ResolveDesign(context.Background(), common.DesignResolveTypeAPP,
		"00000000-0000-0000-0000-000000000001")

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Nil(suite.T(), result.Theme)
	assert.NotNil(suite.T(), result.Layout)
}

// Test ResolveDesign - Success with both theme and layout
func (suite *ResolveServiceTestSuite) TestResolveDesign_SuccessWithBoth() {
	app := &appmodel.Application{
		ID:   "00000000-0000-0000-0000-000000000001",
		Name: "Test App",
		InboundAuthProfile: inboundmodel.InboundAuthProfile{
			ThemeID:  "theme-123",
			LayoutID: "layout-123",
		},
	}
	themeConfig := &thememgt.Theme{
		ID:          "theme-123",
		DisplayName: "Test Theme",
		Theme:       json.RawMessage(`{"colors": {"primary": "#007bff"}}`),
	}
	layoutConfig := &layoutmgt.Layout{
		ID:          "layout-123",
		DisplayName: "Test Layout",
		Layout:      json.RawMessage(`{"structure": "centered"}`),
	}

	suite.mockAppService.On("GetApplication", mock.Anything, "00000000-0000-0000-0000-000000000001").Return(app, nil)
	suite.mockThemeService.On("GetTheme", "theme-123").Return(themeConfig, nil)
	suite.mockLayoutService.On("GetLayout", "layout-123").Return(layoutConfig, nil)

	result, err := suite.service.ResolveDesign(context.Background(), common.DesignResolveTypeAPP,
		"00000000-0000-0000-0000-000000000001")

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.NotNil(suite.T(), result.Theme)
	assert.NotNil(suite.T(), result.Layout)
}

// Test ResolveDesign - Theme not found (data integrity issue)
func (suite *ResolveServiceTestSuite) TestResolveDesign_ThemeNotFound() {
	app := &appmodel.Application{
		ID:   "00000000-0000-0000-0000-000000000001",
		Name: "Test App",
		InboundAuthProfile: inboundmodel.InboundAuthProfile{
			ThemeID:  "theme-missing",
			LayoutID: "",
		},
	}
	suite.mockAppService.On("GetApplication", mock.Anything, "00000000-0000-0000-0000-000000000001").Return(app, nil)
	suite.mockThemeService.On("GetTheme", "theme-missing").
		Return(nil, &thememgt.ErrorThemeNotFound)

	result, err := suite.service.ResolveDesign(context.Background(), common.DesignResolveTypeAPP,
		"00000000-0000-0000-0000-000000000001")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), serviceerror.InternalServerError.Code, err.Code)
}

// Test ResolveDesign - Theme service error propagation
func (suite *ResolveServiceTestSuite) TestResolveDesign_ThemeServiceError() {
	app := &appmodel.Application{
		ID:   "00000000-0000-0000-0000-000000000001",
		Name: "Test App",
		InboundAuthProfile: inboundmodel.InboundAuthProfile{
			ThemeID:  "theme-123",
			LayoutID: "",
		},
	}
	svcErr := &serviceerror.ServiceError{
		Code:  "THM-9999",
		Error: core.I18nMessage{Key: "error.test.unexpected_error", DefaultValue: "unexpected error"},
	}
	suite.mockAppService.On("GetApplication", mock.Anything, "00000000-0000-0000-0000-000000000001").Return(app, nil)
	suite.mockThemeService.On("GetTheme", "theme-123").Return(nil, svcErr)

	result, err := suite.service.ResolveDesign(context.Background(), common.DesignResolveTypeAPP,
		"00000000-0000-0000-0000-000000000001")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "THM-9999", err.Code)
}

// Test ResolveDesign - Nil theme service
func (suite *ResolveServiceTestSuite) TestResolveDesign_NilThemeService() {
	service := newDesignResolveService(nil, suite.mockLayoutService, suite.mockAppService)
	app := &appmodel.Application{
		ID:   "00000000-0000-0000-0000-000000000001",
		Name: "Test App",
		InboundAuthProfile: inboundmodel.InboundAuthProfile{
			ThemeID:  "theme-123",
			LayoutID: "",
		},
	}
	suite.mockAppService.On("GetApplication", mock.Anything, "00000000-0000-0000-0000-000000000001").Return(app, nil)

	result, err := service.ResolveDesign(context.Background(), common.DesignResolveTypeAPP,
		"00000000-0000-0000-0000-000000000001")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), serviceerror.InternalServerError.Code, err.Code)
}

// Test ResolveDesign - Layout not found (data integrity issue)
func (suite *ResolveServiceTestSuite) TestResolveDesign_LayoutNotFound() {
	app := &appmodel.Application{
		ID:   "00000000-0000-0000-0000-000000000001",
		Name: "Test App",
		InboundAuthProfile: inboundmodel.InboundAuthProfile{
			ThemeID:  "",
			LayoutID: "layout-missing",
		},
	}
	suite.mockAppService.On("GetApplication", mock.Anything, "00000000-0000-0000-0000-000000000001").Return(app, nil)
	suite.mockLayoutService.On("GetLayout", "layout-missing").
		Return(nil, &layoutmgt.ErrorLayoutNotFound)

	result, err := suite.service.ResolveDesign(context.Background(), common.DesignResolveTypeAPP,
		"00000000-0000-0000-0000-000000000001")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), serviceerror.InternalServerError.Code, err.Code)
}

// Test ResolveDesign - Layout service error propagation
func (suite *ResolveServiceTestSuite) TestResolveDesign_LayoutServiceError() {
	app := &appmodel.Application{
		ID:   "00000000-0000-0000-0000-000000000001",
		Name: "Test App",
		InboundAuthProfile: inboundmodel.InboundAuthProfile{
			ThemeID:  "",
			LayoutID: "layout-123",
		},
	}
	svcErr := &serviceerror.ServiceError{
		Code:  "LAY-9999",
		Error: core.I18nMessage{Key: "error.test.unexpected_error", DefaultValue: "unexpected error"},
	}
	suite.mockAppService.On("GetApplication", mock.Anything, "00000000-0000-0000-0000-000000000001").Return(app, nil)
	suite.mockLayoutService.On("GetLayout", "layout-123").Return(nil, svcErr)

	result, err := suite.service.ResolveDesign(context.Background(), common.DesignResolveTypeAPP,
		"00000000-0000-0000-0000-000000000001")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "LAY-9999", err.Code)
}

// Test ResolveDesign - Nil layout service
func (suite *ResolveServiceTestSuite) TestResolveDesign_NilLayoutService() {
	service := newDesignResolveService(suite.mockThemeService, nil, suite.mockAppService)
	app := &appmodel.Application{
		ID:   "00000000-0000-0000-0000-000000000001",
		Name: "Test App",
		InboundAuthProfile: inboundmodel.InboundAuthProfile{
			ThemeID:  "",
			LayoutID: "layout-123",
		},
	}
	suite.mockAppService.On("GetApplication", mock.Anything, "00000000-0000-0000-0000-000000000001").Return(app, nil)

	result, err := service.ResolveDesign(context.Background(), common.DesignResolveTypeAPP,
		"00000000-0000-0000-0000-000000000001")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), serviceerror.InternalServerError.Code, err.Code)
}
