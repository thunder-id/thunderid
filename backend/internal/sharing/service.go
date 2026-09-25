// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/sharing/policyengine"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
)

const loggerComponentName = "SharingService"

// ResolvedOverlay is what one organization unit may do with a shared resource.
type ResolvedOverlay struct {
	// OUID is the organization unit the rules were resolved for.
	OUID string
	// Owned reports whether the organization unit owns the resource rather than being shared it.
	Owned bool
	// Visible reports whether the organization unit can see the resource at all. When false the
	// rules are empty: an organization unit the resource never reached has nothing it may do with
	// it, and reporting the type's defaults would read as though it did.
	Visible bool
	// PolicyIDs names every policy that contributed, so an intersection is traceable to its inputs.
	PolicyIDs []string
	// Rules is the effective rule per field key.
	Rules map[string]OverlayRule
	// Sources records whether each field came from a policy or from the declared default, which is
	// the difference between "the owner decided this" and "nobody said anything".
	Sources map[string]string
}

// Rule sources reported on a resolved overlay.
const (
	// SourcePolicy marks a field some policy named explicitly.
	SourcePolicy = "policy"
	// SourceDefault marks a field no policy named, resolved from the type's declaration.
	SourceDefault = "default"
)

// ServiceInterface is the sharing framework's entry point. It orchestrates; it does not decide.
type ServiceInterface interface {
	// RegisterResourceType adds a resource type's declaration to the framework.
	RegisterResourceType(decl ResourceTypeDeclaration)

	// CreatePolicy records one organization unit's decision about one resource. Exactly one policy
	// exists per resource per initiating organization unit; a second create is refused.
	CreatePolicy(
		ctx context.Context, rt ResourceType, resourceID, owningOUID string, req PolicyRequest,
	) (Policy, *tidcommon.ServiceError)
	// CreateDeclarativePolicy records a policy declared by a resource file. It runs the identical
	// validation, but the policy is held in memory and cannot later be edited through the API.
	CreateDeclarativePolicy(
		ctx context.Context, rt ResourceType, resourceID, owningOUID string, req PolicyRequest,
	) (Policy, *tidcommon.ServiceError)
	// UpdatePolicy replaces a policy's contents within the limits of its scope family.
	UpdatePolicy(ctx context.Context, policyID string, req PolicyRequest) (Policy, *tidcommon.ServiceError)
	// DeletePolicy removes a policy and cleans up the state of every organization unit that loses
	// visibility as a result.
	DeletePolicy(ctx context.Context, policyID string) *tidcommon.ServiceError

	// GetPolicy returns one policy by id.
	GetPolicy(ctx context.Context, policyID string) (Policy, *tidcommon.ServiceError)
	// ListPolicies returns every policy recorded for a resource.
	ListPolicies(ctx context.Context, rt ResourceType, resourceID string) ([]Policy, *tidcommon.ServiceError)
	// ExportPolicies returns a resource's policies in an order safe to replay sequentially.
	ExportPolicies(
		ctx context.Context, rt ResourceType, resourceID string,
	) ([]ReplayablePolicy, *tidcommon.ServiceError)

	// IsVisible reports whether an organization unit can see a resource, as its owner or as a
	// sharee. The owner is included, so this is not the same question as "has it been shared".
	IsVisible(ctx context.Context, rt ResourceType, resourceID, ouID string) (bool, *tidcommon.ServiceError)
	// ListVisibleResourceIDs returns the resources of a type an organization unit can see through
	// a sharing policy. Resources it owns are not included: ownership is the resource type's own
	// state, and this framework stores no resource rows to read it from.
	ListVisibleResourceIDs(
		ctx context.Context, rt ResourceType, ouID string,
	) ([]string, *tidcommon.ServiceError)
	// ResolveOverlayRules returns the effective rule per field for one organization unit.
	ResolveOverlayRules(
		ctx context.Context, rt ResourceType, resourceID, ouID string,
	) (ResolvedOverlay, *tidcommon.ServiceError)
}

// service orchestrates policy storage, chain resolution and the resource type's own hooks. Every
// decision is a call into policyengine.
type service struct {
	logger              log.Logger
	store               storeInterface
	declarativeStore    *declarativePolicyStore
	registry            *registry
	ouHierarchyResolver sysauthz.OUHierarchyResolver
	// ouEnumerator walks the tree downwards, which policy deletion needs to find the units beneath
	// a removed target. Kept separate from the resolver above: enumeration is not an access decision.
	ouEnumerator  OUEnumerator
	transactioner providers.Transactioner

	// The caches are all derived from the policy graph, so any write clears them wholesale: one
	// edit can change an unbounded number of resolved answers.
	visibilityCache  cache.CacheInterface[bool]
	overlayRuleCache cache.CacheInterface[ResolvedOverlay]

	allowChildOUCrossTreeSharing bool
}

// newService builds the sharing service.
func newService(
	store storeInterface,
	declarativeStore *declarativePolicyStore,
	ouHierarchyResolver sysauthz.OUHierarchyResolver,
	ouEnumerator OUEnumerator,
	transactioner providers.Transactioner,
	visibilityCache cache.CacheInterface[bool],
	overlayRuleCache cache.CacheInterface[ResolvedOverlay],
	allowChildOUCrossTreeSharing bool,
) ServiceInterface {
	return &service{
		logger: *log.GetLogger().With(
			log.String(log.LoggerKeyComponentName, loggerComponentName)),
		store:                        store,
		declarativeStore:             declarativeStore,
		registry:                     newRegistry(),
		ouHierarchyResolver:          ouHierarchyResolver,
		ouEnumerator:                 ouEnumerator,
		transactioner:                transactioner,
		visibilityCache:              visibilityCache,
		overlayRuleCache:             overlayRuleCache,
		allowChildOUCrossTreeSharing: allowChildOUCrossTreeSharing,
	}
}

// RegisterResourceType adds a resource type's declaration to the framework.
func (s *service) RegisterResourceType(decl ResourceTypeDeclaration) {
	s.registry.register(decl)
}

// CreatePolicy records one organization unit's decision about one resource.
func (s *service) CreatePolicy(
	ctx context.Context, rt ResourceType, resourceID, owningOUID string, req PolicyRequest,
) (Policy, *tidcommon.ServiceError) {
	return s.createPolicy(ctx, rt, resourceID, owningOUID, req, false)
}

// CreateDeclarativePolicy records a policy a resource file declares.
func (s *service) CreateDeclarativePolicy(
	ctx context.Context, rt ResourceType, resourceID, owningOUID string, req PolicyRequest,
) (Policy, *tidcommon.ServiceError) {
	if s.declarativeStore == nil {
		s.logger.Error(ctx, "Declarative policy declared while declarative resources are disabled",
			log.String("resourceType", string(rt)), log.String("resourceID", resourceID))
		return Policy{}, &tidcommon.InternalServerError
	}
	return s.createPolicy(ctx, rt, resourceID, owningOUID, req, true)
}

