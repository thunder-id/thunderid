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

package jwt

import (
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Client errors for JWT service
var (
	ErrorDecodingJWTHeader = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "JWT-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.jwtservice.decoding_header_error",
			DefaultValue: "JWT decode error",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.jwtservice.decoding_header_error_description",
			DefaultValue: "Error occurred while decoding JWT header",
		},
	}

	ErrorDecodingJWTPayload = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "JWT-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.jwtservice.decoding_payload_error",
			DefaultValue: "JWT decode error",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.jwtservice.decoding_payload_error_description",
			DefaultValue: "Error occurred while decoding JWT payload",
		},
	}

	ErrorUnsupportedJWSAlgorithm = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "JWT-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.jwtservice.unsupported_jws_algorithm",
			DefaultValue: "Unsupported JWS algorithm",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.jwtservice.unsupported_jws_algorithm_description",
			DefaultValue: "The specified JWS algorithm is not supported",
		},
	}

	ErrorInvalidTokenSignature = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "JWT-1004",
		Error: tidcommon.I18nMessage{
			Key:          "error.jwtservice.invalid_token_signature",
			DefaultValue: "Invalid token signature",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.jwtservice.invalid_token_signature_description",
			DefaultValue: "The JWT token signature is invalid",
		},
	}

	ErrorInvalidJWTFormat = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "JWT-1005",
		Error: tidcommon.I18nMessage{
			Key:          "error.jwtservice.invalid_jwt_format",
			DefaultValue: "Invalid JWT format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.jwtservice.invalid_jwt_format_description",
			DefaultValue: "The JWT token format is invalid",
		},
	}

	ErrorNoMatchingJWKFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "JWT-1006",
		Error: tidcommon.I18nMessage{
			Key:          "error.jwtservice.no_matching_jwk_found",
			DefaultValue: "No matching JWK found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.jwtservice.no_matching_jwk_found_description",
			DefaultValue: "No matching JWK found for the given Key ID",
		},
	}

	ErrorTokenExpired = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "JWT-1007",
		Error: tidcommon.I18nMessage{
			Key:          "error.jwtservice.token_expired",
			DefaultValue: "Token expired",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.jwtservice.token_expired_description",
			DefaultValue: "The JWT token has expired",
		},
	}

	ErrorFailedToGetJWKS = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "JWT-1008",
		Error: tidcommon.I18nMessage{
			Key:          "error.jwtservice.failed_to_get_jwks",
			DefaultValue: "Failed to retrieve JWKS",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.jwtservice.failed_to_get_jwks_description",
			DefaultValue: "Failed to retrieve JWKS from the specified URL",
		},
	}

	ErrorFailedToParseJWKS = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "JWT-1009",
		Error: tidcommon.I18nMessage{
			Key:          "error.jwtservice.failed_to_parse_jwks",
			DefaultValue: "Failed to parse JWKS",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.jwtservice.failed_to_parse_jwks_description",
			DefaultValue: "Failed to parse JWKS",
		},
	}
)
