// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package eventbus

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	sysContext "github.com/thunder-id/thunderid/internal/system/context"
)

type EventTestSuite struct {
	suite.Suite
}

func TestEventTestSuite(t *testing.T) {
	suite.Run(t, new(EventTestSuite))
}

func (s *EventTestSuite) TestNameGrammar() {
	longest := strings.Repeat("a", 31) + "." + strings.Repeat("b", 32)
	valid := []string{"user.created", "token.issued", "user.credential.changed", "a.b.c.d", "x1.y_2", longest}
	invalid := []string{"", "USER_CREATED", "user", "User.created", "user..created", ".user.created",
		"user.created.", "1user.created", "a.b.c.d.e", "user.created now", "user-created.x", longest + "b",
		"user.created\n"}

	s.Len(longest, maxEventNameLength)
	for _, name := range valid {
		s.NoError(NewEvent(context.Background(), name).With(payloadOne{}).validate(), name)
	}
	for _, name := range invalid {
		s.ErrorIs(NewEvent(context.Background(), name).With(payloadOne{}).validate(), errInvalidEventName, name)
	}
}

func (s *EventTestSuite) TestEventWithoutPayloadIsRejected() {
	s.ErrorIs(NewEvent(context.Background(), testEventName).validate(), errNoPayload)
}

func (s *EventTestSuite) TestNilPayloadAndDuplicateCategoryAreRejected() {
	s.ErrorIs(NewEvent(context.Background(), testEventName).With(nil).validate(), errNilPayload)
	s.ErrorIs(NewEvent(context.Background(), testEventName).With(payloadOne{}).With(payloadOne{}).validate(),
		errDuplicateCategory)
	s.ErrorIs(NewEvent(context.Background(), testEventName).With(payloadOne{}).With(otherPayloadOne{}).validate(),
		errDuplicateCategory)
}

func (s *EventTestSuite) TestFirstErrorIsKept() {
	evt := NewEvent(context.Background(), "BAD").With(nil).With(payloadOne{})

	s.ErrorIs(evt.validate(), errInvalidEventName)
}

func (s *EventTestSuite) TestPayloadsPutEventInTheirCategories() {
	evt := NewEvent(context.Background(), testEventName).With(payloadOne{value: "a"}).With(payloadTwo{value: "b"})

	s.Require().NoError(evt.validate())
	s.Len(evt.payloads, 2)
	s.Contains(evt.payloads, categoryOne)
	s.Contains(evt.payloads, categoryTwo)
}

func (s *EventTestSuite) TestTraceIDIsTakenFromContext() {
	ctx := sysContext.WithTraceID(context.Background(), "trace-1")

	evt := NewEvent(ctx, testEventName).With(payloadOne{})

	s.Equal(testEventName, evt.envelope.Name)
	s.Equal("trace-1", evt.envelope.TraceID)
}

func (s *EventTestSuite) TestTraceIDIsCapped() {
	atLimit := strings.Repeat("a", maxTraceIDLength)
	ctx := sysContext.WithTraceID(context.Background(), atLimit+strings.Repeat("b", 1<<20))

	s.Equal(atLimit, NewEvent(ctx, testEventName).envelope.TraceID)
	s.Equal(atLimit, NewEvent(sysContext.WithTraceID(context.Background(), atLimit), testEventName).envelope.TraceID)
}

func (s *EventTestSuite) TestTraceIDIsEmptyWhenTheContextHasNone() {
	evt := NewEvent(context.Background(), testEventName).With(payloadOne{})

	s.Empty(evt.envelope.TraceID)
}

func (s *EventTestSuite) TestPointerPayloadIsRejected() {
	var typedNil *pointerPayload

	s.ErrorIs(NewEvent(context.Background(), testEventName).With(&pointerPayload{category: categoryOne}).validate(),
		errPointerPayload)
	s.ErrorIs(NewEvent(context.Background(), testEventName).With(typedNil).validate(), errPointerPayload)
}

func (s *EventTestSuite) TestPanickingPayloadIsRejectedWithoutPanicking() {
	s.NotPanics(func() {
		s.ErrorIs(NewEvent(context.Background(), testEventName).With(panickingPayload{}).validate(),
			errPayloadPanicked)
	})
}

func (s *EventTestSuite) TestNameOfANilEventIsEmpty() {
	var evt *Event

	s.Empty(evt.name())
	s.ErrorIs(evt.validate(), errNilEvent)
}
