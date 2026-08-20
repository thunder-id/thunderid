// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	sysContext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/observability/observabilitymock"
)

const (
	testAgentEntityID = "agent-entity-1"
	testAppEntityID   = "app-entity-1"
	testTraceID       = "trace-1"
)

type TokenEventsTestSuite struct {
	suite.Suite
	mockObsSvc *observabilitymock.ObservabilityServiceInterfaceMock
	published  []*providers.Event
}

func TestTokenEventsSuite(t *testing.T) {
	suite.Run(t, new(TokenEventsTestSuite))
}

func (suite *TokenEventsTestSuite) SetupTest() {
	suite.published = nil
	suite.mockObsSvc = observabilitymock.NewObservabilityServiceInterfaceMock(suite.T())
	suite.mockObsSvc.On("IsEnabled").Return(true).Maybe()
	suite.mockObsSvc.
		On("PublishEvent", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			suite.published = append(suite.published, args.Get(1).(*providers.Event))
		}).
		Return().Maybe()
}

func (suite *TokenEventsTestSuite) newService() *tokenService {
	return &tokenService{observabilitySvc: suite.mockObsSvc}
}

func (suite *TokenEventsTestSuite) ctx() context.Context {
	return sysContext.WithTraceID(context.Background(), testTraceID)
}

// lastEvent returns the single event published by the call under test.
func (suite *TokenEventsTestSuite) lastEvent() *providers.Event {
	suite.Require().Len(suite.published, 1)
	return suite.published[0]
}

func agentClient() *providers.OAuthClient {
	return &providers.OAuthClient{
		ID:             testAgentEntityID,
		ClientID:       "agent-client-id",
		EntityCategory: providers.EntityCategoryAgent,
	}
}

func appClient() *providers.OAuthClient {
	return &providers.OAuthClient{
		ID:             testAppEntityID,
		ClientID:       "app-client-id",
		EntityCategory: providers.EntityCategoryApp,
	}
}

func (suite *TokenEventsTestSuite) TestIssuanceStarted_AgentReportsActorTypeAndEntityID() {
	suite.newService().publishTokenIssuanceStartedEvent(
		suite.ctx(), agentClient(), "agent-client-id", "client_credentials", "")

	evt := suite.lastEvent()
	assert.Equal(suite.T(), event.PrincipalTypeAgent, evt.Data[event.DataKey.ActorType])
	assert.Equal(suite.T(), testAgentEntityID, evt.Data[event.DataKey.EntityID])
	assert.Equal(suite.T(), testTraceID, evt.Data[event.DataKey.CorrelationID])
}

// The entity vocabulary spells an application "app" while the reported principal type spells it
// "application", matching the token's sub_type claim.
func (suite *TokenEventsTestSuite) TestIssuanceStarted_ApplicationReportsApplicationActorType() {
	suite.newService().publishTokenIssuanceStartedEvent(
		suite.ctx(), appClient(), "app-client-id", "authorization_code", "")

	assert.Equal(suite.T(), event.PrincipalTypeApplication, suite.lastEvent().Data[event.DataKey.ActorType])
}

func (suite *TokenEventsTestSuite) TestIssuanceStarted_UnresolvedClientOmitsPrincipalFields() {
	suite.newService().publishTokenIssuanceStartedEvent(suite.ctx(), nil, "", "", "")

	evt := suite.lastEvent()
	assert.NotContains(suite.T(), evt.Data, event.DataKey.ActorType)
	assert.NotContains(suite.T(), evt.Data, event.DataKey.EntityID)
}

func (suite *TokenEventsTestSuite) TestIssued_M2MReportsAgentSubjectAndNoDelegation() {
	app := agentClient()
	respDTO := &model.TokenResponseDTO{
		AccessToken: model.TokenDTO{
			SubjectID:       testAgentEntityID,
			SubjectCategory: string(providers.EntityCategoryAgent),
		},
	}

	suite.newService().publishTokenIssuedEvent(
		suite.ctx(), app, respDTO, app.ClientID, "client_credentials", "", 0)

	evt := suite.lastEvent()
	assert.Equal(suite.T(), event.PrincipalTypeAgent, evt.Data[event.DataKey.ActorType])
	assert.Equal(suite.T(), testAgentEntityID, evt.Data[event.DataKey.Subject])
	assert.Equal(suite.T(), event.PrincipalTypeAgent, evt.Data[event.DataKey.SubjectType])
	assert.Equal(suite.T(), false, evt.Data[event.DataKey.IsDelegated])
	assert.NotContains(suite.T(), evt.Data, event.DataKey.ActorSub)
}

