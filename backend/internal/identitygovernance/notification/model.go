// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package notification delivers account lock notices through the notification template and sender services.
package notification

import (
	"context"

	governanceconfig "github.com/thunder-id/thunderid/internal/identitygovernance/config"
	notifcommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/notificationtemplate"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// DefaultRecipientAttribute is the profile attribute a lock notice is sent to when none is configured.
const DefaultRecipientAttribute = governanceconfig.DefaultLockEmailRecipientAttribute

// ProfileReader reads an entity's profile, without its runtime data.
type ProfileReader interface {
	GetEntityProfile(ctx context.Context, entityID string) (*providers.Entity, error)
}

// ConfigSource supplies the effective account-access configuration.
type ConfigSource interface {
	Current(ctx context.Context) governanceconfig.AccountAccessValue
}

// TemplateRenderer resolves a notification template by handle.
type TemplateRenderer interface {
	Resolve(ctx context.Context, channel notificationtemplate.ChannelType, handle string,
		in notificationtemplate.RenderInput) (*notificationtemplate.ResolvedContent, *tidcommon.ServiceError)
}

// EmailSender sends an email through a named email sender.
type EmailSender interface {
	SendEmail(ctx context.Context, senderID string, data notifcommon.EmailData) *tidcommon.ServiceError
}

// SenderSource names the server's default email sender, or "" when none is configured.
type SenderSource interface {
	DefaultEmailSenderID(ctx context.Context) string
}
