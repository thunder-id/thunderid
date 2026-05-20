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

// Package ou handles the organization unit management operations.
package ou

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/filter"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const loggerComponentNameService = "OrganizationUnitService"

// OrganizationUnitServiceInterface defines the interface for organization unit service operations.
type OrganizationUnitServiceInterface interface {
	GetOrganizationUnitList(
		ctx context.Context, limit, offset int, f *filter.FilterGroup,
	) (*OrganizationUnitListResponse, *serviceerror.ServiceError)
	CreateOrganizationUnit(
		ctx context.Context, request OrganizationUnitRequestWithID,
	) (OrganizationUnit, *serviceerror.ServiceError)
	GetOrganizationUnit(ctx context.Context, id string) (OrganizationUnit, *serviceerror.ServiceError)
	GetOrganizationUnitByPath(ctx context.Context, handlePath string) (OrganizationUnit, *serviceerror.ServiceError)
	IsOrganizationUnitExists(ctx context.Context, id string) (bool, *serviceerror.ServiceError)
	IsOrganizationUnitDeclarative(ctx context.Context, id string) bool
	IsParent(ctx context.Context, parentID, childID string) (bool, *serviceerror.ServiceError)
	UpdateOrganizationUnit(
		ctx context.Context, id string, request OrganizationUnitRequestWithID,
	) (OrganizationUnit, *serviceerror.ServiceError)
	UpdateOrganizationUnitByPath(
		ctx context.Context, handlePath string, request OrganizationUnitRequestWithID,
	) (OrganizationUnit, *serviceerror.ServiceError)
	DeleteOrganizationUnit(ctx context.Context, id string) *serviceerror.ServiceError
	DeleteOrganizationUnitByPath(ctx context.Context, handlePath string) *serviceerror.ServiceError
	GetOrganizationUnitChildren(
		ctx context.Context, id string, limit, offset int, f *filter.FilterGroup,
	) (*OrganizationUnitListResponse, *serviceerror.ServiceError)
	GetOrganizationUnitChildrenByPath(
		ctx context.Context, handlePath string, limit, offset int, f *filter.FilterGroup,
	) (*OrganizationUnitListResponse, *serviceerror.ServiceError)
	GetOrganizationUnitUsers(
		ctx context.Context, id string, limit, offset int, includeDisplay bool,
	) (*UserListResponse, *serviceerror.ServiceError)
	GetOrganizationUnitUsersByPath(
		ctx context.Context, handlePath string, limit, offset int, includeDisplay bool,
	) (*UserListResponse, *serviceerror.ServiceError)
	GetOrganizationUnitGroups(
		ctx context.Context, id string, limit, offset int,
	) (*GroupListResponse, *serviceerror.ServiceError)
	GetOrganizationUnitGroupsByPath(
		ctx context.Context, handlePath string, limit, offset int,
	) (*GroupListResponse, *serviceerror.ServiceError)
	GetOrganizationUnitHandlesByIDs(
		ctx context.Context, ids []string,
	) (map[string]string, *serviceerror.ServiceError)
}

// ConfigurableOUService extends OrganizationUnitServiceInterface with methods for
// two-phase initialization of resolvers. This is intentionally separate from the
// main interface so consumers don't see bootstrap-only methods.
type ConfigurableOUService interface {
	OrganizationUnitServiceInterface
	SetOUUserResolver(resolver OUUserResolver)
	SetOUGroupResolver(resolver OUGroupResolver)
}

// OrganizationUnitService provides organization unit management operations.
type organizationUnitService struct {
	authzService  sysauthz.SystemAuthorizationServiceInterface
	ouStore       organizationUnitStoreInterface
	transactioner transaction.Transactioner
	userResolver  OUUserResolver
	groupResolver OUGroupResolver
}

func (ous *organizationUnitService) SetOUUserResolver(resolver OUUserResolver) {
	ous.userResolver = resolver
}

func (ous *organizationUnitService) SetOUGroupResolver(resolver OUGroupResolver) {
	ous.groupResolver = resolver
}

// newOrganizationUnitService creates a new instance of OrganizationUnitService.
func newOrganizationUnitService(
	authzService sysauthz.SystemAuthorizationServiceInterface,
	ouStore organizationUnitStoreInterface,
	transactioner transaction.Transactioner,
) ConfigurableOUService {
	return &organizationUnitService{
		authzService:  authzService,
		ouStore:       ouStore,
		transactioner: transactioner,
	}
}

// GetOrganizationUnitList retrieves a list of organization units.
// limit should be a positive integer and offset should be non-negative.
func (ous *organizationUnitService) GetOrganizationUnitList(
	ctx context.Context, limit, offset int, f *filter.FilterGroup,
) (
	*OrganizationUnitListResponse, *serviceerror.ServiceError,
) {
	if err := validatePaginationParams(limit, offset); err != nil {
		return nil, err
	}

	if f != nil {
		for _, clause := range f.Clauses {
			if _, ok := ouFilterableColumns[clause.Expr.Attribute]; !ok {
				return nil, &ErrorInvalidFilter
			}
		}
	}

	// Resolve the set of organization units the caller is authorized to see.
	accessible, svcErr := ous.authzService.GetAccessibleResources(
		ctx, security.ActionListOUs, security.ResourceTypeOU)
	if svcErr != nil {
		return nil, &serviceerror.InternalServerError
	}

	// Unfiltered path: the caller can see all organization units.
	if accessible.AllAllowed {
		return ous.listAllOrganizationUnits(ctx, limit, offset, f)
	}

	// Filtered path: the caller has a restricted set of accessible organization units.
	return ous.listAccessibleOrganizationUnits(ctx, accessible.IDs, limit, offset, f)
}

