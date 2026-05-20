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

package group

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entity"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

// stubTransactioner is a stub implementation of Transactioner for testing.
// It simply executes the function without actual transaction management.
type stubTransactioner struct{}

func (s *stubTransactioner) Transact(ctx context.Context, txFunc func(context.Context) error) error {
	return txFunc(ctx)
}

const (
	testOUID1 = "ou-123"
	testOUID2 = "ou-456"
)

// newAllowAllAuthz returns a mock AuthzService that grants full access.
func newAllowAllAuthz(t *testing.T) sysauthz.SystemAuthorizationServiceInterface {
	mockAuthz := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
	mockAuthz.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
		Return(true, (*serviceerror.ServiceError)(nil)).Maybe()
	mockAuthz.On("GetAccessibleResources", mock.Anything, mock.Anything, security.ResourceTypeOU).
		Return(&sysauthz.AccessibleResources{AllAllowed: true}, (*serviceerror.ServiceError)(nil)).Maybe()
	return mockAuthz
}

// newAuthzError returns a mock that simulates an internal authorization error.
func newAuthzError(t *testing.T) sysauthz.SystemAuthorizationServiceInterface {
	mockAuthz := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
	mockAuthz.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
		Return(false, &serviceerror.InternalServerError).Maybe()
	mockAuthz.On("GetAccessibleResources", mock.Anything, mock.Anything, security.ResourceTypeOU).
		Return((*sysauthz.AccessibleResources)(nil), &serviceerror.InternalServerError).Maybe()
	return mockAuthz
}

type GroupServiceTestSuite struct {
	suite.Suite
}

func TestGroupServiceTestSuite(t *testing.T) {
	suite.Run(t, new(GroupServiceTestSuite))
}

type groupRequestValidationTestCase[T any] struct {
	name    string
	request T
	wantErr bool
}

type groupListExpectations struct {
	totalResults int
	count        int
	startIndex   int
	groupNames   []string
	linkRels     []string
	linkHrefs    []string
}

func (suite *GroupServiceTestSuite) assertGroupListResponse(
	response *GroupListResponse,
	expected *groupListExpectations,
) {
	suite.Require().NotNil(response)
	suite.Require().Equal(expected.totalResults, response.TotalResults)
	suite.Require().Equal(expected.count, response.Count)
	suite.Require().Equal(expected.startIndex, response.StartIndex)
	suite.Require().Len(response.Groups, len(expected.groupNames))
	for idx, name := range expected.groupNames {
		suite.Require().Equal(name, response.Groups[idx].Name)
	}
	suite.Require().Len(response.Links, len(expected.linkRels))
	for idx := range expected.linkRels {
		suite.Require().Equal(expected.linkRels[idx], response.Links[idx].Rel)
		suite.Require().Equal(expected.linkHrefs[idx], response.Links[idx].Href)
	}
}

func runGroupRequestValidationTests[T any](
	suite *GroupServiceTestSuite,
	testCases []groupRequestValidationTestCase[T],
	validate func(T) *serviceerror.ServiceError,
) {
	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			err := validate(tc.request)
			if tc.wantErr {
				suite.Require().NotNil(err)
			} else {
				suite.Require().Nil(err)
			}
		})
	}
}

