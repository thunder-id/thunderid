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

import {describe, it, expect} from 'vitest';
import {isValidPropertyName} from '../isValidPropertyName';

describe('isValidPropertyName', () => {
  it('accepts letters, digits, and underscores', () => {
    expect(isValidPropertyName('email')).toBe(true);
    expect(isValidPropertyName('family_name')).toBe(true);
    expect(isValidPropertyName('address2')).toBe(true);
    expect(isValidPropertyName('_internal')).toBe(true);
  });

  it('rejects names containing hyphens', () => {
    expect(isValidPropertyName('silver-mail')).toBe(false);
    expect(isValidPropertyName('postal-code')).toBe(false);
  });

  it('rejects names with other invalid characters', () => {
    expect(isValidPropertyName('first name')).toBe(false);
    expect(isValidPropertyName('email.address')).toBe(false);
    expect(isValidPropertyName('name!')).toBe(false);
  });

  it('rejects an empty name', () => {
    expect(isValidPropertyName('')).toBe(false);
  });
});
