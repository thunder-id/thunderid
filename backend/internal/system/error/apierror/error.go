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

// Package apierror defines the error structures for the API.
package apierror

import (
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
)

// ErrorResponse defines an API error response with i18n support.
type ErrorResponse struct {
	Code        string           `json:"code"`
	Message     core.I18nMessage `json:"message"`
	Description core.I18nMessage `json:"description"`
}

// Authentication and authorization error responses, returned by the security middleware.
var (
	// ErrUnauthorized is returned when authentication credentials are missing or invalid (HTTP 401).
	ErrUnauthorized = ErrorResponse{
		Code: "AUTH-4010",
		Message: core.I18nMessage{
			Key:          "error.auth.unauthorized",
			DefaultValue: "Unauthorized",
		},
		Description: core.I18nMessage{
			Key:          "error.auth.unauthorized_description",
			DefaultValue: "Authentication is required to access this resource",
		},
	}

	// ErrForbidden is returned when the caller is authenticated but lacks sufficient permissions (HTTP 403).
	ErrForbidden = ErrorResponse{
		Code: "AUTH-4030",
		Message: core.I18nMessage{
			Key:          "error.auth.forbidden",
			DefaultValue: "Forbidden",
		},
		Description: core.I18nMessage{
			Key:          "error.auth.forbidden_description",
			DefaultValue: "You do not have sufficient permissions to access this resource",
		},
	}
)
