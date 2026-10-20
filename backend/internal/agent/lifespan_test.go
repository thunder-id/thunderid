// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/agent/model"
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/observability/observabilitymock"
)

const (
	testFutureValidUntil = "2099-01-01T00:00:00Z"
	testPastValidUntil   = "2020-01-01T00:00:00Z"
)

// withLifespanEvents attaches an enabled observability mock to svc and returns the event types it
// receives, in order.
func (suite *AgentServiceTestSuite) withLifespanEvents(svc *agentService) *[]string {
	obs := observabilitymock.NewObservabilityServiceInterfaceMock(suite.T())
	obs.On("IsEnabled").Return(true).Maybe()
	published := []string{}
	obs.On("PublishEvent", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		published = append(published, args.Get(1).(*providers.Event).Type)
	}).Maybe()
	svc.observabilitySvc = obs
	return &published
}

// agentEntityWithValidUntil returns the agent fixture with the given stored lifespan end.
func agentEntityWithValidUntil(validUntil string) *providers.Entity {
	e := buildAgentEntityFixture("old-name", "", "", "")
	e.SystemAttributes, _ = json.Marshal(map[string]interface{}{
		authnprovidercm.SystemAttrValidUntil: validUntil,
	})
	return e
}

// stubAgentEntity serves e as the stored agent. Any other entity, such as the caller whose category a
// lifespan event reports, is not found.
func stubAgentEntity(mockEntity *entitymock.EntityServiceInterfaceMock, e *providers.Entity) {
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(e, nil)
	mockEntity.On("GetEntity", mock.Anything, mock.Anything).
		Maybe().Return((*providers.Entity)(nil), entity.ErrEntityNotFound)
}

// callerContext returns a context whose token names subject, with the given act claim when set.
func callerContext(subject string, act map[string]interface{}) context.Context {
	attrs := map[string]interface{}{}
	if act != nil {
		attrs[claimAct] = act
	}
	return security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest(subject, "", "", nil, attrs))
}

func (suite *AgentServiceTestSuite) TestResolveRequestedValidUntil() {
	now := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	str := func(s string) *string { return &s }
	cases := []struct {
		name    string
		value   *string
		want    string
		wantErr string
	}{
		{name: "absent is unbounded", value: nil, want: authnprovidercm.UnboundedValidUntil},
		{name: "future instant", value: str("2027-03-31T00:00:00Z"), want: "2027-03-31T00:00:00Z"},
		{name: "offset normalized to UTC", value: str("2027-03-31T05:30:00+05:30"), want: "2027-03-31T00:00:00Z"},
		{name: "sub-second truncated", value: str("2027-03-31T00:00:00.9Z"), want: "2027-03-31T00:00:00Z"},
		{name: "sentinel is unbounded", value: str(authnprovidercm.UnboundedValidUntil),
			want: authnprovidercm.UnboundedValidUntil},
		{name: "past the sentinel in UTC is unbounded", value: str("9999-12-31T23:00:00-05:00"),
			want: authnprovidercm.UnboundedValidUntil},
		{name: "date only", value: str("2027-03-31"), wantErr: ErrorInvalidLifespan.Code},
		{name: "malformed", value: str("tomorrow"), wantErr: ErrorInvalidLifespan.Code},
		{name: "now", value: str("2026-10-14T00:00:00Z"), wantErr: ErrorLifespanNotInFuture.Code},
		{name: "past", value: str(testPastValidUntil), wantErr: ErrorLifespanNotInFuture.Code},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			got, svcErr := resolveRequestedValidUntil(tc.value, now, false)
			if tc.wantErr != "" {
				suite.Require().NotNil(svcErr)
				suite.Equal(tc.wantErr, svcErr.Code)
				return
			}
			suite.Require().Nil(svcErr)
			suite.Equal(tc.want, got.String())
		})
	}
}

