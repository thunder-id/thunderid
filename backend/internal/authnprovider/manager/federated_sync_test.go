// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package manager

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
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/providermock"
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
	federated  = map[string]interface{}{authnprovidercm.CredentialTypeFederated: "code"}
	byID       = map[string]interface{}{authnprovidercm.CredentialTypeProvisionedEntityID: syncUserID}
	member     = group.Member{ID: syncUserID, Type: group.MemberTypeUser}
	assignee   = role.RoleAssignment{ID: syncUserID, Type: role.AssigneeTypeUser}
	runtimeCtx = mock.MatchedBy(security.IsRuntimeContext)
)

type FederatedSyncTestSuite struct {
	suite.Suite
	provider      *providermock.AuthnProviderInterfaceMock
	entities      *entityprovidermock.EntityProviderInterfaceMock
	entityTypeSvc *entitytypemock.EntityTypeServiceInterfaceMock
	idpSvc        *idpprovidermock.IDPProviderMock
	groupSvc      *groupmock.GroupServiceInterfaceMock
	roleSvc       *rolemock.RoleAssignmentServiceInterfaceMock
	mgr           providers.AuthnProviderManager
}

func TestFederatedSyncTestSuite(t *testing.T) {
	suite.Run(t, new(FederatedSyncTestSuite))
}

func (s *FederatedSyncTestSuite) SetupTest() {
	s.initRuntime(config.Config{})
	s.provider = providermock.NewAuthnProviderInterfaceMock(s.T())
	s.entities = entityprovidermock.NewEntityProviderInterfaceMock(s.T())
	s.entityTypeSvc = entitytypemock.NewEntityTypeServiceInterfaceMock(s.T())
	s.idpSvc = idpprovidermock.NewIDPProviderMock(s.T())
	s.groupSvc = groupmock.NewGroupServiceInterfaceMock(s.T())
	s.roleSvc = rolemock.NewRoleAssignmentServiceInterfaceMock(s.T())
	mgr, err := Initialize(s.provider, nil, newDiscardingBus(s.T()), FederatedSyncDependencies{
		EntityProvider:        s.entities,
		EntityTypeService:     s.entityTypeSvc,
		GroupService:          s.groupSvc,
		RoleAssignmentService: s.roleSvc,
		IDPProvider:           s.idpSvc,
	})
	s.Require().NoError(err)
	s.mgr = mgr
}

func (s *FederatedSyncTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

// initRuntime initializes the server runtime with cfg.
func (s *FederatedSyncTestSuite) initRuntime(cfg config.Config) {
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), &cfg))
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
							{Type: providers.AuthorizationTargetPermission, ResourceServerID: "rs-1",
								Permission: "read"},
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

func (s *FederatedSyncTestSuite) expectConnection(idp *providers.IDPDTO) {
	s.idpSvc.On("GetIdentityProvider", mock.Anything, syncIdpID).Return(idp, noSvcError)
	s.idpSvc.On("ResolveDirectAuthorizationTargets", mock.Anything, idp, mock.Anything).
		Return([]providers.AuthorizationTarget(nil), []providers.AuthorizationTarget(nil), noSvcError).Maybe()
}

func (s *FederatedSyncTestSuite) expectSchema() {
	s.entityTypeSvc.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, syncUserType,
		entitytype.AttributeFilter{AllowNonCredential: true}).
		Return([]entitytype.AttributeInfo{
			{Attribute: "email", Type: entitytypemodel.TypeString},
			{Attribute: "nickname", Type: entitytypemodel.TypeString},
			{Attribute: "iss", Type: entitytypemodel.TypeString},
		}, noSvcError)
}

// expectHolder makes syncUserID the holder of the link, with the stored attributes.
func (s *FederatedSyncTestSuite) expectHolder(stored map[string]interface{}) {
	encoded, err := json.Marshal(stored)
	s.Require().NoError(err)
	s.entities.On("GetLinkedEntities", mock.Anything, syncIdpID, syncSub).Return([]*providers.Entity{{
		ID: syncUserID, Category: providers.EntityCategoryUser, Type: syncUserType, OUID: syncOUID,
		Attributes: encoded,
	}}, noEPError)
}

