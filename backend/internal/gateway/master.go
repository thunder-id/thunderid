// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"time"
)

// importOutcome is one line of a gateway's account of an import, as the import API answers it.
type importOutcome struct {
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId"`
	ResourceName string `json:"resourceName"`
	Operation    string `json:"operation"`
	Status       string `json:"status"`
	Code         string `json:"code"`
	Message      string `json:"message"`
	Description  string `json:"description"`
}

// How an import names its operations and outcomes.
const (
	outcomeSuccess   = "success"
	outcomeFailed    = "failed"
	operationDeleted = "delete"
)

// unansweredFailure is recorded for a resource the gateway's answer did not name while it refused
// something: whether this one was written is not known, so it is offered again.
const unansweredFailure = "the gateway's answer did not say whether this resource was written"

// outcomesOf reads the per-resource outcomes from a gateway's answer, and whether the answer carried
// them. An answer without its results says nothing of what the gateway wrote.
func outcomesOf(answer json.RawMessage) ([]importOutcome, bool) {
	var parsed struct {
		Results *[]importOutcome `json:"results"`
	}
	if err := json.Unmarshal(answer, &parsed); err != nil || parsed.Results == nil {
		return nil, false
	}
	return *parsed.Results, true
}

// writeMode says where a master write takes the sources it records from.
type writeMode int

const (
	// forwardWrite applies a version: what the gateway holds as the version has it comes from it.
	forwardWrite writeMode = iota
	// revertWrite returns the gateway to an earlier master: what it writes takes the sources that master
	// records.
	revertWrite
)

// writeModeOf is the mode an apply writes the master in: a revert, a project's rollback included, or a
// forward apply of a version.
func writeModeOf(options applyOptions) writeMode {
	if options.isRevert || options.revertTo != nil {
		return revertWrite
	}
	return forwardWrite
}

// masterWrite is one apply's effect on a gateway's master: what was sent to be written and removed,
// what the gateway answered, and where it all came from.
type masterWrite struct {
	mode      writeMode
	sent      []bundleResource
	deletions []gatewayDeletion
	answer    json.RawMessage
	source    string
	origin    string
	by        *Actor
	at        time.Time
	// earlier is the master a revert returns to, when it has one: what the revert writes takes the source
	// and origin that master records for it.
	earlier *Master
	// sourceContent names each resource of the version a forward write applies by its content hash, so a
	// resource the gateway holds as that version has it is recorded as coming from it.
	sourceContent map[string]string
	// scope is the project a forward apply was scoped to, if any. With owners set, a write names its
	// version only for scope's resources, if any, and for those of no project: another project's resource
	// keeps the version its own deploy named, so what a project shows as deployed is never moved by another
	// project's deploy or by a selection that deployed nothing of it.
	scope string
	// owners names the project each resource belongs to, by key, for a write limited as scope says. Nil for
	// a whole deploy, which names its version for everything the gateway holds as the version has it.
	owners map[string]string
	// project is the project the apply was scoped to, and projectRevert marks a rollback of it.
	project       string
	projectRevert bool
}

