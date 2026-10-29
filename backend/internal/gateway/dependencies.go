// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"regexp"
	"sort"
	"strings"
)

// What a resource needs to run is read from its own document: an application names its flows and
// its unit by id, a flow its connections, a role its resource server, and so on. Every scalar in a
// document that is the id of another document in the same bundle is taken as a dependency, so
// nothing has to be declared per resource type, and a new type joins in by being in the bundle. Ids
// are UUIDs, so a scalar equal to one is not something else by coincidence. A reference by name or
// handle is not read: a resource named that way is expected to be on the gateway already.

// scalarLine reads the scalar a block-style line holds: a mapping's value, a list item, or a list
// item's mapping value, quoted or not.
var scalarLine = regexp.MustCompile(`^\s*(?:-\s+` + mappingKey + `\s*:\s*|-\s*|` + mappingKey + `\s*:\s*)` +
	`['"]?([^'"#]+?)['"]?\s*$`)

// needsOf reads, for each resource in a bundle, the keys of the resources its document names by id,
// sorted, each once. A document never needs itself.
func needsOf(resources []bundleResource) map[string][]string {
	byID := make(map[string]string, len(resources))
	for _, resource := range resources {
		if resource.ID != "" {
			byID[resource.ID] = resource.key()
		}
	}
	needs := make(map[string][]string, len(resources))
	for _, resource := range resources {
		own := resource.key()
		seen := map[string]bool{}
		for _, line := range strings.Split(resource.Content, "\n") {
			match := scalarLine.FindStringSubmatch(line)
			if match == nil {
				continue
			}
			needed, ok := byID[strings.TrimSpace(match[3])]
			if !ok || needed == own || seen[needed] {
				continue
			}
			seen[needed] = true
			needs[own] = append(needs[own], needed)
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
// dependency the gateway holds as it is is not sent. It returns the keys added, each with the selected
// key that first needed it.
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
			if !ok || chosen[needed] || !sends(change) {
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
// that the gateway does not hold as it is, sorted by the selected key.
func lacking(selection []string, changes []Change) []string {
	byKey := make(map[string]Change, len(changes))
	for _, change := range changes {
		byKey[change.Key] = change
	}
	chosen := toSet(selection)
	var missing []string
	for _, key := range selection {
		for _, needed := range byKey[key].Needs {
			if change, ok := byKey[needed]; ok && !chosen[needed] && sends(change) {
				missing = append(missing, key+" needs "+needed)
			}
		}
	}
	sort.Strings(missing)
	return missing
}

// stillReferenced returns the removals that a document the gateway keeps still names by id, each with
// the key that names it: removing it would leave that document pointing at nothing. A document
// removed in the same apply does not count.
func stillReferenced(deletions []gatewayDeletion, held []bundleResource) []string {
	removing := map[string]bool{}
	for _, deletion := range deletions {
		removing[deletion.ResourceType+"/"+deletion.ID] = true
	}
	var referenced []string
	for holder, needs := range needsOf(held) {
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
