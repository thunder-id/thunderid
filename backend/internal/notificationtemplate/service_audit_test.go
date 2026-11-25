// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import (
	"context"
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
	testAuditAdminID        = "admin-1"
	testAuditTemplateID     = "0196a1b2-0000-7000-8000-0000000000b1"
	testAuditTemplateHandle = "audited-welcome"
	testAuditDisplayName    = "Audited Welcome"
	testAuditSubject        = "Welcome to the audited deployment"
	testAuditBody           = "<p>Hi {{ctx(name)}}, your audited account is ready</p>"
)

// publishedEvent is one audit event the template service published.
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
	require.NoError(t, audit.Subscribe(registry, "notification-template-service-test", sink))
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

// committingTransactioner runs a transaction and records when it has returned, so a test can tell
// whether an event was published inside it.
type committingTransactioner struct {
	committed bool
}

func (c *committingTransactioner) Transact(ctx context.Context, txFunc func(context.Context) error) error {
	c.committed = false
	err := txFunc(ctx)
	c.committed = true
	return err
}

// commitCheckingBus fails the test when an event is published before the transaction returned.
type commitCheckingBus struct {
	t         *testing.T
	tx        *committingTransactioner
	published int
}

func (b *commitCheckingBus) Publish(context.Context, *eventbus.Event) error {
	require.True(b.t, b.tx.committed, "an event was published inside the transaction")
	b.published++
	return nil
}

// adminContext is the context of a management call made by the admin.
func adminContext() context.Context {
	return security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest(testAuditAdminID, "ou-1", "", nil, nil))
}

// auditedCreateRequest is a request for an email template with a subject and a body.
func auditedCreateRequest() CreateTemplateRequest {
	return CreateTemplateRequest{
		Handle:      testAuditTemplateHandle,
		DisplayName: testAuditDisplayName,
		Description: "Sent when an account is created",
		Content:     TemplateContent{Subject: testAuditSubject, Body: testAuditBody},
	}
}

// auditedUpdateRequest is a request that rewrites the template's content.
func auditedUpdateRequest() UpdateTemplateRequest {
	return UpdateTemplateRequest{
		DisplayName: testAuditDisplayName,
		Content:     TemplateContent{Subject: testAuditSubject, Body: testAuditBody},
	}
}

// storedTemplate is the template the store holds under testAuditTemplateID.
func storedTemplate() templateDAO {
	return templateDAO{
		ID:          testAuditTemplateID,
		Channel:     ChannelTypeEmail,
		Handle:      testAuditTemplateHandle,
		OUID:        "ou-1",
		DisplayName: testAuditDisplayName,
		Content:     TemplateContent{Subject: testAuditSubject, Body: testAuditBody},
	}
}

type NotificationTemplateServiceAuditTestSuite struct {
	suite.Suite
	mockStore *notificationTemplateStoreInterfaceMock
	service   *notificationTemplateService
}

func TestNotificationTemplateServiceAuditTestSuite(t *testing.T) {
	suite.Run(t, new(NotificationTemplateServiceAuditTestSuite))
}

func (suite *NotificationTemplateServiceAuditTestSuite) SetupTest() {
	suite.reset()
}

func (suite *NotificationTemplateServiceAuditTestSuite) SetupSubTest() {
	suite.reset()
}

// reset builds the service over a fresh store in a mutable deployment.
func (suite *NotificationTemplateServiceAuditTestSuite) reset() {
	initMutableTestRuntime(suite.T())
	suite.mockStore = newNotificationTemplateStoreInterfaceMock(suite.T())
	suite.service = newNotificationTemplateService(suite.mockStore, executingTransactioner(suite.T()), nil,
		allowAllAuthz(suite.T()), nil, false, discardingBus{}).(*notificationTemplateService)
}

// requireOnlyEvent checks that exactly one event was published, named name, for an attempt by the admin
// on the template with templateID. An attempt that succeeded has the template as
// subject too; a failed one has no subject.
func (suite *NotificationTemplateServiceAuditTestSuite) requireOnlyEvent(events []publishedEvent, name string,
	action audit.Action, templateID string, outcome audit.Outcome) {
	suite.T().Helper()
	suite.Require().Len(events, 1)
	event := events[0]
	template := audit.Ref{Type: audit.TargetTypeNotificationTemplate, ID: templateID}
	suite.Equal(name, event.name)
	suite.Equal(template, event.payload.Target())
	suite.Equal(action, event.payload.Action())
	suite.Equal(outcome, event.payload.Outcome())
	suite.Equal(audit.Actor{Type: audit.ActorTypeUser, ID: testAuditAdminID}, event.payload.Actor())
	suite.Empty(event.payload.Details(), "no part of the template is recorded")
	if outcome.Result == audit.ResultFailed || templateID == "" {
		suite.Equal(audit.Ref{}, event.payload.Subject(), "a failed attempt may name no template")
	} else {
		suite.Equal(template, event.payload.Subject())
	}
	recorded := fmt.Sprintf("%+v", event.payload)
	for _, value := range []string{testAuditTemplateHandle, testAuditDisplayName, testAuditSubject, testAuditBody} {
		suite.NotContains(recorded, value, "no part of the template is recorded")
	}
}

