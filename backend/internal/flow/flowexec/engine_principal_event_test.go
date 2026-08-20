// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowexec

import (
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/observability/observabilitymock"
)

// setupCapturingObservability returns a mock observability service that records every published
// event, so tests can assert on the data an event carries rather than only that it was published.
func setupCapturingObservability(t *testing.T) (
	*observabilitymock.ObservabilityServiceInterfaceMock, *[]*providers.Event) {
	t.Helper()

	// setupMockObservability is called for its side effect of initializing the server runtime with
	// observability enabled; its mock registers a catch-all PublishEvent that would shadow the
	// capturing expectation below, so a dedicated mock is used here.
	setupMockObservability(t)

	captured := &[]*providers.Event{}
	mockObs := &observabilitymock.ObservabilityServiceInterfaceMock{}
	mockObs.On("IsEnabled").Return(true).Maybe()
	mockObs.
		On("PublishEvent", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			*captured = append(*captured, args.Get(1).(*providers.Event))
		}).
		Return().Maybe()

	return mockObs, captured
}

// agentFlowContext returns an engine context for an authentication flow driven by an agent that has
// authenticated a user. The subject is recorded on AuthUser, which is where the authentication
// executors put it — EngineContext.AuthenticatedUser is never written during execution, so a test
// that seeds it would assert against a path production never takes.
func agentFlowContext() *EngineContext {
	ctx := &EngineContext{
		ExecutionID: "flow-exec-1",
		TraceID:     "request-trace-1",
		FlowType:    providers.FlowTypeAuthentication,
		AppID:       "agent-entity-1",
		Application: providers.Application{
			ID:             "agent-entity-1",
			EntityCategory: providers.EntityCategoryAgent,
			InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
				{
					Type:        providers.OAuthInboundAuthType,
					OAuthConfig: &providers.OAuthConfigWithSecret{ClientID: "agent-client-id"},
				},
			},
		},
		ExecutionHistory: make(map[string]*providers.NodeExecutionRecord),
	}
	ctx.AuthUser.SetStateFor("credentials", providers.AuthState{
		EntityReference: &providers.EntityReference{
			EntityID:       "user-123",
			EntityCategory: string(providers.EntityCategoryUser),
			EntityType:     "Person",
		},
	})
	return ctx
}

func TestFlowStartedEvent_ReportsPrincipalAndCorrelation(t *testing.T) {
	mockObs, captured := setupCapturingObservability(t)
	defer config.ResetServerRuntime()

	publishFlowStartedEvent(agentFlowContext(), mockObs)

	if len(*captured) != 1 {
		t.Fatalf("expected 1 published event, got %d", len(*captured))
	}
	evt := (*captured)[0]

	// The whole flow must share one trace, keyed on the execution id, so FLOW_STARTED stitches to
	// its own child events.
	if evt.TraceID != "flow-exec-1" {
		t.Errorf("TraceID = %q, want the execution id %q", evt.TraceID, "flow-exec-1")
	}

	assertEventData(t, evt, map[string]interface{}{
		event.DataKey.ActorType:     event.PrincipalTypeAgent,
		event.DataKey.ClientID:      "agent-client-id",
		event.DataKey.EntityID:      "agent-entity-1",
		event.DataKey.CorrelationID: "flow-exec-1",
		event.DataKey.Subject:       "user-123",
		event.DataKey.SubjectType:   event.PrincipalTypeUser,
	})
}

// subjectFlowContext returns a minimal context whose only authenticated subject is the given entity
// reference, isolating the subject fields from the application-side ones.
func subjectFlowContext(ref providers.EntityReference) *EngineContext {
	ctx := &EngineContext{
		ExecutionID:      "flow-exec-3",
		FlowType:         providers.FlowTypeAuthentication,
		AppID:            "app-3",
		ExecutionHistory: make(map[string]*providers.NodeExecutionRecord),
	}
	ctx.AuthUser.SetStateFor("credentials", providers.AuthState{EntityReference: &ref})
	return ctx
}

