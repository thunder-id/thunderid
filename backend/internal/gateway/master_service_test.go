// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

// A gateway's masters are kept within the same bound as the versions.
func TestMastersBeyondTheBoundArePruned(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a")
	f.capture(t, "ou-a", "ou-b")
	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(1)})
	require.Nil(t, svcErr)
	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(2)})
	require.Nil(t, svcErr)

	masters, svcErr := f.svc.ListMasters(context.Background(), "gw-1")

	require.Nil(t, svcErr)
	require.Len(t, masters, maxVersions())
	assert.Equal(t, 2, masters[0].Seq, "the newest master was pruned")
}

// An apply records the gateway's master from the import's own outcomes, resource by resource, and
// answers with it.
func TestAnApplyRecordsTheMasterFromTheImportsOutcomes(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a", "ou-b")
	f.client.imported = answerOf(wrote("ou-a"), refused("ou-b", "ou-b", "OU-1"))
	alice := signedIn("u-1", map[string]interface{}{"username": "alice"})

	result, svcErr := f.svc.Apply(alice, "gw-1", ApplyRequest{Origin: " dev "})

	require.Nil(t, svcErr)
	require.Len(t, result.Master, 64)
	master, svcErr := f.svc.GetMaster(context.Background(), "gw-1")
	require.Nil(t, svcErr)
	assert.Equal(t, result.Master, master.Hash)
	assert.Equal(t, 1, master.Seq)
	assert.Equal(t, &Actor{ID: "u-1", Name: "alice"}, master.CreatedBy)
	assert.Equal(t, MasterSummary{Deployed: 1, Failed: 1}, master.Summary)
	require.Len(t, master.Entries, 2)
	assert.Equal(t, EntryDeployed, master.Entries[0].State)
	assert.Equal(t, "dev", master.Entries[0].Origin)
	assert.Equal(t, f.hash(1), master.Entries[0].SourceVersion)
	assert.Equal(t, &Actor{ID: "u-1", Name: "alice"}, master.Entries[0].By)
	assert.Equal(t, EntryFailed, master.Entries[1].State)
	assert.Equal(t, "OU-1", master.Entries[1].Failure.Code)
	assert.Equal(t, unit("ou-a"), master.Resources, "the master holds what the gateway accepted")
}

// A later apply sends only what differs, and the master carries the rest over from before; a dry run
// writes no master.
func TestEachApplyWritesANewMasterAndADryRunNone(t *testing.T) {
	keepVersions(t, 10)
	f := newVersionFixture(t)
	f.capture(t, "ou-a")
	f.capture(t, "ou-a", "ou-b")
	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(1)})
	require.Nil(t, svcErr)

	dry, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(2), DryRun: true})
	require.Nil(t, svcErr)
	assert.Empty(t, dry.Master)
	masters, _ := f.svc.ListMasters(context.Background(), "gw-1")
	assert.Len(t, masters, 1, "a dry run wrote a master")

	f.client.imported = answerOf(wrote("ou-b"))
	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(2)})
	require.Nil(t, svcErr)

	master, _ := f.svc.GetMaster(context.Background(), "gw-1")
	assert.Equal(t, 2, master.Seq)
	require.Len(t, master.Entries, 2)
	assert.Equal(t, f.hash(1), master.Entries[0].SourceVersion, "a resource that did not change was rewritten")
	assert.Equal(t, f.hash(2), master.Entries[1].SourceVersion)
	masters, _ = f.svc.ListMasters(context.Background(), "gw-1")
	require.Len(t, masters, 2)
	assert.Equal(t, []int{2, 1}, []int{masters[0].Seq, masters[1].Seq})
	assert.Nil(t, masters[0].Entries, "a listing carried the entries")
	assert.Empty(t, masters[0].Resources)
}

