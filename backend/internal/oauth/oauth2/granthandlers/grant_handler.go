// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package granthandlers provides an interface and implementations for handling OAuth 2.0 grant types.
package granthandlers

import (
	"context"
	"errors"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// GrantHandlerInterface defines the interface for handling OAuth 2.0 grants.
type GrantHandlerInterface interface {
	ValidateGrant(
		ctx context.Context,
		tokenRequest *model.TokenRequest,
		oauthApp *providers.OAuthClient,
	) *model.ErrorResponse
	HandleGrant(ctx context.Context, tokenRequest *model.TokenRequest, oauthApp *providers.OAuthClient) (
		*model.TokenResponseDTO, *model.ErrorResponse)
}

// RefreshTokenGrantHandlerInterface defines the interface for handling refresh token grants.
type RefreshTokenGrantHandlerInterface interface {
	GrantHandlerInterface
	IssueRefreshToken(
		ctx context.Context,
		tokenResponse *model.TokenResponseDTO,
		oauthApp *providers.OAuthClient,
		subject string, audiences []string, grantType string,
		scopes []string,
		claimsRequest *model.ClaimsRequest,
		claimsLocales string,
		attributeCacheID string,
		tokenFamilyID string,
		expiresAt int64,
	) *model.ErrorResponse
}

// tokenBuildErrorResponse returns the response for a token the builder could not build. A token
// refused because the lifespan of an agent it is issued to or for has ended answers invalid_grant,
// without saying why; any other failure is a server error with the given description.
func tokenBuildErrorResponse(err error, description string) *model.ErrorResponse {
	if errors.Is(err, tokenservice.ErrLifespanEnded) {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidGrant,
			ErrorDescription: "The authorization grant can no longer be used to issue tokens",
		}
	}
	return &model.ErrorResponse{
		Error:            constants.ErrorServerError,
		ErrorDescription: description,
	}
}
