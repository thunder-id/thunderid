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
	s.service = newRevocationService(s.jwtServiceMock, s.storeMock, time.Hour, 0, true, true, s.obsMock,
		testMaxEntriesPerChange)
}

func (s *RevocationServiceTestSuite) SetupSubTest() {
	s.SetupTest()
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

	result, err := s.service.RevokeToken(context.Background(), token, "", testClientID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), RevokeOutcomeRevoked, result.Outcome)
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

	result, err := s.service.RevokeToken(context.Background(), token, "", testClientID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), RevokeOutcomeRevoked, result.Outcome)
}

func (s *RevocationServiceTestSuite) TestRevokeToken_PublishesAuditEvent() {
	token := buildToken(map[string]interface{}{"jti": "jti-evt", "client_id": testClientID})
	s.jwtServiceMock.On("VerifyJWTSignature", mock.Anything, token).Return(nil)
	s.storeMock.On("InsertRevokedToken", mock.Anything, mock.Anything).Return(nil)
	s.obsMock.On("IsEnabled").Return(true)
	s.obsMock.On("PublishEvent", mock.Anything, mock.Anything).Return()

	result, err := s.service.RevokeToken(context.Background(), token, "refresh_token", testClientID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), RevokeOutcomeRevoked, result.Outcome)
}

func (s *RevocationServiceTestSuite) TestRevokeToken_InvalidSignatureIsNoOp() {
	token := buildToken(map[string]interface{}{"jti": "jti-123", "client_id": testClientID})
	s.jwtServiceMock.On("VerifyJWTSignature", mock.Anything, token).Return(&serviceerror.ServiceError{
		Type: serviceerror.ServerErrorType, Code: "INVALID_SIGNATURE",
	})

	result, err := s.service.RevokeToken(context.Background(), token, "", testClientID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), RevokeOutcomeRevoked, result.Outcome)
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

	result, err := s.service.RevokeToken(context.Background(), token, "", testClientID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), RevokeOutcomeRevoked, result.Outcome)
}

func (s *RevocationServiceTestSuite) TestRevokeToken_NotOwnedByClient() {
	token := buildToken(map[string]interface{}{"jti": "jti-123", "client_id": "another-client"})
	s.jwtServiceMock.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	result, err := s.service.RevokeToken(context.Background(), token, "", testClientID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), RevokeOutcomeNotOwned, result.Outcome)
	s.storeMock.AssertNotCalled(s.T(), "InsertRevokedToken", mock.Anything, mock.Anything)
}

// A refresh token carries no client_id claim (its owning client is the subject), so ownership must be
// enforced via sub: a refresh token presented by a different client is rejected per RFC 7009 §2.1.
func (s *RevocationServiceTestSuite) TestRevokeToken_RefreshTokenNotOwnedByClient() {
	token := buildToken(map[string]interface{}{"jti": "rt-jti", "sub": "another-client"})
	s.jwtServiceMock.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	result, err := s.service.RevokeToken(context.Background(), token, "", testClientID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), RevokeOutcomeNotOwned, result.Outcome)
	s.storeMock.AssertNotCalled(s.T(), "InsertRevokedToken", mock.Anything, mock.Anything)
}

// A refresh token whose subject is the authenticated client is owned by it and revoked.
func (s *RevocationServiceTestSuite) TestRevokeToken_RefreshTokenOwnedBySubjectSucceeds() {
	token := buildToken(map[string]interface{}{"jti": "rt-jti", "sub": testClientID})
	s.jwtServiceMock.On("VerifyJWTSignature", mock.Anything, token).Return(nil)
	s.storeMock.On("InsertRevokedToken", mock.Anything, mock.Anything).Return(nil)
	s.obsMock.On("IsEnabled").Return(false)

	result, err := s.service.RevokeToken(context.Background(), token, "", testClientID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), RevokeOutcomeRevoked, result.Outcome)
}

