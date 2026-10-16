// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernance

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type StatusTestSuite struct {
	suite.Suite
	now time.Time
}

func TestStatusTestSuite(t *testing.T) {
	suite.Run(t, new(StatusTestSuite))
}

func (s *StatusTestSuite) SetupTest() {
	s.now = time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)
}

func (s *StatusTestSuite) at(offset time.Duration) string {
	return s.now.Add(offset).UTC().Format(time.RFC3339)
}

// newTestLockEnforcement returns an enforcement that skips the given scopes.
func newTestLockEnforcement(unenforced map[providers.AccessScope]struct{}) lockEnforcement {
	return lockEnforcement{unenforced: unenforced}
}

// SUSPENDED takes precedence over LOCKED, which takes precedence over ACTIVE.
func (s *StatusTestSuite) TestEffectiveStatusPrecedence() {
	locked := AccessState{}
	putScopeLock(&locked, testScope, ScopeLock{UnlockAt: s.at(5 * time.Minute)})

	suspended := AccessState{Suspend: &SuspendState{SuspendedAt: s.at(-time.Hour)}}

	suspendedAndLocked := suspended
	putScopeLock(&suspendedAndLocked, testScope, ScopeLock{UnlockAt: s.at(5 * time.Minute)})

	cases := []struct {
		name  string
		state providers.EntityState
		doc   AccessState
		want  providers.AccessStatus
	}{
		{"nothing held", providers.EntityStateActive, AccessState{}, providers.AccessStatusActive},
		{"lock only", providers.EntityStateActive, locked, providers.AccessStatusLocked},
		// Either store alone reports the suspension.
		{"the document alone", providers.EntityStateActive, suspended,
			providers.AccessStatusSuspended},
		{"the column alone", providers.EntityStateSuspended, AccessState{},
			providers.AccessStatusSuspended},
		{"a suspension beats a lock", providers.EntityStateSuspended, suspendedAndLocked,
			providers.AccessStatusSuspended},
		{"an expired lock is not a lock", providers.EntityStateActive,
			func() AccessState {
				var lapsed AccessState
				putScopeLock(&lapsed, testScope, ScopeLock{UnlockAt: s.at(-time.Second)})
				return lapsed
			}(), providers.AccessStatusActive},
		// Only SUSPENDED on the state column holds.
		{"an unrecognized state is not a suspension", providers.EntityState("SOME_FUTURE_STATE"),
			AccessState{}, providers.AccessStatusActive},
		{"an empty state is not a suspension", providers.EntityState(""),
			AccessState{}, providers.AccessStatusActive},
		{"an unrecognized state still reports its lock", providers.EntityState("SOME_FUTURE_STATE"),
			locked, providers.AccessStatusLocked},
		{"an unrecognized state with a suspension member is suspended",
			providers.EntityState("SOME_FUTURE_STATE"), suspended, providers.AccessStatusSuspended},
		{"the post-suspension hold reads LOCKED", providers.EntityStateActive,
			func() AccessState {
				var held AccessState
				putScopeLock(&held, AccessScopeEntity,
					ScopeLock{UnlockAt: PermanentUnlockAt, Reason: LockReasonPostSuspension})
				return held
			}(), providers.AccessStatusLocked},
	}

	for _, c := range cases {
		s.Equal(c.want, reportFor(c.state, c.doc, s.now, lockEnforcement{}).Value, c.name)
	}
}