// listAllOrganizationUnits retrieves organization units without authorization filtering.
func (ous *organizationUnitService) listAllOrganizationUnits(
	ctx context.Context, limit, offset int, f *filter.FilterGroup,
) (*OrganizationUnitListResponse, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameService))
	totalCount, err := ous.ouStore.GetOrganizationUnitListCount(ctx, f)
	if err != nil {
		logger.Error("Failed to get organization unit count", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	ouList, err := ous.ouStore.GetOrganizationUnitList(ctx, limit, offset, f)
	if err != nil {
		// Check if it's a limit exceeded error
		if errors.Is(err, ErrResultLimitExceededInCompositeMode) {
			return nil, &ErrorResultLimitExceeded
		}
		logger.Error("Failed to list organization units", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	return &OrganizationUnitListResponse{
		TotalResults:      totalCount,
		OrganizationUnits: ouList,
		StartIndex:        offset + 1,
		Count:             len(ouList),
		Links:             utils.BuildPaginationLinks("/organization-units", limit, offset, totalCount, ""),
	}, nil
}

// listAccessibleOrganizationUnits retrieves only the organization units the caller is authorized to access.
// When g is nil it paginates the ID slice first and fetches only the needed page (efficient path).
// When g is non-nil it fetches all authorized OUs, applies the filter in memory, then paginates.
func (ous *organizationUnitService) listAccessibleOrganizationUnits(
	ctx context.Context, ids []string, limit, offset int, g *filter.FilterGroup,
) (*OrganizationUnitListResponse, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameService))

	if len(ids) == 0 {
		return &OrganizationUnitListResponse{
			TotalResults:      0,
			OrganizationUnits: []OrganizationUnitBasic{},
			StartIndex:        1,
			Count:             0,
			Links:             utils.BuildPaginationLinks("/organization-units", limit, offset, 0, ""),
		}, nil
	}

	if g != nil {
		// Fetch all authorized OUs then apply the filter in memory so TotalResults
		// reflects the filtered count, not the raw authorized-ID count.
		allOUs, err := ous.ouStore.GetOrganizationUnitsByIDs(ctx, ids)
		if err != nil {
			logger.Error("Failed to get organization units by IDs", log.Error(err))
			return nil, &serviceerror.InternalServerError
		}

		filtered := make([]OrganizationUnitBasic, 0, len(allOUs))
		for _, ou := range allOUs {
			if matchesOUBasicFilter(ou, g) {
				filtered = append(filtered, ou)
			}
		}

		total := len(filtered)
		start := offset
		if start > total {
			start = total
		}
		end := start + limit
		if end > total {
			end = total
		}
		page := filtered[start:end]

		return &OrganizationUnitListResponse{
			TotalResults:      total,
			OrganizationUnits: page,
			StartIndex:        offset + 1,
			Count:             len(page),
			Links:             utils.BuildPaginationLinks("/organization-units", limit, offset, total, ""),
		}, nil
	}

	// No filter: paginate the ID list before hitting the store (efficient path).
	total := len(ids)
	start := offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}
	pageIDs := ids[start:end]

	if len(pageIDs) == 0 {
		return &OrganizationUnitListResponse{
			TotalResults:      total,
			OrganizationUnits: []OrganizationUnitBasic{},
			StartIndex:        offset + 1,
			Count:             0,
			Links:             utils.BuildPaginationLinks("/organization-units", limit, offset, total, ""),
		}, nil
	}

	pageOUs, err := ous.ouStore.GetOrganizationUnitsByIDs(ctx, pageIDs)
	if err != nil {
		logger.Error("Failed to get organization units by IDs", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	return &OrganizationUnitListResponse{
		TotalResults:      total,
		OrganizationUnits: pageOUs,
		StartIndex:        offset + 1,
		Count:             len(pageOUs),
		Links:             utils.BuildPaginationLinks("/organization-units", limit, offset, total, ""),
	}, nil
}

// CreateOrganizationUnit creates a new organization unit.
func (ous *organizationUnitService) CreateOrganizationUnit(
	ctx context.Context, request OrganizationUnitRequestWithID,
) (OrganizationUnit, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameService))
	logger.Debug("Creating organization unit", log.String("name", request.Name))

	// Fail if store is in declarative mode
	if isDeclarativeModeEnabled() {
		return OrganizationUnit{}, &ErrorCannotModifyDeclarativeResource
	}

	var createdOU OrganizationUnit
	var capturedSvcErr *serviceerror.ServiceError

	err := ous.transactioner.Transact(ctx, func(txCtx context.Context) error {
		if svcErr := ous.validateOUName(request.Name); svcErr != nil {
			capturedSvcErr = svcErr
			return errors.New("validation error")
		}

		if svcErr := ous.validateOUHandle(request.Handle); svcErr != nil {
			capturedSvcErr = svcErr
			return errors.New("validation error")
		}

		if request.Parent != nil {
			if svcErr := ous.checkOUAccess(txCtx, security.ActionCreateOU, *request.Parent); svcErr != nil {
				capturedSvcErr = svcErr
				return errors.New("authz error")
			}
			exists, err := ous.ouStore.IsOrganizationUnitExists(txCtx, *request.Parent)
			if err != nil {
				return err
			}
			if !exists {
				capturedSvcErr = &ErrorParentOrganizationUnitNotFound
				return errors.New("parent not found")
			}
		} else {
			if svcErr := ous.checkOUAccess(txCtx, security.ActionCreateOU, ""); svcErr != nil {
				capturedSvcErr = svcErr
				return errors.New("authz error")
			}
		}

		conflict, err := ous.ouStore.CheckOrganizationUnitNameConflict(txCtx, request.Name, request.Parent)
		if err != nil {
			return err
		}
		if conflict {
			capturedSvcErr = &ErrorOrganizationUnitNameConflict
			return errors.New("conflict")
		}

		handleConflict, err := ous.ouStore.CheckOrganizationUnitHandleConflict(txCtx, request.Handle, request.Parent)
		if err != nil {
			return err
		}
		if handleConflict {
			capturedSvcErr = &ErrorOrganizationUnitHandleConflict
			return errors.New("conflict")
		}

		ouID := request.ID
		if request.ID == "" {
			ouID, err = utils.GenerateUUIDv7()
			if err != nil {
				return err
			}
		}

		now := time.Now().UTC()
		createdOU = OrganizationUnit{
			ID:              ouID,
			Handle:          request.Handle,
			Name:            request.Name,
			Description:     request.Description,
			Parent:          request.Parent,
			ThemeID:         request.ThemeID,
			LayoutID:        request.LayoutID,
			LogoURL:         request.LogoURL,
			TosURI:          request.TosURI,
			PolicyURI:       request.PolicyURI,
			CookiePolicyURI: request.CookiePolicyURI,
			CreatedAt:       now,
			UpdatedAt:       now,
		}

		err = ous.ouStore.CreateOrganizationUnit(txCtx, createdOU)
		return err
	})

	if capturedSvcErr != nil {
		return OrganizationUnit{}, capturedSvcErr
	}
	if err != nil {
		logger.Error("Failed to create organization unit", log.Error(err), log.String("name", request.Name))
		return OrganizationUnit{}, &serviceerror.InternalServerError
	}

	logger.Debug("Successfully created organization unit", log.String("ouID", createdOU.ID))

	return createdOU, nil
}

