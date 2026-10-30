// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"context"

	"github.com/thunder-id/thunderid/internal/connection/httpconnection"
	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const valueCaptureLoggerComponentName = "ConnectionValueCapture"

// captureIDPValues hands a written identity-provider-backed connection to the value capturer, in the
// shape its exporter reads back. The export names each value by where it sits in that shape, so a
// capture handed any other shape would name nothing the export refers to.
//
// An update carries the stored secret forward when the request leaves it out, so the secret is
// captured again under the connection's current name, which keeps a renamed connection's secret
// where its export now refers to it.
func (s *service) captureIDPValues(ctx context.Context, dto *providers.IDPDTO) {
	if s.valueCapturer == nil || dto == nil {
		return
	}
	model, err := connectionModelFromIDPDTO(*dto)
	s.captureModel(ctx, dto.ID, model, err)
}

// captureSenderValues hands a written message-sender-backed connection to the value capturer, in the
// shape its exporter reads back, for the same reasons captureIDPValues does.
func (s *service) captureSenderValues(ctx context.Context, dto *ncommon.NotificationSenderDTO) {
	if s.valueCapturer == nil || dto == nil {
		return
	}
	model, err := connectionModelFromSenderDTO(*dto)
	s.captureModel(ctx, dto.ID, model, err)
}

// captureHTTPValues hands a written HTTP connection to the value capturer, in the shape its exporter
// reads back, for the same reasons captureIDPValues does.
func (s *service) captureHTTPValues(ctx context.Context, connection *httpconnection.HTTPConnection) {
	if s.valueCapturer == nil || connection == nil {
		return
	}
	model, err := connectionModelFromHTTP(*connection)
	s.captureModel(ctx, connection.ID, model, err)
}

// releaseHTTPSecrets hands the value capturer the secrets of an HTTP connection that were dropped: all
// of them once it is deleted, when current is nil, or else each one current no longer holds a value
// for. A secret current still holds, even under a new name after a rename, is not released, so a
// version captured before the update still finds it.
func (s *service) releaseHTTPSecrets(ctx context.Context, previous, current *httpconnection.HTTPConnection) {
	if s.valueCapturer == nil || previous == nil {
		return
	}
	model, err := connectionModelFromHTTP(*previous)
	if err != nil {
		s.logReleaseFailure(ctx, previous.ID, err)
		return
	}
	if current != nil {
		kept, err := connectionModelFromHTTP(*current)
		if err != nil {
			s.logReleaseFailure(ctx, current.ID, err)
			return
		}
		for name, value := range kept.Authentication.Properties {
			if value != "" {
				delete(model.Authentication.Properties, name)
			}
		}
	}
	s.valueCapturer.ReleaseSecrets(ctx, resourceTypeConnection, &model)
}

// logReleaseFailure logs why a connection's secrets could not be released.
func (s *service) logReleaseFailure(ctx context.Context, id string, err error) {
	log.GetLogger().With(log.String(log.LoggerKeyComponentName, valueCaptureLoggerComponentName)).Warn(ctx,
		"Failed to read a connection's values, so none were released",
		log.String("connectionId", id), log.Error(err))
}

// captureModel hands a connection's export model to the value capturer, or logs why it could not be
// built.
func (s *service) captureModel(ctx context.Context, id string, model connectionExportModel, err error) {
	if err != nil {
		log.GetLogger().With(log.String(log.LoggerKeyComponentName, valueCaptureLoggerComponentName)).Warn(ctx,
			"Failed to read a connection's values, so none were captured",
			log.String("connectionId", id), log.Error(err))
		return
	}
	s.valueCapturer.CaptureValues(ctx, resourceTypeConnection, &model)
}
