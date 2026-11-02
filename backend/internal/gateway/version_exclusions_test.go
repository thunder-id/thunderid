// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/export"
)

const (
	keyA = "organization_unit/ou-a"
	keyB = "organization_unit/ou-b"
	keyC = "organization_unit/ou-c"
)

func selecting(keys ...string) *[]string { return &keys }

// excludedKeys reads the keys a gateway is set to leave alone for good.
func (f *versionFixture) excludedKeys(t *testing.T) []string {
	return f.exclusionKeys(t, false)
}

// owedKeys reads the keys a skipped apply left owed to a gateway.
func (f *versionFixture) owedKeys(t *testing.T) []string {
	return f.exclusionKeys(t, true)
}

func (f *versionFixture) exclusionKeys(t *testing.T, once bool) []string {
	t.Helper()
	excluded, err := f.versions.GetExcluded(context.Background(), "gw-1")
	require.NoError(t, err)
	keys := make([]string, 0, len(excluded))
	for _, resource := range excluded {
		if resource.Once == once {
			keys = append(keys, resource.Key)
		}
	}
	return keys
}

// changeByKey finds a change in a diff.
func changeByKey(changes []Change, key string) *Change {
	for i := range changes {
		if changes[i].Key == key {
			return &changes[i]
		}
	}
	return nil
}

// A selection decides only for the changes on offer: an unchanged resource not left alone keeps
// what was chosen for it before, a selected one is taken back, and one left out is held back with
// its identity.
func TestNextExclusionsReconsidersOnlyTheChangesOnOffer(t *testing.T) {
	changes := []Change{
		{Key: keyA, Change: ChangeUnchanged},
		{Key: keyB, Change: ChangeAdded, Excluded: true},
		{Key: keyC, ResourceType: "organization_unit", ID: "ou-c", Name: "ou-c", Change: ChangeDeleted},
	}
	existing := []excludedResource{{Key: "organization_unit/elsewhere"}, {Key: keyB}}

	next, exclude, include := nextExclusions(existing, changes, []string{keyB})

	assert.Equal(t, []excludedResource{
		{Key: "organization_unit/elsewhere"},
		{Key: keyC, Type: "organization_unit", ID: "ou-c", Name: "ou-c"},
	}, next)
	assert.Equal(t, []excludedResource{{Key: keyC, Type: "organization_unit", ID: "ou-c", Name: "ou-c"}}, exclude)
	assert.Equal(t, []string{keyB}, include)
}

// A change left out of a selection is neither written nor, once a later version drops it, removed;
// and the gateway goes on leaving it alone without being asked again.
func TestAResourceLeftOutIsLeftAloneFromThenOn(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a")
	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(1)})
	require.Nil(t, svcErr)
	f.capture(t, "ou-a", "ou-b", "ou-c")

	result, svcErr := f.svc.Apply(context.Background(), "gw-1",
		ApplyRequest{Version: f.hash(2), Selection: selecting(keyB)})

	require.Nil(t, svcErr)
	assert.Contains(t, f.client.last().Content, "ou-b")
	assert.NotContains(t, f.client.last().Content, "ou-c", "a resource left out was written")
	assert.Equal(t, DiffSummary{Added: 1, Unchanged: 1}, result.Diff.Summary)
	assert.Equal(t, []string{keyC}, f.excludedKeys(t))

	diff, svcErr := f.svc.Diff(context.Background(), "gw-1", f.hash(2))
	require.Nil(t, svcErr)
	for _, change := range diff.Changes {
		assert.Equal(t, change.Key == keyC, change.Excluded, change.Key)
	}

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(2)})
	require.Nil(t, svcErr)
	assert.NotContains(t, f.client.last().Content, "ou-c", "the choice was not kept for the next apply")

	f.capture(t, "ou-a", "ou-c")
	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(3)})
	require.Nil(t, svcErr)
	assert.Equal(t, []gatewayDeletion{{ResourceType: "organization_unit", ID: "ou-b"}}, f.client.last().Deletions,
		"a resource left alone was removed")
}

// Leaving a removal out keeps the resource on the gateway, and selecting it again later removes it.
func TestARemovalLeftOutKeepsTheResourceUntilSelected(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a", "ou-b")
	_, _ = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(1)})
	f.capture(t, "ou-a")

	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(2), Selection: selecting()})
	require.Nil(t, svcErr)
	assert.Empty(t, f.client.last().Deletions)

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(2), Selection: selecting(keyB)})
	require.Nil(t, svcErr)
	assert.Equal(t, []gatewayDeletion{{ResourceType: "organization_unit", ID: "ou-b"}}, f.client.last().Deletions)
	assert.Empty(t, f.excludedKeys(t))
}

