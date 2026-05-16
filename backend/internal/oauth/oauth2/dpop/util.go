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

package dpop

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
)

// ExtractCnfJkt returns the DPoP key thumbprint from the cnf.jkt confirmation claim
// on a token claims map. Returns "" with no error when the
// cnf claim is absent or contains no jkt member. Returns an error when cnf is present
// but not an object, or when jkt is present but not a non-empty string.
func ExtractCnfJkt(claims map[string]any) (string, error) {
	cnfRaw, exists := claims["cnf"]
	if !exists {
		return "", nil
	}
	cnf, ok := cnfRaw.(map[string]any)
	if !ok {
		return "", fmt.Errorf("invalid 'cnf' claim: must be an object")
	}
	jktRaw, hasJKT := cnf["jkt"]
	if !hasJKT {
		return "", nil
	}
	jkt, ok := jktRaw.(string)
	if !ok || jkt == "" {
		return "", fmt.Errorf("invalid 'cnf.jkt' claim")
	}
	return jkt, nil
}

// SetCnfJkt sets the DPoP key thumbprint on a token claims map under cnf.jkt.
// No-op when jkt is empty.
func SetCnfJkt(claims map[string]any, jkt string) {
	if jkt == "" {
		return
	}
	claims["cnf"] = map[string]any{"jkt": jkt}
}

// IsDPoPAuth checks if the Authorization header uses the DPoP scheme.
// Scheme matching is case-insensitive.
func IsDPoPAuth(authHeader string) bool {
	parts := strings.SplitN(authHeader, " ", 2)
	return len(parts) >= 1 && strings.EqualFold(parts[0], "DPoP")
}

// ExtractDPoPToken extracts the access token from a DPoP-scheme Authorization header.
// It validates that the header starts with "DPoP" (case-insensitive) and contains a
// non-empty token. Returns the token and an error if validation fails.
func ExtractDPoPToken(authHeader string) (string, error) {
	if authHeader == "" {
		return "", errors.New("missing Authorization header")
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "DPoP") {
		return "", errors.New("invalid Authorization header format. Expected: DPoP <token>")
	}

	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", errors.New("missing access token")
	}

	return token, nil
}

// VerifyProofBinding checks that the DPoP proof key thumbprint in ctx matches the
// thumbprint bound to a previously issued artifact (authorization code, refresh
// token, subject token, etc.). Returns nil when boundJkt is empty (artifact is
// not DPoP-bound) or matches the proof. Otherwise returns an invalid_grant
// ErrorResponse; tokenLabel is interpolated into the description.
func VerifyProofBinding(ctx context.Context, boundJkt, tokenLabel string) *model.ErrorResponse {
	if boundJkt == "" {
		return nil
	}
	proofJkt := GetJkt(ctx)
	if proofJkt == "" {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidGrant,
			ErrorDescription: "DPoP proof required for this " + tokenLabel,
		}
	}
	if proofJkt != boundJkt {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidGrant,
			ErrorDescription: "DPoP proof key does not match " + tokenLabel + " binding",
		}
	}
	return nil
}
