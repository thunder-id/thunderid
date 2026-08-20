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

package executormeta

import (
	"fmt"
	"slices"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// catalog is the static metadata of every built-in executor, keyed by name.
//
// It mirrors what each executor reports from GetMeta at runtime, so that a server which only
// validates flow definitions can read it without constructing an executor and thereby linking the
// data-plane services those constructors need. TestCatalogMatchesRegistry in the executor package
// holds the two together: it registers the real executors and fails on any difference.
var catalog = map[string]providers.ExecutorMeta{
	ExecutorNameAttributeCollect:             {},
	ExecutorNameAttributeUniquenessValidator: {},
	ExecutorNameAuthAssert: {
		SupportedProperties: []providers.ExecutorSupportedProperties{
			{Property: propertyKeyCallbackType},
		},
	},
	ExecutorNameAuthorization: {},
	ExecutorNameConsent: {
		SupportedProperties: []providers.ExecutorSupportedProperties{
			{Property: "timeout"},
			{Property: "failOnDeny"},
		},
	},
	ExecutorNameCredentialSetter: {},
	ExecutorNameCredentialsAuth:  {},
	ExecutorNameCriteriaRevocation: {
		SupportedFlowTypes: []providers.FlowType{"ADMINISTRATION"},
	},
	ExecutorNameEmailExecutor: {
		SupportedModes: []string{ExecutorModeSend},
		SupportedProperties: []providers.ExecutorSupportedProperties{
			{Property: propertyKeyEmailTemplate, IsRequired: true},
		},
	},
	ExecutorNameFederatedAuthResolver: {},
	ExecutorNameGitHubAuth: {
		SupportedProperties: []providers.ExecutorSupportedProperties{
			{Property: "idpId", IsRequired: true},
			{Property: "allowAuthenticationWithoutLocalUser"},
			{Property: "allowRegistrationWithExistingUser"},
		},
	},
	ExecutorNameGoogleAuth: {
		SupportedProperties: []providers.ExecutorSupportedProperties{
			{Property: "idpId", IsRequired: true},
			{Property: "allowAuthenticationWithoutLocalUser"},
			{Property: "allowRegistrationWithExistingUser"},
		},
	},
	ExecutorNameHTTPRequest: {
		SupportedProperties: []providers.ExecutorSupportedProperties{
			{Property: "url", IsRequired: true},
			{Property: "method"},
			{Property: "headers"},
			{Property: "body"},
			{Property: "timeout"},
			{Property: "responseMapping"},
			{Property: "errorHandling"},
		},
	},
	ExecutorNameIdentifying: {
		DefaultMode:    ExecutorModeIdentify,
		SupportedModes: []string{ExecutorModeIdentify, ExecutorModeResolve, ExecutorModeCheckState},
		SupportedProperties: []providers.ExecutorSupportedProperties{
			{Property: propertyKeyLoginHintAttribute},
		},
	},
	ExecutorNameInviteExecutor: {
		SupportedModes: []string{ExecutorModeGenerate, passkeyExecutorModeVerify},
		SupportedProperties: []providers.ExecutorSupportedProperties{
			{Property: propertyKeyInviteBaseURL},
		},
	},
	ExecutorNameMagicLink: {
		SupportedModes: []string{ExecutorModeGenerate, passkeyExecutorModeVerify},
		SupportedProperties: []providers.ExecutorSupportedProperties{
			{Property: propertyKeyTokenExpiry},
			{Property: propertyKeyMagicLinkURL},
		},
	},
	ExecutorNameOAuth: {
		SupportedProperties: []providers.ExecutorSupportedProperties{
			{Property: "idpId", IsRequired: true},
			{Property: "allowAuthenticationWithoutLocalUser"},
			{Property: "allowRegistrationWithExistingUser"},
		},
	},
	ExecutorNameOIDCAuth: {
		SupportedProperties: []providers.ExecutorSupportedProperties{
			{Property: "idpId", IsRequired: true},
			{Property: "allowAuthenticationWithoutLocalUser"},
			{Property: "allowRegistrationWithExistingUser"},
		},
	},
	ExecutorNameOTPExecutor: {
		SupportedModes: []string{ExecutorModeGenerate, passkeyExecutorModeVerify},
		SupportedProperties: []providers.ExecutorSupportedProperties{
			{Property: propertyKeyMaxOTPAttempts},
			{Property: "otpLength"},
			{Property: "otpUseNumericOnly"},
			{Property: "otpValidityPeriodSeconds"},
		},
	},
	ExecutorNameOUCreation: {
		SupportedProperties: []providers.ExecutorSupportedProperties{
			{Property: "parentOuId"},
		},
	},
	ExecutorNameOUResolver: {
		SupportedProperties: []providers.ExecutorSupportedProperties{
			{Property: "resolveFrom"},
		},
	},
	ExecutorNameOpenID4VPVerify: {
		SupportedProperties: []providers.ExecutorSupportedProperties{
			{Property: propertyKeyPresentationDefinitionID, IsRequired: true},
			{Property: "allowAuthenticationWithoutLocalUser"},
		},
	},
	ExecutorNamePasskeyAuth: {
		SupportedModes: []string{passkeyExecutorModeChallenge, passkeyExecutorModeVerify, passkeyExecutorModeRegStart,
			passkeyExecutorModeRegFinish},
		SupportedProperties: []providers.ExecutorSupportedProperties{
			{Property: "relyingPartyId", IsRequired: true, ApplicableModes: []string{passkeyExecutorModeChallenge,
				passkeyExecutorModeRegStart}},
			{Property: "relyingPartyName"},
			{Property: "authenticatorSelection"},
			{Property: "attestation"},
		},
	},
	ExecutorNamePermissionValidator: {
		SupportedProperties: []providers.ExecutorSupportedProperties{
			{Property: propertyKeyRequiredScopes},
		},
	},
	ExecutorNamePreDelete: {
		DefaultMode:        "revoke_all",
		SupportedModes:     []string{"revoke_all"},
		SupportedFlowTypes: []providers.FlowType{"ADMINISTRATION"},
	},
	ExecutorNameProvisioning: {
		SupportedFlowTypes: []providers.FlowType{"AUTHENTICATION", "REGISTRATION", "USER_ONBOARDING"},
		SupportedProperties: []providers.ExecutorSupportedProperties{
			{Property: propertyKeyDynamicInputsIncludeOptional},
			{Property: propertyKeyDynamicInputsIncludeOptionalCredentials},
			{Property: propertyKeyMaxDynamicInputsPerPrompt},
			{Property: propertyKeyAssignGroup},
			{Property: propertyKeyAssignRole},
			{Property: "allowCrossOUProvisioning"},
		},
	},
	ExecutorNameSMSExecutor: {
		SupportedProperties: []providers.ExecutorSupportedProperties{
			{Property: propertyKeyNotificationSenderID, IsRequired: true},
			{Property: propertyKeySMSTemplate, IsRequired: true},
		},
	},
	ExecutorNameSSOCheck: {
		SupportedFlowTypes: []providers.FlowType{"AUTHENTICATION"},
		SupportedProperties: []providers.ExecutorSupportedProperties{
			{Property: "checkpointRef", IsRequired: true},
		},
	},
	ExecutorNameSession: {
		SupportedFlowTypes: []providers.FlowType{"AUTHENTICATION"},
	},
	ExecutorNameSessionRevocation: {
		SupportedFlowTypes: []providers.FlowType{"ADMINISTRATION"},
	},
	ExecutorNameSessionSignOut: {
		SupportedFlowTypes: []providers.FlowType{"SIGNOUT"},
	},
	ExecutorNameUserDelete: {
		SupportedFlowTypes: []providers.FlowType{"ADMINISTRATION"},
	},
	ExecutorNameUserTypeResolver: {
		SupportedProperties: []providers.ExecutorSupportedProperties{
			{Property: propertyKeyAllowedUserTypes},
		},
	},
}

// Names returns every built-in executor name, sorted.
func Names() []string {
	names := make([]string, 0, len(catalog))
	for name := range catalog {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// MetaFor returns a copy of the static metadata for a built-in executor. The boolean reports whether
// the name is one. A copy is returned so a caller cannot alter what every other caller reads.
func MetaFor(name string) (providers.ExecutorMeta, bool) {
	meta, ok := catalog[name]
	return meta, ok
}

// Registry is the runtime-free view of the executor registry that flow definition validation and
// graph building need. It answers from the static catalog and holds no executor, so a server that
// only validates flows links none of the services an executor is constructed with.
//
// It honors the same configured subset as the runtime registry: an empty names list registers every
// built-in, and a non-empty one restricts to those named, so validation on one plane agrees with
// what another plane will actually run.
type Registry struct {
	registered map[string]bool
}

// NewRegistry builds the metadata view for the named built-in executors. An empty list means all of
// them. An unknown name is an error, matching the runtime registry rather than silently ignoring it.
func NewRegistry(names []string) (*Registry, error) {
	registered := make(map[string]bool, len(catalog))
	if len(names) == 0 {
		for name := range catalog {
			registered[name] = true
		}
		return &Registry{registered: registered}, nil
	}
	for _, name := range names {
		if _, ok := catalog[name]; !ok {
			return nil, fmt.Errorf("unknown built-in executor: %q", name)
		}
		registered[name] = true
	}
	return &Registry{registered: registered}, nil
}

// IsRegistered reports whether the named executor is one this deployment runs.
func (r *Registry) IsRegistered(name string) bool {
	return r.registered[name]
}

// GetExecutorMeta returns the metadata of a registered executor. A name this deployment does not
// register is an error, so a flow referencing it fails validation rather than passing on metadata
// nothing will honor.
func (r *Registry) GetExecutorMeta(name string) (*providers.ExecutorMeta, error) {
	if !r.registered[name] {
		return nil, fmt.Errorf("executor '%s' not found", name)
	}
	meta := catalog[name]
	return &meta, nil
}
