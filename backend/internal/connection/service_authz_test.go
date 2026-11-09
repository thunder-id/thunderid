// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/connection/authzenpdp"
	"github.com/thunder-id/thunderid/internal/connection/httpconnection"
	"github.com/thunder-id/thunderid/internal/idp"
	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
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
// action is unexpected. The boundary is deployment wide, so a refusal is not answered from sharing.
func expectOwnOUOnly(m *sysauthzmock.SystemAuthorizationServiceInterfaceMock, actions ...security.Action) {
	m.On("GetBoundary", mock.Anything).Return(sysauthz.Boundary{Type: sysauthz.BoundaryTypeDeployment}).Maybe()
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

	connection, _, svcErr := s.svc.getHTTP(context.Background(), "http-other")
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
	sharingSvc := sharingmock.NewSharingServiceInterfaceMock(s.T())
	sharingSvc.On("ListVisibleResourceIDs", mock.Anything, ConnectionSharingType, authzOwnOU).
		Return([]string{}, (*tidcommon.ServiceError)(nil))
	s.svc.sharingService = sharingSvc

	connections, svcErr := s.svc.listHTTP(context.Background())
	s.Require().Nil(svcErr)
	s.Require().Len(connections, 1)
	s.Equal("http-own", connections[0].ID)
}

