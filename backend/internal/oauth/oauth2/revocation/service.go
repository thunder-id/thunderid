// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocation

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	sharedrevocation "github.com/thunder-id/thunderid/internal/revocation"
	syscontext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/eventlistener"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
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
	// RevokeCriteriaBatch records a set of idempotent many-token revocations, refusing a set over the cap.
	RevokeCriteriaBatch(ctx context.Context, revocations []CriteriaRevocation) *tidcommon.ServiceError

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
	// validationLeeway is how long past exp a token is still accepted, so every row outlives it by that much.
	validationLeeway  time.Duration
	revokeTokenFamily bool
	observabilitySvc  providers.ObservabilityProvider
	// maxEntriesPerChange bounds one RevokeCriteriaBatch call.
	maxEntriesPerChange int
	// events announces each deny-list entry once it is recorded.
	events *eventlistener.Topic[sharedrevocation.Event]
	logger *log.Logger
}

// newRevocationService creates a new revocationService (internal use). It returns
// RevocationServiceInterface; consumers receive it narrowed to the embedded RefreshTokenRevokerInterface
// or CriteriaRevokerInterface subset, so they cannot invoke the full revocation API (e.g. RevokeToken).
// When revokeTokenFamily is true, an explicit revocation of a token carrying a token family id also revokes
// the whole family (so a login's access tokens drop with its refresh token). tokenFamilyLifetime bounds
// each token-family deny-list entry (revoked_at + tokenFamilyLifetime); a non-positive value falls back
// to defaultTokenFamilyRevocationTTL. validationLeeway extends every deny-list row by the JWT leeway.
func newRevocationService(
	jwtService jwt.JWTServiceInterface,
	store revocationStoreInterface,
	tokenFamilyLifetime time.Duration,
	validationLeeway time.Duration,
	revokeTokenFamily bool,
	observabilitySvc providers.ObservabilityProvider,
	maxEntriesPerChange int,
) RevocationServiceInterface {
	return buildRevocationService(jwtService, store, tokenFamilyLifetime, validationLeeway, revokeTokenFamily,
		observabilitySvc, maxEntriesPerChange)
}

// buildRevocationService creates the concrete service, for the caller that also needs its event topic.
func buildRevocationService(
	jwtService jwt.JWTServiceInterface,
	store revocationStoreInterface,
	tokenFamilyLifetime time.Duration,
	validationLeeway time.Duration,
	revokeTokenFamily bool,
	observabilitySvc providers.ObservabilityProvider,
	maxEntriesPerChange int,
) *revocationService {
	if tokenFamilyLifetime <= 0 {
		tokenFamilyLifetime = defaultTokenFamilyRevocationTTL
	}
	return &revocationService{
		jwtService:          jwtService,
		store:               store,
		tokenFamilyLifetime: tokenFamilyLifetime,
		validationLeeway:    max(validationLeeway, 0),
		revokeTokenFamily:   revokeTokenFamily,
		observabilitySvc:    observabilitySvc,
		maxEntriesPerChange: maxEntriesPerChange,
		events:              eventlistener.NewTopic[sharedrevocation.Event]("revocation"),
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
		ExpiryTime:       extractExpiryTime(payload).Add(s.validationLeeway),
	}
	if err := s.store.InsertRevokedToken(ctx, revoked); err != nil {
		return RevokeOutcomeRevoked, fmt.Errorf("failed to record token revocation: %w", err)
	}
	s.notifyTokenRevoked(ctx, revoked)

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
		ExpiryTime:       expiryTime.Add(s.validationLeeway),
	}
	if err := s.store.InsertRevokedToken(ctx, revoked); err != nil {
		return fmt.Errorf("failed to record refresh token revocation: %w", err)
	}
	s.notifyTokenRevoked(ctx, revoked)
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
	revocation := CriteriaRevocation{
		Criterion: Criterion{Type: CriterionTypeTokenFamily, Value: tokenFamilyID},
		Mode:      RevocationModeAll,
		Reason:    reason,
	}
	row, err := s.validatedCriterionRow(revocation, time.Now().UTC())
	if err != nil {
		return err
	}
	if row == nil {
		return nil
	}
	if err := s.store.insertCriterion(ctx, *row); err != nil {
		s.publishCriteriaRevocationFailedEvent(ctx, revocation)
		return fmt.Errorf("failed to revoke tokens by criteria: %w", err)
	}
	s.notifyCriteriaRevoked(ctx, *row)

	s.logger.Debug(ctx, "Revoked tokens by criteria",
		log.String("criterionType", string(CriterionTypeTokenFamily)),
		log.String("reason", string(reason)))
	return nil
}

