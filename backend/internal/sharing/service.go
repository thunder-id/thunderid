// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package sharing provides a resource-type-agnostic framework for sharing a resource from its
// owning organization unit to other organization units, and for resolving what those organization
// units may do with it. A resource type is onboarded by registering a
// ResourceOverlayFieldDeclaration; the engine, storage and rule resolution are identical for every
// type.
package sharing

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/cache"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// SharingServiceInterface is the sharing framework's entry point. It orchestrates; it does not decide.
type SharingServiceInterface interface {
	// RegisterResourceType adds a resource type's declaration to the framework.
	RegisterResourceType(decl ResourceOverlayFieldDeclaration)

	// CreatePolicy records one organization unit's decision about one resource. Exactly one policy
	// exists per resource per initiating organization unit; a second create is refused.
	CreatePolicy(
		ctx context.Context, rt ResourceType, resourceID, owningOUID string, req PolicyRequest,
		origin PolicyOrigin,
	) (Policy, *tidcommon.ServiceError)
	// LoadDeclarativeResources reads a resource type's declarative documents and declares the
	// sharing policies they carry. Called once per shareable resource type, at startup.
	LoadDeclarativeResources(ctx context.Context, cfg DeclarativeLoaderConfig) error
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
	// ListPolicies returns every policy recorded for a resource, unbounded. This is the whole-set
	// read the framework itself uses; a management API wants GetPolicyList instead.
	ListPolicies(ctx context.Context, rt ResourceType, resourceID string) ([]Policy, *tidcommon.ServiceError)
	// GetPolicyList returns one page of a resource's policies, for a management API to serve.
	GetPolicyList(
		ctx context.Context, rt ResourceType, resourceID string, limit, offset int,
	) (PolicyList, *tidcommon.ServiceError)
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
	// ResolveOverlayValues returns the effective value per field for one organization unit: what it
	// chose for itself where its rule let it choose, and what the rule carries where it did not.
	ResolveOverlayValues(
		ctx context.Context, rt ResourceType, resourceID, ouID string,
	) (ResolvedValues, *tidcommon.ServiceError)

	// SetOverlayValue records one organization unit's own value for one field of a shared resource.
	// Refused when the unit cannot see the resource, when its rule pins the field, or when the
	// value reaches outside what that rule offers.
	SetOverlayValue(
		ctx context.Context, rt ResourceType, resourceID, ouID, fieldKey string, value []string,
	) *tidcommon.ServiceError
	// DeleteOverlayValue drops an organization unit's own value for one field, returning the field
	// to whatever its rule carries.
	DeleteOverlayValue(
		ctx context.Context, rt ResourceType, resourceID, ouID, fieldKey string,
	) *tidcommon.ServiceError
}

// sharingService orchestrates policy storage, chain resolution and the resource type's own hooks. It
// fetches, stores and clears caches; every judgement it makes is a call into the decision functions
// in policy_engine.go.
type sharingService struct {
	logger *log.Logger
	// store is the composite: declared policies and stored ones behind one interface.
	store sharingPolicyStoreInterface
	// dbStore is the stored half alone, for the one question the composite cannot answer: whether a
	// database row governs an organization unit, asked while seeding a declaration that may be
	// replacing itself.
	dbStore sharingPolicyStoreInterface
	// fileStore is the declared half alone, for seeding and for the declared-id checks, neither of
	// which is a store-interface operation.
	fileStore           *fileBasedStore
	registry            *registry
	ouHierarchyResolver sysauthz.OUHierarchyResolver
	// ouEnumerator walks the tree downwards, which policy deletion needs to find the units beneath
	// a removed target. Kept separate from the resolver above: enumeration is not an access decision.
	ouEnumerator  oupkg.HierarchyEnumeratorInterface
	transactioner providers.Transactioner

	// The caches are all derived from the policy graph, so any write clears them wholesale: one
	// edit can change an unbounded number of resolved answers.
	visibilityCache  cache.CacheInterface[bool]
	overlayRuleCache cache.CacheInterface[ResolvedOverlay]

	allowChildOUCrossTreeSharing bool
}

// declaredHalf returns the file store as an interface, keeping a nil one nil: a typed nil in an
// interface is not nil, and the composite decides whether to wrap on exactly that test.
func declaredHalf(f *fileBasedStore) sharingPolicyStoreInterface {
	if f == nil {
		return nil
	}
	return f
}

// newSharingService builds the sharing service.
func newSharingService(
	dbStore sharingPolicyStoreInterface,
	fileStore *fileBasedStore,
	ouHierarchyResolver sysauthz.OUHierarchyResolver,
	ouEnumerator oupkg.HierarchyEnumeratorInterface,
	transactioner providers.Transactioner,
	visibilityCache cache.CacheInterface[bool],
	overlayRuleCache cache.CacheInterface[ResolvedOverlay],
	allowChildOUCrossTreeSharing bool,
) SharingServiceInterface {
	return &sharingService{
		logger: log.GetLogger().With(
			log.String(log.LoggerKeyComponentName, "SharingService")),
		store:                        newCompositeStore(declaredHalf(fileStore), dbStore),
		dbStore:                      dbStore,
		fileStore:                    fileStore,
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
func (s *sharingService) RegisterResourceType(decl ResourceOverlayFieldDeclaration) {
	s.registry.register(decl)
}

// PolicyOrigin says where a create came from, which decides whether the policy may name a reach
// the issuing organization unit's own position in the tree does not bound. Such a reach is a
// deployment decision, so it is written where those are reviewed.
type PolicyOrigin string

const (
	// PolicyFromRequest is an interactive create through a management API.
	PolicyFromRequest PolicyOrigin = "request"
	// PolicyFromImport is a document an export produced, imported back. Reviewed configuration,
	// the same as a resource file.
	PolicyFromImport PolicyOrigin = "import"
	// policyFromDeclaration is a resource file's own policy.
	policyFromDeclaration PolicyOrigin = "declaration"
)

// CreatePolicy records one organization unit's decision about one resource.
func (s *sharingService) CreatePolicy(
	ctx context.Context, rt ResourceType, resourceID, owningOUID string, req PolicyRequest,
	origin PolicyOrigin,
) (Policy, *tidcommon.ServiceError) {
	// Anything but an import is treated as a request, so an origin this method does not serve
	// gets the stricter half rather than the looser one.
	if origin != PolicyFromImport {
		origin = PolicyFromRequest
	}
	return s.createPolicy(ctx, rt, resourceID, owningOUID, req, origin)
}

// LoadDeclarativeResources reads a resource type's documents and declares the policies they carry.
func (s *sharingService) LoadDeclarativeResources(
	ctx context.Context, cfg DeclarativeLoaderConfig,
) error {
	return loadDeclarativeResources(ctx, s, s.fileStore, cfg)
}

// CreateDeclarativePolicy records a policy a resource file declares.
func (s *sharingService) CreateDeclarativePolicy(
	ctx context.Context, rt ResourceType, resourceID, owningOUID string, req PolicyRequest,
) (Policy, *tidcommon.ServiceError) {
	if s.fileStore == nil {
		s.logger.Error(ctx, "Declarative policy declared while declarative resources are disabled",
			log.String("resourceType", string(rt)), log.String("resourceID", resourceID))
		return Policy{}, &tidcommon.InternalServerError
	}
	return s.createPolicy(ctx, rt, resourceID, owningOUID, req, policyFromDeclaration)
}

// createPolicy is the shared body behind CreatePolicy and CreateDeclarativePolicy. A declared
// policy is seeded into the in-memory store instead of the database, which is the only step the
// two do differently.
func (s *sharingService) createPolicy(
	ctx context.Context, rt ResourceType, resourceID, owningOUID string, req PolicyRequest,
	origin PolicyOrigin,
) (Policy, *tidcommon.ServiceError) {
	declared := origin == policyFromDeclaration
	if _, ok := s.registry.get(rt); !ok {
		return Policy{}, &ErrorResourceTypeNotRegistered
	}
	if resourceID == "" {
		return Policy{}, withDetail(ErrorInvalidRequestFormat, "the resource is not named")
	}
	if owningOUID == "" {
		return Policy{}, withDetail(ErrorInvalidRequestFormat, "the resource has no owning organization unit")
	}
	// A reach not bounded by the issuer's own position in the tree is a deployment decision, so it
	// is written where such decisions are reviewed: a resource file, or a document replayed from
	// one. An interactive request is neither.
	if origin == PolicyFromRequest {
		if svcErr := requireDeploymentWideScopeFromConfiguration(req.Targets); svcErr != nil {
			return Policy{}, svcErr
		}
	}

	initiatingOUID := req.InitiatingOUID
	if initiatingOUID == "" {
		initiatingOUID = owningOUID
	}

	if declared {
		if svcErr := s.requireUsableDeclaredID(req.ID, rt, resourceID, initiatingOUID); svcErr != nil {
			return Policy{}, svcErr
		}
	} else if req.ID != "" {
		if svcErr := s.requireUnusedStoredID(ctx, req.ID); svcErr != nil {
			return Policy{}, svcErr
		}
	}

	// One policy per organization unit per resource, checked before the insert.
	if svcErr := s.requireSoleGovernor(ctx, rt, resourceID, initiatingOUID, declared); svcErr != nil {
		return Policy{}, svcErr
	}

	policy, svcErr := s.buildPolicy(ctx, rt, resourceID, owningOUID, initiatingOUID, req, declared)
	if svcErr != nil {
		return Policy{}, svcErr
	}

	if declared {
		s.fileStore.seed(policy)
		s.clearCaches(ctx)
		return policy, nil
	}

	if err := s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		return s.store.CreatePolicy(txCtx, policy)
	}); err != nil {
		// The check above and this insert are not atomic, so a concurrent create can take the one
		// slot between them and leave this one rejected. Re-reading names the policy that won,
		// which is the one to edit, rather than reporting a bare failure. Asking the database again
		// is also what keeps this free of driver-specific error parsing.
		//
		// The row carries two constraints and either can be the one that rejected it: one policy
		// per organization unit per resource, and the policy id itself. A replay supplies its own
		// id, so both are reachable, and the id is looked up as well rather than falling through to
		// a server error that describes neither.
		existing, lookupErr := s.store.GetPolicyByInitiator(ctx, rt, resourceID, initiatingOUID)
		if lookupErr == nil {
			return Policy{}, withDetail(ErrorPolicyExists, "existing policy "+existing.ID)
		}
		if policy.ID != "" {
			if byID, idErr := s.store.GetPolicy(ctx, policy.ID); idErr == nil {
				return Policy{}, withDetail(ErrorPolicyExists, "existing policy "+byID.ID)
			}
		}
		s.logger.Error(ctx, "Failed to create sharing policy", log.Error(err))
		return Policy{}, &tidcommon.InternalServerError
	}
	s.clearCaches(ctx)
	return policy, nil
}

