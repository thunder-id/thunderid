// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importer

import (
	"context"
	"fmt"
	"slices"

	"github.com/thunder-id/thunderid/internal/application"
	"github.com/thunder-id/thunderid/internal/connection"
	flowmgt "github.com/thunder-id/thunderid/internal/flow/mgt"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/user"
)

// SharingPolicyRemoval names a sharing policy the document no longer carries for a resource, so the import removes it
// from the resource once the resource itself is written.
type SharingPolicyRemoval struct {
	// ResourceType is the type of the resource the policy governs, as the resource's own document names it.
	ResourceType string `json:"resourceType"`
	// ID is the id of the resource.
	ID string `json:"id"`
	// InitiatingOUID is the organization unit that issued the policy, empty for the resource's owner.
	InitiatingOUID string `json:"initiatingOuId,omitempty"`
}

// sharingTypeOf returns the sharing resource type of a resource type that can be shared.
func sharingTypeOf(resourceType string) (sharing.ResourceType, bool) {
	switch resourceType {
	case resourceTypeFlow:
		return flowmgt.FlowSharingType, true
	case resourceTypeApplication:
		return application.ApplicationSharingType, true
	case resourceTypeRole:
		return role.RoleSharingType, true
	case resourceTypeUser:
		return user.UserSharingType, true
	case resourceTypeConnection:
		return connection.ConnectionSharingType, true
	default:
		return "", false
	}
}

type removalTarget struct{ resourceType, id string }

// removalPlan is what a request says about the sharing policies of the resources it writes: the policies to remove, and
// for each document the units it still carries a policy for. A removal comes from comparing how two versions of a
// document write each unit, and the owner may be written as an explicit id in one and left blank in the other, which
// reads as a drop. The units a document still names let the removal tell that case from a real one.
type removalPlan struct {
	removals []SharingPolicyRemoval
	named    map[removalTarget][]string
}

// newRemovalPlan reads, from each document, the units it still carries a sharing policy for. It returns nil when the
// request removes nothing, which includes a dry run.
func newRemovalPlan(removals []SharingPolicyRemoval, docs []parsedDocument, dryRun bool) *removalPlan {
	if len(removals) == 0 || dryRun {
		return nil
	}
	plan := &removalPlan{removals: removals, named: map[removalTarget][]string{}}
	for _, doc := range docs {
		var head struct {
			ID              string `yaml:"id"`
			SharingPolicies []struct {
				InitiatingOUID string `yaml:"initiatingOuId"`
			} `yaml:"sharingPolicies"`
		}
		if doc.Node == nil || doc.Node.Decode(&head) != nil || head.ID == "" {
			continue
		}
		key := removalTarget{doc.ResourceType, head.ID}
		for _, policy := range head.SharingPolicies {
			plan.named[key] = append(plan.named[key], policy.InitiatingOUID)
		}
	}
	return plan
}

// initiators returns the organization units whose policies the plan removes from one resource, the empty id standing
// for the owner. The resource may be named by either of the ids it is known by.
func (p *removalPlan) initiators(resourceType string, ids ...string) []string {
	if p == nil {
		return nil
	}
	var initiators []string
	for _, removal := range p.removals {
		if removal.ResourceType == resourceType && slices.Contains(ids, removal.ID) {
			initiators = append(initiators, removal.InitiatingOUID)
		}
	}
	return initiators
}

// stillNamed returns the units the resource's document still carries a policy for, the empty id standing for the owner.
func (p *removalPlan) stillNamed(resourceType string, ids ...string) []string {
	if p == nil {
		return nil
	}
	var named []string
	for i, id := range ids {
		if slices.Contains(ids[:i], id) {
			continue
		}
		named = append(named, p.named[removalTarget{resourceType, id}]...)
	}
	return named
}

// removeDroppedSharing removes the sharing policies the request names, from the resources it wrote. The removal
// belongs to the outcome of the resource: a refusal fails that outcome, so the caller keeps the resource as not
// accepted and offers it again, and what was removed is listed on it. A resource the request did not write
// successfully is left as it is, and so is everything on a dry run, which removes nothing. A flow is left out here,
// as its policies were removed before it was written. It returns how many outcomes turned from imported to failed.
func (s *importService) removeDroppedSharing(
	ctx context.Context, plan *removalPlan, results []ImportItemOutcome, dryRun bool,
) int {
	if plan == nil || dryRun {
		return 0
	}

	type target struct{ resourceType, id string }
	order := []target{}
	initiators := map[target][]string{}
	for _, removal := range plan.removals {
		if removal.ResourceType == resourceTypeFlow {
			continue
		}
		key := target{removal.ResourceType, removal.ID}
		if _, seen := initiators[key]; !seen {
			order = append(order, key)
		}
		initiators[key] = append(initiators[key], removal.InitiatingOUID)
	}

	refused := 0
	for _, key := range order {
		index := -1
		for i := range results {
			if results[i].ResourceType == key.resourceType && results[i].ResourceID == key.id &&
				results[i].Status == statusSuccess && results[i].Operation != operationDelete {
				index = i
				break
			}
		}
		if index < 0 {
			continue
		}
		removed, failure := s.removeSharingOf(ctx, key.resourceType, key.id, initiators[key],
			plan.stillNamed(key.resourceType, key.id))
		if failure != nil {
			results[index].Status = statusFailed
			results[index].Code = failure.Code
			results[index].Message = failure.Message
			refused++
			continue
		}
		results[index].RemovedSharing = append(results[index].RemovedSharing, removed...)
	}
	return refused
}

// sharingRemovalFailure is why removing a resource's sharing policies was refused.
type sharingRemovalFailure struct {
	Code    string
	Message string
}

// removeSharingOf removes the policies the given organization units issued for one resource, and returns what was
// removed or why it was refused.
func (s *importService) removeSharingOf(
	ctx context.Context, resourceType, id string, initiatingOUIDs, stillNamed []string,
) ([]RemovedSharingPolicy, *sharingRemovalFailure) {
	rt, ok := sharingTypeOf(resourceType)
	if !ok {
		return nil, &sharingRemovalFailure{ErrorInvalidImportRequest.Code,
			fmt.Sprintf("sharing policies cannot be removed from a %s", resourceType)}
	}
	if s.sharingService == nil {
		return nil, &sharingRemovalFailure{ErrorInvalidImportRequest.Code, "sharing adapter is not configured"}
	}

	replay, replayErr := s.sharingService.RemovePolicies(ctx, rt, id, initiatingOUIDs, stillNamed)
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ImportService"))
	for _, skipped := range replay.Skipped {
		logger.Warn(ctx, "Sharing policy was not removed; a declared policy governs this unit",
			log.String("resourceType", resourceType), log.String("resourceID", id),
			log.String("initiatingOUID", skipped.InitiatingOUID),
			log.String("declaredPolicyID", skipped.DeclaredPolicyID))
	}
	if replayErr != nil {
		return nil, &sharingRemovalFailure{replayErr.Err.Code,
			fmt.Sprintf("removing the sharing policy of organization unit %q: %s",
				replayErr.InitiatingOUID, replayErr.Err.ErrorDescription.DefaultValue)}
	}
	removed := make([]RemovedSharingPolicy, 0, len(replay.Removed))
	for _, policy := range replay.Removed {
		removed = append(removed, RemovedSharingPolicy{
			PolicyID: policy.PolicyID, InitiatingOUID: policy.InitiatingOUID, Reshare: policy.Reshare})
	}
	return removed, nil
}
