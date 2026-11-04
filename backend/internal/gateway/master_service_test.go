// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// keepVersions configures how many versions, and so how many masters per gateway, are kept, and puts
// the configuration the package's tests run with back afterwards. Without one, one is kept.
func keepVersions(t *testing.T, count int) {
	t.Helper()
	bound := shippedGatewayBound
	config.ResetServerRuntime()
	require.NoError(t, config.InitializeServerRuntime("", &config.Config{
		Gateway: config.GatewayConfig{MaxGateways: &bound, AllowPrivateAddresses: true, MaxVersions: &count},
	}))
	t.Cleanup(func() { configureGatewayBound(shippedGatewayBound) })
}

type MasterServiceTestSuite struct {
	suite.Suite
}

func TestMasterServiceTestSuite(t *testing.T) {
	suite.Run(t, new(MasterServiceTestSuite))
}

// Every test starts from the package's shipped configuration and leaves it there: the retention bound
// is read from the runtime, so a test that set it must not decide the next test's.
func (s *MasterServiceTestSuite) SetupTest() {
	config.ResetServerRuntime()
	configureGatewayBound(shippedGatewayBound)
}

func (s *MasterServiceTestSuite) TearDownTest() {
	config.ResetServerRuntime()
	configureGatewayBound(shippedGatewayBound)
}

// A gateway's masters are kept within the same bound as the versions.
func (s *MasterServiceTestSuite) TestMastersBeyondTheBoundArePruned() {
	f := newVersionFixture(s.T())
	f.capture(s.T(), "ou-a")
	f.capture(s.T(), "ou-a", "ou-b")
	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(1)})
	s.Require().Nil(svcErr)
	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(2)})
	s.Require().Nil(svcErr)

	masters, svcErr := f.svc.ListMasters(context.Background(), "gw-1")

	s.Require().Nil(svcErr)
	s.Require().Len(masters, maxVersions())
	s.Equal(2, masters[0].Seq, "the newest master was pruned")
}

// An apply records the gateway's master from the import's own outcomes, resource by resource, and
// answers with it.
func (s *MasterServiceTestSuite) TestAnApplyRecordsTheMasterFromTheImportsOutcomes() {
	f := newVersionFixture(s.T())
	f.capture(s.T(), "ou-a", "ou-b")
	f.client.imported = answerOf(wrote("ou-a"), refused("ou-b", "ou-b", "OU-1"))
	alice := signedIn("u-1", map[string]interface{}{"username": "alice"})

	result, svcErr := f.svc.Apply(alice, "gw-1", ApplyRequest{Origin: " dev "})

	s.Require().Nil(svcErr)
	s.Require().Len(result.Master, 64)
	master, svcErr := f.svc.GetMaster(context.Background(), "gw-1")
	s.Require().Nil(svcErr)
	s.Equal(result.Master, master.Hash)
	s.Equal(1, master.Seq)
	s.Equal(&Actor{ID: "u-1", Name: "alice"}, master.CreatedBy)
	s.Equal(MasterSummary{Deployed: 1, Failed: 1}, master.Summary)
	s.Require().Len(master.Entries, 2)
	s.Equal(EntryDeployed, master.Entries[0].State)
	s.Equal("dev", master.Entries[0].Origin)
	s.Equal(f.hash(1), master.Entries[0].SourceVersion)
	s.Equal(&Actor{ID: "u-1", Name: "alice"}, master.Entries[0].By)
	s.Equal(EntryFailed, master.Entries[1].State)
	s.Equal("OU-1", master.Entries[1].Failure.Code)
	s.Equal(unit("ou-a"), master.Resources, "the master holds what the gateway accepted")
}

// An origin longer than a master records is refused before anything is sent; one at the bound is kept.
func (s *MasterServiceTestSuite) TestAnOriginLongerThanTheBoundIsRefused() {
	f := newVersionFixture(s.T())
	f.capture(s.T(), "ou-a")

	_, svcErr := f.svc.Apply(context.Background(), "gw-1",
		ApplyRequest{Origin: strings.Repeat("é", maxOriginLength+1)})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorOriginTooLong.Code, svcErr.Code)
	s.Empty(f.versions.masters["gw-1"], "a refused apply recorded a master")

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Origin: strings.Repeat("é", maxOriginLength)})
	s.Require().Nil(svcErr)
}

