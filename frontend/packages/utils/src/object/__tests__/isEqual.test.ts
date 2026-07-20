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
import isEqual from '../isEqual';

describe('isEqual', () => {
  describe('primitives', () => {
    it('should return true for identical strings', () => {
      expect(isEqual('a', 'a')).toBe(true);
    });

    it('should return false for different strings', () => {
      expect(isEqual('a', 'b')).toBe(false);
    });

    it('should return true for identical numbers', () => {
      expect(isEqual(1, 1)).toBe(true);
    });

    it('should return true for NaN compared to NaN', () => {
      expect(isEqual(NaN, NaN)).toBe(true);
    });

    it('should return false for different numbers', () => {
      expect(isEqual(1, 2)).toBe(false);
    });

    it('should return true for null compared to null', () => {
      expect(isEqual(null, null)).toBe(true);
    });

    it('should return true for undefined compared to undefined', () => {
      expect(isEqual(undefined, undefined)).toBe(true);
    });

    it('should return false for null compared to undefined', () => {
      expect(isEqual(null, undefined)).toBe(false);
    });
  });

  describe('arrays', () => {
    it('should return true for arrays with the same values in the same order', () => {
      expect(isEqual([1, 2, 3], [1, 2, 3])).toBe(true);
    });

    it('should return false for arrays with the same values in a different order', () => {
      expect(isEqual([1, 2], [2, 1])).toBe(false);
    });

    it('should return false for arrays with different lengths', () => {
      expect(isEqual([1], [1, 2])).toBe(false);
    });

    it('should return false when comparing an array to a plain object', () => {
      expect(isEqual([1, 2], {0: 1, 1: 2})).toBe(false);
    });

    it('should deeply compare nested arrays', () => {
      expect(isEqual([[1, 2], [3]], [[1, 2], [3]])).toBe(true);
    });
  });

  describe('plain objects', () => {
    it('should return true for objects with the same keys and values regardless of order', () => {
      expect(isEqual({a: 1, b: 2}, {b: 2, a: 1})).toBe(true);
    });

    it('should return false when a value differs', () => {
      expect(isEqual({a: 1}, {a: 2})).toBe(false);
    });

    it('should return false when key sets differ', () => {
      expect(isEqual({a: 1}, {a: 1, b: 2})).toBe(false);
    });

    it('should deeply compare nested objects', () => {
      expect(isEqual({a: {b: {c: 1}}}, {a: {b: {c: 1}}})).toBe(true);
    });

    it('should return false when comparing a plain object to a class instance', () => {
      class Foo {
        a = 1;
      }
      expect(isEqual({a: 1}, new Foo())).toBe(false);
    });
  });
});