func (s *RevocationServiceTestSuite) TestRevokeToken_NoJtiIsNoOp() {
	token := buildToken(map[string]interface{}{"client_id": testClientID})
	s.jwtServiceMock.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	result, err := s.service.RevokeToken(context.Background(), token, "", testClientID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), RevokeOutcomeRevoked, result.Outcome)
	s.storeMock.AssertNotCalled(s.T(), "InsertRevokedToken", mock.Anything, mock.Anything)
}

// A revoked token names its user only for a user access token, or a refresh token carrying its
// access_token_sub, that the actor provider resolves to a user; the claims are the shapes the token builder mints.
func (s *RevocationServiceTestSuite) TestRevokeToken_ReportsTheTokenSubject() {
	const (
		userID   = "01900000-0000-7000-8000-0000000000a1"
		entityID = "01900000-0000-7000-8000-0000000000a2"
	)
	exp := float64(time.Now().Add(time.Hour).Unix())
	user := &providers.Entity{ID: userID, Category: providers.EntityCategoryUser}
	cases := []struct {
		name     string
		claims   map[string]interface{}
		lookupID string
		entity   *providers.Entity
		lookErr  *serviceerror.ServiceError
		want     string
	}{
		{
			name:     "a user access token",
			claims:   map[string]interface{}{"sub": userID, "client_id": testClientID},
			lookupID: userID, entity: user, want: userID,
		},
		{
			name:     "a refresh token naming its user in access_token_sub",
			claims:   map[string]interface{}{"sub": testClientID, "access_token_sub": userID, "tfid": "tfid-1"},
			lookupID: userID, entity: user, want: userID,
		},
		{
			name:     "a client_credentials access token of an application",
			claims:   map[string]interface{}{"sub": entityID, "client_id": testClientID},
			lookupID: entityID, entity: &providers.Entity{ID: entityID, Category: providers.EntityCategoryApp},
		},
		{
			name:     "a client_credentials access token of an agent",
			claims:   map[string]interface{}{"sub": entityID, "client_id": testClientID},
			lookupID: entityID, entity: &providers.Entity{ID: entityID, Category: providers.EntityCategoryAgent},
		},
		{
			name:     "an access token of an external or mapped subject",
			claims:   map[string]interface{}{"sub": "alex@example.com", "client_id": testClientID},
			lookupID: "alex@example.com", lookErr: &serviceerror.ServiceError{Code: "ENTITY_NOT_FOUND"},
		},
		{
			name:     "an access token whose subject lookup fails",
			claims:   map[string]interface{}{"sub": userID, "client_id": testClientID},
			lookupID: userID, lookErr: &serviceerror.ServiceError{Code: "INTERNAL_SERVER_ERROR"},
		},
		{
			name:   "a jwt-bearer access token with an idp claim",
			claims: map[string]interface{}{"sub": userID, "client_id": testClientID, "idp": "https://idp.example"},
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			provider := newActorLookupMock(s.T())
			s.service.(*revocationService).actorProvider = provider
			claims := map[string]interface{}{"jti": "jti-subject", "exp": exp}
			for key, value := range tc.claims {
				claims[key] = value
			}
			token := buildToken(claims)
			s.jwtServiceMock.On("VerifyJWTSignature", mock.Anything, token).Return(nil).Once()
			s.storeMock.On("InsertRevokedToken", mock.Anything, mock.Anything).Return(nil).Once()
			s.storeMock.On("insertCriterion", mock.Anything, mock.Anything).Return(nil).Maybe()
			s.obsMock.On("IsEnabled").Return(false).Once()
			if tc.lookupID != "" {
				provider.EXPECT().GetActor(mock.Anything, tc.lookupID).Return(tc.entity, tc.lookErr).Once()
			}

			result, err := s.service.RevokeToken(context.Background(), token, "", testClientID)

			s.NoError(err)
			s.Equal(RevokeResult{Outcome: RevokeOutcomeRevoked, TokenSubject: tc.want}, result)
		})
	}
}

