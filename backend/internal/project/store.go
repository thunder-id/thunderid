// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"context"
	"errors"
	"fmt"

	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/deployment"
	"github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

var (
	// errProjectNotFound is the store-level not-found sentinel.
	errProjectNotFound = errors.New("project not found")
	// errProjectIDConflict is returned when another project already has the ID.
	errProjectIDConflict = errors.New("a project with the same ID already exists")
	// errProjectHandleConflict is returned when another project already has the handle.
	errProjectHandleConflict = errors.New("a project with the same handle already exists")
)

// projectStoreInterface persists projects in the entity database, next to the organization
// units they own.
type projectStoreInterface interface {
	CreateProject(ctx context.Context, project Project) error
	GetProject(ctx context.Context, id string) (Project, error)
	GetProjectByHandle(ctx context.Context, handle string) (Project, error)
	GetProjectList(ctx context.Context, limit, offset int) ([]Project, error)
	GetProjectListCount(ctx context.Context) (int, error)
	UpdateProject(ctx context.Context, project Project) error
	DeleteProject(ctx context.Context, id string) error
}

// projectStore is the database-backed project store.
type projectStore struct {
	dbProvider provider.DBProviderInterface
}

// newProjectStore returns the entity database backed project store and its transactioner.
func newProjectStore() (projectStoreInterface, providers.Transactioner, error) {
	dbProvider := provider.GetDBProvider()
	transactioner, err := dbProvider.GetEntityDBTransactioner()
	if err != nil {
		return nil, nil, err
	}
	return &projectStore{dbProvider: dbProvider}, transactioner, nil
}

// CreateProject inserts a project, returning errProjectIDConflict when the ID is taken and
// errProjectHandleConflict when the handle is.
func (s *projectStore) CreateProject(ctx context.Context, project Project) error {
	dbClient, err := s.dbProvider.GetEntityDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	_, err = dbClient.ExecuteContext(ctx, queryCreateProject, project.ID, project.Handle,
		project.DisplayName, project.Description, project.CreatedAt, s.scope(ctx))
	switch {
	case err == nil:
		return nil
	case provider.IsUniqueIndexViolation(err, primaryKeyName, primaryKeyColumns):
		return fmt.Errorf("%w: %w", errProjectIDConflict, err)
	case provider.IsUniqueIndexViolation(err, handleIndexName, handleIndexColumns):
		return fmt.Errorf("%w: %w", errProjectHandleConflict, err)
	default:
		return fmt.Errorf("failed to create project: %w", err)
	}
}

// GetProject returns the project with the given ID, or errProjectNotFound.
func (s *projectStore) GetProject(ctx context.Context, id string) (Project, error) {
	return s.getOne(ctx, queryGetProjectByID, id)
}

// GetProjectByHandle returns the project with the given handle, or errProjectNotFound.
func (s *projectStore) GetProjectByHandle(ctx context.Context, handle string) (Project, error) {
	return s.getOne(ctx, queryGetProjectByHandle, handle)
}

// GetProjectList returns a page of projects, oldest first.
func (s *projectStore) GetProjectList(ctx context.Context, limit, offset int) ([]Project, error) {
	dbClient, err := s.dbProvider.GetEntityDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	results, err := dbClient.QueryContext(ctx, queryGetProjectList, limit, offset, s.scope(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}
	projects := make([]Project, 0, len(results))
	for _, row := range results {
		project, err := buildProject(row)
		if err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	return projects, nil
}

// GetProjectListCount returns the number of projects.
func (s *projectStore) GetProjectListCount(ctx context.Context) (int, error) {
	dbClient, err := s.dbProvider.GetEntityDBClient()
	if err != nil {
		return 0, fmt.Errorf("failed to get database client: %w", err)
	}
	results, err := dbClient.QueryContext(ctx, queryGetProjectListCount, s.scope(ctx))
	if err != nil {
		return 0, fmt.Errorf("failed to count projects: %w", err)
	}
	return parseTotal(results)
}

// UpdateProject replaces the display name and description of a project.
func (s *projectStore) UpdateProject(ctx context.Context, project Project) error {
	dbClient, err := s.dbProvider.GetEntityDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	if _, err := dbClient.ExecuteContext(ctx, queryUpdateProject, project.ID, project.DisplayName,
		project.Description, s.scope(ctx)); err != nil {
		return fmt.Errorf("failed to update project: %w", err)
	}
	return nil
}

// DeleteProject removes the project with the given ID.
func (s *projectStore) DeleteProject(ctx context.Context, id string) error {
	dbClient, err := s.dbProvider.GetEntityDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	if _, err := dbClient.ExecuteContext(ctx, queryDeleteProject, id, s.scope(ctx)); err != nil {
		return fmt.Errorf("failed to delete project: %w", err)
	}
	return nil
}

// scope returns the deployment id this request acts for.
func (s *projectStore) scope(ctx context.Context) string {
	return deployment.Resolve(ctx)
}

// getOne runs a single-row project query with the given identifier.
func (s *projectStore) getOne(ctx context.Context, query dbmodel.DBQuery, identifier string) (Project, error) {
	dbClient, err := s.dbProvider.GetEntityDBClient()
	if err != nil {
		return Project{}, fmt.Errorf("failed to get database client: %w", err)
	}
	results, err := dbClient.QueryContext(ctx, query, identifier, s.scope(ctx))
	if err != nil {
		return Project{}, fmt.Errorf("failed to get project: %w", err)
	}
	if len(results) == 0 {
		return Project{}, errProjectNotFound
	}
	return buildProject(results[0])
}

// buildProject maps a result row to a project.
func buildProject(row map[string]interface{}) (Project, error) {
	createdAt, err := utils.ParseDBTimeField(row["created_at"], "created_at")
	if err != nil {
		return Project{}, err
	}
	return Project{
		ID:          columnString(row["id"]),
		Handle:      columnString(row["handle"]),
		DisplayName: columnString(row["display_name"]),
		Description: columnString(row["description"]),
		CreatedAt:   createdAt,
	}, nil
}

// parseTotal reads the "total" column of a count query result.
func parseTotal(results []map[string]interface{}) (int, error) {
	if len(results) == 0 {
		return 0, nil
	}
	total, ok := results[0]["total"].(int64)
	if !ok {
		return 0, fmt.Errorf("unexpected type for total: %T", results[0]["total"])
	}
	return int(total), nil
}

// columnString coerces a result-row value to a string, tolerating string and []byte.
func columnString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return ""
	}
}
