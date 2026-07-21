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
import getMcpDiscoveryEndpointRows from '../getMcpDiscoveryEndpointRows';

const identityT = (_key: string, fallback: string): string => fallback;

describe('getMcpDiscoveryEndpointRows', () => {
  it('returns all rows derived from a fully populated wellKnown document', () => {
    const rows = getMcpDiscoveryEndpointRows(
      {
        issuer: 'https://localhost:8090',
        authorization_endpoint: 'https://localhost:8090/oauth2/authorize',
        token_endpoint: 'https://localhost:8090/oauth2/token',
      },
      identityT,
    );

    expect(rows).toEqual([
      {key: 'issuer', label: 'Issuer', value: 'https://localhost:8090'},
      {
        key: 'oauthMetadata',
        label: 'Authorization server metadata',
        value: 'https://localhost:8090/.well-known/oauth-authorization-server',
      },
      {
        key: 'oidcDiscovery',
        label: 'OpenID Connect discovery',
        value: 'https://localhost:8090/.well-known/openid-configuration',
      },
      {key: 'authorize', label: 'Authorization endpoint', value: 'https://localhost:8090/oauth2/authorize'},
      {key: 'token', label: 'Token endpoint', value: 'https://localhost:8090/oauth2/token'},
    ]);
  });

  it('omits the derived metadata/discovery rows when the issuer is missing', () => {
    const rows = getMcpDiscoveryEndpointRows(
      {authorization_endpoint: 'https://localhost:8090/oauth2/authorize'},
      identityT,
    );

    expect(rows.map((row) => row.key)).toEqual(['authorize']);
  });

  it('returns an empty array when wellKnown is undefined', () => {
    expect(getMcpDiscoveryEndpointRows(undefined, identityT)).toEqual([]);
  });

  it('returns an empty array when wellKnown is null', () => {
    expect(getMcpDiscoveryEndpointRows(null, identityT)).toEqual([]);
  });

  it('uses the translation function to resolve each row label', () => {
    const t = (key: string): string => `translated:${key}`;

    const rows = getMcpDiscoveryEndpointRows({issuer: 'https://localhost:8090'}, t);

    expect(rows.find((row) => row.key === 'issuer')?.label).toBe(
      'translated:applications:onboarding.mcp.complete.endpoints.issuer',
    );
  });
});
