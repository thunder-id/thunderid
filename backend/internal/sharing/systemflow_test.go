// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	flowcommon "github.com/thunder-id/thunderid/internal/flow/common"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

const (
	testUnsharePreFlow  = "system-test-unshare-pre"
	testUnshareResource = "targetTestId"
)

// revokingDeclaration is the test type with tokens revoked in the units that lose it.
type revokingDeclaration struct {
	testDeclaration
}

// The test types must keep implementing UnshareRevoker, which is only found by type assertion.
var (
	_ UnshareRevoker = (*revokingDeclaration)(nil)
	_ UnshareRevoker = (*everyUnitDeclaration)(nil)
)

func (d *revokingDeclaration) UnshareRevocationFlow() UnshareFlow {
	return UnshareFlow{Handle: testUnsharePreFlow, ResourceInput: testUnshareResource}
}

// everyUnitDeclaration is the test type whose pre flow covers every unit it loses in one run.
type everyUnitDeclaration struct {
	testDeclaration
}

func (d *everyUnitDeclaration) UnshareRevocationFlow() UnshareFlow {
	return UnshareFlow{Handle: testUnsharePreFlow, ResourceInput: testUnshareResource, EveryUnit: true}
}

type SystemFlowTestSuite struct {
	suite.Suite
	store *storeState
	flows *systemFlowProviderMock
	svc   *sharingService
	// preFlowOUs lists, in order, the units a pre flow ran for.
	preFlowOUs []string
}

func TestSystemFlowTestSuite(t *testing.T) {
	suite.Run(t, new(SystemFlowTestSuite))
}

func (s *SystemFlowTestSuite) SetupTest() {
	storeMock, storeState := newMockStore(s.T())
	s.store = storeState
	s.flows = newSystemFlowProviderMock(s.T())
	s.preFlowOUs = nil
	hierarchy, enumerator := testResolver(s.T())
	svc := newSharingService(storeMock, newFileBasedStore(), hierarchy, enumerator,
		rollbackTx(s.T(), storeState), nil, nil, false)
	svc.RegisterResourceType(&revokingDeclaration{})
	svc.RegisterResourceType(&fieldlessDeclaration{})
	s.Require().NoError(SetSystemFlowProvider(svc, s.flows))
	s.svc = svc.(*sharingService)
}

// expectFlows answers every pre flow with a plan naming its unit, and every post flow with nothing.
func (s *SystemFlowTestSuite) expectFlows(preErr, postErr *tidcommon.ServiceError) {
	s.flows.EXPECT().ExecuteSystemFlow(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, sysCtx *flowcommon.SystemFlowContext) (*flowcommon.SystemFlowResult,
			*tidcommon.ServiceError) {
			ouID := sysCtx.Inputs[systemFlowInputOU]
			switch sysCtx.Handle {
			case testUnsharePreFlow:
				s.Equal(testResource, sysCtx.Inputs[testUnshareResource])
				s.preFlowOUs = append(s.preFlowOUs, ouID)
				if preErr != nil {
					return nil, preErr
				}
				return &flowcommon.SystemFlowResult{RuntimeData: map[string]string{"plan": ouID}}, nil
			case systemFlowRevocationPost:
				s.Equal(map[string]string{"plan": ouID}, sysCtx.RuntimeData, "the post flow continues its pre flow")
				if postErr != nil {
					return nil, postErr
				}
				return &flowcommon.SystemFlowResult{}, nil
			}
			s.Failf("unexpected system flow", "%s", sysCtx.Handle)
			return nil, nil
		})
}

// sharedWith has the owner share the test resource into the named tree roots, each with its subtree.
func (s *SystemFlowTestSuite) sharedWith(rootOUIDs ...string) Policy {
	named := make([]RootUnitRequest, 0, len(rootOUIDs))
	for _, ouID := range rootOUIDs {
		named = append(named, root(ouID))
	}
	s.flows.AssertNotCalled(s.T(), "ExecuteSystemFlow", mock.Anything, mock.Anything)
	p, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU,
		PolicyRequest{OtherTreeRoots: roots(named...)})
	s.Require().Nil(svcErr)
	return p
}

