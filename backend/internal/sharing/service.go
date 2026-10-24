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
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
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
	// ReplacePoliciesForResource applies the policies an imported document carries for one resource.
	ReplacePoliciesForResource(ctx context.Context, rt ResourceType, resourceID, owningOUID string,
		requests []PolicyRequest) (PolicyReplay, *PolicyReplayError)
	// DeletePoliciesForResource removes every policy recorded for one resource; its delete must call it.
	DeletePoliciesForResource(ctx context.Context, rt ResourceType, resourceID string) *tidcommon.ServiceError
	// GetResourceDependencies reports the policies an organization unit issued, each of which keeps
	// the unit from being deleted. It implements resourcedependency.Provider.
	GetResourceDependencies(
		ctx context.Context, resourceType, id string,
	) ([]resourcedependency.ResourceDependency, error)
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
	// a sharing policy, sorted by ID. Resources it owns are not included: ownership is the resource
	// type's own state, and this framework stores no resource rows to read it from.
	ListVisibleResourceIDs(
		ctx context.Context, rt ResourceType, ouID string,
	) ([]string, *tidcommon.ServiceError)
	// ListResourceSharedOUIDs returns the organization units a resource's policies reach. It is the reverse
	// of ListVisibleResourceIDs and leaves the owner out the same way.
	ListResourceSharedOUIDs(ctx context.Context, rt ResourceType, resourceID string) ([]string, *tidcommon.ServiceError)
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

// CreatePolicy records one organization unit's decision about one resource, as an interactive
// create. A replay goes through this package's own path instead, since it keeps the id and version
// its document carries.
func (s *sharingService) CreatePolicy(
	ctx context.Context, rt ResourceType, resourceID, owningOUID string, req PolicyRequest,
) (Policy, *tidcommon.ServiceError) {
	// The id and the version are the framework's on an interactive create: a caller choosing
	// either would be replaying something it has no document for.
	req.ID = ""
	req.Version = 0
	return s.createPolicy(ctx, rt, resourceID, owningOUID, req, false)
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
	return s.createPolicy(ctx, rt, resourceID, owningOUID, req, true)
}

