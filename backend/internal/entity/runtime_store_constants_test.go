// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	governancemodel "github.com/thunder-id/thunderid/internal/identitygovernance/model"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/deployment"
)

// AccessStateStoreTestSuite runs the access-state statements against a real SQLite database,
// so counting and the compare-and-swap guards are exercised by the database itself.
type AccessStateStoreTestSuite struct {
	suite.Suite
	*sqliteHarness
	ctx context.Context
}

func TestAccessStateStoreTestSuite(t *testing.T) {
	suite.Run(t, new(AccessStateStoreTestSuite))
}

func (s *AccessStateStoreTestSuite) SetupTest() {
	s.sqliteHarness = newSQLiteHarness(s.T(), strings.ReplaceAll(s.T().Name(), "/", "-"))
	s.ctx = deployment.WithID(context.Background(), harnessDeploymentID)
}

func (s *AccessStateStoreTestSuite) seed(id string, systemAttrs interface{}) {
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
func (s *AccessStateStoreTestSuite) accessStateDoc(id string) map[string]interface{} {
	var raw string
	s.Require().NoError(s.runtimeDB.QueryRow(
		`SELECT RUNTIME_ATTRIBUTES FROM "ENTITY_RUNTIME_DATA" WHERE ENTITY_ID=$1`, id).Scan(&raw))
	if raw == "" {
		return nil
	}
	attrs := map[string]interface{}{}
	s.Require().NoError(json.Unmarshal([]byte(raw), &attrs))
	doc, _ := attrs[governancemodel.SystemAttrAccessState].(map[string]interface{})
	return doc
}

// scopeEntry reads one scope's stored lock entry, or nil when it has none. The entity scope is at
// lock.entity and others at lock.authenticationMethods.<scope>.
func (s *AccessStateStoreTestSuite) scopeEntry(id string,
	scope governancemodel.AccessScope) map[string]interface{} {
	doc := s.accessStateDoc(id)
	if doc == nil {
		return nil
	}
	lock, ok := doc["lock"].(map[string]interface{})
	if !ok {
		return nil
	}
	if scope == governancemodel.AccessScopeEntity {
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

const testScope = governancemodel.AccessScopeCredential

// A first failure creates the nested document.
func (s *AccessStateStoreTestSuite) TestFirstFailureCreatesTheEntry() {
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

func (s *AccessStateStoreTestSuite) TestFailuresInsideTheWindowAccumulate() {
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

func (s *AccessStateStoreTestSuite) TestFailureOutsideTheWindowRestartsTheCount() {
	s.seed("e3", `{"accessState":{"lock":{"authenticationMethods":{"credential":`+
		`{"failureCount":4,"lastFailedAt":"2026-09-08T09:00:00Z"}}}}}`)
	now := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)

	snapshot, _, err := s.runtime.IncrementAccessFailure(
		s.ctx, "e3", testScope, now, now.Add(-15*time.Minute))
	s.Require().NoError(err)
	s.Equal(1, snapshot.FailureCount, "a failure an hour later starts a new window")
}

// The first failure after a lock lapses restarts the count at 1.
func (s *AccessStateStoreTestSuite) TestFirstFailureAfterExpiryResetsTheCount() {
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
func (s *AccessStateStoreTestSuite) TestExpiryResetHappensOnlyOnce() {
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
func (s *AccessStateStoreTestSuite) TestFailuresDuringALiveLockAreNotCounted() {
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

func (s *AccessStateStoreTestSuite) TestALockOnOneMethodStillCountsAnother() {
	s.seed("e6b", `{"accessState":{"lock":{"authenticationMethods":{"credential":{"failureCount":0,"lockCount":1,`+
		`"lastFailedAt":"2026-09-08T10:00:00Z","unlockAt":"2026-09-08T10:15:00Z"}}}}}`)
	now := time.Date(2026, 9, 8, 10, 5, 0, 0, time.UTC)

	snapshot, recorded, err := s.runtime.IncrementAccessFailure(
		s.ctx, "e6b", governancemodel.AccessScopeOTP, now, now.Add(-15*time.Minute))
	s.Require().NoError(err)
	s.True(recorded, "a lock on another method must not suppress this one")
	s.Equal(1, snapshot.FailureCount)

	held := s.scopeEntry("e6b", testScope)
	s.Equal(float64(1), held["lockCount"], "the held method's own entry must be untouched")
}

func (s *AccessStateStoreTestSuite) TestConcurrentFailuresCountSeparately() {
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
func (s *AccessStateStoreTestSuite) TestFormAccessLockOpensTheEpisode() {
	s.seed("e8", `{"accessState":{"lock":{"authenticationMethods":{"credential":{"failureCount":5,"lockCount":1,`+
		`"lastFailedAt":"2026-09-08T10:00:00Z"}}}}}`)
	now := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)

	formed, err := s.runtime.FormAccessLock(s.ctx, "e8", testScope,
		governancemodel.ScopeLock{FailureCount: 5, LockCount: 1},
		governancemodel.LockEpisode{LockCount: 2, UnlockAt: "2026-09-08T10:15:00Z"}, now)
	s.Require().NoError(err)
	s.True(formed)

	entry := s.scopeEntry("e8", testScope)
	s.Equal(float64(0), entry["failureCount"], "the count returns to zero when a lock forms")
	s.Equal(float64(2), entry["lockCount"])
	s.Equal("2026-09-08T10:15:00Z", entry["unlockAt"])
}

func (s *AccessStateStoreTestSuite) TestFormAccessLockRefusesAStaleFailureCount() {
	s.seed("e9", `{"accessState":{"lock":{"authenticationMethods":{"credential":{"failureCount":6,"lockCount":1}}}}}`)
	now := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)

	formed, err := s.runtime.FormAccessLock(s.ctx, "e9", testScope,
		governancemodel.ScopeLock{FailureCount: 5, LockCount: 1},
		governancemodel.LockEpisode{LockCount: 2, UnlockAt: "2026-09-08T10:15:00Z"}, now)
	s.Require().NoError(err)
	s.False(formed)
	s.Nil(s.scopeEntry("e9", testScope)["unlockAt"])
}

// A clear between the increment and the formation prevents the lock.
func (s *AccessStateStoreTestSuite) TestFormAccessLockRefusesAfterAConcurrentReset() {
	s.seed("e10", `{"accessState":{"lock":{"authenticationMethods":{"credential":{"failureCount":5,"lockCount":1}}}}}`)
	now := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)

	s.Require().NoError(s.runtime.ClearAccessState(s.ctx, "e10", []governancemodel.AccessScope{testScope}))

	formed, err := s.runtime.FormAccessLock(s.ctx, "e10", testScope,
		governancemodel.ScopeLock{FailureCount: 5, LockCount: 1},
		governancemodel.LockEpisode{LockCount: 2, UnlockAt: "2026-09-08T10:15:00Z"}, now)
	s.Require().NoError(err)
	s.False(formed, "a reset between the increment and the formation must win")
	s.Nil(s.scopeEntry("e10", testScope))
}

// A live lock blocks a second episode whatever the count.
func (s *AccessStateStoreTestSuite) TestFormAccessLockRefusesWhileALockIsLive() {
	s.seed("e11", `{"accessState":{"lock":{"authenticationMethods":{"credential":{"failureCount":5,"lockCount":1,`+
		`"unlockAt":"2026-09-08T10:15:00Z"}}}}}`)
	now := time.Date(2026, 9, 8, 10, 5, 0, 0, time.UTC)

	formed, err := s.runtime.FormAccessLock(s.ctx, "e11", testScope,
		governancemodel.ScopeLock{FailureCount: 5, LockCount: 1, UnlockAt: "2026-09-08T10:15:00Z"},
		governancemodel.LockEpisode{LockCount: 2, UnlockAt: "2026-09-08T10:30:00Z"}, now)
	s.Require().NoError(err)
	s.False(formed)
	s.Equal("2026-09-08T10:15:00Z", s.scopeEntry("e11", testScope)["unlockAt"],
		"the live episode keeps the duration it was given")
}

// Two threshold crossings advance the lock count once.
func (s *AccessStateStoreTestSuite) TestConcurrentThresholdCrossingsAdvanceOneLock() {
	s.seed("e12", `{"accessState":{"lock":{"authenticationMethods":{"credential":{"failureCount":5,"lockCount":1}}}}}`)
	now := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)
	observed := governancemodel.ScopeLock{FailureCount: 5, LockCount: 1}

	var wg sync.WaitGroup
	formedCount := 0
	var mu sync.Mutex
	for i := range 2 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			unlockAt := []string{"2026-09-08T10:15:00Z", "2026-09-08T10:30:00Z"}[idx]
			formed, err := s.runtime.FormAccessLock(s.ctx, "e12", testScope, observed,
				governancemodel.LockEpisode{LockCount: 2, UnlockAt: unlockAt}, now)
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

func (s *AccessStateStoreTestSuite) TestClearAccessStateRemovesTheEntry() {
	s.seed("e13", `{"name":"kept","accessState":{"lock":{"authenticationMethods":{"credential":`+
		`{"failureCount":5,"lockCount":3,"unlockAt":"2026-09-08T10:15:00Z"},`+
		`"otp":{"failureCount":1}}}}}`)

	s.Require().NoError(s.runtime.ClearAccessState(s.ctx, "e13", []governancemodel.AccessScope{testScope}))

	s.Nil(s.scopeEntry("e13", testScope), "the whole entry goes, lockCount included")
	s.Equal(float64(1), s.scopeEntry("e13", governancemodel.AccessScopeOTP)["failureCount"],
		"another scope's lock must survive")
}

func (s *AccessStateStoreTestSuite) TestClearAccessStateRemovesSeveralScopes() {
	s.seed("e14", `{"accessState":{"lock":{"entity":{"lockCount":2},"authenticationMethods":{`+
		`"credential":{"lockCount":1},"otp":{"failureCount":1}}}}}`)

	s.Require().NoError(s.runtime.ClearAccessState(s.ctx, "e14",
		[]governancemodel.AccessScope{testScope, governancemodel.AccessScopeEntity}))

	s.Nil(s.scopeEntry("e14", testScope))
	s.Nil(s.scopeEntry("e14", governancemodel.AccessScopeEntity))
	s.NotNil(s.scopeEntry("e14", governancemodel.AccessScopeOTP))
}

// Clearing a scope with no entry succeeds and writes nothing.
func (s *AccessStateStoreTestSuite) TestClearAccessStateOnAnAbsentEntry() {
	s.seed("e15", `{"name":"kept"}`)

	s.Require().NoError(s.runtime.ClearAccessState(s.ctx, "e15",
		[]governancemodel.AccessScope{testScope, governancemodel.AccessScopeEntity}))
	s.Nil(s.scopeEntry("e15", testScope))
}

func (s *AccessStateStoreTestSuite) TestIncrementOnAMissingRowRecordsNothing() {
	snapshot, recorded, err := s.runtime.IncrementAccessFailure(
		s.ctx, "no-such-entity", testScope, time.Now().UTC(), time.Now().UTC().Add(-time.Minute))
	s.NoError(err)
	s.False(recorded)
	s.Equal(governancemodel.ScopeLock{}, snapshot)
}

func (s *AccessStateStoreTestSuite) TestAccessStateWritesAreDeploymentScoped() {
	_, err := s.runtimeDB.Exec(`INSERT INTO "ENTITY_RUNTIME_DATA" ` +
		`(DEPLOYMENT_ID,ENTITY_ID,STATE,RUNTIME_ATTRIBUTES,CREATED_AT,UPDATED_AT) ` +
		`VALUES ('other-dep','e16','ACTIVE','{}','t','t')`)
	s.Require().NoError(err)
	now := time.Now().UTC()

	_, recorded, err := s.runtime.IncrementAccessFailure(s.ctx, "e16", testScope, now, now)
	s.Require().NoError(err)
	s.False(recorded)

	formed, err := s.runtime.FormAccessLock(s.ctx, "e16", testScope, governancemodel.ScopeLock{},
		governancemodel.LockEpisode{LockCount: 1, UnlockAt: "2026-09-08T10:15:00Z"}, now)
	s.Require().NoError(err)
	s.False(formed)

	s.Require().NoError(s.runtime.ClearAccessState(s.ctx, "e16", []governancemodel.AccessScope{testScope}))
	s.Nil(s.scopeEntry("e16", testScope))
}

// A scope outside the registry is refused, since it becomes part of the JSON path.
func (s *AccessStateStoreTestSuite) TestUnregisteredScopeIsRefused() {
	s.seed("e17", `{"accessState":{"lock":{"authenticationMethods":{"credential":{"failureCount":2,"lockCount":1}}}}}`)
	now := time.Now().UTC()

	for _, scope := range []governancemodel.AccessScope{"credential.unlockAt", "credential\"]", "", "notAScope"} {
		_, _, err := s.runtime.IncrementAccessFailure(s.ctx, "e17", scope, now, now)
		s.ErrorIs(err, errInvalidAccessScope, "increment must refuse %q", scope)

		_, err = s.runtime.FormAccessLock(s.ctx, "e17", scope, governancemodel.ScopeLock{},
			governancemodel.LockEpisode{LockCount: 1, UnlockAt: "2026-09-08T10:15:00Z"}, now)
		s.ErrorIs(err, errInvalidAccessScope, "formation must refuse %q", scope)

		err = s.runtime.ClearAccessState(s.ctx, "e17", []governancemodel.AccessScope{scope})
		s.ErrorIs(err, errInvalidAccessScope, "clear must refuse %q", scope)
	}

	entry := s.scopeEntry("e17", testScope)
	s.Equal(float64(2), entry["failureCount"], "a refused scope must not have touched the document")
	s.Equal(float64(1), entry["lockCount"])
}

func (s *AccessStateStoreTestSuite) TestPermanentLockIsNeverExpired() {
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
		governancemodel.ScopeLock{LockCount: 3, UnlockAt: "9999-12-31T23:59:59Z"},
		governancemodel.LockEpisode{LockCount: 4, UnlockAt: "9999-12-31T23:59:59Z"}, now)
	s.Require().NoError(err)
	s.False(formed, "a permanent lock must block further episodes")
}

// suspendEntry reads the stored suspension member, or nil when there is none.
func (s *AccessStateStoreTestSuite) suspendEntry(id string) map[string]interface{} {
	doc := s.accessStateDoc(id)
	if doc == nil {
		return nil
	}
	entry, _ := doc["suspend"].(map[string]interface{})
	return entry
}

// Suspension writes only its own member and keeps the lock and last-login members.
func (s *AccessStateStoreTestSuite) TestSetAccessSuspensionLeavesTheRestAlone() {
	s.seed("b1", `{"name":"kept","accessState":{"lock":{"authenticationMethods":{"credential":`+
		`{"failureCount":2,"lockCount":1}}},"lastLoginAt":"2026-09-07T08:00:00Z"}`+`}`)
	at := time.Date(2026, 9, 8, 11, 0, 0, 0, time.UTC)

	s.Require().NoError(s.runtime.SetAccessSuspension(s.ctx, "b1", at, "Credential exposure"))

	s.Equal("2026-09-08T11:00:00Z", s.suspendEntry("b1")["suspendedAt"])
	s.Equal("Credential exposure", s.suspendEntry("b1")["operatorNote"])
	s.Equal(float64(2), s.scopeEntry("b1", testScope)["failureCount"], "the counter must survive")
	s.Equal("2026-09-07T08:00:00Z", s.accessStateDoc("b1")["lastLoginAt"])
}

// A first suspension creates the document; an empty note is stored as empty.
func (s *AccessStateStoreTestSuite) TestSetAccessSuspensionCreatesTheDocument() {
	s.seed("b2", nil)
	at := time.Date(2026, 9, 8, 11, 0, 0, 0, time.UTC)

	s.Require().NoError(s.runtime.SetAccessSuspension(s.ctx, "b2", at, ""))
	s.Equal("2026-09-08T11:00:00Z", s.suspendEntry("b2")["suspendedAt"])
	s.Equal("", s.suspendEntry("b2")["operatorNote"])
}

// Clearing without a hold removes only the suspension member.
func (s *AccessStateStoreTestSuite) TestClearAccessSuspensionWithoutAHold() {
	s.seed("b3", `{"accessState":{"suspend":{"suspendedAt":"2026-09-08T11:00:00Z"},`+
		`"lastLoginAt":"2026-09-07T08:00:00Z"}}`)

	s.Require().NoError(s.runtime.ClearAccessSuspension(s.ctx, "b3", nil, "2026-09-08T11:00:00Z"))

	s.Nil(s.suspendEntry("b3"))
	s.Equal("2026-09-07T08:00:00Z", s.accessStateDoc("b3")["lastLoginAt"])
}

// Clearing with a hold removes the suspension and replaces the whole lock member in one statement.
func (s *AccessStateStoreTestSuite) TestClearAccessSuspensionWithAHoldReplacesTheLock() {
	s.seed("b4", `{"accessState":{"lock":{"authenticationMethods":{"credential":`+
		`{"failureCount":5,"lockCount":3,"unlockAt":"2026-09-08T10:15:00Z"}}},`+
		`"suspend":{"suspendedAt":"2026-09-08T11:00:00Z"},"lastLoginAt":"2026-09-07T08:00:00Z"}}`)

	s.Require().NoError(s.runtime.ClearAccessSuspension(s.ctx, "b4", &governancemodel.EntityHold{
		UnlockAt: "9999-12-31T23:59:59Z", Reason: "POST_SUSPENDED",
	}, "2026-09-08T11:00:00Z"))

	s.Nil(s.suspendEntry("b4"))
	s.Nil(s.scopeEntry("b4", testScope), "the authentication_method escalation goes with the replacement")

	entity := s.scopeEntry("b4", governancemodel.AccessScopeEntity)
	s.Require().NotNil(entity)
	s.Equal("9999-12-31T23:59:59Z", entity["unlockAt"])
	s.Equal("POST_SUSPENDED", entity["reason"])
	s.Nil(entity["lockCount"], "nothing is counted under the hold, so no lock count is carried across")
	s.Equal("2026-09-07T08:00:00Z", s.accessStateDoc("b4")["lastLoginAt"])
}

// Nothing is counted while an entity-wide lock is live.
func (s *AccessStateStoreTestSuite) TestNoIncrementUnderALiveEntityHold() {
	s.seed("b5", `{"accessState":{"lock":{"entity":`+
		`{"unlockAt":"9999-12-31T23:59:59Z","reason":"POST_SUSPENDED"}}}}`)
	now := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)

	_, recorded, err := s.runtime.IncrementAccessFailure(
		s.ctx, "b5", testScope, now, now.Add(-15*time.Minute))
	s.Require().NoError(err)
	s.False(recorded, "the statement must match no row while the hold is live")
	s.Nil(s.scopeEntry("b5", testScope), "and it must write nothing")
}

func (s *AccessStateStoreTestSuite) TestALapsedEntityEntryDoesNotSuppressCounting() {
	s.seed("b6", `{"accessState":{"lock":{"entity":{"unlockAt":"2026-09-08T09:00:00Z"}}}}`)
	now := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)

	snapshot, recorded, err := s.runtime.IncrementAccessFailure(
		s.ctx, "b6", testScope, now, now.Add(-15*time.Minute))
	s.Require().NoError(err)
	s.True(recorded)
	s.Equal(1, snapshot.FailureCount)
}

func (s *AccessStateStoreTestSuite) TestClearAccessStateCannotReachSuspendOrLastLogin() {
	s.seed("b7", `{"accessState":{"lock":{"authenticationMethods":{"credential":{"lockCount":1}}},`+
		`"suspend":{"suspendedAt":"2026-09-08T11:00:00Z"},"lastLoginAt":"2026-09-07T08:00:00Z"}}`)

	s.Require().NoError(s.runtime.ClearAccessState(s.ctx, "b7",
		[]governancemodel.AccessScope{testScope, governancemodel.AccessScopeEntity}))

	s.Nil(s.scopeEntry("b7", testScope))
	s.Equal("2026-09-08T11:00:00Z", s.suspendEntry("b7")["suspendedAt"],
		"a suspension is a different dimension")
	s.Equal("2026-09-07T08:00:00Z", s.accessStateDoc("b7")["lastLoginAt"])
}

func (s *AccessStateStoreTestSuite) TestSuspensionOnAMissingRowIsRefused() {
	s.ErrorIs(s.runtime.SetAccessSuspension(s.ctx, "no-such-entity", time.Now().UTC(), ""),
		ErrEntityNotFound)
	s.ErrorIs(s.runtime.ClearAccessSuspension(s.ctx, "no-such-entity", nil, ""), ErrEntityNotFound)
}

func (s *AccessStateStoreTestSuite) TestStaleReleaseCannotClearNewSuspension() {
	s.seed("release-race", nil)
	old := time.Now().UTC()
	current := old.Add(time.Nanosecond)
	s.Require().NoError(s.runtime.SetAccessSuspension(s.ctx, "release-race", old, "first"))
	s.Require().NoError(s.runtime.SetAccessSuspension(s.ctx, "release-race", current, "second"))
	s.Error(s.runtime.ClearAccessSuspension(s.ctx, "release-race", nil, old.Format(time.RFC3339Nano)))
	s.Equal("second", s.suspendEntry("release-race")["operatorNote"])
	s.Require().NoError(s.runtime.ClearAccessSuspension(s.ctx, "release-race", nil, current.Format(time.RFC3339Nano)))
	s.Nil(s.suspendEntry("release-race"))
}

// Recording a login writes only lastLoginAt and keeps the lock and suspension members.
func (s *AccessStateStoreTestSuite) TestRecordAccessLoginLeavesTheRestAlone() {
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

func (s *AccessStateStoreTestSuite) TestRecordAccessLoginCreatesTheDocument() {
	s.seed("l2", nil)
	at := time.Date(2026, 9, 9, 12, 30, 0, 0, time.UTC)

	s.Require().NoError(s.runtime.RecordAccessLogin(s.ctx, "l2", at, at.Add(-time.Hour)))

	s.Equal("2026-09-09T12:30:00Z", s.accessStateDoc("l2")["lastLoginAt"])
}

// Only a stored value inside the window skips the write. An older value, a future value and a
// non-timestamp value are overwritten.
func (s *AccessStateStoreTestSuite) TestRecordAccessLoginAppliesTheWindow() {
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
func (s *AccessStateStoreTestSuite) TestRecordAccessLoginWithNoResolutionWritesEveryTime() {
	at := time.Date(2026, 9, 9, 12, 30, 0, 0, time.UTC)
	s.seed("l3", `{"accessState":{"lastLoginAt":"2026-09-09T12:29:59Z"}}`)

	s.Require().NoError(s.runtime.RecordAccessLogin(s.ctx, "l3", at, at))

	s.Equal("2026-09-09T12:30:00Z", s.accessStateDoc("l3")["lastLoginAt"])
}

func (s *AccessStateStoreTestSuite) TestRecordAccessLoginOnAMissingRowIsNotAnError() {
	now := time.Now().UTC()
	s.Require().NoError(s.runtime.RecordAccessLogin(s.ctx, "no-such-entity", now, now.Add(-time.Hour)))
}

func (s *AccessStateStoreTestSuite) TestConditionalClearPreservesNewerFailures() {
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

func (s *AccessStateStoreTestSuite) TestConditionalClearPreservesNewerPostSuspensionHold() {
	s.seed("e1", `{"accessState":{"lock":{"entity":{"failureCount":1}}}}`)
	now := time.Now().UTC()
	s.Require().NoError(s.runtime.SetAccessSuspension(s.ctx, "e1", now, "operator"))
	s.Require().NoError(s.runtime.ClearAccessSuspension(s.ctx, "e1", &governancemodel.EntityHold{
		UnlockAt: governancemodel.PermanentUnlockAt, Reason: governancemodel.ReasonPostSuspension,
	}, now.Format(time.RFC3339Nano)))

	_, err := s.runtime.ClearAccessStateIfUnchanged(s.ctx, "e1", governancemodel.AccessScopeEntity, 0)
	s.Require().NoError(err)
	s.Equal(governancemodel.PermanentUnlockAt, s.scopeEntry("e1", governancemodel.AccessScopeEntity)["unlockAt"])
	// Explicit recovery/operator release remains unconditional.
	s.Require().NoError(s.runtime.ClearAccessState(s.ctx, "e1", governancemodel.AllAccessScopes))
	s.Nil(s.scopeEntry("e1", governancemodel.AccessScopeEntity))
}

func (s *AccessStateStoreTestSuite) TestConditionalClearIsDeploymentScoped() {
	s.seed("e1", `{"accessState":{"lock":{"entity":{"failureCount":1}}}}`)
	_, err := s.runtimeDB.Exec(`INSERT INTO "ENTITY_RUNTIME_DATA"
		(DEPLOYMENT_ID, ENTITY_ID, STATE, RUNTIME_ATTRIBUTES, CREATED_AT, UPDATED_AT)
		SELECT 'other-deployment', ENTITY_ID, STATE, RUNTIME_ATTRIBUTES, CREATED_AT, UPDATED_AT
		FROM "ENTITY_RUNTIME_DATA" WHERE ENTITY_ID = 'e1'`)
	s.Require().NoError(err)
	_, err = s.runtime.ClearAccessStateIfUnchanged(s.ctx, "e1", governancemodel.AccessScopeEntity, 0)
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
