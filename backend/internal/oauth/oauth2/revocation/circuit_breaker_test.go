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
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type CircuitBreakerTestSuite struct {
	suite.Suite
}

func TestCircuitBreakerTestSuite(t *testing.T) {
	suite.Run(t, new(CircuitBreakerTestSuite))
}

// A fresh breaker starts closed and allows calls.
func (s *CircuitBreakerTestSuite) TestStartsClosedAndAllows() {
	cb := newCircuitBreaker(3, time.Minute)
	s.Assert().True(cb.allow())
	s.Assert().Equal(circuitClosed, cb.state)
}

// Failures below the threshold leave the circuit closed.
func (s *CircuitBreakerTestSuite) TestStaysClosedBelowThreshold() {
	cb := newCircuitBreaker(3, time.Minute)
	s.Assert().False(cb.recordFailure())
	s.Assert().False(cb.recordFailure())
	s.Assert().True(cb.allow(), "circuit should remain closed below the failure threshold")
}

// Reaching the threshold trips the circuit open and short-circuits calls.
func (s *CircuitBreakerTestSuite) TestTripsAtThreshold() {
	cb := newCircuitBreaker(3, time.Minute)
	s.Assert().False(cb.recordFailure())
	s.Assert().False(cb.recordFailure())
	s.Assert().True(cb.recordFailure(), "third failure should trip the circuit open")
	s.Assert().False(cb.allow(), "open circuit must short-circuit calls")
}

// A success resets the consecutive-failure count.
func (s *CircuitBreakerTestSuite) TestSuccessResetsFailureCount() {
	cb := newCircuitBreaker(3, time.Minute)
	cb.recordFailure()
	cb.recordFailure()
	cb.recordSuccess()
	s.Assert().False(cb.recordFailure())
	s.Assert().False(cb.recordFailure())
	s.Assert().True(cb.allow(), "success should have reset the consecutive-failure count")
}

// After the cooldown a trial call is allowed (half-open); a success closes the circuit.
func (s *CircuitBreakerTestSuite) TestHalfOpenAfterCooldownThenClose() {
	cb := newCircuitBreaker(1, time.Minute)
	s.Assert().True(cb.recordFailure())
	s.Assert().False(cb.allow())

	// Simulate the cooldown window elapsing.
	cb.openedAt = time.Now().Add(-2 * time.Minute)

	s.Assert().True(cb.allow(), "after cooldown a trial call is allowed (half-open)")
	s.Assert().Equal(circuitHalfOpen, cb.state)

	cb.recordSuccess()
	s.Assert().Equal(circuitClosed, cb.state)
	s.Assert().True(cb.allow())
}

// In the half-open state only a single trial probe is admitted; concurrent callers are rejected
// until the probe resolves.
func (s *CircuitBreakerTestSuite) TestHalfOpenAdmitsSingleProbe() {
	cb := newCircuitBreaker(1, time.Minute)
	s.Assert().True(cb.recordFailure(), "first failure trips the circuit open")
	cb.openedAt = time.Now().Add(-2 * time.Minute) // elapse the cooldown

	s.Assert().True(cb.allow(), "first caller after cooldown is the single trial probe")
	s.Assert().Equal(circuitHalfOpen, cb.state)
	s.Assert().False(cb.allow(), "a second caller must not enter the half-open trial window")
	s.Assert().False(cb.allow(), "still only one probe in flight until it resolves")

	// Once the probe succeeds the circuit closes and calls are admitted again.
	cb.recordSuccess()
	s.Assert().True(cb.allow())
}

// A failed trial call in the half-open state re-trips the circuit open.
func (s *CircuitBreakerTestSuite) TestHalfOpenFailureReopens() {
	cb := newCircuitBreaker(1, time.Minute)
	cb.recordFailure()
	cb.openedAt = time.Now().Add(-2 * time.Minute)

	s.Assert().True(cb.allow())
	s.Assert().Equal(circuitHalfOpen, cb.state)

	s.Assert().True(cb.recordFailure(), "half-open failure should re-trip and report a fresh open transition")
	s.Assert().False(cb.allow(), "circuit should be open again after a failed trial call")
}
