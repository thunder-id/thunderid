// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// DeployTestSuite covers what an apply sends a gateway, from its master and a selection. Every test
// starts from the package's shipped configuration and leaves it there.
type DeployTestSuite struct {
	suite.Suite
}

func TestDeployTestSuite(t *testing.T) {
	suite.Run(t, new(DeployTestSuite))
}

func (s *DeployTestSuite) SetupTest() {
	config.ResetServerRuntime()
	configureGatewayBound(shippedGatewayBound)
}

func (s *DeployTestSuite) TearDownTest() {
	config.ResetServerRuntime()
	configureGatewayBound(shippedGatewayBound)
}

// An apply sends the gateway only what its master says it lacks or holds differently, and removes
// only what the master records that the version no longer has.
func (s *DeployTestSuite) TestAnApplySendsOnlyWhatTheMasterLacks() {
	t := s.T()
	keepVersions(t, 10)
	f := newVersionFixture(t)
	f.capture(t, "ou-a", "ou-b")
	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.Nil(t, svcErr)
	assert.Contains(t, f.client.last().Content, "ou-a")
	assert.Contains(t, f.client.last().Content, "ou-b")
	f.capture(t, "ou-a", "ou-c")

	result, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})

	require.Nil(t, svcErr)
	assert.Equal(t, unit("ou-c"), f.client.last().Content, "a resource the gateway holds as it is was sent")
	assert.Equal(t, []gatewayDeletion{{ResourceType: "organization_unit", ID: "ou-b"}}, f.client.last().Deletions)
	assert.Equal(t, DiffSummary{Added: 1, Deleted: 1, Unchanged: 1}, result.Diff.Summary)
	assert.Equal(t, f.versions.masters["gw-1"][0].Hash, result.Diff.FromMaster)
	assert.True(t, result.Recorded)
}

// A selection sends the changes named and leaves the rest on offer; nothing about it is kept, so the
// next apply without a selection sends what was left out.
func (s *DeployTestSuite) TestASelectionSendsTheChangesNamedAndKeepsNothing() {
	t := s.T()
	f := newVersionFixture(t)
	f.capture(t, "ou-a", "ou-b")

	result, svcErr := f.svc.Apply(context.Background(), "gw-1",
		ApplyRequest{Selection: &[]string{"organization_unit/ou-a"}})

	require.Nil(t, svcErr)
	assert.Equal(t, unit("ou-a"), f.client.last().Content)
	assert.True(t, result.Recorded)
	selected := map[string]bool{}
	for _, change := range result.Diff.Changes {
		selected[change.Key] = change.Selected
	}
	assert.Equal(t, map[string]bool{"organization_unit/ou-a": true, "organization_unit/ou-b": false}, selected)

	diff, svcErr := f.svc.Diff(context.Background(), "gw-1", "")
	require.Nil(t, svcErr)
	assert.Equal(t, DiffSummary{Added: 1, Unchanged: 1}, diff.Summary, "the change left out is no longer offered")
	for _, change := range diff.Changes {
		assert.False(t, change.Selected, "a selection outlived its apply")
	}

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.Nil(t, svcErr)
	assert.Equal(t, unit("ou-b"), f.client.last().Content, "the change left out was not sent by the next apply")

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Selection: &[]string{"organization_unit/ou-z"}})
	assert.Equal(t, &ErrorInvalidSelection, svcErr, "a key the diff does not report was accepted")
}