// A master is recorded whatever the applied version does: an apply whose removal the gateway refused
// is not recorded as the applied version, but the master says what the gateway holds.
func TestTheMasterIsRecordedEvenWhenTheAppliedVersionIsNot(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a", "ou-b")
	f.capture(t, "ou-a")
	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(1)})
	require.Nil(t, svcErr)
	f.client.imported = answerOf(importOutcome{ResourceType: "organization_unit", ResourceID: "ou-b",
		Operation: operationDeleted, Status: outcomeFailed, Code: "OU-9", Message: "in use"})

	result, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(2)})

	require.Nil(t, svcErr)
	assert.False(t, result.Recorded)
	assert.NotEmpty(t, result.Master)
	master, _ := f.svc.GetMaster(context.Background(), "gw-1")
	assert.Equal(t, 2, master.Seq)
	entry, ok := entryOf(master, "organization_unit/ou-b")
	require.True(t, ok)
	assert.Equal(t, EntryFailed, entry.State)
	assert.Equal(t, "in use", entry.Failure.Message)
	assert.Contains(t, master.Resources, "ou-b", "a resource the gateway kept was dropped from the master")
}

// A gateway that applied a version before masters were recorded is given its first master from that
// version, less what it left alone, when its master is first read.
func TestAGatewayWithAnAppliedVersionIsGivenItsFirstMaster(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a", "ou-b")
	bob := signedIn("u-2", nil)
	_, svcErr := f.svc.Apply(bob, "gw-1", ApplyRequest{Selection: &[]string{"organization_unit/ou-a"}})
	require.Nil(t, svcErr)
	// As a gateway stood before masters were recorded.
	delete(f.versions.masters, "gw-1")

	master, svcErr := f.svc.GetMaster(context.Background(), "gw-1")

	require.Nil(t, svcErr)
	assert.Equal(t, 1, master.Seq)
	require.Len(t, master.Entries, 1)
	assert.Equal(t, "organization_unit/ou-a", master.Entries[0].Key)
	assert.Equal(t, f.hash(1), master.Entries[0].SourceVersion)
	assert.Equal(t, &Actor{ID: "u-2", Name: "bob"}, master.Entries[0].By)
	assert.Equal(t, &Actor{ID: "u-2", Name: "bob"}, master.CreatedBy)
	assert.Len(t, f.versions.masters["gw-1"], 1, "the first master was not kept")

	again, svcErr := f.svc.GetMaster(context.Background(), "gw-1")
	require.Nil(t, svcErr)
	assert.Equal(t, master.Hash, again.Hash)
	assert.Len(t, f.versions.masters["gw-1"], 1, "a second read gave the gateway another first master")
}

// A gateway nothing was applied to holds the empty master, which is not kept, and has no history.
func TestAGatewayNothingWasAppliedToHoldsTheEmptyMaster(t *testing.T) {
	f := newVersionFixture(t)

	master, svcErr := f.svc.GetMaster(context.Background(), "gw-1")

	require.Nil(t, svcErr)
	assert.Equal(t, &Master{GatewayID: "gw-1", Entries: []MasterEntry{}}, master)
	assert.Empty(t, f.versions.masters)
	masters, svcErr := f.svc.ListMasters(context.Background(), "gw-1")
	require.Nil(t, svcErr)
	assert.Empty(t, masters)
	_, svcErr = f.svc.GetResourceHistory(context.Background(), "gw-1", "organization_unit/ou-a")
	assert.Equal(t, &ErrorResourceNotDeployed, svcErr)
}

// A resource's history is its entry across the gateway's masters, newest first; a resource no master
// records is not found.
func TestAResourcesHistoryIsItsEntryAcrossTheMasters(t *testing.T) {
	keepVersions(t, 10)
	f := newVersionFixture(t)
	f.capture(t, "ou-a", "ou-b")
	f.capture(t, "ou-a")
	_, svcErr := f.svc.Apply(signedIn("u-1", nil), "gw-1", ApplyRequest{Version: f.hash(1)})
	require.Nil(t, svcErr)
	f.client.imported = answerOf(removed("ou-b"))
	_, svcErr = f.svc.Apply(signedIn("u-2", nil), "gw-1", ApplyRequest{Version: f.hash(2)})
	require.Nil(t, svcErr)

	history, svcErr := f.svc.GetResourceHistory(context.Background(), "gw-1", "organization_unit/ou-b")

	require.Nil(t, svcErr)
	assert.Equal(t, "organization_unit/ou-b", history.Key)
	require.Len(t, history.History, 2)
	assert.Equal(t, 2, history.History[0].Number)
	assert.Equal(t, EntryRemoved, history.History[0].Entry.State)
	assert.Equal(t, &Actor{ID: "u-2", Name: "bob"}, history.History[0].Entry.By)
	assert.Equal(t, 1, history.History[1].Number)
	assert.Equal(t, EntryDeployed, history.History[1].Entry.State)
	assert.Equal(t, &Actor{ID: "u-1", Name: "alice"}, history.History[1].Entry.By)

	_, svcErr = f.svc.GetResourceHistory(context.Background(), "gw-1", "organization_unit/ou-z")
	assert.Equal(t, &ErrorResourceNotDeployed, svcErr)
}

