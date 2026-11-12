// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"encoding/json"

	"github.com/thunder-id/thunderid/internal/agent/model"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// captureValues hands a written agent to the value capturer, in the shape its exporter reads back.
// The export names each value by where it sits in that shape, so a capture handed any other shape
// would name nothing the export refers to.
//
// It is given what the write returns rather than what a read returns, because only the write still
// holds a client secret: it is hashed when stored and no read gives it back. The credentials the
// agent's type declares are read from the attributes as submitted, for the same reason.
//
// The owner is captured only when the write sets it, as a create does or an update that changes it.
// A gateway names its own owner for the agent, so an update that leaves the owner as it was must not
// put this deployment's owner back in place of the one the gateway was given.
func (s *agentService) captureValues(ctx context.Context, agent *model.AgentCompleteResponse,
	submitted json.RawMessage, ownerSet bool) {
	if s.valueCapturer == nil || agent == nil {
		return
	}
	credentials, err := s.entityService.GetDeclaredCredentialValues(ctx, providers.EntityCategoryAgent,
		agent.Type, submitted)
	if err != nil {
		s.logger.Warn(ctx, "Failed to read which attributes are credentials, so none were captured",
			log.MaskedString("agentID", agent.ID), log.Error(err))
		credentials = nil
	}
	resource := &model.AgentGetResponse{
		ID:                agent.ID,
		OUID:              agent.OUID,
		Type:              agent.Type,
		Kind:              agent.Kind,
		InboundAuthConfig: agent.InboundAuthConfig,
	}
	if ownerSet {
		resource.Owner = agent.Owner
	}
	s.capture(ctx, resource, agent.Attributes, credentials)
}

// captureCredentials hands the credentials just set on an agent to the value capturer.
func (s *agentService) captureCredentials(ctx context.Context, agent *providers.Entity,
	credentials map[string]string) {
	if s.valueCapturer == nil || agent == nil || len(credentials) == 0 {
		return
	}
	s.capture(ctx, &model.AgentGetResponse{ID: agent.ID, OUID: agent.OUID, Type: agent.Type},
		agent.Attributes, credentials)
}

// capture hands an agent to the value capturer with its credentials, in the shape the export reads back.
func (s *agentService) capture(ctx context.Context, resource *model.AgentGetResponse,
	attributes json.RawMessage, credentials map[string]string) {
	displayPaths := s.resolveDisplayPaths(ctx, []string{resource.Type})
	resource.Display = sysutils.ResolveDisplay(resource.ID, resource.Type, attributes, displayPaths)
	if len(credentials) > 0 {
		resource.Credentials = make(map[string]interface{}, len(credentials))
		for credType, value := range credentials {
			resource.Credentials[credType] = value
		}
	}
	s.valueCapturer.CaptureValues(ctx, resourceTypeAgent, resource)
}
