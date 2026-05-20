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

import OIDCRequestConstants from '../constants/OIDCRequestConstants';
import ThunderIDRuntimeError from '../errors/ThunderIDRuntimeError';

/**
 * Processes OpenID scopes to ensure they are in the correct format.
 * If the input is a string, it returns it as is.
 * If the input is an array, it joins the elements into a single string separated by spaces.
 * If the input is neither, it throws an error.
 *
 * Default scopes are only injected when no scopes are configured (undefined, empty string,
 * or empty array). If the caller explicitly provides scopes, those are used as-is.
 *
 * @param scopes - The OpenID scopes to process, which can be a string, an array of strings,
 *   or undefined/null when not configured.
 * @returns A string of OpenID scopes separated by spaces.
 *
 * @example
 * ```typescript
 * processOpenIDScopes("openid profile email"); // returns "openid profile email"
 * processOpenIDScopes(["openid", "profile", "email"]); // returns "openid profile email"
 * processOpenIDScopes(undefined); // returns default scopes
 * processOpenIDScopes(123); // throws ThunderIDRuntimeError
 * processOpenIDScopes({}); // throws ThunderIDRuntimeError
 * ```
 */
const processOpenIDScopes = (scopes: string | string[] | undefined | null): string => {
  let processedScopes: string[] = [];
  let userConfiguredScopes = false;

  if (scopes !== undefined && scopes !== null) {
    if (Array.isArray(scopes)) {
      processedScopes = scopes;
      userConfiguredScopes = scopes.length > 0;
    } else if (typeof scopes === 'string') {
      processedScopes = scopes ? scopes.split(' ') : [];
      userConfiguredScopes = scopes.length > 0;
    } else {
      throw new ThunderIDRuntimeError(
        'Scopes must be a string or an array of strings.',
        'processOpenIDScopes-Invalid-001',
        'javascript',
        'The provided scopes are not in the expected format. Please provide a string or an array of strings.',
      );
    }
  }

  if (!userConfiguredScopes) {
    OIDCRequestConstants.SignIn.Payload.DEFAULT_SCOPES.forEach((defaultScope: string) => {
      if (!processedScopes.includes(defaultScope)) {
        processedScopes.push(defaultScope);
      }
    });
  }

  return processedScopes.join(' ');
};

export default processOpenIDScopes;