// A master that cannot be recorded fails the apply after the gateway wrote, like the applied version;
// a store that cannot answer fails every read; a gateway that is not registered is not found.
func TestMasterFailuresAreReported(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a")
	f.versions.masterErr = errors.New("database is down")

	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	assert.Equal(t, &tidcommon.InternalServerError, svcErr)
	_, svcErr = f.svc.GetMaster(context.Background(), "gw-1")
	assert.Equal(t, &tidcommon.InternalServerError, svcErr)
	_, svcErr = f.svc.ListMasters(context.Background(), "gw-1")
	assert.Equal(t, &tidcommon.InternalServerError, svcErr)
	_, svcErr = f.svc.GetResourceHistory(context.Background(), "gw-1", "organization_unit/ou-a")
	assert.Equal(t, &tidcommon.InternalServerError, svcErr)

	f.versions.masterErr = nil
	for _, call := range []func() *tidcommon.ServiceError{
		func() *tidcommon.ServiceError { _, err := f.svc.GetMaster(context.Background(), "gw-9"); return err },
		func() *tidcommon.ServiceError { _, err := f.svc.ListMasters(context.Background(), "gw-9"); return err },
		func() *tidcommon.ServiceError {
			_, err := f.svc.GetResourceHistory(context.Background(), "gw-9", "organization_unit/ou-a")
			return err
		},
	} {
		assert.Equal(t, &ErrorGatewayNotFound, call())
	}
}

// Two applies recording a gateway's master at the same moment collide on its number; the one refused
// answers with what the other kept rather than failing.
func TestAMasterCollidingWithAnotherAnswersWithTheOneKept(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a")
	other := deployedMaster("other", "ou-a")
	f.versions.masters = map[string][]Master{"gw-1": {*other}}
	f.versions.masterErr = errors.New("UNIQUE constraint failed")
	// The read after the collision has to succeed, so only the write fails.
	f.versions.masters["gw-1"][0].Seq = 1
	store := &collidingMasterStore{fakeVersionStore: f.versions}
	f.svc = newVersionService(&fakeStore{gateways: []Gateway{{ID: "gw-1", BaseURL: "https://dp.test",
		Key: "the-key"}}}, store, f.exporter, f.client, testSecretFields, f.names)
	f.versions.masterErr = nil

	result, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})

	require.Nil(t, svcErr)
	assert.Equal(t, other.Hash, result.Master)
}

// collidingMasterStore refuses every master written, as a number taken by another write does.
type collidingMasterStore struct {
	*fakeVersionStore
}

func (c *collidingMasterStore) AddMaster(context.Context, Master) (*Master, error) {
	return nil, errors.New("UNIQUE constraint failed")
}

// Forgetting a gateway drops its masters with the rest.
func TestForgetDropsTheGatewaysMasters(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a")
	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.Nil(t, svcErr)
	require.NotEmpty(t, f.versions.masters["gw-1"])

	f.svc.Forget(context.Background(), "gw-1")

	assert.Empty(t, f.versions.masters["gw-1"])
}

// The gateway's own answer is passed to the master as it came, so an answer with no results marks
// every sent resource as not known to be written.
func TestAnAnswerWithoutResultsLeavesEveryResourceFailed(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a")
	f.client.imported = json.RawMessage(`{"summary":{"failed":0}}`)

	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})

	require.Nil(t, svcErr)
	master, _ := f.svc.GetMaster(context.Background(), "gw-1")
	assert.Equal(t, MasterSummary{Failed: 1}, master.Summary)
}