// createPolicy is the shared body behind CreatePolicy and CreateDeclarativePolicy. A declared
// policy is seeded into the in-memory store instead of the database, which is the only step the
// two do differently.
func (s *service) createPolicy(
	ctx context.Context, rt ResourceType, resourceID, owningOUID string, req PolicyRequest, declared bool,
) (Policy, *tidcommon.ServiceError) {
	if _, ok := s.registry.get(rt); !ok {
		return Policy{}, &ErrorResourceTypeNotRegistered
	}
	if resourceID == "" || owningOUID == "" {
		return Policy{}, &ErrorInvalidRequestFormat
	}
	if !declared {
		if svcErr := requireDeclarativeForDeploymentWideScope(req.TargetOUScope); svcErr != nil {
			return Policy{}, svcErr
		}
	}

	initiatingOUID := req.InitiatingOUID
	if initiatingOUID == "" {
		initiatingOUID = owningOUID
	}

	if existing, err := s.findPolicy(ctx, rt, resourceID, initiatingOUID); err == nil {
		return Policy{}, withDetail(ErrorPolicyExists, "existing policy "+existing.ID)
	} else if !errors.Is(err, ErrPolicyNotFound) {
		s.logger.Error(ctx, "Failed to check for an existing policy", log.Error(err))
		return Policy{}, &tidcommon.InternalServerError
	}

	policy, svcErr := s.buildPolicy(ctx, rt, resourceID, owningOUID, initiatingOUID, req, declared)
	if svcErr != nil {
		return Policy{}, svcErr
	}

	if declared {
		s.declarativeStore.seed(policy)
		s.clearCaches(ctx)
		return policy, nil
	}

	if err := s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		return s.store.CreatePolicy(txCtx, policy)
	}); err != nil {
		// The check above and this insert are not atomic, so a concurrent create can take the one
		// slot between them and leave this one rejected by the uniqueness constraint. Re-reading
		// names the policy that won, which is the one to edit, rather than reporting a bare failure.
		// Asking the database again is also what keeps this free of driver-specific error parsing.
		if existing, lookupErr := s.findPolicy(ctx, rt, resourceID, initiatingOUID); lookupErr == nil {
			return Policy{}, withDetail(ErrorPolicyExists, "existing policy "+existing.ID)
		}
		s.logger.Error(ctx, "Failed to create sharing policy", log.Error(err))
		return Policy{}, &tidcommon.InternalServerError
	}
	s.clearCaches(ctx)
	return policy, nil
}

// requireDeclarativeForDeploymentWideScope refuses the two scopes whose reach is not bounded by the
// initiator's own position in the tree, unless the policy comes from a resource file.
func requireDeclarativeForDeploymentWideScope(scope TargetOUScope) *tidcommon.ServiceError {
	switch {
	case scope.AllOUs:
		return withDetail(ErrorDeploymentWideScopeDeclarativeOnly, "allOus")
	case scope.AllRoots:
		return withDetail(ErrorDeploymentWideScopeDeclarativeOnly, "allRoots")
	default:
		return nil
	}
}

// buildPolicy validates a request and materializes its rules against what the initiator holds.
func (s *service) buildPolicy(
	ctx context.Context, rt ResourceType, resourceID, owningOUID, initiatingOUID string,
	req PolicyRequest, declared bool,
) (Policy, *tidcommon.ServiceError) {
	stage := StageShare
	parentPolicyID := ""
	if initiatingOUID != owningOUID {
		stage = StageReshare
		visible, covering, svcErr := s.resolveVisibility(ctx, rt, resourceID, owningOUID, initiatingOUID)
		if svcErr != nil {
			return Policy{}, svcErr
		}
		if !visible {
			return Policy{}, withDetail(ErrorNotShared, initiatingOUID)
		}
		// Only a frontier unit may reshare: one the covering policy named and stopped at. Where
		// that policy already reaches below the initiator, its descendants hold the resource on
		// the granter's terms and there is nothing here left to grant.
		if svcErr := requireFrontierUnit(covering); svcErr != nil {
			return Policy{}, svcErr
		}
		parentPolicyID = canonicalParentID(covering)
	}

	targets, svcErr := s.buildTargets(ctx, req.TargetOUScope, initiatingOUID, owningOUID, stage)
	if svcErr != nil {
		return Policy{}, svcErr
	}

	excluded := s.collectExclusions(req.TargetOUScope)
	if svcErr := s.requireExclusionsInReach(ctx, targets, excluded); svcErr != nil {
		return Policy{}, svcErr
	}

	policy := Policy{
		ID:             newID(),
		ResourceType:   rt,
		ResourceID:     resourceID,
		OwningOUID:     owningOUID,
		InitiatingOUID: initiatingOUID,
		Stage:          stage,
		ParentPolicyID: parentPolicyID,
		Declared:       declared,
		Version:        1,
		Targets:        targets,
		ExcludedOUIDs:  excluded,
	}

	rules, svcErr := s.materializeRules(ctx, rt, resourceID, owningOUID, initiatingOUID, req, targets)
	if svcErr != nil {
		return Policy{}, svcErr
	}
	policy.Rules = rules
	return policy, nil
}

// requireEditLeavesFrontiersIntact refuses an edit that would reach past a unit which has already
// handed the resource on.
//
// Widening a target from one unit to that unit and its subtree is the case: the unit below stops
// being a frontier, and everything it granted would then be covered twice, by its own policy and
// by this one. The unit has to withdraw what it granted before the reach above it can be widened.
func (s *service) requireEditLeavesFrontiersIntact(
	ctx context.Context, proposed Policy,
) *tidcommon.ServiceError {
	policies, err := s.store.ListPoliciesForResource(ctx, proposed.ResourceType, proposed.ResourceID)
	if err != nil {
		s.logger.Error(ctx, "Failed to list policies for resource", log.Error(err))
		return &tidcommon.InternalServerError
	}

	for _, p := range policies {
		if p.ID == proposed.ID || p.InitiatingOUID == proposed.InitiatingOUID {
			continue
		}
		chain, svcErr := s.buildChain(ctx, p.InitiatingOUID)
		if svcErr != nil {
			return svcErr
		}
		if reachedByCascadingTarget(proposed.Targets, p.InitiatingOUID, chain) {
			return withDetail(ErrorReshareNotPermitted, p.InitiatingOUID+" has shared this resource on")
		}
	}
	return nil
}

// reachedByCascadingTarget reports whether a target that carries on past the unit it names reaches
// ouID, which is what would strip ouID of a frontier unit's standing.
func reachedByCascadingTarget(targets []Target, ouID string, chain []string) bool {
	cascading := make([]Target, 0, len(targets))
	for _, t := range targets {
		if cascadingScope(policyengine.TargetScope(t.Scope)) {
			cascading = append(cascading, t)
		}
	}
	return reachedByAnyTarget(cascading, ouID, chain)
}

// requireExclusionsInReach refuses an exclusion naming an organization unit none of the policy's
// own targets reach. Nothing would apply it, and accepting it would read as a withholding that
// never happened.
func (s *service) requireExclusionsInReach(
	ctx context.Context, targets []Target, excluded []string,
) *tidcommon.ServiceError {
	for _, ouID := range excluded {
		chain, svcErr := s.buildChain(ctx, ouID)
		if svcErr != nil {
			return svcErr
		}
		if !reachedByAnyTarget(targets, ouID, chain) {
			return withDetail(ErrorExclusionOutOfReach, ouID)
		}
	}
	return nil
}

// reachedByAnyTarget reports whether a target covers ouID, given the chain from its tree's root
// down to it. A cascading target reaches anything below its anchor; every other scope reaches
// only the unit it names.
func reachedByAnyTarget(targets []Target, ouID string, chain []string) bool {
	for _, t := range targets {
		switch t.Scope {
		case TargetScopeAllOUs:
			return true
		case TargetScopeAllRoots:
			if len(chain) > 0 && chain[0] == ouID {
				return true
			}
		case TargetScopeAllChildren, TargetScopeOUSubtree:
			for _, ancestor := range chain {
				if ancestor == t.OUID && ancestor != ouID {
					return true
				}
			}
			if t.Scope == TargetScopeOUSubtree && t.OUID == ouID {
				return true
			}
		case TargetScopeRoot, TargetScopeOU:
			if t.OUID == ouID {
				return true
			}
		}
	}
	return false
}

// cascadingScope reports whether a target reaches below the organization unit it names, which is
// what leaves that unit nothing of its own to grant.
func cascadingScope(scope policyengine.TargetScope) bool {
	switch scope {
	case policyengine.TargetScopeAllOUs,
		policyengine.TargetScopeAllChildren,
		policyengine.TargetScopeOUSubtree:
		return true
	}
	return false
}

// requireFrontierUnit refuses a reshare when every target covering the initiator carries on past
// it. One target that stops at the initiator is enough, since that reach is the initiator's to
// hand on.
func requireFrontierUnit(covering []policyengine.Coverage) *tidcommon.ServiceError {
	for _, c := range covering {
		for _, t := range c.Policy.Targets {
			if t.ID == c.TargetID && !cascadingScope(t.Scope) {
				return nil
			}
		}
	}
	return &ErrorReshareNotPermitted
}

