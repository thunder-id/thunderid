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
	"context"
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/internal/system/database/model"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// store is the single runtime-persistent-DB-backed persistence implementation for SSO sessions. It satisfies
// the sessionStore interface — session rows, per-checkpoint contexts, and participants all back onto
// the same runtime persistent datasource, so they share one struct rather than duplicating the provider and
// deployment id across parallel stores.
type store struct {
	dbProvider   provider.DBProviderInterface
	deploymentID string
}

// newStore creates the session store backed by the given runtime persistent DB provider. It returns the
// concrete type so Initialize can hand the one instance to each of the store interfaces the service
// depends on.
func newStore(dbProvider provider.DBProviderInterface, deploymentID string) *store {
	return &store{
		dbProvider:   dbProvider,
		deploymentID: deploymentID,
	}
}

// Create persists a new session.
func (st *store) Create(ctx context.Context, s Session) error {
	return withRuntimePersistentDBClient(st.dbProvider, func(dbClient provider.DBClientInterface) error {
		_, err := dbClient.ExecuteContext(ctx, queryCreateSession,
			s.SessionID, st.deploymentID, s.SubjectID, s.FlowID, s.FlowVersion,
			s.FlowExecutionID, s.HandleID,
			s.AuthenticatedAt, s.CreatedAt, s.LastActiveAt,
			nullableTime(s.IdleExpiresAt), nullableTime(s.AbsoluteExpiresAt), string(s.State), s.Version)
		if err != nil {
			return fmt.Errorf("failed to create session: %w", err)
		}
		return nil
	})
}

// GetByHandle fetches a session by its opaque handle ID.
func (st *store) GetByHandle(ctx context.Context, handleID string) (*Session, error) {
	return st.getSingle(ctx, queryGetSessionByHandle, handleID)
}

// GetByExecutionID fetches the session established by the given flow execution.
func (st *store) GetByExecutionID(ctx context.Context, flowExecutionID string) (*Session, error) {
	return st.getSingle(ctx, queryGetSessionByExecutionID, flowExecutionID)
}

