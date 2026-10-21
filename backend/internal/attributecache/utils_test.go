// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package attributecache

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type UtilsTestSuite struct {
	suite.Suite
}

func TestUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(UtilsTestSuite))
}

func (suite *UtilsTestSuite) TestWithSessionGrants_DropsReservedAttribute() {
	stored := withSessionGrants(map[string]interface{}{"email": "a@example.com", sessionGrantsKey: "forged"}, nil)
	suite.Equal(map[string]interface{}{"email": "a@example.com"}, stored)
}

func (suite *UtilsTestSuite) TestWithSessionGrants_StoresGrants() {
	grants := &SessionGrants{GroupIDs: []string{"g1"}}
	stored := withSessionGrants(map[string]interface{}{sessionGrantsKey: "forged"}, grants)
	suite.Equal(map[string]interface{}{sessionGrantsKey: grants}, stored)
}

func (suite *UtilsTestSuite) TestSplitSessionGrants_None() {
	attributes := map[string]interface{}{"email": "a@example.com"}
	grants, err := splitSessionGrants(attributes)
	suite.NoError(err)
	suite.Nil(grants)
	suite.Equal(map[string]interface{}{"email": "a@example.com"}, attributes)
}

func (suite *UtilsTestSuite) TestSplitSessionGrants_DecodesAndRemoves() {
	attributes := map[string]interface{}{"email": "a@example.com", sessionGrantsKey: map[string]interface{}{
		"groupIds": []interface{}{"g1"}, "roleIds": []interface{}{"r1"},
	}}
	grants, err := splitSessionGrants(attributes)
	suite.NoError(err)
	suite.Equal(&SessionGrants{GroupIDs: []string{"g1"}, RoleIDs: []string{"r1"}}, grants)
	suite.Equal(map[string]interface{}{"email": "a@example.com"}, attributes)
}

func (suite *UtilsTestSuite) TestSplitSessionGrants_InvalidValue() {
	_, err := splitSessionGrants(map[string]interface{}{sessionGrantsKey: "not an object"})
	suite.Error(err)
}
