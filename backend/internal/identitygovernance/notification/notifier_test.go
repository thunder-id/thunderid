// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notification

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	governanceconfig "github.com/thunder-id/thunderid/internal/identitygovernance/config"
	"github.com/thunder-id/thunderid/internal/identitygovernance/model"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	notifcommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/notificationtemplate"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type LockNotifierTestSuite struct {
	suite.Suite

	profiles  *ProfileReaderMock
	templates *TemplateRendererMock
	senders   *EmailSenderMock
	sender    *SenderSourceMock
	sent      chan notifcommon.EmailData
}

// testSenderID is the default email sender the suite configures.
const testSenderID = "sender-1"

func TestLockNotifierTestSuite(t *testing.T) {
	suite.Run(t, new(LockNotifierTestSuite))
}

func (s *LockNotifierTestSuite) SetupTest() {
	s.profiles = NewProfileReaderMock(s.T())
	s.templates = NewTemplateRendererMock(s.T())
	s.senders = NewEmailSenderMock(s.T())
	s.sender = NewSenderSourceMock(s.T())
	s.sender.EXPECT().DefaultEmailSenderID(mock.Anything).Return(testSenderID).Maybe()
	s.sent = make(chan notifcommon.EmailData, 4)
}

// notifierFor builds a notifier over the suite's mocks, reading the given settings.
func (s *LockNotifierTestSuite) notifierFor(settings governanceconfig.AccountAccessLockEmail) *lockEmailNotifier {
	notifier := NewLockEmailNotifier(staticSource{lockEmailConfig(settings)},
		s.profiles, s.templates, s.senders, s.sender)
	s.T().Cleanup(notifier.Close)
	return notifier
}

// expectProfile answers the profile read with the given string attributes.
func (s *LockNotifierTestSuite) expectProfile(entity model.GovernedEntity, attributes map[string]string) {
	raw, err := json.Marshal(attributes)
	s.Require().NoError(err)
	s.profiles.EXPECT().GetEntityProfile(mock.Anything, entity.ID).
		Return(&providers.Entity{ID: entity.ID, Category: entity.Category, Attributes: raw}, nil)
}

// expectRender returns a rendered mail and captures the template data.
func (s *LockNotifierTestSuite) expectRender(captured *map[string]string) {
	s.templates.EXPECT().Resolve(mock.Anything, notificationtemplate.ChannelTypeEmail, lockTemplateHandle,
		mock.Anything).
		RunAndReturn(func(_ context.Context, _ notificationtemplate.ChannelType, _ string,
			in notificationtemplate.RenderInput) (*notificationtemplate.ResolvedContent, *tidcommon.ServiceError) {
			*captured = in.Data
			return &notificationtemplate.ResolvedContent{
				Subject: "Sign-in was locked on your account",
				Body:    "body",
			}, nil
		})
}

// expectSend records what reached the default email sender.
func (s *LockNotifierTestSuite) expectSend(svcErr *tidcommon.ServiceError) {
	s.senders.EXPECT().SendEmail(mock.Anything, testSenderID, mock.Anything).
		RunAndReturn(func(_ context.Context, _ string, data notifcommon.EmailData) *tidcommon.ServiceError {
			s.sent <- data
			return svcErr
		})
}

// assertNothingSent checks the sender was never called.
func (s *LockNotifierTestSuite) assertNothingSent() {
	s.senders.AssertNotCalled(s.T(), "SendEmail", mock.Anything, mock.Anything, mock.Anything)
}

// awaitSend waits for the worker goroutine to send.
func (s *LockNotifierTestSuite) awaitSend() notifcommon.EmailData {
	select {
	case data := <-s.sent:
		return data
	case <-time.After(2 * time.Second):
		s.FailNow("timed out waiting for the lock notification to be sent")
		return notifcommon.EmailData{}
	}
}

func (s *LockNotifierTestSuite) TestSendsToTheDefaultAttribute() {
	entity := testUser()
	s.expectProfile(entity, map[string]string{"email": "member@example.com"})
	var data map[string]string
	s.expectRender(&data)
	s.expectSend(nil)

	notifier := s.notifierFor(governanceconfig.AccountAccessLockEmail{Enabled: boolPtr(true)})
	notifier.NotifyLockFormed(context.Background(), entity, model.AccessScopeCredential, testLockEpisode())

	sent := s.awaitSend()
	s.Equal([]string{"member@example.com"}, sent.To)
	s.True(sent.IsHTML)
}

