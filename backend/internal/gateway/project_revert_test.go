// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/thunder-id/thunderid/internal/system/config"
)

// shopV2 is the suite's configuration with the shop's application changed.
func shopV2() []string {
	next := docs(flow("flow-1"))
	next[6] += "description: second\n"
	return next
}

// When two projects deploy to one gateway, rolling one back restores only its own resources to what the
// gateway held before its last deploy; the other project's stay as they are, and so does the version
// the gateway is recorded as running.
func (s *ProjectScopeTestSuite) TestRollingBackOneProjectLeavesTheOtherAsItIs() {
	ctx := context.Background()
	s.captureDocs(docs(flow("flow-1"))...)
	first, svcErr := s.f.svc.GetVersion(ctx, "latest")
	s.Require().Nil(svcErr)
	_, svcErr = s.f.svc.Apply(ctx, "gw-1", ApplyRequest{Project: "shop"})
	s.Require().Nil(svcErr)
	s.captureDocs(shopV2()...)
	_, svcErr = s.f.svc.Apply(ctx, "gw-1", ApplyRequest{Project: "billing"})
	s.Require().Nil(svcErr)
	_, svcErr = s.f.svc.Apply(ctx, "gw-1", ApplyRequest{Project: "shop"})
	s.Require().Nil(svcErr)
	billingBefore := s.heldDocument("application/app-bill")
	appliedBefore, svcErr := s.f.svc.GetApplied(ctx, "gw-1")
	s.Require().Nil(svcErr)

	status, svcErr := s.f.svc.ProjectStatus(ctx, "gw-1", "shop", "", "")
	s.Require().Nil(svcErr)
	s.True(status.Project.Rollback.Available)
	s.Require().NotNil(status.Project.Rollback.ToVersion)
	s.Equal(first.Hash, status.Project.Rollback.ToVersion.Hash)
	s.NotNil(status.Project.DeployedAt)

	dry, svcErr := s.f.svc.Revert(ctx, "gw-1", RevertRequest{Project: "shop", DryRun: true})
	s.Require().Nil(svcErr)
	s.True(dry.DryRun)
	s.True(changeNamed(&dry.Diff, "application/app-shop").Selected)

	_, svcErr = s.f.svc.Revert(ctx, "gw-1", RevertRequest{Project: "shop"})

	s.Require().Nil(svcErr)
	sent := s.f.client.last()
	s.Contains(sent.Content, "id: app-shop")
	s.NotContains(sent.Content, "description: second")
	s.NotContains(sent.Content, "app-bill", "the other project's resources were sent")
	s.Empty(sent.Deletions)
	s.Equal(billingBefore, s.heldDocument("application/app-bill"))
	s.NotContains(s.heldDocument("application/app-shop"), "description: second")
	appliedAfter, svcErr := s.f.svc.GetApplied(ctx, "gw-1")
	s.Require().Nil(svcErr)
	s.Equal(appliedBefore.Applied, appliedAfter.Applied)
	s.Equal(appliedBefore.Previous, appliedAfter.Previous, "the gateway's own revert moved")

	status, svcErr = s.f.svc.ProjectStatus(ctx, "gw-1", "shop", "", "")
	s.Require().Nil(svcErr)
	s.Equal(first.Hash, status.Project.Versions[0].Hash, "the restored resource lost where it came from")
	s.False(status.Project.Rollback.Available)

	_, svcErr = s.f.svc.Revert(ctx, "gw-1", RevertRequest{Project: "shop"})
	s.Equal(&ErrorProjectNothingToRevert, svcErr, "a project was rolled back twice")
	_, svcErr = s.f.svc.Revert(ctx, "gw-1", RevertRequest{Project: "billing"})
	s.Require().Nil(svcErr, "the other project could no longer roll back")
}