func (suite *TokenEventsTestSuite) TestIssued_OBOReportsUserSubjectWithAgentActor() {
	app := agentClient()
	respDTO := &model.TokenResponseDTO{
		AccessToken: model.TokenDTO{
			SubjectID:       "user-1",
			SubjectCategory: string(providers.EntityCategoryUser),
			ActorSub:        testAgentEntityID,
		},
	}

	suite.newService().publishTokenIssuedEvent(
		suite.ctx(), app, respDTO, app.ClientID, "authorization_code", "", 0)

	evt := suite.lastEvent()
	assert.Equal(suite.T(), event.PrincipalTypeAgent, evt.Data[event.DataKey.ActorType])
	assert.Equal(suite.T(), "user-1", evt.Data[event.DataKey.Subject])
	assert.Equal(suite.T(), event.PrincipalTypeUser, evt.Data[event.DataKey.SubjectType])
	assert.Equal(suite.T(), testAgentEntityID, evt.Data[event.DataKey.ActorSub])
	assert.Equal(suite.T(), true, evt.Data[event.DataKey.IsDelegated])
}

func (suite *TokenEventsTestSuite) TestIssued_GrantCorrelationIDWinsOverTraceID() {
	app := agentClient()
	respDTO := &model.TokenResponseDTO{
		AccessToken:   model.TokenDTO{SubjectID: "user-1"},
		CorrelationID: "flow-execution-1",
	}

	suite.newService().publishTokenIssuedEvent(
		suite.ctx(), app, respDTO, app.ClientID, "authorization_code", "", 0)

	assert.Equal(suite.T(), "flow-execution-1", suite.lastEvent().Data[event.DataKey.CorrelationID])
}

func (suite *TokenEventsTestSuite) TestIssued_FallsBackToTraceIDWhenGrantHasNoCorrelationID() {
	app := agentClient()
	respDTO := &model.TokenResponseDTO{AccessToken: model.TokenDTO{SubjectID: testAgentEntityID}}

	suite.newService().publishTokenIssuedEvent(
		suite.ctx(), app, respDTO, app.ClientID, "client_credentials", "", 0)

	assert.Equal(suite.T(), testTraceID, suite.lastEvent().Data[event.DataKey.CorrelationID])
}

func (suite *TokenEventsTestSuite) TestIssuanceFailed_ReportsActorTypeWhenClientIsKnown() {
	publishTokenIssuanceFailedEvent(suite.mockObsSvc, suite.ctx(), agentClient(),
		"agent-client-id", "client_credentials", "", 400, "invalid_scope", 0)

	evt := suite.lastEvent()
	assert.Equal(suite.T(), event.PrincipalTypeAgent, evt.Data[event.DataKey.ActorType])
	assert.Equal(suite.T(), testAgentEntityID, evt.Data[event.DataKey.EntityID])
	assert.Equal(suite.T(), testTraceID, evt.Data[event.DataKey.CorrelationID])
}

// The subject's category is resolved while the token is built, so the event reports whatever the
// token carries rather than inferring it from the client.
func (suite *TokenEventsTestSuite) TestIssued_SubjectTypeIsReadFromTheToken() {
	app := agentClient()
	respDTO := &model.TokenResponseDTO{
		AccessToken: model.TokenDTO{
			SubjectID:       "agent-b",
			SubjectCategory: string(providers.EntityCategoryAgent),
		},
	}

	suite.newService().publishTokenIssuedEvent(
		suite.ctx(), app, respDTO, app.ClientID, "urn:ietf:params:oauth:grant-type:token-exchange", "", 0)

	evt := suite.lastEvent()
	assert.Equal(suite.T(), "agent-b", evt.Data[event.DataKey.Subject])
	assert.Equal(suite.T(), event.PrincipalTypeAgent, evt.Data[event.DataKey.SubjectType])
}

func (suite *TokenEventsTestSuite) TestIssued_SubjectTypeOmittedWhenTokenHasNoCategory() {
	app := agentClient()
	respDTO := &model.TokenResponseDTO{AccessToken: model.TokenDTO{SubjectID: "user-1"}}

	suite.newService().publishTokenIssuedEvent(
		suite.ctx(), app, respDTO, app.ClientID, "authorization_code", "", 0)

	evt := suite.lastEvent()
	assert.Equal(suite.T(), "user-1", evt.Data[event.DataKey.Subject])
	assert.NotContains(suite.T(), evt.Data, event.DataKey.SubjectType)
}
