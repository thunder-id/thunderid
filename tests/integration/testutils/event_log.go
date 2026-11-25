// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// eventWaitTimeout bounds how long a reader waits for an event to reach the sink. The file
	// adapter buffers writes and flushes on a five second ticker, so the bound has to clear that with
	// room to spare.
	eventWaitTimeout = 30 * time.Second

	// eventPollInterval is how often the log is re-read while waiting.
	eventPollInterval = 250 * time.Millisecond
)

// ObservabilityEvent mirrors the JSON the observability file sink writes for one event.
type ObservabilityEvent struct {
	TraceID   string                 `json:"trace_id"`
	EventID   string                 `json:"event_id"`
	Type      string                 `json:"type"`
	Component string                 `json:"component"`
	Status    string                 `json:"status"`
	Data      map[string]interface{} `json:"data"`
	// Raw is the record as the sink wrote it, for a check on anything the fields above omit.
	Raw string `json:"-"`
}

// Str returns the event's data value for key as a string, or "" when the key is absent or holds a
// non-string value.
func (e ObservabilityEvent) Str(key string) string {
	value, _ := e.Data[key].(string)
	return value
}

// Has reports whether the event carries the given data key at all.
func (e ObservabilityEvent) Has(key string) bool {
	_, ok := e.Data[key]
	return ok
}

// EventOfTypeWith matches events of the given type carrying the given data key/value pair.
func EventOfTypeWith(eventType, key, value string) func(ObservabilityEvent) bool {
	return func(evt ObservabilityEvent) bool {
		return evt.Type == eventType && evt.Str(key) == value
	}
}

// EventLogPath is a file sink path under the extracted product home, so the log is cleaned up with the
// rest of the test distribution.
func EventLogPath(fileName string) string {
	return filepath.Join(GetExtractedProductHome(), "logs", "observability", fileName)
}

// ObservabilityFileSinkPatch builds the deployment.yaml observability section that writes every event,
// as JSON, to path. Enabling only the file sink keeps the events out of the server's stdout, which the
// harness already uses for its own logs.
func ObservabilityFileSinkPatch(path string, enabled bool) map[string]interface{} {
	return map[string]interface{}{
		"observability": map[string]interface{}{
			"enabled": enabled,
			"output": map[string]interface{}{
				"file": map[string]interface{}{
					"enabled":    enabled,
					"file_path":  path,
					"format":     "json",
					"categories": []string{"observability.all"},
				},
			},
		},
	}
}

// EventLogReader tails the event log from the offset it last read, so each scenario sees only the
// events its own requests produced rather than every event the suite has published so far.
type EventLogReader = logReader[ObservabilityEvent]

// NewEventLogReader returns a reader of the file sink at path.
func NewEventLogReader(path string) *EventLogReader {
	return &EventLogReader{path: path, parse: parseObservabilityEvent}
}

// parseObservabilityEvent parses one record of the observability file sink, every one of which is JSON.
func parseObservabilityEvent(line string) (ObservabilityEvent, bool, error) {
	var evt ObservabilityEvent
	if err := json.Unmarshal([]byte(line), &evt); err != nil {
		return ObservabilityEvent{}, false, err
	}
	evt.Raw = line
	return evt, true, nil
}

// logReader tails a log from the offset it last read and keeps the records parse accepts.
type logReader[T any] struct {
	path string
	// parse returns the record a line holds, false for a line that holds none, and an error for a
	// line that should hold one and cannot be read.
	parse   func(line string) (T, bool, error)
	offset  int64
	batched []T
}

// Reset drops everything read so far and moves the offset past the last complete record, so the next
// read returns only records written after this call. Call it immediately before the requests under
// test.
//
// The offset lands after the last newline rather than at the end of the file: a buffered writer can
// flush mid-record, and an offset inside one would make the next read start on a fragment. Any bytes
// of that partial record are re-read once the record is complete.
func (r *logReader[T]) Reset() error {
	r.batched = nil

	data, err := os.ReadFile(r.path)
	if os.IsNotExist(err) {
		r.offset = 0
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to read the log %s: %w", r.path, err)
	}

	r.offset = int64(bytes.LastIndexByte(data, '\n') + 1)
	return nil
}

// Await polls the log until a record matching the predicate appears, and returns it. It reports an
// error, rather than the zero record, when the wait times out, so the caller can fail with a message
// naming what it was waiting for.
func (r *logReader[T]) Await(match func(T) bool) (T, error) {
	deadline := time.Now().Add(eventWaitTimeout)
	for {
		if err := r.poll(); err != nil {
			var zero T
			return zero, err
		}
		for _, record := range r.batched {
			if match(record) {
				return record, nil
			}
		}
		if time.Now().After(deadline) {
			var zero T
			return zero, fmt.Errorf(
				"no matching record within %s (%d records read)", eventWaitTimeout, len(r.batched))
		}
		time.Sleep(eventPollInterval)
	}
}

// Read returns every record written since the last Reset that has reached the log. It does not wait,
// so a caller counting records first awaits one written after them.
func (r *logReader[T]) Read() ([]T, error) {
	if err := r.poll(); err != nil {
		return nil, err
	}
	records := make([]T, len(r.batched))
	copy(records, r.batched)
	return records, nil
}

// poll reads whatever complete lines have been appended since the last read and adds their records to
// the batch. A partial trailing line is left for the next poll: a buffered writer can flush mid-record
// once a record straddles its buffer.
func (r *logReader[T]) poll() error {
	file, err := os.Open(r.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to open the log %s: %w", r.path, err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat the log %s: %w", r.path, err)
	}
	if info.Size() <= r.offset {
		return nil
	}

	buf := make([]byte, info.Size()-r.offset)
	read, err := file.ReadAt(buf, r.offset)
	if err != nil && read == 0 {
		return fmt.Errorf("failed to read the log %s: %w", r.path, err)
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
		record, ok, err := r.parse(line)
		if err != nil {
			return fmt.Errorf("failed to parse the log line %q: %w", line, err)
		}
		if ok {
			r.batched = append(r.batched, record)
		}
	}

	r.offset += int64(lastNewline) + 1
	return nil
}
