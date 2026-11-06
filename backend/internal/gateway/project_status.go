// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"sort"
	"time"

	"github.com/thunder-id/thunderid/internal/system/log"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// ProjectStatus is where one project stands on a gateway: the versions its own resources were deployed
// from, and what a deploy scoped to it would change, apart from what it would offer to share.
type ProjectStatus struct {
	Project ProjectState `json:"project"`
	Shared  SharedState  `json:"shared"`
	Needed  NeededState  `json:"needed"`
}

// ProjectState is the project's own part of a gateway.
type ProjectState struct {
	// Versions are the versions the project's own resources on the gateway were deployed from, newest
	// first, each with how many of its resources came from it. A gateway holding none has none.
	Versions []ProjectVersion `json:"versions"`
	// Pending counts the project's own changes a deploy scoped to it would send.
	Pending int `json:"pending"`
	// Required counts the changes the project's resources need that the gateway lacks or refused.
	Required int `json:"required"`
	// DeployedAt and DeployedBy are when the project's own resources on the gateway were last written,
	// and by whose apply. A gateway holding none of them has neither.
	DeployedAt *time.Time `json:"deployedAt,omitempty"`
	DeployedBy *Actor     `json:"deployedBy,omitempty"`
	// LastAction is what that last write was: "deployed", "promoted" from another gateway, or
	// "rolledBack" by the project's rollback. A gateway holding none of the project's resources, and
	// never having rolled the project back, has none.
	LastAction string `json:"lastAction,omitempty"`
	// Rollback says whether the project can be rolled back on the gateway.
	Rollback ProjectRollbackState `json:"rollback"`
}

// ProjectRollbackState is whether a project can be rolled back on a gateway, and to which version.
type ProjectRollbackState struct {
	// Available is false while what a rollback returns to holds the project's own resources exactly as
	// the gateway holds them now: there would be nothing to undo.
	Available bool `json:"available"`
	// Changes counts the project's own resources the rollback would write or remove.
	Changes int `json:"changes"`
	// SameVersion marks a rollback to the version the project's resources are on now, which still
	// changes them, as when a resource refused then was applied since, or values changed.
	SameVersion bool `json:"sameVersion,omitempty"`
	// ToVersion is the version the project's own resources were deployed from in the master a rollback
	// returns to, the newest when they came from several. Absent while available, the gateway held none
	// of them then, so a rollback removes them.
	ToVersion *VersionRef `json:"toVersion,omitempty"`
}

// VersionRef names a version: its hash, and its number while it is kept.
type VersionRef struct {
	Hash   string `json:"hash"`
	Number int    `json:"number,omitempty"`
}

// ProjectVersion is a version some of a project's resources on a gateway were deployed from.
type ProjectVersion struct {
	Hash string `json:"hash"`
	// Number is the version's number, absent when the version is no longer kept.
	Number    int `json:"number,omitempty"`
	Resources int `json:"resources"`
}

// SharedState is what belongs to no project that a deploy could share with the gateway.
type SharedState struct {
	Pending int         `json:"pending"`
	ByType  []TypeCount `json:"byType"`
}

// TypeCount counts changes of one resource type.
type TypeCount struct {
	ResourceType string `json:"resourceType"`
	Count        int    `json:"count"`
}

// NeededState is what the project's resources need that the gateway holds in another form.
type NeededState struct {
	Differing int `json:"differing"`
}

