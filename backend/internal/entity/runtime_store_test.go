// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	_ "modernc.org/sqlite"

	"github.com/thunder-id/thunderid/internal/identitygovernance"
	governancemodel "github.com/thunder-id/thunderid/internal/identitygovernance/model"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	sysconfig "github.com/thunder-id/thunderid/internal/system/config"
	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
	"github.com/thunder-id/thunderid/internal/system/deployment"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

const harnessDeploymentID = "dep1"

// RuntimeStoreTestSuite runs the entity service and the runtime store against real SQLite databases.
type RuntimeStoreTestSuite struct {
	suite.Suite
	*sqliteHarness
	ctx context.Context
	svc *entityService
}

func TestRuntimeStoreTestSuite(t *testing.T) {
	suite.Run(t, new(RuntimeStoreTestSuite))
}

func (s *RuntimeStoreTestSuite) SetupTest() {
	s.sqliteHarness = newSQLiteHarness(s.T(), strings.ReplaceAll(s.T().Name(), "/", "-"))
	s.ctx = deployment.WithID(context.Background(), harnessDeploymentID)
	s.svc = s.newService(s.store)
}

// newService returns an entity service over the given profile store and the harness runtime store.
func (s *RuntimeStoreTestSuite) newService(store entityStoreInterface) *entityService {
	return newEntityService(store, s.runtime, nil, nil, nil,
		transaction.NewTransactioner(s.db, "entity")).(*entityService)
}

func (s *RuntimeStoreTestSuite) TestDatabaseSeedDefaultsActiveWithoutProfileWrites() {
	profile := `{"verifiedAttributes":{"email":{"verified":true}},"name":"kept",` +
		`"accessState":{"suspend":{"suspendedAt":"2026-09-16T00:00:00Z"}},` +
		`"credentialUpdatedAt":"2026-09-16T00:00:00Z"}`
	s.seedEntity(s.T(), "seeded", profile)
	_, err := s.db.Exec(`
 CREATE TRIGGER readonly_profile BEFORE UPDATE ON "ENTITY" BEGIN SELECT RAISE(ABORT,'profile is read only'); END;
 CREATE TRIGGER readonly_profile_delete BEFORE DELETE ON "ENTITY" BEGIN SELECT RAISE(ABORT,'profile is read only'); END;
 CREATE TRIGGER readonly_profile_insert BEFORE INSERT ON "ENTITY"
 BEGIN SELECT RAISE(ABORT,'profile is read only'); END;`)
	s.Require().NoError(err)

	e, err := s.svc.GetEntity(s.ctx, "seeded")
	s.Require().NoError(err)
	s.Equal(providers.EntityStateActive, e.State, "a database profile has no state to seed")
	s.JSONEq(`{}`, string(e.RuntimeAttributes), "no runtime value is read from system attributes")
	s.JSONEq(profile, string(e.SystemAttributes))

	_, recorded, err := s.runtime.IncrementAccessFailure(s.ctx, e.ID, testScope, time.Now(), time.Time{})
	s.Require().NoError(err)
	s.True(recorded)
	s.Require().NoError(s.runtime.SetAccessSuspension(s.ctx, e.ID,
		time.Date(2026, 9, 25, 0, 0, 0, 123, time.UTC), "operator"))

	e, err = s.svc.GetEntity(s.ctx, e.ID)
	s.Require().NoError(err)
	s.Contains(string(e.RuntimeAttributes), "operator")

	s.Require().NoError(s.runtime.ClearAccessSuspension(s.ctx, e.ID, nil, "2026-09-25T00:00:00.000000123Z"))
	e, err = s.svc.GetEntity(s.ctx, e.ID)
	s.Require().NoError(err)
	s.Equal(providers.EntityStateActive, e.State)
	s.JSONEq(profile, s.storedSystemAttributes(s.T(), e.ID))

	var stamp string
	s.Require().NoError(s.db.QueryRow(`SELECT UPDATED_AT FROM "ENTITY" WHERE ID=$1`, e.ID).Scan(&stamp))
	s.Equal("t", stamp)

	_, err = s.svc.GetEntity(s.ctx, "absent")
	s.ErrorIs(err, ErrEntityNotFound)
	rows, err := s.runtime.GetRuntimeData(s.ctx, []string{"absent"})
	s.Require().NoError(err)
	s.Empty(rows)
}

