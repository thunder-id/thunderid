// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package backchannel implements OIDC Back-Channel Logout 1.0 on the provider side: the logout
// token sent to each application that shared a terminated session, and, in a later change, the
// dispatcher that delivers it.
package backchannel

import (
	"context"
	"errors"
	"fmt"

	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/jwksresolver"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	"github.com/thunder-id/thunderid/internal/system/jose/jwe"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const (
	// claimEvents is the logout token claim that names the event it carries (Back-Channel Logout §2.4).
	claimEvents = "events"
	// eventBackchannelLogout is the single member of the events claim, with an empty object value.
	eventBackchannelLogout = "http://schemas.openid.net/event/backchannel-logout"
	// defaultTokenValidityPeriod applies when the configuration carries no positive value. It is
	// the maximum the specification recommends.
	defaultTokenValidityPeriod int64 = 120
)

// LogoutTokenBuilder builds the logout token for one application and one terminated session.
type LogoutTokenBuilder interface {
	// Build returns a signed logout token for client about subjectID's session sessionID, encrypted
	// when the client negotiated ID token encryption. Every call yields a fresh jti.
	Build(ctx context.Context, client *providers.OAuthClient, subjectID, sessionID string) (string, error)
}

type logoutTokenBuilder struct {
	cfg          oauthconfig.Config
	jwtService   jwt.JWTServiceInterface
	jweService   jwe.JWEServiceInterface
	jwksResolver *jwksresolver.Resolver
}

// NewLogoutTokenBuilder creates a LogoutTokenBuilder over the shared JWT and JWE services.
// jweService may be nil when no client encrypts ID tokens; building for such a client then fails
// rather than downgrading to a signed token, which a relying party that negotiated encryption would
// reject anyway.
func NewLogoutTokenBuilder(cfg oauthconfig.Config, jwtService jwt.JWTServiceInterface,
	jweService jwe.JWEServiceInterface, resolver *jwksresolver.Resolver) LogoutTokenBuilder {
	return &logoutTokenBuilder{cfg: cfg, jwtService: jwtService, jweService: jweService, jwksResolver: resolver}
}

// Build implements LogoutTokenBuilder.
func (b *logoutTokenBuilder) Build(ctx context.Context, client *providers.OAuthClient, subjectID, sessionID string) (
	string, error) {
	if client == nil || client.ClientID == "" {
		return "", errors.New("logout token needs a client with a client id")
	}
	if subjectID == "" || sessionID == "" {
		return "", errors.New("logout token needs a subject and a session id")
	}

	// The issuer and signing algorithm are the ones the client's ID tokens use, so the relying
	// party verifies the logout token with the key it already trusts.
	tokenConfig := tokenservice.ResolveTokenConfig(b.cfg, client, tokenservice.TokenTypeID, 0)
	validity := b.cfg.OAuth.Logout.Backchannel.TokenValidityPeriod
	if validity <= 0 {
		validity = defaultTokenValidityPeriod
	}

	// iss, iat, exp, nbf and jti come from the JWT service; nonce is never set (§2.4).
	claims := map[string]interface{}{
		"aud":                    client.ClientID,
		constants.ClaimSessionID: sessionID,
		claimEvents:              map[string]interface{}{eventBackchannelLogout: map[string]interface{}{}},
	}
	token, _, svcErr := b.jwtService.GenerateJWT(ctx, tokenservice.SubjectClaim(subjectID), tokenConfig.Issuer,
		validity, claims, jwt.TokenTypeLogout, tokenConfig.SigningAlg)
	if svcErr != nil {
		return "", fmt.Errorf("failed to sign logout token: %s", svcErr.Error)
	}

	if !encryptsIDTokens(client) {
		return token, nil
	}
	return b.encrypt(ctx, client, token, tokenConfig.Issuer)
}

// encrypt applies the client's ID token encryption to the signed logout token, replicating iss in
// the JWE header as §2.4 recommends so the relying party can pick its key before decrypting.
func (b *logoutTokenBuilder) encrypt(ctx context.Context, client *providers.OAuthClient, signed, issuer string) (
	string, error) {
	if b.jweService == nil || b.jwksResolver == nil {
		return "", errors.New("client requires encrypted logout tokens but no JWE service is configured")
	}
	idTokenCfg := client.Token.IDToken
	rpKey, rpKID, svcErr := b.jwksResolver.ResolveEncryptionKey(ctx, client.Certificate, idTokenCfg.EncryptionAlg,
		jwksresolver.KeyUseLenientEnc)
	if svcErr != nil {
		return "", fmt.Errorf("failed to resolve logout token encryption key: %s", svcErr.Error)
	}
	encrypted, svcErr := b.jweService.Encrypt(ctx, []byte(signed), &providers.KeyRef{PublicKeyJWK: rpKey},
		idTokenCfg.EncryptionAlg, jwe.ContentEncAlgorithm(idTokenCfg.EncryptionEnc), "JWT", rpKID,
		jwe.WithProtectedHeader("iss", issuer))
	if svcErr != nil {
		return "", fmt.Errorf("failed to encrypt logout token: %s", svcErr.Error)
	}
	return encrypted, nil
}

// encryptsIDTokens reports whether the client negotiated ID token encryption, in which case its
// logout tokens must be encrypted the same way.
func encryptsIDTokens(client *providers.OAuthClient) bool {
	if client.Token == nil || client.Token.IDToken == nil {
		return false
	}
	rt := client.Token.IDToken.ResponseType
	return rt == providers.IDTokenResponseTypeJWE || rt == providers.IDTokenResponseTypeNESTEDJWT
}
