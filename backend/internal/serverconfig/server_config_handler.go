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
	//
	// The context carries the deployment the value is being set for. A section validating against
	// stored resources has to read them under that deployment: on a multi-tenant server the same
	// resource id means a different row in each tenant, and looking one up without the deployment
	// finds nothing.
	Validate(ctx context.Context, incoming, readOnly, writable any) error
	// Merge composes the readOnly and writable layers into the effective value.
	Merge(readOnly, writable any) any
}
