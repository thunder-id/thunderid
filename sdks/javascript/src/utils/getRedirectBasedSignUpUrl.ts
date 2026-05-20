/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com). All Rights Reserved.
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

import identifyPlatform from './identifyPlatform';
import isRecognizedBaseUrlPattern from './isRecognizedBaseUrlPattern';
import logger from './logger';
import {Config} from '../models/config';
import {Platform} from '../models/platforms';

/**
 * Utility to generate the redirect-based sign-up URL for ThunderID.
 *
 * If the baseUrl is recognized (standard ThunderID pattern), constructs the sign-up URL.
 * Otherwise, returns an empty string.
 *
 * @param baseUrl - The base URL of the ThunderID identity server (string or undefined)
 * @returns The sign-up URL if baseUrl is recognized, otherwise an empty string
 */
const getRedirectBasedSignUpUrl = (config: Config): string => {
  const {baseUrl} = config;

  if (!isRecognizedBaseUrlPattern(baseUrl)) return '';

  let signUpBaseUrl: string = baseUrl;

  if (identifyPlatform(config) === Platform.ThunderID) {
    try {
      const url: URL = new URL(baseUrl);

      // Replace 'api.' with 'accounts.' in the hostname, preserving subdomains like 'dev.'
      if (/([a-z0-9-]+\.)*api\.thunderid\.io$/i.test(url.hostname)) {
        url.hostname = url.hostname.replace('api.', 'accounts.');
        signUpBaseUrl = url.toString().replace(/\/$/, ''); // Remove trailing slash if any
      }
    } catch {
      logger.debug(
        `[getRedirectBasedSignUpUrl] Could not parse base URL to replace 'api.' with 'accounts.'. Base URL: ${baseUrl}`,
      );
    }
  }

  const url: URL = new URL(`${signUpBaseUrl}/accountrecoveryendpoint/register.do`);

  if (config.clientId) {
    url.searchParams.set('client_id', config.clientId);
  }

  if (config.applicationId) {
    url.searchParams.set('spId', config.applicationId);
  }

  logger.debug(`[getRedirectBasedSignUpUrl] Generated sign-up URL: ${url.toString()}`);

  return url.toString();
};

export default getRedirectBasedSignUpUrl;
