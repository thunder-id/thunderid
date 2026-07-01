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

package openid4vp

import (
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/system/jose/jws"
	"github.com/thunder-id/thunderid/internal/system/jose/sdjwt"
)

// verifySDJWTPresentation runs the SD-JWT verification stack: parsing, optional
// trust/signature enforcement, selective-disclosure resolution, and optional
// key-binding enforcement. Policy enforcement (VCT, claim policy) is
// finalizePresentation's job.
func verifySDJWTPresentation(
	presentation string, trust *trustAnchorStore,
	expectedAudience, expectedNonce string, leeway, maxIATAge time.Duration,
	enforceTrustedIssuer, enforceKeyBinding bool, allowedAnchors []string,
) (*verifiedCredential, error) {
	p, err := sdjwt.Parse(presentation)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidPresentation, err)
	}

	issuerClaims, err := p.IssuerClaims()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidPresentation, err)
	}
	issuer, _ := issuerClaims["iss"].(string)
	if issuer == "" {
		return nil, fmt.Errorf("%w: credential missing iss", ErrInvalidPresentation)
	}

	if enforceTrustedIssuer {
		if trust == nil {
			return nil, fmt.Errorf("%w: no trust anchors configured", ErrUntrustedIssuer)
		}
		chain, err := x5cChain(p.IssuerJWT)
		if err != nil {
			return nil, err
		}
		leaf, err := trust.verifyChain(chain, time.Now(), allowedAnchors)
		if err != nil {
			return nil, err
		}
		if err := sdjwt.VerifyIssuerSignature(p, leaf.PublicKey); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidPresentation, err)
		}
	}

	cred, err := sdjwt.ResolveDisclosures(p)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidPresentation, err)
	}

	if enforceKeyBinding || p.HasKeyBinding() {
		if err := sdjwt.VerifyKeyBinding(p, cred, sdjwt.VerifyOptions{
			ExpectedAudience: expectedAudience,
			ExpectedNonce:    expectedNonce,
			Leeway:           leeway,
			MaxIATAge:        maxIATAge,
		}); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidPresentation, err)
		}
	}

	vct, _ := cred.Claims["vct"].(string)
	// Derive the holder key-binding thumbprint, used as the fallback subject
	// identifier when the credential carries no "sub". Best-effort: a malformed
	// or absent cnf.jwk simply yields no thumbprint.
	var keyBindingThumbprint string
	if cred.ConfirmationKey != nil {
		if jkt, jktErr := jws.ComputeJKT(cred.ConfirmationKey); jktErr == nil {
			keyBindingThumbprint = jkt
		}
	}
	return &verifiedCredential{
		Issuer:               issuer,
		VCT:                  vct,
		Claims:               cred.Claims,
		DisclosedPaths:       cred.DisclosedPaths,
		KeyBindingThumbprint: keyBindingThumbprint,
	}, nil
}

// x5cChain extracts the leaf-first X.509 certificate chain from the x5c header
// of the issuer JWS (RFC 7515: base64-STANDARD-encoded DER, leaf first).
func x5cChain(issuerJWT string) ([]*x509.Certificate, error) {
	header, err := jws.DecodeHeader(issuerJWT)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidPresentation, err)
	}
	raw, ok := header["x5c"].([]interface{})
	if !ok || len(raw) == 0 {
		return nil, fmt.Errorf("%w: issuer JWT missing x5c header", ErrInvalidPresentation)
	}
	chain := make([]*x509.Certificate, 0, len(raw))
	for _, entry := range raw {
		encoded, ok := entry.(string)
		if !ok {
			return nil, fmt.Errorf("%w: malformed x5c entry", ErrInvalidPresentation)
		}
		der, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidPresentation, err)
		}
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidPresentation, err)
		}
		chain = append(chain, cert)
	}
	return chain, nil
}

