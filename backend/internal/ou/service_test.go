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

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/filter"
	i18ncore "github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

type OrganizationUnitServiceTestSuite struct {
	suite.Suite
}

var testParentID = "parent"
var testOUID = "ou-1"
var testMidID = "mid-1"
var testGrandID = "grand"

func TestOUService_OrganizationUnitServiceTestSuite_Run(t *testing.T) {
	suite.Run(t, new(OrganizationUnitServiceTestSuite))
}

func (suite *OrganizationUnitServiceTestSuite) SetupTest() {
	// Initialize server runtime with declarative mode disabled by default
	config.ResetServerRuntime()
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)
}

func (suite *OrganizationUnitServiceTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

type ouListExpectations struct {
	totalResults int
	count        int
	startIndex   int
	handles      []string
	linkRels     []string
	linkHrefs    []string
}

type pathListTestConfig[Resp any] struct {
	invalidPath    string
	validPath      string
	validPathSlice []string
	limit          int
	offset         int
	setupSuccess   func(*organizationUnitStoreInterfaceMock)
	assertSuccess  func(Resp)
	invoke         func(*organizationUnitService, string, int, int) (Resp, *serviceerror.ServiceError)
}

type pathListInvoker[Resp any] func(*organizationUnitService, string, int, int) (Resp, *serviceerror.ServiceError)

// runOUPathListTests de-duplicates the repeated path-based list scenarios across children/users/groups.
func runOUPathListTests[Resp any](suite *OrganizationUnitServiceTestSuite, cfg pathListTestConfig[Resp]) {
	suite.Run("invalid path", func() {
		store := newOrganizationUnitStoreInterfaceMock(suite.T())
		service := suite.newService(store, newAllowAllAuthz(suite.T()))

		resp, err := cfg.invoke(service, cfg.invalidPath, cfg.limit, cfg.offset)

		suite.Require().Nil(resp)
		suite.Require().Equal(ErrorInvalidHandlePath, *err)
		store.AssertNumberOfCalls(suite.T(), "GetOrganizationUnitByPath", 0)
	})

	suite.Run("not found", func() {
		store := newOrganizationUnitStoreInterfaceMock(suite.T())
		store.On("GetOrganizationUnitByPath", mock.Anything, cfg.validPathSlice).
			Return(OrganizationUnit{}, ErrOrganizationUnitNotFound).
			Once()

		service := suite.newService(store, newAllowAllAuthz(suite.T()))
		resp, err := cfg.invoke(service, cfg.validPath, cfg.limit, cfg.offset)

		suite.Require().Nil(resp)
		suite.Require().Equal(ErrorOrganizationUnitNotFound, *err)
	})

	suite.Run("store error", func() {
		store := newOrganizationUnitStoreInterfaceMock(suite.T())
		store.On("GetOrganizationUnitByPath", mock.Anything, cfg.validPathSlice).
			Return(OrganizationUnit{}, errors.New("boom")).
			Once()

		service := suite.newService(store, newAllowAllAuthz(suite.T()))
		resp, err := cfg.invoke(service, cfg.validPath, cfg.limit, cfg.offset)

		suite.Require().Nil(resp)
		suite.Require().Equal(serviceerror.InternalServerError, *err)
	})

	suite.Run("success", func() {
		store := newOrganizationUnitStoreInterfaceMock(suite.T())
		if cfg.setupSuccess != nil {
			cfg.setupSuccess(store)
		}

		service := suite.newService(store, newAllowAllAuthz(suite.T()))
		resp, err := cfg.invoke(service, cfg.validPath, cfg.limit, cfg.offset)

		suite.Require().Nil(err)
		if cfg.assertSuccess != nil {
			cfg.assertSuccess(resp)
		} else {
			suite.Require().NotNil(resp)
		}
	})
}

func setupDefaultPathSuccess(
	store *organizationUnitStoreInterfaceMock,
	limit, offset int,
	listMethod string,
	listReturn interface{},
	countMethod string,
	countReturn interface{},
	extraArgs ...interface{},
) {
	store.On("GetOrganizationUnitByPath", mock.Anything, []string{"root"}).
		Return(OrganizationUnit{ID: "ou-1"}, nil).
		Once()
	store.On("IsOrganizationUnitExists", mock.Anything, "ou-1").
		Return(true, nil).
		Once()
	listArgs := []interface{}{mock.Anything, "ou-1", limit, offset}
	listArgs = append(listArgs, extraArgs...)
	store.On(listMethod, listArgs...).
		Return(listReturn, nil).
		Once()
	countArgs := []interface{}{mock.Anything, "ou-1"}
	countArgs = append(countArgs, extraArgs...)
	store.On(countMethod, countArgs...).
		Return(countReturn, nil).
		Once()
}

func newDefaultPathListConfig[Resp any](
	invalidPath string,
	limit, offset int,
	listMethod string,
	listReturn interface{},
	countMethod string,
	countReturn interface{},
	assert func(Resp),
	invoker pathListInvoker[Resp],
	extraListArgs ...interface{},
) pathListTestConfig[Resp] {
	return pathListTestConfig[Resp]{
		invalidPath:    invalidPath,
		validPath:      "root",
		validPathSlice: []string{"root"},
		limit:          limit,
		offset:         offset,
		setupSuccess: func(store *organizationUnitStoreInterfaceMock) {
			setupDefaultPathSuccess(
				store, limit, offset, listMethod, listReturn,
				countMethod, countReturn, extraListArgs...)
		},
		assertSuccess: assert,
		invoke:        invoker,
	}
}

func invokeChildrenByPath(
	service *organizationUnitService,
	path string,
	limit, offset int,
) (*OrganizationUnitListResponse, *serviceerror.ServiceError) {
	return service.GetOrganizationUnitChildrenByPath(context.Background(), path, limit, offset, nil)
}

func (suite *OrganizationUnitServiceTestSuite) newService(
	store *organizationUnitStoreInterfaceMock,
	authzService *sysauthzmock.SystemAuthorizationServiceInterfaceMock,
) *organizationUnitService {
	return suite.newServiceWithResolvers(store, authzService, nil, nil)
}

func (suite *OrganizationUnitServiceTestSuite) newServiceWithResolvers(
	store *organizationUnitStoreInterfaceMock,
	authzService *sysauthzmock.SystemAuthorizationServiceInterfaceMock,
	userResolver OUUserResolver,
	groupResolver OUGroupResolver,
) *organizationUnitService {
	mtx := new(mockTransactioner)
	mtx.On("Transact", mock.Anything, mock.Anything).Return(nil).Maybe()
	return &organizationUnitService{
		ouStore:       store,
		authzService:  authzService,
		transactioner: mtx,
		userResolver:  userResolver,
		groupResolver: groupResolver,
	}
}

type mockTransactioner struct {
	mock.Mock
}

func (m *mockTransactioner) Transact(ctx context.Context, fn func(context.Context) error) error {
	args := m.Called(ctx, fn)
	if args.Get(0) == nil {
		return fn(ctx)
	}
	return args.Error(0)
}

func newAllowAllAuthz(t interface {
	mock.TestingT
	Cleanup(func())
}) *sysauthzmock.SystemAuthorizationServiceInterfaceMock {
	authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
	authzMock.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
		Return(true, nil).Maybe()
	authzMock.On("GetAccessibleResources", mock.Anything, mock.Anything, mock.Anything).
		Return(&sysauthz.AccessibleResources{AllAllowed: true}, nil).Maybe()
	return authzMock
}

func (suite *OrganizationUnitServiceTestSuite) assertOUListResponse(
	resp *OrganizationUnitListResponse,
	expected *ouListExpectations,
) {
	suite.Require().NotNil(resp)
	suite.Require().Equal(expected.totalResults, resp.TotalResults)
	suite.Require().Equal(expected.count, resp.Count)
	suite.Require().Equal(expected.startIndex, resp.StartIndex)
	suite.Require().Len(resp.OrganizationUnits, len(expected.handles))
	for idx, handle := range expected.handles {
		suite.Require().Equal(handle, resp.OrganizationUnits[idx].Handle)
	}
	suite.Require().Len(resp.Links, len(expected.linkRels))
	for idx := range expected.linkRels {
		suite.Require().Equal(expected.linkRels[idx], resp.Links[idx].Rel)
		suite.Require().Equal(expected.linkHrefs[idx], resp.Links[idx].Href)
	}
}

func (suite *OrganizationUnitServiceTestSuite) TestOUService_GetOrganizationUnitList() {
	testCases := []struct {
		name       string
		limit      int
		offset     int
		filterExpr *filter.FilterGroup
		setup      func(*organizationUnitStoreInterfaceMock)
		wantErr    *serviceerror.ServiceError
		wantResult *ouListExpectations
	}{
		{
			name:   "success",
			limit:  2,
			offset: 1,
			setup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("GetOrganizationUnitListCount", mock.Anything, mock.Anything).
					Return(3, nil).
					Once()
				store.On("GetOrganizationUnitList", mock.Anything, 2, 1, mock.Anything).
					Return([]OrganizationUnitBasic{
						{ID: "ou-1", Handle: "root", Name: "Root"},
						{ID: "ou-2", Handle: "child", Name: "Child"},
					}, nil).
					Once()
			},
			wantResult: &ouListExpectations{
				totalResults: 3,
				count:        2,
				startIndex:   2,
				handles:      []string{"root", "child"},
				linkRels:     []string{"first", "prev", "last"},
				linkHrefs: []string{
					"/organization-units?offset=0&limit=2",
					"/organization-units?offset=0&limit=2",
					"/organization-units?offset=2&limit=2",
				},
			},
		},
		{
			name:    "invalid pagination",
			limit:   0,
			offset:  0,
			wantErr: &ErrorInvalidLimit,
		},
		{
			name:   "invalid filter attribute",
			limit:  5,
			offset: 0,
			filterExpr: &filter.FilterGroup{Clauses: []filter.FilterClause{
				{Expr: filter.FilterExpression{Attribute: "id", Operator: filter.OperatorEq, Value: "ou-1"}},
			}},
			wantErr: &ErrorInvalidFilter,
		},
		{
			name:   "count failure",
			limit:  5,
			offset: 0,
			setup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("GetOrganizationUnitListCount", mock.Anything, mock.Anything).
					Return(0, errors.New("count failed")).
					Once()
			},
			wantErr: &serviceerror.InternalServerError,
		},
		{
			name:   "list failure",
			limit:  5,
			offset: 0,
			setup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("GetOrganizationUnitListCount", mock.Anything, mock.Anything).
					Return(10, nil).
					Once()
				store.On("GetOrganizationUnitList", mock.Anything, 5, 0, mock.Anything).
					Return(nil, errors.New("list failed")).
					Once()
			},
			wantErr: &serviceerror.InternalServerError,
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			store := newOrganizationUnitStoreInterfaceMock(suite.T())
			if tc.setup != nil {
				tc.setup(store)
			}

			service := suite.newService(store, newAllowAllAuthz(suite.T()))
			resp, err := service.GetOrganizationUnitList(context.Background(), tc.limit, tc.offset, tc.filterExpr)

			if tc.wantErr != nil {
				suite.Require().Nil(resp)
				suite.Require().NotNil(err)
				suite.Require().Equal(*tc.wantErr, *err)
			} else {
				suite.Require().Nil(err)
				suite.assertOUListResponse(resp, tc.wantResult)
			}

			if tc.wantErr == &ErrorInvalidLimit || tc.wantErr == &ErrorInvalidFilter {
				store.AssertNumberOfCalls(suite.T(), "GetOrganizationUnitListCount", 0)
			}
			store.AssertExpectations(suite.T())
		})
	}
}

func (suite *OrganizationUnitServiceTestSuite) TestOUService_SetResolvers() {
	service := &organizationUnitService{}

	userResolver := new(OUUserResolverMock)
	groupResolver := new(OUGroupResolverMock)

	service.SetOUUserResolver(userResolver)
	service.SetOUGroupResolver(groupResolver)

	suite.Require().Equal(userResolver, service.userResolver)
	suite.Require().Equal(groupResolver, service.groupResolver)
}

