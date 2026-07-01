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

package openid4vci

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/openid4vci/credential"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/jose/sdjwt"
)

// IssueCredential validates the bearer access token and holder proof, then
// issues an SD-JWT VC bound to the holder key with claims sourced from the
// authenticated subject's profile. The credential the wallet is authorized for
// is determined by the access-token scope (matched against credential configs).
func (s *service) IssueCredential(ctx context.Context, accessToken string, body []byte) (*CredentialResponse, error) {
	if accessToken == "" {
		return nil, fmt.Errorf("%w: missing access token", ErrInvalidToken)
	}
	if svcErr := s.jwtService.VerifyJWT(ctx, accessToken, "", ""); svcErr != nil {
		return nil, fmt.Errorf("%w: access token verification failed", ErrInvalidToken)
	}
	payload, err := jwt.DecodeJWTPayload(accessToken)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	subject, _ := payload["sub"].(string)
	if subject == "" {
		return nil, fmt.Errorf("%w: access token missing subject", ErrInvalidToken)
	}
	scopes := strings.Fields(scopeString(payload))

	var req CredentialRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidRequest, err)
	}

	cred, err := s.authorizedCredential(ctx, req.CredentialConfigurationID, scopes)
	if err != nil {
		return nil, err
	}

	proofs := req.holderProofs()
	if len(proofs) == 0 {
		return nil, fmt.Errorf("%w: missing holder proof", ErrInvalidProof)
	}
	if len(proofs) > s.cfg.BatchSize {
		return nil, fmt.Errorf("%w: %d proofs exceeds batch size %d", ErrInvalidRequest, len(proofs), s.cfg.BatchSize)
	}

	holderJWKs, err := s.verifyProofs(ctx, proofs)
	if err != nil {
		return nil, err
	}

	claims, err := s.resolveClaims(ctx, subject, cred.SDClaims)
	if err != nil {
		return nil, err
	}

	validity := s.cfg.CredentialValidity
	if cred.Validity > 0 {
		validity = cred.Validity
	}

	now := time.Now()
	issued := make([]IssuedCredential, 0, len(holderJWKs))
	for _, holderJWK := range holderJWKs {
		combined, _, err := sdjwt.Issue(sdjwt.IssueParams{
			Header:          s.signer.header(cred.Format),
			Issuer:          s.cfg.CredentialIssuer,
			VCT:             cred.VCT,
			IssuedAt:        now,
			ExpiresAt:       now.Add(validity),
			SelectiveClaims: claims,
			AlwaysVisible:   map[string]interface{}{"sub": subject},
			ConfirmationJWK: holderJWK,
		}, func(signingInput string) ([]byte, error) {
			return s.signer.sign(ctx, signingInput)
		})
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrIssuance, err)
		}
		issued = append(issued, IssuedCredential{Credential: combined})
	}

	return &CredentialResponse{Credentials: issued}, nil
}

// authorizedCredential resolves the credential configuration the request is
// authorized for. When a credential_configuration_id is supplied it must match a
// managed credential; scope binding is enforced only when configured. Without a
// configuration id, the first managed credential whose scope is in the token is used.
func (s *service) authorizedCredential(
	ctx context.Context, configID string, scopes []string,
) (credentialConfig, error) {
	scopeSet := make(map[string]bool, len(scopes))
	for _, sc := range scopes {
		scopeSet[sc] = true
	}

	if configID != "" {
		dto, svcErr := s.creds.GetCredentialConfigurationByHandle(ctx, configID)
		if svcErr != nil {
			return credentialConfig{}, fmt.Errorf("%w: %s", ErrUnsupportedCredential, configID)
		}
		if s.cfg.EnforceScope && !scopeSet[dto.Handle] {
			return credentialConfig{}, fmt.Errorf("%w: scope %q not authorized", ErrInvalidToken, dto.Handle)
		}
		return dtoToCredentialConfig(*dto), nil
	}

	configs, svcErr := s.creds.ListCredentialConfigurations(ctx)
	if svcErr != nil {
		return credentialConfig{}, fmt.Errorf("%w: failed to resolve credential configurations", ErrIssuance)
	}
	for _, dto := range configs {
		if scopeSet[dto.Handle] {
			return dtoToCredentialConfig(dto), nil
		}
	}
	return credentialConfig{}, fmt.Errorf("%w: no authorized credential scope present", ErrInvalidToken)
}

// dtoToCredentialConfig maps a managed credential DTO to the issuer's internal config.
func dtoToCredentialConfig(dto credential.CredentialConfigurationDTO) credentialConfig {
	format := dto.Format
	if format == "" {
		format = credential.DefaultCredentialFormat
	}
	names := make([]string, 0, len(dto.Claims))
	for _, c := range dto.Claims {
		names = append(names, c.Name)
	}
	var validity time.Duration
	if dto.ValiditySeconds != nil {
		validity = time.Duration(*dto.ValiditySeconds) * time.Second
	}
	return credentialConfig{
		Format:   format,
		VCT:      dto.VCT,
		SDClaims: names,
		Validity: validity,
	}
}

// scopeString extracts the space-delimited scope claim, tolerating either a
// string or a []string representation.
func scopeString(payload map[string]interface{}) string {
	switch v := payload["scope"].(type) {
	case string:
		return v
	case []interface{}:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, " ")
	default:
		return ""
	}
}