// signIn runs a federated sign-in the provider resolves to syncUserID.
func (s *FederatedSyncTestSuite) signIn(metadata *providers.AuthnMetadata) (
	providers.AuthUser, providers.AuthenticatedClaims, *tidcommon.ServiceError) {
	ref := providers.EntityReference{EntityID: syncUserID, EntityCategory: string(providers.EntityCategoryUser),
		EntityType: syncUserType, OUID: syncOUID}
	s.provider.On("Authenticate", mock.Anything, mock.Anything, federated, mock.Anything).
		Return(&providers.AuthnResult{
			AuthenticatedClaims: providers.AuthenticatedClaims{"sub": syncSub},
			EntityReference:     &ref,
			AttributeToken:      map[string]interface{}{"userID": syncUserID},
			FederatedIdentity:   identity(receivedClaims()),
		}, noSvcError).Once()
	return s.mgr.AuthenticateUser(context.Background(), nil, federated, nil, metadata, providers.AuthUser{})
}

// runSync runs the deferred sync of the identity pending on pendingOn, signing in syncUserID by ID.
func (s *FederatedSyncTestSuite) runSync(created bool, pendingOn string, claims map[string]interface{}) (
	providers.AuthUser, providers.AuthenticatedClaims, *tidcommon.ServiceError) {
	ref := providers.EntityReference{EntityID: syncUserID, EntityCategory: string(providers.EntityCategoryUser),
		EntityType: syncUserType, OUID: syncOUID}
	authUser := authUserWithStates(map[string]providers.AuthState{pendingOn: {
		EntityReferenceToken: "token", AttributeToken: "token", PendingIdentity: identity(claims),
	}})
	s.provider.On("Authenticate", mock.Anything, mock.Anything, byID, mock.Anything).
		Return(&providers.AuthnResult{
			AuthenticatedClaims: providers.AuthenticatedClaims{"userID": syncUserID},
			EntityReference:     &ref,
			AttributeToken:      map[string]interface{}{"userID": syncUserID},
		}, noSvcError).Once()
	return s.mgr.AuthenticateUser(context.Background(), nil, byID, nil,
		&providers.AuthnMetadata{Created: created}, authUser)
}

func sessionValuesOf(claims providers.AuthenticatedClaims) *authnprovidercm.SessionValues {
	values, _ := claims[authnprovidercm.ClaimSessionValues].(*authnprovidercm.SessionValues)
	return values
}

func pendingOf(authUser providers.AuthUser, name string) *providers.FederatedIdentity {
	state, _ := authUser.StateFor(name)
	return state.PendingIdentity
}

// A sign-in syncs the account holding the link: Override received writes the mapped value that differs
// with the runtime's authority, the session values are returned, and nothing stays pending.
func (s *FederatedSyncTestSuite) TestSignIn_SyncsTheHolder() {
	s.expectConnection(connection(providers.SyncModeOverrideReceived, providers.SyncModeNoSync))
	s.expectSchema()
	s.expectHolder(map[string]interface{}{"email": "old@example.com", "nickname": "kept"})
	s.entities.On("UpdateAttributes", runtimeCtx, syncUserID, mock.MatchedBy(func(raw json.RawMessage) bool {
		var written map[string]interface{}
		return json.Unmarshal(raw, &written) == nil && written["email"] == syncNewEmail &&
			written["nickname"] == "kept"
	})).Return(noEPError).Once()

	authUser, claims, svcErr := s.signIn(&providers.AuthnMetadata{})

	s.Require().Nil(svcErr)
	s.Nil(pendingOf(authUser, defaultProviderName))
	values := sessionValuesOf(claims)
	s.Require().NotNil(values)
	s.Equal([]providers.AuthorizationTarget{
		{Type: providers.AuthorizationTargetPermission, ResourceServerID: "rs-1", Permission: "read"},
	}, values.Permissions)
	s.Equal(syncSub, claims["sub"])
	state, _ := authUser.StateFor(defaultProviderName)
	s.NotNil(state.AttributeToken, "attributes load after the writes")
}

