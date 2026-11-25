// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"encoding/json"
	"strings"
)

// auditLinePrefix starts every audit line the server writes with the log_type marker.
const auditLinePrefix = `{"log_type":"AUDIT",`

// AuditRef is an actor, the principal it acted for, a subject or a target of an audit line.
type AuditRef struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// AuditLine mirrors one audit line the server writes to standard output.
type AuditLine struct {
	LogType       string            `json:"log_type"`
	SchemaVersion int               `json:"schema_version"`
	ID            string            `json:"id"`
	Time          string            `json:"time"`
	Event         string            `json:"event"`
	TraceID       string            `json:"trace_id"`
	Actor         AuditRef          `json:"actor"`
	OnBehalfOf    AuditRef          `json:"on_behalf_of"`
	Subject       AuditRef          `json:"subject"`
	Target        AuditRef          `json:"target"`
	Action        string            `json:"action"`
	Outcome       map[string]string `json:"outcome"`
	Client        map[string]string `json:"client"`
	Details       map[string]string `json:"details"`
	// Raw is the line as the server wrote it, for a check on anything the fields above omit.
	Raw string `json:"-"`
}

// AuditLogReader tails the server's standard output for audit lines.
type AuditLogReader = logReader[AuditLine]

// NewAuditLogReader returns a reader of the audit lines in the server log. The server writes there
// only when a test subprocess started it, so a suite restarts the server before reading.
func NewAuditLogReader() *AuditLogReader {
	return &AuditLogReader{path: ServerLogPath(), parse: parseAuditLine}
}

// parseAuditLine parses a line that starts with the audit marker and skips every other line, such as
// a diagnostic or access log line.
func parseAuditLine(line string) (AuditLine, bool, error) {
	if !strings.HasPrefix(line, auditLinePrefix) {
		return AuditLine{}, false, nil
	}
	var audit AuditLine
	if err := json.Unmarshal([]byte(line), &audit); err != nil {
		return AuditLine{}, false, err
	}
	audit.Raw = line
	return audit, true, nil
}
