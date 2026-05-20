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
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/idp"
)

// githubOAuthExecutor implements the OAuth authentication executor for GitHub.
type githubOAuthExecutor struct {
	oAuthExecutorInterface
	githubAuthService authngithub.GithubOAuthAuthnServiceInterface
}

var _ core.ExecutorInterface = (*githubOAuthExecutor)(nil)

// newGithubOAuthExecutor creates a new instance of GithubOAuthExecutor with the provided details.
func newGithubOAuthExecutor(
	flowFactory core.FlowFactoryInterface,
	idpService idp.IDPServiceInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
	authService authngithub.GithubOAuthAuthnServiceInterface,
	authnProvider authnprovidermgr.AuthnProviderManagerInterface,
) oAuthExecutorInterface {
	oauthSvcCast, ok := authService.(authnoauth.OAuthAuthnCoreServiceInterface)
	if !ok {
		panic("failed to cast GithubOAuthAuthnService to OAuthAuthnCoreServiceInterface")
	}

	base := newOAuthExecutor(ExecutorNameGitHubAuth, []common.Input{}, []common.Input{},
		flowFactory, idpService, entityTypeService, oauthSvcCast, authnProvider, idp.IDPTypeGitHub)

	return &githubOAuthExecutor{
		oAuthExecutorInterface: base,
		githubAuthService:      authService,
	}
}