// A stored number the sync doesn't change is written back exactly, even beyond float64 precision.
func (s *FederatedSyncTestSuite) TestSignIn_KeepsUnchangedNumbersExact() {
	s.expectConnection(connection(providers.SyncModeOverrideReceived, providers.SyncModeNoSync))
	s.expectSchema()
	s.expectHolder(map[string]interface{}{"email": "old@example.com", "employeeNo": json.Number("9007199254740993")})
	s.entities.On("UpdateAttributes", runtimeCtx, syncUserID, mock.MatchedBy(func(raw json.RawMessage) bool {
		return bytes.Contains(raw, []byte(`"employeeNo":9007199254740993`)) && bytes.Contains(raw, []byte(syncNewEmail))
	})).Return(noEPError).Once()

	_, _, svcErr := s.signIn(&providers.AuthnMetadata{})

	s.Require().Nil(svcErr)
}

// A deferred sign-in maps, reads and writes nothing, and keeps its identity pending.
func (s *FederatedSyncTestSuite) TestSignIn_DeferSyncKeepsTheIdentityPending() {
	authUser, claims, svcErr := s.signIn(&providers.AuthnMetadata{DeferSync: true})

	s.Require().Nil(svcErr)
	s.Equal(identity(receivedClaims()), pendingOf(authUser, defaultProviderName))
	s.Nil(sessionValuesOf(claims))
	s.idpSvc.AssertNotCalled(s.T(), "GetIdentityProvider", mock.Anything, mock.Anything)
}

// A sign-in no single account holds keeps its identity pending.
func (s *FederatedSyncTestSuite) TestSignIn_NoHolderKeepsTheIdentityPending() {
	s.provider.On("Authenticate", mock.Anything, mock.Anything, federated, mock.Anything).
		Return(&providers.AuthnResult{EntityReferenceToken: "token", AttributeToken: "token",
			FederatedIdentity: identity(nil)}, noSvcError).Once()

	authUser, _, svcErr := s.mgr.AuthenticateUser(context.Background(), nil, federated, nil, nil,
		providers.AuthUser{})

	s.Require().Nil(svcErr)
	s.Equal(identity(nil), pendingOf(authUser, defaultProviderName))
	s.idpSvc.AssertNotCalled(s.T(), "GetIdentityProvider", mock.Anything, mock.Anything)
}

// The sync at creation leaves the attributes Provisioning stored, and only adds the mapped groups and
// roles, even under Override all.
func (s *FederatedSyncTestSuite) TestDeferredSync_CreatedOnlyAddsGroupsAndRoles() {
	s.expectConnection(connection(providers.SyncModeOverrideReceived, providers.SyncModeOverrideAll))
	s.expectHolder(map[string]interface{}{"email": "verified@example.com"})
	s.groupSvc.On("AddMemberToExistingGroups", runtimeCtx, member, []string{"group-a"}).Return(noSvcError).Once()
	s.roleSvc.On("AddAssigneeToExistingRoles", runtimeCtx, assignee, []string{"role-a"}).Return(noSvcError).Once()

	authUser, _, svcErr := s.runSync(true, defaultProviderName, receivedClaims())

	s.Require().Nil(svcErr)
	s.Nil(pendingOf(authUser, defaultProviderName))
	s.entities.AssertNotCalled(s.T(), "UpdateAttributes", mock.Anything, mock.Anything, mock.Anything)
	s.groupSvc.AssertNotCalled(s.T(), "GetMemberGroupIDs", mock.Anything, mock.Anything)
	s.groupSvc.AssertNotCalled(s.T(), "RemoveGroupMembers", mock.Anything, mock.Anything, mock.Anything)
}

