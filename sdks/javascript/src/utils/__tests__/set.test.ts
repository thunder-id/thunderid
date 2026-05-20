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
import set from '../set';

describe('set', () => {
  it('should set a simple top-level property', () => {
    const obj: any = {};
    set(obj, 'a', 1);
    expect(obj).toEqual({a: 1});
  });

  it('should create nested objects for a dotted path', () => {
    const obj: any = {};
    set(obj, 'a.b.c', 42);
    expect(obj).toEqual({a: {b: {c: 42}}});
  });

  it('should create arrays when the next key is numeric', () => {
    const obj: any = {};
    set(obj, 'a.0', 'x');
    set(obj, 'a.1', 'y');
    expect(obj).toEqual({a: ['x', 'y']});
  });

  it('should support path as an array', () => {
    const obj: any = {};
    set(obj, ['x', 'y', 'z'], true);
    expect(obj).toEqual({x: {y: {z: true}}});
  });

  it('should do nothing if object is falsy', () => {
    const obj: any = null;
    expect(set(obj, 'a.b', 1)).toBeNull();
  });

  it('should return the object unchanged if path is falsy', () => {
    const obj: any = {a: 1};
    const out: any = set(obj, '', 1);
    expect(out).toBe(obj);
    expect(obj).toEqual({a: 1});
  });

  it('should overwrite existing value at the final segment', () => {
    const obj: any = {a: {b: 1}};
    set(obj, 'a.b', 2);
    expect(obj).toEqual({a: {b: 2}});
  });

  it('should reuse existing objects when traversing', () => {
    const obj: any = {a: {b: {c: 1}}};
    const refB: any = obj.a.b;
    set(obj, 'a.b.d', 2);
    expect(obj.a.b).toBe(refB);
    expect(obj).toEqual({a: {b: {c: 1, d: 2}}});
  });

  it('should replace non-object intermediates when needed (number -> object/array as required)', () => {
    const obj: any = {a: 123};
    set(obj, 'a.b.c', 7);
    expect(obj).toEqual({a: {b: {c: 7}}});

    const obj2: any = {a: 123};
    set(obj2, 'a.0', 'x');
    expect(obj2).toEqual({a: ['x']});
  });

  it('should replace null intermediates when needed', () => {
    const obj: any = {a: null};
    set(obj, 'a.b.c', 5);
    expect(obj).toEqual({a: {b: {c: 5}}});
  });

  it('should replace intermediate types as path demands (string -> array for numeric next, string -> object for non-numeric next)', () => {
    const obj: any = {a: {b: 'string'}};
    set(obj, 'a.b.0', 'x');
    expect(obj).toEqual({a: {b: ['x']}});

    const obj2: any = {a: 'string'};
    set(obj2, 'a.b.c', 9);
    expect(obj2).toEqual({a: {b: {c: 9}}});
  });

  it('should handle setting inside an existing array index', () => {
    const obj: any = {a: [{}, {}]};
    set(obj, 'a.1.x', 10);
    expect(obj).toEqual({a: [{}, {x: 10}]});
  });

  it('should create sparse arrays if setting a far index', () => {
    const obj: any = {};
    set(obj, 'a.3', 'z');
    expect(obj.a.length).toBe(4);
    expect(obj.a[3]).toBe('z');
  });
});
