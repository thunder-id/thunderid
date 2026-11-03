// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/connection/authzenpdp"
	"github.com/thunder-id/thunderid/internal/connection/httpconnection"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/notification"
	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/connection/authzenpdpmock"
	"github.com/thunder-id/thunderid/tests/mocks/connection/httpconnectionmock"
	"github.com/thunder-id/thunderid/tests/mocks/idp/idpmock"
	"github.com/thunder-id/thunderid/tests/mocks/notification/notificationmock"
	"github.com/thunder-id/thunderid/tests/mocks/sharingmock"
)

type SharingTestSuite struct {
	suite.Suite
	mockIDP     *idpmock.IDPServiceInterfaceMock
	mockNotif   *notificationmock.NotificationSenderMgtSvcInterfaceMock
	mockPDP     *authzenpdpmock.AuthZENPDPServiceInterfaceMock
	mockHTTP    *httpconnectionmock.HTTPConnectionServiceInterfaceMock
	mockSharing *sharingmock.SharingServiceInterfaceMock
	svc         *service
}

func TestSharingSuite(t *testing.T) {
	suite.Run(t, new(SharingTestSuite))
}

func (s *SharingTestSuite) SetupTest() {
	initConfigWithTestCryptoKey(s.T())
	s.mockIDP = idpmock.NewIDPServiceInterfaceMock(s.T())
	s.mockNotif = notificationmock.NewNotificationSenderMgtSvcInterfaceMock(s.T())
	s.mockPDP = authzenpdpmock.NewAuthZENPDPServiceInterfaceMock(s.T())
	s.mockHTTP = httpconnectionmock.NewHTTPConnectionServiceInterfaceMock(s.T())
	s.mockSharing = sharingmock.NewSharingServiceInterfaceMock(s.T())
	s.svc = newService(s.mockIDP, s.mockNotif, &testResourceServerLister{}, s.mockPDP, s.mockHTTP)
	s.svc.sharingService = s.mockSharing
}

func (s *SharingTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

// ownedBy makes the connection resolve to the given organization unit through the kind that holds it.
func (s *SharingTestSuite) ownedByIDP(id, ouID string) {
	s.mockIDP.On("GetIdentityProvider", mock.Anything, id).
		Return(&providers.IDPDTO{ID: id, Type: providers.IDPTypeGoogle, OUID: ouID}, (*tidcommon.ServiceError)(nil))
}

func (s *SharingTestSuite) idpMissing(id string) {
	s.mockIDP.On("GetIdentityProvider", mock.Anything, id).Return(nil, &idp.ErrorIDPNotFound)
}

func (s *SharingTestSuite) senderMissing(id string) {
	s.mockNotif.On("GetSender", mock.Anything, id).Return(nil, &notification.ErrorSenderNotFound)
}

func (s *SharingTestSuite) pdpMissing(id string) {
	s.mockPDP.On("GetAuthZENPDP", mock.Anything, id).
		Return((*authzenpdp.AuthZENPDPConnection)(nil), (*tidcommon.ServiceError)(nil))
}

func (s *SharingTestSuite) TestDeclarationIsReadOnly() {
	decl := newConnectionSharingDeclaration(s.svc)

	s.Equal(ConnectionSharingType, decl.ResourceType())
	s.Empty(decl.Fields(), "a connection declares no overlay field, so a sharee can override nothing")
}

func (s *SharingTestSuite) TestOwningOUIDResolvesEveryKind() {
	s.Run("identity provider", func() {
		s.SetupTest()
		s.ownedByIDP("c-1", "ou-idp")

		ouID, svcErr := s.svc.owningOUID(context.Background(), "c-1")

		s.Nil(svcErr)
		s.Equal("ou-idp", ouID)
	})
	s.Run("notification sender", func() {
		s.SetupTest()
		s.idpMissing("c-1")
		s.mockNotif.On("GetSender", mock.Anything, "c-1").Return(
			&ncommon.NotificationSenderDTO{ID: "c-1", OUID: "ou-sender"}, (*tidcommon.ServiceError)(nil))

		ouID, svcErr := s.svc.owningOUID(context.Background(), "c-1")

		s.Nil(svcErr)
		s.Equal("ou-sender", ouID)
	})
	s.Run("authzen pdp", func() {
		s.SetupTest()
		s.idpMissing("c-1")
		s.senderMissing("c-1")
		s.mockPDP.On("GetAuthZENPDP", mock.Anything, "c-1").Return(
			&authzenpdp.AuthZENPDPConnection{ID: "c-1", OUID: "ou-pdp"}, (*tidcommon.ServiceError)(nil))

		ouID, svcErr := s.svc.owningOUID(context.Background(), "c-1")

		s.Nil(svcErr)
		s.Equal("ou-pdp", ouID)
	})
	s.Run("http", func() {
		s.SetupTest()
		s.idpMissing("c-1")
		s.senderMissing("c-1")
		s.pdpMissing("c-1")
		s.mockHTTP.On("GetHTTPConnection", mock.Anything, "c-1").Return(
			&httpconnection.HTTPConnection{ID: "c-1", OUID: "ou-http"}, (*tidcommon.ServiceError)(nil))

		ouID, svcErr := s.svc.owningOUID(context.Background(), "c-1")

		s.Nil(svcErr)
		s.Equal("ou-http", ouID)
	})
}

func (s *SharingTestSuite) TestOwningOUIDUnknownConnection() {
	s.idpMissing("c-1")
	s.senderMissing("c-1")
	s.pdpMissing("c-1")
	s.mockHTTP.On("GetHTTPConnection", mock.Anything, "c-1").Return(nil, &httpconnection.ErrorNotFound)

	_, svcErr := s.svc.owningOUID(context.Background(), "c-1")

	s.Require().NotNil(svcErr)
	s.Equal(authzenpdp.ErrorNotFound.Code, svcErr.Code)
}

func (s *SharingTestSuite) TestOwningOUIDStopsOnLookupFailure() {
	s.mockIDP.On("GetIdentityProvider", mock.Anything, "c-1").Return(nil, &tidcommon.InternalServerError)

	_, svcErr := s.svc.owningOUID(context.Background(), "c-1")

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	s.mockNotif.AssertNotCalled(s.T(), "GetSender", mock.Anything, mock.Anything)
}

func (s *SharingTestSuite) TestCreateSharingPolicyIssuesFromOwner() {
	s.ownedByIDP("c-1", "ou-owner")
	req := sharing.PolicyRequest{}
	s.mockSharing.On("CreatePolicy", mock.Anything, ConnectionSharingType, "c-1", "ou-owner", req).
		Return(sharing.Policy{ID: "p-1"}, (*tidcommon.ServiceError)(nil))

	policy, svcErr := s.svc.createSharingPolicy(context.Background(), s.svc.owningOUID, "c-1", req)

	s.Nil(svcErr)
	s.Equal("p-1", policy.ID)
}

func (s *SharingTestSuite) TestCreateSharingPolicyTranslatesFrameworkErrors() {
	s.ownedByIDP("c-1", "ou-owner")
	s.mockSharing.On("CreatePolicy", mock.Anything, ConnectionSharingType, "c-1", "ou-owner", mock.Anything).
		Return(sharing.Policy{}, &sharing.ErrorPolicyExists)

	_, svcErr := s.svc.createSharingPolicy(context.Background(), s.svc.owningOUID, "c-1", sharing.PolicyRequest{})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorSharingPolicyExists.Code, svcErr.Code)
}

