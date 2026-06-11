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
	"encoding/json"
	"fmt"
)

// ParseAuthorizationResponse parses the decrypted OpenID4VP response body.
func parseAuthorizationResponse(body []byte) (*authorizationResponse, error) {
	var raw struct {
		State   string          `json:"state"`
		VPToken json.RawMessage `json:"vp_token"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidResponse, err)
	}
	if len(raw.VPToken) == 0 {
		return nil, fmt.Errorf("%w: missing vp_token", ErrInvalidResponse)
	}

	var byID map[string]json.RawMessage
	if err := json.Unmarshal(raw.VPToken, &byID); err != nil {
		return nil, fmt.Errorf("%w: vp_token is not a DCQL object: %w", ErrInvalidResponse, err)
	}

	presentations := make(map[string][]string, len(byID))
	for id, val := range byID {
		list, err := decodePresentationValue(val)
		if err != nil {
			return nil, fmt.Errorf("%w: credential %q: %w", ErrInvalidResponse, id, err)
		}
		presentations[id] = list
	}

	return &authorizationResponse{State: raw.State, Presentations: presentations}, nil
}

// Presentation returns the single presentation for credentialID, erroring when
// it is absent or ambiguous.
func (r *authorizationResponse) presentation(credentialID string) (string, error) {
	list, ok := r.Presentations[credentialID]
	if !ok || len(list) == 0 {
		return "", fmt.Errorf("%w: no presentation for credential %q", ErrInvalidResponse, credentialID)
	}
	if len(list) > 1 {
		return "", fmt.Errorf("%w: multiple presentations for credential %q", ErrInvalidResponse, credentialID)
	}
	return list[0], nil
}

// VerifyResponse parses a decrypted OpenID4VP response body, extracts the
// presentation for credentialID, and verifies it against policy and nonce.
// The caller is responsible for decrypting the JWE (jwe.DecryptWithKey) and
// for correlating authorizationResponse.State with the issued request
// beforehand.
func (v *verifier) verifyResponse(
	ctx context.Context, body []byte, credentialID, nonce string,
) (*VerifiedPresentation, error) {
	resp, err := parseAuthorizationResponse(body)
	if err != nil {
		return nil, err
	}
	presentation, err := resp.presentation(credentialID)
	if err != nil {
		return nil, err
	}
	return v.verify(ctx, presentation, nonce)
}

// decodePresentationValue accepts either a single presentation string or an
// array of them.
func decodePresentationValue(raw json.RawMessage) ([]string, error) {
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		return []string{single}, nil
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err == nil {
		return list, nil
	}
	return nil, fmt.Errorf("presentation must be a string or array of strings")
}
