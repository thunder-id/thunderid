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

	sharedrevocation "github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	serviceerror "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
	"github.com/thunder-id/thunderid/tests/mocks/observability/observabilitymock"
)

const (
	testClientID            = "test-client-id"
	testMaxEntriesPerChange = 100
)

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
	s.service = newRevocationService(s.jwtServiceMock, s.storeMock, time.Hour, 0, true, s.obsMock,
		testMaxEntriesPerChange)
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

	revoker := newRevocationService(nil, store, time.Hour, 0, false, nil, testMaxEntriesPerChange)
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
	revoker := newRevocationService(nil, store, time.Hour, 0, false, nil, testMaxEntriesPerChange)

	err := revoker.RevokeTokenFamily(context.Background(), "", RevocationReasonSessionLogout)
	assert.NoError(t, err)
	store.AssertNotCalled(t, "insertCriterion", mock.Anything, mock.Anything)
}

func (s *RevocationServiceTestSuite) TestRevokeCriteriaBatch_UsesCutoffAsRevokedAt() {
	store := newRevocationStoreInterfaceMock(s.T())
	cutoff := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	store.On("insertCriteria", mock.Anything, mock.MatchedBy(func(rows []revocationCriterion) bool {
		return len(rows) == 1 && rows[0].Type == CriterionTypeApplicationID &&
			rows[0].Value == "app-123" &&
			rows[0].Reason == RevocationReasonApplicationSecretRegenerated &&
			rows[0].RevokedAt.Equal(cutoff)
	})).Return(nil)

	revoker := newRevocationService(nil, store, time.Hour, 0, false, nil, testMaxEntriesPerChange)
	svcErr := revoker.RevokeCriteriaBatch(context.Background(), []CriteriaRevocation{{
		Criterion: Criterion{Type: CriterionTypeApplicationID, Value: "app-123"},
		Mode:      RevocationModeBeforeAction,
		Cutoff:    cutoff,
		Reason:    RevocationReasonApplicationSecretRegenerated,
	}})

	s.Nil(svcErr)
}

func TestRevokeTokenFamily_PropagatesStoreError(t *testing.T) {
	store := newRevocationStoreInterfaceMock(t)
	store.On("insertCriterion", mock.Anything, mock.Anything).Return(errors.New("db down"))

	revoker := newRevocationService(nil, store, time.Hour, 0, false, nil, testMaxEntriesPerChange)
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

	revoker := newRevocationService(nil, store, 0, 0, false, nil, testMaxEntriesPerChange)
	err := revoker.RevokeTokenFamily(context.Background(), "tfid-abc", RevocationReasonCodeReplay)
	assert.NoError(t, err)
	assert.WithinDuration(t, captured.RevokedAt.Add(defaultTokenFamilyRevocationTTL), captured.ExpiryTime, time.Second)
}

func (s *RevocationServiceTestSuite) TestRevokeCriteriaBatch_RequestedTTLExtendsTheRow() {
	store := newRevocationStoreInterfaceMock(s.T())
	requested := 30 * 24 * time.Hour
	before := time.Now().UTC()
	store.On("insertCriteria", mock.Anything, mock.MatchedBy(func(rows []revocationCriterion) bool {
		return len(rows) == 1 && rows[0].ExpiryTime.After(before.Add(requested-time.Minute)) &&
			rows[0].ExpiryTime.Before(before.Add(requested+time.Minute))
	})).Return(nil)

	revoker := newRevocationService(nil, store, time.Hour, 0, false, nil, testMaxEntriesPerChange)
	svcErr := revoker.RevokeCriteriaBatch(context.Background(), []CriteriaRevocation{{
		Criterion: Criterion{Type: CriterionTypeApplicationKey, Value: "client-long-lived"},
		Mode:      RevocationModeAll,
		Reason:    RevocationReasonApplicationDeleted,
		TTL:       requested,
	}})

	s.Nil(svcErr)
}

