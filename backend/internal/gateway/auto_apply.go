// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"

	"github.com/thunder-id/thunderid/internal/system/log"
)

// applyToFirstEnvironment applies the latest version to the first environment of the pipeline when
// auto apply is on, so a change made on this control plane reaches it once it is captured. Without a
// configured pipeline that is the default gateway. It never fails the capture: a refused apply is
// logged, and the operator resolves it as any other apply.
func (s *versionService) applyToFirstEnvironment(ctx context.Context) {
	if s.autoApply == nil || !s.autoApply.enabled(ctx) {
		return
	}
	gateways, err := s.gateways.List(ctx)
	if err != nil {
		s.logger.Warn(ctx, "Failed to list the gateways to apply the latest version to", log.Error(err))
		return
	}
	gw, ok := s.pipeline.first(ctx, gateways)
	if !ok {
		return
	}
	if _, svcErr := s.Apply(ctx, gw.ID, ApplyRequest{Version: latestVersion}); svcErr != nil {
		s.logger.Warn(ctx, "Failed to apply the latest version to the first environment",
			log.String("gatewayId", gw.ID), log.String("code", svcErr.Code))
	}
}