// nextMaster builds the master a gateway holds after an apply from the one it held before and the
// import's outcomes. For each resource sent, a success replaces or adds its document and records it
// deployed; a refusal keeps the document the gateway last accepted and records the failure. For each
// removal, a success drops the document and records it removed; a refusal keeps it with the failure.
// A role or group the gateway did not take every removed assignee or member off is recorded refused,
// with the document it held before, so the next diff offers it, and the removal, again.
// Every other document is carried over as it was, as is a sent resource the gateway already held as it
// is. A resource the answer does not name is deployed when the gateway refused nothing it did not name,
// and failed otherwise, since it is not known. After a forward apply, every resource the gateway holds
// as the applied version has it names that version as its source, sent or not (see repoint).
func nextMaster(previous *Master, write masterWrite) *Master {
	outcomes, answered := outcomesOf(write.answer)
	matcher := newOutcomeMatcher(outcomes)

	documents := map[string]string{}
	held := map[string]string{}
	entries := map[string]MasterEntry{}
	if previous != nil {
		for _, resource := range parseBundle(previous.Resources) {
			documents[resource.key()] = resource.Content
			held[resource.key()] = resource.Content
		}
		for _, entry := range previous.Entries {
			entries[entry.Key] = entry
		}
	}

	type pending struct {
		resource bundleResource
		removal  bool
	}
	unanswered := []pending{}
	record := func(resource bundleResource, outcome *importOutcome, removal bool) {
		key := resource.key()
		entry := MasterEntry{Key: key, ResourceType: resource.Type, ID: resource.ID, Name: resource.Name,
			SourceVersion: write.source, Origin: write.origin, At: write.at, By: write.by}
		switch {
		case outcome.Status != outcomeSuccess:
			entry.State = EntryFailed
			entry.Failure = &EntryFailure{Code: outcome.Code, Message: outcome.Message,
				Description: outcome.Description}
			if kept, ok := documents[key]; ok {
				entry.ContentHash = contentHash(kept)
			}
		case removal:
			entry.State = EntryRemoved
			delete(documents, key)
		default:
			entry.State = EntryDeployed
			entry.ContentHash = contentHash(resource.Content)
			documents[key] = resource.Content
		}
		entries[key] = entry
	}

	for _, resource := range write.sent {
		outcome := matcher.take(resource, false)
		// A resource the gateway already holds as it is keeps its entry, and so where it came from,
		// unless the gateway refused it this time.
		if heldAsIs(entries, resource) && (outcome == nil || outcome.Status == outcomeSuccess) {
			continue
		}
		if outcome == nil {
			unanswered = append(unanswered, pending{resource: resource})
			continue
		}
		record(resource, outcome, false)
	}
	type memberRemoval struct {
		key     string
		outcome *importOutcome
	}
	var memberRemovalsSent []memberRemoval
	for _, deletion := range write.deletions {
		resource := bundleResource{Type: deletion.ResourceType, ID: deletion.ID}
		if listType, ok := memberListOf(deletion.ResourceType); ok {
			memberRemovalsSent = append(memberRemovalsSent, memberRemoval{key: resourceKey(listType, deletion.ID, ""),
				outcome: matcher.take(resource, true)})
			continue
		}
		if entry, ok := entries[resource.key()]; ok {
			resource.Name = entry.Name
		}
		outcome := matcher.take(resource, true)
		if outcome == nil {
			unanswered = append(unanswered, pending{resource: resource, removal: true})
			continue
		}
		record(resource, outcome, true)
	}
	// What the answer did not name was written when the gateway refused nothing else it did not name.
	assumed := &importOutcome{Status: outcomeSuccess}
	if !answered || matcher.refusedUnnamed() {
		assumed = &importOutcome{Status: outcomeFailed, Message: unansweredFailure}
	}
	for _, item := range unanswered {
		record(item.resource, assumed, item.removal)
	}
	for _, removal := range memberRemovalsSent {
		outcome := removal.outcome
		if outcome == nil {
			outcome = assumed
		}
		entry, ok := entries[removal.key]
		if !ok || outcome.Status == outcomeSuccess || entry.State == EntryFailed {
			continue
		}
		entry.State = EntryFailed
		entry.Failure = &EntryFailure{Code: outcome.Code, Message: outcome.Message, Description: outcome.Description}
		entry.ContentHash = ""
		if kept, ok := held[removal.key]; ok {
			documents[removal.key] = kept
			entry.ContentHash = contentHash(kept)
		}
		entries[removal.key] = entry
	}
	repoint(entries, write)

	return assemble(previous, entries, documents, write)
}

// repoint settles where each resource the next master records came from, without touching when or by
// whom its entry was last written. A revert gives what it returns to the sources the master it returns
// to records (see restoreSources). A forward apply names its version, with its origin, for every
// resource the gateway holds deployed as that version has it, and for every removal the version agrees
// with by not holding the resource, leaving another project's resources to a deploy scoped to one.
// A removal keeps no document to say which project it was, and its source is read only to keep the
// version it names, so any later version that agrees with it may take it over; who removed it, and
// when, stay as they were.
// Otherwise a resource unchanged since an older version, or removed in
// one and never deployed again, goes on naming that version, and a version a kept master names is never
// pruned, so versions would pile up past gateway.max_versions.
func repoint(entries map[string]MasterEntry, write masterWrite) {
	if write.mode == revertWrite {
		restoreSources(entries, write)
		return
	}
	for key, entry := range entries {
		if entry.SourceVersion == write.source && entry.Origin == write.origin {
			continue
		}
		if owner := write.owners[key]; write.owners != nil && owner != "" && owner != write.scope {
			continue
		}
		hash, inVersion := write.sourceContent[key]
		switch {
		case entry.State == EntryDeployed && inVersion && hash == entry.ContentHash:
		case entry.State == EntryRemoved && !inVersion:
		default:
			continue
		}
		entry.SourceVersion, entry.Origin = write.source, write.origin
		entries[key] = entry
	}
}

