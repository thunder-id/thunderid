// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package managedresource records what a control plane deployed to this data plane, so the data plane
// can keep it read only. Only the managed import marks anything; every other write path asks whether
// what it is about to change is marked, and refuses it if so.
package managedresource

import (
	"context"
	"fmt"
	"slices"

	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/deployment"
)

// The collections of a resource whose entries a control plane owns one by one. Each entry is keyed
// "<type>/<id>", such as "user/<id>" for a group member.
const (
	// PartMember names a group's members.
	PartMember = "member"
	// PartAssignment names a role's assignments.
	PartAssignment = "assignment"
)

// StoreInterface reads and writes the marks.
type StoreInterface interface {
	// Mark records a resource as deployed by a control plane.
	Mark(ctx context.Context, resourceType, id string) error
	// Unmark forgets a resource and every entry of it.
	Unmark(ctx context.Context, resourceType, id string) error
	// IsManaged reports whether the id is marked as one of the given resource types.
	IsManaged(ctx context.Context, id string, resourceTypes ...string) (bool, error)
	// ManagedIDs reports which of the ids are marked as one of the given resource types, for a list.
	ManagedIDs(ctx context.Context, ids []string, resourceTypes ...string) (map[string]bool, error)
	// MarkParts records entries of one collection of a resource as deployed by a control plane.
	MarkParts(ctx context.Context, resourceType, id, part string, keys []string) error
	// UnmarkParts forgets entries of one collection of a resource.
	UnmarkParts(ctx context.Context, resourceType, id, part string, keys []string) error
	// PartKeys reads the marked entries of one collection of a resource.
	PartKeys(ctx context.Context, resourceType, id, part string) ([]string, error)
	// IsPartManaged reports whether one entry of a resource is marked.
	IsPartManaged(ctx context.Context, resourceType, id, part, key string) (bool, error)
}

type store struct {
	dbProvider provider.DBProviderInterface
}

// defaultStore is the store Initialize made, nil until then.
var defaultStore StoreInterface

// Initialize makes the store of marks, in the configuration database, the one every service reads. The
// server calls it before the services start. A control plane does not: it never holds what a control plane
// deployed, so its services check nothing.
func Initialize() StoreInterface {
	defaultStore = &store{dbProvider: provider.GetDBProvider()}
	return defaultStore
}

// GetStore returns the store Initialize made, or nil where it was not called. Every helper here takes a
// nil store as one that marks nothing.
func GetStore() StoreInterface {
	return defaultStore
}

func (s *store) Mark(ctx context.Context, resourceType, id string) error {
	if err := s.execute(ctx, queryMark, resourceType, id, "", "", deployment.Resolve(ctx)); err != nil {
		return fmt.Errorf("failed to mark %s %s as managed: %w", resourceType, id, err)
	}
	return nil
}

func (s *store) Unmark(ctx context.Context, resourceType, id string) error {
	if err := s.execute(ctx, queryUnmark, resourceType, id, deployment.Resolve(ctx)); err != nil {
		return fmt.Errorf("failed to unmark %s %s: %w", resourceType, id, err)
	}
	return nil
}

func (s *store) IsManaged(ctx context.Context, id string, resourceTypes ...string) (bool, error) {
	rows, err := s.query(ctx, queryGetTypes, id, deployment.Resolve(ctx))
	if err != nil {
		return false, fmt.Errorf("failed to read whether %s is managed: %w", id, err)
	}
	for _, row := range rows {
		if t, ok := row["resource_type"].(string); ok && slices.Contains(resourceTypes, t) {
			return true, nil
		}
	}
	return false, nil
}

func (s *store) ManagedIDs(ctx context.Context, ids []string,
	resourceTypes ...string) (map[string]bool, error) {
	managed := make(map[string]bool)
	if len(ids) == 0 {
		return managed, nil
	}
	query, args := buildManagedIDsQuery(ids, deployment.Resolve(ctx))
	rows, err := s.query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to read which resources are managed: %w", err)
	}
	for _, row := range rows {
		id, idOK := row["resource_id"].(string)
		t, typeOK := row["resource_type"].(string)
		if idOK && typeOK && slices.Contains(resourceTypes, t) {
			managed[id] = true
		}
	}
	return managed, nil
}

func (s *store) MarkParts(ctx context.Context, resourceType, id, part string, keys []string) error {
	deploymentID := deployment.Resolve(ctx)
	for chunk := range slices.Chunk(keys, partsChunk) {
		query, args := buildMarkPartsQuery(resourceType, id, part, chunk, deploymentID)
		if err := s.execute(ctx, query, args...); err != nil {
			return fmt.Errorf("failed to mark %d %s of %s %s as managed: %w", len(chunk), part, resourceType, id, err)
		}
	}
	return nil
}

func (s *store) UnmarkParts(ctx context.Context, resourceType, id, part string, keys []string) error {
	deploymentID := deployment.Resolve(ctx)
	for chunk := range slices.Chunk(keys, partsChunk) {
		query, args := buildUnmarkPartsQuery(resourceType, id, part, chunk, deploymentID)
		if err := s.execute(ctx, query, args...); err != nil {
			return fmt.Errorf("failed to unmark %d %s of %s %s: %w", len(chunk), part, resourceType, id, err)
		}
	}
	return nil
}

func (s *store) PartKeys(ctx context.Context, resourceType, id, part string) ([]string, error) {
	rows, err := s.query(ctx, queryGetPartKeys, resourceType, id, part, deployment.Resolve(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to read the managed %s of %s %s: %w", part, resourceType, id, err)
	}
	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		if key, ok := row["part_key"].(string); ok {
			keys = append(keys, key)
		}
	}
	return keys, nil
}

func (s *store) IsPartManaged(ctx context.Context, resourceType, id, part, key string) (bool, error) {
	rows, err := s.query(ctx, queryIsPartMarked, resourceType, id, part, key, deployment.Resolve(ctx))
	if err != nil {
		return false, fmt.Errorf("failed to read whether %s %s of %s %s is managed: %w",
			part, key, resourceType, id, err)
	}
	return len(rows) > 0, nil
}

func (s *store) execute(ctx context.Context, query dbmodel.DBQuery, args ...interface{}) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	_, err = dbClient.ExecuteContext(ctx, query, args...)
	return err
}

func (s *store) query(ctx context.Context, query dbmodel.DBQuery,
	args ...interface{}) ([]map[string]interface{}, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	return dbClient.QueryContext(ctx, query, args...)
}
