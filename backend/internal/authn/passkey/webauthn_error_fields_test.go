/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package passkey

import (
	"errors"
	"fmt"
	"testing"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/stretchr/testify/assert"

	"github.com/thunder-id/thunderid/internal/system/log"
)

// fieldMap collapses the log fields into a map keyed by field name for easy assertions.
func fieldMap(fields []log.Field) map[string]interface{} {
	m := make(map[string]interface{}, len(fields))
	for _, f := range fields {
		m[f.Key] = f.Value
	}
	return m
}

// TestWebAuthnErrorFields_ProtocolError verifies that a go-webauthn protocol error is broken
// down into its category, detail and debug info for server-side logging.
func TestWebAuthnErrorFields_ProtocolError(t *testing.T) {
	libErr := protocol.ErrVerification.
		WithDetails("stored challenge and received challenge do not match").
		WithInfo("origin mismatch debug info")

	fields := fieldMap(webAuthnErrorFields(libErr))

	assert.Equal(t, "verification_error", fields["webAuthnErrorType"])
	assert.Equal(t, "stored challenge and received challenge do not match", fields["webAuthnErrorDetail"])
	assert.Equal(t, "origin mismatch debug info", fields["webAuthnErrorDebug"])
	// A plain "error" field must not be used for protocol errors; the detail is structured.
	_, hasPlainError := fields["error"]
	assert.False(t, hasPlainError)
}

// TestWebAuthnErrorFields_WrappedProtocolError verifies a protocol error wrapped with
// fmt.Errorf is still recognized through errors.As.
func TestWebAuthnErrorFields_WrappedProtocolError(t *testing.T) {
	wrapped := fmt.Errorf("failed to validate assertion: %w", protocol.ErrAssertionSignature)

	fields := fieldMap(webAuthnErrorFields(wrapped))

	assert.Equal(t, "invalid_signature", fields["webAuthnErrorType"])
}

// TestWebAuthnErrorFields_NonProtocolError verifies non-protocol errors fall back to a single
// error field carrying the message.
func TestWebAuthnErrorFields_NonProtocolError(t *testing.T) {
	fields := fieldMap(webAuthnErrorFields(errors.New("some plain error")))

	assert.Equal(t, "some plain error", fields["error"])
	_, hasType := fields["webAuthnErrorType"]
	assert.False(t, hasType)
}
