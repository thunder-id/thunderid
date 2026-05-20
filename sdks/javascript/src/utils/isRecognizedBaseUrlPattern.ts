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

import logger from './logger';
import ThunderIDRuntimeError from '../errors/ThunderIDRuntimeError';

/**
 * Utility to determine if sensible ThunderID fallbacks can be used based on the given base URL.
 *
 * This checks if the URL follows the standard ThunderID pattern: /t/{orgHandle}
 * Returns true if sensible fallbacks (like deriving organization handle, tenant, etc.) can be used, false otherwise.
 *
 * @param baseUrl - The base URL of the ThunderID identity server (string or undefined)
 * @returns boolean - true if sensible fallbacks can be used, false otherwise
 *
 * @example
 * isRecognizedBaseUrlPattern('https://dev.asgardeo.io/t/dxlab'); // true
 * isRecognizedBaseUrlPattern('https://custom.example.com/auth'); // false
 */
const isRecognizedBaseUrlPattern = (baseUrl: string | undefined): boolean => {
  if (!baseUrl) {
    throw new ThunderIDRuntimeError(
      'Base URL is required to derive if the `baseUrl` is recognized.',
      'isRecognizedBaseUrlPattern-ValidationError-001',
      'javascript',
      'A valid base URL must be provided to derive if the `baseUrl` is recognized to use the sensible fallbacks.',
    );
  }

  let parsedUrl: URL;

  try {
    parsedUrl = new URL(baseUrl);
  } catch (error) {
    throw new ThunderIDRuntimeError(
      `Invalid base URL format: ${baseUrl}`,
      'isRecognizedBaseUrlPattern-ValidationError-002',
      'javascript',
      'The provided base URL does not conform to valid URL syntax.',
    );
  }

  // Extract the organization handle from the path pattern: /t/{orgHandle}
  const pathSegments: string[] = parsedUrl.pathname?.split('/')?.filter((segment: string) => segment?.length > 0);

  if (pathSegments.length < 2 || pathSegments[0] !== 't') {
    logger.warn(
      '[isRecognizedBaseUrlPattern] The provided base URL does not follow the expected URL pattern (/t/{orgHandle}).',
    );

    return false;
  }

  return true;
};

export default isRecognizedBaseUrlPattern;
