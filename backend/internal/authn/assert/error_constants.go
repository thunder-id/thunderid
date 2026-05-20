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

package assert

import (
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
)

// Client errors for authentication assertion operations.
var (
	// ErrorNoAuthenticators is the error returned when no authenticators are provided.
	ErrorNoAuthenticators = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AST-1001",
		Error: core.I18nMessage{
			Key:          "error.assertservice.no_authenticators",
			DefaultValue: "No authenticators",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.assertservice.no_authenticators_description",
			DefaultValue: "Cannot generate assertion without authenticators",
		},
	}
	// ErrorInvalidAuthenticator is the error returned when authenticator name is invalid.
	ErrorInvalidAuthenticator = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AST-1002",
		Error: core.I18nMessage{
			Key:          "error.assertservice.invalid_authenticator",
			DefaultValue: "Invalid authenticator",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.assertservice.invalid_authenticator_description",
			DefaultValue: "Authenticator name cannot be empty",
		},
	}
	// ErrorNilAssuranceContext is the error returned when assurance context is nil.
	ErrorNilAssuranceContext = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AST-1003",
		Error: core.I18nMessage{
			Key:          "error.assertservice.nil_assurance_context",
			DefaultValue: "Nil assurance context",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.assertservice.nil_assurance_context_description",
			DefaultValue: "Assurance context cannot be nil for verification",
		},
	}
	// ErrorNoAssuranceRequirements is the error returned when no assurance requirements are specified.
	ErrorNoAssuranceRequirements = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AST-1004",
		Error: core.I18nMessage{
			Key:          "error.assertservice.no_assurance_requirements",
			DefaultValue: "No assurance requirements",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.assertservice.no_assurance_requirements_description",
			DefaultValue: "At least one assurance level (AAL or IAL) must be specified for verification",
		},
	}
)
