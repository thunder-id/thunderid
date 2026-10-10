// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"context"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

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
	schemaMock.On("GetDisplayAttributesByHandles", mock.Anything, mock.Anything,
		mock.MatchedBy(func(names []string) bool {
			if len(names) != 2 {
				return false
			}
			has := map[string]bool{names[0]: true, names[1]: true}
			return has["employee"] && has["contractor"]
		})).Return(map[string]string{"employee": "email", "contractor": "name"},
		(*tidcommon.ServiceError)(nil))

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
	schemaMock.On("GetDisplayAttributesByHandles", mock.Anything, mock.Anything, []string{"employee"}).
		Return((map[string]string)(nil),
			&tidcommon.ServiceError{
				Code:  "500",
				Error: tidcommon.I18nMessage{DefaultValue: "schema unavailable"},
			})

	logger := log.GetLogger()
	result := ResolveDisplayAttributePaths(context.Background(), []string{"employee"}, schemaMock, logger)
	suite.Nil(result)
}
