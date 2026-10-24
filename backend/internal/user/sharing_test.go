// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// UserSharingTestSuite covers how a user participates in sharing: what it declares to the framework,
// and how the policies its file declares are parsed.
type UserSharingTestSuite struct {
	suite.Suite
}

func TestUserSharingTestSuite(t *testing.T) {
	suite.Run(t, new(UserSharingTestSuite))
}

// A user declares no field yet, so a policy governs reach only.
func (s *UserSharingTestSuite) TestNoFieldsAreDeclared() {
	decl := newUserSharingDeclaration(NewUserServiceInterfaceMock(s.T()))

	s.Equal(UserSharingType, decl.ResourceType())
	s.Empty(decl.Fields(), "a user shares no per-organization-unit state yet")
}

// Ownership is read through the runtime context. The framework asks in order to decide access, so an
// access-checked read here would recurse back through the framework.
func (s *UserSharingTestSuite) TestOwnerIsResolvedWithoutAnAccessCheck() {
	userService := NewUserServiceInterfaceMock(s.T())
	userService.On("GetUser", mock.MatchedBy(security.IsRuntimeContext), svcTestUserID1, false).
		Return(&providers.User{ID: svcTestUserID1, OUID: shareOwnerOU}, (*tidcommon.ServiceError)(nil)).Once()

	owner, svcErr := newUserSharingDeclaration(userService).OwningOUID(context.Background(), svcTestUserID1)

	s.Require().Nil(svcErr)
	s.Equal(shareOwnerOU, owner)
}

// A failure to read the user is reported rather than answered as "no owner", which the framework
// would read as a resource it does not recognize.
func (s *UserSharingTestSuite) TestOwnerResolutionCarriesTheServiceError() {
	userService := NewUserServiceInterfaceMock(s.T())
	userService.On("GetUser", mock.Anything, svcTestUserID1, false).
		Return((*providers.User)(nil), &tidcommon.InternalServerError).Once()

	owner, svcErr := newUserSharingDeclaration(userService).OwningOUID(context.Background(), svcTestUserID1)

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	s.Empty(owner)
}

// The policies have to survive the parse, because a declared user's own sharing has no other way in.
func (s *UserSharingTestSuite) TestDeclaredPoliciesSurviveYAMLParsing() {
	doc := []byte(`
id: ` + svcTestUserID1 + `
type: person
ouId: ` + shareOwnerOU + `
attributes:
  username: shared-user
sharingPolicies:
  - id: shared-user-policy
    otherTreeRoots:
      mode: selected
      units:
        - ouId: partner-root
`)

	var declared userDeclarativeResource
	s.Require().NoError(yaml.Unmarshal(doc, &declared))

	s.Require().Len(declared.SharingPolicies, 1)
	policy := declared.SharingPolicies[0]
	s.Equal("shared-user-policy", policy.ID)
	s.Require().NotNil(policy.OtherTreeRoots)
	s.Require().Len(policy.OtherTreeRoots.Units, 1)
	s.Equal("partner-root", policy.OtherTreeRoots.Units[0].OUID)
	s.Nil(policy.AllUnits)
	s.Nil(policy.ChildUnits)
}