func (s *SharingTestSuite) TestCreateSharingPolicyUnknownConnection() {
	s.idpMissing("c-1")
	s.senderMissing("c-1")
	s.pdpMissing("c-1")
	s.mockHTTP.On("GetHTTPConnection", mock.Anything, "c-1").Return(nil, &httpconnection.ErrorNotFound)

	_, svcErr := s.svc.createSharingPolicy(context.Background(), s.svc.owningOUID, "c-1", sharing.PolicyRequest{})

	s.Require().NotNil(svcErr)
	s.Equal(authzenpdp.ErrorNotFound.Code, svcErr.Code)
}

func (s *SharingTestSuite) TestCreateSharingPolicyRefusedInDeclarativeMode() {
	s.ownedByIDP("c-1", "ou-owner")
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime("/tmp/test", &config.Config{
		DeclarativeResources: config.DeclarativeResources{Enabled: true},
	}))

	_, svcErr := s.svc.createSharingPolicy(context.Background(), s.svc.owningOUID, "c-1", sharing.PolicyRequest{})

	s.NotNil(svcErr)
	s.mockSharing.AssertNotCalled(s.T(), "CreatePolicy", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything)
}

func (s *SharingTestSuite) TestPolicyOfAnotherConnectionIsNotFound() {
	s.ownedByIDP("c-1", "ou-owner")
	s.mockSharing.On("GetPolicy", mock.Anything, "p-1").Return(
		sharing.Policy{ID: "p-1", ResourceType: ConnectionSharingType, ResourceID: "c-2"},
		(*tidcommon.ServiceError)(nil))

	_, svcErr := s.svc.getSharingPolicy(context.Background(), s.svc.owningOUID, "c-1", "p-1")

	s.Require().NotNil(svcErr)
	s.Equal(ErrorSharingPolicyNotFound.Code, svcErr.Code)
}

func (s *SharingTestSuite) TestPolicyOfAnotherResourceTypeIsNotFound() {
	s.ownedByIDP("c-1", "ou-owner")
	s.mockSharing.On("GetPolicy", mock.Anything, "p-1").Return(
		sharing.Policy{ID: "p-1", ResourceType: "application", ResourceID: "c-1"},
		(*tidcommon.ServiceError)(nil))

	_, svcErr := s.svc.getSharingPolicy(context.Background(), s.svc.owningOUID, "c-1", "p-1")

	s.Require().NotNil(svcErr)
	s.Equal(ErrorSharingPolicyNotFound.Code, svcErr.Code)
}

func (s *SharingTestSuite) TestListSharingPolicies() {
	s.ownedByIDP("c-1", "ou-owner")
	s.mockSharing.On("GetPolicyList", mock.Anything, ConnectionSharingType, "c-1", 10, 0).Return(
		sharing.PolicyList{TotalResults: 1, Count: 1, Policies: []sharing.Policy{{ID: "p-1"}}},
		(*tidcommon.ServiceError)(nil))

	list, svcErr := s.svc.listSharingPolicies(context.Background(), s.svc.owningOUID, "c-1", 10, 0)

	s.Nil(svcErr)
	s.Equal(1, list.Count)
}