// Each case asserts both the status and its details.
func (s *StatusTestSuite) TestStatusDetailsDescribeTheHold() {
	automatic := AccessState{}
	putScopeLock(&automatic, providers.AccessScopeOTP,
		ScopeLock{LockCount: 1, UnlockAt: s.at(5 * time.Minute)})

	postSuspension := AccessState{}
	putScopeLock(&postSuspension, AccessScopeEntity,
		ScopeLock{UnlockAt: PermanentUnlockAt, Reason: LockReasonPostSuspension})

	suspended := AccessState{Suspend: &SuspendState{
		SuspendedAt: s.at(-time.Hour), OperatorNote: "Credential exposure",
	}}

	suspendedAndLocked := suspended
	putScopeLock(&suspendedAndLocked, providers.AccessScopeOTP,
		ScopeLock{UnlockAt: s.at(5 * time.Minute)})

	cases := []struct {
		name   string
		state  providers.EntityState
		doc    AccessState
		status providers.AccessStatus
		want   *providers.AccessStatusDetails
	}{
		{
			name: "nothing held", state: providers.EntityStateActive, doc: AccessState{},
			status: providers.AccessStatusActive,
			want:   nil,
		},
		{
			name:  "an automatic lock names its authentication_method and when it lifts",
			state: providers.EntityStateActive, doc: automatic,
			status: providers.AccessStatusLocked,
			want: &providers.AccessStatusDetails{
				Reason: providers.AccessHoldReasonFailedAttempts,
				LockedScopes: []providers.AccessLockedScope{
					{Scope: string(providers.AccessScopeOTP), ExpiresAt: s.at(5 * time.Minute)},
				},
			},
		},
		{
			name:  "the post-suspension hold reports no expiry",
			state: providers.EntityStateActive, doc: postSuspension,
			status: providers.AccessStatusLocked,
			want: &providers.AccessStatusDetails{
				Reason: providers.AccessHoldReasonSuspensionReleaseLock,
				LockedScopes: []providers.AccessLockedScope{
					{Scope: string(AccessScopeEntity)},
				},
			},
		},
		{
			name:  "a suspension carries when it was placed and why",
			state: providers.EntityStateSuspended, doc: suspended,
			status: providers.AccessStatusSuspended,
			want: &providers.AccessStatusDetails{
				Reason:       providers.AccessHoldReasonAdministrativeSuspension,
				Since:        s.at(-time.Hour),
				OperatorNote: "Credential exposure",
			},
		},
		{
			name:  "the details follow the same precedence as the status",
			state: providers.EntityStateSuspended, doc: suspendedAndLocked,
			status: providers.AccessStatusSuspended,
			want: &providers.AccessStatusDetails{
				Reason:       providers.AccessHoldReasonAdministrativeSuspension,
				Since:        s.at(-time.Hour),
				OperatorNote: "Credential exposure",
			},
		},
		{
			name:  "a suspension known only from the column reports no circumstances",
			state: providers.EntityStateSuspended, doc: AccessState{},
			status: providers.AccessStatusSuspended,
			want: &providers.AccessStatusDetails{
				Reason: providers.AccessHoldReasonAdministrativeSuspension,
			},
		},
		{
			name:   "an operator who gave no note produces none",
			state:  providers.EntityStateSuspended,
			doc:    AccessState{Suspend: &SuspendState{SuspendedAt: s.at(-time.Hour)}},
			status: providers.AccessStatusSuspended,
			want: &providers.AccessStatusDetails{
				Reason: providers.AccessHoldReasonAdministrativeSuspension,
				Since:  s.at(-time.Hour),
			},
		},
	}

	for _, c := range cases {
		report := reportFor(c.state, c.doc, s.now, lockEnforcement{})
		s.Equal(c.status, report.Value, c.name)
		s.Equal(c.want, report.Details, c.name)
	}
}

func (s *StatusTestSuite) TestEveryLiveAuthenticationMethodLockIsReported() {
	both := AccessState{}
	putScopeLock(&both, providers.AccessScopeOTP,
		ScopeLock{LockCount: 1, UnlockAt: s.at(10 * time.Minute)})
	putScopeLock(&both, providers.AccessScopeCredential,
		ScopeLock{LockCount: 2, UnlockAt: s.at(5 * time.Minute)})

	report := reportFor(providers.EntityStateActive, both, s.now, lockEnforcement{})
	s.Equal(providers.AccessStatusLocked, report.Value)
	s.Require().NotNil(report.Details)
	s.Equal(providers.AccessHoldReasonFailedAttempts, report.Details.Reason)
	s.Equal([]providers.AccessLockedScope{
		{Scope: string(providers.AccessScopeCredential), ExpiresAt: s.at(5 * time.Minute)},
		{Scope: string(providers.AccessScopeOTP), ExpiresAt: s.at(10 * time.Minute)},
	}, report.Details.LockedScopes, "in the registry's order, so two reads agree")

	// A lapsed entry stays in the document but is not reported.
	oneLapsed := AccessState{}
	putScopeLock(&oneLapsed, providers.AccessScopeOTP,
		ScopeLock{LockCount: 1, UnlockAt: s.at(10 * time.Minute)})
	putScopeLock(&oneLapsed, providers.AccessScopeCredential,
		ScopeLock{LockCount: 2, UnlockAt: s.at(-time.Minute)})

	report = reportFor(providers.EntityStateActive, oneLapsed, s.now, lockEnforcement{})
	s.Require().NotNil(report.Details)
	s.Equal([]providers.AccessLockedScope{
		{Scope: string(providers.AccessScopeOTP), ExpiresAt: s.at(10 * time.Minute)},
	}, report.Details.LockedScopes)
}