// GetOrganizationUnit retrieves an organization unit by ID.
func (ous *organizationUnitService) GetOrganizationUnit(
	ctx context.Context, id string,
) (OrganizationUnit, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameService))
	logger.Debug("Getting organization unit", log.String("ouID", id))

	if svcErr := ous.checkOUAccess(ctx, security.ActionReadOU, id); svcErr != nil {
		return OrganizationUnit{}, svcErr
	}

	ou, err := ous.ouStore.GetOrganizationUnit(ctx, id)
	if err != nil {
		if errors.Is(err, ErrOrganizationUnitNotFound) {
			return OrganizationUnit{}, &ErrorOrganizationUnitNotFound
		}
		logger.Error("Failed to get organization unit", log.Error(err))
		return OrganizationUnit{}, &serviceerror.InternalServerError
	}

	return ou, nil
}

// GetOrganizationUnitByPath retrieves an organization unit by hierarchical handle path.
func (ous *organizationUnitService) GetOrganizationUnitByPath(
	ctx context.Context, handlePath string,
) (OrganizationUnit, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameService))
	logger.Debug("Getting organization unit by path", log.String("path", handlePath))

	handles, serviceError := validateAndProcessHandlePath(handlePath)
	if serviceError != nil {
		return OrganizationUnit{}, serviceError
	}

	ou, err := ous.ouStore.GetOrganizationUnitByPath(ctx, handles)
	if err != nil {
		if errors.Is(err, ErrOrganizationUnitNotFound) {
			return OrganizationUnit{}, &ErrorOrganizationUnitNotFound
		}
		logger.Error("Failed to get organization unit by path", log.Error(err))
		return OrganizationUnit{}, &serviceerror.InternalServerError
	}

	if svcErr := ous.checkOUAccess(ctx, security.ActionReadOU, ou.ID); svcErr != nil {
		return OrganizationUnit{}, svcErr
	}

	return ou, nil
}

// IsOrganizationUnitExists checks if an organization unit exists by ID.
func (ous *organizationUnitService) IsOrganizationUnitExists(
	ctx context.Context, id string,
) (bool, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameService))
	logger.Debug("Checking if organization unit exists", log.String("ouID", id))

	exists, err := ous.ouStore.IsOrganizationUnitExists(ctx, id)
	if err != nil {
		logger.Error("Failed to check organization unit existence", log.Error(err))
		return false, &serviceerror.InternalServerError
	}

	return exists, nil
}

func (ous *organizationUnitService) IsOrganizationUnitDeclarative(ctx context.Context, id string) bool {
	return ous.ouStore.IsOrganizationUnitDeclarative(ctx, id)
}