func (s *RuntimeStoreTestSuite) TestDeclarativeSourcePersistsAcrossReload() {
	file := newEntityFileBasedStore()
	e := *testEntity("declarative-runtime")
	e.SystemAttributes = json.RawMessage(`{"verifiedAttributes":{"email":true}}`)
	s.Require().NoError(file.CreateEntity(s.ctx, e, nil, nil))
	svc := s.newService(newEntityCompositeStore(file, s.store))

	got, err := svc.GetEntity(s.ctx, e.ID)
	s.Require().NoError(err)
	s.True(got.IsReadOnly)
	s.Require().NoError(svc.SetSuspension(s.ctx, e.ID, time.Now(), "review"))

	// A new file store stands for a restart with the same declarative source.
	reloaded := newEntityFileBasedStore()
	s.Require().NoError(reloaded.CreateEntity(s.ctx, e, nil, nil))
	other := s.newService(newEntityCompositeStore(reloaded, s.store))

	got, err = other.GetEntity(s.ctx, e.ID)
	s.Require().NoError(err)
	s.Equal(providers.EntityStateSuspended, got.State)
	s.Contains(string(got.RuntimeAttributes), "review")

	var count int
	s.Require().NoError(s.db.QueryRow(`SELECT count(*) FROM "ENTITY"`).Scan(&count))
	s.Zero(count)
}

// Creating a database entity with a declarative entity's id is refused.
func (s *RuntimeStoreTestSuite) TestCreateRefusesADeclarativeID() {
	file := newEntityFileBasedStore()
	e := *testEntity("declarative-held")
	s.Require().NoError(file.CreateEntity(s.ctx, e, nil, nil))
	svc := s.newService(newEntityCompositeStore(file, s.store))
	_, err := svc.GetEntity(s.ctx, e.ID)
	s.Require().NoError(err)
	s.Require().NoError(svc.SetSuspension(s.ctx, e.ID, time.Now(), "held"))

	_, err = svc.CreateEntity(s.ctx, testEntity(e.ID), nil)

	s.ErrorIs(err, ErrAttributeConflict)
	got, err := svc.GetEntity(s.ctx, e.ID)
	s.Require().NoError(err)
	s.Equal(providers.EntityStateSuspended, got.State, "the suspension is kept")
	var count int
	s.Require().NoError(s.db.QueryRow(`SELECT count(*) FROM "ENTITY"`).Scan(&count))
	s.Zero(count)
}

func (s *RuntimeStoreTestSuite) TestListHydrationIsBatched() {
	entities := make([]providers.Entity, 2*runtimeBatchSize+1)
	for i := range entities {
		entities[i] = *testEntity(fmt.Sprintf("batch-%d", i))
	}

	s.Require().NoError(s.svc.populateRuntime(s.ctx, entities))
	s.Equal(3, s.queries["ERD-ENTITY_MGT-02"])
	s.Equal(6, s.queries["ERD-ENTITY_MGT-01"])

	s.Require().NoError(s.svc.populateRuntime(s.ctx, entities))
	s.Equal(3, s.queries["ERD-ENTITY_MGT-02"])
	s.Equal(9, s.queries["ERD-ENTITY_MGT-01"])
}

func (s *RuntimeStoreTestSuite) TestHydrationRereadsOnlyTheMissingRows() {
	entities := make([]providers.Entity, runtimeBatchSize+runtimeBatchSize/2)
	for i := range entities {
		entities[i] = *testEntity(fmt.Sprintf("present-%d", i))
	}
	s.Require().NoError(s.svc.populateRuntime(s.ctx, entities))
	s.queries = map[string]int{}

	entities = append(entities, *testEntity("missing"))
	s.Require().NoError(s.svc.populateRuntime(s.ctx, entities))

	s.Equal(1, s.queries["ERD-ENTITY_MGT-02"])
	s.Equal(3, s.queries["ERD-ENTITY_MGT-01"], "two batches, then one read of the seeded row")
	s.Equal(providers.EntityStateActive, entities[len(entities)-1].State)
}