func (suite *GroupServiceTestSuite) TestGroupService_GetGroupList() {
	testCases := []struct {
		name       string
		limit      int
		offset     int
		setup      func(*groupStoreInterfaceMock)
		authzSetup func(*testing.T) sysauthz.SystemAuthorizationServiceInterface
		wantErr    *serviceerror.ServiceError
		wantResult *groupListExpectations
	}{
		{
			name:   "success",
			limit:  2,
			offset: 1,
			setup: func(storeMock *groupStoreInterfaceMock) {
				storeMock.On("GetGroupListCount", mock.Anything).
					Return(3, nil).
					Once()
				storeMock.On("GetGroupList", mock.Anything, 2, 1).
					Return([]GroupBasicDAO{
						{ID: "g1", Name: "group-1", Description: "desc-1", OUID: "ou-1"},
						{ID: "g2", Name: "group-2", Description: "desc-2", OUID: "ou-2"},
					}, nil).
					Once()
			},
			wantResult: &groupListExpectations{
				totalResults: 3,
				count:        2,
				startIndex:   2,
				groupNames:   []string{"group-1", "group-2"},
				linkRels:     []string{"first", "prev", "last"},
				linkHrefs: []string{"/groups?offset=0&limit=2", "/groups?offset=0&limit=2",
					"/groups?offset=2&limit=2"},
			},
		},
		{
			name:    "invalid pagination",
			limit:   0,
			offset:  0,
			wantErr: &ErrorInvalidLimit,
		},
		{
			name:   "count retrieval error",
			limit:  5,
			offset: 0,
			setup: func(storeMock *groupStoreInterfaceMock) {
				storeMock.On("GetGroupListCount", mock.Anything).
					Return(0, errors.New("count failure")).
					Once()
			},
			wantErr: &serviceerror.InternalServerError,
		},
		{
			name:   "list retrieval error",
			limit:  5,
			offset: 0,
			setup: func(storeMock *groupStoreInterfaceMock) {
				storeMock.On("GetGroupListCount", mock.Anything).
					Return(2, nil).
					Once()
				storeMock.On("GetGroupList", mock.Anything, 5, 0).
					Return(nil, errors.New("list failure")).
					Once()
			},
			wantErr: &serviceerror.InternalServerError,
		},
		{
			name:   "filtered by OUIDs",
			limit:  5,
			offset: 0,
			authzSetup: func(t *testing.T) sysauthz.SystemAuthorizationServiceInterface {
				authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
				authzMock.On(
					"GetAccessibleResources",
					mock.Anything,
					security.ActionListGroups,
					security.ResourceTypeOU,
				).Return(
					&sysauthz.AccessibleResources{AllAllowed: false, IDs: []string{testOUID1, testOUID2}},
					(*serviceerror.ServiceError)(nil),
				)
				return authzMock
			},
			setup: func(storeMock *groupStoreInterfaceMock) {
				ouIDs := []string{testOUID1, testOUID2}
				storeMock.On("GetGroupListCountByOUIDs", mock.Anything, ouIDs).Return(1, nil).Once()
				storeMock.On("GetGroupListByOUIDs", mock.Anything, ouIDs, 5, 0).
					Return([]GroupBasicDAO{{ID: "id1", Name: "name1", OUID: testOUID1}}, nil).Once()
			},
			wantResult: &groupListExpectations{
				totalResults: 1,
				count:        1,
				startIndex:   1,
				groupNames:   []string{"name1"},
				linkRels:     []string{},
				linkHrefs:    []string{},
			},
		},
		{
			name:   "empty OUIDs returns empty list",
			limit:  5,
			offset: 0,
			authzSetup: func(t *testing.T) sysauthz.SystemAuthorizationServiceInterface {
				authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
				authzMock.On(
					"GetAccessibleResources",
					mock.Anything,
					security.ActionListGroups,
					security.ResourceTypeOU,
				).Return(
					&sysauthz.AccessibleResources{AllAllowed: false, IDs: []string{}},
					(*serviceerror.ServiceError)(nil),
				)
				return authzMock
			},
			wantResult: &groupListExpectations{
				totalResults: 0,
				count:        0,
				startIndex:   1,
				groupNames:   []string{},
				linkRels:     []string{},
				linkHrefs:    []string{},
			},
		},
		{
			name:       "authz error",
			limit:      5,
			offset:     0,
			authzSetup: newAuthzError,
			wantErr:    &serviceerror.InternalServerError,
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			storeMock := newGroupStoreInterfaceMock(suite.T())

			if tc.setup != nil {
				tc.setup(storeMock)
			}

			var authzSvc sysauthz.SystemAuthorizationServiceInterface
			if tc.authzSetup != nil {
				authzSvc = tc.authzSetup(suite.T())
			} else {
				authzSvc = newAllowAllAuthz(suite.T())
			}
			service := &groupService{
				authzService: authzSvc,
				groupStore:   storeMock,
			}

			response, err := service.GetGroupList(context.Background(), tc.limit, tc.offset, false)

			if tc.wantErr != nil {
				suite.Require().Nil(response)
				suite.Require().NotNil(err)
				suite.Require().Equal(*tc.wantErr, *err)
			} else {
				suite.Require().Nil(err)
				suite.assertGroupListResponse(response, tc.wantResult)
			}

			if tc.wantErr == &ErrorInvalidLimit {
				storeMock.AssertNotCalled(suite.T(), "GetGroupListCount", mock.Anything)
			}
			storeMock.AssertExpectations(suite.T())
		})
	}
}
func (suite *GroupServiceTestSuite) TestGroupService_GetGroupsByPath() {
	testCases := []struct {
		name   string
		path   string
		limit  int
		offset int
		setup  func(
			*groupStoreInterfaceMock,
			*oumock.OrganizationUnitServiceInterfaceMock,
		) *serviceerror.ServiceError
		authzSetup          func(*testing.T) sysauthz.SystemAuthorizationServiceInterface
		wantErr             *serviceerror.ServiceError
		wantErrFromSetup    bool
		wantResult          *groupListExpectations
		assertStoreCalls    func(*groupStoreInterfaceMock)
		assertOUServiceCall func(*oumock.OrganizationUnitServiceInterfaceMock)
	}{
		{
			name:   "success",
			path:   "root/child",
			limit:  2,
			offset: 0,
			setup: func(
				storeMock *groupStoreInterfaceMock,
				ouMock *oumock.OrganizationUnitServiceInterfaceMock,
			) *serviceerror.ServiceError {
				storeMock.On("GetGroupsByOrganizationUnitCount", mock.Anything, "ou-123").
					Return(4, nil).
					Once()
				storeMock.On("GetGroupsByOrganizationUnit", mock.Anything, "ou-123", 2, 0).
					Return([]GroupBasicDAO{
						{ID: "g1", Name: "group-1", OUID: "ou-123"},
						{ID: "g2", Name: "group-2", OUID: "ou-123"},
					}, nil).
					Once()

				ouMock.On("GetOrganizationUnitByPath", mock.Anything, "root/child").
					Return(oupkg.OrganizationUnit{ID: "ou-123"}, nil).
					Once()
				return nil
			},
			wantResult: &groupListExpectations{
				totalResults: 4,
				count:        2,
				startIndex:   1,
				groupNames:   []string{"group-1", "group-2"},
				linkRels:     []string{"next", "last"},
				linkHrefs: []string{
					"/groups/tree/root/child?offset=2&limit=2",
					"/groups/tree/root/child?offset=2&limit=2",
				},
			},
		},
		{
			name:    "invalid path",
			path:    "  ",
			limit:   10,
			offset:  0,
			wantErr: &ErrorInvalidRequestFormat,
			assertOUServiceCall: func(ouMock *oumock.OrganizationUnitServiceInterfaceMock) {
				ouMock.AssertNotCalled(suite.T(), "GetOrganizationUnitByPath", mock.Anything)
			},
			assertStoreCalls: func(storeMock *groupStoreInterfaceMock) {
				storeMock.AssertNotCalled(suite.T(), "GetGroupsByOrganizationUnitCount", mock.Anything, mock.Anything)
			},
		},
		{
			name:   "organization unit not found",
			path:   "root/child",
			limit:  10,
			offset: 0,
			setup: func(
				storeMock *groupStoreInterfaceMock,
				ouMock *oumock.OrganizationUnitServiceInterfaceMock,
			) *serviceerror.ServiceError {
				ouMock.On("GetOrganizationUnitByPath", mock.Anything, "root/child").
					Return(oupkg.OrganizationUnit{}, &oupkg.ErrorOrganizationUnitNotFound).
					Once()
				return nil
			},
			wantErr: &ErrorGroupNotFound,
			assertStoreCalls: func(storeMock *groupStoreInterfaceMock) {
				storeMock.AssertNotCalled(suite.T(), "GetGroupsByOrganizationUnitCount", mock.Anything, mock.Anything)
			},
		},
		{
			name:   "organization unit service error",
			path:   "root/child",
			limit:  5,
			offset: 0,
			setup: func(
				storeMock *groupStoreInterfaceMock,
				ouMock *oumock.OrganizationUnitServiceInterfaceMock,
			) *serviceerror.ServiceError {
				expectedErr := &serviceerror.ServiceError{
					Code: "OU-5000",
					Type: serviceerror.ServerErrorType,
				}
				ouMock.On("GetOrganizationUnitByPath", mock.Anything, "root/child").
					Return(oupkg.OrganizationUnit{}, expectedErr).
					Once()
				return expectedErr
			},
			wantErrFromSetup: true,
		},
		{
			name:   "invalid pagination",
			path:   "root/child",
			limit:  0,
			offset: 0,
			setup: func(
				storeMock *groupStoreInterfaceMock,
				ouMock *oumock.OrganizationUnitServiceInterfaceMock,
			) *serviceerror.ServiceError {
				ouMock.On("GetOrganizationUnitByPath", mock.Anything, "root/child").
					Return(oupkg.OrganizationUnit{ID: "ou-1"}, nil).
					Once()
				return nil
			},
			wantErr: &ErrorInvalidLimit,
			assertStoreCalls: func(storeMock *groupStoreInterfaceMock) {
				storeMock.AssertNotCalled(suite.T(), "GetGroupsByOrganizationUnitCount", mock.Anything, mock.Anything)
			},
		},
		{
			name:   "count retrieval error",
			path:   "root/child",
			limit:  5,
			offset: 0,
			setup: func(
				storeMock *groupStoreInterfaceMock,
				ouMock *oumock.OrganizationUnitServiceInterfaceMock,
			) *serviceerror.ServiceError {
				storeMock.On("GetGroupsByOrganizationUnitCount", mock.Anything, "ou-123").
					Return(0, errors.New("count fail")).
					Once()

				ouMock.On("GetOrganizationUnitByPath", mock.Anything, "root/child").
					Return(oupkg.OrganizationUnit{ID: "ou-123"}, nil).
					Once()
				return nil
			},
			wantErr: &serviceerror.InternalServerError,
			assertStoreCalls: func(storeMock *groupStoreInterfaceMock) {
				storeMock.AssertNotCalled(suite.T(), "GetGroupsByOrganizationUnit",
					mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			},
		},
		{
			name:   "list retrieval error",
			path:   "root/child",
			limit:  5,
			offset: 0,
			setup: func(
				storeMock *groupStoreInterfaceMock,
				ouMock *oumock.OrganizationUnitServiceInterfaceMock,
			) *serviceerror.ServiceError {
				storeMock.On("GetGroupsByOrganizationUnitCount", mock.Anything, "ou-123").
					Return(1, nil).
					Once()
				storeMock.On("GetGroupsByOrganizationUnit", mock.Anything, "ou-123", 5, 0).
					Return(nil, errors.New("list fail")).
					Once()

				ouMock.On("GetOrganizationUnitByPath", mock.Anything, "root/child").
					Return(oupkg.OrganizationUnit{ID: "ou-123"}, nil).
					Once()
				return nil
			},
			wantErr: &serviceerror.InternalServerError,
		},
		{
			name:   "access denied",
			path:   "/org",
			limit:  5,
			offset: 0,
			setup: func(
				_ *groupStoreInterfaceMock,
				ouMock *oumock.OrganizationUnitServiceInterfaceMock,
			) *serviceerror.ServiceError {
				ouMock.On("GetOrganizationUnitByPath", mock.Anything, "/org").
					Return(oupkg.OrganizationUnit{ID: testOUID1}, (*serviceerror.ServiceError)(nil)).Once()
				return nil
			},
			authzSetup: func(t *testing.T) sysauthz.SystemAuthorizationServiceInterface {
				authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
				authzMock.On("IsActionAllowed", mock.Anything, security.ActionListGroups,
					&sysauthz.ActionContext{OUID: testOUID1, ResourceType: security.ResourceTypeGroup}).
					Return(false, (*serviceerror.ServiceError)(nil))
				return authzMock
			},
			wantErr: &serviceerror.ErrorUnauthorized,
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			storeMock := newGroupStoreInterfaceMock(suite.T())
			ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())

			var expectedErr *serviceerror.ServiceError
			if tc.setup != nil {
				expectedErr = tc.setup(storeMock, ouServiceMock)
			}

			var authzSvc sysauthz.SystemAuthorizationServiceInterface
			if tc.authzSetup != nil {
				authzSvc = tc.authzSetup(suite.T())
			} else {
				authzSvc = newAllowAllAuthz(suite.T())
			}
			service := &groupService{
				authzService: authzSvc,
				groupStore:   storeMock,
				ouService:    ouServiceMock,
			}

			response, err := service.GetGroupsByPath(context.Background(), tc.path, tc.limit, tc.offset, false)

			if tc.wantErr != nil || tc.wantErrFromSetup {
				suite.Require().Nil(response)
				suite.Require().NotNil(err)
				if tc.wantErrFromSetup {
					suite.Require().Equal(expectedErr, err)
				} else {
					suite.Require().Equal(*tc.wantErr, *err)
				}
			} else {
				suite.Require().Nil(err)
				suite.assertGroupListResponse(response, tc.wantResult)
			}

			if tc.assertStoreCalls != nil {
				tc.assertStoreCalls(storeMock)
			}
			if tc.assertOUServiceCall != nil {
				tc.assertOUServiceCall(ouServiceMock)
			}

			storeMock.AssertExpectations(suite.T())
			ouServiceMock.AssertExpectations(suite.T())
		})
	}
}

