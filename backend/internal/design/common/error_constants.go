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

package common

import (
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
)

// Client errors for design resolve operations.
var (
	// ErrorInvalidResolveType is the error returned when resolve type parameter is missing or invalid.
	ErrorInvalidResolveType = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "DSR-1001",
		Error: core.I18nMessage{
			Key:          "design.resolve.error.invalid_type",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "design.resolve.error.invalid_type_description",
			DefaultValue: "The 'type' query parameter is required and must be either 'APP' or 'OU'",
		},
	}
	// ErrorMissingResolveID is the error returned when resolve id parameter is missing.
	ErrorMissingResolveID = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "DSR-1002",
		Error: core.I18nMessage{
			Key:          "design.resolve.error.missing_id",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "design.resolve.error.missing_id_description",
			DefaultValue: "The 'id' query parameter is required",
		},
	}
	// ErrorUnsupportedResolveType is the error returned when resolve type is not yet supported.
	ErrorUnsupportedResolveType = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "DSR-1003",
		Error: core.I18nMessage{
			Key:          "design.resolve.error.unsupported_type",
			DefaultValue: "Unsupported resolve type",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "design.resolve.error.unsupported_type_description",
			DefaultValue: "The specified resolve type is not yet supported. Currently only 'APP' type is supported",
		},
	}
	// ErrorApplicationNotFound is the error returned when an application is not found.
	ErrorApplicationNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "DSR-1004",
		Error: core.I18nMessage{
			Key:          "design.resolve.error.app_not_found",
			DefaultValue: "Application not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "design.resolve.error.app_not_found_description",
			DefaultValue: "The application with the specified id does not exist",
		},
	}
	// ErrorApplicationHasNoDesign is the error returned when an application has no associated design.
	ErrorApplicationHasNoDesign = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "DSR-1005",
		Error: core.I18nMessage{
			Key:          "design.resolve.error.app_no_design",
			DefaultValue: "Application has no design configuration",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "design.resolve.error.app_no_design_description",
			DefaultValue: "The specified application does not have an associated theme or layout configuration",
		},
	}
)