func (s *RevocationServiceTestSuite) TestRevokeCriteriaBatch_ShorterRequestedTTLKeepsTheConfiguredLifetime() {
	store := newRevocationStoreInterfaceMock(s.T())
	configured := 24 * time.Hour
	before := time.Now().UTC()
	store.On("insertCriteria", mock.Anything, mock.MatchedBy(func(rows []revocationCriterion) bool {
		return len(rows) == 1 && rows[0].ExpiryTime.After(before.Add(configured-time.Minute))
	})).Return(nil)

	revoker := newRevocationService(nil, store, configured, 0, false, nil, testMaxEntriesPerChange)
	svcErr := revoker.RevokeCriteriaBatch(context.Background(), []CriteriaRevocation{{
		Criterion: Criterion{Type: CriterionTypeApplicationKey, Value: "client-short-lived"},
		Mode:      RevocationModeAll,
		Reason:    RevocationReasonApplicationDeleted,
		TTL:       time.Minute,
	}})

	s.Nil(svcErr)
}

func (s *RevocationServiceTestSuite) TestRevokeCriteriaBatch_ZeroTTLUsesTheConfiguredLifetime() {
	store := newRevocationStoreInterfaceMock(s.T())
	configured := 2 * time.Hour
	before := time.Now().UTC()
	store.On("insertCriteria", mock.Anything, mock.MatchedBy(func(rows []revocationCriterion) bool {
		return len(rows) == 1 && rows[0].ExpiryTime.After(before.Add(configured-time.Minute)) &&
			rows[0].ExpiryTime.Before(before.Add(configured+time.Minute))
	})).Return(nil)

	revoker := newRevocationService(nil, store, configured, 0, false, nil, testMaxEntriesPerChange)
	svcErr := revoker.RevokeCriteriaBatch(context.Background(), []CriteriaRevocation{{
		Criterion: Criterion{Type: CriterionTypeSubject, Value: "user-1"},
		Mode:      RevocationModeAll,
		Reason:    RevocationReasonUserDeleted,
		TTL:       0,
	}})

	s.Nil(svcErr)
}

func (s *RevocationServiceTestSuite) TestRevokeCriteriaBatch_EmptySliceSkipsTheStore() {
	store := newRevocationStoreInterfaceMock(s.T())

	revoker := newRevocationService(nil, store, time.Hour, 0, false, nil, testMaxEntriesPerChange)
	svcErr := revoker.RevokeCriteriaBatch(context.Background(), nil)

	s.Nil(svcErr)
	store.AssertNotCalled(s.T(), "insertCriteria", mock.Anything, mock.Anything)
}

func (s *RevocationServiceTestSuite) TestRevokeCriteriaBatch_EmptyValuesAreFilteredNotWritten() {
	store := newRevocationStoreInterfaceMock(s.T())

	revoker := newRevocationService(nil, store, time.Hour, 0, false, nil, testMaxEntriesPerChange)
	svcErr := revoker.RevokeCriteriaBatch(context.Background(), []CriteriaRevocation{
		{Criterion: Criterion{Type: CriterionTypeSubject, Value: ""}, Mode: RevocationModeAll,
			Reason: RevocationReasonUserDeleted},
	})

	s.Nil(svcErr)
	store.AssertNotCalled(s.T(), "insertCriteria", mock.Anything, mock.Anything)
}

func (s *RevocationServiceTestSuite) TestRevokeCriteriaBatch_OneInvalidEntryWritesNothing() {
	store := newRevocationStoreInterfaceMock(s.T())

	revoker := newRevocationService(nil, store, time.Hour, 0, false, nil, testMaxEntriesPerChange)
	svcErr := revoker.RevokeCriteriaBatch(context.Background(), []CriteriaRevocation{
		{Criterion: Criterion{Type: CriterionTypeEntityScope, Value: "digest-1"},
			Mode: RevocationModeBeforeAction, Cutoff: time.Now().UTC(),
			Reason: RevocationReasonRoleAssignmentRemoved},
		{Criterion: Criterion{Type: CriterionTypeEntityScope, Value: "digest-2"},
			Mode: RevocationModeBeforeAction, Reason: RevocationReasonRoleAssignmentRemoved},
	})

	s.Require().NotNil(svcErr)
	s.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
	store.AssertNotCalled(s.T(), "insertCriteria", mock.Anything, mock.Anything)
}

