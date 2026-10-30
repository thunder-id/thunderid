// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocation

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// wire round-trips claims through JSON, as a signed token does, so numbers arrive as float64.
func wire(t *testing.T, claims map[string]interface{}) map[string]interface{} {
	t.Helper()
	raw, err := json.Marshal(claims)
	require.NoError(t, err)
	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &decoded))
	return decoded
}

func TestEvent_RoundTrip(t *testing.T) {
	revokedAt := time.Unix(1_700_000_000, 0).UTC()
	expiry := revokedAt.Add(time.Hour)

	tests := []struct {
		name      string
		event     Event
		eventType string
	}{
		{
			name: "token",
			event: Event{JTI: "jti-1", Reason: ReasonExplicit, RevokedAt: revokedAt,
				ExpiryTime: expiry},
			eventType: EventTypeTokenRevoked,
		},
		{
			name: "criterion",
			event: Event{Criterion: Criterion{Type: CriterionTypeSubject, Value: "user-1"},
				Reason: ReasonIdentitySuspended, RevokedAt: revokedAt, ExpiryTime: expiry},
			eventType: EventTypeCriterionRevoked,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.eventType, tt.event.EventType())

			parsed, err := ParseEvent(wire(t, tt.event.Claims()))

			require.NoError(t, err)
			assert.Equal(t, tt.event, parsed)
		})
	}
}

func TestParseEvent_Rejects(t *testing.T) {
	valid := func(payload map[string]interface{}) map[string]interface{} {
		return map[string]interface{}{"events": map[string]interface{}{EventTypeCriterionRevoked: payload}}
	}
	base := func() map[string]interface{} {
		return map[string]interface{}{
			"criterion_type": "subject", "criterion_value": "user-1", "reason": "explicit",
			"revoked_at": float64(1), "expiry_time": float64(2),
		}
	}
	without := func(key string) map[string]interface{} {
		payload := base()
		delete(payload, key)
		return valid(payload)
	}

	tests := []struct {
		name   string
		claims map[string]interface{}
	}{
		{"no events claim", map[string]interface{}{}},
		{"empty events", map[string]interface{}{"events": map[string]interface{}{}}},
		{"payload not an object", map[string]interface{}{"events": map[string]interface{}{
			EventTypeCriterionRevoked: "x"}}},
		{"missing reason", without("reason")},
		{"missing revoked_at", without("revoked_at")},
		{"missing expiry_time", without("expiry_time")},
		{"missing criterion type", without("criterion_type")},
		{"missing criterion value", without("criterion_value")},
		{"token event without jti", map[string]interface{}{"events": map[string]interface{}{
			EventTypeTokenRevoked: map[string]interface{}{
				"reason": "explicit", "revoked_at": float64(1), "expiry_time": float64(2)}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseEvent(tt.claims)

			require.Error(t, err)
			assert.NotErrorIs(t, err, ErrUnsupportedEventType)
		})
	}
}

func TestParseEvent_UnsupportedEventType(t *testing.T) {
	_, err := ParseEvent(map[string]interface{}{
		"events": map[string]interface{}{"urn:example:other": map[string]interface{}{}}})

	assert.ErrorIs(t, err, ErrUnsupportedEventType)
}
