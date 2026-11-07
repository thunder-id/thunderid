// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package stdoutsink

import (
	"maps"
	"slices"
	"unicode/utf8"

	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
)

// newLine maps an audit event to its line.
func newLine(env eventbus.Envelope, payload audit.Payload) line {
	actor := payload.Actor()
	target := payload.Target()
	outcome := payload.Outcome()
	l := line{
		LogType:       logTypeAudit,
		SchemaVersion: schemaVersion,
		ID:            env.ID,
		Time:          env.Time.UTC().Format(timeLayout),
		Event:         env.Name,
		TraceID:       env.TraceID,
		Actor:         refLine{Type: string(actor.Type), ID: actor.ID},
		OnBehalfOf:    optionalRef(payload.OnBehalfOf()),
		Subject:       optionalRef(payload.Subject()),
		Target:        refLine{Type: target.Type, ID: target.ID},
		Action:        string(payload.Action()),
		Outcome:       outcomeLine{Result: string(outcome.Result), Reason: outcome.Reason},
		Details:       payload.Details(),
	}
	if client := payload.Client(); client != (audit.Client{}) {
		l.Client = &clientLine{Address: client.Address, UserAgent: client.UserAgent}
	}
	return l
}

// optionalRef returns the line form of ref, nil for the zero Ref.
func optionalRef(ref audit.Ref) *refLine {
	if ref == (audit.Ref{}) {
		return nil
	}
	return &refLine{Type: ref.Type, ID: ref.ID}
}

// largestDetail returns the key of the largest detail, the first in key order on a tie.
func largestDetail(details map[string]string) string {
	largest, size := "", -1
	for _, key := range slices.Sorted(maps.Keys(details)) {
		if entry := len(key) + len(details[key]); entry > size {
			largest, size = key, entry
		}
	}
	return largest
}

// capFields cuts every free-form field of l to maxFieldBytes.
func capFields(l *line) {
	fields := []*string{
		&l.TraceID, &l.Actor.Type, &l.Actor.ID, &l.Target.Type, &l.Target.ID, &l.Action, &l.Outcome.Reason,
	}
	for _, ref := range []*refLine{l.OnBehalfOf, l.Subject} {
		if ref != nil {
			fields = append(fields, &ref.Type, &ref.ID)
		}
	}
	if l.Client != nil {
		fields = append(fields, &l.Client.Address, &l.Client.UserAgent)
	}
	for _, field := range fields {
		*field = truncateBytes(*field, maxFieldBytes)
	}
}

// truncateBytes returns the longest prefix of value within limit bytes that ends on a rune boundary.
func truncateBytes(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	limit = max(limit, 0)
	for limit > 0 && !utf8.RuneStart(value[limit]) {
		limit--
	}
	return value[:limit]
}