func (s *ConnectionAuthzTestSuite) TestListInstances_HTTPAndPDPSharedIntoOwnOU() {
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

	listed := func(category connectionCategory, ouID string) []string {
		list, svcErr := s.svc.listInstances(context.Background(), category, ouID, 10, 0)
		s.Require().Nil(svcErr)
		ids := make([]string, 0, len(list.Connections))
		for _, c := range list.Connections {
			ids = append(ids, c.ID)
		}
		return ids
	}

	// The organization unit filter lists the connections shared into the unit after its own.
	s.Equal([]string{"http-own", "http-shared"}, listed(categoryHTTP, authzOwnOU))
	s.Equal([]string{"pdp-own", "pdp-shared"}, listed(categoryAuthorizationPDP, authzOwnOU))
	// Without it, the caller lists its unit the same way.
	s.Equal([]string{"http-own", "http-shared"}, listed(categoryHTTP, ""))
	s.Equal([]string{"pdp-own", "pdp-shared"}, listed(categoryAuthorizationPDP, ""))
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

// The kinds of connection the shared fixture holds, each the prefix of its connections' IDs.
const (
	kindIDP  = "idp"
	kindSMS  = "sms"
	kindSMTP = "smtp"
	kindPDP  = "pdp"
	kindHTTP = "http"
)

// sharedConnectionKinds names the kinds of connection the shared fixture holds, as the prefix of their IDs.
var sharedConnectionKinds = []string{kindIDP, kindSMS, kindSMTP, kindPDP, kindHTTP}

// confineToOwnOU confines the caller to its own organization unit, where it may do anything, and gives the
// service one connection of each kind the unit owns ("<kind>-own"), one shared into it ("<kind>-shared") and one
// neither ("<kind>-other"). The sharing service is asked whether a connection is shared as the runtime.
func (s *ConnectionAuthzTestSuite) confineToOwnOU() *sharingmock.SharingServiceInterfaceMock {
	s.authz.On("GetBoundary", mock.Anything).
		Return(sysauthz.Boundary{Type: sysauthz.BoundaryTypeOU, OUID: authzOwnOU}).Maybe()
	s.authz.On("GetAccessibleResources", mock.Anything, security.ActionListConnections, security.ResourceTypeOU).
		Return(&sysauthz.AccessibleResources{IDs: []string{authzOwnOU}}, nil).Maybe()
	s.authz.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).Return(
		func(ctx context.Context, _ security.Action, c *sysauthz.ActionContext) (bool, *tidcommon.ServiceError) {
			return security.IsRuntimeContext(ctx) || c.OUID == authzOwnOU, nil
		}).Maybe()

	ouOf := func(id string) string {
		if strings.HasSuffix(id, "-own") {
			return authzOwnOU
		}
		return authzOtherOU
	}
	idps := make([]idp.BasicIDPDTO, 0, 4)
	sms := make([]ncommon.NotificationSenderDTO, 0)
	smtp := make([]ncommon.NotificationSenderDTO, 0)
	pdps := make([]authzenpdp.AuthZENPDPConnection, 0, 3)
	https := make([]httpconnection.HTTPConnection, 0, 3)
	notif := notificationmock.NewNotificationSenderMgtSvcInterfaceMock(s.T())
	pdp := authzenpdpmock.NewAuthZENPDPServiceInterfaceMock(s.T())
	for _, suffix := range []string{"-own", "-shared", "-other"} {
		id := kindIDP + suffix
		idps = append(idps, idp.BasicIDPDTO{ID: id, Name: id, Type: providers.IDPTypeGoogle, OUID: ouOf(id)})
		s.idps.On("GetIdentityProvider", mock.Anything, id).Return(&providers.IDPDTO{
			ID: id, Name: id, Type: providers.IDPTypeGoogle, OUID: ouOf(id)}, (*tidcommon.ServiceError)(nil)).Maybe()

		for _, sender := range []struct {
			kind         string
			senderType   ncommon.NotificationSenderType
			provider     ncommon.NotificationProviderType
			listedSender *[]ncommon.NotificationSenderDTO
		}{
			{kindSMS, ncommon.NotificationSenderTypeMessage, ncommon.NotificationProviderTypeTwilio, &sms},
			{kindSMTP, ncommon.NotificationSenderTypeEmail, ncommon.NotificationProviderTypeSMTP, &smtp},
		} {
			dto := ncommon.NotificationSenderDTO{ID: sender.kind + suffix, Name: sender.kind + suffix,
				Type: sender.senderType, Provider: sender.provider, OUID: ouOf(suffix)}
			*sender.listedSender = append(*sender.listedSender, dto)
			notif.On("GetSender", mock.Anything, dto.ID).Return(&dto, (*tidcommon.ServiceError)(nil)).Maybe()
		}

		pdpID := kindPDP + suffix
		pdpConnection := authzenpdp.AuthZENPDPConnection{ID: pdpID, Name: pdpID, OUID: ouOf(suffix)}
		pdps = append(pdps, pdpConnection)
		pdp.On("GetAuthZENPDP", mock.Anything, pdpConnection.ID).
			Return(&pdpConnection, (*tidcommon.ServiceError)(nil)).Maybe()

		httpID := kindHTTP + suffix
		httpConnection := httpconnection.HTTPConnection{ID: httpID, Name: httpID, OUID: ouOf(suffix)}
		https = append(https, httpConnection)
		s.storedHTTP(httpConnection.ID, httpConnection.OUID)
	}
	// A connection of another vendor shared into the unit is left out of a vendor's list.
	idps = append(idps, idp.BasicIDPDTO{ID: "github-shared", Type: providers.IDPTypeGitHub, OUID: authzOtherOU})
	s.idps.On("GetIdentityProviderList", mock.Anything).Return(idps, (*tidcommon.ServiceError)(nil)).Maybe()
	notif.On("ListSendersByType", mock.Anything, ncommon.NotificationSenderTypeMessage).
		Return(sms, (*tidcommon.ServiceError)(nil)).Maybe()
	notif.On("ListSendersByType", mock.Anything, ncommon.NotificationSenderTypeEmail).
		Return(smtp, (*tidcommon.ServiceError)(nil)).Maybe()
	pdp.On("ListAuthZENPDPs", mock.Anything).Return(pdps, (*tidcommon.ServiceError)(nil)).Maybe()
	s.http.On("ListHTTPConnections", mock.Anything).Return(https, (*tidcommon.ServiceError)(nil)).Maybe()
	s.svc.notificationService = notif
	s.svc.authZENPDPService = pdp

	sharingSvc := sharingmock.NewSharingServiceInterfaceMock(s.T())
	sharingSvc.On("ListVisibleResourceIDs", mock.Anything, ConnectionSharingType, authzOwnOU).
		Return([]string{"github-shared", "http-shared", "idp-shared", "pdp-shared", "sms-shared", "smtp-shared"},
			(*tidcommon.ServiceError)(nil)).Maybe()
	sharingSvc.On("IsVisible", mock.MatchedBy(security.IsRuntimeContext), ConnectionSharingType, mock.Anything,
		authzOwnOU).Return(func(_ context.Context, _ sharing.ResourceType, id, _ string) (bool,
		*tidcommon.ServiceError) {
		return strings.HasSuffix(id, "-shared"), nil
	}).Maybe()
	s.svc.sharingService = sharingSvc
	return sharingSvc
}

