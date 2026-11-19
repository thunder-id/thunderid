// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/export"
	"github.com/thunder-id/thunderid/internal/system/varname"
	"github.com/thunder-id/thunderid/internal/variablestore"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

const (
	appA = "0198f3a1-7c2e-7b10-9a44-5d3e8f21c6b1"
	appB = "0198f3a1-7c2e-7b10-9a44-5d3e8f21c6b2"
	appC = "0198f3a1-7c2e-7b10-9a44-5d3e8f21c6b3"

	sweepKey   = "key"
	sweepGrace = 24 * time.Hour
)

var (
	sweepGateway = &Gateway{ID: "gw-1", BaseURL: "https://dp.test"}
	longAgo      = time.Now().Add(-72 * time.Hour)
)

// secretOf and hostOf are the names the export gives an application's client secret and host.
func secretOf(id string) string { return varname.DeriveVariableName("application", id, "clientSecret") }
func hostOf(id string) string   { return varname.DeriveVariableName("application", id, "host") }

// applicationUsing is an application document that refers to its client secret and its host.
func applicationUsing(id string) string {
	return "resource_type: application\nid: " + id + "\nname: " + id + "\ndescription: var:" + hostOf(id) +
		"\ninboundAuthConfig:\n  - type: oauth2\n    config:\n      clientSecret: sec:" + secretOf(id)
}

// keep adds a version holding the given applications, as a capture keeps one.
func keep(versions *fakeVersionStore, ids ...string) Version {
	documents := make([]string, 0, len(ids))
	for _, id := range ids {
		documents = append(documents, applicationUsing(id))
	}
	content := joinBundle(documents)
	version := Version{Seq: len(versions.versions) + 1, Hash: versionHash(content), Resources: content}
	versions.versions = append(versions.versions, version)
	return version
}

// prune removes a version, as retention does once it is past gateway.max_versions.
func prune(versions *fakeVersionStore, seq int) {
	kept := versions.versions[:0]
	for _, version := range versions.versions {
		if version.Seq != seq {
			kept = append(kept, version)
		}
	}
	versions.versions = kept
}

// currentExport is a configuration that holds the given applications, and an organization unit, so it is
// never empty.
func currentExport(ids ...string) *export.ExportResponse {
	files := make([]export.ExportFile, 0, len(ids)+1)
	files = append(files, export.ExportFile{Content: "resource_type: organization_unit\nid: ou-1\nname: Root"})
	for _, id := range ids {
		files = append(files, export.ExportFile{Content: applicationUsing(id)})
	}
	return &export.ExportResponse{Files: files}
}

// heldValues is what the gateway's store holds in a test, by name, each with when it was set. The
// valueStoreClient mock answers from it.
type heldValues struct {
	variables map[string]time.Time
	secrets   map[string]time.Time
	// queries are the listings asked for, in order.
	queries []StoreListQuery
	// removed are the values removed, as "var:" or "sec:" and the name.
	removed []string
	// bulkCalls are the names each removal of many names was asked for.
	bulkCalls [][]string
}

func newHeldValues() *heldValues {
	return &heldValues{variables: map[string]time.Time{}, secrets: map[string]time.Time{}}
}

// holdBoth puts an application's client secret and host in the store, set at the given time.
func (h *heldValues) holdBoth(id string, at time.Time) {
	h.secrets[secretOf(id)] = at
	h.variables[hostOf(id)] = at
}

// page answers a listing of names the way the store does: sorted by name, from offset, limit at most.
func (h *heldValues) page(held map[string]time.Time, query StoreListQuery) ([]string, int) {
	h.queries = append(h.queries, query)
	limit, _ := strconv.Atoi(query.Limit)
	offset, _ := strconv.Atoi(query.Offset)
	names := make([]string, 0, len(held))
	for name := range held {
		names = append(names, name)
	}
	sort.Strings(names)
	if offset >= len(names) {
		return nil, len(names)
	}
	return names[offset:min(offset+limit, len(names))], len(names)
}

func (h *heldValues) listVariables(_ context.Context, _ *Gateway, _ string,
	query StoreListQuery) (*variablestore.VariableListResponse, error) {
	names, total := h.page(h.variables, query)
	listing := &variablestore.VariableListResponse{TotalResults: total, Count: len(names)}
	for _, name := range names {
		listing.Variables = append(listing.Variables, variablestore.Variable{Name: name,
			UpdatedAt: storeTime(h.variables[name])})
	}
	return listing, nil
}

func (h *heldValues) listSecrets(_ context.Context, _ *Gateway, _ string,
	query StoreListQuery) (*variablestore.SecretListResponse, error) {
	names, total := h.page(h.secrets, query)
	listing := &variablestore.SecretListResponse{TotalResults: total, Count: len(names)}
	for _, name := range names {
		listing.Secrets = append(listing.Secrets, variablestore.Secret{Name: name, Exists: true,
			UpdatedAt: storeTime(h.secrets[name])})
	}
	return listing, nil
}

// removeMany removes the names held and last set longer ago than the age, by the store's own clock,
// as the store's removal of many names does, and answers with them.
func (h *heldValues) removeMany(held map[string]time.Time, prefix string, names []string,
	unchangedFor time.Duration) []string {
	h.bulkCalls = append(h.bulkCalls, names)
	var deleted []string
	for _, name := range names {
		at, ok := held[name]
		if !ok || (unchangedFor > 0 && !at.Before(time.Now().Add(-unchangedFor))) {
			continue
		}
		delete(held, name)
		h.removed = append(h.removed, prefix+name)
		deleted = append(deleted, name)
	}
	return deleted
}

