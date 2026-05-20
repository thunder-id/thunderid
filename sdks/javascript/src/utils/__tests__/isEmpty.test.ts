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

import isEmpty from '../isEmpty';

describe('isEmpty', () => {
  it('should return true for null', () => {
    expect(isEmpty(null)).toBe(true);
  });

  it('should return true for undefined', () => {
    expect(isEmpty(undefined)).toBe(true);
  });

  it('should return true for empty string', () => {
    expect(isEmpty('')).toBe(true);
  });

  it('should return true for whitespace-only string', () => {
    expect(isEmpty('   ')).toBe(true);
    expect(isEmpty('\t')).toBe(true);
    expect(isEmpty('\n')).toBe(true);
    expect(isEmpty(' \t\n ')).toBe(true);
  });

  it('should return false for non-empty string', () => {
    expect(isEmpty('hello')).toBe(false);
    expect(isEmpty(' hello ')).toBe(false);
    expect(isEmpty('0')).toBe(false);
  });

  it('should return true for empty array', () => {
    expect(isEmpty([])).toBe(true);
  });

  it('should return false for non-empty array', () => {
    expect(isEmpty([1, 2, 3])).toBe(false);
    expect(isEmpty([''])).toBe(false);
    expect(isEmpty([null])).toBe(false);
  });

  it('should return true for empty object', () => {
    expect(isEmpty({})).toBe(true);
  });

  it('should return false for non-empty object', () => {
    expect(isEmpty({name: 'John'})).toBe(false);
    expect(isEmpty({'': ''})).toBe(false);
    expect(isEmpty({a: undefined})).toBe(false);
  });

  it('should return false for numbers', () => {
    expect(isEmpty(0)).toBe(false);
    expect(isEmpty(1)).toBe(false);
    expect(isEmpty(-1)).toBe(false);
    expect(isEmpty(3.14)).toBe(false);
    expect(isEmpty(NaN)).toBe(false);
    expect(isEmpty(Infinity)).toBe(false);
  });

  it('should return false for booleans', () => {
    expect(isEmpty(true)).toBe(false);
    expect(isEmpty(false)).toBe(false);
  });

  it('should return false for functions', () => {
    expect(isEmpty(() => {})).toBe(false);
    expect(isEmpty(() => {})).toBe(false);
  });

  it('should return false for dates', () => {
    expect(isEmpty(new Date())).toBe(false);
  });

  it('should return false for other object types', () => {
    expect(isEmpty(new Set())).toBe(false);
    expect(isEmpty(new Map())).toBe(false);
    expect(isEmpty(/regex/)).toBe(false);
  });
});
