// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/agent/model"
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/identitygovernance"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
	"github.com/thunder-id/thunderid/tests/mocks/identitygovernancemock"
	"github.com/thunder-id/thunderid/tests/mocks/inboundclientmock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

const (
	testAuditAdminID      = "admin-1"
	testAuditAgentName    = "audited-billing-agent"
	testAuditClientSecret = "audit-test-agent-secret-0123456789"
)

// publishedEvent is one audit event the agent service published.
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
	require.NoError(t, audit.Subscribe(registry, "agent-service-test", sink))
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

// AgentServiceAuditTestSuite tests the audit events of the agents the agent service creates, updates and
// deletes.
type AgentServiceAuditTestSuite struct {
	suite.Suite
	entities   *entitymock.EntityServiceInterfaceMock
	inbound    *inboundclientmock.InboundClientServiceInterfaceMock
	ous        *oumock.OrganizationUnitServiceInterfaceMock
	authz      *sysauthzmock.SystemAuthorizationServiceInterfaceMock
	governance *identitygovernancemock.IdentityGovernanceServiceInterfaceMock
	types      *entitytypemock.EntityTypeServiceInterfaceMock
	service    *agentService
}

func TestAgentServiceAuditTestSuite(t *testing.T) {
	suite.Run(t, new(AgentServiceAuditTestSuite))
}

func (s *AgentServiceAuditTestSuite) SetupTest() {
	s.reset()
}

func (s *AgentServiceAuditTestSuite) SetupSubTest() {
	s.reset()
}

func (s *AgentServiceAuditTestSuite) reset() {
	s.entities = entitymock.NewEntityServiceInterfaceMock(s.T())
	s.inbound = inboundclientmock.NewInboundClientServiceInterfaceMock(s.T())
	s.ous = oumock.NewOrganizationUnitServiceInterfaceMock(s.T())
	s.authz = sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	s.authz.On("CanControlEntity", mock.Anything, mock.Anything).Return((*tidcommon.ServiceError)(nil)).Maybe()
	s.governance = identitygovernancemock.NewIdentityGovernanceServiceInterfaceMock(s.T())
	// The agent's type sits in the agent's organization unit, so the placement check passes.
	s.types = entitytypemock.NewEntityTypeServiceInterfaceMock(s.T())
	s.types.On("GetEntityTypeByHandle", mock.Anything, entitytype.TypeCategoryAgent, mock.Anything).
		Maybe().Return(&entitytype.EntityType{Handle: testAgentType, OUID: testOUID}, (*tidcommon.ServiceError)(nil))
	s.service = &agentService{
		logger:               log.GetLogger().With(log.String(log.LoggerKeyComponentName, "AgentService")),
		authzService:         s.authz,
		entityService:        s.entities,
		inboundClientService: s.inbound,
		entityTypeService:    s.types,
		ouService:            s.ous,
		dependencyRegistry:   noopDepRegistry{},
		governance:           s.governance,
		transactioner:        transaction.NewNoOpTransactioner(),
		bus:                  discardingBus{},
	}
	regeneratesThroughTheHolder(s.service, s.entities)
}

// record replaces the service's bus with a recording one and returns what it published.
func (s *AgentServiceAuditTestSuite) record() func() []publishedEvent {
	bus, published := newRecordingBus(s.T())
	s.service.bus = bus
	return published
}

// access makes the service's access check answer allowed once.
func (s *AgentServiceAuditTestSuite) access(allowed bool) {
	s.authz.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
		Return(allowed, (*tidcommon.ServiceError)(nil)).Once()
}

// ouResolvedForCreate makes the organization unit check a create runs before its access check pass once.
func (s *AgentServiceAuditTestSuite) ouResolvedForCreate() {
	s.ous.On("IsOrganizationUnitExists", mock.Anything, testOUID).
		Return(true, (*tidcommon.ServiceError)(nil)).Once()
}

