// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/entitytype"
	entitytypemodel "github.com/thunder-id/thunderid/internal/entitytype/model"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/groupmock"
	"github.com/thunder-id/thunderid/tests/mocks/rolemock"
)

type FederatedSyncTestSuite struct {
	suite.Suite
	entityProvider    *entityprovidermock.EntityProviderInterfaceMock
	entityTypeService *entitytypemock.EntityTypeServiceInterfaceMock
	groupService      *groupmock.GroupServiceInterfaceMock
	roleAssignments   *rolemock.RoleAssignmentServiceInterfaceMock
	authnProvider     *managermock.AuthnProviderManagerMock
	syncer            *federatedSyncer
	logger            *log.Logger
}

func TestFederatedSyncTestSuite(t *testing.T) {
	suite.Run(t, new(FederatedSyncTestSuite))
}

func (s *FederatedSyncTestSuite) SetupTest() {
	s.entityProvider = entityprovidermock.NewEntityProviderInterfaceMock(s.T())
	s.entityTypeService = entitytypemock.NewEntityTypeServiceInterfaceMock(s.T())
	s.groupService = groupmock.NewGroupServiceInterfaceMock(s.T())
	s.roleAssignments = rolemock.NewRoleAssignmentServiceInterfaceMock(s.T())
	s.authnProvider = managermock.NewAuthnProviderManagerMock(s.T())
	s.syncer = newFederatedSyncer(ExecutorDependencies{
		EntityProvider: s.entityProvider, EntityTypeService: s.entityTypeService, GroupService: s.groupService,
		RoleAssignmentService: s.roleAssignments, AuthnProvider: s.authnProvider,
	})
	s.logger = log.GetLogger()
	initSyncTestRuntime(s.T(), nil)
}

