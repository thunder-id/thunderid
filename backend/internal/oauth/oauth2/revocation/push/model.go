// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package push

import (
	"time"

	sharedrevocation "github.com/thunder-id/thunderid/internal/revocation"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// job is one revocation event bound for one receiver: the dispatcher's unit of work.
type job struct {
	traceID  string
	event    sharedrevocation.Event
	receiver engineconfig.TokenRevocationReceiver
	attempt  int
}

// outcome classifies one attempt.
type outcome struct {
	delivered bool
	retryable bool
	reason    failureReason
	status    int
	// err is the cause of a failure, logged with the retry or the terminal record.
	err error
}

// failureReason is the reason code logged on a failed delivery.
type failureReason string

// Reason codes logged on a failed delivery.
const (
	reasonUnreachable    failureReason = "unreachable"
	reasonServerError    failureReason = "server_error"
	reasonRejected       failureReason = "rejected"
	reasonPrivateAddress failureReason = "private_address"
	reasonQueueFull      failureReason = "queue_full"
	reasonShutdown       failureReason = "shutdown"
	reasonNotStarted     failureReason = "not_started"
)

const (
	// eventValidityPeriod is the lifetime of one event token, in seconds. A token is built per attempt,
	// so it only has to outlive a single request.
	eventValidityPeriod = 300
	// responseDrainLimit caps how much of a response body is read before it is discarded.
	responseDrainLimit = 4 << 10
	// workerCount is the number of goroutines that deliver events.
	workerCount = 4
	// shutdownBudget bounds how long Stop lets queued and in-flight deliveries finish.
	shutdownBudget = 2 * time.Second
)