func (suite *GroupServiceTestSuite) TestGroupService_CreateGroup() {
	type setupArgs struct {
		store  *groupStoreInterfaceMock
		ou     *oumock.OrganizationUnitServiceInterfaceMock
		entity *entitymock.EntityServiceInterfaceMock
	}

	testCases := []struct {
		name       string
		request    CreateGroupRequest
		setup      func(*setupArgs)
		authzSetup func(*testing.T) sysauthz.SystemAuthorizationServiceInterface
		expectErr  *serviceerror.ServiceError
		expectRes  bool
	}{
		{
			name: "success",
			request: CreateGroupRequest{
				Name:        "engineering",
				Description: "Engineers",
				OUID:        "ou-001",
				Members: []Member{
					{ID: "usr-001", Type: MemberTypeUser},
					{ID: "grp-002", Type: MemberTypeGroup},
				},
			},
			setup: func(args *setupArgs) {
				args.store.On("CheckGroupNameConflictForCreate", mock.Anything, "engineering", "ou-001").
					Return(nil).
					Once()
				args.store.On("ValidateGroupIDs", mock.Anything, []string{"grp-002"}).
					Return([]string{}, nil).
					Once()
				args.store.On("CreateGroup", mock.Anything, mock.MatchedBy(func(group GroupDAO) bool {
					return group.Name == "engineering" &&
						group.OUID == "ou-001" &&
						len(group.Members) == 2
				})).
					Return(nil).
					Once()

				args.ou.On("IsOrganizationUnitExists", mock.Anything, "ou-001").
					Return(true, nil).
					Once()

				args.entity.On("GetEntitiesByIDs", mock.Anything, []string{"usr-001"}).
					Return([]entity.Entity{{ID: "usr-001", Category: entity.EntityCategoryUser}}, nil).
					Times(2)
			},
			expectRes: true,
		},
		{
			name: "invalid organization unit",
			request: CreateGroupRequest{
				Name: "engineering",
				OUID: "ou-unknown",
			},
			setup: func(args *setupArgs) {
				args.ou.On("IsOrganizationUnitExists", mock.Anything, "ou-unknown").
					Return(false, nil).
					Once()
			},
			expectErr: &ErrorInvalidOUID,
		},
		{
			name: "invalid user IDs",
			request: CreateGroupRequest{
				Name:    "engineering",
				OUID:    "ou-001",
				Members: []Member{{ID: "usr-invalid", Type: MemberTypeUser}},
			},
			setup: func(args *setupArgs) {
				args.ou.On("IsOrganizationUnitExists", mock.Anything, "ou-001").
					Return(true, nil).
					Once()
				args.entity.On("GetEntitiesByIDs", mock.Anything, []string{"usr-invalid"}).
					Return([]entity.Entity{}, nil).
					Once()
			},
			expectErr: &ErrorInvalidMemberID,
		},
		{
			name: "name conflict",
			request: CreateGroupRequest{
				Name: "engineering",
				OUID: "ou-001",
			},
			setup: func(args *setupArgs) {
				args.store.On("CheckGroupNameConflictForCreate", mock.Anything, "engineering", "ou-001").
					Return(ErrGroupNameConflict).
					Once()
				args.ou.On("IsOrganizationUnitExists", mock.Anything, "ou-001").
					Return(true, nil).
					Once()
			},
			expectErr: &ErrorGroupNameConflict,
		},
		{
			name: "conflict check error",
			request: CreateGroupRequest{
				Name: "engineering",
				OUID: "ou-001",
			},
			setup: func(args *setupArgs) {
				args.store.On("CheckGroupNameConflictForCreate", mock.Anything, "engineering", "ou-001").
					Return(errors.New("db failure")).
					Once()
				args.ou.On("IsOrganizationUnitExists", mock.Anything, "ou-001").
					Return(true, nil).
					Once()
			},
			expectErr: &serviceerror.InternalServerError,
		},
		{
			name: "create error",
			request: CreateGroupRequest{
				Name: "engineering",
				OUID: "ou-001",
			},
			setup: func(args *setupArgs) {
				args.store.On("CheckGroupNameConflictForCreate", mock.Anything, "engineering", "ou-001").
					Return(nil).
					Once()
				args.store.On("CreateGroup", mock.Anything, mock.Anything).
					Return(errors.New("create fail")).
					Once()
				args.ou.On("IsOrganizationUnitExists", mock.Anything, "ou-001").
					Return(true, nil).
					Once()
			},
			expectErr: &serviceerror.InternalServerError,
		},
		{
			name: "organization unit service error",
			request: CreateGroupRequest{
				Name: "engineering",
				OUID: "ou-001",
			},
			setup: func(args *setupArgs) {
				args.ou.On("IsOrganizationUnitExists", mock.Anything, "ou-001").
					Return(false,
						&serviceerror.ServiceError{Code: "OU-5000", Type: serviceerror.ServerErrorType}).
					Once()
			},
			expectErr: &serviceerror.InternalServerError,
		},
		{
			name: "access denied",
			request: CreateGroupRequest{
				Name: "developers",
				OUID: testOUID1,
			},
			setup: func(args *setupArgs) {
				args.ou.On("IsOrganizationUnitExists", mock.Anything, testOUID1).
					Return(true, (*serviceerror.ServiceError)(nil)).Once()
			},
			authzSetup: func(t *testing.T) sysauthz.SystemAuthorizationServiceInterface {
				authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
				authzMock.On("IsActionAllowed", mock.Anything, security.ActionCreateGroup,
					&sysauthz.ActionContext{OUID: testOUID1, ResourceType: security.ResourceTypeGroup}).
					Return(false, (*serviceerror.ServiceError)(nil))
				return authzMock
			},
			expectErr: &serviceerror.ErrorUnauthorized,
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			var storeMock *groupStoreInterfaceMock
			var ouServiceMock *oumock.OrganizationUnitServiceInterfaceMock
			var entityServiceMock *entitymock.EntityServiceInterfaceMock

			if tc.setup != nil {
				storeMock = newGroupStoreInterfaceMock(suite.T())
				ouServiceMock = oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())
				entityServiceMock = entitymock.NewEntityServiceInterfaceMock(suite.T())
				tc.setup(&setupArgs{store: storeMock, ou: ouServiceMock, entity: entityServiceMock})
			}

			var authzSvc sysauthz.SystemAuthorizationServiceInterface
			if tc.authzSetup != nil {
				authzSvc = tc.authzSetup(suite.T())
			} else {
				authzSvc = newAllowAllAuthz(suite.T())
			}
			service := &groupService{
				authzService:  authzSvc,
				groupStore:    storeMock,
				ouService:     ouServiceMock,
				entityService: entityServiceMock,
				transactioner: &stubTransactioner{},
			}

			group, err := service.CreateGroup(context.Background(), tc.request)

			if tc.expectErr != nil {
				suite.Require().Nil(group)
				suite.Require().NotNil(err)
				suite.Require().Equal(*tc.expectErr, *err)
			} else if tc.expectRes {
				suite.Require().Nil(err)
				suite.Require().NotNil(group)
			} else {
				suite.Require().Nil(err)
			}

			if storeMock != nil {
				storeMock.AssertExpectations(suite.T())
			}
			if ouServiceMock != nil {
				ouServiceMock.AssertExpectations(suite.T())
			}
			if entityServiceMock != nil {
				entityServiceMock.AssertExpectations(suite.T())
			}
		})
	}
}

func (suite *GroupServiceTestSuite) TestGroupService_CreateGroupByPath() {
	type setupArgs struct {
		store  *groupStoreInterfaceMock
		ou     *oumock.OrganizationUnitServiceInterfaceMock
		entity *entitymock.EntityServiceInterfaceMock
	}

	testCases := []struct {
		name      string
		path      string
		request   CreateGroupByPathRequest
		setup     func(*setupArgs) *serviceerror.ServiceError
		expectErr *serviceerror.ServiceError
	}{
		{
			name:      "invalid path",
			path:      " ",
			request:   CreateGroupByPathRequest{Name: "n"},
			expectErr: &ErrorInvalidRequestFormat,
		},
		{
			name:    "organization unit service error",
			path:    "root",
			request: CreateGroupByPathRequest{Name: "n"},
			setup: func(args *setupArgs) *serviceerror.ServiceError {
				expected := &serviceerror.ServiceError{Code: "OU-5000", Type: serviceerror.ServerErrorType}
				args.ou.On("GetOrganizationUnitByPath", mock.Anything, "root").
					Return(oupkg.OrganizationUnit{}, expected).
					Once()
				return expected
			},
			expectErr: &serviceerror.ServiceError{Code: "OU-5000", Type: serviceerror.ServerErrorType},
		},
		{
			name:    "organization unit not found",
			path:    "root",
			request: CreateGroupByPathRequest{Name: "n"},
			setup: func(args *setupArgs) *serviceerror.ServiceError {
				args.ou.On("GetOrganizationUnitByPath", mock.Anything, "root").
					Return(oupkg.OrganizationUnit{}, &oupkg.ErrorOrganizationUnitNotFound).
					Once()
				return nil
			},
			expectErr: &ErrorGroupNotFound,
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			var storeMock *groupStoreInterfaceMock
			var ouServiceMock *oumock.OrganizationUnitServiceInterfaceMock
			var entityServiceMock *entitymock.EntityServiceInterfaceMock
			var expectedOUError *serviceerror.ServiceError

			if tc.setup != nil {
				storeMock = newGroupStoreInterfaceMock(suite.T())
				ouServiceMock = oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())
				entityServiceMock = entitymock.NewEntityServiceInterfaceMock(suite.T())
				expectedOUError = tc.setup(&setupArgs{store: storeMock, ou: ouServiceMock, entity: entityServiceMock})
			}

			service := &groupService{
				authzService:  newAllowAllAuthz(suite.T()),
				groupStore:    storeMock,
				ouService:     ouServiceMock,
				entityService: entityServiceMock,
				transactioner: &stubTransactioner{},
			}

			group, err := service.CreateGroupByPath(context.Background(), tc.path, tc.request)

			if tc.expectErr != nil {
				if expectedOUError != nil {
					suite.Require().Equal(expectedOUError, err)
				} else {
					suite.Require().Nil(group)
					suite.Require().NotNil(err)
					suite.Require().Equal(*tc.expectErr, *err)
				}
			}

			if storeMock != nil {
				storeMock.AssertExpectations(suite.T())
			}
			if ouServiceMock != nil {
				ouServiceMock.AssertExpectations(suite.T())
			}
			if entityServiceMock != nil {
				entityServiceMock.AssertExpectations(suite.T())
			}
		})
	}
}

func (suite *GroupServiceTestSuite) TestGroupService_GetGroup() {
	testCases := []struct {
		name       string
		id         string
		setup      func(*groupStoreInterfaceMock)
		authzSetup func(*testing.T) sysauthz.SystemAuthorizationServiceInterface
		wantErr    *serviceerror.ServiceError
	}{
		{
			name:    "missing id",
			id:      "",
			wantErr: &ErrorMissingGroupID,
		},
		{
			name: "internal error",
			id:   "grp-001",
			setup: func(storeMock *groupStoreInterfaceMock) {
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{}, errors.New("db error")).
					Once()
			},
			wantErr: &serviceerror.InternalServerError,
		},
		{
			name: "not found",
			id:   "grp-404",
			setup: func(storeMock *groupStoreInterfaceMock) {
				storeMock.On("GetGroup", mock.Anything, "grp-404").
					Return(GroupDAO{}, ErrGroupNotFound).
					Once()
			},
			wantErr: &ErrorGroupNotFound,
		},
		{
			name: "success",
			id:   "grp-001",
			setup: func(storeMock *groupStoreInterfaceMock) {
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", Name: "test", OUID: testOUID1}, nil).
					Once()
			},
		},
		{
			name: "access denied",
			id:   "grp-001",
			setup: func(storeMock *groupStoreInterfaceMock) {
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", OUID: testOUID1}, nil).
					Once()
			},
			authzSetup: func(t *testing.T) sysauthz.SystemAuthorizationServiceInterface {
				authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
				authzMock.On(
					"IsActionAllowed",
					mock.Anything,
					security.ActionReadGroup,
					&sysauthz.ActionContext{
						OUID:         testOUID1,
						ResourceType: security.ResourceTypeGroup,
						ResourceID:   "grp-001",
					},
				).Return(false, (*serviceerror.ServiceError)(nil))
				return authzMock
			},
			wantErr: &serviceerror.ErrorUnauthorized,
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			var storeMock *groupStoreInterfaceMock

			if tc.setup != nil {
				storeMock = newGroupStoreInterfaceMock(suite.T())
				tc.setup(storeMock)
			}

			var authzSvc sysauthz.SystemAuthorizationServiceInterface
			if tc.authzSetup != nil {
				authzSvc = tc.authzSetup(suite.T())
			} else {
				authzSvc = newAllowAllAuthz(suite.T())
			}
			service := &groupService{
				authzService: authzSvc,
				groupStore:   storeMock,
			}

			group, err := service.GetGroup(context.Background(), tc.id, false)

			if tc.wantErr != nil {
				suite.Require().Nil(group)
				suite.Require().NotNil(err)
				suite.Require().Equal(*tc.wantErr, *err)
			} else {
				suite.Require().Nil(err)
				suite.Require().NotNil(group)
			}

			if storeMock != nil {
				storeMock.AssertExpectations(suite.T())
			}
		})
	}
}