// A later apply sends only what differs, and the master carries the rest over from before; a dry run
// writes no master.
func (s *MasterServiceTestSuite) TestEachApplyWritesANewMasterAndADryRunNone() {
	keepVersions(s.T(), 10)
	f := newVersionFixture(s.T())
	f.capture(s.T(), "ou-a")
	f.capture(s.T(), "ou-a", "ou-b")
	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(1)})
	s.Require().Nil(svcErr)

	dry, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(2), DryRun: true})
	s.Require().Nil(svcErr)
	s.Empty(dry.Master)
	masters, _ := f.svc.ListMasters(context.Background(), "gw-1")
	s.Len(masters, 1, "a dry run wrote a master")

	f.client.imported = answerOf(wrote("ou-b"))
	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(2)})
	s.Require().Nil(svcErr)

	master, _ := f.svc.GetMaster(context.Background(), "gw-1")
	s.Equal(2, master.Seq)
	s.Require().Len(master.Entries, 2)
	s.Equal(f.hash(1), master.Entries[0].SourceVersion, "a resource that did not change was rewritten")
	s.Equal(f.hash(2), master.Entries[1].SourceVersion)
	masters, _ = f.svc.ListMasters(context.Background(), "gw-1")
	s.Require().Len(masters, 2)
	s.Equal([]int{2, 1}, []int{masters[0].Seq, masters[1].Seq})
	s.Nil(masters[0].Entries, "a listing carried the entries")
	s.Empty(masters[0].Resources)
}

// An apply whose removal the gateway refused records the version as partly applied, and the master
// says what the gateway holds.
func (s *MasterServiceTestSuite) TestTheMasterKeepsAResourceWhoseRemovalWasRefused() {
	f := newVersionFixture(s.T())
	f.capture(s.T(), "ou-a", "ou-b")
	f.capture(s.T(), "ou-a")
	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(1)})
	s.Require().Nil(svcErr)
	f.client.imported = answerOf(importOutcome{ResourceType: "organization_unit", ResourceID: "ou-b",
		Operation: operationDeleted, Status: outcomeFailed, Code: "OU-9", Message: "in use"})

	result, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(2)})

	s.Require().Nil(svcErr)
	s.True(result.Recorded)
	s.False(result.Complete)
	s.NotEmpty(result.Master)
	master, _ := f.svc.GetMaster(context.Background(), "gw-1")
	s.Equal(2, master.Seq)
	entry, ok := entryOf(master, "organization_unit/ou-b")
	s.Require().True(ok)
	s.Equal(EntryFailed, entry.State)
	s.Equal("in use", entry.Failure.Message)
	s.Contains(master.Resources, "ou-b", "a resource the gateway kept was dropped from the master")
}

// A gateway that applied a version before masters were recorded is given its first master from that
// version, less what it left alone, when its master is first read.
func (s *MasterServiceTestSuite) TestAGatewayWithAnAppliedVersionIsGivenItsFirstMaster() {
	f := newVersionFixture(s.T())
	f.capture(s.T(), "ou-a", "ou-b")
	bob := signedIn("u-2", nil)
	_, svcErr := f.svc.Apply(bob, "gw-1", ApplyRequest{})
	s.Require().Nil(svcErr)
	// As a gateway stood before masters were recorded: a version applied, and a unit left alone.
	delete(f.versions.masters, "gw-1")
	f.versions.excluded = map[string]map[string]excludedResource{"gw-1": {
		"organization_unit/ou-b": {Key: "organization_unit/ou-b", Type: "organization_unit", ID: "ou-b"}}}

	master, svcErr := f.svc.GetMaster(context.Background(), "gw-1")

	s.Require().Nil(svcErr)
	s.Equal(1, master.Seq)
	s.Require().Len(master.Entries, 1)
	s.Equal("organization_unit/ou-a", master.Entries[0].Key)
	s.Equal(f.hash(1), master.Entries[0].SourceVersion)
	s.Equal(&Actor{ID: "u-2", Name: "bob"}, master.Entries[0].By)
	s.Equal(&Actor{ID: "u-2", Name: "bob"}, master.CreatedBy)
	s.Len(f.versions.masters["gw-1"], 1, "the first master was not kept")

	again, svcErr := f.svc.GetMaster(context.Background(), "gw-1")
	s.Require().Nil(svcErr)
	s.Equal(master.Hash, again.Hash)
	s.Len(f.versions.masters["gw-1"], 1, "a second read gave the gateway another first master")
}

