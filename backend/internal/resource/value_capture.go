// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// captureValues hands a written resource server to the value capturer, in the shape its exporter
// reads back. The export names each value by where it sits in that shape, so a capture handed any
// other shape would name nothing the export refers to.
func (rs *resourceService) captureValues(ctx context.Context, server *providers.ResourceServer) {
	if rs.valueCapturer == nil || server == nil {
		return
	}
	rs.valueCapturer.CaptureValues(ctx, resourceTypeResourceServer, server)
}

// checkDataPlaneReady refuses a create whose values could not reach a data plane yet: while none is
// registered, or the default one is still being provisioned. Without a capturer nothing is refused.
func (rs *resourceService) checkDataPlaneReady(ctx context.Context) *tidcommon.ServiceError {
	if rs.valueCapturer == nil {
		return nil
	}
	return rs.valueCapturer.CheckDataPlaneReady(ctx)
}
