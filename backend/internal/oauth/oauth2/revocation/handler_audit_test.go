// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocation

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/clientauth"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
)

const (
	testAuditAppID   = "01900000-0000-7000-8000-000000000071"
	testAuditUserID  = "01900000-0000-7000-8000-000000000072"
	testAuditSecret  = "audit-revoke-secret-0123456789"
	testAuditRefresh = "eyJhbGciOiJSUzI1NiJ9.eyJ0eXAiOiJyZWZyZXNoIn0.c2lnbmF0dXJlLXJldm9rZQ"
)

// publishedEvent is one audit event the revocation handler published.
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
// an event the allowlist refuses is never recorded. The returned function stops the bus, which delivers
// every queued event, and returns what the sink received.
func newRecordingBus(t *testing.T) (eventbus.EventBusInterface, func() []publishedEvent) {
	bus, registry, stopper := eventbus.Initialize()
	t.Cleanup(stopper.Stop)

	sink := &recordingSink{}
	require.NoError(t, audit.Subscribe(registry, "revocation-test", sink))
	return bus, func() []publishedEvent {
		stopper.Stop()
		sink.mu.Lock()
		defer sink.mu.Unlock()
		return sink.events
	}
}

// discardingBus accepts every event and keeps none, for tests the audit event does not matter to.
type discardingBus struct{}

func (discardingBus) Publish(context.Context, *eventbus.Event) error {
	return nil
}

type RevocationHandlerAuditTestSuite struct {
	suite.Suite
	service *RevocationServiceInterfaceMock
}

func TestRevocationHandlerAuditTestSuite(t *testing.T) {
	suite.Run(t, new(RevocationHandlerAuditTestSuite))
}

func (suite *RevocationHandlerAuditTestSuite) SetupTest() {
	suite.service = NewRevocationServiceInterfaceMock(suite.T())
}

func (suite *RevocationHandlerAuditTestSuite) SetupSubTest() {
	suite.SetupTest()
}

// revokeRequest builds a revocation request carrying form, by a client of category authenticated with a
// secret, and with a verified bearer token of a user that must not decide the actor.
func revokeRequest(form url.Values, category providers.EntityCategory) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/oauth2/revoke", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx := security.WithSecurityContextTest(request.Context(),
		security.NewSecurityContextForTest(testAuditUserID, "", "", nil, nil))
	ctx = context.WithValue(ctx, clientauth.OAuthClientKey, &clientauth.OAuthClientInfo{
		ClientID: testClientID, ClientSecret: testAuditSecret,
		OAuthApp: &providers.OAuthClient{ID: testAuditAppID, ClientID: testClientID, EntityCategory: category},
	})
	return request.WithContext(ctx)
}

// Every exit of the handler is one token.revoke event by the client, named by its application or agent
// ID. A revoked, unknown or invalid token succeeds alike, so the line never tells whether a token
// existed; another client's token is denied. No event carries the token, a subject or a detail.
func (suite *RevocationHandlerAuditTestSuite) TestEveryExitPublishesOneEvent() {
	cases := []struct {
		name     string
		form     url.Values
		category providers.EntityCategory
		setup    func()
		status   int
		actor    audit.Actor
		outcome  audit.Outcome
	}{
		{
			name: "a revoked token", form: url.Values{"token": {testAuditRefresh}},
			category: providers.EntityCategoryApp,
			setup: func() {
				suite.service.EXPECT().RevokeToken(mock.Anything, testAuditRefresh, "", testClientID).
					Return(RevokeOutcomeRevoked, nil).Once()
			},
			status: http.StatusOK, actor: audit.Actor{Type: audit.ActorTypeApplication, ID: testAuditAppID},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name: "an unknown token, by an agent", form: url.Values{"token": {"not-a-token"}},
			category: providers.EntityCategoryAgent,
			setup: func() {
				suite.service.EXPECT().RevokeToken(mock.Anything, "not-a-token", "", testClientID).
					Return(RevokeOutcomeRevoked, nil).Once()
			},
			status: http.StatusOK, actor: audit.Actor{Type: audit.ActorTypeAgent, ID: testAuditAppID},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name: "another client's token", form: url.Values{"token": {testAuditRefresh}},
			category: providers.EntityCategoryApp,
			setup: func() {
				suite.service.EXPECT().RevokeToken(mock.Anything, testAuditRefresh, "", testClientID).
					Return(RevokeOutcomeNotOwned, nil).Once()
			},
			status: http.StatusBadRequest, actor: audit.Actor{Type: audit.ActorTypeApplication, ID: testAuditAppID},
			outcome: audit.OutcomeDenied(constants.ErrorInvalidGrant),
		},
		{
			name: "a store failure", form: url.Values{"token": {testAuditRefresh}},
			category: providers.EntityCategoryApp,
			setup: func() {
				suite.service.EXPECT().RevokeToken(mock.Anything, testAuditRefresh, "", testClientID).
					Return(RevokeOutcomeRevoked, errors.New("deny list unavailable")).Once()
			},
			status:  http.StatusInternalServerError,
			actor:   audit.Actor{Type: audit.ActorTypeApplication, ID: testAuditAppID},
			outcome: audit.OutcomeFailed(constants.ErrorServerError),
		},
		{
			name: "no token", form: url.Values{"token_type_hint": {"refresh_token"}},
			category: providers.EntityCategoryApp, setup: func() {},
			status: http.StatusBadRequest, actor: audit.Actor{Type: audit.ActorTypeApplication, ID: testAuditAppID},
			outcome: audit.OutcomeFailed(constants.ErrorInvalidRequest),
		},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			tc.setup()
			bus, published := newRecordingBus(suite.T())
			recorder := httptest.NewRecorder()

			newRevocationHandler(suite.service, bus).HandleRevoke(recorder, revokeRequest(tc.form, tc.category))

			suite.Require().Equal(tc.status, recorder.Code)
			events := published()
			suite.Require().Len(events, 1)
			event := events[0]
			suite.Equal(audit.EventTokenRevoke, event.name)
			suite.Equal(audit.ActionRevoke, event.payload.Action())
			suite.Equal(audit.Ref{Type: audit.TargetTypeToken}, event.payload.Target())
			suite.Equal(tc.actor, event.payload.Actor())
			suite.Equal(audit.Ref{}, event.payload.OnBehalfOf(), "a bearer token never decides the actor")
			suite.Equal(audit.Ref{}, event.payload.Subject())
			suite.Equal(tc.outcome, event.payload.Outcome())
			suite.Empty(event.payload.Details())
			recorded := fmt.Sprintf("%+v", event.payload)
			for _, value := range []string{testAuditRefresh, "not-a-token", testAuditSecret, testClientID} {
				suite.NotContains(recorded, value)
			}
		})
	}
}

// A bus that fails to publish does not change the answer to a revocation request.
func (suite *RevocationHandlerAuditTestSuite) TestAFailingBusDoesNotChangeTheAnswer() {
	suite.service.EXPECT().RevokeToken(mock.Anything, testAuditRefresh, "", testClientID).
		Return(RevokeOutcomeRevoked, nil).Once()
	bus := eventbusmock.NewEventBusInterfaceMock(suite.T())
	bus.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("bus closed")).Once()
	recorder := httptest.NewRecorder()

	newRevocationHandler(suite.service, bus).HandleRevoke(recorder,
		revokeRequest(url.Values{"token": {testAuditRefresh}}, providers.EntityCategoryApp))

	suite.Equal(http.StatusOK, recorder.Code)
}
