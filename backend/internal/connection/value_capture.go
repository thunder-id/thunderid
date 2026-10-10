// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"context"

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
	if err != nil {
		log.GetLogger().With(log.String(log.LoggerKeyComponentName, valueCaptureLoggerComponentName)).Warn(ctx,
			"Failed to read a connection's values, so none were captured",
			log.String("connectionId", dto.ID), log.Error(err))
		return
	}
	s.valueCapturer.CaptureValues(ctx, resourceTypeConnection, &model)
}

// captureSenderValues hands a written message-sender-backed connection to the value capturer, in the
// shape its exporter reads back, for the same reasons captureIDPValues does.
func (s *service) captureSenderValues(ctx context.Context, dto *ncommon.NotificationSenderDTO) {
	if s.valueCapturer == nil || dto == nil {
		return
	}
	model, err := connectionModelFromSenderDTO(*dto)
	if err != nil {
		log.GetLogger().With(log.String(log.LoggerKeyComponentName, valueCaptureLoggerComponentName)).Warn(ctx,
			"Failed to read a connection's values, so none were captured",
			log.String("connectionId", dto.ID), log.Error(err))
		return
	}
	s.valueCapturer.CaptureValues(ctx, resourceTypeConnection, &model)
}
