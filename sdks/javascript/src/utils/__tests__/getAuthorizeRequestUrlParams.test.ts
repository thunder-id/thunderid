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

import {describe, it, expect, vi} from 'vitest';
import OIDCRequestConstants from '../../constants/OIDCRequestConstants';
import ThunderIDRuntimeError from '../../errors/ThunderIDRuntimeError';
import getAuthorizeRequestUrlParams from '../getAuthorizeRequestUrlParams';

vi.mock(
  '../generateStateParamForRequestCorrelation',
  (): {
    default: (pkceKey: string, state?: string) => string;
  } => ({
    default: (pkceKey: string, state: string) => `${state || ''}_request_${pkceKey.split('_').pop()}`,
  }),
);

describe('getAuthorizeRequestUrlParams', (): void => {
  const pkceKey = 'pkce_code_verifier_1';

  it('should include openid in scopes (string)', (): void => {
    const params: Map<string, string> = getAuthorizeRequestUrlParams(
      {
        clientId: 'client123',
        redirectUri: 'https://app/callback',
        scopes: 'openid',
      },
      {key: pkceKey},
      {},
    );

    expect(params.get('scope')).toContain('openid');
  });

  it('should not duplicate openid in scope', (): void => {
    const params: Map<string, string> = getAuthorizeRequestUrlParams(
      {
        clientId: 'client123',
        redirectUri: 'https://app/callback',
        scopes: 'openid profile',
      },
      {key: pkceKey},
      {},
    );
    const scopes: string[] | undefined = params.get('scope')?.split(' ');

    expect(scopes?.filter((s: string): boolean => s === 'openid').length).toBe(1);
  });

  it('should set response_mode if provided', (): void => {
    const params: Map<string, string> = getAuthorizeRequestUrlParams(
      {
        clientId: 'client123',
        redirectUri: 'https://app/callback',
        responseMode: 'fragment',
      },
      {key: pkceKey},
      {},
    );

    expect(params.get('response_mode')).toBe('fragment');
  });

  it('should set code_challenge and code_challenge_method if provided', (): void => {
    const params: Map<string, string> = getAuthorizeRequestUrlParams(
      {
        clientId: 'client123',
        codeChallenge: 'abc',
        codeChallengeMethod: 'S256',
        redirectUri: 'https://app/callback',
      },
      {key: pkceKey},
      {},
    );

    expect(params.get('code_challenge')).toBe('abc');
    expect(params.get('code_challenge_method')).toBe('S256');
  });

  it('should throw if code_challenge is provided without code_challenge_method', (): void => {
    expect((): void => {
      getAuthorizeRequestUrlParams(
        {
          clientId: 'client123',
          codeChallenge: 'abc',
          redirectUri: 'https://app/callback',
        },
        {key: pkceKey},
        {},
      );
    }).toThrow(ThunderIDRuntimeError);
  });

  it('should set prompt if provided', (): void => {
    const params: Map<string, string> = getAuthorizeRequestUrlParams(
      {
        clientId: 'client123',
        prompt: 'login',
        redirectUri: 'https://app/callback',
      },
      {key: pkceKey},
      {},
    );

    expect(params.get('prompt')).toBe('login');
  });

  it('should add custom params except state', (): void => {
    const params: Map<string, string> = getAuthorizeRequestUrlParams(
      {
        clientId: 'client123',
        redirectUri: 'https://app/callback',
      },
      {key: pkceKey},
      {foo: 'bar', [OIDCRequestConstants.Params.STATE]: 'shouldNotAppear'},
    );

    expect(params.get('foo')).toBe('bar');
    expect(params.get(OIDCRequestConstants.Params.STATE)).not.toBe('shouldNotAppear');
  });

  it('should generate state param using pkceKey and custom state', (): void => {
    const params: Map<string, string> = getAuthorizeRequestUrlParams(
      {
        clientId: 'client123',
        redirectUri: 'https://app/callback',
      },
      {key: pkceKey},
      {[OIDCRequestConstants.Params.STATE]: 'customState'},
    );

    expect(params.get(OIDCRequestConstants.Params.STATE)).toBe('customState_request_1');
  });

  it('should set scope to undefined if none provided', (): void => {
    const params: Map<string, string> = getAuthorizeRequestUrlParams(
      {
        clientId: 'client123',
        redirectUri: 'https://app/callback',
      },
      {key: pkceKey},
      {},
    );

    // Since the implementation does not default to "openid"
    expect(params.get('scope')).toBeUndefined();
    expect(params.get('client_id')).toBe('client123');
    expect(params.get('redirect_uri')).toBe('https://app/callback');
  });
});
