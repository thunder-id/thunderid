// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"

	"github.com/thunder-id/thunderid/internal/application/model"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// captureValues hands a written application to the value capturer, in the shape its exporter reads
// back. The export names each value by where it sits in that shape, so a capture handed any other
// shape would name nothing the export refers to.
//
// It is given what the write returns rather than what a read returns, because only the write still
// holds a client secret: it is hashed when stored and no read gives it back.
func (as *applicationService) captureValues(ctx context.Context, app *model.ApplicationDTO) {
	if as.valueCapturer == nil || app == nil {
		return
	}
	as.valueCapturer.CaptureValues(ctx, resourceTypeApplication, &providers.Application{
		ID:                app.ID,
		OUID:              app.OUID,
		Name:              app.Name,
		Type:              string(app.Type),
		InboundAuthConfig: app.InboundAuthConfig,
	})
}

// checkDataPlaneReady refuses a create whose values could not reach a data plane yet: while none is
// registered, or the default one is still being provisioned. Without a capturer nothing is refused.
func (as *applicationService) checkDataPlaneReady(ctx context.Context) *tidcommon.ServiceError {
	if as.valueCapturer == nil {
		return nil
	}
	return as.valueCapturer.CheckDataPlaneReady(ctx)
}