// A deferred sync uses the identity pending on another provider, such as the one that verified the
// federated sign-in, and clears it there.
func (s *FederatedSyncTestSuite) TestDeferredSync_FindsTheIdentityOnAnyProvider() {
	s.expectConnection(connection(providers.SyncModeNoSync, providers.SyncModeNoSync))
	s.expectHolder(map[string]interface{}{})

	authUser, claims, svcErr := s.runSync(false, "rest", receivedClaims())

	s.Require().Nil(svcErr)
	s.Nil(pendingOf(authUser, "rest"))
	s.Nil(pendingOf(authUser, defaultProviderName))
	s.NotNil(sessionValuesOf(claims))
}

// Override all on a returning account replaces its direct groups and roles, keeping the ones a direct
// mapping name matched ambiguously.
func (s *FederatedSyncTestSuite) TestDeferredSync_OverrideAllReplacesAndKeepsAmbiguousMatches() {
	idpDTO := connection(providers.SyncModeNoSync, providers.SyncModeOverrideAll)
	s.idpSvc.On("GetIdentityProvider", mock.Anything, syncIdpID).Return(idpDTO, noSvcError)
	s.idpSvc.On("ResolveDirectAuthorizationTargets", mock.Anything, idpDTO, mock.Anything).
		Return([]providers.AuthorizationTarget(nil), []providers.AuthorizationTarget{
			{Type: providers.AuthorizationTargetGroup, ID: "group-ambiguous"},
			{Type: providers.AuthorizationTargetRole, ID: "role-ambiguous"},
		}, noSvcError).Once()
	s.expectHolder(map[string]interface{}{})
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

	_, _, svcErr := s.runSync(false, defaultProviderName, receivedClaims())

	s.Require().Nil(svcErr)
	s.groupSvc.AssertNotCalled(s.T(), "AddMemberToExistingGroups", mock.Anything, mock.Anything, mock.Anything)
}

// Session only stores nothing and returns the values for the session, with the groups' ancestors.
func (s *FederatedSyncTestSuite) TestDeferredSync_SessionOnlyReturnsSessionValues() {
	s.expectConnection(connection(providers.SyncModeSessionOnly, providers.SyncModeSessionOnly))
	s.expectSchema()
	s.expectHolder(map[string]interface{}{"email": "stored@example.com"})
	s.entities.On("GetTransitiveGroupAncestors", mock.Anything, "group-a").Return([]string{"group-parent"}, noEPError)

	_, claims, svcErr := s.runSync(true, defaultProviderName, receivedClaims())

	s.Require().Nil(svcErr)
	values := sessionValuesOf(claims)
	s.Require().NotNil(values)
	s.Equal(map[string]interface{}{"email": syncNewEmail, "nickname": "nick"}, values.Attributes)
	s.Equal([]string{"group-a"}, values.GroupIDs)
	s.Equal([]string{"group-a", "group-parent"}, values.GroupIDsWithAncestors)
	s.Equal([]string{"role-a"}, values.RoleIDs)
	s.entities.AssertNotCalled(s.T(), "UpdateAttributes", mock.Anything, mock.Anything, mock.Anything)
	s.groupSvc.AssertNotCalled(s.T(), "AddMemberToExistingGroups", mock.Anything, mock.Anything, mock.Anything)
}

// A declarative-only group or role store holds no memberships, so the mapped ones go to the session.
func (s *FederatedSyncTestSuite) TestDeferredSync_DeclarativeStoresKeepGroupsAndRolesToTheSession() {
	cfg := config.Config{}
	cfg.Group.Store = "declarative"
	cfg.Role.Store = "declarative"
	s.initRuntime(cfg)
	s.expectConnection(connection(providers.SyncModeNoSync, providers.SyncModeOverrideAll))
	s.expectHolder(map[string]interface{}{})
	s.entities.On("GetTransitiveGroupAncestors", mock.Anything, "group-a").Return([]string{}, noEPError)

	_, claims, svcErr := s.runSync(false, defaultProviderName, receivedClaims())

	s.Require().Nil(svcErr)
	values := sessionValuesOf(claims)
	s.Equal([]string{"group-a"}, values.GroupIDs)
	s.Equal([]string{"role-a"}, values.RoleIDs)
}

