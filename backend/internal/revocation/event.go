// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocation

import (
	"errors"
	"fmt"
	"time"
)

// Security event types carried in the events claim of a revocation Security Event Token (RFC 8417).
const (
	// EventTypeTokenRevoked announces the revocation of a single token, identified by its jti.
	EventTypeTokenRevoked = "urn:secevent:revocation:token" //nolint:gosec // event type URI, not a credential
	// EventTypeCriterionRevoked announces the revocation of every artifact matching a criterion.
	EventTypeCriterionRevoked = "urn:secevent:revocation:criterion"
)

// SecurityEventTokenType is the typ header of a Security Event Token (RFC 8417 §2.3).
const SecurityEventTokenType = "secevent+jwt" //nolint:gosec // JWT typ header value, not a credential

// ErrUnsupportedEventType is returned by ParseEvent for an event type this server does not handle.
var ErrUnsupportedEventType = errors.New("unsupported revocation event type")

// Keys of the event payload.
const (
	claimEvents         = "events"
	claimJTI            = "jti"
	claimCriterionType  = "criterion_type"
	claimCriterionValue = "criterion_value"
	claimReason         = "reason"
	claimRevokedAt      = "revoked_at"
	claimExpiryTime     = "expiry_time"
)

// Event is a deny-list entry as it was recorded, announced to the Resource Servers that enforce it.
type Event struct {
	// JTI is set for a single-token revocation and empty for a criterion revocation.
	JTI string
	// Criterion is set for a criterion revocation.
	Criterion  Criterion
	Reason     Reason
	RevokedAt  time.Time
	ExpiryTime time.Time
}

// EventType returns the event type URI this event is announced under.
func (e Event) EventType() string {
	if e.JTI != "" {
		return EventTypeTokenRevoked
	}
	return EventTypeCriterionRevoked
}

// Claims returns the claims a Security Event Token announcing this event carries besides the
// registered ones: a single events entry keyed by the event type.
func (e Event) Claims() map[string]interface{} {
	payload := map[string]interface{}{
		claimReason:     string(e.Reason),
		claimRevokedAt:  e.RevokedAt.Unix(),
		claimExpiryTime: e.ExpiryTime.Unix(),
	}
	if e.JTI != "" {
		payload[claimJTI] = e.JTI
	} else {
		payload[claimCriterionType] = string(e.Criterion.Type)
		payload[claimCriterionValue] = e.Criterion.Value
	}
	return map[string]interface{}{claimEvents: map[string]interface{}{e.EventType(): payload}}
}

// ParseEvent reads the event from the claims of a verified Security Event Token. A token that
// carries no event type this server handles returns ErrUnsupportedEventType.
func ParseEvent(claims map[string]interface{}) (Event, error) {
	events, ok := claims[claimEvents].(map[string]interface{})
	if !ok || len(events) == 0 {
		return Event{}, errors.New("security event token carries no events")
	}
	for eventType, raw := range events {
		if eventType != EventTypeTokenRevoked && eventType != EventTypeCriterionRevoked {
			continue
		}
		payload, ok := raw.(map[string]interface{})
		if !ok {
			return Event{}, fmt.Errorf("event %q payload is not an object", eventType)
		}
		return parseEventPayload(eventType, payload)
	}
	return Event{}, ErrUnsupportedEventType
}

func parseEventPayload(eventType string, payload map[string]interface{}) (Event, error) {
	reason, _ := payload[claimReason].(string)
	revokedAt, revokedOK := payload[claimRevokedAt].(float64)
	expiry, expiryOK := payload[claimExpiryTime].(float64)
	if reason == "" || !revokedOK || !expiryOK {
		return Event{}, errors.New("event is missing its reason, revoked_at or expiry_time")
	}
	event := Event{
		Reason:     Reason(reason),
		RevokedAt:  time.Unix(int64(revokedAt), 0).UTC(),
		ExpiryTime: time.Unix(int64(expiry), 0).UTC(),
	}
	if eventType == EventTypeTokenRevoked {
		event.JTI, _ = payload[claimJTI].(string)
		if event.JTI == "" {
			return Event{}, errors.New("token revocation event carries no jti")
		}
		return event, nil
	}
	criterionType, _ := payload[claimCriterionType].(string)
	criterionValue, _ := payload[claimCriterionValue].(string)
	if criterionType == "" || criterionValue == "" {
		return Event{}, errors.New("criterion revocation event carries no criterion")
	}
	event.Criterion = Criterion{Type: CriterionType(criterionType), Value: criterionValue}
	return event, nil
}