// IsParent checks whether the provided parentID is an ancestor of childID.
// Returns true if the parent and child are the same or if parentID is an ancestor of childID.
func (ous *organizationUnitService) IsParent(
	ctx context.Context, parentID, childID string,
) (bool, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameService))

	if strings.TrimSpace(parentID) == "" || strings.TrimSpace(childID) == "" {
		return false, &ErrorInvalidRequestFormat
	}

	currentParent := &childID
	for currentParent != nil {
		if *currentParent == parentID {
			return true, nil
		}

		parentOU, err := ous.ouStore.GetOrganizationUnit(ctx, *currentParent)
		if err != nil {
			if errors.Is(err, ErrOrganizationUnitNotFound) {
				logger.Debug("Encountered missing organization unit in hierarchy", log.String("ouID", *currentParent))
				return false, &ErrorOrganizationUnitNotFound
			}
			logger.Error("Failed to traverse organization unit hierarchy", log.Error(err))
			return false, &serviceerror.InternalServerError
		}

		currentParent = parentOU.Parent
	}

	return false, nil
}

// UpdateOrganizationUnit updates an organization unit.
func (ous *organizationUnitService) UpdateOrganizationUnit(
	ctx context.Context, id string, request OrganizationUnitRequestWithID,
) (OrganizationUnit, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameService))
	logger.Debug("Updating organization unit", log.String("ouID", id))

	if svcErr := ous.checkOUAccess(ctx, security.ActionUpdateOU, id); svcErr != nil {
		return OrganizationUnit{}, svcErr
	}

	var updatedOU OrganizationUnit
	var capturedSvcErr *serviceerror.ServiceError

	err := ous.transactioner.Transact(ctx, func(txCtx context.Context) error {
		existingOU, err := ous.ouStore.GetOrganizationUnit(txCtx, id)
		if err != nil {
			if errors.Is(err, ErrOrganizationUnitNotFound) {
				capturedSvcErr = &ErrorOrganizationUnitNotFound
				return err
			}
			return err
		}

		var svcErr *serviceerror.ServiceError
		updatedOU, svcErr = ous.updateOUInternal(txCtx, id, request, existingOU, logger)
		if svcErr != nil {
			capturedSvcErr = svcErr
			return errors.New("update error")
		}
		return nil
	})

	if capturedSvcErr != nil {
		return OrganizationUnit{}, capturedSvcErr
	}
	if err != nil {
		logger.Error("Failed to update organization unit", log.Error(err), log.String("ouID", id))
		return OrganizationUnit{}, &serviceerror.InternalServerError
	}

	logger.Debug("Successfully updated organization unit", log.String("ouID", id))
	return updatedOU, nil
}

// UpdateOrganizationUnitByPath updates an organization unit by hierarchical handle path.
func (ous *organizationUnitService) UpdateOrganizationUnitByPath(
	ctx context.Context, handlePath string, request OrganizationUnitRequestWithID,
) (OrganizationUnit, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameService))
	logger.Debug("Updating organization unit by path", log.String("path", handlePath))

	handles, serviceError := validateAndProcessHandlePath(handlePath)
	if serviceError != nil {
		return OrganizationUnit{}, serviceError
	}

	var updatedOU OrganizationUnit
	var capturedSvcErr *serviceerror.ServiceError

	err := ous.transactioner.Transact(ctx, func(txCtx context.Context) error {
		existingOU, err := ous.ouStore.GetOrganizationUnitByPath(txCtx, handles)
		if err != nil {
			if errors.Is(err, ErrOrganizationUnitNotFound) {
				capturedSvcErr = &ErrorOrganizationUnitNotFound
				return err
			}
			return err
		}

		if svcErr := ous.checkOUAccess(txCtx, security.ActionUpdateOU, existingOU.ID); svcErr != nil {
			capturedSvcErr = svcErr
			return errors.New("authz error")
		}

		// Check if OU is declarative (for composite mode)
		if ous.ouStore.IsOrganizationUnitDeclarative(txCtx, existingOU.ID) {
			capturedSvcErr = &ErrorCannotModifyDeclarativeResource
			return errors.New("declarative resource")
		}

		var svcErr *serviceerror.ServiceError
		updatedOU, svcErr = ous.updateOUInternal(txCtx, existingOU.ID, request, existingOU, logger)
		if svcErr != nil {
			capturedSvcErr = svcErr
			return errors.New("update error")
		}
		return nil
	})

	if capturedSvcErr != nil {
		return OrganizationUnit{}, capturedSvcErr
	}
	if err != nil {
		logger.Error("Failed to update organization unit by path", log.Error(err), log.String("path", handlePath))
		return OrganizationUnit{}, &serviceerror.InternalServerError
	}

	logger.Debug("Successfully updated organization unit by path", log.String("ouID", updatedOU.ID))
	return updatedOU, nil
}

