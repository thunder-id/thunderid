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
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/idp/idpmock"
)

type ServiceTestSuite struct {
	suite.Suite
	svc     *service
	mockIDP *idpmock.IDPServiceInterfaceMock
}

func TestServiceSuite(t *testing.T) {
	suite.Run(t, new(ServiceTestSuite))
}

func (s *ServiceTestSuite) SetupTest() {
	initConfigWithTestCryptoKey()
	s.mockIDP = idpmock.NewIDPServiceInterfaceMock(s.T())
	s.svc = newService(s.mockIDP)
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
