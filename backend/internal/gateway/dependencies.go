// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"
)

// What a resource needs to run is read from its own document: an application names its flows and
// its unit by id, a flow its connections, a role its resource server, and so on. Every scalar in a
// document that is the id of another document in the same bundle is taken as a dependency, so
// nothing has to be declared per resource type, and a new type joins in by being in the bundle. A value
// under a field that describes the resource, such as its name, is never a reference. A reference by
// name or handle is not read: a resource named that way is expected to be on the gateway already.

// scalarLine reads the scalar a block-style line holds: a mapping's value, a list item, or a list
// item's mapping value, quoted or not.
var scalarLine = regexp.MustCompile(`^\s*(?:-\s+` + mappingKey + `\s*:\s*|-\s*|` + mappingKey + `\s*:\s*)` +
	`['"]?([^'"#]+?)['"]?\s*$`)

// describingKeys are the fields that describe a resource in words rather than name another, so a value
// under one is never read as a reference, even when it equals an id: an id given in a declarative file
// need not be a UUID, and such an id can also be someone's name or description.
var describingKeys = map[string]bool{"name": true, "displayName": true, "description": true, "handle": true,
	"text": true, "title": true, "label": true, "username": true, "email": true}

// kindKeys say what kind of thing a document, or an entry in it, is, at any depth, such as a member's
// `type: user`, and never name another resource. A document's own top-level `id` is its identity, not a
// reference either, while a nested id, such as a member's, is one.
var kindKeys = map[string]bool{"resource_type": true, "type": true, "category": true}

// needsOf reads, for each resource in a bundle, the keys of the resources its document names by id,
// sorted, each once. A document never needs itself. Resources of different types that share an id are
// all needed, since the document names the id alone.
func needsOf(resources []bundleResource) map[string][]string {
	byID := make(map[string][]string, len(resources))
	for _, resource := range resources {
		if resource.ID != "" {
			byID[resource.ID] = append(byID[resource.ID], resource.key())
		}
	}
	needs := make(map[string][]string, len(resources))
	for _, resource := range resources {
		own := resource.key()
		seen := map[string]bool{}
		for _, line := range strings.Split(resource.Content, "\n") {
			match := scalarLine.FindStringSubmatch(line)
			if len(match) < 4 {
				continue
			}
			field := strings.Trim(match[1]+match[2], `"'`)
			if describingKeys[field] || kindKeys[field] ||
				(field == "id" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "-")) {
				continue
			}
			for _, needed := range byID[strings.TrimSpace(match[3])] {
				if needed == own || seen[needed] {
					continue
				}
				seen[needed] = true
				needs[own] = append(needs[own], needed)
			}
		}
		sort.Strings(needs[own])
	}
	return needs
}

// sends reports whether a change writes its resource to the gateway.
func sends(change Change) bool {
	return change.Change == ChangeAdded || change.Change == ChangeUpdated
}

// withNeeded widens a selection to what the selected changes need that the gateway lacks or holds
// differently: each needed resource whose change writes it, followed through its own needs. A
// dependency the gateway holds as it is is not sent, and one the gateway holds back is not taken in:
// only selecting it releases it. It returns the keys added, each with the selected key that first
// needed it.
func withNeeded(selection []string, changes []Change) (expanded []string, required map[string]string) {
	byKey := make(map[string]Change, len(changes))
	for _, change := range changes {
		byKey[change.Key] = change
	}
	chosen := toSet(selection)
	required = map[string]string{}
	expanded = append([]string{}, selection...)
	queue := append([]string{}, selection...)
	for len(queue) > 0 {
		key := queue[0]
		queue = queue[1:]
		for _, needed := range byKey[key].Needs {
			change, ok := byKey[needed]
			if !ok || chosen[needed] || !sends(change) || change.HeldBack {
				continue
			}
			chosen[needed] = true
			root := key
			if by, ok := required[key]; ok {
				root = by
			}
			required[needed] = root
			expanded = append(expanded, needed)
			queue = append(queue, needed)
		}
	}
	return expanded, required
}

// lacking returns, for a selection sent without its dependencies, what each selected change needs
// that the gateway does not hold or holds refused, sorted by the selected key, and whether the gateway
// holds back any of it. A dependency the gateway holds in another form is held: the selected change can
// name it, and replacing it is a choice of its own, which can change what else on the gateway uses it.
func lacking(selection []string, changes []Change) (missing []string, heldBack bool) {
	byKey := make(map[string]Change, len(changes))
	for _, change := range changes {
		byKey[change.Key] = change
	}
	chosen := toSet(selection)
	for _, key := range selection {
		for _, needed := range byKey[key].Needs {
			if change, ok := byKey[needed]; ok && !chosen[needed] && sends(change) &&
				(change.Change == ChangeAdded || change.Failed) {
				missing = append(missing, key+" needs "+needed)
				heldBack = heldBack || change.HeldBack
			}
		}
	}
	sort.Strings(missing)
	return missing, heldBack
}

