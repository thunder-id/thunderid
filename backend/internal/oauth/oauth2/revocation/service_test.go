// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocation

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	serviceerror "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
	"github.com/thunder-id/thunderid/tests/mocks/observability/observabilitymock"
)

const testClientID = "test-client-id"

type RevocationServiceTestSuite struct {
	suite.Suite
	jwtServiceMock *jwtmock.JWTServiceInterfaceMock
	storeMock      *revocationStoreInterfaceMock
	obsMock        *observabilitymock.ObservabilityServiceInterfaceMock
	service        RevocationServiceInterface
}

func TestRevocationServiceTestSuite(t *testing.T) {
	suite.Run(t, new(RevocationServiceTestSuite))
}

func (s *RevocationServiceTestSuite) SetupTest() {
	s.jwtServiceMock = jwtmock.NewJWTServiceInterfaceMock(s.T())
	s.storeMock = newRevocationStoreInterfaceMock(s.T())
	s.obsMock = observabilitymock.NewObservabilityServiceInterfaceMock(s.T())
	s.service = newRevocationService(s.jwtServiceMock, s.storeMock, time.Hour, true, s.obsMock)
}

// buildToken constructs a JWT-shaped string with the given claims. DecodeJWT only base64-decodes the
// header/payload (signature verification is mocked), so a dummy signature segment is sufficient.
func buildToken(claims map[string]interface{}) string {
	header, _ := json.Marshal(map[string]interface{}{"alg": "RS256", "typ": "JWT"})
	payload, _ := json.Marshal(claims)
	return base64.RawURLEncoding.EncodeToString(header) + "." +
		base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}

func (s *RevocationServiceTestSuite) TestRevokeToken_Success() {
	token := buildToken(map[string]interface{}{
		"jti":       "jti-123",
		"client_id": testClientID,
		"exp":       float64(time.Now().Add(time.Hour).Unix()),
	})
	s.jwtServiceMock.On("VerifyJWTSignature", mock.Anything, token).Return(nil)
	s.storeMock.On("InsertRevokedToken", mock.Anything, mock.MatchedBy(func(rt RevokedToken) bool {
		return rt.JTI == "jti-123" && rt.RevocationReason == RevocationReasonExplicit
	})).Return(nil)
	s.obsMock.On("IsEnabled").Return(false)

	revokeOutcome, err := s.service.RevokeToken(context.Background(), token, "", testClientID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), RevokeOutcomeRevoked, revokeOutcome)
}

func (s *RevocationServiceTestSuite) TestRevokeToken_RevokesTokenFamily() {
	token := buildToken(map[string]interface{}{
		"jti":       "jti-fam",
		"client_id": testClientID,
		"tfid":      "tfid-77",
		"exp":       float64(time.Now().Add(time.Hour).Unix()),
	})
	s.jwtServiceMock.On("VerifyJWTSignature", mock.Anything, token).Return(nil)
	s.storeMock.On("InsertRevokedToken", mock.Anything, mock.Anything).Return(nil)
	s.storeMock.On("insertCriterion", mock.Anything, mock.MatchedBy(func(c revocationCriterion) bool {
		return c.Type == CriterionTypeTokenFamily && c.Value == "tfid-77" &&
			c.Reason == RevocationReasonExplicitTokenFamily
	})).Return(nil)
	s.obsMock.On("IsEnabled").Return(false)

	revokeOutcome, err := s.service.RevokeToken(context.Background(), token, "", testClientID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), RevokeOutcomeRevoked, revokeOutcome)
}

func (s *RevocationServiceTestSuite) TestRevokeToken_PublishesAuditEvent() {
	token := buildToken(map[string]interface{}{"jti": "jti-evt", "client_id": testClientID})
	s.jwtServiceMock.On("VerifyJWTSignature", mock.Anything, token).Return(nil)
	s.storeMock.On("InsertRevokedToken", mock.Anything, mock.Anything).Return(nil)
	s.obsMock.On("IsEnabled").Return(true)
	s.obsMock.On("PublishEvent", mock.Anything, mock.Anything).Return()

	revokeOutcome, err := s.service.RevokeToken(context.Background(), token, "refresh_token", testClientID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), RevokeOutcomeRevoked, revokeOutcome)
}

