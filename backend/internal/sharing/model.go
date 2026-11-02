// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import (
	"bytes"
	"encoding/json"

	"github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"gopkg.in/yaml.v3"
)

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

// policyStage records who issued a policy, independent of which sections it states.
type policyStage string

const (
	// stageShare is a policy issued by the resource's own owning organization unit.
	stageShare policyStage = "share"
	// stageReshare is a policy issued by an organization unit the resource was shared to.
	stageReshare policyStage = "reshare"
)

// Section names one of the decisions a policy states. It is exported because a stored rule
// and a coverage answer both say which decision they belong to.
type Section string

const (
	// SectionAllUnits is the decision to reach every organization unit in the deployment.
	SectionAllUnits Section = "allUnits"
	// SectionChildUnits is the decision about the units beneath the initiator, in its own tree.
	SectionChildUnits Section = "childUnits"
	// SectionOtherTreeRoots is the decision about the root units of other trees, which only the
	// owner makes.
	SectionOtherTreeRoots Section = "otherTreeRoots"
)

// ChildUnitsMode says whether a policy reaches everything beneath its initiator or names the
// direct children it reaches.
type ChildUnitsMode string

const (
	// ChildUnitsAll reaches every organization unit beneath the initiator, any depth.
	ChildUnitsAll ChildUnitsMode = "all"
	// ChildUnitsSelected reaches the direct children the policy names, each as far as it says.
	ChildUnitsSelected ChildUnitsMode = "selected"
)

// OtherTreeRootsMode says whether a policy reaches every other tree's root or names the roots it
// reaches.
type OtherTreeRootsMode string

const (
	// OtherTreeRootsAll reaches the root organization unit of every other tree.
	OtherTreeRootsAll OtherTreeRootsMode = "all"
	// OtherTreeRootsSelected reaches the root organization units the policy names.
	OtherTreeRootsSelected OtherTreeRootsMode = "selected"
)

// UnitReach says how far a named child unit carries the resource.
type UnitReach string

const (
	// ReachUnit stops at the named unit, which may then hand the resource on itself.
	ReachUnit UnitReach = "unit"
	// ReachSubtree carries on to everything beneath the named unit.
	ReachSubtree UnitReach = "subtree"
)

// AllUnits is the decision to reach every organization unit at every depth, current and future.
type AllUnits struct {
	// ExcludedOUIDs withholds organization units, and their subtrees.
	ExcludedOUIDs []string
}

// ChildUnit is one direct child a policy names.
type ChildUnit struct {
	// OUID is the named organization unit.
	OUID string
	// Reach is how far the resource carries from it.
	Reach UnitReach
}

// ChildUnits is the decision about the organization units beneath the initiator.
type ChildUnits struct {
	// Mode selects between everything beneath the initiator and the children named in Units.
	Mode ChildUnitsMode
	// ExcludedOUIDs withholds direct children, and their subtrees. Only mode all carries any.
	ExcludedOUIDs []string
	// Units are the direct children reached. Only mode selected carries any.
	Units []ChildUnit
}

// OtherTreeRoots is the decision about the root organization units of other trees.
type OtherTreeRoots struct {
	// Mode selects between every other tree's root and the roots named in Units.
	Mode OtherTreeRootsMode
	// ExcludedOUIDs withholds roots, and with them their trees. Only mode all carries any.
	ExcludedOUIDs []string
	// Units are the root organization units reached. Only mode selected carries any.
	Units []string
}

// GrantRef identifies one grant inside a policy: a section's blanket reach when OUID is empty, and
// the organization unit that section names otherwise. It is what a rule and a coverage answer point
// at, and it is stable across edits because it is made of what the grant selects.
type GrantRef struct {
	// Section is the decision the grant belongs to.
	Section Section
	// OUID is the named organization unit, empty for a section's blanket grant.
	OUID string
}

// grantScope is the breadth of one grant, as the chain walk reads it.
type grantScope string

const (
	// scopeAllOUs reaches every organization unit at every depth, current and future.
	scopeAllOUs grantScope = "allOus"
	// scopeAllRoots reaches every tree's root organization unit.
	scopeAllRoots grantScope = "allRoots"
	// scopeRoot reaches one named root organization unit.
	scopeRoot grantScope = "root"
	// scopeAllChildren reaches every organization unit beneath the initiator, any depth.
	scopeAllChildren grantScope = "allChildren"
	// scopeChild reaches one direct child of the initiator alone.
	scopeChild grantScope = "child"
	// scopeChildSubtree reaches one direct child and everything beneath it.
	scopeChildSubtree grantScope = "childSubtree"
)