// Without an actor provider a user's access token revokes without naming the user.
func (s *RevocationServiceTestSuite) TestRevokeToken_WithoutActorProviderReportsNoSubject() {
	token := buildToken(map[string]interface{}{"jti": "jti-1", "sub": "user-1", "client_id": testClientID})
	s.jwtServiceMock.On("VerifyJWTSignature", mock.Anything, token).Return(nil)
	s.storeMock.On("InsertRevokedToken", mock.Anything, mock.Anything).Return(nil)
	s.obsMock.On("IsEnabled").Return(false)

	result, err := s.service.RevokeToken(context.Background(), token, "", testClientID)

	s.NoError(err)
	s.Equal(RevokeResult{Outcome: RevokeOutcomeRevoked}, result)
}

// A token of another client is not revoked and its user is never looked up or named.
func (s *RevocationServiceTestSuite) TestRevokeToken_NotOwnedNamesNoSubject() {
	s.service.(*revocationService).actorProvider = newActorLookupMock(s.T())
	token := buildToken(map[string]interface{}{"jti": "jti-1", "sub": "user-1", "client_id": "another-client"})
	s.jwtServiceMock.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	result, err := s.service.RevokeToken(context.Background(), token, "", testClientID)

	s.NoError(err)
	s.Equal(RevokeResult{Outcome: RevokeOutcomeNotOwned}, result)
}

func (s *RevocationServiceTestSuite) TestRevokeToken_StoreErrorReturnsError() {
	token := buildToken(map[string]interface{}{"jti": "jti-123", "client_id": testClientID})
	s.jwtServiceMock.On("VerifyJWTSignature", mock.Anything, token).Return(nil)
	s.storeMock.On("InsertRevokedToken", mock.Anything, mock.Anything).Return(errors.New("db error"))

	result, err := s.service.RevokeToken(context.Background(), token, "", testClientID)
	assert.Error(s.T(), err)
	assert.Equal(s.T(), RevokeResult{}, result)
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

	revoker := newRevocationService(nil, store, time.Hour, 0, false, true, nil, testMaxEntriesPerChange)
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
	revoker := newRevocationService(nil, store, time.Hour, 0, false, true, nil, testMaxEntriesPerChange)

	err := revoker.RevokeTokenFamily(context.Background(), "", RevocationReasonSessionLogout)
	assert.NoError(t, err)
	store.AssertNotCalled(t, "insertCriterion", mock.Anything, mock.Anything)
}

func TestIsTokenFamilyRevoked_ReadsTheDenyList(t *testing.T) {
	cases := []struct {
		name     string
		revoked  bool
		storeErr error
	}{
		{name: "revoked family", revoked: true},
		{name: "live family"},
		{name: "deny list unavailable", storeErr: errors.New("db down")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newRevocationStoreInterfaceMock(t)
			store.On("areCriteriaRevoked", mock.Anything,
				[]Criterion{{Type: CriterionTypeTokenFamily, Value: "tfid-1"}}, mock.Anything).
				Return(tc.revoked, tc.storeErr)
			revoker := newRevocationService(nil, store, time.Hour, 0, false, true, nil, testMaxEntriesPerChange)

			revoked, err := revoker.IsTokenFamilyRevoked(context.Background(), "tfid-1")

			assert.ErrorIs(t, err, tc.storeErr)
			assert.Equal(t, tc.revoked, revoked)
		})
	}
}

func TestIsTokenFamilyRevoked_SkipsTheDenyList(t *testing.T) {
	cases := []struct {
		name             string
		denyListEnforced bool
		tokenFamilyID    string
	}{
		{name: "token revocation disabled", tokenFamilyID: "tfid-1"},
		{name: "empty family", denyListEnforced: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// No areCriteriaRevoked expectation: the check must not read the deny list.
			store := newRevocationStoreInterfaceMock(t)
			revoker := newRevocationService(nil, store, time.Hour, 0, false, tc.denyListEnforced, nil,
				testMaxEntriesPerChange)

			revoked, err := revoker.IsTokenFamilyRevoked(context.Background(), tc.tokenFamilyID)

			assert.NoError(t, err)
			assert.False(t, revoked)
		})
	}
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

	revoker := newRevocationService(nil, store, time.Hour, 0, false, true, nil, testMaxEntriesPerChange)
	svcErr := revoker.RevokeCriteriaBatch(context.Background(), []CriteriaRevocation{{
		Criterion: Criterion{Type: CriterionTypeApplicationID, Value: "app-123"},
		Mode:      RevocationModeBeforeAction,
		Cutoff:    cutoff,
		Reason:    RevocationReasonApplicationSecretRegenerated,
	}})

	s.Nil(svcErr)
}