func (ous *organizationUnitService) updateOUInternal(
	ctx context.Context,
	id string,
	request OrganizationUnitRequestWithID,
	existingOU OrganizationUnit,
	logger *log.Logger,
) (OrganizationUnit, *serviceerror.ServiceError) {
	// Check if OU is immutable (for composite mode)
	if ous.ouStore.IsOrganizationUnitDeclarative(ctx, id) {
		return OrganizationUnit{}, &ErrorCannotModifyDeclarativeResource
	}

	if err := ous.validateOUName(request.Name); err != nil {
		return OrganizationUnit{}, err
	}

	if err := ous.validateOUHandle(request.Handle); err != nil {
		return OrganizationUnit{}, err
	}

	if request.Parent != nil {
		exists, err := ous.ouStore.IsOrganizationUnitExists(ctx, *request.Parent)
		if err != nil {
			logger.Error("Failed to check parent organization unit existence", log.Error(err))
			return OrganizationUnit{}, &serviceerror.InternalServerError
		}
		if !exists {
			return OrganizationUnit{}, &ErrorParentOrganizationUnitNotFound
		}
	}

	if err := ous.checkCircularDependency(ctx, id, request.Parent); err != nil {
		return OrganizationUnit{}, err
	}

	parentChanged := !stringPtrEqual(existingOU.Parent, request.Parent)

	var nameConflict bool
	var err error
	if parentChanged || existingOU.Name != request.Name {
		nameConflict, err = ous.ouStore.CheckOrganizationUnitNameConflict(ctx, request.Name, request.Parent)
		if err != nil {
			logger.Error("Failed to check organization unit name conflict", log.Error(err))
			return OrganizationUnit{}, &serviceerror.InternalServerError
		}
	}

	if nameConflict {
		return OrganizationUnit{}, &ErrorOrganizationUnitNameConflict
	}

	var handleConflict bool
	if parentChanged || existingOU.Handle != request.Handle {
		handleConflict, err = ous.ouStore.CheckOrganizationUnitHandleConflict(ctx, request.Handle, request.Parent)
		if err != nil {
			logger.Error("Failed to check organization unit handle conflict", log.Error(err))
			return OrganizationUnit{}, &serviceerror.InternalServerError
		}
	}

	if handleConflict {
		return OrganizationUnit{}, &ErrorOrganizationUnitHandleConflict
	}

	updatedOU := OrganizationUnit{
		ID:              existingOU.ID,
		Handle:          request.Handle,
		Name:            request.Name,
		Description:     request.Description,
		Parent:          request.Parent,
		ThemeID:         request.ThemeID,
		LayoutID:        request.LayoutID,
		LogoURL:         request.LogoURL,
		TosURI:          request.TosURI,
		PolicyURI:       request.PolicyURI,
		CookiePolicyURI: request.CookiePolicyURI,
		CreatedAt:       existingOU.CreatedAt,
		UpdatedAt:       time.Now().UTC(),
	}

	err = ous.ouStore.UpdateOrganizationUnit(ctx, updatedOU)
	if err != nil {
		if errors.Is(err, ErrOrganizationUnitNotFound) {
			return OrganizationUnit{}, &ErrorOrganizationUnitNotFound
		}
		logger.Error("Failed to update organization unit", log.Error(err))
		return OrganizationUnit{}, &serviceerror.InternalServerError
	}
	return updatedOU, nil
}

// DeleteOrganizationUnit deletes an organization unit.
func (ous *organizationUnitService) DeleteOrganizationUnit(
	ctx context.Context, id string) *serviceerror.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameService))
	logger.Debug("Deleting organization unit", log.String("ouID", id))

	if svcErr := ous.checkOUAccess(ctx, security.ActionDeleteOU, id); svcErr != nil {
		return svcErr
	}

	var capturedSvcErr *serviceerror.ServiceError

	err := ous.transactioner.Transact(ctx, func(txCtx context.Context) error {
		// Check if organization unit exists
		exists, err := ous.ouStore.IsOrganizationUnitExists(txCtx, id)
		if err != nil {
			return err
		}
		if !exists {
			capturedSvcErr = &ErrorOrganizationUnitNotFound
			return errors.New("not found")
		}

		svcErr := ous.deleteOUInternal(txCtx, id, logger)
		if svcErr != nil {
			capturedSvcErr = svcErr
			return errors.New("delete error")
		}
		return nil
	})

	if capturedSvcErr != nil {
		return capturedSvcErr
	}
	if err != nil {
		logger.Error("Failed to delete organization unit", log.Error(err), log.String("ouID", id))
		return &serviceerror.InternalServerError
	}

	logger.Debug("Successfully deleted organization unit", log.String("ouID", id))
	return nil
}

// DeleteOrganizationUnitByPath deletes an organization unit by hierarchical handle path.
func (ous *organizationUnitService) DeleteOrganizationUnitByPath(
	ctx context.Context, handlePath string,
) *serviceerror.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameService))
	logger.Debug("Deleting organization unit by path", log.String("path", handlePath))

	handles, serviceError := validateAndProcessHandlePath(handlePath)
	if serviceError != nil {
		return serviceError
	}

	var ouID string
	var capturedSvcErr *serviceerror.ServiceError

	err := ous.transactioner.Transact(ctx, func(txCtx context.Context) error {
		existingOU, err := ous.ouStore.GetOrganizationUnitByPath(txCtx, handles)
		if err != nil {
			if errors.Is(err, ErrOrganizationUnitNotFound) {
				capturedSvcErr = &ErrorOrganizationUnitNotFound
				return err
			}
			return err
		}
		ouID = existingOU.ID

		if svcErr := ous.checkOUAccess(txCtx, security.ActionDeleteOU, ouID); svcErr != nil {
			capturedSvcErr = svcErr
			return errors.New("authz error")
		}

		// Check if OU is declarative (for composite mode)
		if ous.ouStore.IsOrganizationUnitDeclarative(txCtx, ouID) {
			capturedSvcErr = &ErrorCannotModifyDeclarativeResource
			return errors.New("declarative resource")
		}

		svcErr := ous.deleteOUInternal(txCtx, ouID, logger)
		if svcErr != nil {
			capturedSvcErr = svcErr
			return errors.New("delete error")
		}
		return nil
	})

	if capturedSvcErr != nil {
		return capturedSvcErr
	}
	if err != nil {
		logger.Error("Failed to delete organization unit by path", log.Error(err), log.String("path", handlePath))
		return &serviceerror.InternalServerError
	}

	logger.Debug("Successfully deleted organization unit by path", log.String("ouID", ouID))
	return nil
}