// lossOf is the policy a recorded loss names: one of the resource's, owned by the owner.
func lossOf(rt ResourceType, resourceID string) Policy {
	return Policy{ResourceType: rt, ResourceID: resourceID, OwningOUID: ownerOU}
}

func (s *SystemFlowTestSuite) visible(ouID string) bool {
	visible, svcErr := s.svc.IsVisible(context.Background(), testType, testResource, ouID)
	s.Require().Nil(svcErr)
	return visible
}

func (s *SystemFlowTestSuite) TestDeleteRevokesInEveryUnitItTakesTheResourceFromBeforeDeleting() {
	p := s.sharedWith(rootOU, otherOU)
	s.flows.EXPECT().ExecuteSystemFlow(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, sysCtx *flowcommon.SystemFlowContext) (*flowcommon.SystemFlowResult,
			*tidcommon.ServiceError) {
			if sysCtx.Handle == testUnsharePreFlow {
				_, stillThere := s.store.policies[p.ID]
				s.True(stillThere, "a pre flow runs before the change is written")
				s.preFlowOUs = append(s.preFlowOUs, sysCtx.Inputs[systemFlowInputOU])
				return &flowcommon.SystemFlowResult{RuntimeData: map[string]string{
					"plan": sysCtx.Inputs[systemFlowInputOU]}}, nil
			}
			_, stillThere := s.store.policies[p.ID]
			s.False(stillThere, "a post flow runs after the change is written")
			return &flowcommon.SystemFlowResult{}, nil
		})

	s.Require().Nil(s.svc.DeletePolicy(context.Background(), p.ID))

	// A named root reaches the root alone, so the units beneath it never saw the resource and hold nothing.
	s.Equal([]string{otherOU, rootOU}, s.preFlowOUs, "one pre flow per unit, in a stable order")
	s.flows.AssertNumberOfCalls(s.T(), "ExecuteSystemFlow", 4)
	s.False(s.visible(rootOU))
	s.False(s.visible(otherOU))
}

func (s *SystemFlowTestSuite) TestANarrowingEditRevokesOnlyInTheUnitItDrops() {
	p := s.sharedWith(rootOU, otherOU)
	s.expectFlows(nil, nil)

	_, svcErr := s.svc.UpdatePolicy(context.Background(), p.ID, PolicyRequest{
		Version: p.Version, OtherTreeRoots: roots(root(rootOU))})

	s.Require().Nil(svcErr)
	s.Equal([]string{otherOU}, s.preFlowOUs)
	s.True(s.visible(rootOU))
	s.False(s.visible(otherOU))
}

func (s *SystemFlowTestSuite) TestAnEditThatTakesNothingAwayRunsNoFlow() {
	p := s.sharedWith(rootOU)

	_, svcErr := s.svc.UpdatePolicy(context.Background(), p.ID, PolicyRequest{
		Version: p.Version, OtherTreeRoots: roots(root(rootOU), root(otherOU))})

	s.Require().Nil(svcErr)
	s.flows.AssertNotCalled(s.T(), "ExecuteSystemFlow", mock.Anything, mock.Anything)
	s.True(s.visible(otherOU))
}

func (s *SystemFlowTestSuite) TestAFailedPreFlowLeavesTheShareInPlace() {
	p := s.sharedWith(otherOU)
	refusal := &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "RVS-1001"}
	s.expectFlows(refusal, nil)

	svcErr := s.svc.DeletePolicy(context.Background(), p.ID)

	s.Equal(refusal, svcErr)
	s.Contains(s.store.policies, p.ID)
	s.True(s.visible(otherOU))
	s.flows.AssertNumberOfCalls(s.T(), "ExecuteSystemFlow", 1)
}

func (s *SystemFlowTestSuite) TestAFailedPostFlowIsReportedAfterTheChange() {
	p := s.sharedWith(otherOU)
	failure := &tidcommon.InternalServerError
	s.expectFlows(nil, failure)

	svcErr := s.svc.DeletePolicy(context.Background(), p.ID)

	s.Equal(failure, svcErr)
	s.NotContains(s.store.policies, p.ID)
}

