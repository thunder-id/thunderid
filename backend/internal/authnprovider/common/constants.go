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

package common

// Authenticator name constants.
const (
	AuthenticatorCredentials = "CredentialsAuthenticator"
	AuthenticatorSMSOTP      = "SMSOTPAuthenticator"
	AuthenticatorGoogle      = "GoogleOIDCAuthenticator"
	AuthenticatorGithub      = "GithubOAuthAuthenticator"
	AuthenticatorOAuth       = "OAuthAuthenticator"
	AuthenticatorOIDC        = "OIDCAuthenticator"
	AuthenticatorPasskey     = "Passkey"
)

// Authn Data type constants.
const (
	AuthnDataTypeCredentials = "Credentials"
	AuthnDataTypeOTP         = "OTP"
	AuthnDataTypeFederated   = "Federated"
	AuthnDataTypePasskey     = "Passkey"
)

const (
	// FactorKnowledge represents "something you know" (e.g., password, PIN).
	FactorKnowledge AuthenticationFactor = "KNOWLEDGE"
	// FactorPossession represents "something you have" (e.g., OTP device, SMS).
	FactorPossession AuthenticationFactor = "POSSESSION"
	// FactorInherence represents "something you are" (e.g., biometrics).
	FactorInherence AuthenticationFactor = "INHERENCE"
)
