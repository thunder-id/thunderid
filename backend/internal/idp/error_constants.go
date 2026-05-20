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

package idp

import (
	"errors"

	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
)

// ErrIDPNotFound is returned when the IdP is not found in the system.
var ErrIDPNotFound = errors.New("IdP not found")

// ErrResultLimitExceededInCompositeMode is the internal sentinel error for composite mode limit exceeded.
var ErrResultLimitExceededInCompositeMode = errors.New("result limit exceeded in composite mode")

// Client errors for identity provider operations.
var (
	// ErrorIDPNotFound is the error returned when an identity provider is not found.
	ErrorIDPNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "IDP-1001",
		Error: core.I18nMessage{
			Key:          "error.idpservice.idp_not_found",
			DefaultValue: "Identity provider not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.idpservice.idp_not_found_description",
			DefaultValue: "The requested identity provider could not be found",
		},
	}
	// ErrorInvalidIDPID is the error returned when an invalid identity provider ID is provided.
	ErrorInvalidIDPID = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "IDP-1002",
		Error: core.I18nMessage{
			Key:          "error.idpservice.invalid_idp_id",
			DefaultValue: "Invalid identity provider ID",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.idpservice.invalid_idp_id_description",
			DefaultValue: "The provided identity provider ID is invalid or empty",
		},
	}
	// ErrorInvalidIDPName is the error returned when an invalid identity provider name is provided.
	ErrorInvalidIDPName = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "IDP-1003",
		Error: core.I18nMessage{
			Key:          "error.idpservice.invalid_idp_name",
			DefaultValue: "Invalid identity provider name",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.idpservice.invalid_idp_name_description",
			DefaultValue: "The provided identity provider name is invalid or empty",
		},
	}
	// ErrorInvalidIDPType is the error returned when an invalid identity provider type is provided.
	ErrorInvalidIDPType = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "IDP-1004",
		Error: core.I18nMessage{
			Key:          "error.idpservice.invalid_idp_type",
			DefaultValue: "Invalid identity provider type",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.idpservice.invalid_idp_type_description",
			DefaultValue: "The provided identity provider type is invalid or empty",
		},
	}
	// ErrorIDPAlreadyExists is the error returned when an identity provider with the same name already exists.
	ErrorIDPAlreadyExists = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "IDP-1005",
		Error: core.I18nMessage{
			Key:          "error.idpservice.idp_already_exists",
			DefaultValue: "Identity provider already exists",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.idpservice.idp_already_exists_description",
			DefaultValue: "An identity provider with the same name already exists",
		},
	}
	// ErrorInvalidIDPProperty is the error returned when an invalid identity provider property is provided.
	ErrorInvalidIDPProperty = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "IDP-1006",
		Error: core.I18nMessage{
			Key:          "error.idpservice.invalid_idp_property",
			DefaultValue: "Invalid identity provider property",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.idpservice.invalid_idp_property_description",
			DefaultValue: "One or more identity provider properties are invalid or empty",
		},
	}
	// ErrorUnsupportedIDPProperty is the error returned when an unsupported identity provider property is provided.
	ErrorUnsupportedIDPProperty = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "IDP-1007",
		Error: core.I18nMessage{
			Key:          "error.idpservice.unsupported_idp_property",
			DefaultValue: "Unsupported identity provider property",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.idpservice.unsupported_idp_property_description",
			DefaultValue: "One or more identity provider properties are not supported",
		},
	}
	// ErrorIDPNil is the error returned when the identity provider object is nil.
	ErrorIDPNil = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "IDP-1008",
		Error: core.I18nMessage{
			Key:          "error.idpservice.idp_nil",
			DefaultValue: "Identity provider cannot be null",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.idpservice.idp_nil_description",
			DefaultValue: "The identity provider object cannot be null or empty",
		},
	}
	// ErrorInvalidRequestFormat is the error returned when the request format is invalid.
	ErrorInvalidRequestFormat = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "IDP-1009",
		Error: core.I18nMessage{
			Key:          "error.idpservice.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.idpservice.invalid_request_format_description",
			DefaultValue: "The request body is malformed or contains invalid data",
		},
	}
	// ErrorIDPDeclarativeReadOnly is the error returned when attempting to modify a declarative (immutable) IDP.
	ErrorIDPDeclarativeReadOnly = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "IDP-1010",
		Error: core.I18nMessage{
			Key:          "error.idpservice.idp_declarative_read_only",
			DefaultValue: "Identity provider is immutable",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.idpservice.idp_declarative_read_only_description",
			DefaultValue: "The requested identity provider is declarative and cannot be modified or deleted",
		},
	}
	// ErrorResultLimitExceededInCompositeMode is the error returned when the total number of records exceeds
	ErrorResultLimitExceededInCompositeMode = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "IDP-1011",
		Error: core.I18nMessage{
			Key:          "error.idpservice.result_limit_exceeded",
			DefaultValue: "Result limit exceeded in composite mode",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.idpservice.result_limit_exceeded_description",
			DefaultValue: "The total number of records exceeds the maximum limit in composite mode",
		},
	}
)
