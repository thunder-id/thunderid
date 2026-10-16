// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernance

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/thunder-id/thunderid/internal/notification"
	notifcommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/notificationtemplate"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// notificationTemplateRenderer resolves a notification template by handle.
type notificationTemplateRenderer interface {
	Resolve(ctx context.Context, channel notificationtemplate.ChannelType, handle string,
		in notificationtemplate.RenderInput) (*notificationtemplate.ResolvedContent, *tidcommon.ServiceError)
}

// lockNotifierInterface sends a notice when an automatic lock forms.
type lockNotifierInterface interface {
	// NotifyLockFormed schedules a notice for a committed lock. It must not block or affect the
	// authentication response.
	NotifyLockFormed(ctx context.Context, entity GovernedEntity, scope providers.AccessScope, episode LockEpisode)
	// Close stops accepting notices and waits for the queued ones to be delivered.
	Close()
}

// lockedSignInLabels names, per scope, the sign-in that stopped working.
var lockedSignInLabels = map[providers.AccessScope]string{
	providers.AccessScopeCredential: "Credential sign-in",
	providers.AccessScopeOTP:        "Verification code sign-in",
	AccessScopeEntity:               "Sign-in to your account",
}

// lockEmailNotifier delivers lock notices as email through the server's default email sender.
type lockEmailNotifier struct {
	source    accountAccessSourceInterface
	profiles  EntityStateProvider
	templates notificationTemplateRenderer
	senders   notification.NotificationSenderServiceInterface
	sender    notification.DefaultEmailSenderInterface
	queue     chan lockNotice
	timeout   time.Duration
	logger    *log.Logger
	workers   sync.WaitGroup
	closeOnce sync.Once
	mu        sync.RWMutex
	closed    bool
}