func (s *RevocationServiceTestSuite) TestRevokeToken_InvalidSignatureIsNoOp() {
	token := buildToken(map[string]interface{}{"jti": "jti-123", "client_id": testClientID})
	s.jwtServiceMock.On("VerifyJWTSignature", mock.Anything, token).Return(&serviceerror.ServiceError{
		Type: serviceerror.ServerErrorType, Code: "INVALID_SIGNATURE",
	})

	revokeOutcome, err := s.service.RevokeToken(context.Background(), token, "", testClientID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), RevokeOutcomeRevoked, revokeOutcome)
	s.storeMock.AssertNotCalled(s.T(), "InsertRevokedToken", mock.Anything, mock.Anything)
}

func (s *RevocationServiceTestSuite) TestRevokeToken_ExpiredTokenStillRevocable() {
	token := buildToken(map[string]interface{}{
		"jti":       "jti-expired",
		"client_id": testClientID,
		"exp":       float64(time.Now().Add(-time.Hour).Unix()),
	})
	s.jwtServiceMock.On("VerifyJWTSignature", mock.Anything, token).Return(nil)
	s.storeMock.On("InsertRevokedToken", mock.Anything, mock.Anything).Return(nil)
	s.obsMock.On("IsEnabled").Return(false)

	revokeOutcome, err := s.service.RevokeToken(context.Background(), token, "", testClientID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), RevokeOutcomeRevoked, revokeOutcome)
}

func (s *RevocationServiceTestSuite) TestRevokeToken_NotOwnedByClient() {
	token := buildToken(map[string]interface{}{"jti": "jti-123", "client_id": "another-client"})
	s.jwtServiceMock.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	revokeOutcome, err := s.service.RevokeToken(context.Background(), token, "", testClientID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), RevokeOutcomeNotOwned, revokeOutcome)
	s.storeMock.AssertNotCalled(s.T(), "InsertRevokedToken", mock.Anything, mock.Anything)
}

// A refresh token carries no client_id claim (its owning client is the subject), so ownership must be
// enforced via sub: a refresh token presented by a different client is rejected per RFC 7009 §2.1.
func (s *RevocationServiceTestSuite) TestRevokeToken_RefreshTokenNotOwnedByClient() {
	token := buildToken(map[string]interface{}{"jti": "rt-jti", "sub": "another-client"})
	s.jwtServiceMock.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	revokeOutcome, err := s.service.RevokeToken(context.Background(), token, "", testClientID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), RevokeOutcomeNotOwned, revokeOutcome)
	s.storeMock.AssertNotCalled(s.T(), "InsertRevokedToken", mock.Anything, mock.Anything)
}

// A refresh token whose subject is the authenticated client is owned by it and revoked.
func (s *RevocationServiceTestSuite) TestRevokeToken_RefreshTokenOwnedBySubjectSucceeds() {
	token := buildToken(map[string]interface{}{"jti": "rt-jti", "sub": testClientID})
	s.jwtServiceMock.On("VerifyJWTSignature", mock.Anything, token).Return(nil)
	s.storeMock.On("InsertRevokedToken", mock.Anything, mock.Anything).Return(nil)
	s.obsMock.On("IsEnabled").Return(false)

	revokeOutcome, err := s.service.RevokeToken(context.Background(), token, "", testClientID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), RevokeOutcomeRevoked, revokeOutcome)
}

func (s *RevocationServiceTestSuite) TestRevokeToken_NoJtiIsNoOp() {
	token := buildToken(map[string]interface{}{"client_id": testClientID})
	s.jwtServiceMock.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	revokeOutcome, err := s.service.RevokeToken(context.Background(), token, "", testClientID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), RevokeOutcomeRevoked, revokeOutcome)
	s.storeMock.AssertNotCalled(s.T(), "InsertRevokedToken", mock.Anything, mock.Anything)
}

func (s *RevocationServiceTestSuite) TestRevokeToken_StoreErrorReturnsError() {
	token := buildToken(map[string]interface{}{"jti": "jti-123", "client_id": testClientID})
	s.jwtServiceMock.On("VerifyJWTSignature", mock.Anything, token).Return(nil)
	s.storeMock.On("InsertRevokedToken", mock.Anything, mock.Anything).Return(errors.New("db error"))

	revokeOutcome, err := s.service.RevokeToken(context.Background(), token, "", testClientID)
	assert.Error(s.T(), err)
	assert.Equal(s.T(), RevokeOutcomeRevoked, revokeOutcome)
	assert.Contains(s.T(), err.Error(), "failed to record token revocation")
}