func (suite *GroupServiceTestSuite) TestGroupService_GetGroup_WithIncludeDisplay() {
	suite.Run("populates OUHandle when includeDisplay is true", func() {
		storeMock := newGroupStoreInterfaceMock(suite.T())
		storeMock.On("GetGroup", mock.Anything, "grp-001").
			Return(GroupDAO{ID: "grp-001", Name: "test", OUID: testOUID1}, nil).
			Once()

		ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())
		ouServiceMock.On(
			"GetOrganizationUnitHandlesByIDs", mock.Anything, []string{testOUID1},
		).Return(map[string]string{testOUID1: "root"}, nil).Once()

		service := &groupService{
			authzService: newAllowAllAuthz(suite.T()),
			groupStore:   storeMock,
			ouService:    ouServiceMock,
		}

		group, err := service.GetGroup(context.Background(), "grp-001", true)
		suite.Require().Nil(err)
		suite.Require().NotNil(group)
		suite.Equal("root", group.OUHandle)
		storeMock.AssertExpectations(suite.T())
		ouServiceMock.AssertExpectations(suite.T())
	})

	suite.Run("does not populate OUHandle when includeDisplay is false", func() {
		storeMock := newGroupStoreInterfaceMock(suite.T())
		storeMock.On("GetGroup", mock.Anything, "grp-001").
			Return(GroupDAO{ID: "grp-001", Name: "test", OUID: testOUID1}, nil).
			Once()

		service := &groupService{
			authzService: newAllowAllAuthz(suite.T()),
			groupStore:   storeMock,
		}

		group, err := service.GetGroup(context.Background(), "grp-001", false)
		suite.Require().Nil(err)
		suite.Require().NotNil(group)
		suite.Equal("", group.OUHandle)
		storeMock.AssertExpectations(suite.T())
	})

	suite.Run("returns group with empty ouHandle when OU handle resolution fails", func() {
		storeMock := newGroupStoreInterfaceMock(suite.T())
		storeMock.On("GetGroup", mock.Anything, "grp-001").
			Return(GroupDAO{ID: "grp-001", Name: "test", OUID: testOUID1}, nil).
			Once()

		ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())
		ouServiceMock.On(
			"GetOrganizationUnitHandlesByIDs", mock.Anything, []string{testOUID1},
		).Return(
			(map[string]string)(nil), &serviceerror.ServiceError{Code: "OU-5000"},
		).Once()

		service := &groupService{
			authzService: newAllowAllAuthz(suite.T()),
			groupStore:   storeMock,
			ouService:    ouServiceMock,
		}

		group, err := service.GetGroup(context.Background(), "grp-001", true)
		suite.Require().Nil(err)
		suite.Require().NotNil(group)
		suite.Equal("grp-001", group.ID)
		suite.Empty(group.OUHandle)
	})
}

func (suite *GroupServiceTestSuite) TestGroupService_GetGroupList_WithIncludeDisplay() {
	storeMock := newGroupStoreInterfaceMock(suite.T())
	storeMock.On("GetGroupListCount", mock.Anything).Return(2, nil).Once()
	storeMock.On("GetGroupList", mock.Anything, 10, 0).
		Return([]GroupBasicDAO{
			{ID: "g1", Name: "group-1", OUID: testOUID1},
			{ID: "g2", Name: "group-2", OUID: testOUID2},
		}, nil).Once()

	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())
	ouServiceMock.On(
		"GetOrganizationUnitHandlesByIDs",
		mock.Anything,
		mock.MatchedBy(func(ids []string) bool {
			if len(ids) != 2 {
				return false
			}
			expected := map[string]bool{testOUID1: true, testOUID2: true}
			return expected[ids[0]] && expected[ids[1]]
		}),
	).Return(map[string]string{
		testOUID1: "handle-1",
		testOUID2: "handle-2",
	}, nil).Once()

	service := &groupService{
		authzService: newAllowAllAuthz(suite.T()),
		groupStore:   storeMock,
		ouService:    ouServiceMock,
	}

	response, err := service.GetGroupList(
		context.Background(), 10, 0, true)
	suite.Require().Nil(err)
	suite.Require().NotNil(response)
	suite.Require().Len(response.Groups, 2)
	suite.Equal("handle-1", response.Groups[0].OUHandle)
	suite.Equal("handle-2", response.Groups[1].OUHandle)
	storeMock.AssertExpectations(suite.T())
	ouServiceMock.AssertExpectations(suite.T())
}

