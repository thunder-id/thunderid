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
import ThunderIDRuntimeError from '../../errors/ThunderIDRuntimeError';
import resolveFieldName from '../resolveFieldName';

describe('resolveFieldName', () => {
  it('should return the field.param when provided', () => {
    const field: {param: string} = {param: 'username'};
    expect(resolveFieldName(field)).toBe('username');
  });

  it('should return the field.param as-is (without trimming)', () => {
    const field: {param: string} = {param: '  custom_param  '};
    expect(resolveFieldName(field)).toBe('  custom_param  ');
  });

  it('should throw ThunderIDRuntimeError when field.param is an empty string', () => {
    const field: {param: string} = {param: ''};
    expect(() => resolveFieldName(field)).toThrow(ThunderIDRuntimeError);
    expect(() => resolveFieldName(field)).toThrow('Field name is not supported');
  });

  it('should throw ThunderIDRuntimeError when field.param is missing', () => {
    const field: {somethingElse: string} = {somethingElse: 'value'} as any;
    expect(() => resolveFieldName(field)).toThrow(ThunderIDRuntimeError);
    expect(() => resolveFieldName(field)).toThrow('Field name is not supported');
  });

  it('should throw ThunderIDRuntimeError when field.param is null', () => {
    const field: {param: null} = {param: null} as any;
    expect(() => resolveFieldName(field)).toThrow(ThunderIDRuntimeError);
  });

  it('should throw a TypeError when field itself is undefined (current behavior)', () => {
    expect(() => resolveFieldName(undefined as any)).toThrow(TypeError);
  });
});
