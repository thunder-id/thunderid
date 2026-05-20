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

// Package github implements an authentication service for authentication via GitHub OAuth.
package github

import (
	"context"
	"slices"

	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	authnoauth "github.com/thunder-id/thunderid/internal/authn/oauth"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	syshttp "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const (
	loggerComponentName = "GithubAuthnService"
)

// GithubOAuthAuthnServiceInterface defines the contract for GitHub OAuth based authenticator services.
type GithubOAuthAuthnServiceInterface interface {
	authnoauth.OAuthAuthnCoreServiceInterface
}

// githubOAuthAuthnService is the default implementation of GithubOAuthAuthnServiceInterface.
type githubOAuthAuthnService struct {
	internal   authnoauth.OAuthAuthnServiceInterface
	httpClient syshttp.HTTPClientInterface
	logger     *log.Logger
}

// newGithubOAuthAuthnService creates a new instance of GitHub OAuth authenticator service.
func newGithubOAuthAuthnService(internal authnoauth.OAuthAuthnServiceInterface,
	httpClient syshttp.HTTPClientInterface) GithubOAuthAuthnServiceInterface {
	return &githubOAuthAuthnService{
		internal:   internal,
		httpClient: httpClient,
		logger:     log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName)),
	}
}

// BuildAuthorizeURL constructs the authorization request URL for GitHub OAuth authentication.
func (g *githubOAuthAuthnService) BuildAuthorizeURL(
	ctx context.Context, idpID string) (string, *serviceerror.ServiceError) {
	return g.internal.BuildAuthorizeURL(ctx, idpID)
}

// ExchangeCodeForToken exchanges the authorization code for a token with GitHub.
func (g *githubOAuthAuthnService) ExchangeCodeForToken(ctx context.Context, idpID, code string, validateResponse bool) (
	*authnoauth.TokenResponse, *serviceerror.ServiceError) {
	return g.internal.ExchangeCodeForToken(ctx, idpID, code, validateResponse)
}

// FetchUserInfo retrieves user information from the Github API, ensuring email resolution if necessary.
func (g *githubOAuthAuthnService) FetchUserInfo(ctx context.Context, idpID, accessToken string) (
	map[string]interface{}, *serviceerror.ServiceError) {
	logger := g.logger
	oAuthClientConfig, svcErr := g.internal.GetOAuthClientConfig(ctx, idpID)
	if svcErr != nil {
		return nil, svcErr
	}

	userInfo, svcErr := g.internal.FetchUserInfoWithClientConfig(oAuthClientConfig, accessToken)
	if svcErr != nil {
		return userInfo, svcErr
	}

	// If email is already present in the user info or email scope is not requested, return it directly.
	email := authnoauth.GetStringUserClaimValue(userInfo, "email")
	if email != "" || !g.shouldFetchEmail(oAuthClientConfig.Scopes) {
		logger.Debug("Email is already present in the user info or email scope not requested")
		authnoauth.ProcessSubClaim(userInfo)
		return userInfo, nil
	}

	// Fetch primary email from the GitHub user emails endpoint.
	primaryEmail, svcErr := g.fetchPrimaryEmail(oAuthClientConfig, accessToken)
	if svcErr != nil {
		return nil, svcErr
	}
	if primaryEmail != "" {
		userInfo["email"] = primaryEmail
	}

	authnoauth.ProcessSubClaim(userInfo)
	return userInfo, nil
}

// shouldFetchEmail check whether user email should be fetched from the emails endpoint based on the scopes.
func (g *githubOAuthAuthnService) shouldFetchEmail(scopes []string) bool {
	return slices.Contains(scopes, UserScope) || slices.Contains(scopes, UserEmailScope)
}

// fetchPrimaryEmail fetches the primary email of the user from the GitHub user emails endpoint.
func (g *githubOAuthAuthnService) fetchPrimaryEmail(
	oAuthClientConfig *authnoauth.OAuthClientConfig, accessToken string) (
	string, *serviceerror.ServiceError) {
	logger := g.logger
	logger.Debug("Fetching primary email from GitHub user emails endpoint")

	if oAuthClientConfig.OAuthEndpoints.UserEmailEndpoint == "" {
		logger.Error("User email endpoint is not configured in OAuth client config")
		return "", &serviceerror.InternalServerError
	}

	req, svcErr := buildUserEmailRequest(oAuthClientConfig.OAuthEndpoints.UserEmailEndpoint, accessToken, logger)
	if svcErr != nil {
		return "", svcErr
	}

	emails, svcErr := sendUserEmailRequest(req, g.httpClient, logger)
	if svcErr != nil {
		return "", svcErr
	}

	for _, emailEntry := range emails {
		if isPrimary, ok := emailEntry["primary"].(bool); ok && isPrimary {
			if primaryEmail, ok := emailEntry["email"].(string); ok {
				return primaryEmail, nil
			}
		}
	}

	return "", nil
}

// GetInternalUser retrieves the internal user based on the external subject identifier.
func (g *githubOAuthAuthnService) GetInternalUser(sub string) (*entityprovider.Entity, *serviceerror.ServiceError) {
	return g.internal.GetInternalUser(sub)
}

// GetOAuthClientConfig retrieves and validates the OAuth client configuration for the given identity provider ID.
func (g *githubOAuthAuthnService) GetOAuthClientConfig(ctx context.Context, idpID string) (
	*authnoauth.OAuthClientConfig, *serviceerror.ServiceError) {
	return g.internal.GetOAuthClientConfig(ctx, idpID)
}

// Authenticate performs the full GitHub OAuth authentication flow: exchanges the code for a token,
// fetches user info, and resolves the internal user.
// A missing internal user is NOT an error — the caller decides how to handle it.
func (g *githubOAuthAuthnService) Authenticate(ctx context.Context, idpID, code string) (
	*authncm.FederatedAuthResult, *serviceerror.ServiceError) {
	logger := g.logger.With(log.String("idpId", idpID))
	logger.Debug("Performing federated GitHub OAuth authentication")

	tokenResp, svcErr := g.ExchangeCodeForToken(ctx, idpID, code, true)
	if svcErr != nil {
		return nil, svcErr
	}

	userInfo, svcErr := g.FetchUserInfo(ctx, idpID, tokenResp.AccessToken)
	if svcErr != nil {
		return nil, svcErr
	}

	sub := ""
	if subVal, ok := userInfo["sub"]; ok && subVal != nil {
		if subStr, ok := subVal.(string); ok && subStr != "" {
			sub = subStr
		}
	}
	if sub == "" {
		logger.Debug("sub claim not found in user info")
		return nil, &authncm.ErrorSubClaimNotFound
	}

	result := &authncm.FederatedAuthResult{
		Sub:    sub,
		Claims: userInfo,
	}
	user, svcErr := g.GetInternalUser(sub)
	if svcErr != nil {
		if svcErr.Code == authncm.ErrorUserNotFound.Code {
			return result, nil
		}
		if svcErr.Code == authncm.ErrorAmbiguousUser.Code {
			result.IsAmbiguousUser = true
			return result, nil
		}
		return nil, svcErr
	}
	result.InternalEntity = user
	return result, nil
}