// Rolling back a project's first deploy to a gateway removes its own resources, and leaves what they
// needed in place.
func (s *ProjectScopeTestSuite) TestRollingBackAProjectsFirstDeployRemovesItsResources() {
	ctx := context.Background()
	s.captureDocs(docs(flow("flow-1"))...)
	_, svcErr := s.f.svc.Apply(ctx, "gw-1", ApplyRequest{Project: "shop"})
	s.Require().Nil(svcErr)

	status, svcErr := s.f.svc.ProjectStatus(ctx, "gw-1", "shop", "", "")
	s.Require().Nil(svcErr)
	s.True(status.Project.Rollback.Available)
	s.Nil(status.Project.Rollback.ToVersion)

	_, svcErr = s.f.svc.Revert(ctx, "gw-1", RevertRequest{Project: "shop"})

	s.Require().Nil(svcErr)
	s.Equal([]gatewayDeletion{{ResourceType: "application", ID: "app-shop"}}, s.f.client.last().Deletions)
	s.Empty(strings.TrimSpace(s.f.client.last().Content))
	s.NotEmpty(s.heldDocument("flow/flow-1"))
}

// A project with no deploy of its own, and an unknown project, cannot be rolled back.
func (s *ProjectScopeTestSuite) TestAProjectWithoutADeployCannotRollBack() {
	ctx := context.Background()
	s.captureDocs(docs(flow("flow-1"))...)
	_, svcErr := s.f.svc.Apply(ctx, "gw-1", ApplyRequest{})
	s.Require().Nil(svcErr)

	_, svcErr = s.f.svc.Revert(ctx, "gw-1", RevertRequest{Project: "shop"})
	s.Equal(&ErrorProjectNothingToRevert, svcErr)
	_, svcErr = s.f.svc.Revert(ctx, "gw-1", RevertRequest{Project: "nope"})
	s.Equal(&ErrorProjectNotFound, svcErr)

	status, svcErr := s.f.svc.ProjectStatus(ctx, "gw-1", "shop", "", "")
	s.Require().Nil(svcErr)
	s.False(status.Project.Rollback.Available)
}

// A rollback whose earlier master is no longer kept has nothing to return to.
func (s *ProjectScopeTestSuite) TestARollbackWithoutItsEarlierMasterIsRefused() {
	ctx := context.Background()
	s.captureDocs(docs(flow("flow-1"))...)
	_, svcErr := s.f.svc.Apply(ctx, "gw-1", ApplyRequest{Project: "billing"})
	s.Require().Nil(svcErr)
	s.captureDocs(shopV2()...)
	_, svcErr = s.f.svc.Apply(ctx, "gw-1", ApplyRequest{Project: "shop"})
	s.Require().Nil(svcErr)
	s.Require().NoError(s.f.versions.PruneMasters(ctx, "gw-1", 1, nil))

	_, svcErr = s.f.svc.Revert(ctx, "gw-1", RevertRequest{Project: "shop"})

	s.Equal(&ErrorProjectNothingToRevert, svcErr)
}

// heldDocument is the document gw-1's master holds under a key, or "" when it holds none.
func (s *ProjectScopeTestSuite) heldDocument(key string) string {
	master, svcErr := s.f.svc.GetMaster(context.Background(), "gw-1")
	s.Require().Nil(svcErr)
	for _, resource := range parseBundle(master.Resources) {
		if resource.key() == key {
			return resource.Content
		}
	}
	return ""
}

// The project's own resources are taken from the earlier master, and every other resource from what the
// gateway holds now.
func TestWithProjectFromTakesOnlyTheProjectsResources(t *testing.T) {
	now := &Master{Resources: joinBundle([]string{projectApp("a", "shop", "ou", "f") + "description: now\n",
		projectApp("b", "billing", "ou", "f"), projectApp("c", "shop", "ou", "f")})}
	earlier := &Master{Resources: joinBundle([]string{projectApp("a", "shop", "ou", "f"),
		projectApp("d", "shop", "ou", "f"), projectApp("b", "billing", "ou", "f") + "description: old\n"})}

	content, owned := withProjectFrom("shop", parseBundle(now.Resources), parseBundle(earlier.Resources))

	assert.Equal(t, map[string]bool{"application/a": true, "application/c": true, "application/d": true}, owned)
	keys := map[string]string{}
	for _, resource := range parseBundle(content) {
		keys[resource.key()] = resource.Content
	}
	assert.Len(t, keys, 3)
	assert.NotContains(t, keys["application/a"], "description: now")
	assert.NotContains(t, keys["application/b"], "description: old")
	assert.Contains(t, keys, "application/d")
	assert.NotContains(t, keys, "application/c")
}

