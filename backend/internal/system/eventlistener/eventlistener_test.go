// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package eventlistener

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ctxKey struct{}

// recorder is a Listener that records what it saw and optionally runs a function per event.
type recorder struct {
	calls int
	ctx   context.Context
	fn    func()
}

func (r *recorder) OnEvent(ctx context.Context, _ string) {
	r.calls++
	r.ctx = ctx
	if r.fn != nil {
		r.fn()
	}
}

type TopicTestSuite struct {
	suite.Suite
}

func TestTopicTestSuite(t *testing.T) {
	suite.Run(t, new(TopicTestSuite))
}

func (s *TopicTestSuite) TestAdd_RejectsNilAndKeepsOrder() {
	topic := NewTopic[string]("test")
	var order []string
	first := &recorder{fn: func() { order = append(order, "first") }}
	second := &recorder{fn: func() { order = append(order, "second") }}

	s.Require().Error(topic.Hook().Add(nil), "a nil listener is rejected")
	s.False(topic.HasListeners())
	s.Require().NoError(topic.Hook().Add(first))
	s.Require().NoError(topic.Hook().Add(second))
	s.True(topic.HasListeners())

	topic.Notify(context.Background(), "e1")

	s.Equal([]string{"first", "second"}, order, "listeners run in registration order")
}

func (s *TopicTestSuite) TestNotify_WithoutListenersIsANoOp() {
	topic := NewTopic[string]("test")
	s.NotPanics(func() { topic.Notify(context.Background(), "e1") })

	var none *Topic[string]
	s.False(none.HasListeners(), "a nil topic has no listeners")
	s.NotPanics(func() { none.Notify(context.Background(), "e1") })
}

func (s *TopicTestSuite) TestNotify_RecoversAPanicAndContinues() {
	topic := NewTopic[string]("test")
	first := &recorder{fn: func() { panic("listener bug") }}
	second := &recorder{}
	s.Require().NoError(topic.Hook().Add(first))
	s.Require().NoError(topic.Hook().Add(second))

	s.NotPanics(func() { topic.Notify(context.Background(), "e1") })

	s.Equal(1, first.calls)
	s.Equal(1, second.calls, "a panic in one listener must not silence the next")
}

func (s *TopicTestSuite) TestNotify_ContextKeepsValuesButNotCancellation() {
	topic := NewTopic[string]("test")
	listener := &recorder{}
	s.Require().NoError(topic.Hook().Add(listener))
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), ctxKey{}, "trace-1"))

	topic.Notify(ctx, "e1")
	cancel()

	s.Require().NotNil(listener.ctx)
	s.Equal("trace-1", listener.ctx.Value(ctxKey{}), "request values travel with the event")
	s.NoError(listener.ctx.Err(), "the listener's context must outlive the request")
}
