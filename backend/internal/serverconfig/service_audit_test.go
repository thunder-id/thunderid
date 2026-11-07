// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package serverconfig

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

	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
)

const (
	testAuditAdminID = "admin-1"
	testAuditOUID    = "ou-1"
	// testAuditOrigin is part of the value written; no event may carry it.
	testAuditOrigin = "https://audit-secret-origin.example.com"
)

// publishedEvent is one audit event the server config service published.
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
	require.NoError(t, audit.Subscribe(registry, "server-config-service-test", sink))
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
		security.NewSecurityContextForTest(testAuditAdminID, testAuditOUID, "", nil, nil))
}

type ServerConfigServiceAuditTestSuite struct {
	suite.Suite
	mockStore   *serverConfigStoreInterfaceMock
	mockHandler *ServerConfigHandlerInterfaceMock
	value       json.RawMessage
}

func TestServerConfigServiceAuditTestSuite(t *testing.T) {
	suite.Run(t, new(ServerConfigServiceAuditTestSuite))
}

func (suite *ServerConfigServiceAuditTestSuite) SetupTest() {
	suite.mockStore = newServerConfigStoreInterfaceMock(suite.T())
	suite.mockHandler = NewServerConfigHandlerInterfaceMock(suite.T())
	suite.value = json.RawMessage(`{"allowedOrigins":["` + testAuditOrigin + `"]}`)
}

func (suite *ServerConfigServiceAuditTestSuite) SetupSubTest() {
	suite.SetupTest()
}

// service builds the service over the suite mocks, publishing on bus.
func (suite *ServerConfigServiceAuditTestSuite) service(bus eventbus.EventBusInterface) ServerConfigService {
	return newServerConfigService(suite.mockStore,
		map[ConfigName]ServerConfigHandlerInterface{ConfigNameCORS: suite.mockHandler}, nil, bus)
}

// expectWrite arranges a write of the suite value to the CORS section that reaches the store, which
// answers with storeErr.
func (suite *ServerConfigServiceAuditTestSuite) expectWrite(storeErr error) {
	suite.mockHandler.EXPECT().Decode(suite.value).Return("incoming", nil).Once()
	suite.mockStore.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).Return(storeLayers{}, nil).Once()
	suite.mockHandler.EXPECT().Decode(json.RawMessage(nil)).Return(nil, nil).Twice()
	suite.mockHandler.EXPECT().Validate(mock.Anything, "incoming", nil, nil).Return(nil).Once()
	suite.mockStore.EXPECT().UpsertServerConfig(mock.Anything, ServerConfig{Name: ConfigNameCORS, Value: suite.value}).
		Return(storeErr).Once()
}

// requireOnlyEvent checks that exactly one server_config.update was published, for an attempt by the
// admin on the section name, with outcome. A succeeded attempt has the section as subject too; a failed
// one has no subject. No part of the value written is recorded.
func (suite *ServerConfigServiceAuditTestSuite) requireOnlyEvent(events []publishedEvent, name string,
	outcome audit.Outcome) {
	suite.T().Helper()
	suite.Require().Len(events, 1)
	event := events[0]
	section := audit.Ref{Type: audit.TargetTypeServerConfig, ID: name}
	suite.Equal(audit.EventServerConfigUpdate, event.name)
	suite.Equal(section, event.payload.Target())
	suite.Equal(audit.ActionUpdate, event.payload.Action())
	suite.Equal(outcome, event.payload.Outcome())
	suite.Equal(audit.Actor{Type: audit.ActorTypeUser, ID: testAuditAdminID}, event.payload.Actor())
	suite.Empty(event.payload.Details(), "no part of the value is recorded")
	suite.NotContains(fmt.Sprintf("%+v", event.payload), testAuditOrigin, "the value is never recorded")
	if outcome.Result == audit.ResultFailed {
		suite.Equal(audit.Ref{}, event.payload.Subject())
	} else {
		suite.Equal(section, event.payload.Subject())
	}
}

func (suite *ServerConfigServiceAuditTestSuite) TestSetConfig_PublishesTheChangedSection() {
	bus, published := newRecordingBus(suite.T())
	suite.expectWrite(nil)

	svcErr := suite.service(bus).SetConfig(adminContext(), ConfigNameCORS, suite.value)

	suite.Require().Nil(svcErr)
	suite.requireOnlyEvent(published(), string(ConfigNameCORS), audit.OutcomeSucceeded)
}

