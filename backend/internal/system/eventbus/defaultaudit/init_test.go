// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package defaultaudit

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
)

const testUserID = "01900000-0000-7000-8000-000000000030"

type InitTestSuite struct {
	suite.Suite
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

// auditConfig is the audit section with audit on or off.
func auditConfig(enabled bool) config.AuditConfig {
	return config.AuditConfig{Enabled: &enabled}
}

// userDelete is the event of a user deletion published from outside a request.
func userDelete(outcome audit.Outcome) *eventbus.Event {
	ctx := context.Background()
	return eventbus.NewEvent(ctx, audit.EventUserDelete).
		With(audit.NewPayload(ctx, audit.Ref{Type: audit.TargetTypeUser, ID: testUserID}, audit.ActionDelete,
			outcome))
}

// captureStdout sends standard output to a pipe until the returned function is called, which returns
// what was written.
func (s *InitTestSuite) captureStdout() func() []byte {
	reader, writer, err := os.Pipe()
	s.Require().NoError(err)
	original := os.Stdout
	os.Stdout = writer
	s.T().Cleanup(func() { os.Stdout = original })
	return func() []byte {
		os.Stdout = original
		s.Require().NoError(writer.Close())
		written, err := io.ReadAll(reader)
		s.Require().NoError(err)
		return written
	}
}

// Stop drains the queue, so every event published before it is written.
func (s *InitTestSuite) TestAuditOnWritesOneLinePerValidEventToStandardOutput() {
	read := s.captureStdout()
	bus, stopper, err := Initialize(auditConfig(true))
	s.Require().NoError(err)

	s.Require().NoError(bus.Publish(context.Background(), userDelete(audit.OutcomeSucceeded)))
	s.Require().NoError(bus.Publish(context.Background(), userDelete(audit.OutcomeDenied(""))))
	stopper.Stop()

	written := read()
	s.Equal(1, bytes.Count(written, []byte("\n")), "the event without a reason is dropped: %s", written)
	s.True(bytes.HasPrefix(written, []byte(`{"log_type":"AUDIT","schema_version":1,`)))
	s.Contains(string(written), `"event":"user.delete"`)
}

// With audit off the bus still validates every event, then drops it.
func (s *InitTestSuite) TestAuditOffValidatesAndDrops() {
	for name, cfg := range map[string]config.AuditConfig{
		"audit off":   auditConfig(false),
		"audit unset": {},
	} {
		s.Run(name, func() {
			read := s.captureStdout()
			bus, stopper, err := Initialize(cfg)
			s.Require().NoError(err)

			s.NoError(bus.Publish(context.Background(), userDelete(audit.OutcomeSucceeded)))
			s.Error(bus.Publish(context.Background(), eventbus.NewEvent(context.Background(), "User Deleted").
				With(audit.NewPayload(context.Background(), audit.Ref{Type: audit.TargetTypeUser},
					audit.ActionDelete, audit.OutcomeSucceeded))))
			stopper.Stop()

			s.Empty(read())
		})
	}
}
