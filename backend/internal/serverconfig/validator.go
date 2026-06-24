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

// ServerConfigValidatorInterface validates the raw JSON value of a config section. Each supported
// config registers an implementation with the service, so the validation rules live in the
// consuming package rather than in this generic layer.
type ServerConfigValidatorInterface interface {
	// Validate returns nil if the value is valid for the config section, or a non-nil error
	// describing why it is invalid.
	Validate(value json.RawMessage) error
}
