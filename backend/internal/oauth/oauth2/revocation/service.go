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
	oauth2utils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	syscontext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// statusRevoked is the Token Status List status value for a revoked token (spec 0x01, INVALID).
const statusRevoked = 1

// TokenStatusWriter records a token's status in the Token Status List, the sole revocation mechanism.
// It is satisfied by the statuslist subsystem and injected at the composition root; a nil writer means
// the list feature is disabled, in which case revocation has nothing to record and is a no-op. The
// signature matches the subsystem's service so it is satisfied structurally without the revocation
// package importing it.
type TokenStatusWriter interface {
	SetStatus(ctx context.Context, uri string, idx int64, status int, expiry time.Time) error
}

// RevocationServiceInterface defines the OAuth2 token revocation service (RFC 7009).
type RevocationServiceInterface interface {
	RefreshTokenRevokerInterface

	// RevokeToken revokes the presented token on behalf of the authenticated client.
	//
	// token_type_hint is accepted per RFC 7009 §2.1 but intentionally not acted on. The hint exists to help
	// a server that stores opaque tokens in type-partitioned stores decide which store to search first. Our
	// tokens are self-contained JWTs revoked by flipping their status-list bit, so the type is
	// auto-detectable from the token and never guides a lookup — the case where RFC 7009 §2.1 explicitly
	// permits ignoring it. It is retained in the signature as a forward-fit for a future opaque-token model.
	//
	// It returns an error only on server errors; all token-state outcomes are conveyed via RevokeOutcome.
	RevokeToken(ctx context.Context, token, tokenTypeHint, authenticatedClientID string) (RevokeOutcome, error)
}

// RefreshTokenRevokerInterface is the narrow write seam the refresh grant uses to enforce single-use
// refresh tokens (RFC 9700 §4.14.2): the consumed refresh token's status-list bit is flipped so it
// cannot be replayed. It exposes no read or client-facing revocation.
type RefreshTokenRevokerInterface interface {
	// RevokeRefreshToken records a consumed refresh token as revoked on rotation by flipping its
	// status-list bit. expiryTime bounds the recorded entry's lifetime. An empty statusURI (the list
	// feature is disabled, or a pre-feature token) is a no-op.
	RevokeRefreshToken(ctx context.Context, statusURI string, statusIdx int64, jti string, expiryTime time.Time) error
}

// revocationService implements RevocationServiceInterface.
type revocationService struct {
	jwtService       jwt.JWTServiceInterface
	statusWriter     TokenStatusWriter
	observabilitySvc providers.ObservabilityProvider
	logger           *log.Logger
}

// newRevocationService creates a new revocationService (internal use). It returns
// RevocationServiceInterface; the same instance is handed to the refresh grant narrowed to the
// embedded RefreshTokenRevokerInterface subset, so the grant cannot invoke the full revocation API.
// statusWriter may be nil, in which case revocation is a no-op (the Token Status List feature is off).
func newRevocationService(
	jwtService jwt.JWTServiceInterface,
	statusWriter TokenStatusWriter,
	observabilitySvc providers.ObservabilityProvider,
) RevocationServiceInterface {
	return &revocationService{
		jwtService:       jwtService,
		statusWriter:     statusWriter,
		observabilitySvc: observabilitySvc,
		logger:           log.GetLogger().With(log.String(log.LoggerKeyComponentName, "RevocationService")),
	}
}

