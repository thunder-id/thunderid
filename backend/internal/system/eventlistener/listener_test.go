// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package eventlistener

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type ctxKey struct{}

type EventListenerTestSuite struct {
	suite.Suite
}

func TestEventListenerTestSuite(t *testing.T) {
	suite.Run(t, new(EventListenerTestSuite))
}

func (s *EventListenerTestSuite) TestNotify_WithoutListenersIsANoOp() {
	topic := NewTopic[string]("test")
	s.NotPanics(func() { topic.Notify(context.Background(), "e1") })

	var none *Topic[string]
	s.False(none.HasListeners(), "a nil topic has no listeners")
	s.NotPanics(func() { none.Notify(context.Background(), "e1") })
}

func (s *EventListenerTestSuite) TestNotify_RecoversAPanicAndContinues() {
	topic := NewTopic[string]("test")
	first := NewListenerMock[string](s.T())
	second := NewListenerMock[string](s.T())
	first.EXPECT().OnEvent(mock.Anything, "e1").
		Run(func(context.Context, string) { panic("listener bug") }).Once()
	second.EXPECT().OnEvent(mock.Anything, "e1").Once()
	s.Require().NoError(topic.Hook().Add(first))
	s.Require().NoError(topic.Hook().Add(second))

	s.NotPanics(func() { topic.Notify(context.Background(), "e1") })

	second.AssertExpectations(s.T())
}

func (s *EventListenerTestSuite) TestNotify_ContextKeepsValuesButNotCancellation() {
	topic := NewTopic[string]("test")
	listener := NewListenerMock[string](s.T())
	var got context.Context
	listener.EXPECT().OnEvent(mock.Anything, "e1").Run(func(ctx context.Context, _ string) { got = ctx }).Once()
	s.Require().NoError(topic.Hook().Add(listener))
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), ctxKey{}, "trace-1"))

	topic.Notify(ctx, "e1")
	cancel()

	s.Require().NotNil(got)
	s.Equal("trace-1", got.Value(ctxKey{}), "request values travel with the event")
	s.NoError(got.Err(), "the listener's context must outlive the request")
}
