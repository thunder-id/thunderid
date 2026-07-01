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

package executor

import (
	authngithub "github.com/thunder-id/thunderid/internal/authn/github"
	authnoauth "github.com/thunder-id/thunderid/internal/authn/oauth"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/idp"
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
	idpService idp.IDPServiceInterface,
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
