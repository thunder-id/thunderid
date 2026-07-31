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
import isEqualIgnoringEmpty from '../isEqualIgnoringEmpty';

describe('isEqualIgnoringEmpty', () => {
  describe('empty-ish equivalence', () => {
    it('treats undefined and empty string as equal', () => {
      expect(isEqualIgnoringEmpty(undefined, '')).toBe(true);
    });

    it('treats null and undefined as equal', () => {
      expect(isEqualIgnoringEmpty(null, undefined)).toBe(true);
    });

    it('treats empty array and undefined as equal', () => {
      expect(isEqualIgnoringEmpty([], undefined)).toBe(true);
    });

    it('treats empty array and empty string as equal', () => {
      expect(isEqualIgnoringEmpty([], '')).toBe(true);
    });
  });

  describe('primitive comparisons', () => {
    it('returns true for identical strings', () => {
      expect(isEqualIgnoringEmpty('hello', 'hello')).toBe(true);
    });

    it('returns false for different, non-empty strings', () => {
      expect(isEqualIgnoringEmpty('hello', 'world')).toBe(false);
    });

    it('returns false when one side is empty and the other is not', () => {
      expect(isEqualIgnoringEmpty('', 'hello')).toBe(false);
    });

    it('returns true for identical numbers', () => {
      expect(isEqualIgnoringEmpty(3600, 3600)).toBe(true);
    });

    it('returns false for different numbers', () => {
      expect(isEqualIgnoringEmpty(3600, 7200)).toBe(false);
    });
  });

  describe('array comparisons', () => {
    it('returns true for arrays with the same values in the same order', () => {
      expect(isEqualIgnoringEmpty(['a', 'b'], ['a', 'b'])).toBe(true);
    });

    it('returns false for arrays with the same values in a different order', () => {
      expect(isEqualIgnoringEmpty(['a', 'b'], ['b', 'a'])).toBe(false);
    });

    it('returns false for arrays with different lengths', () => {
      expect(isEqualIgnoringEmpty(['a'], ['a', 'b'])).toBe(false);
    });
  });

  describe('nested object comparisons', () => {
    it('returns true for deeply equal objects regardless of key order', () => {
      expect(isEqualIgnoringEmpty({a: 1, b: {c: 2}}, {b: {c: 2}, a: 1})).toBe(true);
    });

    it('returns false when a nested value differs', () => {
      expect(isEqualIgnoringEmpty({a: 1, b: {c: 2}}, {a: 1, b: {c: 3}})).toBe(false);
    });

    it('returns true when reconstructing an object yields the same values', () => {
      const original = {redirectUris: ['https://example.com/callback'], grantTypes: ['authorization_code']};
      const reconstructed = {...original, redirectUris: [...original.redirectUris]};
      expect(isEqualIgnoringEmpty(reconstructed, original)).toBe(true);
    });
  });

  describe('nested empty-ish equivalence', () => {
    it('treats a nested empty string as equal to an absent key', () => {
      expect(isEqualIgnoringEmpty({email: 'a@b.com', bio: ''}, {email: 'a@b.com'})).toBe(true);
    });

    it('treats nested null and empty array as equal to an absent key', () => {
      expect(isEqualIgnoringEmpty({name: 'x', tags: [], parent: null}, {name: 'x'})).toBe(true);
    });

    it('treats an object of only-empty members as equal to an empty object', () => {
      expect(isEqualIgnoringEmpty({a: '', b: null, c: []}, {})).toBe(true);
    });

    it('ignores empty-ish elements nested inside arrays', () => {
      expect(isEqualIgnoringEmpty([{a: 1, b: ''}], [{a: 1}])).toBe(true);
    });

    it('still returns false when a real nested value differs from empty', () => {
      expect(isEqualIgnoringEmpty({email: 'a@b.com', bio: 'x'}, {email: 'a@b.com'})).toBe(false);
    });
  });
});
