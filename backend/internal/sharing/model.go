// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

// ResourceType identifies a resource type registered with the sharing framework.
type ResourceType string

// FieldKind tells the framework how to interpret a field's members, which it cannot otherwise know.
// It mirrors the resource type's own declaration.
type FieldKind string

const (
	// FieldScalar is one opaque value, compared by equality.
	FieldScalar FieldKind = "scalar"
	// FieldReferenceSet is a set of ids naming other resources, compared by set membership.
	FieldReferenceSet FieldKind = "referenceSet"
	// FieldHierarchy is a set of delimiter-joined paths, where a path denotes itself and everything
	// beneath it.
	FieldHierarchy FieldKind = "hierarchy"
)

// policyStage records who issued a policy, independent of which target scope it used.
type policyStage string

const (
	// stageShare is a policy issued by the resource's own owning organization unit.
	stageShare policyStage = "share"
	// stageReshare is a policy issued by an organization unit the resource was shared to.
	stageReshare policyStage = "reshare"
)

// TargetScope identifies the breadth of one target. It is exported because a policy request names
// it, in a resource file as much as over the API.
type TargetScope string

const (
	// ScopeAllOUs reaches every organization unit at every depth, current and future.
	ScopeAllOUs TargetScope = "allOus"
	// ScopeAllRoots reaches every tree's root organization unit.
	ScopeAllRoots TargetScope = "allRoots"
	// ScopeRoot reaches one named root organization unit.
	ScopeRoot TargetScope = "root"
	// ScopeAllChildren reaches every organization unit beneath the initiator, any depth.
	ScopeAllChildren TargetScope = "allChildren"
	// ScopeChild reaches one direct child of the initiator alone.
	ScopeChild TargetScope = "child"
	// ScopeChildSubtree reaches one direct child and everything beneath it.
	ScopeChildSubtree TargetScope = "childSubtree"
)

// Target is one organization unit selection within a policy.
type Target struct {
	// ID identifies the target row, so per-target rules can point at it.
	ID string
	// Scope is the breadth this entry reaches.
	Scope TargetScope
	// OUID is the named organization unit. Empty only for allOus and allRoots: every other
	// scope anchors on a unit, allChildren included, where it holds the issuing unit itself.
	OUID string
	// ExcludedOUIDs carves organization units, and their subtrees, out of this target alone.
	// Another target of the same policy may still reach them, which is how one unit is given
	// terms of its own without leaving the broad target to cover everyone else.
	ExcludedOUIDs []string
}

// Coverage records which policy and target made one chain position visible.
type Coverage struct {
	// Policy is the covering policy.
	Policy Policy
	// TargetID is the target entry that covered the position; empty when the owner covered itself.
	TargetID string
	// ByAllOUs marks coverage that only a deployment-wide policy provided.
	ByAllOUs bool
}

// scopeFamily groups the target scopes that may be exchanged for one another by an edit.
type scopeFamily string

const (
	// familyBlanket covers the scopes that already reach everything in their family.
	familyBlanket scopeFamily = "blanket"
	// familySelective covers the scopes that name organization units individually.
	familySelective scopeFamily = "selective"
)

// Lineage is the parent link of one policy, which is all the ordering and cascade walks need.
type Lineage struct {
	// ID identifies the policy.
	ID string
	// ParentID is the policy that made this one's initiator visible. Empty for owner-issued
	// policies and for anything derived from a declared one.
	ParentID string
}

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
	// TargetID is the target entry the rule governs. Terms travel with the target that carries
	// them, so every rule names one.
	TargetID string
	// Resolved is the rule after narrowing against the initiator's own effective rule.
	Resolved OverlayRule
	// Requested is what the initiator asked for, before narrowing.
	Requested OverlayRule
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
	Stage policyStage
	// ParentPolicyID is the policy that made the initiator visible; empty for owner-issued policies
	// and for anything derived from a declarative one.
	ParentPolicyID string
	// Declared marks a policy loaded from a declarative resource file, which cannot be edited.
	Declared bool
	// Version backs optimistic concurrency on edit.
	Version int
	// Targets is the set of organization unit selections, each with the terms it carries.
	Targets []Target
	// Rules holds the overlay rules, each scoped to one target.
	Rules []StoredRule
}

// PolicyList is one page of a resource's policies, for a management API to serve.
//
// It carries no pagination links: the framework is shared by every resource type, so the path a
// policy is listed under belongs to the type's own API rather than to the framework. The handler
// that knows the path builds the links from these counts.
type PolicyList struct {
	// TotalResults is how many policies the resource has, not how many this page holds.
	TotalResults int
	// StartIndex is the one-based position of the first policy in this page.
	StartIndex int
	// Count is how many policies this page holds.
	Count int
	// Policies is the page itself.
	Policies []Policy
}

