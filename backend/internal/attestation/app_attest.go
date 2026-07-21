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
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"fmt"

	"github.com/fxamacker/cbor/v2"

	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// appAttestVerifier verifies Apple App Attest attestation objects for iOS clients, entirely
// offline: it validates the attestation certificate chain against Apple's App Attest root and
// matches the attested app identifier and key.
//
// It does not yet bind verification to a server-issued challenge — the same scope limitation as the
// Play Integrity verifier's lack of request-freshness checking. Closing that gap is tracked as a
// follow-up.
//
// Because verification never leaves the process, any problem with the token is a definitive
// rejection; only a missing or incomplete configuration is an operational error.
type appAttestVerifier struct {
	rootPool *x509.CertPool
	logger   *log.Logger
}

// newAppAttestVerifier creates a verifier trusted against the configured Apple App Attestation Root
// CA. It errors if the root certificate is missing or fails to parse, so the caller can fail server
// startup instead of running with a non-functional verifier.
func newAppAttestVerifier(rootPEM string) (providers.AttestationProvider, error) {
	if rootPEM == "" {
		return nil, fmt.Errorf("attestation: Apple App Attestation Root CA certificate is not configured")
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM([]byte(rootPEM)) {
		return nil, fmt.Errorf("attestation: failed to parse configured Apple App Attestation Root CA")
	}
	return &appAttestVerifier{
		rootPool: pool,
		logger:   log.GetLogger().With(log.String(log.LoggerKeyComponentName, "AppAttestVerifier")),
	}, nil
}

// Verify decodes the attestation object and checks the attested app identifier and key against the
// registered Team ID and Bundle ID. A definitive rejection (malformed object, bad chain, identity
// mismatch) is (false, nil); a missing or incomplete configuration is (false, ServiceError).
func (v *appAttestVerifier) Verify(ctx context.Context, cfg *providers.AttestationConfig, token string) (
	bool, *tidcommon.ServiceError) {
	if cfg == nil || cfg.Apple == nil {
		v.logger.Error(ctx, "Attestation requested without an Apple attestation configuration")
		return false, &tidcommon.InternalServerError
	}

	// Both Team ID and Bundle ID are required to verify identity; reject an incomplete config up front.
	apple := cfg.Apple
	if apple.TeamID == "" || apple.BundleID == "" {
		v.logger.Error(ctx, "Apple attestation configuration is incomplete")
		return false, &tidcommon.InternalServerError
	}

	if err := v.verifyAttestation(apple, token); err != nil {
		v.logger.Debug(ctx, "Attestation token rejected", log.Error(err))
		return false, nil
	}
	return true, nil
}

// verifyAttestation decodes and validates the attestation object against the registered Apple config,
// returning a non-nil error describing the first check that fails.
func (v *appAttestVerifier) verifyAttestation(apple *providers.AppleAttestationConfig, token string) error {
	raw, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		// Attestation objects may also be presented URL-safe; try that before failing.
		raw, err = base64.URLEncoding.DecodeString(token)
		if err != nil {
			return fmt.Errorf("%w: %w", errInvalidPayload, err)
		}
	}

	var obj attestationObject
	if err := cbor.Unmarshal(raw, &obj); err != nil {
		return fmt.Errorf("%w: %w", errInvalidPayload, err)
	}
	if obj.Fmt != appAttestFormat || len(obj.AttStmt.X5C) == 0 {
		return errInvalidPayload
	}

	leaf, err := v.verifyCertificateChain(obj.AttStmt.X5C)
	if err != nil {
		return err
	}

	authData, err := v.parseAuthData(obj.AuthData)
	if err != nil {
		return err
	}

	if err := v.verifyAppIdentifier(authData.rpIDHash, apple); err != nil {
		return err
	}
	if !v.isRecognizedAAGUID(authData.aaguid) {
		return errEnvironmentUnrecognized
	}
	if authData.signCount != 0 {
		return errSignCountNonZero
	}
	return v.verifyKeyIdentifier(leaf, authData.credentialID)
}

