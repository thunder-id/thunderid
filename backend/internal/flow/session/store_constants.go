// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"fmt"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/database/model"
)

var (
	// queryCreateSession inserts a new SSO session. It is idempotent per establishing flow execution:
	// on a FLOW_EXECUTION_ID conflict it does nothing, so concurrent joins in one execution converge
	// on the single session that won the race (the caller re-reads it via queryGetSessionByExecutionID).
	// The ON CONFLICT ... DO NOTHING form is valid in both PostgreSQL and SQLite.
	queryCreateSession = model.DBQuery{
		ID: "SEQ-SESS-01",
		Query: `INSERT INTO "SESSION" (SESSION_ID, DEPLOYMENT_ID, TYPE, SUBJECT_ID, FLOW_ID, FLOW_VERSION, ` +
			`FLOW_EXECUTION_ID, HANDLE_ID, ` +
			`AUTHENTICATED_AT, CREATED_AT, LAST_ACTIVE_AT, IDLE_EXPIRES_AT, ABSOLUTE_EXPIRES_AT, STATE, VERSION) ` +
			`VALUES ($1, $2, 'SSO', $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14) ` +
			`ON CONFLICT (FLOW_EXECUTION_ID, DEPLOYMENT_ID) DO NOTHING`,
	}

	// queryGetSessionByHandle fetches a session by its opaque handle ID. Liveness checks
	// (state, deadlines) are applied by the resolver, not here.
	queryGetSessionByHandle = model.DBQuery{
		ID: "SEQ-SESS-02",
		Query: `SELECT SESSION_ID, SUBJECT_ID, FLOW_ID, FLOW_VERSION, FLOW_EXECUTION_ID, HANDLE_ID, ` +
			`AUTHENTICATED_AT, CREATED_AT, LAST_ACTIVE_AT, IDLE_EXPIRES_AT, ABSOLUTE_EXPIRES_AT, STATE, VERSION ` +
			`FROM "SESSION" WHERE HANDLE_ID = $1 AND DEPLOYMENT_ID = $2 AND TYPE = 'SSO'`,
	}

	// queryGetSessionByExecutionID fetches the session established by a given flow execution, or no
	// rows when that execution has not established one. Used on the fresh join path to attach later
	// checkpoints to the session an earlier join in the same execution already created.
	queryGetSessionByExecutionID = model.DBQuery{
		ID: "SEQ-SESS-03",
		Query: `SELECT SESSION_ID, SUBJECT_ID, FLOW_ID, FLOW_VERSION, FLOW_EXECUTION_ID, HANDLE_ID, ` +
			`AUTHENTICATED_AT, CREATED_AT, LAST_ACTIVE_AT, IDLE_EXPIRES_AT, ABSOLUTE_EXPIRES_AT, STATE, VERSION ` +
			`FROM "SESSION" WHERE FLOW_EXECUTION_ID = $1 AND DEPLOYMENT_ID = $2 AND TYPE = 'SSO'`,
	}

	// queryUpdateSession updates the mutable fields of a session under an optimistic-lock
	// guard: it only matches when the stored VERSION equals the expected version, and it
	// bumps VERSION on success. It never touches the session context.
	queryUpdateSession = model.DBQuery{
		ID: "SEQ-SESS-04",
		Query: `UPDATE "SESSION" SET FLOW_VERSION = $1, HANDLE_ID = $2, ` +
			`LAST_ACTIVE_AT = $3, IDLE_EXPIRES_AT = $4, ABSOLUTE_EXPIRES_AT = $5, STATE = $6, ` +
			`VERSION = VERSION + 1, UPDATED_AT = CURRENT_TIMESTAMP ` +
			`WHERE SESSION_ID = $7 AND DEPLOYMENT_ID = $8 AND VERSION = $9`,
	}

	// queryTouchAuthenticatedAt refreshes the session's authentication time after the subject
	// authenticated again within an existing session (prompt=login, or a max_age the previous
	// authentication no longer satisfied). It is separate from queryUpdateSession because that
	// statement carries the optimistic-concurrency guard for liveness updates, whereas this one
	// records a fact about an authentication that has already happened: losing a race with a
	// concurrent slide must not discard it.
	queryTouchAuthenticatedAt = model.DBQuery{
		ID: "SEQ-SESS-16",
		Query: `UPDATE "SESSION" SET AUTHENTICATED_AT = $1, LAST_ACTIVE_AT = $2, ` +
			`IDLE_EXPIRES_AT = $3, VERSION = VERSION + 1, UPDATED_AT = CURRENT_TIMESTAMP ` +
			`WHERE SESSION_ID = $4 AND DEPLOYMENT_ID = $5 AND AUTHENTICATED_AT <= $1`,
	}

	// queryCreateSessionContext upserts a checkpoint's session context. Re-saving the same checkpoint
	// (re-execution or a concurrent request) overwrites it rather than erroring on the primary key.
	// The ON CONFLICT ... DO UPDATE form is valid in both PostgreSQL and SQLite.
	queryCreateSessionContext = model.DBQuery{
		ID: "SEQ-SESS-05",
		Query: `INSERT INTO "SSO_SESSION_CONTEXT" (SESSION_ID, DEPLOYMENT_ID, CHECKPOINT_ID, CONTEXT, ` +
			`CONTEXT_VERSION) VALUES ($1, $2, $3, $4, $5) ` +
			`ON CONFLICT (SESSION_ID, DEPLOYMENT_ID, CHECKPOINT_ID) DO UPDATE SET ` +
			`CONTEXT = excluded.CONTEXT, CONTEXT_VERSION = excluded.CONTEXT_VERSION`,
	}

	// queryGetSessionContextByCheckpoint fetches one checkpoint's session context for a session.
	queryGetSessionContextByCheckpoint = model.DBQuery{
		ID: "SEQ-SESS-06",
		Query: `SELECT SESSION_ID, CHECKPOINT_ID, CONTEXT, CONTEXT_VERSION FROM "SSO_SESSION_CONTEXT" ` +
			`WHERE SESSION_ID = $1 AND DEPLOYMENT_ID = $2 AND CHECKPOINT_ID = $3`,
	}

	// queryDeleteSessionContext removes all of a session's checkpoint contexts.
	queryDeleteSessionContext = model.DBQuery{
		ID:    "SEQ-SESS-07",
		Query: `DELETE FROM "SSO_SESSION_CONTEXT" WHERE SESSION_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryUpsertParticipant records an application as a participant of a session under an organization
	// unit, refreshing LAST_ACTIVE_AT and the current-grant TFID (but preserving FIRST_JOINED_AT) when the
	// application has already joined under that unit. TFID moves to the latest grant so logout revokes the
	// most recent family; another unit is another row with its own family.
	// The ON CONFLICT ... DO UPDATE form is valid in both PostgreSQL and SQLite.
	queryUpsertParticipant = model.DBQuery{
		ID: "SEQ-SESS-09",
		Query: `INSERT INTO "SESSION_PARTICIPANT" ` +
			`(SESSION_ID, DEPLOYMENT_ID, APP_ID, OU_ID, FIRST_JOINED_AT, LAST_ACTIVE_AT, TFID) ` +
			`VALUES ($1, $2, $3, $4, $5, $6, $7) ` +
			`ON CONFLICT (SESSION_ID, DEPLOYMENT_ID, APP_ID, OU_ID) DO UPDATE SET ` +
			`LAST_ACTIVE_AT = excluded.LAST_ACTIVE_AT, TFID = excluded.TFID`,
	}

	// queryListParticipantsBySessionID returns the applications that have joined a session, oldest
	// first.
	queryListParticipantsBySessionID = model.DBQuery{
		ID: "SEQ-SESS-10",
		Query: `SELECT SESSION_ID, APP_ID, OU_ID, FIRST_JOINED_AT, LAST_ACTIVE_AT, TFID ` +
			`FROM "SESSION_PARTICIPANT" WHERE SESSION_ID = $1 AND DEPLOYMENT_ID = $2 ORDER BY FIRST_JOINED_AT`,
	}

	// queryDeleteParticipantsBySessionID removes all participants of a session.
	queryDeleteParticipantsBySessionID = model.DBQuery{
		ID:    "SEQ-SESS-11",
		Query: `DELETE FROM "SESSION_PARTICIPANT" WHERE SESSION_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryDeleteSession removes the session row itself.
	queryDeleteSession = model.DBQuery{
		ID:    "SEQ-SESS-12",
		Query: `DELETE FROM "SESSION" WHERE SESSION_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryListSessionsBySubject returns all SSO sessions owned by a subject.
	queryListSessionsBySubject = model.DBQuery{
		ID: "SEQ-SESS-13",
		Query: `SELECT SESSION_ID, SUBJECT_ID, FLOW_ID, FLOW_VERSION, FLOW_EXECUTION_ID, HANDLE_ID, ` +
			`AUTHENTICATED_AT, CREATED_AT, LAST_ACTIVE_AT, IDLE_EXPIRES_AT, ABSOLUTE_EXPIRES_AT, STATE, VERSION ` +
			`FROM "SESSION" WHERE SUBJECT_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryListParticipantsByAppID returns every session the application participates in, oldest first.
	// Backed by idx_session_participant_app, since the table's primary key leads with SESSION_ID.
	queryListParticipantsByAppID = model.DBQuery{
		ID: "SEQ-SESS-14",
		Query: `SELECT SESSION_ID, APP_ID, OU_ID, FIRST_JOINED_AT, LAST_ACTIVE_AT, TFID ` +
			`FROM "SESSION_PARTICIPANT" WHERE APP_ID = $1 AND DEPLOYMENT_ID = $2 ORDER BY FIRST_JOINED_AT`,
	}

	// queryDeleteParticipant removes one application's participation in one session under one
	// organization unit, leaving the session's other participants intact.
	queryDeleteParticipant = model.DBQuery{
		ID: "SEQ-SESS-15",
		Query: `DELETE FROM "SESSION_PARTICIPANT" ` +
			`WHERE SESSION_ID = $1 AND APP_ID = $2 AND OU_ID = $3 AND DEPLOYMENT_ID = $4`,
	}

	// querySlideIdleDeadline refreshes a session's last-active timestamp and idle deadline. It matches
	// only an active, unexpired session whose last refresh is older than the throttle cutoff, so it
	// neither revives an expired session nor writes on every call.
	querySlideIdleDeadline = model.DBQuery{
		ID: "SEQ-SESS-18",
		Query: `UPDATE "SESSION" SET LAST_ACTIVE_AT = $1, IDLE_EXPIRES_AT = $2, ` +
			`VERSION = VERSION + 1, UPDATED_AT = CURRENT_TIMESTAMP ` +
			`WHERE SESSION_ID = $3 AND STATE = $4 AND LAST_ACTIVE_AT <= $5 ` +
			`AND (IDLE_EXPIRES_AT IS NULL OR IDLE_EXPIRES_AT > $1) ` +
			`AND (ABSOLUTE_EXPIRES_AT IS NULL OR ABSOLUTE_EXPIRES_AT > $1) AND DEPLOYMENT_ID = $6`,
	}

	// The live-session queries below apply the resolver's liveness rule in SQL, since expired rows
	// stay in the table until cleanup removes them: STATE is ACTIVE and neither deadline has been
	// reached at the caller-supplied time. A NULL deadline never expires.

	// queryListLiveSessionsBySubject returns one page of a subject's live sessions, most recently
	// active first. Backed by idx_session_subject.
	queryListLiveSessionsBySubject = model.DBQuery{
		ID: "SEQ-SESS-19",
		Query: `SELECT SESSION_ID, SUBJECT_ID, FLOW_ID, FLOW_VERSION, FLOW_EXECUTION_ID, HANDLE_ID, ` +
			`AUTHENTICATED_AT, CREATED_AT, LAST_ACTIVE_AT, IDLE_EXPIRES_AT, ABSOLUTE_EXPIRES_AT, ` +
			`PROPERTIES, STATE, VERSION ` +
			`FROM "SESSION" WHERE SUBJECT_ID = $1 AND STATE = 'ACTIVE' ` +
			`AND (IDLE_EXPIRES_AT IS NULL OR IDLE_EXPIRES_AT > $2) ` +
			`AND (ABSOLUTE_EXPIRES_AT IS NULL OR ABSOLUTE_EXPIRES_AT > $2) ` +
			`AND DEPLOYMENT_ID = $5 ` +
			`ORDER BY LAST_ACTIVE_AT DESC, SESSION_ID DESC LIMIT $3 OFFSET $4`,
	}

	// queryCountLiveSessionsBySubject counts a subject's live sessions.
	queryCountLiveSessionsBySubject = model.DBQuery{
		ID: "SEQ-SESS-20",
		Query: `SELECT COUNT(*) AS total FROM "SESSION" WHERE SUBJECT_ID = $1 AND STATE = 'ACTIVE' ` +
			`AND (IDLE_EXPIRES_AT IS NULL OR IDLE_EXPIRES_AT > $2) ` +
			`AND (ABSOLUTE_EXPIRES_AT IS NULL OR ABSOLUTE_EXPIRES_AT > $2) ` +
			`AND DEPLOYMENT_ID = $3`,
	}

	// queryListLiveSessionsByApp returns one page of the live sessions an application has joined,
	// most recently active first. An application signed in under several organization units has a
	// participant row per unit, so the sessions are matched through a subquery, which returns each one
	// once. Backed by idx_session_participant_app.
	queryListLiveSessionsByApp = model.DBQuery{
		ID: "SEQ-SESS-21",
		Query: `SELECT S.SESSION_ID, S.SUBJECT_ID, S.FLOW_ID, S.FLOW_VERSION, S.FLOW_EXECUTION_ID, ` +
			`S.HANDLE_ID, S.AUTHENTICATED_AT, S.CREATED_AT, S.LAST_ACTIVE_AT, S.IDLE_EXPIRES_AT, ` +
			`S.ABSOLUTE_EXPIRES_AT, S.PROPERTIES, S.STATE, S.VERSION ` +
			`FROM "SESSION" S ` +
			`WHERE S.SESSION_ID IN (SELECT P.SESSION_ID FROM "SESSION_PARTICIPANT" P ` +
			`WHERE P.APP_ID = $1 AND P.DEPLOYMENT_ID = $5) ` +
			`AND S.STATE = 'ACTIVE' ` +
			`AND (S.IDLE_EXPIRES_AT IS NULL OR S.IDLE_EXPIRES_AT > $2) ` +
			`AND (S.ABSOLUTE_EXPIRES_AT IS NULL OR S.ABSOLUTE_EXPIRES_AT > $2) ` +
			`AND S.DEPLOYMENT_ID = $5 ` +
			`ORDER BY S.LAST_ACTIVE_AT DESC, S.SESSION_ID DESC LIMIT $3 OFFSET $4`,
	}

	// queryCountLiveSessionsByApp counts the live sessions an application has joined, each once however
	// many organization units it signed in under.
	queryCountLiveSessionsByApp = model.DBQuery{
		ID: "SEQ-SESS-22",
		Query: `SELECT COUNT(*) AS total FROM "SESSION" S ` +
			`WHERE S.SESSION_ID IN (SELECT P.SESSION_ID FROM "SESSION_PARTICIPANT" P ` +
			`WHERE P.APP_ID = $1 AND P.DEPLOYMENT_ID = $3) ` +
			`AND S.STATE = 'ACTIVE' ` +
			`AND (S.IDLE_EXPIRES_AT IS NULL OR S.IDLE_EXPIRES_AT > $2) ` +
			`AND (S.ABSOLUTE_EXPIRES_AT IS NULL OR S.ABSOLUTE_EXPIRES_AT > $2) ` +
			`AND S.DEPLOYMENT_ID = $3`,
	}
)

// participantsBySessionIDsChunkSize caps the session ids per query, keeping it under the
// bind-parameter limit of either database.
const participantsBySessionIDsChunkSize = 200

// buildListParticipantsBySessionIDsQuery returns the participants of n sessions at once. Placeholders
// are $1..$n for the ids and $n+1 for the deployment; both databases accept the $N form.
func buildListParticipantsBySessionIDsQuery(n int) model.DBQuery {
	placeholders := make([]string, n)
	for i := range placeholders {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	return model.DBQuery{
		ID: "SEQ-SESS-17",
		Query: `SELECT SESSION_ID, APP_ID, OU_ID, FIRST_JOINED_AT, LAST_ACTIVE_AT, TFID ` +
			`FROM "SESSION_PARTICIPANT" ` +
			`WHERE SESSION_ID IN (` + strings.Join(placeholders, ", ") + `) AND DEPLOYMENT_ID = $` +
			fmt.Sprint(n+1) + ` ORDER BY SESSION_ID, FIRST_JOINED_AT`,
	}
}
