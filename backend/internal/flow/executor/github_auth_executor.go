// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	authngithub "github.com/thunder-id/thunderid/internal/authn/github"
	authnoauth "github.com/thunder-id/thunderid/internal/authn/oauth"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// githubOAuthExecutor implements the OAuth authentication executor for GitHub.
type githubOAuthExecutor struct {
	oAuthExecutorInterface
	githubAuthService authngithub.GithubOAuthAuthnServiceInterface
}

var _ providers.Executor = (*githubOAuthExecutor)(nil)

// newGithubOAuthExecutor creates a new instance of GithubOAuthExecutor with the provided details.
func newGithubOAuthExecutor(
	flowFactory core.FlowFactoryInterface,
	idpService providers.IDPProvider,
	authService authngithub.GithubOAuthAuthnServiceInterface,
	authnProvider providers.AuthnProviderManager,
) oAuthExecutorInterface {
	oauthSvcCast, ok := authService.(authnoauth.OAuthAuthnCoreServiceInterface)
	if !ok {
		panic("failed to cast GithubOAuthAuthnService to OAuthAuthnCoreServiceInterface")
	}

	base := newOAuthExecutor(ExecutorNameGitHubAuth, []providers.Input{}, []providers.Input{},
		flowFactory, idpService, oauthSvcCast, authnProvider, providers.IDPTypeGitHub)

	return &githubOAuthExecutor{
		oAuthExecutorInterface: base,
		githubAuthService:      authService,
	}
}
