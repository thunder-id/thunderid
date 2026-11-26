// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"fmt"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/database/model"
)

var (
	// queryCreateSession inserts a new session. It is idempotent per establishing flow execution:
	// on a FLOW_EXECUTION_ID conflict it does nothing, so concurrent joins in one execution converge
	// on the single session that won the race (the caller re-reads it via queryGetSessionByExecutionID).
	// A direct session has no flow execution, so it never conflicts.
	// The ON CONFLICT ... DO NOTHING form is valid in both PostgreSQL and SQLite.
	queryCreateSession = model.DBQuery{
		ID: "SEQ-SESS-01",
		Query: `INSERT INTO "SESSION" (SESSION_ID, TYPE, SUBJECT_ID, FLOW_ID, FLOW_VERSION, ` +
			`FLOW_EXECUTION_ID, HANDLE_ID, ` +
			`AUTHENTICATED_AT, CREATED_AT, LAST_ACTIVE_AT, IDLE_EXPIRES_AT, ABSOLUTE_EXPIRES_AT, ` +
			`PROPERTIES, STATE, VERSION, DEPLOYMENT_ID) ` +
			`VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16) ` +
			`ON CONFLICT (FLOW_EXECUTION_ID, DEPLOYMENT_ID) DO NOTHING`,
	}

	// queryGetSessionByHandle fetches a session by its opaque handle ID. Liveness checks
	// (state, deadlines) are applied by the resolver, not here.
	queryGetSessionByHandle = model.DBQuery{
		ID: "SEQ-SESS-02",
		Query: `SELECT SESSION_ID, TYPE, SUBJECT_ID, FLOW_ID, FLOW_VERSION, FLOW_EXECUTION_ID, HANDLE_ID, ` +
			`AUTHENTICATED_AT, CREATED_AT, LAST_ACTIVE_AT, IDLE_EXPIRES_AT, ABSOLUTE_EXPIRES_AT, ` +
			`PROPERTIES, STATE, VERSION ` +
			`FROM "SESSION" WHERE HANDLE_ID = $1 AND DEPLOYMENT_ID = $2 AND TYPE = 'SSO'`,
	}

	// queryGetSessionByExecutionID fetches the session established by a given flow execution, or no
	// rows when that execution has not established one. Used on the fresh join path to attach later
	// checkpoints to the session an earlier join in the same execution already created.
	queryGetSessionByExecutionID = model.DBQuery{
		ID: "SEQ-SESS-03",
		Query: `SELECT SESSION_ID, TYPE, SUBJECT_ID, FLOW_ID, FLOW_VERSION, FLOW_EXECUTION_ID, HANDLE_ID, ` +
			`AUTHENTICATED_AT, CREATED_AT, LAST_ACTIVE_AT, IDLE_EXPIRES_AT, ABSOLUTE_EXPIRES_AT, ` +
			`PROPERTIES, STATE, VERSION ` +
			`FROM "SESSION" WHERE FLOW_EXECUTION_ID = $1 AND DEPLOYMENT_ID = $2 AND TYPE = 'SSO'`,
	}

	// queryUpdateSession updates the mutable fields of a session under an optimistic-lock
	// guard: it only matches when the stored VERSION equals the expected version, and it
	// bumps VERSION on success. It never touches the session context.
	queryUpdateSession = model.DBQuery{
		ID: "SEQ-SESS-04",
		Query: `UPDATE "SESSION" SET FLOW_VERSION = $1, HANDLE_ID = $2, ` +
			`LAST_ACTIVE_AT = $3, PROPERTIES = $4, IDLE_EXPIRES_AT = $5, ABSOLUTE_EXPIRES_AT = $6, ` +
			`STATE = $7, VERSION = VERSION + 1, UPDATED_AT = CURRENT_TIMESTAMP ` +
			`WHERE SESSION_ID = $8 AND DEPLOYMENT_ID = $9 AND VERSION = $10`,
	}

	// queryTouchAuthenticatedAt refreshes the session's authentication time after the subject
	// authenticated again within an existing session (prompt=login, or a max_age the previous
	// authentication no longer satisfied). It is separate from queryUpdateSession because that
	// statement carries the optimistic-concurrency guard for liveness updates, whereas this one
	// records a fact about an authentication that has already happened: losing a race with a
	// concurrent slide must not discard it.
	queryTouchAuthenticatedAt = model.DBQuery{
		ID: "SEQ-SESS-16",
		Query: `UPDATE "SESSION" SET AUTHENTICATED_AT = $1, LAST_ACTIVE_AT = $2, PROPERTIES = $3, ` +
			`IDLE_EXPIRES_AT = $4, VERSION = VERSION + 1, UPDATED_AT = CURRENT_TIMESTAMP ` +
			`WHERE SESSION_ID = $5 AND DEPLOYMENT_ID = $6 AND AUTHENTICATED_AT <= $1`,
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

	// queryUpsertParticipant records an application as a participant of a session under an organization unit
	// and returns its TFID. A rejoin refreshes LAST_ACTIVE_AT and keeps a non-empty TFID. The ON CONFLICT ...
	// RETURNING form is valid in both PostgreSQL and SQLite.
	queryUpsertParticipant = model.DBQuery{
		ID: "SEQ-SESS-09",
		Query: `INSERT INTO "SESSION_PARTICIPANT" ` +
			`(SESSION_ID, DEPLOYMENT_ID, APP_ID, OU_ID, FIRST_JOINED_AT, LAST_ACTIVE_AT, TFID) ` +
			`VALUES ($1, $2, $3, $4, $5, $6, $7) ` +
			`ON CONFLICT (SESSION_ID, DEPLOYMENT_ID, APP_ID, OU_ID) DO UPDATE SET ` +
			`LAST_ACTIVE_AT = excluded.LAST_ACTIVE_AT, ` +
			`TFID = COALESCE(NULLIF("SESSION_PARTICIPANT".TFID, ''), excluded.TFID) ` +
			`RETURNING TFID`,
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

	// queryListSessionsBySubject returns every session owned by a subject, SSO and direct.
	queryListSessionsBySubject = model.DBQuery{
		ID: "SEQ-SESS-13",
		Query: `SELECT SESSION_ID, TYPE, SUBJECT_ID, FLOW_ID, FLOW_VERSION, FLOW_EXECUTION_ID, HANDLE_ID, ` +
			`AUTHENTICATED_AT, CREATED_AT, LAST_ACTIVE_AT, IDLE_EXPIRES_AT, ABSOLUTE_EXPIRES_AT, ` +
			`PROPERTIES, STATE, VERSION ` +
			`FROM "SESSION" WHERE SUBJECT_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryListParticipantsByAppID returns every session the application participates in, oldest first, with
	// the subject of each session. The join is a LEFT JOIN so a participation whose session row is gone is
	// still returned, with no subject. Backed by idx_session_participant_app, since the table's primary key
	// leads with SESSION_ID.
	queryListParticipantsByAppID = model.DBQuery{
		ID: "SEQ-SESS-14",
		Query: `SELECT p.SESSION_ID, p.APP_ID, p.OU_ID, p.FIRST_JOINED_AT, p.LAST_ACTIVE_AT, p.TFID, ` +
			`s.SUBJECT_ID ` +
			`FROM "SESSION_PARTICIPANT" p LEFT JOIN "SESSION" s ` +
			`ON s.SESSION_ID = p.SESSION_ID AND s.DEPLOYMENT_ID = p.DEPLOYMENT_ID ` +
			`WHERE p.APP_ID = $1 AND p.DEPLOYMENT_ID = $2 ORDER BY p.FIRST_JOINED_AT`,
	}

	// queryDeleteParticipant removes one application's participation in one session under one
	// organization unit, leaving the session's other participants intact.
	queryDeleteParticipant = model.DBQuery{
		ID: "SEQ-SESS-15",
		Query: `DELETE FROM "SESSION_PARTICIPANT" ` +
			`WHERE SESSION_ID = $1 AND APP_ID = $2 AND OU_ID = $3 AND DEPLOYMENT_ID = $4`,
	}

	// querySlideIdleDeadline refreshes a session's last-active timestamp and idle deadline. It matches
	// only an active session whose last refresh is older than the throttle cutoff and that is within
	// both its stored deadlines and the timeouts in effect now, so it neither revives an expired
	// session nor writes on every call.
	querySlideIdleDeadline = model.DBQuery{
		ID: "SEQ-SESS-18",
		Query: `UPDATE "SESSION" SET LAST_ACTIVE_AT = $1, IDLE_EXPIRES_AT = $2, ` +
			`VERSION = VERSION + 1, UPDATED_AT = CURRENT_TIMESTAMP ` +
			`WHERE SESSION_ID = $3 AND STATE = $4 AND LAST_ACTIVE_AT <= $5 ` +
			`AND IDLE_EXPIRES_AT > $1 AND ABSOLUTE_EXPIRES_AT > $1 ` +
			`AND LAST_ACTIVE_AT > $6 AND CREATED_AT > $7 AND DEPLOYMENT_ID = $8`,
	}

	// The live-session queries below list SSO sessions only, and apply the resolver's liveness rule in
	// SQL, since expired rows stay in the table until cleanup removes them: STATE is ACTIVE and neither
	// deadline has been reached at the caller-supplied time. A NULL deadline never expires.

	// queryListLiveSessionsBySubject returns one page of a subject's live sessions, most recently
	// active first. Backed by idx_session_subject.
	queryListLiveSessionsBySubject = model.DBQuery{
		ID: "SEQ-SESS-19",
		Query: `SELECT SESSION_ID, TYPE, SUBJECT_ID, FLOW_ID, FLOW_VERSION, FLOW_EXECUTION_ID, HANDLE_ID, ` +
			`AUTHENTICATED_AT, CREATED_AT, LAST_ACTIVE_AT, IDLE_EXPIRES_AT, ABSOLUTE_EXPIRES_AT, ` +
			`PROPERTIES, STATE, VERSION ` +
			`FROM "SESSION" WHERE SUBJECT_ID = $1 AND TYPE = 'SSO' AND STATE = 'ACTIVE' ` +
			`AND IDLE_EXPIRES_AT > $2 AND ABSOLUTE_EXPIRES_AT > $2 ` +
			`AND DEPLOYMENT_ID = $5 ` +
			`ORDER BY LAST_ACTIVE_AT DESC, SESSION_ID DESC LIMIT $3 OFFSET $4`,
	}

	// queryCountLiveSessionsBySubject counts a subject's live sessions.
	queryCountLiveSessionsBySubject = model.DBQuery{
		ID: "SEQ-SESS-20",
		Query: `SELECT COUNT(*) AS total FROM "SESSION" WHERE SUBJECT_ID = $1 AND TYPE = 'SSO' AND STATE = 'ACTIVE' ` +
			`AND IDLE_EXPIRES_AT > $2 AND ABSOLUTE_EXPIRES_AT > $2 ` +
			`AND DEPLOYMENT_ID = $3`,
	}

	// queryListLiveSessionsByApp returns one page of the live sessions an application has joined,
	// most recently active first. An application signed in under several organization units has a
	// participant row per unit, so the sessions are matched through a subquery, which returns each one
	// once. Backed by idx_session_participant_app.
	queryListLiveSessionsByApp = model.DBQuery{
		ID: "SEQ-SESS-21",
		Query: `SELECT S.SESSION_ID, S.TYPE, S.SUBJECT_ID, S.FLOW_ID, S.FLOW_VERSION, S.FLOW_EXECUTION_ID, ` +
			`S.HANDLE_ID, S.AUTHENTICATED_AT, S.CREATED_AT, S.LAST_ACTIVE_AT, S.IDLE_EXPIRES_AT, ` +
			`S.ABSOLUTE_EXPIRES_AT, S.PROPERTIES, S.STATE, S.VERSION ` +
			`FROM "SESSION" S ` +
			`WHERE S.SESSION_ID IN (SELECT P.SESSION_ID FROM "SESSION_PARTICIPANT" P ` +
			`WHERE P.APP_ID = $1 AND P.DEPLOYMENT_ID = $5) ` +
			`AND S.TYPE = 'SSO' AND S.STATE = 'ACTIVE' ` +
			`AND S.IDLE_EXPIRES_AT > $2 AND S.ABSOLUTE_EXPIRES_AT > $2 ` +
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
			`AND S.TYPE = 'SSO' AND S.STATE = 'ACTIVE' ` +
			`AND S.IDLE_EXPIRES_AT > $2 AND S.ABSOLUTE_EXPIRES_AT > $2 ` +
			`AND S.DEPLOYMENT_ID = $3`,
	}

	// The InOU variants below keep only the sessions with an application signed in under one organization
	// unit, for a caller confined to it.

	// queryListLiveSessionsBySubjectInOU returns one page of a subject's live sessions with an application
	// signed in under the organization unit, most recently active first. Backed by idx_session_subject, and
	// the participant check by the participant table's primary key.
	queryListLiveSessionsBySubjectInOU = model.DBQuery{
		ID: "SEQ-SESS-23",
		Query: `SELECT S.SESSION_ID, S.TYPE, S.SUBJECT_ID, S.FLOW_ID, S.FLOW_VERSION, S.FLOW_EXECUTION_ID, ` +
			`S.HANDLE_ID, S.AUTHENTICATED_AT, S.CREATED_AT, S.LAST_ACTIVE_AT, S.IDLE_EXPIRES_AT, ` +
			`S.ABSOLUTE_EXPIRES_AT, S.PROPERTIES, S.STATE, S.VERSION ` +
			`FROM "SESSION" S WHERE S.SUBJECT_ID = $1 ` +
			`AND EXISTS (SELECT 1 FROM "SESSION_PARTICIPANT" P WHERE P.SESSION_ID = S.SESSION_ID ` +
			`AND P.DEPLOYMENT_ID = S.DEPLOYMENT_ID AND P.OU_ID = $2) ` +
			`AND S.TYPE = 'SSO' AND S.STATE = 'ACTIVE' ` +
			`AND S.IDLE_EXPIRES_AT > $3 AND S.ABSOLUTE_EXPIRES_AT > $3 ` +
			`AND S.DEPLOYMENT_ID = $6 ` +
			`ORDER BY S.LAST_ACTIVE_AT DESC, S.SESSION_ID DESC LIMIT $4 OFFSET $5`,
	}

	// queryCountLiveSessionsBySubjectInOU counts a subject's live sessions with an application signed in
	// under the organization unit.
	queryCountLiveSessionsBySubjectInOU = model.DBQuery{
		ID: "SEQ-SESS-24",
		Query: `SELECT COUNT(*) AS total FROM "SESSION" S WHERE S.SUBJECT_ID = $1 ` +
			`AND EXISTS (SELECT 1 FROM "SESSION_PARTICIPANT" P WHERE P.SESSION_ID = S.SESSION_ID ` +
			`AND P.DEPLOYMENT_ID = S.DEPLOYMENT_ID AND P.OU_ID = $2) ` +
			`AND S.TYPE = 'SSO' AND S.STATE = 'ACTIVE' ` +
			`AND S.IDLE_EXPIRES_AT > $3 AND S.ABSOLUTE_EXPIRES_AT > $3 ` +
			`AND S.DEPLOYMENT_ID = $4`,
	}

	// queryListLiveSessionsByAppInOU returns one page of the live sessions the application has joined under
	// the organization unit, most recently active first. Backed by idx_session_participant_app.
	queryListLiveSessionsByAppInOU = model.DBQuery{
		ID: "SEQ-SESS-25",
		Query: `SELECT S.SESSION_ID, S.TYPE, S.SUBJECT_ID, S.FLOW_ID, S.FLOW_VERSION, S.FLOW_EXECUTION_ID, ` +
			`S.HANDLE_ID, S.AUTHENTICATED_AT, S.CREATED_AT, S.LAST_ACTIVE_AT, S.IDLE_EXPIRES_AT, ` +
			`S.ABSOLUTE_EXPIRES_AT, S.PROPERTIES, S.STATE, S.VERSION ` +
			`FROM "SESSION" S ` +
			`WHERE S.SESSION_ID IN (SELECT P.SESSION_ID FROM "SESSION_PARTICIPANT" P ` +
			`WHERE P.APP_ID = $1 AND P.OU_ID = $2 AND P.DEPLOYMENT_ID = $6) ` +
			`AND S.TYPE = 'SSO' AND S.STATE = 'ACTIVE' ` +
			`AND S.IDLE_EXPIRES_AT > $3 AND S.ABSOLUTE_EXPIRES_AT > $3 ` +
			`AND S.DEPLOYMENT_ID = $6 ` +
			`ORDER BY S.LAST_ACTIVE_AT DESC, S.SESSION_ID DESC LIMIT $4 OFFSET $5`,
	}

	// queryCountLiveSessionsByAppInOU counts the live sessions the application has joined under the
	// organization unit.
	queryCountLiveSessionsByAppInOU = model.DBQuery{
		ID: "SEQ-SESS-26",
		Query: `SELECT COUNT(*) AS total FROM "SESSION" S ` +
			`WHERE S.SESSION_ID IN (SELECT P.SESSION_ID FROM "SESSION_PARTICIPANT" P ` +
			`WHERE P.APP_ID = $1 AND P.OU_ID = $2 AND P.DEPLOYMENT_ID = $4) ` +
			`AND S.TYPE = 'SSO' AND S.STATE = 'ACTIVE' ` +
			`AND S.IDLE_EXPIRES_AT > $3 AND S.ABSOLUTE_EXPIRES_AT > $3 ` +
			`AND S.DEPLOYMENT_ID = $4`,
	}

	// queryGetSubjectParticipantByTokenFamily returns the participant of one of the subject's SSO sessions
	// that holds the token family. The subject's sessions bound the lookup through idx_session_subject.
	queryGetSubjectParticipantByTokenFamily = model.DBQuery{
		ID: "SEQ-SESS-27",
		Query: `SELECT SESSION_ID, APP_ID, OU_ID, FIRST_JOINED_AT, LAST_ACTIVE_AT, TFID ` +
			`FROM "SESSION_PARTICIPANT" WHERE TFID = $1 AND DEPLOYMENT_ID = $3 ` +
			`AND SESSION_ID IN (SELECT SESSION_ID FROM "SESSION" ` +
			`WHERE SUBJECT_ID = $2 AND TYPE = 'SSO' AND DEPLOYMENT_ID = $3)`,
	}

	// queryReplaceParticipantTokenFamily moves a participant to a new TFID only while it holds the expected one,
	// so concurrent joins converge, and returns the TFID it holds.
	queryReplaceParticipantTokenFamily = model.DBQuery{
		ID: "SEQ-SESS-28",
		Query: `UPDATE "SESSION_PARTICIPANT" SET TFID = CASE WHEN TFID = $1 THEN $2 ELSE TFID END ` +
			`WHERE SESSION_ID = $3 AND APP_ID = $4 AND OU_ID = $5 AND DEPLOYMENT_ID = $6 RETURNING TFID`,
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