func (suite *OrganizationUnitServiceTestSuite) TestOUService_CreateOrganizationUnit() {
	parentID := testParentOUID
	validRequest := OrganizationUnitRequestWithID{
		Handle:      "finance",
		Name:        "Finance",
		Description: "desc",
	}

	testCases := []struct {
		name    string
		request OrganizationUnitRequestWithID
		setup   func(*organizationUnitStoreInterfaceMock)
		wantErr *serviceerror.ServiceError
	}{
		{
			name:    "invalid name",
			request: OrganizationUnitRequestWithID{Handle: "handle", Name: "  "},
			wantErr: &ErrorInvalidRequestFormat,
		},
		{
			name:    "invalid handle",
			request: OrganizationUnitRequestWithID{Handle: " ", Name: "Finance"},
			wantErr: &ErrorInvalidRequestFormat,
		},
		{
			name: "parent existence check error",
			request: OrganizationUnitRequestWithID{
				Handle: "finance",
				Name:   "Finance",
				Parent: &parentID,
			},
			setup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("IsOrganizationUnitExists", mock.Anything, parentID).
					Return(false, errors.New("boom")).
					Once()
			},
			wantErr: &serviceerror.InternalServerError,
		},
		{
			name: "parent not found",
			request: OrganizationUnitRequestWithID{
				Handle: "finance",
				Name:   "Finance",
				Parent: &parentID,
			},
			setup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("IsOrganizationUnitExists", mock.Anything, parentID).
					Return(false, nil).
					Once()
			},
			wantErr: &ErrorParentOrganizationUnitNotFound,
		},
		{
			name:    "name conflict error",
			request: validRequest,
			setup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("CheckOrganizationUnitNameConflict", mock.Anything, "Finance", (*string)(nil)).
					Return(true, nil).
					Once()
			},
			wantErr: &ErrorOrganizationUnitNameConflict,
		},
		{
			name:    "name conflict check failure",
			request: validRequest,
			setup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("CheckOrganizationUnitNameConflict", mock.Anything, "Finance", (*string)(nil)).
					Return(false, errors.New("name check failed")).
					Once()
			},
			wantErr: &serviceerror.InternalServerError,
		},
		{
			name:    "handle conflict",
			request: validRequest,
			setup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("CheckOrganizationUnitNameConflict", mock.Anything, "Finance", (*string)(nil)).
					Return(false, nil).
					Once()
				store.On("CheckOrganizationUnitHandleConflict", mock.Anything, "finance", (*string)(nil)).
					Return(true, nil).
					Once()
			},
			wantErr: &ErrorOrganizationUnitHandleConflict,
		},
		{
			name:    "handle conflict check failure",
			request: validRequest,
			setup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("CheckOrganizationUnitNameConflict", mock.Anything, "Finance", (*string)(nil)).
					Return(false, nil).
					Once()
				store.On("CheckOrganizationUnitHandleConflict", mock.Anything, "finance", (*string)(nil)).
					Return(false, errors.New("handle check failed")).
					Once()
			},
			wantErr: &serviceerror.InternalServerError,
		},
		{
			name:    "create failure",
			request: validRequest,
			setup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("CheckOrganizationUnitNameConflict", mock.Anything, "Finance", (*string)(nil)).
					Return(false, nil).
					Once()
				store.On("CheckOrganizationUnitHandleConflict", mock.Anything, "finance", (*string)(nil)).
					Return(false, nil).
					Once()
				store.On("CreateOrganizationUnit", mock.Anything, mock.AnythingOfType("ou.OrganizationUnit")).
					Return(errors.New("insert failed")).
					Once()
			},
			wantErr: &serviceerror.InternalServerError,
		},
		{
			name:    "success",
			request: validRequest,
			setup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("CheckOrganizationUnitNameConflict", mock.Anything, "Finance", (*string)(nil)).
					Return(false, nil).
					Once()
				store.On("CheckOrganizationUnitHandleConflict", mock.Anything, "finance", (*string)(nil)).
					Return(false, nil).
					Once()
				store.On("CreateOrganizationUnit", mock.Anything, mock.MatchedBy(func(ou OrganizationUnit) bool {
					return ou.Name == "Finance" && ou.Handle == "finance"
				})).
					Return(nil).
					Once()
			},
		},
		{
			name: "success with design fields",
			request: OrganizationUnitRequestWithID{
				Handle:   "finance",
				Name:     "Finance",
				ThemeID:  "theme-123",
				LayoutID: "layout-456",
				LogoURL:  "https://example.com/logo.png",
			},
			setup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("CheckOrganizationUnitNameConflict", mock.Anything, "Finance", (*string)(nil)).
					Return(false, nil).
					Once()
				store.On("CheckOrganizationUnitHandleConflict", mock.Anything, "finance", (*string)(nil)).
					Return(false, nil).
					Once()
				store.On("CreateOrganizationUnit", mock.Anything, mock.MatchedBy(func(ou OrganizationUnit) bool {
					return ou.Name == "Finance" && ou.Handle == "finance" &&
						ou.ThemeID == "theme-123" && ou.LayoutID == "layout-456" &&
						ou.LogoURL == "https://example.com/logo.png"
				})).
					Return(nil).
					Once()
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			store := newOrganizationUnitStoreInterfaceMock(suite.T())
			if tc.setup != nil {
				tc.setup(store)
			}

			service := suite.newService(store, newAllowAllAuthz(suite.T()))
			result, err := service.CreateOrganizationUnit(context.Background(), tc.request)

			if tc.wantErr != nil {
				suite.Require().NotNil(err)
				suite.Require().Equal(*tc.wantErr, *err)
			} else {
				suite.Require().Nil(err)
				suite.Require().Equal(tc.request.Name, result.Name)
				suite.Require().Equal(tc.request.Handle, result.Handle)
				suite.Require().NotEmpty(result.ID)
			}

			if tc.wantErr == &ErrorInvalidRequestFormat {
				store.AssertNumberOfCalls(suite.T(), "CheckOrganizationUnitNameConflict", 0)
			}
			store.AssertExpectations(suite.T())
		})
	}
}

func (suite *OrganizationUnitServiceTestSuite) TestOUService_GetOrganizationUnit() {
	testCases := []struct {
		name    string
		setup   func(*organizationUnitStoreInterfaceMock)
		wantErr *serviceerror.ServiceError
	}{
		{
			name: "success",
			setup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("GetOrganizationUnit", mock.Anything, "ou-1").
					Return(OrganizationUnit{ID: "ou-1", Name: "Root"}, nil).
					Once()
			},
		},
		{
			name: "not found",
			setup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("GetOrganizationUnit", mock.Anything, "ou-1").
					Return(OrganizationUnit{}, ErrOrganizationUnitNotFound).
					Once()
			},
			wantErr: &ErrorOrganizationUnitNotFound,
		},
		{
			name: "store error",
			setup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("GetOrganizationUnit", mock.Anything, "ou-1").
					Return(OrganizationUnit{}, errors.New("boom")).
					Once()
			},
			wantErr: &serviceerror.InternalServerError,
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			store := newOrganizationUnitStoreInterfaceMock(suite.T())
			tc.setup(store)

			service := suite.newService(store, newAllowAllAuthz(suite.T()))
			result, err := service.GetOrganizationUnit(context.Background(), "ou-1")

			if tc.wantErr != nil {
				suite.Require().NotNil(err)
				suite.Require().Equal(*tc.wantErr, *err)
			} else {
				suite.Require().Nil(err)
				suite.Require().Equal("ou-1", result.ID)
			}
		})
	}
}

func (suite *OrganizationUnitServiceTestSuite) TestOUService_GetOrganizationUnitByPath() {
	testCases := []struct {
		name    string
		path    string
		setup   func(*organizationUnitStoreInterfaceMock)
		wantErr *serviceerror.ServiceError
	}{
		{
			name:    "invalid path",
			path:    "   ",
			wantErr: &ErrorInvalidHandlePath,
		},
		{
			name: "not found",
			path: "/root/child/",
			setup: func(store *organizationUnitStoreInterfaceMock) {
				store.
					On("GetOrganizationUnitByPath", mock.Anything, []string{"root", "child"}).
					Return(OrganizationUnit{}, ErrOrganizationUnitNotFound).
					Once()
			},
			wantErr: &ErrorOrganizationUnitNotFound,
		},
		{
			name: "store error",
			path: "root",
			setup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("GetOrganizationUnitByPath", mock.Anything, []string{"root"}).
					Return(OrganizationUnit{}, errors.New("boom")).
					Once()
			},
			wantErr: &serviceerror.InternalServerError,
		},
		{
			name: "success",
			path: "root",
			setup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("GetOrganizationUnitByPath", mock.Anything, []string{"root"}).
					Return(OrganizationUnit{ID: "ou-1", Handle: "root"}, nil).
					Once()
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			store := newOrganizationUnitStoreInterfaceMock(suite.T())
			if tc.setup != nil {
				tc.setup(store)
			}

			service := suite.newService(store, newAllowAllAuthz(suite.T()))
			result, err := service.GetOrganizationUnitByPath(context.Background(), tc.path)

			if tc.wantErr != nil {
				suite.Require().NotNil(err)
				suite.Require().Equal(*tc.wantErr, *err)
			} else {
				suite.Require().Nil(err)
				suite.Require().Equal("ou-1", result.ID)
			}

			if tc.wantErr == &ErrorInvalidHandlePath {
				store.AssertNumberOfCalls(suite.T(), "GetOrganizationUnitByPath", 0)
			}
		})
	}
}

func (suite *OrganizationUnitServiceTestSuite) TestOUService_IsOrganizationUnitExists() {
	testCases := []struct {
		name    string
		setup   func(*organizationUnitStoreInterfaceMock)
		wantErr *serviceerror.ServiceError
		want    bool
	}{
		{
			name: "success",
			setup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("IsOrganizationUnitExists", mock.Anything, "ou-1").
					Return(true, nil).
					Once()
			},
			want: true,
		},
		{
			name: "store error",
			setup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("IsOrganizationUnitExists", mock.Anything, "ou-1").
					Return(false, errors.New("boom")).
					Once()
			},
			wantErr: &serviceerror.InternalServerError,
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			store := newOrganizationUnitStoreInterfaceMock(suite.T())
			tc.setup(store)

			service := suite.newService(store, newAllowAllAuthz(suite.T()))
			result, err := service.IsOrganizationUnitExists(context.Background(), "ou-1")

			if tc.wantErr != nil {
				suite.Require().NotNil(err)
				suite.Require().Equal(*tc.wantErr, *err)
			} else {
				suite.Require().Nil(err)
				suite.Require().Equal(tc.want, result)
			}
		})
	}
}

func (suite *OrganizationUnitServiceTestSuite) TestOUService_IsParent() {
	parentID := "parent-1"
	childID := "child-1"

	suite.Run("returns true when IDs are equal", func() {
		service := suite.newService(newOrganizationUnitStoreInterfaceMock(suite.T()), newAllowAllAuthz(suite.T()))

		result, err := service.IsParent(context.Background(), parentID, parentID)

		suite.Require().True(result)
		suite.Require().Nil(err)
	})

	suite.Run("returns true for direct parent", func() {
		store := newOrganizationUnitStoreInterfaceMock(suite.T())
		store.On("GetOrganizationUnit", mock.Anything, childID).
			Return(OrganizationUnit{ID: childID, Parent: &parentID}, nil).
			Once()

		service := suite.newService(store, newAllowAllAuthz(suite.T()))

		result, err := service.IsParent(context.Background(), parentID, childID)

		suite.Require().True(result)
		suite.Require().Nil(err)
	})

	suite.Run("returns true for ancestor", func() {
		store := newOrganizationUnitStoreInterfaceMock(suite.T())
		store.On("GetOrganizationUnit", mock.Anything, childID).
			Return(OrganizationUnit{ID: childID, Parent: &testMidID}, nil).
			Once()
		store.On("GetOrganizationUnit", mock.Anything, "mid-1").
			Return(OrganizationUnit{ID: "mid-1", Parent: &parentID}, nil).
			Once()

		service := suite.newService(store, newAllowAllAuthz(suite.T()))

		result, err := service.IsParent(context.Background(), parentID, childID)

		suite.Require().True(result)
		suite.Require().Nil(err)
	})

	suite.Run("returns false when parent not in hierarchy", func() {
		store := newOrganizationUnitStoreInterfaceMock(suite.T())
		store.On("GetOrganizationUnit", mock.Anything, childID).
			Return(OrganizationUnit{ID: childID, Parent: &testMidID}, nil).
			Once()
		store.On("GetOrganizationUnit", mock.Anything, "mid-1").
			Return(OrganizationUnit{ID: "mid-1"}, nil).
			Once()

		service := suite.newService(store, newAllowAllAuthz(suite.T()))

		result, err := service.IsParent(context.Background(), parentID, childID)

		suite.Require().False(result)
		suite.Require().Nil(err)
	})

	suite.Run("returns error when child not found", func() {
		store := newOrganizationUnitStoreInterfaceMock(suite.T())
		store.On("GetOrganizationUnit", mock.Anything, childID).
			Return(OrganizationUnit{}, ErrOrganizationUnitNotFound).
			Once()

		service := suite.newService(store, newAllowAllAuthz(suite.T()))

		result, err := service.IsParent(context.Background(), parentID, childID)

		suite.Require().False(result)
		suite.Require().Equal(ErrorOrganizationUnitNotFound, *err)
	})

	suite.Run("returns error on store failure", func() {
		store := newOrganizationUnitStoreInterfaceMock(suite.T())
		store.On("GetOrganizationUnit", mock.Anything, childID).
			Return(OrganizationUnit{}, errors.New("boom")).
			Once()

		service := suite.newService(store, newAllowAllAuthz(suite.T()))

		result, err := service.IsParent(context.Background(), parentID, childID)

		suite.Require().False(result)
		suite.Require().Equal(serviceerror.InternalServerError, *err)
	})
}

