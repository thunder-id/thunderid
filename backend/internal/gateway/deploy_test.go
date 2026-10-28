// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An apply sends the gateway only what its master says it lacks or holds differently, and removes
// only what the master records that the version no longer has.
func TestAnApplySendsOnlyWhatTheMasterLacks(t *testing.T) {
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
func TestASelectionSendsTheChangesNamedAndKeepsNothing(t *testing.T) {
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

// A resource the gateway refused is offered again by every diff, as updated and flagged failed, and
// sent again by the next apply even though the version did not change.
func TestARefusedResourceIsOfferedAgainUntilAccepted(t *testing.T) {
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
func TestARefusalOfARunningResourceIsOfferedAsUpdated(t *testing.T) {
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
func TestAnApplyWithNothingToSendDoesNotCallTheGateway(t *testing.T) {
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
func TestAChangedValueSendsTheResourcesReferringToItAgain(t *testing.T) {
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
func TestAChangedValueStaysOnOfferUntilItsResourceIsSent(t *testing.T) {
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
func TestMissingValuesAreCheckedForWhatIsSentAlone(t *testing.T) {
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
func TestAPartialSelectionIsRecordedAsPartlyApplied(t *testing.T) {
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
func TestARevertReturnsToThePreviousMaster(t *testing.T) {
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
