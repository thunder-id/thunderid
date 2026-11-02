// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
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
