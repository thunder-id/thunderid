// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package agentmgtprovider implements the runtime-to-agent-management boundary for agent operations.
package agentmgtprovider

import (
	"context"

	"github.com/thunder-id/thunderid/internal/agent"
	"github.com/thunder-id/thunderid/internal/agent/model"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// defaultAgentLogoURL stands in when an agent is provisioned without one, so a listing has
// something to draw. The avatar scheme is resolved by the client rather than fetched.
const defaultAgentLogoURL = "avatar:shape=circle,variant=anonymous_entity,content=bot_head,colors=0"

type defaultAgentMgtProvider struct {
	agentSvc agent.AgentServiceInterface
}

// newDefaultAgentMgtProvider creates a new default agent provider.
func newDefaultAgentMgtProvider() AgentMgtProviderService {
	return &defaultAgentMgtProvider{}
}

// SetAgentService injects the agent service. See AgentMgtProviderService.
func (p *defaultAgentMgtProvider) SetAgentService(agentSvc agent.AgentServiceInterface) {
	p.agentSvc = agentSvc
}

// CreateAgent provisions an agent through the agent service.
func (p *defaultAgentMgtProvider) CreateAgent(
	ctx context.Context, agent *providers.Agent, delegated bool,
) (*providers.Agent, *tidcommon.ServiceError) {
	if agent == nil {
		return nil, &ErrorInvalidRequestFormat
	}
	if p.agentSvc == nil {
		return nil, &ErrorAgentProvisioningDisabled
	}

	created, svcErr := p.agentSvc.CreateAgent(security.WithRuntimeContext(ctx),
		applyCreateDefaults(agent, delegated, agent.AllowedUserTypes))
	if svcErr != nil {
		return nil, svcErr
	}
	return toProviderAgent(created), nil
}

// applyCreateDefaults fills in the fields the caller left unset: a logo to represent the agent,
// and the inbound authentication shape.
//
// The inbound client service accepts only two combinations, one for an agent acting on its own
// behalf and one for an agent acting for a signed-in user, so the shape is derived here rather
// than accepted from the caller. The agent is copied rather than mutated. The owner is forwarded
// as given: the agent service resolves an absent one to the authenticated subject.
func applyCreateDefaults(
	agent *providers.Agent, delegated bool, allowedUserTypes []string,
) *providers.Agent {
	created := *agent
	if created.LogoURL == "" {
		created.LogoURL = defaultAgentLogoURL
	}
	created.InboundAuthConfig = []providers.InboundAuthConfigWithSecret{
		{
			Type:        providers.OAuthInboundAuthType,
			OAuthConfig: buildOAuthConfig(callerRedirectURIs(agent), delegated),
		},
	}
	// Allowed user types are meaningless without delegation, so clearing them keeps a caller from
	// attaching a restriction that would never be enforced.
	created.AllowedUserTypes = nil
	if delegated {
		created.AllowedUserTypes = allowedUserTypes
	}
	return &created
}

// callerRedirectURIs reads the one OAuth value a caller may supply. Anything else left on the
// inbound auth config is discarded.
func callerRedirectURIs(agent *providers.Agent) []string {
	for _, inbound := range agent.InboundAuthConfig {
		if inbound.Type == providers.OAuthInboundAuthType && inbound.OAuthConfig != nil {
			return inbound.OAuthConfig.RedirectURIs
		}
	}
	return nil
}

// buildOAuthConfig derives the OAuth client configuration. Flow identifiers and token settings are
// left unset so the inbound client service applies the organization unit and server defaults.
func buildOAuthConfig(redirectURIs []string, delegated bool) *providers.OAuthConfigWithSecret {
	config := &providers.OAuthConfigWithSecret{
		GrantTypes:              []providers.GrantType{providers.GrantTypeClientCredentials},
		TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
	}
	if delegated {
		config.GrantTypes = append(config.GrantTypes,
			providers.GrantTypeAuthorizationCode, providers.GrantTypeRefreshToken)
		config.ResponseTypes = []providers.ResponseType{providers.ResponseTypeCode}
		config.RedirectURIs = redirectURIs
		config.PKCERequired = true
	}
	return config
}

// toProviderAgent converts the agent service's create response into the provider contract shape.
// Every profile field is copied, not only the ones the service populates today, so a field the
// service starts returning later is not silently dropped.
func toProviderAgent(resp *model.AgentCompleteResponse) *providers.Agent {
	if resp == nil {
		return nil
	}
	return &providers.Agent{
		ID:          resp.ID,
		OUID:        resp.OUID,
		OUHandle:    resp.OUHandle,
		Type:        resp.Type,
		Name:        resp.Name,
		Description: resp.Description,
		LogoURL:     resp.LogoURL,
		Owner:       resp.Owner,
		Attributes:  resp.Attributes,
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID:                resp.AuthFlowID,
			AuthFlowHandle:            resp.AuthFlowHandle,
			RegistrationFlowID:        resp.RegistrationFlowID,
			RegistrationFlowHandle:    resp.RegistrationFlowHandle,
			IsRegistrationFlowEnabled: resp.IsRegistrationFlowEnabled,
			RecoveryFlowID:            resp.RecoveryFlowID,
			RecoveryFlowHandle:        resp.RecoveryFlowHandle,
			IsRecoveryFlowEnabled:     resp.IsRecoveryFlowEnabled,
			SignOutFlowID:             resp.SignOutFlowID,
			SignOutFlowHandle:         resp.SignOutFlowHandle,
			ThemeID:                   resp.ThemeID,
			LayoutID:                  resp.LayoutID,
			Assertion:                 resp.Assertion,
			LoginConsent:              resp.LoginConsent,
			AllowedUserTypes:          resp.AllowedUserTypes,
			SubjectAttribute:          resp.SubjectAttribute,
			PasskeyAllowedOrigins:     resp.PasskeyAllowedOrigins,
			Attestation:               resp.Attestation,
		},
		InboundAuthConfig: resp.InboundAuthConfig,
	}
}
