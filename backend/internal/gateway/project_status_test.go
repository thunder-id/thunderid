// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Against a version, a project's status counts its own changes, what it needs that the gateway lacks,
// and what is shared, apart from another project's and what only another project needs; a gateway
// holding none of it was deployed from none.
func (s *ProjectScopeTestSuite) TestAProjectsStatusCountsEachSection() {
	s.captureDocs(docs(flow("flow-1"))...)

	status, svcErr := s.f.svc.ProjectStatus(context.Background(), "gw-1", "shop", "", "")

	s.Require().Nil(svcErr)
	s.Equal(ProjectState{Versions: []ProjectVersion{}, Pending: 1, Required: 3}, status.Project)
	s.Equal(2, status.Shared.Pending, "what only billing needs was counted as shared")
	s.Equal([]TypeCount{{"translation", 1}, {"user", 1}}, status.Shared.ByType)
	s.Equal(0, status.Needed.Differing)
}

// Once deployed, the project's resources name the version they came from, and nothing of it is pending.
func (s *ProjectScopeTestSuite) TestADeployedProjectNamesItsVersion() {
	s.captureDocs(docs(flow("flow-1"))...)
	_, svcErr := s.f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Project: "shop"})
	s.Require().Nil(svcErr)
	latest, svcErr := s.f.svc.GetVersion(context.Background(), "latest")
	s.Require().Nil(svcErr)

	status, svcErr := s.f.svc.ProjectStatus(context.Background(), "gw-1", "shop", "", "")

	s.Require().Nil(svcErr)
	s.Equal([]ProjectVersion{{Hash: latest.Hash, Number: latest.Seq, Resources: 1}}, status.Project.Versions)
	s.Equal(0, status.Project.Pending)
	s.Equal(0, status.Project.Required)
	s.Equal(2, status.Shared.Pending, "what the shop deploy left out is still on offer")

	billing, svcErr := s.f.svc.ProjectStatus(context.Background(), "gw-1", "billing", "", "")
	s.Require().Nil(svcErr)
	s.Empty(billing.Project.Versions)
	s.Equal(1, billing.Project.Pending)
}

// Against a source gateway, the status counts only what a promotion would send: what the source holds
// as its version has it.
func (s *ProjectScopeTestSuite) TestAgainstASourceOnlyWhatItHoldsCounts() {
	s.captureDocs(docs(flow("flow-1"))...)
	_, svcErr := s.f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Project: "shop"})
	s.Require().Nil(svcErr)

	status, svcErr := s.f.svc.ProjectStatus(context.Background(), scopeTarget, "shop", "gw-1", "")

	s.Require().Nil(svcErr)
	s.Equal(1, status.Project.Pending)
	s.Equal(3, status.Project.Required)
	s.Equal(0, status.Shared.Pending, "the source holds nothing shared to promote")

	_, svcErr = s.f.svc.Promote(context.Background(), PromoteRequest{SourceGatewayID: "gw-1",
		TargetGatewayID: scopeTarget, Project: "shop"})
	s.Require().Nil(svcErr)
	status, svcErr = s.f.svc.ProjectStatus(context.Background(), scopeTarget, "shop", "gw-1", "")
	s.Require().Nil(svcErr)
	s.Equal(0, status.Project.Pending)
	s.Len(status.Project.Versions, 1)
}

// A needed resource the gateway holds in another form, which other projects use too, is counted apart,
// as a choice.
func (s *ProjectScopeTestSuite) TestANeededResourceHeldInAnotherFormIsDiffering() {
	sharedFlow := projectApp("app-bill-2", "billing", "ou-bill", "flow-1")
	s.captureDocs(append(docs("resource_type: flow\nid: flow-1\nname: old"), sharedFlow)...)
	_, svcErr := s.f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	s.Require().Nil(svcErr)
	s.captureDocs(append(docs("resource_type: flow\nid: flow-1\nname: new"), sharedFlow)...)

	status, svcErr := s.f.svc.ProjectStatus(context.Background(), "gw-1", "shop", "", "")

	s.Require().Nil(svcErr)
	s.Equal(1, status.Needed.Differing)
	s.Equal(0, status.Project.Required)
	s.Equal(0, status.Project.Pending)
}

// A changed resource only one project's resources need is that project's pending change, and is not an
// update shared with the other projects.
func (s *ProjectScopeTestSuite) TestWhatOnlyOneProjectNeedsIsItsPendingChange() {
	s.captureDocs(docs(flow("flow-1"))...)
	_, svcErr := s.f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	s.Require().Nil(svcErr)
	next := docs(flow("flow-1"))
	next[4] = "resource_type: flow\nid: flow-2\nname: billing login, edited"
	s.captureDocs(next...)

	billing, svcErr := s.f.svc.ProjectStatus(context.Background(), "gw-1", "billing", "", "")
	s.Require().Nil(svcErr)
	s.Equal(1, billing.Project.Pending)
	s.Equal(0, billing.Shared.Pending)
	shop, svcErr := s.f.svc.ProjectStatus(context.Background(), "gw-1", "shop", "", "")
	s.Require().Nil(svcErr)
	s.Equal(0, shop.Project.Pending)
	s.Equal(0, shop.Shared.Pending, "billing's flow was offered to the shop as shared")
}

