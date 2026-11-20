// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowmgt

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
	"github.com/thunder-id/thunderid/tests/mocks/sharingmock"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

const (
	testAuditPolicyID = "p1"
	// testAuditVariableValue is a value an organization unit sets; no event may carry it, nor the name of
	// the variable.
	testAuditVariableValue = "help@emea.example"
	testAuditVariableName  = "supportEmail"
)

// FlowSharingAuditTestSuite checks the audit event each change to a sharing policy of a flow, and to the
// values an organization unit holds for its variables, publishes.
type FlowSharingAuditTestSuite struct {
	suite.Suite
	store     *flowStoreInterfaceMock
	sharing   *sharingmock.SharingServiceInterfaceMock
	published func() []publishedEvent
	service   *flowMgtService
	// allowedOU is the one organization unit the caller may change flows of.
	allowedOU string
}

func TestFlowSharingAuditTestSuite(t *testing.T) {
	suite.Run(t, new(FlowSharingAuditTestSuite))
}

// SetupTest wires a flow owned by ownerOU that declares one variable, and a caller who may change flows of
// ownerOU only.
func (s *FlowSharingAuditTestSuite) SetupTest() {
	s.allowedOU = ownerOU
	s.store = newFlowStoreInterfaceMock(s.T())
	flow := &providers.CompleteFlowDefinition{
		ID: flowIDVar, Handle: testAuditFlowHandle, Name: testAuditFlowName,
		FlowType: providers.FlowTypeAuthentication, OUID: ownerOU,
		Nodes: []providers.NodeDefinition{
			{ID: "start", Type: "START", OnSuccess: "vars"},
			{ID: "vars", Type: "VARIABLES", OnSuccess: "end", Inputs: []providers.VariableDefinition{
				{Identifier: testAuditVariableName, Type: providers.InputTypeEmail,
					DefaultValue: "support@example.com"},
			}},
			{ID: "end", Type: "END"},
		},
	}
	s.store.EXPECT().GetFlowByID(mock.Anything, flowIDVar).Return(flow, nil).Maybe()
	s.store.EXPECT().GetFlowByID(mock.Anything, "missing").Return(nil, errFlowNotFound).Maybe()
	s.sharing = sharingmock.NewSharingServiceInterfaceMock(s.T())

	authz := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	allowed := func(actionCtx *sysauthz.ActionContext) bool {
		return actionCtx == nil || actionCtx.OUID == "" || actionCtx.OUID == s.allowedOU
	}
	authz.On("IsActionAllowed", mock.Anything, mock.Anything, mock.MatchedBy(allowed)).Return(true, nil).Maybe()
	authz.On("IsActionAllowed", mock.Anything, mock.Anything,
		mock.MatchedBy(func(actionCtx *sysauthz.ActionContext) bool { return !allowed(actionCtx) })).
		Return(false, nil).Maybe()

	var bus eventbus.EventBusInterface
	bus, s.published = newRecordingBus(s.T())
	s.service = newFlowMgtService(s.store, nil, nil, nil, nil, nil, nil, &stubTransactioner{}, nil,
		newOuProviderMock(s.T()), nil, nil, s.sharing, newIdpProviderMock(s.T()), bus, nil, authz)
}

func (s *FlowSharingAuditTestSuite) SetupSubTest() {
	s.SetupTest()
}

// flowPolicy is a policy of the flow made by the initiating organization unit.
func flowPolicy(initiating string) sharing.Policy {
	return sharing.Policy{ID: testAuditPolicyID, ResourceType: FlowSharingType, ResourceID: flowIDVar,
		InitiatingOUID: initiating}
}

