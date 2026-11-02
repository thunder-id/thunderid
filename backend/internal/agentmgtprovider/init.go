// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agentmgtprovider

import (
	"github.com/thunder-id/thunderid/internal/agent"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize initializes the agent provider.
func Initialize(agentSvc agent.AgentServiceInterface) providers.AgentMgtProvider {
	agentMgtProviderConfig := config.GetServerRuntime().Config.AgentMgtProvider
	switch agentMgtProviderConfig.Type {
	case "disabled":
		return initializeDisabledAgentMgtProvider()
	default:
		return initializeDefaultAgentMgtProvider(agentSvc)
	}
}

// initializeDefaultAgentMgtProvider initializes the default agent provider.
func initializeDefaultAgentMgtProvider(agentSvc agent.AgentServiceInterface) providers.AgentMgtProvider {
	return newDefaultAgentMgtProvider(agentSvc)
}

// initializeDisabledAgentMgtProvider initializes the disabled agent provider.
func initializeDisabledAgentMgtProvider() providers.AgentMgtProvider {
	return NewDisabledAgentMgtProvider()
}
