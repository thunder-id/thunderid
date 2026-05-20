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

package entitytype

import (
	"errors"

	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
)

// Client errors for entity type management operations.
var (
	// ErrorInvalidRequestFormat is the error returned when the request format is invalid.
	ErrorInvalidRequestFormat = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USRS-1001",
		Error: core.I18nMessage{
			Key:          "error.entitytypeservice.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.entitytypeservice.invalid_request_format_description",
			DefaultValue: "The request body is malformed or contains invalid data",
		},
	}
	// ErrorEntityTypeNotFound is the error returned when an entity type is not found.
	ErrorEntityTypeNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USRS-1002",
		Error: core.I18nMessage{
			Key:          "error.entitytypeservice.entity_type_not_found",
			DefaultValue: "Entity type not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.entitytypeservice.entity_type_not_found_description",
			DefaultValue: "The entity type with the specified id does not exist",
		},
	}
	// ErrorEntityTypeNameConflict is the error returned when entity type name already exists.
	ErrorEntityTypeNameConflict = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USRS-1003",
		Error: core.I18nMessage{
			Key:          "error.entitytypeservice.entity_type_name_conflict",
			DefaultValue: "Entity type name conflict",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.entitytypeservice.entity_type_name_conflict_description",
			DefaultValue: "An entity type with the same name already exists",
		},
	}
	// ErrorInvalidEntityTypeRequest is the error returned when entity type request is invalid.
	ErrorInvalidEntityTypeRequest = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USRS-1004",
		Error: core.I18nMessage{
			Key:          "error.entitytypeservice.invalid_entity_type_request",
			DefaultValue: "Invalid entity type request",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.entitytypeservice.invalid_entity_type_request_description",
			DefaultValue: "The entity type request contains invalid or missing required fields",
		},
	}
	// ErrorInvalidLimit is the error returned when limit parameter is invalid.
	ErrorInvalidLimit = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USRS-1005",
		Error: core.I18nMessage{
			Key:          "error.entitytypeservice.invalid_limit_parameter",
			DefaultValue: "Invalid pagination parameter",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.entitytypeservice.invalid_limit_parameter_description",
			DefaultValue: "The limit parameter must be a positive integer",
		},
	}
	// ErrorInvalidOffset is the error returned when offset parameter is invalid.
	ErrorInvalidOffset = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USRS-1006",
		Error: core.I18nMessage{
			Key:          "error.entitytypeservice.invalid_offset_parameter",
			DefaultValue: "Invalid pagination parameter",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.entitytypeservice.invalid_offset_parameter_description",
			DefaultValue: "The offset parameter must be a non-negative integer",
		},
	}
	// ErrorUserValidationFailed is the error returned when user attributes do not conform to the schema.
	ErrorUserValidationFailed = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USRS-1007",
		Error: core.I18nMessage{
			Key:          "error.entitytypeservice.user_validation_failed",
			DefaultValue: "User validation failed",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.entitytypeservice.user_validation_failed_description",
			DefaultValue: "User attributes do not conform to the required schema",
		},
	}
	// ErrorCannotModifyDeclarativeResource is the error returned when trying to modify a declarative resource.
	ErrorCannotModifyDeclarativeResource = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USRS-1008",
		Error: core.I18nMessage{
			Key:          "error.entitytypeservice.cannot_modify_declarative_resource",
			DefaultValue: "Cannot modify declarative resource",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.entitytypeservice.cannot_modify_declarative_resource_description",
			DefaultValue: "The user type is declarative and cannot be modified or deleted",
		},
	}
	// ErrorResultLimitExceededInCompositeMode is the error returned when
	// the result limit is exceeded in composite mode.
	ErrorResultLimitExceededInCompositeMode = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USRS-1009",
		Error: core.I18nMessage{
			Key:          "error.entitytypeservice.result_limit_exceeded",
			DefaultValue: "Result limit exceeded",
		},
		ErrorDescription: core.I18nMessage{
			Key: "error.entitytypeservice.result_limit_exceeded_description",
			DefaultValue: "The combined result set from both file-based and database " +
				"stores exceeds the maximum limit. Please refine your query to return " +
				"fewer results.",
		},
	}
	// ErrorConsentSyncFailed is the error returned when entity type changes failed to sync with the consent service.
	ErrorConsentSyncFailed = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USRS-1010",
		Error: core.I18nMessage{
			Key:          "error.entitytypeservice.consent_synchronization_failed",
			DefaultValue: "Consent synchronization failed",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.entitytypeservice.consent_synchronization_failed_description",
			DefaultValue: "Failed to synchronize consent configurations for the entity type",
		},
	}
	// ErrorInvalidDisplayAttribute is the error returned when the display attribute
	// does not reference a valid top-level attribute in the schema.
	ErrorInvalidDisplayAttribute = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USRS-1011",
		Error: core.I18nMessage{
			Key:          "error.entitytypeservice.invalid_display_attribute",
			DefaultValue: "Invalid display attribute",
		},
		ErrorDescription: core.I18nMessage{
			Key: "error.entitytypeservice.invalid_display_attribute_description",
			DefaultValue: "Display attribute must reference an attribute defined in the schema " +
				"(use dot notation for nested attributes, e.g. 'address.city')",
		},
	}
	// ErrorNonDisplayableAttribute is the error returned when the display attribute
	// references an attribute with a non-displayable type (e.g. object or array).
	ErrorNonDisplayableAttribute = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USRS-1012",
		Error: core.I18nMessage{
			Key:          "error.entitytypeservice.non_displayable_attribute_type",
			DefaultValue: "Non-displayable attribute type",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.entitytypeservice.non_displayable_attribute_type_description",
			DefaultValue: "Display attribute must reference a string or number type",
		},
	}
	// ErrorCredentialDisplayAttribute is the error returned when the display attribute
	// references an attribute marked as a credential.
	ErrorCredentialDisplayAttribute = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USRS-1013",
		Error: core.I18nMessage{
			Key:          "error.entitytypeservice.credential_attribute_not_allowed_as_display",
			DefaultValue: "Credential attribute not allowed as display",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.entitytypeservice.credential_attribute_not_allowed_as_display_description",
			DefaultValue: "Display attribute must not reference a credential attribute",
		},
	}

	// ErrorAgentTypeOnlyDefaultAllowed is returned when a non-`default` agent type is created or renamed.
	// Agent types are restricted to a single bootstrap-provisioned `default` schema.
	ErrorAgentTypeOnlyDefaultAllowed = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USRS-1014",
		Error: core.I18nMessage{
			Key:          "error.entitytypeservice.agent_type_only_default_allowed",
			DefaultValue: "Only the default agent type is allowed",
		},
		ErrorDescription: core.I18nMessage{
			Key: "error.entitytypeservice.agent_type_only_default_allowed_description",
			DefaultValue: "Agent types are restricted to a single 'default' schema; " +
				"create or rename to other names is not permitted",
		},
	}

	// ErrorAgentTypeCannotDelete is returned when an attempt is made to delete an agent type.
	// The default agent type cannot be removed; agent creation depends on it.
	ErrorAgentTypeCannotDelete = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USRS-1015",
		Error: core.I18nMessage{
			Key:          "error.entitytypeservice.agent_type_cannot_delete",
			DefaultValue: "Agent type cannot be deleted",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.entitytypeservice.agent_type_cannot_delete_description",
			DefaultValue: "The default agent type cannot be deleted. Edit the schema instead",
		},
	}
)