// requireOnlyEvent checks that exactly one event was published, named name, on target, for an attempt by
// the admin. A succeeded or denied attempt has the flow as subject; a failed one has none. Nothing of the
// flow, the policy or a variable is recorded.
func (s *FlowSharingAuditTestSuite) requireOnlyEvent(name string, action audit.Action, target audit.Ref,
	outcome audit.Outcome) {
	s.T().Helper()
	events := s.published()
	s.Require().Len(events, 1)
	event := events[0]
	s.Equal(name, event.name)
	s.Equal(target, event.payload.Target())
	s.Equal(action, event.payload.Action())
	s.Equal(outcome, event.payload.Outcome())
	s.Equal(audit.Actor{Type: audit.ActorTypeUser, ID: testAuditAdminID}, event.payload.Actor())
	s.Empty(event.payload.Details())
	if outcome.Result == audit.ResultFailed {
		s.Equal(audit.Ref{}, event.payload.Subject())
	} else {
		s.Equal(audit.Ref{Type: audit.TargetTypeFlow, ID: flowIDVar}, event.payload.Subject())
	}
	recorded := fmt.Sprintf("%+v", event.payload)
	for _, value := range []string{testAuditFlowName, testAuditFlowHandle, ownerOU, childOU,
		testAuditVariableName, testAuditVariableValue} {
		s.NotContains(recorded, value, "nothing of the flow, the policy or a variable is recorded")
	}
}

func policyTarget(policyID string) audit.Ref {
	return audit.Ref{Type: audit.TargetTypeFlowSharingPolicy, ID: policyID}
}

func (s *FlowSharingAuditTestSuite) TestCreateSharingPolicy_PublishesOneEventPerAttempt() {
	cases := []struct {
		name     string
		flowID   string
		request  sharing.PolicyRequest
		arrange  func()
		policyID string
		outcome  audit.Outcome
	}{
		{
			name: "the policy is created", flowID: flowIDVar,
			arrange: func() {
				s.sharing.EXPECT().CreatePolicy(mock.Anything, FlowSharingType, flowIDVar, ownerOU, mock.Anything).
					Return(flowPolicy(ownerOU), nil).Once()
			},
			policyID: testAuditPolicyID, outcome: audit.OutcomeSucceeded,
		},
		{
			name: "the caller may not act for the initiating unit", flowID: flowIDVar,
			request: sharing.PolicyRequest{InitiatingOUID: childOU},
			arrange: func() {}, outcome: audit.OutcomeDenied("forbidden"),
		},
		{
			name: "the framework refuses the policy", flowID: flowIDVar,
			arrange: func() {
				s.sharing.EXPECT().CreatePolicy(mock.Anything, FlowSharingType, flowIDVar, ownerOU, mock.Anything).
					Return(sharing.Policy{}, &sharing.ErrorPolicyExists).Once()
			},
			outcome: audit.OutcomeFailed("flm_1031"),
		},
		{
			name: "the flow does not exist", flowID: "missing",
			arrange: func() {}, outcome: audit.OutcomeFailed("flm_1003"),
		},
		{
			// The flow is looked up before the caller is checked, so a flow that is not there is a failed
			// attempt whoever the initiating unit is, and records no subject.
			name: "the flow does not exist and the caller may not act for the initiating unit", flowID: "missing",
			request: sharing.PolicyRequest{InitiatingOUID: childOU},
			arrange: func() {}, outcome: audit.OutcomeFailed("flm_1003"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			tc.arrange()

			_, svcErr := s.service.CreateSharingPolicy(adminContext(), tc.flowID, tc.request)

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(audit.EventFlowSharingPolicyCreate, audit.ActionCreate, policyTarget(tc.policyID),
				tc.outcome)
		})
	}
}

