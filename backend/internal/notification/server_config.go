// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notification

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/serverconfig"
	"github.com/thunder-id/thunderid/internal/system/log"
)

var (
	// errDeclarativeSenderLocked is returned when a declarative default sender is overridden.
	errDeclarativeSenderLocked = errors.New(
		"default email sender is set declaratively and cannot be overridden")
	// errUnknownDefaultEmailSender is returned when the default sender does not exist.
	errUnknownDefaultEmailSender = errors.New("default email sender does not resolve to an existing sender")
	// errDefaultSenderNotEmail is returned when the default sender is not an email sender.
	errDefaultSenderNotEmail = errors.New("default email sender is not an email sender")
	// errDefaultSenderLookupFailed is returned when the sender lookup fails.
	errDefaultSenderLookupFailed = errors.New("failed to resolve default email sender")
)

// serverConfig is the value of the "notification" server-config section.
type serverConfig struct {
	// DefaultEmailSenderID names the email sender used by server-sent email that is not part of a
	// flow, such as the account lock notice. Flow email nodes still name their own sender.
	DefaultEmailSenderID string `json:"defaultEmailSenderId" yaml:"defaultEmailSenderId"`
}

// ServerConfigHandler decodes, validates and merges the "notification" section.
type ServerConfigHandler struct {
	senders NotificationSenderMgtSvcInterface
}

// NewServerConfigHandler creates the "notification" section handler.
func NewServerConfigHandler(senders NotificationSenderMgtSvcInterface) *ServerConfigHandler {
	if senders == nil {
		panic("notification server config handler requires a non-nil sender management service")
	}
	return &ServerConfigHandler{senders: senders}
}

// Decode parses a raw section value. Empty input yields the zero value.
func (*ServerConfigHandler) Decode(raw json.RawMessage) (any, error) {
	if len(raw) == 0 {
		return serverConfig{}, nil
	}
	var cfg serverConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate refuses a write while the sender is set declaratively, and checks that a named sender
// exists and sends email.
func (h *ServerConfigHandler) Validate(incoming, readOnly, _ any) error {
	if ro, ok := readOnly.(serverConfig); ok && ro.DefaultEmailSenderID != "" {
		return errDeclarativeSenderLocked
	}
	cfg, _ := incoming.(serverConfig)
	senderID := strings.TrimSpace(cfg.DefaultEmailSenderID)
	if senderID == "" {
		return nil
	}
	sender, svcErr := h.senders.GetSender(context.Background(), senderID)
	if svcErr != nil {
		if svcErr.Code == ErrorSenderNotFound.Code {
			return errUnknownDefaultEmailSender
		}
		return errDefaultSenderLookupFailed
	}
	if sender.Type != common.NotificationSenderTypeEmail {
		return errDefaultSenderNotEmail
	}
	return nil
}

// Merge returns the declarative layer when it names a sender, otherwise the writable layer.
func (*ServerConfigHandler) Merge(readOnly, writable any) any {
	if ro, ok := readOnly.(serverConfig); ok && ro.DefaultEmailSenderID != "" {
		return ro
	}
	if wr, ok := writable.(serverConfig); ok {
		return wr
	}
	return serverConfig{}
}

// DefaultEmailSenderInterface names the server's default email sender.
type DefaultEmailSenderInterface interface {
	// DefaultEmailSenderID returns the configured default email sender, or "" when none is set.
	DefaultEmailSenderID(ctx context.Context) string
}

// defaultEmailSender reads the server's default email sender on every call. Without a reader, or
// when a read fails, there is no default sender.
type defaultEmailSender struct {
	reader serverconfig.ServerConfigService
	logger *log.Logger
}

// NewDefaultEmailSender creates a default sender source over the server-config reader.
func NewDefaultEmailSender(reader serverconfig.ServerConfigService) DefaultEmailSenderInterface {
	return &defaultEmailSender{
		reader: reader,
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "DefaultEmailSender")),
	}
}

// DefaultEmailSenderID returns the configured default email sender, or "" when none is set.
func (d *defaultEmailSender) DefaultEmailSenderID(ctx context.Context) string {
	if d.reader == nil {
		return ""
	}
	value, svcErr := d.reader.GetMergedConfig(ctx, string(serverconfig.ConfigNameNotification))
	if svcErr != nil {
		d.logger.Warn(ctx, "Failed to read the notification server config; no default email sender",
			log.String("code", svcErr.Code))
		return ""
	}
	cfg, _ := value.(serverConfig)
	return strings.TrimSpace(cfg.DefaultEmailSenderID)
}
