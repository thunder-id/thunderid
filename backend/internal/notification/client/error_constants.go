// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"errors"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// errSMTPReadLimitExceeded is returned when an SMTP server sends more than a session may read.
var errSMTPReadLimitExceeded = errors.New("smtp server sent more data than the session allows")

// ErrorInvalidProvider is the error returned when an invalid provider is specified.
var ErrorInvalidProvider = tidcommon.ServiceError{
	Type: tidcommon.ClientErrorType,
	Code: "MNC-1001",
	Error: tidcommon.I18nMessage{
		Key:          "error.notificationclient.unsupported_notification_provider",
		DefaultValue: "Unsupported notification provider",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.notificationclient.unsupported_notification_provider.description",
		DefaultValue: "The requested notification provider is not supported.",
	},
}
