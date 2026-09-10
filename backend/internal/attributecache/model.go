// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package attributecache

// AttributeCache represents a cached attribute entry.
type AttributeCache struct {
	// ID is the unique identifier for the cache entry.
	ID string `json:"id"`

	// Attributes contains the cached attributes.
	Attributes map[string]interface{} `json:"attributes"`

	// TTLSeconds is the time-to-live in seconds for this cache entry.
	TTLSeconds int64 `json:"ttlSeconds"`

	// SubjectID is the resource ID of the entity the cached attributes belong to, and
	// SubjectCategory its entity category. They are recorded when the entry is created, where the
	// authentication has just resolved the entity, so a later grant that holds only this entry's ID
	// can report the subject without resolving it again. This matters for a token whose sub claim is
	// mapped to an attribute such as an email address: the resource ID is not recoverable from the
	// token, and this entry is the only server-side record of it. Observability-only: no
	// authorization decision reads either field, and both may be empty.
	SubjectID       string `json:"subjectId,omitempty"`
	SubjectCategory string `json:"subjectCategory,omitempty"`
}
