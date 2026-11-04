// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/log"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// An apply sends a gateway only what its master says the gateway lacks or holds differently, and a
// selection narrows that to the changes named. A selection is for that apply alone; a change an apply
// names to leave out is held back from the applies after it too, until one selects it.

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
// dependencies, the selection is widened to what the selected resources need that the gateway lacks,
// and refused when one of those is held back; without, a selection needing such a resource is
// refused. The changes sent are flagged Selected on the diff, and those sent for another's need
// Required.
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
			if missing := lacking(expanded, diff.Changes); len(missing) > 0 {
				return shipment{}, tidcommon.CustomServiceError(ErrorDependencyNotDeployed, tidcommon.I18nMessage{
					Key: ErrorDependencyNotDeployed.ErrorDescription.Key,
					DefaultValue: strings.Join(missing, "; ") + ". This gateway holds that back: select it as " +
						"well to send it, or leave out what needs it."})
			}
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

// refuseRemovalsInUse refuses removals that a document the gateway keeps still names once this apply's
// documents are written over it. A removal that only an update in the same apply frees goes through,
// sent after that update, so the import cannot refuse the update and still make the removal.
func refuseRemovalsInUse(sent shipment, master *Master, updated []bundleResource) *tidcommon.ServiceError {
	referenced := stillReferenced(sent.deletions, parseBundle(master.Resources), updated)
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

// heldBack reads the changes a gateway holds back, with those this apply leaves out. A row an apply
// made before masters to skip a resource once is not a standing choice, so it is not held back.
func (s *versionService) heldBack(ctx context.Context, gatewayID string, leaveOut []string) (
	map[string]bool, *tidcommon.ServiceError) {
	rows, err := s.versions.GetExcluded(ctx, gatewayID)
	if err != nil {
		s.logger.Error(ctx, "Failed to read what a gateway holds back", log.String("gatewayId", gatewayID),
			log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	held := make(map[string]bool, len(rows)+len(leaveOut))
	for _, row := range rows {
		if !row.Once {
			held[row.Key] = true
		}
	}
	for _, key := range leaveOut {
		held[key] = true
	}
	return held, nil
}

// holdBackOn reads what the gateway holds back, with what this apply leaves out, and marks it on the
// diff as holdBack does.
func (s *versionService) holdBackOn(ctx context.Context, gatewayID string, diff *Diff, selection *[]string,
	leaveOut []string) (*[]string, *tidcommon.ServiceError) {
	held, svcErr := s.heldBack(ctx, gatewayID, leaveOut)
	if svcErr != nil {
		return nil, svcErr
	}
	return holdBack(diff, selection, leaveOut, held)
}

// holdBack marks the changes held back on the diff, and, for an apply without a selection, selects
// every other change so the held back ones are not sent. A change left out must be one the diff
// reports as a change: a resource the gateway already holds as the version has it has nothing to
// hold back, and a hold on it would show nowhere until the resource changed. A selection cannot both
// name a change and leave it out.
func holdBack(diff *Diff, selection *[]string, leaveOut []string, held map[string]bool) (
	*[]string, *tidcommon.ServiceError) {
	offered := map[string]bool{}
	for _, change := range diff.Changes {
		if change.Change != ChangeUnchanged {
			offered[change.Key] = true
		}
	}
	selected := map[string]bool{}
	if selection != nil {
		selected = toSet(*selection)
	}
	for _, key := range leaveOut {
		if !offered[key] || selected[key] {
			return nil, &ErrorInvalidSelection
		}
	}
	marked := false
	for i, change := range diff.Changes {
		if change.Change != ChangeUnchanged && held[change.Key] {
			diff.Changes[i].HeldBack = true
			marked = true
		}
	}
	if selection != nil || !marked {
		return selection, nil
	}
	rest := []string{}
	for _, change := range diff.Changes {
		if change.Change != ChangeUnchanged && !change.HeldBack {
			rest = append(rest, change.Key)
		}
	}
	return &rest, nil
}

// rememberHeldBack keeps what an apply left out held back, and stops holding back what it sent. It
// runs in the transaction the apply records its outcome in: a hold the caller asked for is part of
// what the apply leaves, so a failure to keep it records nothing, and the apply fails as a failure to
// record the master does.
func (s *versionService) rememberHeldBack(ctx context.Context, gatewayID string, diff *Diff,
	leaveOut []string) *tidcommon.ServiceError {
	leaving := toSet(leaveOut)
	var hold []excludedResource
	var release []string
	for _, change := range diff.Changes {
		switch {
		case leaving[change.Key]:
			hold = append(hold, excludedResource{Key: change.Key, Type: change.ResourceType, ID: change.ID,
				Name: change.Name})
		case change.HeldBack && change.Selected:
			release = append(release, change.Key)
		}
	}
	if len(hold) == 0 && len(release) == 0 {
		return nil
	}
	if err := s.versions.SetHeldBack(ctx, gatewayID, hold, release); err != nil {
		s.logger.Error(ctx, "Failed to record what a gateway holds back", log.String("gatewayId", gatewayID),
			log.Error(err))
		return &tidcommon.InternalServerError
	}
	return nil
}