func (s *FlowSharingAuditTestSuite) TestUpdateSharingPolicy_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		policy  sharing.Policy
		arrange func()
		outcome audit.Outcome
	}{
		{
			name: "the policy is updated", policy: flowPolicy(ownerOU),
			arrange: func() {
				s.sharing.EXPECT().UpdatePolicy(mock.Anything, testAuditPolicyID, mock.Anything).
					Return(flowPolicy(ownerOU), nil).Once()
			},
			outcome: audit.OutcomeSucceeded,
		},
		{name: "another unit's policy", policy: flowPolicy(childOU), arrange: func() {},
			outcome: audit.OutcomeDenied("forbidden")},
		{
			name: "the policy belongs to another flow",
			policy: sharing.Policy{ID: testAuditPolicyID, ResourceType: FlowSharingType, ResourceID: "another-flow",
				InitiatingOUID: ownerOU},
			arrange: func() {}, outcome: audit.OutcomeFailed("flm_1030"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.sharing.EXPECT().GetPolicy(mock.Anything, testAuditPolicyID).Return(tc.policy, nil).Once()
			tc.arrange()

			_, svcErr := s.service.UpdateSharingPolicy(adminContext(), flowIDVar, testAuditPolicyID,
				sharing.PolicyRequest{Version: 1})

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(audit.EventFlowSharingPolicyUpdate, audit.ActionUpdate,
				policyTarget(testAuditPolicyID), tc.outcome)
		})
	}
}

func (s *FlowSharingAuditTestSuite) TestDeleteSharingPolicy_PublishesOneEventPerAttempt() {
	declared := flowPolicy(ownerOU)
	declared.Declared = true
	cases := []struct {
		name    string
		policy  sharing.Policy
		arrange func()
		outcome audit.Outcome
	}{
		{
			name: "the policy is deleted", policy: flowPolicy(ownerOU),
			arrange: func() {
				s.sharing.EXPECT().DeletePolicy(mock.Anything, testAuditPolicyID).Return(nil).Once()
			},
			outcome: audit.OutcomeSucceeded,
		},
		{name: "another unit's policy", policy: flowPolicy(childOU), arrange: func() {},
			outcome: audit.OutcomeDenied("forbidden")},
		{name: "a declared policy", policy: declared, arrange: func() {}, outcome: audit.OutcomeFailed("flm_1033")},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.sharing.EXPECT().GetPolicy(mock.Anything, testAuditPolicyID).Return(tc.policy, nil).Once()
			tc.arrange()

			svcErr := s.service.DeleteSharingPolicy(adminContext(), flowIDVar, testAuditPolicyID)

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(audit.EventFlowSharingPolicyDelete, audit.ActionDelete,
				policyTarget(testAuditPolicyID), tc.outcome)
		})
	}
}

// A delete stays idempotent, but one that finds no policy of the flow records nothing, so a caller cannot
// write a succeeded delete for any ID it chooses.
func (s *FlowSharingAuditTestSuite) TestDeleteSharingPolicy_PublishesNothingForAMissingPolicy() {
	cases := []struct {
		name   string
		policy sharing.Policy
		err    *tidcommon.ServiceError
	}{
		{name: "no policy has the ID", err: &sharing.ErrorPolicyNotFound},
		{name: "the policy belongs to another flow", policy: sharing.Policy{ID: testAuditPolicyID,
			ResourceType: FlowSharingType, ResourceID: "another-flow", InitiatingOUID: ownerOU}},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.sharing.EXPECT().GetPolicy(mock.Anything, testAuditPolicyID).Return(tc.policy, tc.err).Once()

			svcErr := s.service.DeleteSharingPolicy(adminContext(), flowIDVar, testAuditPolicyID)

			s.Nil(svcErr)
			s.Empty(s.published())
		})
	}
}

// A flow that is not there fails the attempt before any policy is read.
func (s *FlowSharingAuditTestSuite) TestUpdateAndDeleteSharingPolicy_FailWhenTheFlowDoesNotExist() {
	s.Run("update", func() {
		_, svcErr := s.service.UpdateSharingPolicy(adminContext(), "missing", testAuditPolicyID,
			sharing.PolicyRequest{Version: 1})

		s.Require().NotNil(svcErr)
		s.requireOnlyEvent(audit.EventFlowSharingPolicyUpdate, audit.ActionUpdate,
			audit.Ref{Type: audit.TargetTypeFlowSharingPolicy, ID: testAuditPolicyID},
			audit.OutcomeFailed("flm_1003"))
	})
	s.Run("delete", func() {
		svcErr := s.service.DeleteSharingPolicy(adminContext(), "missing", testAuditPolicyID)

		s.Require().NotNil(svcErr)
		s.requireOnlyEvent(audit.EventFlowSharingPolicyDelete, audit.ActionDelete,
			audit.Ref{Type: audit.TargetTypeFlowSharingPolicy, ID: testAuditPolicyID},
			audit.OutcomeFailed("flm_1003"))
	})
}

