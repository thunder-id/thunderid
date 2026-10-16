// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernance

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/mocks/notification/notificationmock"
)

type InitTestSuite struct {
	suite.Suite
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (s *InitTestSuite) TestInitializeRequiresAnEntityStateProvider() {
	svc, stop, err := Initialize(nil, Config{}, nil, nil, nil, nil)
	s.Require().ErrorIs(err, errNoEntityStateProvider)
	s.Nil(svc)
	s.Nil(stop)
}

// Without notification dependencies the service is built and the stop function does nothing.
func (s *InitTestSuite) TestInitializeWithoutNotificationsBuildsTheService() {
	svc, stop, err := Initialize(NewEntityStateProviderMock(s.T()), Config{}, nil, nil, nil, nil)
	s.Require().NoError(err)
	s.NotNil(svc)
	s.Require().NotNil(stop)
	s.NotPanics(stop)
}

// With notification dependencies the stop function closes the notifier, so a later notice is dropped.
func (s *InitTestSuite) TestStopClosesTheNotifier() {
	svc, stop, err := Initialize(NewEntityStateProviderMock(s.T()), Config{}, nil,
		&NotificationDependencies{
			Templates:     newNotificationTemplateRendererMock(s.T()),
			Senders:       notificationmock.NewNotificationSenderServiceInterfaceMock(s.T()),
			DefaultSender: notificationmock.NewDefaultEmailSenderInterfaceMock(s.T()),
		}, nil, nil)
	s.Require().NoError(err)

	stop()

	notifier, ok := svc.(*service).notifier.(*lockEmailNotifier)
	s.Require().True(ok)
	s.True(notifier.closed)
	s.NotPanics(stop, "stopping twice is safe")
}
