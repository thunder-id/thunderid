// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import "errors"

var (
	// errVersionConflict is returned by Update when the optimistic-lock version no longer
	// matches (the row was updated concurrently or no longer exists).
	errVersionConflict = errors.New("session version conflict")

	// errSessionContextTooLarge is returned when a serialized session context exceeds
	// MaxSessionContextBytes. The bounded snapshot keeps the sibling row small.
	errSessionContextTooLarge = errors.New("session context exceeds maximum size")

	// errSessionNotFound is returned when the session a load resolved no longer exists, as after a concurrent
	// sign-out.
	errSessionNotFound = errors.New("resolved session no longer exists")

	// errOUNotEligible is returned when the sign-in can no longer reuse the session because of the
	// organization unit it needs.
	errOUNotEligible = errors.New("the sign-in can no longer reuse the session")

	// errNilEventBus is returned when the service is initialized without the event bus it audits on.
	errNilEventBus = errors.New("the SSO session service needs an event bus")
)
