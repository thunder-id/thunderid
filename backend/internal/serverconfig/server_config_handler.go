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
	Validate(incoming, readOnly, writable any) error
	// Merge composes the readOnly and writable layers into the effective value.
	Merge(readOnly, writable any) any
}
