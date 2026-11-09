// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package ou handles the organization unit management operations.
package ou

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/managedresource"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const loggerComponentNameService = "OrganizationUnitService"

const (
	// projectIDQueryParam is the list query parameter that names a project.
	projectIDQueryParam = "projectId"
	// maxProjectIDLength is the length of the PROJECT_ID column. A longer project ID names no project.
	maxProjectIDLength = 36
)

// OrganizationUnitServiceInterface defines the interface for organization unit service operations.
type OrganizationUnitServiceInterface interface {
	GetOrganizationUnitList(
		ctx context.Context, limit, offset int, f *tidcommon.FilterGroup,
	) (*OrganizationUnitListResponse, *tidcommon.ServiceError)
	// GetOrganizationUnitListByProject lists the top organization units of a project: those whose parent is not
	// in the project. The others are listed under their parent. A project that does not exist has none.
	GetOrganizationUnitListByProject(
		ctx context.Context, projectID string, limit, offset int, f *tidcommon.FilterGroup,
	) (*OrganizationUnitListResponse, *tidcommon.ServiceError)
	// ProjectHasOrganizationUnits reports whether an organization unit belongs to the project.
	ProjectHasOrganizationUnits(ctx context.Context, projectID string) (bool, *tidcommon.ServiceError)
	CreateOrganizationUnit(
		ctx context.Context, request OrganizationUnitRequestWithID,
	) (OrganizationUnit, *tidcommon.ServiceError)
	GetOrganizationUnit(ctx context.Context, id string) (OrganizationUnit, *tidcommon.ServiceError)
	GetOrganizationUnitByPath(
		ctx context.Context,
		handlePath string,
	) (OrganizationUnit, *tidcommon.ServiceError)
	IsOrganizationUnitExists(ctx context.Context, id string) (bool, *tidcommon.ServiceError)
	GetSystemOUID(ctx context.Context) (string, *tidcommon.ServiceError)
	ResolveOwner(ctx context.Context, ouID, ouHandle string, errs OwnerErrors) (string, *tidcommon.ServiceError)
	ResolveOwnerForUpdate(
		ctx context.Context, requestedID, requestedHandle, currentID string, errs OwnerErrors,
	) (string, *tidcommon.ServiceError)
	IsOrganizationUnitDeclarative(ctx context.Context, id string) bool
	IsParent(ctx context.Context, parentID, childID string) (bool, *tidcommon.ServiceError)
	UpdateOrganizationUnit(
		ctx context.Context, id string, request OrganizationUnitRequestWithID,
	) (OrganizationUnit, *tidcommon.ServiceError)
	UpdateOrganizationUnitByPath(
		ctx context.Context, handlePath string, request OrganizationUnitRequestWithID,
	) (OrganizationUnit, *tidcommon.ServiceError)
	DeleteOrganizationUnit(ctx context.Context, id string) *tidcommon.ServiceError
	DeleteOrganizationUnitByPath(ctx context.Context, handlePath string) *tidcommon.ServiceError
	GetOrganizationUnitChildren(
		ctx context.Context, id string, limit, offset int, f *tidcommon.FilterGroup,
	) (*OrganizationUnitListResponse, *tidcommon.ServiceError)
	GetOrganizationUnitChildrenByPath(
		ctx context.Context, handlePath string, limit, offset int, f *tidcommon.FilterGroup,
	) (*OrganizationUnitListResponse, *tidcommon.ServiceError)
	GetOrganizationUnitUsers(
		ctx context.Context, id string, limit, offset int, includeDisplay bool,
	) (*UserListResponse, *tidcommon.ServiceError)
	GetOrganizationUnitUsersByPath(
		ctx context.Context, handlePath string, limit, offset int, includeDisplay bool,
	) (*UserListResponse, *tidcommon.ServiceError)
	GetOrganizationUnitGroups(
		ctx context.Context, id string, limit, offset int,
	) (*GroupListResponse, *tidcommon.ServiceError)
	GetOrganizationUnitGroupsByPath(
		ctx context.Context, handlePath string, limit, offset int,
	) (*GroupListResponse, *tidcommon.ServiceError)
	GetOrganizationUnitRoles(
		ctx context.Context, id string, limit, offset int,
	) (*RoleListResponse, *tidcommon.ServiceError)
	GetOrganizationUnitRolesByPath(
		ctx context.Context, handlePath string, limit, offset int,
	) (*RoleListResponse, *tidcommon.ServiceError)
	GetOrganizationUnitHandlesByIDs(
		ctx context.Context, ids []string,
	) (map[string]string, *tidcommon.ServiceError)
}

// ConfigurableOUService extends OrganizationUnitServiceInterface with methods for
// two-phase initialization of resolvers. This is intentionally separate from the
// main interface so consumers don't see bootstrap-only methods.
type ConfigurableOUService interface {
	OrganizationUnitServiceInterface
	SetOUUserResolver(resolver OUUserResolver)
	SetOUGroupResolver(resolver OUGroupResolver)
	SetOURoleResolver(resolver OURoleResolver)
	SetProjectChecker(checker projectChecker)
	SetDependencyRegistry(r resourcedependency.Registry)
	GetResourceDependencies(
		ctx context.Context, resourceType, id string) ([]resourcedependency.ResourceDependency, error)
}

// OrganizationUnitService provides organization unit management operations.
type organizationUnitService struct {
	authzService       sysauthz.SystemAuthorizationServiceInterface
	ouStore            organizationUnitStoreInterface
	transactioner      providers.Transactioner
	userResolver       OUUserResolver
	groupResolver      OUGroupResolver
	roleResolver       OURoleResolver
	dependencyRegistry resourcedependency.Registry
	// hierarchy finds the descendants an organization unit carries along when it moves to another project.
	hierarchy HierarchyEnumeratorInterface
	// projectChecker is nil on a deployment that serves no projects, where a project ID is kept unchecked.
	projectChecker projectChecker
	// projectRequired is set on a deployment that serves projects, where every organization unit but the system
	// one belongs to a project.
	projectRequired bool
	bus             eventbus.EventBusInterface
	// managed says which organization units a control plane deployed, which stay read only here. Nil
	// where nothing is deployed from one.
	managed managedresource.StoreInterface
}

// projectChecker reports whether the project an organization unit names exists.
type projectChecker interface {
	ProjectExists(ctx context.Context, id string) (bool, *tidcommon.ServiceError)
}

func (ous *organizationUnitService) SetOUUserResolver(resolver OUUserResolver) {
	ous.userResolver = resolver
}

// SetDependencyRegistry injects the dependency registry. Called by servicemanager after the
// provider services are initialized to avoid a cyclic import.
func (ous *organizationUnitService) SetDependencyRegistry(r resourcedependency.Registry) {
	ous.dependencyRegistry = r
}

func (ous *organizationUnitService) SetOUGroupResolver(resolver OUGroupResolver) {
	ous.groupResolver = resolver
}

func (ous *organizationUnitService) SetOURoleResolver(resolver OURoleResolver) {
	ous.roleResolver = resolver
}

// SetProjectChecker injects the check of the project an organization unit names. Called by servicemanager once
// the project service, which depends on this service, is initialized.
func (ous *organizationUnitService) SetProjectChecker(checker projectChecker) {
	ous.projectChecker = checker
}

// refuseChange refuses a change to an organization unit declared in a file, or deployed by a control
// plane unless the change is the managed import's.
func (ous *organizationUnitService) refuseChange(ctx context.Context, id string) *tidcommon.ServiceError {
	if ous.ouStore.IsOrganizationUnitDeclarative(ctx, id) {
		return &ErrorCannotModifyDeclarativeResource
	}
	managed, err := managedresource.IsReadOnly(ctx, ous.managed, id, managedresource.TypeOrganizationUnit)
	if err != nil {
		log.GetLogger().Error(ctx, "Failed to read whether a control plane deployed the organization unit",
			log.String("id", id), log.Error(err))
		return &tidcommon.InternalServerError
	}
	if managed {
		return &ErrorManagedOrganizationUnit
	}
	return nil
}