// The named attribute is used even when an "email" attribute also exists.
func (s *LockNotifierTestSuite) TestSendsToTheNamedAttribute() {
	entity := testUser()
	s.expectProfile(entity, map[string]string{
		"email":     "personal@example.com",
		"workEmail": "work@example.com",
	})
	var data map[string]string
	s.expectRender(&data)
	s.expectSend(nil)

	notifier := s.notifierFor(governanceconfig.AccountAccessLockEmail{
		Enabled: boolPtr(true), RecipientAttribute: stringPtr("workEmail"),
	})
	notifier.NotifyLockFormed(context.Background(), entity, model.AccessScopeCredential, testLockEpisode())

	s.Equal([]string{"work@example.com"}, s.awaitSend().To)
}

// A user missing the named attribute is skipped, even if another address exists.
func (s *LockNotifierTestSuite) TestMissingNamedAttributeNeverFallsBack() {
	entity := testUser()
	s.expectProfile(entity, map[string]string{"email": "personal@example.com"})

	notifier := s.notifierFor(governanceconfig.AccountAccessLockEmail{
		Enabled: boolPtr(true), RecipientAttribute: stringPtr("workEmail"),
	})
	notifier.NotifyLockFormed(context.Background(), entity, model.AccessScopeCredential, testLockEpisode())

	notifier.Close()
	s.assertNothingSent()
}

func (s *LockNotifierTestSuite) TestSkipsAnUnusableAddress() {
	entity := testUser()
	s.expectProfile(entity, map[string]string{"email": "not an address"})

	notifier := s.notifierFor(governanceconfig.AccountAccessLockEmail{Enabled: boolPtr(true)})
	notifier.NotifyLockFormed(context.Background(), entity, model.AccessScopeCredential, testLockEpisode())

	notifier.Close()
	s.assertNothingSent()
}

// Settings are re-read at delivery, so a switch turned off after queuing is honored.
func (s *LockNotifierTestSuite) TestSwitchedOffBeforeDeliveryIsNotSent() {
	entity := testUser()
	source := &switchableSource{cfg: lockEmailConfig(governanceconfig.AccountAccessLockEmail{Enabled: boolPtr(true)})}
	notifier := &lockEmailNotifier{
		section:   source,
		profiles:  s.profiles,
		templates: s.templates,
		senders:   s.senders,
		sender:    s.sender,
		queue:     make(chan lockNotice, 1),
		timeout:   noticeTimeout,
		logger:    log.GetLogger(),
	}

	notifier.NotifyLockFormed(context.Background(), entity, model.AccessScopeCredential, testLockEpisode())
	source.cfg = lockEmailConfig(governanceconfig.AccountAccessLockEmail{})
	notifier.deliver(<-notifier.queue)

	s.profiles.AssertNotCalled(s.T(), "GetEntityProfile", mock.Anything, mock.Anything)
	s.assertNothingSent()
}

// A notice after Close is dropped instead of sending on the closed queue.
func (s *LockNotifierTestSuite) TestANoticeAfterCloseIsDropped() {
	notifier := s.notifierFor(governanceconfig.AccountAccessLockEmail{Enabled: boolPtr(true)})
	notifier.Close()

	s.NotPanics(func() {
		notifier.NotifyLockFormed(context.Background(), testUser(), model.AccessScopeCredential, testLockEpisode())
	})
	s.profiles.AssertNotCalled(s.T(), "GetEntityProfile", mock.Anything, mock.Anything)
	s.assertNothingSent()
}

// When disabled, nothing is queued and no profile is read.
func (s *LockNotifierTestSuite) TestDisabledIsNotEvenQueued() {
	notifier := s.notifierFor(governanceconfig.AccountAccessLockEmail{})
	notifier.NotifyLockFormed(context.Background(), testUser(), model.AccessScopeCredential, testLockEpisode())

	notifier.Close()
	s.profiles.AssertNotCalled(s.T(), "GetEntityProfile", mock.Anything, mock.Anything)
}

// A failed or empty profile read skips the notice without retry.
func (s *LockNotifierTestSuite) TestProfileReadFailureSkipsDelivery() {
	for _, read := range []struct {
		profile *providers.Entity
		err     error
	}{{nil, errors.New("store down")}, {nil, nil}} {
		profiles := NewProfileReaderMock(s.T())
		profiles.EXPECT().GetEntityProfile(mock.Anything, "user-1").Return(read.profile, read.err)
		notifier := NewLockEmailNotifier(staticSource{lockEmailConfig(governanceconfig.AccountAccessLockEmail{
			Enabled: boolPtr(true),
		})}, profiles, s.templates, s.senders, s.sender)

		notifier.NotifyLockFormed(context.Background(), testUser(), model.AccessScopeCredential, testLockEpisode())
		notifier.Close()
	}
	s.assertNothingSent()
}

