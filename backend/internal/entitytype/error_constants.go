// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entitytype

import (
	"errors"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Client errors for entity type management operations.
var (
	// ErrorInvalidRequestFormat is the error returned when the request format is invalid.
	ErrorInvalidRequestFormat = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USRS-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.entitytypeservice.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.entitytypeservice.invalid_request_format_description",
			DefaultValue: "The request body is malformed or contains invalid data",
		},
	}
	// ErrorEntityTypeNotFound is the error returned when an entity type is not found.
	ErrorEntityTypeNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USRS-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.entitytypeservice.entity_type_not_found",
			DefaultValue: "Entity type not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.entitytypeservice.entity_type_not_found_description",
			DefaultValue: "The entity type with the specified id does not exist",
		},
	}
	// ErrorEntityTypeHandleConflict is the error returned when entity type handle already exists.
	ErrorEntityTypeHandleConflict = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USRS-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.entitytypeservice.entity_type_handle_conflict",
			DefaultValue: "Entity type handle conflict",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.entitytypeservice.entity_type_handle_conflict_description",
			DefaultValue: "An entity type with the same handle already exists",
		},
	}
	// ErrorInvalidEntityTypeRequest is the error returned when entity type request is invalid.
	ErrorInvalidEntityTypeRequest = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USRS-1004",
		Error: tidcommon.I18nMessage{
			Key:          "error.entitytypeservice.invalid_entity_type_request",
			DefaultValue: "Invalid entity type request",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.entitytypeservice.invalid_entity_type_request_description",
			DefaultValue: "The entity type request contains invalid or missing required fields",
		},
	}
	// ErrorInvalidLimit is the error returned when limit parameter is invalid.
	ErrorInvalidLimit = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USRS-1005",
		Error: tidcommon.I18nMessage{
			Key:          "error.entitytypeservice.invalid_limit_parameter",
			DefaultValue: "Invalid pagination parameter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.entitytypeservice.invalid_limit_parameter_description",
			DefaultValue: "The limit parameter must be a positive integer",
		},
	}
	// ErrorInvalidOffset is the error returned when offset parameter is invalid.
	ErrorInvalidOffset = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USRS-1006",
		Error: tidcommon.I18nMessage{
			Key:          "error.entitytypeservice.invalid_offset_parameter",
			DefaultValue: "Invalid pagination parameter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.entitytypeservice.invalid_offset_parameter_description",
			DefaultValue: "The offset parameter must be a non-negative integer",
		},
	}
	// ErrorCannotModifyDeclarativeResource is the error returned when trying to modify a declarative resource.
	ErrorCannotModifyDeclarativeResource = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USRS-1008",
		Error: tidcommon.I18nMessage{
			Key:          "error.entitytypeservice.cannot_modify_declarative_resource",
			DefaultValue: "Cannot modify declarative resource",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.entitytypeservice.cannot_modify_declarative_resource_description",
			DefaultValue: "The user type is declarative and cannot be modified or deleted",
		},
	}
	// ErrorInvalidDisplayAttribute is the error returned when the display attribute
	// does not reference a valid top-level attribute in the schema.
	ErrorInvalidDisplayAttribute = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USRS-1011",
		Error: tidcommon.I18nMessage{
			Key:          "error.entitytypeservice.invalid_display_attribute",
			DefaultValue: "Invalid display attribute",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.entitytypeservice.invalid_display_attribute_description",
			DefaultValue: "Display attribute must reference an attribute defined in the schema " +
				"(use dot notation for nested attributes, e.g. 'address.city')",
		},
	}
	// ErrorNonDisplayableAttribute is the error returned when the display attribute
	// references an attribute with a non-displayable type (e.g. object or array).
	ErrorNonDisplayableAttribute = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USRS-1012",
		Error: tidcommon.I18nMessage{
			Key:          "error.entitytypeservice.non_displayable_attribute_type",
			DefaultValue: "Non-displayable attribute type",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.entitytypeservice.non_displayable_attribute_type_description",
			DefaultValue: "Display attribute must reference a string or number type",
		},
	}
	// ErrorCredentialDisplayAttribute is the error returned when the display attribute
	// references an attribute marked as a credential.
	ErrorCredentialDisplayAttribute = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USRS-1013",
		Error: tidcommon.I18nMessage{
			Key:          "error.entitytypeservice.credential_attribute_not_allowed_as_display",
			DefaultValue: "Credential attribute not allowed as display",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.entitytypeservice.credential_attribute_not_allowed_as_display_description",
			DefaultValue: "Display attribute must not reference a credential attribute",
		},
	}

	// ErrorAgentTypeOnlyDefaultAllowed is returned when an agent type is created or updated with a
	// handle other than `default`.
	// Agent types are restricted to a single bootstrap-provisioned `default` schema.
	ErrorAgentTypeOnlyDefaultAllowed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USRS-1014",
		Error: tidcommon.I18nMessage{
			Key:          "error.entitytypeservice.agent_type_only_default_allowed",
			DefaultValue: "Only the default agent type is allowed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.entitytypeservice.agent_type_only_default_allowed_description",
			DefaultValue: "Agent types are restricted to a single 'default' schema; " +
				"other handles are not permitted",
		},
	}

	// ErrorAgentTypeCannotDelete is returned when an attempt is made to delete an agent type.
	// The default agent type cannot be removed; agent creation depends on it.
	ErrorAgentTypeCannotDelete = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USRS-1015",
		Error: tidcommon.I18nMessage{
			Key:          "error.entitytypeservice.agent_type_cannot_delete",
			DefaultValue: "Agent type cannot be deleted",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.entitytypeservice.agent_type_cannot_delete_description",
			DefaultValue: "The default agent type cannot be deleted. Edit the schema instead",
		},
	}

	// ErrorInvalidEntityTypeHandle is returned when the entity type handle has an invalid format.
	ErrorInvalidEntityTypeHandle = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USRS-1016",
		Error: tidcommon.I18nMessage{
			Key:          "error.entitytypeservice.invalid_entity_type_handle",
			DefaultValue: "Invalid entity type handle",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.entitytypeservice.invalid_entity_type_handle_description",
			DefaultValue: "The handle must contain only lowercase letters, numbers, hyphens and underscores, " +
				"and must start and end with a letter or a number",
		},
	}

	// ErrorEntityTypeHandleUpdateNotAllowed is returned when an update attempts to change the handle.
	ErrorEntityTypeHandleUpdateNotAllowed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USRS-1017",
		Error: tidcommon.I18nMessage{
			Key:          "error.entitytypeservice.entity_type_handle_update_not_allowed",
			DefaultValue: "Entity type handle cannot be changed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.entitytypeservice.entity_type_handle_update_not_allowed_description",
			DefaultValue: "The handle of an entity type cannot be changed once it is created",
		},
	}
)

