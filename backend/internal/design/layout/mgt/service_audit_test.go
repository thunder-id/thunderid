// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package layoutmgt

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

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
)

const (
	testAuditAdminID     = "admin-1"
	testAuditLayoutID    = "019a0000-0000-7000-8000-0000000000a1"
	testAuditHandle      = "audit-brand-handle"
	testAuditDisplayName = "Audit Brand Display Name"
	testAuditDescription = "Audit brand description"
	testAuditColor       = "#a1b2c3"
)

// publishedEvent is one audit event the layout service published.
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
	require.NoError(t, audit.Subscribe(registry, "layout-service-test", sink))
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
		security.NewSecurityContextForTest(testAuditAdminID, "default-ou", "", nil, nil))
}

// auditedLayoutContent is the content of the layout every audit test sends, so a test can check none of
// it is recorded.
func auditedLayoutContent() json.RawMessage {
	return json.RawMessage(`{"colors": {"primary": "` + testAuditColor + `"}}`)
}

type LayoutServiceAuditTestSuite struct {
	suite.Suite
	store     *layoutMgtStoreInterfaceMock
	service   LayoutMgtServiceInterface
	published func() []publishedEvent
}

func TestLayoutServiceAuditTestSuite(t *testing.T) {
	suite.Run(t, new(LayoutServiceAuditTestSuite))
}

func (suite *LayoutServiceAuditTestSuite) SetupTest() {
	config.ResetServerRuntime()
	suite.Require().NoError(config.InitializeServerRuntime("/tmp/test", &config.Config{}))
	suite.resetService()
}

func (suite *LayoutServiceAuditTestSuite) SetupSubTest() {
	suite.resetService()
}

func (suite *LayoutServiceAuditTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (suite *LayoutServiceAuditTestSuite) resetService() {
	bus, published := newRecordingBus(suite.T())
	suite.store = newLayoutMgtStoreInterfaceMock(suite.T())
	suite.service = newLayoutMgtService(suite.store, bus)
	suite.published = published
}

// requireOnlyEvent checks that exactly one event was published, named name, for an attempt by the admin
// on the layout with id, and that it records nothing of the layout but its ID. An attempt that succeeded
// has the layout as subject too; a failed one has no subject.
func (suite *LayoutServiceAuditTestSuite) requireOnlyEvent(name string, action audit.Action, id string,
	outcome audit.Outcome) {
	suite.T().Helper()
	events := suite.published()
	suite.Require().Len(events, 1)
	event := events[0]
	layout := audit.Ref{Type: audit.TargetTypeLayout, ID: id}
	suite.Equal(name, event.name)
	suite.Equal(layout, event.payload.Target())
	suite.Equal(action, event.payload.Action())
	suite.Equal(outcome, event.payload.Outcome())
	suite.Equal(audit.Actor{Type: audit.ActorTypeUser, ID: testAuditAdminID}, event.payload.Actor())
	suite.Empty(event.payload.Details(), "no attribute of the layout is recorded")
	if outcome.Result == audit.ResultFailed || id == "" {
		suite.Equal(audit.Ref{}, event.payload.Subject(), "a failed attempt may name no layout")
	} else {
		suite.Equal(layout, event.payload.Subject())
	}
	recorded := fmt.Sprintf("%+v", event.payload)
	for _, value := range []string{testAuditHandle, testAuditDisplayName, testAuditDescription, testAuditColor} {
		suite.NotContains(recorded, value, "no handle, name, description or content of a layout is recorded")
	}
}

func (suite *LayoutServiceAuditTestSuite) TestCreateLayout_PublishesOneEventPerAttempt() {
	request := func() CreateLayoutRequestWithID {
		return CreateLayoutRequestWithID{ID: testAuditLayoutID, Handle: testAuditHandle,
			DisplayName: testAuditDisplayName, Description: testAuditDescription, Layout: auditedLayoutContent()}
	}
	cases := []struct {
		name    string
		request func() CreateLayoutRequestWithID
		arrange func()
		id      string
		outcome audit.Outcome
	}{
		{
			name:    "the layout is created",
			request: request,
			arrange: func() {
				suite.store.On("IsLayoutHandleConflict", mock.Anything, testAuditHandle, "").Return(false, nil)
				suite.store.On("CreateLayout", mock.Anything, testAuditLayoutID, mock.Anything).Return(nil)
			},
			id:      testAuditLayoutID,
			outcome: audit.OutcomeSucceeded,
		},
		{
			name: "the request has no display name",
			request: func() CreateLayoutRequestWithID {
				r := request()
				r.DisplayName = ""
				return r
			},
			arrange: func() {},
			outcome: audit.OutcomeFailed("lay_1005"),
		},
		{
			name:    "the handle is taken",
			request: request,
			arrange: func() {
				suite.store.On("IsLayoutHandleConflict", mock.Anything, testAuditHandle, "").Return(true, nil)
			},
			outcome: audit.OutcomeFailed("lay_1016"),
		},
		{
			name:    "the store fails",
			request: request,
			arrange: func() {
				suite.store.On("IsLayoutHandleConflict", mock.Anything, testAuditHandle, "").Return(false, nil)
				suite.store.On("CreateLayout", mock.Anything, testAuditLayoutID, mock.Anything).
					Return(errors.New("store failure"))
			},
			outcome: audit.OutcomeFailed("sse_5000"),
		},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			tc.arrange()

			created, svcErr := suite.service.CreateLayout(adminContext(), tc.request())

			suite.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			if svcErr == nil {
				suite.Equal(testAuditLayoutID, created.ID)
			}
			suite.requireOnlyEvent(audit.EventLayoutCreate, audit.ActionCreate, tc.id, tc.outcome)
		})
	}
}

