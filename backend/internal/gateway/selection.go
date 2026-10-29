// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

// An apply sends a gateway only what its master says the gateway lacks or holds differently, and a
// selection narrows that to the changes named. Nothing about the selection is kept: a change left
// out this time is offered again by the next diff, since the master still says the gateway lacks it.

// excludedResource is a resource a gateway was set to leave alone by an apply made before masters
// were recorded. It is read once, when such a gateway is given its first master, so the resources it
// never received are not recorded as deployed.
type excludedResource struct {
	Key  string
	Type string
	ID   string
	Name string
	Once bool
}

// shipment is what one apply sends: the documents to write and the resources to remove.
type shipment struct {
	content   string
	deletions []gatewayDeletion
}

// chosen works out what an apply sends from a diff and a selection. Without a selection every change
// is sent. A selection names changes by key, and is refused when it names one the diff does not
// report: a typo or a key from a stale diff would otherwise leave out the resource that was meant.
// Naming a change that changes nothing is allowed; there is nothing to send for it. The changes sent
// are flagged Selected on the diff.
func chosen(diff *Diff, target *Version, selection *[]string) (shipment, bool) {
	wanted := map[string]bool{}
	if selection != nil {
		wanted = toSet(*selection)
		named := 0
		for _, change := range diff.Changes {
			if wanted[change.Key] {
				named++
			}
		}
		if named != len(wanted) {
			return shipment{}, false
		}
	}
	take := func(change Change) bool { return selection == nil || wanted[change.Key] }

	sending := map[string]bool{}
	var deletions []gatewayDeletion
	for i, change := range diff.Changes {
		if !take(change) || change.Change == ChangeUnchanged {
			continue
		}
		diff.Changes[i].Selected = true
		switch change.Change {
		case ChangeDeleted:
			// A resource without an id cannot be named to the gateway, so it is left in place.
			if change.ID != "" {
				deletions = append(deletions, gatewayDeletion{ResourceType: change.ResourceType, ID: change.ID})
			}
		default:
			sending[change.Key] = true
		}
	}
	documents := make([]string, 0, len(sending))
	for _, resource := range parseBundle(target.Resources) {
		if sending[resource.key()] {
			documents = append(documents, resource.Content)
		}
	}
	return shipment{content: joinBundle(documents), deletions: deletions}, true
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
