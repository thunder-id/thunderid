// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"

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
			if missing, _ := lacking(expanded, diff.Changes); len(missing) > 0 {
				return shipment{}, tidcommon.CustomServiceError(ErrorDependencyNotDeployed, tidcommon.I18nMessage{
					Key: "error.gatewayservice.dependency_held_back.description",
					DefaultValue: "{{param(missing)}}. This gateway holds that back: select it as well to send " +
						"it, or leave out what needs it.",
					Params: map[string]string{"missing": strings.Join(missing, "; ")}})
			}
			wanted = toSet(expanded)
		} else if missing, heldBack := lacking(*selection, diff.Changes); len(missing) > 0 {
			return shipment{}, dependenciesMissing(missing, heldBack)
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

// shipmentOf works out what an apply sends and the documents it writes, and refuses a removal that what
// the gateway is left with still needs. A promotion sends only what the source runs, so what it would
// leave unfinished, a resource naming one the source does not run, or a removal still needed by a
// resource the source does not run as the version has it, is refused in a promotion's terms, naming
// the source to deploy the version to first, since a promotion takes no selection to widen.
func shipmentOf(diff *Diff, target *Version, previous *Master, selection *[]string,
	options applyOptions) (shipment, []bundleResource, *tidcommon.ServiceError) {
	// What an apply planned from a master read before it held the lease is sent only while the gateway
	// still holds that master: another apply that finished in between leaves the plan stale.
	if options.plannedOn != nil && previous.Hash != *options.plannedOn {
		return shipment{}, nil, &ErrorAppliedChanged
	}
	if options.promotedFrom != "" && selection != nil {
		if missing, _ := lacking(*selection, diff.Changes); len(missing) > 0 {
			return shipment{}, nil, tidcommon.CustomServiceError(ErrorPromotionIncomplete, tidcommon.I18nMessage{
				Key: "error.gatewayservice.promotion_needs_unrun.description",
				DefaultValue: "{{param(missing)}}, which {{param(source)}} does not run as the version has it, " +
					"so it cannot be promoted. Deploy it to {{param(source)}} first.",
				Params: map[string]string{"missing": strings.Join(missing, "; "), "source": options.promotedFrom}})
		}
	}
	sent, svcErr := chosen(diff, target, selection, options.withDependencies)
	if svcErr != nil {
		return shipment{}, nil, svcErr
	}
	documents := parseBundle(sent.content)
	referenced := stillReferenced(sent.deletions, parseBundle(previous.Resources), documents)
	switch {
	case len(referenced) == 0:
		return sent, documents, nil
	case options.promotedFrom != "":
		return shipment{}, nil, tidcommon.CustomServiceError(ErrorPromotionIncomplete, tidcommon.I18nMessage{
			Key: "error.gatewayservice.promotion_removal_in_use.description",
			DefaultValue: "{{param(referenced)}}, which {{param(source)}} does not run as the version has it, " +
				"so this promotion cannot remove it. Deploy the version to {{param(source)}} first.",
			Params: map[string]string{"referenced": strings.Join(referenced, "; "), "source": options.promotedFrom}})
	default:
		return shipment{}, nil, removalsInUse(referenced)
	}
}

// memberRemovalTypes maps a resource type whose document lists who belongs to it to the removal that
// takes them off it.
var memberRemovalTypes = map[string]string{resourceTypeRole: "role_assignment", resourceTypeGroup: "group_member"}

// memberRemovals works out, for each role and group an apply writes over a document the master holds,
// the assignees or members that document has and the written one no longer does. An import adds who a
// document names but never takes off who it leaves out, so they are removed on their own. Only those
// the master recorded are removed: an assignee or member the gateway added itself stays.
func memberRemovals(held, written []bundleResource) []gatewayDeletion {
	previous := make(map[string]string, len(held))
	for _, resource := range held {
		previous[resource.key()] = resource.Content
	}
	var removals []gatewayDeletion
	for _, resource := range written {
		removalType, lists := memberRemovalTypes[resource.Type]
		was, isHeld := previous[resource.key()]
		if !lists || !isHeld || resource.ID == "" {
			continue
		}
		before, unit, readBefore := membersOf(resource.Type, was)
		after, _, readAfter := membersOf(resource.Type, resource.Content)
		if !readBefore || !readAfter {
			continue
		}
		// A role assignment captured before assignments named their organization unit has none, so an
		// assignee without one matches the same assignee in any unit.
		kept := make(map[gatewayMember]bool, len(after))
		inAnyUnit := make(map[gatewayMember]bool, len(after))
		withoutUnit := make(map[gatewayMember]bool, len(after))
		for _, member := range after {
			kept[member] = true
			assignee := gatewayMember{ID: member.ID, Type: member.Type}
			inAnyUnit[assignee] = true
			withoutUnit[assignee] = withoutUnit[assignee] || member.OUID == ""
		}
		var removed []gatewayMember
		for _, member := range before {
			assignee := gatewayMember{ID: member.ID, Type: member.Type}
			if kept[member] || withoutUnit[assignee] || (member.OUID == "" && inAnyUnit[assignee]) {
				continue
			}
			if member.OUID == "" {
				member.OUID = unit
			}
			removed = append(removed, member)
		}
		if len(removed) > 0 {
			removals = append(removals, gatewayDeletion{ResourceType: removalType, ID: resource.ID, Members: removed})
		}
	}
	return removals
}

// membersOf reads the assignees a role document lists or the members a group document lists, the
// organization unit owning a role, and whether the document could be read.
func membersOf(resourceType, document string) ([]gatewayMember, string, bool) {
	var listed struct {
		OUID        string          `yaml:"ouId"`
		Assignments []gatewayMember `yaml:"assignments"`
		Members     []gatewayMember `yaml:"members"`
	}
	if err := yaml.Unmarshal([]byte(quotePlaceholders(document)), &listed); err != nil {
		return nil, "", false
	}
	if resourceType == resourceTypeRole {
		return listed.Assignments, listed.OUID, true
	}
	return listed.Members, "", true
}

// refuseRemovalsInUse refuses removals that a document the gateway keeps still names once this apply's
// documents are written over it. A removal that only an update in the same apply frees goes through,
// sent after that update, so the import cannot refuse the update and still make the removal.
func refuseRemovalsInUse(sent shipment, master *Master, updated []bundleResource) *tidcommon.ServiceError {
	referenced := stillReferenced(sent.deletions, parseBundle(master.Resources), updated)
	if len(referenced) == 0 {
		return nil
	}
	return removalsInUse(referenced)
}

// removalsInUse refuses removals still needed, naming each with what needs it.
func removalsInUse(referenced []string) *tidcommon.ServiceError {
	return tidcommon.CustomServiceError(ErrorRemovalInUse, tidcommon.I18nMessage{
		Key:          "error.gatewayservice.removal_in_use_by.description",
		DefaultValue: "{{param(referenced)}}. Remove or update them in the same apply, or leave it in place.",
		Params:       map[string]string{"referenced": strings.Join(referenced, "; ")}})
}

// dependenciesMissing refuses a selection sent without its dependencies, naming what it lacks. Sending
// with dependencies brings along only what the gateway does not hold back, so when a held-back change
// is among what it lacks, the caller is told to select it rather than to send dependencies along.
func dependenciesMissing(missing []string, heldBack bool) *tidcommon.ServiceError {
	if heldBack {
		return tidcommon.CustomServiceError(ErrorDependencyNotDeployed, tidcommon.I18nMessage{
			Key:          "error.gatewayservice.dependency_held_back_missing.description",
			DefaultValue: "{{param(missing)}}. This gateway holds some of that back: select it as well to send it.",
			Params:       map[string]string{"missing": strings.Join(missing, "; ")}})
	}
	return tidcommon.CustomServiceError(ErrorDependencyNotDeployed, tidcommon.I18nMessage{
		Key:          "error.gatewayservice.dependency_missing.description",
		DefaultValue: "{{param(missing)}}. Select them as well, or apply with withDependencies.",
		Params:       map[string]string{"missing": strings.Join(missing, "; ")}})
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