func (suite *LayoutServiceAuditTestSuite) TestUpdateLayout_PublishesOneEventPerAttempt() {
	stored := Layout{ID: testAuditLayoutID, Handle: testAuditHandle, DisplayName: testAuditDisplayName,
		Layout: auditedLayoutContent()}
	request := func() UpdateLayoutRequest {
		return UpdateLayoutRequest{DisplayName: testAuditDisplayName, Description: testAuditDescription,
			Layout: auditedLayoutContent()}
	}
	cases := []struct {
		name    string
		request func() UpdateLayoutRequest
		arrange func()
		outcome audit.Outcome
	}{
		{
			name:    "the layout is updated",
			request: request,
			arrange: func() {
				suite.store.On("IsLayoutDeclarative", mock.Anything, testAuditLayoutID).Return(false)
				suite.store.On("GetLayout", mock.Anything, testAuditLayoutID).Return(stored, nil)
				suite.store.On("UpdateLayout", mock.Anything, testAuditLayoutID, mock.Anything).Return(nil)
			},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name:    "the layout does not exist",
			request: request,
			arrange: func() {
				suite.store.On("IsLayoutDeclarative", mock.Anything, testAuditLayoutID).Return(false)
				suite.store.On("GetLayout", mock.Anything, testAuditLayoutID).Return(Layout{}, errLayoutNotFound)
			},
			outcome: audit.OutcomeFailed("lay_1003"),
		},
		{
			name:    "the layout is declarative",
			request: request,
			arrange: func() {
				suite.store.On("IsLayoutDeclarative", mock.Anything, testAuditLayoutID).Return(true)
			},
			outcome: audit.OutcomeFailed("lay_1015"),
		},
		{
			name: "the request changes the handle",
			request: func() UpdateLayoutRequest {
				r := request()
				r.Handle = "another-handle"
				return r
			},
			arrange: func() {
				suite.store.On("IsLayoutDeclarative", mock.Anything, testAuditLayoutID).Return(false)
				suite.store.On("GetLayout", mock.Anything, testAuditLayoutID).Return(stored, nil)
			},
			outcome: audit.OutcomeFailed("lay_1018"),
		},
		{
			name:    "the store fails",
			request: request,
			arrange: func() {
				suite.store.On("IsLayoutDeclarative", mock.Anything, testAuditLayoutID).Return(false)
				suite.store.On("GetLayout", mock.Anything, testAuditLayoutID).Return(stored, nil)
				suite.store.On("UpdateLayout", mock.Anything, testAuditLayoutID, mock.Anything).
					Return(errors.New("store failure"))
			},
			outcome: audit.OutcomeFailed("sse_5000"),
		},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			tc.arrange()

			_, svcErr := suite.service.UpdateLayout(adminContext(), testAuditLayoutID, tc.request())

			suite.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			suite.requireOnlyEvent(audit.EventLayoutUpdate, audit.ActionUpdate, testAuditLayoutID, tc.outcome)
		})
	}
}