// withManaged reports an organization unit a control plane deployed as read only. A failed read of that is
// logged and taken as not: every write to such a unit is refused whatever the read says.
func (ous *organizationUnitService) withManaged(
	ctx context.Context, ou OrganizationUnit) (OrganizationUnit, *tidcommon.ServiceError) {
	deployed, err := managedresource.IsDeployed(ctx, ous.managed, ou.ID, managedresource.TypeOrganizationUnit)
	if err != nil {
		log.GetLogger().Error(ctx, "Failed to read whether a control plane deployed the organization unit",
			log.String("id", ou.ID), log.Error(err))
	}
	ou.IsReadOnly = deployed
	return ou, nil
}

// markManaged reports the listed organization units a control plane deployed as read only.
func (ous *organizationUnitService) markManaged(ctx context.Context, units []OrganizationUnitBasic) {
	if err := managedresource.MarkDeployed(ctx, ous.managed, units,
		func(u *OrganizationUnitBasic) string { return u.ID },
		func(u *OrganizationUnitBasic) { u.IsReadOnly = true },
		managedresource.TypeOrganizationUnit); err != nil {
		// Every write to such a unit is refused whatever the list says, so it is served without the flag.
		log.GetLogger().Error(ctx, "Failed to read which organization units a control plane deployed",
			log.Error(err))
	}
}

// newOrganizationUnitService creates a new instance of OrganizationUnitService.
func newOrganizationUnitService(
	authzService sysauthz.SystemAuthorizationServiceInterface,
	ouStore organizationUnitStoreInterface,
	transactioner providers.Transactioner,
	projectRequired bool,
	bus eventbus.EventBusInterface,
) ConfigurableOUService {
	return &organizationUnitService{
		authzService:    authzService,
		ouStore:         ouStore,
		transactioner:   transactioner,
		hierarchy:       newOUHierarchyEnumerator(ouStore),
		projectRequired: projectRequired,
		bus:             bus,
	}
}

// GetOrganizationUnitList retrieves a list of organization units.
// limit should be a positive integer and offset should be non-negative.
func (ous *organizationUnitService) GetOrganizationUnitList(
	ctx context.Context, limit, offset int, f *tidcommon.FilterGroup,
) (
	*OrganizationUnitListResponse, *tidcommon.ServiceError,
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
		return nil, &tidcommon.InternalServerError
	}

	var list *OrganizationUnitListResponse
	if accessible.AllAllowed {
		// Unfiltered path: the caller can see all organization units.
		list, svcErr = ous.listAllOrganizationUnits(ctx, limit, offset, f)
	} else {
		// Filtered path: the caller has a restricted set of accessible organization units.
		list, svcErr = ous.listAccessibleOrganizationUnits(ctx, accessible.IDs, "", limit, offset, f)
	}
	if list != nil {
		ous.markManaged(ctx, list.OrganizationUnits)
	}
	return list, svcErr
}

// GetOrganizationUnitListByProject lists the top organization units of a project: its root organization units,
// and any unit whose parent is not in the project. An empty project ID names no project, so it lists none
// rather than every organization unit.
func (ous *organizationUnitService) GetOrganizationUnitListByProject(
	ctx context.Context, projectID string, limit, offset int, f *tidcommon.FilterGroup,
) (*OrganizationUnitListResponse, *tidcommon.ServiceError) {
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

	extraQuery := "&" + projectIDQueryParam + "=" + url.QueryEscape(projectID)
	if projectID == "" {
		return &OrganizationUnitListResponse{
			OrganizationUnits: []OrganizationUnitBasic{},
			StartIndex:        offset + 1,
			Links:             utils.BuildPaginationLinks("/organization-units", limit, offset, 0, extraQuery),
		}, nil
	}

	accessible, svcErr := ous.authzService.GetAccessibleResources(
		ctx, security.ActionListOUs, security.ResourceTypeOU)
	if svcErr != nil {
		return nil, &tidcommon.InternalServerError
	}
	if !accessible.AllAllowed {
		return ous.listAccessibleOrganizationUnits(ctx, accessible.IDs, projectID, limit, offset, f)
	}

	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameService))
	totalCount, err := ous.ouStore.GetOrganizationUnitListCountByProject(ctx, projectID, f)
	if errors.Is(err, ErrResultLimitExceededInCompositeMode) {
		return nil, &ErrorResultLimitExceeded
	}
	if err != nil {
		logger.Error(ctx, "Failed to get organization unit count in project", log.String("projectID", projectID),
			log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	ouList, err := ous.ouStore.GetOrganizationUnitListByProject(ctx, projectID, limit, offset, f)
	if err != nil {
		if errors.Is(err, ErrResultLimitExceededInCompositeMode) {
			return nil, &ErrorResultLimitExceeded
		}
		logger.Error(ctx, "Failed to list organization units in project", log.String("projectID", projectID),
			log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	return &OrganizationUnitListResponse{
		TotalResults:      totalCount,
		OrganizationUnits: ouList,
		StartIndex:        offset + 1,
		Count:             len(ouList),
		Links:             utils.BuildPaginationLinks("/organization-units", limit, offset, totalCount, extraQuery),
	}, nil
}

// ProjectHasOrganizationUnits reports whether an organization unit belongs to the project. Every organization
// unit of a project is under one of its top organization units, so counting those is enough.
func (ous *organizationUnitService) ProjectHasOrganizationUnits(
	ctx context.Context, projectID string,
) (bool, *tidcommon.ServiceError) {
	count, err := ous.ouStore.GetOrganizationUnitListCountByProject(ctx, projectID, nil)
	// A project with more organization units than a composite read returns still has organization units.
	if errors.Is(err, ErrResultLimitExceededInCompositeMode) {
		return true, nil
	}
	if err != nil {
		log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameService)).Error(ctx,
			"Failed to count the organization units of a project", log.String("projectID", projectID),
			log.Error(err))
		return false, &tidcommon.InternalServerError
	}
	return count > 0, nil
}

// listAllOrganizationUnits retrieves organization units without authorization filtering.
func (ous *organizationUnitService) listAllOrganizationUnits(
	ctx context.Context, limit, offset int, f *tidcommon.FilterGroup,
) (*OrganizationUnitListResponse, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameService))
	totalCount, err := ous.ouStore.GetOrganizationUnitListCount(ctx, f)
	if err != nil {
		logger.Error(ctx, "Failed to get organization unit count", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	ouList, err := ous.ouStore.GetOrganizationUnitList(ctx, limit, offset, f)
	if err != nil {
		// Check if it's a limit exceeded error
		if errors.Is(err, ErrResultLimitExceededInCompositeMode) {
			return nil, &ErrorResultLimitExceeded
		}
		logger.Error(ctx, "Failed to list organization units", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	return &OrganizationUnitListResponse{
		TotalResults:      totalCount,
		OrganizationUnits: ouList,
		StartIndex:        offset + 1,
		Count:             len(ouList),
		Links:             utils.BuildPaginationLinks("/organization-units", limit, offset, totalCount, ""),
	}, nil
}

// listAccessibleOrganizationUnits retrieves only the organization units the caller is authorized to access,
// and only the ones in a project when a project is given.
// When g is nil and no project is given it paginates the ID slice first and fetches only the needed page
// (efficient path). Otherwise it fetches all authorized OUs, applies the filter in memory, then paginates.
func (ous *organizationUnitService) listAccessibleOrganizationUnits(
	ctx context.Context, ids []string, projectID string, limit, offset int, g *tidcommon.FilterGroup,
) (*OrganizationUnitListResponse, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameService))
	extraQuery := ""
	if projectID != "" {
		extraQuery = "&" + projectIDQueryParam + "=" + url.QueryEscape(projectID)
	}

	if len(ids) == 0 {
		return &OrganizationUnitListResponse{
			TotalResults:      0,
			OrganizationUnits: []OrganizationUnitBasic{},
			StartIndex:        1,
			Count:             0,
			Links:             utils.BuildPaginationLinks("/organization-units", limit, offset, 0, extraQuery),
		}, nil
	}

	if g != nil || projectID != "" {
		// Fetch all authorized OUs then apply the filter in memory so TotalResults
		// reflects the filtered count, not the raw authorized-ID count.
		allOUs, err := ous.ouStore.GetOrganizationUnitsByIDs(ctx, ids)
		if err != nil {
			logger.Error(ctx, "Failed to get organization units by IDs", log.Error(err))
			return nil, &tidcommon.InternalServerError
		}

		filtered := make([]OrganizationUnitBasic, 0, len(allOUs))
		for _, ou := range allOUs {
			if (projectID == "" || ou.ProjectID == projectID) && matchesOUBasicFilter(ou, g) {
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
			Links:             utils.BuildPaginationLinks("/organization-units", limit, offset, total, extraQuery),
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
			Links:             utils.BuildPaginationLinks("/organization-units", limit, offset, total, extraQuery),
		}, nil
	}

	pageOUs, err := ous.ouStore.GetOrganizationUnitsByIDs(ctx, pageIDs)
	if err != nil {
		logger.Error(ctx, "Failed to get organization units by IDs", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	return &OrganizationUnitListResponse{
		TotalResults:      total,
		OrganizationUnits: pageOUs,
		StartIndex:        offset + 1,
		Count:             len(pageOUs),
		Links:             utils.BuildPaginationLinks("/organization-units", limit, offset, total, extraQuery),
	}, nil
}

// CreateOrganizationUnit creates a new organization unit.
func (ous *organizationUnitService) CreateOrganizationUnit(
	ctx context.Context, request OrganizationUnitRequestWithID,
) (OrganizationUnit, *tidcommon.ServiceError) {
	created, svcErr := ous.createOrganizationUnit(ctx, request)
	var ouID string
	if svcErr == nil {
		ouID = created.ID
	}
	ous.publishOUEvent(ctx, audit.EventOUCreate, audit.ActionCreate, ouID, svcErr)
	return created, svcErr
}

// createOrganizationUnit creates a new organization unit without publishing its audit event.
func (ous *organizationUnitService) createOrganizationUnit(
	ctx context.Context, request OrganizationUnitRequestWithID,
) (OrganizationUnit, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameService))
	logger.Debug(ctx, "Creating organization unit", log.String("name", request.Name))

	// Fail if store is in declarative mode
	if isDeclarativeModeEnabled() {
		return OrganizationUnit{}, &ErrorCannotModifyDeclarativeResource
	}

	var createdOU OrganizationUnit
	var capturedSvcErr *tidcommon.ServiceError

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

		if svcErr := ous.validateNewOUProject(txCtx, request); svcErr != nil {
			capturedSvcErr = svcErr
			return errors.New("project error")
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
			ID:                        ouID,
			Handle:                    request.Handle,
			Name:                      request.Name,
			Description:               request.Description,
			Parent:                    request.Parent,
			ProjectID:                 request.ProjectID,
			ThemeID:                   request.ThemeID,
			LayoutID:                  request.LayoutID,
			AuthFlowID:                request.AuthFlowID,
			RegistrationFlowID:        request.RegistrationFlowID,
			IsRegistrationFlowEnabled: request.IsRegistrationFlowEnabled,
			RecoveryFlowID:            request.RecoveryFlowID,
			IsRecoveryFlowEnabled:     request.IsRecoveryFlowEnabled,
			SignOutFlowID:             request.SignOutFlowID,
			UserOnboardingFlowID:      request.UserOnboardingFlowID,
			LogoURL:                   request.LogoURL,
			TosURI:                    request.TosURI,
			PolicyURI:                 request.PolicyURI,
			CookiePolicyURI:           request.CookiePolicyURI,
			CreatedAt:                 now,
			UpdatedAt:                 now,
		}

		err = ous.ouStore.CreateOrganizationUnit(txCtx, createdOU)
		return err
	})

	if capturedSvcErr != nil {
		return OrganizationUnit{}, capturedSvcErr
	}
	if err != nil {
		logger.Error(ctx, "Failed to create organization unit",
			log.Error(err), log.String("name", request.Name))
		return OrganizationUnit{}, &tidcommon.InternalServerError
	}

	logger.Debug(ctx, "Successfully created organization unit", log.String("ouID", createdOU.ID))

	return createdOU, nil
}

