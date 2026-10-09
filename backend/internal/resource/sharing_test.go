// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/config"
	syscontext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
	"github.com/thunder-id/thunderid/tests/mocks/sharingmock"
)

const (
	sharedRSID    = "rs-shared"
	sharedRSOwner = "ou-owner"
	sharedRSOU    = "ou-sharee"
)

// ResourceServerSharingTestSuite covers how a resource server participates in sharing: what it
// declares to the framework, the rule shapes it refuses, and which of its permissions an
// organization unit may use as a result.
type ResourceServerSharingTestSuite struct {
	suite.Suite
	mockStore   *resourceStoreInterfaceMock
	mockSharing *sharingmock.SharingServiceInterfaceMock
	service     ResourceServiceInterface
}

func TestResourceServerSharingTestSuite(t *testing.T) {
	suite.Run(t, new(ResourceServerSharingTestSuite))
}

func (s *ResourceServerSharingTestSuite) SetupTest() {
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", &config.Config{
		Server: engineconfig.ServerConfig{Identifier: "test-deployment"},
	})
	require.NoError(s.T(), err)
	defer config.ResetServerRuntime()

	s.mockStore = newResourceStoreInterfaceMock(s.T())
	s.mockSharing = sharingmock.NewSharingServiceInterfaceMock(s.T())
	s.service, err = newResourceService(
		new(oumock.OrganizationUnitServiceInterfaceMock), s.mockStore, &fakeTransactioner{}, nil, s.mockSharing)
	s.Require().NoError(err)
}

// sharedServer stubs the store with a resource server owned by sharedRSOwner whose permissions all
// exist, so a test isolates the organization unit's entitlement.
func (s *ResourceServerSharingTestSuite) sharedServer(permissions []string) {
	s.mockStore.On("GetResourceServer", mock.Anything, sharedRSID).Return(providers.ResourceServer{
		ID: sharedRSID, OUID: sharedRSOwner, Delimiter: ":",
	}, nil)
	s.mockStore.On("ValidatePermissions", mock.Anything, sharedRSID, permissions).Return([]string{}, nil)
}

// resolvesTo stubs what the framework resolves for the sharee.
func (s *ResourceServerSharingTestSuite) resolvesTo(overlay sharing.ResolvedOverlay) {
	s.mockSharing.EXPECT().ResolveOverlayRules(mock.Anything, ResourceServerSharingType, sharedRSID, sharedRSOU).
		Return(overlay, nil)
}

// hasPolicies stubs whether any sharing policy names the server at all. A server with none is open
// to every organization unit, so a test about a gated server has to give it one.
func (s *ResourceServerSharingTestSuite) hasPolicies(policies ...sharing.Policy) {
	s.mockSharing.EXPECT().ListPolicies(mock.Anything, ResourceServerSharingType, sharedRSID).
		Return(policies, nil)
}

func ptr(v ...string) *[]string {
	return &v
}

// asOU is a context naming ouID as the accessing organization unit.
func asOU(ouID string) context.Context {
	return syscontext.WithAccessingOUID(context.Background(), ouID)
}

// matchesOU matches a context naming ouID as the accessing organization unit.
func matchesOU(ouID string) interface{} {
	return mock.MatchedBy(func(ctx context.Context) bool {
		return syscontext.GetAccessingOUID(ctx) == ouID
	})
}

// The owner may use every permission it defines, and the framework is not consulted for it.
func (s *ResourceServerSharingTestSuite) TestTheOwnerMayUseEveryPermission() {
	perms := []string{"orders:read", "orders:delete"}
	s.sharedServer(perms)

	invalid, svcErr := s.service.ValidatePermissions(asOU(sharedRSOwner), sharedRSID, perms)

	s.Nil(svcErr)
	s.Empty(invalid)
}

// With no accessing organization unit on the context, only definition is checked, as it was before
// sharing existed.
func (s *ResourceServerSharingTestSuite) TestNoOrganizationUnitChecksDefinitionAlone() {
	perms := []string{"orders:read"}
	s.sharedServer(perms)

	invalid, svcErr := s.service.ValidatePermissions(context.Background(), sharedRSID, perms)

	s.Nil(svcErr)
	s.Empty(invalid)
}

