// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package project manages projects. A project groups identity configuration in the default
// organization unit, and its resources carry the project ID.
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
	// maxDisplayNameLength is the longest display name a project may have.
	maxDisplayNameLength = 100
	// maxDescriptionLength is the longest description a project may have.
	maxDescriptionLength = 255
)

// ProjectServiceInterface manages projects and resolves the organization units they include.
type ProjectServiceInterface interface {
	CreateProject(ctx context.Context, request ProjectRequest) (*Project, *tidcommon.ServiceError)
	GetProject(ctx context.Context, id string) (*Project, *tidcommon.ServiceError)
	GetProjectList(ctx context.Context, limit, offset int) (*ProjectListResponse, *tidcommon.ServiceError)
	UpdateProject(ctx context.Context, id string, request ProjectRequest) (*Project, *tidcommon.ServiceError)
	DeleteProject(ctx context.Context, id string) *tidcommon.ServiceError
	ResolveProjectOU(ctx context.Context, projectID, ouID string) (string, *tidcommon.ServiceError)
	ProjectExists(ctx context.Context, id string) (bool, *tidcommon.ServiceError)
	ImportProject(ctx context.Context, id, handle, displayName, description string, upsert bool) (
		bool, *tidcommon.ServiceError)
	GetResourceDependencies(
		ctx context.Context, resourceType, id string) ([]resourcedependency.ResourceDependency, error)
}

// projectEntityReader reports whether any entity, such as an application or an agent, is in a project.
type projectEntityReader interface {
	ProjectHasEntities(ctx context.Context, projectID string) (bool, error)
}

// projectService is the default implementation of ProjectServiceInterface.
type projectService struct {
	store         projectStoreInterface
	transactioner providers.Transactioner
	ouService     ou.OrganizationUnitServiceInterface
	entities      projectEntityReader
	logger        *log.Logger
}

// newProjectService creates a project service.
func newProjectService(store projectStoreInterface, transactioner providers.Transactioner,
	ouService ou.OrganizationUnitServiceInterface, entities projectEntityReader) *projectService {
	return &projectService{
		store:         store,
		transactioner: transactioner,
		ouService:     ouService,
		entities:      entities,
		logger:        log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ProjectService")),
	}
}

// CreateProject creates a project. An empty ID in the request gets a generated one.
func (s *projectService) CreateProject(
	ctx context.Context, request ProjectRequest) (*Project, *tidcommon.ServiceError) {
	if request.ID != "" && !utils.IsValidUUID(request.ID) {
		return nil, &ErrorInvalidProjectID
	}
	if svcErr := validateDetails(request); svcErr != nil {
		return nil, svcErr
	}
	return s.createProject(ctx, request)
}

// GetProject returns the project with the given ID.
func (s *projectService) GetProject(ctx context.Context, id string) (*Project, *tidcommon.ServiceError) {
	project, err := s.store.GetProject(ctx, id)
	if err != nil {
		return nil, s.mapStoreError(ctx, err, "Failed to get project")
	}
	return &project, nil
}

