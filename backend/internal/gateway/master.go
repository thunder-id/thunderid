// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
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

// masterWrite is one apply's effect on a gateway's master: what was sent to be written and removed,
// what the gateway answered, and where it all came from.
type masterWrite struct {
	sent      []bundleResource
	deletions []gatewayDeletion
	answer    json.RawMessage
	source    string
	origin    string
	by        *Actor
	at        time.Time
}

// nextMaster builds the master a gateway holds after an apply from the one it held before and the
// import's outcomes. For each resource sent, a success replaces or adds its document and records it
// deployed; a refusal keeps the document the gateway last accepted and records the failure. For each
// removal, a success drops the document and records it removed; a refusal keeps it with the failure.
// Every other document is carried over as it was, as is a sent resource the gateway already held as it
// is, which keeps where it came from. A resource the answer does not name is deployed when the
// gateway refused nothing it did not name, and failed otherwise, since it is not known.
func nextMaster(previous *Master, write masterWrite) *Master {
	outcomes, answered := outcomesOf(write.answer)
	matcher := newOutcomeMatcher(outcomes)

	documents := map[string]string{}
	entries := map[string]MasterEntry{}
	if previous != nil {
		for _, resource := range parseBundle(previous.Resources) {
			documents[resource.key()] = resource.Content
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
			entry.Failure = &EntryFailure{Code: outcome.Code, Message: outcome.Message}
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
	for _, deletion := range write.deletions {
		resource := bundleResource{Type: deletion.ResourceType, ID: deletion.ID}
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

	return assemble(previous, entries, documents, write)
}

// heldAsIs reports whether a master records a resource deployed with the same content.
func heldAsIs(entries map[string]MasterEntry, resource bundleResource) bool {
	entry, ok := entries[resource.key()]
	return ok && entry.State == EntryDeployed && entry.ContentHash == contentHash(resource.Content)
}

// assemble writes the master's file and manifest in key order and names it by its hash.
func assemble(previous *Master, entries map[string]MasterEntry, documents map[string]string,
	write masterWrite) *Master {
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
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
		candidates = append(candidates, m.byName[outcomeKey(resource.Type, resource.Name, removal)]...)
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
		h.Write([]byte(strings.Join([]string{entry.Key, entry.ContentHash, string(entry.State),
			entry.SourceVersion, entry.Origin}, "\x1f")))
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