// Once a server has a policy, a unit no policy reaches may use none of it: entitlement is strict.
func (s *ResourceServerSharingTestSuite) TestAUnitTheServerDoesNotReachMayUseNothing() {
	perms := []string{"orders:read", "reports:view"}
	s.sharedServer(perms)
	s.resolvesTo(sharing.ResolvedOverlay{OUID: sharedRSOU, Visible: false})
	s.hasPolicies(sharing.Policy{ID: "policy-1"})

	invalid, svcErr := s.service.ValidatePermissions(asOU(sharedRSOU), sharedRSID, perms)

	s.Nil(svcErr)
	s.ElementsMatch(perms, invalid)
}

// A value names what is shared: the paths it lists and everything beneath them.
func (s *ResourceServerSharingTestSuite) TestAValueSharesWhatItNames() {
	perms := []string{"orders", "orders:read", "reports:view", "audit:read"}
	s.sharedServer(perms)
	s.resolvesTo(sharing.ResolvedOverlay{OUID: sharedRSOU, Visible: true, Rules: map[string]sharing.OverlayRule{
		SharingFieldPermissions: {Value: ptr("orders", "reports:view")},
	}})

	invalid, svcErr := s.service.ValidatePermissions(asOU(sharedRSOU), sharedRSID, perms)

	s.Nil(svcErr)
	s.Equal([]string{"audit:read"}, invalid)
}

// Excluded values name what is withheld, with everything else shared.
func (s *ResourceServerSharingTestSuite) TestExcludedValuesWithholdWhatTheyName() {
	perms := []string{"orders:read", "orders:delete", "audit", "audit:read"}
	s.sharedServer(perms)
	s.resolvesTo(sharing.ResolvedOverlay{OUID: sharedRSOU, Visible: true, Rules: map[string]sharing.OverlayRule{
		SharingFieldPermissions: {ExcludedValues: ptr("orders:delete", "audit")},
	}})

	invalid, svcErr := s.service.ValidatePermissions(asOU(sharedRSOU), sharedRSID, perms)

	s.Nil(svcErr)
	s.Equal([]string{"orders:delete", "audit", "audit:read"}, invalid)
}

// A policy naming no rule answers through the declared default, which shares the whole server.
func (s *ResourceServerSharingTestSuite) TestThePolicyDefaultSharesTheWholeServer() {
	perms := []string{"orders:read", "audit:read"}
	s.sharedServer(perms)
	s.resolvesTo(sharing.ResolvedOverlay{OUID: sharedRSOU, Visible: true, Rules: map[string]sharing.OverlayRule{
		SharingFieldPermissions: *newResourceServerSharingDeclaration(nil).Fields()[0].Default,
	}})

	invalid, svcErr := s.service.ValidatePermissions(asOU(sharedRSOU), sharedRSID, perms)

	s.Nil(svcErr)
	s.Empty(invalid)
}

// Undefined permissions stay invalid, and are not asked about twice.
func (s *ResourceServerSharingTestSuite) TestUndefinedPermissionsAreReportedOnce() {
	perms := []string{"orders:read", "ghost"}
	s.mockStore.On("GetResourceServer", mock.Anything, sharedRSID).Return(providers.ResourceServer{
		ID: sharedRSID, OUID: sharedRSOwner, Delimiter: ":",
	}, nil)
	s.mockStore.On("ValidatePermissions", mock.Anything, sharedRSID, perms).Return([]string{"ghost"}, nil)
	s.resolvesTo(sharing.ResolvedOverlay{OUID: sharedRSOU, Visible: false})
	s.hasPolicies(sharing.Policy{ID: "policy-1"})

	invalid, svcErr := s.service.ValidatePermissions(asOU(sharedRSOU), sharedRSID, perms)

	s.Nil(svcErr)
	s.Equal([]string{"ghost", "orders:read"}, invalid)
}