// GetOrganizationUnit retrieves an organization unit by ID.
func (ous *organizationUnitService) GetOrganizationUnit(
	ctx context.Context, id string,
) (OrganizationUnit, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameService))
	logger.Debug(ctx, "Getting organization unit", log.String("ouID", id))

	if svcErr := ous.checkOUAccess(ctx, security.ActionReadOU, id); svcErr != nil {
		return OrganizationUnit{}, svcErr
	}

	ou, err := ous.ouStore.GetOrganizationUnit(ctx, id)
	if err != nil {
		if errors.Is(err, ErrOrganizationUnitNotFound) {
			return OrganizationUnit{}, &ErrorOrganizationUnitNotFound
		}
		logger.Error(ctx, "Failed to get organization unit", log.Error(err))
		return OrganizationUnit{}, &tidcommon.InternalServerError
	}

	return ous.withManaged(ctx, ou)
}

// GetOrganizationUnitByPath retrieves an organization unit by hierarchical handle path.
func (ous *organizationUnitService) GetOrganizationUnitByPath(
	ctx context.Context, handlePath string,
) (OrganizationUnit, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameService))
	logger.Debug(ctx, "Getting organization unit by path", log.String("path", handlePath))

	handles, serviceError := validateAndProcessHandlePath(handlePath)
	if serviceError != nil {
		return OrganizationUnit{}, serviceError
	}

	ou, err := ous.ouStore.GetOrganizationUnitByPath(ctx, handles)
	if err != nil {
		if errors.Is(err, ErrOrganizationUnitNotFound) {
			return OrganizationUnit{}, &ErrorOrganizationUnitNotFound
		}
		logger.Error(ctx, "Failed to get organization unit by path", log.Error(err))
		return OrganizationUnit{}, &tidcommon.InternalServerError
	}

	if svcErr := ous.checkOUAccess(ctx, security.ActionReadOU, ou.ID); svcErr != nil {
		return OrganizationUnit{}, svcErr
	}

	return ous.withManaged(ctx, ou)
}

// IsOrganizationUnitExists checks if an organization unit exists by ID.
func (ous *organizationUnitService) IsOrganizationUnitExists(
	ctx context.Context, id string,
) (bool, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameService))
	logger.Debug(ctx, "Checking if organization unit exists", log.String("ouID", id))

	exists, err := ous.ouStore.IsOrganizationUnitExists(ctx, id)
	if err != nil {
		logger.Error(ctx, "Failed to check organization unit existence", log.Error(err))
		return false, &tidcommon.InternalServerError
	}

	return exists, nil
}

// GetSystemOUID returns the ID of the system organization unit that server.system_ou_id names: the one
// resources in a project are placed in, and that resources naming no organization unit go to. It is the
// deployment's configuration, so one that is not configured or does not exist is a server error.
func (ous *organizationUnitService) GetSystemOUID(ctx context.Context) (string, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameService))
	ouID := configuredSystemOUID()
	if ouID == "" {
		logger.Error(ctx, "No system organization unit is configured in server.system_ou_id")
		return "", &tidcommon.InternalServerError
	}
	exists, err := ous.ouStore.IsOrganizationUnitExists(ctx, ouID)
	if err != nil {
		logger.Error(ctx, "Failed to check the system organization unit", log.Error(err))
		return "", &tidcommon.InternalServerError
	}
	if !exists {
		logger.Error(ctx, "The system organization unit in server.system_ou_id does not exist",
			log.String("ouID", ouID))
		return "", &tidcommon.InternalServerError
	}
	return ouID, nil
}

func (ous *organizationUnitService) IsOrganizationUnitDeclarative(ctx context.Context, id string) bool {
	return ous.ouStore.IsOrganizationUnitDeclarative(ctx, id)
}

// IsParent checks whether the provided parentID is an ancestor of childID.
// Returns true if the parent and child are the same or if parentID is an ancestor of childID.
func (ous *organizationUnitService) IsParent(
	ctx context.Context, parentID, childID string,
) (bool, *tidcommon.ServiceError) {
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
				logger.Debug(ctx, "Encountered missing organization unit in hierarchy",
					log.String("ouID", *currentParent))
				return false, &ErrorOrganizationUnitNotFound
			}
			logger.Error(ctx, "Failed to traverse organization unit hierarchy", log.Error(err))
			return false, &tidcommon.InternalServerError
		}

		currentParent = parentOU.Parent
	}

	return false, nil
}