func (suite *OrganizationUnitServiceTestSuite) TestOUService_UpdateOrganizationUnit() {
	parentID := testParentID
	tests := []struct {
		name    string
		id      string
		request OrganizationUnitRequestWithID
		setup   func(*organizationUnitStoreInterfaceMock)
		wantErr *serviceerror.ServiceError
		assert  func(OrganizationUnit)
	}{
		{
			name: "success",
			id:   "ou-1",
			request: OrganizationUnitRequestWithID{
				Handle:      "root",
				Name:        "Root",
				Description: "updated",
			},
			setup: func(store *organizationUnitStoreInterfaceMock) {
				existing := OrganizationUnit{
					ID:          "ou-1",
					Handle:      "root",
					Name:        "Root",
					Description: "old",
				}
				store.On("GetOrganizationUnit", mock.Anything, "ou-1").
					Return(existing, nil).
					Once()
				store.On("IsOrganizationUnitDeclarative", mock.Anything, "ou-1").
					Return(false).
					Once()
				store.On("UpdateOrganizationUnit", mock.Anything, mock.Anything).
					Return(nil).
					Once()
			},
			assert: func(ou OrganizationUnit) {
				suite.Equal("updated", ou.Description)
			},
		},
		{
			name: "success with design fields",
			id:   "ou-1",
			request: OrganizationUnitRequestWithID{
				Handle:   "root",
				Name:     "Root",
				ThemeID:  "theme-new",
				LayoutID: "layout-new",
				LogoURL:  "https://example.com/new-logo.png",
			},
			setup: func(store *organizationUnitStoreInterfaceMock) {
				existing := OrganizationUnit{
					ID:       "ou-1",
					Handle:   "root",
					Name:     "Root",
					ThemeID:  "theme-old",
					LayoutID: "layout-old",
					LogoURL:  "https://example.com/old-logo.png",
				}
				store.On("GetOrganizationUnit", mock.Anything, "ou-1").
					Return(existing, nil).
					Once()
				store.On("IsOrganizationUnitDeclarative", mock.Anything, "ou-1").
					Return(false).
					Once()
				store.On("UpdateOrganizationUnit", mock.Anything, mock.Anything).
					Return(nil).
					Once()
			},
			assert: func(ou OrganizationUnit) {
				suite.Equal("theme-new", ou.ThemeID)
				suite.Equal("layout-new", ou.LayoutID)
				suite.Equal("https://example.com/new-logo.png", ou.LogoURL)
			},
		},
		{
			name: "not found on fetch",
			id:   "missing",
			request: OrganizationUnitRequestWithID{
				Handle: "root",
				Name:   "Root",
			},
			setup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("GetOrganizationUnit", mock.Anything, "missing").
					Return(OrganizationUnit{}, ErrOrganizationUnitNotFound).
					Once()
			},
			wantErr: &ErrorOrganizationUnitNotFound,
		},
		{
			name: "fetch failure",
			id:   "ou-1",
			request: OrganizationUnitRequestWithID{
				Handle: "root",
				Name:   "Root",
			},
			setup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("GetOrganizationUnit", mock.Anything, "ou-1").
					Return(OrganizationUnit{}, errors.New("boom")).
					Once()
			},
			wantErr: &serviceerror.InternalServerError,
		},
		{
			name: "invalid handle",
			id:   "ou-1",
			request: OrganizationUnitRequestWithID{
				Handle: " ",
				Name:   "Root",
			},
			setup: func(store *organizationUnitStoreInterfaceMock) {
				existing := OrganizationUnit{ID: "ou-1", Handle: "root", Name: "Root"}
				store.On("GetOrganizationUnit", mock.Anything, "ou-1").
					Return(existing, nil).
					Once()
				store.On("IsOrganizationUnitDeclarative", mock.Anything, "ou-1").
					Return(false).
					Once()
			},
			wantErr: &ErrorInvalidRequestFormat,
		},
		{
			name: "parent existence check failure",
			id:   "ou-1",
			request: OrganizationUnitRequestWithID{
				Handle: "root",
				Name:   "Root",
				Parent: &parentID,
			},
			setup: func(store *organizationUnitStoreInterfaceMock) {
				existing := OrganizationUnit{ID: "ou-1", Handle: "root", Name: "Root"}
				store.On("GetOrganizationUnit", mock.Anything, "ou-1").
					Return(existing, nil).
					Once()
				store.On("IsOrganizationUnitDeclarative", mock.Anything, "ou-1").
					Return(false).
					Once()
				store.On("IsOrganizationUnitExists", mock.Anything, parentID).
					Return(false, errors.New("boom")).
					Once()
			},
			wantErr: &serviceerror.InternalServerError,
		},
		{
			name: "parent not found",
			id:   "ou-1",
			request: OrganizationUnitRequestWithID{
				Handle: "root",
				Name:   "Root",
				Parent: &parentID,
			},
			setup: func(store *organizationUnitStoreInterfaceMock) {
				existing := OrganizationUnit{ID: "ou-1", Handle: "root", Name: "Root"}
				store.On("GetOrganizationUnit", mock.Anything, "ou-1").
					Return(existing, nil).
					Once()
				store.On("IsOrganizationUnitDeclarative", mock.Anything, "ou-1").
					Return(false).
					Once()
				store.On("IsOrganizationUnitExists", mock.Anything, parentID).
					Return(false, nil).
					Once()
			},
			wantErr: &ErrorParentOrganizationUnitNotFound,
		},
		{
			name: "circular dependency",
			id:   "ou-1",
			request: OrganizationUnitRequestWithID{
				Handle: "root",
				Name:   "Root",
				Parent: &testOUID,
			},
			setup: func(store *organizationUnitStoreInterfaceMock) {
				existing := OrganizationUnit{ID: "ou-1", Handle: "root", Name: "Root"}
				store.On("GetOrganizationUnit", mock.Anything, "ou-1").
					Return(existing, nil).
					Once()
				store.On("IsOrganizationUnitDeclarative", mock.Anything, "ou-1").
					Return(false).
					Once()
				store.On("IsOrganizationUnitExists", mock.Anything, "ou-1").
					Return(true, nil).
					Once()
			},
			wantErr: &ErrorCircularDependency,
		},
		{
			name: "name conflict",
			id:   "ou-1",
			request: OrganizationUnitRequestWithID{
				Handle: "root",
				Name:   "Finance",
			},
			setup: func(store *organizationUnitStoreInterfaceMock) {
				existing := OrganizationUnit{ID: "ou-1", Handle: "root", Name: "Root"}
				store.On("GetOrganizationUnit", mock.Anything, "ou-1").
					Return(existing, nil).
					Once()
				store.On("IsOrganizationUnitDeclarative", mock.Anything, "ou-1").
					Return(false).
					Once()
				store.On("CheckOrganizationUnitNameConflict", mock.Anything, "Finance", (*string)(nil)).
					Return(true, nil).
					Once()
			},
			wantErr: &ErrorOrganizationUnitNameConflict,
		},
		{
			name: "name conflict check failure",
			id:   "ou-1",
			request: OrganizationUnitRequestWithID{
				Handle: "root",
				Name:   "Finance",
			},
			setup: func(store *organizationUnitStoreInterfaceMock) {
				existing := OrganizationUnit{ID: "ou-1", Handle: "root", Name: "Root"}
				store.On("GetOrganizationUnit", mock.Anything, "ou-1").
					Return(existing, nil).
					Once()
				store.On("IsOrganizationUnitDeclarative", mock.Anything, "ou-1").
					Return(false).
					Once()
				store.On("CheckOrganizationUnitNameConflict", mock.Anything, "Finance", (*string)(nil)).
					Return(false, errors.New("boom")).
					Once()
			},
			wantErr: &serviceerror.InternalServerError,
		},
		{
			name: "handle conflict",
			id:   "ou-1",
			request: OrganizationUnitRequestWithID{
				Handle: "finance",
				Name:   "Root",
			},
			setup: func(store *organizationUnitStoreInterfaceMock) {
				existing := OrganizationUnit{ID: "ou-1", Handle: "root", Name: "Root"}
				store.On("GetOrganizationUnit", mock.Anything, "ou-1").
					Return(existing, nil).
					Once()
				store.On("IsOrganizationUnitDeclarative", mock.Anything, "ou-1").
					Return(false).
					Once()
				store.On("CheckOrganizationUnitHandleConflict", mock.Anything, "finance", (*string)(nil)).
					Return(true, nil).
					Once()
			},
			wantErr: &ErrorOrganizationUnitHandleConflict,
		},
		{
			name: "handle conflict check failure",
			id:   "ou-1",
			request: OrganizationUnitRequestWithID{
				Handle: "finance",
				Name:   "Root",
			},
			setup: func(store *organizationUnitStoreInterfaceMock) {
				existing := OrganizationUnit{ID: "ou-1", Handle: "root", Name: "Root"}
				store.On("GetOrganizationUnit", mock.Anything, "ou-1").
					Return(existing, nil).
					Once()
				store.On("IsOrganizationUnitDeclarative", mock.Anything, "ou-1").
					Return(false).
					Once()
				store.On("CheckOrganizationUnitHandleConflict", mock.Anything, "finance", (*string)(nil)).
					Return(false, errors.New("boom")).
					Once()
			},
			wantErr: &serviceerror.InternalServerError,
		},
		{
			name: "update returns not found",
			id:   "ou-1",
			request: OrganizationUnitRequestWithID{
				Handle: "root",
				Name:   "Root",
			},
			setup: func(store *organizationUnitStoreInterfaceMock) {
				existing := OrganizationUnit{ID: "ou-1", Handle: "root", Name: "Root"}
				store.On("GetOrganizationUnit", mock.Anything, "ou-1").
					Return(existing, nil).
					Once()
				store.On("IsOrganizationUnitDeclarative", mock.Anything, "ou-1").
					Return(false).
					Once()
				store.On("UpdateOrganizationUnit", mock.Anything, mock.AnythingOfType("ou.OrganizationUnit")).
					Return(ErrOrganizationUnitNotFound).
					Once()
			},
			wantErr: &ErrorOrganizationUnitNotFound,
		},
		{
			name: "update failure",
			id:   "ou-1",
			request: OrganizationUnitRequestWithID{
				Handle: "root",
				Name:   "Root",
			},
			setup: func(store *organizationUnitStoreInterfaceMock) {
				existing := OrganizationUnit{ID: "ou-1", Handle: "root", Name: "Root"}
				store.On("GetOrganizationUnit", mock.Anything, "ou-1").
					Return(existing, nil).
					Once()
				store.On("IsOrganizationUnitDeclarative", mock.Anything, "ou-1").
					Return(false).
					Once()
				store.On("UpdateOrganizationUnit", mock.Anything, mock.AnythingOfType("ou.OrganizationUnit")).
					Return(errors.New("boom")).
					Once()
			},
			wantErr: &serviceerror.InternalServerError,
		},
	}

	for _, tc := range tests {
		tc := tc
		suite.Run(tc.name, func() {
			store := newOrganizationUnitStoreInterfaceMock(suite.T())
			if tc.setup != nil {
				tc.setup(store)
			}

			service := suite.newService(store, newAllowAllAuthz(suite.T()))
			result, err := service.UpdateOrganizationUnit(context.Background(), tc.id, tc.request)

			if tc.wantErr != nil {
				suite.Require().NotNil(err)
				suite.Require().Equal(*tc.wantErr, *err)
			} else {
				suite.Require().Nil(err)
				if tc.assert != nil {
					tc.assert(result)
				}
			}

			store.AssertExpectations(suite.T())
		})
	}
}

