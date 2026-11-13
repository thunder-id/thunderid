// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package serverconfig

import (
	"context"
	"encoding/json"
)

// ServerConfigHandlerInterface decodes, validates, and merges the value of one server-config section.
// Each supported section registers an implementation at service construction, so the section-specific
// rules live in the consuming package rather than in this generic layer. Decoded values (not raw bytes)
// flow through the service; (de)serialization is confined to the store, HTTP, and YAML edges.
type ServerConfigHandlerInterface interface {
	// Decode parses a raw stored or incoming value into the section's typed value. Empty input yields the
	// section's representation of an absent layer — either nil or an empty typed value (e.g. an empty list).
	// It is the single structural gate; later steps receive typed values.
	Decode(raw json.RawMessage) (any, error)
	// Validate reports whether incoming is a valid value for the section. The current readOnly
	// (declarative) and writable (db) layers are provided so a section can validate against existing
	// state; both are nil when validating a declarative resource at load time.
	Validate(ctx context.Context, incoming, readOnly, writable any) error
	// Merge composes the readOnly and writable layers into the section's configured value, which the
	// service lays over Default.
	Merge(readOnly, writable any) any
	// Default returns the value a field takes when the configured value leaves it unset, or nil when the
	// section has none. A field is laid over the default field by field, and an unset field keeps it.
	Default() any
	// IsLocked reports whether readOnly, the declarative layer, makes the section read-only. A locked
	// section refuses every write.
	IsLocked(readOnly any) bool
}
