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
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/filter"
	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

const testDeploymentID = "test-deployment-id"

type OrganizationUnitStoreTestSuite struct {
	suite.Suite
	providerMock *providermock.DBProviderInterfaceMock
	dbClientMock *providermock.DBClientInterfaceMock
	store        *organizationUnitStore
}

func TestOrganizationUnitStoreTestSuite(t *testing.T) {
	suite.Run(t, new(OrganizationUnitStoreTestSuite))
}

func (suite *OrganizationUnitStoreTestSuite) SetupTest() {
	suite.providerMock = providermock.NewDBProviderInterfaceMock(suite.T())
	suite.dbClientMock = providermock.NewDBClientInterfaceMock(suite.T())
	suite.store = &organizationUnitStore{
		dbProvider:   suite.providerMock,
		deploymentID: testDeploymentID,
	}
}

func (suite *OrganizationUnitStoreTestSuite) expectDBClient() {
	suite.providerMock.On("GetUserDBClient").Return(suite.dbClientMock, nil)
}

type conflictTestCase struct {
	name      string
	hasParent bool
	parentVal string
	setup     func(parent *string)
	want      bool
	wantErr   string
}

// runConflictTestCases centralizes the repeated assertion flow for parent-aware conflict checks.
func (suite *OrganizationUnitStoreTestSuite) runConflictTestCases(
	tests []conflictTestCase,
	invoke func(parent *string) (bool, error),
) {
	for _, tc := range tests {
		tc := tc
		suite.Run(tc.name, func() {
			suite.SetupTest()
			var parent *string
			if tc.hasParent {
				p := tc.parentVal
				parent = &p
			}
			if tc.setup != nil {
				tc.setup(parent)
			}

			result, err := invoke(parent)

			if tc.wantErr != "" {
				suite.Require().Error(err)
				suite.Contains(err.Error(), tc.wantErr)
				return
			}

			suite.Require().NoError(err)
			suite.Equal(tc.want, result)
		})
	}
}

type countTestCase struct {
	name    string
	setup   func()
	want    int
	wantErr string
}

// runCountTestCases removes duplicated boilerplate around count store methods.
func (suite *OrganizationUnitStoreTestSuite) runCountTestCases(
	tests []countTestCase,
	invoke func() (int, error),
) {
	for _, tc := range tests {
		tc := tc
		suite.Run(tc.name, func() {
			suite.SetupTest()
			if tc.setup != nil {
				tc.setup()
			}

			result, err := invoke()

			if tc.wantErr != "" {
				suite.Require().Error(err)
				suite.Contains(err.Error(), tc.wantErr)
				return
			}

			suite.Require().NoError(err)
			suite.Equal(tc.want, result)
		})
	}
}

func (suite *OrganizationUnitStoreTestSuite) runConflictQueryScenario(
	withParentQueryID, withoutParentQueryID interface{}, value string,
	withParentCount, withoutParentCount int64,
	invoke func(parent *string) (bool, error),
) {
	parentLabel := "with parent"
	if withParentCount > 0 {
		parentLabel += " conflict"
	} else {
		parentLabel += " no conflict"
	}

	rootLabel := "without parent"
	if withoutParentCount > 0 {
		rootLabel += " conflict"
	} else {
		rootLabel += " no conflict"
	}

	suite.runConflictTestCases(
		[]conflictTestCase{
			{
				name:      parentLabel,
				hasParent: true,
				parentVal: testParentID,
				setup: func(parent *string) {
					suite.expectDBClient()
					suite.dbClientMock.
						On(
							"QueryContext", mock.Anything, withParentQueryID, value, *parent, testDeploymentID).
						Return([]map[string]interface{}{{"count": withParentCount}}, nil).
						Once()
				},
				want: withParentCount > 0,
			},
			{
				name: rootLabel,
				setup: func(_ *string) {
					suite.expectDBClient()
					suite.dbClientMock.
						On(
							"QueryContext", mock.Anything, withoutParentQueryID, value, testDeploymentID).
						Return([]map[string]interface{}{{"count": withoutParentCount}}, nil).
						Once()
				},
				want: withoutParentCount > 0,
			},
			{
				name: "query error",
				setup: func(_ *string) {
					suite.expectDBClient()
					suite.dbClientMock.
						On(
							"QueryContext", mock.Anything, withoutParentQueryID, value, testDeploymentID).
						Return(nil, errors.New("query err")).
						Once()
				},
				wantErr: "failed to execute query",
			},
			{
				name: "db client error",
				setup: func(_ *string) {
					suite.providerMock.
						On("GetUserDBClient").
						Return(nil, errors.New("db err")).
						Once()
				},
				wantErr: "failed to get database client",
			},
		},
		invoke,
	)
}

func (suite *OrganizationUnitStoreTestSuite) runCountQueryScenario(
	queryID interface{}, arg string, deploymentID string,
	successCount int,
	invoke func() (int, error),
) {
	suite.runCountTestCases(
		[]countTestCase{
			{
				name: "success",
				setup: func() {
					suite.expectDBClient()
					suite.dbClientMock.
						On(
							"QueryContext", mock.Anything, queryID, arg, deploymentID).
						Return([]map[string]interface{}{{"total": int64(successCount)}}, nil).
						Once()
				},
				want: successCount,
			},
			{
				name: "empty result",
				setup: func() {
					suite.expectDBClient()
					suite.dbClientMock.
						On(
							"QueryContext", mock.Anything, queryID, arg, deploymentID).
						Return([]map[string]interface{}{}, nil).
						Once()
				},
				want: 0,
			},
			{
				name: "invalid type",
				setup: func() {
					suite.expectDBClient()
					suite.dbClientMock.
						On(
							"QueryContext", mock.Anything, queryID, arg, deploymentID).
						Return([]map[string]interface{}{{"total": "bad"}}, nil).
						Once()
				},
				wantErr: "failed to parse count result",
			},
			{
				name: "query error",
				setup: func() {
					suite.expectDBClient()
					suite.dbClientMock.
						On(
							"QueryContext", mock.Anything, queryID, arg, deploymentID).
						Return(nil, errors.New("query err")).
						Once()
				},
				wantErr: "failed to execute count query",
			},
			{
				name: "db client error",
				setup: func() {
					suite.providerMock.
						On("GetUserDBClient").
						Return(nil, errors.New("db err")).
						Once()
				},
				wantErr: "failed to get database client",
			},
		},
		invoke,
	)
}

func makeOUResultRow(id, handle, name, description string, parent *string) map[string]interface{} {
	row := map[string]interface{}{
		"ou_id":       id,
		"handle":      handle,
		"name":        name,
		"description": description,
		"metadata":    nil,
		"created_at":  "2025-01-01 10:00:00",
		"updated_at":  "2025-01-01 10:00:00",
	}
	if parent != nil {
		row["parent_id"] = *parent
	} else {
		row["parent_id"] = nil
	}
	return row
}

func makeOUResultRowWithLogoURL(
	id, handle, name, description string, parent *string, logoURL string,
) map[string]interface{} {
	row := makeOUResultRow(id, handle, name, description, parent)
	row["metadata"] = `{"logo_url":"` + logoURL + `"}`
	return row
}

