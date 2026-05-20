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

import {User} from '@thunderid/browser';
import getMappedUserProfileValue from './getMappedUserProfileValue';

/**
 * Get the display name of a user by mapping their profile attributes.
 *
 * @param mergedMappings - The merged attribute mappings.
 * @param user - The user object containing profile information.
 * @param displayAttributes - Optional array of attribute keys or paths to try first.
 *   Each entry is resolved via `getMappedUserProfileValue`. The first non-empty
 *   value found is returned. If none resolve, the default fallback chain is used.
 *
 * @example
 * ```ts
 * // Default behavior — tries firstName+lastName, then username, email, name
 * const displayName = getDisplayName(mergedMappings, user);
 *
 * // Custom attributes — try 'nickname' first, then fall back to defaults
 * const displayName = getDisplayName(mergedMappings, user, ['nickname']);
 *
 * // Multiple custom attributes
 * const displayName = getDisplayName(mergedMappings, user, ['preferred_username', 'nickname']);
 * ```
 *
 * @returns The display name of the user.
 */
const getDisplayName = (
  mergedMappings: Record<string, string | string[] | undefined>,
  user: User,
  displayAttributes?: string[],
): string => {
  if (displayAttributes && displayAttributes.length > 0) {
    let foundValue: string | undefined;
    displayAttributes.some((attr: string) => {
      const value: any = getMappedUserProfileValue(attr, mergedMappings, user);

      if (value !== undefined && value !== null && value !== '') {
        foundValue = String(value);
        return true;
      }
      return false;
    });
    if (foundValue !== undefined) {
      return foundValue;
    }
  }

  const firstName: any = getMappedUserProfileValue('firstName', mergedMappings, user);
  const lastName: any = getMappedUserProfileValue('lastName', mergedMappings, user);

  if (firstName && lastName) {
    return `${firstName} ${lastName}`;
  }

  return (
    getMappedUserProfileValue('username', mergedMappings, user) ||
    getMappedUserProfileValue('email', mergedMappings, user) ||
    getMappedUserProfileValue('name', mergedMappings, user) ||
    'User'
  );
};

export default getDisplayName;