// The governed read seeds a missing runtime row, so a later increment has a row to update.
func (s *RuntimeStoreTestSuite) TestGovernedReadSeedsTheRowAFailureCountsAgainst() {
	s.seedEntity(s.T(), "unseeded", `{}`)
	now := time.Now().UTC()

	_, recorded, err := s.svc.IncrementFailure(s.ctx, "unseeded", testScope, now, now.Add(-time.Minute))
	s.Require().NoError(err)
	s.False(recorded, "with no row the increment has nothing to write")

	_, err = s.svc.GetGovernedEntity(s.ctx, "unseeded")
	s.Require().NoError(err)
	observed, recorded, err := s.svc.IncrementFailure(s.ctx, "unseeded", testScope, now, now.Add(-time.Minute))
	s.Require().NoError(err)
	s.True(recorded, "the governed read seeded the row")
	s.Equal(1, observed.FailureCount)
}

// Declarative sources retain states that this build does not define.
func (s *RuntimeStoreTestSuite) TestSeedKeepsAnUnknownDeclarativeState() {
	file := newEntityFileBasedStore()
	e := *testEntity("future-state")
	e.State = providers.EntityState("SOME_FUTURE_STATE")
	s.Require().NoError(file.CreateEntity(s.ctx, e, nil, nil))
	svc := s.newService(newEntityCompositeStore(file, s.store))
	got, err := svc.GetEntity(s.ctx, e.ID)
	s.Require().NoError(err)
	s.Equal(e.State, got.State)
	var stored string
	s.Require().NoError(s.runtimeDB.QueryRow(
		`SELECT STATE FROM "ENTITY_RUNTIME_DATA" WHERE ENTITY_ID = 'future-state'`).Scan(&stored))
	s.Equal(string(e.State), stored)
}

func (s *RuntimeStoreTestSuite) TestDatabaseCreateStartsActive() {
	e := testEntity("database-active")
	e.State = providers.EntityStateSuspended
	got, err := s.svc.CreateEntity(s.ctx, e, nil)
	s.Require().NoError(err)
	s.Equal(providers.EntityStateActive, got.State)
	profile, err := s.store.GetEntity(s.ctx, e.ID)
	s.Require().NoError(err)
	s.Empty(profile.State)
}

func (s *RuntimeStoreTestSuite) TestRevisionRejectsResetABA() {
	s.Require().NoError(s.runtime.InitializeRuntimeData(s.ctx, []entityRuntimeData{{ID: "aba",
		State: providers.EntityStateActive, Attributes: json.RawMessage(`{}`)}}))
	now := time.Now().UTC()

	before, _, err := s.runtime.IncrementAccessFailure(s.ctx, "aba", testScope, now, now.Add(-time.Minute))
	s.Require().NoError(err)
	s.Require().NoError(s.runtime.ClearAccessState(s.ctx, "aba", []governancemodel.AccessScope{testScope}))
	after, _, err := s.runtime.IncrementAccessFailure(s.ctx, "aba", testScope, now, now.Add(-time.Minute))
	s.Require().NoError(err)
	s.Equal(before.FailureCount, after.FailureCount)
	s.Greater(after.Revision, before.Revision)

	episode := governancemodel.LockEpisode{LockCount: 1, UnlockAt: now.Add(time.Minute).Format(time.RFC3339Nano)}
	formed, err := s.runtime.FormAccessLock(s.ctx, "aba", testScope, before, episode, now)
	s.Require().NoError(err)
	s.False(formed)
	formed, err = s.runtime.FormAccessLock(s.ctx, "aba", testScope, after, episode, now)
	s.Require().NoError(err)
	s.True(formed)
}

