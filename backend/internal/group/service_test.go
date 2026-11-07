// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package group

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entitytype"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/projectscope"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
	"github.com/thunder-id/thunderid/tests/mocks/sharingmock"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
	"github.com/thunder-id/thunderid/tests/mocks/transactionmock"
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
		Return(true, (*tidcommon.ServiceError)(nil)).Maybe()
	mockAuthz.On("GetAccessibleResources", mock.Anything, mock.Anything, security.ResourceTypeOU).
		Return(&sysauthz.AccessibleResources{AllAllowed: true}, (*tidcommon.ServiceError)(nil)).Maybe()
	mockAuthz.On("CanGrantMembership", mock.Anything, mock.Anything, mock.Anything).
		Return((*tidcommon.ServiceError)(nil)).Maybe()
	return mockAuthz
}

// newAuthzError returns a mock that simulates an internal authorization error.
func newAuthzError(t *testing.T) sysauthz.SystemAuthorizationServiceInterface {
	mockAuthz := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
	mockAuthz.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
		Return(false, &tidcommon.InternalServerError).Maybe()
	mockAuthz.On("GetAccessibleResources", mock.Anything, mock.Anything, security.ResourceTypeOU).
		Return((*sysauthz.AccessibleResources)(nil), &tidcommon.InternalServerError).Maybe()
	mockAuthz.On("CanGrantMembership", mock.Anything, mock.Anything, mock.Anything).
		Return(&tidcommon.InternalServerError).Maybe()
	return mockAuthz
}

type GroupServiceTestSuite struct {
	suite.Suite
}

func (suite *GroupServiceTestSuite) SetupSuite() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{Enabled: false},
	}
	config.ResetServerRuntime()
	if err := config.InitializeServerRuntime("/tmp/test", testConfig); err != nil {
		suite.Fail("Failed to initialize runtime", err)
	}
}

func (suite *GroupServiceTestSuite) TearDownSuite() {
	config.ResetServerRuntime()
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
	validate func(T) *tidcommon.ServiceError,
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
		wantErr    *tidcommon.ServiceError
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
			wantErr: &tidcommon.InternalServerError,
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
			wantErr: &tidcommon.InternalServerError,
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
					(*tidcommon.ServiceError)(nil),
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
					(*tidcommon.ServiceError)(nil),
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
			wantErr:    &tidcommon.InternalServerError,
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
				bus:          discardingBus{},
				authzService: authzSvc,
				groupStore:   storeMock,
			}

			response, err := service.GetGroupList(context.Background(), tc.limit, tc.offset, nil, false)

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
		) *tidcommon.ServiceError
		authzSetup          func(*testing.T) sysauthz.SystemAuthorizationServiceInterface
		wantErr             *tidcommon.ServiceError
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
			) *tidcommon.ServiceError {
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
			) *tidcommon.ServiceError {
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
			) *tidcommon.ServiceError {
				expectedErr := &tidcommon.ServiceError{
					Code: "OU-5000",
					Type: tidcommon.ServerErrorType,
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
			) *tidcommon.ServiceError {
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
			) *tidcommon.ServiceError {
				storeMock.On("GetGroupsByOrganizationUnitCount", mock.Anything, "ou-123").
					Return(0, errors.New("count fail")).
					Once()

				ouMock.On("GetOrganizationUnitByPath", mock.Anything, "root/child").
					Return(oupkg.OrganizationUnit{ID: "ou-123"}, nil).
					Once()
				return nil
			},
			wantErr: &tidcommon.InternalServerError,
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
			) *tidcommon.ServiceError {
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
			wantErr: &tidcommon.InternalServerError,
		},
		{
			name:   "access denied",
			path:   "/org",
			limit:  5,
			offset: 0,
			setup: func(
				_ *groupStoreInterfaceMock,
				ouMock *oumock.OrganizationUnitServiceInterfaceMock,
			) *tidcommon.ServiceError {
				ouMock.On("GetOrganizationUnitByPath", mock.Anything, "/org").
					Return(oupkg.OrganizationUnit{ID: testOUID1}, (*tidcommon.ServiceError)(nil)).Once()
				return nil
			},
			authzSetup: func(t *testing.T) sysauthz.SystemAuthorizationServiceInterface {
				authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
				authzMock.On("IsActionAllowed", mock.Anything, security.ActionListGroups,
					&sysauthz.ActionContext{OUID: testOUID1, ResourceType: security.ResourceTypeGroup}).
					Return(false, (*tidcommon.ServiceError)(nil))
				return authzMock
			},
			wantErr: &tidcommon.ErrorUnauthorized,
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			storeMock := newGroupStoreInterfaceMock(suite.T())
			ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())

			var expectedErr *tidcommon.ServiceError
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
				bus:          discardingBus{},
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
		expectErr  *tidcommon.ServiceError
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

				args.store.On("GetGroupsByIDs", mock.Anything, []string{"grp-002"}).
					Return([]GroupBasicDAO{{ID: "grp-002", OUID: "ou-001"}}, nil).
					Once()

				args.entity.On("GetEntitiesByIDs", mock.Anything, []string{"usr-001"}).
					Return([]providers.Entity{
						{ID: "usr-001", Category: providers.EntityCategoryUser, OUID: "ou-001"}}, nil).
					Times(2)
			},
			expectRes: true,
		},
		{
			name: "user member outside group OU",
			request: CreateGroupRequest{
				Name:    "engineering",
				OUID:    "ou-001",
				Members: []Member{{ID: "usr-001", Type: MemberTypeUser}},
			},
			setup: func(args *setupArgs) {
				args.ou.On("IsOrganizationUnitExists", mock.Anything, "ou-001").
					Return(true, nil).
					Once()
				args.entity.On("GetEntitiesByIDs", mock.Anything, []string{"usr-001"}).
					Return([]providers.Entity{
						{ID: "usr-001", Category: providers.EntityCategoryUser, OUID: "ou-002"}}, nil).
					Once()
			},
			expectErr: &ErrorMemberOUMismatch,
		},
		{
			name: "app member outside group OU",
			request: CreateGroupRequest{
				Name:    "engineering",
				OUID:    "ou-001",
				Members: []Member{{ID: "app-001", Type: MemberTypeApp}},
			},
			setup: func(args *setupArgs) {
				args.ou.On("IsOrganizationUnitExists", mock.Anything, "ou-001").
					Return(true, nil).
					Once()
				args.entity.On("GetEntitiesByIDs", mock.Anything, []string{"app-001"}).
					Return([]providers.Entity{
						{ID: "app-001", Category: providers.EntityCategoryApp, OUID: "ou-002"}}, nil).
					Once()
			},
			expectErr: &ErrorMemberOUMismatch,
		},
		{
			name: "group member outside group OU",
			request: CreateGroupRequest{
				Name:    "engineering",
				OUID:    "ou-001",
				Members: []Member{{ID: "grp-002", Type: MemberTypeGroup}},
			},
			setup: func(args *setupArgs) {
				args.ou.On("IsOrganizationUnitExists", mock.Anything, "ou-001").
					Return(true, nil).
					Once()
				args.store.On("GetGroupsByIDs", mock.Anything, []string{"grp-002"}).
					Return([]GroupBasicDAO{{ID: "grp-002", OUID: "ou-002"}}, nil).
					Once()
			},
			expectErr: &ErrorMemberOUMismatch,
		},
		{
			name: "member OU check error",
			request: CreateGroupRequest{
				Name:    "engineering",
				OUID:    "ou-001",
				Members: []Member{{ID: "grp-002", Type: MemberTypeGroup}},
			},
			setup: func(args *setupArgs) {
				args.ou.On("IsOrganizationUnitExists", mock.Anything, "ou-001").
					Return(true, nil).
					Once()
				args.store.On("GetGroupsByIDs", mock.Anything, []string{"grp-002"}).
					Return(nil, errors.New("db failure")).
					Once()
			},
			expectErr: &tidcommon.InternalServerError,
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
					Return([]providers.Entity{}, nil).
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
			expectErr: &tidcommon.InternalServerError,
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
			expectErr: &tidcommon.InternalServerError,
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
						&tidcommon.ServiceError{Code: "OU-5000", Type: tidcommon.ServerErrorType}).
					Once()
			},
			expectErr: &tidcommon.InternalServerError,
		},
		{
			name: "access denied",
			request: CreateGroupRequest{
				Name: "developers",
				OUID: testOUID1,
			},
			setup: func(args *setupArgs) {
				args.ou.On("IsOrganizationUnitExists", mock.Anything, testOUID1).
					Return(true, (*tidcommon.ServiceError)(nil)).Once()
			},
			authzSetup: func(t *testing.T) sysauthz.SystemAuthorizationServiceInterface {
				authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
				authzMock.On("IsActionAllowed", mock.Anything, security.ActionCreateGroup,
					&sysauthz.ActionContext{OUID: testOUID1, ResourceType: security.ResourceTypeGroup}).
					Return(false, (*tidcommon.ServiceError)(nil))
				return authzMock
			},
			expectErr: &tidcommon.ErrorUnauthorized,
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
				bus:           discardingBus{},
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
		setup     func(*setupArgs) *tidcommon.ServiceError
		expectErr *tidcommon.ServiceError
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
			setup: func(args *setupArgs) *tidcommon.ServiceError {
				expected := &tidcommon.ServiceError{Code: "OU-5000", Type: tidcommon.ServerErrorType}
				args.ou.On("GetOrganizationUnitByPath", mock.Anything, "root").
					Return(oupkg.OrganizationUnit{}, expected).
					Once()
				return expected
			},
			expectErr: &tidcommon.ServiceError{Code: "OU-5000", Type: tidcommon.ServerErrorType},
		},
		{
			name:    "organization unit not found",
			path:    "root",
			request: CreateGroupByPathRequest{Name: "n"},
			setup: func(args *setupArgs) *tidcommon.ServiceError {
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
			var expectedOUError *tidcommon.ServiceError

			if tc.setup != nil {
				storeMock = newGroupStoreInterfaceMock(suite.T())
				ouServiceMock = oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())
				entityServiceMock = entitymock.NewEntityServiceInterfaceMock(suite.T())
				expectedOUError = tc.setup(&setupArgs{store: storeMock, ou: ouServiceMock, entity: entityServiceMock})
			}

			service := &groupService{
				bus:           discardingBus{},
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
		wantErr    *tidcommon.ServiceError
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
			wantErr: &tidcommon.InternalServerError,
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
				).Return(false, (*tidcommon.ServiceError)(nil))
				return authzMock
			},
			wantErr: &tidcommon.ErrorUnauthorized,
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
				bus:          discardingBus{},
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
			bus:          discardingBus{},
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
			bus:          discardingBus{},
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
			(map[string]string)(nil), &tidcommon.ServiceError{Code: "OU-5000"},
		).Once()

		service := &groupService{
			bus:          discardingBus{},
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
		bus:          discardingBus{},
		authzService: newAllowAllAuthz(suite.T()),
		groupStore:   storeMock,
		ouService:    ouServiceMock,
	}

	response, err := service.GetGroupList(
		context.Background(), 10, 0, nil, true)
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
		roles  *RoleAssignmentResolverMock
	}

	testCases := []struct {
		name           string
		groupID        string
		request        UpdateGroupRequest
		setup          func(*setupArgs)
		authzSetup     func(*testing.T) sysauthz.SystemAuthorizationServiceInterface
		expectErr      *tidcommon.ServiceError
		expectGroup    bool
		noRoleResolver bool
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
				args.store.On("IsGroupDeclarative", mock.Anything, "grp-001").Return(false, nil).Once()
				args.store.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", Name: "old", Description: "legacy",
						OUID: "ou-old"}, nil).
					Once()
				args.store.On("GetGroupMemberCount", mock.Anything, "grp-001").Return(0, nil).Once()
				args.store.On("GetDirectGroupParents", mock.Anything, []string{"grp-001"}).
					Return([]string{}, nil).
					Once()
				args.roles.On("HasGroupRoleAssignments", mock.Anything, "grp-001").
					Return(false, (*tidcommon.ServiceError)(nil)).
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
			name:    "move rejected for group with members",
			groupID: "grp-001",
			request: UpdateGroupRequest{Name: "old", OUID: "ou-new"},
			setup: func(args *setupArgs) {
				args.store.On("IsGroupDeclarative", mock.Anything, "grp-001").Return(false, nil).Once()
				args.store.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", Name: "old", OUID: "ou-old"}, nil).
					Once()
				args.ou.On("IsOrganizationUnitExists", mock.Anything, "ou-new").Return(true, nil).Once()
				args.store.On("GetGroupMemberCount", mock.Anything, "grp-001").Return(2, nil).Once()
				args.store.On("GetDirectGroupParents", mock.Anything, []string{"grp-001"}).
					Return([]string{}, nil).
					Once()
			},
			expectErr: &ErrorGroupOUChangeNotAllowed,
		},
		{
			name:    "move rejected for group inside another group",
			groupID: "grp-001",
			request: UpdateGroupRequest{Name: "old", OUID: "ou-new"},
			setup: func(args *setupArgs) {
				args.store.On("IsGroupDeclarative", mock.Anything, "grp-001").Return(false, nil).Once()
				args.store.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", Name: "old", OUID: "ou-old"}, nil).
					Once()
				args.ou.On("IsOrganizationUnitExists", mock.Anything, "ou-new").Return(true, nil).Once()
				args.store.On("GetGroupMemberCount", mock.Anything, "grp-001").Return(0, nil).Once()
				args.store.On("GetDirectGroupParents", mock.Anything, []string{"grp-001"}).
					Return([]string{"grp-parent"}, nil).
					Once()
			},
			expectErr: &ErrorGroupOUChangeNotAllowed,
		},
		{
			name:    "move member count error",
			groupID: "grp-001",
			request: UpdateGroupRequest{Name: "old", OUID: "ou-new"},
			setup: func(args *setupArgs) {
				args.store.On("IsGroupDeclarative", mock.Anything, "grp-001").Return(false, nil).Once()
				args.store.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", Name: "old", OUID: "ou-old"}, nil).
					Once()
				args.ou.On("IsOrganizationUnitExists", mock.Anything, "ou-new").Return(true, nil).Once()
				args.store.On("GetGroupMemberCount", mock.Anything, "grp-001").
					Return(0, errors.New("db error")).
					Once()
			},
			expectErr: &tidcommon.InternalServerError,
		},
		{
			name:    "move parent fetch error",
			groupID: "grp-001",
			request: UpdateGroupRequest{Name: "old", OUID: "ou-new"},
			setup: func(args *setupArgs) {
				args.store.On("IsGroupDeclarative", mock.Anything, "grp-001").Return(false, nil).Once()
				args.store.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", Name: "old", OUID: "ou-old"}, nil).
					Once()
				args.ou.On("IsOrganizationUnitExists", mock.Anything, "ou-new").Return(true, nil).Once()
				args.store.On("GetGroupMemberCount", mock.Anything, "grp-001").Return(0, nil).Once()
				args.store.On("GetDirectGroupParents", mock.Anything, []string{"grp-001"}).
					Return(nil, errors.New("db error")).
					Once()
			},
			expectErr: &tidcommon.InternalServerError,
		},
		{
			name:    "move rejected for group assigned roles",
			groupID: "grp-001",
			request: UpdateGroupRequest{Name: "old", OUID: "ou-new"},
			setup: func(args *setupArgs) {
				args.store.On("IsGroupDeclarative", mock.Anything, "grp-001").Return(false, nil).Once()
				args.store.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", Name: "old", OUID: "ou-old"}, nil).
					Once()
				args.ou.On("IsOrganizationUnitExists", mock.Anything, "ou-new").Return(true, nil).Once()
				args.store.On("GetGroupMemberCount", mock.Anything, "grp-001").Return(0, nil).Once()
				args.store.On("GetDirectGroupParents", mock.Anything, []string{"grp-001"}).
					Return([]string{}, nil).
					Once()
				args.roles.On("HasGroupRoleAssignments", mock.Anything, "grp-001").
					Return(true, (*tidcommon.ServiceError)(nil)).
					Once()
			},
			expectErr: &ErrorGroupOUChangeNotAllowed,
		},
		{
			name:    "move role assignment lookup error",
			groupID: "grp-001",
			request: UpdateGroupRequest{Name: "old", OUID: "ou-new"},
			setup: func(args *setupArgs) {
				args.store.On("IsGroupDeclarative", mock.Anything, "grp-001").Return(false, nil).Once()
				args.store.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", Name: "old", OUID: "ou-old"}, nil).
					Once()
				args.ou.On("IsOrganizationUnitExists", mock.Anything, "ou-new").Return(true, nil).Once()
				args.store.On("GetGroupMemberCount", mock.Anything, "grp-001").Return(0, nil).Once()
				args.store.On("GetDirectGroupParents", mock.Anything, []string{"grp-001"}).
					Return([]string{}, nil).
					Once()
				args.roles.On("HasGroupRoleAssignments", mock.Anything, "grp-001").
					Return(false, &tidcommon.InternalServerError).
					Once()
			},
			expectErr: &tidcommon.InternalServerError,
		},
		{
			name:    "move refused without a role assignment resolver",
			groupID: "grp-001",
			request: UpdateGroupRequest{Name: "old", OUID: "ou-new"},
			setup: func(args *setupArgs) {
				args.store.On("IsGroupDeclarative", mock.Anything, "grp-001").Return(false, nil).Once()
				args.store.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", Name: "old", OUID: "ou-old"}, nil).
					Once()
				args.ou.On("IsOrganizationUnitExists", mock.Anything, "ou-new").Return(true, nil).Once()
				args.store.On("GetGroupMemberCount", mock.Anything, "grp-001").Return(0, nil).Once()
				args.store.On("GetDirectGroupParents", mock.Anything, []string{"grp-001"}).
					Return([]string{}, nil).
					Once()
			},
			noRoleResolver: true,
			expectErr:      &tidcommon.InternalServerError,
		},
		{
			name:    "name conflict",
			groupID: "grp-001",
			request: UpdateGroupRequest{
				Name: "new-name",
				OUID: "ou-new",
			},
			setup: func(args *setupArgs) {
				args.store.On("IsGroupDeclarative", mock.Anything, "grp-001").Return(false, nil).Once()
				args.store.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", Name: "old", OUID: "ou-old"}, nil).
					Once()
				args.store.On("GetGroupMemberCount", mock.Anything, "grp-001").Return(0, nil).Once()
				args.store.On("GetDirectGroupParents", mock.Anything, []string{"grp-001"}).
					Return([]string{}, nil).
					Once()
				args.roles.On("HasGroupRoleAssignments", mock.Anything, "grp-001").
					Return(false, (*tidcommon.ServiceError)(nil)).
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
				args.store.On("IsGroupDeclarative", mock.Anything, "grp-001").Return(false, nil).Once()
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
				args.store.On("IsGroupDeclarative", mock.Anything, "grp-001").Return(false, nil).Once()
				args.store.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{}, errors.New("db error")).
					Once()
			},
			expectErr: &tidcommon.InternalServerError,
		},
		{
			name:    "validate organization unit error",
			groupID: "grp-001",
			request: UpdateGroupRequest{
				Name: "name",
				OUID: "ou-new",
			},
			setup: func(args *setupArgs) {
				args.store.On("IsGroupDeclarative", mock.Anything, "grp-001").Return(false, nil).Once()
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
				args.store.On("IsGroupDeclarative", mock.Anything, "grp-001").Return(false, nil).Once()
				args.store.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", Name: "old", OUID: "ou"}, nil).
					Once()
				args.store.On("CheckGroupNameConflictForUpdate", mock.Anything, "new", "ou", "grp-001").
					Return(errors.New("db error")).
					Once()
			},
			expectErr: &tidcommon.InternalServerError,
		},
		{
			name:    "update error",
			groupID: "grp-001",
			request: UpdateGroupRequest{
				Name: "new",
				OUID: "ou",
			},
			setup: func(args *setupArgs) {
				args.store.On("IsGroupDeclarative", mock.Anything, "grp-001").Return(false, nil).Once()
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
			expectErr: &tidcommon.InternalServerError,
		},
		{
			name:    "access denied on source OU",
			groupID: "grp-001",
			request: UpdateGroupRequest{
				Name: "new-name",
				OUID: testOUID1,
			},
			setup: func(args *setupArgs) {
				args.store.On("IsGroupDeclarative", mock.Anything, "grp-001").Return(false, nil).Once()
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
				).Return(false, (*tidcommon.ServiceError)(nil))
				return authzMock
			},
			expectErr: &tidcommon.ErrorUnauthorized,
		},
		{
			name:    "access denied on target OU",
			groupID: "grp-001",
			request: UpdateGroupRequest{
				Name: "new-name",
				OUID: testOUID2,
			},
			setup: func(args *setupArgs) {
				args.store.On("IsGroupDeclarative", mock.Anything, "grp-001").Return(false, nil).Once()
				args.store.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", OUID: testOUID1}, nil).Once()
				args.ou.On("IsOrganizationUnitExists", mock.Anything, testOUID2).
					Return(true, (*tidcommon.ServiceError)(nil)).Once()
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
				).Return(true, (*tidcommon.ServiceError)(nil))
				authzMock.On(
					"IsActionAllowed",
					mock.Anything,
					security.ActionUpdateGroup,
					&sysauthz.ActionContext{
						OUID:         testOUID2,
						ResourceType: security.ResourceTypeGroup,
						ResourceID:   "grp-001",
					},
				).Return(false, (*tidcommon.ServiceError)(nil))
				return authzMock
			},
			expectErr: &tidcommon.ErrorUnauthorized,
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			var storeMock *groupStoreInterfaceMock
			var ouServiceMock *oumock.OrganizationUnitServiceInterfaceMock
			var entityServiceMock *entitymock.EntityServiceInterfaceMock
			var rolesMock *RoleAssignmentResolverMock

			if tc.setup != nil {
				storeMock = newGroupStoreInterfaceMock(suite.T())
				ouServiceMock = oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())
				entityServiceMock = entitymock.NewEntityServiceInterfaceMock(suite.T())
				rolesMock = NewRoleAssignmentResolverMock(suite.T())
				tc.setup(&setupArgs{store: storeMock, ou: ouServiceMock, entity: entityServiceMock, roles: rolesMock})
			}

			var authzSvc sysauthz.SystemAuthorizationServiceInterface
			if tc.authzSetup != nil {
				authzSvc = tc.authzSetup(suite.T())
			} else {
				authzSvc = newAllowAllAuthz(suite.T())
			}
			service := &groupService{
				bus:           discardingBus{},
				authzService:  authzSvc,
				groupStore:    storeMock,
				ouService:     ouServiceMock,
				entityService: entityServiceMock,
				transactioner: &stubTransactioner{},
			}
			if rolesMock != nil && !tc.noRoleResolver {
				service.roleAssignments = rolesMock
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
		expectErr  *tidcommon.ServiceError
	}{
		{
			name: "success",
			id:   "grp-001",
			setup: func(storeMock *groupStoreInterfaceMock) {
				storeMock.On("IsGroupDeclarative", mock.Anything, "grp-001").Return(false, nil).Once()
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
				storeMock.On("IsGroupDeclarative", mock.Anything, "grp-001").Return(false, nil).Once()
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{}, errors.New("db error")).
					Once()
			},
			expectErr: &tidcommon.InternalServerError,
		},
		{
			name: "delete error",
			id:   "grp-001",
			setup: func(storeMock *groupStoreInterfaceMock) {
				storeMock.On("IsGroupDeclarative", mock.Anything, "grp-001").Return(false, nil).Once()
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001"}, nil).
					Once()
				storeMock.On("DeleteGroup", mock.Anything, "grp-001").
					Return(errors.New("delete fail")).
					Once()
			},
			expectErr: &tidcommon.InternalServerError,
		},
		{
			name: "group not found",
			id:   "grp-001",
			setup: func(storeMock *groupStoreInterfaceMock) {
				storeMock.On("IsGroupDeclarative", mock.Anything, "grp-001").Return(false, nil).Once()
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
				storeMock.On("IsGroupDeclarative", mock.Anything, "grp-001").Return(false, nil).Once()
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
				).Return(false, (*tidcommon.ServiceError)(nil))
				return authzMock
			},
			expectErr: &tidcommon.ErrorUnauthorized,
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
				bus:                discardingBus{},
				authzService:       authzSvc,
				groupStore:         storeMock,
				transactioner:      &stubTransactioner{},
				dependencyRegistry: noopDepRegistry{},
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
		expectErr   *tidcommon.ServiceError
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
					Return([]providers.Entity{
						{ID: "usr-001", Category: providers.EntityCategoryUser},
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
			expectErr: &tidcommon.InternalServerError,
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
			expectErr: &tidcommon.InternalServerError,
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
			expectErr: &tidcommon.InternalServerError,
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
				).Return(false, (*tidcommon.ServiceError)(nil))
				return authzMock
			},
			expectErr: &tidcommon.ErrorUnauthorized,
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
				bus:           discardingBus{},
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
		Return([]providers.Entity{
			{
				ID:         "usr-001",
				Category:   providers.EntityCategoryUser,
				Type:       "employee",
				Attributes: json.RawMessage(`{"name":"Alice"}`),
			},
		}, nil).Once()

	schemaMock := entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	schemaMock.On("GetDisplayAttributesByHandles", mock.Anything, mock.Anything, mock.Anything).
		Return(map[string]string{"employee": "name"}, (*tidcommon.ServiceError)(nil)).Once()

	storeMock.On("GetGroupsByIDs", mock.Anything, []string{"grp-002"}).
		Return([]GroupBasicDAO{
			{ID: "grp-002", Name: "Engineering", OUID: "ou-1"},
		}, nil).Once()

	service := &groupService{
		bus:               discardingBus{},
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
		bus:          discardingBus{},
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
		bus:          discardingBus{},
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
		Return(false, &tidcommon.ServiceError{
			Code: "OU-5000",
			Type: tidcommon.ServerErrorType,
		}).
		Once()

	service := &groupService{
		bus:          discardingBus{},
		authzService: newAllowAllAuthz(suite.T()),
		ouService:    ouServiceMock,
	}

	err := service.validateOU(context.Background(), "ou-1")

	require.NotNil(t, err)
	require.Equal(t, tidcommon.InternalServerError, *err)
}

