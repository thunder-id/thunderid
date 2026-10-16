// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/identitygovernance"
	"github.com/thunder-id/thunderid/internal/system/database/model"
	"github.com/thunder-id/thunderid/internal/system/deployment"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// RuntimeStoreConstantsTestSuite checks the runtime statements: the SQLite forms run against a real
// database, so counting and the compare-and-swap guards are exercised by the database itself, and the
// Postgres forms are checked as statements.
type RuntimeStoreConstantsTestSuite struct {
	suite.Suite
	*sqliteHarness
	ctx context.Context
}

func TestRuntimeStoreConstantsTestSuite(t *testing.T) {
	suite.Run(t, new(RuntimeStoreConstantsTestSuite))
}

func (s *RuntimeStoreConstantsTestSuite) SetupTest() {
	s.sqliteHarness = newSQLiteHarness(s.T(), strings.ReplaceAll(s.T().Name(), "/", "-"))
	s.ctx = deployment.WithID(context.Background(), harnessDeploymentID)
}

func (s *RuntimeStoreConstantsTestSuite) seed(id string, systemAttrs interface{}) {
	s.seedEntity(s.T(), id, systemAttrs)
	if systemAttrs == nil {
		systemAttrs = "{}"
	}
	_, err := s.runtimeDB.Exec(`INSERT INTO "ENTITY_RUNTIME_DATA" `+
		`(DEPLOYMENT_ID,ENTITY_ID,STATE,RUNTIME_ATTRIBUTES,CREATED_AT,UPDATED_AT) `+
		`VALUES ($1,$2,'ACTIVE',$3,'t','t')`, harnessDeploymentID, id, systemAttrs)
	s.Require().NoError(err)
}

// accessStateDoc reads the whole stored document, or nil when there is none.
func (s *RuntimeStoreConstantsTestSuite) accessStateDoc(id string) map[string]interface{} {
	var raw string
	s.Require().NoError(s.runtimeDB.QueryRow(
		`SELECT RUNTIME_ATTRIBUTES FROM "ENTITY_RUNTIME_DATA" WHERE ENTITY_ID=$1`, id).Scan(&raw))
	if raw == "" {
		return nil
	}
	attrs := map[string]interface{}{}
	s.Require().NoError(json.Unmarshal([]byte(raw), &attrs))
	doc, _ := attrs[pgAccessStateKey].(map[string]interface{})
	return doc
}

// scopeEntry reads one scope's stored lock entry, or nil when it has none. The entity scope is at
// lock.entity and others at lock.authenticationMethods.<scope>.
func (s *RuntimeStoreConstantsTestSuite) scopeEntry(id string,
	scope providers.AccessScope) map[string]interface{} {
	doc := s.accessStateDoc(id)
	if doc == nil {
		return nil
	}
	lock, ok := doc["lock"].(map[string]interface{})
	if !ok {
		return nil
	}
	if scope == identitygovernance.AccessScopeEntity {
		entry, _ := lock["entity"].(map[string]interface{})
		return entry
	}
	authenticationMethods, ok := lock["authenticationMethods"].(map[string]interface{})
	if !ok {
		return nil
	}
	entry, _ := authenticationMethods[string(scope)].(map[string]interface{})
	return entry
}

const testScope = providers.AccessScopeCredential

// A first failure creates the nested document.
func (s *RuntimeStoreConstantsTestSuite) TestFirstFailureCreatesTheEntry() {
	s.seed("e1", nil)
	now := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)

	snapshot, recorded, err := s.runtime.IncrementAccessFailure(
		s.ctx, "e1", testScope, now, now.Add(-15*time.Minute))
	s.Require().NoError(err)
	s.True(recorded)
	s.Equal(1, snapshot.FailureCount)
	s.Equal(0, snapshot.LockCount)
	s.Empty(snapshot.UnlockAt)

	entry := s.scopeEntry("e1", testScope)
	s.Equal(float64(1), entry["failureCount"])
	s.Equal("2026-09-08T10:00:00Z", entry["lastFailedAt"])
}

func (s *RuntimeStoreConstantsTestSuite) TestFailuresInsideTheWindowAccumulate() {
	s.seed("e2", nil)
	base := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)

	for i := 1; i <= 3; i++ {
		at := base.Add(time.Duration(i) * time.Minute)
		snapshot, recorded, err := s.runtime.IncrementAccessFailure(
			s.ctx, "e2", testScope, at, at.Add(-15*time.Minute))
		s.Require().NoError(err)
		s.True(recorded)
		s.Equal(i, snapshot.FailureCount)
	}
}

func (s *RuntimeStoreConstantsTestSuite) TestFailureOutsideTheWindowRestartsTheCount() {
	s.seed("e3", `{"accessState":{"lock":{"authenticationMethods":{"credential":`+
		`{"failureCount":4,"lastFailedAt":"2026-09-08T09:00:00Z"}}}}}`)
	now := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)

	snapshot, _, err := s.runtime.IncrementAccessFailure(
		s.ctx, "e3", testScope, now, now.Add(-15*time.Minute))
	s.Require().NoError(err)
	s.Equal(1, snapshot.FailureCount, "a failure an hour later starts a new window")
}

// The first failure after a lock lapses restarts the count at 1.
func (s *RuntimeStoreConstantsTestSuite) TestFirstFailureAfterExpiryResetsTheCount() {
	s.seed("e4", `{"accessState":{"lock":{"authenticationMethods":{"credential":{"failureCount":5,"lockCount":2,`+
		`"lastFailedAt":"2026-09-08T09:50:00Z","unlockAt":"2026-09-08T09:55:00Z"}}}}}`)
	now := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)

	snapshot, _, err := s.runtime.IncrementAccessFailure(
		s.ctx, "e4", testScope, now, now.Add(-15*time.Minute))
	s.Require().NoError(err)
	s.Equal(1, snapshot.FailureCount, "the expired episode's count must not carry over")
	s.Equal(2, snapshot.LockCount, "lockCount survives, so the escalation remembers")
	s.Equal("2026-09-08T09:55:00Z", snapshot.UnlockAt, "the old unlockAt stays for lockDecay")
}

// The expiry reset happens once; later failures accumulate.
func (s *RuntimeStoreConstantsTestSuite) TestExpiryResetHappensOnlyOnce() {
	s.seed("e5", `{"accessState":{"lock":{"authenticationMethods":{"credential":{"failureCount":5,"lockCount":2,`+
		`"lastFailedAt":"2026-09-08T09:50:00Z","unlockAt":"2026-09-08T09:55:00Z"}}}}}`)
	base := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)

	first, _, err := s.runtime.IncrementAccessFailure(
		s.ctx, "e5", testScope, base, base.Add(-15*time.Minute))
	s.Require().NoError(err)
	s.Equal(1, first.FailureCount)

	second, _, err := s.runtime.IncrementAccessFailure(
		s.ctx, "e5", testScope, base.Add(time.Minute), base.Add(-14*time.Minute))
	s.Require().NoError(err)
	s.Equal(2, second.FailureCount, "the reset must not repeat while the same episode is referenced")
}

