// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sessionmgt

import (
	"time"

	flowsession "github.com/thunder-id/thunderid/internal/flow/session"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// participantResponse is an application that has joined a session. AppName is omitted when it can't
// be resolved.
type participantResponse struct {
	AppID         string `json:"appId"`
	AppName       string `json:"appName,omitempty"`
	FirstJoinedAt string `json:"firstJoinedAt"`
	LastActiveAt  string `json:"lastActiveAt"`
}

// sessionResponse is the API view of a live session. It never carries the session handle, the
// checkpoint contexts or the token family ids. UserName is omitted when it can't be resolved, and
// IsCurrent is set only on the caller's own sessions.
type sessionResponse struct {
	ID                string                `json:"id"`
	UserID            string                `json:"userId"`
	UserName          string                `json:"userName,omitempty"`
	UserAgent         string                `json:"userAgent,omitempty"`
	CreationIP        string                `json:"creationIp,omitempty"`
	LastActiveIP      string                `json:"lastActiveIp,omitempty"`
	AuthenticatedAt   string                `json:"authenticatedAt"`
	CreatedAt         string                `json:"createdAt"`
	LastActiveAt      string                `json:"lastActiveAt"`
	IdleExpiresAt     string                `json:"idleExpiresAt,omitempty"`
	AbsoluteExpiresAt string                `json:"absoluteExpiresAt,omitempty"`
	IsCurrent         *bool                 `json:"isCurrent,omitempty"`
	Participants      []participantResponse `json:"participants"`
}

// sessionListResponse is the paginated body of GET /sessions and GET /users/me/sessions.
type sessionListResponse struct {
	TotalResults int               `json:"totalResults"`
	StartIndex   int               `json:"startIndex"`
	Count        int               `json:"count"`
	Sessions     []sessionResponse `json:"sessions"`
	Links        []sysutils.Link   `json:"links"`
}

// formatTime renders a timestamp as RFC 3339 in UTC, or "" for the zero value (no deadline).
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// toSessionResponse maps a session and its participants to the API view, filling in the resolved
// names. A name missing from userNames or appNames is left out.
func toSessionResponse(s flowsession.SessionDetail, userNames, appNames map[string]string) sessionResponse {
	participants := make([]participantResponse, 0, len(s.Participants))
	for _, p := range s.Participants {
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
		UserName:          userNames[s.SubjectID],
		UserAgent:         s.Properties.UserAgent,
		CreationIP:        s.Properties.CreationIP,
		LastActiveIP:      s.Properties.LastActiveIP,
		AuthenticatedAt:   formatTime(s.AuthenticatedAt),
		CreatedAt:         formatTime(s.CreatedAt),
		LastActiveAt:      formatTime(s.LastActiveAt),
		IdleExpiresAt:     formatTime(s.IdleExpiresAt),
		AbsoluteExpiresAt: formatTime(s.AbsoluteExpiresAt),
		Participants:      participants,
	}
}