func (s *RuntimeStoreTestSuite) TestDeletionAndReuse() {
	e := testEntity("reused")
	_, err := s.svc.CreateEntity(s.ctx, e, nil)
	s.Require().NoError(err)
	s.Require().NoError(s.runtime.SetAccessSuspension(s.ctx, e.ID, time.Now(), "old"))
	s.Require().NoError(s.svc.DeleteEntity(s.ctx, e.ID))

	rows, err := s.runtime.GetRuntimeData(s.ctx, []string{e.ID})
	s.Require().NoError(err)
	s.Empty(rows)

	// An orphan row at the same id is reset when a new database entity is created there.
	s.Require().NoError(s.runtime.InitializeRuntimeData(s.ctx, []entityRuntimeData{{ID: e.ID,
		State:      providers.EntityStateSuspended,
		Attributes: json.RawMessage(`{"accessState":{"suspend":{}}}`)}}))
	got, err := s.svc.CreateEntity(s.ctx, testEntity(e.ID), nil)
	s.Require().NoError(err)
	s.Equal(providers.EntityStateActive, got.State)
	s.JSONEq(`{}`, string(got.RuntimeAttributes))
}

func (s *RuntimeStoreTestSuite) TestCreateRollsBackWhenTheRuntimeWriteFails() {
	_, err := s.runtimeDB.Exec(`DROP TABLE "ENTITY_RUNTIME_DATA"`)
	s.Require().NoError(err)

	_, err = s.svc.CreateEntity(s.ctx, testEntity("rolled-back"), nil)

	s.Error(err)
	var count int
	s.Require().NoError(s.db.QueryRow(`SELECT count(*) FROM "ENTITY"`).Scan(&count))
	s.Zero(count)
}

// The runtime write is not part of the entity transaction, so its row stays when the entity
// transaction fails. A later create at the same id resets it.
func (s *RuntimeStoreTestSuite) TestRuntimeRowOutlivesAFailedEntityTransaction() {
	svc := s.newService(failingReadStore{s.store})

	_, err := svc.CreateEntity(s.ctx, testEntity("orphaned"), nil)

	s.Error(err)
	var count int
	s.Require().NoError(s.db.QueryRow(`SELECT count(*) FROM "ENTITY"`).Scan(&count))
	s.Zero(count, "the entity insert is rolled back")
	rows, err := s.runtime.GetRuntimeData(s.ctx, []string{"orphaned"})
	s.Require().NoError(err)
	s.Require().Contains(rows, "orphaned")
	s.Equal(providers.EntityStateActive, rows["orphaned"].State)
	s.JSONEq(`{}`, string(rows["orphaned"].Attributes))
}

func (s *RuntimeStoreTestSuite) TestDeleteSucceedsWhenTheRuntimeDeleteFails() {
	e := testEntity("delete-orphan")
	_, err := s.svc.CreateEntity(s.ctx, e, nil)
	s.Require().NoError(err)
	_, err = s.runtimeDB.Exec(`DROP TABLE "ENTITY_RUNTIME_DATA"`)
	s.Require().NoError(err)

	s.NoError(s.svc.DeleteEntity(s.ctx, e.ID))

	var count int
	s.Require().NoError(s.db.QueryRow(`SELECT count(*) FROM "ENTITY"`).Scan(&count))
	s.Zero(count)
}

func (s *RuntimeStoreTestSuite) TestUnavailableRuntimeStoreFailsTheRead() {
	s.seedEntity(s.T(), "known", nil)
	_, err := s.runtimeDB.Exec(`DROP TABLE "ENTITY_RUNTIME_DATA"`)
	s.Require().NoError(err)

	e, err := s.svc.GetEntity(s.ctx, "known")

	s.Error(err)
	s.Nil(e)
}

