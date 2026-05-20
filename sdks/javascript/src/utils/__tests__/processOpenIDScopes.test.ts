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
import processOpenIDScopes from '../processOpenIDScopes';

vi.mock('../../constants/OIDCRequestConstants', () => ({
  default: {
    SignIn: {
      Payload: {
        DEFAULT_SCOPES: ['openid', 'email'],
      },
    },
  },
}));

describe('processOpenIDScopes', () => {
  it('should return user-configured string scopes exactly as provided (no default injection)', () => {
    const input = 'email openid profile';
    const out: string = processOpenIDScopes(input);
    expect(out).toBe('email openid profile');
  });

  it('should return user-configured string scopes without injecting defaults', () => {
    const input = 'profile';
    const out: string = processOpenIDScopes(input);
    expect(out).toBe('profile');
  });

  it('should return user-configured array scopes joined as a string without injecting defaults', () => {
    const input: string[] = ['profile', 'email'];
    const out: string = processOpenIDScopes(input);
    expect(out).toBe('profile email');
  });

  it('should return user-configured array scopes without duplicating values', () => {
    const input: string[] = ['openid', 'email'];
    const out: string = processOpenIDScopes(input);
    expect(out).toBe('openid email');
  });

  it('should return only defaults for an empty string (not configured)', () => {
    const input = '';
    const out: string = processOpenIDScopes(input);
    expect(out).toBe('openid email');
  });

  it('should return only defaults for an empty array (not configured)', () => {
    const input: string[] = [];
    const out: string = processOpenIDScopes(input);
    expect(out).toBe('openid email');
  });

  it('should return only defaults when scopes is undefined (not configured)', () => {
    const out: string = processOpenIDScopes(undefined);
    expect(out).toBe('openid email');
  });

  it('should return only defaults when scopes is null (not configured)', () => {
    const out: string = processOpenIDScopes(null);
    expect(out).toBe('openid email');
  });

  it('should throw ThunderIDRuntimeError for non-string/array input (number)', () => {
    expect(() => processOpenIDScopes(123)).toThrow(ThunderIDRuntimeError);
  });

  it('should throw ThunderIDRuntimeError for non-string/array input (object)', () => {
    expect(() => processOpenIDScopes({})).toThrow(ThunderIDRuntimeError);
  });

  it('should return custom scopes exactly without appending defaults', () => {
    const input = 'custom-scope another';
    const out: string = processOpenIDScopes(input);
    expect(out).toBe('custom-scope another');
  });
});
