// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernance

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/notification"
	notifcommon "github.com/thunder-id/thunderid/internal/notification/common"
	notificationconfig "github.com/thunder-id/thunderid/internal/notification/config"
	"github.com/thunder-id/thunderid/internal/notificationtemplate"
	"github.com/thunder-id/thunderid/internal/serverconfig"
	syscontext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/deployment"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// notificationTemplateRenderer resolves a notification template by handle.
type notificationTemplateRenderer interface {
	Resolve(ctx context.Context, channel notificationtemplate.ChannelType, handle string,
		in notificationtemplate.RenderInput) (*notificationtemplate.ResolvedContent, *tidcommon.ServiceError)
}

// LockNotifierInterface sends a notice when an automatic lock forms, from background workers.
type LockNotifierInterface interface {
	// NotifyLockFormed schedules a notice for a committed lock. It must not block or affect the
	// authentication response.
	NotifyLockFormed(ctx context.Context, entity GovernedEntity, method providers.AuthenticationMethod,
		episode LockEpisode)
	// Stop refuses new notices and lets the queued ones finish within the shutdown budget, then
	// records anything abandoned as dropped. It is safe to call more than once.
	Stop()
}

// lockEmailNotifier delivers lock notices as email through the server's default email sender.
type lockEmailNotifier struct {
	source      accountAccessSourceInterface
	profiles    entityStateProvider
	entityTypes entitytype.EntityTypeServiceInterface
	templates   notificationTemplateRenderer
	senders     notification.NotificationSenderServiceInterface
	// serverConfig reads the notification section's default email sender.
	serverConfig serverconfig.ServerConfigService
	queue        chan lockNotice
	timeout      time.Duration
	// ctx is the workers' context, cancelled when Stop runs out of its shutdown budget.
	ctx            context.Context
	cancel         context.CancelFunc
	shutdownBudget time.Duration
	logger         *log.Logger
	workers        sync.WaitGroup
	closeOnce      sync.Once
	mu             sync.RWMutex
	closed         bool
}

// newLockEmailNotifier builds the notifier and starts its workers.
func newLockEmailNotifier(source accountAccessSourceInterface, serverConfig serverconfig.ServerConfigService,
	profiles entityStateProvider, entityTypes entitytype.EntityTypeServiceInterface,
	templates notificationTemplateRenderer, senders notification.NotificationSenderServiceInterface,
) *lockEmailNotifier {
	ctx, cancel := context.WithCancel(context.Background())
	notifier := &lockEmailNotifier{
		source:         source,
		profiles:       profiles,
		entityTypes:    entityTypes,
		templates:      templates,
		senders:        senders,
		serverConfig:   serverConfig,
		queue:          make(chan lockNotice, noticeQueueCapacity),
		timeout:        noticeTimeout,
		ctx:            ctx,
		cancel:         cancel,
		shutdownBudget: noticeShutdownBudget,
		logger:         log.GetLogger().With(log.String(log.LoggerKeyComponentName, "IdentityGovernanceLockNotifier")),
	}

	notifier.workers.Add(noticeWorkers)
	for range noticeWorkers {
		go notifier.run()
	}

	return notifier
}

// NotifyLockFormed queues a notice, or records why it did not. Settings are checked here and again at
// delivery.
func (n *lockEmailNotifier) NotifyLockFormed(ctx context.Context, entity GovernedEntity,
	method providers.AuthenticationMethod, episode LockEpisode) {
	if lockEmailFor(n.source.Current(ctx), entity.Category) == nil {
		n.record(ctx, entity.ID, method, outcomeDisabled)
		return
	}

	n.mu.RLock()
	defer n.mu.RUnlock()
	if n.closed {
		n.record(ctx, entity.ID, method, outcomeClosed)
		return
	}

	select {
	case n.queue <- lockNotice{traceID: syscontext.GetTraceID(ctx), deploymentID: deployment.Resolve(ctx),
		entityID: entity.ID, category: entity.Category, method: method, episode: episode}:
		n.record(ctx, entity.ID, method, outcomeQueued)
	default:
		// Queue full: drop and record, so the failed-login path never blocks.
		n.record(ctx, entity.ID, method, outcomeQueueFull)
	}
}

// Stop implements LockNotifierInterface. It waits up to the shutdown budget for the queue, then
// cancels the sends in flight and records the notices left as dropped.
func (n *lockEmailNotifier) Stop() {
	n.closeOnce.Do(func() {
		n.mu.Lock()
		n.closed = true
		close(n.queue)
		n.mu.Unlock()

		if !n.waitWorkers(n.shutdownBudget) {
			n.cancel()
			n.workers.Wait()
		}
		n.cancel()
	})
}

