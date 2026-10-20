// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package granthandlers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/thunder-id/thunderid/internal/attributecache"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/actorprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/attributecachemock"
)

const (
	testMappedSub     = "agent@example.com"
	testAgentEntityID = "agent-entity"
	testAttrCacheID   = "aci-1"
	testActorToken    = "test-actor-token" //nolint:gosec // Test token, not a real credential
)

func TestTokenBuildErrorResponse(t *testing.T) {
	ended := tokenBuildErrorResponse(
		errors.Join(errors.New("wrapped"), tokenservice.ErrLifespanEnded), "Failed to generate token")
	assert.Equal(t, constants.ErrorInvalidGrant, ended.Error)
	assert.NotContains(t, ended.ErrorDescription, "lifespan")

	other := tokenBuildErrorResponse(errors.New("signing failed"), "Failed to generate token")
	assert.Equal(t, constants.ErrorServerError, other.Error)
	assert.Equal(t, "Failed to generate token", other.ErrorDescription)
}

// withLifespanMocks gives the suite's handler an actor provider and an attribute cache, which only the
// lifespan resolution of a self-issued token's subject or actor reads.
func (suite *TokenExchangeGrantHandlerTestSuite) withLifespanMocks() (
	*actorprovidermock.ActorProviderMock, *attributecachemock.AttributeCacheServiceInterfaceMock) {
	actors := actorprovidermock.NewActorProviderMock(suite.T())
	attrCache := attributecachemock.NewAttributeCacheServiceInterfaceMock(suite.T())
	suite.handler.actorProvider = actors
	suite.handler.attrCache = attrCache
	return actors, attrCache
}

func (suite *TokenExchangeGrantHandlerTestSuite) exchangeRequest(withActor bool) *model.TokenRequest {
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub": testMappedSub,
		"iss": testCustomIssuer,
		"aud": "app123",
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
	})
	req := &model.TokenRequest{
		GrantType:        string(providers.GrantTypeTokenExchange),
		ClientID:         testClientID,
		SubjectToken:     subjectToken,
		SubjectTokenType: string(constants.TokenTypeIdentifierAccessToken),
	}
	if withActor {
		req.ActorToken = testActorToken
		req.ActorTokenType = string(constants.TokenTypeIdentifierJWT)
	}
	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:              testMappedSub,
			Iss:              testCustomIssuer,
			Scopes:           []string{testScopeRead},
			AttributeCacheID: testAttrCacheID,
		}, nil)
	return req
}

func exchangedToken() *model.TokenDTO {
	return &model.TokenDTO{Token: testTokenExchangeJWT, TokenType: constants.TokenTypeBearer, ExpiresIn: 60}
}

// A self-issued subject token whose sub is a mapped attribute is resolved to its entity through the
// attribute cache entry, so the builder caps the exchanged token at that entity's lifespan.
func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_MappedSubjectResolvedThroughAci() {
	actors, attrCache := suite.withLifespanMocks()
	req := suite.exchangeRequest(false)
	actors.On("GetActor", testMappedSub).
		Return((*providers.Entity)(nil), &tidcommon.ServiceError{Type: tidcommon.ClientErrorType})
	attrCache.On("GetAttributeCache", mock.Anything, testAttrCacheID).
		Return(&attributecache.AttributeCache{
			ID: testAttrCacheID, SubjectID: testAgentEntityID, SubjectCategory: string(providers.EntityCategoryAgent),
		}, (*tidcommon.ServiceError)(nil))
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return ctx.Subject == testMappedSub && ctx.SubjectEntityID == testAgentEntityID &&
				ctx.SubjectCategory == string(providers.EntityCategoryAgent)
		})).Return(exchangedToken(), nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), req, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
}