// RevokeCriteriaBatch validates every revocation before writing any of them; a repeated criterion counts once.
func (s *revocationService) RevokeCriteriaBatch(ctx context.Context,
	revocations []CriteriaRevocation) *tidcommon.ServiceError {
	revocations = dedupeRevocations(revocations)
	if len(revocations) == 0 {
		return nil
	}
	if len(revocations) > s.maxEntriesPerChange {
		return sharedrevocation.ErrorEntryLimitExceeded.WithParams(map[string]string{
			"entries": strconv.Itoa(len(revocations)),
			"max":     strconv.Itoa(s.maxEntriesPerChange),
		})
	}
	now := time.Now().UTC()
	rows := make([]revocationCriterion, 0, len(revocations))
	for _, revocation := range revocations {
		row, err := s.validatedCriterionRow(revocation, now)
		if err != nil {
			s.logger.Error(ctx, "Refused an invalid criteria revocation", log.Error(err))
			return &tidcommon.InternalServerError
		}
		if row != nil {
			rows = append(rows, *row)
		}
	}
	if len(rows) == 0 {
		return nil
	}
	if err := s.store.insertCriteria(ctx, rows); err != nil {
		s.logger.Error(ctx, "Failed to revoke tokens by criteria", log.Error(err))
		for _, revocation := range revocations {
			s.publishCriteriaRevocationFailedEvent(ctx, revocation)
		}
		return &tidcommon.InternalServerError
	}
	s.notifyCriteriaRevoked(ctx, rows...)

	s.logger.Debug(ctx, "Revoked tokens by criteria batch", log.Int("count", len(rows)))
	return nil
}

// notifyTokenRevoked announces a recorded single-token revocation to the event listeners.
func (s *revocationService) notifyTokenRevoked(ctx context.Context, token RevokedToken) {
	if !s.events.HasListeners() {
		return
	}
	s.events.Notify(ctx, sharedrevocation.Event{
		JTI:        token.JTI,
		Reason:     token.RevocationReason,
		RevokedAt:  token.RevokedAt,
		ExpiryTime: token.ExpiryTime,
	})
}

// notifyCriteriaRevoked announces recorded criteria revocations to the event listeners.
func (s *revocationService) notifyCriteriaRevoked(ctx context.Context, rows ...revocationCriterion) {
	if !s.events.HasListeners() {
		return
	}
	for _, row := range rows {
		s.events.Notify(ctx, sharedrevocation.Event{
			Criterion:  Criterion{Type: row.Type, Value: row.Value},
			Reason:     row.Reason,
			RevokedAt:  row.RevokedAt,
			ExpiryTime: row.ExpiryTime,
		})
	}
}

