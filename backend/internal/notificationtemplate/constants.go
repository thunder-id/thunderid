// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import "regexp"

// handleFormatRegex matches a lowercase kebab/snake identifier (e.g. otp-verification): starts and
// ends with an alphanumeric, with dashes/underscores allowed inside. Matches the flow handle
// convention (internal/flow/mgt).
var handleFormatRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*[a-z0-9]$|^[a-z0-9]$`)

// translationNamespace is the translation namespace that {{t(...)}} keys resolve under.
const translationNamespace = "notification"

// ChannelType is the notification channel a template belongs to.
type ChannelType string

// Notification channels.
const (
	ChannelTypeEmail ChannelType = "email"
	ChannelTypeSMS   ChannelType = "sms"
)

// Color theme variants.
const (
	colorSchemeLight = "light"
	colorSchemeDark  = "dark"
)

// Field length limits (in characters), aligned with the NOTIFICATION_TEMPLATE column definitions.
const (
	maxHandleLength      = 255
	maxDisplayNameLength = 255
	maxDescriptionLength = 512
)

// systemEndpointReferrer is the dependency ResourceType for a reserved-template dependent. Local, not
// a shared type: it only surfaces (humanized) in the delete-conflict summary.
const systemEndpointReferrer = "systemEndpoint"

// systemReservedTemplates are the templates required by built-in out-of-flow endpoints.
var systemReservedTemplates = []reservedTemplate{
	{
		channel:     ChannelTypeSMS,
		handle:      "otp",
		consumerID:  "sms-otp-authentication",
		displayName: "SMS OTP authentication",
	},
}
