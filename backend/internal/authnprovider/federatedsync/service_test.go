// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package federatedsync

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/entitytype"
	entitytypemodel "github.com/thunder-id/thunderid/internal/entitytype/model"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/groupmock"
	"github.com/thunder-id/thunderid/tests/mocks/idpprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/rolemock"
)

const (
	syncIdpID    = "idp-a"
	syncSub      = "sub-1"
	syncUserID   = "user-1"
	syncUserType = "customer"
	syncOUID     = "ou-1"
	syncNewEmail = "new@example.com"
)

var (
	noEPError  = (*entityprovider.EntityProviderError)(nil)
	noSvcError = (*tidcommon.ServiceError)(nil)
	member     = group.Member{ID: syncUserID, Type: group.MemberTypeUser}
	assignee   = role.RoleAssignment{ID: syncUserID, Type: role.AssigneeTypeUser, OUID: syncOUID}
	runtimeCtx = mock.MatchedBy(security.IsRuntimeContext)
	syncRef    = providers.EntityReference{EntityID: syncUserID, EntityCategory: string(providers.EntityCategoryUser),
		EntityType: syncUserType, OUID: syncOUID}
	readPermission = providers.AuthorizationTarget{Type: providers.AuthorizationTargetPermission,
		ResourceServerID: "rs-1", Permission: "read"}
	rejected = &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "REJ-1001"}
)

type FederatedSyncServiceTestSuite struct {
	suite.Suite
	entities      *entityprovidermock.EntityProviderInterfaceMock
	entityTypeSvc *entitytypemock.EntityTypeServiceInterfaceMock
	idpSvc        *idpprovidermock.IDPProviderMock
	groupSvc      *groupmock.GroupServiceInterfaceMock
	roleSvc       *rolemock.RoleAssignmentServiceInterfaceMock
	service       FederatedSyncServiceInterface
}

func TestFederatedSyncServiceTestSuite(t *testing.T) {
	suite.Run(t, new(FederatedSyncServiceTestSuite))
}

func (s *FederatedSyncServiceTestSuite) SetupTest() {
	s.entities = entityprovidermock.NewEntityProviderInterfaceMock(s.T())
	s.entityTypeSvc = entitytypemock.NewEntityTypeServiceInterfaceMock(s.T())
	s.idpSvc = idpprovidermock.NewIDPProviderMock(s.T())
	s.groupSvc = groupmock.NewGroupServiceInterfaceMock(s.T())
	s.roleSvc = rolemock.NewRoleAssignmentServiceInterfaceMock(s.T())
	s.service = Initialize(s.idpSvc, s.entities, s.entityTypeSvc, s.groupSvc, s.roleSvc)
}

// connection returns a connection mapping the provider's mail claim onto email for the user type, with a
// rule granting group-a, role-a and a permission to members of the "admins" team.
func connection(attributes, groupsAndRoles providers.SyncMode) *providers.IDPDTO {
	return &providers.IDPDTO{
		ID: syncIdpID,
		AttributeConfiguration: &providers.AttributeConfiguration{
			UserTypeResolution: &providers.UserTypeResolution{Default: syncUserType},
			UserTypeAttributeMappings: []providers.UserTypeAttributeMapping{{
				UserType:   syncUserType,
				Attributes: []providers.AttributeMapping{{ExternalAttribute: "mail", LocalAttribute: "email"}},
			}},
			AuthorizationMapping: &providers.AuthorizationMapping{
				Rules: []providers.AuthorizationRuleMapping{{
					Claim: "team",
					Values: []providers.AuthorizationRule{{
						Operator: providers.AuthorizationOperatorEquals, Value: "admins",
						Targets: []providers.AuthorizationTarget{
							{Type: providers.AuthorizationTargetGroup, ID: "group-a"},
							{Type: providers.AuthorizationTargetRole, ID: "role-a"},
							readPermission,
						},
					}},
				}},
			},
			Sync: &providers.SyncConfiguration{Attributes: attributes, GroupsAndRoles: groupsAndRoles},
		},
	}
}

func receivedClaims() map[string]interface{} {
	return map[string]interface{}{"sub": syncSub, "mail": syncNewEmail, "nickname": "nick", "team": "admins",
		"iss": "https://idp.example.com"}
}