// Failures during a live lock on the same authentication method are not written.
func (s *RuntimeStoreConstantsTestSuite) TestFailuresDuringALiveLockAreNotCounted() {
	s.seed("e6", `{"accessState":{"lock":{"authenticationMethods":{"credential":{"failureCount":0,"lockCount":1,`+
		`"lastFailedAt":"2026-09-08T10:00:00Z","unlockAt":"2026-09-08T10:15:00Z"}}}}}`)
	now := time.Date(2026, 9, 8, 10, 5, 0, 0, time.UTC)

	_, recorded, err := s.runtime.IncrementAccessFailure(
		s.ctx, "e6", testScope, now, now.Add(-15*time.Minute))
	s.Require().NoError(err)
	s.False(recorded, "a live lock on this method must suppress the write")

	entry := s.scopeEntry("e6", testScope)
	s.Equal(float64(0), entry["failureCount"], "the counter must not have moved")
	s.Equal("2026-09-08T10:00:00Z", entry["lastFailedAt"], "nor the timestamp the reset arm reads")
	s.Equal(float64(1), entry["lockCount"], "spraying through a live lock must not advance the escalation")
}

func (s *RuntimeStoreConstantsTestSuite) TestALockOnOneMethodStillCountsAnother() {
	s.seed("e6b", `{"accessState":{"lock":{"authenticationMethods":{"credential":{"failureCount":0,"lockCount":1,`+
		`"lastFailedAt":"2026-09-08T10:00:00Z","unlockAt":"2026-09-08T10:15:00Z"}}}}}`)
	now := time.Date(2026, 9, 8, 10, 5, 0, 0, time.UTC)

	snapshot, recorded, err := s.runtime.IncrementAccessFailure(
		s.ctx, "e6b", providers.AccessScopeOTP, now, now.Add(-15*time.Minute))
	s.Require().NoError(err)
	s.True(recorded, "a lock on another method must not suppress this one")
	s.Equal(1, snapshot.FailureCount)

	held := s.scopeEntry("e6b", testScope)
	s.Equal(float64(1), held["lockCount"], "the held method's own entry must be untouched")
}

func (s *RuntimeStoreConstantsTestSuite) TestConcurrentFailuresCountSeparately() {
	s.seed("e7", nil)
	now := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)

	const attempts = 8
	var wg sync.WaitGroup
	errs := make([]error, attempts)
	for i := range attempts {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, _, err := s.runtime.IncrementAccessFailure(
				s.ctx, "e7", testScope, now, now.Add(-15*time.Minute))
			errs[idx] = err
		}(i)
	}
	wg.Wait()
	for _, err := range errs {
		s.Require().NoError(err)
	}

	s.Equal(float64(attempts), s.scopeEntry("e7", testScope)["failureCount"],
		"every concurrent failure must be counted")
}

// Lock formation writes the episode and resets the failure count to 0.
func (s *RuntimeStoreConstantsTestSuite) TestFormAccessLockOpensTheEpisode() {
	s.seed("e8", `{"accessState":{"lock":{"authenticationMethods":{"credential":{"failureCount":5,"lockCount":1,`+
		`"lastFailedAt":"2026-09-08T10:00:00Z"}}}}}`)
	now := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)

	formed, err := s.runtime.FormAccessLock(s.ctx, "e8", testScope,
		identitygovernance.ScopeLock{FailureCount: 5, LockCount: 1},
		identitygovernance.LockEpisode{LockCount: 2, UnlockAt: "2026-09-08T10:15:00Z"}, now)
	s.Require().NoError(err)
	s.True(formed)

	entry := s.scopeEntry("e8", testScope)
	s.Equal(float64(0), entry["failureCount"], "the count returns to zero when a lock forms")
	s.Equal(float64(2), entry["lockCount"])
	s.Equal("2026-09-08T10:15:00Z", entry["unlockAt"])
}

func (s *RuntimeStoreConstantsTestSuite) TestFormAccessLockRefusesAStaleFailureCount() {
	s.seed("e9", `{"accessState":{"lock":{"authenticationMethods":{"credential":{"failureCount":6,"lockCount":1}}}}}`)
	now := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)

	formed, err := s.runtime.FormAccessLock(s.ctx, "e9", testScope,
		identitygovernance.ScopeLock{FailureCount: 5, LockCount: 1},
		identitygovernance.LockEpisode{LockCount: 2, UnlockAt: "2026-09-08T10:15:00Z"}, now)
	s.Require().NoError(err)
	s.False(formed)
	s.Nil(s.scopeEntry("e9", testScope)["unlockAt"])
}

// A clear between the increment and the formation prevents the lock.
func (s *RuntimeStoreConstantsTestSuite) TestFormAccessLockRefusesAfterAConcurrentReset() {
	s.seed("e10", `{"accessState":{"lock":{"authenticationMethods":{"credential":{"failureCount":5,"lockCount":1}}}}}`)
	now := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)

	s.Require().NoError(s.runtime.ClearAccessState(s.ctx, "e10", []providers.AccessScope{testScope}))

	formed, err := s.runtime.FormAccessLock(s.ctx, "e10", testScope,
		identitygovernance.ScopeLock{FailureCount: 5, LockCount: 1},
		identitygovernance.LockEpisode{LockCount: 2, UnlockAt: "2026-09-08T10:15:00Z"}, now)
	s.Require().NoError(err)
	s.False(formed, "a reset between the increment and the formation must win")
	s.Nil(s.scopeEntry("e10", testScope))
}

// A live lock blocks a second episode whatever the count.
func (s *RuntimeStoreConstantsTestSuite) TestFormAccessLockRefusesWhileALockIsLive() {
	s.seed("e11", `{"accessState":{"lock":{"authenticationMethods":{"credential":{"failureCount":5,"lockCount":1,`+
		`"unlockAt":"2026-09-08T10:15:00Z"}}}}}`)
	now := time.Date(2026, 9, 8, 10, 5, 0, 0, time.UTC)

	formed, err := s.runtime.FormAccessLock(s.ctx, "e11", testScope,
		identitygovernance.ScopeLock{FailureCount: 5, LockCount: 1, UnlockAt: "2026-09-08T10:15:00Z"},
		identitygovernance.LockEpisode{LockCount: 2, UnlockAt: "2026-09-08T10:30:00Z"}, now)
	s.Require().NoError(err)
	s.False(formed)
	s.Equal("2026-09-08T10:15:00Z", s.scopeEntry("e11", testScope)["unlockAt"],
		"the live episode keeps the duration it was given")
}