// A change an apply leaves out is held back from the applies after it, on that gateway alone, until
// one selects it again.
func (s *DeployTestSuite) TestAChangeLeftOutIsHeldBackUntilSelected() {
	t := s.T()
	keepVersions(t, 10)
	f := newVersionFixture(t)
	f.capture(t, "ou-a", "ou-b")

	result, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{
		Selection: &[]string{"organization_unit/ou-a"}, LeaveOut: []string{"organization_unit/ou-b"}})

	require.Nil(t, svcErr)
	assert.Equal(t, unit("ou-a"), f.client.last().Content)
	diff, svcErr := f.svc.Diff(context.Background(), "gw-1", "")
	require.Nil(t, svcErr)
	held := map[string]bool{}
	for _, change := range diff.Changes {
		held[change.Key] = change.HeldBack
	}
	assert.Equal(t, map[string]bool{"organization_unit/ou-a": false, "organization_unit/ou-b": true}, held)
	assert.True(t, result.Recorded)

	// A later version changes both; an apply without a selection sends only what is not held back.
	f.capture(t, "ou-a", "ou-b", "ou-c")
	calls := len(f.client.sent)
	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.Nil(t, svcErr)
	require.Len(t, f.client.sent, calls+1)
	assert.Equal(t, unit("ou-c"), f.client.last().Content, "a held back change was sent")

	// Selecting it sends it and stops holding it back.
	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Selection: &[]string{"organization_unit/ou-b"}})
	require.Nil(t, svcErr)
	assert.Equal(t, unit("ou-b"), f.client.last().Content)
	diff, _ = f.svc.Diff(context.Background(), "gw-1", "")
	for _, change := range diff.Changes {
		assert.False(t, change.HeldBack, "a selected change stayed held back")
	}
	assert.Empty(t, f.versions.excluded["gw-1"])
}

// What is left out must be a change the diff reports, and cannot be selected in the same apply.
func (s *DeployTestSuite) TestLeavingOutAChangeTheDiffDoesNotReportIsRefused() {
	t := s.T()
	f := newVersionFixture(t)
	f.capture(t, "ou-a", "ou-b")

	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{LeaveOut: []string{"organization_unit/ou-z"}})
	assert.Equal(t, &ErrorInvalidSelection, svcErr)

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{
		Selection: &[]string{"organization_unit/ou-a"}, LeaveOut: []string{"organization_unit/ou-a"}})
	assert.Equal(t, &ErrorInvalidSelection, svcErr)
	assert.Empty(t, f.versions.excluded["gw-1"])
}

// A resource the gateway refused is offered again by every diff, as updated and flagged failed, and
// sent again by the next apply even though the version did not change.
func (s *DeployTestSuite) TestARefusedResourceIsOfferedAgainUntilAccepted() {
	t := s.T()
	f := newVersionFixture(t)
	f.capture(t, "ou-a", "ou-b")
	f.client.imported = answerOf(wrote("ou-a"), refused("ou-b", "ou-b", "OU-1"))
	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.Nil(t, svcErr)

	diff, svcErr := f.svc.Diff(context.Background(), "gw-1", "")
	require.Nil(t, svcErr)
	require.Len(t, diff.Changes, 2)
	assert.Equal(t, ChangeUnchanged, diff.Changes[0].Change)
	assert.Equal(t, ChangeAdded, diff.Changes[1].Change, "a resource the gateway never accepted is not an addition")
	assert.True(t, diff.Changes[1].Failed)

	f.client.imported = nil
	result, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.Nil(t, svcErr)
	assert.Equal(t, unit("ou-b"), f.client.last().Content, "only the refused resource is sent again")
	master, _ := f.svc.GetMaster(context.Background(), "gw-1")
	assert.Equal(t, MasterSummary{Deployed: 2}, master.Summary)
	assert.Equal(t, master.Hash, result.Master)

	diff, _ = f.svc.Diff(context.Background(), "gw-1", "")
	assert.Equal(t, DiffSummary{Unchanged: 2}, diff.Summary)
	assert.False(t, diff.Changes[1].Failed)
}

