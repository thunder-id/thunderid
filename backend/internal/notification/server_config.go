// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notification

import (
	"bytes"
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

// Validate refuses a write over a declarative default sender.
func (*serverConfigHandler) Validate(_, readOnly, _ any) error {
	if ro, ok := readOnly.(notificationconfig.NotificationSectionConfig); ok && ro.DefaultEmailSenderID != "" {
		return errDeclarativeSenderLocked
	}
	return nil
}

// Merge returns the declarative layer when it names a sender, otherwise the writable layer.
func (*serverConfigHandler) Merge(readOnly, writable any) any {
	if ro, ok := readOnly.(notificationconfig.NotificationSectionConfig); ok && ro.DefaultEmailSenderID != "" {
		return ro
	}
	if wr, ok := writable.(notificationconfig.NotificationSectionConfig); ok {
		return wr
	}
	return notificationconfig.NotificationSectionConfig{}
}