func identity(claims map[string]interface{}) *providers.FederatedIdentity {
	return &providers.FederatedIdentity{IdpID: syncIdpID, Sub: syncSub, Claims: claims}
}

// expectConnection loads idpDTO, whose direct mapping resolves to nothing.
func (s *FederatedSyncServiceTestSuite) expectConnection(idpDTO *providers.IDPDTO) {
	s.idpSvc.On("GetIdentityProvider", mock.Anything, syncIdpID).Return(idpDTO, noSvcError)
	s.idpSvc.On("ResolveDirectAuthorizationTargets", mock.Anything, idpDTO, mock.Anything).
		Return([]providers.AuthorizationTarget(nil), []providers.AuthorizationTarget(nil), noSvcError)
}

func (s *FederatedSyncServiceTestSuite) expectSchema() {
	s.entityTypeSvc.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, syncUserType,
		entitytype.AttributeFilter{AllowNonCredential: true}).
		Return([]entitytype.AttributeInfo{
			{Attribute: "email", Type: entitytypemodel.TypeString},
			{Attribute: "nickname", Type: entitytypemodel.TypeString},
			{Attribute: "iss", Type: entitytypemodel.TypeString},
		}, noSvcError)
}

// expectHolder makes syncUserID the holder of the link, with the stored attributes.
func (s *FederatedSyncServiceTestSuite) expectHolder(stored map[string]interface{}) {
	s.expectLinked(providers.EntityCategoryUser, stored)
}

// expectLinked makes syncUserID, of the category, the holder of the link, with the stored attributes.
func (s *FederatedSyncServiceTestSuite) expectLinked(category providers.EntityCategory,
	stored map[string]interface{}) {
	encoded, err := json.Marshal(stored)
	s.Require().NoError(err)
	s.entities.On("GetLinkedEntities", mock.Anything, syncIdpID, syncSub).Return([]*providers.Entity{{
		ID: syncUserID, Category: category, Type: syncUserType, OUID: syncOUID, Attributes: encoded,
	}}, noEPError)
}

// expectStores makes the group and role stores writable, or declarative-only.
func (s *FederatedSyncServiceTestSuite) expectStores(declarativeOnly bool) {
	s.groupSvc.On("IsDeclarativeOnly").Return(declarativeOnly)
	s.roleSvc.On("IsDeclarativeOnly").Return(declarativeOnly)
}

func (s *FederatedSyncServiceTestSuite) sync(created bool, claims map[string]interface{}) (
	*authnprovidercm.SessionValues, bool, *tidcommon.ServiceError) {
	return s.service.SyncAccount(context.Background(), syncRef, identity(claims), created)
}

// Override received writes the mapped value that differs with the runtime's authority, and returns the
// mapped permissions for the session.
func (s *FederatedSyncServiceTestSuite) TestSync_OverrideReceivedWritesTheChangedMappedValue() {
	s.expectConnection(connection(providers.SyncModeOverrideReceived, providers.SyncModeNoSync))
	s.expectSchema()
	s.expectHolder(map[string]interface{}{"email": "old@example.com", "nickname": "kept"})
	s.expectStores(false)
	s.entities.On("UpdateAttributes", runtimeCtx, syncUserID, mock.MatchedBy(func(raw json.RawMessage) bool {
		var written map[string]interface{}
		return json.Unmarshal(raw, &written) == nil && written["email"] == syncNewEmail &&
			written["nickname"] == "kept"
	})).Return(noEPError).Once()

	values, synced, svcErr := s.sync(false, receivedClaims())

	s.Require().Nil(svcErr)
	s.True(synced)
	s.Require().NotNil(values)
	s.Equal([]providers.AuthorizationTarget{readPermission}, values.Permissions)
	s.Nil(values.Attributes)
	s.Nil(values.GroupIDs)
	s.Nil(values.RoleIDs)
}

// A stored number the sync doesn't change is written back exactly, even beyond float64 precision.
func (s *FederatedSyncServiceTestSuite) TestSync_KeepsUnchangedNumbersExact() {
	s.expectConnection(connection(providers.SyncModeOverrideReceived, providers.SyncModeNoSync))
	s.expectSchema()
	s.expectHolder(map[string]interface{}{"email": "old@example.com", "employeeNo": json.Number("9007199254740993")})
	s.expectStores(false)
	s.entities.On("UpdateAttributes", runtimeCtx, syncUserID, mock.MatchedBy(func(raw json.RawMessage) bool {
		return bytes.Contains(raw, []byte(`"employeeNo":9007199254740993`)) && bytes.Contains(raw, []byte(syncNewEmail))
	})).Return(noEPError).Once()

	_, _, svcErr := s.sync(false, receivedClaims())

	s.Require().Nil(svcErr)
}

