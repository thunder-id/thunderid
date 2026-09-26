// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package common contains the common models and constants for notification package.
package common

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
)

// MessageData holds the channel-agnostic payload for sending a message notification.
type MessageData struct {
	Recipient string
	Body      string
}

// EmailData holds the payload for sending an email notification.
type EmailData struct {
	To      []string
	CC      []string
	BCC     []string
	Subject string
	Body    string
	IsHTML  bool
}

// IsValidHeaderText reports whether value is safe to place inside a message header. A value
// carrying CR or LF would end the header and let the rest be read as headers of its own, which
// is how a configured display name turns into an injected Bcc.
func IsValidHeaderText(value string) bool {
	return !strings.ContainsAny(value, CRLF)
}

// IsValidEmailAddress reports whether the given value is a syntactically valid bare email
// address. Display-name forms ("Bob" <bob@example.com>) are rejected, as is any value
// containing CR or LF, which would allow header injection.
func IsValidEmailAddress(address string) bool {
	// Reject CR/LF before trimming so they cannot be trimmed away.
	if strings.ContainsAny(address, CRLF) {
		return false
	}

	address = strings.TrimSpace(address)
	if address == "" {
		return false
	}

	parsed, err := mail.ParseAddress(address)
	if err != nil {
		return false
	}

	return parsed.Address == address
}

// Validate trims the address lists and subject in place and rejects a payload that has no
// recipient, a malformed address, or a subject carrying a header-injection sequence.
func (e *EmailData) Validate() error {
	e.Subject = strings.TrimSpace(e.Subject)
	e.To = trimAddresses(e.To)
	e.CC = trimAddresses(e.CC)
	e.BCC = trimAddresses(e.BCC)

	if len(e.To) == 0 {
		return errors.New("email must have at least one recipient address")
	}

	for _, addresses := range [][]string{e.To, e.CC, e.BCC} {
		for _, address := range addresses {
			if !IsValidEmailAddress(address) {
				return fmt.Errorf("invalid recipient address: %s", address)
			}
		}
	}

	if strings.ContainsAny(e.Subject, CRLF) {
		return errors.New("subject contains invalid characters")
	}

	return nil
}

// trimAddresses trims each address and drops the entries that trim to empty.
func trimAddresses(addresses []string) []string {
	if len(addresses) == 0 {
		return nil
	}
	trimmed := make([]string, 0, len(addresses))
	for _, address := range addresses {
		if value := strings.TrimSpace(address); value != "" {
			trimmed = append(trimmed, value)
		}
	}
	return trimmed
}

// NotificationSenderDTO represents the data transfer object for a notification sender.
type NotificationSenderDTO struct {
	ID          string                   `yaml:"id,omitempty"`
	Name        string                   `yaml:"name"`
	Description string                   `yaml:"description,omitempty"`
	Type        NotificationSenderType   `yaml:"-"`
	Provider    NotificationProviderType `yaml:"provider"`
	Properties  []cmodels.Property       `yaml:"properties,omitempty"`
}

// VerifyOTPDTO represents the service layer data structure for verifying an OTP.
type VerifyOTPDTO struct {
	SessionToken string
	OTPCode      string
}

// VerifyOTPResultDTO represents the service layer result for OTP verify operation.
type VerifyOTPResultDTO struct {
	Status        OTPVerifyStatus
	Recipient     string
	RecipientAttr string
}

// OTPConfig holds optional OTP generation overrides.
// Nil pointer = use server default.
type OTPConfig struct {
	Length                *int
	UseNumericOnly        *bool
	ValidityPeriodSeconds *int
}