// createPolicy is the shared body behind CreatePolicy and CreateDeclarativePolicy. A declared
// policy is seeded into the in-memory store instead of the database, which is the only step the
// two do differently.
func (s *sharingService) createPolicy(
	ctx context.Context, rt ResourceType, resourceID, owningOUID string, req PolicyRequest,
	declared bool,
) (Policy, *tidcommon.ServiceError) {
	if _, ok := s.registry.get(rt); !ok {
		return Policy{}, &ErrorResourceTypeNotRegistered
	}
	// A stored policy's version counts the edits made to it in this deployment, so a new one starts
	// at the first, wherever its request came from. A version a document carries describes edits
	// made somewhere else.
	if !declared {
		req.Version = 0
	}
	if resourceID == "" {
		return Policy{}, withDetail(ErrorInvalidRequestFormat, "the resource is not named")
	}
	if owningOUID == "" {
		return Policy{}, withDetail(ErrorInvalidRequestFormat, "the resource has no owning organization unit")
	}
	initiatingOUID := req.InitiatingOUID
	if initiatingOUID == "" {
		initiatingOUID = owningOUID
	}

	if declared {
		if svcErr := s.requireUsableDeclaredID(ctx, req.ID, rt, resourceID, initiatingOUID); svcErr != nil {
			return Policy{}, svcErr
		}
	} else if req.ID != "" {
		id, svcErr := s.replayableStoredID(ctx, req.ID, rt, resourceID)
		if svcErr != nil {
			return Policy{}, svcErr
		}
		req.ID = id
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
	ctx context.Context, id string, rt ResourceType, resourceID, initiatingOUID string,
) *tidcommon.ServiceError {
	if id == "" {
		return withDetail(ErrorDeclaredPolicyIDRequired, string(rt)+" "+resourceID)
	}
	// A declaration never has a database row of its own, so an id the database holds belongs to
	// some other policy. Lookups answer from the database first, so letting the declaration keep
	// the id would leave it unreachable by that id while still listed under its resource.
	if _, err := s.dbStore.GetPolicy(ctx, id); err == nil {
		return withDetail(ErrorDeclaredPolicyIDConflict, id)
	} else if !errors.Is(err, errPolicyNotFound) {
		s.logger.Error(ctx, "Failed to check a declared policy id against stored policies",
			log.String("policyID", id), log.Error(err))
		return &tidcommon.InternalServerError
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

// replayableStoredID decides the id a replayed policy is stored under. A free id is kept, so a
// round trip reproduces the policy it exported. An id another resource's policy holds means the
// document was copied to make a new resource, and nothing in it depends on that id, since a
// reshare's parent is worked out again on replay; the policy gets a fresh one, signaled by an
// empty id. An id another policy of the same resource holds is a real conflict, answered as one
// rather than as a failed insert.
func (s *sharingService) replayableStoredID(
	ctx context.Context, id string, rt ResourceType, resourceID string,
) (string, *tidcommon.ServiceError) {
	holder, svcErr := s.GetPolicy(ctx, id)
	switch {
	case svcErr == nil && (holder.ResourceType != rt || holder.ResourceID != resourceID):
		return "", nil
	case svcErr == nil:
		return "", withDetail(ErrorPolicyExists, id)
	case svcErr.Code == ErrorPolicyNotFound.Code:
		return id, nil
	default:
		return "", svcErr
	}
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

	// A request carrying an id is a replay of one that already existed, so it keeps it: a reshare
	// beneath it points at that id, and an import that minted a new one would orphan it. A request
	// carrying none is a fresh create and is given one.
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
	}

	if svcErr := s.buildSections(ctx, req, &policy); svcErr != nil {
		return Policy{}, svcErr
	}

	if svcErr := s.requireExclusionsInReach(ctx, policy.grants(), initiatingOUID); svcErr != nil {
		return Policy{}, svcErr
	}

	rules, svcErr := s.materializeRules(ctx, rt, resourceID, owningOUID, initiatingOUID, req)
	if svcErr != nil {
		return Policy{}, svcErr
	}
	policy.Rules = rules
	return policy, nil
}

// requireEditLeavesFrontiersIntact refuses an edit that would reach past a unit which has already
// handed the resource on.
//
// Widening a named unit's reach from the unit alone to the unit and its subtree is the case: the
// unit below stops being a frontier, and everything it granted would then be covered twice, by its
// own policy and by this one. The unit has to withdraw what it granted before the reach above it
// can be widened.
func (s *sharingService) requireEditLeavesFrontiersIntact(
	ctx context.Context, proposed Policy,
) *tidcommon.ServiceError {
	policies, err := s.store.ListAllPoliciesForResource(ctx, proposed.ResourceType, proposed.ResourceID)
	if err != nil {
		s.logger.Error(ctx, "Failed to list policies for resource", log.Error(err))
		return &tidcommon.InternalServerError
	}

	grants := proposed.grants()
	for _, p := range policies {
		if p.ID == proposed.ID || p.InitiatingOUID == proposed.InitiatingOUID {
			continue
		}
		chain, svcErr := s.buildChain(ctx, p.InitiatingOUID)
		if svcErr != nil {
			// A unit that no longer exists has no standing left to protect, and its policies go
			// in the cleanup that follows the edit.
			if svcErr.Code == oupkg.ErrorOrganizationUnitNotFound.Code {
				continue
			}
			return svcErr
		}
		if reachedByCascadingGrant(grants, p.InitiatingOUID, chain) {
			return withDetail(ErrorReshareNotPermitted, p.InitiatingOUID+" has shared this resource on")
		}
	}
	return nil
}

// reachedByCascadingGrant reports whether a grant that carries on past the unit it names reaches
// ouID, which is what would strip ouID of a frontier unit's standing.
//
// A grant that excludes ouID, or a unit above it, does not reach it: an exclusion takes everything
// beneath it out of reach, exactly as the chain walk reads it.
func reachedByCascadingGrant(grants []grant, ouID string, chain []string) bool {
	cascading := make([]grant, 0, len(grants))
	for _, g := range grants {
		if cascadingScope(g.scope) && !excludedAnywhere(chain, g.excludedOUIDs) {
			cascading = append(cascading, g)
		}
	}
	return reachedByAnyGrant(cascading, ouID, chain)
}

// requireExclusionsInReach refuses an exclusion naming an organization unit its own grant does not
// reach. Nothing would apply it, and accepting it would read as a withholding that never happened.
//
// Only a blanket grant carries exclusions, so there is no named unit here for one to cancel out.
func (s *sharingService) requireExclusionsInReach(
	ctx context.Context, grants []grant, initiatingOUID string,
) *tidcommon.ServiceError {
	for _, g := range grants {
		for _, ouID := range g.excludedOUIDs {
			chain, svcErr := s.buildChain(ctx, ouID)
			if svcErr != nil {
				return svcErr
			}
			// Reaching every child unit names organization units the way a selected section does,
			// one hop below the issuer. Carving out a grandchild would decide for the child over
			// its head, and beneath a cascading grant that child holds no policy of its own to
			// reverse it with.
			if g.scope == scopeAllChildren && (len(chain) < 2 || chain[len(chain)-2] != initiatingOUID) {
				return withDetail(ErrorExclusionNotADirectChild, ouID)
			}
			if !reachedByAnyGrant([]grant{g}, ouID, chain) {
				return withDetail(ErrorExclusionOutOfReach, ouID)
			}
		}
	}
	return nil
}

// reachedByAnyGrant reports whether a grant covers ouID, given the chain from its tree's root
// down to it. A cascading grant reaches anything below its anchor; every other scope reaches
// only the unit it names.
func reachedByAnyGrant(grants []grant, ouID string, chain []string) bool {
	for _, g := range grants {
		switch g.scope {
		case scopeAllOUs:
			return true
		case scopeAllRoots:
			if len(chain) > 0 && chain[0] == ouID {
				return true
			}
		case scopeAllChildren, scopeChildSubtree:
			for _, ancestor := range chain {
				if ancestor == g.ouID && ancestor != ouID {
					return true
				}
			}
			if g.scope == scopeChildSubtree && g.ouID == ouID {
				return true
			}
		case scopeRoot, scopeChild:
			if g.ouID == ouID {
				return true
			}
		}
	}
	return false
}

// cascadingScope reports whether a grant reaches below the organization unit it names, which is
// what leaves that unit nothing of its own to grant.
func cascadingScope(scope grantScope) bool {
	switch scope {
	case scopeAllOUs,
		scopeAllChildren,
		scopeChildSubtree:
		return true
	}
	return false
}

// requireFrontierUnit refuses a reshare when every grant covering the initiator carries on past
// it. One grant that stops at the initiator is enough, since that reach is the initiator's to
// hand on.
func requireFrontierUnit(covering []Coverage) *tidcommon.ServiceError {
	for _, c := range covering {
		for _, g := range c.Policy.grants() {
			if g.ref == c.Grant && !cascadingScope(g.scope) {
				return nil
			}
		}
	}
	return &ErrorReshareNotPermitted
}

// buildSections turns the decisions a request states into their stored form on the policy,
// enforcing the stage, ownership and one-hop rules along the way.
//
// Each section is either one blanket grant or a list of named units, never both. That is what
// keeps a unit from being reached twice by one policy: a blanket grant has one set of terms for
// everything it reaches, and a unit wanting terms of its own is named in a selected section.
func (s *sharingService) buildSections(
	ctx context.Context, req PolicyRequest, policy *Policy,
) *tidcommon.ServiceError {
	if req.AllUnits == nil && req.ChildUnits == nil && req.OtherTreeRoots == nil {
		return withDetail(ErrorInvalidRequestFormat, "the policy reaches no organization unit")
	}

	initiatingOUID := policy.InitiatingOUID
	isOwner := initiatingOUID == policy.OwningOUID

	if req.AllUnits != nil {
		// Every organization unit already includes whatever the other two sections could reach,
		// and a unit reached twice has no basis for deciding whose terms it holds the resource on.
		if req.ChildUnits != nil || req.OtherTreeRoots != nil {
			return withDetail(ErrorInvalidRequestFormat, fmt.Sprintf(
				"%s cannot be combined with %s or %s",
				SectionAllUnits, SectionChildUnits, SectionOtherTreeRoots))
		}
		// Reaching every organization unit is the owner's call alone, and only as a first hop.
		if !isOwner || policy.Stage != stageShare {
			return withDetail(ErrorInvalidTargetOU, string(SectionAllUnits)+" is owner-only")
		}
		// Reaching every organization unit necessarily leaves the initiator's own tree.
		if svcErr := s.requireCrossTreeAllowed(ctx, initiatingOUID); svcErr != nil {
			return svcErr
		}
		policy.AllUnits = &AllUnits{ExcludedOUIDs: utils.UniqueStrings(req.AllUnits.ExcludedOUIDs)}
		return nil
	}

	// Naming one organization unit twice is rejected rather than deduplicated, because the two
	// entries carry their own overlay rules and there is no basis for picking a winner.
	seen := make(map[string]struct{})

	if req.ChildUnits != nil {
		childUnits, svcErr := s.buildChildUnits(ctx, *req.ChildUnits, initiatingOUID, seen)
		if svcErr != nil {
			return svcErr
		}
		policy.ChildUnits = childUnits
	}

	if req.OtherTreeRoots != nil {
		// Another tree is reached by the resource's owner and by nobody else: a unit the resource
		// was shared to hands it on beneath itself, never sideways.
		if !isOwner {
			return withDetail(ErrorInvalidTargetOU, string(SectionOtherTreeRoots)+" is owner-only")
		}
		otherTreeRoots, svcErr := s.buildOtherTreeRoots(ctx, *req.OtherTreeRoots, initiatingOUID, seen)
		if svcErr != nil {
			return svcErr
		}
		policy.OtherTreeRoots = otherTreeRoots
	}
	return nil
}

// buildChildUnits validates the decision about the units beneath the initiator.
func (s *sharingService) buildChildUnits(
	ctx context.Context, req ChildUnitsRequest, initiatingOUID string, seen map[string]struct{},
) (*ChildUnits, *tidcommon.ServiceError) {
	switch req.Mode {
	case ChildUnitsAll:
		if len(req.Units) > 0 {
			return nil, modeTakesNo(SectionChildUnits, string(req.Mode), "units")
		}
		return &ChildUnits{
			Mode:          ChildUnitsAll,
			ExcludedOUIDs: utils.UniqueStrings(req.ExcludedOUIDs),
		}, nil

	case ChildUnitsSelected:
		// A named unit is either reached or left out of the list, and it holds the resource on
		// the terms stated beside it, so neither blanket field has anything to apply to.
		if len(req.ExcludedOUIDs) > 0 {
			return nil, modeTakesNo(SectionChildUnits, string(req.Mode), "excludedOuIds")
		}
		if len(req.OverlayRules) > 0 {
			return nil, modeTakesNo(SectionChildUnits, string(req.Mode), "overlayRules of its own")
		}
		if len(req.Units) == 0 {
			return nil, modeNamesNothing(SectionChildUnits, string(req.Mode))
		}

		out := &ChildUnits{Mode: ChildUnitsSelected, Units: make([]ChildUnit, 0, len(req.Units))}
		for _, u := range req.Units {
			if u.OUID == "" {
				return nil, withDetail(ErrorInvalidRequestFormat, "a child unit needs an ouId")
			}
			if u.Reach != ReachUnit && u.Reach != ReachSubtree {
				return nil, withDetail(ErrorInvalidRequestFormat,
					fmt.Sprintf("unknown reach %q for %s", u.Reach, u.OUID))
			}
			if svcErr := s.validateChildTarget(ctx, u.OUID, initiatingOUID); svcErr != nil {
				return nil, svcErr
			}
			if svcErr := nameOnce(seen, u.OUID); svcErr != nil {
				return nil, svcErr
			}
			out.Units = append(out.Units, ChildUnit{OUID: u.OUID, Reach: u.Reach})
		}
		return out, nil

	default:
		return nil, withDetail(ErrorInvalidRequestFormat,
			fmt.Sprintf("unknown %s mode %q", SectionChildUnits, req.Mode))
	}
}

// buildOtherTreeRoots validates the decision about the roots of other trees. The caller has already
// established that the initiator owns the resource.
func (s *sharingService) buildOtherTreeRoots(
	ctx context.Context, req OtherTreeRootsRequest, initiatingOUID string, seen map[string]struct{},
) (*OtherTreeRoots, *tidcommon.ServiceError) {
	switch req.Mode {
	case OtherTreeRootsAll:
		if len(req.Units) > 0 {
			return nil, modeTakesNo(SectionOtherTreeRoots, string(req.Mode), "units")
		}
		if svcErr := s.requireCrossTreeAllowed(ctx, initiatingOUID); svcErr != nil {
			return nil, svcErr
		}
		// The initiator's own tree is not another tree, so its root is withheld whether or not
		// the request says so. A unit inside a tree handing the resource up to its own root would
		// let that root hand it back down over the unit's head.
		excluded := utils.UniqueStrings(req.ExcludedOUIDs)
		ownRoot, svcErr := s.treeRootOf(ctx, initiatingOUID)
		if svcErr != nil {
			return nil, svcErr
		}
		if ownRoot != initiatingOUID && !slices.Contains(excluded, ownRoot) {
			excluded = append(excluded, ownRoot)
		}
		return &OtherTreeRoots{Mode: OtherTreeRootsAll, ExcludedOUIDs: excluded}, nil

	case OtherTreeRootsSelected:
		if len(req.ExcludedOUIDs) > 0 {
			return nil, modeTakesNo(SectionOtherTreeRoots, string(req.Mode), "excludedOuIds")
		}
		if len(req.OverlayRules) > 0 {
			return nil, modeTakesNo(SectionOtherTreeRoots, string(req.Mode), "overlayRules of its own")
		}
		if len(req.Units) == 0 {
			return nil, modeNamesNothing(SectionOtherTreeRoots, string(req.Mode))
		}

		out := &OtherTreeRoots{Mode: OtherTreeRootsSelected, Units: make([]string, 0, len(req.Units))}
		for _, root := range req.Units {
			if root.OUID == "" {
				return nil, withDetail(ErrorInvalidRequestFormat, "a root unit needs an ouId")
			}
			if svcErr := s.validateRootTarget(ctx, root.OUID, initiatingOUID); svcErr != nil {
				return nil, svcErr
			}
			if svcErr := nameOnce(seen, root.OUID); svcErr != nil {
				return nil, svcErr
			}
			out.Units = append(out.Units, root.OUID)
		}
		return out, nil

	default:
		return nil, withDetail(ErrorInvalidRequestFormat,
			fmt.Sprintf("unknown %s mode %q", SectionOtherTreeRoots, req.Mode))
	}
}

// nameOnce records an organization unit a policy names, refusing one it has named already.
func nameOnce(seen map[string]struct{}, ouID string) *tidcommon.ServiceError {
	if _, repeated := seen[ouID]; repeated {
		return withDetail(ErrorInvalidTargetOU, ouID+" is named more than once")
	}
	seen[ouID] = struct{}{}
	return nil
}

// modeTakesNo reports a section carrying a field its mode has no use for.
func modeTakesNo(section Section, mode, field string) *tidcommon.ServiceError {
	return withDetail(ErrorInvalidRequestFormat,
		fmt.Sprintf("%s with mode %s takes no %s", section, mode, field))
}

// modeNamesNothing reports a selected section naming no organization unit, which states a decision
// to reach nobody. Leaving the section out says that.
func modeNamesNothing(section Section, mode string) *tidcommon.ServiceError {
	return withDetail(ErrorInvalidRequestFormat,
		fmt.Sprintf("%s with mode %s names no organization unit", section, mode))
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

// validateRootTarget checks a named root really is a root, that it is the root of another tree, and
// that the initiator may reach outside its own.
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
	// The initiator's own tree is not another tree. A unit inside a tree handing the resource up to
	// its own root would let that root hand it back down over the unit's head, and a root naming
	// itself reaches nobody.
	if targetOUID == initiatorRoot {
		return withDetail(ErrorInvalidTargetOU, targetOUID+" is the root of the initiator's own tree")
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
	ctx context.Context, rt ResourceType, resourceID, owningOUID, initiatingOUID string, req PolicyRequest,
) ([]StoredRule, *tidcommon.ServiceError) {
	initiatorRules, _, svcErr := s.initiatorEffectiveRules(ctx, rt, resourceID, owningOUID, initiatingOUID)
	if svcErr != nil {
		return nil, svcErr
	}

	var out []StoredRule
	for _, g := range req.requestedGrants() {
		for fieldKey, requested := range g.rules {
			stored, svcErr := s.narrowOne(
				ctx, rt, resourceID, fieldKey, g.ref, initiatingOUID, initiatorRules, requested)
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
	ctx context.Context, rt ResourceType, resourceID, fieldKey string, ref GrantRef,
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
		Grant:     ref,
		Resolved:  copyRule(c.clamp(parent, requested)),
		Requested: requested,
	}, nil
}

// narrowOne folds one requested rule and reports the offending field on rejection.
func (s *sharingService) narrowOne(
	ctx context.Context, rt ResourceType, resourceID, fieldKey string, ref GrantRef, initiatingOUID string,
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
		Grant:     ref,
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

// ReplacePoliciesForResource applies the policies a document carries for one resource, matched by
// issuing organization unit: a missing policy is created, an existing one is edited under the API's
// update rules. The document applies as a whole or not at all, and policies it does not name are
// kept unless a narrowing edit removes them.
func (s *sharingService) ReplacePoliciesForResource(
	ctx context.Context, rt ResourceType, resourceID, owningOUID string, requests []PolicyRequest,
) (PolicyReplay, *PolicyReplayError) {
	var replay PolicyReplay
	if len(requests) == 0 {
		return replay, nil
	}

	var failure *PolicyReplayError
	// One transaction around the whole document. Each policy write is itself transactional and
	// joins this one, so a refusal rolls back every earlier write with it. Compensating afterwards
	// cannot do the same job: a narrowing edit deletes the reshares its sharees issued, and those
	// are not the document's to recreate.
	err := s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		replay = PolicyReplay{}
		failure = s.replayPolicies(txCtx, rt, resourceID, owningOUID, requests, &replay)
		if failure != nil {
			return errReplayRefused
		}
		return nil
	})
	// The writes inside cleared the caches before the transaction ended, so a reader in between
	// could have cached a state the commit replaces or the rollback undoes. Cleared either way.
	s.clearCaches(ctx)
	if failure != nil {
		return PolicyReplay{}, failure
	}
	if err != nil {
		s.logger.Error(ctx, "Failed to replay the sharing policies a document carries",
			log.String("resourceType", string(rt)), log.String("resourceID", resourceID),
			log.Error(err))
		return PolicyReplay{}, &PolicyReplayError{
			InitiatingOUID: owningOUID, Err: &tidcommon.InternalServerError}
	}
	return replay, nil
}

// policyOfInitiator reads the policy one organization unit holds for a resource, as it stands at
// this point in a replay. A unit holding none is the create case rather than a failure.
func (s *sharingService) policyOfInitiator(
	ctx context.Context, rt ResourceType, resourceID, initiatingOUID string, index int,
) (Policy, bool, *PolicyReplayError) {
	policy, err := s.store.GetPolicyByInitiator(ctx, rt, resourceID, initiatingOUID)
	switch {
	case err == nil:
		return policy, true, nil
	case errors.Is(err, errPolicyNotFound):
		return Policy{}, false, nil
	default:
		s.logger.Error(ctx, "Failed to read the policy an organization unit holds while replaying",
			log.String("resourceType", string(rt)), log.String("resourceID", resourceID),
			log.String("initiatingOUID", initiatingOUID), log.Error(err))
		return Policy{}, false, &PolicyReplayError{
			Index: index, InitiatingOUID: initiatingOUID, Err: &tidcommon.InternalServerError}
	}
}

// replayPolicies applies each of the document's policies in turn. It is the body of
// ReplacePoliciesForResource, kept apart so the transaction wrapping it stays readable.
func (s *sharingService) replayPolicies(
	ctx context.Context, rt ResourceType, resourceID, owningOUID string, requests []PolicyRequest,
	replay *PolicyReplay,
) *PolicyReplayError {
	for i, req := range requests {
		initiator := req.InitiatingOUID
		if initiator == "" {
			initiator = owningOUID
		}
		// Read the organization unit's policy as it stands now, not from a list taken before the
		// loop. An earlier entry can narrow a policy and take a reshare with it, and a stale list
		// would still show that reshare, so the entry describing it would be read as unchanged and
		// skipped. The document would then report success with the reshare gone.
		current, found, replayErr := s.policyOfInitiator(ctx, rt, resourceID, initiator, i)
		if replayErr != nil {
			return replayErr
		}
		if !found {
			// The private create keeps the id the document carries, so the policy comes back under
			// the id anything pointing at it already uses.
			if _, svcErr := s.createPolicy(
				ctx, rt, resourceID, owningOUID, req, false); svcErr != nil {
				return &PolicyReplayError{Index: i, InitiatingOUID: initiator, Err: svcErr}
			}
			continue
		}
		// A declared policy belongs to the file that declares it.
		if current.Declared {
			replay.Skipped = append(replay.Skipped, SkippedPolicy{
				Index: i, InitiatingOUID: initiator, DeclaredPolicyID: current.ID})
			continue
		}
		// An export replayed unchanged must come back out as it went in. Every edit bumps the
		// version, so rewriting identical contents would make export, import, export disagree with
		// itself. Only the sections are compared: the policy was found by its initiating unit, and
		// an edit leaves the id alone.
		if sameSections(req, requestFromPolicy(current)) {
			continue
		}
		req.Version = current.Version
		if _, svcErr := s.UpdatePolicy(ctx, current.ID, req); svcErr != nil {
			// These refusals name deleting the policy as the way through, which an import cannot do.
			if svcErr.Code == ErrorScopeFamilyChange.Code || svcErr.Code == ErrorBlanketNarrowOnly.Code {
				svcErr = withDetail(*svcErr, "an import edits an existing policy and cannot delete it; "+
					"delete the policy through the sharing policies API, then import again")
			}
			return &PolicyReplayError{Index: i, InitiatingOUID: initiator, Err: svcErr}
		}
	}
	return nil
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

// blanketRulesNarrowOnly reports whether an edit keeps every restriction a blanket grant's rules
// currently carry.
func (s *sharingService) blanketRulesNarrowOnly(
	ctx context.Context, current, proposed Policy,
) *tidcommon.ServiceError {
	// ruleKey identifies one stored rule: a field and the grant carrying it. A grant is named by
	// what it selects, so the same grant carries the same key on both sides of an edit.
	type ruleKey struct {
		fieldKey string
		grant    GrantRef
	}

	next := make(map[ruleKey]OverlayRule, len(proposed.Rules))
	for _, r := range proposed.Rules {
		next[ruleKey{r.FieldKey, r.Grant}] = r.Resolved
	}

	for _, stored := range current.Rules {
		// Only a blanket grant has nothing to expand toward. A named unit is bounded by the
		// one-hop rule wherever it appears, so its terms stay editable in both directions.
		if stored.Grant.OUID != "" {
			continue
		}
		proposedRule, kept := next[ruleKey{stored.FieldKey, stored.Grant}]
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
				stored.FieldKey, stored.Grant, initiatorRules, stored.Requested)
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
//
// A policy whose initiator no longer exists is left out. It has no ceiling to be re-narrowed
// against, and the cleanup after the edit removes it.
func (s *sharingService) orderByInitiatorDepth(
	ctx context.Context, policies []Policy,
) ([]Policy, *tidcommon.ServiceError) {
	depth := make(map[string]int, len(policies))
	gone := make(map[string]struct{})
	for _, p := range policies {
		if _, done := depth[p.InitiatingOUID]; done {
			continue
		}
		if _, done := gone[p.InitiatingOUID]; done {
			continue
		}
		ancestors, svcErr := s.ouHierarchyResolver.GetAncestorOUIDs(ctx, p.InitiatingOUID)
		if svcErr != nil {
			if svcErr.Code == oupkg.ErrorOrganizationUnitNotFound.Code {
				gone[p.InitiatingOUID] = struct{}{}
				continue
			}
			return nil, svcErr
		}
		depth[p.InitiatingOUID] = len(ancestors)
	}

	out := make([]Policy, 0, len(policies))
	for _, p := range policies {
		if _, isGone := gone[p.InitiatingOUID]; !isGone {
			out = append(out, p)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if depth[out[i].InitiatingOUID] != depth[out[j].InitiatingOUID] {
			return depth[out[i].InitiatingOUID] < depth[out[j].InitiatingOUID]
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// GetResourceDependencies reports every policy an organization unit issued, for any resource. Each
// one blocks the unit's deletion: a policy stands on its issuer's view of the resource, and a policy
// whose issuer is gone would be left behind with nothing under it, still taking part in every later
// edit of that resource. The unit's policies have to be removed before the unit can be.
func (s *sharingService) GetResourceDependencies(
	ctx context.Context, resourceType, id string,
) ([]resourcedependency.ResourceDependency, error) {
	if resourceType != resourcedependency.ResourceTypeOU {
		return []resourcedependency.ResourceDependency{}, nil
	}
	issued, err := s.store.ListPoliciesIssuedBy(ctx, id)
	if err != nil {
		return nil, err
	}
	deps := make([]resourcedependency.ResourceDependency, 0, len(issued))
	for _, p := range issued {
		deps = append(deps, resourcedependency.ResourceDependency{
			ResourceType:     resourcedependency.ResourceTypeSharingPolicy,
			ID:               p.ID,
			DisplayName:      fmt.Sprintf("%s %s", p.ResourceType, p.ResourceID),
			BehaviorOnDelete: resourcedependency.BehaviorRestrict,
		})
	}
	return deps, nil
}

// DeletePoliciesForResource removes every policy recorded for one resource, declared ones excepted:
// a declaration is its file and goes away with it.
//
// Deleting one policy cascades to the reshares beneath it, so a policy later in the list may already
// be gone by the time it is reached. That is the shape of the walk rather than a failure, so a
// policy that is no longer there is skipped.
func (s *sharingService) DeletePoliciesForResource(
	ctx context.Context, rt ResourceType, resourceID string,
) *tidcommon.ServiceError {
	policies, svcErr := s.ListPolicies(ctx, rt, resourceID)
	if svcErr != nil {
		return svcErr
	}
	if len(policies) == 0 {
		return nil
	}

	var failure *tidcommon.ServiceError
	err := s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		for _, p := range policies {
			if p.Declared {
				continue
			}
			if svcErr := s.DeletePolicy(txCtx, p.ID); svcErr != nil {
				if svcErr.Code == ErrorPolicyNotFound.Code {
					continue
				}
				failure = svcErr
				return errPolicyPurgeRefused
			}
		}
		return nil
	})
	// Each delete inside cleared the caches before the transaction ended, so they are cleared again
	// whether it committed or rolled back.
	s.clearCaches(ctx)
	if failure != nil {
		return failure
	}
	if err != nil {
		s.logger.Error(ctx, "Failed to remove the sharing policies of a resource",
			log.String("resourceType", string(rt)), log.String("resourceID", resourceID),
			log.Error(err))
		return &tidcommon.InternalServerError
	}
	return nil
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
		switch {
		case svcErr == nil && stillVisible:
			continue
		case svcErr != nil && svcErr.Code != oupkg.ErrorOrganizationUnitNotFound.Code:
			return fmt.Errorf("failed to resolve visibility for %s: %s", ouID, svcErr.Code)
		case svcErr != nil:
			// An organization unit a policy names can be deleted while the policy still names it.
			// It cannot become visible again, so it has lost the resource like any other unit, and
			// is cleaned up the same way: whatever it shared onward goes too. Failing here instead
			// would make the policy undeletable, since every retry would stop at the same unit.
			s.logger.Debug(ctx, "Cleaning up after an organization unit that no longer exists",
				log.String("ouID", ouID))
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

// candidatesFrom expands one policy's grants into the organization units it reached, which are the
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

// ousReachedBy enumerates every organization unit a policy's grants could have reached.
//
// A grant records an anchor rather than the set it covers, so the units that may have just lost
// visibility are almost never the ones the policy names: a subtree or root grant reaches
// everything below its anchor, an all-children grant reaches everything below its initiator but
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

	for _, g := range p.grants() {
		switch g.scope {
		case scopeAllOUs, scopeAllRoots:
			// Losing a root cuts off everything below it, so the whole deployment is in scope.
			if s.ouEnumerator == nil {
				continue
			}
			all, svcErr := s.ouEnumerator.AllOUIDs(ctx)
			if svcErr != nil {
				return nil, fmt.Errorf("failed to enumerate organization units: %s", svcErr.Code)
			}
			out = append(out, all...)
		case scopeAllChildren:
			// The anchor is the initiator, which keeps its own visibility; only what lies beneath
			// it was reached by this policy.
			if err := appendSubtree(g.ouID, false); err != nil {
				return nil, err
			}
		case scopeChildSubtree, scopeRoot:
			if err := appendSubtree(g.ouID, true); err != nil {
				return nil, err
			}
		case scopeChild:
			out = append(out, g.ouID)
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
		req := requestFromPolicy(p)
		if p.InitiatingOUID == p.OwningOUID {
			req.InitiatingOUID = ""
		}
		out = append(out, ReplayablePolicy{InitiatingOUID: p.InitiatingOUID, Request: req})
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
		if r, found := ruleFor(p, c.Grant, fieldKey); found {
			out = append(out, r)
		}
	}
	return out
}

// ruleFor returns the rule the covering grant carries for one field. Terms travel with the
// grant, so the grant that reached the organization unit is the one whose terms apply.
func ruleFor(p Policy, ref GrantRef, fieldKey string) (OverlayRule, bool) {
	for _, r := range p.Rules {
		if r.FieldKey == fieldKey && r.Grant == ref {
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

// ListResourceSharedOUIDs returns the organization units a resource's policies reach, owner excluded.
//
// Each grant yields the units it could cover, and every candidate is confirmed by the chain walk
// IsVisible runs, over the policies already loaded.
func (s *sharingService) ListResourceSharedOUIDs(
	ctx context.Context, rt ResourceType, resourceID string,
) ([]string, *tidcommon.ServiceError) {
	owningOUID, known, svcErr := s.ownerOf(ctx, rt, resourceID)
	if svcErr != nil {
		return nil, svcErr
	}
	if !known {
		return []string{}, nil
	}
	policies, svcErr := s.ListPolicies(ctx, rt, resourceID)
	if svcErr != nil {
		return nil, svcErr
	}

	if s.ouEnumerator == nil {
		s.logger.Error(ctx, "Organization unit resolver cannot enumerate descendants; reach cannot be listed")
		return nil, &tidcommon.InternalServerError
	}

	var candidates []string
	for _, p := range policies {
		for _, g := range p.grants() {
			ouIDs, svcErr := s.reachCandidates(ctx, g)
			if svcErr != nil {
				return nil, svcErr
			}
			candidates = append(candidates, ouIDs...)
		}
	}

	out := []string{}
	for _, ouID := range utils.UniqueStrings(candidates) {
		if ouID == owningOUID {
			continue
		}
		chain, svcErr := s.buildChain(ctx, ouID)
		if svcErr != nil {
			// A unit a policy names can be deleted while the policy still names it.
			if svcErr.Code == oupkg.ErrorOrganizationUnitNotFound.Code {
				continue
			}
			return nil, svcErr
		}
		if visible, _ := evaluateChain(chain, policies); visible {
			out = append(out, ouID)
		}
	}
	return out, nil
}

// reachCandidates lists the organization units one grant could cover. Unlike ousReachedBy, a root
// grant stops at its root: what lies beneath is reached only through the root's own reshares.
func (s *sharingService) reachCandidates(ctx context.Context, g grant) ([]string, *tidcommon.ServiceError) {
	switch g.scope {
	case scopeChild, scopeRoot:
		return []string{g.ouID}, nil
	case scopeChildSubtree:
		below, svcErr := s.ouEnumerator.DescendantOUIDs(ctx, g.ouID)
		if svcErr != nil {
			return nil, svcErr
		}
		return append([]string{g.ouID}, below...), nil
	case scopeAllChildren:
		return s.ouEnumerator.DescendantOUIDs(ctx, g.ouID)
	case scopeAllRoots:
		return s.ouEnumerator.RootOUIDs(ctx)
	case scopeAllOUs:
		return s.ouEnumerator.AllOUIDs(ctx)
	}
	return nil, nil
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
		// The chain walk counts the owner as covered, so a resource the unit owns would come back
		// whenever one of its policies is relevant here.
		if byResource[resourceID][0].OwningOUID == ouID {
			continue
		}
		if visible, _ := evaluateChain(chain, byResource[resourceID]); visible {
			out = append(out, resourceID)
		}
	}
	slices.Sort(out)
	return out, nil
}
