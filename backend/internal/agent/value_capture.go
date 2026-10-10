// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"

	"github.com/thunder-id/thunderid/internal/agent/model"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// captureValues hands a written agent to the value capturer, in the shape its exporter reads back.
// The export names each value by where it sits in that shape, so a capture handed any other shape
// would name nothing the export refers to. The export names an agent by its display value, so the
// capture resolves the same one.
//
// It is given what the write returns rather than what a read returns, because only the write still
// holds a client secret: it is hashed when stored and no read gives it back.
func (s *agentService) captureValues(ctx context.Context, agent *model.AgentCompleteResponse) {
	if s.valueCapturer == nil || agent == nil {
		return
	}
	displayPaths := s.resolveDisplayPaths(ctx, []string{agent.Type})
	s.valueCapturer.CaptureValues(ctx, resourceTypeAgent, &model.AgentGetResponse{
		ID:                agent.ID,
		OUID:              agent.OUID,
		Type:              agent.Type,
		Display:           sysutils.ResolveDisplay(agent.ID, agent.Type, agent.Attributes, displayPaths),
		InboundAuthConfig: agent.InboundAuthConfig,
	})
}
