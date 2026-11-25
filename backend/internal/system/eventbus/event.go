// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package eventbus

import (
	"context"
	"reflect"

	sysContext "github.com/thunder-id/thunderid/internal/system/context"
)

// Event is one fact with the payloads of the categories it belongs to.
type Event struct {
	envelope Envelope
	payloads map[Category]Payload
	err      error
}

// NewEvent starts an event named name, taking the request's trace from ctx. A malformed event is
// reported by Publish, so the call chain stays fluent.
func NewEvent(ctx context.Context, name string) *Event {
	traceID, _ := ctx.Value(sysContext.TraceIDKey).(string)
	if len(traceID) > maxTraceIDLength {
		traceID = traceID[:maxTraceIDLength]
	}
	evt := &Event{
		envelope: Envelope{
			Name:    name,
			TraceID: traceID,
		},
		payloads: make(map[Category]Payload),
	}
	if !isValidEventName(name) {
		evt.err = errInvalidEventName
	}
	return evt
}

// With attaches a payload, which puts the event in the payload's category.
func (e *Event) With(payload Payload) *Event {
	if e.err != nil {
		return e
	}
	if payload == nil {
		e.err = errNilPayload
		return e
	}
	// A pointer payload would let one subscriber change the payload every other subscriber receives.
	if reflect.ValueOf(payload).Kind() == reflect.Pointer {
		e.err = errPointerPayload
		return e
	}
	category, err := payloadCategory(payload)
	if err != nil {
		e.err = err
		return e
	}
	if _, exists := e.payloads[category]; exists {
		e.err = errDuplicateCategory
		return e
	}
	e.payloads[category] = payload
	return e
}

// name returns the event name, empty for a nil event.
func (e *Event) name() string {
	if e == nil {
		return ""
	}
	return e.envelope.Name
}

// validate reports why the event cannot be published.
func (e *Event) validate() error {
	if e == nil {
		return errNilEvent
	}
	if e.err != nil {
		return e.err
	}
	if len(e.payloads) == 0 {
		return errNoPayload
	}
	return nil
}