// TargetRules returns the overlay rules one target carries.
func (p Policy) TargetRules(targetID string) map[string]OverlayRule {
	out := make(map[string]OverlayRule)
	for _, r := range p.Rules {
		if r.TargetID == targetID {
			out[r.FieldKey] = r.Resolved
		}
	}
	return out
}

// TargetRequest is one organization unit selection in a policy request, together with the terms
// that selection carries.
type TargetRequest struct {
	// Scope is the breadth this target reaches.
	Scope TargetScope `json:"scope" yaml:"scope"`
	// OUID names the organization unit the scope anchors on. It is required for root, child and
	// childSubtree, and must be empty for the scopes that name no unit.
	OUID string `json:"ouId,omitempty" yaml:"ouId,omitempty"`
	// ExcludedOUIDs carves organization units, and their subtrees, out of this target alone.
	ExcludedOUIDs []string `json:"excludedOuIds,omitempty" yaml:"excludedOuIds,omitempty"`
	// OverlayRules are the terms this target holds the resource on.
	OverlayRules map[string]OverlayRule `json:"overlayRules,omitempty" yaml:"overlayRules,omitempty"`
}

// PolicyRequest is one create or edit of a policy.
type PolicyRequest struct {
	// ID is the policy's own identifier. A resource file declaring a policy has to supply one, the
	// way a declared role supplies its own, so a reshare beneath it points at the same policy on
	// every startup. Ignored on an API create, which mints its own.
	ID string `json:"id,omitempty" yaml:"id,omitempty"`
	// InitiatingOUID is the organization unit making the decision; empty means the resource's owner.
	InitiatingOUID string `json:"initiatingOuId,omitempty" yaml:"initiatingOuId,omitempty"`
	// Targets are the organization unit selections the policy makes, each with its own terms.
	Targets []TargetRequest `json:"targets" yaml:"targets"`
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

// DeclaredResourcePolicies is the sharing half of one resource's declarative document: the resource
// the policies belong to, and the policies themselves.
//
// It is a bundle rather than a single policy because a document declares a list, and the loader
// stores one object per document.
type DeclaredResourcePolicies struct {
	// ResourceID identifies the resource the policies are declared on.
	ResourceID string
	// ResourceName is carried so a startup failure can name the offending document.
	ResourceName string
	// OwningOUID is the organization unit that owns the resource.
	OwningOUID string
	// Policies are the decisions the document declares, in the order it declares them.
	Policies []PolicyRequest
}

// DeclarativeLoaderConfig tells the framework where a resource type's documents live and how to
// read the sharing half out of them. The directory is the resource type's own: a policy has no
// document of its own, it is carried inside the resource it shares.
type DeclarativeLoaderConfig struct {
	// ResourceType is the sharing resource type the policies are declared for.
	ResourceType ResourceType
	// DirectoryName is the declarative resources subdirectory the documents live in.
	DirectoryName string
	// Parser reads one document and returns its sharing half, or nil when it declares none.
	Parser func(data []byte) (*DeclaredResourcePolicies, error)
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

// ResolvedValue is what one organization unit effectively holds for one field of a shared resource.
type ResolvedValue struct {
	// Value is the effective member set, after the governing rule's bound and carve-outs.
	Value []string
	// Source records whether the organization unit chose this itself or took it from the rule.
	Source string
	// Editable reports whether the organization unit may still change it, which is what tells a
	// caller apart a value it happens to agree with from one it has no say over.
	Editable bool
}

// ResolvedValues is the effective value per field for one organization unit.
//
// Separate from ResolvedOverlay because the two answer different questions: the rules say what an
// organization unit may do with a resource, these say what it currently holds. A caller rendering
// a shared resource needs the second; a caller deciding whether to offer an edit needs the first.
type ResolvedValues struct {
	// OUID is the organization unit the values were resolved for.
	OUID string
	// Owned reports whether the organization unit owns the resource rather than being shared it.
	Owned bool
	// Visible reports whether the organization unit can see the resource at all. When false the
	// values are empty, for the same reason the rules are.
	Visible bool
	// Values is the effective value per field key. A field governed by no rule has no entry, and a
	// dynamic member appears under its own key rather than under the prefix that declared it.
	Values map[string]ResolvedValue
}

// Value sources reported on a resolved value.
const (
	// SourceOverlay marks a value the organization unit chose for itself.
	SourceOverlay = "overlay"
	// SourceRule marks a value the organization unit took from the governing rule, either because
	// it has chosen nothing or because the field is not its to choose.
	SourceRule = "rule"
)