// A refusal of a resource the gateway already runs is offered as updated although the version and the
// master hold the same document, since the gateway does not run what was last sent.
func (s *DeployTestSuite) TestARefusalOfARunningResourceIsOfferedAsUpdated() {
	t := s.T()
	f := newVersionFixture(t)
	f.capture(t, "ou-a")
	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.Nil(t, svcErr)
	f.versions.masters["gw-1"][0].Entries[0].State = EntryFailed

	diff, svcErr := f.svc.Diff(context.Background(), "gw-1", "")

	require.Nil(t, svcErr)
	require.Len(t, diff.Changes, 1)
	assert.Equal(t, ChangeUpdated, diff.Changes[0].Change)
	assert.True(t, diff.Changes[0].Failed)
	assert.Nil(t, diff.Changes[0].Lines, "a document that did not change has lines")
}

// An apply with nothing to send does not call the gateway, records the version as applied, and
// leaves the master as it is.
func (s *DeployTestSuite) TestAnApplyWithNothingToSendDoesNotCallTheGateway() {
	t := s.T()
	keepVersions(t, 2)
	f := newVersionFixture(t)
	f.capture(t, "ou-a")
	first, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.Nil(t, svcErr)
	require.Len(t, f.client.sent, 1)

	again, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})

	require.Nil(t, svcErr)
	assert.Len(t, f.client.sent, 1, "the gateway was called with nothing to send")
	assert.True(t, again.Recorded)
	assert.Equal(t, first.Master, again.Master)
	masters, _ := f.svc.ListMasters(context.Background(), "gw-1")
	assert.Len(t, masters, 1, "a master was written for an apply that sent nothing")
}

// A value set on the gateway again since the last apply sends the resources that refer to it again,
// marked as such, so the gateway reads the new value although no document changed.
func (s *DeployTestSuite) TestAChangedValueSendsTheResourcesReferringToItAgain() {
	t := s.T()
	f := newVersionFixture(t)
	f.captureReferring(t)
	f.client.held = map[string]bool{"APP_CLIENT_ID": true, "APP_SECRET": true}
	set := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	f.client.heldSince = map[string]time.Time{"APP_CLIENT_ID": set, "APP_SECRET": set}
	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.Nil(t, svcErr)
	f.client.heldSince["APP_SECRET"] = set.Add(time.Minute)

	result, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})

	require.Nil(t, svcErr)
	require.Len(t, f.client.sent, 2)
	assert.Contains(t, f.client.last().Content, "app-1")
	require.Len(t, result.Diff.Changes, 1)
	assert.Equal(t, ChangeUpdated, result.Diff.Changes[0].Change)
	assert.True(t, result.Diff.Changes[0].ValueChanged)
	assert.Equal(t, []string{"APP_SECRET"}, result.Changed.Secrets)

	// The master keeps the entry as it was: the document the gateway runs did not change.
	master, _ := f.svc.GetMaster(context.Background(), "gw-1")
	assert.Equal(t, f.hash(1), master.Entries[0].SourceVersion)
	assert.Equal(t, EntryDeployed, master.Entries[0].State)
}

// A value set again stays on offer while a selection leaves out the resource that refers to it: the
// next apply without a selection sends that resource.
func (s *DeployTestSuite) TestAChangedValueStaysOnOfferUntilItsResourceIsSent() {
	t := s.T()
	f := newVersionFixture(t)
	f.captureReferring(t)
	f.client.held = map[string]bool{"APP_CLIENT_ID": true, "APP_SECRET": true}
	set := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	f.client.heldSince = map[string]time.Time{"APP_CLIENT_ID": set, "APP_SECRET": set}
	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.Nil(t, svcErr)
	f.client.heldSince["APP_SECRET"] = set.Add(time.Minute)

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Selection: &[]string{}})
	require.Nil(t, svcErr)
	require.Len(t, f.client.sent, 1, "an empty selection sent something")

	result, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})

	require.Nil(t, svcErr)
	require.Len(t, f.client.sent, 2, "the resource referring to the changed value was not sent")
	assert.Contains(t, f.client.last().Content, "app-1")
	require.Len(t, result.Diff.Changes, 1)
	assert.True(t, result.Diff.Changes[0].ValueChanged)

	// Once sent, the new time is recorded and nothing is offered again.
	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.Nil(t, svcErr)
	assert.Len(t, f.client.sent, 2, "the resource was sent again after the new value was recorded")
}

