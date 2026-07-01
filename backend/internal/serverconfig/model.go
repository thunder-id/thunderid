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
// layer, the writable (configdb) layer, and their merged effective value. It is the response body for
// GET /server-config/{name}; each field marshals to JSON via the section's value type.
type ServerConfigLayers struct {
	ReadOnly any `json:"readOnly"`
	Writable any `json:"writable"`
	Merged   any `json:"merged"`
}
