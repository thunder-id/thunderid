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

package session

import (
	"time"
)

// DefaultIdleTimeout is the maximum inactivity period before a session expires. The idle deadline
// slides forward on each activity touch.
const DefaultIdleTimeout = 30 * time.Minute

// DefaultAbsoluteTimeout is the maximum lifetime of a session regardless of activity. It is fixed
// at creation and never extended. It also bounds the transport cookie's max-age.
const DefaultAbsoluteTimeout = 8 * time.Hour

// Timeouts holds the resolved session lifetime durations used when minting and refreshing sessions.
type Timeouts struct {
	// Idle is the maximum inactivity period; the idle deadline slides on each activity touch.
	Idle time.Duration
	// Absolute is the maximum lifetime of a session regardless of activity.
	Absolute time.Duration
}

// DefaultTimeouts returns the built-in default session timeouts.
func DefaultTimeouts() Timeouts {
	return Timeouts{
		Idle:     DefaultIdleTimeout,
		Absolute: DefaultAbsoluteTimeout,
	}
}

// NewTimeouts builds session timeouts from per-field second values, falling back to the built-in
// default for any non-positive value. The idle window is clamped to the absolute lifetime so the
// pair is always valid — a defaulted idle can otherwise outrun a small configured absolute, which
// SessionConfig validation does not catch (it only compares the raw, positive config values).
func NewTimeouts(idleSeconds, absoluteSeconds int64) Timeouts {
	t := DefaultTimeouts()
	if idleSeconds > 0 {
		t.Idle = time.Duration(idleSeconds) * time.Second
	}
	if absoluteSeconds > 0 {
		t.Absolute = time.Duration(absoluteSeconds) * time.Second
	}
	if t.Idle > t.Absolute {
		t.Idle = t.Absolute
	}
	return t
}