// initSyncTestRuntime sets the server runtime the sync reads the group and role store modes from, with
// the default configuration when cfg is nil.
func initSyncTestRuntime(t *testing.T, cfg *config.Config) {
	if cfg == nil {
		cfg = &config.Config{}
	}
	config.ResetServerRuntime()
	if err := config.InitializeServerRuntime("", cfg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(config.ResetServerRuntime)
}

func syncTestIDP(attributes, groupsAndRoles providers.SyncMode) *providers.IDPDTO {
	return &providers.IDPDTO{ID: "idp-1", AttributeConfiguration: &providers.AttributeConfiguration{
		Sync: &providers.SyncConfiguration{Attributes: attributes, GroupsAndRoles: groupsAndRoles},
		UserTypeAttributeMappings: []providers.UserTypeAttributeMapping{{
			UserType: "Person",
			Attributes: []providers.AttributeMapping{
				{ExternalAttribute: "mail", LocalAttribute: "email"},
				{ExternalAttribute: "given_name", LocalAttribute: "firstName"},
				{ExternalAttribute: "id", LocalAttribute: "sub"},
				{ExternalAttribute: "phone", LocalAttribute: "mobile"},
			},
		}, {
			UserType: "Contractor",
			Attributes: []providers.AttributeMapping{
				{ExternalAttribute: "contractor_mail", LocalAttribute: "email"},
				{ExternalAttribute: "mail", LocalAttribute: "workEmail"},
			},
		}, {
			UserType:   "Guest",
			Attributes: []providers.AttributeMapping{{ExternalAttribute: "phone", LocalAttribute: "mobile"}},
		}},
	}}
}

var syncTestReceived = map[string]interface{}{
	"mail": "new@example.com", "email": "new@example.com", "given_name": "Fed", "firstName": "Fed",
	"id": "fed-sub", "sub": "fed-sub", "picture": "x", "contractor_mail": "contractor@example.com",
}

var syncTestLink = providers.LinkedAccount{IdpID: "idp-1", Sub: "fed-sub"}

// Neither mode overriding the account means nothing is synced.
func (s *FederatedSyncTestSuite) TestBuildFederatedSync_NoOverride() {
	for _, mode := range []providers.SyncMode{providers.SyncModeNoSync, providers.SyncModeSessionOnly} {
		s.Nil(buildFederatedSync(syncTestIDP(mode, mode), &idp.ResolvedAuthorizationMapping{Complete: true},
			syncTestReceived, "fed-sub"))
	}
}

func (s *FederatedSyncTestSuite) TestBuildFederatedSync_Attributes() {
	sync := buildFederatedSync(syncTestIDP(providers.SyncModeOverrideReceived, providers.SyncModeNoSync),
		&idp.ResolvedAuthorizationMapping{Complete: true}, syncTestReceived, "fed-sub")

	s.Require().NotNil(sync)
	s.Equal(syncTestLink, providers.LinkedAccount{IdpID: sync.IdpID, Sub: sync.Sub})
	s.Equal(map[string]map[string]interface{}{
		"Person":     {"email": "new@example.com", "firstName": "Fed"},
		"Contractor": {"email": "contractor@example.com", "workEmail": "new@example.com"},
	}, sync.AttributesByType)
	s.Nil(sync.Groups)
	s.Nil(sync.Roles)
}

// Groups and roles alone still sync, without writing any attribute.
func (s *FederatedSyncTestSuite) TestBuildFederatedSync_GroupsAndRolesWithoutAttributes() {
	sync := buildFederatedSync(syncTestIDP(providers.SyncModeNoSync, providers.SyncModeOverrideAll),
		&idp.ResolvedAuthorizationMapping{Complete: true}, syncTestReceived, "fed-sub")

	s.Require().NotNil(sync)
	s.Equal(syncTestLink, providers.LinkedAccount{IdpID: sync.IdpID, Sub: sync.Sub})
	s.Nil(sync.AttributesByType)
	s.Nil(sync.Groups)
	s.Nil(sync.Roles)
}

func (s *FederatedSyncTestSuite) TestBuildFederatedSync_GroupsAndRoles() {
	targets := []providers.AuthorizationTarget{
		{Type: providers.AuthorizationTargetGroup, ID: "g1"},
		{Type: providers.AuthorizationTargetRole, ID: "r1"},
		{Type: providers.AuthorizationTargetPermission, Permission: "read"},
	}

	tests := []struct {
		name                 string
		mode                 providers.SyncMode
		mapsGroups, mapsRole bool
		targets              []providers.AuthorizationTarget
		groups, roles        *membershipSync
	}{
		{"override all sets both", providers.SyncModeOverrideAll, true, true, targets,
			&membershipSync{IDs: []string{"g1"}}, &membershipSync{IDs: []string{"r1"}}},
		{"override all with nothing resolved clears both", providers.SyncModeOverrideAll, true, true, nil,
			&membershipSync{}, &membershipSync{}},
		{"override all leaves roles when the mapping can't resolve roles", providers.SyncModeOverrideAll,
			true, false, targets, &membershipSync{IDs: []string{"g1"}}, nil},
		{"no sync leaves both", providers.SyncModeNoSync, true, true, targets, nil, nil},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			sync := buildFederatedSync(syncTestIDP(providers.SyncModeOverrideReceived, tc.mode),
				&idp.ResolvedAuthorizationMapping{
					MapsGroups: tc.mapsGroups, MapsRoles: tc.mapsRole, Targets: tc.targets, Complete: true,
				}, syncTestReceived, "fed-sub")
			s.Require().NotNil(sync)
			s.Equal(tc.groups, sync.Groups)
			s.Equal(tc.roles, sync.Roles)
			s.False(sync.MappingIncomplete)
		})
	}
}

func (s *FederatedSyncTestSuite) TestBuildFederatedSync_KeepsAmbiguousMatches() {
	sync := buildFederatedSync(syncTestIDP(providers.SyncModeNoSync, providers.SyncModeOverrideAll),
		&idp.ResolvedAuthorizationMapping{
			MapsGroups: true, Complete: true,
			Targets: []providers.AuthorizationTarget{{Type: providers.AuthorizationTargetGroup, ID: "g1"}},
			Ambiguous: []providers.AuthorizationTarget{
				{Type: providers.AuthorizationTargetGroup, ID: "g2"},
				{Type: providers.AuthorizationTargetGroup, ID: "g3"},
				{Type: providers.AuthorizationTargetRole, ID: "r2"},
			},
		}, syncTestReceived, "fed-sub")

	s.Require().NotNil(sync)
	s.Equal(&membershipSync{IDs: []string{"g1"}, RetainIDs: []string{"g2", "g3"}}, sync.Groups)
	s.Nil(sync.Roles)
}

