// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agentmgtprovider

import (
	"context"

	"github.com/thunder-id/thunderid/internal/agent"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// disabledAgentMgtProvider is an agent provider that rejects all operations.
type disabledAgentMgtProvider struct{}

// NewDisabledAgentMgtProvider creates an agent provider that rejects every operation. It is the
// fallback for deployments and embedders that do not provision agents from the runtime.
func NewDisabledAgentMgtProvider() AgentMgtProviderService {
	return &disabledAgentMgtProvider{}
}

func (p *disabledAgentMgtProvider) SetAgentService(_ agent.AgentServiceInterface) {}

func (p *disabledAgentMgtProvider) CreateAgent(
	_ context.Context, _ *providers.Agent, _ bool,
) (*providers.Agent, *tidcommon.ServiceError) {
	return nil, &ErrorAgentProvisioningDisabled
}