// stillReferenced returns the removals that a document the gateway keeps still names by id, each with
// the key that names it: removing it would leave that document pointing at nothing. The documents are
// read as this apply leaves them: one the apply writes over is read as the version has it, so a
// removal it no longer names goes through, and one removed in the same apply does not count.
func stillReferenced(deletions []gatewayDeletion, held, updated []bundleResource) []string {
	if len(deletions) == 0 {
		return nil
	}
	removing := map[string]bool{}
	for _, deletion := range deletions {
		removing[deletion.ResourceType+"/"+deletion.ID] = true
	}
	var referenced []string
	for holder, needs := range needsOf(projectedOver(held, updated)) {
		if removing[holder] {
			continue
		}
		for _, needed := range needs {
			if removing[needed] {
				referenced = append(referenced, needed+" is needed by "+holder)
			}
		}
	}
	sort.Strings(referenced)
	return referenced
}

// projectedOver is what a gateway keeps once an apply's documents are written over it.
func projectedOver(held, updated []bundleResource) []bundleResource {
	byKey := make(map[string]bundleResource, len(held)+len(updated))
	order := make([]string, 0, len(held)+len(updated))
	for _, resource := range held {
		byKey[resource.key()] = resource
		order = append(order, resource.key())
	}
	for _, resource := range updated {
		if _, ok := byKey[resource.key()]; !ok {
			order = append(order, resource.key())
		}
		byKey[resource.key()] = resource
	}
	projected := make([]bundleResource, 0, len(order))
	for _, key := range order {
		projected = append(projected, byKey[key])
	}
	return projected
}

// orderedRemovals puts each removal before the removals of what its document names, as the gateway
// holds it: a child unit before its parent, an application before its flows. The gateway's import
// keeps the order of removals of one type, so a resource is not refused for still being needed by one
// removed later in the same apply. Removals that need nothing of each other keep their order.
func orderedRemovals(deletions []gatewayDeletion, held []bundleResource) []gatewayDeletion {
	if len(deletions) < 2 {
		return deletions
	}
	byKey := make(map[string]gatewayDeletion, len(deletions))
	for _, deletion := range deletions {
		byKey[deletion.ResourceType+"/"+deletion.ID] = deletion
	}
	dependents := map[string][]string{}
	for holder, needs := range needsOf(held) {
		if _, removing := byKey[holder]; !removing {
			continue
		}
		for _, needed := range needs {
			if _, removing := byKey[needed]; removing {
				dependents[needed] = append(dependents[needed], holder)
			}
		}
	}
	ordered := make([]gatewayDeletion, 0, len(deletions))
	visited := map[string]bool{}
	var visit func(key string)
	visit = func(key string) {
		if visited[key] {
			return
		}
		visited[key] = true
		sort.Strings(dependents[key])
		for _, dependent := range dependents[key] {
			visit(dependent)
		}
		ordered = append(ordered, byKey[key])
	}
	for _, deletion := range deletions {
		visit(deletion.ResourceType + "/" + deletion.ID)
	}
	return ordered
}

