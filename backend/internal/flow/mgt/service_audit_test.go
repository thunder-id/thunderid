// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowmgt

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/executormock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/interceptormock"
	"github.com/thunder-id/thunderid/tests/mocks/sharingmock"
)

const (
	testAuditAdminID = "admin-1"
	testAuditFlowID  = "0190b2c8-7a1e-7c3e-9d2a-5f6e7a8b9c0d"
	// testAuditFlowName, testAuditFlowHandle and testAuditNodeID are parts of the definition; no event may
	// carry them.
	testAuditFlowName   = "Audited Sign In Flow"
	testAuditFlowHandle = "audited-sign-in"
	testAuditNodeID     = "audited-prompt-node"
)

// publishedEvent is one audit event the flow management service published.
type publishedEvent struct {
	name    string
	payload audit.Payload
}

// recordingSink keeps every audit event it is handed, in order.
type recordingSink struct {
	mu     sync.Mutex
	events []publishedEvent
}

func (r *recordingSink) Handle(_ context.Context, env eventbus.Envelope, payload audit.Payload) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, publishedEvent{name: env.Name, payload: payload})
	return nil
}

// newRecordingBus returns an event bus with a sink subscribed the way the binaries subscribe theirs, so
// an event audit refuses never reaches it, and a function that stops the bus and returns the audit
// events written in order.
func newRecordingBus(t *testing.T) (eventbus.EventBusInterface, func() []publishedEvent) {
	bus, registry, stopper := eventbus.Initialize()
	t.Cleanup(stopper.Stop)

	sink := &recordingSink{}
	require.NoError(t, audit.Subscribe(registry, "flow-mgt-service-test", sink))
	return bus, func() []publishedEvent {
		stopper.Stop()
		sink.mu.Lock()
		defer sink.mu.Unlock()
		return sink.events
	}
}

// discardingBus is an event bus that drops every event, for a test the audit event does not matter to.
// A test of what the service publishes uses newRecordingBus.
type discardingBus struct{}

func (discardingBus) Publish(context.Context, *eventbus.Event) error {
	return nil
}

// adminContext is the context of a management call made by the admin.
func adminContext() context.Context {
	return security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest(testAuditAdminID, "ou-1", "", nil, nil))
}

// auditedFlowDefinition is an authentication flow as an admin would send it.
func auditedFlowDefinition() *FlowDefinition {
	return &FlowDefinition{
		OUID:     "ou-1",
		Handle:   testAuditFlowHandle,
		Name:     testAuditFlowName,
		FlowType: providers.FlowTypeAuthentication,
		Nodes: []providers.NodeDefinition{
			{ID: "start", Type: "START", OnSuccess: testAuditNodeID},
			{ID: testAuditNodeID, Type: "PROMPT", Next: "end"},
			{ID: "end", Type: "END"},
		},
	}
}

// storedFlow is the flow the store holds for flowDef under the audited ID.
func storedFlow(flowDef *FlowDefinition, version int) *providers.CompleteFlowDefinition {
	return &providers.CompleteFlowDefinition{
		ID:            testAuditFlowID,
		OUID:          flowDef.OUID,
		Handle:        flowDef.Handle,
		Name:          flowDef.Name,
		FlowType:      flowDef.FlowType,
		ActiveVersion: version,
		Nodes:         flowDef.Nodes,
	}
}

type FlowMgtServiceAuditTestSuite struct {
	suite.Suite
	mockStore        *flowStoreInterfaceMock
	mockGraphBuilder *graphBuilderInterfaceMock
	mockValidator    *FlowValidatorInterfaceMock
	mockOU           *ouProviderMock
	mockSharing      *sharingmock.SharingServiceInterfaceMock
}

func TestFlowMgtServiceAuditTestSuite(t *testing.T) {
	suite.Run(t, new(FlowMgtServiceAuditTestSuite))
}

