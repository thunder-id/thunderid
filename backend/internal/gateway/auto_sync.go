// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/thunder-id/thunderid/internal/system/log"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// AutoSyncRun is the outcome of a project's last auto sync: when it ran, the version it deployed, and
// why the gateway refused it, when it did.
type AutoSyncRun struct {
	At      time.Time     `json:"at"`
	Version VersionRef    `json:"version"`
	Error   *AutoSyncFail `json:"error,omitempty"`
}

// AutoSyncFail is why a project's auto sync was refused.
type AutoSyncFail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// syncRuns keeps each project's last auto sync on this node, by project id.
type syncRuns struct {
	mu   sync.Mutex
	runs map[string]AutoSyncRun
}

func (r *syncRuns) record(projectID string, run AutoSyncRun) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runs[projectID] = run
}

func (r *syncRuns) last(projectID string) (AutoSyncRun, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	run, ok := r.runs[projectID]
	return run, ok
}

// latestResources is the content of the newest version before a capture, for what the capture changed
// to be read from. A deployment without a version, or one that cannot be read, has none.
func (s *versionService) latestResources(ctx context.Context) string {
	seq, err := s.versions.Latest(ctx)
	if err != nil || seq == 0 {
		return ""
	}
	latest, err := s.versions.Get(ctx, seq)
	if err != nil || latest == nil {
		return ""
	}
	return latest.Resources
}

// syncAfterCapture deploys what a capture changed. While no project has a sync setting of its own, auto
// apply sends the whole version to the first gateway. Otherwise each project is synced on its own.
func (s *versionService) syncAfterCapture(ctx context.Context, before string, captured *Version) {
	if s.autoApply == nil {
		return
	}
	settings := s.autoApply.projectSettings(ctx)
	var projects map[string]string
	if len(settings) > 0 {
		var svcErr *tidcommon.ServiceError
		if projects, svcErr = s.projects.ProjectNames(ctx); svcErr != nil {
			s.logger.Warn(ctx, "Failed to read the projects to sync", log.String("errorCode", svcErr.Code))
			return
		}
	}
	// Settings that name only projects the deployment no longer has leave the whole version to auto apply.
	if !namesAny(settings, projects) {
		s.applyToFirstGateway(ctx, captured)
		return
	}
	if captured == nil {
		return
	}
	s.syncProjects(ctx, before, captured, settings, projects, s.autoApply.enabled(ctx))
}

// namesAny reports whether the settings name any of the projects.
func namesAny(settings map[string]ProjectSyncSetting, projects map[string]string) bool {
	for id := range settings {
		if _, ok := projects[id]; ok {
			return true
		}
	}
	return false
}

// syncProjects deploys, for each project whose auto sync is on, what the capture changed of its own and
// every shared change, to the project's first gateway. Shared changes go with any project's sync, since
// they belong to none. Projects are synced one after another; one refused does not stop the rest, and
// each outcome is kept for the project's status.
func (s *versionService) syncProjects(ctx context.Context, before string, captured *Version,
	settings map[string]ProjectSyncSetting, projects map[string]string, inherited bool) {
	previous, current := parseBundle(before), parseBundle(captured.Resources)
	owners := ownersOf(previous, current)
	sole := soleNeeders(current)
	own := map[string]map[string]bool{}
	shared := map[string]bool{}
	for _, change := range diffBundles(previous, current) {
		if change.Change == ChangeUnchanged {
			continue
		}
		// What only one project's resources need goes with that project's sync alone.
		owner := owners[change.Key]
		if owner == "" {
			owner = sole[change.Key]
		}
		if owner == "" {
			shared[change.Key] = true
			continue
		}
		if own[owner] == nil {
			own[owner] = map[string]bool{}
		}
		own[owner][change.Key] = true
	}
	ids := make([]string, 0, len(projects))
	for id := range projects {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if !autoSyncOf(settings[id], inherited) || (len(own[id]) == 0 && len(shared) == 0) {
			continue
		}
		s.syncProject(ctx, id, settings[id].FirstGateway, captured, own[id], shared)
	}
}

// autoSyncOf is whether a project is synced: its own setting, or the default it inherits.
func autoSyncOf(setting ProjectSyncSetting, inherited bool) bool {
	if setting.AutoSync != nil {
		return *setting.AutoSync
	}
	return inherited
}

