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

package user

import (
	"errors"

	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
)

// Client errors for user management operations.
var (
	// ErrorInvalidRequestFormat is the error returned when the request format is invalid.
	ErrorInvalidRequestFormat = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USR-1001",
		Error: core.I18nMessage{
			Key:          "error.userservice.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.userservice.invalid_request_format_description",
			DefaultValue: "The request body is malformed or contains invalid data",
		},
	}
	// ErrorMissingUserID is the error returned when user ID is missing.
	ErrorMissingUserID = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USR-1002",
		Error: core.I18nMessage{
			Key:          "error.userservice.missing_user_id",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.userservice.missing_user_id_description",
			DefaultValue: "User ID is required",
		},
	}
	// ErrorUserNotFound is the error returned when a user is not found.
	ErrorUserNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USR-1003",
		Error: core.I18nMessage{
			Key:          "error.userservice.user_not_found",
			DefaultValue: "User not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.userservice.user_not_found_description",
			DefaultValue: "The user with the specified id does not exist",
		},
	}
	// ErrorOrganizationUnitNotFound is the error returned when an organization unit is not found.
	ErrorOrganizationUnitNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USR-1005",
		Error: core.I18nMessage{
			Key:          "error.userservice.organization_unit_not_found",
			DefaultValue: "Organization unit not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.userservice.organization_unit_not_found_description",
			DefaultValue: "The specified organization unit does not exist",
		},
	}
	// ErrorInvalidGroupID is the error returned when group ID is invalid.
	ErrorInvalidGroupID = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USR-1007",
		Error: core.I18nMessage{
			Key:          "error.userservice.invalid_group_id",
			DefaultValue: "Invalid group ID",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.userservice.invalid_group_id_description",
			DefaultValue: "One or more group IDs in the request do not exist",
		},
	}
	// ErrorHandlePathRequired is the error returned when handle path is missing.
	ErrorHandlePathRequired = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USR-1008",
		Error: core.I18nMessage{
			Key:          "error.userservice.handle_path_required",
			DefaultValue: "Handle path required",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.userservice.handle_path_required_description",
			DefaultValue: "Handle path is required for this operation",
		},
	}
	// ErrorInvalidHandlePath is the error returned when handle path format is invalid.
	ErrorInvalidHandlePath = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USR-1009",
		Error: core.I18nMessage{
			Key:          "error.userservice.invalid_handle_path",
			DefaultValue: "Invalid handle path",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.userservice.invalid_handle_path_description",
			DefaultValue: "Handle path must contain valid organizational unit identifiers separated by forward slashes",
		},
	}
	// ErrorInvalidLimit is the error returned when limit parameter is invalid.
	ErrorInvalidLimit = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USR-1011",
		Error: core.I18nMessage{
			Key:          "error.userservice.invalid_limit_parameter",
			DefaultValue: "Invalid pagination parameter",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.userservice.invalid_limit_parameter_description",
			DefaultValue: "The limit parameter must be a positive integer",
		},
	}
	// ErrorInvalidOffset is the error returned when offset parameter is invalid.
	ErrorInvalidOffset = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USR-1012",
		Error: core.I18nMessage{
			Key:          "error.userservice.invalid_offset_parameter",
			DefaultValue: "Invalid pagination parameter",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.userservice.invalid_offset_parameter_description",
			DefaultValue: "The offset parameter must be a non-negative integer",
		},
	}
	// ErrorAttributeConflict is the error returned when a unique attribute already exists.
	ErrorAttributeConflict = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USR-1014",
		Error: core.I18nMessage{
			Key:          "error.userservice.attribute_conflict",
			DefaultValue: "Attribute conflict",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.userservice.attribute_conflict_description",
			DefaultValue: "A user with the same unique attribute value already exists",
		},
	}
	// ErrorEmailConflict is the error returned when email already exists.
	ErrorEmailConflict = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USR-1015",
		Error: core.I18nMessage{
			Key:          "error.userservice.email_conflict",
			DefaultValue: "Email conflict",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.userservice.email_conflict_description",
			DefaultValue: "A user with the same email already exists",
		},
	}
	// ErrorMissingRequiredFields is the error returned when required fields are missing.
	ErrorMissingRequiredFields = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USR-1016",
		Error: core.I18nMessage{
			Key:          "error.userservice.missing_required_fields",
			DefaultValue: "Missing required fields",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.userservice.missing_required_fields_description",
			DefaultValue: "At least one identifying attribute must be provided",
		},
	}
	// ErrorMissingCredentials is the error returned when credentials are missing.
	ErrorMissingCredentials = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USR-1017",
		Error: core.I18nMessage{
			Key:          "error.userservice.missing_credentials",
			DefaultValue: "Missing credentials",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.userservice.missing_credentials_description",
			DefaultValue: "At least one credential field must be provided",
		},
	}
	// ErrorAuthenticationFailed is the error returned when authentication fails.
	ErrorAuthenticationFailed = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USR-1018",
		Error: core.I18nMessage{
			Key:          "error.userservice.authentication_failed",
			DefaultValue: "Authentication failed",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.userservice.authentication_failed_description",
			DefaultValue: "Invalid credentials provided",
		},
	}
	// ErrorSchemaValidationFailed is the error returned when user attributes fail schema validation.
	ErrorSchemaValidationFailed = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USR-1019",
		Error: core.I18nMessage{
			Key:          "error.userservice.schema_validation_failed",
			DefaultValue: "Schema validation failed",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.userservice.schema_validation_failed_description",
			DefaultValue: "User attributes do not conform to the required schema",
		},
	}
	// ErrorInvalidFilter is the error returned when the filter parameter is invalid.
	ErrorInvalidFilter = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USR-1020",
		Error: core.I18nMessage{
			Key:          "error.userservice.invalid_filter_parameter",
			DefaultValue: "Invalid filter parameter",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.userservice.invalid_filter_parameter_description",
			DefaultValue: "The filter format is invalid",
		},
	}
	// ErrorEntityTypeNotFound is the error returned when the specified user type is not found.
	ErrorEntityTypeNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USR-1021",
		Error: core.I18nMessage{
			Key:          "error.userservice.user_type_not_found",
			DefaultValue: "User type not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.userservice.user_type_not_found_description",
			DefaultValue: "The specified user type does not exist",
		},
	}
	// ErrorInvalidOUID is returned when the organization unit ID is missing or malformed.
	ErrorInvalidOUID = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USR-1022",
		Error: core.I18nMessage{
			Key:          "error.userservice.invalid_organization_unit",
			DefaultValue: "Invalid organization unit",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.userservice.invalid_organization_unit_description",
			DefaultValue: "Organization unit id must be specified as a valid UUID",
		},
	}
	// ErrorOrganizationUnitMismatch is returned when the organization unit does not match the user type definition.
	ErrorOrganizationUnitMismatch = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USR-1023",
		Error: core.I18nMessage{
			Key:          "error.userservice.organization_unit_mismatch",
			DefaultValue: "Organization unit mismatch",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.userservice.organization_unit_mismatch_description",
			DefaultValue: "The organization unit does not match the user type configuration",
		},
	}
	// ErrorInvalidCredential is the error returned when credentials are invalid.
	ErrorInvalidCredential = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USR-1024",
		Error: core.I18nMessage{
			Key:          "error.userservice.invalid_credential",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.userservice.invalid_credential_description",
			DefaultValue: "Invalid credential fields in request",
		},
	}
	// ErrorCannotModifyDeclarativeResource is the error returned when trying to modify a declarative user.
	ErrorCannotModifyDeclarativeResource = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USR-1025",
		Error: core.I18nMessage{
			Key:          "error.userservice.cannot_modify_declarative_resource",
			DefaultValue: "Cannot modify declarative resource",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.userservice.cannot_modify_declarative_resource_description",
			DefaultValue: "The user is declarative and cannot be modified or deleted",
		},
	}
	// ErrorAmbiguousUser is the error returned when multiple users match the provided filters.
	ErrorAmbiguousUser = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USR-1026",
		Error: core.I18nMessage{
			Key:          "error.userservice.ambiguous_user",
			DefaultValue: "Ambiguous user",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.userservice.ambiguous_user_description",
			DefaultValue: "Multiple users match the provided filters",
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