func (s *SystemFlowTestSuite) TestARefusedChangeRunsNoFlowAndIsReported() {
	p := s.sharedWith(otherOU)

	_, svcErr := s.svc.UpdatePolicy(context.Background(), p.ID, PolicyRequest{
		Version: p.Version + 1, OtherTreeRoots: roots(root(rootOU))})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorVersionMismatch.Code, svcErr.Code)
	s.flows.AssertNotCalled(s.T(), "ExecuteSystemFlow", mock.Anything, mock.Anything)
	s.True(s.visible(otherOU))
}

func (s *SystemFlowTestSuite) TestATypeThatRevokesNothingRunsNoFlow() {
	p, svcErr := s.svc.CreatePolicy(context.Background(), fieldlessType, testResource, ownerOU,
		PolicyRequest{OtherTreeRoots: roots(root(otherOU))})
	s.Require().Nil(svcErr)

	s.Require().Nil(s.svc.DeletePolicy(context.Background(), p.ID))

	s.flows.AssertNotCalled(s.T(), "ExecuteSystemFlow", mock.Anything, mock.Anything)
}

func (s *SystemFlowTestSuite) TestWithoutSystemFlowsTheChangeRunsOnce() {
	p := s.sharedWith(otherOU)
	s.svc.systemFlows = nil

	s.Require().Nil(s.svc.DeletePolicy(context.Background(), p.ID))

	s.NotContains(s.store.policies, p.ID)
}

func (s *SystemFlowTestSuite) TestAnImportRemovalRevokesAndReportsWhatItRemovedOnce() {
	p := s.sharedWith(otherOU)
	s.expectFlows(nil, nil)
	ctx, report := WithReplayReport(context.Background())

	replay, replayErr := s.svc.RemovePolicies(ctx, testType, testResource, []string{""}, nil)

	s.Require().Nil(replayErr)
	s.Equal([]string{otherOU}, s.preFlowOUs)
	s.Equal([]RemovedPolicy{{PolicyID: p.ID, InitiatingOUID: ownerOU}}, replay.Removed)
	s.Empty(report.Removed(), "a removal reports through its replay only")
}

func (s *SystemFlowTestSuite) TestAnImportThatNarrowsRevokesWhatItDrops() {
	s.sharedWith(rootOU, otherOU)
	s.expectFlows(nil, nil)

	_, replayErr := s.svc.ReplacePoliciesForResource(context.Background(), testType, testResource, ownerOU,
		[]PolicyRequest{{OtherTreeRoots: roots(root(rootOU))}})

	s.Require().Nil(replayErr)
	s.Equal([]string{otherOU}, s.preFlowOUs)
}

func (s *SystemFlowTestSuite) TestAFailedImportRevocationIsReportedAsTheReplayError() {
	p := s.sharedWith(otherOU)
	refusal := &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "RVS-1001"}
	s.expectFlows(refusal, nil)

	_, replayErr := s.svc.RemovePolicies(context.Background(), testType, testResource, []string{""}, nil)

	s.Require().NotNil(replayErr)
	s.Equal(refusal, replayErr.Err)
	s.Contains(s.store.policies, p.ID)
}

// A post flow that fails leaves the others to run, so one unit's failure does not leave another's revocation
// unfinished, and the failure is still reported.
func (s *SystemFlowTestSuite) TestEveryPostFlowRunsWhenOneFails() {
	p := s.sharedWith(rootOU, otherOU)
	var postOUs []string
	s.flows.EXPECT().ExecuteSystemFlow(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, sysCtx *flowcommon.SystemFlowContext) (*flowcommon.SystemFlowResult,
			*tidcommon.ServiceError) {
			ouID := sysCtx.Inputs[systemFlowInputOU]
			if sysCtx.Handle == testUnsharePreFlow {
				return &flowcommon.SystemFlowResult{}, nil
			}
			postOUs = append(postOUs, ouID)
			if ouID == otherOU {
				return nil, &tidcommon.InternalServerError
			}
			return &flowcommon.SystemFlowResult{}, nil
		})

	svcErr := s.svc.DeletePolicy(context.Background(), p.ID)

	s.Equal(&tidcommon.InternalServerError, svcErr)
	s.Equal([]string{otherOU, rootOU}, postOUs, "the post flow after the failed one still ran")
	s.NotContains(s.store.policies, p.ID)
}

