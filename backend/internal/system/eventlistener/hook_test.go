// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package eventlistener

import (
	"context"

	"github.com/stretchr/testify/mock"
)

func (s *EventListenerTestSuite) TestAdd_RejectsNilAndKeepsOrder() {
	topic := NewTopic[string]("test")
	var order []string
	first := NewListenerMock[string](s.T())
	second := NewListenerMock[string](s.T())
	first.EXPECT().OnEvent(mock.Anything, "e1").
		Run(func(context.Context, string) { order = append(order, "first") }).Once()
	second.EXPECT().OnEvent(mock.Anything, "e1").
		Run(func(context.Context, string) { order = append(order, "second") }).Once()

	s.Require().Error(topic.Hook().Add(nil), "a nil listener is rejected")
	s.False(topic.HasListeners())
	s.Require().NoError(topic.Hook().Add(first))
	s.Require().NoError(topic.Hook().Add(second))
	s.True(topic.HasListeners())

	topic.Notify(context.Background(), "e1")

	s.Equal([]string{"first", "second"}, order, "listeners run in registration order")
}
