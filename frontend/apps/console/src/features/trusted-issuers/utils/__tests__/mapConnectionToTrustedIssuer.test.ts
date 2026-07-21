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
import type {TrustedIssuer} from '../../models/trusted-issuer';
import mapConnectionToTrustedIssuer from '../mapConnectionToTrustedIssuer';
import {ConnectionTypes, type ConnectionResponse} from '@thunderid/configure-connections';

const BASE_CONNECTION: ConnectionResponse = {
  id: 'ti-1',
  type: ConnectionTypes.OIDC,
  name: 'Acme Okta',
  clientId: '',
  redirectUri: '',
  authorizationEndpoint: '',
  tokenEndpoint: '',
  issuer: 'https://acme.okta.com',
  jwksEndpoint: 'https://acme.okta.com/keys',
  idJagEnabled: true,
};

describe('mapConnectionToTrustedIssuer', () => {
  it('should map the core trusted-issuer fields', () => {
    const result: TrustedIssuer = mapConnectionToTrustedIssuer(BASE_CONNECTION);

    expect(result).toEqual(
      expect.objectContaining({
        id: 'ti-1',
        name: 'Acme Okta',
        issuer: 'https://acme.okta.com',
        jwksEndpoint: 'https://acme.okta.com/keys',
        idJagEnabled: true,
      }),
    );
  });
});