// UpdateOrganizationUnit updates an organization unit.
func (ous *organizationUnitService) UpdateOrganizationUnit(
	ctx context.Context, id string, request OrganizationUnitRequestWithID,
) (OrganizationUnit, *tidcommon.ServiceError) {
	updated, svcErr := ous.updateOrganizationUnit(ctx, id, request)
	ous.publishOUEvent(ctx, audit.EventOUUpdate, audit.ActionUpdate, id, svcErr)
	return updated, svcErr
}

// updateOrganizationUnit updates an organization unit without publishing its audit event.
func (ous *organizationUnitService) updateOrganizationUnit(
	ctx context.Context, id string, request OrganizationUnitRequestWithID,
) (OrganizationUnit, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameService))
	logger.Debug(ctx, "Updating organization unit", log.String("ouID", id))

	if svcErr := ous.checkOUAccess(ctx, security.ActionUpdateOU, id); svcErr != nil {
		return OrganizationUnit{}, svcErr
	}

	var updatedOU OrganizationUnit
	var capturedSvcErr *tidcommon.ServiceError

	err := ous.transactioner.Transact(ctx, func(txCtx context.Context) error {
		existingOU, err := ous.ouStore.GetOrganizationUnit(txCtx, id)
		if err != nil {
			if errors.Is(err, ErrOrganizationUnitNotFound) {
				capturedSvcErr = &ErrorOrganizationUnitNotFound
				return err
			}
			return err
		}

		var svcErr *tidcommon.ServiceError
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
		logger.Error(ctx, "Failed to update organization unit", log.Error(err), log.String("ouID", id))
		return OrganizationUnit{}, &tidcommon.InternalServerError
	}

	logger.Debug(ctx, "Successfully updated organization unit", log.String("ouID", id))
	return updatedOU, nil
}

// UpdateOrganizationUnitByPath updates an organization unit by hierarchical handle path.
func (ous *organizationUnitService) UpdateOrganizationUnitByPath(
	ctx context.Context, handlePath string, request OrganizationUnitRequestWithID,
) (OrganizationUnit, *tidcommon.ServiceError) {
	updated, ouID, svcErr := ous.updateOrganizationUnitByPath(ctx, handlePath, request)
	ous.publishOUEvent(ctx, audit.EventOUUpdate, audit.ActionUpdate, ouID, svcErr)
	return updated, svcErr
}

// updateOrganizationUnitByPath updates an organization unit by hierarchical handle path without
// publishing its audit event. It returns the ID the path resolved to, empty when it resolved to none.
func (ous *organizationUnitService) updateOrganizationUnitByPath(
	ctx context.Context, handlePath string, request OrganizationUnitRequestWithID,
) (OrganizationUnit, string, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameService))
	logger.Debug(ctx, "Updating organization unit by path", log.String("path", handlePath))

	handles, serviceError := validateAndProcessHandlePath(handlePath)
	if serviceError != nil {
		return OrganizationUnit{}, "", serviceError
	}

	var ouID string
	var updatedOU OrganizationUnit
	var capturedSvcErr *tidcommon.ServiceError

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

		if svcErr := ous.checkOUAccess(txCtx, security.ActionUpdateOU, existingOU.ID); svcErr != nil {
			capturedSvcErr = svcErr
			return errors.New("authz error")
		}

		// Check if OU is declarative (for composite mode) or deployed by a control plane
		if svcErr := ous.refuseChange(txCtx, existingOU.ID); svcErr != nil {
			capturedSvcErr = svcErr
			return errors.New("declarative resource")
		}

		var svcErr *tidcommon.ServiceError
		updatedOU, svcErr = ous.updateOUInternal(txCtx, existingOU.ID, request, existingOU, logger)
		if svcErr != nil {
			capturedSvcErr = svcErr
			return errors.New("update error")
		}
		return nil
	})

	if capturedSvcErr != nil {
		return OrganizationUnit{}, ouID, capturedSvcErr
	}
	if err != nil {
		logger.Error(ctx, "Failed to update organization unit by path",
			log.Error(err), log.String("path", handlePath))
		return OrganizationUnit{}, ouID, &tidcommon.InternalServerError
	}

	logger.Debug(ctx, "Successfully updated organization unit by path", log.String("ouID", updatedOU.ID))
	return updatedOU, ouID, nil
}

func (ous *organizationUnitService) updateOUInternal(
	ctx context.Context,
	id string,
	request OrganizationUnitRequestWithID,
	existingOU OrganizationUnit,
	logger *log.Logger,
) (OrganizationUnit, *tidcommon.ServiceError) {
	// Check if OU is immutable (for composite mode) or deployed by a control plane
	if svcErr := ous.refuseChange(ctx, id); svcErr != nil {
		return OrganizationUnit{}, svcErr
	}

	if err := ous.validateOUName(request.Name); err != nil {
		return OrganizationUnit{}, err
	}

	if err := ous.validateOUHandle(request.Handle); err != nil {
		return OrganizationUnit{}, err
	}

	// Moving an organization unit to the root makes it a root organization unit, so the caller must be
	// allowed to create one.
	parentChanged := !stringPtrEqual(existingOU.Parent, request.Parent)
	if parentChanged && request.Parent == nil {
		if svcErr := ous.checkOUAccess(ctx, security.ActionCreateOU, ""); svcErr != nil {
			return OrganizationUnit{}, svcErr
		}
	}

	if request.Parent != nil {
		exists, err := ous.ouStore.IsOrganizationUnitExists(ctx, *request.Parent)
		if err != nil {
			logger.Error(ctx, "Failed to check parent organization unit existence", log.Error(err))
			return OrganizationUnit{}, &tidcommon.InternalServerError
		}
		if !exists {
			return OrganizationUnit{}, &ErrorParentOrganizationUnitNotFound
		}
	}

	if err := ous.checkCircularDependency(ctx, id, request.Parent); err != nil {
		return OrganizationUnit{}, err
	}

	projectID, svcErr := ous.resolveUpdatedOUProject(ctx, existingOU, request, parentChanged)
	if svcErr != nil {
		return OrganizationUnit{}, svcErr
	}

	var nameConflict bool
	var err error
	if parentChanged || existingOU.Name != request.Name {
		nameConflict, err = ous.ouStore.CheckOrganizationUnitNameConflict(ctx, request.Name, request.Parent)
		if err != nil {
			logger.Error(ctx, "Failed to check organization unit name conflict", log.Error(err))
			return OrganizationUnit{}, &tidcommon.InternalServerError
		}
	}

	if nameConflict {
		return OrganizationUnit{}, &ErrorOrganizationUnitNameConflict
	}

	var handleConflict bool
	if parentChanged || existingOU.Handle != request.Handle {
		handleConflict, err = ous.ouStore.CheckOrganizationUnitHandleConflict(ctx, request.Handle, request.Parent)
		if err != nil {
			logger.Error(ctx, "Failed to check organization unit handle conflict", log.Error(err))
			return OrganizationUnit{}, &tidcommon.InternalServerError
		}
	}

	if handleConflict {
		return OrganizationUnit{}, &ErrorOrganizationUnitHandleConflict
	}

	updatedOU := OrganizationUnit{
		ID:                        existingOU.ID,
		Handle:                    request.Handle,
		Name:                      request.Name,
		Description:               request.Description,
		Parent:                    request.Parent,
		ProjectID:                 projectID,
		ThemeID:                   request.ThemeID,
		LayoutID:                  request.LayoutID,
		AuthFlowID:                request.AuthFlowID,
		RegistrationFlowID:        request.RegistrationFlowID,
		IsRegistrationFlowEnabled: request.IsRegistrationFlowEnabled,
		RecoveryFlowID:            request.RecoveryFlowID,
		IsRecoveryFlowEnabled:     request.IsRecoveryFlowEnabled,
		SignOutFlowID:             request.SignOutFlowID,
		UserOnboardingFlowID:      request.UserOnboardingFlowID,
		LogoURL:                   request.LogoURL,
		TosURI:                    request.TosURI,
		PolicyURI:                 request.PolicyURI,
		CookiePolicyURI:           request.CookiePolicyURI,
		CreatedAt:                 existingOU.CreatedAt,
		UpdatedAt:                 time.Now().UTC(),
	}

	// A unit that moves to another project carries its descendants along. They move first and the unit is
	// written last, so a failure leaves no new value in the cache of a transaction that rolls back.
	if projectID != existingOU.ProjectID {
		if svcErr := ous.moveDescendantsToProject(ctx, id, projectID, updatedOU.UpdatedAt, logger); svcErr != nil {
			return OrganizationUnit{}, svcErr
		}
	}

	err = ous.ouStore.UpdateOrganizationUnit(ctx, updatedOU)
	if err != nil {
		if errors.Is(err, ErrOrganizationUnitNotFound) {
			return OrganizationUnit{}, &ErrorOrganizationUnitNotFound
		}
		logger.Error(ctx, "Failed to update organization unit", log.Error(err))
		return OrganizationUnit{}, &tidcommon.InternalServerError
	}
	return updatedOU, nil
}

