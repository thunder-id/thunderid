// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"errors"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/sharing"
)

// Client errors for resource management operations.
var (
	// ErrorInvalidRequestFormat is returned when the request format is invalid.
	ErrorInvalidRequestFormat = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RES-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.resourceservice.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.resourceservice.invalid_request_format_description",
			DefaultValue: "The request body is malformed or contains invalid data",
		},
	}
	// ErrorMissingID is returned when resource server/resource/action ID is missing.
	ErrorMissingID = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RES-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.resourceservice.missing_id",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.resourceservice.missing_id_description",
			DefaultValue: "ID is required",
		},
	}
	// ErrorResourceServerNotFound is returned when a resource server is not found.
	ErrorResourceServerNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RES-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.resourceservice.resource_server_not_found",
			DefaultValue: "Resource server not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.resourceservice.resource_server_not_found_description",
			DefaultValue: "The resource server with the specified id does not exist",
		},
	}
	// ErrorNameConflict is returned when a name already exists.
	ErrorNameConflict = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RES-1004",
		Error: tidcommon.I18nMessage{
			Key:          "error.resourceservice.name_conflict",
			DefaultValue: "Name conflict",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.resourceservice.name_conflict_description",
			DefaultValue: "A resource server with the same name already exists",
		},
	}
	// ErrorParentResourceNotFound is returned when a parent resource is not found.
	ErrorParentResourceNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RES-1005",
		Error: tidcommon.I18nMessage{
			Key:          "error.resourceservice.parent_resource_not_found",
			DefaultValue: "Parent resource not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.resourceservice.parent_resource_not_found_description",
			DefaultValue: "The specified parent resource does not exist",
		},
	}
	// ErrorCannotDelete is returned when resource server/resource cannot be deleted.
	ErrorCannotDelete = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RES-1006",
		Error: tidcommon.I18nMessage{
			Key:          "error.resourceservice.cannot_delete",
			DefaultValue: "Cannot delete",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.resourceservice.cannot_delete_description",
			DefaultValue: "Cannot delete resource server/resource that has dependencies",
		},
	}
	// ErrorResourceNotFound is returned when a resource is not found.
	ErrorResourceNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RES-1008",
		Error: tidcommon.I18nMessage{
			Key:          "error.resourceservice.resource_not_found",
			DefaultValue: "Resource not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.resourceservice.resource_not_found_description",
			DefaultValue: "The resource with the specified id does not exist",
		},
	}
	// ErrorActionNotFound is returned when an action is not found.
	ErrorActionNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RES-1009",
		Error: tidcommon.I18nMessage{
			Key:          "error.resourceservice.action_not_found",
			DefaultValue: "Action not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.resourceservice.action_not_found_description",
			DefaultValue: "The action with the specified id does not exist",
		},
	}
	// ErrorOrganizationUnitNotFound is returned when organization unit is not found.
	ErrorOrganizationUnitNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RES-1010",
		Error: tidcommon.I18nMessage{
			Key:          "error.resourceservice.organization_unit_not_found",
			DefaultValue: "Organization unit not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.resourceservice.organization_unit_not_found_description",
			DefaultValue: "The specified organization unit does not exist",
		},
	}
	// ErrorInvalidLimit is returned when limit parameter is invalid.
	ErrorInvalidLimit = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RES-1011",
		Error: tidcommon.I18nMessage{
			Key:          "error.resourceservice.invalid_limit_parameter",
			DefaultValue: "Invalid limit parameter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.resourceservice.invalid_limit_parameter_description",
			DefaultValue: "The limit parameter must be a positive integer",
		},
	}
	// ErrorInvalidOffset is returned when offset parameter is invalid.
	ErrorInvalidOffset = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RES-1012",
		Error: tidcommon.I18nMessage{
			Key:          "error.resourceservice.invalid_offset_parameter",
			DefaultValue: "Invalid offset parameter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.resourceservice.invalid_offset_parameter_description",
			DefaultValue: "The offset parameter must be a non-negative integer",
		},
	}
	// ErrorIdentifierConflict is returned when an identifier already exists.
	ErrorIdentifierConflict = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RES-1013",
		Error: tidcommon.I18nMessage{
			Key:          "error.resourceservice.identifier_conflict",
			DefaultValue: "Identifier conflict",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.resourceservice.identifier_conflict_description",
			DefaultValue: "A resource server with the same identifier already exists",
		},
	}
	// ErrorHandleConflict is returned when a handle already exists.
	ErrorHandleConflict = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RES-1014",
		Error: tidcommon.I18nMessage{
			Key:          "error.resourceservice.handle_conflict",
			DefaultValue: "Handle conflict",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.resourceservice.handle_conflict_description",
			DefaultValue: "The same handle already exists within the specified resource",
		},
	}
	// ErrorInvalidDelimiter is returned when delimiter is invalid.
	ErrorInvalidDelimiter = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RES-1015",
		Error: tidcommon.I18nMessage{
			Key:          "error.resourceservice.invalid_delimiter",
			DefaultValue: "Invalid delimiter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.resourceservice.invalid_delimiter_description",
			DefaultValue: "Delimiter must be a single valid character (a-z A-Z 0-9 . _ : - /)",
		},
	}
	// ErrorInvalidHandle is returned when handle contains invalid characters.
	ErrorInvalidHandle = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RES-1016",
		Error: tidcommon.I18nMessage{
			Key:          "error.resourceservice.invalid_handle",
			DefaultValue: "Invalid handle",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.resourceservice.invalid_handle_description",
			DefaultValue: "Handle length must be less than 100 characters " +
				"and contain valid characters (a-z A-Z 0-9 . _ : - /)",
		},
	}
	// ErrorDelimiterInHandle is returned when handle contains invalid characters.
	ErrorDelimiterInHandle = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RES-1017",
		Error: tidcommon.I18nMessage{
			Key:          "error.resourceservice.delimiter_conflict_in_handle",
			DefaultValue: "Delimiter conflict in handle",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.resourceservice.delimiter_conflict_in_handle_description",
			DefaultValue: "Handle cannot contain the delimiter character",
		},
	}
	// ErrorImmutableResourceServer is returned when attempting to modify a declarative resource server.
	ErrorImmutableResourceServer = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RES-1018",
		Error: tidcommon.I18nMessage{
			Key:          "error.resourceservice.cannot_modify_declarative_resource_server",
			DefaultValue: "Cannot modify declarative resource server",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.resourceservice.cannot_modify_declarative_resource_server_description",
			DefaultValue: "Resource server {{param(id)}} is defined in declarative " +
				"configuration and cannot be modified",
		},
	}
	// ErrorImmutableResource is returned when attempting to modify a declarative resource.
	ErrorImmutableResource = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RES-1019",
		Error: tidcommon.I18nMessage{
			Key:          "error.resourceservice.cannot_modify_declarative_resource",
			DefaultValue: "Cannot modify declarative resource",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.resourceservice.cannot_modify_declarative_resource_description",
			DefaultValue: "Resource {{param(id)}} is defined in declarative configuration and cannot be modified",
		},
	}
	// ErrorImmutableAction is returned when attempting to modify a declarative action.
	ErrorImmutableAction = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RES-1020",
		Error: tidcommon.I18nMessage{
			Key:          "error.resourceservice.cannot_modify_declarative_action",
			DefaultValue: "Cannot modify declarative action",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.resourceservice.cannot_modify_declarative_action_description",
			DefaultValue: "Action {{param(id)}} is defined in declarative configuration and cannot be modified",
		},
	}
	// ErrResultLimitExceededInCompositeMode is the error returned when the total number of records exceeds
	ErrResultLimitExceededInCompositeMode = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RES-1021",
		Error: tidcommon.I18nMessage{
			Key:          "error.resourceservice.result_limit_exceeded_in_composite_mode",
			DefaultValue: "Result limit exceeded in composite mode",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.resourceservice.result_limit_exceeded_in_composite_mode_description",
			DefaultValue: "The total number of records exceeds the maximum limit in composite mode",
		},
	}
	// ErrorResourceServerIDConflict is returned when a resource server with the specified ID already exists.
	ErrorResourceServerIDConflict = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RES-1023",
		Error: tidcommon.I18nMessage{
			Key:          "error.resourceservice.resource_server_id_conflict",
			DefaultValue: "Resource server ID conflict",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.resourceservice.resource_server_id_conflict_description",
			DefaultValue: "A resource server with the specified ID already exists",
		},
	}
	// ErrorInvalidMCPIdentifier is returned when an MCP resource server identifier is not an http or https URL.
	ErrorInvalidMCPIdentifier = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RES-1024",
		Error: tidcommon.I18nMessage{
			Key:          "error.resourceservice.invalid_mcp_identifier",
			DefaultValue: "Invalid MCP server identifier",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.resourceservice.invalid_mcp_identifier_description",
			DefaultValue: "Use an http or https URL with no fragment, such as https://mcp.example.com/mcp",
		},
	}
	// ErrorInvalidProject is returned when a resource server names a project that does not exist.
	ErrorInvalidProject = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RES-1025",
		Error: tidcommon.I18nMessage{
			Key:          "error.resourceservice.invalid_project",
			DefaultValue: "Invalid project",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.resourceservice.invalid_project_description",
			DefaultValue: "No project exists with the given project ID",
		},
	}
	// ErrorOUNotInProject is returned when a resource server names an organization unit outside the project
	// it names.
	ErrorOUNotInProject = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RES-1026",
		Error: tidcommon.I18nMessage{
			Key:          "error.resourceservice.ou_not_in_project",
			DefaultValue: "Organization unit not in project",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.resourceservice.ou_not_in_project_description",
			DefaultValue: "The organization unit must belong to the project",
		},
	}
	// ErrorInvalidFilter is returned when a resource server listing carries a filter it cannot apply.
	ErrorInvalidFilter = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RES-1027",
		Error: tidcommon.I18nMessage{
			Key:          "error.resourceservice.invalid_filter_parameter",
			DefaultValue: "Invalid filter parameter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.resourceservice.invalid_filter_parameter_description",
			DefaultValue: "The filter must be a single ouId eq expression with a quoted organization unit ID",
		},
	}
	// ErrorManagedResourceServer is the error returned when attempting to change a resource server a
	// control plane deployed, or its resources or actions. Only a deploy from the control plane changes
	// them.
	ErrorManagedResourceServer = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RES-1028",
		Error: tidcommon.I18nMessage{
			Key:          "error.resourceservice.cannot_modify_managed_resource_server",
			DefaultValue: "Cannot modify a resource server deployed by the control plane",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.resourceservice.cannot_modify_managed_resource_server_description",
			DefaultValue: "Resource server {{param(id)}} was deployed by the control plane and can only be " +
				"changed there",
		},
	}
	// ErrorInvalidSharingPolicy is returned when a sharing policy is not one the issuing organization unit
	// may write.
	ErrorInvalidSharingPolicy = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RES-1029",
		Error: tidcommon.I18nMessage{
			Key:          "error.resourceservice.invalid_sharing_policy",
			DefaultValue: "Invalid sharing policy",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.resourceservice.invalid_sharing_policy_description",
			DefaultValue: "The sharing policy is not one the issuing organization unit may write",
		},
	}
	// ErrorSharingPolicyNotFound is returned when the resource server holds no policy under the requested
	// id. A policy belonging to another resource answers the same way.
	ErrorSharingPolicyNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RES-1030",
		Error: tidcommon.I18nMessage{
			Key:          "error.resourceservice.sharing_policy_not_found",
			DefaultValue: "Sharing policy not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.resourceservice.sharing_policy_not_found_description",
			DefaultValue: "The resource server has no sharing policy with the specified id",
		},
	}
	// ErrorSharingPolicyExists is returned when the organization unit already holds a policy for this
	// resource server.
	ErrorSharingPolicyExists = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RES-1031",
		Error: tidcommon.I18nMessage{
			Key:          "error.resourceservice.sharing_policy_exists",
			DefaultValue: "Sharing policy already exists",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.resourceservice.sharing_policy_exists_description",
			DefaultValue: "This organization unit already has a sharing policy for the resource server; " +
				"edit it instead",
		},
	}
	// ErrorSharingPolicyVersionMismatch is returned when an edit names a version the policy has moved
	// past.
	ErrorSharingPolicyVersionMismatch = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RES-1032",
		Error: tidcommon.I18nMessage{
			Key:          "error.resourceservice.sharing_policy_version_mismatch",
			DefaultValue: "Sharing policy version mismatch",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.resourceservice.sharing_policy_version_mismatch_description",
			DefaultValue: "The sharing policy changed since it was read; read it again and retry the edit",
		},
	}
	// ErrorSharingPolicyDeclared is returned when an edit or a delete is aimed at a policy a resource file
	// declares.
	ErrorSharingPolicyDeclared = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RES-1033",
		Error: tidcommon.I18nMessage{
			Key:          "error.resourceservice.sharing_policy_declared",
			DefaultValue: "Sharing policy is declared",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.resourceservice.sharing_policy_declared_description",
			DefaultValue: "The sharing policy is declared in a resource file and cannot be changed through the API",
		},
	}
	// ErrorResourceServerNotSharedToOU is returned when an organization unit cannot see the resource
	// server, so there is nothing to resolve for it.
	ErrorResourceServerNotSharedToOU = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RES-1034",
		Error: tidcommon.I18nMessage{
			Key:          "error.resourceservice.resource_server_not_shared_to_ou",
			DefaultValue: "Resource server is not shared to the organization unit",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.resourceservice.resource_server_not_shared_to_ou_description",
			DefaultValue: "No sharing policy reaches the specified organization unit, so the resource server is " +
				"not available there",
		},
	}
	// ErrorMissingOUIDParameter is returned when an overlay-rules request does not say which organization
	// unit to resolve for.
	ErrorMissingOUIDParameter = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RES-1035",
		Error: tidcommon.I18nMessage{
			Key:          "error.resourceservice.missing_ou_id_parameter",
			DefaultValue: "Missing required parameter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.resourceservice.missing_ou_id_parameter_description",
			DefaultValue: "The 'ouId' query parameter is required",
		},
	}
	// ErrorDeploymentWideSharingNotAllowed is returned when a create through the API states a section
	// whose reach is not bounded by where the issuing organization unit sits in the tree.
	ErrorDeploymentWideSharingNotAllowed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RES-1036",
		Error: tidcommon.I18nMessage{
			Key:          "error.resourceservice.deployment_wide_sharing_not_allowed",
			DefaultValue: "Deployment-wide sharing is not allowed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.resourceservice.deployment_wide_sharing_not_allowed_description",
			DefaultValue: "A policy reaching all organization units or the roots of all other trees " +
				"can only be created from a declarative resource file or an import",
		},
	}
	// ErrorSharedResourceServerCannotMove is returned when a resource server that sharing policies govern
	// is moved to another organization unit or project.
	ErrorSharedResourceServerCannotMove = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RES-1037",
		Error: tidcommon.I18nMessage{
			Key:          "error.resourceservice.shared_resource_server_cannot_move",
			DefaultValue: "Resource server with sharing policies cannot be moved",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.resourceservice.shared_resource_server_cannot_move_description",
			DefaultValue: "The resource server is shared into other organization units. Delete its sharing " +
				"policies before moving it to another organization unit or project",
		},
	}
	// ErrorPermissionsRuleNotSupported is returned when a sharing policy's permissions rule names the
	// permissions it shares or withholds, which a policy cannot do yet.
	ErrorPermissionsRuleNotSupported = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RES-1038",
		Error: tidcommon.I18nMessage{
			Key:          "error.resourceservice.permissions_rule_not_supported",
			DefaultValue: "Sharing part of a resource server is not supported",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.resourceservice.permissions_rule_not_supported_description",
			DefaultValue: "A sharing policy shares the whole resource server. Its permissions rule cannot " +
				"name a value or excludedValues",
		},
	}
)