func (suite *GroupServiceTestSuite) TestGroupService_ValidateAndProcessHandlePath() {
	t := suite.T()
	service := &groupService{
		bus:          discardingBus{},
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
		expectErr *tidcommon.ServiceError
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
			expectErr: &tidcommon.InternalServerError,
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			storeMock := newGroupStoreInterfaceMock(suite.T())
			service := &groupService{
				bus:          discardingBus{},
				authzService: newAllowAllAuthz(suite.T()), groupStore: storeMock}
			tc.setup(storeMock)

			err := service.ValidateGroupIDs(context.Background(), []string{"grp-001"})

			suite.Require().NotNil(err)
			suite.Require().Equal(*tc.expectErr, *err)

			storeMock.AssertExpectations(suite.T())
		})
	}
}

func (suite *GroupServiceTestSuite) TestGroupService_AddMembersToGroups() {
	defaultMembers := []Member{{ID: "usr-001", Type: MemberTypeUser}}
	testCases := []struct {
		name     string
		members  []Member
		groupIDs []string
		setup    func(*groupStoreInterfaceMock, *entitymock.EntityServiceInterfaceMock)
		txErr    error
		wantErr  *tidcommon.ServiceError
	}{
		{
			name:     "empty group IDs",
			members:  defaultMembers,
			groupIDs: []string{},
			wantErr:  nil,
		},
		{
			name:     "empty members",
			members:  []Member{},
			groupIDs: []string{"grp-001"},
			wantErr:  nil,
		},
		{
			name:     "validate store error",
			members:  defaultMembers,
			groupIDs: []string{"grp-001"},
			setup: func(storeMock *groupStoreInterfaceMock, _ *entitymock.EntityServiceInterfaceMock) {
				storeMock.On("ValidateGroupIDs", mock.Anything, []string{"grp-001"}).
					Return(nil, errors.New("db error")).Once()
			},
			wantErr: &tidcommon.InternalServerError,
		},
		{
			name:     "invalid group IDs",
			members:  defaultMembers,
			groupIDs: []string{"grp-bad"},
			setup: func(storeMock *groupStoreInterfaceMock, _ *entitymock.EntityServiceInterfaceMock) {
				storeMock.On("ValidateGroupIDs", mock.Anything, []string{"grp-bad"}).
					Return([]string{"grp-bad"}, nil).Once()
			},
			wantErr: &ErrorInvalidGroupMemberID,
		},
		{
			name:     "add group members fails",
			members:  defaultMembers,
			groupIDs: []string{"grp-001"},
			setup: func(storeMock *groupStoreInterfaceMock, _ *entitymock.EntityServiceInterfaceMock) {
				storeMock.On("ValidateGroupIDs", mock.Anything, []string{"grp-001"}).
					Return(nil, nil).Once()
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{}, ErrGroupNotFound).Once()
			},
			wantErr: &ErrorGroupNotFound,
		},
		{
			name:     "transactioner error",
			members:  defaultMembers,
			groupIDs: []string{"grp-001"},
			setup: func(storeMock *groupStoreInterfaceMock, _ *entitymock.EntityServiceInterfaceMock) {
				storeMock.On("ValidateGroupIDs", mock.Anything, []string{"grp-001"}).
					Return(nil, nil).Once()
			},
			txErr:   errors.New("tx failed"),
			wantErr: &tidcommon.InternalServerError,
		},
		{
			name:     "success single group",
			members:  defaultMembers,
			groupIDs: []string{"grp-001"},
			setup: func(storeMock *groupStoreInterfaceMock, entityMock *entitymock.EntityServiceInterfaceMock) {
				storeMock.On("ValidateGroupIDs", mock.Anything, []string{"grp-001"}).
					Return(nil, nil).Once()
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", OUID: testOUID1, Name: "g1"}, nil)
				entityMock.On("GetEntitiesByIDs", mock.Anything, []string{"usr-001"}).
					Return([]providers.Entity{
						{ID: "usr-001", Category: providers.EntityCategoryUser, OUID: testOUID1}}, nil)
				storeMock.On("AddGroupMembers", mock.Anything, "grp-001", mock.Anything).
					Return(nil).Once()
			},
			wantErr: nil,
		},
		{
			name:     "success multiple groups",
			members:  defaultMembers,
			groupIDs: []string{"grp-001", "grp-002"},
			setup: func(storeMock *groupStoreInterfaceMock, entityMock *entitymock.EntityServiceInterfaceMock) {
				storeMock.On("ValidateGroupIDs", mock.Anything, []string{"grp-001", "grp-002"}).
					Return(nil, nil).Once()
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", OUID: testOUID1, Name: "g1"}, nil)
				storeMock.On("GetGroup", mock.Anything, "grp-002").
					Return(GroupDAO{ID: "grp-002", OUID: testOUID1, Name: "g2"}, nil)
				entityMock.On("GetEntitiesByIDs", mock.Anything, []string{"usr-001"}).
					Return([]providers.Entity{
						{ID: "usr-001", Category: providers.EntityCategoryUser, OUID: testOUID1}}, nil)
				storeMock.On("AddGroupMembers", mock.Anything, "grp-001", mock.Anything).Return(nil).Once()
				storeMock.On("AddGroupMembers", mock.Anything, "grp-002", mock.Anything).Return(nil).Once()
			},
			wantErr: nil,
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			storeMock := newGroupStoreInterfaceMock(suite.T())
			entityMock := entitymock.NewEntityServiceInterfaceMock(suite.T())
			if tc.setup != nil {
				tc.setup(storeMock, entityMock)
			}

			var txner providers.Transactioner
			if tc.txErr != nil {
				txMock := transactionmock.NewTransactionerMock(suite.T())
				txMock.On("Transact", mock.Anything, mock.Anything).Return(tc.txErr)
				txner = txMock
			} else {
				txner = &stubTransactioner{}
			}
			svc := &groupService{
				bus:           discardingBus{},
				authzService:  newAllowAllAuthz(suite.T()),
				groupStore:    storeMock,
				entityService: entityMock,
				transactioner: txner,
			}

			err := svc.AddMembersToGroups(context.Background(), tc.members, tc.groupIDs)

			if tc.wantErr != nil {
				suite.Require().NotNil(err)
				suite.Require().Equal(*tc.wantErr, *err)
			} else {
				suite.Require().Nil(err)
			}

			storeMock.AssertExpectations(suite.T())
			entityMock.AssertExpectations(suite.T())
		})
	}
}

