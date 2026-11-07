// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/thunder-id/thunderid/internal/serverconfig"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// ProjectAutoSyncRequest changes one project's entry in the gateway section. AutoSync is true or false
// to set it, null to inherit autoApply, and left out to keep it. FirstGateway is a gateway's id to set
// where the project's pipeline starts, "" to start at the configured pipeline's first stage, and left
// out to keep it. AutoSync null without FirstGateway removes the project's entry.
type ProjectAutoSyncRequest struct {
	AutoSync     json.RawMessage `json:"autoSync,omitempty"`
	FirstGateway *string         `json:"firstGateway,omitempty"`
}

// projectSyncUpdate changes one project's entry of the gateway section, from the section as it is when
// the write is made, so an update of another project's entry at the same moment is kept.
type projectSyncUpdate struct {
	projectID string
	// remove drops the entry; otherwise autoSync and firstGateway are set when given.
	remove       bool
	setAutoSync  bool
	autoSync     *bool
	firstGateway *string
	// result is the entry as written, or nil when it was removed.
	result *ProjectSyncSetting
}

func (u *projectSyncUpdate) Update(current any) (any, error) {
	section, ok := current.(settingsValue)
	if !ok {
		return nil, fmt.Errorf("unexpected gateway section %T", current)
	}
	projects := make(map[string]ProjectSyncSetting, len(section.Projects)+1)
	for id, setting := range section.Projects {
		projects[id] = setting
	}
	u.result = nil
	if u.remove {
		delete(projects, u.projectID)
	} else {
		entry := projects[u.projectID]
		if u.setAutoSync {
			entry.AutoSync = u.autoSync
		}
		if u.firstGateway != nil {
			entry.FirstGateway = *u.firstGateway
		}
		if entry.AutoSync == nil && entry.FirstGateway == "" {
			delete(projects, u.projectID)
		} else {
			projects[u.projectID] = entry
			u.result = &entry
		}
	}
	if len(projects) == 0 {
		projects = nil
	}
	section.Projects = projects
	return section, nil
}

// SetProjectAutoSync changes one project's entry in the gateway section, on the server, so updates of
// different projects at the same moment, on any node, are all kept. It answers the entry as written, or
// nil when the update removed it.
func (s *versionService) SetProjectAutoSync(ctx context.Context, projectID string,
	req ProjectAutoSyncRequest) (*ProjectSyncSetting, *tidcommon.ServiceError) {
	found, svcErr := s.projects.ProjectExists(ctx, projectID)
	if svcErr != nil {
		return nil, svcErr
	}
	if !found {
		return nil, &ErrorProjectNotFound
	}
	update := &projectSyncUpdate{projectID: projectID, firstGateway: req.FirstGateway}
	if req.AutoSync != nil {
		update.setAutoSync = true
		if string(req.AutoSync) != "null" {
			var autoSync bool
			if err := json.Unmarshal(req.AutoSync, &autoSync); err != nil {
				return nil, &ErrorInvalidRegistrationBody
			}
			update.autoSync = &autoSync
		}
	}
	update.remove = update.setAutoSync && update.autoSync == nil && req.FirstGateway == nil
	if req.FirstGateway != nil && *req.FirstGateway != "" {
		if _, svcErr := s.gateway(ctx, *req.FirstGateway); svcErr != nil {
			return nil, svcErr
		}
	}
	if svcErr := s.updateSyncSettings(ctx, update); svcErr != nil {
		return nil, svcErr
	}
	return update.result, nil
}

// DeleteProjectAutoSync removes one project's entry from the gateway section, so it inherits autoApply
// and starts at the configured pipeline's first stage. A project the deployment no longer has can be
// removed too, so an entry outliving its project can be cleaned up.
func (s *versionService) DeleteProjectAutoSync(ctx context.Context, projectID string) *tidcommon.ServiceError {
	return s.updateSyncSettings(ctx, &projectSyncUpdate{projectID: projectID, remove: true})
}

func (s *versionService) updateSyncSettings(ctx context.Context, update *projectSyncUpdate) *tidcommon.ServiceError {
	if s.syncSettings == nil {
		return &ErrorProjectNotFound
	}
	return s.syncSettings.UpdateConfig(ctx, serverconfig.ConfigNameGateway, update)
}