// The authentication executors record the resolved entity on AuthUser; AuthenticatedUser carries no
// id during execution, so the subject must be read from AuthUser for it to appear at all.
func TestFlowStartedEvent_SubjectComesFromAuthUser(t *testing.T) {
	mockObs, captured := setupCapturingObservability(t)
	defer config.ResetServerRuntime()

	publishFlowStartedEvent(subjectFlowContext(providers.EntityReference{
		EntityID:       "user-999",
		EntityCategory: string(providers.EntityCategoryUser),
		EntityType:     "Person",
	}), mockObs)

	assertEventData(t, (*captured)[0], map[string]interface{}{
		event.DataKey.Subject:     "user-999",
		event.DataKey.SubjectType: event.PrincipalTypeUser,
	})
}

// An agent can authenticate through a flow, so the subject category must come from the resolved
// entity reference rather than being assumed to be a user.
func TestFlowStartedEvent_AgentSubjectReportsAgentCategory(t *testing.T) {
	mockObs, captured := setupCapturingObservability(t)
	defer config.ResetServerRuntime()

	publishFlowStartedEvent(subjectFlowContext(providers.EntityReference{
		EntityID:       "agent-999",
		EntityCategory: string(providers.EntityCategoryAgent),
	}), mockObs)

	assertEventData(t, (*captured)[0], map[string]interface{}{
		event.DataKey.Subject:     "agent-999",
		event.DataKey.SubjectType: event.PrincipalTypeAgent,
	})
}

func TestFlowStartedEvent_SubjectTypeOmittedWhenCategoryUnknown(t *testing.T) {
	mockObs, captured := setupCapturingObservability(t)
	defer config.ResetServerRuntime()

	publishFlowStartedEvent(subjectFlowContext(providers.EntityReference{EntityID: "user-999"}), mockObs)

	evt := (*captured)[0]
	if got, ok := evt.Data[event.DataKey.SubjectType]; ok {
		t.Errorf("subject_type should be omitted when the category is unknown, got %v", got)
	}
	assertEventData(t, evt, map[string]interface{}{event.DataKey.Subject: "user-999"})
}

// The entity vocabulary spells an application "app" while the reported principal type spells it
// "application", matching the token's sub_type claim. The events must carry the claim's spelling.
func TestFlowStartedEvent_ApplicationReportsApplicationActorType(t *testing.T) {
	mockObs, captured := setupCapturingObservability(t)
	defer config.ResetServerRuntime()

	ctx := agentFlowContext()
	ctx.Application.EntityCategory = providers.EntityCategoryApp

	publishFlowStartedEvent(ctx, mockObs)

	assertEventData(t, (*captured)[0], map[string]interface{}{
		event.DataKey.ActorType: event.PrincipalTypeApplication,
	})
}

func TestFlowStartedEvent_OmitsUnknownPrincipalAndSubjectFields(t *testing.T) {
	mockObs, captured := setupCapturingObservability(t)
	defer config.ResetServerRuntime()

	ctx := &EngineContext{
		ExecutionID:      "flow-exec-2",
		FlowType:         providers.FlowTypeRegistration,
		AppID:            "app-2",
		ExecutionHistory: make(map[string]*providers.NodeExecutionRecord),
	}

	publishFlowStartedEvent(ctx, mockObs)

	evt := (*captured)[0]
	for _, key := range []string{
		event.DataKey.ActorType, event.DataKey.ClientID,
		event.DataKey.Subject, event.DataKey.SubjectType,
	} {
		if _, ok := evt.Data[key]; ok {
			t.Errorf("expected %q to be omitted, got %v", key, evt.Data[key])
		}
	}
	if evt.Data[event.DataKey.CorrelationID] != "flow-exec-2" {
		t.Errorf("correlation_id = %v, want %q", evt.Data[event.DataKey.CorrelationID], "flow-exec-2")
	}
}

