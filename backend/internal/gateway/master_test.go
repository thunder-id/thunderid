// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var masterAt = time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC)

// answerOf is a gateway's account of an import, one line per outcome.
func answerOf(outcomes ...importOutcome) json.RawMessage {
	answer, err := json.Marshal(map[string]interface{}{"results": outcomes})
	if err != nil {
		panic(err)
	}
	return answer
}

// wrote, refused and removed are a gateway's outcomes for a unit it wrote, refused and removed.
func wrote(id string) importOutcome {
	return importOutcome{ResourceType: "organization_unit", ResourceID: id, ResourceName: id, Operation: "update",
		Status: outcomeSuccess}
}

func refused(id, name, code string) importOutcome {
	return importOutcome{ResourceType: "organization_unit", ResourceID: id, ResourceName: name, Operation: "create",
		Status: outcomeFailed, Code: code, Message: "refused"}
}

func removed(id string) importOutcome {
	return importOutcome{ResourceType: "organization_unit", ResourceID: id, Operation: operationDeleted,
		Status: outcomeSuccess}
}

// deployedMaster is a gateway's master holding the given units, every one deployed from source.
func deployedMaster(source string, ids ...string) *Master {
	version := &Version{Hash: source, Resources: joinBundle(unitsOf(ids...))}
	return masterOfVersion("gw-1", version, nil, masterAt, &Actor{ID: "u-1"})
}

func unitsOf(ids ...string) []string {
	documents := make([]string, 0, len(ids))
	for _, id := range ids {
		documents = append(documents, unit(id))
	}
	return documents
}

// A master built from a version applied whole records every resource deployed from it, except those
// the gateway left alone, and names itself by what it holds.
func TestAMasterOfAVersionDeploysEverythingButWhatWasLeftAlone(t *testing.T) {
	version := &Version{Hash: "v1", Resources: joinBundle(unitsOf("ou-a", "ou-b", "ou-c"))}

	master := masterOfVersion("gw-1", version, []excludedResource{{Key: "organization_unit/ou-b"}}, masterAt,
		&Actor{ID: "u-1"})

	require.Len(t, master.Entries, 2)
	assert.Equal(t, "organization_unit/ou-a", master.Entries[0].Key)
	assert.Equal(t, "organization_unit/ou-c", master.Entries[1].Key)
	for _, entry := range master.Entries {
		assert.Equal(t, EntryDeployed, entry.State)
		assert.Equal(t, "v1", entry.SourceVersion)
		assert.Equal(t, contentHash(unit(entry.ID)), entry.ContentHash)
		assert.Equal(t, masterAt, entry.At)
		assert.Equal(t, &Actor{ID: "u-1"}, entry.By)
	}
	assert.Equal(t, joinBundle(unitsOf("ou-a", "ou-c")), master.Resources)
	assert.Equal(t, MasterSummary{Deployed: 2}, master.Summary)
	assert.Len(t, master.Hash, 64)
	assert.Equal(t, master.Hash, masterOfVersion("gw-2", version, []excludedResource{{Key: "organization_unit/ou-b"}},
		masterAt.Add(time.Hour), &Actor{ID: "u-2"}).Hash, "the same resources from the same source are the same master")
}

