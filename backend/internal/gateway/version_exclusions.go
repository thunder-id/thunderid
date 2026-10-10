// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"sort"
	"unicode/utf8"
)

// Leaving a resource out of an apply is a standing choice, not a choice for one apply. Deselecting a
// change means "not this resource, on this gateway", and asking again on every apply would have it
// made again each time until it was let through by mistake. So it is kept for the gateway, applies
// by default, and selecting the resource again clears it.
//
// What a gateway is recorded as holding is the version applied to it, which says nothing of the
// resources left out of it. So a resource left alone stays on offer in every diff, whatever the
// version says of it, until it is selected again.

// excludedResource is a resource a gateway is set to leave alone. Its type, id and name are kept
// with its key, so it can be offered once no version names it any more: a removal held back is
// still owed to the gateway then.
type excludedResource struct {
	Key  string
	Type string
	ID   string
	Name string
}

// offered reports whether a change is one a selection decides: one that changes something, or one
// the gateway is set to leave alone, since what it holds of that resource is not known.
func offered(change Change) bool {
	return change.Change != ChangeUnchanged || change.Excluded
}

// The longest key, and type, id and name, a gateway can keep for a resource it leaves alone: the
// widths of the columns that keep them, in characters.
const (
	maxResourceKey   = 512
	maxResourceField = 255
)

// keepable reports whether a resource left out fits the columns that keep it. A column's width is
// counted in characters, so a value is too.
func keepable(change Change) bool {
	fits := func(value string, width int) bool { return utf8.RuneCountInString(value) <= width }
	return fits(change.Key, maxResourceKey) && fits(change.ResourceType, maxResourceField) &&
		fits(change.ID, maxResourceField) && fits(change.Name, maxResourceField)
}

// validSelection reports whether a selection names only resources the diff reports, and leaves out
// none too long to keep. A key that names nothing in the diff is a typo or comes from a stale diff,
// and ignoring it would leave out the resource that was meant instead. Naming a resource that did not
// change is allowed: there is nothing to apply for it either way.
func validSelection(changes []Change, excluded []excludedResource, selection []string) bool {
	selected := toSet(selection)
	held := keysOf(excluded)
	named := 0
	for _, change := range changes {
		if selected[change.Key] {
			named++
		} else if offered(change) && !held[change.Key] && !keepable(change) {
			return false
		}
	}
	return named == len(selected)
}

// nextExclusions folds a selection into a gateway's standing exclusions.
//
// Only the changes on offer are reconsidered: a resource that did not change and is not left alone
// is neither left out nor taken back, so the choice outlasts an apply that never showed it.
func nextExclusions(existing []excludedResource, changes []Change,
	selection []string) (next, exclude []excludedResource, include []string) {
	held := make(map[string]excludedResource, len(existing))
	for _, resource := range existing {
		held[resource.Key] = resource
	}
	selected := toSet(selection)
	for _, change := range changes {
		if !offered(change) {
			continue
		}
		_, isHeld := held[change.Key]
		switch {
		case selected[change.Key] && isHeld:
			delete(held, change.Key)
			include = append(include, change.Key)
		case !selected[change.Key] && !isHeld:
			resource := excludedResource{Key: change.Key, Type: change.ResourceType, ID: change.ID, Name: change.Name}
			held[change.Key] = resource
			exclude = append(exclude, resource)
		}
	}
	next = make([]excludedResource, 0, len(held))
	for _, resource := range held {
		next = append(next, resource)
	}
	sort.Slice(next, func(i, j int) bool { return next[i].Key < next[j].Key })
	return next, exclude, include
}

// markExcluded flags the changes a gateway is set to leave alone, and offers as a removal each one
// left alone that neither version names: one whose removal was held back is still owed then.
func markExcluded(changes []Change, excluded []excludedResource) []Change {
	keys := keysOf(excluded)
	named := make(map[string]bool, len(changes))
	for i := range changes {
		changes[i].Excluded = keys[changes[i].Key]
		named[changes[i].Key] = true
	}
	for _, resource := range excluded {
		if !named[resource.Key] {
			changes = append(changes, Change{Key: resource.Key, ResourceType: resource.Type, ID: resource.ID,
				Name: resource.Name, Change: ChangeDeleted, Excluded: true})
		}
	}
	sortChanges(changes)
	return changes
}

// contentWithout is a version's content less the resources a gateway is set to leave alone. With
// none left out it is the content as captured.
func contentWithout(content string, excluded []excludedResource) string {
	resources := parseBundle(content)
	keys := keysOf(excluded)
	kept := make([]string, 0, len(resources))
	for _, resource := range resources {
		if !keys[resource.key()] {
			kept = append(kept, resource.Content)
		}
	}
	if len(kept) == len(resources) {
		return content
	}
	return joinBundle(kept)
}

func keysOf(excluded []excludedResource) map[string]bool {
	keys := make(map[string]bool, len(excluded))
	for _, resource := range excluded {
		keys[resource.Key] = true
	}
	return keys
}

func toSet(keys []string) map[string]bool {
	set := make(map[string]bool, len(keys))
	for _, key := range keys {
		set[key] = true
	}
	return set
}