// A framework that cannot answer is an internal failure, never a quiet "available".
func (s *ResourceServerSharingTestSuite) TestAFailedResolutionIsAnInternalError() {
	perms := []string{"orders:read"}
	s.sharedServer(perms)
	s.mockSharing.EXPECT().ResolveOverlayRules(mock.Anything, ResourceServerSharingType, sharedRSID, sharedRSOU).
		Return(sharing.ResolvedOverlay{}, &tidcommon.InternalServerError)

	invalid, svcErr := s.service.ValidatePermissions(asOU(sharedRSOU), sharedRSID, perms)

	s.Nil(invalid)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

// A server no policy names is open to every organization unit, as it was before sharing existed.
// This holds until resource servers can be shared through the API, since until then a server not
// loaded from a declarative file could never be given a policy.
func (s *ResourceServerSharingTestSuite) TestAServerNoPolicyNamesIsOpenToEveryUnit() {
	perms := []string{"orders:read", "audit:read"}
	s.sharedServer(perms)
	s.resolvesTo(sharing.ResolvedOverlay{OUID: sharedRSOU, Visible: false})
	s.hasPolicies()

	invalid, svcErr := s.service.ValidatePermissions(asOU(sharedRSOU), sharedRSID, perms)

	s.Nil(svcErr)
	s.Empty(invalid)
}

// A unit the server is shared with is answered by its rule, without listing the server's policies.
func (s *ResourceServerSharingTestSuite) TestASharedUnitDoesNotListThePolicies() {
	perms := []string{"orders:read"}
	s.sharedServer(perms)
	s.resolvesTo(sharing.ResolvedOverlay{OUID: sharedRSOU, Visible: true, Rules: map[string]sharing.OverlayRule{
		SharingFieldPermissions: {},
	}})

	_, svcErr := s.service.ValidatePermissions(asOU(sharedRSOU), sharedRSID, perms)

	s.Nil(svcErr)
	s.mockSharing.AssertNotCalled(s.T(), "ListPolicies", mock.Anything, mock.Anything, mock.Anything)
}

// Not knowing whether a server has policies is an internal failure, never a quiet "open".
func (s *ResourceServerSharingTestSuite) TestAFailedPolicyListingIsAnInternalError() {
	perms := []string{"orders:read"}
	s.sharedServer(perms)
	s.resolvesTo(sharing.ResolvedOverlay{OUID: sharedRSOU, Visible: false})
	s.mockSharing.EXPECT().ListPolicies(mock.Anything, ResourceServerSharingType, sharedRSID).
		Return(nil, &tidcommon.InternalServerError)

	invalid, svcErr := s.service.ValidatePermissions(asOU(sharedRSOU), sharedRSID, perms)

	s.Nil(invalid)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

// Without the framework no policy can name the server, so it is open like any unshared one.
func (s *ResourceServerSharingTestSuite) TestWithoutTheFrameworkTheServerIsOpen() {
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime("/tmp/test", &config.Config{
		Server: engineconfig.ServerConfig{Identifier: "test-deployment"},
	}))
	defer config.ResetServerRuntime()
	svc, err := newResourceService(
		new(oumock.OrganizationUnitServiceInterfaceMock), s.mockStore, &fakeTransactioner{}, nil, nil)
	s.Require().NoError(err)
	perms := []string{"orders:read"}
	s.sharedServer(perms)

	invalid, svcErr := svc.ValidatePermissions(asOU(sharedRSOU), sharedRSID, perms)

	s.Nil(svcErr)
	s.Empty(invalid)
}

// ResourceServerSharingDeclarationTestSuite covers what a resource server declares to the sharing
// framework.
type ResourceServerSharingDeclarationTestSuite struct {
	suite.Suite
	mockService *ResourceServiceInterfaceMock
	decl        *resourceServerSharingDeclaration
}

func TestResourceServerSharingDeclarationTestSuite(t *testing.T) {
	suite.Run(t, new(ResourceServerSharingDeclarationTestSuite))
}

func (s *ResourceServerSharingDeclarationTestSuite) SetupTest() {
	s.mockService = NewResourceServiceInterfaceMock(s.T())
	s.decl = newResourceServerSharingDeclaration(s.mockService)
}

func (s *ResourceServerSharingDeclarationTestSuite) TestResourceType() {
	s.Equal(ResourceServerSharingType, s.decl.ResourceType())
}

// One hierarchical field, defaulting to the whole server and pinned.
func (s *ResourceServerSharingDeclarationTestSuite) TestFields() {
	fields := s.decl.Fields()

	s.Require().Len(fields, 1)
	s.Equal(SharingFieldPermissions, fields[0].Key)
	s.Equal(sharing.FieldHierarchy, fields[0].Kind)
	s.Require().NotNil(fields[0].Default)
	s.Equal(sharing.OverlayRule{Editable: false}, *fields[0].Default)
}

func (s *ResourceServerSharingDeclarationTestSuite) TestOwningOUIDAndDelimiterComeFromTheServer() {
	s.mockService.EXPECT().GetResourceServer(mock.Anything, sharedRSID).
		Return(&providers.ResourceServer{ID: sharedRSID, OUID: sharedRSOwner, Delimiter: "/"}, nil)

	owner, svcErr := s.decl.OwningOUID(context.Background(), sharedRSID)
	s.Nil(svcErr)
	s.Equal(sharedRSOwner, owner)

	delimiter, svcErr := s.decl.FieldDelimiter(context.Background(), sharedRSID, SharingFieldPermissions)
	s.Nil(svcErr)
	s.Equal("/", delimiter)
}

func (s *ResourceServerSharingDeclarationTestSuite) TestOwningOUIDAndDelimiterReportAMissingServer() {
	s.mockService.EXPECT().GetResourceServer(mock.Anything, sharedRSID).
		Return(nil, &ErrorResourceServerNotFound)

	_, svcErr := s.decl.OwningOUID(context.Background(), sharedRSID)
	s.Equal(&ErrorResourceServerNotFound, svcErr)

	_, svcErr = s.decl.FieldDelimiter(context.Background(), sharedRSID, SharingFieldPermissions)
	s.Equal(&ErrorResourceServerNotFound, svcErr)
}

// A rule names the shared permissions or the withheld ones. A menu, an editable rule, or both lists
// at once are refused, each with its own reason.
func (s *ResourceServerSharingDeclarationTestSuite) TestValidateOverlayRule() {
	cases := []struct {
		name string
		rule sharing.OverlayRule
		want *tidcommon.ServiceError
	}{
		{"no lists shares everything", sharing.OverlayRule{}, nil},
		{"value alone", sharing.OverlayRule{Value: ptr("orders")}, nil},
		{"excluded values alone", sharing.OverlayRule{ExcludedValues: ptr("audit")}, nil},
		{"allowed values", sharing.OverlayRule{AllowedValues: ptr("orders")},
			&ErrorSharingAllowedValuesNotSupported},
		{"an empty allowed values list", sharing.OverlayRule{AllowedValues: ptr()},
			&ErrorSharingAllowedValuesNotSupported},
		{"editable", sharing.OverlayRule{Editable: true}, &ErrorSharingEditableNotSupported},
		{"value with excluded values",
			sharing.OverlayRule{Value: ptr("orders"), ExcludedValues: ptr("orders:delete")},
			&ErrorSharingValueWithExcludedValues},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.Equal(tc.want, s.decl.ValidateOverlayRule(
				context.Background(), sharedRSID, SharingFieldPermissions, tc.rule))
		})
	}
}

