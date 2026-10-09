// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernance

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	governanceconfig "github.com/thunder-id/thunderid/internal/identitygovernance/config"
	"github.com/thunder-id/thunderid/internal/identitygovernance/model"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// SUSPENDED takes precedence over LOCKED, which takes precedence over ACTIVE.
func (s *StatusTestSuite) TestEffectiveStatusPrecedence() {
	locked := model.AccessState{}
	putScopeLock(&locked, testScope, model.ScopeLock{UnlockAt: s.at(5 * time.Minute)})

	suspended := model.AccessState{Suspend: &model.SuspendState{SuspendedAt: s.at(-time.Hour)}}

	suspendedAndLocked := suspended
	putScopeLock(&suspendedAndLocked, testScope, model.ScopeLock{UnlockAt: s.at(5 * time.Minute)})

	cases := []struct {
		name  string
		state providers.EntityState
		doc   model.AccessState
		want  providers.AccessStatus
	}{
		{"nothing held", providers.EntityStateActive, model.AccessState{}, providers.AccessStatusActive},
		{"lock only", providers.EntityStateActive, locked, providers.AccessStatusLocked},
		// Either store alone reports the suspension.
		{"the document alone", providers.EntityStateActive, suspended,
			providers.AccessStatusSuspended},
		{"the column alone", providers.EntityStateSuspended, model.AccessState{},
			providers.AccessStatusSuspended},
		{"a suspension beats a lock", providers.EntityStateSuspended, suspendedAndLocked,
			providers.AccessStatusSuspended},
		{"an expired lock is not a lock", providers.EntityStateActive,
			func() model.AccessState {
				var lapsed model.AccessState
				putScopeLock(&lapsed, testScope, model.ScopeLock{UnlockAt: s.at(-time.Second)})
				return lapsed
			}(), providers.AccessStatusActive},
		// Only SUSPENDED on the state column holds.
		{"an unrecognized state is not a suspension", providers.EntityState("SOME_FUTURE_STATE"),
			model.AccessState{}, providers.AccessStatusActive},
		{"an empty state is not a suspension", providers.EntityState(""),
			model.AccessState{}, providers.AccessStatusActive},
		{"an unrecognized state still reports its lock", providers.EntityState("SOME_FUTURE_STATE"),
			locked, providers.AccessStatusLocked},
		{"an unrecognized state with a suspension member is suspended",
			providers.EntityState("SOME_FUTURE_STATE"), suspended, providers.AccessStatusSuspended},
		{"the post-suspension hold reads LOCKED", providers.EntityStateActive,
			func() model.AccessState {
				var held model.AccessState
				putScopeLock(&held, model.AccessScopeEntity,
					model.ScopeLock{UnlockAt: model.PermanentUnlockAt, Reason: model.ReasonPostSuspension})
				return held
			}(), providers.AccessStatusLocked},
	}

	for _, c := range cases {
		s.Equal(c.want, ReportFor(c.state, c.doc, s.now, governanceconfig.LockEnforcement{}).Value, c.name)
	}
}

// Each case asserts both the status and its details.
func (s *StatusTestSuite) TestStatusDetailsDescribeTheHold() {
	automatic := model.AccessState{}
	putScopeLock(&automatic, model.AccessScopeOTP,
		model.ScopeLock{LockCount: 1, UnlockAt: s.at(5 * time.Minute)})

	postSuspension := model.AccessState{}
	putScopeLock(&postSuspension, model.AccessScopeEntity,
		model.ScopeLock{UnlockAt: model.PermanentUnlockAt, Reason: model.ReasonPostSuspension})

	suspended := model.AccessState{Suspend: &model.SuspendState{
		SuspendedAt: s.at(-time.Hour), OperatorNote: "Credential exposure",
	}}

	suspendedAndLocked := suspended
	putScopeLock(&suspendedAndLocked, model.AccessScopeOTP,
		model.ScopeLock{UnlockAt: s.at(5 * time.Minute)})

	cases := []struct {
		name   string
		state  providers.EntityState
		doc    model.AccessState
		status providers.AccessStatus
		want   *providers.AccessStatusDetails
	}{
		{
			name: "nothing held", state: providers.EntityStateActive, doc: model.AccessState{},
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
					{Scope: string(model.AccessScopeOTP), ExpiresAt: s.at(5 * time.Minute)},
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
					{Scope: string(model.AccessScopeEntity)},
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
			state: providers.EntityStateSuspended, doc: model.AccessState{},
			status: providers.AccessStatusSuspended,
			want: &providers.AccessStatusDetails{
				Reason: providers.AccessHoldReasonAdministrativeSuspension,
			},
		},
		{
			name:   "an operator who gave no note produces none",
			state:  providers.EntityStateSuspended,
			doc:    model.AccessState{Suspend: &model.SuspendState{SuspendedAt: s.at(-time.Hour)}},
			status: providers.AccessStatusSuspended,
			want: &providers.AccessStatusDetails{
				Reason: providers.AccessHoldReasonAdministrativeSuspension,
				Since:  s.at(-time.Hour),
			},
		},
	}

	for _, c := range cases {
		report := ReportFor(c.state, c.doc, s.now, governanceconfig.LockEnforcement{})
		s.Equal(c.status, report.Value, c.name)
		s.Equal(c.want, report.Details, c.name)
	}
}