// verifyCertificateChain verifies the credCert (x5c[0]) chains to the trusted root, through any
// intermediates presented in x5c[1:]. It returns the parsed leaf certificate on success.
func (v *appAttestVerifier) verifyCertificateChain(x5c [][]byte) (*x509.Certificate, error) {
	leaf, err := x509.ParseCertificate(x5c[0])
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errInvalidPayload, err)
	}

	intermediates := x509.NewCertPool()
	for _, der := range x5c[1:] {
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", errInvalidPayload, err)
		}
		intermediates.AddCert(cert)
	}

	// App Attest certs aren't TLS server certs, so relax the default ExtKeyUsageServerAuth requirement.
	opts := x509.VerifyOptions{
		Roots:         v.rootPool,
		Intermediates: intermediates,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	}
	if _, err := leaf.Verify(opts); err != nil {
		return nil, fmt.Errorf("%w: %w", errCertificateChainInvalid, err)
	}
	return leaf, nil
}

// parseAuthData parses the fixed-layout authenticator data fields, with explicit bounds checks so
// a short or malformed value returns an error instead of panicking.
func (v *appAttestVerifier) parseAuthData(data []byte) (*parsedAuthData, error) {
	if len(data) < authDataMinLen {
		return nil, errInvalidPayload
	}

	rpIDHash := data[0:authDataRPIDHashLen]
	flags := data[authDataRPIDHashLen]
	if flags&authDataFlagAttestedCD == 0 {
		return nil, errInvalidPayload
	}

	signCountOffset := authDataRPIDHashLen + authDataFlagsLen
	signCount := binary.BigEndian.Uint32(data[signCountOffset : signCountOffset+authDataSignCountLen])

	aaguidOffset := signCountOffset + authDataSignCountLen
	var aaguid [16]byte
	copy(aaguid[:], data[aaguidOffset:aaguidOffset+authDataAAGUIDLen])

	credIDLenOffset := aaguidOffset + authDataAAGUIDLen
	credIDLen := int(binary.BigEndian.Uint16(data[credIDLenOffset : credIDLenOffset+authDataCredIDLenLen]))

	credIDOffset := credIDLenOffset + authDataCredIDLenLen
	if len(data) < credIDOffset+credIDLen {
		return nil, errInvalidPayload
	}
	credentialID := data[credIDOffset : credIDOffset+credIDLen]

	return &parsedAuthData{
		rpIDHash:     rpIDHash,
		signCount:    signCount,
		aaguid:       aaguid,
		credentialID: credentialID,
	}, nil
}

// verifyAppIdentifier checks that the authenticator data's RP ID hash matches the SHA-256 hash of
// the registered Team ID and Bundle ID, as Apple's App Attest spec defines the App ID.
func (v *appAttestVerifier) verifyAppIdentifier(rpIDHash []byte, apple *providers.AppleAttestationConfig) error {
	expected := sha256.Sum256([]byte(apple.TeamID + "." + apple.BundleID))
	if string(rpIDHash) != string(expected[:]) {
		return errAppIdentifierMismatch
	}
	return nil
}

// isRecognizedAAGUID reports whether aaguid identifies a genuine App Attest key, in either the
// production or development environment.
func (v *appAttestVerifier) isRecognizedAAGUID(aaguid [16]byte) bool {
	return aaguid == aaguidProduction || aaguid == aaguidDevelopment
}

// verifyKeyIdentifier checks that the authenticator data's credential ID equals the SHA-256 hash of
// the credCert's public key, encoded as an ANSI X9.63 uncompressed point (0x04 || X || Y), not the
// DER SubjectPublicKeyInfo.
func (v *appAttestVerifier) verifyKeyIdentifier(leaf *x509.Certificate, credentialID []byte) error {
	pub, ok := leaf.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return errInvalidPayload
	}
	ecdhPub, err := pub.ECDH()
	if err != nil {
		return fmt.Errorf("%w: %w", errInvalidPayload, err)
	}
	expected := sha256.Sum256(ecdhPub.Bytes())
	if string(credentialID) != string(expected[:]) {
		return errKeyIdentifierMismatch
	}
	return nil
}
