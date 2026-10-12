// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import (
	"context"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// The interfaces below are the capabilities a resource type brings to the framework. Every one of
// them is implemented outside this package.
//
// The two organization unit capabilities the framework also needs are declared where they are
// implemented rather than here, and they sit in different places for a reason worth recording:
// ou.HierarchyEnumeratorInterface walks the tree downwards and belongs to the package that owns the
// tree, while sysauthz.OUHierarchyResolver answers upward questions and lives in sysauthz because
// sysauthz consumes it too and ou imports sysauthz.

// ResourceOverlayFieldDeclaration is implemented by every resource type onboarded onto the
// framework. It is the registration contract: the type names itself, and declares the fields a
// policy may carry overlay rules for. A type with no per-organization-unit state of its own
// declares none and onboards for visibility alone.
type ResourceOverlayFieldDeclaration interface {
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

// FieldDelimiterResolver reports the separator joining the segments of a hierarchical path.
//
// Optional only for a type that declares no field of kind FieldHierarchy. A type that declares one
// must implement this: there is no fallback, and every operation touching such a field is refused
// as an internal error until it does. The same happens to a resolver that answers with an empty
// string, because a hierarchy compared with no separator is raw string prefixing, under which
// "billing" would cover "billingx" and a policy would hand over a sibling nobody named.
type FieldDelimiterResolver interface {
	// FieldDelimiter returns the separator joining path segments for one field of one resource.
	FieldDelimiter(
		ctx context.Context, resourceID, fieldKey string,
	) (string, *tidcommon.ServiceError)
}

// PolicyHooks is an optional capability letting a resource type clean up its own per-organization
// unit state when visibility is lost. It fires on total loss and covers the whole resource, which
// is what the method below explains.
type PolicyHooks interface {
	// OnVisibilityLost is called once per organization unit that stopped being able to see
	// resourceID, and covers the whole resource rather than a list of fields.
	//
	// There is no field list because the hook fires only on total loss: the unit can see nothing of
	// the resource, so everything it held for it goes. A list derived from the removed policy could
	// not be complete either. A field that policy never named can still hold state, either because
	// the type declared an editable default for it, which a unit may write with no policy
	// mentioning the field, or because the field uses dynamic keys, where a rule on the prefix does
	// not enumerate the instances beneath it.
	OnVisibilityLost(ctx context.Context, resourceID, ouID string) error
}

// OverlayCleaner is an optional capability for a resource type that keeps an organization unit's
// own values somewhere other than the framework's overlay table.
//
// When a unit loses sight of a resource every value it had chosen for that resource is moot, so the
// framework deletes them. It deletes from its own table by default; a type that stores them
// elsewhere implements this and is called instead, since only that type knows where they live.
//
// Like PolicyHooks, this fires only when a unit has lost the resource outright, and at that point
// every value it holds for the resource is dead, not just the ones the removed policy happened to
// govern. Neither hook is field-scoped, because a field the removed policy never named can still
// hold state that is just as dead.
type OverlayCleaner interface {
	// DeleteOverlayValues removes every value ouID holds for resourceID.
	DeleteOverlayValues(ctx context.Context, resourceID, ouID string) error
}

// OverlayRuleValidator is an optional capability letting a resource type refuse rule shapes the
// framework's algebra accepts but the type gives no meaning to, such as a menu on a field nobody
// may choose for.
type OverlayRuleValidator interface {
	// ValidateOverlayRule reports whether a requested rule for fieldKey of resourceID is one the
	// type supports. It sees the rule as the initiator wrote it, before any narrowing.
	ValidateOverlayRule(
		ctx context.Context, resourceID, fieldKey string, rule OverlayRule,
	) *tidcommon.ServiceError
}

// MemberValidator is an optional capability letting a resource type reject members the initiating
// organization unit cannot itself see. Only the resource type knows what a member id denotes.
type MemberValidator interface {
	// ValidateMembers reports whether initiatingOUID may name these members for fieldKey of
	// resourceID. The resource is named because what a member denotes is usually relative to it:
	// a permission string means nothing without the server that defines it.
	//
	// A client error means the members are refused, and the caller is told so without the reason.
	// Any other error means the check itself failed, which is reported as an internal failure: a
	// store the type could not reach says nothing about whether the members were nameable.
	ValidateMembers(
		ctx context.Context, resourceID, fieldKey, initiatingOUID string, members []string,
	) *tidcommon.ServiceError
}
