// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notification

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/security"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
)

const (
	testAuditAdminID    = "admin-1"
	testAuditSenderID   = "0196a1b2-0000-7000-8000-0000000000a1"
	testAuditSenderName = "Audited Twilio Sender"
	testAuditAuthToken  = "audit-test-auth-token-0123456789"
	testAuditAccountSID = "AC" + "0123456789abcdef" + "0123456789abcdef"
)

// publishedEvent is one audit event the sender management service published.
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
	require.NoError(t, audit.Subscribe(registry, "notification-sender-service-test", sink))
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

// auditedTwilioSender is a Twilio sender whose properties carry its provider credentials.
func auditedTwilioSender() common.NotificationSenderDTO {
	return common.NotificationSenderDTO{
		Name:        testAuditSenderName,
		OUID:        "ou-1",
		Description: "Sends the one-time passwords",
		Type:        common.NotificationSenderTypeMessage,
		Provider:    common.NotificationProviderTypeTwilio,
		Properties: []cmodels.Property{
			createTestProperty("account_sid", testAuditAccountSID, true),
			createTestProperty("auth_token", testAuditAuthToken, true),
			createTestProperty("sender_id", "+15551234567", false),
		},
	}
}

type NotificationSenderMgtServiceAuditTestSuite struct {
	suite.Suite
	mockStore *notificationStoreInterfaceMock
	service   *notificationSenderMgtService
}

func TestNotificationSenderMgtServiceAuditTestSuite(t *testing.T) {
	suite.Run(t, new(NotificationSenderMgtServiceAuditTestSuite))
}

func (suite *NotificationSenderMgtServiceAuditTestSuite) SetupSuite() {
	config.ResetServerRuntime()
	suite.Require().NoError(config.InitializeServerRuntime("", &config.Config{
		Crypto: config.CryptoConfig{
			Encryption: engineconfig.EncryptionConfig{
				Key: "0579f866ac7c9273580d0ff163fa01a7b2401a7ff3ddc3e3b14ae3136fa6025e",
			},
		},
	}))
}

func (suite *NotificationSenderMgtServiceAuditTestSuite) TearDownSuite() {
	config.ResetServerRuntime()
}

func (suite *NotificationSenderMgtServiceAuditTestSuite) SetupTest() {
	suite.reset()
}

func (suite *NotificationSenderMgtServiceAuditTestSuite) SetupSubTest() {
	suite.reset()
}

// reset builds the service over a fresh store whose senders belong to the organization unit "ou-1".
func (suite *NotificationSenderMgtServiceAuditTestSuite) reset() {
	suite.mockStore = newNotificationStoreInterfaceMock(suite.T())
	mockOU := oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())
	mockOU.On("ResolveOwner", mock.Anything, "ou-1", "", mock.Anything).Return("ou-1", nil).Maybe()
	mockOU.On("ResolveOwnerForUpdate", mock.Anything, "ou-1", "", "ou-1", mock.Anything).
		Return("ou-1", nil).Maybe()
	suite.service = &notificationSenderMgtService{
		ouService:          mockOU,
		notificationStore:  suite.mockStore,
		transactioner:      &fakeTransactioner{},
		dependencyRegistry: newNoBlockingDepsRegistry(),
		uuidGenerator:      sysutils.GenerateUUIDv7,
		bus:                discardingBus{},
	}
}

// storedSender is the sender the store holds under testAuditSenderID.
func storedSender() *common.NotificationSenderDTO {
	sender := auditedTwilioSender()
	sender.ID = testAuditSenderID
	return &sender
}

// requireOnlyEvent checks that exactly one event was published, named name, for an attempt by the admin
// on the sender with senderID. An attempt that succeeded has the sender as subject too;
// a failed one has no subject.
func (suite *NotificationSenderMgtServiceAuditTestSuite) requireOnlyEvent(events []publishedEvent, name string,
	action audit.Action, senderID string, outcome audit.Outcome) {
	suite.T().Helper()
	suite.Require().Len(events, 1)
	event := events[0]
	sender := audit.Ref{Type: audit.TargetTypeNotificationSender, ID: senderID}
	suite.Equal(name, event.name)
	suite.Equal(sender, event.payload.Target())
	suite.Equal(action, event.payload.Action())
	suite.Equal(outcome, event.payload.Outcome())
	suite.Equal(audit.Actor{Type: audit.ActorTypeUser, ID: testAuditAdminID}, event.payload.Actor())
	suite.Empty(event.payload.Details(), "no property of the sender is recorded")
	if outcome.Result == audit.ResultFailed || senderID == "" {
		suite.Equal(audit.Ref{}, event.payload.Subject(), "a failed attempt may name no sender")
	} else {
		suite.Equal(sender, event.payload.Subject())
	}
	suite.requireNoCredential(event)
}