// ProjectStatus reports where a project stands on a gateway, against what a promotion from another
// gateway would send, or against a version when no source is given.
func (s *versionService) ProjectStatus(ctx context.Context, gatewayID, projectID, sourceGatewayID,
	ref string) (*ProjectStatus, *tidcommon.ServiceError) {
	if _, svcErr := s.gateway(ctx, gatewayID); svcErr != nil {
		return nil, svcErr
	}
	target, sourceMaster, svcErr := s.statusTarget(ctx, gatewayID, sourceGatewayID, ref)
	if svcErr != nil {
		return nil, svcErr
	}
	_, diff, master, svcErr := s.plan(ctx, gatewayID, target)
	if svcErr != nil {
		return nil, svcErr
	}
	held, svcErr := s.heldBack(ctx, gatewayID, nil)
	if svcErr != nil {
		return nil, svcErr
	}
	if _, svcErr := holdBack(diff, nil, nil, held); svcErr != nil {
		return nil, svcErr
	}
	if _, svcErr := s.scope(ctx, projectID, diff, target, master); svcErr != nil {
		return nil, svcErr
	}
	var promotable map[string]bool
	if sourceMaster != nil {
		promotable = toSet(promoteSelection(sourceMaster, target, diff))
	}
	status := countSections(diff, promotable, projectID, soleNeeders(parseBundle(target.Resources)))
	status.Project.Versions = s.projectVersions(ctx, projectID, master)
	latest, svcErr := s.latestProjectMaster(ctx, gatewayID, projectID)
	if svcErr != nil {
		return nil, svcErr
	}
	status.Project.DeployedAt, status.Project.DeployedBy, status.Project.LastAction = lastWriteOf(projectID,
		master, latest)
	if status.Project.DeployedBy != nil {
		s.names.fill(ctx, status.Project.DeployedBy)
		status.Project.DeployedBy = shown(status.Project.DeployedBy)
	}
	rollback, ok, svcErr := s.projectRollbackOf(ctx, gatewayID, projectID)
	if svcErr != nil {
		return nil, svcErr
	}
	if ok {
		status.Project.Rollback = rollbackStateOf(projectID, master, rollback.before,
			s.projectVersions(ctx, projectID, rollback.before), status.Project.Versions)
	}
	return status, nil
}

// rollbackStateOf is what a project's rollback would undo: available only while it changes any of the
// project's own resources, with the version it returns them to.
func rollbackStateOf(projectID string, now, before *Master, earlier, current []ProjectVersion) ProjectRollbackState {
	state := ProjectRollbackState{Changes: projectChanges(projectID, now, before)}
	if state.Changes == 0 {
		return state
	}
	state.Available = true
	if len(earlier) > 0 {
		state.ToVersion = &VersionRef{Hash: earlier[0].Hash, Number: earlier[0].Number}
		state.SameVersion = len(current) > 0 && current[0].Hash == earlier[0].Hash
	}
	return state
}

// projectChanges counts the project's own resources an earlier master holds otherwise than the gateway
// holds them now: one only one of them holds, one held in another form, and one refused in one and
// accepted in the other.
func projectChanges(projectID string, now, before *Master) int {
	owners := ownersOf(parseBundle(before.Resources), parseBundle(now.Resources))
	held := func(master *Master) map[string]string {
		states := map[string]string{}
		for _, entry := range master.Entries {
			if entry.State == EntryRemoved || owners[entry.Key] != projectID {
				continue
			}
			states[entry.Key] = string(entry.State) + "/" + entry.ContentHash
		}
		return states
	}
	nowHeld, beforeHeld := held(now), held(before)
	changes := 0
	for key, state := range nowHeld {
		if beforeHeld[key] != state {
			changes++
		}
	}
	for key := range beforeHeld {
		if _, ok := nowHeld[key]; !ok {
			changes++
		}
	}
	return changes
}

// Project actions a status reports the last write as.
const (
	actionDeployed   = "deployed"
	actionPromoted   = "promoted"
	actionRolledBack = "rolledBack"
)

// lastWriteOf is when the project's own resources on the gateway were last written, by whom, and how.
// A write is a promotion when it named the gateway it came from. The project's rollback, when it is the
// last write, is reported at its own time, even when it only removed the project's resources.
func lastWriteOf(projectID string, master *Master, latest *Master) (*time.Time, *Actor, string) {
	owners := ownersOf(parseBundle(master.Resources))
	var last *MasterEntry
	for i := range master.Entries {
		entry := &master.Entries[i]
		if entry.State != EntryDeployed || owners[entry.Key] != projectID || entry.At.IsZero() {
			continue
		}
		if last == nil || entry.At.After(last.At) {
			last = entry
		}
	}
	if latest != nil && latest.ProjectRevert {
		if written := lastWrittenEntry(latest); written != nil && (last == nil || !written.At.Before(last.At)) {
			return timeOf(written.At), copyActor(written.By), actionRolledBack
		}
	}
	if last == nil {
		return nil, nil, ""
	}
	action := actionDeployed
	if last.Origin != "" {
		action = actionPromoted
	}
	return timeOf(last.At), copyActor(last.By), action
}

// lastWrittenEntry is the entry a master's own write recorded last: every entry its apply wrote carries
// the time of that apply, and the rest the times of earlier ones.
func lastWrittenEntry(master *Master) *MasterEntry {
	var last *MasterEntry
	for i := range master.Entries {
		entry := &master.Entries[i]
		if !entry.At.IsZero() && (last == nil || entry.At.After(last.At)) {
			last = entry
		}
	}
	return last
}

func timeOf(at time.Time) *time.Time { return &at }

