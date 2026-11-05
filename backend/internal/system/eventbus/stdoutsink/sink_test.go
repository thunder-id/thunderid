// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package stdoutsink

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

const (
	testUserID    = "01900000-0000-7000-8000-000000000030"
	testUserAgent = "Go-http-client/1.1"
)

var (
	testTarget = audit.Ref{Type: audit.TargetTypeUser, ID: testUserID}

	// lineTimePattern is the time format of a line: UTC with exactly six fractional digits.
	lineTimePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{6}Z$`)
)

// knownLine is how a consumer of schema version 1 reads a line.
type knownLine struct {
	LogType       string            `json:"log_type"`
	SchemaVersion int               `json:"schema_version"`
	ID            string            `json:"id"`
	Time          string            `json:"time"`
	Event         string            `json:"event"`
	TraceID       string            `json:"trace_id"`
	Actor         map[string]string `json:"actor"`
	OnBehalfOf    map[string]string `json:"on_behalf_of"`
	Subject       map[string]string `json:"subject"`
	Target        map[string]string `json:"target"`
	Action        string            `json:"action"`
	Outcome       map[string]string `json:"outcome"`
	Client        map[string]string `json:"client"`
	Details       map[string]string `json:"details"`
}

type SinkTestSuite struct {
	suite.Suite
}

func TestSinkTestSuite(t *testing.T) {
	suite.Run(t, new(SinkTestSuite))
}

// envelope is what the bus fills in for one event, published in a zone other than UTC.
func envelope() eventbus.Envelope {
	return eventbus.Envelope{
		ID:       "01934f6e-7a2b-7c3d-8e4f-5a6b7c8d9e0f",
		Name:     audit.EventUserUpdate,
		Category: audit.CategoryAudit,
		Time:     time.Date(2026, 11, 2, 23, 4, 20, 145326000, time.FixedZone("IST", 19800)),
		TraceID:  "56672f4d-0d6e-4a4b-9f0e-6c1d2e3f4a5b",
	}
}

// clientContext returns the context the client middleware records for a request with userAgent.
func clientContext(userAgent string) context.Context {
	r := httptest.NewRequest(http.MethodPut, "/users/"+testUserID, nil)
	r.RemoteAddr = "127.0.0.1:52814"
	r.Header.Set("User-Agent", userAgent)
	var ctx context.Context
	middleware.ClientMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		ctx = r.Context()
	})).ServeHTTP(httptest.NewRecorder(), r)
	return ctx
}

// payload is an audit payload as a user update builds it, with a client.
func payload() audit.Payload {
	return audit.NewPayload(clientContext(testUserAgent), testTarget, audit.ActionUpdate, audit.OutcomeSucceeded)
}

// write handles one event with a sink that writes to a buffer and returns what it wrote.
func (s *SinkTestSuite) write(env eventbus.Envelope, p audit.Payload) []byte {
	var out bytes.Buffer
	s.Require().NoError(newStdoutSink(&out).Handle(context.Background(), env, p))
	return out.Bytes()
}

// decode parses one written line, asserting it is a single newline-terminated JSON object.
func (s *SinkTestSuite) decode(written []byte) knownLine {
	s.Require().True(bytes.HasSuffix(written, []byte("\n")), "a line ends with a newline")
	s.Require().Equal(1, bytes.Count(written, []byte("\n")), "an event is exactly one line")
	s.Require().True(json.Valid(written), "a line is valid JSON: %s", written)
	var decoded knownLine
	s.Require().NoError(json.Unmarshal(written, &decoded))
	return decoded
}

// keys returns the top-level keys of a JSON object in the order they were written.
func (s *SinkTestSuite) keys(written []byte) []string {
	decoder := json.NewDecoder(bytes.NewReader(written))
	_, err := decoder.Token()
	s.Require().NoError(err)
	var keys []string
	for decoder.More() {
		token, err := decoder.Token()
		s.Require().NoError(err)
		keys = append(keys, token.(string))
		var skip json.RawMessage
		s.Require().NoError(decoder.Decode(&skip))
	}
	return keys
}

func (s *SinkTestSuite) TestALineCarriesTheMarkerFirstAndTheWholeEvent() {
	written := s.write(envelope(), payload().WithSubject(testTarget).
		WithDetails(map[string]string{"reason": "requested"}))

	s.True(bytes.HasPrefix(written, []byte(`{"log_type":"AUDIT","schema_version":1,`)))
	s.Equal([]string{
		"log_type", "schema_version", "id", "time", "event", "trace_id", "actor", "subject", "target",
		"action", "outcome", "client", "details",
	}, s.keys(written))

	decoded := s.decode(written)
	env := envelope()
	s.Equal(env.ID, decoded.ID)
	s.Equal("2026-11-02T17:34:20.145326Z", decoded.Time)
	s.Equal(env.Name, decoded.Event)
	s.Equal(env.TraceID, decoded.TraceID)
	s.Equal(map[string]string{"type": "anonymous"}, decoded.Actor)
	s.Equal(map[string]string{"type": "user", "id": testUserID}, decoded.Subject)
	s.Equal(map[string]string{"type": "user", "id": testUserID}, decoded.Target)
	s.Equal("update", decoded.Action)
	s.Equal(map[string]string{"result": "succeeded"}, decoded.Outcome)
	s.Equal(map[string]string{"address": "127.0.0.1", "user_agent": testUserAgent}, decoded.Client)
	s.Equal(map[string]string{"reason": "requested"}, decoded.Details)
}

func (s *SinkTestSuite) TestTheTimeIsUTCWithSixFractionalDigits() {
	env := envelope()
	env.Time = time.Date(2026, 11, 2, 17, 34, 20, 0, time.UTC)

	decoded := s.decode(s.write(env, payload()))

	s.Regexp(lineTimePattern, decoded.Time)
	s.Equal("2026-11-02T17:34:20.000000Z", decoded.Time)
}

// trace_id is always present, so a consumer can tell an event without one from a missing field.
func (s *SinkTestSuite) TestOptionalFieldsAreLeftOut() {
	env := envelope()
	env.TraceID = ""
	p := audit.NewPayload(context.Background(), audit.Ref{Type: audit.TargetTypeUser}, audit.ActionCreate,
		audit.OutcomeFailed("usr_1003"))

	written := s.write(env, p)

	s.Equal([]string{
		"log_type", "schema_version", "id", "time", "event", "trace_id", "actor", "target", "action", "outcome",
	}, s.keys(written))
	decoded := s.decode(written)
	s.Equal(map[string]string{"type": audit.TargetTypeUser}, decoded.Target)
	s.Equal(map[string]string{"result": "failed", "reason": "usr_1003"}, decoded.Outcome)
}

// details is a nested object, so a key in it cannot take the place of the top-level marker.
func (s *SinkTestSuite) TestDetailsCannotReplaceTheMarker() {
	written := s.write(envelope(), payload().WithDetails(map[string]string{"log_type": "DIAGNOSTIC"}))

	s.True(bytes.HasPrefix(written, []byte(`{"log_type":"AUDIT",`)))
	decoded := s.decode(written)
	s.Equal("AUDIT", decoded.LogType)
	s.Equal(map[string]string{"log_type": "DIAGNOSTIC"}, decoded.Details)
}

// A consumer that knows schema version 1 reads the same values from a line that gained a field.
func (s *SinkTestSuite) TestAnAddedFieldLeavesTheKnownFieldsUnchanged() {
	written := s.write(envelope(), payload())
	extended := append([]byte(`{"added_later":{"nested":true},`), written[1:]...)

	s.Equal(s.decode(written), s.decode(extended))
}

func (s *SinkTestSuite) TestAnOversizedLineDropsTheLargestDetailsFirst() {
	p := payload().WithDetails(map[string]string{
		"largest": strings.Repeat("l", maxLineBytes),
		"larger":  strings.Repeat("m", maxLineBytes-maxFieldBytes),
		"fits":    strings.Repeat("f", maxLineBytes/4),
		"small":   "kept",
	})

	written := s.write(envelope(), p)

	s.LessOrEqual(len(written), maxLineBytes)
	decoded := s.decode(written)
	s.Equal(map[string]string{"fits": strings.Repeat("f", maxLineBytes/4), "small": "kept"}, decoded.Details,
		"only as many details as needed are dropped, largest first")
	s.Equal(testUserAgent, decoded.Client["user_agent"], "the user agent is kept while details suffice")
}

func (s *SinkTestSuite) TestThenTheUserAgent() {
	long := strings.Repeat("ü", 256)
	p := audit.NewPayload(clientContext(long), testTarget, audit.ActionUpdate, audit.OutcomeSucceeded).
		WithDetails(map[string]string{"large": strings.Repeat("d", maxLineBytes-300)})
	env := envelope()

	written := s.write(env, p)

	s.LessOrEqual(len(written), maxLineBytes)
	decoded := s.decode(written)
	s.Empty(decoded.Details)
	s.Equal(long, decoded.Client["user_agent"], "the user agent is kept once dropping the details suffices")

	l := newLine(env, p)
	l.Details = nil
	l.TraceID = strings.Repeat("t", maxLineBytes-len(long))
	buf, err := render(l)
	s.Require().NoError(err)
	s.LessOrEqual(len(buf), maxLineBytes)
	cut := s.decode(buf)
	s.NotEmpty(cut.Client["user_agent"], "the user agent is cut, not dropped, when cutting suffices")
	s.True(utf8.ValidString(cut.Client["user_agent"]))
	s.True(strings.HasPrefix(long, cut.Client["user_agent"]))
	s.Equal(l.TraceID, cut.TraceID, "the user agent is cut before any other field")
}

// A caller controls the correlation ID, so it can be large.
func (s *SinkTestSuite) TestThenTheCallerControlledFields() {
	env := envelope()
	env.TraceID = strings.Repeat("t", 3*maxLineBytes)

	written := s.write(env, payload())

	s.LessOrEqual(len(written), maxLineBytes)
	s.True(bytes.HasPrefix(written, []byte(`{"log_type":"AUDIT",`)))
	decoded := s.decode(written)
	s.Equal(strings.Repeat("t", maxFieldBytes), decoded.TraceID)
	s.Equal(env.ID, decoded.ID)
}

// A field that escapes to six bytes per character can keep a line over the limit after every cap.
func (s *SinkTestSuite) TestALineThatCannotFitIsNotWritten() {
	escaped := strings.Repeat("\x01", maxFieldBytes)
	l := newLine(envelope(), payload())
	l.TraceID, l.Actor.Type, l.Actor.ID, l.Target.Type, l.Target.ID, l.Action =
		escaped, escaped, escaped, escaped, escaped, escaped

	buf, err := render(l)

	s.ErrorIs(err, errLineTooLong)
	s.Nil(buf)
}

func (s *SinkTestSuite) TestAWriteFailureIsReturned() {
	closed, err := os.CreateTemp(s.T().TempDir(), "closed")
	s.Require().NoError(err)
	s.Require().NoError(closed.Close())

	s.Error(newStdoutSink(closed).Handle(context.Background(), envelope(), payload()))
}

func (s *SinkTestSuite) TestInitializeWritesToStandardOutput() {
	reader, writer, err := os.Pipe()
	s.Require().NoError(err)
	original := os.Stdout
	os.Stdout = writer
	defer func() { os.Stdout = original }()

	sink := Initialize()
	os.Stdout = original
	s.Require().NoError(sink.Handle(context.Background(), envelope(), payload()))
	s.Require().NoError(writer.Close())

	written, err := io.ReadAll(bufio.NewReader(reader))
	s.Require().NoError(err)
	s.Equal("AUDIT", s.decode(written).LogType)
}