// A sub that names an entity needs no attribute cache read.
func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_SubjectNamingAnEntitySkipsAci() {
	actors, attrCache := suite.withLifespanMocks()
	req := suite.exchangeRequest(false)
	actors.On("GetActor", testMappedSub).
		Return(&providers.Entity{ID: testMappedSub, Category: providers.EntityCategoryUser},
			(*tidcommon.ServiceError)(nil))
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return ctx.SubjectEntityID == testMappedSub && ctx.SubjectCategory == string(providers.EntityCategoryUser)
		})).Return(exchangedToken(), nil)

	_, errResp := suite.handler.HandleGrant(context.Background(), req, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	attrCache.AssertNotCalled(suite.T(), "GetAttributeCache", mock.Anything, mock.Anything)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_UnreadableAciFailsExchange() {
	tests := []struct {
		name      string
		entry     *attributecache.AttributeCache
		cacheErr  *tidcommon.ServiceError
		wantError string
	}{
		{"entry missing", nil, nil, constants.ErrorInvalidGrant},
		{"entry names no subject", &attributecache.AttributeCache{ID: testAttrCacheID}, nil,
			constants.ErrorInvalidGrant},
		{"entry not found", nil, &tidcommon.ServiceError{Type: tidcommon.ClientErrorType},
			constants.ErrorInvalidGrant},
		{"cache unavailable", nil, &tidcommon.InternalServerError, constants.ErrorServerError},
	}
	for _, tc := range tests {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			actors, attrCache := suite.withLifespanMocks()
			req := suite.exchangeRequest(false)
			actors.On("GetActor", testMappedSub).
				Return((*providers.Entity)(nil), &tidcommon.ServiceError{Type: tidcommon.ClientErrorType})
			attrCache.On("GetAttributeCache", mock.Anything, testAttrCacheID).Return(tc.entry, tc.cacheErr)

			result, errResp := suite.handler.HandleGrant(context.Background(), req, suite.oauthApp)

			assert.Nil(suite.T(), result)
			assert.Equal(suite.T(), tc.wantError, errResp.Error)
			suite.mockTokenBuilder.AssertNotCalled(suite.T(), "BuildAccessToken", mock.Anything, mock.Anything)
		})
	}
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_SubjectLookupFailureIsServerError() {
	actors, attrCache := suite.withLifespanMocks()
	req := suite.exchangeRequest(false)
	actors.On("GetActor", testMappedSub).Return((*providers.Entity)(nil), &tidcommon.InternalServerError)

	result, errResp := suite.handler.HandleGrant(context.Background(), req, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), constants.ErrorServerError, errResp.Error)
	attrCache.AssertNotCalled(suite.T(), "GetAttributeCache", mock.Anything, mock.Anything)
}

// An actor_token's mapped sub is resolved the same way, and passed to the builder as the actor's
// resource ID.
func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_MappedActorResolvedThroughAci() {
	actors, attrCache := suite.withLifespanMocks()
	req := suite.exchangeRequest(true)
	suite.mockTokenValidator.On("ValidateActorToken", mock.Anything, testActorToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub: "actor@example.com", Iss: testCustomIssuer, AttributeCacheID: "aci-actor",
		}, nil)
	actors.On("GetActor", testMappedSub).
		Return(&providers.Entity{ID: testMappedSub}, (*tidcommon.ServiceError)(nil))
	actors.On("GetActor", "actor@example.com").
		Return((*providers.Entity)(nil), &tidcommon.ServiceError{Type: tidcommon.ClientErrorType})
	attrCache.On("GetAttributeCache", mock.Anything, "aci-actor").
		Return(&attributecache.AttributeCache{
			ID: "aci-actor", SubjectID: testAgentEntityID, SubjectCategory: string(providers.EntityCategoryAgent),
		}, (*tidcommon.ServiceError)(nil))
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return ctx.ActorClaims != nil && ctx.ActorClaims.Sub == "actor@example.com" &&
				ctx.ActorClaims.EntityID == testAgentEntityID &&
				ctx.ActorClaims.EntityCategory == string(providers.EntityCategoryAgent)
		})).Return(exchangedToken(), nil)

	_, errResp := suite.handler.HandleGrant(context.Background(), req, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
}