func (s *SharingTestSuite) TestUpdateSharingPolicy() {
	s.ownedByIDP("c-1", "ou-owner")
	s.mockSharing.On("GetPolicy", mock.Anything, "p-1").Return(
		sharing.Policy{ID: "p-1", ResourceType: ConnectionSharingType, ResourceID: "c-1"},
		(*tidcommon.ServiceError)(nil))
	s.mockSharing.On("UpdatePolicy", mock.Anything, "p-1", mock.Anything).
		Return(sharing.Policy{ID: "p-1"}, (*tidcommon.ServiceError)(nil))

	policy, svcErr := s.svc.updateSharingPolicy(
		context.Background(), s.svc.owningOUID, "c-1", "p-1", sharing.PolicyRequest{})

	s.Nil(svcErr)
	s.Equal("p-1", policy.ID)
}

func (s *SharingTestSuite) TestDeleteSharingPolicy() {
	s.ownedByIDP("c-1", "ou-owner")
	s.mockSharing.On("GetPolicy", mock.Anything, "p-1").Return(
		sharing.Policy{ID: "p-1", ResourceType: ConnectionSharingType, ResourceID: "c-1"},
		(*tidcommon.ServiceError)(nil))
	s.mockSharing.On("DeletePolicy", mock.Anything, "p-1").Return((*tidcommon.ServiceError)(nil))

	s.Nil(s.svc.deleteSharingPolicy(context.Background(), s.svc.owningOUID, "c-1", "p-1"))
}

func (s *SharingTestSuite) TestDeleteSharingPolicyThatIsGoneSucceeds() {
	s.ownedByIDP("c-1", "ou-owner")
	s.mockSharing.On("GetPolicy", mock.Anything, "p-1").Return(sharing.Policy{}, &sharing.ErrorPolicyNotFound)

	s.Nil(s.svc.deleteSharingPolicy(context.Background(), s.svc.owningOUID, "c-1", "p-1"))
	s.mockSharing.AssertNotCalled(s.T(), "DeletePolicy", mock.Anything, mock.Anything)
}

func (s *SharingTestSuite) TestDeleteDeclaredSharingPolicyIsRefused() {
	s.ownedByIDP("c-1", "ou-owner")
	s.mockSharing.On("GetPolicy", mock.Anything, "p-1").Return(
		sharing.Policy{ID: "p-1", ResourceType: ConnectionSharingType, ResourceID: "c-1", Declared: true},
		(*tidcommon.ServiceError)(nil))

	svcErr := s.svc.deleteSharingPolicy(context.Background(), s.svc.owningOUID, "c-1", "p-1")

	s.Require().NotNil(svcErr)
	s.Equal(ErrorSharingPolicyDeclared.Code, svcErr.Code)
	s.mockSharing.AssertNotCalled(s.T(), "DeletePolicy", mock.Anything, mock.Anything)
}

func (s *SharingTestSuite) TestResolveSharingOverlay() {
	s.ownedByIDP("c-1", "ou-owner")
	s.mockSharing.On("ResolveOverlayRules", mock.Anything, ConnectionSharingType, "c-1", "ou-child").
		Return(sharing.ResolvedOverlay{OUID: "ou-child", Visible: true}, (*tidcommon.ServiceError)(nil))

	overlay, svcErr := s.svc.resolveSharingOverlay(context.Background(), s.svc.owningOUID, "c-1", "ou-child")

	s.Nil(svcErr)
	s.True(overlay.Visible)
	s.Empty(overlay.Rules, "a shared connection carries no overridable field")
}

func (s *SharingTestSuite) TestResolveSharingOverlayNotVisible() {
	s.ownedByIDP("c-1", "ou-owner")
	s.mockSharing.On("ResolveOverlayRules", mock.Anything, ConnectionSharingType, "c-1", "ou-other").
		Return(sharing.ResolvedOverlay{OUID: "ou-other"}, (*tidcommon.ServiceError)(nil))

	_, svcErr := s.svc.resolveSharingOverlay(context.Background(), s.svc.owningOUID, "c-1", "ou-other")

	s.Require().NotNil(svcErr)
	s.Equal(ErrorConnectionNotSharedToOU.Code, svcErr.Code)
}

func (s *SharingTestSuite) TestResolveSharingOverlayUnknownOU() {
	s.ownedByIDP("c-1", "ou-owner")
	s.mockSharing.On("ResolveOverlayRules", mock.Anything, ConnectionSharingType, "c-1", "ou-gone").
		Return(sharing.ResolvedOverlay{}, &oupkg.ErrorOrganizationUnitNotFound)

	_, svcErr := s.svc.resolveSharingOverlay(context.Background(), s.svc.owningOUID, "c-1", "ou-gone")

	s.Require().NotNil(svcErr)
	s.Equal(ErrorConnectionNotSharedToOU.Code, svcErr.Code)
}

func (s *SharingTestSuite) TestResolveSharingOverlayRequiresOU() {
	s.ownedByIDP("c-1", "ou-owner")

	_, svcErr := s.svc.resolveSharingOverlay(context.Background(), s.svc.owningOUID, "c-1", "")

	s.Require().NotNil(svcErr)
	s.Equal(ErrorMissingOUIDParameter.Code, svcErr.Code)
}

// hasPolicies makes the connection hold one sharing policy.
func (s *SharingTestSuite) hasPolicies(id string) {
	s.mockSharing.On("ListPolicies", mock.Anything, ConnectionSharingType, id).
		Return([]sharing.Policy{{ID: "p-1"}}, (*tidcommon.ServiceError)(nil))
}

// goneEverywhere makes every service that holds connections report that it has none with the id.
func (s *SharingTestSuite) goneEverywhere(id string) {
	s.idpMissing(id)
	s.senderMissing(id)
	s.pdpMissing(id)
	s.mockHTTP.On("GetHTTPConnection", mock.Anything, id).Return(nil, &httpconnection.ErrorNotFound)
}