// requireNoCredential checks that no part of the event carries a property or the name of the sender.
func (suite *NotificationSenderMgtServiceAuditTestSuite) requireNoCredential(event publishedEvent) {
	suite.T().Helper()
	recorded := fmt.Sprintf("%+v", event.payload)
	suite.NotContains(recorded, testAuditAuthToken, "a provider credential is never recorded")
	suite.NotContains(recorded, testAuditAccountSID, "no property of the sender is recorded")
	suite.NotContains(recorded, testAuditSenderName, "the name of the sender is not recorded")
}

func (suite *NotificationSenderMgtServiceAuditTestSuite) TestCreateSender_PublishesTheCreatedSender() {
	bus, published := newRecordingBus(suite.T())
	suite.service.bus = bus
	suite.mockStore.EXPECT().getSenderByName(mock.Anything, testAuditSenderName).Return(nil, nil).Once()
	suite.mockStore.EXPECT().createSender(mock.Anything, mock.Anything).Return(nil).Once()

	created, svcErr := suite.service.CreateSender(adminContext(), auditedTwilioSender())

	suite.Require().Nil(svcErr)
	suite.Require().NotEmpty(created.ID)
	suite.requireOnlyEvent(published(), audit.EventNotificationSenderCreate, audit.ActionCreate, created.ID,
		audit.OutcomeSucceeded)
}

func (suite *NotificationSenderMgtServiceAuditTestSuite) TestCreateSender_PublishesAFailure() {
	cases := []struct {
		name    string
		sender  func() common.NotificationSenderDTO
		arrange func()
		outcome audit.Outcome
	}{
		{
			name: "the request has no name",
			sender: func() common.NotificationSenderDTO {
				sender := auditedTwilioSender()
				sender.Name = ""
				return sender
			},
			outcome: audit.OutcomeFailed("mns_1003"),
		},
		{
			name:   "a sender with the name exists",
			sender: auditedTwilioSender,
			arrange: func() {
				suite.mockStore.EXPECT().getSenderByName(mock.Anything, testAuditSenderName).
					Return(storedSender(), nil).Once()
			},
			outcome: audit.OutcomeFailed("mns_1005"),
		},
		{
			name:   "the store fails",
			sender: auditedTwilioSender,
			arrange: func() {
				suite.mockStore.EXPECT().getSenderByName(mock.Anything, testAuditSenderName).Return(nil, nil).Once()
				suite.mockStore.EXPECT().createSender(mock.Anything, mock.Anything).
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

			created, svcErr := suite.service.CreateSender(adminContext(), tc.sender())

			suite.Nil(created)
			suite.Require().NotNil(svcErr)
			suite.requireOnlyEvent(published(), audit.EventNotificationSenderCreate, audit.ActionCreate, "",
				tc.outcome)
		})
	}
}

func (suite *NotificationSenderMgtServiceAuditTestSuite) TestUpdateSender_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		sender  func() common.NotificationSenderDTO
		arrange func()
		outcome audit.Outcome
	}{
		{
			name:   "the sender is updated",
			sender: auditedTwilioSender,
			arrange: func() {
				suite.mockStore.EXPECT().getSenderByID(mock.Anything, testAuditSenderID).
					Return(storedSender(), nil).Once()
				suite.mockStore.EXPECT().updateSender(mock.Anything, testAuditSenderID, mock.Anything).
					Return(nil).Once()
			},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name:   "the sender does not exist",
			sender: auditedTwilioSender,
			arrange: func() {
				suite.mockStore.EXPECT().getSenderByID(mock.Anything, testAuditSenderID).Return(nil, nil).Once()
			},
			outcome: audit.OutcomeFailed("mns_1001"),
		},
		{
			name: "another sender has the new name",
			sender: func() common.NotificationSenderDTO {
				sender := auditedTwilioSender()
				sender.Name = "Another Sender"
				return sender
			},
			arrange: func() {
				suite.mockStore.EXPECT().getSenderByID(mock.Anything, testAuditSenderID).
					Return(storedSender(), nil).Once()
				suite.mockStore.EXPECT().getSenderByName(mock.Anything, "Another Sender").
					Return(&common.NotificationSenderDTO{ID: "another-id", Name: "Another Sender"}, nil).Once()
			},
			outcome: audit.OutcomeFailed("mns_1005"),
		},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			bus, published := newRecordingBus(suite.T())
			suite.service.bus = bus
			tc.arrange()

			_, svcErr := suite.service.UpdateSender(adminContext(), testAuditSenderID, tc.sender())

			suite.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			suite.requireOnlyEvent(published(), audit.EventNotificationSenderUpdate, audit.ActionUpdate,
				testAuditSenderID, tc.outcome)
		})
	}
}

