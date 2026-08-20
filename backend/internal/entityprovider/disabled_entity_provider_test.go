// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entityprovider

import (
	"context"
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
	id, err := suite.provider.IdentifyEntity(context.Background(), map[string]interface{}{})
	suite.Nil(id)
	suite.Equal(errNotImplemented, err)
}

func (suite *DisabledEntityProviderTestSuite) TestGetEntity() {
	e, err := suite.provider.GetEntity(context.Background(), "entity-id")
	suite.Nil(e)
	suite.Equal(errNotImplemented, err)
}

func (suite *DisabledEntityProviderTestSuite) TestCreateEntity() {
	e, err := suite.provider.CreateEntity(context.Background(), &providers.Entity{}, json.RawMessage{})
	suite.Nil(e)
	suite.Equal(errNotImplemented, err)
}

func (suite *DisabledEntityProviderTestSuite) TestUpdateEntity() {
	e, err := suite.provider.UpdateEntity(context.Background(), "entity-id", &providers.Entity{})
	suite.Nil(e)
	suite.Equal(errNotImplemented, err)
}

func (suite *DisabledEntityProviderTestSuite) TestDeleteEntity() {
	err := suite.provider.DeleteEntity(context.Background(), "entity-id")
	suite.Equal(errNotImplemented, err)
}

func (suite *DisabledEntityProviderTestSuite) TestUpdateCredentials() {
	err := suite.provider.UpdateCredentials(context.Background(), "entity-id", json.RawMessage{})
	suite.Equal(errNotImplemented, err)
}

func (suite *DisabledEntityProviderTestSuite) TestUpdateSystemAttributes() {
	err := suite.provider.UpdateSystemAttributes(context.Background(), "entity-id", json.RawMessage{})
	suite.Equal(errNotImplemented, err)
}

func (suite *DisabledEntityProviderTestSuite) TestUpdateSystemCredentials() {
	err := suite.provider.UpdateSystemCredentials(context.Background(), "entity-id", json.RawMessage{})
	suite.Equal(errNotImplemented, err)
}

func (suite *DisabledEntityProviderTestSuite) TestGetTransitiveEntityGroups() {
	groups, err := suite.provider.GetTransitiveEntityGroups(context.Background(), "entity-id")
	suite.Nil(groups)
	suite.Equal(errNotImplemented, err)
}

func (suite *DisabledEntityProviderTestSuite) TestValidateEntityIDs() {
	ids, err := suite.provider.ValidateEntityIDs(context.Background(), []string{"id1"})
	suite.Nil(ids)
	suite.Equal(errNotImplemented, err)
}

func (suite *DisabledEntityProviderTestSuite) TestGetEntitiesByIDs() {
	entities, err := suite.provider.GetEntitiesByIDs(context.Background(), []string{"id1"})
	suite.Nil(entities)
	suite.Equal(errNotImplemented, err)
}
