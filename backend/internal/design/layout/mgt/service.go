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

// Package layoutmgt provides layout management functionality.
package layoutmgt

import (
	"encoding/json"
	"errors"
	"fmt"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const loggerComponentName = "LayoutMgtService"

// LayoutMgtServiceInterface defines the interface for the layout management service.
type LayoutMgtServiceInterface interface {
	GetLayoutList(limit, offset int) (*LayoutList, *serviceerror.ServiceError)
	CreateLayout(layout CreateLayoutRequest) (*Layout, *serviceerror.ServiceError)
	GetLayout(id string) (*Layout, *serviceerror.ServiceError)
	UpdateLayout(id string, layout UpdateLayoutRequest) (*Layout, *serviceerror.ServiceError)
	DeleteLayout(id string) *serviceerror.ServiceError
	IsLayoutExist(id string) (bool, *serviceerror.ServiceError)
}

// layoutMgtService is the default implementation of the LayoutMgtServiceInterface.
type layoutMgtService struct {
	layoutMgtStore layoutMgtStoreInterface
	logger         *log.Logger
}

// newLayoutMgtService creates a new instance of LayoutMgtService with injected dependencies.
func newLayoutMgtService(layoutMgtStore layoutMgtStoreInterface) LayoutMgtServiceInterface {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	return &layoutMgtService{
		layoutMgtStore: layoutMgtStore,
		logger:         logger,
	}
}

// GetLayoutList retrieves a list of layout configurations.
func (ls *layoutMgtService) GetLayoutList(limit, offset int) (*LayoutList, *serviceerror.ServiceError) {
	if err := validatePaginationParams(limit, offset); err != nil {
		return nil, err
	}

	totalCount, err := ls.layoutMgtStore.GetLayoutListCount()
	if err != nil {
		ls.logger.Error("Failed to get layout count", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	layouts, err := ls.layoutMgtStore.GetLayoutList(limit, offset)
	if err != nil {
		ls.logger.Error("Failed to list layouts", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	response := &LayoutList{
		TotalResults: totalCount,
		Layouts:      layouts,
		StartIndex:   offset + 1,
		Count:        len(layouts),
		Links:        buildPaginationLinks(limit, offset, totalCount),
	}

	return response, nil
}

// CreateLayout creates a new layout configuration.
func (ls *layoutMgtService) CreateLayout(layout CreateLayoutRequest) (*Layout, *serviceerror.ServiceError) {
	ls.logger.Debug("Creating layout configuration")

	if layout.DisplayName == "" {
		return nil, &ErrorMissingDisplayName
	}

	if layout.Handle == "" {
		return nil, &ErrorMissingLayoutHandle
	}

	// Check if store is in pure declarative mode
	if isDeclarativeModeEnabled() {
		return nil, &ErrorCannotModifyDeclarativeResource
	}

	conflict, err := ls.layoutMgtStore.IsLayoutHandleConflict(layout.Handle, "")
	if err != nil {
		ls.logger.Error("Failed to check layout handle conflict", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}
	if conflict {
		return nil, &ErrorDuplicateLayoutHandle
	}

	if err := ls.validateLayoutPreferences(layout.Layout); err != nil {
		return nil, err
	}

	id, err := utils.GenerateUUIDv7()
	if err != nil {
		ls.logger.Error("Failed to generate UUID", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	if err := ls.layoutMgtStore.CreateLayout(id, layout); err != nil {
		ls.logger.Error("Failed to create layout", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	createdLayout := &Layout{
		ID:          id,
		Handle:      layout.Handle,
		DisplayName: layout.DisplayName,
		Description: layout.Description,
		Layout:      layout.Layout,
	}

	ls.logger.Debug("Successfully created layout", log.String("id", id))
	return createdLayout, nil
}

// GetLayout retrieves a specific layout configuration by its id.
func (ls *layoutMgtService) GetLayout(id string) (*Layout, *serviceerror.ServiceError) {
	ls.logger.Debug("Retrieving layout", log.String("id", id))

	if id == "" {
		return nil, &ErrorInvalidLayoutID
	}

	layout, err := ls.layoutMgtStore.GetLayout(id)
	if err != nil {
		if errors.Is(err, errLayoutNotFound) {
			ls.logger.Debug("Layout not found", log.String("id", id))
			return nil, &ErrorLayoutNotFound
		}
		ls.logger.Error("Failed to retrieve layout", log.String("id", id), log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	ls.logger.Debug("Successfully retrieved layout", log.String("id", layout.ID))
	return &layout, nil
}

// UpdateLayout updates an existing layout configuration.
func (ls *layoutMgtService) UpdateLayout(
	id string, layout UpdateLayoutRequest) (*Layout, *serviceerror.ServiceError) {
	ls.logger.Debug("Updating layout", log.String("id", id))

	if id == "" {
		return nil, &ErrorInvalidLayoutID
	}

	if layout.DisplayName == "" {
		return nil, &ErrorMissingDisplayName
	}

	// Check if the layout is declarative (read-only)
	if ls.layoutMgtStore.IsLayoutDeclarative(id) {
		return nil, &ErrorCannotModifyDeclarativeResource
	}

	// Fetch existing layout to enforce handle immutability
	existingLayout, err := ls.layoutMgtStore.GetLayout(id)
	if err != nil {
		if errors.Is(err, errLayoutNotFound) {
			return nil, &ErrorLayoutNotFound
		}
		ls.logger.Error("Failed to retrieve layout", log.String("id", id), log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	// Handle is immutable; reject if a different value is provided
	if layout.Handle != "" && layout.Handle != existingLayout.Handle {
		return nil, &ErrorLayoutHandleImmutable
	}

	if err := ls.validateLayoutPreferences(layout.Layout); err != nil {
		return nil, err
	}

	if err := ls.layoutMgtStore.UpdateLayout(id, layout); err != nil {
		ls.logger.Error("Failed to update layout", log.String("id", id), log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	updatedLayout := &Layout{
		ID:          id,
		Handle:      existingLayout.Handle,
		DisplayName: layout.DisplayName,
		Description: layout.Description,
		Layout:      layout.Layout,
	}

	ls.logger.Debug("Successfully updated layout", log.String("id", id))
	return updatedLayout, nil
}

// DeleteLayout deletes a layout configuration.
func (ls *layoutMgtService) DeleteLayout(id string) *serviceerror.ServiceError {
	ls.logger.Debug("Deleting layout", log.String("id", id))

	if id == "" {
		return &ErrorInvalidLayoutID
	}

	// Check if the layout is declarative (read-only)
	if ls.layoutMgtStore.IsLayoutDeclarative(id) {
		return &ErrorCannotModifyDeclarativeResource
	}

	// Check if layout exists. Return success for non-existing layouts (idempotent delete).
	exists, err := ls.layoutMgtStore.IsLayoutExist(id)
	if err != nil {
		ls.logger.Error("Failed to check layout existence", log.String("id", id), log.Error(err))
		return &serviceerror.InternalServerError
	}

	if !exists {
		ls.logger.Debug("Layout not found for deletion, returning success", log.String("id", id))
		return nil
	}

	// Check if layout is used by any applications
	count, err := ls.layoutMgtStore.GetApplicationsCountByLayoutID(id)
	if err != nil {
		ls.logger.Error("Failed to check applications using layout", log.String("id", id), log.Error(err))
		return &serviceerror.InternalServerError
	}

	if count > 0 {
		return serviceerror.CustomServiceError(ErrorLayoutInUse, core.I18nMessage{
			Key:          "error.layoutservice.layout_in_use_description",
			DefaultValue: fmt.Sprintf("Layout is being used by %d application(s)", count),
		})
	}

	if err := ls.layoutMgtStore.DeleteLayout(id); err != nil {
		ls.logger.Error("Failed to delete layout", log.String("id", id), log.Error(err))
		return &serviceerror.InternalServerError
	}

	ls.logger.Debug("Successfully deleted layout", log.String("id", id))
	return nil
}

// IsLayoutExist checks if a layout exists.
func (ls *layoutMgtService) IsLayoutExist(id string) (bool, *serviceerror.ServiceError) {
	if id == "" {
		return false, &ErrorInvalidLayoutID
	}

	exists, err := ls.layoutMgtStore.IsLayoutExist(id)
	if err != nil {
		ls.logger.Error("Failed to check layout existence", log.String("id", id), log.Error(err))
		return false, &serviceerror.InternalServerError
	}

	return exists, nil
}

// validateLayoutPreferences validates the layout JSON.
func (ls *layoutMgtService) validateLayoutPreferences(layout json.RawMessage) *serviceerror.ServiceError {
	if len(layout) == 0 {
		return &ErrorMissingLayout
	}

	var result map[string]interface{}
	if err := json.Unmarshal(layout, &result); err != nil {
		ls.logger.Debug("Invalid layout JSON", log.Error(err))
		return &ErrorInvalidLayoutFormat
	}

	return nil
}

// validatePaginationParams validates limit and offset parameters.
func validatePaginationParams(limit, offset int) *serviceerror.ServiceError {
	if limit < 1 || limit > serverconst.MaxPageSize {
		return serviceerror.CustomServiceError(ErrorInvalidLimitValue, core.I18nMessage{
			Key:          "error.layoutservice.invalid_limit_value_description",
			DefaultValue: fmt.Sprintf("Limit must be between 1 and %d", serverconst.MaxPageSize),
		})
	}

	if offset < 0 {
		return &ErrorInvalidOffsetValue
	}

	return nil
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
			Href: fmt.Sprintf("/design/layouts?limit=%d&offset=%d", limit, prevOffset),
			Rel:  "previous",
		})
	}

	// Next link
	if offset+limit < totalCount {
		nextOffset := offset + limit
		links = append(links, Link{
			Href: fmt.Sprintf("/design/layouts?limit=%d&offset=%d", limit, nextOffset),
			Rel:  "next",
		})
	}

	return links
}