// listKind lists the connections of a kind through its vendor's list.
func (s *ConnectionAuthzTestSuite) listKind(kind string) ([]connectionInstance, *tidcommon.ServiceError) {
	ctx := context.Background()
	switch kind {
	case kindIDP:
		return s.svc.listByType(ctx, providers.IDPTypeGoogle)
	case kindSMS:
		return s.svc.listSendersByProvider(ctx, ncommon.NotificationSenderTypeMessage,
			ncommon.NotificationProviderTypeTwilio)
	case kindSMTP:
		return s.svc.listSendersByProvider(ctx, ncommon.NotificationSenderTypeEmail,
			ncommon.NotificationProviderTypeSMTP)
	case kindPDP:
		return s.svc.listAuthZENPDP(ctx)
	default:
		return s.svc.listHTTP(ctx)
	}
}

// getKind reads a connection through its vendor's get, returning the organization unit it is read in and whether
// that unit owns it.
func (s *ConnectionAuthzTestSuite) getKind(kind, id string) (string, bool, *tidcommon.ServiceError) {
	ctx := context.Background()
	switch kind {
	case kindIDP:
		dto, owned, svcErr := s.svc.getByType(ctx, providers.IDPTypeGoogle, id)
		if svcErr != nil {
			return "", false, svcErr
		}
		return dto.OUID, owned, nil
	case kindSMS, kindSMTP:
		senderType, provider := ncommon.NotificationSenderTypeMessage, ncommon.NotificationProviderTypeTwilio
		if kind == kindSMTP {
			senderType, provider = ncommon.NotificationSenderTypeEmail, ncommon.NotificationProviderTypeSMTP
		}
		dto, owned, svcErr := s.svc.getSenderByProvider(ctx, senderType, provider, id)
		if svcErr != nil {
			return "", false, svcErr
		}
		return dto.OUID, owned, nil
	case kindPDP:
		connection, owned, svcErr := s.svc.getAuthZENPDP(ctx, id)
		if svcErr != nil {
			return "", false, svcErr
		}
		return connection.OUID, owned, nil
	default:
		connection, owned, svcErr := s.svc.getHTTP(ctx, id)
		if svcErr != nil {
			return "", false, svcErr
		}
		return connection.OUID, owned, nil
	}
}

// writeKind updates, or with remove deletes, a connection through its vendor's route.
func (s *ConnectionAuthzTestSuite) writeKind(kind, id string, remove bool) *tidcommon.ServiceError {
	ctx := context.Background()
	switch kind {
	case kindIDP:
		if remove {
			return s.svc.deleteByType(ctx, providers.IDPTypeGoogle, id)
		}
		_, svcErr := s.svc.update(ctx, providers.IDPTypeGoogle, id, &providers.IDPDTO{})
		return svcErr
	case kindSMS, kindSMTP:
		senderType, provider := ncommon.NotificationSenderTypeMessage, ncommon.NotificationProviderTypeTwilio
		if kind == kindSMTP {
			senderType, provider = ncommon.NotificationSenderTypeEmail, ncommon.NotificationProviderTypeSMTP
		}
		if remove {
			return s.svc.deleteSenderByProvider(ctx, senderType, provider, id)
		}
		_, svcErr := s.svc.updateSender(ctx, senderType, provider, id, ncommon.NotificationSenderDTO{})
		return svcErr
	case kindPDP:
		if remove {
			return s.svc.deleteAuthZENPDP(ctx, id)
		}
		_, svcErr := s.svc.updateAuthZENPDP(ctx, id, authzenpdp.ConnectionRequest{})
		return svcErr
	default:
		if remove {
			return s.svc.deleteHTTP(ctx, id)
		}
		_, svcErr := s.svc.updateHTTP(ctx, id, httpconnection.HTTPConnection{})
		return svcErr
	}
}

