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
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/system/jose/sdjwt"
)

// verifier composes a trust store and a policy; used in tests to exercise the
// SD-JWT VC verification pipeline directly.
type verifier struct {
	trust  *staticTrustStore
	policy policy
}

func newVerifier(trust *staticTrustStore, policy policy) (*verifier, error) {
	if trust == nil && policy.EnforceTrustedIssuer {
		return nil, fmt.Errorf("%w: trust resolver is required", ErrPolicy)
	}
	if policy.ExpectedVCT == "" {
		return nil, fmt.Errorf("%w: expected vct is required", ErrPolicy)
	}
	if policy.Audience == "" {
		return nil, fmt.Errorf("%w: audience is required", ErrPolicy)
	}
	return &verifier{trust: trust, policy: policy}, nil
}

func (v *verifier) verify(ctx context.Context, presentation, nonce string) (*VerifiedPresentation, error) {
	cred, err := verifySDJWTPresentation(
		ctx, presentation, v.trust, v.policy.Audience, nonce, v.policy.Leeway, v.policy.KeyBindingMaxAge,
		v.policy.EnforceTrustedIssuer, v.policy.EnforceKeyBinding)
	if err != nil {
		return nil, err
	}
	return finalizePresentation(cred, v.policy)
}

// verifySDJWTPresentation runs the SD-JWT verification stack: parsing, optional
// trust/signature enforcement, selective-disclosure resolution, and optional
// key-binding enforcement. Policy enforcement (VCT, claim policy) is
// finalizePresentation's job.
func verifySDJWTPresentation(
	ctx context.Context, presentation string, trust *staticTrustStore,
	expectedAudience, expectedNonce string, leeway, maxIATAge time.Duration,
	enforceTrustedIssuer, enforceKeyBinding bool,
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
		issuerKey, err := trust.resolveIssuerKey(ctx, issuer)
		if err != nil {
			return nil, err
		}
		if err := sdjwt.VerifyIssuerSignature(p, issuerKey); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidPresentation, err)
		}
	}

	cred, err := sdjwt.ResolveDisclosures(p)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidPresentation, err)
	}

	if enforceKeyBinding {
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
	return &verifiedCredential{
		Issuer:         issuer,
		VCT:            vct,
		Claims:         cred.Claims,
		DisclosedPaths: cred.DisclosedPaths,
	}, nil
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
		Subject:        subject,
		Issuer:         cred.Issuer,
		VCT:            cred.VCT,
		Claims:         flattenClaims(cred.Claims),
		DisclosedPaths: cred.DisclosedPaths,
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
	return nil
}

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