// requireSoleGovernor holds the one-policy-per-organization-unit-per-resource rule, which is what
// gives an edit a single well-defined subject.
//
// What counts as a collision differs by origin. An API create collides with anything already
// governing the pair, stored or declared. A declaration collides only with a stored row: another
// declaration for the same pair is the file being re-applied, which replaces what it declared
// rather than duplicating it, and seed is an upsert for exactly that reason.
//
// A declaration is not exempt from the stored half. Nothing folds a declared policy together with a
// stored one, so letting a file declare over a row the API already wrote leaves the organization
// unit governed by both at once: on the next startup the declaration is seeded beside the row, and
// resolution answers through the two folded together rather than through either one. The file and
// the database disagreeing about who governs a pair is an operator's problem to settle, so it is
// reported rather than silently resolved in favor of one of them.
func (s *sharingService) requireSoleGovernor(
	ctx context.Context, rt ResourceType, resourceID, initiatingOUID string, declared bool,
) *tidcommon.ServiceError {
	// A declaration re-applied from its own file replaces itself, so only a database row conflicts
	// with it; an API create conflicts with either store, which is what the composite answers.
	lookup, conflict := s.store.GetPolicyByInitiator, ErrorPolicyExists
	if declared {
		lookup, conflict = s.dbStore.GetPolicyByInitiator, ErrorDeclaredPolicyConflictsWithStored
	}

	existing, err := lookup(ctx, rt, resourceID, initiatingOUID)
	switch {
	case err == nil:
		return withDetail(conflict, "existing policy "+existing.ID)
	case errors.Is(err, errPolicyNotFound):
		return nil
	default:
		s.logger.Error(ctx, "Failed to check for an existing policy", log.Error(err))
		return &tidcommon.InternalServerError
	}
}

// requireUsableDeclaredID holds a declaration to carrying its own id, and to not reusing one that
// another declaration already claims.
//
// A declared policy has no database row, so a reshare beneath it stores this id as a plain value
// rather than a foreign key, exactly as a stored role assignment references a declared role. That
// only holds together if the id comes from the file: an id minted here would differ on every
// startup and every reshare beneath it would point at a policy that no longer exists.
func (s *sharingService) requireUsableDeclaredID(
	id string, rt ResourceType, resourceID, initiatingOUID string,
) *tidcommon.ServiceError {
	if id == "" {
		return withDetail(ErrorDeclaredPolicyIDRequired, string(rt)+" "+resourceID)
	}
	existing, found := s.fileStore.get(id)
	if !found {
		return nil
	}
	// Re-applying the same declaration is the upsert case, not a conflict.
	if existing.ResourceType == rt && existing.ResourceID == resourceID &&
		existing.InitiatingOUID == initiatingOUID {
		return nil
	}
	return withDetail(ErrorDeclaredPolicyIDConflict, id)
}

// requireUnusedStoredID refuses a replay that names an id some other policy already holds, so the
// collision is answered as a conflict rather than as a failed insert.
func (s *sharingService) requireUnusedStoredID(ctx context.Context, id string) *tidcommon.ServiceError {
	if _, svcErr := s.GetPolicy(ctx, id); svcErr == nil {
		return withDetail(ErrorPolicyExists, id)
	} else if svcErr.Code != ErrorPolicyNotFound.Code {
		return svcErr
	}
	return nil
}

// requireDeploymentWideScopeFromConfiguration refuses the two scopes whose reach is not bounded by
// the initiator's own position in the tree, for a request that is not reviewed configuration.
func requireDeploymentWideScopeFromConfiguration(targets []TargetRequest) *tidcommon.ServiceError {
	for _, t := range targets {
		switch t.Scope {
		case ScopeAllOUs, ScopeAllRoots:
			return withDetail(ErrorDeploymentWideScopeDeclarativeOnly, string(t.Scope))
		}
	}
	return nil
}

