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

package template

import (
	"errors"

	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
)

// Internal error definitions for template operations.
var (
	// errTemplateNotFound indicates the requested template was not found.
	errTemplateNotFound = errors.New("template not found")
)

// Client errors for template operations.
var (
	// ErrorTemplateNotFound is returned when the requested template does not exist.
	ErrorTemplateNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "TMP-1001",
		Error: core.I18nMessage{
			Key:          "error.templateservice.template_not_found",
			DefaultValue: "Template not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.templateservice.template_not_found_description",
			DefaultValue: "The requested template does not exist for the given scenario",
		},
	}
)