func copyActor(actor *Actor) *Actor {
	if actor == nil {
		return nil
	}
	copied := *actor
	return &copied
}

// latestProjectMaster is the newest master an apply scoped to the project wrote on the gateway, or nil.
func (s *versionService) latestProjectMaster(ctx context.Context, gatewayID, projectID string) (*Master,
	*tidcommon.ServiceError) {
	masters, err := s.versions.ListMasters(ctx, gatewayID)
	if err != nil {
		s.logger.Error(ctx, "Failed to list the gateway's masters", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	for i := range masters {
		if masters[i].Project == projectID {
			return &masters[i], nil
		}
	}
	return nil, nil
}

// statusTarget is the version a project's status is read against: what a promotion from the source
// gateway would send, with what the source holds, or the version named.
func (s *versionService) statusTarget(ctx context.Context, gatewayID, sourceGatewayID,
	ref string) (*Version, *Master, *tidcommon.ServiceError) {
	if sourceGatewayID == "" {
		target, svcErr := s.resolve(ctx, ref)
		return target, nil, svcErr
	}
	if sourceGatewayID == gatewayID {
		return nil, nil, &ErrorPromoteToItself
	}
	source, svcErr := s.GetApplied(ctx, sourceGatewayID)
	if svcErr != nil {
		return nil, nil, svcErr
	}
	if source.Applied == "" {
		return nil, nil, &ErrorSourceRunsNothing
	}
	master, svcErr := s.GetMaster(ctx, sourceGatewayID)
	if svcErr != nil {
		return nil, nil, svcErr
	}
	target, svcErr := s.resolve(ctx, source.Applied)
	if svcErr != nil {
		return nil, nil, svcErr
	}
	return target, master, nil
}

// countSections counts a scoped diff's changes by section, leaving out what is unchanged, held back, or,
// for a promotion, what the source does not hold as the version has it. What only this project's
// resources need counts as the project's own pending change; what only another project's need is that
// project's, and is not counted as shared here.
func countSections(diff *Diff, promotable map[string]bool, projectID string, sole map[string]string) *ProjectStatus {
	status := &ProjectStatus{Project: ProjectState{Versions: []ProjectVersion{}},
		Shared: SharedState{ByType: []TypeCount{}}}
	byType := map[string]int{}
	for _, change := range diff.Changes {
		if change.Change == ChangeUnchanged || change.HeldBack || (promotable != nil && !promotable[change.Key]) {
			continue
		}
		neededBy := sole[change.Key]
		switch change.Section {
		case sectionProject:
			status.Project.Pending++
		case sectionNeeded:
			switch {
			case change.Required:
				status.Project.Required++
			case neededBy == projectID:
				status.Project.Pending++
			default:
				status.Needed.Differing++
			}
		case sectionShared:
			if neededBy != "" && neededBy != projectID {
				continue
			}
			status.Shared.Pending++
			byType[change.ResourceType]++
		}
	}
	for resourceType, count := range byType {
		status.Shared.ByType = append(status.Shared.ByType, TypeCount{ResourceType: resourceType, Count: count})
	}
	sort.Slice(status.Shared.ByType, func(i, j int) bool {
		a, b := status.Shared.ByType[i], status.Shared.ByType[j]
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		return a.ResourceType < b.ResourceType
	})
	return status
}

// projectVersions groups the project's own resources the gateway holds by the version each was deployed
// from, newest first. A version no longer kept goes last, without its number.
func (s *versionService) projectVersions(ctx context.Context, projectID string, master *Master) []ProjectVersion {
	owners := ownersOf(parseBundle(master.Resources))
	counts := map[string]int{}
	for _, entry := range master.Entries {
		if entry.State == EntryDeployed && entry.SourceVersion != "" && owners[entry.Key] == projectID {
			counts[entry.SourceVersion]++
		}
	}
	versions := make([]ProjectVersion, 0, len(counts))
	for hash, count := range counts {
		version := ProjectVersion{Hash: hash, Resources: count}
		found, err := s.versions.Find(ctx, hash)
		if err != nil {
			s.logger.Warn(ctx, "Failed to look for a version a project was deployed from", log.Error(err))
		} else if len(found) > 0 {
			version.Number = found[0]
		}
		versions = append(versions, version)
	}
	sort.Slice(versions, func(i, j int) bool {
		if versions[i].Number != versions[j].Number {
			return versions[i].Number > versions[j].Number
		}
		return versions[i].Hash < versions[j].Hash
	})
	return versions
}
