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

// A role whose update the gateway refused keeps that refusal, whatever became of its member removal, and a
// removal the answer does not name is failed when the gateway refused something it did not name.
func (s *MasterTestSuite) TestAMemberRemovalFailsItsRoleUnlessTheRoleAlreadyFailed() {
	previous := masterOfVersion("gw-1", &Version{Hash: "v1", Resources: roleWith("user-1", "user-2")}, nil,
		masterAt, nil)
	removal := []gatewayDeletion{{ResourceType: "role_assignment", ID: "role-1",
		Members: []gatewayMember{{ID: "user-1", Type: "user", OUID: "ou-1"}}}}
	updateRefused := importOutcome{ResourceType: "role", ResourceID: "role-1", Operation: "update",
		Status: outcomeFailed, Code: "ROL-1001"}
	removalRefused := importOutcome{ResourceType: "role_assignment", ResourceID: "role-1",
		Operation: operationDeleted, Status: outcomeFailed, Code: "ROL-1040"}
	unnamed := importOutcome{ResourceType: "translation", Status: outcomeFailed, Code: "T-1"}

	cases := []struct {
		name   string
		answer json.RawMessage
		code   string
	}{
		{"the update and the removal refused", answerOf(updateRefused, removalRefused), "ROL-1001"},
		{"the update refused and the removal unnamed", answerOf(updateRefused, unnamed), "ROL-1001"},
		{"the update unnamed and the removal unnamed", answerOf(unnamed), ""},
	}
	for _, tc := range cases {
		next := nextMaster(previous, masterWrite{sent: parseBundle(roleWith("user-2")), deletions: removal,
			answer: tc.answer, source: "v2", at: masterAt})

		entry, ok := entryOf(next, "role/role-1")
		s.Require().True(ok, tc.name)
		s.Equal(EntryFailed, entry.State, tc.name)
		s.Equal(tc.code, entry.Failure.Code, tc.name)
		s.Equal(contentHash(roleWith("user-1", "user-2")), entry.ContentHash, tc.name)
		s.Len(next.Entries, 1, "%s: the removal was recorded as a resource of its own", tc.name)
	}
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

// After a forward apply, every resource the gateway holds deployed as the applied version has it names
// that version and its origin, sent or not, so an older version stops being named and can be pruned. A
// resource the version holds otherwise, or not at all, keeps its source, and nothing that was not
// written gets a new time or author. A refused resource is recorded as before.
func (s *MasterTestSuite) TestAForwardApplyNamesTheVersionForWhatItHoldsAsIs() {
	previous := deployedMaster("v1", "ou-a", "ou-b", "ou-c", "ou-e")
	newD := unit("ou-d")
	refusedE := "resource_type: organization_unit\nid: ou-e\nname: ou-e\ndescription: refused"
	version := &Version{Hash: "v2", Resources: joinBundle([]string{unit("ou-a"),
		"resource_type: organization_unit\nid: ou-c\nname: ou-c\ndescription: newer", newD, refusedE})}

	next := nextMaster(previous, masterWrite{
		sent:   parseBundle(joinBundle([]string{newD, refusedE})),
		answer: answerOf(wrote("ou-d"), refused("ou-e", "ou-e", "OU-1")),
		source: "v2", origin: "dev", by: &Actor{ID: "u-2"}, at: masterAt.Add(time.Hour),
		sourceContent: contentHashes(version),
	})

	byKey := map[string]MasterEntry{}
	for _, entry := range next.Entries {
		byKey[entry.Key] = entry
	}
	unchanged := byKey["organization_unit/ou-a"]
	s.Equal("v2", unchanged.SourceVersion, "a resource the version holds as it is still names the older version")
	s.Equal("dev", unchanged.Origin)
	s.Equal(masterAt, unchanged.At, "a resource that was not written got a new time")
	s.Equal(&Actor{ID: "u-1"}, unchanged.By, "a resource that was not written got a new author")

	s.Equal("v1", byKey["organization_unit/ou-b"].SourceVersion, "a resource the version does not hold was renamed")
	s.Equal("v1", byKey["organization_unit/ou-c"].SourceVersion, "a resource the version holds otherwise was renamed")
	s.Equal("v2", byKey["organization_unit/ou-d"].SourceVersion)
	s.Equal(EntryFailed, byKey["organization_unit/ou-e"].State)
}

// A revert names no version for what it carries over: a resource it leaves as the master it returns to
// records it takes that master's source, not the version the revert sends.
func (s *MasterTestSuite) TestARevertKeepsTheSourcesItCarriesOver() {
	previous := deployedMaster("v1", "ou-a", "ou-b")
	newB := "resource_type: organization_unit\nid: ou-b\nname: ou-b\ndescription: back"

	next := nextMaster(previous, masterWrite{
		sent:   parseBundle(joinBundle([]string{newB})),
		answer: answerOf(wrote("ou-b")),
		source: "v0", by: &Actor{ID: "u-2"}, at: masterAt.Add(time.Hour),
		mode: revertWrite, earlier: deployedMaster("v0", "ou-a"),
	})

	s.Equal("v0", next.Entries[0].SourceVersion, "a carried-over resource did not take the earlier master's source")
}

// A resource a version removed stays recorded removed until something is deployed under its key again.
// A later forward apply whose version does not hold it either names that version for the removal, so the
// version that removed it is not kept for it forever; one whose version holds it keeps the removal's
// source, since the gateway does not run the resource as the version has it.
func (s *MasterTestSuite) TestALaterApplyNamesItsVersionForARemovalItAgreesWith() {
	previous := nextMaster(deployedMaster("v1", "ou-a", "ou-b", "ou-c"), masterWrite{
		deletions: []gatewayDeletion{{ResourceType: "organization_unit", ID: "ou-b"},
			{ResourceType: "organization_unit", ID: "ou-c"}},
		answer: answerOf(removed("ou-b"), removed("ou-c")),
		source: "v2", by: &Actor{ID: "u-2"}, at: masterAt.Add(time.Hour),
	})
	version := &Version{Hash: "v3", Resources: joinBundle(unitsOf("ou-a", "ou-c"))}

	next := nextMaster(previous, masterWrite{
		answer: answerOf(), source: "v3", origin: "dev", by: &Actor{ID: "u-3"}, at: masterAt.Add(2 * time.Hour),
		sourceContent: contentHashes(version),
	})

	byKey := map[string]MasterEntry{}
	for _, entry := range next.Entries {
		byKey[entry.Key] = entry
	}
	gone := byKey["organization_unit/ou-b"]
	s.Equal(EntryRemoved, gone.State)
	s.Equal("v3", gone.SourceVersion, "a removal the applied version agrees with still names the version that made it")
	s.Equal(masterAt.Add(time.Hour), gone.At, "a removal that did not change got a new time")
	s.Equal("v2", byKey["organization_unit/ou-c"].SourceVersion,
		"a removal of a resource the applied version holds was renamed")
	s.Equal("v3", byKey["organization_unit/ou-a"].SourceVersion)
}

// A refused create the applied version no longer holds is dropped: the gateway holds nothing for it and
// nothing will offer it again. A refusal of what the gateway holds, or of what the version still holds,
// stays as it was, so it is offered again.
func (s *MasterTestSuite) TestALaterApplyDropsARefusedCreateItsVersionNoLongerHolds() {
	changedA := "resource_type: organization_unit\nid: ou-a\nname: ou-a\ndescription: changed"
	changedB := "resource_type: organization_unit\nid: ou-b\nname: ou-b\ndescription: changed"
	newC := unit("ou-c")
	previous := nextMaster(deployedMaster("v1", "ou-a", "ou-b"), masterWrite{
		sent: parseBundle(joinBundle([]string{changedA, changedB, newC})),
		answer: answerOf(refused("ou-a", "ou-a", "OU-1"), refused("ou-b", "ou-b", "OU-1"),
			refused("ou-c", "ou-c", "OU-1")),
		source: "v2", by: &Actor{ID: "u-2"}, at: masterAt.Add(time.Hour),
	})
	// v3 holds ou-a as the gateway kept it, still changes ou-b, and no longer holds ou-c.
	version := &Version{Hash: "v3", Resources: joinBundle([]string{unit("ou-a"), changedB})}

	next := nextMaster(previous, masterWrite{
		answer: answerOf(), source: "v3", origin: "dev", by: &Actor{ID: "u-3"}, at: masterAt.Add(2 * time.Hour),
		sourceContent: contentHashes(version),
	})

	byKey := map[string]MasterEntry{}
	for _, entry := range next.Entries {
		byKey[entry.Key] = entry
	}
	s.NotContains(byKey, "organization_unit/ou-c", "a refused create the version no longer holds was kept")
	for _, key := range []string{"organization_unit/ou-a", "organization_unit/ou-b"} {
		s.Equal(EntryFailed, byKey[key].State, "%s: a refusal of what the gateway holds was forgotten", key)
		s.Equal("v2", byKey[key].SourceVersion, "%s: a refusal was renamed", key)
	}
}

// A revert gives a resource the gateway already held as the master it returns to records it that
// master's source and origin, though a forward apply since named a later version for it, and keeps when
// and by whom its entry was last written.
func (s *MasterTestSuite) TestARevertRestoresTheSourceOfWhatItLeavesAsItWas() {
	earlier := deployedMaster("v1", "ou-a", "ou-b")
	forward := nextMaster(earlier, masterWrite{
		sent:   parseBundle(unit("ou-c")),
		answer: answerOf(wrote("ou-c")),
		source: "v2", origin: "dev", by: &Actor{ID: "u-2"}, at: masterAt.Add(time.Hour),
		sourceContent: contentHashes(&Version{Hash: "v2", Resources: joinBundle(unitsOf("ou-a", "ou-b", "ou-c"))}),
	})
	s.Require().Equal("v2", forward.Entries[0].SourceVersion)

	reverted := nextMaster(forward, masterWrite{
		deletions: []gatewayDeletion{{ResourceType: "organization_unit", ID: "ou-c"}},
		answer:    answerOf(removed("ou-c")),
		source:    "v1", by: &Actor{ID: "u-3"}, at: masterAt.Add(2 * time.Hour),
		mode: revertWrite, earlier: earlier,
	})

	byKey := map[string]MasterEntry{}
	for _, entry := range reverted.Entries {
		byKey[entry.Key] = entry
	}
	unchanged := byKey["organization_unit/ou-a"]
	s.Equal("v1", unchanged.SourceVersion, "a resource the revert left as it was kept the later version")
	s.Empty(unchanged.Origin)
	s.Equal(masterAt, unchanged.At, "a resource the revert did not write got a new time")
	s.Equal(&Actor{ID: "u-1"}, unchanged.By)
	s.Equal(EntryRemoved, byKey["organization_unit/ou-c"].State)
}

// A project's rollback restores the source of only what it writes: a resource outside the project keeps
// the source the gateway holds it with now, though the master the rollback returns to records another.
func (s *MasterTestSuite) TestAProjectRollbackRestoresOnlyWhatItWrites() {
	earlier := deployedMaster("v1", "ou-a", "ou-b")
	forward := nextMaster(earlier, masterWrite{
		sent:   parseBundle(joinBundle(unitsOf("ou-a", "ou-b"))),
		answer: answerOf(wrote("ou-a"), wrote("ou-b")),
		source: "v2", origin: "dev", by: &Actor{ID: "u-2"}, at: masterAt.Add(time.Hour),
		sourceContent: contentHashes(&Version{Hash: "v2", Resources: joinBundle(unitsOf("ou-a", "ou-b"))}),
	})

	reverted := nextMaster(forward, masterWrite{
		mode: revertWrite, projectRevert: true, project: "p-1",
		sent:   parseBundle(unit("ou-a")),
		answer: answerOf(wrote("ou-a")),
		source: "v1", by: &Actor{ID: "u-3"}, at: masterAt.Add(2 * time.Hour),
		earlier: earlier,
	})

	byKey := map[string]MasterEntry{}
	for _, entry := range reverted.Entries {
		byKey[entry.Key] = entry
	}
	s.Equal("v1", byKey["organization_unit/ou-a"].SourceVersion, "what the rollback wrote kept the later version")
	s.Equal("v2", byKey["organization_unit/ou-b"].SourceVersion, "a resource outside the rollback took its source")
	s.Equal("dev", byKey["organization_unit/ou-b"].Origin)
}

// A deploy scoped to a project names its version only for that project's resources and those of no
// project: another project's resource keeps the version its own deploy named.
func (s *MasterTestSuite) TestAProjectDeployLeavesAnotherProjectsSourceAsItWas() {
	previous := deployedMaster("v1", "ou-a", "ou-b", "ou-c")
	version := &Version{Hash: "v2", Resources: joinBundle(unitsOf("ou-a", "ou-b", "ou-c"))}

	next := nextMaster(previous, masterWrite{
		answer: answerOf(), source: "v2", origin: "dev", by: &Actor{ID: "u-2"}, at: masterAt.Add(time.Hour),
		sourceContent: contentHashes(version), scope: "shop",
		owners: map[string]string{"organization_unit/ou-a": "shop", "organization_unit/ou-b": "billing"},
	})

	byKey := map[string]MasterEntry{}
	for _, entry := range next.Entries {
		byKey[entry.Key] = entry
	}
	s.Equal("v2", byKey["organization_unit/ou-a"].SourceVersion, "the project's own resource kept the older version")
	s.Equal("v1", byKey["organization_unit/ou-b"].SourceVersion, "another project's resource was renamed")
	s.Equal("v2", byKey["organization_unit/ou-c"].SourceVersion, "a resource of no project kept the older version")
}

// A removal keeps no document to say which project it was, and its source only keeps the version it
// names from being pruned. A later deploy of another project whose version also lacks the resource
// names its version for the removal, leaving who removed it, and when, as they were.
func (s *MasterTestSuite) TestAnotherProjectsDeployTakesOverARemovalItsVersionAgreesWith() {
	previous := deployedMaster("v1", "ou-a", "ou-b")
	billing := &Version{Hash: "v2", Resources: joinBundle(unitsOf("ou-a"))}
	removedAt := masterAt.Add(time.Hour)
	afterRemoval := nextMaster(previous, masterWrite{
		deletions: []gatewayDeletion{{ResourceType: "organization_unit", ID: "ou-b"}},
		answer:    answerOf(removed("ou-b")), source: "v2", by: &Actor{ID: "u-2"}, at: removedAt,
		sourceContent: contentHashes(billing), scope: "billing",
		owners: map[string]string{"organization_unit/ou-a": "shop", "organization_unit/ou-b": "billing"},
	})

	shop := &Version{Hash: "v3", Resources: joinBundle(unitsOf("ou-a"))}
	next := nextMaster(afterRemoval, masterWrite{
		answer: answerOf(), source: "v3", by: &Actor{ID: "u-3"}, at: masterAt.Add(2 * time.Hour),
		sourceContent: contentHashes(shop), scope: "shop",
		owners: ownersOf(parseBundle(afterRemoval.Resources), parseBundle(shop.Resources)),
	})

	byKey := map[string]MasterEntry{}
	for _, entry := range next.Entries {
		byKey[entry.Key] = entry
	}
	removal := byKey["organization_unit/ou-b"]
	s.Equal(EntryRemoved, removal.State)
	s.Equal("v3", removal.SourceVersion, "the removal kept naming a version no deploy may need again")
	s.Equal(removedAt, removal.At, "the removal was taken to be written again")
	s.Equal(&Actor{ID: "u-2"}, removal.By)
}

// A deploy of a selection that is not scoped to a project names its version for what belongs to no
// project, and leaves every project's resource with the version its own deploy named.
func (s *MasterTestSuite) TestASelectiveDeployLeavesEveryProjectsSourceAsItWas() {
	previous := deployedMaster("v1", "ou-a", "ou-b")
	version := &Version{Hash: "v2", Resources: joinBundle(unitsOf("ou-a", "ou-b"))}

	next := nextMaster(previous, masterWrite{
		answer: answerOf(), source: "v2", origin: "dev", by: &Actor{ID: "u-2"}, at: masterAt.Add(time.Hour),
		sourceContent: contentHashes(version), owners: map[string]string{"organization_unit/ou-a": "shop"},
	})

	byKey := map[string]MasterEntry{}
	for _, entry := range next.Entries {
		byKey[entry.Key] = entry
	}
	s.Equal("v1", byKey["organization_unit/ou-a"].SourceVersion, "a project's resource took the selection's version")
	s.Equal("v2", byKey["organization_unit/ou-b"].SourceVersion, "a resource of no project kept the older version")
}

// A forward apply that sends nothing still names its version for what the gateway holds as it has it.
func (s *MasterTestSuite) TestAForwardApplyThatSendsNothingStillRepoints() {
	previous := deployedMaster("v1", "ou-a")
	version := &Version{Hash: "v2", Resources: joinBundle(unitsOf("ou-a"))}

	next := nextMaster(previous, masterWrite{
		answer: answerOf(), source: "v2", origin: "dev", by: &Actor{ID: "u-2"}, at: masterAt.Add(time.Hour),
		sourceContent: contentHashes(version),
	})

	s.Equal("v2", next.Entries[0].SourceVersion)
	s.Equal(masterAt, next.Entries[0].At, "a resource that was not written got a new time")
}

// Applying the version a resource already names again, with another origin, records that origin for
// what the gateway holds as the version has it.
func (s *MasterTestSuite) TestAnApplyOfTheSameVersionFromElsewhereRecordsItsOrigin() {
	version := &Version{Hash: "v2", Resources: joinBundle(unitsOf("ou-a", "ou-b"))}
	first := nextMaster(deployedMaster("v1", "ou-a"), masterWrite{
		sent: parseBundle(unit("ou-b")), answer: answerOf(wrote("ou-b")),
		source: "v2", origin: "dev", by: &Actor{ID: "u-2"}, at: masterAt.Add(time.Hour),
		sourceContent: contentHashes(version),
	})

	next := nextMaster(first, masterWrite{
		answer: answerOf(), source: "v2", origin: "staging", by: &Actor{ID: "u-3"}, at: masterAt.Add(2 * time.Hour),
		sourceContent: contentHashes(version),
	})

	for _, entry := range next.Entries {
		s.Equal("v2", entry.SourceVersion)
		s.Equal("staging", entry.Origin, "%s kept the earlier apply's origin", entry.Key)
	}
}