// Internal error constants.
var (
	// errResourceServerNotFound is returned when the resource server is not found.
	errResourceServerNotFound = errors.New("resource server not found")

	// errResourceNotFound is returned when the resource is not found.
	errResourceNotFound = errors.New("resource not found")

	// errActionNotFound is returned when the action is not found.
	errActionNotFound = errors.New("action not found")

	// errResultLimitExceededInCompositeMode is the internal sentinel error for composite mode limit exceeded.
	errResultLimitExceededInCompositeMode = errors.New("result limit exceeded in composite mode")

	// errUnknownDefaultResourceServer is returned when the configured resource server does not exist.
	errUnknownDefaultResourceServer = errors.New(
		"default resource server does not resolve to an existing resource server")

	// errDefaultResourceServerLookupFailed is returned when resource server lookup fails.
	errDefaultResourceServerLookupFailed = errors.New("failed to resolve default resource server")

	// errDependencyRegistryNotSet is returned when a cascade delete is attempted before the
	// dependency registry has been injected.
	errDependencyRegistryNotSet = errors.New("dependency registry not set")

	// errNilEventBus is returned when the service is initialized without the event bus it audits on.
	errNilEventBus = errors.New("the resource service needs an event bus")

	// errSharingPolicyRemovalRefused rolls back a resource server delete whose sharing policies could not be
	// removed.
	errSharingPolicyRemovalRefused = errors.New("sharing policy removal refused")
)