func (h *heldValues) removeVariables(_ context.Context, _ *Gateway, _ string, names []string,
	unchangedFor time.Duration) ([]string, error) {
	return h.removeMany(h.variables, "var:", names, unchangedFor), nil
}

func (h *heldValues) removeSecrets(_ context.Context, _ *Gateway, _ string, names []string,
	unchangedFor time.Duration) ([]string, error) {
	return h.removeMany(h.secrets, "sec:", names, unchangedFor), nil
}

// storeTime writes a time as the store does; zero, as a store that did not say.
func storeTime(at time.Time) string {
	if at.IsZero() {
		return ""
	}
	return at.UTC().Format(time.RFC3339)
}

// UnusedValueSweeperTestSuite sweeps a gateway's store held in heldValues, through generated mocks of
// the store client, the export and the runner.
type UnusedValueSweeperTestSuite struct {
	suite.Suite
	versions *fakeVersionStore
	held     *heldValues
	store    *valueStoreClientMock
	current  *configurationExporterMock
	runner   *sweepRunnerMock
}

func TestUnusedValueSweeperTestSuite(t *testing.T) {
	suite.Run(t, new(UnusedValueSweeperTestSuite))
}

func (s *UnusedValueSweeperTestSuite) SetupTest() {
	s.versions = &fakeVersionStore{}
	s.held = newHeldValues()
	s.store = newValueStoreClientMock(s.T())
	s.current = newConfigurationExporterMock(s.T())
	s.runner = newSweepRunnerMock(s.T())
}

// sweeper sweeps with the suite's mocks, the grace of a day, and the given interval.
func (s *UnusedValueSweeperTestSuite) sweeper(interval time.Duration) *unusedValueSweeper {
	return newUnusedValueSweeper(s.versions, s.current, s.store, s.runner, sweepGrace, interval)
}

// sweep runs one sweep of sweepGateway.
func (s *UnusedValueSweeperTestSuite) sweep(sweeper *unusedValueSweeper) (sweepOutcome, error) {
	return sweeper.sweep(context.Background(), sweepGateway, sweepKey)
}

// expectListing has the store answer every listing from what it holds.
func (s *UnusedValueSweeperTestSuite) expectListing() {
	s.store.EXPECT().ListVariables(mock.Anything, mock.Anything, sweepKey, mock.Anything).
		RunAndReturn(s.held.listVariables)
	s.store.EXPECT().ListSecrets(mock.Anything, mock.Anything, sweepKey, mock.Anything).
		RunAndReturn(s.held.listSecrets)
}

// expectExport has every export of the configuration hold the given applications.
func (s *UnusedValueSweeperTestSuite) expectExport(ids ...string) {
	s.current.EXPECT().exportConfiguration(mock.Anything).Return(currentExport(ids...), nil)
}

// sweepAge matches the age a sweep asks the store to check: the grace, the whole seconds since the
// listing, and one more, which in a test that takes well under a minute is a few seconds past the grace.
func sweepAge() interface{} {
	return mock.MatchedBy(func(age time.Duration) bool {
		return age >= sweepGrace+2*time.Second && age <= sweepGrace+time.Minute && age%time.Second == 0
	})
}

// expectBulkVariables and expectBulkSecrets have the store remove many names at once, each only when
// last set longer ago than the age the sweep passes for the store to check.
func (s *UnusedValueSweeperTestSuite) expectBulkVariables() {
	s.store.EXPECT().DeleteVariables(mock.Anything, mock.Anything, sweepKey, mock.Anything, sweepAge()).
		RunAndReturn(s.held.removeVariables)
}

func (s *UnusedValueSweeperTestSuite) expectBulkSecrets() {
	s.store.EXPECT().DeleteSecrets(mock.Anything, mock.Anything, sweepKey, mock.Anything, sweepAge()).
		RunAndReturn(s.held.removeSecrets)
}

// expectSynchronousRuns runs each sweep as it is started, so a test reads what it did right after.
func (s *UnusedValueSweeperTestSuite) expectSynchronousRuns() {
	s.runner.EXPECT().run(mock.Anything).Run(func(sweep func()) { sweep() })
}

// A value nothing kept refers to, named by this plane and set long ago, is removed, variable and
// secret alike; the values a kept version refers to stay.
func (s *UnusedValueSweeperTestSuite) TestASweepRemovesOnlyUnreferencedGeneratedValues() {
	keep(s.versions, appA)
	s.held.holdBoth(appA, longAgo)
	s.held.holdBoth(appB, longAgo)
	s.expectListing()
	s.expectExport()
	s.expectBulkVariables()
	s.expectBulkSecrets()

	outcome, err := s.sweep(s.sweeper(0))

	s.Require().NoError(err)
	s.ElementsMatch([]string{"sec:" + secretOf(appB), "var:" + hostOf(appB)}, s.held.removed)
	s.Contains(s.held.secrets, secretOf(appA))
	s.Contains(s.held.variables, hostOf(appA))
	s.Equal(1, outcome.removedSecrets)
	s.Equal(1, outcome.removedVariables)
	s.Equal(4, outcome.listed)
}

