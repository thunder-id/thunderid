// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"time"

	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// GetMaster reports what a gateway holds, resource by resource.
func (s *versionService) GetMaster(ctx context.Context, gatewayID string) (*Master, *tidcommon.ServiceError) {
	if _, svcErr := s.gateway(ctx, gatewayID); svcErr != nil {
		return nil, svcErr
	}
	master, svcErr := s.master(ctx, gatewayID)
	if svcErr != nil {
		return nil, svcErr
	}
	s.nameMaster(ctx, master)
	return master, nil
}

// ListMasters reports a gateway's masters, newest first, without their entries.
func (s *versionService) ListMasters(ctx context.Context, gatewayID string) ([]Master, *tidcommon.ServiceError) {
	if _, svcErr := s.gateway(ctx, gatewayID); svcErr != nil {
		return nil, svcErr
	}
	masters, svcErr := s.masters(ctx, gatewayID)
	if svcErr != nil {
		return nil, svcErr
	}
	actors := make([]*Actor, 0, len(masters))
	for i := range masters {
		actors = append(actors, masters[i].CreatedBy)
	}
	s.names.fill(ctx, actors...)
	for i := range masters {
		masters[i].CreatedBy = shown(masters[i].CreatedBy)
		masters[i].Entries = nil
	}
	return masters, nil
}

// GetResourceHistory reports one resource's entry across a gateway's masters, newest first.
func (s *versionService) GetResourceHistory(ctx context.Context, gatewayID,
	key string) (*ResourceHistory, *tidcommon.ServiceError) {
	if _, svcErr := s.gateway(ctx, gatewayID); svcErr != nil {
		return nil, svcErr
	}
	masters, svcErr := s.masters(ctx, gatewayID)
	if svcErr != nil {
		return nil, svcErr
	}
	history := &ResourceHistory{GatewayID: gatewayID, Key: key, History: []ResourceHistoryEntry{}}
	actors := []*Actor{}
	for _, master := range masters {
		entry, ok := entryOf(&master, key)
		if !ok {
			continue
		}
		actors = append(actors, entry.By)
		history.History = append(history.History, ResourceHistoryEntry{Master: master.Hash, Number: master.Seq,
			Entry: entry})
	}
	if len(history.History) == 0 {
		return nil, &ErrorResourceNotDeployed
	}
	s.names.fill(ctx, actors...)
	for i := range history.History {
		history.History[i].Entry.By = shown(history.History[i].Entry.By)
	}
	return history, nil
}

// nameMaster puts names to the users a master records.
func (s *versionService) nameMaster(ctx context.Context, master *Master) {
	actors := make([]*Actor, 0, len(master.Entries)+1)
	actors = append(actors, master.CreatedBy)
	for i := range master.Entries {
		actors = append(actors, master.Entries[i].By)
	}
	s.names.fill(ctx, actors...)
	master.CreatedBy = shown(master.CreatedBy)
	for i := range master.Entries {
		master.Entries[i].By = shown(master.Entries[i].By)
	}
}

// master reads the master a gateway holds now. A gateway that applied a version before masters were
// recorded holds that version, less what it left alone, and is given its first master from it. One
// nothing was applied to holds the empty master.
func (s *versionService) master(ctx context.Context, gatewayID string) (*Master, *tidcommon.ServiceError) {
	kept, err := s.versions.GetMaster(ctx, gatewayID)
	if err != nil {
		s.logger.Error(ctx, "Failed to read the gateway's master", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if kept != nil {
		return kept, nil
	}
	applied, err := s.versions.GetApplied(ctx, gatewayID)
	if err != nil {
		s.logger.Error(ctx, "Failed to read what the gateway holds", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if applied == nil || applied.AppliedVersion == 0 {
		return emptyMaster(gatewayID), nil
	}
	leftAlone, err := s.versions.GetExcluded(ctx, gatewayID)
	if err != nil {
		s.logger.Error(ctx, "Failed to read what the gateway leaves alone", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	version, svcErr := s.bySeq(ctx, applied.AppliedVersion)
	if svcErr != nil {
		return nil, svcErr
	}
	at := time.Now().UTC()
	if applied.AppliedAt != nil {
		at = *applied.AppliedAt
	}
	return s.keepMaster(ctx, masterOfVersion(gatewayID, version, leftAlone, at, applied.AppliedBy))
}

// masters reads a gateway's masters, newest first, giving one its first master when it has none.
func (s *versionService) masters(ctx context.Context, gatewayID string) ([]Master, *tidcommon.ServiceError) {
	current, svcErr := s.master(ctx, gatewayID)
	if svcErr != nil {
		return nil, svcErr
	}
	if current.Seq == 0 {
		return []Master{}, nil
	}
	masters, err := s.versions.ListMasters(ctx, gatewayID)
	if err != nil {
		s.logger.Error(ctx, "Failed to list the gateway's masters", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	return masters, nil
}

// keepMaster records a master as the one a gateway holds now and keeps the gateway's history within
// the same bound as the versions. Two writes at the same moment collide on the master's number; the
// one refused answers with what the other kept.
func (s *versionService) keepMaster(ctx context.Context, master *Master) (*Master, *tidcommon.ServiceError) {
	kept, err := s.versions.AddMaster(ctx, *master)
	if err != nil {
		if other, readErr := s.versions.GetMaster(ctx, master.GatewayID); readErr == nil && other != nil {
			return other, nil
		}
		s.logger.Error(ctx, "Failed to record the gateway's master", log.String("gatewayId", master.GatewayID),
			log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if err := s.versions.PruneMasters(ctx, master.GatewayID, kept.Seq-maxVersions()); err != nil {
		// The master is recorded; an older one surviving a little longer is not worth failing for.
		s.logger.Warn(ctx, "Failed to remove the gateway's old masters", log.Error(err))
	}
	return kept, nil
}

// recordMaster writes the master a gateway holds after an apply sent it an import, from the
// gateway's own account of what it wrote and refused.
func (s *versionService) recordMaster(ctx context.Context, previous *Master,
	write masterWrite) (*Master, *tidcommon.ServiceError) {
	next := nextMaster(previous, write)
	next.GatewayID = previous.GatewayID
	kept, svcErr := s.keepMaster(ctx, next)
	if svcErr != nil {
		return nil, svcErr
	}
	s.logger.Info(ctx, "Recorded the gateway's master", log.String("gatewayId", kept.GatewayID),
		log.Int("master", kept.Seq), log.Int("deployed", kept.Summary.Deployed), log.Int("failed", kept.Summary.Failed),
		log.Int("removed", kept.Summary.Removed))
	return kept, nil
}