func (s *RevocationServiceTestSuite) TestRevokeCriteriaBatch_WritesTheOrganizationUnitScopedTypes() {
	types := []CriterionType{CriterionTypeSubjectOU, CriterionTypeApplicationKeyOU, CriterionTypeEntityScopeOU,
		CriterionTypeEntityScopeSubtreeOU}
	store := newRevocationStoreInterfaceMock(s.T())
	store.On("insertCriteria", mock.Anything, mock.MatchedBy(func(rows []revocationCriterion) bool {
		if len(rows) != len(types) {
			return false
		}
		for i, row := range rows {
			if row.Type != types[i] {
				return false
			}
		}
		return true
	})).Return(nil)
	revocations := make([]CriteriaRevocation, 0, len(types))
	for _, criterionType := range types {
		revocations = append(revocations, CriteriaRevocation{
			Criterion: Criterion{Type: criterionType, Value: "digest-" + string(criterionType)},
			Mode:      RevocationModeBeforeAction,
			Cutoff:    time.Now().UTC(),
			Reason:    RevocationReason("resource_unshared"),
		})
	}

	revoker := newRevocationService(nil, store, time.Hour, 0, false, true, nil, testMaxEntriesPerChange)

	s.Nil(revoker.RevokeCriteriaBatch(context.Background(), revocations))
}