// httpHolds makes the HTTP connection service hold a connection with the id.
func (s *SharingTestSuite) httpHolds(id string) {
	s.mockHTTP.On("GetHTTPConnection", mock.Anything, id).Return(
		&httpconnection.HTTPConnection{ID: id}, (*tidcommon.ServiceError)(nil))
}

// Deleting a connection removes its sharing policies once the connection itself is gone.
func (s *SharingTestSuite) TestDeleteRemovesSharingPolicies() {
	s.Run("identity provider", func() {
		s.SetupTest()
		s.mockIDP.On("GetIdentityProvider", mock.Anything, "g-1").Return(
			&providers.IDPDTO{ID: "g-1", Type: providers.IDPTypeGoogle}, (*tidcommon.ServiceError)(nil))
		s.mockIDP.On("DeleteIdentityProvider", mock.Anything, "g-1").Return((*tidcommon.ServiceError)(nil)).Once()
		s.mockSharing.On("DeletePoliciesForResource", mock.Anything, ConnectionSharingType, "g-1").
			Return((*tidcommon.ServiceError)(nil)).Once()

		s.Nil(s.svc.deleteByType(context.Background(), providers.IDPTypeGoogle, "g-1"))
	})
	s.Run("notification sender", func() {
		s.SetupTest()
		s.mockNotif.On("GetSender", mock.Anything, "s-1").Return(&ncommon.NotificationSenderDTO{
			ID: "s-1", Type: ncommon.NotificationSenderTypeMessage,
			Provider: ncommon.NotificationProviderTypeTwilio,
		}, (*tidcommon.ServiceError)(nil))
		s.mockNotif.On("DeleteSender", mock.Anything, "s-1").Return((*tidcommon.ServiceError)(nil)).Once()
		s.mockSharing.On("DeletePoliciesForResource", mock.Anything, ConnectionSharingType, "s-1").
			Return((*tidcommon.ServiceError)(nil)).Once()

		s.Nil(s.svc.deleteSenderByProvider(context.Background(), ncommon.NotificationSenderTypeMessage,
			ncommon.NotificationProviderTypeTwilio, "s-1"))
	})
	s.Run("authzen pdp", func() {
		s.SetupTest()
		s.svc.resourceService = &testResourceServerLister{}
		s.mockPDP.On("GetAuthZENPDP", mock.Anything, "pdp-1").Return(
			&authzenpdp.AuthZENPDPConnection{ID: "pdp-1"}, (*tidcommon.ServiceError)(nil))
		s.mockPDP.On("DeleteAuthZENPDPConnection", mock.Anything, "pdp-1").
			Return((*tidcommon.ServiceError)(nil)).Once()
		s.mockSharing.On("DeletePoliciesForResource", mock.Anything, ConnectionSharingType, "pdp-1").
			Return((*tidcommon.ServiceError)(nil)).Once()

		s.Nil(s.svc.deleteAuthZENPDP(context.Background(), "pdp-1"))
	})
	s.Run("http", func() {
		s.SetupTest()
		s.httpHolds("h-1")
		s.mockHTTP.On("DeleteHTTPConnection", mock.Anything, "h-1").Return((*tidcommon.ServiceError)(nil)).Once()
		s.mockSharing.On("DeletePoliciesForResource", mock.Anything, ConnectionSharingType, "h-1").
			Return((*tidcommon.ServiceError)(nil)).Once()

		s.Nil(s.svc.deleteHTTP(context.Background(), "h-1"))
	})
}

// A delete the connection's own service refuses keeps the policies: the connection is still there. The service
// knows reasons no check made beforehand could, such as the connection being held as immutable.
func (s *SharingTestSuite) TestRefusedDeleteKeepsSharingPolicies() {
	s.mockIDP.On("GetIdentityProvider", mock.Anything, "g-1").Return(
		&providers.IDPDTO{ID: "g-1", Type: providers.IDPTypeGoogle}, (*tidcommon.ServiceError)(nil))
	s.mockIDP.On("DeleteIdentityProvider", mock.Anything, "g-1").Return(&idp.ErrorIDPHasBlockingDependencies)

	svcErr := s.svc.deleteByType(context.Background(), providers.IDPTypeGoogle, "g-1")

	s.Require().NotNil(svcErr)
	s.Equal(idp.ErrorIDPHasBlockingDependencies.Code, svcErr.Code)
	s.mockSharing.AssertNotCalled(s.T(), "DeletePoliciesForResource", mock.Anything, mock.Anything, mock.Anything)
}

