// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/connection/authzenpdp"
	"github.com/thunder-id/thunderid/internal/connection/httpconnection"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/notification"
	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/connection/authzenpdpmock"
	"github.com/thunder-id/thunderid/tests/mocks/connection/httpconnectionmock"
	"github.com/thunder-id/thunderid/tests/mocks/idp/idpmock"
	"github.com/thunder-id/thunderid/tests/mocks/notification/notificationmock"
)

type ServiceTestSuite struct {
	suite.Suite
	svc       *service
	mockIDP   *idpmock.IDPServiceInterfaceMock
	mockNotif *notificationmock.NotificationSenderMgtSvcInterfaceMock
}

type testResourceServerLister struct {
	resource.ResourceServiceInterface
	lists  map[int]*resource.ResourceServerList
	err    *tidcommon.ServiceError
	called []int
}

func (l *testResourceServerLister) GetResourceServerList(
	_ context.Context,
	_ int,
	offset int,
) (*resource.ResourceServerList, *tidcommon.ServiceError) {
	l.called = append(l.called, offset)
	if l.err != nil {
		return nil, l.err
	}
	if list, ok := l.lists[offset]; ok {
		return list, nil
	}
	return &resource.ResourceServerList{}, nil
}

func TestServiceSuite(t *testing.T) {
	suite.Run(t, new(ServiceTestSuite))
}

func (s *ServiceTestSuite) SetupTest() {
	initConfigWithTestCryptoKey(s.T())
	s.mockIDP = idpmock.NewIDPServiceInterfaceMock(s.T())
	s.mockNotif = notificationmock.NewNotificationSenderMgtSvcInterfaceMock(s.T())
	s.svc = newService(s.mockIDP, s.mockNotif, &testResourceServerLister{},
		newAuthZENPDPServiceMock(s.T()), newHTTPConnectionServiceMock(s.T()))
}