func (s *FlowMgtServiceAuditTestSuite) SetupTest() {
	s.mockStore = newFlowStoreInterfaceMock(s.T())
	s.mockGraphBuilder = newGraphBuilderInterfaceMock(s.T())
	s.mockValidator = NewFlowValidatorInterfaceMock(s.T())
	s.mockOU = newOuProviderMock(s.T())
	s.mockOU.EXPECT().ResolveOwner(mock.Anything, "ou-1", "", mock.Anything).Return("ou-1", nil).Maybe()
	s.mockOU.EXPECT().ResolveOwnerForUpdate(mock.Anything, "ou-1", "", "ou-1", mock.Anything).
		Return("ou-1", nil).Maybe()
	// A flow that was never shared has no policies, saved values or sharees.
	s.mockSharing = sharingmock.NewSharingServiceInterfaceMock(s.T())
	s.mockSharing.EXPECT().ListPolicies(mock.Anything, FlowSharingType, mock.Anything).
		Return([]sharing.Policy{}, nil).Maybe()
	s.mockSharing.EXPECT().ListOverlayValues(mock.Anything, FlowSharingType, mock.Anything).
		Return(map[string]map[string][]string{}, nil).Maybe()
	s.mockSharing.EXPECT().DeletePoliciesForResource(mock.Anything, FlowSharingType, mock.Anything).
		Return(nil).Maybe()
	s.mockStore.EXPECT().InvalidateCache(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()
	s.mockGraphBuilder.EXPECT().InvalidateCache(mock.Anything, mock.Anything).Maybe()

	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime("test", &config.Config{
		Flow: engineconfig.FlowConfig{AutoInferRegistration: false},
	}))
}

func (s *FlowMgtServiceAuditTestSuite) SetupSubTest() {
	s.SetupTest()
}

func (s *FlowMgtServiceAuditTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

// service builds the service over the suite mocks, publishing on bus.
func (s *FlowMgtServiceAuditTestSuite) service(bus eventbus.EventBusInterface) *flowMgtService {
	return newFlowMgtService(s.mockStore, newFlowInferenceServiceInterfaceMock(s.T()), s.mockGraphBuilder,
		executormock.NewExecutorRegistryInterfaceMock(s.T()),
		interceptormock.NewInterceptorRegistryInterfaceMock(s.T()), s.mockValidator, nil, &stubTransactioner{},
		nil, s.mockOU, nil, nil, s.mockSharing, nil, bus)
}

// requireOnlyEvent checks that exactly one event was published, named name, for an attempt by the admin
// on the flow with flowID, and that no part of the definition is recorded. An attempt that succeeded has
// the flow as subject too; a failed one has no subject.
func (s *FlowMgtServiceAuditTestSuite) requireOnlyEvent(events []publishedEvent, name string, action audit.Action,
	flowID string, outcome audit.Outcome) {
	s.T().Helper()
	s.Require().Len(events, 1)
	event := events[0]
	flow := audit.Ref{Type: audit.TargetTypeFlow, ID: flowID}
	s.Equal(name, event.name)
	s.Equal(flow, event.payload.Target())
	s.Equal(action, event.payload.Action())
	s.Equal(outcome, event.payload.Outcome())
	s.Equal(audit.Actor{Type: audit.ActorTypeUser, ID: testAuditAdminID}, event.payload.Actor())
	s.Empty(event.payload.Details(), "no part of the flow is recorded")
	recorded := fmt.Sprintf("%+v", event.payload)
	for _, value := range []string{testAuditFlowName, testAuditFlowHandle, testAuditNodeID} {
		s.NotContains(recorded, value, "no part of the definition is recorded")
	}
	if outcome.Result == audit.ResultFailed || flowID == "" {
		s.Equal(audit.Ref{}, event.payload.Subject(), "a failed attempt may name no flow")
	} else {
		s.Equal(flow, event.payload.Subject())
	}
}

func (s *FlowMgtServiceAuditTestSuite) TestCreateFlow_PublishesTheCreatedFlow() {
	bus, published := newRecordingBus(s.T())
	flowDef := auditedFlowDefinition()
	s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, flowDef).Return(nil).Once()
	s.mockStore.EXPECT().IsFlowExistsByHandle(mock.Anything, testAuditFlowHandle, providers.FlowTypeAuthentication).
		Return(false, nil).Once()
	s.mockStore.EXPECT().CreateFlow(mock.Anything, mock.Anything, flowDef).
		RunAndReturn(func(_ context.Context, flowID string, def *FlowDefinition) (
			*providers.CompleteFlowDefinition, error) {
			created := storedFlow(def, 1)
			created.ID = flowID
			return created, nil
		}).Once()

	created, svcErr := s.service(bus).CreateFlow(adminContext(), flowDef)

	s.Require().Nil(svcErr)
	s.Require().NotEmpty(created.ID)
	s.requireOnlyEvent(published(), audit.EventFlowCreate, audit.ActionCreate, created.ID, audit.OutcomeSucceeded)
}

