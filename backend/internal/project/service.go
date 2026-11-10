// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package project manages projects. A project groups identity configuration, and its resources carry the
// project ID. They are placed in the system organization unit or in an organization unit of the project.
package project

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/thunder-id/thunderid/internal/ou"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/managedresource"
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
	ValidateProjectOU(ctx context.Context, projectID, ouID string) *tidcommon.ServiceError
	ProjectExists(ctx context.Context, id string) (bool, *tidcommon.ServiceError)
	ProjectNames(ctx context.Context) (map[string]string, *tidcommon.ServiceError)
	ImportProject(ctx context.Context, id, handle, displayName, description string, upsert bool) (
		bool, *tidcommon.ServiceError)
	AddResourceCheck(check ResourceCheck)
}

// ResourceCheck reports whether any resource of one type, such as a theme, still belongs to a project.
type ResourceCheck func(ctx context.Context, projectID string) (bool, *tidcommon.ServiceError)

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
	// resourceChecks cover the resources kept outside the entity store, which the delete
	// transaction cannot see.
	resourceChecks []ResourceCheck
	logger         *log.Logger
	bus            eventbus.EventBusInterface
	// managed says which projects a control plane deployed, which stay read only here. Nil where
	// nothing is deployed from one.
	managed managedresource.StoreInterface
}

// newProjectService creates a project service.
func newProjectService(store projectStoreInterface, transactioner providers.Transactioner,
	ouService ou.OrganizationUnitServiceInterface, entities projectEntityReader,
	bus eventbus.EventBusInterface) *projectService {
	return &projectService{
		store:         store,
		transactioner: transactioner,
		ouService:     ouService,
		entities:      entities,
		bus:           bus,
		logger:        log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ProjectService")),
	}
}

// CreateProject creates a project. An empty ID in the request gets a generated one.
func (s *projectService) CreateProject(
	ctx context.Context, request ProjectRequest) (*Project, *tidcommon.ServiceError) {
	created, svcErr := s.createRequestedProject(ctx, request)
	return s.publishCreate(ctx, created, svcErr)
}