// restoreSources gives what a revert returns to the source and origin the master it returns to records.
// A resource the revert writes or removes takes that master's whatever the gateway answered. A whole
// revert also restores a resource it carries over as that master records it, deployed or removed with
// the same document, since a forward apply since may have named a later version for it. A project's
// rollback leaves every resource it does not write as the gateway holds it now, source included.
func restoreSources(entries map[string]MasterEntry, write masterWrite) {
	if write.earlier == nil {
		return
	}
	recorded := make(map[string]MasterEntry, len(write.earlier.Entries))
	for _, entry := range write.earlier.Entries {
		recorded[entry.Key] = entry
	}
	written := make(map[string]bool, len(write.sent)+len(write.deletions))
	for _, resource := range write.sent {
		written[resource.key()] = true
	}
	for _, deletion := range write.deletions {
		written[bundleResource{Type: deletion.ResourceType, ID: deletion.ID}.key()] = true
	}
	for key, entry := range entries {
		was, ok := recorded[key]
		if !ok {
			continue
		}
		asRecorded := was.State == entry.State && was.ContentHash == entry.ContentHash &&
			(entry.State == EntryDeployed || entry.State == EntryRemoved)
		if !written[key] && (write.projectRevert || !asRecorded) {
			continue
		}
		entry.SourceVersion, entry.Origin = was.SourceVersion, was.Origin
		entries[key] = entry
	}
}

// contentHashes names each resource of a version by its content hash, keyed as a master keys it.
func contentHashes(version *Version) map[string]string {
	hashes := map[string]string{}
	for _, resource := range parseBundle(version.Resources) {
		hashes[resource.key()] = contentHash(resource.Content)
	}
	return hashes
}

// memberListOf returns the resource type whose assignees or members a removal type takes off.
func memberListOf(removalType string) (string, bool) {
	for resourceType, removal := range memberRemovalTypes {
		if removal == removalType {
			return resourceType, true
		}
	}
	return "", false
}

// heldAsIs reports whether a master records a resource deployed with the same content.
func heldAsIs(entries map[string]MasterEntry, resource bundleResource) bool {
	entry, ok := entries[resource.key()]
	return ok && entry.State == EntryDeployed && entry.ContentHash == contentHash(resource.Content)
}

// assemble writes the master's file and manifest in key order and names it by its hash.
func assemble(previous *Master, entries map[string]MasterEntry, documents map[string]string,
	write masterWrite) *Master {
	keys := slices.Sorted(maps.Keys(entries))
	master := &Master{CreatedAt: write.at, CreatedBy: write.by, Entries: make([]MasterEntry, 0, len(keys))}
	if previous != nil {
		master.GatewayID = previous.GatewayID
	}
	kept := make([]string, 0, len(documents))
	for _, key := range keys {
		entry := entries[key]
		master.Entries = append(master.Entries, entry)
		if document, ok := documents[key]; ok {
			kept = append(kept, document)
		}
	}
	master.Resources = joinBundle(kept)
	master.Summary = summarizeMaster(master.Entries)
	master.Hash = masterHash(master)
	return master
}

// masterOfVersion is the master a gateway that applied a version whole holds: every resource of the
// version deployed from it, except those left out.
func masterOfVersion(gatewayID string, version *Version, leftOut []excludedResource, at time.Time,
	by *Actor) *Master {
	keys := keysOf(leftOut)
	entries := map[string]MasterEntry{}
	documents := map[string]string{}
	for _, resource := range parseBundle(version.Resources) {
		key := resource.key()
		if keys[key] {
			continue
		}
		documents[key] = resource.Content
		entries[key] = MasterEntry{Key: key, ResourceType: resource.Type, ID: resource.ID, Name: resource.Name,
			ContentHash: contentHash(resource.Content), SourceVersion: version.Hash, State: EntryDeployed,
			At: at, By: by}
	}
	master := assemble(nil, entries, documents, masterWrite{at: at, by: by})
	master.GatewayID = gatewayID
	return master
}

// emptyMaster is what a gateway nothing was applied to holds.
func emptyMaster(gatewayID string) *Master {
	return &Master{GatewayID: gatewayID, Entries: []MasterEntry{}}
}

// outcomeMatcher pairs the resources an apply sent with the outcomes the gateway answered. An
// outcome names a resource by its id when the gateway got that far, and by its name alone when it
// refused the document before reading an id, so a resource is looked up by its id first and by its
// name after. Each outcome answers for one resource.
type outcomeMatcher struct {
	byID, byName map[string][]*importOutcome
	taken        map[*importOutcome]bool
	all          []*importOutcome
}

