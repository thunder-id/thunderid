// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package client defines the service and interfaces for sending messages.
package client

import (
	"context"
	"time"

	"github.com/thunder-id/thunderid/internal/notification/common"
)

// httpClientTimeout is the timeout duration for the HTTP client.
const httpClientTimeout = 10 * time.Second

// NotificationClientInterface is the common contract every provider client satisfies. The
// channel-specific interfaces below carry the send method, so a caller that resolved a sender
// asserts to the one matching the channel it wants and cannot dispatch across channels.
type NotificationClientInterface interface {
	GetName() string
}

// MessageClientInterface defines the provider client interface for sending messages.
type MessageClientInterface interface {
	NotificationClientInterface
	IsChannelSupported(channel common.ChannelType) bool
	Send(ctx context.Context, channel common.ChannelType, data common.MessageData) error
}

// EmailClientInterface defines the provider client interface for sending emails.
type EmailClientInterface interface {
	NotificationClientInterface
	Send(ctx context.Context, data common.EmailData) error
}