// createRequestedProject creates a project without publishing its audit event.
func (s *projectService) createRequestedProject(
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

// ValidateProjectOU checks that a resource scoped to the project is in an organization unit the project
// includes: the system organization unit, or an organization unit that belongs to the project. It returns
// ErrorProjectNotFound when the project does not exist, ErrorOUNotInProject when the organization unit is
// outside the project, and a server error when the system organization unit is not configured or does not
// exist.
func (s *projectService) ValidateProjectOU(ctx context.Context, projectID, ouID string) *tidcommon.ServiceError {
	if _, err := s.store.GetProject(ctx, projectID); err != nil {
		return s.mapStoreError(ctx, err, "Failed to get project")
	}
	systemOUID, svcErr := s.ouService.GetSystemOUID(ctx)
	if svcErr != nil {
		return svcErr
	}
	if ouID == systemOUID {
		return nil
	}
	if ouID == "" {
		return &ErrorOUNotInProject
	}
	ouProjectID, svcErr := s.ouService.GetOrganizationUnitProjectID(ctx, ouID)
	if svcErr != nil {
		if svcErr.Code == ou.ErrorOrganizationUnitNotFound.Code {
			return &ErrorOUNotInProject
		}
		return svcErr
	}
	if ouProjectID != projectID {
		return &ErrorOUNotInProject
	}
	return nil
}

// ProjectNames maps each project's id to the name it is shown by: its display name, or its handle when
// it has none.
func (s *projectService) ProjectNames(ctx context.Context) (map[string]string, *tidcommon.ServiceError) {
	names := map[string]string{}
	for offset := 0; ; offset += serverconst.MaxPageSize {
		page, svcErr := s.GetProjectList(ctx, serverconst.MaxPageSize, offset)
		if svcErr != nil {
			return nil, svcErr
		}
		for _, found := range page.Projects {
			name := found.DisplayName
			if name == "" {
				name = found.Handle
			}
			names[found.ID] = name
		}
		if len(page.Projects) < serverconst.MaxPageSize {
			return names, nil
		}
	}
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
	updated, svcErr := s.updateProject(ctx, id, request)
	s.publishProjectEvent(ctx, audit.EventProjectUpdate, audit.ActionUpdate, id, svcErr)
	return updated, svcErr
}

// updateProject updates a project without publishing its audit event.
func (s *projectService) updateProject(
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
	if svcErr := s.refuseManaged(ctx, id); svcErr != nil {
		return nil, svcErr
	}

	project.DisplayName = request.DisplayName
	project.Description = request.Description
	if err := s.store.UpdateProject(ctx, project); err != nil {
		s.logger.Error(ctx, "Failed to update project", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	return &project, nil
}

// DeleteProject deletes a project. It is refused while an application or agent is still in the project,
// or while a registered resource check finds resources of another type in it. The application and agent
// check and the delete run in one transaction in the entity database. The resource checks run before that
// transaction: their resources are in other databases, which it cannot cover, so a resource created
// between a check and the delete keeps the ID of the deleted project. Deleting a project that does not
// exist succeeds.
func (s *projectService) DeleteProject(ctx context.Context, id string) *tidcommon.ServiceError {
	svcErr := s.deleteProject(ctx, id)
	// A delete that found nothing to remove succeeds without an event, so no caller can record a
	// deletion of an ID it made up.
	if svcErr == errProjectNotFoundNoop {
		return nil
	}
	s.publishProjectEvent(ctx, audit.EventProjectDelete, audit.ActionDelete, id, svcErr)
	return svcErr
}

// deleteProject deletes a project without publishing its audit event.
func (s *projectService) deleteProject(ctx context.Context, id string) *tidcommon.ServiceError {
	if svcErr := s.refuseManaged(ctx, id); svcErr != nil {
		return svcErr
	}
	for _, check := range s.resourceChecks {
		hasResources, svcErr := check(ctx, id)
		if svcErr != nil {
			s.logger.Error(ctx, "Failed to check the resources of a project", log.String("projectID", id))
			return &tidcommon.InternalServerError
		}
		if hasResources {
			return &ErrorProjectNotEmpty
		}
	}

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
		err = s.store.DeleteProject(txCtx, id)
		if errors.Is(err, errProjectNotFound) {
			capturedSvcErr = errProjectNotFoundNoop
		}
		return err
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

// AddResourceCheck adds a check that refuses to delete a project while resources of one type still
// belong to it. Checks are added while the server starts, before it serves requests.
func (s *projectService) AddResourceCheck(check ResourceCheck) {
	s.resourceChecks = append(s.resourceChecks, check)
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
	created, updating, svcErr := s.importProject(ctx, id, handle, displayName, description, upsert)
	// An import is recorded as a create unless it updated, or tried to update, a project that exists.
	if updating {
		s.publishProjectEvent(ctx, audit.EventProjectUpdate, audit.ActionUpdate, id, svcErr)
		return created, svcErr
	}
	var projectID string
	if svcErr == nil {
		projectID = id
	}
	s.publishProjectEvent(ctx, audit.EventProjectCreate, audit.ActionCreate, projectID, svcErr)
	return created, svcErr
}

// importProject imports a project without publishing its audit event. It also reports whether it
// went on to update a project that exists.
func (s *projectService) importProject(ctx context.Context, id, handle, displayName,
	description string, upsert bool) (bool, bool, *tidcommon.ServiceError) {
	if !utils.IsValidUUID(id) {
		return false, false, &ErrorInvalidProjectID
	}
	request := ProjectRequest{ID: id, Handle: handle, DisplayName: displayName, Description: description}
	if svcErr := validateDetails(request); svcErr != nil {
		return false, false, svcErr
	}

	_, err := s.store.GetProject(ctx, id)
	if err == nil {
		if !upsert {
			return false, false, &ErrorProjectIDConflict
		}
		_, svcErr := s.updateProject(ctx, id, request)
		return false, true, svcErr
	}
	if !errors.Is(err, errProjectNotFound) {
		s.logger.Error(ctx, "Failed to get project", log.Error(err))
		return false, false, &tidcommon.InternalServerError
	}

	_, svcErr := s.createProject(ctx, request)
	return true, false, svcErr
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

// publishCreate publishes the audit event of an attempt to create a project and returns its result.
func (s *projectService) publishCreate(ctx context.Context, created *Project,
	svcErr *tidcommon.ServiceError) (*Project, *tidcommon.ServiceError) {
	var projectID string
	if svcErr == nil {
		projectID = created.ID
	}
	s.publishProjectEvent(ctx, audit.EventProjectCreate, audit.ActionCreate, projectID, svcErr)
	return created, svcErr
}

// publishProjectEvent publishes the audit event of an attempt on the project with projectID, after the
// change committed or was refused. The project service has no access check of its own, so nothing it
// records is denied.
func (s *projectService) publishProjectEvent(ctx context.Context, name string, action audit.Action,
	projectID string, svcErr *tidcommon.ServiceError) {
	// A failed attempt keeps the ID the caller asked for, even one that matches no project, as forensic
	// evidence of what was probed; the payload caps it and the sink escapes it.
	target := audit.Ref{Type: audit.TargetTypeProject, ID: projectID}
	eventbus.TryPublish(ctx, s.bus,
		eventbus.NewEvent(ctx, name).With(audit.ChangePayload(ctx, target, action, svcErr)))
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

// refuseManaged refuses a change to a project a control plane deployed, unless the change is the managed
// import's: only a deploy from the control plane changes or removes it.
func (s *projectService) refuseManaged(ctx context.Context, id string) *tidcommon.ServiceError {
	managed, err := managedresource.IsReadOnly(ctx, s.managed, id, managedresource.TypeProject)
	if err != nil {
		s.logger.Error(ctx, "Failed to read whether a control plane deployed the project",
			log.String("projectID", id), log.Error(err))
		return &tidcommon.InternalServerError
	}
	if managed {
		return &ErrorManagedProject
	}
	return nil
}
