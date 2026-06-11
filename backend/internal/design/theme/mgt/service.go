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

// Package thememgt provides theme management functionality.
package thememgt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const loggerComponentName = "ThemeMgtService"

// ApplicationUsageReader is defined here (consumer side) so the theme service can query
// application references without importing the application package.
type ApplicationUsageReader interface {
	GetApplicationsByThemeID(ctx context.Context, themeID string, limit, offset int) ([]ApplicationUsage, int, error)
}

// ApplicationUsage is a minimal projection of an application used by the usages endpoint.
type ApplicationUsage struct {
	ID          string
	DisplayName string
}

// ThemeMgtServiceInterface defines the interface for the theme management service.
type ThemeMgtServiceInterface interface {
	GetThemeList(ctx context.Context, limit, offset int) (*ThemeList, *serviceerror.ServiceError)
	CreateTheme(ctx context.Context, theme CreateThemeRequestWithID) (*Theme, *serviceerror.ServiceError)
	GetTheme(ctx context.Context, id string) (*Theme, *serviceerror.ServiceError)
	UpdateTheme(ctx context.Context, id string, theme UpdateThemeRequest) (*Theme, *serviceerror.ServiceError)
	DeleteTheme(ctx context.Context, id string) *serviceerror.ServiceError
	IsThemeExist(ctx context.Context, id string) (bool, *serviceerror.ServiceError)
	SetApplicationReader(r ApplicationUsageReader)
	GetThemeUsages(ctx context.Context, id string, limit, offset int) (*ThemeUsagesResponse, *serviceerror.ServiceError)
}

// themeMgtService is the default implementation of the ThemeMgtServiceInterface.
type themeMgtService struct {
	themeMgtStore     themeMgtStoreInterface
	applicationReader ApplicationUsageReader
	logger            *log.Logger
}

// newThemeMgtService creates a new instance of ThemeMgtService with injected dependencies.
func newThemeMgtService(themeMgtStore themeMgtStoreInterface) ThemeMgtServiceInterface {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	return &themeMgtService{
		themeMgtStore: themeMgtStore,
		logger:        logger,
	}
}

