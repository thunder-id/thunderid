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
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Client errors for JWS operations
var (
	ErrorUnsupportedAlgorithm = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "JWS-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.jwsservice.unsupported_algorithm",
			DefaultValue: "Unsupported JWS algorithm",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.jwsservice.unsupported_algorithm_description",
			DefaultValue: "The specified JWS algorithm is not supported",
		},
	}

	ErrorInvalidSignature = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "JWS-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.jwsservice.invalid_signature",
			DefaultValue: "Invalid signature",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.jwsservice.invalid_signature_description",
			DefaultValue: "The signature is invalid",
		},
	}

	ErrorInvalidFormat = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "JWS-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.jwsservice.invalid_format",
			DefaultValue: "Invalid JWS format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.jwsservice.invalid_format_description",
			DefaultValue: "The JWS token format is invalid",
		},
	}

	ErrorDecodingHeader = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "JWS-1004",
		Error: tidcommon.I18nMessage{
			Key:          "error.jwsservice.decoding_header_error",
			DefaultValue: "JWS decode error",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.jwsservice.decoding_header_error_description",
			DefaultValue: "Error occurred while decoding JWS header",
		},
	}
)