// When the policies cannot be removed after the connection is gone, the caller is told so it can retry.
func (s *SharingTestSuite) TestDeleteReportsPoliciesThatCannotBeRemoved() {
	s.httpHolds("h-1")
	s.mockHTTP.On("DeleteHTTPConnection", mock.Anything, "h-1").Return((*tidcommon.ServiceError)(nil))
	s.mockSharing.On("DeletePoliciesForResource", mock.Anything, ConnectionSharingType, "h-1").
		Return(&tidcommon.InternalServerError)

	svcErr := s.svc.deleteHTTP(context.Background(), "h-1")

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

// Retrying the delete of a connection that is gone removes the policies a failed cleanup left behind, and succeeds.
func (s *SharingTestSuite) TestRetriedDeleteRemovesOrphanedPolicies() {
	s.Run("identity provider", func() {
		s.SetupTest()
		s.goneEverywhere("g-1")
		s.hasPolicies("g-1")
		s.mockSharing.On("DeletePoliciesForResource", mock.Anything, ConnectionSharingType, "g-1").
			Return((*tidcommon.ServiceError)(nil)).Once()

		s.Nil(s.svc.deleteByType(context.Background(), providers.IDPTypeGoogle, "g-1"))
	})
	s.Run("notification sender", func() {
		s.SetupTest()
		s.goneEverywhere("s-1")
		s.hasPolicies("s-1")
		s.mockSharing.On("DeletePoliciesForResource", mock.Anything, ConnectionSharingType, "s-1").
			Return((*tidcommon.ServiceError)(nil)).Once()

		s.Nil(s.svc.deleteSenderByProvider(context.Background(), ncommon.NotificationSenderTypeMessage,
			ncommon.NotificationProviderTypeTwilio, "s-1"))
	})
	s.Run("authzen pdp", func() {
		s.SetupTest()
		s.goneEverywhere("pdp-1")
		s.hasPolicies("pdp-1")
		s.mockSharing.On("DeletePoliciesForResource", mock.Anything, ConnectionSharingType, "pdp-1").
			Return((*tidcommon.ServiceError)(nil)).Once()

		s.Nil(s.svc.deleteAuthZENPDP(context.Background(), "pdp-1"))
	})
	s.Run("http", func() {
		s.SetupTest()
		s.goneEverywhere("h-1")
		s.hasPolicies("h-1")
		s.mockSharing.On("DeletePoliciesForResource", mock.Anything, ConnectionSharingType, "h-1").
			Return((*tidcommon.ServiceError)(nil)).Once()

		s.Nil(s.svc.deleteHTTP(context.Background(), "h-1"))
	})
}

// A delete of a connection that is gone and left nothing behind answers as it did before sharing existed.
func (s *SharingTestSuite) TestRetriedDeleteWithNothingLeftIsNotFound() {
	s.goneEverywhere("g-1")
	s.mockSharing.On("ListPolicies", mock.Anything, ConnectionSharingType, "g-1").
		Return([]sharing.Policy{}, (*tidcommon.ServiceError)(nil))

	svcErr := s.svc.deleteByType(context.Background(), providers.IDPTypeGoogle, "g-1")

	s.Require().NotNil(svcErr)
	s.Equal(idp.ErrorIDPNotFound.Code, svcErr.Code)
	s.mockSharing.AssertNotCalled(s.T(), "DeletePoliciesForResource", mock.Anything, mock.Anything, mock.Anything)
}

// A request on one vendor's path that names a connection of another kind must not remove that connection's
// policies: the connection is not gone, it is only not of that kind.
func (s *SharingTestSuite) TestDeleteOnTheWrongVendorKeepsPolicies() {
	s.idpMissing("s-1")
	s.mockNotif.On("GetSender", mock.Anything, "s-1").Return(
		&ncommon.NotificationSenderDTO{ID: "s-1", OUID: "ou-owner"}, (*tidcommon.ServiceError)(nil))

	svcErr := s.svc.deleteByType(context.Background(), providers.IDPTypeGoogle, "s-1")

	s.Require().NotNil(svcErr)
	s.Equal(idp.ErrorIDPNotFound.Code, svcErr.Code)
	s.mockSharing.AssertNotCalled(s.T(), "ListPolicies", mock.Anything, mock.Anything, mock.Anything)
	s.mockSharing.AssertNotCalled(s.T(), "DeletePoliciesForResource", mock.Anything, mock.Anything, mock.Anything)
}

// The HTTP connection service treats deleting a connection it does not hold as success, so a delete on the HTTP
// path that names a connection of another kind must still leave that connection's policies alone.
func (s *SharingTestSuite) TestHTTPDeleteOfAnotherKindKeepsPolicies() {
	s.mockHTTP.On("GetHTTPConnection", mock.Anything, "g-1").Return(nil, &httpconnection.ErrorNotFound)
	s.ownedByIDP("g-1", "ou-owner")

	s.Nil(s.svc.deleteHTTP(context.Background(), "g-1"))

	s.mockHTTP.AssertNotCalled(s.T(), "DeleteHTTPConnection", mock.Anything, mock.Anything)
	s.mockSharing.AssertNotCalled(s.T(), "ListPolicies", mock.Anything, mock.Anything, mock.Anything)
	s.mockSharing.AssertNotCalled(s.T(), "DeletePoliciesForResource", mock.Anything, mock.Anything, mock.Anything)
}

// A cleanup that fails on the retry is reported, so the caller can retry again.
func (s *SharingTestSuite) TestRetriedDeleteReportsFailedCleanup() {
	s.goneEverywhere("g-1")
	s.hasPolicies("g-1")
	s.mockSharing.On("DeletePoliciesForResource", mock.Anything, ConnectionSharingType, "g-1").
		Return(&tidcommon.InternalServerError)

	svcErr := s.svc.deleteByType(context.Background(), providers.IDPTypeGoogle, "g-1")

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *SharingTestSuite) TestHandlerRefusesDeploymentWideSharing() {
	h := newHandler(s.svc)
	mux := http.NewServeMux()
	registerRoutes(mux, h)

	req := httptest.NewRequest(http.MethodPost, "/connections/google/g-1/sharing-policies",
		strings.NewReader(`{"allUnits":{}}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Contains(rec.Body.String(), ErrorDeploymentWideSharingNotAllowed.Code)
}

func (s *SharingTestSuite) TestHandlerRoutesSharingPolicyLifecycle() {
	h := newHandler(s.svc)
	mux := http.NewServeMux()
	registerRoutes(mux, h)
	s.ownedByIDP("g-1", "ou-owner")
	s.mockSharing.On("CreatePolicy", mock.Anything, ConnectionSharingType, "g-1", "ou-owner", mock.Anything).
		Return(sharing.Policy{ID: "p-1", ResourceType: ConnectionSharingType, ResourceID: "g-1"},
			(*tidcommon.ServiceError)(nil))
	s.mockSharing.On("GetPolicy", mock.Anything, "p-1").Return(
		sharing.Policy{ID: "p-1", ResourceType: ConnectionSharingType, ResourceID: "g-1"},
		(*tidcommon.ServiceError)(nil))
	s.mockSharing.On("DeletePolicy", mock.Anything, "p-1").Return((*tidcommon.ServiceError)(nil))

	create := httptest.NewRecorder()
	mux.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/connections/google/g-1/sharing-policies",
		strings.NewReader(`{"childUnits":{"mode":"all"}}`)))
	s.Equal(http.StatusCreated, create.Code)

	get := httptest.NewRecorder()
	mux.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/connections/google/g-1/sharing-policies/p-1", nil))
	s.Equal(http.StatusOK, get.Code)

	del := httptest.NewRecorder()
	mux.ServeHTTP(del, httptest.NewRequest(http.MethodDelete,
		"/connections/google/g-1/sharing-policies/p-1", nil))
	s.Equal(http.StatusNoContent, del.Code)
}

// Every sharing route answers not found for another vendor's connection, as the connection's own get
// route does, and never reaches the sharing service.
func (s *SharingTestSuite) TestHandlerRefusesSharingRoutesOnTheWrongVendor() {
	h := newHandler(s.svc)
	mux := http.NewServeMux()
	registerRoutes(mux, h)
	s.idpMissing("s-1")
	s.mockNotif.On("GetSender", mock.Anything, "s-1").Return(
		&ncommon.NotificationSenderDTO{ID: "s-1", OUID: "ou-owner",
			Type: ncommon.NotificationSenderTypeMessage, Provider: ncommon.NotificationProviderTypeTwilio},
		(*tidcommon.ServiceError)(nil))

	for _, call := range []struct{ method, path, body string }{
		{http.MethodPost, "/connections/google/s-1/sharing-policies", `{"childUnits":{"mode":"all"}}`},
		{http.MethodGet, "/connections/google/s-1/sharing-policies", ""},
		{http.MethodGet, "/connections/google/s-1/overlay-rules?ouId=ou-child", ""},
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(call.method, call.path, strings.NewReader(call.body)))
		s.Equal(http.StatusNotFound, rec.Code, call.method+" "+call.path)
	}

	// The same sender is reachable through its own vendor.
	s.mockSharing.On("GetPolicyList", mock.Anything, ConnectionSharingType, "s-1", mock.Anything, mock.Anything).
		Return(sharing.PolicyList{}, (*tidcommon.ServiceError)(nil))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/connections/twilio/s-1/sharing-policies", nil))
	s.Equal(http.StatusOK, rec.Code)

	s.mockSharing.AssertNotCalled(s.T(), "CreatePolicy", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything)
	s.mockSharing.AssertNotCalled(s.T(), "ResolveOverlayRules", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything)
}

func (s *SharingTestSuite) TestHandlerMapsSharingErrors() {
	h := newHandler(s.svc)
	mux := http.NewServeMux()
	registerRoutes(mux, h)
	s.ownedByIDP("g-1", "ou-owner")
	s.mockSharing.On("ResolveOverlayRules", mock.Anything, ConnectionSharingType, "g-1", "ou-other").
		Return(sharing.ResolvedOverlay{}, (*tidcommon.ServiceError)(nil))
	s.mockSharing.On("CreatePolicy", mock.Anything, ConnectionSharingType, "g-1", "ou-owner", mock.Anything).
		Return(sharing.Policy{}, &sharing.ErrorPolicyExists)

	notShared := httptest.NewRecorder()
	mux.ServeHTTP(notShared, httptest.NewRequest(http.MethodGet,
		"/connections/google/g-1/overlay-rules?ouId=ou-other", nil))
	s.Equal(http.StatusNotFound, notShared.Code)

	exists := httptest.NewRecorder()
	mux.ServeHTTP(exists, httptest.NewRequest(http.MethodPost, "/connections/google/g-1/sharing-policies",
		strings.NewReader(`{"childUnits":{"mode":"all"}}`)))
	s.Equal(http.StatusConflict, exists.Code)
}

func (s *SharingTestSuite) TestExportCarriesSharingPolicies() {
	s.mockIDP.On("GetIdentityProvider", mock.Anything, "g-1").Return(
		&providers.IDPDTO{ID: "g-1", Name: "Google", Type: providers.IDPTypeGoogle, OUID: "ou-owner"},
		(*tidcommon.ServiceError)(nil))
	request := sharing.PolicyRequest{ChildUnits: &sharing.ChildUnitsRequest{Mode: sharing.ChildUnitsAll}}
	s.mockSharing.On("ExportPolicies", mock.Anything, ConnectionSharingType, "g-1").Return(
		[]sharing.ReplayablePolicy{{Request: request}}, (*tidcommon.ServiceError)(nil))
	exporter := newConnectionExporter(s.mockIDP, s.mockNotif, s.mockPDP, s.mockHTTP)
	exporter.sharingService = s.mockSharing

	resource, name, svcErr := exporter.GetResourceByID(context.Background(), "g-1")

	s.Nil(svcErr)
	s.Equal("Google", name)
	model := resource.(*connectionExportModel)
	s.Require().Len(model.SharingPolicies, 1)
	s.Equal(request, model.SharingPolicies[0])
}

func (s *SharingTestSuite) TestExportFailsWhenPoliciesCannotBeRead() {
	s.mockIDP.On("GetIdentityProvider", mock.Anything, "g-1").Return(
		&providers.IDPDTO{ID: "g-1", Name: "Google", Type: providers.IDPTypeGoogle, OUID: "ou-owner"},
		(*tidcommon.ServiceError)(nil))
	s.mockSharing.On("ExportPolicies", mock.Anything, ConnectionSharingType, "g-1").
		Return(nil, &tidcommon.InternalServerError)
	exporter := newConnectionExporter(s.mockIDP, s.mockNotif, s.mockPDP, s.mockHTTP)
	exporter.sharingService = s.mockSharing

	resource, _, svcErr := exporter.GetResourceByID(context.Background(), "g-1")

	s.Nil(resource)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *SharingTestSuite) TestDeclaredPolicyParserReadsOwnerAndPolicies() {
	parse := makeConnectionSharingParser(nil, nil, nil, nil)
	doc := "id: pdp-1\ntype: authzen-pdp\nname: PDP\nendpoint: http://localhost:3592/access/v1/evaluation\n" +
		"ouId: ou-owner\n" +
		"sharingPolicies:\n  - childUnits:\n      mode: all\n"

	declared, err := parse([]byte(doc))

	s.Require().NoError(err)
	s.Equal("pdp-1", declared.ResourceID)
	s.Equal("PDP", declared.ResourceName)
	s.Equal("ou-owner", declared.OwningOUID)
	s.Len(declared.Policies, 1)
}

func (s *SharingTestSuite) TestDeclaredPolicyParserReadsAnHTTPConnection() {
	s.mockHTTP.On("ValidateHTTPConnection", mock.Anything, mock.Anything).Return((*tidcommon.ServiceError)(nil))
	s.mockHTTP.On("ResolveConnectionOU", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { args.Get(1).(*httpconnection.HTTPConnection).OUID = "ou-owner" }).
		Return((*tidcommon.ServiceError)(nil))
	parse := makeConnectionSharingParser(nil, nil, nil, s.mockHTTP)
	doc := "id: http-1\ntype: http\nname: Risk\nurl: https://risk.example.com/evaluate\nouHandle: root\n" +
		"sharingPolicies:\n  - childUnits:\n      mode: all\n"

	declared, err := parse([]byte(doc))

	s.Require().NoError(err)
	s.Equal("http-1", declared.ResourceID)
	s.Equal("Risk", declared.ResourceName)
	s.Equal("ou-owner", declared.OwningOUID)
	s.Len(declared.Policies, 1)
}

func (s *SharingTestSuite) TestExportCarriesAnHTTPConnectionsSharingPolicies() {
	s.idpMissing("http-1")
	s.senderMissing("http-1")
	s.pdpMissing("http-1")
	s.mockHTTP.On("GetHTTPConnection", mock.Anything, "http-1").Return(
		&httpconnection.HTTPConnection{ID: "http-1", Name: "Risk", URL: "https://risk.example.com/evaluate",
			OUID: "ou-owner"}, (*tidcommon.ServiceError)(nil))
	request := sharing.PolicyRequest{ChildUnits: &sharing.ChildUnitsRequest{Mode: sharing.ChildUnitsAll}}
	s.mockSharing.On("ExportPolicies", mock.Anything, ConnectionSharingType, "http-1").Return(
		[]sharing.ReplayablePolicy{{Request: request}}, (*tidcommon.ServiceError)(nil))
	exporter := newConnectionExporter(s.mockIDP, s.mockNotif, s.mockPDP, s.mockHTTP)
	exporter.sharingService = s.mockSharing

	resource, name, svcErr := exporter.GetResourceByID(context.Background(), "http-1")

	s.Nil(svcErr)
	s.Equal("Risk", name)
	model := resource.(*connectionExportModel)
	s.Require().Len(model.SharingPolicies, 1)
	s.Equal(request, model.SharingPolicies[0])
}

func (s *SharingTestSuite) TestWriterResolvesAnHTTPConnectionsOwner() {
	s.mockHTTP.On("ResolveConnectionOU", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { args.Get(1).(*httpconnection.HTTPConnection).OUID = "ou-owner" }).
		Return((*tidcommon.ServiceError)(nil))
	connection := &httpconnection.HTTPConnection{OUHandle: "root"}

	svcErr := (&httpConnectionWriter{svc: s.svc}).ResolveConnectionOU(context.Background(), connection)

	s.Nil(svcErr)
	s.Equal("ou-owner", connection.OUID)
}

func (s *SharingTestSuite) TestDeclaredPolicyParserSkipsDocumentsWithoutPolicies() {
	parse := makeConnectionSharingParser(nil, nil, nil, nil)

	declared, err := parse([]byte("id: g-1\ntype: google\nname: Google\n"))

	s.Require().NoError(err)
	s.Equal("g-1", declared.ResourceID)
	s.Empty(declared.Policies)
}

func (s *SharingTestSuite) TestDeclaredPolicyParserRefusesConnectionWithoutOwner() {
	parse := makeConnectionSharingParser(nil, nil, nil, nil)
	doc := "id: pdp-1\ntype: authzen-pdp\nname: PDP\nendpoint: http://localhost:3592/access/v1/evaluation\n" +
		"sharingPolicies:\n  - childUnits:\n      mode: all\n"

	_, err := parse([]byte(doc))

	s.ErrorContains(err, "needs an organization unit")
}

func (s *SharingTestSuite) TestParseConnectionSharingPolicies() {
	var node yaml.Node
	s.Require().NoError(yaml.Unmarshal(
		[]byte("id: g-1\ntype: google\nsharingPolicies:\n  - childUnits:\n      mode: all\n"), &node))

	policies, err := ParseConnectionSharingPolicies(node.Content[0])

	s.Require().NoError(err)
	s.Len(policies, 1)
}

// listConnections stubs every kind of connection with an identity provider per entry and nothing else.
func (s *SharingTestSuite) listConnections(idps []idp.BasicIDPDTO) {
	s.mockIDP.On("GetIdentityProviderList", mock.Anything).Return(idps, (*tidcommon.ServiceError)(nil)).Maybe()
	s.mockNotif.On("ListSendersByType", mock.Anything, mock.Anything).
		Return([]ncommon.NotificationSenderDTO{}, (*tidcommon.ServiceError)(nil)).Maybe()
	s.mockPDP.On("ListAuthZENPDPs", mock.Anything).
		Return([]authzenpdp.AuthZENPDPConnection{}, (*tidcommon.ServiceError)(nil)).Maybe()
	s.mockHTTP.On("ListHTTPConnections", mock.Anything).
		Return([]httpconnection.HTTPConnection{}, (*tidcommon.ServiceError)(nil)).Maybe()
}

func (s *SharingTestSuite) TestListInstancesInAnOrganizationUnitListsOwnedThenShared() {
	s.listConnections([]idp.BasicIDPDTO{
		{ID: "shared-2", Name: "A", Type: providers.IDPTypeGoogle, OUID: "owner-ou"},
		{ID: "mine-b", Name: "B", Type: providers.IDPTypeGoogle, OUID: "ou-1"},
		{ID: "shared-1", Name: "Z", Type: providers.IDPTypeGoogle, OUID: "owner-ou"},
		{ID: "mine-a", Name: "A", Type: providers.IDPTypeGoogle, OUID: "ou-1"},
		{ID: "elsewhere", Name: "E", Type: providers.IDPTypeGoogle, OUID: "other-ou"},
	})
	s.mockSharing.On("ListVisibleResourceIDs", mock.Anything, ConnectionSharingType, "ou-1").
		Return([]string{"gone", "shared-1", "shared-2"}, (*tidcommon.ServiceError)(nil))

	got, svcErr := s.svc.listInstances(context.Background(), "", "ou-1", 3, 1)

	s.Require().Nil(svcErr)
	s.Equal(4, got.TotalResults, "a shared connection that no longer exists is not counted")
	s.Equal(2, got.StartIndex)
	s.Require().Len(got.Connections, 3)
	s.Equal("mine-b", got.Connections[0].ID, "owned in listing order: mine-a is on the first page")
	s.True(*got.Connections[0].IsOwnedByOU)
	s.Equal("shared-1", got.Connections[1].ID, "then shared in ID order")
	s.False(*got.Connections[1].IsOwnedByOU)
	s.Equal("owner-ou", got.Connections[1].OUID, "the owner's, not the one listed")
	s.Equal("shared-2", got.Connections[2].ID)
	s.Require().NotEmpty(got.Links)
	s.Contains(got.Links[0].Href, "filter=ouId+eq+%22ou-1%22", "the links keep the filter")
}

func (s *SharingTestSuite) TestListInstancesInAnOrganizationUnitKeepsTheCategory() {
	s.listConnections([]idp.BasicIDPDTO{{ID: "mine", Type: providers.IDPTypeGoogle, OUID: "ou-1"}})
	s.mockSharing.On("ListVisibleResourceIDs", mock.Anything, ConnectionSharingType, "ou-1").
		Return([]string{}, (*tidcommon.ServiceError)(nil))

	got, svcErr := s.svc.listInstances(context.Background(), categoryIdentityProvider, "ou-1", 1, 0)

	s.Require().Nil(svcErr)
	s.Require().Len(got.Connections, 1)
	s.Equal("mine", got.Connections[0].ID)
}

func (s *SharingTestSuite) TestListInstancesWithoutAnOrganizationUnitMarksEveryConnectionOwned() {
	s.listConnections([]idp.BasicIDPDTO{{ID: "a", Type: providers.IDPTypeGoogle, OUID: "ou-1"}})

	got, svcErr := s.svc.listInstances(context.Background(), "", "", serverconst.DefaultPageSize, 0)

	s.Require().Nil(svcErr)
	s.Require().Len(got.Connections, 1)
	s.True(*got.Connections[0].IsOwnedByOU)
}

func (s *SharingTestSuite) TestListInstancesInAnOrganizationUnitFailsWhenSharingCannotBeRead() {
	s.listConnections([]idp.BasicIDPDTO{})
	s.mockSharing.On("ListVisibleResourceIDs", mock.Anything, ConnectionSharingType, "ou-1").
		Return(([]string)(nil), &tidcommon.InternalServerError)

	_, svcErr := s.svc.listInstances(context.Background(), "", "ou-1", serverconst.DefaultPageSize, 0)

	s.Require().NotNil(svcErr)
}
