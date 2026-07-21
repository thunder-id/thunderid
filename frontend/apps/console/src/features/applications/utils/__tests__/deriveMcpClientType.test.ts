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
import deriveMcpClientType from '../deriveMcpClientType';

describe('deriveMcpClientType', () => {
  it('returns m2m when only client_credentials is granted', () => {
    expect(deriveMcpClientType(['client_credentials'])).toBe('m2m');
  });

  it('returns userDelegated when authorization_code and client_credentials are both granted', () => {
    expect(deriveMcpClientType(['authorization_code', 'client_credentials'])).toBe('userDelegated');
  });

  it('returns userDelegated when only authorization_code and refresh_token are granted', () => {
    expect(deriveMcpClientType(['authorization_code', 'refresh_token'])).toBe('userDelegated');
  });

  it('returns userDelegated when grantTypes is undefined', () => {
    expect(deriveMcpClientType(undefined)).toBe('userDelegated');
  });

  it('returns userDelegated when grantTypes is empty', () => {
    expect(deriveMcpClientType([])).toBe('userDelegated');
  });
});
