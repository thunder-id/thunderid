// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/managedresource"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/tests/mocks/managedresourcemock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
	"github.com/thunder-id/thunderid/tests/mocks/sharingmock"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

const (
	testSharingServerID = "rs-shared"
	testSharingOwnerOU  = "ou-owner"
	testSharingChildOU  = "ou-child"
	testSharingPolicyID = "policy-1"
)

type SharingTestSuite struct {
	suite.Suite
	store   *resourceStoreInterfaceMock
	ou      *oumock.OrganizationUnitServiceInterfaceMock
	sharing *sharingmock.SharingServiceInterfaceMock
	svc     *resourceService
}

func TestSharingTestSuite(t *testing.T) {
	suite.Run(t, new(SharingTestSuite))
}

func (s *SharingTestSuite) SetupTest() {
	s.setStoreMode("")
	s.store = newResourceStoreInterfaceMock(s.T())
	s.ou = oumock.NewOrganizationUnitServiceInterfaceMock(s.T())
	s.sharing = sharingmock.NewSharingServiceInterfaceMock(s.T())
	s.svc = s.newService(allowAllAuthz(s.T()))
}

func (s *SharingTestSuite) SetupSubTest() {
	s.SetupTest()
}

func (s *SharingTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (s *SharingTestSuite) setStoreMode(mode string) {
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime("/tmp/test", &config.Config{
		Resource: engineconfig.ResourceConfig{Store: mode},
	}))
}

func (s *SharingTestSuite) newService(authz *sysauthzmock.SystemAuthorizationServiceInterfaceMock) *resourceService {
	service, err := newResourceService(s.ou, s.store, &fakeTransactioner{}, nil, nil, nil, discardingBus{}, authz,
		s.sharing)
	s.Require().NoError(err)
	service.SetDependencyRegistry(resourcedependency.Initialize(service))
	return service.(*resourceService)
}

func (s *SharingTestSuite) denyAll() {
	authz := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	authz.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).Return(false, nil).Maybe()
	s.svc = s.newService(authz)
}

func (s *SharingTestSuite) serverExists() {
	s.store.On("GetResourceServer", mock.Anything, testSharingServerID).Return(providers.ResourceServer{
		ID: testSharingServerID, Name: "Booking API", Identifier: "booking", OUID: testSharingOwnerOU, Delimiter: ":",
	}, nil)
}

func (s *SharingTestSuite) serverMissing() {
	s.store.On("GetResourceServer", mock.Anything, testSharingServerID).
		Return(providers.ResourceServer{}, errResourceServerNotFound)
}

func (s *SharingTestSuite) policyOf(resourceID, initiatingOUID string) sharing.Policy {
	return sharing.Policy{
		ID: testSharingPolicyID, ResourceType: ResourceServerSharingType, ResourceID: resourceID,
		OwningOUID: testSharingOwnerOU, InitiatingOUID: initiatingOUID,
	}
}

func childUnitsRequest(initiatingOUID string) sharing.PolicyRequest {
	return sharing.PolicyRequest{
		InitiatingOUID: initiatingOUID,
		ChildUnits:     &sharing.ChildUnitsRequest{Mode: sharing.ChildUnitsAll},
	}
}

func (s *SharingTestSuite) TestDeclarationSharesTheWholeServer() {
	decl := newResourceServerSharingDeclaration(s.svc)

	s.Equal(ResourceServerSharingType, decl.ResourceType())
	s.Equal(sharing.ResourceType("resource_server"), decl.ResourceType())
	s.Equal([]sharing.FieldDeclaration{{
		Key:     permissionsFieldKey,
		Kind:    sharing.FieldHierarchy,
		Default: &sharing.OverlayRule{Editable: false},
	}}, decl.Fields(), "with no rule a reached unit gets every permission, fixed")
}

func (s *SharingTestSuite) TestFieldDelimiter() {
	s.Run("the resource server's own delimiter", func() {
		s.serverExists()

		delimiter, svcErr := newResourceServerSharingDeclaration(s.svc).FieldDelimiter(context.Background(),
			testSharingServerID, permissionsFieldKey)

		s.Nil(svcErr)
		s.Equal(":", delimiter)
	})
	s.Run("an unknown resource server", func() {
		s.serverMissing()

		_, svcErr := newResourceServerSharingDeclaration(s.svc).FieldDelimiter(context.Background(),
			testSharingServerID, permissionsFieldKey)

		s.Equal(ErrorResourceServerNotFound.Code, svcErr.Code)
	})
}

func (s *SharingTestSuite) TestOnPolicyWrittenRefusesAPermissionsRule() {
	values := &[]string{"orders"}
	cases := []struct {
		name string
		rule sharing.OverlayRule
		want *tidcommon.ServiceError
	}{
		{name: "no value, fixed", rule: sharing.OverlayRule{Editable: false}},
		{name: "a value", rule: sharing.OverlayRule{Value: values}, want: &ErrorPermissionsRuleNotSupported},
		{name: "excluded values", rule: sharing.OverlayRule{ExcludedValues: values},
			want: &ErrorPermissionsRuleNotSupported},
		{name: "a value with others", rule: sharing.OverlayRule{Value: values, ExcludedValues: values},
			want: &ErrorPermissionsRuleNotSupported},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			policy := s.policyOf(testSharingServerID, testSharingOwnerOU)
			policy.Rules = []sharing.StoredRule{{FieldKey: permissionsFieldKey, Requested: tc.rule, Resolved: tc.rule}}

			revocations, svcErr := newResourceServerSharingDeclaration(s.svc).OnPolicyWritten(
				context.Background(), policy)

			s.Equal(tc.want, svcErr)
			s.Empty(revocations)
		})
	}
	s.Run("a policy with no rule", func() {
		decl := newResourceServerSharingDeclaration(s.svc)

		revocations, svcErr := decl.OnPolicyWritten(context.Background(),
			s.policyOf(testSharingServerID, testSharingOwnerOU))
		s.Nil(svcErr)
		s.Empty(revocations)
		revocations, svcErr = decl.OnPolicyRemoved(context.Background(),
			s.policyOf(testSharingServerID, testSharingOwnerOU))
		s.Nil(svcErr)
		s.Empty(revocations, "a resource server keeps no state per organization unit to revoke")
	})
}

