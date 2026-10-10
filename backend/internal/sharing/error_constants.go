// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import (
	"errors"
	"fmt"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Client errors for the sharing service. Every rejection names the offending field key or
// organization unit, because a validation error saying only "invalid policy" costs an afternoon.
var (
	// ErrorInvalidRequestFormat is returned when a policy request is malformed, including when it
	// names no target at all or a target whose scope and organization unit disagree.
	ErrorInvalidRequestFormat = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SHR-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.sharingservice.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.sharingservice.invalid_request_format_description",
			DefaultValue: "The request body is malformed, names no target, or names a target " +
				"whose scope and organization unit do not go together",
		},
	}
	// ErrorResourceTypeNotRegistered is returned when a resource type has no declaration.
	ErrorResourceTypeNotRegistered = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SHR-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.sharingservice.resource_type_not_registered",
			DefaultValue: "Resource type not registered",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.sharingservice.resource_type_not_registered_description",
			DefaultValue: "The given resource type has not been registered with the sharing framework",
		},
	}
	// ErrorPolicyNotFound is returned when a referenced policy does not exist.
	ErrorPolicyNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SHR-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.sharingservice.policy_not_found",
			DefaultValue: "Sharing policy not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.sharingservice.policy_not_found_description",
			DefaultValue: "The sharing policy with the specified id does not exist",
		},
	}
	// ErrorInvalidTargetOU is returned when a target does not satisfy the policy's constraints.
	ErrorInvalidTargetOU = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SHR-1004",
		Error: tidcommon.I18nMessage{
			Key:          "error.sharingservice.invalid_target_ou",
			DefaultValue: "Invalid target organization unit",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.sharingservice.invalid_target_ou_description",
			DefaultValue: "The target organization unit does not satisfy the policy's constraints",
		},
	}
	// ErrorNotShared is returned when the initiating organization unit has no standing to act.
	ErrorNotShared = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SHR-1005",
		Error: tidcommon.I18nMessage{
			Key:          "error.sharingservice.not_shared",
			DefaultValue: "Resource not shared to this organization unit",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.sharingservice.not_shared_description",
			DefaultValue: "The resource has not been shared to the initiating organization unit",
		},
	}
	// ErrorCrossTreeShareRestricted is returned when a non-root owner reaches a foreign tree.
	ErrorCrossTreeShareRestricted = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SHR-1006",
		Error: tidcommon.I18nMessage{
			Key:          "error.sharingservice.cross_tree_share_restricted",
			DefaultValue: "Sharing is not allowed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.sharingservice.cross_tree_share_restricted_description",
			DefaultValue: "The organization unit may not share this resource to the requested target",
		},
	}
	// ErrorPolicyDeclared is returned when a caller tries to modify a declared policy.
	ErrorPolicyDeclared = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SHR-1007",
		Error: tidcommon.I18nMessage{
			Key:          "error.sharingservice.policy_declared",
			DefaultValue: "Sharing policy cannot be modified",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.sharingservice.policy_declared_description",
			DefaultValue: "The policy is defined in a declarative resource file and can only be changed there",
		},
	}
	// ErrorRuleWidens is returned when a rule asks for more than the initiator itself holds.
	ErrorRuleWidens = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SHR-1008",
		Error: tidcommon.I18nMessage{
			Key:          "error.sharingservice.rule_widens",
			DefaultValue: "Overlay rule exceeds what the initiating organization unit holds",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.sharingservice.rule_widens_description",
			DefaultValue: "An overlay rule may only narrow what the initiating organization unit itself holds",
		},
	}
	// ErrorValueOutsideAllowed is returned when a rule's value is not within its own allowed set.
	ErrorValueOutsideAllowed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SHR-1009",
		Error: tidcommon.I18nMessage{
			Key:          "error.sharingservice.value_outside_allowed",
			DefaultValue: "Value is not within allowedValues",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.sharingservice.value_outside_allowed_description",
			DefaultValue: "An overlay rule's value must be drawn from its own allowed values",
		},
	}
	// ErrorUnknownFieldKey is returned for a field the resource type does not declare.
	ErrorUnknownFieldKey = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SHR-1010",
		Error: tidcommon.I18nMessage{
			Key:          "error.sharingservice.unknown_field_key",
			DefaultValue: "Unknown field key",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.sharingservice.unknown_field_key_description",
			DefaultValue: "The field key is not declared by this resource type",
		},
	}
	// ErrorPinnedWithAllowed is returned when a non-editable rule also bounds a choice.
	ErrorPinnedWithAllowed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SHR-1011",
		Error: tidcommon.I18nMessage{
			Key:          "error.sharingservice.pinned_with_allowed",
			DefaultValue: "A non-editable field cannot carry allowedValues",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.sharingservice.pinned_with_allowed_description",
			DefaultValue: "A field that cannot be edited offers no choice to bound",
		},
	}
	// ErrorMemberNotVisible is returned when a rule names a member the initiator cannot see.
	ErrorMemberNotVisible = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SHR-1012",
		Error: tidcommon.I18nMessage{
			Key:          "error.sharingservice.member_not_visible",
			DefaultValue: "Overlay rule names a member that cannot be used",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.sharingservice.member_not_visible_description",
			DefaultValue: "An overlay rule may only name members the initiating organization unit is " +
				"entitled to reference",
		},
	}
	// ErrorPolicyExists is returned when an organization unit already has a policy for a resource.
	ErrorPolicyExists = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SHR-1013",
		Error: tidcommon.I18nMessage{
			Key:          "error.sharingservice.policy_exists",
			DefaultValue: "A sharing policy already exists for this organization unit",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.sharingservice.policy_exists_description",
			DefaultValue: "Each organization unit holds one policy per resource; update the existing one instead",
		},
	}
	// ErrorBlanketNarrowOnly is returned when an edit changes which family a blanket policy reaches,
	// or drops or widens one of its overlay rules. Its exclusions are not what this refuses: those
	// may be edited in both directions, because carving a unit out and handing it back are both the
	// issuer's own decision about its own reach.
	ErrorBlanketNarrowOnly = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SHR-1014",
		Error: tidcommon.I18nMessage{
			Key:          "error.sharingservice.blanket_narrow_only",
			DefaultValue: "A blanket policy may only be narrowed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.sharingservice.blanket_narrow_only_description",
			DefaultValue: "A policy reaching a whole family keeps the family it reaches, and its overlay " +
				"rules may only narrow; its exclusions may be added or removed freely",
		},
	}
	// ErrorVersionMismatch is returned when an edit is based on a stale version of the policy.
	ErrorVersionMismatch = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SHR-1015",
		Error: tidcommon.I18nMessage{
			Key:          "error.sharingservice.version_mismatch",
			DefaultValue: "Sharing policy version mismatch",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.sharingservice.version_mismatch_description",
			DefaultValue: "The policy changed since it was read; reread it and reapply the change",
		},
	}
	// ErrorDeploymentWideScopeDeclarativeOnly is returned when an API request asks for a scope whose
	// reach is not bounded by the initiator's own position in the tree.
	ErrorDeploymentWideScopeDeclarativeOnly = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SHR-1016",
		Error: tidcommon.I18nMessage{
			Key:          "error.sharingservice.deployment_wide_scope_declarative_only",
			DefaultValue: "Deployment-wide sharing is not allowed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.sharingservice.deployment_wide_scope_declarative_only_description",
			DefaultValue: "allOus and allRoots can only be set in a declarative resource file, " +
				"not through the API",
		},
	}
	// ErrorReshareNotPermitted is returned when the policy covering the initiating organization
	// unit already reaches below it, leaving it no reach of its own to grant.
	ErrorReshareNotPermitted = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SHR-1017",
		Error: tidcommon.I18nMessage{
			Key:          "error.sharingservice.reshare_not_permitted",
			DefaultValue: "Resharing is not allowed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.sharingservice.reshare_not_permitted_description",
			DefaultValue: "The policy sharing this resource already reaches below this " +
				"organization unit, so its terms are set where that policy was defined",
		},
	}
	// ErrorExclusionOutOfReach is returned when an exclusion names an organization unit none of
	// the policy's own targets reach, where it would have no effect.
	ErrorExclusionOutOfReach = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SHR-1018",
		Error: tidcommon.I18nMessage{
			Key:          "error.sharingservice.exclusion_out_of_reach",
			DefaultValue: "Excluded organization unit is out of reach",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.sharingservice.exclusion_out_of_reach_description",
			DefaultValue: "The excluded organization unit is not reached by the target that carves it out",
		},
	}
	// ErrorDeclaredPolicyIDRequired is returned when a resource file declares a policy without an id.
	ErrorDeclaredPolicyIDRequired = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SHR-1019",
		Error: tidcommon.I18nMessage{
			Key:          "error.sharingservice.declared_policy_id_required",
			DefaultValue: "A declared sharing policy needs an id",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.sharingservice.declared_policy_id_required_description",
			DefaultValue: "A policy declared in a resource file must carry its own id, so that a " +
				"reshare beneath it keeps pointing at the same policy across restarts",
		},
	}
	// ErrorDeclaredPolicyIDConflict is returned when two declarations claim the same policy id.
	ErrorDeclaredPolicyIDConflict = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SHR-1020",
		Error: tidcommon.I18nMessage{
			Key:          "error.sharingservice.declared_policy_id_conflict",
			DefaultValue: "Declared sharing policy id is already in use",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.sharingservice.declared_policy_id_conflict_description",
			DefaultValue: "Another declared policy already carries this id, and sharing one id " +
				"between two policies would re-parent the reshares beneath them",
		},
	}
	// ErrorFieldNotEditable is returned when an organization unit writes a field it may not choose.
	ErrorFieldNotEditable = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SHR-1021",
		Error: tidcommon.I18nMessage{
			Key:          "error.sharingservice.field_not_editable",
			DefaultValue: "Field is not editable by this organization unit",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.sharingservice.field_not_editable_description",
			DefaultValue: "The rule governing this field pins its value, so the organization unit " +
				"holds what the field was shared with rather than a choice of its own",
		},
	}
	// ErrorValueNotPermitted is returned when a chosen value falls outside what the rule offers.
	ErrorValueNotPermitted = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SHR-1022",
		Error: tidcommon.I18nMessage{
			Key:          "error.sharingservice.value_not_permitted",
			DefaultValue: "Value is not permitted for this field",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.sharingservice.value_not_permitted_description",
			DefaultValue: "An organization unit may only choose members its effective rule allows " +
				"and has not excluded",
		},
	}
	// ErrorDeclaredPolicyConflictsWithStored is returned when a declaration reaches an organization
	// unit the database already holds a policy for.
	ErrorDeclaredPolicyConflictsWithStored = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SHR-1023",
		Error: tidcommon.I18nMessage{
			Key:          "error.sharingservice.declared_policy_conflicts_with_stored",
			DefaultValue: "A stored sharing policy already governs this organization unit",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.sharingservice.declared_policy_conflicts_with_stored_description",
			DefaultValue: "A resource file cannot declare a policy for a resource and organization " +
				"unit the API has already recorded one for; remove one of the two",
		},
	}
	// ErrorScopeFamilyChange is returned when an edit converts a policy between scope families.
	ErrorScopeFamilyChange = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SHR-1024",
		Error: tidcommon.I18nMessage{
			Key:          "error.sharingservice.scope_family_change",
			DefaultValue: "A sharing policy cannot change how broadly it reaches",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.sharingservice.scope_family_change_description",
			DefaultValue: "A policy naming organization units individually cannot become one reaching " +
				"a whole family, or the reverse; delete it and issue the policy you want instead",
		},
	}
	// ErrorInvalidLimit is returned when a listing asks for a page size outside what is allowed.
	ErrorInvalidLimit = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SHR-1025",
		Error: tidcommon.I18nMessage{
			Key:          "error.sharingservice.invalid_limit_parameter",
			DefaultValue: "Invalid limit parameter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.sharingservice.invalid_limit_parameter_description",
			DefaultValue: "The limit parameter must be a positive integer within the maximum page size",
		},
	}
	// ErrorInvalidOffset is returned when a listing starts before the first result.
	ErrorInvalidOffset = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SHR-1026",
		Error: tidcommon.I18nMessage{
			Key:          "error.sharingservice.invalid_offset_parameter",
			DefaultValue: "Invalid offset parameter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.sharingservice.invalid_offset_parameter_description",
			DefaultValue: "The offset parameter must be a non-negative integer",
		},
	}
	// ErrorResultLimitExceededInCompositeMode is returned when a resource has more policies across
	// the two stores than can be merged to page them. Paging is decided after the merge, so the
	// merge cannot itself be paged, and answering with a truncated page would silently hide
	// policies that do apply.
	ErrorResultLimitExceededInCompositeMode = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SHR-1027",
		Error: tidcommon.I18nMessage{
			Key:          "error.sharingservice.result_limit_exceeded_in_composite_mode",
			DefaultValue: "Result limit exceeded in composite mode",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.sharingservice.result_limit_exceeded_in_composite_mode_description",
			DefaultValue: "The resource has more sharing policies than can be listed while " +
				"declarative resources are loaded",
		},
	}
	// ErrorOverlappingTargets is returned when two targets of one policy reach the same
	// organization unit, which leaves no basis for deciding whose terms it holds the resource on.
	ErrorOverlappingTargets = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SHR-1028",
		Error: tidcommon.I18nMessage{
			Key:          "error.sharingservice.overlapping_targets",
			DefaultValue: "Overlapping targets",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.sharingservice.overlapping_targets_description",
			DefaultValue: "Two targets of this policy reach the same organization unit; " +
				"exclude it from the broader target to give it terms of its own",
		},
	}
	// ErrorExclusionEmptiesTarget is returned when an exclusion names the organization unit its
	// own target anchors on, which leaves that target reaching nothing at all.
	ErrorExclusionEmptiesTarget = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SHR-1029",
		Error: tidcommon.I18nMessage{
			Key:          "error.sharingservice.exclusion_empties_target",
			DefaultValue: "Exclusion empties its target",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.sharingservice.exclusion_empties_target_description",
			DefaultValue: "The excluded organization unit is the one its target names, so the " +
				"target would reach nothing; drop the target instead",
		},
	}
	// ErrorExclusionNotADirectChild is returned when a target reaching into the initiating
	// organization unit's own tree carves out something deeper than one of its direct children.
	ErrorExclusionNotADirectChild = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SHR-1030",
		Error: tidcommon.I18nMessage{
			Key:          "error.sharingservice.exclusion_not_a_direct_child",
			DefaultValue: "Exclusion does not name a direct child",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.sharingservice.exclusion_not_a_direct_child_description",
			DefaultValue: "A target reaching into the initiating organization unit's own tree may " +
				"only carve out one of that unit's direct children",
		},
	}
)