func (s *FederatedSyncTestSuite) TestBuildFederatedSync_IncompleteMapping() {
	sync := buildFederatedSync(syncTestIDP(providers.SyncModeNoSync, providers.SyncModeOverrideAll),
		&idp.ResolvedAuthorizationMapping{MapsGroups: true}, syncTestReceived, "fed-sub")

	s.Require().NotNil(sync)
	s.True(sync.MappingIncomplete)
	s.Nil(sync.Groups)
}

func syncTestContext(pending string) *providers.NodeContext {
	return &providers.NodeContext{Context: context.Background(),
		RuntimeData: map[string]string{common.RuntimeKeyPendingFederatedSync: pending}}
}

func syncTestResponse() *providers.ExecutorResponse {
	return &providers.ExecutorResponse{RuntimeData: map[string]string{}}
}

func encodedSync(s *FederatedSyncTestSuite, sync federatedSync) string {
	encoded, err := json.Marshal(sync)
	s.Require().NoError(err)
	return string(encoded)
}

// syncTestUser is the stored account of user-1, a Person holding the link idp-1, fed-sub.
func syncTestUser() *providers.Entity {
	return &providers.Entity{ID: "user-1", Category: providers.EntityCategoryUser, Type: "Person", OUID: "ou-1",
		Attributes: json.RawMessage(`{"email":"old@example.com","age":30}`)}
}

var syncTestMember = group.Member{ID: "user-1", Type: group.MemberTypeUser}

var syncTestAssignee = role.RoleAssignment{ID: "user-1", Type: role.AssigneeTypeUser}

// expectHolder makes user the only holder of the link idp-1, fed-sub.
func (s *FederatedSyncTestSuite) expectHolder(user *providers.Entity) {
	s.entityProvider.On("GetLinkedEntities", mock.Anything, "idp-1", "fed-sub").
		Return([]*providers.Entity{{ID: "other-user"}, user}, nil).Once()
}

// expectSchema returns the Person type's attributes: email and age, with nickname as an array.
func (s *FederatedSyncTestSuite) expectSchema() {
	s.entityTypeService.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, "Person",
		entitytype.AttributeFilter{AllowNonCredential: true}).Return([]entitytype.AttributeInfo{
		{Attribute: "email", Type: entitytypemodel.TypeString},
		{Attribute: "age", Type: entitytypemodel.TypeNumber},
		{Attribute: "nickname", Type: entitytypemodel.TypeArray},
	}, nil).Once()
}

// expectUpdate expects the user's attributes to be stored as want, answered with epErr.
func (s *FederatedSyncTestSuite) expectUpdate(want string, epErr *entityprovider.EntityProviderError) {
	s.entityProvider.On("UpdateAttributes", mock.Anything, "user-1", mock.MatchedBy(func(got json.RawMessage) bool {
		return jsonEqual(json.RawMessage(want), got)
	})).Return(epErr).Once()
}

// expectSignIn expects the user to be signed in again by ID, answered with svcErr.
func (s *FederatedSyncTestSuite) expectSignIn(svcErr *tidcommon.ServiceError) providers.AuthUser {
	signedIn := providers.AuthUser{}
	signedIn.SetStateFor("default", providers.AuthState{
		EntityReference: &providers.EntityReference{EntityID: "user-1"},
		Attributes:      &providers.AttributesResponse{},
	})
	s.authnProvider.On("AuthenticateUser", mock.Anything, mock.Anything,
		map[string]interface{}{authnprovidercm.CredentialTypeProvisionedEntityID: "user-1"},
		mock.Anything, mock.Anything, providers.AuthUser{}).
		Return(signedIn, providers.AuthenticatedClaims{userAttributeUserID: "user-1"}, svcErr).Once()
	return signedIn
}

func (s *FederatedSyncTestSuite) syncUser(sync federatedSync, execResp *providers.ExecutorResponse) (bool, error) {
	return s.syncer.syncPendingUser(syncTestContext(encodedSync(s, sync)), execResp, "user-1", s.logger)
}

func attributeSync(received map[string]interface{}) federatedSync {
	return federatedSync{IdpID: "idp-1", Sub: "fed-sub",
		AttributesByType: map[string]map[string]interface{}{
			"Person": received, "Guest": {"email": "guest@example.com"},
		}}
}

