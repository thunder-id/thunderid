/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

// Package auth provides authentication utilities for the MCP server.
package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"

	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// NewTokenVerifier creates a TokenVerifier function that verifies tokens
// issued by the OAuth server. This implements the auth.TokenVerifier
// function type from the MCP SDK.
func NewTokenVerifier(
	jwtService jwt.JWTServiceInterface,
	issuer string,
	mcpURL string,
) auth.TokenVerifier {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "MCPTokenVerifier"))

	return func(ctx context.Context, token string, req *http.Request) (*auth.TokenInfo, error) {
		// Verify JWT signature and claims (iss, aud, exp, nbf)
		if err := jwtService.VerifyJWT(token, mcpURL, issuer); err != nil {
			logger.Error("JWT verification failed", log.String("error", err.Error.DefaultValue))
			return nil, auth.ErrInvalidToken
		}

		// Decode payload to extract claims for TokenInfo
		payload, err := jwt.DecodeJWTPayload(token)
		if err != nil {
			logger.Error("Failed to decode JWT payload", log.Error(err))
			return nil, auth.ErrInvalidToken
		}

		// Extract expiration time for SDK middleware
		var expiration time.Time
		if exp, ok := payload["exp"].(float64); ok {
			expiration = time.Unix(int64(exp), 0)
		}

		// Extract scopes from token
		var scopes []string
		if scopeStr, ok := payload["scope"].(string); ok && scopeStr != "" {
			scopes = strings.Fields(scopeStr)
			logger.Debug("Token scopes extracted",
				log.String("scopes", strings.Join(scopes, ",")),
				log.String("path", req.URL.Path))
		} else {
			logger.Warn("Token missing 'scope' claim", log.String("path", req.URL.Path))
		}

		// Extract user ID from 'sub' claim
		userID := ""
		if sub, ok := payload["sub"].(string); ok && sub != "" {
			userID = sub
		}

		// Build TokenInfo with user ID, scopes, and expiration
		tokenInfo := &auth.TokenInfo{
			UserID:     userID,
			Scopes:     scopes,
			Expiration: expiration,
		}

		return tokenInfo, nil
	}
}