// A gateway nothing was applied to holds the empty master, which is not kept, and has no history.
func (s *MasterServiceTestSuite) TestAGatewayNothingWasAppliedToHoldsTheEmptyMaster() {
	f := newVersionFixture(s.T())

	master, svcErr := f.svc.GetMaster(context.Background(), "gw-1")

	s.Require().Nil(svcErr)
	s.Equal(&Master{GatewayID: "gw-1", Entries: []MasterEntry{}}, master)
	s.Empty(f.versions.masters)
	masters, svcErr := f.svc.ListMasters(context.Background(), "gw-1")
	s.Require().Nil(svcErr)
	s.Empty(masters)
	_, svcErr = f.svc.GetResourceHistory(context.Background(), "gw-1", "organization_unit/ou-a")
	s.Equal(&ErrorResourceNotDeployed, svcErr)
}

// A resource's history is its entry across the gateway's masters, newest first; a resource no master
// records is not found.
func (s *MasterServiceTestSuite) TestAResourcesHistoryIsItsEntryAcrossTheMasters() {
	keepVersions(s.T(), 10)
	f := newVersionFixture(s.T())
	f.capture(s.T(), "ou-a", "ou-b")
	f.capture(s.T(), "ou-a")
	_, svcErr := f.svc.Apply(signedIn("u-1", nil), "gw-1", ApplyRequest{Version: f.hash(1)})
	s.Require().Nil(svcErr)
	f.client.imported = answerOf(removed("ou-b"))
	_, svcErr = f.svc.Apply(signedIn("u-2", nil), "gw-1", ApplyRequest{Version: f.hash(2)})
	s.Require().Nil(svcErr)

	history, svcErr := f.svc.GetResourceHistory(context.Background(), "gw-1", "organization_unit/ou-b")

	s.Require().Nil(svcErr)
	s.Equal("organization_unit/ou-b", history.Key)
	s.Require().Len(history.History, 2)
	s.Equal(2, history.History[0].Number)
	s.Equal(EntryRemoved, history.History[0].Entry.State)
	s.Equal(&Actor{ID: "u-2", Name: "bob"}, history.History[0].Entry.By)
	s.Equal(1, history.History[1].Number)
	s.Equal(EntryDeployed, history.History[1].Entry.State)
	s.Equal(&Actor{ID: "u-1", Name: "alice"}, history.History[1].Entry.By)

	_, svcErr = f.svc.GetResourceHistory(context.Background(), "gw-1", "organization_unit/ou-z")
	s.Equal(&ErrorResourceNotDeployed, svcErr)
}

// A master that cannot be recorded fails the apply after the gateway wrote, like the applied version;
// a store that cannot answer fails every read; a gateway that is not registered is not found.
func (s *MasterServiceTestSuite) TestMasterFailuresAreReported() {
	f := newVersionFixture(s.T())
	f.capture(s.T(), "ou-a")
	f.versions.masterErr = errors.New("database is down")

	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	s.Equal(&tidcommon.InternalServerError, svcErr)
	_, svcErr = f.svc.GetMaster(context.Background(), "gw-1")
	s.Equal(&tidcommon.InternalServerError, svcErr)
	_, svcErr = f.svc.ListMasters(context.Background(), "gw-1")
	s.Equal(&tidcommon.InternalServerError, svcErr)
	_, svcErr = f.svc.GetResourceHistory(context.Background(), "gw-1", "organization_unit/ou-a")
	s.Equal(&tidcommon.InternalServerError, svcErr)

	f.versions.masterErr = nil
	for _, call := range []func() *tidcommon.ServiceError{
		func() *tidcommon.ServiceError { _, err := f.svc.GetMaster(context.Background(), "gw-9"); return err },
		func() *tidcommon.ServiceError { _, err := f.svc.ListMasters(context.Background(), "gw-9"); return err },
		func() *tidcommon.ServiceError {
			_, err := f.svc.GetResourceHistory(context.Background(), "gw-9", "organization_unit/ou-a")
			return err
		},
	} {
		s.Equal(&ErrorGatewayNotFound, call())
	}
}