func (suite *OrganizationUnitServiceTestSuite) TestOUService_UpdateOrganizationUnitByPath() {
	request := OrganizationUnitRequestWithID{Handle: "root", Name: "Root"}

	suite.Run("invalid path", func() {
		store := newOrganizationUnitStoreInterfaceMock(suite.T())
		service := suite.newService(store, newAllowAllAuthz(suite.T()))

		_, err := service.UpdateOrganizationUnitByPath(context.Background(), "   ", request)

		suite.Require().Equal(ErrorInvalidHandlePath, *err)
		store.AssertNumberOfCalls(suite.T(), "GetOrganizationUnitByPath", 0)
	})

	suite.Run("not found", func() {
		store := newOrganizationUnitStoreInterfaceMock(suite.T())
		store.On("GetOrganizationUnitByPath", mock.Anything, []string{"root"}).
			Return(OrganizationUnit{}, ErrOrganizationUnitNotFound).
			Once()

		service := suite.newService(store, newAllowAllAuthz(suite.T()))
		_, err := service.UpdateOrganizationUnitByPath(context.Background(), "root", request)

		suite.Require().Equal(ErrorOrganizationUnitNotFound, *err)
	})

	suite.Run("get by path error", func() {
		store := newOrganizationUnitStoreInterfaceMock(suite.T())
		store.On("GetOrganizationUnitByPath", mock.Anything, []string{"root"}).
			Return(OrganizationUnit{}, errors.New("boom")).
			Once()

		service := suite.newService(store, newAllowAllAuthz(suite.T()))
		_, err := service.UpdateOrganizationUnitByPath(context.Background(), "root", request)

		suite.Require().Equal(serviceerror.InternalServerError, *err)
	})

	suite.Run("success", func() {
		store := newOrganizationUnitStoreInterfaceMock(suite.T())
		existing := OrganizationUnit{ID: "ou-1", Handle: "root", Name: "Root"}
		store.On("GetOrganizationUnitByPath", mock.Anything, []string{"root"}).
			Return(existing, nil).
			Once()
		store.On("IsOrganizationUnitDeclarative", mock.Anything, "ou-1").
			Return(false).
			Twice() // Called in UpdateOrganizationUnitByPath and updateOUInternal
		store.On("UpdateOrganizationUnit", mock.Anything, mock.AnythingOfType("ou.OrganizationUnit")).
			Return(nil).
			Once()

		service := suite.newService(store, newAllowAllAuthz(suite.T()))
		result, err := service.UpdateOrganizationUnitByPath(context.Background(), "root", request)

		suite.Require().Nil(err)
		suite.Require().Equal("ou-1", result.ID)
	})

	suite.Run("declarative resource cannot be updated", func() {
		store := newOrganizationUnitStoreInterfaceMock(suite.T())
		existing := OrganizationUnit{ID: "ou-1", Handle: "root", Name: "Root"}
		store.On("GetOrganizationUnitByPath", mock.Anything, []string{"root"}).
			Return(existing, nil).
			Once()
		store.On("IsOrganizationUnitDeclarative", mock.Anything, "ou-1").
			Return(true).
			Once()

		service := suite.newService(store, newAllowAllAuthz(suite.T()))
		_, err := service.UpdateOrganizationUnitByPath(context.Background(), "root", request)

		suite.Require().Equal(ErrorCannotModifyDeclarativeResource, *err)
		store.AssertNumberOfCalls(suite.T(), "UpdateOrganizationUnit", 0)
	})
}

func (suite *OrganizationUnitServiceTestSuite) TestOUService_DeleteOrganizationUnit() {
	type resolverSetup struct {
		userResolver  *OUUserResolverMock
		groupResolver *OUGroupResolverMock
	}

	testCases := []struct {
		name          string
		setup         func(*organizationUnitStoreInterfaceMock)
		resolverSetup func(*resolverSetup)
		wantErr       *serviceerror.ServiceError
	}{
		{
			name: "existence check error",
			setup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("IsOrganizationUnitExists", mock.Anything, "ou-1").
					Return(false, errors.New("boom")).
					Once()
			},
			wantErr: &serviceerror.InternalServerError,
		},
		{
			name: "not found",
			setup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("IsOrganizationUnitExists", mock.Anything, "ou-1").
					Return(false, nil).
					Once()
			},
			wantErr: &ErrorOrganizationUnitNotFound,
		},
		{
			name: "has child OUs",
			setup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("IsOrganizationUnitExists", mock.Anything, "ou-1").
					Return(true, nil).Once()
				store.On("IsOrganizationUnitDeclarative", mock.Anything, "ou-1").
					Return(false).Once()
				store.On("GetOrganizationUnitChildrenCount", mock.Anything, "ou-1", mock.Anything).
					Return(1, nil).Once()
			},
			wantErr: &ErrorCannotDeleteOrganizationUnit,
		},
		{
			name: "child OU check failure",
			setup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("IsOrganizationUnitExists", mock.Anything, "ou-1").
					Return(true, nil).Once()
				store.On("IsOrganizationUnitDeclarative", mock.Anything, "ou-1").
					Return(false).Once()
				store.On("GetOrganizationUnitChildrenCount", mock.Anything, "ou-1", mock.Anything).
					Return(0, errors.New("boom")).Once()
			},
			wantErr: &serviceerror.InternalServerError,
		},
		{
			name: "has users",
			setup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("IsOrganizationUnitExists", mock.Anything, "ou-1").
					Return(true, nil).Once()
				store.On("IsOrganizationUnitDeclarative", mock.Anything, "ou-1").
					Return(false).Once()
				store.On("GetOrganizationUnitChildrenCount", mock.Anything, "ou-1", mock.Anything).
					Return(0, nil).Once()
			},
			resolverSetup: func(rs *resolverSetup) {
				rs.userResolver.On("GetUserCountByOUID", mock.Anything, "ou-1").
					Return(3, nil).Once()
			},
			wantErr: &ErrorCannotDeleteOrganizationUnit,
		},
		{
			name: "has groups",
			setup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("IsOrganizationUnitExists", mock.Anything, "ou-1").
					Return(true, nil).Once()
				store.On("IsOrganizationUnitDeclarative", mock.Anything, "ou-1").
					Return(false).Once()
				store.On("GetOrganizationUnitChildrenCount", mock.Anything, "ou-1", mock.Anything).
					Return(0, nil).Once()
			},
			resolverSetup: func(rs *resolverSetup) {
				rs.userResolver.On("GetUserCountByOUID", mock.Anything, "ou-1").
					Return(0, nil).Once()
				rs.groupResolver.On("GetGroupCountByOUID", mock.Anything, "ou-1").
					Return(2, nil).Once()
			},
			wantErr: &ErrorCannotDeleteOrganizationUnit,
		},
		{
			name: "delete failure",
			setup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("IsOrganizationUnitExists", mock.Anything, "ou-1").
					Return(true, nil).Once()
				store.On("IsOrganizationUnitDeclarative", mock.Anything, "ou-1").
					Return(false).Once()
				store.On("GetOrganizationUnitChildrenCount", mock.Anything, "ou-1", mock.Anything).
					Return(0, nil).Once()
				store.On("DeleteOrganizationUnit", mock.Anything, "ou-1").
					Return(errors.New("boom")).Once()
			},
			resolverSetup: func(rs *resolverSetup) {
				rs.userResolver.On("GetUserCountByOUID", mock.Anything, "ou-1").
					Return(0, nil).Once()
				rs.groupResolver.On("GetGroupCountByOUID", mock.Anything, "ou-1").
					Return(0, nil).Once()
			},
			wantErr: &serviceerror.InternalServerError,
		},
		{
			name: "delete not found",
			setup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("IsOrganizationUnitExists", mock.Anything, "ou-1").
					Return(true, nil).Once()
				store.On("IsOrganizationUnitDeclarative", mock.Anything, "ou-1").
					Return(false).Once()
				store.On("GetOrganizationUnitChildrenCount", mock.Anything, "ou-1", mock.Anything).
					Return(0, nil).Once()
				store.On("DeleteOrganizationUnit", mock.Anything, "ou-1").
					Return(ErrOrganizationUnitNotFound).Once()
			},
			resolverSetup: func(rs *resolverSetup) {
				rs.userResolver.On("GetUserCountByOUID", mock.Anything, "ou-1").
					Return(0, nil).Once()
				rs.groupResolver.On("GetGroupCountByOUID", mock.Anything, "ou-1").
					Return(0, nil).Once()
			},
			wantErr: &ErrorOrganizationUnitNotFound,
		},
		{
			name: "success",
			setup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("IsOrganizationUnitExists", mock.Anything, "ou-1").
					Return(true, nil).Once()
				store.On("IsOrganizationUnitDeclarative", mock.Anything, "ou-1").
					Return(false).Once()
				store.On("GetOrganizationUnitChildrenCount", mock.Anything, "ou-1", mock.Anything).
					Return(0, nil).Once()
				store.On("DeleteOrganizationUnit", mock.Anything, "ou-1").
					Return(nil).Once()
			},
			resolverSetup: func(rs *resolverSetup) {
				rs.userResolver.On("GetUserCountByOUID", mock.Anything, "ou-1").
					Return(0, nil).Once()
				rs.groupResolver.On("GetGroupCountByOUID", mock.Anything, "ou-1").
					Return(0, nil).Once()
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			store := newOrganizationUnitStoreInterfaceMock(suite.T())
			tc.setup(store)

			rs := &resolverSetup{
				userResolver:  new(OUUserResolverMock),
				groupResolver: new(OUGroupResolverMock),
			}
			if tc.resolverSetup != nil {
				tc.resolverSetup(rs)
			}

			service := suite.newServiceWithResolvers(
				store, newAllowAllAuthz(suite.T()), rs.userResolver, rs.groupResolver,
			)
			err := service.DeleteOrganizationUnit(context.Background(), "ou-1")

			if tc.wantErr != nil {
				suite.Require().Equal(*tc.wantErr, *err)
			} else {
				suite.Require().Nil(err)
			}
		})
	}
}

