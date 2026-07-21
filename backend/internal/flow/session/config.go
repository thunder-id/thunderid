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

package session

import (
	"encoding/json"
	"fmt"
)

// Config is the value of the server-config "session" section: the SSO session lifetime
// configuration. Durations are in seconds; a zero or omitted value falls back to the built-in
// default (see NewTimeouts).
type Config struct {
	IdleTimeoutSeconds     int64 `json:"idleTimeoutSeconds"     yaml:"idleTimeoutSeconds"`
	AbsoluteTimeoutSeconds int64 `json:"absoluteTimeoutSeconds" yaml:"absoluteTimeoutSeconds"`
}

// Validate ensures the configured session timeouts are coherent. Unset (zero) values are allowed
// and fall back to defaults, so only set values are checked.
func (c Config) Validate() error {
	if c.IdleTimeoutSeconds < 0 {
		return fmt.Errorf("session.idleTimeoutSeconds must be greater than or equal to 0")
	}
	if c.AbsoluteTimeoutSeconds < 0 {
		return fmt.Errorf("session.absoluteTimeoutSeconds must be greater than or equal to 0")
	}
	if c.IdleTimeoutSeconds > 0 && c.AbsoluteTimeoutSeconds > 0 &&
		c.IdleTimeoutSeconds > c.AbsoluteTimeoutSeconds {
		return fmt.Errorf("session.idleTimeoutSeconds must not exceed absoluteTimeoutSeconds")
	}
	return nil
}

// ConfigHandler decodes, validates, and merges the "session" server-config section. It implements
// the serverconfig section-handler contract structurally so the section can be registered at the
// composition root without this package depending on the serverconfig package.
type ConfigHandler struct{}

// Decode parses a raw JSON session value into Config. Empty input yields the zero Config, which
// resolves to the built-in default timeouts.
func (ConfigHandler) Decode(raw json.RawMessage) (any, error) {
	if len(raw) == 0 {
		return Config{}, nil
	}
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate checks that the incoming value is a coherent session config.
func (ConfigHandler) Validate(incoming, _, _ any) error {
	cfg, _ := incoming.(Config)
	return cfg.Validate()
}

// Merge overlays the writable (db) layer onto the read-only (declarative) layer: a positive writable
// value wins for its field, otherwise the read-only value stands.
func (ConfigHandler) Merge(readOnly, writable any) any {
	merged, _ := readOnly.(Config)
	wr, _ := writable.(Config)
	if wr.IdleTimeoutSeconds > 0 {
		merged.IdleTimeoutSeconds = wr.IdleTimeoutSeconds
	}
	if wr.AbsoluteTimeoutSeconds > 0 {
		merged.AbsoluteTimeoutSeconds = wr.AbsoluteTimeoutSeconds
	}
	return merged
}
