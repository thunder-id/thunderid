// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package introspect provides functionality for the OAuth2 token introspection endpoint
package introspect

import (
	"context"
	"errors"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dpop"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/revocation"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// TokenIntrospectionServiceInterface defines the interface for OAuth 2.0 token introspection.
type TokenIntrospectionServiceInterface interface {
	IntrospectToken(ctx context.Context, token, tokenTypeHint, clientID string) (*IntrospectResponse, error)
}

// tokenIntrospectionService implements the TokenIntrospectionServiceInterface.
type tokenIntrospectionService struct {
	tokenValidator tokenservice.TokenValidatorInterface
}

// newTokenIntrospectionService creates a new tokenIntrospectionService instance (internal use).
func newTokenIntrospectionService(
	tokenValidator tokenservice.TokenValidatorInterface,
) TokenIntrospectionServiceInterface {
	return &tokenIntrospectionService{
		tokenValidator: tokenValidator,
	}
}

// IntrospectToken validates and introspects the token. It only returns an error if a server error occurs.
// All other failures are treated as inactive token as defined in the RFC 7662.
func (s *tokenIntrospectionService) IntrospectToken(
	ctx context.Context, token, tokenTypeHint, clientID string,
) (*IntrospectResponse, error) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "TokenIntrospectionService"))

	if token == "" {
		return nil, errors.New("token is required")
	}

	// ValidateToken verifies the signature and enforces the RFC 7009 deny list. A revoked or otherwise
	// invalid token is inactive per RFC 7662; if the deny list cannot be consulted we fail closed
	// (surface a server error) rather than asserting the token is active.
	payload, err := s.tokenValidator.ValidateToken(ctx, token)
	if err != nil {
		if errors.Is(err, revocation.ErrEnforcementUnavailable) {
			logger.Error(ctx, "Token revocation status could not be verified", log.Error(err))
			return nil, err
		}
		logger.Debug(ctx, "Token is inactive", log.Error(err))
		return &IntrospectResponse{
			Active: false,
		}, nil
	}

	if !isAuthorizedIntrospector(payload, clientID) {
		logger.Debug(ctx, "Caller is not a party to the token; reporting it inactive")
		return &IntrospectResponse{
			Active: false,
		}, nil
	}

	return s.prepareValidResponse(payload), nil
}

// isAuthorizedIntrospector reports whether the authenticated client is a party to the token and may
// therefore introspect it. RFC 7662 section 2.1 requires the authorization server to determine whether
// the caller is authorized for the presented token. An unauthorized caller is answered with
// {"active": false} rather than an error, so the endpoint cannot be used to enumerate metadata about
// tokens belonging to other clients.
//
// The two token types name their client differently. An access token carries the client it was issued
// to in "client_id". A refresh token carries no "client_id" and is issued to the client as its subject,
// so "sub" identifies the client there. A resource server the token is audienced to is also a party to
// it, so a matching "aud" entry authorizes the caller as well.
func isAuthorizedIntrospector(payload map[string]interface{}, clientID string) bool {
	if clientID == "" {
		return false
	}

	if tokenClientID, ok := payload["client_id"].(string); ok && tokenClientID != "" {
		if tokenClientID == clientID {
			return true
		}
	} else if sub, ok := payload[constants.ClaimSub].(string); ok && sub == clientID {
		return true
	}

	return audienceContains(payload[constants.ClaimAud], clientID)
}

// audienceContains reports whether the "aud" claim, which JWT allows to be either a single string or
// an array of strings, includes the given value.
func audienceContains(aud interface{}, clientID string) bool {
	switch value := aud.(type) {
	case string:
		return value == clientID
	case []interface{}:
		for _, entry := range value {
			if entryStr, ok := entry.(string); ok && entryStr == clientID {
				return true
			}
		}
	}
	return false
}

// prepareValidResponse prepares the response for a valid token introspection.
func (s *tokenIntrospectionService) prepareValidResponse(payload map[string]interface{}) *IntrospectResponse {
	response := &IntrospectResponse{
		Active:    true,
		TokenType: constants.TokenTypeBearer,
	}

	if jkt, _ := dpop.ExtractCnfJkt(payload); jkt != "" {
		response.Cnf = &CnfClaim{Jkt: jkt}
		response.TokenType = constants.TokenTypeDPoP
	}

	if scope, ok := payload["scope"].(string); ok {
		response.Scope = scope
	}
	if clientID, ok := payload["client_id"].(string); ok {
		response.ClientID = clientID
	}
	if username, ok := payload["username"].(string); ok {
		response.Username = username
	}

	if exp, ok := payload[constants.ClaimExp].(float64); ok {
		response.Exp = int64(exp)
	}
	if iat, ok := payload[constants.ClaimIat].(float64); ok {
		response.Iat = int64(iat)
	}
	if nbf, ok := payload["nbf"].(float64); ok {
		response.Nbf = int64(nbf)
	}

	if sub, ok := payload[constants.ClaimSub].(string); ok {
		response.Sub = sub
	}
	switch aud := payload[constants.ClaimAud].(type) {
	case string:
		response.Aud = aud
	case []interface{}:
		audSlice := make([]string, 0, len(aud))
		for _, v := range aud {
			if s, ok := v.(string); ok {
				audSlice = append(audSlice, s)
			}
		}
		if len(audSlice) > 0 {
			response.Aud = audSlice
		}
	}
	if iss, ok := payload[constants.ClaimIss].(string); ok {
		response.Iss = iss
	}
	if jti, ok := payload["jti"].(string); ok {
		response.Jti = jti
	}

	return response
}