func (s *FlowMgtServiceAuditTestSuite) TestCreateFlow_PublishesTheSameNameWhenItFails() {
	cases := []struct {
		name    string
		arrange func(flowDef *FlowDefinition)
		outcome audit.Outcome
	}{
		{
			name: "the definition is invalid",
			arrange: func(flowDef *FlowDefinition) {
				s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, flowDef).
					Return(&ErrorInvalidFlowType).Once()
			},
			outcome: audit.OutcomeFailed("flm_1004"),
		},
		{
			name: "the handle is taken",
			arrange: func(flowDef *FlowDefinition) {
				s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, flowDef).Return(nil).Once()
				s.mockStore.EXPECT().IsFlowExistsByHandle(mock.Anything, testAuditFlowHandle,
					providers.FlowTypeAuthentication).Return(true, nil).Once()
			},
			outcome: audit.OutcomeFailed("flm_1013"),
		},
		{
			name: "the store fails",
			arrange: func(flowDef *FlowDefinition) {
				s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, flowDef).Return(nil).Once()
				s.mockStore.EXPECT().IsFlowExistsByHandle(mock.Anything, testAuditFlowHandle,
					providers.FlowTypeAuthentication).Return(false, nil).Once()
				s.mockStore.EXPECT().CreateFlow(mock.Anything, mock.Anything, flowDef).
					Return(nil, errors.New("store failure")).Once()
			},
			outcome: audit.OutcomeFailed("sse_5000"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			flowDef := auditedFlowDefinition()
			tc.arrange(flowDef)

			created, svcErr := s.service(bus).CreateFlow(adminContext(), flowDef)

			s.Nil(created)
			s.Require().NotNil(svcErr)
			s.requireOnlyEvent(published(), audit.EventFlowCreate, audit.ActionCreate, "", tc.outcome)
		})
	}
}

// UpdateFlow and UpdateFlowWithSharing are both an update, and each records it once.
func (s *FlowMgtServiceAuditTestSuite) TestUpdateFlow_PublishesTheUpdatedFlow() {
	cases := []struct {
		name   string
		update func(service *flowMgtService, flowDef *FlowDefinition) *tidcommon.ServiceError
	}{
		{
			name: "an update",
			update: func(service *flowMgtService, flowDef *FlowDefinition) *tidcommon.ServiceError {
				_, svcErr := service.UpdateFlow(adminContext(), testAuditFlowID, flowDef)
				return svcErr
			},
		},
		{
			name: "an update with no sharing policies",
			update: func(service *flowMgtService, flowDef *FlowDefinition) *tidcommon.ServiceError {
				_, svcErr := service.UpdateFlowWithSharing(adminContext(), testAuditFlowID, flowDef, nil)
				return svcErr
			},
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			flowDef := auditedFlowDefinition()
			s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, flowDef).Return(nil).Once()
			s.mockStore.EXPECT().GetFlowByID(mock.Anything, testAuditFlowID).Return(storedFlow(flowDef, 1), nil).Once()
			s.mockStore.EXPECT().UpdateFlow(mock.Anything, testAuditFlowID, flowDef).
				Return(storedFlow(flowDef, 2), nil).Once()

			svcErr := tc.update(s.service(bus), flowDef)

			s.Require().Nil(svcErr)
			s.requireOnlyEvent(published(), audit.EventFlowUpdate, audit.ActionUpdate, testAuditFlowID,
				audit.OutcomeSucceeded)
		})
	}
}

