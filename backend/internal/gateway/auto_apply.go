// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"

	"github.com/thunder-id/thunderid/internal/system/log"
)

// applyToFirstGateway applies the version a capture answered with to the first gateway of the
// pipeline when auto apply is on, so a change made on this control plane reaches it once it is
// captured. Without a configured pipeline that is the default gateway. It never fails the capture: a
// refused apply is logged, and the operator resolves it as any other apply.
func (s *versionService) applyToFirstGateway(ctx context.Context, captured *Version) {
	if s.autoApply == nil || !s.autoApply.enabled(ctx) {
		return
	}
	ref := s.autoApplyRef(ctx, captured)
	gateways, err := s.gateways.List(ctx)
	if err != nil {
		s.logger.Warn(ctx, "Failed to list the gateways to apply the captured version to", log.Error(err))
		return
	}
	gw, ok := s.pipeline.first(ctx, gateways)
	if !ok {
		return
	}
	if _, svcErr := s.Apply(ctx, gw.ID, ApplyRequest{Version: ref}); svcErr != nil {
		s.logger.Warn(ctx, "Failed to apply the captured version to the first gateway",
			log.String("gatewayId", gw.ID), log.String("code", svcErr.Code))
	}
}

// autoApplyRef names the version to apply. A capture of a configuration an older version already
// holds, as a revert leaves it, answers with that older version, which "latest" would not name. It is
// named by its hash then; otherwise "latest" is asked for, which a burst of changes folds into the
// apply already running.
func (s *versionService) autoApplyRef(ctx context.Context, captured *Version) string {
	if captured == nil || captured.Hash == "" {
		return latestVersion
	}
	latest, err := s.versions.Latest(ctx)
	if err != nil {
		s.logger.Warn(ctx, "Failed to read the latest version", log.Error(err))
		return captured.Hash
	}
	if captured.Seq == latest {
		return latestVersion
	}
	return captured.Hash
}