// A pre flow that refuses stops every entry point that revokes, before anything is written, and the refusal comes
// back as it is: for an import, as a refusal of the change as a whole.
func (s *SystemFlowTestSuite) TestARefusedPreFlowStopsEveryEntryPoint() {
	refusal := &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "RVS-1001"}
	for name, change := range map[string]func(p Policy) *tidcommon.ServiceError{
		"update": func(p Policy) *tidcommon.ServiceError {
			_, svcErr := s.svc.UpdatePolicy(context.Background(), p.ID, PolicyRequest{
				Version: p.Version, OtherTreeRoots: roots(root(rootOU))})
			return svcErr
		},
		"delete": func(p Policy) *tidcommon.ServiceError {
			return s.svc.DeletePolicy(context.Background(), p.ID)
		},
		"import replay": func(Policy) *tidcommon.ServiceError {
			_, replayErr := s.svc.ReplacePoliciesForResource(context.Background(), testType, testResource, ownerOU,
				[]PolicyRequest{{OtherTreeRoots: roots(root(rootOU))}})
			s.Require().NotNil(replayErr)
			s.True(replayErr.Whole)
			return replayErr.Err
		},
		"import removal": func(Policy) *tidcommon.ServiceError {
			_, replayErr := s.svc.RemovePolicies(context.Background(), testType, testResource, []string{""}, nil)
			s.Require().NotNil(replayErr)
			s.True(replayErr.Whole)
			return replayErr.Err
		},
	} {
		s.Run(name, func() {
			s.SetupTest()
			p := s.sharedWith(rootOU, otherOU)
			s.expectFlows(refusal, nil)

			s.Equal(refusal, change(p))
			s.Equal(p, s.store.policies[p.ID], "nothing was written")
			s.True(s.visible(otherOU))
			s.flows.AssertNumberOfCalls(s.T(), "ExecuteSystemFlow", 1)
		})
	}
}

func (s *SystemFlowTestSuite) TestTheDryRunIsRolledBack() {
	s.sharedWith(otherOU)
	s.expectFlows(nil, nil)
	runs := 0

	svcErr := s.svc.revokeAround(context.Background(), testType, testResource, func(ctx context.Context) bool {
		runs++
		s.Require().NoError(recordLoss(ctx, lossOf(testType, testResource), otherOU))
		s.store.policies[fmt.Sprintf("written-%d", runs)] = Policy{ID: "written"}
		return true
	})

	s.Require().Nil(svcErr)
	s.Equal(2, runs)
	s.NotContains(s.store.policies, "written-1", "the dry run's writes are undone")
	s.Contains(s.store.policies, "written-2")
}

// A change that takes the resource from no unit and requires no revocation is kept as its first run made it, so it
// runs once, and what it removed reaches the caller's replay report once.
func (s *SystemFlowTestSuite) TestAChangeThatTakesNothingAwayRunsOnce() {
	ctx, report := WithReplayReport(context.Background())
	runs := 0

	svcErr := s.svc.revokeAround(ctx, testType, testResource, func(ctx context.Context) bool {
		runs++
		s.store.policies["written"] = Policy{ID: "written"}
		RecordRemoved(ctx, []RemovedPolicy{{PolicyID: "reshare"}})
		return true
	})

	s.Require().Nil(svcErr)
	s.Equal(1, runs)
	s.Contains(s.store.policies, "written", "the first run is kept")
	s.Equal([]RemovedPolicy{{PolicyID: "reshare"}}, report.Removed())
	s.flows.AssertNotCalled(s.T(), "ExecuteSystemFlow", mock.Anything, mock.Anything)
}