// An account that doesn't hold the link isn't written, and the identity stays pending.
func (s *FederatedSyncTestSuite) TestDeferredSync_NonHolderKeepsTheIdentityPending() {
	s.expectConnection(connection(providers.SyncModeOverrideReceived, providers.SyncModeOverrideAll))
	s.entities.On("GetLinkedEntities", mock.Anything, syncIdpID, syncSub).
		Return([]*providers.Entity{{ID: "someone-else"}}, noEPError)

	authUser, claims, svcErr := s.runSync(false, defaultProviderName, receivedClaims())

	s.Require().Nil(svcErr)
	s.NotNil(pendingOf(authUser, defaultProviderName))
	s.Nil(sessionValuesOf(claims))
}

// With the entity provider disabled, the sync reads and writes nothing and still returns the session values.
func (s *FederatedSyncTestSuite) TestDeferredSync_DisabledEntityProviderSkipsTheStore() {
	s.expectConnection(connection(providers.SyncModeSessionOnly, providers.SyncModeSessionOnly))
	s.expectSchema()
	s.entities.On("GetLinkedEntities", mock.Anything, syncIdpID, syncSub).Return([]*providers.Entity(nil),
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeNotImplemented, "Not implemented", ""))

	authUser, claims, svcErr := s.runSync(false, defaultProviderName, receivedClaims())

	s.Require().Nil(svcErr)
	s.Nil(pendingOf(authUser, defaultProviderName))
	values := sessionValuesOf(claims)
	s.Require().NotNil(values)
	s.Equal(map[string]interface{}{"email": syncNewEmail, "nickname": "nick"}, values.Attributes)
	s.Equal([]string{"group-a"}, values.GroupIDsWithAncestors)
	s.Equal([]string{"role-a"}, values.RoleIDs)
	s.entities.AssertNotCalled(s.T(), "GetTransitiveGroupAncestors", mock.Anything, mock.Anything)
}

// Any other entity provider failure fails the sign-in.
func (s *FederatedSyncTestSuite) TestDeferredSync_EntityProviderFailureFails() {
	s.expectConnection(connection(providers.SyncModeOverrideReceived, providers.SyncModeOverrideAll))
	s.entities.On("GetLinkedEntities", mock.Anything, syncIdpID, syncSub).Return([]*providers.Entity(nil),
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeSystemError, "System error", ""))

	_, _, svcErr := s.runSync(false, defaultProviderName, receivedClaims())

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

// An Override all mapping whose direct lookups fail fails the sync before anything is read or written.
func (s *FederatedSyncTestSuite) TestDeferredSync_IncompleteOverrideAllMappingFails() {
	idpDTO := connection(providers.SyncModeOverrideReceived, providers.SyncModeOverrideAll)
	idpDTO.AttributeConfiguration.AuthorizationMapping.Direct = []providers.AuthorizationDirectMapping{
		{Claim: "groups", TargetType: providers.AuthorizationTargetGroup},
	}
	s.idpSvc.On("GetIdentityProvider", mock.Anything, syncIdpID).Return(idpDTO, noSvcError)
	s.idpSvc.On("ResolveDirectAuthorizationTargets", mock.Anything, idpDTO, mock.Anything).
		Return([]providers.AuthorizationTarget(nil), []providers.AuthorizationTarget(nil),
			&tidcommon.InternalServerError)

	_, _, svcErr := s.runSync(false, defaultProviderName, receivedClaims())

	s.Require().NotNil(svcErr)
	s.Equal(ErrorFederatedSyncFailed.Code, svcErr.Code)
	s.entities.AssertNotCalled(s.T(), "GetLinkedEntities", mock.Anything, mock.Anything, mock.Anything)
}