// A resource the project held then that another project holds now keeps that project's document, and is
// not the rolled back project's to restore.
func (s *ProjectScopeTestSuite) TestWithProjectFromKeepsAResourceAnotherProjectHoldsNow() {
	now := parseBundle(joinBundle([]string{projectApp("a", "billing", "ou", "f") + "description: billing\n",
		projectApp("c", "shop", "ou", "f")}))
	earlier := parseBundle(joinBundle([]string{projectApp("a", "shop", "ou", "f") + "description: shop\n"}))

	content, owned := withProjectFrom("shop", now, earlier)

	s.Equal(map[string]bool{"application/c": true}, owned)
	bundle := parseBundle(content)
	s.Require().Len(bundle, 1)
	s.Contains(bundle[0].Content, "description: billing")
}

// A project's rollback is refused when a master is written between reading the project's history and
// the master the rollback is planned from, since the history no longer says what it returns to.
func (s *ProjectScopeTestSuite) TestARollbackPlannedAcrossANewMasterIsRefused() {
	versions := newVersionStoreInterfaceMock(s.T())
	svc := newVersionService(s.f.gateways, versions, s.f.exporter, s.f.client, testSecretFields, s.f.names,
		newPipeline(), discardingBus{})
	svc.projects = s.projects
	before := Master{GatewayID: "gw-1", Seq: 1}
	last := Master{GatewayID: "gw-1", Seq: 2, Project: "shop"}
	// The history lists the shop's deploy as the newest master; another one is written before the
	// master is read.
	versions.EXPECT().ListMasters(mock.Anything, "gw-1").Return([]Master{last, before}, nil)
	versions.EXPECT().GetMasterAt(mock.Anything, "gw-1", 1).Return(&before, nil)
	versions.EXPECT().GetApplied(mock.Anything, "gw-1").Return(&AppliedVersion{GatewayID: "gw-1", AppliedVersion: 1},
		nil)
	versions.EXPECT().Get(mock.Anything, 1).Return(&Version{Seq: 1}, nil)
	versions.EXPECT().GetMaster(mock.Anything, "gw-1").Return(&Master{GatewayID: "gw-1", Seq: 3, Project: "shop"}, nil)

	_, svcErr := svc.Revert(context.Background(), "gw-1", RevertRequest{Project: "shop", DryRun: true})

	s.Equal(&ErrorAppliedChanged, svcErr)
}

