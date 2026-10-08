// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package backchannel

import "time"

// failureReason is the reason code recorded on a failed delivery.
type failureReason string

// Reason codes recorded on a failed delivery.
const (
	reasonUnreachable    failureReason = "unreachable"
	reasonServerError    failureReason = "server_error"
	reasonRejected       failureReason = "rejected"
	reasonPrivateAddress failureReason = "private_address"
	reasonClientNotFound failureReason = "client_not_found"
	reasonQueueFull      failureReason = "queue_full"
	reasonShutdown       failureReason = "shutdown"
)

const (
	// formParamLogoutToken is the single body parameter of a back-channel logout request (§2.5).
	formParamLogoutToken = "logout_token"
	// responseDrainLimit caps how much of a response body is read before it is discarded.
	responseDrainLimit = 4 << 10
	// defaultWorkers is the number of goroutines that expand terminations into deliveries.
	defaultWorkers = 4
	// clientShareDivisor sets each relying party's share of max_in_flight: a quarter, at least one,
	// so an endpoint that hangs cannot hold every permit while healthy ones wait.
	clientShareDivisor = 4
	// defaultShutdownBudget bounds Stop. The server's graceful shutdown has five seconds in total
	// and the observability service must still shut down afterwards.
	defaultShutdownBudget = 2 * time.Second
	// shutdownGrace is how long Stop waits, after canceling, for attempts to record their outcome.
	shutdownGrace = 250 * time.Millisecond
)
