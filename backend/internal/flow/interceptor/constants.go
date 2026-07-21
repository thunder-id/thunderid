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

const (
	// PriorityDefault is the priority for default (always-enforced) interceptors.
	PriorityDefault = 100

	// BasePriorityConfigurable is the base priority for configurable (flow-declared) interceptors.
	BasePriorityConfigurable = 200
)

// Interceptor name constants.
const (
	// ChallengeTokenInterceptor is the registered name of the challenge token interceptor.
	ChallengeTokenInterceptor = "ChallengeTokenInterceptor"
	// CaptchaInterceptor is the registered name of the captcha interceptor.
	CaptchaInterceptor = "CaptchaInterceptor"
)

// Interceptor user input identifier constants.
const (
	// captchaTokenFieldKey is the user-input field that carries the captcha token.
	captchaTokenFieldKey = "captcha_token" //nolint:gosec // field key, not a credential
)

// Interceptor shared data key constants.
const (
	// sharedDataKeyChallengeTokenHash is the shared data key for the stored challenge token hash.
	sharedDataKeyChallengeTokenHash = "challengeTokenHash"
)
