// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernance

import (
	"math"
	"time"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// formationRetries bounds the re-reads when a concurrent writer wins the lock-formation guard.
const formationRetries = 3

// PermanentUnlockAt is the stored unlock time for a permanent lock (duration zero). It compares
// correctly both as a string and as a timestamp.
const PermanentUnlockAt = "9999-12-31T23:59:59Z"

// LockReason says why an entity-wide lock entry exists. Descriptive only; the unlock time decides
// expiry.
type LockReason string

// LockReasonPostSuspension marks the entity-wide lock written on unsuspend.
const LockReasonPostSuspension LockReason = "POST_SUSPENDED"

// granularity is what a lock holds: the authentication method that failed, or the whole entity.
type granularity string

const (
	// granularityAuthenticationMethod locks only the method that failed. The default.
	granularityAuthenticationMethod granularity = "authentication_method"

	// granularityEntity counts every lockable failure under the entity scope, and a lock holds every
	// method.
	granularityEntity granularity = "entity"
)

// disclosureMode is whether a refusal may state the account's hold.
type disclosureMode string

const (
	// discloseNever answers a held account the same as a wrong secret.
	discloseNever disclosureMode = "never"

	// discloseAlways lets the refusal say the account is locked or suspended.
	discloseAlways disclosureMode = "always"
)

// defaultDisclosureMode is the disclosure mode every policy resolves to. It is not configurable.
const defaultDisclosureMode = discloseAlways

// defaultLockEmailRecipientAttribute is the profile attribute the lock email is sent to when none is
// configured.
const defaultLockEmailRecipientAttribute = "email"

// defaultLastLoginResolutionSeconds is the resolution used when recording is on and none is set.
const defaultLastLoginResolutionSeconds = 3600

// maxDurationSeconds is the largest seconds value that converts to a time.Duration without
// overflowing. Writes above it are refused; conversion clamps.
const maxDurationSeconds = int(math.MaxInt64 / time.Second)

// Lock email queue settings. Fixed, not configurable.
const (
	// noticeQueueCapacity bounds how many undelivered notices are held.
	noticeQueueCapacity = 256

	// noticeWorkers bounds how many sends are in flight.
	noticeWorkers = 2

	// noticeTimeout bounds one delivery attempt: profile read, render and send.
	noticeTimeout = 30 * time.Second
)

// Lock email delivery outcomes, logged in the "outcome" field.
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

// lockTemplateHandle is the notification template the lock email renders.
const lockTemplateHandle = "account-locked"

// Lock email template placeholders. All are always supplied, since the renderer leaves unmatched
// placeholders as literal text.
const (
	templateKeyLockedSignIn = "lockedSignIn"
	templateKeyLockDuration = "lockDuration"
	templateKeyUnlockTime   = "unlockTime"
)

// permanentLockTemplateHandle is the notification template a lock that does not expire renders.
const permanentLockTemplateHandle = "account-locked-permanent"

// unlockTimeLayout writes the unlock time locale-neutrally, to the minute.
const unlockTimeLayout = "2006-01-02 15:04 UTC"

// AccessScopeEntity is the reserved key for entity-wide lock counting. It is never derived from an
// authentication method, so no caller names it.
const AccessScopeEntity providers.AccessScope = "entity"

// AllAccessScopes lists every registered access scope.
var AllAccessScopes = []providers.AccessScope{
	providers.AccessScopeCredential,
	providers.AccessScopeOTP,
	providers.AccessScopePasskey,
	providers.AccessScopeFederated,
	providers.AccessScopeMagicLink,
	providers.AccessScopeOpenID4VP,
	providers.AccessScopeSystemCredential,
	AccessScopeEntity,
}

// lockableScopes says whether failures at each registered scope are counted and may form a lock. It
// is fixed per scope, not configurable, and applies to every entity category.
var lockableScopes = map[providers.AccessScope]bool{
	providers.AccessScopeCredential: true,
	providers.AccessScopeOTP:        true,

	// Not locked, so a stale client secret in a retry loop cannot take the client offline.
	providers.AccessScopeSystemCredential: false,

	// These name no entity on the failure path.
	providers.AccessScopePasskey:   false,
	providers.AccessScopeMagicLink: false,
	providers.AccessScopeFederated: false,
	providers.AccessScopeOpenID4VP: false,

	// Entity-wide scope; never derived.
	AccessScopeEntity: true,
}
