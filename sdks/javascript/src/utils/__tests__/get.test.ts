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

import {describe, it, expect} from 'vitest';
import get from '../get';

describe('get', () => {
  it('should return top-level property', () => {
    const o: Record<string, number> = {a: 1};
    expect(get(o, 'a')).toBe(1);
  });

  it('should return nested property via dotted path', () => {
    const o: Record<string, Record<string, Record<string, number>>> = {a: {b: {c: 5}}};
    expect(get(o, 'a.b.c')).toBe(5);
  });

  it('should return nested property via path array', () => {
    const o: Record<string, Record<string, Record<string, number>>> = {a: {b: {c: 5}}};
    expect(get(o, ['a', 'b', 'c'])).toBe(5);
  });

  it('should work with arrays using numeric indices in dotted path', () => {
    const o: Record<string, string[]> = {items: ['x', 'y', 'z']};
    expect(get(o, 'items.1')).toBe('y');
  });

  it('should work with arrays using numeric indices in path array', () => {
    const o: Record<string, string[]> = {items: ['x', 'y', 'z']};
    expect(get(o, ['items', '2'])).toBe('z');
  });

  it('should return defaultValue when path does not exist', () => {
    const o: Record<string, Record<string, unknown>> = {a: {}};
    expect(get(o, 'a.missing', 'def')).toBe('def');
  });

  it('should return undefined when path does not exist and no defaultValue is provided', () => {
    const o: Record<string, Record<string, unknown>> = {a: {}};
    expect(get(o, 'a.missing')).toBeUndefined();
  });

  it('should not use defaultValue for falsy but defined values: 0', () => {
    const o: Record<string, Record<string, number>> = {a: {n: 0}};
    expect(get(o, 'a.n', 42)).toBe(0);
  });

  it('should not use defaultValue for falsy but defined values: false', () => {
    const o: Record<string, Record<string, boolean>> = {a: {f: false}};
    expect(get(o, 'a.f', true)).toBe(false);
  });

  it('should not use defaultValue for falsy but defined values: empty string', () => {
    const o: Record<string, Record<string, string>> = {a: {s: ''}};
    expect(get(o, 'a.s', 'fallback')).toBe('');
  });

  it('should treat null as a defined value (does not return default)', () => {
    const o: Record<string, Record<string, null>> = {a: {v: null}};
    expect(get(o, 'a.v', 'def')).toBeNull();
  });

  it('should return defaultValue when object is null or undefined', () => {
    expect(get(null as any, 'a.b', 'def')).toBe('def');
    expect(get(undefined as any, 'a.b', 'def')).toBe('def');
  });

  it('should return defaultValue when path is empty/invalid', () => {
    const o: Record<string, number> = {a: 1};
    expect(get(o, '' as any, 'def')).toBe('def');
    expect(get(o, undefined as any, 'def')).toBe('def');
  });

  it('should stop safely when encountering a non-object in the chain', () => {
    const o: Record<string, number> = {a: 1};
    expect(get(o, 'a.b.c', 'def')).toBe('def');
  });

  it('should support keys that contain dots when using path array', () => {
    const o: Record<string, Record<string, number>> = {'a.b': {c: 7}};
    expect(get(o, ['a.b', 'c'])).toBe(7);
  });
});
