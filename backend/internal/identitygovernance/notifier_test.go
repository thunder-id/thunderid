// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernance

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	syscontext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/deployment"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	notifcommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/notificationtemplate"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/notification/notificationmock"
)

type NotifierTestSuite struct {
	suite.Suite

	profiles  *EntityStateProviderMock
	templates *notificationTemplateRendererMock
	senders   *notificationmock.NotificationSenderServiceInterfaceMock
	sender    *notificationmock.DefaultEmailSenderInterfaceMock
	sent      chan notifcommon.EmailData
}

// testSenderID is the default email sender the suite configures.
const testSenderID = "sender-1"

func TestNotifierTestSuite(t *testing.T) {
	suite.Run(t, new(NotifierTestSuite))
}

func (s *NotifierTestSuite) SetupTest() {
	s.profiles = NewEntityStateProviderMock(s.T())
	s.templates = newNotificationTemplateRendererMock(s.T())
	s.senders = notificationmock.NewNotificationSenderServiceInterfaceMock(s.T())
	s.sender = notificationmock.NewDefaultEmailSenderInterfaceMock(s.T())
	s.sender.EXPECT().DefaultEmailSenderID(mock.Anything).Return(testSenderID).Maybe()
	s.sent = make(chan notifcommon.EmailData, 4)
}

func testUser() GovernedEntity {
	return GovernedEntity{ID: "user-1", Category: providers.EntityCategoryUser, Type: "member"}
}

func testLockEpisode() LockEpisode {
	return LockEpisode{Duration: 5 * time.Minute, UnlockAt: "2026-10-08T10:05:00Z", LockCount: 1}
}

func lockEmailConfig(email accountAccessLockEmail) accountAccessValue {
	return accountAccessValue{User: &accountAccessCategory{
		Notifications: &accountAccessNotifications{
			OnLock: &accountAccessLockNotification{Email: &email},
		},
	}}
}

// notifierFor builds a notifier over the suite's mocks, reading the given settings through the source.
func (s *NotifierTestSuite) notifierFor(settings accountAccessLockEmail) *lockEmailNotifier {
	notifier := newLockEmailNotifier(newAccountAccessSource(lockEmailConfig(settings), nil),
		s.profiles, s.templates, s.senders, s.sender)
	s.T().Cleanup(notifier.Close)
	return notifier
}

// expectProfile answers the profile read with the given string attributes.
func (s *NotifierTestSuite) expectProfile(entity GovernedEntity, attributes map[string]string) {
	raw, err := json.Marshal(attributes)
	s.Require().NoError(err)
	s.profiles.EXPECT().GetEntityProfile(mock.Anything, entity.ID).
		Return(&providers.Entity{ID: entity.ID, Category: entity.Category, Attributes: raw}, nil)
}

// expectRender returns a rendered mail and captures the handle and template data.
func (s *NotifierTestSuite) expectRender(handle *string, data *map[string]string) {
	s.templates.EXPECT().Resolve(mock.Anything, notificationtemplate.ChannelTypeEmail, mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, _ notificationtemplate.ChannelType, h string,
			in notificationtemplate.RenderInput) (*notificationtemplate.ResolvedContent, *tidcommon.ServiceError) {
			*handle, *data = h, in.Data
			return &notificationtemplate.ResolvedContent{Subject: "Sign-in was locked", Body: "body"}, nil
		})
}

// expectSend records what reached the default email sender.
func (s *NotifierTestSuite) expectSend(svcErr *tidcommon.ServiceError) {
	s.senders.EXPECT().SendEmail(mock.Anything, testSenderID, mock.Anything).
		RunAndReturn(func(_ context.Context, _ string, data notifcommon.EmailData) *tidcommon.ServiceError {
			s.sent <- data
			return svcErr
		})
}

// assertNothingSent checks the sender was never called.
func (s *NotifierTestSuite) assertNothingSent() {
	s.senders.AssertNotCalled(s.T(), "SendEmail", mock.Anything, mock.Anything, mock.Anything)
}

// awaitSend waits for the worker goroutine to send.
func (s *NotifierTestSuite) awaitSend() notifcommon.EmailData {
	select {
	case data := <-s.sent:
		return data
	case <-time.After(2 * time.Second):
		s.FailNow("timed out waiting for the lock notification to be sent")
		return notifcommon.EmailData{}
	}
}