// Missing values are checked for what is sent alone: a resource left on offer needs nothing yet.
func (s *DeployTestSuite) TestMissingValuesAreCheckedForWhatIsSentAlone() {
	t := s.T()
	f := newVersionFixture(t)
	f.captureReferring(t)
	f.client.held = map[string]bool{}

	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Selection: &[]string{}})

	require.Nil(t, svcErr, "an apply sending nothing was refused for the values it does not need")
	dry, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{DryRun: true})
	require.Nil(t, svcErr)
	assert.Equal(t, []string{"APP_CLIENT_ID"}, dry.Missing.Variables)
	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	assert.Equal(t, &ErrorMissingValues, svcErr)
}

// A selection that leaves changes out records the version as partly applied, with how many of its
// resources the gateway holds; the gateway's configuration shows what it holds, not the version
// whole; and the apply that sends the rest records the version complete.
func (s *DeployTestSuite) TestAPartialSelectionIsRecordedAsPartlyApplied() {
	t := s.T()
	f := newAppliedConfigurationFixture(t, unit("ou-a"), unit("ou-b"))
	f.versions.applied = nil

	result, svcErr := f.versionFixture.svc.Apply(context.Background(), "gw-1",
		ApplyRequest{Selection: &[]string{"organization_unit/ou-a"}})

	require.Nil(t, svcErr)
	assert.True(t, result.Recorded)
	assert.False(t, result.Complete)
	held, _ := f.versionFixture.svc.GetApplied(context.Background(), "gw-1")
	assert.Equal(t, f.hash(1), held.Applied)
	assert.False(t, held.Complete)
	assert.Equal(t, [2]int{1, 2}, [2]int{held.ResourcesHeld, held.ResourcesTotal})
	assert.Equal(t, result.Master, held.Master)

	configuration, svcErr := f.svc.GetAppliedConfiguration(context.Background(), "gw-1")
	require.Nil(t, svcErr)
	assert.False(t, configuration.Complete)
	require.Len(t, configuration.Resources, 1, "a resource the gateway never received was shown")
	assert.Equal(t, "ou-a", configuration.Resources[0].ID)
	assert.Equal(t, f.hash(1), configuration.Resources[0].SourceVersion)

	result, svcErr = f.versionFixture.svc.Apply(context.Background(), "gw-1", ApplyRequest{})

	require.Nil(t, svcErr)
	assert.Equal(t, unit("ou-b"), f.client.last().Content, "the rest was not what was sent")
	assert.True(t, result.Complete)
	held, _ = f.versionFixture.svc.GetApplied(context.Background(), "gw-1")
	assert.True(t, held.Complete)
	assert.Equal(t, [2]int{2, 2}, [2]int{held.ResourcesHeld, held.ResourcesTotal})
}

// A revert returns the gateway to the master it held before, so a version that was applied in part is
// returned to in part, each resource keeping the version that sent it.
func (s *DeployTestSuite) TestARevertReturnsToThePreviousMaster() {
	t := s.T()
	keepVersions(t, 10)
	f := newVersionFixture(t)
	f.capture(t, "ou-a", "ou-b")
	f.capture(t, "ou-c")
	_, svcErr := f.svc.Apply(context.Background(), "gw-1",
		ApplyRequest{Version: f.hash(1), Selection: &[]string{"organization_unit/ou-a"}})
	require.Nil(t, svcErr)
	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(2)})
	require.Nil(t, svcErr)

	result, svcErr := f.svc.Revert(context.Background(), "gw-1", RevertRequest{})

	require.Nil(t, svcErr)
	assert.True(t, result.Recorded)
	assert.Equal(t, unit("ou-a"), f.client.last().Content, "the revert sent more than the earlier master held")
	assert.Equal(t, []gatewayDeletion{{ResourceType: "organization_unit", ID: "ou-c"}}, f.client.last().Deletions)
	held, _ := f.svc.GetApplied(context.Background(), "gw-1")
	assert.Equal(t, [2]int{1, 0}, [2]int{held.AppliedVersion, held.PreviousVersion})
	assert.False(t, held.Complete)
	assert.Equal(t, [2]int{1, 2}, [2]int{held.ResourcesHeld, held.ResourcesTotal})
	master, _ := f.svc.GetMaster(context.Background(), "gw-1")
	entry, ok := entryOf(master, "organization_unit/ou-a")
	require.True(t, ok)
	assert.Equal(t, f.hash(1), entry.SourceVersion)
	_, ok = entryOf(master, "organization_unit/ou-b")
	assert.False(t, ok, "a resource the earlier master never held was recorded")
}

