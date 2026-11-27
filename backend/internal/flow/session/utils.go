// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import sysutils "github.com/thunder-id/thunderid/internal/system/utils"

// toSessionResponse maps a session and its participants to the API view, filling in the resolved names and
// logos. A value missing from displays or apps is left out.
func toSessionResponse(sess sessionDetail, displays map[string]string, apps map[string]appDisplay) sessionResponse {
	participants := make([]participantResponse, 0, len(sess.Participants))
	for _, p := range sess.Participants {
		participants = append(participants, participantResponse{
			AppID:         p.AppID,
			OUID:          p.OUID,
			AppName:       apps[p.AppID].name,
			AppLogoURL:    apps[p.AppID].logoURL,
			FirstJoinedAt: sysutils.FormatRFC3339(p.FirstJoinedAt),
			LastActiveAt:  sysutils.FormatRFC3339(p.LastActiveAt),
		})
	}
	return sessionResponse{
		ID:                sess.SessionID,
		Type:              sess.Type,
		UserID:            sess.SubjectID,
		UserDisplayName:   displays[sess.SubjectID],
		UserAgent:         sess.Properties.UserAgent,
		CreationIP:        sess.Properties.CreationIP,
		LastActiveIP:      sess.Properties.LastActiveIP,
		AuthenticatedAt:   sysutils.FormatRFC3339(sess.AuthenticatedAt),
		CreatedAt:         sysutils.FormatRFC3339(sess.CreatedAt),
		LastActiveAt:      sysutils.FormatRFC3339(sess.LastActiveAt),
		IdleExpiresAt:     sysutils.FormatRFC3339(sess.IdleExpiresAt),
		AbsoluteExpiresAt: sysutils.FormatRFC3339(sess.AbsoluteExpiresAt),
		Participants:      participants,
	}
}