func (s *SharingTestSuite) TestOwningOUID() {
	s.Run("resolves the owner", func() {
		s.serverExists()

		ouID, svcErr := newResourceServerSharingDeclaration(s.svc).OwningOUID(context.Background(),
			testSharingServerID)

		s.Nil(svcErr)
		s.Equal(testSharingOwnerOU, ouID)
	})
	s.Run("an unknown resource server", func() {
		s.serverMissing()

		_, svcErr := newResourceServerSharingDeclaration(s.svc).OwningOUID(context.Background(),
			testSharingServerID)

		s.Equal(ErrorResourceServerNotFound.Code, svcErr.Code)
	})
}

func (s *SharingTestSuite) TestCreateSharingPolicy() {
	s.Run("the owner's policy is recorded against the owner", func() {
		s.serverExists()
		s.store.On("IsResourceServerDeclarative", testSharingServerID).Return(false)
		req := childUnitsRequest("")
		s.sharing.On("CreatePolicy", mock.Anything, ResourceServerSharingType, testSharingServerID,
			testSharingOwnerOU, req).Return(s.policyOf(testSharingServerID, testSharingOwnerOU), nil).Once()

		policy, svcErr := s.svc.CreateSharingPolicy(context.Background(), testSharingServerID, req)

		s.Nil(svcErr)
		s.Equal(testSharingPolicyID, policy.ID)
	})
	s.Run("an unknown resource server", func() {
		s.serverMissing()

		_, svcErr := s.svc.CreateSharingPolicy(context.Background(), testSharingServerID, childUnitsRequest(""))

		s.Equal(ErrorResourceServerNotFound.Code, svcErr.Code)
	})
	s.Run("a caller who may not update resource servers in the initiating unit", func() {
		s.denyAll()
		s.serverExists()

		_, svcErr := s.svc.CreateSharingPolicy(context.Background(), testSharingServerID, childUnitsRequest(""))

		s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	})
	s.Run("the owner's policy of a declared resource server", func() {
		s.serverExists()
		s.store.On("IsResourceServerDeclarative", testSharingServerID).Return(true)

		_, svcErr := s.svc.CreateSharingPolicy(context.Background(), testSharingServerID, childUnitsRequest(""))

		s.Equal(ErrorImmutableResourceServer.Code, svcErr.Code)
	})
	s.Run("a reshare of a declared resource server", func() {
		s.serverExists()
		req := childUnitsRequest(testSharingChildOU)
		s.sharing.On("CreatePolicy", mock.Anything, ResourceServerSharingType, testSharingServerID,
			testSharingOwnerOU, req).Return(s.policyOf(testSharingServerID, testSharingChildOU), nil).Once()

		_, svcErr := s.svc.CreateSharingPolicy(context.Background(), testSharingServerID, req)

		s.Nil(svcErr)
		s.store.AssertNotCalled(s.T(), "IsResourceServerDeclarative", mock.Anything)
	})
	s.Run("the owner's policy of a resource server a control plane deployed", func() {
		managed := managedresourcemock.NewStoreInterfaceMock(s.T())
		managed.EXPECT().IsManaged(mock.Anything, testSharingServerID, managedresource.TypeResourceServer).
			Return(true, nil)
		s.svc.managed = managed
		s.serverExists()

		_, svcErr := s.svc.CreateSharingPolicy(context.Background(), testSharingServerID, childUnitsRequest(""))

		s.Equal(ErrorManagedResourceServer.Code, svcErr.Code)
	})
	s.Run("every write when resource servers load from files only", func() {
		s.setStoreMode("declarative")
		s.serverExists()

		_, svcErr := s.svc.CreateSharingPolicy(context.Background(), testSharingServerID,
			childUnitsRequest(testSharingChildOU))

		s.Equal(ErrorImmutableResourceServer.Code, svcErr.Code)
	})
	s.Run("a framework refusal answers in this service's codes", func() {
		s.serverExists()
		s.store.On("IsResourceServerDeclarative", testSharingServerID).Return(false)
		s.sharing.On("CreatePolicy", mock.Anything, ResourceServerSharingType, testSharingServerID,
			testSharingOwnerOU, mock.Anything).Return(sharing.Policy{}, &sharing.ErrorPolicyExists).Once()

		_, svcErr := s.svc.CreateSharingPolicy(context.Background(), testSharingServerID, childUnitsRequest(""))

		s.Equal(ErrorSharingPolicyExists.Code, svcErr.Code)
	})
	s.Run("a permissions rule refusal keeps its own code", func() {
		s.serverExists()
		s.store.On("IsResourceServerDeclarative", testSharingServerID).Return(false)
		s.sharing.On("CreatePolicy", mock.Anything, ResourceServerSharingType, testSharingServerID,
			testSharingOwnerOU, mock.Anything).Return(sharing.Policy{}, &ErrorPermissionsRuleNotSupported).Once()

		_, svcErr := s.svc.CreateSharingPolicy(context.Background(), testSharingServerID, childUnitsRequest(""))

		s.Equal(ErrorPermissionsRuleNotSupported.Code, svcErr.Code)
	})
}

