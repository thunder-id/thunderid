// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
)

// PreviewTestSuite covers what a preview reports, what binds an apply to it, and what it keeps.
type PreviewTestSuite struct {
	suite.Suite
}

func TestPreviewTestSuite(t *testing.T) {
	suite.Run(t, new(PreviewTestSuite))
}

func (s *PreviewTestSuite) SetupTest() {
	config.ResetServerRuntime()
}

func (s *PreviewTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

// A preview reports what a dry run does, the changes, the values the gateway lacks and the removals,
// without sending the gateway an import.
func (s *PreviewTestSuite) TestAPreviewReportsWhatADryRunDoesWithoutAnImport() {
	t := s.T()
	f := newVersionFixture(t)
	f.captureReferring(t)
	f.client.held = map[string]bool{"APP_CLIENT_ID": true}

	preview, svcErr := f.svc.Preview(context.Background(), "gw-1", PreviewRequest{})

	require.Nil(t, svcErr)
	assert.Empty(t, f.client.sent, "a preview sent the gateway an import")
	dry, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{DryRun: true})
	require.Nil(t, svcErr)
	assert.Equal(t, dry.Missing, preview.Missing)
	assert.Equal(t, dry.Diff.Changes, preview.Changes)
	assert.Equal(t, dry.Diff.Summary, preview.Summary)
	assert.Equal(t, &RemovalCount{Threshold: removalConfirmationLimit()}, preview.Removals)
	assert.Nil(t, preview.Changed)
}

// A preview whose removal a held-back update still needs reports the diff with that removal marked,
// rather than refusing it, while the apply still refuses the removal until the update goes with it.
func (s *PreviewTestSuite) TestAPreviewMarksARemovalAHeldBackUpdateStillNeeds() {
	t := s.T()
	ctx := context.Background()
	f := newVersionFixture(t)
	f.exporter.response = documents(unit("ou-1"), flow("flow-1"), app("app-1", "ou-1", "flow-1"))
	_, svcErr := f.svc.Capture(ctx, CaptureRequest{})
	require.Nil(t, svcErr)
	_, svcErr = f.svc.Apply(ctx, "gw-1", ApplyRequest{})
	require.Nil(t, svcErr)
	f.exporter.response = documents(unit("ou-1"), flow("flow-2"), app("app-1", "ou-1", "flow-2"))
	_, svcErr = f.svc.Capture(ctx, CaptureRequest{})
	require.Nil(t, svcErr)
	_, svcErr = f.svc.Apply(ctx, "gw-1", ApplyRequest{Selection: &[]string{"flow/flow-2"},
		LeaveOut: []string{"application/app-1"}})
	require.Nil(t, svcErr)
	sent := len(f.client.sent)

	preview, svcErr := f.svc.Preview(ctx, "gw-1", PreviewRequest{})

	require.Nil(t, svcErr)
	assert.Equal(t, []RemovalInUse{{Key: "flow/flow-1", NeededBy: "application/app-1"}}, preview.RemovalsInUse)
	assert.Len(t, f.client.sent, sent, "a preview sent the gateway an import")
	_, svcErr = f.svc.Apply(ctx, "gw-1", ApplyRequest{})
	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorRemovalInUse.Code, svcErr.Code)
	_, svcErr = f.svc.Apply(ctx, "gw-1", ApplyRequest{Selection: &[]string{"flow/flow-1", "application/app-1"},
		Plan: preview.Plan})
	assert.Nil(t, svcErr)
}

// A preview counts the removals the apply sends and says when they need confirming.
func (s *PreviewTestSuite) TestAPreviewCountsTheRemovalsAgainstTheThreshold() {
	t := s.T()
	keepVersions(t, 10)
	limit := 1
	config.GetServerRuntime().Config.Gateway.RemovalConfirmationThreshold = &limit
	f := newVersionFixture(t)
	f.capture(t, "ou-a", "ou-b", "ou-c")
	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.Nil(t, svcErr)
	f.capture(t, "ou-a")

	preview, svcErr := f.svc.Preview(context.Background(), "gw-1", PreviewRequest{})

	require.Nil(t, svcErr)
	assert.Equal(t, &RemovalCount{Count: 2, Threshold: 1, NeedsConfirmation: true}, preview.Removals)
	assert.Len(t, f.client.sent, 1, "a preview sent the gateway an import")
}

