// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
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
		Status: outcomeFailed, Code: code, Message: "refused", Description: "refused for " + name}
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

type MasterTestSuite struct {
	suite.Suite
}

func TestMasterTestSuite(t *testing.T) {
	suite.Run(t, new(MasterTestSuite))
}

// A master built from a version applied whole records every resource deployed from it, except those
// the gateway left alone, and names itself by what it holds.
func (s *MasterTestSuite) TestAMasterOfAVersionDeploysEverythingButWhatWasLeftAlone() {
	version := &Version{Hash: "v1", Resources: joinBundle(unitsOf("ou-a", "ou-b", "ou-c"))}

	master := masterOfVersion("gw-1", version, []excludedResource{{Key: "organization_unit/ou-b"}}, masterAt,
		&Actor{ID: "u-1"})

	s.Require().Len(master.Entries, 2)
	s.Equal("organization_unit/ou-a", master.Entries[0].Key)
	s.Equal("organization_unit/ou-c", master.Entries[1].Key)
	for _, entry := range master.Entries {
		s.Equal(EntryDeployed, entry.State)
		s.Equal("v1", entry.SourceVersion)
		s.Equal(contentHash(unit(entry.ID)), entry.ContentHash)
		s.Equal(masterAt, entry.At)
		s.Equal(&Actor{ID: "u-1"}, entry.By)
	}
	s.Equal(joinBundle(unitsOf("ou-a", "ou-c")), master.Resources)
	s.Equal(MasterSummary{Deployed: 2}, master.Summary)
	s.Len(master.Hash, 64)
	s.Equal(master.Hash, masterOfVersion("gw-2", version, []excludedResource{{Key: "organization_unit/ou-b"}},
		masterAt.Add(time.Hour), &Actor{ID: "u-2"}).Hash, "the same resources from the same source are the same master")
}

// An apply's outcomes update the master one resource at a time: what the gateway accepted replaces
// its document, what it refused keeps the document it last accepted with the failure, what it removed
// loses its document, and what was not sent is carried over as it was.
func (s *MasterTestSuite) TestTheNextMasterFollowsEachOutcome() {
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
	s.Require().Len(byKey, 4)

	untouched := byKey["organization_unit/ou-a"]
	s.Equal(EntryDeployed, untouched.State)
	s.Equal("v1", untouched.SourceVersion, "a resource not sent was rewritten")
	s.Equal(masterAt, untouched.At)

	updated := byKey["organization_unit/ou-b"]
	s.Equal(EntryDeployed, updated.State)
	s.Equal("v2", updated.SourceVersion)
	s.Equal("dev", updated.Origin)
	s.Equal(contentHash(changedB), updated.ContentHash)
	s.Equal(&Actor{ID: "u-2"}, updated.By)

	failed := byKey["organization_unit/ou-d"]
	s.Equal(EntryFailed, failed.State)
	s.Equal("v2", failed.SourceVersion)
	s.Equal(&EntryFailure{Code: "OU-1", Message: "refused", Description: "refused for ou-d"},
		failed.Failure)
	s.Empty(failed.ContentHash, "a resource the gateway never accepted has no document")

	gone := byKey["organization_unit/ou-c"]
	s.Equal(EntryRemoved, gone.State)
	s.Empty(gone.ContentHash)
	s.Equal("ou-c", gone.Name, "the removed resource lost its name")

	s.Equal(joinBundle([]string{unit("ou-a"), changedB}), next.Resources,
		"the file holds what the gateway accepted, in key order")
	s.Equal(MasterSummary{Deployed: 2, Failed: 1, Removed: 1}, next.Summary)
	s.NotEqual(previous.Hash, next.Hash)
}