func (s *SharingTestSuite) TestCreateSharingPolicyPublishesAnEvent() {
	bus, published := newRecordingBus(s.T())
	service, err := newResourceService(s.ou, s.store, &fakeTransactioner{}, nil, nil, nil, bus,
		allowAllAuthz(s.T()), s.sharing)
	s.Require().NoError(err)
	s.serverExists()
	s.store.On("IsResourceServerDeclarative", testSharingServerID).Return(false)
	s.sharing.On("CreatePolicy", mock.Anything, ResourceServerSharingType, testSharingServerID,
		testSharingOwnerOU, mock.Anything).Return(s.policyOf(testSharingServerID, testSharingOwnerOU), nil)

	_, svcErr := service.CreateSharingPolicy(adminContext(), testSharingServerID, childUnitsRequest(""))

	s.Nil(svcErr)
	events := published()
	s.Require().Len(events, 1)
	s.Equal(audit.EventResourceServerSharingPolicyCreate, events[0].name)
	s.Equal(audit.Ref{Type: audit.TargetTypeResourceServerSharingPolicy, ID: testSharingPolicyID},
		events[0].payload.Target())
}

// auditedSharingService returns a service publishing on a recording bus, and what it published. The caller may act
// in the owning organization unit only.
func (s *SharingTestSuite) auditedSharingService() (*resourceService, func() []publishedEvent) {
	authz := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	authz.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).Return(
		func(_ context.Context, _ security.Action, actionCtx *sysauthz.ActionContext) bool {
			return actionCtx == nil || actionCtx.OUID == "" || actionCtx.OUID == testSharingOwnerOU
		}, nil).Maybe()
	bus, published := newRecordingBus(s.T())
	service, err := newResourceService(s.ou, s.store, &fakeTransactioner{}, nil, nil, nil, bus, authz, s.sharing)
	s.Require().NoError(err)
	return service.(*resourceService), published
}

// requireOnlySharingEvent checks that exactly one event was published, named name, for an attempt on the policy. A
// succeeded or denied attempt has the resource server as subject; a failed one has none.
func (s *SharingTestSuite) requireOnlySharingEvent(events []publishedEvent, name string, action audit.Action,
	outcome audit.Outcome) {
	s.T().Helper()
	s.Require().Len(events, 1)
	s.Equal(name, events[0].name)
	s.Equal(audit.Ref{Type: audit.TargetTypeResourceServerSharingPolicy, ID: testSharingPolicyID},
		events[0].payload.Target())
	s.Equal(action, events[0].payload.Action())
	s.Equal(outcome, events[0].payload.Outcome())
	if outcome.Result == audit.ResultFailed {
		s.Equal(audit.Ref{}, events[0].payload.Subject())
	} else {
		s.Equal(audit.Ref{Type: audit.TargetTypeResourceServer, ID: testSharingServerID}, events[0].payload.Subject())
	}
}

func (s *SharingTestSuite) TestUpdateSharingPolicyPublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		policy  sharing.Policy
		arrange func()
		outcome audit.Outcome
	}{
		{
			name: "the policy is updated", policy: s.policyOf(testSharingServerID, testSharingOwnerOU),
			arrange: func() {
				s.sharing.On("UpdatePolicy", mock.Anything, testSharingPolicyID, mock.Anything).
					Return(s.policyOf(testSharingServerID, testSharingOwnerOU), nil).Once()
			},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name: "another unit's policy", policy: s.policyOf(testSharingServerID, testSharingChildOU),
			arrange: func() {}, outcome: audit.OutcomeDenied("forbidden"),
		},
		{
			name: "a stale version", policy: s.policyOf(testSharingServerID, testSharingOwnerOU),
			arrange: func() {
				s.sharing.On("UpdatePolicy", mock.Anything, testSharingPolicyID, mock.Anything).
					Return(sharing.Policy{}, &sharing.ErrorVersionMismatch).Once()
			},
			outcome: audit.OutcomeFailed("res_1032"),
		},
		{
			name:    "the policy belongs to another resource server",
			policy:  s.policyOf("another-server", testSharingOwnerOU),
			arrange: func() {}, outcome: audit.OutcomeFailed("res_1030"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			service, published := s.auditedSharingService()
			s.serverExists()
			s.store.On("IsResourceServerDeclarative", testSharingServerID).Return(false).Maybe()
			s.sharing.On("GetPolicy", mock.Anything, testSharingPolicyID).Return(tc.policy, nil).Once()
			tc.arrange()

			_, svcErr := service.UpdateSharingPolicy(adminContext(), testSharingServerID, testSharingPolicyID,
				childUnitsRequest(""))

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlySharingEvent(published(), audit.EventResourceServerSharingPolicyUpdate, audit.ActionUpdate,
				tc.outcome)
		})
	}
}

