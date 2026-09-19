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
	visibilityCache cache.CacheInterface[bool]
	overlayCache    cache.CacheInterface[ResolvedOverlay]

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
	overlayCache cache.CacheInterface[ResolvedOverlay],
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
		overlayCache:                 overlayCache,
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

func (s *service) createPolicy(
	ctx context.Context, rt ResourceType, resourceID, owningOUID string, req PolicyRequest, declared bool,
) (Policy, *tidcommon.ServiceError) {
	if _, ok := s.registry.get(rt); !ok {
		return Policy{}, &ErrorResourceTypeNotRegistered
	}
	if resourceID == "" || owningOUID == "" {
		return Policy{}, &ErrorInvalidRequestFormat
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
		// A reshare records one covering policy as its parent purely to fix a replay order for
		// export. It is not the re-materialization dependency: coverage can come from several
		// policies at once, so rules are recomputed against all of them rather than along this edge.
		parentPolicyID = canonicalParentID(covering)
	}

	targets, svcErr := s.buildTargets(ctx, req.TargetOUScope, initiatingOUID, owningOUID, stage)
	if svcErr != nil {
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
		ExcludedOUIDs:  s.collectExclusions(req.TargetOUScope),
	}

	rules, svcErr := s.materializeRules(ctx, rt, resourceID, owningOUID, initiatingOUID, req, targets)
	if svcErr != nil {
		return Policy{}, svcErr
	}
	policy.Rules = rules
	return policy, nil
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
		out := make([]Target, 0, len(scope.OUIDs))
		for _, entry := range scope.OUIDs {
			if entry.OUID == "" {
				return nil, &ErrorInvalidRequestFormat
			}
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
		stored, svcErr := s.narrowOne(ctx, rt, fieldKey, "", initiatingOUID, initiatorRules, requested)
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
	for _, entry := range req.TargetOUScope.OUIDs {
		for fieldKey, requested := range entry.OverlayRules {
			stored, svcErr := s.narrowOne(
				ctx, rt, fieldKey, byOU[entry.OUID], initiatingOUID, initiatorRules, requested)
			if svcErr != nil {
				return nil, svcErr
			}
			out = append(out, stored)
		}
	}
	return out, nil
}

// narrowOne folds one requested rule and reports the offending field on rejection.
func (s *service) narrowOne(
	ctx context.Context, rt ResourceType, fieldKey, targetID, initiatingOUID string,
	initiatorRules map[string]OverlayRule, requested OverlayRule,
) (StoredRule, *tidcommon.ServiceError) {
	decl, ok := s.registry.field(rt, fieldKey)
	if !ok {
		return StoredRule{}, withDetail(ErrorUnknownFieldKey, fieldKey)
	}
	if svcErr := s.validateMembers(ctx, rt, fieldKey, initiatingOUID, requested); svcErr != nil {
		return StoredRule{}, svcErr
	}

	c := policyengine.NewContainment(policyengine.FieldKind(decl.Kind), "")
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
	ctx context.Context, rt ResourceType, fieldKey, initiatingOUID string, r OverlayRule,
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
	if err := validator.ValidateMembers(ctx, fieldKey, initiatingOUID, dedupeStrings(members)); err != nil {
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
	if current.Declared {
		return Policy{}, &ErrorPolicyDeclared
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

	if err := s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		if err := s.store.ReplacePolicyContents(txCtx, proposed, current.Version); err != nil {
			return err
		}
		// Rules stored beneath this policy were clamped against a ceiling that may just have
		// moved, so they are recomputed in the same transaction rather than left stale.
		return s.rematerializeDependents(txCtx, current)
	}); err != nil {
		if errors.Is(err, ErrPolicyNotFound) {
			return Policy{}, &ErrorVersionMismatch
		}
		s.logger.Error(ctx, "Failed to update sharing policy", log.Error(err))
		return Policy{}, &tidcommon.InternalServerError
	}

	s.clearCaches(ctx)
	proposed.Version = current.Version + 1
	return proposed, nil
}

// canonicalParentID picks one covering policy to record as a reshare's parent, deterministically,
// so an export replays in the same order every time. Declared policies are skipped because they are
// not replayed through the stored path.
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

// rematerializeDependents recomputes every reshare whose ceiling the edited policy may have moved.
//
// Coverage is not a single edge. An organization unit can be reached by several policies at once and
// its effective rules are the intersection of all of them, so following one stored parent would miss
// reshares that a second covering policy also bounds. Every reshare of the resource is recomputed
// instead, shallowest initiator first: a covering policy's initiator always sits above the unit it
// covers, so that order rebuilds each ceiling before the rules clamped against it.
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
			next, svcErr := s.narrowOne(ctx, descendant.ResourceType, stored.FieldKey, stored.TargetID,
				descendant.InitiatingOUID, initiatorRules, stored.Requested)
			if svcErr != nil {
				return fmt.Errorf("failed to re-materialize policy %s: %s", descendant.ID, svcErr.Code)
			}
			rebuilt = append(rebuilt, next)
		}
		// Recomputing every reshare means most of them are unchanged, and rewriting those would bump
		// their version and fail a concurrent edit of a policy this one never touched.
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