func (s *StatusTestSuite) TestEveryLiveAuthenticationMethodLockIsReported() {
	both := model.AccessState{}
	putScopeLock(&both, model.AccessScopeOTP,
		model.ScopeLock{LockCount: 1, UnlockAt: s.at(10 * time.Minute)})
	putScopeLock(&both, model.AccessScopeCredential,
		model.ScopeLock{LockCount: 2, UnlockAt: s.at(5 * time.Minute)})

	report := ReportFor(providers.EntityStateActive, both, s.now, governanceconfig.LockEnforcement{})
	s.Equal(providers.AccessStatusLocked, report.Value)
	s.Require().NotNil(report.Details)
	s.Equal(providers.AccessHoldReasonFailedAttempts, report.Details.Reason)
	s.Equal([]providers.AccessLockedScope{
		{Scope: string(model.AccessScopeCredential), ExpiresAt: s.at(5 * time.Minute)},
		{Scope: string(model.AccessScopeOTP), ExpiresAt: s.at(10 * time.Minute)},
	}, report.Details.LockedScopes, "in the registry's order, so two reads agree")

	// A lapsed entry stays in the document but is not reported.
	oneLapsed := model.AccessState{}
	putScopeLock(&oneLapsed, model.AccessScopeOTP,
		model.ScopeLock{LockCount: 1, UnlockAt: s.at(10 * time.Minute)})
	putScopeLock(&oneLapsed, model.AccessScopeCredential,
		model.ScopeLock{LockCount: 2, UnlockAt: s.at(-time.Minute)})

	report = ReportFor(providers.EntityStateActive, oneLapsed, s.now, governanceconfig.LockEnforcement{})
	s.Require().NotNil(report.Details)
	s.Equal([]providers.AccessLockedScope{
		{Scope: string(model.AccessScopeOTP), ExpiresAt: s.at(10 * time.Minute)},
	}, report.Details.LockedScopes)
}

// The entity-wide entry reports as the single entity scope, not as a list of methods.
func (s *StatusTestSuite) TestTheEntityEntryReportsAsOneScope() {
	doc := model.AccessState{}
	putScopeLock(&doc, model.AccessScopeEntity,
		model.ScopeLock{LockCount: 1, UnlockAt: s.at(30 * time.Minute)})
	putScopeLock(&doc, model.AccessScopeOTP,
		model.ScopeLock{LockCount: 1, UnlockAt: s.at(5 * time.Minute)})

	report := ReportFor(providers.EntityStateActive, doc, s.now, governanceconfig.LockEnforcement{})
	s.Require().NotNil(report.Details)
	s.Equal([]providers.AccessLockedScope{
		{Scope: string(model.AccessScopeEntity), ExpiresAt: s.at(30 * time.Minute)},
	}, report.Details.LockedScopes)
}

// A permanent lock reports no expiry.
func (s *StatusTestSuite) TestTheSentinelIsNeverReportedAsADate() {
	held := model.AccessState{}
	putScopeLock(&held, model.AccessScopeCredential,
		model.ScopeLock{LockCount: 4, UnlockAt: model.PermanentUnlockAt})

	details := ReportFor(providers.EntityStateActive, held, s.now, governanceconfig.LockEnforcement{}).Details
	s.Require().NotNil(details)
	s.Require().Len(details.LockedScopes, 1)
	s.Empty(details.LockedScopes[0].ExpiresAt, "a hold that does not lift reports no expiry")
	s.NotContains(details.LockedScopes[0].ExpiresAt, "9999")
	s.Equal(providers.AccessHoldReasonFailedAttempts, details.Reason,
		"a permanent lock is still the escalation's doing, not a suspension release")
}

