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

package revocation

import (
	"context"
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	syscontext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// RevokeOutcome is the protocol-level result of a revocation request, mapped to HTTP by the handler.
type RevokeOutcome int

const (
	// RevokeOutcomeRevoked indicates success — the token was revoked, or it was invalid/expired/unknown
	// and treated as a no-op success per RFC 7009 §2.2. Maps to HTTP 200.
	RevokeOutcomeRevoked RevokeOutcome = iota
	// RevokeOutcomeNotOwned indicates the token was issued to a different client. Maps to 400 invalid_grant.
	RevokeOutcomeNotOwned
)

// RevocationServiceInterface defines the OAuth2 token revocation service (RFC 7009).
type RevocationServiceInterface interface {
	// RevokeToken revokes the presented token on behalf of the authenticated client.
	//
	// token_type_hint is accepted per RFC 7009 §2.1 but intentionally not acted on. The hint exists to help
	// a server that stores opaque tokens in type-partitioned stores decide which store to search first. Our
	// tokens are self-contained JWTs revoked by jti into a single deny-list, so the type is auto-detectable
	// from the token and never guides a lookup — the case where RFC 7009 §2.1 explicitly permits ignoring it.
	// It is retained in the signature as a forward-fit for a future opaque/reference-token model.
	//
	// It returns an error only on server errors; all token-state outcomes are conveyed via RevokeOutcome.
	RevokeToken(ctx context.Context, token, tokenTypeHint, authenticatedClientID string) (RevokeOutcome, error)
}

// revocationService implements RevocationServiceInterface.
type revocationService struct {
	jwtService       jwt.JWTServiceInterface
	store            RevokedTokenWriterInterface
	observabilitySvc providers.ObservabilityProvider
}

// newRevocationService creates a new revocationService (internal use).
func newRevocationService(
	jwtService jwt.JWTServiceInterface,
	store RevokedTokenWriterInterface,
	observabilitySvc providers.ObservabilityProvider,
) RevocationServiceInterface {
	return &revocationService{
		jwtService:       jwtService,
		store:            store,
		observabilitySvc: observabilitySvc,
	}
}

// RevokeToken validates and revokes the presented token. The token_type_hint parameter is ignored
// (blank identifier) for the reasons documented on RevocationServiceInterface.RevokeToken.
//
// Per RFC 7009: signature is verified but expiry is intentionally not checked (expired tokens remain
// revocable). An invalid, unparseable, or unknown token is a successful no-op. A token issued to a
// different client is rejected with invalid_grant.
func (s *revocationService) RevokeToken(
	ctx context.Context, token, _, authenticatedClientID string,
) (RevokeOutcome, error) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "RevocationService"))

	// Signature-only verification: a token we did not issue (or a tampered one) must not pollute the
	// deny list. Expiry is deliberately ignored so expired tokens remain revocable.
	if err := s.jwtService.VerifyJWTSignature(ctx, token); err != nil {
		logger.Debug(ctx, "Revocation request for a token that failed signature verification; "+
			"treating as a no-op success per RFC 7009")
		return RevokeOutcomeRevoked, nil
	}

	_, payload, decodeErr := jwt.DecodeJWT(token)
	if decodeErr != nil {
		logger.Debug(ctx, "Revocation request for an undecodable token; treating as a no-op success")
		return RevokeOutcomeRevoked, nil
	}

	jti, _ := payload[constants.ClaimJTI].(string)
	if jti == "" {
		logger.Debug(ctx, "Revocation request for a token without a jti claim; nothing to revoke")
		return RevokeOutcomeRevoked, nil
	}

	// Ownership enforcement: a client may only revoke tokens issued to it. ThunderID tokens carry the
	// owning client in the client_id claim (no azp), so ownership is checked against client_id; a
	// mismatch is rejected with invalid_grant per RFC 7009 §2.1.
	tokenClientID, _ := payload["client_id"].(string)
	if tokenClientID != "" && authenticatedClientID != "" && tokenClientID != authenticatedClientID {
		logger.Debug(ctx, "Revocation request for a token belonging to a different client")
		return RevokeOutcomeNotOwned, nil
	}

	revoked := RevokedToken{
		JTI:              jti,
		RevocationReason: RevocationReasonExplicit,
		RevokedAt:        time.Now().UTC(),
		ExpiryTime:       extractExpiryTime(payload),
	}
	if err := s.store.InsertRevokedToken(ctx, revoked); err != nil {
		return RevokeOutcomeRevoked, fmt.Errorf("failed to record token revocation: %w", err)
	}

	s.publishTokenRevokedEvent(ctx, authenticatedClientID, jti)
	return RevokeOutcomeRevoked, nil
}

// extractExpiryTime returns the token's exp claim as a time, falling back to now when absent
// (an absent/expired exp simply makes the deny-list row immediately cleanup-eligible).
func extractExpiryTime(payload map[string]interface{}) time.Time {
	if exp, ok := payload["exp"].(float64); ok {
		return time.Unix(int64(exp), 0).UTC()
	}
	return time.Now().UTC()
}

// publishTokenRevokedEvent emits a TOKEN_REVOKED audit event.
func (s *revocationService) publishTokenRevokedEvent(ctx context.Context, clientID, jti string) {
	if s.observabilitySvc == nil || !s.observabilitySvc.IsEnabled() {
		return
	}

	evt := event.NewEvent(
		syscontext.GetTraceID(ctx),
		string(event.EventTypeTokenRevoked),
		event.ComponentAuthHandler,
	).
		WithStatus(providers.StatusSuccess).
		WithData(event.DataKey.ClientID, clientID).
		WithData(event.DataKey.JTI, jti).
		WithData("revocation_reason", string(RevocationReasonExplicit))

	s.observabilitySvc.PublishEvent(ctx, evt)
}