// A hold the caller asked for is part of what the apply leaves: when it cannot be kept, the apply
// fails and records nothing, as it does when the master cannot be recorded, rather than reporting the
// change recorded with the hold silently missing.
func (s *DeployTestSuite) TestAHoldThatCannotBeKeptFailsTheApply() {
	t := s.T()
	f := newVersionFixture(t)
	f.capture(t, "ou-a", "ou-b")
	f.versions.heldBackErr = errors.New("database is down")

	result, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{
		Selection: &[]string{"organization_unit/ou-a"}, LeaveOut: []string{"organization_unit/ou-b"}})

	assert.Equal(t, &tidcommon.InternalServerError, svcErr)
	assert.Nil(t, result)
	assert.Empty(t, f.versions.masters["gw-1"], "a master was recorded without the hold")
	assert.Nil(t, f.versions.applied["gw-1"], "the version was recorded without the hold")
}

// A resource the gateway already holds as the version has it cannot be left out: there is nothing to
// hold back, and a hold on it would show nowhere until the resource changed.
func (s *DeployTestSuite) TestLeavingOutAnUnchangedResourceIsRefused() {
	t := s.T()
	f := newVersionFixture(t)
	f.capture(t, "ou-a", "ou-b")
	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.Nil(t, svcErr)
	f.capture(t, "ou-a", "ou-b", "ou-c")

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{LeaveOut: []string{"organization_unit/ou-a"}})

	assert.Equal(t, &ErrorInvalidSelection, svcErr)
	assert.Empty(t, f.versions.excluded["gw-1"], "a hold was kept on a resource that did not change")
}

// Removals the gateway holds back are not sent, so the check an apply of the latest version passes
// before it is queued does not count them either.
func (s *DeployTestSuite) TestHeldBackRemovalsAreNotCountedBeforeQueueing() {
	t := s.T()
	shortLeaseWait(t)
	f := newVersionFixture(t)
	f.capture(t, "ou-a", "ou-b", "ou-c", "ou-d", "ou-e", "ou-f", "ou-g")
	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.Nil(t, svcErr)
	f.capture(t, "ou-a")
	// Every removal but one is held back, so an apply removes one unit: under the threshold.
	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Selection: &[]string{},
		LeaveOut: []string{"organization_unit/ou-b", "organization_unit/ou-c", "organization_unit/ou-d",
			"organization_unit/ou-e", "organization_unit/ou-f", "organization_unit/ou-g"}})
	require.Nil(t, svcErr)
	f.versions.leases["gw-1"].Token, f.versions.leases["gw-1"].Since = "a-node-elsewhere", time.Now()

	result, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})

	require.Nil(t, svcErr, "an apply that removes only what is not held back was refused")
	assert.True(t, result.Queued)
	f.versions.leases["gw-1"].PendingLatest = false
}

