// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package thememgt

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
	testAuditThemeID     = "019a0000-0000-7000-8000-0000000000a1"
	testAuditHandle      = "audit-brand-handle"
	testAuditDisplayName = "Audit Brand Display Name"
	testAuditDescription = "Audit brand description"
	testAuditColor       = "#a1b2c3"
)

// publishedEvent is one audit event the theme service published.
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
	require.NoError(t, audit.Subscribe(registry, "theme-service-test", sink))
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

// auditedThemeContent is the content of the theme every audit test sends, so a test can check none of
// it is recorded.
func auditedThemeContent() json.RawMessage {
	return json.RawMessage(`{"colors": {"primary": "` + testAuditColor + `"}}`)
}

type ThemeServiceAuditTestSuite struct {
	suite.Suite
	store     *themeMgtStoreInterfaceMock
	service   ThemeMgtServiceInterface
	published func() []publishedEvent
}

func TestThemeServiceAuditTestSuite(t *testing.T) {
	suite.Run(t, new(ThemeServiceAuditTestSuite))
}

func (suite *ThemeServiceAuditTestSuite) SetupTest() {
	config.ResetServerRuntime()
	suite.Require().NoError(config.InitializeServerRuntime("/tmp/test", &config.Config{}))
	suite.resetService()
}

func (suite *ThemeServiceAuditTestSuite) SetupSubTest() {
	suite.resetService()
}

func (suite *ThemeServiceAuditTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (suite *ThemeServiceAuditTestSuite) resetService() {
	bus, published := newRecordingBus(suite.T())
	suite.store = newThemeMgtStoreInterfaceMock(suite.T())
	suite.service = newThemeMgtService(suite.store, bus)
	suite.published = published
}

// requireOnlyEvent checks that exactly one event was published, named name, for an attempt by the admin
// on the theme with id, and that it records nothing of the theme but its ID. An attempt that succeeded
// has the theme as subject too; a failed one has no subject.
func (suite *ThemeServiceAuditTestSuite) requireOnlyEvent(name string, action audit.Action, id string,
	outcome audit.Outcome) {
	suite.T().Helper()
	events := suite.published()
	suite.Require().Len(events, 1)
	event := events[0]
	theme := audit.Ref{Type: audit.TargetTypeTheme, ID: id}
	suite.Equal(name, event.name)
	suite.Equal(theme, event.payload.Target())
	suite.Equal(action, event.payload.Action())
	suite.Equal(outcome, event.payload.Outcome())
	suite.Equal(audit.Actor{Type: audit.ActorTypeUser, ID: testAuditAdminID}, event.payload.Actor())
	suite.Empty(event.payload.Details(), "no attribute of the theme is recorded")
	if outcome.Result == audit.ResultFailed || id == "" {
		suite.Equal(audit.Ref{}, event.payload.Subject(), "a failed attempt may name no theme")
	} else {
		suite.Equal(theme, event.payload.Subject())
	}
	recorded := fmt.Sprintf("%+v", event.payload)
	for _, value := range []string{testAuditHandle, testAuditDisplayName, testAuditDescription, testAuditColor} {
		suite.NotContains(recorded, value, "no handle, name, description or content of a theme is recorded")
	}
}

func (suite *ThemeServiceAuditTestSuite) TestCreateTheme_PublishesOneEventPerAttempt() {
	request := func() CreateThemeRequestWithID {
		return CreateThemeRequestWithID{ID: testAuditThemeID, Handle: testAuditHandle,
			DisplayName: testAuditDisplayName, Description: testAuditDescription, Theme: auditedThemeContent()}
	}
	cases := []struct {
		name    string
		request func() CreateThemeRequestWithID
		arrange func()
		id      string
		outcome audit.Outcome
	}{
		{
			name:    "the theme is created",
			request: request,
			arrange: func() {
				suite.store.On("IsThemeHandleConflict", mock.Anything, testAuditHandle, "").Return(false, nil)
				suite.store.On("CreateTheme", mock.Anything, testAuditThemeID, mock.Anything).Return(nil)
			},
			id:      testAuditThemeID,
			outcome: audit.OutcomeSucceeded,
		},
		{
			name: "the request has no display name",
			request: func() CreateThemeRequestWithID {
				r := request()
				r.DisplayName = ""
				return r
			},
			arrange: func() {},
			outcome: audit.OutcomeFailed("thm_1005"),
		},
		{
			name:    "the handle is taken",
			request: request,
			arrange: func() {
				suite.store.On("IsThemeHandleConflict", mock.Anything, testAuditHandle, "").Return(true, nil)
			},
			outcome: audit.OutcomeFailed("thm_1015"),
		},
		{
			name:    "the store fails",
			request: request,
			arrange: func() {
				suite.store.On("IsThemeHandleConflict", mock.Anything, testAuditHandle, "").Return(false, nil)
				suite.store.On("CreateTheme", mock.Anything, testAuditThemeID, mock.Anything).
					Return(errors.New("store failure"))
			},
			outcome: audit.OutcomeFailed("sse_5000"),
		},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			tc.arrange()

			created, svcErr := suite.service.CreateTheme(adminContext(), tc.request())

			suite.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			if svcErr == nil {
				suite.Equal(testAuditThemeID, created.ID)
			}
			suite.requireOnlyEvent(audit.EventThemeCreate, audit.ActionCreate, tc.id, tc.outcome)
		})
	}
}

