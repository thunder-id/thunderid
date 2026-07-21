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

const (
	// TokenTypeJWT is the standard JWT type header value used for general-purpose JWTs.
	TokenTypeJWT = "JWT"

	// TokenTypeAccessToken is the JWT type header value for access tokens as defined in RFC 9068.
	TokenTypeAccessToken = "at+jwt"

	// TokenTypeIDJAG is the JWT type header value for an Identity Assertion Authorization Grant
	// (draft-ietf-oauth-identity-assertion-authz-grant).
	TokenTypeIDJAG = "oauth-id-jag+jwt" //nolint:gosec // JWT typ header value, not a credential
)