// deleteOUInternal deletes an organization unit by ID after checking if it has child resources.
func (ous *organizationUnitService) deleteOUInternal(
	ctx context.Context, id string, logger *log.Logger,
) *serviceerror.ServiceError {
	// Check if OU is immutable (for composite mode)
	if ous.ouStore.IsOrganizationUnitDeclarative(ctx, id) {
		return &ErrorCannotModifyDeclarativeResource
	}

	// Check child OUs (own table).
	childCount, err := ous.ouStore.GetOrganizationUnitChildrenCount(ctx, id, nil)
	if err != nil {
		logger.Error("Failed to check child organization units", log.Error(err))
		return &serviceerror.InternalServerError
	}
	if childCount > 0 {
		return &ErrorCannotDeleteOrganizationUnit
	}

	// Check users via resolver.
	if ous.userResolver == nil {
		logger.Error("OUUserResolver not initialized")
		return &serviceerror.InternalServerError
	}
	userCount, err := ous.userResolver.GetUserCountByOUID(ctx, id)
	if err != nil {
		logger.Error("Failed to check organization unit users", log.Error(err))
		return &serviceerror.InternalServerError
	}
	if userCount > 0 {
		return &ErrorCannotDeleteOrganizationUnit
	}

	// Check groups via resolver.
	if ous.groupResolver == nil {
		logger.Error("OUGroupResolver not initialized")
		return &serviceerror.InternalServerError
	}
	groupCount, err := ous.groupResolver.GetGroupCountByOUID(ctx, id)
	if err != nil {
		logger.Error("Failed to check organization unit groups", log.Error(err))
		return &serviceerror.InternalServerError
	}
	if groupCount > 0 {
		return &ErrorCannotDeleteOrganizationUnit
	}

	err = ous.ouStore.DeleteOrganizationUnit(ctx, id)
	if err != nil {
		if errors.Is(err, ErrOrganizationUnitNotFound) {
			return &ErrorOrganizationUnitNotFound
		}
		logger.Error("Failed to delete organization unit", log.Error(err))
		return &serviceerror.InternalServerError
	}
	return nil
}

// checkOUAccess validates that the caller is authorized to perform the given action on an organization unit.
// Pass an empty ouID when there is no specific resource context (e.g. creating a root-level OU).
func (ous *organizationUnitService) checkOUAccess(
	ctx context.Context, action security.Action, ouID string,
) *serviceerror.ServiceError {
	allowed, svcErr := ous.authzService.IsActionAllowed(ctx, action,
		&sysauthz.ActionContext{ResourceType: security.ResourceTypeOU, OUID: ouID})
	if svcErr != nil {
		return &serviceerror.InternalServerError
	}
	if !allowed {
		return &serviceerror.ErrorUnauthorized
	}
	return nil
}

// GetOrganizationUnitUsers retrieves a list of users for a given organization unit ID.
func (ous *organizationUnitService) GetOrganizationUnitUsers(
	ctx context.Context, id string, limit, offset int, includeDisplay bool,
) (*UserListResponse, *serviceerror.ServiceError) {
	if svcErr := ous.checkOUAccess(ctx, security.ActionReadUser, id); svcErr != nil {
		return nil, svcErr
	}
	if ous.userResolver == nil {
		return nil, &serviceerror.InternalServerError
	}

	items, totalCount, svcErr := ous.getResourceListWithExistenceCheck(
		ctx, id, limit, offset, "users",
		func(ctx context.Context, id string, limit, offset int) (interface{}, error) {
			return ous.userResolver.GetUserListByOUID(ctx, id, limit, offset, includeDisplay)
		},
		ous.userResolver.GetUserCountByOUID,
		false, // No composite error mapping for users
	)
	if svcErr != nil {
		return nil, svcErr
	}

	users, ok := items.([]User)
	if !ok {
		logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameService))
		logger.Error("Failed to cast user list response for organization unit", log.String("ouID", id))
		return nil, &serviceerror.InternalServerError
	}

	base := fmt.Sprintf("/organization-units/%s/users", id)
	return buildUserListResponse(base, users, totalCount, limit, offset, includeDisplay)
}

// GetOrganizationUnitGroups retrieves a list of groups for a given organization unit ID.
func (ous *organizationUnitService) GetOrganizationUnitGroups(
	ctx context.Context, id string, limit, offset int,
) (*GroupListResponse, *serviceerror.ServiceError) {
	if svcErr := ous.checkOUAccess(ctx, security.ActionReadGroup, id); svcErr != nil {
		return nil, svcErr
	}
	if ous.groupResolver == nil {
		return nil, &serviceerror.InternalServerError
	}

	items, totalCount, svcErr := ous.getResourceListWithExistenceCheck(
		ctx, id, limit, offset, "groups",
		func(ctx context.Context, id string, limit, offset int) (interface{}, error) {
			return ous.groupResolver.GetGroupListByOUID(ctx, id, limit, offset)
		},
		ous.groupResolver.GetGroupCountByOUID,
		false, // No composite error mapping for groups
	)
	if svcErr != nil {
		return nil, svcErr
	}
	base := fmt.Sprintf("/organization-units/%s/groups", id)
	return buildGroupListResponse(base, items, totalCount, limit, offset)
}