// DeleteOrganizationUnit deletes an organization unit.
func (ous *organizationUnitService) DeleteOrganizationUnit(
	ctx context.Context, id string) *tidcommon.ServiceError {
	svcErr := ous.deleteOrganizationUnit(ctx, id)
	ous.publishOUEvent(ctx, audit.EventOUDelete, audit.ActionDelete, id, svcErr)
	return svcErr
}

// deleteOrganizationUnit deletes an organization unit without publishing its audit event.
func (ous *organizationUnitService) deleteOrganizationUnit(
	ctx context.Context, id string) *tidcommon.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameService))
	logger.Debug(ctx, "Deleting organization unit", log.String("ouID", id))

	if svcErr := ous.checkOUAccess(ctx, security.ActionDeleteOU, id); svcErr != nil {
		return svcErr
	}

	var capturedSvcErr *tidcommon.ServiceError

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
		logger.Error(ctx, "Failed to delete organization unit", log.Error(err), log.String("ouID", id))
		return &tidcommon.InternalServerError
	}

	logger.Debug(ctx, "Successfully deleted organization unit", log.String("ouID", id))
	return nil
}

// DeleteOrganizationUnitByPath deletes an organization unit by hierarchical handle path.
func (ous *organizationUnitService) DeleteOrganizationUnitByPath(
	ctx context.Context, handlePath string,
) *tidcommon.ServiceError {
	ouID, svcErr := ous.deleteOrganizationUnitByPath(ctx, handlePath)
	ous.publishOUEvent(ctx, audit.EventOUDelete, audit.ActionDelete, ouID, svcErr)
	return svcErr
}

// deleteOrganizationUnitByPath deletes an organization unit by hierarchical handle path without
// publishing its audit event. It returns the ID the path resolved to, empty when it resolved to none.
func (ous *organizationUnitService) deleteOrganizationUnitByPath(
	ctx context.Context, handlePath string,
) (string, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameService))
	logger.Debug(ctx, "Deleting organization unit by path", log.String("path", handlePath))

	handles, serviceError := validateAndProcessHandlePath(handlePath)
	if serviceError != nil {
		return "", serviceError
	}

	var ouID string
	var capturedSvcErr *tidcommon.ServiceError

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

		// Check if OU is declarative (for composite mode) or deployed by a control plane
		if svcErr := ous.refuseChange(txCtx, ouID); svcErr != nil {
			capturedSvcErr = svcErr
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
		return ouID, capturedSvcErr
	}
	if err != nil {
		logger.Error(ctx, "Failed to delete organization unit by path",
			log.Error(err), log.String("path", handlePath))
		return ouID, &tidcommon.InternalServerError
	}

	logger.Debug(ctx, "Successfully deleted organization unit by path", log.String("ouID", ouID))
	return ouID, nil
}

// publishOUEvent publishes the audit event of an attempt on the organization unit with ouID, after the
// change committed or was refused. Anything removed along with a deleted unit writes no event of its own.
func (ous *organizationUnitService) publishOUEvent(ctx context.Context, name string, action audit.Action,
	ouID string, svcErr *tidcommon.ServiceError) {
	// A failed attempt keeps the ID the caller asked for, even one that matches no organization unit, as
	// forensic evidence of what was probed; the payload caps it and the sink escapes it. An attempt by
	// handle path names the unit the path resolved to, and none when it resolved to none. An update or a
	// delete by ID is authorized before the unit is looked up, so a caller refused there may be denied on
	// an ID that matches no organization unit, which is then the subject: a known limitation.
	target := audit.Ref{Type: audit.TargetTypeOU, ID: ouID}
	eventbus.TryPublish(ctx, ous.bus,
		eventbus.NewEvent(ctx, name).With(audit.ChangePayload(ctx, target, action, svcErr)))
}

// deleteOUInternal deletes an organization unit by ID after checking if it has child resources.
func (ous *organizationUnitService) deleteOUInternal(
	ctx context.Context, id string, logger *log.Logger,
) *tidcommon.ServiceError {
	if id == configuredSystemOUID() {
		return &ErrorCannotDeleteSystemOU
	}
	// Check if OU is immutable (for composite mode) or deployed by a control plane
	if svcErr := ous.refuseChange(ctx, id); svcErr != nil {
		return svcErr
	}

	// Refuse deletion when child organization units, users or groups still depend on this
	// organization unit. Dependencies are aggregated through the dependency registry.
	if svcErr := ous.ensureNoBlockingDependencies(ctx, id, logger); svcErr != nil {
		return svcErr
	}

	err := ous.ouStore.DeleteOrganizationUnit(ctx, id)
	if err != nil {
		if errors.Is(err, ErrOrganizationUnitNotFound) {
			return &ErrorOrganizationUnitNotFound
		}
		logger.Error(ctx, "Failed to delete organization unit", log.Error(err))
		return &tidcommon.InternalServerError
	}
	return nil
}

// ensureNoBlockingDependencies refuses deletion when other resources depend on the organization unit
// in a way that forbids it (behaviorOnDelete == restrict), such as child organization units, users or
// groups. Because deletion is destructive, it fails closed: if dependency data cannot be determined,
// the deletion is refused rather than allowed.
func (ous *organizationUnitService) ensureNoBlockingDependencies(
	ctx context.Context, id string, logger *log.Logger,
) *tidcommon.ServiceError {
	if ous.dependencyRegistry == nil {
		logger.Error(ctx, "Dependency registry not set; refusing to delete organization unit")
		return &tidcommon.InternalServerError
	}

	deps, err := ous.dependencyRegistry.GetDependencies(ctx, resourcedependency.ResourceTypeOU, id)
	if err != nil {
		logger.Error(ctx, "Failed to evaluate organization unit dependencies", log.Error(err))
		return &tidcommon.InternalServerError
	}
	// Fail closed: nil TotalResults means a provider failed to report, so usage is unknown.
	if deps == nil || deps.TotalResults == nil {
		logger.Error(ctx, "Organization unit dependency data unavailable; refusing to delete")
		return &tidcommon.InternalServerError
	}

	if len(resourcedependency.BlockingUsages(deps)) == 0 {
		return nil
	}

	return &ErrorCannotDeleteOrganizationUnit
}

// GetResourceDependencies implements resourcedependency.Provider. It reports the child organization
// units of the given organization unit, which block its deletion (a child cannot exist without its
// parent). Only organization unit targets are handled; other resource types have no child
// dependencies. The number of children scanned is bounded by MaxCompositeStoreRecords.
func (ous *organizationUnitService) GetResourceDependencies(
	ctx context.Context, resourceType, id string) ([]resourcedependency.ResourceDependency, error) {
	if resourceType != resourcedependency.ResourceTypeOU {
		return []resourcedependency.ResourceDependency{}, nil
	}

	children, err := ous.ouStore.GetOrganizationUnitChildrenList(
		ctx, id, serverconst.MaxCompositeStoreRecords, 0, nil)
	if err != nil {
		return nil, err
	}

	deps := make([]resourcedependency.ResourceDependency, 0, len(children))
	for _, child := range children {
		deps = append(deps, resourcedependency.ResourceDependency{
			ResourceType:     resourcedependency.ResourceTypeOU,
			ID:               child.ID,
			DisplayName:      child.Name,
			BehaviorOnDelete: resourcedependency.BehaviorRestrict,
		})
	}
	return deps, nil
}

