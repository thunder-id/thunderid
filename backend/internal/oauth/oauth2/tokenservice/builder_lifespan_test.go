// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package tokenservice

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/actorprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
)

const (
	lifespanIssuer        = "https://example.com"
	lifespanValidity      = int64(1200)
	lifespanAgentClientID = "agent-client"
	lifespanUserID        = "user-1"
)

func lifespanAttrs(t *testing.T, validUntil string) json.RawMessage {
	t.Helper()
	if validUntil == "" {
		return json.RawMessage(`{"name":"agent"}`)
	}
	raw, err := json.Marshal(map[string]string{authnprovidercm.SystemAttrValidUntil: validUntil})
	assert.NoError(t, err)
	return raw
}

func lifespanAgent(t *testing.T, id, validUntil string) *providers.Entity {
	return &providers.Entity{
		ID:               id,
		Category:         providers.EntityCategoryAgent,
		SystemAttributes: lifespanAttrs(t, validUntil),
	}
}

func inFromNow(d time.Duration) string {
	return time.Now().Add(d).UTC().Format(time.RFC3339)
}

func newLifespanBuilder(t *testing.T) (*tokenBuilder, *jwtmock.JWTServiceInterfaceMock,
	*actorprovidermock.ActorProviderMock) {
	jwtService := jwtmock.NewJWTServiceInterfaceMock(t)
	actors := actorprovidermock.NewActorProviderMock(t)
	return &tokenBuilder{
		cfg: oauthconfig.Config{
			JWT: engineconfig.JWTConfig{Issuer: lifespanIssuer, ValidityPeriod: 3600},
		},
		jwtService:    jwtService,
		actorProvider: actors,
	}, jwtService, actors
}

// within reports whether a validity period lies in [low, high]. The builder holds back a second, and
// the test clock may cross a second boundary, so a capped validity is checked against a range.
func within(low, high int64) interface{} {
	return mock.MatchedBy(func(v int64) bool { return v >= low && v <= high })
}

func agentClientApp() *providers.OAuthClient {
	return &providers.OAuthClient{
		ID:             lifespanAgentClientID,
		ClientID:       lifespanAgentClientID,
		EntityCategory: providers.EntityCategoryAgent,
	}
}

func TestBuildAccessToken_CappedAtClientAgentLifespan(t *testing.T) {
	tb, jwtService, actors := newLifespanBuilder(t)
	actors.On("GetActor", lifespanAgentClientID).
		Return(lifespanAgent(t, lifespanAgentClientID, inFromNow(5*time.Minute)), (*tidcommon.ServiceError)(nil))
	jwtService.On("GenerateJWT", mock.Anything, lifespanAgentClientID, lifespanIssuer, within(298, 299),
		mock.Anything, jwt.TokenTypeAccessToken, "").Return(testAccessToken, time.Now().Unix(), nil)

	token, err := tb.BuildAccessToken(t.Context(), &AccessTokenBuildContext{
		Subject:        lifespanAgentClientID,
		Audiences:      []string{"rs"},
		ClientID:       lifespanAgentClientID,
		OAuthApp:       agentClientApp(),
		ValidityPeriod: lifespanValidity,
	})

	assert.NoError(t, err)
	assert.GreaterOrEqual(t, token.ExpiresIn, int64(298))
	assert.LessOrEqual(t, token.ExpiresIn, int64(299))
}

func TestBuildAccessToken_EarliestLifespanOfInvolvedAgentsApplies(t *testing.T) {
	tb, jwtService, actors := newLifespanBuilder(t)
	actors.On("GetActor", "agent-subject").
		Return(lifespanAgent(t, "agent-subject", inFromNow(10*time.Minute)), (*tidcommon.ServiceError)(nil))
	actors.On("GetActor", "agent-actor").
		Return(lifespanAgent(t, "agent-actor", inFromNow(3*time.Minute)), (*tidcommon.ServiceError)(nil))
	jwtService.On("GenerateJWT", mock.Anything, "agent-subject", lifespanIssuer, within(178, 179),
		mock.Anything, jwt.TokenTypeAccessToken, "").Return(testAccessToken, time.Now().Unix(), nil)

	token, err := tb.BuildAccessToken(t.Context(), &AccessTokenBuildContext{
		Subject:   "agent-subject",
		Audiences: []string{"rs"},
		ClientID:  "app-client",
		OAuthApp: &providers.OAuthClient{
			ID: "app", ClientID: "app-client", EntityCategory: providers.EntityCategoryApp,
		},
		ActorClaims:    &SubjectTokenClaims{Sub: "agent-actor"},
		ValidityPeriod: lifespanValidity,
	})

	assert.NoError(t, err)
	assert.LessOrEqual(t, token.ExpiresIn, int64(179))
}

