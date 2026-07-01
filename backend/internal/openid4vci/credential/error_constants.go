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

package credential

import (
	"errors"
	"net/http"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Internal sentinel errors for the composite/file-based credential store.
var (
	// ErrNotFound is the store-level not-found sentinel.
	ErrNotFound = errors.New("openid4vci: credential configuration not found")

	// ErrConfigurationIsImmutable is returned when trying to modify or delete an
	// immutable (file-based) credential configuration.
	ErrConfigurationIsImmutable = errors.New("credential configuration is immutable")

	// ErrResultLimitExceededInCompositeMode is returned when composite store results
	// exceed the configured limit.
	ErrResultLimitExceededInCompositeMode = errors.New("result limit exceeded in composite mode")

	// ErrConfigurationDataCorrupted is returned when declarative store data is malformed.
	ErrConfigurationDataCorrupted = errors.New("credential configuration data is corrupted")
)

// Client-facing API errors for the credential-configuration management endpoints.
var (
	// ErrorConfigurationInvalidRequest indicates a malformed create/update request.
	ErrorConfigurationInvalidRequest = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "VCI-2001",
		Error: tidcommon.I18nMessage{
			Key:          "error.vci.configuration_invalid_request",
			DefaultValue: "Invalid request",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.vci.configuration_invalid_request_description",
			DefaultValue: "The credential configuration request is missing required fields or is malformed",
		},
	}

	// ErrorConfigurationNotFound indicates the credential configuration does not exist.
	ErrorConfigurationNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "VCI-2002",
		Error: tidcommon.I18nMessage{
			Key:          "error.vci.configuration_not_found",
			DefaultValue: "Credential configuration not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.vci.configuration_not_found_description",
			DefaultValue: "No credential configuration exists for the supplied identifier",
		},
	}

	// ErrorConfigurationAlreadyExists indicates the handle is already in use.
	ErrorConfigurationAlreadyExists = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "VCI-2003",
		Error: tidcommon.I18nMessage{
			Key:          "error.vci.configuration_already_exists",
			DefaultValue: "Credential configuration already exists",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.vci.configuration_already_exists_description",
			DefaultValue: "A credential configuration with the supplied handle already exists",
		},
	}

	// ErrorConfigurationUnsupportedFormat indicates an unsupported credential format.
	ErrorConfigurationUnsupportedFormat = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "VCI-2004",
		Error: tidcommon.I18nMessage{
			Key:          "error.vci.configuration_unsupported_format",
			DefaultValue: "Unsupported credential format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.vci.configuration_unsupported_format_description",
			DefaultValue: "Only the dc+sd-jwt credential format is supported",
		},
	}

	// ErrorConfigurationImmutable indicates the credential configuration is declarative
	// (file-based) and cannot be modified or deleted via the management API.
	ErrorConfigurationImmutable = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "VCI-2005",
		Error: tidcommon.I18nMessage{
			Key:          "error.vci.configuration_immutable",
			DefaultValue: "Credential configuration is immutable",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.vci.configuration_immutable_description",
			DefaultValue: "The credential configuration is defined in declarative configuration " +
				"and cannot be modified or deleted",
		},
	}

	// ErrorConfigurationResultLimitExceeded indicates the merged composite-store result
	// set exceeds the supported maximum.
	ErrorConfigurationResultLimitExceeded = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "VCI-2006",
		Error: tidcommon.I18nMessage{
			Key:          "error.vci.configuration_result_limit_exceeded",
			DefaultValue: "Result limit exceeded",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.vci.configuration_result_limit_exceeded_description",
			DefaultValue: "The number of credential configurations exceeds the supported limit in " +
				"hybrid mode",
		},
	}

	// ErrorConfigurationInvalidOU indicates the organization unit is missing or does not exist.
	ErrorConfigurationInvalidOU = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "VCI-2007",
		Error: tidcommon.I18nMessage{
			Key:          "error.vci.configuration_invalid_ou",
			DefaultValue: "Invalid organization unit",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.vci.configuration_invalid_ou_description",
			DefaultValue: "A valid organization unit (ouId or ouHandle) is required",
		},
	}
)

// configurationClientErrorStatus maps a client-facing error to its HTTP status.
func configurationClientErrorStatus(code string) int {
	switch code {
	case ErrorConfigurationNotFound.Code:
		return http.StatusNotFound
	case ErrorConfigurationAlreadyExists.Code, ErrorConfigurationImmutable.Code:
		return http.StatusConflict
	default:
		return http.StatusBadRequest
	}
}
