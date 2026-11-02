// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package backchannel

import (
	"time"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// termination is the listener's copy of a terminated session, so the caller's data never crosses
// into a goroutine.
type termination struct {
	traceID   string
	sessionID string
	subjectID string
	appIDs    []string
}

// job is one participant of one termination: the dispatcher's unit of work.
type job struct {
	traceID   string
	sessionID string
	subjectID string
	appID     string
	client    *providers.OAuthClient
	attempt   int
}

// outcome classifies one attempt.
type outcome struct {
	delivered  bool
	retryable  bool
	reason     failureReason
	status     int
	retryAfter time.Duration
	// err is the cause of a failure, logged with the retry or the terminal record.
	err error
}
