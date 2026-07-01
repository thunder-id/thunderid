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

package interceptor

import (
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// ErrorInterceptorFailed defines the error for interceptor validation failures.
var ErrorInterceptorFailed = tidcommon.ServiceError{
	Code: "ICS-1001",
	Type: tidcommon.ClientErrorType,
	Error: tidcommon.I18nMessage{
		Key:          "error.interceptor.failed",
		DefaultValue: "Interceptor validation failed",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.interceptor.failed_description",
		DefaultValue: "A flow interceptor rejected the request",
	},
}

// ErrorChallengeTokenInvalid defines the error when a challenge token is not provided.
var ErrorChallengeTokenInvalid = tidcommon.ServiceError{
	Code: "ICS-1002",
	Type: tidcommon.ClientErrorType,
	Error: tidcommon.I18nMessage{
		Key:          "error.interceptor.challenge_token_invalid",
		DefaultValue: "Invalid challenge token",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.interceptor.challenge_token_invalid_description",
		DefaultValue: "The challenge token is missing or invalid",
	},
}
