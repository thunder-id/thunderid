// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package capability

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

type ServiceTestSuite struct {
	suite.Suite
	authz *sysauthzmock.SystemAuthorizationServiceInterfaceMock
}

func TestServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceTestSuite))
}

func (s *ServiceTestSuite) SetupTest() {
	security.InitControlPlanePermissions("")
	s.authz = sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
}

// allow makes the mock allow exactly the given actions, with or without a record.
func (s *ServiceTestSuite) allow(actions ...security.Action) {
	allowed := make(map[security.Action]bool, len(actions))
	for _, action := range actions {
		allowed[action] = true
	}
	s.authz.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
		Return(func(_ context.Context, action security.Action, _ *sysauthz.ActionContext) bool {
			return allowed[action]
		}, (*tidcommon.ServiceError)(nil))
}

// A unit created with no parent is reported apart from one created beneath a parent, since a plane may accept
// it only through import.
func (s *ServiceTestSuite) TestReportsWhetherARootUnitIsCreated() {
	rootCreation := &sysauthz.ActionContext{ResourceType: security.ResourceTypeOU}
	s.authz.On("IsActionAllowed", mock.Anything, security.ActionCreateOU, rootCreation).Return(false, nil).Once()
	s.allow(security.ActionReadOU, security.ActionCreateOU)
	s.authz.On("GetBoundary", mock.Anything).Return(sysauthz.Boundary{Type: sysauthz.BoundaryTypeOU, OUID: "ou-1"})

	resp, svcErr := newCapabilityService(s.authz).GetCapabilities(context.Background())

	s.Require().Nil(svcErr)
	s.True(resp.Resources[security.ResourceTypeOU]["create"])
	s.False(resp.Resources[security.ResourceTypeOU]["createRoot"])
}

func (s *ServiceTestSuite) TestReportsWhatTheBoundaryAllows() {
	s.allow(security.ActionReadApplication, security.ActionUpdateApplication, security.ActionReadProject)
	s.authz.On("GetBoundary", mock.Anything).Return(sysauthz.Boundary{Type: sysauthz.BoundaryTypeOU, OUID: "ou-1"})

	resp, svcErr := newCapabilityService(s.authz).GetCapabilities(context.Background())

	s.Require().Nil(svcErr)
	s.Equal(capabilityScope{Type: sysauthz.BoundaryTypeOU, OUID: "ou-1"}, resp.Scope)
	s.Equal(map[string]bool{"view": true, "create": false, "update": true, "delete": false},
		resp.Resources[security.ResourceTypeApplication])
	s.True(resp.Resources[security.ResourceTypeProject]["view"])
	s.Equal(map[string]bool{"view": false, "update": false}, resp.Resources[security.ResourceTypeServerConfig])
	s.Equal(rootCapabilities{}, resp.Root)
}

func (s *ServiceTestSuite) TestAnActionNeedsEveryDependency() {
	s.allow(security.ActionReadApplication, security.ActionCreateApplication, security.ActionReadOU,
		security.ActionReadFlow, security.ActionCreateFlow, security.ActionReadGroup, security.ActionCreateGroup)
	s.authz.On("GetBoundary", mock.Anything).Return(sysauthz.Boundary{Type: sysauthz.BoundaryTypeDeployment})

	resp, svcErr := newCapabilityService(s.authz).GetCapabilities(context.Background())

	s.Require().Nil(svcErr)
	s.False(resp.Resources[security.ResourceTypeApplication]["create"], "connections can't be listed")
	s.True(resp.Resources[security.ResourceTypeGroup]["create"])
}

func (s *ServiceTestSuite) TestAnActionNeedsWhatItsResourceViewNeeds() {
	s.allow(security.ActionReadUser, security.ActionUpdateUser, security.ActionCreateGroup)
	s.authz.On("GetBoundary", mock.Anything).Return(sysauthz.Boundary{Type: sysauthz.BoundaryTypeDeployment})

	resp, svcErr := newCapabilityService(s.authz).GetCapabilities(context.Background())

	s.Require().Nil(svcErr)
	s.False(resp.Resources[security.ResourceTypeUser]["update"], "user types can't be read")
	s.False(resp.Resources[security.ResourceTypeGroup]["create"], "groups can't be read")
}

func (s *ServiceTestSuite) TestEvaluatesEachActionOnce() {
	s.allow(security.ActionReadOU)
	s.authz.On("GetBoundary", mock.Anything).Return(sysauthz.Boundary{Type: sysauthz.BoundaryTypeDeployment})

	_, svcErr := newCapabilityService(s.authz).GetCapabilities(context.Background())

	s.Require().Nil(svcErr)
	evaluated := make(map[security.Action]int)
	for _, call := range s.authz.Calls {
		if call.Method == "IsActionAllowed" {
			evaluated[call.Arguments.Get(1).(security.Action)]++
		}
	}
	s.Equal(1, evaluated[security.ActionReadOU])
}

func (s *ServiceTestSuite) TestRootComesFromTheSystemPermission() {
	s.allow()
	s.authz.On("GetBoundary", mock.Anything).Return(sysauthz.Boundary{Type: sysauthz.BoundaryTypeDeployment})
	ctx := security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest("admin", "ou-1", "token", []string{"system"}, nil))

	resp, svcErr := newCapabilityService(s.authz).GetCapabilities(ctx)

	s.Require().Nil(svcErr)
	s.Equal(rootCapabilities{Manage: true}, resp.Root)
	s.Equal(capabilityScope{Type: sysauthz.BoundaryTypeDeployment}, resp.Scope)
}

func (s *ServiceTestSuite) TestReturnsTheAuthorizationError() {
	s.authz.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
		Return(false, &tidcommon.InternalServerError).Once()

	resp, svcErr := newCapabilityService(s.authz).GetCapabilities(context.Background())

	s.Nil(resp)
	s.Equal(&tidcommon.InternalServerError, svcErr)
}
