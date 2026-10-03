// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"

	"github.com/thunder-id/thunderid/internal/application/model"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
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

// captureRegeneratedSecret hands the application, carrying the secret just generated for it, to the
// value capturer. A regeneration returns only the secret, so the application is read for the rest of
// what its export names the secret by.
func (as *applicationService) captureRegeneratedSecret(ctx context.Context, appID, secret string) {
	if as.valueCapturer == nil {
		return
	}
	app, svcErr := as.GetApplication(ctx, appID)
	if svcErr != nil {
		as.logger.Warn(ctx, "Failed to read the application whose secret was regenerated, so the secret "+
			"was not captured", log.String("appID", appID))
		return
	}
	for i := range app.InboundAuthConfig {
		if app.InboundAuthConfig[i].Type == providers.OAuthInboundAuthType &&
			app.InboundAuthConfig[i].OAuthConfig != nil {
			app.InboundAuthConfig[i].OAuthConfig.ClientSecret = secret
		}
	}
	as.valueCapturer.CaptureValues(ctx, resourceTypeApplication, app)
}