// A change that does take the resource away runs twice, and only the run that is kept reports what it removed.
func (s *SystemFlowTestSuite) TestOnlyTheKeptRunReportsWhatItRemoved() {
	s.sharedWith(otherOU)
	s.expectFlows(nil, nil)
	ctx, report := WithReplayReport(context.Background())

	svcErr := s.svc.revokeAround(ctx, testType, testResource, func(ctx context.Context) bool {
		s.Require().NoError(recordLoss(ctx, lossOf(testType, testResource), otherOU))
		RecordRemoved(ctx, []RemovedPolicy{{PolicyID: "reshare"}})
		return true
	})

	s.Require().Nil(svcErr)
	s.Equal([]RemovedPolicy{{PolicyID: "reshare"}}, report.Removed())
}

func (s *SystemFlowTestSuite) TestALossTheDryRunDidNotSeeRefusesTheChange() {
	s.sharedWith(otherOU)
	s.expectFlows(nil, nil)
	runs := 0

	svcErr := s.svc.revokeAround(context.Background(), testType, testResource, func(ctx context.Context) bool {
		runs++
		s.Require().NoError(recordLoss(ctx, lossOf(testType, testResource), otherOU))
		if runs == 2 {
			// The sharing changed between the two runs, so this run takes the resource from one more unit.
			s.Require().ErrorIs(recordLoss(ctx, lossOf(testType, testResource), rootOU), errUnplannedLoss)
			return false
		}
		return true
	})

	s.Equal(&ErrorSharingChangedDuringRevocation, svcErr)
	s.Equal([]string{otherOU}, s.preFlowOUs)
	s.flows.AssertNumberOfCalls(s.T(), "ExecuteSystemFlow", 1)
}

// An edit that adds a unit while a delete runs its pre flows is caught when the delete runs for real: it reads the
// policy again, so its cleanup reaches the added unit, and the delete is refused rather than taking that unit's
// access away unrevoked.
func (s *SystemFlowTestSuite) TestAUnitAddedWhileTheDeleteRevokesRefusesTheDelete() {
	p := s.sharedWith(otherOU)
	s.flows.EXPECT().ExecuteSystemFlow(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, sysCtx *flowcommon.SystemFlowContext) (*flowcommon.SystemFlowResult,
			*tidcommon.ServiceError) {
			s.Require().Equal(testUnsharePreFlow, sysCtx.Handle, "no post flow runs for a refused change")
			s.preFlowOUs = append(s.preFlowOUs, sysCtx.Inputs[systemFlowInputOU])
			_, svcErr := s.svc.updatePolicy(context.Background(), p.ID, PolicyRequest{
				Version: p.Version, OtherTreeRoots: roots(root(otherOU), root(rootOU))}, false)
			s.Require().Nil(svcErr)
			return &flowcommon.SystemFlowResult{}, nil
		}).Once()

	svcErr := s.svc.DeletePolicy(context.Background(), p.ID)

	s.Equal(&ErrorSharingChangedDuringRevocation, svcErr)
	s.Equal([]string{otherOU}, s.preFlowOUs)
	s.Contains(s.store.policies, p.ID, "the refused delete is rolled back")
	s.True(s.visible(rootOU), "the added unit keeps the resource")
}

// The dry run reads a state that is rolled back, so it caches none of it for other requests to see; the run that
// is kept caches as usual.
func (s *SystemFlowTestSuite) TestTheDryRunCachesNothingItReads() {
	visibility, cached := newMockCache[bool](s.T())
	storeMock, storeState := newMockStore(s.T())
	s.store = storeState
	hierarchy, enumerator := testResolver(s.T())
	svc := newSharingService(storeMock, newFileBasedStore(), hierarchy, enumerator,
		rollbackTx(s.T(), storeState), visibility, nil, false)
	svc.RegisterResourceType(&revokingDeclaration{})
	s.Require().NoError(SetSystemFlowProvider(svc, s.flows))
	s.svc = svc.(*sharingService)
	s.sharedWith(otherOU)
	s.expectFlows(nil, nil)
	cached.sets = 0
	runs := 0

	svcErr := s.svc.revokeAround(context.Background(), testType, testResource, func(ctx context.Context) bool {
		runs++
		// A hook reads visibility the way IsVisible answers it, through the cache.
		_, readErr := s.svc.IsVisible(ctx, testType, testResource, otherOU)
		s.Require().Nil(readErr)
		s.Require().NoError(recordLoss(ctx, lossOf(testType, testResource), otherOU))
		if runs == 1 {
			s.Zero(cached.sets, "the dry run cached what it read")
		}
		return true
	})

	s.Require().Nil(svcErr)
	s.Equal(2, runs)
	s.Equal(1, cached.sets, "the run that is kept caches what it reads")
}