// shareWithChild has the framework answer that the child holds the flow and may set its variable.
func (s *FlowSharingAuditTestSuite) shareWithChild() {
	key := sharingKey(testAuditVariableName)
	rule := sharing.FieldRule{OUID: childOU, Visible: true, Source: sharing.SourcePolicy,
		Rule: sharing.OverlayRule{Editable: true}}
	s.sharing.EXPECT().IsVisible(mock.Anything, FlowSharingType, flowIDVar, childOU).Return(true, nil).Maybe()
	s.sharing.EXPECT().ResolveFieldRule(mock.Anything, FlowSharingType, flowIDVar, childOU, key).
		Return(rule, nil).Maybe()
	s.sharing.EXPECT().ResolveFieldRules(mock.Anything, FlowSharingType, flowIDVar, childOU, mock.Anything).
		Return(map[string]sharing.FieldRule{key: rule}, nil).Maybe()
}

func (s *FlowSharingAuditTestSuite) TestSetVariables_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		flowID  string
		value   interface{}
		arrange func()
		outcome audit.Outcome
	}{
		{
			name: "the values are set", flowID: flowIDVar,
			arrange: func() {
				s.allowedOU = childOU
				s.sharing.EXPECT().ReplaceOverlayValues(mock.Anything, FlowSharingType, flowIDVar, childOU,
					mock.Anything, mock.Anything).Return(nil).Once()
				s.sharing.EXPECT().ResolveOverlayValues(mock.Anything, FlowSharingType, flowIDVar, childOU).
					Return(sharing.ResolvedValues{OUID: childOU, Visible: true}, nil).Once()
			},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name: "the caller may not act for the unit", flowID: flowIDVar,
			arrange: func() {}, outcome: audit.OutcomeDenied("forbidden"),
		},
		{
			name: "the store fails", flowID: flowIDVar,
			arrange: func() {
				s.allowedOU = childOU
				s.sharing.EXPECT().ReplaceOverlayValues(mock.Anything, FlowSharingType, flowIDVar, childOU,
					mock.Anything, mock.Anything).
					Return(&sharing.OverlayReplaceError{Err: &tidcommon.InternalServerError}).Once()
			},
			outcome: audit.OutcomeFailed("sse_5000"),
		},
		{
			name: "a value is not valid for its variable", flowID: flowIDVar, value: "not an email",
			arrange: func() {
				s.allowedOU = childOU
			},
			outcome: audit.OutcomeFailed("flm_1036"),
		},
		{
			name: "the store refuses a value of a variable", flowID: flowIDVar,
			arrange: func() {
				s.allowedOU = childOU
				s.sharing.EXPECT().ReplaceOverlayValues(mock.Anything, FlowSharingType, flowIDVar, childOU,
					mock.Anything, mock.Anything).Return(&sharing.OverlayReplaceError{
					FieldKey: sharingFieldKey + "." + testAuditVariableName,
					Err:      &sharing.ErrorFieldNotEditable}).Once()
			},
			outcome: audit.OutcomeFailed("flm_1037"),
		},
		{
			name: "the flow does not exist", flowID: "missing",
			arrange: func() {}, outcome: audit.OutcomeFailed("flm_1003"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.shareWithChild()
			tc.arrange()
			value := tc.value
			if value == nil {
				value = testAuditVariableValue
			}

			_, svcErr := s.service.SetVariables(adminContext(), tc.flowID, childOU, SetVariablesRequest{
				Values: map[string]interface{}{testAuditVariableName: value}})

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			flowTarget := audit.Ref{Type: audit.TargetTypeFlow, ID: tc.flowID}
			s.requireOnlyEvent(audit.EventFlowVariablesUpdate, audit.ActionUpdate, flowTarget, tc.outcome)
		})
	}
}