func (suite *AgentServiceTestSuite) TestParseUpdateValidUntil() {
	cases := []struct {
		name          string
		raw           json.RawMessage
		want          string
		wantRequested bool
		wantErr       bool
	}{
		{name: "absent", raw: nil},
		{name: "null", raw: json.RawMessage(`null`), want: authnprovidercm.UnboundedValidUntil, wantRequested: true},
		{name: "timestamp", raw: json.RawMessage(`"2027-03-31T00:00:00Z"`), want: "2027-03-31T00:00:00Z",
			wantRequested: true},
		{name: "past is parsed, not refused", raw: json.RawMessage(`"` + testPastValidUntil + `"`),
			want: testPastValidUntil, wantRequested: true},
		{name: "number", raw: json.RawMessage(`12`), wantRequested: true, wantErr: true},
		{name: "malformed", raw: json.RawMessage(`"not-a-date"`), wantRequested: true, wantErr: true},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			got, requested, svcErr := parseUpdateValidUntil(tc.raw)
			suite.Equal(tc.wantRequested, requested)
			if tc.wantErr {
				suite.Require().NotNil(svcErr)
				suite.Equal(ErrorInvalidLifespan.Code, svcErr.Code)
				return
			}
			suite.Require().Nil(svcErr)
			if tc.wantRequested {
				suite.Equal(tc.want, got.String())
			}
		})
	}
}

func (suite *AgentServiceTestSuite) TestResolveDeclarativeValidUntil() {
	got, err := resolveDeclarativeValidUntil("")
	suite.NoError(err)
	suite.Empty(got)

	got, err = resolveDeclarativeValidUntil(testPastValidUntil)
	suite.NoError(err)
	suite.Equal(testPastValidUntil, got)

	_, err = resolveDeclarativeValidUntil("2027-03-31")
	suite.ErrorIs(err, errInvalidDeclarativeLifespan)

	_, err = resolveDeclarativeValidUntil("0000-01-01T00:00:00+01:00")
	suite.ErrorIs(err, errInvalidDeclarativeLifespan, "an instant before year 0 in UTC cannot be stored")
}

func (suite *AgentServiceTestSuite) TestLifespanView() {
	now := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	end := func(stored string) *lifespanEnd {
		e, svcErr := parseLifespanEnd(stored)
		suite.Require().Nil(svcErr)
		return &e
	}

	validUntil, lifespan := lifespanView(&lifespanEnd{}, now)
	suite.Empty(validUntil)
	suite.Equal(model.LifespanPhaseUnbounded, lifespan.Phase)

	validUntil, lifespan = lifespanView(end(testFutureValidUntil), now)
	suite.Equal(testFutureValidUntil, validUntil)
	suite.Equal(model.LifespanPhaseActive, lifespan.Phase)

	validUntil, lifespan = lifespanView(end("2026-10-14T00:00:00Z"), now)
	suite.Equal("2026-10-14T00:00:00Z", validUntil)
	suite.Equal(model.LifespanPhaseExpired, lifespan.Phase, "the agent is expired from its validUntil instant")

	validUntil, lifespan = lifespanView(nil, now)
	suite.Empty(validUntil)
	suite.Nil(lifespan, "an unreadable stored value reports no lifespan")
}

func (suite *AgentServiceTestSuite) TestLifespanEnd_EndsAfter() {
	earlier, _ := parseLifespanEnd("2098-01-01T00:00:00Z")
	later, _ := parseLifespanEnd(testFutureValidUntil)
	unbounded := lifespanEnd{}

	suite.True(later.endsAfter(earlier))
	suite.False(earlier.endsAfter(later))
	suite.False(later.endsAfter(later))
	suite.True(unbounded.endsAfter(later), "a lifespan that never ends ends after any other")
	suite.False(later.endsAfter(unbounded))
	suite.False(unbounded.endsAfter(unbounded))
}

func (suite *AgentServiceTestSuite) TestStoredLifespanEnd() {
	suite.Equal(authnprovidercm.UnboundedValidUntil, storedLifespanEnd(nil).String())
	suite.Equal(authnprovidercm.UnboundedValidUntil, storedLifespanEnd(json.RawMessage(`{"owner":"o"}`)).String(),
		"an agent registered before lifespans existed is unbounded")
	suite.Equal(testPastValidUntil,
		storedLifespanEnd(agentEntityWithValidUntil(testPastValidUntil).SystemAttributes).String())
	suite.Nil(storedLifespanEnd(agentEntityWithValidUntil("corrupt").SystemAttributes))
}

