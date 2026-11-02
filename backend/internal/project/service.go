// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package project manages projects. A project groups identity configuration under exactly one root
// organization unit: resources keep referencing organization units, and belong to a project through
// the root organization unit's subtree.
package project

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/thunder-id/thunderid/internal/ou"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const (
	// maxDisplayNameLength matches the name length of the root organization unit that backs a project.
	maxDisplayNameLength = 100
	// maxDescriptionLength matches the description length of the root organization unit.
	maxDescriptionLength = 255
)

// ProjectServiceInterface manages projects and reports the root organization units they hold.
type ProjectServiceInterface interface {
	CreateProject(ctx context.Context, request ProjectRequest) (*Project, *tidcommon.ServiceError)
	GetProject(ctx context.Context, id string) (*Project, *tidcommon.ServiceError)
	GetProjectList(ctx context.Context, limit, offset int) (*ProjectListResponse, *tidcommon.ServiceError)
	UpdateProject(ctx context.Context, id string, request ProjectRequest) (*Project, *tidcommon.ServiceError)
	DeleteProject(ctx context.Context, id string) *tidcommon.ServiceError
	ResolveProjectOU(ctx context.Context, projectID, ouID string) (string, *tidcommon.ServiceError)
	ProjectExists(ctx context.Context, id string) (bool, *tidcommon.ServiceError)
	ImportProject(ctx context.Context, id, rootOUID, handle, displayName, description string, upsert bool) (
		bool, *tidcommon.ServiceError)
	GetResourceDependencies(
		ctx context.Context, resourceType, id string) ([]resourcedependency.ResourceDependency, error)
}

// projectService is the default implementation of ProjectServiceInterface.
type projectService struct {
	store         projectStoreInterface
	transactioner providers.Transactioner
	ouService     ou.OrganizationUnitServiceInterface
	logger        *log.Logger
}

// newProjectService creates a project service.
func newProjectService(store projectStoreInterface, transactioner providers.Transactioner,
	ouService ou.OrganizationUnitServiceInterface) *projectService {
	return &projectService{
		store:         store,
		transactioner: transactioner,
		ouService:     ouService,
		logger:        log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ProjectService")),
	}
}

// CreateProject creates a project together with the root organization unit that backs it. The
// organization unit takes the request's handle, display name and description.
func (s *projectService) CreateProject(
	ctx context.Context, request ProjectRequest) (*Project, *tidcommon.ServiceError) {
	if request.ID != "" && !utils.IsValidUUID(request.ID) {
		return nil, &ErrorInvalidProjectID
	}
	if svcErr := validateDetails(request); svcErr != nil {
		return nil, svcErr
	}

	return s.createProject(ctx, request.ID, ou.OrganizationUnitRequestWithID{
		Handle:      request.Handle,
		Name:        request.DisplayName,
		Description: request.Description,
	})
}

// GetProject returns the project with the given ID.
func (s *projectService) GetProject(ctx context.Context, id string) (*Project, *tidcommon.ServiceError) {
	project, err := s.store.GetProject(ctx, id)
	if err != nil {
		return nil, s.mapStoreError(ctx, err, "Failed to get project")
	}
	return s.buildProject(ctx, project)
}

// ResolveProjectOU returns the organization unit that a resource scoped to the project belongs to: the
// project's root organization unit when ouID is empty, or ouID when it is the root organization unit or
// one below it. It returns ErrorProjectNotFound when the project does not exist, ErrorOUNotInProject
// when the organization unit is outside the project, and the organization unit service's error when
// the organization unit does not exist.
func (s *projectService) ResolveProjectOU(
	ctx context.Context, projectID, ouID string) (string, *tidcommon.ServiceError) {
	project, err := s.store.GetProject(ctx, projectID)
	if err != nil {
		return "", s.mapStoreError(ctx, err, "Failed to get project")
	}
	if ouID == "" {
		return project.RootOUID, nil
	}
	inProject, svcErr := s.ouService.IsParent(ctx, project.RootOUID, ouID)
	if svcErr != nil {
		return "", svcErr
	}
	if !inProject {
		return "", &ErrorOUNotInProject
	}
	return ouID, nil
}