// validatedForCreate makes the checks a create runs after its access check pass once.
func (s *AgentServiceAuditTestSuite) validatedForCreate() {
	s.inbound.On("ResolveInboundAuthProfileHandles", mock.Anything, mock.Anything).Return(nil).Once()
	s.inbound.On("Validate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Once()
}

// inboundRecreated makes an update find no OAuth client for the agent and create the one it asks for.
func (s *AgentServiceAuditTestSuite) inboundRecreated() {
	s.inbound.On("GetOAuthProfileByEntityID", mock.Anything, testAgentID).
		Return((*providers.OAuthProfile)(nil), inboundclient.ErrInboundClientNotFound).Once()
	s.inbound.On("GetInboundClientByEntityID", mock.Anything, testAgentID).
		Return((*inboundmodel.InboundClient)(nil), inboundclient.ErrInboundClientNotFound).Once()
	s.inbound.On("ResolveInboundAuthProfileHandles", mock.Anything, mock.Anything).Return(nil).Once()
	s.inbound.On("CreateInboundClient", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Once()
}

// ouHandleRead makes the response's organization unit handle lookup answer once.
func (s *AgentServiceAuditTestSuite) ouHandleRead() {
	s.ous.On("GetOrganizationUnitHandlesByIDs", mock.Anything, []string{testOUID}).
		Return(map[string]string{}, (*tidcommon.ServiceError)(nil)).Once()
}

// inboundDeleted makes a delete find no inbound client of the agent to remove, once.
func (s *AgentServiceAuditTestSuite) inboundDeleted() {
	s.inbound.On("DeleteInboundClient", mock.Anything, testAgentID).
		Return(inboundclient.ErrInboundClientNotFound).Once()
}

// existing makes the entity service hold the agent as an agent profile.
func (s *AgentServiceAuditTestSuite) existing() {
	s.entities.On("GetEntityProfile", mock.Anything, testAgentID).
		Return(buildAgentEntityFixture(testAuditAgentName, "", "", ""), nil).Once()
}

// adminContext is the context of a management call made by the admin.
func adminContext() context.Context {
	return security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest(testAuditAdminID, testOUID, "", nil, nil))
}

// confidentialAgent is an agent whose OAuth client authenticates with the client secret it sends.
func confidentialAgent() *providers.Agent {
	return &providers.Agent{
		Type:       testAgentType,
		OUID:       testOUID,
		Attributes: json.RawMessage(`{"name":"` + testAuditAgentName + `"}`),
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{{
			Type: providers.OAuthInboundAuthType,
			OAuthConfig: &providers.OAuthConfigWithSecret{
				ClientSecret:            testAuditClientSecret,
				GrantTypes:              []providers.GrantType{providers.GrantTypeClientCredentials},
				TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
			},
		}},
	}
}

// updateRequest is a full replacement of the agent that keeps it confidential with a new client secret.
func updateRequest() *model.UpdateAgentRequest {
	agent := confidentialAgent()
	return &model.UpdateAgentRequest{
		Type:              agent.Type,
		OUID:              agent.OUID,
		Attributes:        agent.Attributes,
		InboundAuthConfig: agent.InboundAuthConfig,
	}
}

// requireOnlyEvent checks that exactly one event was published, named name, for an attempt by the admin
// on the agent with agentID, and that it records neither the agent's credentials nor its attributes. An
// attempt that succeeded or was denied and names an agent has it as subject too; a failed one has none.
func (s *AgentServiceAuditTestSuite) requireOnlyEvent(events []publishedEvent, name string,
	action audit.Action, agentID string, outcome audit.Outcome) {
	s.T().Helper()
	s.Require().Len(events, 1)
	event := events[0]
	agent := audit.Ref{Type: audit.TargetTypeAgent, ID: agentID}
	s.Equal(name, event.name)
	s.Equal(agent, event.payload.Target())
	s.Equal(action, event.payload.Action())
	s.Equal(outcome, event.payload.Outcome())
	s.Equal(audit.Actor{Type: audit.ActorTypeUser, ID: testAuditAdminID}, event.payload.Actor())
	s.Empty(event.payload.Details(), "no attribute of the agent is recorded")
	if outcome.Result == audit.ResultFailed || agentID == "" {
		s.Equal(audit.Ref{}, event.payload.Subject())
	} else {
		s.Equal(agent, event.payload.Subject())
	}
	recorded := fmt.Sprintf("%+v", event.payload)
	s.NotContains(recorded, testAuditClientSecret, "a client secret is never recorded")
	s.NotContains(recorded, testAuditAgentName, "no attribute value is recorded")
}

func (s *AgentServiceAuditTestSuite) TestCreateAgent_PublishesTheCreatedAgent() {
	published := s.record()
	s.ouResolvedForCreate()
	s.access(true)
	s.validatedForCreate()
	s.entities.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(buildAgentEntityFixture(testAuditAgentName, "", "", ""), nil).Once()
	s.inbound.On("CreateInboundClient", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Once()
	s.ouHandleRead()

	created, svcErr := s.service.CreateAgent(adminContext(), confidentialAgent())

	s.Require().Nil(svcErr)
	s.Require().NotEmpty(created.ID)
	s.Require().Len(created.InboundAuthConfig, 1)
	s.Require().NotEmpty(created.InboundAuthConfig[0].OAuthConfig.ClientID)
	events := published()
	s.requireOnlyEvent(events, audit.EventAgentCreate, audit.ActionCreate, created.ID, audit.OutcomeSucceeded)
	s.NotContains(fmt.Sprintf("%+v", events[0].payload), created.InboundAuthConfig[0].OAuthConfig.ClientID,
		"the client ID is never recorded")
}

func (s *AgentServiceAuditTestSuite) TestCreateAgent_PublishesTheSameNameWhenItDoesNotSucceed() {
	cases := []struct {
		name    string
		agent   func() *providers.Agent
		arrange func()
		outcome audit.Outcome
	}{
		{
			name: "the request has no type",
			agent: func() *providers.Agent {
				agent := confidentialAgent()
				agent.Type = ""
				return agent
			},
			outcome: audit.OutcomeFailed("agt_1010"),
		},
		{
			name:  "the store fails",
			agent: confidentialAgent,
			arrange: func() {
				s.ouResolvedForCreate()
				s.access(true)
				s.validatedForCreate()
				s.entities.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return((*providers.Entity)(nil), errors.New("store failure")).Once()
			},
			outcome: audit.OutcomeFailed("sse_5000"),
		},
		{
			name:  "the caller may not create agents in the organization unit",
			agent: confidentialAgent,
			arrange: func() {
				s.ouResolvedForCreate()
				s.access(false)
			},
			outcome: audit.OutcomeDenied("forbidden"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			published := s.record()
			if tc.arrange != nil {
				tc.arrange()
			}

			created, svcErr := s.service.CreateAgent(adminContext(), tc.agent())

			s.Nil(created)
			s.Require().NotNil(svcErr)
			s.requireOnlyEvent(published(), audit.EventAgentCreate, audit.ActionCreate, "", tc.outcome)
		})
	}
}

// An update that sets a new client secret is recorded as an update, without the secret.
func (s *AgentServiceAuditTestSuite) TestUpdateAgent_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		arrange func()
		outcome audit.Outcome
	}{
		{
			name: "the agent is updated",
			arrange: func() {
				s.existing()
				s.access(true)
				s.inboundRecreated()
				s.entities.On("UpdateEntity", mock.Anything, testAgentID, mock.Anything).
					Return(&providers.Entity{ID: testAgentID}, nil).Once()
				s.entities.On("UpdateSystemCredentials", mock.Anything, testAgentID, mock.Anything).
					Return(nil).Once()
				s.governance.EXPECT().ReadEntityAccess(mock.Anything, mock.Anything).
					Return(identitygovernance.AccessRead{
						Status: providers.AccessStatusReport{Value: providers.AccessStatusActive},
					}).Once()
				s.ouHandleRead()
			},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name: "the caller may not change the agent",
			arrange: func() {
				s.existing()
				s.access(false)
			},
			outcome: audit.OutcomeDenied("forbidden"),
		},
		{
			name: "the agent does not exist",
			arrange: func() {
				s.entities.On("GetEntityProfile", mock.Anything, testAgentID).
					Return((*providers.Entity)(nil), entity.ErrEntityNotFound).Once()
			},
			outcome: audit.OutcomeFailed("agt_1004"),
		},
		{
			name: "the agent is declarative",
			arrange: func() {
				declared := buildAgentEntityFixture(testAuditAgentName, "", "", "")
				declared.IsReadOnly = true
				s.entities.On("GetEntityProfile", mock.Anything, testAgentID).Return(declared, nil).Once()
			},
			outcome: audit.OutcomeFailed("agt_1027"),
		},
		{
			name: "the store fails",
			arrange: func() {
				s.existing()
				s.access(true)
				s.inboundRecreated()
				s.entities.On("UpdateEntity", mock.Anything, testAgentID, mock.Anything).
					Return((*providers.Entity)(nil), errors.New("store failure")).Once()
			},
			outcome: audit.OutcomeFailed("sse_5000"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			published := s.record()
			tc.arrange()

			_, svcErr := s.service.UpdateAgent(adminContext(), testAgentID, updateRequest())

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(published(), audit.EventAgentUpdate, audit.ActionUpdate, testAgentID, tc.outcome)
		})
	}
}

