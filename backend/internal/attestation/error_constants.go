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

package attestation

import "errors"

var (
	// errInvalidPayload is returned when the decoded integrity token does not carry the app
	// integrity details required for verification.
	errInvalidPayload = errors.New("integrity token payload is missing app integrity details")
	// errPackageNameMismatch is returned when the attested package name does not match the
	// registered package name.
	errPackageNameMismatch = errors.New("attested package name does not match the registered package name")
	// errSigningIdentityMismatch is returned when none of the attested signing certificate digests
	// match the registered signing identity.
	errSigningIdentityMismatch = errors.New("attested signing certificate does not match registered identity")
	// errAppNotPlayRecognized is returned when Play Integrity does not recognize the app as a
	// genuine, Play-distributed binary.
	errAppNotPlayRecognized = errors.New("app is not recognized by Google Play")

	// errCertificateChainInvalid is returned when an App Attest attestation certificate chain does
	// not validate against Apple's App Attest root certificate authority.
	errCertificateChainInvalid = errors.New("attestation certificate chain is invalid")
	// errAppIdentifierMismatch is returned when the attested App ID does not match the registered
	// Team ID and Bundle ID.
	errAppIdentifierMismatch = errors.New("attested app identifier does not match the registered identity")
	// errKeyIdentifierMismatch is returned when the attested credential ID does not match the hash of
	// the attestation certificate's public key.
	errKeyIdentifierMismatch = errors.New("attested credential identifier does not match the certificate key")
	// errEnvironmentUnrecognized is returned when the App Attest AAGUID identifies neither the
	// production nor the development environment.
	errEnvironmentUnrecognized = errors.New("attestation environment is not recognized")
	// errSignCountNonZero is returned when a fresh App Attest key reports a non-zero signature count.
	errSignCountNonZero = errors.New("attestation signature count is not zero")
)