// waitWorkers reports whether the workers finished within the timeout.
func (n *lockEmailNotifier) waitWorkers(timeout time.Duration) bool {
	done := make(chan struct{})
	go func() {
		n.workers.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}

// run is one worker loop. Once the workers' context is cancelled, a queued notice is recorded as
// dropped instead of delivered.
func (n *lockEmailNotifier) run() {
	defer n.workers.Done()
	for notice := range n.queue {
		if n.ctx.Err() != nil {
			n.record(n.workCtx(notice), notice.entityID, notice.method, outcomeShutdown)
			continue
		}
		n.deliver(notice)
	}
}

// workCtx returns the workers' context carrying the notice's trace and deployment.
func (n *lockEmailNotifier) workCtx(notice lockNotice) context.Context {
	return deployment.WithID(syscontext.WithTraceID(n.ctx, notice.traceID), notice.deploymentID)
}

// deliver makes exactly one delivery attempt, with no retry.
func (n *lockEmailNotifier) deliver(notice lockNotice) {
	ctx, cancel := context.WithTimeout(n.workCtx(notice), n.timeout)
	defer cancel()

	email := lockEmailFor(n.source.Current(ctx), notice.category)
	if email == nil {
		n.record(ctx, notice.entityID, notice.method, outcomeDisabled)
		return
	}
	handle, data := lockTemplateFor(email, notice.method, notice.episode)
	if handle == "" {
		n.record(ctx, notice.entityID, notice.method, outcomeNoTemplate)
		return
	}
	profile, err := n.profiles.GetEntityProfile(ctx, notice.entityID)
	if err != nil {
		n.record(ctx, notice.entityID, notice.method, outcomeReadFailed, log.Error(err))
		return
	}
	attribute, svcErr := n.primaryEmailAttribute(ctx, profile.Type)
	if svcErr != nil {
		n.record(ctx, notice.entityID, notice.method, outcomeReadFailed, log.String("errorCode", svcErr.Code))
		return
	}
	// Only the primary email is used; a type without one, or a user without a value, gets no email.
	address := sysutils.ExtractDisplayValue(profile.Attributes, attribute)
	if address == "" {
		n.record(ctx, notice.entityID, notice.method, outcomeMissingContact)
		return
	}
	if !notifcommon.IsValidEmailAddress(address) {
		n.record(ctx, notice.entityID, notice.method, outcomeInvalidContact)
		return
	}

	senderID, svcErr := n.defaultSenderID(ctx)
	if svcErr != nil {
		n.record(ctx, notice.entityID, notice.method, outcomeNoSender, log.String("errorCode", svcErr.Code))
		return
	}
	if senderID == "" {
		n.record(ctx, notice.entityID, notice.method, outcomeNoSender)
		return
	}

	rendered, svcErr := n.templates.Resolve(ctx, notificationtemplate.ChannelTypeEmail, handle,
		notificationtemplate.RenderInput{Data: data})
	if svcErr != nil {
		n.record(ctx, notice.entityID, notice.method, outcomeTemplateFailed, log.String("errorCode", svcErr.Code))
		return
	}

	if svcErr := n.senders.SendEmail(ctx, senderID, notifcommon.EmailData{
		To:      []string{address},
		Subject: rendered.Subject,
		Body:    rendered.Body,
		IsHTML:  true,
	}); svcErr != nil {
		n.record(ctx, notice.entityID, notice.method, outcomeSendFailed, log.String("errorCode", svcErr.Code))
		return
	}

	// Accepted by the mail server; not proof of receipt.
	n.record(ctx, notice.entityID, notice.method, outcomeAccepted)
}

// record logs one delivery outcome. The entity ID is masked and the address is never logged.
func (n *lockEmailNotifier) record(ctx context.Context, entityID string, method providers.AuthenticationMethod,
	outcome noticeOutcome, extra ...log.Field) {
	fields := append([]log.Field{
		log.MaskedString(log.LoggerKeyEntityID, entityID),
		log.String("authenticationMethod", string(method)),
		log.String("outcome", string(outcome)),
	}, extra...)

	switch outcome {
	case outcomeQueued, outcomeAccepted, outcomeDisabled, outcomeClosed, outcomeMissingContact:
		n.logger.Debug(ctx, "Account lock notification", fields...)
	case outcomeInvalidContact, outcomeNoSender, outcomeNoTemplate, outcomeQueueFull, outcomeShutdown:
		n.logger.Warn(ctx, "Account lock notification not delivered", fields...)
	default:
		n.logger.Error(ctx, "Account lock notification not delivered", fields...)
	}
}

// defaultSenderID reads the default email sender from the notification section, or "" when none is
// set.
func (n *lockEmailNotifier) defaultSenderID(ctx context.Context) (string, *tidcommon.ServiceError) {
	value, svcErr := n.serverConfig.GetMergedConfig(ctx, string(serverconfig.ConfigNameNotification))
	if svcErr != nil {
		return "", svcErr
	}
	section, _ := value.(notificationconfig.NotificationSectionConfig)
	return strings.TrimSpace(section.DefaultEmailSenderID), nil
}

// primaryEmailAttribute returns the user type's primary email attribute, or "" when the type has none.
func (n *lockEmailNotifier) primaryEmailAttribute(ctx context.Context,
	userType string) (string, *tidcommon.ServiceError) {
	if userType == "" {
		return "", nil
	}
	// The notice has no caller, so the type is read with runtime privilege.
	entityType, svcErr := n.entityTypes.GetEntityTypeByHandle(security.WithRuntimeContext(ctx),
		entitytype.TypeCategoryUser, userType)
	if svcErr != nil {
		return "", svcErr
	}
	if entityType == nil || entityType.SystemAttributes == nil {
		return "", nil
	}
	return entityType.SystemAttributes.PrimaryEmail, nil
}