// newLockEmailNotifier builds the notifier and starts its workers.
func newLockEmailNotifier(source accountAccessSourceInterface, profiles EntityStateProvider,
	templates notificationTemplateRenderer, senders notification.NotificationSenderServiceInterface,
	sender notification.DefaultEmailSenderInterface) *lockEmailNotifier {
	notifier := &lockEmailNotifier{
		source:    source,
		profiles:  profiles,
		templates: templates,
		senders:   senders,
		sender:    sender,
		queue:     make(chan lockNotice, noticeQueueCapacity),
		timeout:   noticeTimeout,
		logger:    log.GetLogger().With(log.String(log.LoggerKeyComponentName, "IdentityGovernanceLockNotifier")),
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
	scope providers.AccessScope, episode LockEpisode) {
	if _, enabled := lockEmailRecipient(n.source.Current(ctx), entity.Category); !enabled {
		n.record(ctx, entity.ID, scope, outcomeDisabled)
		return
	}

	n.mu.RLock()
	defer n.mu.RUnlock()
	if n.closed {
		n.record(ctx, entity.ID, scope, outcomeClosed)
		return
	}

	select {
	case n.queue <- lockNotice{ctx: context.WithoutCancel(ctx), entityID: entity.ID, category: entity.Category,
		scope: scope, episode: episode}:
		n.record(ctx, entity.ID, scope, outcomeQueued)
	default:
		// Queue full: drop and record, so the failed-login path never blocks.
		n.record(ctx, entity.ID, scope, outcomeQueueFull)
	}
}

// Close closes the queue and waits for the workers to drain it. A notice after Close is dropped.
func (n *lockEmailNotifier) Close() {
	n.closeOnce.Do(func() {
		n.mu.Lock()
		n.closed = true
		close(n.queue)
		n.mu.Unlock()
		n.workers.Wait()
	})
}

// run is one worker loop.
func (n *lockEmailNotifier) run() {
	defer n.workers.Done()
	for notice := range n.queue {
		n.deliver(notice)
	}
}

// deliver makes exactly one delivery attempt, with no retry.
func (n *lockEmailNotifier) deliver(notice lockNotice) {
	// The request's context is cancelled by now, so the notice carries it detached, bounded here.
	ctx, cancel := context.WithTimeout(notice.ctx, n.timeout)
	defer cancel()

	attribute, enabled := lockEmailRecipient(n.source.Current(ctx), notice.category)
	if !enabled {
		n.record(ctx, notice.entityID, notice.scope, outcomeDisabled)
		return
	}
	profile, err := n.profiles.GetEntityProfile(ctx, notice.entityID)
	if err != nil || profile == nil {
		n.record(ctx, notice.entityID, notice.scope, outcomeReadFailed)
		return
	}
	address := stringAttribute(profile.Attributes, attribute)
	if address == "" {
		// Attribute missing: skip, never fall back to another attribute.
		n.record(ctx, notice.entityID, notice.scope, outcomeMissingContact)
		return
	}
	if !notifcommon.IsValidEmailAddress(address) {
		n.record(ctx, notice.entityID, notice.scope, outcomeInvalidContact)
		return
	}

	senderID := n.sender.DefaultEmailSenderID(ctx)
	if senderID == "" {
		n.record(ctx, notice.entityID, notice.scope, outcomeNoSender)
		return
	}

	handle, data := lockTemplateFor(notice.scope, notice.episode)
	rendered, svcErr := n.templates.Resolve(ctx, notificationtemplate.ChannelTypeEmail, handle,
		notificationtemplate.RenderInput{Data: data})
	if svcErr != nil {
		n.record(ctx, notice.entityID, notice.scope, outcomeTemplateFailed, log.String("errorCode", svcErr.Code))
		return
	}

	if svcErr := n.senders.SendEmail(ctx, senderID, notifcommon.EmailData{
		To:      []string{address},
		Subject: rendered.Subject,
		Body:    rendered.Body,
		IsHTML:  true,
	}); svcErr != nil {
		n.record(ctx, notice.entityID, notice.scope, outcomeSendFailed, log.String("errorCode", svcErr.Code))
		return
	}

	// Accepted by the mail server; not proof of receipt.
	n.record(ctx, notice.entityID, notice.scope, outcomeAccepted)
}

// record logs one delivery outcome. The entity ID is masked and the address is never logged.
func (n *lockEmailNotifier) record(ctx context.Context, entityID string, scope providers.AccessScope, outcome string,
	extra ...log.Field) {
	fields := append([]log.Field{
		log.MaskedString("entityId", entityID),
		log.String("scope", string(scope)),
		log.String("outcome", outcome),
	}, extra...)

	switch outcome {
	case outcomeQueued, outcomeAccepted, outcomeDisabled, outcomeClosed, outcomeMissingContact:
		n.logger.Debug(ctx, "Account lock notification", fields...)
	case outcomeInvalidContact, outcomeNoSender, outcomeQueueFull:
		n.logger.Warn(ctx, "Account lock notification not delivered", fields...)
	default:
		n.logger.Error(ctx, "Account lock notification not delivered", fields...)
	}
}

// lockEmailRecipient returns the profile attribute a lock email for this category is sent to, and
// false when the category is not notified. Only users are notified.
func lockEmailRecipient(cfg accountAccessValue, category providers.EntityCategory) (string, bool) {
	if category != providers.EntityCategoryUser || cfg.User == nil {
		return "", false
	}
	email := cfg.User.Notifications.LockEmail()
	if email == nil || !derefOr(email.Enabled, false) {
		return "", false
	}
	return strings.TrimSpace(derefOr(email.RecipientAttribute, "")), true
}

// stringAttribute returns a top-level string attribute, or "" if it is missing or not a string.
func stringAttribute(attributes json.RawMessage, name string) string {
	var top map[string]json.RawMessage
	if name == "" || len(attributes) == 0 || json.Unmarshal(attributes, &top) != nil {
		return ""
	}
	var value string
	if json.Unmarshal(top[name], &value) != nil {
		return ""
	}
	return value
}

// lockTemplateFor picks the template for a lock and the facts it fills: no address, identifier or
// failure count. A lock that does not expire has its own template, so each template's copy is fixed.
func lockTemplateFor(scope providers.AccessScope, episode LockEpisode) (string, map[string]string) {
	lockedSignIn, ok := lockedSignInLabels[scope]
	if !ok {
		lockedSignIn = lockedSignInLabels[AccessScopeEntity]
	}
	data := map[string]string{
		templateKeyLockedSignIn: lockedSignIn,
		templateKeyLockDuration: "",
		templateKeyUnlockTime:   "",
	}

	ends, err := time.Parse(time.RFC3339, episode.UnlockAt)
	if episode.UnlockAt == PermanentUnlockAt || episode.Duration <= 0 || err != nil {
		return permanentLockTemplateHandle, data
	}
	data[templateKeyLockDuration] = sysutils.FormatExpiryDuration(int64(episode.Duration / time.Second))
	data[templateKeyUnlockTime] = ends.UTC().Format(unlockTimeLayout)
	return lockTemplateHandle, data
}