// A value set again after the store was listed is not removed: the gateway removes only what it finds
// last set longer ago than the grace, by its own clock, and what it kept is not counted.
func (s *UnusedValueSweeperTestSuite) TestASweepKeepsAValueSetAgainAfterTheListing() {
	keep(s.versions, appA)
	s.held.holdBoth(appB, longAgo)
	s.held.holdBoth(appC, longAgo)
	s.expectListing()
	// The export runs after the listing; the administrator sets appB's host again meanwhile.
	s.current.EXPECT().exportConfiguration(mock.Anything).RunAndReturn(
		func(context.Context) (*export.ExportResponse, *tidcommon.ServiceError) {
			s.held.variables[hostOf(appB)] = time.Now()
			return currentExport(), nil
		})
	s.expectBulkVariables()
	s.expectBulkSecrets()

	outcome, err := s.sweep(s.sweeper(0))

	s.Require().NoError(err)
	s.Contains(s.held.variables, hostOf(appB), "a value set again after the listing was removed")
	s.ElementsMatch([]string{"var:" + hostOf(appC), "sec:" + secretOf(appB), "sec:" + secretOf(appC)},
		s.held.removed)
	s.Equal(1, outcome.removedVariables)
	s.Equal(2, outcome.removedSecrets)
}

// A value set again just after the listing stays though the sweep then runs past the grace before it
// removes anything: the age it asks the store to check grows with the time since the listing.
func (s *UnusedValueSweeperTestSuite) TestASweepKeepsAValueSetAgainAfterTheListingHoweverLongItRuns() {
	keep(s.versions, appA)
	s.held.holdBoth(appB, longAgo)
	s.held.holdBoth(appC, longAgo)
	s.expectListing()
	s.current.EXPECT().exportConfiguration(mock.Anything).RunAndReturn(
		func(context.Context) (*export.ExportResponse, *tidcommon.ServiceError) {
			s.held.variables[hostOf(appB)] = time.Now()
			s.held.secrets[secretOf(appB)] = time.Now()
			// Longer than the one-second grace, as a long export would be.
			time.Sleep(1200 * time.Millisecond)
			return currentExport(), nil
		})
	s.store.EXPECT().DeleteVariables(mock.Anything, mock.Anything, sweepKey, mock.Anything, mock.Anything).
		RunAndReturn(s.held.removeVariables)
	s.store.EXPECT().DeleteSecrets(mock.Anything, mock.Anything, sweepKey, mock.Anything, mock.Anything).
		RunAndReturn(s.held.removeSecrets)
	sweeper := newUnusedValueSweeper(s.versions, s.current, s.store, s.runner, time.Second, 0)

	outcome, err := s.sweep(sweeper)

	s.Require().NoError(err)
	s.Contains(s.held.variables, hostOf(appB), "a variable set again after the listing was removed")
	s.Contains(s.held.secrets, secretOf(appB), "a secret set again after the listing was removed")
	s.ElementsMatch([]string{"var:" + hostOf(appC), "sec:" + secretOf(appC)}, s.held.removed)
	s.Equal(1, outcome.removedVariables)
	s.Equal(1, outcome.removedSecrets)
}

// The age a removal asks for is the grace, the time since the listing rounded up to the second, and
// one second more, so it grows as the sweep runs.
func (s *UnusedValueSweeperTestSuite) TestTheAgeARemovalAsksForGrowsWithTheTimeSinceTheListing() {
	sweeper := newUnusedValueSweeper(s.versions, s.current, s.store, s.runner, time.Minute, 0)
	for _, tc := range []struct {
		since time.Duration
		want  time.Duration
	}{
		// A listing at least a moment old, so the rounding up below always adds a second.
		{time.Millisecond, time.Minute + 2*time.Second},
		{1500 * time.Millisecond, time.Minute + 3*time.Second},
		{10*time.Second + time.Millisecond, time.Minute + 12*time.Second},
	} {
		removal := &batchRemoval{sweeper: sweeper, listedAt: time.Now().Add(-tc.since)}
		age := removal.age()
		// The time the test itself takes can add at most one more second.
		s.GreaterOrEqual(age, tc.want, tc.since.String())
		s.LessOrEqual(age, tc.want+time.Second, tc.since.String())
	}
}

// A value an administrator named is never removed, however long ago it was set and whatever refers
// to it.
func (s *UnusedValueSweeperTestSuite) TestASweepKeepsAHandNamedValue() {
	keep(s.versions, appA)
	s.held.variables["DATA_PLANE_URL"] = longAgo
	s.held.secrets["MY_API_KEY"] = longAgo
	s.held.variables["APPLICATION__MY_APP__HOST"] = longAgo
	s.expectListing()

	outcome, err := s.sweep(s.sweeper(0))

	s.Require().NoError(err)
	s.Empty(s.held.removed)
	s.Zero(outcome.generated)
}

// Every version the deployment keeps protects its values, not only the one the gateway runs, so a
// rollback or a later apply of an older version still finds them.
func (s *UnusedValueSweeperTestSuite) TestASweepKeepsTheValuesOfEveryRetainedVersion() {
	older := keep(s.versions, appA, appB)
	keep(s.versions, appA)
	s.versions.applied = map[string]*AppliedVersion{"gw-1": {GatewayID: "gw-1", AppliedVersion: older.Seq + 1}}
	s.held.holdBoth(appA, longAgo)
	s.held.holdBoth(appB, longAgo)
	s.expectListing()
	s.expectExport()

	_, err := s.sweep(s.sweeper(0))

	s.Require().NoError(err)
	s.Empty(s.held.removed, "a value an older retained version refers to was removed")
}

