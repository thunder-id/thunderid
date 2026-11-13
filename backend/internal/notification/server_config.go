// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notification

import (
	"bytes"
	"context"
	"encoding/json"

	notificationconfig "github.com/thunder-id/thunderid/internal/notification/config"
)

// serverConfigHandler decodes, validates and merges the "notification" section.
type serverConfigHandler struct{}

// newServerConfigHandler creates the "notification" section handler.
func newServerConfigHandler() *serverConfigHandler {
	return &serverConfigHandler{}
}

// Decode parses a raw section value. Empty input yields the zero value, and unknown fields are
// rejected.
func (*serverConfigHandler) Decode(raw json.RawMessage) (any, error) {
	if len(raw) == 0 {
		return notificationconfig.NotificationSectionConfig{}, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()

	var cfg notificationconfig.NotificationSectionConfig
	if err := decoder.Decode(&cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate accepts any decoded value: the sender is resolved when it is used, not when it is set.
func (*serverConfigHandler) Validate(_ context.Context, _, _, _ any) error {
	return nil
}

// Merge returns the declarative layer when it names a sender, otherwise the writable layer.
func (h *serverConfigHandler) Merge(readOnly, writable any) any {
	if h.IsLocked(readOnly) {
		return readOnly
	}
	if wr, ok := writable.(notificationconfig.NotificationSectionConfig); ok {
		return wr
	}
	return notificationconfig.NotificationSectionConfig{}
}

// Default returns nil: the section has no default.
func (*serverConfigHandler) Default() any {
	return nil
}

// IsLocked reports whether a declarative document names the default sender.
func (*serverConfigHandler) IsLocked(readOnly any) bool {
	ro, ok := readOnly.(notificationconfig.NotificationSectionConfig)
	return ok && ro.DefaultEmailSenderID != ""
}