// With no attribute configured, the source's default attribute is used.
func (s *NotifierTestSuite) TestSendsToTheDefaultAttribute() {
	entity := testUser()
	s.expectProfile(entity, map[string]string{"email": "member@example.com"})
	var handle string
	var data map[string]string
	s.expectRender(&handle, &data)
	s.expectSend(nil)

	notifier := s.notifierFor(accountAccessLockEmail{Enabled: new(true)})
	notifier.NotifyLockFormed(context.Background(), entity, providers.AccessScopeCredential, testLockEpisode())

	sent := s.awaitSend()
	s.Equal([]string{"member@example.com"}, sent.To)
	s.True(sent.IsHTML)
	s.Equal(lockTemplateHandle, handle)
}

// The delivery outlives the failed sign-in's request but keeps its trace ID and deployment.
func (s *NotifierTestSuite) TestDeliveryKeepsTheRequestContext() {
	entity := testUser()
	s.expectProfile(entity, map[string]string{"email": "member@example.com"})
	var handle string
	var data map[string]string
	s.expectRender(&handle, &data)
	delivered := make(chan context.Context, 1)
	s.senders.EXPECT().SendEmail(mock.Anything, testSenderID, mock.Anything).
		RunAndReturn(func(ctx context.Context, _ string, _ notifcommon.EmailData) *tidcommon.ServiceError {
			delivered <- ctx
			return nil
		})

	notifier := s.notifierFor(accountAccessLockEmail{Enabled: new(true)})
	reqCtx, cancel := context.WithCancel(
		deployment.WithID(syscontext.WithTraceID(context.Background(), "trace-1"), "deployment-1"))
	notifier.NotifyLockFormed(reqCtx, entity, providers.AccessScopeCredential, testLockEpisode())
	cancel()

	select {
	case ctx := <-delivered:
		s.Equal("trace-1", syscontext.GetTraceID(ctx))
		s.Equal("deployment-1", deployment.Resolve(ctx))
	case <-time.After(2 * time.Second):
		s.FailNow("timed out waiting for the lock notification to be sent")
	}
}

// The named attribute is used even when an "email" attribute also exists.
func (s *NotifierTestSuite) TestSendsToTheNamedAttribute() {
	entity := testUser()
	s.expectProfile(entity, map[string]string{
		"email":     "personal@example.com",
		"workEmail": "work@example.com",
	})
	var handle string
	var data map[string]string
	s.expectRender(&handle, &data)
	s.expectSend(nil)

	notifier := s.notifierFor(accountAccessLockEmail{
		Enabled: new(true), RecipientAttribute: new("workEmail"),
	})
	notifier.NotifyLockFormed(context.Background(), entity, providers.AccessScopeCredential, testLockEpisode())

	s.Equal([]string{"work@example.com"}, s.awaitSend().To)
}

// A user missing the named attribute is skipped, even if another address exists.
func (s *NotifierTestSuite) TestMissingNamedAttributeNeverFallsBack() {
	entity := testUser()
	s.expectProfile(entity, map[string]string{"email": "personal@example.com"})

	notifier := s.notifierFor(accountAccessLockEmail{
		Enabled: new(true), RecipientAttribute: new("workEmail"),
	})
	notifier.NotifyLockFormed(context.Background(), entity, providers.AccessScopeCredential, testLockEpisode())

	notifier.Close()
	s.assertNothingSent()
}

func (s *NotifierTestSuite) TestSkipsAnUnusableAddress() {
	entity := testUser()
	s.expectProfile(entity, map[string]string{"email": "not an address"})

	notifier := s.notifierFor(accountAccessLockEmail{Enabled: new(true)})
	notifier.NotifyLockFormed(context.Background(), entity, providers.AccessScopeCredential, testLockEpisode())

	notifier.Close()
	s.assertNothingSent()
}

// Settings are re-read at delivery, so a switch turned off after queuing is honored.
func (s *NotifierTestSuite) TestSwitchedOffBeforeDeliveryIsNotSent() {
	source := newAccountAccessSourceInterfaceMock(s.T())
	source.EXPECT().Current(mock.Anything).
		Return(lockEmailConfig(accountAccessLockEmail{Enabled: new(true)})).Once()
	source.EXPECT().Current(mock.Anything).Return(lockEmailConfig(accountAccessLockEmail{})).Once()
	notifier := &lockEmailNotifier{
		source:    source,
		profiles:  s.profiles,
		templates: s.templates,
		senders:   s.senders,
		sender:    s.sender,
		queue:     make(chan lockNotice, 1),
		timeout:   noticeTimeout,
		logger:    log.GetLogger(),
	}

	notifier.NotifyLockFormed(context.Background(), testUser(), providers.AccessScopeCredential, testLockEpisode())
	notifier.deliver(<-notifier.queue)

	s.profiles.AssertNotCalled(s.T(), "GetEntityProfile", mock.Anything, mock.Anything)
	s.assertNothingSent()
}