// GetProjectList returns a page of projects, oldest first.
func (s *projectService) GetProjectList(
	ctx context.Context, limit, offset int) (*ProjectListResponse, *tidcommon.ServiceError) {
	if limit < 1 || limit > serverconst.MaxPageSize {
		return nil, &ErrorInvalidLimit
	}
	if offset < 0 {
		return nil, &ErrorInvalidOffset
	}

	total, err := s.store.GetProjectListCount(ctx)
	if err != nil {
		s.logger.Error(ctx, "Failed to count projects", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	records, err := s.store.GetProjectList(ctx, limit, offset)
	if err != nil {
		s.logger.Error(ctx, "Failed to list projects", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	projects := make([]Project, 0, len(records))
	for _, record := range records {
		project, svcErr := s.buildProject(ctx, record)
		if svcErr != nil {
			return nil, svcErr
		}
		projects = append(projects, *project)
	}

	return &ProjectListResponse{
		TotalResults: total,
		StartIndex:   offset + 1,
		Count:        len(projects),
		Projects:     projects,
		Links:        utils.BuildPaginationLinks(projectsPath, limit, offset, total, ""),
	}, nil
}

// UpdateProject updates the display name and description of a project, which are its root
// organization unit's name and description. The handle cannot change.
func (s *projectService) UpdateProject(
	ctx context.Context, id string, request ProjectRequest) (*Project, *tidcommon.ServiceError) {
	if svcErr := validateDetails(request); svcErr != nil {
		return nil, svcErr
	}

	project, err := s.store.GetProject(ctx, id)
	if err != nil {
		return nil, s.mapStoreError(ctx, err, "Failed to get project")
	}
	rootOU, svcErr := s.ouService.GetOrganizationUnit(ctx, project.RootOUID)
	if svcErr != nil {
		return nil, svcErr
	}
	if request.Handle != "" && request.Handle != rootOU.Handle {
		return nil, &ErrorHandleChangeNotAllowed
	}

	updated, svcErr := s.ouService.UpdateOrganizationUnit(ctx, project.RootOUID,
		rootOUUpdateRequest(rootOU, request.DisplayName, request.Description))
	if svcErr != nil {
		return nil, svcErr
	}
	return toProject(project, updated), nil
}

// DeleteProject deletes a project and its root organization unit. It is refused while the root
// organization unit still holds child organization units or other resources. Deleting a project
// that does not exist succeeds.
func (s *projectService) DeleteProject(ctx context.Context, id string) *tidcommon.ServiceError {
	var capturedSvcErr *tidcommon.ServiceError

	err := s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		project, err := s.store.GetProject(txCtx, id)
		if err != nil {
			if errors.Is(err, errProjectNotFound) {
				return nil
			}
			return err
		}

		// The project row goes first so that it no longer blocks deletion of its root organization unit.
		if err := s.store.DeleteProject(txCtx, id); err != nil {
			return err
		}
		if svcErr := s.ouService.DeleteOrganizationUnit(txCtx, project.RootOUID); svcErr != nil {
			capturedSvcErr = svcErr
			return errors.New("root organization unit deletion failed")
		}
		return nil
	})

	if capturedSvcErr != nil {
		return capturedSvcErr
	}
	if err != nil {
		s.logger.Error(ctx, "Failed to delete project", log.Error(err))
		return &tidcommon.InternalServerError
	}
	return nil
}

// GetResourceDependencies implements resourcedependency.Provider. It reports the project an
// organization unit backs, which blocks deletion of that organization unit outside the project API.
func (s *projectService) GetResourceDependencies(
	ctx context.Context, resourceType, id string) ([]resourcedependency.ResourceDependency, error) {
	if resourceType != resourcedependency.ResourceTypeOU {
		return []resourcedependency.ResourceDependency{}, nil
	}

	project, err := s.store.GetProjectByRootOUID(ctx, id)
	if err != nil {
		if errors.Is(err, errProjectNotFound) {
			return []resourcedependency.ResourceDependency{}, nil
		}
		return nil, err
	}
	return []resourcedependency.ResourceDependency{{
		ResourceType:     resourcedependency.ResourceTypeProject,
		ID:               project.ID,
		DisplayName:      project.ID,
		BehaviorOnDelete: resourcedependency.BehaviorRestrict,
	}}, nil
}

// ProjectExists reports whether a project with the given ID exists.
func (s *projectService) ProjectExists(ctx context.Context, id string) (bool, *tidcommon.ServiceError) {
	_, err := s.store.GetProject(ctx, id)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, errProjectNotFound) {
		return false, nil
	}
	s.logger.Error(ctx, "Failed to get project", log.Error(err))
	return false, &tidcommon.InternalServerError
}

// ImportProject brings in a declared project with the ID another system gave it. A project that
// does not exist is created with its root organization unit, or linked to that organization unit
// when it already exists as a root without a project. With upsert, an existing project gets the
// declared display name and description; its root organization unit and handle cannot change. It
// reports whether it created the project, or tried to, rather than updating an existing one.
func (s *projectService) ImportProject(ctx context.Context, id, rootOUID, handle, displayName,
	description string, upsert bool) (bool, *tidcommon.ServiceError) {
	if !utils.IsValidUUID(id) {
		return false, &ErrorInvalidProjectID
	}
	details := ProjectRequest{Handle: handle, DisplayName: displayName, Description: description}
	if svcErr := validateDetails(details); svcErr != nil {
		return false, svcErr
	}

	existing, err := s.store.GetProject(ctx, id)
	if err == nil {
		if !upsert {
			return false, &ErrorProjectIDConflict
		}
		if rootOUID != "" && rootOUID != existing.RootOUID {
			return false, &ErrorRootOUMismatch
		}
		_, svcErr := s.UpdateProject(ctx, id, details)
		return false, svcErr
	}
	if !errors.Is(err, errProjectNotFound) {
		s.logger.Error(ctx, "Failed to get project", log.Error(err))
		return false, &tidcommon.InternalServerError
	}

	if rootOUID != "" {
		rootOU, svcErr := s.ouService.GetOrganizationUnit(ctx, rootOUID)
		if svcErr == nil {
			return true, s.linkProject(ctx, id, rootOU, details)
		}
		if svcErr.Code != ou.ErrorOrganizationUnitNotFound.Code {
			return false, svcErr
		}
	}

	_, svcErr := s.createProject(ctx, id, ou.OrganizationUnitRequestWithID{
		ID: rootOUID, Handle: handle, Name: displayName, Description: description,
	})
	return true, svcErr
}

// linkProject makes an existing root organization unit the root of a new project with the given ID,
// and gives the organization unit the project's display name and description.
func (s *projectService) linkProject(ctx context.Context, id string, rootOU ou.OrganizationUnit,
	details ProjectRequest) *tidcommon.ServiceError {
	if rootOU.Parent != nil && *rootOU.Parent != "" {
		return &ErrorNotARootOU
	}
	if details.Handle != "" && details.Handle != rootOU.Handle {
		return &ErrorHandleChangeNotAllowed
	}
	if _, err := s.store.GetProjectByRootOUID(ctx, rootOU.ID); err == nil {
		return &ErrorRootOUInUse
	} else if !errors.Is(err, errProjectNotFound) {
		s.logger.Error(ctx, "Failed to get project by root organization unit", log.Error(err))
		return &tidcommon.InternalServerError
	}

	var capturedSvcErr *tidcommon.ServiceError
	err := s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		if err := s.store.CreateProject(txCtx, projectRecord{
			ID: id, RootOUID: rootOU.ID, CreatedAt: time.Now().UTC().Truncate(time.Microsecond),
		}); err != nil {
			return err
		}
		if rootOU.Name == details.DisplayName && rootOU.Description == details.Description {
			return nil
		}
		if _, svcErr := s.ouService.UpdateOrganizationUnit(txCtx, rootOU.ID,
			rootOUUpdateRequest(rootOU, details.DisplayName, details.Description)); svcErr != nil {
			capturedSvcErr = svcErr
			return errors.New("root organization unit update failed")
		}
		return nil
	})
	if capturedSvcErr != nil {
		return capturedSvcErr
	}
	if err != nil {
		s.logger.Error(ctx, "Failed to link project to its root organization unit", log.Error(err))
		return &tidcommon.InternalServerError
	}
	return nil
}