// getSingle runs a single-key lookup query and maps the at-most-one row into a Session, returning
// (nil, nil) when no row matches.
func (st *store) getSingle(ctx context.Context, query model.DBQuery, key string) (*Session, error) {
	var result *Session

	err := withRuntimePersistentDBClient(st.dbProvider, func(dbClient provider.DBClientInterface) error {
		results, err := dbClient.QueryContext(ctx, query, key, st.deploymentID)
		if err != nil {
			return fmt.Errorf("failed to execute query: %w", err)
		}
		if len(results) == 0 {
			return nil
		}
		if len(results) != 1 {
			return fmt.Errorf("unexpected number of results: %d", len(results))
		}

		s, buildErr := buildSessionFromRow(results[0])
		if buildErr != nil {
			return buildErr
		}
		result = s
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Update writes the mutable fields of an existing session under an optimistic-lock guard. It
// touches only SESSION — never the auth context — so an activity touch stays lean.
func (st *store) Update(ctx context.Context, s *Session) error {
	return withRuntimePersistentDBClient(st.dbProvider, func(dbClient provider.DBClientInterface) error {
		rowsAffected, err := dbClient.ExecuteContext(ctx, queryUpdateSession,
			s.FlowVersion, s.HandleID,
			s.LastActiveAt,
			nullableTime(s.IdleExpiresAt), nullableTime(s.AbsoluteExpiresAt), string(s.State),
			s.SessionID, st.deploymentID, s.Version)
		if err != nil {
			return fmt.Errorf("failed to update session: %w", err)
		}
		if rowsAffected == 0 {
			return errVersionConflict
		}
		s.Version++
		return nil
	})
}

// CreateContext persists the session context. It rejects payloads exceeding MaxSessionContextBytes.
func (st *store) CreateContext(ctx context.Context, c SessionContext) error {
	payload, err := c.serializePayload()
	if err != nil {
		return err
	}
	if len(payload) > MaxSessionContextBytes {
		return errSessionContextTooLarge
	}

	return withRuntimePersistentDBClient(st.dbProvider, func(dbClient provider.DBClientInterface) error {
		_, execErr := dbClient.ExecuteContext(ctx, queryCreateSessionContext,
			c.SessionID, st.deploymentID, c.CheckpointID, payload, c.ContextVersion)
		if execErr != nil {
			return fmt.Errorf("failed to create session context: %w", execErr)
		}
		return nil
	})
}

// GetByCheckpoint fetches one checkpoint's session context.
func (st *store) GetByCheckpoint(ctx context.Context, sessionID,
	checkpointID string) (*SessionContext, error) {
	var result *SessionContext

	err := withRuntimePersistentDBClient(st.dbProvider, func(dbClient provider.DBClientInterface) error {
		results, queryErr := dbClient.QueryContext(ctx, queryGetSessionContextByCheckpoint,
			sessionID, st.deploymentID, checkpointID)
		if queryErr != nil {
			return fmt.Errorf("failed to execute query: %w", queryErr)
		}
		if len(results) == 0 {
			return nil
		}
		if len(results) != 1 {
			return fmt.Errorf("unexpected number of results: %d", len(results))
		}

		c, buildErr := st.buildSessionContextFromRow(results[0])
		if buildErr != nil {
			return buildErr
		}
		result = c
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ListCheckpointIDs returns the checkpoint ids a session has saved, without decrypting any payload.
func (st *store) ListCheckpointIDs(ctx context.Context, sessionID string) ([]string, error) {
	var ids []string

	err := withRuntimePersistentDBClient(st.dbProvider, func(dbClient provider.DBClientInterface) error {
		results, queryErr := dbClient.QueryContext(ctx, queryListCheckpointsBySessionID, sessionID, st.deploymentID)
		if queryErr != nil {
			return fmt.Errorf("failed to execute query: %w", queryErr)
		}
		for _, row := range results {
			id, parseErr := parseString(row["checkpoint_id"], "checkpoint_id")
			if parseErr != nil {
				return parseErr
			}
			ids = append(ids, id)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ids, nil
}

// Delete removes a session's session context.
func (st *store) Delete(ctx context.Context, sessionID string) error {
	return withRuntimePersistentDBClient(st.dbProvider, func(dbClient provider.DBClientInterface) error {
		_, err := dbClient.ExecuteContext(ctx, queryDeleteSessionContext, sessionID, st.deploymentID)
		if err != nil {
			return fmt.Errorf("failed to delete session context: %w", err)
		}
		return nil
	})
}

// DeleteSession removes the session row itself.
func (st *store) DeleteSession(ctx context.Context, sessionID string) error {
	return withRuntimePersistentDBClient(st.dbProvider, func(dbClient provider.DBClientInterface) error {
		_, err := dbClient.ExecuteContext(ctx, queryDeleteSession, sessionID, st.deploymentID)
		if err != nil {
			return fmt.Errorf("failed to delete session: %w", err)
		}
		return nil
	})
}

// buildSessionContextFromRow parses a result row into an SessionContext.
func (st *store) buildSessionContextFromRow(row map[string]interface{}) (*SessionContext, error) {
	sessionID, err := parseString(row["session_id"], "session_id")
	if err != nil {
		return nil, err
	}
	checkpointID, err := parseString(row["checkpoint_id"], "checkpoint_id")
	if err != nil {
		return nil, err
	}
	contextVersion, err := parseInt(row["context_version"], "context_version")
	if err != nil {
		return nil, err
	}
	payload, err := parseSessionContextPayload(parseNullableString(row["context"]))
	if err != nil {
		return nil, err
	}

	return &SessionContext{
		SessionID:      sessionID,
		CheckpointID:   checkpointID,
		RuntimeData:    payload.RuntimeData,
		AuthUser:       payload.AuthUser,
		CompletedSteps: payload.CompletedSteps,
		ContextVersion: contextVersion,
	}, nil
}

// Record inserts or refreshes a participant under the upsert query.
func (st *store) Record(ctx context.Context, p Participant) error {
	return withRuntimePersistentDBClient(st.dbProvider, func(dbClient provider.DBClientInterface) error {
		_, err := dbClient.ExecuteContext(ctx, queryUpsertParticipant,
			p.SessionID, st.deploymentID, p.AppID, p.FirstJoinedAt, p.LastActiveAt, p.TokenFamilyID)
		if err != nil {
			return fmt.Errorf("failed to record session participant: %w", err)
		}
		return nil
	})
}

// ListBySessionID returns the participants of a session, oldest first.
func (st *store) ListBySessionID(ctx context.Context, sessionID string) ([]Participant, error) {
	var result []Participant

	err := withRuntimePersistentDBClient(st.dbProvider, func(dbClient provider.DBClientInterface) error {
		results, queryErr := dbClient.QueryContext(ctx, queryListParticipantsBySessionID, sessionID, st.deploymentID)
		if queryErr != nil {
			return fmt.Errorf("failed to execute query: %w", queryErr)
		}
		for _, row := range results {
			p, buildErr := buildParticipantFromRow(row)
			if buildErr != nil {
				return buildErr
			}
			result = append(result, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// DeleteBySessionID removes all participants of a session.
func (st *store) DeleteBySessionID(ctx context.Context, sessionID string) error {
	return withRuntimePersistentDBClient(st.dbProvider, func(dbClient provider.DBClientInterface) error {
		_, err := dbClient.ExecuteContext(ctx, queryDeleteParticipantsBySessionID, sessionID, st.deploymentID)
		if err != nil {
			return fmt.Errorf("failed to delete session participants: %w", err)
		}
		return nil
	})
}

// buildParticipantFromRow maps a database result row into a Participant.
func buildParticipantFromRow(row map[string]interface{}) (Participant, error) {
	sessionID, err := parseString(row["session_id"], "session_id")
	if err != nil {
		return Participant{}, err
	}
	appID, err := parseString(row["app_id"], "app_id")
	if err != nil {
		return Participant{}, err
	}
	firstJoinedAt, err := sysutils.ParseDBTimeField(row["first_joined_at"], "first_joined_at")
	if err != nil {
		return Participant{}, err
	}
	lastActiveAt, err := sysutils.ParseDBTimeField(row["last_active_at"], "last_active_at")
	if err != nil {
		return Participant{}, err
	}
	return Participant{
		SessionID:     sessionID,
		AppID:         appID,
		TokenFamilyID: parseNullableString(row["tfid"]),
		FirstJoinedAt: firstJoinedAt,
		LastActiveAt:  lastActiveAt,
	}, nil
}

// withRuntimePersistentDBClient runs fn with an runtime persistent database client. SSO sessions are persistent
// state that must survive a runtime database flush, so they live in the runtime persistent datasource.
func withRuntimePersistentDBClient(dbProvider provider.DBProviderInterface,
	fn func(provider.DBClientInterface) error) error {
	dbClient, err := dbProvider.GetRuntimePersistentDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	return fn(dbClient)
}

// nullableTime returns nil for a zero time so nullable columns store NULL, otherwise the time.
func nullableTime(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return t
}

// buildSessionFromRow maps a database result row into a Session.
func buildSessionFromRow(row map[string]interface{}) (*Session, error) {
	sessionID, err := parseString(row["session_id"], "session_id")
	if err != nil {
		return nil, err
	}
	subjectID, err := parseString(row["subject_id"], "subject_id")
	if err != nil {
		return nil, err
	}
	flowID, err := parseString(row["flow_id"], "flow_id")
	if err != nil {
		return nil, err
	}
	flowVersion, err := parseInt(row["flow_version"], "flow_version")
	if err != nil {
		return nil, err
	}
	flowExecutionID, err := parseString(row["flow_execution_id"], "flow_execution_id")
	if err != nil {
		return nil, err
	}
	handleID, err := parseString(row["handle_id"], "handle_id")
	if err != nil {
		return nil, err
	}
	authenticatedAt, err := sysutils.ParseDBTimeField(row["authenticated_at"], "authenticated_at")
	if err != nil {
		return nil, err
	}
	createdAt, err := sysutils.ParseDBTimeField(row["created_at"], "created_at")
	if err != nil {
		return nil, err
	}
	lastActiveAt, err := sysutils.ParseDBTimeField(row["last_active_at"], "last_active_at")
	if err != nil {
		return nil, err
	}
	version, err := parseInt(row["version"], "version")
	if err != nil {
		return nil, err
	}

	return &Session{
		SessionID:         sessionID,
		SubjectID:         subjectID,
		FlowID:            flowID,
		FlowVersion:       flowVersion,
		FlowExecutionID:   flowExecutionID,
		HandleID:          handleID,
		AuthenticatedAt:   authenticatedAt,
		CreatedAt:         createdAt,
		LastActiveAt:      lastActiveAt,
		IdleExpiresAt:     parseNullableTime(row["idle_expires_at"]),
		AbsoluteExpiresAt: parseNullableTime(row["absolute_expires_at"]),
		State:             State(parseNullableString(row["state"])),
		Version:           version,
	}, nil
}

// parseString parses a required string column.
func parseString(value interface{}, field string) (string, error) {
	if s := parseNullableStringPtr(value); s != nil {
		return *s, nil
	}
	return "", fmt.Errorf("failed to parse %s as string", field)
}

// parseNullableString parses an optional string column, returning "" when null.
func parseNullableString(value interface{}) string {
	if s := parseNullableStringPtr(value); s != nil {
		return *s
	}
	return ""
}

// parseNullableStringPtr parses a string column, handling the []byte form some drivers return.
func parseNullableStringPtr(value interface{}) *string {
	switch v := value.(type) {
	case string:
		return &v
	case []byte:
		s := string(v)
		return &s
	default:
		return nil
	}
}

// parseInt parses an integer column across the numeric forms drivers may return.
func parseInt(value interface{}, field string) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	default:
		return 0, fmt.Errorf("failed to parse %s as int: got %T", field, value)
	}
}

// parseNullableTime parses an optional time column, returning the zero time when null or
// unparseable.
func parseNullableTime(value interface{}) time.Time {
	if value == nil {
		return time.Time{}
	}
	t, err := sysutils.ParseDBTimeField(value, "")
	if err != nil {
		return time.Time{}
	}
	return t
}