// RevokeToken validates and revokes the presented token. The token_type_hint parameter is ignored
// (blank identifier) for the reasons documented on RevocationServiceInterface.RevokeToken.
//
// Per RFC 7009: signature is verified but expiry is intentionally not checked (expired tokens remain
// revocable). An invalid, unparseable, or unknown token is a successful no-op. A token issued to a
// different client is rejected with invalid_grant. Revocation flips the token's status-list bit; a
// token with no status reference (the list feature is off, or the token predates it) has no revocation
// channel and is treated as a no-op success.
func (s *revocationService) RevokeToken(
	ctx context.Context, token, _, authenticatedClientID string,
) (RevokeOutcome, error) {
	// Signature-only verification: a token we did not issue (or a tampered one) must not touch the
	// status list. Expiry is deliberately ignored so expired tokens remain revocable.
	if err := s.jwtService.VerifyJWTSignature(ctx, token); err != nil {
		s.logger.Debug(ctx, "Revocation request for a token that failed signature verification; "+
			"treating as a no-op success per RFC 7009")
		return RevokeOutcomeRevoked, nil
	}

	_, payload, decodeErr := jwt.DecodeJWT(token)
	if decodeErr != nil {
		s.logger.Debug(ctx, "Revocation request for an undecodable token; treating as a no-op success")
		return RevokeOutcomeRevoked, nil
	}

	jti, _ := payload[constants.ClaimJTI].(string)
	if jti == "" {
		s.logger.Debug(ctx, "Revocation request for a token without a jti claim; nothing to revoke")
		return RevokeOutcomeRevoked, nil
	}

	// Ownership enforcement: a client may only revoke tokens issued to it. ThunderID tokens carry the
	// owning client in the client_id claim (no azp), so ownership is checked against client_id; a
	// mismatch is rejected with invalid_grant per RFC 7009 §2.1.
	tokenClientID, _ := payload[constants.ClaimClientID].(string)
	if tokenClientID != "" && authenticatedClientID != "" && tokenClientID != authenticatedClientID {
		s.logger.Debug(ctx, "Revocation request for a token belonging to a different client")
		return RevokeOutcomeNotOwned, nil
	}

	// Flip the token's status-list bit. A token without a status reference has no revocation channel
	// (the list feature is off, or it predates the feature); nothing can be recorded, so per RFC 7009
	// the request is a no-op success.
	// A malformed reference (ok=false with a non-nil error) leaves nothing to flip; per RFC 7009 an
	// unrevocable token is still a success, and the enforcement path fails such a token closed anyway.
	uri, idx, ok, _ := oauth2utils.ExtractStatusListReference(payload)
	if s.statusWriter == nil || !ok {
		s.logger.Debug(ctx, "Revocation request for a token without a status list reference; "+
			"nothing to revoke")
		return RevokeOutcomeRevoked, nil
	}

	if err := s.statusWriter.SetStatus(ctx, uri, idx, statusRevoked, extractExpiryTime(payload)); err != nil {
		return RevokeOutcomeRevoked, fmt.Errorf("failed to record token revocation: %w", err)
	}

	s.publishTokenRevokedEvent(ctx, authenticatedClientID, jti)
	return RevokeOutcomeRevoked, nil
}

// RevokeRefreshToken records a consumed refresh token as revoked on rotation, enforcing single-use. The
// token was already validated by the refresh grant, so no signature or ownership check is repeated here.
// The rotated token's status-list bit is flipped. An empty statusURI (the list feature is off, or a
// pre-feature token) leaves nothing to record and is a no-op. jti is retained for logging/forward-fit.
func (s *revocationService) RevokeRefreshToken(
	ctx context.Context, statusURI string, statusIdx int64, _ string, expiryTime time.Time,
) error {
	if s.statusWriter == nil || statusURI == "" {
		return nil
	}
	if err := s.statusWriter.SetStatus(ctx, statusURI, statusIdx, statusRevoked, expiryTime); err != nil {
		return fmt.Errorf("failed to record refresh token revocation: %w", err)
	}
	s.logger.Debug(ctx, "Revoked refresh token via status list")
	return nil
}

// extractExpiryTime returns the token's exp claim as a time, falling back to now when absent
// (an absent/expired exp simply makes the deny-list row immediately cleanup-eligible).
func extractExpiryTime(payload map[string]interface{}) time.Time {
	if exp, ok := payload[constants.ClaimExp].(float64); ok {
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
		WithData(event.DataKey.RevocationReason, string(RevocationReasonExplicit))

	s.observabilitySvc.PublishEvent(ctx, evt)
}
