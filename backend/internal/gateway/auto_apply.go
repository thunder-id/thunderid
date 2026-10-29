// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"

	"github.com/thunder-id/thunderid/internal/system/log"
)

// applyToDefaultGateway applies the latest version to the default gateway when gateway.auto_apply
// is on, so a change made on this control plane reaches that gateway once it is captured. It never
// fails the capture: a refused apply is logged, and the operator resolves it as any other apply.
func (s *versionService) applyToDefaultGateway(ctx context.Context) {
	if !s.autoApply {
		return
	}
	gateways, err := s.gateways.List(ctx)
	if err != nil {
		s.logger.Warn(ctx, "Failed to list the gateways to apply the latest version to", log.Error(err))
		return
	}
	for _, gw := range gateways {
		if !gw.IsDefault {
			continue
		}
		if _, svcErr := s.Apply(ctx, gw.ID, ApplyRequest{Version: latestVersion}); svcErr != nil {
			s.logger.Warn(ctx, "Failed to apply the latest version to the default gateway",
				log.String("gatewayId", gw.ID), log.String("code", svcErr.Code))
		}
		return
	}
}
