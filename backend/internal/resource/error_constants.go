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

package resource

import (
	"errors"

	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
)

// Client errors for resource management operations.
var (
	// ErrorInvalidRequestFormat is returned when the request format is invalid.
	ErrorInvalidRequestFormat = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "RES-1001",
		Error: core.I18nMessage{
			Key:          "error.resourceservice.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.resourceservice.invalid_request_format_description",
			DefaultValue: "The request body is malformed or contains invalid data",
		},
	}
	// ErrorMissingID is returned when resource server/resource/action ID is missing.
	ErrorMissingID = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "RES-1002",
		Error: core.I18nMessage{
			Key:          "error.resourceservice.missing_id",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.resourceservice.missing_id_description",
			DefaultValue: "ID is required",
		},
	}
	// ErrorResourceServerNotFound is returned when a resource server is not found.
	ErrorResourceServerNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "RES-1003",
		Error: core.I18nMessage{
			Key:          "error.resourceservice.resource_server_not_found",
			DefaultValue: "Resource server not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.resourceservice.resource_server_not_found_description",
			DefaultValue: "The resource server with the specified id does not exist",
		},
	}
	// ErrorNameConflict is returned when a name already exists.
	ErrorNameConflict = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "RES-1004",
		Error: core.I18nMessage{
			Key:          "error.resourceservice.name_conflict",
			DefaultValue: "Name conflict",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.resourceservice.name_conflict_description",
			DefaultValue: "A resource server with the same name already exists",
		},
	}
	// ErrorParentResourceNotFound is returned when a parent resource is not found.
	ErrorParentResourceNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "RES-1005",
		Error: core.I18nMessage{
			Key:          "error.resourceservice.parent_resource_not_found",
			DefaultValue: "Parent resource not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.resourceservice.parent_resource_not_found_description",
			DefaultValue: "The specified parent resource does not exist",
		},
	}
	// ErrorCannotDelete is returned when resource server/resource cannot be deleted.
	ErrorCannotDelete = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "RES-1006",
		Error: core.I18nMessage{
			Key:          "error.resourceservice.cannot_delete",
			DefaultValue: "Cannot delete",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.resourceservice.cannot_delete_description",
			DefaultValue: "Cannot delete resource server/resource that has dependencies",
		},
	}
	// ErrorCircularDependency is returned when a circular dependency is detected.
	ErrorCircularDependency = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "RES-1007",
		Error: core.I18nMessage{
			Key:          "error.resourceservice.circular_dependency_detected",
			DefaultValue: "Circular dependency detected",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.resourceservice.circular_dependency_detected_description",
			DefaultValue: "Setting this parent would create a circular dependency",
		},
	}
	// ErrorResourceNotFound is returned when a resource is not found.
	ErrorResourceNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "RES-1008",
		Error: core.I18nMessage{
			Key:          "error.resourceservice.resource_not_found",
			DefaultValue: "Resource not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.resourceservice.resource_not_found_description",
			DefaultValue: "The resource with the specified id does not exist",
		},
	}
	// ErrorActionNotFound is returned when an action is not found.
	ErrorActionNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "RES-1009",
		Error: core.I18nMessage{
			Key:          "error.resourceservice.action_not_found",
			DefaultValue: "Action not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.resourceservice.action_not_found_description",
			DefaultValue: "The action with the specified id does not exist",
		},
	}
	// ErrorOrganizationUnitNotFound is returned when organization unit is not found.
	ErrorOrganizationUnitNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "RES-1010",
		Error: core.I18nMessage{
			Key:          "error.resourceservice.organization_unit_not_found",
			DefaultValue: "Organization unit not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.resourceservice.organization_unit_not_found_description",
			DefaultValue: "The specified organization unit does not exist",
		},
	}
	// ErrorInvalidLimit is returned when limit parameter is invalid.
	ErrorInvalidLimit = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "RES-1011",
		Error: core.I18nMessage{
			Key:          "error.resourceservice.invalid_limit_parameter",
			DefaultValue: "Invalid limit parameter",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.resourceservice.invalid_limit_parameter_description",
			DefaultValue: "The limit parameter must be a positive integer",
		},
	}
	// ErrorInvalidOffset is returned when offset parameter is invalid.
	ErrorInvalidOffset = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "RES-1012",
		Error: core.I18nMessage{
			Key:          "error.resourceservice.invalid_offset_parameter",
			DefaultValue: "Invalid offset parameter",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.resourceservice.invalid_offset_parameter_description",
			DefaultValue: "The offset parameter must be a non-negative integer",
		},
	}
	// ErrorIdentifierConflict is returned when an identifier already exists.
	ErrorIdentifierConflict = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "RES-1013",
		Error: core.I18nMessage{
			Key:          "error.resourceservice.identifier_conflict",
			DefaultValue: "Identifier conflict",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.resourceservice.identifier_conflict_description",
			DefaultValue: "A resource server with the same identifier already exists",
		},
	}
	// ErrorHandleConflict is returned when a handle already exists.
	ErrorHandleConflict = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "RES-1014",
		Error: core.I18nMessage{
			Key:          "error.resourceservice.handle_conflict",
			DefaultValue: "Handle conflict",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.resourceservice.handle_conflict_description",
			DefaultValue: "The same handle already exists within the specified resource",
		},
	}
	// ErrorInvalidDelimiter is returned when delimiter is invalid.
	ErrorInvalidDelimiter = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "RES-1015",
		Error: core.I18nMessage{
			Key:          "error.resourceservice.invalid_delimiter",
			DefaultValue: "Invalid delimiter",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.resourceservice.invalid_delimiter_description",
			DefaultValue: "Delimiter must be a single valid character (a-z A-Z 0-9 . _ : - /)",
		},
	}
	// ErrorInvalidHandle is returned when handle contains invalid characters.
	ErrorInvalidHandle = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "RES-1016",
		Error: core.I18nMessage{
			Key:          "error.resourceservice.invalid_handle",
			DefaultValue: "Invalid handle",
		},
		ErrorDescription: core.I18nMessage{
			Key: "error.resourceservice.invalid_handle_description",
			DefaultValue: "Handle length must be less than 100 characters " +
				"and contain valid characters (a-z A-Z 0-9 . _ : - /)",
		},
	}
	// ErrorDelimiterInHandle is returned when handle contains invalid characters.
	ErrorDelimiterInHandle = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "RES-1017",
		Error: core.I18nMessage{
			Key:          "error.resourceservice.delimiter_conflict_in_handle",
			DefaultValue: "Delimiter conflict in handle",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.resourceservice.delimiter_conflict_in_handle_description",
			DefaultValue: "Handle cannot contain the delimiter character",
		},
	}
	// ErrorImmutableResourceServer is returned when attempting to modify a declarative resource server.
	ErrorImmutableResourceServer = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "RES-1018",
		Error: core.I18nMessage{
			Key:          "error.resourceservice.cannot_modify_declarative_resource_server",
			DefaultValue: "Cannot modify declarative resource server",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.resourceservice.cannot_modify_declarative_resource_server_description",
			DefaultValue: "Resource server %s is defined in declarative configuration and cannot be modified",
		},
	}
	// ErrorImmutableResource is returned when attempting to modify a declarative resource.
	ErrorImmutableResource = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "RES-1019",
		Error: core.I18nMessage{
			Key:          "error.resourceservice.cannot_modify_declarative_resource",
			DefaultValue: "Cannot modify declarative resource",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.resourceservice.cannot_modify_declarative_resource_description",
			DefaultValue: "Resource %s is defined in declarative configuration and cannot be modified",
		},
	}
	// ErrorImmutableAction is returned when attempting to modify a declarative action.
	ErrorImmutableAction = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "RES-1020",
		Error: core.I18nMessage{
			Key:          "error.resourceservice.cannot_modify_declarative_action",
			DefaultValue: "Cannot modify declarative action",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.resourceservice.cannot_modify_declarative_action_description",
			DefaultValue: "Action %s is defined in declarative configuration and cannot be modified",
		},
	}
	// ErrResultLimitExceededInCompositeMode is the error returned when the total number of records exceeds
	ErrResultLimitExceededInCompositeMode = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "RES-1021",
		Error: core.I18nMessage{
			Key:          "error.resourceservice.result_limit_exceeded_in_composite_mode",
			DefaultValue: "Result limit exceeded in composite mode",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.resourceservice.result_limit_exceeded_in_composite_mode_description",
			DefaultValue: "The total number of records exceeds the maximum limit in composite mode",
		},
	}
	// ErrorResourceServerIDConflict is returned when a resource server with the specified ID already exists.
	ErrorResourceServerIDConflict = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "RES-1023",
		Error: core.I18nMessage{
			Key:          "error.resourceservice.resource_server_id_conflict",
			DefaultValue: "Resource server ID conflict",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.resourceservice.resource_server_id_conflict_description",
			DefaultValue: "A resource server with the specified ID already exists",
		},
	}
	// ErrorDelimiterInResourceServerHandle is returned when the resource server handle contains the delimiter.
	ErrorDelimiterInResourceServerHandle = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "RES-1022",
		Error: core.I18nMessage{
			Key:          "error.resourceservice.delimiter_conflict_in_resource_server_handle",
			DefaultValue: "Delimiter conflict in handle",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.resourceservice.delimiter_conflict_in_resource_server_handle_description",
			DefaultValue: "Resource server handle cannot contain the delimiter character",
		},
	}
	// ErrorConsentSyncFailed is returned when resource permission changes fail to sync with the consent service.
	ErrorConsentSyncFailed = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "RES-1024",
		Error: core.I18nMessage{
			Key:          "error.resourceservice.consent_sync_failed",
			DefaultValue: "Consent sync failed",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.resourceservice.consent_sync_failed_description",
			DefaultValue: "Failed to sync resource permission changes with the consent service",
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
)

// consentSyncError wraps an underlying ServiceError from the consent service, allowing callers
// to translate consent-service failures encountered during resource CRUD into their own error vocabulary.
type consentSyncError struct {
	Underlying *serviceerror.ServiceError
}

// Error implements the error interface. Falls back through (description → code → generic) so
// the returned string is never empty even when the underlying error has no description.
func (e *consentSyncError) Error() string {
	if e.Underlying != nil {
		if msg := e.Underlying.ErrorDescription.DefaultValue; msg != "" {
			return msg
		}
		if e.Underlying.Code != "" {
			return "consent sync failed (code " + e.Underlying.Code + ")"
		}
	}
	return "consent sync failed"
}

// IsClientError reports whether the underlying error is a client error.
func (e *consentSyncError) IsClientError() bool {
	return e.Underlying != nil && e.Underlying.Type == serviceerror.ClientErrorType
}
