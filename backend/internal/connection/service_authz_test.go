// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/connection/authzenpdp"
	"github.com/thunder-id/thunderid/internal/connection/httpconnection"
	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/connection/authzenpdpmock"
	"github.com/thunder-id/thunderid/tests/mocks/connection/httpconnectionmock"
	"github.com/thunder-id/thunderid/tests/mocks/idp/idpmock"
	"github.com/thunder-id/thunderid/tests/mocks/notification/notificationmock"
	"github.com/thunder-id/thunderid/tests/mocks/sharingmock"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

const (
	authzOwnOU   = "ou-own"
	authzOtherOU = "ou-other"
)

// expectOwnOUOnly expects each action to be checked, and allows it on the caller's own organization unit
// or in a runtime context. A list action lists the caller's own organization unit alone. Any other
// action is unexpected.
func expectOwnOUOnly(m *sysauthzmock.SystemAuthorizationServiceInterfaceMock, actions ...security.Action) {
	for _, action := range actions {
		if strings.HasSuffix(string(action), ":list") {
			m.On("GetAccessibleResources", mock.Anything, action, security.ResourceTypeOU).
				Return(&sysauthz.AccessibleResources{IDs: []string{authzOwnOU}}, nil)
			continue
		}
		m.On("IsActionAllowed", mock.Anything, action, mock.Anything).Return(
			func(ctx context.Context, _ security.Action, c *sysauthz.ActionContext) (bool, *tidcommon.ServiceError) {
				return security.IsRuntimeContext(ctx) || c.OUID == authzOwnOU, nil
			})
	}
}

// ConnectionAuthzTestSuite covers the organization unit checks on the connection management operations.
type ConnectionAuthzTestSuite struct {
	suite.Suite
	authz *sysauthzmock.SystemAuthorizationServiceInterfaceMock
	idps  *idpmock.IDPServiceInterfaceMock
	http  *httpconnectionmock.HTTPConnectionServiceInterfaceMock
	svc   *service
}

func TestConnectionAuthzTestSuite(t *testing.T) {
	suite.Run(t, new(ConnectionAuthzTestSuite))
}

func (s *ConnectionAuthzTestSuite) SetupTest() {
	s.authz = sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	s.idps = idpmock.NewIDPServiceInterfaceMock(s.T())
	s.http = httpconnectionmock.NewHTTPConnectionServiceInterfaceMock(s.T())
	s.svc = newService(s.idps, notificationmock.NewNotificationSenderMgtSvcInterfaceMock(s.T()),
		&testResourceServerLister{}, newAuthZENPDPServiceMock(s.T()), s.http)
	s.svc.authzService = s.authz
	s.svc.sharingService = newSharingServiceMock(s.T())
}

func (s *ConnectionAuthzTestSuite) storedHTTP(id, ouID string) {
	s.http.On("GetHTTPConnection", mock.Anything, id).
		Return(&httpconnection.HTTPConnection{ID: id, OUID: ouID}, (*tidcommon.ServiceError)(nil)).Maybe()
}

func (s *ConnectionAuthzTestSuite) TestGetHTTP_OtherOU() {
	expectOwnOUOnly(s.authz, security.ActionReadConnection)
	s.storedHTTP("http-other", authzOtherOU)

	connection, svcErr := s.svc.getHTTP(context.Background(), "http-other")
	s.Nil(connection)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
}

func (s *ConnectionAuthzTestSuite) TestDeleteHTTP_OtherOU() {
	expectOwnOUOnly(s.authz, security.ActionDeleteConnection)
	s.storedHTTP("http-other", authzOtherOU)

	svcErr := s.svc.deleteHTTP(context.Background(), "http-other")
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	s.http.AssertNotCalled(s.T(), "DeleteHTTPConnection", mock.Anything, mock.Anything)
}