func (s *ServiceTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (s *ServiceTestSuite) clientSecret(value string) []cmodels.Property {
	return []cmodels.Property{mustProperty(s.T(), idp.PropClientSecret, value, true)}
}

func (s *ServiceTestSuite) TestListByTypeFilters() {
	s.mockIDP.On("GetIdentityProviderList", mock.Anything).Return([]idp.BasicIDPDTO{
		{ID: "1", Type: providers.IDPTypeGoogle},
		{ID: "2", Type: providers.IDPTypeOIDC},
		{ID: "3", Type: providers.IDPTypeGoogle},
	}, (*tidcommon.ServiceError)(nil))

	got, svcErr := s.svc.listByType(context.Background(), providers.IDPTypeGoogle)
	s.Nil(svcErr)
	s.Len(got, 2)
}

func (s *ServiceTestSuite) TestListByTypeError() {
	s.mockIDP.On("GetIdentityProviderList", mock.Anything).
		Return(([]idp.BasicIDPDTO)(nil), &tidcommon.InternalServerError)

	_, svcErr := s.svc.listByType(context.Background(), providers.IDPTypeGoogle)
	s.NotNil(svcErr)
}

func (s *ServiceTestSuite) TestCreateAuthZENPDPStoresConfiguredEndpoints() {
	pdpService := authzenpdpmock.NewAuthZENPDPServiceInterfaceMock(s.T())
	var createRequest authzenpdp.ConnectionRequest
	pdpService.On("CreateAuthZENPDPConnection", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { createRequest = args.Get(1).(authzenpdp.ConnectionRequest) }).
		Return(&authzenpdp.AuthZENPDPConnection{
			ID: "pdp-1", Name: "PDP",
			Endpoint:      "https://pdp.example.com/access/v1/evaluation",
			BatchEndpoint: "https://pdp.example.com/access/v1/evaluations",
		}, (*tidcommon.ServiceError)(nil))
	s.svc = newService(s.mockIDP, s.mockNotif, &testResourceServerLister{},
		pdpService, newHTTPConnectionServiceMock(s.T()))

	created, svcErr := s.svc.createAuthZENPDP(context.Background(), authzenpdp.ConnectionRequest{
		Name:          "PDP",
		Endpoint:      " https://pdp.example.com/access/v1/evaluation ",
		BatchEndpoint: " https://pdp.example.com/access/v1/evaluations ",
	})

	s.Nil(svcErr)
	s.Require().NotNil(created)
	s.Equal("https://pdp.example.com/access/v1/evaluation", created.Endpoint)
	s.Equal("https://pdp.example.com/access/v1/evaluations", created.BatchEndpoint)
	s.Equal(" https://pdp.example.com/access/v1/evaluation ", createRequest.Endpoint)
	s.Equal(" https://pdp.example.com/access/v1/evaluations ", createRequest.BatchEndpoint)
}

func (s *ServiceTestSuite) TestCreateAuthZENPDPRejectsDuplicateName() {
	pdpService := authzenpdpmock.NewAuthZENPDPServiceInterfaceMock(s.T())
	pdpService.On("CreateAuthZENPDPConnection", mock.Anything, mock.Anything).
		Return((*authzenpdp.AuthZENPDPConnection)(nil), &authzenpdp.ErrorAlreadyExists)
	s.svc = newService(s.mockIDP, s.mockNotif, &testResourceServerLister{},
		pdpService, newHTTPConnectionServiceMock(s.T()))

	created, svcErr := s.svc.createAuthZENPDP(context.Background(), authzenpdp.ConnectionRequest{
		Name:          "PDP",
		Endpoint:      "https://pdp.example.com/access/v1/evaluation",
		BatchEndpoint: "https://pdp.example.com/access/v1/evaluations",
	})

	s.Nil(created)
	s.Equal(authzenpdp.ErrorAlreadyExists.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestUpdateAuthZENPDPStoresConfiguredEndpoints() {
	pdpService := authzenpdpmock.NewAuthZENPDPServiceInterfaceMock(s.T())
	pdpService.On("UpdateAuthZENPDPConnection", mock.Anything, "pdp-1", mock.Anything).
		Return(&authzenpdp.AuthZENPDPConnection{
			ID:            "pdp-1",
			Name:          "New PDP",
			Endpoint:      "https://new-pdp.example.com/access/v1/evaluation",
			BatchEndpoint: "https://new-pdp.example.com/access/v1/evaluations",
		}, (*tidcommon.ServiceError)(nil))
	s.svc = newService(s.mockIDP, s.mockNotif, &testResourceServerLister{},
		pdpService, newHTTPConnectionServiceMock(s.T()))

	updated, svcErr := s.svc.updateAuthZENPDP(context.Background(), "pdp-1", authzenpdp.ConnectionRequest{
		Name:          "New PDP",
		Endpoint:      "https://new-pdp.example.com/access/v1/evaluation",
		BatchEndpoint: "https://new-pdp.example.com/access/v1/evaluations",
	})

	s.Nil(svcErr)
	s.Require().NotNil(updated)
	s.Equal("https://new-pdp.example.com/access/v1/evaluation", updated.Endpoint)
	s.Equal("https://new-pdp.example.com/access/v1/evaluations", updated.BatchEndpoint)
}

func (s *ServiceTestSuite) TestUpdateAuthZENPDPPropagatesDomainError() {
	pdpService := authzenpdpmock.NewAuthZENPDPServiceInterfaceMock(s.T())
	pdpService.On("UpdateAuthZENPDPConnection", mock.Anything, "pdp-1", mock.Anything).
		Return((*authzenpdp.AuthZENPDPConnection)(nil), &authzenpdp.ErrorAlreadyExists)
	s.svc = newService(s.mockIDP, s.mockNotif, &testResourceServerLister{},
		pdpService, newHTTPConnectionServiceMock(s.T()))

	updated, svcErr := s.svc.updateAuthZENPDP(context.Background(), "pdp-1", authzenpdp.ConnectionRequest{})

	s.Nil(updated)
	s.Equal(authzenpdp.ErrorAlreadyExists.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestListInstancesAllCategories() {
	s.mockIDP.On("GetIdentityProviderList", mock.Anything).Return([]idp.BasicIDPDTO{
		{ID: "1", Name: "google B", Type: providers.IDPTypeGoogle},
		{ID: "2", Name: "Google A", Type: providers.IDPTypeGoogle},
		{ID: "3", Name: "Legacy", Type: providers.IDPType("SAML")},
	}, (*tidcommon.ServiceError)(nil))
	s.mockNotif.On("ListSendersByType", mock.Anything, ncommon.NotificationSenderTypeMessage).
		Return([]ncommon.NotificationSenderDTO{
			{ID: "s1", Name: "SMS", Type: ncommon.NotificationSenderTypeMessage,
				Provider: ncommon.NotificationProviderTypeCustom},
		}, (*tidcommon.ServiceError)(nil))
	s.mockNotif.On("ListSendersByType", mock.Anything, ncommon.NotificationSenderTypeEmail).
		Return([]ncommon.NotificationSenderDTO{}, (*tidcommon.ServiceError)(nil))

	got, svcErr := s.svc.listInstances(context.Background(), "", serverconst.DefaultPageSize, 0)
	s.Nil(svcErr)
	s.Require().Len(got.Connections, 3) // unknown IdP type skipped; senders are already message-only
	s.Equal(3, got.TotalResults)
	s.Equal(1, got.StartIndex)
	s.Equal(3, got.Count)

	// Sorted by type, then lowercase name, then ID; case-insensitive name ordering.
	// "google" sorts before "sms-gateway" (the custom sender's vendor name).
	s.Equal("2", got.Connections[0].ID) // "Google A" before "google B"
	s.Equal("google", got.Connections[0].Type)
	s.Equal([]connectionCategory{categoryIdentityProvider}, got.Connections[0].Categories)
	s.Equal("1", got.Connections[1].ID)
	s.Equal("google", got.Connections[1].Type)
	s.Equal([]connectionCategory{categoryIdentityProvider}, got.Connections[1].Categories)
	s.Equal("s1", got.Connections[2].ID)
	s.Equal("sms-gateway", got.Connections[2].Type)
	s.Equal([]connectionCategory{categorySMSProvider}, got.Connections[2].Categories)
}

func (s *ServiceTestSuite) TestListInstancesIDJagEnabled() {
	trusted := true
	plainDisabled := false
	s.mockIDP.On("GetIdentityProviderList", mock.Anything).Return([]idp.BasicIDPDTO{
		{ID: "1", Name: "Trusted", Type: providers.IDPTypeOIDC, IDJagEnabled: &trusted},
		{ID: "2", Name: "Disabled", Type: providers.IDPTypeOIDC, IDJagEnabled: &plainDisabled},
		{ID: "3", Name: "Plain", Type: providers.IDPTypeOIDC},
	}, (*tidcommon.ServiceError)(nil))

	got, svcErr := s.svc.listInstances(context.Background(), categoryIdentityProvider,
		serverconst.DefaultPageSize, 0)
	s.Nil(svcErr)
	s.Require().Len(got.Connections, 3)

	byID := make(map[string]connectionInstance, len(got.Connections))
	for _, c := range got.Connections {
		byID[c.ID] = c
	}

	s.Require().NotNil(byID["1"].IDJagEnabled)
	s.True(*byID["1"].IDJagEnabled)
	s.Require().NotNil(byID["2"].IDJagEnabled)
	s.False(*byID["2"].IDJagEnabled)
	s.Nil(byID["3"].IDJagEnabled)
}

func (s *ServiceTestSuite) TestListInstancesPaginates() {
	s.mockIDP.On("GetIdentityProviderList", mock.Anything).Return([]idp.BasicIDPDTO{
		{ID: "1", Name: "A", Type: providers.IDPTypeGoogle},
		{ID: "2", Name: "B", Type: providers.IDPTypeGoogle},
		{ID: "3", Name: "C", Type: providers.IDPTypeGoogle},
	}, (*tidcommon.ServiceError)(nil))

	got, svcErr := s.svc.listInstances(context.Background(), categoryIdentityProvider, 1, 1)
	s.Nil(svcErr)
	s.Equal(3, got.TotalResults)
	s.Equal(2, got.StartIndex)
	s.Equal(1, got.Count)
	s.Require().Len(got.Connections, 1)
	s.Equal("2", got.Connections[0].ID)

	// Page links carry the category filter.
	s.Require().NotEmpty(got.Links)
	for _, link := range got.Links {
		s.Contains(link.Href, "category=identity-provider")
	}
}

func (s *ServiceTestSuite) TestListInstancesOffsetPastEnd() {
	s.mockIDP.On("GetIdentityProviderList", mock.Anything).Return([]idp.BasicIDPDTO{
		{ID: "1", Name: "A", Type: providers.IDPTypeGoogle},
	}, (*tidcommon.ServiceError)(nil))
	s.mockNotif.On("ListSendersByType", mock.Anything, ncommon.NotificationSenderTypeMessage).
		Return([]ncommon.NotificationSenderDTO{}, (*tidcommon.ServiceError)(nil))
	s.mockNotif.On("ListSendersByType", mock.Anything, ncommon.NotificationSenderTypeEmail).
		Return([]ncommon.NotificationSenderDTO{}, (*tidcommon.ServiceError)(nil))

	got, svcErr := s.svc.listInstances(context.Background(), "", serverconst.DefaultPageSize, 10)
	s.Nil(svcErr)
	s.Equal(1, got.TotalResults)
	s.Equal(11, got.StartIndex)
	s.Equal(0, got.Count)
	s.NotNil(got.Connections)
	s.Empty(got.Connections)
}

func (s *ServiceTestSuite) TestListInstancesInvalidPagination() {
	cases := []struct{ limit, offset int }{
		{0, 0}, {-1, 0}, {serverconst.MaxPageSize + 1, 0}, {10, -1},
	}
	for _, tc := range cases {
		_, svcErr := s.svc.listInstances(context.Background(), "", tc.limit, tc.offset)
		s.Require().NotNil(svcErr, "limit=%d offset=%d", tc.limit, tc.offset)
	}
	s.mockIDP.AssertNotCalled(s.T(), "GetIdentityProviderList", mock.Anything)
	s.mockNotif.AssertNotCalled(s.T(), "ListSendersByType", mock.Anything, ncommon.NotificationSenderTypeMessage)
}

func (s *ServiceTestSuite) TestListInstancesIdentityProviderSkipsSenders() {
	s.mockIDP.On("GetIdentityProviderList", mock.Anything).Return([]idp.BasicIDPDTO{
		{ID: "1", Name: "G", Type: providers.IDPTypeGoogle},
	}, (*tidcommon.ServiceError)(nil))

	got, svcErr := s.svc.listInstances(context.Background(), categoryIdentityProvider,
		serverconst.DefaultPageSize, 0)
	s.Nil(svcErr)
	s.Len(got.Connections, 1)
	s.mockNotif.AssertNotCalled(s.T(), "ListSendersByType", mock.Anything, ncommon.NotificationSenderTypeMessage)
}

func (s *ServiceTestSuite) TestListInstancesSMSSkipsIdPs() {
	s.mockNotif.On("ListSendersByType", mock.Anything, ncommon.NotificationSenderTypeMessage).
		Return([]ncommon.NotificationSenderDTO{
			{ID: "s1", Name: "SMS", Type: ncommon.NotificationSenderTypeMessage,
				Provider: ncommon.NotificationProviderTypeTwilio},
		}, (*tidcommon.ServiceError)(nil))

	got, svcErr := s.svc.listInstances(context.Background(), categorySMSProvider,
		serverconst.DefaultPageSize, 0)
	s.Nil(svcErr)
	s.Require().Len(got.Connections, 1)
	s.Equal("s1", got.Connections[0].ID)
	s.mockIDP.AssertNotCalled(s.T(), "GetIdentityProviderList", mock.Anything)
}

func (s *ServiceTestSuite) TestListInstancesSkipsUnregisteredSenderProvider() {
	s.mockNotif.On("ListSendersByType", mock.Anything, ncommon.NotificationSenderTypeMessage).
		Return([]ncommon.NotificationSenderDTO{
			{ID: "s1", Name: "SMS", Type: ncommon.NotificationSenderTypeMessage,
				Provider: ncommon.NotificationProviderTypeTwilio},
			{ID: "s2", Name: "Unregistered", Type: ncommon.NotificationSenderTypeMessage,
				Provider: ncommon.NotificationProviderType("unregistered-provider")},
		}, (*tidcommon.ServiceError)(nil))

	got, svcErr := s.svc.listInstances(context.Background(), categorySMSProvider,
		serverconst.DefaultPageSize, 0)
	s.Nil(svcErr)
	s.Require().Len(got.Connections, 1)
	s.Equal("s1", got.Connections[0].ID)
}

func (s *ServiceTestSuite) TestListInstancesSortsByIDWhenTypeAndNameTie() {
	s.mockIDP.On("GetIdentityProviderList", mock.Anything).Return([]idp.BasicIDPDTO{
		{ID: "2", Name: "Same Name", Type: providers.IDPTypeGoogle},
		{ID: "1", Name: "Same Name", Type: providers.IDPTypeGoogle},
	}, (*tidcommon.ServiceError)(nil))
	s.mockNotif.On("ListSendersByType", mock.Anything, ncommon.NotificationSenderTypeMessage).
		Return([]ncommon.NotificationSenderDTO{}, (*tidcommon.ServiceError)(nil))
	s.mockNotif.On("ListSendersByType", mock.Anything, ncommon.NotificationSenderTypeEmail).
		Return([]ncommon.NotificationSenderDTO{}, (*tidcommon.ServiceError)(nil))

	got, svcErr := s.svc.listInstances(context.Background(), "", serverconst.DefaultPageSize, 0)
	s.Nil(svcErr)
	s.Require().Len(got.Connections, 2)
	s.Equal("1", got.Connections[0].ID)
	s.Equal("2", got.Connections[1].ID)
}

func (s *ServiceTestSuite) TestListInstancesError() {
	s.mockIDP.On("GetIdentityProviderList", mock.Anything).
		Return(([]idp.BasicIDPDTO)(nil), &tidcommon.InternalServerError)

	_, svcErr := s.svc.listInstances(context.Background(), "", serverconst.DefaultPageSize, 0)
	s.NotNil(svcErr)
}

func (s *ServiceTestSuite) TestGetByTypeReturnsMatch() {
	s.mockIDP.On("GetIdentityProvider", mock.Anything, "g-1").
		Return(&providers.IDPDTO{ID: "g-1", Type: providers.IDPTypeGoogle}, (*tidcommon.ServiceError)(nil))

	got, svcErr := s.svc.getByType(context.Background(), providers.IDPTypeGoogle, "g-1")
	s.Nil(svcErr)
	s.Equal("g-1", got.ID)
}

func (s *ServiceTestSuite) TestGetByTypeMismatchReturnsNotFound() {
	s.mockIDP.On("GetIdentityProvider", mock.Anything, "x").
		Return(&providers.IDPDTO{ID: "x", Type: providers.IDPTypeGitHub}, (*tidcommon.ServiceError)(nil))

	_, svcErr := s.svc.getByType(context.Background(), providers.IDPTypeGoogle, "x")
	s.Require().NotNil(svcErr)
	s.Equal(idp.ErrorIDPNotFound.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestGetByTypeNotFound() {
	s.mockIDP.On("GetIdentityProvider", mock.Anything, "missing").
		Return((*providers.IDPDTO)(nil), &idp.ErrorIDPNotFound)

	_, svcErr := s.svc.getByType(context.Background(), providers.IDPTypeGoogle, "missing")
	s.Require().NotNil(svcErr)
	s.Equal(idp.ErrorIDPNotFound.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestUpdateOmittedSecretKeepsStored() {
	s.mockIDP.On("GetIdentityProvider", mock.Anything, "g-1").
		Return(&providers.IDPDTO{ID: "g-1", Type: providers.IDPTypeGoogle, Properties: s.clientSecret("stored")},
			(*tidcommon.ServiceError)(nil))

	var captured *providers.IDPDTO
	s.mockIDP.On("UpdateIdentityProvider", mock.Anything, "g-1", mock.Anything).
		Run(func(args mock.Arguments) { captured = args.Get(2).(*providers.IDPDTO) }).
		Return(&providers.IDPDTO{ID: "g-1", Type: providers.IDPTypeGoogle}, (*tidcommon.ServiceError)(nil))

	// Update carries no secret property at all → the stored secret is preserved.
	dto := &providers.IDPDTO{Name: "g", Type: providers.IDPTypeGoogle, Properties: nil}
	_, svcErr := s.svc.update(context.Background(), providers.IDPTypeGoogle, "g-1", dto)

	s.Nil(svcErr)
	s.Require().NotNil(captured)
	s.Require().Len(captured.Properties, 1)
	v, err := captured.Properties[0].GetValue()
	s.NoError(err)
	s.Equal("stored", v)
}

func (s *ServiceTestSuite) TestUpdateKeepsNewSecret() {
	s.mockIDP.On("GetIdentityProvider", mock.Anything, "g-1").
		Return(&providers.IDPDTO{ID: "g-1", Type: providers.IDPTypeGoogle, Properties: s.clientSecret("stored")},
			(*tidcommon.ServiceError)(nil))

	var captured *providers.IDPDTO
	s.mockIDP.On("UpdateIdentityProvider", mock.Anything, "g-1", mock.Anything).
		Run(func(args mock.Arguments) { captured = args.Get(2).(*providers.IDPDTO) }).
		Return(&providers.IDPDTO{}, (*tidcommon.ServiceError)(nil))

	dto := &providers.IDPDTO{Name: "g", Type: providers.IDPTypeGoogle, Properties: s.clientSecret("brand-new")}
	_, svcErr := s.svc.update(context.Background(), providers.IDPTypeGoogle, "g-1", dto)

	s.Nil(svcErr)
	s.Require().NotNil(captured)
	v, err := captured.Properties[0].GetValue()
	s.NoError(err)
	s.Equal("brand-new", v)
}

func (s *ServiceTestSuite) TestUpdateTypeMismatch() {
	s.mockIDP.On("GetIdentityProvider", mock.Anything, "x").
		Return(&providers.IDPDTO{ID: "x", Type: providers.IDPTypeGitHub}, (*tidcommon.ServiceError)(nil))

	dto := &providers.IDPDTO{Type: providers.IDPTypeGoogle}
	_, svcErr := s.svc.update(context.Background(), providers.IDPTypeGoogle, "x", dto)
	s.Require().NotNil(svcErr)
	s.Equal(idp.ErrorIDPNotFound.Code, svcErr.Code)
	s.mockIDP.AssertNotCalled(s.T(), "UpdateIdentityProvider", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestDeleteByTypeDelegates() {
	s.mockIDP.On("GetIdentityProvider", mock.Anything, "g-1").
		Return(&providers.IDPDTO{ID: "g-1", Type: providers.IDPTypeGoogle}, (*tidcommon.ServiceError)(nil))
	s.mockIDP.On("DeleteIdentityProvider", mock.Anything, "g-1").Return((*tidcommon.ServiceError)(nil))

	svcErr := s.svc.deleteByType(context.Background(), providers.IDPTypeGoogle, "g-1")
	s.Nil(svcErr)
}

func (s *ServiceTestSuite) TestDeleteByTypeGetFails() {
	s.mockIDP.On("GetIdentityProvider", mock.Anything, "missing").
		Return((*providers.IDPDTO)(nil), &idp.ErrorIDPNotFound)

	svcErr := s.svc.deleteByType(context.Background(), providers.IDPTypeGoogle, "missing")
	s.Require().NotNil(svcErr)
	s.mockIDP.AssertNotCalled(s.T(), "DeleteIdentityProvider", mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) authToken(value string) []cmodels.Property {
	return []cmodels.Property{mustProperty(s.T(), ncommon.TwilioPropKeyAuthToken, value, true)}
}

func (s *ServiceTestSuite) TestListSMSByProviderFilters() {
	s.mockNotif.On("ListSendersByType", mock.Anything, ncommon.NotificationSenderTypeMessage).
		Return([]ncommon.NotificationSenderDTO{
			{ID: "1", Type: ncommon.NotificationSenderTypeMessage, Provider: ncommon.NotificationProviderTypeTwilio},
			{ID: "2", Type: ncommon.NotificationSenderTypeMessage, Provider: ncommon.NotificationProviderTypeVonage},
			{ID: "3", Type: ncommon.NotificationSenderTypeMessage, Provider: ncommon.NotificationProviderTypeTwilio},
		}, (*tidcommon.ServiceError)(nil))

	got, svcErr := s.svc.listSendersByProvider(context.Background(), ncommon.NotificationSenderTypeMessage,
		ncommon.NotificationProviderTypeTwilio)
	s.Nil(svcErr)
	s.Len(got, 2)
}

func (s *ServiceTestSuite) TestListSMSByProviderError() {
	s.mockNotif.On("ListSendersByType", mock.Anything, ncommon.NotificationSenderTypeMessage).
		Return(([]ncommon.NotificationSenderDTO)(nil), &tidcommon.InternalServerError)

	_, svcErr := s.svc.listSendersByProvider(context.Background(), ncommon.NotificationSenderTypeMessage,
		ncommon.NotificationProviderTypeTwilio)
	s.NotNil(svcErr)
}

func (s *ServiceTestSuite) TestGetSMSByProviderMismatchReturnsNotFound() {
	s.mockNotif.On("GetSender", mock.Anything, "x").Return(&ncommon.NotificationSenderDTO{
		ID: "x", Type: ncommon.NotificationSenderTypeMessage, Provider: ncommon.NotificationProviderTypeVonage,
	}, (*tidcommon.ServiceError)(nil))

	_, svcErr := s.svc.getSenderByProvider(context.Background(), ncommon.NotificationSenderTypeMessage,
		ncommon.NotificationProviderTypeTwilio, "x")
	s.Require().NotNil(svcErr)
	s.Equal(notification.ErrorSenderNotFound.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestGetSMSByProviderError() {
	s.mockNotif.On("GetSender", mock.Anything, "missing").
		Return((*ncommon.NotificationSenderDTO)(nil), &notification.ErrorSenderNotFound)

	_, svcErr := s.svc.getSenderByProvider(context.Background(), ncommon.NotificationSenderTypeMessage,
		ncommon.NotificationProviderTypeTwilio, "missing")
	s.Require().NotNil(svcErr)
	s.Equal(notification.ErrorSenderNotFound.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestDeleteSMSByProviderMissingIsNoOp() {
	s.mockNotif.On("GetSender", mock.Anything, "missing").
		Return((*ncommon.NotificationSenderDTO)(nil), &notification.ErrorSenderNotFound)

	svcErr := s.svc.deleteSenderByProvider(context.Background(), ncommon.NotificationSenderTypeMessage,
		ncommon.NotificationProviderTypeTwilio, "missing")
	s.Nil(svcErr)
	s.mockNotif.AssertNotCalled(s.T(), "DeleteSender", mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestDeleteSMSByProviderGetFails() {
	s.mockNotif.On("GetSender", mock.Anything, "tw-1").
		Return((*ncommon.NotificationSenderDTO)(nil), &tidcommon.InternalServerError)

	svcErr := s.svc.deleteSenderByProvider(context.Background(), ncommon.NotificationSenderTypeMessage,
		ncommon.NotificationProviderTypeTwilio, "tw-1")
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	s.mockNotif.AssertNotCalled(s.T(), "DeleteSender", mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestDeleteSMSByProviderMismatch() {
	s.mockNotif.On("GetSender", mock.Anything, "x").Return(&ncommon.NotificationSenderDTO{
		ID: "x", Type: ncommon.NotificationSenderTypeMessage, Provider: ncommon.NotificationProviderTypeVonage,
	}, (*tidcommon.ServiceError)(nil))

	svcErr := s.svc.deleteSenderByProvider(context.Background(), ncommon.NotificationSenderTypeMessage,
		ncommon.NotificationProviderTypeTwilio, "x")
	s.Require().NotNil(svcErr)
	s.Equal(notification.ErrorSenderNotFound.Code, svcErr.Code)
	s.mockNotif.AssertNotCalled(s.T(), "DeleteSender", mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestUpdateSMSOmittedSecretKeepsStored() {
	s.mockNotif.On("GetSender", mock.Anything, "tw-1").Return(&ncommon.NotificationSenderDTO{
		ID: "tw-1", Type: ncommon.NotificationSenderTypeMessage, Provider: ncommon.NotificationProviderTypeTwilio,
		Properties: s.authToken("stored"),
	}, (*tidcommon.ServiceError)(nil))

	var captured ncommon.NotificationSenderDTO
	s.mockNotif.On("UpdateSender", mock.Anything, "tw-1", mock.Anything).
		Run(func(args mock.Arguments) { captured = args.Get(2).(ncommon.NotificationSenderDTO) }).
		Return(&ncommon.NotificationSenderDTO{ID: "tw-1"}, (*tidcommon.ServiceError)(nil))

	// Update carries no secret property at all → the stored secret is preserved.
	dto := ncommon.NotificationSenderDTO{
		Name: "tw", Type: ncommon.NotificationSenderTypeMessage, Provider: ncommon.NotificationProviderTypeTwilio,
	}
	_, svcErr := s.svc.updateSender(context.Background(), ncommon.NotificationSenderTypeMessage,
		ncommon.NotificationProviderTypeTwilio, "tw-1", dto)

	s.Nil(svcErr)
	s.Require().Len(captured.Properties, 1)
	v, err := captured.Properties[0].GetValue()
	s.NoError(err)
	s.Equal("stored", v)
}

func (s *ServiceTestSuite) TestUpdateSMSProviderMismatch() {
	s.mockNotif.On("GetSender", mock.Anything, "x").Return(&ncommon.NotificationSenderDTO{
		ID: "x", Type: ncommon.NotificationSenderTypeMessage, Provider: ncommon.NotificationProviderTypeVonage,
	}, (*tidcommon.ServiceError)(nil))

	dto := ncommon.NotificationSenderDTO{
		Type: ncommon.NotificationSenderTypeMessage, Provider: ncommon.NotificationProviderTypeTwilio,
	}
	_, svcErr := s.svc.updateSender(context.Background(), ncommon.NotificationSenderTypeMessage,
		ncommon.NotificationProviderTypeTwilio, "x", dto)
	s.Require().NotNil(svcErr)
	s.Equal(notification.ErrorSenderNotFound.Code, svcErr.Code)
	s.mockNotif.AssertNotCalled(s.T(), "UpdateSender", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestDeleteSMSByProviderDelegates() {
	s.mockNotif.On("GetSender", mock.Anything, "tw-1").Return(&ncommon.NotificationSenderDTO{
		ID: "tw-1", Type: ncommon.NotificationSenderTypeMessage, Provider: ncommon.NotificationProviderTypeTwilio,
	}, (*tidcommon.ServiceError)(nil))
	s.mockNotif.On("DeleteSender", mock.Anything, "tw-1").Return((*tidcommon.ServiceError)(nil))

	svcErr := s.svc.deleteSenderByProvider(context.Background(), ncommon.NotificationSenderTypeMessage,
		ncommon.NotificationProviderTypeTwilio, "tw-1")
	s.Nil(svcErr)
}

func (s *ServiceTestSuite) TestUsagesByTypeDelegates() {
	total := 1
	usages := &resourcedependency.DependenciesResponse{
		TotalResults: &total,
		Count:        1,
		Summary:      map[string]int{"flow": 1},
		Usages: []resourcedependency.ResourceDependency{
			{ResourceType: "flow", ID: "flow-1", DisplayName: "Login Flow", BehaviorOnDelete: "restrict"},
		},
	}
	s.mockIDP.On("GetIdentityProvider", mock.Anything, "g-1").
		Return(&providers.IDPDTO{ID: "g-1", Type: providers.IDPTypeGoogle}, (*tidcommon.ServiceError)(nil))
	s.mockIDP.On("GetIDPUsages", mock.Anything, "g-1").Return(usages, (*tidcommon.ServiceError)(nil))

	result, svcErr := s.svc.usagesByType(context.Background(), providers.IDPTypeGoogle, "g-1")
	s.Nil(svcErr)
	s.Equal(usages, result)
}

func (s *ServiceTestSuite) TestUsagesByTypeGetFails() {
	s.mockIDP.On("GetIdentityProvider", mock.Anything, "missing").
		Return((*providers.IDPDTO)(nil), &idp.ErrorIDPNotFound)

	result, svcErr := s.svc.usagesByType(context.Background(), providers.IDPTypeGoogle, "missing")
	s.Require().NotNil(svcErr)
	s.Nil(result)
	s.mockIDP.AssertNotCalled(s.T(), "GetIDPUsages", mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestUsagesSMSByProviderDelegates() {
	total := 1
	usages := &resourcedependency.DependenciesResponse{
		TotalResults: &total,
		Count:        1,
		Summary:      map[string]int{"flow": 1},
		Usages: []resourcedependency.ResourceDependency{
			{ResourceType: "flow", ID: "flow-1", DisplayName: "SMS OTP", BehaviorOnDelete: "restrict"},
		},
	}
	s.mockNotif.On("GetSender", mock.Anything, "tw-1").Return(&ncommon.NotificationSenderDTO{
		ID: "tw-1", Type: ncommon.NotificationSenderTypeMessage, Provider: ncommon.NotificationProviderTypeTwilio,
	}, (*tidcommon.ServiceError)(nil))
	s.mockNotif.On("GetSenderUsages", mock.Anything, "tw-1").Return(usages, (*tidcommon.ServiceError)(nil))

	result, svcErr := s.svc.usagesSenderByProvider(context.Background(), ncommon.NotificationSenderTypeMessage,
		ncommon.NotificationProviderTypeTwilio, "tw-1")
	s.Nil(svcErr)
	s.Equal(usages, result)
}

func (s *ServiceTestSuite) TestUsagesAuthZENPDPReturnsReferencingResourceServers() {
	pdpService := authzenpdpmock.NewAuthZENPDPServiceInterfaceMock(s.T())
	pdpService.On("GetAuthZENPDP", mock.Anything, "pdp-1").Return(&authzenpdp.AuthZENPDPConnection{
		ID:            "pdp-1",
		Name:          "PDP",
		Endpoint:      "https://pdp.example.com/access/v1/evaluation",
		BatchEndpoint: "https://pdp.example.com/access/v1/evaluations",
	}, (*tidcommon.ServiceError)(nil))
	resourceLister := &testResourceServerLister{
		lists: map[int]*resource.ResourceServerList{
			0: {
				TotalResults: 2,
				Count:        2,
				ResourceServers: []providers.ResourceServer{
					{
						ID:   "rs-1",
						Name: "Travel API",
						AuthorizationEngine: providers.AuthorizationEngineConfig{
							Type: providers.AuthorizationEngineTypeAuthZENPDP,
							Properties: providers.AuthorizationEngineProperties{
								PDPConnectionID: " pdp-1 ",
							},
						},
					},
					{
						ID:   "rs-2",
						Name: "Billing API",
						AuthorizationEngine: providers.AuthorizationEngineConfig{
							Type: providers.AuthorizationEngineTypeAuthZENPDP,
							Properties: providers.AuthorizationEngineProperties{
								PDPConnectionID: "other",
							},
						},
					},
				},
			},
		},
	}
	s.svc = newService(s.mockIDP, s.mockNotif, resourceLister,
		pdpService, newHTTPConnectionServiceMock(s.T()))

	result, svcErr := s.svc.usagesAuthZENPDP(context.Background(), "pdp-1")

	s.Nil(svcErr)
	s.Require().NotNil(result.TotalResults)
	s.Equal(1, *result.TotalResults)
	s.Equal(1, result.Count)
	s.Equal(1, result.Summary[resourcedependency.ResourceTypeResourceServer])
	s.Require().Len(result.Usages, 1)
	s.Equal("rs-1", result.Usages[0].ID)
	s.Equal("Travel API", result.Usages[0].DisplayName)
	s.Equal(resourcedependency.BehaviorRestrict, result.Usages[0].BehaviorOnDelete)
}

func (s *ServiceTestSuite) TestDeleteAuthZENPDPBlocksWhenResourceServerReferencesIt() {
	pdpService := authzenpdpmock.NewAuthZENPDPServiceInterfaceMock(s.T())
	pdpService.On("GetAuthZENPDP", mock.Anything, "pdp-1").Return(&authzenpdp.AuthZENPDPConnection{
		ID:            "pdp-1",
		Name:          "PDP",
		Endpoint:      "https://pdp.example.com/access/v1/evaluation",
		BatchEndpoint: "https://pdp.example.com/access/v1/evaluations",
	}, (*tidcommon.ServiceError)(nil))
	s.svc = newService(s.mockIDP, s.mockNotif, &testResourceServerLister{
		lists: map[int]*resource.ResourceServerList{
			0: {
				TotalResults: 1,
				Count:        1,
				ResourceServers: []providers.ResourceServer{
					{
						ID:   "rs-1",
						Name: "Travel API",
						AuthorizationEngine: providers.AuthorizationEngineConfig{
							Type: providers.AuthorizationEngineTypeAuthZENPDP,
							Properties: providers.AuthorizationEngineProperties{
								PDPConnectionID: " pdp-1 ",
							},
						},
					},
				},
			},
		},
	}, pdpService, newHTTPConnectionServiceMock(s.T()))

	svcErr := s.svc.deleteAuthZENPDP(context.Background(), "pdp-1")

	s.Require().NotNil(svcErr)
	s.Equal(authzenpdp.ErrorHasBlockingDependencies.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestDeleteAuthZENPDPRejectsImmutableConnection() {
	pdpService := authzenpdpmock.NewAuthZENPDPServiceInterfaceMock(s.T())
	pdpService.On("GetAuthZENPDP", mock.Anything, "pdp-1").
		Return(&authzenpdp.AuthZENPDPConnection{ID: "pdp-1", Name: "PDP"}, (*tidcommon.ServiceError)(nil))
	pdpService.On("DeleteAuthZENPDPConnection", mock.Anything, "pdp-1").
		Return(&declarativeresource.ErrorDeclarativeResourceDeleteOperation)
	s.svc = newService(s.mockIDP, s.mockNotif, &testResourceServerLister{}, pdpService,
		newHTTPConnectionServiceMock(s.T()))

	svcErr := s.svc.deleteAuthZENPDP(context.Background(), "pdp-1")

	s.Require().NotNil(svcErr)
	s.Equal(declarativeresource.ErrorDeclarativeResourceDeleteOperation.Code, svcErr.Code)
}

// TestUsagesSMSByProviderWrongProvider verifies a sender of another provider is not exposed
// through a vendor's usages endpoint.
func (s *ServiceTestSuite) TestUsagesSMSByProviderWrongProvider() {
	s.mockNotif.On("GetSender", mock.Anything, "vo-1").Return(&ncommon.NotificationSenderDTO{
		ID: "vo-1", Type: ncommon.NotificationSenderTypeMessage, Provider: ncommon.NotificationProviderTypeVonage,
	}, (*tidcommon.ServiceError)(nil))

	result, svcErr := s.svc.usagesSenderByProvider(context.Background(), ncommon.NotificationSenderTypeMessage,
		ncommon.NotificationProviderTypeTwilio, "vo-1")
	s.Require().NotNil(svcErr)
	s.Nil(result)
	s.mockNotif.AssertNotCalled(s.T(), "GetSenderUsages", mock.Anything, mock.Anything)
}

const storedHTTPURL = "https://risk.example.com/evaluate"

// storedHTTPAuth renders an authentication configuration as the stored, encrypted property bag.
func (s *ServiceTestSuite) storedHTTPAuth(authType outboundauth.Type, values map[string]string) []cmodels.Property {
	props, err := outboundauth.ToProperties(outboundauth.Config{Type: authType, Properties: values})
	s.Require().NoError(err)
	return props
}

func (s *ServiceTestSuite) storedBasic() []cmodels.Property {
	return s.storedHTTPAuth(outboundauth.TypeBasic, map[string]string{
		outboundauth.FieldBasicUsername: "svc-user", outboundauth.FieldBasicPassword: "stored-password"})
}

// updateHTTPWithStored maps request through the update mapping, as the handler and the MCP tool do,
// and updates a connection stored at storedHTTPURL with storedAuth. It returns the authentication
// handed to the HTTP connection service, which is not called when the update is refused.
func (s *ServiceTestSuite) updateHTTPWithStored(storedAuth []cmodels.Property, request httpConnectionRequest) (
	outboundauth.Config, *tidcommon.ServiceError) {
	s.T().Helper()
	mockHTTP := httpconnectionmock.NewHTTPConnectionServiceInterfaceMock(s.T())
	s.svc = newService(s.mockIDP, s.mockNotif, &testResourceServerLister{}, newAuthZENPDPServiceMock(s.T()), mockHTTP)
	mockHTTP.On("GetHTTPConnection", mock.Anything, "http-1").Return(&httpconnection.HTTPConnection{
		ID: "http-1", Name: "Risk", URL: storedHTTPURL, AuthProperties: storedAuth,
	}, (*tidcommon.ServiceError)(nil))
	var received *httpconnection.HTTPConnection
	mockHTTP.On("UpdateHTTPConnection", mock.Anything, "http-1", mock.Anything).
		Run(func(args mock.Arguments) {
			connection := args.Get(2).(httpconnection.HTTPConnection)
			received = &connection
		}).
		Return(&httpconnection.HTTPConnection{ID: "http-1"}, (*tidcommon.ServiceError)(nil)).Maybe()

	request.Name = "Risk"
	dto, err := httpUpdateToDTO(request)
	s.Require().NoError(err)
	_, svcErr := s.svc.updateHTTP(context.Background(), "http-1", *dto)
	if svcErr != nil {
		s.Nil(received, "a refused update must not reach the HTTP connection service")
		return outboundauth.Config{}, svcErr
	}
	s.Require().NotNil(received)
	authConfig, err := outboundauth.FromProperties(received.AuthProperties)
	s.Require().NoError(err)
	return authConfig, nil
}

func (s *ServiceTestSuite) TestUpdateHTTPKeepsStoredPasswordForTheSameTarget() {
	for name, url := range map[string]string{
		"same url":              storedHTTPURL,
		"path change":           "https://risk.example.com/v2/evaluate?x=1",
		"explicit default port": "https://risk.example.com:443/evaluate",
		"host case":             "https://RISK.example.com/evaluate",
	} {
		s.Run(name, func() {
			authConfig, svcErr := s.updateHTTPWithStored(s.storedBasic(), httpConnectionRequest{
				URL: url, Authentication: basicAuthRequest("svc-user", "")})
			s.Require().Nil(svcErr)
			s.Equal("stored-password", authConfig.Get(outboundauth.FieldBasicPassword))
		})
	}
}

func (s *ServiceTestSuite) TestUpdateHTTPRefusesAnOmittedPasswordForAnotherTarget() {
	cases := map[string]httpConnectionRequest{
		"host":     {URL: "https://attacker.example.net/evaluate", Authentication: basicAuthRequest("svc-user", "")},
		"port":     {URL: "https://risk.example.com:8443/evaluate", Authentication: basicAuthRequest("svc-user", "")},
		"scheme":   {URL: "http://risk.example.com/evaluate", Authentication: basicAuthRequest("svc-user", "")},
		"username": {URL: storedHTTPURL, Authentication: basicAuthRequest("other-user", "")},
		"type": {URL: storedHTTPURL, Authentication: &outboundauth.Authentication{
			Type: string(outboundauth.TypeBearer)}},
	}
	for name, request := range cases {
		s.Run(name, func() {
			_, svcErr := s.updateHTTPWithStored(s.storedBasic(), request)
			s.Require().NotNil(svcErr)
			s.Equal(httpconnection.ErrorInvalidAuthentication.Code, svcErr.Code)
		})
	}
}

func (s *ServiceTestSuite) TestUpdateHTTPToNoneDropsTheStoredPassword() {
	authConfig, svcErr := s.updateHTTPWithStored(s.storedBasic(), httpConnectionRequest{URL: storedHTTPURL})

	s.Require().Nil(svcErr)
	s.Equal(outboundauth.Config{Type: outboundauth.TypeNone, Properties: map[string]string{}}, authConfig)
}

func (s *ServiceTestSuite) TestUpdateHTTPUsesASuppliedPassword() {
	authConfig, svcErr := s.updateHTTPWithStored(s.storedBasic(), httpConnectionRequest{
		URL: "https://attacker.example.net/evaluate", Authentication: basicAuthRequest("svc-user", "new-password"),
	})

	s.Require().Nil(svcErr)
	s.Equal("new-password", authConfig.Get(outboundauth.FieldBasicPassword))
}

func (s *ServiceTestSuite) TestUpdateHTTPRetainsABearerTokenOnlyForTheSameOrigin() {
	stored := s.storedHTTPAuth(outboundauth.TypeBearer,
		map[string]string{outboundauth.FieldBearerToken: "stored-token"})
	bearer := &outboundauth.Authentication{Type: string(outboundauth.TypeBearer)}

	authConfig, svcErr := s.updateHTTPWithStored(stored, httpConnectionRequest{
		URL: "https://risk.example.com:443/v2", Authentication: bearer})
	s.Require().Nil(svcErr)
	s.Equal("stored-token", authConfig.Get(outboundauth.FieldBearerToken))

	_, svcErr = s.updateHTTPWithStored(stored, httpConnectionRequest{
		URL: "https://attacker.example.net/evaluate", Authentication: bearer})
	s.Require().NotNil(svcErr)
	s.Equal(httpconnection.ErrorInvalidAuthentication.Code, svcErr.Code)
}

// An API-key update is a full list: a header sent as the mask keeps its own stored value while the
// origin is unchanged, a header left out is removed, and a new header needs its value.
func (s *ServiceTestSuite) TestUpdateHTTPResolvesAPIKeyHeadersOneByOne() {
	stored := s.storedHTTPAuth(outboundauth.TypeAPIKey, map[string]string{
		"X-Tenant-Key": "stored-tenant-key", "X-Region-Key": "stored-region-key"})
	apiKey := func(headers map[string]string) *outboundauth.Authentication {
		return &outboundauth.Authentication{Type: string(outboundauth.TypeAPIKey), Properties: headers}
	}

	authConfig, svcErr := s.updateHTTPWithStored(stored, httpConnectionRequest{URL: storedHTTPURL,
		Authentication: apiKey(map[string]string{"x-tenant-key": maskedSecretValue, "X-Extra-Key": "new-key"})})
	s.Require().Nil(svcErr)
	s.Equal(map[string]string{"X-Tenant-Key": "stored-tenant-key", "X-Extra-Key": "new-key"},
		authConfig.Properties)

	storedBearer := s.storedHTTPAuth(outboundauth.TypeBearer,
		map[string]string{outboundauth.FieldBearerToken: "stored-token"})
	for name, tc := range map[string]struct {
		stored  []cmodels.Property
		request httpConnectionRequest
	}{
		"another origin": {stored, httpConnectionRequest{URL: "https://attacker.example.net/evaluate",
			Authentication: apiKey(map[string]string{"X-Tenant-Key": maskedSecretValue})}},
		"another header name": {stored, httpConnectionRequest{URL: storedHTTPURL,
			Authentication: apiKey(map[string]string{"X-Other-Key": maskedSecretValue})}},
		"another method stored": {storedBearer, httpConnectionRequest{URL: storedHTTPURL,
			Authentication: apiKey(map[string]string{"Token": maskedSecretValue})}},
	} {
		s.Run(name, func() {
			_, svcErr := s.updateHTTPWithStored(tc.stored, tc.request)
			s.Require().NotNil(svcErr)
			s.Equal(httpconnection.ErrorInvalidAuthentication.Code, svcErr.Code)
		})
	}
}

func (s *ServiceTestSuite) TestListInstancesCarriesOU() {
	s.mockIDP.On("GetIdentityProviderList", mock.Anything).Return([]idp.BasicIDPDTO{
		{ID: "1", Name: "Owned", Type: providers.IDPTypeOIDC, OUID: "ou-1"},
		{ID: "2", Name: "Unowned", Type: providers.IDPTypeOIDC},
	}, (*tidcommon.ServiceError)(nil))

	got, svcErr := s.svc.listInstances(context.Background(), categoryIdentityProvider, serverconst.DefaultPageSize, 0)

	s.Nil(svcErr)
	s.Require().Len(got.Connections, 2)
	byID := map[string]connectionInstance{}
	for _, c := range got.Connections {
		byID[c.ID] = c
	}
	s.Equal("ou-1", byID["1"].OUID)
	s.Empty(byID["2"].OUID)
}

func (s *ServiceTestSuite) TestListInstancesCarriesOwnerOfSenderPDPAndHTTP() {
	mockHTTP := httpconnectionmock.NewHTTPConnectionServiceInterfaceMock(s.T())
	mockHTTP.On("ListHTTPConnections", mock.Anything).
		Return([]httpconnection.HTTPConnection{{ID: "h1", Name: "Risk", OUID: "ou-http"}},
			(*tidcommon.ServiceError)(nil)).Once()
	pdpService := authzenpdpmock.NewAuthZENPDPServiceInterfaceMock(s.T())
	pdpService.On("ListAuthZENPDPs", mock.Anything).
		Return([]authzenpdp.AuthZENPDPConnection{{ID: "p1", Name: "PDP", OUID: "ou-pdp"}},
			(*tidcommon.ServiceError)(nil)).Once()
	s.svc = newService(s.mockIDP, s.mockNotif, &testResourceServerLister{}, pdpService, mockHTTP)
	s.mockNotif.On("ListSendersByType", mock.Anything, ncommon.NotificationSenderTypeMessage).
		Return([]ncommon.NotificationSenderDTO{{ID: "s1", Name: "SMS", OUID: "ou-sms",
			Type: ncommon.NotificationSenderTypeMessage, Provider: ncommon.NotificationProviderTypeTwilio}},
			(*tidcommon.ServiceError)(nil)).Once()
	s.mockNotif.On("ListSendersByType", mock.Anything, ncommon.NotificationSenderTypeEmail).
		Return([]ncommon.NotificationSenderDTO{{ID: "e1", Name: "Mail", OUID: "ou-mail",
			Type: ncommon.NotificationSenderTypeEmail, Provider: ncommon.NotificationProviderTypeSMTP}},
			(*tidcommon.ServiceError)(nil)).Once()

	owners := map[string]string{}
	for _, category := range []connectionCategory{
		categorySMSProvider, categoryEmailProvider, categoryAuthorizationPDP, categoryHTTP,
	} {
		got, svcErr := s.svc.listInstances(context.Background(), category, serverconst.DefaultPageSize, 0)
		s.Nil(svcErr)
		for _, c := range got.Connections {
			owners[c.ID] = c.OUID
		}
	}

	s.Equal(map[string]string{"s1": "ou-sms", "e1": "ou-mail", "p1": "ou-pdp", "h1": "ou-http"}, owners)
}

// withHTTPWriter serves the HTTP connection writer the importer is handed over an HTTP connection
// service mock holding a connection stored at storedHTTPURL with storedAuth.
func (s *ServiceTestSuite) withHTTPWriter(storedAuth []cmodels.Property) (*httpConnectionWriter,
	*httpconnectionmock.HTTPConnectionServiceInterfaceMock, *httpconnection.HTTPConnection) {
	mockHTTP := httpconnectionmock.NewHTTPConnectionServiceInterfaceMock(s.T())
	s.svc = newService(s.mockIDP, s.mockNotif, &testResourceServerLister{}, newAuthZENPDPServiceMock(s.T()), mockHTTP)
	stored := &httpconnection.HTTPConnection{ID: "http-1", Name: "Risk", URL: storedHTTPURL,
		AuthProperties: storedAuth}
	return &httpConnectionWriter{svc: s.svc}, mockHTTP, stored
}

// importedHTTPConnection parses a connection document as an import does.
func (s *ServiceTestSuite) importedHTTPConnection(url, authentication string) httpconnection.HTTPConnection {
	s.T().Helper()
	var node yaml.Node
	s.Require().NoError(yaml.Unmarshal([]byte("resource_type: connection\nid: http-1\ntype: http\nname: Risk\n"+
		"url: "+url+"\nauthentication:\n"+authentication), &node))
	resource, err := ParseConnectionFromNode(node.Content[0])
	s.Require().NoError(err)
	connection, ok := resource.(*httpconnection.HTTPConnection)
	s.Require().True(ok)
	return *connection
}

func (s *ServiceTestSuite) TestTheWriterUpdatesAnImportedConnectionUnderTheRetentionRule() {
	omitted := "  type: basic\n  properties:\n    username: svc-user\n"
	for name, tc := range map[string]struct {
		url, authentication, want string
	}{
		"same origin keeps it":  {"https://RISK.example.com:443/v2", omitted, "stored-password"},
		"another host drops it": {"https://collector.example.net/evaluate", omitted, ""},
		"another port drops it": {"https://risk.example.com:8443/evaluate", omitted, ""},
		"a supplied password is new": {"https://collector.example.net/evaluate", omitted + "    password: doc-pw\n",
			"doc-pw"},
	} {
		s.Run(name, func() {
			writer, mockHTTP, stored := s.withHTTPWriter(s.storedBasic())
			var received *httpconnection.HTTPConnection
			mockHTTP.On("UpdateHTTPConnection", mock.Anything, "http-1", mock.Anything).
				Run(func(args mock.Arguments) {
					connection := args.Get(2).(httpconnection.HTTPConnection)
					received = &connection
				}).Return(stored, (*tidcommon.ServiceError)(nil)).Maybe()

			_, svcErr := writer.UpdateHTTPConnection(context.Background(), stored,
				s.importedHTTPConnection(tc.url, tc.authentication))

			if tc.want == "" {
				s.Require().NotNil(svcErr)
				s.Equal(httpconnection.ErrorInvalidAuthentication.Code, svcErr.Code)
				s.Nil(received, "a refused update must not reach the HTTP connection service")
				return
			}
			s.Require().Nil(svcErr)
			s.Require().NotNil(received)
			authConfig, err := outboundauth.FromProperties(received.AuthProperties)
			s.Require().NoError(err)
			s.Equal(tc.want, authConfig.Get(outboundauth.FieldBasicPassword))
		})
	}
}

func (s *ServiceTestSuite) TestTheWriterKeepsAMaskedAPIKeyHeaderOfAnImportedConnection() {
	writer, mockHTTP, stored := s.withHTTPWriter(s.storedHTTPAuth(outboundauth.TypeAPIKey,
		map[string]string{"X-Primary-Key": "stored-primary", "X-Secondary-Key": "stored-secondary"}))
	var received httpconnection.HTTPConnection
	mockHTTP.On("UpdateHTTPConnection", mock.Anything, "http-1", mock.Anything).
		Run(func(args mock.Arguments) { received = args.Get(2).(httpconnection.HTTPConnection) }).
		Return(stored, (*tidcommon.ServiceError)(nil))

	_, svcErr := writer.UpdateHTTPConnection(context.Background(), stored, s.importedHTTPConnection(storedHTTPURL,
		"  type: api_key\n  properties:\n    X-Primary-Key: \"******\"\n"))

	s.Require().Nil(svcErr)
	authConfig, err := outboundauth.FromProperties(received.AuthProperties)
	s.Require().NoError(err)
	s.Equal(map[string]string{"X-Primary-Key": "stored-primary"}, authConfig.Properties)
}

func (s *ServiceTestSuite) TestTheWritersUpdateValidationAppliesTheRetentionRule() {
	omitted := "  type: basic\n  properties:\n    username: svc-user\n"
	s.Run("same origin keeps the password", func() {
		writer, mockHTTP, stored := s.withHTTPWriter(s.storedBasic())
		var validated *httpconnection.HTTPConnection
		mockHTTP.On("ValidateHTTPConnectionUpdate", mock.Anything, stored, mock.Anything).
			Run(func(args mock.Arguments) { validated = args.Get(2).(*httpconnection.HTTPConnection) }).
			Return((*tidcommon.ServiceError)(nil))

		svcErr := writer.ValidateHTTPConnectionUpdate(context.Background(), stored,
			s.importedHTTPConnection("https://risk.example.com/v2", omitted))

		s.Require().Nil(svcErr)
		s.Require().NotNil(validated)
		authConfig, err := outboundauth.FromProperties(validated.AuthProperties)
		s.Require().NoError(err)
		s.Equal("stored-password", authConfig.Get(outboundauth.FieldBasicPassword))
	})
	s.Run("another origin without the password is refused", func() {
		writer, _, stored := s.withHTTPWriter(s.storedBasic())

		svcErr := writer.ValidateHTTPConnectionUpdate(context.Background(), stored,
			s.importedHTTPConnection("https://collector.example.net/evaluate", omitted))

		s.Require().NotNil(svcErr)
		s.Equal(httpconnection.ErrorInvalidAuthentication.Code, svcErr.Code)
	})
}
