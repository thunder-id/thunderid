// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package variablestore

import "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

// Errors this service returns. The VAR prefix names the service that produced them.
//
// None of these say anything about a secret's value, including when one fails to decrypt: a caller
// learns that the store could not serve the request, not what it holds.
var (
	// ErrorInvalidRequestFormat is returned when a body cannot be read as the expected document.
	ErrorInvalidRequestFormat = common.ServiceError{
		Type: common.ClientErrorType,
		Code: "VAR-1001",
		Error: common.I18nMessage{
			Key:          "error.variablestore.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: common.I18nMessage{
			Key:          "error.variablestore.invalid_request_format_description",
			DefaultValue: "The request body could not be read as a variable store request",
		},
	}

	// ErrorInvalidName is returned when a name is empty, too long, or shaped wrongly.
	ErrorInvalidName = common.ServiceError{
		Type: common.ClientErrorType,
		Code: "VAR-1002",
		Error: common.I18nMessage{
			Key:          "error.variablestore.invalid_name",
			DefaultValue: "Invalid name",
		},
		ErrorDescription: common.I18nMessage{
			Key:          "error.variablestore.invalid_name_description",
			DefaultValue: "A name must match ^[A-Za-z_][A-Za-z0-9_]*$ and be at most 255 characters",
		},
	}

	// ErrorInvalidValue is returned when a value is missing or longer than the store accepts.
	ErrorInvalidValue = common.ServiceError{
		Type: common.ClientErrorType,
		Code: "VAR-1003",
		Error: common.I18nMessage{
			Key:          "error.variablestore.invalid_value",
			DefaultValue: "Invalid value",
		},
		ErrorDescription: common.I18nMessage{
			Key:          "error.variablestore.invalid_value_description",
			DefaultValue: "A value is required and must be at most 8192 characters",
		},
	}

	// ErrorNotFound is returned when nothing of that name is stored.
	ErrorNotFound = common.ServiceError{
		Type: common.ClientErrorType,
		Code: "VAR-1004",
		Error: common.I18nMessage{
			Key:          "error.variablestore.not_found",
			DefaultValue: "Not found",
		},
		ErrorDescription: common.I18nMessage{
			Key:          "error.variablestore.not_found_description",
			DefaultValue: "Nothing of that name is stored",
		},
	}

	// ErrorInvalidFilter is returned when a filter expression cannot be understood.
	ErrorInvalidFilter = common.ServiceError{
		Type: common.ClientErrorType,
		Code: "VAR-1005",
		Error: common.I18nMessage{
			Key:          "error.variablestore.invalid_filter",
			DefaultValue: "Invalid filter",
		},
		ErrorDescription: common.I18nMessage{
			Key: "error.variablestore.invalid_filter_description",
			DefaultValue: "A filter must read: name eq \"value\" or name sw \"value\". " +
				"Only the name attribute is filterable",
		},
	}

	// ErrorInvalidPagination is returned when limit or offset is outside what the store serves.
	ErrorInvalidPagination = common.ServiceError{
		Type: common.ClientErrorType,
		Code: "VAR-1006",
		Error: common.I18nMessage{
			Key:          "error.variablestore.invalid_pagination",
			DefaultValue: "Invalid pagination",
		},
		ErrorDescription: common.I18nMessage{
			Key:          "error.variablestore.invalid_pagination_description",
			DefaultValue: "limit must be between 1 and 100, and offset must not be negative",
		},
	}

	// ErrorTooManyNames is returned when a names= lookup asks for more than the store answers at once.
	ErrorTooManyNames = common.ServiceError{
		Type: common.ClientErrorType,
		Code: "VAR-1007",
		Error: common.I18nMessage{
			Key:          "error.variablestore.too_many_names",
			DefaultValue: "Too many names",
		},
		ErrorDescription: common.I18nMessage{
			Key:          "error.variablestore.too_many_names_description",
			DefaultValue: "At most 100 names may be requested at once",
		},
	}

	// ErrorUnknownParameter is returned when a query carries a parameter this listing does not
	// understand, or supplies a known one more than once or with no value.
	ErrorUnknownParameter = common.ServiceError{
		Type: common.ClientErrorType,
		Code: "VAR-1010",
		Error: common.I18nMessage{
			Key:          "error.variablestore.unknown_parameter",
			DefaultValue: "Unknown query parameter",
		},
		ErrorDescription: common.I18nMessage{
			Key:          "error.variablestore.unknown_parameter_description",
			DefaultValue: "Only limit, offset, names and filter are accepted, each given once and with a value",
		},
	}

	// ErrorInvalidDescription is returned when a description is longer than the column holds.
	ErrorInvalidDescription = common.ServiceError{
		Type: common.ClientErrorType,
		Code: "VAR-1008",
		Error: common.I18nMessage{
			Key:          "error.variablestore.invalid_description",
			DefaultValue: "Invalid description",
		},
		ErrorDescription: common.I18nMessage{
			Key:          "error.variablestore.invalid_description_description",
			DefaultValue: "A description may be at most 1000 characters",
		},
	}

	// ErrorAlreadyExists is returned when adding something whose name is taken in that collection.
	ErrorAlreadyExists = common.ServiceError{
		Type: common.ClientErrorType,
		Code: "VAR-1009",
		Error: common.I18nMessage{
			Key:          "error.variablestore.already_exists",
			DefaultValue: "Already exists",
		},
		ErrorDescription: common.I18nMessage{
			Key:          "error.variablestore.already_exists_description",
			DefaultValue: "Something of that name already exists. Use PUT to replace it",
		},
	}

	// ErrorInternalServerError is returned when the store cannot serve the request.
	ErrorInternalServerError = common.ServiceError{
		Type: common.ServerErrorType,
		Code: "VAR-5000",
		Error: common.I18nMessage{
			Key:          "error.variablestore.internal_error",
			DefaultValue: "Internal server error",
		},
		ErrorDescription: common.I18nMessage{
			Key:          "error.variablestore.internal_error_description",
			DefaultValue: "An unexpected error occurred while processing the request",
		},
	}
)
