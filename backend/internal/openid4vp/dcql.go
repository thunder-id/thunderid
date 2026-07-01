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
	"fmt"
	"strings"
)

const (
	// FormatSDJWTVC is the OpenID4VP credential format identifier for SD-JWT VC.
	FormatSDJWTVC = "dc+sd-jwt"
)

// BuildQuery builds the DCQL query requesting the configured claims as an
// SD-JWT VC presentation for the configured credential id and vct. An mdoc
// option can be added later without changing callers.
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