func TestNodeExecutionStartedEvent_ReportsSubjectAndPrincipal(t *testing.T) {
	mockObs, captured := setupCapturingObservability(t)
	defer config.ResetServerRuntime()

	node := coremock.NewNodeInterfaceMock(t)
	node.On("GetID").Return("node-1")
	node.On("GetType").Return(common.NodeTypeTaskExecution)

	publishNodeExecutionStartedEvent(agentFlowContext(), node, mockObs)

	assertEventData(t, (*captured)[0], map[string]interface{}{
		event.DataKey.ActorType:     string(providers.EntityCategoryAgent),
		event.DataKey.ClientID:      "agent-client-id",
		event.DataKey.CorrelationID: "flow-exec-1",
		event.DataKey.Subject:       "user-123",
		event.DataKey.SubjectType:   string(providers.EntityCategoryUser),
	})
}

// The engine merges a node's AuthUser into the context only after the node-completed event is
// published, so the node that authenticates the subject must report it from its own response.
// Otherwise the subject first appears one node later and the authenticating node's event — the one an
// auditor reads to see who authenticated — carries none.
func TestNodeExecutionCompletedEvent_SubjectComesFromTheNodeResponse(t *testing.T) {
	mockObs, captured := setupCapturingObservability(t)
	defer config.ResetServerRuntime()

	node := coremock.NewNodeInterfaceMock(t)
	node.On("GetID").Return("credentials_auth")
	node.On("GetType").Return(common.NodeTypeTaskExecution)

	// The context has no subject yet, exactly as during the authenticating node's own completion.
	ctx := &EngineContext{
		ExecutionID:      "flow-exec-4",
		FlowType:         providers.FlowTypeAuthentication,
		AppID:            "app-4",
		ExecutionHistory: make(map[string]*providers.NodeExecutionRecord),
	}
	recordNodeExecution(ctx, node, &common.NodeResponse{Status: common.NodeStatusComplete}, nil, 0, 1)

	nodeResp := &common.NodeResponse{Status: common.NodeStatusComplete}
	nodeResp.AuthUser.SetStateFor("credentials", providers.AuthState{
		EntityReference: &providers.EntityReference{
			EntityID:       "user-777",
			EntityCategory: string(providers.EntityCategoryUser),
		},
	})

	publishNodeExecutionCompletedEvent(ctx, node, nodeResp, nil, 0, 1, mockObs)

	assertEventData(t, (*captured)[0], map[string]interface{}{
		event.DataKey.NodeID:      "credentials_auth",
		event.DataKey.Subject:     "user-777",
		event.DataKey.SubjectType: event.PrincipalTypeUser,
	})
}

func TestFlowCompletedEvent_ReportsPrincipalAndCorrelation(t *testing.T) {
	mockObs, captured := setupCapturingObservability(t)
	defer config.ResetServerRuntime()

	publishFlowCompletedEvent(agentFlowContext(), 0, 5, mockObs)

	assertEventData(t, (*captured)[0], map[string]interface{}{
		event.DataKey.ActorType:     string(providers.EntityCategoryAgent),
		event.DataKey.CorrelationID: "flow-exec-1",
		event.DataKey.Subject:       "user-123",
	})
}

func TestFlowFailedEvent_SharesTheFlowTrace(t *testing.T) {
	mockObs, captured := setupCapturingObservability(t)
	defer config.ResetServerRuntime()

	publishFlowFailedEvent(agentFlowContext(), nil, 0, 5, mockObs)

	evt := (*captured)[0]
	if evt.TraceID != "flow-exec-1" {
		t.Errorf("TraceID = %q, want the execution id %q", evt.TraceID, "flow-exec-1")
	}
	assertEventData(t, evt, map[string]interface{}{
		event.DataKey.ActorType:     string(providers.EntityCategoryAgent),
		event.DataKey.CorrelationID: "flow-exec-1",
	})
}

// assertEventData checks that the event carries each expected key with the expected value.
func assertEventData(t *testing.T, evt *providers.Event, want map[string]interface{}) {
	t.Helper()

	for key, wantValue := range want {
		got, ok := evt.Data[key]
		if !ok {
			t.Errorf("event data is missing key %q", key)
			continue
		}
		if got != wantValue {
			t.Errorf("event data %q = %v, want %v", key, got, wantValue)
		}
	}
}