// Two threshold crossings advance the lock count once.
func (s *RuntimeStoreConstantsTestSuite) TestConcurrentThresholdCrossingsAdvanceOneLock() {
	s.seed("e12", `{"accessState":{"lock":{"authenticationMethods":{"credential":{"failureCount":5,"lockCount":1}}}}}`)
	now := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)
	observed := identitygovernance.ScopeLock{FailureCount: 5, LockCount: 1}

	var wg sync.WaitGroup
	formedCount := 0
	var mu sync.Mutex
	for i := range 2 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			unlockAt := []string{"2026-09-08T10:15:00Z", "2026-09-08T10:30:00Z"}[idx]
			formed, err := s.runtime.FormAccessLock(s.ctx, "e12", testScope, observed,
				identitygovernance.LockEpisode{LockCount: 2, UnlockAt: unlockAt}, now)
			mu.Lock()
			defer mu.Unlock()
			s.NoError(err)
			if formed {
				formedCount++
			}
		}(i)
	}
	wg.Wait()

	s.Equal(1, formedCount, "only one of two concurrent crossings may open an episode")
	s.Equal(float64(2), s.scopeEntry("e12", testScope)["lockCount"], "one lock, not two")
}

func (s *RuntimeStoreConstantsTestSuite) TestClearAccessStateRemovesTheEntry() {
	s.seed("e13", `{"name":"kept","accessState":{"lock":{"authenticationMethods":{"credential":`+
		`{"failureCount":5,"lockCount":3,"unlockAt":"2026-09-08T10:15:00Z"},`+
		`"otp":{"failureCount":1}}}}}`)

	s.Require().NoError(s.runtime.ClearAccessState(s.ctx, "e13", []providers.AccessScope{testScope}))

	s.Nil(s.scopeEntry("e13", testScope), "the whole entry goes, lockCount included")
	s.Equal(float64(1), s.scopeEntry("e13", providers.AccessScopeOTP)["failureCount"],
		"another scope's lock must survive")
}

func (s *RuntimeStoreConstantsTestSuite) TestClearAccessStateRemovesSeveralScopes() {
	s.seed("e14", `{"accessState":{"lock":{"entity":{"lockCount":2},"authenticationMethods":{`+
		`"credential":{"lockCount":1},"otp":{"failureCount":1}}}}}`)

	s.Require().NoError(s.runtime.ClearAccessState(s.ctx, "e14",
		[]providers.AccessScope{testScope, identitygovernance.AccessScopeEntity}))

	s.Nil(s.scopeEntry("e14", testScope))
	s.Nil(s.scopeEntry("e14", identitygovernance.AccessScopeEntity))
	s.NotNil(s.scopeEntry("e14", providers.AccessScopeOTP))
}

// Clearing a scope with no entry succeeds and writes nothing.
func (s *RuntimeStoreConstantsTestSuite) TestClearAccessStateOnAnAbsentEntry() {
	s.seed("e15", `{"name":"kept"}`)

	s.Require().NoError(s.runtime.ClearAccessState(s.ctx, "e15",
		[]providers.AccessScope{testScope, identitygovernance.AccessScopeEntity}))
	s.Nil(s.scopeEntry("e15", testScope))
}

func (s *RuntimeStoreConstantsTestSuite) TestIncrementOnAMissingRowRecordsNothing() {
	snapshot, recorded, err := s.runtime.IncrementAccessFailure(
		s.ctx, "no-such-entity", testScope, time.Now().UTC(), time.Now().UTC().Add(-time.Minute))
	s.NoError(err)
	s.False(recorded)
	s.Equal(identitygovernance.ScopeLock{}, snapshot)
}

func (s *RuntimeStoreConstantsTestSuite) TestAccessStateWritesAreDeploymentScoped() {
	_, err := s.runtimeDB.Exec(`INSERT INTO "ENTITY_RUNTIME_DATA" ` +
		`(DEPLOYMENT_ID,ENTITY_ID,STATE,RUNTIME_ATTRIBUTES,CREATED_AT,UPDATED_AT) ` +
		`VALUES ('other-dep','e16','ACTIVE','{}','t','t')`)
	s.Require().NoError(err)
	now := time.Now().UTC()

	_, recorded, err := s.runtime.IncrementAccessFailure(s.ctx, "e16", testScope, now, now)
	s.Require().NoError(err)
	s.False(recorded)

	formed, err := s.runtime.FormAccessLock(s.ctx, "e16", testScope, identitygovernance.ScopeLock{},
		identitygovernance.LockEpisode{LockCount: 1, UnlockAt: "2026-09-08T10:15:00Z"}, now)
	s.Require().NoError(err)
	s.False(formed)

	s.Require().NoError(s.runtime.ClearAccessState(s.ctx, "e16", []providers.AccessScope{testScope}))
	s.Nil(s.scopeEntry("e16", testScope))
}

// A scope outside the registry is refused, since it becomes part of the JSON path.
func (s *RuntimeStoreConstantsTestSuite) TestUnregisteredScopeIsRefused() {
	s.seed("e17", `{"accessState":{"lock":{"authenticationMethods":{"credential":{"failureCount":2,"lockCount":1}}}}}`)
	now := time.Now().UTC()

	for _, scope := range []providers.AccessScope{"credential.unlockAt", "credential\"]", "", "notAScope"} {
		_, _, err := s.runtime.IncrementAccessFailure(s.ctx, "e17", scope, now, now)
		s.ErrorIs(err, errInvalidAccessScope, "increment must refuse %q", scope)

		_, err = s.runtime.FormAccessLock(s.ctx, "e17", scope, identitygovernance.ScopeLock{},
			identitygovernance.LockEpisode{LockCount: 1, UnlockAt: "2026-09-08T10:15:00Z"}, now)
		s.ErrorIs(err, errInvalidAccessScope, "formation must refuse %q", scope)

		err = s.runtime.ClearAccessState(s.ctx, "e17", []providers.AccessScope{scope})
		s.ErrorIs(err, errInvalidAccessScope, "clear must refuse %q", scope)
	}

	entry := s.scopeEntry("e17", testScope)
	s.Equal(float64(2), entry["failureCount"], "a refused scope must not have touched the document")
	s.Equal(float64(1), entry["lockCount"])
}