// cleanUpLostVisibility fires the resource type's hook for every organization unit the removed
// policy was the last one covering.
//
// The set is computed from visibility after the delete rather than from the policy's target list,
// so an organization unit another policy still covers keeps its own state.
func (s *service) cleanUpLostVisibility(ctx context.Context, removed Policy) error {
	decl, ok := s.registry.get(removed.ResourceType)
	if !ok {
		return nil
	}
	hooks, ok := decl.(PolicyHooks)
	if !ok {
		return nil
	}

	fields := make([]string, 0, len(removed.Rules))
	for _, r := range removed.Rules {
		fields = append(fields, r.FieldKey)
	}

	candidates, err := s.ousReachedBy(ctx, removed)
	if err != nil {
		return err
	}

	for _, ouID := range candidates {
		stillVisible, _, svcErr := s.resolveVisibility(
			ctx, removed.ResourceType, removed.ResourceID, removed.OwningOUID, ouID)
		if svcErr != nil {
			return fmt.Errorf("failed to resolve visibility for %s: %s", ouID, svcErr.Code)
		}
		if stillVisible {
			continue
		}
		if err := hooks.OnVisibilityLost(
			ctx, removed.ResourceID, ouID, dedupeStrings(fields)); err != nil {
			return err
		}
	}
	return nil
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
	if s.declarativeStore != nil {
		if p, ok := s.declarativeStore.get(policyID); ok {
			return p, nil
		}
	}
	p, err := s.store.GetPolicy(ctx, policyID)
	if err != nil {
		if errors.Is(err, ErrPolicyNotFound) {
			return Policy{}, &ErrorPolicyNotFound
		}
		s.logger.Error(ctx, "Failed to get sharing policy", log.Error(err))
		return Policy{}, &tidcommon.InternalServerError
	}
	return p, nil
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
		stored = append(stored, s.declarativeStore.listForResource(rt, resourceID)...)
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
	key := cache.CacheKey{Key: fmt.Sprintf("%s:%s:%s", rt, resourceID, ouID)}
	if s.visibilityCache != nil {
		if cached, ok := s.visibilityCache.Get(ctx, key); ok {
			return cached, nil
		}
	}

	// The owner sees its own resource whether or not a policy exists. Answering that from the
	// policies alone would make a resource nobody has shared yet invisible to the unit that owns it.
	// The answer is deliberately not cached: ownership is the resource type's own state, and this
	// cache is only ever cleared by policy writes.
	owner, known, svcErr := s.ownerOf(ctx, rt, resourceID)
	if svcErr != nil {
		return false, svcErr
	}
	if known && owner == ouID {
		return true, nil
	}

	policies, svcErr := s.relevantPolicies(ctx, rt, resourceID, ouID)
	if svcErr != nil {
		return false, svcErr
	}
	chain, svcErr := s.buildChain(ctx, ouID)
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
	key := cache.CacheKey{Key: fmt.Sprintf("%s:%s:%s", rt, resourceID, ouID)}
	if s.overlayCache != nil {
		if cached, ok := s.overlayCache.Get(ctx, key); ok {
			return cached, nil
		}
	}

	out, svcErr := s.resolveOverlayRules(ctx, rt, resourceID, ouID)
	if svcErr != nil {
		return ResolvedOverlay{}, svcErr
	}

	if s.overlayCache != nil {
		if err := s.overlayCache.Set(ctx, key, out); err != nil {
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
	policies, svcErr := s.relevantPolicies(ctx, rt, resourceID, ouID)
	if svcErr != nil {
		return ResolvedOverlay{}, svcErr
	}

	out := ResolvedOverlay{
		OUID:    ouID,
		Rules:   map[string]OverlayRule{},
		Sources: map[string]string{},
	}
	if len(policies) > 0 && policies[0].OwningOUID == ouID {
		out.Owned = true
	}

	chain, svcErr := s.buildChain(ctx, ouID)
	if svcErr != nil {
		return ResolvedOverlay{}, svcErr
	}
	_, covering := policyengine.EvaluateChain(chain, toEnginePolicies(policies))

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

		c := policyengine.NewContainment(policyengine.FieldKind(f.Kind), "")
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

// relevantPolicies fetches every policy that could bear on one organization unit, from both the
// database and any declared in resource files.
func (s *service) relevantPolicies(
	ctx context.Context, rt ResourceType, resourceID, ouID string,
) ([]Policy, *tidcommon.ServiceError) {
	policies, svcErr := s.ListPolicies(ctx, rt, resourceID)
	if svcErr != nil {
		return nil, svcErr
	}
	_ = ouID
	return policies, nil
}

// findPolicy looks up an organization unit's policy for a resource across both stores.
func (s *service) findPolicy(
	ctx context.Context, rt ResourceType, resourceID, initiatingOUID string,
) (Policy, error) {
	if s.declarativeStore != nil {
		if p, ok := s.declarativeStore.getByInitiator(rt, resourceID, initiatingOUID); ok {
			return p, nil
		}
	}
	return s.store.GetPolicyByInitiator(ctx, rt, resourceID, initiatingOUID)
}

// resolveVisibility answers whether one organization unit can see a resource, and through which
// policies, without consulting the cache.
func (s *service) resolveVisibility(
	ctx context.Context, rt ResourceType, resourceID, owningOUID, ouID string,
) (bool, []policyengine.Coverage, *tidcommon.ServiceError) {
	if ouID == owningOUID {
		return true, nil, nil
	}
	policies, svcErr := s.relevantPolicies(ctx, rt, resourceID, ouID)
	if svcErr != nil {
		return false, nil, svcErr
	}
	chain, svcErr := s.buildChain(ctx, ouID)
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
	if s.overlayCache != nil {
		if err := s.overlayCache.Clear(ctx); err != nil {
			s.logger.Warn(ctx, "Failed to clear the overlay cache", log.Error(err))
		}
	}
}
