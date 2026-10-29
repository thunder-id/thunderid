// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/connection/authzenpdp"
	"github.com/thunder-id/thunderid/internal/connection/httpconnection"
	"github.com/thunder-id/thunderid/internal/idp"
	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/connection/authzenpdpmock"
	"github.com/thunder-id/thunderid/tests/mocks/connection/httpconnectionmock"
	"github.com/thunder-id/thunderid/tests/mocks/idp/idpmock"
	"github.com/thunder-id/thunderid/tests/mocks/notification/notificationmock"
)

type OUDependencyTestSuite struct {
	suite.Suite
	idpService          *idpmock.IDPServiceInterfaceMock
	notificationService *notificationmock.NotificationSenderMgtSvcInterfaceMock
	pdpService          *authzenpdpmock.AuthZENPDPServiceInterfaceMock
	httpService         *httpconnectionmock.HTTPConnectionServiceInterfaceMock
	provider            resourcedependency.Provider
}

func TestOUDependencyTestSuite(t *testing.T) {
	suite.Run(t, new(OUDependencyTestSuite))
}

func (s *OUDependencyTestSuite) SetupTest() {
	s.idpService = idpmock.NewIDPServiceInterfaceMock(s.T())
	s.notificationService = notificationmock.NewNotificationSenderMgtSvcInterfaceMock(s.T())
	s.pdpService = authzenpdpmock.NewAuthZENPDPServiceInterfaceMock(s.T())
	s.httpService = httpconnectionmock.NewHTTPConnectionServiceInterfaceMock(s.T())
	s.provider = newOUDependencyProvider(s.idpService, s.notificationService, s.pdpService, s.httpService)
}

// armIDPs makes the identity provider service list the given providers, or fail with the given error.
func (s *OUDependencyTestSuite) armIDPs(list []idp.BasicIDPDTO, svcErr *tidcommon.ServiceError) {
	s.idpService.On("GetIdentityProviderList", mock.Anything).Return(list, svcErr).Once()
}

// armSenders makes the notification service list the given senders, or fail with the given error.
func (s *OUDependencyTestSuite) armSenders(list []ncommon.NotificationSenderDTO, svcErr *tidcommon.ServiceError) {
	s.notificationService.On("ListSenders", mock.Anything).Return(list, svcErr).Once()
}

// armPDPs makes the AuthZEN PDP service list the given connections, or fail with the given error.
func (s *OUDependencyTestSuite) armPDPs(list []authzenpdp.AuthZENPDPConnection, svcErr *tidcommon.ServiceError) {
	s.pdpService.On("ListAuthZENPDPs", mock.Anything).Return(list, svcErr).Once()
}

// armHTTPS makes the HTTP connection service list the given connections, or fail with the given error.
func (s *OUDependencyTestSuite) armHTTPS(list []httpconnection.HTTPConnection, svcErr *tidcommon.ServiceError) {
	s.httpService.On("ListHTTPConnections", mock.Anything).Return(list, svcErr).Once()
}

func (s *OUDependencyTestSuite) TestReportsOwnedConnections() {
	s.armIDPs([]idp.BasicIDPDTO{
		{ID: "idp-1", Name: "Google", Type: providers.IDPTypeGoogle, OUID: "ou-1"},
		{ID: "idp-2", Name: "Other", Type: providers.IDPTypeGoogle, OUID: "ou-2"},
	}, nil)
	s.armSenders([]ncommon.NotificationSenderDTO{
		{ID: "sender-1", Name: "SMS", OUID: "ou-1"}, {ID: "sender-2", Name: "Mail", OUID: "ou-2"},
	}, nil)
	s.armPDPs([]authzenpdp.AuthZENPDPConnection{
		{ID: "pdp-1", Name: "PDP", OUID: "ou-1"}, {ID: "pdp-2", Name: "Other PDP", OUID: "ou-2"},
	}, nil)
	s.armHTTPS([]httpconnection.HTTPConnection{{ID: "http-1", Name: "Risk", OUID: "ou-1"}}, nil)

	deps, err := s.provider.GetResourceDependencies(context.Background(), resourcedependency.ResourceTypeOU, "ou-1")

	s.Require().NoError(err)
	got := map[string]string{}
	for _, dep := range deps {
		s.Equal(resourcedependency.BehaviorRestrict, dep.BehaviorOnDelete)
		got[dep.ID] = dep.ResourceType
	}
	s.Equal(map[string]string{
		"idp-1":    resourcedependency.ResourceTypeIDP,
		"sender-1": resourcedependency.ResourceTypeNotificationSender,
		"pdp-1":    resourcedependency.ResourceTypeAuthZENPDPConnection,
		"http-1":   resourcedependency.ResourceTypeHTTPConnection,
	}, got)
}

func (s *OUDependencyTestSuite) TestReportsNothingForUnownedOU() {
	s.armIDPs([]idp.BasicIDPDTO{{ID: "idp-1", Name: "Google", OUID: "ou-1"}}, nil)
	s.armSenders([]ncommon.NotificationSenderDTO{{ID: "sender-1", Name: "SMS", OUID: "ou-1"}}, nil)
	s.armPDPs([]authzenpdp.AuthZENPDPConnection{{ID: "pdp-1", Name: "PDP", OUID: "ou-1"}}, nil)
	s.armHTTPS([]httpconnection.HTTPConnection{{ID: "http-1", Name: "Risk", OUID: "ou-1"}}, nil)

	deps, err := s.provider.GetResourceDependencies(context.Background(), resourcedependency.ResourceTypeOU, "ou-3")

	s.Require().NoError(err)
	s.Empty(deps)
}

func (s *OUDependencyTestSuite) TestIgnoresOtherResources() {
	deps, err := s.provider.GetResourceDependencies(
		context.Background(), resourcedependency.ResourceTypeIDP, "idp-1")

	s.Require().NoError(err)
	s.Empty(deps)
}

func (s *OUDependencyTestSuite) TestFailsClosed() {
	tests := []struct {
		name string
		arm  func()
	}{
		{
			name: "identity providers cannot be listed",
			arm:  func() { s.armIDPs(nil, &tidcommon.InternalServerError) },
		},
		{
			name: "notification senders cannot be listed",
			arm: func() {
				s.armIDPs(nil, nil)
				s.armSenders(nil, &tidcommon.InternalServerError)
			},
		},
		{
			name: "AuthZEN PDP connections cannot be listed",
			arm: func() {
				s.armIDPs(nil, nil)
				s.armSenders(nil, nil)
				s.armPDPs(nil, &tidcommon.InternalServerError)
			},
		},
		{
			name: "HTTP connections cannot be listed",
			arm: func() {
				s.armIDPs(nil, nil)
				s.armSenders(nil, nil)
				s.armPDPs(nil, nil)
				s.armHTTPS(nil, &tidcommon.InternalServerError)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			tc.arm()

			_, err := s.provider.GetResourceDependencies(
				context.Background(), resourcedependency.ResourceTypeOU, "ou-1")

			s.Require().Error(err)
		})
	}
}