// A change too long for the columns that keep a hold is refused before anything is sent, as a key
// the diff does not report is, rather than failing the apply after the gateway has written it.
func (s *DeployTestSuite) TestLeavingOutAChangeTooLongToKeepIsRefused() {
	t := s.T()
	f := newVersionFixture(t)
	long := strings.Repeat("x", maxResourceField+1)
	f.capture(t, "ou-a", long)

	_, svcErr := f.svc.Apply(context.Background(), "gw-1",
		ApplyRequest{LeaveOut: []string{"organization_unit/" + long}})

	assert.Equal(t, &ErrorInvalidSelection, svcErr)
	assert.Empty(t, f.client.sent, "something was sent before the refusal")
	assert.Empty(t, f.versions.excluded["gw-1"])
}

// A row an apply made before masters were recorded, to skip a resource once, was owed to the gateway
// rather than a standing choice: it is not held back, while a resource the gateway was set to leave
// alone is.
func (s *DeployTestSuite) TestASkipOwedBeforeMastersIsNotHeldBack() {
	t := s.T()
	f := newVersionFixture(t)
	f.capture(t, "ou-a")
	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.Nil(t, svcErr)
	f.capture(t, "ou-a", "ou-b", "ou-c")
	f.versions.excluded = map[string]map[string]excludedResource{"gw-1": {
		"organization_unit/ou-b": {Key: "organization_unit/ou-b", Type: "organization_unit", ID: "ou-b", Once: true},
		"organization_unit/ou-c": {Key: "organization_unit/ou-c", Type: "organization_unit", ID: "ou-c"},
	}}

	diff, svcErr := f.svc.Diff(context.Background(), "gw-1", "")
	require.Nil(t, svcErr)
	held := map[string]bool{}
	for _, change := range diff.Changes {
		held[change.Key] = change.HeldBack
	}
	assert.Equal(t, map[string]bool{"organization_unit/ou-a": false, "organization_unit/ou-b": false,
		"organization_unit/ou-c": true}, held)

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.Nil(t, svcErr)
	assert.Equal(t, unit("ou-b"), f.client.last().Content, "the skip owed once was held back, or the hold was not")
}

// A gateway recorded before masters were, or whose earlier master is no longer kept, is returned to
// the earlier version whole: there is no earlier master to return it to.
func (s *DeployTestSuite) TestARevertWithoutAnEarlierMasterReturnsToTheVersionWhole() {
	t := s.T()
	keepVersions(t, 10)
	for name, forget := range map[string]func(f *versionFixture){
		"recorded before masters": func(f *versionFixture) { f.versions.applied["gw-1"].PreviousMasterSeq = 0 },
		"earlier master pruned": func(f *versionFixture) {
			f.versions.masters["gw-1"] = f.versions.masters["gw-1"][1:]
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newVersionFixture(t)
			f.capture(t, "ou-a", "ou-b")
			f.capture(t, "ou-c")
			_, svcErr := f.svc.Apply(context.Background(), "gw-1",
				ApplyRequest{Version: f.hash(1), Selection: &[]string{"organization_unit/ou-a"}})
			require.Nil(t, svcErr)
			_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(2)})
			require.Nil(t, svcErr)
			forget(f)

			result, svcErr := f.svc.Revert(context.Background(), "gw-1", RevertRequest{})

			require.Nil(t, svcErr)
			assert.True(t, result.Recorded)
			assert.Equal(t, joinBundle([]string{unit("ou-a"), unit("ou-b")}), f.client.last().Content,
				"the revert sent less than the earlier version whole")
			assert.Equal(t, []gatewayDeletion{{ResourceType: "organization_unit", ID: "ou-c"}},
				f.client.last().Deletions)
			held, _ := f.svc.GetApplied(context.Background(), "gw-1")
			assert.Equal(t, [2]int{1, 0}, [2]int{held.AppliedVersion, held.PreviousVersion})
			assert.True(t, held.Complete, "the earlier version was not recorded as held whole")
			assert.Equal(t, [2]int{2, 2}, [2]int{held.ResourcesHeld, held.ResourcesTotal})
		})
	}
}