// createProject creates the root organization unit and the project record in one transaction. An
// empty projectID gets a generated one. The organization unit request has no parent, so the
// organization unit is created as a root.
func (s *projectService) createProject(ctx context.Context, projectID string,
	rootOU ou.OrganizationUnitRequestWithID) (*Project, *tidcommon.ServiceError) {
	var created *Project
	var capturedSvcErr *tidcommon.ServiceError

	err := s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		if projectID != "" {
			_, err := s.store.GetProject(txCtx, projectID)
			if err == nil {
				capturedSvcErr = &ErrorProjectIDConflict
				return errors.New("project ID conflict")
			}
			if !errors.Is(err, errProjectNotFound) {
				return err
			}
		} else {
			id, err := utils.GenerateUUIDv7()
			if err != nil {
				return err
			}
			projectID = id
		}

		createdOU, svcErr := s.ouService.CreateOrganizationUnit(txCtx, rootOU)
		if svcErr != nil {
			capturedSvcErr = svcErr
			return errors.New("root organization unit creation failed")
		}

		// Microseconds are the finest precision every supported database keeps, so the time returned
		// here is the time a later read returns.
		record := projectRecord{
			ID: projectID, RootOUID: createdOU.ID, CreatedAt: time.Now().UTC().Truncate(time.Microsecond),
		}
		if err := s.store.CreateProject(txCtx, record); err != nil {
			return err
		}
		created = toProject(record, createdOU)
		return nil
	})

	if capturedSvcErr != nil {
		return nil, capturedSvcErr
	}
	if err != nil {
		s.logger.Error(ctx, "Failed to create project", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	s.logger.Debug(ctx, "Created project", log.String("projectID", created.ID))
	return created, nil
}

// buildProject completes a project record with its root organization unit's details.
func (s *projectService) buildProject(ctx context.Context, record projectRecord) (*Project, *tidcommon.ServiceError) {
	rootOU, svcErr := s.ouService.GetOrganizationUnit(ctx, record.RootOUID)
	if svcErr != nil {
		return nil, svcErr
	}
	return toProject(record, rootOU), nil
}

// mapStoreError maps a store error to the project service error, logging a server failure.
func (s *projectService) mapStoreError(ctx context.Context, err error, message string) *tidcommon.ServiceError {
	if errors.Is(err, errProjectNotFound) {
		return &ErrorProjectNotFound
	}
	s.logger.Error(ctx, message, log.Error(err))
	return &tidcommon.InternalServerError
}

// toProject builds a project from its record and its root organization unit.
func toProject(record projectRecord, rootOU ou.OrganizationUnit) *Project {
	return &Project{
		ID:          record.ID,
		Handle:      rootOU.Handle,
		DisplayName: rootOU.Name,
		Description: rootOU.Description,
		RootOUID:    record.RootOUID,
		CreatedAt:   record.CreatedAt,
	}
}

// rootOUUpdateRequest copies a root organization unit into an update request with a new name and
// description, leaving every other field as it is.
func rootOUUpdateRequest(rootOU ou.OrganizationUnit, name, description string) ou.OrganizationUnitRequestWithID {
	return ou.OrganizationUnitRequestWithID{
		ID:                        rootOU.ID,
		Handle:                    rootOU.Handle,
		Name:                      name,
		Description:               description,
		Parent:                    rootOU.Parent,
		ThemeID:                   rootOU.ThemeID,
		LayoutID:                  rootOU.LayoutID,
		AuthFlowID:                rootOU.AuthFlowID,
		RegistrationFlowID:        rootOU.RegistrationFlowID,
		IsRegistrationFlowEnabled: rootOU.IsRegistrationFlowEnabled,
		RecoveryFlowID:            rootOU.RecoveryFlowID,
		IsRecoveryFlowEnabled:     rootOU.IsRecoveryFlowEnabled,
		SignOutFlowID:             rootOU.SignOutFlowID,
		UserOnboardingFlowID:      rootOU.UserOnboardingFlowID,
		LogoURL:                   rootOU.LogoURL,
		TosURI:                    rootOU.TosURI,
		PolicyURI:                 rootOU.PolicyURI,
		CookiePolicyURI:           rootOU.CookiePolicyURI,
	}
}

// validateDetails validates the display name and description of a project request.
func validateDetails(request ProjectRequest) *tidcommon.ServiceError {
	if strings.TrimSpace(request.DisplayName) == "" ||
		utf8.RuneCountInString(request.DisplayName) > maxDisplayNameLength {
		return &ErrorInvalidDisplayName
	}
	if utf8.RuneCountInString(request.Description) > maxDescriptionLength {
		return &ErrorInvalidDescription
	}
	return nil
}