func (s *SystemFlowTestSuite) TestAUnitThatNeverSawTheResourceIsNotRevoked() {
	ran := 0

	svcErr := s.svc.revokeAround(context.Background(), testType, testResource, func(ctx context.Context) bool {
		ran++
		s.Require().NoError(recordLoss(ctx, lossOf(testType, testResource), otherOU))
		return true
	})

	s.Require().Nil(svcErr)
	s.Equal(2, ran)
	s.flows.AssertNotCalled(s.T(), "ExecuteSystemFlow", mock.Anything, mock.Anything)
}

func (s *SystemFlowTestSuite) TestAUnitDeletedWhileNamedIsStillRevoked() {
	s.expectFlows(nil, nil)

	svcErr := s.svc.revokeAround(context.Background(), testType, testResource, func(ctx context.Context) bool {
		s.Require().NoError(recordLoss(ctx, lossOf(testType, testResource), "deleted-ou"))
		return true
	})

	s.Require().Nil(svcErr)
	s.Equal([]string{"deleted-ou"}, s.preFlowOUs)
}

func (s *SystemFlowTestSuite) TestAFailedVisibilityCheckStopsTheChange() {
	hierarchy := sysauthzmock.NewOUHierarchyResolverMock(s.T())
	hierarchy.EXPECT().GetAncestorOUIDs(mock.Anything, otherOU).Return(nil, &tidcommon.InternalServerError)
	ran := 0

	svcErr := s.svc.revokeAround(context.Background(), testType, testResource, func(ctx context.Context) bool {
		ran++
		s.svc.ouHierarchyResolver = hierarchy
		s.Require().NoError(recordLoss(ctx, lossOf(testType, testResource), otherOU))
		return true
	})

	s.Equal(&tidcommon.InternalServerError, svcErr)
	s.Equal(1, ran, "the change is not made")
	s.flows.AssertNotCalled(s.T(), "ExecuteSystemFlow", mock.Anything, mock.Anything)
}

func (s *SystemFlowTestSuite) TestALossOfAnotherResourceIsNotCollected() {
	collector := &lossCollector{rt: testType, resourceID: testResource, planned: map[string]struct{}{}}
	ctx := context.WithValue(context.Background(), lossCollectorKey{}, collector)

	s.Require().NoError(recordLoss(ctx, lossOf(testType, "another-resource"), otherOU))
	s.Require().NoError(recordLoss(ctx, lossOf(fieldlessType, testResource), otherOU))

	s.Empty(collector.lost)
	s.False(collector.unplanned)
}

func (s *SystemFlowTestSuite) TestAFailedDryRunTransactionIsAnInternalError() {
	tx := rollbackTx(s.T(), s.store)
	tx.ExpectedCalls = nil
	tx.EXPECT().Transact(mock.Anything, mock.Anything).Return(errors.New("begin failed"))
	s.svc.transactioner = tx
	ran := false

	svcErr := s.svc.revokeAround(context.Background(), testType, testResource, func(context.Context) bool {
		ran = true
		return true
	})

	s.Equal(&tidcommon.InternalServerError, svcErr)
	s.False(ran)
}

func (s *SystemFlowTestSuite) TestSetSystemFlowProviderRefusesAnotherImplementation() {
	s.Error(SetSystemFlowProvider(NewSharingServiceInterfaceMock(s.T()), s.flows))
}

// testRequired is a revocation a hook of the test type requires.
var testRequired = Revocation{PreFlow: "system-test-required-pre", Inputs: map[string]string{"targetTestId": "x"}}

