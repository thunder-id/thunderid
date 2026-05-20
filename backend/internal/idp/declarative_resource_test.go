/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package idp_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	i18ncore "github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/idp/idpmock"
)

// IDPExporterTestSuite tests the idpExporter.
type IDPExporterTestSuite struct {
	suite.Suite
	mockService *idpmock.IDPServiceInterfaceMock
	exporter    declarativeresource.ResourceExporter
	logger      *log.Logger
}

func TestIDPExporterTestSuite(t *testing.T) {
	suite.Run(t, new(IDPExporterTestSuite))
}

func (s *IDPExporterTestSuite) SetupTest() {
	s.mockService = idpmock.NewIDPServiceInterfaceMock(s.T())
	s.exporter = idp.NewIDPExporterForTest(s.mockService)
	s.logger = log.GetLogger()
}

func (s *IDPExporterTestSuite) TestNewIDPExporter() {
	assert.NotNil(s.T(), s.exporter)
}

func (s *IDPExporterTestSuite) TestGetResourceType() {
	assert.Equal(s.T(), "identity_provider", s.exporter.GetResourceType())
}

func (s *IDPExporterTestSuite) TestGetParameterizerType() {
	assert.Equal(s.T(), "IdentityProvider", s.exporter.GetParameterizerType())
}

func (s *IDPExporterTestSuite) TestGetAllResourceIDs_Success() {
	expectedIDPs := []idp.BasicIDPDTO{
		{ID: "idp1", Name: "IDP 1"},
		{ID: "idp2", Name: "IDP 2"},
	}

	s.mockService.EXPECT().GetIdentityProviderList(mock.Anything).Return(expectedIDPs, nil)

	ids, err := s.exporter.GetAllResourceIDs(context.Background())

	assert.Nil(s.T(), err)
	assert.Len(s.T(), ids, 2)
	assert.Equal(s.T(), "idp1", ids[0])
	assert.Equal(s.T(), "idp2", ids[1])
}

func (s *IDPExporterTestSuite) TestGetAllResourceIDs_Error() {
	expectedError := &serviceerror.ServiceError{
		Code:  "ERR_CODE",
		Error: i18ncore.I18nMessage{DefaultValue: "test error"},
	}

	s.mockService.EXPECT().GetIdentityProviderList(mock.Anything).Return(nil, expectedError)

	ids, err := s.exporter.GetAllResourceIDs(context.Background())

	assert.Nil(s.T(), ids)
	assert.Equal(s.T(), expectedError, err)
}

func (s *IDPExporterTestSuite) TestGetAllResourceIDs_EmptyList() {
	expectedIDPs := []idp.BasicIDPDTO{}

	s.mockService.EXPECT().GetIdentityProviderList(mock.Anything).Return(expectedIDPs, nil)

	ids, err := s.exporter.GetAllResourceIDs(context.Background())

	assert.Nil(s.T(), err)
	assert.Len(s.T(), ids, 0)
}

func (s *IDPExporterTestSuite) TestGetResourceByID_Success() {
	expectedIDP := &idp.IDPDTO{
		ID:   "idp1",
		Name: "Test IDP",
	}

	s.mockService.EXPECT().GetIdentityProvider(mock.Anything, "idp1").Return(expectedIDP, nil)

	resource, name, err := s.exporter.GetResourceByID(context.Background(), "idp1")

	assert.Nil(s.T(), err)
	assert.Equal(s.T(), "Test IDP", name)
	assert.Equal(s.T(), expectedIDP, resource)
}

func (s *IDPExporterTestSuite) TestGetResourceByID_Error() {
	expectedError := &serviceerror.ServiceError{
		Code:  "ERR_CODE",
		Error: i18ncore.I18nMessage{DefaultValue: "test error"},
	}

	s.mockService.EXPECT().GetIdentityProvider(mock.Anything, "idp1").Return(nil, expectedError)

	resource, name, err := s.exporter.GetResourceByID(context.Background(), "idp1")

	assert.Nil(s.T(), resource)
	assert.Empty(s.T(), name)
	assert.Equal(s.T(), expectedError, err)
}

func (s *IDPExporterTestSuite) TestValidateResource_Success() {
	prop, _ := cmodels.NewProperty("key1", "value1", false)
	idpDTO := &idp.IDPDTO{
		ID:         "idp1",
		Name:       "Valid IDP",
		Properties: []cmodels.Property{*prop},
	}

	name, err := s.exporter.ValidateResource(idpDTO, "idp1", s.logger)

	assert.Nil(s.T(), err)
	assert.Equal(s.T(), "Valid IDP", name)
}

func (s *IDPExporterTestSuite) TestValidateResource_InvalidType() {
	invalidResource := "not an IDP"

	name, err := s.exporter.ValidateResource(invalidResource, "idp1", s.logger)

	assert.Empty(s.T(), name)
	assert.NotNil(s.T(), err)
	assert.Equal(s.T(), "identity_provider", err.ResourceType)
	assert.Equal(s.T(), "idp1", err.ResourceID)
	assert.Equal(s.T(), "INVALID_TYPE", err.Code)
}

func (s *IDPExporterTestSuite) TestValidateResource_EmptyName() {
	idpDTO := &idp.IDPDTO{
		ID:   "idp1",
		Name: "",
	}

	name, err := s.exporter.ValidateResource(idpDTO, "idp1", s.logger)

	assert.Empty(s.T(), name)
	assert.NotNil(s.T(), err)
	assert.Equal(s.T(), "identity_provider", err.ResourceType)
	assert.Equal(s.T(), "idp1", err.ResourceID)
	assert.Equal(s.T(), "IDP_VALIDATION_ERROR", err.Code)
	assert.Contains(s.T(), err.Error, "name is empty")
}

func (s *IDPExporterTestSuite) TestValidateResource_NoProperties() {
	idpDTO := &idp.IDPDTO{
		ID:         "idp1",
		Name:       "Test IDP",
		Properties: []cmodels.Property{},
	}

	name, err := s.exporter.ValidateResource(idpDTO, "idp1", s.logger)

	// Should still succeed but log a warning
	assert.Nil(s.T(), err)
	assert.Equal(s.T(), "Test IDP", name)
}