func TestBuildAccessToken_NoCapWithoutABoundedAgent(t *testing.T) {
	tests := []struct {
		name   string
		entity *providers.Entity
	}{
		{"unbounded sentinel", lifespanAgent(t, lifespanAgentClientID, authnprovidercm.UnboundedValidUntil)},
		{"no validUntil", lifespanAgent(t, lifespanAgentClientID, "")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tb, jwtService, actors := newLifespanBuilder(t)
			actors.On("GetActor", lifespanAgentClientID).Return(tc.entity, (*tidcommon.ServiceError)(nil))
			jwtService.On("GenerateJWT", mock.Anything, lifespanAgentClientID, lifespanIssuer, lifespanValidity,
				mock.Anything, jwt.TokenTypeAccessToken, "").Return(testAccessToken, time.Now().Unix(), nil)

			token, err := tb.BuildAccessToken(t.Context(), &AccessTokenBuildContext{
				Subject:        lifespanAgentClientID,
				Audiences:      []string{"rs"},
				OAuthApp:       agentClientApp(),
				ValidityPeriod: lifespanValidity,
			})

			assert.NoError(t, err)
			assert.Equal(t, lifespanValidity, token.ExpiresIn)
		})
	}
}

// A user subject, an application client, and a mapped subject attribute impose no cap. The user is
// read because its category is unknown to the builder; the application is skipped without a read.
func TestBuildAccessToken_NonAgentsImposeNoCap(t *testing.T) {
	tb, jwtService, actors := newLifespanBuilder(t)
	actors.On("GetActor", lifespanUserID).
		Return(&providers.Entity{ID: lifespanUserID, Category: providers.EntityCategoryUser,
			SystemAttributes: lifespanAttrs(t, inFromNow(-time.Hour))}, (*tidcommon.ServiceError)(nil))
	actors.On("GetActor", "alice@example.com").
		Return((*providers.Entity)(nil), &tidcommon.ServiceError{Type: tidcommon.ClientErrorType})
	jwtService.On("GenerateJWT", mock.Anything, lifespanUserID, lifespanIssuer, lifespanValidity,
		mock.Anything, jwt.TokenTypeAccessToken, "").Return(testAccessToken, time.Now().Unix(), nil)

	token, err := tb.BuildAccessToken(t.Context(), &AccessTokenBuildContext{
		Subject:   lifespanUserID,
		Audiences: []string{"rs"},
		OAuthApp: &providers.OAuthClient{
			ID: "app", ClientID: "app-client", EntityCategory: providers.EntityCategoryApp,
		},
		ActorClaims:    &SubjectTokenClaims{Sub: "alice@example.com"},
		ValidityPeriod: lifespanValidity,
	})

	assert.NoError(t, err)
	assert.Equal(t, lifespanValidity, token.ExpiresIn)
	actors.AssertNotCalled(t, "GetActor", "app")
}

func TestBuildAccessToken_RefusedOnceLifespanEnded(t *testing.T) {
	tests := []struct {
		name       string
		validUntil string
	}{
		{"in the past", inFromNow(-time.Minute)},
		{"now", inFromNow(0)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tb, jwtService, actors := newLifespanBuilder(t)
			actors.On("GetActor", lifespanAgentClientID).
				Return(lifespanAgent(t, lifespanAgentClientID, tc.validUntil), (*tidcommon.ServiceError)(nil))

			token, err := tb.BuildAccessToken(t.Context(), &AccessTokenBuildContext{
				Subject:        lifespanAgentClientID,
				Audiences:      []string{"rs"},
				OAuthApp:       agentClientApp(),
				ValidityPeriod: lifespanValidity,
			})

			assert.Nil(t, token)
			assert.ErrorIs(t, err, ErrLifespanEnded)
			jwtService.AssertNotCalled(t, "GenerateJWT")
		})
	}
}