func (s *RevocationServiceTestSuite) TestRevokeCriteriaBatch_WritesEveryEntryInOneCall() {
	store := newRevocationStoreInterfaceMock(s.T())
	before := time.Now().UTC()
	boundaryCutoff := before.Add(-time.Minute).Truncate(time.Second)
	store.On("insertCriteria", mock.Anything, mock.MatchedBy(func(rows []revocationCriterion) bool {
		if len(rows) != 2 {
			return false
		}
		// A terminal entry's RevokedAt is the time of the call.
		return rows[0].Type == CriterionTypeApplicationKey && rows[0].Value == "client-1" &&
			!rows[0].RevokedAt.Before(before) &&
			rows[1].Type == CriterionTypeEntityScope && rows[1].Value == "digest-1" &&
			rows[1].RevokedAt.Equal(boundaryCutoff)
	})).Return(nil)

	revoker := newRevocationService(nil, store, time.Hour, 0, false, nil, testMaxEntriesPerChange)
	svcErr := revoker.RevokeCriteriaBatch(context.Background(), []CriteriaRevocation{
		{Criterion: Criterion{Type: CriterionTypeApplicationKey, Value: "client-1"},
			Mode: RevocationModeAll, Reason: RevocationReasonApplicationDeleted},
		{Criterion: Criterion{Type: CriterionTypeEntityScope, Value: "digest-1"},
			Mode: RevocationModeBeforeAction, Cutoff: boundaryCutoff,
			Reason: RevocationReasonRoleAssignmentRemoved},
	})

	s.Nil(svcErr)
}

func (s *RevocationServiceTestSuite) TestRevokeCriteriaBatch_PropagatesStoreError() {
	store := newRevocationStoreInterfaceMock(s.T())
	store.On("insertCriteria", mock.Anything, mock.Anything).Return(errors.New("db down"))

	revoker := newRevocationService(nil, store, time.Hour, 0, false, nil, testMaxEntriesPerChange)
	svcErr := revoker.RevokeCriteriaBatch(context.Background(), []CriteriaRevocation{
		{Criterion: Criterion{Type: CriterionTypeSubject, Value: "user-1"}, Mode: RevocationModeAll,
			Reason: RevocationReasonUserDeleted},
	})

	s.Require().NotNil(svcErr)
	s.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
}

func (s *RevocationServiceTestSuite) TestRevokeCriteriaBatch_OverTheCapWritesNothing() {
	store := newRevocationStoreInterfaceMock(s.T())
	cutoff := time.Now().UTC()

	revoker := newRevocationService(nil, store, time.Hour, 0, false, nil, 1)
	svcErr := revoker.RevokeCriteriaBatch(context.Background(), []CriteriaRevocation{
		{Criterion: Criterion{Type: CriterionTypeEntityScope, Value: "digest-1"},
			Mode: RevocationModeBeforeAction, Cutoff: cutoff, Reason: RevocationReasonRoleDeleted},
		{Criterion: Criterion{Type: CriterionTypeEntityScope, Value: "digest-2"},
			Mode: RevocationModeBeforeAction, Cutoff: cutoff, Reason: RevocationReasonRoleDeleted},
	})

	s.Require().NotNil(svcErr)
	s.Equal(serviceerror.ClientErrorType, svcErr.Type)
	s.Equal(sharedrevocation.ErrorEntryLimitExceeded.Code, svcErr.Code)
	s.Equal("The change would record 2 revocation entries, more than the 1 allowed for one change",
		svcErr.ErrorDescription.String())
	store.AssertNotCalled(s.T(), "insertCriteria", mock.Anything, mock.Anything)
}

