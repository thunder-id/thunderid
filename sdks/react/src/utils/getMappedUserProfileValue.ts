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

import {User, get} from '@thunderid/browser';

/**
 * Retrieves a user profile value based on attribute mapping configuration.
 *
 * This function allows flexible mapping of component attribute names to actual
 * user profile field paths. It supports both simple string mappings and arrays
 * of potential field paths for fallback scenarios.
 *
 * @param key - The logical attribute name to retrieve (e.g., 'firstName', 'email')
 * @param mappings - Object mapping logical names to user profile field paths
 * @param user - The user object to extract values from
 * @returns The mapped value from the user profile, or undefined if not found
 *
 * @example
 * ```typescript
 * const mappings = {
 *   firstName: 'name.givenName',
 *   email: 'emails[0]',
 *   picture: ['profileUrl', 'profile', 'avatar'] // fallback options
 * };
 *
 * const user = {
 *   name: { givenName: 'John' },
 *   emails: ['john@example.com'],
 *   profileUrl: 'https://example.com/avatar.jpg'
 * };
 *
 * getMappedUserProfileValue('firstName', mappings, user); // 'John'
 * getMappedUserProfileValue('email', mappings, user); // 'john@example.com'
 * getMappedUserProfileValue('picture', mappings, user); // 'https://example.com/avatar.jpg'
 * ```
 */
const getMappedUserProfileValue = (key: string, mappings: Record<string, string | string[]>, user: User): any => {
  if (!key || !mappings || !user) {
    return undefined;
  }

  const mapping: string | string[] = mappings[key];

  if (!mapping) {
    // If no mapping defined, try to get the value directly from the user object
    return get(user, key);
  }

  // If mapping is an array, try each path until we find a value
  if (Array.isArray(mapping)) {
    let foundValue: any;
    let found = false;
    mapping.some((path: string) => {
      const value: any = get(user, path);
      if (value !== undefined && value !== null && value !== '') {
        foundValue = value;
        found = true;
        return true;
      }
      return false;
    });
    return found ? foundValue : undefined;
  }

  // For single string mapping, get the value directly
  return get(user, mapping);
};

export default getMappedUserProfileValue;