// The sync at creation leaves the attributes Provisioning stored, and only adds the mapped groups and
// roles, even under Override all. The role assignee names its unit, so it isn't read again.
func (s *FederatedSyncServiceTestSuite) TestSync_CreatedOnlyAddsGroupsAndRoles() {
	s.expectConnection(connection(providers.SyncModeOverrideReceived, providers.SyncModeOverrideAll))
	s.expectHolder(map[string]interface{}{"email": "verified@example.com"})
	s.expectStores(false)
	s.groupSvc.On("AddMemberToExistingGroups", runtimeCtx, member, []string{"group-a"}).Return(noSvcError).Once()
	s.roleSvc.On("AddAssigneeToExistingRoles", runtimeCtx, assignee, []string{"role-a"}).Return(noSvcError).Once()

	_, synced, svcErr := s.sync(true, receivedClaims())

	s.Require().Nil(svcErr)
	s.True(synced)
	s.entities.AssertNotCalled(s.T(), "UpdateAttributes", mock.Anything, mock.Anything, mock.Anything)
	s.groupSvc.AssertNotCalled(s.T(), "GetMemberGroupIDs", mock.Anything, mock.Anything)
	s.groupSvc.AssertNotCalled(s.T(), "RemoveGroupMembers", mock.Anything, mock.Anything, mock.Anything)
}

// Override all on a returning account replaces its direct groups and roles, keeping the ones a direct
// mapping name matched ambiguously and the ones the sync can't remove.
func (s *FederatedSyncServiceTestSuite) TestSync_OverrideAllReplacesAndKeepsAmbiguousMatches() {
	idpDTO := connection(providers.SyncModeNoSync, providers.SyncModeOverrideAll)
	s.idpSvc.On("GetIdentityProvider", mock.Anything, syncIdpID).Return(idpDTO, noSvcError)
	s.idpSvc.On("ResolveDirectAuthorizationTargets", mock.Anything, idpDTO, mock.Anything).
		Return([]providers.AuthorizationTarget(nil), []providers.AuthorizationTarget{
			{Type: providers.AuthorizationTargetGroup, ID: "group-ambiguous"},
			{Type: providers.AuthorizationTargetRole, ID: "role-ambiguous"},
		}, noSvcError).Once()
	s.expectHolder(map[string]interface{}{})
	s.expectStores(false)
	s.groupSvc.On("GetMemberGroupIDs", mock.Anything, member).
		Return([]string{"group-a", "group-ambiguous", "group-old", "group-managed"}, noSvcError)
	s.groupSvc.On("RemoveGroupMembers", runtimeCtx, "group-old", []group.Member{member}).
		Return((*group.Group)(nil), noSvcError).Once()
	s.groupSvc.On("RemoveGroupMembers", runtimeCtx, "group-managed", []group.Member{member}).
		Return((*group.Group)(nil), &group.ErrorManagedGroupMember).Once()
	s.roleSvc.On("GetDirectRoleIDs", mock.Anything, syncUserID, syncOUID).
		Return([]string{"role-ambiguous", "role-old", "role-mandated"}, noSvcError)
	s.roleSvc.On("AddAssigneeToExistingRoles", runtimeCtx, assignee, []string{"role-a"}).Return(noSvcError).Once()
	s.roleSvc.On("RemoveAssignments", runtimeCtx, "role-old", syncOUID, []role.RoleAssignment{assignee}).
		Return(noSvcError).Once()
	s.roleSvc.On("RemoveAssignments", runtimeCtx, "role-mandated", syncOUID, []role.RoleAssignment{assignee}).
		Return(&role.ErrorAssignmentMandated).Once()

	_, _, svcErr := s.sync(false, receivedClaims())

	s.Require().Nil(svcErr)
	s.groupSvc.AssertNotCalled(s.T(), "AddMemberToExistingGroups", mock.Anything, mock.Anything, mock.Anything)
}

