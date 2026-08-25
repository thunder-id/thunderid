// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocation

import (
	"context"
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	sharedrevocation "github.com/thunder-id/thunderid/internal/revocation"
	syscontext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// RevocationServiceInterface is the OAuth2 revocation write service. It covers single-token revocation
// (the RFC 7009 endpoint and single-use refresh rotation, recorded on the JTI deny list) and
// token-family revocation (recorded on the criteria deny list). The enforcement service is the
// matching read path.
type RevocationServiceInterface interface {
	RefreshTokenRevokerInterface
	CriteriaRevokerInterface

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

// RefreshTokenRevokerInterface is the narrow write seam the refresh grant uses to enforce single-use
// refresh tokens (RFC 9700 §4.14.2): the consumed refresh token is recorded on the deny list so it
// cannot be replayed. It exposes no read or client-facing revocation.
type RefreshTokenRevokerInterface interface {
	// RevokeRefreshToken records the refresh token's jti on the deny list with the refresh_rotation
	// reason. expiryTime is the token's original expiry, which bounds the deny-list entry's lifetime.
	// An empty jti is a no-op.
	RevokeRefreshToken(ctx context.Context, jti string, expiryTime time.Time) error
}

// CriteriaRevokerInterface is the narrow write seam for criteria-based (many-token) revocation: it
// records a revocation criterion so every token matching it is rejected. The only criterion today is
// token_family, which revokes a whole authorization grant by its token family id (tfid); the seam is
// shaped so future criteria (e.g. by subject or client) reuse the same writer and store. It is
// consumed by the refresh grant (reuse), the RFC 7009 endpoint (explicit), the authorization service
// (code replay), and session sign-out (logout).
type CriteriaRevokerInterface interface {
	// RevokeByCriteria records an idempotent many-token revocation.
	RevokeByCriteria(ctx context.Context, revocation CriteriaRevocation) error

	// RevokeTokenFamily records a terminal revocation of the token family identified by tokenFamilyID, so
	// every access and refresh token carrying that tfid is rejected. An empty tokenFamilyID is a
	// no-op. The write is idempotent.
	RevokeTokenFamily(ctx context.Context, tokenFamilyID string, reason RevocationReason) error
}

// defaultTokenFamilyRevocationTTL bounds a family revocation entry when no refresh-token lifetime is
// configured. It is a safe upper bound: the entry only needs to outlive the longest-lived token of
// the family, and expired tokens are rejected on their own exp regardless.
const defaultTokenFamilyRevocationTTL = 30 * 24 * time.Hour

// revocationService implements RevocationServiceInterface.
type revocationService struct {
	jwtService          jwt.JWTServiceInterface
	store               revocationStoreInterface
	tokenFamilyLifetime time.Duration
	revokeTokenFamily   bool
	observabilitySvc    providers.ObservabilityProvider
	logger              *log.Logger
}

// newRevocationService creates a new revocationService (internal use). It returns
// RevocationServiceInterface; consumers receive it narrowed to the embedded RefreshTokenRevokerInterface
// or CriteriaRevokerInterface subset, so they cannot invoke the full revocation API (e.g. RevokeToken).
// When revokeTokenFamily is true, an explicit revocation of a token carrying a token family id also revokes
// the whole family (so a login's access tokens drop with its refresh token). tokenFamilyLifetime bounds
// each token-family deny-list entry (revoked_at + tokenFamilyLifetime); a non-positive value falls back
// to defaultTokenFamilyRevocationTTL.
func newRevocationService(
	jwtService jwt.JWTServiceInterface,
	store revocationStoreInterface,
	tokenFamilyLifetime time.Duration,
	revokeTokenFamily bool,
	observabilitySvc providers.ObservabilityProvider,
) RevocationServiceInterface {
	if tokenFamilyLifetime <= 0 {
		tokenFamilyLifetime = defaultTokenFamilyRevocationTTL
	}
	return &revocationService{
		jwtService:          jwtService,
		store:               store,
		tokenFamilyLifetime: tokenFamilyLifetime,
		revokeTokenFamily:   revokeTokenFamily,
		observabilitySvc:    observabilitySvc,
		logger:              log.GetLogger().With(log.String(log.LoggerKeyComponentName, "RevocationService")),
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
	// Signature-only verification: a token we did not issue (or a tampered one) must not pollute the
	// deny list. Expiry is deliberately ignored so expired tokens remain revocable.
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

	// Ownership enforcement: a client may only revoke tokens issued to it (RFC 7009 §2.1). ThunderID
	// records the owning client in the client_id claim on access tokens; refresh tokens carry no
	// client_id claim but are minted with the owning client as their subject, so fall back to sub when
	// client_id is absent. Without this fallback the check would be skipped for refresh tokens, letting
	// any authenticated client revoke another client's refresh token (and cascade its token family). A
	// mismatch is rejected with invalid_grant.
	tokenClientID, _ := payload[constants.ClaimClientID].(string)
	if tokenClientID == "" {
		tokenClientID, _ = payload[constants.ClaimSub].(string)
	}
	if tokenClientID != "" && authenticatedClientID != "" && tokenClientID != authenticatedClientID {
		s.logger.Debug(ctx, "Revocation request for a token belonging to a different client")
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

	// When the revoked token carries a token family id, drop the whole family so revoking a login's
	// refresh token also invalidates the access tokens issued from that same login (RFC 7009 §2.1).
	if s.revokeTokenFamily {
		if tokenFamilyID, _ := payload[constants.ClaimTokenFamilyID].(string); tokenFamilyID != "" {
			if err := s.RevokeTokenFamily(ctx, tokenFamilyID,
				RevocationReasonExplicitTokenFamily); err != nil {
				return RevokeOutcomeRevoked, fmt.Errorf("failed to revoke token family: %w", err)
			}
		}
	}

	s.publishTokenRevokedEvent(ctx, authenticatedClientID, jti)
	return RevokeOutcomeRevoked, nil
}

// RevokeRefreshToken records a refresh token on the deny list with the refresh_rotation reason,
// enforcing single-use on rotation. The token was already validated by the refresh grant, so no
// signature or ownership check is repeated here. An empty jti is a no-op.
func (s *revocationService) RevokeRefreshToken(ctx context.Context, jti string, expiryTime time.Time) error {
	if jti == "" {
		return nil
	}
	revoked := RevokedToken{
		JTI:              jti,
		RevocationReason: RevocationReasonRefreshRotation,
		RevokedAt:        time.Now().UTC(),
		ExpiryTime:       expiryTime,
	}
	if err := s.store.InsertRevokedToken(ctx, revoked); err != nil {
		return fmt.Errorf("failed to record refresh token revocation: %w", err)
	}
	s.logger.Debug(ctx, "Revoked refresh token")
	return nil
}

// RevokeTokenFamily records a terminal revocation of the token family (one authorization grant)
// identified by tokenFamilyID, writing a token_family criterion to the deny list so every access and
// refresh token carrying that tfid is rejected. The refresh grant (reuse), the authorization service
// (code replay), and session sign-out (logout) revoke families through this same method that backs the
// RFC 7009 endpoint's explicit-revoke family cascade. An empty tokenFamilyID is a no-op; the write is
// idempotent.
func (s *revocationService) RevokeTokenFamily(ctx context.Context, tokenFamilyID string,
	reason RevocationReason) error {
	return s.RevokeByCriteria(ctx, CriteriaRevocation{
		Criterion: Criterion{Type: CriterionTypeTokenFamily, Value: tokenFamilyID},
		Mode:      RevocationModeAll,
		Reason:    reason,
	})
}

// RevokeByCriteria records a validated many-token revocation in the criteria deny list.
func (s *revocationService) RevokeByCriteria(ctx context.Context, revocation CriteriaRevocation) error {
	if revocation.Criterion.Value == "" {
		return nil
	}
	if !isSupportedCriterionType(revocation.Criterion.Type) {
		return fmt.Errorf("unsupported revocation criterion type: %q", revocation.Criterion.Type)
	}
	if revocation.Mode != RevocationModeAll && revocation.Mode != RevocationModeBeforeAction {
		return fmt.Errorf("unsupported revocation mode: %q", revocation.Mode)
	}
	if revocation.Mode == RevocationModeBeforeAction && revocation.Cutoff.IsZero() {
		return fmt.Errorf("cutoff is required for %s", RevocationModeBeforeAction)
	}
	if (revocation.Mode == RevocationModeBeforeAction) != isBoundaryReason(revocation.Reason) {
		return fmt.Errorf("revocation mode %q does not match reason %q", revocation.Mode, revocation.Reason)
	}
	if revocation.Mode == RevocationModeAll {
		revocation.Cutoff = time.Time{}
	}

	now := time.Now().UTC()
	revokedAt := now
	if revocation.Mode == RevocationModeBeforeAction {
		revokedAt = revocation.Cutoff.UTC()
	}
	if err := s.store.insertCriterion(ctx, revocationCriterion{
		Type:       revocation.Criterion.Type,
		Value:      revocation.Criterion.Value,
		Reason:     revocation.Reason,
		RevokedAt:  revokedAt,
		ExpiryTime: now.Add(s.resolveCriterionLifetime(revocation.TTL)),
	}); err != nil {
		return fmt.Errorf("failed to revoke tokens by criteria: %w", err)
	}

	s.logger.Debug(ctx, "Revoked tokens by criteria",
		log.String("criterionType", string(revocation.Criterion.Type)),
		log.String("reason", string(revocation.Reason)))
	return nil
}

// resolveCriterionLifetime returns how long a criteria deny-list row must survive.
//
// The configured lifetime is derived from the deployment-wide refresh-token validity, which is only a
// default: token validity is configurable per application and uncapped, so a caller that knows the
// lifetime of the artifacts its criterion matches can ask for longer. The longer of the two wins, so a
// caller can only extend a row, never cut it short.
func (s *revocationService) resolveCriterionLifetime(requested time.Duration) time.Duration {
	if requested > s.tokenFamilyLifetime {
		return requested
	}
	return s.tokenFamilyLifetime
}

func isSupportedCriterionType(criterionType CriterionType) bool {
	switch criterionType {
	case CriterionTypeTokenFamily, CriterionTypeSubject, CriterionTypeApplicationID,
		CriterionTypeApplicationKey, CriterionTypeOrganizationUnit, CriterionTypeRole,
		CriterionTypeGroup, CriterionTypeConsent, CriterionTypeCredentialVersion:
		return true
	default:
		return false
	}
}

func isBoundaryReason(reason RevocationReason) bool {
	return sharedrevocation.IsBoundaryReason(reason)
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
