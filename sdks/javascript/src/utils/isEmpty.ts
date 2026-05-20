/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

/**
 * Checks if a value is considered empty.
 *
 * A value is considered empty if it is:
 * - null
 * - undefined
 * - empty string ("")
 * - string containing only whitespace characters
 * - empty array ([])
 * - empty object ({})
 *
 * @param value - The value to check
 * @returns true if the value is empty, false otherwise
 *
 * @example
 * ```typescript
 * isEmpty(null);              // true
 * isEmpty(undefined);         // true
 * isEmpty("");                // true
 * isEmpty("   ");             // true
 * isEmpty("hello");           // false
 * isEmpty([]);                // true
 * isEmpty([1, 2, 3]);         // false
 * isEmpty({});                // true
 * isEmpty({ name: "John" });  // false
 * isEmpty(0);                 // false
 * isEmpty(false);             // false
 * ```
 */
const isEmpty = (value: any): boolean => {
  if (value === null || value === undefined) {
    return true;
  }

  if (typeof value === 'string') {
    return value.trim() === '';
  }

  if (Array.isArray(value)) {
    return value.length === 0;
  }

  if (typeof value === 'object' && value.constructor === Object) {
    return Object.keys(value).length === 0;
  }

  return false;
};

export default isEmpty;