func TestBuildAccessToken_LookupFailureFailsIssuance(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(*actorprovidermock.ActorProviderMock)
		tokCtx *AccessTokenBuildContext
	}{
		{
			name: "agent client cannot be read",
			setup: func(a *actorprovidermock.ActorProviderMock) {
				a.On("GetActor", lifespanAgentClientID).
					Return((*providers.Entity)(nil), &tidcommon.InternalServerError)
			},
			tokCtx: &AccessTokenBuildContext{Subject: lifespanAgentClientID, OAuthApp: agentClientApp()},
		},
		{
			name: "agent client not found",
			setup: func(a *actorprovidermock.ActorProviderMock) {
				a.On("GetActor", lifespanAgentClientID).
					Return((*providers.Entity)(nil), &tidcommon.ServiceError{Type: tidcommon.ClientErrorType})
			},
			tokCtx: &AccessTokenBuildContext{Subject: lifespanAgentClientID, OAuthApp: agentClientApp()},
		},
		{
			name: "subject read fails",
			setup: func(a *actorprovidermock.ActorProviderMock) {
				a.On("GetActor", lifespanUserID).
					Return((*providers.Entity)(nil), &tidcommon.InternalServerError)
			},
			tokCtx: &AccessTokenBuildContext{Subject: lifespanUserID},
		},
		{
			name: "stored validUntil is corrupt",
			setup: func(a *actorprovidermock.ActorProviderMock) {
				a.On("GetActor", lifespanAgentClientID).
					Return(lifespanAgent(t, lifespanAgentClientID, "not-a-time"), (*tidcommon.ServiceError)(nil))
			},
			tokCtx: &AccessTokenBuildContext{Subject: lifespanAgentClientID, OAuthApp: agentClientApp()},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tb, jwtService, actors := newLifespanBuilder(t)
			tc.setup(actors)

			token, err := tb.BuildAccessToken(t.Context(), tc.tokCtx)

			assert.Nil(t, token)
			assert.Error(t, err)
			assert.NotErrorIs(t, err, ErrLifespanEnded)
			jwtService.AssertNotCalled(t, "GenerateJWT")
		})
	}
}

// Without an actor provider the builder cannot read any lifespan, and keeps issuing as before.
func TestBuildAccessToken_NoProviderNoCap(t *testing.T) {
	tb, jwtService, _ := newLifespanBuilder(t)
	tb.actorProvider = nil
	jwtService.On("GenerateJWT", mock.Anything, lifespanAgentClientID, lifespanIssuer, lifespanValidity,
		mock.Anything, jwt.TokenTypeAccessToken, "").Return(testAccessToken, time.Now().Unix(), nil)

	token, err := tb.BuildAccessToken(t.Context(), &AccessTokenBuildContext{
		Subject: lifespanAgentClientID, OAuthApp: agentClientApp(), ValidityPeriod: lifespanValidity,
	})

	assert.NoError(t, err)
	assert.Equal(t, lifespanValidity, token.ExpiresIn)
}

// A subject resolved by the grant, such as through a mapped sub's attribute cache entry, is capped by
// its resource ID rather than the token's sub.
func TestBuildAccessToken_CapsSubjectByResolvedEntityID(t *testing.T) {
	tb, jwtService, actors := newLifespanBuilder(t)
	actors.On("GetActor", "agent-subject").
		Return(lifespanAgent(t, "agent-subject", inFromNow(5*time.Minute)), (*tidcommon.ServiceError)(nil))
	jwtService.On("GenerateJWT", mock.Anything, "agent@example.com", lifespanIssuer, within(298, 299),
		mock.Anything, jwt.TokenTypeAccessToken, "").Return(testAccessToken, time.Now().Unix(), nil)

	_, err := tb.BuildAccessToken(t.Context(), &AccessTokenBuildContext{
		Subject:         "agent@example.com",
		SubjectEntityID: "agent-subject",
		Audiences:       []string{"rs"},
		ValidityPeriod:  lifespanValidity,
	})

	assert.NoError(t, err)
	actors.AssertNotCalled(t, "GetActor", "agent@example.com")
}

func TestBuildRefreshToken_CappedAtLifespan(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt int64
		low, high int64
	}{
		{"first issuance", 0, 298, 299},
		{"rotation inherits a later expiry", time.Now().Add(time.Hour).Unix(), 298, 299},
		{"rotation inherits an earlier expiry", time.Now().Add(time.Minute).Unix(), 59, 60},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tb, jwtService, actors := newLifespanBuilder(t)
			actors.On("GetActor", lifespanAgentClientID).
				Return(lifespanAgent(t, lifespanAgentClientID, inFromNow(5*time.Minute)),
					(*tidcommon.ServiceError)(nil))
			jwtService.On("GenerateJWT", mock.Anything, lifespanAgentClientID, lifespanIssuer,
				within(tc.low, tc.high), mock.Anything, jwt.TokenTypeRefreshToken, "").
				Return(testRefreshToken, time.Now().Unix(), nil)

			token, err := tb.BuildRefreshToken(t.Context(), &RefreshTokenBuildContext{
				ClientID:           lifespanAgentClientID,
				AccessTokenSubject: lifespanAgentClientID,
				OAuthApp:           agentClientApp(),
				ExpiresAt:          tc.expiresAt,
			})

			assert.NoError(t, err)
			assert.LessOrEqual(t, token.ExpiresIn, tc.high)
		})
	}
}