// What the gateway runs protects its values even when no kept version refers to them any more, as
// with a resource held back from later applies.
func (s *UnusedValueSweeperTestSuite) TestASweepKeepsTheValuesOfTheGatewaysMaster() {
	keep(s.versions, appA)
	s.versions.masters = map[string][]Master{"gw-1": {{GatewayID: "gw-1", Seq: 1, Hash: "m-1",
		Resources: joinBundle([]string{applicationUsing(appA), applicationUsing(appC)})}}}
	s.held.holdBoth(appC, longAgo)
	s.expectListing()
	s.expectExport()

	_, err := s.sweep(s.sweeper(0))

	s.Require().NoError(err)
	s.Empty(s.held.removed, "a value the gateway's master refers to was removed")
}

// A value set within the grace window stays though nothing refers to it yet, so one set just before
// the version that uses it is deployed is still there for it; a store that did not say when it was set
// keeps it too. Nothing is a candidate, so the configuration is not even exported.
func (s *UnusedValueSweeperTestSuite) TestASweepKeepsAValueSetWithinTheGraceWindow() {
	keep(s.versions, appA)
	s.held.holdBoth(appB, time.Now().Add(-time.Hour))
	s.held.holdBoth(appC, time.Time{})
	s.expectListing()

	outcome, err := s.sweep(s.sweeper(0))

	s.Require().NoError(err)
	s.Empty(s.held.removed)
	s.Equal(4, outcome.recent)
}

// Once the last version that refers to a value is pruned and no master holds it, the value goes: it
// can no longer be applied or rolled back to.
func (s *UnusedValueSweeperTestSuite) TestASweepRemovesAValueOnceItsVersionIsPruned() {
	first := keep(s.versions, appA, appB)
	keep(s.versions, appA)
	s.held.holdBoth(appA, longAgo)
	s.held.holdBoth(appB, longAgo)
	s.expectListing()
	s.expectExport()
	sweeper := s.sweeper(0)

	_, err := s.sweep(sweeper)
	s.Require().NoError(err)
	s.Require().Empty(s.held.removed)

	prune(s.versions, first.Seq)
	s.expectBulkVariables()
	s.expectBulkSecrets()
	_, err = s.sweep(sweeper)

	s.Require().NoError(err)
	s.ElementsMatch([]string{"sec:" + secretOf(appB), "var:" + hostOf(appB)}, s.held.removed)
}

// A sweep that finds no version at all removes nothing: every value would otherwise look unused.
func (s *UnusedValueSweeperTestSuite) TestASweepWithNoVersionRemovesNothing() {
	s.held.holdBoth(appA, longAgo)
	s.expectListing()

	_, err := s.sweep(s.sweeper(0))

	s.Require().ErrorIs(err, errNoRetainedVersion)
	s.Empty(s.held.removed)
}

// A version or master that cannot be read stops the sweep before anything is removed.
func (s *UnusedValueSweeperTestSuite) TestASweepThatCannotReadTheVersionsRemovesNothing() {
	keep(s.versions, appA)
	s.versions.masterErr = errors.New("database unavailable")
	s.held.holdBoth(appB, longAgo)
	s.expectListing()

	_, err := s.sweep(s.sweeper(0))

	s.Require().Error(err)
	s.Empty(s.held.removed)
}

// A store that cannot be listed stops the sweep before anything else is read.
func (s *UnusedValueSweeperTestSuite) TestASweepThatCannotListTheStoreRemovesNothing() {
	s.store.EXPECT().ListVariables(mock.Anything, mock.Anything, sweepKey, mock.Anything).
		Return(nil, errors.New("the gateway's store is down")).Once()

	_, err := s.sweep(s.sweeper(0))

	s.Require().Error(err)
	s.Contains(err.Error(), "failed to list the gateway's variables")
}

// The store is read a page at a time, at most as many as it answers for in one call, until it says
// it has no more.
func (s *UnusedValueSweeperTestSuite) TestASweepPagesThroughTheStore() {
	keep(s.versions, appA)
	for i := range 250 {
		s.held.variables[fmt.Sprintf("HAND_NAMED_%03d", i)] = longAgo
	}
	s.expectListing()

	outcome, err := s.sweep(s.sweeper(0))

	s.Require().NoError(err)
	s.Equal(250, outcome.listed)
	offsets := make([]string, 0, len(s.held.queries))
	for _, query := range s.held.queries {
		s.Equal(strconv.Itoa(maxNamesPerLookup), query.Limit)
		offsets = append(offsets, query.Offset)
	}
	s.Equal([]string{"0", "100", "200", "0"}, offsets)
}