// Two reads giving a gateway its first master at once write the same master; the one refused answers
// with what the other kept rather than failing.
func (s *MasterServiceTestSuite) TestAFirstMasterCollidingWithAnotherTakesTheOneKept() {
	f := newVersionFixture(s.T())
	f.capture(s.T(), "ou-a")
	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	s.Require().Nil(svcErr)
	other := f.versions.masters["gw-1"][0]
	f.versions.masters = map[string][]Master{}
	store := &collidingMasterStore{fakeVersionStore: f.versions, winner: &other}
	f.svc = newVersionService(f.gateways, store, f.exporter, f.client, testSecretFields, f.names)

	master, svcErr := f.svc.GetMaster(context.Background(), "gw-1")

	s.Require().Nil(svcErr)
	s.Equal(other.Hash, master.Hash)
}

// An apply whose master collides with another write is refused: the master kept does not hold this
// apply's outcomes, so it is not answered as this apply's.
func (s *MasterServiceTestSuite) TestAnApplyWhoseMasterCollidesIsRefused() {
	f := newVersionFixture(s.T())
	f.capture(s.T(), "ou-a", "ou-b")
	f.versions.masters = map[string][]Master{"gw-1": {*deployedMaster("other", "ou-a")}}
	f.versions.masters["gw-1"][0].Seq = 1
	store := &collidingMasterStore{fakeVersionStore: f.versions}
	f.svc = newVersionService(f.gateways, store, f.exporter, f.client, testSecretFields, f.names)

	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})

	s.Equal(&ErrorAppliedChanged, svcErr)
}

// collidingMasterStore refuses every master written, as a number taken by another write does. With a
// winner, that write is the winner's, which is kept in its place.
type collidingMasterStore struct {
	*fakeVersionStore
	winner *Master
}

func (c *collidingMasterStore) AddMaster(_ context.Context, master Master, _ string) (*Master, error) {
	if c.winner != nil {
		c.masters[master.GatewayID] = append(c.masters[master.GatewayID], *c.winner)
	}
	return nil, fmt.Errorf("%w: UNIQUE constraint failed", errMasterNumberTaken)
}

// An apply whose lease another pod took over while its import was in flight records neither its
// master nor the version: the gateway stays recorded as the apply that took over leaves it.
func (s *MasterServiceTestSuite) TestAnApplySupersededByATakeOverRecordsNothing() {
	f := newVersionFixture(s.T())
	f.capture(s.T(), "ou-a")
	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	s.Require().Nil(svcErr)
	f.capture(s.T(), "ou-a", "ou-b")
	f.client.onImport = func() { f.versions.leases["gw-1"].Token = "another-pod" }

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})

	s.Equal(&ErrorAppliedChanged, svcErr)
	s.Len(f.versions.masters["gw-1"], 1, "a superseded apply recorded its master")
	s.Equal(1, f.versions.applied["gw-1"].AppliedVersion)
}

// An apply whose removals the gateway did not complete records only its master, so the lease is all
// that keeps a superseded one from recording it.
func (s *MasterServiceTestSuite) TestASupersededApplyWithRemovalsLeftRecordsNoMaster() {
	f := newVersionFixture(s.T())
	f.capture(s.T(), "ou-a", "ou-b")
	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	s.Require().Nil(svcErr)
	f.capture(s.T(), "ou-a")
	refusedRemoval := removed("ou-b")
	refusedRemoval.Status = outcomeFailed
	f.client.imported = answerOf(refusedRemoval)
	f.client.onImport = func() { f.versions.leases["gw-1"].Token = "another-pod" }

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})

	s.Equal(&ErrorAppliedChanged, svcErr)
	s.Len(f.versions.masters["gw-1"], 1, "a superseded apply recorded its master")
}