func (s *RuntimeStoreConstantsTestSuite) TestPermanentLockIsNeverExpired() {
	s.seed("e18", `{"accessState":{"lock":{"authenticationMethods":{"credential":{"failureCount":0,"lockCount":3,`+
		`"unlockAt":"9999-12-31T23:59:59Z"}}}}}`)
	now := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)

	_, recorded, err := s.runtime.IncrementAccessFailure(
		s.ctx, "e18", testScope, now, now.Add(-15*time.Minute))
	s.Require().NoError(err)
	s.False(recorded, "a permanent lock is live, so it suppresses the count like any other lock")
	s.Equal(float64(0), s.scopeEntry("e18", testScope)["failureCount"],
		"a permanent lock is not an expired episode, so nothing about it resets")

	formed, err := s.runtime.FormAccessLock(s.ctx, "e18", testScope,
		identitygovernance.ScopeLock{LockCount: 3, UnlockAt: "9999-12-31T23:59:59Z"},
		identitygovernance.LockEpisode{LockCount: 4, UnlockAt: "9999-12-31T23:59:59Z"}, now)
	s.Require().NoError(err)
	s.False(formed, "a permanent lock must block further episodes")
}

// suspendEntry reads the stored suspension member, or nil when there is none.
func (s *RuntimeStoreConstantsTestSuite) suspendEntry(id string) map[string]interface{} {
	doc := s.accessStateDoc(id)
	if doc == nil {
		return nil
	}
	entry, _ := doc["suspend"].(map[string]interface{})
	return entry
}

// Suspension writes only its own member and keeps the lock and last-login members.
func (s *RuntimeStoreConstantsTestSuite) TestSetAccessSuspensionLeavesTheRestAlone() {
	s.seed("b1", `{"name":"kept","accessState":{"lock":{"authenticationMethods":{"credential":`+
		`{"failureCount":2,"lockCount":1}}},"lastLoginAt":"2026-09-07T08:00:00Z"}`+`}`)
	at := time.Date(2026, 9, 8, 11, 0, 0, 0, time.UTC)

	s.True(s.suspend("b1", at, "Credential exposure"))

	s.Equal("2026-09-08T11:00:00Z", s.suspendEntry("b1")["suspendedAt"])
	s.Equal("Credential exposure", s.suspendEntry("b1")["operatorNote"])
	s.Equal(float64(2), s.scopeEntry("b1", testScope)["failureCount"], "the counter must survive")
	s.Equal("2026-09-07T08:00:00Z", s.accessStateDoc("b1")["lastLoginAt"])
}

// A first suspension creates the document; an empty note is stored as empty.
func (s *RuntimeStoreConstantsTestSuite) TestSetAccessSuspensionCreatesTheDocument() {
	s.seed("b2", nil)
	at := time.Date(2026, 9, 8, 11, 0, 0, 0, time.UTC)

	s.True(s.suspend("b2", at, ""))
	s.Equal("2026-09-08T11:00:00Z", s.suspendEntry("b2")["suspendedAt"])
	s.Equal("", s.suspendEntry("b2")["operatorNote"])
}

// Clearing without a hold removes only the suspension member.
func (s *RuntimeStoreConstantsTestSuite) TestClearAccessSuspensionWithoutAHold() {
	s.seed("b3", `{"accessState":{"suspend":{"suspendedAt":"2026-09-08T11:00:00Z"},`+
		`"lastLoginAt":"2026-09-07T08:00:00Z"}}`)

	s.True(s.release("b3", nil, "2026-09-08T11:00:00Z"))

	s.Nil(s.suspendEntry("b3"))
	s.Equal("2026-09-07T08:00:00Z", s.accessStateDoc("b3")["lastLoginAt"])
}

// Clearing with a hold removes the suspension and replaces the whole lock member in one statement.
func (s *RuntimeStoreConstantsTestSuite) TestClearAccessSuspensionWithAHoldReplacesTheLock() {
	s.seed("b4", `{"accessState":{"lock":{"authenticationMethods":{"credential":`+
		`{"failureCount":5,"lockCount":3,"unlockAt":"2026-09-08T10:15:00Z"}}},`+
		`"suspend":{"suspendedAt":"2026-09-08T11:00:00Z"},"lastLoginAt":"2026-09-07T08:00:00Z"}}`)

	s.True(s.release("b4", &identitygovernance.EntityHold{
		UnlockAt: "9999-12-31T23:59:59Z", Reason: "POST_SUSPENDED",
	}, "2026-09-08T11:00:00Z"))

	s.Nil(s.suspendEntry("b4"))
	s.Nil(s.scopeEntry("b4", testScope), "the authentication_method escalation goes with the replacement")

	entity := s.scopeEntry("b4", identitygovernance.AccessScopeEntity)
	s.Require().NotNil(entity)
	s.Equal("9999-12-31T23:59:59Z", entity["unlockAt"])
	s.Equal("POST_SUSPENDED", entity["reason"])
	s.Nil(entity["lockCount"], "nothing is counted under the hold, so no lock count is carried across")
	s.Equal("2026-09-07T08:00:00Z", s.accessStateDoc("b4")["lastLoginAt"])
}

// Nothing is counted while an entity-wide lock is live.
func (s *RuntimeStoreConstantsTestSuite) TestNoIncrementUnderALiveEntityHold() {
	s.seed("b5", `{"accessState":{"lock":{"entity":`+
		`{"unlockAt":"9999-12-31T23:59:59Z","reason":"POST_SUSPENDED"}}}}`)
	now := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)

	_, recorded, err := s.runtime.IncrementAccessFailure(
		s.ctx, "b5", testScope, now, now.Add(-15*time.Minute))
	s.Require().NoError(err)
	s.False(recorded, "the statement must match no row while the hold is live")
	s.Nil(s.scopeEntry("b5", testScope), "and it must write nothing")
}

func (s *RuntimeStoreConstantsTestSuite) TestALapsedEntityEntryDoesNotSuppressCounting() {
	s.seed("b6", `{"accessState":{"lock":{"entity":{"unlockAt":"2026-09-08T09:00:00Z"}}}}`)
	now := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)

	snapshot, recorded, err := s.runtime.IncrementAccessFailure(
		s.ctx, "b6", testScope, now, now.Add(-15*time.Minute))
	s.Require().NoError(err)
	s.True(recorded)
	s.Equal(1, snapshot.FailureCount)
}