func (suite *ThemeServiceAuditTestSuite) TestUpdateTheme_PublishesOneEventPerAttempt() {
	stored := Theme{ID: testAuditThemeID, Handle: testAuditHandle, DisplayName: testAuditDisplayName,
		Theme: auditedThemeContent()}
	request := func() UpdateThemeRequest {
		return UpdateThemeRequest{DisplayName: testAuditDisplayName, Description: testAuditDescription,
			Theme: auditedThemeContent()}
	}
	cases := []struct {
		name    string
		request func() UpdateThemeRequest
		arrange func()
		outcome audit.Outcome
	}{
		{
			name:    "the theme is updated",
			request: request,
			arrange: func() {
				suite.store.On("IsThemeDeclarative", mock.Anything, testAuditThemeID).Return(false)
				suite.store.On("GetTheme", mock.Anything, testAuditThemeID).Return(stored, nil)
				suite.store.On("UpdateTheme", mock.Anything, testAuditThemeID, mock.Anything).Return(nil)
			},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name:    "the theme does not exist",
			request: request,
			arrange: func() {
				suite.store.On("IsThemeDeclarative", mock.Anything, testAuditThemeID).Return(false)
				suite.store.On("GetTheme", mock.Anything, testAuditThemeID).Return(Theme{}, errThemeNotFound)
			},
			outcome: audit.OutcomeFailed("thm_1003"),
		},
		{
			name:    "the theme is declarative",
			request: request,
			arrange: func() {
				suite.store.On("IsThemeDeclarative", mock.Anything, testAuditThemeID).Return(true)
			},
			outcome: audit.OutcomeFailed("thm_1014"),
		},
		{
			name: "the request changes the handle",
			request: func() UpdateThemeRequest {
				r := request()
				r.Handle = "another-handle"
				return r
			},
			arrange: func() {
				suite.store.On("IsThemeDeclarative", mock.Anything, testAuditThemeID).Return(false)
				suite.store.On("GetTheme", mock.Anything, testAuditThemeID).Return(stored, nil)
			},
			outcome: audit.OutcomeFailed("thm_1017"),
		},
		{
			name:    "the store fails",
			request: request,
			arrange: func() {
				suite.store.On("IsThemeDeclarative", mock.Anything, testAuditThemeID).Return(false)
				suite.store.On("GetTheme", mock.Anything, testAuditThemeID).Return(stored, nil)
				suite.store.On("UpdateTheme", mock.Anything, testAuditThemeID, mock.Anything).
					Return(errors.New("store failure"))
			},
			outcome: audit.OutcomeFailed("sse_5000"),
		},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			tc.arrange()

			_, svcErr := suite.service.UpdateTheme(adminContext(), testAuditThemeID, tc.request())

			suite.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			suite.requireOnlyEvent(audit.EventThemeUpdate, audit.ActionUpdate, testAuditThemeID, tc.outcome)
		})
	}
}