func (s *RevocationServiceTestSuite) TestRevokeRefreshToken_RecordsWithRotationReason() {
	revoker := s.service.(RefreshTokenRevokerInterface)
	expiry := time.Now().Add(time.Hour).UTC()
	s.storeMock.On("InsertRevokedToken", mock.Anything, mock.MatchedBy(func(rt RevokedToken) bool {
		return rt.JTI == "rotated-jti" &&
			rt.RevocationReason == RevocationReasonRefreshRotation &&
			rt.ExpiryTime.Equal(expiry)
	})).Return(nil)

	err := revoker.RevokeRefreshToken(context.Background(), "rotated-jti", expiry)
	assert.NoError(s.T(), err)
}

func (s *RevocationServiceTestSuite) TestRevokeRefreshToken_EmptyJTIIsNoOp() {
	revoker := s.service.(RefreshTokenRevokerInterface)

	err := revoker.RevokeRefreshToken(context.Background(), "", time.Now().UTC())
	assert.NoError(s.T(), err)
	s.storeMock.AssertNotCalled(s.T(), "InsertRevokedToken", mock.Anything, mock.Anything)
}

func (s *RevocationServiceTestSuite) TestRevokeRefreshToken_StoreErrorPropagates() {
	revoker := s.service.(RefreshTokenRevokerInterface)
	s.storeMock.On("InsertRevokedToken", mock.Anything, mock.Anything).
		Return(errors.New("runtime persistent database unavailable"))

	err := revoker.RevokeRefreshToken(context.Background(), "jti-x", time.Now().UTC())
	assert.Error(s.T(), err)
}

func TestRevokeTokenFamily_WritesTokenFamilyCriterion(t *testing.T) {
	store := newRevocationStoreInterfaceMock(t)
	var captured revocationCriterion
	store.On("insertCriterion", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			captured = args.Get(1).(revocationCriterion)
		}).
		Return(nil)

	revoker := newRevocationService(nil, store, time.Hour, false, nil)
	err := revoker.RevokeTokenFamily(context.Background(), "tfid-abc", RevocationReasonSessionLogout)

	assert.NoError(t, err)
	assert.Equal(t, CriterionTypeTokenFamily, captured.Type)
	assert.Equal(t, "tfid-abc", captured.Value)
	assert.Equal(t, RevocationReasonSessionLogout, captured.Reason)
	assert.WithinDuration(t, captured.RevokedAt.Add(time.Hour), captured.ExpiryTime, time.Second)
}

func TestRevokeTokenFamily_EmptyIDIsNoOp(t *testing.T) {
	store := newRevocationStoreInterfaceMock(t)
	// No insertCriterion expectation: an empty tfid must not write.
	revoker := newRevocationService(nil, store, time.Hour, false, nil)

	err := revoker.RevokeTokenFamily(context.Background(), "", RevocationReasonSessionLogout)
	assert.NoError(t, err)
	store.AssertNotCalled(t, "insertCriterion", mock.Anything, mock.Anything)
}

func TestRevokeByCriteria_UsesCutoffAsRevokedAt(t *testing.T) {
	store := newRevocationStoreInterfaceMock(t)
	cutoff := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	store.On("insertCriterion", mock.Anything, mock.MatchedBy(func(criterion revocationCriterion) bool {
		return criterion.Type == CriterionTypeApplicationID &&
			criterion.Value == "app-123" &&
			criterion.Reason == RevocationReasonApplicationSecretRegenerated &&
			criterion.RevokedAt.Equal(cutoff)
	})).Return(nil)

	revoker := newRevocationService(nil, store, time.Hour, false, nil)
	err := revoker.RevokeByCriteria(context.Background(), CriteriaRevocation{
		Criterion: Criterion{Type: CriterionTypeApplicationID, Value: "app-123"},
		Mode:      RevocationModeBeforeAction,
		Cutoff:    cutoff,
		Reason:    RevocationReasonApplicationSecretRegenerated,
	})

	assert.NoError(t, err)
}

