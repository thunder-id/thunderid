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

package oidc

import (
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
)

// Client errors for OIDC authentication.
var (
	// ErrorInvalidIDToken is the error when the ID token is invalid or malformed.
	ErrorInvalidIDToken = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AUTH-OIDC-1001",
		Error: core.I18nMessage{
			Key:          "error.authoidcservice.invalid_id_token",
			DefaultValue: "Invalid ID token",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authoidcservice.invalid_id_token_description",
			DefaultValue: "The ID token is invalid or malformed",
		},
	}
	// ErrorInvalidIDTokenSignature is the error when the ID token signature verification fails.
	ErrorInvalidIDTokenSignature = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AUTH-OIDC-1002",
		Error: core.I18nMessage{
			Key:          "error.authoidcservice.invalid_id_token_signature",
			DefaultValue: "Invalid ID token signature",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authoidcservice.invalid_id_token_signature_description",
			DefaultValue: "The ID token signature verification failed",
		},
	}
)
