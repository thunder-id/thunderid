// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package introspect provides functionality for the OAuth2 token introspection endpoint
package introspect

import (
	"context"
	"errors"
	"fmt"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dpop"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/revocation"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// TokenIntrospectionServiceInterface defines the interface for OAuth 2.0 token introspection.
type TokenIntrospectionServiceInterface interface {
	IntrospectToken(ctx context.Context, token, tokenTypeHint string) (*IntrospectResponse, error)
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
	ctx context.Context, token, tokenTypeHint string,
) (*IntrospectResponse, error) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "TokenIntrospectionService"))

	if token == "" {
		return nil, errors.New("token is required")
	}

	// RFC 7662 Section 2.1 scopes introspection to access and refresh tokens, so anything else this
	// server signs (ID tokens, flow assertions) is reported inactive.
	payload, err := s.validateByType(ctx, token)
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

	return s.prepareValidResponse(payload), nil
}

// validateByType validates the token with the validator for its typ header and returns its claims.
func (s *tokenIntrospectionService) validateByType(
	ctx context.Context, token string,
) (map[string]interface{}, error) {
	header, err := jwt.DecodeJWTHeader(token)
	if err != nil {
		return nil, fmt.Errorf("failed to decode token header: %w", err)
	}

	switch typ, _ := header["typ"].(string); typ {
	case jwt.TokenTypeAccessToken:
		claims, validateErr := s.tokenValidator.ValidateAccessToken(ctx, token)
		if validateErr != nil {
			return nil, validateErr
		}
		return claims.Claims, nil
	case jwt.TokenTypeJWT:
		claims, validateErr := s.tokenValidator.ValidateRefreshToken(ctx, token)
		if validateErr != nil {
			return nil, validateErr
		}
		return claims.Claims, nil
	default:
		return nil, fmt.Errorf("token type %q is not introspectable", typ)
	}
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
	if subType, ok := payload[constants.ClaimSubType].(string); ok {
		response.SubType = subType
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
