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
import isPlainObject from '../isPlainObject';

describe('isPlainObject', () => {
  it('should return true for an object literal', () => {
    expect(isPlainObject({a: 1})).toBe(true);
  });

  it('should return true for an empty object literal', () => {
    expect(isPlainObject({})).toBe(true);
  });

  it('should return true for an object created with Object.create(null)', () => {
    expect(isPlainObject(Object.create(null))).toBe(true);
  });

  it('should return true for an object created with new Object()', () => {
    expect(isPlainObject(new Object())).toBe(true);
  });

  it('should return false for an array', () => {
    expect(isPlainObject([1, 2])).toBe(false);
  });

  it('should return false for null', () => {
    expect(isPlainObject(null)).toBe(false);
  });

  it('should return false for undefined', () => {
    expect(isPlainObject(undefined)).toBe(false);
  });

  it('should return false for a string', () => {
    expect(isPlainObject('hello')).toBe(false);
  });

  it('should return false for a number', () => {
    expect(isPlainObject(42)).toBe(false);
  });

  it('should return false for a function', () => {
    expect(isPlainObject(() => null)).toBe(false);
  });

  it('should return false for a Date', () => {
    expect(isPlainObject(new Date())).toBe(false);
  });

  it('should return false for a class instance', () => {
    class Foo {}
    expect(isPlainObject(new Foo())).toBe(false);
  });
});