// The entity-wide entry reports as the single entity scope, not as a list of methods.
func (s *StatusTestSuite) TestTheEntityEntryReportsAsOneScope() {
	doc := AccessState{}
	putScopeLock(&doc, AccessScopeEntity,
		ScopeLock{LockCount: 1, UnlockAt: s.at(30 * time.Minute)})
	putScopeLock(&doc, providers.AccessScopeOTP,
		ScopeLock{LockCount: 1, UnlockAt: s.at(5 * time.Minute)})

	report := reportFor(providers.EntityStateActive, doc, s.now, lockEnforcement{})
	s.Require().NotNil(report.Details)
	s.Equal([]providers.AccessLockedScope{
		{Scope: string(AccessScopeEntity), ExpiresAt: s.at(30 * time.Minute)},
	}, report.Details.LockedScopes)
}

// A permanent lock reports no expiry.
func (s *StatusTestSuite) TestTheSentinelIsNeverReportedAsADate() {
	held := AccessState{}
	putScopeLock(&held, providers.AccessScopeCredential,
		ScopeLock{LockCount: 4, UnlockAt: PermanentUnlockAt})

	details := reportFor(providers.EntityStateActive, held, s.now, lockEnforcement{}).Details
	s.Require().NotNil(details)
	s.Require().Len(details.LockedScopes, 1)
	s.Empty(details.LockedScopes[0].ExpiresAt, "a hold that does not lift reports no expiry")
	s.NotContains(details.LockedScopes[0].ExpiresAt, "9999")
	s.Equal(providers.AccessHoldReasonFailedAttempts, details.Reason,
		"a permanent lock is still the escalation's doing, not a suspension release")
}

// An entry under a scope whose class is not lockable is not reported.
func (s *StatusTestSuite) TestAnUnlockableScopeIsNotReported() {
	doc := AccessState{}
	putScopeLock(&doc, providers.AccessScopePasskey,
		ScopeLock{UnlockAt: s.at(5 * time.Minute)})

	report := reportFor(providers.EntityStateActive, doc, s.now, lockEnforcement{})
	s.Equal(providers.AccessStatusActive, report.Value)
	s.Nil(report.Details)
}

// A lock whose policy is switched off is not reported, since the runtime admits past it.
func (s *StatusTestSuite) TestAnUnenforcedScopeIsNotReported() {
	var doc AccessState
	putScopeLock(&doc, providers.AccessScopeCredential,
		ScopeLock{UnlockAt: s.at(5 * time.Minute)})

	off := newTestLockEnforcement(map[providers.AccessScope]struct{}{
		providers.AccessScopeCredential: {},
	})

	s.Equal(providers.AccessStatusLocked,
		reportFor(providers.EntityStateActive, doc, s.now, lockEnforcement{}).Value,
		"the zero value enforces everything")
	s.Equal(providers.AccessStatusActive,
		reportFor(providers.EntityStateActive, doc, s.now, off).Value,
		"a scope the runtime admits past must not read as locked")
}

func (s *StatusTestSuite) TestOnlyTheUnenforcedScopeIsDropped() {
	var doc AccessState
	putScopeLock(&doc, providers.AccessScopeCredential,
		ScopeLock{UnlockAt: s.at(5 * time.Minute)})
	putScopeLock(&doc, providers.AccessScopeOTP, ScopeLock{UnlockAt: s.at(5 * time.Minute)})

	report := reportFor(providers.EntityStateActive, doc, s.now, newTestLockEnforcement(
		map[providers.AccessScope]struct{}{providers.AccessScopeCredential: {}}))

	s.Equal(providers.AccessStatusLocked, report.Value)
	s.Require().Len(report.Details.LockedScopes, 1)
	s.Equal(string(providers.AccessScopeOTP), report.Details.LockedScopes[0].Scope)
}

