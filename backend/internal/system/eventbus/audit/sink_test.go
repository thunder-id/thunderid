// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/eventbus"
)

type SinkTestSuite struct {
	suite.Suite
}

func TestSinkTestSuite(t *testing.T) {
	suite.Run(t, new(SinkTestSuite))
}

func (s *SinkTestSuite) TestANilSinkIsRefused() {
	_, registry, stopper := eventbus.Initialize()
	s.T().Cleanup(stopper.Stop)

	s.ErrorIs(Subscribe(registry, "nil-sink", nil), errNilSink)
}

func (s *SinkTestSuite) TestAValidEventReachesTheSink() {
	bus, events := newAuditBus(s.T())
	ctx := context.Background()
	payload := NewPayload(ctx, testTarget, ActionDelete, OutcomeSucceeded).WithSubject(testTarget)

	s.Require().NoError(bus.Publish(ctx, eventbus.NewEvent(ctx, EventUserDelete).With(payload)))

	got := awaitEvent(s.T(), events)
	s.Equal(EventUserDelete, got.env.Name)
	s.Equal(CategoryAudit, got.env.Category)
	s.Equal(payload, got.payload)
}

// An event that fails validation is dropped before the sink, so the next valid event is the first the
// sink sees.
func (s *SinkTestSuite) TestAnInvalidEventNeverReachesTheSink() {
	bus, events := newAuditBus(s.T())
	ctx := context.Background()

	for _, evt := range []*eventbus.Event{
		eventbus.NewEvent(ctx, EventUserUpdate).With(NewPayload(ctx, testTarget, ActionUpdate, OutcomeSucceeded).
			WithDetails(map[string]string{"password": "s3cr3t"})),
		eventbus.NewEvent(ctx, "user.rename").With(NewPayload(ctx, testTarget, ActionUpdate, OutcomeSucceeded)),
		eventbus.NewEvent(ctx, EventUserDelete).With(NewPayload(ctx, testTarget, ActionDelete, OutcomeDenied(""))),
	} {
		s.Require().NoError(bus.Publish(ctx, evt))
	}
	s.Require().NoError(bus.Publish(ctx, eventbus.NewEvent(ctx, EventUserCreate).
		With(NewPayload(ctx, testTarget, ActionCreate, OutcomeSucceeded))))

	s.Equal(EventUserCreate, awaitEvent(s.T(), events).env.Name)
	s.Empty(events)
}
