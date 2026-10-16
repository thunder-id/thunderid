// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
	// LocaleInterceptor is the registered name of the locale interceptor.
	LocaleInterceptor = "LocaleInterceptor"
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
