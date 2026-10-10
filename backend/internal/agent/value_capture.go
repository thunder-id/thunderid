// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"

	"github.com/thunder-id/thunderid/internal/agent/model"
)

// captureValues hands a written agent to the value capturer, in the shape its exporter reads back.
// The export names each value by where it sits in that shape, so a capture handed any other shape
// would name nothing the export refers to.
//
// It is given what the write returns rather than what a read returns, because only the write still
// holds a client secret: it is hashed when stored and no read gives it back.
func (s *agentService) captureValues(ctx context.Context, agent *model.AgentCompleteResponse) {
	if s.valueCapturer == nil || agent == nil {
		return
	}
	s.valueCapturer.CaptureValues(ctx, resourceTypeAgent, &model.AgentGetResponse{
		ID:                agent.ID,
		OUID:              agent.OUID,
		Type:              agent.Type,
		Name:              agent.Name,
		InboundAuthConfig: agent.InboundAuthConfig,
	})
}