// A notice after Close is dropped instead of sending on the closed queue.
func (s *NotifierTestSuite) TestANoticeAfterCloseIsDropped() {
	notifier := s.notifierFor(accountAccessLockEmail{Enabled: new(true)})
	notifier.Close()

	s.NotPanics(func() {
		notifier.NotifyLockFormed(context.Background(), testUser(), providers.AccessScopeCredential, testLockEpisode())
	})
	s.profiles.AssertNotCalled(s.T(), "GetEntityProfile", mock.Anything, mock.Anything)
	s.assertNothingSent()
}

// When disabled, nothing is queued and no profile is read.
func (s *NotifierTestSuite) TestDisabledIsNotEvenQueued() {
	notifier := s.notifierFor(accountAccessLockEmail{})
	notifier.NotifyLockFormed(context.Background(), testUser(), providers.AccessScopeCredential, testLockEpisode())

	notifier.Close()
	s.profiles.AssertNotCalled(s.T(), "GetEntityProfile", mock.Anything, mock.Anything)
}

// A failed or empty profile read skips the notice without retry.
func (s *NotifierTestSuite) TestProfileReadFailureSkipsDelivery() {
	for _, read := range []struct {
		profile *providers.Entity
		err     error
	}{{nil, errors.New("store down")}, {nil, nil}} {
		profiles := NewEntityStateProviderMock(s.T())
		profiles.EXPECT().GetEntityProfile(mock.Anything, "user-1").Return(read.profile, read.err)
		notifier := newLockEmailNotifier(newAccountAccessSource(lockEmailConfig(
			accountAccessLockEmail{Enabled: new(true)}), nil),
			profiles, s.templates, s.senders, s.sender)

		notifier.NotifyLockFormed(context.Background(), testUser(), providers.AccessScopeCredential, testLockEpisode())
		notifier.Close()
	}
	s.assertNothingSent()
}

// A missing template sends nothing.
func (s *NotifierTestSuite) TestTemplateFailureSendsNothing() {
	entity := testUser()
	s.expectProfile(entity, map[string]string{"email": "member@example.com"})
	s.templates.EXPECT().Resolve(mock.Anything, notificationtemplate.ChannelTypeEmail, lockTemplateHandle,
		mock.Anything).Return(nil, &tidcommon.ServiceError{Code: "NTPL-404"})

	notifier := s.notifierFor(accountAccessLockEmail{Enabled: new(true)})
	notifier.NotifyLockFormed(context.Background(), entity, providers.AccessScopeCredential, testLockEpisode())

	notifier.Close()
	s.assertNothingSent()
}

// A refused send is recorded, not retried.
func (s *NotifierTestSuite) TestSendFailureIsNotRetried() {
	entity := testUser()
	s.expectProfile(entity, map[string]string{"email": "member@example.com"})
	var handle string
	var data map[string]string
	s.expectRender(&handle, &data)
	s.expectSend(&tidcommon.InternalServerError)

	notifier := s.notifierFor(accountAccessLockEmail{Enabled: new(true)})
	notifier.NotifyLockFormed(context.Background(), entity, providers.AccessScopeCredential, testLockEpisode())

	s.awaitSend()
	notifier.Close()
	s.senders.AssertNumberOfCalls(s.T(), "SendEmail", 1)
}