// An addition left out stays on offer once the gateway records the version, though the version no
// longer reports it as a change, and selecting it then writes it.
func TestAnAdditionLeftOutStaysOnOfferAfterTheVersionIsRecorded(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a", "ou-b")
	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Selection: selecting(keyA)})
	require.Nil(t, svcErr)

	diff, svcErr := f.svc.Diff(context.Background(), "gw-1", "latest")
	require.Nil(t, svcErr)
	assert.Contains(t, diff.Changes, Change{Key: keyB, ResourceType: "organization_unit", ID: "ou-b", Name: "ou-b",
		Change: ChangeUnchanged, Excluded: true})

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Selection: selecting(keyA, keyB)})
	require.Nil(t, svcErr)
	assert.Contains(t, f.client.last().Content, "ou-b")
	assert.Empty(t, f.excludedKeys(t))
}

// A dry run applies its selection to what it checks but keeps nothing.
func TestADryRunSelectionIsNotKept(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a", "ou-b")

	result, svcErr := f.svc.Apply(context.Background(), "gw-1",
		ApplyRequest{DryRun: true, Selection: selecting(keyA)})

	require.Nil(t, svcErr)
	assert.NotContains(t, f.client.last().Content, "ou-b")
	assert.Equal(t, DiffSummary{Added: 1}, result.Diff.Summary)
	assert.Empty(t, f.excludedKeys(t))
}

// A revert leaves alone what the gateway is set to leave alone, as an apply does.
func TestARevertLeavesAloneWhatTheGatewayLeavesAlone(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a", "ou-b")
	f.capture(t, "ou-a")
	_, _ = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(1), Selection: selecting(keyA)})
	_, _ = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(2)})

	_, svcErr := f.svc.Revert(context.Background(), "gw-1", RevertRequest{})

	require.Nil(t, svcErr)
	assert.NotContains(t, f.client.last().Content, "ou-b")
}

// A value only a resource left alone refers to is not needed, so its absence does not stop the apply.
func TestAValueOnlyAResourceLeftAloneNeedsIsNotRequired(t *testing.T) {
	f := newVersionFixture(t)
	f.captureReferring(t)

	_, svcErr := f.svc.Apply(context.Background(), "gw-1",
		ApplyRequest{Selection: selecting()})

	require.Nil(t, svcErr)
	assert.NotContains(t, f.client.last().Content, "app-1")
}

// A selection that cannot be kept stops the apply before the gateway is written.
func TestASelectionThatCannotBeKeptStopsTheApply(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a", "ou-b")
	f.versions.excludeErr = errors.New("database is down")

	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Selection: selecting(keyA)})

	require.NotNil(t, svcErr)
	assert.Empty(t, f.client.sent)
}

// An apply refused for a value the gateway lacks keeps nothing it chose, so the next apply works from
// the choice as it was.
func TestAnApplyRefusedForMissingValuesKeepsNoChoice(t *testing.T) {
	f := newVersionFixture(t)
	f.exporter.response = &export.ExportResponse{Files: []export.ExportFile{
		{Content: unit("ou-a")},
		{Content: "resource_type: application\nid: app-1\nname: app\nclient_id: var:APP_CLIENT_ID\n"},
	}}
	_, svcErr := f.svc.Capture(context.Background(), CaptureRequest{})
	require.Nil(t, svcErr)

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Selection: selecting("application/app-1")})

	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorMissingValues.Code, svcErr.Code)
	assert.Empty(t, f.excludedKeys(t), "a refused apply kept its choice")
	assert.Empty(t, f.client.sent)
}

// An apply the gateway wrote nothing of puts the choice back as it was: what it took back is left
// alone again, and what it began leaving alone is offered again.
func TestAnApplyTheGatewayWroteNothingOfPutsTheChoiceBack(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a", "ou-b")
	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Selection: selecting(keyA)})
	require.Nil(t, svcErr)
	require.Equal(t, []string{keyB}, f.excludedKeys(t))
	f.capture(t, "ou-a", "ou-b", "ou-c")
	f.client.err = fmt.Errorf("%w: connection refused", errImportNotWritten)

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Selection: selecting(keyA, keyB)})

	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorGatewayUnreachable.Code, svcErr.Code)
	assert.Equal(t, []string{keyB}, f.excludedKeys(t), "a failed apply kept its choice")
}

