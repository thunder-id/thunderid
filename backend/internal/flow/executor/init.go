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
	"github.com/asgardeo/thunder/internal/attributecache"
	"github.com/asgardeo/thunder/internal/authn/assert"
	"github.com/asgardeo/thunder/internal/authn/consent"
	"github.com/asgardeo/thunder/internal/authn/github"
	"github.com/asgardeo/thunder/internal/authn/google"
	"github.com/asgardeo/thunder/internal/authn/oauth"
	"github.com/asgardeo/thunder/internal/authn/oidc"
	"github.com/asgardeo/thunder/internal/authn/otp"
	"github.com/asgardeo/thunder/internal/authn/passkey"
	authnprovidermgr "github.com/asgardeo/thunder/internal/authnprovider/manager"
	"github.com/asgardeo/thunder/internal/authz"
	"github.com/asgardeo/thunder/internal/entityprovider"
	"github.com/asgardeo/thunder/internal/flow/common"
	"github.com/asgardeo/thunder/internal/flow/core"
	"github.com/asgardeo/thunder/internal/group"
	"github.com/asgardeo/thunder/internal/idp"
	"github.com/asgardeo/thunder/internal/notification"
	"github.com/asgardeo/thunder/internal/ou"
	"github.com/asgardeo/thunder/internal/role"
	"github.com/asgardeo/thunder/internal/system/email"
	"github.com/asgardeo/thunder/internal/system/jose/jwt"
	"github.com/asgardeo/thunder/internal/system/observability"
	"github.com/asgardeo/thunder/internal/system/template"

	"github.com/asgardeo/thunder/internal/entitytype"
)

// Initialize registers available executors and returns the executor registry.
func Initialize(
	flowFactory core.FlowFactoryInterface,
	ouService ou.OrganizationUnitServiceInterface,
	idpService idp.IDPServiceInterface,
	notifSenderSvc notification.NotificationSenderServiceInterface,
	jwtService jwt.JWTServiceInterface,
	authAssertGen assert.AuthAssertGeneratorInterface,
	consentEnforcer consent.ConsentEnforcerServiceInterface,
	authnProvider authnprovidermgr.AuthnProviderManagerInterface,
	otpService otp.OTPAuthnServiceInterface,
	passkeyService passkey.PasskeyServiceInterface,
	authZService authz.AuthorizationServiceInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
	observabilitySvc observability.ObservabilityServiceInterface,
	groupService group.GroupServiceInterface,
	roleService role.RoleServiceInterface,
	entityProvider entityprovider.EntityProviderInterface,
	attributeCacheSvc attributecache.AttributeCacheServiceInterface,
	emailClient email.EmailClientInterface,
	templateService template.TemplateServiceInterface,
	oauthSvc oauth.OAuthAuthnServiceInterface,
	oidcSvc oidc.OIDCAuthnServiceInterface,
	githubSvc github.GithubOAuthAuthnServiceInterface,
	googleSvc google.GoogleOIDCAuthnServiceInterface,
) ExecutorRegistryInterface {
	reg := newExecutorRegistry()
	reg.RegisterExecutor(ExecutorNameBasicAuth, newBasicAuthExecutor(
		flowFactory, entityProvider, authnProvider))
	reg.RegisterExecutor(ExecutorNameSMSAuth, newSMSOTPAuthExecutor(
		flowFactory, otpService, authnProvider, entityProvider))
	reg.RegisterExecutor(ExecutorNamePasskeyAuth, newPasskeyAuthExecutor(
		flowFactory, passkeyService, authnProvider, entityProvider))

	reg.RegisterExecutor(ExecutorNameOAuth, newOAuthExecutor(
		"", []common.Input{}, []common.Input{}, flowFactory, idpService, entityTypeService,
		oauthSvc, authnProvider, idp.IDPTypeOAuth))
	reg.RegisterExecutor(ExecutorNameOIDCAuth, newOIDCAuthExecutor(
		"", []common.Input{}, []common.Input{}, flowFactory, idpService, entityTypeService,
		oidcSvc, authnProvider, idp.IDPTypeOIDC))
	reg.RegisterExecutor(ExecutorNameGitHubAuth, newGithubOAuthExecutor(
		flowFactory, idpService, entityTypeService, githubSvc, authnProvider))
	reg.RegisterExecutor(ExecutorNameGoogleAuth, newGoogleOIDCAuthExecutor(
		flowFactory, idpService, entityTypeService, googleSvc, authnProvider))

	reg.RegisterExecutor(ExecutorNameProvisioning, newProvisioningExecutor(flowFactory,
		groupService, roleService, entityProvider, entityTypeService, authnProvider))
	reg.RegisterExecutor(ExecutorNameOUCreation, newOUExecutor(flowFactory, ouService))

	reg.RegisterExecutor(ExecutorNameAttributeCollect, newAttributeCollector(
		flowFactory, entityProvider, authnProvider))
	reg.RegisterExecutor(ExecutorNameAuthAssert, newAuthAssertExecutor(flowFactory, jwtService,
		ouService, authAssertGen, authnProvider, entityProvider,
		attributeCacheSvc, roleService))
	reg.RegisterExecutor(ExecutorNameAuthorization, newAuthorizationExecutor(
		flowFactory, authZService, entityProvider, authnProvider))
	reg.RegisterExecutor(ExecutorNameHTTPRequest, newHTTPRequestExecutor(flowFactory, ouService))
	reg.RegisterExecutor(ExecutorNameUserTypeResolver, newUserTypeResolver(flowFactory, entityTypeService, ouService))
	reg.RegisterExecutor(ExecutorNameInviteExecutor, newInviteExecutor(flowFactory))
	reg.RegisterExecutor(ExecutorNameEmailExecutor, newEmailExecutor(
		flowFactory, emailClient, templateService, entityProvider))
	reg.RegisterExecutor(ExecutorNameCredentialSetter, newCredentialSetter(flowFactory, entityProvider))
	reg.RegisterExecutor(ExecutorNamePermissionValidator, newPermissionValidator(flowFactory))
	reg.RegisterExecutor(ExecutorNameIdentifying, newIdentifyingExecutor(
		"", []common.Input{{Identifier: userAttributeUsername, Type: "string", Required: true}}, []common.Input{},
		flowFactory, entityProvider))
	reg.RegisterExecutor(ExecutorNameConsent, newConsentExecutor(flowFactory, consentEnforcer, authnProvider))
	reg.RegisterExecutor(ExecutorNameOUResolver, newOUResolverExecutor(flowFactory, ouService))
	reg.RegisterExecutor(ExecutorNameAttributeUniquenessValidator, newAttributeUniquenessValidator(
		flowFactory, entityTypeService, entityProvider))
	reg.RegisterExecutor(ExecutorNameSMSExecutor, newSMSExecutor(flowFactory, notifSenderSvc, templateService))
	reg.RegisterExecutor(ExecutorNameFederatedAuthResolver,
		newFederatedAuthResolverExecutor(flowFactory, authnProvider))

	return reg
}