func (s *RevocationServiceTestSuite) TestRevokeCriteriaBatch_ExactlyTheCapIsWritten() {
	store := newRevocationStoreInterfaceMock(s.T())
	store.On("insertCriteria", mock.Anything, mock.MatchedBy(func(rows []revocationCriterion) bool {
		return len(rows) == 2
	})).Return(nil)
	cutoff := time.Now().UTC()

	revoker := newRevocationService(nil, store, time.Hour, 0, false, nil, 2)
	svcErr := revoker.RevokeCriteriaBatch(context.Background(), []CriteriaRevocation{
		{Criterion: Criterion{Type: CriterionTypeEntityScope, Value: "digest-1"},
			Mode: RevocationModeBeforeAction, Cutoff: cutoff, Reason: RevocationReasonRoleDeleted},
		{Criterion: Criterion{Type: CriterionTypeEntityScope, Value: "digest-2"},
			Mode: RevocationModeBeforeAction, Cutoff: cutoff, Reason: RevocationReasonRoleDeleted},
	})

	s.Nil(svcErr)
}

func (s *RevocationServiceTestSuite) TestRevokeCriteriaBatch_ZeroCapRefusesEveryRevocation() {
	store := newRevocationStoreInterfaceMock(s.T())

	revoker := newRevocationService(nil, store, time.Hour, 0, false, nil, 0)
	svcErr := revoker.RevokeCriteriaBatch(context.Background(), []CriteriaRevocation{
		{Criterion: Criterion{Type: CriterionTypeSubject, Value: "user-1"}, Mode: RevocationModeAll,
			Reason: RevocationReasonUserDeleted},
	})

	s.Require().NotNil(svcErr)
	s.Equal(sharedrevocation.ErrorEntryLimitExceeded.Code, svcErr.Code)
	store.AssertNotCalled(s.T(), "insertCriteria", mock.Anything, mock.Anything)
}

func (s *RevocationServiceTestSuite) TestRevokeCriteriaBatch_RepeatedCriterionIsWrittenOnceAndCountedOnce() {
	store := newRevocationStoreInterfaceMock(s.T())
	store.On("insertCriteria", mock.Anything, mock.MatchedBy(func(rows []revocationCriterion) bool {
		return len(rows) == 1 && rows[0].Value == "digest-1"
	})).Return(nil).Once()
	cutoff := time.Now().UTC()
	repeated := CriteriaRevocation{Criterion: Criterion{Type: CriterionTypeEntityScope, Value: "digest-1"},
		Mode: RevocationModeBeforeAction, Cutoff: cutoff, Reason: RevocationReasonRolePermissionRemoved}

	revoker := newRevocationService(nil, store, time.Hour, 0, false, nil, 1)
	svcErr := revoker.RevokeCriteriaBatch(context.Background(), []CriteriaRevocation{repeated, repeated})

	s.Nil(svcErr)
}

func (s *RevocationServiceTestSuite) TestRevokeCriteriaBatch_DifferingRepeatsAreWrittenOnceAndCountedOnce() {
	cutoff := time.Now().UTC()
	boundary := CriteriaRevocation{Criterion: Criterion{Type: CriterionTypeEntityScope, Value: "digest-1"},
		Mode: RevocationModeBeforeAction, Cutoff: cutoff, Reason: RevocationReasonRolePermissionRemoved}
	later := boundary
	later.Cutoff = cutoff.Add(time.Minute)
	store := newRevocationStoreInterfaceMock(s.T())
	store.On("insertCriteria", mock.Anything, mock.MatchedBy(func(rows []revocationCriterion) bool {
		return len(rows) == 1 && rows[0].Value == boundary.Criterion.Value && rows[0].RevokedAt.Equal(cutoff)
	})).Return(nil).Once()

	revoker := newRevocationService(nil, store, time.Hour, 0, false, nil, 1)
	svcErr := revoker.RevokeCriteriaBatch(context.Background(), []CriteriaRevocation{boundary, later})

	s.Nil(svcErr)
}

