// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notification

// ServerConfig is the value of the "notification" server-config section.
type ServerConfig struct {
	// DefaultEmailSenderID names the email sender used by server-sent email that is not part of a
	// flow, such as the account lock notice. Flow email nodes still name their own sender.
	DefaultEmailSenderID string `json:"defaultEmailSenderId" yaml:"defaultEmailSenderId"`
}
