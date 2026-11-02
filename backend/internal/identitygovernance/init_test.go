// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernance

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/serverconfig"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/notification/notificationmock"
	"github.com/thunder-id/thunderid/tests/mocks/serverconfigmock"
)

type InitTestSuite struct {
	suite.Suite
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (s *InitTestSuite) TestInitializeRequiresAnEntityStateProvider() {
	svc, notifier, err := Initialize(nil, serverconfigmock.NewServerConfigServiceMock(s.T()), Config{}, nil, nil, nil)
	s.Require().ErrorIs(err, errNoEntityStateProvider)
	s.Nil(svc)
	s.Nil(notifier)
}

func (s *InitTestSuite) TestInitializeRequiresAServerConfigService() {
	svc, notifier, err := Initialize(newEntityStateProviderMock(s.T()), nil, Config{}, nil, nil, nil)
	s.Require().ErrorIs(err, errNoServerConfigService)
	s.Nil(svc)
	s.Nil(notifier)
}

// Without notification dependencies the service is built and there is no notifier to stop.
func (s *InitTestSuite) TestInitializeWithoutNotificationsBuildsTheService() {
	svc, notifier, err := Initialize(newEntityStateProviderMock(s.T()),
		serverconfigmock.NewServerConfigServiceMock(s.T()), Config{}, nil, nil, nil)
	s.Require().NoError(err)
	s.NotNil(svc)
	s.Nil(notifier)
}

// With notification dependencies the returned notifier stops the workers, so a later notice is dropped.
func (s *InitTestSuite) TestTheReturnedNotifierStops() {
	svc, notifier, err := Initialize(newEntityStateProviderMock(s.T()),
		serverconfigmock.NewServerConfigServiceMock(s.T()), Config{}, s.notificationDependencies(), nil, nil)
	s.Require().NoError(err)
	s.Require().NotNil(notifier)

	notifier.Stop()

	emailNotifier, ok := svc.(*identityGovernanceService).notifier.(*lockEmailNotifier)
	s.Require().True(ok)
	s.Same(emailNotifier, notifier)
	s.True(emailNotifier.closed)
	s.NotPanics(notifier.Stop, "stopping twice is safe")
}

// The lock email needs every notification dependency, so with any one missing no notifier is built.
func (s *InitTestSuite) TestInitializeBuildsNoNotifierWithoutEveryDependency() {
	cases := []struct {
		name   string
		remove func(*NotificationDependencies)
	}{
		{name: "templates", remove: func(d *NotificationDependencies) { d.Templates = nil }},
		{name: "senders", remove: func(d *NotificationDependencies) { d.Senders = nil }},
		{name: "entity types", remove: func(d *NotificationDependencies) { d.EntityTypes = nil }},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			deps := s.notificationDependencies()
			tc.remove(deps)

			svc, notifier, err := Initialize(newEntityStateProviderMock(s.T()),
				serverconfigmock.NewServerConfigServiceMock(s.T()), Config{}, deps, nil, nil)

			s.Require().NoError(err)
			s.Nil(svc.(*identityGovernanceService).notifier)
			s.Nil(notifier)
		})
	}
}

// The accountAccess section and the lock email's default sender are read through the given service.
func (s *InitTestSuite) TestTheSectionAndTheSenderAreReadThroughTheServerConfigService() {
	section := accountAccessValue{User: &accountAccessCategory{Default: &accountAccessPolicy{Threshold: new(3)}}}
	reader := serverconfigmock.NewServerConfigServiceMock(s.T())
	reader.EXPECT().GetMergedConfig(mock.Anything, string(serverconfig.ConfigNameAccountAccess)).
		Return(section, nil).Once()
	svc, notifier, err := Initialize(newEntityStateProviderMock(s.T()), reader,
		Config{AccountAccess: testAccountAccessConfig()}, s.notificationDependencies(), nil, nil)
	s.Require().NoError(err)
	defer notifier.Stop()

	governance := svc.(*identityGovernanceService)
	s.Equal(section, governance.source.Current(context.Background()))
	emailNotifier, ok := governance.notifier.(*lockEmailNotifier)
	s.Require().True(ok)
	s.Same(reader, emailNotifier.serverConfig)
}

// notificationDependencies returns every dependency the lock email needs, as mocks.
func (s *InitTestSuite) notificationDependencies() *NotificationDependencies {
	return &NotificationDependencies{
		Templates:   newNotificationTemplateRendererMock(s.T()),
		Senders:     notificationmock.NewNotificationSenderServiceInterfaceMock(s.T()),
		EntityTypes: entitytypemock.NewEntityTypeServiceInterfaceMock(s.T()),
	}
}