// checkOUAccess validates that the caller is authorized to perform the given action on an organization unit.
// Pass an empty ouID when there is no specific resource context (e.g. creating a root-level OU).
func (ous *organizationUnitService) checkOUAccess(
	ctx context.Context, action security.Action, ouID string,
) *tidcommon.ServiceError {
	allowed, svcErr := ous.authzService.IsActionAllowed(ctx, action,
		&sysauthz.ActionContext{ResourceType: security.ResourceTypeOU, OUID: ouID})
	if svcErr != nil {
		return &tidcommon.InternalServerError
	}
	if !allowed {
		return &tidcommon.ErrorUnauthorized
	}
	return nil
}

// GetOrganizationUnitUsers retrieves a list of users for a given organization unit ID.
func (ous *organizationUnitService) GetOrganizationUnitUsers(
	ctx context.Context, id string, limit, offset int, includeDisplay bool,
) (*UserListResponse, *tidcommon.ServiceError) {
	if svcErr := ous.checkOUAccess(ctx, security.ActionReadUser, id); svcErr != nil {
		return nil, svcErr
	}
	if ous.userResolver == nil {
		return nil, &tidcommon.InternalServerError
	}

	items, totalCount, svcErr := ous.getResourceListWithExistenceCheck(
		ctx, id, limit, offset, "users",
		func(ctx context.Context, id string, limit, offset int) (interface{}, int, error) {
			return ous.userResolver.GetUserListByOUID(ctx, id, limit, offset, includeDisplay)
		},
		false, // No composite error mapping for users
	)
	if svcErr != nil {
		return nil, svcErr
	}

	users, ok := items.([]User)
	if !ok {
		logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameService))
		logger.Error(ctx, "Failed to cast user list response for organization unit", log.String("ouID", id))
		return nil, &tidcommon.InternalServerError
	}

	base := fmt.Sprintf("/organization-units/%s/users", id)
	return buildUserListResponse(base, users, totalCount, limit, offset, includeDisplay)
}

// GetOrganizationUnitGroups retrieves a list of groups for a given organization unit ID.
func (ous *organizationUnitService) GetOrganizationUnitGroups(
	ctx context.Context, id string, limit, offset int,
) (*GroupListResponse, *tidcommon.ServiceError) {
	if svcErr := ous.checkOUAccess(ctx, security.ActionReadGroup, id); svcErr != nil {
		return nil, svcErr
	}
	if ous.groupResolver == nil {
		return nil, &tidcommon.InternalServerError
	}

	items, totalCount, svcErr := ous.getResourceListWithExistenceCheck(
		ctx, id, limit, offset, "groups",
		listThenCount(
			func(ctx context.Context, id string, limit, offset int) (interface{}, error) {
				return ous.groupResolver.GetGroupListByOUID(ctx, id, limit, offset)
			},
			ous.groupResolver.GetGroupCountByOUID,
		),
		false, // No composite error mapping for groups
	)
	if svcErr != nil {
		return nil, svcErr
	}
	base := fmt.Sprintf("/organization-units/%s/groups", id)
	return buildGroupListResponse(base, items, totalCount, limit, offset)
}

// GetOrganizationUnitRoles retrieves a list of roles for a given organization unit ID.
func (ous *organizationUnitService) GetOrganizationUnitRoles(
	ctx context.Context, id string, limit, offset int,
) (*RoleListResponse, *tidcommon.ServiceError) {
	if svcErr := ous.checkOUAccess(ctx, security.ActionReadRole, id); svcErr != nil {
		return nil, svcErr
	}
	if ous.roleResolver == nil {
		return nil, &tidcommon.InternalServerError
	}

	items, totalCount, svcErr := ous.getResourceListWithExistenceCheck(
		ctx, id, limit, offset, "roles",
		func(ctx context.Context, id string, limit, offset int) (interface{}, int, error) {
			return ous.roleResolver.GetRoleListByOUID(ctx, id, limit, offset)
		},
		false, // No composite error mapping for roles
	)
	if svcErr != nil {
		return nil, svcErr
	}
	base := fmt.Sprintf("/organization-units/%s/roles", id)
	return buildRoleListResponse(base, items, totalCount, limit, offset)
}

// GetOrganizationUnitChildren retrieves a list of child organization units for a given organization unit ID.
func (ous *organizationUnitService) GetOrganizationUnitChildren(
	ctx context.Context, id string, limit, offset int, f *tidcommon.FilterGroup,
) (*OrganizationUnitListResponse, *tidcommon.ServiceError) {
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
		listThenCount(
			func(ctx context.Context, id string, limit, offset int) (interface{}, error) {
				return ous.ouStore.GetOrganizationUnitChildrenList(ctx, id, limit, offset, f)
			},
			func(ctx context.Context, id string) (int, error) {
				return ous.ouStore.GetOrganizationUnitChildrenCount(ctx, id, f)
			},
		),
		true,
	)
	if svcErr != nil {
		return nil, svcErr
	}
	base := fmt.Sprintf("/organization-units/%s/ous", id)
	list, svcErr := buildOrganizationUnitListResponse(base, items, totalCount, limit, offset)
	if list != nil {
		ous.markManaged(ctx, list.OrganizationUnits)
	}
	return list, svcErr
}

