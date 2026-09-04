// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/notification"
	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/idp/idpmock"
	"github.com/thunder-id/thunderid/tests/mocks/notification/notificationmock"
)

type ServiceTestSuite struct {
	suite.Suite
	svc       *service
	mockIDP   *idpmock.IDPServiceInterfaceMock
	mockNotif *notificationmock.NotificationSenderMgtSvcInterfaceMock
}

type testAuthZENPDPStore struct {
	connections map[string]authZENPDPConnection
}

type testResourceServerLister struct {
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

func newTestAuthZENPDPStore() *testAuthZENPDPStore {
	return &testAuthZENPDPStore{connections: map[string]authZENPDPConnection{}}
}

func (s *testAuthZENPDPStore) create(_ context.Context, connection authZENPDPConnection) error {
	s.connections[connection.ID] = connection
	return nil
}

func (s *testAuthZENPDPStore) list(_ context.Context) ([]authZENPDPConnection, error) {
	connections := make([]authZENPDPConnection, 0, len(s.connections))
	for _, connection := range s.connections {
		connections = append(connections, connection)
	}
	return connections, nil
}

func (s *testAuthZENPDPStore) get(_ context.Context, id string) (*authZENPDPConnection, error) {
	connection, ok := s.connections[id]
	if !ok {
		return nil, nil
	}
	return &connection, nil
}

func (s *testAuthZENPDPStore) update(_ context.Context, id string, connection authZENPDPConnection) error {
	connection.ID = id
	s.connections[id] = connection
	return nil
}

func (s *testAuthZENPDPStore) delete(_ context.Context, id string) error {
	delete(s.connections, id)
	return nil
}

func TestServiceSuite(t *testing.T) {
	suite.Run(t, new(ServiceTestSuite))
}

func (s *ServiceTestSuite) SetupTest() {
	initConfigWithTestCryptoKey(s.T())
	s.mockIDP = idpmock.NewIDPServiceInterfaceMock(s.T())
	s.mockNotif = notificationmock.NewNotificationSenderMgtSvcInterfaceMock(s.T())
	s.svc = newService(s.mockIDP, s.mockNotif, &testResourceServerLister{}, newTestAuthZENPDPStore())
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
	store := newTestAuthZENPDPStore()
	s.svc = newService(s.mockIDP, s.mockNotif, &testResourceServerLister{}, store)

	created, svcErr := s.svc.createAuthZENPDP(context.Background(), authZENPDPConnection{
		Name:          "PDP",
		Endpoint:      " https://pdp.example.com/access/v1/evaluation ",
		BatchEndpoint: " https://pdp.example.com/access/v1/evaluations ",
	})

	s.Nil(svcErr)
	s.Require().NotNil(created)
	s.NotEmpty(created.ID)
	s.Equal("https://pdp.example.com/access/v1/evaluation", created.Endpoint)
	s.Equal("https://pdp.example.com/access/v1/evaluations", created.BatchEndpoint)
	s.Equal(created.Endpoint, store.connections[created.ID].Endpoint)
	s.Equal(created.BatchEndpoint, store.connections[created.ID].BatchEndpoint)
}

func (s *ServiceTestSuite) TestUpdateAuthZENPDPStoresConfiguredEndpoints() {
	store := newTestAuthZENPDPStore()
	store.connections["pdp-1"] = authZENPDPConnection{
		ID:            "pdp-1",
		Name:          "PDP",
		Endpoint:      "https://old-pdp.example.com/access/v1/evaluation",
		BatchEndpoint: "https://old-pdp.example.com/access/v1/evaluations",
	}
	s.svc = newService(s.mockIDP, s.mockNotif, &testResourceServerLister{}, store)

	updated, svcErr := s.svc.updateAuthZENPDP(context.Background(), "pdp-1", authZENPDPConnection{
		Name:          "New PDP",
		Endpoint:      "https://new-pdp.example.com/access/v1/evaluation",
		BatchEndpoint: "https://new-pdp.example.com/access/v1/evaluations",
	})

	s.Nil(svcErr)
	s.Require().NotNil(updated)
	s.Equal("https://new-pdp.example.com/access/v1/evaluation", updated.Endpoint)
	s.Equal("https://new-pdp.example.com/access/v1/evaluations", updated.BatchEndpoint)
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
				Provider: ncommon.MessageProviderTypeCustom},
		}, (*tidcommon.ServiceError)(nil))

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
				Provider: ncommon.MessageProviderTypeTwilio},
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
				Provider: ncommon.MessageProviderTypeTwilio},
			{ID: "s2", Name: "Unregistered", Type: ncommon.NotificationSenderTypeMessage,
				Provider: ncommon.MessageProviderType("unregistered-provider")},
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

	got, svcErr := s.svc.listInstances(context.Background(), "", serverconst.DefaultPageSize, 0)
	s.Nil(svcErr)
	s.Require().Len(got.Connections, 2)
	s.Equal("1", got.Connections[0].ID)
	s.Equal("2", got.Connections[1].ID)
}

