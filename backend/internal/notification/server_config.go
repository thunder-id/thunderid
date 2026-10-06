// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"

	"github.com/thunder-id/thunderid/internal/notification/common"
)

// serverConfigHandler decodes, validates and merges the "notification" section.
type serverConfigHandler struct {
	senders NotificationSenderMgtSvcInterface
}

// newServerConfigHandler creates the "notification" section handler.
func newServerConfigHandler(senders NotificationSenderMgtSvcInterface) *serverConfigHandler {
	return &serverConfigHandler{senders: senders}
}

// Decode parses a raw section value. Empty input yields the zero value, and unknown fields are
// rejected.
func (*serverConfigHandler) Decode(raw json.RawMessage) (any, error) {
	if len(raw) == 0 {
		return ServerConfig{}, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()

	var cfg ServerConfig
	if err := decoder.Decode(&cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate refuses a write while the sender is set declaratively, and checks that a named sender
// exists and sends email.
func (h *serverConfigHandler) Validate(incoming, readOnly, _ any) error {
	if ro, ok := readOnly.(ServerConfig); ok && ro.DefaultEmailSenderID != "" {
		return errDeclarativeSenderLocked
	}
	cfg, _ := incoming.(ServerConfig)
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
func (*serverConfigHandler) Merge(readOnly, writable any) any {
	if ro, ok := readOnly.(ServerConfig); ok && ro.DefaultEmailSenderID != "" {
		return ro
	}
	if wr, ok := writable.(ServerConfig); ok {
		return wr
	}
	return ServerConfig{}
}