// finalizePresentation enforces VCT and selective-disclosure policy on a raw credential.
func finalizePresentation(cred *verifiedCredential, policy policy) (*VerifiedPresentation, error) {
	if cred.VCT != policy.ExpectedVCT {
		return nil, fmt.Errorf("%w: got %q, want %q", ErrUnexpectedVCT, cred.VCT, policy.ExpectedVCT)
	}
	if err := enforceClaimPolicy(cred.DisclosedPaths, cred.Claims, policy); err != nil {
		return nil, err
	}

	subject, _ := cred.Claims["sub"].(string)
	return &VerifiedPresentation{
		Subject:              subject,
		Issuer:               cred.Issuer,
		VCT:                  cred.VCT,
		Claims:               flattenClaims(cred.Claims),
		DisclosedPaths:       cred.DisclosedPaths,
		KeyBindingThumbprint: cred.KeyBindingThumbprint,
	}, nil
}

// enforceClaimPolicy applies data minimisation and mandatory-claim checks.
func enforceClaimPolicy(disclosed []string, claims map[string]interface{}, policy policy) error {
	if len(policy.RequestedClaims) > 0 {
		requested := make(map[string]bool, len(policy.RequestedClaims))
		for _, c := range policy.RequestedClaims {
			requested[c] = true
		}
		for _, path := range disclosed {
			if !requested[path] {
				return fmt.Errorf("%w: %s", ErrUnrequestedClaim, path)
			}
		}
	}

	for _, mandatory := range policy.MandatoryClaims {
		if _, ok := lookupClaim(claims, mandatory); !ok {
			return fmt.Errorf("%w: %s", ErrMissingMandatoryClaim, mandatory)
		}
	}

	// Value constraints: when a constrained claim is disclosed, its value must be
	// one of the allowed values. Absent (optional, undisclosed) claims are not
	// checked here — mandatory presence is enforced above.
	for path, allowed := range policy.ClaimValues {
		value, ok := lookupClaim(claims, path)
		if !ok {
			continue
		}
		if !valueAllowed(value, allowed) {
			return fmt.Errorf("%w: %s", ErrClaimValueNotAllowed, path)
		}
	}
	return nil
}

// valueAllowed reports whether the disclosed claim value matches one of the
// allowed values. Values are compared as strings so string, boolean and numeric
// claims are handled uniformly.
func valueAllowed(value interface{}, allowed []string) bool {
	return slices.Contains(allowed, fmt.Sprint(value))
}

// lookupClaim resolves a dotted-path claim value from the nested claims map.
func lookupClaim(claims map[string]interface{}, path string) (interface{}, bool) {
	segments := strings.Split(path, ".")
	var current interface{} = claims
	for _, seg := range segments {
		obj, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current, ok = obj[seg]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

// flattenClaims returns dotted-path-keyed attributes, omitting SD-JWT metadata claims.
func flattenClaims(claims map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{})
	flattenInto(out, "", claims)
	for _, meta := range []string{"iss", "vct", "cnf", "iat", "exp", "nbf", "status", "_sd_alg"} {
		delete(out, meta)
	}
	return out
}

// flattenInto recursively flattens node into out using dotted-path keys prefixed by prefix.
func flattenInto(out map[string]interface{}, prefix string, node map[string]interface{}) {
	for k, val := range node {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		if nested, ok := val.(map[string]interface{}); ok {
			flattenInto(out, key, nested)
			continue
		}
		out[key] = val
	}
}

// keyBindingSubjectPrefix namespaces subjects derived from the holder key-binding
// thumbprint so they are self-describing and never collide with issuer-provided
// "sub" values or disclosed claim values.
const keyBindingSubjectPrefix = "urn:ietf:params:oauth:jwk-thumbprint:sha-256:"

// defaultSubjectDeriver returns a subjectDeriver that resolves the subject in
// priority order: the credential's own "sub" claim, then the holder key-binding
// thumbprint as an automatic fallback. The thumbprint guarantees a stable,
// unique identifier even when the credential carries no "sub", so a returning
// holder resolves to the same subject across presentations.
func defaultSubjectDeriver() subjectDeriver {
	return func(vp *VerifiedPresentation) string {
		if vp == nil {
			return ""
		}
		if vp.Subject != "" {
			return vp.Subject
		}
		if vp.KeyBindingThumbprint != "" {
			return keyBindingSubjectPrefix + vp.KeyBindingThumbprint
		}
		return ""
	}
}