func (suite *OrganizationUnitServiceTestSuite) TestOUService_DeleteOrganizationUnitByPath() {
	suite.Run("invalid path", func() {
		store := newOrganizationUnitStoreInterfaceMock(suite.T())
		service := suite.newService(store, newAllowAllAuthz(suite.T()))

		err := service.DeleteOrganizationUnitByPath(context.Background(), "  ")

		suite.Require().Equal(ErrorInvalidHandlePath, *err)
		store.AssertNumberOfCalls(suite.T(), "GetOrganizationUnitByPath", 0)
	})

	suite.Run("not found", func() {
		store := newOrganizationUnitStoreInterfaceMock(suite.T())
		store.On("GetOrganizationUnitByPath", mock.Anything, []string{"root"}).
			Return(OrganizationUnit{}, ErrOrganizationUnitNotFound).
			Once()

		service := suite.newService(store, newAllowAllAuthz(suite.T()))
		err := service.DeleteOrganizationUnitByPath(context.Background(), "root")

		suite.Require().Equal(ErrorOrganizationUnitNotFound, *err)
	})

	suite.Run("get by path error", func() {
		store := newOrganizationUnitStoreInterfaceMock(suite.T())
		store.On("GetOrganizationUnitByPath", mock.Anything, []string{"root"}).
			Return(OrganizationUnit{}, errors.New("boom")).
			Once()

		service := suite.newService(store, newAllowAllAuthz(suite.T()))
		err := service.DeleteOrganizationUnitByPath(context.Background(), "root")

		suite.Require().Equal(serviceerror.InternalServerError, *err)
	})

	suite.Run("cannot delete - has child OUs", func() {
		store := newOrganizationUnitStoreInterfaceMock(suite.T())
		store.On("GetOrganizationUnitByPath", mock.Anything, []string{"root"}).
			Return(OrganizationUnit{ID: "ou-1"}, nil).Once()
		store.On("IsOrganizationUnitDeclarative", mock.Anything, "ou-1").
			Return(false).Twice()
		store.On("GetOrganizationUnitChildrenCount", mock.Anything, "ou-1", mock.Anything).
			Return(1, nil).Once()

		userRes := new(OUUserResolverMock)
		groupRes := new(OUGroupResolverMock)
		service := suite.newServiceWithResolvers(
			store, newAllowAllAuthz(suite.T()), userRes, groupRes,
		)
		err := service.DeleteOrganizationUnitByPath(context.Background(), "root")

		suite.Require().Equal(ErrorCannotDeleteOrganizationUnit, *err)
	})

	suite.Run("success", func() {
		store := newOrganizationUnitStoreInterfaceMock(suite.T())
		store.On("GetOrganizationUnitByPath", mock.Anything, []string{"root"}).
			Return(OrganizationUnit{ID: "ou-1"}, nil).Once()
		store.On("IsOrganizationUnitDeclarative", mock.Anything, "ou-1").
			Return(false).Twice()
		store.On("GetOrganizationUnitChildrenCount", mock.Anything, "ou-1", mock.Anything).
			Return(0, nil).Once()
		store.On("DeleteOrganizationUnit", mock.Anything, "ou-1").
			Return(nil).Once()

		userRes := new(OUUserResolverMock)
		userRes.On("GetUserCountByOUID", mock.Anything, "ou-1").Return(0, nil).Once()
		groupRes := new(OUGroupResolverMock)
		groupRes.On("GetGroupCountByOUID", mock.Anything, "ou-1").Return(0, nil).Once()

		service := suite.newServiceWithResolvers(
			store, newAllowAllAuthz(suite.T()), userRes, groupRes,
		)
		err := service.DeleteOrganizationUnitByPath(context.Background(), "root")

		suite.Require().Nil(err)
	})

	suite.Run("declarative resource cannot be deleted", func() {
		store := newOrganizationUnitStoreInterfaceMock(suite.T())
		store.On("GetOrganizationUnitByPath", mock.Anything, []string{"root"}).
			Return(OrganizationUnit{ID: "ou-1"}, nil).Once()
		store.On("IsOrganizationUnitDeclarative", mock.Anything, "ou-1").
			Return(true).Once()

		service := suite.newService(store, newAllowAllAuthz(suite.T()))
		err := service.DeleteOrganizationUnitByPath(context.Background(), "root")

		suite.Require().Equal(ErrorCannotModifyDeclarativeResource, *err)
		store.AssertNumberOfCalls(suite.T(), "GetOrganizationUnitChildrenCount", 0)
		store.AssertNumberOfCalls(suite.T(), "DeleteOrganizationUnit", 0)
	})
}

func (suite *OrganizationUnitServiceTestSuite) TestOUService_GetOrganizationUnitChildren() {
	testCases := []struct {
		name       string
		limit      int
		offset     int
		filterExpr *filter.FilterGroup
		setup      func(*organizationUnitStoreInterfaceMock)
		wantErr    *serviceerror.ServiceError
		wantResult *ouListExpectations
	}{
		{
			name:    "invalid pagination",
			limit:   0,
			offset:  0,
			wantErr: &ErrorInvalidLimit,
		},
		{
			name:  "invalid filter attribute",
			limit: 5,
			filterExpr: &filter.FilterGroup{Clauses: []filter.FilterClause{
				{Expr: filter.FilterExpression{Attribute: "id", Operator: filter.OperatorEq, Value: "ou-1"}},
			}},
			wantErr: &ErrorInvalidFilter,
		},
		{
			name:  "ou not found",
			limit: 5,
			setup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("IsOrganizationUnitExists", mock.Anything, "ou-1").
					Return(false, nil).
					Once()
			},
			wantErr: &ErrorOrganizationUnitNotFound,
		},
		{
			name:  "existence check error",
			limit: 5,
			setup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("IsOrganizationUnitExists", mock.Anything, "ou-1").
					Return(false, errors.New("boom")).
					Once()
			},
			wantErr: &serviceerror.InternalServerError,
		},
		{
			name:  "list failure",
			limit: 5,
			setup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("IsOrganizationUnitExists", mock.Anything, "ou-1").
					Return(true, nil).
					Once()
				store.On("GetOrganizationUnitChildrenList", mock.Anything, "ou-1", 5, 0, mock.Anything).
					Return(nil, errors.New("list fail")).
					Once()
			},
			wantErr: &serviceerror.InternalServerError,
		},
		{
			name:  "composite limit exceeded",
			limit: 5,
			setup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("IsOrganizationUnitExists", mock.Anything, "ou-1").
					Return(true, nil).
					Once()
				store.On("GetOrganizationUnitChildrenList", mock.Anything, "ou-1", 5, 0, mock.Anything).
					Return(nil, ErrResultLimitExceededInCompositeMode).
					Once()
			},
			wantErr: &ErrorResultLimitExceeded,
		},
		{
			name:  "count failure",
			limit: 5,
			setup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("IsOrganizationUnitExists", mock.Anything, "ou-1").
					Return(true, nil).
					Once()
				store.On("GetOrganizationUnitChildrenList", mock.Anything, "ou-1", 5, 0, mock.Anything).
					Return([]OrganizationUnitBasic{}, nil).
					Once()
				store.On("GetOrganizationUnitChildrenCount", mock.Anything, "ou-1", mock.Anything).
					Return(0, errors.New("count fail")).
					Once()
			},
			wantErr: &serviceerror.InternalServerError,
		},
		{
			name:  "success",
			limit: 2,
			setup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("IsOrganizationUnitExists", mock.Anything, "ou-1").
					Return(true, nil).
					Once()
				store.On("GetOrganizationUnitChildrenList", mock.Anything, "ou-1", 2, 0, mock.Anything).
					Return([]OrganizationUnitBasic{
						{ID: "child-1", Handle: "finance", Name: "Finance"},
					}, nil).
					Once()
				store.On("GetOrganizationUnitChildrenCount", mock.Anything, "ou-1", mock.Anything).
					Return(1, nil).
					Once()
			},
			wantResult: &ouListExpectations{
				totalResults: 1,
				count:        1,
				startIndex:   1,
				handles:      []string{"finance"},
				linkRels:     []string{},
				linkHrefs:    []string{},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			store := newOrganizationUnitStoreInterfaceMock(suite.T())
			if tc.setup != nil {
				tc.setup(store)
			}

			service := suite.newService(store, newAllowAllAuthz(suite.T()))
			resp, err := service.GetOrganizationUnitChildren(
				context.Background(), "ou-1", tc.limit, tc.offset, tc.filterExpr,
			)

			if tc.wantErr != nil {
				suite.Require().NotNil(err)
				suite.Require().Equal(*tc.wantErr, *err)
			} else {
				suite.Require().Nil(err)
				suite.assertOUListResponse(resp, tc.wantResult)
			}

			if tc.wantErr == &ErrorInvalidLimit || tc.wantErr == &ErrorInvalidFilter {
				store.AssertNumberOfCalls(suite.T(), "IsOrganizationUnitExists", 0)
			}
		})
	}
}

func (suite *OrganizationUnitServiceTestSuite) TestOUService_validateOUHandle() {
	service := &organizationUnitService{}

	suite.Run("blank handle", func() {
		err := service.validateOUHandle("   ")
		suite.Require().Equal(ErrorInvalidRequestFormat, *err)
	})

	suite.Run("handle with slash", func() {
		err := service.validateOUHandle("root/child")
		suite.Require().Equal(ErrorInvalidRequestFormat, *err)
	})

	suite.Run("valid handle", func() {
		err := service.validateOUHandle("finance")
		suite.Require().Nil(err)
	})
}

func (suite *OrganizationUnitServiceTestSuite) TestOUService_GetOrganizationUnitChildrenByPath() {
	config := newDefaultPathListConfig[*OrganizationUnitListResponse](
		" ",
		5,
		0,
		"GetOrganizationUnitChildrenList",
		[]OrganizationUnitBasic{},
		"GetOrganizationUnitChildrenCount",
		0,
		func(resp *OrganizationUnitListResponse) {
			suite.Require().NotNil(resp)
		},
		invokeChildrenByPath,
		mock.Anything, // filter arg
	)
	runOUPathListTests(suite, config)
}

func (suite *OrganizationUnitServiceTestSuite) TestOUService_GetOrganizationUnitUsers() {
	testCases := []struct {
		name          string
		limit         int
		offset        int
		storeSetup    func(*organizationUnitStoreInterfaceMock)
		resolverSetup func(*OUUserResolverMock)
		nilResolver   bool
		wantErr       *serviceerror.ServiceError
	}{
		{
			name:    "invalid pagination",
			limit:   0,
			offset:  0,
			wantErr: &ErrorInvalidLimit,
		},
		{
			name:        "nil user resolver",
			limit:       5,
			nilResolver: true,
			wantErr:     &serviceerror.InternalServerError,
		},
		{
			name:  "not found",
			limit: 5,
			storeSetup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("IsOrganizationUnitExists", mock.Anything, "ou-1").
					Return(false, nil).Once()
			},
			wantErr: &ErrorOrganizationUnitNotFound,
		},
		{
			name:  "existence error",
			limit: 5,
			storeSetup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("IsOrganizationUnitExists", mock.Anything, "ou-1").
					Return(false, errors.New("boom")).Once()
			},
			wantErr: &serviceerror.InternalServerError,
		},
		{
			name:  "list error",
			limit: 5,
			storeSetup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("IsOrganizationUnitExists", mock.Anything, "ou-1").
					Return(true, nil).Once()
			},
			resolverSetup: func(ur *OUUserResolverMock) {
				ur.On("GetUserListByOUID", mock.Anything, "ou-1", 5, 0, false).
					Return([]User(nil), errors.New("list")).Once()
			},
			wantErr: &serviceerror.InternalServerError,
		},
		{
			name:  "count error",
			limit: 5,
			storeSetup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("IsOrganizationUnitExists", mock.Anything, "ou-1").
					Return(true, nil).Once()
			},
			resolverSetup: func(ur *OUUserResolverMock) {
				ur.On("GetUserListByOUID", mock.Anything, "ou-1", 5, 0, false).
					Return([]User{{ID: "user-1"}}, nil).Once()
				ur.On("GetUserCountByOUID", mock.Anything, "ou-1").
					Return(0, errors.New("count")).Once()
			},
			wantErr: &serviceerror.InternalServerError,
		},
		{
			name:  "success",
			limit: 5,
			storeSetup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("IsOrganizationUnitExists", mock.Anything, "ou-1").
					Return(true, nil).Once()
			},
			resolverSetup: func(ur *OUUserResolverMock) {
				ur.On("GetUserListByOUID", mock.Anything, "ou-1", 5, 0, false).
					Return([]User{{ID: "user-1"}, {ID: "user-2"}}, nil).Once()
				ur.On("GetUserCountByOUID", mock.Anything, "ou-1").
					Return(2, nil).Once()
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			store := newOrganizationUnitStoreInterfaceMock(suite.T())
			if tc.storeSetup != nil {
				tc.storeSetup(store)
			}

			var userRes OUUserResolver
			if !tc.nilResolver {
				mock := new(OUUserResolverMock)
				if tc.resolverSetup != nil {
					tc.resolverSetup(mock)
				}
				userRes = mock
			}

			service := suite.newServiceWithResolvers(
				store, newAllowAllAuthz(suite.T()), userRes, nil,
			)
			resp, err := service.GetOrganizationUnitUsers(context.Background(), "ou-1", tc.limit, tc.offset, false)

			if tc.wantErr != nil {
				suite.Require().Nil(resp)
				suite.Require().Equal(*tc.wantErr, *err)
			} else {
				suite.Require().Nil(err)
				suite.Require().Equal(2, resp.TotalResults)
				suite.Require().Len(resp.Users, 2)
			}

			if tc.wantErr == &ErrorInvalidLimit {
				store.AssertNumberOfCalls(suite.T(), "IsOrganizationUnitExists", 0)
			}
		})
	}
}

func (suite *OrganizationUnitServiceTestSuite) TestOUService_GetOrganizationUnitUsers_WithDisplay() {
	store := newOrganizationUnitStoreInterfaceMock(suite.T())
	store.On("IsOrganizationUnitExists", mock.Anything, "ou-1").
		Return(true, nil).Once()

	userRes := NewOUUserResolverMock(suite.T())
	userRes.On("GetUserListByOUID", mock.Anything, "ou-1", 5, 0, true).
		Return([]User{
			{ID: "user-1", Type: "employee", Display: "alice@example.com"},
			{ID: "user-2", Type: "contractor", Display: "Bob Smith"},
		}, nil).Once()
	userRes.On("GetUserCountByOUID", mock.Anything, "ou-1").
		Return(2, nil).Once()

	service := suite.newServiceWithResolvers(
		store, newAllowAllAuthz(suite.T()), userRes, nil,
	)

	resp, err := service.GetOrganizationUnitUsers(context.Background(), "ou-1", 5, 0, true)
	suite.Require().Nil(err)
	suite.Require().Len(resp.Users, 2)
	suite.Equal("employee", resp.Users[0].Type)
	suite.Equal("alice@example.com", resp.Users[0].Display)
	suite.Equal("contractor", resp.Users[1].Type)
	suite.Equal("Bob Smith", resp.Users[1].Display)
}

