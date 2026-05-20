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

import generateStateParamForRequestCorrelation from './generateStateParamForRequestCorrelation';
import OIDCRequestConstants from '../constants/OIDCRequestConstants';
import ThunderIDRuntimeError from '../errors/ThunderIDRuntimeError';
import {ExtendedAuthorizeRequestUrlParams} from '../models/oauth-request';

/**
 * Generates a map of authorization request URL parameters for OIDC authorization requests.
 *
 * This utility ensures the `openid` scope is always included, handles both string and array forms of the `scope` parameter,
 * and supports PKCE and custom parameters. Throws if a code challenge is provided without a code challenge method.
 *
 * @param options - The main options for the authorization request, including redirectUri, clientId, scope, responseMode, codeChallenge, codeChallengeMethod, and prompt.
 * @param pkceOptions - PKCE options, including the PKCE key for state correlation.
 * @param customParams - Optional custom parameters to include in the request (excluding the `state` param, which is handled separately).
 * @returns A Map of key-value pairs representing the authorization request URL parameters.
 *
 * @throws {ThunderIDRuntimeError} If a code challenge is provided without a code challenge method.
 *
 * @example
 * const params = getAuthorizeRequestUrlParams({
 *   options: {
 *     redirectUri: 'https://app/callback',
 *     clientId: 'client123',
 *     scope: ['openid', 'profile'],
 *     responseMode: 'query',
 *     codeChallenge: 'abc',
 *     codeChallengeMethod: 'S256',
 *     prompt: 'login'
 *   },
 *   pkceOptions: { key: 'pkce_code_verifier_1' },
 *   customParams: { foo: 'bar' }
 * });
 * // Returns a Map with all required OIDC params, PKCE, and custom params.
 */
const getAuthorizeRequestUrlParams = (
  options: {
    clientId: string;
    codeChallenge?: string;
    codeChallengeMethod?: string;
    instanceId?: string;
    prompt?: string;
    redirectUri: string;
    responseMode?: string;
    scopes?: string;
  } & ExtendedAuthorizeRequestUrlParams,
  pkceOptions: {key: string},
  customParams: Record<string, string | number | boolean>,
): Map<string, string> => {
  const {redirectUri, clientId, scopes, responseMode, codeChallenge, codeChallengeMethod, prompt} = options;
  const authorizeRequestParams: Map<string, string> = new Map<string, string>();

  authorizeRequestParams.set('response_type', 'code');
  authorizeRequestParams.set('client_id', clientId);

  authorizeRequestParams.set('scope', scopes);
  authorizeRequestParams.set('redirect_uri', redirectUri);

  if (responseMode) {
    authorizeRequestParams.set('response_mode', responseMode);
  }

  const pkceKey: string = pkceOptions?.key;

  if (codeChallenge) {
    authorizeRequestParams.set('code_challenge', codeChallenge);

    if (codeChallengeMethod) {
      authorizeRequestParams.set('code_challenge_method', codeChallengeMethod);
    } else {
      throw new ThunderIDRuntimeError(
        'Code challenge method is required when code challenge is provided.',
        'getAuthorizeRequestUrlParams-ValidationError-001',
        'javascript',
        'When PKCE is enabled, the code challenge method must be provided along with the code challenge.',
      );
    }
  }

  if (prompt) {
    authorizeRequestParams.set('prompt', prompt);
  }

  if (customParams) {
    Object.entries(customParams).forEach(([key, value]: [string, string | number | boolean]) => {
      if (key !== '' && value !== '' && key !== OIDCRequestConstants.Params.STATE) {
        authorizeRequestParams.set(key, value.toString());
      }
    });
  }

  const AUTH_INSTANCE_PREFIX = 'instance_';
  let customStateValue = '';

  if (options.instanceId) {
    customStateValue = AUTH_INSTANCE_PREFIX + options.instanceId;
  } else if (customParams) {
    customStateValue = customParams[OIDCRequestConstants.Params.STATE]?.toString() ?? '';
  }

  authorizeRequestParams.set(
    OIDCRequestConstants.Params.STATE,
    generateStateParamForRequestCorrelation(pkceKey, customStateValue),
  );

  return authorizeRequestParams;
};

export default getAuthorizeRequestUrlParams;
