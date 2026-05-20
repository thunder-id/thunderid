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

package ou

import (
	"context"
	"testing"

	"github.com/thunder-id/thunderid/internal/system/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

// DeclarativeModeServiceTestSuite tests service behavior in declarative mode.
type DeclarativeModeServiceTestSuite struct {
	suite.Suite
	service OrganizationUnitServiceInterface
	store   *organizationUnitStoreInterfaceMock
}

func (suite *DeclarativeModeServiceTestSuite) SetupTest() {
	// Initialize runtime with declarative mode enabled
	config.ResetServerRuntime()
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: true,
		},
	}
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	suite.Require().NoError(err)

	// Create service with mock store and dependencies
	suite.store = newOrganizationUnitStoreInterfaceMock(suite.T())
	mtx := new(mockTransactioner)
	mtx.On("Transact", mock.Anything, mock.Anything).Return(nil).Maybe()
	suite.service = &organizationUnitService{
		ouStore:       suite.store,
		authzService:  newAllowAllAuthz(suite.T()),
		transactioner: mtx,
	}
}

func (suite *DeclarativeModeServiceTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (suite *DeclarativeModeServiceTestSuite) TestCreateOrganizationUnit_FailsInDeclarativeMode() {
	request := OrganizationUnitRequestWithID{
		Name:        "Test OU",
		Handle:      "test-ou",
		Description: "Test Description",
	}

	ou, err := suite.service.CreateOrganizationUnit(context.Background(), request)

	// Should fail with immutable resource error
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorCannotModifyDeclarativeResource.Code, err.Code)
	assert.Equal(suite.T(), OrganizationUnit{}, ou)
}

func (suite *DeclarativeModeServiceTestSuite) TestUpdateOrganizationUnit_FailsInDeclarativeMode() {
	suite.store.On("GetOrganizationUnit", mock.Anything, "ou-1").Return(OrganizationUnit{
		ID:          "ou-1",
		Name:        "Existing OU",
		Handle:      "existing-ou",
		Description: "Existing Description",
	}, nil).Once()
	suite.store.On("IsOrganizationUnitDeclarative", mock.Anything, "ou-1").Return(true).Once()

	request := OrganizationUnitRequestWithID{
		Name:        "Updated OU",
		Handle:      "updated-ou",
		Description: "Updated Description",
	}

	ou, err := suite.service.UpdateOrganizationUnit(context.Background(), "ou-1", request)

	// Should fail with immutable resource error
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorCannotModifyDeclarativeResource.Code, err.Code)
	assert.Equal(suite.T(), OrganizationUnit{}, ou)
}

func (suite *DeclarativeModeServiceTestSuite) TestUpdateOrganizationUnitByPath_FailsInDeclarativeMode() {
	suite.store.On("GetOrganizationUnitByPath", mock.Anything, []string{"path", "to", "ou"}).Return(OrganizationUnit{
		ID:          "ou-1",
		Name:        "Existing OU",
		Handle:      "existing-ou",
		Description: "Existing Description",
	}, nil).Once()
	suite.store.On("IsOrganizationUnitDeclarative", mock.Anything, "ou-1").Return(true).Once()

	request := OrganizationUnitRequestWithID{
		Name:        "Updated OU",
		Handle:      "updated-ou",
		Description: "Updated Description",
	}

	ou, err := suite.service.UpdateOrganizationUnitByPath(context.Background(), "/path/to/ou", request)

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorCannotModifyDeclarativeResource.Code, err.Code)
	assert.Equal(suite.T(), OrganizationUnit{}, ou)
}

func (suite *DeclarativeModeServiceTestSuite) TestDeleteOrganizationUnit_FailsInDeclarativeMode() {
	suite.store.On("IsOrganizationUnitExists", mock.Anything, "ou-1").Return(true, nil).Once()
	suite.store.On("IsOrganizationUnitDeclarative", mock.Anything, "ou-1").Return(true).Once()

	err := suite.service.DeleteOrganizationUnit(context.Background(), "ou-1")

	// Should fail with immutable resource error
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorCannotModifyDeclarativeResource.Code, err.Code)
}

func (suite *DeclarativeModeServiceTestSuite) TestDeleteOrganizationUnitByPath_FailsInDeclarativeMode() {
	suite.store.On("GetOrganizationUnitByPath", mock.Anything, []string{"path", "to", "ou"}).Return(OrganizationUnit{
		ID: "ou-1",
	}, nil).Once()
	suite.store.On("IsOrganizationUnitDeclarative", mock.Anything, "ou-1").Return(true).Once()

	err := suite.service.DeleteOrganizationUnitByPath(context.Background(), "/path/to/ou")

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorCannotModifyDeclarativeResource.Code, err.Code)
}

func TestDeclarativeModeServiceTestSuite(t *testing.T) {
	suite.Run(t, new(DeclarativeModeServiceTestSuite))
}
