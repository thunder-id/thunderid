// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package security

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
)

type BearerAuthenticatorTestSuite struct {
	suite.Suite
	mockJWT      *jwtmock.JWTServiceInterfaceMock
	mockEnforcer *RevocationEnforcerInterfaceMock
	token        string
}

func TestBearerAuthenticatorSuite(t *testing.T) {
	suite.Run(t, new(BearerAuthenticatorTestSuite))
}

func (suite *BearerAuthenticatorTestSuite) SetupTest() {
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("", &config.Config{
		JWT: engineconfig.JWTConfig{Issuer: testLocalIssuer},
	})
	suite.mockJWT = jwtmock.NewJWTServiceInterfaceMock(suite.T())
	suite.mockEnforcer = NewRevocationEnforcerInterfaceMock(suite.T())
	suite.token = buildFakeJWT(accessTokenHeader(), map[string]interface{}{
		"sub":       "user-123",
		"client_id": "client-abc",
		"jti":       "jti-1",
		"iat":       float64(1_700_000_000),
		"iss":       testLocalIssuer,
	})
	suite.mockJWT.On("VerifyJWT", mock.Anything, suite.token, testResourceIdentifier, "").Return(nil).Maybe()
}

func (suite *BearerAuthenticatorTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (suite *BearerAuthenticatorTestSuite) expectedIdentity() RevocationIdentity {
	return RevocationIdentity{
		JTI:           "jti-1",
		Subject:       "user-123",
		AppKey:        "client-abc",
		Scopes:        []string{},
		EstablishedAt: time.Unix(1_700_000_000, 0).UTC(),
	}
}

func (suite *BearerAuthenticatorTestSuite) TestAuthenticate_PassesTheClientToTheRevocationCheck() {
	suite.mockEnforcer.On("EnsureNotRevoked", mock.Anything, suite.expectedIdentity()).Return(nil).Once()
	auth := NewBearerAuthenticator(suite.mockJWT, suite.mockEnforcer, testResourceIdentifier)

	securityCtx, err := auth.Authenticate(context.Background(), suite.token)

	suite.Require().NoError(err)
	suite.Equal("user-123", securityCtx.subject)
}

func (suite *BearerAuthenticatorTestSuite) TestAuthenticate_RejectsARevokedClient() {
	suite.mockEnforcer.On("EnsureNotRevoked", mock.Anything, suite.expectedIdentity()).
		Return(errors.New("revoked")).Once()
	auth := NewBearerAuthenticator(suite.mockJWT, suite.mockEnforcer, testResourceIdentifier)

	securityCtx, err := auth.Authenticate(context.Background(), suite.token)

	suite.Nil(securityCtx)
	suite.ErrorIs(err, errInvalidToken)
}

func (suite *BearerAuthenticatorTestSuite) TestEnforcesEveryRevocationDimension() {
	issuedAt := time.Unix(1_700_000_000, 0).UTC()
	token := selfIssuedBearerToken(issuedAt, []interface{}{testResourceIdentifier})
	suite.mockJWT.On("VerifyJWT", mock.Anything, token, testResourceIdentifier, "").Return(nil)
	suite.mockEnforcer.On("EnsureNotRevoked", mock.Anything, RevocationIdentity{
		JTI:           "jti-1",
		TokenFamilyID: "tfid-1",
		Subject:       "user-1",
		AppKey:        "app-1",
		Audiences:     []string{testResourceIdentifier},
		Scopes:        []string{"read:docs", "write:docs"},
		EstablishedAt: issuedAt,
	}).Return(nil).Once()

	securityCtx, err := NewBearerAuthenticator(suite.mockJWT, suite.mockEnforcer, testResourceIdentifier).
		Authenticate(context.Background(), token)

	suite.NoError(err)
	suite.NotNil(securityCtx)
}

func (suite *BearerAuthenticatorTestSuite) TestRejectsTokenTheEnforcerReportsRevoked() {
	token := selfIssuedBearerToken(time.Now(), []interface{}{testResourceIdentifier})
	suite.mockJWT.On("VerifyJWT", mock.Anything, token, testResourceIdentifier, "").Return(nil)
	suite.mockEnforcer.On("EnsureNotRevoked", mock.Anything, mock.MatchedBy(func(identity RevocationIdentity) bool {
		return len(identity.Audiences) == 1 && identity.Audiences[0] == testResourceIdentifier &&
			identity.AppKey == "app-1"
	})).Return(errors.New("scope revoked")).Once()

	securityCtx, err := NewBearerAuthenticator(suite.mockJWT, suite.mockEnforcer, testResourceIdentifier).
		Authenticate(context.Background(), token)

	suite.ErrorIs(err, errInvalidToken)
	suite.Nil(securityCtx)
}

func (suite *BearerAuthenticatorTestSuite) TestRejectsScopedTokenWithMalformedAudienceBeforeEnforcement() {
	token := selfIssuedBearerToken(time.Now(), []interface{}{testResourceIdentifier, float64(7)})
	suite.mockJWT.On("VerifyJWT", mock.Anything, token, testResourceIdentifier, "").Return(nil)

	securityCtx, err := NewBearerAuthenticator(suite.mockJWT, suite.mockEnforcer, testResourceIdentifier).
		Authenticate(context.Background(), token)

	suite.ErrorIs(err, errInvalidToken)
	suite.Nil(securityCtx)
	suite.mockEnforcer.AssertNotCalled(suite.T(), "EnsureNotRevoked", mock.Anything, mock.Anything)
}

// selfIssuedBearerToken returns a self-issued access token carrying every revocation claim.
func selfIssuedBearerToken(issuedAt time.Time, audience interface{}) string {
	return buildFakeJWT(accessTokenHeader(), map[string]interface{}{
		"iss":       testLocalIssuer,
		"sub":       "user-1",
		"jti":       "jti-1",
		"tfid":      "tfid-1",
		"client_id": "app-1",
		"aud":       audience,
		"scope":     "read:docs write:docs",
		"iat":       float64(issuedAt.Unix()),
		"exp":       float64(issuedAt.Add(time.Hour).Unix()),
	})
}