// Per-category ServiceError constants — used as the actual returned errors.
// ErrorEntityTypeNotFound / ErrorEntityTypeNameConflict / ErrorInvalidEntityTypeRequest
// are kept above solely for their .Code value (cross-package comparisons).
var (
	ErrorUserTypeNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USRS-1002",
		Error: core.I18nMessage{
			Key:          "error.entitytypeservice.user_type_not_found",
			DefaultValue: "User type not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.entitytypeservice.user_type_not_found_description",
			DefaultValue: "The user type with the specified id does not exist",
		},
	}
	ErrorAgentTypeNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USRS-1002",
		Error: core.I18nMessage{
			Key:          "error.entitytypeservice.agent_type_not_found",
			DefaultValue: "Agent type not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.entitytypeservice.agent_type_not_found_description",
			DefaultValue: "The agent type with the specified id does not exist",
		},
	}
	ErrorUserTypeNameConflict = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USRS-1003",
		Error: core.I18nMessage{
			Key:          "error.entitytypeservice.user_type_name_conflict",
			DefaultValue: "User type name conflict",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.entitytypeservice.user_type_name_conflict_description",
			DefaultValue: "A user type with the same name already exists",
		},
	}
	ErrorAgentTypeNameConflict = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USRS-1003",
		Error: core.I18nMessage{
			Key:          "error.entitytypeservice.agent_type_name_conflict",
			DefaultValue: "Agent type name conflict",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.entitytypeservice.agent_type_name_conflict_description",
			DefaultValue: "An agent type with the same name already exists",
		},
	}
	ErrorInvalidUserTypeRequest = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USRS-1004",
		Error: core.I18nMessage{
			Key:          "error.entitytypeservice.invalid_user_type_request",
			DefaultValue: "Invalid user type request",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.entitytypeservice.invalid_user_type_request_description",
			DefaultValue: "The user type request contains invalid or missing required fields",
		},
	}
	ErrorInvalidAgentTypeRequest = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "USRS-1004",
		Error: core.I18nMessage{
			Key:          "error.entitytypeservice.invalid_agent_type_request",
			DefaultValue: "Invalid agent type request",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.entitytypeservice.invalid_agent_type_request_description",
			DefaultValue: "The agent type request contains invalid or missing required fields",
		},
	}
)