func TestBuildRefreshToken_RefusedOnceLifespanEnded(t *testing.T) {
	tb, jwtService, actors := newLifespanBuilder(t)
	actors.On("GetActor", "agent-subject").
		Return(lifespanAgent(t, "agent-subject", inFromNow(-time.Second)), (*tidcommon.ServiceError)(nil))

	_, err := tb.BuildRefreshToken(t.Context(), &RefreshTokenBuildContext{
		ClientID:           "app-client",
		AccessTokenSubject: "agent@example.com",
		SubjectEntityID:    "agent-subject",
		OAuthApp: &providers.OAuthClient{
			ID: "app", ClientID: "app-client", EntityCategory: providers.EntityCategoryApp,
		},
	})

	assert.ErrorIs(t, err, ErrLifespanEnded)
	jwtService.AssertNotCalled(t, "GenerateJWT")
}

func TestBuildIDToken_CappedAtLifespan(t *testing.T) {
	tb, jwtService, actors := newLifespanBuilder(t)
	actors.On("GetActor", "agent-subject").
		Return(lifespanAgent(t, "agent-subject", inFromNow(5*time.Minute)), (*tidcommon.ServiceError)(nil))
	jwtService.On("GenerateJWT", mock.Anything, "agent-subject", lifespanIssuer, within(298, 299),
		mock.Anything, jwt.TokenTypeJWT, "").Return(testIDToken, time.Now().Unix(), nil)

	token, err := tb.BuildIDToken(t.Context(), &IDTokenBuildContext{
		Subject:  "agent-subject",
		Audience: "app-client",
		OAuthApp: &providers.OAuthClient{
			ID: "app", ClientID: "app-client", EntityCategory: providers.EntityCategoryApp,
		},
	})

	assert.NoError(t, err)
	assert.LessOrEqual(t, token.ExpiresIn, int64(299))
}

func TestBuildIDToken_RefusedOnceLifespanEnded(t *testing.T) {
	tb, jwtService, actors := newLifespanBuilder(t)
	actors.On("GetActor", "agent-subject").
		Return(lifespanAgent(t, "agent-subject", inFromNow(-time.Hour)), (*tidcommon.ServiceError)(nil))

	_, err := tb.BuildIDToken(t.Context(), &IDTokenBuildContext{
		Subject: "agent-subject", Audience: "app-client", SubjectEntityID: "agent-subject",
	})

	assert.ErrorIs(t, err, ErrLifespanEnded)
	jwtService.AssertNotCalled(t, "GenerateJWT")
}

