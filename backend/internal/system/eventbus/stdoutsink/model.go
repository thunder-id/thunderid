// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package stdoutsink

// line is the JSON shape of one audit line. The marker is the first field, so it leads the line.
type line struct {
	LogType       string            `json:"log_type"`
	SchemaVersion int               `json:"schema_version"`
	ID            string            `json:"id"`
	Time          string            `json:"time"`
	Event         string            `json:"event"`
	TraceID       string            `json:"trace_id"`
	Actor         refLine           `json:"actor"`
	OnBehalfOf    *refLine          `json:"on_behalf_of,omitempty"`
	Subject       *refLine          `json:"subject,omitempty"`
	Target        refLine           `json:"target"`
	Action        string            `json:"action"`
	Outcome       outcomeLine       `json:"outcome"`
	Client        *clientLine       `json:"client,omitempty"`
	Details       map[string]string `json:"details,omitempty"`
}

// refLine is an actor, a subject or a target: a type and, where one exists, an ID.
type refLine struct {
	Type string `json:"type"`
	ID   string `json:"id,omitempty"`
}

// outcomeLine is how the attempt ended.
type outcomeLine struct {
	Result string `json:"result"`
	Reason string `json:"reason,omitempty"`
}

// clientLine is where the request came from.
type clientLine struct {
	Address   string `json:"address,omitempty"`
	UserAgent string `json:"user_agent,omitempty"`
}
