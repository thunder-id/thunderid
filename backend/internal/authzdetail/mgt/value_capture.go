// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzdetailmgt

import (
	"context"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// captureValues hands a written type to the value capturer, in the shape its exporter reads back. The export
// names each value by where it sits in that shape, so a capture handed any other shape would name nothing the
// export refers to.
func (s *authorizationDetailMgtService) captureValues(ctx context.Context, t *providers.AuthorizationDetailType) {
	if s.valueCapturer == nil || t == nil {
		return
	}
	resource, err := declarativeResourceOf(t)
	if err != nil {
		s.logger.Warn(ctx, "Failed to read an authorization details type, so its values were not captured",
			log.String("id", t.ID), log.Error(err))
		return
	}
	s.valueCapturer.CaptureValues(ctx, resourceTypeAuthorizationDetailType, resource)
}