func (suite *NotificationTemplateServiceAuditTestSuite) TestCreateTemplate_PublishesTheCreatedTemplate() {
	bus, published := newRecordingBus(suite.T())
	suite.service.bus = bus
	suite.mockStore.On("IsHandleExists", mock.Anything, ChannelTypeEmail, testAuditTemplateHandle).
		Return(false, nil).Once()
	suite.mockStore.On("CreateTemplate", mock.Anything, mock.Anything).Return(nil).Once()

	created, svcErr := suite.service.CreateTemplate(adminContext(), ChannelTypeEmail, auditedCreateRequest())

	suite.Require().Nil(svcErr)
	suite.Require().NotEmpty(created.ID)
	suite.requireOnlyEvent(published(), audit.EventNotificationTemplateCreate, audit.ActionCreate, created.ID,
		audit.OutcomeSucceeded)
}

func (suite *NotificationTemplateServiceAuditTestSuite) TestCreateTemplate_PublishesAFailure() {
	cases := []struct {
		name    string
		request func() CreateTemplateRequest
		arrange func()
		outcome audit.Outcome
	}{
		{
			name: "the request has no handle",
			request: func() CreateTemplateRequest {
				request := auditedCreateRequest()
				request.Handle = ""
				return request
			},
			outcome: audit.OutcomeFailed("ntm_1008"),
		},
		{
			name:    "a template with the handle exists",
			request: auditedCreateRequest,
			arrange: func() {
				suite.mockStore.On("IsHandleExists", mock.Anything, ChannelTypeEmail, testAuditTemplateHandle).
					Return(true, nil).Once()
			},
			outcome: audit.OutcomeFailed("ntm_1009"),
		},
		{
			name:    "the store fails",
			request: auditedCreateRequest,
			arrange: func() {
				suite.mockStore.On("IsHandleExists", mock.Anything, ChannelTypeEmail, testAuditTemplateHandle).
					Return(false, nil).Once()
				suite.mockStore.On("CreateTemplate", mock.Anything, mock.Anything).
					Return(errors.New("store failure")).Once()
			},
			outcome: audit.OutcomeFailed("sse_5000"),
		},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			bus, published := newRecordingBus(suite.T())
			suite.service.bus = bus
			if tc.arrange != nil {
				tc.arrange()
			}

			created, svcErr := suite.service.CreateTemplate(adminContext(), ChannelTypeEmail, tc.request())

			suite.Nil(created)
			suite.Require().NotNil(svcErr)
			suite.requireOnlyEvent(published(), audit.EventNotificationTemplateCreate, audit.ActionCreate, "",
				tc.outcome)
		})
	}
}

func (suite *NotificationTemplateServiceAuditTestSuite) TestUpdateTemplate_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		arrange func()
		outcome audit.Outcome
	}{
		{
			name: "the template is updated",
			arrange: func() {
				suite.mockStore.On("GetTemplate", mock.Anything, ChannelTypeEmail, testAuditTemplateID).
					Return(storedTemplate(), nil).Once()
				suite.mockStore.On("UpdateTemplate", mock.Anything, mock.Anything).Return(nil).Once()
			},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name: "the template does not exist",
			arrange: func() {
				suite.mockStore.On("GetTemplate", mock.Anything, ChannelTypeEmail, testAuditTemplateID).
					Return(templateDAO{}, errTemplateNotFound).Once()
			},
			outcome: audit.OutcomeFailed("ntm_1004"),
		},
		{
			name: "the template is declared in a file",
			arrange: func() {
				declared := storedTemplate()
				declared.IsReadOnly = true
				suite.mockStore.On("GetTemplate", mock.Anything, ChannelTypeEmail, testAuditTemplateID).
					Return(declared, nil).Once()
			},
			outcome: audit.OutcomeFailed("ntm_1022"),
		},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			bus, published := newRecordingBus(suite.T())
			suite.service.bus = bus
			tc.arrange()

			_, svcErr := suite.service.UpdateTemplate(adminContext(), ChannelTypeEmail, testAuditTemplateID,
				auditedUpdateRequest())

			suite.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			suite.requireOnlyEvent(published(), audit.EventNotificationTemplateUpdate, audit.ActionUpdate,
				testAuditTemplateID, tc.outcome)
		})
	}
}

