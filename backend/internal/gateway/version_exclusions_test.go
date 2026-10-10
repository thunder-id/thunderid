// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

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

// excludedKeys reads the keys a gateway is set to leave alone.
func (f *versionFixture) excludedKeys(t *testing.T) []string {
	t.Helper()
	excluded, err := f.versions.GetExcluded(context.Background(), "gw-1")
	require.NoError(t, err)
	keys := []string{}
	for _, resource := range excluded {
		keys = append(keys, resource.Key)
	}
	return keys
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

// Applies to one gateway run one at a time, so the choice one keeps is never interleaved with
// another's import.
func TestAppliesToOneGatewayRunOneAtATime(t *testing.T) {
	var locks gatewayLocks
	unlock := locks.lock("gw-1")
	otherDone := make(chan struct{})
	go func() {
		locks.lock("gw-2")()
		close(otherDone)
	}()
	<-otherDone

	acquired := make(chan struct{})
	go func() {
		locks.lock("gw-1")()
		close(acquired)
	}()
	select {
	case <-acquired:
		t.Fatal("a second apply to the gateway ran while the first held it")
	case <-time.After(50 * time.Millisecond):
	}
	unlock()
	<-acquired
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

// A gateway's lock is kept only while an apply holds it or waits for it.
func TestAGatewaysLockIsDroppedOnceNoApplyNeedsIt(t *testing.T) {
	var locks gatewayLocks
	unlock := locks.lock("gw-1")
	waited := make(chan struct{})
	go func() {
		locks.lock("gw-1")()
		close(waited)
	}()
	require.Eventually(t, func() bool {
		locks.mu.Lock()
		defer locks.mu.Unlock()
		return locks.locks["gw-1"].users == 2
	}, time.Second, time.Millisecond)

	unlock()
	<-waited

	locks.mu.Lock()
	defer locks.mu.Unlock()
	assert.Empty(t, locks.locks)
}