// A subject the grant already knows to be a user cannot have a lifespan, so the ID and refresh tokens
// issued for it read no entity, and an entity store failure cannot fail their issuance.
func TestBuildIDAndRefreshToken_KnownUserSubjectIsNotLookedUp(t *testing.T) {
	tb, jwtService, actors := newLifespanBuilder(t)
	app := &providers.OAuthClient{ID: "app", ClientID: "app-client", EntityCategory: providers.EntityCategoryApp}
	jwtService.On("GenerateJWT", mock.Anything, mock.Anything, lifespanIssuer, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return(testIDToken, time.Now().Unix(), nil)

	_, err := tb.BuildIDToken(t.Context(), &IDTokenBuildContext{
		Subject: "alice@example.com", Audience: "app-client", OAuthApp: app,
		SubjectEntityID: lifespanUserID, SubjectCategory: string(providers.EntityCategoryUser),
	})
	assert.NoError(t, err)

	_, err = tb.BuildRefreshToken(t.Context(), &RefreshTokenBuildContext{
		ClientID: "app-client", AccessTokenSubject: "alice@example.com", ActorSub: "app", OAuthApp: app,
		SubjectEntityID: lifespanUserID, SubjectCategory: string(providers.EntityCategoryUser),
	})
	assert.NoError(t, err)

	actors.AssertNotCalled(t, "GetActor", mock.Anything)
}

// An actor the grant resolved with a known non-agent category is not read again.
func TestBuildAccessToken_KnownActorCategoryIsNotLookedUp(t *testing.T) {
	tb, jwtService, actors := newLifespanBuilder(t)
	jwtService.On("GenerateJWT", mock.Anything, lifespanUserID, lifespanIssuer, lifespanValidity,
		mock.Anything, jwt.TokenTypeAccessToken, "").Return(testAccessToken, time.Now().Unix(), nil)

	_, err := tb.BuildAccessToken(t.Context(), &AccessTokenBuildContext{
		Subject: lifespanUserID, SubjectEntityID: lifespanUserID,
		SubjectCategory: string(providers.EntityCategoryUser), ValidityPeriod: lifespanValidity,
		ActorClaims: &SubjectTokenClaims{
			Sub: "app@example.com", EntityID: "app", EntityCategory: string(providers.EntityCategoryApp),
		},
	})

	assert.NoError(t, err)
	actors.AssertNotCalled(t, "GetActor", mock.Anything)
}

// An actor provider that cannot look up entities, as when the entity provider is disabled, holds no
// agents, so a subject of any category is issued an uncapped token.
func TestBuildAccessToken_LookupNotSupportedImposesNoCap(t *testing.T) {
	tb, jwtService, actors := newLifespanBuilder(t)
	actors.On("GetActor", lifespanUserID).
		Return((*providers.Entity)(nil), &providers.ErrorActorLookupNotSupported)
	jwtService.On("GenerateJWT", mock.Anything, lifespanUserID, lifespanIssuer, lifespanValidity,
		mock.Anything, jwt.TokenTypeAccessToken, "").Return(testAccessToken, time.Now().Unix(), nil)

	token, err := tb.BuildAccessToken(t.Context(), &AccessTokenBuildContext{
		Subject: lifespanUserID, Audiences: []string{"rs"}, ValidityPeriod: lifespanValidity,
	})

	assert.NoError(t, err)
	assert.Equal(t, lifespanValidity, token.ExpiresIn)
}

func idjagClientApp() *providers.OAuthClient {
	app := agentClientApp()
	app.Token = &providers.OAuthTokenConfig{IDJAG: &providers.IDJAGConfig{ValidityPeriod: lifespanValidity}}
	return app
}

func TestBuildIDJAG_CappedAtLifespan(t *testing.T) {
	tb, jwtService, actors := newLifespanBuilder(t)
	actors.On("GetActor", lifespanAgentClientID).
		Return(lifespanAgent(t, lifespanAgentClientID, inFromNow(5*time.Minute)), (*tidcommon.ServiceError)(nil))
	actors.On("GetActor", lifespanUserID).
		Return(&providers.Entity{ID: lifespanUserID, Category: providers.EntityCategoryUser},
			(*tidcommon.ServiceError)(nil))
	jwtService.On("GenerateJWT", mock.Anything, lifespanUserID, lifespanIssuer, within(298, 299),
		mock.Anything, jwt.TokenTypeIDJAG, "").Return(testAccessToken, time.Now().Unix(), nil)

	token, err := tb.BuildIDJAG(t.Context(), &IDJAGBuildContext{
		Subject: lifespanUserID, Audience: "rs", ClientID: lifespanAgentClientID, OAuthApp: idjagClientApp(),
	})

	assert.NoError(t, err)
	assert.LessOrEqual(t, token.ExpiresIn, int64(299))
}

func TestBuildIDJAG_RefusedOnceLifespanEnded(t *testing.T) {
	tb, jwtService, actors := newLifespanBuilder(t)
	actors.On("GetActor", lifespanAgentClientID).
		Return(lifespanAgent(t, lifespanAgentClientID, inFromNow(-time.Hour)), (*tidcommon.ServiceError)(nil))
	actors.On("GetActor", lifespanUserID).
		Return(&providers.Entity{ID: lifespanUserID, Category: providers.EntityCategoryUser},
			(*tidcommon.ServiceError)(nil)).Maybe()

	token, err := tb.BuildIDJAG(t.Context(), &IDJAGBuildContext{
		Subject: lifespanUserID, Audience: "rs", ClientID: lifespanAgentClientID, OAuthApp: idjagClientApp(),
	})

	assert.Nil(t, token)
	assert.ErrorIs(t, err, ErrLifespanEnded)
	jwtService.AssertNotCalled(t, "GenerateJWT")
}
