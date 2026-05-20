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

// Package serviceerror defines the error structures for the service layer.
package serviceerror

import (
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
)

// ServiceErrorType defines the type of service error.
type ServiceErrorType string

const (
	// ClientErrorType denotes the client error type.
	ClientErrorType ServiceErrorType = "client_error"
	// ServerErrorType denotes the server error type.
	ServerErrorType ServiceErrorType = "server_error"
)

// CustomServiceError creates a new service error based on an existing error with a custom description.
// The caller must supply a complete core.I18nMessage with both Key and DefaultValue so that the
// translation system has a unique key to resolve, not the base error's generic key.
func CustomServiceError(svcError ServiceError, errorDesc core.I18nMessage) *ServiceError {
	err := &ServiceError{
		Type:             svcError.Type,
		Code:             svcError.Code,
		Error:            svcError.Error,
		ErrorDescription: svcError.ErrorDescription,
	}
	if !errorDesc.IsEmpty() {
		err.ErrorDescription = errorDesc
	}
	return err
}

// ServiceError defines a service error structure with i18n support.
// This is the new error type that should be used for services being migrated to i18n.
// Translatable fields use core.Message instead of plain strings.
type ServiceError struct {
	Code             string           `json:"code"`
	Type             ServiceErrorType `json:"type"`
	Error            core.I18nMessage `json:"error"`
	ErrorDescription core.I18nMessage `json:"error_description,omitempty"`
}

// Authorization errors
var (
	// ErrorUnauthorized is the error returned when the caller is not authorized to perform the operation.
	ErrorUnauthorized = ServiceError{
		Type: ClientErrorType,
		Code: "SSE-4030",
		Error: core.I18nMessage{
			Key:          "error.unauthorized",
			DefaultValue: "Unauthorized",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.unauthorized_description",
			DefaultValue: "The caller is not authorized to perform this operation",
		},
	}
)

// Server errors
var (
	// InternalServerError is the error returned for unexpected server errors.
	InternalServerError = ServiceError{
		Type: ServerErrorType,
		Code: "SSE-5000",
		Error: core.I18nMessage{
			Key:          "error.internal_server_error",
			DefaultValue: "Internal server error",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.internal_server_error_description",
			DefaultValue: "An unexpected error occurred while processing the request",
		},
	}

	// ErrorEncodingError is the error returned when encoding the response fails.
	ErrorEncodingError = ServiceError{
		Type: ServerErrorType,
		Code: "SSE-5001",
		Error: core.I18nMessage{
			Key:          "error.encoding_error",
			DefaultValue: "Encoding error",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.encoding_error_description",
			DefaultValue: "An error occurred while encoding the response",
		},
	}
)
