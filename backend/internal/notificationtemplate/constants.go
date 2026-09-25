// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

// Notification channels.
const (
	ChannelEmail = "email"
	ChannelSMS   = "sms"
)

// Body media types.
const (
	ContentTypeHTML  = "text/html"
	ContentTypePlain = "text/plain"
)

// Color theme variants.
const (
	ColorSchemeLight = "light"
	ColorSchemeDark  = "dark"
)

// Field length limits (in characters), aligned with the NOTIFICATION_TEMPLATE column definitions.
const (
	maxNameLength        = 255
	maxDescriptionLength = 512
)
