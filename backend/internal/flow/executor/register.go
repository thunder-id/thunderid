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
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/idp"
)

// DefaultExecutorNames returns the full built-in executor name list.
func DefaultExecutorNames() []string {
	return []string{
		ExecutorNameBasicAuth,
		ExecutorNameSMSAuth,
		ExecutorNamePasskeyAuth,
		ExecutorNameMagicLinkAuth,
		ExecutorNameOAuth,
		ExecutorNameOIDCAuth,
		ExecutorNameGitHubAuth,
		ExecutorNameGoogleAuth,
		ExecutorNameProvisioning,
		ExecutorNameOUCreation,
		ExecutorNameAttributeCollect,
		ExecutorNameAuthAssert,
		ExecutorNameAuthorization,
		ExecutorNameHTTPRequest,
		ExecutorNameUserTypeResolver,
		ExecutorNameInviteExecutor,
		ExecutorNameEmailExecutor,
		ExecutorNameCredentialSetter,
		ExecutorNamePermissionValidator,
		ExecutorNameIdentifying,
		ExecutorNameConsent,
		ExecutorNameOUResolver,
		ExecutorNameAttributeUniquenessValidator,
		ExecutorNameSMSExecutor,
		ExecutorNameFederatedAuthResolver,
	}
}

func shouldRegisterExecutor(name string, names []string) bool {
	if len(names) == 0 {
		return true
	}
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}