func (s *RuntimeStoreConstantsTestSuite) TestClearAccessStateCannotReachSuspendOrLastLogin() {
	s.seed("b7", `{"accessState":{"lock":{"authenticationMethods":{"credential":{"lockCount":1}}},`+
		`"suspend":{"suspendedAt":"2026-09-08T11:00:00Z"},"lastLoginAt":"2026-09-07T08:00:00Z"}}`)

	s.Require().NoError(s.runtime.ClearAccessState(s.ctx, "b7",
		[]providers.AccessScope{testScope, identitygovernance.AccessScopeEntity}))

	s.Nil(s.scopeEntry("b7", testScope))
	s.Equal("2026-09-08T11:00:00Z", s.suspendEntry("b7")["suspendedAt"],
		"a suspension is a different dimension")
	s.Equal("2026-09-07T08:00:00Z", s.accessStateDoc("b7")["lastLoginAt"])
}

// suspend places a suspension and reports whether it was written.
func (s *RuntimeStoreConstantsTestSuite) suspend(id string, at time.Time, note string) bool {
	placed, err := s.runtime.SetAccessSuspension(s.ctx, id, at, note)
	s.Require().NoError(err)
	return placed
}

// release clears the suspension placed at suspendedAt and reports whether it was cleared.
func (s *RuntimeStoreConstantsTestSuite) release(id string, hold *identitygovernance.EntityHold,
	suspendedAt string) bool {
	cleared, err := s.runtime.ClearAccessSuspension(s.ctx, id, hold, suspendedAt)
	s.Require().NoError(err)
	return cleared
}

// A missing row writes nothing and is not an error.
func (s *RuntimeStoreConstantsTestSuite) TestSuspensionOnAMissingRowWritesNothing() {
	s.False(s.suspend("no-such-entity", time.Now().UTC(), ""))
	s.False(s.release("no-such-entity", nil, ""))
}

// A second suspend keeps the first placement time and note.
func (s *RuntimeStoreConstantsTestSuite) TestARepeatedSuspendKeepsTheFirstPlacement() {
	s.seed("repeat", nil)
	first := time.Date(2026, 9, 8, 11, 0, 0, 0, time.UTC)

	s.True(s.suspend("repeat", first, "first"))
	s.False(s.suspend("repeat", first.Add(time.Minute), "second"))

	s.Equal("2026-09-08T11:00:00Z", s.suspendEntry("repeat")["suspendedAt"])
	s.Equal("first", s.suspendEntry("repeat")["operatorNote"])
}

// A suspension recorded only in the state column also refuses a second placement.
func (s *RuntimeStoreConstantsTestSuite) TestASuspendedStateAloneRefusesASecondPlacement() {
	s.seed("state-only", nil)
	_, err := s.runtimeDB.Exec(`UPDATE "ENTITY_RUNTIME_DATA" SET STATE = 'SUSPENDED' WHERE ENTITY_ID = $1`,
		"state-only")
	s.Require().NoError(err)

	s.False(s.suspend("state-only", time.Now().UTC(), ""))
	s.Nil(s.suspendEntry("state-only"))
}

// A release guarded on an earlier suspension cannot clear a newer one.
func (s *RuntimeStoreConstantsTestSuite) TestStaleReleaseCannotClearNewSuspension() {
	s.seed("release-race", nil)
	old := time.Now().UTC()
	current := old.Add(time.Nanosecond)
	s.True(s.suspend("release-race", old, "first"))
	s.True(s.release("release-race", nil, old.Format(time.RFC3339Nano)))
	s.True(s.suspend("release-race", current, "second"))

	s.False(s.release("release-race", nil, old.Format(time.RFC3339Nano)))
	s.Equal("second", s.suspendEntry("release-race")["operatorNote"])
	s.True(s.release("release-race", nil, current.Format(time.RFC3339Nano)))
	s.Nil(s.suspendEntry("release-race"))
}

// Recording a login writes only lastLoginAt and keeps the lock and suspension members.
func (s *RuntimeStoreConstantsTestSuite) TestRecordAccessLoginLeavesTheRestAlone() {
	s.seed("l1", `{"name":"kept","accessState":{"lock":{"authenticationMethods":{"credential":`+
		`{"failureCount":2,"lockCount":1}}},"suspend":{"suspendedAt":"2026-09-08T11:00:00Z",`+
		`"operatorNote":"kept"},"lastLoginAt":"2026-09-07T08:00:00Z"}}`)
	at := time.Date(2026, 9, 9, 12, 30, 0, 0, time.UTC)

	s.Require().NoError(s.runtime.RecordAccessLogin(s.ctx, "l1", at, at.Add(-time.Hour)))

	s.Equal("2026-09-09T12:30:00Z", s.accessStateDoc("l1")["lastLoginAt"])
	s.Equal(float64(2), s.scopeEntry("l1", testScope)["failureCount"], "the counter must survive")
	s.Equal("2026-09-08T11:00:00Z", s.suspendEntry("l1")["suspendedAt"], "the hold must survive")
	s.Equal("kept", s.suspendEntry("l1")["operatorNote"])
}

func (s *RuntimeStoreConstantsTestSuite) TestRecordAccessLoginCreatesTheDocument() {
	s.seed("l2", nil)
	at := time.Date(2026, 9, 9, 12, 30, 0, 0, time.UTC)

	s.Require().NoError(s.runtime.RecordAccessLogin(s.ctx, "l2", at, at.Add(-time.Hour)))

	s.Equal("2026-09-09T12:30:00Z", s.accessStateDoc("l2")["lastLoginAt"])
}

// Only a stored value inside the window skips the write. An older value, a future value and a
// non-timestamp value are overwritten.
func (s *RuntimeStoreConstantsTestSuite) TestRecordAccessLoginAppliesTheWindow() {
	at := time.Date(2026, 9, 9, 12, 30, 0, 0, time.UTC)
	notBefore := at.Add(-time.Hour)
	stamp := "2026-09-09T12:30:00Z"

	for name, tc := range map[string]struct{ stored, want string }{
		"inside the window": {"2026-09-09T12:00:00Z", "2026-09-09T12:00:00Z"},
		"older than it":     {"2026-09-09T10:00:00Z", stamp},
		"ahead of now":      {"2026-09-09T18:00:00Z", stamp},
		"not a timestamp":   {"never", stamp},
	} {
		s.Run(name, func() {
			id := "w-" + name
			s.seed(id, `{"accessState":{"lastLoginAt":"`+tc.stored+`"}}`)

			s.Require().NoError(s.runtime.RecordAccessLogin(s.ctx, id, at, notBefore))

			s.Equal(tc.want, s.accessStateDoc(id)["lastLoginAt"])
		})
	}
}

// At resolution zero only a write of the identical value is skipped.
func (s *RuntimeStoreConstantsTestSuite) TestRecordAccessLoginWithNoResolutionWritesEveryTime() {
	at := time.Date(2026, 9, 9, 12, 30, 0, 0, time.UTC)
	s.seed("l3", `{"accessState":{"lastLoginAt":"2026-09-09T12:29:59Z"}}`)

	s.Require().NoError(s.runtime.RecordAccessLogin(s.ctx, "l3", at, at))

	s.Equal("2026-09-09T12:30:00Z", s.accessStateDoc("l3")["lastLoginAt"])
}

