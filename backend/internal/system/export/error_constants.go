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

package export

import (
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
)

// Client errors for export operations.
var (
	// ErrorInvalidRequest is the error returned when an invalid export request is provided.
	ErrorInvalidRequest = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "EXP-1001",
		Error: core.I18nMessage{
			Key:          "error.exportservice.invalid_request",
			DefaultValue: "Invalid export request",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.exportservice.invalid_request_description",
			DefaultValue: "The provided export request is invalid or malformed",
		},
	}

	// ErrorNoResourcesFound is the error returned when no valid resources are found for export.
	ErrorNoResourcesFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "EXP-1002",
		Error: core.I18nMessage{
			Key:          "error.exportservice.no_resources_found",
			DefaultValue: "No resources found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.exportservice.no_resources_found_description",
			DefaultValue: "No valid resources found for the provided identifiers",
		},
	}
)
