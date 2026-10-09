// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernance

import "errors"

// Service-level errors. A refusal is returned as a Decision, not as one of these.
var (
	// ErrNoEntityStateProvider is returned when the service is built without an EntityStateProvider.
	ErrNoEntityStateProvider = errors.New("identity governance requires an entity state port")

	// ErrNoSubject is returned when a lifecycle operation names no identity.
	ErrNoSubject = errors.New("identity governance operation requires a subject")
	// ErrNotAuthorized is returned when the caller may not act on the subject, or authorization could
	// not be decided.
	ErrNotAuthorized = errors.New("caller is not authorized to govern this subject")

	// ErrNotApplicable is returned when the operation does not apply to the subject's current state,
	// such as unsuspending an identity that is not suspended.
	ErrNotApplicable = errors.New("the operation does not apply to the subject's current state")
)
