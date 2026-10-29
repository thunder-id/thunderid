// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ou

import (
	"context"
	"errors"

	"github.com/stretchr/testify/mock"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

var testOwnerErrors = OwnerErrors{
	Required: tidcommon.ServiceError{Code: "T-1"},
	NotFound: tidcommon.ServiceError{Code: "T-2"},
	Changed:  tidcommon.ServiceError{Code: "T-3"},
}

func (suite *OrganizationUnitServiceTestSuite) TestOUService_ResolveOwner() {
	testCases := []struct {
		name    string
		id      string
		handle  string
		setup   func(*organizationUnitStoreInterfaceMock)
		wantID  string
		wantErr *tidcommon.ServiceError
	}{
		{name: "neither id nor handle", wantErr: &testOwnerErrors.Required},
		{
			name: "id that exists",
			id:   "ou-1",
			setup: func(s *organizationUnitStoreInterfaceMock) {
				s.On("IsOrganizationUnitExists", mock.Anything, "ou-1").Return(true, nil).Once()
			},
			wantID: "ou-1",
		},
		{
			name: "id wins when both are given",
			id:   "ou-1", handle: "acme",
			setup: func(s *organizationUnitStoreInterfaceMock) {
				s.On("IsOrganizationUnitExists", mock.Anything, "ou-1").Return(true, nil).Once()
			},
			wantID: "ou-1",
		},
		{
			name: "id that does not exist",
			id:   "missing",
			setup: func(s *organizationUnitStoreInterfaceMock) {
				s.On("IsOrganizationUnitExists", mock.Anything, "missing").Return(false, nil).Once()
			},
			wantErr: &testOwnerErrors.NotFound,
		},
		{
			name: "failed id lookup",
			id:   "ou-1",
			setup: func(s *organizationUnitStoreInterfaceMock) {
				s.On("IsOrganizationUnitExists", mock.Anything, "ou-1").Return(false, errors.New("boom")).Once()
			},
			wantErr: &tidcommon.InternalServerError,
		},
		{
			name:   "handle that exists",
			handle: "acme",
			setup: func(s *organizationUnitStoreInterfaceMock) {
				s.On("GetOrganizationUnitByPath", mock.Anything, []string{"acme"}).
					Return(OrganizationUnit{ID: "ou-acme"}, nil).Once()
			},
			wantID: "ou-acme",
		},
		{
			name:   "handle that does not exist",
			handle: "nowhere",
			setup: func(s *organizationUnitStoreInterfaceMock) {
				s.On("GetOrganizationUnitByPath", mock.Anything, []string{"nowhere"}).
					Return(OrganizationUnit{}, ErrOrganizationUnitNotFound).Once()
			},
			wantErr: &testOwnerErrors.NotFound,
		},
		{
			name:   "failed handle lookup is not taken for a missing unit",
			handle: "acme",
			setup: func(s *organizationUnitStoreInterfaceMock) {
				s.On("GetOrganizationUnitByPath", mock.Anything, []string{"acme"}).
					Return(OrganizationUnit{}, errors.New("boom")).Once()
			},
			wantErr: &tidcommon.InternalServerError,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			store := newOrganizationUnitStoreInterfaceMock(suite.T())
			if tc.setup != nil {
				tc.setup(store)
			}
			service := suite.newService(store, newAllowAllAuthz(suite.T()))

			id, svcErr := service.ResolveOwner(context.Background(), tc.id, tc.handle, testOwnerErrors)

			if tc.wantErr != nil {
				suite.Require().NotNil(svcErr)
				suite.Equal(tc.wantErr.Code, svcErr.Code)
				return
			}
			suite.Require().Nil(svcErr)
			suite.Equal(tc.wantID, id)
		})
	}
}

func (suite *OrganizationUnitServiceTestSuite) TestOUService_ResolveOwnerForUpdate() {
	testCases := []struct {
		name      string
		id        string
		handle    string
		setup     func(*organizationUnitStoreInterfaceMock)
		wantErr   *tidcommon.ServiceError
		wantOwner string
	}{
		{name: "keeps the current owner when none is requested", wantOwner: "ou-1"},
		{name: "accepts the current owner by id", id: "ou-1", wantOwner: "ou-1"},
		{
			name:   "accepts the current owner by handle",
			handle: "acme",
			setup: func(s *organizationUnitStoreInterfaceMock) {
				s.On("GetOrganizationUnitByPath", mock.Anything, []string{"acme"}).
					Return(OrganizationUnit{ID: "ou-1"}, nil).Once()
			},
			wantOwner: "ou-1",
		},
		{name: "refuses another owner", id: "ou-2", wantErr: &testOwnerErrors.Changed},
		{
			name:   "refuses a handle that cannot be resolved",
			handle: "nowhere",
			setup: func(s *organizationUnitStoreInterfaceMock) {
				s.On("GetOrganizationUnitByPath", mock.Anything, []string{"nowhere"}).
					Return(OrganizationUnit{}, ErrOrganizationUnitNotFound).Once()
			},
			wantErr: &testOwnerErrors.NotFound,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			store := newOrganizationUnitStoreInterfaceMock(suite.T())
			if tc.setup != nil {
				tc.setup(store)
			}
			service := suite.newService(store, newAllowAllAuthz(suite.T()))

			owner, svcErr := service.ResolveOwnerForUpdate(
				context.Background(), tc.id, tc.handle, "ou-1", testOwnerErrors)

			if tc.wantErr != nil {
				suite.Require().NotNil(svcErr)
				suite.Equal(tc.wantErr.Code, svcErr.Code)
				return
			}
			suite.Require().Nil(svcErr)
			suite.Equal(tc.wantOwner, owner)
		})
	}
}