// An entry under a scope whose class is not lockable is not reported.
func (s *StatusTestSuite) TestAnUnlockableScopeIsNotReported() {
	doc := model.AccessState{}
	putScopeLock(&doc, model.AccessScopePasskey,
		model.ScopeLock{UnlockAt: s.at(5 * time.Minute)})

	report := ReportFor(providers.EntityStateActive, doc, s.now, governanceconfig.LockEnforcement{})
	s.Equal(providers.AccessStatusActive, report.Value)
	s.Nil(report.Details)
}

// A lock whose policy is switched off is not reported, since the runtime admits past it.
func (s *StatusTestSuite) TestAnUnenforcedScopeIsNotReported() {
	var doc model.AccessState
	putScopeLock(&doc, model.AccessScopeCredential,
		model.ScopeLock{UnlockAt: s.at(5 * time.Minute)})

	off := governanceconfig.NewLockEnforcement(map[model.AccessScope]struct{}{
		model.AccessScopeCredential: {},
	})

	s.Equal(providers.AccessStatusLocked,
		ReportFor(providers.EntityStateActive, doc, s.now, governanceconfig.LockEnforcement{}).Value,
		"the zero value enforces everything")
	s.Equal(providers.AccessStatusActive,
		ReportFor(providers.EntityStateActive, doc, s.now, off).Value,
		"a scope the runtime admits past must not read as locked")
}

func (s *StatusTestSuite) TestOnlyTheUnenforcedScopeIsDropped() {
	var doc model.AccessState
	putScopeLock(&doc, model.AccessScopeCredential,
		model.ScopeLock{UnlockAt: s.at(5 * time.Minute)})
	putScopeLock(&doc, model.AccessScopeOTP, model.ScopeLock{UnlockAt: s.at(5 * time.Minute)})

	report := ReportFor(providers.EntityStateActive, doc, s.now, governanceconfig.NewLockEnforcement(
		map[model.AccessScope]struct{}{model.AccessScopeCredential: {}}))

	s.Equal(providers.AccessStatusLocked, report.Value)
	s.Require().Len(report.Details.LockedScopes, 1)
	s.Equal(string(model.AccessScopeOTP), report.Details.LockedScopes[0].Scope)
}

// An entity-wide hold is reported regardless of enforcement, since it is enforced regardless of the
// lockout setting.
func (s *StatusTestSuite) TestAnEntityWideHoldIsReportedEvenWhenNoMethodScopeIs() {
	doc := model.AccessState{Lock: model.LockState{Entity: &model.ScopeLock{
		UnlockAt: model.PermanentUnlockAt, Reason: model.ReasonPostSuspension,
	}}}

	report := ReportFor(providers.EntityStateActive, doc, s.now,
		governanceconfig.NewLockEnforcement(allMethodScopes()))

	s.Equal(providers.AccessStatusLocked, report.Value)
	s.Equal(providers.AccessHoldReasonSuspensionReleaseLock, report.Details.Reason)
}

// readOf runs the management read over a hydrated entity with the given runtime attributes.
func readOf(state providers.EntityState, runtimeAttrs string) AccessRead {
	entity := providers.Entity{ID: "e1", Category: providers.EntityCategoryUser, State: state}
	if runtimeAttrs != "" {
		entity.RuntimeAttributes = json.RawMessage(runtimeAttrs)
	}
	return AccessReader{}.Read(entity)
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

func allMethodScopes() map[model.AccessScope]struct{} {
	scopes := make(map[model.AccessScope]struct{})
	for _, scope := range model.AllAccessScopes {
		if scope != model.AccessScopeEntity {
			scopes[scope] = struct{}{}
		}
	}
	return scopes
}

type StatusTestSuite struct {
	suite.Suite
	now time.Time
}

func TestStatusTestSuite(t *testing.T) { suite.Run(t, new(StatusTestSuite)) }
func (s *StatusTestSuite) SetupTest()  { s.now = time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC) }
func (s *StatusTestSuite) at(offset time.Duration) string {
	return s.now.Add(offset).UTC().Format(time.RFC3339)
}