// errPolicyNotFound is the store's own signal that no row carries an id, kept unexported because
// nothing outside this package can hold a store to be answered with it. It is not the client's
// answer for a bad id: that is ErrorPolicyNotFound, which the service returns once the declarative
// store has also been consulted. The two are separate because absence means different things to
// different callers, from "the id is free to use" to "another writer moved the row".
var errPolicyNotFound = errors.New("sharing policy not found")

// errResultLimitExceededInCompositeMode is the composite store's own signal that the two halves
// together hold more policies than it can merge. The service turns it into
// ErrorResultLimitExceededInCompositeMode; nothing outside this package sees it.
var errResultLimitExceededInCompositeMode = errors.New("result limit exceeded in composite mode")

// withDetail returns a copy of err whose description names the offending value.
func withDetail(err tidcommon.ServiceError, detail string) *tidcommon.ServiceError {
	out := err
	out.ErrorDescription.DefaultValue = fmt.Sprintf("%s: %s", err.ErrorDescription.DefaultValue, detail)
	return &out
}

// Errors returned by the narrowing algebra. The service maps each to its own SHR code.
var (
	// errWidens is returned when a rule asks for more than the initiating OU itself holds.
	errWidens = errors.New("rule widens what the initiating organization unit holds")
	// errValueOutsideAllowed is returned when a rule's value is not within its own allowed set.
	errValueOutsideAllowed = errors.New("value is not within allowedValues")
	// errPinnedWithAllowed is returned when a non-editable rule also bounds a choice nobody can make.
	errPinnedWithAllowed = errors.New("editable false cannot be combined with allowedValues")
)

// Errors returned by edit validation.
var (
	// errBlanketScopeNarrowOnly is returned when an edit tries to do more to a blanket policy than
	// add exclusions.
	errBlanketScopeNarrowOnly = errors.New("a blanket policy may only be narrowed by exclusions")
	// errScopeFamilyChange is returned when an edit tries to convert between scope families.
	errScopeFamilyChange = errors.New("a policy cannot change scope family")
	// errVersionMismatchEngine is returned when an edit is based on a stale version of the policy.
	errVersionMismatchEngine = errors.New("policy version mismatch")
)