func (suite *OrganizationUnitServiceTestSuite) TestOUService_GetOrganizationUnitUsers_WithDisplay_FallbackToID() {
	// When the resolver cannot resolve a display attribute (schema error or attribute mismatch),
	// it falls back to the user ID. The OU service simply passes through whatever the resolver returns.
	store := newOrganizationUnitStoreInterfaceMock(suite.T())
	store.On("IsOrganizationUnitExists", mock.Anything, "ou-1").
		Return(true, nil).Once()

	userRes := NewOUUserResolverMock(suite.T())
	userRes.On("GetUserListByOUID", mock.Anything, "ou-1", 5, 0, true).
		Return([]User{
			{ID: "user-1", Type: "employee", Display: "user-1"},
		}, nil).Once()
	userRes.On("GetUserCountByOUID", mock.Anything, "ou-1").
		Return(1, nil).Once()

	service := suite.newServiceWithResolvers(
		store, newAllowAllAuthz(suite.T()), userRes, nil,
	)

	resp, err := service.GetOrganizationUnitUsers(context.Background(), "ou-1", 5, 0, true)
	suite.Require().Nil(err)
	suite.Require().Len(resp.Users, 1)
	suite.Equal("user-1", resp.Users[0].Display)
}

func (suite *OrganizationUnitServiceTestSuite) TestOUService_GetOrganizationUnitUsers_AccessDenied() {
	store := newOrganizationUnitStoreInterfaceMock(suite.T())
	authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(suite.T())
	authzMock.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
		Return(false, nil).Once()

	userRes := NewOUUserResolverMock(suite.T())
	service := suite.newServiceWithResolvers(store, authzMock, userRes, nil)

	resp, err := service.GetOrganizationUnitUsers(context.Background(), "ou-1", 5, 0, false)
	suite.Require().Nil(resp)
	suite.Require().Equal(serviceerror.ErrorUnauthorized.Code, err.Code)
	suite.Require().Equal(serviceerror.ErrorUnauthorized.Type, err.Type)
	suite.Require().Equal(serviceerror.ErrorUnauthorized.Error.DefaultValue, err.Error.DefaultValue)
	suite.Require().Equal(serviceerror.ErrorUnauthorized.ErrorDescription.DefaultValue,
		err.ErrorDescription.DefaultValue)

	// Verify no store or resolver calls were made
	store.AssertNumberOfCalls(suite.T(), "IsOrganizationUnitExists", 0)
}

func (suite *OrganizationUnitServiceTestSuite) TestOUService_GetOrganizationUnitGroups_AccessDenied() {
	store := newOrganizationUnitStoreInterfaceMock(suite.T())
	authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(suite.T())
	authzMock.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
		Return(false, nil).Once()

	groupRes := new(OUGroupResolverMock)
	service := suite.newServiceWithResolvers(store, authzMock, nil, groupRes)

	resp, err := service.GetOrganizationUnitGroups(context.Background(), "ou-1", 5, 0)
	suite.Require().Nil(resp)
	suite.Require().Equal(serviceerror.ErrorUnauthorized.Code, err.Code)
	suite.Require().Equal(serviceerror.ErrorUnauthorized.Type, err.Type)
	suite.Require().Equal(serviceerror.ErrorUnauthorized.Error.DefaultValue, err.Error.DefaultValue)
	suite.Require().Equal(serviceerror.ErrorUnauthorized.ErrorDescription.DefaultValue,
		err.ErrorDescription.DefaultValue)

	store.AssertNumberOfCalls(suite.T(), "IsOrganizationUnitExists", 0)
}

func (suite *OrganizationUnitServiceTestSuite) TestOUService_GetOrganizationUnitUsers_AuthzError() {
	store := newOrganizationUnitStoreInterfaceMock(suite.T())
	authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(suite.T())
	authzMock.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
		Return(false, &serviceerror.ServiceError{
			Code:  "500",
			Error: i18ncore.I18nMessage{DefaultValue: "authz service unavailable"},
		}).Once()

	userRes := NewOUUserResolverMock(suite.T())
	service := suite.newServiceWithResolvers(store, authzMock, userRes, nil)

	resp, err := service.GetOrganizationUnitUsers(context.Background(), "ou-1", 5, 0, false)
	suite.Require().Nil(resp)
	suite.Require().Equal(serviceerror.InternalServerError, *err)
}

func (suite *OrganizationUnitServiceTestSuite) TestOUService_GetOrganizationUnitChildren_AccessDenied() {
	store := newOrganizationUnitStoreInterfaceMock(suite.T())
	authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(suite.T())
	authzMock.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
		Return(false, nil).Once()

	service := suite.newService(store, authzMock)

	resp, err := service.GetOrganizationUnitChildren(context.Background(), "ou-1", 5, 0, nil)
	suite.Require().Nil(resp)
	suite.Require().Equal(serviceerror.ErrorUnauthorized.Code, err.Code)
	suite.Require().Equal(serviceerror.ErrorUnauthorized.Type, err.Type)
	suite.Require().Equal(serviceerror.ErrorUnauthorized.Error.DefaultValue, err.Error.DefaultValue)
	suite.Require().Equal(serviceerror.ErrorUnauthorized.ErrorDescription.DefaultValue,
		err.ErrorDescription.DefaultValue)

	store.AssertNumberOfCalls(suite.T(), "IsOrganizationUnitExists", 0)
}

func (suite *OrganizationUnitServiceTestSuite) TestOUService_GetOrganizationUnitChildren_AuthzError() {
	store := newOrganizationUnitStoreInterfaceMock(suite.T())
	authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(suite.T())
	authzMock.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
		Return(false, &serviceerror.ServiceError{
			Code:  "500",
			Error: i18ncore.I18nMessage{DefaultValue: "authz service unavailable"},
		}).Once()

	service := suite.newService(store, authzMock)

	resp, err := service.GetOrganizationUnitChildren(context.Background(), "ou-1", 5, 0, nil)
	suite.Require().Nil(resp)
	suite.Require().Equal(serviceerror.InternalServerError, *err)
}

func (suite *OrganizationUnitServiceTestSuite) TestOUService_GetOrganizationUnitGroups_AuthzError() {
	store := newOrganizationUnitStoreInterfaceMock(suite.T())
	authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(suite.T())
	authzMock.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
		Return(false, &serviceerror.ServiceError{
			Code:  "500",
			Error: i18ncore.I18nMessage{DefaultValue: "authz service unavailable"},
		}).Once()

	groupRes := new(OUGroupResolverMock)
	service := suite.newServiceWithResolvers(store, authzMock, nil, groupRes)

	resp, err := service.GetOrganizationUnitGroups(context.Background(), "ou-1", 5, 0)
	suite.Require().Nil(resp)
	suite.Require().Equal(serviceerror.InternalServerError, *err)
}

// runResolverPathListTests runs common path-based list tests for user/group resolver-backed endpoints.
func (suite *OrganizationUnitServiceTestSuite) runResolverPathListTests(
	invalidPath string,
	setupResolvers func() (OUUserResolver, OUGroupResolver),
	invoke func(*organizationUnitService, string, int, int) (interface{}, *serviceerror.ServiceError),
) {
	suite.Run("invalid path", func() {
		store := newOrganizationUnitStoreInterfaceMock(suite.T())
		service := suite.newService(store, newAllowAllAuthz(suite.T()))

		resp, err := invoke(service, invalidPath, 5, 0)
		suite.Require().Nil(resp)
		suite.Require().Equal(ErrorInvalidHandlePath, *err)
	})

	suite.Run("not found", func() {
		store := newOrganizationUnitStoreInterfaceMock(suite.T())
		store.On("GetOrganizationUnitByPath", mock.Anything, []string{"root"}).
			Return(OrganizationUnit{}, ErrOrganizationUnitNotFound).Once()

		service := suite.newService(store, newAllowAllAuthz(suite.T()))
		resp, err := invoke(service, "root", 5, 0)
		suite.Require().Nil(resp)
		suite.Require().Equal(ErrorOrganizationUnitNotFound, *err)
	})

	suite.Run("store error", func() {
		store := newOrganizationUnitStoreInterfaceMock(suite.T())
		store.On("GetOrganizationUnitByPath", mock.Anything, []string{"root"}).
			Return(OrganizationUnit{}, errors.New("db connection failed")).Once()

		service := suite.newService(store, newAllowAllAuthz(suite.T()))
		resp, err := invoke(service, "root", 5, 0)
		suite.Require().Nil(resp)
		suite.Require().Equal(serviceerror.InternalServerError, *err)
	})

	suite.Run("success", func() {
		store := newOrganizationUnitStoreInterfaceMock(suite.T())
		store.On("GetOrganizationUnitByPath", mock.Anything, []string{"root"}).
			Return(OrganizationUnit{ID: "ou-1"}, nil).Once()
		store.On("IsOrganizationUnitExists", mock.Anything, "ou-1").
			Return(true, nil).Once()

		userRes, groupRes := setupResolvers()
		service := suite.newServiceWithResolvers(
			store, newAllowAllAuthz(suite.T()), userRes, groupRes,
		)
		resp, err := invoke(service, "root", 5, 0)
		suite.Require().Nil(err)
		suite.Require().NotNil(resp)
	})
}

func (suite *OrganizationUnitServiceTestSuite) TestOUService_GetOrganizationUnitUsersByPath() {
	suite.runResolverPathListTests("   ",
		func() (OUUserResolver, OUGroupResolver) {
			userRes := new(OUUserResolverMock)
			userRes.On("GetUserListByOUID", mock.Anything, "ou-1", 5, 0, false).
				Return([]User{}, nil).Once()
			userRes.On("GetUserCountByOUID", mock.Anything, "ou-1").
				Return(0, nil).Once()
			return userRes, nil
		},
		func(svc *organizationUnitService, path string, limit,
			offset int) (interface{}, *serviceerror.ServiceError) {
			return svc.GetOrganizationUnitUsersByPath(context.Background(), path, limit, offset, false)
		},
	)
}

func (suite *OrganizationUnitServiceTestSuite) TestOUService_GetOrganizationUnitUsersByPath_WithDisplay() {
	store := newOrganizationUnitStoreInterfaceMock(suite.T())
	store.On("GetOrganizationUnitByPath", mock.Anything, []string{"engineering"}).
		Return(OrganizationUnit{ID: "ou-1"}, nil).Once()
	store.On("IsOrganizationUnitExists", mock.Anything, "ou-1").
		Return(true, nil).Once()

	userRes := NewOUUserResolverMock(suite.T())
	userRes.On("GetUserListByOUID", mock.Anything, "ou-1", 2, 0, true).
		Return([]User{
			{ID: "user-1", Type: "employee", Display: "alice@example.com"},
			{ID: "user-2", Type: "employee", Display: "bob@example.com"},
		}, nil).Once()
	userRes.On("GetUserCountByOUID", mock.Anything, "ou-1").
		Return(4, nil).Once()

	service := suite.newServiceWithResolvers(
		store, newAllowAllAuthz(suite.T()), userRes, nil,
	)

	resp, err := service.GetOrganizationUnitUsersByPath(
		context.Background(), "engineering", 2, 0, true,
	)
	suite.Require().Nil(err)
	suite.Require().Len(resp.Users, 2)
	suite.Equal("employee", resp.Users[0].Type)
	suite.Equal("alice@example.com", resp.Users[0].Display)
	suite.Equal("employee", resp.Users[1].Type)
	suite.Equal("bob@example.com", resp.Users[1].Display)

	// Verify pagination links preserve the include=display query parameter
	suite.Require().NotEmpty(resp.Links)
	for _, link := range resp.Links {
		suite.Contains(link.Href, "include=display",
			"pagination link %q should preserve include=display", link.Rel)
	}
}

