// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"errors"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Client errors for user management operations.
var (
	// ErrorInvalidRequestFormat is the error returned when the request format is invalid.
	ErrorInvalidRequestFormat = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USR-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.userservice.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.userservice.invalid_request_format_description",
			DefaultValue: "The request body is malformed or contains invalid data",
		},
	}
	// ErrorMissingUserID is the error returned when user ID is missing.
	ErrorMissingUserID = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USR-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.userservice.missing_user_id",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.userservice.missing_user_id_description",
			DefaultValue: "User ID is required",
		},
	}
	// ErrorUserNotFound is the error returned when a user is not found.
	ErrorUserNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USR-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.userservice.user_not_found",
			DefaultValue: "User not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.userservice.user_not_found_description",
			DefaultValue: "The user with the specified id does not exist",
		},
	}
	// ErrorOrganizationUnitNotFound is the error returned when an organization unit is not found.
	ErrorOrganizationUnitNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USR-1005",
		Error: tidcommon.I18nMessage{
			Key:          "error.userservice.organization_unit_not_found",
			DefaultValue: "Organization unit not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.userservice.organization_unit_not_found_description",
			DefaultValue: "The specified organization unit does not exist",
		},
	}
	// ErrorHandlePathRequired is the error returned when handle path is missing.
	ErrorHandlePathRequired = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USR-1008",
		Error: tidcommon.I18nMessage{
			Key:          "error.userservice.handle_path_required",
			DefaultValue: "Handle path required",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.userservice.handle_path_required_description",
			DefaultValue: "Handle path is required for this operation",
		},
	}
	// ErrorInvalidHandlePath is the error returned when handle path format is invalid.
	ErrorInvalidHandlePath = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USR-1009",
		Error: tidcommon.I18nMessage{
			Key:          "error.userservice.invalid_handle_path",
			DefaultValue: "Invalid handle path",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.userservice.invalid_handle_path_description",
			DefaultValue: "Handle path must contain valid organizational unit identifiers separated by forward slashes",
		},
	}
	// ErrorInvalidLimit is the error returned when limit parameter is invalid.
	ErrorInvalidLimit = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USR-1011",
		Error: tidcommon.I18nMessage{
			Key:          "error.userservice.invalid_limit_parameter",
			DefaultValue: "Invalid pagination parameter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.userservice.invalid_limit_parameter_description",
			DefaultValue: "The limit parameter must be a positive integer",
		},
	}
	// ErrorInvalidOffset is the error returned when offset parameter is invalid.
	ErrorInvalidOffset = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USR-1012",
		Error: tidcommon.I18nMessage{
			Key:          "error.userservice.invalid_offset_parameter",
			DefaultValue: "Invalid pagination parameter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.userservice.invalid_offset_parameter_description",
			DefaultValue: "The offset parameter must be a non-negative integer",
		},
	}
	// ErrorAttributeConflict is the error returned when a unique attribute already exists.
	ErrorAttributeConflict = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USR-1014",
		Error: tidcommon.I18nMessage{
			Key:          "error.userservice.attribute_conflict",
			DefaultValue: "Attribute conflict",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.userservice.attribute_conflict_description",
			DefaultValue: "A user with the same unique attribute value already exists",
		},
	}
	// ErrorMissingRequiredFields is the error returned when required fields are missing.
	ErrorMissingRequiredFields = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USR-1016",
		Error: tidcommon.I18nMessage{
			Key:          "error.userservice.missing_required_fields",
			DefaultValue: "Missing required fields",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.userservice.missing_required_fields_description",
			DefaultValue: "At least one identifying attribute must be provided",
		},
	}
	// ErrorMissingCredentials is the error returned when credentials are missing.
	ErrorMissingCredentials = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USR-1017",
		Error: tidcommon.I18nMessage{
			Key:          "error.userservice.missing_credentials",
			DefaultValue: "Missing credentials",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.userservice.missing_credentials_description",
			DefaultValue: "At least one credential field must be provided",
		},
	}
	// ErrorAuthenticationFailed is the error returned when authentication fails.
	ErrorAuthenticationFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USR-1018",
		Error: tidcommon.I18nMessage{
			Key:          "error.userservice.authentication_failed",
			DefaultValue: "Authentication failed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.userservice.authentication_failed_description",
			DefaultValue: "Invalid credentials provided",
		},
	}
	// ErrorSchemaValidationFailed is the error returned when user attributes fail schema validation.
	ErrorSchemaValidationFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USR-1019",
		Error: tidcommon.I18nMessage{
			Key:          "error.userservice.schema_validation_failed",
			DefaultValue: "Schema validation failed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.userservice.schema_validation_failed_description",
			DefaultValue: "User attributes do not conform to the required schema",
		},
	}
	// ErrorInvalidFilter is the error returned when the filter parameter is invalid.
	ErrorInvalidFilter = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USR-1020",
		Error: tidcommon.I18nMessage{
			Key:          "error.userservice.invalid_filter_parameter",
			DefaultValue: "Invalid filter parameter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.userservice.invalid_filter_parameter_description",
			DefaultValue: "The filter format is invalid",
		},
	}
	// ErrorEntityTypeNotFound is the error returned when the specified user type is not found.
	ErrorEntityTypeNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USR-1021",
		Error: tidcommon.I18nMessage{
			Key:          "error.userservice.user_type_not_found",
			DefaultValue: "User type not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.userservice.user_type_not_found_description",
			DefaultValue: "The specified user type does not exist",
		},
	}
	// ErrorInvalidOUID is returned when the organization unit ID is missing or malformed.
	ErrorInvalidOUID = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USR-1022",
		Error: tidcommon.I18nMessage{
			Key:          "error.userservice.invalid_organization_unit",
			DefaultValue: "Invalid organization unit",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.userservice.invalid_organization_unit_description",
			DefaultValue: "Organization unit id must be specified as a valid UUID",
		},
	}
	// ErrorOrganizationUnitMismatch is returned when the organization unit does not match the user type definition.
	ErrorOrganizationUnitMismatch = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USR-1023",
		Error: tidcommon.I18nMessage{
			Key:          "error.userservice.organization_unit_mismatch",
			DefaultValue: "Organization unit mismatch",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.userservice.organization_unit_mismatch_description",
			DefaultValue: "The organization unit does not match the user type configuration",
		},
	}
	// ErrorInvalidCredential is the error returned when credentials are invalid.
	ErrorInvalidCredential = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USR-1024",
		Error: tidcommon.I18nMessage{
			Key:          "error.userservice.invalid_credential",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.userservice.invalid_credential_description",
			DefaultValue: "Invalid credential fields in request",
		},
	}
	// ErrorCannotModifyDeclarativeResource is the error returned when trying to modify a declarative user.
	ErrorCannotModifyDeclarativeResource = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USR-1025",
		Error: tidcommon.I18nMessage{
			Key:          "error.userservice.cannot_modify_declarative_resource",
			DefaultValue: "Cannot modify declarative resource",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.userservice.cannot_modify_declarative_resource_description",
			DefaultValue: "The user is declarative and cannot be modified or deleted",
		},
	}
	// ErrorAmbiguousUser is the error returned when multiple users match the provided filters.
	ErrorAmbiguousUser = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USR-1026",
		Error: tidcommon.I18nMessage{
			Key:          "error.userservice.ambiguous_user",
			DefaultValue: "Ambiguous user",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.userservice.ambiguous_user_description",
			DefaultValue: "Multiple users match the provided filters",
		},
	}
	// ErrorUserHasBlockingDependencies is returned when a user cannot be deleted because other
	// resources depend on it in a way that forbids deletion (e.g. agents owned by the user).
	ErrorUserHasBlockingDependencies = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USR-1027",
		Error: tidcommon.I18nMessage{
			Key:          "error.userservice.user_has_blocking_dependencies",
			DefaultValue: "User cannot be deleted",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.userservice.user_has_blocking_dependencies_description",
			DefaultValue: "The user cannot be deleted because other resources depend on it. " +
				"Remove or reassign them first.",
		},
	}
	// ErrorCredentialUpdateNotAllowed is returned when an attempt is made to update credentials
	// for a user that does not allow it.
	ErrorCredentialUpdateNotAllowed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USR-1028",
		Error: tidcommon.I18nMessage{
			Key:          "error.userservice.credential_update_not_allowed",
			DefaultValue: "Credential update not allowed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.userservice.credential_update_not_allowed_description",
			DefaultValue: "The credential updates through this endpoint are not allowed",
		},
	}

	// Sharing policy errors. Each carries the sharing framework's own description through, so a
	// refusal keeps the half that says what is wrong.

	// ErrorInvalidSharingPolicy is returned when a sharing policy is not one the issuing
	// organization unit may write.
	ErrorInvalidSharingPolicy = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USR-1029",
		Error: tidcommon.I18nMessage{
			Key:          "error.userservice.invalid_sharing_policy",
			DefaultValue: "Invalid sharing policy",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.userservice.invalid_sharing_policy_description",
			DefaultValue: "The sharing policy is not one the issuing organization unit may write",
		},
	}
	// ErrorSharingPolicyNotFound is returned when the user holds no policy under the requested id.
	// A policy belonging to another resource answers the same way.
	ErrorSharingPolicyNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USR-1030",
		Error: tidcommon.I18nMessage{
			Key:          "error.userservice.sharing_policy_not_found",
			DefaultValue: "Sharing policy not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.userservice.sharing_policy_not_found_description",
			DefaultValue: "The user has no sharing policy with the specified id",
		},
	}
	// ErrorSharingPolicyExists is returned when the organization unit already holds a policy for
	// this user.
	ErrorSharingPolicyExists = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USR-1031",
		Error: tidcommon.I18nMessage{
			Key:          "error.userservice.sharing_policy_exists",
			DefaultValue: "Sharing policy already exists",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.userservice.sharing_policy_exists_description",
			DefaultValue: "This organization unit already has a sharing policy for the user; " +
				"edit it instead",
		},
	}
	// ErrorSharingPolicyVersionMismatch is returned when an edit names a version the policy has
	// moved past.
	ErrorSharingPolicyVersionMismatch = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USR-1032",
		Error: tidcommon.I18nMessage{
			Key:          "error.userservice.sharing_policy_version_mismatch",
			DefaultValue: "Sharing policy version mismatch",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.userservice.sharing_policy_version_mismatch_description",
			DefaultValue: "The sharing policy changed since it was read; read it again and " +
				"retry the edit",
		},
	}
	// ErrorSharingPolicyDeclared is returned when an edit or a delete is aimed at a policy a
	// resource file declares.
	ErrorSharingPolicyDeclared = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USR-1033",
		Error: tidcommon.I18nMessage{
			Key:          "error.userservice.sharing_policy_declared",
			DefaultValue: "Sharing policy is declared",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.userservice.sharing_policy_declared_description",
			DefaultValue: "The sharing policy is declared in a resource file and cannot be " +
				"changed through the API",
		},
	}
	// ErrorUserNotSharedToOU is returned when an organization unit cannot see the user, so there
	// are no terms to resolve for it.
	ErrorUserNotSharedToOU = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USR-1034",
		Error: tidcommon.I18nMessage{
			Key:          "error.userservice.user_not_shared_to_ou",
			DefaultValue: "User is not shared to the organization unit",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.userservice.user_not_shared_to_ou_description",
			DefaultValue: "No sharing policy reaches the specified organization unit, so the user " +
				"is not available there",
		},
	}
	// ErrorMissingOUIDParameter is returned when an overlay-rules request does not say which
	// organization unit to resolve for.
	ErrorMissingOUIDParameter = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USR-1035",
		Error: tidcommon.I18nMessage{
			Key:          "error.userservice.missing_ou_id_parameter",
			DefaultValue: "Missing required parameter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.userservice.missing_ou_id_parameter_description",
			DefaultValue: "The 'ouId' query parameter is required",
		},
	}
	// ErrorInvalidScope is returned when the accessible organization unit listing names a scope it
	// does not offer.
	ErrorInvalidScope = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USR-1036",
		Error: tidcommon.I18nMessage{
			Key:          "error.userservice.invalid_scope_parameter",
			DefaultValue: "Invalid scope parameter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.userservice.invalid_scope_parameter_description",
			DefaultValue: "The scope parameter, when given, must be siblings",
		},
	}
	// ErrorDeploymentWideSharingNotAllowed is returned when a create through the API states a section
	// whose reach is not bounded by where the issuing organization unit sits in the tree.
	ErrorDeploymentWideSharingNotAllowed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USR-1037",
		Error: tidcommon.I18nMessage{
			Key:          "error.userservice.deployment_wide_sharing_not_allowed",
			DefaultValue: "Deployment-wide sharing is not allowed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.userservice.deployment_wide_sharing_not_allowed_description",
			DefaultValue: "A policy reaching all organization units or the roots of all other trees " +
				"can only be created from a declarative resource file or an import",
		},
	}
	// ErrorSharedUserCannotMove is returned when a user that sharing policies govern is moved to
	// another organization unit.
	ErrorSharedUserCannotMove = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USR-1038",
		Error: tidcommon.I18nMessage{
			Key:          "error.userservice.shared_user_cannot_move",
			DefaultValue: "User with sharing policies cannot be moved",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.userservice.shared_user_cannot_move_description",
			DefaultValue: "The user is shared into other organization units. Delete its sharing policies " +
				"before moving it to another organization unit",
		},
	}
)

// Error variables
var (
	// ErrUserNotFound is returned when the user is not found in the system.
	ErrUserNotFound = errors.New("user not found")

	// ErrBadAttributesInRequest is returned when the attributes in the request are invalid.
	ErrBadAttributesInRequest = errors.New("failed to marshal attributes")
)
