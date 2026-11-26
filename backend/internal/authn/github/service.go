// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package github implements an authentication service for authentication via GitHub OAuth.
package github

import (
	"context"
	"slices"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	authnoauth "github.com/thunder-id/thunderid/internal/authn/oauth"
	syshttp "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const (
	loggerComponentName      = "GithubAuthnService"
	teamsClaim               = "teams"
	defaultUserTeamsEndpoint = "https://api.github.com/user/teams"
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
	ctx context.Context, idpID string) (string, map[string]string, *tidcommon.ServiceError) {
	return g.internal.BuildAuthorizeURL(ctx, idpID)
}

// ExchangeCodeForToken exchanges the authorization code for a token with GitHub.
func (g *githubOAuthAuthnService) ExchangeCodeForToken(ctx context.Context, idpID, code string, validateResponse bool) (
	*authnoauth.TokenResponse, *tidcommon.ServiceError) {
	return g.internal.ExchangeCodeForToken(ctx, idpID, code, validateResponse)
}

// FetchUserInfo retrieves user information from the Github API, resolving email and team claims when configured.
func (g *githubOAuthAuthnService) FetchUserInfo(ctx context.Context, idpID, accessToken string) (
	map[string]interface{}, *tidcommon.ServiceError) {
	logger := g.logger
	oAuthClientConfig, svcErr := g.internal.GetOAuthClientConfig(ctx, idpID)
	if svcErr != nil {
		return nil, svcErr
	}

	userInfo, svcErr := g.internal.FetchUserInfoWithClientConfig(ctx, oAuthClientConfig, accessToken)
	if svcErr != nil {
		return userInfo, svcErr
	}

	// Fetch primary email if absent and an email-capable scope was requested.
	email := authnoauth.GetStringUserClaimValue(userInfo, "email")
	if email == "" && g.shouldFetchEmail(oAuthClientConfig.Scopes) {
		primaryEmail, svcErr := g.fetchPrimaryEmail(ctx, oAuthClientConfig, accessToken)
		if svcErr != nil {
			return nil, svcErr
		}
		if primaryEmail != "" {
			userInfo["email"] = primaryEmail
		}
	} else {
		logger.Debug(ctx, "Email is already present in the user info or email scope not requested")
	}

	if g.shouldFetchTeams(oAuthClientConfig.Scopes) {
		teams, svcErr := g.fetchUserTeams(ctx, oAuthClientConfig, accessToken)
		if svcErr != nil {
			return nil, svcErr
		}
		userInfo[teamsClaim] = teams
	}

	authnoauth.ProcessSubClaim(userInfo)
	return userInfo, nil
}

// shouldFetchEmail check whether user email should be fetched from the emails endpoint based on the scopes.
func (g *githubOAuthAuthnService) shouldFetchEmail(scopes []string) bool {
	return slices.Contains(scopes, UserScope) || slices.Contains(scopes, UserEmailScope)
}

// shouldFetchTeams reports whether user team memberships should be fetched based on the scopes.
func (g *githubOAuthAuthnService) shouldFetchTeams(scopes []string) bool {
	return slices.Contains(scopes, ReadOrgScope)
}

// fetchUserTeams fetches all user team memberships from the GitHub user teams endpoint.
func (g *githubOAuthAuthnService) fetchUserTeams(ctx context.Context,
	oAuthClientConfig *authnoauth.OAuthClientConfig, accessToken string) (
	[]string, *tidcommon.ServiceError) {
	logger := g.logger
	logger.Debug(ctx, "Fetching user teams from GitHub user teams endpoint")

	nextURL := oAuthClientConfig.OAuthEndpoints.UserTeamsEndpoint
	if nextURL == "" {
		nextURL = defaultUserTeamsEndpoint
	}

	teams := []string{}
	visited := make(map[string]bool)
	for nextURL != "" && !visited[nextURL] {
		visited[nextURL] = true

		req, svcErr := buildUserTeamsRequest(ctx, nextURL, accessToken, logger)
		if svcErr != nil {
			return nil, svcErr
		}

		pageTeams, next, svcErr := sendUserTeamsRequest(req, g.httpClient, logger)
		if svcErr != nil {
			return nil, svcErr
		}

		for _, team := range pageTeams {
			if team.Organization.Login == "" || team.Slug == "" {
				continue
			}
			teams = append(teams, team.Organization.Login+"/"+team.Slug)
		}

		nextURL = next
	}

	return teams, nil
}

// fetchPrimaryEmail fetches the primary email of the user from the GitHub user emails endpoint.
func (g *githubOAuthAuthnService) fetchPrimaryEmail(ctx context.Context,
	oAuthClientConfig *authnoauth.OAuthClientConfig, accessToken string) (
	string, *tidcommon.ServiceError) {
	logger := g.logger
	logger.Debug(ctx, "Fetching primary email from GitHub user emails endpoint")

	if oAuthClientConfig.OAuthEndpoints.UserEmailEndpoint == "" {
		logger.Error(ctx, "User email endpoint is not configured in OAuth client config")
		return "", &tidcommon.InternalServerError
	}

	req, svcErr := buildUserEmailRequest(ctx, oAuthClientConfig.OAuthEndpoints.UserEmailEndpoint, accessToken, logger)
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

// GetOAuthClientConfig retrieves and validates the OAuth client configuration for the given identity provider ID.
func (g *githubOAuthAuthnService) GetOAuthClientConfig(ctx context.Context, idpID string) (
	*authnoauth.OAuthClientConfig, *tidcommon.ServiceError) {
	return g.internal.GetOAuthClientConfig(ctx, idpID)
}

// Authenticate performs the full GitHub OAuth authentication flow: exchanges the code for a token,
// fetches user info, and builds the federated token. The entity is resolved later through recorded links.
func (g *githubOAuthAuthnService) Authenticate(ctx context.Context, idpID string,
	authzData authncm.AuthorizationData) (*authncm.AuthnResult, *tidcommon.ServiceError) {
	logger := g.logger.With(log.String(log.LoggerKeyIDPID, idpID))
	logger.Debug(ctx, "Performing federated GitHub OAuth authentication")

	tokenResp, svcErr := g.ExchangeCodeForToken(ctx, idpID, authzData.Code, true)
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
		logger.Debug(ctx, "sub claim not found in user info")
		return nil, &authncm.ErrorSubClaimNotFound
	}

	return g.internal.BuildFederatedAuthResult(ctx, idpID, sub, userInfo)
}

// BuildFederatedAuthResult delegates to the underlying OAuth service, which applies attribute mapping
// uniformly for all federated authenticators.
func (g *githubOAuthAuthnService) BuildFederatedAuthResult(ctx context.Context, idpID, sub string,
	claims map[string]interface{}) (*authncm.AuthnResult, *tidcommon.ServiceError) {
	return g.internal.BuildFederatedAuthResult(ctx, idpID, sub, claims)
}