// A boundary row lives until the last token it covers has expired and its leeway has passed: that
// token was issued at the cutoff, which a suspension sets ahead of the write.
func (s *RevocationServiceTestSuite) TestRevokeCriteriaBatch_ExpiryCountsFromACutoffAheadAndAddsTheLeeway() {
	store := newRevocationStoreInterfaceMock(s.T())
	configured := time.Hour
	leeway := 30 * time.Second
	cutoff := time.Now().UTC().Add(leeway)
	store.On("insertCriteria", mock.Anything, mock.MatchedBy(func(rows []revocationCriterion) bool {
		return len(rows) == 1 && rows[0].RevokedAt.Equal(cutoff) &&
			rows[0].ExpiryTime.Equal(cutoff.Add(configured+leeway))
	})).Return(nil)

	revoker := newRevocationService(nil, store, configured, leeway, false, nil, testMaxEntriesPerChange)
	svcErr := revoker.RevokeCriteriaBatch(context.Background(), []CriteriaRevocation{{
		Criterion: Criterion{Type: CriterionTypeSubject, Value: "user-1"},
		Mode:      RevocationModeBeforeAction,
		Cutoff:    cutoff,
		Reason:    RevocationReasonIdentitySuspended,
	}})

	s.Nil(svcErr)
}

// A criteria revocation that cannot be stored is published for an operator to alert on, naming the
// reason and the target.
func (s *RevocationServiceTestSuite) TestRevokeCriteriaBatch_StoreFailureIsPublished() {
	for _, tc := range []struct {
		criterion Criterion
		dataKey   string
	}{
		{Criterion{Type: CriterionTypeSubject, Value: "user-1"}, event.DataKey.Subject},
		{Criterion{Type: CriterionTypeApplicationKey, Value: "client-1"}, event.DataKey.ClientID},
	} {
		s.Run(string(tc.criterion.Type), func() {
			store := newRevocationStoreInterfaceMock(s.T())
			store.On("insertCriteria", mock.Anything, mock.Anything).Return(errors.New("db unavailable"))
			obsMock := observabilitymock.NewObservabilityServiceInterfaceMock(s.T())
			obsMock.On("IsEnabled").Return(true)
			var published *providers.Event
			obsMock.On("PublishEvent", mock.Anything, mock.Anything).
				Run(func(args mock.Arguments) { published = args.Get(1).(*providers.Event) }).Once()

			revoker := newRevocationService(nil, store, time.Hour, 0, false, obsMock, testMaxEntriesPerChange)
			svcErr := revoker.RevokeCriteriaBatch(context.Background(), []CriteriaRevocation{{
				Criterion: tc.criterion,
				Mode:      RevocationModeBeforeAction,
				Cutoff:    time.Now().UTC(),
				Reason:    RevocationReasonIdentitySuspended,
			}})

			s.Require().NotNil(svcErr)
			s.Equal(serviceerror.ServerErrorType, svcErr.Type)
			s.Require().NotNil(published)
			s.Equal(string(event.EventTypeTokenRevocationFailed), published.Type)
			s.Equal(providers.StatusFailure, published.Status)
			s.Equal(string(RevocationReasonIdentitySuspended), published.Data[event.DataKey.RevocationReason])
			s.Equal(string(tc.criterion.Type), published.Data[event.DataKey.CriterionType])
			s.Equal(tc.criterion.Value, published.Data[tc.dataKey])
		})
	}
}

// A terminal row counts from the write, plus the leeway.
func (s *RevocationServiceTestSuite) TestRevokeCriteriaBatch_TerminalExpiryAddsTheLeeway() {
	store := newRevocationStoreInterfaceMock(s.T())
	configured := time.Hour
	leeway := 30 * time.Second
	before := time.Now().UTC()
	store.On("insertCriteria", mock.Anything, mock.MatchedBy(func(rows []revocationCriterion) bool {
		return len(rows) == 1 && !rows[0].ExpiryTime.Before(before.Add(configured+leeway)) &&
			rows[0].ExpiryTime.Before(before.Add(configured+leeway+time.Minute))
	})).Return(nil)

	revoker := newRevocationService(nil, store, configured, leeway, false, nil, testMaxEntriesPerChange)
	svcErr := revoker.RevokeCriteriaBatch(context.Background(), []CriteriaRevocation{{
		Criterion: Criterion{Type: CriterionTypeSubject, Value: "user-1"},
		Mode:      RevocationModeAll,
		Reason:    RevocationReasonUserDeleted,
	}})

	s.Nil(svcErr)
}