func TestRevokeTokenFamily_PropagatesStoreError(t *testing.T) {
	store := newRevocationStoreInterfaceMock(t)
	store.On("insertCriterion", mock.Anything, mock.Anything).Return(errors.New("db down"))

	revoker := newRevocationService(nil, store, time.Hour, 0, false, true, nil, testMaxEntriesPerChange)
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

	revoker := newRevocationService(nil, store, 0, 0, false, true, nil, testMaxEntriesPerChange)
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

	revoker := newRevocationService(nil, store, time.Hour, 0, false, true, nil, testMaxEntriesPerChange)
	svcErr := revoker.RevokeCriteriaBatch(context.Background(), []CriteriaRevocation{{
		Criterion: Criterion{Type: CriterionTypeApplicationKey, Value: "client-long-lived"},
		Mode:      RevocationModeAll,
		Reason:    RevocationReasonUserDeleted,
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

	revoker := newRevocationService(nil, store, configured, 0, false, true, nil, testMaxEntriesPerChange)
	svcErr := revoker.RevokeCriteriaBatch(context.Background(), []CriteriaRevocation{{
		Criterion: Criterion{Type: CriterionTypeApplicationKey, Value: "client-short-lived"},
		Mode:      RevocationModeAll,
		Reason:    RevocationReasonUserDeleted,
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

	revoker := newRevocationService(nil, store, configured, 0, false, true, nil, testMaxEntriesPerChange)
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

	revoker := newRevocationService(nil, store, time.Hour, 0, false, true, nil, testMaxEntriesPerChange)
	svcErr := revoker.RevokeCriteriaBatch(context.Background(), nil)

	s.Nil(svcErr)
	store.AssertNotCalled(s.T(), "insertCriteria", mock.Anything, mock.Anything)
}

func (s *RevocationServiceTestSuite) TestRevokeCriteriaBatch_EmptyValuesAreFilteredNotWritten() {
	store := newRevocationStoreInterfaceMock(s.T())

	revoker := newRevocationService(nil, store, time.Hour, 0, false, true, nil, testMaxEntriesPerChange)
	svcErr := revoker.RevokeCriteriaBatch(context.Background(), []CriteriaRevocation{
		{Criterion: Criterion{Type: CriterionTypeSubject, Value: ""}, Mode: RevocationModeAll,
			Reason: RevocationReasonUserDeleted},
	})

	s.Nil(svcErr)
	store.AssertNotCalled(s.T(), "insertCriteria", mock.Anything, mock.Anything)
}

func (s *RevocationServiceTestSuite) TestRevokeCriteriaBatch_OneInvalidEntryWritesNothing() {
	store := newRevocationStoreInterfaceMock(s.T())

	revoker := newRevocationService(nil, store, time.Hour, 0, false, true, nil, testMaxEntriesPerChange)
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

	revoker := newRevocationService(nil, store, time.Hour, 0, false, true, nil, testMaxEntriesPerChange)
	svcErr := revoker.RevokeCriteriaBatch(context.Background(), []CriteriaRevocation{
		{Criterion: Criterion{Type: CriterionTypeApplicationKey, Value: "client-1"},
			Mode: RevocationModeAll, Reason: RevocationReasonUserDeleted},
		{Criterion: Criterion{Type: CriterionTypeEntityScope, Value: "digest-1"},
			Mode: RevocationModeBeforeAction, Cutoff: boundaryCutoff,
			Reason: RevocationReasonRoleAssignmentRemoved},
	})

	s.Nil(svcErr)
}

func (s *RevocationServiceTestSuite) TestRevokeCriteriaBatch_PropagatesStoreError() {
	store := newRevocationStoreInterfaceMock(s.T())
	store.On("insertCriteria", mock.Anything, mock.Anything).Return(errors.New("db down"))

	revoker := newRevocationService(nil, store, time.Hour, 0, false, true, nil, testMaxEntriesPerChange)
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

	revoker := newRevocationService(nil, store, time.Hour, 0, false, true, nil, 1)
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

	revoker := newRevocationService(nil, store, time.Hour, 0, false, true, nil, 2)
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

	revoker := newRevocationService(nil, store, time.Hour, 0, false, true, nil, 0)
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

	revoker := newRevocationService(nil, store, time.Hour, 0, false, true, nil, 1)
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

	revoker := newRevocationService(nil, store, time.Hour, 0, false, true, nil, 1)
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

	revoker := newRevocationService(nil, store, configured, leeway, false, true, nil, testMaxEntriesPerChange)
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

			revoker := newRevocationService(nil, store, time.Hour, 0, false, true, obsMock, testMaxEntriesPerChange)
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

	revoker := newRevocationService(nil, store, configured, leeway, false, true, nil, testMaxEntriesPerChange)
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

	revoker := newRevocationService(s.jwtServiceMock, s.storeMock, time.Hour, leeway, false, true, s.obsMock,
		testMaxEntriesPerChange)
	result, err := revoker.RevokeToken(context.Background(), token, "", testClientID)

	s.NoError(err)
	s.Equal(RevokeOutcomeRevoked, result.Outcome)
}

func (s *RevocationServiceTestSuite) TestRevokeRefreshToken_ExpiryAddsTheLeeway() {
	leeway := 30 * time.Second
	expiry := time.Now().Add(time.Hour).UTC()
	s.storeMock.On("InsertRevokedToken", mock.Anything, mock.MatchedBy(func(rt RevokedToken) bool {
		return rt.JTI == "rotated-jti" && rt.ExpiryTime.Equal(expiry.Add(leeway))
	})).Return(nil)

	revoker := newRevocationService(nil, s.storeMock, time.Hour, leeway, false, true, nil, testMaxEntriesPerChange)
	err := revoker.(RefreshTokenRevokerInterface).RevokeRefreshToken(context.Background(), "rotated-jti", expiry)

	s.NoError(err)
}

// A token family row lives the configured lifetime plus the leeway.
func (s *RevocationServiceTestSuite) TestRevokeTokenFamily_ExpiryAddsTheLeeway() {
	leeway := 30 * time.Second
	var captured revocationCriterion
	s.storeMock.On("insertCriterion", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { captured = args.Get(1).(revocationCriterion) }).Return(nil)

	revoker := newRevocationService(nil, s.storeMock, time.Hour, leeway, false, true, nil, testMaxEntriesPerChange)
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