// grant is one organization unit selection a policy makes, flattened out of its sections for the
// chain walk.
type grant struct {
	// ref names the section, and the unit within it, the grant came from.
	ref GrantRef
	// scope is the breadth this grant reaches.
	scope grantScope
	// ouID is the organization unit the grant anchors on. Empty only for allOus and allRoots:
	// every other scope anchors on a unit, allChildren included, where it holds the issuing unit.
	ouID string
	// excludedOUIDs withholds organization units, and their subtrees. Only a blanket grant
	// carries any.
	excludedOUIDs []string
}

// Coverage records which policy and grant made one chain position visible.
type Coverage struct {
	// Policy is the covering policy.
	Policy Policy
	// Grant is the grant that covered the position.
	Grant GrantRef
	// ByAllOUs marks coverage that only a deployment-wide policy provided.
	ByAllOUs bool
}

// sectionKind is what one section of a policy holds, which is all an edit is checked against.
type sectionKind string

const (
	// kindNone is a section the policy does not state.
	kindNone sectionKind = "none"
	// kindSelected is a section naming organization units one by one.
	kindSelected sectionKind = "selected"
	// kindBlanket is a section reaching everything in its family.
	kindBlanket sectionKind = "blanket"
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
	// editableMissing records that a decoded rule left editable out, which a field taking editable
	// alone refuses rather than reading as false.
	editableMissing bool
}

// overlayRuleDocument is an OverlayRule as written, where an absent editable stays absent.
type overlayRuleDocument struct {
	Editable       *bool     `json:"editable" yaml:"editable"`
	Value          *[]string `json:"value,omitempty" yaml:"value,omitempty"`
	AllowedValues  *[]string `json:"allowedValues,omitempty" yaml:"allowedValues,omitempty"`
	ExcludedValues *[]string `json:"excludedValues,omitempty" yaml:"excludedValues,omitempty"`
}

// rule returns the OverlayRule the document states.
func (d overlayRuleDocument) rule() OverlayRule {
	return OverlayRule{
		Editable:        d.Editable != nil && *d.Editable,
		Value:           d.Value,
		AllowedValues:   d.AllowedValues,
		ExcludedValues:  d.ExcludedValues,
		editableMissing: d.Editable == nil,
	}
}

// UnmarshalJSON decodes an OverlayRule, refusing a key it does not declare.
func (r *OverlayRule) UnmarshalJSON(data []byte) error {
	var doc overlayRuleDocument
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&doc); err != nil {
		return err
	}
	*r = doc.rule()
	return nil
}

// UnmarshalYAML decodes an OverlayRule, refusing a key it does not declare.
func (r *OverlayRule) UnmarshalYAML(node *yaml.Node) error {
	var doc overlayRuleDocument
	if err := decodeYAMLStrict(node, &doc); err != nil {
		return err
	}
	*r = doc.rule()
	return nil
}

// statesEditableOnly reports whether the rule names editable and nothing else.
func (r OverlayRule) statesEditableOnly() bool {
	return !r.editableMissing && r.Value == nil && r.AllowedValues == nil && r.ExcludedValues == nil
}