// Only the changed values of the user's own type are written, on top of the stored profile. The pending
// sync is then cleared so a later node doesn't write it again, and the user is signed in again, since
// the AuthUser loaded the attributes before the write.
func (s *FederatedSyncTestSuite) TestSyncPendingUser_WritesChangedAttributes() {
	s.expectHolder(syncTestUser())
	s.expectSchema()
	s.expectUpdate(`{"email":"new@example.com","age":30,"nickname":["Fed"]}`, nil)
	signedIn := s.expectSignIn(nil)
	execResp := syncTestResponse()

	ok, err := s.syncUser(attributeSync(map[string]interface{}{
		"email": "new@example.com", "age": "30", "nickname": "Fed", "unmapped": "x", "sub": "",
	}), execResp)

	s.True(ok)
	s.NoError(err)
	s.Empty(execResp.Status)
	s.Equal(signedIn, execResp.AuthUser)
	s.Equal("", execResp.RuntimeData[common.RuntimeKeyPendingFederatedSync])
	s.Contains(execResp.RuntimeData, common.RuntimeKeyPendingFederatedSync)
}

// A sync that changes nothing writes nothing and leaves the AuthUser as it is.
func (s *FederatedSyncTestSuite) TestSyncPendingUser_UnchangedAttributesWriteNothing() {
	s.expectHolder(syncTestUser())
	s.expectSchema()
	execResp := syncTestResponse()

	ok, err := s.syncUser(attributeSync(map[string]interface{}{
		"email": "old@example.com", "age": 30, "nickname": map[string]interface{}{"a": 1},
	}), execResp)

	s.True(ok)
	s.NoError(err)
	s.Equal("", execResp.RuntimeData[common.RuntimeKeyPendingFederatedSync])
	s.entityProvider.AssertNotCalled(s.T(), "UpdateAttributes", mock.Anything, mock.Anything, mock.Anything)
	s.authnProvider.AssertNotCalled(s.T(), "AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything)
}

func (s *FederatedSyncTestSuite) TestSyncPendingUser_AttributesLeftAlone() {
	readOnly := syncTestUser()
	readOnly.IsReadOnly = true
	otherType := syncTestUser()
	otherType.Type = "Contractor"

	for name, user := range map[string]*providers.Entity{"read-only user": readOnly, "unmapped user type": otherType} {
		s.Run(name, func() {
			s.expectHolder(user)

			ok, err := s.syncUser(attributeSync(map[string]interface{}{"email": "new@example.com"}), syncTestResponse())

			s.True(ok)
			s.NoError(err)
			s.entityProvider.AssertNotCalled(s.T(), "UpdateAttributes", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

// A value the account can't take is left out, and the rest are written one at a time.
func (s *FederatedSyncTestSuite) TestSyncPendingUser_RefusedAttributeIsSkipped() {
	for _, code := range []entityprovider.ErrorCode{
		entityprovider.ErrorCodeAttributeConflict, entityprovider.ErrorCodeSchemaValidationFailed,
	} {
		s.Run(string(code), func() {
			refused := entityprovider.NewEntityProviderError(code, "refused", "")
			s.expectHolder(syncTestUser())
			s.expectSchema()
			s.expectUpdate(`{"email":"taken@example.com","age":31}`, refused)
			s.expectUpdate(`{"email":"old@example.com","age":31}`, nil)
			s.expectUpdate(`{"email":"taken@example.com","age":31}`, refused)
			s.expectSignIn(nil)

			ok, err := s.syncUser(attributeSync(map[string]interface{}{"email": "taken@example.com", "age": 31}),
				syncTestResponse())

			s.True(ok)
			s.NoError(err)
		})
	}
}

func (s *FederatedSyncTestSuite) TestSyncPendingUser_AllAttributesRefused() {
	refused := entityprovider.NewEntityProviderError(entityprovider.ErrorCodeAttributeConflict, "refused", "")
	s.expectHolder(syncTestUser())
	s.expectSchema()
	s.expectUpdate(`{"email":"taken@example.com","age":30}`, refused)
	s.expectUpdate(`{"email":"taken@example.com","age":30}`, refused)

	ok, err := s.syncUser(attributeSync(map[string]interface{}{"email": "taken@example.com"}), syncTestResponse())

	s.True(ok)
	s.NoError(err)
	s.authnProvider.AssertNotCalled(s.T(), "AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything)
}

func (s *FederatedSyncTestSuite) TestSyncPendingUser_AttributeServerErrors() {
	s.Run("schema lookup fails", func() {
		s.expectHolder(syncTestUser())
		s.entityTypeService.On("GetAttributes", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil, &tidcommon.InternalServerError).Once()

		ok, err := s.syncUser(attributeSync(map[string]interface{}{"email": "new@example.com"}), syncTestResponse())

		s.False(ok)
		s.Error(err)
	})

	s.Run("stored attributes are malformed", func() {
		user := syncTestUser()
		user.Attributes = json.RawMessage(`{`)
		s.expectHolder(user)
		s.expectSchema()

		ok, err := s.syncUser(attributeSync(map[string]interface{}{"email": "new@example.com"}), syncTestResponse())

		s.False(ok)
		s.Error(err)
	})

	s.Run("write fails", func() {
		s.expectHolder(syncTestUser())
		s.expectSchema()
		s.expectUpdate(`{"email":"new@example.com","age":30}`,
			entityprovider.NewEntityProviderError(entityprovider.ErrorCodeSystemError, "failed", ""))
		execResp := syncTestResponse()

		ok, err := s.syncUser(attributeSync(map[string]interface{}{"email": "new@example.com"}), execResp)

		s.False(ok)
		s.Error(err)
		s.NotContains(execResp.RuntimeData, common.RuntimeKeyPendingFederatedSync)
	})
}

func (s *FederatedSyncTestSuite) TestSyncPendingUser_SignInAfterWrite() {
	tests := []struct {
		name   string
		svcErr *tidcommon.ServiceError
		server bool
	}{
		{"server error", &tidcommon.InternalServerError, true},
		{"refused sign-in fails the flow", syncRefused, false},
	}
	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.expectHolder(syncTestUser())
			s.expectSchema()
			s.expectUpdate(`{"email":"new@example.com","age":30}`, nil)
			s.expectSignIn(tc.svcErr)
			execResp := syncTestResponse()

			ok, err := s.syncUser(attributeSync(map[string]interface{}{"email": "new@example.com"}), execResp)

			s.False(ok)
			s.Equal(tc.server, err != nil)
			if !tc.server {
				s.Equal(providers.ExecFailure, execResp.Status)
				s.Equal(&ErrFederatedUserSyncFailed, execResp.Error)
			}
		})
	}
}

// The mapped groups are added, the rest are removed, and a retained group or one the sync can't remove
// is kept.
func (s *FederatedSyncTestSuite) TestSyncPendingUser_ReplacesGroups() {
	s.expectHolder(syncTestUser())
	s.groupService.On("GetMemberGroupIDs", mock.Anything, syncTestMember).
		Return([]string{"g1", "g-stale", "g-retained", "g-managed", "g-declared", "g-gone"}, nil).Once()
	s.groupService.On("AddMemberToExistingGroups", mock.Anything, syncTestMember, []string{"g2"}).Return(nil).Once()
	s.groupService.On("RemoveGroupMembers", mock.Anything, "g-stale", []group.Member{syncTestMember}).
		Return(nil, nil).Once()
	for id, kept := range map[string]*tidcommon.ServiceError{
		"g-managed": &group.ErrorManagedGroupMember, "g-declared": &group.ErrorImmutableGroup,
		"g-gone": &group.ErrorGroupNotFound,
	} {
		s.groupService.On("RemoveGroupMembers", mock.Anything, id, []group.Member{syncTestMember}).
			Return(nil, kept).Once()
	}

	ok, err := s.syncUser(federatedSync{IdpID: "idp-1", Sub: "fed-sub",
		Groups: &membershipSync{IDs: []string{"g1", "g2", "g2"}, RetainIDs: []string{"g-retained"}}},
		syncTestResponse())

	s.True(ok)
	s.NoError(err)
}

// The mapped roles are assigned in the user's organization unit, the rest are removed, and a retained
// role or one the sync can't remove is kept.
func (s *FederatedSyncTestSuite) TestSyncPendingUser_ReplacesRoles() {
	s.expectHolder(syncTestUser())
	s.roleAssignments.On("GetDirectRoleIDs", mock.Anything, "user-1", "ou-1").
		Return([]string{"r1", "r-stale", "r-retained", "r-mandated", "r-managed", "r-unshared", "r-gone"}, nil).Once()
	s.roleAssignments.On("AddAssigneeToExistingRoles", mock.Anything, syncTestAssignee, []string{"r2"}).
		Return(nil).Once()
	s.roleAssignments.On("RemoveAssignments", mock.Anything, "r-stale", "ou-1",
		[]role.RoleAssignment{syncTestAssignee}).Return(nil).Once()
	for id, kept := range map[string]*tidcommon.ServiceError{
		"r-mandated": &role.ErrorAssignmentMandated, "r-managed": &role.ErrorManagedRoleAssignment,
		"r-unshared": &role.ErrorRoleNotSharedToOU, "r-gone": &role.ErrorRoleNotFound,
	} {
		s.roleAssignments.On("RemoveAssignments", mock.Anything, id, "ou-1",
			[]role.RoleAssignment{syncTestAssignee}).Return(kept).Once()
	}

	ok, err := s.syncUser(federatedSync{IdpID: "idp-1", Sub: "fed-sub",
		Roles: &membershipSync{IDs: []string{"r1", "r2"}, RetainIDs: []string{"r-retained"}}}, syncTestResponse())

	s.True(ok)
	s.NoError(err)
}

// Declarative-only stores hold no writable memberships, so nothing is read or written.
func (s *FederatedSyncTestSuite) TestSyncPendingUser_DeclarativeOnlyMembershipsAreLeftAlone() {
	initSyncTestRuntime(s.T(), &config.Config{
		Group: config.GroupConfig{Store: "declarative"}, Role: config.RoleConfig{Store: "declarative"},
	})
	s.expectHolder(syncTestUser())

	ok, err := s.syncUser(federatedSync{IdpID: "idp-1", Sub: "fed-sub",
		Groups: &membershipSync{IDs: []string{"g1"}}, Roles: &membershipSync{IDs: []string{"r1"}}},
		syncTestResponse())

	s.True(ok)
	s.NoError(err)
}

func (s *FederatedSyncTestSuite) TestSyncPendingUser_MembershipErrors() {
	refused := &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "X-1"}
	both := federatedSync{IdpID: "idp-1", Sub: "fed-sub",
		Groups: &membershipSync{IDs: []string{"g1"}}, Roles: &membershipSync{IDs: []string{"r1"}}}
	tests := []struct {
		name   string
		expect func(svcErr *tidcommon.ServiceError)
	}{
		{"reading groups", func(svcErr *tidcommon.ServiceError) {
			s.groupService.On("GetMemberGroupIDs", mock.Anything, syncTestMember).Return(nil, svcErr).Once()
		}},
		{"adding groups", func(svcErr *tidcommon.ServiceError) {
			s.groupService.On("GetMemberGroupIDs", mock.Anything, syncTestMember).Return(nil, nil).Once()
			s.groupService.On("AddMemberToExistingGroups", mock.Anything, syncTestMember, []string{"g1"}).
				Return(svcErr).Once()
		}},
		{"removing a group", func(svcErr *tidcommon.ServiceError) {
			s.groupService.On("GetMemberGroupIDs", mock.Anything, syncTestMember).
				Return([]string{"g1", "g-stale"}, nil).Once()
			s.groupService.On("RemoveGroupMembers", mock.Anything, "g-stale", []group.Member{syncTestMember}).
				Return(nil, svcErr).Once()
		}},
		{"reading roles", func(svcErr *tidcommon.ServiceError) {
			s.expectGroupsInSync()
			s.roleAssignments.On("GetDirectRoleIDs", mock.Anything, "user-1", "ou-1").Return(nil, svcErr).Once()
		}},
		{"assigning roles", func(svcErr *tidcommon.ServiceError) {
			s.expectGroupsInSync()
			s.roleAssignments.On("GetDirectRoleIDs", mock.Anything, "user-1", "ou-1").Return(nil, nil).Once()
			s.roleAssignments.On("AddAssigneeToExistingRoles", mock.Anything, syncTestAssignee, []string{"r1"}).
				Return(svcErr).Once()
		}},
		{"removing a role", func(svcErr *tidcommon.ServiceError) {
			s.expectGroupsInSync()
			s.roleAssignments.On("GetDirectRoleIDs", mock.Anything, "user-1", "ou-1").
				Return([]string{"r1", "r-stale"}, nil).Once()
			s.roleAssignments.On("RemoveAssignments", mock.Anything, "r-stale", "ou-1",
				[]role.RoleAssignment{syncTestAssignee}).Return(svcErr).Once()
		}},
	}

	for _, tc := range tests {
		s.Run(tc.name+" is rejected", func() {
			s.expectHolder(syncTestUser())
			tc.expect(refused)
			execResp := syncTestResponse()

			ok, err := s.syncUser(both, execResp)

			s.False(ok)
			s.NoError(err)
			s.Equal(providers.ExecFailure, execResp.Status)
			s.Equal(&ErrFederatedUserSyncFailed, execResp.Error)
			s.NotContains(execResp.RuntimeData, common.RuntimeKeyPendingFederatedSync)
		})
		s.Run(tc.name+" fails", func() {
			s.expectHolder(syncTestUser())
			tc.expect(&tidcommon.InternalServerError)
			execResp := syncTestResponse()

			ok, err := s.syncUser(both, execResp)

			s.False(ok)
			s.Error(err)
			s.Empty(execResp.Status)
		})
	}
}

// expectGroupsInSync leaves the user's groups as the sync wants them.
func (s *FederatedSyncTestSuite) expectGroupsInSync() {
	s.groupService.On("GetMemberGroupIDs", mock.Anything, syncTestMember).Return([]string{"g1"}, nil).Once()
}

// A user that doesn't hold the sign-in's link is left unchanged, and the sync isn't tried again.
func (s *FederatedSyncTestSuite) TestSyncPendingUser_WithoutTheLinkWritesNothing() {
	s.entityProvider.On("GetLinkedEntities", mock.Anything, "idp-1", "fed-sub").
		Return([]*providers.Entity{{ID: "other-user"}}, nil).Once()
	execResp := syncTestResponse()

	ok, err := s.syncUser(attributeSync(map[string]interface{}{"email": "new@example.com"}), execResp)

	s.True(ok)
	s.NoError(err)
	s.Equal("", execResp.RuntimeData[common.RuntimeKeyPendingFederatedSync])
}

func (s *FederatedSyncTestSuite) TestSyncPendingUser_LinkLookupErrors() {
	tests := []struct {
		name       string
		code       entityprovider.ErrorCode
		ok, server bool
	}{
		{"system error is a server error", entityprovider.ErrorCodeSystemError, false, true},
		{"not implemented skips sync", entityprovider.ErrorCodeNotImplemented, true, false},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.entityProvider.On("GetLinkedEntities", mock.Anything, "idp-1", "fed-sub").
				Return(nil, entityprovider.NewEntityProviderError(tc.code, "failed", "detail")).Once()
			execResp := syncTestResponse()

			ok, err := s.syncUser(attributeSync(nil), execResp)

			s.Equal(tc.ok, ok)
			s.Equal(tc.server, err != nil)
			s.Empty(execResp.Status)
		})
	}
}

// A sync the same node just kept is read from its response before the flow's runtime data.
func (s *FederatedSyncTestSuite) TestSyncPendingUser_ReadsTheResponseFirst() {
	execResp := syncTestResponse()
	s.Require().NoError(setPendingFederatedSync(execResp, nil))

	ok, err := s.syncUser(attributeSync(nil), execResp)

	s.True(ok)
	s.NoError(err)
}

func (s *FederatedSyncTestSuite) TestSyncPendingUser_NothingToSync() {
	for _, tc := range []struct{ name, pending, userID string }{
		{"nothing pending", "", "user-1"},
		{"no user settled", "{}", ""},
	} {
		s.Run(tc.name, func() {
			ok, err := s.syncer.syncPendingUser(syncTestContext(tc.pending), syncTestResponse(), tc.userID, s.logger)
			s.True(ok)
			s.NoError(err)
		})
	}
}

func (s *FederatedSyncTestSuite) TestSyncPendingUser_InvalidPendingSync() {
	ok, err := s.syncer.syncPendingUser(syncTestContext("{"), syncTestResponse(), "user-1", s.logger)
	s.False(ok)
	s.Error(err)
}

func (s *FederatedSyncTestSuite) TestSetPendingFederatedSync() {
	sync := attributeSync(map[string]interface{}{"email": "new@example.com"})
	execResp := syncTestResponse()
	s.Require().NoError(setPendingFederatedSync(execResp, &sync))
	s.JSONEq(encodedSync(s, sync), execResp.RuntimeData[common.RuntimeKeyPendingFederatedSync])

	s.Require().NoError(setPendingFederatedSync(execResp, nil))
	s.Equal("", execResp.RuntimeData[common.RuntimeKeyPendingFederatedSync])
}

func (s *FederatedSyncTestSuite) TestSyncValue() {
	cases := []struct {
		name       string
		received   interface{}
		schemaType string
		want       interface{}
		wantOK     bool
	}{
		{"null", nil, entitytypemodel.TypeString, nil, true},
		{"empty string", "", entitytypemodel.TypeArray, nil, true},
		{"string", "a", entitytypemodel.TypeString, "a", true},
		{"number from string", "30", entitytypemodel.TypeNumber, float64(30), true},
		{"boolean from string", "true", entitytypemodel.TypeBoolean, true, true},
		{"array kept", []interface{}{"a", "b"}, entitytypemodel.TypeArray, []interface{}{"a", "b"}, true},
		{"single value becomes an array", "a", entitytypemodel.TypeArray, []interface{}{"a"}, true},
		{"object for an array", map[string]interface{}{"a": 1}, entitytypemodel.TypeArray, nil, false},
		{"object kept", map[string]interface{}{"a": 1}, entitytypemodel.TypeObject,
			map[string]interface{}{"a": 1}, true},
		{"string for an object", "a", entitytypemodel.TypeObject, nil, false},
		{"array for a string", []interface{}{"a"}, entitytypemodel.TypeString, nil, false},
		{"object for a string", map[string]interface{}{"a": 1}, entitytypemodel.TypeString, nil, false},
		{"empty array kept", []interface{}{}, entitytypemodel.TypeArray, []interface{}{}, true},
		{"NaN for a number", "NaN", entitytypemodel.TypeNumber, nil, false},
		{"infinity for a number", "-Inf", entitytypemodel.TypeNumber, nil, false},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			got, ok := syncValue(tc.received, tc.schemaType)
			s.Equal(tc.wantOK, ok)
			if tc.wantOK {
				s.Equal(tc.want, got)
			}
		})
	}
}