// GetOrganizationUnitChildren retrieves a list of child organization units for a given organization unit ID.
func (ous *organizationUnitService) GetOrganizationUnitChildren(
	ctx context.Context, id string, limit, offset int, f *filter.FilterGroup,
) (*OrganizationUnitListResponse, *serviceerror.ServiceError) {
	if svcErr := ous.checkOUAccess(ctx, security.ActionListChildOUs, id); svcErr != nil {
		return nil, svcErr
	}

	if f != nil {
		for _, clause := range f.Clauses {
			if _, ok := ouFilterableColumns[clause.Expr.Attribute]; !ok {
				return nil, &ErrorInvalidFilter
			}
		}
	}

	items, totalCount, svcErr := ous.getResourceListWithExistenceCheck(
		ctx, id, limit, offset, "child organization units",
		func(ctx context.Context, id string, limit, offset int) (interface{}, error) {
			return ous.ouStore.GetOrganizationUnitChildrenList(ctx, id, limit, offset, f)
		},
		func(ctx context.Context, id string) (int, error) {
			return ous.ouStore.GetOrganizationUnitChildrenCount(ctx, id, f)
		},
		true,
	)
	if svcErr != nil {
		return nil, svcErr
	}
	base := fmt.Sprintf("/organization-units/%s/ous", id)
	return buildOrganizationUnitListResponse(base, items, totalCount, limit, offset)
}

// GetOrganizationUnitChildrenByPath retrieves a list of child organization units by hierarchical handle path.
func (ous *organizationUnitService) GetOrganizationUnitChildrenByPath(
	ctx context.Context, handlePath string, limit, offset int, f *filter.FilterGroup,
) (*OrganizationUnitListResponse, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameService))
	logger.Debug("Getting organization unit children by path", log.String("path", handlePath))

	handles, serviceError := validateAndProcessHandlePath(handlePath)
	if serviceError != nil {
		return nil, serviceError
	}

	ou, err := ous.ouStore.GetOrganizationUnitByPath(ctx, handles)
	if err != nil {
		if errors.Is(err, ErrOrganizationUnitNotFound) {
			return nil, &ErrorOrganizationUnitNotFound
		}
		logger.Error("Failed to get organization unit by path", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	return ous.GetOrganizationUnitChildren(ctx, ou.ID, limit, offset, f)
}

// GetOrganizationUnitUsersByPath retrieves a list of users by hierarchical handle path.
func (ous *organizationUnitService) GetOrganizationUnitUsersByPath(
	ctx context.Context, handlePath string, limit, offset int, includeDisplay bool,
) (*UserListResponse, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameService))
	logger.Debug("Getting organization unit users by path", log.String("path", handlePath))

	handles, serviceError := validateAndProcessHandlePath(handlePath)
	if serviceError != nil {
		return nil, serviceError
	}

	ou, err := ous.ouStore.GetOrganizationUnitByPath(ctx, handles)
	if err != nil {
		if errors.Is(err, ErrOrganizationUnitNotFound) {
			return nil, &ErrorOrganizationUnitNotFound
		}
		logger.Error("Failed to get organization unit by path", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	return ous.GetOrganizationUnitUsers(ctx, ou.ID, limit, offset, includeDisplay)
}

// GetOrganizationUnitGroupsByPath retrieves a list of groups by hierarchical handle path.
func (ous *organizationUnitService) GetOrganizationUnitGroupsByPath(
	ctx context.Context, handlePath string, limit, offset int,
) (*GroupListResponse, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameService))
	logger.Debug("Getting organization unit groups by path", log.String("path", handlePath))

	handles, serviceError := validateAndProcessHandlePath(handlePath)
	if serviceError != nil {
		return nil, serviceError
	}

	ou, err := ous.ouStore.GetOrganizationUnitByPath(ctx, handles)
	if err != nil {
		if errors.Is(err, ErrOrganizationUnitNotFound) {
			return nil, &ErrorOrganizationUnitNotFound
		}
		logger.Error("Failed to get organization unit by path", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	return ous.GetOrganizationUnitGroups(ctx, ou.ID, limit, offset)
}

// checkCircularDependency checks if setting the parent would create a circular dependency.
func (ous *organizationUnitService) checkCircularDependency(
	ctx context.Context, ouID string, parentID *string,
) *serviceerror.ServiceError {
	if parentID == nil {
		return nil
	}

	if ouID == *parentID {
		return &ErrorCircularDependency
	}

	currentParentID := parentID
	for currentParentID != nil {
		if *currentParentID == ouID {
			return &ErrorCircularDependency
		}

		parentOU, err := ous.ouStore.GetOrganizationUnit(ctx, *currentParentID)
		if err != nil {
			if errors.Is(err, ErrOrganizationUnitNotFound) {
				break
			}
			return &serviceerror.InternalServerError
		}

		currentParentID = parentOU.Parent
	}

	return nil
}

// validateOUName validates organization unit name.
func (ous *organizationUnitService) validateOUName(name string) *serviceerror.ServiceError {
	if strings.TrimSpace(name) == "" {
		return &ErrorInvalidRequestFormat
	}

	return nil
}

// validateOUHandle validates organization unit handle.
func (ous *organizationUnitService) validateOUHandle(handle string) *serviceerror.ServiceError {
	trimmed := strings.TrimSpace(handle)
	if trimmed == "" {
		return &ErrorInvalidRequestFormat
	}

	if strings.Contains(trimmed, "/") {
		return &ErrorInvalidRequestFormat
	}

	return nil
}

func validateAndProcessHandlePath(handlePath string) ([]string, *serviceerror.ServiceError) {
	if strings.TrimSpace(handlePath) == "" {
		return nil, &ErrorInvalidHandlePath
	}

	trimmed := strings.Trim(handlePath, "/")
	if trimmed == "" {
		return nil, &ErrorInvalidHandlePath
	}

	handles := strings.Split(trimmed, "/")
	var validHandles []string
	for _, handle := range handles {
		if strings.TrimSpace(handle) != "" {
			validHandles = append(validHandles, strings.TrimSpace(handle))
		}
	}
	return validHandles, nil
}

// validatePaginationParams validates pagination parameters.
func validatePaginationParams(limit, offset int) *serviceerror.ServiceError {
	if limit < 1 || limit > serverconst.MaxPageSize {
		return &ErrorInvalidLimit
	}
	if offset < 0 {
		return &ErrorInvalidOffset
	}
	return nil
}

// getResourceListWithExistenceCheck is a generic function to get resources for an
// organization unit with existence check.
// If mapCompositeError is true, it will map ErrResultLimitExceededInCompositeMode to ErrorResultLimitExceeded.
func (ous *organizationUnitService) getResourceListWithExistenceCheck(
	ctx context.Context, id string, limit, offset int, resourceType string,
	getListFunc func(context.Context, string, int, int) (interface{}, error),
	getCountFunc func(context.Context, string) (int, error),
	mapCompositeError bool,
) (interface{}, int, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameService))
	logger.Debug("Getting resource for organization unit", log.String("resource_type", resourceType),
		log.String("ouID", id))

	if err := validatePaginationParams(limit, offset); err != nil {
		return nil, 0, err
	}

	// Check if the organization unit exists
	exists, err := ous.ouStore.IsOrganizationUnitExists(ctx, id)
	if err != nil {
		logger.Error("Failed to check organization unit existence", log.Error(err))
		return nil, 0, &serviceerror.InternalServerError
	}
	if !exists {
		return nil, 0, &ErrorOrganizationUnitNotFound
	}

	items, err := getListFunc(ctx, id, limit, offset)
	if err != nil {
		// Map composite limit error if requested
		if mapCompositeError && errors.Is(err, ErrResultLimitExceededInCompositeMode) {
			return nil, 0, &ErrorResultLimitExceeded
		}
		logger.Error("Failed to list resource", log.String("resource_type", resourceType), log.Error(err))
		return nil, 0, &serviceerror.InternalServerError
	}

	totalCount, err := getCountFunc(ctx, id)
	if err != nil {
		logger.Error("Failed to get resource count", log.String("resource_type", resourceType), log.Error(err))
		return nil, 0, &serviceerror.InternalServerError
	}

	return items, totalCount, nil
}