func (s *RuntimeStoreTestSuite) TestGovernanceSnapshotKeepsRevisionWithoutExtraRuntimeRead() {
	s.seedEntity(s.T(), "snapshot", `{}`)
	_, err := s.svc.GetEntity(s.ctx, "snapshot")
	s.Require().NoError(err)
	now := time.Now().UTC()
	_, recorded, err := s.svc.IncrementFailure(s.ctx, "snapshot", testScope, now, now.Add(-time.Minute))
	s.Require().NoError(err)
	s.Require().True(recorded)

	reads := s.queries["ERD-ENTITY_MGT-01"]
	before, err := s.svc.GetGovernedEntity(s.ctx, "snapshot")
	s.Require().NoError(err)
	s.Equal(reads+1, s.queries["ERD-ENTITY_MGT-01"])
	oldLock := before.AccessState.Lock.AuthenticationMethods[testScope]
	oldLock.Revision = before.Revision
	s.Require().Positive(oldLock.Revision)

	s.Require().NoError(s.svc.ClearAccessState(s.ctx, "snapshot", []governancemodel.AccessScope{testScope}))
	_, recorded, err = s.svc.IncrementFailure(s.ctx, "snapshot", testScope, now, now.Add(-time.Minute))
	s.Require().NoError(err)
	s.Require().True(recorded)
	after, err := s.svc.GetGovernedEntity(s.ctx, "snapshot")
	s.Require().NoError(err)
	newLock := after.AccessState.Lock.AuthenticationMethods[testScope]
	newLock.Revision = after.Revision
	s.Equal(oldLock.FailureCount, newLock.FailureCount)
	s.Greater(newLock.Revision, oldLock.Revision)

	episode := governancemodel.LockEpisode{LockCount: 1, UnlockAt: now.Add(time.Minute).Format(time.RFC3339Nano)}
	formed, err := s.svc.FormLock(s.ctx, "snapshot", testScope, oldLock, episode, now)
	s.Require().NoError(err)
	s.False(formed)
	formed, err = s.svc.FormLock(s.ctx, "snapshot", testScope, newLock, episode, now)
	s.Require().NoError(err)
	s.True(formed)
}

func (s *RuntimeStoreTestSuite) TestAuthenticationStateQueryCost() {
	s.T().Cleanup(config.ResetServerRuntime)
	s.Require().NoError(config.InitializeServerRuntime("test", &config.Config{}))
	for _, mode := range []string{"warm", "cold", "disabled"} {
		s.Run(mode, func() {
			h := newSQLiteHarness(s.T(), strings.ReplaceAll(s.T().Name(), "/", "-"))
			h.seedEntity(s.T(), "subject", `{}`)
			s.Require().NoError(h.runtime.InitializeRuntimeData(s.ctx, []entityRuntimeData{{
				ID: "subject", State: providers.EntityStateActive, Attributes: json.RawMessage(`{}`),
			}}))
			cm := cache.Initialize(engineconfig.CacheConfig{
				Disabled: mode == "disabled", Size: 100, TTL: 3600,
			}, harnessDeploymentID)
			s.T().Cleanup(cm.Close)
			store := newCacheBackedEntityStore(h.store,
				cache.GetCache[*providers.Entity](cm, "EntityByIDCache"),
				cache.GetCache[*entityWithCredentials](cm, "EntityWithCredentialsByIDCache"),
				cache.GetCache[*string](cm, "EntityIDByIdentifierCache"))
			svc := newEntityService(store, h.runtime, nil, nil, nil, nil)
			if mode == "warm" {
				_, err := store.GetEntity(s.ctx, "subject")
				s.Require().NoError(err)
			}
			governance, err := identitygovernance.Initialize(svc, sysconfig.AccountAccessConfig{}, nil, nil, nil)
			s.Require().NoError(err)
			clear(h.queries)

			_, err = svc.GetEntityProfile(s.ctx, "subject")
			s.Require().NoError(err)
			decision, err := governance.AdmitAuthenticationStep(s.ctx, "subject", testScope)
			s.Require().NoError(err)
			s.True(decision.Admitted)
			decision, err = governance.AdmitSignIn(s.ctx, "subject")
			s.Require().NoError(err)
			s.True(decision.Admitted)

			s.T().Logf("cache=%s profile ASQ-ENTITY_MGT-05=%d runtime ERD-ENTITY_MGT-01=%d clear ERD-07/08=%d",
				mode, h.queries["ASQ-ENTITY_MGT-05"], h.queries["ERD-ENTITY_MGT-01"],
				h.queries["ERD-ENTITY_MGT-07"]+h.queries["ERD-ENTITY_MGT-08"])
			s.Equal(map[string]int{"warm": 0, "cold": 1, "disabled": 3}[mode], h.queries["ASQ-ENTITY_MGT-05"])
			s.Equal(2, h.queries["ERD-ENTITY_MGT-01"])
			s.Zero(h.queries["ERD-ENTITY_MGT-07"] + h.queries["ERD-ENTITY_MGT-08"])
		})
	}
}

