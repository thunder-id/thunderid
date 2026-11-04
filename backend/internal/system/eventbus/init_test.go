// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package eventbus

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type InitTestSuite struct {
	busSuite
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (s *InitTestSuite) TestBusDeliversToASubscriber() {
	bus, registry := s.newBus()
	rec := subscribe[payloadOne](&s.busSuite, registry, "one", 1, nil)

	s.Require().NoError(s.publish(bus, testEventName, payloadOne{}))
	s.shutdown(bus)

	s.Equal(1, rec.count())
}

func (s *InitTestSuite) TestBusCannotBeAssertedToTheRegistryOrTheStopper() {
	bus, _ := s.newBus()

	_, isRegistry := bus.(RegistryInterface)
	_, isStopper := bus.(StopperInterface)

	s.False(isRegistry)
	s.False(isStopper)
}