// validatedCriterionRow validates a revocation and builds its deny-list row, or nil when it has no value.
func (s *revocationService) validatedCriterionRow(revocation CriteriaRevocation, now time.Time) (
	*revocationCriterion, error) {
	if revocation.Criterion.Value == "" {
		return nil, nil
	}
	if !isSupportedCriterionType(revocation.Criterion.Type) {
		return nil, fmt.Errorf("unsupported revocation criterion type: %q", revocation.Criterion.Type)
	}
	if revocation.Mode != RevocationModeAll && revocation.Mode != RevocationModeBeforeAction {
		return nil, fmt.Errorf("unsupported revocation mode: %q", revocation.Mode)
	}
	if revocation.Mode == RevocationModeBeforeAction && revocation.Cutoff.IsZero() {
		return nil, fmt.Errorf("cutoff is required for %s", RevocationModeBeforeAction)
	}
	if (revocation.Mode == RevocationModeBeforeAction) != isBoundaryReason(revocation.Reason) {
		return nil, fmt.Errorf("revocation mode %q does not match reason %q", revocation.Mode, revocation.Reason)
	}

	cutoff := revocation.Cutoff
	if revocation.Mode == RevocationModeAll {
		cutoff = time.Time{}
	}
	revokedAt := now
	if revocation.Mode == RevocationModeBeforeAction {
		revokedAt = cutoff.UTC()
	}
	return &revocationCriterion{
		Type:       revocation.Criterion.Type,
		Value:      revocation.Criterion.Value,
		Reason:     revocation.Reason,
		RevokedAt:  revokedAt,
		ExpiryTime: s.criterionExpiry(now, revokedAt, revocation.TTL),
	}, nil
}

// criterionExpiry is when a row may lapse: after the last token it covers, one issued at the cutoff,
// has expired and its leeway has passed. A cutoff may lie ahead of the write.
func (s *revocationService) criterionExpiry(now, revokedAt time.Time, requested time.Duration) time.Time {
	base := now
	if revokedAt.After(base) {
		base = revokedAt
	}
	return base.Add(s.resolveCriterionLifetime(requested) + s.validationLeeway)
}

// resolveCriterionLifetime returns how long a criteria deny-list row must survive. The configured
// lifetime is only a default, so a caller that knows its artifacts live longer can ask for more: the
// longer of the two wins, and a caller can never cut a row short.
func (s *revocationService) resolveCriterionLifetime(requested time.Duration) time.Duration {
	if requested > s.tokenFamilyLifetime {
		return requested
	}
	return s.tokenFamilyLifetime
}

// dedupeRevocations keeps the first revocation of each criterion, since one change revokes it under one plan.
func dedupeRevocations(revocations []CriteriaRevocation) []CriteriaRevocation {
	seen := make(map[Criterion]struct{}, len(revocations))
	unique := make([]CriteriaRevocation, 0, len(revocations))
	for _, revocation := range revocations {
		if _, ok := seen[revocation.Criterion]; ok {
			continue
		}
		seen[revocation.Criterion] = struct{}{}
		unique = append(unique, revocation)
	}
	return unique
}

func isSupportedCriterionType(criterionType CriterionType) bool {
	switch criterionType {
	case CriterionTypeTokenFamily, CriterionTypeSubject, CriterionTypeApplicationID,
		CriterionTypeApplicationKey, CriterionTypeOrganizationUnit, CriterionTypeRole,
		CriterionTypeGroup, CriterionTypeConsent, CriterionTypeCredentialVersion,
		CriterionTypeEntityScope, CriterionTypeScope:
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

// publishCriteriaRevocationFailedEvent emits a TOKEN_REVOCATION_FAILED alert for a criteria revocation
// that could not be stored.
func (s *revocationService) publishCriteriaRevocationFailedEvent(ctx context.Context,
	revocation CriteriaRevocation) {
	if s.observabilitySvc == nil || !s.observabilitySvc.IsEnabled() {
		return
	}

	evt := event.NewEvent(
		syscontext.GetTraceID(ctx),
		string(event.EventTypeTokenRevocationFailed),
		event.ComponentAuthHandler,
	).
		WithStatus(providers.StatusFailure).
		WithData(event.DataKey.CriterionType, string(revocation.Criterion.Type)).
		WithData(event.DataKey.RevocationReason, string(revocation.Reason))
	switch revocation.Criterion.Type {
	case CriterionTypeSubject:
		evt.WithData(event.DataKey.Subject, revocation.Criterion.Value)
	case CriterionTypeApplicationKey:
		evt.WithData(event.DataKey.ClientID, revocation.Criterion.Value)
	}

	s.observabilitySvc.PublishEvent(ctx, evt)
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
