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

package connection

import (
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Client errors for connection operations.
var (
	// ErrorInvalidConnectionCategory is the error returned when the category query parameter
	// on GET /connections is not a recognized category.
	ErrorInvalidConnectionCategory = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CON-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_category",
			DefaultValue: "Invalid connection category",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_category_description",
			DefaultValue: "The category must be one of: identity-provider, sms-provider",
		},
	}
	// ErrorInvalidLimit is the error returned when an invalid limit query parameter is provided.
	ErrorInvalidLimit = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CON-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_limit_parameter",
			DefaultValue: "Invalid limit parameter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_limit_parameter_description",
			DefaultValue: "The limit parameter must be a positive integer",
		},
	}
	// ErrorInvalidOffset is the error returned when an invalid offset query parameter is provided.
	ErrorInvalidOffset = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CON-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_offset_parameter",
			DefaultValue: "Invalid offset parameter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_offset_parameter_description",
			DefaultValue: "The offset parameter must be a non-negative integer",
		},
	}
)
