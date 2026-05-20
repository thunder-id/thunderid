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

package ou

import (
	"errors"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
)

// Client errors for organization unit management operations.
var (
	// ErrorInvalidRequestFormat is the error returned when the request format is invalid.
	ErrorInvalidRequestFormat = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "OU-1001",
		Error: core.I18nMessage{
			Key:          "error.ouservice.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.ouservice.invalid_request_format_description",
			DefaultValue: "The request body is malformed, contains invalid data, or required fields are missing/empty",
		},
	}
	// ErrorMissingOUID is the error returned when organization unit ID is missing.
	ErrorMissingOUID = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "OU-1002",
		Error: core.I18nMessage{
			Key:          "error.ouservice.missing_ou_id",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.ouservice.missing_ou_id_description",
			DefaultValue: "Organization unit ID is required",
		},
	}
	// ErrorOrganizationUnitNotFound is the error returned when an organization unit is not found.
	ErrorOrganizationUnitNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "OU-1003",
		Error: core.I18nMessage{
			Key:          "error.ouservice.organization_unit_not_found",
			DefaultValue: "Organization unit not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.ouservice.organization_unit_not_found_description",
			DefaultValue: "The organization unit with the specified id does not exist",
		},
	}
	// ErrorOrganizationUnitNameConflict is the error returned when an organization unit name conflicts.
	ErrorOrganizationUnitNameConflict = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "OU-1004",
		Error: core.I18nMessage{
			Key:          "error.ouservice.organization_unit_name_conflict",
			DefaultValue: "Organization unit name conflict",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.ouservice.organization_unit_name_conflict_description",
			DefaultValue: "An organization unit with the same name exists under the same parent",
		},
	}
	// ErrorParentOrganizationUnitNotFound is the error returned when parent organization unit is not found.
	ErrorParentOrganizationUnitNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "OU-1005",
		Error: core.I18nMessage{
			Key:          "error.ouservice.parent_organization_unit_not_found",
			DefaultValue: "Parent organization unit not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.ouservice.parent_organization_unit_not_found_description",
			DefaultValue: "Parent organization unit not found",
		},
	}
	// ErrorCannotDeleteOrganizationUnit is the error returned when organization unit cannot be deleted.
	ErrorCannotDeleteOrganizationUnit = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "OU-1006",
		Error: core.I18nMessage{
			Key:          "error.ouservice.organization_unit_has_children",
			DefaultValue: "Organization unit has children",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.ouservice.organization_unit_has_children_description",
			DefaultValue: "Cannot delete organization unit with children or users/groups",
		},
	}
	// ErrorCircularDependency is the error returned when a circular dependency is detected.
	ErrorCircularDependency = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "OU-1007",
		Error: core.I18nMessage{
			Key:          "error.ouservice.circular_dependency_detected",
			DefaultValue: "Circular dependency detected",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.ouservice.circular_dependency_detected_description",
			DefaultValue: "Setting this parent would create a circular dependency",
		},
	}
	// ErrorOrganizationUnitHandleConflict is the error returned when an organization unit handle conflicts.
	ErrorOrganizationUnitHandleConflict = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "OU-1008",
		Error: core.I18nMessage{
			Key:          "error.ouservice.organization_unit_handle_conflict",
			DefaultValue: "Organization unit handle conflict",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.ouservice.organization_unit_handle_conflict_description",
			DefaultValue: "An organization unit with the same handle already exists under the same parent",
		},
	}
	// ErrorInvalidHandlePath is the error returned when handle path is invalid.
	ErrorInvalidHandlePath = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "OU-1009",
		Error: core.I18nMessage{
			Key:          "error.ouservice.invalid_handle_path",
			DefaultValue: "Invalid handle path",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.ouservice.invalid_handle_path_description",
			DefaultValue: "The specified handle path does not exist",
		},
	}
	// ErrorInvalidLimit is the error returned when limit parameter is invalid.
	ErrorInvalidLimit = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "OU-1010",
		Error: core.I18nMessage{
			Key:          "error.ouservice.invalid_limit_parameter",
			DefaultValue: "Invalid limit parameter",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.ouservice.invalid_limit_parameter_description",
			DefaultValue: "The limit parameter must be a positive integer",
		},
	}
	// ErrorInvalidOffset is the error returned when offset parameter is invalid.
	ErrorInvalidOffset = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "OU-1011",
		Error: core.I18nMessage{
			Key:          "error.ouservice.invalid_offset_parameter",
			DefaultValue: "Invalid offset parameter",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.ouservice.invalid_offset_parameter_description",
			DefaultValue: "The offset parameter must be a non-negative integer",
		},
	}
	// ErrorCannotModifyDeclarativeResource is the error returned when trying to modify a declarative resource.
	ErrorCannotModifyDeclarativeResource = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "OU-1012",
		Error: core.I18nMessage{
			Key:          "error.ouservice.cannot_modify_declarative_resource",
			DefaultValue: "Cannot modify declarative resource",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.ouservice.cannot_modify_declarative_resource_description",
			DefaultValue: "The organization unit is declarative and cannot be modified or deleted",
		},
	}
	// ErrorResultLimitExceeded is the error returned when the result limit is exceeded in composite mode.
	ErrorResultLimitExceeded = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "OU-1013",
		Error: core.I18nMessage{
			Key:          "error.ouservice.result_limit_exceeded",
			DefaultValue: "Result limit exceeded",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.ouservice.result_limit_exceeded_description",
			DefaultValue: serverconst.CompositeStoreLimitWarning,
		},
	}
	// ErrorInvalidFilter is the error returned when the filter parameter is invalid.
	ErrorInvalidFilter = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "OU-1014",
		Error: core.I18nMessage{
			Key:          "error.ouservice.invalid_filter",
			DefaultValue: "Invalid filter parameter",
		},
		ErrorDescription: core.I18nMessage{
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
