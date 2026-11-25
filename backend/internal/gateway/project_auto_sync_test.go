// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/serverconfig"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// sectionStore is a server-config service holding the gateway section, updated one update at a time as
// the server-config service writes it.
type sectionStore struct {
	serverconfig.ServerConfigService
	mu      sync.Mutex
	section settingsValue
	err     *tidcommon.ServiceError
}

func (s *sectionStore) UpdateConfig(_ context.Context, name serverconfig.ConfigName,
	updater serverconfig.ConfigUpdater) *tidcommon.ServiceError {
	if s.err != nil {
		return s.err
	}
	if name != serverconfig.ConfigNameGateway {
		return &serverconfig.ErrorUnsupportedConfigName
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next, err := updater.Update(s.section)
	if err != nil {
		return &serverconfig.ErrorInvalidConfigValue
	}
	s.section = next.(settingsValue)
	return nil
}

func syncBody(t *testing.T, raw string) ProjectAutoSyncRequest {
	t.Helper()
	var req ProjectAutoSyncRequest
	require.NoError(t, json.Unmarshal([]byte(raw), &req))
	return req
}

// A project's entry is set, kept when a field is left out, and removed when auto sync is set to inherit
// without a first gateway; another project's entry and autoApply are kept throughout.
func (s *ProjectScopeTestSuite) TestAProjectsAutoSyncEntryIsSetAndRemoved() {
	ctx := context.Background()
	store := &sectionStore{section: settingsValue{AutoApply: on(), Projects: map[string]ProjectSyncSetting{
		"billing": {AutoSync: off()}}}}
	svc := s.f.svc.(*versionService)
	svc.syncSettings = store

	entry, svcErr := s.f.svc.SetProjectAutoSync(ctx, "shop", syncBody(s.T(), `{"autoSync":true,"firstGateway":"gw-1"}`))
	s.Require().Nil(svcErr)
	s.Equal(&ProjectSyncSetting{AutoSync: on(), FirstGateway: "gw-1"}, entry)

	entry, svcErr = s.f.svc.SetProjectAutoSync(ctx, "shop", syncBody(s.T(), `{"firstGateway":"gw-2"}`))
	s.Require().Nil(svcErr)
	s.Equal(&ProjectSyncSetting{AutoSync: on(), FirstGateway: "gw-2"}, entry, "a left-out autoSync changed")

	entry, svcErr = s.f.svc.SetProjectAutoSync(ctx, "shop", syncBody(s.T(), `{"autoSync":null,"firstGateway":"gw-1"}`))
	s.Require().Nil(svcErr)
	s.Equal(&ProjectSyncSetting{FirstGateway: "gw-1"}, entry, "null did not make the project inherit")

	entry, svcErr = s.f.svc.SetProjectAutoSync(ctx, "shop", syncBody(s.T(), `{"autoSync":null}`))
	s.Require().Nil(svcErr)
	s.Nil(entry)
	s.Equal(settingsValue{AutoApply: on(), Projects: map[string]ProjectSyncSetting{"billing": {AutoSync: off()}}},
		store.section)

	s.Require().Nil(s.f.svc.DeleteProjectAutoSync(ctx, "billing"))
	s.Equal(settingsValue{AutoApply: on()}, store.section)
}

// An unknown project or gateway, a body that is not a boolean, a plane without the section and a section
// the server refuses are refused.
func (s *ProjectScopeTestSuite) TestAProjectsAutoSyncRefusals() {
	ctx := context.Background()
	store := &sectionStore{}
	svc := s.f.svc.(*versionService)
	svc.syncSettings = store

	_, svcErr := s.f.svc.SetProjectAutoSync(ctx, "nope", syncBody(s.T(), `{"autoSync":true}`))
	s.Equal(&ErrorProjectNotFound, svcErr)
	_, svcErr = s.f.svc.SetProjectAutoSync(ctx, "shop", syncBody(s.T(), `{"firstGateway":"gw-gone"}`))
	s.Equal(&ErrorGatewayNotFound, svcErr)
	_, svcErr = s.f.svc.SetProjectAutoSync(ctx, "shop", syncBody(s.T(), `{"autoSync":"yes"}`))
	s.Equal(&ErrorInvalidRegistrationBody, svcErr)
	store.err = &serverconfig.ErrorInvalidConfigValue
	_, svcErr = s.f.svc.SetProjectAutoSync(ctx, "shop", syncBody(s.T(), `{"autoSync":true}`))
	s.Equal(&serverconfig.ErrorInvalidConfigValue, svcErr, "a declarative section was not refused")
	svc.syncSettings = nil
	s.Equal(&ErrorProjectNotFound, s.f.svc.DeleteProjectAutoSync(ctx, "shop"))

	s.projects.err = &tidcommon.InternalServerError
	_, svcErr = s.f.svc.SetProjectAutoSync(ctx, "shop", syncBody(s.T(), `{"autoSync":true}`))
	s.Equal(&tidcommon.InternalServerError, svcErr)
}

// Two projects' entries set at the same moment both survive: each update starts from the section as it
// is when it is written.
func (s *ProjectScopeTestSuite) TestConcurrentProjectEntriesBothSurvive() {
	store := &sectionStore{}
	s.f.svc.(*versionService).syncSettings = store
	var wg sync.WaitGroup
	for _, id := range []string{"shop", "billing"} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			_, svcErr := s.f.svc.SetProjectAutoSync(context.Background(), id, ProjectAutoSyncRequest{
				AutoSync: json.RawMessage(`true`)})
			s.Nil(svcErr)
		}(id)
	}
	wg.Wait()

	s.Len(store.section.Projects, 2)
}

// The update refuses a section of another type.
func TestAProjectSyncUpdateNeedsTheGatewaySection(t *testing.T) {
	_, err := (&projectSyncUpdate{projectID: "p"}).Update("not a section")
	assert.Error(t, err)
}
