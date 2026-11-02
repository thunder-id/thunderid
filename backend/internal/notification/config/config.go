// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package notificationconfig holds notification configuration injected at initialization.
package notificationconfig

import (
	"github.com/thunder-id/thunderid/internal/system/config"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// Config holds configuration values required by notification services.
type Config struct {
	OTP                         config.OTPConfig
	JWT                         engineconfig.JWTConfig
	DeclarativeResourcesEnabled bool
}

// NotificationSectionConfig is the value of the "notification" server-config section.
type NotificationSectionConfig struct {
	// DefaultEmailSenderID names the sender used by server-sent email outside a flow.
	DefaultEmailSenderID string `json:"defaultEmailSenderId" yaml:"defaultEmailSenderId"`
}

// FromServerRuntime builds notification configuration from the global server runtime.
func FromServerRuntime() Config {
	runtime := config.GetServerRuntime()
	return Config{
		OTP:                         runtime.Config.Notification.OTP,
		JWT:                         runtime.Config.JWT,
		DeclarativeResourcesEnabled: runtime.Config.DeclarativeResources.Enabled,
	}
}