// expectRequiredFlows answers the required pre flow and the post flow that follows it, counting the pre flow runs.
func (s *SystemFlowTestSuite) expectRequiredFlows(preRuns *int) {
	s.flows.EXPECT().ExecuteSystemFlow(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, sysCtx *flowcommon.SystemFlowContext) (*flowcommon.SystemFlowResult,
			*tidcommon.ServiceError) {
			switch sysCtx.Handle {
			case testRequired.PreFlow:
				*preRuns++
				s.Equal(testRequired.Inputs, sysCtx.Inputs)
				return &flowcommon.SystemFlowResult{RuntimeData: map[string]string{"plan": "required"}}, nil
			case systemFlowRevocationPost:
				s.Equal(map[string]string{"plan": "required"}, sysCtx.RuntimeData)
				return &flowcommon.SystemFlowResult{}, nil
			}
			s.Failf("unexpected system flow", "%s", sysCtx.Handle)
			return nil, nil
		})
}

func (s *SystemFlowTestSuite) TestARequiredRevocationRunsOnceBeforeTheChange() {
	preRuns, runs := 0, 0
	s.expectRequiredFlows(&preRuns)

	svcErr := s.svc.revokeAround(context.Background(), testType, testResource, func(ctx context.Context) bool {
		runs++
		s.Equal(runs-1, preRuns, "the pre flow runs between the dry run and the change")
		s.Require().NoError(requireRevocations(ctx, []Revocation{testRequired}))
		s.Require().NoError(requireRevocations(ctx, []Revocation{testRequired}),
			"a repeated revocation is required once")
		return true
	})

	s.Require().Nil(svcErr)
	s.Equal(2, runs)
	s.Equal(1, preRuns)
	s.flows.AssertNumberOfCalls(s.T(), "ExecuteSystemFlow", 2)
}

func (s *SystemFlowTestSuite) TestARequiredRevocationTheDryRunDidNotSeeRefusesTheChange() {
	preRuns, runs := 0, 0
	s.expectRequiredFlows(&preRuns)

	svcErr := s.svc.revokeAround(context.Background(), testType, testResource, func(ctx context.Context) bool {
		runs++
		s.Require().NoError(requireRevocations(ctx, []Revocation{testRequired}))
		if runs == 2 {
			other := Revocation{PreFlow: testRequired.PreFlow, Inputs: map[string]string{"targetTestId": "y"}}
			s.Require().ErrorIs(requireRevocations(ctx, []Revocation{other}), errUnplannedLoss)
			return false
		}
		return true
	})

	s.Equal(&ErrorSharingChangedDuringRevocation, svcErr)
	s.Equal(1, preRuns)
}

func (s *SystemFlowTestSuite) TestARequiredRevocationOutsideARevokingChangeRecordsNothing() {
	s.NoError(requireRevocations(context.Background(), []Revocation{testRequired}))
}

// requiringDeclaration is the test type with a write hook that answers a removal with testRequired.
type requiringDeclaration struct {
	revokingDeclaration
}

func (d *requiringDeclaration) OnPolicyWritten(context.Context, Policy) ([]Revocation, *tidcommon.ServiceError) {
	return nil, nil
}

func (d *requiringDeclaration) OnPolicyRemoved(context.Context, Policy) ([]Revocation, *tidcommon.ServiceError) {
	return []Revocation{testRequired}, nil
}

