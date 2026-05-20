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

import get from './get';
import set from './set';
import {User} from '../models/user';

/**
 * Creates a profile structure from ME response based on processed schemas
 *
 * This function processes each schema attribute and populates the profile dynamically by:
 * - Extracting values from the ME response using the schema attribute names
 * - Handling multi-valued attributes by converting single values to arrays when needed
 * - Setting appropriate default values based on schema type and multiValued properties
 * - Using dynamic property setting to build the final profile object
 *
 * @param meResponse - The ME API response containing user data
 * @param processedSchemas - The processed and flattened schemas with name, type, and multiValued properties
 * @returns Flat profile object with dynamically populated user attributes
 *
 * @example
 * ```typescript
 * const meResponse = {
 *   userName: 'john.doe',
 *   emails: ['john@example.com', 'john.doe@work.com'],
 *   name: { givenName: 'John', familyName: 'Doe' }
 * };
 *
 * const schemas = [
 *   { name: 'userName', type: 'STRING', multiValued: false },
 *   { name: 'emails', type: 'STRING', multiValued: true },
 *   { name: 'name.givenName', type: 'STRING', multiValued: false }
 * ];
 *
 * const profile = generateUserProfile(meResponse, schemas);
 * // Result: {
 * //   userName: 'john.doe',
 * //   emails: ['john@example.com', 'john.doe@work.com'],
 * //   'name.givenName': 'John'
 * // }
 * ```
 */
const generateUserProfile = (meResponse: any, processedSchemas: any[]): User => {
  const profile: User = {};

  processedSchemas.forEach((schema: any) => {
    const {name, type, multiValued} = schema;

    if (!name) return;

    let value: any = get(meResponse, name);

    if (value !== undefined) {
      if (multiValued && !Array.isArray(value)) {
        value = [value];
      }
    } else if (multiValued) {
      value = undefined;
    } else if (type === 'STRING') {
      value = '';
    } else {
      value = undefined;
    }

    set(profile, name, value);
  });

  return profile;
};

export default generateUserProfile;
