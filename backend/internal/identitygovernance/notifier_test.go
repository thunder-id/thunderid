// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entitytype"
	notifcommon "github.com/thunder-id/thunderid/internal/notification/common"
	notificationconfig "github.com/thunder-id/thunderid/internal/notification/config"
	"github.com/thunder-id/thunderid/internal/notificationtemplate"
	"github.com/thunder-id/thunderid/internal/serverconfig"
	syscontext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/deployment"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/notification/notificationmock"
	"github.com/thunder-id/thunderid/tests/mocks/serverconfigmock"
	"github.com/thunder-id/thunderid/tests/testhelpers"
)

type NotifierTestSuite struct {
	suite.Suite

	profiles     *entityStateProviderMock
	entityTypes  *entitytypemock.EntityTypeServiceInterfaceMock
	templates    *notificationTemplateRendererMock
	senders      *notificationmock.NotificationSenderServiceInterfaceMock
	serverConfig *serverconfigmock.ServerConfigServiceMock
	sent         chan notifcommon.EmailData
}

// testSenderID is the default email sender the suite configures.
const testSenderID = "sender-1"

func TestNotifierTestSuite(t *testing.T) {
	suite.Run(t, new(NotifierTestSuite))
}

func (s *NotifierTestSuite) SetupTest() {
	s.profiles = newEntityStateProviderMock(s.T())
	// The test user's type maps its primary email to email.
	s.entityTypes = entitytypemock.NewEntityTypeServiceInterfaceMock(s.T())
	s.entityTypes.EXPECT().GetEntityTypeByHandle(mock.MatchedBy(security.IsRuntimeContext),
		entitytype.TypeCategoryUser, testUser().Type).
		Return(&entitytype.EntityType{Handle: testUser().Type,
			SystemAttributes: &entitytype.SystemAttributes{PrimaryEmail: "email"}}, nil).Maybe()
	s.templates = newNotificationTemplateRendererMock(s.T())
	s.senders = notificationmock.NewNotificationSenderServiceInterfaceMock(s.T())
	// The notification section names the default email sender.
	s.serverConfig = serverconfigmock.NewServerConfigServiceMock(s.T())
	s.serverConfig.EXPECT().GetMergedConfig(mock.Anything, string(serverconfig.ConfigNameNotification)).
		Return(notificationconfig.NotificationSectionConfig{DefaultEmailSenderID: testSenderID}, nil).Maybe()
	s.sent = make(chan notifcommon.EmailData, 4)
}

func testUser() GovernedEntity {
	return GovernedEntity{ID: "user-1", Category: providers.EntityCategoryUser, Type: "member"}
}

func testLockEpisode() LockEpisode {
	return LockEpisode{Duration: 5 * time.Minute, UnlockAt: "2026-10-08T10:05:00Z", LockCount: 1}
}

// testLockTemplate is the expiring lock template the enabled lock email names.
const testLockTemplate = "lock-expiring"

// enabledLockEmail is a switched-on lock email with both templates set.
func enabledLockEmail() accountAccessLockEmail {
	return accountAccessLockEmail{Enabled: new(true), Templates: &accountAccessLockTemplates{
		lockEmailTemplatePair: lockEmailTemplatePair{Temporary: testLockTemplate, Permanent: "lock-permanent"}}}
}

func lockEmailConfig(email accountAccessLockEmail) accountAccessValue {
	return accountAccessValue{User: &accountAccessCategory{
		Notifications: &accountAccessNotifications{
			OnLock: &accountAccessLockNotification{Email: &email},
		},
	}}
}

// settingsSource is a section source that serves the given lock email settings.
func (s *NotifierTestSuite) settingsSource(settings accountAccessLockEmail) accountAccessSourceInterface {
	source := newAccountAccessSourceInterfaceMock(s.T())
	source.EXPECT().Current(mock.Anything).Return(lockEmailConfig(settings)).Maybe()
	return source
}