// A resource the project held at its rollback point that another project holds now is neither shown nor
// written by the project's rollback: the gateway keeps the other project's document.
func (s *ProjectScopeTestSuite) TestARollbackKeepsAResourceAnotherProjectHoldsNow() {
	ctx := context.Background()
	moved := func(projectID, description string) string {
		return projectApp("app-x", projectID, "ou-team", "flow-1") + "description: " + description + "\n"
	}
	s.captureDocs(append(docs(flow("flow-1")), moved("shop", "shop"))...)
	_, svcErr := s.f.svc.Apply(ctx, "gw-1", ApplyRequest{Project: "shop"})
	s.Require().Nil(svcErr)
	s.captureDocs(append(shopV2(), moved("shop", "shop"))...)
	_, svcErr = s.f.svc.Apply(ctx, "gw-1", ApplyRequest{Project: "shop"})
	s.Require().Nil(svcErr)
	s.captureDocs(append(shopV2(), moved("billing", "billing"))...)
	_, svcErr = s.f.svc.Apply(ctx, "gw-1", ApplyRequest{Project: "billing"})
	s.Require().Nil(svcErr)
	s.Require().Contains(s.heldDocument("application/app-x"), "description: billing")

	preview, svcErr := s.f.svc.Preview(ctx, "gw-1", PreviewRequest{Project: "shop", Revert: true})
	s.Require().Nil(svcErr)
	s.False(changeNamed(preview, "application/app-x").Selected, "the rollback offered another project's resource")
	s.True(changeNamed(preview, "application/app-shop").Selected)
	_, svcErr = s.f.svc.Revert(ctx, "gw-1", RevertRequest{Project: "shop", Plan: preview.Plan})

	s.Require().Nil(svcErr)
	s.NotContains(s.f.client.last().Content, "id: app-x")
	s.Contains(s.heldDocument("application/app-x"), "description: billing")
	s.NotContains(s.heldDocument("application/app-shop"), "description: second")
}

// A project's rollback point outlives the bound on kept masters while other projects keep deploying: the
// master before its last deploy, and that deploy's own, are kept.
func (s *ProjectScopeTestSuite) TestAProjectsRollbackPointIsKeptPastTheBound() {
	ctx := context.Background()
	kept := 2
	config.GetServerRuntime().Config.Gateway.MaxVersions = &kept
	s.captureDocs(docs(flow("flow-1"))...)
	_, svcErr := s.f.svc.Apply(ctx, "gw-1", ApplyRequest{Project: "shop"})
	s.Require().Nil(svcErr)
	s.captureDocs(shopV2()...)
	_, svcErr = s.f.svc.Apply(ctx, "gw-1", ApplyRequest{Project: "shop"})
	s.Require().Nil(svcErr)
	for i := 0; i < kept+2; i++ {
		next := shopV2()
		next[7] += "description: billing " + strconv.Itoa(i) + "\n"
		s.captureDocs(next...)
		_, svcErr = s.f.svc.Apply(ctx, "gw-1", ApplyRequest{Project: "billing"})
		s.Require().Nil(svcErr)
	}
	masters, svcErr := s.f.svc.ListMasters(ctx, "gw-1")
	s.Require().Nil(svcErr)
	s.Less(len(masters), 2+kept+2, "nothing was pruned")

	_, svcErr = s.f.svc.Revert(ctx, "gw-1", RevertRequest{Project: "shop"})

	s.Require().Nil(svcErr)
	s.NotContains(s.heldDocument("application/app-shop"), "description: second")
	s.Contains(s.heldDocument("application/app-bill"), "description: billing "+strconv.Itoa(kept+1))
}

// A project's last deploy that wrote none of its own resources anew, such as one that only took a
// shared change with it, leaves nothing of the project to undo: no rollback is offered or run.
func (s *ProjectScopeTestSuite) TestARollbackThatChangesNothingIsNotOffered() {
	ctx := context.Background()
	s.captureDocs(docs(flow("flow-1"))...)
	own := []string{"application/app-shop", "organization_unit/ou-team", "organization_unit/ou-shop", "flow/flow-1"}
	_, svcErr := s.f.svc.Apply(ctx, "gw-1", ApplyRequest{Project: "shop", Selection: &own})
	s.Require().Nil(svcErr)
	_, svcErr = s.f.svc.Apply(ctx, "gw-1", ApplyRequest{Project: "shop", Selection: &[]string{"translation/en-US"}})
	s.Require().Nil(svcErr)

	status, svcErr := s.f.svc.ProjectStatus(ctx, "gw-1", "shop", "", "")

	s.Require().Nil(svcErr)
	s.Equal(ProjectRollbackState{}, status.Project.Rollback)
	_, svcErr = s.f.svc.Revert(ctx, "gw-1", RevertRequest{Project: "shop"})
	s.Equal(&ErrorProjectNothingToRevert, svcErr)
}

