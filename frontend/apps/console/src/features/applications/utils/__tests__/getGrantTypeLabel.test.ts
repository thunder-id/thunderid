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

import {describe, it, expect} from 'vitest';
import {getGrantTypeLabel} from '../getGrantTypeLabel';

const t = (_key: string, fallback: string) => fallback;

describe('getGrantTypeLabel', () => {
  it('returns the friendly CIBA label for the CIBA URN', () => {
    expect(getGrantTypeLabel('urn:openid:params:grant-type:ciba', t)).toBe(
      'CIBA (Client-Initiated Backchannel Authentication)',
    );
  });

  it('returns the raw value unchanged for authorization_code', () => {
    expect(getGrantTypeLabel('authorization_code', t)).toBe('authorization_code');
  });

  it('returns the raw value unchanged for an unknown/arbitrary grant type', () => {
    expect(getGrantTypeLabel('urn:ietf:params:oauth:grant-type:token-exchange', t)).toBe(
      'urn:ietf:params:oauth:grant-type:token-exchange',
    );
  });
});
