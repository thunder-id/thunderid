// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

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

	content, owned := withProjectFrom("shop", now, earlier)

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

	_, _, svcErr := shipmentOf(&Diff{}, &Version{}, &Master{Hash: "master-b"}, nil,
		applyOptions{plannedOn: &planned})
	_, _, unchanged := shipmentOf(&Diff{}, &Version{}, &Master{Hash: "master-a"}, nil,
		applyOptions{plannedOn: &planned})

	assert.Equal(t, &ErrorAppliedChanged, svcErr)
	assert.Nil(t, unchanged)
}
