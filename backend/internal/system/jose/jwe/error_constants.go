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

package jwe

import (
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
)

// Client errors for JWE service
var (
	// ErrorDecodingJWE is the error returned when decoding the JWE token fails.
	ErrorDecodingJWE = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "JWE-1001",
		Error: core.I18nMessage{
			Key:          "error.jweservice.decoding_jwe_error",
			DefaultValue: "JWE decode error",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.jweservice.decoding_jwe_error_description",
			DefaultValue: "Error occurred while decoding JWE token",
		},
	}

	// ErrorJWEDecryptionFailed is the error returned when the JWE token decryption fails.
	ErrorJWEDecryptionFailed = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "JWE-1002",
		Error: core.I18nMessage{
			Key:          "error.jweservice.decryption_failed",
			DefaultValue: "JWE decryption failed",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.jweservice.decryption_failed_description",
			DefaultValue: "Failed to decrypt the JWE token",
		},
	}

	// ErrorUnsupportedJWEAlgorithm is the error returned when the JWE algorithm is unsupported.
	ErrorUnsupportedJWEAlgorithm = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "JWE-1003",
		Error: core.I18nMessage{
			Key:          "error.jweservice.unsupported_algorithm",
			DefaultValue: "Unsupported JWE algorithm",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.jweservice.unsupported_algorithm_description",
			DefaultValue: "The specified JWE algorithm is not supported",
		},
	}

	// ErrorUnsupportedEncryptionAlgorithm is the error returned when the encryption algorithm is unsupported.
	ErrorUnsupportedEncryptionAlgorithm = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "JWE-1004",
		Error: core.I18nMessage{
			Key:          "error.jweservice.unsupported_encryption_algorithm",
			DefaultValue: "Unsupported encryption algorithm",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.jweservice.unsupported_encryption_algorithm_description",
			DefaultValue: "The specified encryption algorithm is not supported",
		},
	}
)