// RegisterDefaultExecutors registers built-in executors on reg. When names is empty, all defaults are registered.
func RegisterDefaultExecutors(reg ExecutorRegistryInterface, deps ExecutorDeps, names []string) {
	if shouldRegisterExecutor(ExecutorNameBasicAuth, names) {
		reg.RegisterExecutor(ExecutorNameBasicAuth, newBasicAuthExecutor(
			deps.FlowFactory, deps.EntityProvider, deps.AuthnProvider))
	}
	if shouldRegisterExecutor(ExecutorNameSMSAuth, names) {
		reg.RegisterExecutor(ExecutorNameSMSAuth, newSMSOTPAuthExecutor(
			deps.FlowFactory, deps.OTPService, deps.AuthnProvider, deps.EntityProvider))
	}
	if shouldRegisterExecutor(ExecutorNamePasskeyAuth, names) {
		reg.RegisterExecutor(ExecutorNamePasskeyAuth, newPasskeyAuthExecutor(
			deps.FlowFactory, deps.PasskeyService, deps.AuthnProvider, deps.EntityProvider))
	}
	if shouldRegisterExecutor(ExecutorNameMagicLinkAuth, names) {
		reg.RegisterExecutor(ExecutorNameMagicLinkAuth, newMagicLinkAuthExecutor(
			deps.FlowFactory, deps.MagicLinkService, deps.EntityProvider))
	}
	if shouldRegisterExecutor(ExecutorNameOAuth, names) {
		reg.RegisterExecutor(ExecutorNameOAuth, newOAuthExecutor(
			"", []common.Input{}, []common.Input{}, deps.FlowFactory, deps.IDPService, deps.EntityTypeService,
			deps.OAuthSvc, deps.AuthnProvider, idp.IDPTypeOAuth))
	}
	if shouldRegisterExecutor(ExecutorNameOIDCAuth, names) {
		reg.RegisterExecutor(ExecutorNameOIDCAuth, newOIDCAuthExecutor(
			"", []common.Input{}, []common.Input{}, deps.FlowFactory, deps.IDPService, deps.EntityTypeService,
			deps.OIDCSvc, deps.AuthnProvider, idp.IDPTypeOIDC))
	}
	if shouldRegisterExecutor(ExecutorNameGitHubAuth, names) {
		reg.RegisterExecutor(ExecutorNameGitHubAuth, newGithubOAuthExecutor(
			deps.FlowFactory, deps.IDPService, deps.EntityTypeService, deps.GithubSvc, deps.AuthnProvider))
	}
	if shouldRegisterExecutor(ExecutorNameGoogleAuth, names) {
		reg.RegisterExecutor(ExecutorNameGoogleAuth, newGoogleOIDCAuthExecutor(
			deps.FlowFactory, deps.IDPService, deps.EntityTypeService, deps.GoogleSvc, deps.AuthnProvider))
	}
	if shouldRegisterExecutor(ExecutorNameProvisioning, names) {
		reg.RegisterExecutor(ExecutorNameProvisioning, newProvisioningExecutor(
			deps.FlowFactory, deps.GroupService, deps.RoleService, deps.RoleAssignmentService,
			deps.EntityProvider, deps.EntityTypeService,
		))
	}
	if shouldRegisterExecutor(ExecutorNameOUCreation, names) {
		reg.RegisterExecutor(ExecutorNameOUCreation, newOUExecutor(deps.FlowFactory, deps.OUService))
	}
	if shouldRegisterExecutor(ExecutorNameAttributeCollect, names) {
		reg.RegisterExecutor(ExecutorNameAttributeCollect, newAttributeCollector(deps.FlowFactory, deps.EntityProvider))
	}
	if shouldRegisterExecutor(ExecutorNameAuthAssert, names) {
		reg.RegisterExecutor(ExecutorNameAuthAssert, newAuthAssertExecutor(deps.FlowFactory, deps.JWTService,
			deps.OUService, deps.AuthAssertGen, deps.AuthnProvider, deps.EntityProvider,
			deps.AttributeCacheSvc, deps.RoleService))
	}
	if shouldRegisterExecutor(ExecutorNameAuthorization, names) {
		reg.RegisterExecutor(ExecutorNameAuthorization, newAuthorizationExecutor(
			deps.FlowFactory, deps.AuthZService, deps.EntityProvider))
	}
	if shouldRegisterExecutor(ExecutorNameHTTPRequest, names) {
		reg.RegisterExecutor(ExecutorNameHTTPRequest, newHTTPRequestExecutor(deps.FlowFactory, deps.OUService))
	}
	if shouldRegisterExecutor(ExecutorNameUserTypeResolver, names) {
		reg.RegisterExecutor(ExecutorNameUserTypeResolver, newUserTypeResolver(
			deps.FlowFactory, deps.EntityTypeService, deps.OUService))
	}
	if shouldRegisterExecutor(ExecutorNameInviteExecutor, names) {
		reg.RegisterExecutor(ExecutorNameInviteExecutor, newInviteExecutor(deps.FlowFactory))
	}
	if shouldRegisterExecutor(ExecutorNameEmailExecutor, names) {
		reg.RegisterExecutor(ExecutorNameEmailExecutor, newEmailExecutor(
			deps.FlowFactory, deps.EmailClient, deps.TemplateService, deps.EntityProvider))
	}
	if shouldRegisterExecutor(ExecutorNameCredentialSetter, names) {
		reg.RegisterExecutor(ExecutorNameCredentialSetter, newCredentialSetter(deps.FlowFactory, deps.EntityProvider))
	}
	if shouldRegisterExecutor(ExecutorNamePermissionValidator, names) {
		reg.RegisterExecutor(ExecutorNamePermissionValidator, newPermissionValidator(deps.FlowFactory))
	}
	if shouldRegisterExecutor(ExecutorNameIdentifying, names) {
		reg.RegisterExecutor(ExecutorNameIdentifying, newIdentifyingExecutor(
			"", []common.Input{{Identifier: userAttributeUsername, Type: "string", Required: true}}, []common.Input{},
			deps.FlowFactory, deps.EntityProvider))
	}
	if shouldRegisterExecutor(ExecutorNameConsent, names) {
		reg.RegisterExecutor(ExecutorNameConsent, newConsentExecutor(
			deps.FlowFactory, deps.ConsentEnforcer, deps.AuthnProvider,
		))
	}
	if shouldRegisterExecutor(ExecutorNameOUResolver, names) {
		reg.RegisterExecutor(ExecutorNameOUResolver, newOUResolverExecutor(deps.FlowFactory, deps.OUService))
	}
	if shouldRegisterExecutor(ExecutorNameAttributeUniquenessValidator, names) {
		reg.RegisterExecutor(ExecutorNameAttributeUniquenessValidator, newAttributeUniquenessValidator(
			deps.FlowFactory, deps.EntityTypeService, deps.EntityProvider))
	}
	if shouldRegisterExecutor(ExecutorNameSMSExecutor, names) {
		reg.RegisterExecutor(ExecutorNameSMSExecutor, newSMSExecutor(
			deps.FlowFactory, deps.NotifSenderSvc, deps.TemplateService,
		))
	}
	if shouldRegisterExecutor(ExecutorNameFederatedAuthResolver, names) {
		reg.RegisterExecutor(ExecutorNameFederatedAuthResolver, newFederatedAuthResolverExecutor(deps.FlowFactory))
	}
}