// Per-category ServiceError constants — used as the actual returned errors.
// ErrorEntityTypeNotFound / ErrorEntityTypeHandleConflict / ErrorInvalidEntityTypeRequest
// are kept above solely for their .Code value (cross-package comparisons).
var (
	ErrorUserTypeNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USRS-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.entitytypeservice.user_type_not_found",
			DefaultValue: "User type not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.entitytypeservice.user_type_not_found_description",
			DefaultValue: "The user type with the specified id does not exist",
		},
	}
	ErrorAgentTypeNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USRS-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.entitytypeservice.agent_type_not_found",
			DefaultValue: "Agent type not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.entitytypeservice.agent_type_not_found_description",
			DefaultValue: "The agent type with the specified id does not exist",
		},
	}
	ErrorUserTypeHandleConflict = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USRS-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.entitytypeservice.user_type_handle_conflict",
			DefaultValue: "User type handle conflict",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.entitytypeservice.user_type_handle_conflict_description",
			DefaultValue: "A user type with the same handle already exists",
		},
	}
	ErrorAgentTypeHandleConflict = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USRS-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.entitytypeservice.agent_type_handle_conflict",
			DefaultValue: "Agent type handle conflict",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.entitytypeservice.agent_type_handle_conflict_description",
			DefaultValue: "An agent type with the same handle already exists",
		},
	}
	ErrorInvalidUserTypeRequest = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USRS-1004",
		Error: tidcommon.I18nMessage{
			Key:          "error.entitytypeservice.invalid_user_type_request",
			DefaultValue: "Invalid user type request",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.entitytypeservice.invalid_user_type_request_description",
			DefaultValue: "The user type request contains invalid or missing required fields",
		},
	}
	ErrorInvalidAgentTypeRequest = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "USRS-1004",
		Error: tidcommon.I18nMessage{
			Key:          "error.entitytypeservice.invalid_agent_type_request",
			DefaultValue: "Invalid agent type request",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.entitytypeservice.invalid_agent_type_request_description",
			DefaultValue: "The agent type request contains invalid or missing required fields",
		},
	}
)

// entityTypeNotFoundErr returns the category-specific not-found ServiceError.
func entityTypeNotFoundErr(category TypeCategory) *tidcommon.ServiceError {
	if category == TypeCategoryAgent {
		return &ErrorAgentTypeNotFound
	}
	return &ErrorUserTypeNotFound
}

// entityTypeHandleConflictErr returns the category-specific handle-conflict ServiceError.
func entityTypeHandleConflictErr(category TypeCategory) *tidcommon.ServiceError {
	if category == TypeCategoryAgent {
		return &ErrorAgentTypeHandleConflict
	}
	return &ErrorUserTypeHandleConflict
}

// invalidEntityTypeRequestErr returns the category-specific invalid-request ServiceError,
// with an optional detail appended to the description's default value.
func invalidEntityTypeRequestErr(category TypeCategory, detail string) *tidcommon.ServiceError {
	var e tidcommon.ServiceError
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

	// ErrEntityTypeAlreadyExists is returned when an entity type with the same handle already exists.
	ErrEntityTypeAlreadyExists = errors.New("user type already exists")

	// ErrInvalidSchemaDefinition is returned when the schema definition is invalid.
	ErrInvalidSchemaDefinition = errors.New("invalid schema definition")

	// errResultLimitExceededInCompositeMode is returned when the result limit is exceeded in composite mode.
	errResultLimitExceededInCompositeMode = errors.New("result limit exceeded in composite mode")
)