// A token is accepted until exp plus the leeway, so its row lives that long too.
func (s *RevocationServiceTestSuite) TestRevokeToken_ExpiryAddsTheLeeway() {
	leeway := 30 * time.Second
	exp := time.Now().Add(time.Hour).Truncate(time.Second).UTC()
	token := buildToken(map[string]interface{}{"jti": "jti-leeway", "client_id": testClientID,
		"exp": float64(exp.Unix())})
	s.jwtServiceMock.On("VerifyJWTSignature", mock.Anything, token).Return(nil)
	s.storeMock.On("InsertRevokedToken", mock.Anything, mock.MatchedBy(func(rt RevokedToken) bool {
		return rt.JTI == "jti-leeway" && rt.ExpiryTime.Equal(exp.Add(leeway))
	})).Return(nil)
	s.obsMock.On("IsEnabled").Return(false)

	revoker := newRevocationService(s.jwtServiceMock, s.storeMock, time.Hour, leeway, false, s.obsMock,
		testMaxEntriesPerChange)
	revokeOutcome, err := revoker.RevokeToken(context.Background(), token, "", testClientID)

	s.NoError(err)
	s.Equal(RevokeOutcomeRevoked, revokeOutcome)
}

func (s *RevocationServiceTestSuite) TestRevokeRefreshToken_ExpiryAddsTheLeeway() {
	leeway := 30 * time.Second
	expiry := time.Now().Add(time.Hour).UTC()
	s.storeMock.On("InsertRevokedToken", mock.Anything, mock.MatchedBy(func(rt RevokedToken) bool {
		return rt.JTI == "rotated-jti" && rt.ExpiryTime.Equal(expiry.Add(leeway))
	})).Return(nil)

	revoker := newRevocationService(nil, s.storeMock, time.Hour, leeway, false, nil, testMaxEntriesPerChange)
	err := revoker.(RefreshTokenRevokerInterface).RevokeRefreshToken(context.Background(), "rotated-jti", expiry)

	s.NoError(err)
}

// A token family row lives the configured lifetime plus the leeway.
func (s *RevocationServiceTestSuite) TestRevokeTokenFamily_ExpiryAddsTheLeeway() {
	leeway := 30 * time.Second
	var captured revocationCriterion
	s.storeMock.On("insertCriterion", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { captured = args.Get(1).(revocationCriterion) }).Return(nil)

	revoker := newRevocationService(nil, s.storeMock, time.Hour, leeway, false, nil, testMaxEntriesPerChange)
	err := revoker.RevokeTokenFamily(context.Background(), "tfid-1", RevocationReasonExplicitTokenFamily)

	s.Require().NoError(err)
	s.WithinDuration(captured.RevokedAt.Add(time.Hour+leeway), captured.ExpiryTime, time.Second)
}

func (s *RevocationServiceTestSuite) TestRevokeTokenFamily_StoreFailureIsPublished() {
	s.storeMock.On("insertCriterion", mock.Anything, mock.Anything).Return(errors.New("db unavailable"))
	s.obsMock.On("IsEnabled").Return(true)
	var published *providers.Event
	s.obsMock.On("PublishEvent", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { published = args.Get(1).(*providers.Event) }).Once()

	err := s.service.RevokeTokenFamily(context.Background(), "tfid-1", RevocationReasonExplicitTokenFamily)

	s.Require().Error(err)
	s.Require().NotNil(published)
	s.Equal(string(event.EventTypeTokenRevocationFailed), published.Type)
	s.Equal(providers.StatusFailure, published.Status)
	s.Equal(string(CriterionTypeTokenFamily), published.Data[event.DataKey.CriterionType])
	s.Equal(string(RevocationReasonExplicitTokenFamily), published.Data[event.DataKey.RevocationReason])
}
