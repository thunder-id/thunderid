// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package eventbus

import (
	"regexp"
	"time"
)

const (
	loggerComponentName = "EventBus"
	queueSize           = 1024
	// flushWait bounds how long Stop lets the queues drain before it cancels the deliveries in flight.
	flushWait          = 5 * time.Second
	maxEventNameLength = 64
	// maxTraceIDLength caps a trace ID, which comes from an unauthenticated header and is kept by every
	// queued delivery.
	maxTraceIDLength = 128
	// invalidEventName stands in for a name that fails the grammar, so a caller's text never reaches the log.
	invalidEventName = "invalid"
)

// eventNamePattern is the grammar of an event name: two to four lowercase segments joined by dots.
var eventNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*){1,3}$`)
