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

package revocation

import (
	"sync"
	"time"
)

const (
	// enforcementFailureThreshold is the number of consecutive deny-list read failures that trips
	// the circuit open.
	enforcementFailureThreshold = 5
	// enforcementOpenDuration is how long the circuit stays open before a single trial call is
	// allowed (half-open).
	enforcementOpenDuration = 30 * time.Second
)

// circuitState is the state of the runtime-persistent-DB circuit breaker.
type circuitState int

const (
	circuitClosed circuitState = iota
	circuitOpen
	circuitHalfOpen
)

// circuitBreaker is a minimal consecutive-failure circuit breaker guarding deny-list reads.
// While open it short-circuits calls so a failing runtime persistent DB is not hammered on every request.
type circuitBreaker struct {
	mu               sync.Mutex
	state            circuitState
	consecutiveFails int
	failureThreshold int
	openDuration     time.Duration
	openedAt         time.Time
}

// newCircuitBreaker creates a circuit breaker in the closed state.
func newCircuitBreaker(failureThreshold int, openDuration time.Duration) *circuitBreaker {
	return &circuitBreaker{
		state:            circuitClosed,
		failureThreshold: failureThreshold,
		openDuration:     openDuration,
	}
}

// allow reports whether a call may proceed. When the open window has elapsed it transitions to
// half-open and admits a single trial call; further callers are rejected until that probe resolves
// (via recordSuccess/recordFailure), so a burst of concurrent requests cannot all hit a still-failing
// runtime persistent DB on each reopen.
func (cb *circuitBreaker) allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == circuitOpen {
		if time.Since(cb.openedAt) >= cb.openDuration {
			cb.state = circuitHalfOpen
			return true
		}
		return false
	}
	if cb.state == circuitHalfOpen {
		// A trial probe is already in flight; admit only one until it resolves.
		return false
	}
	return true
}

// recordSuccess closes the circuit and resets the consecutive-failure count.
func (cb *circuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.consecutiveFails = 0
	cb.state = circuitClosed
}

// recordFailure registers a failure and returns true when this failure tripped the circuit open
// (a closed→open or half-open→open transition), so the caller can alert exactly once per trip.
func (cb *circuitBreaker) recordFailure() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == circuitHalfOpen {
		cb.state = circuitOpen
		cb.openedAt = time.Now()
		return true
	}

	cb.consecutiveFails++
	if cb.state == circuitClosed && cb.consecutiveFails >= cb.failureThreshold {
		cb.state = circuitOpen
		cb.openedAt = time.Now()
		return true
	}
	return false
}