// newNotifier builds a notifier over the suite's mocks, reading the given settings through the source.
func (s *NotifierTestSuite) newNotifier(settings accountAccessLockEmail) *lockEmailNotifier {
	return newLockEmailNotifier(s.settingsSource(settings), s.serverConfig, s.profiles, s.entityTypes,
		s.templates, s.senders)
}

// notifierFor builds a notifier with newNotifier and closes it when the test ends.
func (s *NotifierTestSuite) notifierFor(settings accountAccessLockEmail) *lockEmailNotifier {
	notifier := s.newNotifier(settings)
	s.T().Cleanup(notifier.Stop)
	return notifier
}

// expectProfile answers the profile read with the given string attributes.
func (s *NotifierTestSuite) expectProfile(entity GovernedEntity, attributes map[string]string) {
	raw, err := json.Marshal(attributes)
	s.Require().NoError(err)
	s.expectProfileJSON(entity, string(raw))
}

// expectProfileJSON answers the profile read with the given attributes document.
func (s *NotifierTestSuite) expectProfileJSON(entity GovernedEntity, attributes string) {
	s.profiles.EXPECT().GetEntityProfile(mock.Anything, entity.ID).Return(
		&providers.Entity{ID: entity.ID, Category: entity.Category, Type: entity.Type,
			Attributes: json.RawMessage(attributes)}, nil)
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

// The lock email is sent as HTML to the primary email, with the configured template and the method.
func (s *NotifierTestSuite) TestSendsTheConfiguredTemplateToThePrimaryEmail() {
	entity := testUser()
	s.expectProfile(entity, map[string]string{"email": "member@example.com"})
	var handle string
	var data map[string]string
	s.expectRender(&handle, &data)
	s.expectSend(nil)

	notifier := s.notifierFor(enabledLockEmail())
	notifier.NotifyLockFormed(context.Background(), entity, providers.AuthenticationMethodCredential, testLockEpisode())

	sent := s.awaitSend()
	s.Equal([]string{"member@example.com"}, sent.To)
	s.True(sent.IsHTML)
	s.Equal(testLockTemplate, handle)
	s.Equal(string(providers.AuthenticationMethodCredential), data[templateKeyLockedAuthenticationMethod])
}

// deliveryContext is what one delivery step saw of its context, read while the step ran.
type deliveryContext struct {
	traceID, deploymentID string
	callerValue           any
	err                   error
	bounded               bool
}

// callerKey is a context key only the failed sign-in's context carries.
type callerKey struct{}

func observedContext(ctx context.Context) deliveryContext {
	_, bounded := ctx.Deadline()
	return deliveryContext{traceID: syscontext.GetTraceID(ctx), deploymentID: deployment.Resolve(ctx),
		callerValue: ctx.Value(callerKey{}), err: ctx.Err(), bounded: bounded}
}

// Every delivery step runs under the failed sign-in's trace ID and deployment, bounded by the delivery
// timeout, and under none of the caller's other values or its cancellation.
func (s *NotifierTestSuite) TestDeliveryKeepsOnlyTheTraceAndDeploymentOfTheRequest() {
	entity := testUser()
	seen := make(chan deliveryContext, 3)
	s.profiles.EXPECT().GetEntityProfile(mock.Anything, entity.ID).RunAndReturn(
		func(ctx context.Context, _ string) (*providers.Entity, error) {
			seen <- observedContext(ctx)
			return &providers.Entity{ID: entity.ID, Type: entity.Type,
				Attributes: json.RawMessage(`{"email":"member@example.com"}`)}, nil
		})
	s.templates.EXPECT().
		Resolve(mock.Anything, notificationtemplate.ChannelTypeEmail,
			testLockTemplate, mock.Anything).
		RunAndReturn(func(ctx context.Context, _ notificationtemplate.ChannelType, _ string,
			_ notificationtemplate.RenderInput) (*notificationtemplate.ResolvedContent, *tidcommon.ServiceError) {
			seen <- observedContext(ctx)
			return &notificationtemplate.ResolvedContent{Subject: "Sign-in was locked", Body: "body"}, nil
		})
	delivered := make(chan struct{})
	s.senders.EXPECT().SendEmail(mock.Anything, testSenderID, mock.Anything).
		RunAndReturn(func(ctx context.Context, _ string, _ notifcommon.EmailData) *tidcommon.ServiceError {
			seen <- observedContext(ctx)
			close(delivered)
			return nil
		})
	notifier := s.notifierFor(enabledLockEmail())
	reqCtx, cancel := context.WithCancel(context.WithValue(
		deployment.WithID(syscontext.WithTraceID(context.Background(), "trace-1"), "deployment-1"),
		callerKey{}, "caller only"))
	cancel()

	notifier.NotifyLockFormed(reqCtx, entity, providers.AuthenticationMethodCredential, testLockEpisode())

	select {
	case <-delivered:
	case <-time.After(2 * time.Second):
		s.FailNow("timed out waiting for the lock notification to be sent")
	}
	for _, step := range []string{"profile read", "render", "send"} {
		s.Equal(deliveryContext{traceID: "trace-1", deploymentID: "deployment-1", bounded: true}, <-seen, step)
	}
}

// A primary email value that is not a usable address is skipped and recorded, never sent and never
// logged. A value that is not text is a missing contact; text or a number that is not an address is an
// invalid one.
func (s *NotifierTestSuite) TestAnUnusableRecipientIsNotSent() {
	for name, tc := range map[string]struct {
		profile string
		outcome noticeOutcome
	}{
		"text that is not an address": {`{"email":"not an address"}`, outcomeInvalidContact},
		"a number":                    {`{"email":5550142}`, outcomeInvalidContact},
		"an object":                   {`{"email":{"value":"a@example.com"}}`, outcomeMissingContact},
		"a boolean":                   {`{"email":true}`, outcomeMissingContact},
		"null":                        {`{"email":null}`, outcomeMissingContact},
	} {
		s.Run(name, func() {
			s.SetupTest()
			entity := testUser()
			s.expectProfileJSON(entity, tc.profile)
			notifier := s.newNotifier(enabledLockEmail())

			logs := testhelpers.CaptureLogs(s.T(), "debug", func() {
				notifier.NotifyLockFormed(context.Background(), entity, providers.AuthenticationMethodCredential,
					testLockEpisode())
				notifier.Stop()
			})

			s.Contains(logs, "outcome="+string(tc.outcome))
			s.NotContains(logs, "not an address")
			s.NotContains(logs, "5550142")
			s.assertNothingSent()
		})
	}
}

// Settings are re-read at delivery, so a switch turned off after queuing is honored.
func (s *NotifierTestSuite) TestSwitchedOffBeforeDeliveryIsNotSent() {
	source := newAccountAccessSourceInterfaceMock(s.T())
	source.EXPECT().Current(mock.Anything).
		Return(lockEmailConfig(enabledLockEmail())).Once()
	source.EXPECT().Current(mock.Anything).Return(lockEmailConfig(accountAccessLockEmail{})).Once()
	notifier := &lockEmailNotifier{
		source:       source,
		profiles:     s.profiles,
		templates:    s.templates,
		senders:      s.senders,
		serverConfig: s.serverConfig,
		queue:        make(chan lockNotice, 1),
		timeout:      noticeTimeout,
		ctx:          context.Background(),
		logger:       log.GetLogger(),
	}

	notifier.NotifyLockFormed(context.Background(), testUser(), providers.AuthenticationMethodCredential,
		testLockEpisode())
	notifier.deliver(<-notifier.queue)

	s.profiles.AssertNotCalled(s.T(), "GetEntityProfile", mock.Anything, mock.Anything)
	s.assertNothingSent()
}

// A notice after Stop is dropped instead of sending on the closed queue.
func (s *NotifierTestSuite) TestANoticeAfterStopIsDropped() {
	notifier := s.notifierFor(enabledLockEmail())
	notifier.Stop()

	s.NotPanics(func() {
		notifier.NotifyLockFormed(context.Background(), testUser(), providers.AuthenticationMethodCredential,
			testLockEpisode())
	})
	s.profiles.AssertNotCalled(s.T(), "GetEntityProfile", mock.Anything, mock.Anything)
	s.assertNothingSent()
}

// Stop waits only its budget for sends that do not return. It then cancels them, records the notice
// still queued as dropped instead of delivering it, and drops a notice that comes after.
func (s *NotifierTestSuite) TestStopIsBoundedWhenSendsBlock() {
	s.profiles.EXPECT().GetEntityProfile(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, id string) (*providers.Entity, error) {
			return &providers.Entity{ID: id, Type: testUser().Type,
				Attributes: json.RawMessage(`{"email":"member@example.com"}`)}, nil
		})
	s.templates.EXPECT().Resolve(mock.Anything, notificationtemplate.ChannelTypeEmail, mock.Anything, mock.Anything).
		Return(&notificationtemplate.ResolvedContent{Subject: "Sign-in was locked", Body: "body"}, nil)
	sending := make(chan struct{}, noticeWorkers+1)
	s.senders.EXPECT().SendEmail(mock.Anything, testSenderID, mock.Anything).
		RunAndReturn(func(ctx context.Context, _ string, _ notifcommon.EmailData) *tidcommon.ServiceError {
			sending <- struct{}{}
			<-ctx.Done()
			return &tidcommon.InternalServerError
		})
	// Not closed on cleanup: if Stop hangs, a second Stop would wait on the first.
	notifier := s.newNotifier(enabledLockEmail())
	notifier.shutdownBudget = 100 * time.Millisecond

	// Every worker blocks in a send, and one more notice waits in the queue behind them.
	for i := range noticeWorkers + 1 {
		user := testUser()
		user.ID = fmt.Sprintf("user-%d", i)
		notifier.NotifyLockFormed(context.Background(), user, providers.AuthenticationMethodCredential,
			testLockEpisode())
	}
	for range noticeWorkers {
		select {
		case <-sending:
		case <-time.After(2 * time.Second):
			s.FailNow("timed out waiting for the workers to start sending")
		}
	}

	logs := testhelpers.CaptureLogs(s.T(), "debug", func() {
		closed := make(chan struct{})
		go func() {
			notifier.Stop()
			close(closed)
		}()
		select {
		case <-closed:
		case <-time.After(5 * time.Second):
			s.FailNow("Stop did not return after its shutdown budget")
		}
		notifier.NotifyLockFormed(context.Background(), testUser(), providers.AuthenticationMethodCredential,
			testLockEpisode())
	})

	s.profiles.AssertNumberOfCalls(s.T(), "GetEntityProfile", noticeWorkers)
	s.senders.AssertNumberOfCalls(s.T(), "SendEmail", noticeWorkers)
	s.Equal(noticeWorkers, strings.Count(logs, "outcome="+string(outcomeSendFailed)), "the cancelled sends")
	s.Equal(1, strings.Count(logs, "outcome="+string(outcomeShutdown)), "the queued notice")
	s.Equal(1, strings.Count(logs, "outcome="+string(outcomeClosed)), "the notice after Stop")
}