// GetOrganizationUnitChildrenByPath retrieves a list of child organization units by hierarchical handle path.
func (ous *organizationUnitService) GetOrganizationUnitChildrenByPath(
	ctx context.Context, handlePath string, limit, offset int, f *tidcommon.FilterGroup,
) (*OrganizationUnitListResponse, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameService))
	logger.Debug(ctx, "Getting organization unit children by path", log.String("path", handlePath))

	handles, serviceError := validateAndProcessHandlePath(handlePath)
	if serviceError != nil {
		return nil, serviceError
	}

	ou, err := ous.ouStore.GetOrganizationUnitByPath(ctx, handles)
	if err != nil {
		if errors.Is(err, ErrOrganizationUnitNotFound) {
			return nil, &ErrorOrganizationUnitNotFound
		}
		logger.Error(ctx, "Failed to get organization unit by path", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	return ous.GetOrganizationUnitChildren(ctx, ou.ID, limit, offset, f)
}

// GetOrganizationUnitUsersByPath retrieves a list of users by hierarchical handle path.
func (ous *organizationUnitService) GetOrganizationUnitUsersByPath(
	ctx context.Context, handlePath string, limit, offset int, includeDisplay bool,
) (*UserListResponse, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameService))
	logger.Debug(ctx, "Getting organization unit users by path", log.String("path", handlePath))

	handles, serviceError := validateAndProcessHandlePath(handlePath)
	if serviceError != nil {
		return nil, serviceError
	}

	if ous.userResolver == nil {
		return nil, &tidcommon.InternalServerError
	}

	ou, err := ous.ouStore.GetOrganizationUnitByPath(ctx, handles)
	if err != nil {
		if errors.Is(err, ErrOrganizationUnitNotFound) {
			return nil, &ErrorOrganizationUnitNotFound
		}
		logger.Error(ctx, "Failed to get organization unit by path", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	if svcErr := ous.checkOUAccess(ctx, security.ActionReadUser, ou.ID); svcErr != nil {
		return nil, svcErr
	}

	items, totalCount, svcErr := ous.getResourceListWithExistenceCheck(
		ctx, ou.ID, limit, offset, "users",
		func(ctx context.Context, id string, limit, offset int) (interface{}, int, error) {
			return ous.userResolver.GetUserListByOUID(ctx, id, limit, offset, includeDisplay)
		},
		false,
	)
	if svcErr != nil {
		return nil, svcErr
	}

	users, ok := items.([]User)
	if !ok {
		logger.Error(ctx, "Failed to cast user list response for organization unit", log.String("ouPath", handlePath))
		return nil, &tidcommon.InternalServerError
	}

	base := fmt.Sprintf("/organization-units/tree/%s/users", handlePath)
	return buildUserListResponse(base, users, totalCount, limit, offset, includeDisplay)
}

// GetOrganizationUnitGroupsByPath retrieves a list of groups by hierarchical handle path.
func (ous *organizationUnitService) GetOrganizationUnitGroupsByPath(
	ctx context.Context, handlePath string, limit, offset int,
) (*GroupListResponse, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameService))
	logger.Debug(ctx, "Getting organization unit groups by path", log.String("path", handlePath))

	handles, serviceError := validateAndProcessHandlePath(handlePath)
	if serviceError != nil {
		return nil, serviceError
	}

	if ous.groupResolver == nil {
		return nil, &tidcommon.InternalServerError
	}

	ou, err := ous.ouStore.GetOrganizationUnitByPath(ctx, handles)
	if err != nil {
		if errors.Is(err, ErrOrganizationUnitNotFound) {
			return nil, &ErrorOrganizationUnitNotFound
		}
		logger.Error(ctx, "Failed to get organization unit by path", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	if svcErr := ous.checkOUAccess(ctx, security.ActionReadGroup, ou.ID); svcErr != nil {
		return nil, svcErr
	}

	items, totalCount, svcErr := ous.getResourceListWithExistenceCheck(
		ctx, ou.ID, limit, offset, "groups",
		listThenCount(
			func(ctx context.Context, id string, limit, offset int) (interface{}, error) {
				return ous.groupResolver.GetGroupListByOUID(ctx, id, limit, offset)
			},
			ous.groupResolver.GetGroupCountByOUID,
		),
		false,
	)
	if svcErr != nil {
		return nil, svcErr
	}

	base := fmt.Sprintf("/organization-units/tree/%s/groups", handlePath)
	return buildGroupListResponse(base, items, totalCount, limit, offset)
}

// GetOrganizationUnitRolesByPath retrieves a list of roles by hierarchical handle path.
func (ous *organizationUnitService) GetOrganizationUnitRolesByPath(
	ctx context.Context, handlePath string, limit, offset int,
) (*RoleListResponse, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameService))
	logger.Debug(ctx, "Getting organization unit roles by path", log.String("path", handlePath))

	handles, serviceError := validateAndProcessHandlePath(handlePath)
	if serviceError != nil {
		return nil, serviceError
	}

	if ous.roleResolver == nil {
		return nil, &tidcommon.InternalServerError
	}

	ou, err := ous.ouStore.GetOrganizationUnitByPath(ctx, handles)
	if err != nil {
		if errors.Is(err, ErrOrganizationUnitNotFound) {
			return nil, &ErrorOrganizationUnitNotFound
		}
		logger.Error(ctx, "Failed to get organization unit by path", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	if svcErr := ous.checkOUAccess(ctx, security.ActionReadRole, ou.ID); svcErr != nil {
		return nil, svcErr
	}

	items, totalCount, svcErr := ous.getResourceListWithExistenceCheck(
		ctx, ou.ID, limit, offset, "roles",
		func(ctx context.Context, id string, limit, offset int) (interface{}, int, error) {
			return ous.roleResolver.GetRoleListByOUID(ctx, id, limit, offset)
		},
		false,
	)
	if svcErr != nil {
		return nil, svcErr
	}
	base := fmt.Sprintf("/organization-units/tree/%s/roles", handlePath)
	return buildRoleListResponse(base, items, totalCount, limit, offset)
}

// checkCircularDependency checks if setting the parent would create a circular dependency.
func (ous *organizationUnitService) checkCircularDependency(
	ctx context.Context, ouID string, parentID *string,
) *tidcommon.ServiceError {
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
			return &tidcommon.InternalServerError
		}

		currentParentID = parentOU.Parent
	}

	return nil
}

// validateOUName validates organization unit name.
func (ous *organizationUnitService) validateOUName(name string) *tidcommon.ServiceError {
	if strings.TrimSpace(name) == "" {
		return &ErrorInvalidRequestFormat
	}

	return nil
}

// validateOUHandle validates organization unit handle.
func (ous *organizationUnitService) validateOUHandle(handle string) *tidcommon.ServiceError {
	trimmed := strings.TrimSpace(handle)
	if trimmed == "" {
		return &ErrorInvalidRequestFormat
	}

	if strings.Contains(trimmed, "/") {
		return &ErrorInvalidRequestFormat
	}

	return nil
}

// ResolveOwner returns the id of the organization unit that owns a resource. An id is used as given, and a handle is
// resolved to the id of the unit it names; when both are given the id wins. The unit has to exist. Only a lookup that
// finds no such unit is reported as errs.NotFound, so a failed or refused lookup is not taken for one.
//
// A handle is resolved with the runtime context: it comes from an import or a declarative file, which name the owner
// by handle on behalf of the deployment and not of whoever runs the import.
func (ous *organizationUnitService) ResolveOwner(
	ctx context.Context, ouID, ouHandle string, errs OwnerErrors,
) (string, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameService))
	if ouID != "" && ouHandle != "" {
		logger.Warn(ctx, "Both ouId and ouHandle provided for a resource; ouHandle ignored")
	}

	if ouID == "" {
		if ouHandle == "" {
			return "", &errs.Required
		}
		unit, svcErr := ous.GetOrganizationUnitByPath(security.WithRuntimeContext(ctx), ouHandle)
		if svcErr != nil {
			if svcErr.Code == ErrorOrganizationUnitNotFound.Code {
				return "", &errs.NotFound
			}
			logger.Error(ctx, "Failed to look up the owner organization unit by handle",
				log.String("errorCode", svcErr.Code))
			return "", &tidcommon.InternalServerError
		}
		return unit.ID, nil
	}

	exists, svcErr := ous.IsOrganizationUnitExists(ctx, ouID)
	if svcErr != nil {
		logger.Error(ctx, "Failed to look up the owner organization unit", log.String("errorCode", svcErr.Code))
		return "", &tidcommon.InternalServerError
	}
	if !exists {
		return "", &errs.NotFound
	}
	return ouID, nil
}

// ResolveOwnerForUpdate returns the owner an updated resource has. An owner cannot change once the resource is
// created, so a request that leaves the owner out keeps the current one, and one that names an owner has to name the
// one the resource has, by id or by handle.
func (ous *organizationUnitService) ResolveOwnerForUpdate(
	ctx context.Context, requestedID, requestedHandle, currentID string, errs OwnerErrors,
) (string, *tidcommon.ServiceError) {
	if requestedID == "" && requestedHandle == "" {
		return currentID, nil
	}
	resolved := requestedID
	if resolved == "" {
		var svcErr *tidcommon.ServiceError
		if resolved, svcErr = ous.ResolveOwner(ctx, "", requestedHandle, errs); svcErr != nil {
			return "", svcErr
		}
	}
	if resolved != currentID {
		return "", &errs.Changed
	}
	return currentID, nil
}

func validateAndProcessHandlePath(handlePath string) ([]string, *tidcommon.ServiceError) {
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
func validatePaginationParams(limit, offset int) *tidcommon.ServiceError {
	if limit < 1 || limit > serverconst.MaxPageSize {
		return &ErrorInvalidLimit
	}
	if offset < 0 {
		return &ErrorInvalidOffset
	}
	return nil
}

// getResourceListWithExistenceCheck is a generic function to get one page of resources for an
// organization unit with existence check. getPageFunc returns the page and the total across every page.
// If mapCompositeError is true, it will map ErrResultLimitExceededInCompositeMode to ErrorResultLimitExceeded.
func (ous *organizationUnitService) getResourceListWithExistenceCheck(
	ctx context.Context, id string, limit, offset int, resourceType string,
	getPageFunc func(context.Context, string, int, int) (interface{}, int, error),
	mapCompositeError bool,
) (interface{}, int, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameService))
	logger.Debug(ctx, "Getting resource for organization unit", log.String("resource_type", resourceType),
		log.String("ouID", id))

	if err := validatePaginationParams(limit, offset); err != nil {
		return nil, 0, err
	}

	// Check if the organization unit exists
	exists, err := ous.ouStore.IsOrganizationUnitExists(ctx, id)
	if err != nil {
		logger.Error(ctx, "Failed to check organization unit existence", log.Error(err))
		return nil, 0, &tidcommon.InternalServerError
	}
	if !exists {
		return nil, 0, &ErrorOrganizationUnitNotFound
	}

	items, totalCount, err := getPageFunc(ctx, id, limit, offset)
	if err != nil {
		// Map composite limit error if requested
		if mapCompositeError && errors.Is(err, ErrResultLimitExceededInCompositeMode) {
			return nil, 0, &ErrorResultLimitExceeded
		}
		logger.Error(ctx, "Failed to list resource",
			log.String("resource_type", resourceType), log.Error(err))
		return nil, 0, &tidcommon.InternalServerError
	}

	return items, totalCount, nil
}