// A selection that turned an owed change into the gateway's standing choice, in an apply the gateway
// wrote nothing of, leaves the change owed again rather than left alone for good.
func TestAnApplyTheGatewayWroteNothingOfLeavesAnOwedChangeOwed(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a")
	_, _ = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(1)})
	f.capture(t, "ou-a", "ou-b")
	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(2), Skip: []string{keyB}})
	require.Nil(t, svcErr)
	require.Equal(t, []string{keyB}, f.owedKeys(t))
	f.client.err = fmt.Errorf("%w: connection refused", errImportNotWritten)

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(2), Selection: selecting()})

	require.NotNil(t, svcErr)
	assert.Equal(t, []string{keyB}, f.owedKeys(t), "the owed change was not owed again")
	assert.Empty(t, f.excludedKeys(t), "the failed apply's choice was kept")
}

// An apply that may have reached the gateway, a timeout or a connection lost after the request went
// out, keeps its choice: the gateway may hold what it was sent, and putting the choice back would
// leave that resource alone in every later diff.
func TestAnApplyThatMayHaveReachedTheGatewayKeepsTheChoice(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a", "ou-b")
	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Selection: selecting(keyA)})
	require.Nil(t, svcErr)
	f.capture(t, "ou-a", "ou-b", "ou-c")
	f.client.err = errors.New("context deadline exceeded")

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Selection: selecting(keyA, keyB)})

	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorGatewayUnreachable.Code, svcErr.Code)
	assert.Equal(t, []string{keyC}, f.excludedKeys(t), "the choice the gateway may hold was put back")
}

// A selection naming a resource the diff does not report is refused before anything is kept or sent:
// a typo or a key from a stale diff would otherwise leave out the resource that was meant.
func TestASelectionNamingAResourceNotInTheDiffIsRefused(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a", "ou-b")
	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.Nil(t, svcErr)
	f.capture(t, "ou-a", "ou-b", "ou-c")
	sent := len(f.client.sent)

	_, svcErr = f.svc.Apply(context.Background(), "gw-1",
		ApplyRequest{Selection: selecting(keyC, "organization_unit/ou-typo")})

	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorInvalidSelection.Code, svcErr.Code)
	assert.Empty(t, f.excludedKeys(t))
	assert.Len(t, f.client.sent, sent, "a refused selection reached the gateway")
}

// A resource too long for the columns that keep it cannot be left out, so leaving it out is refused
// rather than failing in the database; selecting it is fine.
func TestAResourceTooLongToKeepCannotBeLeftOut(t *testing.T) {
	long := strings.Repeat("x", maxResourceField+1)
	f := newVersionFixture(t)
	f.capture(t, "ou-a", long)
	longKey := "organization_unit/" + long

	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Selection: selecting(keyA)})

	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorInvalidSelection.Code, svcErr.Code)

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Selection: selecting(longKey)})

	require.Nil(t, svcErr)
	assert.Equal(t, []string{keyA}, f.excludedKeys(t))
}

// Once the import wrote the gateway, the choice it sent stays even if recording the version fails:
// putting it back would leave alone a resource the gateway was just sent, and a later version that
// drops it would never remove it.
func TestAnApplyThatWroteTheGatewayKeepsTheChoiceWhenRecordingFails(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a", "ou-b")
	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Selection: selecting(keyA)})
	require.Nil(t, svcErr)
	require.Equal(t, []string{keyB}, f.excludedKeys(t))
	f.capture(t, "ou-a", "ou-b", "ou-c")
	f.versions.setErr = errAppliedChanged

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Selection: selecting(keyA, keyB)})

	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorAppliedChanged.Code, svcErr.Code)
	assert.Contains(t, f.client.last().Content, "ou-b", "the import did not send what was taken back")
	assert.Equal(t, []string{keyC}, f.excludedKeys(t), "the choice the gateway was sent was put back")
}

// A removed gateway's choices are forgotten with what it held.
func TestForgettingAGatewayDropsWhatItLeftAlone(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a", "ou-b")
	_, _ = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Selection: selecting(keyA)})

	f.svc.Forget(context.Background(), "gw-1")

	assert.Empty(t, f.excludedKeys(t))
}

