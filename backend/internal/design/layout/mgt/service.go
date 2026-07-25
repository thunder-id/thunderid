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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const loggerComponentName = "LayoutMgtService"

// LayoutMgtServiceInterface defines the interface for the layout management service.
type LayoutMgtServiceInterface interface {
	GetLayoutList(ctx context.Context, limit, offset int) (*LayoutList, *tidcommon.ServiceError)
	CreateLayout(ctx context.Context, layout CreateLayoutRequestWithID) (*Layout, *tidcommon.ServiceError)
	GetLayout(ctx context.Context, id string) (*Layout, *tidcommon.ServiceError)
	UpdateLayout(ctx context.Context, id string, layout UpdateLayoutRequest) (*Layout, *tidcommon.ServiceError)
	DeleteLayout(ctx context.Context, id string) *tidcommon.ServiceError
	IsLayoutExist(ctx context.Context, id string) (bool, *tidcommon.ServiceError)
	SetDependencyRegistry(r resourcedependency.Registry)
	GetLayoutUsages(ctx context.Context, id string, limit, offset int) (
		*resourcedependency.DependenciesResponse, *tidcommon.ServiceError)
}

// layoutMgtService is the default implementation of the LayoutMgtServiceInterface.
type layoutMgtService struct {
	layoutMgtStore     layoutMgtStoreInterface
	dependencyRegistry resourcedependency.Registry
	logger             *log.Logger
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
func (ls *layoutMgtService) GetLayoutList(
	ctx context.Context, limit, offset int) (*LayoutList, *tidcommon.ServiceError) {
	if err := validatePaginationParams(limit, offset); err != nil {
		return nil, err
	}

	totalCount, err := ls.layoutMgtStore.GetLayoutListCount()
	if err != nil {
		ls.logger.Error(ctx, "Failed to get layout count", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	layouts, err := ls.layoutMgtStore.GetLayoutList(limit, offset)
	if err != nil {
		ls.logger.Error(ctx, "Failed to list layouts", log.Error(err))
		return nil, &tidcommon.InternalServerError
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
func (ls *layoutMgtService) CreateLayout(
	ctx context.Context, layout CreateLayoutRequestWithID) (*Layout, *tidcommon.ServiceError) {
	ls.logger.Debug(ctx, "Creating layout configuration")

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
		ls.logger.Error(ctx, "Failed to check layout handle conflict", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if conflict {
		return nil, &ErrorDuplicateLayoutHandle
	}

	if err := ls.validateLayoutPreferences(ctx, layout.Layout); err != nil {
		return nil, err
	}

	id := layout.ID
	if id == "" {
		var err error
		id, err = utils.GenerateUUIDv7()
		if err != nil {
			ls.logger.Error(ctx, "Failed to generate UUID", log.Error(err))
			return nil, &tidcommon.InternalServerError
		}
	}

	storeReq := CreateLayoutRequest{
		Handle:      layout.Handle,
		DisplayName: layout.DisplayName,
		Description: layout.Description,
		Layout:      layout.Layout,
	}

	if err := ls.layoutMgtStore.CreateLayout(id, storeReq); err != nil {
		ls.logger.Error(ctx, "Failed to create layout", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	createdLayout := &Layout{
		ID:          id,
		Handle:      layout.Handle,
		DisplayName: layout.DisplayName,
		Description: layout.Description,
		Layout:      layout.Layout,
	}

	ls.logger.Debug(ctx, "Successfully created layout", log.String("id", id))
	return createdLayout, nil
}

// GetLayout retrieves a specific layout configuration by its id.
func (ls *layoutMgtService) GetLayout(ctx context.Context, id string) (*Layout, *tidcommon.ServiceError) {
	ls.logger.Debug(ctx, "Retrieving layout", log.String("id", id))

	if id == "" {
		return nil, &ErrorInvalidLayoutID
	}

	layout, err := ls.layoutMgtStore.GetLayout(id)
	if err != nil {
		if errors.Is(err, errLayoutNotFound) {
			ls.logger.Debug(ctx, "Layout not found", log.String("id", id))
			return nil, &ErrorLayoutNotFound
		}
		ls.logger.Error(ctx, "Failed to retrieve layout", log.String("id", id), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	ls.logger.Debug(ctx, "Successfully retrieved layout", log.String("id", layout.ID))
	return &layout, nil
}

// UpdateLayout updates an existing layout configuration.
func (ls *layoutMgtService) UpdateLayout(ctx context.Context,
	id string, layout UpdateLayoutRequest) (*Layout, *tidcommon.ServiceError) {
	ls.logger.Debug(ctx, "Updating layout", log.String("id", id))

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
		ls.logger.Error(ctx, "Failed to retrieve layout", log.String("id", id), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	// Handle is immutable; reject if a different value is provided
	if layout.Handle != "" && layout.Handle != existingLayout.Handle {
		return nil, &ErrorLayoutHandleImmutable
	}

	if err := ls.validateLayoutPreferences(ctx, layout.Layout); err != nil {
		return nil, err
	}

	if err := ls.layoutMgtStore.UpdateLayout(id, layout); err != nil {
		ls.logger.Error(ctx, "Failed to update layout", log.String("id", id), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	updatedLayout := &Layout{
		ID:          id,
		Handle:      existingLayout.Handle,
		DisplayName: layout.DisplayName,
		Description: layout.Description,
		Layout:      layout.Layout,
	}

	ls.logger.Debug(ctx, "Successfully updated layout", log.String("id", id))
	return updatedLayout, nil
}

// DeleteLayout deletes a layout configuration.
func (ls *layoutMgtService) DeleteLayout(ctx context.Context, id string) *tidcommon.ServiceError {
	ls.logger.Debug(ctx, "Deleting layout", log.String("id", id))

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
		ls.logger.Error(ctx, "Failed to check layout existence", log.String("id", id), log.Error(err))
		return &tidcommon.InternalServerError
	}

	if !exists {
		ls.logger.Debug(ctx, "Layout not found for deletion, returning success", log.String("id", id))
		return nil
	}

	// A layout can be deleted even while applications reference it: those applications keep their
	// reference and fall back to the system default layout at read time (see the design resolve
	// service). References are surfaced informationally through GetLayoutUsages.
	if err := ls.layoutMgtStore.DeleteLayout(id); err != nil {
		ls.logger.Error(ctx, "Failed to delete layout", log.String("id", id), log.Error(err))
		return &tidcommon.InternalServerError
	}

	ls.logger.Debug(ctx, "Successfully deleted layout", log.String("id", id))
	return nil
}

// IsLayoutExist checks if a layout exists.
func (ls *layoutMgtService) IsLayoutExist(ctx context.Context, id string) (bool, *tidcommon.ServiceError) {
	if id == "" {
		return false, &ErrorInvalidLayoutID
	}

	exists, err := ls.layoutMgtStore.IsLayoutExist(id)
	if err != nil {
		ls.logger.Error(ctx, "Failed to check layout existence", log.String("id", id), log.Error(err))
		return false, &tidcommon.InternalServerError
	}

	return exists, nil
}

// SetDependencyRegistry injects the dependency registry. Called by servicemanager after the
// provider services are initialized to avoid a cyclic import.
func (ls *layoutMgtService) SetDependencyRegistry(r resourcedependency.Registry) {
	ls.dependencyRegistry = r
}

// GetLayoutUsages returns the resources that reference this layout.
func (ls *layoutMgtService) GetLayoutUsages(
	ctx context.Context, id string, limit, offset int,
) (*resourcedependency.DependenciesResponse, *tidcommon.ServiceError) {
	if id == "" {
		return nil, &ErrorInvalidLayoutID
	}

	if err := validatePaginationParams(limit, offset); err != nil {
		return nil, err
	}

	exists, err := ls.layoutMgtStore.IsLayoutExist(id)
	if err != nil {
		ls.logger.Error(ctx, "Failed to check layout existence", log.String("id", id), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if !exists {
		return nil, &ErrorLayoutNotFound
	}

	if ls.dependencyRegistry == nil {
		ls.logger.Warn(ctx, "Dependency registry not set; returning unknown dependencies", log.String("id", id))
		return &resourcedependency.DependenciesResponse{
			TotalResults: nil,
			Count:        0,
			Summary:      nil,
			Usages:       []resourcedependency.ResourceDependency{},
		}, nil
	}

	result, err := ls.dependencyRegistry.GetDependencies(ctx, resourcedependency.ResourceTypeLayout, id)
	if err != nil {
		ls.logger.Error(ctx, "Failed to get layout usages", log.String("id", id), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	return resourcedependency.PaginateUsages(result, limit, offset), nil
}

// validateLayoutPreferences validates the layout JSON.
func (ls *layoutMgtService) validateLayoutPreferences(
	ctx context.Context, layout json.RawMessage) *tidcommon.ServiceError {
	if len(layout) == 0 {
		return &ErrorMissingLayout
	}

	var result map[string]interface{}
	if err := json.Unmarshal(layout, &result); err != nil {
		ls.logger.Debug(ctx, "Invalid layout JSON", log.Error(err))
		return &ErrorInvalidLayoutFormat
	}

	return nil
}

// validatePaginationParams validates limit and offset parameters.
func validatePaginationParams(limit, offset int) *tidcommon.ServiceError {
	if limit < 1 || limit > serverconst.MaxPageSize {
		return tidcommon.CustomServiceError(ErrorInvalidLimitValue, tidcommon.I18nMessage{
			Key:          "error.layoutservice.invalid_limit_value_description",
			DefaultValue: "Limit must be between 1 and {{param(max)}}",
			Params:       map[string]string{"max": strconv.Itoa(serverconst.MaxPageSize)},
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