// buildPolicy validates a request and materializes its rules against what the initiator holds.
func (s *sharingService) buildPolicy(
	ctx context.Context, rt ResourceType, resourceID, owningOUID, initiatingOUID string,
	req PolicyRequest, declared bool,
) (Policy, *tidcommon.ServiceError) {
	stage := stageShare
	parentPolicyID := ""
	if initiatingOUID != owningOUID {
		stage = stageReshare
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

	targets, svcErr := s.buildTargets(ctx, req.Targets, initiatingOUID, owningOUID, stage)
	if svcErr != nil {
		return Policy{}, svcErr
	}

	if svcErr := s.requireExclusionsInReach(ctx, targets, initiatingOUID); svcErr != nil {
		return Policy{}, svcErr
	}

	// A request carrying an id is a replay of one that already existed, so it keeps it: a reshare
	// beneath it points at that id, and an import that minted a new one would orphan it. A request
	// carrying none is a fresh create and is given one. The version it was exported at travels the
	// same way, so an export of the replayed policy matches the document it came from.
	id := req.ID
	if id == "" {
		generated, err := utils.GenerateUUIDv7()
		if err != nil {
			s.logger.Error(ctx, "Failed to generate a policy identifier", log.Error(err))
			return Policy{}, &tidcommon.InternalServerError
		}
		id = generated
	}

	policy := Policy{
		ID:             id,
		ResourceType:   rt,
		ResourceID:     resourceID,
		OwningOUID:     owningOUID,
		InitiatingOUID: initiatingOUID,
		Stage:          stage,
		ParentPolicyID: parentPolicyID,
		Declared:       declared,
		Version:        max(req.Version, 1),
		Targets:        targets,
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
func (s *sharingService) requireEditLeavesFrontiersIntact(
	ctx context.Context, proposed Policy,
) *tidcommon.ServiceError {
	policies, err := s.store.ListAllPoliciesForResource(ctx, proposed.ResourceType, proposed.ResourceID)
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
		if cascadingScope(t.Scope) {
			cascading = append(cascading, t)
		}
	}
	return reachedByAnyTarget(cascading, ouID, chain)
}

// requireExclusionsInReach refuses an exclusion naming an organization unit none of the policy's
// own targets reach. Nothing would apply it, and accepting it would read as a withholding that
// never happened.
func (s *sharingService) requireExclusionsInReach(
	ctx context.Context, targets []Target, initiatingOUID string,
) *tidcommon.ServiceError {
	for _, t := range targets {
		for _, ouID := range t.ExcludedOUIDs {
			// Carving out the unit a target anchors on cancels the target rather than narrowing
			// it: root and child stop at the unit they name, and childSubtree starts from it, so
			// in every case nothing is left for the target to reach.
			if ouID == t.OUID && anchoredScope(t.Scope) {
				return withDetail(ErrorExclusionEmptiesTarget, ouID)
			}
			chain, svcErr := s.buildChain(ctx, ouID)
			if svcErr != nil {
				return svcErr
			}
			// A target reaching down the initiator's own tree names organization units the way
			// its targets do, one hop below the issuer. Carving out a grandchild would decide for
			// the child over its head, and under a cascading target that child holds no policy of
			// its own to reverse it with.
			if ownTreeScope(t.Scope) && (len(chain) < 2 || chain[len(chain)-2] != initiatingOUID) {
				return withDetail(ErrorExclusionNotADirectChild, ouID)
			}
			if !reachedByAnyTarget([]Target{t}, ouID, chain) {
				return withDetail(ErrorExclusionOutOfReach, ouID)
			}
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
		case ScopeAllOUs:
			return true
		case ScopeAllRoots:
			if len(chain) > 0 && chain[0] == ouID {
				return true
			}
		case ScopeAllChildren, ScopeChildSubtree:
			for _, ancestor := range chain {
				if ancestor == t.OUID && ancestor != ouID {
					return true
				}
			}
			if t.Scope == ScopeChildSubtree && t.OUID == ouID {
				return true
			}
		case ScopeRoot, ScopeChild:
			if t.OUID == ouID {
				return true
			}
		}
	}
	return false
}

// ownTreeScope reports whether a target reaches down the initiating organization unit's own tree.
// The deployment-wide and root scopes are not bounded by where the initiator sits, so the one-hop
// rule has nothing to say about what they may carve out.
func ownTreeScope(scope TargetScope) bool {
	switch scope {
	case ScopeChild, ScopeChildSubtree, ScopeAllChildren:
		return true
	}
	return false
}

// anchoredScope reports whether a target's reach is defined by the organization unit it names,
// rather than by a family of units. allChildren is not one: it carries the issuing unit so the row
// has an anchor, but it reaches below that unit and never the unit itself.
func anchoredScope(scope TargetScope) bool {
	switch scope {
	case ScopeRoot, ScopeChild, ScopeChildSubtree:
		return true
	}
	return false
}

// cascadingScope reports whether a target reaches below the organization unit it names, which is
// what leaves that unit nothing of its own to grant.
func cascadingScope(scope TargetScope) bool {
	switch scope {
	case ScopeAllOUs,
		ScopeAllChildren,
		ScopeChildSubtree:
		return true
	}
	return false
}

// requireFrontierUnit refuses a reshare when every target covering the initiator carries on past
// it. One target that stops at the initiator is enough, since that reach is the initiator's to
// hand on.
func requireFrontierUnit(covering []Coverage) *tidcommon.ServiceError {
	for _, c := range covering {
		for _, t := range c.Policy.Targets {
			if t.ID == c.TargetID && !cascadingScope(t.Scope) {
				return nil
			}
		}
	}
	return &ErrorReshareNotPermitted
}

// newTargetID mints one target row's identifier. Five branches below need one, and a failure has to
// reach the caller rather than being swallowed: a target row with no id is not a target.
func (s *sharingService) newTargetID(ctx context.Context) (string, *tidcommon.ServiceError) {
	id, err := utils.GenerateUUIDv7()
	if err != nil {
		s.logger.Error(ctx, "Failed to generate a target identifier", log.Error(err))
		return "", &tidcommon.InternalServerError
	}
	return id, nil
}

// buildTargets turns a request's targets into stored target rows, enforcing the stage, ownership
// and one-hop rules along the way.
//
// The list is flat and may mix breadths: a broad target carving one organization unit out, beside
// a target naming that unit on terms of its own, is the whole point of letting it. Each entry is
// checked on its own, and then the set is checked for a unit named twice.
func (s *sharingService) buildTargets(
	ctx context.Context, requested []TargetRequest, initiatingOUID, owningOUID string, stage policyStage,
) ([]Target, *tidcommon.ServiceError) {
	if len(requested) == 0 {
		return nil, withDetail(ErrorInvalidRequestFormat, "the policy names no target")
	}

	isOwner := initiatingOUID == owningOUID
	out := make([]Target, 0, len(requested))
	// Naming one organization unit twice is rejected rather than deduplicated, because the two
	// entries carry their own overlay rules and there is no basis for picking a winner. A unit
	// named by child and by childSubtree is the same clash: the rows differ, so the database
	// would accept both while the rules attached to one of them alone.
	seen := make(map[string]struct{}, len(requested))

	for _, t := range requested {
		ouID := t.OUID
		switch t.Scope {
		case ScopeAllOUs:
			// Reaching every organization unit is the owner's call alone, and only as a first hop.
			if !isOwner || stage != stageShare {
				return nil, withDetail(ErrorInvalidTargetOU, "allOus is owner-only")
			}
			if ouID != "" {
				return nil, targetTakesNoOU(t.Scope)
			}
			// Reaching every organization unit necessarily leaves the initiator's own tree.
			if svcErr := s.requireCrossTreeAllowed(ctx, initiatingOUID); svcErr != nil {
				return nil, svcErr
			}

		case ScopeAllRoots:
			if !isOwner {
				return nil, withDetail(ErrorInvalidTargetOU, "root targeting is owner-only")
			}
			if ouID != "" {
				return nil, targetTakesNoOU(t.Scope)
			}
			if svcErr := s.requireCrossTreeAllowed(ctx, initiatingOUID); svcErr != nil {
				return nil, svcErr
			}

		case ScopeRoot:
			if !isOwner {
				return nil, withDetail(ErrorInvalidTargetOU, "root targeting is owner-only")
			}
			if ouID == "" {
				return nil, targetNeedsOU(t.Scope)
			}
			if svcErr := s.validateRootTarget(ctx, ouID, initiatingOUID); svcErr != nil {
				return nil, svcErr
			}

		case ScopeAllChildren:
			if ouID != "" {
				return nil, targetTakesNoOU(t.Scope)
			}
			// The subtree target anchors on the unit issuing it, which the row has to carry.
			ouID = initiatingOUID

		case ScopeChild, ScopeChildSubtree:
			if ouID == "" {
				return nil, targetNeedsOU(t.Scope)
			}
			if svcErr := s.validateChildTarget(ctx, ouID, initiatingOUID); svcErr != nil {
				return nil, svcErr
			}

		default:
			return nil, withDetail(ErrorInvalidRequestFormat, fmt.Sprintf("unknown target scope %q", t.Scope))
		}

		key := ouID
		if key == "" {
			key = string(t.Scope)
		}
		if _, repeated := seen[key]; repeated {
			return nil, withDetail(ErrorInvalidTargetOU, key+" is named more than once")
		}
		seen[key] = struct{}{}

		id, svcErr := s.newTargetID(ctx)
		if svcErr != nil {
			return nil, svcErr
		}
		out = append(out, Target{
			ID:            id,
			Scope:         t.Scope,
			OUID:          ouID,
			ExcludedOUIDs: utils.UniqueStrings(t.ExcludedOUIDs),
		})
	}

	if svcErr := requireDisjointTargets(out); svcErr != nil {
		return nil, svcErr
	}
	return out, nil
}

// requireDisjointTargets refuses a policy whose targets reach the same organization unit twice.
//
// Only three pairings can overlap at all. Every selective target names a direct child of the
// initiator or a tree root, so two of them are disjoint unless they name the same unit, which is
// refused earlier as a repeat.
func requireDisjointTargets(targets []Target) *tidcommon.ServiceError {
	for i, broad := range targets {
		for j, narrow := range targets {
			if i == j || !swallows(broad.Scope, narrow.Scope) {
				continue
			}
			// The unit the narrower target anchors on, which is what the broader one has to carve
			// out. An exclusion takes the subtree with it, so carving out the anchor is enough.
			// A target reaching a whole family anchors on nothing and cannot be made room for.
			anchor := narrow.OUID
			if anchor == "" {
				return withDetail(ErrorOverlappingTargets,
					string(broad.Scope)+" already reaches everything "+string(narrow.Scope)+" names")
			}
			if !slices.Contains(broad.ExcludedOUIDs, anchor) {
				// The unit is named, because a policy may hold several targets of the narrower
				// scope and the caller has to know which one to carve out.
				return withDetail(ErrorOverlappingTargets,
					anchor+" is already reached by "+string(broad.Scope))
			}
		}
	}
	return nil
}

// swallows reports whether a target of the broader scope reaches everything one of the narrower
// scope does, before exclusions are taken into account.
func swallows(broad, narrow TargetScope) bool {
	switch broad {
	case ScopeAllOUs:
		return narrow != ScopeAllOUs
	case ScopeAllRoots:
		return narrow == ScopeRoot
	case ScopeAllChildren:
		return narrow == ScopeChild || narrow == ScopeChildSubtree
	}
	return false
}

// validateChildTarget enforces the one-hop rule: a policy may only name organization units directly
// beneath its initiator, so every extra level of reach is a fresh decision by the unit that holds
// it rather than something an ancestor can grant over its descendants' heads.
func (s *sharingService) validateChildTarget(
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
func (s *sharingService) validateRootTarget(
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
func (s *sharingService) requireCrossTreeAllowed(
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
		return withDetail(ErrorCrossTreeShareRestricted, initiatingOUID+
			" is not a root organization unit, and sharing outside its own tree is not enabled")
	}
	return nil
}

// targetTakesNoOU reports a target naming an organization unit its scope already decides: every
// unit, every root, or the initiator's own subtree.
func targetTakesNoOU(scope TargetScope) *tidcommon.ServiceError {
	return withDetail(ErrorInvalidRequestFormat, fmt.Sprintf("scope %s takes no ouId", scope))
}

// targetNeedsOU reports a target leaving out the organization unit its scope anchors on.
func targetNeedsOU(scope TargetScope) *tidcommon.ServiceError {
	return withDetail(ErrorInvalidRequestFormat, fmt.Sprintf("scope %s needs an ouId", scope))
}

// treeRootOf returns the root of the tree an organization unit belongs to, which is the unit itself
// when it is already a root.
func (s *sharingService) treeRootOf(ctx context.Context, ouID string) (string, *tidcommon.ServiceError) {
	ancestors, svcErr := s.ouHierarchyResolver.GetAncestorOUIDs(ctx, ouID)
	if svcErr != nil {
		return "", svcErr
	}
	if len(ancestors) == 0 {
		return ouID, nil
	}
	return ancestors[len(ancestors)-1], nil
}

// materializeRules folds every requested rule against what the initiating organization unit itself
// holds, and stores the result. Resolving at write means a read is one lookup rather than a walk.
func (s *sharingService) materializeRules(
	ctx context.Context, rt ResourceType, resourceID, owningOUID, initiatingOUID string,
	req PolicyRequest, targets []Target,
) ([]StoredRule, *tidcommon.ServiceError) {
	initiatorRules, _, svcErr := s.initiatorEffectiveRules(ctx, rt, resourceID, owningOUID, initiatingOUID)
	if svcErr != nil {
		return nil, svcErr
	}

	// buildTargets preserved the request's order, so the two lists line up entry for entry and the
	// rules can be attached to the row that was built from the entry carrying them.
	out := make([]StoredRule, 0, len(targets))
	for i, t := range targets {
		for fieldKey, requested := range req.Targets[i].OverlayRules {
			stored, svcErr := s.narrowOne(
				ctx, rt, resourceID, fieldKey, t.ID, initiatingOUID, initiatorRules, requested)
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
// "billingx" and a policy hands over a sibling nobody named. A type declaring a hierarchical field
// therefore has to resolve one, and both ways of failing to, not implementing the capability and
// answering with an empty string, are refused here rather than papered over with a default.
func (s *sharingService) containmentFor(
	ctx context.Context, rt ResourceType, resourceID, fieldKey string, kind FieldKind,
) (containment, *tidcommon.ServiceError) {
	delimiter := ""
	if kind == FieldHierarchy {
		decl, ok := s.registry.get(rt)
		if !ok {
			return containment{}, &ErrorResourceTypeNotRegistered
		}
		resolver, ok := decl.(FieldDelimiterResolver)
		if !ok {
			s.logger.Error(ctx, "Resource type declares a hierarchical field but resolves no delimiter",
				log.String("resourceType", string(rt)), log.String("fieldKey", fieldKey))
			return containment{}, &tidcommon.InternalServerError
		}
		got, svcErr := resolver.FieldDelimiter(ctx, resourceID, fieldKey)
		if svcErr != nil {
			return containment{}, svcErr
		}
		if got == "" {
			s.logger.Error(ctx, "Resource type resolved an empty delimiter for a hierarchical field",
				log.String("resourceType", string(rt)), log.String("resourceID", resourceID))
			return containment{}, &tidcommon.InternalServerError
		}
		delimiter = got
	}
	return newContainment(kind, delimiter), nil
}

// clampOne re-resolves one stored rule against the initiator's current ceiling, cutting it back
// rather than refusing it.
//
// Replay starts from the original request, so an ancestor that later widens again restores what it
// had cut back rather than shrinking further each time. It clamps rather than refuses because the
// request was accepted once already, and a sharee's old request must not stop the owner from
// tightening its own resource. The type's member check is not repeated: the clamped result cannot
// exceed the ceiling, and that ceiling's own members were validated when it was written.
func (s *sharingService) clampOne(
	ctx context.Context, rt ResourceType, resourceID, fieldKey, targetID string,
	initiatorRules map[string]OverlayRule, requested OverlayRule,
) (StoredRule, *tidcommon.ServiceError) {
	decl, ok := s.registry.field(rt, fieldKey)
	if !ok {
		return StoredRule{}, withDetail(ErrorUnknownFieldKey, fieldKey)
	}
	c, svcErr := s.containmentFor(ctx, rt, resourceID, fieldKey, decl.Kind)
	if svcErr != nil {
		return StoredRule{}, svcErr
	}
	parent := s.effectiveOrDefault(rt, fieldKey, initiatorRules)

	return StoredRule{
		FieldKey:  fieldKey,
		TargetID:  targetID,
		Resolved:  copyRule(c.clamp(parent, requested)),
		Requested: requested,
	}, nil
}

// narrowOne folds one requested rule and reports the offending field on rejection.
func (s *sharingService) narrowOne(
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

	resolved, err := c.narrow(parent, requested)
	if err != nil {
		return StoredRule{}, mapEngineError(err, fieldKey)
	}

	return StoredRule{
		FieldKey:  fieldKey,
		TargetID:  targetID,
		Resolved:  copyRule(resolved),
		Requested: requested,
	}, nil
}

// validateMembers asks the resource type whether the initiator may name these members, because
// nothing in the narrowing algebra stops an organization unit pinning an id belonging to another.
func (s *sharingService) validateMembers(
	ctx context.Context, rt ResourceType, resourceID, fieldKey, initiatingOUID string, r OverlayRule,
) *tidcommon.ServiceError {
	var members []string
	for _, set := range []*[]string{r.Value, r.AllowedValues, r.ExcludedValues} {
		if set != nil {
			members = append(members, *set...)
		}
	}
	if len(members) == 0 {
		return nil
	}
	decl, ok := s.registry.get(rt)
	if !ok {
		return nil
	}
	validator, ok := decl.(MemberValidator)
	if !ok {
		return nil
	}
	svcErr := validator.ValidateMembers(ctx, resourceID, fieldKey, initiatingOUID,
		utils.UniqueStrings(members))
	if svcErr == nil {
		return nil
	}

	fields := []log.Field{
		log.String("resourceType", string(rt)), log.String("resourceID", resourceID),
		log.String("fieldKey", fieldKey), log.String("initiatingOUID", initiatingOUID),
		log.String("code", svcErr.Code),
	}
	// A check that could not run has not refused anything, so it must not be reported as a refusal:
	// telling a caller its members are not visible when a store was unreachable sends it to fix a
	// request that was fine.
	if svcErr.Type != tidcommon.ClientErrorType {
		s.logger.Error(ctx, "Resource type failed to validate the members named by an overlay rule", fields...)
		return &tidcommon.InternalServerError
	}

	// Debug, not error: the caller asked for something it may not have, which is its mistake to fix
	// rather than a fault here. The reason is still worth recording somewhere, because the refusal
	// deliberately carries none of it to the caller, so an operator debugging a rejected rule would
	// otherwise have only the field name.
	s.logger.Debug(ctx, "Resource type refused the members named by an overlay rule", fields...)
	return withDetail(ErrorMemberNotVisible, fieldKey)
}

// effectiveOrDefault returns the initiator's own rule for a field, falling back to the coarser
// field it declares and then to the type's declared default.
func (s *sharingService) effectiveOrDefault(
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
// It also reports whether the initiator can see the resource at all, which the resolution already
// worked out. A caller that skipped that would read an empty rule set as "no opinion" and reach for
// the type's defaults, which have nothing to do with the ceiling the initiator actually held.
func (s *sharingService) initiatorEffectiveRules(
	ctx context.Context, rt ResourceType, resourceID, owningOUID, initiatingOUID string,
) (map[string]OverlayRule, bool, *tidcommon.ServiceError) {
	if initiatingOUID == owningOUID {
		return map[string]OverlayRule{}, true, nil
	}
	// Uncached on purpose: this runs inside the write transaction.
	resolved, svcErr := s.resolveOverlayRules(ctx, rt, resourceID, initiatingOUID)
	if svcErr != nil {
		return nil, false, svcErr
	}
	return resolved.Rules, resolved.Visible, nil
}

// UpdatePolicy replaces a policy's contents within the limits of its scope family.
func (s *sharingService) UpdatePolicy(
	ctx context.Context, policyID string, req PolicyRequest,
) (Policy, *tidcommon.ServiceError) {
	current, svcErr := s.GetPolicy(ctx, policyID)
	if svcErr != nil {
		return Policy{}, svcErr
	}
	// A declared policy is its file, and never has a stored row. Changing where sharing starts is
	// an edit to that file; what an organization unit the declaration reached may do here is issue
	// a policy of its own, which is a reshare rather than a change to this policy's reach.
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

	if err := validateEdit(
		current, proposed, req.Version, current.Version,
	); err != nil {
		return Policy{}, mapEditError(err)
	}
	if svcErr := s.blanketRulesNarrowOnly(ctx, current, proposed); svcErr != nil {
		return Policy{}, svcErr
	}
	if svcErr := s.requireEditLeavesFrontiersIntact(ctx, proposed); svcErr != nil {
		return Policy{}, svcErr
	}

	if err := s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		// Read before the write, for the same reason the delete path does: cleanup needs to know
		// which organization units the reshares beneath this policy had reached.
		before, err := s.store.ListAllPoliciesForResource(txCtx, current.ResourceType, current.ResourceID)
		if err != nil {
			return err
		}
		if err := s.store.ReplacePolicyContents(txCtx, proposed, current.Version); err != nil {
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
		return s.cleanUpLostVisibility(txCtx, current, before)
	}); err != nil {
		if errors.Is(err, errPolicyNotFound) {
			return Policy{}, &ErrorVersionMismatch
		}
		s.logger.Error(ctx, "Failed to update sharing policy", log.Error(err))
		return Policy{}, &tidcommon.InternalServerError
	}

	s.clearCaches(ctx)
	proposed.Version = current.Version + 1
	return proposed, nil
}

// canonicalParentID records the covering policy as a reshare's parent. Only a frontier unit may
// reshare, so there is one such policy, and the parent is a real dependency rather than a hint:
// if it goes, its issuer stops seeing the resource and every policy that issuer wrote is dead.
//
// A declared parent is recorded like any other. It has no database row, so the column holds its id
// as a plain value rather than a foreign key, which is how a stored role assignment references a
// role its own file declares. Declarations carry their own ids for exactly this reason.
func canonicalParentID(covering []Coverage) string {
	out := ""
	for _, c := range covering {
		if out == "" || c.Policy.ID < out {
			out = c.Policy.ID
		}
	}
	return out
}

// blanketRulesNarrowOnly reports whether an edit keeps every restriction a blanket policy's rules
// currently carry.
func (s *sharingService) blanketRulesNarrowOnly(
	ctx context.Context, current, proposed Policy,
) *tidcommon.ServiceError {
	if policyFamily(current) != familyBlanket {
		return nil
	}

	// ruleKey identifies one stored rule: a field and the target carrying it. The target is keyed
	// by what it selects rather than by its row id, because an edit mints fresh target rows and
	// the ids would never line up across the two policies.
	type ruleKey struct {
		fieldKey string
		scope    TargetScope
		ouID     string
	}
	keyOf := func(p Policy, r StoredRule) (ruleKey, bool) {
		for _, t := range p.Targets {
			if t.ID == r.TargetID {
				return ruleKey{r.FieldKey, t.Scope, t.OUID}, familyOf(t.Scope) == familyBlanket
			}
		}
		return ruleKey{fieldKey: r.FieldKey}, false
	}

	next := make(map[ruleKey]OverlayRule, len(proposed.Rules))
	for _, r := range proposed.Rules {
		k, _ := keyOf(proposed, r)
		next[k] = r.Resolved
	}

	for _, stored := range current.Rules {
		// A policy may hold a selective target beside a blanket one. Only the blanket target has
		// nothing to expand toward; the named one is bounded by the one-hop rule wherever it
		// appears, so its terms stay editable in both directions.
		k, blanket := keyOf(current, stored)
		if !blanket {
			continue
		}
		proposedRule, kept := next[k]
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
		if c.widens(stored.Resolved, proposedRule) {
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
func (s *sharingService) rematerializeDependents(ctx context.Context, edited Policy) error {
	policies, err := s.store.ListAllPoliciesForResource(ctx, edited.ResourceType, edited.ResourceID)
	if err != nil {
		return err
	}

	dependents := make([]Policy, 0, len(policies))
	for _, p := range policies {
		if p.ID == edited.ID || p.Declared || p.Stage != stageReshare {
			continue
		}
		dependents = append(dependents, p)
	}

	ordered, svcErr := s.orderByInitiatorDepth(ctx, dependents)
	if svcErr != nil {
		return fmt.Errorf("failed to order policies for re-materialization: %s", svcErr.Code)
	}

	for _, descendant := range ordered {
		initiatorRules, visible, svcErr := s.initiatorEffectiveRules(ctx, descendant.ResourceType,
			descendant.ResourceID, descendant.OwningOUID, descendant.InitiatingOUID)
		if svcErr != nil {
			return fmt.Errorf("failed to resolve rules for policy %s: %s", descendant.ID, svcErr.Code)
		}
		// This edit may have taken the resource away from the initiator. There is no ceiling to
		// re-narrow against then, and resolution answers an invisible unit with empty rules, which
		// would be read as "no policy has an opinion" and send the clamp to the type's defaults.
		// Those can be wider than the ceiling this policy held, so the dead policy would be rewritten
		// wider than it ever was. Cleanup deletes it moments later; it is not this pass's to fix up.
		if !visible {
			continue
		}

		rebuilt := make([]StoredRule, 0, len(descendant.Rules))
		for _, stored := range descendant.Rules {
			// Re-narrowing starts from the original request, so an ancestor that later widens
			// again restores what it had clamped rather than ratcheting permanently downward.
			next, svcErr := s.clampOne(ctx, descendant.ResourceType, descendant.ResourceID,
				stored.FieldKey, stored.TargetID, initiatorRules, stored.Requested)
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
func (s *sharingService) orderByInitiatorDepth(
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
func (s *sharingService) DeletePolicy(ctx context.Context, policyID string) *tidcommon.ServiceError {
	policy, svcErr := s.GetPolicy(ctx, policyID)
	if svcErr != nil {
		return svcErr
	}
	if policy.Declared {
		return &ErrorPolicyDeclared
	}

	if err := s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		// Listed before the delete, not after. PARENT_POLICY_ID cascades, so the reshares beneath
		// this policy are gone by the time it returns, and cleanup would never learn which
		// organization units they had reached. Those units are exactly the ones losing the resource.
		before, err := s.store.ListAllPoliciesForResource(txCtx, policy.ResourceType, policy.ResourceID)
		if err != nil {
			return err
		}
		if err := s.store.DeletePolicy(txCtx, policyID); err != nil {
			return err
		}
		return s.cleanUpLostVisibility(txCtx, policy, before)
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
func (s *sharingService) cleanUpLostVisibility(ctx context.Context, changed Policy, before []Policy) error {
	issuedBy := make(map[string][]Policy, len(before))
	for _, p := range before {
		issuedBy[p.InitiatingOUID] = append(issuedBy[p.InitiatingOUID], p)
	}

	queue, err := s.candidatesFrom(ctx, changed)
	if err != nil {
		return err
	}

	checked := make(map[string]struct{}, len(queue))
	for len(queue) > 0 {
		ouID := queue[0]
		queue = queue[1:]
		if _, done := checked[ouID]; done {
			continue
		}
		checked[ouID] = struct{}{}

		stillVisible, _, svcErr := s.resolveVisibility(
			ctx, changed.ResourceType, changed.ResourceID, changed.OwningOUID, ouID)
		if svcErr != nil {
			return fmt.Errorf("failed to resolve visibility for %s: %s", ouID, svcErr.Code)
		}
		if stillVisible {
			continue
		}

		if err := s.clearOverlayValues(ctx, changed.ResourceType, changed.ResourceID, ouID); err != nil {
			return err
		}
		if err := s.notifyVisibilityLost(
			ctx, changed.ResourceType, changed.ResourceID, ouID); err != nil {
			return err
		}

		// Nothing this unit shared onward can stand now that the unit itself cannot see the
		// resource, so everything those policies reached joins the queue and the policies go.
		//
		// They are deleted here rather than by a foreign key cascade, because PARENT_POLICY_ID
		// carries no foreign key: a declared parent has no row to reference. A declared policy is
		// never deleted through this path, so it is left alone.
		for _, p := range issuedBy[ouID] {
			downstream, err := s.candidatesFrom(ctx, p)
			if err != nil {
				return err
			}
			queue = append(queue, downstream...)

			if p.Declared {
				continue
			}
			if err := s.store.DeletePolicy(ctx, p.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

// candidatesFrom expands one policy's targets into the organization units it reached, which are the
// ones worth re-checking after it changed.
func (s *sharingService) candidatesFrom(ctx context.Context, p Policy) ([]string, error) {
	return s.ousReachedBy(ctx, p)
}

// clearOverlayValues drops the values a unit chose for a resource it can no longer see.
//
// A resource type that keeps those values in its own storage rather than the framework's implements
// OverlayCleaner and is called instead, since the framework's table holds nothing for it to delete.
func (s *sharingService) clearOverlayValues(
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
func (s *sharingService) notifyVisibilityLost(
	ctx context.Context, rt ResourceType, resourceID, ouID string,
) error {
	decl, ok := s.registry.get(rt)
	if !ok {
		return nil
	}
	hooks, ok := decl.(PolicyHooks)
	if !ok {
		return nil
	}
	return hooks.OnVisibilityLost(ctx, resourceID, ouID)
}

// ousReachedBy enumerates every organization unit a policy's targets could have reached.
//
// A target records an anchor rather than the set it covers, so the units that may have just lost
// visibility are almost never the ones named on the targets: a subtree or root target reaches
// everything below its anchor, an all-children target reaches everything below its initiator but
// not the initiator itself, and the blanket scopes reach the deployment.
func (s *sharingService) ousReachedBy(ctx context.Context, p Policy) ([]string, error) {
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
		case ScopeAllOUs, ScopeAllRoots:
			// Losing a root cuts off everything below it, so the whole deployment is in scope.
			if s.ouEnumerator == nil {
				continue
			}
			all, svcErr := s.ouEnumerator.AllOUIDs(ctx)
			if svcErr != nil {
				return nil, fmt.Errorf("failed to enumerate organization units: %s", svcErr.Code)
			}
			out = append(out, all...)
		case ScopeAllChildren:
			// The anchor is the initiator, which keeps its own visibility; only what lies beneath
			// it was reached by this policy.
			if err := appendSubtree(t.OUID, false); err != nil {
				return nil, err
			}
		case ScopeChildSubtree, ScopeRoot:
			if err := appendSubtree(t.OUID, true); err != nil {
				return nil, err
			}
		case ScopeChild:
			out = append(out, t.OUID)
		}
	}

	return utils.UniqueStrings(out), nil
}

// GetPolicy returns one policy by id.
func (s *sharingService) GetPolicy(ctx context.Context, policyID string) (Policy, *tidcommon.ServiceError) {
	p, err := s.store.GetPolicy(ctx, policyID)
	if err == nil {
		return p, nil
	}
	if !errors.Is(err, errPolicyNotFound) {
		s.logger.Error(ctx, "Failed to get sharing policy", log.Error(err))
		return Policy{}, &tidcommon.InternalServerError
	}
	// Neither stored nor declared: not a policy at all.
	return Policy{}, &ErrorPolicyNotFound
}

// ListPolicies returns every policy recorded for a resource, declared ones included.
func (s *sharingService) ListPolicies(
	ctx context.Context, rt ResourceType, resourceID string,
) ([]Policy, *tidcommon.ServiceError) {
	stored, err := s.store.ListAllPoliciesForResource(ctx, rt, resourceID)
	if err != nil {
		s.logger.Error(ctx, "Failed to list sharing policies", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	return stored, nil
}

// GetPolicyList returns one page of a resource's policies.
//
// This is the only read in the framework that may answer with part of a resource's policies. It is
// safe here because a listing is shown to a person, whereas every evaluation read decides coverage
// from the set as a whole and goes through ListPoliciesForResource instead.
func (s *sharingService) GetPolicyList(
	ctx context.Context, rt ResourceType, resourceID string, limit, offset int,
) (PolicyList, *tidcommon.ServiceError) {
	if svcErr := validatePaginationParams(limit, offset); svcErr != nil {
		return PolicyList{}, svcErr
	}

	total, err := s.store.CountPoliciesForResource(ctx, rt, resourceID)
	if err != nil {
		return PolicyList{}, s.listingError(ctx, err)
	}
	policies, err := s.store.ListPoliciesForResource(ctx, rt, resourceID, limit, offset)
	if err != nil {
		return PolicyList{}, s.listingError(ctx, err)
	}

	return PolicyList{
		TotalResults: total,
		StartIndex:   offset + 1,
		Count:        len(policies),
		Policies:     policies,
	}, nil
}

// validatePaginationParams refuses a page the store should never be asked for.
func validatePaginationParams(limit, offset int) *tidcommon.ServiceError {
	if limit < 1 || limit > serverconst.MaxPageSize {
		return &ErrorInvalidLimit
	}
	if offset < 0 {
		return &ErrorInvalidOffset
	}
	return nil
}

// listingError separates the one failure a caller can act on, by listing a resource with fewer
// policies, from the ones only an operator can.
func (s *sharingService) listingError(ctx context.Context, err error) *tidcommon.ServiceError {
	if errors.Is(err, errResultLimitExceededInCompositeMode) {
		return &ErrorResultLimitExceededInCompositeMode
	}
	s.logger.Error(ctx, "Failed to list sharing policies", log.Error(err))
	return &tidcommon.InternalServerError
}

// ExportPolicies returns a resource's policies in an order safe to replay sequentially: the policy
// that made an initiator visible always precedes the policy that initiator issued.
func (s *sharingService) ExportPolicies(
	ctx context.Context, rt ResourceType, resourceID string,
) ([]ReplayablePolicy, *tidcommon.ServiceError) {
	policies, svcErr := s.ListPolicies(ctx, rt, resourceID)
	if svcErr != nil {
		return nil, svcErr
	}

	// Policies are queued per id rather than stored one-per-id, so two entries sharing an id both
	// reach the export instead of one silently replacing the other.
	queued := make(map[string][]Policy, len(policies))
	lineages := make([]Lineage, 0, len(policies))
	for _, p := range policies {
		queued[p.ID] = append(queued[p.ID], p)
		lineages = append(lineages, Lineage{ID: p.ID, ParentID: p.ParentPolicyID})
	}

	ordered := orderByDependency(lineages)
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
func (s *sharingService) IsVisible(
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

	visible, _ := evaluateChain(chain, policies)
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
func (s *sharingService) ownerOf(
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
func (s *sharingService) ResolveOverlayRules(
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
func (s *sharingService) resolveOverlayRules(
	ctx context.Context, rt ResourceType, resourceID, ouID string,
) (ResolvedOverlay, *tidcommon.ServiceError) {
	scope, svcErr := s.overlayScopeFor(ctx, rt, resourceID, ouID)
	if svcErr != nil {
		return ResolvedOverlay{}, svcErr
	}

	out := ResolvedOverlay{
		OUID:    ouID,
		Owned:   scope.owned,
		Visible: scope.visible,
		Rules:   map[string]OverlayRule{},
		Sources: map[string]string{},
	}

	// An organization unit that cannot see the resource is not answered with the type's defaults.
	// Those describe what an organization unit holding the resource may do when no policy says
	// otherwise, which is a different statement from "this one may do nothing".
	if !out.Visible {
		out.PolicyIDs = []string{}
		return out, nil
	}

	// Every declared field gets an answer, so a caller never has to distinguish "no rule" from
	// "no opinion" itself.
	decl, ok := s.registry.get(rt)
	if !ok {
		return out, nil
	}
	for _, f := range decl.Fields() {
		rule, source, svcErr := s.resolvedRule(ctx, rt, resourceID, f.Key, f.Kind, scope)
		if svcErr != nil {
			return ResolvedOverlay{}, svcErr
		}
		if source == "" {
			continue
		}
		out.Rules[f.Key] = rule
		out.Sources[f.Key] = source
	}

	for _, c := range scope.covering {
		out.PolicyIDs = append(out.PolicyIDs, c.Policy.ID)
	}
	out.PolicyIDs = utils.UniqueStrings(out.PolicyIDs)

	return out, nil
}

// overlayScope is the state every per-field resolution for one organization unit starts from:
// whether the unit holds the resource at all, and the policies covering it indexed for rule lookup.
//
// It exists because resolution is asked two different ways. ResolveOverlayRules walks the fields a
// type declares; a write to one field, or a read of a dynamic member, asks about a single key that
// has no entry of its own in that walk. Both need the same chain, the same fetch and the same
// ownership answer, and computing them twice would let the two disagree.
type overlayScope struct {
	owned    bool
	visible  bool
	covering []Coverage
	byID     map[string]Policy
}

// overlayScopeFor resolves one organization unit's standing with respect to a resource.
func (s *sharingService) overlayScopeFor(
	ctx context.Context, rt ResourceType, resourceID, ouID string,
) (overlayScope, *tidcommon.ServiceError) {
	chain, svcErr := s.buildChain(ctx, ouID)
	if svcErr != nil {
		return overlayScope{}, svcErr
	}
	policies, svcErr := s.relevantPolicies(ctx, rt, resourceID, chain)
	if svcErr != nil {
		return overlayScope{}, svcErr
	}

	var out overlayScope

	// Ownership is asked of the resource type and nowhere else, so the owner is answered even for a
	// resource nobody has shared yet. Registration refuses a type that cannot answer, and a type
	// answering with no owner is reporting a resource it does not know: that is a denial rather
	// than something to second-guess from a policy the narrowed fetch may not even have returned.
	owner, known, svcErr := s.ownerOf(ctx, rt, resourceID)
	if svcErr != nil {
		return overlayScope{}, svcErr
	}
	out.owned = known && owner == ouID

	visible, covering := evaluateChain(chain, policies)
	out.visible = visible || out.owned
	out.covering = covering

	out.byID = make(map[string]Policy, len(policies))
	for _, p := range policies {
		out.byID[p.ID] = p
	}
	return out, nil
}

// resolvedRule folds what every covering policy says about one field key into the rule governing
// it, and reports where that rule came from. An empty source means nothing governs the field: no
// policy named it and the type declares no default for it.
func (s *sharingService) resolvedRule(
	ctx context.Context, rt ResourceType, resourceID, fieldKey string, kind FieldKind, scope overlayScope,
) (OverlayRule, string, *tidcommon.ServiceError) {
	contributing := contributingRules(scope.covering, scope.byID, fieldKey)

	// Nothing named this field, so the coarser field it falls back to governs it. Without this a
	// lock on the coarse field would not reach the finer one, and naming the finer field would be a
	// way around the lock: its own default would answer instead. For a dynamic member the coarser
	// field is the prefix that declared it, which is what registry.fallbackKey resolves first.
	if len(contributing) == 0 {
		if fallback, ok := s.registry.fallbackKey(rt, fieldKey); ok {
			contributing = contributingRules(scope.covering, scope.byID, fallback)
		}
	}

	// Only now is it true that no policy has an opinion, so the type's default answers. That is the
	// same answer for an owner and a sharee: an owner is bounded by nothing it did not declare
	// itself, and a field no policy named is exactly that for either of them.
	if len(contributing) == 0 {
		if d, ok := s.registry.defaultRule(rt, fieldKey); ok {
			return d, SourceDefault, nil
		}
		return OverlayRule{}, "", nil
	}

	c, svcErr := s.containmentFor(ctx, rt, resourceID, fieldKey, kind)
	if svcErr != nil {
		return OverlayRule{}, "", svcErr
	}
	return copyRule(c.intersect(contributing)), SourcePolicy, nil
}

// ResolveOverlayValues returns the effective value per field for one organization unit.
//
// Not served from the overlay rule cache, and not cached itself. That cache holds resolved rules,
// which only a policy write invalidates; a value changes without any policy changing, so serving
// values from it would hand back the value as it stood before the unit last edited it.
func (s *sharingService) ResolveOverlayValues(
	ctx context.Context, rt ResourceType, resourceID, ouID string,
) (ResolvedValues, *tidcommon.ServiceError) {
	if !s.ownsOverlayValues(ctx, rt) {
		return ResolvedValues{}, &tidcommon.InternalServerError
	}

	scope, svcErr := s.overlayScopeFor(ctx, rt, resourceID, ouID)
	if svcErr != nil {
		return ResolvedValues{}, svcErr
	}

	out := ResolvedValues{
		OUID:    ouID,
		Owned:   scope.owned,
		Visible: scope.visible,
		Values:  map[string]ResolvedValue{},
	}
	if !out.Visible {
		return out, nil
	}

	stored, svcErr := s.storedOverlayValues(ctx, rt, resourceID, ouID)
	if svcErr != nil {
		return ResolvedValues{}, svcErr
	}

	for _, key := range s.valueKeys(rt, stored) {
		f, ok := s.registry.field(rt, key)
		if !ok {
			// A value stored under a key the type no longer declares. Reporting it would claim the
			// unit holds something of a resource the type has since stopped exposing.
			continue
		}
		rule, source, svcErr := s.resolvedRule(ctx, rt, resourceID, key, f.Kind, scope)
		if svcErr != nil {
			return ResolvedValues{}, svcErr
		}
		if source == "" {
			continue
		}
		c, svcErr := s.containmentFor(ctx, rt, resourceID, key, f.Kind)
		if svcErr != nil {
			return ResolvedValues{}, svcErr
		}

		// A stored row is a choice even when it holds nothing: clearing a field is something the
		// unit did, and reading it back as no choice at all would restore the rule's value under it.
		// The distinction rides on nil versus empty, and a cleared set round-trips through JSON null.
		var chosen []string
		raw, hasChoice := stored[key]
		if hasChoice {
			chosen = []string{}
			if raw != nil {
				chosen = raw
			}
		}
		value := ResolvedValue{
			Value:    c.effective(rule, chosen),
			Source:   SourceRule,
			Editable: rule.Editable,
		}
		// A choice only counts as one while the rule still lets the unit make it. Once the field is
		// pinned the stored row is ignored, so reporting it as the unit's own would be wrong.
		if hasChoice && rule.Editable {
			value.Source = SourceOverlay
		}
		out.Values[key] = value
	}
	return out, nil
}

// valueKeys returns every field key worth resolving a value for: the keys the type declares, plus
// the keys this organization unit has stored a value under.
//
// The second half is what covers dynamic members. A type declaring "nodes" with DynamicKeys never
// enumerates "nodes.mfa-step", so a member the unit has chosen a value for is only reachable
// through what it stored. The result is sorted, so two calls answer in the same order.
func (s *sharingService) valueKeys(rt ResourceType, stored map[string][]string) []string {
	keys := make([]string, 0, len(stored))
	seen := make(map[string]struct{}, len(stored))
	add := func(k string) {
		if _, dup := seen[k]; dup {
			return
		}
		seen[k] = struct{}{}
		keys = append(keys, k)
	}
	if decl, ok := s.registry.get(rt); ok {
		for _, f := range decl.Fields() {
			add(f.Key)
		}
	}
	for k := range stored {
		add(k)
	}
	sort.Strings(keys)
	return keys
}

// SetOverlayValue records one organization unit's own value for one field of a shared resource.
func (s *sharingService) SetOverlayValue(
	ctx context.Context, rt ResourceType, resourceID, ouID, fieldKey string, value []string,
) *tidcommon.ServiceError {
	rule, kind, svcErr := s.writableRule(ctx, rt, resourceID, ouID, fieldKey)
	if svcErr != nil {
		return svcErr
	}

	c, svcErr := s.containmentFor(ctx, rt, resourceID, fieldKey, kind)
	if svcErr != nil {
		return svcErr
	}
	// Refused rather than clamped: the unit is choosing now, and silently storing something other
	// than what it asked for would leave it believing a value it does not hold. Clamping belongs on
	// the read path, where the rule may have tightened since.
	if !c.permits(rule, value) {
		return withDetail(ErrorValueNotPermitted, fieldKey)
	}

	if err := s.store.SetOverlayValue(ctx, rt, resourceID, ouID, fieldKey, value); err != nil {
		s.logger.Error(ctx, "Failed to set overlay value",
			log.String("resourceType", string(rt)), log.String("resourceID", resourceID),
			log.String("ouID", ouID), log.String("fieldKey", fieldKey), log.Error(err))
		return &tidcommon.InternalServerError
	}
	return nil
}

// DeleteOverlayValue drops an organization unit's own value for one field.
//
// Editability is checked the same way a write is. A pinned field holds no choice of the unit's to
// drop, and answering a delete there with success would report that one had been removed.
func (s *sharingService) DeleteOverlayValue(
	ctx context.Context, rt ResourceType, resourceID, ouID, fieldKey string,
) *tidcommon.ServiceError {
	if _, _, svcErr := s.writableRule(ctx, rt, resourceID, ouID, fieldKey); svcErr != nil {
		return svcErr
	}

	if err := s.store.DeleteOverlayValue(ctx, rt, resourceID, ouID, fieldKey); err != nil {
		s.logger.Error(ctx, "Failed to delete overlay value",
			log.String("resourceType", string(rt)), log.String("resourceID", resourceID),
			log.String("ouID", ouID), log.String("fieldKey", fieldKey), log.Error(err))
		return &tidcommon.InternalServerError
	}
	return nil
}

// writableRule resolves the rule governing one field for one organization unit and reports whether
// that unit may write it, which is the check every overlay value write shares.
//
// Resolution goes through the uncached path. The cached entry point answers for declared field keys
// only, and a dynamic member has no entry there; resolving the single key asked about is both the
// correct answer and a narrower one.
func (s *sharingService) writableRule(
	ctx context.Context, rt ResourceType, resourceID, ouID, fieldKey string,
) (OverlayRule, FieldKind, *tidcommon.ServiceError) {
	if !s.ownsOverlayValues(ctx, rt) {
		return OverlayRule{}, "", &tidcommon.InternalServerError
	}

	f, ok := s.registry.field(rt, fieldKey)
	if !ok {
		return OverlayRule{}, "", withDetail(ErrorUnknownFieldKey, fieldKey)
	}

	scope, svcErr := s.overlayScopeFor(ctx, rt, resourceID, ouID)
	if svcErr != nil {
		return OverlayRule{}, "", svcErr
	}
	if !scope.visible {
		return OverlayRule{}, "", &ErrorNotShared
	}

	rule, source, svcErr := s.resolvedRule(ctx, rt, resourceID, fieldKey, f.Kind, scope)
	if svcErr != nil {
		return OverlayRule{}, "", svcErr
	}
	// Nothing governs the field: no policy named it and the type declares no default. There is no
	// rule granting the unit a choice, so there is none to write against.
	if source == "" || !rule.Editable {
		return OverlayRule{}, "", withDetail(ErrorFieldNotEditable, fieldKey)
	}
	return rule, f.Kind, nil
}

// ownsOverlayValues reports whether the framework's own table is where this resource type's overlay
// values live.
//
// A type implementing OverlayCleaner has said they live somewhere else, and the framework never
// touches its own table for that type again. These three operations therefore have nothing to
// answer with: reading would report no choices for a unit that has made some, and writing would
// leave rows behind that cleanup, which defers to the type, will never come back for. Only the type
// itself can serve them, so it is told rather than silently half-served.
func (s *sharingService) ownsOverlayValues(ctx context.Context, rt ResourceType) bool {
	decl, ok := s.registry.get(rt)
	if !ok {
		return true
	}
	if _, external := decl.(OverlayCleaner); external {
		s.logger.Error(ctx, "Resource type keeps its own overlay values and cannot use the framework's",
			log.String("resourceType", string(rt)))
		return false
	}
	return true
}

// storedOverlayValues reads the values one organization unit has chosen for a resource.
func (s *sharingService) storedOverlayValues(
	ctx context.Context, rt ResourceType, resourceID, ouID string,
) (map[string][]string, *tidcommon.ServiceError) {
	stored, err := s.store.GetOverlayValues(ctx, rt, resourceID, ouID)
	if err != nil {
		s.logger.Error(ctx, "Failed to read overlay values",
			log.String("resourceType", string(rt)), log.String("resourceID", resourceID),
			log.String("ouID", ouID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	return stored, nil
}

// contributingRules collects what every covering policy says about one field key.
func contributingRules(
	covering []Coverage, byID map[string]Policy, fieldKey string,
) []OverlayRule {
	out := make([]OverlayRule, 0, len(covering))
	for _, c := range covering {
		p, ok := byID[c.Policy.ID]
		if !ok {
			continue
		}
		if r, found := ruleFor(p, c.TargetID, fieldKey); found {
			out = append(out, r)
		}
	}
	return out
}

// ruleFor returns the rule the covering target carries for one field. Terms travel with the
// target, so the target that reached the organization unit is the one whose terms apply.
func ruleFor(p Policy, targetID, fieldKey string) (OverlayRule, bool) {
	for _, r := range p.Rules {
		if r.FieldKey == fieldKey && r.TargetID == targetID {
			return r.Resolved, true
		}
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
func (s *sharingService) relevantPolicies(
	ctx context.Context, rt ResourceType, resourceID string, chain []string,
) ([]Policy, *tidcommon.ServiceError) {
	policies, err := s.store.ListPoliciesRelevantToChain(ctx, rt, resourceID, chain)
	if err != nil {
		s.logger.Error(ctx, "Failed to list policies relevant to an organization unit chain",
			log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	return policies, nil
}

// resolveVisibility answers whether one organization unit can see a resource, and through which
// policies, without consulting the cache.
func (s *sharingService) resolveVisibility(
	ctx context.Context, rt ResourceType, resourceID, owningOUID, ouID string,
) (bool, []Coverage, *tidcommon.ServiceError) {
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
	visible, covering := evaluateChain(chain, policies)
	return visible, covering, nil
}

// buildChain returns the organization unit chain, tree root first and ouID last.
func (s *sharingService) buildChain(ctx context.Context, ouID string) ([]string, *tidcommon.ServiceError) {
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
func (s *sharingService) clearCaches(ctx context.Context) {
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
func (s *sharingService) ListVisibleResourceIDs(
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
		if visible, _ := evaluateChain(chain, byResource[resourceID]); visible {
			out = append(out, resourceID)
		}
	}
	return out, nil
}