func (suite *GroupServiceTestSuite) TestGroupService_UpdateGroup() {
	type setupArgs struct {
		store  *groupStoreInterfaceMock
		ou     *oumock.OrganizationUnitServiceInterfaceMock
		entity *entitymock.EntityServiceInterfaceMock
	}

	testCases := []struct {
		name        string
		groupID     string
		request     UpdateGroupRequest
		setup       func(*setupArgs)
		authzSetup  func(*testing.T) sysauthz.SystemAuthorizationServiceInterface
		expectErr   *serviceerror.ServiceError
		expectGroup bool
	}{
		{
			name:      "missing id",
			groupID:   "",
			expectErr: &ErrorMissingGroupID,
		},
		{
			name:      "invalid request",
			groupID:   "grp-001",
			request:   UpdateGroupRequest{},
			expectErr: &ErrorInvalidRequestFormat,
		},
		{
			name:    "success",
			groupID: "grp-001",
			request: UpdateGroupRequest{
				Name:        "new-name",
				Description: "New desc",
				OUID:        "ou-new",
			},
			setup: func(args *setupArgs) {
				args.store.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", Name: "old", Description: "legacy",
						OUID: "ou-old"}, nil).
					Once()
				args.store.On("CheckGroupNameConflictForUpdate", mock.Anything, "new-name", "ou-new", "grp-001").
					Return(nil).
					Once()
				args.store.On("UpdateGroup", mock.Anything, mock.MatchedBy(func(group GroupDAO) bool {
					return group.ID == "grp-001" && group.Name == "new-name" && group.OUID == "ou-new"
				})).
					Return(nil).
					Once()
				args.ou.On("IsOrganizationUnitExists", mock.Anything, "ou-new").
					Return(true, nil).
					Once()
			},
			expectGroup: true,
		},
		{
			name:    "name conflict",
			groupID: "grp-001",
			request: UpdateGroupRequest{
				Name: "new-name",
				OUID: "ou-new",
			},
			setup: func(args *setupArgs) {
				args.store.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", Name: "old", OUID: "ou-old"}, nil).
					Once()
				args.store.On("CheckGroupNameConflictForUpdate", mock.Anything, "new-name", "ou-new", "grp-001").
					Return(ErrGroupNameConflict).
					Once()
				args.ou.On("IsOrganizationUnitExists", mock.Anything, "ou-new").
					Return(true, nil).
					Once()
			},
			expectErr: &ErrorGroupNameConflict,
		},
		{
			name:    "group not found",
			groupID: "grp-001",
			request: UpdateGroupRequest{
				Name: "name",
				OUID: "ou",
			},
			setup: func(args *setupArgs) {
				args.store.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{}, ErrGroupNotFound).
					Once()
			},
			expectErr: &ErrorGroupNotFound,
		},
		{
			name:    "get group error",
			groupID: "grp-001",
			request: UpdateGroupRequest{
				Name: "name",
				OUID: "ou",
			},
			setup: func(args *setupArgs) {
				args.store.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{}, errors.New("db error")).
					Once()
			},
			expectErr: &serviceerror.InternalServerError,
		},
		{
			name:    "validate organization unit error",
			groupID: "grp-001",
			request: UpdateGroupRequest{
				Name: "name",
				OUID: "ou-new",
			},
			setup: func(args *setupArgs) {
				args.store.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", Name: "name", OUID: "ou-old"}, nil).
					Once()
				args.ou.On("IsOrganizationUnitExists", mock.Anything, "ou-new").
					Return(false, nil).
					Once()
			},
			expectErr: &ErrorInvalidOUID,
		},
		{
			name:    "conflict check error",
			groupID: "grp-001",
			request: UpdateGroupRequest{
				Name: "new",
				OUID: "ou",
			},
			setup: func(args *setupArgs) {
				args.store.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", Name: "old", OUID: "ou"}, nil).
					Once()
				args.store.On("CheckGroupNameConflictForUpdate", mock.Anything, "new", "ou", "grp-001").
					Return(errors.New("db error")).
					Once()
			},
			expectErr: &serviceerror.InternalServerError,
		},
		{
			name:    "update error",
			groupID: "grp-001",
			request: UpdateGroupRequest{
				Name: "new",
				OUID: "ou",
			},
			setup: func(args *setupArgs) {
				args.store.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", Name: "old-name", OUID: "ou"}, nil).
					Once()
				args.store.On("CheckGroupNameConflictForUpdate", mock.Anything, "new", "ou", "grp-001").
					Return(nil).
					Once()
				args.store.On("UpdateGroup", mock.Anything, mock.Anything).
					Return(errors.New("update fail")).
					Once()
			},
			expectErr: &serviceerror.InternalServerError,
		},
		{
			name:    "access denied on source OU",
			groupID: "grp-001",
			request: UpdateGroupRequest{
				Name: "new-name",
				OUID: testOUID1,
			},
			setup: func(args *setupArgs) {
				args.store.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", OUID: testOUID1}, nil).Once()
			},
			authzSetup: func(t *testing.T) sysauthz.SystemAuthorizationServiceInterface {
				authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
				authzMock.On(
					"IsActionAllowed",
					mock.Anything,
					security.ActionUpdateGroup,
					&sysauthz.ActionContext{
						OUID:         testOUID1,
						ResourceType: security.ResourceTypeGroup,
						ResourceID:   "grp-001",
					},
				).Return(false, (*serviceerror.ServiceError)(nil))
				return authzMock
			},
			expectErr: &serviceerror.ErrorUnauthorized,
		},
		{
			name:    "access denied on target OU",
			groupID: "grp-001",
			request: UpdateGroupRequest{
				Name: "new-name",
				OUID: testOUID2,
			},
			setup: func(args *setupArgs) {
				args.store.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", OUID: testOUID1}, nil).Once()
				args.ou.On("IsOrganizationUnitExists", mock.Anything, testOUID2).
					Return(true, (*serviceerror.ServiceError)(nil)).Once()
			},
			authzSetup: func(t *testing.T) sysauthz.SystemAuthorizationServiceInterface {
				authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
				authzMock.On(
					"IsActionAllowed",
					mock.Anything,
					security.ActionUpdateGroup,
					&sysauthz.ActionContext{
						OUID:         testOUID1,
						ResourceType: security.ResourceTypeGroup,
						ResourceID:   "grp-001",
					},
				).Return(true, (*serviceerror.ServiceError)(nil))
				authzMock.On(
					"IsActionAllowed",
					mock.Anything,
					security.ActionUpdateGroup,
					&sysauthz.ActionContext{
						OUID:         testOUID2,
						ResourceType: security.ResourceTypeGroup,
						ResourceID:   "grp-001",
					},
				).Return(false, (*serviceerror.ServiceError)(nil))
				return authzMock
			},
			expectErr: &serviceerror.ErrorUnauthorized,
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			var storeMock *groupStoreInterfaceMock
			var ouServiceMock *oumock.OrganizationUnitServiceInterfaceMock
			var entityServiceMock *entitymock.EntityServiceInterfaceMock

			if tc.setup != nil {
				storeMock = newGroupStoreInterfaceMock(suite.T())
				ouServiceMock = oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())
				entityServiceMock = entitymock.NewEntityServiceInterfaceMock(suite.T())
				tc.setup(&setupArgs{store: storeMock, ou: ouServiceMock, entity: entityServiceMock})
			}

			var authzSvc sysauthz.SystemAuthorizationServiceInterface
			if tc.authzSetup != nil {
				authzSvc = tc.authzSetup(suite.T())
			} else {
				authzSvc = newAllowAllAuthz(suite.T())
			}
			service := &groupService{
				authzService:  authzSvc,
				groupStore:    storeMock,
				ouService:     ouServiceMock,
				entityService: entityServiceMock,
				transactioner: &stubTransactioner{},
			}

			group, err := service.UpdateGroup(context.Background(), tc.groupID, tc.request)

			if tc.expectErr != nil {
				suite.Require().Nil(group)
				suite.Require().NotNil(err)
				suite.Require().Equal(*tc.expectErr, *err)
			} else if tc.expectGroup {
				suite.Require().Nil(err)
				suite.Require().NotNil(group)
			} else {
				suite.Require().Nil(err)
			}

			if storeMock != nil {
				storeMock.AssertExpectations(suite.T())
			}
			if ouServiceMock != nil {
				ouServiceMock.AssertExpectations(suite.T())
			}
			if entityServiceMock != nil {
				entityServiceMock.AssertExpectations(suite.T())
			}
		})
	}
}

func (suite *GroupServiceTestSuite) TestGroupService_DeleteGroup() {
	testCases := []struct {
		name       string
		id         string
		setup      func(*groupStoreInterfaceMock)
		authzSetup func(*testing.T) sysauthz.SystemAuthorizationServiceInterface
		expectErr  *serviceerror.ServiceError
	}{
		{
			name: "success",
			id:   "grp-001",
			setup: func(storeMock *groupStoreInterfaceMock) {
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001"}, nil).
					Once()
				storeMock.On("DeleteGroup", mock.Anything, "grp-001").
					Return(nil).
					Once()
			},
		},
		{
			name:      "missing id",
			id:        "",
			expectErr: &ErrorMissingGroupID,
		},
		{
			name: "get group error",
			id:   "grp-001",
			setup: func(storeMock *groupStoreInterfaceMock) {
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{}, errors.New("db error")).
					Once()
			},
			expectErr: &serviceerror.InternalServerError,
		},
		{
			name: "delete error",
			id:   "grp-001",
			setup: func(storeMock *groupStoreInterfaceMock) {
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001"}, nil).
					Once()
				storeMock.On("DeleteGroup", mock.Anything, "grp-001").
					Return(errors.New("delete fail")).
					Once()
			},
			expectErr: &serviceerror.InternalServerError,
		},
		{
			name: "group not found",
			id:   "grp-001",
			setup: func(storeMock *groupStoreInterfaceMock) {
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{}, ErrGroupNotFound).
					Once()
			},
			expectErr: &ErrorGroupNotFound,
		},
		{
			name: "access denied",
			id:   "grp-001",
			setup: func(storeMock *groupStoreInterfaceMock) {
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", OUID: testOUID1}, nil).Once()
			},
			authzSetup: func(t *testing.T) sysauthz.SystemAuthorizationServiceInterface {
				authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
				authzMock.On(
					"IsActionAllowed",
					mock.Anything,
					security.ActionDeleteGroup,
					&sysauthz.ActionContext{
						OUID:         testOUID1,
						ResourceType: security.ResourceTypeGroup,
						ResourceID:   "grp-001",
					},
				).Return(false, (*serviceerror.ServiceError)(nil))
				return authzMock
			},
			expectErr: &serviceerror.ErrorUnauthorized,
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			var storeMock *groupStoreInterfaceMock
			if tc.setup != nil {
				storeMock = newGroupStoreInterfaceMock(suite.T())
				tc.setup(storeMock)
			}

			var authzSvc sysauthz.SystemAuthorizationServiceInterface
			if tc.authzSetup != nil {
				authzSvc = tc.authzSetup(suite.T())
			} else {
				authzSvc = newAllowAllAuthz(suite.T())
			}
			service := &groupService{
				authzService:  authzSvc,
				groupStore:    storeMock,
				transactioner: &stubTransactioner{},
			}

			err := service.DeleteGroup(context.Background(), tc.id)

			if tc.expectErr != nil {
				suite.Require().NotNil(err)
				suite.Require().Equal(*tc.expectErr, *err)
			} else {
				suite.Require().Nil(err)
			}

			if storeMock != nil {
				storeMock.AssertExpectations(suite.T())
			}
		})
	}
}

