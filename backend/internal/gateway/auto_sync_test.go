// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// projectAutoApply is an auto apply setting with a default and each project's own setting.
type projectAutoApply struct {
	on       bool
	projects map[string]ProjectSyncSetting
}

func (p projectAutoApply) enabled(context.Context) bool { return p.on }

func (p projectAutoApply) projectSettings(context.Context) map[string]ProjectSyncSetting {
	return p.projects
}

func on() *bool  { v := true; return &v }
func off() *bool { v := false; return &v }

// syncedDocs is the suite's configuration with the shop's and the billing application changed, and the
// translation.
func syncedDocs() []string {
	next := docs(flow("flow-1"))
	next[5] = "resource_type: translation\nlanguage: en-US\ntext: hello"
	next[6] += "description: shop change\n"
	next[7] += "description: billing change\n"
	return next
}

// captureAfter captures the given documents after the suite's configuration was deployed whole to gw-1,
// and syncs what the capture changed.
func (s *ProjectScopeTestSuite) captureAfter(setting projectAutoApply, next []string) *Version {
	ctx := context.Background()
	svc := s.f.svc.(*versionService)
	s.captureDocs(docs(flow("flow-1"))...)
	_, svcErr := s.f.svc.Apply(ctx, "gw-1", ApplyRequest{})
	s.Require().Nil(svcErr)
	svc.autoApply = setting
	before := svc.latestResources(ctx)
	s.f.exporter.response = documents(next...)
	captured, svcErr := s.f.svc.Capture(ctx, CaptureRequest{})
	s.Require().Nil(svcErr)
	s.f.client.sent = nil
	svc.syncAfterCapture(ctx, before, captured)
	return captured
}

// A project with auto sync on takes its own change and every shared change to its first gateway; a
// project with it off keeps its change pending there.
func (s *ProjectScopeTestSuite) TestAutoSyncDeploysOnlyTheProjectsThatHaveItOn() {
	captured := s.captureAfter(projectAutoApply{projects: map[string]ProjectSyncSetting{
		"shop":    {AutoSync: on(), FirstGateway: "gw-1"},
		"billing": {AutoSync: off(), FirstGateway: "gw-1"},
	}}, syncedDocs())

	require.Len(s.T(), s.f.client.sent, 1)
	sent := s.f.client.last().Content
	s.Contains(sent, "shop change")
	s.Contains(sent, "text: hello", "the shared translation did not ride with the sync")
	s.NotContains(sent, "billing change", "a project with auto sync off was deployed")

	ctx := context.Background()
	billing, svcErr := s.f.svc.ProjectStatus(ctx, "gw-1", "billing", "", "")
	s.Require().Nil(svcErr)
	s.Equal(1, billing.Project.Pending)
	s.Equal(ProjectAutoSync{Enabled: false, Inherited: false}, billing.AutoSync)
	shop, svcErr := s.f.svc.ProjectStatus(ctx, "gw-1", "shop", "", "")
	s.Require().Nil(svcErr)
	s.Equal(0, shop.Project.Pending)
	s.True(shop.AutoSync.Enabled)
	s.False(shop.AutoSync.Inherited)
	s.Require().NotNil(shop.AutoSync.LastRun)
	s.Equal(VersionRef{Hash: captured.Hash, Number: captured.Seq}, shop.AutoSync.LastRun.Version)
	s.Nil(shop.AutoSync.LastRun.Error)
}

// One project's refused sync is recorded on it and does not stop the next project's.
func (s *ProjectScopeTestSuite) TestARefusedSyncDoesNotStopTheOthers() {
	s.captureAfter(projectAutoApply{projects: map[string]ProjectSyncSetting{
		"billing": {AutoSync: on(), FirstGateway: "gw-gone"},
		"shop":    {AutoSync: on(), FirstGateway: "gw-1"},
	}}, syncedDocs())

	require.Len(s.T(), s.f.client.sent, 1)
	s.Contains(s.f.client.last().Content, "shop change")
	billing, svcErr := s.f.svc.ProjectStatus(context.Background(), "gw-1", "billing", "", "")
	s.Require().Nil(svcErr)
	s.Require().NotNil(billing.AutoSync.LastRun)
	s.Require().NotNil(billing.AutoSync.LastRun.Error)
	s.Equal(ErrorGatewayNotFound.Code, billing.AutoSync.LastRun.Error.Code)
}