func (s *FlowMgtServiceAuditTestSuite) TestUpdateFlow_PublishesTheCallersIDWhenItFails() {
	cases := []struct {
		name    string
		arrange func(flowDef *FlowDefinition)
		outcome audit.Outcome
	}{
		{
			name: "the flow does not exist",
			arrange: func(flowDef *FlowDefinition) {
				s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, flowDef).Return(nil).Once()
				s.mockStore.EXPECT().GetFlowByID(mock.Anything, testAuditFlowID).Return(nil, errFlowNotFound).Once()
			},
			outcome: audit.OutcomeFailed("flm_1003"),
		},
		{
			name: "the flow is declared in a file",
			arrange: func(flowDef *FlowDefinition) {
				declared := storedFlow(flowDef, 1)
				declared.IsReadOnly = true
				s.mockValidator.EXPECT().ValidateFlowDefinition(mock.Anything, flowDef).Return(nil).Once()
				s.mockStore.EXPECT().GetFlowByID(mock.Anything, testAuditFlowID).Return(declared, nil).Once()
			},
			outcome: audit.OutcomeFailed("flm_1017"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			flowDef := auditedFlowDefinition()
			tc.arrange(flowDef)

			updated, svcErr := s.service(bus).UpdateFlow(adminContext(), testAuditFlowID, flowDef)

			s.Nil(updated)
			s.Require().NotNil(svcErr)
			s.requireOnlyEvent(published(), audit.EventFlowUpdate, audit.ActionUpdate, testAuditFlowID, tc.outcome)
		})
	}
}

func (s *FlowMgtServiceAuditTestSuite) TestDeleteFlow_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		arrange func()
		outcome audit.Outcome
	}{
		{
			name: "the flow is deleted",
			arrange: func() {
				s.mockStore.EXPECT().GetFlowByID(mock.Anything, testAuditFlowID).
					Return(storedFlow(auditedFlowDefinition(), 1), nil).Once()
				s.mockStore.EXPECT().DeleteFlow(mock.Anything, testAuditFlowID).Return(nil).Once()
			},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name: "the flow is declared in a file",
			arrange: func() {
				declared := storedFlow(auditedFlowDefinition(), 1)
				declared.IsReadOnly = true
				s.mockStore.EXPECT().GetFlowByID(mock.Anything, testAuditFlowID).Return(declared, nil).Once()
			},
			outcome: audit.OutcomeFailed("flm_1017"),
		},
		{
			name: "the store fails",
			arrange: func() {
				s.mockStore.EXPECT().GetFlowByID(mock.Anything, testAuditFlowID).
					Return(storedFlow(auditedFlowDefinition(), 1), nil).Once()
				s.mockStore.EXPECT().DeleteFlow(mock.Anything, testAuditFlowID).Return(errors.New("db error")).Once()
			},
			outcome: audit.OutcomeFailed("sse_5000"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			tc.arrange()

			svcErr := s.service(bus).DeleteFlow(adminContext(), testAuditFlowID)

			s.Equal(tc.outcome.Result == audit.ResultSucceeded, svcErr == nil)
			s.requireOnlyEvent(published(), audit.EventFlowDelete, audit.ActionDelete, testAuditFlowID, tc.outcome)
		})
	}
}

