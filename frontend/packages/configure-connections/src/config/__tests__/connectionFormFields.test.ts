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
import {ConnectionTypes} from '../../models/connection';
import {fieldsForMode} from '../connectionFormFields';

function fieldNames(type: (typeof ConnectionTypes)[keyof typeof ConnectionTypes], mode: 'create' | 'edit'): string[] {
  return fieldsForMode(type, mode).map((field) => field.name);
}

describe('fieldsForMode', () => {
  it('hides the redirect URI and scopes on create for Google/GitHub, showing them on edit', () => {
    expect(fieldNames(ConnectionTypes.GOOGLE, 'create')).toEqual(['name', 'clientId', 'clientSecret']);
    expect(fieldNames(ConnectionTypes.GOOGLE, 'edit')).toEqual([
      'name',
      'clientId',
      'clientSecret',
      'redirectUri',
      'scopes',
    ]);
  });

  it('shows only the required fields for OIDC on create, all fields on edit', () => {
    expect(fieldNames(ConnectionTypes.OIDC, 'create')).toEqual([
      'name',
      'clientId',
      'clientSecret',
      'authorizationEndpoint',
      'tokenEndpoint',
    ]);
    expect(fieldNames(ConnectionTypes.OIDC, 'edit')).toEqual([
      'name',
      'clientId',
      'clientSecret',
      'authorizationEndpoint',
      'tokenEndpoint',
      'issuer',
      'userInfoEndpoint',
      'jwksEndpoint',
      'redirectUri',
      'scopes',
      'tokenExchangeEnabled',
      'trustedTokenAudience',
    ]);
  });

  it('shows only the required fields for OAuth 2.0 on create, all fields on edit', () => {
    expect(fieldNames(ConnectionTypes.OAUTH, 'create')).toEqual([
      'name',
      'clientId',
      'clientSecret',
      'authorizationEndpoint',
      'tokenEndpoint',
      'userInfoEndpoint',
    ]);
    expect(fieldNames(ConnectionTypes.OAUTH, 'edit')).toEqual([
      'name',
      'clientId',
      'clientSecret',
      'authorizationEndpoint',
      'tokenEndpoint',
      'userInfoEndpoint',
      'redirectUri',
      'scopes',
    ]);
  });

  it('does not hide any SMS vendor fields on create', () => {
    expect(fieldNames(ConnectionTypes.TWILIO, 'create')).toEqual(fieldNames(ConnectionTypes.TWILIO, 'edit'));
    expect(fieldNames(ConnectionTypes.VONAGE, 'create')).toEqual(fieldNames(ConnectionTypes.VONAGE, 'edit'));
  });
});