// A version and a master are read with their content once: both are immutable, so a later sweep
// takes what they refer to from the cache.
func (s *UnusedValueSweeperTestSuite) TestASweepReadsEachVersionOnce() {
	first, second := keep(s.versions, appA), keep(s.versions, appB)
	master := Master{GatewayID: "gw-1", Seq: 1, Hash: "m-1", Resources: applicationUsing(appA)}
	versions := newVersionStoreInterfaceMock(s.T())
	versions.EXPECT().List(mock.Anything).Return([]Version{first, second}, nil).Times(3)
	versions.EXPECT().GetApplied(mock.Anything, "gw-1").Return(nil, nil).Times(3)
	versions.EXPECT().ListMasters(mock.Anything, "gw-1").Return([]Master{master}, nil).Times(3)
	versions.EXPECT().Get(mock.Anything, first.Seq).Return(&first, nil).Once()
	versions.EXPECT().Get(mock.Anything, second.Seq).Return(&second, nil).Once()
	versions.EXPECT().GetMasterAt(mock.Anything, "gw-1", master.Seq).Return(&master, nil).Once()
	s.expectListing()
	s.expectExport()
	s.expectBulkVariables()
	s.expectBulkSecrets()
	sweeper := newUnusedValueSweeper(versions, s.current, s.store, s.runner, sweepGrace, 0)

	for range 3 {
		s.held.holdBoth(appC, longAgo)
		_, err := s.sweep(sweeper)
		s.Require().NoError(err)
	}
}

// A recorded apply starts a sweep of the gateway it was applied to.
func (s *UnusedValueSweeperTestSuite) TestARecordedApplyStartsASweep() {
	f := newVersionFixture(s.T())
	f.capture(s.T(), "ou-a")
	sweeper := newUnusedValueSweeperInterfaceMock(s.T())
	sweeper.EXPECT().afterApply(mock.Anything, mock.MatchedBy(func(gw *Gateway) bool { return gw.ID == "gw-1" }),
		mock.Anything).Once()
	f.svc.(*versionService).unusedValues = sweeper

	result, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})

	s.Require().Nil(svcErr)
	s.True(result.Recorded)
}

// A dry run writes nothing, so it never sweeps.
func (s *UnusedValueSweeperTestSuite) TestADryRunNeverSweeps() {
	f := newVersionFixture(s.T())
	f.capture(s.T(), "ou-a")
	f.svc.(*versionService).unusedValues = newUnusedValueSweeperInterfaceMock(s.T())

	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{DryRun: true})

	s.Require().Nil(svcErr)
}

// An apply the gateway could not take never sweeps: what the gateway holds is not known.
func (s *UnusedValueSweeperTestSuite) TestAFailedApplyNeverSweeps() {
	f := newVersionFixture(s.T())
	f.capture(s.T(), "ou-a")
	f.client.err = errors.New("connection refused")
	f.svc.(*versionService).unusedValues = newUnusedValueSweeperInterfaceMock(s.T())

	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})

	s.Require().NotNil(svcErr)
}

// An apply refused for a value the gateway lacks never sweeps.
func (s *UnusedValueSweeperTestSuite) TestARefusedApplyNeverSweeps() {
	f := newVersionFixture(s.T())
	f.exporter.response = &export.ExportResponse{Files: []export.ExportFile{{Content: applicationUsing(appA)}}}
	_, svcErr := f.svc.Capture(context.Background(), CaptureRequest{})
	s.Require().Nil(svcErr)
	f.svc.(*versionService).unusedValues = newUnusedValueSweeperInterfaceMock(s.T())

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorMissingValues.Code, svcErr.Code)
}

// A sweep that fails is logged and nothing more: the apply it followed has answered, and stands.
func (s *UnusedValueSweeperTestSuite) TestASweepErrorDoesNotFailTheApply() {
	f := newVersionFixture(s.T())
	f.capture(s.T(), "ou-a")
	s.store.EXPECT().ListVariables(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("the gateway's store is down")).Once()
	s.expectSynchronousRuns()
	f.svc.(*versionService).unusedValues = newUnusedValueSweeper(f.versions, s.current, s.store, s.runner,
		sweepGrace, 0)

	result, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})

	s.Require().Nil(svcErr)
	s.True(result.Recorded)
}

// After a real apply, the sweep runs and removes what nothing the gateway runs or the deployment keeps
// refers to, and only that.
func (s *UnusedValueSweeperTestSuite) TestAnApplySweepsTheGatewaysUnusedValues() {
	f := newVersionFixture(s.T())
	f.exporter.response = &export.ExportResponse{Files: []export.ExportFile{{Content: applicationUsing(appA)}}}
	_, svcErr := f.svc.Capture(context.Background(), CaptureRequest{})
	s.Require().Nil(svcErr)
	f.client.held = map[string]bool{secretOf(appA): true, hostOf(appA): true}
	s.held.holdBoth(appA, longAgo)
	s.held.holdBoth(appB, longAgo)
	s.held.variables["DATA_PLANE_URL"] = longAgo
	s.store.EXPECT().ListVariables(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(s.held.listVariables)
	s.store.EXPECT().ListSecrets(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(s.held.listSecrets)
	s.store.EXPECT().DeleteVariables(mock.Anything, mock.Anything, mock.Anything, mock.Anything, sweepAge()).
		RunAndReturn(s.held.removeVariables).Once()
	s.store.EXPECT().DeleteSecrets(mock.Anything, mock.Anything, mock.Anything, mock.Anything, sweepAge()).
		RunAndReturn(s.held.removeSecrets).Once()
	s.expectSynchronousRuns()
	svc := f.svc.(*versionService)
	svc.unusedValues = newUnusedValueSweeper(f.versions, svc, s.store, s.runner, sweepGrace, 0)

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})

	s.Require().Nil(svcErr)
	s.ElementsMatch([]string{"sec:" + secretOf(appB), "var:" + hostOf(appB)}, s.held.removed)
}

