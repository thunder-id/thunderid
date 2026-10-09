// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	governancemodel "github.com/thunder-id/thunderid/internal/identitygovernance/model"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// GetGovernedEntity reads the profile, a fresh runtime row and its parsed access state. A document
// that cannot be parsed fails the read.
func (s *entityService) GetGovernedEntity(ctx context.Context,
	entityID string) (governancemodel.GovernedEntity, error) {
	entity, err := s.store.GetEntity(ctx, entityID)
	if err != nil {
		return governancemodel.GovernedEntity{}, err
	}

	entities := []providers.Entity{entity}
	records, err := s.populateRuntimeSnapshot(ctx, entities)
	if err != nil {
		return governancemodel.GovernedEntity{}, err
	}

	governed, err := governedEntityOf(entities[0])
	if err != nil {
		return governancemodel.GovernedEntity{}, err
	}
	governed.Revision = records[entityID].Revision
	return governed, nil
}

// IncrementFailure records one failed attempt against a scope. The second return is false when
// nothing was written: the entity has no runtime row or a lock is live.
func (s *entityService) IncrementFailure(ctx context.Context, entityID string, scope governancemodel.AccessScope,
	now, windowStart time.Time) (governancemodel.ScopeLock, bool, error) {
	return s.runtimeStore.IncrementAccessFailure(ctx, entityID, scope, now, windowStart)
}

// FormLock opens a lock episode, guarded on the counters the increment observed. It returns false
// when the row changed since then.
func (s *entityService) FormLock(ctx context.Context, entityID string, scope governancemodel.AccessScope,
	observed governancemodel.ScopeLock, episode governancemodel.LockEpisode, now time.Time) (bool, error) {
	return s.runtimeStore.FormAccessLock(ctx, entityID, scope, observed, episode, now)
}

// ClearAccessState removes the given scopes' failure and lock state.
func (s *entityService) ClearAccessState(ctx context.Context, entityID string,
	scopes []governancemodel.AccessScope) error {
	return s.runtimeStore.ClearAccessState(ctx, entityID, scopes)
}

// ClearAccessStateIfUnchanged removes one scope's state only if the runtime row still has the given
// revision, and reports whether it did.
func (s *entityService) ClearAccessStateIfUnchanged(ctx context.Context, entityID string,
	scope governancemodel.AccessScope, revision int64) (bool, error) {
	return s.runtimeStore.ClearAccessStateIfUnchanged(ctx, entityID, scope, revision)
}

// SetSuspension suspends the entity, recording when and the operator's note.
func (s *entityService) SetSuspension(ctx context.Context, entityID string,
	at time.Time, operatorNote string) error {
	return s.runtimeStore.SetAccessSuspension(ctx, entityID, at, operatorNote)
}

// ClearSuspension removes the suspension placed at suspendedAt, optionally replacing it with an
// entity-wide hold in the same write.
func (s *entityService) ClearSuspension(ctx context.Context, entityID string,
	hold *governancemodel.EntityHold, suspendedAt string) error {
	return s.runtimeStore.ClearAccessSuspension(ctx, entityID, hold, suspendedAt)
}

// RecordLogin stores when access was last granted, unless the stored value is already within
// [notBefore, at].
func (s *entityService) RecordLogin(ctx context.Context, entityID string,
	at, notBefore time.Time) error {
	return s.runtimeStore.RecordAccessLogin(ctx, entityID, at, notBefore)
}

// governedEntityOf narrows a hydrated entity to the part governance needs.
func governedEntityOf(entity providers.Entity) (governancemodel.GovernedEntity, error) {
	var stored governancemodel.RuntimeAttributes
	if len(entity.RuntimeAttributes) > 0 {
		if err := json.Unmarshal(entity.RuntimeAttributes, &stored); err != nil {
			return governancemodel.GovernedEntity{},
				fmt.Errorf("failed to unmarshal runtime attributes: %w", err)
		}
	}

	return governancemodel.GovernedEntity{
		ID:          entity.ID,
		Category:    entity.Category,
		Type:        entity.Type,
		OUID:        entity.OUID,
		State:       entity.State,
		AccessState: stored.AccessState,
	}, nil
}

// populateRuntime sets the runtime state and attributes on each entity.
func (s *entityService) populateRuntime(ctx context.Context, entities []providers.Entity) error {
	_, err := s.populateRuntimeSnapshot(ctx, entities)
	return err
}

// populateRuntimeSnapshot reads fresh runtime rows for the entities, outside the profile cache, and
// sets them on the entities. A missing row starts ACTIVE, or with the declarative file state.
func (s *entityService) populateRuntimeSnapshot(ctx context.Context,
	entities []providers.Entity) (map[string]entityRuntimeData, error) {
	if len(entities) == 0 {
		return nil, nil
	}

	ids := make([]string, len(entities))
	for i := range entities {
		ids[i] = entities[i].ID
	}

	records, err := s.runtimeStore.GetRuntimeData(ctx, ids)
	if err != nil {
		return nil, err
	}

	missing := make([]entityRuntimeData, 0)
	missingIDs := make([]string, 0)
	for _, e := range entities {
		if _, ok := records[e.ID]; ok {
			continue
		}
		state := e.State
		if state == "" {
			state = providers.EntityStateActive
		}
		missing = append(missing, entityRuntimeData{ID: e.ID, State: state, Attributes: json.RawMessage(`{}`)})
		missingIDs = append(missingIDs, e.ID)
	}

	if len(missing) > 0 {
		if err := s.runtimeStore.InitializeRuntimeData(ctx, missing); err != nil {
			return nil, err
		}

		// Read the seeded rows back: a row another request wrote first wins over this seed.
		seeded, err := s.runtimeStore.GetRuntimeData(ctx, missingIDs)
		if err != nil {
			return nil, err
		}
		for id, record := range seeded {
			records[id] = record
		}
	}

	for i := range entities {
		record, ok := records[entities[i].ID]
		if !ok {
			return nil, fmt.Errorf("entity runtime row disappeared during initialization")
		}
		entities[i].State = record.State
		entities[i].RuntimeAttributes = record.Attributes
	}

	return records, nil
}