// Session only stores nothing and returns the mapped attributes and the groups with their ancestors for
// the session. A claim no mapping fills, though it names an attribute, is left out.
func (s *FederatedSyncServiceTestSuite) TestSync_SessionOnlyReturnsSessionValues() {
	s.expectConnection(connection(providers.SyncModeSessionOnly, providers.SyncModeSessionOnly))
	s.expectSchema()
	s.expectHolder(map[string]interface{}{"email": "stored@example.com"})
	s.entities.On("GetTransitiveGroupAncestors", mock.Anything, "group-a").Return([]string{"group-parent"}, noEPError)

	values, _, svcErr := s.sync(true, receivedClaims())

	s.Require().Nil(svcErr)
	s.Require().NotNil(values)
	s.Equal(map[string]interface{}{"email": syncNewEmail}, values.Attributes)
	s.Equal([]string{"group-a"}, values.GroupIDs)
	s.Equal([]string{"group-a", "group-parent"}, values.GroupIDsWithAncestors)
	s.Equal([]string{"role-a"}, values.RoleIDs)
	s.entities.AssertNotCalled(s.T(), "UpdateAttributes", mock.Anything, mock.Anything, mock.Anything)
	s.groupSvc.AssertNotCalled(s.T(), "AddMemberToExistingGroups", mock.Anything, mock.Anything, mock.Anything)
}

// Session only leaves an attribute out when its mapping finds no claim, even if a claim of the attribute's
// own name arrives.
func (s *FederatedSyncServiceTestSuite) TestSync_SessionOnlyIgnoresUnmappedSameNameClaim() {
	s.expectConnection(connection(providers.SyncModeSessionOnly, providers.SyncModeNoSync))
	s.expectSchema()
	s.expectHolder(map[string]interface{}{"email": "stored@example.com"})
	s.expectStores(false)

	values, _, svcErr := s.sync(false, map[string]interface{}{"sub": syncSub, "email": "raw@example.com"})

	s.Require().Nil(svcErr)
	s.Empty(values.Attributes)
}

// A declarative-only group or role store holds no memberships, so the mapped ones go to the session.
func (s *FederatedSyncServiceTestSuite) TestSync_DeclarativeStoresKeepGroupsAndRolesToTheSession() {
	s.expectConnection(connection(providers.SyncModeNoSync, providers.SyncModeOverrideAll))
	s.expectHolder(map[string]interface{}{})
	s.expectStores(true)
	s.entities.On("GetTransitiveGroupAncestors", mock.Anything, "group-a").Return([]string{}, noEPError)

	values, _, svcErr := s.sync(false, receivedClaims())

	s.Require().Nil(svcErr)
	s.Equal([]string{"group-a"}, values.GroupIDs)
	s.Equal([]string{"role-a"}, values.RoleIDs)
	s.groupSvc.AssertNotCalled(s.T(), "GetMemberGroupIDs", mock.Anything, mock.Anything)
	s.roleSvc.AssertNotCalled(s.T(), "GetDirectRoleIDs", mock.Anything, mock.Anything, mock.Anything)
}

// An account that doesn't hold the link isn't written.
func (s *FederatedSyncServiceTestSuite) TestSync_NonHolderIsNotSynced() {
	s.expectConnection(connection(providers.SyncModeOverrideReceived, providers.SyncModeOverrideAll))
	s.entities.On("GetLinkedEntities", mock.Anything, syncIdpID, syncSub).
		Return([]*providers.Entity{{ID: "someone-else"}}, noEPError)

	values, synced, svcErr := s.sync(false, receivedClaims())

	s.Require().Nil(svcErr)
	s.False(synced)
	s.Nil(values)
}

// With the entity provider disabled, the sync reads and writes no account, and the mapping goes to the
// session.
func (s *FederatedSyncServiceTestSuite) TestSync_DisabledEntityProviderGrantsTheSession() {
	s.expectConnection(connection(providers.SyncModeSessionOnly, providers.SyncModeNoSync))
	s.expectSchema()
	s.entities.On("GetLinkedEntities", mock.Anything, syncIdpID, syncSub).Return([]*providers.Entity(nil),
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeNotImplemented, "Not implemented", ""))

	values, synced, svcErr := s.sync(false, receivedClaims())

	s.Require().Nil(svcErr)
	s.True(synced)
	s.Require().NotNil(values)
	s.Equal(map[string]interface{}{"email": syncNewEmail}, values.Attributes)
	s.Equal([]string{"group-a"}, values.GroupIDsWithAncestors)
	s.Equal([]string{"role-a"}, values.RoleIDs)
	s.entities.AssertNotCalled(s.T(), "GetTransitiveGroupAncestors", mock.Anything, mock.Anything)
}