type groupMemberTestCase struct {
	name       string
	groupID    string
	members    []Member
	setup      func(*groupStoreInterfaceMock, *entitymock.EntityServiceInterfaceMock)
	authzSetup func(*testing.T) sysauthz.SystemAuthorizationServiceInterface
	wantErr    *tidcommon.ServiceError
}

func newAccessDeniedUpdateGroupAuthz(t *testing.T) sysauthz.SystemAuthorizationServiceInterface {
	authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
	authzMock.On("GetAccessibleResources", mock.Anything, mock.Anything, security.ResourceTypeOU).
		Return(&sysauthz.AccessibleResources{AllAllowed: true}, (*tidcommon.ServiceError)(nil)).Maybe()
	authzMock.On(
		"IsActionAllowed",
		mock.Anything,
		security.ActionUpdateGroup,
		&sysauthz.ActionContext{
			OUID:         testOUID1,
			ResourceType: security.ResourceTypeGroup,
			ResourceID:   "grp-001",
		},
	).Return(false, (*tidcommon.ServiceError)(nil))
	return authzMock
}

func (suite *GroupServiceTestSuite) runGroupMemberTests(
	testCases []groupMemberTestCase,
	op func(*groupService, context.Context, string, []Member) (*Group, *tidcommon.ServiceError),
) {
	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			storeMock := newGroupStoreInterfaceMock(suite.T())
			entityServiceMock := entitymock.NewEntityServiceInterfaceMock(suite.T())

			if tc.setup != nil {
				tc.setup(storeMock, entityServiceMock)
			}
			storeMock.On("IsGroupDeclarative", mock.Anything, mock.Anything).Return(false, nil).Maybe()

			var authzSvc sysauthz.SystemAuthorizationServiceInterface
			if tc.authzSetup != nil {
				authzSvc = tc.authzSetup(suite.T())
			} else {
				authzSvc = newAllowAllAuthz(suite.T())
			}
			service := &groupService{
				bus:           discardingBus{},
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
					Return([]providers.Entity{}, nil).Once()
			},
			wantErr: &ErrorInvalidMemberID,
		},
		{
			name:    "store failure",
			groupID: "grp-001",
			members: []Member{{ID: "usr-001", Type: MemberTypeUser}},
			setup: func(storeMock *groupStoreInterfaceMock, entityServiceMock *entitymock.EntityServiceInterfaceMock) {
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", Name: "test", OUID: testOUID1}, nil).Twice()
				entityServiceMock.On("GetEntitiesByIDs", mock.Anything, []string{"usr-001"}).
					Return([]providers.Entity{
						{ID: "usr-001", Category: providers.EntityCategoryUser, OUID: testOUID1}}, nil).Once()
				storeMock.On("AddGroupMembers", mock.Anything, "grp-001", mock.Anything).
					Return(errors.New("db error")).Once()
			},
			wantErr: &tidcommon.InternalServerError,
		},
		{
			name:    "success",
			groupID: "grp-001",
			members: []Member{{ID: "usr-001", Type: MemberTypeUser}},
			setup: func(storeMock *groupStoreInterfaceMock, entityServiceMock *entitymock.EntityServiceInterfaceMock) {
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", Name: "test", OUID: testOUID1}, nil)
				entityServiceMock.On("GetEntitiesByIDs", mock.Anything, []string{"usr-001"}).
					Return([]providers.Entity{
						{ID: "usr-001", Category: providers.EntityCategoryUser, OUID: testOUID1}}, nil).Once()
				storeMock.On("AddGroupMembers", mock.Anything, "grp-001",
					[]Member{{ID: "usr-001", Type: memberTypeEntity}}).
					Return(nil).Once()
			},
			wantErr: nil,
		},
		{
			name:    "user member outside group OU",
			groupID: "grp-001",
			members: []Member{{ID: "usr-001", Type: MemberTypeUser}},
			setup: func(storeMock *groupStoreInterfaceMock, entityServiceMock *entitymock.EntityServiceInterfaceMock) {
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", Name: "test", OUID: testOUID1}, nil).Once()
				entityServiceMock.On("GetEntitiesByIDs", mock.Anything, []string{"usr-001"}).
					Return([]providers.Entity{
						{ID: "usr-001", Category: providers.EntityCategoryUser, OUID: testOUID2}}, nil).Once()
			},
			wantErr: &ErrorMemberOUMismatch,
		},
		{
			name:    "agent member outside group OU",
			groupID: "grp-001",
			members: []Member{{ID: "agt-001", Type: MemberTypeAgent}},
			setup: func(storeMock *groupStoreInterfaceMock, entityServiceMock *entitymock.EntityServiceInterfaceMock) {
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", Name: "test", OUID: testOUID1}, nil).Once()
				entityServiceMock.On("GetEntitiesByIDs", mock.Anything, []string{"agt-001"}).
					Return([]providers.Entity{
						{ID: "agt-001", Category: providers.EntityCategoryAgent, OUID: testOUID2}}, nil).Once()
			},
			wantErr: &ErrorMemberOUMismatch,
		},
		{
			name:    "nested group outside group OU",
			groupID: "grp-001",
			members: []Member{{ID: "grp-002", Type: MemberTypeGroup}},
			setup: func(storeMock *groupStoreInterfaceMock, _ *entitymock.EntityServiceInterfaceMock) {
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", Name: "test", OUID: testOUID1}, nil).Once()
				storeMock.On("GetGroupsByIDs", mock.Anything, []string{"grp-002"}).
					Return([]GroupBasicDAO{{ID: "grp-002", OUID: testOUID2}}, nil).Once()
			},
			wantErr: &ErrorMemberOUMismatch,
		},
		{
			name:    "group moved to another OU before the members are added",
			groupID: "grp-001",
			members: []Member{{ID: "usr-001", Type: MemberTypeUser}},
			setup: func(storeMock *groupStoreInterfaceMock, entityServiceMock *entitymock.EntityServiceInterfaceMock) {
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", Name: "test", OUID: testOUID1}, nil).Once()
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", Name: "test", OUID: testOUID2}, nil).Once()
				entityServiceMock.On("GetEntitiesByIDs", mock.Anything, []string{"usr-001"}).
					Return([]providers.Entity{
						{ID: "usr-001", Category: providers.EntityCategoryUser, OUID: testOUID1}}, nil).Twice()
			},
			wantErr: &ErrorMemberOUMismatch,
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
			wantErr:    &tidcommon.ErrorUnauthorized,
		},
	}

	suite.runGroupMemberTests(testCases, func(svc *groupService, ctx context.Context, id string, members []Member) (
		*Group, *tidcommon.ServiceError,
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
					Return([]providers.Entity{{ID: "usr-001", Category: providers.EntityCategoryUser}}, nil).Once()
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", Name: "test"}, nil).Once()
				storeMock.On("RemoveGroupMembers", mock.Anything, "grp-001", mock.Anything).
					Return(errors.New("db error")).Once()
			},
			wantErr: &tidcommon.InternalServerError,
		},
		{
			name:    "success",
			groupID: "grp-001",
			members: []Member{{ID: "usr-001", Type: MemberTypeUser}},
			setup: func(storeMock *groupStoreInterfaceMock, entityServiceMock *entitymock.EntityServiceInterfaceMock) {
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", Name: "test"}, nil)
				entityServiceMock.On("GetEntitiesByIDs", mock.Anything, []string{"usr-001"}).
					Return([]providers.Entity{{ID: "usr-001", Category: providers.EntityCategoryUser}}, nil).Once()
				storeMock.On("RemoveGroupMembers", mock.Anything, "grp-001",
					[]Member{{ID: "usr-001", Type: memberTypeEntity}}).
					Return(nil).Once()
			},
			wantErr: nil,
		},
		{
			name:    "success for member outside group OU",
			groupID: "grp-001",
			members: []Member{{ID: "usr-001", Type: MemberTypeUser}},
			setup: func(storeMock *groupStoreInterfaceMock, entityServiceMock *entitymock.EntityServiceInterfaceMock) {
				storeMock.On("GetGroup", mock.Anything, "grp-001").
					Return(GroupDAO{ID: "grp-001", Name: "test", OUID: testOUID1}, nil)
				entityServiceMock.On("GetEntitiesByIDs", mock.Anything, []string{"usr-001"}).
					Return([]providers.Entity{
						{ID: "usr-001", Category: providers.EntityCategoryUser, OUID: testOUID2}}, nil).Once()
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
			wantErr:    &tidcommon.ErrorUnauthorized,
		},
	}

	suite.runGroupMemberTests(testCases, func(svc *groupService, ctx context.Context, id string, members []Member) (
		*Group, *tidcommon.ServiceError,
	) {
		return svc.RemoveGroupMembers(ctx, id, members)
	})
}

// resolveUserDisplay Tests

func TestResolveUserDisplay_WithDisplayAttr(t *testing.T) {
	e := &providers.Entity{
		ID:         "user-1",
		Type:       "employee",
		Attributes: json.RawMessage(`{"email":"alice@example.com"}`),
	}
	paths := map[string]string{"employee": "email"}
	require.Equal(t, "alice@example.com", utils.ResolveDisplay(e.ID, e.Type, e.Attributes, paths))
}

func TestResolveUserDisplay_FallbackToID(t *testing.T) {
	e := &providers.Entity{
		ID:         "user-1",
		Type:       "employee",
		Attributes: json.RawMessage(`{"name":"Alice"}`),
	}
	paths := map[string]string{"employee": "nonexistent"}
	require.Equal(t, "user-1", utils.ResolveDisplay(e.ID, e.Type, e.Attributes, paths))
}

func TestResolveUserDisplay_NilPaths(t *testing.T) {
	e := &providers.Entity{ID: "user-1", Type: "employee"}
	require.Equal(t, "user-1", utils.ResolveDisplay(e.ID, e.Type, e.Attributes, nil))
}

// resolveMembers Tests