// A second apply while a sweep of the same gateway runs does not start another.
func (s *UnusedValueSweeperTestSuite) TestASweepRunsOncePerGatewayAtATime() {
	sweeper := s.sweeper(0)
	sweeper.gateways[sweepGateway.ID] = &gatewaySweep{running: true}

	sweeper.afterApply(context.Background(), sweepGateway, sweepKey)

	s.runner.AssertNotCalled(s.T(), "run", mock.Anything)
}

// A sweep that is started is marked running until it ends, and then records when it ended.
func (s *UnusedValueSweeperTestSuite) TestASweepIsRunningUntilItEnds() {
	keep(s.versions, appA)
	sweeper := s.sweeper(time.Hour)
	var during gatewaySweep
	s.store.EXPECT().ListVariables(mock.Anything, mock.Anything, sweepKey, mock.Anything).RunAndReturn(
		func(ctx context.Context, gw *Gateway, key string,
			query StoreListQuery) (*variablestore.VariableListResponse, error) {
			sweeper.mu.Lock()
			during = *sweeper.gateways[sweepGateway.ID]
			sweeper.mu.Unlock()
			return s.held.listVariables(ctx, gw, key, query)
		}).Once()
	s.store.EXPECT().ListSecrets(mock.Anything, mock.Anything, sweepKey, mock.Anything).
		RunAndReturn(s.held.listSecrets).Once()
	s.expectSynchronousRuns()

	sweeper.afterApply(context.Background(), sweepGateway, sweepKey)

	s.True(during.running)
	s.False(sweeper.gateways[sweepGateway.ID].running)
	s.WithinDuration(time.Now(), sweeper.gateways[sweepGateway.ID].finished, time.Minute)
}

// A value of a resource that still exists is kept however long ago it was set, though no version
// refers to it because the resource was never deployed. Once the resource is deleted, and no kept
// version or master refers to the value, it goes.
func (s *UnusedValueSweeperTestSuite) TestASweepKeepsTheValuesOfAResourceThatStillExists() {
	keep(s.versions, appA)
	s.held.holdBoth(appB, longAgo)
	s.expectListing()
	s.current.EXPECT().exportConfiguration(mock.Anything).Return(currentExport(appA, appB), nil).Once()
	sweeper := s.sweeper(0)

	_, err := s.sweep(sweeper)
	s.Require().NoError(err)
	s.Empty(s.held.removed, "a value of a resource that exists was removed")

	s.current.EXPECT().exportConfiguration(mock.Anything).Return(currentExport(appA), nil).Once()
	s.expectBulkVariables()
	s.expectBulkSecrets()
	_, err = s.sweep(sweeper)

	s.Require().NoError(err)
	s.ElementsMatch([]string{"sec:" + secretOf(appB), "var:" + hostOf(appB)}, s.held.removed)
}

// A deleted resource's values stay while a version that can still be rolled back to refers to them.
func (s *UnusedValueSweeperTestSuite) TestASweepKeepsADeletedResourcesValuesWhileAVersionNeedsThem() {
	first := keep(s.versions, appA, appB)
	keep(s.versions, appA)
	s.held.holdBoth(appB, longAgo)
	s.expectListing()
	s.expectExport(appA)
	sweeper := s.sweeper(0)

	_, err := s.sweep(sweeper)
	s.Require().NoError(err)
	s.Empty(s.held.removed)

	prune(s.versions, first.Seq)
	s.expectBulkVariables()
	s.expectBulkSecrets()
	_, err = s.sweep(sweeper)

	s.Require().NoError(err)
	s.Len(s.held.removed, 2)
}

// An export of the configuration that fails, holds nothing, or left out a resource it names no ID for
// removes nothing: a value of a resource it missed would otherwise look unused.
func (s *UnusedValueSweeperTestSuite) TestASweepThatCannotExportTheConfigurationRemovesNothing() {
	leftOut := currentExport()
	leftOut.Summary = &export.ExportSummary{Errors: []declarativeresource.ExportError{
		{ResourceType: "application", ResourceID: appC}, {ResourceType: "user"}}}
	cases := map[string]struct {
		response *export.ExportResponse
		err      *tidcommon.ServiceError
	}{
		"failed":                 {err: &tidcommon.InternalServerError},
		"nil":                    {},
		"empty":                  {response: &export.ExportResponse{}},
		"left out without an ID": {response: leftOut},
	}
	for name, tc := range cases {
		s.Run(name, func() {
			s.SetupTest()
			keep(s.versions, appA)
			s.held.holdBoth(appB, longAgo)
			s.expectListing()
			s.current.EXPECT().exportConfiguration(mock.Anything).Return(tc.response, tc.err).Once()

			_, err := s.sweep(s.sweeper(0))

			s.Require().ErrorIs(err, errNoCurrentConfiguration)
			s.Empty(s.held.removed)
		})
	}
}