func (suite *OrganizationUnitServiceTestSuite) TestOUService_GetOrganizationUnitGroups() {
	testCases := []struct {
		name          string
		limit         int
		offset        int
		storeSetup    func(*organizationUnitStoreInterfaceMock)
		resolverSetup func(*OUGroupResolverMock)
		nilResolver   bool
		wantErr       *serviceerror.ServiceError
		assert        func(*GroupListResponse)
	}{
		{
			name:    "invalid pagination",
			limit:   0,
			wantErr: &ErrorInvalidLimit,
		},
		{
			name:        "nil group resolver",
			limit:       5,
			nilResolver: true,
			wantErr:     &serviceerror.InternalServerError,
		},
		{
			name:  "list error",
			limit: 5,
			storeSetup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("IsOrganizationUnitExists", mock.Anything, "ou-1").
					Return(true, nil).Once()
			},
			resolverSetup: func(gr *OUGroupResolverMock) {
				gr.On("GetGroupListByOUID", mock.Anything, "ou-1", 5, 0).
					Return([]Group(nil), errors.New("boom")).Once()
			},
			wantErr: &serviceerror.InternalServerError,
		},
		{
			name:  "success",
			limit: 5,
			storeSetup: func(store *organizationUnitStoreInterfaceMock) {
				store.On("IsOrganizationUnitExists", mock.Anything, "ou-1").
					Return(true, nil).Once()
			},
			resolverSetup: func(gr *OUGroupResolverMock) {
				gr.On("GetGroupListByOUID", mock.Anything, "ou-1", 5, 0).
					Return([]Group{{ID: "g1"}}, nil).Once()
				gr.On("GetGroupCountByOUID", mock.Anything, "ou-1").
					Return(1, nil).Once()
			},
			assert: func(resp *GroupListResponse) {
				suite.Require().Equal(1, resp.TotalResults)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			store := newOrganizationUnitStoreInterfaceMock(suite.T())
			if tc.storeSetup != nil {
				tc.storeSetup(store)
			}

			var groupRes OUGroupResolver
			if !tc.nilResolver {
				mock := new(OUGroupResolverMock)
				if tc.resolverSetup != nil {
					tc.resolverSetup(mock)
				}
				groupRes = mock
			}

			service := suite.newServiceWithResolvers(
				store, newAllowAllAuthz(suite.T()), nil, groupRes,
			)
			resp, err := service.GetOrganizationUnitGroups(context.Background(), "ou-1", tc.limit, tc.offset)

			if tc.wantErr != nil {
				suite.Require().Nil(resp)
				suite.Require().Equal(*tc.wantErr, *err)
				if tc.wantErr == &ErrorInvalidLimit {
					store.AssertNumberOfCalls(suite.T(), "IsOrganizationUnitExists", 0)
				}
				return
			}

			suite.Require().Nil(err)
			if tc.assert != nil {
				tc.assert(resp)
			}
		})
	}
}

func (suite *OrganizationUnitServiceTestSuite) TestOUService_BuildUserListResponse() {
	users := []User{{ID: "u1"}, {ID: "u2"}}
	resp, err := buildUserListResponse("/organization-units", users, 10, 5, 0, false)
	suite.Nil(err)
	suite.NotNil(resp)
	suite.Equal(10, resp.TotalResults)
	suite.Equal(2, resp.Count)
	suite.Equal(1, resp.StartIndex)
}

func (suite *OrganizationUnitServiceTestSuite) TestOUService_BuildGroupListResponse_InvalidType() {
	resp, err := buildGroupListResponse("/organization-units", 123, 10, 5, 0)
	suite.Nil(resp)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError, *err)
}

func (suite *OrganizationUnitServiceTestSuite) TestOUService_BuildOrganizationUnitListResponse_InvalidType() {
	resp, err := buildOrganizationUnitListResponse("/organization-units", struct{}{}, 10, 5, 0)
	suite.Nil(resp)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError, *err)
}

func (suite *OrganizationUnitServiceTestSuite) TestOUService_GetOrganizationUnitGroupsByPath() {
	suite.runResolverPathListTests("  ",
		func() (OUUserResolver, OUGroupResolver) {
			groupRes := new(OUGroupResolverMock)
			groupRes.On("GetGroupListByOUID", mock.Anything, "ou-1", 5, 0).
				Return([]Group{}, nil).Once()
			groupRes.On("GetGroupCountByOUID", mock.Anything, "ou-1").
				Return(0, nil).Once()
			return nil, groupRes
		},
		func(svc *organizationUnitService, path string, limit,
			offset int) (interface{}, *serviceerror.ServiceError) {
			return svc.GetOrganizationUnitGroupsByPath(context.Background(), path, limit, offset)
		},
	)
}

func TestOUService_ValidateAndProcessHandlePath(t *testing.T) {
	t.Run("invalid path", func(t *testing.T) {
		handles, err := validateAndProcessHandlePath("   ")

		require.Nil(t, handles)
		require.Equal(t, &ErrorInvalidHandlePath, err)
	})

	t.Run("only slashes", func(t *testing.T) {
		handles, err := validateAndProcessHandlePath("///")

		require.Nil(t, handles)
		require.Equal(t, &ErrorInvalidHandlePath, err)
	})

	t.Run("success", func(t *testing.T) {
		handles, err := validateAndProcessHandlePath(" /root/ child / ")

		require.Nil(t, err)
		require.Equal(t, []string{"root", "child"}, handles)
	})

	t.Run("ignores empty segments", func(t *testing.T) {
		handles, err := validateAndProcessHandlePath("root//child")

		require.Nil(t, err)
		require.Equal(t, []string{"root", "child"}, handles)
	})
}

func TestOUService_ValidatePaginationParams(t *testing.T) {
	require.Equal(t, &ErrorInvalidLimit, validatePaginationParams(0, 0))
	require.Equal(t, &ErrorInvalidLimit, validatePaginationParams(serverconst.MaxPageSize+1, 0))
	require.Equal(t, &ErrorInvalidOffset, validatePaginationParams(10, -1))
	require.Nil(t, validatePaginationParams(10, 0))
}

func TestOUService_BuildPaginationLinks(t *testing.T) {
	links := utils.BuildPaginationLinks("/organization-units", 5, 5, 20, "")
	require.Len(t, links, 4)
	require.Equal(t, "first", links[0].Rel)
	require.Equal(t, "/organization-units?offset=0&limit=5", links[0].Href)

	require.Equal(t, "prev", links[1].Rel)
	require.Equal(t, "/organization-units?offset=0&limit=5", links[1].Href)

	require.Equal(t, "next", links[2].Rel)
	require.Equal(t, "/organization-units?offset=10&limit=5", links[2].Href)

	require.Equal(t, "last", links[3].Rel)
	require.Equal(t, "/organization-units?offset=15&limit=5", links[3].Href)
}

func (suite *OrganizationUnitServiceTestSuite) TestOUService_CheckCircularDependency() {
	store := newOrganizationUnitStoreInterfaceMock(suite.T())
	service := suite.newService(store, newAllowAllAuthz(suite.T()))
	parentID := testParentID

	store.On("GetOrganizationUnit", mock.Anything, parentID).
		Return(OrganizationUnit{ID: parentID, Parent: &testGrandID}, nil).
		Once()
	store.On("GetOrganizationUnit", mock.Anything, "grand").
		Return(OrganizationUnit{ID: "grand", Parent: &testOUID}, nil).
		Once()

	err := service.checkCircularDependency(context.Background(), "ou-1", &parentID)

	suite.Require().Equal(&ErrorCircularDependency, err)

	store2 := newOrganizationUnitStoreInterfaceMock(suite.T())
	service2 := suite.newService(store2, newAllowAllAuthz(suite.T()))
	store2.On("GetOrganizationUnit", mock.Anything, parentID).
		Return(OrganizationUnit{}, ErrOrganizationUnitNotFound).
		Once()

	err = service2.checkCircularDependency(context.Background(), "ou-1", &parentID)
	suite.Require().Nil(err)

	store3 := newOrganizationUnitStoreInterfaceMock(suite.T())
	service3 := suite.newService(store3, newAllowAllAuthz(suite.T()))
	store3.On("GetOrganizationUnit", mock.Anything, parentID).
		Return(OrganizationUnit{}, errors.New("boom")).
		Once()

	err = service3.checkCircularDependency(context.Background(), "ou-1", &parentID)
	suite.Require().Equal(&serviceerror.InternalServerError, err)
}

func (suite *OrganizationUnitServiceTestSuite) TestOUService_UpdateOrganizationUnit_SameParent() {
	parentID := testParentOUID

	suite.Run("skips conflict checks when name handle and parent unchanged", func() {
		store := newOrganizationUnitStoreInterfaceMock(suite.T())
		existing := OrganizationUnit{
			ID:          testOUID,
			Handle:      "finance",
			Name:        "Finance",
			Description: "old",
			Parent:      &parentID,
		}
		store.On("GetOrganizationUnit", mock.Anything, testOUID).
			Return(existing, nil).
			Once()
		store.On("IsOrganizationUnitDeclarative", mock.Anything, testOUID).
			Return(false).
			Once()
		store.On("IsOrganizationUnitExists", mock.Anything, parentID).
			Return(true, nil).
			Once()
		// checkCircularDependency walks up from parent
		store.On("GetOrganizationUnit", mock.Anything, parentID).
			Return(OrganizationUnit{ID: parentID, Parent: nil}, nil).
			Once()
		store.On("UpdateOrganizationUnit", mock.Anything, mock.MatchedBy(func(ou OrganizationUnit) bool {
			return ou.ID == testOUID && ou.Description == "updated" && *ou.Parent == parentID
		})).
			Return(nil).
			Once()

		service := suite.newService(store, newAllowAllAuthz(suite.T()))
		result, err := service.UpdateOrganizationUnit(context.Background(), testOUID, OrganizationUnitRequestWithID{
			Handle:      "finance",
			Name:        "Finance",
			Description: "updated",
			Parent:      &parentID,
		})

		suite.Require().Nil(err)
		suite.Require().Equal("updated", result.Description)
		store.AssertNumberOfCalls(suite.T(), "CheckOrganizationUnitNameConflict", 0)
		store.AssertNumberOfCalls(suite.T(), "CheckOrganizationUnitHandleConflict", 0)
		store.AssertExpectations(suite.T())
	})

	suite.Run("runs conflict checks when parent changes from nil to value", func() {
		store := newOrganizationUnitStoreInterfaceMock(suite.T())
		existing := OrganizationUnit{
			ID:     testOUID,
			Handle: "finance",
			Name:   "Finance",
			Parent: nil,
		}
		store.On("GetOrganizationUnit", mock.Anything, testOUID).
			Return(existing, nil).
			Once()
		store.On("IsOrganizationUnitDeclarative", mock.Anything, testOUID).
			Return(false).
			Once()
		store.On("IsOrganizationUnitExists", mock.Anything, parentID).
			Return(true, nil).
			Once()
		// checkCircularDependency walks up from parent
		store.On("GetOrganizationUnit", mock.Anything, parentID).
			Return(OrganizationUnit{ID: parentID, Parent: nil}, nil).
			Once()
		store.On("CheckOrganizationUnitNameConflict", mock.Anything, "Finance", mock.MatchedBy(func(p *string) bool {
			return p != nil && *p == parentID
		})).
			Return(false, nil).
			Once()
		store.On("CheckOrganizationUnitHandleConflict", mock.Anything, "finance", mock.MatchedBy(func(p *string) bool {
			return p != nil && *p == parentID
		})).
			Return(false, nil).
			Once()
		store.On("UpdateOrganizationUnit", mock.Anything, mock.MatchedBy(func(ou OrganizationUnit) bool {
			return ou.ID == testOUID && *ou.Parent == parentID
		})).
			Return(nil).
			Once()

		service := suite.newService(store, newAllowAllAuthz(suite.T()))
		result, err := service.UpdateOrganizationUnit(context.Background(), testOUID, OrganizationUnitRequestWithID{
			Handle: "finance",
			Name:   "Finance",
			Parent: &parentID,
		})

		suite.Require().Nil(err)
		suite.Require().Equal(testOUID, result.ID)
		store.AssertExpectations(suite.T())
	})

	suite.Run("runs conflict checks when parent changes from value to nil", func() {
		store := newOrganizationUnitStoreInterfaceMock(suite.T())
		existing := OrganizationUnit{
			ID:     testOUID,
			Handle: "finance",
			Name:   "Finance",
			Parent: &parentID,
		}
		store.On("GetOrganizationUnit", mock.Anything, testOUID).
			Return(existing, nil).
			Once()
		store.On("IsOrganizationUnitDeclarative", mock.Anything, testOUID).
			Return(false).
			Once()
		store.On("CheckOrganizationUnitNameConflict", mock.Anything, "Finance", (*string)(nil)).
			Return(false, nil).
			Once()
		store.On("CheckOrganizationUnitHandleConflict", mock.Anything, "finance", (*string)(nil)).
			Return(false, nil).
			Once()
		store.On("UpdateOrganizationUnit", mock.Anything, mock.MatchedBy(func(ou OrganizationUnit) bool {
			return ou.ID == testOUID && ou.Parent == nil
		})).
			Return(nil).
			Once()

		service := suite.newService(store, newAllowAllAuthz(suite.T()))
		result, err := service.UpdateOrganizationUnit(context.Background(), testOUID, OrganizationUnitRequestWithID{
			Handle: "finance",
			Name:   "Finance",
			Parent: nil,
		})

		suite.Require().Nil(err)
		suite.Require().Equal(testOUID, result.ID)
		store.AssertExpectations(suite.T())
	})
}