// A column's width is counted in characters, so a name of multibyte characters that fits in
// characters but not in bytes can be left out.
func TestAResourceIsMeasuredInCharacters(t *testing.T) {
	name := strings.Repeat("é", maxResourceField)
	require.Greater(t, len(name), maxResourceField)

	assert.True(t, keepable(Change{Key: "organization_unit/ou-1", ResourceType: "organization_unit", ID: "ou-1",
		Name: name}))
	assert.False(t, keepable(Change{Key: "organization_unit/ou-1", ResourceType: "organization_unit", ID: "ou-1",
		Name: name + "é"}))
}

// A dry run says what each missing value is for only among the resources it applies: one left alone
// is not applied, so the values it refers to are not reported.
func TestADryRunReportsReferencesOnlyForWhatItApplies(t *testing.T) {
	f := newVersionFixture(t)
	f.exporter.response = &export.ExportResponse{Files: []export.ExportFile{
		{Content: "resource_type: application\nid: app-1\nname: one\nclient_id: var:APP_ONE_CLIENT_ID\n"},
		{Content: "resource_type: application\nid: app-2\nname: two\nclient_id: var:APP_TWO_CLIENT_ID\n"},
	}}
	_, svcErr := f.svc.Capture(context.Background(), CaptureRequest{})
	require.Nil(t, svcErr)

	result, svcErr := f.svc.Apply(context.Background(), "gw-1",
		ApplyRequest{DryRun: true, Selection: selecting("application/app-1")})

	require.Nil(t, svcErr)
	require.NotNil(t, result.Missing)
	assert.Equal(t, []string{"APP_ONE_CLIENT_ID"}, result.Missing.Variables)
	require.Len(t, result.Missing.References, 1)
	assert.Equal(t, "APP_ONE_CLIENT_ID", result.Missing.References[0].Name)
	assert.Equal(t, "app-1", result.Missing.References[0].ResourceID)
}

// A skipped change is left out of that apply alone: it is neither written nor kept as the gateway's
// choice, but owed to it, so the next diff offers it again and the next apply writes it.
func TestASkippedChangeIsLeftAloneThisOnce(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a")
	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(1)})
	require.Nil(t, svcErr)
	f.capture(t, "ou-a", "ou-b", "ou-c")

	result, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(2), Skip: []string{keyC}})

	require.Nil(t, svcErr)
	assert.True(t, result.Recorded, "an apply with a skip was not recorded")
	assert.Contains(t, f.client.last().Content, "ou-b")
	assert.NotContains(t, f.client.last().Content, "ou-c", "a skipped resource was written")
	assert.Equal(t, DiffSummary{Added: 1, Unchanged: 1}, result.Diff.Summary)
	assert.Empty(t, f.excludedKeys(t), "a skip was kept as the gateway's choice")
	assert.Equal(t, []string{keyC}, f.owedKeys(t), "a skip was not kept as owed to the gateway")

	offered, svcErr := f.svc.Diff(context.Background(), "gw-1", f.hash(2))
	require.Nil(t, svcErr)
	assert.Equal(t, DiffSummary{Updated: 1, Unchanged: 2}, offered.Summary, "the skipped change was not offered again")
	skipped := changeByKey(offered.Changes, keyC)
	require.NotNil(t, skipped)
	assert.Equal(t, ChangeUpdated, skipped.Change)
	assert.True(t, skipped.Skipped, "the change offered again does not say it was skipped")

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(2)})
	require.Nil(t, svcErr)
	assert.Contains(t, f.client.last().Content, "ou-c", "the next apply did not write what was skipped")
	assert.Empty(t, f.owedKeys(t), "what the next apply wrote stayed owed")
}

// A skipped removal is owed too. The version is recorded, so the versions alone would never show the
// removal again; the diff offers it until an apply removes the resource.
func TestASkippedRemovalIsOfferedUntilAnApplyRemovesIt(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a", "ou-b")
	_, _ = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(1)})
	f.capture(t, "ou-a")

	result, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(2), Skip: []string{keyB}})

	require.Nil(t, svcErr)
	assert.True(t, result.Recorded)
	assert.Empty(t, f.client.last().Deletions, "a skipped removal was sent")

	offered, svcErr := f.svc.Diff(context.Background(), "gw-1", f.hash(2))
	require.Nil(t, svcErr)
	assert.Equal(t, DiffSummary{Deleted: 1, Unchanged: 1}, offered.Summary, "the skipped removal was not offered again")
	skipped := changeByKey(offered.Changes, keyB)
	require.NotNil(t, skipped)
	assert.Equal(t, ChangeDeleted, skipped.Change)
	assert.True(t, skipped.Skipped)

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(2), DryRun: true})
	require.Nil(t, svcErr)
	assert.Equal(t, []string{keyB}, f.owedKeys(t), "a dry run cleared what the gateway is owed")

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(2)})
	require.Nil(t, svcErr)
	assert.Equal(t, []gatewayDeletion{{ResourceType: "organization_unit", ID: "ou-b"}}, f.client.last().Deletions,
		"the next apply did not remove what was skipped")
	assert.Empty(t, f.owedKeys(t))
}

