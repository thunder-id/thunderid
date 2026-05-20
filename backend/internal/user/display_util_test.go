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

package user

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	i18ncore "github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
)

// DisplayUtilTestSuite tests the display attribute resolution utility functions.
type DisplayUtilTestSuite struct {
	suite.Suite
}

func TestDisplayUtilTestSuite(t *testing.T) {
	suite.Run(t, new(DisplayUtilTestSuite))
}

func (suite *DisplayUtilTestSuite) TestResolveDisplayAttributePaths_DeduplicatesTypes() {
	schemaMock := entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	schemaMock.On("GetDisplayAttributesByNames", mock.Anything, mock.Anything,
		mock.MatchedBy(func(names []string) bool {
			if len(names) != 2 {
				return false
			}
			has := map[string]bool{names[0]: true, names[1]: true}
			return has["employee"] && has["contractor"]
		})).Return(map[string]string{"employee": "email", "contractor": "name"},
		(*serviceerror.ServiceError)(nil))

	result := ResolveDisplayAttributePaths(context.Background(),
		[]string{"employee", "contractor", "employee"}, schemaMock, nil)
	suite.Equal("email", result["employee"])
	suite.Equal("name", result["contractor"])
}

func (suite *DisplayUtilTestSuite) TestResolveDisplayAttributePaths_NilSchemaService() {
	result := ResolveDisplayAttributePaths(context.Background(), []string{"employee"}, nil, nil)
	suite.Nil(result)
}

func (suite *DisplayUtilTestSuite) TestResolveDisplayAttributePaths_EmptyTypes() {
	schemaMock := entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	result := ResolveDisplayAttributePaths(context.Background(), []string{}, schemaMock, nil)
	suite.Nil(result)
}

func (suite *DisplayUtilTestSuite) TestResolveDisplayAttributePaths_AllEmptyStrings() {
	schemaMock := entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	result := ResolveDisplayAttributePaths(context.Background(), []string{"", ""}, schemaMock, nil)
	suite.Nil(result)
}

func (suite *DisplayUtilTestSuite) TestResolveDisplayAttributePaths_SchemaServiceError() {
	schemaMock := entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	schemaMock.On("GetDisplayAttributesByNames", mock.Anything, mock.Anything, []string{"employee"}).
		Return((map[string]string)(nil),
			&serviceerror.ServiceError{
				Code:  "500",
				Error: i18ncore.I18nMessage{DefaultValue: "schema unavailable"},
			})

	logger := log.GetLogger()
	result := ResolveDisplayAttributePaths(context.Background(), []string{"employee"}, schemaMock, logger)
	suite.Nil(result)
}