func (suite *GroupServiceTestSuite) TestGroupService_GetGroupMembers() {
	testCases := []struct {
		name        string
		id          string
		limit       int
		offset      int
		setup       func(*groupStoreInterfaceMock)
		entitySetup func(*testing.T) entity.EntityServiceInterface
		authzSetup  func(*testing.T) sysauthz.SystemAuthorizationServiceInterface
		expectErr   *serviceerror.ServiceError
		expectRes   bool
	}{
		{
			name:   "success",
			id:     "grp-001",
			limit:  2,
			offset: 0,
			setup: func(storeMock *groupStoreInterfaceMock) {
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001"}, nil).
					Once()
				storeMock.On("GetGroupMemberCount", mock.Anything, "grp-001").
					Return(3, nil).
					Once()
				storeMock.On("GetGroupMembers", mock.Anything, "grp-001", 2, 0).
					Return([]Member{
						{ID: "usr-001", Type: memberTypeEntity},
						{ID: "grp-002", Type: MemberTypeGroup},
					}, nil).
					Once()
			},
			entitySetup: func(t *testing.T) entity.EntityServiceInterface {
				entitySvcMock := entitymock.NewEntityServiceInterfaceMock(t)
				entitySvcMock.On("GetEntitiesByIDs", mock.Anything, []string{"usr-001"}).
					Return([]entity.Entity{
						{ID: "usr-001", Category: entity.EntityCategoryUser},
					}, nil).Once()
				return entitySvcMock
			},
			expectRes: true,
		},
		{
			name:   "group not found",
			id:     "grp-001",
			limit:  5,
			offset: 0,
			setup: func(storeMock *groupStoreInterfaceMock) {
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{}, ErrGroupNotFound).
					Once()
			},
			expectErr: &ErrorGroupNotFound,
		},
		{
			name:      "invalid pagination",
			id:        "grp-001",
			limit:     0,
			offset:    0,
			expectErr: &ErrorInvalidLimit,
		},
		{
			name:      "missing id",
			id:        "",
			limit:     5,
			offset:    0,
			expectErr: &ErrorMissingGroupID,
		},
		{
			name:   "get group error",
			id:     "grp-001",
			limit:  5,
			offset: 0,
			setup: func(storeMock *groupStoreInterfaceMock) {
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{}, errors.New("db error")).
					Once()
			},
			expectErr: &serviceerror.InternalServerError,
		},
		{
			name:   "count error",
			id:     "grp-001",
			limit:  5,
			offset: 0,
			setup: func(storeMock *groupStoreInterfaceMock) {
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001"}, nil).
					Once()
				storeMock.On("GetGroupMemberCount", mock.Anything, "grp-001").
					Return(0, errors.New("count fail")).
					Once()
			},
			expectErr: &serviceerror.InternalServerError,
		},
		{
			name:   "list error",
			id:     "grp-001",
			limit:  5,
			offset: 0,
			setup: func(storeMock *groupStoreInterfaceMock) {
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001"}, nil).
					Once()
				storeMock.On("GetGroupMemberCount", mock.Anything, "grp-001").
					Return(1, nil).
					Once()
				storeMock.On("GetGroupMembers", mock.Anything, "grp-001", 5, 0).
					Return(nil, errors.New("list fail")).
					Once()
			},
			expectErr: &serviceerror.InternalServerError,
		},
		{
			name:   "access denied",
			id:     "grp-001",
			limit:  5,
			offset: 0,
			setup: func(storeMock *groupStoreInterfaceMock) {
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", OUID: testOUID1}, nil).Once()
			},
			authzSetup: func(t *testing.T) sysauthz.SystemAuthorizationServiceInterface {
				authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
				authzMock.On(
					"IsActionAllowed",
					mock.Anything,
					security.ActionReadGroup,
					&sysauthz.ActionContext{
						OUID:         testOUID1,
						ResourceType: security.ResourceTypeGroup,
						ResourceID:   "grp-001",
					},
				).Return(false, (*serviceerror.ServiceError)(nil))
				return authzMock
			},
			expectErr: &serviceerror.ErrorUnauthorized,
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			var storeMock *groupStoreInterfaceMock
			if tc.setup != nil {
				storeMock = newGroupStoreInterfaceMock(suite.T())
				tc.setup(storeMock)
			}

			var authzSvc sysauthz.SystemAuthorizationServiceInterface
			if tc.authzSetup != nil {
				authzSvc = tc.authzSetup(suite.T())
			} else {
				authzSvc = newAllowAllAuthz(suite.T())
			}
			var entitySvc entity.EntityServiceInterface
			if tc.entitySetup != nil {
				entitySvc = tc.entitySetup(suite.T())
			}
			service := &groupService{
				authzService:  authzSvc,
				groupStore:    storeMock,
				entityService: entitySvc,
			}

			response, err := service.GetGroupMembers(context.Background(), tc.id, tc.limit, tc.offset, false)

			if tc.expectErr != nil {
				suite.Require().Nil(response)
				suite.Require().NotNil(err)
				suite.Require().Equal(*tc.expectErr, *err)
			} else if tc.expectRes {
				suite.Require().Nil(err)
				suite.Require().NotNil(response)
				suite.Require().Equal(3, response.TotalResults)
				suite.Require().Equal(2, response.Count)
				suite.Require().Equal(1, response.StartIndex)
				suite.Require().Len(response.Members, 2)
				suite.Require().Equal(MemberTypeUser, response.Members[0].Type)
				suite.Require().Equal(MemberTypeGroup, response.Members[1].Type)
			} else {
				suite.Require().Nil(err)
			}

			if storeMock != nil {
				storeMock.AssertExpectations(suite.T())
			}
		})
	}
}

func (suite *GroupServiceTestSuite) TestGroupService_GetGroupMembers_WithDisplay() {
	storeMock := newGroupStoreInterfaceMock(suite.T())
	storeMock.On("GetGroup", mock.Anything, "grp-001").
		Return(GroupDAO{ID: "grp-001"}, nil).Once()
	storeMock.On("GetGroupMemberCount", mock.Anything, "grp-001").
		Return(2, nil).Once()
	storeMock.On("GetGroupMembers", mock.Anything, "grp-001", 5, 0).
		Return([]Member{
			{ID: "usr-001", Type: memberTypeEntity},
			{ID: "grp-002", Type: MemberTypeGroup},
		}, nil).Once()

	entitySvcMock := entitymock.NewEntityServiceInterfaceMock(suite.T())
	entitySvcMock.On("GetEntitiesByIDs", mock.Anything, []string{"usr-001"}).
		Return([]entity.Entity{
			{
				ID:         "usr-001",
				Category:   entity.EntityCategoryUser,
				Type:       "employee",
				Attributes: json.RawMessage(`{"name":"Alice"}`),
			},
		}, nil).Once()

	schemaMock := entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	schemaMock.On("GetDisplayAttributesByNames", mock.Anything, mock.Anything, mock.Anything).
		Return(map[string]string{"employee": "name"}, (*serviceerror.ServiceError)(nil)).Once()

	storeMock.On("GetGroupsByIDs", mock.Anything, []string{"grp-002"}).
		Return([]GroupBasicDAO{
			{ID: "grp-002", Name: "Engineering", OUID: "ou-1"},
		}, nil).Once()

	service := &groupService{
		authzService:      newAllowAllAuthz(suite.T()),
		groupStore:        storeMock,
		entityService:     entitySvcMock,
		entityTypeService: schemaMock,
	}

	resp, err := service.GetGroupMembers(context.Background(), "grp-001", 5, 0, true)
	suite.Require().Nil(err)
	suite.Require().NotNil(resp)
	suite.Require().Len(resp.Members, 2)
	suite.Require().Equal(MemberTypeUser, resp.Members[0].Type)
	suite.Require().Equal(MemberTypeGroup, resp.Members[1].Type)
	suite.Require().Equal("Alice", resp.Members[0].Display)
	suite.Require().Equal("Engineering", resp.Members[1].Display)
}

func (suite *GroupServiceTestSuite) TestGroupService_ValidateCreateGroupRequest() {
	service := &groupService{
		authzService: newAllowAllAuthz(suite.T())}

	testCases := []groupRequestValidationTestCase[CreateGroupRequest]{
		{
			name:    "missing fields",
			request: CreateGroupRequest{},
			wantErr: true,
		},
		{
			name:    "missing organization unit",
			request: CreateGroupRequest{Name: "name"},
			wantErr: true,
		},
		{
			name: "invalid member type",
			request: CreateGroupRequest{
				Name:    "name",
				OUID:    "ou",
				Members: []Member{{ID: "id", Type: "invalid"}},
			},
			wantErr: true,
		},
		{
			name: "missing member id",
			request: CreateGroupRequest{
				Name:    "name",
				OUID:    "ou",
				Members: []Member{{ID: "", Type: MemberTypeUser}},
			},
			wantErr: true,
		},
		{
			name: "valid request",
			request: CreateGroupRequest{
				Name:    "name",
				OUID:    "ou",
				Members: []Member{{ID: "usr-1", Type: MemberTypeUser}},
			},
			wantErr: false,
		},
	}

	runGroupRequestValidationTests(suite, testCases, service.validateCreateGroupRequest)
}

func (suite *GroupServiceTestSuite) TestGroupService_ValidateUpdateGroupRequest() {
	service := &groupService{
		authzService: newAllowAllAuthz(suite.T())}

	testCases := []groupRequestValidationTestCase[UpdateGroupRequest]{
		{
			name:    "missing fields",
			request: UpdateGroupRequest{},
			wantErr: true,
		},
		{
			name:    "missing organization unit",
			request: UpdateGroupRequest{Name: "name"},
			wantErr: true,
		},
		{
			name: "valid request",
			request: UpdateGroupRequest{
				Name: "name",
				OUID: "ou",
			},
			wantErr: false,
		},
	}

	runGroupRequestValidationTests(suite, testCases, service.validateUpdateGroupRequest)
}

func (suite *GroupServiceTestSuite) TestGroupService_ValidateOUHandlesInternalError() {
	t := suite.T()
	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	ouServiceMock.On("IsOrganizationUnitExists", mock.Anything, "ou-1").
		Return(false, &serviceerror.ServiceError{
			Code: "OU-5000",
			Type: serviceerror.ServerErrorType,
		}).
		Once()

	service := &groupService{
		authzService: newAllowAllAuthz(suite.T()),
		ouService:    ouServiceMock,
	}

	err := service.validateOU(context.Background(), "ou-1")

	require.NotNil(t, err)
	require.Equal(t, serviceerror.InternalServerError, *err)
}

func (suite *GroupServiceTestSuite) TestGroupService_ValidateAndProcessHandlePath() {
	t := suite.T()
	service := &groupService{
		authzService: newAllowAllAuthz(suite.T())}

	testCases := []struct {
		name        string
		handlePath  string
		expectError bool
	}{
		{
			name:        "empty string",
			handlePath:  "",
			expectError: true,
		},
		{
			name:        "whitespace only",
			handlePath:  "   ",
			expectError: true,
		},
		{
			name:        "only slashes",
			handlePath:  "///",
			expectError: true,
		},
		{
			name:        "double slash between handles",
			handlePath:  "root//child",
			expectError: true,
		},
		{
			name:        "single slash",
			handlePath:  "/",
			expectError: true,
		},
		{
			name:        "valid handles",
			handlePath:  "root/child",
			expectError: false,
		},
		{
			name:        "valid handles with surrounding whitespace and slashes",
			handlePath:  "  /root/child/  ",
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := service.validateAndProcessHandlePath(tc.handlePath)
			if tc.expectError {
				require.NotNil(t, err)
				require.Equal(t, ErrorInvalidRequestFormat, *err)
				return
			}

			require.Nil(t, err)
		})
	}
}

