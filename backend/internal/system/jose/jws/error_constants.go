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

package jws

import (
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
)

// Client errors for JWS operations
var (
	ErrorUnsupportedAlgorithm = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "JWS-1001",
		Error: core.I18nMessage{
			Key:          "error.jwsservice.unsupported_algorithm",
			DefaultValue: "Unsupported JWS algorithm",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.jwsservice.unsupported_algorithm_description",
			DefaultValue: "The specified JWS algorithm is not supported",
		},
	}

	ErrorInvalidSignature = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "JWS-1002",
		Error: core.I18nMessage{
			Key:          "error.jwsservice.invalid_signature",
			DefaultValue: "Invalid signature",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.jwsservice.invalid_signature_description",
			DefaultValue: "The signature is invalid",
		},
	}

	ErrorInvalidFormat = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "JWS-1003",
		Error: core.I18nMessage{
			Key:          "error.jwsservice.invalid_format",
			DefaultValue: "Invalid JWS format",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.jwsservice.invalid_format_description",
			DefaultValue: "The JWS token format is invalid",
		},
	}

	ErrorDecodingHeader = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "JWS-1004",
		Error: core.I18nMessage{
			Key:          "error.jwsservice.decoding_header_error",
			DefaultValue: "JWS decode error",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.jwsservice.decoding_header_error_description",
			DefaultValue: "Error occurred while decoding JWS header",
		},
	}
)
