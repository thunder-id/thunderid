// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"encoding/json"

	"github.com/thunder-id/thunderid/internal/agent/model"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// captureValues hands a written agent to the value capturer, in the shape its exporter reads back.
// The export names each value by where it sits in that shape, so a capture handed any other shape
// would name nothing the export refers to. The export names an agent by its display value, so the
// capture resolves the same one.
//
// It is given what the write returns rather than what a read returns, because only the write still
// holds a client secret: it is hashed when stored and no read gives it back. The credential-type
// attributes are read from the attributes as submitted, for the same reason: the write strips them
// from what it returns and stores only their hash.
func (s *agentService) captureValues(ctx context.Context, agent *model.AgentCompleteResponse,
	submitted json.RawMessage) {
	if s.valueCapturer == nil || agent == nil {
		return
	}
	displayPaths := s.resolveDisplayPaths(ctx, []string{agent.Type})
	s.valueCapturer.CaptureValues(ctx, resourceTypeAgent, &model.AgentGetResponse{
		ID:                agent.ID,
		OUID:              agent.OUID,
		Type:              agent.Type,
		Display:           sysutils.ResolveDisplay(agent.ID, agent.Type, agent.Attributes, displayPaths),
		Credentials:       s.submittedCredentials(ctx, agent, submitted),
		InboundAuthConfig: agent.InboundAuthConfig,
	})
}

// submittedCredentials picks the credential-type attributes out of the attributes an agent was
// written with. Which attributes are credentials is the agent type's to say; a type that cannot be
// read leaves none captured rather than failing the write.
func (s *agentService) submittedCredentials(ctx context.Context, agent *model.AgentCompleteResponse,
	submitted json.RawMessage) map[string]interface{} {
	if len(submitted) == 0 || s.entityTypeService == nil {
		return nil
	}
	var attrs map[string]interface{}
	if err := json.Unmarshal(submitted, &attrs); err != nil {
		return nil
	}
	declared, svcErr := s.entityTypeService.GetAttributes(ctx, entitytype.TypeCategoryAgent, agent.Type,
		entitytype.AttributeFilter{AllowCredential: true})
	if svcErr != nil {
		s.logger.Warn(ctx, "Failed to read which attributes are credentials, so none were captured",
			log.MaskedString(log.LoggerKeyEntityID, agent.ID))
		return nil
	}
	credentials := map[string]interface{}{}
	for _, attribute := range declared {
		if value, ok := attrs[attribute.Attribute].(string); ok && value != "" {
			credentials[attribute.Attribute] = value
		}
	}
	return credentials
}
