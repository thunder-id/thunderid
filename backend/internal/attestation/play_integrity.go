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

// Package attestation verifies the binary identity of a mobile client through platform-native
// attestation mechanisms (currently Google Play Integrity for Android).
package attestation

import (
	"context"
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	kmprovider "github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	playintegrity "google.golang.org/api/playintegrity/v1"
)

const (
	// appRecognitionVerdictPlayRecognized is the Play Integrity verdict indicating the app is a
	// genuine binary distributed by Google Play.
	appRecognitionVerdictPlayRecognized = "PLAY_RECOGNIZED"

	// verifyTimeout bounds the outbound verification call (which reaches Google's Play Integrity
	// API) so a slow or unresponsive provider cannot stall flow initiation. Cancellation still
	// propagates from the parent context.
	verifyTimeout = 10 * time.Second
)

// playIntegrityVerifier verifies Google Play Integrity tokens for Android clients.
type playIntegrityVerifier struct {
	decoder   integrityTokenDecoder
	cryptoSvc kmprovider.RuntimeCryptoProvider
	logger    *log.Logger
}

// newPlayIntegrityVerifier creates a platform attestation verifier backed by the given token
// decoder.
func newPlayIntegrityVerifier(decoder integrityTokenDecoder,
	cryptoSvc kmprovider.RuntimeCryptoProvider) providers.AttestationProvider {
	return &playIntegrityVerifier{
		decoder:   decoder,
		cryptoSvc: cryptoSvc,
		logger:    log.GetLogger().With(log.String(log.LoggerKeyComponentName, "PlayIntegrityVerifier")),
	}
}

// Verify decrypts the stored service account credentials, decodes the Play Integrity token against
// Google's API within a bounded deadline, and checks that the attested app matches the application's
// registered package name and signing identity. It returns true only when every check passes; a
// definitive rejection (invalid payload, identity mismatch, or unrecognized app) is reported as
// (false, nil), while an operational failure (decrypt, decode, or configuration problem) is reported
// as (false, ServiceError).
func (v *playIntegrityVerifier) Verify(ctx context.Context, cfg *providers.AttestationConfig, token string) (
	bool, *tidcommon.ServiceError) {
	if cfg == nil || cfg.Android == nil {
		v.logger.Error(ctx, "Attestation requested without an attestation configuration")
		return false, &tidcommon.InternalServerError
	}

	// A configured application must register a package name, at least one signing certificate
	// digest, and service account credentials; without all three the binary identity cannot be
	// verified. Reject an incomplete configuration up front, before making an outbound call to
	// Google's Play Integrity API.
	android := cfg.Android
	if android.PackageName == "" || len(android.CertificateSha256Digests) == 0 ||
		android.ServiceAccountCredentials == "" {
		v.logger.Error(ctx, "Attestation configuration is incomplete")
		return false, &tidcommon.InternalServerError
	}

	decrypted, err := v.decryptConfig(ctx, android)
	if err != nil {
		v.logger.Error(ctx, "Failed to decrypt attestation credentials", log.Error(err))
		return false, &tidcommon.InternalServerError
	}

	verifyCtx, cancel := context.WithTimeout(ctx, verifyTimeout)
	defer cancel()

	payload, err := v.decoder.Decode(verifyCtx, decrypted.ServiceAccountCredentials, decrypted.PackageName, token)
	if err != nil {
		v.logger.Error(ctx, "Failed to decode play integrity token", log.Error(err))
		return false, &tidcommon.InternalServerError
	}

	if err := verifyAndroidPayload(payload, decrypted); err != nil {
		v.logger.Debug(ctx, "Attestation token rejected", log.Error(err))
		return false, nil
	}

	return true, nil
}

// decryptConfig returns a copy of the Android attestation config with the stored service account
// credentials decrypted, without mutating the shared stored profile.
func (v *playIntegrityVerifier) decryptConfig(ctx context.Context, cfg *providers.AndroidAttestationConfig) (
	*providers.AndroidAttestationConfig, error) {
	android := *cfg
	if android.ServiceAccountCredentials == "" {
		return &android, nil
	}

	params := cryptolib.AlgorithmParams{Algorithm: cryptolib.AlgorithmAESGCM}
	plaintext, err := v.cryptoSvc.Decrypt(ctx, nil, params, []byte(android.ServiceAccountCredentials))
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt attestation credentials: %w", err)
	}
	android.ServiceAccountCredentials = string(plaintext)
	return &android, nil
}

// verifyAndroidPayload checks the decoded payload against the registered Android attestation config.
func verifyAndroidPayload(payload *playintegrity.TokenPayloadExternal,
	android *providers.AndroidAttestationConfig) error {
	if payload == nil || payload.AppIntegrity == nil {
		return errInvalidPayload
	}

	appIntegrity := payload.AppIntegrity
	if appIntegrity.PackageName != android.PackageName {
		return fmt.Errorf("%w (attested=%q, registered=%q)",
			errPackageNameMismatch, appIntegrity.PackageName, android.PackageName)
	}
	if !hasCommonDigest(appIntegrity.CertificateSha256Digest, android.CertificateSha256Digests) {
		return fmt.Errorf("%w (attested=%v, registered=%v)",
			errSigningIdentityMismatch, appIntegrity.CertificateSha256Digest, android.CertificateSha256Digests)
	}
	if appIntegrity.AppRecognitionVerdict != appRecognitionVerdictPlayRecognized {
		return errAppNotPlayRecognized
	}
	return nil
}

// hasCommonDigest reports whether the attested and registered digest sets share at least one value.
func hasCommonDigest(attested, registered []string) bool {
	registeredSet := make(map[string]struct{}, len(registered))
	for _, d := range registered {
		registeredSet[d] = struct{}{}
	}
	for _, d := range attested {
		if _, ok := registeredSet[d]; ok {
			return true
		}
	}
	return false
}