func (suite *AgentServiceTestSuite) TestIsCallerAgent() {
	suite.True(isCallerAgent(callerContext(testAgentID, nil), testAgentID))
	suite.False(isCallerAgent(callerContext("owner-1", nil), testAgentID))
	suite.True(isCallerAgent(callerContext("owner-1", map[string]interface{}{"sub": testAgentID}), testAgentID),
		"the agent as the actor of a delegated token")
	suite.True(isCallerAgent(callerContext("owner-1", map[string]interface{}{
		"sub": "other-agent", claimAct: map[string]interface{}{"sub": testAgentID},
	}), testAgentID), "the agent as a prior actor in the delegation chain")
	suite.False(isCallerAgent(callerContext("owner-1", map[string]interface{}{"sub": "other-agent"}), testAgentID))
}

func (suite *AgentServiceTestSuite) TestCreateAgent_StoresLifespanAndPublishesSet() {
	svc, mockEntity, _, _, _ := suite.setupService()
	published := suite.withLifespanEvents(svc)

	var stored json.RawMessage
	clearMockCalls(mockEntity, "CreateEntity")
	mockEntity.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			stored = args.Get(1).(*providers.Entity).SystemAttributes
		}).Return(&providers.Entity{ID: testAgentID}, nil)

	validUntil := testFutureValidUntil
	resp, svcErr := svc.CreateAgent(context.Background(), &providers.Agent{
		OUID: testOUID, Type: testAgentType, ValidUntil: &validUntil,
	})
	suite.Require().Nil(svcErr)
	suite.Equal(testFutureValidUntil, resp.ValidUntil)
	suite.Equal(model.LifespanPhaseActive, resp.Lifespan.Phase)
	suite.Equal(testFutureValidUntil, storedLifespanEnd(stored).String())
	suite.Equal([]string{string(event.EventTypeAgentLifespanSet)}, *published)
}

func (suite *AgentServiceTestSuite) TestCreateAgent_WithoutLifespanStoresSentinel() {
	svc, mockEntity, _, _, _ := suite.setupService()

	var stored json.RawMessage
	clearMockCalls(mockEntity, "CreateEntity")
	mockEntity.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			stored = args.Get(1).(*providers.Entity).SystemAttributes
		}).Return(&providers.Entity{ID: testAgentID}, nil)

	resp, svcErr := svc.CreateAgent(context.Background(), &providers.Agent{OUID: testOUID, Type: testAgentType})
	suite.Require().Nil(svcErr)
	suite.Empty(resp.ValidUntil)
	suite.Equal(model.LifespanPhaseUnbounded, resp.Lifespan.Phase)
	var attrs map[string]interface{}
	suite.Require().NoError(json.Unmarshal(stored, &attrs))
	suite.Equal(authnprovidercm.UnboundedValidUntil, attrs[authnprovidercm.SystemAttrValidUntil])
}

// An import restores an exported expired agent as it was, rather than refusing its past validUntil.
func (suite *AgentServiceTestSuite) TestCreateAgent_RestoreAcceptsPastLifespan() {
	svc, mockEntity, _, _, _ := suite.setupService()
	var stored json.RawMessage
	clearMockCalls(mockEntity, "CreateEntity")
	mockEntity.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			stored = args.Get(1).(*providers.Entity).SystemAttributes
		}).Return(&providers.Entity{ID: testAgentID}, nil)

	validUntil := testPastValidUntil
	resp, svcErr := svc.CreateAgent(WithLifespanRestore(context.Background()), &providers.Agent{
		OUID: testOUID, Type: testAgentType, ValidUntil: &validUntil,
	})
	suite.Require().Nil(svcErr)
	suite.Equal(model.LifespanPhaseExpired, resp.Lifespan.Phase)
	suite.Equal(testPastValidUntil, storedLifespanEnd(stored).String())
}

