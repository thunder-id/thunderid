// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"

	"github.com/thunder-id/thunderid/internal/application/model"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// captureValues hands a written application to the value capturer, in the shape its exporter reads
// back. The export names each value by where it sits in that shape, so a capture handed any other
// shape would name nothing the export refers to.
//
// It is given what the write returns rather than what a read returns, because only the write still
// holds a client secret: it is hashed when stored and no read gives it back. The same holds for the
// Flow Secret, which is passed apart because a write does not return it beyond a create; it is empty
// when the write set none.
func (as *applicationService) captureValues(ctx context.Context, app *model.ApplicationDTO, flowSecret string) {
	if as.valueCapturer == nil || app == nil {
		return
	}
	as.valueCapturer.CaptureValues(ctx, resourceTypeApplication, &applicationDeclarativeResource{
		Application: providers.Application{
			ID:                app.ID,
			OUID:              app.OUID,
			Name:              app.Name,
			Type:              string(app.Type),
			InboundAuthConfig: app.InboundAuthConfig,
		},
		FlowSecret: flowSecret,
	})
}
