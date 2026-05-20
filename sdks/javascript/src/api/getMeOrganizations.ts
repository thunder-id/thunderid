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

import ThunderIDAPIError from '../errors/ThunderIDAPIError';
import {Organization} from '../models/organization';

/**
 * Configuration for the getMeOrganizations request
 */
export interface GetMeOrganizationsConfig extends Omit<RequestInit, 'method'> {
  /**
   * Base64 encoded cursor value for forward pagination
   */
  after?: string;
  /**
   * Authorized application name filter
   */
  authorizedAppName?: string;
  /**
   * The base URL for the API endpoint.
   */
  baseUrl: string;
  /**
   * Base64 encoded cursor value for backward pagination
   */
  before?: string;
  /**
   * Optional custom fetcher function.
   * If not provided, native fetch will be used
   */
  fetcher?: (url: string, config: RequestInit) => Promise<Response>;
  /**
   * Filter expression for organizations
   */
  filter?: string;
  /**
   * Maximum number of organizations to return
   */
  limit?: number;
  /**
   * Whether to include child organizations recursively
   */
  recursive?: boolean;
}

/**
 * Retrieves the organizations associated with the current user.
 *
 * @param config - Configuration object containing baseUrl, optional query parameters, and request config.
 * @returns A promise that resolves with the organizations information.
 * @example
 * ```typescript
 * // Using default fetch
 * try {
 *   const organizations = await getMeOrganizations({
 *     baseUrl: "https://api.asgardeo.io/t/<ORGANIZATION>",
 *     after: "",
 *     before: "",
 *     filter: "",
 *     limit: 10,
 *     recursive: false
 *   });
 *   console.log(organizations);
 * } catch (error) {
 *   if (error instanceof ThunderIDAPIError) {
 *     console.error('Failed to get organizations:', error.message);
 *   }
 * }
 * ```
 *
 * @example
 * ```typescript
 * // Using custom fetcher (e.g., axios-based httpClient)
 * try {
 *   const organizations = await getMeOrganizations({
 *     baseUrl: "https://api.asgardeo.io/t/<ORGANIZATION>",
 *     after: "",
 *     before: "",
 *     filter: "",
 *     limit: 10,
 *     recursive: false,
 *     fetcher: async (url, config) => {
 *       const response = await httpClient({
 *         url,
 *         method: config.method,
 *         headers: config.headers,
 *         ...config
 *       });
 *       // Convert axios-like response to fetch-like Response
 *       return {
 *         ok: response.status >= 200 && response.status < 300,
 *         status: response.status,
 *         statusText: response.statusText,
 *         json: () => Promise.resolve(response.data),
 *         text: () => Promise.resolve(typeof response.data === 'string' ? response.data : JSON.stringify(response.data))
 *       } as Response;
 *     }
 *   });
 *   console.log(organizations);
 * } catch (error) {
 *   if (error instanceof ThunderIDAPIError) {
 *     console.error('Failed to get organizations:', error.message);
 *   }
 * }
 * ```
 */
const getMeOrganizations = async ({
  baseUrl,
  after = '',
  authorizedAppName = '',
  before = '',
  filter = '',
  limit = 10,
  recursive = false,
  fetcher,
  ...requestConfig
}: GetMeOrganizationsConfig): Promise<Organization[]> => {
  try {
    // eslint-disable-next-line no-new
    new URL(baseUrl);
  } catch (error) {
    throw new ThunderIDAPIError(
      `Invalid base URL provided. ${error?.toString()}`,
      'getMeOrganizations-ValidationError-001',
      'javascript',
      400,
      'The provided `baseUrl` does not adhere to the URL schema.',
    );
  }

  const queryParams: URLSearchParams = new URLSearchParams(
    Object.fromEntries(
      Object.entries({
        after,
        authorizedAppName,
        before,
        filter,
        limit: limit.toString(),
        recursive: recursive.toString(),
      }).filter(([, value]: [string, string]) => Boolean(value)),
    ),
  );

  const fetchFn: typeof fetch = fetcher || fetch;
  const resolvedUrl = `${baseUrl}/api/users/v1/me/organizations?${queryParams.toString()}`;

  const requestInit: RequestInit = {
    ...requestConfig,
    headers: {
      Accept: 'application/json',
      'Content-Type': 'application/json',
      ...requestConfig.headers,
    },
    method: 'GET',
  };

  try {
    const response: Response = await fetchFn(resolvedUrl, requestInit);

    if (!response?.ok) {
      const errorText: string = await response.text();

      throw new ThunderIDAPIError(
        errorText,
        'getMeOrganizations-ResponseError-001',
        'javascript',
        response.status,
        response.statusText,
        'Failed to fetch associated organizations of the user',
      );
    }

    const data: Record<string, unknown> = await response.json();
    return (data['organizations'] as Organization[]) || [];
  } catch (error) {
    if (error instanceof ThunderIDAPIError) {
      throw error;
    }

    throw new ThunderIDAPIError(
      `Network or parsing error: ${error instanceof Error ? error.message : 'Unknown error'}`,
      'getMeOrganizations-NetworkError-001',
      'javascript',
      0,
      'Network Error',
    );
  }
};

export default getMeOrganizations;