func (s *SharingTestSuite) TestDeleteSharingPolicyPublishesOneEventPerAttempt() {
	declared := s.policyOf(testSharingServerID, testSharingOwnerOU)
	declared.Declared = true
	cases := []struct {
		name    string
		policy  sharing.Policy
		arrange func()
		outcome audit.Outcome
	}{
		{
			name: "the policy is deleted", policy: s.policyOf(testSharingServerID, testSharingOwnerOU),
			arrange: func() {
				s.sharing.On("DeletePolicy", mock.Anything, testSharingPolicyID).Return(nil).Once()
			},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name: "another unit's policy", policy: s.policyOf(testSharingServerID, testSharingChildOU),
			arrange: func() {}, outcome: audit.OutcomeDenied("forbidden"),
		},
		{name: "a declared policy", policy: declared, arrange: func() {}, outcome: audit.OutcomeFailed("res_1033")},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			service, published := s.auditedSharingService()
			s.serverExists()
			s.store.On("IsResourceServerDeclarative", testSharingServerID).Return(false).Maybe()
			s.sharing.On("GetPolicy", mock.Anything, testSharingPolicyID).Return(tc.policy, nil).Once()
			tc.arrange()

			svcErr := service.DeleteSharingPolicy(adminContext(), testSharingServerID, testSharingPolicyID)

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlySharingEvent(published(), audit.EventResourceServerSharingPolicyDelete, audit.ActionDelete,
				tc.outcome)
		})
	}
}

// A delete stays idempotent, but one that finds no policy of the resource server records nothing, so a caller cannot
// write a succeeded delete for any ID it chooses.
func (s *SharingTestSuite) TestDeleteSharingPolicyPublishesNothingForAMissingPolicy() {
	cases := []struct {
		name   string
		policy sharing.Policy
		err    *tidcommon.ServiceError
	}{
		{name: "no policy has the ID", err: &sharing.ErrorPolicyNotFound},
		{
			name:   "the policy belongs to another resource server",
			policy: s.policyOf("another-server", testSharingOwnerOU),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			service, published := s.auditedSharingService()
			s.serverExists()
			s.sharing.On("GetPolicy", mock.Anything, testSharingPolicyID).Return(tc.policy, tc.err).Once()

			svcErr := service.DeleteSharingPolicy(adminContext(), testSharingServerID, testSharingPolicyID)

			s.Nil(svcErr)
			s.Empty(published())
		})
	}
}

func (s *SharingTestSuite) TestGetSharingPolicy() {
	s.Run("returns the resource server's policy", func() {
		s.serverExists()
		s.sharing.On("GetPolicy", mock.Anything, testSharingPolicyID).
			Return(s.policyOf(testSharingServerID, testSharingOwnerOU), nil)

		policy, svcErr := s.svc.GetSharingPolicy(context.Background(), testSharingServerID, testSharingPolicyID)

		s.Nil(svcErr)
		s.Equal(testSharingPolicyID, policy.ID)
	})
	s.Run("refuses another resource's policy", func() {
		s.serverExists()
		s.sharing.On("GetPolicy", mock.Anything, testSharingPolicyID).
			Return(s.policyOf("another-server", testSharingOwnerOU), nil)

		_, svcErr := s.svc.GetSharingPolicy(context.Background(), testSharingServerID, testSharingPolicyID)

		s.Equal(ErrorSharingPolicyNotFound.Code, svcErr.Code)
	})
	s.Run("an unknown policy", func() {
		s.serverExists()
		s.sharing.On("GetPolicy", mock.Anything, testSharingPolicyID).
			Return(sharing.Policy{}, &sharing.ErrorPolicyNotFound)

		_, svcErr := s.svc.GetSharingPolicy(context.Background(), testSharingServerID, testSharingPolicyID)

		s.Equal(ErrorSharingPolicyNotFound.Code, svcErr.Code)
	})
	s.Run("a caller who may read resource servers in neither the owning nor the initiating unit", func() {
		s.denyAll()
		s.serverExists()
		s.sharing.On("GetPolicy", mock.Anything, testSharingPolicyID).
			Return(s.policyOf(testSharingServerID, testSharingChildOU), nil)

		_, svcErr := s.svc.GetSharingPolicy(context.Background(), testSharingServerID, testSharingPolicyID)

		s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	})
}

func (s *SharingTestSuite) TestListSharingPolicies() {
	s.Run("lists the resource server's policies", func() {
		s.serverExists()
		s.sharing.On("GetPolicyList", mock.Anything, ResourceServerSharingType, testSharingServerID, 10, 0).
			Return(sharing.PolicyList{TotalResults: 1, Count: 1,
				Policies: []sharing.Policy{s.policyOf(testSharingServerID, testSharingOwnerOU)}}, nil)

		list, svcErr := s.svc.ListSharingPolicies(context.Background(), testSharingServerID, 10, 0)

		s.Nil(svcErr)
		s.Equal(1, list.TotalResults)
	})
	s.Run("a caller who may read only in a reached unit sees that unit's policies", func() {
		authz := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
		authz.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).Return(
			func(_ context.Context, _ security.Action, actionCtx *sysauthz.ActionContext) bool {
				return actionCtx != nil && actionCtx.OUID == testSharingChildOU
			}, nil)
		s.svc = s.newService(authz)
		s.serverExists()
		reshare := s.policyOf(testSharingServerID, testSharingChildOU)
		s.sharing.On("ListPolicies", mock.Anything, ResourceServerSharingType, testSharingServerID).
			Return([]sharing.Policy{s.policyOf(testSharingServerID, testSharingOwnerOU), reshare}, nil)

		list, svcErr := s.svc.ListSharingPolicies(context.Background(), testSharingServerID, 10, 0)

		s.Nil(svcErr)
		s.Equal([]sharing.Policy{reshare}, list.Policies)
	})
	s.Run("a caller who may read resource servers in no unit", func() {
		s.denyAll()
		s.serverExists()
		s.sharing.On("ListPolicies", mock.Anything, ResourceServerSharingType, testSharingServerID).
			Return([]sharing.Policy{s.policyOf(testSharingServerID, testSharingOwnerOU)}, nil)

		_, svcErr := s.svc.ListSharingPolicies(context.Background(), testSharingServerID, 10, 0)

		s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	})
}

