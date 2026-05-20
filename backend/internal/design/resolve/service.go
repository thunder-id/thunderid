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

// Package resolve provides functionality for resolving design configurations.
package resolve

import (
	"context"

	"github.com/thunder-id/thunderid/internal/application"
	"github.com/thunder-id/thunderid/internal/design/common"
	layoutmgt "github.com/thunder-id/thunderid/internal/design/layout/mgt"
	thememgt "github.com/thunder-id/thunderid/internal/design/theme/mgt"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const serviceLogger = "DesignResolveService"

// DesignResolveServiceInterface defines the interface for the design resolve service.
type DesignResolveServiceInterface interface {
	ResolveDesign(
		ctx context.Context, resolveType common.DesignResolveType, id string,
	) (*common.DesignResponse, *serviceerror.ServiceError)
}

// designResolveService is the default implementation of the DesignResolveServiceInterface.
type designResolveService struct {
	themeMgtService    thememgt.ThemeMgtServiceInterface
	layoutMgtService   layoutmgt.LayoutMgtServiceInterface
	applicationService application.ApplicationServiceInterface
	logger             *log.Logger
}

// newDesignResolveService creates a new instance of DesignResolveService with injected dependencies.
func newDesignResolveService(
	themeMgtService thememgt.ThemeMgtServiceInterface,
	layoutMgtService layoutmgt.LayoutMgtServiceInterface,
	applicationService application.ApplicationServiceInterface,
) DesignResolveServiceInterface {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, serviceLogger))
	return &designResolveService{
		themeMgtService:    themeMgtService,
		layoutMgtService:   layoutMgtService,
		applicationService: applicationService,
		logger:             logger,
	}
}

// ResolveDesign resolves a design configuration by type and ID.
// TODO: Add support for OU type and fallback logic.
func (drs *designResolveService) ResolveDesign(
	ctx context.Context, resolveType common.DesignResolveType, id string,
) (*common.DesignResponse, *serviceerror.ServiceError) {
	if resolveType == "" {
		return nil, &common.ErrorInvalidResolveType
	}

	if id == "" {
		return nil, &common.ErrorMissingResolveID
	}

	// Currently only APP type is supported
	if resolveType != common.DesignResolveTypeAPP {
		return nil, &common.ErrorUnsupportedResolveType
	}

	// Get the application by ID
	if drs.applicationService == nil {
		drs.logger.Error("Application service is not available")
		return nil, &serviceerror.InternalServerError
	}

	app, svcErr := drs.applicationService.GetApplication(ctx, id)
	if svcErr != nil {
		// Convert application service errors to design resolve errors
		if svcErr.Code == application.ErrorInvalidApplicationID.Code {
			return nil, &common.ErrorMissingResolveID
		}
		if svcErr.Code == application.ErrorApplicationNotFound.Code {
			return nil, &common.ErrorApplicationNotFound
		}
		return nil, svcErr
	}

	// Check if the application has theme or layout configured
	if app.ThemeID == "" && app.LayoutID == "" {
		return nil, &common.ErrorApplicationHasNoDesign
	}

	designResponse := &common.DesignResponse{}

	// Get theme configuration if available
	if app.ThemeID != "" {
		if drs.themeMgtService == nil {
			drs.logger.Error("Theme management service is not available")
			return nil, &serviceerror.InternalServerError
		}

		themeConfig, svcErr := drs.themeMgtService.GetTheme(app.ThemeID)
		if svcErr != nil {
			if svcErr.Code == thememgt.ErrorThemeNotFound.Code {
				drs.logger.Error("Data integrity issue: application references non-existent theme",
					log.String("applicationId", id),
					log.String("themeId", app.ThemeID))
				return nil, &serviceerror.InternalServerError
			}
			return nil, svcErr
		}

		designResponse.Theme = themeConfig.Theme
	}

	// Get layout configuration if available
	if app.LayoutID != "" {
		if drs.layoutMgtService == nil {
			drs.logger.Error("Layout management service is not available")
			return nil, &serviceerror.InternalServerError
		}

		layoutConfig, svcErr := drs.layoutMgtService.GetLayout(app.LayoutID)
		if svcErr != nil {
			if svcErr.Code == layoutmgt.ErrorLayoutNotFound.Code {
				drs.logger.Error("Data integrity issue: application references non-existent layout",
					log.String("applicationId", id),
					log.String("layoutId", app.LayoutID))
				return nil, &serviceerror.InternalServerError
			}
			return nil, svcErr
		}

		designResponse.Layout = layoutConfig.Layout
	}

	drs.logger.Debug("Successfully resolved design configuration",
		log.String("type", string(resolveType)),
		log.String("id", id),
		log.String("themeId", app.ThemeID),
		log.String("layoutId", app.LayoutID))

	return designResponse, nil
}