func (s *ConnectionAuthzTestSuite) TestCreateHTTP_OtherOU() {
	expectOwnOUOnly(s.authz, security.ActionCreateConnection)
	_, svcErr := s.svc.createHTTP(context.Background(), httpconnection.HTTPConnection{OUID: authzOtherOU})
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	s.http.AssertNotCalled(s.T(), "CreateHTTPConnection", mock.Anything, mock.Anything)
}

func (s *ConnectionAuthzTestSuite) TestListHTTP_OwnOUOnly() {
	expectOwnOUOnly(s.authz, security.ActionListConnections)
	s.http.On("ListHTTPConnections", mock.Anything).Return([]httpconnection.HTTPConnection{
		{ID: "http-own", OUID: authzOwnOU}, {ID: "http-other", OUID: authzOtherOU},
	}, (*tidcommon.ServiceError)(nil))

	connections, svcErr := s.svc.listHTTP(context.Background())
	s.Require().Nil(svcErr)
	s.Require().Len(connections, 1)
	s.Equal("http-own", connections[0].ID)
}

func (s *ConnectionAuthzTestSuite) TestListInstances_KeepsHTTPAndPDPSharedIntoOwnOU() {
	expectOwnOUOnly(s.authz, security.ActionListConnections)
	pdp := authzenpdpmock.NewAuthZENPDPServiceInterfaceMock(s.T())
	pdp.On("ListAuthZENPDPs", mock.Anything).Return([]authzenpdp.AuthZENPDPConnection{
		{ID: "pdp-own", OUID: authzOwnOU}, {ID: "pdp-shared", OUID: authzOtherOU},
	}, (*tidcommon.ServiceError)(nil))
	s.svc.authZENPDPService = pdp
	s.http.On("ListHTTPConnections", mock.Anything).Return([]httpconnection.HTTPConnection{
		{ID: "http-own", OUID: authzOwnOU}, {ID: "http-shared", OUID: authzOtherOU},
		{ID: "http-other", OUID: authzOtherOU},
	}, (*tidcommon.ServiceError)(nil))
	sharingSvc := sharingmock.NewSharingServiceInterfaceMock(s.T())
	sharingSvc.On("ListVisibleResourceIDs", mock.Anything, ConnectionSharingType, authzOwnOU).
		Return([]string{"pdp-shared", "http-shared"}, (*tidcommon.ServiceError)(nil))
	s.svc.sharingService = sharingSvc

	for _, category := range []connectionCategory{categoryAuthorizationPDP, categoryHTTP} {
		list, svcErr := s.svc.listInstances(context.Background(), category, "", 10, 0)
		s.Require().Nil(svcErr)
		ids := make([]string, 0, len(list.Connections))
		for _, c := range list.Connections {
			ids = append(ids, c.ID)
		}
		if category == categoryHTTP {
			s.Equal([]string{"http-own", "http-shared"}, ids)
		} else {
			s.Equal([]string{"pdp-own", "pdp-shared"}, ids)
		}
	}
}

func (s *ConnectionAuthzTestSuite) TestUpdateIdentityProvider_OtherOU() {
	expectOwnOUOnly(s.authz, security.ActionUpdateConnection)
	s.idps.On("GetIdentityProvider", mock.Anything, "idp-other").Return(&providers.IDPDTO{
		ID: "idp-other", Type: providers.IDPTypeGoogle, OUID: authzOtherOU,
	}, (*tidcommon.ServiceError)(nil))

	_, svcErr := s.svc.update(context.Background(), providers.IDPTypeGoogle, "idp-other", &providers.IDPDTO{})
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	s.idps.AssertNotCalled(s.T(), "UpdateIdentityProvider", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ConnectionAuthzTestSuite) TestCreateSharingPolicy_InitiatedByOtherOU() {
	expectOwnOUOnly(s.authz, security.ActionUpdateConnection)
	s.storedHTTP("http-own", authzOwnOU)

	_, svcErr := s.svc.createSharingPolicy(context.Background(), s.svc.httpOwner, "http-own",
		sharing.PolicyRequest{InitiatingOUID: authzOtherOU})
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
}