// buildTargets turns a request's scope selector into stored target rows, enforcing the mode,
// stage and one-hop rules along the way.
func (s *service) buildTargets(
	ctx context.Context, scope TargetOUScope, initiatingOUID, owningOUID string, stage PolicyStage,
) ([]Target, *tidcommon.ServiceError) {
	blanket, root, children, valid := scope.Mode()
	if !valid {
		return nil, &ErrorInvalidRequestFormat
	}

	isOwner := initiatingOUID == owningOUID
	switch {
	case blanket:
		// Reaching every organization unit is the owner's call alone, and only as a first hop.
		if !isOwner || stage != StageShare {
			return nil, withDetail(ErrorInvalidTargetOU, "allOus is owner-only")
		}
		// Reaching every organization unit necessarily leaves the initiator's own tree.
		if svcErr := s.requireCrossTreeAllowed(ctx, initiatingOUID); svcErr != nil {
			return nil, svcErr
		}
		return []Target{{ID: newID(), Scope: TargetScopeAllOUs}}, nil

	case root:
		if !isOwner {
			return nil, withDetail(ErrorInvalidTargetOU, "root targeting is owner-only")
		}
		if scope.AllRoots {
			if svcErr := s.requireCrossTreeAllowed(ctx, initiatingOUID); svcErr != nil {
				return nil, svcErr
			}
			return []Target{{ID: newID(), Scope: TargetScopeAllRoots}}, nil
		}
		out := make([]Target, 0, len(scope.RootOUIDs))
		for _, ouID := range dedupeStrings(scope.RootOUIDs) {
			if svcErr := s.validateRootTarget(ctx, ouID, initiatingOUID); svcErr != nil {
				return nil, svcErr
			}
			out = append(out, Target{ID: newID(), Scope: TargetScopeRoot, OUID: ouID})
		}
		return out, nil

	case children:
		if scope.AllChildren {
			return []Target{{ID: newID(), Scope: TargetScopeAllChildren, OUID: initiatingOUID}}, nil
		}
		out := make([]Target, 0, len(scope.ChildOUIDs))
		// Naming one organization unit twice is rejected rather than deduplicated, because the two
		// entries carry their own overlay rules and there is no basis for picking a winner. Two
		// entries differing only in allChildren also satisfy the target uniqueness constraint, so
		// the database would accept both rows while per-target rules attached to one of them alone.
		seen := make(map[string]struct{}, len(scope.ChildOUIDs))
		for _, entry := range scope.ChildOUIDs {
			if entry.OUID == "" {
				return nil, &ErrorInvalidRequestFormat
			}
			if _, repeated := seen[entry.OUID]; repeated {
				return nil, withDetail(ErrorInvalidTargetOU, entry.OUID+" is named more than once")
			}
			seen[entry.OUID] = struct{}{}
			if svcErr := s.validateChildTarget(ctx, entry.OUID, initiatingOUID); svcErr != nil {
				return nil, svcErr
			}
			targetScope := TargetScopeOU
			if entry.AllChildren {
				targetScope = TargetScopeOUSubtree
			}
			out = append(out, Target{ID: newID(), Scope: targetScope, OUID: entry.OUID})
		}
		return out, nil
	}
	return nil, &ErrorInvalidRequestFormat
}

// validateChildTarget enforces the one-hop rule: a policy may only name organization units directly
// beneath its initiator, so every extra level of reach is a fresh decision by the unit that holds
// it rather than something an ancestor can grant over its descendants' heads.
func (s *service) validateChildTarget(
	ctx context.Context, targetOUID, initiatingOUID string,
) *tidcommon.ServiceError {
	if targetOUID == initiatingOUID {
		return withDetail(ErrorInvalidTargetOU, targetOUID+" cannot target itself")
	}
	ancestors, svcErr := s.ouHierarchyResolver.GetAncestorOUIDs(ctx, targetOUID)
	if svcErr != nil {
		return svcErr
	}
	if len(ancestors) == 0 || ancestors[0] != initiatingOUID {
		return withDetail(ErrorInvalidTargetOU, targetOUID+" is not a direct child of "+initiatingOUID)
	}
	return nil
}

// validateRootTarget checks a named root target really is a root, and that reaching it does not
// leave the initiator's own tree unless that is allowed.
func (s *service) validateRootTarget(
	ctx context.Context, targetOUID, initiatingOUID string,
) *tidcommon.ServiceError {
	ancestors, svcErr := s.ouHierarchyResolver.GetAncestorOUIDs(ctx, targetOUID)
	if svcErr != nil {
		return svcErr
	}
	if len(ancestors) > 0 {
		return withDetail(ErrorInvalidTargetOU, targetOUID+" is not a root organization unit")
	}

	initiatorRoot, svcErr := s.treeRootOf(ctx, initiatingOUID)
	if svcErr != nil {
		return svcErr
	}
	if targetOUID == initiatorRoot {
		return nil
	}
	return s.requireCrossTreeAllowed(ctx, initiatingOUID)
}

// requireCrossTreeAllowed refuses a target outside the initiator's own tree when the initiator sits
// inside one. A root organization unit reaching other roots is the ordinary business-to-business
// case; a unit within a tree doing the same is what the configuration gates.
func (s *service) requireCrossTreeAllowed(
	ctx context.Context, initiatingOUID string,
) *tidcommon.ServiceError {
	if s.allowChildOUCrossTreeSharing {
		return nil
	}
	ancestors, svcErr := s.ouHierarchyResolver.GetAncestorOUIDs(ctx, initiatingOUID)
	if svcErr != nil {
		return svcErr
	}
	if len(ancestors) > 0 {
		return &ErrorCrossTreeShareRestricted
	}
	return nil
}

// treeRootOf returns the root of the tree an organization unit belongs to, which is the unit itself
// when it is already a root.
func (s *service) treeRootOf(ctx context.Context, ouID string) (string, *tidcommon.ServiceError) {
	ancestors, svcErr := s.ouHierarchyResolver.GetAncestorOUIDs(ctx, ouID)
	if svcErr != nil {
		return "", svcErr
	}
	if len(ancestors) == 0 {
		return ouID, nil
	}
	return ancestors[len(ancestors)-1], nil
}

// collectExclusions merges the two exclusion lists; which one a caller used depends on the mode,
// and they carve out the same thing.
func (s *service) collectExclusions(scope TargetOUScope) []string {
	out := append([]string{}, scope.ExcludedOUIDs...)
	out = append(out, scope.ExcludedRootOUIDs...)
	return dedupeStrings(out)
}

// materializeRules folds every requested rule against what the initiating organization unit itself
// holds, and stores the result. Resolving at write means a read is one lookup rather than a walk.
func (s *service) materializeRules(
	ctx context.Context, rt ResourceType, resourceID, owningOUID, initiatingOUID string,
	req PolicyRequest, targets []Target,
) ([]StoredRule, *tidcommon.ServiceError) {
	initiatorRules, svcErr := s.initiatorEffectiveRules(ctx, rt, resourceID, owningOUID, initiatingOUID)
	if svcErr != nil {
		return nil, svcErr
	}

	out := make([]StoredRule, 0, len(req.OverlayRules))
	for fieldKey, requested := range req.OverlayRules {
		stored, svcErr := s.narrowOne(ctx, rt, resourceID, fieldKey, "", initiatingOUID, initiatorRules, requested)
		if svcErr != nil {
			return nil, svcErr
		}
		out = append(out, stored)
	}

	// A per-target rule overrides the policy-level one for that target alone, and is bounded by
	// the same ceiling rather than by the policy-level rule it replaces.
	byOU := make(map[string]string, len(targets))
	for _, t := range targets {
		byOU[t.OUID] = t.ID
	}
	for _, entry := range req.TargetOUScope.ChildOUIDs {
		for fieldKey, requested := range entry.OverlayRules {
			stored, svcErr := s.narrowOne(
				ctx, rt, resourceID, fieldKey, byOU[entry.OUID], initiatingOUID, initiatorRules, requested)
			if svcErr != nil {
				return nil, svcErr
			}
			out = append(out, stored)
		}
	}
	return out, nil
}

