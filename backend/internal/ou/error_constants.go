// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ou

import (
	"errors"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
)

// Client errors for organization unit management operations.
var (
	// ErrorInvalidRequestFormat is the error returned when the request format is invalid.
	ErrorInvalidRequestFormat = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "OU-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.ouservice.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.ouservice.invalid_request_format_description",
			DefaultValue: "The request body is malformed, contains invalid data, or required fields are missing/empty",
		},
	}
	// ErrorMissingOUID is the error returned when organization unit ID is missing.
	ErrorMissingOUID = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "OU-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.ouservice.missing_ou_id",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.ouservice.missing_ou_id_description",
			DefaultValue: "Organization unit ID is required",
		},
	}
	// ErrorOrganizationUnitNotFound is the error returned when an organization unit is not found.
	ErrorOrganizationUnitNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "OU-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.ouservice.organization_unit_not_found",
			DefaultValue: "Organization unit not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.ouservice.organization_unit_not_found_description",
			DefaultValue: "The organization unit with the specified id does not exist",
		},
	}
	// ErrorOrganizationUnitNameConflict is the error returned when an organization unit name
	// conflicts. Defined by the provider contract, since the flow engine branches on its code.
	ErrorOrganizationUnitNameConflict = providers.ErrorOrganizationUnitNameConflict
	// ErrorParentOrganizationUnitNotFound is the error returned when parent organization unit is not found.
	ErrorParentOrganizationUnitNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "OU-1005",
		Error: tidcommon.I18nMessage{
			Key:          "error.ouservice.parent_organization_unit_not_found",
			DefaultValue: "Parent organization unit not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.ouservice.parent_organization_unit_not_found_description",
			DefaultValue: "Parent organization unit not found",
		},
	}
	// ErrorCannotDeleteOrganizationUnit is the error returned when organization unit cannot be deleted.
	ErrorCannotDeleteOrganizationUnit = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "OU-1006",
		Error: tidcommon.I18nMessage{
			Key:          "error.ouservice.organization_unit_has_children",
			DefaultValue: "Organization unit has children",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.ouservice.organization_unit_has_children_description",
			DefaultValue: "Cannot delete an organization unit that has child organization units, users, " +
				"groups, user types, agent types, connections, flows, or sharing policies it issued",
		},
	}
	// ErrorCircularDependency is the error returned when a circular dependency is detected.
	ErrorCircularDependency = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "OU-1007",
		Error: tidcommon.I18nMessage{
			Key:          "error.ouservice.circular_dependency_detected",
			DefaultValue: "Circular dependency detected",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.ouservice.circular_dependency_detected_description",
			DefaultValue: "Setting this parent would create a circular dependency",
		},
	}
	// ErrorOrganizationUnitHandleConflict is the error returned when an organization unit handle
	// conflicts. Defined by the provider contract, since the flow engine branches on its code.
	ErrorOrganizationUnitHandleConflict = providers.ErrorOrganizationUnitHandleConflict
	// ErrorInvalidHandlePath is the error returned when handle path is invalid.
	ErrorInvalidHandlePath = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "OU-1009",
		Error: tidcommon.I18nMessage{
			Key:          "error.ouservice.invalid_handle_path",
			DefaultValue: "Invalid handle path",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.ouservice.invalid_handle_path_description",
			DefaultValue: "The specified handle path does not exist",
		},
	}
	// ErrorInvalidLimit is the error returned when limit parameter is invalid.
	ErrorInvalidLimit = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "OU-1010",
		Error: tidcommon.I18nMessage{
			Key:          "error.ouservice.invalid_limit_parameter",
			DefaultValue: "Invalid limit parameter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.ouservice.invalid_limit_parameter_description",
			DefaultValue: "The limit parameter must be a positive integer",
		},
	}
	// ErrorInvalidOffset is the error returned when offset parameter is invalid.
	ErrorInvalidOffset = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "OU-1011",
		Error: tidcommon.I18nMessage{
			Key:          "error.ouservice.invalid_offset_parameter",
			DefaultValue: "Invalid offset parameter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.ouservice.invalid_offset_parameter_description",
			DefaultValue: "The offset parameter must be a non-negative integer",
		},
	}
	// ErrorCannotModifyDeclarativeResource is the error returned when trying to modify a declarative resource.
	ErrorCannotModifyDeclarativeResource = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "OU-1012",
		Error: tidcommon.I18nMessage{
			Key:          "error.ouservice.cannot_modify_declarative_resource",
			DefaultValue: "Cannot modify declarative resource",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.ouservice.cannot_modify_declarative_resource_description",
			DefaultValue: "The organization unit is declarative and cannot be modified or deleted",
		},
	}
	// ErrorResultLimitExceeded is the error returned when the result limit is exceeded in composite mode.
	ErrorResultLimitExceeded = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "OU-1013",
		Error: tidcommon.I18nMessage{
			Key:          "error.ouservice.result_limit_exceeded",
			DefaultValue: "Result limit exceeded",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.ouservice.result_limit_exceeded_description",
			DefaultValue: serverconst.CompositeStoreLimitWarning,
		},
	}
	// ErrorInvalidFilter is the error returned when the filter parameter is invalid.
	ErrorInvalidFilter = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "OU-1014",
		Error: tidcommon.I18nMessage{
			Key:          "error.ouservice.invalid_filter",
			DefaultValue: "Invalid filter parameter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.ouservice.invalid_filter_description",
			DefaultValue: "The filter parameter is invalid. Use format: attribute (eq|gt|lt) \"value\"",
		},
	}
)

// Error variables
var (
	// ErrOrganizationUnitNotFound is returned when the organization unit is not found in the system.
	ErrOrganizationUnitNotFound = errors.New("organization unit not found")
	// ErrCannotUpdateDeclarativeOU is returned when attempting to update a declarative organization unit.
	ErrCannotUpdateDeclarativeOU = errors.New("cannot update declarative organization unit")
	// ErrCannotDeleteDeclarativeOU is returned when attempting to delete a declarative organization unit.
	ErrCannotDeleteDeclarativeOU = errors.New("cannot delete declarative organization unit")
	// ErrResultLimitExceededInCompositeMode is returned when the result limit is exceeded in composite mode.
	ErrResultLimitExceededInCompositeMode = errors.New("result limit exceeded in composite mode")
)