// The configuration is exported once per sweep, and only when something could be removed.
func (s *UnusedValueSweeperTestSuite) TestASweepExportsTheConfigurationOncePerSweep() {
	keep(s.versions, appA)
	s.expectListing()
	sweeper := s.sweeper(0)

	_, err := s.sweep(sweeper)
	s.Require().NoError(err)
	s.current.AssertNotCalled(s.T(), "exportConfiguration", mock.Anything)

	s.held.holdBoth(appB, longAgo)
	s.held.holdBoth(appC, longAgo)
	s.current.EXPECT().exportConfiguration(mock.Anything).Return(currentExport(), nil).Once()
	s.expectBulkVariables()
	s.expectBulkSecrets()
	_, err = s.sweep(sweeper)
	s.Require().NoError(err)
}

// A gateway swept within the interval is not swept again; once it has passed, the next apply sweeps.
func (s *UnusedValueSweeperTestSuite) TestASweepIsSkippedWithinTheInterval() {
	keep(s.versions, appA)
	s.expectListing()
	s.expectSynchronousRuns()
	sweeper := s.sweeper(time.Hour)

	sweeper.afterApply(context.Background(), sweepGateway, sweepKey)
	s.Require().NotEmpty(s.held.queries, "the first sweep did not run")

	s.held.queries = nil
	sweeper.afterApply(context.Background(), sweepGateway, sweepKey)
	s.Empty(s.held.queries, "a second sweep ran within the interval")

	sweeper.gateways[sweepGateway.ID].finished = time.Now().Add(-2 * time.Hour)
	sweeper.afterApply(context.Background(), sweepGateway, sweepKey)
	s.NotEmpty(s.held.queries, "no sweep ran after the interval")
	s.runner.AssertNumberOfCalls(s.T(), "run", 2)
}

// With no interval every apply sweeps, and the interval of one gateway does not hold back another.
func (s *UnusedValueSweeperTestSuite) TestASweepWithNoIntervalRunsAfterEveryApply() {
	keep(s.versions, appA)
	s.store.EXPECT().ListVariables(mock.Anything, mock.Anything, sweepKey, mock.Anything).
		RunAndReturn(s.held.listVariables).Times(3)
	s.store.EXPECT().ListSecrets(mock.Anything, mock.Anything, sweepKey, mock.Anything).
		RunAndReturn(s.held.listSecrets).Times(3)
	s.expectSynchronousRuns()
	sweeper := s.sweeper(0)

	for range 2 {
		sweeper.afterApply(context.Background(), sweepGateway, sweepKey)
	}

	throttled := s.sweeper(time.Hour)
	throttled.gateways[sweepGateway.ID] = &gatewaySweep{finished: time.Now()}
	throttled.afterApply(context.Background(), &Gateway{ID: "gw-2", BaseURL: "https://dp2.test"}, sweepKey)
	s.runner.AssertNumberOfCalls(s.T(), "run", 3)
}

// Unused values are removed in calls of at most 100 names each, and no more than fit in a request
// line a proxy takes, variables and secrets apart.
func (s *UnusedValueSweeperTestSuite) TestASweepRemovesInBatches() {
	keep(s.versions, appA)
	for i := range 250 {
		s.held.holdBoth(fmt.Sprintf("0198f3a1-7c2e-7b10-9a44-%012d", i), longAgo)
	}
	s.expectListing()
	s.expectExport()
	s.expectBulkVariables()
	s.expectBulkSecrets()

	outcome, err := s.sweep(s.sweeper(0))

	s.Require().NoError(err)
	s.Equal(250, outcome.removedVariables)
	s.Equal(250, outcome.removedSecrets)
	sizes := make([]int, 0, len(s.held.bulkCalls))
	for _, call := range s.held.bulkCalls {
		sizes = append(sizes, len(call))
		sent := url.Values{"names": {strings.Join(call, ",")}}.Encode()
		s.LessOrEqual(len(sent)-len("names="), maxNamesQueryBytes)
	}
	// A host's name is short enough for 100 in a call; a client secret's is not.
	s.Equal([]int{100, 100, 50, 97, 97, 56}, sizes)
	s.Empty(s.held.variables)
	s.Empty(s.held.secrets)
}

// What is counted removed is what the gateway says it removed, not what it was asked to.
func (s *UnusedValueSweeperTestSuite) TestASweepCountsWhatTheGatewayRemoved() {
	keep(s.versions, appA)
	s.held.holdBoth(appB, longAgo)
	s.held.holdBoth(appC, longAgo)
	s.expectListing()
	s.expectExport()
	// Another caller removed appC's host first.
	s.store.EXPECT().DeleteVariables(mock.Anything, mock.Anything, sweepKey, mock.Anything, sweepAge()).
		Return([]string{hostOf(appB)}, nil).Once()
	s.expectBulkSecrets()

	outcome, err := s.sweep(s.sweeper(0))

	s.Require().NoError(err)
	s.Equal(1, outcome.removedVariables)
	s.Equal(2, outcome.removedSecrets)
}

