// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"errors"
	"net/mail"
	"strings"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
)

// Validate trims the address lists and subject in place and rejects a payload that has no
// recipient, a malformed address, or a subject carrying a header-injection sequence.
func (e *EmailData) Validate() error {
	// Reject CR/LF before trimming so they cannot be trimmed away.
	for _, addresses := range [][]string{e.To, e.CC, e.BCC} {
		for _, address := range addresses {
			if !IsValidHeaderText(address) {
				return errors.New("invalid recipient address")
			}
		}
	}
	if !IsValidHeaderText(e.Subject) {
		return errors.New("subject contains invalid characters")
	}

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
				return errors.New("invalid recipient address")
			}
		}
	}

	return nil
}

// IsValidHeaderText reports whether value is safe to place inside a message header. A value
// carrying CR or LF would end the header and let the rest be read as headers of its own, which
// is how a configured display name turns into an injected Bcc.
func IsValidHeaderText(value string) bool {
	return !strings.ContainsAny(value, serverconst.CRLF)
}

// IsValidEmailAddress reports whether the given value is a syntactically valid bare email
// address. Display-name forms ("Bob" <bob@example.com>) are rejected, as is any value
// containing CR or LF, which would allow header injection.
func IsValidEmailAddress(address string) bool {
	// Reject CR/LF before trimming so they cannot be trimmed away.
	if strings.ContainsAny(address, serverconst.CRLF) {
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

// ParseTLSMode returns the TLS mode for the given value. An empty value yields the
// secure default, STARTTLS.
func ParseTLSMode(value string) (TLSMode, bool) {
	switch TLSMode(strings.ToLower(strings.TrimSpace(value))) {
	case "":
		return TLSModeSTARTTLS, true
	case TLSModeNone:
		return TLSModeNone, true
	case TLSModeSTARTTLS:
		return TLSModeSTARTTLS, true
	case TLSModeImplicit:
		return TLSModeImplicit, true
	default:
		return "", false
	}
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