func (s *ConnectionAuthzTestSuite) TestVendorListsOwnedThenShared() {
	s.confineToOwnOU()
	for _, kind := range sharedConnectionKinds {
		s.Run(kind, func() {
			instances, svcErr := s.listKind(kind)
			s.Require().Nil(svcErr)
			s.Require().Len(instances, 2)
			s.Equal(kind+"-own", instances[0].ID)
			s.Equal(authzOwnOU, instances[0].OUID)
			s.Require().NotNil(instances[0].IsOwnedByOU)
			s.True(*instances[0].IsOwnedByOU)
			s.Equal(kind+"-shared", instances[1].ID)
			s.Equal(authzOwnOU, instances[1].OUID)
			s.Require().NotNil(instances[1].IsOwnedByOU)
			s.False(*instances[1].IsOwnedByOU)
		})
	}
}

func (s *ConnectionAuthzTestSuite) TestVendorListsEveryConnectionAsOwnedWhenNotConfined() {
	s.idps.On("GetIdentityProviderList", mock.Anything).Return([]idp.BasicIDPDTO{
		{ID: "idp-own", Type: providers.IDPTypeGoogle, OUID: authzOwnOU},
		{ID: "idp-other", Type: providers.IDPTypeGoogle, OUID: authzOtherOU},
	}, (*tidcommon.ServiceError)(nil))
	s.authz.On("GetAccessibleResources", mock.Anything, security.ActionListConnections, security.ResourceTypeOU).
		Return(&sysauthz.AccessibleResources{AllAllowed: true}, nil)

	instances, svcErr := s.svc.listByType(context.Background(), providers.IDPTypeGoogle)
	s.Require().Nil(svcErr)
	s.Require().Len(instances, 2)
	for _, instance := range instances {
		s.Require().NotNil(instance.IsOwnedByOU)
		s.True(*instance.IsOwnedByOU)
	}
	s.Equal(authzOtherOU, instances[1].OUID)
}

func (s *ConnectionAuthzTestSuite) TestVendorListFailsWhenSharingCannotBeRead() {
	sharingSvc := s.confineToOwnOU()
	sharingSvc.ExpectedCalls = nil
	sharingSvc.On("ListVisibleResourceIDs", mock.Anything, ConnectionSharingType, authzOwnOU).
		Return([]string(nil), &tidcommon.InternalServerError)

	_, svcErr := s.svc.listHTTP(context.Background())
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ConnectionAuthzTestSuite) TestGetReadsOwnedAndSharedConnections() {
	s.confineToOwnOU()
	for _, kind := range sharedConnectionKinds {
		s.Run(kind, func() {
			ouID, owned, svcErr := s.getKind(kind, kind+"-own")
			s.Require().Nil(svcErr)
			s.True(owned)
			s.Equal(authzOwnOU, ouID)

			ouID, owned, svcErr = s.getKind(kind, kind+"-shared")
			s.Require().Nil(svcErr)
			s.False(owned)
			s.Equal(authzOwnOU, ouID)

			_, _, svcErr = s.getKind(kind, kind+"-other")
			s.Require().NotNil(svcErr)
			s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
		})
	}
}

func (s *ConnectionAuthzTestSuite) TestSharedGetLeavesTheStoredConnectionUnchanged() {
	s.confineToOwnOU()
	stored := &providers.IDPDTO{ID: "idp-stored-shared", Type: providers.IDPTypeGoogle, OUID: authzOtherOU}
	s.idps.On("GetIdentityProvider", mock.Anything, stored.ID).Return(stored, (*tidcommon.ServiceError)(nil))

	dto, owned, svcErr := s.svc.getByType(context.Background(), providers.IDPTypeGoogle, stored.ID)
	s.Require().Nil(svcErr)
	s.False(owned)
	s.Equal(authzOwnOU, dto.OUID)
	s.Equal(authzOtherOU, stored.OUID)
}

func (s *ConnectionAuthzTestSuite) TestSharedGetNeedsReadInTheCallersOU() {
	s.authz.On("GetBoundary", mock.Anything).
		Return(sysauthz.Boundary{Type: sysauthz.BoundaryTypeOU, OUID: authzOwnOU})
	s.authz.On("IsActionAllowed", mock.Anything, security.ActionReadConnection, mock.Anything).Return(false, nil)
	s.storedHTTP("http-shared", authzOtherOU)
	sharingSvc := sharingmock.NewSharingServiceInterfaceMock(s.T())
	sharingSvc.On("IsVisible", mock.MatchedBy(security.IsRuntimeContext), ConnectionSharingType, "http-shared",
		authzOwnOU).Return(true, nil).Once()
	s.svc.sharingService = sharingSvc

	_, _, svcErr := s.svc.getHTTP(context.Background(), "http-shared")
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
}

