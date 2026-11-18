// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package defaultprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	authncommon "github.com/thunder-id/thunderid/internal/authn/common"
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entitytype"
	entitytypemodel "github.com/thunder-id/thunderid/internal/entitytype/model"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/tests/mocks/attributeverificationmock"
	"github.com/thunder-id/thunderid/tests/mocks/authn/commonmock"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/groupmock"
	"github.com/thunder-id/thunderid/tests/mocks/identitygovernancemock"
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

type FederatedSyncTestSuite struct {
	suite.Suite
	entitySvc     *entitymock.EntityServiceInterfaceMock
	entityTypeSvc *entitytypemock.EntityTypeServiceInterfaceMock
	idpSvc        *idpprovidermock.IDPProviderMock
	groupSvc      *groupmock.GroupServiceInterfaceMock
	roleSvc       *rolemock.RoleAssignmentServiceInterfaceMock
	federated     *commonmock.FederatedAuthenticatorMock
	provider      providers.AuthnProviderInterface
}

func TestFederatedSyncTestSuite(t *testing.T) {
	suite.Run(t, new(FederatedSyncTestSuite))
}

func (suite *FederatedSyncTestSuite) SetupTest() {
	suite.initRuntime(config.Config{})
	suite.entitySvc = entitymock.NewEntityServiceInterfaceMock(suite.T())
	suite.entityTypeSvc = entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	suite.idpSvc = idpprovidermock.NewIDPProviderMock(suite.T())
	suite.groupSvc = groupmock.NewGroupServiceInterfaceMock(suite.T())
	suite.roleSvc = rolemock.NewRoleAssignmentServiceInterfaceMock(suite.T())
	suite.federated = commonmock.NewFederatedAuthenticatorMock(suite.T())
	verification := attributeverificationmock.NewAttributeVerificationServiceInterfaceMock(suite.T())
	verification.On("GetVerifiedAttributes", mock.Anything, mock.Anything).
		Return(map[string]providers.VerifiedAttribute{}).Maybe()
	suite.provider = Initialize(suite.entitySvc, nil, nil, nil, nil,
		map[providers.IDPType]authncommon.FederatedAuthenticator{providers.IDPTypeOAuth: suite.federated},
		suite.admitAll(), verification,
		FederatedSyncDependencies{
			EntityTypeService:     suite.entityTypeSvc,
			IDPService:            suite.idpSvc,
			GroupService:          suite.groupSvc,
			RoleAssignmentService: suite.roleSvc,
		})
}

func (suite *FederatedSyncTestSuite) admitAll() *identitygovernancemock.IdentityGovernanceServiceInterfaceMock {
	governance := identitygovernancemock.NewIdentityGovernanceServiceInterfaceMock(suite.T())
	governance.EXPECT().AdmitAuthenticationStep(mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AccessDecision{Admitted: true}, nil).Maybe()
	return governance
}

