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

package serverconfig

import (
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
)

// Client errors for server-config operations.
var (
	// ErrorUnsupportedConfigName is the error returned when the config name is not supported.
	ErrorUnsupportedConfigName = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "SCF-1001",
		Error: core.I18nMessage{
			Key:          "error.serverconfigservice.unsupported_config_name",
			DefaultValue: "Unsupported configuration name",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.serverconfigservice.unsupported_config_name_description",
			DefaultValue: "The requested server configuration name is not supported",
		},
	}

	// ErrorConfigNotFound is the error returned when the requested config is not found.
	ErrorConfigNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "SCF-1002",
		Error: core.I18nMessage{
			Key:          "error.serverconfigservice.config_not_found",
			DefaultValue: "Server configuration not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.serverconfigservice.config_not_found_description",
			DefaultValue: "The requested server configuration does not exist",
		},
	}

	// ErrorInvalidConfigValue is the error returned when the config value is invalid.
	ErrorInvalidConfigValue = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "SCF-1003",
		Error: core.I18nMessage{
			Key:          "error.serverconfigservice.invalid_config_value",
			DefaultValue: "Invalid server configuration value",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.serverconfigservice.invalid_config_value_description",
			DefaultValue: "The provided server configuration value is invalid",
		},
	}

	// ErrorInvalidRequestFormat is the error returned when the request body is malformed.
	ErrorInvalidRequestFormat = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "SCF-1004",
		Error: core.I18nMessage{
			Key:          "error.serverconfigservice.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.serverconfigservice.invalid_request_format_description",
			DefaultValue: "The request body is malformed or contains invalid data",
		},
	}
)