func (s *RuntimeStoreConstantsTestSuite) TestRecordAccessLoginOnAMissingRowIsNotAnError() {
	now := time.Now().UTC()
	s.Require().NoError(s.runtime.RecordAccessLogin(s.ctx, "no-such-entity", now, now.Add(-time.Hour)))
}

func (s *RuntimeStoreConstantsTestSuite) TestConditionalClearPreservesNewerFailures() {
	s.seed("e1", `{}`)
	now := time.Now().UTC()
	first, recorded, err := s.runtime.IncrementAccessFailure(s.ctx, "e1", testScope, now, now.Add(-time.Hour))
	s.Require().NoError(err)
	s.Require().True(recorded)
	second, _, err := s.runtime.IncrementAccessFailure(s.ctx, "e1", testScope, now, now.Add(-time.Hour))
	s.Require().NoError(err)

	cleared, err := s.runtime.ClearAccessStateIfUnchanged(s.ctx, "e1", testScope, first.Revision)
	s.Require().NoError(err)
	s.False(cleared, "a revision older than the row's is not cleared")
	s.Equal(float64(2), s.scopeEntry("e1", testScope)["failureCount"])
	cleared, err = s.runtime.ClearAccessStateIfUnchanged(s.ctx, "e1", testScope, second.Revision)
	s.Require().NoError(err)
	s.True(cleared)
	s.Nil(s.scopeEntry("e1", testScope))
}

func (s *RuntimeStoreConstantsTestSuite) TestConditionalClearPreservesNewerPostSuspensionHold() {
	s.seed("e1", `{"accessState":{"lock":{"entity":{"failureCount":1}}}}`)
	now := time.Now().UTC()
	s.written(s.runtime.SetAccessSuspension(s.ctx, "e1", now, "operator"))
	s.written(s.runtime.ClearAccessSuspension(s.ctx, "e1", &identitygovernance.EntityHold{
		UnlockAt: identitygovernance.PermanentUnlockAt, Reason: identitygovernance.LockReasonPostSuspension,
	}, now.Format(time.RFC3339Nano)))

	_, err := s.runtime.ClearAccessStateIfUnchanged(s.ctx, "e1", identitygovernance.AccessScopeEntity, 0)
	s.Require().NoError(err)
	s.Equal(identitygovernance.PermanentUnlockAt, s.scopeEntry("e1", identitygovernance.AccessScopeEntity)["unlockAt"])
	// Explicit recovery/operator release remains unconditional.
	s.Require().NoError(s.runtime.ClearAccessState(s.ctx, "e1", identitygovernance.AllAccessScopes))
	s.Nil(s.scopeEntry("e1", identitygovernance.AccessScopeEntity))
}

func (s *RuntimeStoreConstantsTestSuite) TestConditionalClearIsDeploymentScoped() {
	s.seed("e1", `{"accessState":{"lock":{"entity":{"failureCount":1}}}}`)
	_, err := s.runtimeDB.Exec(`INSERT INTO "ENTITY_RUNTIME_DATA"
		(DEPLOYMENT_ID, ENTITY_ID, STATE, RUNTIME_ATTRIBUTES, CREATED_AT, UPDATED_AT)
		SELECT 'other-deployment', ENTITY_ID, STATE, RUNTIME_ATTRIBUTES, CREATED_AT, UPDATED_AT
		FROM "ENTITY_RUNTIME_DATA" WHERE ENTITY_ID = 'e1'`)
	s.Require().NoError(err)
	_, err = s.runtime.ClearAccessStateIfUnchanged(s.ctx, "e1", identitygovernance.AccessScopeEntity, 0)
	s.Require().NoError(err)
	var count int
	s.Require().NoError(s.runtimeDB.QueryRow(
		`SELECT json_extract(RUNTIME_ATTRIBUTES, '$.accessState.lock.entity.failureCount')
		FROM "ENTITY_RUNTIME_DATA" WHERE DEPLOYMENT_ID = 'other-deployment' AND ENTITY_ID = 'e1'`).Scan(&count))
	s.Equal(1, count)
}

