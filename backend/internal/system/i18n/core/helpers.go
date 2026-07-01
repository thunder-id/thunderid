/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

// Package core provides internationalization support.
package core

// GetDefault returns the default value for a given i18n key.
// Returns the value and true if found, empty string and false otherwise.
func GetDefault(key string) (string, bool) {
	val, ok := defaultMessages[key]
	return val, ok
}

// GetAllDefaults returns a copy of all default messages.
func GetAllDefaults() map[string]string {
	result := make(map[string]string, len(defaultMessages))
	for k, v := range defaultMessages {
		result[k] = v
	}
	return result
}

// GetAllKeys returns all registered i18n keys.
func GetAllKeys() []string {
	keys := make([]string, 0, len(defaultMessages))
	for k := range defaultMessages {
		keys = append(keys, k)
	}
	return keys
}