func (s *SharingTestSuite) TestUpdateSharingPolicy() {
	s.Run("replaces the policy", func() {
		s.serverExists()
		s.store.On("IsResourceServerDeclarative", testSharingServerID).Return(false)
		s.sharing.On("GetPolicy", mock.Anything, testSharingPolicyID).
			Return(s.policyOf(testSharingServerID, testSharingOwnerOU), nil)
		req := childUnitsRequest("")
		s.sharing.On("UpdatePolicy", mock.Anything, testSharingPolicyID, req).
			Return(s.policyOf(testSharingServerID, testSharingOwnerOU), nil).Once()

		_, svcErr := s.svc.UpdateSharingPolicy(context.Background(), testSharingServerID, testSharingPolicyID, req)

		s.Nil(svcErr)
	})
	s.Run("a stale version", func() {
		s.serverExists()
		s.store.On("IsResourceServerDeclarative", testSharingServerID).Return(false)
		s.sharing.On("GetPolicy", mock.Anything, testSharingPolicyID).
			Return(s.policyOf(testSharingServerID, testSharingOwnerOU), nil)
		s.sharing.On("UpdatePolicy", mock.Anything, testSharingPolicyID, mock.Anything).
			Return(sharing.Policy{}, &sharing.ErrorVersionMismatch)

		_, svcErr := s.svc.UpdateSharingPolicy(context.Background(), testSharingServerID, testSharingPolicyID,
			childUnitsRequest(""))

		s.Equal(ErrorSharingPolicyVersionMismatch.Code, svcErr.Code)
	})
	for _, tc := range s.writeRefusals() {
		s.Run(tc.name, func() {
			tc.arrange()
			s.sharing.On("GetPolicy", mock.Anything, testSharingPolicyID).
				Return(s.policyOf(testSharingServerID, tc.initiatingOUID), nil)

			_, svcErr := s.svc.UpdateSharingPolicy(context.Background(), testSharingServerID, testSharingPolicyID,
				childUnitsRequest(""))

			s.Equal(tc.code, svcErr.Code)
			s.sharing.AssertNotCalled(s.T(), "UpdatePolicy", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

// sharingWriteRefusal is a sharing write the service refuses before it reaches the framework.
type sharingWriteRefusal struct {
	name           string
	initiatingOUID string
	arrange        func()
	code           string
}

// writeRefusals are the refusals every update and delete of an existing policy shares.
func (s *SharingTestSuite) writeRefusals() []sharingWriteRefusal {
	return []sharingWriteRefusal{
		{
			name:           "a caller who may not update resource servers in the initiating unit",
			initiatingOUID: testSharingOwnerOU,
			arrange:        func() { s.denyAll(); s.serverExists() }, code: tidcommon.ErrorUnauthorized.Code,
		},
		{
			name: "the owner's policy of a declared resource server", initiatingOUID: testSharingOwnerOU,
			arrange: func() {
				s.serverExists()
				s.store.On("IsResourceServerDeclarative", testSharingServerID).Return(true)
			},
			code: ErrorImmutableResourceServer.Code,
		},
		{
			name:           "the owner's policy of a resource server a control plane deployed",
			initiatingOUID: testSharingOwnerOU,
			arrange: func() {
				managed := managedresourcemock.NewStoreInterfaceMock(s.T())
				managed.EXPECT().IsManaged(mock.Anything, testSharingServerID, managedresource.TypeResourceServer).
					Return(true, nil)
				s.svc.managed = managed
				s.serverExists()
			},
			code: ErrorManagedResourceServer.Code,
		},
		{
			name: "a reshare when resource servers load from files only", initiatingOUID: testSharingChildOU,
			arrange: func() { s.setStoreMode("declarative"); s.serverExists() },
			code:    ErrorImmutableResourceServer.Code,
		},
	}
}

func (s *SharingTestSuite) TestDeleteSharingPolicy() {
	s.Run("removes the policy", func() {
		s.serverExists()
		s.store.On("IsResourceServerDeclarative", testSharingServerID).Return(false)
		s.sharing.On("GetPolicy", mock.Anything, testSharingPolicyID).
			Return(s.policyOf(testSharingServerID, testSharingOwnerOU), nil)
		s.sharing.On("DeletePolicy", mock.Anything, testSharingPolicyID).Return(nil).Once()

		s.Nil(s.svc.DeleteSharingPolicy(context.Background(), testSharingServerID, testSharingPolicyID))
	})
	s.Run("an unknown policy is already gone", func() {
		s.serverExists()
		s.sharing.On("GetPolicy", mock.Anything, testSharingPolicyID).
			Return(sharing.Policy{}, &sharing.ErrorPolicyNotFound)

		s.Nil(s.svc.DeleteSharingPolicy(context.Background(), testSharingServerID, testSharingPolicyID))
		s.sharing.AssertNotCalled(s.T(), "DeletePolicy", mock.Anything, mock.Anything)
	})
	s.Run("a declared policy stays", func() {
		s.serverExists()
		policy := s.policyOf(testSharingServerID, testSharingChildOU)
		policy.Declared = true
		s.sharing.On("GetPolicy", mock.Anything, testSharingPolicyID).Return(policy, nil)

		svcErr := s.svc.DeleteSharingPolicy(context.Background(), testSharingServerID, testSharingPolicyID)

		s.Equal(ErrorSharingPolicyDeclared.Code, svcErr.Code)
	})
	for _, tc := range s.writeRefusals() {
		s.Run(tc.name, func() {
			tc.arrange()
			s.sharing.On("GetPolicy", mock.Anything, testSharingPolicyID).
				Return(s.policyOf(testSharingServerID, tc.initiatingOUID), nil)

			svcErr := s.svc.DeleteSharingPolicy(context.Background(), testSharingServerID, testSharingPolicyID)

			s.Equal(tc.code, svcErr.Code)
			s.sharing.AssertNotCalled(s.T(), "DeletePolicy", mock.Anything, mock.Anything)
		})
	}
}

func (s *SharingTestSuite) TestResolveSharingOverlay() {
	wholeServer := func() sharing.ResolvedOverlay {
		return sharing.ResolvedOverlay{OUID: testSharingChildOU, Visible: true,
			PolicyIDs: []string{testSharingPolicyID},
			Rules:     map[string]sharing.OverlayRule{permissionsFieldKey: {Editable: false}},
			Sources:   map[string]string{permissionsFieldKey: sharing.SourceDefault}}
	}
	s.Run("a reached organization unit holds every permission of the server", func() {
		s.serverExists()
		resolved := wholeServer()
		s.sharing.On("ResolveOverlayRules", mock.Anything, ResourceServerSharingType, testSharingServerID,
			testSharingChildOU).Return(resolved, nil)
		s.store.On("GetActionListCount", mock.Anything, testSharingServerID, (*string)(nil),
			providers.ActionKind("")).Return(1, nil)
		s.store.On("GetActionList", mock.Anything, testSharingServerID, (*string)(nil), providers.ActionKind(""), 1, 0).
			Return([]providers.Action{{Permission: "admin"}}, nil)
		s.store.On("GetResourceListCountByParent", mock.Anything, testSharingServerID, (*string)(nil)).
			Return(1, nil)
		s.store.On("GetResourceListByParent", mock.Anything, testSharingServerID, (*string)(nil), 1, 0).
			Return([]providers.Resource{{Permission: "orders"}}, nil)
		s.store.On("GetDeclaredPermissions", mock.Anything, testSharingServerID, []string{"orders"}, ":").
			Return([]string{"orders", "orders:items", "orders:items:read", "orders:read"}, nil)

		overlay, svcErr := s.svc.ResolveSharingOverlay(context.Background(), testSharingServerID,
			testSharingChildOU)

		s.Nil(svcErr)
		s.True(overlay.Visible)
		s.Equal(sharing.OverlayRule{Editable: false, Value: &[]string{
			"admin", "orders", "orders:items", "orders:items:read", "orders:read"}},
			overlay.Rules[permissionsFieldKey])
		s.Nil(resolved.Rules[permissionsFieldKey].Value, "the framework's own copy is left as it was")
	})
	s.Run("a resource server that defines no permission", func() {
		s.serverExists()
		s.sharing.On("ResolveOverlayRules", mock.Anything, ResourceServerSharingType, testSharingServerID,
			testSharingChildOU).Return(wholeServer(), nil)
		s.store.On("GetActionListCount", mock.Anything, testSharingServerID, (*string)(nil),
			providers.ActionKind("")).Return(0, nil)
		s.store.On("GetResourceListCountByParent", mock.Anything, testSharingServerID, (*string)(nil)).
			Return(0, nil)

		overlay, svcErr := s.svc.ResolveSharingOverlay(context.Background(), testSharingServerID,
			testSharingChildOU)

		s.Nil(svcErr)
		s.Equal(&[]string{}, overlay.Rules[permissionsFieldKey].Value)
	})
	s.Run("a rule that already names its permissions", func() {
		s.serverExists()
		resolved := wholeServer()
		resolved.Rules[permissionsFieldKey] = sharing.OverlayRule{Value: &[]string{"orders"}}
		s.sharing.On("ResolveOverlayRules", mock.Anything, ResourceServerSharingType, testSharingServerID,
			testSharingChildOU).Return(resolved, nil)

		overlay, svcErr := s.svc.ResolveSharingOverlay(context.Background(), testSharingServerID,
			testSharingChildOU)

		s.Nil(svcErr)
		s.Equal(&[]string{"orders"}, overlay.Rules[permissionsFieldKey].Value)
		s.store.AssertNotCalled(s.T(), "GetActionListCount", mock.Anything, mock.Anything, mock.Anything,
			mock.Anything)
	})
	s.Run("the permissions cannot be listed", func() {
		s.serverExists()
		s.sharing.On("ResolveOverlayRules", mock.Anything, ResourceServerSharingType, testSharingServerID,
			testSharingChildOU).Return(wholeServer(), nil)
		s.store.On("GetActionListCount", mock.Anything, testSharingServerID, (*string)(nil),
			providers.ActionKind("")).Return(1, nil)
		s.store.On("GetActionList", mock.Anything, testSharingServerID, (*string)(nil), providers.ActionKind(""), 1, 0).
			Return(nil, errResultLimitExceededInCompositeMode)

		_, svcErr := s.svc.ResolveSharingOverlay(context.Background(), testSharingServerID, testSharingChildOU)

		s.Equal(ErrResultLimitExceededInCompositeMode.Code, svcErr.Code)
	})
	s.Run("the actions cannot be counted", func() {
		s.serverExists()
		s.sharing.On("ResolveOverlayRules", mock.Anything, ResourceServerSharingType, testSharingServerID,
			testSharingChildOU).Return(wholeServer(), nil)
		s.store.On("GetActionListCount", mock.Anything, testSharingServerID, (*string)(nil),
			providers.ActionKind("")).Return(0, errors.New("db down"))

		_, svcErr := s.svc.ResolveSharingOverlay(context.Background(), testSharingServerID, testSharingChildOU)

		s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	})
	s.Run("the resources cannot be listed", func() {
		s.serverExists()
		s.sharing.On("ResolveOverlayRules", mock.Anything, ResourceServerSharingType, testSharingServerID,
			testSharingChildOU).Return(wholeServer(), nil)
		s.store.On("GetActionListCount", mock.Anything, testSharingServerID, (*string)(nil),
			providers.ActionKind("")).Return(0, nil)
		s.store.On("GetResourceListCountByParent", mock.Anything, testSharingServerID, (*string)(nil)).
			Return(1, nil)
		s.store.On("GetResourceListByParent", mock.Anything, testSharingServerID, (*string)(nil), 1, 0).
			Return(nil, errors.New("db down"))

		_, svcErr := s.svc.ResolveSharingOverlay(context.Background(), testSharingServerID, testSharingChildOU)

		s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	})
	s.Run("too many actions to count in composite mode", func() {
		s.serverExists()
		s.sharing.On("ResolveOverlayRules", mock.Anything, ResourceServerSharingType, testSharingServerID,
			testSharingChildOU).Return(wholeServer(), nil)
		s.store.On("GetActionListCount", mock.Anything, testSharingServerID, (*string)(nil),
			providers.ActionKind("")).Return(0, errResultLimitExceededInCompositeMode)

		_, svcErr := s.svc.ResolveSharingOverlay(context.Background(), testSharingServerID, testSharingChildOU)

		s.Equal(ErrResultLimitExceededInCompositeMode.Code, svcErr.Code)
	})
	s.Run("too many resources to count in composite mode", func() {
		s.serverExists()
		s.sharing.On("ResolveOverlayRules", mock.Anything, ResourceServerSharingType, testSharingServerID,
			testSharingChildOU).Return(wholeServer(), nil)
		s.store.On("GetActionListCount", mock.Anything, testSharingServerID, (*string)(nil),
			providers.ActionKind("")).Return(0, nil)
		s.store.On("GetResourceListCountByParent", mock.Anything, testSharingServerID, (*string)(nil)).
			Return(0, errResultLimitExceededInCompositeMode)

		_, svcErr := s.svc.ResolveSharingOverlay(context.Background(), testSharingServerID, testSharingChildOU)

		s.Equal(ErrResultLimitExceededInCompositeMode.Code, svcErr.Code)
	})
	s.Run("a caller who may read resource servers in neither the owning unit nor the one named", func() {
		s.denyAll()
		s.serverExists()

		_, svcErr := s.svc.ResolveSharingOverlay(context.Background(), testSharingServerID, testSharingChildOU)

		s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
		s.sharing.AssertNotCalled(s.T(), "ResolveOverlayRules", mock.Anything, mock.Anything, mock.Anything,
			mock.Anything)
	})
	s.Run("no organization unit named", func() {
		s.serverExists()

		_, svcErr := s.svc.ResolveSharingOverlay(context.Background(), testSharingServerID, "")

		s.Equal(ErrorMissingOUIDParameter.Code, svcErr.Code)
	})
	s.Run("an organization unit never reached", func() {
		s.serverExists()
		s.sharing.On("ResolveOverlayRules", mock.Anything, ResourceServerSharingType, testSharingServerID,
			testSharingChildOU).Return(sharing.ResolvedOverlay{OUID: testSharingChildOU}, nil)

		_, svcErr := s.svc.ResolveSharingOverlay(context.Background(), testSharingServerID, testSharingChildOU)

		s.Equal(ErrorResourceServerNotSharedToOU.Code, svcErr.Code)
	})
	s.Run("an organization unit that does not exist", func() {
		s.serverExists()
		s.sharing.On("ResolveOverlayRules", mock.Anything, ResourceServerSharingType, testSharingServerID,
			"ou-missing").Return(sharing.ResolvedOverlay{}, &oupkg.ErrorOrganizationUnitNotFound)

		_, svcErr := s.svc.ResolveSharingOverlay(context.Background(), testSharingServerID, "ou-missing")

		s.Equal(ErrorResourceServerNotSharedToOU.Code, svcErr.Code)
	})
}

func (s *SharingTestSuite) TestSharedResourceServerCannotMove() {
	s.Run("to another organization unit", func() {
		s.serverExists()
		s.store.On("IsResourceServerDeclarative", testSharingServerID).Return(false)
		s.ou.On("GetOrganizationUnit", mock.Anything, "ou-other").
			Return(oupkg.OrganizationUnit{ID: "ou-other"}, nil)
		s.sharing.On("ListPolicies", mock.Anything, ResourceServerSharingType, testSharingServerID).
			Return([]sharing.Policy{s.policyOf(testSharingServerID, testSharingOwnerOU)}, nil)

		_, svcErr := s.svc.UpdateResourceServer(context.Background(), testSharingServerID, providers.ResourceServer{
			Name: "Booking API", OUID: "ou-other",
		})

		s.Equal(ErrorSharedResourceServerCannotMove.Code, svcErr.Code)
		s.store.AssertNotCalled(s.T(), "UpdateResourceServer", mock.Anything, mock.Anything, mock.Anything)
	})
	s.Run("an unshared one moves", func() {
		s.serverExists()
		s.store.On("IsResourceServerDeclarative", testSharingServerID).Return(false)
		s.ou.On("GetOrganizationUnit", mock.Anything, "ou-other").
			Return(oupkg.OrganizationUnit{ID: "ou-other"}, nil)
		s.sharing.On("ListPolicies", mock.Anything, ResourceServerSharingType, testSharingServerID).
			Return([]sharing.Policy{}, nil)
		s.store.On("UpdateResourceServer", mock.Anything, testSharingServerID, mock.Anything).Return(nil)

		updated, svcErr := s.svc.UpdateResourceServer(context.Background(), testSharingServerID,
			providers.ResourceServer{Name: "Booking API", OUID: "ou-other"})

		s.Nil(svcErr)
		s.Equal("ou-other", updated.OUID)
	})
}

func (s *SharingTestSuite) TestDeleteResourceServerRemovesItsPolicies() {
	s.Run("the policies go with the resource server", func() {
		s.serverExists()
		s.store.On("IsResourceServerDeclarative", testSharingServerID).Return(false)
		s.store.On("CheckResourceServerHasDependencies", mock.Anything, testSharingServerID).Return(false, nil)
		s.store.On("DeleteResourceServer", mock.Anything, testSharingServerID).Return(nil)
		s.sharing.On("DeletePoliciesForResource", mock.Anything, ResourceServerSharingType, testSharingServerID).
			Return(nil).Once()

		s.Nil(s.svc.DeleteResourceServer(context.Background(), testSharingServerID))
	})
	s.Run("a refused policy removal fails the delete", func() {
		s.serverExists()
		s.store.On("IsResourceServerDeclarative", testSharingServerID).Return(false)
		s.store.On("CheckResourceServerHasDependencies", mock.Anything, testSharingServerID).Return(false, nil)
		s.store.On("DeleteResourceServer", mock.Anything, testSharingServerID).Return(nil)
		s.sharing.On("DeletePoliciesForResource", mock.Anything, ResourceServerSharingType, testSharingServerID).
			Return(&tidcommon.InternalServerError)

		svcErr := s.svc.DeleteResourceServer(context.Background(), testSharingServerID)

		s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	})
}

func (s *SharingTestSuite) TestSharingParser() {
	s.Run("a document without policies declares none", func() {
		service := NewResourceServiceInterfaceMock(s.T())

		declared, err := makeResourceServerSharingParser(service)([]byte("id: rs-1\nname: Booking API\n"))

		s.NoError(err)
		s.Equal("rs-1", declared.ResourceID)
		s.Empty(declared.Policies)
	})
	s.Run("a document naming its organization unit by handle", func() {
		service := NewResourceServiceInterfaceMock(s.T())
		service.On("ResolveResourceServerOUHandle", mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				args.Get(1).(*providers.ResourceServer).OUID = testSharingOwnerOU
			}).Return(nil)
		doc := "id: rs-1\nname: Booking API\nouHandle: owner\nsharingPolicies:\n" +
			"  - id: to-children\n    childUnits:\n      mode: all\n"

		declared, err := makeResourceServerSharingParser(service)([]byte(doc))

		s.NoError(err)
		s.Equal(testSharingOwnerOU, declared.OwningOUID)
		s.Equal("Booking API", declared.ResourceName)
		s.Require().Len(declared.Policies, 1)
		s.Equal("to-children", declared.Policies[0].ID)
	})
	s.Run("an unknown organization unit handle", func() {
		service := NewResourceServiceInterfaceMock(s.T())
		service.On("ResolveResourceServerOUHandle", mock.Anything, mock.Anything).
			Return(&ErrorOrganizationUnitNotFound)
		doc := "id: rs-1\nname: Booking API\nouHandle: missing\nsharingPolicies:\n" +
			"  - id: to-children\n    childUnits:\n      mode: all\n"

		_, err := makeResourceServerSharingParser(service)([]byte(doc))

		s.ErrorContains(err, `organization unit with handle "missing" not found`)
	})
	s.Run("a malformed document", func() {
		_, err := makeResourceServerSharingParser(NewResourceServiceInterfaceMock(s.T()))([]byte("id: [\n"))

		s.Error(err)
	})
}

func (s *SharingTestSuite) TestMakeResourceServerSharingConfig() {
	cfg := makeResourceServerSharingConfig(s.svc)

	s.Equal(ResourceServerSharingType, cfg.ResourceType)
	s.Equal("resource_servers", cfg.DirectoryName)
	s.NotNil(cfg.Parser)
}