// The event describes the write, not the read of the stored values that follows it: a caller who may update
// the values but not view them still wrote them, and a read that fails afterwards does not undo the write.
func (s *FlowSharingAuditTestSuite) TestSetVariables_RecordsTheWriteWhateverTheReadBackDoes() {
	stored := sharing.ResolvedValues{OUID: childOU, Visible: true, Values: map[string]sharing.ResolvedValue{
		sharingKey(testAuditVariableName): {Value: []string{testAuditVariableValue}, Source: sharing.SourceOverlay,
			Editable: true},
	}}
	cases := []struct {
		name       string
		mayView    bool
		readBack   func()
		wantValues map[string]interface{}
		wantErr    bool
	}{
		{
			name: "the caller may view the values", mayView: true,
			readBack: func() {
				s.sharing.EXPECT().ResolveOverlayValues(mock.Anything, FlowSharingType, flowIDVar, childOU).
					Return(stored, nil).Once()
			},
			wantValues: map[string]interface{}{testAuditVariableName: testAuditVariableValue},
		},
		{
			name: "the caller may update but not view the values",
			readBack: func() {
				s.sharing.EXPECT().ResolveOverlayValues(mock.Anything, FlowSharingType, flowIDVar, childOU).
					Return(stored, nil).Once()
			},
			wantValues: map[string]interface{}{testAuditVariableName: testAuditVariableValue},
		},
		{
			name: "the read of the stored values fails", mayView: true,
			readBack: func() {
				s.sharing.EXPECT().ResolveOverlayValues(mock.Anything, FlowSharingType, flowIDVar, childOU).
					Return(sharing.ResolvedValues{}, &tidcommon.InternalServerError).Once()
			},
			wantErr: true,
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.shareWithChild()
			authz := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
			authz.On("IsActionAllowed", mock.Anything, security.ActionUpdateFlow, mock.Anything).
				Return(true, nil).Maybe()
			authz.On("IsActionAllowed", mock.Anything, security.ActionReadFlow, mock.Anything).
				Return(tc.mayView, nil).Maybe()
			s.service.authzService = authz
			s.sharing.EXPECT().ReplaceOverlayValues(mock.Anything, FlowSharingType, flowIDVar, childOU,
				mock.Anything, mock.Anything).Return(nil).Once()
			tc.readBack()

			resp, svcErr := s.service.SetVariables(adminContext(), flowIDVar, childOU, SetVariablesRequest{
				Values: map[string]interface{}{testAuditVariableName: testAuditVariableValue}})

			s.Equal(tc.wantErr, svcErr != nil)
			if !tc.wantErr {
				s.Require().NotNil(resp)
				s.Equal(tc.wantValues, resp.Values)
			}
			s.requireOnlyEvent(audit.EventFlowVariablesUpdate, audit.ActionUpdate,
				audit.Ref{Type: audit.TargetTypeFlow, ID: flowIDVar}, audit.OutcomeSucceeded)
		})
	}
}

// A bus that cannot take the event never fails the change it describes.
func (s *FlowSharingAuditTestSuite) TestCreateSharingPolicy_SucceedsWhenTheBusRefusesTheEvent() {
	failing := eventbusmock.NewEventBusInterfaceMock(s.T())
	failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("audit queue is full")).Once()
	s.service.bus = failing
	s.sharing.EXPECT().CreatePolicy(mock.Anything, FlowSharingType, flowIDVar, ownerOU, mock.Anything).
		Return(flowPolicy(ownerOU), nil).Once()

	policy, svcErr := s.service.CreateSharingPolicy(adminContext(), flowIDVar, sharing.PolicyRequest{})

	s.Nil(svcErr)
	s.Equal(testAuditPolicyID, policy.ID)
}