func TestPopulateMemberDisplayNames_MixedMembers(t *testing.T) {
	entitySvcMock := entitymock.NewEntityServiceInterfaceMock(t)
	entitySvcMock.On("GetEntitiesByIDs", mock.Anything, []string{"user-1"}).
		Return([]providers.Entity{
			{
				ID:         "user-1",
				Category:   providers.EntityCategoryUser,
				Type:       "employee",
				Attributes: json.RawMessage(`{"name":"Alice"}`),
			},
		}, nil).Once()

	schemaMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	schemaMock.On("GetDisplayAttributesByHandles", mock.Anything, mock.Anything, mock.Anything).
		Return(map[string]string{"employee": "name"}, (*tidcommon.ServiceError)(nil)).Once()

	storeMock := newGroupStoreInterfaceMock(t)
	storeMock.On("GetGroupsByIDs", mock.Anything, []string{"group-1"}).
		Return([]GroupBasicDAO{
			{ID: "group-1", Name: "Engineering", OUID: "ou-1"},
		}, nil).Once()

	service := &groupService{
		bus:               discardingBus{},
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

func TestPopulateMemberDisplayNames_UserAgentAndAppMembers(t *testing.T) {
	entitySvcMock := entitymock.NewEntityServiceInterfaceMock(t)
	entitySvcMock.On("GetEntitiesByIDs", mock.Anything, []string{"user-1", "agent-1", "agent-2", "app-1"}).
		Return([]providers.Entity{
			{ID: "user-1", Category: providers.EntityCategoryUser, Type: "shared",
				Attributes: json.RawMessage(`{"username":"alice","name":"wrong"}`)},
			{ID: "agent-1", Category: providers.EntityCategoryAgent, Type: "shared",
				Attributes: json.RawMessage(`{"username":"wrong","name":"Ledger Agent"}`)},
			{ID: "agent-2", Category: providers.EntityCategoryAgent, Type: "shared",
				Attributes: json.RawMessage(`{"model":"x"}`)},
			{ID: "app-1", Category: providers.EntityCategoryApp, SystemAttributes: json.RawMessage(`{"name":"Shop"}`)},
		}, nil).Once()

	schemaMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	schemaMock.On("GetDisplayAttributesByHandles", mock.Anything, entitytype.TypeCategoryUser, []string{"shared"}).
		Return(map[string]string{"shared": "username"}, (*tidcommon.ServiceError)(nil)).Once()
	schemaMock.On("GetDisplayAttributesByHandles", mock.Anything, entitytype.TypeCategoryAgent, []string{"shared"}).
		Return(map[string]string{"shared": "name"}, (*tidcommon.ServiceError)(nil)).Once()

	service := &groupService{entityService: entitySvcMock, entityTypeService: schemaMock}

	members := []Member{
		{ID: "user-1", Type: memberTypeEntity},
		{ID: "agent-1", Type: memberTypeEntity},
		{ID: "agent-2", Type: memberTypeEntity},
		{ID: "app-1", Type: memberTypeEntity},
	}

	resolved, svcErr := service.resolveMembers(context.Background(), members, true, log.GetLogger())
	require.Nil(t, svcErr)
	require.Len(t, resolved, 4)
	require.Equal(t, "alice", resolved[0].Display)
	require.Equal(t, "Ledger Agent", resolved[1].Display)
	require.Equal(t, "agent-2", resolved[2].Display)
	require.Equal(t, "Shop", resolved[3].Display)
}

func TestPopulateMemberDisplayNames_UserFallbackToID(t *testing.T) {
	entitySvcMock := entitymock.NewEntityServiceInterfaceMock(t)
	entitySvcMock.On("GetEntitiesByIDs", mock.Anything, []string{"user-1"}).
		Return([]providers.Entity{
			{ID: "user-1", Category: providers.EntityCategoryUser, Type: "employee", Attributes: json.RawMessage(`{}`)},
		}, nil).Once()

	schemaMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	schemaMock.On("GetDisplayAttributesByHandles", mock.Anything, mock.Anything, mock.Anything).
		Return(map[string]string{"employee": "missing"}, (*tidcommon.ServiceError)(nil)).Once()

	service := &groupService{
		bus:               discardingBus{},
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
		Return([]providers.Entity(nil), errors.New("entity service error")).Once()

	service := &groupService{
		bus:           discardingBus{},
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
		bus:        discardingBus{},
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
		Return([]providers.Entity{
			{
				ID:         "user-1",
				Category:   providers.EntityCategoryUser,
				Type:       "employee",
				Attributes: json.RawMessage(`{"name":"Alice"}`),
			},
		}, nil).Once()

	schemaMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	schemaMock.On("GetDisplayAttributesByHandles", mock.Anything, mock.Anything, mock.Anything).
		Return(map[string]string(nil), &tidcommon.ServiceError{Code: "ERR"}).Once()

	service := &groupService{
		bus:               discardingBus{},
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
		Return([]providers.Entity{
			{
				ID:         "user-1",
				Category:   providers.EntityCategoryUser,
				Type:       "employee",
				Attributes: json.RawMessage(`{"name":"Alice"}`),
			},
		}, nil).Once()

	schemaMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	schemaMock.On("GetDisplayAttributesByHandles", mock.Anything, mock.Anything, mock.Anything).
		Return(map[string]string(nil), &tidcommon.ServiceError{Code: "ERR"}).Once()

	groupStoreMock.On("GetGroupsByIDs", mock.Anything, []string{"group-1"}).
		Return([]GroupBasicDAO{{ID: "group-1", Name: "Engineering", OUID: "ou-1"}}, nil).Once()

	service := &groupService{
		bus:               discardingBus{},
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

func TestGetGroup_DatabaseFailure(t *testing.T) {
	groupStoreMock := newGroupStoreInterfaceMock(t)

	// Fix the type mismatch here by returning an empty GroupBasicDAO{} instead of Group{}
	groupStoreMock.On("GetGroup", mock.Anything, "group1").
		Return(GroupDAO{}, errors.New("mock query timeout")).Once()

	service := &groupService{
		bus:        discardingBus{},
		groupStore: groupStoreMock,
	}

	result, err := service.GetGroup(context.Background(), "group1", false)

	require.Nil(t, result)
	require.NotNil(t, err)
	require.Equal(t, tidcommon.InternalServerError.Code, err.Code)
}

// Test Case to hit Line 960 (Entity service fetch failure)
func TestUpdateGroupMembers_EntityFetchFailure(t *testing.T) {
	entitySvcMock := entitymock.NewEntityServiceInterfaceMock(t)

	// Force the entity service tool to crash when called
	entitySvcMock.On("GetEntitiesByIDs", mock.Anything, []string{"user-1"}).
		Return([]providers.Entity{}, errors.New("mock network timeout")).Once()

	service := &groupService{
		bus:           discardingBus{},
		entityService: entitySvcMock,
	}
	logger := log.GetLogger()

	// This calls the internal member resolver method to trigger your modified Line 960
	resolved, svcErr := service.resolveMembers(context.Background(), []Member{
		{ID: "user-1", Type: memberTypeEntity},
	}, true, logger)

	require.Nil(t, resolved)
	require.NotNil(t, svcErr)
	require.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)
}

func newScopedListAuthz(t *testing.T) sysauthz.SystemAuthorizationServiceInterface {
	authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
	authzMock.On("GetAccessibleResources", mock.Anything, security.ActionListGroups, security.ResourceTypeOU).
		Return(&sysauthz.AccessibleResources{AllAllowed: false, IDs: []string{testOUID1}},
			(*tidcommon.ServiceError)(nil))
	return authzMock
}

func TestListGroupsByOUIDs_CountError(t *testing.T) {
	storeMock := newGroupStoreInterfaceMock(t)
	storeMock.On("GetGroupListCountByOUIDs", mock.Anything, []string{testOUID1}).
		Return(0, errors.New("count fail")).Once()

	service := &groupService{
		bus:          discardingBus{},
		authzService: newScopedListAuthz(t),
		groupStore:   storeMock,
	}

	response, err := service.GetGroupList(context.Background(), 5, 0, nil, false)
	require.Nil(t, response)
	require.NotNil(t, err)
	require.Equal(t, tidcommon.InternalServerError.Code, err.Code)
}

func TestListGroupsByOUIDs_ListError(t *testing.T) {
	storeMock := newGroupStoreInterfaceMock(t)
	storeMock.On("GetGroupListCountByOUIDs", mock.Anything, []string{testOUID1}).
		Return(2, nil).Once()
	storeMock.On("GetGroupListByOUIDs", mock.Anything, []string{testOUID1}, 5, 0).
		Return(nil, errors.New("list fail")).Once()

	service := &groupService{
		bus:          discardingBus{},
		authzService: newScopedListAuthz(t),
		groupStore:   storeMock,
	}

	response, err := service.GetGroupList(context.Background(), 5, 0, nil, false)
	require.Nil(t, response)
	require.NotNil(t, err)
	require.Equal(t, tidcommon.InternalServerError.Code, err.Code)
}

func TestGetGroupList_OUFilter(t *testing.T) {
	cases := map[string]struct {
		authz func(t *testing.T) sysauthz.SystemAuthorizationServiceInterface
		ouID  string
	}{
		"system caller":                  {authz: newAllowAllAuthz, ouID: testOUID2},
		"caller scoped to the filter OU": {authz: newScopedListAuthz, ouID: testOUID1},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			storeMock := newGroupStoreInterfaceMock(t)
			storeMock.On("GetGroupListCountByOUIDs", mock.Anything, []string{tc.ouID}).Return(3, nil).Once()
			storeMock.On("GetGroupListByOUIDs", mock.Anything, []string{tc.ouID}, 2, 0).
				Return([]GroupBasicDAO{
					{ID: "grp-1", Name: "Admins", OUID: tc.ouID},
					{ID: "grp-2", Name: "Editors", OUID: tc.ouID},
				}, nil).Once()

			service := &groupService{
				bus:          discardingBus{},
				authzService: tc.authz(t),
				groupStore:   storeMock,
			}

			response, err := service.GetGroupList(
				context.Background(), 2, 0, map[string]interface{}{"ouId": tc.ouID}, false)
			require.Nil(t, err)
			require.Equal(t, 3, response.TotalResults)
			require.Equal(t, 2, response.Count)
			require.Equal(t, "grp-1", response.Groups[0].ID)
			require.Equal(t, "grp-2", response.Groups[1].ID)
			filterQuery := "&filter=" + url.QueryEscape(`ouId eq "`+tc.ouID+`"`)
			require.Equal(t, []utils.Link{
				{Href: "/groups?offset=2&limit=2" + filterQuery, Rel: "next"},
				{Href: "/groups?offset=2&limit=2" + filterQuery, Rel: "last"},
			}, response.Links)
		})
	}
}

func TestGetGroupList_OUFilter_NoAccessToOU(t *testing.T) {
	storeMock := newGroupStoreInterfaceMock(t)

	service := &groupService{
		bus:          discardingBus{},
		authzService: newScopedListAuthz(t),
		groupStore:   storeMock,
	}

	response, err := service.GetGroupList(
		context.Background(), 5, 0, map[string]interface{}{"ouId": testOUID2}, false)
	require.Nil(t, err)
	require.Equal(t, 0, response.TotalResults)
	require.Empty(t, response.Groups)
	storeMock.AssertNotCalled(t, "GetGroupListCountByOUIDs", mock.Anything, mock.Anything)
}

func TestGetGroupList_InvalidFilter(t *testing.T) {
	for name, filters := range map[string]map[string]interface{}{
		"another attribute":            {"name": "Admins"},
		"an ouId with another":         {"ouId": testOUID1, "name": "Admins"},
		"an ouId that is not a string": {"ouId": int64(5)},
		"an empty ouId":                {"ouId": ""},
	} {
		t.Run(name, func(t *testing.T) {
			service := &groupService{
				bus:          discardingBus{},
				authzService: sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t),
				groupStore:   newGroupStoreInterfaceMock(t),
			}

			response, err := service.GetGroupList(context.Background(), 5, 0, filters, false)
			require.Nil(t, response)
			require.NotNil(t, err)
			require.Equal(t, ErrorInvalidFilter.Code, err.Code)
		})
	}
}

func TestUpdateGroup_IsDeclarativeError(t *testing.T) {
	storeMock := newGroupStoreInterfaceMock(t)
	storeMock.On("IsGroupDeclarative", mock.Anything, "grp-001").
		Return(false, errors.New("declarative check fail")).Once()

	service := &groupService{
		bus:           discardingBus{},
		authzService:  newAllowAllAuthz(t),
		groupStore:    storeMock,
		transactioner: &stubTransactioner{},
	}

	group, err := service.UpdateGroup(context.Background(), "grp-001",
		UpdateGroupRequest{Name: "name", OUID: "ou"})
	require.Nil(t, group)
	require.NotNil(t, err)
	require.Equal(t, tidcommon.InternalServerError.Code, err.Code)
}

func TestDeleteGroup_IsDeclarativeError(t *testing.T) {
	storeMock := newGroupStoreInterfaceMock(t)
	storeMock.On("IsGroupDeclarative", mock.Anything, "grp-001").
		Return(false, errors.New("declarative check fail")).Once()

	service := &groupService{
		bus:           discardingBus{},
		authzService:  newAllowAllAuthz(t),
		groupStore:    storeMock,
		transactioner: &stubTransactioner{},
	}

	err := service.DeleteGroup(context.Background(), "grp-001")
	require.NotNil(t, err)
	require.Equal(t, tidcommon.InternalServerError.Code, err.Code)
}

func TestResolveMembers_GetGroupsByIDsError(t *testing.T) {
	storeMock := newGroupStoreInterfaceMock(t)
	storeMock.On("GetGroupsByIDs", mock.Anything, []string{"group-1"}).
		Return(nil, errors.New("groups fetch fail")).Once()

	service := &groupService{
		bus:        discardingBus{},
		groupStore: storeMock,
	}
	logger := log.GetLogger()

	members := []Member{{ID: "group-1", Type: MemberTypeGroup}}
	resolved, svcErr := service.resolveMembers(context.Background(), members, true, logger)
	require.Nil(t, svcErr)
	require.Len(t, resolved, 1)
	require.Equal(t, "group-1", resolved[0].Display)
}

func TestResolveMembers_OrphanedEntityMember(t *testing.T) {
	entitySvcMock := entitymock.NewEntityServiceInterfaceMock(t)
	entitySvcMock.On("GetEntitiesByIDs", mock.Anything, []string{"user-1"}).
		Return([]providers.Entity{}, nil).Once()

	service := &groupService{
		bus:           discardingBus{},
		entityService: entitySvcMock,
	}
	logger := log.GetLogger()

	members := []Member{{ID: "user-1", Type: memberTypeEntity}}
	resolved, svcErr := service.resolveMembers(context.Background(), members, false, logger)
	require.Nil(t, svcErr)
	require.Empty(t, resolved)
}

func TestModifyGroupMembers_GetGroupError(t *testing.T) {
	storeMock := newGroupStoreInterfaceMock(t)
	storeMock.On("GetGroup", mock.Anything, "grp-001").
		Return(GroupDAO{}, errors.New("db error")).Once()

	service := &groupService{
		bus:           discardingBus{},
		authzService:  newAllowAllAuthz(t),
		groupStore:    storeMock,
		transactioner: &stubTransactioner{},
	}

	group, err := service.AddGroupMembers(context.Background(), "grp-001",
		[]Member{{ID: "usr-001", Type: MemberTypeUser}})
	require.Nil(t, group)
	require.NotNil(t, err)
	require.Equal(t, tidcommon.InternalServerError.Code, err.Code)
}

func TestModifyGroupMembers_InnerGroupNotFound(t *testing.T) {
	storeMock := newGroupStoreInterfaceMock(t)
	entitySvcMock := entitymock.NewEntityServiceInterfaceMock(t)

	storeMock.On("GetGroup", mock.Anything, "grp-001").
		Return(GroupDAO{ID: "grp-001"}, nil).Once()
	entitySvcMock.On("GetEntitiesByIDs", mock.Anything, []string{"usr-001"}).
		Return([]providers.Entity{{ID: "usr-001", Category: providers.EntityCategoryUser}}, nil).Once()
	storeMock.On("GetGroup", mock.Anything, "grp-001").
		Return(GroupDAO{}, ErrGroupNotFound).Once()

	service := &groupService{
		bus:           discardingBus{},
		authzService:  newAllowAllAuthz(t),
		groupStore:    storeMock,
		entityService: entitySvcMock,
		transactioner: &stubTransactioner{},
	}

	group, err := service.AddGroupMembers(context.Background(), "grp-001",
		[]Member{{ID: "usr-001", Type: MemberTypeUser}})
	require.Nil(t, group)
	require.NotNil(t, err)
	require.Equal(t, ErrorGroupNotFound.Code, err.Code)
}

func TestValidateEntityMembers_GetEntitiesError(t *testing.T) {
	entitySvcMock := entitymock.NewEntityServiceInterfaceMock(t)
	entitySvcMock.On("GetEntitiesByIDs", mock.Anything, []string{"usr-001"}).
		Return(nil, errors.New("entity fetch fail")).Once()

	service := &groupService{
		bus:           discardingBus{},
		authzService:  newAllowAllAuthz(t),
		entityService: entitySvcMock,
	}

	err := service.validateEntityMembers(context.Background(),
		[]Member{{ID: "usr-001", Type: MemberTypeUser}}, "ou-1", true)
	require.NotNil(t, err)
	require.Equal(t, tidcommon.InternalServerError.Code, err.Code)
}

func TestValidateEntityMembers_TypeMismatch(t *testing.T) {
	entitySvcMock := entitymock.NewEntityServiceInterfaceMock(t)
	entitySvcMock.On("GetEntitiesByIDs", mock.Anything, []string{"usr-001"}).
		Return([]providers.Entity{{ID: "usr-001", Category: providers.EntityCategoryApp}}, nil).Once()

	service := &groupService{
		bus:           discardingBus{},
		authzService:  newAllowAllAuthz(t),
		entityService: entitySvcMock,
	}

	err := service.validateEntityMembers(context.Background(),
		[]Member{{ID: "usr-001", Type: MemberTypeUser}}, "ou-1", true)
	require.NotNil(t, err)
	require.Equal(t, ErrorInvalidMemberID.Code, err.Code)
}

const (
	testUserSharingType sharing.ResourceType = "user"
	testAppSharingType  sharing.ResourceType = "application"
)

// newSharingAwareService returns a service that checks sharing through the given mock.
func newSharingAwareService(
	entitySvc *entitymock.EntityServiceInterfaceMock, sharingSvc *sharingmock.SharingServiceInterfaceMock,
) *groupService {
	registerSharedCategories(entitySvc)
	return &groupService{
		bus:            discardingBus{},
		entityService:  entitySvc,
		sharingService: sharingSvc,
	}
}

// registerSharedCategories answers the shared resource type lookup as the server does: users and
// applications are shareable, agents are not.
func registerSharedCategories(entitySvc *entitymock.EntityServiceInterfaceMock) {
	entitySvc.On("GetSharedResourceType", providers.EntityCategoryUser).Return(testUserSharingType, true).Maybe()
	entitySvc.On("GetSharedResourceType", providers.EntityCategoryApp).Return(testAppSharingType, true).Maybe()
	entitySvc.On("GetSharedResourceType", providers.EntityCategoryAgent).
		Return(sharing.ResourceType(""), false).Maybe()
}

func (suite *GroupServiceTestSuite) TestFindMembersNotAvailableInOU_SharedEntitiesAreInside() {
	t := suite.T()
	entitySvcMock := entitymock.NewEntityServiceInterfaceMock(t)
	sharingSvcMock := sharingmock.NewSharingServiceInterfaceMock(t)
	entitySvcMock.On("GetEntitiesByIDs", mock.Anything, []string{"usr-001", "app-001", "usr-002"}).
		Return([]providers.Entity{
			{ID: "usr-001", Category: providers.EntityCategoryUser, OUID: "ou-2"},
			{ID: "app-001", Category: providers.EntityCategoryApp, OUID: "ou-2"},
			{ID: "usr-002", Category: providers.EntityCategoryUser, OUID: "ou-1"},
		}, nil).Once()
	sharingSvcMock.On("IsVisible", mock.Anything, testUserSharingType, "usr-001", "ou-1").
		Return(true, (*tidcommon.ServiceError)(nil)).Once()
	sharingSvcMock.On("IsVisible", mock.Anything, testAppSharingType, "app-001", "ou-1").
		Return(true, (*tidcommon.ServiceError)(nil)).Once()

	outside, err := newSharingAwareService(entitySvcMock, sharingSvcMock).findMembersNotAvailableInOU(
		context.Background(), "ou-1", []string{"usr-001", "app-001", "usr-002"}, nil)

	require.NoError(t, err)
	require.Empty(t, outside)
}

func (suite *GroupServiceTestSuite) TestFindMembersNotAvailableInOU_NotShared() {
	t := suite.T()
	entitySvcMock := entitymock.NewEntityServiceInterfaceMock(t)
	sharingSvcMock := sharingmock.NewSharingServiceInterfaceMock(t)
	entitySvcMock.On("GetEntitiesByIDs", mock.Anything, []string{"usr-001", "agt-001"}).
		Return([]providers.Entity{
			{ID: "usr-001", Category: providers.EntityCategoryUser, OUID: "ou-2"},
			{ID: "agt-001", Category: providers.EntityCategoryAgent, OUID: "ou-2"},
		}, nil).Once()
	sharingSvcMock.On("IsVisible", mock.Anything, testUserSharingType, "usr-001", "ou-1").
		Return(false, (*tidcommon.ServiceError)(nil)).Once()

	outside, err := newSharingAwareService(entitySvcMock, sharingSvcMock).findMembersNotAvailableInOU(
		context.Background(), "ou-1", []string{"usr-001", "agt-001"}, nil)

	require.NoError(t, err)
	require.ElementsMatch(t, []string{"usr-001", "agt-001"}, outside)
}

func (suite *GroupServiceTestSuite) TestFindMembersNotAvailableInOU_AgentIsNeverShared() {
	t := suite.T()
	entitySvcMock := entitymock.NewEntityServiceInterfaceMock(t)
	sharingSvcMock := sharingmock.NewSharingServiceInterfaceMock(t)
	entitySvcMock.On("GetEntitiesByIDs", mock.Anything, []string{"agt-001"}).
		Return([]providers.Entity{{ID: "agt-001", Category: providers.EntityCategoryAgent, OUID: "ou-2"}}, nil).
		Once()

	outside, err := newSharingAwareService(entitySvcMock, sharingSvcMock).findMembersNotAvailableInOU(
		context.Background(), "ou-1", []string{"agt-001"}, nil)

	require.NoError(t, err)
	require.Equal(t, []string{"agt-001"}, outside)
	sharingSvcMock.AssertNotCalled(t, "IsVisible", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (suite *GroupServiceTestSuite) TestFindMembersNotAvailableInOU_SharingLookupError() {
	t := suite.T()
	entitySvcMock := entitymock.NewEntityServiceInterfaceMock(t)
	sharingSvcMock := sharingmock.NewSharingServiceInterfaceMock(t)
	entitySvcMock.On("GetEntitiesByIDs", mock.Anything, []string{"usr-001"}).
		Return([]providers.Entity{{ID: "usr-001", Category: providers.EntityCategoryUser, OUID: "ou-2"}}, nil).
		Once()
	sharingSvcMock.On("IsVisible", mock.Anything, testUserSharingType, "usr-001", "ou-1").
		Return(false, &tidcommon.InternalServerError).Once()

	_, err := newSharingAwareService(entitySvcMock, sharingSvcMock).findMembersNotAvailableInOU(
		context.Background(), "ou-1", []string{"usr-001"}, nil)

	require.Error(t, err)
}

func (suite *GroupServiceTestSuite) TestValidateEntityMembers_MemberOutsideGroupOU() {
	cases := []struct {
		name    string
		member  providers.Entity
		memType MemberType
		rt      sharing.ResourceType
		shared  bool
		wantErr *tidcommon.ServiceError
	}{
		{name: "shared user passes", member: providers.Entity{ID: "usr-001",
			Category: providers.EntityCategoryUser, OUID: "ou-2"},
			memType: MemberTypeUser, rt: testUserSharingType, shared: true},
		{name: "unshared user is refused", member: providers.Entity{ID: "usr-001",
			Category: providers.EntityCategoryUser, OUID: "ou-2"},
			memType: MemberTypeUser, rt: testUserSharingType, wantErr: &ErrorMemberOUMismatch},
		{name: "unshared app is refused", member: providers.Entity{ID: "app-001",
			Category: providers.EntityCategoryApp, OUID: "ou-2"},
			memType: MemberTypeApp, rt: testAppSharingType, wantErr: &ErrorMemberOUMismatch},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			t := suite.T()
			entitySvcMock := entitymock.NewEntityServiceInterfaceMock(t)
			sharingSvcMock := sharingmock.NewSharingServiceInterfaceMock(t)
			entitySvcMock.On("GetEntitiesByIDs", mock.Anything, []string{tc.member.ID}).
				Return([]providers.Entity{tc.member}, nil).Once()
			sharingSvcMock.On("IsVisible", mock.Anything, tc.rt, tc.member.ID, "ou-1").
				Return(tc.shared, (*tidcommon.ServiceError)(nil)).Once()

			err := newSharingAwareService(entitySvcMock, sharingSvcMock).validateEntityMembers(
				context.Background(), []Member{{ID: tc.member.ID, Type: tc.memType}}, "ou-1", true)

			if tc.wantErr == nil {
				require.Nil(t, err)
			} else {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErr.Code, err.Code)
			}
		})
	}
}

// Without the organization unit check, as when members are removed, a member owned elsewhere is
// accepted without asking the sharing service.
func (suite *GroupServiceTestSuite) TestValidateEntityMembers_WithoutOUCheck() {
	t := suite.T()
	entitySvcMock := entitymock.NewEntityServiceInterfaceMock(t)
	sharingSvcMock := sharingmock.NewSharingServiceInterfaceMock(t)
	entitySvcMock.On("GetEntitiesByIDs", mock.Anything, []string{"usr-001"}).
		Return([]providers.Entity{{ID: "usr-001", Category: providers.EntityCategoryUser, OUID: "ou-2"}}, nil).
		Once()

	err := newSharingAwareService(entitySvcMock, sharingSvcMock).validateEntityMembers(
		context.Background(), []Member{{ID: "usr-001", Type: MemberTypeUser}}, "ou-1", false)

	require.Nil(t, err)
	sharingSvcMock.AssertNotCalled(t, "IsVisible", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (suite *GroupServiceTestSuite) TestValidateGroupMembers() {
	suite.Run("missing group", func() {
		storeMock := newGroupStoreInterfaceMock(suite.T())
		storeMock.On("GetGroupsByIDs", mock.Anything, []string{"grp-002"}).Return([]GroupBasicDAO{}, nil).Once()

		err := (&groupService{groupStore: storeMock}).validateGroupMembers(
			context.Background(), []string{"grp-002"}, "ou-1", true)

		suite.Require().NotNil(err)
		suite.Equal(ErrorInvalidGroupMemberID.Code, err.Code)
	})
	suite.Run("group owned by the organization unit", func() {
		storeMock := newGroupStoreInterfaceMock(suite.T())
		storeMock.On("GetGroupsByIDs", mock.Anything, []string{"grp-002"}).
			Return([]GroupBasicDAO{{ID: "grp-002", OUID: "ou-1"}}, nil).Once()

		err := (&groupService{groupStore: storeMock}).validateGroupMembers(
			context.Background(), []string{"grp-002"}, "ou-1", true)

		suite.Nil(err)
	})
	suite.Run("lookup error", func() {
		storeMock := newGroupStoreInterfaceMock(suite.T())
		storeMock.On("GetGroupsByIDs", mock.Anything, []string{"grp-002"}).
			Return(nil, errors.New("db error")).Once()

		err := (&groupService{groupStore: storeMock}).validateGroupMembers(
			context.Background(), []string{"grp-002"}, "ou-1", true)

		suite.Require().NotNil(err)
		suite.Equal(tidcommon.InternalServerError.Code, err.Code)
	})
	suite.Run("existence only without the organization unit check", func() {
		storeMock := newGroupStoreInterfaceMock(suite.T())
		storeMock.On("ValidateGroupIDs", mock.Anything, []string{"grp-002"}).Return([]string{}, nil).Once()

		err := (&groupService{groupStore: storeMock}).validateGroupMembers(
			context.Background(), []string{"grp-002"}, "ou-1", false)

		suite.Nil(err)
	})
}

func (suite *GroupServiceTestSuite) TestValidateEntityMembers_SharingLookupError() {
	t := suite.T()
	entitySvcMock := entitymock.NewEntityServiceInterfaceMock(t)
	sharingSvcMock := sharingmock.NewSharingServiceInterfaceMock(t)
	entitySvcMock.On("GetEntitiesByIDs", mock.Anything, []string{"usr-001"}).
		Return([]providers.Entity{{ID: "usr-001", Category: providers.EntityCategoryUser, OUID: "ou-2"}}, nil).
		Once()
	sharingSvcMock.On("IsVisible", mock.Anything, testUserSharingType, "usr-001", "ou-1").
		Return(false, &tidcommon.InternalServerError).Once()

	err := newSharingAwareService(entitySvcMock, sharingSvcMock).validateEntityMembers(
		context.Background(), []Member{{ID: "usr-001", Type: MemberTypeUser}}, "ou-1", true)

	require.NotNil(t, err)
	require.Equal(t, tidcommon.InternalServerError.Code, err.Code)
}

func TestGetGroupsByIDs_StoreError(t *testing.T) {
	storeMock := newGroupStoreInterfaceMock(t)
	storeMock.On("GetGroupsByIDs", mock.Anything, []string{"grp-001"}).
		Return(nil, errors.New("store fail")).Once()

	service := &groupService{
		bus:        discardingBus{},
		groupStore: storeMock,
	}

	result, err := service.GetGroupsByIDs(context.Background(), []string{"grp-001"})
	require.Nil(t, result)
	require.NotNil(t, err)
	require.Equal(t, tidcommon.InternalServerError.Code, err.Code)
}

// TestGetGroupsByNames_DedupesInputAndGroupsAmbiguousMatches confirms duplicate input names are
// deduped before the store call, and that more than one group sharing a name are both returned under
// that name rather than one silently overwriting the other.
func TestGetGroupsByNames_DedupesInputAndGroupsAmbiguousMatches(t *testing.T) {
	storeMock := newGroupStoreInterfaceMock(t)
	storeMock.On("GetGroupsByNames", mock.Anything, []string{"engineering"}).
		Return([]GroupBasicDAO{
			{ID: "grp-1", Name: "engineering", OUID: "ou-1"},
			{ID: "grp-2", Name: "engineering", OUID: "ou-2"},
		}, nil).Once()

	service := &groupService{groupStore: storeMock}

	result, err := service.GetGroupsByNames(context.Background(), []string{"engineering", "engineering"})
	require.Nil(t, err)
	require.Len(t, result["engineering"], 2)
}

func TestGetGroupsByNames_EmptyInputReturnsEmptyMap(t *testing.T) {
	service := &groupService{groupStore: newGroupStoreInterfaceMock(t)}

	result, err := service.GetGroupsByNames(context.Background(), nil)
	require.Nil(t, err)
	require.Empty(t, result)
}

func TestGetGroupsByNames_StoreError(t *testing.T) {
	storeMock := newGroupStoreInterfaceMock(t)
	storeMock.On("GetGroupsByNames", mock.Anything, []string{"engineering"}).
		Return(nil, errors.New("store fail")).Once()

	service := &groupService{groupStore: storeMock}

	result, err := service.GetGroupsByNames(context.Background(), []string{"engineering"})
	require.Nil(t, result)
	require.NotNil(t, err)
	require.Equal(t, tidcommon.InternalServerError.Code, err.Code)
}

func TestPopulateGroupOUHandles_ServiceError(t *testing.T) {
	ouServiceMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	ouServiceMock.On("GetOrganizationUnitHandlesByIDs", mock.Anything, []string{testOUID1}).
		Return((map[string]string)(nil), &tidcommon.ServiceError{Code: "OU-5000"}).Once()

	service := &groupService{
		bus:       discardingBus{},
		ouService: ouServiceMock,
	}
	logger := log.GetLogger()

	groups := []GroupBasic{{ID: "g1", OUID: testOUID1}}
	service.populateGroupOUHandles(context.Background(), groups, logger)
	require.Empty(t, groups[0].OUHandle)
}

func TestCheckGroupAccess_AuthzError(t *testing.T) {
	service := &groupService{
		bus:          discardingBus{},
		authzService: newAuthzError(t),
	}

	err := service.checkGroupAccess(context.Background(), security.ActionReadGroup, testOUID1, "grp-001")
	require.NotNil(t, err)
	require.Equal(t, tidcommon.InternalServerError.Code, err.Code)
}

// --- Cascade / dependency provider ---

func (suite *GroupServiceTestSuite) TestGetResourceDependencies_ReturnsEmpty() {
	service := &groupService{}
	result, err := service.GetResourceDependencies(
		context.Background(), resourcedependency.ResourceTypeUser, "user-1")
	suite.NoError(err)
	suite.Empty(result)
}

func (suite *GroupServiceTestSuite) TestCascadeDeleteDependencies_User_DeletesMemberships() {
	storeMock := newGroupStoreInterfaceMock(suite.T())
	storeMock.On("DeleteMembershipsByMember", mock.Anything, string(memberTypeEntity), "user-1").
		Return(int64(2), nil)
	service := &groupService{groupStore: storeMock}

	deleted, err := service.CascadeDeleteDependencies(
		context.Background(), resourcedependency.ResourceTypeUser, "user-1")

	suite.NoError(err)
	suite.Equal(2, deleted)
}

func (suite *GroupServiceTestSuite) TestCascadeDeleteDependencies_UnknownType_NoOp() {
	storeMock := newGroupStoreInterfaceMock(suite.T())
	service := &groupService{groupStore: storeMock}

	deleted, err := service.CascadeDeleteDependencies(context.Background(), "theme", "theme-1")

	suite.NoError(err)
	suite.Equal(0, deleted)
	storeMock.AssertNotCalled(suite.T(), "DeleteMembershipsByMember",
		mock.Anything, mock.Anything, mock.Anything)
}

func (suite *GroupServiceTestSuite) TestCascadeDeleteDependencies_StoreError() {
	storeMock := newGroupStoreInterfaceMock(suite.T())
	storeMock.On("DeleteMembershipsByMember", mock.Anything, string(memberTypeEntity), "user-1").
		Return(int64(0), errors.New("db error"))
	service := &groupService{groupStore: storeMock}

	deleted, err := service.CascadeDeleteDependencies(
		context.Background(), resourcedependency.ResourceTypeUser, "user-1")

	suite.Error(err)
	suite.Equal(0, deleted)
}

// noopDepRegistry is a no-op resourcedependency.Registry for tests that don't exercise cascade.
type noopDepRegistry struct{ cascadeErr error }

func (noopDepRegistry) RegisterProvider(resourcedependency.Provider) {}

func (noopDepRegistry) GetDependencies(
	context.Context, string, string) (*resourcedependency.DependenciesResponse, error) {
	return &resourcedependency.DependenciesResponse{}, nil
}

func (r noopDepRegistry) CascadeDelete(context.Context, string, string) (int, error) {
	return 0, r.cascadeErr
}

func (noopDepRegistry) ValidateReferenceUpdate(
	context.Context, string, string) *tidcommon.ServiceError {
	return nil
}

func (suite *GroupServiceTestSuite) TestCascadeDeleteDependencies_Group_DeletesMemberships() {
	storeMock := newGroupStoreInterfaceMock(suite.T())
	storeMock.On("DeleteMembershipsByMember", mock.Anything, string(MemberTypeGroup), "group-1").
		Return(int64(1), nil)
	service := &groupService{groupStore: storeMock}

	deleted, err := service.CascadeDeleteDependencies(
		context.Background(), resourcedependency.ResourceTypeGroup, "group-1")

	suite.NoError(err)
	suite.Equal(1, deleted)
}

func (suite *GroupServiceTestSuite) TestDeleteGroup_AbortedWhenCascadeFails() {
	storeMock := newGroupStoreInterfaceMock(suite.T())
	storeMock.On("IsGroupDeclarative", mock.Anything, "grp-001").Return(false, nil).Once()
	storeMock.On("GetGroup", mock.Anything, "grp-001").
		Return(GroupDAO{ID: "grp-001", OUID: testOUID1}, nil).Once()
	service := &groupService{
		bus:                discardingBus{},
		authzService:       newAllowAllAuthz(suite.T()),
		groupStore:         storeMock,
		transactioner:      &stubTransactioner{},
		dependencyRegistry: noopDepRegistry{cascadeErr: errors.New("cascade failed")},
	}

	err := service.DeleteGroup(context.Background(), "grp-001")

	suite.Require().NotNil(err)
	storeMock.AssertNotCalled(suite.T(), "DeleteGroup", mock.Anything, mock.Anything)
}

func TestResolveAppDisplay_AppUsesSystemAttributesName(t *testing.T) {
	app := providers.Entity{
		ID:               "app-1",
		Category:         providers.EntityCategoryApp,
		Attributes:       json.RawMessage(`{"name":"ignored"}`),
		SystemAttributes: json.RawMessage(`{"name":"My App"}`),
	}
	require.Equal(t, "My App", resolveAppDisplay(app))
}

type groupChangeFixture struct {
	store    *groupStoreInterfaceMock
	entities *entitymock.EntityServiceInterfaceMock
	authz    *sysauthzmock.SystemAuthorizationServiceInterfaceMock
	service  *groupService
}

func newGroupChangeFixture(t *testing.T) *groupChangeFixture {
	f := &groupChangeFixture{
		store:    newGroupStoreInterfaceMock(t),
		entities: entitymock.NewEntityServiceInterfaceMock(t),
		authz:    sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t),
	}
	f.service = &groupService{
		bus:           discardingBus{},
		groupStore:    f.store,
		entityService: f.entities,
		authzService:  f.authz,
		transactioner: &stubTransactioner{},
	}
	return f
}

func (f *groupChangeFixture) mayUpdateGroup() {
	f.store.On("GetGroup", mock.Anything, "grp-001").Return(GroupDAO{ID: "grp-001", OUID: testOUID1}, nil)
	f.store.On("IsGroupDeclarative", mock.Anything, "grp-001").Return(false, nil).Maybe()
	f.authz.On("IsActionAllowed", mock.Anything, security.ActionUpdateGroup, &sysauthz.ActionContext{
		ResourceType: security.ResourceTypeGroup, OUID: testOUID1, ResourceID: "grp-001",
	}).Return(true, (*tidcommon.ServiceError)(nil))
	f.authz.On("CanGrantMembership", mock.Anything, sysauthz.PrincipalTypeGroup, "grp-001").
		Return((*tidcommon.ServiceError)(nil))
}

func (suite *GroupServiceTestSuite) TestValidateDeleteGroup_AcceptsWithoutDeleting() {
	f := newGroupChangeFixture(suite.T())
	f.store.On("IsGroupDeclarative", mock.Anything, "grp-001").Return(false, nil)
	f.store.On("GetGroup", mock.Anything, "grp-001").Return(GroupDAO{ID: "grp-001", OUID: testOUID1}, nil)
	f.authz.On("IsActionAllowed", mock.Anything, security.ActionDeleteGroup, &sysauthz.ActionContext{
		ResourceType: security.ResourceTypeGroup, OUID: testOUID1, ResourceID: "grp-001",
	}).Return(true, (*tidcommon.ServiceError)(nil))
	f.authz.On("CanGrantMembership", mock.Anything, sysauthz.PrincipalTypeGroup, "grp-001").
		Return((*tidcommon.ServiceError)(nil))

	svcErr := f.service.ValidateDeleteGroup(context.Background(), "grp-001")

	suite.Require().Nil(svcErr)
	f.store.AssertNotCalled(suite.T(), "DeleteGroup", mock.Anything, mock.Anything)
}

func (suite *GroupServiceTestSuite) TestValidateDeleteGroup_Refusals() {
	cases := map[string]struct {
		groupID string
		setup   func(f *groupChangeFixture)
		want    tidcommon.ServiceError
	}{
		"missing id": {"", func(*groupChangeFixture) {}, ErrorMissingGroupID},
		"declarative group": {"grp-001", func(f *groupChangeFixture) {
			f.store.On("IsGroupDeclarative", mock.Anything, "grp-001").Return(true, nil)
		}, ErrorImmutableGroup},
		"declarative check failure": {"grp-001", func(f *groupChangeFixture) {
			f.store.On("IsGroupDeclarative", mock.Anything, "grp-001").Return(false, errors.New("db down"))
		}, tidcommon.InternalServerError},
		"group not found": {"grp-001", func(f *groupChangeFixture) {
			f.store.On("IsGroupDeclarative", mock.Anything, "grp-001").Return(false, nil)
			f.store.On("GetGroup", mock.Anything, "grp-001").Return(GroupDAO{}, ErrGroupNotFound)
		}, ErrorGroupNotFound},
		"caller may not delete": {"grp-001", func(f *groupChangeFixture) {
			f.store.On("IsGroupDeclarative", mock.Anything, "grp-001").Return(false, nil)
			f.store.On("GetGroup", mock.Anything, "grp-001").Return(GroupDAO{ID: "grp-001", OUID: testOUID1}, nil)
			f.authz.On("IsActionAllowed", mock.Anything, security.ActionDeleteGroup, mock.Anything).
				Return(false, (*tidcommon.ServiceError)(nil))
		}, tidcommon.ErrorUnauthorized},
		"grant not permitted": {"grp-001", func(f *groupChangeFixture) {
			f.store.On("IsGroupDeclarative", mock.Anything, "grp-001").Return(false, nil)
			f.store.On("GetGroup", mock.Anything, "grp-001").Return(GroupDAO{ID: "grp-001", OUID: testOUID1}, nil)
			f.authz.On("IsActionAllowed", mock.Anything, security.ActionDeleteGroup, mock.Anything).
				Return(true, (*tidcommon.ServiceError)(nil))
			f.authz.On("CanGrantMembership", mock.Anything, sysauthz.PrincipalTypeGroup, "grp-001").
				Return(&sysauthz.ErrorGrantNotPermitted)
		}, sysauthz.ErrorGrantNotPermitted},
	}
	for name, tc := range cases {
		suite.Run(name, func() {
			f := newGroupChangeFixture(suite.T())
			tc.setup(f)

			svcErr := f.service.ValidateDeleteGroup(context.Background(), tc.groupID)

			suite.Require().NotNil(svcErr)
			suite.Equal(tc.want.Code, svcErr.Code)
		})
	}
}

func (suite *GroupServiceTestSuite) TestValidateRemoveGroupMembers_AcceptsWithoutRemoving() {
	f := newGroupChangeFixture(suite.T())
	f.mayUpdateGroup()
	f.entities.On("GetEntitiesByIDs", mock.Anything, []string{"usr-001"}).
		Return([]providers.Entity{{ID: "usr-001", Category: providers.EntityCategoryUser}}, nil)

	svcErr := f.service.ValidateRemoveGroupMembers(context.Background(), "grp-001",
		[]Member{{ID: "usr-001", Type: MemberTypeUser}})

	suite.Require().Nil(svcErr)
	f.store.AssertNotCalled(suite.T(), "RemoveGroupMembers", mock.Anything, mock.Anything, mock.Anything)
}

// A member owned by an organization unit the caller cannot reach, such as one added across units
// before members were bound to the group's unit, can still be removed by the group's administrator.
func (suite *GroupServiceTestSuite) TestValidateRemoveGroupMembers_AllowsAMemberOutsideTheCallersOU() {
	f := newGroupChangeFixture(suite.T())
	f.mayUpdateGroup()
	f.entities.On("GetEntitiesByIDs", mock.Anything, []string{"usr-001"}).
		Return([]providers.Entity{{ID: "usr-001", Category: providers.EntityCategoryUser, OUID: testOUID2}}, nil)

	svcErr := f.service.ValidateRemoveGroupMembers(context.Background(), "grp-001",
		[]Member{{ID: "usr-001", Type: MemberTypeUser}})

	suite.Require().Nil(svcErr)
}

func (suite *GroupServiceTestSuite) TestValidateRemoveGroupMembers_RefusesACallerWhoMayNotChangeMembers() {
	f := newGroupChangeFixture(suite.T())
	f.store.On("GetGroup", mock.Anything, "grp-001").Return(GroupDAO{ID: "grp-001", OUID: testOUID1}, nil)
	f.authz.On("IsActionAllowed", mock.Anything, security.ActionUpdateGroup, mock.Anything).
		Return(true, (*tidcommon.ServiceError)(nil))
	f.authz.On("CanGrantMembership", mock.Anything, sysauthz.PrincipalTypeGroup, "grp-001").
		Return(&tidcommon.ErrorUnauthorized)

	svcErr := f.service.ValidateRemoveGroupMembers(context.Background(), "grp-001",
		[]Member{{ID: "usr-001", Type: MemberTypeUser}})

	suite.Require().NotNil(svcErr)
	suite.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	f.entities.AssertNotCalled(suite.T(), "GetEntitiesByIDs", mock.Anything, mock.Anything)
}

func (suite *GroupServiceTestSuite) TestValidateRemoveGroupMembers_RefusesACallerWhoMayNotUpdateTheGroup() {
	f := newGroupChangeFixture(suite.T())
	f.store.On("GetGroup", mock.Anything, "grp-001").Return(GroupDAO{ID: "grp-001", OUID: testOUID1}, nil)
	f.authz.On("IsActionAllowed", mock.Anything, security.ActionUpdateGroup, &sysauthz.ActionContext{
		ResourceType: security.ResourceTypeGroup, OUID: testOUID1, ResourceID: "grp-001",
	}).Return(false, (*tidcommon.ServiceError)(nil))

	svcErr := f.service.ValidateRemoveGroupMembers(context.Background(), "grp-001",
		[]Member{{ID: "usr-001", Type: MemberTypeUser}})

	suite.Require().NotNil(svcErr)
	suite.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	f.authz.AssertNotCalled(suite.T(), "CanGrantMembership", mock.Anything, mock.Anything, mock.Anything)
	f.entities.AssertNotCalled(suite.T(), "GetEntitiesByIDs", mock.Anything, mock.Anything)
}

func (suite *GroupServiceTestSuite) TestValidateRemoveGroupMembers_RefusesAMalformedRequest() {
	f := newGroupChangeFixture(suite.T())
	cases := map[string]struct {
		groupID string
		members []Member
		want    tidcommon.ServiceError
	}{
		"missing group":  {"", []Member{{ID: "usr-001", Type: MemberTypeUser}}, ErrorMissingGroupID},
		"no members":     {"grp-001", nil, ErrorEmptyMembers},
		"unknown type":   {"grp-001", []Member{{ID: "usr-001", Type: "robot"}}, ErrorInvalidMemberType},
		"missing member": {"grp-001", []Member{{Type: MemberTypeUser}}, ErrorInvalidRequestFormat},
	}
	for name, tc := range cases {
		suite.Run(name, func() {
			svcErr := f.service.ValidateRemoveGroupMembers(context.Background(), tc.groupID, tc.members)

			suite.Require().NotNil(svcErr)
			suite.Equal(tc.want.Code, svcErr.Code)
		})
	}
	f.store.AssertNotCalled(suite.T(), "GetGroup", mock.Anything, mock.Anything)
}

func (suite *GroupServiceTestSuite) TestValidateRemoveGroupMembers_RefusesAnUnknownGroupMember() {
	f := newGroupChangeFixture(suite.T())
	f.mayUpdateGroup()
	f.store.On("ValidateGroupIDs", mock.Anything, []string{"child-grp"}).Return([]string{"child-grp"}, nil)

	svcErr := f.service.ValidateRemoveGroupMembers(context.Background(), "grp-001",
		[]Member{{ID: "child-grp", Type: MemberTypeGroup}})

	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorInvalidGroupMemberID.Code, svcErr.Code)
}

// readableGroup stubs a group the caller may read, holding the members as the store records them.
func (f *groupChangeFixture) readableGroup(groupID string, members ...Member) {
	f.store.On("GetGroup", mock.Anything, groupID).Return(GroupDAO{ID: groupID, OUID: testOUID1}, nil)
	f.authz.On("IsActionAllowed", mock.Anything, security.ActionReadGroup, &sysauthz.ActionContext{
		ResourceType: security.ResourceTypeGroup, OUID: testOUID1, ResourceID: groupID,
	}).Return(true, (*tidcommon.ServiceError)(nil))
	f.store.On("GetGroupMemberCount", mock.Anything, groupID).Return(len(members), nil)
	f.store.On("GetGroupMembers", mock.Anything, groupID, serverconst.MaxPageSize, 0).Return(members, nil)
}

func (f *groupChangeFixture) knownUsers(ids ...string) {
	users := make([]providers.Entity, 0, len(ids))
	for _, id := range ids {
		users = append(users, providers.Entity{ID: id, Category: providers.EntityCategoryUser})
	}
	f.entities.On("GetEntitiesByIDs", mock.Anything, ids).Return(users, nil)
}

func (suite *GroupServiceTestSuite) TestGetTransitiveMemberEntities_SurvivesAMembershipCycle() {
	f := newGroupChangeFixture(suite.T())
	f.readableGroup("grp-001", Member{ID: "child-grp", Type: MemberTypeGroup})
	f.readableGroup("child-grp",
		Member{ID: "grp-001", Type: MemberTypeGroup}, Member{ID: "usr-001", Type: memberTypeEntity})
	f.knownUsers("usr-001")

	entityIDs, svcErr := f.service.GetTransitiveMemberEntities(context.Background(), "grp-001")

	suite.Require().Nil(svcErr)
	suite.Equal([]string{"usr-001"}, entityIDs)
}

func (suite *GroupServiceTestSuite) TestGetTransitiveMemberEntities_DeduplicatesADiamond() {
	f := newGroupChangeFixture(suite.T())
	f.readableGroup("grp-001", Member{ID: "left", Type: MemberTypeGroup}, Member{ID: "right", Type: MemberTypeGroup})
	f.readableGroup("left", Member{ID: "usr-001", Type: memberTypeEntity})
	f.readableGroup("right", Member{ID: "usr-001", Type: memberTypeEntity})
	f.knownUsers("usr-001")

	entityIDs, svcErr := f.service.GetTransitiveMemberEntities(context.Background(), "grp-001")

	suite.Require().Nil(svcErr)
	suite.Equal([]string{"usr-001"}, entityIDs)
}

func (suite *GroupServiceTestSuite) TestGetTransitiveMemberEntities_ReadsEveryPage() {
	f := newGroupChangeFixture(suite.T())
	f.store.On("GetGroup", mock.Anything, "grp-001").Return(GroupDAO{ID: "grp-001", OUID: testOUID1}, nil)
	f.authz.On("IsActionAllowed", mock.Anything, security.ActionReadGroup, mock.Anything).
		Return(true, (*tidcommon.ServiceError)(nil))
	f.store.On("GetGroupMemberCount", mock.Anything, "grp-001").Return(serverconst.MaxPageSize+1, nil)
	f.store.On("GetGroupMembers", mock.Anything, "grp-001", serverconst.MaxPageSize, 0).
		Return([]Member{{ID: "usr-001", Type: memberTypeEntity}}, nil)
	f.store.On("GetGroupMembers", mock.Anything, "grp-001", serverconst.MaxPageSize, serverconst.MaxPageSize).
		Return([]Member{{ID: "usr-002", Type: memberTypeEntity}}, nil)
	f.knownUsers("usr-001")
	f.knownUsers("usr-002")

	entityIDs, svcErr := f.service.GetTransitiveMemberEntities(context.Background(), "grp-001")

	suite.Require().Nil(svcErr)
	suite.Equal([]string{"usr-001", "usr-002"}, entityIDs)
}

func (suite *GroupServiceTestSuite) TestGetTransitiveMemberEntities_RefusesNestingBeyondTheBound() {
	f := newGroupChangeFixture(suite.T())
	for depth := 0; depth < maxGroupNestingDepth; depth++ {
		f.readableGroup(fmt.Sprintf("g-%d", depth), Member{ID: fmt.Sprintf("g-%d", depth+1), Type: MemberTypeGroup})
	}

	_, svcErr := f.service.GetTransitiveMemberEntities(context.Background(), "g-0")

	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorGroupNestingTooDeep.Code, svcErr.Code)
	suite.Equal(fmt.Sprintf("The group's members are nested more than %d levels deep, which cannot be expanded",
		maxGroupNestingDepth), svcErr.ErrorDescription.String())
	f.store.AssertNotCalled(suite.T(), "GetGroup", mock.Anything, fmt.Sprintf("g-%d", maxGroupNestingDepth))
}

// Every nested group is read with the caller's own access, so a group it cannot read refuses the walk.
func (suite *GroupServiceTestSuite) TestGetTransitiveMemberEntities_RefusesANestedGroupTheCallerCannotRead() {
	f := newGroupChangeFixture(suite.T())
	f.readableGroup("grp-001", Member{ID: "child-grp", Type: MemberTypeGroup})
	f.store.On("GetGroup", mock.Anything, "child-grp").Return(GroupDAO{ID: "child-grp", OUID: testOUID2}, nil)
	f.authz.On("IsActionAllowed", mock.Anything, security.ActionReadGroup, &sysauthz.ActionContext{
		ResourceType: security.ResourceTypeGroup, OUID: testOUID2, ResourceID: "child-grp",
	}).Return(false, (*tidcommon.ServiceError)(nil))

	_, svcErr := f.service.GetTransitiveMemberEntities(context.Background(), "grp-001")

	suite.Require().NotNil(svcErr)
	suite.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	f.store.AssertNotCalled(suite.T(), "GetGroupMembers", mock.Anything, "child-grp", mock.Anything, mock.Anything)
}

// readableGroupWithMember sets up grp-001 for a caller that can read it, and the store's single-member
// lookup in it.
func (f *groupChangeFixture) readableGroupWithMember(memberID string, member *Member) {
	f.store.On("GetGroup", mock.Anything, "grp-001").Return(GroupDAO{ID: "grp-001", OUID: testOUID1}, nil)
	f.authz.On("IsActionAllowed", mock.Anything, security.ActionReadGroup, &sysauthz.ActionContext{
		ResourceType: security.ResourceTypeGroup, OUID: testOUID1, ResourceID: "grp-001",
	}).Return(true, (*tidcommon.ServiceError)(nil))
	f.store.On("GetGroupMember", mock.Anything, "grp-001", memberID).Return(member, nil)
}

func (suite *GroupServiceTestSuite) TestGetDirectGroupMember_FindsTheMember() {
	f := newGroupChangeFixture(suite.T())
	f.readableGroupWithMember("child-grp", &Member{ID: "child-grp", Type: MemberTypeGroup})

	member, svcErr := f.service.GetDirectGroupMember(context.Background(), "grp-001", "child-grp")

	suite.Require().Nil(svcErr)
	suite.Equal(&Member{ID: "child-grp", Type: MemberTypeGroup}, member)
}

// The member is looked up on its own, so the group's membership is never listed.
func (suite *GroupServiceTestSuite) TestGetDirectGroupMember_ResolvesAnEntityWithoutListingTheMembership() {
	f := newGroupChangeFixture(suite.T())
	f.readableGroupWithMember("usr-002", &Member{ID: "usr-002", Type: memberTypeEntity})
	f.knownUsers("usr-002")

	member, svcErr := f.service.GetDirectGroupMember(context.Background(), "grp-001", "usr-002")

	suite.Require().Nil(svcErr)
	suite.Equal(&Member{ID: "usr-002", Type: MemberTypeUser}, member)
	f.store.AssertNotCalled(suite.T(), "GetGroupMembers", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	f.store.AssertNotCalled(suite.T(), "GetGroupMemberCount", mock.Anything, mock.Anything)
}

func (suite *GroupServiceTestSuite) TestGetDirectGroupMember_NilForANonMember() {
	f := newGroupChangeFixture(suite.T())
	f.readableGroupWithMember("usr-002", nil)

	member, svcErr := f.service.GetDirectGroupMember(context.Background(), "grp-001", "usr-002")

	suite.Require().Nil(svcErr)
	suite.Nil(member)
}

func (suite *GroupServiceTestSuite) TestGetDirectGroupMember_NilForAnOrphanedEntity() {
	f := newGroupChangeFixture(suite.T())
	f.readableGroupWithMember("usr-002", &Member{ID: "usr-002", Type: memberTypeEntity})
	f.entities.On("GetEntitiesByIDs", mock.Anything, []string{"usr-002"}).Return([]providers.Entity{}, nil)

	member, svcErr := f.service.GetDirectGroupMember(context.Background(), "grp-001", "usr-002")

	suite.Require().Nil(svcErr)
	suite.Nil(member)
}

func (suite *GroupServiceTestSuite) TestGetDirectGroupMember_CarriesTheLookupError() {
	f := newGroupChangeFixture(suite.T())
	f.store.On("GetGroup", mock.Anything, "grp-001").Return(GroupDAO{ID: "grp-001", OUID: testOUID1}, nil)
	f.authz.On("IsActionAllowed", mock.Anything, security.ActionReadGroup, mock.Anything).
		Return(true, (*tidcommon.ServiceError)(nil))
	f.store.On("GetGroupMember", mock.Anything, "grp-001", "usr-001").Return(nil, errors.New("db down"))

	member, svcErr := f.service.GetDirectGroupMember(context.Background(), "grp-001", "usr-001")

	suite.Require().NotNil(svcErr)
	suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	suite.Nil(member)
}

func (suite *GroupServiceTestSuite) TestGetDirectGroupMember_RefusesACallerThatCannotReadTheGroup() {
	f := newGroupChangeFixture(suite.T())
	f.store.On("GetGroup", mock.Anything, "grp-001").Return(GroupDAO{ID: "grp-001", OUID: testOUID1}, nil)
	f.authz.On("IsActionAllowed", mock.Anything, security.ActionReadGroup, mock.Anything).
		Return(false, (*tidcommon.ServiceError)(nil))

	member, svcErr := f.service.GetDirectGroupMember(context.Background(), "grp-001", "usr-001")

	suite.Require().NotNil(svcErr)
	suite.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	suite.Nil(member)
	f.store.AssertNotCalled(suite.T(), "GetGroupMember", mock.Anything, mock.Anything, mock.Anything)
}

func (suite *GroupServiceTestSuite) TestGetDirectGroupMember_RequiresAGroupID() {
	f := newGroupChangeFixture(suite.T())

	member, svcErr := f.service.GetDirectGroupMember(context.Background(), "", "usr-001")

	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorMissingGroupID.Code, svcErr.Code)
	suite.Nil(member)
}

func (suite *GroupServiceTestSuite) TestGetDirectGroupMember_CarriesTheReadError() {
	f := newGroupChangeFixture(suite.T())
	f.store.On("GetGroup", mock.Anything, "grp-001").Return(GroupDAO{}, ErrGroupNotFound)

	member, svcErr := f.service.GetDirectGroupMember(context.Background(), "grp-001", "usr-001")

	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorGroupNotFound.Code, svcErr.Code)
	suite.Nil(member)
}

