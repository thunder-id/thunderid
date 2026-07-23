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

import {describe, expect, it} from 'vitest';
import isDuplicateAppNameError from '../isDuplicateAppNameError';

describe('isDuplicateAppNameError', () => {
  it('returns true for an APP-1020 error response', () => {
    expect(isDuplicateAppNameError({response: {data: {code: 'APP-1020'}}})).toBe(true);
  });

  it('returns false for other error codes', () => {
    expect(isDuplicateAppNameError({response: {data: {code: 'APP-1001'}}})).toBe(false);
    expect(isDuplicateAppNameError({response: {status: 400, data: {}}})).toBe(false);
  });

  it('returns false for non-HTTP errors', () => {
    expect(isDuplicateAppNameError(new Error('boom'))).toBe(false);
    expect(isDuplicateAppNameError(null)).toBe(false);
    expect(isDuplicateAppNameError(undefined)).toBe(false);
  });
});