func (s *ServiceTestSuite) TestSMSVendorNameUnregisteredProviderReturnsFalse() {
	name, ok := smsVendorName(ncommon.MessageProviderType("unregistered-provider"))
	s.False(ok)
	s.Empty(name)
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
			{ID: "1", Type: ncommon.NotificationSenderTypeMessage, Provider: ncommon.MessageProviderTypeTwilio},
			{ID: "2", Type: ncommon.NotificationSenderTypeMessage, Provider: ncommon.MessageProviderTypeVonage},
			{ID: "3", Type: ncommon.NotificationSenderTypeMessage, Provider: ncommon.MessageProviderTypeTwilio},
		}, (*tidcommon.ServiceError)(nil))

	got, svcErr := s.svc.listSMSByProvider(context.Background(), ncommon.MessageProviderTypeTwilio)
	s.Nil(svcErr)
	s.Len(got, 2)
}

func (s *ServiceTestSuite) TestListSMSByProviderError() {
	s.mockNotif.On("ListSendersByType", mock.Anything, ncommon.NotificationSenderTypeMessage).
		Return(([]ncommon.NotificationSenderDTO)(nil), &tidcommon.InternalServerError)

	_, svcErr := s.svc.listSMSByProvider(context.Background(), ncommon.MessageProviderTypeTwilio)
	s.NotNil(svcErr)
}