// An apply's outcomes update the master one resource at a time: what the gateway accepted replaces
// its document, what it refused keeps the document it last accepted with the failure, what it removed
// loses its document, and what was not sent is carried over as it was.
func TestTheNextMasterFollowsEachOutcome(t *testing.T) {
	previous := deployedMaster("v1", "ou-a", "ou-b", "ou-c")
	changedB := "resource_type: organization_unit\nid: ou-b\nname: ou-b\ndescription: changed"
	newD := unit("ou-d")

	next := nextMaster(previous, masterWrite{
		sent:      parseBundle(joinBundle([]string{changedB, newD})),
		deletions: []gatewayDeletion{{ResourceType: "organization_unit", ID: "ou-c"}},
		answer:    answerOf(wrote("ou-b"), refused("", "ou-d", "OU-1"), removed("ou-c")),
		source:    "v2", origin: "dev", by: &Actor{ID: "u-2"}, at: masterAt.Add(time.Hour),
	})

	byKey := map[string]MasterEntry{}
	for _, entry := range next.Entries {
		byKey[entry.Key] = entry
	}
	require.Len(t, byKey, 4)

	untouched := byKey["organization_unit/ou-a"]
	assert.Equal(t, EntryDeployed, untouched.State)
	assert.Equal(t, "v1", untouched.SourceVersion, "a resource not sent was rewritten")
	assert.Equal(t, masterAt, untouched.At)

	updated := byKey["organization_unit/ou-b"]
	assert.Equal(t, EntryDeployed, updated.State)
	assert.Equal(t, "v2", updated.SourceVersion)
	assert.Equal(t, "dev", updated.Origin)
	assert.Equal(t, contentHash(changedB), updated.ContentHash)
	assert.Equal(t, &Actor{ID: "u-2"}, updated.By)

	failed := byKey["organization_unit/ou-d"]
	assert.Equal(t, EntryFailed, failed.State)
	assert.Equal(t, "v2", failed.SourceVersion)
	assert.Equal(t, &EntryFailure{Code: "OU-1", Message: "refused"}, failed.Failure)
	assert.Empty(t, failed.ContentHash, "a resource the gateway never accepted has no document")

	gone := byKey["organization_unit/ou-c"]
	assert.Equal(t, EntryRemoved, gone.State)
	assert.Empty(t, gone.ContentHash)
	assert.Equal(t, "ou-c", gone.Name, "the removed resource lost its name")

	assert.Equal(t, joinBundle([]string{unit("ou-a"), changedB}), next.Resources,
		"the file holds what the gateway accepted, in key order")
	assert.Equal(t, MasterSummary{Deployed: 2, Failed: 1, Removed: 1}, next.Summary)
	assert.NotEqual(t, previous.Hash, next.Hash)
}

// A resource the gateway refused on an update keeps the document it accepted before, so the master
// still says what the gateway runs, and the failure says what it refused.
func TestARefusedUpdateKeepsTheDocumentTheGatewayRuns(t *testing.T) {
	previous := deployedMaster("v1", "ou-a")
	changed := "resource_type: organization_unit\nid: ou-a\nname: ou-a\ndescription: changed"

	next := nextMaster(previous, masterWrite{sent: parseBundle(changed),
		answer: answerOf(refused("ou-a", "ou-a", "OU-2")), source: "v2", at: masterAt})

	require.Len(t, next.Entries, 1)
	assert.Equal(t, EntryFailed, next.Entries[0].State)
	assert.Equal(t, contentHash(unit("ou-a")), next.Entries[0].ContentHash)
	assert.Equal(t, unit("ou-a"), next.Resources)

	// A refused removal likewise leaves the resource on the gateway.
	refusedRemoval := importOutcome{ResourceType: "organization_unit", ResourceID: "ou-a", Operation: operationDeleted,
		Status: outcomeFailed, Code: "OU-3"}
	next = nextMaster(previous, masterWrite{
		deletions: []gatewayDeletion{{ResourceType: "organization_unit", ID: "ou-a"}},
		answer:    answerOf(refusedRemoval), source: "v2", at: masterAt})
	require.Len(t, next.Entries, 1)
	assert.Equal(t, EntryFailed, next.Entries[0].State)
	assert.Equal(t, "OU-3", next.Entries[0].Failure.Code)
	assert.Equal(t, unit("ou-a"), next.Resources)
}

// A resource sent again as the gateway already holds it keeps its entry, so an apply that sends a
// version whole does not rewrite where every resource came from; one the gateway refuses this time is
// recorded failed all the same.
func TestAResourceHeldAsItIsKeepsItsEntry(t *testing.T) {
	previous := deployedMaster("v1", "ou-a", "ou-b")

	next := nextMaster(previous, masterWrite{sent: parseBundle(joinBundle(unitsOf("ou-a", "ou-b"))),
		answer: answerOf(wrote("ou-a"), refused("ou-b", "ou-b", "OU-4")), source: "v2", at: masterAt.Add(time.Hour)})

	kept, _ := entryOf(next, "organization_unit/ou-a")
	assert.Equal(t, "v1", kept.SourceVersion)
	assert.Equal(t, masterAt, kept.At)
	nowFailed, _ := entryOf(next, "organization_unit/ou-b")
	assert.Equal(t, EntryFailed, nowFailed.State)
	assert.Equal(t, "v2", nowFailed.SourceVersion)
	assert.Equal(t, contentHash(unit("ou-b")), nowFailed.ContentHash, "the gateway still runs what it accepted before")
	assert.Equal(t, previous.Resources, next.Resources)
}