// Skipping a change that changes nothing skips nothing, and a dry run owes the gateway nothing.
func TestASkipOfAnUnchangedResourceOrInADryRunOwesNothing(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a")
	_, _ = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(1)})
	f.capture(t, "ou-a", "ou-b")

	dry, svcErr := f.svc.Apply(context.Background(), "gw-1",
		ApplyRequest{Version: f.hash(2), DryRun: true, Skip: []string{keyB}})
	require.Nil(t, svcErr)
	assert.True(t, changeByKey(dry.Diff.Changes, keyB).Excluded, "a dry run did not report the skip")
	assert.Empty(t, f.owedKeys(t), "a dry run owed the gateway a change")

	result, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(2), Skip: []string{keyA}})
	require.Nil(t, svcErr)
	assert.Contains(t, f.client.last().Content, "ou-a", "an unchanged resource named in a skip was left out")
	assert.False(t, changeByKey(result.Diff.Changes, keyA).Excluded)
	assert.Empty(t, f.owedKeys(t), "an unchanged resource was owed")
}

// A selection that leaves out a change owed from a skip makes it the gateway's standing choice.
func TestASelectionTurnsAnOwedChangeIntoAStandingChoice(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a")
	_, _ = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(1)})
	f.capture(t, "ou-a", "ou-b")
	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(2), Skip: []string{keyB}})
	require.Nil(t, svcErr)
	require.Equal(t, []string{keyB}, f.owedKeys(t))

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(2), Selection: selecting()})

	require.Nil(t, svcErr)
	assert.Equal(t, []string{keyB}, f.excludedKeys(t), "the deselected change is not the gateway's choice")
	assert.Empty(t, f.owedKeys(t), "the deselected change is still owed")
}

// Skipping a change the gateway already leaves alone keeps that choice, and a selection decides the
// changes that are not skipped.
func TestASkipKeepsTheStandingChoiceAndASelectionDecidesTheRest(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a")
	_, _ = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(1)})
	f.capture(t, "ou-a", "ou-b", "ou-c")
	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(2), Selection: selecting(keyB)})
	require.Nil(t, svcErr)
	require.Equal(t, []string{keyC}, f.excludedKeys(t))

	_, svcErr = f.svc.Apply(context.Background(), "gw-1",
		ApplyRequest{Version: f.hash(2), Skip: []string{keyC}, Selection: selecting()})

	require.Nil(t, svcErr)
	assert.Equal(t, []string{keyC}, f.excludedKeys(t), "a skip cleared the gateway's choice")
}

// A skip names only changes the diff reports, and never one the selection also names.
func TestAnInvalidSkipStopsTheApply(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a")
	_, _ = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(1)})
	f.capture(t, "ou-a", "ou-b")
	sent := len(f.client.sent)

	_, svcErr := f.svc.Apply(context.Background(), "gw-1",
		ApplyRequest{Version: f.hash(2), Skip: []string{"organization_unit/ou-z"}})
	assert.Equal(t, ErrorInvalidSelection.Code, svcErr.Code, "a skip naming nothing was taken")

	_, svcErr = f.svc.Apply(context.Background(), "gw-1",
		ApplyRequest{Version: f.hash(2), Skip: []string{keyB}, Selection: selecting(keyB)})
	assert.Equal(t, ErrorInvalidSelection.Code, svcErr.Code, "a change both skipped and selected was taken")

	long := strings.Repeat("x", maxResourceField+1)
	f.capture(t, "ou-a", long)
	_, svcErr = f.svc.Apply(context.Background(), "gw-1",
		ApplyRequest{Version: f.hash(3), Skip: []string{"organization_unit/" + long}})
	assert.Equal(t, ErrorInvalidSelection.Code, svcErr.Code, "a resource too long to keep was skipped")
	assert.Len(t, f.client.sent, sent, "an invalid skip reached the gateway")
}
