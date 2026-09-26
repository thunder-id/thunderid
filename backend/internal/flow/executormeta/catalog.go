// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executormeta

import (
	"fmt"
	"slices"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// catalog is the static metadata of every built-in executor, keyed by name.
//
// It mirrors what each executor reports from GetMeta at runtime, so a build that only validates
// flow definitions can read it without constructing an executor and thereby linking the runtime
// services those constructors need. TestCatalogMatchesRegistry in the executor package holds the
// two together: it registers the real executors and fails on any difference.
var catalog = map[string]providers.ExecutorMeta{
	ExecutorNameApplicationActionValidator: {
		SupportedModes:     []string{"revoke_all", "revoke_before_action"},
		SupportedFlowTypes: []providers.FlowType{"ADMINISTRATION"},
	},
	ExecutorNameApplicationDelete: {
		SupportedFlowTypes: []providers.FlowType{"ADMINISTRATION"},
	},
	ExecutorNameAttributeCollect: {},
	ExecutorNameAttributeUniquenessValidator: {
		SupportedProperties: []providers.ExecutorSupportedProperties{
			{Property: "mode"},
		},
	},
	ExecutorNameAuthAssert: {
		SupportedProperties: []providers.ExecutorSupportedProperties{
			{Property: "callbackType"},
		},
	},
	ExecutorNameAuthorization: {},
	ExecutorNameClientSecret: {
		SupportedFlowTypes: []providers.FlowType{"ADMINISTRATION"},
	},
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
		SupportedModes: []string{"send"},
		SupportedProperties: []providers.ExecutorSupportedProperties{
			{Property: "emailTemplate", IsRequired: true},
			{Property: "senderId"},
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
		DefaultMode:    "identify",
		SupportedModes: []string{"identify", "resolve", "check_state"},
		SupportedProperties: []providers.ExecutorSupportedProperties{
			{Property: "loginHintAttribute"},
		},
	},
	ExecutorNameInviteExecutor: {
		SupportedModes: []string{"generate", "verify"},
		SupportedProperties: []providers.ExecutorSupportedProperties{
			{Property: "inviteBaseURL"},
		},
	},
	ExecutorNameMagicLink: {
		SupportedModes: []string{"generate", "verify"},
		SupportedProperties: []providers.ExecutorSupportedProperties{
			{Property: "tokenExpiry"},
			{Property: "magicLinkURL"},
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
		SupportedModes: []string{"generate", "verify"},
		SupportedProperties: []providers.ExecutorSupportedProperties{
			{Property: "maxAttempts"},
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
			{Property: "presentation_definition_id", IsRequired: true},
			{Property: "allowAuthenticationWithoutLocalUser"},
		},
	},
	ExecutorNamePasskeyAuth: {
		SupportedModes: []string{"challenge", "verify", "register_start", "register_finish"},
		SupportedProperties: []providers.ExecutorSupportedProperties{
			{Property: "relyingPartyId", IsRequired: true, ApplicableModes: []string{"challenge", "register_start"}},
			{Property: "relyingPartyName"},
			{Property: "authenticatorSelection"},
			{Property: "attestation"},
		},
	},
	ExecutorNamePermissionValidator: {
		SupportedProperties: []providers.ExecutorSupportedProperties{
			{Property: "requiredScopes"},
		},
	},
	ExecutorNamePreDelete: {
		DefaultMode:        "revoke_all",
		SupportedModes:     []string{"revoke_all"},
		SupportedFlowTypes: []providers.FlowType{"ADMINISTRATION"},
	},
	ExecutorNameProvisioning: {
		SupportedFlowTypes: []providers.FlowType{
			"AUTHENTICATION", "REGISTRATION", "USER_ONBOARDING", "ADMINISTRATION",
		},
		SupportedProperties: []providers.ExecutorSupportedProperties{
			{Property: "mode"},
			{Property: "includeOptional"},
			{Property: "includeOptionalCredentials"},
			{Property: "maxPerPrompt"},
			{Property: "assignGroup"},
			{Property: "assignRole"},
			{Property: "seedGroupsFromMapping"},
			{Property: "seedRolesFromMapping"},
			{Property: "allowCrossOUProvisioning"},
		},
	},
	ExecutorNameSMSExecutor: {
		SupportedProperties: []providers.ExecutorSupportedProperties{
			{Property: "senderId", IsRequired: true},
			{Property: "smsTemplate", IsRequired: true},
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
			{Property: "allowedUserTypes"},
		},
	},
	ExecutorNameAgentTypeResolver: {
		SupportedFlowTypes: []providers.FlowType{"ADMINISTRATION", "AUTHENTICATION"},
		SupportedProperties: []providers.ExecutorSupportedProperties{
			{Property: "allowedAgentTypes"},
		},
	},
	ExecutorNameOwnerResolver: {
		SupportedFlowTypes: []providers.FlowType{"ADMINISTRATION", "REGISTRATION"},
	},
}

// Names returns the names of every executor in the catalog, sorted.
func Names() []string {
	names := make([]string, 0, len(catalog))
	for name := range catalog {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// cloneMeta returns a copy of meta that shares no backing array with the catalog. The catalog is a
// package-level value handed to every caller, so returning it by value still aliases its slices: a
// caller appending to or writing through one would change what every later lookup reports.
func cloneMeta(meta providers.ExecutorMeta) providers.ExecutorMeta {
	meta.SupportedModes = slices.Clone(meta.SupportedModes)
	meta.SupportedFlowTypes = slices.Clone(meta.SupportedFlowTypes)
	meta.SupportedProperties = slices.Clone(meta.SupportedProperties)
	for i := range meta.SupportedProperties {
		meta.SupportedProperties[i].ApplicableModes = slices.Clone(meta.SupportedProperties[i].ApplicableModes)
	}
	return meta
}

// MetaFor returns the static metadata for the named executor, reporting whether it is known.
func MetaFor(name string) (providers.ExecutorMeta, bool) {
	meta, ok := catalog[name]
	if !ok {
		return providers.ExecutorMeta{}, false
	}
	return cloneMeta(meta), true
}

// registry answers the metadata questions flow validation asks, from the static catalog alone.
type registry struct {
	names map[string]struct{}
}

// NewRegistry builds a metadata-only executor registry over the named executors. An unknown name is
// an error rather than a silently absent executor, so a misconfigured executor list fails at
// startup instead of at the first flow that names it. An empty list enables the whole catalog.
func NewRegistry(names []string) (*registry, error) {
	enabled := make(map[string]struct{}, len(names))
	if len(names) == 0 {
		for name := range catalog {
			enabled[name] = struct{}{}
		}
		return &registry{names: enabled}, nil
	}
	for _, name := range names {
		if _, ok := catalog[name]; !ok {
			return nil, fmt.Errorf("unknown executor %q", name)
		}
		enabled[name] = struct{}{}
	}
	return &registry{names: enabled}, nil
}

// IsRegistered reports whether the named executor is enabled on this server.
func (r *registry) IsRegistered(name string) bool {
	_, ok := r.names[name]
	return ok
}

// GetExecutorMeta returns the static metadata of an enabled executor.
func (r *registry) GetExecutorMeta(name string) (*providers.ExecutorMeta, error) {
	if _, ok := r.names[name]; !ok {
		return nil, fmt.Errorf("executor %q is not registered", name)
	}
	meta := cloneMeta(catalog[name])
	return &meta, nil
}