func (suite *ServerConfigServiceAuditTestSuite) TestSetConfig_PublishesTheSameNameWhenItFails() {
	cases := []struct {
		name    string
		section ConfigName
		arrange func()
		outcome audit.Outcome
	}{
		{
			name:    "the section is not one the server has",
			section: ConfigName("no-such-section"),
			outcome: audit.OutcomeFailed("scf_1001"),
		},
		{
			name:    "the value does not decode",
			section: ConfigNameCORS,
			arrange: func() {
				suite.mockHandler.EXPECT().Decode(suite.value).Return(nil, errors.New("not a CORS value")).Once()
			},
			outcome: audit.OutcomeFailed("scf_1003"),
		},
		{
			name:    "the value is refused by the section",
			section: ConfigNameCORS,
			arrange: func() {
				suite.mockHandler.EXPECT().Decode(suite.value).Return("incoming", nil).Once()
				suite.mockStore.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).
					Return(storeLayers{}, nil).Once()
				suite.mockHandler.EXPECT().Decode(json.RawMessage(nil)).Return(nil, nil).Twice()
				suite.mockHandler.EXPECT().Validate(mock.Anything, "incoming", nil, nil).
					Return(errors.New("an origin is not allowed")).Once()
			},
			outcome: audit.OutcomeFailed("scf_1003"),
		},
		{
			name:    "the store fails",
			section: ConfigNameCORS,
			arrange: func() { suite.expectWrite(errors.New("db error")) },
			outcome: audit.OutcomeFailed("sse_5000"),
		},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			bus, published := newRecordingBus(suite.T())
			if tc.arrange != nil {
				tc.arrange()
			}

			svcErr := suite.service(bus).SetConfig(adminContext(), tc.section, suite.value)

			suite.Require().NotNil(svcErr)
			suite.requireOnlyEvent(published(), string(tc.section), tc.outcome)
		})
	}
}

// A bus that cannot take the event never fails the change it describes.
func (suite *ServerConfigServiceAuditTestSuite) TestSetConfig_SucceedsWhenTheBusRefusesTheEvent() {
	failing := eventbusmock.NewEventBusInterfaceMock(suite.T())
	failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("audit queue is full")).Once()
	suite.expectWrite(nil)

	svcErr := suite.service(failing).SetConfig(adminContext(), ConfigNameCORS, suite.value)

	suite.Nil(svcErr)
}

// An update worked out from the section's current value is a write to the section, recorded once however
// many times it reads the section again.
func (suite *ServerConfigServiceAuditTestSuite) TestUpdateConfig_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		store   serverConfigStoreInterface
		section ConfigName
		updater ConfigUpdater
		outcome audit.Outcome
	}{
		{
			name:    "the section is updated after another write",
			store:   &memStore{layers: map[ConfigName]json.RawMessage{}},
			section: ConfigNameCORS,
			updater: setKey{"origin", testAuditOrigin},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name:    "the section keeps changing",
			store:   &busyStore{memStore: memStore{layers: map[ConfigName]json.RawMessage{}}},
			section: ConfigNameCORS,
			updater: setKey{"origin", testAuditOrigin},
			outcome: audit.OutcomeFailed("scf_1005"),
		},
		{
			name:    "the update is refused",
			store:   &memStore{layers: map[ConfigName]json.RawMessage{}},
			section: ConfigNameCORS,
			updater: refuse{},
			outcome: audit.OutcomeFailed("scf_1003"),
		},
		{
			name:    "the section is not one the server has",
			store:   &memStore{layers: map[ConfigName]json.RawMessage{}},
			section: ConfigName("no-such-section"),
			updater: setKey{"origin", testAuditOrigin},
			outcome: audit.OutcomeFailed("scf_1001"),
		},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			if mem, ok := tc.store.(*memStore); ok && tc.outcome.Result == audit.ResultSucceeded {
				mem.beforeWrite = func() { mem.layers[ConfigNameCORS] = json.RawMessage(`{"other":"node"}`) }
			}
			bus, published := newRecordingBus(suite.T())
			service := newServerConfigService(tc.store,
				map[ConfigName]ServerConfigHandlerInterface{ConfigNameCORS: mapHandler{}}, nil, bus)

			svcErr := service.UpdateConfig(adminContext(), tc.section, tc.updater)

			suite.Equal(tc.outcome.Result == audit.ResultSucceeded, svcErr == nil)
			suite.requireOnlyEvent(published(), string(tc.section), tc.outcome)
		})
	}
}