// containmentFor builds the set semantics for one field, asking the resource type for the
// separator when the field is hierarchical.
//
// A hierarchy compared with no delimiter is raw string prefixing, under which "billing" covers
// "billingx", so a type declaring a hierarchical field is expected to resolve one.
func (s *service) containmentFor(
	ctx context.Context, rt ResourceType, resourceID, fieldKey string, kind FieldKind,
) (policyengine.Containment, *tidcommon.ServiceError) {
	delimiter := ""
	if kind == FieldHierarchy {
		decl, ok := s.registry.get(rt)
		if !ok {
			return policyengine.Containment{}, &ErrorResourceTypeNotRegistered
		}
		resolver, ok := decl.(FieldDelimiterResolver)
		if !ok {
			s.logger.Error(ctx, "Resource type declares a hierarchical field but resolves no delimiter",
				log.String("resourceType", string(rt)), log.String("fieldKey", fieldKey))
			return policyengine.Containment{}, &tidcommon.InternalServerError
		}
		got, svcErr := resolver.FieldDelimiter(ctx, resourceID, fieldKey)
		if svcErr != nil {
			return policyengine.Containment{}, svcErr
		}
		if got == "" {
			s.logger.Error(ctx, "Resource type resolved an empty delimiter for a hierarchical field",
				log.String("resourceType", string(rt)), log.String("resourceID", resourceID))
			return policyengine.Containment{}, &tidcommon.InternalServerError
		}
		delimiter = got
	}
	return policyengine.NewContainment(policyengine.FieldKind(kind), delimiter), nil
}

// narrowOne folds one requested rule and reports the offending field on rejection.
func (s *service) narrowOne(
	ctx context.Context, rt ResourceType, resourceID, fieldKey, targetID, initiatingOUID string,
	initiatorRules map[string]OverlayRule, requested OverlayRule,
) (StoredRule, *tidcommon.ServiceError) {
	decl, ok := s.registry.field(rt, fieldKey)
	if !ok {
		return StoredRule{}, withDetail(ErrorUnknownFieldKey, fieldKey)
	}
	if svcErr := s.validateMembers(ctx, rt, resourceID, fieldKey, initiatingOUID, requested); svcErr != nil {
		return StoredRule{}, svcErr
	}

	c, svcErr := s.containmentFor(ctx, rt, resourceID, fieldKey, decl.Kind)
	if svcErr != nil {
		return StoredRule{}, svcErr
	}
	parent := s.effectiveOrDefault(rt, fieldKey, initiatorRules)

	resolved, err := c.Narrow(toEngineRule(parent), toEngineRule(requested))
	if err != nil {
		return StoredRule{}, mapEngineError(err, fieldKey)
	}

	return StoredRule{
		FieldKey:  fieldKey,
		TargetID:  targetID,
		Resolved:  fromEngineRule(resolved),
		Requested: requested,
	}, nil
}

// validateMembers asks the resource type whether the initiator may name these members, because
// nothing in the narrowing algebra stops an organization unit pinning an id belonging to another.
func (s *service) validateMembers(
	ctx context.Context, rt ResourceType, resourceID, fieldKey, initiatingOUID string, r OverlayRule,
) *tidcommon.ServiceError {
	decl, ok := s.registry.get(rt)
	if !ok {
		return nil
	}
	validator, ok := decl.(MemberValidator)
	if !ok {
		return nil
	}
	var members []string
	for _, set := range []*[]string{r.Value, r.AllowedValues, r.ExcludedValues} {
		if set != nil {
			members = append(members, *set...)
		}
	}
	if len(members) == 0 {
		return nil
	}
	err := validator.ValidateMembers(ctx, resourceID, fieldKey, initiatingOUID, dedupeStrings(members))
	if err != nil {
		return withDetail(ErrorMemberNotVisible, fieldKey)
	}
	return nil
}

// effectiveOrDefault returns the initiator's own rule for a field, falling back to the coarser
// field it declares and then to the type's declared default.
func (s *service) effectiveOrDefault(
	rt ResourceType, fieldKey string, initiatorRules map[string]OverlayRule,
) OverlayRule {
	if r, ok := initiatorRules[fieldKey]; ok {
		return r
	}
	if fallback, ok := s.registry.fallbackKey(rt, fieldKey); ok {
		if r, ok := initiatorRules[fallback]; ok {
			return r
		}
	}
	if d, ok := s.registry.defaultRule(rt, fieldKey); ok {
		return d
	}
	if fallback, ok := s.registry.fallbackKey(rt, fieldKey); ok {
		if d, ok := s.registry.defaultRule(rt, fallback); ok {
			return d
		}
	}
	return OverlayRule{}
}

// initiatorEffectiveRules resolves what the initiating organization unit itself holds, through the
// identical resolver reads use. Folding against anything narrower would let an initiator launder a
// wide rule through a second policy it also holds.
func (s *service) initiatorEffectiveRules(
	ctx context.Context, rt ResourceType, resourceID, owningOUID, initiatingOUID string,
) (map[string]OverlayRule, *tidcommon.ServiceError) {
	if initiatingOUID == owningOUID {
		return map[string]OverlayRule{}, nil
	}
	// Uncached on purpose: this runs inside the write transaction.
	resolved, svcErr := s.resolveOverlayRules(ctx, rt, resourceID, initiatingOUID)
	if svcErr != nil {
		return nil, svcErr
	}
	return resolved.Rules, nil
}

// UpdatePolicy replaces a policy's contents within the limits of its scope family.
func (s *service) UpdatePolicy(
	ctx context.Context, policyID string, req PolicyRequest,
) (Policy, *tidcommon.ServiceError) {
	current, svcErr := s.GetPolicy(ctx, policyID)
	if svcErr != nil {
		return Policy{}, svcErr
	}

	proposed, svcErr := s.buildPolicy(ctx, current.ResourceType, current.ResourceID,
		current.OwningOUID, current.InitiatingOUID, req, false)
	if svcErr != nil {
		return Policy{}, svcErr
	}
	proposed.ID = current.ID
	proposed.Version = current.Version

	if err := policyengine.ValidateEdit(
		toEnginePolicy(current), toEnginePolicy(proposed), req.Version, current.Version,
	); err != nil {
		return Policy{}, mapEditError(err)
	}
	if svcErr := s.blanketRulesNarrowOnly(ctx, current, proposed); svcErr != nil {
		return Policy{}, svcErr
	}
	if svcErr := s.requireEditLeavesFrontiersIntact(ctx, proposed); svcErr != nil {
		return Policy{}, svcErr
	}

	// Editing a declared policy writes it to the database for the first time, under the same id so
	// the policy a caller was looking at keeps its identity. The file is left untouched, and stays
	// the value the policy reverts to if the stored row is later deleted.
	materializing := current.Declared
	if materializing {
		proposed.Version = current.Version + 1
	}

	if err := s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		if materializing {
			if err := s.store.CreatePolicy(txCtx, proposed); err != nil {
				return err
			}
		} else if err := s.store.ReplacePolicyContents(txCtx, proposed, current.Version); err != nil {
			return err
		}
		// Rules stored beneath this policy were clamped against a ceiling that may just have
		// moved, so they are recomputed in the same transaction rather than left stale.
		if err := s.rematerializeDependents(txCtx, current); err != nil {
			return err
		}
		// An edit narrows reach as well as terms. A new exclusion on a blanket policy, or a target
		// this policy no longer names, takes the resource away from units that had it, and that
		// needs exactly the cleanup a delete needs. current is passed because it is the reach the
		// edit moved away from.
		return s.cleanUpLostVisibility(txCtx, current)
	}); err != nil {
		if errors.Is(err, ErrPolicyNotFound) {
			return Policy{}, &ErrorVersionMismatch
		}
		s.logger.Error(ctx, "Failed to update sharing policy", log.Error(err))
		return Policy{}, &tidcommon.InternalServerError
	}

	s.clearCaches(ctx)
	if !materializing {
		proposed.Version = current.Version + 1
	}
	return proposed, nil
}