// entityTypeNotFoundErr returns the category-specific not-found ServiceError.
func entityTypeNotFoundErr(category TypeCategory) *serviceerror.ServiceError {
	if category == TypeCategoryAgent {
		return &ErrorAgentTypeNotFound
	}
	return &ErrorUserTypeNotFound
}

// entityTypeNameConflictErr returns the category-specific name-conflict ServiceError.
func entityTypeNameConflictErr(category TypeCategory) *serviceerror.ServiceError {
	if category == TypeCategoryAgent {
		return &ErrorAgentTypeNameConflict
	}
	return &ErrorUserTypeNameConflict
}

// invalidEntityTypeRequestErr returns the category-specific invalid-request ServiceError,
// with an optional detail appended to the description's default value.
func invalidEntityTypeRequestErr(category TypeCategory, detail string) *serviceerror.ServiceError {
	var e serviceerror.ServiceError
	if category == TypeCategoryAgent {
		e = ErrorInvalidAgentTypeRequest
	} else {
		e = ErrorInvalidUserTypeRequest
	}
	if detail != "" {
		e.ErrorDescription.DefaultValue += ": " + detail
	}
	return &e
}

// Error variables for entity type operations.
var (
	// ErrEntityTypeNotFound is returned when an entity type is not found in the system.
	ErrEntityTypeNotFound = errors.New("entity type not found")

	// ErrEntityTypeAlreadyExists is returned when an entity type with the same name already exists.
	ErrEntityTypeAlreadyExists = errors.New("user type already exists")

	// ErrInvalidSchemaDefinition is returned when the schema definition is invalid.
	ErrInvalidSchemaDefinition = errors.New("invalid schema definition")

	// errResultLimitExceededInCompositeMode is returned when the result limit is exceeded in composite mode.
	errResultLimitExceededInCompositeMode = errors.New("result limit exceeded in composite mode")
)