func BenchmarkAccessStateQueries(b *testing.B) {
	b.Run("increment", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := buildIncrementAccessFailureQuery(testScope); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("formation", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := buildFormAccessLockQuery(testScope); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// written requires that a guarded suspension write succeeded and applied.
func (s *RuntimeStoreConstantsTestSuite) written(applied bool, err error) {
	s.T().Helper()
	s.Require().NoError(err)
	s.Require().True(applied)
}

// Every runtime statement has a distinct ID, numbered in the order the statements are declared.
func (s *RuntimeStoreConstantsTestSuite) TestQueryIDsAreDistinctAndAscending() {
	getQuery, _ := buildGetRuntimeDataQuery([]string{"e1"}, testDeploymentID)
	initQuery, _ := buildInitializeRuntimeDataQuery([]entityRuntimeData{{ID: "e1"}}, time.Now(), testDeploymentID)
	clearQuery, _, err := buildClearAccessStateQuery("e1", []providers.AccessScope{testScope},
		time.Now(), nil, testDeploymentID)
	s.Require().NoError(err)
	revision := int64(1)
	guardedClearQuery, _, err := buildClearAccessStateQuery("e1", []providers.AccessScope{testScope},
		time.Now(), &revision, testDeploymentID)
	s.Require().NoError(err)
	incrementQuery, err := buildIncrementAccessFailureQuery(testScope)
	s.Require().NoError(err)
	formQuery, err := buildFormAccessLockQuery(testScope)
	s.Require().NoError(err)

	queries := []model.DBQuery{queryResetRuntimeData, queryDeleteRuntimeData, querySetAccessSuspension,
		queryClearAccessSuspension, queryClearAccessSuspensionWithHold, queryRecordAccessLogin, getQuery,
		initQuery, clearQuery, guardedClearQuery, incrementQuery, formQuery}
	for i, query := range queries {
		s.Equal(fmt.Sprintf("ERD-ENTITY_MGT-%02d", i+1), query.ID)
	}
}

// Runtime statements name only the runtime table, never the profile table, so a runtime write cannot
// touch a profile row and a profile update cannot reach the runtime row.
func (s *RuntimeStoreConstantsTestSuite) TestRuntimeStatementsNeverNameTheProfileTable() {
	getQuery, _ := buildGetRuntimeDataQuery([]string{"e1"}, testDeploymentID)
	initQuery, _ := buildInitializeRuntimeDataQuery([]entityRuntimeData{{ID: "e1"}}, time.Now(), testDeploymentID)
	revision := int64(1)
	clearQuery, _, err := buildClearAccessStateQuery("e1", []providers.AccessScope{testScope},
		time.Now(), nil, testDeploymentID)
	s.Require().NoError(err)
	guardedClearQuery, _, err := buildClearAccessStateQuery("e1", []providers.AccessScope{testScope},
		time.Now(), &revision, testDeploymentID)
	s.Require().NoError(err)

	queries := make([]model.DBQuery, 0, 10+2*len(identitygovernance.AllAccessScopes))
	queries = append(queries, queryResetRuntimeData, queryDeleteRuntimeData, querySetAccessSuspension,
		queryClearAccessSuspension, queryClearAccessSuspensionWithHold, queryRecordAccessLogin, getQuery,
		initQuery, clearQuery, guardedClearQuery)
	for _, scope := range identitygovernance.AllAccessScopes {
		increment, err := scopeQuery(incrementAccessQueries, scope)
		s.Require().NoError(err)
		form, err := scopeQuery(formAccessQueries, scope)
		s.Require().NoError(err)
		queries = append(queries, increment, form)
	}

	profileTable := regexp.MustCompile(`(?i)"ENTITY"|\b(from|join|update|into)\s+ENTITY\b`)
	for _, query := range queries {
		for _, dialect := range []string{"postgres", "sqlite"} {
			statement := query.GetQuery(dialect)
			s.Contains(statement, `"ENTITY_RUNTIME_DATA"`, "%s (%s)", query.ID, dialect)
			s.False(profileTable.MatchString(statement), "%s (%s) names the profile table: %s",
				query.ID, dialect, statement)
		}
	}
}

// Statements whose dialects differ set both forms and no generic one.
func (s *RuntimeStoreConstantsTestSuite) TestDialectSpecificStatementsSetBothForms() {
	clearQuery, _, err := buildClearAccessStateQuery("e1", []providers.AccessScope{testScope},
		time.Now(), nil, testDeploymentID)
	s.Require().NoError(err)
	incrementQuery, err := buildIncrementAccessFailureQuery(testScope)
	s.Require().NoError(err)
	formQuery, err := buildFormAccessLockQuery(testScope)
	s.Require().NoError(err)

	for _, query := range []model.DBQuery{querySetAccessSuspension, queryClearAccessSuspension,
		queryClearAccessSuspensionWithHold, queryRecordAccessLogin, clearQuery, incrementQuery, formQuery} {
		s.Empty(query.Query, query.ID)
		s.NotEmpty(query.PostgresQuery, query.ID)
		s.NotEmpty(query.SQLiteQuery, query.ID)
		s.Equal(query.PostgresQuery, query.GetQuery("postgres"), query.ID)
		s.Equal(query.SQLiteQuery, query.GetQuery("sqlite"), query.ID)
	}
}

func (s *RuntimeStoreConstantsTestSuite) TestResetRuntimeData_PostgresQuery() {
	q := queryResetRuntimeData.GetQuery("postgres")
	s.Contains(q, `INSERT INTO "ENTITY_RUNTIME_DATA"`)
	s.Contains(q, `VALUES ($1, $2, '{}', $3, $3, $4)`)
	s.Contains(q, `ON CONFLICT (DEPLOYMENT_ID, ENTITY_ID) DO UPDATE SET`)
	s.Contains(q, `RUNTIME_ATTRIBUTES = '{}'`)
	s.Contains(q, `REVISION = "ENTITY_RUNTIME_DATA".REVISION + 1`)
}

func (s *RuntimeStoreConstantsTestSuite) TestDeleteRuntimeData_PostgresQuery() {
	s.Equal(`DELETE FROM "ENTITY_RUNTIME_DATA" WHERE ENTITY_ID = $1 AND DEPLOYMENT_ID = $2`,
		queryDeleteRuntimeData.GetQuery("postgres"))
}

func (s *RuntimeStoreConstantsTestSuite) TestSetAccessSuspension_PostgresQuery() {
	q := querySetAccessSuspension.PostgresQuery
	s.Contains(q, `SET STATE = 'SUSPENDED'`)
	s.Contains(q, `jsonb_build_object('suspend', `+
		`jsonb_build_object('suspendedAt', $2::text, 'operatorNote', $3::text))`)
	s.Contains(q, `COALESCE(RUNTIME_ATTRIBUTES->'accessState', '{}'::jsonb)`, "the other members are kept")
	s.Contains(q, `UPDATED_AT = $4`)
	s.Contains(q, `STATE <> 'SUSPENDED'`, "an existing suspension is not replaced")
	s.Contains(q, `RUNTIME_ATTRIBUTES->'accessState'->'suspend' IS NULL`)
	s.True(strings.HasSuffix(q, `AND DEPLOYMENT_ID = $5`))
}

func (s *RuntimeStoreConstantsTestSuite) TestClearAccessSuspension_PostgresQuery() {
	q := queryClearAccessSuspension.PostgresQuery
	s.Contains(q, `SET STATE = 'ACTIVE'`)
	s.Contains(q, `COALESCE(RUNTIME_ATTRIBUTES->'accessState', '{}'::jsonb) - 'suspend'`)
	s.Contains(q, `COALESCE(RUNTIME_ATTRIBUTES->'accessState'->'suspend'->>'suspendedAt', '') = $3`)
	s.True(strings.HasSuffix(q, `AND DEPLOYMENT_ID = $4`))
}

func (s *RuntimeStoreConstantsTestSuite) TestClearAccessSuspensionWithHold_PostgresQuery() {
	q := queryClearAccessSuspensionWithHold.PostgresQuery
	s.Contains(q, `SET STATE = 'ACTIVE'`)
	s.Contains(q, `(COALESCE(RUNTIME_ATTRIBUTES->'accessState', '{}'::jsonb) - 'suspend')`)
	s.Contains(q, `jsonb_build_object('lock', jsonb_build_object('entity', `+
		`jsonb_build_object('unlockAt', $2::text, 'reason', $3::text)))`)
	s.Contains(q, `COALESCE(RUNTIME_ATTRIBUTES->'accessState'->'suspend'->>'suspendedAt', '') = $5`)
	s.True(strings.HasSuffix(q, `AND DEPLOYMENT_ID = $6`))
}

func (s *RuntimeStoreConstantsTestSuite) TestRecordAccessLogin_PostgresQuery() {
	q := queryRecordAccessLogin.PostgresQuery
	s.Contains(q, `jsonb_build_object('lastLoginAt', $2::text)`)
	s.Contains(q, `COALESCE(RUNTIME_ATTRIBUTES->'accessState'->>'lastLoginAt', '') >= $3::text`)
	s.Contains(q, `COALESCE(RUNTIME_ATTRIBUTES->'accessState'->>'lastLoginAt', '') <= $2::text`)
	s.True(strings.HasSuffix(q, `AND DEPLOYMENT_ID = $5`))
}

func (s *RuntimeStoreConstantsTestSuite) TestGetRuntimeData_PostgresQuery() {
	q, args := buildGetRuntimeDataQuery([]string{"e1", "e2"}, testDeploymentID)

	s.Equal(`SELECT ENTITY_ID, STATE, RUNTIME_ATTRIBUTES, REVISION FROM "ENTITY_RUNTIME_DATA" `+
		`WHERE ENTITY_ID IN ($1,$2) AND DEPLOYMENT_ID = $3`, q.GetQuery("postgres"))
	s.Equal([]interface{}{"e1", "e2", testDeploymentID}, args)
}

func (s *RuntimeStoreConstantsTestSuite) TestInitializeRuntimeData_PostgresQuery() {
	now := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)
	q, args := buildInitializeRuntimeDataQuery([]entityRuntimeData{
		{ID: "e1", State: "ACTIVE", Attributes: []byte(`{}`)},
		{ID: "e2", State: "SUSPENDED", Attributes: []byte(`{}`)},
	}, now, testDeploymentID)

	pg := q.GetQuery("postgres")
	s.Contains(pg, `VALUES ($1,$2,$3,$4,$4,$9),($5,$6,$7,$8,$8,$9)`)
	s.True(strings.HasSuffix(pg, `ON CONFLICT (DEPLOYMENT_ID, ENTITY_ID) DO NOTHING`))
	s.Len(args, 9)
	s.Equal(testDeploymentID, args[8])
}

func (s *RuntimeStoreConstantsTestSuite) TestClearAccessState_PostgresQuery() {
	q, args, err := buildClearAccessStateQuery("e1",
		[]providers.AccessScope{testScope, identitygovernance.AccessScopeEntity},
		time.Now(), nil, testDeploymentID)
	s.Require().NoError(err)

	s.Contains(q.PostgresQuery, ` #- ARRAY['accessState','lock','authenticationMethods','credential']::text[]`)
	s.Contains(q.PostgresQuery, ` #- ARRAY['accessState','lock','entity']::text[]`)
	s.True(strings.HasSuffix(q.PostgresQuery, `WHERE ENTITY_ID = $2 AND DEPLOYMENT_ID = $3`))
	s.Len(args, 3)
}

func (s *RuntimeStoreConstantsTestSuite) TestClearAccessStateIfUnchanged_PostgresQuery() {
	revision := int64(7)
	q, args, err := buildClearAccessStateQuery("e1", []providers.AccessScope{testScope},
		time.Now(), &revision, testDeploymentID)
	s.Require().NoError(err)

	s.True(strings.HasSuffix(q.PostgresQuery, `WHERE ENTITY_ID = $2 AND REVISION = $3 AND DEPLOYMENT_ID = $4`))
	s.Equal(int64(7), args[2])
	s.Equal(testDeploymentID, args[3])
}

func (s *RuntimeStoreConstantsTestSuite) TestIncrementAccessFailure_PostgresQuery() {
	for _, scope := range []providers.AccessScope{testScope, identitygovernance.AccessScopeEntity} {
		q, err := buildIncrementAccessFailureQuery(scope)
		s.Require().NoError(err)
		pg := q.PostgresQuery

		s.Contains(pg, `'failureCount', CASE WHEN`)
		s.Contains(pg, `< $2::timestamptz THEN 1 ELSE`, "a failure outside the window restarts the count")
		s.Contains(pg, `'lastFailedAt', $4::text`)
		s.Contains(pg, `COALESCE((RUNTIME_ATTRIBUTES->'accessState'->'lock'->'entity'->>'unlockAt')::timestamptz, `+
			`'epoch'::timestamptz) <= $3::timestamptz`, "a live entity-wide lock blocks counting")
		s.Contains(pg, `AND DEPLOYMENT_ID = $6 RETURNING`)
		s.Contains(pg, `AS FAILURE_COUNT`)
		s.Contains(pg, `AS LOCK_COUNT`)
		s.Contains(pg, `AS UNLOCK_AT, REVISION`)
	}
	q, err := buildIncrementAccessFailureQuery(testScope)
	s.Require().NoError(err)
	s.Contains(q.PostgresQuery, `->'authenticationMethods'->'credential'->>'failureCount'`)
}

func (s *RuntimeStoreConstantsTestSuite) TestFormAccessLock_PostgresQuery() {
	q, err := buildFormAccessLockQuery(testScope)
	s.Require().NoError(err)
	pg := q.PostgresQuery

	s.Contains(pg, `'failureCount', 0, 'lockCount', $3::int, 'unlockAt', $4::text`)
	s.Contains(pg, `WHERE ENTITY_ID = $1 AND REVISION = $8`)
	s.Contains(pg, `->>'failureCount')::int, 0) = $5::int`)
	s.Contains(pg, `->>'lockCount')::int, 0) = $6::int`)
	s.Contains(pg, `'epoch'::timestamptz) <= $2::timestamptz`)
	s.True(strings.HasSuffix(pg, `AND DEPLOYMENT_ID = $9`))
}

// A scope outside the registry never becomes statement text, in either dialect.
func (s *RuntimeStoreConstantsTestSuite) TestAnUnknownScopeBuildsNoStatement() {
	_, err := buildIncrementAccessFailureQuery("x'; DROP TABLE")
	s.ErrorIs(err, errInvalidAccessScope)
	_, err = buildFormAccessLockQuery("x'; DROP TABLE")
	s.ErrorIs(err, errInvalidAccessScope)
	_, _, err = buildClearAccessStateQuery("e1", []providers.AccessScope{"x'; DROP TABLE"},
		time.Now(), nil, testDeploymentID)
	s.ErrorIs(err, errInvalidAccessScope)
}

// Profile reads and writes no longer carry the lifecycle state, which lives in the runtime row.
func (s *RuntimeStoreConstantsTestSuite) TestProfileStatementsDoNotCarryTheState() {
	for _, query := range []model.DBQuery{QueryGetEntityList, QuerySearchEntityList, QueryCreateEntity,
		QueryGetEntityByID, QueryUpdateEntity, QueryGetEntityWithCredentials} {
		s.NotContains(query.GetQuery("postgres"), "STATE", query.ID)
		s.NotContains(query.GetQuery("sqlite"), "STATE", query.ID)
	}
}