// GetThemeList retrieves a list of theme configurations.
func (ts *themeMgtService) GetThemeList(
	ctx context.Context, limit, offset int) (*ThemeList, *serviceerror.ServiceError) {
	if err := validatePaginationParams(limit, offset); err != nil {
		return nil, err
	}

	totalCount, err := ts.themeMgtStore.GetThemeListCount()
	if err != nil {
		ts.logger.ErrorWithContext(ctx, "Failed to get theme count", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	themes, err := ts.themeMgtStore.GetThemeList(limit, offset)
	if err != nil {
		ts.logger.ErrorWithContext(ctx, "Failed to list themes", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	response := &ThemeList{
		TotalResults: totalCount,
		Themes:       themes,
		StartIndex:   offset + 1,
		Count:        len(themes),
		Links:        buildPaginationLinks(limit, offset, totalCount),
	}

	return response, nil
}

// CreateTheme creates a new theme configuration.
func (ts *themeMgtService) CreateTheme(
	ctx context.Context, theme CreateThemeRequestWithID) (*Theme, *serviceerror.ServiceError) {
	ts.logger.DebugWithContext(ctx, "Creating theme configuration")

	if theme.DisplayName == "" {
		return nil, &ErrorMissingDisplayName
	}

	if theme.Handle == "" {
		return nil, &ErrorMissingThemeHandle
	}

	// Check if store is in pure declarative mode
	if isDeclarativeModeEnabled() {
		return nil, &ErrorCannotModifyDeclarativeResource
	}

	conflict, err := ts.themeMgtStore.IsThemeHandleConflict(theme.Handle, "")
	if err != nil {
		ts.logger.ErrorWithContext(ctx, "Failed to check theme handle conflict", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}
	if conflict {
		return nil, &ErrorDuplicateThemeHandle
	}

	if err := ts.validateThemePreferences(ctx, theme.Theme); err != nil {
		return nil, err
	}

	id := theme.ID
	if id == "" {
		var err error
		id, err = utils.GenerateUUIDv7()
		if err != nil {
			ts.logger.ErrorWithContext(ctx, "Failed to generate UUID", log.Error(err))
			return nil, &serviceerror.InternalServerError
		}
	}

	storeReq := CreateThemeRequest{
		Handle:      theme.Handle,
		DisplayName: theme.DisplayName,
		Description: theme.Description,
		Theme:       theme.Theme,
	}

	if err := ts.themeMgtStore.CreateTheme(id, storeReq); err != nil {
		ts.logger.ErrorWithContext(ctx, "Failed to create theme", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	createdTheme := &Theme{
		ID:          id,
		Handle:      theme.Handle,
		DisplayName: theme.DisplayName,
		Description: theme.Description,
		Theme:       theme.Theme,
	}

	ts.logger.DebugWithContext(ctx, "Successfully created theme", log.String("id", id))
	return createdTheme, nil
}

// GetTheme retrieves a specific theme configuration by its id.
func (ts *themeMgtService) GetTheme(ctx context.Context, id string) (*Theme, *serviceerror.ServiceError) {
	ts.logger.DebugWithContext(ctx, "Retrieving theme", log.String("id", id))

	if id == "" {
		return nil, &ErrorInvalidThemeID
	}

	theme, err := ts.themeMgtStore.GetTheme(id)
	if err != nil {
		if errors.Is(err, errThemeNotFound) {
			ts.logger.DebugWithContext(ctx, "Theme not found", log.String("id", id))
			return nil, &ErrorThemeNotFound
		}
		ts.logger.ErrorWithContext(ctx, "Failed to retrieve theme", log.String("id", id), log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	ts.logger.DebugWithContext(ctx, "Successfully retrieved theme", log.String("id", theme.ID))
	return &theme, nil
}

// UpdateTheme updates an existing theme configuration.
func (ts *themeMgtService) UpdateTheme(
	ctx context.Context, id string, theme UpdateThemeRequest) (*Theme, *serviceerror.ServiceError) {
	ts.logger.DebugWithContext(ctx, "Updating theme", log.String("id", id))

	if id == "" {
		return nil, &ErrorInvalidThemeID
	}

	if theme.DisplayName == "" {
		return nil, &ErrorMissingDisplayName
	}

	// Check if the theme is declarative (read-only)
	if ts.themeMgtStore.IsThemeDeclarative(id) {
		return nil, &ErrorCannotModifyDeclarativeResource
	}

	// Fetch existing theme to enforce handle immutability
	existingTheme, err := ts.themeMgtStore.GetTheme(id)
	if err != nil {
		if errors.Is(err, errThemeNotFound) {
			return nil, &ErrorThemeNotFound
		}
		ts.logger.ErrorWithContext(ctx, "Failed to retrieve theme", log.String("id", id), log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	// Handle is immutable; reject if a different value is provided
	if theme.Handle != "" && theme.Handle != existingTheme.Handle {
		return nil, &ErrorThemeHandleImmutable
	}

	if err := ts.validateThemePreferences(ctx, theme.Theme); err != nil {
		return nil, err
	}

	if err := ts.themeMgtStore.UpdateTheme(id, theme); err != nil {
		ts.logger.ErrorWithContext(ctx, "Failed to update theme", log.String("id", id), log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	updatedTheme := &Theme{
		ID:          id,
		Handle:      existingTheme.Handle,
		DisplayName: theme.DisplayName,
		Description: theme.Description,
		Theme:       theme.Theme,
	}

	ts.logger.DebugWithContext(ctx, "Successfully updated theme", log.String("id", id))
	return updatedTheme, nil
}

// DeleteTheme deletes a theme configuration.
func (ts *themeMgtService) DeleteTheme(ctx context.Context, id string) *serviceerror.ServiceError {
	ts.logger.DebugWithContext(ctx, "Deleting theme", log.String("id", id))

	if id == "" {
		return &ErrorInvalidThemeID
	}

	// Check if the theme is declarative (read-only)
	if ts.themeMgtStore.IsThemeDeclarative(id) {
		return &ErrorCannotModifyDeclarativeResource
	}

	// Check if theme exists. Return success for non-existing themes (idempotent delete).
	exists, err := ts.themeMgtStore.IsThemeExist(id)
	if err != nil {
		ts.logger.ErrorWithContext(ctx, "Failed to check theme existence", log.String("id", id), log.Error(err))
		return &serviceerror.InternalServerError
	}

	if !exists {
		ts.logger.DebugWithContext(ctx, "Theme not found for deletion, returning success", log.String("id", id))
		return nil
	}

	// Check if theme is used by any applications
	count, err := ts.themeMgtStore.GetApplicationsCountByThemeID(id)
	if err != nil {
		ts.logger.ErrorWithContext(ctx, "Failed to check applications using theme",
			log.String("id", id), log.Error(err))
		return &serviceerror.InternalServerError
	}

	if count > 0 {
		return &ErrorThemeInUse
	}

	if err := ts.themeMgtStore.DeleteTheme(id); err != nil {
		ts.logger.ErrorWithContext(ctx, "Failed to delete theme", log.String("id", id), log.Error(err))
		return &serviceerror.InternalServerError
	}

	ts.logger.DebugWithContext(ctx, "Successfully deleted theme", log.String("id", id))
	return nil
}

// IsThemeExist checks if a theme exists.
func (ts *themeMgtService) IsThemeExist(ctx context.Context, id string) (bool, *serviceerror.ServiceError) {
	if id == "" {
		return false, &ErrorInvalidThemeID
	}

	exists, err := ts.themeMgtStore.IsThemeExist(id)
	if err != nil {
		ts.logger.ErrorWithContext(ctx, "Failed to check theme existence", log.String("id", id), log.Error(err))
		return false, &serviceerror.InternalServerError
	}

	return exists, nil
}

// SetApplicationReader injects the application usage reader. Called by servicemanager
// after both theme and application services are initialized to avoid a cyclic import.
func (ts *themeMgtService) SetApplicationReader(r ApplicationUsageReader) {
	ts.applicationReader = r
}

// GetThemeUsages returns a paginated list of resources that reference this theme.
func (ts *themeMgtService) GetThemeUsages(
	ctx context.Context, id string, limit, offset int) (*ThemeUsagesResponse, *serviceerror.ServiceError) {
	if id == "" {
		return nil, &ErrorInvalidThemeID
	}

	if err := validatePaginationParams(limit, offset); err != nil {
		return nil, err
	}

	exists, err := ts.themeMgtStore.IsThemeExist(id)
	if err != nil {
		ts.logger.ErrorWithContext(ctx, "Failed to check theme existence", log.String("id", id), log.Error(err))
		return nil, &serviceerror.InternalServerError
	}
	if !exists {
		return nil, &ErrorThemeNotFound
	}

	if ts.applicationReader == nil {
		ts.logger.WarnWithContext(ctx, "ApplicationReader not set; returning unknown usages", log.String("id", id))
		return &ThemeUsagesResponse{
			TotalResults: nil,
			StartIndex:   offset + 1,
			Count:        0,
			Summary:      ThemeUsagesSummary{Applications: nil},
			Usages:       []ThemeUsage{},
			Links:        []LinkResponse{},
		}, nil
	}

	apps, total, err := ts.applicationReader.GetApplicationsByThemeID(ctx, id, limit, offset)
	if err != nil {
		ts.logger.ErrorWithContext(ctx, "Failed to get applications by theme ID", log.String("id", id), log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	usages := make([]ThemeUsage, len(apps))
	for i, app := range apps {
		usages[i] = ThemeUsage{
			ResourceType:     "application",
			ID:               app.ID,
			DisplayName:      app.DisplayName,
			BehaviorOnDelete: "fallback",
		}
	}

	links := buildUsagesPaginationLinks(id, limit, offset, total)

	return &ThemeUsagesResponse{
		TotalResults: &total,
		StartIndex:   offset + 1,
		Count:        len(usages),
		Summary:      ThemeUsagesSummary{Applications: &total},
		Usages:       usages,
		Links:        links,
	}, nil
}

// validateThemePreferences validates the theme JSON.
func (ts *themeMgtService) validateThemePreferences(
	ctx context.Context, theme json.RawMessage) *serviceerror.ServiceError {
	if len(theme) == 0 {
		return &ErrorMissingTheme
	}

	var result map[string]interface{}
	if err := json.Unmarshal(theme, &result); err != nil {
		ts.logger.DebugWithContext(ctx, "Invalid theme JSON", log.Error(err))
		return &ErrorInvalidThemeFormat
	}

	return nil
}

// validatePaginationParams validates limit and offset parameters.
func validatePaginationParams(limit, offset int) *serviceerror.ServiceError {
	if limit < 1 || limit > serverconst.MaxPageSize {
		return serviceerror.CustomServiceError(ErrorInvalidLimitValue, core.I18nMessage{
			Key:          "error.themeservice.invalid_limit_value_description",
			DefaultValue: fmt.Sprintf("Limit must be between 1 and %d", serverconst.MaxPageSize),
		})
	}

	if offset < 0 {
		return &ErrorInvalidOffsetValue
	}

	return nil
}

// buildUsagesPaginationLinks builds pagination links for the theme usages response.
func buildUsagesPaginationLinks(themeID string, limit, offset, totalCount int) []LinkResponse {
	links := make([]LinkResponse, 0)
	base := fmt.Sprintf("/design/themes/%s/usages", themeID)
	if offset > 0 {
		prevOffset := offset - limit
		if prevOffset < 0 {
			prevOffset = 0
		}
		links = append(links, LinkResponse{
			Href: fmt.Sprintf("%s?limit=%d&offset=%d", base, limit, prevOffset),
			Rel:  "previous",
		})
	}
	if offset+limit < totalCount {
		links = append(links, LinkResponse{
			Href: fmt.Sprintf("%s?limit=%d&offset=%d", base, limit, offset+limit),
			Rel:  "next",
		})
	}
	return links
}

// buildPaginationLinks builds pagination links for the response.
func buildPaginationLinks(limit, offset, totalCount int) []Link {
	links := make([]Link, 0)

	// Previous link
	if offset > 0 {
		prevOffset := offset - limit
		if prevOffset < 0 {
			prevOffset = 0
		}
		links = append(links, Link{
			Href: fmt.Sprintf("/design/themes?limit=%d&offset=%d", limit, prevOffset),
			Rel:  "previous",
		})
	}

	// Next link
	if offset+limit < totalCount {
		nextOffset := offset + limit
		links = append(links, Link{
			Href: fmt.Sprintf("/design/themes?limit=%d&offset=%d", limit, nextOffset),
			Rel:  "next",
		})
	}

	return links
}