func (suite *NotificationSenderMgtServiceAuditTestSuite) TestDeleteSender_PublishesOneEventPerAttempt() {
	blocking := 1
	cases := []struct {
		name    string
		arrange func()
		outcome audit.Outcome
	}{
		{
			name: "the sender is deleted",
			arrange: func() {
				suite.mockStore.EXPECT().getSenderByID(mock.Anything, testAuditSenderID).
					Return(storedSender(), nil).Once()
				suite.mockStore.EXPECT().deleteSender(mock.Anything, testAuditSenderID).Return(nil).Once()
			},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name: "a flow uses the sender",
			arrange: func() {
				suite.service.dependencyRegistry = &stubDependencyRegistry{
					resp: &resourcedependency.DependenciesResponse{
						TotalResults: &blocking,
						Count:        1,
						Usages: []resourcedependency.ResourceDependency{{
							ResourceType: resourcedependency.ResourceTypeFlow, ID: "flow-1",
							DisplayName: "SMS OTP", BehaviorOnDelete: resourcedependency.BehaviorRestrict,
						}},
					}}
			},
			outcome: audit.OutcomeFailed("mns_1016"),
		},
		{
			name: "the store fails",
			arrange: func() {
				suite.mockStore.EXPECT().getSenderByID(mock.Anything, testAuditSenderID).
					Return(storedSender(), nil).Once()
				suite.mockStore.EXPECT().deleteSender(mock.Anything, testAuditSenderID).
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

			svcErr := suite.service.DeleteSender(adminContext(), testAuditSenderID)

			suite.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			suite.requireOnlyEvent(published(), audit.EventNotificationSenderDelete, audit.ActionDelete,
				testAuditSenderID, tc.outcome)
		})
	}
}

// A delete stays idempotent, but one that finds no sender records nothing, so a caller cannot write a
// succeeded delete for any ID it chooses. The store deletes without reporting whether a row matched, so
// the service looks the sender up and never reaches the delete.
func (suite *NotificationSenderMgtServiceAuditTestSuite) TestDeleteSender_PublishesNothingForAMissingSender() {
	bus, published := newRecordingBus(suite.T())
	suite.service.bus = bus
	suite.mockStore.EXPECT().getSenderByID(mock.Anything, testAuditSenderID).Return(nil, nil).Once()

	svcErr := suite.service.DeleteSender(adminContext(), testAuditSenderID)

	suite.Nil(svcErr)
	suite.Empty(published())
	suite.mockStore.AssertNotCalled(suite.T(), "deleteSender", mock.Anything, mock.Anything)
}

// Every change is published after its transaction returns, so a change that rolls back is never
// recorded as made.
func (suite *NotificationSenderMgtServiceAuditTestSuite) TestEveryChangeIsPublishedAfterItsTransaction() {
	tx := &committingTransactioner{}
	bus := &commitCheckingBus{t: suite.T(), tx: tx}
	suite.service.transactioner = tx
	suite.service.bus = bus
	suite.mockStore.EXPECT().getSenderByName(mock.Anything, testAuditSenderName).Return(nil, nil).Once()
	suite.mockStore.EXPECT().createSender(mock.Anything, mock.Anything).Return(nil).Once()
	suite.mockStore.EXPECT().getSenderByID(mock.Anything, testAuditSenderID).Return(storedSender(), nil).Twice()
	suite.mockStore.EXPECT().updateSender(mock.Anything, testAuditSenderID, mock.Anything).Return(nil).Once()
	suite.mockStore.EXPECT().deleteSender(mock.Anything, testAuditSenderID).Return(nil).Once()

	_, svcErr := suite.service.CreateSender(adminContext(), auditedTwilioSender())
	suite.Require().Nil(svcErr)
	_, svcErr = suite.service.UpdateSender(adminContext(), testAuditSenderID, auditedTwilioSender())
	suite.Require().Nil(svcErr)
	suite.Require().Nil(suite.service.DeleteSender(adminContext(), testAuditSenderID))

	suite.Equal(3, bus.published)
}

// A bus that cannot take the event never fails the change it describes.
func (suite *NotificationSenderMgtServiceAuditTestSuite) TestCreateSender_SucceedsWhenTheBusRefusesTheEvent() {
	failing := eventbusmock.NewEventBusInterfaceMock(suite.T())
	failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("audit queue is full")).Once()
	suite.service.bus = failing
	suite.mockStore.EXPECT().getSenderByName(mock.Anything, testAuditSenderName).Return(nil, nil).Once()
	suite.mockStore.EXPECT().createSender(mock.Anything, mock.Anything).Return(nil).Once()

	created, svcErr := suite.service.CreateSender(adminContext(), auditedTwilioSender())

	suite.Nil(svcErr)
	suite.NotEmpty(created.ID)
}