// An unknown project, a promotion to itself and a source that runs nothing are refused.
func (s *ProjectScopeTestSuite) TestRefusals() {
	s.captureDocs(docs(flow("flow-1"))...)
	ctx := context.Background()

	_, svcErr := s.f.svc.ProjectStatus(ctx, "gw-1", "nope", "", "")
	s.Equal(&ErrorProjectNotFound, svcErr)
	_, svcErr = s.f.svc.ProjectStatus(ctx, "gw-1", "shop", "gw-1", "")
	s.Equal(&ErrorPromoteToItself, svcErr)
	_, svcErr = s.f.svc.ProjectStatus(ctx, "gw-1", "shop", scopeTarget, "")
	s.Equal(&ErrorSourceRunsNothing, svcErr)
	_, svcErr = s.f.svc.ProjectStatus(ctx, "gw-9", "shop", "", "")
	s.Equal(&ErrorGatewayNotFound, svcErr)
}

// A version that is no longer kept is listed last, without its number.
func TestAVersionNoLongerKeptGoesLast(t *testing.T) {
	f := newVersionFixture(t)
	svc := f.svc.(*versionService)
	master := &Master{
		Resources: joinBundle([]string{projectApp("a", "shop", "ou", "f"), projectApp("b", "shop", "ou", "f"),
			projectApp("c", "billing", "ou", "f")}),
		Entries: []MasterEntry{
			{Key: "application/a", State: EntryDeployed, SourceVersion: "gone"},
			{Key: "application/b", State: EntryDeployed, SourceVersion: "gone-too"},
			{Key: "application/c", State: EntryDeployed, SourceVersion: "other"},
		},
	}

	versions := svc.projectVersions(context.Background(), "shop", master)

	assert.Equal(t, []ProjectVersion{{Hash: "gone", Resources: 1}, {Hash: "gone-too", Resources: 1}}, versions)
}

// The status names the last write to the project's resources: a deploy, a promotion from another
// gateway, and a rollback, which is reported at its own time and by whom even when it only removed them.
func (s *ProjectScopeTestSuite) TestTheStatusNamesTheLastWrite() {
	alice := withActor(context.Background(), &Actor{ID: "u-1"})
	bob := withActor(context.Background(), &Actor{ID: "u-2"})
	s.captureDocs(docs(flow("flow-1"))...)
	_, svcErr := s.f.svc.Apply(alice, "gw-1", ApplyRequest{Project: "shop"})
	s.Require().Nil(svcErr)

	dev, svcErr := s.f.svc.ProjectStatus(alice, "gw-1", "shop", "", "")
	s.Require().Nil(svcErr)
	s.Equal(actionDeployed, dev.Project.LastAction)
	s.Equal(&Actor{ID: "u-1", Name: "alice"}, dev.Project.DeployedBy)

	_, svcErr = s.f.svc.Promote(bob, PromoteRequest{SourceGatewayID: "gw-1", TargetGatewayID: scopeTarget,
		Project: "shop"})
	s.Require().Nil(svcErr)
	promoted, svcErr := s.f.svc.ProjectStatus(bob, scopeTarget, "shop", "gw-1", "")
	s.Require().Nil(svcErr)
	s.Equal(actionPromoted, promoted.Project.LastAction)
	s.Equal(&Actor{ID: "u-2", Name: "bob"}, promoted.Project.DeployedBy)
	s.Require().NotNil(promoted.Project.DeployedAt)

	_, svcErr = s.f.svc.Revert(alice, scopeTarget, RevertRequest{Project: "shop"})
	s.Require().Nil(svcErr)
	rolledBack, svcErr := s.f.svc.ProjectStatus(alice, scopeTarget, "shop", "gw-1", "")
	s.Require().Nil(svcErr)
	s.Equal(actionRolledBack, rolledBack.Project.LastAction)
	s.Equal(&Actor{ID: "u-1", Name: "alice"}, rolledBack.Project.DeployedBy)
	s.Require().NotNil(rolledBack.Project.DeployedAt)
	s.False(rolledBack.Project.DeployedAt.Before(*promoted.Project.DeployedAt))
	s.Empty(rolledBack.Project.Versions, "the rollback did not remove the project's resources")
}

// A rollback that a later deploy of the project followed is no longer the last write.
func TestALaterDeployFollowsARollback(t *testing.T) {
	rolledAt := time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)
	latest := &Master{Project: "shop", ProjectRevert: true, Entries: []MasterEntry{
		{Key: "application/a", At: rolledAt, By: &Actor{ID: "u-1"}}}}
	master := &Master{Resources: projectApp("a", "shop", "ou", "f"), Entries: []MasterEntry{
		{Key: "application/a", State: EntryDeployed, At: rolledAt.Add(time.Hour), Origin: "dev"}}}

	at, _, action := lastWriteOf("shop", master, latest)

	assert.Equal(t, rolledAt.Add(time.Hour), *at)
	assert.Equal(t, actionPromoted, action)
	at, by, action := lastWriteOf("shop", &Master{}, nil)
	assert.Nil(t, at)
	assert.Nil(t, by)
	assert.Empty(t, action)
}