// The master and the version recorded as applied are written together: when recording the version is
// refused, the master written with it is rolled back.
func (s *MasterServiceTestSuite) TestARefusedRecordRollsBackTheMaster() {
	f := newVersionFixture(s.T())
	f.capture(s.T(), "ou-a")
	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	s.Require().Nil(svcErr)
	f.capture(s.T(), "ou-a", "ou-b")
	f.versions.setErr = errAppliedChanged

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})

	s.Equal(&ErrorAppliedChanged, svcErr)
	s.Len(f.versions.masters["gw-1"], 1, "the master outlived the record it was written with")
}

// An apply that changes nothing the master records keeps the master it has rather than adding the
// same one again.
func (s *MasterServiceTestSuite) TestARepeatedApplyKeepsTheMaster() {
	f := newVersionFixture(s.T())
	f.capture(s.T(), "ou-a")
	first, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	s.Require().Nil(svcErr)

	again, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})

	s.Require().Nil(svcErr)
	s.Equal(first.Master, again.Master)
	s.Len(f.versions.masters["gw-1"], 1)
}

// The same resource refused again for another reason is a new master, so the failure shown is the
// latest.
func (s *MasterServiceTestSuite) TestARefusalForAnotherReasonIsANewMaster() {
	f := newVersionFixture(s.T())
	f.capture(s.T(), "ou-a")
	f.client.imported = answerOf(refused("ou-a", "ou-a", "OU-1"))
	first, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	s.Require().Nil(svcErr)

	f.client.imported = answerOf(refused("ou-a", "ou-a", "OU-2"))
	again, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})

	s.Require().Nil(svcErr)
	s.NotEqual(first.Master, again.Master)
	master, svcErr := f.svc.GetMaster(context.Background(), "gw-1")
	s.Require().Nil(svcErr)
	s.Require().Len(master.Entries, 1)
	s.Equal("OU-2", master.Entries[0].Failure.Code)
}

// A master that fails to record for any reason but a collision fails the apply, rather than answering
// with the master the gateway held before.
func (s *MasterServiceTestSuite) TestAMasterThatFailsToRecordFailsTheApply() {
	f := newVersionFixture(s.T())
	f.capture(s.T(), "ou-a")
	f.versions.masters = map[string][]Master{"gw-1": {*deployedMaster("other", "ou-b")}}
	f.versions.masters["gw-1"][0].Seq = 1
	store := &failingMasterStore{fakeVersionStore: f.versions}
	f.svc = newVersionService(&fakeStore{gateways: []Gateway{{ID: "gw-1", BaseURL: "https://dp.test",
		Key: "the-key"}}}, store, f.exporter, f.client, testSecretFields, f.names)

	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

// failingMasterStore fails every master written, as a database that cannot take the write does.
type failingMasterStore struct {
	*fakeVersionStore
}

func (c *failingMasterStore) AddMaster(context.Context, Master, string) (*Master, error) {
	return nil, errors.New("statement timeout")
}

// Forgetting a gateway drops its masters with the rest.
func (s *MasterServiceTestSuite) TestForgetDropsTheGatewaysMasters() {
	f := newVersionFixture(s.T())
	f.capture(s.T(), "ou-a")
	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	s.Require().Nil(svcErr)
	s.Require().NotEmpty(f.versions.masters["gw-1"])

	f.svc.Forget(context.Background(), "gw-1")

	s.Empty(f.versions.masters["gw-1"])
}

// The gateway's own answer is passed to the master as it came, so an answer with no results marks
// every sent resource as not known to be written.
func (s *MasterServiceTestSuite) TestAnAnswerWithoutResultsLeavesEveryResourceFailed() {
	f := newVersionFixture(s.T())
	f.capture(s.T(), "ou-a")
	f.client.imported = json.RawMessage(`{"summary":{"failed":0}}`)

	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})

	s.Require().Nil(svcErr)
	master, _ := f.svc.GetMaster(context.Background(), "gw-1")
	s.Equal(MasterSummary{Failed: 1}, master.Summary)
}