// A rejected group write fails the sync with the sync error, and a failed one with a server error.
func (s *FederatedSyncTestSuite) TestDeferredSync_FailedGroupWrite() {
	for _, tc := range []struct {
		name     string
		writeErr *tidcommon.ServiceError
		wantCode string
	}{
		{"rejected", &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "GRP-1"},
			ErrorFederatedSyncFailed.Code},
		{"failed", &tidcommon.InternalServerError, tidcommon.InternalServerError.Code},
	} {
		s.Run(tc.name, func() {
			s.SetupTest()
			s.expectConnection(connection(providers.SyncModeNoSync, providers.SyncModeOverrideAll))
			s.expectHolder(map[string]interface{}{})
			s.groupSvc.On("GetMemberGroupIDs", mock.Anything, member).Return([]string{}, noSvcError)
			s.groupSvc.On("AddMemberToExistingGroups", mock.Anything, member, mock.Anything).Return(tc.writeErr)

			_, _, svcErr := s.runSync(false, defaultProviderName, receivedClaims())

			s.Require().NotNil(svcErr)
			s.Equal(tc.wantCode, svcErr.Code)
		})
	}
}

// A value a returning account refuses, such as a unique one another user holds, is left out while the
// others are written.
func (s *FederatedSyncTestSuite) TestDeferredSync_RefusedValueIsSkipped() {
	idpDTO := connection(providers.SyncModeOverrideReceived, providers.SyncModeNoSync)
	mapping := &idpDTO.AttributeConfiguration.UserTypeAttributeMappings[0]
	mapping.Attributes = append(mapping.Attributes,
		providers.AttributeMapping{ExternalAttribute: "nickname", LocalAttribute: "nickname"})
	s.expectConnection(idpDTO)
	s.expectSchema()
	s.expectHolder(map[string]interface{}{})
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

	_, _, svcErr := s.runSync(false, defaultProviderName, receivedClaims())

	s.Require().Nil(svcErr)
	s.entities.AssertNumberOfCalls(s.T(), "UpdateAttributes", 3)
}

// A failed write the account didn't refuse is a server error.
func (s *FederatedSyncTestSuite) TestDeferredSync_AttributeWriteServerError() {
	s.expectConnection(connection(providers.SyncModeOverrideReceived, providers.SyncModeNoSync))
	s.expectSchema()
	s.expectHolder(map[string]interface{}{})
	s.entities.On("UpdateAttributes", runtimeCtx, syncUserID, mock.Anything).
		Return(entityprovider.NewEntityProviderError(entityprovider.ErrorCodeSystemError, "System error", ""))

	_, _, svcErr := s.runSync(false, defaultProviderName, receivedClaims())

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ServerErrorType, svcErr.Type)
}

// Override received leaves an attribute alone when its mapping finds no claim, even if a claim of the
// attribute's own name arrives.
func (s *FederatedSyncTestSuite) TestDeferredSync_OverrideReceivedIgnoresUnmappedSameNameClaim() {
	s.expectConnection(connection(providers.SyncModeOverrideReceived, providers.SyncModeNoSync))
	s.expectSchema()
	s.expectHolder(map[string]interface{}{"email": "old@example.com"})

	_, _, svcErr := s.runSync(false, defaultProviderName,
		map[string]interface{}{"sub": syncSub, "email": "raw@example.com"})

	s.Require().Nil(svcErr)
	s.entities.AssertNotCalled(s.T(), "UpdateAttributes", mock.Anything, mock.Anything, mock.Anything)
}

// Without an entity provider, as in the embedded engine, no sign-in is synced.
func (s *FederatedSyncTestSuite) TestSignIn_NoEntityProviderSyncsNothing() {
	mgr, err := Initialize(s.provider, nil, newDiscardingBus(s.T()), FederatedSyncDependencies{})
	s.Require().NoError(err)
	s.mgr = mgr

	authUser, claims, svcErr := s.signIn(nil)

	s.Require().Nil(svcErr)
	s.Nil(sessionValuesOf(claims))
	s.Equal(identity(receivedClaims()), pendingOf(authUser, defaultProviderName))
}