// An import upserting an exported expired agent restores its past validUntil.
func (suite *AgentServiceTestSuite) TestUpdateAgent_RestoreAcceptsPastLifespan() {
	svc, mockEntity, _ := suite.setupLifespanUpdate(testFutureValidUntil)
	mockEntity.On("SetValidUntil", mock.Anything, testAgentID, testPastValidUntil).Return(nil).Once()
	mockEntity.On("InvalidateEntityCache", mock.Anything, testAgentID).Return().Once()

	resp, svcErr := svc.UpdateAgent(WithLifespanRestore(callerContext("admin-1", nil)), testAgentID,
		&model.UpdateAgentRequest{Type: testAgentType, ValidUntil: json.RawMessage(`"` + testPastValidUntil + `"`)})
	suite.Require().Nil(svcErr)
	suite.Equal(model.LifespanPhaseExpired, resp.Lifespan.Phase)
}

func (suite *AgentServiceTestSuite) TestValidateLifespanFormat() {
	suite.Nil(ValidateLifespanFormat(testPastValidUntil), "a past value is well formed")
	suite.Nil(ValidateLifespanFormat(authnprovidercm.UnboundedValidUntil))
	suite.Require().NotNil(ValidateLifespanFormat("2027-03-31"))
	suite.Equal(ErrorInvalidLifespan.Code, ValidateLifespanFormat("2027-03-31").Code)
}

func (suite *AgentServiceTestSuite) TestCreateAgent_PastLifespanRefused() {
	svc, mockEntity, _, _, _ := suite.setupService()

	validUntil := testPastValidUntil
	resp, svcErr := svc.CreateAgent(context.Background(), &providers.Agent{
		OUID: testOUID, Type: testAgentType, ValidUntil: &validUntil,
	})
	suite.Nil(resp)
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorLifespanNotInFuture.Code, svcErr.Code)
	mockEntity.AssertNotCalled(suite.T(), "CreateEntity", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// setupLifespanUpdate returns a service whose stored agent has the given lifespan end.
func (suite *AgentServiceTestSuite) setupLifespanUpdate(stored string) (
	*agentService, *entitymock.EntityServiceInterfaceMock, *[]string) {
	svc, mockEntity, _, _, _ := suite.setupService()
	stubAgentEntity(mockEntity, agentEntityWithValidUntil(stored))
	return svc, mockEntity, suite.withLifespanEvents(svc)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_LifespanAbsentLeavesItUnchanged() {
	svc, mockEntity, published := suite.setupLifespanUpdate(testFutureValidUntil)

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{Type: testAgentType})
	suite.Require().Nil(svcErr)
	suite.Equal(testFutureValidUntil, resp.ValidUntil)
	mockEntity.AssertNotCalled(suite.T(), "SetValidUntil", mock.Anything, mock.Anything, mock.Anything)
	suite.Empty(*published)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_LifespanEqualIsNoChange() {
	// Resending an expired agent's lifespan unchanged is not refused as a past value.
	svc, mockEntity, published := suite.setupLifespanUpdate(testPastValidUntil)

	resp, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Type: testAgentType, ValidUntil: json.RawMessage(`"` + testPastValidUntil + `"`),
	})
	suite.Require().Nil(svcErr)
	suite.Equal(model.LifespanPhaseExpired, resp.Lifespan.Phase)
	mockEntity.AssertNotCalled(suite.T(), "SetValidUntil", mock.Anything, mock.Anything, mock.Anything)
	suite.Empty(*published)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_LifespanChanges() {
	cases := []struct {
		name       string
		stored     string
		requested  string
		wantStored string
		wantEvents []string
	}{
		{name: "extension", stored: "2098-01-01T00:00:00Z", requested: `"2099-01-01T00:00:00Z"`,
			wantStored: "2099-01-01T00:00:00Z",
			wantEvents: []string{string(event.EventTypeAgentLifespanExtended)}},
		{name: "reduction", stored: "2099-01-01T00:00:00Z", requested: `"2098-01-01T00:00:00Z"`,
			wantStored: "2098-01-01T00:00:00Z",
			wantEvents: []string{string(event.EventTypeAgentLifespanReduced)}},
		{name: "bounded to unbounded", stored: testFutureValidUntil, requested: `null`,
			wantStored: authnprovidercm.UnboundedValidUntil,
			wantEvents: []string{string(event.EventTypeAgentLifespanExtended)}},
		{name: "unbounded to bounded", stored: authnprovidercm.UnboundedValidUntil,
			requested: `"` + testFutureValidUntil + `"`, wantStored: testFutureValidUntil,
			wantEvents: []string{string(event.EventTypeAgentLifespanReduced)}},
		{name: "reactivation", stored: testPastValidUntil, requested: `"` + testFutureValidUntil + `"`,
			wantStored: testFutureValidUntil,
			wantEvents: []string{
				string(event.EventTypeAgentLifespanExtended), string(event.EventTypeAgentReactivated),
			}},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			svc, mockEntity, _, _, _ := suite.setupService()
			stubAgentEntity(mockEntity, agentEntityWithValidUntil(tc.stored))
			mockEntity.On("SetValidUntil", mock.Anything, testAgentID, tc.wantStored).Return(nil).Once()
			// The cached agent is dropped again after the commit, so a read made before the commit
			// cannot keep the old lifespan end in the cache.
			mockEntity.On("InvalidateEntityCache", mock.Anything, testAgentID).Return().Once()
			published := suite.withLifespanEvents(svc)

			resp, svcErr := svc.UpdateAgent(callerContext("admin-1", nil), testAgentID, &model.UpdateAgentRequest{
				Type: testAgentType, ValidUntil: json.RawMessage(tc.requested),
			})
			suite.Require().Nil(svcErr)
			suite.NotNil(resp.Lifespan)
			suite.Equal(tc.wantEvents, *published)
		})
	}
}

