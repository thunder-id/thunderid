/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package scim

import (
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Internal SCIM service errors.
// These codes are NEVER sent to SCIM clients over the wire.
// handleSCIMError translates these into the SCIM-standard wire format.

var (
	// ErrorInvalidRequestBody is returned when the request body is not valid JSON or
	// does not conform to the expected SCIM schema.
	ErrorInvalidRequestBody = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SCIM-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.scim.invalid_request_body",
			DefaultValue: "Invalid request body",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.scim.invalid_request_body_description",
			DefaultValue: "The request body is not valid JSON or does not conform to the SCIM schema.",
		},
	}
	// ErrorMissingSchemas is returned when the request body does not include
	// a schemas array per RFC 7644 §3.
	ErrorMissingSchemas = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SCIM-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.scim.missing_schemas",
			DefaultValue: "Missing schemas",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.scim.missing_schemas_description",
			DefaultValue: "The request must include a schemas array",
		},
	}
	// ErrorDuplicateSchemas is returned when the schemas array contains
	// duplicate URNs.
	ErrorDuplicateSchemas = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SCIM-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.scim.duplicate_schemas",
			DefaultValue: "Duplicate schemas",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.scim.duplicate_schemas_description",
			DefaultValue: "The schemas array must not contain duplicate URNs",
		},
	}
	// ErrorMissingCoreUserSchema is returned when the schemas array does not
	// include the SCIM Core User schema URN.
	ErrorMissingCoreUserSchema = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SCIM-1004",
		Error: tidcommon.I18nMessage{
			Key:          "error.scim.missing_core_user_schema",
			DefaultValue: "Missing SCIM core User schema",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.scim.missing_core_user_schema_description",
			DefaultValue: "The schemas array must include the SCIM Core User schema URN",
		},
	}
	// ErrorMissingCustomSchema is returned when the request does not include
	// exactly one ThunderID custom user schema URN.
	ErrorMissingCustomSchema = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SCIM-1005",
		Error: tidcommon.I18nMessage{
			Key:          "error.scim.missing_custom_schema",
			DefaultValue: "Missing custom schema",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.scim.missing_custom_schema_description",
			DefaultValue: "The request must include exactly one ThunderID custom user schema URN",
		},
	}
	// ErrorMultipleCustomSchemas is returned when the request includes more
	// than one ThunderID custom user schema URN.
	ErrorMultipleCustomSchemas = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SCIM-1006",
		Error: tidcommon.I18nMessage{
			Key:          "error.scim.multiple_custom_schemas",
			DefaultValue: "Multiple custom schemas",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.scim.multiple_custom_schemas_description",
			DefaultValue: "The request must include exactly one ThunderID custom user schema URN",
		},
	}
	// ErrorInvalidCustomSchemaURN is returned when the provided ThunderID
	// schema URN is malformed.
	ErrorInvalidCustomSchemaURN = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SCIM-1007",
		Error: tidcommon.I18nMessage{
			Key:          "error.scim.invalid_custom_schema_urn",
			DefaultValue: "Invalid custom schema URN",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.scim.invalid_custom_schema_urn_description",
			DefaultValue: "The provided ThunderID schema URN is malformed",
		},
	}
	// ErrorMissingCustomSchemaObject is returned when the request body does
	// not contain a key matching the custom schema URN.
	ErrorMissingCustomSchemaObject = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SCIM-1008",
		Error: tidcommon.I18nMessage{
			Key:          "error.scim.missing_custom_schema_object",
			DefaultValue: "Missing custom schema object",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.scim.missing_custom_schema_object_description",
			DefaultValue: "The request body must contain a key matching the custom schema URN",
		},
	}
	// ErrorUnknownUserType is returned when the user type derived from the
	// custom schema URN does not exist.
	ErrorUnknownUserType = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SCIM-1009",
		Error: tidcommon.I18nMessage{
			Key:          "error.scim.unknown_user_type",
			DefaultValue: "Unknown user type",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.scim.unknown_user_type_description",
			DefaultValue: "The user type derived from the custom schema URN does not exist",
		},
	}
	// ErrorUserNotFound is returned when no user exists for the specified ID.
	ErrorUserNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SCIM-1010",
		Error: tidcommon.I18nMessage{
			Key:          "error.scim.user_not_found",
			DefaultValue: "User not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.scim.user_not_found_description",
			DefaultValue: "The user with the specified ID does not exist",
		},
	}
	// ErrorSchemaNotFound is returned when no schema exists for the provided URN.
	ErrorSchemaNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SCIM-1011",
		Error: tidcommon.I18nMessage{
			Key:          "error.scim.schema_not_found",
			DefaultValue: "Schema not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.scim.schema_not_found_description",
			DefaultValue: "No schema exists for the provided URN",
		},
	}
	// ErrorUnsupportedOperation is returned when the requested SCIM operation
	// is not supported by this server.
	ErrorUnsupportedOperation = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SCIM-1012",
		Error: tidcommon.I18nMessage{
			Key:          "error.scim.unsupported_operation",
			DefaultValue: "Unsupported operation",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.scim.unsupported_operation_description",
			DefaultValue: "This SCIM operation is not supported",
		},
	}

	// ErrorResourceTypeNotFound is returned when the requested resource type ID
	// does not match any known resource type. ThunderID only exposes "User".
	ErrorResourceTypeNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SCIM-1013",
		Error: tidcommon.I18nMessage{
			Key:          "error.scim.resource_type_not_found",
			DefaultValue: "ResourceType not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.scim.resource_type_not_found_description",
			DefaultValue: "No resource type exists for the provided ID",
		},
	}

	// ErrorInternalServer is returned when an unexpected server-side error
	// occurs that is not attributable to the client request.
	ErrorInternalServer = tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		Code: "SCIM-1014",
		Error: tidcommon.I18nMessage{
			Key:          "error.scim.internal_server_error",
			DefaultValue: "Internal server error",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.scim.internal_server_error_description",
			DefaultValue: "An unexpected server-side error occurred",
		},
	}
)