// A preview of a revert reports what the revert would write, without an import.
func (s *PreviewTestSuite) TestAPreviewOfARevertReportsWhatItWouldWrite() {
	t := s.T()
	keepVersions(t, 10)
	f := newVersionFixture(t)
	f.capture(t, "ou-a")
	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.Nil(t, svcErr)
	f.capture(t, "ou-a", "ou-b")
	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.Nil(t, svcErr)
	sent := len(f.client.sent)

	preview, svcErr := f.svc.Preview(context.Background(), "gw-1", PreviewRequest{Revert: true})

	require.Nil(t, svcErr)
	assert.Len(t, f.client.sent, sent, "a preview sent the gateway an import")
	assert.Equal(t, DiffSummary{Deleted: 1, Unchanged: 1}, preview.Summary)
	assert.Equal(t, 1, preview.Removals.Count)
	held, svcErr := f.svc.GetApplied(context.Background(), "gw-1")
	require.Nil(t, svcErr)
	assert.Equal(t, 2, held.AppliedVersion, "a preview of a revert recorded one")
}

// A diff kept for a version and a master is handed out as a copy, so marking one does not mark the next.
func (s *PreviewTestSuite) TestAKeptDiffIsHandedOutAsACopy() {
	t := s.T()
	cache := newPlanCache()
	master := &Master{Hash: "m", Resources: unit("ou-a")}
	target := &Version{Hash: "v", Resources: unit("ou-a") + "\n---\n" + unit("ou-b")}
	computed := 0
	plan := func(held, resources *parsedBundle) *Diff {
		computed++
		return &Diff{Changes: diffBundles(held.resources, resources.resources)}
	}

	first := cache.diff("gw-1", master, target, plan)
	first.Changes[0].Selected = true
	second := cache.diff("gw-1", master, target, plan)

	assert.Equal(t, 1, computed, "a kept diff was worked out again")
	assert.False(t, second.Changes[0].Selected, "a change marked on one copy was marked on the next")
	other := &Master{Hash: "m2", Resources: unit("ou-b")}
	cache.diff("gw-1", other, target, plan)
	assert.Equal(t, 2, computed, "a diff against another master was taken from the kept one")
}

// A bounded cache keeps the values used most recently and has concurrent callers share a computation.
func (s *PreviewTestSuite) TestABoundedCacheKeepsTheRecentAndSharesAComputation() {
	t := s.T()
	cache := newBoundedCache[int](2)
	var computed atomic.Int32
	value := func(v int) func() int {
		return func() int {
			computed.Add(1)
			return v
		}
	}
	assert.Equal(t, 1, cache.get("a", value(1)))
	assert.Equal(t, 2, cache.get("b", value(2)))
	assert.Equal(t, 1, cache.get("a", value(9)), "a kept value was computed again")
	assert.Equal(t, 3, cache.get("c", value(3)))
	assert.Equal(t, 8, cache.get("b", value(8)), "the least recently used value was kept beyond the limit")

	// A caller asking for a key another caller is computing waits on that computation and takes its value,
	// which is kept; when the computation fails, the waiting caller computes the value itself.
	computed.Store(0)
	s.Equal(4, s.joinWhileComputing(cache, "d", func() int {
		computed.Add(1)
		return 4
	}, value(9)), "a caller arriving during a computation")
	s.Equal(int32(1), computed.Load(), "a caller computed although the computation it waited on finished")
	s.Equal(4, cache.get("d", value(9)), "the shared value was not kept")
	computed.Store(0)
	s.Equal(7, s.joinWhileComputing(cache, "e", func() int { panic("the computation failed") }, value(7)),
		"a caller waiting on a computation that failed")
	s.Equal(int32(1), computed.Load())
}

// joinWhileComputing has one caller compute key, and a second ask for it while the first computes. The
// first goes on until the second is seen waiting on a computation, then ends with finish, which may
// panic. It returns what the second caller got. The first caller is released on every way out.
func (s *PreviewTestSuite) joinWhileComputing(cache *boundedCache[int], key string, finish func() int,
	join func() int) int {
	t := s.T()
	release, started := make(chan struct{}), make(chan struct{})
	var once sync.Once
	free := func() { once.Do(func() { close(release) }) }
	defer free()
	go func() {
		defer func() { _ = recover() }()
		cache.get(key, func() int {
			close(started)
			<-release
			return finish()
		})
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("the first caller never computed")
	}
	joined := make(chan int, 1)
	go func() { joined <- cache.get(key, join) }()
	deadline := time.After(5 * time.Second)
	for !waitingOnAComputation() {
		select {
		case result := <-joined:
			t.Fatalf("a caller asking during a computation returned %d without waiting on it", result)
		case <-deadline:
			t.Fatal("a caller asking during a computation was never seen waiting on it")
		case <-time.After(time.Millisecond):
		}
	}
	free()
	select {
	case result := <-joined:
		return result
	case <-time.After(5 * time.Second):
		t.Fatal("a caller waiting on a computation did not return once it ended")
		return 0
	}
}

