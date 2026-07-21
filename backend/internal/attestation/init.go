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

import (
	"github.com/thunder-id/thunderid/internal/system/config"
	kmprovider "github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize creates the platform attestation provider, dispatching to Google Play Integrity for
// Android clients and Apple App Attest for iOS clients based on the application's configuration. It
// returns an error if a platform verifier cannot be constructed, so the caller can fail server
// startup rather than run with a non-functional verifier.
func Initialize(cryptoSvc kmprovider.RuntimeCryptoProvider) (providers.AttestationProvider, error) {
	appleRootPEM := config.GetServerRuntime().Config.Attestation.Apple.RootCertificate
	appAttestVsvc, err := newAppAttestVerifier(appleRootPEM)
	if err != nil {
		return nil, err
	}
	return newCompositeVerifier(
		newPlayIntegrityVerifier(newGooglePlayIntegrityDecoder(), cryptoSvc),
		appAttestVsvc,
	), nil
}