func (suite *LayoutServiceAuditTestSuite) TestDeleteLayout_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		arrange func()
		outcome audit.Outcome
	}{
		{
			name: "the layout is deleted",
			arrange: func() {
				suite.store.On("IsLayoutDeclarative", mock.Anything, testAuditLayoutID).Return(false)
				suite.store.On("IsLayoutExist", mock.Anything, testAuditLayoutID).Return(true, nil)
				suite.store.On("DeleteLayout", mock.Anything, testAuditLayoutID).Return(nil)
			},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name: "the layout is declarative",
			arrange: func() {
				suite.store.On("IsLayoutDeclarative", mock.Anything, testAuditLayoutID).Return(true)
			},
			outcome: audit.OutcomeFailed("lay_1015"),
		},
		{
			name: "the layout cannot be looked up",
			arrange: func() {
				suite.store.On("IsLayoutDeclarative", mock.Anything, testAuditLayoutID).Return(false)
				suite.store.On("IsLayoutExist", mock.Anything, testAuditLayoutID).
					Return(false, errors.New("store failure"))
			},
			outcome: audit.OutcomeFailed("sse_5000"),
		},
		{
			name: "the store fails",
			arrange: func() {
				suite.store.On("IsLayoutDeclarative", mock.Anything, testAuditLayoutID).Return(false)
				suite.store.On("IsLayoutExist", mock.Anything, testAuditLayoutID).Return(true, nil)
				suite.store.On("DeleteLayout", mock.Anything, testAuditLayoutID).Return(errors.New("store failure"))
			},
			outcome: audit.OutcomeFailed("sse_5000"),
		},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			tc.arrange()

			svcErr := suite.service.DeleteLayout(adminContext(), testAuditLayoutID)

			suite.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			suite.requireOnlyEvent(audit.EventLayoutDelete, audit.ActionDelete, testAuditLayoutID, tc.outcome)
		})
	}
}

// A delete stays idempotent, but one that finds no layout records nothing, so a caller cannot write a
// succeeded delete for any ID it chooses. The database store answers a missing ID with a count of zero,
// not an error, as the mock does here.
func (suite *LayoutServiceAuditTestSuite) TestDeleteLayout_PublishesNothingForAMissingLayout() {
	suite.store.On("IsLayoutDeclarative", mock.Anything, testAuditLayoutID).Return(false)
	suite.store.On("IsLayoutExist", mock.Anything, testAuditLayoutID).Return(false, nil)

	svcErr := suite.service.DeleteLayout(adminContext(), testAuditLayoutID)

	suite.Nil(svcErr)
	suite.Empty(suite.published())
	suite.store.AssertNotCalled(suite.T(), "DeleteLayout", mock.Anything, mock.Anything)
}

// A bus that cannot take the event never fails the change it describes.
func (suite *LayoutServiceAuditTestSuite) TestCreateLayout_SucceedsWhenTheBusRefusesTheEvent() {
	failing := eventbusmock.NewEventBusInterfaceMock(suite.T())
	failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("audit queue is full")).Once()
	suite.service = newLayoutMgtService(suite.store, failing)
	suite.store.On("IsLayoutHandleConflict", mock.Anything, testAuditHandle, "").Return(false, nil)
	suite.store.On("CreateLayout", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	created, svcErr := suite.service.CreateLayout(adminContext(), CreateLayoutRequestWithID{
		Handle: testAuditHandle, DisplayName: testAuditDisplayName, Layout: auditedLayoutContent()})

	suite.Nil(svcErr)
	suite.NotEmpty(created.ID)
}
