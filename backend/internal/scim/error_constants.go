package scim

import (
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
)

// Internal SCIM service errors.
// These codes are NEVER sent to SCIM clients over the wire.
// handleSCIMError translates these into the SCIM-standard wire format.

var (
	ErrorInvalidRequestBody = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "SCIM-1001",
		Error: core.I18nMessage{
			Key:          "error.scim.invalid_request_body",
			DefaultValue: "Invalid request body",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.scim.invalid_request_body_description",
			DefaultValue: "The request body is not a valid JSON object or does not conform to the expected SCIM schema.",
		},
	}
	ErrorMissingSchemas = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "SCIM-1002",
		Error: core.I18nMessage{
			Key:          "error.scim.missing_schemas",
			DefaultValue: "Missing schemas",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.scim.missing_schemas_description",
			DefaultValue: "The request must include a schemas array",
		},
	}
	ErrorDuplicateSchemas = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "SCIM-1003",
		Error: core.I18nMessage{
			Key:          "error.scim.duplicate_schemas",
			DefaultValue: "Duplicate schemas",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.scim.duplicate_schemas_description",
			DefaultValue: "The schemas array must not contain duplicate URNs",
		},
	}
	ErrorMissingCoreUserSchema = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "SCIM-1004",
		Error: core.I18nMessage{
			Key:          "error.scim.missing_core_user_schema",
			DefaultValue: "Missing core User schema",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.scim.missing_core_user_schema_description",
			DefaultValue: "The schemas array must include the SCIM core User schema URN",
		},
	}
	ErrorMissingCustomSchema = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "SCIM-1005",
		Error: core.I18nMessage{
			Key:          "error.scim.missing_custom_schema",
			DefaultValue: "Missing custom schema",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.scim.missing_custom_schema_description",
			DefaultValue: "The request must include exactly one ThunderID custom user schema URN",
		},
	}
	ErrorMultipleCustomSchemas = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "SCIM-1006",
		Error: core.I18nMessage{
			Key:          "error.scim.multiple_custom_schemas",
			DefaultValue: "Multiple custom schemas",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.scim.multiple_custom_schemas_description",
			DefaultValue: "The request must include exactly one ThunderID custom user schema URN",
		},
	}
	ErrorInvalidCustomSchemaURN = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "SCIM-1007",
		Error: core.I18nMessage{
			Key:          "error.scim.invalid_custom_schema_urn",
			DefaultValue: "Invalid custom schema URN",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.scim.invalid_custom_schema_urn_description",
			DefaultValue: "The provided ThunderID schema URN is malformed",
		},
	}
	ErrorMissingCustomSchemaObject = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "SCIM-1008",
		Error: core.I18nMessage{
			Key:          "error.scim.missing_custom_schema_object",
			DefaultValue: "Missing custom schema object",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.scim.missing_custom_schema_object_description",
			DefaultValue: "The request body must contain a key matching the custom schema URN",
		},
	}
	ErrorUnknownUserType = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "SCIM-1009",
		Error: core.I18nMessage{
			Key:          "error.scim.unknown_user_type",
			DefaultValue: "Unknown user type",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.scim.unknown_user_type_description",
			DefaultValue: "The user type derived from the custom schema URN does not exist",
		},
	}
	ErrorUserNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "SCIM-1010",
		Error: core.I18nMessage{
			Key:          "error.scim.user_not_found",
			DefaultValue: "User not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.scim.user_not_found_description",
			DefaultValue: "The user with the specified ID does not exist",
		},
	}
	ErrorSchemaNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "SCIM-1011",
		Error: core.I18nMessage{
			Key:          "error.scim.schema_not_found",
			DefaultValue: "Schema not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.scim.schema_not_found_description",
			DefaultValue: "No schema exists for the provided URN",
		},
	}
	ErrorUnsupportedOperation = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "SCIM-1012",
		Error: core.I18nMessage{
			Key:          "error.scim.unsupported_operation",
			DefaultValue: "Unsupported operation",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.scim.unsupported_operation_description",
			DefaultValue: "This SCIM operation is not supported",
		},
	}
)
