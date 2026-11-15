// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package openid4vp

import (
	"fmt"
	"strings"
)

const (
	// FormatSDJWTVC is the OpenID4VP credential format identifier for SD-JWT VC.
	FormatSDJWTVC = "dc+sd-jwt"
)

// BuildQuery builds the DCQL query requesting the configured claims as an
// SD-JWT VC presentation for the configured credential id and vct.
func buildQuery(cfg dcqlConfig) (*dcqlQuery, error) {
	credentialID := cfg.CredentialID
	vct := cfg.VCT
	claims := cfg.Claims
	if credentialID == "" || vct == "" || len(claims) == 0 {
		return nil, fmt.Errorf("%w: credential_id, vct and at least one claim are required", ErrPolicy)
	}

	dcqlClaims := make([]dcqlClaim, 0, len(claims))
	for _, path := range claims {
		segments, err := claimPathToSegments(path)
		if err != nil {
			return nil, err
		}
		dcqlClaims = append(dcqlClaims, dcqlClaim{Path: segments, Values: dcqlValues(cfg.ClaimValues[path])})
	}

	credential := dcqlCredential{
		ID:     credentialID,
		Format: FormatSDJWTVC,
		Meta:   &dcqlMeta{VCTValues: []string{vct}},
		Claims: dcqlClaims,
	}
	if len(cfg.TrustedAuthorityKeyIDs) > 0 {
		credential.TrustedAuthorities = []trustedAuthority{
			{Type: "aki", Values: cfg.TrustedAuthorityKeyIDs},
		}
	}

	return &dcqlQuery{
		Credentials: []dcqlCredential{credential},
	}, nil
}

// dcqlValues converts the configured allowed values to the DCQL "values" array; nil when unconstrained.
func dcqlValues(values []string) []interface{} {
	if len(values) == 0 {
		return nil
	}
	out := make([]interface{}, 0, len(values))
	for _, v := range values {
		out = append(out, v)
	}
	return out
}

// claimPathToSegments converts a dotted claim path into DCQL path segments.
func claimPathToSegments(path string) ([]interface{}, error) {
	if path == "" {
		return nil, fmt.Errorf("%w: empty claim path", ErrPolicy)
	}
	parts := strings.Split(path, ".")
	segments := make([]interface{}, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			return nil, fmt.Errorf("%w: malformed claim path %q", ErrPolicy, path)
		}
		segments = append(segments, part)
	}
	return segments, nil
}
