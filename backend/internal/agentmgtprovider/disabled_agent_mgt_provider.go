// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agentmgtprovider

import (
	"context"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// disabledAgentMgtProvider is an agent provider that rejects all operations.
type disabledAgentMgtProvider struct{}

// NewDisabledAgentMgtProvider creates an agent provider that rejects every operation. It is the
// fallback for deployments and embedders that do not provision agents from the runtime.
func NewDisabledAgentMgtProvider() providers.AgentMgtProvider {
	return &disabledAgentMgtProvider{}
}

func (p *disabledAgentMgtProvider) CreateAgent(
	_ context.Context, _ *providers.Agent,
) (*providers.Agent, *tidcommon.ServiceError) {
	return nil, &ErrorAgentProvisioningDisabled
}