// A stored value that cannot be read is replaced by any requested value, null included, and the change
// is published as the lifespan being set, since it is neither an extension nor a reduction.
func (suite *AgentServiceTestSuite) TestUpdateAgent_UnreadableStoredLifespanIsReplaced() {
	svc, mockEntity, published := suite.setupLifespanUpdate("corrupt")
	mockEntity.On("SetValidUntil", mock.Anything, testAgentID, authnprovidercm.UnboundedValidUntil).
		Return(nil).Once()
	mockEntity.On("InvalidateEntityCache", mock.Anything, testAgentID).Return().Once()

	resp, svcErr := svc.UpdateAgent(callerContext("admin-1", nil), testAgentID, &model.UpdateAgentRequest{
		Type: testAgentType, ValidUntil: json.RawMessage(`null`),
	})
	suite.Require().Nil(svcErr)
	suite.Equal(model.LifespanPhaseUnbounded, resp.Lifespan.Phase)
	suite.Equal([]string{string(event.EventTypeAgentLifespanSet)}, *published)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_LifespanRefused() {
	cases := []struct {
		name      string
		ctx       context.Context
		requested string
		wantErr   string
	}{
		{name: "agent changes itself", ctx: callerContext(testAgentID, nil),
			requested: `"` + testFutureValidUntil + `"`, wantErr: ErrorCannotChangeOwnLifespan.Code},
		{name: "agent changes itself as actor",
			ctx:       callerContext("owner-1", map[string]interface{}{"sub": testAgentID}),
			requested: `"` + testFutureValidUntil + `"`, wantErr: ErrorCannotChangeOwnLifespan.Code},
		{name: "past value", ctx: callerContext("admin-1", nil),
			requested: `"` + testPastValidUntil + `"`, wantErr: ErrorLifespanNotInFuture.Code},
		{name: "malformed value", ctx: callerContext("admin-1", nil),
			requested: `"2099-01-01"`, wantErr: ErrorInvalidLifespan.Code},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			svc, mockEntity, mockInbound, _, _ := suite.setupService()
			stubAgentEntity(mockEntity, agentEntityWithValidUntil("2098-01-01T00:00:00Z"))
			published := suite.withLifespanEvents(svc)

			resp, svcErr := svc.UpdateAgent(tc.ctx, testAgentID, &model.UpdateAgentRequest{
				Type: testAgentType, ValidUntil: json.RawMessage(tc.requested),
			})
			suite.Nil(resp)
			suite.Require().NotNil(svcErr)
			suite.Equal(tc.wantErr, svcErr.Code)
			suite.Equal([]string{string(event.EventTypeAgentLifespanUpdateFailed)}, *published)
			mockEntity.AssertNotCalled(suite.T(), "UpdateEntity", mock.Anything, mock.Anything, mock.Anything)
			mockEntity.AssertNotCalled(suite.T(), "SetValidUntil", mock.Anything, mock.Anything, mock.Anything)
			mockInbound.AssertNotCalled(suite.T(), "UpdateInboundClient",
				mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_LifespanOnDeclarativeAgentRefused() {
	svc, mockEntity, _, _, _ := suite.setupService()
	readOnly := agentEntityWithValidUntil(testFutureValidUntil)
	readOnly.IsReadOnly = true
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(readOnly, nil)
	published := suite.withLifespanEvents(svc)

	_, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Type: testAgentType, ValidUntil: json.RawMessage(`null`),
	})
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorCannotModifyDeclarativeResource.Code, svcErr.Code)
	suite.Equal([]string{string(event.EventTypeAgentLifespanUpdateFailed)}, *published)
}

func (suite *AgentServiceTestSuite) TestUpdateAgent_LifespanWriteFailure() {
	svc, mockEntity, _, _, _ := suite.setupService()
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).
		Return(agentEntityWithValidUntil(testFutureValidUntil), nil)
	mockEntity.On("SetValidUntil", mock.Anything, testAgentID, authnprovidercm.UnboundedValidUntil).
		Return(errors.New("db error")).Once()
	published := suite.withLifespanEvents(svc)

	_, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Type: testAgentType, ValidUntil: json.RawMessage(`null`),
	})
	suite.Require().NotNil(svcErr)
	mockEntity.AssertNotCalled(suite.T(), "InvalidateEntityCache", mock.Anything, mock.Anything)
	suite.Equal([]string{string(event.EventTypeAgentLifespanUpdateFailed)}, *published)
}