// deferredRemovals splits an apply's removals into those sent with its documents and those sent only
// once the documents are written: a removal that a document the gateway keeps names, which this apply
// updates to stop naming it, waits for that update, so a refused update leaves the resource in place.
// For each removal that waits it returns the documents the gateway has to accept first.
func deferredRemovals(deletions []gatewayDeletion, held,
	updated []bundleResource) (now, after []gatewayDeletion, awaiting map[string][]bundleResource) {
	if len(deletions) == 0 || len(updated) == 0 {
		return deletions, nil, nil
	}
	updating := make(map[string]bundleResource, len(updated))
	for _, resource := range updated {
		updating[resource.key()] = resource
	}
	holders := map[string][]bundleResource{}
	for holder, needs := range needsOf(held) {
		resource, ok := updating[holder]
		if !ok {
			continue
		}
		for _, needed := range needs {
			holders[needed] = append(holders[needed], resource)
		}
	}
	awaiting = map[string][]bundleResource{}
	for _, deletion := range deletions {
		key := deletion.ResourceType + "/" + deletion.ID
		if held, ok := holders[key]; ok {
			awaiting[key] = held
		}
	}
	// A removal that a waiting removal's document names waits with it, for the same updates: the gateway
	// keeps that document, and so its reference, until the waiting removal is made.
	removing := make(map[string]bool, len(deletions))
	for _, deletion := range deletions {
		removing[deletion.ResourceType+"/"+deletion.ID] = true
	}
	// A removal waits for every update any removal it follows waits for, so it is made only once all of
	// them were accepted.
	heldNeeds := needsOf(held)
	for changed := true; changed; {
		changed = false
		for waiting, updates := range awaiting {
			for _, needed := range heldNeeds[waiting] {
				if !removing[needed] {
					continue
				}
				merged, grew := unionResources(awaiting[needed], updates)
				if grew || awaiting[needed] == nil {
					awaiting[needed] = merged
					changed = true
				}
			}
		}
	}
	for _, deletion := range deletions {
		if _, waits := awaiting[deletion.ResourceType+"/"+deletion.ID]; waits {
			after = append(after, deletion)
			continue
		}
		now = append(now, deletion)
	}
	return now, after, awaiting
}

// unionResources adds to a list the resources of another it lacks, by key, and reports whether it grew.
func unionResources(into, from []bundleResource) ([]bundleResource, bool) {
	have := make(map[string]bool, len(into))
	for _, resource := range into {
		have[resource.key()] = true
	}
	merged, grew := append([]bundleResource{}, into...), false
	for _, resource := range from {
		if !have[resource.key()] {
			have[resource.key()] = true
			merged = append(merged, resource)
			grew = true
		}
	}
	return merged, grew
}

// releasedRemovals returns, of the removals that waited for an update, those whose updates the
// gateway accepted, read from its answer the way the master reads it. One whose update the gateway
// refused, or did not answer for while refusing something, stays in place.
func releasedRemovals(after []gatewayDeletion, awaiting map[string][]bundleResource,
	answer json.RawMessage) []gatewayDeletion {
	if len(after) == 0 {
		return nil
	}
	outcomes, answered := outcomesOf(answer)
	if !answered {
		return nil
	}
	// Read as the master reads the answer: a document the answer names is accepted when it says so,
	// and one it does not name is taken as written unless the gateway refused something it did not name.
	matcher := newOutcomeMatcher(outcomes)
	named := map[string]*importOutcome{}
	for _, held := range awaiting {
		for _, resource := range held {
			if _, seen := named[resource.key()]; !seen {
				named[resource.key()] = matcher.take(resource, false)
			}
		}
	}
	unnamedFailure := matcher.refusedUnnamed()
	accepted := make(map[string]bool, len(named))
	for key, outcome := range named {
		if outcome == nil {
			accepted[key] = !unnamedFailure
			continue
		}
		accepted[key] = outcome.Status == outcomeSuccess
	}
	var released []gatewayDeletion
	for _, deletion := range after {
		ready := true
		for _, resource := range awaiting[deletion.ResourceType+"/"+deletion.ID] {
			ready = ready && accepted[resource.key()]
		}
		if ready {
			released = append(released, deletion)
		}
	}
	return released
}

// mergeAnswers joins the answers of two imports into one: the results in order, the summary summed. It
// reports false when either answer cannot be read, so what the second import was for is not taken as
// done on the first answer alone.
func mergeAnswers(first, second json.RawMessage) (json.RawMessage, bool) {
	type parsed struct {
		Summary map[string]json.RawMessage `json:"summary"`
		Results []json.RawMessage          `json:"results"`
	}
	var a, b parsed
	if _, answered := outcomesOf(second); !answered {
		return first, false
	}
	if json.Unmarshal(first, &a) != nil || json.Unmarshal(second, &b) != nil {
		return first, false
	}
	summary := map[string]interface{}{}
	for _, part := range []map[string]json.RawMessage{a.Summary, b.Summary} {
		for key, raw := range part {
			var number float64
			if json.Unmarshal(raw, &number) == nil {
				sum, _ := summary[key].(float64)
				summary[key] = sum + number
				continue
			}
			if _, ok := summary[key]; !ok {
				summary[key] = raw
			}
		}
	}
	results := append(append([]json.RawMessage{}, a.Results...), b.Results...)
	if results == nil {
		results = []json.RawMessage{}
	}
	merged, err := json.Marshal(map[string]interface{}{"summary": summary, "results": results})
	if err != nil {
		return first, false
	}
	return merged, true
}
