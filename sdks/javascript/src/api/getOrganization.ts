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

/**
 * Extended organization interface with additional properties
 */
export interface OrganizationDetails {
  attributes?: Record<string, any>;
  created?: string;
  description?: string;
  id: string;
  lastModified?: string;
  name: string;
  orgHandle: string;
  parent?: {
    id: string;
    ref: string;
  };
  permissions?: string[];
  status?: string;
  type?: string;
}

/**
 * Configuration for the getOrganization request
 */
export interface GetOrganizationConfig extends Omit<RequestInit, 'method'> {
  /**
   * The base URL for the API endpoint.
   */
  baseUrl: string;
  /**
   * Optional custom fetcher function.
   * If not provided, native fetch will be used
   */
  fetcher?: (url: string, config: RequestInit) => Promise<Response>;
  /**
   * The ID of the organization to retrieve
   */
  organizationId: string;
}

/**
 * Retrieves detailed information for a specific organization.
 *
 * @param config - Configuration object containing baseUrl, organizationId, and request config.
 * @returns A promise that resolves with the organization details.
 * @example
 * ```typescript
 * // Using default fetch
 * try {
 *   const organization = await getOrganization({
 *     baseUrl: "https://api.asgardeo.io/t/dxlab",
 *     organizationId: "0d5e071b-d3d3-475d-b3c6-1a20ee2fa9b1"
 *   });
 *   console.log(organization);
 * } catch (error) {
 *   if (error instanceof ThunderIDAPIError) {
 *     console.error('Failed to get organization:', error.message);
 *   }
 * }
 * ```
 *
 * @example
 * ```typescript
 * // Using custom fetcher (e.g., axios-based httpClient)
 * try {
 *   const organization = await getOrganization({
 *     baseUrl: "https://api.asgardeo.io/t/dxlab",
 *     organizationId: "0d5e071b-d3d3-475d-b3c6-1a20ee2fa9b1",
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
 *   console.log(organization);
 * } catch (error) {
 *   if (error instanceof ThunderIDAPIError) {
 *     console.error('Failed to get organization:', error.message);
 *   }
 * }
 * ```
 */
const getOrganization = async ({
  baseUrl,
  organizationId,
  fetcher,
  ...requestConfig
}: GetOrganizationConfig): Promise<OrganizationDetails> => {
  try {
    // eslint-disable-next-line no-new
    new URL(baseUrl);
  } catch (error) {
    throw new ThunderIDAPIError(
      `Invalid base URL provided. ${error?.toString()}`,
      'getOrganization-ValidationError-001',
      'javascript',
      400,
      'The provided `baseUrl` does not adhere to the URL schema.',
    );
  }

  if (!organizationId) {
    throw new ThunderIDAPIError(
      'Organization ID is required',
      'getOrganization-ValidationError-002',
      'javascript',
      400,
      'Invalid Request',
    );
  }

  const fetchFn: typeof fetch = fetcher || fetch;
  const resolvedUrl = `${baseUrl}/api/server/v1/organizations/${organizationId}`;

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
        'getOrganization-ResponseError-001',
        'javascript',
        response.status,
        response.statusText,
        'Failed to fetch organization details',
      );
    }

    return (await response.json()) as OrganizationDetails;
  } catch (error) {
    if (error instanceof ThunderIDAPIError) {
      throw error;
    }

    throw new ThunderIDAPIError(
      `Network or parsing error: ${error instanceof Error ? error.message : 'Unknown error'}`,
      'getOrganization-NetworkError-001',
      'javascript',
      0,
      'Network Error',
    );
  }
};

export default getOrganization;