// When disabled, nothing is queued and no profile is read.
func (s *NotifierTestSuite) TestDisabledIsNotEvenQueued() {
	notifier := s.notifierFor(accountAccessLockEmail{})
	notifier.NotifyLockFormed(context.Background(), testUser(), providers.AuthenticationMethodCredential,
		testLockEpisode())

	notifier.Stop()
	s.profiles.AssertNotCalled(s.T(), "GetEntityProfile", mock.Anything, mock.Anything)
}

// A failed profile read skips the notice without retry, and the error is logged with the outcome.
func (s *NotifierTestSuite) TestProfileReadFailureSkipsDelivery() {
	s.profiles.EXPECT().GetEntityProfile(mock.Anything, "user-1").Return(nil, errors.New("store down")).Once()
	notifier := s.newNotifier(enabledLockEmail())

	logs := testhelpers.CaptureLogs(s.T(), "info", func() {
		notifier.NotifyLockFormed(context.Background(), testUser(), providers.AuthenticationMethodCredential,
			testLockEpisode())
		notifier.Stop()
	})

	s.Contains(logs, "outcome="+string(outcomeReadFailed))
	s.Contains(logs, "store down")
	s.assertNothingSent()
}

// A missing template sends nothing.
func (s *NotifierTestSuite) TestTemplateFailureSendsNothing() {
	entity := testUser()
	s.expectProfile(entity, map[string]string{"email": "member@example.com"})
	s.templates.EXPECT().Resolve(mock.Anything, notificationtemplate.ChannelTypeEmail,
		testLockTemplate,
		mock.Anything).Return(nil, &tidcommon.ServiceError{Code: "NTPL-404"})

	notifier := s.notifierFor(enabledLockEmail())
	notifier.NotifyLockFormed(context.Background(), entity, providers.AuthenticationMethodCredential, testLockEpisode())

	notifier.Stop()
	s.assertNothingSent()
}