var (
	// errResourceServerNotFoundNoop is returned by deleteResourceServer when the resource server does not
	// exist. DeleteResourceServer turns it into a success, so it never reaches a caller.
	errResourceServerNotFoundNoop = &tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: ErrorResourceServerNotFound.Code,
	}

	// errResourceNotFoundNoop is returned by deleteResource when the resource, or its resource server, does
	// not exist. DeleteResource turns it into a success, so it never reaches a caller.
	errResourceNotFoundNoop = &tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: ErrorResourceNotFound.Code,
	}

	// errActionNotFoundNoop is returned by deleteAction when the action, or the resource server or resource
	// it is under, does not exist. DeleteAction turns it into a success, so it never reaches a caller.
	errActionNotFoundNoop = &tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: ErrorActionNotFound.Code,
	}

	// errSharingPolicyNotFoundNoop is returned by deleteSharingPolicy when the resource server holds no
	// policy under the requested id. DeleteSharingPolicy turns it into a success, so it never reaches a
	// caller.
	errSharingPolicyNotFoundNoop = &tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: ErrorSharingPolicyNotFound.Code,
	}
)

// sharingErrors answers the sharing framework's client errors in this service's own codes.
var sharingErrors = sharing.ErrorTranslation{
	PolicyNotFound:  ErrorSharingPolicyNotFound,
	PolicyExists:    ErrorSharingPolicyExists,
	VersionMismatch: ErrorSharingPolicyVersionMismatch,
	PolicyDeclared:  ErrorSharingPolicyDeclared,
	InvalidLimit:    ErrorInvalidLimit,
	InvalidOffset:   ErrorInvalidOffset,
	InvalidPolicy:   ErrorInvalidSharingPolicy,
}
