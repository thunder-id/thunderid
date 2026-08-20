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

package environmentvariable

import (
	"errors"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Client and server errors for environment variable management operations.
var (
	// ErrorInvalidEnvironmentVariableRequest is returned when the request cannot be parsed or carries
	// invalid or missing fields.
	ErrorInvalidEnvironmentVariableRequest = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "ENV-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.environmentvariableservice.invalid_request",
			DefaultValue: "Invalid environment variable request",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.environmentvariableservice.invalid_request_description",
			DefaultValue: "The request is malformed, or the environment variable key does not start with a " +
				"letter or underscore and contain only letters, digits, and underscores",
		},
	}
	// ErrorEnvironmentVariableNotFound is returned when an environment variable does not exist.
	ErrorEnvironmentVariableNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "ENV-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.environmentvariableservice.environment_variable_not_found",
			DefaultValue: "Environment variable not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.environmentvariableservice.environment_variable_not_found_description",
			DefaultValue: "The environment variable with the specified id does not exist",
		},
	}
	// ErrorEnvironmentVariableKeyConflict is returned when a variable with the same key already exists.
	ErrorEnvironmentVariableKeyConflict = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "ENV-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.environmentvariableservice.environment_variable_key_conflict",
			DefaultValue: "Environment variable key conflict",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.environmentvariableservice.environment_variable_key_conflict_description",
			DefaultValue: "An environment variable with the same key already exists",
		},
	}
	// ErrorInternalServer is returned when an unexpected server-side error occurs.
	ErrorInternalServer = tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		Code: "ENV-5001",
		Error: tidcommon.I18nMessage{
			Key:          "error.environmentvariableservice.internal_server_error",
			DefaultValue: "Internal server error",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.environmentvariableservice.internal_server_error_description",
			DefaultValue: "An unexpected error occurred while processing the environment variable",
		},
	}
)

// errEnvironmentVariableNotFound is the sentinel error returned by the store when a row is absent.
var errEnvironmentVariableNotFound = errors.New("environment variable not found")
