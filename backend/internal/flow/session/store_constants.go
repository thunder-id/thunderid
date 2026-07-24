/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package session

import (
	"github.com/thunder-id/thunderid/internal/system/database/model"
)

var (
	// queryCreateSession inserts a new SSO session. It is idempotent per establishing flow execution:
	// on a FLOW_EXECUTION_ID conflict it does nothing, so concurrent joins in one execution converge
	// on the single session that won the race (the caller re-reads it via queryGetSessionByExecutionID).
	// The ON CONFLICT ... DO NOTHING form is valid in both PostgreSQL and SQLite.
	queryCreateSession = model.DBQuery{
		ID: "SSO-SESS-01",
		Query: `INSERT INTO "SSO_SESSION" (SESSION_ID, DEPLOYMENT_ID, SUBJECT_ID, FLOW_ID, FLOW_VERSION, ` +
			`FLOW_EXECUTION_ID, HANDLE_ID, ` +
			`AUTHENTICATED_AT, CREATED_AT, LAST_ACTIVE_AT, IDLE_EXPIRES_AT, ABSOLUTE_EXPIRES_AT, STATE, VERSION) ` +
			`VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14) ` +
			`ON CONFLICT (FLOW_EXECUTION_ID, DEPLOYMENT_ID) DO NOTHING`,
	}

	// queryGetSessionByHandle fetches a session by its opaque handle ID. Liveness checks
	// (state, deadlines) are applied by the resolver, not here.
	queryGetSessionByHandle = model.DBQuery{
		ID: "SSO-SESS-02",
		Query: `SELECT SESSION_ID, SUBJECT_ID, FLOW_ID, FLOW_VERSION, FLOW_EXECUTION_ID, HANDLE_ID, ` +
			`AUTHENTICATED_AT, CREATED_AT, LAST_ACTIVE_AT, IDLE_EXPIRES_AT, ABSOLUTE_EXPIRES_AT, STATE, VERSION ` +
			`FROM "SSO_SESSION" WHERE HANDLE_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryGetSessionByExecutionID fetches the session established by a given flow execution, or no
	// rows when that execution has not established one. Used on the fresh join path to attach later
	// checkpoints to the session an earlier join in the same execution already created.
	queryGetSessionByExecutionID = model.DBQuery{
		ID: "SSO-SESS-03",
		Query: `SELECT SESSION_ID, SUBJECT_ID, FLOW_ID, FLOW_VERSION, FLOW_EXECUTION_ID, HANDLE_ID, ` +
			`AUTHENTICATED_AT, CREATED_AT, LAST_ACTIVE_AT, IDLE_EXPIRES_AT, ABSOLUTE_EXPIRES_AT, STATE, VERSION ` +
			`FROM "SSO_SESSION" WHERE FLOW_EXECUTION_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryUpdateSession updates the mutable fields of a session under an optimistic-lock
	// guard: it only matches when the stored VERSION equals the expected version, and it
	// bumps VERSION on success. It never touches the session context.
	queryUpdateSession = model.DBQuery{
		ID: "SSO-SESS-04",
		Query: `UPDATE "SSO_SESSION" SET FLOW_VERSION = $1, HANDLE_ID = $2, ` +
			`LAST_ACTIVE_AT = $3, IDLE_EXPIRES_AT = $4, ABSOLUTE_EXPIRES_AT = $5, STATE = $6, ` +
			`VERSION = VERSION + 1, UPDATED_AT = CURRENT_TIMESTAMP ` +
			`WHERE SESSION_ID = $7 AND DEPLOYMENT_ID = $8 AND VERSION = $9`,
	}

	// queryCreateSessionContext upserts a checkpoint's session context. Re-saving the same checkpoint
	// (re-execution or a concurrent request) overwrites it rather than erroring on the primary key.
	// The ON CONFLICT ... DO UPDATE form is valid in both PostgreSQL and SQLite.
	queryCreateSessionContext = model.DBQuery{
		ID: "SSO-SESS-05",
		Query: `INSERT INTO "SSO_SESSION_CONTEXT" (SESSION_ID, DEPLOYMENT_ID, CHECKPOINT_ID, CONTEXT, ` +
			`CONTEXT_VERSION) VALUES ($1, $2, $3, $4, $5) ` +
			`ON CONFLICT (SESSION_ID, DEPLOYMENT_ID, CHECKPOINT_ID) DO UPDATE SET ` +
			`CONTEXT = excluded.CONTEXT, CONTEXT_VERSION = excluded.CONTEXT_VERSION`,
	}

	// queryGetSessionContextByCheckpoint fetches one checkpoint's session context for a session.
	queryGetSessionContextByCheckpoint = model.DBQuery{
		ID: "SSO-SESS-06",
		Query: `SELECT SESSION_ID, CHECKPOINT_ID, CONTEXT, CONTEXT_VERSION FROM "SSO_SESSION_CONTEXT" ` +
			`WHERE SESSION_ID = $1 AND DEPLOYMENT_ID = $2 AND CHECKPOINT_ID = $3`,
	}

	// queryDeleteSessionContext removes all of a session's checkpoint contexts.
	queryDeleteSessionContext = model.DBQuery{
		ID:    "SSO-SESS-07",
		Query: `DELETE FROM "SSO_SESSION_CONTEXT" WHERE SESSION_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryListCheckpointsBySessionID returns the checkpoint ids a session has saved. It is the
	// existence check the SSO-Check node uses to decide availability without decrypting any context.
	queryListCheckpointsBySessionID = model.DBQuery{
		ID: "SSO-SESS-08",
		Query: `SELECT CHECKPOINT_ID FROM "SSO_SESSION_CONTEXT" ` +
			`WHERE SESSION_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryUpsertParticipant records an application as a participant of a session, refreshing
	// LAST_ACTIVE_AT and the current-grant TFID (but preserving FIRST_JOINED_AT) when the application
	// has already joined. TFID moves to the latest grant so logout revokes the most recent family.
	// The ON CONFLICT ... DO UPDATE form is valid in both PostgreSQL and SQLite.
	queryUpsertParticipant = model.DBQuery{
		ID: "SSO-SESS-09",
		Query: `INSERT INTO "SSO_SESSION_PARTICIPANT" ` +
			`(SESSION_ID, DEPLOYMENT_ID, APP_ID, FIRST_JOINED_AT, LAST_ACTIVE_AT, TFID) ` +
			`VALUES ($1, $2, $3, $4, $5, $6) ` +
			`ON CONFLICT (SESSION_ID, DEPLOYMENT_ID, APP_ID) DO UPDATE SET ` +
			`LAST_ACTIVE_AT = excluded.LAST_ACTIVE_AT, TFID = excluded.TFID`,
	}

	// queryListParticipantsBySessionID returns the applications that have joined a session, oldest
	// first.
	queryListParticipantsBySessionID = model.DBQuery{
		ID: "SSO-SESS-10",
		Query: `SELECT SESSION_ID, APP_ID, FIRST_JOINED_AT, LAST_ACTIVE_AT, TFID FROM "SSO_SESSION_PARTICIPANT" ` +
			`WHERE SESSION_ID = $1 AND DEPLOYMENT_ID = $2 ORDER BY FIRST_JOINED_AT`,
	}

	// queryDeleteParticipantsBySessionID removes all participants of a session.
	queryDeleteParticipantsBySessionID = model.DBQuery{
		ID:    "SSO-SESS-11",
		Query: `DELETE FROM "SSO_SESSION_PARTICIPANT" WHERE SESSION_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryDeleteSession removes the session row itself.
	queryDeleteSession = model.DBQuery{
		ID:    "SSO-SESS-12",
		Query: `DELETE FROM "SSO_SESSION" WHERE SESSION_ID = $1 AND DEPLOYMENT_ID = $2`,
	}
)