func (s *RuntimeStoreTestSuite) TestEntityProfileDoesNotReadOrInitializeRuntimeState() {
	s.seedEntity(s.T(), "subject",
		`{"accessState":{"suspend":{"operatorNote":"legacy"}},"credentialUpdatedAt":"stamp"}`)

	profile, err := s.svc.GetEntityProfile(s.ctx, "subject")
	s.Require().NoError(err)
	s.Empty(profile.State)
	s.Empty(profile.RuntimeAttributes)
	s.Zero(s.queries["ERD-ENTITY_MGT-01"])
	s.Zero(s.queries["ERD-ENTITY_MGT-02"])

	// Governance reads the runtime row only; a profile key of the same name holds nothing.
	governed, err := s.svc.GetGovernedEntity(s.ctx, "subject")
	s.Require().NoError(err)
	s.Nil(governed.AccessState.Suspend)
}

// failingReadStore fails the read-back that ends the create transaction.
type failingReadStore struct{ *entityDBStore }

func (failingReadStore) GetEntity(context.Context, string) (providers.Entity, error) {
	return providers.Entity{}, errors.New("read failed")
}

// sqliteHarness runs the real entity store and runtime store against two in-memory SQLite
// databases, one per logical database, each behind its own client as in production.
type sqliteHarness struct {
	mu        sync.Mutex
	queries   map[string]int
	db        *sql.DB
	runtimeDB *sql.DB
	store     *entityDBStore
	runtime   *entityRuntimeDBStore
}

// newSQLiteHarness creates the two databases for name and returns stores wired to them. Use a name
// unique to the test: shared-cache databases with the same name are the same database.
func newSQLiteHarness(t *testing.T, name string) *sqliteHarness {
	t.Helper()

	h := &sqliteHarness{
		db:        openSQLite(t, "file:"+name+"-entity?mode=memory&cache=shared"),
		runtimeDB: openSQLite(t, "file:"+name+"-runtime?mode=memory&cache=shared"),
		queries:   map[string]int{},
	}
	h.createTables(t)

	provider := providermock.NewDBProviderInterfaceMock(t)
	entityClient := providermock.NewDBClientInterfaceMock(t)
	runtimeClient := providermock.NewDBClientInterfaceMock(t)
	provider.On("GetEntityDBClient").Return(entityClient, nil).Maybe()
	provider.On("GetRuntimePersistentDBClient").Return(runtimeClient, nil).Maybe()
	provider.On("GetRuntimePersistentDBTransactioner").
		Return(transaction.NewTransactioner(h.runtimeDB, "runtime_persistent"), nil).Maybe()
	h.wireClient(entityClient, h.db, "entity")
	h.wireClient(runtimeClient, h.runtimeDB, "runtime_persistent")

	h.store = &entityDBStore{
		indexedAttributes: map[string]bool{},
		dbProvider:        provider,
		logger:            log.GetLogger(),
	}
	h.runtime = &entityRuntimeDBStore{dbProvider: provider}
	return h
}

// openSQLite opens one shared-cache database with a single connection, so overlapping writes wait
// instead of failing on a table lock.
func openSQLite(t *testing.T, dsn string) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	return db
}

