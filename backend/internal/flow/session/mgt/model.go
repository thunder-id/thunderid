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

package mgt

import (
	"time"

	flowsession "github.com/thunder-id/thunderid/internal/flow/session"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// participantResponse is one application that has joined a session. AppName is resolved
// server-side and omitted when it cannot be resolved, so the client falls back to AppID.
type participantResponse struct {
	AppID         string `json:"appId"`
	AppName       string `json:"appName,omitempty"`
	FirstJoinedAt string `json:"firstJoinedAt"`
	LastActiveAt  string `json:"lastActiveAt"`
}

// sessionResponse is the client-facing view of a live session. It never carries the session
// handle (the cookie credential). UserName is resolved server-side and omitted when it cannot be
// resolved, so the client falls back to UserID.
type sessionResponse struct {
	ID                string                `json:"id"`
	UserID            string                `json:"userId"`
	UserName          string                `json:"userName,omitempty"`
	LoginFlowID       string                `json:"loginFlowId"`
	AuthenticatedAt   string                `json:"authenticatedAt"`
	CreatedAt         string                `json:"createdAt"`
	LastActiveAt      string                `json:"lastActiveAt"`
	IdleExpiresAt     string                `json:"idleExpiresAt,omitempty"`
	AbsoluteExpiresAt string                `json:"absoluteExpiresAt,omitempty"`
	Participants      []participantResponse `json:"participants"`
}

// sessionListResponse is the paginated payload for GET /sessions and GET /sessions/me.
type sessionListResponse struct {
	TotalResults int               `json:"totalResults"`
	StartIndex   int               `json:"startIndex"`
	Count        int               `json:"count"`
	Sessions     []sessionResponse `json:"sessions"`
	Links        []sysutils.Link   `json:"links"`
}

// formatTime renders a timestamp as RFC 3339 UTC, or "" for the zero value (unset deadline).
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// toSessionResponse maps a session and its participants to the client-facing representation.
// userName is the resolved display name for the session's subject, and appNames maps each
// participant's app id to its resolved name; both may be empty, in which case the id is used.
func toSessionResponse(s flowsession.Session, parts []flowsession.Participant,
	userName string, appNames map[string]string) sessionResponse {
	participants := make([]participantResponse, 0, len(parts))
	for _, p := range parts {
		participants = append(participants, participantResponse{
			AppID:         p.AppID,
			AppName:       appNames[p.AppID],
			FirstJoinedAt: formatTime(p.FirstJoinedAt),
			LastActiveAt:  formatTime(p.LastActiveAt),
		})
	}
	return sessionResponse{
		ID:                s.SessionID,
		UserID:            s.SubjectID,
		UserName:          userName,
		LoginFlowID:       s.FlowID,
		AuthenticatedAt:   formatTime(s.AuthenticatedAt),
		CreatedAt:         formatTime(s.CreatedAt),
		LastActiveAt:      formatTime(s.LastActiveAt),
		IdleExpiresAt:     formatTime(s.IdleExpiresAt),
		AbsoluteExpiresAt: formatTime(s.AbsoluteExpiresAt),
		Participants:      participants,
	}
}
