// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agentmgtprovider

import (
	"github.com/thunder-id/thunderid/internal/agent"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// AgentMgtProviderService is the agent management provider. It exposes SetAgentService so the agent
// service can be injected after construction, since that service depends on components initialized
// after the flow executor registry.
type AgentMgtProviderService interface {
	providers.AgentMgtProvider
	// SetAgentService injects the agent service.
	SetAgentService(agentSvc agent.AgentServiceInterface)
}

// Initialize initializes the agent provider.
func Initialize(entityTypeSvc entitytype.EntityTypeServiceInterface) AgentMgtProviderService {
	agentMgtProviderConfig := config.GetServerRuntime().Config.AgentMgtProvider
	switch agentMgtProviderConfig.Type {
	case "disabled":
		return initializeDisabledAgentMgtProvider()
	default:
		return initializeDefaultAgentMgtProvider(entityTypeSvc)
	}
}

// initializeDefaultAgentMgtProvider initializes the default agent provider.
func initializeDefaultAgentMgtProvider(
	entityTypeSvc entitytype.EntityTypeServiceInterface,
) AgentMgtProviderService {
	return newDefaultAgentMgtProvider(entityTypeSvc)
}

// initializeDisabledAgentMgtProvider initializes the disabled agent provider.
func initializeDisabledAgentMgtProvider() AgentMgtProviderService {
	return NewDisabledAgentMgtProvider()
}
