// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"unicode/utf8"

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

// The longest key, and type, id and name, a gateway can keep for a change it holds back: the widths
// of the columns that keep them, in characters.
const (
	maxResourceKey   = 512
	maxResourceField = 255
)

// keepable reports whether a change held back fits the columns that keep it. A column's width is
// counted in characters, so a value is too.
func keepable(change Change) bool {
	fits := func(value string, width int) bool { return utf8.RuneCountInString(value) <= width }
	return fits(change.Key, maxResourceKey) && fits(change.ResourceType, maxResourceField) &&
		fits(change.ID, maxResourceField) && fits(change.Name, maxResourceField)
}

// holdBack marks the changes held back on the diff, and, for an apply without a selection, selects
// every other change so the held back ones are not sent. A change left out must be one the diff
// reports as a change: a resource the gateway already holds as the version has it has nothing to
// hold back, and a hold on it would show nowhere until the resource changed. A selection cannot both
// name a change and leave it out, and a change too long to keep is refused before anything is sent,
// rather than failing the apply once the gateway has written it.
func holdBack(diff *Diff, selection *[]string, leaveOut []string, held map[string]bool) (
	*[]string, *tidcommon.ServiceError) {
	offered := map[string]Change{}
	for _, change := range diff.Changes {
		if change.Change != ChangeUnchanged {
			offered[change.Key] = change
		}
	}
	selected := map[string]bool{}
	if selection != nil {
		selected = toSet(*selection)
	}
	for _, key := range leaveOut {
		change, ok := offered[key]
		if !ok || selected[key] || !keepable(change) {
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