// canonicalParentID records the covering policy as a reshare's parent. Only a frontier unit may
// reshare, so there is one such policy, and the parent is a real dependency rather than a hint:
// if it goes, its issuer stops seeing the resource and every policy that issuer wrote is dead.
// Declared policies are skipped: they have no stored row to reference, and are replayed from their
// resource file rather than through the stored path.
func canonicalParentID(covering []policyengine.Coverage) string {
	out := ""
	for _, c := range covering {
		if c.Policy.Declared {
			continue
		}
		if out == "" || c.Policy.ID < out {
			out = c.Policy.ID
		}
	}
	return out
}

// ruleKey identifies one stored rule: a field, optionally scoped to a single target.
type ruleKey struct {
	fieldKey string
	targetID string
}

// blanketRulesNarrowOnly reports whether an edit keeps every restriction a blanket policy's rules
// currently carry.
//
// A blanket policy already reaches everything in its family, so there is nothing to expand toward
// and the only edit open to it is a narrowing one. The policy engine holds its targets to that; its
// rules are held to it here, because the engine's policy carries no rules to compare.
//
// Dropping a rule counts as widening rather than as leaving the field alone. A field no policy
// names falls back to the resource type's declared default, which is the value the rule was put
// there to override, so omitting it hands back whatever the default allows.
func (s *service) blanketRulesNarrowOnly(
	ctx context.Context, current, proposed Policy,
) *tidcommon.ServiceError {
	if policyengine.PolicyFamily(toEnginePolicy(current)) != policyengine.FamilyBlanket {
		return nil
	}

	next := make(map[ruleKey]OverlayRule, len(proposed.Rules))
	for _, r := range proposed.Rules {
		next[ruleKey{r.FieldKey, r.TargetID}] = r.Resolved
	}

	for _, stored := range current.Rules {
		proposedRule, kept := next[ruleKey{stored.FieldKey, stored.TargetID}]
		if !kept {
			return withDetail(ErrorBlanketNarrowOnly, stored.FieldKey)
		}
		decl, ok := s.registry.field(current.ResourceType, stored.FieldKey)
		if !ok {
			return withDetail(ErrorUnknownFieldKey, stored.FieldKey)
		}
		c, svcErr := s.containmentFor(ctx, current.ResourceType, current.ResourceID,
			stored.FieldKey, decl.Kind)
		if svcErr != nil {
			return svcErr
		}
		if c.Widens(toEngineRule(stored.Resolved), toEngineRule(proposedRule)) {
			return withDetail(ErrorBlanketNarrowOnly, stored.FieldKey)
		}
	}
	return nil
}

// rematerializeDependents recomputes every reshare whose ceiling the edited policy may have moved.
//
// Walking the stored parent links would now reach the same set, since a reshare has exactly one
// covering policy. Every reshare of the resource is recomputed anyway: it is the simpler statement,
// and it still holds for a frontier unit whose covering policy was declarative and so left no
// parent to walk from. The order is shallowest initiator first, since a covering policy's initiator
// always sits above the unit it covers, so each ceiling is rebuilt before the rules clamped to it.
func (s *service) rematerializeDependents(ctx context.Context, edited Policy) error {
	policies, err := s.store.ListPoliciesForResource(ctx, edited.ResourceType, edited.ResourceID)
	if err != nil {
		return err
	}

	dependents := make([]Policy, 0, len(policies))
	for _, p := range policies {
		if p.ID == edited.ID || p.Declared || p.Stage != StageReshare {
			continue
		}
		dependents = append(dependents, p)
	}

	ordered, svcErr := s.orderByInitiatorDepth(ctx, dependents)
	if svcErr != nil {
		return fmt.Errorf("failed to order policies for re-materialization: %s", svcErr.Code)
	}

	for _, descendant := range ordered {
		initiatorRules, svcErr := s.initiatorEffectiveRules(ctx, descendant.ResourceType,
			descendant.ResourceID, descendant.OwningOUID, descendant.InitiatingOUID)
		if svcErr != nil {
			return fmt.Errorf("failed to resolve rules for policy %s: %s", descendant.ID, svcErr.Code)
		}

		rebuilt := make([]StoredRule, 0, len(descendant.Rules))
		for _, stored := range descendant.Rules {
			// Re-narrowing starts from the original request, so an ancestor that later widens
			// again restores what it had clamped rather than ratcheting permanently downward.
			next, svcErr := s.narrowOne(ctx, descendant.ResourceType, descendant.ResourceID,
				stored.FieldKey, stored.TargetID, descendant.InitiatingOUID, initiatorRules,
				stored.Requested)
			if svcErr != nil {
				return fmt.Errorf("failed to re-materialize policy %s: %s", descendant.ID, svcErr.Code)
			}
			rebuilt = append(rebuilt, next)
		}
		// Recomputing every reshare means most of them are unchanged, and rewriting those would bump
		// their version and fail a concurrent edit of a policy this one never touched.
		if len(descendant.Rules) == 0 && len(rebuilt) == 0 {
			continue
		}
		if reflect.DeepEqual(descendant.Rules, rebuilt) {
			continue
		}
		descendant.Rules = rebuilt
		if err := s.store.ReplacePolicyContents(ctx, descendant, descendant.Version); err != nil {
			return err
		}
	}
	return nil
}