func TestBuildOrganizationUnitBasicFromResultRow(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		row := map[string]interface{}{
			"ou_id":       "ou1",
			"handle":      "root",
			"name":        "Root",
			"description": "desc",
			"created_at":  "2025-01-01 10:00:00",
			"updated_at":  "2025-01-01 10:00:00",
		}

		ou, err := buildOrganizationUnitBasicFromResultRow(row)

		require.NoError(t, err)
		require.Equal(t, "ou1", ou.ID)
		require.Equal(t, "desc", ou.Description)
	})

	t.Run("success with logo url in metadata", func(t *testing.T) {
		row := map[string]interface{}{
			"ou_id":       "ou1",
			"handle":      "root",
			"name":        "Root",
			"description": "desc",
			"metadata":    `{"logo_url":"https://example.com/logo.png"}`,
			"created_at":  "2025-01-01 10:00:00",
			"updated_at":  "2025-01-01 10:00:00",
		}

		ou, err := buildOrganizationUnitBasicFromResultRow(row)

		require.NoError(t, err)
		require.Equal(t, "ou1", ou.ID)
		require.Equal(t, "https://example.com/logo.png", ou.LogoURL)
	})

	t.Run("success with nil metadata", func(t *testing.T) {
		row := map[string]interface{}{
			"ou_id":       "ou1",
			"handle":      "root",
			"name":        "Root",
			"description": "desc",
			"metadata":    nil,
			"created_at":  "2025-01-01 10:00:00",
			"updated_at":  "2025-01-01 10:00:00",
		}

		ou, err := buildOrganizationUnitBasicFromResultRow(row)

		require.NoError(t, err)
		require.Equal(t, "ou1", ou.ID)
		require.Equal(t, "", ou.LogoURL)
	})

	t.Run("error on invalid metadata", func(t *testing.T) {
		row := map[string]interface{}{
			"ou_id":    "ou1",
			"handle":   "root",
			"name":     "Root",
			"metadata": 12345,
		}

		_, err := buildOrganizationUnitBasicFromResultRow(row)

		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to parse OU Metadata")
	})

	tests := []struct {
		name string
		row  map[string]interface{}
		want string
	}{
		{
			name: "missing ou id",
			row: map[string]interface{}{
				"name":   "Root",
				"handle": "root",
			},
			want: "ou_id is not a string",
		},
		{
			name: "missing name",
			row: map[string]interface{}{
				"ou_id":  "ou1",
				"handle": "root",
			},
			want: "name is not a string",
		},
		{
			name: "missing handle",
			row: map[string]interface{}{
				"ou_id": "ou1",
				"name":  "Root",
			},
			want: "handle is not a string",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildOrganizationUnitBasicFromResultRow(tc.row)

			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestBuildOrganizationUnitFromResultRow(t *testing.T) {
	parentID := testParentID
	row := map[string]interface{}{
		"ou_id":       "child",
		"handle":      "child",
		"name":        "Child",
		"description": "",
		"parent_id":   parentID,
		"created_at":  "2025-01-01 10:00:00",
		"updated_at":  "2025-01-01 10:00:00",
	}

	ou, err := buildOrganizationUnitFromResultRow(row)

	require.NoError(t, err)
	require.NotNil(t, ou.Parent)
	require.Equal(t, parentID, *ou.Parent)

	t.Run("with design fields", func(t *testing.T) {
		row := map[string]interface{}{
			"ou_id":       "ou1",
			"handle":      "root",
			"name":        "Root",
			"description": "desc",
			"parent_id":   nil,
			"theme_id":    "theme-abc",
			"layout_id":   "layout-def",
			"created_at":  "2025-01-01 10:00:00",
			"updated_at":  "2025-06-15 12:30:00",
			"metadata": `{"logo_url":"https://example.com/logo.png","tos_uri":""` +
				`,"policy_uri":"","cookie_policy_uri":""}`,
		}

		ou, err := buildOrganizationUnitFromResultRow(row)

		require.NoError(t, err)
		require.Nil(t, ou.Parent)
		require.Equal(t, "theme-abc", ou.ThemeID)
		require.Equal(t, "layout-def", ou.LayoutID)
		require.Equal(t, "https://example.com/logo.png", ou.LogoURL)
	})

	t.Run("invalid parent type", func(t *testing.T) {
		row := map[string]interface{}{
			"ou_id":       "ou1",
			"handle":      "root",
			"name":        "Root",
			"description": "",
			"parent_id":   123,
			"created_at":  "2025-01-01 10:00:00",
			"updated_at":  "2025-01-01 10:00:00",
		}

		ou, err := buildOrganizationUnitFromResultRow(row)

		require.NoError(t, err)
		require.Nil(t, ou.Parent)
	})

	t.Run("builder error", func(t *testing.T) {
		row := map[string]interface{}{
			"handle": "root",
			"name":   "Root",
		}

		_, err := buildOrganizationUnitFromResultRow(row)

		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to build organization unit")
	})
}

func (suite *OrganizationUnitStoreTestSuite) TestOUStore_CheckOrganizationUnitNameConflict() {
	suite.runConflictQueryScenario(
		queryCheckOrganizationUnitNameConflict,
		queryCheckOrganizationUnitNameConflictRoot,
		"Finance",
		1,
		0,
		func(parent *string) (bool, error) {
			return suite.store.CheckOrganizationUnitNameConflict(context.Background(), "Finance", parent)
		},
	)
}

func (suite *OrganizationUnitStoreTestSuite) TestOUStore_CheckOrganizationUnitHandleConflict() {
	suite.runConflictQueryScenario(
		queryCheckOrganizationUnitHandleConflict,
		queryCheckOrganizationUnitHandleConflictRoot,
		"finance",
		0,
		2,
		func(parent *string) (bool, error) {
			return suite.store.CheckOrganizationUnitHandleConflict(context.Background(), "finance", parent)
		},
	)
}

func (suite *OrganizationUnitStoreTestSuite) TestOUStore_GetOrganizationUnitChildrenCount() {
	suite.runCountQueryScenario(
		mock.Anything,
		"root",
		testDeploymentID,
		5,
		func() (int, error) {
			return suite.store.GetOrganizationUnitChildrenCount(context.Background(), "root", nil)
		},
	)

	suite.Run("build count query error", func() {
		suite.SetupTest()
		suite.expectDBClient()
		badFilter := &filter.FilterGroup{Clauses: []filter.FilterClause{
			{Expr: filter.FilterExpression{Attribute: "name", Operator: filter.Operator("co"), Value: "x"}},
		}}

		count, err := suite.store.GetOrganizationUnitChildrenCount(context.Background(), "root", badFilter)

		suite.Require().Error(err)
		suite.Zero(count)
		suite.Contains(err.Error(), "failed to build count query")
	})

	suite.Run("missing total field", func() {
		suite.SetupTest()
		suite.expectDBClient()
		suite.dbClientMock.
			On("QueryContext", mock.Anything, mock.Anything, "root", testDeploymentID).
			Return([]map[string]interface{}{{}}, nil).
			Once()

		count, err := suite.store.GetOrganizationUnitChildrenCount(context.Background(), "root", nil)

		suite.Require().Error(err)
		suite.Zero(count)
		suite.Contains(err.Error(), "failed to parse count result")
	})
}

func (suite *OrganizationUnitStoreTestSuite) TestOUStore_GetOrganizationUnitChildrenList() {
	tests := []struct {
		name    string
		parent  string
		limit   int
		offset  int
		setup   func(parent string, limit, offset int)
		assert  func(children []OrganizationUnitBasic)
		wantErr string
	}{
		{
			name:   "success",
			parent: "root",
			limit:  5,
			offset: 10,
			setup: func(parent string, limit, offset int) {
				suite.expectDBClient()
				suite.dbClientMock.
					On(
						"QueryContext", mock.Anything,
						mock.Anything, parent, limit, offset, testDeploymentID).
					Return([]map[string]interface{}{
						makeOUResultRow("child1", "child1", "Child 1", "", &parent),
						makeOUResultRow("child2", "child2", "Child 2", "desc", &parent),
					}, nil).
					Once()
			},
			assert: func(children []OrganizationUnitBasic) {
				suite.Len(children, 2)
				suite.Equal("child1", children[0].ID)
			},
		},
		{
			name:   "query error",
			parent: "root",
			limit:  1,
			offset: 0,
			setup: func(parent string, limit, offset int) {
				suite.expectDBClient()
				suite.dbClientMock.
					On(
						"QueryContext", mock.Anything,
						mock.Anything, parent, limit, offset, testDeploymentID).
					Return(nil, errors.New("query err")).
					Once()
			},
			wantErr: "failed to execute query",
		},
		{
			name:   "builder error",
			parent: "root",
			limit:  1,
			offset: 0,
			setup: func(parent string, limit, offset int) {
				suite.expectDBClient()
				suite.dbClientMock.
					On(
						"QueryContext", mock.Anything,
						mock.Anything, parent, limit, offset, testDeploymentID).
					Return([]map[string]interface{}{{"ou_id": 1}}, nil).
					Once()
			},
			wantErr: "failed to build organization unit basic",
		},
		{
			name:   "db client error",
			parent: "root",
			limit:  1,
			offset: 0,
			setup: func(parent string, limit, offset int) {
				suite.providerMock.
					On("GetUserDBClient").
					Return(nil, errors.New("db err")).
					Once()
			},
			wantErr: "failed to get database client",
		},
		{
			name:   "build list query error",
			parent: "root",
			limit:  1,
			offset: 0,
			setup: func(parent string, limit, offset int) {
				suite.expectDBClient()
			},
			wantErr: "failed to build list query",
		},
	}

	for _, tc := range tests {
		tc := tc
		suite.Run(tc.name, func() {
			suite.SetupTest()
			tc.setup(tc.parent, tc.limit, tc.offset)

			filterExpr := (*filter.FilterGroup)(nil)
			if tc.name == "build list query error" {
				filterExpr = &filter.FilterGroup{Clauses: []filter.FilterClause{
					{Expr: filter.FilterExpression{Attribute: "name", Operator: filter.Operator("co"), Value: "x"}},
				}}
			}

			children, err := suite.store.GetOrganizationUnitChildrenList(
				context.Background(), tc.parent, tc.limit, tc.offset, filterExpr,
			)

			if tc.wantErr != "" {
				suite.Require().Error(err)
				suite.Contains(err.Error(), tc.wantErr)
				return
			}

			suite.Require().NoError(err)
			if tc.assert != nil {
				tc.assert(children)
			}
		})
	}
}

func (suite *OrganizationUnitStoreTestSuite) TestOUStore_UpdateOrganizationUnit() {
	tests := []struct {
		name    string
		ou      OrganizationUnit
		setup   func(ou OrganizationUnit)
		wantErr string
	}{
		{
			name: "success",
			ou: func() OrganizationUnit {
				parent := "parent1"
				return OrganizationUnit{
					ID:          "ou1",
					Parent:      &parent,
					Handle:      "root",
					Name:        "Root",
					Description: "desc",
				}
			}(),
			setup: func(ou OrganizationUnit) {
				suite.expectDBClient()
				suite.dbClientMock.
					On(
						"ExecuteContext", mock.Anything,
						queryUpdateOrganizationUnit,
						ou.ID,
						ou.Parent,
						ou.Handle,
						ou.Name,
						ou.Description,
						ou.ThemeID,
						ou.LayoutID,
						`{"cookie_policy_uri":"","logo_url":"","policy_uri":"","tos_uri":""}`,
						mock.Anything,
						testDeploymentID,
					).
					Return(int64(1), nil).
					Once()
			},
		},
		{
			name: "success with design fields",
			ou: func() OrganizationUnit {
				parent := "parent1"
				return OrganizationUnit{
					ID:          "ou1",
					Parent:      &parent,
					Handle:      "root",
					Name:        "Root",
					Description: "desc",
					ThemeID:     "theme-123",
					LayoutID:    "layout-456",
					LogoURL:     "https://example.com/logo.png",
				}
			}(),
			setup: func(ou OrganizationUnit) {
				suite.expectDBClient()
				suite.dbClientMock.
					On(
						"ExecuteContext", mock.Anything,
						queryUpdateOrganizationUnit,
						ou.ID,
						ou.Parent,
						ou.Handle,
						ou.Name,
						ou.Description,
						ou.ThemeID,
						ou.LayoutID,
						`{"cookie_policy_uri":"","logo_url":"https://example.com/logo.png",`+
							`"policy_uri":"","tos_uri":""}`,
						mock.Anything,
						testDeploymentID,
					).
					Return(int64(1), nil).
					Once()
			},
		},
		{
			name: "execute error",
			ou:   OrganizationUnit{ID: "ou1"},
			setup: func(ou OrganizationUnit) {
				suite.expectDBClient()
				suite.dbClientMock.
					On(
						"ExecuteContext", mock.Anything,
						queryUpdateOrganizationUnit,
						ou.ID,
						ou.Parent,
						ou.Handle,
						ou.Name,
						ou.Description,
						ou.ThemeID,
						ou.LayoutID,
						`{"cookie_policy_uri":"","logo_url":"","policy_uri":"","tos_uri":""}`,
						mock.Anything,
						testDeploymentID,
					).
					Return(int64(0), errors.New("update failed")).
					Once()
			},
			wantErr: "failed to execute query",
		},
		{
			name: "db client error",
			ou:   OrganizationUnit{ID: "ou1"},
			setup: func(ou OrganizationUnit) {
				suite.providerMock.
					On("GetUserDBClient").
					Return(nil, errors.New("db err")).
					Once()
			},
			wantErr: "failed to get database client",
		},
	}

	for _, tc := range tests {
		tc := tc
		suite.Run(tc.name, func() {
			suite.SetupTest()
			tc.setup(tc.ou)

			err := suite.store.UpdateOrganizationUnit(context.Background(), tc.ou)

			if tc.wantErr != "" {
				suite.Require().Error(err)
				suite.Contains(err.Error(), tc.wantErr)
				return
			}

			suite.Require().NoError(err)
		})
	}
}

func (suite *OrganizationUnitStoreTestSuite) TestOUStore_DeleteOrganizationUnit() {
	tests := []struct {
		name    string
		setup   func()
		wantErr string
	}{
		{
			name: "success",
			setup: func() {
				suite.expectDBClient()
				suite.dbClientMock.
					On(
						"ExecuteContext", mock.Anything, queryDeleteOrganizationUnit, "ou1", testDeploymentID).
					Return(int64(1), nil).
					Once()
			},
		},
		{
			name: "execute error",
			setup: func() {
				suite.expectDBClient()
				suite.dbClientMock.
					On(
						"ExecuteContext", mock.Anything, queryDeleteOrganizationUnit, "ou1", testDeploymentID).
					Return(int64(0), errors.New("delete failed")).
					Once()
			},
			wantErr: "failed to execute query",
		},
		{
			name: "db client error",
			setup: func() {
				suite.providerMock.
					On("GetUserDBClient").
					Return(nil, errors.New("db err")).
					Once()
			},
			wantErr: "failed to get database client",
		},
	}

	for _, tc := range tests {
		tc := tc
		suite.Run(tc.name, func() {
			suite.SetupTest()
			tc.setup()

			err := suite.store.DeleteOrganizationUnit(context.Background(), "ou1")

			if tc.wantErr != "" {
				suite.Require().Error(err)
				suite.Contains(err.Error(), tc.wantErr)
				return
			}

			suite.Require().NoError(err)
		})
	}
}

func (suite *OrganizationUnitStoreTestSuite) TestOUStore_IsOrganizationUnitExists() {
	tests := []struct {
		name    string
		setup   func()
		want    bool
		wantErr string
	}{
		{
			name: "true",
			setup: func() {
				suite.expectDBClient()
				suite.dbClientMock.
					On(
						"QueryContext", mock.Anything, queryCheckOrganizationUnitExists, "ou1", testDeploymentID).
					Return([]map[string]interface{}{{"count": int64(1)}}, nil).
					Once()
			},
			want: true,
		},
		{
			name: "false on empty result",
			setup: func() {
				suite.expectDBClient()
				suite.dbClientMock.
					On(
						"QueryContext", mock.Anything, queryCheckOrganizationUnitExists, "ou1", testDeploymentID).
					Return([]map[string]interface{}{}, nil).
					Once()
			},
			want: false,
		},
		{
			name: "false on zero count",
			setup: func() {
				suite.expectDBClient()
				suite.dbClientMock.
					On(
						"QueryContext", mock.Anything, queryCheckOrganizationUnitExists, "ou1", testDeploymentID).
					Return([]map[string]interface{}{{"count": int64(0)}}, nil).
					Once()
			},
			want: false,
		},
		{
			name: "invalid type",
			setup: func() {
				suite.expectDBClient()
				suite.dbClientMock.
					On(
						"QueryContext", mock.Anything, queryCheckOrganizationUnitExists, "ou1", testDeploymentID).
					Return([]map[string]interface{}{{"count": "bad"}}, nil).
					Once()
			},
			wantErr: "failed to parse existence check result",
		},
		{
			name: "query error",
			setup: func() {
				suite.expectDBClient()
				suite.dbClientMock.
					On(
						"QueryContext", mock.Anything, queryCheckOrganizationUnitExists, "ou1", testDeploymentID).
					Return(nil, errors.New("query err")).
					Once()
			},
			wantErr: "failed to execute existence check query",
		},
		{
			name: "db client error",
			setup: func() {
				suite.providerMock.
					On("GetUserDBClient").
					Return(nil, errors.New("db fail")).
					Once()
			},
			wantErr: "failed to get database client",
		},
	}

	for _, tc := range tests {
		tc := tc
		suite.Run(tc.name, func() {
			suite.SetupTest()
			tc.setup()

			exists, err := suite.store.IsOrganizationUnitExists(context.Background(), "ou1")

			if tc.wantErr != "" {
				suite.Require().Error(err)
				suite.Contains(err.Error(), tc.wantErr)
				return
			}

			suite.Require().NoError(err)
			suite.Equal(tc.want, exists)
		})
	}
}

func (suite *OrganizationUnitStoreTestSuite) TestOUStore_GetOrganizationUnitByPath() {
	tests := []struct {
		name          string
		path          []string
		setup         func(path []string)
		assert        func(ou OrganizationUnit)
		wantErr       error
		wantErrString string
		after         func()
	}{
		{
			name: "success",
			path: []string{"root", "child"},
			setup: func(_ []string) {
				rootID := "root-id"
				childID := "child-id"
				suite.expectDBClient()
				suite.dbClientMock.
					On(
						"QueryContext", mock.Anything, queryGetRootOrganizationUnitByHandle, "root", testDeploymentID).
					Return([]map[string]interface{}{
						makeOUResultRow(rootID, "root", "Root", "desc", nil),
					}, nil).
					Once()
				suite.dbClientMock.
					On(
						"QueryContext", mock.Anything,
						queryGetOrganizationUnitByHandle, "child", rootID, testDeploymentID).
					Return([]map[string]interface{}{
						makeOUResultRow(childID, "child", "Child", "", &rootID),
					}, nil).
					Once()
			},
			assert: func(ou OrganizationUnit) {
				suite.Equal("child-id", ou.ID)
				suite.NotNil(ou.Parent)
				suite.Equal("root-id", *ou.Parent)
			},
		},
		{
			name:    "empty path",
			path:    []string{},
			wantErr: ErrOrganizationUnitNotFound,
			after: func() {
				suite.providerMock.AssertNotCalled(suite.T(), "GetUserDBClient", mock.Anything)
			},
		},
		{
			name: "db client error",
			path: []string{"root"},
			setup: func(_ []string) {
				suite.providerMock.
					On("GetUserDBClient").
					Return(nil, errors.New("db err")).
					Once()
			},
			wantErrString: "failed to get database client",
		},
		{
			name: "query error root",
			path: []string{"root"},
			setup: func(_ []string) {
				suite.expectDBClient()
				suite.dbClientMock.
					On(
						"QueryContext", mock.Anything, queryGetRootOrganizationUnitByHandle, "root", testDeploymentID).
					Return(nil, errors.New("query")).
					Once()
			},
			wantErrString: "failed to execute query for handle root",
		},
		{
			name: "not found",
			path: []string{"root"},
			setup: func(_ []string) {
				suite.expectDBClient()
				suite.dbClientMock.
					On(
						"QueryContext", mock.Anything, queryGetRootOrganizationUnitByHandle, "root", testDeploymentID).
					Return([]map[string]interface{}{}, nil).
					Once()
			},
			wantErr: ErrOrganizationUnitNotFound,
		},
		{
			name: "builder error",
			path: []string{"root"},
			setup: func(_ []string) {
				suite.expectDBClient()
				suite.dbClientMock.
					On(
						"QueryContext", mock.Anything, queryGetRootOrganizationUnitByHandle, "root", testDeploymentID).
					Return([]map[string]interface{}{{"ou_id": 1}}, nil).
					Once()
			},
			wantErrString: "failed to build organization unit for handle root",
		},
		{
			name: "child query error",
			path: []string{"root", "child"},
			setup: func(_ []string) {
				rootID := "root"
				suite.expectDBClient()
				suite.dbClientMock.
					On(
						"QueryContext", mock.Anything, queryGetRootOrganizationUnitByHandle, "root", testDeploymentID).
					Return([]map[string]interface{}{makeOUResultRow(rootID, "root", "Root", "", nil)}, nil).
					Once()
				suite.dbClientMock.
					On(
						"QueryContext", mock.Anything,
						queryGetOrganizationUnitByHandle, "child", rootID, testDeploymentID).
					Return(nil, errors.New("child query failed")).
					Once()
			},
			wantErrString: "failed to execute query for handle child",
		},
	}

	for _, tc := range tests {
		tc := tc
		suite.Run(tc.name, func() {
			suite.SetupTest()
			if tc.setup != nil {
				tc.setup(tc.path)
			}

			ou, err := suite.store.GetOrganizationUnitByPath(context.Background(), tc.path)

			switch {
			case tc.wantErr != nil:
				suite.Require().ErrorIs(err, tc.wantErr)
			case tc.wantErrString != "":
				suite.Require().Error(err)
				suite.Contains(err.Error(), tc.wantErrString)
			default:
				suite.Require().NoError(err)
				if tc.assert != nil {
					tc.assert(ou)
				}
			}

			if tc.after != nil {
				tc.after()
			}
		})
	}
}

func (suite *OrganizationUnitStoreTestSuite) TestOUStore_GetOrganizationUnit() {
	tests := []struct {
		name          string
		id            string
		setup         func(id string)
		assert        func(ou OrganizationUnit)
		wantErr       error
		wantErrString string
	}{
		{
			name: "success",
			id:   "ou1",
			setup: func(id string) {
				parentID := testParentID
				row := makeOUResultRow(id, "root", "Root", "desc", &parentID)
				suite.expectDBClient()
				suite.dbClientMock.
					On(
						"QueryContext", mock.Anything, queryGetOrganizationUnitByID, id, testDeploymentID).
					Return([]map[string]interface{}{row}, nil).
					Once()
			},
			assert: func(ou OrganizationUnit) {
				suite.Equal("ou1", ou.ID)
				suite.NotNil(ou.Parent)
				suite.Equal(testParentID, *ou.Parent)
			},
		},
		{
			name: "query error",
			id:   "ou1",
			setup: func(id string) {
				suite.expectDBClient()
				suite.dbClientMock.
					On(
						"QueryContext", mock.Anything, queryGetOrganizationUnitByID, id, testDeploymentID).
					Return(nil, errors.New("query err")).
					Once()
			},
			wantErrString: "failed to execute query",
		},
		{
			name: "not found",
			id:   "missing",
			setup: func(id string) {
				suite.expectDBClient()
				suite.dbClientMock.
					On(
						"QueryContext", mock.Anything, queryGetOrganizationUnitByID, id, testDeploymentID).
					Return([]map[string]interface{}{}, nil).
					Once()
			},
			wantErr: ErrOrganizationUnitNotFound,
		},
		{
			name: "builder error",
			id:   "ou1",
			setup: func(id string) {
				suite.expectDBClient()
				suite.dbClientMock.
					On(
						"QueryContext", mock.Anything, queryGetOrganizationUnitByID, id, testDeploymentID).
					Return([]map[string]interface{}{{"ou_id": 2}}, nil).
					Once()
			},
			wantErrString: "failed to build organization unit",
		},
		{
			name: "db client error",
			id:   "ou1",
			setup: func(id string) {
				suite.providerMock.
					On("GetUserDBClient").
					Return(nil, errors.New("db err")).
					Once()
			},
			wantErrString: "failed to get database client",
		},
	}

	for _, tc := range tests {
		tc := tc
		suite.Run(tc.name, func() {
			suite.SetupTest()
			tc.setup(tc.id)

			ou, err := suite.store.GetOrganizationUnit(context.Background(), tc.id)

			switch {
			case tc.wantErr != nil:
				suite.Require().ErrorIs(err, tc.wantErr)
			case tc.wantErrString != "":
				suite.Require().Error(err)
				suite.Contains(err.Error(), tc.wantErrString)
			default:
				suite.Require().NoError(err)
				if tc.assert != nil {
					tc.assert(ou)
				}
			}
		})
	}
}

func (suite *OrganizationUnitStoreTestSuite) TestOUStore_GetOrganizationUnitByHandle() {
	parentID := testParentID
	tests := []struct {
		name          string
		handle        string
		parent        *string
		setup         func(handle string, parent *string)
		assert        func(ou OrganizationUnit)
		wantErr       error
		wantErrString string
	}{
		{
			name:   "success root ou (nil parent)",
			handle: "root",
			parent: nil,
			setup: func(handle string, parent *string) {
				row := makeOUResultRow("ou1", handle, "Root", "desc", nil)
				suite.expectDBClient()
				suite.dbClientMock.
					On("QueryContext", mock.Anything, queryGetRootOrganizationUnitByHandle,
						handle, testDeploymentID).
					Return([]map[string]interface{}{row}, nil).
					Once()
			},
			assert: func(ou OrganizationUnit) {
				suite.Equal("ou1", ou.ID)
				suite.Equal("root", ou.Handle)
				suite.Nil(ou.Parent)
			},
		},
		{
			name:   "success child ou (non-nil parent)",
			handle: "child",
			parent: &parentID,
			setup: func(handle string, parent *string) {
				row := makeOUResultRow("ou2", handle, "Child", "desc", parent)
				suite.expectDBClient()
				suite.dbClientMock.
					On("QueryContext", mock.Anything, queryGetOrganizationUnitByHandle,
						handle, testParentID, testDeploymentID).
					Return([]map[string]interface{}{row}, nil).
					Once()
			},
			assert: func(ou OrganizationUnit) {
				suite.Equal("ou2", ou.ID)
				suite.Equal("child", ou.Handle)
			},
		},
		{
			name:   "not found",
			handle: "missing",
			parent: nil,
			setup: func(handle string, parent *string) {
				suite.expectDBClient()
				suite.dbClientMock.
					On("QueryContext", mock.Anything, queryGetRootOrganizationUnitByHandle,
						handle, testDeploymentID).
					Return([]map[string]interface{}{}, nil).
					Once()
			},
			wantErr: ErrOrganizationUnitNotFound,
		},
		{
			name:   "query error",
			handle: "root",
			parent: nil,
			setup: func(handle string, parent *string) {
				suite.expectDBClient()
				suite.dbClientMock.
					On("QueryContext", mock.Anything, queryGetRootOrganizationUnitByHandle,
						handle, testDeploymentID).
					Return(nil, errors.New("query err")).
					Once()
			},
			wantErrString: "failed to execute query for handle",
		},
		{
			name:   "builder error",
			handle: "root",
			parent: nil,
			setup: func(handle string, parent *string) {
				suite.expectDBClient()
				suite.dbClientMock.
					On("QueryContext", mock.Anything, queryGetRootOrganizationUnitByHandle,
						handle, testDeploymentID).
					Return([]map[string]interface{}{{"ou_id": 2}}, nil).
					Once()
			},
			wantErrString: "failed to build organization unit for handle",
		},
		{
			name:   "db client error",
			handle: "root",
			parent: nil,
			setup: func(handle string, parent *string) {
				suite.providerMock.
					On("GetUserDBClient").
					Return(nil, errors.New("db err")).
					Once()
			},
			wantErrString: "failed to get database client",
		},
	}

	for _, tc := range tests {
		tc := tc
		suite.Run(tc.name, func() {
			suite.SetupTest()
			tc.setup(tc.handle, tc.parent)

			ou, err := suite.store.GetOrganizationUnitByHandle(context.Background(), tc.handle, tc.parent)

			switch {
			case tc.wantErr != nil:
				suite.Require().ErrorIs(err, tc.wantErr)
			case tc.wantErrString != "":
				suite.Require().Error(err)
				suite.Contains(err.Error(), tc.wantErrString)
			default:
				suite.Require().NoError(err)
				if tc.assert != nil {
					tc.assert(ou)
				}
			}
		})
	}
}

func (suite *OrganizationUnitStoreTestSuite) TestOUStore_CreateOrganizationUnit() {
	tests := []struct {
		name    string
		ou      OrganizationUnit
		setup   func(ou OrganizationUnit)
		wantErr string
	}{
		{
			name: "success",
			ou: OrganizationUnit{
				ID:          "ou1",
				Handle:      "root",
				Name:        "Root",
				Description: "desc",
			},
			setup: func(ou OrganizationUnit) {
				suite.expectDBClient()
				suite.dbClientMock.
					On(
						"ExecuteContext", mock.Anything,
						queryCreateOrganizationUnit,
						ou.ID,
						ou.Parent,
						ou.Handle,
						ou.Name,
						ou.Description,
						ou.ThemeID,
						ou.LayoutID,
						`{"cookie_policy_uri":"","logo_url":"","policy_uri":"","tos_uri":""}`,
						testDeploymentID,
						mock.Anything,
						mock.Anything,
					).
					Return(int64(1), nil).
					Once()
			},
		},
		{
			name: "success with design fields",
			ou: OrganizationUnit{
				ID:          "ou1",
				Handle:      "root",
				Name:        "Root",
				Description: "desc",
				ThemeID:     "theme-123",
				LayoutID:    "layout-456",
				LogoURL:     "https://example.com/logo.png",
			},
			setup: func(ou OrganizationUnit) {
				suite.expectDBClient()
				suite.dbClientMock.
					On(
						"ExecuteContext", mock.Anything,
						queryCreateOrganizationUnit,
						ou.ID,
						ou.Parent,
						ou.Handle,
						ou.Name,
						ou.Description,
						ou.ThemeID,
						ou.LayoutID,
						`{"cookie_policy_uri":"","logo_url":"https://example.com/logo.png",`+
							`"policy_uri":"","tos_uri":""}`,
						testDeploymentID,
						mock.Anything,
						mock.Anything,
					).
					Return(int64(1), nil).
					Once()
			},
		},
		{
			name: "execute error",
			ou: OrganizationUnit{
				ID:          "ou-err",
				Handle:      "root",
				Name:        "Root",
				Description: "desc",
			},
			setup: func(ou OrganizationUnit) {
				suite.expectDBClient()
				suite.dbClientMock.
					On(
						"ExecuteContext", mock.Anything,
						queryCreateOrganizationUnit,
						ou.ID,
						ou.Parent,
						ou.Handle,
						ou.Name,
						ou.Description,
						ou.ThemeID,
						ou.LayoutID,
						`{"cookie_policy_uri":"","logo_url":"","policy_uri":"","tos_uri":""}`,
						testDeploymentID,
						mock.Anything,
						mock.Anything,
					).
					Return(int64(0), errors.New("insert failed")).
					Once()
			},
			wantErr: "failed to execute query",
		},
		{
			name: "db client error",
			ou:   OrganizationUnit{ID: "ou1"},
			setup: func(ou OrganizationUnit) {
				suite.providerMock.
					On("GetUserDBClient").
					Return(nil, errors.New("db init failed")).
					Once()
			},
			wantErr: "failed to get database client",
		},
	}

	for _, tc := range tests {
		tc := tc
		suite.Run(tc.name, func() {
			suite.SetupTest()
			if tc.setup != nil {
				tc.setup(tc.ou)
			}

			err := suite.store.CreateOrganizationUnit(context.Background(), tc.ou)

			if tc.wantErr != "" {
				suite.Require().Error(err)
				suite.Contains(err.Error(), tc.wantErr)
				return
			}

			suite.Require().NoError(err)
		})
	}
}

func (suite *OrganizationUnitStoreTestSuite) TestOUStore_GetOrganizationUnitList() {
	tests := []struct {
		name          string
		limit         int
		offset        int
		setup         func(limit, offset int)
		assert        func(ous []OrganizationUnitBasic)
		wantErrString string
	}{
		{
			name:   "success",
			limit:  2,
			offset: 0,
			setup: func(limit, offset int) {
				suite.expectDBClient()
				rows := []map[string]interface{}{
					makeOUResultRowWithLogoURL(
						"root", "root", "Root", "desc", nil, "https://example.com/root-logo.png"),
					makeOUResultRow("child", "child", "Child", "", nil),
				}
				suite.dbClientMock.
					On(
						"QueryContext", mock.Anything,
						mock.Anything, limit, offset, testDeploymentID).
					Return(rows, nil).
					Once()
			},
			assert: func(ous []OrganizationUnitBasic) {
				suite.Len(ous, 2)
				suite.Equal("root", ous[0].ID)
				suite.Equal("https://example.com/root-logo.png", ous[0].LogoURL)
				suite.Equal("child", ous[1].Handle)
				suite.Equal("", ous[1].LogoURL)
			},
		},
		{
			name:   "query error",
			limit:  10,
			offset: 5,
			setup: func(limit, offset int) {
				suite.expectDBClient()
				suite.dbClientMock.
					On(
						"QueryContext", mock.Anything,
						mock.Anything, limit, offset, testDeploymentID).
					Return(nil, errors.New("query error")).
					Once()
			},
			wantErrString: "failed to execute query",
		},
		{
			name:   "builder error",
			limit:  1,
			offset: 0,
			setup: func(limit, offset int) {
				suite.expectDBClient()
				suite.dbClientMock.
					On(
						"QueryContext", mock.Anything,
						mock.Anything, limit, offset, testDeploymentID).
					Return([]map[string]interface{}{{"ou_id": 123}}, nil).
					Once()
			},
			wantErrString: "failed to build organization unit basic",
		},
		{
			name:   "db client error",
			limit:  1,
			offset: 0,
			setup: func(limit, offset int) {
				suite.providerMock.
					On("GetUserDBClient").
					Return(nil, errors.New("db err")).
					Once()
			},
			wantErrString: "failed to get database client",
		},
		{
			name:   "build list query error",
			limit:  1,
			offset: 0,
			setup: func(limit, offset int) {
				suite.expectDBClient()
			},
			wantErrString: "failed to build list query",
		},
	}

	for _, tc := range tests {
		tc := tc
		suite.Run(tc.name, func() {
			suite.SetupTest()
			tc.setup(tc.limit, tc.offset)

			filterExpr := (*filter.FilterGroup)(nil)
			if tc.name == "build list query error" {
				filterExpr = &filter.FilterGroup{Clauses: []filter.FilterClause{
					{Expr: filter.FilterExpression{Attribute: "name", Operator: filter.Operator("co"), Value: "x"}},
				}}
			}

			ous, err := suite.store.GetOrganizationUnitList(context.Background(), tc.limit, tc.offset, filterExpr)

			if tc.wantErrString != "" {
				suite.Require().Error(err)
				suite.Nil(ous)
				suite.Contains(err.Error(), tc.wantErrString)
				return
			}

			suite.Require().NoError(err)
			if tc.assert != nil {
				tc.assert(ous)
			}
		})
	}
}

func (suite *OrganizationUnitStoreTestSuite) TestOUStore_GetOrganizationUnitListCount() {
	tests := []struct {
		name    string
		setup   func()
		want    int
		wantErr string
	}{
		{
			name: "success",
			setup: func() {
				suite.expectDBClient()
				suite.dbClientMock.
					On(
						"QueryContext", mock.Anything, mock.Anything, testDeploymentID).
					Return([]map[string]interface{}{{"total": int64(3)}}, nil).
					Once()
			},
			want: 3,
		},
		{
			name: "query error",
			setup: func() {
				suite.expectDBClient()
				suite.dbClientMock.
					On(
						"QueryContext", mock.Anything, mock.Anything, testDeploymentID).
					Return(nil, errors.New("boom")).
					Once()
			},
			wantErr: "failed to execute count query",
		},
		{
			name: "unexpected type",
			setup: func() {
				suite.expectDBClient()
				suite.dbClientMock.
					On(
						"QueryContext", mock.Anything, mock.Anything, testDeploymentID).
					Return([]map[string]interface{}{{"total": "3"}}, nil).
					Once()
			},
			wantErr: "unexpected type for total",
		},
		{
			name: "db client error",
			setup: func() {
				suite.providerMock.
					On("GetUserDBClient").
					Return(nil, errors.New("no db")).
					Once()
			},
			wantErr: "failed to get database client",
		},
		{
			name: "build count query error",
			setup: func() {
				suite.expectDBClient()
			},
			wantErr: "failed to build count query",
		},
		{
			name: "empty result",
			setup: func() {
				suite.expectDBClient()
				suite.dbClientMock.
					On(
						"QueryContext", mock.Anything, mock.Anything, testDeploymentID).
					Return([]map[string]interface{}{}, nil).
					Once()
			},
			want: 0,
		},
	}

	for _, tc := range tests {
		tc := tc
		suite.Run(tc.name, func() {
			suite.SetupTest()
			tc.setup()

			filterExpr := (*filter.FilterGroup)(nil)
			if tc.name == "build count query error" {
				filterExpr = &filter.FilterGroup{Clauses: []filter.FilterClause{
					{Expr: filter.FilterExpression{Attribute: "name", Operator: filter.Operator("co"), Value: "x"}},
				}}
			}

			count, err := suite.store.GetOrganizationUnitListCount(context.Background(), filterExpr)

			if tc.wantErr != "" {
				suite.Require().Error(err)
				suite.Zero(count)
				suite.Contains(err.Error(), tc.wantErr)
				return
			}

			suite.Require().NoError(err)
			suite.Equal(tc.want, count)
		})
	}
}

func (suite *OrganizationUnitStoreTestSuite) TestOUStore_GetOrganizationUnitsByIDs() {
	tests := []struct {
		name          string
		ids           []string
		setup         func(ids []string)
		assert        func(ous []OrganizationUnitBasic)
		wantErrString string
	}{
		{
			name: "success",
			ids:  []string{"ou1", "ou2"},
			setup: func(ids []string) {
				suite.expectDBClient()
				rows := []map[string]interface{}{
					makeOUResultRowWithLogoURL(
						"ou1", "root1", "Root1", "desc1", nil, "https://example.com/ou1-logo.png"),
					makeOUResultRow("ou2", "root2", "Root2", "desc2", nil),
				}
				suite.dbClientMock.
					On(
						"QueryContext", mock.Anything,
						mock.AnythingOfType("model.DBQuery"), mock.Anything, mock.Anything, mock.Anything).
					Return(rows, nil).
					Once()
			},
			assert: func(ous []OrganizationUnitBasic) {
				suite.Len(ous, 2)
				suite.Equal("ou1", ous[0].ID)
				suite.Equal("https://example.com/ou1-logo.png", ous[0].LogoURL)
				suite.Equal("ou2", ous[1].ID)
				suite.Equal("", ous[1].LogoURL)
			},
		},
		{
			name: "empty ids",
			ids:  []string{},
			assert: func(ous []OrganizationUnitBasic) {
				suite.Len(ous, 0)
			},
		},
		{
			name: "query error",
			ids:  []string{"ou1"},
			setup: func(ids []string) {
				suite.expectDBClient()
				suite.dbClientMock.
					On(
						"QueryContext", mock.Anything,
						mock.AnythingOfType("model.DBQuery"), mock.Anything, mock.Anything).
					Return(nil, errors.New("query error")).
					Once()
			},
			wantErrString: "failed to execute query",
		},
		{
			name: "builder error",
			ids:  []string{"ou1"},
			setup: func(ids []string) {
				suite.expectDBClient()
				suite.dbClientMock.
					On(
						"QueryContext", mock.Anything,
						mock.AnythingOfType("model.DBQuery"), mock.Anything, mock.Anything).
					Return([]map[string]interface{}{{"ou_id": 123}}, nil).
					Once()
			},
			wantErrString: "failed to build organization unit basic",
		},
		{
			name: "db client error",
			ids:  []string{"ou1"},
			setup: func(ids []string) {
				suite.providerMock.
					On("GetUserDBClient").
					Return(nil, errors.New("db err")).
					Once()
			},
			wantErrString: "failed to get database client",
		},
	}

	for _, tc := range tests {
		tc := tc
		suite.Run(tc.name, func() {
			suite.SetupTest()
			if tc.setup != nil {
				tc.setup(tc.ids)
			}

			ous, err := suite.store.GetOrganizationUnitsByIDs(context.Background(), tc.ids)

			if tc.wantErrString != "" {
				suite.Require().Error(err)
				suite.Nil(ous)
				suite.Contains(err.Error(), tc.wantErrString)
				return
			}

			suite.Require().NoError(err)
			if tc.assert != nil {
				tc.assert(ous)
			}
		})
	}
}

func (suite *OrganizationUnitStoreTestSuite) TestOUStore_IsOrganizationUnitDeclarative() {
	suite.Run("returns false", func() {
		suite.SetupTest()
		res := suite.store.IsOrganizationUnitDeclarative(context.Background(), "ou1")
		suite.Require().False(res)
	})
}

func (suite *OrganizationUnitStoreTestSuite) TestOUStore_buildGetOrganizationUnitsByIDsQuery() {
	suite.Run("builds query with correct placeholders", func() {
		ids := []string{"id1", "id2", "id3"}
		query := buildGetOrganizationUnitsByIDsQuery(ids)

		suite.Require().Equal("OUQ-OU_MGT-21", query.ID)
		suite.Require().Contains(query.PostgresQuery, "METADATA")
		suite.Require().Contains(query.PostgresQuery, "$1, $2, $3")
		suite.Require().Contains(query.PostgresQuery, "DEPLOYMENT_ID = $4")
		suite.Require().Contains(query.SQLiteQuery, "METADATA")
		suite.Require().Contains(query.SQLiteQuery, "?, ?, ?")
		suite.Require().Contains(query.SQLiteQuery, "DEPLOYMENT_ID = ?")
	})
}

func TestNewOrganizationUnitStore_TransactionerError(t *testing.T) {
	mockProvider := providermock.NewDBProviderInterfaceMock(t)
	mockProvider.On("GetUserDBTransactioner").Return(nil, errors.New("transactioner error"))

	originalGetDBProvider := getDBProvider
	getDBProvider = func() provider.DBProviderInterface { return mockProvider }
	defer func() { getDBProvider = originalGetDBProvider }()

	_, _, err := newOrganizationUnitStore()
	require.Error(t, err)
}

func TestParseTimeField(t *testing.T) {
	t.Run("parses custom time with suffix", func(t *testing.T) {
		parsed, err := parseTimeField("2025-01-01 10:00:00.123456789 +0000 UTC", "created_at")

		require.NoError(t, err)
		require.Equal(t, 2025, parsed.Year())
		require.Equal(t, time.January, parsed.Month())
		require.Equal(t, 1, parsed.Day())
	})

	t.Run("parses rfc3339", func(t *testing.T) {
		parsed, err := parseTimeField("2025-01-01T10:00:00Z", "updated_at")

		require.NoError(t, err)
		require.Equal(t, 10, parsed.Hour())
	})

	t.Run("accepts time type", func(t *testing.T) {
		now := time.Now().UTC()
		parsed, err := parseTimeField(now, "created_at")

		require.NoError(t, err)
		require.Equal(t, now, parsed)
	})

	t.Run("errors on invalid string", func(t *testing.T) {
		_, err := parseTimeField("bad-time", "created_at")

		require.Error(t, err)
		require.Contains(t, err.Error(), "error parsing created_at")
	})

	t.Run("errors on nil", func(t *testing.T) {
		_, err := parseTimeField(nil, "created_at")

		require.Error(t, err)
		require.Contains(t, err.Error(), "created_at is nil")
	})

	t.Run("errors on unsupported type", func(t *testing.T) {
		_, err := parseTimeField(42, "created_at")

		require.Error(t, err)
		require.Contains(t, err.Error(), "unexpected type for created_at")
	})
}

func TestTrimTimeString(t *testing.T) {
	require.Equal(t, "2025-01-01 10:00:00.123456789", trimTimeString("2025-01-01 10:00:00.123456789 +0000 UTC"))
	require.Equal(t, "2025-01-01T10:00:00Z", trimTimeString("2025-01-01T10:00:00Z"))
}

func TestParseOUMetadata(t *testing.T) {
	t.Run("missing metadata key", func(t *testing.T) {
		data, err := parseOUMetadata(map[string]interface{}{})

		require.NoError(t, err)
		require.Empty(t, data)
	})

	t.Run("nil metadata", func(t *testing.T) {
		data, err := parseOUMetadata(map[string]interface{}{"metadata": nil})

		require.NoError(t, err)
		require.Empty(t, data)
	})

	t.Run("empty metadata string", func(t *testing.T) {
		data, err := parseOUMetadata(map[string]interface{}{"metadata": ""})

		require.NoError(t, err)
		require.Empty(t, data)
	})

	t.Run("metadata as bytes", func(t *testing.T) {
		data, err := parseOUMetadata(map[string]interface{}{
			"metadata": []byte(`{"logo_url":"https://example.com/logo.png"}`),
		})

		require.NoError(t, err)
		require.Equal(t, "https://example.com/logo.png", data["logo_url"])
	})

	t.Run("invalid metadata type", func(t *testing.T) {
		_, err := parseOUMetadata(map[string]interface{}{"metadata": 123})

		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to parse metadata as string or []byte")
	})

	t.Run("invalid metadata json", func(t *testing.T) {
		_, err := parseOUMetadata(map[string]interface{}{"metadata": "{"})

		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to unmarshal OU Metadata")
	})
}

func TestExtractStringFromOUMetadata(t *testing.T) {
	t.Run("returns empty for missing key", func(t *testing.T) {
		value, err := extractStringFromOUMetadata(map[string]interface{}{}, "logo_url")

		require.NoError(t, err)
		require.Equal(t, "", value)
	})

	t.Run("returns empty for nil key", func(t *testing.T) {
		value, err := extractStringFromOUMetadata(map[string]interface{}{"logo_url": nil}, "logo_url")

		require.NoError(t, err)
		require.Equal(t, "", value)
	})

	t.Run("returns value for string", func(t *testing.T) {
		value, err := extractStringFromOUMetadata(
			map[string]interface{}{"logo_url": "https://example.com/logo.png"},
			"logo_url",
		)

		require.NoError(t, err)
		require.Equal(t, "https://example.com/logo.png", value)
	})

	t.Run("errors on non string", func(t *testing.T) {
		_, err := extractStringFromOUMetadata(map[string]interface{}{"logo_url": 1}, "logo_url")

		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to parse logo_url from OU Metadata")
	})
}

func TestBuildOrganizationUnitBasicFromResultRow_MetadataAndTimeErrors(t *testing.T) {
	t.Run("metadata field type error", func(t *testing.T) {
		row := map[string]interface{}{
			"ou_id":       "ou1",
			"handle":      "root",
			"name":        "Root",
			"description": "desc",
			"metadata":    `{"logo_url":1}`,
			"created_at":  "2025-01-01 10:00:00",
			"updated_at":  "2025-01-01 10:00:00",
		}

		_, err := buildOrganizationUnitBasicFromResultRow(row)

		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to parse logo_url from OU Metadata")
	})

	t.Run("created_at parse error", func(t *testing.T) {
		row := map[string]interface{}{
			"ou_id":       "ou1",
			"handle":      "root",
			"name":        "Root",
			"description": "desc",
			"metadata":    nil,
			"created_at":  nil,
			"updated_at":  "2025-01-01 10:00:00",
		}

		_, err := buildOrganizationUnitBasicFromResultRow(row)

		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to parse created_at")
	})

	t.Run("updated_at parse error", func(t *testing.T) {
		row := map[string]interface{}{
			"ou_id":       "ou1",
			"handle":      "root",
			"name":        "Root",
			"description": "desc",
			"metadata":    nil,
			"created_at":  "2025-01-01 10:00:00",
			"updated_at":  true,
		}

		_, err := buildOrganizationUnitBasicFromResultRow(row)

		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to parse updated_at")
	})
}

func TestBuildOrganizationUnitFromResultRow_MetadataFieldErrors(t *testing.T) {
	tests := []struct {
		name     string
		metadata string
		wantErr  string
	}{
		{
			name:     "tos_uri type error",
			metadata: `{"logo_url":"https://example.com/logo.png","tos_uri":1}`,
			wantErr:  "failed to parse tos_uri from OU Metadata",
		},
		{
			name:     "policy_uri type error",
			metadata: `{"logo_url":"https://example.com/logo.png","tos_uri":"","policy_uri":1}`,
			wantErr:  "failed to parse policy_uri from OU Metadata",
		},
		{
			name:     "cookie_policy_uri type error",
			metadata: `{"logo_url":"https://example.com/logo.png","tos_uri":"","policy_uri":"","cookie_policy_uri":1}`,
			wantErr:  "failed to parse cookie_policy_uri from OU Metadata",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			row := map[string]interface{}{
				"ou_id":       "ou1",
				"handle":      "root",
				"name":        "Root",
				"description": "desc",
				"metadata":    tc.metadata,
				"created_at":  "2025-01-01 10:00:00",
				"updated_at":  "2025-01-01 10:00:00",
			}

			_, err := buildOrganizationUnitFromResultRow(row)

			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestBuildOrganizationUnitFromResultRow_NonStringThemeAndLayout(t *testing.T) {
	row := map[string]interface{}{
		"ou_id":       "ou1",
		"handle":      "root",
		"name":        "Root",
		"description": "desc",
		"parent_id":   nil,
		"theme_id":    123,
		"layout_id":   true,
		"metadata":    []byte(`{"logo_url":"https://example.com/logo.png"}`),
		"created_at":  "2025-01-01 10:00:00",
		"updated_at":  "2025-01-01 10:00:00",
	}

	ou, err := buildOrganizationUnitFromResultRow(row)

	require.NoError(t, err)
	require.Equal(t, "", ou.ThemeID)
	require.Equal(t, "", ou.LayoutID)
	require.Equal(t, "https://example.com/logo.png", ou.LogoURL)
}

func TestBuildOUFilterGroup(t *testing.T) {
	sg := func(attr string, op filter.Operator, val interface{}) *filter.FilterGroup {
		return &filter.FilterGroup{Clauses: []filter.FilterClause{
			{Expr: filter.FilterExpression{Attribute: attr, Operator: op, Value: val}},
		}}
	}
	twoClause := func(
		attr1 string, op1 filter.Operator, val1 interface{},
		conn filter.LogicalOperator,
		attr2 string, op2 filter.Operator, val2 interface{},
	) *filter.FilterGroup {
		return &filter.FilterGroup{Clauses: []filter.FilterClause{
			{Expr: filter.FilterExpression{Attribute: attr1, Operator: op1, Value: val1}},
			{Connector: conn, Expr: filter.FilterExpression{Attribute: attr2, Operator: op2, Value: val2}},
		}}
	}

	tests := []struct {
		name      string
		g         *filter.FilterGroup
		startIdx  int
		wantCond  string
		wantArgs  []interface{}
		wantError string
	}{
		{
			name:     "eq on text column uses LOWER",
			g:        sg("name", filter.OperatorEq, "Finance"),
			startIdx: 2,
			wantCond: " AND LOWER(NAME) = LOWER($2)",
			wantArgs: []interface{}{"Finance"},
		},
		{
			name:     "eq on timestamp column uses plain equals",
			g:        sg("createdAt", filter.OperatorEq, "2025-01-01"),
			startIdx: 3,
			wantCond: " AND CREATED_AT = $3",
			wantArgs: []interface{}{"2025-01-01"},
		},
		{
			name:     "gt operator",
			g:        sg("createdAt", filter.OperatorGt, "2025-01-01"),
			startIdx: 4,
			wantCond: " AND CREATED_AT > $4",
			wantArgs: []interface{}{"2025-01-01"},
		},
		{
			name:     "lt operator",
			g:        sg("updatedAt", filter.OperatorLt, "2026-01-01"),
			startIdx: 5,
			wantCond: " AND UPDATED_AT < $5",
			wantArgs: []interface{}{"2026-01-01"},
		},
		{
			name: "two AND clauses wrapped in parens",
			g: twoClause(
				"name", filter.OperatorEq, "Eng", filter.LogicalAnd, "createdAt", filter.OperatorGt, "2024"),
			startIdx: 2,
			wantCond: " AND (LOWER(NAME) = LOWER($2) AND CREATED_AT > $3)",
			wantArgs: []interface{}{"Eng", "2024"},
		},
		{
			name:     "two OR clauses wrapped in parens",
			g:        twoClause("name", filter.OperatorEq, "A", filter.LogicalOr, "handle", filter.OperatorEq, "a"),
			startIdx: 2,
			wantCond: " AND (LOWER(NAME) = LOWER($2) OR LOWER(HANDLE) = LOWER($3))",
			wantArgs: []interface{}{"A", "a"},
		},
		{
			name:      "non filterable attribute",
			g:         sg("id", filter.OperatorEq, "ou1"),
			startIdx:  2,
			wantError: `attribute "id" is not filterable`,
		},
		{
			name:      "unsupported operator",
			g:         sg("name", filter.Operator("co"), "Finance"),
			startIdx:  2,
			wantError: `unsupported operator "co"`,
		},
		{
			name:     "nil group returns empty cond and nil args",
			g:        nil,
			startIdx: 2,
			wantCond: "",
			wantArgs: nil,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			cond, args, err := buildOUFilterGroup(tc.g, tc.startIdx)

			if tc.wantError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantError)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.wantCond, cond)
			require.Equal(t, tc.wantArgs, args)
		})
	}
}

func TestBuildOUCountQueries(t *testing.T) {
	type countQueryCase struct {
		name           string
		buildFn        func(*filter.FilterGroup) (dbmodel.DBQuery, []interface{}, error)
		queryID        string
		noFilterClause string
		withFilter     *filter.FilterGroup
		wantClause     string
		wantArgs       []interface{}
	}

	cases := []countQueryCase{
		{
			name:           "root OU count",
			buildFn:        buildRootOUCountQuery,
			queryID:        "OUQ-OU_MGT-01",
			noFilterClause: "PARENT_ID IS NULL AND DEPLOYMENT_ID = $1",
			withFilter: &filter.FilterGroup{Clauses: []filter.FilterClause{
				{Expr: filter.FilterExpression{Attribute: "name", Operator: filter.OperatorEq, Value: "Finance"}},
			}},
			wantClause: "LOWER(NAME) = LOWER($2)",
			wantArgs:   []interface{}{"Finance"},
		},
		{
			name:           "children OU count",
			buildFn:        buildChildrenOUCountQuery,
			queryID:        "OUQ-OU_MGT-10",
			noFilterClause: "PARENT_ID = $1 AND DEPLOYMENT_ID = $2",
			withFilter: &filter.FilterGroup{Clauses: []filter.FilterClause{
				{Expr: filter.FilterExpression{Attribute: "description", Operator: filter.OperatorEq, Value: "desc"}},
			}},
			wantClause: "LOWER(DESCRIPTION) = LOWER($3)",
			wantArgs:   []interface{}{"desc"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name+"/without filter", func(t *testing.T) {
			q, args, err := tc.buildFn(nil)
			require.NoError(t, err)
			require.Equal(t, tc.queryID, q.ID)
			require.Contains(t, q.Query, tc.noFilterClause)
			require.Empty(t, args)
		})
		t.Run(tc.name+"/with filter", func(t *testing.T) {
			q, args, err := tc.buildFn(tc.withFilter)
			require.NoError(t, err)
			require.Contains(t, q.Query, tc.wantClause)
			require.Equal(t, tc.wantArgs, args)
		})
		t.Run(tc.name+"/filter error", func(t *testing.T) {
			badF := &filter.FilterGroup{Clauses: []filter.FilterClause{
				{Expr: filter.FilterExpression{Attribute: "name", Operator: filter.Operator("co"), Value: "x"}},
			}}
			_, _, err := tc.buildFn(badF)
			require.Error(t, err)
			require.Contains(t, err.Error(), "unsupported operator")
		})
	}
}

func TestBuildRootOUListQuery(t *testing.T) {
	t.Run("without filter", func(t *testing.T) {
		q, args, err := buildRootOUListQuery(nil)

		require.NoError(t, err)
		require.Equal(t, "OUQ-OU_MGT-02", q.ID)
		require.Contains(t, q.Query, "PARENT_ID IS NULL AND DEPLOYMENT_ID = $3")
		require.Contains(t, q.Query, "ORDER BY NAME LIMIT $1 OFFSET $2")
		require.Empty(t, args)
	})

	t.Run("with filter", func(t *testing.T) {
		f := &filter.FilterGroup{Clauses: []filter.FilterClause{
			{Expr: filter.FilterExpression{Attribute: "handle", Operator: filter.OperatorEq, Value: "root"}},
		}}
		q, args, err := buildRootOUListQuery(f)

		require.NoError(t, err)
		require.Contains(t, q.Query, "LOWER(HANDLE) = LOWER($4)")
		require.Equal(t, []interface{}{"root"}, args)
	})

	t.Run("filter error", func(t *testing.T) {
		f := &filter.FilterGroup{Clauses: []filter.FilterClause{
			{Expr: filter.FilterExpression{Attribute: "invalid", Operator: filter.OperatorEq, Value: "x"}},
		}}
		_, _, err := buildRootOUListQuery(f)

		require.Error(t, err)
		require.Contains(t, err.Error(), "is not filterable")
	})
}

func TestBuildChildrenOUCountQuery(t *testing.T) {
	t.Run("without filter", func(t *testing.T) {
		q, args, err := buildChildrenOUCountQuery(nil)

		require.NoError(t, err)
		require.Equal(t, "OUQ-OU_MGT-10", q.ID)
		require.Contains(t, q.Query, "PARENT_ID = $1 AND DEPLOYMENT_ID = $2")
		require.Empty(t, args)
	})

	t.Run("with filter", func(t *testing.T) {
		f := &filter.FilterGroup{Clauses: []filter.FilterClause{
			{Expr: filter.FilterExpression{Attribute: "description", Operator: filter.OperatorEq, Value: "desc"}},
		}}
		q, args, err := buildChildrenOUCountQuery(f)

		require.NoError(t, err)
		require.Contains(t, q.Query, "LOWER(DESCRIPTION) = LOWER($3)")
		require.Equal(t, []interface{}{"desc"}, args)
	})

	t.Run("filter error", func(t *testing.T) {
		f := &filter.FilterGroup{Clauses: []filter.FilterClause{
			{Expr: filter.FilterExpression{Attribute: "name", Operator: filter.Operator("co"), Value: "Finance"}},
		}}
		_, _, err := buildChildrenOUCountQuery(f)

		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported operator")
	})
}

func TestBuildChildrenOUListQuery(t *testing.T) {
	t.Run("without filter", func(t *testing.T) {
		q, args, err := buildChildrenOUListQuery(nil)

		require.NoError(t, err)
		require.Equal(t, "OUQ-OU_MGT-11", q.ID)
		require.Contains(t, q.Query, "PARENT_ID = $1 AND DEPLOYMENT_ID = $4")
		require.Contains(t, q.Query, "ORDER BY NAME LIMIT $2 OFFSET $3")
		require.Empty(t, args)
	})

	t.Run("with filter", func(t *testing.T) {
		f := &filter.FilterGroup{Clauses: []filter.FilterClause{
			{Expr: filter.FilterExpression{Attribute: "createdAt", Operator: filter.OperatorGt, Value: "2025-01-01"}},
		}}
		q, args, err := buildChildrenOUListQuery(f)

		require.NoError(t, err)
		require.Contains(t, q.Query, "AND CREATED_AT > $5")
		require.Equal(t, []interface{}{"2025-01-01"}, args)
	})

	t.Run("filter error", func(t *testing.T) {
		f := &filter.FilterGroup{Clauses: []filter.FilterClause{
			{Expr: filter.FilterExpression{Attribute: "updatedAt", Operator: filter.Operator("co"), Value: "x"}},
		}}
		_, _, err := buildChildrenOUListQuery(f)

		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported operator")
	})
}
