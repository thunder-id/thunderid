/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package connection

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/notification"
	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
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

func TestServiceSuite(t *testing.T) {
	suite.Run(t, new(ServiceTestSuite))
}

func (s *ServiceTestSuite) SetupTest() {
	initConfigWithTestCryptoKey()
	s.mockIDP = idpmock.NewIDPServiceInterfaceMock(s.T())
	s.mockNotif = notificationmock.NewNotificationSenderMgtSvcInterfaceMock(s.T())
	s.svc = newService(s.mockIDP, s.mockNotif)
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
	s.Equal("s1", got.Connections[0].ID)
	s.Equal("custom", got.Connections[0].Type)
	s.Equal([]connectionCategory{categorySMSProvider}, got.Connections[0].Categories)
	s.Equal("2", got.Connections[1].ID) // "Google A" before "google B"
	s.Equal("google", got.Connections[1].Type)
	s.Equal([]connectionCategory{categoryIdentityProvider}, got.Connections[1].Categories)
	s.Equal("1", got.Connections[2].ID)
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
