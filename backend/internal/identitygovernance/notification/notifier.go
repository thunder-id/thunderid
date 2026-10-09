// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/thunder-id/thunderid/internal/identitygovernance/model"
	notifcommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/notificationtemplate"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Notice queue settings. Fixed, not configurable.
const (
	// noticeQueueCapacity bounds how many undelivered notices are held.
	noticeQueueCapacity = 256

	// noticeWorkers bounds how many sends are in flight.
	noticeWorkers = 2

	// noticeTimeout bounds one delivery attempt: profile read, render and send.
	noticeTimeout = 30 * time.Second
)

// Delivery outcomes, logged in the "outcome" field.
const (
	outcomeQueued         = "queued"
	outcomeDisabled       = "disabled"
	outcomeQueueFull      = "queue_full"
	outcomeReadFailed     = "recipient_read_failed"
	outcomeMissingContact = "missing_contact"
	outcomeInvalidContact = "invalid_contact"
	outcomeTemplateFailed = "template_failed"
	outcomeSendFailed     = "send_failed"
	outcomeNoSender       = "no_default_sender"
	outcomeClosed         = "notifier_closed"
	outcomeAccepted       = "mail_server_accepted"
)

// lockTemplateHandle is the notification template a lock notice renders.
const lockTemplateHandle = "account-locked"

// Template placeholders filled by a lock notice. All are always supplied, since the renderer leaves
// unmatched placeholders as literal text.
const (
	templateKeyLockedAccess   = "lockedAccess"
	templateKeyLockDuration   = "lockDuration"
	templateKeyRecoveryAdvice = "recoveryAdvice"
)

// lockTemplateKeys is the set of placeholders a lock notice fills.
var lockTemplateKeys = map[string]struct{}{
	templateKeyLockedAccess:   {},
	templateKeyLockDuration:   {},
	templateKeyRecoveryAdvice: {},
}

// unlockTimeLayout writes the unlock time locale-neutrally, to the minute.
const unlockTimeLayout = "2006-01-02 15:04 UTC"

// signInCopy names, per scope, the sign-in that stopped working.
var signInCopy = map[model.AccessScope]string{
	model.AccessScopeCredential: "Credential sign-in",
	model.AccessScopeOTP:        "Verification code sign-in",
	model.AccessScopeEntity:     "Sign-in to your account",
}

// lockNotice is one queued notice. It holds no address; the address is read at delivery.
type lockNotice struct {
	entityID string
	category providers.EntityCategory
	scope    model.AccessScope
	episode  model.LockEpisode
}

// lockEmailNotifier delivers lock notices as email through the server's default email sender.
type lockEmailNotifier struct {
	section   ConfigSource
	profiles  ProfileReader
	templates TemplateRenderer
	senders   EmailSender
	sender    SenderSource
	queue     chan lockNotice
	timeout   time.Duration
	logger    *log.Logger
	workers   sync.WaitGroup
	closeOnce sync.Once
	mu        sync.RWMutex
	closed    bool
}

// NewLockEmailNotifier builds the notifier and starts its workers.
func NewLockEmailNotifier(section ConfigSource, profiles ProfileReader, templates TemplateRenderer,
	senders EmailSender, sender SenderSource) *lockEmailNotifier {
	notifier := &lockEmailNotifier{
		section:   section,
		profiles:  profiles,
		templates: templates,
		senders:   senders,
		sender:    sender,
		queue:     make(chan lockNotice, noticeQueueCapacity),
		timeout:   noticeTimeout,
		logger: log.GetLogger().With(
			log.String(log.LoggerKeyComponentName, "IdentityGovernanceLockNotifier")),
	}

	notifier.workers.Add(noticeWorkers)
	for range noticeWorkers {
		go notifier.run()
	}

	return notifier
}

