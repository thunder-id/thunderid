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
	// selects none or more than one target mode.
	ErrorInvalidRequestFormat = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SHR-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.sharingservice.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.sharingservice.invalid_request_format_description",
			DefaultValue: "The request body is malformed or selects more than one target mode",
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
	// ErrorCoreConfigOwnerOnly is returned when a non-owner attempts an owner-only operation.
	ErrorCoreConfigOwnerOnly = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SHR-1007",
		Error: tidcommon.I18nMessage{
			Key:          "error.sharingservice.core_config_owner_only",
			DefaultValue: "Core configuration can only be modified by the owning organization unit",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.sharingservice.core_config_owner_only_description",
			DefaultValue: "Only the owning organization unit may modify this resource's core configuration",
		},
	}
	// ErrorCrossTreeShareRestricted is returned when a non-root owner reaches a foreign tree.
	ErrorCrossTreeShareRestricted = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SHR-1009",
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
		Code: "SHR-1010",
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
		Code: "SHR-1011",
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
		Code: "SHR-1012",
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
		Code: "SHR-1013",
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
		Code: "SHR-1014",
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
		Code: "SHR-1015",
		Error: tidcommon.I18nMessage{
			Key:          "error.sharingservice.member_not_visible",
			DefaultValue: "Overlay rule names a member that is not visible",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.sharingservice.member_not_visible_description",
			DefaultValue: "An overlay rule may only name members the initiating organization unit can itself see",
		},
	}
	// ErrorPolicyExists is returned when an organization unit already has a policy for a resource.
	ErrorPolicyExists = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SHR-1016",
		Error: tidcommon.I18nMessage{
			Key:          "error.sharingservice.policy_exists",
			DefaultValue: "A sharing policy already exists for this organization unit",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.sharingservice.policy_exists_description",
			DefaultValue: "Each organization unit holds one policy per resource; update the existing one instead",
		},
	}
	// ErrorBlanketNarrowOnly is returned when an edit does more to a blanket policy than exclude.
	ErrorBlanketNarrowOnly = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SHR-1017",
		Error: tidcommon.I18nMessage{
			Key:          "error.sharingservice.blanket_narrow_only",
			DefaultValue: "A blanket policy may only be narrowed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.sharingservice.blanket_narrow_only_description",
			DefaultValue: "A policy reaching a whole family may only gain exclusions or narrower overlay rules",
		},
	}
	// ErrorVersionMismatch is returned when an edit is based on a stale version of the policy.
	ErrorVersionMismatch = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SHR-1018",
		Error: tidcommon.I18nMessage{
			Key:          "error.sharingservice.version_mismatch",
			DefaultValue: "Sharing policy version mismatch",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.sharingservice.version_mismatch_description",
			DefaultValue: "The policy changed since it was read; re-read it and reapply the change",
		},
	}
	// ErrorResourceNotFound is returned when a sharing call names a resource that does not exist.
	ErrorResourceNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SHR-1019",
		Error: tidcommon.I18nMessage{
			Key:          "error.sharingservice.resource_not_found",
			DefaultValue: "Resource not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.sharingservice.resource_not_found_description",
			DefaultValue: "The resource with the specified id does not exist",
		},
	}
)

// Internal sentinel errors for the sharing store.
var (
	// ErrPolicyNotFound is returned by the store when a policy id does not exist.
	ErrPolicyNotFound = errors.New("sharing policy not found")
	// ErrPolicyDeclared is returned by the store when a mutation targets a declared policy.
	ErrPolicyDeclared = errors.New("sharing policy is declared")
	// ErrPolicyExists is returned by the store when the one-policy-per-organization-unit
	// constraint would be violated.
	ErrPolicyExists = errors.New("sharing policy already exists for this organization unit")
)

// withDetail returns a copy of err whose description names the offending value.
func withDetail(err tidcommon.ServiceError, detail string) *tidcommon.ServiceError {
	out := err
	out.ErrorDescription.DefaultValue = fmt.Sprintf("%s: %s", err.ErrorDescription.DefaultValue, detail)
	return &out
}
