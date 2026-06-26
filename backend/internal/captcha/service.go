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

// Package captcha defines the contract for verifying captcha tokens against a provider.
package captcha

import (
	"context"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// CaptchaServiceInterface defines the contract for verifying captcha tokens.
type CaptchaServiceInterface interface {
	// Verify validates the given captcha token and returns the verification result. An invalid
	// token is reported through the result's negative verdict, while operational failures (provider
	// unavailable or misconfigured) are returned as a server-side service error.
	Verify(ctx context.Context, token string) (*VerificationResult, *tidcommon.ServiceError)
}
