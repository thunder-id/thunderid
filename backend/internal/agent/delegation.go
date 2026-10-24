// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"slices"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// applyGrantTypes applies the grant policy to an agent being created.
func applyGrantTypes(agent *providers.Agent) *tidcommon.ServiceError {
	configs, svcErr := resolveGrantTypes(agent.InboundAuthConfig, agent.Delegated, agent.Kind)
	if svcErr != nil {
		return svcErr
	}
	agent.InboundAuthConfig = configs
	return nil
}

// resolveGrantTypes fills in and checks the OAuth grant types of an agent being created or updated,
// before the inbound client service sees them. Every agent holds client credentials, except a public
// client, which cannot use them. A delegated agent also needs a grant type that lets it act for a
// signed-in user, or gets the delegated defaults when it asks for none. A personal agent follows its own
// rules whatever delegated says (resolvePersonalGrantTypes). The given configs are copied, not mutated.
//
// A non-delegated agent with no OAuth configuration gets none, since the caller did not ask for a
// client. A delegated or personal one always gets one.
func resolveGrantTypes(
	inboundConfigs []providers.InboundAuthConfigWithSecret, delegated bool, kind providers.AgentKind,
) ([]providers.InboundAuthConfigWithSecret, *tidcommon.ServiceError) {
	personal := kind == providers.AgentKindPersonalAssistant
	index := oauthConfigIndex(inboundConfigs)
	if index < 0 && !delegated && !personal {
		return inboundConfigs, nil
	}

	configs := slices.Clone(inboundConfigs)
	if index < 0 {
		configs = append(configs, providers.InboundAuthConfigWithSecret{Type: providers.OAuthInboundAuthType})
		index = len(configs) - 1
	}
	oauthConfig := providers.OAuthConfigWithSecret{}
	if configs[index].OAuthConfig != nil {
		oauthConfig = *configs[index].OAuthConfig
	}

	if personal {
		if svcErr := resolvePersonalGrantTypes(&oauthConfig); svcErr != nil {
			return nil, svcErr
		}
		configs[index].OAuthConfig = &oauthConfig
		return configs, nil
	}

	givenGrantTypes := slices.Clone(oauthConfig.GrantTypes)
	if !slices.Contains(givenGrantTypes, providers.GrantTypeClientCredentials) && !isPublicClient(&oauthConfig) {
		givenGrantTypes = slices.Concat(defaultGrantTypes, givenGrantTypes)
	}

	if delegated {
		if hasOnlyClientCredentials(givenGrantTypes) {
			givenGrantTypes = slices.Concat(givenGrantTypes, defaultDelegatedGrantTypes)
			// The defaults bring the authorization code flow, set up the way a signed-in user's sign-in
			// needs it.
			if len(oauthConfig.ResponseTypes) == 0 {
				oauthConfig.ResponseTypes = []providers.ResponseType{providers.ResponseTypeCode}
			}
			oauthConfig.PKCERequired = true
		} else if !slices.ContainsFunc(givenGrantTypes, func(grantType providers.GrantType) bool {
			return slices.Contains(delegationGrantTypes, grantType)
		}) {
			return nil, ErrorMissingDelegationGrantType.WithParams(map[string]string{
				"grantTypes": strings.Join(grantTypeStrings(delegationGrantTypes), ", "),
			})
		}
	}
	oauthConfig.GrantTypes = givenGrantTypes

	configs[index].OAuthConfig = &oauthConfig
	return configs, nil
}

// resolvePersonalGrantTypes applies a personal agent's grant rules: it may hold only the personal grants, and
// gets the authorization code grant when it asks for none. Client credentials are never added, so it cannot act
// on its own authority. With the authorization code grant it gets response type code and PKCE.
func resolvePersonalGrantTypes(oauthConfig *providers.OAuthConfigWithSecret) *tidcommon.ServiceError {
	grantTypes := slices.Clone(oauthConfig.GrantTypes)
	if len(grantTypes) == 0 {
		grantTypes = []providers.GrantType{providers.GrantTypeAuthorizationCode}
	}
	if svcErr := validatePersonalAgentGrantTypes(grantTypes); svcErr != nil {
		return svcErr
	}
	if slices.Contains(grantTypes, providers.GrantTypeAuthorizationCode) {
		oauthConfig.ResponseTypes = []providers.ResponseType{providers.ResponseTypeCode}
		oauthConfig.PKCERequired = true
	}
	oauthConfig.GrantTypes = grantTypes
	return nil
}

// isPublicClient reports whether the client authenticates without a secret.
func isPublicClient(oauthConfig *providers.OAuthConfigWithSecret) bool {
	return oauthConfig.PublicClient || oauthConfig.TokenEndpointAuthMethod == providers.TokenEndpointAuthMethodNone
}

// oauthConfigIndex returns the position of the OAuth entry, or -1 when there is none.
func oauthConfigIndex(configs []providers.InboundAuthConfigWithSecret) int {
	return slices.IndexFunc(configs, func(config providers.InboundAuthConfigWithSecret) bool {
		return config.Type == providers.OAuthInboundAuthType
	})
}

// hasOnlyClientCredentials reports whether no grant type beyond client credentials is given.
func hasOnlyClientCredentials(grantTypes []providers.GrantType) bool {
	return !slices.ContainsFunc(grantTypes, func(grantType providers.GrantType) bool {
		return grantType != providers.GrantTypeClientCredentials
	})
}

// grantTypeStrings converts grant types to their wire values.
func grantTypeStrings(grantTypes []providers.GrantType) []string {
	values := make([]string, len(grantTypes))
	for i, grantType := range grantTypes {
		values[i] = string(grantType)
	}
	return values
}