// A missing template sends nothing.
func (s *LockNotifierTestSuite) TestTemplateFailureSendsNothing() {
	entity := testUser()
	s.expectProfile(entity, map[string]string{"email": "member@example.com"})
	s.templates.EXPECT().Resolve(mock.Anything, notificationtemplate.ChannelTypeEmail, lockTemplateHandle,
		mock.Anything).
		Return(nil, &tidcommon.ServiceError{Code: "NTPL-404"})

	notifier := s.notifierFor(governanceconfig.AccountAccessLockEmail{Enabled: boolPtr(true)})
	notifier.NotifyLockFormed(context.Background(), entity, model.AccessScopeCredential, testLockEpisode())

	notifier.Close()
	s.assertNothingSent()
}

// A refused send is recorded, not retried.
func (s *LockNotifierTestSuite) TestSendFailureIsNotRetried() {
	entity := testUser()
	s.expectProfile(entity, map[string]string{"email": "member@example.com"})
	var data map[string]string
	s.expectRender(&data)
	s.expectSend(&tidcommon.InternalServerError)

	notifier := s.notifierFor(governanceconfig.AccountAccessLockEmail{Enabled: boolPtr(true)})
	notifier.NotifyLockFormed(context.Background(), entity, model.AccessScopeCredential, testLockEpisode())

	s.awaitSend()
	notifier.Close()
	s.senders.AssertNumberOfCalls(s.T(), "SendEmail", 1)
}