// ResolveProjectOU returns the organization unit that a resource scoped to the project belongs to: the
// system organization unit when ouID is empty, or ouID when it is the system organization unit or one
// below it. It returns ErrorProjectNotFound when the project does not exist, ErrorOUNotInProject when the
// organization unit is outside the project, and the organization unit service's error when the
// organization unit does not exist.
func (s *projectService) ResolveProjectOU(
	ctx context.Context, projectID, ouID string) (string, *tidcommon.ServiceError) {
	if _, err := s.store.GetProject(ctx, projectID); err != nil {
		return "", s.mapStoreError(ctx, err, "Failed to get project")
	}
	if ouID == "" {
		return ou.SystemOUID, nil
	}
	inProject, svcErr := s.ouService.IsParent(ctx, ou.SystemOUID, ouID)
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
	projects, err := s.store.GetProjectList(ctx, limit, offset)
	if err != nil {
		s.logger.Error(ctx, "Failed to list projects", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	return &ProjectListResponse{
		TotalResults: total,
		StartIndex:   offset + 1,
		Count:        len(projects),
		Projects:     projects,
		Links:        utils.BuildPaginationLinks(projectsPath, limit, offset, total, ""),
	}, nil
}

// UpdateProject updates the display name and description of a project. The handle cannot change.
func (s *projectService) UpdateProject(
	ctx context.Context, id string, request ProjectRequest) (*Project, *tidcommon.ServiceError) {
	if svcErr := validateDetails(request); svcErr != nil {
		return nil, svcErr
	}

	project, err := s.store.GetProject(ctx, id)
	if err != nil {
		return nil, s.mapStoreError(ctx, err, "Failed to get project")
	}
	if request.Handle != "" && request.Handle != project.Handle {
		return nil, &ErrorHandleChangeNotAllowed
	}

	project.DisplayName = request.DisplayName
	project.Description = request.Description
	if err := s.store.UpdateProject(ctx, project); err != nil {
		s.logger.Error(ctx, "Failed to update project", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	return &project, nil
}

// DeleteProject deletes a project. It is refused while an application or agent is still in the project.
// The check and the delete run in one transaction in the entity database. Deleting a project that does
// not exist succeeds.
func (s *projectService) DeleteProject(ctx context.Context, id string) *tidcommon.ServiceError {
	var capturedSvcErr *tidcommon.ServiceError
	err := s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		hasEntities, err := s.entities.ProjectHasEntities(txCtx, id)
		if err != nil {
			return err
		}
		if hasEntities {
			capturedSvcErr = &ErrorProjectNotEmpty
			return errors.New("project is not empty")
		}
		return s.store.DeleteProject(txCtx, id)
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

// GetResourceDependencies implements resourcedependency.Provider. Every project includes the system
// organization unit, so it reports one project while any exists, which blocks the deletion.
func (s *projectService) GetResourceDependencies(
	ctx context.Context, resourceType, id string) ([]resourcedependency.ResourceDependency, error) {
	if resourceType != resourcedependency.ResourceTypeOU || id != ou.SystemOUID {
		return []resourcedependency.ResourceDependency{}, nil
	}

	projects, err := s.store.GetProjectList(ctx, 1, 0)
	if err != nil {
		return nil, err
	}
	dependencies := make([]resourcedependency.ResourceDependency, 0, len(projects))
	for _, project := range projects {
		dependencies = append(dependencies, resourcedependency.ResourceDependency{
			ResourceType:     resourcedependency.ResourceTypeProject,
			ID:               project.ID,
			DisplayName:      project.DisplayName,
			BehaviorOnDelete: resourcedependency.BehaviorRestrict,
		})
	}
	return dependencies, nil
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

// ImportProject brings in a declared project with the ID another system gave it. A project that does
// not exist is created. With upsert, an existing project gets the declared display name and
// description; its handle cannot change. It reports whether it created the project, or tried to,
// rather than updating an existing one.
func (s *projectService) ImportProject(ctx context.Context, id, handle, displayName,
	description string, upsert bool) (bool, *tidcommon.ServiceError) {
	if !utils.IsValidUUID(id) {
		return false, &ErrorInvalidProjectID
	}
	request := ProjectRequest{ID: id, Handle: handle, DisplayName: displayName, Description: description}
	if svcErr := validateDetails(request); svcErr != nil {
		return false, svcErr
	}

	_, err := s.store.GetProject(ctx, id)
	if err == nil {
		if !upsert {
			return false, &ErrorProjectIDConflict
		}
		_, svcErr := s.UpdateProject(ctx, id, request)
		return false, svcErr
	}
	if !errors.Is(err, errProjectNotFound) {
		s.logger.Error(ctx, "Failed to get project", log.Error(err))
		return false, &tidcommon.InternalServerError
	}

	_, svcErr := s.createProject(ctx, request)
	return true, svcErr
}

// createProject validates the handle, checks that the project ID and handle are free, and stores the
// project in one transaction. An empty ID gets a generated one.
func (s *projectService) createProject(
	ctx context.Context, request ProjectRequest) (*Project, *tidcommon.ServiceError) {
	if !utils.IsValidHandle(request.Handle) {
		return nil, &ErrorInvalidHandle
	}

	var created *Project
	var capturedSvcErr *tidcommon.ServiceError

	err := s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		projectID := request.ID
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

		_, err := s.store.GetProjectByHandle(txCtx, request.Handle)
		if err == nil {
			capturedSvcErr = &ErrorHandleConflict
			return errors.New("project handle conflict")
		}
		if !errors.Is(err, errProjectNotFound) {
			return err
		}

		// Microseconds are the finest precision every supported database keeps, so the time returned
		// here is the time a later read returns.
		project := Project{
			ID:          projectID,
			Handle:      request.Handle,
			DisplayName: request.DisplayName,
			Description: request.Description,
			CreatedAt:   time.Now().UTC().Truncate(time.Microsecond),
		}
		// A concurrent create can take the ID or handle after the checks above; the unique indexes
		// still refuse it.
		if err := s.store.CreateProject(txCtx, project); err != nil {
			switch {
			case errors.Is(err, errProjectIDConflict):
				capturedSvcErr = &ErrorProjectIDConflict
			case errors.Is(err, errProjectHandleConflict):
				capturedSvcErr = &ErrorHandleConflict
			}
			return err
		}
		created = &project
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

// mapStoreError maps a store error to the project service error, logging a server failure.
func (s *projectService) mapStoreError(ctx context.Context, err error, message string) *tidcommon.ServiceError {
	if errors.Is(err, errProjectNotFound) {
		return &ErrorProjectNotFound
	}
	s.logger.Error(ctx, message, log.Error(err))
	return &tidcommon.InternalServerError
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