// A gateway that predates the removal of many names, answering 404 or 405, has no way to keep a value
// set again since the listing, so nothing is removed from it: the sweep stops after the one refused
// call, asks for no secrets, and ends without a failure. It is said at warning level once per gateway.
func (s *UnusedValueSweeperTestSuite) TestASweepOfAGatewayWithoutTheRemovalOfManyNamesRemovesNothing() {
	for _, status := range []int{http.StatusNotFound, http.StatusMethodNotAllowed} {
		s.Run(strconv.Itoa(status), func() {
			s.SetupTest()
			keep(s.versions, appA)
			for i := range 150 {
				s.held.holdBoth(fmt.Sprintf("0198f3a1-7c2e-7b10-9a44-%012d", i), longAgo)
			}
			s.expectListing()
			s.expectExport()
			s.store.EXPECT().DeleteVariables(mock.Anything, mock.Anything, sweepKey, mock.Anything, sweepAge()).
				Return(nil, &StoreRefusal{Status: status, Body: []byte(`{}`)}).Twice()
			s.expectSynchronousRuns()
			sweeper := s.sweeper(0)

			sweeper.afterApply(context.Background(), sweepGateway, sweepKey)
			s.True(sweeper.gateways[sweepGateway.ID].toldNoBulkDelete)
			outcome, err := s.sweep(sweeper)

			s.Require().NoError(err)
			s.Zero(outcome.removedVariables)
			s.Zero(outcome.removedSecrets)
			s.Len(s.held.variables, 150)
			s.Len(s.held.secrets, 150)
			s.store.AssertNotCalled(s.T(), "DeleteSecrets", mock.Anything, mock.Anything, mock.Anything,
				mock.Anything, mock.Anything)
		})
	}
}

// Stop cancels a running sweep, waits for it to end, and starts no sweep after it; a second Stop
// returns at once.
func (s *UnusedValueSweeperTestSuite) TestStopCancelsARunningSweepAndStartsNoMore() {
	keep(s.versions, appA)
	sweeper := s.sweeper(0)
	listing := make(chan struct{})
	ended := make(chan struct{})
	var cancelled error
	s.store.EXPECT().ListVariables(mock.Anything, mock.Anything, sweepKey, mock.Anything).RunAndReturn(
		func(ctx context.Context, _ *Gateway, _ string,
			_ StoreListQuery) (*variablestore.VariableListResponse, error) {
			close(listing)
			<-ctx.Done()
			cancelled = ctx.Err()
			return nil, ctx.Err()
		}).Once()
	s.runner.EXPECT().run(mock.Anything).Run(func(sweep func()) {
		go func() {
			defer close(ended)
			sweep()
		}()
	}).Once()

	sweeper.afterApply(context.Background(), sweepGateway, sweepKey)
	<-listing
	sweeper.Stop()

	// Stop returns once the sweep marked itself ended; the wrapper that ran it closes ended just after.
	select {
	case <-ended:
	case <-time.After(5 * time.Second):
		s.FailNow("the sweep did not end after Stop")
	}
	s.ErrorIs(cancelled, context.Canceled)
	sweeper.mu.Lock()
	s.False(sweeper.gateways[sweepGateway.ID].running, "Stop returned before the sweep ended")
	sweeper.mu.Unlock()

	sweeper.afterApply(context.Background(), sweepGateway, sweepKey)
	sweeper.Stop()
	s.runner.AssertNumberOfCalls(s.T(), "run", 1)
}

// Stop with no sweep running returns at once, and a sweeper that is off has nothing to stop.
func (s *UnusedValueSweeperTestSuite) TestStopWithNothingRunningReturnsAtOnce() {
	done := make(chan struct{})
	go func() {
		s.sweeper(0).Stop()
		noUnusedValueSweep{}.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(sweepStopTimeout / 2):
		s.Fail("Stop waited with nothing running")
	}
}

// Any other refusal of a removal of many names stops the sweep, without falling back.
func (s *UnusedValueSweeperTestSuite) TestASweepStopsOnABulkRemovalFailure() {
	keep(s.versions, appA)
	s.held.holdBoth(appB, longAgo)
	s.expectListing()
	s.expectExport()
	s.store.EXPECT().DeleteVariables(mock.Anything, mock.Anything, sweepKey, mock.Anything, mock.Anything).
		Return(nil, &StoreRefusal{Status: http.StatusInternalServerError, Body: []byte(`{}`)}).Once()

	_, err := s.sweep(s.sweeper(0))

	var refusal *StoreRefusal
	s.Require().ErrorAs(err, &refusal)
	s.Empty(s.held.removed)
}

// A resource the export left out, such as one deleted while it ran, keeps its values, whatever its ID
// looks like; the other unused values are still removed.
func (s *UnusedValueSweeperTestSuite) TestASweepKeepsTheValuesOfAResourceTheExportLeftOut() {
	const declaredID = "my-declared-app"
	keep(s.versions, appA)
	s.held.holdBoth(appB, longAgo)
	s.held.holdBoth(appC, longAgo)
	s.held.holdBoth(declaredID, longAgo)
	current := currentExport(appA)
	current.Summary = &export.ExportSummary{Errors: []declarativeresource.ExportError{
		{ResourceType: "agent", ResourceID: appB, Error: "Agent not found"},
		{ResourceType: "application", ResourceID: declaredID, Error: "Application not found"},
	}}
	s.expectListing()
	s.current.EXPECT().exportConfiguration(mock.Anything).Return(current, nil).Once()
	s.expectBulkVariables()
	s.expectBulkSecrets()

	outcome, err := s.sweep(s.sweeper(0))

	s.Require().NoError(err)
	s.ElementsMatch([]string{"sec:" + secretOf(appC), "var:" + hostOf(appC)}, s.held.removed)
	s.Equal(1, outcome.removedVariables)
	s.Equal(1, outcome.removedSecrets)
	s.Contains(s.held.secrets, secretOf(appB))
	s.Contains(s.held.variables, hostOf(declaredID))
}