// listThenCount adapts a list function and a count function to the page function
// getResourceListWithExistenceCheck takes.
func listThenCount(
	getListFunc func(context.Context, string, int, int) (interface{}, error),
	getCountFunc func(context.Context, string) (int, error),
) func(context.Context, string, int, int) (interface{}, int, error) {
	return func(ctx context.Context, id string, limit, offset int) (interface{}, int, error) {
		items, err := getListFunc(ctx, id, limit, offset)
		if err != nil {
			return nil, 0, err
		}
		totalCount, err := getCountFunc(ctx, id)
		if err != nil {
			return nil, 0, err
		}
		return items, totalCount, nil
	}
}

func buildUserListResponse(
	base string, users []User, totalCount, limit, offset int, includeDisplay bool,
) (*UserListResponse, *tidcommon.ServiceError) {
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
) (*GroupListResponse, *tidcommon.ServiceError) {
	groups, ok := items.([]Group)
	if !ok {
		return nil, &tidcommon.InternalServerError
	}
	return &GroupListResponse{
		TotalResults: totalCount,
		Groups:       groups,
		StartIndex:   offset + 1,
		Count:        len(groups),
		Links:        utils.BuildPaginationLinks(base, limit, offset, totalCount, ""),
	}, nil
}

func buildRoleListResponse(
	base string, items interface{}, totalCount, limit, offset int,
) (*RoleListResponse, *tidcommon.ServiceError) {
	roles, ok := items.([]Role)
	if !ok {
		return nil, &tidcommon.InternalServerError
	}
	return &RoleListResponse{
		TotalResults: totalCount,
		Roles:        roles,
		StartIndex:   offset + 1,
		Count:        len(roles),
		Links:        utils.BuildPaginationLinks(base, limit, offset, totalCount, ""),
	}, nil
}

func buildOrganizationUnitListResponse(
	base string, items interface{}, totalCount, limit, offset int,
) (*OrganizationUnitListResponse, *tidcommon.ServiceError) {
	children, ok := items.([]OrganizationUnitBasic)
	if !ok {
		return nil, &tidcommon.InternalServerError
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
) (map[string]string, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameService))

	if len(ids) == 0 {
		return map[string]string{}, nil
	}

	ouBasics, err := ous.ouStore.GetOrganizationUnitsByIDs(ctx, ids)
	if err != nil {
		logger.Error(ctx, "Failed to get organization units by IDs", log.Error(err))
		return nil, &tidcommon.InternalServerError
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

// validateNewOUProject checks the project of a new organization unit. The system organization unit belongs to
// no project. Any other unit belongs to a project on a deployment that requires one, and a unit in a project
// has its parent, when it has one, in the same project or is placed directly under the system organization
// unit, which every project shares.
func (ous *organizationUnitService) validateNewOUProject(
	ctx context.Context, request OrganizationUnitRequestWithID,
) *tidcommon.ServiceError {
	if request.ID != "" && request.ID == configuredSystemOUID() {
		if request.ProjectID != "" {
			return &ErrorSystemOUInProject
		}
		return nil
	}
	if request.ProjectID == "" && ous.projectRequired {
		return &ErrorProjectRequired
	}
	return ous.validateOUProject(ctx, request.ProjectID, request.Parent)
}

// resolveUpdatedOUProject returns the project of an updated organization unit. An update that names no project
// keeps the unit's project. A unit that changes project or parent is checked again.
func (ous *organizationUnitService) resolveUpdatedOUProject(
	ctx context.Context, existingOU OrganizationUnit, request OrganizationUnitRequestWithID, parentChanged bool,
) (string, *tidcommon.ServiceError) {
	if existingOU.ID == configuredSystemOUID() {
		if request.ProjectID != "" {
			return "", &ErrorSystemOUInProject
		}
		return "", nil
	}

	projectID := request.ProjectID
	if projectID == "" {
		projectID = existingOU.ProjectID
	}
	projectChanged := projectID != existingOU.ProjectID
	if !projectChanged && !parentChanged {
		return projectID, nil
	}
	if svcErr := ous.validateOUProject(ctx, projectID, request.Parent); svcErr != nil {
		return "", svcErr
	}
	return projectID, nil
}

// moveDescendantsToProject moves every organization unit beneath ouID to the project its ancestor moves to, so
// that a tree never spans projects. Every descendant is checked before any is written, and the store drops the
// moved units from its caches.
func (ous *organizationUnitService) moveDescendantsToProject(
	ctx context.Context, ouID, projectID string, updatedAt time.Time, logger *log.Logger,
) *tidcommon.ServiceError {
	descendants, svcErr := ous.hierarchy.DescendantOUIDs(ctx, ouID)
	if svcErr != nil {
		return svcErr
	}
	if len(descendants) == 0 {
		return nil
	}
	for _, id := range descendants {
		if svcErr := ous.refuseChange(ctx, id); svcErr != nil {
			return svcErr
		}
	}
	if err := ous.ouStore.UpdateOrganizationUnitsProject(ctx, descendants, projectID, updatedAt); err != nil {
		logger.Error(ctx, "Failed to move descendant organization units to another project",
			log.String("ouID", ouID), log.Error(err))
		return &tidcommon.InternalServerError
	}
	return nil
}

// validateOUProject checks that the project an organization unit names exists and holds the unit's parent. The
// system organization unit belongs to no project and every project shares it, so it may be the parent of a unit
// in any project.
//
// A deployment that serves no projects, such as a Data Plane, cannot check the project. It keeps the project ID
// as given so that imported configuration retains it.
func (ous *organizationUnitService) validateOUProject(
	ctx context.Context, projectID string, parent *string,
) *tidcommon.ServiceError {
	// The column holds 36 characters, so a longer ID is refused on every deployment, checked or not.
	if utf8.RuneCountInString(projectID) > maxProjectIDLength {
		return &ErrorInvalidProject
	}
	if ous.projectChecker == nil {
		return nil
	}
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameService))
	if projectID != "" {
		exists, svcErr := ous.projectChecker.ProjectExists(ctx, projectID)
		if svcErr != nil {
			logger.Error(ctx, "Failed to check the project of an organization unit", log.String("code", svcErr.Code))
			return &tidcommon.InternalServerError
		}
		if !exists {
			return &ErrorInvalidProject
		}
	}
	if parent == nil {
		return nil
	}
	parentOU, err := ous.ouStore.GetOrganizationUnit(ctx, *parent)
	if err != nil {
		if errors.Is(err, ErrOrganizationUnitNotFound) {
			return &ErrorParentOrganizationUnitNotFound
		}
		logger.Error(ctx, "Failed to get parent organization unit", log.Error(err))
		return &tidcommon.InternalServerError
	}
	if parentOU.ProjectID != projectID && parentOU.ID != configuredSystemOUID() {
		return &ErrorParentInAnotherProject
	}
	return nil
}

// configuredSystemOUID returns the ID of the system organization unit the deployment configuration names,
// or an empty string when none is configured.
func configuredSystemOUID() string {
	if !config.IsServerRuntimeInitialized() {
		return ""
	}
	return config.GetServerRuntime().Config.Server.SystemOUID
}
