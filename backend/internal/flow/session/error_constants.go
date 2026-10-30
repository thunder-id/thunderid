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

	// ErrTokenFamilyNotInSession is returned by RevokeSubjectSessions when the session to keep can't
	// be found, because no session of the subject holds the token family.
	ErrTokenFamilyNotInSession = errors.New("token family not held by any session of the subject")
)
