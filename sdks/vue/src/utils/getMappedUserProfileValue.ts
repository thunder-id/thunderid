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

const getMappedUserProfileValue = (key: string, mappings: Record<string, string | string[]>, user: User): any => {
  if (!key || !mappings || !user) return undefined;

  const mapping: string | string[] = mappings[key];

  if (!mapping) return get(user, key);

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

  return get(user, mapping);
};

export default getMappedUserProfileValue;
