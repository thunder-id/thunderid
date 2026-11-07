// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"

	"github.com/thunder-id/thunderid/internal/system/log"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// projectRollback is what a project's rollback on a gateway returns to: the master the gateway held
// before the last deploy scoped to the project.
type projectRollback struct {
	// last is the master the project's last deploy wrote, and before the one it was written over; a
	// deploy that wrote the gateway's first master was written over an empty one.
	last, before *Master
}

// projectRollbackOf finds what a project's rollback on a gateway returns to. It reports false when there
// is nothing: no deploy was scoped to the project, the last one was already rolled back, or the master
// before it is no longer kept. A rollback goes back one step, once, as a revert does.
func (s *versionService) projectRollbackOf(ctx context.Context, gatewayID, projectID string) (projectRollback,
	bool, *tidcommon.ServiceError) {
	masters, err := s.versions.ListMasters(ctx, gatewayID)
	if err != nil {
		s.logger.Error(ctx, "Failed to list the gateway's masters", log.Error(err))
		return projectRollback{}, false, &tidcommon.InternalServerError
	}
	var last *Master
	for i := range masters {
		if masters[i].Project == projectID {
			last = &masters[i]
			break
		}
	}
	if last == nil || last.ProjectRevert {
		return projectRollback{}, false, nil
	}
	if last.Seq <= 1 {
		return projectRollback{last: last, before: &Master{GatewayID: gatewayID}}, true, nil
	}
	before, err := s.versions.GetMasterAt(ctx, gatewayID, last.Seq-1)
	if err != nil {
		s.logger.Error(ctx, "Failed to read the master to roll the project back to", log.Error(err))
		return projectRollback{}, false, &tidcommon.InternalServerError
	}
	if before == nil {
		return projectRollback{}, false, nil
	}
	return projectRollback{last: last, before: before}, true, nil
}

// revertProject rolls a project's own resources on a gateway back to what the gateway held before the
// last deploy scoped to the project: one it added is removed, one it removed is added back and one it
// updated is restored, each keeping where it came from then. Every other resource stays as the gateway
// holds it now, and the gateway stays recorded as running the version it runs. It is planned from the
// master the gateway holds now, so it is refused when another apply changes that master before it is
// sent.
func (s *versionService) revertProject(ctx context.Context, gw *Gateway,
	req RevertRequest) (*ApplyResult, *tidcommon.ServiceError) {
	found, svcErr := s.projects.ProjectExists(ctx, req.Project)
	if svcErr != nil {
		return nil, svcErr
	}
	if !found {
		return nil, &ErrorProjectNotFound
	}
	rollback, ok, svcErr := s.projectRollbackOf(ctx, gw.ID, req.Project)
	if svcErr != nil {
		return nil, svcErr
	}
	applied, err := s.versions.GetApplied(ctx, gw.ID)
	if err != nil {
		s.logger.Error(ctx, "Failed to read what the gateway holds", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if !ok || applied == nil || applied.AppliedVersion == 0 {
		return nil, &ErrorProjectNothingToRevert
	}
	running, svcErr := s.bySeq(ctx, applied.AppliedVersion)
	if svcErr != nil {
		return nil, svcErr
	}
	now, svcErr := s.master(ctx, gw.ID)
	if svcErr != nil {
		return nil, svcErr
	}
	// An earlier master that holds the project's resources as the gateway does now leaves nothing to undo.
	if projectChanges(req.Project, now, rollback.before) == 0 {
		return nil, &ErrorProjectNothingToRevert
	}
	content, owned := withProjectFrom(req.Project, now, rollback.before)
	target := *running
	target.Resources = content
	return s.write(ctx, gw, &target, applyOptions{dryRun: req.DryRun, confirmRemovals: req.ConfirmRemovals,
		revertTo: rollback.before, whole: running, revertProject: req.Project, plannedOn: &now.Hash,
		choose: func(diff *Diff) []string {
			picked := []string{}
			for _, change := range diff.Changes {
				if owned[change.Key] && change.Change != ChangeUnchanged {
					picked = append(picked, change.Key)
				}
			}
			return picked
		}})
}

// withProjectFrom is what a gateway holds now with the project's own resources as an earlier master held
// them, and the keys of the project's own resources in either.
func withProjectFrom(projectID string, now, earlier *Master) (string, map[string]bool) {
	owned := map[string]bool{}
	earlierDocs := map[string]string{}
	for _, resource := range parseBundle(earlier.Resources) {
		if ownerOf(resource) == projectID {
			owned[resource.key()] = true
			earlierDocs[resource.key()] = resource.Content
		}
	}
	documents := []string{}
	for _, resource := range parseBundle(now.Resources) {
		key := resource.key()
		if ownerOf(resource) == projectID {
			owned[key] = true
			continue
		}
		if owned[key] {
			continue
		}
		documents = append(documents, resource.Content)
	}
	for _, resource := range parseBundle(earlier.Resources) {
		if earlierDocs[resource.key()] != "" {
			documents = append(documents, resource.Content)
			delete(earlierDocs, resource.key())
		}
	}
	return joinBundle(documents), owned
}
