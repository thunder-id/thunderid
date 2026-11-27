// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agentmgtprovider

import (
	"context"
	"encoding/json"

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

// UpdateAgentCredentials rejects the update, since the provider is disabled.
func (p *disabledAgentMgtProvider) UpdateAgentCredentials(
	_ context.Context, _ string, _ json.RawMessage,
) *tidcommon.ServiceError {
	return &ErrorAgentManagementDisabled
}

// ValidateRemoveAgentCredential rejects the invalidation, since the provider is disabled.
func (p *disabledAgentMgtProvider) ValidateRemoveAgentCredential(
	_ context.Context, _, _ string,
) (string, *tidcommon.ServiceError) {
	return "", &ErrorAgentManagementDisabled
}

// RemoveAgentCredential rejects the invalidation, since the provider is disabled.
func (p *disabledAgentMgtProvider) RemoveAgentCredential(
	_ context.Context, _, _ string,
) (string, *tidcommon.ServiceError) {
	return "", &ErrorAgentManagementDisabled
}

// IsAgentCredentialUnchangedSince rejects the check, since the provider is disabled.
func (p *disabledAgentMgtProvider) IsAgentCredentialUnchangedSince(
	_ context.Context, _, _ string,
) (bool, *tidcommon.ServiceError) {
	return false, &ErrorAgentManagementDisabled
}