// Members are checked against what the initiator itself may use, which is what stops a reshare
// naming a permission its own sharer withheld.
func (s *ResourceServerSharingDeclarationTestSuite) TestValidateMembersChecksTheInitiatorsEntitlement() {
	s.mockService.EXPECT().ValidatePermissions(matchesOU(sharedRSOU), sharedRSID, []string{"orders"}).
		Return([]string{}, nil).Once()
	s.mockService.EXPECT().ValidatePermissions(matchesOU(sharedRSOU), sharedRSID, []string{"audit"}).
		Return([]string{"audit"}, nil).Once()

	s.Nil(s.decl.ValidateMembers(
		context.Background(), sharedRSID, SharingFieldPermissions, sharedRSOU, []string{"orders"}))
	s.Equal(&ErrorSharingPermissionsNotAvailable, s.decl.ValidateMembers(
		context.Background(), sharedRSID, SharingFieldPermissions, sharedRSOU, []string{"audit"}))
}

func (s *ResourceServerSharingDeclarationTestSuite) TestValidateMembersPassesAFailureThrough() {
	s.mockService.EXPECT().ValidatePermissions(matchesOU(sharedRSOU), sharedRSID, []string{"orders"}).
		Return(nil, &tidcommon.InternalServerError)

	s.Equal(&tidcommon.InternalServerError, s.decl.ValidateMembers(
		context.Background(), sharedRSID, SharingFieldPermissions, sharedRSOU, []string{"orders"}))
}