const (
	testProjectID      = "project-1"
	testOtherProjectID = "project-2"
)

// newProjectGroupService returns a group service whose organization units all exist, that creates and updates
// groups in the given store, and that checks projects with the given validator.
func newProjectGroupService(t *testing.T, storeMock *groupStoreInterfaceMock,
	validator projectOUValidator) (*groupService, *oumock.OrganizationUnitServiceInterfaceMock) {
	ouMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	ouMock.On("IsOrganizationUnitExists", mock.Anything, mock.Anything).
		Return(true, (*tidcommon.ServiceError)(nil)).Maybe()
	return &groupService{
		bus:              discardingBus{},
		authzService:     newAllowAllAuthz(t),
		groupStore:       storeMock,
		ouService:        ouMock,
		transactioner:    &stubTransactioner{},
		projectValidator: validator,
	}, ouMock
}

func (suite *GroupServiceTestSuite) TestCreateGroup_InProject() {
	t := suite.T()
	validator := newProjectOUValidatorMock(t)
	validator.EXPECT().ValidateProjectOU(mock.Anything, testProjectID, testOUID1).
		Return((*tidcommon.ServiceError)(nil)).Once()
	storeMock := newGroupStoreInterfaceMock(t)
	storeMock.On("CheckGroupNameConflictForCreate", mock.Anything, "Shoppers", testOUID1).Return(nil).Once()
	storeMock.On("CreateGroup", mock.Anything, mock.MatchedBy(func(g GroupDAO) bool {
		return g.ProjectID == testProjectID && g.OUID == testOUID1
	})).Return(nil).Once()
	service, _ := newProjectGroupService(t, storeMock, validator)

	created, err := service.CreateGroup(context.Background(),
		CreateGroupRequest{Name: "Shoppers", OUID: testOUID1, ProjectID: testProjectID})

	require.Nil(t, err)
	require.Equal(t, testProjectID, created.ProjectID)
}