func (s *ConnectionAuthzTestSuite) TestUsagesOfASharedConnectionAreRefused() {
	s.confineToOwnOU()

	_, svcErr := s.svc.usagesHTTP(context.Background(), "http-shared")
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
}

func (s *ConnectionAuthzTestSuite) TestWritesToASharedConnectionAreReadOnly() {
	s.confineToOwnOU()
	for _, kind := range sharedConnectionKinds {
		for _, remove := range []bool{false, true} {
			s.Run(fmt.Sprintf("%s remove=%t", kind, remove), func() {
				svcErr := s.writeKind(kind, kind+"-shared", remove)
				s.Require().NotNil(svcErr)
				s.Equal(ErrorSharedConnectionReadOnly.Code, svcErr.Code)

				svcErr = s.writeKind(kind, kind+"-other", remove)
				s.Require().NotNil(svcErr)
				s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
			})
		}
	}
}

func (s *ConnectionAuthzTestSuite) TestSharingFailuresAreServerErrors() {
	sharingSvc := s.confineToOwnOU()
	sharingSvc.ExpectedCalls = nil
	sharingSvc.On("IsVisible", mock.Anything, ConnectionSharingType, "http-shared", authzOwnOU).
		Return(false, &tidcommon.InternalServerError)

	_, _, svcErr := s.svc.getHTTP(context.Background(), "http-shared")
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)

	svcErr = s.svc.deleteHTTP(context.Background(), "http-shared")
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

// A token naming an organization unit that no longer exists has nothing shared into it, so it is refused rather than
// answered with a server error.
func (s *ConnectionAuthzTestSuite) TestTheTokensUnitIsGone() {
	sharingSvc := s.confineToOwnOU()
	sharingSvc.ExpectedCalls = nil
	sharingSvc.On("IsVisible", mock.Anything, ConnectionSharingType, "http-shared", authzOwnOU).
		Return(false, &oupkg.ErrorOrganizationUnitNotFound)

	_, _, svcErr := s.svc.getHTTP(context.Background(), "http-shared")
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)

	svcErr = s.svc.deleteHTTP(context.Background(), "http-shared")
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
}

func (s *ConnectionAuthzTestSuite) TestRoutesAnswerASharedConnection() {
	s.confineToOwnOU()
	mux := http.NewServeMux()
	registerRoutes(mux, newHandler(s.svc))
	serve := func(method, path string) *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, httptest.NewRequest(method, path, strings.NewReader(`{"name":"x"}`)))
		return rr
	}

	for _, path := range []string{"/connections/google/idp-shared", "/connections/twilio/sms-shared",
		"/connections/authzen-pdp/pdp-shared", "/connections/http/http-shared"} {
		s.Run(path, func() {
			rr := serve(http.MethodGet, path)
			s.Equal(http.StatusOK, rr.Code)
			var body map[string]any
			s.Require().NoError(json.Unmarshal(rr.Body.Bytes(), &body))
			s.Equal(false, body["isOwnedByOU"])
			s.Equal(authzOwnOU, body["ouId"])

			rr = serve(http.MethodDelete, path)
			s.Equal(http.StatusForbidden, rr.Code)
			s.Contains(rr.Body.String(), ErrorSharedConnectionReadOnly.Code)
		})
	}

	rr := serve(http.MethodGet, "/connections/http/http-own")
	s.Equal(http.StatusOK, rr.Code)
	s.Contains(rr.Body.String(), `"isOwnedByOU":true`)

	rr = serve(http.MethodGet, "/connections/http")
	s.Equal(http.StatusOK, rr.Code)
	s.JSONEq(`[{"id":"http-own","name":"http-own","ouId":"ou-own","isOwnedByOU":true},`+
		`{"id":"http-shared","name":"http-shared","ouId":"ou-own","isOwnedByOU":false}]`, rr.Body.String())
}