// ResourceServerSharingParserTestSuite covers reading the sharing half of a resource server document.
type ResourceServerSharingParserTestSuite struct {
	suite.Suite
	mockService *ResourceServiceInterfaceMock
	parse       func([]byte) (*sharing.DeclaredResourcePolicies, error)
}

func TestResourceServerSharingParserTestSuite(t *testing.T) {
	suite.Run(t, new(ResourceServerSharingParserTestSuite))
}

func (s *ResourceServerSharingParserTestSuite) SetupTest() {
	s.mockService = NewResourceServiceInterfaceMock(s.T())
	s.parse = makeResourceServerSharingConfig(s.mockService).Parser
}

func (s *ResourceServerSharingParserTestSuite) TestConfigNamesTheResourceServersDirectory() {
	cfg := makeResourceServerSharingConfig(s.mockService)

	s.Equal(ResourceServerSharingType, cfg.ResourceType)
	s.Equal("resource_servers", cfg.DirectoryName)
}

// Most documents declare no policy, and those are read without resolving anything.
func (s *ResourceServerSharingParserTestSuite) TestADocumentWithoutPoliciesDeclaresNone() {
	declared, err := s.parse([]byte("id: rs-1\nname: RS\nidentifier: https://rs\nouId: ou-1\n"))

	s.Require().NoError(err)
	s.Equal(&sharing.DeclaredResourcePolicies{ResourceID: "rs-1"}, declared)
}

// The owner is resolved through the service, so a document naming its unit by handle declares the
// policy against the unit's id.
func (s *ResourceServerSharingParserTestSuite) TestPoliciesAreDeclaredAgainstTheResolvedOwner() {
	s.mockService.EXPECT().ResolveResourceServerOUHandle(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, rs *providers.ResourceServer) *tidcommon.ServiceError {
			rs.OUID = "ou-resolved"
			return nil
		})
	doc := `id: rs-1
name: Orders API
identifier: https://orders
ouHandle: tenants
sharingPolicies:
  - id: rs-1-policy
    targets:
      - scope: child
        ouId: gold
        overlayRules:
          permissions:
            value: [orders]
      - scope: child
        ouId: silver
        overlayRules:
          permissions:
            excludedValues: [orders:delete]
`

	declared, err := s.parse([]byte(doc))

	s.Require().NoError(err)
	s.Equal("rs-1", declared.ResourceID)
	s.Equal("Orders API", declared.ResourceName)
	s.Equal("ou-resolved", declared.OwningOUID)
	s.Require().Len(declared.Policies, 1)
	policy := declared.Policies[0]
	s.Equal("rs-1-policy", policy.ID)
	s.Require().Len(policy.Targets, 2)
	s.Equal(sharing.ScopeChild, policy.Targets[0].Scope)
	s.Equal("gold", policy.Targets[0].OUID)
	s.Equal([]string{"orders"}, *policy.Targets[0].OverlayRules[SharingFieldPermissions].Value)
	s.Equal([]string{"orders:delete"}, *policy.Targets[1].OverlayRules[SharingFieldPermissions].ExcludedValues)
}

func (s *ResourceServerSharingParserTestSuite) TestAnUnresolvableOwnerIsAnError() {
	s.mockService.EXPECT().ResolveResourceServerOUHandle(mock.Anything, mock.Anything).
		Return(&ErrorInvalidRequestFormat)
	doc := "id: rs-1\nname: RS\nidentifier: https://rs\nouHandle: ghost\n" +
		"sharingPolicies:\n  - id: p\n    targets:\n      - scope: allChildren\n"

	_, err := s.parse([]byte(doc))

	s.Require().Error(err)
	s.Contains(err.Error(), "ghost")
}

func (s *ResourceServerSharingParserTestSuite) TestAnUnreadableDocumentIsAnError() {
	_, err := s.parse([]byte("name: missing id\n"))

	s.Require().Error(err)
}