// A group in a project that names no organization unit goes to the system organization unit.
func (suite *GroupServiceTestSuite) TestCreateGroup_InProjectWithoutOUGoesToTheSystemOU() {
	t := suite.T()
	validator := newProjectOUValidatorMock(t)
	validator.EXPECT().ValidateProjectOU(mock.Anything, testProjectID, testOUID2).
		Return((*tidcommon.ServiceError)(nil)).Once()
	storeMock := newGroupStoreInterfaceMock(t)
	storeMock.On("CheckGroupNameConflictForCreate", mock.Anything, "Shoppers", testOUID2).Return(nil).Once()
	storeMock.On("CreateGroup", mock.Anything, mock.MatchedBy(func(g GroupDAO) bool {
		return g.OUID == testOUID2
	})).Return(nil).Once()
	service, ouMock := newProjectGroupService(t, storeMock, validator)
	ouMock.On("GetSystemOUID", mock.Anything).Return(testOUID2, (*tidcommon.ServiceError)(nil)).Once()

	created, err := service.CreateGroup(context.Background(),
		CreateGroupRequest{Name: "Shoppers", ProjectID: testProjectID})

	require.Nil(t, err)
	require.Equal(t, testOUID2, created.OUID)
}

func (suite *GroupServiceTestSuite) TestCreateGroup_InProjectSystemOUFailure() {
	t := suite.T()
	service, ouMock := newProjectGroupService(t, newGroupStoreInterfaceMock(t), newProjectOUValidatorMock(t))
	ouMock.On("GetSystemOUID", mock.Anything).Return("", &tidcommon.InternalServerError).Once()

	_, err := service.CreateGroup(context.Background(), CreateGroupRequest{Name: "Shoppers", ProjectID: testProjectID})

	require.NotNil(t, err)
	require.Equal(t, tidcommon.InternalServerError.Code, err.Code)
}