func buildUserListResponse(
	base string, users []User, totalCount, limit, offset int, includeDisplay bool,
) (*UserListResponse, *serviceerror.ServiceError) {
	displayQuery := utils.DisplayQueryParam(includeDisplay)
	return &UserListResponse{
		TotalResults: totalCount,
		Users:        users,
		StartIndex:   offset + 1,
		Count:        len(users),
		Links:        utils.BuildPaginationLinks(base, limit, offset, totalCount, displayQuery),
	}, nil
}

func buildGroupListResponse(
	base string, items interface{}, totalCount, limit, offset int,
) (*GroupListResponse, *serviceerror.ServiceError) {
	groups, ok := items.([]Group)
	if !ok {
		return nil, &serviceerror.InternalServerError
	}
	return &GroupListResponse{
		TotalResults: totalCount,
		Groups:       groups,
		StartIndex:   offset + 1,
		Count:        len(groups),
		Links:        utils.BuildPaginationLinks(base, limit, offset, totalCount, ""),
	}, nil
}

func buildOrganizationUnitListResponse(
	base string, items interface{}, totalCount, limit, offset int,
) (*OrganizationUnitListResponse, *serviceerror.ServiceError) {
	children, ok := items.([]OrganizationUnitBasic)
	if !ok {
		return nil, &serviceerror.InternalServerError
	}
	return &OrganizationUnitListResponse{
		TotalResults:      totalCount,
		OrganizationUnits: children,
		StartIndex:        offset + 1,
		Count:             len(children),
		Links:             utils.BuildPaginationLinks(base, limit, offset, totalCount, ""),
	}, nil
}

// GetOrganizationUnitHandlesByIDs retrieves a map of organization unit ID to handle
// for the given IDs. This is useful for enriching responses with OU handles.
func (ous *organizationUnitService) GetOrganizationUnitHandlesByIDs(
	ctx context.Context, ids []string,
) (map[string]string, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameService))

	if len(ids) == 0 {
		return map[string]string{}, nil
	}

	ouBasics, err := ous.ouStore.GetOrganizationUnitsByIDs(ctx, ids)
	if err != nil {
		logger.Error("Failed to get organization units by IDs", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	handleMap := make(map[string]string, len(ouBasics))
	for _, ou := range ouBasics {
		handleMap[ou.ID] = ou.Handle
	}

	return handleMap, nil
}

// stringPtrEqual compares two string pointers by their values.
func stringPtrEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}
