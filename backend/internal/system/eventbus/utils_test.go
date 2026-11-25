// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package eventbus

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/testhelpers"
)

const publishFailure = "Failed to publish an event"

type UtilsTestSuite struct {
	busSuite
}

func TestUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(UtilsTestSuite))
}

func (s *UtilsTestSuite) TestTryPublishDeliversTheEvent() {
	bus, registry := s.newBus()
	defer s.shutdown(bus)
	rec := subscribe[payloadOne](&s.busSuite, registry, "one", 1, nil)

	evt := NewEvent(context.Background(), testEventName).With(payloadOne{value: "a"})

	output := testhelpers.CaptureLogs(s.T(), "INFO", func() {
		TryPublish(context.Background(), bus, evt)
		s.eventually(func() bool { return rec.count() == 1 })
	})

	s.NotContains(output, publishFailure)
}

func (s *UtilsTestSuite) TestTryPublishLogsAndReturnsWhenPublishFails() {
	bus, registry := s.newBus()
	rec := subscribe[payloadOne](&s.busSuite, registry, "one", 0, nil)

	output := testhelpers.CaptureLogs(s.T(), "INFO", func() {
		TryPublish(context.Background(), bus, NewEvent(context.Background(), testEventName))
		s.NotPanics(func() { TryPublish(context.Background(), bus, nil) })
		s.shutdown(bus)
	})

	s.Zero(rec.count())
	s.Contains(output, publishFailure)
	s.Contains(output, testEventName)
	s.Contains(output, errNoPayload.Error())
	s.Contains(output, errNilEvent.Error())
	s.Contains(output, "EventBus")
}

func (s *UtilsTestSuite) TestTryPublishNeverLogsAnInvalidName() {
	bus, _ := s.newBus()

	evt := NewEvent(context.Background(), "user.create\r\nforged=1").With(payloadOne{})

	output := testhelpers.CaptureLogs(s.T(), "INFO", func() {
		TryPublish(context.Background(), bus, evt)
	})

	s.Contains(output, publishFailure)
	s.Contains(output, "event="+invalidEventName)
	s.NotContains(output, "forged")
}

func (s *UtilsTestSuite) TestSubscriberIsRegisteredForTheCategoryOfItsPayload() {
	bus, registry := s.newBus()
	rec := subscribe[payloadTwo](&s.busSuite, registry, "two", 1, nil)

	s.Require().NoError(s.publish(bus, testEventName, payloadOne{}, payloadTwo{}))
	s.shutdown(bus)

	envelopes, _ := rec.received()
	s.Require().Len(envelopes, 1)
	s.Equal(categoryTwo, envelopes[0].Category)
}

func (s *UtilsTestSuite) TestNilSubscriberAndInvalidIDsAreRejected() {
	bus, registry := s.newBus()
	defer s.shutdown(bus)
	subscribe[payloadOne](&s.busSuite, registry, "same", 0, nil)

	s.ErrorIs(Subscribe[payloadOne](registry, "nil", nil), errNilDeliveryFunc)
	s.ErrorIs(Subscribe[payloadOne](registry, "same", NewSubscriberMock[payloadOne](s.T())), errDuplicateSubscriberID)
	s.ErrorIs(Subscribe[payloadOne](registry, "", NewSubscriberMock[payloadOne](s.T())), errEmptySubscriberID)
}

func (s *UtilsTestSuite) TestPayloadTypeThatCannotReportItsCategoryIsRejected() {
	bus, registry := s.newBus()
	defer s.shutdown(bus)

	s.ErrorIs(Subscribe[Payload](registry, "interface", NewSubscriberMock[Payload](s.T())), errUnsupportedPayloadType)
	s.ErrorIs(Subscribe[*pointerPayload](registry, "pointer", NewSubscriberMock[*pointerPayload](s.T())),
		errUnsupportedPayloadType)
	s.ErrorIs(Subscribe[panickingPayload](registry, "panicking", NewSubscriberMock[panickingPayload](s.T())),
		errUnsupportedPayloadType)
	s.Zero(s.subscriberCount(bus))
}

func (s *UtilsTestSuite) TestPayloadOfAnotherTypeInTheSameCategoryIsAFailedDelivery() {
	bus, registry := s.newBus()
	rec := subscribe[payloadOne](&s.busSuite, registry, "one", 1, nil)

	output := testhelpers.CaptureLogs(s.T(), "INFO", func() {
		s.Require().NoError(s.publish(bus, testEventName, otherPayloadOne{}))
		s.Require().NoError(s.publish(bus, testEventName, payloadOne{value: "a"}))
		s.shutdown(bus)
	})

	_, payloads := rec.received()
	s.Equal([]payloadOne{{value: "a"}}, payloads)
	s.Contains(output, errPayloadType.Error())
}
