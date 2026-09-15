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

type defaultAgentMgtProvider struct {
	agentSvc agent.AgentServiceInterface
}

// newDefaultAgentMgtProvider creates a new default agent provider.
func newDefaultAgentMgtProvider(agentSvc agent.AgentServiceInterface) providers.AgentMgtProvider {
	return &defaultAgentMgtProvider{
		agentSvc: agentSvc,
	}
}

// CreateAgent provisions an agent through the agent service.
func (p *defaultAgentMgtProvider) CreateAgent(
	ctx context.Context, agent *providers.Agent,
) (*providers.Agent, *tidcommon.ServiceError) {
	if agent == nil {
		return nil, &ErrorInvalidRequestFormat
	}
	if agent.Owner == "" {
		return nil, &ErrorOwnerRequired
	}
	created, svcErr := p.agentSvc.CreateAgent(security.WithRuntimeContext(ctx), agent)
	if svcErr != nil {
		return nil, svcErr
	}
	return toProviderAgent(created), nil
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
