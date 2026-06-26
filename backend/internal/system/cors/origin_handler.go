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

package cors

import "encoding/json"

// OriginConfig is the cors server-config section value. It is an object (not a bare list) so future CORS
// settings can be added without a breaking change.
type OriginConfig struct {
	AllowedOrigins OriginEntries `json:"allowedOrigins" yaml:"allowedOrigins"`
}

// OriginHandler decodes, validates, and merges CORS origin config. It satisfies a consumer's
// value-handler interface structurally, so the cors package stays decoupled from the configuration store.
type OriginHandler struct{}

// Decode parses a raw JSON cors value into OriginConfig. Empty input or a missing allowedOrigins key yields
// an empty (non-nil) list; an explicit allowedOrigins:null is rejected.
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

// Validate checks that incoming carries a valid set of origin entries. The readOnly and writable layers are
// unused for CORS — an origin list is valid on its own.
func (OriginHandler) Validate(incoming, _, _ any) error {
	cfg, _ := incoming.(OriginConfig)
	return Validate(cfg.AllowedOrigins)
}

// Merge returns an OriginConfig whose AllowedOrigins is the union of the readOnly and writable origin lists,
// de-duplicated, preserving order (readOnly entries first). Empty or absent layers contribute nothing.
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