// orderByInitiatorDepth sorts policies so one whose initiator sits above another always comes first,
// breaking ties by id so the order is stable across runs.
func (s *service) orderByInitiatorDepth(
	ctx context.Context, policies []Policy,
) ([]Policy, *tidcommon.ServiceError) {
	depth := make(map[string]int, len(policies))
	for _, p := range policies {
		if _, done := depth[p.InitiatingOUID]; done {
			continue
		}
		ancestors, svcErr := s.ouHierarchyResolver.GetAncestorOUIDs(ctx, p.InitiatingOUID)
		if svcErr != nil {
			return nil, svcErr
		}
		depth[p.InitiatingOUID] = len(ancestors)
	}

	out := append([]Policy{}, policies...)
	sort.SliceStable(out, func(i, j int) bool {
		if depth[out[i].InitiatingOUID] != depth[out[j].InitiatingOUID] {
			return depth[out[i].InitiatingOUID] < depth[out[j].InitiatingOUID]
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// DeletePolicy removes a policy and cleans up the state of every organization unit that loses
// visibility because of it.
func (s *service) DeletePolicy(ctx context.Context, policyID string) *tidcommon.ServiceError {
	policy, svcErr := s.GetPolicy(ctx, policyID)
	if svcErr != nil {
		return svcErr
	}
	if policy.Declared {
		return &ErrorPolicyDeclared
	}

	if err := s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		if err := s.store.DeletePolicy(txCtx, policyID); err != nil {
			return err
		}
		return s.cleanUpLostVisibility(txCtx, policy)
	}); err != nil {
		s.logger.Error(ctx, "Failed to delete sharing policy", log.Error(err))
		return &tidcommon.InternalServerError
	}

	s.clearCaches(ctx)
	return nil
}

// cleanUpLostVisibility clears the per-organization unit state of every unit that stopped being
// able to see a resource because of one policy change.
//
// changed is the policy as it stood before the change: the deleted policy, or the pre-edit form of
// an edited one. Its old reach is where the search starts, because those are the units whose
// visibility the change could have taken away. Whether a unit actually lost it is decided by
// resolving visibility again afterwards, not by reading the old target list, so a unit some other
// policy still covers keeps everything it had.
//
// Loss cascades. A unit that goes dark can no longer hold up the policies it issued, so whatever
// those reached has to be re-checked in turn, and so on until a pass turns up nothing new.
func (s *service) cleanUpLostVisibility(ctx context.Context, changed Policy) error {
	policies, err := s.store.ListPoliciesForResource(ctx, changed.ResourceType, changed.ResourceID)
	if err != nil {
		return err
	}
	issuedBy := make(map[string][]Policy, len(policies))
	for _, p := range policies {
		issuedBy[p.InitiatingOUID] = append(issuedBy[p.InitiatingOUID], p)
	}

	queue, err := s.candidatesFrom(ctx, changed)
	if err != nil {
		return err
	}

	checked := make(map[string]struct{}, len(queue))
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		if _, done := checked[c.ouID]; done {
			continue
		}
		checked[c.ouID] = struct{}{}

		stillVisible, _, svcErr := s.resolveVisibility(
			ctx, changed.ResourceType, changed.ResourceID, changed.OwningOUID, c.ouID)
		if svcErr != nil {
			return fmt.Errorf("failed to resolve visibility for %s: %s", c.ouID, svcErr.Code)
		}
		if stillVisible {
			continue
		}

		if err := s.clearOverlayValues(ctx, changed.ResourceType, changed.ResourceID, c.ouID); err != nil {
			return err
		}
		if err := s.notifyVisibilityLost(
			ctx, changed.ResourceType, changed.ResourceID, c.ouID, c.fields); err != nil {
			return err
		}

		// Nothing this unit shared onward can stand now that the unit itself cannot see the
		// resource, so everything those policies reached joins the queue.
		for _, p := range issuedBy[c.ouID] {
			downstream, err := s.candidatesFrom(ctx, p)
			if err != nil {
				return err
			}
			queue = append(queue, downstream...)
		}
	}
	return nil
}

// lostCandidate is one organization unit worth re-checking, with the fields the policy that reached
// it governed, which is what the resource type's hook is told about.
type lostCandidate struct {
	ouID   string
	fields []string
}

// candidatesFrom expands one policy's targets into the units it reached, paired with the fields it
// governed.
func (s *service) candidatesFrom(ctx context.Context, p Policy) ([]lostCandidate, error) {
	reached, err := s.ousReachedBy(ctx, p)
	if err != nil {
		return nil, err
	}
	fields := make([]string, 0, len(p.Rules))
	for _, r := range p.Rules {
		fields = append(fields, r.FieldKey)
	}
	fields = dedupeStrings(fields)

	out := make([]lostCandidate, 0, len(reached))
	for _, ouID := range reached {
		out = append(out, lostCandidate{ouID: ouID, fields: fields})
	}
	return out, nil
}

// clearOverlayValues drops the values a unit chose for a resource it can no longer see.
//
// A resource type that keeps those values in its own storage rather than the framework's implements
// OverlayCleaner and is called instead, since the framework's table holds nothing for it to delete.
func (s *service) clearOverlayValues(
	ctx context.Context, rt ResourceType, resourceID, ouID string,
) error {
	if decl, ok := s.registry.get(rt); ok {
		if cleaner, ok := decl.(OverlayCleaner); ok {
			if err := cleaner.DeleteOverlayValues(ctx, resourceID, ouID); err != nil {
				return fmt.Errorf("failed to clear overlay values for %s: %w", ouID, err)
			}
			return nil
		}
	}
	if err := s.store.DeleteOverlayValuesForOU(ctx, rt, resourceID, ouID); err != nil {
		return fmt.Errorf("failed to clear overlay values for %s: %w", ouID, err)
	}
	return nil
}

// notifyVisibilityLost tells a resource type to clean up its own state for one unit, if it asked to
// be told. Field-scoped, unlike the overlay values: a policy governing one field has no business
// deleting the state another field's policy put there.
func (s *service) notifyVisibilityLost(
	ctx context.Context, rt ResourceType, resourceID, ouID string, fields []string,
) error {
	decl, ok := s.registry.get(rt)
	if !ok {
		return nil
	}
	hooks, ok := decl.(PolicyHooks)
	if !ok {
		return nil
	}
	return hooks.OnVisibilityLost(ctx, resourceID, ouID, fields)
}

// ousReachedBy enumerates every organization unit a policy's targets could have reached.
//
// A target records an anchor rather than the set it covers, so the units that may have just lost
// visibility are almost never the ones named on the targets: a subtree or root target reaches
// everything below its anchor, an all-children target reaches everything below its initiator but
// not the initiator itself, and the blanket scopes reach the deployment.
func (s *service) ousReachedBy(ctx context.Context, p Policy) ([]string, error) {
	var out []string

	appendSubtree := func(anchor string, includeAnchor bool) error {
		if includeAnchor {
			out = append(out, anchor)
		}
		if s.ouEnumerator == nil {
			return nil
		}
		descendants, svcErr := s.ouEnumerator.DescendantOUIDs(ctx, anchor)
		if svcErr != nil {
			return fmt.Errorf("failed to enumerate organization units beneath %s: %s", anchor, svcErr.Code)
		}
		out = append(out, descendants...)
		return nil
	}

	if s.ouEnumerator == nil {
		// Without downward traversal only the named anchors can be cleaned up, which leaves state
		// behind for everything under them. Loud, because it is a silent data-retention gap.
		s.logger.Error(ctx, "Organization unit resolver cannot enumerate descendants; "+
			"per-organization-unit state beneath a removed policy's targets will not be cleaned up",
			log.String("policyID", p.ID))
	}

	for _, t := range p.Targets {
		switch t.Scope {
		case TargetScopeAllOUs, TargetScopeAllRoots:
			// Losing a root cuts off everything below it, so the whole deployment is in scope.
			if s.ouEnumerator == nil {
				continue
			}
			all, svcErr := s.ouEnumerator.AllOUIDs(ctx)
			if svcErr != nil {
				return nil, fmt.Errorf("failed to enumerate organization units: %s", svcErr.Code)
			}
			out = append(out, all...)
		case TargetScopeAllChildren:
			// The anchor is the initiator, which keeps its own visibility; only what lies beneath
			// it was reached by this policy.
			if err := appendSubtree(t.OUID, false); err != nil {
				return nil, err
			}
		case TargetScopeOUSubtree, TargetScopeRoot:
			if err := appendSubtree(t.OUID, true); err != nil {
				return nil, err
			}
		case TargetScopeOU:
			out = append(out, t.OUID)
		}
	}

	return dedupeStrings(out), nil
}

// GetPolicy returns one policy by id.
func (s *service) GetPolicy(ctx context.Context, policyID string) (Policy, *tidcommon.ServiceError) {
	p, err := s.store.GetPolicy(ctx, policyID)
	if err == nil {
		return p, nil
	}
	if !errors.Is(err, ErrPolicyNotFound) {
		s.logger.Error(ctx, "Failed to get sharing policy", log.Error(err))
		return Policy{}, &tidcommon.InternalServerError
	}
	// Not stored: either never edited, or not a policy at all.
	if s.declarativeStore != nil {
		if declared, ok := s.declarativeStore.get(policyID); ok {
			return declared, nil
		}
	}
	return Policy{}, &ErrorPolicyNotFound
}

// ListPolicies returns every policy recorded for a resource, declared ones included.
func (s *service) ListPolicies(
	ctx context.Context, rt ResourceType, resourceID string,
) ([]Policy, *tidcommon.ServiceError) {
	stored, err := s.store.ListPoliciesForResource(ctx, rt, resourceID)
	if err != nil {
		s.logger.Error(ctx, "Failed to list sharing policies", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if s.declarativeStore != nil {
		// A declared policy whose organization unit already has a stored one has been edited, and
		// the stored row is what applies; listing both would report two policies for one unit.
		superseded := make(map[string]struct{}, len(stored))
		for _, p := range stored {
			superseded[p.InitiatingOUID] = struct{}{}
		}
		for _, declared := range s.declarativeStore.listForResource(rt, resourceID) {
			if _, edited := superseded[declared.InitiatingOUID]; !edited {
				stored = append(stored, declared)
			}
		}
	}
	return stored, nil
}

// ExportPolicies returns a resource's policies in an order safe to replay sequentially: the policy
// that made an initiator visible always precedes the policy that initiator issued.
func (s *service) ExportPolicies(
	ctx context.Context, rt ResourceType, resourceID string,
) ([]ReplayablePolicy, *tidcommon.ServiceError) {
	policies, svcErr := s.ListPolicies(ctx, rt, resourceID)
	if svcErr != nil {
		return nil, svcErr
	}

	// Policies are queued per id rather than stored one-per-id, so two entries sharing an id both
	// reach the export instead of one silently replacing the other.
	queued := make(map[string][]Policy, len(policies))
	lineages := make([]policyengine.Lineage, 0, len(policies))
	for _, p := range policies {
		queued[p.ID] = append(queued[p.ID], p)
		lineages = append(lineages, policyengine.Lineage{ID: p.ID, ParentID: p.ParentPolicyID})
	}

	ordered := policyengine.OrderByDependency(lineages)
	out := make([]ReplayablePolicy, 0, len(ordered))
	for _, l := range ordered {
		pending := queued[l.ID]
		if len(pending) == 0 {
			continue
		}
		queued[l.ID] = pending[1:]
		p := pending[0]
		out = append(out, ReplayablePolicy{InitiatingOUID: p.InitiatingOUID, Request: requestFromPolicy(p)})
	}
	return out, nil
}

// IsVisible reports whether an organization unit can see a resource, as its owner or as a sharee.
func (s *service) IsVisible(
	ctx context.Context, rt ResourceType, resourceID, ouID string,
) (bool, *tidcommon.ServiceError) {
	// The owner sees its own resource whether or not a policy exists. Answering that from the
	// policies alone would make a resource nobody has shared yet invisible to the unit that owns it.
	//
	// Asked before the cache is read, not merely left uncached. ownerOf reports the owner as
	// unknown while the resource type resolves no owner, and the policy-derived false that follows
	// is cached under the owner's own key. Reading the cache first would keep serving that false
	// once ownership became resolvable, because only a policy write ever clears this cache.
	owner, known, svcErr := s.ownerOf(ctx, rt, resourceID)
	if svcErr != nil {
		return false, svcErr
	}
	if known && owner == ouID {
		return true, nil
	}

	key := cache.CacheKey{Key: fmt.Sprintf("%s:%s:%s", rt, resourceID, ouID)}
	if s.visibilityCache != nil {
		if cached, ok := s.visibilityCache.Get(ctx, key); ok {
			return cached, nil
		}
	}

	chain, svcErr := s.buildChain(ctx, ouID)
	if svcErr != nil {
		return false, svcErr
	}
	policies, svcErr := s.relevantPolicies(ctx, rt, resourceID, chain)
	if svcErr != nil {
		return false, svcErr
	}

	visible, _ := policyengine.EvaluateChain(chain, toEnginePolicies(policies))
	if s.visibilityCache != nil {
		if err := s.visibilityCache.Set(ctx, key, visible); err != nil {
			s.logger.Warn(ctx, "Failed to cache visibility", log.Error(err))
		}
	}
	return visible, nil
}

// ownerOf asks a resource type which organization unit owns one of its resources.
//
// The framework stores no resource rows, so ownership can only come from the type itself. A type
// that does not answer reports known false, and the caller falls back to what the policies imply.
func (s *service) ownerOf(
	ctx context.Context, rt ResourceType, resourceID string,
) (ouID string, known bool, svcErr *tidcommon.ServiceError) {
	decl, ok := s.registry.get(rt)
	if !ok {
		return "", false, nil
	}
	resolver, ok := decl.(OwnerResolver)
	if !ok {
		return "", false, nil
	}
	owner, svcErr := resolver.OwningOUID(ctx, resourceID)
	if svcErr != nil {
		return "", false, svcErr
	}
	return owner, owner != "", nil
}

// ResolveOverlayRules returns the effective rule per field for one organization unit.
func (s *service) ResolveOverlayRules(
	ctx context.Context, rt ResourceType, resourceID, ouID string,
) (ResolvedOverlay, *tidcommon.ServiceError) {
	// Ownership is resolved before the cache is consulted, for the same reason IsVisible does it.
	// While the resource type resolves no owner, the denial the policies imply is computed for the
	// owner's own key and cached there, and only a policy write ever clears that cache. Reading the
	// cache first would keep handing the owner that denial once ownership became resolvable, so the
	// unit that owns the resource would be told it holds nothing.
	//
	// Only the owner's own entry can go stale this way: for anyone else the answer never depended
	// on ownership being known, so their cached entry stays correct and is still served.
	owner, known, svcErr := s.ownerOf(ctx, rt, resourceID)
	if svcErr != nil {
		return ResolvedOverlay{}, svcErr
	}
	ownerIsAsking := known && owner == ouID

	key := cache.CacheKey{Key: fmt.Sprintf("%s:%s:%s", rt, resourceID, ouID)}
	if s.overlayRuleCache != nil && !ownerIsAsking {
		if cached, ok := s.overlayRuleCache.Get(ctx, key); ok {
			return cached, nil
		}
	}

	out, svcErr := s.resolveOverlayRules(ctx, rt, resourceID, ouID)
	if svcErr != nil {
		return ResolvedOverlay{}, svcErr
	}

	if s.overlayRuleCache != nil {
		if err := s.overlayRuleCache.Set(ctx, key, out); err != nil {
			s.logger.Warn(ctx, "Failed to cache resolved overlay", log.Error(err))
		}
	}
	return out, nil
}

// resolveOverlayRules is the resolution itself, with no cache on either side of it.
//
// Writes go through this rather than through the cached entry point above: a hit inside a write
// would apply the ceiling as it stood before the edit, and a miss would populate the cache from
// uncommitted state that outlives a rollback, because the caches are only cleared after a commit.
func (s *service) resolveOverlayRules(
	ctx context.Context, rt ResourceType, resourceID, ouID string,
) (ResolvedOverlay, *tidcommon.ServiceError) {
	chain, svcErr := s.buildChain(ctx, ouID)
	if svcErr != nil {
		return ResolvedOverlay{}, svcErr
	}
	policies, svcErr := s.relevantPolicies(ctx, rt, resourceID, chain)
	if svcErr != nil {
		return ResolvedOverlay{}, svcErr
	}

	out := ResolvedOverlay{
		OUID:    ouID,
		Rules:   map[string]OverlayRule{},
		Sources: map[string]string{},
	}

	// Ownership is asked of the resource type rather than read off a policy, so the owner is
	// answered even for a resource nobody has shared yet.
	//
	// The fallback reads any fetched policy rather than a particular one: every policy recorded for
	// a resource carries the same owning organization unit, so the narrowed fetch's lack of an
	// ordering guarantee does not matter. It stays best-effort, because a resource type resolving
	// no owner leaves nothing else to go on.
	owner, known, svcErr := s.ownerOf(ctx, rt, resourceID)
	if svcErr != nil {
		return ResolvedOverlay{}, svcErr
	}
	switch {
	case known:
		out.Owned = owner == ouID
	case len(policies) > 0:
		out.Owned = policies[0].OwningOUID == ouID
	}

	visible, covering := policyengine.EvaluateChain(chain, toEnginePolicies(policies))
	out.Visible = visible || out.Owned

	// An organization unit that cannot see the resource is not answered with the type's defaults.
	// Those describe what an organization unit holding the resource may do when no policy says
	// otherwise, which is a different statement from "this one may do nothing".
	if !out.Visible {
		out.PolicyIDs = []string{}
		return out, nil
	}

	byID := make(map[string]Policy, len(policies))
	for _, p := range policies {
		byID[p.ID] = p
	}

	// Every declared field gets an answer, so a caller never has to distinguish "no rule" from
	// "no opinion" itself.
	decl, ok := s.registry.get(rt)
	if !ok {
		return out, nil
	}
	for _, f := range decl.Fields() {
		contributing := make([]policyengine.Rule, 0, len(covering))
		for _, c := range covering {
			p, ok := byID[c.Policy.ID]
			if !ok {
				continue
			}
			if r, found := ruleFor(p, c.TargetID, f.Key); found {
				contributing = append(contributing, toEngineRule(r))
			}
		}

		if len(contributing) == 0 {
			if out.Owned {
				// The owner is bounded by nothing it did not itself declare.
				if d, ok := s.registry.defaultRule(rt, f.Key); ok {
					out.Rules[f.Key] = d
					out.Sources[f.Key] = SourceDefault
				}
				continue
			}
			if d, ok := s.registry.defaultRule(rt, f.Key); ok {
				out.Rules[f.Key] = d
				out.Sources[f.Key] = SourceDefault
			}
			continue
		}

		c, svcErr := s.containmentFor(ctx, rt, resourceID, f.Key, f.Kind)
		if svcErr != nil {
			return ResolvedOverlay{}, svcErr
		}
		out.Rules[f.Key] = fromEngineRule(c.Intersect(contributing))
		out.Sources[f.Key] = SourcePolicy
	}

	for _, c := range covering {
		out.PolicyIDs = append(out.PolicyIDs, c.Policy.ID)
	}
	out.PolicyIDs = dedupeStrings(out.PolicyIDs)

	return out, nil
}

// ruleFor returns the rule governing one field for one target, preferring a per-target override.
func ruleFor(p Policy, targetID, fieldKey string) (OverlayRule, bool) {
	var policyLevel *OverlayRule
	for i := range p.Rules {
		r := p.Rules[i]
		if r.FieldKey != fieldKey {
			continue
		}
		if r.TargetID == targetID && targetID != "" {
			return r.Resolved, true
		}
		if r.TargetID == "" {
			policyLevel = &p.Rules[i].Resolved
		}
	}
	if policyLevel != nil {
		return *policyLevel, true
	}
	return OverlayRule{}, false
}

// relevantPolicies fetches the policies that could bear on one organization unit chain, from both
// the database and any declared in resource files.
//
// The fetch is narrowed to the chain rather than hydrating every policy the resource has. A policy
// only reaches this chain through a blanket scope or a target anchored on one of its members, and
// hydrating one policy costs a query per target, exclusion and rule, so a resource shared to many
// organization units made answering for a single one proportional to all of them. Re-materialization
// resolves rules for every dependent in turn, which turned that into quadratic work for one edit.
func (s *service) relevantPolicies(
	ctx context.Context, rt ResourceType, resourceID string, chain []string,
) ([]Policy, *tidcommon.ServiceError) {
	policies, err := s.store.ListPoliciesRelevantToChain(ctx, rt, resourceID, chain)
	if err != nil {
		s.logger.Error(ctx, "Failed to list policies relevant to an organization unit chain",
			log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	return s.withDeclared(ctx, rt, resourceID, policies, s.declaredForResource(rt, resourceID))
}

// withDeclared adds the declared policies that no stored row has superseded.
//
// Editing a declared policy writes it to the database under its own id, and from that point the
// stored row is the policy. Which ids have reached that state has to be asked of the database
// rather than inferred from the fetch that was just done: an edit that moved a policy's targets
// away from the chain being asked about leaves the stored row out of that result while the file's
// original still names the chain, and replaying the file version would hand back exactly the reach
// the edit removed.
//
// resourceID is passed straight through, so an empty one spans every resource of the type. Both
// read paths share this, because a filter applied on one of them and not the other is the same bug
// twice.
func (s *service) withDeclared(
	ctx context.Context, rt ResourceType, resourceID string, stored, declared []Policy,
) ([]Policy, *tidcommon.ServiceError) {
	if len(declared) == 0 {
		return stored, nil
	}

	materialized, err := s.store.ListStoredPolicyIDsForResource(ctx, rt, resourceID)
	if err != nil {
		s.logger.Error(ctx, "Failed to list stored sharing policy ids", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	superseded := make(map[string]struct{}, len(materialized))
	for _, id := range materialized {
		superseded[id] = struct{}{}
	}
	for _, d := range declared {
		if _, edited := superseded[d.ID]; !edited {
			stored = append(stored, d)
		}
	}
	return stored, nil
}

// declaredForResource returns the policies a resource file declares for one resource, or nothing
// when the deployment declares none.
func (s *service) declaredForResource(rt ResourceType, resourceID string) []Policy {
	if s.declarativeStore == nil {
		return nil
	}
	return s.declarativeStore.listForResource(rt, resourceID)
}

// findPolicy looks up an organization unit's policy for a resource across both stores.
// A stored policy supersedes a declared one for the same organization unit. Editing a declared
// policy writes it to the database under its own id, and from then on the stored row is the
// policy: the file still describes where it started, not what it is now.
func (s *service) findPolicy(
	ctx context.Context, rt ResourceType, resourceID, initiatingOUID string,
) (Policy, error) {
	p, err := s.store.GetPolicyByInitiator(ctx, rt, resourceID, initiatingOUID)
	if err == nil {
		return p, nil
	}
	if !errors.Is(err, ErrPolicyNotFound) {
		return Policy{}, err
	}
	if s.declarativeStore != nil {
		if declared, ok := s.declarativeStore.getByInitiator(rt, resourceID, initiatingOUID); ok {
			return declared, nil
		}
	}
	return Policy{}, ErrPolicyNotFound
}

// resolveVisibility answers whether one organization unit can see a resource, and through which
// policies, without consulting the cache.
func (s *service) resolveVisibility(
	ctx context.Context, rt ResourceType, resourceID, owningOUID, ouID string,
) (bool, []policyengine.Coverage, *tidcommon.ServiceError) {
	if ouID == owningOUID {
		return true, nil, nil
	}
	chain, svcErr := s.buildChain(ctx, ouID)
	if svcErr != nil {
		return false, nil, svcErr
	}
	policies, svcErr := s.relevantPolicies(ctx, rt, resourceID, chain)
	if svcErr != nil {
		return false, nil, svcErr
	}
	visible, covering := policyengine.EvaluateChain(chain, toEnginePolicies(policies))
	return visible, covering, nil
}

// buildChain returns the organization unit chain, tree root first and ouID last.
func (s *service) buildChain(ctx context.Context, ouID string) ([]string, *tidcommon.ServiceError) {
	ancestors, svcErr := s.ouHierarchyResolver.GetAncestorOUIDs(ctx, ouID)
	if svcErr != nil {
		return nil, svcErr
	}
	chain := make([]string, 0, len(ancestors)+1)
	for i := len(ancestors) - 1; i >= 0; i-- {
		chain = append(chain, ancestors[i])
	}
	return append(chain, ouID), nil
}

// clearCaches drops every derived answer after a write, since one edit can change an unbounded
// number of them and tracking per-key dependencies would cost more than recomputing.
func (s *service) clearCaches(ctx context.Context) {
	if s.visibilityCache != nil {
		if err := s.visibilityCache.Clear(ctx); err != nil {
			s.logger.Warn(ctx, "Failed to clear the visibility cache", log.Error(err))
		}
	}
	if s.overlayRuleCache != nil {
		if err := s.overlayRuleCache.Clear(ctx); err != nil {
			s.logger.Warn(ctx, "Failed to clear the overlay cache", log.Error(err))
		}
	}
}

// ListVisibleResourceIDs returns the resources of a type an organization unit can see.
//
// This is the reverse of IsVisible, and the reason the chain-scoped query carries no resource
// predicate: one narrowed fetch returns every policy that could bear on this organization unit
// across all resources of the type, and each resource is then decided from the policies covering it.
func (s *service) ListVisibleResourceIDs(
	ctx context.Context, rt ResourceType, ouID string,
) ([]string, *tidcommon.ServiceError) {
	if ouID == "" {
		return []string{}, nil
	}

	chain, svcErr := s.buildChain(ctx, ouID)
	if svcErr != nil {
		return nil, svcErr
	}

	// No resource is named: this asks across every resource of the type, which is the one caller
	// that wants the unnarrowed fetch.
	policies, err := s.store.ListPoliciesRelevantToChain(ctx, rt, "", chain)
	if err != nil {
		s.logger.Error(ctx, "Failed to list policies relevant to an organization unit chain",
			log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if s.declarativeStore != nil {
		policies, svcErr = s.withDeclared(ctx, rt, "", policies, s.declarativeStore.listForType(rt))
		if svcErr != nil {
			return nil, svcErr
		}
	}

	byResource := make(map[string][]Policy)
	order := make([]string, 0, len(policies))
	for _, p := range policies {
		if _, seen := byResource[p.ResourceID]; !seen {
			order = append(order, p.ResourceID)
		}
		byResource[p.ResourceID] = append(byResource[p.ResourceID], p)
	}

	out := make([]string, 0, len(order))
	for _, resourceID := range order {
		if visible, _ := policyengine.EvaluateChain(chain, toEnginePolicies(byResource[resourceID])); visible {
			out = append(out, resourceID)
		}
	}
	return out, nil
}
