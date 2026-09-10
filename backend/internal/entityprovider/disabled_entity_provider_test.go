// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
