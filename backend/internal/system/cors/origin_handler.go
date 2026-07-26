// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package cors

import "encoding/json"

// OriginConfig is the cors server-config section value.
type OriginConfig struct {
	AllowedOrigins OriginEntries `json:"allowedOrigins" yaml:"allowedOrigins"`
}

// OriginHandler decodes, validates, and merges CORS origin config.
type OriginHandler struct{}

// Decode parses a raw JSON cors value into OriginConfig.
func (OriginHandler) Decode(raw json.RawMessage) (any, error) {
	if len(raw) == 0 {
		return OriginConfig{AllowedOrigins: OriginEntries{}}, nil
	}
	var cfg OriginConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	if cfg.AllowedOrigins == nil {
		cfg.AllowedOrigins = OriginEntries{}
	}
	return cfg, nil
}

// Validate checks that incoming carries a valid set of origin entries.
func (OriginHandler) Validate(incoming, _, _ any) error {
	cfg, _ := incoming.(OriginConfig)
	return Validate(cfg.AllowedOrigins)
}

// ComposeWritable builds the value to store in the writable layer. Origins the read-only (declarative)
// layer already allows are left out: they need no writable entry, and keeping one would leave the origin
// allowed even after the declarative layer stops declaring it. A non-nil existing (the layer's current
// value, offered on a merging import) is kept and the incoming origins are added to it, so an import
// cannot revoke origins another import registered.
func (h OriginHandler) ComposeWritable(readOnly, existing, incoming any) any {
	declared := make(map[string]struct{})
	ro, _ := readOnly.(OriginConfig)
	for _, e := range ro.AllowedOrigins {
		declared[entryKey(e)] = struct{}{}
	}

	merged, _ := h.Merge(existing, incoming).(OriginConfig)
	out := make(OriginEntries, 0, len(merged.AllowedOrigins))
	for _, e := range merged.AllowedOrigins {
		if _, dup := declared[entryKey(e)]; dup {
			continue
		}
		out = append(out, e)
	}
	return OriginConfig{AllowedOrigins: out}
}

// Merge combines read-only and writable origins, de-duplicated with read-only entries first.
func (OriginHandler) Merge(readOnly, writable any) any {
	seen := make(map[string]struct{})
	out := make(OriginEntries, 0)
	for _, layer := range []any{readOnly, writable} {
		cfg, _ := layer.(OriginConfig)
		for _, e := range cfg.AllowedOrigins {
			key := entryKey(e)
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, e)
		}
	}
	return OriginConfig{AllowedOrigins: out}
}
