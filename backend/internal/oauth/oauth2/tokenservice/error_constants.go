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

package tokenservice

import "errors"

// Reasons a token failed validation, discriminated by callers with errors.Is to pick a specific error_description.
var (
	// ErrTokenExpired indicates the token's exp claim is in the past.
	ErrTokenExpired = errors.New("token has expired")

	// ErrIssuerNotTrusted indicates the token's iss claim resolves to neither this server nor an
	// identity provider registered as a trusted issuer for the grant being used.
	ErrIssuerNotTrusted = errors.New("token issuer is not trusted")

	// ErrAudienceNotAccepted indicates the token's aud claim names none of the audiences the grant
	// accepts, such as this server's issuer or an identity provider's trusted token audience.
	ErrAudienceNotAccepted = errors.New("token audience is not accepted")
)
