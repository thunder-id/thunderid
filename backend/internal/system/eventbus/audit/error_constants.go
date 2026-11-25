// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import "errors"

var (
	errUnknownEvent     = errors.New("event is not an audited event")
	errDetailNotAllowed = errors.New("audit details carry a key the event does not allow")
	errMissingTarget    = errors.New("audit target has no type")
	errMissingAction    = errors.New("audit action is empty")
	errUnknownResult    = errors.New("audit outcome is not succeeded, denied or failed")
	errMissingReason    = errors.New("a denied or failed audit outcome needs a reason")
	errInvalidReason    = errors.New("audit outcome reason is not a stable lowercase code")
	errNilSink          = errors.New("audit sink is nil")
)