func (suite *NotificationTemplateServiceAuditTestSuite) TestDeleteTemplate_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		arrange func()
		outcome audit.Outcome
	}{
		{
			name: "the template is deleted",
			arrange: func() {
				suite.mockStore.On("GetTemplate", mock.Anything, ChannelTypeEmail, testAuditTemplateID).
					Return(storedTemplate(), nil).Once()
				suite.mockStore.On("DeleteTemplate", mock.Anything, ChannelTypeEmail, testAuditTemplateID).
					Return(nil).Once()
			},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name: "the template is declared in a file",
			arrange: func() {
				declared := storedTemplate()
				declared.IsReadOnly = true
				suite.mockStore.On("GetTemplate", mock.Anything, ChannelTypeEmail, testAuditTemplateID).
					Return(declared, nil).Once()
			},
			outcome: audit.OutcomeFailed("ntm_1022"),
		},
		{
			name: "the store fails",
			arrange: func() {
				suite.mockStore.On("GetTemplate", mock.Anything, ChannelTypeEmail, testAuditTemplateID).
					Return(storedTemplate(), nil).Once()
				suite.mockStore.On("DeleteTemplate", mock.Anything, ChannelTypeEmail, testAuditTemplateID).
					Return(errors.New("store failure")).Once()
			},
			outcome: audit.OutcomeFailed("sse_5000"),
		},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			bus, published := newRecordingBus(suite.T())
			suite.service.bus = bus
			tc.arrange()

			svcErr := suite.service.DeleteTemplate(adminContext(), ChannelTypeEmail, testAuditTemplateID)

			suite.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			suite.requireOnlyEvent(published(), audit.EventNotificationTemplateDelete, audit.ActionDelete,
				testAuditTemplateID, tc.outcome)
		})
	}
}

// A delete stays idempotent, but one that finds no template records nothing, so a caller cannot write a
// succeeded delete for any ID it chooses.
func (suite *NotificationTemplateServiceAuditTestSuite) TestDeleteTemplate_PublishesNothingForAMissingTemplate() {
	bus, published := newRecordingBus(suite.T())
	suite.service.bus = bus
	suite.mockStore.On("GetTemplate", mock.Anything, ChannelTypeEmail, testAuditTemplateID).
		Return(templateDAO{}, errTemplateNotFound).Once()

	svcErr := suite.service.DeleteTemplate(adminContext(), ChannelTypeEmail, testAuditTemplateID)

	suite.Nil(svcErr)
	suite.Empty(published())
	suite.mockStore.AssertNotCalled(suite.T(), "DeleteTemplate", mock.Anything, mock.Anything, mock.Anything)
}

// Every change is published after its transaction returns, so a change that rolls back is never
// recorded as made.
func (suite *NotificationTemplateServiceAuditTestSuite) TestEveryChangeIsPublishedAfterItsTransaction() {
	tx := &committingTransactioner{}
	bus := &commitCheckingBus{t: suite.T(), tx: tx}
	suite.service.transactioner = tx
	suite.service.bus = bus
	suite.mockStore.On("IsHandleExists", mock.Anything, ChannelTypeEmail, testAuditTemplateHandle).
		Return(false, nil).Once()
	suite.mockStore.On("CreateTemplate", mock.Anything, mock.Anything).Return(nil).Once()
	suite.mockStore.On("GetTemplate", mock.Anything, ChannelTypeEmail, testAuditTemplateID).
		Return(storedTemplate(), nil).Twice()
	suite.mockStore.On("UpdateTemplate", mock.Anything, mock.Anything).Return(nil).Once()
	suite.mockStore.On("DeleteTemplate", mock.Anything, ChannelTypeEmail, testAuditTemplateID).Return(nil).Once()

	_, svcErr := suite.service.CreateTemplate(adminContext(), ChannelTypeEmail, auditedCreateRequest())
	suite.Require().Nil(svcErr)
	_, svcErr = suite.service.UpdateTemplate(adminContext(), ChannelTypeEmail, testAuditTemplateID,
		auditedUpdateRequest())
	suite.Require().Nil(svcErr)
	suite.Require().Nil(suite.service.DeleteTemplate(adminContext(), ChannelTypeEmail, testAuditTemplateID))

	suite.Equal(3, bus.published)
}

// A bus that cannot take the event never fails the change it describes.
func (suite *NotificationTemplateServiceAuditTestSuite) TestCreateTemplate_SucceedsWhenTheBusRefusesTheEvent() {
	failing := eventbusmock.NewEventBusInterfaceMock(suite.T())
	failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("audit queue is full")).Once()
	suite.service.bus = failing
	suite.mockStore.On("IsHandleExists", mock.Anything, ChannelTypeEmail, testAuditTemplateHandle).
		Return(false, nil).Once()
	suite.mockStore.On("CreateTemplate", mock.Anything, mock.Anything).Return(nil).Once()

	created, svcErr := suite.service.CreateTemplate(adminContext(), ChannelTypeEmail, auditedCreateRequest())

	suite.Nil(svcErr)
	suite.NotEmpty(created.ID)
}
