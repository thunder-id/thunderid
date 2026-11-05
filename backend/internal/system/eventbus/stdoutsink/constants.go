// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package stdoutsink

const (
	// logTypeAudit is the value of the marker that starts every audit line, so a collector can route
	// audit lines apart from diagnostic lines, which carry no marker.
	logTypeAudit = "AUDIT"
	// schemaVersion is the version of the line schema. Fields are only added within a version.
	schemaVersion = 1
	// maxLineBytes caps a line, newline included, at the pipe atomic write size on Linux, so lines
	// written concurrently to one stdout do not interleave.
	maxLineBytes = 4096
	// maxFieldBytes caps each free-form field once dropping details and cutting the user agent did not
	// bring a line under maxLineBytes.
	maxFieldBytes = 256
	// timeLayout renders the event time in UTC with exactly six fractional digits.
	timeLayout = "2006-01-02T15:04:05.000000Z07:00"
)
