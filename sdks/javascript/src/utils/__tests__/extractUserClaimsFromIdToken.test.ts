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

import {describe, expect, it} from 'vitest';
import {IdToken} from '../../models/token';
import extractUserClaimsFromIdToken from '../extractUserClaimsFromIdToken';

describe('extractUserClaimsFromIdToken', (): void => {
  it('should remove protocol claims and keep user claims with original attribute names', (): void => {
    const payload: IdToken = {
      aud: 'client_id',
      email: 'user@example.com',
      exp: 1712345678,
      family_name: 'Doe',
      given_name: 'John',
      iat: 1712345670,
      iss: 'https://example.com',
    };

    const expected: {
      email: string;
      family_name: string;
      given_name: string;
    } = {
      email: 'user@example.com',
      family_name: 'Doe',
      given_name: 'John',
    };

    expect(extractUserClaimsFromIdToken(payload)).toEqual(expected);
  });

  it('should handle empty payload', (): void => {
    const payload: IdToken = {} as IdToken;

    expect(extractUserClaimsFromIdToken(payload)).toEqual({});
  });

  it('should preserve original attribute names without transformation', (): void => {
    const payload: IdToken = {
      custom_claim_value: 'test',
      normalClaim: 'value',
      phone_number: '+1234567890',
    } as IdToken;

    const expected: {
      custom_claim_value: string;
      normalClaim: string;
      phone_number: string;
    } = {
      custom_claim_value: 'test',
      normalClaim: 'value',
      phone_number: '+1234567890',
    };

    expect(extractUserClaimsFromIdToken(payload)).toEqual(expected);
  });

  it('should remove all protocol claims', (): void => {
    const payload: IdToken = {
      acr: '1',
      amr: ['pwd'],
      at_hash: 'hash2',
      aud: 'client_id',
      auth_time: 1712345670,
      azp: 'client_1',
      c_hash: 'hash1',
      custom_claim: 'value',
      exp: 1712345678,
      iat: 1712345670,
      isk: 'key1',
      iss: 'https://example.com',
      nbf: 1712345670,
      nonce: 'abc123',
      sid: 'session1',
    } as IdToken;

    expect(extractUserClaimsFromIdToken(payload)).toEqual({
      custom_claim: 'value',
    });
  });

  it('should preserve non-string claim values such as objects and arrays', () => {
    const payload: IdToken = {
      metadata_info: {active: true, level: 2},
      roles: ['admin', 'editor'],
    } as IdToken;

    expect(extractUserClaimsFromIdToken(payload)).toEqual({
      metadata_info: {active: true, level: 2},
      roles: ['admin', 'editor'],
    });
  });

  it('should retain null and undefined claim values', () => {
    const payload: IdToken = {
      nickname: null,
      preferred_username: undefined,
    } as IdToken;

    expect(extractUserClaimsFromIdToken(payload)).toEqual({
      nickname: null,
      preferred_username: undefined,
    });
  });
});