// With no template set for the lock, nothing is read, rendered or sent, and the outcome is recorded.
func (s *NotifierTestSuite) TestNoTemplateSendsNothing() {
	notifier := s.newNotifier(accountAccessLockEmail{Enabled: new(true), Templates: &accountAccessLockTemplates{
		lockEmailTemplatePair: lockEmailTemplatePair{Permanent: "lock-permanent"}}})

	logs := testhelpers.CaptureLogs(s.T(), "info", func() {
		notifier.NotifyLockFormed(context.Background(), testUser(), providers.AuthenticationMethodCredential,
			testLockEpisode())
		notifier.Stop()
	})

	s.Contains(logs, "outcome="+string(outcomeNoTemplate))
	s.profiles.AssertNotCalled(s.T(), "GetEntityProfile", mock.Anything, mock.Anything)
	s.templates.AssertNotCalled(s.T(), "Resolve", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
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

	notifier := s.notifierFor(enabledLockEmail())
	notifier.NotifyLockFormed(context.Background(), entity, providers.AuthenticationMethodCredential, testLockEpisode())

	s.awaitSend()
	notifier.Stop()
	s.senders.AssertNumberOfCalls(s.T(), "SendEmail", 1)
}

// Without a default email sender nothing is rendered or sent. A blank ID names none.
func (s *NotifierTestSuite) TestNoDefaultSenderSendsNothing() {
	entity := testUser()
	s.expectProfile(entity, map[string]string{"email": "member@example.com"})
	s.serverConfig = serverconfigmock.NewServerConfigServiceMock(s.T())
	s.serverConfig.EXPECT().GetMergedConfig(mock.Anything, string(serverconfig.ConfigNameNotification)).
		Return(notificationconfig.NotificationSectionConfig{DefaultEmailSenderID: "  "}, nil).Once()

	notifier := s.newNotifier(enabledLockEmail())
	notifier.NotifyLockFormed(context.Background(), entity, providers.AuthenticationMethodCredential, testLockEpisode())

	notifier.Stop()
	s.templates.AssertNotCalled(s.T(), "Resolve", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	s.assertNothingSent()
}

// A failed read of the notification section names no sender, and the skip carries the error code.
func (s *NotifierTestSuite) TestAnUnreadableSenderSectionSendsNothing() {
	entity := testUser()
	s.expectProfile(entity, map[string]string{"email": "member@example.com"})
	s.serverConfig = serverconfigmock.NewServerConfigServiceMock(s.T())
	s.serverConfig.EXPECT().GetMergedConfig(mock.Anything, string(serverconfig.ConfigNameNotification)).
		Return(nil, &tidcommon.InternalServerError).Once()
	notifier := s.newNotifier(enabledLockEmail())

	logs := testhelpers.CaptureLogs(s.T(), "info", func() {
		notifier.NotifyLockFormed(context.Background(), entity, providers.AuthenticationMethodCredential,
			testLockEpisode())
		notifier.Stop()
	})

	s.Contains(logs, "outcome="+string(outcomeNoSender))
	s.Contains(logs, "errorCode="+tidcommon.InternalServerError.Code)
	s.assertNothingSent()
}

// The default sender is read from the notification section through the server-config service.
func (s *NotifierTestSuite) TestTheDefaultSenderIsReadThroughTheServerConfigService() {
	entity := testUser()
	s.expectProfile(entity, map[string]string{"email": "member@example.com"})
	var handle string
	var data map[string]string
	s.expectRender(&handle, &data)
	s.expectSend(nil)
	s.serverConfig = serverconfigmock.NewServerConfigServiceMock(s.T())
	s.serverConfig.EXPECT().GetMergedConfig(mock.Anything, string(serverconfig.ConfigNameNotification)).
		Return(notificationconfig.NotificationSectionConfig{DefaultEmailSenderID: testSenderID}, nil).Once()

	notifier := s.notifierFor(enabledLockEmail())
	notifier.NotifyLockFormed(context.Background(), entity, providers.AuthenticationMethodCredential, testLockEpisode())

	s.Equal([]string{"member@example.com"}, s.awaitSend().To)
}

// A full queue drops the notice rather than blocking the caller.
func (s *NotifierTestSuite) TestFullQueueDropsTheNotice() {
	notifier := &lockEmailNotifier{
		source:       s.settingsSource(enabledLockEmail()),
		profiles:     s.profiles,
		templates:    s.templates,
		senders:      s.senders,
		serverConfig: s.serverConfig,
		// No workers and a queue of one, so the second notice is dropped.
		queue:   make(chan lockNotice, 1),
		timeout: noticeTimeout,
		logger:  log.GetLogger(),
	}

	notifier.NotifyLockFormed(context.Background(), testUser(), providers.AuthenticationMethodCredential,
		testLockEpisode())
	notifier.NotifyLockFormed(context.Background(), testUser(), providers.AuthenticationMethodCredential,
		testLockEpisode())

	s.Len(notifier.queue, 1)
}

// staffUser is a user whose type maps its primary email to workEmail.
func staffUser() GovernedEntity {
	return GovernedEntity{ID: "user-2", Category: providers.EntityCategoryUser, Type: "staff"}
}

// expectStaffType answers the staff type read, made with runtime privilege, with the given system
// attributes.
func (s *NotifierTestSuite) expectStaffType(attributes *entitytype.SystemAttributes) {
	s.entityTypes.EXPECT().GetEntityTypeByHandle(mock.Anything, entitytype.TypeCategoryUser, "staff").
		RunAndReturn(func(ctx context.Context, _ entitytype.TypeCategory,
			handle string) (*entitytype.EntityType, *tidcommon.ServiceError) {
			s.True(security.IsRuntimeContext(ctx))
			return &entitytype.EntityType{Handle: handle, SystemAttributes: attributes}, nil
		})
}

// The user type's primary email is used even when an "email" attribute also exists.
func (s *NotifierTestSuite) TestSendsToThePrimaryEmail() {
	entity := staffUser()
	s.expectStaffType(&entitytype.SystemAttributes{PrimaryEmail: "workEmail"})
	s.expectProfile(entity, map[string]string{
		"email":     "personal@example.com",
		"workEmail": "work@example.com",
	})
	var handle string
	var data map[string]string
	s.expectRender(&handle, &data)
	s.expectSend(nil)

	notifier := s.notifierFor(enabledLockEmail())
	notifier.NotifyLockFormed(context.Background(), entity, providers.AuthenticationMethodCredential, testLockEpisode())

	s.Equal([]string{"work@example.com"}, s.awaitSend().To)
}

// A type without a primary email, or a user without a value for it, gets no email, and the skip is
// recorded. No other attribute is tried.
func (s *NotifierTestSuite) TestNoPrimaryEmailSendsNothing() {
	for name, tc := range map[string]struct {
		attributes *entitytype.SystemAttributes
		profile    map[string]string
	}{
		"a type with no system attributes": {nil, map[string]string{"email": "personal@example.com"}},
		"a type with no primary email": {&entitytype.SystemAttributes{Display: "email"},
			map[string]string{"email": "personal@example.com"}},
		"no value": {&entitytype.SystemAttributes{PrimaryEmail: "workEmail"},
			map[string]string{"email": "personal@example.com"}},
		"an empty value": {&entitytype.SystemAttributes{PrimaryEmail: "workEmail"},
			map[string]string{"email": "personal@example.com", "workEmail": ""}},
	} {
		s.Run(name, func() {
			s.SetupTest()
			entity := staffUser()
			s.expectStaffType(tc.attributes)
			s.expectProfile(entity, tc.profile)
			notifier := s.newNotifier(enabledLockEmail())

			logs := testhelpers.CaptureLogs(s.T(), "debug", func() {
				notifier.NotifyLockFormed(context.Background(), entity, providers.AuthenticationMethodCredential,
					testLockEpisode())
				notifier.Stop()
			})

			s.Contains(logs, "outcome="+string(outcomeMissingContact))
			s.NotContains(logs, "personal@example.com")
			s.assertNothingSent()
		})
	}
}

// A profile with no type has no primary email, so no type is read and nothing is sent.
func (s *NotifierTestSuite) TestAProfileWithoutATypeSendsNothing() {
	entity := GovernedEntity{ID: "user-3", Category: providers.EntityCategoryUser}
	s.expectProfile(entity, map[string]string{"email": "personal@example.com"})
	notifier := s.newNotifier(enabledLockEmail())

	logs := testhelpers.CaptureLogs(s.T(), "debug", func() {
		notifier.NotifyLockFormed(context.Background(), entity, providers.AuthenticationMethodCredential,
			testLockEpisode())
		notifier.Stop()
	})

	s.Contains(logs, "outcome="+string(outcomeMissingContact))
	s.entityTypes.AssertNotCalled(s.T(), "GetEntityTypeByHandle", mock.Anything, mock.Anything, mock.Anything)
	s.assertNothingSent()
}

// A failed type read sends nothing and is recorded with its error code.
func (s *NotifierTestSuite) TestAnUnreadableTypeSendsNothing() {
	entity := staffUser()
	s.entityTypes.EXPECT().GetEntityTypeByHandle(mock.Anything, entitytype.TypeCategoryUser, "staff").
		Return(nil, &tidcommon.InternalServerError)
	s.expectProfile(entity, map[string]string{"email": "personal@example.com", "workEmail": "work@example.com"})
	notifier := s.newNotifier(enabledLockEmail())

	logs := testhelpers.CaptureLogs(s.T(), "info", func() {
		notifier.NotifyLockFormed(context.Background(), entity, providers.AuthenticationMethodCredential,
			testLockEpisode())
		notifier.Stop()
	})

	s.Contains(logs, "outcome="+string(outcomeReadFailed))
	s.Contains(logs, "errorCode="+tidcommon.InternalServerError.Code)
	s.assertNothingSent()
}

// An unusable primary email is skipped without trying another attribute.
func (s *NotifierTestSuite) TestAnUnusablePrimaryEmailNeverFallsBack() {
	entity := staffUser()
	s.expectStaffType(&entitytype.SystemAttributes{PrimaryEmail: "workEmail"})
	s.expectProfile(entity, map[string]string{"email": "personal@example.com", "workEmail": "not an address"})

	notifier := s.notifierFor(enabledLockEmail())
	notifier.NotifyLockFormed(context.Background(), entity, providers.AuthenticationMethodCredential, testLockEpisode())

	notifier.Stop()
	s.assertNothingSent()
}
