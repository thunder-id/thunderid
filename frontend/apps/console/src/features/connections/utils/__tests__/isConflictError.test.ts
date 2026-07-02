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

import {describe, expect, it} from 'vitest';
import isConflictError from '../isConflictError';

describe('isConflictError', () => {
  it('returns true for a 409 response', () => {
    expect(isConflictError({response: {status: 409}})).toBe(true);
  });

  it('returns false for other statuses', () => {
    expect(isConflictError({response: {status: 400}})).toBe(false);
    expect(isConflictError({response: {status: 500}})).toBe(false);
  });

  it('returns false for non-HTTP errors', () => {
    expect(isConflictError(new Error('boom'))).toBe(false);
    expect(isConflictError(null)).toBe(false);
    expect(isConflictError(undefined)).toBe(false);
  });
});