// A project without a setting of its own inherits the default, and syncs to the first stage of the
// configured pipeline.
func (s *ProjectScopeTestSuite) TestAProjectInheritsAutoSync() {
	s.f.gateways.gateways[0].IsDefault = true
	s.captureAfter(projectAutoApply{on: true, projects: map[string]ProjectSyncSetting{
		"shop": {FirstGateway: "gw-1"},
	}}, syncedDocs())

	require.Len(s.T(), s.f.client.sent, 2, "each project synced on its own")
	all := s.f.client.sent[0].Content + s.f.client.sent[1].Content
	s.Contains(all, "shop change")
	s.Contains(all, "billing change")
	billing, svcErr := s.f.svc.ProjectStatus(context.Background(), "gw-1", "billing", "", "")
	s.Require().Nil(svcErr)
	s.Equal(ProjectAutoSync{Enabled: true, Inherited: true, LastRun: billing.AutoSync.LastRun}, billing.AutoSync)
	s.NotNil(billing.AutoSync.LastRun)
}

// A capture that changes nothing of a project nor anything shared syncs nothing for it.
func (s *ProjectScopeTestSuite) TestNothingChangedSyncsNothing() {
	s.captureAfter(projectAutoApply{projects: map[string]ProjectSyncSetting{
		"shop": {AutoSync: on(), FirstGateway: "gw-1"},
	}}, docs(flow("flow-1")))

	s.Empty(s.f.client.sent)
	shop, svcErr := s.f.svc.ProjectStatus(context.Background(), "gw-1", "shop", "", "")
	s.Require().Nil(svcErr)
	s.Nil(shop.AutoSync.LastRun)
}

// Without auto apply there is nothing to sync, and the status says auto sync is off by inheritance.
func TestWithoutAutoApplyNothingIsSynced(t *testing.T) {
	f := newVersionFixture(t)
	svc := f.svc.(*versionService)
	svc.autoApply = nil

	svc.syncAfterCapture(context.Background(), "", &Version{})

	assert.Empty(t, f.client.sent)
	assert.Equal(t, ProjectAutoSync{Inherited: true}, svc.projectAutoSync(context.Background(), "shop"))
}

// The section takes each project's sync setting, and refuses a field it does not know at any depth.
func TestTheSectionDecodesProjectSettings(t *testing.T) {
	h := settingsHandler{}

	value, err := h.Decode(json.RawMessage(
		`{"autoApply": false, "projects": {"p-1": {"autoSync": true, "firstGateway": "gw-1"}, "p-2": {}}}`))

	require.NoError(t, err)
	assert.Equal(t, settingsValue{AutoApply: off(), Projects: map[string]ProjectSyncSetting{
		"p-1": {AutoSync: on(), FirstGateway: "gw-1"}, "p-2": {}}}, value)
	_, err = h.Decode(json.RawMessage(`{"projects": {"p-1": {"enabled": true}}}`))
	assert.Error(t, err)
	merged := settingsHandler{base: settingsValue{AutoApply: off()}}.Merge(settingsValue{},
		settingsValue{Projects: map[string]ProjectSyncSetting{"p-1": {AutoSync: on()}}})
	assert.Equal(t, settingsValue{AutoApply: off(), Projects: map[string]ProjectSyncSetting{
		"p-1": {AutoSync: on()}}}, merged)
	assert.False(t, settingsValue{Projects: map[string]ProjectSyncSetting{"p": {}}}.isEmpty())
}

// Without a reader there are no project settings, so auto apply alone decides.
func TestNoReaderHasNoProjectSettings(t *testing.T) {
	assert.Nil(t, newAutoApplySource(nil).projectSettings(context.Background()))
}

// A changed flow only billing's resources need goes with billing's sync alone: the shop's sync leaves
// it, and billing, whose sync is off, shows it pending. A flow both projects need still rides with the
// shop's sync.
func (s *ProjectScopeTestSuite) TestWhatOnlyOneProjectNeedsSyncsWithItAlone() {
	both := projectApp("app-bill-2", "billing", "ou-bill", "flow-1")
	base := append(docs(flow("flow-1")), both)
	next := append(docs("resource_type: flow\nid: flow-1\nname: shared login, edited"), both)
	next[4] = "resource_type: flow\nid: flow-2\nname: billing login, edited"
	ctx := context.Background()
	svc := s.f.svc.(*versionService)
	s.captureDocs(base...)
	_, svcErr := s.f.svc.Apply(ctx, "gw-1", ApplyRequest{})
	s.Require().Nil(svcErr)
	svc.autoApply = projectAutoApply{projects: map[string]ProjectSyncSetting{
		"shop":    {AutoSync: on(), FirstGateway: "gw-1"},
		"billing": {AutoSync: off(), FirstGateway: "gw-1"},
	}}
	before := svc.latestResources(ctx)
	s.f.exporter.response = documents(next...)
	captured, svcErr := s.f.svc.Capture(ctx, CaptureRequest{})
	s.Require().Nil(svcErr)
	s.f.client.sent = nil

	svc.syncAfterCapture(ctx, before, captured)

	require.Len(s.T(), s.f.client.sent, 1)
	sent := s.f.client.last().Content
	s.Contains(sent, "shared login, edited", "the flow both projects need did not ride with the shop's sync")
	s.NotContains(sent, "billing login, edited", "billing's flow went with the shop's sync")
	billing, svcErr := s.f.svc.ProjectStatus(ctx, "gw-1", "billing", "", "")
	s.Require().Nil(svcErr)
	s.Equal(1, billing.Project.Pending)
}