func (suite *GroupServiceTestSuite) TestGroupService_ValidateGroupIDs() {
	testCases := []struct {
		name      string
		setup     func(*groupStoreInterfaceMock)
		expectErr *serviceerror.ServiceError
	}{
		{
			name: "invalid ids",
			setup: func(storeMock *groupStoreInterfaceMock) {
				storeMock.On("ValidateGroupIDs", mock.Anything, []string{"grp-001"}).
					Return([]string{"grp-001"}, nil).
					Once()
			},
			expectErr: &ErrorInvalidGroupMemberID,
		},
		{
			name: "store error",
			setup: func(storeMock *groupStoreInterfaceMock) {
				storeMock.On("ValidateGroupIDs", mock.Anything, []string{"grp-001"}).
					Return(nil, errors.New("db error")).
					Once()
			},
			expectErr: &serviceerror.InternalServerError,
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			storeMock := newGroupStoreInterfaceMock(suite.T())
			service := &groupService{
				authzService: newAllowAllAuthz(suite.T()), groupStore: storeMock}
			tc.setup(storeMock)

			err := service.ValidateGroupIDs(context.Background(), []string{"grp-001"})

			suite.Require().NotNil(err)
			suite.Require().Equal(*tc.expectErr, *err)

			storeMock.AssertExpectations(suite.T())
		})
	}
}

type groupMemberTestCase struct {
	name       string
	groupID    string
	members    []Member
	setup      func(*groupStoreInterfaceMock, *entitymock.EntityServiceInterfaceMock)
	authzSetup func(*testing.T) sysauthz.SystemAuthorizationServiceInterface
	wantErr    *serviceerror.ServiceError
}

func newAccessDeniedUpdateGroupAuthz(t *testing.T) sysauthz.SystemAuthorizationServiceInterface {
	authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
	authzMock.On("GetAccessibleResources", mock.Anything, mock.Anything, security.ResourceTypeOU).
		Return(&sysauthz.AccessibleResources{AllAllowed: true}, (*serviceerror.ServiceError)(nil)).Maybe()
	authzMock.On(
		"IsActionAllowed",
		mock.Anything,
		security.ActionUpdateGroup,
		&sysauthz.ActionContext{
			OUID:         testOUID1,
			ResourceType: security.ResourceTypeGroup,
			ResourceID:   "grp-001",
		},
	).Return(false, (*serviceerror.ServiceError)(nil))
	return authzMock
}

func (suite *GroupServiceTestSuite) runGroupMemberTests(
	testCases []groupMemberTestCase,
	op func(*groupService, context.Context, string, []Member) (*Group, *serviceerror.ServiceError),
) {
	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			storeMock := newGroupStoreInterfaceMock(suite.T())
			entityServiceMock := entitymock.NewEntityServiceInterfaceMock(suite.T())

			if tc.setup != nil {
				tc.setup(storeMock, entityServiceMock)
			}

			var authzSvc sysauthz.SystemAuthorizationServiceInterface
			if tc.authzSetup != nil {
				authzSvc = tc.authzSetup(suite.T())
			} else {
				authzSvc = newAllowAllAuthz(suite.T())
			}
			service := &groupService{
				authzService:  authzSvc,
				groupStore:    storeMock,
				entityService: entityServiceMock,
				transactioner: &stubTransactioner{},
			}

			group, err := op(service, context.Background(), tc.groupID, tc.members)

			if tc.wantErr != nil {
				suite.Require().NotNil(err)
				suite.Require().Equal(*tc.wantErr, *err)
				suite.Require().Nil(group)
			} else {
				suite.Require().Nil(err)
				suite.Require().NotNil(group)
			}

			storeMock.AssertExpectations(suite.T())
			entityServiceMock.AssertExpectations(suite.T())
		})
	}
}

func (suite *GroupServiceTestSuite) TestGroupService_AddGroupMembers() {
	testCases := []groupMemberTestCase{
		{
			name:    "missing group id",
			groupID: "",
			members: []Member{{ID: "usr-001", Type: MemberTypeUser}},
			wantErr: &ErrorMissingGroupID,
		},
		{
			name:    "empty members list",
			groupID: "grp-001",
			members: []Member{},
			wantErr: &ErrorEmptyMembers,
		},
		{
			name:    "invalid member type",
			groupID: "grp-001",
			members: []Member{{ID: "usr-001", Type: "invalid"}},
			wantErr: &ErrorInvalidMemberType,
		},
		{
			name:    "empty member id",
			groupID: "grp-001",
			members: []Member{{ID: "", Type: MemberTypeUser}},
			wantErr: &ErrorInvalidRequestFormat,
		},
		{
			name:    "group not found",
			groupID: "grp-001",
			members: []Member{{ID: "usr-001", Type: MemberTypeUser}},
			setup: func(storeMock *groupStoreInterfaceMock, _ *entitymock.EntityServiceInterfaceMock) {
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{}, ErrGroupNotFound).Once()
			},
			wantErr: &ErrorGroupNotFound,
		},
		{
			name:    "invalid user member id",
			groupID: "grp-001",
			members: []Member{{ID: "usr-invalid", Type: MemberTypeUser}},
			setup: func(storeMock *groupStoreInterfaceMock, entityServiceMock *entitymock.EntityServiceInterfaceMock) {
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001"}, nil).Once()
				entityServiceMock.On("GetEntitiesByIDs", mock.Anything, []string{"usr-invalid"}).
					Return([]entity.Entity{}, nil).Once()
			},
			wantErr: &ErrorInvalidMemberID,
		},
		{
			name:    "store failure",
			groupID: "grp-001",
			members: []Member{{ID: "usr-001", Type: MemberTypeUser}},
			setup: func(storeMock *groupStoreInterfaceMock, entityServiceMock *entitymock.EntityServiceInterfaceMock) {
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", Name: "test"}, nil).Once()
				entityServiceMock.On("GetEntitiesByIDs", mock.Anything, []string{"usr-001"}).
					Return([]entity.Entity{{ID: "usr-001", Category: entity.EntityCategoryUser}}, nil).Once()
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", Name: "test"}, nil).Once()
				storeMock.On("AddGroupMembers", mock.Anything, "grp-001", mock.Anything).
					Return(errors.New("db error")).Once()
			},
			wantErr: &ErrorInternalServerError,
		},
		{
			name:    "success",
			groupID: "grp-001",
			members: []Member{{ID: "usr-001", Type: MemberTypeUser}},
			setup: func(storeMock *groupStoreInterfaceMock, entityServiceMock *entitymock.EntityServiceInterfaceMock) {
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", Name: "test"}, nil)
				entityServiceMock.On("GetEntitiesByIDs", mock.Anything, []string{"usr-001"}).
					Return([]entity.Entity{{ID: "usr-001", Category: entity.EntityCategoryUser}}, nil).Once()
				storeMock.On("AddGroupMembers", mock.Anything, "grp-001",
					[]Member{{ID: "usr-001", Type: memberTypeEntity}}).
					Return(nil).Once()
			},
			wantErr: nil,
		},
		{
			name:    "access denied",
			groupID: "grp-001",
			members: []Member{{ID: "usr-001", Type: MemberTypeUser}},
			setup: func(storeMock *groupStoreInterfaceMock, _ *entitymock.EntityServiceInterfaceMock) {
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", OUID: testOUID1}, nil).Once()
			},
			authzSetup: newAccessDeniedUpdateGroupAuthz,
			wantErr:    &serviceerror.ErrorUnauthorized,
		},
	}

	suite.runGroupMemberTests(testCases, func(svc *groupService, ctx context.Context, id string, members []Member) (
		*Group, *serviceerror.ServiceError,
	) {
		return svc.AddGroupMembers(ctx, id, members)
	})
}

func (suite *GroupServiceTestSuite) TestGroupService_RemoveGroupMembers() {
	testCases := []groupMemberTestCase{
		{
			name:    "missing group id",
			groupID: "",
			members: []Member{{ID: "usr-001", Type: MemberTypeUser}},
			wantErr: &ErrorMissingGroupID,
		},
		{
			name:    "empty members list",
			groupID: "grp-001",
			members: []Member{},
			wantErr: &ErrorEmptyMembers,
		},
		{
			name:    "invalid member type",
			groupID: "grp-001",
			members: []Member{{ID: "usr-001", Type: "invalid"}},
			wantErr: &ErrorInvalidMemberType,
		},
		{
			name:    "empty member id",
			groupID: "grp-001",
			members: []Member{{ID: "", Type: MemberTypeUser}},
			wantErr: &ErrorInvalidRequestFormat,
		},
		{
			name:    "group not found",
			groupID: "grp-001",
			members: []Member{{ID: "usr-001", Type: MemberTypeUser}},
			setup: func(storeMock *groupStoreInterfaceMock, _ *entitymock.EntityServiceInterfaceMock) {
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{}, ErrGroupNotFound).Once()
			},
			wantErr: &ErrorGroupNotFound,
		},
		{
			name:    "invalid group member id",
			groupID: "grp-001",
			members: []Member{{ID: "grp-invalid", Type: MemberTypeGroup}},
			setup: func(storeMock *groupStoreInterfaceMock, _ *entitymock.EntityServiceInterfaceMock) {
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001"}, nil).Once()
				storeMock.On("ValidateGroupIDs", mock.Anything, []string{"grp-invalid"}).
					Return([]string{"grp-invalid"}, nil).Once()
			},
			wantErr: &ErrorInvalidGroupMemberID,
		},
		{
			name:    "store failure",
			groupID: "grp-001",
			members: []Member{{ID: "usr-001", Type: MemberTypeUser}},
			setup: func(storeMock *groupStoreInterfaceMock, entityServiceMock *entitymock.EntityServiceInterfaceMock) {
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", Name: "test"}, nil).Once()
				entityServiceMock.On("GetEntitiesByIDs", mock.Anything, []string{"usr-001"}).
					Return([]entity.Entity{{ID: "usr-001", Category: entity.EntityCategoryUser}}, nil).Once()
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", Name: "test"}, nil).Once()
				storeMock.On("RemoveGroupMembers", mock.Anything, "grp-001", mock.Anything).
					Return(errors.New("db error")).Once()
			},
			wantErr: &ErrorInternalServerError,
		},
		{
			name:    "success",
			groupID: "grp-001",
			members: []Member{{ID: "usr-001", Type: MemberTypeUser}},
			setup: func(storeMock *groupStoreInterfaceMock, entityServiceMock *entitymock.EntityServiceInterfaceMock) {
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", Name: "test"}, nil)
				entityServiceMock.On("GetEntitiesByIDs", mock.Anything, []string{"usr-001"}).
					Return([]entity.Entity{{ID: "usr-001", Category: entity.EntityCategoryUser}}, nil).Once()
				storeMock.On("RemoveGroupMembers", mock.Anything, "grp-001",
					[]Member{{ID: "usr-001", Type: memberTypeEntity}}).
					Return(nil).Once()
			},
			wantErr: nil,
		},
		{
			name:    "access denied",
			groupID: "grp-001",
			members: []Member{{ID: "usr-001", Type: MemberTypeUser}},
			setup: func(storeMock *groupStoreInterfaceMock, _ *entitymock.EntityServiceInterfaceMock) {
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", OUID: testOUID1}, nil).Once()
			},
			authzSetup: newAccessDeniedUpdateGroupAuthz,
			wantErr:    &serviceerror.ErrorUnauthorized,
		},
	}

	suite.runGroupMemberTests(testCases, func(svc *groupService, ctx context.Context, id string, members []Member) (
		*Group, *serviceerror.ServiceError,
	) {
		return svc.RemoveGroupMembers(ctx, id, members)
	})
}

