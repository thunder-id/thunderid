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
import {AuthenticatorTypes} from '../authenticators';
import type {AuthenticatorType} from '../authenticators';

describe('AuthenticatorTypes', () => {
  it('should have CREDENTIALS_AUTH defined with correct value', () => {
    expect(AuthenticatorTypes.CREDENTIALS_AUTH).toBe('credentials_auth');
  });

  it('should have PASSKEY defined with correct value', () => {
    expect(AuthenticatorTypes.PASSKEY).toBe('passkey');
  });

  it('should be a const object with expected keys', () => {
    expect(Object.keys(AuthenticatorTypes)).toEqual(['CREDENTIALS_AUTH', 'PASSKEY']);
  });

  it('should allow type-safe assignment', () => {
    const authenticator: AuthenticatorType = AuthenticatorTypes.CREDENTIALS_AUTH;
    expect(authenticator).toBe('credentials_auth');
  });

  it('should allow type-safe assignment for PASSKEY', () => {
    const authenticator: AuthenticatorType = AuthenticatorTypes.PASSKEY;
    expect(authenticator).toBe('passkey');
  });
});
