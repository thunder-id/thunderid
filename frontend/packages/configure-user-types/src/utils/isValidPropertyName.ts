/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

const PROPERTY_NAME_PATTERN = /^[a-zA-Z0-9_]+$/;

/**
 * Checks whether a schema property name is valid.
 *
 * Property names are used directly as database filter keys for value and
 * uniqueness lookups, so the backend only accepts letters, digits, and
 * underscores. This mirrors that rule so the console can surface the error
 * as the user types instead of failing on submission.
 */
export function isValidPropertyName(name: string): boolean {
  return PROPERTY_NAME_PATTERN.test(name);
}