// A delete of an agent that does not exist fails, so it is recorded as failed with the ID asked for.
func (s *AgentServiceAuditTestSuite) TestDeleteAgent_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		arrange func()
		outcome audit.Outcome
	}{
		{
			name: "the agent is deleted",
			arrange: func() {
				s.existing()
				s.access(true)
				s.inboundDeleted()
				s.entities.On("DeleteEntity", mock.Anything, testAgentID).Return(nil).Once()
			},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name: "the caller may not delete the agent",
			arrange: func() {
				s.existing()
				s.access(false)
			},
			outcome: audit.OutcomeDenied("forbidden"),
		},
		{
			name: "the agent does not exist",
			arrange: func() {
				s.entities.On("GetEntityProfile", mock.Anything, testAgentID).
					Return((*providers.Entity)(nil), entity.ErrEntityNotFound).Once()
			},
			outcome: audit.OutcomeFailed("agt_1004"),
		},
		{
			name: "the ID names an entity that is not an agent",
			arrange: func() {
				user := buildAgentEntityFixture("", "", "", "")
				user.Category = providers.EntityCategoryUser
				s.entities.On("GetEntityProfile", mock.Anything, testAgentID).Return(user, nil).Once()
			},
			outcome: audit.OutcomeFailed("agt_1004"),
		},
		{
			name: "the dependents cannot be removed",
			arrange: func() {
				s.existing()
				s.access(true)
				s.service.dependencyRegistry = noopDepRegistry{cascadeErr: errors.New("cascade failure")}
			},
			outcome: audit.OutcomeFailed("sse_5000"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			published := s.record()
			tc.arrange()

			svcErr := s.service.DeleteAgent(adminContext(), testAgentID)

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(published(), audit.EventAgentDelete, audit.ActionDelete, testAgentID, tc.outcome)
		})
	}
}