// NotifyLockFormed queues a notice, or records why it did not. Settings are checked here and again at
// delivery.
func (n *lockEmailNotifier) NotifyLockFormed(ctx context.Context, entity model.GovernedEntity,
	scope model.AccessScope, episode model.LockEpisode) {
	cfg := n.section.Current(ctx)
	if _, enabled := recipientAttributeFor(cfg, entity.Category); !enabled {
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
	case n.queue <- lockNotice{entityID: entity.ID, category: entity.Category, scope: scope, episode: episode}:
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
	// A fresh bounded context; the request's context is already cancelled.
	ctx, cancel := context.WithTimeout(context.Background(), n.timeout)
	defer cancel()

	cfg := n.section.Current(ctx)
	attribute, enabled := recipientAttributeFor(cfg, notice.category)
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
		// Enabled but no default email sender is configured. Recorded per notice.
		n.record(ctx, notice.entityID, notice.scope, outcomeNoSender)
		return
	}

	rendered, svcErr := n.templates.Resolve(ctx, notificationtemplate.ChannelTypeEmail, lockTemplateHandle,
		notificationtemplate.RenderInput{Data: lockTemplateData(notice.scope, notice.episode)})
	if svcErr != nil {
		n.record(ctx, notice.entityID, notice.scope, outcomeTemplateFailed,
			log.String("errorCode", svcErr.Code))
		return
	}

	if svcErr := n.senders.SendEmail(ctx, senderID, notifcommon.EmailData{
		To:      []string{address},
		Subject: rendered.Subject,
		Body:    rendered.Body,
		IsHTML:  true,
	}); svcErr != nil {
		n.record(ctx, notice.entityID, notice.scope, outcomeSendFailed,
			log.String("errorCode", svcErr.Code))
		return
	}

	// Accepted by the mail server; not proof of receipt.
	n.record(ctx, notice.entityID, notice.scope, outcomeAccepted)
}

// stringAttribute returns a top-level string attribute, or "" if it is missing or not a string.
func stringAttribute(attributes json.RawMessage, name string) string {
	var top map[string]json.RawMessage
	if len(attributes) == 0 || json.Unmarshal(attributes, &top) != nil {
		return ""
	}
	var value string
	if json.Unmarshal(top[name], &value) != nil {
		return ""
	}
	return value
}

// lockTemplateData builds the template data for a lock notice. It carries only scope-derived text:
// no address, identifier or failure count.
func lockTemplateData(scope model.AccessScope, episode model.LockEpisode) map[string]string {
	signIn := signInCopy[model.AccessScopeEntity]
	if phrase, ok := signInCopy[scope]; ok {
		signIn = phrase
	}
	expires := episode.UnlockAt != model.PermanentUnlockAt && episode.Duration > 0

	access, regain := signIn+" has been locked", "To regain access"
	if expires {
		access, regain = signIn+" has been temporarily locked", "To regain access sooner"
	}
	recovery := regain + ", complete account recovery or contact your administrator."
	if scope == model.AccessScopeCredential || scope == model.AccessScopeOTP {
		recovery += " Other sign-in methods remain available."
	}

	duration := "This lock does not expire automatically."
	if expires {
		duration = lockDurationCopy(signIn, episode)
	}
	return map[string]string{
		templateKeyLockedAccess:   access,
		templateKeyLockDuration:   duration,
		templateKeyRecoveryAdvice: recovery,
	}
}

// lockDurationCopy describes the committed episode, not the policy at delivery.
func lockDurationCopy(signIn string, episode model.LockEpisode) string {
	text := signIn + " will be available again in " + formatLockDuration(episode.Duration)
	if ends, err := time.Parse(time.RFC3339, episode.UnlockAt); err == nil {
		text += ", on " + ends.UTC().Format(unlockTimeLayout)
	}
	return text + "."
}

// formatLockDuration writes whole-second policy durations in words.
func formatLockDuration(duration time.Duration) string {
	units := []struct {
		name string
		size time.Duration
	}{
		{"day", 24 * time.Hour}, {"hour", time.Hour}, {"minute", time.Minute}, {"second", time.Second},
	}
	parts := make([]string, 0, len(units))
	for _, unit := range units {
		count := duration / unit.size
		if count == 0 {
			continue
		}
		name := unit.name
		if count != 1 {
			name += "s"
		}
		parts = append(parts, fmt.Sprintf("%d %s", count, name))
		duration %= unit.size
	}
	return strings.Join(parts, ", ")
}

// record logs one delivery outcome. The entity ID is masked and the address is never logged.
func (n *lockEmailNotifier) record(ctx context.Context, entityID string, scope model.AccessScope, outcome string,
	extra ...log.Field) {
	fields := append([]log.Field{
		log.MaskedString("entityId", entityID),
		log.String("scope", string(scope)),
		log.String("outcome", outcome),
	}, extra...)

	switch outcome {
	case outcomeQueued, outcomeAccepted, outcomeDisabled, outcomeClosed:
		n.logger.Debug(ctx, "Account lock notification", fields...)
	default:
		n.logger.Error(ctx, "Account lock notification not delivered", fields...)
	}
}

// SuppliesTemplateKey reports whether a lock notice fills this placeholder.
func SuppliesTemplateKey(key string) bool {
	_, ok := lockTemplateKeys[key]
	return ok
}
