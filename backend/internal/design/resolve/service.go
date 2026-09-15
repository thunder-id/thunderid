// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package resolve provides functionality for resolving design configurations.
package resolve

import (
	"context"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/application"
	"github.com/thunder-id/thunderid/internal/design/common"
	layoutmgt "github.com/thunder-id/thunderid/internal/design/layout/mgt"
	thememgt "github.com/thunder-id/thunderid/internal/design/theme/mgt"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const serviceLogger = "DesignResolveService"

// DesignResolveServiceInterface defines the interface for the design resolve service.
type DesignResolveServiceInterface interface {
	ResolveDesign(
		ctx context.Context, resolveType providers.DesignResolveType, id string,
	) (*providers.DesignResponse, *tidcommon.ServiceError)
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
	ctx context.Context, resolveType providers.DesignResolveType, id string,
) (*providers.DesignResponse, *tidcommon.ServiceError) {
	if resolveType == "" {
		return nil, &common.ErrorInvalidResolveType
	}

	if id == "" {
		return nil, &common.ErrorMissingResolveID
	}

	// Currently only APP type is supported
	if resolveType != providers.DesignResolveTypeAPP {
		return nil, &common.ErrorUnsupportedResolveType
	}

	// Get the application by ID
	if drs.applicationService == nil {
		drs.logger.Error(ctx, "Application service is not available")
		return nil, &tidcommon.InternalServerError
	}

	app, svcErr := drs.applicationService.GetApplication(ctx, id)
	if svcErr != nil {
		// Convert application service errors to design resolve errors
		if svcErr.Code == application.ErrorInvalidApplicationID.Code {
			return nil, &common.ErrorMissingResolveID
		}
		// This endpoint is unauthenticated, so an unknown application reports the same result as a
		// known one with nothing configured. Distinguishing them lets an anonymous caller confirm
		// which application IDs exist.
		if svcErr.Code == application.ErrorApplicationNotFound.Code {
			drs.logger.Debug(ctx, "No design resolved; application does not exist",
				log.String("applicationId", id))
			return nil, &common.ErrorApplicationHasNoDesign
		}
		return nil, svcErr
	}

	// Check if the application has theme or layout configured
	if app.ThemeID == "" && app.LayoutID == "" {
		return nil, &common.ErrorApplicationHasNoDesign
	}

	designResponse := &providers.DesignResponse{}

	// Get theme configuration if available
	if app.ThemeID != "" {
		if drs.themeMgtService == nil {
			drs.logger.Error(ctx, "Theme management service is not available")
			return nil, &tidcommon.InternalServerError
		}

		themeConfig, svcErr := drs.themeMgtService.GetTheme(ctx, app.ThemeID)
		if svcErr != nil {
			if svcErr.Code == thememgt.ErrorThemeNotFound.Code {
				// The referenced theme has been deleted; fall back to the system default by leaving
				// the theme unset in the response.
				drs.logger.Warn(ctx, "Application references a deleted theme; falling back to default",
					log.String("applicationId", id),
					log.String("themeId", app.ThemeID))
			} else {
				return nil, svcErr
			}
		} else {
			designResponse.Theme = themeConfig.Theme
		}
	}

	// Get layout configuration if available
	if app.LayoutID != "" {
		if drs.layoutMgtService == nil {
			drs.logger.Error(ctx, "Layout management service is not available")
			return nil, &tidcommon.InternalServerError
		}

		layoutConfig, svcErr := drs.layoutMgtService.GetLayout(ctx, app.LayoutID)
		if svcErr != nil {
			if svcErr.Code == layoutmgt.ErrorLayoutNotFound.Code {
				// The referenced layout has been deleted; fall back to the system default by leaving
				// the layout unset in the response.
				drs.logger.Warn(ctx, "Application references a deleted layout; falling back to default",
					log.String("applicationId", id),
					log.String("layoutId", app.LayoutID))
			} else {
				return nil, svcErr
			}
		} else {
			designResponse.Layout = layoutConfig.Layout
		}
	}

	drs.logger.Debug(ctx, "Successfully resolved design configuration",
		log.String("type", string(resolveType)),
		log.String("id", id),
		log.String("themeId", app.ThemeID),
		log.String("layoutId", app.LayoutID))

	return designResponse, nil
}