// A resource the gateway refused on an update keeps the document it accepted before, so the master
// still says what the gateway runs, and the failure says what it refused.
func (s *MasterTestSuite) TestARefusedUpdateKeepsTheDocumentTheGatewayRuns() {
	previous := deployedMaster("v1", "ou-a")
	changed := "resource_type: organization_unit\nid: ou-a\nname: ou-a\ndescription: changed"

	next := nextMaster(previous, masterWrite{sent: parseBundle(changed),
		answer: answerOf(refused("ou-a", "ou-a", "OU-2")), source: "v2", at: masterAt})

	s.Require().Len(next.Entries, 1)
	s.Equal(EntryFailed, next.Entries[0].State)
	s.Equal(contentHash(unit("ou-a")), next.Entries[0].ContentHash)
	s.Equal(unit("ou-a"), next.Resources)

	// A refused removal likewise leaves the resource on the gateway.
	refusedRemoval := importOutcome{ResourceType: "organization_unit", ResourceID: "ou-a", Operation: operationDeleted,
		Status: outcomeFailed, Code: "OU-3"}
	next = nextMaster(previous, masterWrite{
		deletions: []gatewayDeletion{{ResourceType: "organization_unit", ID: "ou-a"}},
		answer:    answerOf(refusedRemoval), source: "v2", at: masterAt})
	s.Require().Len(next.Entries, 1)
	s.Equal(EntryFailed, next.Entries[0].State)
	s.Equal("OU-3", next.Entries[0].Failure.Code)
	s.Equal(unit("ou-a"), next.Resources)
}

// A resource sent again as the gateway already holds it keeps its entry, so an apply that sends a
// version whole does not rewrite where every resource came from; one the gateway refuses this time is
// recorded failed all the same.
func (s *MasterTestSuite) TestAResourceHeldAsItIsKeepsItsEntry() {
	previous := deployedMaster("v1", "ou-a", "ou-b")

	next := nextMaster(previous, masterWrite{sent: parseBundle(joinBundle(unitsOf("ou-a", "ou-b"))),
		answer: answerOf(wrote("ou-a"), refused("ou-b", "ou-b", "OU-4")), source: "v2", at: masterAt.Add(time.Hour)})

	kept, _ := entryOf(next, "organization_unit/ou-a")
	s.Equal("v1", kept.SourceVersion)
	s.Equal(masterAt, kept.At)
	nowFailed, _ := entryOf(next, "organization_unit/ou-b")
	s.Equal(EntryFailed, nowFailed.State)
	s.Equal("v2", nowFailed.SourceVersion)
	s.Equal(contentHash(unit("ou-b")), nowFailed.ContentHash, "the gateway still runs what it accepted before")
	s.Equal(previous.Resources, next.Resources)
}

// A resource deployed again under a removed key replaces the removal.
func (s *MasterTestSuite) TestDeployingUnderARemovedKeyReplacesTheRemoval() {
	previous := nextMaster(deployedMaster("v1", "ou-a"), masterWrite{
		deletions: []gatewayDeletion{{ResourceType: "organization_unit", ID: "ou-a"}},
		answer:    answerOf(removed("ou-a")), source: "v2", at: masterAt})
	s.Require().Equal(EntryRemoved, previous.Entries[0].State)
	s.Require().Empty(previous.Resources)

	next := nextMaster(previous, masterWrite{sent: parseBundle(unit("ou-a")),
		answer: answerOf(wrote("ou-a")), source: "v3", at: masterAt})

	s.Require().Len(next.Entries, 1)
	s.Equal(EntryDeployed, next.Entries[0].State)
	s.Equal("v3", next.Entries[0].SourceVersion)
	s.Equal(unit("ou-a"), next.Resources)
}

// What the gateway's answer does not name is taken as written when the gateway refused nothing it did
// not name, and as not known, so failed, when it did or when the answer carries no results at all.
func (s *MasterTestSuite) TestAnUnansweredResourceIsDeployedUnlessSomethingUnnamedFailed() {
	translation := "resource_type: translation\nlanguage: fr\nmessages:\n  hello: bonjour"
	sent := parseBundle(joinBundle([]string{unit("ou-a"), translation}))

	clean := nextMaster(nil, masterWrite{sent: sent, answer: answerOf(wrote("ou-a")), source: "v1", at: masterAt})
	entry, ok := entryOf(clean, "translation/fr")
	s.Require().True(ok)
	s.Equal(EntryDeployed, entry.State, "nothing failed, so the unnamed translation was written")

	unnamedFailure := importOutcome{ResourceType: "translation", Status: outcomeFailed, Code: "T-1", Message: "bad"}
	unsure := nextMaster(nil, masterWrite{sent: sent, answer: answerOf(wrote("ou-a"), unnamedFailure), source: "v1",
		at: masterAt})
	entry, _ = entryOf(unsure, "translation/fr")
	s.Equal(EntryFailed, entry.State)
	s.Equal(unansweredFailure, entry.Failure.Message)
	entry, _ = entryOf(unsure, "organization_unit/ou-a")
	s.Equal(EntryDeployed, entry.State, "a resource the answer named as written is deployed all the same")

	silent := nextMaster(nil, masterWrite{sent: sent, answer: json.RawMessage(`{"summary":{}}`), source: "v1",
		at: masterAt})
	for _, entry := range silent.Entries {
		s.Equal(EntryFailed, entry.State, "%s was taken as written on an answer without results", entry.Key)
	}
}