// A group without a project or an organization unit is refused, as before.
func (suite *GroupServiceTestSuite) TestCreateGroup_WithoutOUOrProjectIsRefused() {
	t := suite.T()
	service, _ := newProjectGroupService(t, newGroupStoreInterfaceMock(t), newProjectOUValidatorMock(t))

	_, err := service.CreateGroup(context.Background(), CreateGroupRequest{Name: "Shoppers"})

	require.NotNil(t, err)
	require.Equal(t, ErrorInvalidRequestFormat.Code, err.Code)
}

func (suite *GroupServiceTestSuite) TestCreateGroup_InProjectRefused() {
	for name, tc := range map[string]struct {
		validatorErr *tidcommon.ServiceError
		expected     string
	}{
		"missing project": {
			validatorErr: &tidcommon.ServiceError{Code: projectscope.ProjectNotFoundErrorCode},
			expected:     ErrorInvalidProject.Code,
		},
		"organization unit outside project": {
			validatorErr: &tidcommon.ServiceError{Code: projectscope.OUNotInProjectErrorCode},
			expected:     ErrorOUNotInProject.Code,
		},
		"validator failure": {
			validatorErr: &tidcommon.InternalServerError,
			expected:     tidcommon.InternalServerError.Code,
		},
	} {
		suite.Run(name, func() {
			t := suite.T()
			validator := newProjectOUValidatorMock(t)
			validator.EXPECT().ValidateProjectOU(mock.Anything, testProjectID, testOUID1).
				Return(tc.validatorErr).Once()
			service, _ := newProjectGroupService(t, newGroupStoreInterfaceMock(t), validator)

			_, err := service.CreateGroup(context.Background(),
				CreateGroupRequest{Name: "Shoppers", OUID: testOUID1, ProjectID: testProjectID})

			require.NotNil(t, err)
			require.Equal(t, tc.expected, err.Code)
		})
	}
}