// A type whose pre flow covers every unit runs it once for all the units the change takes the resource from, and one
// post flow after, however many units there are.
func (s *SystemFlowTestSuite) TestAFlowForEveryUnitRunsOnceForAllTheUnits() {
	storeMock, storeState := newMockStore(s.T())
	s.store = storeState
	hierarchy, enumerator := testResolver(s.T())
	svc := newSharingService(storeMock, newFileBasedStore(), hierarchy, enumerator,
		rollbackTx(s.T(), storeState), nil, nil, false)
	svc.RegisterResourceType(&everyUnitDeclaration{})
	s.Require().NoError(SetSystemFlowProvider(svc, s.flows))
	s.svc = svc.(*sharingService)
	p := s.sharedWith(rootOU, otherOU)
	var runs []*flowcommon.SystemFlowContext
	s.flows.EXPECT().ExecuteSystemFlow(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, sysCtx *flowcommon.SystemFlowContext) (*flowcommon.SystemFlowResult,
			*tidcommon.ServiceError) {
			runs = append(runs, sysCtx)
			return &flowcommon.SystemFlowResult{}, nil
		})

	s.Require().Nil(s.svc.DeletePolicy(context.Background(), p.ID))

	s.Require().Len(runs, 2)
	s.Equal(testUnsharePreFlow, runs[0].Handle)
	s.Equal(map[string]string{testUnshareResource: testResource,
		systemFlowInputOUs: `["` + otherOU + `","` + rootOU + `"]`}, runs[0].Inputs)
	s.Equal(systemFlowRevocationPost, runs[1].Handle)
}

// holderDeclaration is the test type whose own cleanup reports the holders it took away in a unit.
type holderDeclaration struct {
	revokingDeclaration
}

func (d *holderDeclaration) DeleteOverlayValues(context.Context, string, string) ([]string, error) {
	return []string{"late-holder"}, nil
}

// The holders the change's cleanup took away in a unit reach that unit's post flow, for those its pre flow did not
// see; the pre flow, run before the change, is given none.
func (s *SystemFlowTestSuite) TestTheHoldersACleanupRemovedReachTheUnitsPostFlow() {
	storeMock, storeState := newMockStore(s.T())
	s.store = storeState
	hierarchy, enumerator := testResolver(s.T())
	svc := newSharingService(storeMock, newFileBasedStore(), hierarchy, enumerator,
		rollbackTx(s.T(), storeState), nil, nil, false)
	svc.RegisterResourceType(&holderDeclaration{})
	s.Require().NoError(SetSystemFlowProvider(svc, s.flows))
	s.svc = svc.(*sharingService)
	p := s.sharedWith(otherOU)
	inputs := map[string]map[string]string{}
	s.flows.EXPECT().ExecuteSystemFlow(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, sysCtx *flowcommon.SystemFlowContext) (*flowcommon.SystemFlowResult,
			*tidcommon.ServiceError) {
			inputs[sysCtx.Handle] = sysCtx.Inputs
			return &flowcommon.SystemFlowResult{}, nil
		})

	s.Require().Nil(s.svc.DeletePolicy(context.Background(), p.ID))

	s.NotContains(inputs[testUnsharePreFlow], systemFlowInputRemovedHolders)
	s.Equal(`["late-holder"]`, inputs[systemFlowRevocationPost][systemFlowInputRemovedHolders])
	s.Equal(otherOU, inputs[systemFlowRevocationPost][systemFlowInputOU])
}

// The revocations a write hook answers with run before the change, beside the pre flows of the units it loses.
func (s *SystemFlowTestSuite) TestTheRevocationsAHookAnswersWithRunBeforeTheChange() {
	storeMock, storeState := newMockStore(s.T())
	s.store = storeState
	hierarchy, enumerator := testResolver(s.T())
	svc := newSharingService(storeMock, newFileBasedStore(), hierarchy, enumerator,
		rollbackTx(s.T(), storeState), nil, nil, false)
	svc.RegisterResourceType(&requiringDeclaration{})
	s.Require().NoError(SetSystemFlowProvider(svc, s.flows))
	s.svc = svc.(*sharingService)
	p := s.sharedWith(otherOU)
	var handles []string
	s.flows.EXPECT().ExecuteSystemFlow(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, sysCtx *flowcommon.SystemFlowContext) (*flowcommon.SystemFlowResult,
			*tidcommon.ServiceError) {
			handles = append(handles, sysCtx.Handle)
			return &flowcommon.SystemFlowResult{}, nil
		})

	s.Require().Nil(s.svc.DeletePolicy(context.Background(), p.ID))

	s.Equal([]string{testUnsharePreFlow, testRequired.PreFlow, systemFlowRevocationPost, systemFlowRevocationPost},
		handles)
}
