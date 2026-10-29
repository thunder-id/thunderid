// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

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
// Naming a change that changes nothing is allowed; there is nothing to send for it. With
// dependencies, the selection is widened to what the selected resources need that the gateway lacks;
// without, a selection needing such a resource is refused. The changes sent are flagged Selected on
// the diff, and those sent for another's need Required.
func chosen(diff *Diff, target *Version, selection *[]string,
	withDependencies bool) (shipment, *tidcommon.ServiceError) {
	wanted := map[string]bool{}
	required := map[string]string{}
	if selection != nil {
		wanted = toSet(*selection)
		named := 0
		for _, change := range diff.Changes {
			if wanted[change.Key] {
				named++
			}
		}
		if named != len(wanted) {
			return shipment{}, &ErrorInvalidSelection
		}
		if withDependencies {
			var expanded []string
			expanded, required = withNeeded(*selection, diff.Changes)
			wanted = toSet(expanded)
		} else if missing := lacking(*selection, diff.Changes); len(missing) > 0 {
			return shipment{}, tidcommon.CustomServiceError(ErrorDependencyNotDeployed, tidcommon.I18nMessage{
				Key:          ErrorDependencyNotDeployed.ErrorDescription.Key,
				DefaultValue: strings.Join(missing, "; ") + ". Select them as well, or apply with withDependencies."})
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
		if by, ok := required[change.Key]; ok {
			diff.Changes[i].Required = true
			diff.Changes[i].RequiredBy = by
		}
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
	return shipment{content: joinBundle(documents), deletions: deletions}, nil
}

// refuseRemovalsInUse refuses removals that a document the gateway keeps still names.
func refuseRemovalsInUse(sent shipment, master *Master) *tidcommon.ServiceError {
	referenced := stillReferenced(sent.deletions, parseBundle(master.Resources))
	if len(referenced) == 0 {
		return nil
	}
	return tidcommon.CustomServiceError(ErrorRemovalInUse, tidcommon.I18nMessage{
		Key: ErrorRemovalInUse.ErrorDescription.Key,
		DefaultValue: strings.Join(referenced, "; ") +
			". Remove or update them in the same apply, or leave it in place."})
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
