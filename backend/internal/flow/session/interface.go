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

import "context"

// sessionStore is the package-private persistence contract covering SSO sessions, their
// per-checkpoint session contexts, and their participants. A single operation-DB-backed
// implementation (store) satisfies it, and the service depends on this one interface. It is not
// used outside the package.
type sessionStore interface {
	// Create persists a new session.
	Create(ctx context.Context, s Session) error
	// GetByHandle fetches a session by its opaque handle ID. It returns (nil, nil) when no
	// session matches; liveness checks are the resolver's responsibility.
	GetByHandle(ctx context.Context, handleID string) (*Session, error)
	// GetByExecutionID fetches the session established by the given flow execution, or (nil, nil)
	// when that execution has not established one.
	GetByExecutionID(ctx context.Context, flowExecutionID string) (*Session, error)
	// Update writes the mutable fields of an existing session under an optimistic-lock guard. It
	// returns errVersionConflict when the stored version no longer matches, and bumps the in-memory
	// Version on success.
	Update(ctx context.Context, s *Session) error

	// CreateContext persists (or overwrites) one checkpoint's session context for a session.
	CreateContext(ctx context.Context, c SessionContext) error
	// GetByCheckpoint fetches one checkpoint's session context. It returns (nil, nil) when none exists.
	GetByCheckpoint(ctx context.Context, sessionID, checkpointID string) (*SessionContext, error)
	// Delete removes all of a session's checkpoint contexts.
	Delete(ctx context.Context, sessionID string) error
	// ListCheckpointIDs returns the checkpoint ids a session has saved, without loading any context
	// payload — the existence check the SSO-Check node uses to decide checkpoint availability.
	ListCheckpointIDs(ctx context.Context, sessionID string) ([]string, error)

	// Record inserts the participant, or refreshes its LAST_ACTIVE_AT (preserving FIRST_JOINED_AT)
	// when the application has already joined the session.
	Record(ctx context.Context, p Participant) error
	// ListBySessionID returns the applications that have joined the session, oldest first.
	ListBySessionID(ctx context.Context, sessionID string) ([]Participant, error)
	// DeleteBySessionID removes all participants of a session.
	DeleteBySessionID(ctx context.Context, sessionID string) error
}