func newOutcomeMatcher(outcomes []importOutcome) *outcomeMatcher {
	m := &outcomeMatcher{byID: map[string][]*importOutcome{}, byName: map[string][]*importOutcome{},
		taken: map[*importOutcome]bool{}}
	for i := range outcomes {
		outcome := &outcomes[i]
		m.all = append(m.all, outcome)
		removal := outcome.Operation == operationDeleted
		if outcome.ResourceID != "" {
			m.byID[outcomeKey(outcome.ResourceType, outcome.ResourceID, removal)] =
				append(m.byID[outcomeKey(outcome.ResourceType, outcome.ResourceID, removal)], outcome)
		}
		if outcome.ResourceName != "" {
			m.byName[outcomeKey(outcome.ResourceType, outcome.ResourceName, removal)] =
				append(m.byName[outcomeKey(outcome.ResourceType, outcome.ResourceName, removal)], outcome)
		}
	}
	return m
}

func outcomeKey(resourceType, identity string, removal bool) string {
	if removal {
		return "-" + resourceType + "/" + identity
	}
	return resourceType + "/" + identity
}

// take returns the outcome that answers for a resource, once, or nil when none does.
func (m *outcomeMatcher) take(resource bundleResource, removal bool) *importOutcome {
	candidates := []*importOutcome{}
	if resource.ID != "" {
		candidates = append(candidates, m.byID[outcomeKey(resource.Type, resource.ID, removal)]...)
	}
	if resource.Name != "" {
		for _, outcome := range m.byName[outcomeKey(resource.Type, resource.Name, removal)] {
			// An outcome naming an id answers for the resource with that id, not for another one
			// sharing its name.
			if outcome.ResourceID == "" || resource.ID == "" {
				candidates = append(candidates, outcome)
			}
		}
	}
	for _, outcome := range candidates {
		if !m.taken[outcome] {
			m.taken[outcome] = true
			return outcome
		}
	}
	return nil
}

// refusedUnnamed reports whether the gateway refused something no sent resource was matched to.
func (m *outcomeMatcher) refusedUnnamed() bool {
	for _, outcome := range m.all {
		if !m.taken[outcome] && outcome.Status != outcomeSuccess {
			return true
		}
	}
	return false
}

// contentHash names a document by its content.
func contentHash(document string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(document)))
	return hex.EncodeToString(sum[:])
}

// masterHash names a master by what it holds: each entry's key, content, state, source and origin,
// so two gateways running the same resources from the same places hold the same master whoever
// applied it and whenever.
func masterHash(master *Master) string {
	h := sha256.New()
	for _, entry := range master.Entries {
		fields := []string{entry.Key, entry.ContentHash, string(entry.State), entry.SourceVersion, entry.Origin}
		// A refusal is part of what the gateway holds, so the same resource refused for another reason is
		// another master.
		if entry.Failure != nil {
			fields = append(fields, entry.Failure.Code)
		}
		h.Write([]byte(strings.Join(fields, "\x1f")))
		h.Write([]byte("\n"))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func summarizeMaster(entries []MasterEntry) MasterSummary {
	var summary MasterSummary
	for _, entry := range entries {
		switch entry.State {
		case EntryDeployed:
			summary.Deployed++
		case EntryFailed:
			summary.Failed++
		case EntryRemoved:
			summary.Removed++
		}
	}
	return summary
}

// manifestOf encodes a master's entries for storing, with each actor's id alone: a name is read from
// the issuer when shown, never stored.
func manifestOf(entries []MasterEntry) (string, error) {
	stored := make([]MasterEntry, len(entries))
	for i, entry := range entries {
		stored[i] = entry
		if entry.By != nil {
			stored[i].By = &Actor{ID: entry.By.ID}
		}
	}
	encoded, err := json.Marshal(stored)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

// entriesOf reads a stored manifest.
func entriesOf(manifest string) ([]MasterEntry, error) {
	entries := []MasterEntry{}
	if strings.TrimSpace(manifest) == "" {
		return entries, nil
	}
	if err := json.Unmarshal([]byte(manifest), &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// entryOf finds a master's entry for a key.
func entryOf(master *Master, key string) (MasterEntry, bool) {
	for _, entry := range master.Entries {
		if entry.Key == key {
			return entry, true
		}
	}
	return MasterEntry{}, false
}