// waitingOnAComputation reports whether a goroutine is blocked in a bounded cache's get on a channel,
// which is a caller waiting on the computation another caller runs. The caller computing is blocked in
// its own computation instead.
func waitingOnAComputation() bool {
	stacks := make([]byte, 1<<20)
	stacks = stacks[:runtime.Stack(stacks, true)]
	for _, goroutine := range strings.Split(string(stacks), "\n\n") {
		lines := strings.SplitN(goroutine, "\n", 3)
		if len(lines) >= 2 && strings.Contains(lines[0], "[chan receive") &&
			strings.Contains(lines[1], ".(*boundedCache[") && strings.Contains(lines[1], "]).get(") {
			return true
		}
	}
	return false
}

// An apply or a revert bound to a preview's plan goes ahead while the gateway and the version are as
// previewed, and is refused once either changed, so what was reviewed is what is sent.
func (s *PreviewTestSuite) TestAnApplyBoundToAPreviewIsRefusedOnceThePlanChanged() {
	t := s.T()
	keepVersions(t, 10)
	f := newVersionFixture(t)
	f.capture(t, "ou-a", "ou-b")
	stale, svcErr := f.svc.Preview(context.Background(), "gw-1", PreviewRequest{})
	require.Nil(t, svcErr)
	require.NotEmpty(t, stale.Plan)
	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.Nil(t, svcErr)
	sent := len(f.client.sent)

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Plan: stale.Plan})

	assert.Equal(t, &ErrorAppliedChanged, svcErr)
	assert.Len(t, f.client.sent, sent, "an apply off its plan reached the gateway")

	f.capture(t, "ou-a")
	current, svcErr := f.svc.Preview(context.Background(), "gw-1", PreviewRequest{})
	require.Nil(t, svcErr)
	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Plan: current.Plan})
	require.Nil(t, svcErr)

	revert, svcErr := f.svc.Preview(context.Background(), "gw-1", PreviewRequest{Revert: true})
	require.Nil(t, svcErr)
	_, svcErr = f.svc.Revert(context.Background(), "gw-1", RevertRequest{Plan: stale.Plan})
	assert.Equal(t, &ErrorAppliedChanged, svcErr)
	_, svcErr = f.svc.Revert(context.Background(), "gw-1", RevertRequest{Plan: revert.Plan})
	assert.Nil(t, svcErr)
}

// An apply bound to a stale plan is refused before the gateway's store is asked anything, while one on
// its plan still reports a store it cannot read.
func (s *PreviewTestSuite) TestAStalePlanIsRefusedBeforeTheStoreIsRead() {
	t := s.T()
	keepVersions(t, 10)
	f := newVersionFixture(t)
	f.capture(t, "ou-a")
	stale, svcErr := f.svc.Preview(context.Background(), "gw-1", PreviewRequest{})
	require.Nil(t, svcErr)
	f.capture(t, "ou-a", "ou-b")
	current, svcErr := f.svc.Preview(context.Background(), "gw-1", PreviewRequest{})
	require.Nil(t, svcErr)
	f.client.heldErr = errors.New("connection refused")

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Plan: stale.Plan})
	assert.Equal(t, &ErrorAppliedChanged, svcErr)

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Plan: current.Plan})
	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorGatewayUnreachable.Code, svcErr.Code)
}

// A change held back after a preview changes its plan, though the master did not change, since the
// apply would then make other removals.
func (s *PreviewTestSuite) TestHoldingAChangeBackChangesThePlan() {
	t := s.T()
	keepVersions(t, 10)
	f := newVersionFixture(t)
	f.capture(t, "ou-a", "ou-b", "ou-c")
	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.Nil(t, svcErr)
	f.capture(t, "ou-a")
	before, svcErr := f.svc.Preview(context.Background(), "gw-1", PreviewRequest{})
	require.Nil(t, svcErr)
	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Selection: &[]string{},
		LeaveOut: []string{"organization_unit/ou-b"}})
	require.Nil(t, svcErr)

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Plan: before.Plan, ConfirmRemovals: true})

	assert.Equal(t, &ErrorAppliedChanged, svcErr)
}