// A resource the resources of one project need, followed through what that needs, maps to that project;
// one needed by two projects, by none, or owned by a project does not.
func TestSoleNeedersFollowWhatEachProjectNeeds(t *testing.T) {
	sole := soleNeeders(parseBundle(joinBundle([]string{
		projectApp("a", "shop", "ou-1", "flow-a"),
		projectApp("b", "billing", "ou-1", "flow-b"),
		"resource_type: flow\nid: flow-a\nname: a\nnext: flow-deep",
		"resource_type: flow\nid: flow-deep\nname: deep",
		"resource_type: flow\nid: flow-b\nname: b",
		"resource_type: flow\nid: flow-none\nname: none",
		"resource_type: organization_unit\nid: ou-1\nname: one",
	})))

	assert.Equal(t, map[string]string{"flow/flow-a": "shop", "flow/flow-deep": "shop", "flow/flow-b": "billing"},
		sole)
}

// The organization's own unit and the type its agents take stay the organization's, though only one
// project's resources need them.
func TestSoleNeedersLeaveTheOrganizationsOwnResourcesShared(t *testing.T) {
	sole := soleNeeders(parseBundle(joinBundle([]string{
		projectApp("a", "shop", "ou-1", "flow-a"),
		"resource_type: flow\nid: flow-a\nname: a",
		"resource_type: organization_unit\nid: ou-1\nname: one",
		"resource_type: agent_type\nid: at-1\nhandle: default\nname: Default",
		"resource_type: agent\nid: ag-1\nprojectId: shop\ntype: default\nouId: ou-1",
	})))

	assert.Equal(t, map[string]string{"flow/flow-a": "shop"}, sole)
}

// Settings that name only projects the deployment no longer has leave the whole version to auto apply.
func (s *ProjectScopeTestSuite) TestStaleProjectSettingsLeaveTheWholeVersionToAutoApply() {
	s.f.gateways.gateways[0].IsDefault = true

	s.captureAfter(projectAutoApply{on: true, projects: map[string]ProjectSyncSetting{
		"gone": {AutoSync: off()}}}, syncedDocs())

	s.Require().Len(s.f.client.sent, 1)
	sent := s.f.client.last().Content
	s.Contains(sent, "shop change")
	s.Contains(sent, "billing change")
}

// A project change the gateway refused on one sync is sent again by the next, even though that capture
// did not change it; what the gateway holds back stays out.
func (s *ProjectScopeTestSuite) TestALaterSyncRetriesAPendingProjectChangeAndKeepsWhatIsHeldBack() {
	ctx := context.Background()
	svc := s.f.svc.(*versionService)
	setting := projectAutoApply{projects: map[string]ProjectSyncSetting{
		"shop": {AutoSync: on(), FirstGateway: "gw-1"}}}
	s.f.client.imported = answerOf(importOutcome{ResourceType: "application", ResourceID: "app-shop",
		Status: outcomeFailed, Code: "APP-1"})
	s.captureAfter(setting, syncedDocs())
	s.f.client.imported = nil

	before := svc.latestResources(ctx)
	next := syncedDocs()
	next[5] = "resource_type: translation\nlanguage: en-US\ntext: hello again"
	s.f.exporter.response = documents(next...)
	captured, svcErr := s.f.svc.Capture(ctx, CaptureRequest{})
	s.Require().Nil(svcErr)
	_, svcErr = s.f.svc.Apply(ctx, "gw-1", ApplyRequest{Selection: &[]string{},
		LeaveOut: []string{"translation/en-US"}})
	s.Require().Nil(svcErr)
	s.f.client.sent = nil
	svc.syncAfterCapture(ctx, before, captured)

	s.Require().Len(s.f.client.sent, 1)
	sent := s.f.client.last().Content
	s.Contains(sent, "shop change", "the refused project change was not tried again")
	s.NotContains(sent, "hello again", "a change the gateway holds back went with the sync")
	s.NotContains(sent, "billing change")
}
