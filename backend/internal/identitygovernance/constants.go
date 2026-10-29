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

// permanentUnlockAt is the stored unlock time for a permanent lock (duration zero). It compares
// correctly both as a string and as a timestamp.
const permanentUnlockAt = "9999-12-31T23:59:59Z"

// LockReason says why an account-wide lock entry exists. Descriptive only; the unlock time decides
// expiry.
type LockReason string

// lockReasonPostSuspension marks the account-wide lock written on unsuspend.
const lockReasonPostSuspension LockReason = "POST_SUSPENDED"

// granularity is what a lock holds: the authentication method that failed, or the whole account.
type granularity string

const (
	// granularityAuthenticationMethod locks only the method that failed. The default.
	granularityAuthenticationMethod granularity = "authentication_method"

	// granularityAccount counts every lockable failure under the account-wide key, and a lock holds every
	// method.
	granularityAccount granularity = "account"
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

	// noticeShutdownBudget bounds Stop. The server's graceful shutdown has five seconds in total.
	noticeShutdownBudget = 2 * time.Second
)

// lifecycleFailure is the class a failed lifecycle operation is audited with.
type lifecycleFailure string

// Lifecycle failure classes.
const (
	failureNotAuthorized   lifecycleFailure = "not_authorized"
	failureNotApplicable   lifecycleFailure = "not_applicable"
	failureNoSubject       lifecycleFailure = "no_subject"
	failureNotFound        lifecycleFailure = "not_found"
	failureNoteTooLong     lifecycleFailure = "note_too_long"
	failureOperationFailed lifecycleFailure = "operation_failed"
)

// maxOperatorNoteLength is the longest operator note a suspension takes, in characters.
const maxOperatorNoteLength = 100

// Lifecycle audit messages, published with a successful operation.
const (
	messageLocksReleased              = "Every automatic lock released"
	messageSuspensionPlaced           = "Suspension placed"
	messageSuspensionInPlace          = "Suspension already in place"
	messageSuspensionReleased         = "Suspension released"
	messageSuspensionReleasedOntoHold = "Suspension released onto a post-suspension lock"
)

// noticeOutcome is what became of one lock email, logged in the "outcome" field.
type noticeOutcome string

// Lock email delivery outcomes.
const (
	outcomeQueued         noticeOutcome = "queued"
	outcomeDisabled       noticeOutcome = "disabled"
	outcomeQueueFull      noticeOutcome = "queue_full"
	outcomeReadFailed     noticeOutcome = "recipient_read_failed"
	outcomeMissingContact noticeOutcome = "missing_contact"
	outcomeInvalidContact noticeOutcome = "invalid_contact"
	outcomeTemplateFailed noticeOutcome = "template_failed"
	outcomeSendFailed     noticeOutcome = "send_failed"
	outcomeNoSender       noticeOutcome = "no_default_sender"
	outcomeNoTemplate     noticeOutcome = "no_template"
	outcomeClosed         noticeOutcome = "notifier_closed"
	outcomeShutdown       noticeOutcome = "dropped_at_shutdown"
	outcomeAccepted       noticeOutcome = "mail_server_accepted"
)

// Lock email template placeholders. All are always supplied, since the renderer fails on a
// placeholder with no value.
const (
	templateKeyLockedAuthenticationMethod = "lockedAuthenticationMethod"
	templateKeyLockDuration               = "lockDuration"
	templateKeyUnlockTime                 = "unlockTime"
)

// unlockTimeLayout writes the unlock time in UTC, locale-neutrally, to the minute. The templates name
// the zone.
const unlockTimeLayout = "2006-01-02 15:04"

// AuthenticationMethodAccount is the reserved key for account-wide lock counting. It is never derived from an
// authentication method, so no caller names it.
const AuthenticationMethodAccount providers.AuthenticationMethod = "account"

// AllAuthenticationMethods lists every registered authentication method.
var AllAuthenticationMethods = []providers.AuthenticationMethod{
	providers.AuthenticationMethodCredential,
	providers.AuthenticationMethodOTP,
	providers.AuthenticationMethodPasskey,
	providers.AuthenticationMethodFederated,
	providers.AuthenticationMethodMagicLink,
	providers.AuthenticationMethodOpenID4VP,
	providers.AuthenticationMethodSystemCredential,
	AuthenticationMethodAccount,
}

// lockableMethods says whether failures at each registered method are counted and may form a lock. It
// is fixed per method, not configurable, and applies to every entity category.
var lockableMethods = map[providers.AuthenticationMethod]bool{
	providers.AuthenticationMethodCredential: true,
	providers.AuthenticationMethodOTP:        true,

	// Not locked, so a stale client secret in a retry loop cannot take the client offline.
	providers.AuthenticationMethodSystemCredential: false,

	// These name no entity on the failure path.
	providers.AuthenticationMethodPasskey:   false,
	providers.AuthenticationMethodMagicLink: false,
	providers.AuthenticationMethodFederated: false,
	providers.AuthenticationMethodOpenID4VP: false,

	// Account-wide key; never derived.
	AuthenticationMethodAccount: true,
}

// heldAtRecovery says whether an automatic lock still holds each method at the recovery plane. Only a
// reusable secret stays held, so recovery is not an unlocked way to guess it.
var heldAtRecovery = map[providers.AuthenticationMethod]bool{
	providers.AuthenticationMethodCredential:       true,
	providers.AuthenticationMethodSystemCredential: true,

	providers.AuthenticationMethodOTP:       false,
	providers.AuthenticationMethodMagicLink: false,
	providers.AuthenticationMethodPasskey:   false,
	providers.AuthenticationMethodFederated: false,
	providers.AuthenticationMethodOpenID4VP: false,

	// Account-wide key; never derived.
	AuthenticationMethodAccount: false,
}