// A resource deployed again under a removed key replaces the removal.
func TestDeployingUnderARemovedKeyReplacesTheRemoval(t *testing.T) {
	previous := nextMaster(deployedMaster("v1", "ou-a"), masterWrite{
		deletions: []gatewayDeletion{{ResourceType: "organization_unit", ID: "ou-a"}},
		answer:    answerOf(removed("ou-a")), source: "v2", at: masterAt})
	require.Equal(t, EntryRemoved, previous.Entries[0].State)
	require.Empty(t, previous.Resources)

	next := nextMaster(previous, masterWrite{sent: parseBundle(unit("ou-a")),
		answer: answerOf(wrote("ou-a")), source: "v3", at: masterAt})

	require.Len(t, next.Entries, 1)
	assert.Equal(t, EntryDeployed, next.Entries[0].State)
	assert.Equal(t, "v3", next.Entries[0].SourceVersion)
	assert.Equal(t, unit("ou-a"), next.Resources)
}

// What the gateway's answer does not name is taken as written when the gateway refused nothing it did
// not name, and as not known, so failed, when it did or when the answer carries no results at all.
func TestAnUnansweredResourceIsDeployedUnlessSomethingUnnamedFailed(t *testing.T) {
	translation := "resource_type: translation\nlanguage: fr\nmessages:\n  hello: bonjour"
	sent := parseBundle(joinBundle([]string{unit("ou-a"), translation}))

	clean := nextMaster(nil, masterWrite{sent: sent, answer: answerOf(wrote("ou-a")), source: "v1", at: masterAt})
	entry, ok := entryOf(clean, "translation/fr")
	require.True(t, ok)
	assert.Equal(t, EntryDeployed, entry.State, "nothing failed, so the unnamed translation was written")

	unnamedFailure := importOutcome{ResourceType: "translation", Status: outcomeFailed, Code: "T-1", Message: "bad"}
	unsure := nextMaster(nil, masterWrite{sent: sent, answer: answerOf(wrote("ou-a"), unnamedFailure), source: "v1",
		at: masterAt})
	entry, _ = entryOf(unsure, "translation/fr")
	assert.Equal(t, EntryFailed, entry.State)
	assert.Equal(t, unansweredFailure, entry.Failure.Message)
	entry, _ = entryOf(unsure, "organization_unit/ou-a")
	assert.Equal(t, EntryDeployed, entry.State, "a resource the answer named as written is deployed all the same")

	silent := nextMaster(nil, masterWrite{sent: sent, answer: json.RawMessage(`{"summary":{}}`), source: "v1",
		at: masterAt})
	for _, entry := range silent.Entries {
		assert.Equal(t, EntryFailed, entry.State, "%s was taken as written on an answer without results", entry.Key)
	}
}

// A translation is named by its language, which the gateway answers as the resource's name.
func TestATranslationIsMatchedByItsLanguage(t *testing.T) {
	translation := "resource_type: translation\nlanguage: fr\nmessages:\n  hello: bonjour"

	next := nextMaster(nil, masterWrite{sent: parseBundle(translation),
		answer: answerOf(importOutcome{ResourceType: "translation", ResourceName: "fr", Operation: "update",
			Status: outcomeFailed, Code: "T-2"}), source: "v1", at: masterAt})

	require.Len(t, next.Entries, 1)
	assert.Equal(t, "translation/fr", next.Entries[0].Key)
	assert.Equal(t, EntryFailed, next.Entries[0].State)
	assert.Equal(t, "T-2", next.Entries[0].Failure.Code)
}

// A manifest stores each actor's id alone, and reads back as it was written.
func TestAManifestStoresIdsNotNames(t *testing.T) {
	master := deployedMaster("v1", "ou-a")
	master.Entries[0].By.Name = "alice"

	manifest, err := manifestOf(master.Entries)
	require.NoError(t, err)
	assert.NotContains(t, manifest, "alice")

	entries, err := entriesOf(manifest)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, &Actor{ID: "u-1"}, entries[0].By)
	assert.Equal(t, master.Entries[0].ContentHash, entries[0].ContentHash)

	empty, err := entriesOf("")
	require.NoError(t, err)
	assert.Empty(t, empty)
	_, err = entriesOf("{")
	assert.Error(t, err)
}
