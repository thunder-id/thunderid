// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package agentmgtprovider implements the runtime-to-agent-management boundary for agent operations.
package agentmgtprovider

import (
	"context"
	"encoding/json"

	"github.com/thunder-id/thunderid/internal/agent"
	"github.com/thunder-id/thunderid/internal/agent/model"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type defaultAgentMgtProvider struct {
	agentSvc agent.AgentServiceInterface
}

// newDefaultAgentMgtProvider creates a new default agent provider.
func newDefaultAgentMgtProvider(agentSvc agent.AgentServiceInterface) providers.AgentMgtProvider {
	return &defaultAgentMgtProvider{
		agentSvc: agentSvc,
	}
}

// CreateAgent provisions an agent through the agent service, which owns the defaults and validation.
func (p *defaultAgentMgtProvider) CreateAgent(
	ctx context.Context, agent *providers.Agent,
) (*providers.Agent, *tidcommon.ServiceError) {
	if agent == nil {
		return nil, &ErrorInvalidRequestFormat
	}

	created, svcErr := p.agentSvc.CreateAgent(ctx, agent)
	if svcErr != nil {
		return nil, svcErr
	}
	return toProviderAgent(created), nil
}

// UpdateAgentCredentials sets the agent's credentials through the agent service.
func (p *defaultAgentMgtProvider) UpdateAgentCredentials(
	ctx context.Context, agentID string, credentials json.RawMessage,
) *tidcommon.ServiceError {
	return p.agentSvc.UpdateAgentCredentials(ctx, agentID, credentials)
}

// ValidateRemoveAgentCredential checks the invalidation through the agent service, changing nothing, and returns
// the owner's email address.
func (p *defaultAgentMgtProvider) ValidateRemoveAgentCredential(
	ctx context.Context, agentID, credentialType string,
) (string, *tidcommon.ServiceError) {
	return p.agentSvc.ValidateRemoveAgentCredential(ctx, agentID, credentialType)
}

// RemoveAgentCredential removes the agent's credential through the agent service.
func (p *defaultAgentMgtProvider) RemoveAgentCredential(
	ctx context.Context, agentID, credentialType string,
) (string, *tidcommon.ServiceError) {
	return p.agentSvc.RemoveAgentCredential(ctx, agentID, credentialType)
}

// IsAgentCredentialUnchangedSince checks through the agent service whether the agent's credential changed since
// the removal that returned marker.
func (p *defaultAgentMgtProvider) IsAgentCredentialUnchangedSince(
	ctx context.Context, agentID, marker string,
) (bool, *tidcommon.ServiceError) {
	return p.agentSvc.IsAgentCredentialUnchangedSince(ctx, agentID, marker)
}

// toProviderAgent converts the agent service's create response into the provider contract shape.
// Every profile field is copied, not only the ones the service populates today, so a field the
// service starts returning later is not silently dropped.
func toProviderAgent(resp *model.AgentCompleteResponse) *providers.Agent {
	if resp == nil {
		return nil
	}
	var validUntil *string
	if resp.ValidUntil != "" {
		validUntil = &resp.ValidUntil
	}
	return &providers.Agent{
		ID:          resp.ID,
		OUID:        resp.OUID,
		OUHandle:    resp.OUHandle,
		Type:        resp.Type,
		Description: resp.Description,
		LogoURL:     resp.LogoURL,
		Owner:       resp.Owner,
		Attributes:  resp.Attributes,
		ValidUntil:  validUntil,
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
