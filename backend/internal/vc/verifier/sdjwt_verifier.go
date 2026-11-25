// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package verifier

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/thunder-id/thunderid/internal/system/jose/jws"
	"github.com/thunder-id/thunderid/internal/system/jose/sdjwt"
	"github.com/thunder-id/thunderid/internal/vc/truststore"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// FormatSDJWTVC is the credential format identifier for SD-JWT VC.
const FormatSDJWTVC = "dc+sd-jwt"

var (
	// ErrInvalidCredential is returned when a credential fails parsing or cryptographic verification.
	ErrInvalidCredential = errors.New("vc verifier: invalid credential")
	// ErrUnexpectedCredentialType is returned when the credential type does not match the expected type.
	ErrUnexpectedCredentialType = errors.New("vc verifier: unexpected credential type")
)

// sdjwtVCVerifier verifies SD-JWT VC presentations whose issuer is identified by an x5c chain.
type sdjwtVCVerifier struct {
	trust truststore.TrustStoreInterface
}

var _ providers.VCVerifierProvider = (*sdjwtVCVerifier)(nil)

// newSDJWTVCVerifier creates an SD-JWT VC verifier that validates issuer chains against trust.
func newSDJWTVCVerifier(trust truststore.TrustStoreInterface) *sdjwtVCVerifier {
	return &sdjwtVCVerifier{trust: trust}
}

// Verify parses and verifies an SD-JWT VC presentation: issuer trust and signature,
// disclosures, key binding against params.Nonce, and that the credential's vct is one of the
// credential query's vct_values when set.
func (v *sdjwtVCVerifier) Verify(
	_ context.Context, credential string, params providers.VCVerifierParams,
) (providers.VerifiedCredential, error) {
	p, err := sdjwt.Parse(credential)
	if err != nil {
		return providers.VerifiedCredential{}, fmt.Errorf("%w: %w", ErrInvalidCredential, err)
	}
	// sdjwt.Parse has already decoded the issuer payload, so IssuerClaims cannot fail here.
	issuerClaims, _ := p.IssuerClaims()
	issuer, _ := issuerClaims["iss"].(string)
	if issuer == "" {
		return providers.VerifiedCredential{}, fmt.Errorf("%w: credential missing iss", ErrInvalidCredential)
	}

	chain, err := x5cChain(p.IssuerJWT)
	if err != nil {
		return providers.VerifiedCredential{}, err
	}
	issuerLeaf := chain[0]
	if params.EnforceTrustedIssuer {
		issuerLeaf, err = v.trust.VerifyChain(chain, time.Now(), params.TrustedAuthorities)
		if err != nil {
			return providers.VerifiedCredential{}, err
		}
	}
	if err := sdjwt.VerifyIssuerSignature(p, issuerLeaf.PublicKey); err != nil {
		return providers.VerifiedCredential{}, fmt.Errorf("%w: %w", ErrInvalidCredential, err)
	}

	cred, err := sdjwt.ResolveDisclosures(p)
	if err != nil {
		return providers.VerifiedCredential{}, fmt.Errorf("%w: %w", ErrInvalidCredential, err)
	}

	if params.EnforceKeyBinding || p.HasKeyBinding() {
		if err := sdjwt.VerifyKeyBinding(p, cred, sdjwt.VerifyOptions{
			ExpectedAudience: params.Audience,
			ExpectedNonce:    params.Nonce,
			Leeway:           params.Leeway,
			MaxIATAge:        params.KeyBindingMaxAge,
		}); err != nil {
			return providers.VerifiedCredential{}, fmt.Errorf("%w: %w", ErrInvalidCredential, err)
		}
	}

	vct, _ := cred.Claims["vct"].(string)
	if meta := params.CredentialQuery.Meta; meta != nil && len(meta.VCTValues) > 0 &&
		!slices.Contains(meta.VCTValues, vct) {
		return providers.VerifiedCredential{}, fmt.Errorf("%w: got %q, want one of %q",
			ErrUnexpectedCredentialType, vct, meta.VCTValues)
	}

	var keyBindingThumbprint string
	if cred.ConfirmationKey != nil {
		if jkt, jktErr := jws.ComputeJKT(cred.ConfirmationKey); jktErr == nil {
			keyBindingThumbprint = jkt
		}
	}
	return providers.VerifiedCredential{
		Format:               FormatSDJWTVC,
		Issuer:               issuer,
		Type:                 []string{vct},
		Claims:               cred.Claims,
		DisclosedPaths:       cred.DisclosedPaths,
		KeyBindingThumbprint: keyBindingThumbprint,
	}, nil
}

// x5cChain extracts the leaf-first DER certificate chain from the issuer JWT's x5c header (RFC 7515).
func x5cChain(issuerJWT string) ([]*x509.Certificate, error) {
	header, err := jws.DecodeHeader(issuerJWT)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidCredential, err)
	}
	raw, ok := header["x5c"].([]interface{})
	if !ok || len(raw) == 0 {
		return nil, fmt.Errorf("%w: issuer JWT missing x5c header", ErrInvalidCredential)
	}
	chain := make([]*x509.Certificate, 0, len(raw))
	for _, entry := range raw {
		encoded, ok := entry.(string)
		if !ok {
			return nil, fmt.Errorf("%w: malformed x5c entry", ErrInvalidCredential)
		}
		der, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidCredential, err)
		}
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidCredential, err)
		}
		chain = append(chain, cert)
	}
	return chain, nil
}