// Without a default email sender nothing is rendered or sent.
func (s *LockNotifierTestSuite) TestNoDefaultSenderSendsNothing() {
	entity := testUser()
	s.expectProfile(entity, map[string]string{"email": "member@example.com"})
	sender := NewSenderSourceMock(s.T())
	sender.EXPECT().DefaultEmailSenderID(mock.Anything).Return("")

	notifier := NewLockEmailNotifier(staticSource{lockEmailConfig(governanceconfig.AccountAccessLockEmail{
		Enabled: boolPtr(true),
	})}, s.profiles, s.templates, s.senders, sender)
	notifier.NotifyLockFormed(context.Background(), entity, model.AccessScopeCredential, testLockEpisode())

	notifier.Close()
	s.templates.AssertNotCalled(s.T(), "Resolve", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	s.assertNothingSent()
}

// A full queue drops the notice rather than blocking the caller.
func (s *LockNotifierTestSuite) TestFullQueueDropsTheNotice() {
	notifier := &lockEmailNotifier{
		section:   staticSource{lockEmailConfig(governanceconfig.AccountAccessLockEmail{Enabled: boolPtr(true)})},
		profiles:  s.profiles,
		templates: s.templates,
		senders:   s.senders,
		sender:    s.sender,
		// No workers and a queue of one, so the second notice is dropped.
		queue:   make(chan lockNotice, 1),
		timeout: noticeTimeout,
		logger:  log.GetLogger(),
	}

	notifier.NotifyLockFormed(context.Background(), testUser(), model.AccessScopeCredential, testLockEpisode())
	notifier.NotifyLockFormed(context.Background(), testUser(), model.AccessScopeCredential, testLockEpisode())

	s.Len(notifier.queue, 1)
}

// switchableSource is a source a test can change between queuing and delivery.
type switchableSource struct {
	cfg governanceconfig.AccountAccessValue
}

func (s *switchableSource) Current(context.Context) governanceconfig.AccountAccessValue {
	return s.cfg
}

func (s *LockNotifierTestSuite) TestStringAttributeReadsOnlyStrings() {
	attrs := json.RawMessage(`{"email":"a@example.com","n":5,"o":{"x":"y"},"z":null}`)
	s.Equal("a@example.com", stringAttribute(attrs, "email"))
	for _, name := range []string{"n", "o", "z", "missing"} {
		s.Empty(stringAttribute(attrs, name), name)
	}
	s.Empty(stringAttribute(nil, "email"))
	s.Empty(stringAttribute(json.RawMessage(`not json`), "email"))
}

// Each scope gets its own locked-access text and recovery advice.
func (s *LockNotifierTestSuite) TestCopyIsScopeAccurate() {
	scopes := []struct {
		scope    model.AccessScope
		access   string
		recovery string
	}{
		{model.AccessScopeCredential, "Credential sign-in", "account recovery"},
		{model.AccessScopeOTP, "Verification code sign-in", "account recovery"},
		{model.AccessScopeEntity, "Sign-in to your account", "account recovery"},
	}

	for _, tc := range scopes {
		data := lockTemplateData(tc.scope, testLockEpisode())
		s.Contains(data[templateKeyLockedAccess], tc.access, "scope %s", tc.scope)
		s.Contains(data[templateKeyRecoveryAdvice], tc.recovery, "scope %s", tc.scope)
	}
}

// An unknown scope still fills every placeholder.
func (s *LockNotifierTestSuite) TestCopyIsSuppliedForAnUnknownScope() {
	data := lockTemplateData("something-a-later-build-added", testLockEpisode())

	s.NotEmpty(data[templateKeyLockedAccess])
	s.NotEmpty(data[templateKeyRecoveryAdvice])
	s.Equal("Sign-in to your account has been temporarily locked", data[templateKeyLockedAccess],
		"an unrecognized scope should take the widest true wording, not the narrowest")
}

// Template data carries no identifier or failure count.
func (s *LockNotifierTestSuite) TestTemplateDataCarriesNothingIdentifying() {
	data := lockTemplateData(model.AccessScopeCredential, testLockEpisode())

	s.Len(data, 3)
	s.Contains(data, templateKeyLockedAccess)
	s.Contains(data, templateKeyRecoveryAdvice)
}

func testLockEpisode() model.LockEpisode {
	return model.LockEpisode{Duration: 5 * time.Minute, UnlockAt: "2026-10-08T10:05:00Z", LockCount: 1}
}

func (s *LockNotifierTestSuite) TestLockNoticeDescribesCommittedDuration() {
	for _, duration := range []time.Duration{5 * time.Minute, 15 * time.Minute, time.Hour, 90 * time.Second} {
		data := lockTemplateData(model.AccessScopeCredential,
			model.LockEpisode{Duration: duration, UnlockAt: "2026-10-08T10:05:00Z"})
		s.Contains(data[templateKeyLockDuration], formatLockDuration(duration))
		s.Contains(data[templateKeyLockDuration], "2026-10-08 10:05 UTC")
	}
	for scope, want := range map[model.AccessScope]string{
		model.AccessScopeCredential: "Credential sign-in",
		model.AccessScopeOTP:        "Verification code sign-in",
		model.AccessScopeEntity:     "Sign-in to your account",
	} {
		s.Equal(want+" will be available again in 5 minutes, on 2026-10-08 10:05 UTC.",
			lockTemplateData(scope, testLockEpisode())[templateKeyLockDuration])
	}
	for _, tc := range []struct {
		duration time.Duration
		want     string
	}{
		{5 * time.Minute, "5 minutes"}, {15 * time.Minute, "15 minutes"}, {time.Hour, "1 hour"},
		{90 * time.Second, "1 minute, 30 seconds"}, {24 * time.Hour, "1 day"},
	} {
		s.Equal(tc.want, formatLockDuration(tc.duration))
	}
	data := lockTemplateData(model.AccessScopeCredential, model.LockEpisode{UnlockAt: model.PermanentUnlockAt})
	s.Equal("This lock does not expire automatically.", data[templateKeyLockDuration])
	s.NotContains(data[templateKeyLockDuration], "9999")
	s.Equal("Credential sign-in has been locked", data[templateKeyLockedAccess])
	s.True(strings.HasPrefix(data[templateKeyRecoveryAdvice], "To regain access,"))
}

func (s *LockNotifierTestSuite) TestBlankRecipientSettingSendsToEmail() {
	entity := testUser()
	s.expectProfile(entity, map[string]string{"email": "member@example.com"})
	var data map[string]string
	s.expectRender(&data)
	s.expectSend(nil)
	notifier := s.notifierFor(governanceconfig.AccountAccessLockEmail{
		Enabled: boolPtr(true), RecipientAttribute: stringPtr(""),
	})
	notifier.NotifyLockFormed(context.Background(), entity, model.AccessScopeCredential, testLockEpisode())
	s.Equal([]string{"member@example.com"}, s.awaitSend().To)
	s.Equal("Credential sign-in will be available again in 5 minutes, on 2026-10-08 10:05 UTC.",
		data[templateKeyLockDuration])
}