// An externally-issued token carries no attribute cache entry, so the handler resolves nothing and
// leaves the subject to the builder.
func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_ExternalSubjectNotResolved() {
	actors, attrCache := suite.withLifespanMocks()
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub": testUserID, "iss": testCustomIssuer, "aud": "app123",
		"exp": float64(now + 3600), "nbf": float64(now - 60),
	})
	req := &model.TokenRequest{
		GrantType:        string(providers.GrantTypeTokenExchange),
		ClientID:         testClientID,
		SubjectToken:     subjectToken,
		SubjectTokenType: string(constants.TokenTypeIdentifierAccessToken),
	}
	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{Sub: testUserID, Iss: testCustomIssuer}, nil)
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return ctx.SubjectEntityID == ""
		})).Return(exchangedToken(), nil)

	_, errResp := suite.handler.HandleGrant(context.Background(), req, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	actors.AssertNotCalled(suite.T(), "GetActor", mock.Anything)
	attrCache.AssertNotCalled(suite.T(), "GetAttributeCache", mock.Anything, mock.Anything)
}

// The builder refusing an expired agent's token answers invalid_grant, not server_error.
func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_LifespanEndedIsInvalidGrant() {
	actors, _ := suite.withLifespanMocks()
	req := suite.exchangeRequest(false)
	actors.On("GetActor", testMappedSub).
		Return(&providers.Entity{ID: testMappedSub}, (*tidcommon.ServiceError)(nil))
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything, mock.Anything).
		Return((*model.TokenDTO)(nil), tokenservice.ErrLifespanEnded)

	result, errResp := suite.handler.HandleGrant(context.Background(), req, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), constants.ErrorInvalidGrant, errResp.Error)
}

// An actor provider that cannot look up entities, as when the entity provider is disabled, leaves the
// subject unresolved rather than failing the exchange.
func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_SubjectLookupNotSupportedLeavesSubjectUnresolved() {
	actors, attrCache := suite.withLifespanMocks()
	req := suite.exchangeRequest(false)
	actors.On("GetActor", testMappedSub).
		Return((*providers.Entity)(nil), &providers.ErrorActorLookupNotSupported)
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return ctx.Subject == testMappedSub && ctx.SubjectEntityID == "" && ctx.SubjectCategory == ""
		})).Return(exchangedToken(), nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), req, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
	attrCache.AssertNotCalled(suite.T(), "GetAttributeCache", mock.Anything, mock.Anything)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_IDJAG_LifespanEndedIsInvalidGrant() {
	suite.oauthApp.Token.IDJAG = &providers.IDJAGConfig{
		Enabled:          true,
		AllowedAudiences: []string{testIDJAGAudience},
	}
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub": testUserID,
		"iss": testIDJAGServerIssuer,
		"aud": testClientID,
		"exp": float64(time.Now().Unix() + 3600),
	})
	req := &model.TokenRequest{
		GrantType:          string(providers.GrantTypeTokenExchange),
		ClientID:           testClientID,
		SubjectToken:       subjectToken,
		SubjectTokenType:   string(constants.TokenTypeIdentifierIDToken),
		RequestedTokenType: string(constants.TokenTypeIdentifierIDJAG),
		Audiences:          []string{testIDJAGAudience},
	}
	suite.mockTokenValidator.On("ValidateIDJAGSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub: testUserID,
			Iss: testIDJAGServerIssuer,
			Aud: []string{testClientID},
		}, nil)
	suite.mockTokenBuilder.On("BuildIDJAG", mock.Anything, mock.Anything).
		Return(nil, tokenservice.ErrLifespanEnded)

	result, errResp := suite.handler.HandleGrant(context.Background(), req, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), constants.ErrorInvalidGrant, errResp.Error)
}
