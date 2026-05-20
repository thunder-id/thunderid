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

import {describe, it, expect, vi, afterEach} from 'vitest';
import formatDate from '../formatDate';

describe('formatDate', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('returns a formatted date for a valid date string', () => {
    const dateIso = '2025-07-09T12:00:00Z';
    const dateRfc = 'Wed, 09 Jul 2025 12:00:00 GMT';
    expect(formatDate(dateIso)).toBe('July 9, 2025');
    expect(formatDate(dateRfc)).toBe('July 9, 2025');
  });

  it('returns "-" when given undefined or empty', () => {
    expect(formatDate(undefined)).toBe('-');
    expect(formatDate('')).toBe('-');
  });

  it('returns the "Invalid Date" when the date is invalid', () => {
    const invalid = 'invalid-date';
    expect(formatDate(invalid)).toBe('Invalid Date');
  });

  it('returns the original string when parsing/formatting throws', () => {
    const spy: ReturnType<typeof vi.spyOn> = vi.spyOn(Date.prototype, 'toLocaleDateString').mockImplementation(() => {
      throw new RangeError('Forced failure');
    });

    const input = '2025-07-09T12:00:00Z';
    expect(formatDate(input)).toBe(input);

    spy.mockRestore();
  });
});
