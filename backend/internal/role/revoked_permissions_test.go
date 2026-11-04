// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/resourcemock"
)

type RevokedPermissionsTestSuite struct {
	suite.Suite
	resources *resourcemock.ResourceServiceInterfaceMock
}

func TestRevokedPermissionsTestSuite(t *testing.T) {
	suite.Run(t, new(RevokedPermissionsTestSuite))
}

func (s *RevokedPermissionsTestSuite) SetupTest() {
	s.resources = resourcemock.NewResourceServiceInterfaceMock(s.T())
}

// stubMatching declares a resource server whose permissions do not nest.
func (s *RevokedPermissionsTestSuite) stubExact(id string) {
	s.resources.On("GetResourceServer", mock.Anything, id).
		Return(&providers.ResourceServer{ID: id, Delimiter: ":"}, nil)
}

func (s *RevokedPermissionsTestSuite) TestExactServerRevokesOnlyWhatWasDropped() {
	s.stubExact(californiaRSID)

	revoked, svcErr := revokedPermissions(context.Background(), s.resources,
		[]ResourcePermissions{{ResourceServerID: californiaRSID, Permissions: []string{"license", "permits"}}},
		[]ResourcePermissions{{ResourceServerID: californiaRSID, Permissions: []string{"permits"}}})

	s.Require().Nil(svcErr)
	s.Equal([]ResourcePermissions{
		{ResourceServerID: californiaRSID, Permissions: []string{"license"}},
	}, revoked)
}

func (s *RevokedPermissionsTestSuite) TestResourceServerLookupFailureStopsRevocation() {
	s.resources.On("GetResourceServer", mock.Anything, californiaRSID).
		Return((*providers.ResourceServer)(nil), &tidcommon.InternalServerError).Once()

	revoked, svcErr := revokedPermissions(context.Background(), s.resources,
		[]ResourcePermissions{{ResourceServerID: californiaRSID, Permissions: []string{"license"}}}, nil)

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	s.Nil(revoked)
	s.resources.AssertNotCalled(s.T(), "GetAllResourceList", mock.Anything, mock.Anything)
}

func (s *RevokedPermissionsTestSuite) TestMissingResourceServerStopsRevocation() {
	s.resources.On("GetResourceServer", mock.Anything, californiaRSID).
		Return((*providers.ResourceServer)(nil), nil).Once()

	revoked, svcErr := revokedPermissions(context.Background(), s.resources,
		[]ResourcePermissions{{ResourceServerID: californiaRSID, Permissions: []string{"license"}}}, nil)

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	s.Nil(revoked)
}

func (s *RevokedPermissionsTestSuite) TestLookupFailureDoesNotReturnPartialRevocations() {
	s.stubExact(californiaRSID)
	s.resources.On("GetResourceServer", mock.Anything, ohioRSID).
		Return((*providers.ResourceServer)(nil), &tidcommon.InternalServerError).Once()

	revoked, svcErr := revokedPermissions(context.Background(), s.resources,
		[]ResourcePermissions{
			{ResourceServerID: californiaRSID, Permissions: []string{"license"}},
			{ResourceServerID: ohioRSID, Permissions: []string{"license"}},
		}, nil)

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	s.Nil(revoked)
}

func (s *RevokedPermissionsTestSuite) TestNothingDroppedRevokesNothing() {
	s.stubExact(californiaRSID)

	revoked, svcErr := revokedPermissions(context.Background(), s.resources,
		[]ResourcePermissions{{ResourceServerID: californiaRSID, Permissions: []string{"license"}}},
		[]ResourcePermissions{{ResourceServerID: californiaRSID, Permissions: []string{"license", "permits"}}})

	s.Require().Nil(svcErr)
	s.Empty(revoked)
}

func (s *RevokedPermissionsTestSuite) TestDroppedParentRevokesItsSubtree() {
	s.resources.On("GetResourceServer", mock.Anything, californiaRSID).
		Return(&providers.ResourceServer{ID: californiaRSID, Delimiter: ":", HierarchicalPermissions: true}, nil)
	revoked, svcErr := revokedPermissions(context.Background(), s.resources,
		[]ResourcePermissions{{ResourceServerID: californiaRSID, Permissions: []string{"license"}}}, nil)
	s.Require().Nil(svcErr)
	s.Require().Len(revoked, 1)
	s.Equal([]string{"license"}, revoked[0].Permissions)
	s.Equal(":", revoked[0].subtreeDelimiter)
	s.resources.AssertNotCalled(s.T(), "GetAllResourceList", mock.Anything, mock.Anything)
}

// Dropping a child while the role still grants its parent takes nothing away, because the parent
// still confers it. Revoking it would cut off access the role still means to give.
func (s *RevokedPermissionsTestSuite) TestDroppedChildCoveredByKeptParentRevokesNothing() {
	s.resources.On("GetResourceServer", mock.Anything, californiaRSID).
		Return(&providers.ResourceServer{ID: californiaRSID, Delimiter: ":", HierarchicalPermissions: true}, nil)

	revoked, svcErr := revokedPermissions(context.Background(), s.resources,
		[]ResourcePermissions{{ResourceServerID: californiaRSID, Permissions: []string{"license", "license:create"}}},
		[]ResourcePermissions{{ResourceServerID: californiaRSID, Permissions: []string{"license"}}})

	s.Require().Nil(svcErr)
	s.Empty(revoked)
}

func (s *RevokedPermissionsTestSuite) TestSubtreeRevocationIncludesRetainedBranches() {
	s.resources.On("GetResourceServer", mock.Anything, californiaRSID).
		Return(&providers.ResourceServer{ID: californiaRSID, Delimiter: ":", HierarchicalPermissions: true}, nil)
	revoked, svcErr := revokedPermissions(context.Background(), s.resources,
		[]ResourcePermissions{{ResourceServerID: californiaRSID, Permissions: []string{"license"}}},
		[]ResourcePermissions{{ResourceServerID: californiaRSID, Permissions: []string{"license:view"}}})
	s.Require().Nil(svcErr)
	s.Equal([]ResourcePermissions{{ResourceServerID: californiaRSID,
		Permissions: []string{"license"}, subtreeDelimiter: ":"}}, revoked)
	s.resources.AssertNotCalled(s.T(), "GetAllResourceList", mock.Anything, mock.Anything)
}
