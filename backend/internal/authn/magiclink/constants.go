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

package magiclink

const (
	// DefaultExpirySeconds is the default expiry time for magic link tokens in seconds.
	DefaultExpirySeconds = 300

	// tokenAudience is the audience claim for magic link tokens.
	tokenAudience = "magiclink-svc"

	// ClaimMagicLinkUsedJti is the authenticated claim key for the used magic link JTI.
	ClaimMagicLinkUsedJti = "magicLinkUsedJti"

	// ClaimNonce is the standard claim key for the magic link nonce.
	ClaimNonce = "nonce"

	// CredentialKeyNonce is the credential map key for the magic link nonce.
	CredentialKeyNonce = "nonce"

	// CredentialKeyUsedJti is the credential map key for the magic link used JTI.
	CredentialKeyUsedJti = "usedJti"

	// CredentialKeyToken is the credential map key for the magic link token.
	CredentialKeyToken = "token"

	// CredentialKeySubjectAttribute is the credential map key for the magic link subject attribute.
	CredentialKeySubjectAttribute = "subjectAttribute"
)
