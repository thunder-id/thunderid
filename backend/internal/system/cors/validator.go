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

// OriginValidator validates raw JSON origin config by parsing it as OriginEntries and running
// Validate. It satisfies a consumer's value-validator interface structurally, so the cors package
// stays decoupled from the configuration store.
type OriginValidator struct{}

// Validate parses value as JSON origin entries and validates them.
func (OriginValidator) Validate(value json.RawMessage) error {
	var entries OriginEntries
	if err := json.Unmarshal(value, &entries); err != nil {
		return err
	}
	return Validate(entries)
}
