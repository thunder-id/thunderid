// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package eventbus

import "errors"

var (
	// errInvalidEventName is returned when an event name does not follow the grammar or is too long.
	errInvalidEventName = errors.New("event name does not follow the dotted lowercase grammar")
	// errNoPayload is returned when an event is published without a payload.
	errNoPayload = errors.New("event carries no payload")
	// errNilPayload is returned when a nil payload is attached to an event.
	errNilPayload = errors.New("payload is nil")
	// errDuplicateCategory is returned when two payloads of one category are attached to an event.
	errDuplicateCategory = errors.New("event already carries a payload of this category")
	// errNilEvent is returned when a nil event is published.
	errNilEvent = errors.New("event is nil")
	// errBusClosed is returned when an event is published or a subscriber registered after Stop.
	errBusClosed = errors.New("event bus is shut down")
	// errEmptySubscriberID is returned when a subscriber is registered without an id.
	errEmptySubscriberID = errors.New("subscriber id is empty")
	// errDuplicateSubscriberID is returned when a subscriber id is registered twice.
	errDuplicateSubscriberID = errors.New("subscriber id is already registered")
	// errNilDeliveryFunc is returned when a subscriber is registered without a delivery function.
	errNilDeliveryFunc = errors.New("subscriber delivery function is nil")
	// errEventID is returned when an event id cannot be generated.
	errEventID = errors.New("failed to generate an event id")
	// errUnsupportedPayloadType is returned when a subscriber's payload type cannot report its category.
	errUnsupportedPayloadType = errors.New("payload type cannot report its category")
	// errPayloadType is the failed delivery of a payload that is not of the subscriber's type.
	errPayloadType = errors.New("payload is not of the subscriber's type")
	// errPointerPayload is returned when a pointer payload is attached to an event.
	errPointerPayload = errors.New("payload is a pointer")
	// errPayloadPanicked is returned when a payload panics while reporting its category.
	errPayloadPanicked = errors.New("payload panicked")
)
