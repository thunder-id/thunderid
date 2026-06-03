/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package role

import (
	"errors"

	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
)

// Client errors for role management operations.
var (
	// ErrorInvalidRequestFormat is the error returned when the request format is invalid.
	ErrorInvalidRequestFormat = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "ROL-1001",
		Error: core.I18nMessage{
			Key:          "error.roleservice.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.roleservice.invalid_request_format_description",
			DefaultValue: "The request body is malformed or contains invalid data",
		},
	}
	// ErrorMissingRoleID is the error returned when role ID is missing.
	ErrorMissingRoleID = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "ROL-1002",
		Error: core.I18nMessage{
			Key:          "error.roleservice.missing_role_id",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.roleservice.missing_role_id_description",
			DefaultValue: "Role ID is required",
		},
	}
	// ErrorRoleNotFound is the error returned when a role is not found.
	ErrorRoleNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "ROL-1003",
		Error: core.I18nMessage{
			Key:          "error.roleservice.role_not_found",
			DefaultValue: "Role not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.roleservice.role_not_found_description",
			DefaultValue: "The role with the specified id does not exist",
		},
	}
	// ErrorRoleNameConflict is the error returned when a role name already exists in the organization unit.
	ErrorRoleNameConflict = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "ROL-1004",
		Error: core.I18nMessage{
			Key:          "error.roleservice.role_name_conflict",
			DefaultValue: "Role name conflict",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.roleservice.role_name_conflict_description",
			DefaultValue: "A role with the same name exists under the same organization unit",
		},
	}
	// ErrorOrganizationUnitNotFound is the error returned when organization unit is not found.
	ErrorOrganizationUnitNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "ROL-1005",
		Error: core.I18nMessage{
			Key:          "error.roleservice.organization_unit_not_found",
			DefaultValue: "Organization unit not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.roleservice.organization_unit_not_found_description",
			DefaultValue: "Organization unit not found",
		},
	}
	// ErrorCannotDeleteRole is the error returned when role cannot be deleted.
	ErrorCannotDeleteRole = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "ROL-1006",
		Error: core.I18nMessage{
			Key:          "error.roleservice.cannot_delete_role",
			DefaultValue: "Cannot delete role",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.roleservice.cannot_delete_role_description",
			DefaultValue: "Cannot delete role that is currently assigned to users or groups",
		},
	}
	// ErrorInvalidAssignmentID is the error returned when assignment ID is invalid.
	ErrorInvalidAssignmentID = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "ROL-1007",
		Error: core.I18nMessage{
			Key:          "error.roleservice.invalid_assignment_id",
			DefaultValue: "Invalid assignment ID",
		},
		ErrorDescription: core.I18nMessage{
			Key: "error.roleservice.invalid_assignment_id_description",
			DefaultValue: "One or more assignment IDs in the request do not exist " +
				"or do not match the claimed type",
		},
	}
	// ErrorInvalidLimit is the error returned when limit parameter is invalid.
	ErrorInvalidLimit = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "ROL-1008",
		Error: core.I18nMessage{
			Key:          "error.roleservice.invalid_limit_parameter",
			DefaultValue: "Invalid limit parameter",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.roleservice.invalid_limit_parameter_description",
			DefaultValue: "The limit parameter must be a positive integer",
		},
	}
	// ErrorInvalidOffset is the error returned when offset parameter is invalid.
	ErrorInvalidOffset = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "ROL-1009",
		Error: core.I18nMessage{
			Key:          "error.roleservice.invalid_offset_parameter",
			DefaultValue: "Invalid offset parameter",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.roleservice.invalid_offset_parameter_description",
			DefaultValue: "The offset parameter must be a non-negative integer",
		},
	}
	// ErrorEmptyAssignments is the error returned when assignments list is empty.
	ErrorEmptyAssignments = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "ROL-1010",
		Error: core.I18nMessage{
			Key:          "error.roleservice.empty_assignments_list",
			DefaultValue: "Empty assignments list",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.roleservice.empty_assignments_list_description",
			DefaultValue: "At least one assignment must be provided",
		},
	}
	// ErrorMissingEntityOrGroups is the error returned when both entity ID and groups are missing.
	ErrorMissingEntityOrGroups = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "ROL-1011",
		Error: core.I18nMessage{
			Key:          "error.roleservice.missing_entity_or_groups",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: core.I18nMessage{
			Key: "error.roleservice." +
				"either_entity_id_or_groups_must_be_provided_for_authorization_check_description",
			DefaultValue: "Either entityId or groups must be provided for authorization check",
		},
	}
	// ErrorInvalidPermissions is returned when one or more permissions are invalid.
	ErrorInvalidPermissions = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "ROL-1012",
		Error: core.I18nMessage{
			Key:          "error.roleservice.invalid_permissions",
			DefaultValue: "Invalid permissions",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.roleservice.invalid_permissions_description",
			DefaultValue: "One or more permissions do not exist in the resource management system",
		},
	}
	// ErrorImmutableRole is the error returned when attempting to modify a declarative role.
	ErrorImmutableRole = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "ROL-1013",
		Error: core.I18nMessage{
			Key:          "error.roleservice.cannot_modify_declarative_role",
			DefaultValue: "Cannot modify declarative role",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.roleservice.cannot_modify_declarative_role_description",
			DefaultValue: "The role is defined in declarative configuration and cannot be modified",
		},
	}
	// ErrorImmutableAssignment is the error returned when attempting to modify a declarative assignment.
	ErrorImmutableAssignment = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "ROL-1014",
		Error: core.I18nMessage{
			Key:          "error.roleservice.cannot_modify_declarative_assignment",
			DefaultValue: "Cannot modify declarative assignment",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.roleservice.cannot_modify_declarative_assignment_description",
			DefaultValue: "The assignment is defined in declarative configuration and cannot be modified",
		},
	}
	// ErrorDeclarativeModeCreateNotAllowed is the error returned when attempting to create
	// a role in declarative-only mode.
	ErrorDeclarativeModeCreateNotAllowed = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "ROL-1015",
		Error: core.I18nMessage{
			Key:          "error.roleservice.cannot_create_role_in_declarative_only_mode",
			DefaultValue: "Cannot create role in declarative-only mode",
		},
		ErrorDescription: core.I18nMessage{
			Key: "error.roleservice.cannot_create_role_in_declarative_only_mode_description",
			DefaultValue: "Role creation is not allowed when running in declarative-only mode. " +
				"Roles must be defined in declarative configuration files",
		},
	}
	// ErrorInvalidAssigneeType is the error returned when the assignee type query parameter is invalid.
	ErrorInvalidAssigneeType = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "ROL-1016",
		Error: core.I18nMessage{
			Key:          "error.roleservice.invalid_assignee_type",
			DefaultValue: "Invalid assignee type",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.roleservice.invalid_assignee_type_description",
			DefaultValue: "The type parameter must be 'user', 'group', or 'app'",
		},
	}
	// ErrorRoleIDConflict is the error returned when a role with the specified ID already exists.
	ErrorRoleIDConflict = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "ROL-1018",
		Error: core.I18nMessage{
			Key:          "error.roleservice.role_id_conflict",
			DefaultValue: "Role ID conflict",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.roleservice.role_id_conflict_description",
			DefaultValue: "A role with the specified ID already exists",
		},
	}
	// ResultLimitExceededInCompositeMode is the error returned when the total number of records exceeds
	// the maximum limit in composite mode (combining database and declarative resources).
	ResultLimitExceededInCompositeMode = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "ROL-1017",
		Error: core.I18nMessage{
			Key:          "error.roleservice.result_limit_exceeded_in_composite_mode",
			DefaultValue: "Result limit exceeded in composite mode",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.roleservice.result_limit_exceeded_in_composite_mode_description",
			DefaultValue: "The total number of records exceeds the maximum limit in composite mode",
		},
	}
)

// Internal error constants for role management operations.
var (
	// ErrRoleNotFound is returned when the role is not found in the system.
	ErrRoleNotFound = errors.New("role not found")

	// ErrRoleDataCorrupted is returned by the file-based store when a stored entry cannot
	// be converted into a role (type assertion / parse failure). Exposed as a sentinel so
	// callers can use errors.Is to skip these benign cases without conflating them with
	// actionable I/O errors. The originating site already logs the underlying details.
	ErrRoleDataCorrupted = errors.New("role data corrupted")

	// errResultLimitExceededInCompositeMode is the internal sentinel error for composite mode limit exceeded.
	errResultLimitExceededInCompositeMode = errors.New("result limit exceeded in composite mode")
)