func (suite *FederatedSyncTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (suite *FederatedSyncTestSuite) initRuntime(cfg config.Config) {
	config.ResetServerRuntime()
	suite.Require().NoError(config.InitializeServerRuntime(suite.T().TempDir(), &cfg))
}

// connection returns a connection mapping the provider's mail claim onto email for the user type, with a
// rule granting group-a and role-a to members of the "admins" team.
func connection(attributes, groupsAndRoles providers.SyncMode) *providers.IDPDTO {
	return &providers.IDPDTO{
		ID: syncIdpID,
		AttributeConfiguration: &providers.AttributeConfiguration{
			UserTypeResolution: &providers.UserTypeResolution{Default: syncUserType},
			UserTypeAttributeMappings: []providers.UserTypeAttributeMapping{{
				UserType: syncUserType,
				Attributes: []providers.AttributeMapping{
					{ExternalAttribute: "mail", LocalAttribute: "email"},
				},
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

func (suite *FederatedSyncTestSuite) expectConnection(idp *providers.IDPDTO) {
	suite.idpSvc.On("GetIdentityProvider", mock.Anything, syncIdpID).Return(idp, (*tidcommon.ServiceError)(nil))
	suite.idpSvc.On("ResolveDirectAuthorizationTargets", mock.Anything, idp, mock.Anything).
		Return([]providers.AuthorizationTarget(nil), []providers.AuthorizationTarget(nil),
			(*tidcommon.ServiceError)(nil)).Maybe()
}

func (suite *FederatedSyncTestSuite) expectSchema() {
	suite.entityTypeSvc.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, syncUserType,
		entitytype.AttributeFilter{AllowNonCredential: true}).
		Return([]entitytype.AttributeInfo{
			{Attribute: "email", Type: entitytypemodel.TypeString},
			{Attribute: "nickname", Type: entitytypemodel.TypeString},
			{Attribute: "iss", Type: entitytypemodel.TypeString},
		}, (*tidcommon.ServiceError)(nil))
}

const holderSystemAttributes = `{"linkedIds":{"` + syncIdpID + `":{"` + syncSub + `":{}}}}`

// expectHolder makes userID the one holder of the link, with the stored attributes.
func (suite *FederatedSyncTestSuite) expectHolder(stored map[string]interface{}) {
	suite.expectHolderWithSystem(stored, holderSystemAttributes)
}

// expectHolderWithSystem makes userID the one holder of the link, with the stored attributes and system
// attributes.
func (suite *FederatedSyncTestSuite) expectHolderWithSystem(stored map[string]interface{}, system string) {
	encoded, err := json.Marshal(stored)
	suite.Require().NoError(err)
	holder := providers.Entity{ID: syncUserID, Category: providers.EntityCategoryUser, Type: syncUserType,
		OUID: syncOUID, Attributes: encoded, SystemAttributes: json.RawMessage(system)}
	suite.entitySvc.On("ResolveLinkedAccounts", mock.Anything, syncIdpID, syncSub).
		Return([]string{syncUserID}, nil).Maybe()
	suite.entitySvc.On("GetEntityProfile", mock.Anything, syncUserID).Return(&holder, nil)
}

func (suite *FederatedSyncTestSuite) signIn(metadata *providers.AuthnMetadata) (
	*providers.AuthnResult, *tidcommon.ServiceError) {
	return suite.signInWithContext(context.Background(), metadata)
}

func (suite *FederatedSyncTestSuite) signInWithContext(ctx context.Context, metadata *providers.AuthnMetadata) (
	*providers.AuthnResult, *tidcommon.ServiceError) {
	suite.federated.On("Authenticate", mock.Anything, syncIdpID, mock.Anything).Return(&authncommon.AuthnResult{
		Token: map[string]interface{}{
			authnprovidercm.UserAttributeFederatedIdpID: syncIdpID,
			authnprovidercm.UserAttributeSub:            syncSub,
		},
		AuthenticatedClaims: map[string]interface{}{"sub": syncSub},
		ReceivedClaims:      receivedClaims(),
	}, (*tidcommon.ServiceError)(nil))
	return suite.provider.Authenticate(ctx, nil, map[string]interface{}{
		authnprovidercm.CredentialTypeFederated: &authncommon.FederatedAuthCredential{
			IDPID: syncIdpID, IDPType: providers.IDPTypeOAuth,
			AuthorizationData: authncommon.AuthorizationData{Code: "code"},
		},
	}, metadata)
}

func (suite *FederatedSyncTestSuite) runSync(created bool) (*providers.AuthnResult, *tidcommon.ServiceError) {
	return suite.runSyncWithClaims(created, receivedClaims())
}

func (suite *FederatedSyncTestSuite) runSyncWithClaims(created bool, claims map[string]interface{}) (
	*providers.AuthnResult, *tidcommon.ServiceError) {
	return suite.provider.Authenticate(context.Background(), nil,
		map[string]interface{}{authnprovidercm.CredentialTypeProvisionedEntityID: syncUserID},
		&providers.AuthnMetadata{
			PendingIdentity: map[string]interface{}{"idpId": syncIdpID, "sub": syncSub, "claims": claims},
			Created:         created,
		})
}

func sessionValuesOf(result *providers.AuthnResult) *authnprovidercm.SessionValues {
	values, _ := result.AuthenticatedClaims[authnprovidercm.ClaimSessionValues].(*authnprovidercm.SessionValues)
	return values
}

// An authentication flow syncs the one holder at sign-in: Override received writes the mapped value
// that differs, and nothing stays pending.
func (suite *FederatedSyncTestSuite) TestSignIn_AuthenticationSyncsTheHolder() {
	suite.expectConnection(connection(providers.SyncModeOverrideReceived, providers.SyncModeNoSync))
	suite.expectSchema()
	suite.expectHolder(map[string]interface{}{"email": "old@example.com", "nickname": "kept"})
	suite.entitySvc.On("UpdateAttributes", mock.Anything, syncUserID, mock.MatchedBy(func(raw json.RawMessage) bool {
		var written map[string]interface{}
		return json.Unmarshal(raw, &written) == nil && written["email"] == syncNewEmail &&
			written["nickname"] == "kept"
	})).Return(nil).Once()

	result, svcErr := suite.signIn(&providers.AuthnMetadata{FlowType: providers.FlowTypeAuthentication})

	suite.Require().Nil(svcErr)
	suite.Equal(syncUserID, result.EntityReference.EntityID)
	suite.Nil(result.PendingIdentity)
	values := sessionValuesOf(result)
	suite.Require().NotNil(values)
	suite.Equal([]providers.AuthorizationTarget{
		{Type: providers.AuthorizationTargetPermission, ResourceServerID: "rs-1", Permission: "read"},
	}, values.Permissions)
	suite.Equal("sub-1", result.AuthenticatedClaims["sub"])
}

// A stored number the sync doesn't change is written back exactly, even beyond float64 precision.
func (suite *FederatedSyncTestSuite) TestSignIn_KeepsUnchangedNumbersExact() {
	suite.expectConnection(connection(providers.SyncModeOverrideReceived, providers.SyncModeNoSync))
	suite.expectSchema()
	suite.expectHolder(map[string]interface{}{
		"email": "old@example.com", "employeeNo": json.Number("9007199254740993"),
	})
	suite.entitySvc.On("UpdateAttributes", mock.Anything, syncUserID, mock.MatchedBy(func(raw json.RawMessage) bool {
		return bytes.Contains(raw, []byte(`"employeeNo":9007199254740993`)) && bytes.Contains(raw, []byte(syncNewEmail))
	})).Return(nil).Once()

	_, svcErr := suite.signIn(&providers.AuthnMetadata{FlowType: providers.FlowTypeAuthentication})

	suite.Require().Nil(svcErr)
}

// A registration resolves the holder but leaves the sync to the existing-user rules.
func (suite *FederatedSyncTestSuite) TestSignIn_RegistrationDefersTheSync() {
	suite.entitySvc.On("ResolveLinkedAccounts", mock.Anything, syncIdpID, syncSub).Return([]string{syncUserID}, nil)
	suite.entitySvc.On("GetEntityProfile", mock.Anything, syncUserID).Return(&providers.Entity{
		ID: syncUserID, Category: providers.EntityCategoryUser, Type: syncUserType,
		SystemAttributes: json.RawMessage(holderSystemAttributes)}, nil)

	result, svcErr := suite.signIn(&providers.AuthnMetadata{FlowType: providers.FlowTypeRegistration})

	suite.Require().Nil(svcErr)
	suite.Equal(syncUserID, result.EntityReference.EntityID)
	suite.Equal(&pendingIdentity{IdpID: syncIdpID, Sub: syncSub, Claims: receivedClaims()}, result.PendingIdentity)
	suite.Nil(sessionValuesOf(result))
	suite.entitySvc.AssertNotCalled(suite.T(), "UpdateAttributes", mock.Anything, mock.Anything, mock.Anything)
}

// A sign-in no user holds yet is returned as the pending identity.
func (suite *FederatedSyncTestSuite) TestSignIn_NoHolderKeepsTheSyncPending() {
	suite.entitySvc.On("ResolveLinkedAccounts", mock.Anything, syncIdpID, syncSub).Return([]string{}, nil)

	result, svcErr := suite.signIn(nil)

	suite.Require().Nil(svcErr)
	suite.Nil(result.EntityReference)
	suite.Equal(&pendingIdentity{IdpID: syncIdpID, Sub: syncSub, Claims: receivedClaims()}, result.PendingIdentity)
}

// The sync at creation leaves the attributes Provisioning stored, and only adds the mapped groups and
// roles, even under Override all.
func (suite *FederatedSyncTestSuite) TestSyncLinkedAccount_CreatedOnlyAddsGroupsAndRoles() {
	suite.expectConnection(connection(providers.SyncModeOverrideReceived, providers.SyncModeOverrideAll))
	suite.expectHolder(map[string]interface{}{"email": "verified@example.com"})
	member := group.Member{ID: syncUserID, Type: group.MemberTypeUser}
	suite.groupSvc.On("AddMemberToExistingGroups", mock.Anything, member, []string{"group-a"}).
		Return((*tidcommon.ServiceError)(nil)).Once()
	suite.roleSvc.On("AddAssigneeToExistingRoles", mock.Anything,
		role.RoleAssignment{ID: syncUserID, Type: role.AssigneeTypeUser}, []string{"role-a"}).
		Return((*tidcommon.ServiceError)(nil)).Once()

	result, svcErr := suite.runSync(true)

	suite.Require().Nil(svcErr)
	suite.Nil(result.PendingIdentity)
	suite.entitySvc.AssertNotCalled(suite.T(), "UpdateAttributes", mock.Anything, mock.Anything, mock.Anything)
	suite.groupSvc.AssertNotCalled(suite.T(), "GetMemberGroupIDs", mock.Anything, mock.Anything)
	suite.groupSvc.AssertNotCalled(suite.T(), "RemoveGroupMembers", mock.Anything, mock.Anything, mock.Anything)
}

// Override all on a returning account replaces its direct groups and roles, removing the unmapped ones
// and keeping the ones the sync can't remove.
func (suite *FederatedSyncTestSuite) TestSyncLinkedAccount_OverrideAllReplaces() {
	suite.expectConnection(connection(providers.SyncModeNoSync, providers.SyncModeOverrideAll))
	suite.expectHolder(map[string]interface{}{})
	member := group.Member{ID: syncUserID, Type: group.MemberTypeUser}
	assignee := role.RoleAssignment{ID: syncUserID, Type: role.AssigneeTypeUser}
	suite.groupSvc.On("GetMemberGroupIDs", mock.Anything, member).
		Return([]string{"group-a", "group-old", "group-managed"}, (*tidcommon.ServiceError)(nil))
	suite.groupSvc.On("RemoveGroupMembers", mock.Anything, "group-old", []group.Member{member}).
		Return((*group.Group)(nil), (*tidcommon.ServiceError)(nil)).Once()
	suite.groupSvc.On("RemoveGroupMembers", mock.Anything, "group-managed", []group.Member{member}).
		Return((*group.Group)(nil), &group.ErrorManagedGroupMember).Once()
	suite.roleSvc.On("GetDirectRoleIDs", mock.Anything, syncUserID, syncOUID).
		Return([]string{"role-old"}, (*tidcommon.ServiceError)(nil))
	suite.roleSvc.On("AddAssigneeToExistingRoles", mock.Anything, assignee, []string{"role-a"}).
		Return((*tidcommon.ServiceError)(nil)).Once()
	suite.roleSvc.On("RemoveAssignments", mock.Anything, "role-old", syncOUID, []role.RoleAssignment{assignee}).
		Return((*tidcommon.ServiceError)(nil)).Once()

	result, svcErr := suite.runSync(false)

	suite.Require().Nil(svcErr)
	suite.Nil(result.PendingIdentity)
	suite.groupSvc.AssertNotCalled(suite.T(), "AddMemberToExistingGroups", mock.Anything, mock.Anything, mock.Anything)
	suite.entitySvc.AssertNotCalled(suite.T(), "UpdateAttributes", mock.Anything, mock.Anything, mock.Anything)
}

// Override all keeps the existing group membership and role assignment a direct mapping name matched
// ambiguously, and removes only the unmapped ones.
func (suite *FederatedSyncTestSuite) TestSyncLinkedAccount_OverrideAllKeepsAmbiguousMatches() {
	idpDTO := connection(providers.SyncModeNoSync, providers.SyncModeOverrideAll)
	suite.idpSvc.On("GetIdentityProvider", mock.Anything, syncIdpID).Return(idpDTO, (*tidcommon.ServiceError)(nil))
	suite.idpSvc.On("ResolveDirectAuthorizationTargets", mock.Anything, idpDTO, mock.Anything).
		Return([]providers.AuthorizationTarget(nil), []providers.AuthorizationTarget{
			{Type: providers.AuthorizationTargetGroup, ID: "group-ambiguous"},
			{Type: providers.AuthorizationTargetRole, ID: "role-ambiguous"},
		}, (*tidcommon.ServiceError)(nil)).Once()
	suite.expectHolder(map[string]interface{}{})
	member := group.Member{ID: syncUserID, Type: group.MemberTypeUser}
	assignee := role.RoleAssignment{ID: syncUserID, Type: role.AssigneeTypeUser}
	suite.groupSvc.On("GetMemberGroupIDs", mock.Anything, member).
		Return([]string{"group-a", "group-ambiguous", "group-old"}, (*tidcommon.ServiceError)(nil))
	suite.groupSvc.On("RemoveGroupMembers", mock.Anything, "group-old", []group.Member{member}).
		Return((*group.Group)(nil), (*tidcommon.ServiceError)(nil)).Once()
	suite.roleSvc.On("GetDirectRoleIDs", mock.Anything, syncUserID, syncOUID).
		Return([]string{"role-a", "role-ambiguous", "role-old"}, (*tidcommon.ServiceError)(nil))
	suite.roleSvc.On("RemoveAssignments", mock.Anything, "role-old", syncOUID, []role.RoleAssignment{assignee}).
		Return((*tidcommon.ServiceError)(nil)).Once()

	_, svcErr := suite.runSync(false)

	suite.Require().Nil(svcErr)
	suite.groupSvc.AssertNotCalled(suite.T(), "RemoveGroupMembers", mock.Anything, "group-ambiguous", mock.Anything)
	suite.roleSvc.AssertNotCalled(suite.T(), "RemoveAssignments", mock.Anything, "role-ambiguous", mock.Anything,
		mock.Anything)
}

// Session only stores nothing and returns the values for the session.
func (suite *FederatedSyncTestSuite) TestSyncLinkedAccount_SessionOnlyReturnsSessionValues() {
	suite.expectConnection(connection(providers.SyncModeSessionOnly, providers.SyncModeSessionOnly))
	suite.expectSchema()
	suite.expectHolder(map[string]interface{}{"email": "stored@example.com"})
	suite.entitySvc.On("GetTransitiveGroupAncestors", mock.Anything, "group-a").Return([]string{"group-parent"}, nil)

	result, svcErr := suite.runSync(true)

	suite.Require().Nil(svcErr)
	values := sessionValuesOf(result)
	suite.Require().NotNil(values)
	suite.Equal(map[string]interface{}{"email": syncNewEmail, "nickname": "nick"}, values.Attributes)
	suite.Equal([]string{"group-a"}, values.GroupIDs)
	suite.Equal([]string{"group-a", "group-parent"}, values.GroupIDsWithAncestors)
	suite.Equal([]string{"role-a"}, values.RoleIDs)
	suite.entitySvc.AssertNotCalled(suite.T(), "UpdateAttributes", mock.Anything, mock.Anything, mock.Anything)
	suite.groupSvc.AssertNotCalled(suite.T(), "AddMemberToExistingGroups", mock.Anything, mock.Anything, mock.Anything)
}

// A declarative-only group or role store holds no memberships, so the mapped ones go to the session.
func (suite *FederatedSyncTestSuite) TestSyncLinkedAccount_DeclarativeStoresKeepGroupsAndRolesToTheSession() {
	cfg := config.Config{}
	cfg.Group.Store = "declarative"
	cfg.Role.Store = "declarative"
	suite.initRuntime(cfg)
	suite.expectConnection(connection(providers.SyncModeNoSync, providers.SyncModeOverrideAll))
	suite.expectHolder(map[string]interface{}{})
	suite.entitySvc.On("GetTransitiveGroupAncestors", mock.Anything, "group-a").Return([]string{}, nil)

	result, svcErr := suite.runSync(false)

	suite.Require().Nil(svcErr)
	values := sessionValuesOf(result)
	suite.Equal([]string{"group-a"}, values.GroupIDs)
	suite.Equal([]string{"role-a"}, values.RoleIDs)
}

// No sync on a returning account writes nothing.
func (suite *FederatedSyncTestSuite) TestSyncLinkedAccount_NoSyncWritesNothing() {
	suite.expectConnection(connection(providers.SyncModeNoSync, providers.SyncModeNoSync))
	suite.expectHolder(map[string]interface{}{})

	result, svcErr := suite.runSync(false)

	suite.Require().Nil(svcErr)
	suite.Nil(result.PendingIdentity)
	suite.entitySvc.AssertNotCalled(suite.T(), "UpdateAttributes", mock.Anything, mock.Anything, mock.Anything)
}

// An account that doesn't hold the link isn't written, and the sign-in stays pending.
func (suite *FederatedSyncTestSuite) TestSyncLinkedAccount_NonHolderKeepsTheTokenPending() {
	suite.entitySvc.On("GetEntityProfile", mock.Anything, syncUserID).Return(&providers.Entity{
		ID: syncUserID, Category: providers.EntityCategoryUser, Type: syncUserType,
		SystemAttributes: json.RawMessage(`{"linkedIds":{"other-idp":{"` + syncSub + `":{}}}}`)}, nil)

	result, svcErr := suite.runSync(false)

	suite.Require().Nil(svcErr)
	suite.NotNil(result.PendingIdentity)
	suite.Nil(sessionValuesOf(result))
}

// Any other authentication hands the pending identity back unchanged.
func (suite *FederatedSyncTestSuite) TestOtherAuthentication_ReturnsThePendingToken() {
	suite.entitySvc.On("AuthenticateEntityByID", mock.Anything, syncUserID, mock.Anything).
		Return(&entity.AuthenticateResult{EntityID: syncUserID}, "", nil)
	suite.entitySvc.On("GetEntityProfile", mock.Anything, syncUserID).Return(&providers.Entity{
		ID: syncUserID, Category: providers.EntityCategoryUser, Type: syncUserType}, nil)
	pending := map[string]interface{}{"idpId": syncIdpID, "sub": syncSub}

	result, svcErr := suite.provider.Authenticate(context.Background(),
		map[string]interface{}{authnprovidercm.UserAttributeUserID: syncUserID},
		map[string]interface{}{"password": "secret"}, &providers.AuthnMetadata{PendingIdentity: pending})

	suite.Require().Nil(svcErr)
	suite.Equal(pending, result.PendingIdentity)
}

// An Override all mapping whose direct lookups fail fails the sync before anything is written.
func (suite *FederatedSyncTestSuite) TestSyncLinkedAccount_IncompleteOverrideAllMappingFails() {
	idp := connection(providers.SyncModeOverrideReceived, providers.SyncModeOverrideAll)
	idp.AttributeConfiguration.AuthorizationMapping.Direct = []providers.AuthorizationDirectMapping{
		{Claim: "groups", TargetType: providers.AuthorizationTargetGroup},
	}
	suite.idpSvc.On("GetIdentityProvider", mock.Anything, syncIdpID).Return(idp, (*tidcommon.ServiceError)(nil))
	suite.idpSvc.On("ResolveDirectAuthorizationTargets", mock.Anything, idp, mock.Anything).
		Return([]providers.AuthorizationTarget(nil), []providers.AuthorizationTarget(nil),
			&tidcommon.InternalServerError)
	suite.expectHolder(map[string]interface{}{})

	_, svcErr := suite.runSync(false)

	suite.Require().NotNil(svcErr)
	suite.Equal(authnprovidercm.ErrorCodeFederatedSyncFailed, svcErr.Code)
	suite.entitySvc.AssertNotCalled(suite.T(), "UpdateAttributes", mock.Anything, mock.Anything, mock.Anything)
}

// A rejected group write fails the sync with the sync error.
func (suite *FederatedSyncTestSuite) TestSyncLinkedAccount_RejectedGroupWriteFails() {
	suite.expectConnection(connection(providers.SyncModeNoSync, providers.SyncModeOverrideAll))
	suite.expectHolder(map[string]interface{}{})
	suite.groupSvc.On("GetMemberGroupIDs", mock.Anything, mock.Anything).
		Return([]string{}, (*tidcommon.ServiceError)(nil))
	suite.groupSvc.On("AddMemberToExistingGroups", mock.Anything, mock.Anything, mock.Anything).
		Return(&group.ErrorGroupNotFound)

	_, svcErr := suite.runSync(false)

	suite.Require().NotNil(svcErr)
	suite.Equal(authnprovidercm.ErrorCodeFederatedSyncFailed, svcErr.Code)
}

// A value a returning account refuses, such as a unique one another user holds, is left out while the
// others are written.
func (suite *FederatedSyncTestSuite) TestSyncLinkedAccount_RefusedValueIsSkipped() {
	idpDTO := connection(providers.SyncModeOverrideReceived, providers.SyncModeNoSync)
	mapping := &idpDTO.AttributeConfiguration.UserTypeAttributeMappings[0]
	mapping.Attributes = append(mapping.Attributes,
		providers.AttributeMapping{ExternalAttribute: "nickname", LocalAttribute: "nickname"})
	suite.expectConnection(idpDTO)
	suite.expectSchema()
	suite.expectHolder(map[string]interface{}{})
	written := func(want map[string]interface{}) interface{} {
		return mock.MatchedBy(func(raw json.RawMessage) bool {
			var got map[string]interface{}
			return json.Unmarshal(raw, &got) == nil && len(got) == len(want) &&
				got["email"] == want["email"] && got["nickname"] == want["nickname"]
		})
	}
	// The whole set and the email alone are refused, so only the nickname is written.
	suite.entitySvc.On("UpdateAttributes", mock.Anything, syncUserID,
		written(map[string]interface{}{"email": syncNewEmail, "nickname": "nick"})).
		Return(entity.ErrAttributeConflict).Once()
	suite.entitySvc.On("UpdateAttributes", mock.Anything, syncUserID,
		written(map[string]interface{}{"email": syncNewEmail})).Return(entity.ErrAttributeConflict).Once()
	suite.entitySvc.On("UpdateAttributes", mock.Anything, syncUserID,
		written(map[string]interface{}{"nickname": "nick"})).Return(nil).Once()

	_, svcErr := suite.runSync(false)

	suite.Require().Nil(svcErr)
	suite.entitySvc.AssertNumberOfCalls(suite.T(), "UpdateAttributes", 3)
}

// A failed write the account didn't refuse is a server error.
func (suite *FederatedSyncTestSuite) TestSyncLinkedAccount_AttributeWriteServerError() {
	suite.expectConnection(connection(providers.SyncModeOverrideReceived, providers.SyncModeNoSync))
	suite.expectSchema()
	suite.expectHolder(map[string]interface{}{})
	suite.entitySvc.On("UpdateAttributes", mock.Anything, syncUserID, mock.Anything).Return(errors.New("db down"))

	_, svcErr := suite.runSync(false)

	suite.Require().NotNil(svcErr)
	suite.Equal(tidcommon.ServerErrorType, svcErr.Type)
}

func (suite *FederatedSyncTestSuite) TestDecodePendingIdentity() {
	suite.Nil(decodePendingIdentity(nil))
	suite.Nil(decodePendingIdentity(map[string]interface{}{"idpId": syncIdpID}))
	suite.Equal(&pendingIdentity{IdpID: syncIdpID, Sub: syncSub},
		decodePendingIdentity(&pendingIdentity{IdpID: syncIdpID, Sub: syncSub}))
}

// An account whose system attributes also hold scalar values, such as the time its credential last
// changed, is still recognized as the holder and synced.
func (suite *FederatedSyncTestSuite) TestSyncLinkedAccount_HolderWithScalarSystemAttributes() {
	suite.expectConnection(connection(providers.SyncModeOverrideReceived, providers.SyncModeNoSync))
	suite.expectSchema()
	suite.expectHolderWithSystem(map[string]interface{}{"email": "old@example.com"},
		`{"credentialUpdatedAt":"2026-01-01T00:00:00Z","linkedIds":{"`+syncIdpID+`":{"`+syncSub+`":{}}}}`)
	suite.entitySvc.On("UpdateAttributes", mock.Anything, syncUserID, mock.MatchedBy(func(raw json.RawMessage) bool {
		var written map[string]interface{}
		return json.Unmarshal(raw, &written) == nil && written["email"] == syncNewEmail
	})).Return(nil).Once()

	result, svcErr := suite.runSync(false)

	suite.Require().Nil(svcErr)
	suite.NotNil(sessionValuesOf(result))
}

// Attributes stored as null are read as an empty profile.
func (suite *FederatedSyncTestSuite) TestSyncLinkedAccount_NullStoredAttributes() {
	suite.expectConnection(connection(providers.SyncModeOverrideReceived, providers.SyncModeNoSync))
	suite.expectSchema()
	suite.expectHolder(nil)
	suite.entitySvc.On("UpdateAttributes", mock.Anything, syncUserID, mock.MatchedBy(func(raw json.RawMessage) bool {
		var written map[string]interface{}
		return json.Unmarshal(raw, &written) == nil && written["email"] == syncNewEmail
	})).Return(nil).Once()

	_, svcErr := suite.runSync(false)

	suite.Require().Nil(svcErr)
}

// Override received leaves an attribute alone when its mapping finds no claim, even if a claim of the
// attribute's own name arrives.
func (suite *FederatedSyncTestSuite) TestSyncLinkedAccount_OverrideReceivedIgnoresUnmappedSameNameClaim() {
	suite.expectConnection(connection(providers.SyncModeOverrideReceived, providers.SyncModeNoSync))
	suite.expectSchema()
	suite.expectHolder(map[string]interface{}{"email": "old@example.com"})

	_, svcErr := suite.runSyncWithClaims(false, map[string]interface{}{"sub": syncSub, "email": "raw@example.com"})

	suite.Require().Nil(svcErr)
	suite.entitySvc.AssertNotCalled(suite.T(), "UpdateAttributes", mock.Anything, mock.Anything, mock.Anything)
}

// A holder the application doesn't accept as its subject isn't synced, and the sign-in stays pending.
func (suite *FederatedSyncTestSuite) TestSignIn_RefusedSubjectIsNotSynced() {
	suite.expectHolder(map[string]interface{}{"email": "old@example.com"})
	ctx := authnprovidercm.WithSubjectTypeConstraints(context.Background(),
		authnprovidercm.SubjectTypeConstraints{RequiredSubjectID: "another-entity"})

	result, svcErr := suite.signInWithContext(ctx, &providers.AuthnMetadata{
		FlowType: providers.FlowTypeAuthentication})

	suite.Require().Nil(svcErr)
	suite.NotNil(result.PendingIdentity)
	suite.Nil(sessionValuesOf(result))
	suite.idpSvc.AssertNotCalled(suite.T(), "GetIdentityProvider", mock.Anything, mock.Anything)
}