// A deployment that serves no projects keeps the project as given.
func (suite *GroupServiceTestSuite) TestCreateGroup_InProjectWithoutValidator() {
	t := suite.T()
	storeMock := newGroupStoreInterfaceMock(t)
	storeMock.On("CheckGroupNameConflictForCreate", mock.Anything, "Shoppers", testOUID1).Return(nil).Once()
	storeMock.On("CreateGroup", mock.Anything, mock.MatchedBy(func(g GroupDAO) bool {
		return g.ProjectID == testProjectID
	})).Return(nil).Once()
	service, _ := newProjectGroupService(t, storeMock, nil)

	_, err := service.CreateGroup(context.Background(),
		CreateGroupRequest{Name: "Shoppers", OUID: testOUID1, ProjectID: testProjectID})

	require.Nil(t, err)
}

// newProjectUpdateStore returns a store holding the mutable group grp-001 in testOUID1 and the given project.
func newProjectUpdateStore(t *testing.T, storedProjectID string) *groupStoreInterfaceMock {
	storeMock := newGroupStoreInterfaceMock(t)
	storeMock.On("IsGroupDeclarative", mock.Anything, "grp-001").Return(false, nil).Once()
	storeMock.On("GetGroup", mock.Anything, "grp-001").Return(GroupDAO{
		ID: "grp-001", Name: "Shoppers", OUID: testOUID1, ProjectID: storedProjectID}, nil).Once()
	return storeMock
}

// An update that names no project keeps the group in its project, without checking it again.
func (suite *GroupServiceTestSuite) TestUpdateGroup_KeepsTheProject() {
	t := suite.T()
	storeMock := newProjectUpdateStore(t, testProjectID)
	storeMock.On("UpdateGroup", mock.Anything, mock.MatchedBy(func(g GroupDAO) bool {
		return g.ProjectID == testProjectID
	})).Return(nil).Once()
	service, _ := newProjectGroupService(t, storeMock, newProjectOUValidatorMock(t))

	updated, err := service.UpdateGroup(context.Background(), "grp-001",
		UpdateGroupRequest{Name: "Shoppers", OUID: testOUID1})

	require.Nil(t, err)
	require.Equal(t, testProjectID, updated.ProjectID)
}

func (suite *GroupServiceTestSuite) TestUpdateGroup_MovesToAProject() {
	t := suite.T()
	validator := newProjectOUValidatorMock(t)
	validator.EXPECT().ValidateProjectOU(mock.Anything, testOtherProjectID, testOUID1).
		Return((*tidcommon.ServiceError)(nil)).Once()
	storeMock := newProjectUpdateStore(t, testProjectID)
	storeMock.On("UpdateGroup", mock.Anything, mock.MatchedBy(func(g GroupDAO) bool {
		return g.ProjectID == testOtherProjectID
	})).Return(nil).Once()
	service, _ := newProjectGroupService(t, storeMock, validator)

	updated, err := service.UpdateGroup(context.Background(), "grp-001",
		UpdateGroupRequest{Name: "Shoppers", OUID: testOUID1, ProjectID: testOtherProjectID})

	require.Nil(t, err)
	require.Equal(t, testOtherProjectID, updated.ProjectID)
}

func (suite *GroupServiceTestSuite) TestUpdateGroup_RefusesAMissingProject() {
	t := suite.T()
	validator := newProjectOUValidatorMock(t)
	validator.EXPECT().ValidateProjectOU(mock.Anything, testProjectID, testOUID1).
		Return(&tidcommon.ServiceError{Code: projectscope.ProjectNotFoundErrorCode}).Once()
	service, _ := newProjectGroupService(t, newProjectUpdateStore(t, ""), validator)

	_, err := service.UpdateGroup(context.Background(), "grp-001",
		UpdateGroupRequest{Name: "Shoppers", OUID: testOUID1, ProjectID: testProjectID})

	require.NotNil(t, err)
	require.Equal(t, ErrorInvalidProject.Code, err.Code)
}

// newProjectListService returns a group service whose caller can list groups in the given organization units,
// or in all of them when allAllowed is set.
func newProjectListService(t *testing.T, allAllowed bool, ouIDs []string,
	storeMock *groupStoreInterfaceMock) *groupService {
	authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
	authzMock.On("GetAccessibleResources", mock.Anything, security.ActionListGroups, security.ResourceTypeOU).
		Return(&sysauthz.AccessibleResources{AllAllowed: allAllowed, IDs: ouIDs}, (*tidcommon.ServiceError)(nil)).
		Maybe()
	return &groupService{authzService: authzMock, groupStore: storeMock}
}

func (suite *GroupServiceTestSuite) TestGetGroupListByProject() {
	t := suite.T()
	storeMock := newGroupStoreInterfaceMock(t)
	storeMock.On("GetGroupListCountByProject", mock.Anything, testProjectID, []string(nil)).Return(3, nil).Once()
	storeMock.On("GetGroupListByProject", mock.Anything, testProjectID, []string(nil), 1, 0).
		Return([]GroupBasicDAO{{ID: "grp-001", OUID: testOUID1, ProjectID: testProjectID}}, nil).Once()
	service := newProjectListService(t, true, nil, storeMock)

	resp, err := service.GetGroupListByProject(context.Background(), testProjectID, 1, 0, nil, false)

	require.Nil(t, err)
	require.Equal(t, 3, resp.TotalResults)
	require.Equal(t, testProjectID, resp.Groups[0].ProjectID)
	require.Contains(t, resp.Links[0].Href, "projectId="+testProjectID)
}

// A caller limited to some organization units lists only the project's groups in those units, and an
// organization unit filter narrows the list to that unit.
func (suite *GroupServiceTestSuite) TestGetGroupListByProject_NarrowedToOrganizationUnits() {
	t := suite.T()
	storeMock := newGroupStoreInterfaceMock(t)
	storeMock.On("GetGroupListCountByProject", mock.Anything, testProjectID, []string{testOUID1}).
		Return(0, nil).Twice()
	storeMock.On("GetGroupListByProject", mock.Anything, testProjectID, []string{testOUID1}, 10, 0).
		Return([]GroupBasicDAO{}, nil).Twice()

	_, err := newProjectListService(t, false, []string{testOUID1}, storeMock).
		GetGroupListByProject(context.Background(), testProjectID, 10, 0, nil, false)
	require.Nil(t, err)

	_, err = newProjectListService(t, true, nil, storeMock).GetGroupListByProject(context.Background(),
		testProjectID, 10, 0, map[string]interface{}{"ouId": testOUID1}, false)
	require.Nil(t, err)
}

// Some lists have nothing to show without reading the store.
func (suite *GroupServiceTestSuite) TestGetGroupListByProject_ListsNone() {
	for name, tc := range map[string]struct {
		projectID string
		ouIDs     []string
		filters   map[string]interface{}
	}{
		"no project": {ouIDs: []string{testOUID1}},
		"organization unit out of reach": {projectID: testProjectID, ouIDs: []string{testOUID2},
			filters: map[string]interface{}{"ouId": testOUID1}},
		"no organization unit in reach": {projectID: testProjectID},
	} {
		suite.Run(name, func() {
			service := newProjectListService(suite.T(), false, tc.ouIDs, newGroupStoreInterfaceMock(suite.T()))

			resp, err := service.GetGroupListByProject(context.Background(), tc.projectID, 10, 0, tc.filters, false)

			require.Nil(suite.T(), err)
			require.Zero(suite.T(), resp.TotalResults)
			require.Empty(suite.T(), resp.Groups)
		})
	}
}

func (suite *GroupServiceTestSuite) TestGetGroupListByProject_Errors() {
	t := suite.T()
	service := newProjectListService(t, true, nil, newGroupStoreInterfaceMock(t))
	_, err := service.GetGroupListByProject(context.Background(), testProjectID, -1, 0, nil, false)
	require.Equal(t, ErrorInvalidLimit.Code, err.Code)
	_, err = service.GetGroupListByProject(context.Background(), testProjectID, 10, 0,
		map[string]interface{}{"name": "Shoppers"}, false)
	require.Equal(t, ErrorInvalidFilter.Code, err.Code)

	_, err = (&groupService{authzService: newAuthzError(t)}).GetGroupListByProject(context.Background(),
		testProjectID, 10, 0, nil, false)
	require.Equal(t, tidcommon.InternalServerError.Code, err.Code)

	storeMock := newGroupStoreInterfaceMock(t)
	storeMock.On("GetGroupListCountByProject", mock.Anything, "count-fails", []string(nil)).
		Return(0, errors.New("db fail")).Once()
	storeMock.On("GetGroupListCountByProject", mock.Anything, "list-fails", []string(nil)).Return(1, nil).Once()
	storeMock.On("GetGroupListByProject", mock.Anything, "list-fails", []string(nil), 10, 0).
		Return(nil, errors.New("db fail")).Once()
	service = newProjectListService(t, true, nil, storeMock)
	_, err = service.GetGroupListByProject(context.Background(), "count-fails", 10, 0, nil, false)
	require.Equal(t, tidcommon.InternalServerError.Code, err.Code)
	_, err = service.GetGroupListByProject(context.Background(), "list-fails", 10, 0, nil, false)
	require.Equal(t, tidcommon.InternalServerError.Code, err.Code)
}

// A project ID longer than the column is refused on every deployment, even one that checks no project.
func (suite *GroupServiceTestSuite) TestCreateGroup_RefusesATooLongProjectID() {
	t := suite.T()
	for name, validator := range map[string]projectOUValidator{
		"without a validator": nil,
		"with a validator":    newProjectOUValidatorMock(t),
	} {
		suite.Run(name, func() {
			service, _ := newProjectGroupService(suite.T(), newGroupStoreInterfaceMock(suite.T()), validator)

			_, err := service.CreateGroup(context.Background(), CreateGroupRequest{
				Name: "Shoppers", OUID: testOUID1, ProjectID: strings.Repeat("p", maxProjectIDLength+1)})

			require.NotNil(suite.T(), err)
			require.Equal(suite.T(), ErrorInvalidProject.Code, err.Code)
		})
	}
}

// newOUMoveStore returns a store holding the empty group grp-001 in testOUID1 and testProjectID, which is free
// to move to another organization unit.
func newOUMoveStore(t *testing.T) *groupStoreInterfaceMock {
	storeMock := newProjectUpdateStore(t, testProjectID)
	storeMock.On("GetGroupMemberCount", mock.Anything, "grp-001").Return(0, nil).Once()
	storeMock.On("GetDirectGroupParents", mock.Anything, []string{"grp-001"}).Return([]string{}, nil).Once()
	return storeMock
}

// An update that moves a project group to another organization unit checks that unit against the group's
// project, and refuses one outside it.
func (suite *GroupServiceTestSuite) TestUpdateGroup_ChecksTheProjectWhenOnlyTheOUChanges() {
	t := suite.T()
	validator := newProjectOUValidatorMock(t)
	validator.EXPECT().ValidateProjectOU(mock.Anything, testProjectID, testOUID2).
		Return(&tidcommon.ServiceError{Code: projectscope.OUNotInProjectErrorCode}).Once()
	resolver := NewRoleAssignmentResolverMock(t)
	resolver.On("HasGroupRoleAssignments", mock.Anything, "grp-001").
		Return(false, (*tidcommon.ServiceError)(nil)).Once()
	service, _ := newProjectGroupService(t, newOUMoveStore(t), validator)
	service.roleAssignments = resolver

	_, err := service.UpdateGroup(context.Background(), "grp-001",
		UpdateGroupRequest{Name: "Shoppers", OUID: testOUID2})

	require.NotNil(t, err)
	require.Equal(t, ErrorOUNotInProject.Code, err.Code)
}

func (suite *GroupServiceTestSuite) TestUpdateGroup_MovesOUWithinTheProject() {
	t := suite.T()
	validator := newProjectOUValidatorMock(t)
	validator.EXPECT().ValidateProjectOU(mock.Anything, testProjectID, testOUID2).
		Return((*tidcommon.ServiceError)(nil)).Once()
	resolver := NewRoleAssignmentResolverMock(t)
	resolver.On("HasGroupRoleAssignments", mock.Anything, "grp-001").
		Return(false, (*tidcommon.ServiceError)(nil)).Once()
	storeMock := newOUMoveStore(t)
	storeMock.On("CheckGroupNameConflictForUpdate", mock.Anything, "Shoppers", testOUID2, "grp-001").Return(nil).Once()
	storeMock.On("UpdateGroup", mock.Anything, mock.MatchedBy(func(g GroupDAO) bool {
		return g.OUID == testOUID2 && g.ProjectID == testProjectID
	})).Return(nil).Once()
	service, _ := newProjectGroupService(t, storeMock, validator)
	service.roleAssignments = resolver

	updated, err := service.UpdateGroup(context.Background(), "grp-001",
		UpdateGroupRequest{Name: "Shoppers", OUID: testOUID2})

	require.Nil(t, err)
	require.Equal(t, testOUID2, updated.OUID)
	require.Equal(t, testProjectID, updated.ProjectID)
}

func (suite *GroupServiceTestSuite) TestProjectHasGroups() {
	for name, tc := range map[string]struct {
		count    int
		err      error
		expected bool
		svcErr   bool
	}{
		"groups":                   {count: 2, expected: true},
		"no groups":                {count: 0, expected: false},
		"more than can be counted": {err: errResultLimitExceededInCompositeMode, expected: true},
		"store failure":            {err: errors.New("db fail"), svcErr: true},
	} {
		suite.Run(name, func() {
			storeMock := newGroupStoreInterfaceMock(suite.T())
			storeMock.On("GetGroupListCountByProject", mock.Anything, testProjectID, []string(nil)).
				Return(tc.count, tc.err).Once()
			service := &groupService{groupStore: storeMock}

			has, svcErr := service.ProjectHasGroups(context.Background(), testProjectID)

			if tc.svcErr {
				require.NotNil(suite.T(), svcErr)
				require.Equal(suite.T(), tidcommon.InternalServerError.Code, svcErr.Code)
				return
			}
			require.Nil(suite.T(), svcErr)
			require.Equal(suite.T(), tc.expected, has)
		})
	}
}
