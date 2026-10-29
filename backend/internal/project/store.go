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

// errProjectNotFound is the store-level not-found sentinel.
var errProjectNotFound = errors.New("project not found")

// projectStoreInterface persists project records in the entity database, next to their root
// organization units.
type projectStoreInterface interface {
	CreateProject(ctx context.Context, record projectRecord) error
	GetProject(ctx context.Context, id string) (projectRecord, error)
	GetProjectByRootOUID(ctx context.Context, rootOUID string) (projectRecord, error)
	GetProjectList(ctx context.Context, limit, offset int) ([]projectRecord, error)
	GetProjectListCount(ctx context.Context) (int, error)
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

// CreateProject inserts a project record.
func (s *projectStore) CreateProject(ctx context.Context, record projectRecord) error {
	dbClient, err := s.dbProvider.GetEntityDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	if _, err := dbClient.ExecuteContext(ctx, queryCreateProject, record.ID, record.RootOUID,
		record.CreatedAt, s.scope(ctx)); err != nil {
		return fmt.Errorf("failed to create project: %w", err)
	}
	return nil
}

// GetProject returns the project record with the given ID, or errProjectNotFound.
func (s *projectStore) GetProject(ctx context.Context, id string) (projectRecord, error) {
	return s.getOne(ctx, queryGetProjectByID, id)
}

// GetProjectByRootOUID returns the project record backed by the given root organization unit, or
// errProjectNotFound.
func (s *projectStore) GetProjectByRootOUID(ctx context.Context, rootOUID string) (projectRecord, error) {
	return s.getOne(ctx, queryGetProjectByRootOUID, rootOUID)
}

// GetProjectList returns a page of project records, oldest first.
func (s *projectStore) GetProjectList(ctx context.Context, limit, offset int) ([]projectRecord, error) {
	dbClient, err := s.dbProvider.GetEntityDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	results, err := dbClient.QueryContext(ctx, queryGetProjectList, limit, offset, s.scope(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}
	records := make([]projectRecord, 0, len(results))
	for _, row := range results {
		record, err := buildProjectRecord(row)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
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

// DeleteProject removes the project record with the given ID.
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
func (s *projectStore) getOne(ctx context.Context, query dbmodel.DBQuery, identifier string) (projectRecord, error) {
	dbClient, err := s.dbProvider.GetEntityDBClient()
	if err != nil {
		return projectRecord{}, fmt.Errorf("failed to get database client: %w", err)
	}
	results, err := dbClient.QueryContext(ctx, query, identifier, s.scope(ctx))
	if err != nil {
		return projectRecord{}, fmt.Errorf("failed to get project: %w", err)
	}
	if len(results) == 0 {
		return projectRecord{}, errProjectNotFound
	}
	return buildProjectRecord(results[0])
}

// buildProjectRecord maps a result row to a project record.
func buildProjectRecord(row map[string]interface{}) (projectRecord, error) {
	createdAt, err := utils.ParseDBTimeField(row["created_at"], "created_at")
	if err != nil {
		return projectRecord{}, err
	}
	return projectRecord{
		ID:        columnString(row["id"]),
		RootOUID:  columnString(row["root_ou_id"]),
		CreatedAt: createdAt,
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