func (s *ServiceTestSuite) TestGetSMSByProviderMismatchReturnsNotFound() {
	s.mockNotif.On("GetSender", mock.Anything, "x").Return(&ncommon.NotificationSenderDTO{
		ID: "x", Type: ncommon.NotificationSenderTypeMessage, Provider: ncommon.MessageProviderTypeVonage,
	}, (*tidcommon.ServiceError)(nil))

	_, svcErr := s.svc.getSMSByProvider(context.Background(), ncommon.MessageProviderTypeTwilio, "x")
	s.Require().NotNil(svcErr)
	s.Equal(notification.ErrorSenderNotFound.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestGetSMSByProviderError() {
	s.mockNotif.On("GetSender", mock.Anything, "missing").
		Return((*ncommon.NotificationSenderDTO)(nil), &notification.ErrorSenderNotFound)

	_, svcErr := s.svc.getSMSByProvider(context.Background(), ncommon.MessageProviderTypeTwilio, "missing")
	s.Require().NotNil(svcErr)
	s.Equal(notification.ErrorSenderNotFound.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestDeleteSMSByProviderGetFails() {
	s.mockNotif.On("GetSender", mock.Anything, "missing").
		Return((*ncommon.NotificationSenderDTO)(nil), &notification.ErrorSenderNotFound)

	svcErr := s.svc.deleteSMSByProvider(context.Background(), ncommon.MessageProviderTypeTwilio, "missing")
	s.Require().NotNil(svcErr)
	s.mockNotif.AssertNotCalled(s.T(), "DeleteSender", mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestUpdateSMSOmittedSecretKeepsStored() {
	s.mockNotif.On("GetSender", mock.Anything, "tw-1").Return(&ncommon.NotificationSenderDTO{
		ID: "tw-1", Type: ncommon.NotificationSenderTypeMessage, Provider: ncommon.MessageProviderTypeTwilio,
		Properties: s.authToken("stored"),
	}, (*tidcommon.ServiceError)(nil))

	var captured ncommon.NotificationSenderDTO
	s.mockNotif.On("UpdateSender", mock.Anything, "tw-1", mock.Anything).
		Run(func(args mock.Arguments) { captured = args.Get(2).(ncommon.NotificationSenderDTO) }).
		Return(&ncommon.NotificationSenderDTO{ID: "tw-1"}, (*tidcommon.ServiceError)(nil))

	// Update carries no secret property at all → the stored secret is preserved.
	dto := ncommon.NotificationSenderDTO{
		Name: "tw", Type: ncommon.NotificationSenderTypeMessage, Provider: ncommon.MessageProviderTypeTwilio,
	}
	_, svcErr := s.svc.updateSMS(context.Background(), ncommon.MessageProviderTypeTwilio, "tw-1", dto)

	s.Nil(svcErr)
	s.Require().Len(captured.Properties, 1)
	v, err := captured.Properties[0].GetValue()
	s.NoError(err)
	s.Equal("stored", v)
}

func (s *ServiceTestSuite) TestUpdateSMSProviderMismatch() {
	s.mockNotif.On("GetSender", mock.Anything, "x").Return(&ncommon.NotificationSenderDTO{
		ID: "x", Type: ncommon.NotificationSenderTypeMessage, Provider: ncommon.MessageProviderTypeVonage,
	}, (*tidcommon.ServiceError)(nil))

	dto := ncommon.NotificationSenderDTO{
		Type: ncommon.NotificationSenderTypeMessage, Provider: ncommon.MessageProviderTypeTwilio,
	}
	_, svcErr := s.svc.updateSMS(context.Background(), ncommon.MessageProviderTypeTwilio, "x", dto)
	s.Require().NotNil(svcErr)
	s.Equal(notification.ErrorSenderNotFound.Code, svcErr.Code)
	s.mockNotif.AssertNotCalled(s.T(), "UpdateSender", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestDeleteSMSByProviderDelegates() {
	s.mockNotif.On("GetSender", mock.Anything, "tw-1").Return(&ncommon.NotificationSenderDTO{
		ID: "tw-1", Type: ncommon.NotificationSenderTypeMessage, Provider: ncommon.MessageProviderTypeTwilio,
	}, (*tidcommon.ServiceError)(nil))
	s.mockNotif.On("DeleteSender", mock.Anything, "tw-1").Return((*tidcommon.ServiceError)(nil))

	svcErr := s.svc.deleteSMSByProvider(context.Background(), ncommon.MessageProviderTypeTwilio, "tw-1")
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
		ID: "tw-1", Type: ncommon.NotificationSenderTypeMessage, Provider: ncommon.MessageProviderTypeTwilio,
	}, (*tidcommon.ServiceError)(nil))
	s.mockNotif.On("GetSenderUsages", mock.Anything, "tw-1").Return(usages, (*tidcommon.ServiceError)(nil))

	result, svcErr := s.svc.usagesSMSByProvider(context.Background(), ncommon.MessageProviderTypeTwilio, "tw-1")
	s.Nil(svcErr)
	s.Equal(usages, result)
}

func (s *ServiceTestSuite) TestUsagesAuthZENPDPReturnsReferencingResourceServers() {
	store := newTestAuthZENPDPStore()
	store.connections["pdp-1"] = authZENPDPConnection{
		ID:            "pdp-1",
		Name:          "PDP",
		Endpoint:      "https://pdp.example.com/access/v1/evaluation",
		BatchEndpoint: "https://pdp.example.com/access/v1/evaluations",
	}
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
							Type: providers.AuthorizationEngineTypeExternalAuthZENPDP,
							Properties: providers.AuthorizationEngineProperties{
								ExternalPDPConnectionID: "pdp-1",
							},
						},
					},
					{
						ID:   "rs-2",
						Name: "Billing API",
						AuthorizationEngine: providers.AuthorizationEngineConfig{
							Type: providers.AuthorizationEngineTypeExternalAuthZENPDP,
							Properties: providers.AuthorizationEngineProperties{
								ExternalPDPConnectionID: "other",
							},
						},
					},
				},
			},
		},
	}
	s.svc = newService(s.mockIDP, s.mockNotif, resourceLister, store)

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
	store := newTestAuthZENPDPStore()
	store.connections["pdp-1"] = authZENPDPConnection{
		ID:            "pdp-1",
		Name:          "PDP",
		Endpoint:      "https://pdp.example.com/access/v1/evaluation",
		BatchEndpoint: "https://pdp.example.com/access/v1/evaluations",
	}
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
							Type: providers.AuthorizationEngineTypeExternalAuthZENPDP,
							Properties: providers.AuthorizationEngineProperties{
								ExternalPDPConnectionID: "pdp-1",
							},
						},
					},
				},
			},
		},
	}, store)

	svcErr := s.svc.deleteAuthZENPDP(context.Background(), "pdp-1")

	s.Require().NotNil(svcErr)
	s.Equal(ErrorConnectionHasBlockingDependencies.Code, svcErr.Code)
	s.Contains(store.connections, "pdp-1")
}

// TestUsagesSMSByProviderWrongProvider verifies a sender of another provider is not exposed
// through a vendor's usages endpoint.
func (s *ServiceTestSuite) TestUsagesSMSByProviderWrongProvider() {
	s.mockNotif.On("GetSender", mock.Anything, "vo-1").Return(&ncommon.NotificationSenderDTO{
		ID: "vo-1", Type: ncommon.NotificationSenderTypeMessage, Provider: ncommon.MessageProviderTypeVonage,
	}, (*tidcommon.ServiceError)(nil))

	result, svcErr := s.svc.usagesSMSByProvider(context.Background(), ncommon.MessageProviderTypeTwilio, "vo-1")
	s.Require().NotNil(svcErr)
	s.Nil(result)
	s.mockNotif.AssertNotCalled(s.T(), "GetSenderUsages", mock.Anything, mock.Anything)
}
