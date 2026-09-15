// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package observability verifies, end to end against a running server, that the observability events
// published for authentication flows and token issuance describe the principals involved: which
// entity acted, which entity the credential was issued for, and whether the two differ.
//
// The events are read back from the file sink, which is the only client-visible surface they have.
package observability

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	// eventWaitTimeout bounds how long a scenario waits for the events it triggered to reach the
	// sink. The file adapter buffers writes and flushes on a five second ticker, so the bound has to
	// clear that with room to spare.
	eventWaitTimeout = 30 * time.Second

	// eventPollInterval is how often the log is re-read while waiting.
	eventPollInterval = 250 * time.Millisecond
)

// Event data keys asserted by these tests. They mirror event.DataKey in the server, which the tests
// cannot import: the integration suite is a separate module driving the server over HTTP. Spelling
// them out here is deliberate, since the key names are the contract an event consumer reads.
const (
	keyAppID         = "app_id"
	keyClientID      = "client_id"
	keyCorrelationID = "correlation_id"
	keyActorType     = "act_type"
	keyActorSub      = "act_sub"
	keySubject       = "sub"
	keySubjectType   = "sub_type"
	keyIsDelegated   = "is_delegated"
	keyNodeID        = "node_id"
	keyGrantType     = "grant_type"
	keyExecutionID   = "execution_id"
)

// Event types asserted by these tests.
const (
	typeFlowStarted         = "FLOW_STARTED"
	typeFlowCompleted       = "FLOW_COMPLETED"
	typeNodeExecCompleted   = "FLOW_NODE_EXECUTION_COMPLETED"
	typeTokenIssued         = "TOKEN_ISSUED"
	typeTokenIssuanceFailed = "TOKEN_ISSUANCE_FAILED"
)

// Principal type values asserted by these tests. They are the wire values of the token's sub_type
// claim, so "app" the entity category is reported as "application" here.
const (
	principalUser        = "user"
	principalAgent       = "agent"
	principalApplication = "application"
)

// observabilityEvent mirrors the JSON the file sink writes for one event.
type observabilityEvent struct {
	TraceID   string                 `json:"trace_id"`
	EventID   string                 `json:"event_id"`
	Type      string                 `json:"type"`
	Component string                 `json:"component"`
	Status    string                 `json:"status"`
	Data      map[string]interface{} `json:"data"`
}

// str returns the event's data value for key as a string, or "" when the key is absent or holds a
// non-string value.
func (e observabilityEvent) str(key string) string {
	value, _ := e.Data[key].(string)
	return value
}

// has reports whether the event carries the given data key at all.
func (e observabilityEvent) has(key string) bool {
	_, ok := e.Data[key]
	return ok
}

// eventLogPath is where the suite points the file sink. It sits under the extracted product home so
// it is cleaned up with the rest of the test distribution.
func eventLogPath() string {
	return filepath.Join(testutils.GetExtractedProductHome(), "logs", "observability", "integration-events.log")
}

// observabilityPatch builds the deployment.yaml observability section. Enabling only the file sink
// keeps the events out of the server's stdout, which the harness already uses for its own logs.
func observabilityPatch(enabled bool) map[string]interface{} {
	return map[string]interface{}{
		"observability": map[string]interface{}{
			"enabled": enabled,
			"output": map[string]interface{}{
				"file": map[string]interface{}{
					"enabled":    enabled,
					"file_path":  eventLogPath(),
					"format":     "json",
					"categories": []string{"observability.all"},
				},
			},
		},
	}
}

// eventLogReader tails the event log from the offset it last read, so each scenario sees only the
// events its own requests produced rather than every event the suite has published so far.
type eventLogReader struct {
	path    string
	offset  int64
	batched []observabilityEvent
}

func newEventLogReader() *eventLogReader {
	return &eventLogReader{path: eventLogPath()}
}

// reset drops everything read so far and moves the offset past the last complete record, so the next
// read returns only events published after this call. Call it immediately before the requests under
// test.
//
// The offset lands after the last newline rather than at the end of the file: the sink's buffered
// writer can flush mid-record, and an offset inside one would make the next read start on a
// fragment. Any bytes of that partial record are re-read once the record is complete.
func (r *eventLogReader) reset() error {
	r.batched = nil

	data, err := os.ReadFile(r.path)
	if os.IsNotExist(err) {
		r.offset = 0
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to read the event log %s: %w", r.path, err)
	}

	r.offset = int64(bytes.LastIndexByte(data, '\n') + 1)
	return nil
}

// poll reads whatever complete lines have been appended since the last read and adds them to the
// batch. A partial trailing line is left for the next poll: the sink's buffered writer can flush
// mid-record once a record straddles its buffer.
func (r *eventLogReader) poll() error {
	file, err := os.Open(r.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to open the event log %s: %w", r.path, err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat the event log %s: %w", r.path, err)
	}
	if info.Size() <= r.offset {
		return nil
	}

	buf := make([]byte, info.Size()-r.offset)
	read, err := file.ReadAt(buf, r.offset)
	if err != nil && read == 0 {
		return fmt.Errorf("failed to read the event log %s: %w", r.path, err)
	}

	chunk := string(buf[:read])
	lastNewline := strings.LastIndex(chunk, "\n")
	if lastNewline < 0 {
		return nil
	}

	for _, line := range strings.Split(chunk[:lastNewline], "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var evt observabilityEvent
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			return fmt.Errorf("failed to parse the event log line %q: %w", line, err)
		}
		r.batched = append(r.batched, evt)
	}

	r.offset += int64(lastNewline) + 1
	return nil
}

// await polls the log until an event matching the predicate appears, and returns it. It reports an
// error, rather than the zero event, when the wait times out, so the caller can fail with a message
// naming what it was waiting for.
func (r *eventLogReader) await(match func(observabilityEvent) bool) (observabilityEvent, error) {
	deadline := time.Now().Add(eventWaitTimeout)
	for {
		if err := r.poll(); err != nil {
			return observabilityEvent{}, err
		}
		for _, evt := range r.batched {
			if match(evt) {
				return evt, nil
			}
		}
		if time.Now().After(deadline) {
			return observabilityEvent{}, fmt.Errorf(
				"no matching event within %s (%d events read)", eventWaitTimeout, len(r.batched))
		}
		time.Sleep(eventPollInterval)
	}
}

// ofTypeWith matches events of the given type carrying the given data key/value pair.
func ofTypeWith(eventType, key, value string) func(observabilityEvent) bool {
	return func(evt observabilityEvent) bool {
		return evt.Type == eventType && evt.str(key) == value
	}
}