// Without a default email sender nothing is rendered or sent.
func (s *NotifierTestSuite) TestNoDefaultSenderSendsNothing() {
	entity := testUser()
	s.expectProfile(entity, map[string]string{"email": "member@example.com"})
	sender := notificationmock.NewDefaultEmailSenderInterfaceMock(s.T())
	sender.EXPECT().DefaultEmailSenderID(mock.Anything).Return("")

	notifier := newLockEmailNotifier(newAccountAccessSource(lockEmailConfig(
		accountAccessLockEmail{Enabled: new(true)}), nil),
		s.profiles, s.templates, s.senders, sender)
	notifier.NotifyLockFormed(context.Background(), entity, providers.AccessScopeCredential, testLockEpisode())

	notifier.Close()
	s.templates.AssertNotCalled(s.T(), "Resolve", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	s.assertNothingSent()
}

// A full queue drops the notice rather than blocking the caller.
func (s *NotifierTestSuite) TestFullQueueDropsTheNotice() {
	notifier := &lockEmailNotifier{
		source: newAccountAccessSource(lockEmailConfig(
			accountAccessLockEmail{Enabled: new(true)}), nil),
		profiles:  s.profiles,
		templates: s.templates,
		senders:   s.senders,
		sender:    s.sender,
		// No workers and a queue of one, so the second notice is dropped.
		queue:   make(chan lockNotice, 1),
		timeout: noticeTimeout,
		logger:  log.GetLogger(),
	}

	notifier.NotifyLockFormed(context.Background(), testUser(), providers.AccessScopeCredential, testLockEpisode())
	notifier.NotifyLockFormed(context.Background(), testUser(), providers.AccessScopeCredential, testLockEpisode())

	s.Len(notifier.queue, 1)
}

func (s *NotifierTestSuite) TestLockEmailIsOffUnlessEnabled() {
	_, enabled := lockEmailRecipient(lockEmailConfig(accountAccessLockEmail{}),
		providers.EntityCategoryUser)
	s.False(enabled)
	_, enabled = lockEmailRecipient(accountAccessValue{}, providers.EntityCategoryUser)
	s.False(enabled)
}

func (s *NotifierTestSuite) TestLockEmailReadsTheNamedAttribute() {
	attribute, enabled := lockEmailRecipient(lockEmailConfig(accountAccessLockEmail{
		Enabled: new(true), RecipientAttribute: new(" workEmail "),
	}), providers.EntityCategoryUser)
	s.True(enabled)
	s.Equal("workEmail", attribute)
}

// Agents and applications are not notified.
func (s *NotifierTestSuite) TestOnlyUsersAreNotified() {
	cfg := lockEmailConfig(accountAccessLockEmail{Enabled: new(true)})
	for _, category := range []providers.EntityCategory{providers.EntityCategoryAgent, providers.EntityCategoryApp} {
		_, enabled := lockEmailRecipient(cfg, category)
		s.False(enabled, "%s", category)
	}
}

func (s *NotifierTestSuite) TestStringAttributeReadsOnlyStrings() {
	attrs := json.RawMessage(`{"email":"a@example.com","n":5,"o":{"x":"y"},"z":null}`)
	s.Equal("a@example.com", stringAttribute(attrs, "email"))
	for _, name := range []string{"n", "o", "z", "missing", ""} {
		s.Empty(stringAttribute(attrs, name), name)
	}
	s.Empty(stringAttribute(nil, "email"))
	s.Empty(stringAttribute(json.RawMessage(`not json`), "email"))
}

// A lock that expires renders the temporary template with its duration and unlock time.
func (s *NotifierTestSuite) TestATemporaryLockFillsItsFacts() {
	for scope, label := range map[providers.AccessScope]string{
		providers.AccessScopeCredential: "Credential sign-in",
		providers.AccessScopeOTP:        "Verification code sign-in",
		AccessScopeEntity:               "Sign-in to your account",
	} {
		handle, data := lockTemplateFor(scope, testLockEpisode())
		s.Equal(lockTemplateHandle, handle)
		s.Equal(map[string]string{
			templateKeyLockedSignIn: label,
			templateKeyLockDuration: "5 minutes",
			templateKeyUnlockTime:   "2026-10-08 10:05 UTC",
		}, data, "scope %s", scope)
	}
}

// The duration is the committed episode's, formatted like other template durations.
func (s *NotifierTestSuite) TestTheDurationIsTheCommittedEpisodes() {
	for duration, want := range map[time.Duration]string{
		15 * time.Minute: "15 minutes", time.Hour: "1 hour", 24 * time.Hour: "24 hours", 90 * time.Second: "90 seconds",
	} {
		_, data := lockTemplateFor(providers.AccessScopeCredential,
			LockEpisode{Duration: duration, UnlockAt: "2026-10-08T10:05:00Z"})
		s.Equal(want, data[templateKeyLockDuration])
	}
}

// A lock that does not expire renders its own template, with no duration or unlock time.
func (s *NotifierTestSuite) TestAPermanentLockRendersItsOwnTemplate() {
	handle, data := lockTemplateFor(providers.AccessScopeCredential, LockEpisode{UnlockAt: PermanentUnlockAt})

	s.Equal(permanentLockTemplateHandle, handle)
	s.Equal("Credential sign-in", data[templateKeyLockedSignIn])
	s.Empty(data[templateKeyLockDuration])
	s.Empty(data[templateKeyUnlockTime])
}

// An unknown scope takes the widest true wording, and every placeholder is filled.
func (s *NotifierTestSuite) TestAnUnknownScopeTakesTheWidestWording() {
	_, data := lockTemplateFor("something-a-later-build-added", testLockEpisode())

	s.Equal("Sign-in to your account", data[templateKeyLockedSignIn])
	s.Len(data, 3)
}
