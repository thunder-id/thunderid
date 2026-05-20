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
	authngoogle "github.com/thunder-id/thunderid/internal/authn/google"
	authnoidc "github.com/thunder-id/thunderid/internal/authn/oidc"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/idp"
)

// googleOIDCAuthExecutor implements the OIDC authentication executor for Google.
type googleOIDCAuthExecutor struct {
	oidcAuthExecutorInterface
	googleAuthService authngoogle.GoogleOIDCAuthnServiceInterface
}

var _ core.ExecutorInterface = (*googleOIDCAuthExecutor)(nil)

// newGoogleOIDCAuthExecutor creates a new instance of GoogleOIDCAuthExecutor with the provided details.
func newGoogleOIDCAuthExecutor(
	flowFactory core.FlowFactoryInterface,
	idpService idp.IDPServiceInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
	authService authngoogle.GoogleOIDCAuthnServiceInterface,
	authnProvider authnprovidermgr.AuthnProviderManagerInterface,
) oidcAuthExecutorInterface {
	defaultInputs := []common.Input{
		{
			Identifier: "code",
			Type:       "string",
			Required:   true,
		},
		{
			Identifier: "nonce",
			Type:       "string",
			Required:   false,
		},
	}

	oidcSvcCast, ok := authService.(authnoidc.OIDCAuthnCoreServiceInterface)
	if !ok {
		panic("failed to cast GoogleOIDCAuthnService to OIDCAuthnCoreServiceInterface")
	}

	base := newOIDCAuthExecutor(ExecutorNameGoogleAuth, defaultInputs, []common.Input{},
		flowFactory, idpService, entityTypeService, oidcSvcCast, authnProvider, idp.IDPTypeGoogle)

	return &googleOIDCAuthExecutor{
		oidcAuthExecutorInterface: base,
		googleAuthService:         authService,
	}
}