// The post-suspension hold is reported regardless of enforcement, since it is enforced regardless of
// the lockout setting.
func (s *StatusTestSuite) TestAnEntityWideHoldIsReportedEvenWhenNoMethodScopeIs() {
	doc := AccessState{Lock: LockState{Entity: &ScopeLock{
		UnlockAt: PermanentUnlockAt, Reason: LockReasonPostSuspension,
	}}}

	report := reportFor(providers.EntityStateActive, doc, s.now,
		newTestLockEnforcement(allMethodScopes()))

	s.Equal(providers.AccessStatusLocked, report.Value)
	s.Equal(providers.AccessHoldReasonSuspensionReleaseLock, report.Details.Reason)
}

// An entity-wide failure lock follows enforcement, as the check does.
func (s *StatusTestSuite) TestAnUnenforcedEntityWideFailureLockIsNotReported() {
	doc := AccessState{Lock: LockState{Entity: &ScopeLock{LockCount: 3, UnlockAt: PermanentUnlockAt}}}

	report := reportFor(providers.EntityStateActive, doc, s.now, newTestLockEnforcement(
		map[providers.AccessScope]struct{}{AccessScopeEntity: {}}))

	s.Equal(providers.AccessStatusActive, report.Value)
}

// readOf runs the management read over a hydrated entity with the given runtime attributes.
func readOf(state providers.EntityState, runtimeAttrs string) AccessRead {
	entity := providers.Entity{ID: "e1", Category: providers.EntityCategoryUser, State: state}
	if runtimeAttrs != "" {
		entity.RuntimeAttributes = json.RawMessage(runtimeAttrs)
	}
	return accessReader{}.Read(entity)
}

func (s *StatusTestSuite) TestReadReportsTheSuspensionWithItsNote() {
	read := readOf(providers.EntityStateSuspended, `{"accessState":{"suspend":`+
		`{"suspendedAt":"2026-09-10T11:02:00Z","operatorNote":"Credential exposure"},`+
		`"lastLoginAt":"2026-09-09T18:22:04Z"}}`)

	s.Equal(providers.AccessStatusSuspended, read.Status.Value)
	s.Require().NotNil(read.Status.Details)
	s.Equal("Credential exposure", read.Status.Details.OperatorNote)
	s.Equal("2026-09-09T18:22:04Z", read.LastLoginAt)
}

// The document holds while the column says ACTIVE; the column alone reports no circumstances.
func (s *StatusTestSuite) TestReadHoldsOnEitherSuspensionStore() {
	s.Equal(providers.AccessStatusSuspended, readOf(providers.EntityStateActive,
		`{"accessState":{"suspend":{"suspendedAt":"2026-09-10T11:02:00Z"}}}`).Status.Value)

	column := readOf(providers.EntityStateSuspended, `{"accessState":{"lastLoginAt":"2026-09-09T18:22:04Z"}}`)
	s.Equal(providers.AccessStatusSuspended, column.Status.Value)
	s.Require().NotNil(column.Status.Details)
	s.Empty(column.Status.Details.Since)
}

// The entity-wide entry is not read as a method named "entity".
func (s *StatusTestSuite) TestReadReportsTheEntityHoldAsLocked() {
	read := readOf(providers.EntityStateActive,
		`{"accessState":{"lock":{"entity":{"unlockAt":"9999-12-31T23:59:59Z","reason":"POST_SUSPENDED"}}}}`)
	s.Equal(providers.AccessStatusLocked, read.Status.Value)
}

func (s *StatusTestSuite) TestReadReportsAnUnparsableDocumentAsSuspended() {
	read := readOf(providers.EntityStateActive, `not json`)
	s.Equal(providers.AccessStatusSuspended, read.Status.Value)
	s.Require().NotNil(read.Status.Details)
	s.Equal(providers.AccessHoldReasonAdministrativeSuspension, read.Status.Details.Reason)
}

func (s *StatusTestSuite) TestReadWithNothingRecordedIsActive() {
	s.Equal(providers.AccessStatusActive, readOf(providers.EntityStateActive, "").Status.Value)
	s.Equal(providers.AccessStatusActive, readOf(providers.EntityStateActive, `{"other":1}`).Status.Value)
}

func allMethodScopes() map[providers.AccessScope]struct{} {
	scopes := make(map[providers.AccessScope]struct{})
	for _, scope := range AllAccessScopes {
		if scope != AccessScopeEntity {
			scopes[scope] = struct{}{}
		}
	}
	return scopes
}