// Rotating an agent's client secret is a change to the agent, so each attempt is recorded as an update.
func (s *AgentServiceAuditTestSuite) TestRegenerateAgentSecret_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		arrange func()
		outcome audit.Outcome
	}{
		{
			name: "the caller may not change the agent",
			arrange: func() {
				s.existing()
				s.access(false)
			},
			outcome: audit.OutcomeDenied("forbidden"),
		},
		{
			name: "the agent does not exist",
			arrange: func() {
				s.entities.On("GetEntityProfile", mock.Anything, testAgentID).
					Return((*providers.Entity)(nil), entity.ErrEntityNotFound).Once()
			},
			outcome: audit.OutcomeFailed("agt_1004"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			published := s.record()
			tc.arrange()

			_, svcErr := s.service.RegenerateAgentSecret(adminContext(), testAgentID)

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(published(), audit.EventAgentUpdate, audit.ActionUpdate, testAgentID, tc.outcome)
		})
	}
}

// Setting an agent's credentials records the attempt on the agent, never the credential or its type.
func (s *AgentServiceAuditTestSuite) TestUpdateAgentCredentials_PublishesOneEventPerAttempt() {
	credentials := json.RawMessage(`{"password":"` + testAuditClientSecret + `"}`)
	cases := []struct {
		name    string
		agentID string
		arrange func()
		outcome audit.Outcome
	}{
		{
			name:    "the credentials are set",
			agentID: testAgentID,
			arrange: func() {
				s.existing()
				s.access(true)
				s.entities.On("UpdateCredentials", mock.Anything, testAgentID, credentials).Return(nil).Once()
			},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name:    "the caller may not change the agent",
			agentID: testAgentID,
			arrange: func() {
				s.existing()
				s.access(false)
			},
			outcome: audit.OutcomeDenied("forbidden"),
		},
		{
			name:    "the agent does not exist",
			agentID: testAgentID,
			arrange: func() {
				s.entities.On("GetEntityProfile", mock.Anything, testAgentID).
					Return((*providers.Entity)(nil), entity.ErrEntityNotFound).Once()
			},
			outcome: audit.OutcomeFailed("agt_1004"),
		},
		{
			name:    "the agent's type does not declare the credential",
			agentID: testAgentID,
			arrange: func() {
				s.existing()
				s.access(true)
				s.entities.On("UpdateCredentials", mock.Anything, testAgentID, credentials).
					Return(entity.ErrInvalidCredential).Once()
			},
			outcome: audit.OutcomeFailed("agt_1016"),
		},
		{
			name:    "no agent is named",
			arrange: func() {},
			outcome: audit.OutcomeFailed("agt_1008"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			published := s.record()
			tc.arrange()

			svcErr := s.service.UpdateAgentCredentials(adminContext(), tc.agentID, credentials)

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			events := published()
			s.requireOnlyEvent(events, audit.EventAgentCredentialUpdate, audit.ActionUpdate, tc.agentID,
				tc.outcome)
			s.NotContains(fmt.Sprintf("%+v", events[0].payload), "password", "no credential type is recorded")
		})
	}
}

// A bus that cannot take the event never fails the change it describes.
func (s *AgentServiceAuditTestSuite) TestUpdateAgentCredentials_SucceedsWhenTheBusRefusesTheEvent() {
	failing := eventbusmock.NewEventBusInterfaceMock(s.T())
	failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("audit queue is full")).Once()
	s.service.bus = failing
	s.existing()
	s.access(true)
	s.entities.On("UpdateCredentials", mock.Anything, testAgentID, mock.Anything).Return(nil).Once()

	s.Nil(s.service.UpdateAgentCredentials(adminContext(), testAgentID,
		json.RawMessage(`{"password":"`+testAuditClientSecret+`"}`)))
}

// A bus that cannot take the event never fails the change it describes.
func (s *AgentServiceAuditTestSuite) TestDeleteAgent_SucceedsWhenTheBusRefusesTheEvent() {
	failing := eventbusmock.NewEventBusInterfaceMock(s.T())
	failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("audit queue is full")).Once()
	s.service.bus = failing
	s.existing()
	s.access(true)
	s.inboundDeleted()
	s.entities.On("DeleteEntity", mock.Anything, testAgentID).Return(nil).Once()

	s.Nil(s.service.DeleteAgent(adminContext(), testAgentID))
}