// StoredRule keeps what an initiator asked for alongside what it was clamped to.
//
// Both are needed because an ancestor policy can be edited: re-materializing a descendant against a
// changed ancestor has to start from the original request, or repeated narrowing would ratchet
// permanently downward even when the ancestor later widens again.
type StoredRule struct {
	// FieldKey is the field the rule governs.
	FieldKey string
	// Grant is the grant the rule governs. Terms travel with the grant that carries them, so
	// every rule names one.
	Grant GrantRef
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
	// AllUnits, ChildUnits and OtherTreeRoots are the three decisions the policy states. A nil one
	// was not made. AllUnits stands alone; the other two may be stated together.
	AllUnits       *AllUnits
	ChildUnits     *ChildUnits
	OtherTreeRoots *OtherTreeRoots
	// Rules holds the overlay rules, each scoped to one grant.
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

// GrantRules returns the overlay rules one grant carries.
func (p Policy) GrantRules(ref GrantRef) map[string]OverlayRule {
	out := make(map[string]OverlayRule)
	for _, r := range p.Rules {
		if r.Grant == ref {
			out[r.FieldKey] = r.Resolved
		}
	}
	return out
}

// grants flattens the policy's sections into the selections the chain walk reads. A blanket
// section yields one grant and a selected one yields a grant per named unit.
func (p Policy) grants() []grant {
	var out []grant
	if p.AllUnits != nil {
		out = append(out, grant{
			ref:           GrantRef{Section: SectionAllUnits},
			scope:         scopeAllOUs,
			excludedOUIDs: p.AllUnits.ExcludedOUIDs,
		})
	}
	if c := p.ChildUnits; c != nil {
		switch c.Mode {
		case ChildUnitsAll:
			out = append(out, grant{
				ref:   GrantRef{Section: SectionChildUnits},
				scope: scopeAllChildren,
				// The blanket grant anchors on the unit issuing it.
				ouID:          p.InitiatingOUID,
				excludedOUIDs: c.ExcludedOUIDs,
			})
		case ChildUnitsSelected:
			for _, u := range c.Units {
				scope := scopeChild
				if u.Reach == ReachSubtree {
					scope = scopeChildSubtree
				}
				out = append(out, grant{
					ref:   GrantRef{Section: SectionChildUnits, OUID: u.OUID},
					scope: scope,
					ouID:  u.OUID,
				})
			}
		}
	}
	if t := p.OtherTreeRoots; t != nil {
		switch t.Mode {
		case OtherTreeRootsAll:
			out = append(out, grant{
				ref:           GrantRef{Section: SectionOtherTreeRoots},
				scope:         scopeAllRoots,
				excludedOUIDs: t.ExcludedOUIDs,
			})
		case OtherTreeRootsSelected:
			for _, ouID := range t.Units {
				out = append(out, grant{
					ref:   GrantRef{Section: SectionOtherTreeRoots, OUID: ouID},
					scope: scopeRoot,
					ouID:  ouID,
				})
			}
		}
	}
	return out
}

// kindOf reports what one section of the policy holds.
func (p Policy) kindOf(section Section) sectionKind {
	switch section {
	case SectionAllUnits:
		if p.AllUnits != nil {
			return kindBlanket
		}
	case SectionChildUnits:
		if p.ChildUnits != nil {
			if p.ChildUnits.Mode == ChildUnitsAll {
				return kindBlanket
			}
			return kindSelected
		}
	case SectionOtherTreeRoots:
		if p.OtherTreeRoots != nil {
			if p.OtherTreeRoots.Mode == OtherTreeRootsAll {
				return kindBlanket
			}
			return kindSelected
		}
	}
	return kindNone
}

// AllUnitsRequest asks for every organization unit in the deployment. Only a resource file may
// state it, and it stands alone: a policy stating it states neither of the other two sections.
type AllUnitsRequest struct {
	// ExcludedOUIDs withholds organization units, and their subtrees.
	ExcludedOUIDs []string `json:"excludedOuIds,omitempty" yaml:"excludedOuIds,omitempty"`
	// OverlayRules are the terms every unit reached holds the resource on.
	OverlayRules map[string]OverlayRule `json:"overlayRules,omitempty" yaml:"overlayRules,omitempty"`
}

// UnmarshalYAML decodes a AllUnitsRequest, refusing a key it does not declare.
func (r *AllUnitsRequest) UnmarshalYAML(node *yaml.Node) error {
	type plain AllUnitsRequest
	return decodeYAMLStrict(node, (*plain)(r))
}

// ChildUnitRequest names one direct child of the initiator, with the terms it holds the resource on.
type ChildUnitRequest struct {
	// OUID names the direct child.
	OUID string `json:"ouId" yaml:"ouId"`
	// Reach is how far the resource carries from it.
	Reach UnitReach `json:"reach" yaml:"reach"`
	// OverlayRules are the terms this unit holds the resource on.
	OverlayRules map[string]OverlayRule `json:"overlayRules,omitempty" yaml:"overlayRules,omitempty"`
}

// UnmarshalYAML decodes a ChildUnitRequest, refusing a key it does not declare.
func (r *ChildUnitRequest) UnmarshalYAML(node *yaml.Node) error {
	type plain ChildUnitRequest
	return decodeYAMLStrict(node, (*plain)(r))
}

// ChildUnitsRequest is the decision about the organization units beneath the initiator.
//
// The two modes use different fields. Mode all reaches everything beneath the initiator on one set
// of terms, less what it excludes; mode selected names direct children, each on terms of its own.
type ChildUnitsRequest struct {
	// Mode selects between everything beneath the initiator and the children named in Units.
	Mode ChildUnitsMode `json:"mode" yaml:"mode"`
	// ExcludedOUIDs withholds direct children, and their subtrees. Mode all only.
	ExcludedOUIDs []string `json:"excludedOuIds,omitempty" yaml:"excludedOuIds,omitempty"`
	// OverlayRules are the terms everything reached holds the resource on. Mode all only.
	OverlayRules map[string]OverlayRule `json:"overlayRules,omitempty" yaml:"overlayRules,omitempty"`
	// Units are the direct children reached. Mode selected only.
	Units []ChildUnitRequest `json:"units,omitempty" yaml:"units,omitempty"`
}

// UnmarshalYAML decodes a ChildUnitsRequest, refusing a key it does not declare.
func (r *ChildUnitsRequest) UnmarshalYAML(node *yaml.Node) error {
	type plain ChildUnitsRequest
	return decodeYAMLStrict(node, (*plain)(r))
}

// RootUnitRequest names the root organization unit of one tree, with the terms it holds the
// resource on.
type RootUnitRequest struct {
	// OUID names the root organization unit.
	OUID string `json:"ouId" yaml:"ouId"`
	// OverlayRules are the terms this unit holds the resource on.
	OverlayRules map[string]OverlayRule `json:"overlayRules,omitempty" yaml:"overlayRules,omitempty"`
}

// UnmarshalYAML decodes a RootUnitRequest, refusing a key it does not declare.
func (r *RootUnitRequest) UnmarshalYAML(node *yaml.Node) error {
	type plain RootUnitRequest
	return decodeYAMLStrict(node, (*plain)(r))
}

// OtherTreeRootsRequest is the decision about the root organization units of other trees. A tree is
// reached at its root and no deeper: that unit hands the resource on within its own tree.
//
// The two modes use different fields, the same way child units do. Mode all reaches every other
// tree's root on one set of terms, less what it excludes, and only a resource file may state it;
// mode selected names roots, each on terms of its own.
type OtherTreeRootsRequest struct {
	// Mode selects between every other tree's root and the roots named in Units.
	Mode OtherTreeRootsMode `json:"mode" yaml:"mode"`
	// ExcludedOUIDs withholds roots, and with them their trees. Mode all only.
	ExcludedOUIDs []string `json:"excludedOuIds,omitempty" yaml:"excludedOuIds,omitempty"`
	// OverlayRules are the terms every root reached holds the resource on. Mode all only.
	OverlayRules map[string]OverlayRule `json:"overlayRules,omitempty" yaml:"overlayRules,omitempty"`
	// Units are the root organization units reached. Mode selected only.
	Units []RootUnitRequest `json:"units,omitempty" yaml:"units,omitempty"`
}

// UnmarshalYAML decodes a OtherTreeRootsRequest, refusing a key it does not declare.
func (r *OtherTreeRootsRequest) UnmarshalYAML(node *yaml.Node) error {
	type plain OtherTreeRootsRequest
	return decodeYAMLStrict(node, (*plain)(r))
}

// PolicyRequest is one create or edit of a policy. It states up to three sections, and a resource
// shared with nobody has no policy at all rather than one stating none of them.
type PolicyRequest struct {
	// ID is the policy's own identifier. A resource file declaring a policy has to supply one, the
	// way a declared role supplies its own, so a reshare beneath it points at the same policy on
	// every startup. Ignored on an API create, which mints its own.
	ID string `json:"id,omitempty" yaml:"id,omitempty"`
	// InitiatingOUID is the organization unit making the decision; empty means the resource's owner.
	InitiatingOUID string `json:"initiatingOuId,omitempty" yaml:"initiatingOuId,omitempty"`
	// AllUnits reaches every organization unit in the deployment.
	AllUnits *AllUnitsRequest `json:"allUnits,omitempty" yaml:"allUnits,omitempty"`
	// ChildUnits reaches organization units beneath the initiator.
	ChildUnits *ChildUnitsRequest `json:"childUnits,omitempty" yaml:"childUnits,omitempty"`
	// OtherTreeRoots reaches the root organization units of other trees.
	OtherTreeRoots *OtherTreeRootsRequest `json:"otherTreeRoots,omitempty" yaml:"otherTreeRoots,omitempty"`
	// Version is the policy version the caller believes it is editing. Ignored on create.
	Version int `json:"version,omitempty" yaml:"version,omitempty"`
}

// UnmarshalYAML decodes a PolicyRequest, refusing a key it does not declare.
func (r *PolicyRequest) UnmarshalYAML(node *yaml.Node) error {
	type plain PolicyRequest
	return decodeYAMLStrict(node, (*plain)(r))
}

// IsDeploymentWide reports whether the request reaches beyond where its issuer sits in the tree:
// every organization unit, or the root of every other tree. A management API leaves such a reach to
// resource files and imports.
func (r PolicyRequest) IsDeploymentWide() bool {
	return r.AllUnits != nil || (r.OtherTreeRoots != nil && r.OtherTreeRoots.Mode == OtherTreeRootsAll)
}

// requestedGrant is the terms a request attaches to one grant.
type requestedGrant struct {
	// ref names the grant the terms belong to.
	ref GrantRef
	// rules are the terms as the request states them, before narrowing.
	rules map[string]OverlayRule
}

// requestedGrants lists the terms a request carries, grant by grant. Only the fields its mode uses
// are read from a section, so validation has to have refused the others first.
func (r PolicyRequest) requestedGrants() []requestedGrant {
	var out []requestedGrant
	if r.AllUnits != nil {
		out = append(out, requestedGrant{GrantRef{Section: SectionAllUnits}, r.AllUnits.OverlayRules})
	}
	if c := r.ChildUnits; c != nil {
		if c.Mode == ChildUnitsAll {
			out = append(out, requestedGrant{GrantRef{Section: SectionChildUnits}, c.OverlayRules})
		}
		for _, u := range c.Units {
			out = append(out, requestedGrant{GrantRef{Section: SectionChildUnits, OUID: u.OUID}, u.OverlayRules})
		}
	}
	if t := r.OtherTreeRoots; t != nil {
		if t.Mode == OtherTreeRootsAll {
			out = append(out, requestedGrant{GrantRef{Section: SectionOtherTreeRoots}, t.OverlayRules})
		}
		for _, root := range t.Units {
			ref := GrantRef{Section: SectionOtherTreeRoots, OUID: root.OUID}
			out = append(out, requestedGrant{ref, root.OverlayRules})
		}
	}
	return out
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
	// EditableOnly makes a rule for the field state editable and nothing else.
	EditableOnly bool
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

// FieldRule is the rule one organization unit holds for one field key of a shared resource.
//
// It answers for a single key, which is what a dynamic member needs: ResolvedOverlay reports the fields a type
// declares, and a member such as "variables.brand" is not one of them.
type FieldRule struct {
	// OUID is the organization unit the rule was resolved for.
	OUID string
	// Owned reports whether the organization unit owns the resource rather than being shared it.
	Owned bool
	// Visible reports whether the organization unit can see the resource at all. When false there is no rule.
	Visible bool
	// Source records whether a policy or the type's declared default governs the field. It is empty when nothing
	// does, in which case Rule is the zero rule.
	Source string
	// Rule is the effective rule.
	Rule OverlayRule
}

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

// The types below are the wire shape a management API serves, shared by every resource type that
// mounts sharing endpoints of its own.

// PolicyCreateBody is a sharing policy as a management API creates one, which is narrower than what
// the framework can hold. It carries no policy id, which is the framework's to mint on an
// interactive create. Its terms are checked against the fields the resource type declares, so a type
// that declares none refuses every rule.
type PolicyCreateBody struct {
	InitiatingOUID string `json:"initiatingOuId,omitempty"`
	PolicyUpdateBody
}

// PolicyUpdateBody is an edit of a sharing policy: its sections and the version being edited. Whose
// decision the policy is cannot change, since that would be another policy, so an edit does not
// carry it.
type PolicyUpdateBody struct {
	AllUnits       *AllUnitsBody       `json:"allUnits,omitempty"`
	ChildUnits     *ChildUnitsBody     `json:"childUnits,omitempty"`
	OtherTreeRoots *OtherTreeRootsBody `json:"otherTreeRoots,omitempty"`
	Version        int                 `json:"version,omitempty"`
}

// AllUnitsBody is the section reaching every organization unit. A create may not state it; an edit
// restates it to rewrite the exclusions of a policy that already holds it.
type AllUnitsBody struct {
	ExcludedOUIDs []string               `json:"excludedOuIds,omitempty"`
	OverlayRules  map[string]OverlayRule `json:"overlayRules,omitempty"`
}

// ChildUnitsBody is the section about the organization units beneath the issuer.
type ChildUnitsBody struct {
	Mode          string                 `json:"mode"`
	ExcludedOUIDs []string               `json:"excludedOuIds,omitempty"`
	OverlayRules  map[string]OverlayRule `json:"overlayRules,omitempty"`
	Units         []ChildUnitBody        `json:"units,omitempty"`
}

// ChildUnitBody names one direct child of the issuer and how far the resource carries from it.
type ChildUnitBody struct {
	OUID         string                 `json:"ouId"`
	Reach        string                 `json:"reach"`
	OverlayRules map[string]OverlayRule `json:"overlayRules,omitempty"`
}

// OtherTreeRootsBody is the section about the root organization units of other trees.
type OtherTreeRootsBody struct {
	Mode          string                 `json:"mode"`
	ExcludedOUIDs []string               `json:"excludedOuIds,omitempty"`
	OverlayRules  map[string]OverlayRule `json:"overlayRules,omitempty"`
	Units         []RootUnitBody         `json:"units,omitempty"`
}

// RootUnitBody names the root organization unit of one other tree.
type RootUnitBody struct {
	OUID         string                 `json:"ouId"`
	OverlayRules map[string]OverlayRule `json:"overlayRules,omitempty"`
}

// ToPolicyRequest turns the body into the request the framework takes, naming whose decision it is.
func (b PolicyCreateBody) ToPolicyRequest() PolicyRequest {
	req := b.PolicyUpdateBody.ToPolicyRequest()
	req.InitiatingOUID = b.InitiatingOUID
	return req
}

// ToPolicyRequest turns the body into the request the framework takes. The framework is what
// validates it, so a mode or a reach the schema does not list is carried across as it was sent and
// refused there.
func (b PolicyUpdateBody) ToPolicyRequest() PolicyRequest {
	req := PolicyRequest{Version: b.Version}
	if b.AllUnits != nil {
		req.AllUnits = &AllUnitsRequest{ExcludedOUIDs: b.AllUnits.ExcludedOUIDs, OverlayRules: b.AllUnits.OverlayRules}
	}
	if c := b.ChildUnits; c != nil {
		req.ChildUnits = &ChildUnitsRequest{
			Mode: ChildUnitsMode(c.Mode), ExcludedOUIDs: c.ExcludedOUIDs, OverlayRules: c.OverlayRules,
		}
		for _, u := range c.Units {
			req.ChildUnits.Units = append(req.ChildUnits.Units,
				ChildUnitRequest{OUID: u.OUID, Reach: UnitReach(u.Reach), OverlayRules: u.OverlayRules})
		}
	}
	if t := b.OtherTreeRoots; t != nil {
		req.OtherTreeRoots = &OtherTreeRootsRequest{
			Mode: OtherTreeRootsMode(t.Mode), ExcludedOUIDs: t.ExcludedOUIDs, OverlayRules: t.OverlayRules,
		}
		for _, u := range t.Units {
			req.OtherTreeRoots.Units = append(req.OtherTreeRoots.Units, RootUnitRequest(u))
		}
	}
	return req
}

// PolicyResponse is one recorded policy as an API returns it. Its sections come back in the shape
// a request states them in, so an edit copies them, with the version, into its request.
type PolicyResponse struct {
	ID string `json:"id"`
	// InitiatingOUID is whose decision this policy is. The owning unit is not carried: it belongs
	// to the resource, which these endpoints are already mounted under.
	InitiatingOUID string `json:"initiatingOuId"`
	Stage          string `json:"stage"`
	ParentPolicyID string `json:"parentPolicyId,omitempty"`
	IsReadOnly     bool   `json:"isReadOnly"`
	Version        int    `json:"version"`
	// AllUnits, ChildUnits and OtherTreeRoots are the sections the policy states. One it does not
	// state is left out.
	AllUnits       *AllUnitsRequest       `json:"allUnits,omitempty"`
	ChildUnits     *ChildUnitsRequest     `json:"childUnits,omitempty"`
	OtherTreeRoots *OtherTreeRootsRequest `json:"otherTreeRoots,omitempty"`
}

// PolicyListResponse is one page of a resource's policies.
type PolicyListResponse struct {
	TotalResults int              `json:"totalResults"`
	StartIndex   int              `json:"startIndex"`
	Count        int              `json:"count"`
	Policies     []PolicyResponse `json:"policies"`
	Links        []utils.Link     `json:"links"`
}

// ResolvedOverlayResponse is what one organization unit may do with a resource. A type that
// declares no overlay fields answers with empty rules.
type ResolvedOverlayResponse struct {
	OUID        string                 `json:"ouId"`
	IsOwnedByOU bool                   `json:"isOwnedByOU"`
	Visible     bool                   `json:"visible"`
	PolicyIDs   []string               `json:"policyIds"`
	Rules       map[string]OverlayRule `json:"rules"`
}

// ToPolicyResponse shapes a stored policy for an API.
func ToPolicyResponse(p Policy) PolicyResponse {
	sections := requestFromPolicy(p)
	return PolicyResponse{
		ID:             p.ID,
		InitiatingOUID: p.InitiatingOUID,
		Stage:          string(p.Stage),
		ParentPolicyID: p.ParentPolicyID,
		IsReadOnly:     p.Declared,
		Version:        p.Version,
		AllUnits:       sections.AllUnits,
		ChildUnits:     sections.ChildUnits,
		OtherTreeRoots: sections.OtherTreeRoots,
	}
}

// ToPolicyListResponse shapes one page of policies for an API. basePath is where the resource type
// mounts its policy listing, which the framework does not know; the page links are built on it.
func ToPolicyListResponse(list PolicyList, basePath string, limit, offset int) PolicyListResponse {
	policies := make([]PolicyResponse, 0, len(list.Policies))
	for _, p := range list.Policies {
		policies = append(policies, ToPolicyResponse(p))
	}
	return PolicyListResponse{
		TotalResults: list.TotalResults,
		StartIndex:   list.StartIndex,
		Count:        list.Count,
		Policies:     policies,
		Links:        utils.BuildPaginationLinks(basePath, limit, offset, list.TotalResults, ""),
	}
}

// ToResolvedOverlayResponse shapes a resolved overlay for an API. The empty collections are built
// rather than left nil, so a caller reads them without guarding against null.
func ToResolvedOverlayResponse(overlay ResolvedOverlay) ResolvedOverlayResponse {
	rules := overlay.Rules
	if rules == nil {
		rules = map[string]OverlayRule{}
	}
	policyIDs := overlay.PolicyIDs
	if policyIDs == nil {
		policyIDs = []string{}
	}
	return ResolvedOverlayResponse{
		OUID:        overlay.OUID,
		IsOwnedByOU: overlay.Owned,
		Visible:     overlay.Visible,
		PolicyIDs:   policyIDs,
		Rules:       rules,
	}
}

// PolicyReplay reports what a replay did that its caller has to say something about. A policy the
// document carries for an organization unit a declaration already governs is dropped, and the
// replay still succeeds, so the caller is told which ones lost rather than discovering it later.
type PolicyReplay struct {
	Skipped []SkippedPolicy
}

// SkippedPolicy is one policy a replay left out, named the way a document names it: by position.
type SkippedPolicy struct {
	Index            int
	InitiatingOUID   string
	DeclaredPolicyID string
}

// OverlayReplaceError is the refusal of one field of a replacement of the values an organization unit holds, carrying
// which field it was so the caller can say so.
type OverlayReplaceError struct {
	FieldKey string
	Err      *tidcommon.ServiceError
}

// PolicyReplayError is a refusal of one of the document's policies, carrying where in the document
// it sat so the caller can say which one, because a document names them only by position.
type PolicyReplayError struct {
	Index          int
	InitiatingOUID string
	Err            *tidcommon.ServiceError
}