// resolveUserDisplay Tests

func TestResolveUserDisplay_WithDisplayAttr(t *testing.T) {
	e := &entity.Entity{
		ID:         "user-1",
		Type:       "employee",
		Attributes: json.RawMessage(`{"email":"alice@example.com"}`),
	}
	paths := map[string]string{"employee": "email"}
	require.Equal(t, "alice@example.com", utils.ResolveDisplay(e.ID, e.Type, e.Attributes, paths))
}

func TestResolveUserDisplay_FallbackToID(t *testing.T) {
	e := &entity.Entity{
		ID:         "user-1",
		Type:       "employee",
		Attributes: json.RawMessage(`{"name":"Alice"}`),
	}
	paths := map[string]string{"employee": "nonexistent"}
	require.Equal(t, "user-1", utils.ResolveDisplay(e.ID, e.Type, e.Attributes, paths))
}

func TestResolveUserDisplay_NilPaths(t *testing.T) {
	e := &entity.Entity{ID: "user-1", Type: "employee"}
	require.Equal(t, "user-1", utils.ResolveDisplay(e.ID, e.Type, e.Attributes, nil))
}

// resolveMembers Tests

func TestPopulateMemberDisplayNames_MixedMembers(t *testing.T) {
	entitySvcMock := entitymock.NewEntityServiceInterfaceMock(t)
	entitySvcMock.On("GetEntitiesByIDs", mock.Anything, []string{"user-1"}).
		Return([]entity.Entity{
			{
				ID:         "user-1",
				Category:   entity.EntityCategoryUser,
				Type:       "employee",
				Attributes: json.RawMessage(`{"name":"Alice"}`),
			},
		}, nil).Once()

	schemaMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	schemaMock.On("GetDisplayAttributesByNames", mock.Anything, mock.Anything, mock.Anything).
		Return(map[string]string{"employee": "name"}, (*serviceerror.ServiceError)(nil)).Once()

	storeMock := newGroupStoreInterfaceMock(t)
	storeMock.On("GetGroupsByIDs", mock.Anything, []string{"group-1"}).
		Return([]GroupBasicDAO{
			{ID: "group-1", Name: "Engineering", OUID: "ou-1"},
		}, nil).Once()

	service := &groupService{
		entityService:     entitySvcMock,
		entityTypeService: schemaMock,
		groupStore:        storeMock,
	}
	logger := log.GetLogger()

	members := []Member{
		{ID: "user-1", Type: memberTypeEntity},
		{ID: "group-1", Type: MemberTypeGroup},
	}

	resolved, svcErr := service.resolveMembers(context.Background(), members, true, logger)
	require.Nil(t, svcErr)
	require.Len(t, resolved, 2)
	require.Equal(t, "Alice", resolved[0].Display)
	require.Equal(t, "Engineering", resolved[1].Display)
}

func TestPopulateMemberDisplayNames_UserFallbackToID(t *testing.T) {
	entitySvcMock := entitymock.NewEntityServiceInterfaceMock(t)
	entitySvcMock.On("GetEntitiesByIDs", mock.Anything, []string{"user-1"}).
		Return([]entity.Entity{
			{ID: "user-1", Category: entity.EntityCategoryUser, Type: "employee", Attributes: json.RawMessage(`{}`)},
		}, nil).Once()

	schemaMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	schemaMock.On("GetDisplayAttributesByNames", mock.Anything, mock.Anything, mock.Anything).
		Return(map[string]string{"employee": "missing"}, (*serviceerror.ServiceError)(nil)).Once()

	service := &groupService{
		entityService:     entitySvcMock,
		entityTypeService: schemaMock,
	}
	logger := log.GetLogger()

	members := []Member{
		{ID: "user-1", Type: memberTypeEntity},
	}

	resolved, svcErr := service.resolveMembers(context.Background(), members, true, logger)
	require.Nil(t, svcErr)
	require.Len(t, resolved, 1)
	require.Equal(t, "user-1", resolved[0].Display)
}

func TestPopulateMemberDisplayNames_UserServiceError(t *testing.T) {
	entitySvcMock := entitymock.NewEntityServiceInterfaceMock(t)
	entitySvcMock.On("GetEntitiesByIDs", mock.Anything, []string{"user-1"}).
		Return([]entity.Entity(nil), errors.New("entity service error")).Once()

	service := &groupService{
		entityService: entitySvcMock,
	}
	logger := log.GetLogger()

	members := []Member{
		{ID: "user-1", Type: memberTypeEntity},
	}

	// Entity service failure is a hard error.
	result, svcErr := service.resolveMembers(context.Background(), members, true, logger)
	require.NotNil(t, svcErr)
	require.Nil(t, result)
}

func TestPopulateMemberDisplayNames_EmptyMembers(t *testing.T) {
	service := &groupService{}
	logger := log.GetLogger()

	var members []Member
	result, svcErr := service.resolveMembers(context.Background(), members, true, logger)
	require.Nil(t, svcErr)
	require.Empty(t, result)
}

func TestPopulateMemberDisplayNames_GroupFallbackToID(t *testing.T) {
	storeMock := newGroupStoreInterfaceMock(t)
	storeMock.On("GetGroupsByIDs", mock.Anything, []string{"group-1"}).
		Return([]GroupBasicDAO{
			{ID: "group-1", Name: "", OUID: "ou-1"},
		}, nil).Once()

	service := &groupService{
		groupStore: storeMock,
	}
	logger := log.GetLogger()

	members := []Member{
		{ID: "group-1", Type: MemberTypeGroup},
	}

	resolved, svcErr := service.resolveMembers(context.Background(), members, true, logger)
	require.Nil(t, svcErr)
	require.Len(t, resolved, 1)
	// Falls back to member ID when group name is empty.
	require.Equal(t, "group-1", resolved[0].Display)
}

func TestPopulateMemberDisplayNames_SchemaServiceError(t *testing.T) {
	entitySvcMock := entitymock.NewEntityServiceInterfaceMock(t)
	entitySvcMock.On("GetEntitiesByIDs", mock.Anything, []string{"user-1"}).
		Return([]entity.Entity{
			{
				ID:         "user-1",
				Category:   entity.EntityCategoryUser,
				Type:       "employee",
				Attributes: json.RawMessage(`{"name":"Alice"}`),
			},
		}, nil).Once()

	schemaMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	schemaMock.On("GetDisplayAttributesByNames", mock.Anything, mock.Anything, mock.Anything).
		Return(map[string]string(nil), &serviceerror.ServiceError{Code: "ERR"}).Once()

	service := &groupService{
		entityService:     entitySvcMock,
		entityTypeService: schemaMock,
	}
	logger := log.GetLogger()

	members := []Member{
		{ID: "user-1", Type: memberTypeEntity},
	}

	resolved, svcErr := service.resolveMembers(context.Background(), members, true, logger)
	require.Nil(t, svcErr)
	require.Len(t, resolved, 1)
	// Falls back to member ID when schema service fails to resolve display attributes.
	require.Equal(t, "user-1", resolved[0].Display)
}

func TestPopulateMemberDisplayNames_SchemaServiceError_WithGroupMember(t *testing.T) {
	groupStoreMock := newGroupStoreInterfaceMock(t)

	entitySvcMock := entitymock.NewEntityServiceInterfaceMock(t)
	entitySvcMock.On("GetEntitiesByIDs", mock.Anything, []string{"user-1"}).
		Return([]entity.Entity{
			{
				ID:         "user-1",
				Category:   entity.EntityCategoryUser,
				Type:       "employee",
				Attributes: json.RawMessage(`{"name":"Alice"}`),
			},
		}, nil).Once()

	schemaMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	schemaMock.On("GetDisplayAttributesByNames", mock.Anything, mock.Anything, mock.Anything).
		Return(map[string]string(nil), &serviceerror.ServiceError{Code: "ERR"}).Once()

	groupStoreMock.On("GetGroupsByIDs", mock.Anything, []string{"group-1"}).
		Return([]GroupBasicDAO{{ID: "group-1", Name: "Engineering", OUID: "ou-1"}}, nil).Once()

	service := &groupService{
		groupStore:        groupStoreMock,
		entityService:     entitySvcMock,
		entityTypeService: schemaMock,
	}
	logger := log.GetLogger()

	members := []Member{
		{ID: "user-1", Type: memberTypeEntity},
		{ID: "group-1", Type: MemberTypeGroup},
	}

	resolved, svcErr := service.resolveMembers(context.Background(), members, true, logger)
	require.Nil(t, svcErr)
	require.Len(t, resolved, 2)
	// User falls back to ID when schema service fails.
	require.Equal(t, "user-1", resolved[0].Display)
	// Group display still resolved via group name despite schema error (schema only affects users).
	require.Equal(t, "Engineering", resolved[1].Display)
}