func (s *FederatedSyncTestSuite) TestJSONEqual() {
	cases := []struct {
		name string
		a, b interface{}
		want bool
	}{
		{"same number of different types", 30, float64(30), true},
		{"different values", "a", "b", false},
		{"unencodable and nil", math.NaN(), nil, false},
		{"both unencodable", math.NaN(), math.NaN(), false},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.Equal(tc.want, jsonEqual(tc.a, tc.b))
		})
	}
}

// pendingSync is the runtime data entry of a federated sign-in that kept a sync for the link idp-a,
// sub-1.
func pendingSync() string {
	encoded, _ := json.Marshal(federatedSync{IdpID: "idp-a", Sub: "sub-1", Groups: &membershipSync{}})
	return string(encoded)
}

// testFederatedSyncer builds a syncer over the given mocks, with a group service whose answers the
// test sets.
func testFederatedSyncer(t *testing.T, entityProvider entityprovider.EntityProviderInterface,
	authnProvider providers.AuthnProviderManager) (*federatedSyncer, *groupmock.GroupServiceInterfaceMock) {
	initSyncTestRuntime(t, nil)
	groupService := groupmock.NewGroupServiceInterfaceMock(t)
	return newFederatedSyncer(ExecutorDependencies{
		EntityProvider: entityProvider, GroupService: groupService, AuthnProvider: authnProvider,
	}), groupService
}

// expectPendingSync expects the pending sync for the link idp-a, sub-1 to reach its holder userID, and
// answers the read of the holder's groups with svcErr. It returns the link lookup's call.
func expectPendingSync(entityProvider *entityprovidermock.EntityProviderInterfaceMock,
	groupService *groupmock.GroupServiceInterfaceMock, userID string, svcErr *tidcommon.ServiceError) *mock.Call {
	groupService.On("GetMemberGroupIDs", mock.Anything, group.Member{ID: userID, Type: group.MemberTypeUser}).
		Return(nil, svcErr).Once()
	return entityProvider.On("GetLinkedEntities", mock.Anything, "idp-a", "sub-1").Return(
		[]*providers.Entity{{ID: userID, Category: providers.EntityCategoryUser}}, nil).Once()
}

// syncRefused is a client error a group or role write answers a sync with.
var syncRefused = &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "GRP-1"}