// An outcome naming one resource's id does not answer for another resource that shares its name.
func (s *MasterTestSuite) TestAnOutcomeWithAnIDDoesNotAnswerForAnotherResourceOfTheSameName() {
	sent := parseBundle(joinBundle([]string{
		"resource_type: application\nid: app-1\nname: Shop",
		"resource_type: application\nid: app-2\nname: Shop",
	}))
	refused := importOutcome{ResourceType: "application", ResourceID: "app-2", ResourceName: "Shop",
		Operation: "update", Status: outcomeFailed, Code: "APP-1", Message: "bad"}

	next := nextMaster(nil, masterWrite{sent: sent, answer: answerOf(refused), source: "v1", at: masterAt})

	first, ok := entryOf(next, "application/app-1")
	s.Require().True(ok)
	s.Equal(EntryDeployed, first.State, "app-1 took the outcome that answers for app-2")
	second, _ := entryOf(next, "application/app-2")
	s.Equal(EntryFailed, second.State)
	s.Equal("APP-1", second.Failure.Code)
}

// A translation is named by its language, which the gateway answers as the resource's name.
func (s *MasterTestSuite) TestATranslationIsMatchedByItsLanguage() {
	translation := "resource_type: translation\nlanguage: fr\nmessages:\n  hello: bonjour"

	next := nextMaster(nil, masterWrite{sent: parseBundle(translation),
		answer: answerOf(importOutcome{ResourceType: "translation", ResourceName: "fr", Operation: "update",
			Status: outcomeFailed, Code: "T-2"}), source: "v1", at: masterAt})

	s.Require().Len(next.Entries, 1)
	s.Equal("translation/fr", next.Entries[0].Key)
	s.Equal(EntryFailed, next.Entries[0].State)
	s.Equal("T-2", next.Entries[0].Failure.Code)
}

// A manifest stores each actor's id alone, and reads back as it was written.
func (s *MasterTestSuite) TestAManifestStoresIdsNotNames() {
	master := deployedMaster("v1", "ou-a")
	master.Entries[0].By.Name = "alice"

	manifest, err := manifestOf(master.Entries)
	s.Require().NoError(err)
	s.NotContains(manifest, "alice")

	entries, err := entriesOf(manifest)
	s.Require().NoError(err)
	s.Require().Len(entries, 1)
	s.Equal(&Actor{ID: "u-1"}, entries[0].By)
	s.Equal(master.Entries[0].ContentHash, entries[0].ContentHash)

	empty, err := entriesOf("")
	s.Require().NoError(err)
	s.Empty(empty)
	_, err = entriesOf("{")
	s.Error(err)
}

// The master of a gateway nothing was applied to carries no time it was written.
// A version the gateway holds none of still gives how many it holds, as 0 of its total.
func (s *MasterTestSuite) TestAPartlyAppliedVersionAlwaysGivesItsCounts() {
	raw, err := json.Marshal(AppliedVersion{GatewayID: "gw-1", Applied: "aaa", ResourcesTotal: 3})
	s.Require().NoError(err)
	s.JSONEq(`{"gatewayId":"gw-1","appliedVersion":"aaa","complete":false,"resourcesHeld":0,`+
		`"resourcesTotal":3}`, string(raw))
}

func (s *MasterTestSuite) TestTheEmptyMasterHasNoCreationTime() {
	raw, err := json.Marshal(emptyMaster("gw-1"))
	s.Require().NoError(err)
	s.NotContains(string(raw), "createdAt")
	raw, err = json.Marshal(MasterEntry{Key: "organization_unit/ou-a", State: EntryDeployed})
	s.Require().NoError(err)
	s.NotContains(string(raw), `"at"`)
}
