// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package sharing provides a resource-type-agnostic framework for sharing a resource from its
// owning organization unit to other organization units, and for resolving what those organization
// units may do with it. A resource type is onboarded by registering a ResourceTypeDeclaration; the
// engine, storage and rule resolution are identical for every type.
package sharing

import (
	"context"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// ResourceType identifies a resource type registered with the sharing framework.
type ResourceType string

// PolicyStage records who issued a policy, independent of which target scope it used.
type PolicyStage string

const (
	// StageShare is a policy issued by the resource's own owning organization unit.
	StageShare PolicyStage = "share"
	// StageReshare is a policy issued by an organization unit the resource was shared to.
	StageReshare PolicyStage = "reshare"
)

// TargetScope identifies the breadth of one target entry.
type TargetScope string

const (
	// TargetScopeAllOUs reaches every organization unit at every depth, current and future.
	TargetScopeAllOUs TargetScope = "all_ous"
	// TargetScopeAllRoots reaches every tree's root organization unit.
	TargetScopeAllRoots TargetScope = "all_roots"
	// TargetScopeRoot reaches one named root organization unit.
	TargetScopeRoot TargetScope = "root"
	// TargetScopeAllChildren reaches every organization unit beneath the initiator, any depth.
	TargetScopeAllChildren TargetScope = "all_children"
	// TargetScopeOU reaches one named organization unit alone.
	TargetScopeOU TargetScope = "ou"
	// TargetScopeOUSubtree reaches one named organization unit and everything beneath it.
	TargetScopeOUSubtree TargetScope = "ou_subtree"
)

// FieldKind tells the framework how to interpret a field's members, which it cannot otherwise know.
type FieldKind string

const (
	// FieldScalar is one opaque value, compared by equality.
	FieldScalar FieldKind = "scalar"
	// FieldReferenceSet is a set of ids naming other resources, compared by set membership.
	FieldReferenceSet FieldKind = "referenceSet"
	// FieldHierarchy is a set of delimiter-joined paths, where a path denotes itself and
	// everything beneath it.
	FieldHierarchy FieldKind = "hierarchy"
)

// OverlayRule is what one policy says a target organization unit may do with one field.
//
// The three list fields are pointers because absent and empty mean opposite things: omitted leaves
// the field unconstrained, while an explicitly empty list permits nothing. Collapsing them turns
// the most restrictive rule into the least restrictive one.
// omitempty on the pointer fields is what carries absent-versus-empty onto the wire: a nil pointer
// is omitted, while a pointer to an empty list still serializes as [].
type OverlayRule struct {
	// Editable reports whether the target organization unit may write the field.
	Editable bool `json:"editable" yaml:"editable"`
	// Value is what the target starts with, or is pinned to when the field is not editable.
	Value *[]string `json:"value,omitempty" yaml:"value,omitempty"`
	// AllowedValues bounds what the target may choose from. Nil means the field's whole universe.
	AllowedValues *[]string `json:"allowedValues,omitempty" yaml:"allowedValues,omitempty"`
	// ExcludedValues is subtracted from both Value and AllowedValues, last.
	ExcludedValues *[]string `json:"excludedValues,omitempty" yaml:"excludedValues,omitempty"`
}

// StoredRule keeps what an initiator asked for alongside what it was clamped to.
//
// Both are needed because an ancestor policy can be edited: re-materializing a descendant against a
// changed ancestor has to start from the original request, or repeated narrowing would ratchet
// permanently downward even when the ancestor later widens again.
type StoredRule struct {
	// FieldKey is the field the rule governs.
	FieldKey string
	// TargetID scopes the rule to one target entry; empty means it applies to the whole policy.
	TargetID string
	// Resolved is the rule after narrowing against the initiator's own effective rule.
	Resolved OverlayRule
	// Requested is what the initiator asked for, before narrowing.
	Requested OverlayRule
}

// Target is one organization unit selection within a policy.
type Target struct {
	// ID identifies the target row, so per-target rules can point at it.
	ID string
	// Scope is the breadth this entry reaches.
	Scope TargetScope
	// OUID is the named organization unit. Empty only for all_ous and all_roots: every other
	// scope anchors on a unit, all_children included, where it holds the issuing unit itself.
	OUID string
}

// Policy is one organization unit's standing decision about one resource: which organization units
// it reaches, and on what terms. There is exactly one per resource per initiating organization unit.
type Policy struct {
	// ID identifies the policy.
	ID string
	// ResourceType and ResourceID name the shared resource.
	ResourceType ResourceType
	ResourceID   string
	// OwningOUID is the organization unit that owns the resource.
	OwningOUID string
	// InitiatingOUID is the organization unit whose decision this policy is.
	InitiatingOUID string
	// Stage records whether the owner or a sharee issued it.
	Stage PolicyStage
	// ParentPolicyID is the policy that made the initiator visible; empty for owner-issued policies
	// and for anything derived from a declarative one.
	ParentPolicyID string
	// Declared marks a policy loaded from a declarative resource file, which cannot be edited.
	Declared bool
	// Version backs optimistic concurrency on edit.
	Version int
	// Targets is the set of organization unit selections.
	Targets []Target
	// ExcludedOUIDs carves organization units, and their subtrees, out of every target.
	ExcludedOUIDs []string
	// Rules holds the policy-level and per-target overlay rules.
	Rules []StoredRule
}

// PolicyLevelRules returns the rules that apply to every target of the policy.
func (p Policy) PolicyLevelRules() map[string]OverlayRule {
	out := make(map[string]OverlayRule)
	for _, r := range p.Rules {
		if r.TargetID == "" {
			out[r.FieldKey] = r.Resolved
		}
	}
	return out
}

// TargetRules returns the rules that override the policy-level ones for one target.
func (p Policy) TargetRules(targetID string) map[string]OverlayRule {
	out := make(map[string]OverlayRule)
	for _, r := range p.Rules {
		if r.TargetID == targetID {
			out[r.FieldKey] = r.Resolved
		}
	}
	return out
}

// TargetEntry names one organization unit to share to, and optionally overrides the policy-level
// rules for it alone.
type TargetEntry struct {
	// OUID is the organization unit being shared to. It must be a direct child of the initiator.
	OUID string `json:"ouId" yaml:"ouId"`
	// AllChildren additionally reaches everything beneath OUID, at any depth.
	AllChildren bool `json:"allChildren,omitempty" yaml:"allChildren,omitempty"`
	// OverlayRules override the policy-level rules for this target, per field.
	OverlayRules map[string]OverlayRule `json:"overlayRules,omitempty" yaml:"overlayRules,omitempty"`
}

// TargetOUScope is the request-side selector for which organization units a policy reaches.
// Exactly one of the three modes may be populated.
type TargetOUScope struct {
	// AllOUs reaches every organization unit in the deployment. Owner-only, share-stage only.
	AllOUs bool `json:"allOus,omitempty" yaml:"allOus,omitempty"`
	// AllRoots reaches every root organization unit. Owner-only.
	AllRoots bool `json:"allRoots,omitempty" yaml:"allRoots,omitempty"`
	// RootOUIDs names specific root organization units. Owner-only.
	RootOUIDs []string `json:"rootOuIds,omitempty" yaml:"rootOuIds,omitempty"`
	// ExcludedRootOUIDs carves roots out of an AllRoots selection.
	ExcludedRootOUIDs []string `json:"excludedRootOuIds,omitempty" yaml:"excludedRootOuIds,omitempty"`
	// AllChildren reaches the initiator's whole subtree.
	AllChildren bool `json:"allChildren,omitempty" yaml:"allChildren,omitempty"`
	// ChildOUIDs names direct children of the initiator, each deciding whether its subtree comes too.
	ChildOUIDs []TargetEntry `json:"childOuIds,omitempty" yaml:"childOuIds,omitempty"`
	// ExcludedOUIDs carves organization units, and their subtrees, out of the selection.
	ExcludedOUIDs []string `json:"excludedOuIds,omitempty" yaml:"excludedOuIds,omitempty"`
}

// Mode reports which of the three target modes the scope selects, and whether exactly one is set.
func (s TargetOUScope) Mode() (blanket, root, children bool, valid bool) {
	blanket = s.AllOUs
	root = s.AllRoots || len(s.RootOUIDs) > 0
	children = s.AllChildren || len(s.ChildOUIDs) > 0

	set := 0
	for _, m := range []bool{blanket, root, children} {
		if m {
			set++
		}
	}
	return blanket, root, children, set == 1
}

// PolicyRequest is one create or edit of a policy.
type PolicyRequest struct {
	// InitiatingOUID is the organization unit making the decision; empty means the resource's owner.
	InitiatingOUID string `json:"initiatingOuId,omitempty" yaml:"initiatingOuId,omitempty"`
	// TargetOUScope selects which organization units the policy reaches.
	TargetOUScope TargetOUScope `json:"targetOuScope" yaml:"targetOuScope"`
	// OverlayRules are the policy-level terms, defaulted per target unless a target overrides them.
	OverlayRules map[string]OverlayRule `json:"overlayRules,omitempty" yaml:"overlayRules,omitempty"`
	// Version is the policy version the caller believes it is editing. Ignored on create.
	Version int `json:"version,omitempty" yaml:"version,omitempty"`
}

// ReplayablePolicy is a policy in the form that recreates it, for export and declarative round-trip.
type ReplayablePolicy struct {
	// InitiatingOUID is the organization unit whose policy this is.
	InitiatingOUID string `json:"initiatingOuId" yaml:"initiatingOuId"`
	// Request recreates the policy when replayed through the create path.
	Request PolicyRequest `json:"request" yaml:"request"`
}

// FieldDeclaration describes one field a resource type exposes to sharing policies.
type FieldDeclaration struct {
	// Key identifies the field.
	Key string
	// FallbackKey names a coarser field consulted when no rule names Key itself.
	FallbackKey string
	// Kind tells the framework how to compare the field's members.
	Kind FieldKind
	// DynamicKeys makes Key a prefix, so Key + "." + <id> is also a valid rule key.
	DynamicKeys bool
	// Default is the rule that applies when no policy names the field.
	Default *OverlayRule
}

// ResourceTypeDeclaration is implemented by every resource type onboarded onto the framework.
type ResourceTypeDeclaration interface {
	// ResourceType returns the type's identifier.
	ResourceType() ResourceType
	// Fields returns the fields a policy may carry overlay rules for.
	Fields() []FieldDeclaration
}

// OwnerResolver resolves which organization unit owns a resource. Required: the framework stores no
// resource rows of its own and cannot read an owner off one.
type OwnerResolver interface {
	// OwningOUID returns the organization unit that owns resourceID.
	OwningOUID(ctx context.Context, resourceID string) (string, *tidcommon.ServiceError)
}

// OUEnumerator is the downward hierarchy capability policy deletion needs.
//
// A target names an anchor, not a membership list: a subtree, all-children or root target reaches
// everything beneath it, and the blanket scopes reach the whole deployment. Cleaning up after a
// removed policy therefore has to walk down, which the upward-only resolver cannot do.
type OUEnumerator interface {
	// DescendantOUIDs returns every organization unit beneath ouID, at any depth.
	DescendantOUIDs(ctx context.Context, ouID string) ([]string, *tidcommon.ServiceError)
	// AllOUIDs returns every organization unit in the deployment.
	AllOUIDs(ctx context.Context) ([]string, *tidcommon.ServiceError)
}

// FieldDelimiterResolver is an optional capability for resource types whose hierarchical fields are
// keyed by paths joined with a separator the resource itself defines.
//
// Without it a hierarchy field is compared with no delimiter, and prefix containment degenerates
// into raw string prefixing: "billing" would then cover "billingx". A type declaring any field of
// kind FieldHierarchy should implement this.
type FieldDelimiterResolver interface {
	// FieldDelimiter returns the separator joining path segments for one field of one resource.
	FieldDelimiter(
		ctx context.Context, resourceID, fieldKey string,
	) (string, *tidcommon.ServiceError)
}

// PolicyHooks is an optional capability letting a resource type clean up its own per-organization
// unit state when visibility is lost. Field-scoped: a policy governing one field has no business
// deleting another's state.
type PolicyHooks interface {
	// OnVisibilityLost is called once per organization unit that stopped being able to see
	// resourceID, naming the fields the removed policy governed.
	OnVisibilityLost(ctx context.Context, resourceID, ouID string, fields []string) error
}

// OverlayCleaner is an optional capability for a resource type that keeps an organization unit's
// own values somewhere other than the framework's overlay table.
//
// When a unit loses sight of a resource every value it had chosen for that resource is moot, so the
// framework deletes them. It deletes from its own table by default; a type that stores them
// elsewhere implements this and is called instead, since only that type knows where they live.
//
// Not field-scoped, unlike PolicyHooks: this fires only when a unit has lost the resource outright,
// and at that point every value it holds for the resource is dead, not just the ones the removed
// policy happened to govern.
type OverlayCleaner interface {
	// DeleteOverlayValues removes every value ouID holds for resourceID.
	DeleteOverlayValues(ctx context.Context, resourceID, ouID string) error
}

// MemberValidator is an optional capability letting a resource type reject members the initiating
// organization unit cannot itself see. Only the resource type knows what a member id denotes.
type MemberValidator interface {
	// ValidateMembers reports whether initiatingOUID may name these members for fieldKey of
	// resourceID. The resource is named because what a member denotes is usually relative to it:
	// a permission string means nothing without the server that defines it.
	ValidateMembers(
		ctx context.Context, resourceID, fieldKey, initiatingOUID string, members []string,
	) error
}

// DeletionOwnershipError is an optional capability supplying a resource-type-specific rejection
// when a non-owner attempts a delete.
type DeletionOwnershipError interface {
	// ErrorResourceDeletionRestrictedToOwner returns the type's own deletion refusal.
	ErrorResourceDeletionRestrictedToOwner() *tidcommon.ServiceError
}