// createTables mirrors backend/dbscripts/entitydb/sqlite.sql and
// backend/dbscripts/runtime_persistent/sqlite.sql. Tables are dropped first, since a shared-cache
// database outlives the connection that made it.
func (h *sqliteHarness) createTables(t *testing.T) {
	t.Helper()

	_, err := h.runtimeDB.Exec(`DROP TABLE IF EXISTS "ENTITY_RUNTIME_DATA"; CREATE TABLE "ENTITY_RUNTIME_DATA" (
    DEPLOYMENT_ID       VARCHAR(255) NOT NULL,
    ENTITY_ID           VARCHAR(36) NOT NULL,
    STATE               VARCHAR(50) NOT NULL,
    RUNTIME_ATTRIBUTES  TEXT NOT NULL DEFAULT '{}',
    REVISION            BIGINT NOT NULL DEFAULT 0,
    CREATED_AT          TEXT NOT NULL,
    UPDATED_AT          TEXT NOT NULL,
    PRIMARY KEY (DEPLOYMENT_ID, ENTITY_ID),
    CHECK (json_valid(RUNTIME_ATTRIBUTES) AND json_type(RUNTIME_ATTRIBUTES) = 'object')
);`)
	require.NoError(t, err)

	_, err = h.db.Exec(`DROP TABLE IF EXISTS "ENTITY"`)
	require.NoError(t, err)
	_, err = h.db.Exec(`CREATE TABLE "ENTITY" (
		DEPLOYMENT_ID       VARCHAR(255) NOT NULL,
		ID                  VARCHAR(36)  PRIMARY KEY,
		CATEGORY            VARCHAR(50)  NOT NULL,
		TYPE                VARCHAR(50)  NOT NULL,
		OU_ID               VARCHAR(36)  NOT NULL,
		ATTRIBUTES          TEXT,
		SYSTEM_ATTRIBUTES   TEXT,
		CREDENTIALS         TEXT,
		SYSTEM_CREDENTIALS  TEXT,
		CREATED_AT          TEXT NOT NULL,
		UPDATED_AT          TEXT NOT NULL)`)
	require.NoError(t, err)

	_, err = h.db.Exec(`DROP TABLE IF EXISTS "ENTITY_IDENTIFIER"`)
	require.NoError(t, err)
	_, err = h.db.Exec(`CREATE TABLE "ENTITY_IDENTIFIER" (
		DEPLOYMENT_ID VARCHAR(255) NOT NULL, ENTITY_ID VARCHAR(36) NOT NULL,
		NAME VARCHAR(255) NOT NULL, VALUE TEXT NOT NULL, SOURCE VARCHAR(50) NOT NULL,
		CREATED_AT TEXT NOT NULL, PRIMARY KEY (ENTITY_ID, DEPLOYMENT_ID, NAME))`)
	require.NoError(t, err)
}

// wireClient forwards a mocked client to db, using the transaction keyed by dbName when the context
// carries one. Each stub returns both results from one function, so each statement runs once.
func (h *sqliteHarness) wireClient(client *providermock.DBClientInterfaceMock, db *sql.DB, dbName string) {
	anys := make([]interface{}, 805)
	for i := range anys {
		anys[i] = mock.Anything
	}

	client.On("ExecuteContext", anys...).Maybe().Return(
		func(ctx context.Context, query dbmodel.DBQuery, args ...interface{}) (int64, error) {
			h.count(query.ID)

			var res sql.Result
			var err error
			if tx := transaction.KeyedTxFromContext(ctx, dbName); tx != nil {
				res, err = tx.ExecContext(ctx, query.GetQuery("sqlite"), args...)
			} else {
				res, err = db.ExecContext(ctx, query.GetQuery("sqlite"), args...)
			}
			if err != nil {
				return 0, err
			}
			return res.RowsAffected()
		},
	)

	client.On("QueryContext", anys...).Maybe().Return(
		func(ctx context.Context, query dbmodel.DBQuery,
			args ...interface{}) ([]map[string]interface{}, error) {
			h.count(query.ID)
			return queryRows(ctx, db, dbName, query, args...)
		},
	)
}