// Without an entity provider or entity type service, as in the embedded engine, the mapping still goes to
// the session, without attributes.
func (s *FederatedSyncServiceTestSuite) TestSync_NoStoreGrantsTheSession() {
	s.service = Initialize(s.idpSvc, nil, nil, nil, nil)
	s.expectConnection(connection(providers.SyncModeSessionOnly, providers.SyncModeOverrideAll))

	values, synced, svcErr := s.sync(false, receivedClaims())

	s.Require().Nil(svcErr)
	s.True(synced)
	s.Require().NotNil(values)
	s.Nil(values.Attributes)
	s.Equal([]string{"group-a"}, values.GroupIDs)
	s.Equal([]string{"group-a"}, values.GroupIDsWithAncestors)
	s.Equal([]string{"role-a"}, values.RoleIDs)
	s.Equal([]providers.AuthorizationTarget{readPermission}, values.Permissions)
}

// Any other entity provider failure fails the sync.
func (s *FederatedSyncServiceTestSuite) TestSync_EntityProviderFailureFails() {
	s.expectConnection(connection(providers.SyncModeOverrideReceived, providers.SyncModeOverrideAll))
	s.entities.On("GetLinkedEntities", mock.Anything, syncIdpID, syncSub).Return([]*providers.Entity(nil),
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeSystemError, "System error", ""))

	_, _, svcErr := s.sync(false, receivedClaims())

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

// A connection that can't be loaded fails the sync with a server error.
func (s *FederatedSyncServiceTestSuite) TestSync_ConnectionLoadFailureFails() {
	s.idpSvc.On("GetIdentityProvider", mock.Anything, syncIdpID).
		Return((*providers.IDPDTO)(nil), &tidcommon.InternalServerError)

	_, _, svcErr := s.sync(false, receivedClaims())

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	s.entities.AssertNotCalled(s.T(), "GetLinkedEntities", mock.Anything, mock.Anything, mock.Anything)
}

// incompleteOverrideAll returns a connection whose direct group lookup fails with lookupErr.
func (s *FederatedSyncServiceTestSuite) incompleteOverrideAll(attributes providers.SyncMode,
	lookupErr *tidcommon.ServiceError) {
	idpDTO := connection(attributes, providers.SyncModeOverrideAll)
	idpDTO.AttributeConfiguration.AuthorizationMapping.Direct = []providers.AuthorizationDirectMapping{
		{Claim: "groups", TargetType: providers.AuthorizationTargetGroup},
	}
	s.idpSvc.On("GetIdentityProvider", mock.Anything, syncIdpID).Return(idpDTO, noSvcError)
	s.idpSvc.On("ResolveDirectAuthorizationTargets", mock.Anything, idpDTO, mock.Anything).
		Return([]providers.AuthorizationTarget(nil), []providers.AuthorizationTarget(nil), lookupErr)
}

