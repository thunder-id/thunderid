// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package serverconfig

import "encoding/json"

// ServerConfig represents a single server-wide configuration section in the writable (db) layer. Its
// value is the raw JSON persisted to the config database.
type ServerConfig struct {
	Name  ConfigName
	Value json.RawMessage
}

// storeLayers is the raw (byte) form returned by the store: the read-only (declarative) and writable
// (db) layers. The service decodes these once and derives the merged value.
type storeLayers struct {
	ReadOnly json.RawMessage
	Writable json.RawMessage
}

// ServerConfigLayers is the per-section read result with decoded values: the read-only (declarative)
// layer, the writable (configdb) layer, the section's default, and the effective value that lays the
// first two over the default. It is the response body for GET /server-config/{name}; each field
// marshals to JSON via the section's value type.
type ServerConfigLayers struct {
	ReadOnly any `json:"readOnly"`
	Writable any `json:"writable"`
	Default  any `json:"default"`
	Merged   any `json:"merged"`
}