// A resource refused, then applied by deploying the same version again, can be rolled back: to the same
// version, undoing that one resource.
func (s *ProjectScopeTestSuite) TestARefusedThenFixedResourceRollsBackToTheSameVersion() {
	ctx := context.Background()
	s.captureDocs(append(docs(flow("flow-1")), projectApp("app-shop-2", "shop", "ou-team", "flow-1"))...)
	s.f.client.imported = answerOf(
		importOutcome{ResourceType: "application", ResourceID: "app-shop", Status: outcomeSuccess},
		importOutcome{ResourceType: "application", ResourceID: "app-shop-2", Status: outcomeFailed, Code: "APP-1"},
		importOutcome{ResourceType: "flow", ResourceID: "flow-1", Status: outcomeSuccess},
		importOutcome{ResourceType: "organization_unit", ResourceID: "ou-shop", Status: outcomeSuccess},
		importOutcome{ResourceType: "organization_unit", ResourceID: "ou-team", Status: outcomeSuccess})
	_, svcErr := s.f.svc.Apply(ctx, "gw-1", ApplyRequest{Project: "shop"})
	s.Require().Nil(svcErr)
	s.f.client.imported = nil
	_, svcErr = s.f.svc.Apply(ctx, "gw-1", ApplyRequest{Project: "shop"})
	s.Require().Nil(svcErr)

	status, svcErr := s.f.svc.ProjectStatus(ctx, "gw-1", "shop", "", "")

	s.Require().Nil(svcErr)
	rollback := status.Project.Rollback
	s.True(rollback.Available)
	s.True(rollback.SameVersion)
	s.Equal(1, rollback.Changes)
	s.Require().NotNil(rollback.ToVersion)
	s.Equal(status.Project.Versions[0].Hash, rollback.ToVersion.Hash)
}

// A project's rollback keeps what the gateway holds back as it is, even for a change it restores: a
// later apply must still leave out the change held back.
func (s *ProjectScopeTestSuite) TestARollbackKeepsWhatTheGatewayHoldsBack() {
	ctx := context.Background()
	s.captureDocs(docs(flow("flow-1"))...)
	_, svcErr := s.f.svc.Apply(ctx, "gw-1", ApplyRequest{Project: "shop"})
	s.Require().Nil(svcErr)
	s.captureDocs(shopV2()...)
	_, svcErr = s.f.svc.Apply(ctx, "gw-1", ApplyRequest{Project: "shop"})
	s.Require().Nil(svcErr)
	third := shopV2()
	third[5] = "resource_type: translation\nlanguage: en-US\ntext: third"
	third[6] += "displayName: third\n"
	s.captureDocs(third...)
	_, svcErr = s.f.svc.Apply(ctx, "gw-1", ApplyRequest{LeaveOut: []string{"application/app-shop"}})
	s.Require().Nil(svcErr)

	_, svcErr = s.f.svc.Revert(ctx, "gw-1", RevertRequest{Project: "shop"})

	s.Require().Nil(svcErr)
	diff, svcErr := s.f.svc.Diff(ctx, "gw-1", "", "")
	s.Require().Nil(svcErr)
	s.True(changeNamed(diff, "application/app-shop").HeldBack, "the rollback stopped holding the change back")
}

// A rollback planned from a master another apply replaced before the rollback held the lease is refused,
// so it never restores from a stale plan.
func TestARollbackPlannedFromAReplacedMasterIsRefused(t *testing.T) {
	planned := "master-a"

	_, svcErr := shipmentOf(&Diff{}, nil, &Master{Hash: "master-b"}, nil, nil,
		applyOptions{plannedOn: &planned})
	_, unchanged := shipmentOf(&Diff{}, nil, &Master{Hash: "master-a"}, nil, nil,
		applyOptions{plannedOn: &planned})

	assert.Equal(t, &ErrorAppliedChanged, svcErr)
	assert.Nil(t, unchanged)
}
