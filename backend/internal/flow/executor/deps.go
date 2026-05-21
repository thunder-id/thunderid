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

package executor

import (
	"github.com/thunder-id/thunderid/internal/attributecache"
	"github.com/thunder-id/thunderid/internal/authn/assert"
	"github.com/thunder-id/thunderid/internal/authn/consent"
	"github.com/thunder-id/thunderid/internal/authn/github"
	"github.com/thunder-id/thunderid/internal/authn/google"
	"github.com/thunder-id/thunderid/internal/authn/magiclink"
	"github.com/thunder-id/thunderid/internal/authn/oauth"
	"github.com/thunder-id/thunderid/internal/authn/oidc"
	"github.com/thunder-id/thunderid/internal/authn/otp"
	"github.com/thunder-id/thunderid/internal/authn/passkey"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/authz"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/notification"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/system/email"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/template"
)

// ExecutorDeps bundles dependencies required to register built-in executors.
type ExecutorDeps struct {
	FlowFactory           core.FlowFactoryInterface
	OUService             ou.OrganizationUnitServiceInterface
	IDPService            idp.IDPServiceInterface
	NotifSenderSvc        notification.NotificationSenderServiceInterface
	JWTService            jwt.JWTServiceInterface
	AuthAssertGen         assert.AuthAssertGeneratorInterface
	ConsentEnforcer       consent.ConsentEnforcerServiceInterface
	AuthnProvider         authnprovidermgr.AuthnProviderManagerInterface
	OTPService            otp.OTPAuthnServiceInterface
	PasskeyService        passkey.PasskeyServiceInterface
	MagicLinkService      magiclink.MagicLinkAuthnServiceInterface
	AuthZService          authz.AuthorizationServiceInterface
	EntityTypeService     entitytype.EntityTypeServiceInterface
	GroupService          group.GroupServiceInterface
	RoleService           role.RoleServiceInterface
	RoleAssignmentService role.RoleAssignmentServiceInterface
	EntityProvider        entityprovider.EntityProviderInterface
	AttributeCacheSvc     attributecache.AttributeCacheServiceInterface
	EmailClient           email.EmailClientInterface
	TemplateService       template.TemplateServiceInterface
	OAuthSvc              oauth.OAuthAuthnServiceInterface
	OIDCSvc               oidc.OIDCAuthnServiceInterface
	GithubSvc             github.GithubOAuthAuthnServiceInterface
	GoogleSvc             google.GoogleOIDCAuthnServiceInterface
}
