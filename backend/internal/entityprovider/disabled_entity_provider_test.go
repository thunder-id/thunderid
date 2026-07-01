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

package entityprovider

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type DisabledEntityProviderTestSuite struct {
	suite.Suite
	provider EntityProviderInterface
}

func (suite *DisabledEntityProviderTestSuite) SetupTest() {
	suite.provider = newDisabledEntityProvider()
}

func TestDisabledEntityProviderTestSuite(t *testing.T) {
	suite.Run(t, new(DisabledEntityProviderTestSuite))
}

func (suite *DisabledEntityProviderTestSuite) TestIdentifyEntity() {
	id, err := suite.provider.IdentifyEntity(map[string]interface{}{})
	suite.Nil(id)
	suite.Equal(errNotImplemented, err)
}

func (suite *DisabledEntityProviderTestSuite) TestGetEntity() {
	e, err := suite.provider.GetEntity("entity-id")
	suite.Nil(e)
	suite.Equal(errNotImplemented, err)
}

func (suite *DisabledEntityProviderTestSuite) TestCreateEntity() {
	e, err := suite.provider.CreateEntity(&providers.Entity{}, json.RawMessage{})
	suite.Nil(e)
	suite.Equal(errNotImplemented, err)
}

func (suite *DisabledEntityProviderTestSuite) TestUpdateEntity() {
	e, err := suite.provider.UpdateEntity("entity-id", &providers.Entity{})
	suite.Nil(e)
	suite.Equal(errNotImplemented, err)
}

func (suite *DisabledEntityProviderTestSuite) TestDeleteEntity() {
	err := suite.provider.DeleteEntity("entity-id")
	suite.Equal(errNotImplemented, err)
}

func (suite *DisabledEntityProviderTestSuite) TestUpdateCredentials() {
	err := suite.provider.UpdateCredentials("entity-id", json.RawMessage{})
	suite.Equal(errNotImplemented, err)
}

func (suite *DisabledEntityProviderTestSuite) TestUpdateSystemAttributes() {
	err := suite.provider.UpdateSystemAttributes("entity-id", json.RawMessage{})
	suite.Equal(errNotImplemented, err)
}

func (suite *DisabledEntityProviderTestSuite) TestUpdateSystemCredentials() {
	err := suite.provider.UpdateSystemCredentials("entity-id", json.RawMessage{})
	suite.Equal(errNotImplemented, err)
}

func (suite *DisabledEntityProviderTestSuite) TestGetTransitiveEntityGroups() {
	groups, err := suite.provider.GetTransitiveEntityGroups("entity-id")
	suite.Nil(groups)
	suite.Equal(errNotImplemented, err)
}

func (suite *DisabledEntityProviderTestSuite) TestValidateEntityIDs() {
	ids, err := suite.provider.ValidateEntityIDs([]string{"id1"})
	suite.Nil(ids)
	suite.Equal(errNotImplemented, err)
}

func (suite *DisabledEntityProviderTestSuite) TestGetEntitiesByIDs() {
	entities, err := suite.provider.GetEntitiesByIDs([]string{"id1"})
	suite.Nil(entities)
	suite.Equal(errNotImplemented, err)
}
