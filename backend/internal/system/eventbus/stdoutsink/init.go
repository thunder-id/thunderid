// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package stdoutsink is the default audit sink. It writes each audit event to standard output as one
// JSON line that starts with the log_type marker, whatever the diagnostic log level or format.
package stdoutsink

import (
	"os"

	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
)

// Initialize creates the stdout sink.
func Initialize() audit.Sink {
	return newStdoutSink(os.Stdout)
}