func (suite *ThemeServiceAuditTestSuite) TestDeleteTheme_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		arrange func()
		outcome audit.Outcome
	}{
		{
			name: "the theme is deleted",
			arrange: func() {
				suite.store.On("IsThemeDeclarative", mock.Anything, testAuditThemeID).Return(false)
				suite.store.On("IsThemeExist", mock.Anything, testAuditThemeID).Return(true, nil)
				suite.store.On("DeleteTheme", mock.Anything, testAuditThemeID).Return(nil)
			},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name: "the theme is declarative",
			arrange: func() {
				suite.store.On("IsThemeDeclarative", mock.Anything, testAuditThemeID).Return(true)
			},
			outcome: audit.OutcomeFailed("thm_1014"),
		},
		{
			name: "the theme cannot be looked up",
			arrange: func() {
				suite.store.On("IsThemeDeclarative", mock.Anything, testAuditThemeID).Return(false)
				suite.store.On("IsThemeExist", mock.Anything, testAuditThemeID).
					Return(false, errors.New("store failure"))
			},
			outcome: audit.OutcomeFailed("sse_5000"),
		},
		{
			name: "the store fails",
			arrange: func() {
				suite.store.On("IsThemeDeclarative", mock.Anything, testAuditThemeID).Return(false)
				suite.store.On("IsThemeExist", mock.Anything, testAuditThemeID).Return(true, nil)
				suite.store.On("DeleteTheme", mock.Anything, testAuditThemeID).Return(errors.New("store failure"))
			},
			outcome: audit.OutcomeFailed("sse_5000"),
		},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			tc.arrange()

			svcErr := suite.service.DeleteTheme(adminContext(), testAuditThemeID)

			suite.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			suite.requireOnlyEvent(audit.EventThemeDelete, audit.ActionDelete, testAuditThemeID, tc.outcome)
		})
	}
}

// A delete stays idempotent, but one that finds no theme records nothing, so a caller cannot write a
// succeeded delete for any ID it chooses. The database store answers a missing ID with a count of zero,
// not an error, as the mock does here.
func (suite *ThemeServiceAuditTestSuite) TestDeleteTheme_PublishesNothingForAMissingTheme() {
	suite.store.On("IsThemeDeclarative", mock.Anything, testAuditThemeID).Return(false)
	suite.store.On("IsThemeExist", mock.Anything, testAuditThemeID).Return(false, nil)

	svcErr := suite.service.DeleteTheme(adminContext(), testAuditThemeID)

	suite.Nil(svcErr)
	suite.Empty(suite.published())
	suite.store.AssertNotCalled(suite.T(), "DeleteTheme", mock.Anything, mock.Anything)
}

// A bus that cannot take the event never fails the change it describes.
func (suite *ThemeServiceAuditTestSuite) TestCreateTheme_SucceedsWhenTheBusRefusesTheEvent() {
	failing := eventbusmock.NewEventBusInterfaceMock(suite.T())
	failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("audit queue is full")).Once()
	suite.service = newThemeMgtService(suite.store, failing)
	suite.store.On("IsThemeHandleConflict", mock.Anything, testAuditHandle, "").Return(false, nil)
	suite.store.On("CreateTheme", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	created, svcErr := suite.service.CreateTheme(adminContext(), CreateThemeRequestWithID{
		Handle: testAuditHandle, DisplayName: testAuditDisplayName, Theme: auditedThemeContent()})

	suite.Nil(svcErr)
	suite.NotEmpty(created.ID)
}
