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
import extractTenantDomainFromIdTokenPayload from '../extractTenantDomainFromIdTokenPayload';

describe('extractTenantDomainFromIdTokenPayload', (): void => {
  it('should extract tenant domain from sub claim with default separator', (): void => {
    const payload: IdToken = {
      sub: 'user@foo@tenant.com',
    };

    expect(extractTenantDomainFromIdTokenPayload(payload)).toBe('tenant.com');
  });

  it('should extract tenant domain with custom separator', (): void => {
    const payload: IdToken = {
      sub: 'user#foo#custom-tenant',
    };

    expect(extractTenantDomainFromIdTokenPayload(payload, '#')).toBe('custom-tenant');
  });

  it('should return empty string when sub claim is missing', (): void => {
    const payload: IdToken = {} as IdToken;

    expect(extractTenantDomainFromIdTokenPayload(payload)).toBe('');
  });

  it('should return empty string when sub claim has insufficient parts', (): void => {
    const payload: IdToken = {
      sub: 'user@tenant',
    };

    expect(extractTenantDomainFromIdTokenPayload(payload)).toBe('');
  });

  it('should extract the last part when multiple separators exist', () => {
    const payload: IdToken = {
      sub: 'user@foo@bar@tenant.org',
    };
    expect(extractTenantDomainFromIdTokenPayload(payload)).toBe('tenant.org');
  });

  it('should return empty string when sub ends with separator', () => {
    const payload: IdToken = {
      sub: 'user@foo@',
    };
    expect(extractTenantDomainFromIdTokenPayload(payload)).toBe('');
  });

  it('should return empty string when custom separator is not found', () => {
    const payload: IdToken = {
      sub: 'user@foo@tenant.com',
    };
    expect(extractTenantDomainFromIdTokenPayload(payload, '#')).toBe('');
  });

  it('should return empty string when sub is not a string', () => {
    const payload: IdToken = {sub: undefined} as unknown as IdToken;
    expect(extractTenantDomainFromIdTokenPayload(payload)).toBe('');
  });
});