// A delete of an ID no flow has succeeds, as before, without an event: otherwise any caller could record
// the deletion of an ID it made up.
func (s *FlowMgtServiceAuditTestSuite) TestDeleteFlow_PublishesNothingForAMissingFlow() {
	bus, published := newRecordingBus(s.T())
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testAuditFlowID).Return(nil, errFlowNotFound).Once()

	svcErr := s.service(bus).DeleteFlow(adminContext(), testAuditFlowID)

	s.Nil(svcErr)
	s.Empty(published())
}

// A delete of a missing flow whose leftover sharing cannot be removed fails, and is recorded.
func (s *FlowMgtServiceAuditTestSuite) TestDeleteFlow_PublishesAFailedCleanupOfAMissingFlow() {
	bus, published := newRecordingBus(s.T())
	s.mockSharing = sharingmock.NewSharingServiceInterfaceMock(s.T())
	s.mockSharing.EXPECT().ListPolicies(mock.Anything, FlowSharingType, testAuditFlowID).
		Return(nil, &tidcommon.InternalServerError).Once()
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testAuditFlowID).Return(nil, errFlowNotFound).Once()

	svcErr := s.service(bus).DeleteFlow(adminContext(), testAuditFlowID)

	s.Require().NotNil(svcErr)
	s.requireOnlyEvent(published(), audit.EventFlowDelete, audit.ActionDelete, testAuditFlowID,
		audit.OutcomeFailed("sse_5000"))
}

func (s *FlowMgtServiceAuditTestSuite) TestRestoreFlowVersion_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		version int
		arrange func()
		outcome audit.Outcome
	}{
		{
			name:    "the version is restored",
			version: 1,
			arrange: func() {
				flowDef := auditedFlowDefinition()
				s.mockStore.EXPECT().GetFlowVersion(mock.Anything, testAuditFlowID, 1).
					Return(&FlowVersion{Version: 1, Nodes: flowDef.Nodes}, nil).Once()
				s.mockStore.EXPECT().GetFlowByID(mock.Anything, testAuditFlowID).
					Return(storedFlow(flowDef, 2), nil).Once()
				s.mockStore.EXPECT().RestoreFlowVersion(mock.Anything, testAuditFlowID, 1).
					Return(storedFlow(flowDef, 3), nil).Once()
			},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name:    "the version is not a version number",
			version: 0,
			outcome: audit.OutcomeFailed("flm_1009"),
		},
		{
			name:    "the flow has no such version",
			version: 7,
			arrange: func() {
				s.mockStore.EXPECT().GetFlowVersion(mock.Anything, testAuditFlowID, 7).
					Return(nil, errVersionNotFound).Once()
			},
			outcome: audit.OutcomeFailed("flm_1008"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			if tc.arrange != nil {
				tc.arrange()
			}

			_, svcErr := s.service(bus).RestoreFlowVersion(adminContext(), testAuditFlowID, tc.version)

			s.Equal(tc.outcome.Result == audit.ResultSucceeded, svcErr == nil)
			s.requireOnlyEvent(published(), audit.EventFlowRestore, audit.ActionRestore, testAuditFlowID, tc.outcome)
		})
	}
}

// A bus that cannot take the event never fails the change it describes.
func (s *FlowMgtServiceAuditTestSuite) TestDeleteFlow_SucceedsWhenTheBusRefusesTheEvent() {
	failing := eventbusmock.NewEventBusInterfaceMock(s.T())
	failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("audit queue is full")).Once()
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, testAuditFlowID).
		Return(storedFlow(auditedFlowDefinition(), 1), nil).Once()
	s.mockStore.EXPECT().DeleteFlow(mock.Anything, testAuditFlowID).Return(nil).Once()

	svcErr := s.service(failing).DeleteFlow(adminContext(), testAuditFlowID)

	s.Nil(svcErr)
}
