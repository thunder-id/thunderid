/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package introspect

// IntrospectRequest represents the request to the token introspection endpoint
type IntrospectRequest struct {
	Token         string `json:"token" form:"token"`
	TokenTypeHint string `json:"token_type_hint,omitempty" form:"token_type_hint,omitempty"`
}

// IntrospectResponse represents the response from the token introspection endpoint
type IntrospectResponse struct {
	Active    bool      `json:"active"`
	Scope     string    `json:"scope,omitempty"`
	ClientID  string    `json:"client_id,omitempty"`
	Username  string    `json:"username,omitempty"`
	TokenType string    `json:"token_type,omitempty"`
	Exp       int64     `json:"exp,omitempty"`
	Iat       int64     `json:"iat,omitempty"`
	Nbf       int64     `json:"nbf,omitempty"`
	Sub       string    `json:"sub,omitempty"`
	Aud       any       `json:"aud,omitempty"`
	Iss       string    `json:"iss,omitempty"`
	Jti       string    `json:"jti,omitempty"`
	Cnf       *CnfClaim `json:"cnf,omitempty"`
}

// CnfClaim represents the confirmation claim. For DPoP-bound tokens this carries
// the JWK SHA-256 thumbprint.
type CnfClaim struct {
	Jkt string `json:"jkt,omitempty"`
}
