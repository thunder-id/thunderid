// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/security"
)

// agentSharingType is the sharing framework's identifier for agents.
const agentSharingType sharing.ResourceType = resourceTypeAgent

// agentSharingDeclaration is how an agent participates in sharing. An agent is shared only through the
// resource file that declares it.
type agentSharingDeclaration struct {
	service AgentServiceInterface
}

// newAgentSharingDeclaration returns the declaration registered with the sharing framework.
func newAgentSharingDeclaration(service AgentServiceInterface) *agentSharingDeclaration {
	return &agentSharingDeclaration{service: service}
}

// ResourceType identifies agents to the sharing framework.
func (d *agentSharingDeclaration) ResourceType() sharing.ResourceType {
	return agentSharingType
}

// Fields declares nothing, so a policy carries no overlay rules for an agent.
func (d *agentSharingDeclaration) Fields() []sharing.FieldDeclaration {
	return nil
}

// OwningOUID returns the organization unit that owns an agent.
func (d *agentSharingDeclaration) OwningOUID(
	ctx context.Context, resourceID string,
) (string, *tidcommon.ServiceError) {
	a, svcErr := d.service.GetAgent(security.WithRuntimeContext(ctx), resourceID, false)
	if svcErr != nil {
		return "", svcErr
	}
	return a.OUID, nil
}
