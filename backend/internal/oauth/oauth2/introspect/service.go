/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package introspect provides functionality for the OAuth2 token introspection endpoint
package introspect

import (
	"context"
	"errors"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// TokenIntrospectionServiceInterface defines the interface for OAuth 2.0 token introspection.
type TokenIntrospectionServiceInterface interface {
	IntrospectToken(ctx context.Context, token, tokenTypeHint string) (*IntrospectResponse, error)
}

// tokenIntrospectionService implements the TokenIntrospectionServiceInterface.
type tokenIntrospectionService struct {
	jwtService jwt.JWTServiceInterface
}

// newTokenIntrospectionService creates a new tokenIntrospectionService instance (internal use).
func newTokenIntrospectionService(jwtService jwt.JWTServiceInterface) TokenIntrospectionServiceInterface {
	return &tokenIntrospectionService{
		jwtService: jwtService,
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

	if !s.validateToken(logger, token) {
		return &IntrospectResponse{
			Active: false,
		}, nil
	}

	_, payload, err := jwt.DecodeJWT(token)
	if err != nil {
		logger.Debug("Failed to decode JWT", log.Error(err))
		return &IntrospectResponse{
			Active: false,
		}, nil
	}

	// TODO: Add validations for token revocation and validity to be used by the resource server
	//  who makes the introspection call when the support is implemented.

	return s.prepareValidResponse(payload), nil
}

// validateToken verifies the signature and validity of the token.
func (s *tokenIntrospectionService) validateToken(logger *log.Logger, token string) bool {
	if err := s.jwtService.VerifyJWT(token, "", ""); err != nil {
		logger.Debug("Failed to verify refresh token", log.String("error", err.Error.DefaultValue))
		return false
	}
	return true
}

// prepareValidResponse prepares the response for a valid token introspection.
func (s *tokenIntrospectionService) prepareValidResponse(payload map[string]interface{}) *IntrospectResponse {
	response := &IntrospectResponse{
		Active: true,
		// TODO: Revisit if/when adding support for other token types.
		TokenType: constants.TokenTypeBearer,
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