func (suite *OrganizationUnitServiceTestSuite) TestOUService_StringPtrEqual() {
	suite.Run("both nil", func() {
		suite.Require().True(stringPtrEqual(nil, nil))
	})

	suite.Run("first nil", func() {
		b := "value"
		suite.Require().False(stringPtrEqual(nil, &b))
	})

	suite.Run("second nil", func() {
		a := "value"
		suite.Require().False(stringPtrEqual(&a, nil))
	})

	suite.Run("equal values", func() {
		a := "same"
		b := "same"
		suite.Require().True(stringPtrEqual(&a, &b))
	})

	suite.Run("different values", func() {
		a := "one"
		b := "two"
		suite.Require().False(stringPtrEqual(&a, &b))
	})
}

func (suite *OrganizationUnitServiceTestSuite) TestOUService_GetOrganizationUnitList_Authz() {
	testCases := []struct {
		name       string
		limit      int
		offset     int
		setupStore func(*organizationUnitStoreInterfaceMock)
		setupAuthz func(*sysauthzmock.SystemAuthorizationServiceInterfaceMock)
		wantErr    *serviceerror.ServiceError
		wantTotal  int
	}{
		{
			name:   "authz error",
			limit:  10,
			offset: 0,
			setupAuthz: func(authz *sysauthzmock.SystemAuthorizationServiceInterfaceMock) {
				authz.On("GetAccessibleResources", mock.Anything, mock.Anything, mock.Anything).
					Return((*sysauthz.AccessibleResources)(nil), &serviceerror.InternalServerError).
					Once()
			},
			wantErr: &serviceerror.InternalServerError,
		},
		{
			name:   "filtered empty",
			limit:  10,
			offset: 0,
			setupAuthz: func(authz *sysauthzmock.SystemAuthorizationServiceInterfaceMock) {
				authz.On("GetAccessibleResources", mock.Anything, mock.Anything, mock.Anything).
					Return(&sysauthz.AccessibleResources{AllAllowed: false, IDs: []string{}}, nil).
					Once()
			},
			wantTotal: 0,
		},
		{
			name:   "filtered with ids",
			limit:  10,
			offset: 0,
			setupAuthz: func(authz *sysauthzmock.SystemAuthorizationServiceInterfaceMock) {
				authz.On("GetAccessibleResources", mock.Anything, mock.Anything, mock.Anything).
					Return(&sysauthz.AccessibleResources{AllAllowed: false, IDs: []string{"ou-1"}}, nil).
					Once()
			},
			setupStore: func(store *organizationUnitStoreInterfaceMock) {
				store.On("GetOrganizationUnitsByIDs", mock.Anything, []string{"ou-1"}).
					Return([]OrganizationUnitBasic{{ID: "ou-1"}}, nil).
					Once()
			},
			wantTotal: 1,
		},
		{
			name:   "list all out of bounds",
			limit:  10,
			offset: 0,
			setupAuthz: func(authz *sysauthzmock.SystemAuthorizationServiceInterfaceMock) {
				authz.On("GetAccessibleResources", mock.Anything, mock.Anything, mock.Anything).
					Return(&sysauthz.AccessibleResources{AllAllowed: true}, nil).
					Once()
			},
			setupStore: func(store *organizationUnitStoreInterfaceMock) {
				store.On("GetOrganizationUnitListCount", mock.Anything, mock.Anything).Return(0, nil).Once()
				store.On("GetOrganizationUnitList", mock.Anything, 10, 0, mock.Anything).
					Return([]OrganizationUnitBasic{}, ErrResultLimitExceededInCompositeMode).
					Once()
			},
			wantErr: &ErrorResultLimitExceeded,
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			store := newOrganizationUnitStoreInterfaceMock(suite.T())
			authz := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(suite.T())

			if tc.setupStore != nil {
				tc.setupStore(store)
			}
			if tc.setupAuthz != nil {
				tc.setupAuthz(authz)
			}

			service := &organizationUnitService{ouStore: store, authzService: authz}
			resp, err := service.GetOrganizationUnitList(context.Background(), tc.limit, tc.offset, nil)

			if tc.wantErr != nil {
				suite.Require().NotNil(err)
				suite.Require().Equal(*tc.wantErr, *err)
			} else {
				suite.Require().Nil(err)
				suite.Require().NotNil(resp)
				suite.Require().Equal(tc.wantTotal, resp.TotalResults)
			}
			store.AssertExpectations(suite.T())
			authz.AssertExpectations(suite.T())
		})
	}
}

func (suite *OrganizationUnitServiceTestSuite) TestOUService_listAccessibleOrganizationUnits() {
	testCases := []struct {
		name       string
		ids        []string
		limit      int
		offset     int
		filter     *filter.FilterGroup
		setupStore func(*organizationUnitStoreInterfaceMock)
		wantErr    *serviceerror.ServiceError
		wantTotal  int
		wantCount  int
	}{
		{
			name:      "empty ids",
			ids:       []string{},
			limit:     10,
			offset:    0,
			wantTotal: 0,
			wantCount: 0,
		},
		{
			name:      "offset greater than total",
			ids:       []string{"ou-1"},
			limit:     10,
			offset:    5,
			wantTotal: 1,
			wantCount: 0,
		},
		{
			name:   "store error",
			ids:    []string{"ou-1"},
			limit:  10,
			offset: 0,
			setupStore: func(store *organizationUnitStoreInterfaceMock) {
				store.On("GetOrganizationUnitsByIDs", mock.Anything, []string{"ou-1"}).
					Return([]OrganizationUnitBasic{}, errors.New("boom")).
					Once()
			},
			wantErr: &serviceerror.InternalServerError,
		},
		{
			name:   "success pagination",
			ids:    []string{"ou-1", "ou-2", "ou-3"},
			limit:  2,
			offset: 1,
			setupStore: func(store *organizationUnitStoreInterfaceMock) {
				store.On("GetOrganizationUnitsByIDs", mock.Anything, []string{"ou-2", "ou-3"}).
					Return([]OrganizationUnitBasic{{ID: "ou-2"}, {ID: "ou-3"}}, nil).
					Once()
			},
			wantTotal: 3,
			wantCount: 2,
		},
		{
			name:   "filter match — returns filtered subset",
			ids:    []string{"ou-1", "ou-2"},
			limit:  10,
			offset: 0,
			filter: &filter.FilterGroup{Clauses: []filter.FilterClause{
				{Expr: filter.FilterExpression{Attribute: "name", Operator: filter.OperatorEq, Value: "Engineering"}},
			}},
			setupStore: func(store *organizationUnitStoreInterfaceMock) {
				store.On("GetOrganizationUnitsByIDs", mock.Anything, []string{"ou-1", "ou-2"}).
					Return([]OrganizationUnitBasic{
						{ID: "ou-1", Name: "Engineering"},
						{ID: "ou-2", Name: "Sales"},
					}, nil).
					Once()
			},
			wantTotal: 1,
			wantCount: 1,
		},
		{
			name:   "filter no match — returns empty",
			ids:    []string{"ou-1", "ou-2"},
			limit:  10,
			offset: 0,
			filter: &filter.FilterGroup{Clauses: []filter.FilterClause{
				{Expr: filter.FilterExpression{Attribute: "name", Operator: filter.OperatorEq, Value: "__no_match__"}},
			}},
			setupStore: func(store *organizationUnitStoreInterfaceMock) {
				store.On("GetOrganizationUnitsByIDs", mock.Anything, []string{"ou-1", "ou-2"}).
					Return([]OrganizationUnitBasic{
						{ID: "ou-1", Name: "Engineering"},
						{ID: "ou-2", Name: "Sales"},
					}, nil).
					Once()
			},
			wantTotal: 0,
			wantCount: 0,
		},
		{
			name:   "filter store error",
			ids:    []string{"ou-1"},
			limit:  10,
			offset: 0,
			filter: &filter.FilterGroup{Clauses: []filter.FilterClause{
				{Expr: filter.FilterExpression{Attribute: "name", Operator: filter.OperatorEq, Value: "Engineering"}},
			}},
			setupStore: func(store *organizationUnitStoreInterfaceMock) {
				store.On("GetOrganizationUnitsByIDs", mock.Anything, []string{"ou-1"}).
					Return(nil, errors.New("db error")).
					Once()
			},
			wantErr: &serviceerror.InternalServerError,
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			store := newOrganizationUnitStoreInterfaceMock(suite.T())
			if tc.setupStore != nil {
				tc.setupStore(store)
			}
			service := &organizationUnitService{ouStore: store}
			resp, err := service.listAccessibleOrganizationUnits(
				context.Background(), tc.ids, tc.limit, tc.offset, tc.filter)

			if tc.wantErr != nil {
				suite.Require().NotNil(err)
				suite.Require().Equal(*tc.wantErr, *err)
			} else {
				suite.Require().Nil(err)
				suite.Require().NotNil(resp)
				suite.Require().Equal(tc.wantTotal, resp.TotalResults)
				suite.Require().Equal(tc.wantCount, resp.Count)
			}
			store.AssertExpectations(suite.T())
		})
	}
}

func (suite *OrganizationUnitServiceTestSuite) TestOUService_IsOrganizationUnitDeclarative() {
	store := newOrganizationUnitStoreInterfaceMock(suite.T())
	store.On("IsOrganizationUnitDeclarative", mock.Anything, "ou-1").Return(true).Once()

	service := &organizationUnitService{ouStore: store}
	res := service.IsOrganizationUnitDeclarative(context.Background(), "ou-1")
	suite.Require().True(res)
	store.AssertExpectations(suite.T())
}

func (suite *OrganizationUnitServiceTestSuite) TestOUService_GetOrganizationUnitHandlesByIDs() {
	suite.Run("empty ids returns empty map", func() {
		service := &organizationUnitService{}
		result, svcErr := service.GetOrganizationUnitHandlesByIDs(
			context.Background(), []string{})
		suite.Require().Nil(svcErr)
		suite.Require().Empty(result)
	})

	suite.Run("success", func() {
		store := newOrganizationUnitStoreInterfaceMock(suite.T())
		store.On("GetOrganizationUnitsByIDs", mock.Anything, []string{"ou-1", "ou-2"}).
			Return([]OrganizationUnitBasic{
				{ID: "ou-1", Handle: "handle-1"},
				{ID: "ou-2", Handle: "handle-2"},
			}, nil).Once()

		service := &organizationUnitService{ouStore: store}
		result, svcErr := service.GetOrganizationUnitHandlesByIDs(
			context.Background(), []string{"ou-1", "ou-2"})
		suite.Require().Nil(svcErr)
		suite.Require().Len(result, 2)
		suite.Equal("handle-1", result["ou-1"])
		suite.Equal("handle-2", result["ou-2"])
		store.AssertExpectations(suite.T())
	})

	suite.Run("store error returns internal server error", func() {
		store := newOrganizationUnitStoreInterfaceMock(suite.T())
		store.On("GetOrganizationUnitsByIDs", mock.Anything, []string{"ou-1"}).
			Return(nil, errors.New("db error")).Once()

		service := &organizationUnitService{ouStore: store}
		result, svcErr := service.GetOrganizationUnitHandlesByIDs(
			context.Background(), []string{"ou-1"})
		suite.Require().Nil(result)
		suite.Require().NotNil(svcErr)
		suite.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
		store.AssertExpectations(suite.T())
	})
}