func TestRevokeTokenFamily_PropagatesStoreError(t *testing.T) {
	store := newRevocationStoreInterfaceMock(t)
	store.On("insertCriterion", mock.Anything, mock.Anything).Return(errors.New("db down"))

	revoker := newRevocationService(nil, store, time.Hour, false, nil)
	err := revoker.RevokeTokenFamily(context.Background(), "tfid-abc", RevocationReasonRefreshReplay)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "db down")
}

func TestRevokeTokenFamily_NonPositiveTTLFallsBack(t *testing.T) {
	store := newRevocationStoreInterfaceMock(t)
	var captured revocationCriterion
	store.On("insertCriterion", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			captured = args.Get(1).(revocationCriterion)
		}).
		Return(nil)

	revoker := newRevocationService(nil, store, 0, false, nil)
	err := revoker.RevokeTokenFamily(context.Background(), "tfid-abc", RevocationReasonCodeReplay)
	assert.NoError(t, err)
	assert.WithinDuration(t, captured.RevokedAt.Add(defaultTokenFamilyRevocationTTL), captured.ExpiryTime, time.Second)
}

// A caller that knows its artifacts outlive the configured lifetime can ask for a longer row, which is
// what an application-scoped revocation needs: token validity is per application and uncapped, so the
// deployment-wide default can expire the row while matching tokens are still valid.
func TestRevokeByCriteria_RequestedTTLExtendsTheRow(t *testing.T) {
	store := newRevocationStoreInterfaceMock(t)
	requested := 30 * 24 * time.Hour
	before := time.Now().UTC()
	store.On("insertCriterion", mock.Anything, mock.MatchedBy(func(criterion revocationCriterion) bool {
		return criterion.ExpiryTime.After(before.Add(requested-time.Minute)) &&
			criterion.ExpiryTime.Before(before.Add(requested+time.Minute))
	})).Return(nil)

	revoker := newRevocationService(nil, store, time.Hour, false, nil)
	err := revoker.RevokeByCriteria(context.Background(), CriteriaRevocation{
		Criterion: Criterion{Type: CriterionTypeApplicationKey, Value: "client-long-lived"},
		Mode:      RevocationModeAll,
		Reason:    RevocationReasonApplicationDeleted,
		TTL:       requested,
	})

	assert.NoError(t, err)
}

// The requested lifetime raises the row and never lowers it, so a caller cannot shorten a row below
// what the deployment already guarantees.
func TestRevokeByCriteria_ShorterRequestedTTLKeepsTheConfiguredLifetime(t *testing.T) {
	store := newRevocationStoreInterfaceMock(t)
	configured := 24 * time.Hour
	before := time.Now().UTC()
	store.On("insertCriterion", mock.Anything, mock.MatchedBy(func(criterion revocationCriterion) bool {
		return criterion.ExpiryTime.After(before.Add(configured - time.Minute))
	})).Return(nil)

	revoker := newRevocationService(nil, store, configured, false, nil)
	err := revoker.RevokeByCriteria(context.Background(), CriteriaRevocation{
		Criterion: Criterion{Type: CriterionTypeApplicationKey, Value: "client-short-lived"},
		Mode:      RevocationModeAll,
		Reason:    RevocationReasonApplicationDeleted,
		TTL:       time.Minute,
	})

	assert.NoError(t, err)
}

// An unstated lifetime keeps the configured default, so existing writers are unaffected.
func TestRevokeByCriteria_ZeroTTLUsesTheConfiguredLifetime(t *testing.T) {
	store := newRevocationStoreInterfaceMock(t)
	configured := 2 * time.Hour
	before := time.Now().UTC()
	store.On("insertCriterion", mock.Anything, mock.MatchedBy(func(criterion revocationCriterion) bool {
		return criterion.ExpiryTime.After(before.Add(configured-time.Minute)) &&
			criterion.ExpiryTime.Before(before.Add(configured+time.Minute))
	})).Return(nil)

	revoker := newRevocationService(nil, store, configured, false, nil)
	err := revoker.RevokeByCriteria(context.Background(), CriteriaRevocation{
		Criterion: Criterion{Type: CriterionTypeSubject, Value: "user-1"},
		Mode:      RevocationModeAll,
		Reason:    RevocationReasonUserDeleted,
		TTL:       0,
	})

	assert.NoError(t, err)
}
