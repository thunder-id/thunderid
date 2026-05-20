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

// Package granthandlers provides an interface and implementations for handling OAuth 2.0 grant types.
package granthandlers

import (
	"context"

	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
)

// GrantHandlerInterface defines the interface for handling OAuth 2.0 grants.
type GrantHandlerInterface interface {
	ValidateGrant(
		ctx context.Context,
		tokenRequest *model.TokenRequest,
		oauthApp *inboundmodel.OAuthClient,
	) *model.ErrorResponse
	HandleGrant(ctx context.Context, tokenRequest *model.TokenRequest, oauthApp *inboundmodel.OAuthClient) (
		*model.TokenResponseDTO, *model.ErrorResponse)
}

// RefreshTokenGrantHandlerInterface defines the interface for handling refresh token grants.
type RefreshTokenGrantHandlerInterface interface {
	GrantHandlerInterface
	IssueRefreshToken(
		ctx context.Context,
		tokenResponse *model.TokenResponseDTO,
		oauthApp *inboundmodel.OAuthClient,
		subject string, audiences []string, grantType string,
		scopes []string,
		claimsRequest *model.ClaimsRequest,
		claimsLocales string,
		attributeCacheID string,
	) *model.ErrorResponse
}