// An Override all mapping whose direct lookups fail fails a returning account's sync before anything is
// written: with the sync error when the lookup was refused, and a server error when it failed.
func (s *FederatedSyncServiceTestSuite) TestSync_IncompleteOverrideAllMappingFails() {
	for _, tc := range []struct {
		name      string
		lookupErr *tidcommon.ServiceError
		wantCode  string
	}{
		{"refused", rejected, ErrorFederatedSyncFailed.Code},
		{"failed", &tidcommon.InternalServerError, tidcommon.InternalServerError.Code},
	} {
		s.Run(tc.name, func() {
			s.SetupTest()
			s.incompleteOverrideAll(providers.SyncModeOverrideReceived, tc.lookupErr)
			s.expectHolder(map[string]interface{}{"email": "old@example.com"})
			s.expectStores(false)

			_, _, svcErr := s.sync(false, receivedClaims())

			s.Require().NotNil(svcErr)
			s.Equal(tc.wantCode, svcErr.Code)
			s.entities.AssertNotCalled(s.T(), "UpdateAttributes", mock.Anything, mock.Anything, mock.Anything)
			s.groupSvc.AssertNotCalled(s.T(), "GetMemberGroupIDs", mock.Anything, mock.Anything)
			s.roleSvc.AssertNotCalled(s.T(), "GetDirectRoleIDs", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

// An incomplete Override all mapping doesn't fail a sync that replaces nothing with it: an account the
// sign-in created, a declarative-only store, or no store. The sign-in keeps the rule-based targets.
func (s *FederatedSyncServiceTestSuite) TestSync_IncompleteOverrideAllMappingWithoutReplaceContinues() {
	s.Run("created", func() {
		s.SetupTest()
		s.incompleteOverrideAll(providers.SyncModeNoSync, rejected)
		s.expectHolder(map[string]interface{}{})
		s.expectStores(false)
		s.groupSvc.On("AddMemberToExistingGroups", runtimeCtx, member, []string{"group-a"}).
			Return(noSvcError).Once()
		s.roleSvc.On("AddAssigneeToExistingRoles", runtimeCtx, assignee, []string{"role-a"}).
			Return(noSvcError).Once()

		_, synced, svcErr := s.sync(true, receivedClaims())

		s.Require().Nil(svcErr)
		s.True(synced)
	})
	s.Run("declarative only", func() {
		s.SetupTest()
		s.incompleteOverrideAll(providers.SyncModeNoSync, rejected)
		s.expectHolder(map[string]interface{}{})
		s.expectStores(true)
		s.entities.On("GetTransitiveGroupAncestors", mock.Anything, "group-a").Return([]string{}, noEPError)

		values, _, svcErr := s.sync(false, receivedClaims())

		s.Require().Nil(svcErr)
		s.Equal([]string{"group-a"}, values.GroupIDs)
		s.Equal([]string{"role-a"}, values.RoleIDs)
	})
	s.Run("no store", func() {
		s.SetupTest()
		s.service = Initialize(s.idpSvc, nil, nil, nil, nil)
		s.incompleteOverrideAll(providers.SyncModeNoSync, rejected)

		values, _, svcErr := s.sync(false, receivedClaims())

		s.Require().Nil(svcErr)
		s.Equal([]string{"group-a"}, values.GroupIDs)
		s.Equal([]string{"role-a"}, values.RoleIDs)
	})
}

// Outside Override all, a failed direct lookup leaves the sync with the rule-based targets.
func (s *FederatedSyncServiceTestSuite) TestSync_FailedDirectLookupKeepsRuleTargets() {
	idpDTO := connection(providers.SyncModeNoSync, providers.SyncModeSessionOnly)
	s.idpSvc.On("GetIdentityProvider", mock.Anything, syncIdpID).Return(idpDTO, noSvcError)
	s.idpSvc.On("ResolveDirectAuthorizationTargets", mock.Anything, idpDTO, mock.Anything).
		Return([]providers.AuthorizationTarget(nil), []providers.AuthorizationTarget(nil),
			&tidcommon.InternalServerError)
	s.expectHolder(map[string]interface{}{})
	s.entities.On("GetTransitiveGroupAncestors", mock.Anything, "group-a").Return([]string{}, noEPError)

	values, synced, svcErr := s.sync(false, receivedClaims())

	s.Require().Nil(svcErr)
	s.True(synced)
	s.Equal([]string{"group-a"}, values.GroupIDs)
	s.Equal([]string{"role-a"}, values.RoleIDs)
}

// A group whose ancestors can't be read is kept for the session without them.
func (s *FederatedSyncServiceTestSuite) TestSync_FailedAncestorLookupKeepsTheGroup() {
	s.expectConnection(connection(providers.SyncModeNoSync, providers.SyncModeSessionOnly))
	s.expectHolder(map[string]interface{}{})
	s.entities.On("GetTransitiveGroupAncestors", mock.Anything, "group-a").Return([]string(nil),
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeSystemError, "System error", ""))

	values, _, svcErr := s.sync(false, receivedClaims())

	s.Require().Nil(svcErr)
	s.Equal([]string{"group-a"}, values.GroupIDsWithAncestors)
}

// A refused group or role call fails the sync with the sync error, and a failed one with a server error.
func (s *FederatedSyncServiceTestSuite) TestSync_FailedGroupOrRoleCall() {
	for _, call := range []struct {
		name   string
		expect func(err *tidcommon.ServiceError)
	}{
		{"group read", func(err *tidcommon.ServiceError) {
			s.groupSvc.On("GetMemberGroupIDs", mock.Anything, member).Return([]string(nil), err)
		}},
		{"group add", func(err *tidcommon.ServiceError) {
			s.groupSvc.On("GetMemberGroupIDs", mock.Anything, member).Return([]string{}, noSvcError)
			s.groupSvc.On("AddMemberToExistingGroups", runtimeCtx, member, []string{"group-a"}).Return(err)
		}},
		{"group removal", func(err *tidcommon.ServiceError) {
			s.groupSvc.On("GetMemberGroupIDs", mock.Anything, member).Return([]string{"group-a", "group-old"},
				noSvcError)
			s.groupSvc.On("RemoveGroupMembers", runtimeCtx, "group-old", []group.Member{member}).
				Return((*group.Group)(nil), err)
		}},
		{"role read", func(err *tidcommon.ServiceError) {
			s.groupSvc.On("GetMemberGroupIDs", mock.Anything, member).Return([]string{"group-a"}, noSvcError)
			s.roleSvc.On("GetDirectRoleIDs", mock.Anything, syncUserID, syncOUID).Return([]string(nil), err)
		}},
		{"role add", func(err *tidcommon.ServiceError) {
			s.groupSvc.On("GetMemberGroupIDs", mock.Anything, member).Return([]string{"group-a"}, noSvcError)
			s.roleSvc.On("GetDirectRoleIDs", mock.Anything, syncUserID, syncOUID).Return([]string{}, noSvcError)
			s.roleSvc.On("AddAssigneeToExistingRoles", runtimeCtx, assignee, []string{"role-a"}).Return(err)
		}},
		{"role removal", func(err *tidcommon.ServiceError) {
			s.groupSvc.On("GetMemberGroupIDs", mock.Anything, member).Return([]string{"group-a"}, noSvcError)
			s.roleSvc.On("GetDirectRoleIDs", mock.Anything, syncUserID, syncOUID).
				Return([]string{"role-a", "role-old"}, noSvcError)
			s.roleSvc.On("RemoveAssignments", runtimeCtx, "role-old", syncOUID, []role.RoleAssignment{assignee}).
				Return(err)
		}},
	} {
		for _, tc := range []struct {
			name     string
			err      *tidcommon.ServiceError
			wantCode string
		}{
			{"refused", rejected, ErrorFederatedSyncFailed.Code},
			{"failed", &tidcommon.InternalServerError, tidcommon.InternalServerError.Code},
		} {
			s.Run(call.name+" "+tc.name, func() {
				s.SetupTest()
				s.expectConnection(connection(providers.SyncModeNoSync, providers.SyncModeOverrideAll))
				s.expectHolder(map[string]interface{}{})
				s.expectStores(false)
				call.expect(tc.err)

				_, _, svcErr := s.sync(false, receivedClaims())

				s.Require().NotNil(svcErr)
				s.Equal(tc.wantCode, svcErr.Code)
			})
		}
	}
}

// An account that isn't a user takes no attributes, and is a group member and role assignee of its own
// category.
func (s *FederatedSyncServiceTestSuite) TestSync_NonUserAccount() {
	s.expectConnection(connection(providers.SyncModeOverrideReceived, providers.SyncModeOverrideAll))
	s.expectLinked(providers.EntityCategoryAgent, map[string]interface{}{})
	s.expectStores(false)
	agentMember := group.Member{ID: syncUserID, Type: group.MemberTypeAgent}
	agentAssignee := role.RoleAssignment{ID: syncUserID, Type: role.AssigneeTypeAgent, OUID: syncOUID}
	s.groupSvc.On("AddMemberToExistingGroups", runtimeCtx, agentMember, []string{"group-a"}).Return(noSvcError).Once()
	s.roleSvc.On("AddAssigneeToExistingRoles", runtimeCtx, agentAssignee, []string{"role-a"}).
		Return(noSvcError).Once()

	values, _, svcErr := s.sync(true, receivedClaims())

	s.Require().Nil(svcErr)
	s.Nil(values.Attributes)
	s.entityTypeSvc.AssertNotCalled(s.T(), "GetAttributes", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything)
}

// A value a returning account refuses, such as a unique one another user holds, is left out while the
// others are written.
func (s *FederatedSyncServiceTestSuite) TestSync_RefusedValueIsSkipped() {
	idpDTO := connection(providers.SyncModeOverrideReceived, providers.SyncModeNoSync)
	mapping := &idpDTO.AttributeConfiguration.UserTypeAttributeMappings[0]
	mapping.Attributes = append(mapping.Attributes,
		providers.AttributeMapping{ExternalAttribute: "nickname", LocalAttribute: "nickname"})
	s.expectConnection(idpDTO)
	s.expectSchema()
	s.expectHolder(map[string]interface{}{})
	s.expectStores(false)
	written := func(want map[string]interface{}) interface{} {
		return mock.MatchedBy(func(raw json.RawMessage) bool {
			var got map[string]interface{}
			return json.Unmarshal(raw, &got) == nil && len(got) == len(want) &&
				got["email"] == want["email"] && got["nickname"] == want["nickname"]
		})
	}
	conflict := entityprovider.NewEntityProviderError(entityprovider.ErrorCodeAttributeConflict, "Conflict", "")
	// The whole set and the email alone are refused, so only the nickname is written.
	s.entities.On("UpdateAttributes", runtimeCtx, syncUserID,
		written(map[string]interface{}{"email": syncNewEmail, "nickname": "nick"})).Return(conflict).Once()
	s.entities.On("UpdateAttributes", runtimeCtx, syncUserID,
		written(map[string]interface{}{"email": syncNewEmail})).Return(conflict).Once()
	s.entities.On("UpdateAttributes", runtimeCtx, syncUserID,
		written(map[string]interface{}{"nickname": "nick"})).Return(noEPError).Once()

	_, _, svcErr := s.sync(false, receivedClaims())

	s.Require().Nil(svcErr)
	s.entities.AssertNumberOfCalls(s.T(), "UpdateAttributes", 3)
}

// A failed write the account didn't refuse is a server error.
func (s *FederatedSyncServiceTestSuite) TestSync_AttributeWriteServerError() {
	s.expectConnection(connection(providers.SyncModeOverrideReceived, providers.SyncModeNoSync))
	s.expectSchema()
	s.expectHolder(map[string]interface{}{})
	s.expectStores(false)
	s.entities.On("UpdateAttributes", runtimeCtx, syncUserID, mock.Anything).
		Return(entityprovider.NewEntityProviderError(entityprovider.ErrorCodeSystemError, "System error", ""))

	_, _, svcErr := s.sync(false, receivedClaims())

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ServerErrorType, svcErr.Type)
}

// Override received leaves an attribute alone when its mapping finds no claim, even if a claim of the
// attribute's own name arrives.
func (s *FederatedSyncServiceTestSuite) TestSync_OverrideReceivedIgnoresUnmappedSameNameClaim() {
	s.expectConnection(connection(providers.SyncModeOverrideReceived, providers.SyncModeNoSync))
	s.expectSchema()
	s.expectHolder(map[string]interface{}{"email": "old@example.com"})
	s.expectStores(false)

	_, _, svcErr := s.sync(false, map[string]interface{}{"sub": syncSub, "email": "raw@example.com"})

	s.Require().Nil(svcErr)
	s.entities.AssertNotCalled(s.T(), "UpdateAttributes", mock.Anything, mock.Anything, mock.Anything)
}

// A mapping that fills a claim describing the token, such as iss, doesn't sync it.
func (s *FederatedSyncServiceTestSuite) TestSync_MappedTokenMetadataClaimIsLeftOut() {
	idpDTO := connection(providers.SyncModeSessionOnly, providers.SyncModeNoSync)
	mapping := &idpDTO.AttributeConfiguration.UserTypeAttributeMappings[0]
	mapping.Attributes = append(mapping.Attributes, providers.AttributeMapping{
		ExternalAttribute: "issuer", LocalAttribute: "iss"})
	s.expectConnection(idpDTO)
	s.expectSchema()
	s.expectHolder(map[string]interface{}{})
	s.expectStores(false)

	values, _, svcErr := s.sync(false, map[string]interface{}{"sub": syncSub, "mail": syncNewEmail,
		"issuer": "https://idp.example.com"})

	s.Require().Nil(svcErr)
	s.Equal(map[string]interface{}{"email": syncNewEmail}, values.Attributes)
}