// queryRows shapes rows the way the real client does, including the lower-cased column names.
func queryRows(ctx context.Context, db *sql.DB, dbName string, query dbmodel.DBQuery,
	args ...interface{}) ([]map[string]interface{}, error) {
	var rows *sql.Rows
	var err error
	if tx := transaction.KeyedTxFromContext(ctx, dbName); tx != nil {
		rows, err = tx.QueryContext(ctx, query.GetQuery("sqlite"), args...)
	} else {
		rows, err = db.QueryContext(ctx, query.GetQuery("sqlite"), args...)
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	for rows.Next() {
		row := make([]interface{}, len(columns))
		pointers := make([]interface{}, len(columns))
		for i := range row {
			pointers[i] = &row[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, err
		}

		result := map[string]interface{}{}
		for i, col := range columns {
			result[strings.ToLower(col)] = row[i]
		}
		results = append(results, result)
	}
	return results, rows.Err()
}

// seedEntity inserts a user row with the given system-attributes value. Nil stores NULL.
func (h *sqliteHarness) seedEntity(t *testing.T, id string, systemAttrs interface{}) {
	t.Helper()

	_, err := h.db.Exec(`INSERT INTO "ENTITY" VALUES `+
		`($1,$2,'user','person','ou1','{}',$3,NULL,NULL,'t','t')`,
		harnessDeploymentID, id, systemAttrs)
	require.NoError(t, err)
}

// storedSystemAttributes returns the raw SYSTEM_ATTRIBUTES column value.
func (h *sqliteHarness) storedSystemAttributes(t *testing.T, id string) string {
	t.Helper()

	var raw sql.NullString
	require.NoError(t, h.db.QueryRow(
		`SELECT SYSTEM_ATTRIBUTES FROM "ENTITY" WHERE ID = $1`, id).Scan(&raw))
	return raw.String
}

// count records that a statement ran.
func (h *sqliteHarness) count(id string) { h.mu.Lock(); defer h.mu.Unlock(); h.queries[id]++ }

// The file state seeds once; subsequent file loads cannot overwrite runtime decisions.
func (s *RuntimeStoreTestSuite) TestDeclarativeYAMLSeedsInitialStateOnce() {
	cases := []struct {
		yamlState string
		want      providers.EntityState
	}{
		{"", providers.EntityStateActive},
		{"state: ACTIVE\n", providers.EntityStateActive},
		{"state: SUSPENDED\n", providers.EntityStateSuspended},
		{"state: SOME_FUTURE_STATE\n", providers.EntityState("SOME_FUTURE_STATE")},
		{"state: \"\"\n", providers.EntityStateActive},
	}
	s.T().Cleanup(config.ResetServerRuntime)
	for _, category := range []providers.EntityCategory{
		providers.EntityCategoryUser, providers.EntityCategoryAgent, providers.EntityCategoryApp,
	} {
		for i, tc := range cases {
			name := fmt.Sprintf("%s-%d", category, i)
			s.Run(name, func() {
				config.ResetServerRuntime()
				directory := s.T().TempDir()
				resourceDir := filepath.Join(directory, "config", "resources", "entities")
				s.Require().NoError(os.MkdirAll(resourceDir, 0750))
				s.Require().NoError(os.WriteFile(filepath.Join(resourceDir, "identity.yaml"),
					[]byte("id: "+name+"\n"+tc.yamlState), 0600))
				s.Require().NoError(config.InitializeServerRuntime(directory, &config.Config{}))
				file := newEntityFileBasedStore()
				svc := s.newService(newEntityCompositeStore(file, s.store))
				cfg := DeclarativeLoaderConfig{Directory: "entities", Category: category,
					Parser: func([]byte) (*providers.Entity, json.RawMessage, json.RawMessage, error) {
						e := testEntity(name)
						e.Category = category
						// Consumer parsers normally supply ACTIVE; the shared loader reads the file.
						return e, nil, nil, nil
					},
				}
				s.Require().NoError(loadDeclarativeResources(file, svc, cfg))
				got, err := svc.GetEntity(s.ctx, name)
				s.Require().NoError(err)
				s.Equal(tc.want, got.State)
				s.JSONEq(`{}`, string(got.RuntimeAttributes))
				// Runtime authority must survive a reload of the same initial declaration.
				s.Require().NoError(s.runtime.SetAccessSuspension(s.ctx, name, time.Now(), "runtime hold"))
				reloaded := newEntityFileBasedStore()
				svc = s.newService(newEntityCompositeStore(reloaded, s.store))
				s.Require().NoError(loadDeclarativeResources(reloaded, svc, cfg))
				got, err = svc.GetEntity(s.ctx, name)
				s.Require().NoError(err)
				s.Equal(providers.EntityStateSuspended, got.State)
				s.Contains(string(got.RuntimeAttributes), "runtime hold")
				if tc.want == providers.EntityStateSuspended {
					governed, err := svc.GetGovernedEntity(s.ctx, name)
					s.Require().NoError(err)
					s.Require().NoError(svc.ClearSuspension(s.ctx, name, nil, governed.AccessState.Suspend.SuspendedAt))
					got, err = svc.GetEntity(s.ctx, name)
					s.Require().NoError(err)
					s.Equal(providers.EntityStateActive, got.State, "file SUSPENDED is only an initial seed")
				}
			})
		}
	}
}