func (suite *AgentServiceTestSuite) TestGetAgent_ReportsLifespan() {
	svc, mockEntity, _, _, _ := suite.setupService()
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Return(agentEntityWithValidUntil(testPastValidUntil), nil)

	resp, svcErr := svc.GetAgent(context.Background(), testAgentID, false)
	suite.Require().Nil(svcErr)
	suite.Equal(testPastValidUntil, resp.ValidUntil)
	assert.Equal(suite.T(), model.LifespanPhaseExpired, resp.Lifespan.Phase)
}

func (suite *AgentServiceTestSuite) TestPublishLifespanEvent_CarriesValues() {
	svc, mockEntity, _, _, _ := suite.setupService()
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, "admin-1").
		Return(&providers.Entity{ID: "admin-1", Category: providers.EntityCategoryUser}, nil)
	obs := observabilitymock.NewObservabilityServiceInterfaceMock(suite.T())
	obs.On("IsEnabled").Return(true)
	var evt *providers.Event
	obs.On("PublishEvent", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		evt = args.Get(1).(*providers.Event)
	}).Once()
	svc.observabilitySvc = obs

	svc.publishLifespanUpdateFailed(callerContext("admin-1", nil), testAgentID,
		lifespanChange{previous: storedLifespanEnd(agentEntityWithValidUntil(testFutureValidUntil).SystemAttributes)},
		lifespanFailureNotAuthorized)

	suite.Require().NotNil(evt)
	suite.Equal(providers.StatusFailure, evt.Status)
	suite.Equal(testAgentID, evt.Data[event.DataKey.AgentID])
	suite.Equal("admin-1", evt.Data[event.DataKey.ActorSub])
	suite.Equal(event.PrincipalTypeUser, evt.Data[event.DataKey.ActorType])
	suite.Equal(testFutureValidUntil, evt.Data[event.DataKey.PreviousValidUntil])
	suite.Equal(authnprovidercm.UnboundedValidUntil, evt.Data[event.DataKey.ValidUntil])
	suite.Equal(lifespanFailureNotAuthorized, evt.Data[event.DataKey.FailureClass])
}

func (suite *AgentServiceTestSuite) TestPublishLifespanEvent_ObservabilityDisabled() {
	svc, _, _, _, _ := suite.setupService()
	obs := observabilitymock.NewObservabilityServiceInterfaceMock(suite.T())
	obs.On("IsEnabled").Return(false)
	svc.observabilitySvc = obs

	svc.publishLifespanSet(context.Background(), testAgentID, lifespanEnd{})
	obs.AssertNotCalled(suite.T(), "PublishEvent", mock.Anything, mock.Anything)
}
