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

import logger from './logger';
import ThunderIDRuntimeError from '../errors/ThunderIDRuntimeError';

/**
 * Extracts the organization handle from an ThunderID base URL.
 *
 * This function parses ThunderID URLs with the standard pattern:
 * - https://dev.asgardeo.io/t/{orgHandle}
 * - https://stage.asgardeo.io/t/{orgHandle}
 * - https://prod.asgardeo.io/t/{orgHandle}
 * - https://{subdomain}.asgardeo.io/t/{orgHandle}
 *
 * @param baseUrl - The base URL of the ThunderID identity server
 * @returns The extracted organization handle
 * @throws {ThunderIDRuntimeError} When the URL doesn't match the expected ThunderID pattern,
 *   indicating a custom domain is configured and organizationHandle must be provided explicitly
 *
 * @example
 * ```typescript
 * // Standard ThunderID URLs
 * const handle1 = deriveOrganizationHandleFromBaseUrl('https://dev.asgardeo.io/t/dxlab');
 * // Returns: 'dxlab'
 *
 * const handle2 = deriveOrganizationHandleFromBaseUrl('https://stage.asgardeo.io/t/myorg');
 * // Returns: 'myorg'
 *
 * // Custom domain - returns empty string with a warning
 * const handle2 = deriveOrganizationHandleFromBaseUrl('https://custom.example.com/auth');
 * // Returns: '' and logs a warning
 * ```
 */
const deriveOrganizationHandleFromBaseUrl = (baseUrl?: string): string => {
  if (!baseUrl) {
    throw new ThunderIDRuntimeError(
      'Base URL is required to derive organization handle.',
      'javascript-deriveOrganizationHandleFromBaseUrl-ValidationError-001',
      'javascript',
      'A valid base URL must be provided to extract the organization handle.',
    );
  }

  let parsedUrl: URL;

  try {
    parsedUrl = new URL(baseUrl);
  } catch (error) {
    throw new ThunderIDRuntimeError(
      `Invalid base URL format: ${baseUrl}`,
      'javascript-deriveOrganizationHandleFromBaseUrl-ValidationError-002',
      'javascript',
      'The provided base URL does not conform to valid URL syntax.',
    );
  }

  // Extract the organization handle from the path pattern: /t/{orgHandle}
  const pathSegments: string[] = parsedUrl.pathname?.split('/')?.filter((segment: string) => segment?.length > 0);

  if (pathSegments.length < 2 || pathSegments[0] !== 't') {
    logger.warn(
      new ThunderIDRuntimeError(
        'Organization handle is required since a custom domain is configured.',
        'javascript-deriveOrganizationHandleFromBaseUrl-CustomDomainError-002',
        'javascript',
        'The provided base URL does not follow the expected URL pattern (/t/{orgHandle}). Please provide the organizationHandle explicitly in the configuration.',
      ).toString(),
    );

    return '';
  }

  const organizationHandle: string = pathSegments[1];

  if (!organizationHandle || organizationHandle.trim().length === 0) {
    logger.warn(
      new ThunderIDRuntimeError(
        'Organization handle is required since a custom domain is configured.',
        'javascript-deriveOrganizationHandleFromBaseUrl-CustomDomainError-003',
        'javascript',
        'The organization handle could not be extracted from the base URL. Please provide the organizationHandle explicitly in the configuration.',
      ).toString(),
    );

    return '';
  }

  return organizationHandle;
};

export default deriveOrganizationHandleFromBaseUrl;