// syncProject deploys the captured version to a project's first gateway, scoped to the project: what the
// capture changed of the project's own, what they need that the gateway lacks, and every shared change.
func (s *versionService) syncProject(ctx context.Context, projectID, firstGateway string, captured *Version,
	own, shared map[string]bool) {
	gatewayID, svcErr := s.firstGatewayOf(ctx, firstGateway)
	if svcErr == nil && gatewayID == "" {
		return
	}
	var selection []string
	if svcErr == nil {
		selection, svcErr = s.syncSelection(ctx, gatewayID, projectID, captured, own, shared)
	}
	if svcErr == nil && len(selection) == 0 {
		return
	}
	if svcErr == nil {
		_, svcErr = s.Apply(ctx, gatewayID, ApplyRequest{Version: captured.Hash, Project: projectID,
			Selection: &selection})
	}
	run := AutoSyncRun{At: time.Now().UTC(), Version: VersionRef{Hash: captured.Hash, Number: captured.Seq}}
	if svcErr != nil {
		run.Error = &AutoSyncFail{Code: svcErr.Code, Message: svcErr.Error.DefaultValue}
		if svcErr.ErrorDescription.DefaultValue != "" {
			run.Error.Message = svcErr.ErrorDescription.DefaultValue
		}
		s.logger.Warn(ctx, "Failed to sync a project to its first gateway", log.String("projectId", projectID),
			log.String("gatewayId", gatewayID), log.String("code", svcErr.Code))
	}
	s.syncRuns.record(projectID, run)
}

// firstGatewayOf is the gateway a project's pipeline starts at: the one its setting names, or the first
// stage of the configured pipeline. A deployment with neither has none.
func (s *versionService) firstGatewayOf(ctx context.Context, named string) (string, *tidcommon.ServiceError) {
	if named != "" {
		gw, svcErr := s.gateway(ctx, named)
		if svcErr != nil {
			return "", svcErr
		}
		return gw.ID, nil
	}
	gateways, err := s.gateways.List(ctx)
	if err != nil {
		s.logger.Error(ctx, "Failed to list the gateways to sync a project to", log.Error(err))
		return "", &tidcommon.InternalServerError
	}
	gw, ok := s.pipeline.first(ctx, gateways)
	if !ok {
		return "", nil
	}
	return gw.ID, nil
}

// syncSelection is what a project's sync sends to its gateway: the project's own changes still pending
// there, so one an earlier sync had refused is tried again, with what only the project's resources need
// that the capture changed, the needed changes the gateway lacks or refused, and every shared change the
// capture made, as far as the gateway does not hold them already. What the gateway holds back stays
// out: only selecting it stops holding it back, so a sync needing it is refused for it.
func (s *versionService) syncSelection(ctx context.Context, gatewayID, projectID string, captured *Version,
	own, shared map[string]bool) ([]string, *tidcommon.ServiceError) {
	diff, svcErr := s.Diff(ctx, gatewayID, captured.Hash, projectID)
	if svcErr != nil {
		return nil, svcErr
	}
	selection := []string{}
	for _, change := range diff.Changes {
		if (change.Change == ChangeUnchanged && !change.Failed) || change.HeldBack {
			continue
		}
		if change.Section == sectionProject || (change.Section != sectionOther && own[change.Key]) || change.Required ||
			(change.Section != sectionProject && change.Section != sectionOther && shared[change.Key]) {
			selection = append(selection, change.Key)
		}
	}
	return selection, nil
}

// projectAutoSync is a project's auto sync as its status reports it.
func (s *versionService) projectAutoSync(ctx context.Context, projectID string) ProjectAutoSync {
	state := ProjectAutoSync{Inherited: true}
	if s.autoApply == nil {
		return state
	}
	setting := s.autoApply.projectSettings(ctx)[projectID]
	inherited := s.autoApply.enabled(ctx)
	state.Enabled = autoSyncOf(setting, inherited)
	state.Inherited = setting.AutoSync == nil
	if run, ok := s.syncRuns.last(projectID); ok {
		state.LastRun = &run
	}
	return state
}
